package main

import (
	"bytes"
	"crypto/ed25519"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AyoubTadlaoui/GoLogX/audit"
)

// writeChain writes a small audit chain to path, signed if signer is non-nil.
func writeChain(t *testing.T, path string, signer ed25519.PrivateKey) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	h, err := audit.NewHandler(f, &audit.Options{Signer: signer})
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(h)
	log.Info("package installed", "name", "left-pad", "version", "1.3.0")
	log.Warn("outbound connection", "host", "example.com")
	log.Error("blocked", "reason", "typosquat")
}

func TestVerifyCmd_OKUnsigned(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	writeChain(t, path, nil)

	var stdout, stderr bytes.Buffer
	exit := run(strings.NewReader(""), &stdout, &stderr, []string{"verify", path})
	if exit != 0 {
		t.Fatalf("exit=%d, want 0 (stderr=%q)", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), "OK") || !strings.Contains(stdout.String(), "3 entries") {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
}

func TestVerifyCmd_DetectsTamper(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	writeChain(t, path, nil)

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := bytes.Replace(raw, []byte("left-pad"), []byte("left-pwd"), 1)
	if bytes.Equal(raw, tampered) {
		t.Fatal("tamper replacement did not change the file")
	}
	if err := os.WriteFile(path, tampered, 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	exit := run(strings.NewReader(""), &stdout, &stderr, []string{"verify", path})
	if exit != 1 {
		t.Fatalf("exit=%d, want 1 for tampered file (stdout=%q)", exit, stdout.String())
	}
	if !strings.Contains(stdout.String(), "FAIL") || !strings.Contains(stdout.String(), "entry 0") {
		t.Fatalf("expected FAIL at entry 0, got: %q", stdout.String())
	}
}

func TestVerifyCmd_SignedWithPubkey(t *testing.T) {
	dir := t.TempDir()
	pub, priv, err := audit.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	pubPEM, err := audit.MarshalPublicKeyPEM(pub)
	if err != nil {
		t.Fatal(err)
	}
	pubPath := filepath.Join(dir, "audit.pub")
	if err := os.WriteFile(pubPath, pubPEM, 0o644); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "audit.log")
	writeChain(t, logPath, priv)

	var stdout, stderr bytes.Buffer
	exit := run(strings.NewReader(""), &stdout, &stderr, []string{"verify", "-pubkey", pubPath, logPath})
	if exit != 0 {
		t.Fatalf("exit=%d, want 0 (stderr=%q)", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), "chain + signatures") {
		t.Fatalf("expected signature-verified mode label, got: %q", stdout.String())
	}
}

func TestVerifyCmd_SignedFileMissingSigFailsUnderKey(t *testing.T) {
	dir := t.TempDir()
	pub, _, err := audit.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	pubPEM, _ := audit.MarshalPublicKeyPEM(pub)
	pubPath := filepath.Join(dir, "audit.pub")
	if err := os.WriteFile(pubPath, pubPEM, 0o644); err != nil {
		t.Fatal(err)
	}
	// Unsigned chain, but verified with a key -> must fail (entries unsigned).
	logPath := filepath.Join(dir, "audit.log")
	writeChain(t, logPath, nil)

	var stdout, stderr bytes.Buffer
	exit := run(strings.NewReader(""), &stdout, &stderr, []string{"verify", "-pubkey", pubPath, logPath})
	if exit != 1 {
		t.Fatalf("exit=%d, want 1 (unsigned chain under a key) stdout=%q", exit, stdout.String())
	}
}

func TestVerifyCmd_NoFilesExitTwo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := run(strings.NewReader(""), &stdout, &stderr, []string{"verify"})
	if exit != 2 {
		t.Fatalf("exit=%d, want 2", exit)
	}
}

func TestVerifyCmd_MissingFileExitTwo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := run(strings.NewReader(""), &stdout, &stderr, []string{"verify", "/definitely/not/here.log"})
	if exit != 2 {
		t.Fatalf("exit=%d, want 2 (stderr=%q)", exit, stderr.String())
	}
}

func TestKeygenCmd_WritesUsableKeypair(t *testing.T) {
	base := filepath.Join(t.TempDir(), "audit")
	var stdout, stderr bytes.Buffer
	exit := run(strings.NewReader(""), &stdout, &stderr, []string{"keygen", "-out", base})
	if exit != 0 {
		t.Fatalf("exit=%d, want 0 (stderr=%q)", exit, stderr.String())
	}

	privData, err := os.ReadFile(base + ".key")
	if err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(base + ".key"); fi != nil && fi.Mode().Perm() != 0o600 {
		t.Fatalf("private key perms = %v, want 0600", fi.Mode().Perm())
	}
	pubData, err := os.ReadFile(base + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := audit.ParsePrivateKeyPEM(privData); err != nil {
		t.Fatalf("private key does not parse: %v", err)
	}
	if _, err := audit.ParsePublicKeyPEM(pubData); err != nil {
		t.Fatalf("public key does not parse: %v", err)
	}
}

func TestKeygenCmd_RefusesOverwrite(t *testing.T) {
	base := filepath.Join(t.TempDir(), "audit")
	var stdout, stderr bytes.Buffer
	if exit := run(strings.NewReader(""), &stdout, &stderr, []string{"keygen", "-out", base}); exit != 0 {
		t.Fatalf("first keygen exit=%d, want 0", exit)
	}
	stdout.Reset()
	stderr.Reset()
	exit := run(strings.NewReader(""), &stdout, &stderr, []string{"keygen", "-out", base})
	if exit != 2 {
		t.Fatalf("second keygen exit=%d, want 2 (refuse overwrite)", exit)
	}
	if !strings.Contains(stderr.String(), "already exists") {
		t.Fatalf("expected overwrite refusal message, got: %q", stderr.String())
	}
}
