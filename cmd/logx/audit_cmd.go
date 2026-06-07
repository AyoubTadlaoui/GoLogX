package main

import (
	"crypto/ed25519"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/AyoubTadlaoui/GoLogX/audit"
)

// keygenCmd implements `logx keygen`: write an Ed25519 keypair for signing and
// verifying audit logs. The private key is written 0600; the public key 0644.
func keygenCmd(stdout, stderr io.Writer, argv []string) int {
	fs := flag.NewFlagSet("logx keygen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("out", "audit", "base name for the key files (writes <out>.key and <out>.pub)")
	force := fs.Bool("force", false, "overwrite existing key files")
	fs.Usage = func() {
		fmt.Fprintf(stderr, `logx keygen — generate an Ed25519 keypair for signing audit logs

Usage:
  logx keygen [-out audit] [-force]

Flags:
`)
		fs.PrintDefaults()
	}
	if err := fs.Parse(argv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	privPath := *out + ".key"
	pubPath := *out + ".pub"
	if !*force {
		for _, p := range []string{privPath, pubPath} {
			if _, err := os.Stat(p); err == nil {
				fmt.Fprintf(stderr, "logx: %s already exists (use -force to overwrite)\n", p)
				return 2
			}
		}
	}

	pub, priv, err := audit.GenerateKey()
	if err != nil {
		fmt.Fprintln(stderr, "logx:", err)
		return 1
	}
	privPEM, err := audit.MarshalPrivateKeyPEM(priv)
	if err != nil {
		fmt.Fprintln(stderr, "logx:", err)
		return 1
	}
	pubPEM, err := audit.MarshalPublicKeyPEM(pub)
	if err != nil {
		fmt.Fprintln(stderr, "logx:", err)
		return 1
	}
	if err := os.WriteFile(privPath, privPEM, 0o600); err != nil {
		fmt.Fprintln(stderr, "logx:", err)
		return 1
	}
	if err := os.WriteFile(pubPath, pubPEM, 0o644); err != nil {
		fmt.Fprintln(stderr, "logx:", err)
		return 1
	}
	fmt.Fprintf(stdout, "wrote %s (private key — keep it secret, off the logging host)\n", privPath)
	fmt.Fprintf(stdout, "wrote %s (public key — share it with whoever verifies the log)\n", pubPath)
	return 0
}

// verifyCmd implements `logx verify`: walk one or more hash-chained audit logs
// and report whether each is intact. Exit code is 0 when all files are intact,
// 1 when any file is tampered, and 2 when any file could not be read or a key
// could not be loaded. A detected tamper takes precedence so it is never masked
// by an unrelated read error on another file.
func verifyCmd(stdout, stderr io.Writer, argv []string) int {
	fs := flag.NewFlagSet("logx verify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	pubPath := fs.String("pubkey", "", "Ed25519 public key PEM; when set, every entry must carry a valid signature")
	quiet := fs.Bool("quiet", false, "print nothing, rely on the exit code (0 ok, 1 tampered, 2 error)")
	fs.Usage = func() {
		fmt.Fprintf(stderr, `logx verify — check the integrity of a hash-chained audit log

Usage:
  logx verify [-pubkey audit.pub] [-quiet] file ...

Exit codes:
  0  every file is intact
  1  a file was tampered with
  2  a file could not be read, or the key could not be loaded

Flags:
`)
		fs.PrintDefaults()
	}
	if err := fs.Parse(argv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	files := fs.Args()
	if len(files) == 0 {
		fmt.Fprintln(stderr, "logx: verify needs at least one file")
		fs.Usage()
		return 2
	}

	var pub ed25519.PublicKey
	if *pubPath != "" {
		data, err := os.ReadFile(*pubPath)
		if err != nil {
			fmt.Fprintln(stderr, "logx:", err)
			return 2
		}
		pub, err = audit.ParsePublicKeyPEM(data)
		if err != nil {
			fmt.Fprintln(stderr, "logx:", err)
			return 2
		}
	}

	sawTamper := false
	sawError := false
	for _, f := range files {
		rep, err := audit.VerifyFile(f, pub)
		if err != nil {
			if !*quiet {
				fmt.Fprintf(stderr, "logx: %s: %v\n", f, err)
			}
			sawError = true
			continue
		}
		if rep.OK {
			if !*quiet {
				fmt.Fprintf(stdout, "OK   %s — %d entries intact (%s)\n", f, rep.Entries, modeLabel(pub, rep))
			}
			continue
		}
		sawTamper = true
		if !*quiet {
			fmt.Fprintf(stdout, "FAIL %s — tampering detected at entry %d: %s\n", f, rep.BadSeq, rep.Reason)
		}
	}

	switch {
	case sawTamper:
		return 1
	case sawError:
		return 2
	default:
		return 0
	}
}

// modeLabel describes how thoroughly a file was checked, so a chain-only pass is
// never mistaken for a signature-verified one.
func modeLabel(pub ed25519.PublicKey, rep *audit.Report) string {
	switch {
	case pub != nil:
		return "chain + signatures"
	case rep.Signed:
		return "chain only; signatures present but unchecked, pass -pubkey to verify them"
	default:
		return "chain only; log is unsigned"
	}
}
