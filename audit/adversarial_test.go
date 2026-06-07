package audit

import (
	"bytes"
	"encoding/json"
	"testing"
)

// payloads3 is a small legitimate three-record body used across edit/reorder/
// delete attacks. Each is a JSON object exactly like the handler emits.
func payloads3() []json.RawMessage {
	return []json.RawMessage{
		json.RawMessage(`{"level":"INFO","msg":"first","n":1}`),
		json.RawMessage(`{"level":"WARN","msg":"second","name":"left-pad"}`),
		json.RawMessage(`{"level":"ERROR","msg":"third"}`),
	}
}

// ---- Attack class (a): edit a covered field in a line. ----

// TestEditMsg_Detected edits the message text inside entry 1's data.
func TestEditMsg_Detected(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	tampered := bytes.Replace(chain, []byte(`"second"`), []byte(`"forged"`), 1)
	requireBad(t, mustVerify(t, tampered, nil), 1)
}

// TestEditAttrValue_Detected edits an attribute value inside entry 1's data.
func TestEditAttrValue_Detected(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	tampered := bytes.Replace(chain, []byte(`"left-pad"`), []byte(`"evil-pad"`), 1)
	requireBad(t, mustVerify(t, tampered, nil), 1)
}

// TestEditLevel_Detected downgrades an ERROR to INFO inside entry 2's data.
func TestEditLevel_Detected(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	tampered := bytes.Replace(chain, []byte(`"ERROR"`), []byte(`"INFO"`), 1)
	requireBad(t, mustVerify(t, tampered, nil), 2)
}

// TestEditTime_Detected backdates entry 1 by editing the covered time field.
func TestEditTime_Detected(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	ls := lines(chain)
	var e Entry
	if err := json.Unmarshal(ls[1], &e); err != nil {
		t.Fatal(err)
	}
	e.Time = "1999-01-01T00:00:00Z" // backdate but keep the old hash
	repacked, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	ls[1] = repacked
	requireBad(t, mustVerify(t, joinLines(ls), nil), 1)
}

// ---- Attack class (b): deletions. ----

// TestDeleteMiddleLine_Detected drops entry 1; entry 2's prev no longer links.
func TestDeleteMiddleLine_Detected(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	ls := lines(chain)
	kept := [][]byte{ls[0], ls[2]} // drop entry 1
	rep := mustVerify(t, joinLines(kept), nil)
	// After dropping seq 1, the next line is seq 2 where seq 1 was expected:
	// the seq-order guard catches it first.
	requireBad(t, rep, 2)
}

// TestDeleteFirstLine_Detected drops the genesis entry; the new first line has
// seq 1 where seq 0 was expected, and a non-genesis prev.
func TestDeleteFirstLine_Detected(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	ls := lines(chain)
	rep := mustVerify(t, joinLines(ls[1:]), nil)
	requireBad(t, rep, 1)
}

// ---- Attack class (c): reorder adjacent lines. ----

// TestReorderAdjacent_Detected swaps entries 0 and 1.
func TestReorderAdjacent_Detected(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	ls := lines(chain)
	ls[0], ls[1] = ls[1], ls[0]
	rep := mustVerify(t, joinLines(ls), nil)
	// First line is now seq 1 where seq 0 was expected: seq guard fires at 0th
	// position reporting the offending seq.
	requireBad(t, rep, 1)
}

// ---- Attack class (d): insert an attacker-crafted line in the middle. ----

// TestInsertCraftedLine_Unsigned_Detected inserts a fully valid-looking entry
// (correct hash over its own data, prev pointing at entry 0) between entries 0
// and 1. The attacker can compute everything for the inserted line from public
// data, but the FOLLOWING genuine entry still carries the old seq/prev and so
// breaks. This proves an unsigned chain detects single insertions.
func TestInsertCraftedLine_Unsigned_Detected(t *testing.T) {
	pls := payloads3()
	chain := craftChain(t, pls, nil)
	ls := lines(chain)

	// Recompute prev = hash of entry 0 (public data).
	sum0 := entryHash(0, [hashLen]byte{}, fixedTime, pls[0])
	injected, _ := craftEntry(t, 1, sum0, fixedTime, json.RawMessage(`{"level":"INFO","msg":"injected by attacker"}`), nil)

	// Splice: [e0, injected(seq1), e1(seq1), e2(seq2)].
	spliced := [][]byte{ls[0], injected, ls[1], ls[2]}
	rep := mustVerify(t, joinLines(spliced), nil)
	// e0 ok; injected seq1 ok (valid hash/prev); then the genuine old e1 also
	// claims seq 1 -> seq guard fires. Detection is guaranteed at seq 1.
	requireBad(t, rep, 1)
}

// ---- Attack class (e): full-tail rewrite from entry k. ----

// TestFullTailRewrite_Unsigned_DOCUMENTED_PASSES rewrites the entire chain from
// entry 1 onward, recomputing every subsequent hash and prev. Because the whole
// surviving structure is internally consistent, an UNSIGNED Verify accepts it.
// This is the DOCUMENTED unsigned limitation, NOT a P0: with no signature, a
// holder of the whole file can re-author it end to end. The test asserts the
// known-accepted behavior so any future tightening is noticed.
func TestFullTailRewrite_Unsigned_DOCUMENTED_PASSES(t *testing.T) {
	pls := payloads3()
	chain := craftChain(t, pls, nil)

	// Keep entry 0, then re-author 1..2 with attacker content.
	sum0 := entryHash(0, [hashLen]byte{}, fixedTime, pls[0])
	ls := lines(chain)

	prev := sum0
	forged := [][]byte{ls[0]}
	attackerData := []json.RawMessage{
		json.RawMessage(`{"level":"INFO","msg":"attacker rewrote this","amount":1000000}`),
		json.RawMessage(`{"level":"INFO","msg":"and this too"}`),
	}
	for i, d := range attackerData {
		line, sum := craftEntry(t, uint64(i+1), prev, fixedTime, d, nil)
		forged = append(forged, line)
		prev = sum
	}
	rep := mustVerify(t, joinLines(forged), nil)
	requireOK(t, rep) // documented: unsigned cannot resist a full-file rewrite
	if rep.Entries != 3 {
		t.Fatalf("expected 3 entries in the rewritten chain, got %d", rep.Entries)
	}
}

// TestFullTailRewrite_Signed_Detected runs the same end-to-end rewrite on a
// SIGNED chain and asserts Verify now FAILS, because the attacker cannot produce
// a valid Ed25519 signature without the private key. This pair is the proof
// that signing earns its place.
func TestFullTailRewrite_Signed_Detected(t *testing.T) {
	pub, priv, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	pls := payloads3()
	chain := craftChain(t, pls, priv)
	ls := lines(chain)

	// Attacker has no private key. They forge entries 1..2 honestly hashed but
	// sign with a DIFFERENT key they control (the realistic best case).
	_, attackerKey, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	sum0 := entryHash(0, [hashLen]byte{}, fixedTime, pls[0])
	prev := sum0
	forged := [][]byte{ls[0]}
	attackerData := []json.RawMessage{
		json.RawMessage(`{"level":"INFO","msg":"attacker rewrote this","amount":1000000}`),
		json.RawMessage(`{"level":"INFO","msg":"and this too"}`),
	}
	for i, d := range attackerData {
		line, sum := craftEntry(t, uint64(i+1), prev, fixedTime, d, attackerKey)
		forged = append(forged, line)
		prev = sum
	}
	rep := mustVerify(t, joinLines(forged), pub)
	requireBad(t, rep, 1) // first forged entry's sig fails against the real pubkey
}

// ---- Attack class (f): truncate the tail. ----

// TestTruncateTail_DOCUMENTED_PASSES drops the last entry. The surviving prefix
// is itself a valid chain, so Verify accepts it. This is the documented
// not-detectable limitation; anchor the head hash externally if end-truncation
// is in scope. Asserted here so the behavior is pinned, not silently relied on.
func TestTruncateTail_DOCUMENTED_PASSES(t *testing.T) {
	for _, signed := range []bool{false, true} {
		pub, priv := keypairIf(t, signed)
		chain := craftChain(t, payloads3(), priv)
		ls := lines(chain)
		truncated := joinLines(ls[:len(ls)-1]) // drop last entry
		rep := mustVerify(t, truncated, pub)
		requireOK(t, rep)
		if rep.Entries != 2 {
			t.Fatalf("signed=%v expected 2 surviving entries, got %d", signed, rep.Entries)
		}
	}
}
