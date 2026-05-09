package http01

import (
	"net/http"
	"strings"
	"sync"
)

// ChallengePathPrefix is the canonical URL prefix for HTTP-01 challenges.
const ChallengePathPrefix = "/.well-known/acme-challenge/"

// Provider holds challenge tokens in memory and serves them over HTTP.
//
// It implements lego's challenge.Provider (Present / CleanUp) and exposes an
// http.Handler that the gameap-api router mounts under
// /.well-known/acme-challenge/ before the SPA fallback.
//
// Multi-instance limitation: tokens are local to one process. With multiple
// gameap-api replicas behind a load balancer, the instance that received
// Present must also serve the GET; otherwise LE will fail validation. For
// multi-instance deployments use DNS-01 or sticky sessions on /.well-known/.
type Provider struct {
	mu     sync.RWMutex
	tokens map[string]string
}

func New() *Provider {
	return &Provider{tokens: make(map[string]string)}
}

func (p *Provider) Present(_, token, keyAuth string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.tokens[token] = keyAuth

	return nil
}

func (p *Provider) CleanUp(_, token, _ string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.tokens, token)

	return nil
}

// Handler serves GET /.well-known/acme-challenge/{token} requests.
func (p *Provider) Handler() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)

			return
		}

		token := strings.TrimPrefix(r.URL.Path, ChallengePathPrefix)
		if token == "" || strings.ContainsAny(token, "/?#") {
			http.NotFound(rw, r)

			return
		}

		p.mu.RLock()
		keyAuth, ok := p.tokens[token]
		p.mu.RUnlock()

		if !ok {
			http.NotFound(rw, r)

			return
		}

		rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
		rw.Header().Set("Cache-Control", "no-store")

		_, _ = rw.Write([]byte(keyAuth))
	})
}
