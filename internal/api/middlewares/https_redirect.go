package middlewares

import (
	"net/http"
	"strconv"
	"strings"
)

// acmeChallengePathPrefix is the well-known path that must remain reachable
// over plain HTTP for ACME HTTP-01 validation. Mirrored from
// internal/acme/http01 to avoid a cyclic import.
const acmeChallengePathPrefix = "/.well-known/acme-challenge/"

func HTTPSRedirectMiddleware(httpsPort uint16) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.TLS == nil && !strings.HasPrefix(r.URL.Path, acmeChallengePathPrefix) {
				host := r.Host
				if idx := strings.LastIndex(host, ":"); idx != -1 {
					host = host[:idx]
				}

				target := "https://" + host
				if httpsPort != 443 {
					target += ":" + strconv.Itoa(int(httpsPort))
				}
				target += r.URL.RequestURI()

				http.Redirect(w, r, target, http.StatusMovedPermanently)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
