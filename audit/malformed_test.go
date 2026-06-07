package audit

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

// ---- Attack class (h): malformed structural fields. Every case must fail
// cleanly (OK=false), never panic, never return a transport error. ----

// editField unmarshals line idx, mutates it via fn, remarshals, and returns the
// whole file. fn gets a pointer to the Entry to corrupt one field.
func editField(t *testing.T, chain []byte, idx int, fn func(e *Entry)) []byte {
	t.Helper()
	ls := lines(chain)
	var e Entry
	if err := json.Unmarshal(ls[idx], &e); err != nil {
		t.Fatal(err)
	}
	fn(&e)
	repacked, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	ls[idx] = repacked
	return joinLines(ls)
}

func TestMalformed_PrevShort(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	bad := editField(t, chain, 1, func(e *Entry) { e.Prev = hex.EncodeToString(make([]byte, 16)) })
	requireBad(t, mustVerify(t, bad, nil), 1)
}

func TestMalformed_PrevLong(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	bad := editField(t, chain, 1, func(e *Entry) { e.Prev = hex.EncodeToString(make([]byte, 48)) })
	requireBad(t, mustVerify(t, bad, nil), 1)
}

func TestMalformed_PrevNonHex(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	bad := editField(t, chain, 1, func(e *Entry) { e.Prev = strings.Repeat("zz", 32) })
	requireBad(t, mustVerify(t, bad, nil), 1)
}

func TestMalformed_HashShort(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	bad := editField(t, chain, 0, func(e *Entry) { e.Hash = hex.EncodeToString(make([]byte, 16)) })
	requireBad(t, mustVerify(t, bad, nil), 0)
}

func TestMalformed_HashLong(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	bad := editField(t, chain, 0, func(e *Entry) { e.Hash = hex.EncodeToString(make([]byte, 64)) })
	requireBad(t, mustVerify(t, bad, nil), 0)
}

func TestMalformed_HashNonHex(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	bad := editField(t, chain, 0, func(e *Entry) { e.Hash = strings.Repeat("gg", 32) })
	requireBad(t, mustVerify(t, bad, nil), 0)
}

func TestMalformed_SeqGap(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	// Force entry 1 to claim seq 5; seq guard must fire.
	bad := editField(t, chain, 1, func(e *Entry) { e.Seq = 5 })
	requireBad(t, mustVerify(t, bad, nil), 5)
}

func TestMalformed_SeqResetToZero(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	bad := editField(t, chain, 1, func(e *Entry) { e.Seq = 0 })
	requireBad(t, mustVerify(t, bad, nil), 0)
}

func TestMalformed_SeqDuplicate(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	bad := editField(t, chain, 2, func(e *Entry) { e.Seq = 1 })
	requireBad(t, mustVerify(t, bad, nil), 1)
}

func TestMalformed_NonZeroGenesisPrev(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	nonZero := make([]byte, hashLen)
	nonZero[0] = 1
	bad := editField(t, chain, 0, func(e *Entry) { e.Prev = hex.EncodeToString(nonZero) })
	requireBad(t, mustVerify(t, bad, nil), 0)
}

func TestMalformed_NotJSON(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	ls := lines(chain)
	ls[1] = []byte(`{this is not valid json`)
	requireBad(t, mustVerify(t, joinLines(ls), nil), -1)
}

func TestMalformed_DataNotObject(t *testing.T) {
	// data is RawMessage; a non-object value is still hashed verbatim, so a
	// crafted line with data="garbage" can be internally consistent. But if we
	// just swap the data without fixing the hash, it must break.
	chain := craftChain(t, payloads3(), nil)
	bad := editField(t, chain, 1, func(e *Entry) { e.Data = json.RawMessage(`123`) })
	requireBad(t, mustVerify(t, bad, nil), 1)
}

func TestMalformed_MissingDataField(t *testing.T) {
	// Removing the data field entirely makes e.Data nil; the recomputed hash
	// over zero-length data will not match the stored hash.
	chain := craftChain(t, payloads3(), nil)
	ls := lines(chain)
	var e Entry
	if err := json.Unmarshal(ls[1], &e); err != nil {
		t.Fatal(err)
	}
	// Marshal without data by zeroing it; json keeps "data":null for RawMessage,
	// which decodes back to the 4 bytes "null" -> different from original.
	e.Data = nil
	repacked, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	ls[1] = repacked
	requireBad(t, mustVerify(t, joinLines(ls), nil), 1)
}

// TestMalformed_EmptyHashWithoutKey ensures an empty hash field (decodes to 0
// bytes) is rejected as malformed even in unsigned mode.
func TestMalformed_EmptyHashWithoutKey(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	bad := editField(t, chain, 0, func(e *Entry) { e.Hash = "" })
	requireBad(t, mustVerify(t, bad, nil), 0)
}

// TestMalformed_HugeSeqNoPanic confirms a near-uint64-max seq value does not
// panic the int64 conversion path; it is simply out of order.
func TestMalformed_HugeSeqNoPanic(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	bad := editField(t, chain, 1, func(e *Entry) { e.Seq = ^uint64(0) })
	rep := mustVerify(t, bad, nil)
	if rep.OK {
		t.Fatal("expected detection of a giant out-of-order seq")
	}
}

// TestMalformed_SeqKeyCorruptionNonGenesis renames the "seq" key of a
// NON-genesis entry. json then leaves e.Seq at its 0 default, which disagrees
// with the expected seq at that position, so the seq guard fires. This is the
// companion to TestProperty_GenesisSeqKeyFlipIsNoOp: the only place the rename
// is harmless is the genesis entry, and only because 0 == 0 there.
func TestMalformed_SeqKeyCorruptionNonGenesis(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	// Entry 1 legitimately carries "seq":1; rename its key so e.Seq becomes 0.
	bad := bytesReplaceOnce(t, chain, `"seq":1,`, `"xeq":1,`)
	rep := mustVerify(t, bad, nil)
	requireBad(t, rep, 0) // expected seq 1, got default 0
}

// TestMalformed_TimeKeyCorruption renames the "time" key so e.Time becomes "".
// The stored hash was computed over the real time, so the recompute mismatches.
func TestMalformed_TimeKeyCorruption(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	bad := bytesReplaceOnce(t, chain, `"time":`, `"xime":`)
	requireBad(t, mustVerify(t, bad, nil), 0)
}

// TestMalformed_PrevKeyCorruption renames the "prev" key so e.Prev becomes "",
// which decodeHash rejects as not-32-bytes.
func TestMalformed_PrevKeyCorruption(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	bad := bytesReplaceOnce(t, chain, `"prev":`, `"xrev":`)
	requireBad(t, mustVerify(t, bad, nil), 0)
}

// TestMalformed_HashKeyCorruption renames the "hash" key so e.Hash becomes "",
// which decodeHash rejects.
func TestMalformed_HashKeyCorruption(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	bad := bytesReplaceOnce(t, chain, `"hash":`, `"xash":`)
	requireBad(t, mustVerify(t, bad, nil), 0)
}

// TestMalformed_DataKeyCorruption renames the "data" key so e.Data becomes nil,
// hashing to a different value than stored.
func TestMalformed_DataKeyCorruption(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	bad := bytesReplaceOnce(t, chain, `"data":`, `"xata":`)
	requireBad(t, mustVerify(t, bad, nil), 0)
}
