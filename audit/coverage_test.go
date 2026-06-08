package audit

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// This file targets the functions left thin by the rest of the suite: the PEM
// key marshal/parse round trips and their error paths (keys.go), VerifyFile's
// happy path and its real-error path (verify.go), WithGroup / Enabled / the
// non-trivial arms of safeValue (handler.go), and OpenFile's failure path
// (file.go). Helpers from the existing test files (rec, craftChain, payloads3,
// lines, joinLines, mustVerify, requireOK, requireBad) are reused; nothing here
// redeclares them.

// ---- keys.go: PEM marshal / parse round trip + error paths ----

// TestKeyPEM_RoundTripUsableForSignVerify generates a keypair, marshals both
// halves to PEM, parses them back, and proves the parsed keys are not just
// equal byte-for-byte but actually interoperate: a signature made with the
// parsed private key verifies under the parsed public key.
func TestKeyPEM_RoundTripUsableForSignVerify(t *testing.T) {
	pub, priv, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	privPEM, err := MarshalPrivateKeyPEM(priv)
	if err != nil {
		t.Fatalf("MarshalPrivateKeyPEM: %v", err)
	}
	if !bytes.Contains(privPEM, []byte(privPEMType)) {
		t.Fatalf("private PEM missing block type %q:\n%s", privPEMType, privPEM)
	}
	pubPEM, err := MarshalPublicKeyPEM(pub)
	if err != nil {
		t.Fatalf("MarshalPublicKeyPEM: %v", err)
	}
	if !bytes.Contains(pubPEM, []byte(pubPEMType)) {
		t.Fatalf("public PEM missing block type %q:\n%s", pubPEMType, pubPEM)
	}

	gotPriv, err := ParsePrivateKeyPEM(privPEM)
	if err != nil {
		t.Fatalf("ParsePrivateKeyPEM: %v", err)
	}
	if !bytes.Equal(gotPriv, priv) {
		t.Fatal("parsed private key does not equal the original")
	}
	gotPub, err := ParsePublicKeyPEM(pubPEM)
	if err != nil {
		t.Fatalf("ParsePublicKeyPEM: %v", err)
	}
	if !bytes.Equal(gotPub, pub) {
		t.Fatal("parsed public key does not equal the original")
	}

	// The real test of a round trip: sign with the parsed private key and verify
	// with the parsed public key. If either reconstruction were wrong this fails.
	msg := []byte("audit-entry-hash-stand-in")
	sig := ed25519.Sign(gotPriv, msg)
	if !ed25519.Verify(gotPub, msg, sig) {
		t.Fatal("signature from parsed private key did not verify under parsed public key")
	}
	// And the parsed public key must reject a signature over different bytes.
	if ed25519.Verify(gotPub, []byte("different"), sig) {
		t.Fatal("parsed public key verified a signature over the wrong message")
	}
}

// TestKeyPEM_ParseNoPEM feeds garbage with no PEM armor to both parsers; both
// must return the sentinel errNoPEM and a nil key.
func TestKeyPEM_ParseNoPEM(t *testing.T) {
	junk := []byte("this is not a pem block at all\n")

	priv, err := ParsePrivateKeyPEM(junk)
	if !errors.Is(err, errNoPEM) {
		t.Fatalf("ParsePrivateKeyPEM no-PEM: want errNoPEM, got %v", err)
	}
	if priv != nil {
		t.Fatal("ParsePrivateKeyPEM returned a non-nil key on no-PEM input")
	}

	pub, err := ParsePublicKeyPEM(junk)
	if !errors.Is(err, errNoPEM) {
		t.Fatalf("ParsePublicKeyPEM no-PEM: want errNoPEM, got %v", err)
	}
	if pub != nil {
		t.Fatal("ParsePublicKeyPEM returned a non-nil key on no-PEM input")
	}
}

// TestKeyPEM_ParseWrongBlockType hands each parser a validly armored PEM block
// that belongs to the OTHER half (public bytes under the private type and vice
// versa). The byte length happens to differ too, but the type guard fires first
// and the error must name the expected type.
func TestKeyPEM_ParseWrongBlockType(t *testing.T) {
	pub, priv, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	privPEM, err := MarshalPrivateKeyPEM(priv)
	if err != nil {
		t.Fatal(err)
	}
	pubPEM, err := MarshalPublicKeyPEM(pub)
	if err != nil {
		t.Fatal(err)
	}

	// Public-typed PEM into the private parser.
	if _, err := ParsePrivateKeyPEM(pubPEM); err == nil {
		t.Fatal("ParsePrivateKeyPEM accepted a public-typed PEM block")
	} else if !strings.Contains(err.Error(), privPEMType) {
		t.Fatalf("ParsePrivateKeyPEM wrong-type error should mention %q, got: %v", privPEMType, err)
	}

	// Private-typed PEM into the public parser.
	if _, err := ParsePublicKeyPEM(privPEM); err == nil {
		t.Fatal("ParsePublicKeyPEM accepted a private-typed PEM block")
	} else if !strings.Contains(err.Error(), pubPEMType) {
		t.Fatalf("ParsePublicKeyPEM wrong-type error should mention %q, got: %v", pubPEMType, err)
	}
}

// TestKeyPEM_ParseWrongLength builds correctly typed PEM blocks whose payload is
// the wrong number of bytes, so the length guards (not the type guards) reject
// them. This is the "truncated or padded seed/pubkey" corruption case.
func TestKeyPEM_ParseWrongLength(t *testing.T) {
	shortSeed := pemBlock(t, privPEMType, make([]byte, ed25519.SeedSize-1))
	if _, err := ParsePrivateKeyPEM(shortSeed); err == nil {
		t.Fatal("ParsePrivateKeyPEM accepted a too-short seed")
	} else if !strings.Contains(err.Error(), "seed") {
		t.Fatalf("ParsePrivateKeyPEM length error should mention the seed, got: %v", err)
	}

	longPub := pemBlock(t, pubPEMType, make([]byte, ed25519.PublicKeySize+5))
	if _, err := ParsePublicKeyPEM(longPub); err == nil {
		t.Fatal("ParsePublicKeyPEM accepted an over-length public key")
	} else if !strings.Contains(err.Error(), "public key") {
		t.Fatalf("ParsePublicKeyPEM length error should mention the public key, got: %v", err)
	}
}

// TestKeyPEM_MarshalWrongSize passes keys whose byte length is invalid to the
// marshalers; both must refuse rather than emit a malformed PEM block.
func TestKeyPEM_MarshalWrongSize(t *testing.T) {
	// A private key one byte short of ed25519.PrivateKeySize.
	badPriv := ed25519.PrivateKey(make([]byte, ed25519.PrivateKeySize-1))
	if out, err := MarshalPrivateKeyPEM(badPriv); err == nil {
		t.Fatalf("MarshalPrivateKeyPEM accepted a %d-byte key, output: %q", len(badPriv), out)
	} else if !strings.Contains(err.Error(), "private key") {
		t.Fatalf("MarshalPrivateKeyPEM error should mention the private key, got: %v", err)
	}

	// A public key one byte too long.
	badPub := ed25519.PublicKey(make([]byte, ed25519.PublicKeySize+1))
	if out, err := MarshalPublicKeyPEM(badPub); err == nil {
		t.Fatalf("MarshalPublicKeyPEM accepted a %d-byte key, output: %q", len(badPub), out)
	} else if !strings.Contains(err.Error(), "public key") {
		t.Fatalf("MarshalPublicKeyPEM error should mention the public key, got: %v", err)
	}
}

// ---- verify.go: VerifyFile happy path and real-error path ----

// TestVerifyFile_OK writes a genuine signed chain to a temp file, then verifies
// it through VerifyFile and asserts the report is fully OK with the expected
// entry count and signed flag.
func TestVerifyFile_OK(t *testing.T) {
	pub, priv, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	chain := craftChain(t, payloads3(), priv)

	path := filepath.Join(t.TempDir(), "chain.jsonl")
	if err := os.WriteFile(path, chain, 0o600); err != nil {
		t.Fatal(err)
	}

	rep, err := VerifyFile(path, pub)
	if err != nil {
		t.Fatalf("VerifyFile returned a Go error on a good file: %v", err)
	}
	requireOK(t, rep)
	if rep.Entries != len(payloads3()) {
		t.Fatalf("VerifyFile entries: want %d, got %d", len(payloads3()), rep.Entries)
	}
	if !rep.Signed {
		t.Fatal("VerifyFile should report a signed chain as Signed")
	}
}

// TestVerifyFile_MissingPath confirms that an unopenable path is a Go error,
// NOT a tamper report: the function must return (nil, err), keeping read
// failures distinct from integrity failures.
func TestVerifyFile_MissingPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.jsonl")

	rep, err := VerifyFile(missing, nil)
	if err == nil {
		t.Fatal("VerifyFile on a missing path should return a Go error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("VerifyFile missing-path error should be ErrNotExist, got: %v", err)
	}
	if rep != nil {
		t.Fatalf("VerifyFile should return a nil report alongside the error, got %+v", rep)
	}
}

// ---- handler.go: WithGroup, Enabled, safeValue ----

// TestWithGroup_DottedKeysAndVerifies logs through a grouped then nested-grouped
// logger and asserts the emitted data keys are dotted (outer.x, outer.inner.y),
// that an empty group name is a no-op, and that the whole chain still verifies.
func TestWithGroup_DottedKeysAndVerifies(t *testing.T) {
	pub, priv, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	h, err := NewHandler(&buf, &Options{Signer: priv})
	if err != nil {
		t.Fatal(err)
	}

	base := slog.New(h)
	// Empty group name must be a no-op (returns the same handler), so this attr
	// stays at the top level.
	base.WithGroup("").Info("flat", slog.String("top", "t"))
	// Single group: key becomes "outer.x".
	base.WithGroup("outer").Info("one", slog.Int("x", 7))
	// Nested groups: key becomes "outer.inner.y".
	base.WithGroup("outer").WithGroup("inner").Info("two", slog.String("y", "deep"))

	// The chain must verify against the public key.
	rep, err := Verify(bytes.NewReader(buf.Bytes()), pub)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	requireOK(t, rep)
	if rep.Entries != 3 {
		t.Fatalf("want 3 entries, got %d", rep.Entries)
	}

	datas := dataMaps(t, buf.Bytes())
	if got := datas[0]["top"]; got != "t" {
		t.Fatalf("entry 0 should carry top=t at top level, got %#v (keys: %v)", got, keysOf(datas[0]))
	}
	if _, dotted := datas[0]["outer.top"]; dotted {
		t.Fatal("empty WithGroup must not introduce a dotted prefix")
	}
	if got := datas[1]["outer.x"]; got != float64(7) {
		t.Fatalf("entry 1 should carry outer.x=7, got %#v (keys: %v)", got, keysOf(datas[1]))
	}
	if got := datas[2]["outer.inner.y"]; got != "deep" {
		t.Fatalf("entry 2 should carry outer.inner.y=deep, got %#v (keys: %v)", got, keysOf(datas[2]))
	}
}

// TestEnabled_LevelFiltering builds a handler with a minimum level of Warn and
// asserts that below-level records are dropped while at/above-level records are
// written, and that Enabled itself reports the same boundary.
func TestEnabled_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	h, err := NewHandler(&buf, &Options{Level: slog.LevelWarn})
	if err != nil {
		t.Fatal(err)
	}

	// Enabled should mirror the configured threshold exactly.
	ctx := context.Background()
	if h.Enabled(ctx, slog.LevelInfo) {
		t.Fatal("Enabled(Info) should be false when Level=Warn")
	}
	if h.Enabled(ctx, slog.LevelDebug) {
		t.Fatal("Enabled(Debug) should be false when Level=Warn")
	}
	if !h.Enabled(ctx, slog.LevelWarn) {
		t.Fatal("Enabled(Warn) should be true when Level=Warn")
	}
	if !h.Enabled(ctx, slog.LevelError) {
		t.Fatal("Enabled(Error) should be true when Level=Warn")
	}

	// Drive it through slog: Debug/Info are filtered, Warn/Error recorded.
	log := slog.New(h)
	log.Debug("dropped-debug")
	log.Info("dropped-info")
	log.Warn("kept-warn")
	log.Error("kept-error")

	rep, err := Verify(bytes.NewReader(buf.Bytes()), nil)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	requireOK(t, rep)
	if rep.Entries != 2 {
		t.Fatalf("expected exactly 2 entries (Warn,Error), got %d", rep.Entries)
	}

	datas := dataMaps(t, buf.Bytes())
	if datas[0]["msg"] != "kept-warn" || datas[0]["level"] != "WARN" {
		t.Fatalf("first kept entry should be the WARN record, got %#v", datas[0])
	}
	if datas[1]["msg"] != "kept-error" || datas[1]["level"] != "ERROR" {
		t.Fatalf("second kept entry should be the ERROR record, got %#v", datas[1])
	}
}

// TestEnabled_NilLevelRecordsEverything covers the nil-Level arm: with no
// configured level every record, even Debug, is enabled and recorded.
func TestEnabled_NilLevelRecordsEverything(t *testing.T) {
	var buf bytes.Buffer
	h, err := NewHandler(&buf, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !h.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("Enabled(Debug) should be true when no Level is configured")
	}
	log := slog.New(h)
	log.Debug("audit-keeps-everything")
	rep, err := Verify(bytes.NewReader(buf.Bytes()), nil)
	if err != nil {
		t.Fatal(err)
	}
	requireOK(t, rep)
	if rep.Entries != 1 {
		t.Fatalf("nil Level should record the Debug entry, got %d entries", rep.Entries)
	}
}

// TestSafeValue_AllKindsRenderAndVerify exercises every non-string arm of
// safeValue in one record: Duration, Time, Bool, Uint64, Float64, a nested
// slog.Group, and an exotic KindAny value (a func) that json.Marshal cannot
// encode, forcing the String() fallback. It asserts both that the rendered
// values are correct and that the resulting signed chain still verifies.
func TestSafeValue_AllKindsRenderAndVerify(t *testing.T) {
	pub, priv, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	h, err := NewHandler(&buf, &Options{Signer: priv})
	if err != nil {
		t.Fatal(err)
	}

	d := 1500 * time.Millisecond
	when := time.Date(2026, 6, 8, 12, 30, 0, 0, time.UTC)
	// A func value is KindAny and json.Marshal rejects it, so safeValue must fall
	// back to v.String(). slog renders a func as a non-empty string.
	exotic := func() {}

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "kinds", 0)
	r.AddAttrs(
		slog.Duration("dur", d),
		slog.Time("when", when),
		slog.Bool("flag", true),
		slog.Uint64("u", 42),
		slog.Float64("f", 3.5),
		slog.Group("grp", slog.String("inner", "v"), slog.Int("nestedint", 9)),
		slog.Any("weird", exotic),
	)
	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	// Chain still verifies: no exotic value aborted the write.
	rep, err := Verify(bytes.NewReader(buf.Bytes()), pub)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	requireOK(t, rep)

	data := dataMaps(t, buf.Bytes())[0]
	if data["dur"] != d.String() {
		t.Fatalf("dur: want %q, got %#v", d.String(), data["dur"])
	}
	if data["when"] != when.Format(time.RFC3339Nano) {
		t.Fatalf("when: want %q, got %#v", when.Format(time.RFC3339Nano), data["when"])
	}
	if data["flag"] != true {
		t.Fatalf("flag: want true, got %#v", data["flag"])
	}
	if data["u"] != float64(42) {
		t.Fatalf("u: want 42, got %#v", data["u"])
	}
	if data["f"] != 3.5 {
		t.Fatalf("f: want 3.5, got %#v", data["f"])
	}
	// Group folded into dotted keys.
	if data["grp.inner"] != "v" {
		t.Fatalf("grp.inner: want v, got %#v (keys: %v)", data["grp.inner"], keysOf(data))
	}
	if data["grp.nestedint"] != float64(9) {
		t.Fatalf("grp.nestedint: want 9, got %#v", data["grp.nestedint"])
	}
	// The exotic value fell back to a non-empty string rather than aborting.
	s, ok := data["weird"].(string)
	if !ok || s == "" {
		t.Fatalf("weird value should render as a non-empty String() fallback, got %#v", data["weird"])
	}
}

// ---- file.go: OpenFile failure path ----

// TestOpenFile_ParentIsAFile points OpenFile at a path whose parent component is
// itself a regular file, so os.OpenFile cannot create the child. It must return
// an error and a nil handler and nil file (no leaked descriptor).
func TestOpenFile_ParentIsAFile(t *testing.T) {
	dir := t.TempDir()
	parent := filepath.Join(dir, "iam-a-file")
	if err := os.WriteFile(parent, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	// "iam-a-file/child" treats a regular file as a directory: must fail.
	bad := filepath.Join(parent, "child.jsonl")

	h, f, err := OpenFile(bad, nil)
	if err == nil {
		if f != nil {
			_ = f.Close()
		}
		t.Fatal("OpenFile should fail when the parent path is a regular file")
	}
	if h != nil {
		t.Fatal("OpenFile should return a nil handler on error")
	}
	if f != nil {
		_ = f.Close()
		t.Fatal("OpenFile should return a nil file on error")
	}
}

// ---- local helpers (unique names, no collision with existing test files) ----

// pemBlock builds a raw PEM-armored block of the given type and payload without
// going through the marshalers, so a test can craft deliberately malformed
// (wrong-length) but correctly typed blocks.
func pemBlock(t *testing.T, blockType string, payload []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	b.WriteString("-----BEGIN " + blockType + "-----\n")
	// Base64 the payload the way encoding/pem would; the standard library parser
	// is tolerant of our line wrapping for short payloads.
	enc := base64Std(payload)
	b.WriteString(enc)
	b.WriteString("\n-----END " + blockType + "-----\n")
	return b.Bytes()
}

// base64Std is a tiny standard-base64 encoder used by pemBlock. It avoids
// importing encoding/base64 at call sites and keeps the helper self-contained.
func base64Std(in []byte) string {
	const alpha = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var sb strings.Builder
	for i := 0; i < len(in); i += 3 {
		var n uint32
		rem := len(in) - i
		n = uint32(in[i]) << 16
		if rem > 1 {
			n |= uint32(in[i+1]) << 8
		}
		if rem > 2 {
			n |= uint32(in[i+2])
		}
		sb.WriteByte(alpha[(n>>18)&0x3F])
		sb.WriteByte(alpha[(n>>12)&0x3F])
		if rem > 1 {
			sb.WriteByte(alpha[(n>>6)&0x3F])
		} else {
			sb.WriteByte('=')
		}
		if rem > 2 {
			sb.WriteByte(alpha[n&0x3F])
		} else {
			sb.WriteByte('=')
		}
	}
	return sb.String()
}

// dataMaps decodes the Data payload of every entry in a chain file into a map,
// in seq order, so a test can assert on the rendered keys/values.
func dataMaps(t *testing.T, chain []byte) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, l := range lines(chain) {
		var e Entry
		if err := json.Unmarshal(l, &e); err != nil {
			t.Fatalf("decode entry: %v", err)
		}
		m := map[string]any{}
		if err := json.Unmarshal(e.Data, &m); err != nil {
			t.Fatalf("decode data: %v", err)
		}
		out = append(out, m)
	}
	return out
}

// keysOf returns the sorted-ish key set of a data map for readable failures.
func keysOf(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
