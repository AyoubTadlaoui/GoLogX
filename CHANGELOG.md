# Changelog

All notable changes to GoLogX will be documented here.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.14] — 2026-05-18

### Fixed

- Screenshot vignette is now **pixel-exact to the live GLSL shader**, with
  the luminance mask applied. In v0.1.13 the static-PNG overlay washed text
  in the tinted bands because it couldn't honor the per-pixel
  `bg_mask = 1 - smoothstep(0.04, 0.18, lum)` that the real shader uses to
  protect text. Now text stays crisp under the storm-fire glow.

### Changed

- `docs/screenshots/_vignette.py` replaced with `_shader.py` — a NumPy+PIL
  pixel-by-pixel reimplementation of `atlas-ragnarok.glsl`. Same geometry,
  same colors, same luminance mask. Composited per-frame instead of as a
  static overlay.
- `gen.sh` pipeline now writes shaded PNG frames and lets ffmpeg's
  `palettegen`/`paletteuse` build the optimized GIF (instead of PIL's
  weaker GIF encoder), keeping the output around the same size as the raw
  vhs capture (~170KB for the WebP).

## [0.1.13] — 2026-05-18

### Changed

- Screenshots now carry the **atlas-ragnarok storm-fire vignette** — thunder-blue
  glow at the top edge, crimson glow at the bottom, radial darkening toward the
  corners — same colors and geometry as the live Ghostty shader, implemented in
  2D and composited onto every frame in post. Brings the GoLogX visual identity
  into alignment with the author's actual terminal aesthetic.
- Window chrome (Mac traffic-light dots) and the slate-blue margin are gone.
  The terminal is now edge-to-edge so the vignette sits flush over the whole
  canvas, matching what you'd see in a real Ghostty session.

### Added

- `docs/screenshots/_vignette.py` — Python+PIL generator that produces the
  vignette overlay at any dimension. Reproduces the GLSL shader exactly:
  `thunder_blue = vec3(0.055, 0.115, 0.320)`, `red_tint = vec3(0.220, 0.014, 0.028)`,
  `vignette = smoothstep(0.18, 1.0, dist)`, glow bands tightened to
  `smoothstep(0.78, 0.97)` to compensate for the absence of the shader's
  luminance mask in this 2D pipeline.

### Tooling

- `docs/screenshots/gen.sh` now requires `python3` + `pillow` and runs the
  full pipeline: vhs → vignette composite → WebP + MP4. Two new ffmpeg
  passes (one for the hero PNG, one for every frame of the animated GIF).

## [0.1.12] — 2026-05-18

### Changed

- README animation now embedded as an **animated WebP** image rather than a
  `<video>` tag. GitHub's README HTML sanitizer strips `<video>` entirely
  (only allows it when src points to a `user-attachments.githubusercontent.com`
  URL, which is reserved for files uploaded via PR/Issue drag-drop). WebP
  renders through the standard `<img>` path the sanitizer leaves alone, plays
  in every modern browser including Safari, and is smaller than the source
  GIF. The GIF and MP4 copies remain available as sub-links beneath the image.
- `gen.sh` adds `gif2webp` to its requirements (`brew install webp`). It now
  produces `follow.webp` as the README-embedded artifact.

### Fixed

- The animation player is **actually visible** in the rendered README — v0.1.9
  through v0.1.11 left a blank gap between the hero shot and the theme
  attribution because the `<video>` element was being stripped.

## [0.1.11] — 2026-05-18

### Fixed

- README's `<video>` embed for the follow demo now uses the absolute raw
  GitHub URL (`https://github.com/AyoubTadlaoui/GoLogX/raw/main/...`).
  GitHub's README sanitizer rewrites relative paths only inside `<img>`
  and `<a>` — not inside `<video src>` — so v0.1.9 / v0.1.10 rendered an
  empty `<video>` element with a missing source between the hero and the
  theme attribution. The MP4 player now actually shows up.

## [0.1.10] — 2026-05-18

### Changed

- Screenshots now framed as a card — vhs `Set WindowBar Colorful` adds
  Mac-style traffic-light controls, `Set Margin/MarginFill #0F172A` adds a
  slate-blue surround, `Set BorderRadius 10` rounds the corners. The
  pure-black atlas-ragnarok terminal now reads as a distinct rectangle on
  GitHub's dark page background instead of bleeding into it. Theme palette
  (blue WRN, crimson ERR, peppermint INF/DBG, gray attrs) is unchanged but
  much more visible thanks to the clear edge.

## [0.1.9] — 2026-05-18

### Changed

- Screenshots reskinned with the
  [atlas-ragnarok](https://github.com/AyoubTadlaoui/atlas-ragnarok) terminal
  theme — pure black bg, thunder-blue accents, peppermint greens, crimson
  reds. Both the static hero and the animated demo now match the author's
  actual terminal aesthetic.
- The static hero is now produced by vhs's `Screenshot` directive (a new
  `hero.tape`) instead of freeze. freeze's `--config` controls window/font
  chrome but does not override the ANSI palette when rendering `--execute`
  output, so the theme had no effect there. vhs drives both shots, applying
  the same inline theme JSON, so the hero and the animation are always
  visually consistent.

### Added

- `docs/screenshots/follow.mp4` — H.264-encoded copy of the animated demo,
  generated by `gen.sh`. The README now embeds it via `<video controls loop
  muted playsinline>`, with the GIF kept as a fallback link. Safari was
  refusing to animate the GIF even after the user's accessibility toggles
  (Reduce Motion off, Auto-Play on); the MP4 sidesteps that entirely.
- `docs/screenshots/hero.tape` — vhs tape for the static hero shot.

### Removed

- `docs/screenshots/freeze.json` — no longer used (vhs handles both shots).

## [0.1.8] — 2026-05-18

### Added

- **README screenshots** — a static PNG hero shot (rendered via Charmbracelet
  `freeze`) and an animated GIF (rendered via `vhs`) showing `logx -f` tailing
  a JSON stream in real time. Both live in `docs/screenshots/` and are
  embedded at the top of the README.
- `docs/screenshots/gen.sh` regenerates both from
  `docs/screenshots/sample.json` (input for the hero) and
  `docs/screenshots/follow.tape` (vhs script for the GIF). Re-run after CLI
  output changes — no version baked into the images.

## [0.1.7] — 2026-05-18

### Added

- **AUR `logx-bin` is live.** Maintainer setup completed: AUR account
  `AyoubTadlaoui` registered, ed25519 SSH key registered on aurweb,
  matching private key stored as the `AUR_KEY` repo secret on GoLogX.
  `ssh aur@aur.archlinux.org help` now succeeds end-to-end.
  This release is the first to actually exercise the goreleaser `aurs:`
  pipe — it pushes a fresh PKGBUILD to `ssh://aur@aur.archlinux.org/logx-bin.git`.
  Arch users can install with:

  ```bash
  yay -S logx-bin     # or: paru -S logx-bin
  ```

  The single-source-of-truth captcha solver I wrote during setup
  (`pacman -V | sed -r 's#[0-9]+#331#g' | md5sum | cut -c1-6`) is
  preserved in DISTRIBUTION.md for future AUR account creators.

## [0.1.6] — 2026-05-18

### Added — two more install channels (every roadmap channel now wired)

- **Nix flake** at the repo root (`flake.nix`), consumed directly from GitHub —
  no separate registry needed. Built with `pkgs.buildGoModule`, no vendor tree
  (GoLogX has zero external deps), version baked in via `-ldflags`.

  ```bash
  nix run github:AyoubTadlaoui/GoLogX             # one-shot
  nix shell github:AyoubTadlaoui/GoLogX           # in PATH for one shell
  nix profile install github:AyoubTadlaoui/GoLogX # persistent
  nix develop github:AyoubTadlaoui/GoLogX         # dev shell with Go/gopls/golangci-lint/goreleaser
  ```

- **Arch AUR** publishing of the `logx-bin` binary package via goreleaser's
  `aurs:` pipe. On every tag, goreleaser SSHes to `aur@aur.archlinux.org` and
  pushes a fresh PKGBUILD that grabs the prebuilt Linux tarball from this
  release.

  ```bash
  yay -S logx-bin      # or: paru -S logx-bin
  ```

  **Maintainer setup required** (one-time): AUR account + SSH key + `AUR_KEY`
  repo secret. Until that's done, this pipe is skipped cleanly (rest of the
  release still ships). Step-by-step instructions in
  [`DISTRIBUTION.md`](DISTRIBUTION.md).

### Changed

- README install snippet now lists 11 install channels (Homebrew, Scoop, WinGet,
  AUR, Nix, install.sh, .deb, .rpm, Docker, `go install`, prebuilt binaries).
- DISTRIBUTION.md install matrix grew from 9 to 11 channels. The "roadmap
  channels not yet wired" line shrunk to Snap, nixpkgs upstream, Chocolatey —
  every channel from the original distribution roadmap is now done.

## [0.1.5] — 2026-05-18

### Added — four new install channels

- **Linux native packages** (`.deb` + `.rpm`) attached to every GitHub Release
  via goreleaser nfpms. No new infrastructure required.
  ```bash
  sudo dpkg -i logx_0.1.5_linux_amd64.deb     # Debian / Ubuntu
  sudo rpm -i logx-0.1.5-1.x86_64.rpm         # RHEL / Fedora / SUSE
  ```
- **Universal install script** `install.sh` (POSIX `sh`, works on bash / dash /
  busybox). Detects OS + arch, downloads the right release tarball, verifies
  its SHA256 against `checksums.txt`, and installs to `/usr/local/bin` (or
  `$HOME/.local/bin` when not root). Honors `VERSION` and `INSTALL_DIR` env
  vars. Verified end-to-end against v0.1.4.
  ```bash
  curl -fsSL https://raw.githubusercontent.com/AyoubTadlaoui/GoLogX/main/install.sh | sh
  ```
- **Scoop bucket** for Windows users. Manifest auto-pushed to
  [`AyoubTadlaoui/scoop-bucket`](https://github.com/AyoubTadlaoui/scoop-bucket)
  on every tag (reuses the existing `HOMEBREW_TAP_GITHUB_TOKEN` PAT).
  ```powershell
  scoop bucket add atlas https://github.com/AyoubTadlaoui/scoop-bucket
  scoop install logx
  ```
- **WinGet submission** to Microsoft's official Windows package manager. On
  every tag, goreleaser pushes the manifest to a branch on the maintainer's
  fork ([`AyoubTadlaoui/winget-pkgs`](https://github.com/AyoubTadlaoui/winget-pkgs))
  and opens a PR upstream to [`microsoft/winget-pkgs`](https://github.com/microsoft/winget-pkgs).
  Microsoft reviews the first PR (1–3 days); subsequent releases often
  auto-merge.
  ```powershell
  winget install AyoubTadlaoui.logx
  ```

### Changed

- README install section restructured to lead with package managers
  (Homebrew / Scoop / WinGet) before scripts and binaries.
- DISTRIBUTION.md install matrix updated to include all nine channels.

## [0.1.4] — 2026-05-18

### Fixed

- `logx -version` now writes to **stdout**, not stderr. This matches the
  convention used by `git --version`, `go version`, `node --version`,
  `docker --version`, etc., and is what every shell pipeline expects:
  ```bash
  V=$(logx -version)     # now actually captures "0.1.4"
  ```
  Previously the version landed on stderr, which broke
  `brew test AyoubTadlaoui/tap/logx`: the auto-generated formula calls
  `shell_output("#{bin}/logx -version")` (stdout-only) and saw an empty
  string. The regression test was strengthened to assert (a) the version
  shows up on stdout and (b) stderr stays clean.

### Unchanged

- Error messages, parse-failure usage banners, and `-h` help still go to
  stderr — that's where the `flag` package puts them by default and what
  `tool 2>/dev/null` users expect.

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

[Unreleased]: https://github.com/AyoubTadlaoui/GoLogX/compare/v0.1.14...HEAD
[0.1.14]: https://github.com/AyoubTadlaoui/GoLogX/releases/tag/v0.1.14
[0.1.13]: https://github.com/AyoubTadlaoui/GoLogX/releases/tag/v0.1.13
[0.1.12]: https://github.com/AyoubTadlaoui/GoLogX/releases/tag/v0.1.12
[0.1.11]: https://github.com/AyoubTadlaoui/GoLogX/releases/tag/v0.1.11
[0.1.10]: https://github.com/AyoubTadlaoui/GoLogX/releases/tag/v0.1.10
[0.1.9]: https://github.com/AyoubTadlaoui/GoLogX/releases/tag/v0.1.9
[0.1.8]: https://github.com/AyoubTadlaoui/GoLogX/releases/tag/v0.1.8
[0.1.7]: https://github.com/AyoubTadlaoui/GoLogX/releases/tag/v0.1.7
[0.1.6]: https://github.com/AyoubTadlaoui/GoLogX/releases/tag/v0.1.6
[0.1.5]: https://github.com/AyoubTadlaoui/GoLogX/releases/tag/v0.1.5
[0.1.4]: https://github.com/AyoubTadlaoui/GoLogX/releases/tag/v0.1.4
[0.1.3]: https://github.com/AyoubTadlaoui/GoLogX/releases/tag/v0.1.3
[0.1.2]: https://github.com/AyoubTadlaoui/GoLogX/releases/tag/v0.1.2
[0.1.1]: https://github.com/AyoubTadlaoui/GoLogX/releases/tag/v0.1.1
[0.1.0]: https://github.com/AyoubTadlaoui/GoLogX/releases/tag/v0.1.0
