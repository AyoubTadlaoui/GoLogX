package logx

import (
	"io"
	"log/slog"
	"os"
)

// Format selects the output format for New.
type Format int

const (
	// FormatPretty produces colored, human-friendly output.
	FormatPretty Format = iota
	// FormatJSON produces one JSON object per line (slog.NewJSONHandler).
	FormatJSON
	// FormatText produces logfmt-style key=value lines (slog.NewTextHandler).
	FormatText
)

// Options configures New. Zero value is valid: INFO level, pretty format,
// writing to stderr.
type Options struct {
	// Level is the minimum level emitted. nil → INFO.
	Level slog.Leveler
	// Format selects which built-in handler to use when Handler is nil.
	Format Format
	// Output is the writer the chosen handler writes to. nil → os.Stderr.
	Output io.Writer
	// AddSource includes the caller location on each entry.
	AddSource bool
	// NoColor forces colors off (only honored by FormatPretty).
	NoColor bool
	// TimeFormat overrides the timestamp layout for FormatPretty.
	TimeFormat string
	// Handler, if non-nil, fully replaces the handler — Format/Output/etc.
	// are ignored. Use this to plug in MultiHandler or a custom handler.
	Handler slog.Handler
}

// New builds a *slog.Logger from opts. It never returns nil.
func New(opts Options) *slog.Logger {
	return slog.New(buildHandler(opts))
}

// NewHandler returns just the handler. Useful when you want to assemble it
// into a MultiHandler yourself.
func NewHandler(opts Options) slog.Handler {
	return buildHandler(opts)
}

func buildHandler(opts Options) slog.Handler {
	if opts.Handler != nil {
		return opts.Handler
	}
	if opts.Output == nil {
		opts.Output = os.Stderr
	}
	if opts.Level == nil {
		opts.Level = slog.LevelInfo
	}
	switch opts.Format {
	case FormatJSON:
		return slog.NewJSONHandler(opts.Output, &slog.HandlerOptions{
			Level:     opts.Level,
			AddSource: opts.AddSource,
		})
	case FormatText:
		return slog.NewTextHandler(opts.Output, &slog.HandlerOptions{
			Level:     opts.Level,
			AddSource: opts.AddSource,
		})
	default:
		return NewPrettyHandler(opts.Output, &PrettyHandlerOptions{
			Level:      opts.Level,
			TimeFormat: opts.TimeFormat,
			NoColor:    opts.NoColor,
			AddSource:  opts.AddSource,
		})
	}
}

// Default builds an opinionated production-friendly logger: JSON, INFO level
// (overridable with $LOG_LEVEL), source attached, writing to stderr.
func Default() *slog.Logger {
	return New(Options{
		Level:     EnvLevel(slog.LevelInfo),
		Format:    FormatJSON,
		AddSource: true,
	})
}

// Dev builds an opinionated development logger: pretty colored output, DEBUG
// level (overridable with $LOG_LEVEL), writing to stderr.
func Dev() *slog.Logger {
	return New(Options{
		Level:  EnvLevel(slog.LevelDebug),
		Format: FormatPretty,
	})
}
