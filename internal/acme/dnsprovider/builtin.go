package dnsprovider

import (
	"context"
	"strings"

	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/providers/dns/cloudflare"
	"github.com/pkg/errors"
)

// BuiltinRegistry resolves a small whitelist of DNS providers compiled into
// gameap-api. Phase 1 ships with cloudflare only as a fallback before the
// plugin-backed registry replaces this. Adding new built-in providers means
// importing the corresponding lego sub-package here, which materially grows
// the binary — prefer adding via plugins instead.
type BuiltinRegistry struct{}

func NewBuiltinRegistry() *BuiltinRegistry {
	return &BuiltinRegistry{}
}

func (r *BuiltinRegistry) Resolve(_ context.Context, identifier string) (challenge.Provider, error) {
	if strings.Contains(identifier, ":") {
		return nil, errors.Errorf(
			"identifier %q has plugin prefix; use plugin-backed registry for plugin DNS providers",
			identifier,
		)
	}

	switch identifier {
	case "":
		return nil, errors.New("identifier is empty")
	case "cloudflare":
		provider, err := cloudflare.NewDNSProvider()
		if err != nil {
			return nil, errors.Wrap(err, "failed to construct cloudflare DNS provider")
		}

		return provider, nil
	default:
		return nil, errors.Errorf("built-in DNS provider %q is not registered", identifier)
	}
}
