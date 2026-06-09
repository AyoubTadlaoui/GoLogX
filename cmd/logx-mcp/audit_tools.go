package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/AyoubTadlaoui/GoLogX/audit"
	"github.com/AyoubTadlaoui/GoLogX/logx"
)

// scanBuffer matches the audit package's 4 MiB per-line ceiling so reads never
// truncate a long entry that the writer accepted.
const scanBuffer = 4 << 20

// verifyArgs are the arguments for verify_audit_log.
type verifyArgs struct {
	Path       string `json:"path"`
	PubkeyPath string `json:"pubkey_path"`
}

// toolVerify checks a chain's integrity. A missing/unreadable file or a tampered
// chain is surfaced as isError true (a tool-level failure), not a JSON-RPC
// error. Only a malformed arguments shape is a JSON-RPC error.
func (s *server) toolVerify(raw json.RawMessage) (json.RawMessage, *rpcError) {
	var a verifyArgs
	if err := unmarshalArgs(raw, &a); err != nil {
		return nil, &rpcError{Code: codeInvalidParams, Message: err.Error()}
	}
	if a.Path == "" {
		return nil, &rpcError{Code: codeInvalidParams, Message: "verify_audit_log requires a path"}
	}

	pub, rerr := loadPub(a.PubkeyPath)
	if rerr != nil {
		return nil, rerr
	}

	rep, err := audit.VerifyFile(a.Path, pub)
	if err != nil {
		return toolError("could not read %s: %v", a.Path, err)
	}

	if rep.OK {
		text := fmt.Sprintf("OK: %s is intact, %d entries (%s).", a.Path, rep.Entries, modeLabel(pub, rep))
		structured := map[string]any{
			"ok":      true,
			"path":    a.Path,
			"entries": rep.Entries,
			"signed":  rep.Signed,
			"mode":    modeLabel(pub, rep),
		}
		return toolResult(text, structured), nil
	}

	return toolError("TAMPERED: %s, first broken entry at index %d: %s", a.Path, rep.BadSeq, rep.Reason)
}

// appendArgs are the arguments for append_audit_entry.
type appendArgs struct {
	Path        string          `json:"path"`
	Message     string          `json:"message"`
	Level       string          `json:"level"`
	Attrs       json.RawMessage `json:"attrs"`
	PrivkeyPath string          `json:"privkey_path"`
}

// toolAppend opens or resumes the chain at path and appends one entry through
// slog, which is how the audit handler records a hash-chained line.
func (s *server) toolAppend(raw json.RawMessage) (json.RawMessage, *rpcError) {
	var a appendArgs
	if err := unmarshalArgs(raw, &a); err != nil {
		return nil, &rpcError{Code: codeInvalidParams, Message: err.Error()}
	}
	if a.Path == "" {
		return nil, &rpcError{Code: codeInvalidParams, Message: "append_audit_entry requires a path"}
	}
	if a.Message == "" {
		return nil, &rpcError{Code: codeInvalidParams, Message: "append_audit_entry requires a message"}
	}

	level := slog.LevelInfo
	if a.Level != "" {
		lvl, err := logx.LevelFromString(a.Level)
		if err != nil {
			return nil, &rpcError{Code: codeInvalidParams, Message: "invalid level: " + a.Level}
		}
		level = lvl
	}

	attrs, err := decodeAttrs(a.Attrs)
	if err != nil {
		return nil, &rpcError{Code: codeInvalidParams, Message: err.Error()}
	}

	var signer ed25519.PrivateKey
	if a.PrivkeyPath != "" {
		data, err := os.ReadFile(a.PrivkeyPath)
		if err != nil {
			return toolError("could not read private key %s: %v", a.PrivkeyPath, err)
		}
		signer, err = audit.ParsePrivateKeyPEM(data)
		if err != nil {
			return toolError("invalid private key %s: %v", a.PrivkeyPath, err)
		}
	}

	h, f, err := audit.OpenFile(a.Path, &audit.Options{Signer: signer})
	if err != nil {
		return toolError("could not open %s: %v", a.Path, err)
	}
	defer f.Close()

	slog.New(h).LogAttrs(context.Background(), level, a.Message, attrs...)

	signedNote := "unsigned"
	if signer != nil {
		signedNote = "signed"
	}
	text := fmt.Sprintf("Appended a %s entry to %s (%s).", level.String(), a.Path, signedNote)
	structured := map[string]any{
		"path":    a.Path,
		"level":   level.String(),
		"message": a.Message,
		"signed":  signer != nil,
	}
	return toolResult(text, structured), nil
}

// readArgs are the arguments for read_audit_log.
type readArgs struct {
	Path       string `json:"path"`
	Limit      int    `json:"limit"`
	PubkeyPath string `json:"pubkey_path"`
}

// toolRead verifies the chain, then reads entries back. Verification runs first
// so tampered data is never presented as trustworthy: when the chain is broken
// the tool returns isError true and does not emit entry data.
func (s *server) toolRead(raw json.RawMessage) (json.RawMessage, *rpcError) {
	var a readArgs
	if err := unmarshalArgs(raw, &a); err != nil {
		return nil, &rpcError{Code: codeInvalidParams, Message: err.Error()}
	}
	if a.Path == "" {
		return nil, &rpcError{Code: codeInvalidParams, Message: "read_audit_log requires a path"}
	}
	if a.Limit < 0 {
		return nil, &rpcError{Code: codeInvalidParams, Message: "limit must not be negative"}
	}

	pub, rerr := loadPub(a.PubkeyPath)
	if rerr != nil {
		return nil, rerr
	}

	rep, err := audit.VerifyFile(a.Path, pub)
	if err != nil {
		return toolError("could not read %s: %v", a.Path, err)
	}
	if !rep.OK {
		return toolError("refusing to read %s: chain is broken at entry %d: %s", a.Path, rep.BadSeq, rep.Reason)
	}

	entries, err := scanEntries(a.Path)
	if err != nil {
		return toolError("could not read %s: %v", a.Path, err)
	}

	if a.Limit > 0 && len(entries) > a.Limit {
		entries = entries[len(entries)-a.Limit:]
	}

	out, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return toolError("could not encode entries: %v", err)
	}

	structured := map[string]any{
		"path":     a.Path,
		"verified": true,
		"mode":     modeLabel(pub, rep),
		"count":    len(entries),
		"entries":  entries,
	}
	text := fmt.Sprintf("%s verified (%s), showing %d entr%s:\n%s",
		a.Path, modeLabel(pub, rep), len(entries), plural(len(entries)), out)
	return toolResult(text, structured), nil
}

// scanEntries reads every entry from a JSONL audit log. The audit package has no
// public reader, so we scan it ourselves with a buffer large enough to match the
// writer's line ceiling, skipping blank lines.
func scanEntries(path string) ([]audit.Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), scanBuffer)

	var entries []audit.Entry
	for sc.Scan() {
		raw := bytes.TrimSpace(sc.Bytes())
		if len(raw) == 0 {
			continue
		}
		var e audit.Entry
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil, fmt.Errorf("malformed entry: %w", err)
		}
		entries = append(entries, e)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

// loadPub loads an optional Ed25519 public key PEM. An empty path means a
// chain-only check (nil key). A read or parse failure is a JSON-RPC params error
// because it is a caller mistake in the arguments, not a property of the log.
func loadPub(path string) (ed25519.PublicKey, *rpcError) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &rpcError{Code: codeInvalidParams, Message: fmt.Sprintf("could not read public key %s: %v", path, err)}
	}
	pub, err := audit.ParsePublicKeyPEM(data)
	if err != nil {
		return nil, &rpcError{Code: codeInvalidParams, Message: fmt.Sprintf("invalid public key %s: %v", path, err)}
	}
	return pub, nil
}

// decodeAttrs turns the optional attrs object into slog attributes. A JSON
// object is required; anything else is a params error. Keys are recorded in a
// stable order so two identical calls produce byte-identical entry data.
func decodeAttrs(raw json.RawMessage) ([]slog.Attr, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("attrs must be a JSON object: %v", err)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	attrs := make([]slog.Attr, 0, len(keys))
	for _, k := range keys {
		attrs = append(attrs, slog.Any(k, m[k]))
	}
	return attrs, nil
}

// unmarshalArgs decodes a tools/call arguments object, tolerating an absent
// arguments field (no-arg tools) by treating it as an empty object.
func unmarshalArgs(raw json.RawMessage, v any) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("invalid arguments: %v", err)
	}
	return nil
}

// modeLabel describes how thoroughly a file was checked, so a chain-only pass is
// never mistaken for a signature-verified one. It mirrors the CLI's wording.
func modeLabel(pub ed25519.PublicKey, rep *audit.Report) string {
	switch {
	case pub != nil:
		return "chain + signatures"
	case rep.Signed:
		return "chain only; signatures present but unchecked, pass a public key to verify them"
	default:
		return "chain only; log is unsigned"
	}
}

// sortStrings is a tiny insertion sort, matching cmd/logx, to keep attr order
// deterministic without pulling in sort for this small slice.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

func plural(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}
