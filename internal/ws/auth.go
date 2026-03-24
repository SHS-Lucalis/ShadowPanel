package ws

import (
	"context"
	"net/http"

	"github.com/gameap/gameap/pkg/api"
	"github.com/gameap/gameap/pkg/auth"
	"github.com/pkg/errors"
)

func SessionFromRequest(r *http.Request) (*auth.Session, error) {
	session := auth.SessionFromContext(r.Context())
	if !session.IsAuthenticated() {
		return nil, api.WrapHTTPError(
			errors.New("user not authenticated"),
			http.StatusUnauthorized,
		)
	}

	return session, nil
}

func SessionFromContext(ctx context.Context) (*auth.Session, error) {
	session := auth.SessionFromContext(ctx)
	if !session.IsAuthenticated() {
		return nil, errors.New("user not authenticated")
	}

	return session, nil
}
