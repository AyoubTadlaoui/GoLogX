package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// drive feeds the given request lines through run and returns the parsed
// responses, in order. Each input line is one JSON-RPC message. Diagnostics are
// captured separately so a stray stdout write would show up as an extra,
// unparseable response line.
func drive(t *testing.T, lines ...string) []rpcResponse {
	t.Helper()
	in := strings.NewReader(strings.Join(lines, "\n") + "\n")
	var out, logw bytes.Buffer
	if code := run(in, &out, &logw); code != 0 {
		t.Fatalf("run exited %d, stderr=%q", code, logw.String())
	}

	var responses []rpcResponse
	for _, raw := range bytes.Split(bytes.TrimSpace(out.Bytes()), []byte("\n")) {
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		var resp rpcResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			t.Fatalf("response line is not valid JSON: %q: %v", raw, err)
		}
		if resp.JSONRPC != "2.0" {
			t.Fatalf("response jsonrpc = %q, want 2.0", resp.JSONRPC)
		}
		responses = append(responses, resp)
	}
	return responses
}

func TestInitializeHandshake(t *testing.T) {
	resps := drive(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`)
	if len(resps) != 1 {
		t.Fatalf("got %d responses, want 1", len(resps))
	}
	r := resps[0]
	if r.Error != nil {
		t.Fatalf("initialize returned error: %+v", r.Error)
	}
	if string(*r.ID) != "1" {
		t.Fatalf("id = %s, want 1", string(*r.ID))
	}

	var res struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name string `json:"name"`
		} `json:"serverInfo"`
		Capabilities struct {
			Tools map[string]any `json:"tools"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(r.Result, &res); err != nil {
		t.Fatalf("decode initialize result: %v", err)
	}
	if res.ProtocolVersion != protocolVersion {
		t.Fatalf("protocolVersion = %q, want %q", res.ProtocolVersion, protocolVersion)
	}
	if res.ServerInfo.Name != serverName {
		t.Fatalf("serverInfo.name = %q, want %q", res.ServerInfo.Name, serverName)
	}
	if res.Capabilities.Tools == nil {
		t.Fatal("expected tools capability to be advertised")
	}
}

func TestNotificationGetsNoReply(t *testing.T) {
	// A notification (no id) must produce no response. We pair it with an
	// initialize so there is at least one line of output to confirm framing,
	// then assert the notification did not add a second response.
	resps := drive(t,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":7,"method":"ping"}`,
	)
	if len(resps) != 1 {
		t.Fatalf("got %d responses, want 1 (notification must not be answered)", len(resps))
	}
	if string(*resps[0].ID) != "7" {
		t.Fatalf("id = %s, want 7", string(*resps[0].ID))
	}
}

func TestToolsListShape(t *testing.T) {
	resps := drive(t, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	if len(resps) != 1 {
		t.Fatalf("got %d responses, want 1", len(resps))
	}
	var res struct {
		Tools []struct {
			Name        string         `json:"name"`
			Description string         `json:"description"`
			InputSchema map[string]any `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(resps[0].Result, &res); err != nil {
		t.Fatalf("decode tools/list result: %v", err)
	}

	want := map[string]bool{"verify_audit_log": false, "append_audit_entry": false, "read_audit_log": false}
	for _, tool := range res.Tools {
		if _, ok := want[tool.Name]; !ok {
			t.Fatalf("unexpected tool %q", tool.Name)
		}
		want[tool.Name] = true
		if tool.Description == "" {
			t.Fatalf("tool %q has no description", tool.Name)
		}
		if tool.InputSchema["type"] != "object" {
			t.Fatalf("tool %q inputSchema.type = %v, want object", tool.Name, tool.InputSchema["type"])
		}
		if _, ok := tool.InputSchema["properties"]; !ok {
			t.Fatalf("tool %q inputSchema has no properties", tool.Name)
		}
	}
	for name, seen := range want {
		if !seen {
			t.Fatalf("tool %q missing from tools/list", name)
		}
	}
}

func TestUnknownMethodIsJSONRPCError(t *testing.T) {
	resps := drive(t, `{"jsonrpc":"2.0","id":9,"method":"does/not/exist"}`)
	if len(resps) != 1 {
		t.Fatalf("got %d responses, want 1", len(resps))
	}
	if resps[0].Error == nil {
		t.Fatal("expected a JSON-RPC error for an unknown method")
	}
	if resps[0].Error.Code != codeMethodNotFound {
		t.Fatalf("error code = %d, want %d", resps[0].Error.Code, codeMethodNotFound)
	}
}

func TestUnknownToolIsJSONRPCError(t *testing.T) {
	resps := drive(t, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"nope","arguments":{}}}`)
	if resps[0].Error == nil {
		t.Fatal("expected a JSON-RPC error for an unknown tool")
	}
	if resps[0].Error.Code != codeMethodNotFound {
		t.Fatalf("error code = %d, want %d", resps[0].Error.Code, codeMethodNotFound)
	}
}

func TestStringIDEchoedVerbatim(t *testing.T) {
	resps := drive(t, `{"jsonrpc":"2.0","id":"abc","method":"ping"}`)
	if len(resps) != 1 {
		t.Fatalf("got %d responses, want 1", len(resps))
	}
	// The id must come back as a JSON string "abc", not coerced to a number.
	if string(*resps[0].ID) != `"abc"` {
		t.Fatalf("id = %s, want \"abc\"", string(*resps[0].ID))
	}
}

func TestParseErrorHasNullID(t *testing.T) {
	in := strings.NewReader("{not json}\n")
	var out, logw bytes.Buffer
	if code := run(in, &out, &logw); code != 0 {
		t.Fatalf("run exited %d", code)
	}
	var resp rpcResponse
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("parse-error response is not valid JSON: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != codeParseError {
		t.Fatalf("expected parse error, got %+v", resp.Error)
	}
	if resp.ID != nil {
		t.Fatalf("parse-error id = %s, want null", string(*resp.ID))
	}
}
