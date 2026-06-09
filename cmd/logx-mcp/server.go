package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// protocolVersion is the MCP revision this server implements. On initialize we
// echo the client's version when it matches, otherwise we answer with this and
// let the client decide whether to continue.
const protocolVersion = "2025-06-18"

// serverName is the MCP server name reported in initialize. The shipped binary
// is logx-mcp; the advertised name is the short product name.
const serverName = "logx"

// maxLine bounds a single inbound JSON-RPC line. It matches the audit package's
// own 4 MiB entry ceiling so a large append payload still fits in one message.
const maxLine = 4 << 20

// JSON-RPC 2.0 standard error codes.
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
)

// rpcRequest is one inbound JSON-RPC message. id is decoded as a raw message so
// it is echoed back verbatim with its original JSON type (number or string),
// and a nil id marks a notification, which must never be answered.
type rpcRequest struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params"`
}

// rpcError is the error object inside an error response.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// rpcResponse is one outbound JSON-RPC response. Exactly one of Result or Error
// is set. id is copied verbatim from the request it answers.
type rpcResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *rpcError        `json:"error,omitempty"`
}

// run is the testable entry point: it reads newline-delimited JSON-RPC requests
// from in, dispatches each, and writes responses to out, one compact JSON
// object per line. Diagnostics go to logw. It returns a process exit code and
// returns 0 on a clean stdin EOF (the client closing the stream).
func run(in io.Reader, out io.Writer, logw io.Writer) int {
	srv := &server{out: out, logw: logw}

	reader := bufio.NewReaderSize(in, 64*1024)
	for {
		line, err := readLine(reader)
		if len(line) > 0 {
			srv.handleLine(line)
		}
		if err != nil {
			if err == io.EOF {
				return 0
			}
			fmt.Fprintln(logw, "logx-mcp: read error:", err)
			return 1
		}
	}
}

// readLine reads one newline-delimited message, tolerating lines split across
// OS reads and rejecting lines longer than maxLine so a malformed stream cannot
// exhaust memory. The returned bytes exclude the trailing newline.
func readLine(r *bufio.Reader) ([]byte, error) {
	var buf []byte
	for {
		chunk, err := r.ReadSlice('\n')
		buf = append(buf, chunk...)
		if len(buf) > maxLine {
			// Drain to the next newline so the next call resyncs, then report.
			return nil, fmt.Errorf("message exceeds %d bytes", maxLine)
		}
		if err == nil {
			return trimNewline(buf), nil
		}
		if err == bufio.ErrBufferFull {
			// Partial line: a token longer than the bufio buffer. Keep reading.
			continue
		}
		return trimNewline(buf), err
	}
}

func trimNewline(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}

// server holds the output side of the connection. The audit chain itself is not
// kept open between calls: each tool opens, acts, and closes, so concurrent
// clients and external readers always see a flushed, self-consistent file.
type server struct {
	out  io.Writer
	logw io.Writer
}

// handleLine parses one inbound line and routes it. A line that is not valid
// JSON gets a parse-error response with a null id (we cannot recover the id). A
// notification (nil id) is processed for its side effects but never answered.
func (s *server) handleLine(line []byte) {
	var req rpcRequest
	if err := json.Unmarshal(line, &req); err != nil {
		s.writeError(nil, codeParseError, "parse error: "+err.Error(), nil)
		return
	}

	result, rerr, isNotification := s.dispatch(&req)
	if isNotification {
		return
	}
	if rerr != nil {
		s.writeError(req.ID, rerr.Code, rerr.Message, rerr.Data)
		return
	}
	s.writeResult(req.ID, result)
}

// dispatch routes a request by method. It returns either a marshaled result or
// an rpcError, and a flag marking notifications (which must not be answered).
// Methods that carry no id are treated as notifications regardless of name.
func (s *server) dispatch(req *rpcRequest) (json.RawMessage, *rpcError, bool) {
	isNotification := req.ID == nil

	switch req.Method {
	case "initialize":
		res, rerr := s.handleInitialize(req.Params)
		return res, rerr, isNotification
	case "notifications/initialized":
		// Lifecycle notification, no response. Nothing to do but transition.
		return nil, nil, true
	case "ping":
		// MCP ping: an empty result object keeps the connection alive.
		if isNotification {
			return nil, nil, true
		}
		return json.RawMessage(`{}`), nil, false
	case "tools/list":
		res, rerr := s.handleToolsList(req.Params)
		return res, rerr, isNotification
	case "tools/call":
		res, rerr := s.handleToolsCall(req.Params)
		return res, rerr, isNotification
	default:
		if isNotification {
			// Unknown notification: ignore silently per JSON-RPC. We never
			// answer something without an id.
			return nil, nil, true
		}
		return nil, &rpcError{Code: codeMethodNotFound, Message: "method not found: " + req.Method}, false
	}
}

// writeResult marshals result into a success response and writes it as one line.
func (s *server) writeResult(id *json.RawMessage, result json.RawMessage) {
	if result == nil {
		result = json.RawMessage(`{}`)
	}
	s.write(rpcResponse{JSONRPC: "2.0", ID: id, Result: result})
}

// writeError writes an error response as one line.
func (s *server) writeError(id *json.RawMessage, code int, message string, data any) {
	s.write(rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: message, Data: data}})
}

// write marshals one response to compact JSON and writes it followed by a single
// newline delimiter. encoding/json escapes any newline inside string values, so
// the only raw newline emitted is the delimiter. The write is a single Write
// call to os.Stdout (unbuffered in production), so there is nothing to flush.
func (s *server) write(resp rpcResponse) {
	b, err := json.Marshal(resp)
	if err != nil {
		// Should be impossible for our own responses; report and drop.
		fmt.Fprintln(s.logw, "logx-mcp: marshal response:", err)
		return
	}
	b = append(b, '\n')
	if _, err := s.out.Write(b); err != nil {
		fmt.Fprintln(s.logw, "logx-mcp: write response:", err)
	}
}
