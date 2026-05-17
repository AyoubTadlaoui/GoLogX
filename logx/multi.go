package logx

import (
	"context"
	"errors"
	"log/slog"
)

// MultiHandler fans every record out to all of its inner handlers.
//
// Typical use: a pretty handler on stderr plus a JSON handler on a rotating
// file, both seeing the same records.
type MultiHandler struct {
	handlers []slog.Handler
}

// NewMultiHandler returns a MultiHandler that forwards to every h.
//
// It does not own the inner handlers — close them yourself if needed.
func NewMultiHandler(handlers ...slog.Handler) *MultiHandler {
	hs := make([]slog.Handler, 0, len(handlers))
	for _, h := range handlers {
		if h != nil {
			hs = append(hs, h)
		}
	}
	return &MultiHandler{handlers: hs}
}

// Enabled returns true if any inner handler is enabled at level.
// This keeps a single-INFO sink from suppressing DEBUG that another sink wants.
func (m *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// Handle forwards to every enabled handler and joins any errors.
func (m *MultiHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, h := range m.handlers {
		if !h.Enabled(ctx, r.Level) {
			continue
		}
		if err := h.Handle(ctx, r.Clone()); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// WithAttrs returns a MultiHandler whose inner handlers each have attrs attached.
func (m *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clones := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		clones[i] = h.WithAttrs(attrs)
	}
	return &MultiHandler{handlers: clones}
}

// WithGroup returns a MultiHandler whose inner handlers each open group name.
func (m *MultiHandler) WithGroup(name string) slog.Handler {
	clones := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		clones[i] = h.WithGroup(name)
	}
	return &MultiHandler{handlers: clones}
}
