package middlewares

import (
	"fmt"
	"net/http"

	"github.com/gameap/gameap/internal/config"
	"github.com/rs/cors"
)

type CORSMiddleware struct {
	cors *cors.Cors
}

func NewCORSMiddleware(cfg *config.Config) *CORSMiddleware {
	allowedOrigins := cfg.HTTPAllowedOrigins
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{deriveDefaultOrigin(cfg)}
	}

	return &CORSMiddleware{
		cors: cors.New(cors.Options{
			AllowedOrigins:   allowedOrigins,
			AllowCredentials: true,
		}),
	}
}

// deriveDefaultOrigin builds a single allowed origin from the deployment's HTTP
// configuration. The scheme tracks TLS.ForceHTTPS so HTTPS deployments do not
// inadvertently advertise an http:// origin (an issue under L-1 of the
// 2026-04 security audit).
func deriveDefaultOrigin(cfg *config.Config) string {
	scheme := "http"
	port := cfg.HTTPPort
	defaultPort := uint16(80)

	if cfg.TLS.ForceHTTPS {
		scheme = "https"
		port = cfg.HTTPSPort
		defaultPort = 443
	}

	origin := fmt.Sprintf("%s://%s", scheme, cfg.HTTPHost)
	if port != defaultPort {
		origin = fmt.Sprintf("%s:%d", origin, port)
	}

	return origin
}

func (m *CORSMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.cors.Handler(next).ServeHTTP(w, r)
	})
}
