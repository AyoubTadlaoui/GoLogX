package logx

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestNew_FormatPretty(t *testing.T) {
	var buf bytes.Buffer
	log := New(Options{Format: FormatPretty, Output: &buf, NoColor: true})
	log.Info("hi", "k", "v")
	if !strings.Contains(buf.String(), "INF") || !strings.Contains(buf.String(), "k=v") {
		t.Fatalf("pretty output looks wrong: %q", buf.String())
	}
}

func TestNew_FormatJSON(t *testing.T) {
	var buf bytes.Buffer
	log := New(Options{Format: FormatJSON, Output: &buf})
	log.Info("hi", "k", "v")
	if !strings.Contains(buf.String(), `"msg":"hi"`) || !strings.Contains(buf.String(), `"k":"v"`) {
		t.Fatalf("json output looks wrong: %q", buf.String())
	}
}

func TestNew_FormatText(t *testing.T) {
	var buf bytes.Buffer
	log := New(Options{Format: FormatText, Output: &buf})
	log.Info("hi", "k", "v")
	if !strings.Contains(buf.String(), "msg=hi") || !strings.Contains(buf.String(), "k=v") {
		t.Fatalf("text output looks wrong: %q", buf.String())
	}
}

func TestNew_HandlerOverride(t *testing.T) {
	var buf bytes.Buffer
	custom := NewPrettyHandler(&buf, &PrettyHandlerOptions{NoColor: true})
	log := New(Options{Handler: custom, Output: nil, Format: FormatJSON /* ignored */})
	log.Info("hi")
	if !strings.Contains(buf.String(), "INF") {
		t.Fatalf("override handler not used: %q", buf.String())
	}
}

func TestNew_LevelDefault(t *testing.T) {
	var buf bytes.Buffer
	log := New(Options{Output: &buf, NoColor: true})
	log.Debug("hidden")
	log.Info("seen")
	out := buf.String()
	if strings.Contains(out, "hidden") {
		t.Fatalf("default level should suppress DEBUG: %q", out)
	}
	if !strings.Contains(out, "seen") {
		t.Fatalf("INFO missing: %q", out)
	}
}

func TestDev(t *testing.T) {
	log := Dev()
	if log == nil {
		t.Fatal("Dev returned nil")
	}
	if !log.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("Dev should be enabled at DEBUG by default")
	}
}

func TestDefault(t *testing.T) {
	log := Default()
	if log == nil {
		t.Fatal("Default returned nil")
	}
	if log.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("Default should NOT be enabled at DEBUG by default")
	}
	if !log.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("Default should be enabled at INFO")
	}
}
