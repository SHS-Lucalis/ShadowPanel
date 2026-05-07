package acme

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gameap/gameap/internal/acme/http01"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
	"github.com/pkg/errors"
)

const (
	ChallengeHTTP01 = "http-01"
	ChallengeDNS01  = "dns-01"
)

// LegoClient is the subset of lego.Client used by Service. Exposing this as
// an interface enables tests to substitute a fake client without standing up
// a real ACME directory.
type LegoClient interface {
	ObtainCertificate(ctx context.Context, request certificate.ObtainRequest) (*certificate.Resource, error)
	RenewCertificate(ctx context.Context, resource certificate.Resource) (*certificate.Resource, error)
	Register(ctx context.Context, termsOfServiceAgreed bool) (*registration.Resource, error)
}

// LegoFactory builds a LegoClient bound to the supplied user / config.
type LegoFactory func(
	user registration.User,
	cfg ServiceConfig,
	deps LegoFactoryDeps,
) (LegoClient, error)

// LegoFactoryDeps groups the runtime dependencies needed to wire a challenge
// solver onto the lego client. Only the field matching cfg.ChallengeType is
// expected to be non-nil; the factory ignores the others.
type LegoFactoryDeps struct {
	DNSRegistry    DNSProviderRegistry
	HTTP01Provider challenge.Provider
}

type Service struct {
	cfg            ServiceConfig
	storage        Storage
	locker         Locker
	dnsRegistry    DNSProviderRegistry
	http01Provider *http01.Provider
	logger         *slog.Logger
	factory        LegoFactory

	mu          sync.RWMutex
	currentCert atomic.Pointer[tls.Certificate]
	currentRes  *certificate.Resource
	user        *acmeUser
	status      Status

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type ServiceConfig struct {
	ChallengeType        string
	Email                string
	Domains              []string
	DirectoryURL         string
	DNSProvider          string
	RenewalThreshold     time.Duration
	RenewalCheckInterval time.Duration
	PropagationTimeout   time.Duration
	LockTTL              time.Duration
}

func NewService(
	cfg ServiceConfig,
	storage Storage,
	locker Locker,
	dnsRegistry DNSProviderRegistry,
	logger *slog.Logger,
) *Service {
	if logger == nil {
		logger = slog.Default()
	}

	if cfg.LockTTL <= 0 {
		cfg.LockTTL = 60 * time.Second
	}

	if cfg.ChallengeType == "" {
		cfg.ChallengeType = ChallengeHTTP01
	}

	s := &Service{
		cfg:         cfg,
		storage:     storage,
		locker:      locker,
		dnsRegistry: dnsRegistry,
		logger:      logger.With(slog.String("component", "acme")),
		factory:     defaultLegoFactory,
		status: Status{
			Enabled:       true,
			State:         StatePending,
			ChallengeType: cfg.ChallengeType,
			Domains:       append([]string(nil), cfg.Domains...),
			DNSProvider:   cfg.DNSProvider,
		},
	}

	if cfg.ChallengeType == ChallengeHTTP01 {
		s.http01Provider = http01.New()
	}

	return s
}

// HTTP01Handler returns the HTTP-01 challenge handler when challenge type is
// http-01, or nil when DNS-01 is in use. The router mounts this under
// /.well-known/acme-challenge/ ahead of the SPA fallback. Safe to call on
// a nil receiver — test containers and ACME-disabled deployments do not
// construct a Service.
func (s *Service) HTTP01Handler() http.Handler {
	if s == nil || s.http01Provider == nil {
		return nil
	}

	return s.http01Provider.Handler()
}

func (s *Service) legoDeps() LegoFactoryDeps {
	deps := LegoFactoryDeps{
		DNSRegistry: s.dnsRegistry,
	}

	if s.http01Provider != nil {
		deps.HTTP01Provider = s.http01Provider
	}

	return deps
}

// SetLegoFactory overrides the lego client factory (test seam).
func (s *Service) SetLegoFactory(factory LegoFactory) {
	s.factory = factory
}

func (s *Service) Start(ctx context.Context) error {
	if len(s.cfg.Domains) == 0 {
		return errors.New("no domains configured")
	}

	if err := s.ensureAccount(ctx); err != nil {
		return errors.WithMessage(err, "failed to ensure ACME account")
	}

	if err := s.ensureCertificate(ctx); err != nil {
		s.setFailed(err)

		return errors.WithMessage(err, "failed to ensure certificate")
	}

	s.setActive()

	loopCtx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.wg.Add(1)
	go s.renewalLoop(loopCtx)

	return nil
}

func (s *Service) Stop(_ context.Context) error {
	if s.cancel != nil {
		s.cancel()
	}

	s.wg.Wait()

	return nil
}

func (s *Service) GetCertificate(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
	cert := s.currentCert.Load()
	if cert == nil {
		return nil, errors.New("no certificate available")
	}

	return cert, nil
}

func (s *Service) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()

	st := s.status
	st.Domains = append([]string(nil), s.status.Domains...)

	return st
}

func (s *Service) ForceRenew(ctx context.Context) error {
	s.mu.Lock()
	s.status.State = StateRenewing
	s.mu.Unlock()

	if err := s.renew(ctx); err != nil {
		s.setFailed(err)

		return err
	}

	s.setActive()

	return nil
}

func (s *Service) renewalLoop(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.cfg.RenewalCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tickRenewal(ctx)
		}
	}
}

func (s *Service) tickRenewal(ctx context.Context) {
	s.mu.RLock()
	cert := s.currentCert.Load()
	s.mu.RUnlock()

	if cert == nil || cert.Leaf == nil {
		return
	}

	if time.Until(cert.Leaf.NotAfter) > s.cfg.RenewalThreshold {
		s.updateNextCheck()

		return
	}

	s.logger.Info("certificate near expiry, attempting renewal",
		slog.Time("not_after", cert.Leaf.NotAfter),
	)

	if err := s.renew(ctx); err != nil {
		s.logger.Error("renewal failed", slog.String("error", err.Error()))
		s.setFailed(err)

		return
	}

	s.setActive()
}

func (s *Service) loadExistingAccount(ctx context.Context) error {
	acct, err := s.storage.LoadAccount(ctx, s.cfg.Email)
	if err != nil {
		return errors.WithMessage(err, "failed to load account")
	}

	key, err := parsePEMPrivateKey(acct.PrivateKey)
	if err != nil {
		return errors.WithMessage(err, "failed to parse account key")
	}

	var reg registration.Resource
	if len(acct.Registration) > 0 {
		if err := json.Unmarshal(acct.Registration, &reg); err != nil {
			return errors.Wrap(err, "failed to unmarshal registration")
		}
	}

	s.user = &acmeUser{email: acct.Email, key: key, registration: &reg}

	return nil
}

func (s *Service) ensureAccount(ctx context.Context) error {
	exists, err := s.storage.HasAccount(ctx, s.cfg.Email)
	if err != nil {
		return errors.WithMessage(err, "failed to check account existence")
	}

	if exists {
		return s.loadExistingAccount(ctx)
	}

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return errors.Wrap(err, "failed to generate account key")
	}

	user := &acmeUser{email: s.cfg.Email, key: privateKey}

	client, err := s.factory(user, s.cfg, s.legoDeps())
	if err != nil {
		return errors.WithMessage(err, "failed to create lego client for registration")
	}

	reg, err := client.Register(ctx, true)
	if err != nil {
		return errors.WithMessage(err, "failed to register ACME account")
	}

	user.registration = reg

	keyPEM, err := encodePrivateKeyPEM(privateKey)
	if err != nil {
		return errors.WithMessage(err, "failed to encode account key")
	}

	regBytes, err := json.Marshal(reg)
	if err != nil {
		return errors.Wrap(err, "failed to marshal registration")
	}

	if err := s.storage.SaveAccount(ctx, &Account{
		Email:        s.cfg.Email,
		PrivateKey:   keyPEM,
		Registration: regBytes,
	}); err != nil {
		return errors.WithMessage(err, "failed to save account")
	}

	s.user = user

	return nil
}

func (s *Service) ensureCertificate(ctx context.Context) error {
	key := certificateKey(s.cfg.Domains)

	exists, err := s.storage.HasResource(ctx, key)
	if err != nil {
		return errors.WithMessage(err, "failed to check resource existence")
	}

	if exists {
		resource, loadErr := s.storage.LoadResource(ctx, key)
		if loadErr != nil {
			return errors.WithMessage(loadErr, "failed to load resource")
		}

		cert, parseErr := tlsCertificateFromResource(resource)
		if parseErr != nil {
			s.logger.Warn("stored certificate is invalid, will reissue",
				slog.String("error", parseErr.Error()),
			)
		} else if cert.Leaf != nil && time.Until(cert.Leaf.NotAfter) > s.cfg.RenewalThreshold {
			s.applyCertificate(resource, cert)

			return nil
		}
	}

	return s.obtain(ctx, key)
}

func (s *Service) obtain(ctx context.Context, key string) error {
	lock, err := s.locker.Acquire(ctx, key, s.cfg.LockTTL)
	if err != nil {
		return errors.WithMessage(err, "failed to acquire lock")
	}
	defer func() {
		_ = lock.Release(ctx)
	}()

	client, err := s.factory(s.user, s.cfg, s.legoDeps())
	if err != nil {
		return errors.WithMessage(err, "failed to create lego client")
	}

	resource, err := client.ObtainCertificate(ctx, certificate.ObtainRequest{
		Domains: s.cfg.Domains,
		Bundle:  true,
	})
	if err != nil {
		return errors.WithMessage(err, "failed to obtain certificate")
	}

	if err := s.storage.SaveResource(ctx, key, resource); err != nil {
		return errors.WithMessage(err, "failed to save certificate resource")
	}

	cert, err := tlsCertificateFromResource(resource)
	if err != nil {
		return errors.WithMessage(err, "failed to parse obtained certificate")
	}

	s.applyCertificate(resource, cert)

	return nil
}

func (s *Service) renew(ctx context.Context) error {
	s.mu.RLock()
	resource := s.currentRes
	s.mu.RUnlock()

	if resource == nil {
		return s.obtain(ctx, certificateKey(s.cfg.Domains))
	}

	key := certificateKey(s.cfg.Domains)

	lock, err := s.locker.Acquire(ctx, key, s.cfg.LockTTL)
	if err != nil {
		return errors.WithMessage(err, "failed to acquire lock for renewal")
	}
	defer func() {
		_ = lock.Release(ctx)
	}()

	client, err := s.factory(s.user, s.cfg, s.legoDeps())
	if err != nil {
		return errors.WithMessage(err, "failed to create lego client for renewal")
	}

	renewed, err := client.RenewCertificate(ctx, *resource)
	if err != nil {
		return errors.WithMessage(err, "failed to renew certificate")
	}

	if err := s.storage.SaveResource(ctx, key, renewed); err != nil {
		return errors.WithMessage(err, "failed to save renewed resource")
	}

	cert, err := tlsCertificateFromResource(renewed)
	if err != nil {
		return errors.WithMessage(err, "failed to parse renewed certificate")
	}

	s.applyCertificate(renewed, cert)

	return nil
}

func (s *Service) applyCertificate(resource *certificate.Resource, cert *tls.Certificate) {
	s.currentCert.Store(cert)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.currentRes = resource

	if cert != nil && cert.Leaf != nil {
		s.status.NotBefore = cert.Leaf.NotBefore
		s.status.NotAfter = cert.Leaf.NotAfter
	}

	s.status.LastRenewalAt = time.Now()
	s.status.NextRenewalCheckAt = time.Now().Add(s.cfg.RenewalCheckInterval)
	s.status.LastError = ""
}

func (s *Service) setActive() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.status.State = StateActive
	s.status.LastError = ""
}

func (s *Service) setFailed(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.status.State = StateFailed
	s.status.LastError = err.Error()
}

func (s *Service) updateNextCheck() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.status.NextRenewalCheckAt = time.Now().Add(s.cfg.RenewalCheckInterval)
}

type acmeUser struct {
	email        string
	key          crypto.PrivateKey
	registration *registration.Resource
}

func (u *acmeUser) GetEmail() string                        { return u.email }
func (u *acmeUser) GetPrivateKey() crypto.PrivateKey        { return u.key }
func (u *acmeUser) GetRegistration() *registration.Resource { return u.registration }

func certificateKey(domains []string) string {
	sorted := append([]string(nil), domains...)
	sort.Strings(sorted)

	return strings.Join(sorted, ",")
}

func tlsCertificateFromResource(resource *certificate.Resource) (*tls.Certificate, error) {
	if resource == nil {
		return nil, errors.New("resource is nil")
	}

	cert, err := tls.X509KeyPair(resource.Certificate, resource.PrivateKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse cert/key pair")
	}

	if len(cert.Certificate) > 0 {
		leaf, parseErr := x509.ParseCertificate(cert.Certificate[0])
		if parseErr != nil {
			return nil, errors.Wrap(parseErr, "failed to parse leaf certificate")
		}

		cert.Leaf = leaf
	}

	return &cert, nil
}

func parsePEMPrivateKey(data []byte) (crypto.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	return nil, errors.New("unsupported private key format")
}

func encodePrivateKeyPEM(key crypto.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal private key")
	}

	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

func defaultLegoFactory(
	user registration.User,
	cfg ServiceConfig,
	deps LegoFactoryDeps,
) (LegoClient, error) {
	legoCfg := lego.NewConfig(user)
	legoCfg.CADirURL = cfg.DirectoryURL
	legoCfg.Certificate.KeyType = certcrypto.RSA2048

	client, err := lego.NewClient(legoCfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to construct lego client")
	}

	switch cfg.ChallengeType {
	case ChallengeHTTP01:
		if deps.HTTP01Provider == nil {
			return nil, errors.New("HTTP-01 provider is nil")
		}

		if err := client.Challenge.SetHTTP01Provider(deps.HTTP01Provider); err != nil {
			return nil, errors.Wrap(err, "failed to set HTTP-01 provider")
		}
	case ChallengeDNS01:
		if cfg.DNSProvider == "" {
			return nil, errors.New("DNS provider identifier is empty")
		}

		if deps.DNSRegistry == nil {
			return nil, errors.New("DNS provider registry is nil")
		}

		provider, resolveErr := deps.DNSRegistry.Resolve(context.Background(), cfg.DNSProvider)
		if resolveErr != nil {
			return nil, errors.WithMessage(resolveErr, fmt.Sprintf("failed to resolve DNS provider %q", cfg.DNSProvider))
		}

		if err := client.Challenge.SetDNS01Provider(provider); err != nil {
			return nil, errors.Wrap(err, "failed to set DNS-01 provider")
		}
	default:
		return nil, errors.Errorf("unsupported challenge type %q", cfg.ChallengeType)
	}

	return &legoClientAdapter{client: client}, nil
}

type legoClientAdapter struct {
	client *lego.Client
}

func (a *legoClientAdapter) ObtainCertificate(
	_ context.Context,
	request certificate.ObtainRequest,
) (*certificate.Resource, error) {
	return a.client.Certificate.Obtain(request)
}

func (a *legoClientAdapter) RenewCertificate(
	_ context.Context,
	resource certificate.Resource,
) (*certificate.Resource, error) {
	return a.client.Certificate.RenewWithOptions(resource, &certificate.RenewOptions{Bundle: true})
}

func (a *legoClientAdapter) Register(_ context.Context, tos bool) (*registration.Resource, error) {
	return a.client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: tos})
}
