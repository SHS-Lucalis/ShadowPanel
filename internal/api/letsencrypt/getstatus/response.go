package getstatus

import (
	"time"

	"github.com/gameap/gameap/internal/acme"
)

type response struct {
	Enabled            bool      `json:"enabled"`
	State              string    `json:"state"`
	ChallengeType      string    `json:"challenge_type"`
	Domains            []string  `json:"domains"`
	DNSProvider        string    `json:"dns_provider"`
	NotBefore          time.Time `json:"not_before,omitzero"`
	NotAfter           time.Time `json:"not_after,omitzero"`
	LastRenewalAt      time.Time `json:"last_renewal_at,omitzero"`
	NextRenewalCheckAt time.Time `json:"next_renewal_check_at,omitzero"`
	LastError          string    `json:"last_error,omitempty"`
}

func newResponse(status acme.Status) response {
	return response{
		Enabled:            status.Enabled,
		State:              string(status.State),
		ChallengeType:      status.ChallengeType,
		Domains:            status.Domains,
		DNSProvider:        status.DNSProvider,
		NotBefore:          status.NotBefore,
		NotAfter:           status.NotAfter,
		LastRenewalAt:      status.LastRenewalAt,
		NextRenewalCheckAt: status.NextRenewalCheckAt,
		LastError:          status.LastError,
	}
}

func disabledResponse() response {
	return response{
		Enabled: false,
		State:   string(acme.StateDisabled),
	}
}
