package audit

import (
	"bytes"
	"encoding/json"
	"testing"
)

// ---- Attack class (i): byte-level canonicalization probes. The hash MUST be
// computed over the raw data bytes, so ANY byte change to data breaks it. A
// lenient verifier that re-canonicalized JSON before hashing would be a hole. ----

// TestCanon_WhitespaceInsideData_Detected adds a single space inside the data
// object of entry 0 without touching the hash. If Verify hashed raw bytes, the
// extra space changes the hash and the edit is caught. If it canonicalized
// first, the change would slip through (which would be a bug).
func TestCanon_WhitespaceInsideData_Detected(t *testing.T) {
	pls := []json.RawMessage{json.RawMessage(`{"level":"INFO","msg":"x","k":"v"}`)}
	chain := craftChain(t, pls, nil)
	// Insert a space after the first brace inside data only.
	tampered := bytes.Replace(chain, []byte(`{"level":"INFO","msg":"x","k":"v"}`), []byte(`{ "level":"INFO","msg":"x","k":"v"}`), 1)
	if bytes.Equal(tampered, chain) {
		t.Fatal("test setup: data substring not found")
	}
	requireBad(t, mustVerify(t, tampered, nil), 0)
}

// TestCanon_KeyReorderInData_Detected reorders keys inside data without fixing
// the hash. Raw-byte hashing must catch this; semantic JSON equality must not be
// used.
func TestCanon_KeyReorderInData_Detected(t *testing.T) {
	pls := []json.RawMessage{json.RawMessage(`{"a":1,"b":2}`)}
	chain := craftChain(t, pls, nil)
	tampered := bytes.Replace(chain, []byte(`{"a":1,"b":2}`), []byte(`{"b":2,"a":1}`), 1)
	if bytes.Equal(tampered, chain) {
		t.Fatal("test setup: data substring not found")
	}
	requireBad(t, mustVerify(t, tampered, nil), 0)
}

// TestCanon_DuplicateKeyInData_Detected injects a duplicate JSON key into data.
// json.RawMessage stores the raw bytes verbatim, so the hash changes and the
// tamper is caught. (This probes that the verifier never re-parses data to
// "normalize" duplicate keys.)
func TestCanon_DuplicateKeyInData_Detected(t *testing.T) {
	pls := []json.RawMessage{json.RawMessage(`{"amount":1}`)}
	chain := craftChain(t, pls, nil)
	tampered := bytes.Replace(chain, []byte(`{"amount":1}`), []byte(`{"amount":1,"amount":999}`), 1)
	if bytes.Equal(tampered, chain) {
		t.Fatal("test setup: data substring not found")
	}
	requireBad(t, mustVerify(t, tampered, nil), 0)
}

// TestCanon_UnicodeEscapeInData_Detected swaps a literal character for its
// equivalent JSON \u escape inside data. Both decode to the same string ("A"),
// but the bytes differ: a raw-byte hash must break here, while a
// semantic-equality verifier (a bug) would let it through.
func TestCanon_UnicodeEscapeInData_Detected(t *testing.T) {
	// Build the escaped form byte-by-byte so the backslash-u sequence is
	// guaranteed present regardless of source-literal handling. Both forms
	// decode to {"msg":"A"} but differ in raw bytes.
	literal := []byte(`{"msg":"A"}`)
	escaped := []byte(`{"msg":"` + string([]byte{'\\', 'u', '0', '0', '4', '1'}) + `"}`)
	if !json.Valid(escaped) {
		t.Fatalf("test setup: escaped form is not valid JSON: %q", escaped)
	}
	if bytes.Equal(literal, escaped) {
		t.Fatal("test setup: literal and escaped forms are identical bytes")
	}
	pls := []json.RawMessage{json.RawMessage(literal)}
	chain := craftChain(t, pls, nil)
	tampered := bytes.Replace(chain, literal, escaped, 1)
	if bytes.Equal(tampered, chain) {
		t.Fatal("test setup: data substring not found")
	}
	requireBad(t, mustVerify(t, tampered, nil), 0)
}

// TestCanon_ControlCharInData_Detected confirms a control char swapped into data
// changes the bytes and is caught (and does not panic the scanner).
func TestCanon_ControlCharInData_Detected(t *testing.T) {
	pls := []json.RawMessage{json.RawMessage(`{"msg":"ab"}`)}
	chain := craftChain(t, pls, nil)
	tampered := bytes.Replace(chain, []byte(`{"msg":"ab"}`), []byte(`{"msg":"a\tb"}`), 1)
	if bytes.Equal(tampered, chain) {
		t.Fatal("test setup: data substring not found")
	}
	requireBad(t, mustVerify(t, tampered, nil), 0)
}

// ---- Attack class (j): file-shape probes. Sensible behavior, no panic. ----

// TestShape_EmptyFile reports OK with zero entries: an empty log is trivially
// intact.
func TestShape_EmptyFile(t *testing.T) {
	rep := mustVerify(t, nil, nil)
	requireOK(t, rep)
	if rep.Entries != 0 {
		t.Fatalf("expected 0 entries, got %d", rep.Entries)
	}
}

// TestShape_OnlyWhitespace treats a file of only blank lines as empty.
func TestShape_OnlyWhitespace(t *testing.T) {
	rep := mustVerify(t, []byte("\n\n   \n\t\n"), nil)
	requireOK(t, rep)
	if rep.Entries != 0 {
		t.Fatalf("expected 0 entries, got %d", rep.Entries)
	}
}

// TestShape_BlankLinesInterspersed inserts blank lines between valid entries.
// They are skipped, leaving a valid chain.
func TestShape_BlankLinesInterspersed(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	ls := lines(chain)
	var buf bytes.Buffer
	for _, l := range ls {
		buf.WriteString("\n")
		buf.Write(l)
		buf.WriteString("\n\n")
	}
	rep := mustVerify(t, buf.Bytes(), nil)
	requireOK(t, rep)
	if rep.Entries != 3 {
		t.Fatalf("expected 3 entries, got %d", rep.Entries)
	}
}

// TestShape_NoTrailingNewline drops the final newline; the last entry must still
// verify.
func TestShape_NoTrailingNewline(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	trimmed := bytes.TrimRight(chain, "\n")
	rep := mustVerify(t, trimmed, nil)
	requireOK(t, rep)
	if rep.Entries != 3 {
		t.Fatalf("expected 3 entries, got %d", rep.Entries)
	}
}

// TestShape_CRLFLineEndings rewrites the file with CRLF endings. bufio.Scanner
// strips the trailing \r and TrimSpace handles any residue, so the chain still
// verifies (the \r is outside the data RawMessage and not hashed).
func TestShape_CRLFLineEndings(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	crlf := bytes.ReplaceAll(chain, []byte("\n"), []byte("\r\n"))
	rep := mustVerify(t, crlf, nil)
	requireOK(t, rep)
	if rep.Entries != 3 {
		t.Fatalf("expected 3 entries, got %d", rep.Entries)
	}
}

// TestShape_LeadingTrailingSpacesPerLine wraps each entry in spaces and tabs.
// TrimSpace before JSON parse means these are cosmetic and the chain verifies.
func TestShape_LeadingTrailingSpacesPerLine(t *testing.T) {
	chain := craftChain(t, payloads3(), nil)
	ls := lines(chain)
	var buf bytes.Buffer
	for _, l := range ls {
		buf.WriteString("   \t")
		buf.Write(l)
		buf.WriteString("  \t\n")
	}
	rep := mustVerify(t, buf.Bytes(), nil)
	requireOK(t, rep)
	if rep.Entries != 3 {
		t.Fatalf("expected 3 entries, got %d", rep.Entries)
	}
}

// TestShape_OverlongLineNoPanic feeds a single line larger than maxLine. The
// scanner stops with bufio.ErrTooLong, which Verify surfaces as a transport
// error (not a false OK). We assert it is an error, not a clean report.
func TestShape_OverlongLineNoPanic(t *testing.T) {
	huge := make([]byte, maxLine+1024)
	for i := range huge {
		huge[i] = 'a'
	}
	_, err := Verify(bytes.NewReader(huge), nil)
	if err == nil {
		t.Fatal("expected a transport error for an over-long line, got nil")
	}
}
