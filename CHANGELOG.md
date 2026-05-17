# Changelog

All notable changes to GoLogX will be documented here.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.0] — 2026-05-17

First public release.

### Added

- **`logx` library** — zero-dependency toolkit on top of `log/slog`:
  - `PrettyHandler` — colored, concise, human-friendly `slog.Handler` with buffer pooling, attr/group support, optional source location, and NoColor mode.
  - `MultiHandler` — fan a single record out to N underlying handlers; errors are joined, not lost.
  - `RotatingWriter` — size-based rotating `io.WriteCloser` with configurable backup count.
  - `New(Options)` / `NewHandler(Options)` constructors with `FormatPretty` / `FormatJSON` / `FormatText`.
  - `Default()` and `Dev()` opinionated presets.
  - `LevelFromString` and `EnvLevel` for env-driven level wiring (`$LOG_LEVEL`).
- **`logx` CLI** (`cmd/logx`) — pretty-prints JSON `slog` logs from stdin or files:
  - `-level`, `-grep`, `-f` (follow), `-no-color`, `-source`, `-time`, `-version` flags.
  - Non-JSON lines pass through unchanged so panics aren't lost.
- **Tooling** — Makefile, golangci-lint config, GitHub Actions CI on Go 1.22 + 1.23 with race detector and lint.
- **Distribution** — goreleaser config; tagged releases publish prebuilt binaries for linux/darwin/windows × amd64/arm64.
- **Docs** — top-level README with quickstart, runnable `examples/basic`, godoc examples, CONTRIBUTING.

[Unreleased]: https://github.com/AyoubTadlaoui/GoLogX/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/AyoubTadlaoui/GoLogX/releases/tag/v0.1.0
