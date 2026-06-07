package audit

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Options configures NewHandler. The zero value produces an unsigned chain that
// records every level and starts from genesis.
type Options struct {
	// Signer, if set, Ed25519-signs every entry. Without it the chain is
	// tamper-evident but a holder of the whole file can rewrite it end to end;
	// with it, entries cannot be re-forged without the private key.
	Signer ed25519.PrivateKey
	// Level is the minimum level recorded. nil records everything, which is the
	// right default for an audit trail.
	Level slog.Leveler
	// StartSeq is the seq assigned to the next entry. Use it with PrevHash to
	// resume an existing chain. Zero starts a new chain.
	StartSeq uint64
	// PrevHash is the hex hash of the last existing entry when resuming. Empty
	// means genesis.
	PrevHash string
}

// core is the shared, mutable chain state. Every Handler derived via WithAttrs
// or WithGroup points at the same core, so all of them append to one chain.
type core struct {
	mu     sync.Mutex
	seq    uint64
	prev   [hashLen]byte
	signer ed25519.PrivateKey
	w      io.Writer
}

// Handler is an slog.Handler that writes an append-only, hash-chained,
// optionally Ed25519-signed JSONL audit log. Read it back with Verify.
type Handler struct {
	core   *core
	attrs  []kv     // accumulated via WithAttrs, already group-qualified
	groups []string // currently open groups, applied to record attrs
	level  slog.Leveler
}

// kv is a flattened, JSON-ready attribute. Groups are folded into dotted keys.
type kv struct {
	key string
	val any
}

// NewHandler returns a Handler writing the chain to w. It returns an error only
// when opts.PrevHash is set but not a valid 32-byte hex hash, which would mean
// a resume against a corrupt or wrong head.
func NewHandler(w io.Writer, opts *Options) (*Handler, error) {
	if opts == nil {
		opts = &Options{}
	}
	var prev [hashLen]byte
	if opts.PrevHash != "" {
		p, err := decodeHash(opts.PrevHash)
		if err != nil {
			return nil, err
		}
		prev = p
	}
	return &Handler{
		core: &core{
			seq:    opts.StartSeq,
			prev:   prev,
			signer: opts.Signer,
			w:      w,
		},
		level: opts.Level,
	}, nil
}

// Enabled reports whether level is recorded. With no configured level, every
// record is captured, which is what an audit log wants.
func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	if h.level == nil {
		return true
	}
	return level >= h.level.Level()
}

// Handle appends one entry to the chain. It is safe for concurrent use: the
// hash, sign, and write happen under a single lock so chain links never
// interleave.
func (h *Handler) Handle(_ context.Context, r slog.Record) error {
	data, err := h.render(r)
	if err != nil {
		return err
	}

	ts := ""
	if !r.Time.IsZero() {
		ts = r.Time.UTC().Format(time.RFC3339Nano)
	}

	h.core.mu.Lock()
	defer h.core.mu.Unlock()

	sum := entryHash(h.core.seq, h.core.prev, ts, data)
	e := Entry{
		Seq:  h.core.seq,
		Time: ts,
		Prev: hex.EncodeToString(h.core.prev[:]),
		Data: data,
		Hash: hex.EncodeToString(sum[:]),
	}
	if h.core.signer != nil {
		e.Sig = hex.EncodeToString(ed25519.Sign(h.core.signer, sum[:]))
	}

	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	if _, err := h.core.w.Write(line); err != nil {
		return err
	}

	// Advance the chain only after a successful write, so a failed sink does
	// not leave a gap that would later read as tampering.
	h.core.prev = sum
	h.core.seq++
	return nil
}

// WithAttrs returns a Handler that always records attrs, sharing the same
// underlying chain. attrs are flattened under the groups open right now.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	prefix := h.prefix()
	merged := make([]kv, len(h.attrs), len(h.attrs)+len(attrs))
	copy(merged, h.attrs)
	for _, a := range attrs {
		merged = flatten(merged, prefix, a)
	}
	return &Handler{core: h.core, attrs: merged, groups: h.groups, level: h.level}
}

// WithGroup returns a Handler that nests subsequent record attrs under name,
// rendered as a dotted key prefix. An empty name is a no-op per slog.
func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	groups := make([]string, len(h.groups)+1)
	copy(groups, h.groups)
	groups[len(h.groups)] = name
	return &Handler{core: h.core, attrs: h.attrs, groups: groups, level: h.level}
}

// prefix is the dotted key prefix for the currently open groups.
func (h *Handler) prefix() string {
	if len(h.groups) == 0 {
		return ""
	}
	return strings.Join(h.groups, ".") + "."
}

// render builds the verbatim data bytes for a record: level, msg, the handler's
// accumulated attrs, and the record's own attrs (under the open groups).
func (h *Handler) render(r slog.Record) (json.RawMessage, error) {
	out := make(map[string]any, len(h.attrs)+4)
	out["level"] = r.Level.String()
	out["msg"] = r.Message
	for _, a := range h.attrs {
		out[a.key] = a.val
	}
	prefix := h.prefix()
	var rec []kv
	r.Attrs(func(a slog.Attr) bool {
		rec = flatten(rec, prefix, a)
		return true
	})
	for _, a := range rec {
		out[a.key] = a.val
	}
	return json.Marshal(out)
}

// flatten resolves one attr into out, folding groups into dotted keys. It
// mirrors slog's conventions: an empty-key group is inlined (no prefix
// segment), and an empty-key non-group attr is dropped.
func flatten(out []kv, prefix string, a slog.Attr) []kv {
	a.Value = a.Value.Resolve()
	if a.Value.Kind() == slog.KindGroup {
		groupAttrs := a.Value.Group()
		if len(groupAttrs) == 0 {
			return out
		}
		next := prefix
		if a.Key != "" {
			next = prefix + a.Key + "."
		}
		for _, ga := range groupAttrs {
			out = flatten(out, next, ga)
		}
		return out
	}
	if a.Key == "" {
		return out
	}
	return append(out, kv{key: prefix + a.Key, val: safeValue(a.Value)})
}

// safeValue converts a non-group slog.Value into something json.Marshal is
// guaranteed to accept, so one exotic value can never abort a chain write.
func safeValue(v slog.Value) any {
	switch v.Kind() {
	case slog.KindString:
		return v.String()
	case slog.KindInt64:
		return v.Int64()
	case slog.KindUint64:
		return v.Uint64()
	case slog.KindFloat64:
		return v.Float64()
	case slog.KindBool:
		return v.Bool()
	case slog.KindDuration:
		return v.Duration().String()
	case slog.KindTime:
		return v.Time().UTC().Format(time.RFC3339Nano)
	default:
		raw := v.Any()
		if _, err := json.Marshal(raw); err != nil {
			return v.String()
		}
		return raw
	}
}
