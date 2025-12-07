//go:build wasip1

package log

import (
	"context"
	"log/slog"
)

// Handler implements slog.Handler using the plugin's LogService.
type Handler struct {
	service LogService
	attrs   []slog.Attr
	groups  []string
}

// NewHandler creates a new slog.Handler that logs via the plugin's LogService.
func NewHandler() *Handler {
	return &Handler{
		service: NewLogService(),
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *Handler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

// Handle handles the Record.
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	level := levelToString(r.Level)

	fields := make(map[string]string)

	// Add pre-set attributes
	for _, attr := range h.attrs {
		addAttrToFields(fields, h.groups, attr)
	}

	// Add record attributes
	r.Attrs(func(attr slog.Attr) bool {
		addAttrToFields(fields, h.groups, attr)
		return true
	})

	_, err := h.service.Log(ctx, &LogRequest{
		Level:   level,
		Message: r.Message,
		Fields:  fields,
	})

	return err
}

// WithAttrs returns a new Handler with the given attributes added.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)

	return &Handler{
		service: h.service,
		attrs:   newAttrs,
		groups:  h.groups,
	}
}

// WithGroup returns a new Handler with the given group appended to the receiver's existing groups.
func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	newGroups := make([]string, len(h.groups)+1)
	copy(newGroups, h.groups)
	newGroups[len(h.groups)] = name

	return &Handler{
		service: h.service,
		attrs:   h.attrs,
		groups:  newGroups,
	}
}

// NewLogger creates a new slog.Logger that logs via the plugin's LogService.
func NewLogger() *slog.Logger {
	return slog.New(NewHandler())
}

func levelToString(level slog.Level) string {
	switch {
	case level < slog.LevelInfo:
		return "debug"
	case level < slog.LevelWarn:
		return "info"
	case level < slog.LevelError:
		return "warn"
	default:
		return "error"
	}
}

func addAttrToFields(fields map[string]string, groups []string, attr slog.Attr) {
	if attr.Key == "" {
		return
	}

	key := attr.Key
	for i := len(groups) - 1; i >= 0; i-- {
		key = groups[i] + "." + key
	}

	fields[key] = attr.Value.String()
}
