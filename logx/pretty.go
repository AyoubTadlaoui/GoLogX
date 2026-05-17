package logx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"sync"
	"time"
)

// PrettyHandlerOptions configures PrettyHandler.
type PrettyHandlerOptions struct {
	// Level is the minimum level emitted. nil means slog.LevelInfo.
	Level slog.Leveler
	// TimeFormat is passed to time.Time.Format. Empty means "15:04:05.000".
	TimeFormat string
	// NoColor disables ANSI escapes — set automatically when the output
	// is not a TTY (callers should pass a value reflecting that decision).
	NoColor bool
	// AddSource includes the caller file:line for each log entry.
	AddSource bool
}

// PrettyHandler is a slog.Handler that produces concise, color-coded,
// human-friendly output. It's designed for development; for shipping to log
// aggregators, prefer slog.NewJSONHandler.
//
// PrettyHandler is safe for concurrent use. It pools its output buffers, so
// most calls perform zero heap allocations beyond what slog itself does.
type PrettyHandler struct {
	w     io.Writer
	mu    *sync.Mutex // shared across cloned handlers so writes stay serialized
	opts  PrettyHandlerOptions
	attrs []slog.Attr
	group string // dotted current group prefix, e.g. "request.headers"
}

// NewPrettyHandler returns a PrettyHandler writing to w.
func NewPrettyHandler(w io.Writer, opts *PrettyHandlerOptions) *PrettyHandler {
	o := PrettyHandlerOptions{}
	if opts != nil {
		o = *opts
	}
	if o.Level == nil {
		o.Level = slog.LevelInfo
	}
	if o.TimeFormat == "" {
		o.TimeFormat = "15:04:05.000"
	}
	return &PrettyHandler{
		w:    w,
		mu:   &sync.Mutex{},
		opts: o,
	}
}

// Enabled implements slog.Handler.
func (h *PrettyHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

// WithAttrs implements slog.Handler.
func (h *PrettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	clone := *h
	clone.attrs = make([]slog.Attr, 0, len(h.attrs)+len(attrs))
	clone.attrs = append(clone.attrs, h.attrs...)
	clone.attrs = append(clone.attrs, attrs...)
	return &clone
}

// WithGroup implements slog.Handler.
func (h *PrettyHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := *h
	if clone.group == "" {
		clone.group = name
	} else {
		clone.group = clone.group + "." + name
	}
	return &clone
}

// ANSI color codes. Using constants keeps the hot path free of map lookups.
const (
	ansiReset  = "\x1b[0m"
	ansiDim    = "\x1b[2m"
	ansiBold   = "\x1b[1m"
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiBlue   = "\x1b[34m"
	ansiCyan   = "\x1b[36m"
)

var bufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

// Handle implements slog.Handler.
//
//nolint:gocyclo // the formatter is naturally branchy; splitting helps no one
func (h *PrettyHandler) Handle(_ context.Context, r slog.Record) error {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	color := !h.opts.NoColor

	// timestamp (dim)
	if !r.Time.IsZero() {
		writeColor(buf, ansiDim, color)
		buf.WriteString(r.Time.Format(h.opts.TimeFormat))
		writeColor(buf, ansiReset, color)
		buf.WriteByte(' ')
	}

	// level tag (colored, fixed width)
	writeColor(buf, levelColor(r.Level), color)
	buf.WriteString(levelLabel(r.Level))
	writeColor(buf, ansiReset, color)
	buf.WriteByte(' ')

	// message (bold)
	writeColor(buf, ansiBold, color)
	buf.WriteString(r.Message)
	writeColor(buf, ansiReset, color)

	// source (dim, after message)
	if h.opts.AddSource && r.PC != 0 {
		if src := recordSource(r); src != "" {
			buf.WriteByte(' ')
			writeColor(buf, ansiDim, color)
			buf.WriteByte('(')
			buf.WriteString(src)
			buf.WriteByte(')')
			writeColor(buf, ansiReset, color)
		}
	}

	// preset attrs (added via WithAttrs)
	for _, a := range h.attrs {
		writeAttr(buf, h.group, a, color)
	}
	// record-level attrs
	r.Attrs(func(a slog.Attr) bool {
		writeAttr(buf, h.group, a, color)
		return true
	})

	buf.WriteByte('\n')

	h.mu.Lock()
	_, err := h.w.Write(buf.Bytes())
	h.mu.Unlock()
	return err
}

func writeColor(buf *bytes.Buffer, code string, on bool) {
	if on {
		buf.WriteString(code)
	}
}

func levelLabel(l slog.Level) string {
	switch {
	case l < slog.LevelInfo:
		return "DBG"
	case l < slog.LevelWarn:
		return "INF"
	case l < slog.LevelError:
		return "WRN"
	default:
		return "ERR"
	}
}

func levelColor(l slog.Level) string {
	switch {
	case l < slog.LevelInfo:
		return ansiCyan
	case l < slog.LevelWarn:
		return ansiGreen
	case l < slog.LevelError:
		return ansiYellow
	default:
		return ansiRed
	}
}

func writeAttr(buf *bytes.Buffer, group string, a slog.Attr, color bool) {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) || (a.Key == "" && a.Value.Kind() != slog.KindGroup) {
		return
	}
	if a.Value.Kind() == slog.KindGroup {
		newGroup := a.Key
		if group != "" {
			newGroup = group + "." + a.Key
		}
		for _, ga := range a.Value.Group() {
			writeAttr(buf, newGroup, ga, color)
		}
		return
	}
	buf.WriteByte(' ')
	writeColor(buf, ansiBlue, color)
	if group != "" {
		buf.WriteString(group)
		buf.WriteByte('.')
	}
	buf.WriteString(a.Key)
	writeColor(buf, ansiReset, color)
	buf.WriteByte('=')
	writeValue(buf, a.Value)
}

func writeValue(buf *bytes.Buffer, v slog.Value) {
	switch v.Kind() {
	case slog.KindString:
		s := v.String()
		if needsQuoting(s) {
			buf.WriteString(strconv.Quote(s))
		} else {
			buf.WriteString(s)
		}
	case slog.KindInt64:
		buf.WriteString(strconv.FormatInt(v.Int64(), 10))
	case slog.KindUint64:
		buf.WriteString(strconv.FormatUint(v.Uint64(), 10))
	case slog.KindFloat64:
		buf.WriteString(strconv.FormatFloat(v.Float64(), 'g', -1, 64))
	case slog.KindBool:
		buf.WriteString(strconv.FormatBool(v.Bool()))
	case slog.KindDuration:
		buf.WriteString(v.Duration().String())
	case slog.KindTime:
		buf.WriteString(v.Time().Format(time.RFC3339))
	case slog.KindGroup, slog.KindLogValuer:
		// LogValuer is resolved upstream in writeAttr; Groups handled there too.
		// Fallthrough renders an opaque %v just in case.
		fmt.Fprintf(buf, "%v", v.Any())
	default:
		fmt.Fprintf(buf, "%v", v.Any())
	}
}

func needsQuoting(s string) bool {
	if s == "" {
		return true
	}
	for _, r := range s {
		if r <= ' ' || r == '=' || r == '"' {
			return true
		}
	}
	return false
}
