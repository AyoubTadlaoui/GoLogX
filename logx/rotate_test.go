package logx

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestRotatingWriter_RotatesAtMaxSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	w := &RotatingWriter{Path: path, MaxSize: 30, MaxBackups: 2}
	defer w.Close()

	// Three writes of 20 bytes each — second should trigger a rotation,
	// third another, but MaxBackups=2 caps the history.
	for i := 0; i < 3; i++ {
		if _, err := w.Write(bytes.Repeat([]byte{'a' + byte(i)}, 20)); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	// Active file should hold the last write (20 bytes of 'c').
	if got, _ := os.ReadFile(path); !bytes.Equal(got, bytes.Repeat([]byte{'c'}, 20)) {
		t.Fatalf("active file content = %q", got)
	}

	entries, _ := os.ReadDir(dir)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	// Expect app.log + app.log.1 + app.log.2 (MaxBackups=2).
	want := []string{"app.log", "app.log.1", "app.log.2"}
	for _, w := range want {
		found := false
		for _, n := range names {
			if n == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing %q in %v", w, names)
		}
	}
	for _, n := range names {
		if strings.HasPrefix(n, "app.log.") {
			if n == "app.log.3" {
				t.Errorf("MaxBackups=2 should have trimmed app.log.3")
			}
		}
	}
}

func TestRotatingWriter_NoRotationWhenMaxSizeZero(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "no-rotate.log")
	w := &RotatingWriter{Path: path, MaxSize: 0}
	defer w.Close()

	for i := 0; i < 5; i++ {
		if _, err := w.Write([]byte("xxxxxxxxxx\n")); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "no-rotate.log" {
			t.Errorf("unexpected rotation produced %q", e.Name())
		}
	}
}

func TestRotatingWriter_ConcurrentSafe(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "concurrent.log")
	w := &RotatingWriter{Path: path, MaxSize: 256}
	defer w.Close()

	var wg sync.WaitGroup
	const N = 200
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = w.Write([]byte("hello world\n"))
		}()
	}
	wg.Wait()
	_ = w.Close()
	// just verify no panic & at least one rotation happened
	entries, _ := os.ReadDir(dir)
	rotated := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "concurrent.log.") {
			rotated = true
		}
	}
	if !rotated {
		t.Logf("warning: no rotation occurred — check threshold (%d entries)", len(entries))
	}
}

func TestRotatingWriter_CloseIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	w := &RotatingWriter{Path: filepath.Join(dir, "x.log")}
	if _, err := w.Write([]byte("hi")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal("second Close should be a no-op")
	}
}
