package audit

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"testing"
	"time"
)

// This file holds shared helpers for the adversarial suite. The attack tests
// live in adversarial_test.go, the signed-mode tests in signed_test.go, the
// structural/malformed probes in malformed_test.go, the canonicalization and
// I/O-shape probes in canonical_test.go, and the property/round-trip tests in
// property_test.go. Helpers here let a test hand-build either a legitimate or a
// fully attacker-forged chain at the byte level, because the only honest way to
// prove tamper-evidence is to compute the exact bytes a sophisticated attacker
// could compute from public data and confirm Verify still refuses them.

// fixedTime gives every crafted entry a deterministic timestamp so tests are
// reproducible and the framing inputs are stable.
const fixedTime = "2026-06-07T00:00:00Z"

// craftEntry builds one Entry exactly the way the honest writer would: it
// computes the chain hash over (domain, seq, prev, time, data) and, if signer
// is non-nil, signs the raw hash. This is the attacker's toolkit: everything
// here is computable from public data alone (the data, the prev hash, the seq,
// the time), which is the whole point of attack class (d) and (e).
func craftEntry(t *testing.T, seq uint64, prev [hashLen]byte, ts string, data json.RawMessage, signer ed25519.PrivateKey) ([]byte, [hashLen]byte) {
	t.Helper()
	sum := entryHash(seq, prev, ts, data)
	e := Entry{
		Seq:  seq,
		Time: ts,
		Prev: hex.EncodeToString(prev[:]),
		Data: data,
		Hash: hex.EncodeToString(sum[:]),
	}
	if signer != nil {
		e.Sig = hex.EncodeToString(ed25519.Sign(signer, sum[:]))
	}
	line, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("craftEntry marshal: %v", err)
	}
	return line, sum
}

// craftChain builds a complete legitimate chain from a list of data payloads,
// signing with signer when non-nil. It returns the full file bytes (one JSONL
// line per entry, trailing newline on each).
func craftChain(t *testing.T, payloads []json.RawMessage, signer ed25519.PrivateKey) []byte {
	t.Helper()
	var buf bytes.Buffer
	var prev [hashLen]byte
	for i, d := range payloads {
		line, sum := craftEntry(t, uint64(i), prev, fixedTime, d, signer)
		buf.Write(line)
		buf.WriteByte('\n')
		prev = sum
	}
	return buf.Bytes()
}

// writeChain runs n records through a real Handler and returns the file bytes.
// It exercises the production write path (slog -> Handle -> JSONL) rather than
// the hand-built path, so the positive cases test what real callers produce.
func writeChain(t *testing.T, signer ed25519.PrivateKey, recs ...slog.Record) []byte {
	t.Helper()
	var buf bytes.Buffer
	h, err := NewHandler(&buf, &Options{Signer: signer})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	for _, r := range recs {
		if err := h.Handle(context.Background(), r); err != nil {
			t.Fatalf("Handle: %v", err)
		}
	}
	return buf.Bytes()
}

// recAt builds a record with an explicit timestamp and level so crafted-vs-real
// comparisons line up byte for byte where needed.
func recAt(ts time.Time, level slog.Level, msg string, attrs ...slog.Attr) slog.Record {
	r := slog.NewRecord(ts, level, msg, 0)
	r.AddAttrs(attrs...)
	return r
}

// lines splits a chain file into its non-final-newline JSONL lines (dropping a
// single trailing empty element from the terminal newline).
func lines(b []byte) [][]byte {
	parts := bytes.Split(b, []byte("\n"))
	if len(parts) > 0 && len(parts[len(parts)-1]) == 0 {
		parts = parts[:len(parts)-1]
	}
	return parts
}

// joinLines reassembles JSONL lines into a file with a trailing newline.
func joinLines(ls [][]byte) []byte {
	var buf bytes.Buffer
	for _, l := range ls {
		buf.Write(l)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// mustVerify runs Verify and fails the test on a transport error (Verify only
// returns a Go error for I/O problems, never for tamper detection).
func mustVerify(t *testing.T, b []byte, pub ed25519.PublicKey) *Report {
	t.Helper()
	rep, err := Verify(bytes.NewReader(b), pub)
	if err != nil {
		t.Fatalf("Verify returned a transport error (should only happen on I/O failure): %v", err)
	}
	if rep == nil {
		t.Fatal("Verify returned nil report with nil error")
	}
	return rep
}

// requireBad asserts Verify caught tampering at the expected seq.
func requireBad(t *testing.T, rep *Report, wantSeq int64) {
	t.Helper()
	if rep.OK {
		t.Fatalf("expected tamper to be DETECTED (OK=false) but Verify reported OK=true; this is a P0 bypass")
	}
	if wantSeq >= 0 && rep.BadSeq != wantSeq {
		t.Fatalf("detected, but at wrong seq: want BadSeq=%d, got %d (reason: %s)", wantSeq, rep.BadSeq, rep.Reason)
	}
}

// requireOK asserts Verify accepted the chain (used for legitimate chains and
// for documented limitations that the current design cannot catch).
func requireOK(t *testing.T, rep *Report) {
	t.Helper()
	if !rep.OK {
		t.Fatalf("expected OK=true, got OK=false at seq %d: %s", rep.BadSeq, rep.Reason)
	}
}

// bytesReplaceOnce replaces the first occurrence of old with new and fails the
// test if old was not present, so a refactor that changes the JSON layout
// surfaces as a test setup failure instead of a silent no-op pass.
func bytesReplaceOnce(t *testing.T, b []byte, old, new string) []byte {
	t.Helper()
	out := bytes.Replace(b, []byte(old), []byte(new), 1)
	if bytes.Equal(out, b) {
		t.Fatalf("test setup: substring %q not found in chain", old)
	}
	return out
}
