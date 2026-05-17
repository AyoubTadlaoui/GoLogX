// Package logx is a small, fast, zero-dependency toolkit on top of the
// standard library's log/slog.
//
// It provides three things slog does not ship with:
//
//   - PrettyHandler — an ANSI-colored, human-friendly handler for development.
//   - MultiHandler  — fan a single record out to N underlying handlers
//     (e.g. pretty to stderr + JSON to a file).
//   - RotatingWriter — a size-based rotating io.Writer with no dependencies.
//
// Plus a small set of helpers: New, LevelFromString, EnvLevel.
//
// Quickstart:
//
//	log := logx.New(logx.Options{
//	    Level:  slog.LevelInfo,
//	    Format: logx.FormatPretty,
//	})
//	log.Info("server up", "port", 8080)
//
// See the examples in this directory for typical setups (pretty-to-stderr,
// JSON-to-file, multi-sink).
package logx
