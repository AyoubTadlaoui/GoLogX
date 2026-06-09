package main

import (
	"encoding/json"
	"fmt"
)

// toolDefinitions is the static MCP tool catalog. Each inputSchema is a JSON
// Schema object (type object, with properties and required) that the client
// validates arguments against before calling.
func toolDefinitions() []map[string]any {
	return []map[string]any{
		{
			"name": "verify_audit_log",
			"description": "Verify the integrity of a GoLogX hash-chained audit log file. " +
				"Reports whether the chain is intact or, if not, the index and detail of the first " +
				"entry that was edited, deleted, reordered, or forged. Optionally checks Ed25519 " +
				"signatures when a public key PEM path is given.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Filesystem path to the audit log file (JSONL).",
					},
					"pubkey_path": map[string]any{
						"type": "string",
						"description": "Optional path to an Ed25519 public key PEM. When set, every entry " +
							"must carry a signature that verifies against it.",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			"name": "append_audit_entry",
			"description": "Append one tamper-evident entry to a GoLogX audit log, creating the file " +
				"if needed and resuming the existing hash chain across calls. Optionally signs the entry " +
				"with an Ed25519 private key PEM. Each call adds exactly one hash-chained record.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Filesystem path to the audit log file. Created if absent, appended otherwise.",
					},
					"message": map[string]any{
						"type":        "string",
						"description": "The log message for this entry.",
					},
					"level": map[string]any{
						"type":        "string",
						"description": "Log level: debug, info, warn, or error. Defaults to info.",
						"enum":        []string{"debug", "info", "warn", "error"},
					},
					"attrs": map[string]any{
						"type":        "object",
						"description": "Optional structured key/value attributes recorded with the entry.",
					},
					"privkey_path": map[string]any{
						"type":        "string",
						"description": "Optional path to an Ed25519 private key PEM. When set, the entry is signed.",
					},
				},
				"required": []string{"path", "message"},
			},
		},
		{
			"name": "read_audit_log",
			"description": "Read entries back from a GoLogX audit log. The chain is verified first, so " +
				"tampered data is never surfaced as trustworthy; the result reports the verification " +
				"outcome alongside the entries. Use limit to return only the most recent entries.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Filesystem path to the audit log file (JSONL).",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of most-recent entries to return. Omit or 0 for all entries.",
						"minimum":     0,
					},
					"pubkey_path": map[string]any{
						"type":        "string",
						"description": "Optional path to an Ed25519 public key PEM used to verify signatures before reading.",
					},
				},
				"required": []string{"path"},
			},
		},
	}
}

// toolCallParams is the params object of a tools/call request.
type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// handleToolsCall validates the call envelope and routes to the named tool.
// An unknown tool name or malformed params shape is a JSON-RPC error
// (-32602 / -32601). A tool that runs but fails (verify reports tampering, file
// unreadable) is a normal result with isError true, which the per-tool handlers
// build via toolError.
func (s *server) handleToolsCall(params json.RawMessage) (json.RawMessage, *rpcError) {
	var p toolCallParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &rpcError{Code: codeInvalidParams, Message: "invalid tools/call params: " + err.Error()}
	}
	if p.Name == "" {
		return nil, &rpcError{Code: codeInvalidParams, Message: "tools/call requires a tool name"}
	}

	switch p.Name {
	case "verify_audit_log":
		return s.toolVerify(p.Arguments)
	case "append_audit_entry":
		return s.toolAppend(p.Arguments)
	case "read_audit_log":
		return s.toolRead(p.Arguments)
	default:
		return nil, &rpcError{Code: codeMethodNotFound, Message: "unknown tool: " + p.Name}
	}
}

// toolResult builds a successful tools/call result: a single text content block,
// optionally carrying a structured mirror of the same data.
func toolResult(text string, structured any) json.RawMessage {
	res := map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": false,
	}
	if structured != nil {
		res["structuredContent"] = structured
	}
	return mustMarshal(res)
}

// toolError builds a tool-execution failure result: a text content block with
// isError true. This is NOT a JSON-RPC error; the call itself succeeded, the
// underlying operation did not.
func toolError(format string, args ...any) (json.RawMessage, *rpcError) {
	msg := fmt.Sprintf(format, args...)
	res := map[string]any{
		"content": []map[string]any{{"type": "text", "text": msg}},
		"isError": true,
	}
	return mustMarshal(res), nil
}
