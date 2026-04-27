package sqlite

import (
	"context"
	"database/sql"

	pkgstrings "github.com/gameap/gameap/pkg/strings"
)

// Up007 hashes any existing plaintext daemon API tokens with SHA-256 so that a
// database leak no longer exposes a usable daemon credential. Daemons that
// presented the plaintext token before the migration continue to authenticate:
// the middleware now hashes the presented X-Auth-Token before lookup.
//
// Rows whose stored value already looks like a SHA-256 hex digest (64 lowercase
// hex characters) are left untouched, making the migration safe to re-run.
func Up007(ctx context.Context, tx *sql.Tx) error {
	return hashDaemonAPITokens(ctx, tx)
}

// Down007 cannot reverse a one-way hash. To return to a working state after a
// rollback, daemons must re-enroll via /gdaemon_api/get_token. We null the
// affected column to make that requirement explicit.
func Down007(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE dedicated_servers SET gdaemon_api_token = NULL WHERE gdaemon_api_token IS NOT NULL`)

	return err
}

func hashDaemonAPITokens(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT id, gdaemon_api_token FROM dedicated_servers WHERE gdaemon_api_token IS NOT NULL AND gdaemon_api_token != ''`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	type pending struct {
		id    int64
		token string
	}

	var toUpdate []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.id, &p.token); err != nil {
			return err
		}

		if looksLikeSHA256Hex(p.token) {
			continue
		}

		p.token = pkgstrings.SHA256(p.token)
		toUpdate = append(toUpdate, p)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, p := range toUpdate {
		if _, err := tx.ExecContext(ctx,
			`UPDATE dedicated_servers SET gdaemon_api_token = ? WHERE id = ?`, p.token, p.id); err != nil {
			return err
		}
	}

	return nil
}

func looksLikeSHA256Hex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		default:
			return false
		}
	}

	return true
}
