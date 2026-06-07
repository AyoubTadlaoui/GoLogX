// Command audit-example shows a tamper-evident audit trail end to end: it
// records what an AI agent did into a signed, hash-chained log, verifies the
// log offline with only the public key, then tampers with one entry and shows
// the verifier catch it at the exact line.
//
// Run it:
//
//	go run ./examples/audit
package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/AyoubTadlaoui/GoLogX/audit"
	"github.com/AyoubTadlaoui/GoLogX/logx"
)

func main() {
	if err := demo(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func demo() error {
	dir, err := os.MkdirTemp("", "gologx-audit")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	logPath := filepath.Join(dir, "agent-audit.log")

	// A signing key. In production the private key lives off the logging host
	// (anyone who holds it can forge entries); here we make one on the fly.
	pub, priv, err := audit.GenerateKey()
	if err != nil {
		return err
	}

	// One record fans out to two sinks: a pretty handler so a human can watch,
	// and a signed, hash-chained audit handler writing to disk.
	f, err := os.Create(logPath)
	if err != nil {
		return err
	}
	defer f.Close()
	chain, err := audit.NewHandler(f, &audit.Options{Signer: priv})
	if err != nil {
		return err
	}
	pretty := logx.NewHandler(logx.Options{Format: logx.FormatPretty})
	log := slog.New(logx.NewMultiHandler(pretty, chain))

	// Record what an AI agent did. npmguard decides whether an install is safe;
	// this is the tamper-proof record that it then actually happened.
	log.Info("npm install approved", "package", "left-pad", "version", "1.3.0", "verdict", "clean")
	log.Warn("npm install blocked", "package", "loadsh", "reason", "typosquat of lodash")
	log.Info("command executed", "cmd", "node build.js", "exit", 0)

	// Verify offline with only the public key.
	rep, err := audit.VerifyFile(logPath, pub)
	if err != nil {
		return err
	}
	fmt.Printf("\nverify before tamper: ok=%v entries=%d\n", rep.OK, rep.Entries)

	// An attacker edits the log to hide that a typosquat was blocked, rewriting
	// "loadsh" to "lodash" so the line reads like a normal install.
	if err := forge(logPath); err != nil {
		return err
	}

	rep, err = audit.VerifyFile(logPath, pub)
	if err != nil {
		return err
	}
	fmt.Printf("verify after tamper:  ok=%v badSeq=%d\n  reason: %s\n", rep.OK, rep.BadSeq, rep.Reason)
	if rep.OK {
		return fmt.Errorf("tamper was NOT detected — this should never happen")
	}
	fmt.Println("\nthe edit was caught at the exact entry. nothing left the chain unseen.")
	return nil
}

// forge rewrites one byte sequence in the log to simulate an attacker editing
// history after the fact.
func forge(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	raw = replaceFirst(raw, []byte("loadsh"), []byte("lodash"))
	return os.WriteFile(path, raw, 0o600)
}

// replaceFirst replaces the first occurrence of old with new in b. new must be
// the same length as old so the surrounding JSON stays well-formed, proving the
// chain catches even a length-preserving edit.
func replaceFirst(b, old, new []byte) []byte {
	i := indexOf(b, old)
	if i < 0 || len(old) != len(new) {
		return b
	}
	out := make([]byte, len(b))
	copy(out, b)
	copy(out[i:], new)
	return out
}

func indexOf(b, sub []byte) int {
	for i := 0; i+len(sub) <= len(b); i++ {
		if string(b[i:i+len(sub)]) == string(sub) {
			return i
		}
	}
	return -1
}
