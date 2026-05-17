package logx

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// fixedTime returns a logger whose records all use a known timestamp so
// golden output is deterministic.
func newPrettyTestLogger(buf *bytes.Buffer, noColor bool) *slog.Logger {
	h := NewPrettyHandler(buf, &PrettyHandlerOptions{
		Level:      slog.LevelDebug,
		TimeFormat: "15:04:05",
		NoColor:    noColor,
	})
	return slog.New(h)
}

func TestPrettyHandler_OutputShape(t *testing.T) {
	var buf bytes.Buffer
	log := newPrettyTestLogger(&buf, true) // no color → easier asserts

	log.Info("server up", "port", 8080, "env", "prod")
	out := buf.String()
	if !strings.Contains(out, "INF") {
		t.Errorf("missing level tag in %q", out)
	}
	if !strings.Contains(out, "server up") {
		t.Errorf("missing message in %q", out)
	}
	if !strings.Contains(out, "port=8080") {
		t.Errorf("missing int attr in %q", out)
	}
	if !strings.Contains(out, "env=prod") {
		t.Errorf("missing string attr in %q", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("missing trailing newline in %q", out)
	}
}

func TestPrettyHandler_LevelFilter(t *testing.T) {
	var buf bytes.Buffer
	h := NewPrettyHandler(&buf, &PrettyHandlerOptions{Level: slog.LevelWarn, NoColor: true})
	log := slog.New(h)

	log.Debug("hidden")
	log.Info("hidden")
	log.Warn("seen", "k", "v")
	log.Error("also seen")

	out := buf.String()
	if strings.Contains(out, "hidden") {
		t.Errorf("level filter leaked sub-WARN lines: %q", out)
	}
	if !strings.Contains(out, "WRN") || !strings.Contains(out, "ERR") {
		t.Errorf("missing WRN/ERR lines in %q", out)
	}
}

func TestPrettyHandler_Colors(t *testing.T) {
	var buf bytes.Buffer
	log := newPrettyTestLogger(&buf, false)
	log.Info("hello")
	if !strings.Contains(buf.String(), "\x1b[") {
		t.Errorf("color output expected ANSI escape, got %q", buf.String())
	}
}

func TestPrettyHandler_WithAttrsAndGroup(t *testing.T) {
	var buf bytes.Buffer
	h := NewPrettyHandler(&buf, &PrettyHandlerOptions{Level: slog.LevelDebug, NoColor: true})
	log := slog.New(h).With("svc", "api").WithGroup("req")

	log.Info("hit", "path", "/")
	out := buf.String()

	if !strings.Contains(out, "svc=api") {
		t.Errorf("with-attr missing in %q", out)
	}
	if !strings.Contains(out, "req.path=/") {
		t.Errorf("group prefix missing in %q", out)
	}
}

func TestPrettyHandler_QuotesStringsWithSpaces(t *testing.T) {
	var buf bytes.Buffer
	log := newPrettyTestLogger(&buf, true)
	log.Info("m", "msg", "hello world")
	if !strings.Contains(buf.String(), `msg="hello world"`) {
		t.Errorf("expected quoted value, got %q", buf.String())
	}
}

func TestPrettyHandler_GroupAttrFlattens(t *testing.T) {
	var buf bytes.Buffer
	log := newPrettyTestLogger(&buf, true)
	log.Info("m", slog.Group("user", slog.String("name", "ayoub"), slog.Int("age", 29)))
	out := buf.String()
	if !strings.Contains(out, "user.name=ayoub") || !strings.Contains(out, "user.age=29") {
		t.Errorf("group did not flatten: %q", out)
	}
}

func TestPrettyHandler_AddSource(t *testing.T) {
	var buf bytes.Buffer
	h := NewPrettyHandler(&buf, &PrettyHandlerOptions{Level: slog.LevelDebug, NoColor: true, AddSource: true})
	log := slog.New(h)
	log.Info("with source")
	if !strings.Contains(buf.String(), "pretty_test.go:") {
		t.Errorf("expected source path, got %q", buf.String())
	}
}

func TestPrettyHandler_ConcurrentWrites(t *testing.T) {
	var buf bytes.Buffer
	log := newPrettyTestLogger(&buf, true)
	var wg sync.WaitGroup
	const N = 200
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			log.Info("m", "i", i)
		}(i)
	}
	wg.Wait()
	lines := strings.Count(buf.String(), "\n")
	if lines != N {
		t.Fatalf("got %d lines, want %d (writes likely interleaved)", lines, N)
	}
}

// silence unused-import warning for time when no other test uses it directly
var _ = time.RFC3339
