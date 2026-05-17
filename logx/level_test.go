package logx

import (
	"log/slog"
	"testing"
)

func TestLevelFromString(t *testing.T) {
	cases := []struct {
		in      string
		want    slog.Level
		wantErr bool
	}{
		{"", slog.LevelInfo, false},
		{"INFO", slog.LevelInfo, false},
		{" info ", slog.LevelInfo, false},
		{"debug", slog.LevelDebug, false},
		{"DBG", slog.LevelDebug, false},
		{"trace", slog.LevelDebug, false},
		{"warn", slog.LevelWarn, false},
		{"Warning", slog.LevelWarn, false},
		{"error", slog.LevelError, false},
		{"err", slog.LevelError, false},
		{"fatal", slog.LevelError, false},
		{"nope", slog.LevelInfo, true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := LevelFromString(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr=%v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEnvLevel(t *testing.T) {
	t.Setenv("LOG_LEVEL", "warn")
	if got := EnvLevel(slog.LevelInfo); got != slog.LevelWarn {
		t.Fatalf("EnvLevel = %v, want WARN", got)
	}
	t.Setenv("LOG_LEVEL", "")
	if got := EnvLevel(slog.LevelInfo); got != slog.LevelInfo {
		t.Fatalf("EnvLevel unset = %v, want INFO", got)
	}
	t.Setenv("LOG_LEVEL", "garbage")
	if got := EnvLevel(slog.LevelError); got != slog.LevelError {
		t.Fatalf("EnvLevel garbage = %v, want fallback ERROR", got)
	}
}
