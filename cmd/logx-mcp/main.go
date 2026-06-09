// Command logx-mcp is a Model Context Protocol (MCP) server for GoLogX audit
// logs. It speaks JSON-RPC 2.0 over stdio, the transport an MCP client such as
// Claude Code, Cursor, or Codex launches it with, and exposes three tools that
// wrap the GoLogX audit package:
//
//	verify_audit_log    check a hash-chained log and report the first broken entry
//	append_audit_entry  append one tamper-evident, optionally signed entry
//	read_audit_log      read entries back, after verifying the chain
//
// Framing follows the MCP stdio spec: one JSON-RPC message per line, UTF-8,
// no embedded newlines. stdin is the request channel, stdout is the response
// channel and carries nothing but valid MCP messages, and every diagnostic
// goes to stderr. It uses only the Go standard library and the local GoLogX
// packages, no third-party modules.
package main

import (
	"os"
)

// version is overridden at build time via -ldflags "-X main.version=...".
// It is reported in the initialize response's serverInfo.
var version = "dev"

func main() {
	// run reads requests from stdin and writes responses to stdout. All
	// diagnostics go to stderr so stdout stays a clean protocol channel.
	os.Exit(run(os.Stdin, os.Stdout, os.Stderr))
}
