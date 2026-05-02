package upload

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"
)

// Janitor garbage-collects expired upload sessions from the shared transfer
// storage. It is intentionally a best-effort fallback for sessions that the
// happy-path Service.Complete cleanup did not reach (client abandoned, daemon
// dispatch failed mid-flight, instance crashed before fire-and-forget cleanup
// ran, etc.).
//
// Multi-instance safety: every API instance can run its own Janitor against
// the same storage. Storage.DeleteByPrefix is idempotent, so duplicate sweeps
// across instances are harmless beyond a few extra storage calls.
type Janitor struct {
	storage  Storage
	clock    Clock
	interval time.Duration
	logger   *slog.Logger
}

// NewJanitor returns a Janitor that scans the storage every interval. A nil
// logger or clock falls back to slog.Default and the real wall clock — both
// are accepted so production wiring stays terse and tests can inject fakes.
func NewJanitor(storage Storage, clock Clock, interval time.Duration, logger *slog.Logger) *Janitor {
	if logger == nil {
		logger = slog.Default()
	}
	if clock == nil {
		clock = realClock{}
	}

	return &Janitor{
		storage:  storage,
		clock:    clock,
		interval: interval,
		logger:   logger,
	}
}

// Run blocks until ctx is cancelled, performing one Sweep immediately and
// another every interval. A non-positive interval disables the loop entirely
// (useful for tests and for opting out via FILES_UPLOAD_JANITOR_INTERVAL=0).
func (j *Janitor) Run(ctx context.Context) {
	if j.interval <= 0 {
		return
	}

	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	j.Sweep(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			j.Sweep(ctx)
		}
	}
}

// Sweep walks every "transfers/{uploadID}/..." entry in storage once and
// removes the whole transfer prefix for sessions whose ExpiresAt has passed.
// All errors are logged and swallowed — a single broken session must not stop
// the rest of the sweep. Exported so tests and operators can trigger an
// immediate scan without waiting for the next tick.
func (j *Janitor) Sweep(ctx context.Context) {
	paths, err := j.storage.List(ctx, transferPrefix)
	if err != nil {
		j.logger.Warn("upload janitor list failed", "error", err)

		return
	}

	seen := make(map[string]struct{})
	for _, p := range paths {
		uploadID, ok := uploadIDFromPath(p)
		if !ok {
			continue
		}
		if _, dup := seen[uploadID]; dup {
			continue
		}
		seen[uploadID] = struct{}{}
		j.maybeRemove(ctx, uploadID)
	}
}

// maybeRemove deletes one upload-session prefix if its metadata says it has
// expired. A missing or unreadable upload.json is treated as "not yet
// observable" and skipped: the chunks may belong to a session that is still
// being created on a peer instance, and deleting them would race the
// in-progress writer. Truly orphaned data is reclaimed at the storage level
// by lifecycle policies, not here.
func (j *Janitor) maybeRemove(ctx context.Context, uploadID string) {
	raw, err := j.storage.Read(ctx, metadataPath(uploadID))
	if err != nil {
		return
	}
	var sess Session
	if unmarshalErr := json.Unmarshal(raw, &sess); unmarshalErr != nil {
		j.logger.Warn(
			"upload janitor failed to unmarshal session",
			"upload_id", uploadID,
			"error", unmarshalErr,
		)

		return
	}
	if sess.ExpiresAt.IsZero() || j.clock.Now().Before(sess.ExpiresAt) {
		return
	}
	if delErr := j.storage.DeleteByPrefix(ctx, transferRoot(uploadID)); delErr != nil {
		j.logger.Warn(
			"upload janitor failed to delete expired session",
			"upload_id", uploadID,
			"error", delErr,
		)

		return
	}
	j.logger.Info("upload janitor removed expired session", "upload_id", uploadID)
}

// uploadIDFromPath extracts the {uploadID} segment from any object path under
// "transfers/", e.g. "transfers/abc/chunks/000000" → "abc". Returns false for
// anything that does not match the layout so the sweep can skip it cleanly.
// uploadIDFromPath extracts the upload ID from a Storage.List entry produced
// by listing transferPrefix. Accepts all three storage formats:
//   - LocalFileManager: bare directory name ("<id>")
//   - S3FileManager:    relative key, optionally with trailing slash ("<id>/")
//   - InMemoryFileManager: full recursive path ("transfers/<id>/upload.json")
func uploadIDFromPath(path string) (string, bool) {
	if rest, hasPrefix := strings.CutPrefix(path, transferPrefix); hasPrefix {
		slash := strings.Index(rest, "/")
		if slash <= 0 {
			return "", false
		}

		return rest[:slash], true
	}

	rest := strings.TrimSuffix(path, "/")
	if rest == "" || strings.Contains(rest, "/") {
		return "", false
	}

	return rest, true
}
