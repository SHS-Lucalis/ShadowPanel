package acme_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pkg/errors"

	"github.com/gameap/gameap/internal/acme"
	"github.com/gameap/gameap/internal/acme/locker"
	"github.com/gameap/gameap/internal/acme/storage"
	"github.com/gameap/gameap/internal/files"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/registration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeClient struct {
	registerCalls atomic.Int32
	obtainCalls   atomic.Int32
	renewCalls    atomic.Int32
	obtainFn      func() (*certificate.Resource, error)
	renewFn       func(certificate.Resource) (*certificate.Resource, error)
}

func (c *fakeClient) Register(_ context.Context, _ bool) (*registration.Resource, error) {
	c.registerCalls.Add(1)

	return &registration.Resource{URI: "https://test/reg/1"}, nil
}

func (c *fakeClient) ObtainCertificate(_ context.Context, _ certificate.ObtainRequest) (*certificate.Resource, error) {
	c.obtainCalls.Add(1)

	return c.obtainFn()
}

func (c *fakeClient) RenewCertificate(_ context.Context, res certificate.Resource) (*certificate.Resource, error) {
	c.renewCalls.Add(1)

	return c.renewFn(res)
}

type fakeRegistry struct{}

func (fakeRegistry) Resolve(_ context.Context, _ string) (challenge.Provider, error) {
	return fakeProvider{}, nil
}

type fakeProvider struct{}

func (fakeProvider) Present(_, _, _ string) error { return nil }
func (fakeProvider) CleanUp(_, _, _ string) error { return nil }

func TestService_Status_DefaultsToPending(t *testing.T) {
	svc := newServiceForTest(t, nil)

	st := svc.Status()
	assert.True(t, st.Enabled)
	assert.Equal(t, acme.StatePending, st.State)
	assert.Equal(t, []string{"example.com"}, st.Domains)
}

func TestService_HTTP01Handler_NilForDNS01(t *testing.T) {
	cfg := serviceConfigFor("example.com")
	cfg.ChallengeType = acme.ChallengeDNS01

	fm := files.NewInMemoryFileManager()
	st := storage.NewFileStorage(fm, "acme")
	svc := acme.NewService(cfg, st, locker.NewInMemoryLocker(), fakeRegistry{}, nil)

	assert.Nil(t, svc.HTTP01Handler())
}

func TestService_HTTP01Handler_PresentForHTTP01(t *testing.T) {
	cfg := serviceConfigFor("example.com")
	cfg.ChallengeType = acme.ChallengeHTTP01

	fm := files.NewInMemoryFileManager()
	st := storage.NewFileStorage(fm, "acme")
	svc := acme.NewService(cfg, st, locker.NewInMemoryLocker(), nil, nil)

	assert.NotNil(t, svc.HTTP01Handler())
}

func TestService_GetCertificate_ReturnsErrorBeforeStart(t *testing.T) {
	svc := newServiceForTest(t, nil)

	cert, err := svc.GetCertificate(nil)
	require.Error(t, err)
	assert.Nil(t, cert)
	assert.Contains(t, err.Error(), "no certificate available")
}

func TestService_Start_ObtainsAndCachesCertificate(t *testing.T) {
	resource := generateTestResource(t, "example.com", time.Now().Add(60*24*time.Hour))

	client := &fakeClient{
		obtainFn: func() (*certificate.Resource, error) {
			return resource, nil
		},
	}

	svc := newServiceForTest(t, client)

	require.NoError(t, svc.Start(context.Background()))
	t.Cleanup(func() { _ = svc.Stop(context.Background()) })

	assert.Equal(t, int32(1), client.registerCalls.Load())
	assert.Equal(t, int32(1), client.obtainCalls.Load())

	cert, err := svc.GetCertificate(nil)
	require.NoError(t, err)
	require.NotNil(t, cert)
	require.NotNil(t, cert.Leaf)
	assert.Equal(t, "example.com", cert.Leaf.Subject.CommonName)

	st := svc.Status()
	assert.Equal(t, acme.StateActive, st.State)
	assert.WithinDuration(t, time.Now(), st.LastRenewalAt, 5*time.Second)
}

func TestService_Start_FailsWhenObtainFails(t *testing.T) {
	leUnreachableErr := errors.New("LE unreachable")

	client := &fakeClient{
		obtainFn: func() (*certificate.Resource, error) {
			return nil, leUnreachableErr
		},
	}

	svc := newServiceForTest(t, client)

	err := svc.Start(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LE unreachable")

	st := svc.Status()
	assert.Equal(t, acme.StateFailed, st.State)
	assert.Contains(t, st.LastError, "LE unreachable")
}

func TestService_ForceRenew_ReplacesCertificate(t *testing.T) {
	original := generateTestResource(t, "example.com", time.Now().Add(60*24*time.Hour))
	renewed := generateTestResource(t, "example.com", time.Now().Add(90*24*time.Hour))

	client := &fakeClient{
		obtainFn: func() (*certificate.Resource, error) { return original, nil },
		renewFn:  func(_ certificate.Resource) (*certificate.Resource, error) { return renewed, nil },
	}

	svc := newServiceForTest(t, client)

	require.NoError(t, svc.Start(context.Background()))
	t.Cleanup(func() { _ = svc.Stop(context.Background()) })

	originalCert, err := svc.GetCertificate(nil)
	require.NoError(t, err)
	originalNotAfter := originalCert.Leaf.NotAfter

	require.NoError(t, svc.ForceRenew(context.Background()))
	assert.Equal(t, int32(1), client.renewCalls.Load())

	renewedCert, err := svc.GetCertificate(nil)
	require.NoError(t, err)
	assert.True(t, renewedCert.Leaf.NotAfter.After(originalNotAfter),
		"expected renewed cert to expire later than original")
}

func TestService_Start_LoadsCachedCertificateWhenFresh(t *testing.T) {
	fm := files.NewInMemoryFileManager()
	st := storage.NewFileStorage(fm, "acme")
	resource := generateTestResource(t, "example.com", time.Now().Add(60*24*time.Hour))

	require.NoError(t, st.SaveResource(context.Background(), "example.com", resource))
	require.NoError(t, st.SaveAccount(context.Background(), &acme.Account{
		Email:        "ops@example.com",
		PrivateKey:   testAccountKeyPEM(t),
		Registration: []byte(`{"uri":"https://test/reg"}`),
	}))

	client := &fakeClient{
		obtainFn: func() (*certificate.Resource, error) {
			t.Fatal("Obtain should not be called when storage has fresh certificate")

			return nil, nil
		},
	}

	svc := acme.NewService(serviceConfigFor("example.com"), st, locker.NewInMemoryLocker(), fakeRegistry{}, nil)
	svc.SetLegoFactory(func(_ registration.User, _ acme.ServiceConfig, _ acme.LegoFactoryDeps) (acme.LegoClient, error) {
		return client, nil
	})

	require.NoError(t, svc.Start(context.Background()))
	t.Cleanup(func() { _ = svc.Stop(context.Background()) })

	assert.Equal(t, int32(0), client.obtainCalls.Load())
}

func newServiceForTest(t *testing.T, client *fakeClient) *acme.Service {
	t.Helper()

	fm := files.NewInMemoryFileManager()
	st := storage.NewFileStorage(fm, "acme")
	svc := acme.NewService(serviceConfigFor("example.com"), st, locker.NewInMemoryLocker(), fakeRegistry{}, nil)

	if client != nil {
		svc.SetLegoFactory(func(_ registration.User, _ acme.ServiceConfig, _ acme.LegoFactoryDeps) (acme.LegoClient, error) {
			return client, nil
		})
	}

	return svc
}

func serviceConfigFor(domain string) acme.ServiceConfig { //nolint:unparam
	return acme.ServiceConfig{
		ChallengeType:        acme.ChallengeDNS01,
		Email:                "ops@example.com",
		Domains:              []string{domain},
		DirectoryURL:         "https://test.acme/dir",
		DNSProvider:          "fake",
		RenewalThreshold:     30 * 24 * time.Hour,
		RenewalCheckInterval: 1 * time.Hour,
		PropagationTimeout:   2 * time.Minute,
		LockTTL:              30 * time.Second,
	}
}

func generateTestResource(t *testing.T, cn string, notAfter time.Time) *certificate.Resource { //nolint:unparam
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     notAfter,
		DNSNames:     []string{cn},
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	return &certificate.Resource{
		Domain:            cn,
		CertURL:           "https://test/cert",
		PrivateKey:        keyPEM,
		Certificate:       certPEM,
		IssuerCertificate: certPEM,
	}
}

func testAccountKeyPEM(t *testing.T) []byte {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}
