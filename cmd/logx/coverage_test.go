package main

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AyoubTadlaoui/GoLogX/audit"
)

// --- main.go: stripTrailingNewline -----------------------------------------

func TestStripTrailingNewline(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"with trailing newline", "hello\n", "hello"},
		{"without trailing newline", "hello", "hello"},
		{"empty", "", ""},
		{"only newline", "\n", ""},
		{"trailing CR kept (only \\n stripped)", "hello\r\n", "hello\r"},
		{"interior newline untouched", "a\nb", "a\nb"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := stripTrailingNewline([]byte(c.in))
			if string(got) != c.want {
				t.Fatalf("stripTrailingNewline(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// --- main.go: resolvedVersion (explicit + dev-fallback) ---------------------

func TestResolvedVersion_ExplicitVersionWins(t *testing.T) {
	// version is a package-level var; save and restore so we don't leak state
	// into other tests that read resolvedVersion().
	saved := version
	t.Cleanup(func() { version = saved })

	version = "v9.9.9"
	if got := resolvedVersion(); got != "v9.9.9" {
		t.Fatalf("resolvedVersion() = %q, want explicit %q", got, "v9.9.9")
	}
}

func TestResolvedVersion_DevFallsBackToBuildInfo(t *testing.T) {
	saved := version
	t.Cleanup(func() { version = saved })

	// "dev" forces the build-info branch. Under `go test`, debug.ReadBuildInfo
	// reports no release version (Main.Version is "" or "(devel)"), so the
	// function falls all the way through to `return version`, i.e. "dev".
	// That makes the dev-fallback path's final return value deterministic here.
	version = "dev"
	if got := resolvedVersion(); got != "dev" {
		t.Fatalf("resolvedVersion() on dev = %q, want %q under go test", got, "dev")
	}

	// The empty string is also treated as "unset": it takes the same build-info
	// branch (it is not returned verbatim by the first guard) and, with no
	// module version available, returns version itself (""). The key behavior
	// is that "" is NOT short-circuited as an explicit version.
	version = ""
	if got := resolvedVersion(); got != "" {
		t.Fatalf("resolvedVersion() on empty version = %q, want %q (no build version in test)", got, "")
	}
}

// --- main.go: streamReader (mixed JSON + non-JSON, pass-through, exit 0) -----

func TestStreamReader_MixedLinesPassThrough(t *testing.T) {
	input := strings.Join([]string{
		`{"time":"2026-01-01T10:00:01Z","level":"INFO","msg":"structured-line","k":"v"}`,
		`a raw panic line that is not json`,
		``, // empty line: skipped, must not appear or crash
		`{"level":"ERROR","msg":"second-structured"}`,
	}, "\n") + "\n"

	cfg := &config{level: slog.LevelDebug, timeFormat: "15:04:05.000", noColor: true}
	var out bytes.Buffer
	handler := slog.NewTextHandler(&out, &slog.HandlerOptions{})

	var stderr bytes.Buffer
	code := streamReader(strings.NewReader(input), &out, handler, cfg, &stderr)
	if code != 0 {
		t.Fatalf("streamReader exit = %d, want 0 (stderr=%q)", code, stderr.String())
	}
	got := out.String()
	// Non-JSON line is passed through verbatim.
	if !strings.Contains(got, "a raw panic line that is not json") {
		t.Fatalf("non-JSON line was not passed through: %q", got)
	}
	// JSON lines are re-emitted through the handler (msg shows up).
	if !strings.Contains(got, "structured-line") || !strings.Contains(got, "second-structured") {
		t.Fatalf("structured lines missing from output: %q", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

// errReader fails on Read, exercising streamReader's scanner.Err() path.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, os.ErrInvalid }

func TestStreamReader_ReadErrorExitOne(t *testing.T) {
	cfg := &config{level: slog.LevelDebug, timeFormat: "15:04:05.000", noColor: true}
	var out, stderr bytes.Buffer
	handler := slog.NewTextHandler(&out, nil)
	code := streamReader(errReader{}, &out, handler, cfg, &stderr)
	if code != 1 {
		t.Fatalf("streamReader on read error exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "read:") {
		t.Fatalf("expected read error on stderr, got: %q", stderr.String())
	}
}

// --- main.go: followFile (tail -f a growing file, then stop cleanly) --------

func TestFollowFile_PicksUpAppendedLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "follow.json")

	// Seed with one line already present so followFile streams existing bytes.
	if err := os.WriteFile(path,
		[]byte(`{"level":"INFO","msg":"seed-line"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := &config{level: slog.LevelDebug, timeFormat: "15:04:05.000", noColor: true}

	// syncBuf guards the shared output buffer: followFile writes from its
	// goroutine while the test reads it. It is heap-allocated and outlives the
	// test, so the follower goroutine (which can only stop on a non-EOF read
	// error that a regular file never produces) is harmless once we return:
	// it sits in time.Sleep/ReadBytes hitting EOF and touches only this buffer
	// under its mutex.
	out := &syncBuf{}
	stderr := &syncBuf{}
	handler := slog.NewTextHandler(out, nil)

	go followFile(path, out, handler, cfg, stderr)

	// Wait for the seed line to be processed (proves the existing-bytes path).
	waitForContains(t, out, "seed-line", 3*time.Second)

	// Append a new line; followFile polls every 200ms and must pick it up.
	// This is the core tail -f guarantee: data written after the open is seen.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"level":"WARN","msg":"appended-line"}` + "\n"); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	waitForContains(t, out, "appended-line", 3*time.Second)

	// The seed line was INFO and the appended line WARN; with -level=debug both
	// pass the threshold, so both messages must be present in order.
	got := out.String()
	if !strings.Contains(got, "seed-line") || !strings.Contains(got, "appended-line") {
		t.Fatalf("followFile output missing expected lines: %q", got)
	}
	if stderr.String() != "" {
		t.Fatalf("unexpected stderr from followFile: %q", stderr.String())
	}
}

func TestFollowFile_MissingFileExitOne(t *testing.T) {
	cfg := &config{level: slog.LevelDebug, timeFormat: "15:04:05.000", noColor: true}
	var out, stderr bytes.Buffer
	handler := slog.NewTextHandler(&out, nil)
	code := followFile(filepath.Join(t.TempDir(), "nope.json"), &out, handler, cfg, &stderr)
	if code != 1 {
		t.Fatalf("followFile on missing file exit = %d, want 1", code)
	}
	if stderr.Len() == 0 {
		t.Fatal("expected stderr message for missing file in followFile")
	}
}

// syncBuf is a goroutine-safe bytes.Buffer wrapper for the followFile test.
type syncBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// waitForContains polls the buffer until it contains want or the deadline hits.
func waitForContains(t *testing.T, b *syncBuf, want string, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if strings.Contains(b.String(), want) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %q; buffer was: %q", want, b.String())
}

// --- audit_cmd.go: modeLabel (all three branches) ---------------------------

func TestModeLabel_AllBranches(t *testing.T) {
	pub, _, err := audit.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	// Branch 1: a public key was supplied -> signatures were verified.
	if got := modeLabel(pub, &audit.Report{Signed: true}); got != "chain + signatures" {
		t.Fatalf("with pubkey: modeLabel = %q, want %q", got, "chain + signatures")
	}

	// Branch 2: no key, but the log carries signatures -> present but unchecked.
	got := modeLabel(nil, &audit.Report{Signed: true})
	if !strings.Contains(got, "signatures present but unchecked") {
		t.Fatalf("no key + signed: modeLabel = %q, want the unchecked-signatures text", got)
	}

	// Branch 3: no key and unsigned log -> chain only, unsigned.
	if got := modeLabel(nil, &audit.Report{Signed: false}); got != "chain only; log is unsigned" {
		t.Fatalf("no key + unsigned: modeLabel = %q, want %q", got, "chain only; log is unsigned")
	}
}

// --- audit_cmd.go: keygenCmd error paths ------------------------------------

func TestKeygenCmd_UnwritableOutExitOne(t *testing.T) {
	dir := t.TempDir()
	// Make a regular file and try to use it as a parent directory: writing
	// <blocker>/audit.key fails because the parent path component is a file.
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	outBase := filepath.Join(blocker, "audit") // blocker/audit.key cannot be created

	var stdout, stderr bytes.Buffer
	exit := keygenCmd(&stdout, &stderr, []string{"-out", outBase})
	if exit != 1 {
		t.Fatalf("keygenCmd exit = %d, want 1 (unwritable out); stderr=%q", exit, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("keygenCmd wrote to stdout on failure: %q", stdout.String())
	}
	if stderr.Len() == 0 {
		t.Fatal("expected an error message on stderr")
	}
	// Nothing should have been left behind.
	if _, err := os.Stat(outBase + ".key"); err == nil {
		t.Fatal("private key file was created despite the write failure")
	}
}

func TestKeygenCmd_BadFlagExitTwo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := keygenCmd(&stdout, &stderr, []string{"-nope"})
	if exit != 2 {
		t.Fatalf("keygenCmd with bad flag exit = %d, want 2", exit)
	}
}

func TestKeygenCmd_HelpExitZero(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := keygenCmd(&stdout, &stderr, []string{"-h"})
	if exit != 0 {
		t.Fatalf("keygenCmd -h exit = %d, want 0", exit)
	}
}

// --- audit_cmd.go: verifyCmd pubkey + quiet paths ---------------------------

func TestVerifyCmd_MissingPubkeyFileExitTwo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	writeChain(t, path, nil)

	var stdout, stderr bytes.Buffer
	exit := verifyCmd(&stdout, &stderr, []string{"-pubkey", "/no/such/key.pub", path})
	if exit != 2 {
		t.Fatalf("verifyCmd missing pubkey exit = %d, want 2", exit)
	}
	if stderr.Len() == 0 {
		t.Fatal("expected stderr message for missing pubkey")
	}
}

func TestVerifyCmd_MalformedPubkeyExitTwo(t *testing.T) {
	dir := t.TempDir()
	pubPath := filepath.Join(dir, "bad.pub")
	if err := os.WriteFile(pubPath, []byte("this is not a PEM public key\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "audit.log")
	writeChain(t, logPath, nil)

	var stdout, stderr bytes.Buffer
	exit := verifyCmd(&stdout, &stderr, []string{"-pubkey", pubPath, logPath})
	if exit != 2 {
		t.Fatalf("verifyCmd malformed pubkey exit = %d, want 2 (stderr=%q)", exit, stderr.String())
	}
	if stderr.Len() == 0 {
		t.Fatal("expected stderr message for malformed pubkey")
	}
}

func TestVerifyCmd_QuietTamperedExitOneNoStdout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	writeChain(t, path, nil)

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := bytes.Replace(raw, []byte("left-pad"), []byte("left-pwn"), 1)
	if bytes.Equal(raw, tampered) {
		t.Fatal("tamper replacement did not change the file")
	}
	if err := os.WriteFile(path, tampered, 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	exit := verifyCmd(&stdout, &stderr, []string{"-quiet", path})
	if exit != 1 {
		t.Fatalf("verifyCmd -quiet tampered exit = %d, want 1", exit)
	}
	if stdout.Len() != 0 {
		t.Fatalf("-quiet must print nothing to stdout, got: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("-quiet must print nothing to stderr on tamper, got: %q", stderr.String())
	}
}

func TestVerifyCmd_QuietMissingFileNoOutput(t *testing.T) {
	// -quiet suppresses the per-file read error too; only the exit code talks.
	var stdout, stderr bytes.Buffer
	exit := verifyCmd(&stdout, &stderr, []string{"-quiet", "/definitely/not/here.log"})
	if exit != 2 {
		t.Fatalf("verifyCmd -quiet missing file exit = %d, want 2", exit)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("-quiet leaked output: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestVerifyCmd_BadFlagExitTwo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := verifyCmd(&stdout, &stderr, []string{"-bogus"})
	if exit != 2 {
		t.Fatalf("verifyCmd bad flag exit = %d, want 2", exit)
	}
}

// Ensure processLine via handler in streamReader actually runs the handler path
// for a non-default context (regression guard on handler invocation).
func TestProcessLine_HandlerInvoked(t *testing.T) {
	var out bytes.Buffer
	h := slog.NewTextHandler(&out, nil)
	cfg := &config{level: slog.LevelInfo}
	processLine([]byte(`{"level":"INFO","msg":"hit"}`), &out, h, cfg)
	if !strings.Contains(out.String(), "hit") {
		t.Fatalf("handler was not invoked for an INFO record: %q", out.String())
	}
	// Below-threshold record is dropped: nothing new written.
	before := out.Len()
	processLine([]byte(`{"level":"DEBUG","msg":"drop"}`), &out, h, cfg)
	if out.Len() != before {
		t.Fatalf("below-level record should have been dropped: %q", out.String())
	}
}
