package audit

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"testing"
)

// keypairIf returns a fresh keypair when signed is true, or (nil, nil) for the
// unsigned case, so a single loop can drive both modes.
func keypairIf(t *testing.T, signed bool) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	if !signed {
		return nil, nil
	}
	pub, priv, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

// ---- Attack class (g): signed-mode attacks. ----

// TestSigned_StripSig_Detected removes the signature but keeps a valid hash.
// When a public key is supplied, an unsigned entry must be rejected: the
// verifier was told to require authenticity.
func TestSigned_StripSig_Detected(t *testing.T) {
	pub, priv := keypairIf(t, true)
	chain := craftChain(t, payloads3(), priv)
	ls := lines(chain)

	var e Entry
	if err := json.Unmarshal(ls[1], &e); err != nil {
		t.Fatal(err)
	}
	e.Sig = "" // strip signature, hash stays valid
	repacked, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	ls[1] = repacked
	requireBad(t, mustVerify(t, joinLines(ls), pub), 1)
}

// TestSigned_TamperAndResignWrongKey_Detected tampers data, recomputes a valid
// hash for the new data, and signs with a DIFFERENT key. Verified against the
// REAL public key it must fail: the signature is over the right bytes but by the
// wrong signer.
func TestSigned_TamperAndResignWrongKey_Detected(t *testing.T) {
	pub, priv := keypairIf(t, true)
	chain := craftChain(t, payloads3(), priv)
	ls := lines(chain)

	// Rebuild entry 1 with tampered data, correct prev, correct new hash, but
	// signed by an attacker-controlled key.
	_, attackerKey := keypairIf(t, true)
	sum0 := entryHash(0, [hashLen]byte{}, fixedTime, payloads3()[0])
	tamperedData := json.RawMessage(`{"level":"WARN","msg":"second","name":"evil-pad"}`)
	forged, _ := craftEntry(t, 1, sum0, fixedTime, tamperedData, attackerKey)
	ls[1] = forged
	requireBad(t, mustVerify(t, joinLines(ls), pub), 1)
}

// TestSigned_TamperHashKeepOldSig_Detected tampers data and rewrites e.Hash to
// match the tampered data (so the recompute passes), but leaves the ORIGINAL
// signature in place. The old sig is over the old hash, so it cannot verify
// against the new hash. This is the classic "fix the hash, forget the sig" miss.
func TestSigned_TamperHashKeepOldSig_Detected(t *testing.T) {
	pub, priv := keypairIf(t, true)
	chain := craftChain(t, payloads3(), priv)
	ls := lines(chain)

	var e Entry
	if err := json.Unmarshal(ls[1], &e); err != nil {
		t.Fatal(err)
	}
	oldSig := e.Sig
	// Tamper data and recompute a matching hash so the hash check would pass.
	sum0 := entryHash(0, [hashLen]byte{}, fixedTime, payloads3()[0])
	e.Data = json.RawMessage(`{"level":"WARN","msg":"second","name":"evil-pad"}`)
	newSum := entryHash(1, sum0, e.Time, e.Data)
	e.Hash = hex.EncodeToString(newSum[:])
	e.Sig = oldSig // stale signature over the OLD hash
	repacked, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	ls[1] = repacked
	requireBad(t, mustVerify(t, joinLines(ls), pub), 1)
}

// TestSigned_MalformedSig_Detected feeds a range of broken signatures: short,
// long, non-hex, and odd-length. None may verify and none may panic.
func TestSigned_MalformedSig_Detected(t *testing.T) {
	pub, priv := keypairIf(t, true)
	chain := craftChain(t, payloads3(), priv)
	ls := lines(chain)

	cases := map[string]string{
		"short":      hex.EncodeToString(make([]byte, 10)),
		"long":       hex.EncodeToString(make([]byte, 100)),
		"nonhex":     "zzzznothexzzzz",
		"oddlen":     "abc",
		"empty-hash": "", // empty sig with pub set -> "not signed"
	}
	for name, sig := range cases {
		t.Run(name, func(t *testing.T) {
			work := append([][]byte(nil), ls...)
			var e Entry
			if err := json.Unmarshal(work[1], &e); err != nil {
				t.Fatal(err)
			}
			e.Sig = sig
			repacked, err := json.Marshal(e)
			if err != nil {
				t.Fatal(err)
			}
			work[1] = repacked
			requireBad(t, mustVerify(t, joinLines(work), pub), 1)
		})
	}
}

// TestSigned_VerifyAgainstWrongKey_Detected verifies a perfectly intact signed
// chain against an UNRELATED public key. Every signature must fail at entry 0.
func TestSigned_VerifyAgainstWrongKey_Detected(t *testing.T) {
	_, priv := keypairIf(t, true)
	chain := craftChain(t, payloads3(), priv)
	wrongPub, _ := keypairIf(t, true)
	requireBad(t, mustVerify(t, chain, wrongPub), 0)
}

// TestSigned_VerifyWithoutKey_StillChecksHashChain confirms that verifying a
// signed chain WITHOUT a key still validates the hash chain (and reports it as
// signed), so dropping the key does not silently disable integrity checking.
func TestSigned_VerifyWithoutKey_StillChecksHashChain(t *testing.T) {
	_, priv := keypairIf(t, true)
	chain := craftChain(t, payloads3(), priv)
	rep := mustVerify(t, chain, nil)
	requireOK(t, rep)
	if !rep.Signed {
		t.Fatal("expected report to flag the chain as signed even without a key")
	}
	// And a data edit is still caught in keyless mode on a signed chain.
	tampered := jsonReplace(t, chain, 1, `{"level":"WARN","msg":"second","name":"evil"}`)
	requireBad(t, mustVerify(t, tampered, nil), 1)
}

// jsonReplace rewrites the data field of line idx with newData, leaving hash and
// sig untouched (a naive tamper), and returns the reassembled file.
func jsonReplace(t *testing.T, chain []byte, idx int, newData string) []byte {
	t.Helper()
	ls := lines(chain)
	var e Entry
	if err := json.Unmarshal(ls[idx], &e); err != nil {
		t.Fatal(err)
	}
	e.Data = json.RawMessage(newData)
	repacked, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	ls[idx] = repacked
	return joinLines(ls)
}
