package auth

import "time"

type Claims interface {
	GetSubject() (string, error)
	// GetExpirationTime returns the token's `exp` claim if present, or nil
	// when the token has no expiration (e.g. some Personal Access Token
	// flows). Implementations may return an error if claims are unparseable.
	GetExpirationTime() (*time.Time, error)
}
