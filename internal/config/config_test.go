package config

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gameap/gameap/internal/application/defaults"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	t.Run("with_required_env_vars", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "mysql://localhost/test")
		t.Setenv("AUTH_SECRET", "test-secret")

		cfg, err := LoadConfig()
		require.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Equal(t, "mysql://localhost/test", cfg.DatabaseURL)
		assert.Equal(t, "test-secret", cfg.AuthSecret)
	})

	t.Run("without_database_url", func(t *testing.T) {
		t.Setenv("AUTH_SECRET", "test-secret")

		_, err := LoadConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "DATABASE_URL")
	})

	t.Run("without_auth_secret", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "mysql://localhost/test")

		_, err := LoadConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "AUTH_SECRET")
	})

	t.Run("default_values", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "mysql://localhost/test")
		t.Setenv("AUTH_SECRET", "test-secret")

		cfg, err := LoadConfig()
		require.NoError(t, err)

		assert.Equal(t, "0.0.0.0", cfg.HTTPHost)
		assert.Equal(t, uint16(8025), cfg.HTTPPort)
		assert.Equal(t, uint16(443), cfg.HTTPSPort)
		assert.Equal(t, "mysql", cfg.DatabaseDriver)
		assert.Equal(t, "paseto", cfg.AuthService)
		assert.Equal(t, "30s", cfg.RBAC.CacheTTL)
		assert.Equal(t, "memory", cfg.Cache.Driver)
		assert.Equal(t, "local", cfg.Files.Driver)
		assert.Equal(t, "info", cfg.Logger.Level)
		assert.False(t, cfg.Logger.LogDBQueries)
		assert.Equal(t, "https://api.gameap.com", cfg.GlobalAPI.URL)
	})
}

func TestNormalizeConfigValues(t *testing.T) {
	tests := []struct {
		name                   string
		databaseDriver         string
		cacheDriver            string
		expectedDatabaseDriver string
		expectedCacheDriver    string
	}{
		{
			name:                   "postgres_normalized",
			databaseDriver:         "postgres",
			cacheDriver:            "memory",
			expectedDatabaseDriver: "pgx",
			expectedCacheDriver:    "memory",
		},
		{
			name:                   "postgresql_normalized",
			databaseDriver:         "postgresql",
			cacheDriver:            "memory",
			expectedDatabaseDriver: "pgx",
			expectedCacheDriver:    "memory",
		},
		{
			name:                   "pgx_unchanged",
			databaseDriver:         "pgx",
			cacheDriver:            "memory",
			expectedDatabaseDriver: "pgx",
			expectedCacheDriver:    "memory",
		},
		{
			name:                   "pg_normalized",
			databaseDriver:         "pg",
			cacheDriver:            "memory",
			expectedDatabaseDriver: "pgx",
			expectedCacheDriver:    "memory",
		},
		{
			name:                   "pgsql_normalized",
			databaseDriver:         "pgsql",
			cacheDriver:            "memory",
			expectedDatabaseDriver: "pgx",
			expectedCacheDriver:    "memory",
		},
		{
			name:                   "mysql_unchanged",
			databaseDriver:         "mysql",
			cacheDriver:            "memory",
			expectedDatabaseDriver: "mysql",
			expectedCacheDriver:    "memory",
		},
		{
			name:                   "uppercase_postgres_normalized",
			databaseDriver:         "POSTGRES",
			cacheDriver:            "memory",
			expectedDatabaseDriver: "pgx",
			expectedCacheDriver:    "memory",
		},
		{
			name:                   "cache_postgres_normalized",
			databaseDriver:         "mysql",
			cacheDriver:            "postgres",
			expectedDatabaseDriver: "mysql",
			expectedCacheDriver:    "postgres",
		},
		{
			name:                   "cache_postgresql_normalized",
			databaseDriver:         "mysql",
			cacheDriver:            "postgresql",
			expectedDatabaseDriver: "mysql",
			expectedCacheDriver:    "postgres",
		},
		{
			name:                   "cache_pgx_normalized",
			databaseDriver:         "mysql",
			cacheDriver:            "pgx",
			expectedDatabaseDriver: "mysql",
			expectedCacheDriver:    "postgres",
		},
		{
			name:                   "cache_pg_normalized",
			databaseDriver:         "mysql",
			cacheDriver:            "pg",
			expectedDatabaseDriver: "mysql",
			expectedCacheDriver:    "postgres",
		},
		{
			name:                   "cache_pgsql_normalized",
			databaseDriver:         "mysql",
			cacheDriver:            "pgsql",
			expectedDatabaseDriver: "mysql",
			expectedCacheDriver:    "postgres",
		},
		{
			name:                   "cache_redis_unchanged",
			databaseDriver:         "mysql",
			cacheDriver:            "redis",
			expectedDatabaseDriver: "mysql",
			expectedCacheDriver:    "redis",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := &Config{
				DatabaseDriver: test.databaseDriver,
			}
			cfg.Cache.Driver = test.cacheDriver

			normalizeConfigValues(cfg)

			assert.Equal(t, test.expectedDatabaseDriver, cfg.DatabaseDriver)
			assert.Equal(t, test.expectedCacheDriver, cfg.Cache.Driver)
		})
	}
}

func TestSetDefaultConfigValues(t *testing.T) {
	t.Run("empty_legacy_paths_set_to_defaults", func(t *testing.T) {
		cfg := &Config{}

		setDefaultConfigValues(cfg)

		assert.Equal(t, defaults.LegacyPath, cfg.Legacy.Path)
		assert.Equal(t, defaults.LegacyEnvPath, cfg.Legacy.EnvPath)
	})

	t.Run("non_empty_legacy_paths_preserved", func(t *testing.T) {
		cfg := &Config{}
		cfg.Legacy.Path = "/custom/path"
		cfg.Legacy.EnvPath = "/custom/env/path"

		setDefaultConfigValues(cfg)

		assert.Equal(t, "/custom/path", cfg.Legacy.Path)
		assert.Equal(t, "/custom/env/path", cfg.Legacy.EnvPath)
	})
}

func TestConfig_TLSEnabled(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected bool
	}{
		{
			name:     "no_tls_configured",
			config:   Config{},
			expected: false,
		},
		{
			name: "cert_and_key_files_set",
			config: Config{
				TLS: struct {
					CertFile   string `env:"TLS_CERT_FILE" envDefault:""`
					KeyFile    string `env:"TLS_KEY_FILE" envDefault:""`
					Cert       string `env:"TLS_CERT" envDefault:""`
					Key        string `env:"TLS_KEY" envDefault:""`
					ForceHTTPS bool   `env:"TLS_FORCE_HTTPS" envDefault:"false"`
				}{
					CertFile: "/path/to/cert.pem",
					KeyFile:  "/path/to/key.pem",
				},
			},
			expected: true,
		},
		{
			name: "cert_and_key_content_set",
			config: Config{
				TLS: struct {
					CertFile   string `env:"TLS_CERT_FILE" envDefault:""`
					KeyFile    string `env:"TLS_KEY_FILE" envDefault:""`
					Cert       string `env:"TLS_CERT" envDefault:""`
					Key        string `env:"TLS_KEY" envDefault:""`
					ForceHTTPS bool   `env:"TLS_FORCE_HTTPS" envDefault:"false"`
				}{
					Cert: "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
					Key:  "-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----",
				},
			},
			expected: true,
		},
		{
			name: "only_cert_file_set",
			config: Config{
				TLS: struct {
					CertFile   string `env:"TLS_CERT_FILE" envDefault:""`
					KeyFile    string `env:"TLS_KEY_FILE" envDefault:""`
					Cert       string `env:"TLS_CERT" envDefault:""`
					Key        string `env:"TLS_KEY" envDefault:""`
					ForceHTTPS bool   `env:"TLS_FORCE_HTTPS" envDefault:"false"`
				}{
					CertFile: "/path/to/cert.pem",
				},
			},
			expected: false,
		},
		{
			name: "only_key_file_set",
			config: Config{
				TLS: struct {
					CertFile   string `env:"TLS_CERT_FILE" envDefault:""`
					KeyFile    string `env:"TLS_KEY_FILE" envDefault:""`
					Cert       string `env:"TLS_CERT" envDefault:""`
					Key        string `env:"TLS_KEY" envDefault:""`
					ForceHTTPS bool   `env:"TLS_FORCE_HTTPS" envDefault:"false"`
				}{
					KeyFile: "/path/to/key.pem",
				},
			},
			expected: false,
		},
		{
			name: "only_cert_content_set",
			config: Config{
				TLS: struct {
					CertFile   string `env:"TLS_CERT_FILE" envDefault:""`
					KeyFile    string `env:"TLS_KEY_FILE" envDefault:""`
					Cert       string `env:"TLS_CERT" envDefault:""`
					Key        string `env:"TLS_KEY" envDefault:""`
					ForceHTTPS bool   `env:"TLS_FORCE_HTTPS" envDefault:"false"`
				}{
					Cert: "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
				},
			},
			expected: false,
		},
		{
			name: "only_key_content_set",
			config: Config{
				TLS: struct {
					CertFile   string `env:"TLS_CERT_FILE" envDefault:""`
					KeyFile    string `env:"TLS_KEY_FILE" envDefault:""`
					Cert       string `env:"TLS_CERT" envDefault:""`
					Key        string `env:"TLS_KEY" envDefault:""`
					ForceHTTPS bool   `env:"TLS_FORCE_HTTPS" envDefault:"false"`
				}{
					Key: "-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----",
				},
			},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := test.config.TLSEnabled()
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestConfig_LoadTLSCertificate(t *testing.T) {
	certPEM, keyPEM := generateTestCertificate(t)

	t.Run("from_files", func(t *testing.T) {
		tempDir := t.TempDir()
		certFile := filepath.Join(tempDir, "cert.pem")
		keyFile := filepath.Join(tempDir, "key.pem")

		err := os.WriteFile(certFile, certPEM, 0o600)
		require.NoError(t, err)
		err = os.WriteFile(keyFile, keyPEM, 0o600)
		require.NoError(t, err)

		cfg := &Config{}
		cfg.TLS.CertFile = certFile
		cfg.TLS.KeyFile = keyFile

		cert, err := cfg.LoadTLSCertificate()
		require.NoError(t, err)
		assert.NotNil(t, cert)
	})

	t.Run("from_content", func(t *testing.T) {
		cfg := &Config{}
		cfg.TLS.Cert = string(certPEM)
		cfg.TLS.Key = string(keyPEM)

		cert, err := cfg.LoadTLSCertificate()
		require.NoError(t, err)
		assert.NotNil(t, cert)
	})

	t.Run("from_base64_content", func(t *testing.T) {
		cfg := &Config{}
		cfg.TLS.Cert = base64.StdEncoding.EncodeToString(certPEM)
		cfg.TLS.Key = base64.StdEncoding.EncodeToString(keyPEM)

		cert, err := cfg.LoadTLSCertificate()
		require.NoError(t, err)
		assert.NotNil(t, cert)
	})

	t.Run("invalid_file_paths", func(t *testing.T) {
		cfg := &Config{}
		cfg.TLS.CertFile = "/nonexistent/cert.pem"
		cfg.TLS.KeyFile = "/nonexistent/key.pem"

		cert, err := cfg.LoadTLSCertificate()
		require.Error(t, err)
		assert.Nil(t, cert)
		assert.Contains(t, err.Error(), "failed to load TLS certificate from files")
	})

	t.Run("invalid_content", func(t *testing.T) {
		cfg := &Config{}
		cfg.TLS.Cert = "invalid-cert-content"
		cfg.TLS.Key = "invalid-key-content"

		cert, err := cfg.LoadTLSCertificate()
		require.Error(t, err)
		assert.Nil(t, cert)
		assert.Contains(t, err.Error(), "failed to load TLS certificate from content")
	})

	t.Run("files_take_priority_over_content", func(t *testing.T) {
		tempDir := t.TempDir()
		certFile := filepath.Join(tempDir, "cert.pem")
		keyFile := filepath.Join(tempDir, "key.pem")

		err := os.WriteFile(certFile, certPEM, 0o600)
		require.NoError(t, err)
		err = os.WriteFile(keyFile, keyPEM, 0o600)
		require.NoError(t, err)

		cfg := &Config{}
		cfg.TLS.CertFile = certFile
		cfg.TLS.KeyFile = keyFile
		cfg.TLS.Cert = "invalid-cert-content"
		cfg.TLS.Key = "invalid-key-content"

		cert, err := cfg.LoadTLSCertificate()
		require.NoError(t, err)
		assert.NotNil(t, cert)
	})
}

func TestNormalizeConfigValues_DefaultLanguage(t *testing.T) {
	tests := []struct {
		name             string
		defaultLanguage  string
		expectedLanguage string
	}{
		{
			name:             "empty_unchanged",
			defaultLanguage:  "",
			expectedLanguage: "",
		},
		{
			name:             "en_unchanged",
			defaultLanguage:  "en",
			expectedLanguage: "en",
		},
		{
			name:             "ru_unchanged",
			defaultLanguage:  "ru",
			expectedLanguage: "ru",
		},
		{
			name:             "uppercase_EN_normalized_to_lowercase",
			defaultLanguage:  "EN",
			expectedLanguage: "en",
		},
		{
			name:             "uppercase_RU_normalized_to_lowercase",
			defaultLanguage:  "RU",
			expectedLanguage: "ru",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			cfg.UI.DefaultLanguage = tt.defaultLanguage

			normalizeConfigValues(cfg)

			assert.Equal(t, tt.expectedLanguage, cfg.UI.DefaultLanguage)
		})
	}
}

func generateTestCertificate(t *testing.T) (certPEM []byte, keyPEM []byte) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	certPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	keyPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	return certPEM, keyPEM
}
