package audit

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Report is the outcome of verifying a chain. A tampered or corrupt file is a
// successful verification with OK == false and BadSeq pointing at the first
// broken entry; it is not a Go error. Errors are reserved for failing to read
// the stream at all.
type Report struct {
	// Entries is the count of entries checked before any failure (all of them
	// when OK).
	Entries int
	// Signed is true if the checked entries carried signatures.
	Signed bool
	// OK is true only if the whole chain is intact (and, when a key was given,
	// every signature verified).
	OK bool
	// BadSeq is the seq of the first broken entry, or -1 when OK.
	BadSeq int64
	// Reason explains the failure in one line, empty when OK.
	Reason string
}

// maxLine bounds a single JSONL entry so a corrupt file with no newlines cannot
// exhaust memory. It matches the CLI's existing scanner ceiling.
const maxLine = 4 << 20 // 4 MiB

// Verify walks a hash-chained log from r and reports whether it is intact.
//
// When pub is non-nil, every entry must carry a signature that verifies against
// pub; this proves both integrity and authenticity. When pub is nil, only the
// hash chain is checked: that still detects any edit, deletion, reorder, or
// inserted line, but a holder of the whole file could in principle rewrite it
// end to end, which signatures prevent.
//
// Note that neither mode detects entries dropped from the very end of the file:
// the surviving prefix is itself a valid chain. Anchor the head hash externally
// if end-truncation is part of your threat model.
func Verify(r io.Reader, pub ed25519.PublicKey) (*Report, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), maxLine)

	var prev [hashLen]byte // genesis: all zero
	var expectSeq uint64
	entries := 0
	signed := false
	lineNo := 0

	for sc.Scan() {
		lineNo++
		raw := bytes.TrimSpace(sc.Bytes())
		if len(raw) == 0 {
			continue
		}

		var e Entry
		if err := json.Unmarshal(raw, &e); err != nil {
			return fail(int64Of(expectSeq), fmt.Sprintf("line %d is not a valid entry: %v", lineNo, err)), nil
		}

		if e.Seq != expectSeq {
			return fail(int64Of(e.Seq), fmt.Sprintf("sequence out of order at line %d: expected seq %d, got %d", lineNo, expectSeq, e.Seq)), nil
		}

		prevGot, err := decodeHash(e.Prev)
		if err != nil {
			return fail(int64Of(e.Seq), fmt.Sprintf("entry %d has a malformed prev hash: %v", e.Seq, err)), nil
		}
		if prevGot != prev {
			return fail(int64Of(e.Seq), fmt.Sprintf("broken chain at entry %d: prev hash does not match the previous entry (a line was edited, deleted, reordered, or inserted)", e.Seq)), nil
		}

		sum := entryHash(e.Seq, prev, e.Time, e.Data)
		hashGot, err := decodeHash(e.Hash)
		if err != nil {
			return fail(int64Of(e.Seq), fmt.Sprintf("entry %d has a malformed hash: %v", e.Seq, err)), nil
		}
		if sum != hashGot {
			return fail(int64Of(e.Seq), fmt.Sprintf("entry %d was modified: recomputed hash does not match", e.Seq)), nil
		}

		if pub != nil {
			if e.Sig == "" {
				return fail(int64Of(e.Seq), fmt.Sprintf("entry %d is not signed but a public key was provided", e.Seq)), nil
			}
			sig, err := hex.DecodeString(e.Sig)
			if err != nil || len(sig) != ed25519.SignatureSize {
				return fail(int64Of(e.Seq), fmt.Sprintf("entry %d has a malformed signature", e.Seq)), nil
			}
			if !ed25519.Verify(pub, sum[:], sig) {
				return fail(int64Of(e.Seq), fmt.Sprintf("entry %d signature does not verify against the given key", e.Seq)), nil
			}
			signed = true
		} else if e.Sig != "" {
			signed = true
		}

		prev = sum
		expectSeq++
		entries++
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	return &Report{Entries: entries, Signed: signed, OK: true, BadSeq: -1}, nil
}

// VerifyFile is Verify over a file path.
func VerifyFile(path string, pub ed25519.PublicKey) (*Report, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Verify(f, pub)
}

func fail(badSeq int64, reason string) *Report {
	return &Report{OK: false, BadSeq: badSeq, Reason: reason}
}

// int64Of converts a seq to int64 for Report.BadSeq. Audit chains never
// approach the int64 ceiling in practice.
func int64Of(seq uint64) int64 { return int64(seq) }
