// Package audit turns Go's log/slog into a tamper-evident audit trail.
//
// Every record written through Handler becomes an append-only entry in a
// hash chain: each entry's SHA-256 covers the one before it, so editing,
// deleting, reordering, or injecting a line breaks the chain at that exact
// point. With an Ed25519 signer the entries are also signed, so they cannot be
// re-forged into a clean-looking chain without the private key. Read a log back
// offline with Verify, which reports the first broken entry, if any.
//
// The whole package is built on the standard library alone (crypto/sha256,
// crypto/ed25519, crypto/rand, encoding/pem). The thing that proves your logs
// were not tampered with carries no third-party code in its own trust path.
//
//	h, _ := audit.NewHandler(file, &audit.Options{Signer: priv})
//	log := slog.New(h)
//	log.Info("package installed", "name", "left-pad", "version", "1.3.0")
//	// ... later, on another machine that only has the public key:
//	report, _ := audit.VerifyFile("audit.log", pub)
//	if !report.OK { /* entry report.BadSeq was tampered with */ }
//
// What it does not do: detect entries dropped from the very end of the file
// (the surviving prefix is itself a valid chain). Anchor the head hash
// externally if end-truncation is in your threat model.
package audit
