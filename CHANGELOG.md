# Changelog

All notable changes to GoLogX will be documented here.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.3] — 2026-05-17

First release with **end-to-end automated tap updates**: the
`HOMEBREW_TAP_GITHUB_TOKEN` secret is now configured on the GoLogX repo, so
goreleaser pushes a fresh `Formula/logx.rb` to
[`AyoubTadlaoui/homebrew-tap`](https://github.com/AyoubTadlaoui/homebrew-tap)
on every tag. `brew upgrade AyoubTadlaoui/tap/logx` now picks up every release
without any maintainer touch.

### Fixed

- **Docs**: docker tags drop the leading `v` (e.g. `ghcr.io/ayoubtadlaoui/logx:0.1.3`,
  not `:v0.1.3`) per common Docker convention. The published image format was
  always correct; only the README / DISTRIBUTION.md / CHANGELOG examples had
  drifted.

### Changed

- **DISTRIBUTION.md**: GHCR visibility flip is now documented as done
  (`ghcr.io/ayoubtadlaoui/logx` is public, anonymous `docker pull` verified
  end-to-end). Homebrew tap PAT instructions tightened to a pre-filled URL
  plus a one-paste `gh secret set` flow with `read -rs` to keep the token off
  the screen and out of shell history.

## [0.1.2] — 2026-05-17

### Added

- **Homebrew tap**: install with `brew install AyoubTadlaoui/tap/logx`. The
  formula lives in [`AyoubTadlaoui/homebrew-tap`](https://github.com/AyoubTadlaoui/homebrew-tap)
  and is auto-updated by goreleaser on every release (when the
  `HOMEBREW_TAP_GITHUB_TOKEN` secret is configured — see DISTRIBUTION.md).
- **Docker image on GHCR**: multi-arch (linux/amd64, linux/arm64) images
  built from a minimal distroless base. Pull with
  `docker pull ghcr.io/ayoubtadlaoui/logx:0.1.2` (or `:latest`).
  Note: docker tags omit the leading `v` (`0.1.2`, not `v0.1.2`) per the
  common Docker convention; goreleaser tags + git tags keep the `v`.
- **`DISTRIBUTION.md`**: maintainer reference covering every install channel,
  the tag-release flow, the one-time PAT setup for tap automation, and the
  one-time visibility flip for the GHCR image.

### Changed

- README CLI install section restructured to lead with Homebrew + Docker,
  with `go install` and prebuilt binaries as alternatives.

## [0.1.1] — 2026-05-17

### Fixed

- `logx -version` now reports a real version for source installs. Previously,
  `go install github.com/AyoubTadlaoui/GoLogX/cmd/logx@vX.Y.Z` printed `dev`
  because `-ldflags="-X main.version=..."` is only applied by goreleaser and
  the Makefile. A new `resolvedVersion()` falls back to
  `debug.ReadBuildInfo().Main.Version` when the build-time variable is unset,
  so `go install ...@vX.Y.Z` now prints `vX.Y.Z` and `@main` prints the Go
  pseudo-version. Goreleaser builds keep precedence through the existing
  `-ldflags` injection.

### Changed

- CLI `-grep` help text and README description: clarified that the substring
  match is applied to the **raw input line** (before JSON parsing), so it
  filters pass-through non-JSON lines too. Behavior is unchanged.

### Docs

- README pretty-output sample updated to match real output (removed decorative
  double-spaces between message and attrs).

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

[Unreleased]: https://github.com/AyoubTadlaoui/GoLogX/compare/v0.1.3...HEAD
[0.1.3]: https://github.com/AyoubTadlaoui/GoLogX/releases/tag/v0.1.3
[0.1.2]: https://github.com/AyoubTadlaoui/GoLogX/releases/tag/v0.1.2
[0.1.1]: https://github.com/AyoubTadlaoui/GoLogX/releases/tag/v0.1.1
[0.1.0]: https://github.com/AyoubTadlaoui/GoLogX/releases/tag/v0.1.0
