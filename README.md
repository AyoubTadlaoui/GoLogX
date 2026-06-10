

# GoLogX

> Tamper-evident logging for Go. An append-only, hash-chained, optionally signed `log/slog` handler: if anyone edits, deletes, reorders, or forges a line, `logx verify` catches it. Plus the everyday slog niceties, colored output, JSON, rotation, fan-out, and a CLI. No third-party dependencies, the integrity code is built on the Go standard library alone.

[![CI](https://github.com/AyoubTadlaoui/GoLogX/actions/workflows/ci.yml/badge.svg)](https://github.com/AyoubTadlaoui/GoLogX/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/AyoubTadlaoui/GoLogX/branch/main/graph/badge.svg)](https://codecov.io/gh/AyoubTadlaoui/GoLogX)
[![Release](https://github.com/AyoubTadlaoui/GoLogX/actions/workflows/release.yml/badge.svg)](https://github.com/AyoubTadlaoui/GoLogX/actions/workflows/release.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/AyoubTadlaoui/GoLogX/logx.svg)](https://pkg.go.dev/github.com/AyoubTadlaoui/GoLogX/logx)
[![Go Report Card](https://goreportcard.com/badge/github.com/AyoubTadlaoui/GoLogX)](https://goreportcard.com/report/github.com/AyoubTadlaoui/GoLogX)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Glama MCP server](https://glama.ai/mcp/servers/AyoubTadlaoui/GoLogX/badge)](https://glama.ai/mcp/servers/AyoubTadlaoui/GoLogX)

![logx verify catching a forged audit entry, atlas-ragnarok theme](docs/screenshots/verify.webp)

<sub>An agent's actions recorded into a signed, hash-chained log, then a single forged byte caught at the exact entry. Also available as [GIF](docs/screenshots/verify.gif) or [MP4](docs/screenshots/verify.mp4). Theme: [atlas-ragnarok](https://github.com/AyoubTadlaoui/atlas-ragnarok).</sub>

---

## Why

Most logs are advisory. If something goes wrong and someone with write access wants the record to say something else, nothing stops them. For an audit trail, the kind you keep for a security review, a compliance ask, or to know what an autonomous agent actually did, that is not good enough. You want a log where any change after the fact is detectable.

GoLogX gives you that as an ordinary `slog.Handler`:

- Every record becomes an entry in a **hash chain**. Each entry's SHA-256 covers the one before it, so editing, deleting, reordering, or injecting a line breaks the chain at that exact point.
- With an **Ed25519 key** the entries are also signed, so they cannot be re-forged into a clean-looking chain by anyone who does not hold the private key.
- You read a log back offline with **`logx verify`**, which tells you whether it is intact and, if not, the first entry that was touched.

And it is still a good everyday logger: pretty colored output for humans, JSON for machines, size-based rotation, fan-out to several sinks, and a CLI that pretty-prints JSON logs from any pipeline. Built on the standard library, with **zero external dependencies**.

### A note on where this fits

I also build [npmguard](https://github.com/AyoubTadlaoui/npmguard), a pre-install gate that decides whether an npm package is safe before an AI agent installs it. GoLogX is the complementary half: npmguard gates what an agent is allowed to install, GoLogX keeps a tamper-evident record of what it then did. They are two separate tools that cover complementary ends of the same problem, not wired together.

---

## Install

### As a library

```bash
go get github.com/AyoubTadlaoui/GoLogX@latest
```

### As a CLI

```bash
# Homebrew (macOS + Linux)
brew install AyoubTadlaoui/tap/logx

# Scoop (Windows)
scoop bucket add atlas https://github.com/AyoubTadlaoui/scoop-bucket
scoop install logx

# Arch Linux (via AUR)
yay -S logx-bin     # or: paru -S logx-bin

# Nix / NixOS
nix run github:AyoubTadlaoui/GoLogX

# Universal install script (Linux + macOS, amd64 + arm64), verifies SHA256
curl -fsSL https://raw.githubusercontent.com/AyoubTadlaoui/GoLogX/main/install.sh | sh

# Docker
docker run --rm -i ghcr.io/ayoubtadlaoui/logx:latest < app.json

# Go install (any OS with Go ≥ 1.22)
go install github.com/AyoubTadlaoui/GoLogX/cmd/logx@latest
```

The full channel matrix (WinGet, deb, rpm, prebuilt binaries) is on the [Releases](https://github.com/AyoubTadlaoui/GoLogX/releases) page. Library use requires Go ≥ 1.22.

---

## Quickstart: tamper-evident audit log

Make a signing key once:

```bash
logx keygen -out audit
# wrote audit.key (private key, keep it secret and off the logging host)
# wrote audit.pub (public key, share it with whoever verifies the log)
```

Write a signed, hash-chained log from your program:

```go
package main

import (
    "log/slog"
    "os"

    "github.com/AyoubTadlaoui/GoLogX/audit"
)

func main() {
    // Load the private key produced by `logx keygen`.
    keyPEM, _ := os.ReadFile("audit.key")
    priv, _ := audit.ParsePrivateKeyPEM(keyPEM)

    f, _ := os.OpenFile("agent-audit.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
    defer f.Close()

    h, _ := audit.NewHandler(f, &audit.Options{Signer: priv})
    log := slog.New(h)

    log.Info("npm install approved", "package", "left-pad", "verdict", "clean")
    log.Warn("npm install blocked", "package", "loadsh", "reason", "typosquat")
    log.Info("command executed", "cmd", "node build.js", "exit", 0)
}
```

Later, on any machine that has only the **public** key, check it:

```bash
logx verify -pubkey audit.pub agent-audit.log
# OK   agent-audit.log, 3 entries intact (chain + signatures)
```

If someone edits a single byte, even a length-preserving one, verification fails at the exact entry and the exit code is `1`:

```bash
logx verify -pubkey audit.pub agent-audit.log
# FAIL agent-audit.log, tampering detected at entry 1: entry 1 was modified: recomputed hash does not match
```

A complete runnable version is in [`examples/audit`](examples/audit) (`go run ./examples/audit`).

### Watch it live and keep the proof at the same time

One record can fan out to a pretty handler you read on screen and a signed audit handler on disk:

```go
chain, _ := audit.NewHandler(file, &audit.Options{Signer: priv})
pretty := logx.NewHandler(logx.Options{Format: logx.FormatPretty})

log := slog.New(logx.NewMultiHandler(pretty, chain))
log.Info("hit", "path", "/healthz")
```

For a long-running service, `audit.OpenFile` reopens an existing log and continues the same chain across restarts instead of starting over.

---

## How verification works

- Each entry is `{seq, time, prev, data, hash, sig}`, one JSON object per line.
- `hash = SHA-256(seq, prev, time, data)`, with each field length-prefixed so no crafted input can shift the boundaries. `prev` is the hash of the previous entry; the first entry chains against a fixed genesis value.
- `sig`, when present, is an Ed25519 signature over the entry's hash.
- `logx verify` walks the file, recomputes each hash from the **verbatim** bytes it reads (no re-encoding, so there is nothing lenient to exploit), checks that every `prev` links to the entry before it, and, when you pass `-pubkey`, checks every signature. The first thing that does not line up is reported, with its entry number.

## What it protects against, and what it does not

Honesty matters more here than a long feature list, so this is exact.

**Detected:**
- Editing any field of any entry (message, an attribute, level, timestamp).
- Deleting, reordering, or inserting lines.
- With `-pubkey`: forging or re-signing entries without the private key.

**Not detected on its own, by design:**
- **Dropping entries from the very end of the file.** The surviving prefix is still a valid chain. If end-truncation is in your threat model, record the head hash and entry count somewhere the writer cannot reach (a second host, an object store with retention) and compare.
- **A full rewrite of an unsigned log.** Without a signature, anyone holding the whole file can recompute every hash. This is exactly what signing prevents, so sign the log and keep the private key off the host that writes it. Hand only the public key to whoever verifies.

The package depends on nothing outside the standard library (`crypto/sha256`, `crypto/ed25519`, `crypto/rand`, `encoding/pem`). The thing that proves your logs were not tampered with carries no third-party code in its own trust path.

---

## Quickstart: everyday logging

The tamper-evident audit core is built on an ordinary `log/slog` handler, so GoLogX is also a complete everyday slog toolkit. Use it for plain logging and turn on the integrity guarantees when you need them.

```go
log := logx.New(logx.Options{
    Level:  logx.EnvLevel(slog.LevelInfo), // honors $LOG_LEVEL
    Format: logx.FormatPretty,
    Output: os.Stderr,
})

log.Info("server up", "port", 8080, "env", "prod")
log.Warn("slow query", "ms", 230, "table", "users")
```

```text
21:10:27.036 INF server up port=8080 env=prod
21:10:27.120 WRN slow query ms=230 table=users
```

Pretty to stderr plus JSON to a rotating file, both seeing the same record:

```go
rotator := &logx.RotatingWriter{Path: "app.log", MaxSize: 1 << 20, MaxBackups: 3}
defer rotator.Close()

multi := logx.NewMultiHandler(
    logx.NewPrettyHandler(os.Stderr, nil),
    slog.NewJSONHandler(rotator, &slog.HandlerOptions{Level: slog.LevelInfo}),
)
log := slog.New(multi).With("service", "api")
```

---

## CLI

The `logx` binary pretty-prints JSON `slog` logs, and verifies audit logs.

```bash
# pretty-print a JSON log stream
myapp 2>&1 | logx
logx -level=warn -grep=timeout app.log
logx -f /var/log/app.json          # tail follow

# audit
logx keygen -out audit             # make a signing keypair
logx verify -pubkey audit.pub a.log   # check integrity + signatures
logx verify a.log                  # chain-only check, no key
```

| Command | Purpose |
|---|---|
| `logx [flags] [file ...]` | Pretty-print JSON slog logs (stdin or files) |
| `logx verify [-pubkey k] [-quiet] file ...` | Check a hash-chained log. Exit `0` intact, `1` tampered, `2` unreadable |
| `logx keygen [-out audit] [-force]` | Write an Ed25519 keypair (`<out>.key`, `<out>.pub`) |

Pretty-print flags (`-level`, `-grep`, `-f`, `-no-color`, `-source`, `-time`, `-version`) are unchanged; lines that are not JSON are passed through untouched.

---

## MCP server for AI agents

`logx-mcp` is a [Model Context Protocol](https://modelcontextprotocol.io) server that lets an AI agent keep and check a tamper-evident audit log of what it does. It speaks JSON-RPC 2.0 over stdio, the transport that Claude Code, Cursor, and Codex launch a server with, and exposes three tools wired to the audit core. It is the same zero-dependency code as the rest of GoLogX: standard library plus the local `audit` package, no MCP SDK.

The idea is simple. An agent that takes actions on your behalf should leave a record you can trust later. `append_audit_entry` writes each step into a hash chain, optionally signed, and `verify_audit_log` proves afterward that nothing in that record was edited, deleted, or reordered.

| Tool | What it does |
|---|---|
| `verify_audit_log` | Verify a chain's integrity. Returns intact, or the index and detail of the first entry that was edited, deleted, reordered, or forged. Pass `pubkey_path` to also check Ed25519 signatures. |
| `append_audit_entry` | Append one tamper-evident entry (`message`, optional `level`, `attrs`, `privkey_path`) to a log, creating it if needed and resuming the existing chain across calls. |
| `read_audit_log` | Read entries back. The chain is verified first, so tampered data is never returned as trustworthy. Use `limit` for the most recent N. |

A failed operation (a tampered log, an unreadable file) comes back as a tool result with `isError` set, with the detail in the text. A malformed call (unknown tool, missing required argument) comes back as a JSON-RPC error. Diagnostics go to stderr; stdout carries nothing but protocol messages.

### Setup

Install the binary from source (or grab it from a release):

```bash
go install github.com/AyoubTadlaoui/GoLogX/cmd/logx-mcp@latest
```

**Claude Code** (`claude mcp add`, or in `~/.claude.json` / a project `.mcp.json`):

```json
{
  "mcpServers": {
    "gologx": {
      "command": "logx-mcp"
    }
  }
}
```

**Cursor** (`~/.cursor/mcp.json` or `.cursor/mcp.json` in the project):

```json
{
  "mcpServers": {
    "gologx": {
      "command": "logx-mcp"
    }
  }
}
```

**Codex** (`~/.codex/config.toml`):

```toml
[mcp_servers.gologx]
command = "logx-mcp"
```

Use an absolute path to the binary if it is not on the client's `PATH`. Once it is connected, the agent can call the three tools above. Sign the log by passing `privkey_path` to `append_audit_entry` (and the matching `pubkey_path` to `verify_audit_log`); make the keypair with `logx keygen`.

---

## API surface

| Symbol | What it is |
|---|---|
| [`audit.NewHandler(w, opts)`](https://pkg.go.dev/github.com/AyoubTadlaoui/GoLogX/audit#NewHandler) | A hash-chained, optionally signed `slog.Handler` |
| [`audit.OpenFile(path, opts)`](https://pkg.go.dev/github.com/AyoubTadlaoui/GoLogX/audit#OpenFile) | Same, resuming the chain already in a file |
| [`audit.Verify(r, pub)` / `audit.VerifyFile(path, pub)`](https://pkg.go.dev/github.com/AyoubTadlaoui/GoLogX/audit#Verify) | Walk a log and report the first broken entry |
| [`audit.GenerateKey()`](https://pkg.go.dev/github.com/AyoubTadlaoui/GoLogX/audit#GenerateKey) and the `…KeyPEM` helpers | Make and load Ed25519 keys |
| [`logx.New(opts)` / `logx.NewHandler(opts)`](https://pkg.go.dev/github.com/AyoubTadlaoui/GoLogX/logx#New) | Build a logger / handler from `Options` |
| [`logx.NewPrettyHandler`](https://pkg.go.dev/github.com/AyoubTadlaoui/GoLogX/logx#NewPrettyHandler), [`logx.NewMultiHandler`](https://pkg.go.dev/github.com/AyoubTadlaoui/GoLogX/logx#NewMultiHandler), [`logx.RotatingWriter`](https://pkg.go.dev/github.com/AyoubTadlaoui/GoLogX/logx#RotatingWriter) | Pretty handler, fan-out, rotation |

---

## Performance

Microbenchmarks on `darwin/arm64`, single record with three attributes, output discarded:

```text
BenchmarkPretty-8        597.8 ns/op    16 B/op    1 allocs/op
BenchmarkStdlibJSON-8    579.1 ns/op     0 B/op    0 allocs/op
BenchmarkMulti-8         829.7 ns/op    16 B/op    1 allocs/op
```

The pretty handler is on par with stdlib `TextHandler` / `JSONHandler`. The audit handler does one SHA-256 (and one Ed25519 sign, if enabled) per record; run `make bench` to measure it on your hardware.

---

## Common tasks

```bash
make test        # go test ./...
make test-race   # go test -race ./...
make bench       # microbenchmarks
make check       # gofmt + vet + race tests
```

---

## Contributing

PRs and issues welcome. The bar is in [CONTRIBUTING.md](CONTRIBUTING.md). Run `make check` and open the PR once it is clean.

## License

MIT, see [LICENSE](LICENSE).

---

### About the author

**Ayoub Tadlaoui**, *Atlas Kaisar*, a problem-solver from Morocco, building software since 2016.

> "High performance knows no part-time commitment."
