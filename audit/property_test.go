package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"math/rand"
	"testing"
	"time"
)

// randRecord builds a slog.Record with a random mix of attribute kinds:
// string, int, uint, float, bool, duration, time, a group, and a nested group.
// This exercises every branch of safeValue and flatten in the writer.
func randRecord(rng *rand.Rand) slog.Record {
	levels := []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError}
	r := slog.NewRecord(
		time.Unix(rng.Int63n(2_000_000_000), rng.Int63n(1_000_000_000)).UTC(),
		levels[rng.Intn(len(levels))],
		randString(rng, 1+rng.Intn(20)),
		0,
	)
	n := rng.Intn(6)
	for i := 0; i < n; i++ {
		r.AddAttrs(randAttr(rng, 0))
	}
	return r
}

func randAttr(rng *rand.Rand, depth int) slog.Attr {
	kind := rng.Intn(9)
	if depth >= 2 && kind == 8 {
		kind = 0 // cap nesting
	}
	key := randString(rng, 1+rng.Intn(8))
	switch kind {
	case 0:
		return slog.String(key, randString(rng, rng.Intn(24)))
	case 1:
		return slog.Int64(key, rng.Int63()-rng.Int63())
	case 2:
		return slog.Uint64(key, rng.Uint64())
	case 3:
		return slog.Float64(key, rng.NormFloat64())
	case 4:
		return slog.Bool(key, rng.Intn(2) == 0)
	case 5:
		return slog.Duration(key, time.Duration(rng.Int63n(int64(72*time.Hour))))
	case 6:
		return slog.Time(key, time.Unix(rng.Int63n(2_000_000_000), 0).UTC())
	case 7:
		// flat group
		return slog.Group(key, slog.String("g1", randString(rng, 5)), slog.Int("g2", rng.Intn(100)))
	default:
		// nested group
		return slog.Group(key,
			randAttr(rng, depth+1),
			slog.Group("inner", randAttr(rng, depth+1), slog.Bool("flag", true)),
		)
	}
}

const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 ._-/\t\"\\"

func randString(rng *rand.Rand, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[rng.Intn(len(alphabet))]
	}
	return string(b)
}

// TestProperty_RandomRoundTrip writes N random records (signed and unsigned)
// and asserts the resulting chain always verifies OK with the right count. It
// covers every attr kind including odd characters in keys/values.
func TestProperty_RandomRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, signed := range []bool{false, true} {
		for trial := 0; trial < 30; trial++ {
			pub, priv := keypairIf(t, signed)
			var buf bytes.Buffer
			h, err := NewHandler(&buf, &Options{Signer: priv})
			if err != nil {
				t.Fatal(err)
			}
			n := 1 + rng.Intn(40)
			for i := 0; i < n; i++ {
				if err := h.Handle(context.Background(), randRecord(rng)); err != nil {
					t.Fatalf("Handle: %v", err)
				}
			}
			rep := mustVerify(t, buf.Bytes(), pub)
			if !rep.OK {
				t.Fatalf("signed=%v trial=%d random chain failed at seq %d: %s", signed, trial, rep.BadSeq, rep.Reason)
			}
			if rep.Entries != n {
				t.Fatalf("signed=%v trial=%d expected %d entries, got %d", signed, trial, n, rep.Entries)
			}
			if rep.Signed != signed {
				t.Fatalf("signed=%v trial=%d report.Signed=%v", signed, trial, rep.Signed)
			}
		}
	}
}

// TestProperty_SingleByteFlipBreaksChain flips one byte at every position of a
// real chain file and asserts Verify fails -- UNLESS the flipped byte is pure
// structural whitespace between entries (a newline) or lands inside the part of
// a line that is trimmed/cosmetic, which the chain legitimately tolerates. The
// test classifies each flip: any flip that lands on a covered byte (inside the
// JSON of an entry) MUST be detected. This is the strongest single-mutation
// guarantee: no covered byte can change without Verify noticing.
func TestProperty_SingleByteFlipBreaksChain(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for _, signed := range []bool{false, true} {
		pub, priv := keypairIf(t, signed)
		var buf bytes.Buffer
		h, err := NewHandler(&buf, &Options{Signer: priv})
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 5; i++ {
			if err := h.Handle(context.Background(), randRecord(rng)); err != nil {
				t.Fatal(err)
			}
		}
		base := buf.Bytes()

		// genesisSeqKey marks the byte range of the literal "seq" KEY token in the
		// first entry. Corrupting those 3 letters renames the key, so json leaves
		// e.Seq at its default of 0 -- which equals the genesis entry's real seq.
		// That single class of flip is a provable semantic NO-OP (the covered
		// value is unchanged), so Verify legitimately stays OK. Every OTHER flip
		// changes a covered byte and MUST be caught. We compute the allowed range
		// precisely so the test still fails hard on any genuine bypass elsewhere.
		firstLineEnd := bytes.IndexByte(base, '\n')
		if firstLineEnd < 0 {
			t.Fatal("no newline in chain")
		}
		keyIdx := bytes.Index(base[:firstLineEnd], []byte(`"seq"`))
		if keyIdx < 0 {
			t.Fatal("could not locate the seq key in the first entry")
		}
		// The three key letters s,e,q sit at keyIdx+1 .. keyIdx+3 (inside quotes).
		allowedLo, allowedHi := keyIdx+1, keyIdx+3

		missed := 0
		for pos := 0; pos < len(base); pos++ {
			orig := base[pos]
			if orig == '\n' {
				// Flipping the inter-entry newline only reshapes line splitting;
				// not a covered byte. Skip -- handled by shape tests.
				continue
			}
			mutated := append([]byte(nil), base...)
			mutated[pos] ^= 0x01 // flip the low bit: always changes the byte, stays printable-ish

			rep, err := Verify(bytes.NewReader(mutated), pub)
			if err != nil {
				// A transport error (e.g. mutated into an enormous token) is not a
				// false OK; acceptable.
				continue
			}
			if rep.OK {
				// Survivors are only acceptable inside the genesis "seq" key token,
				// where the flip is a proven no-op. Anything else is a real miss.
				if pos >= allowedLo && pos <= allowedHi {
					continue
				}
				missed++
				t.Errorf("signed=%v: flipping byte at pos %d (0x%02x->0x%02x) was NOT detected; surrounding bytes: %q",
					signed, pos, orig, mutated[pos], excerpt(base, pos))
			}
		}
		if missed > 0 {
			t.Fatalf("signed=%v: %d single-byte flips on COVERED bytes slipped past Verify (see errors above) -- this is a P0 bypass", signed, missed)
		}
	}
}

// TestProperty_GenesisSeqKeyFlipIsNoOp pins the one benign single-byte survivor:
// renaming the genesis entry's "seq" key leaves e.Seq at 0, which equals the
// real genesis seq, so the chain is unchanged in every covered respect. This is
// documented as a non-finding (an attacker gains nothing: they cannot set seq 0
// to a different value, reorder, or alter any covered field). For ANY non-genesis
// entry the same trick is caught because the default 0 disagrees with the
// expected seq, as TestMalformed_SeqKeyCorruptionNonGenesis proves.
func TestProperty_GenesisSeqKeyFlipIsNoOp(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	mutated := bytes.Replace(chain, []byte(`{"seq":0,`), []byte(`{"req":0,`), 1)
	if bytes.Equal(mutated, chain) {
		t.Fatal("setup: genesis seq key not found")
	}
	rep := mustVerify(t, mutated, nil)
	requireOK(t, rep) // benign no-op: covered values identical
	if rep.Entries != 3 {
		t.Fatalf("expected 3 entries, got %d", rep.Entries)
	}
}

// excerpt returns a short window around pos for diagnostics.
func excerpt(b []byte, pos int) string {
	lo := pos - 8
	if lo < 0 {
		lo = 0
	}
	hi := pos + 8
	if hi > len(b) {
		hi = len(b)
	}
	return string(b[lo:hi])
}

// TestProperty_HashFieldIsRawByteHash directly cross-checks that the stored hash
// equals an independent recomputation over the verbatim data bytes, for a random
// chain. This pins the "hash covers raw bytes, not canonicalized JSON" contract.
func TestProperty_HashFieldIsRawByteHash(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	var buf bytes.Buffer
	h, err := NewHandler(&buf, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		if err := h.Handle(context.Background(), randRecord(rng)); err != nil {
			t.Fatal(err)
		}
	}
	var prev [hashLen]byte
	for i, l := range lines(buf.Bytes()) {
		var e Entry
		if err := json.Unmarshal(l, &e); err != nil {
			t.Fatal(err)
		}
		want := entryHash(e.Seq, prev, e.Time, e.Data)
		got, derr := decodeHash(e.Hash)
		if derr != nil {
			t.Fatalf("entry %d hash decode: %v", i, derr)
		}
		if want != got {
			t.Fatalf("entry %d stored hash does not match raw-byte recompute", i)
		}
		prev = want
	}
}
