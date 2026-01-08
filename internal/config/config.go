package config

import (
	"crypto/tls"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/gameap/gameap/internal/application/defaults"
	"github.com/gameap/gameap/internal/certificates"
	"github.com/pkg/errors"
)

type Config struct {
	HTTPHost  string `env:"HTTP_HOST" envDefault:"0.0.0.0"`
	HTTPPort  uint16 `env:"HTTP_PORT" envDefault:"8025"`
	HTTPSPort uint16 `env:"HTTPS_PORT" envDefault:"443"`

	TLS struct {
		CertFile   string `env:"TLS_CERT_FILE" envDefault:""`
		KeyFile    string `env:"TLS_KEY_FILE" envDefault:""`
		Cert       string `env:"TLS_CERT" envDefault:""`
		Key        string `env:"TLS_KEY" envDefault:""`
		ForceHTTPS bool   `env:"TLS_FORCE_HTTPS" envDefault:"false"`
	}

	DatabaseDriver string `env:"DATABASE_DRIVER,required" envDefault:"mysql"`
	DatabaseURL    string `env:"DATABASE_URL,required,notEmpty"`

	EncryptionKey string `env:"ENCRYPTION_KEY" envDefault:""`
	AuthSecret    string `env:"AUTH_SECRET,required,notEmpty" envDefault:""`
	AuthService   string `env:"AUTH_SERVICE" envDefault:"paseto"`

	RBAC struct {
		CacheTTL string `env:"RBAC_CACHE_TTL" envDefault:"30s"`
	}

	Cache struct {
		Driver string `env:"CACHE_DRIVER" envDefault:"memory"`

		Redis struct {
			Addr     string `env:"CACHE_REDIS_ADDR" envDefault:"localhost:6379"`
			Password string `env:"CACHE_REDIS_PASSWORD" envDefault:""`
			DB       int    `env:"CACHE_REDIS_DB" envDefault:"0"`
		}

		// TTL configurations for different cache types
		TTL struct {
			RBAC           string `env:"CACHE_TTL_RBAC" envDefault:"24h"`
			Games          string `env:"CACHE_TTL_GAMES" envDefault:"48h"`
			Nodes          string `env:"CACHE_TTL_NODES" envDefault:"24h"`
			Users          string `env:"CACHE_TTL_USERS" envDefault:"6h"`
			PersonalTokens string `env:"CACHE_TTL_PERSONAL_TOKENS" envDefault:"24h"`
			ServerSettings string `env:"CACHE_TTL_SERVER_SETTINGS" envDefault:"12h"`
		}
	}

	Files struct {
		Driver string `env:"FILES_DRIVER" envDefault:"local"`

		Local struct {
			BasePath string `env:"FILES_LOCAL_BASE_PATH" envDefault:""`
		}

		S3 struct {
			Endpoint        string `env:"FILES_S3_ENDPOINT" envDefault:""`
			UseSSL          bool   `env:"FILES_S3_USE_SSL" envDefault:"true"`
			AccessKeyID     string `env:"FILES_S3_ACCESS_KEY_ID" envDefault:""`
			SecretAccessKey string `env:"FILES_S3_SECRET_ACCESS_KEY" envDefault:""`
			Bucket          string `env:"FILES_S3_BUCKET" envDefault:""`
		}
	}

	Logger struct {
		Level        string `env:"LOGGER_LEVEL" envDefault:"info"`
		LogDBQueries bool   `env:"LOGGER_LOG_DB_QUERIES" envDefault:"false"`
	}

	Legacy struct {
		Path    string `env:"LEGACY_PATH" envDefault:""`
		EnvPath string `env:"LEGACY_ENV_PATH" envDefault:""`
	}

	GlobalAPI struct {
		URL string `env:"GLOBAL_API_URL" envDefault:"https://api.gameap.com"`
	}

	UI struct {
		DefaultLanguage string `env:"DEFAULT_LANGUAGE" envDefault:""`
	}

	Plugins struct {
		Disabled bool     `env:"PLUGINS_DISABLED" envDefault:"false"`
		AutoLoad []string `env:"PLUGINS_AUTOLOAD" envDefault:"" envSeparator:","`
	}

	PluginStore struct {
		URL        string `env:"PLUGIN_STORE_URL" envDefault:"https://plugins.gameap.dev/api"`
		LicenseKey string `env:"PLUGIN_STORE_LICENSE_KEY" envDefault:""`
	}
}

func LoadConfig() (*Config, error) {
	var cfg Config
	var err error

	if cfg, err = env.ParseAs[Config](); err != nil {
		return nil, errors.WithMessage(err, "failed to parse config")
	}

	setDefaultConfigValues(&cfg)

	normalizeConfigValues(&cfg)

	return &cfg, nil
}

func setDefaultConfigValues(cfg *Config) {
	if cfg.Legacy.Path == "" {
		cfg.Legacy.Path = defaults.LegacyPath
	}

	if cfg.Legacy.EnvPath == "" {
		cfg.Legacy.EnvPath = defaults.LegacyEnvPath
	}
}

func normalizeConfigValues(cfg *Config) {
	cfg.DatabaseDriver = strings.ToLower(cfg.DatabaseDriver)

	switch cfg.DatabaseDriver {
	case "postgres", "postgresql", "pgx", "pg", "pgsql": //nolint:goconst
		cfg.DatabaseDriver = "pgx"
	}

	cfg.Cache.Driver = strings.ToLower(cfg.Cache.Driver)
	switch cfg.Cache.Driver {
	case "postgres", "postgresql", "pgx", "pg", "pgsql": //nolint:goconst,nolintlint
		cfg.Cache.Driver = "postgres"
	}

	cfg.UI.DefaultLanguage = strings.ToLower(cfg.UI.DefaultLanguage)
}

func (c *Config) TLSEnabled() bool {
	return (c.TLS.CertFile != "" && c.TLS.KeyFile != "") ||
		(c.TLS.Cert != "" && c.TLS.Key != "")
}

func (c *Config) LoadTLSCertificate() (*tls.Certificate, error) {
	if c.TLS.CertFile != "" && c.TLS.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(c.TLS.CertFile, c.TLS.KeyFile)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load TLS certificate from files")
		}

		return &cert, nil
	}

	certPEM := certificates.DecodePossibleBase64(c.TLS.Cert)
	keyPEM := certificates.DecodePossibleBase64(c.TLS.Key)

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load TLS certificate from content")
	}

	return &cert, nil
}
