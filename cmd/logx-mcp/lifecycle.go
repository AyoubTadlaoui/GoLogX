package main

import "encoding/json"

// initializeParams is the subset of the client's initialize request we read.
type initializeParams struct {
	ProtocolVersion string `json:"protocolVersion"`
}

// handleInitialize answers the lifecycle handshake. We advertise a tools-only
// capability set with a static tool list (listChanged false), echo the client's
// protocol version when we support it, and otherwise answer with our own.
func (s *server) handleInitialize(params json.RawMessage) (json.RawMessage, *rpcError) {
	var p initializeParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &rpcError{Code: codeInvalidParams, Message: "invalid initialize params: " + err.Error()}
		}
	}

	negotiated := protocolVersion
	if p.ProtocolVersion == protocolVersion {
		negotiated = p.ProtocolVersion
	}

	result := map[string]any{
		"protocolVersion": negotiated,
		"capabilities": map[string]any{
			"tools": map[string]any{"listChanged": false},
		},
		"serverInfo": map[string]any{
			"name":    serverName,
			"version": version,
		},
		"instructions": "GoLogX audit tools. Use verify_audit_log to check a hash-chained log's " +
			"integrity, append_audit_entry to add a tamper-evident entry, and read_audit_log to " +
			"read entries back after verification.",
	}
	return mustMarshal(result), nil
}

// handleToolsList returns the static tool catalog. The set never changes at
// runtime, so cursor pagination is ignored and no nextCursor is emitted.
func (s *server) handleToolsList(_ json.RawMessage) (json.RawMessage, *rpcError) {
	result := map[string]any{"tools": toolDefinitions()}
	return mustMarshal(result), nil
}

// mustMarshal marshals a value we control (maps/slices of plain types). These
// never fail to marshal; a failure would be a programming error, so it panics
// rather than silently emitting a malformed response.
func mustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic("logx-mcp: marshal internal value: " + err.Error())
	}
	return b
}
