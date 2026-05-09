package storage

import (
	"context"
	"encoding/json"
	"path"
	"strings"

	"github.com/gameap/gameap/internal/acme"
	"github.com/gameap/gameap/internal/files"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/pkg/errors"
)

type FileStorage struct {
	fm       files.FileManager
	basePath string
}

func NewFileStorage(fm files.FileManager, basePath string) *FileStorage {
	return &FileStorage{
		fm:       fm,
		basePath: strings.TrimSuffix(basePath, "/"),
	}
}

func (s *FileStorage) accountPath(email string) string {
	return path.Join(s.basePath, "accounts", sanitizeEmail(email)+".json")
}

func (s *FileStorage) resourcePath(key string) string {
	return path.Join(s.basePath, "certificates", sanitizeKey(key)+".json")
}

func (s *FileStorage) SaveAccount(ctx context.Context, account *acme.Account) error {
	if account == nil || account.Email == "" {
		return errors.New("account or email is empty")
	}

	// gosec G117: account.PrivateKey is a PEM-encoded ACME account private key
	// that legitimately must be persisted to storage so the same account can
	// re-authenticate against Let's Encrypt across restarts.
	data, err := json.Marshal(account) //nolint:gosec
	if err != nil {
		return errors.Wrap(err, "failed to marshal account")
	}

	if err := s.fm.Write(ctx, s.accountPath(account.Email), data); err != nil {
		return errors.WithMessage(err, "failed to write account")
	}

	return nil
}

func (s *FileStorage) LoadAccount(ctx context.Context, email string) (*acme.Account, error) {
	if email == "" {
		return nil, errors.New("email is empty")
	}

	data, err := s.fm.Read(ctx, s.accountPath(email))
	if err != nil {
		return nil, errors.WithMessage(err, "failed to read account")
	}

	var account acme.Account
	if err := json.Unmarshal(data, &account); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal account")
	}

	return &account, nil
}

func (s *FileStorage) HasAccount(ctx context.Context, email string) (bool, error) {
	if email == "" {
		return false, nil
	}

	return s.fm.Exists(ctx, s.accountPath(email)), nil
}

type storedResource struct {
	Domain            string `json:"domain"`
	CertURL           string `json:"cert_url"`
	CertStableURL     string `json:"cert_stable_url"`
	PrivateKey        []byte `json:"private_key"`
	Certificate       []byte `json:"certificate"`
	IssuerCertificate []byte `json:"issuer_certificate"`
	CSR               []byte `json:"csr,omitempty"`
}

func (s *FileStorage) SaveResource(ctx context.Context, key string, resource *certificate.Resource) error {
	if resource == nil {
		return errors.New("resource is nil")
	}

	stored := storedResource{
		Domain:            resource.Domain,
		CertURL:           resource.CertURL,
		CertStableURL:     resource.CertStableURL,
		PrivateKey:        resource.PrivateKey,
		Certificate:       resource.Certificate,
		IssuerCertificate: resource.IssuerCertificate,
		CSR:               resource.CSR,
	}

	// gosec G117: resource.PrivateKey is the certificate's PEM-encoded private
	// key — it has to land in shared storage so any panel instance can serve
	// HTTPS without re-running ACME on each restart.
	data, err := json.Marshal(stored) //nolint:gosec
	if err != nil {
		return errors.Wrap(err, "failed to marshal resource")
	}

	if err := s.fm.Write(ctx, s.resourcePath(key), data); err != nil {
		return errors.WithMessage(err, "failed to write resource")
	}

	return nil
}

func (s *FileStorage) LoadResource(ctx context.Context, key string) (*certificate.Resource, error) {
	data, err := s.fm.Read(ctx, s.resourcePath(key))
	if err != nil {
		return nil, errors.WithMessage(err, "failed to read resource")
	}

	var stored storedResource
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal resource")
	}

	return &certificate.Resource{
		Domain:            stored.Domain,
		CertURL:           stored.CertURL,
		CertStableURL:     stored.CertStableURL,
		PrivateKey:        stored.PrivateKey,
		Certificate:       stored.Certificate,
		IssuerCertificate: stored.IssuerCertificate,
		CSR:               stored.CSR,
	}, nil
}

func (s *FileStorage) HasResource(ctx context.Context, key string) (bool, error) {
	return s.fm.Exists(ctx, s.resourcePath(key)), nil
}

func (s *FileStorage) DeleteResource(ctx context.Context, key string) error {
	if err := s.fm.Delete(ctx, s.resourcePath(key)); err != nil {
		return errors.WithMessage(err, "failed to delete resource")
	}

	return nil
}

func sanitizeEmail(email string) string {
	r := strings.NewReplacer("@", "_at_", "/", "_", "\\", "_", "..", "_")

	return r.Replace(email)
}

func sanitizeKey(key string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", "..", "_", " ", "_", "*", "_wildcard_")

	return r.Replace(key)
}
