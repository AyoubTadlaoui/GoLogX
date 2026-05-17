package logx

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// LevelFromString parses a level name into a slog.Level. It accepts the
// canonical slog names (debug, info, warn, error) plus a few common aliases
// (warning, err, fatal, trace). Matching is case-insensitive.
//
// Unknown values return an error so callers can decide how to handle them.
func LevelFromString(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug", "dbg", "trace":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error", "err", "fatal":
		return slog.LevelError, nil
	}
	return slog.LevelInfo, fmt.Errorf("logx: unknown level %q", s)
}

// EnvLevel reads $LOG_LEVEL and returns the parsed slog.Level. If the variable
// is unset or unparseable, fallback is returned.
//
// This is the recommended way to wire log level to operations: ops can flip
// LOG_LEVEL=debug at runtime without a code change.
func EnvLevel(fallback slog.Level) slog.Level {
	v := os.Getenv("LOG_LEVEL")
	if v == "" {
		return fallback
	}
	lvl, err := LevelFromString(v)
	if err != nil {
		return fallback
	}
	return lvl
}
