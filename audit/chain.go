package audit

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
)

// domain pins the hashing scheme to this format version. It is mixed into
// every entry hash so a hash from a different protocol (or a future, changed
// GoLogX format) can never be mistaken for a valid v1 entry hash.
const domain = "gologx-audit-v1"

// hashLen is the SHA-256 digest length in bytes.
const hashLen = sha256.Size

// Entry is one record in the append-only audit log, serialized as a single
// JSON object per line (JSONL). Every field except Hash and Sig is folded into
// Hash; Sig (when present) covers Hash. So any change to seq, prev, time, or
// data is detected by recomputing Hash, and Hash itself cannot be re-forged
// without the signing key.
type Entry struct {
	// Seq is the 0-based position of this entry in its chain.
	Seq uint64 `json:"seq"`
	// Time is the record timestamp (RFC3339Nano). It is covered by Hash, so a
	// backdated entry is detectable.
	Time string `json:"time"`
	// Prev is the hex SHA-256 of the previous entry (genesis: 64 zeros).
	Prev string `json:"prev"`
	// Data is the logged payload, stored verbatim. Verify re-hashes these exact
	// bytes, so no canonical re-encoding is needed or trusted.
	Data json.RawMessage `json:"data"`
	// Hash is the hex SHA-256 over (domain, seq, prev, time, data).
	Hash string `json:"hash"`
	// Sig is the hex Ed25519 signature over the raw Hash bytes. Empty when the
	// chain is unsigned.
	Sig string `json:"sig,omitempty"`
}

// entryHash computes the SHA-256 of an entry from its covered fields using
// length-prefixed framing. Framing each variable-length field with its length
// makes the input unambiguous: no choice of field contents can produce the same
// byte stream as a different (seq, prev, time, data) tuple, so an attacker
// cannot shift field boundaries to collide two distinct entries.
//
// This is the single source of truth for hashing. The writer (Handler) and the
// reader (Verify) both call it, so they can never disagree on what a valid hash
// is.
func entryHash(seq uint64, prev [hashLen]byte, time string, data []byte) [hashLen]byte {
	h := sha256.New()
	writeField(h, []byte(domain))
	var seqBytes [8]byte
	binary.BigEndian.PutUint64(seqBytes[:], seq)
	writeField(h, seqBytes[:])
	writeField(h, prev[:])
	writeField(h, []byte(time))
	writeField(h, data)
	var out [hashLen]byte
	h.Sum(out[:0])
	return out
}

// fieldHasher is the subset of hash.Hash that writeField needs.
type fieldHasher interface{ Write([]byte) (int, error) }

// writeField feeds one length-prefixed field into the hasher: an 8-byte
// big-endian length followed by the raw bytes. Writing to a hash.Hash never
// errors, so the return is ignored deliberately.
func writeField(h fieldHasher, b []byte) {
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], uint64(len(b)))
	_, _ = h.Write(n[:])
	_, _ = h.Write(b)
}

// errBadHashLen is returned when a hex hash field does not decode to exactly 32
// bytes, which means the line is malformed or truncated.
var errBadHashLen = errors.New("hash is not 32 bytes")

// decodeHash decodes a hex-encoded 32-byte hash. It rejects any length other
// than 32 bytes so a short/long value can never be silently zero-padded into a
// valid-looking chain link.
func decodeHash(s string) ([hashLen]byte, error) {
	var out [hashLen]byte
	raw, err := hex.DecodeString(s)
	if err != nil {
		return out, err
	}
	if len(raw) != hashLen {
		return out, errBadHashLen
	}
	copy(out[:], raw)
	return out, nil
}
