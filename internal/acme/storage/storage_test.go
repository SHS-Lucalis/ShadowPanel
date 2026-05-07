package storage_test

import (
	"context"
	"testing"

	"github.com/gameap/gameap/internal/acme"
	"github.com/gameap/gameap/internal/acme/storage"
	"github.com/gameap/gameap/internal/files"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileStorage_Account(t *testing.T) {
	tests := []struct {
		name      string
		account   *acme.Account
		wantError string
	}{
		{
			name: "regular_email",
			account: &acme.Account{
				Email:        "ops@example.com",
				PrivateKey:   []byte("PEM-DATA"),
				Registration: []byte(`{"uri":"https://acme/reg/1"}`),
			},
		},
		{
			name: "email_with_special_chars",
			account: &acme.Account{
				Email:        "ops+letsencrypt@example.com",
				PrivateKey:   []byte("KEY"),
				Registration: []byte(`{}`),
			},
		},
		{
			name:      "empty_email_rejected",
			account:   &acme.Account{Email: ""},
			wantError: "email is empty",
		},
		{
			name:      "nil_account_rejected",
			account:   nil,
			wantError: "account or email is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := files.NewInMemoryFileManager()
			s := storage.NewFileStorage(fm, "acme")

			err := s.SaveAccount(context.Background(), tt.account)
			if tt.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)

				return
			}

			require.NoError(t, err)

			has, err := s.HasAccount(context.Background(), tt.account.Email)
			require.NoError(t, err)
			assert.True(t, has)

			loaded, err := s.LoadAccount(context.Background(), tt.account.Email)
			require.NoError(t, err)
			assert.Equal(t, tt.account.Email, loaded.Email)
			assert.Equal(t, tt.account.PrivateKey, loaded.PrivateKey)
			assert.Equal(t, []byte(tt.account.Registration), []byte(loaded.Registration))
		})
	}
}

func TestFileStorage_Resource(t *testing.T) {
	fm := files.NewInMemoryFileManager()
	s := storage.NewFileStorage(fm, "acme")
	ctx := context.Background()

	resource := &certificate.Resource{
		Domain:            "*.example.com",
		CertURL:           "https://acme/cert/1",
		CertStableURL:     "https://acme/cert/stable/1",
		PrivateKey:        []byte("PRIVATE-KEY-PEM"),
		Certificate:       []byte("CERT-PEM"),
		IssuerCertificate: []byte("ISSUER-PEM"),
	}

	t.Run("save_load_round_trip", func(t *testing.T) {
		err := s.SaveResource(ctx, "*.example.com,example.com", resource)
		require.NoError(t, err)

		has, err := s.HasResource(ctx, "*.example.com,example.com")
		require.NoError(t, err)
		assert.True(t, has)

		loaded, err := s.LoadResource(ctx, "*.example.com,example.com")
		require.NoError(t, err)
		assert.Equal(t, resource.Domain, loaded.Domain)
		assert.Equal(t, resource.CertURL, loaded.CertURL)
		assert.Equal(t, resource.PrivateKey, loaded.PrivateKey)
		assert.Equal(t, resource.Certificate, loaded.Certificate)
		assert.Equal(t, resource.IssuerCertificate, loaded.IssuerCertificate)
	})

	t.Run("delete_removes_resource", func(t *testing.T) {
		err := s.SaveResource(ctx, "to-delete", resource)
		require.NoError(t, err)

		err = s.DeleteResource(ctx, "to-delete")
		require.NoError(t, err)

		has, err := s.HasResource(ctx, "to-delete")
		require.NoError(t, err)
		assert.False(t, has)
	})

	t.Run("missing_resource_returns_error", func(t *testing.T) {
		_, err := s.LoadResource(ctx, "non-existent")
		assert.Error(t, err)
	})

	t.Run("nil_resource_rejected", func(t *testing.T) {
		err := s.SaveResource(ctx, "key", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "resource is nil")
	})
}

func TestFileStorage_KeySanitization(t *testing.T) {
	fm := files.NewInMemoryFileManager()
	s := storage.NewFileStorage(fm, "acme")

	resource := &certificate.Resource{
		Domain:      "*.example.com",
		Certificate: []byte("CERT"),
		PrivateKey:  []byte("KEY"),
	}

	keyWithWildcard := "*.example.com,example.com"
	err := s.SaveResource(context.Background(), keyWithWildcard, resource)
	require.NoError(t, err)

	loaded, err := s.LoadResource(context.Background(), keyWithWildcard)
	require.NoError(t, err)
	assert.Equal(t, resource.Domain, loaded.Domain)
}
