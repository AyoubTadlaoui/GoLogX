package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AyoubTadlaoui/GoLogX/audit"
)

// callTool drives a single tools/call request through run and returns the parsed
// tools/call result (content + isError) plus any JSON-RPC error.
func callTool(t *testing.T, name string, args map[string]any) (toolCallResult, *rpcError) {
	t.Helper()
	argBytes, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": json.RawMessage(argBytes)},
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	in := strings.NewReader(string(reqBytes) + "\n")
	var out, logw bytes.Buffer
	if code := run(in, &out, &logw); code != 0 {
		t.Fatalf("run exited %d, stderr=%q", code, logw.String())
	}

	var resp rpcResponse
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("response not valid JSON: %q: %v", out.Bytes(), err)
	}
	if resp.Error != nil {
		return toolCallResult{}, resp.Error
	}
	var res toolCallResult
	if err := json.Unmarshal(resp.Result, &res); err != nil {
		t.Fatalf("decode tools/call result: %v", err)
	}
	return res, nil
}

type toolCallResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	IsError bool `json:"isError"`
}

func (r toolCallResult) text() string {
	var b strings.Builder
	for _, c := range r.Content {
		b.WriteString(c.Text)
	}
	return b.String()
}

// TestVerifyDetectsTamper writes a chain with the audit package, flips one byte
// in the message of the first entry (length-preserving), and asserts the
// verify_audit_log tool reports tampering at that entry.
func TestVerifyDetectsTamper(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	// Build a genuine chain through the audit package directly.
	writeChain(t, path, []string{"first event", "second event", "third event"})

	// First confirm the tool sees it as intact.
	res, rerr := callTool(t, "verify_audit_log", map[string]any{"path": path})
	if rerr != nil {
		t.Fatalf("verify returned JSON-RPC error: %+v", rerr)
	}
	if res.IsError {
		t.Fatalf("expected intact log, got isError with text: %s", res.text())
	}
	if !strings.Contains(res.text(), "intact") {
		t.Fatalf("expected 'intact' in verify text, got: %s", res.text())
	}

	// Tamper: replace a character inside the first entry's data without changing
	// the byte length, so only the hash check (not a JSON parse) catches it.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	tampered := bytes.Replace(raw, []byte("first event"), []byte("FIRST event"), 1)
	if bytes.Equal(tampered, raw) {
		t.Fatal("tamper did not change the file")
	}
	if len(tampered) != len(raw) {
		t.Fatal("tamper changed the byte length; test intends a length-preserving edit")
	}
	if err := os.WriteFile(path, tampered, 0o600); err != nil {
		t.Fatalf("write tampered log: %v", err)
	}

	res, rerr = callTool(t, "verify_audit_log", map[string]any{"path": path})
	if rerr != nil {
		t.Fatalf("verify returned JSON-RPC error: %+v", rerr)
	}
	if !res.IsError {
		t.Fatalf("expected tampering to be reported as isError, text: %s", res.text())
	}
	if !strings.Contains(res.text(), "TAMPERED") {
		t.Fatalf("expected 'TAMPERED' in text, got: %s", res.text())
	}
	if !strings.Contains(res.text(), "index 0") {
		t.Fatalf("expected the first entry (index 0) flagged, got: %s", res.text())
	}
}

// TestAppendVerifyReadRoundTrip appends entries via the tool, verifies the chain
// via the tool, then reads them back via the tool and confirms the messages
// survive the round trip in order.
func TestAppendVerifyReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "round.log")

	// Append two entries, the second with structured attrs.
	if res, rerr := callTool(t, "append_audit_entry", map[string]any{
		"path": path, "message": "agent started", "level": "info",
	}); rerr != nil || res.IsError {
		t.Fatalf("first append failed: rerr=%+v text=%s", rerr, res.text())
	}
	if res, rerr := callTool(t, "append_audit_entry", map[string]any{
		"path": path, "message": "package blocked", "level": "warn",
		"attrs": map[string]any{"package": "loadsh", "reason": "typosquat"},
	}); rerr != nil || res.IsError {
		t.Fatalf("second append failed: rerr=%+v text=%s", rerr, res.text())
	}

	// Verify through the tool.
	res, rerr := callTool(t, "verify_audit_log", map[string]any{"path": path})
	if rerr != nil || res.IsError {
		t.Fatalf("verify after append failed: rerr=%+v text=%s", rerr, res.text())
	}

	// Independent check with the audit package: two intact entries.
	rep, err := audit.VerifyFile(path, nil)
	if err != nil {
		t.Fatalf("audit.VerifyFile: %v", err)
	}
	if !rep.OK || rep.Entries != 2 {
		t.Fatalf("expected 2 intact entries, got ok=%v entries=%d reason=%s", rep.OK, rep.Entries, rep.Reason)
	}

	// Read back through the tool and confirm both messages are present and ordered.
	res, rerr = callTool(t, "read_audit_log", map[string]any{"path": path})
	if rerr != nil || res.IsError {
		t.Fatalf("read failed: rerr=%+v text=%s", rerr, res.text())
	}
	text := res.text()
	iStart := strings.Index(text, "agent started")
	iBlock := strings.Index(text, "package blocked")
	if iStart < 0 || iBlock < 0 {
		t.Fatalf("read text missing an appended message: %s", text)
	}
	if iStart > iBlock {
		t.Fatalf("entries out of order in read output: %s", text)
	}
	if !strings.Contains(text, "typosquat") {
		t.Fatalf("structured attr not present in read output: %s", text)
	}

	// limit returns only the most recent entry.
	res, rerr = callTool(t, "read_audit_log", map[string]any{"path": path, "limit": float64(1)})
	if rerr != nil || res.IsError {
		t.Fatalf("limited read failed: rerr=%+v text=%s", rerr, res.text())
	}
	if strings.Contains(res.text(), "agent started") {
		t.Fatalf("limit=1 should have dropped the first entry: %s", res.text())
	}
	if !strings.Contains(res.text(), "package blocked") {
		t.Fatalf("limit=1 should keep the most recent entry: %s", res.text())
	}
}

// TestReadRefusesTamperedLog confirms read_audit_log verifies first and refuses
// to surface entry data from a broken chain.
func TestReadRefusesTamperedLog(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.log")
	writeChain(t, path, []string{"keep me honest"})

	raw, _ := os.ReadFile(path)
	tampered := bytes.Replace(raw, []byte("keep me honest"), []byte("KEEP me honest"), 1)
	if err := os.WriteFile(path, tampered, 0o600); err != nil {
		t.Fatalf("write tampered log: %v", err)
	}

	res, rerr := callTool(t, "read_audit_log", map[string]any{"path": path})
	if rerr != nil {
		t.Fatalf("unexpected JSON-RPC error: %+v", rerr)
	}
	if !res.IsError {
		t.Fatalf("expected read to refuse a broken chain, text: %s", res.text())
	}
	if strings.Contains(res.text(), "KEEP me honest") {
		t.Fatalf("read leaked tampered data: %s", res.text())
	}
}

// TestMissingPathIsParamsError confirms an absent required argument is a JSON-RPC
// params error, not a tool-level isError result.
func TestMissingPathIsParamsError(t *testing.T) {
	_, rerr := callTool(t, "verify_audit_log", map[string]any{})
	if rerr == nil {
		t.Fatal("expected a JSON-RPC params error for a missing path")
	}
	if rerr.Code != codeInvalidParams {
		t.Fatalf("error code = %d, want %d", rerr.Code, codeInvalidParams)
	}
}

// TestVerifyMissingFileIsToolError confirms an unreadable file is a tool-level
// failure (isError), not a JSON-RPC error.
func TestVerifyMissingFileIsToolError(t *testing.T) {
	res, rerr := callTool(t, "verify_audit_log", map[string]any{"path": filepath.Join(t.TempDir(), "nope.log")})
	if rerr != nil {
		t.Fatalf("unexpected JSON-RPC error for a missing file: %+v", rerr)
	}
	if !res.IsError {
		t.Fatalf("expected isError for a missing file, text: %s", res.text())
	}
}

// writeChain writes an unsigned chain of the given messages directly through the
// audit package, so the round-trip tests start from a genuine, valid log.
func writeChain(t *testing.T, path string, messages []string) {
	t.Helper()
	h, f, err := audit.OpenFile(path, nil)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	defer f.Close()
	log := slog.New(h)
	for _, m := range messages {
		log.Info(m)
	}
}
