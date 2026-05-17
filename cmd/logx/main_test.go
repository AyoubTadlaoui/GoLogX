package main

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeJSONLog(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "app.json")
	content := strings.Join([]string{
		`{"time":"2026-01-01T10:00:00Z","level":"DEBUG","msg":"hidden by default"}`,
		`{"time":"2026-01-01T10:00:01Z","level":"INFO","msg":"hello","user":"ayoub","id":42}`,
		`{"time":"2026-01-01T10:00:02Z","level":"WARN","msg":"slow query","ms":230}`,
		`{"time":"2026-01-01T10:00:03Z","level":"ERROR","msg":"boom","err":"timeout"}`,
		`not json — should be passed through`,
		``,
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRun_StdinPipeline(t *testing.T) {
	input := strings.NewReader(`{"time":"2026-01-01T10:00:00Z","level":"INFO","msg":"hello","x":1}` + "\n")
	var stdout, stderr bytes.Buffer
	exit := run(input, &stdout, &stderr, []string{"-no-color"})
	if exit != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "INF") || !strings.Contains(out, "hello") || !strings.Contains(out, "x=1") {
		t.Fatalf("unexpected stdout: %q", out)
	}
}

func TestRun_FileLevelFilter(t *testing.T) {
	path := makeJSONLog(t)
	var stdout, stderr bytes.Buffer
	exit := run(strings.NewReader(""), &stdout, &stderr, []string{"-no-color", "-level=warn", path})
	if exit != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
	}
	out := stdout.String()
	if strings.Contains(out, "hello") || strings.Contains(out, "hidden by default") {
		t.Errorf("level filter leaked INFO/DEBUG: %q", out)
	}
	if !strings.Contains(out, "slow query") || !strings.Contains(out, "boom") {
		t.Errorf("WARN/ERROR missing: %q", out)
	}
}

func TestRun_GrepFilter(t *testing.T) {
	path := makeJSONLog(t)
	var stdout, stderr bytes.Buffer
	exit := run(strings.NewReader(""), &stdout, &stderr, []string{"-no-color", "-grep=timeout", path})
	if exit != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
	}
	out := stdout.String()
	if strings.Contains(out, "hello") || strings.Contains(out, "slow query") {
		t.Errorf("grep should have filtered those out: %q", out)
	}
	if !strings.Contains(out, "boom") {
		t.Errorf("matching line missing: %q", out)
	}
}

func TestRun_PassThroughNonJSON(t *testing.T) {
	path := makeJSONLog(t)
	var stdout, stderr bytes.Buffer
	_ = run(strings.NewReader(""), &stdout, &stderr, []string{"-no-color", path})
	if !strings.Contains(stdout.String(), "not json — should be passed through") {
		t.Fatalf("pass-through line missing: %q", stdout.String())
	}
}

func TestRun_MissingFileExitNonZero(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := run(strings.NewReader(""), &stdout, &stderr, []string{"/definitely/does/not/exist"})
	if exit == 0 {
		t.Fatalf("exit=0, want non-zero (stderr=%q)", stderr.String())
	}
	if stderr.Len() == 0 {
		t.Fatal("expected stderr message for missing file")
	}
}

func TestRun_VersionFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := run(strings.NewReader(""), &stdout, &stderr, []string{"-version"})
	if exit != 0 {
		t.Fatalf("exit=%d, want 0", exit)
	}
	if !strings.Contains(stderr.String(), version) {
		t.Fatalf("version output missing %q: %q", version, stderr.String())
	}
}

func TestRun_BadLevelExitTwo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := run(strings.NewReader(""), &stdout, &stderr, []string{"-level=garbage"})
	if exit != 2 {
		t.Fatalf("exit=%d, want 2", exit)
	}
}

func TestDecodeRecord_NumericLevel(t *testing.T) {
	rec, ok := decodeRecord([]byte(`{"level":8,"msg":"err-as-number"}`))
	if !ok {
		t.Fatal("decodeRecord returned ok=false")
	}
	if rec.Level != slog.LevelError {
		t.Fatalf("level = %v, want ERROR(8)", rec.Level)
	}
}

func TestDecodeRecord_SourceObject(t *testing.T) {
	rec, ok := decodeRecord([]byte(`{"level":"info","msg":"x","source":{"file":"/a/b/c.go","line":42}}`))
	if !ok {
		t.Fatal("decodeRecord returned ok=false")
	}
	found := false
	rec.Attrs(func(a slog.Attr) bool {
		if a.Key == "source" && strings.HasSuffix(a.Value.String(), "c.go:42") {
			found = true
		}
		return true
	})
	if !found {
		t.Fatal("source attr missing")
	}
}

// quiet unused-import warning for io
var _ = io.Discard
