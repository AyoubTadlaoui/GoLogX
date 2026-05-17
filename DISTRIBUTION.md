# Distribution

How GoLogX is shipped, by audience.

## Install matrix

| Channel | Audience | One-liner | Auto on each tag? |
|---|---|---|---|
| **Go module proxy / pkg.go.dev** | Library users | `go get github.com/AyoubTadlaoui/GoLogX/logx@vX.Y.Z` | ✓ (built into the Go ecosystem) |
| **`go install`** | CLI users on any Go-friendly machine | `go install github.com/AyoubTadlaoui/GoLogX/cmd/logx@vX.Y.Z` | ✓ (built into the Go ecosystem) |
| **Homebrew tap** | macOS + Linux CLI users | `brew install AyoubTadlaoui/tap/logx` | ✓ (goreleaser pushes to [`AyoubTadlaoui/homebrew-tap`](https://github.com/AyoubTadlaoui/homebrew-tap), **requires `HOMEBREW_TAP_GITHUB_TOKEN` secret**) |
| **GitHub Releases binaries** | All OSes, no Go required | Download from [releases](https://github.com/AyoubTadlaoui/GoLogX/releases) | ✓ (goreleaser, `GITHUB_TOKEN`) |
| **GHCR Docker image** | Container / CI users | `docker run --rm -i ghcr.io/ayoubtadlaoui/logx:X.Y.Z < log.json` | ✓ (multi-arch via goreleaser + buildx, `GITHUB_TOKEN` is enough) |

Roadmap channels not yet wired: Scoop (Windows), WinGet, Arch AUR, Nix flake.

## Tagging and releasing

Standard release flow:

```bash
# 1. land changes on main, ensure CI is green
# 2. bump CHANGELOG.md
# 3. tag and push
git tag -a vX.Y.Z -m "vX.Y.Z"
git push origin vX.Y.Z
```

The [`Release` workflow](.github/workflows/release.yml) takes over:

1. Builds `cmd/logx` for linux / darwin / windows × amd64 / arm64.
2. Creates a GitHub Release with the binaries and `checksums.txt`.
3. Builds and pushes multi-arch Docker images to `ghcr.io/ayoubtadlaoui/logx:X.Y.Z` (no `v` prefix — Docker convention) and `:latest`.
4. Regenerates `Formula/logx.rb` in `AyoubTadlaoui/homebrew-tap` and commits it (if the PAT secret is set).

## Maintainer setup (one-time)

### Homebrew tap PAT

The tap auto-update step needs cross-repo write access, which the default `GITHUB_TOKEN` doesn't have.

1. **Create the PAT.**
   - GitHub → Settings → Developer settings → Personal access tokens → Tokens (classic) → **Generate new token (classic)**.
   - Scope: **`repo`** (full control of private repositories — needed to push commits to the tap repo).
   - Note: name it something like `goreleaser:homebrew-tap`. No expiration is acceptable for a tap-only token; rotate it if you suspect compromise.
2. **Store it as a repo secret on GoLogX.**
   ```bash
   gh secret set HOMEBREW_TAP_GITHUB_TOKEN \
     --repo AyoubTadlaoui/GoLogX \
     --body "<paste PAT here>"
   ```
   (Or via the web UI: Settings → Secrets and variables → Actions → New repository secret.)
3. **Verify** on the next tag — the release workflow's `goreleaser` step should report a commit pushed to `AyoubTadlaoui/homebrew-tap`.

If the secret is missing, the rest of the release still publishes; only the formula-push step fails.

### GHCR image visibility

The first time `ghcr.io/ayoubtadlaoui/logx` is pushed it lands as a **private** package by default. To make it pullable without authentication:

1. Go to https://github.com/users/AyoubTadlaoui/packages/container/logx/settings
2. Under "Danger Zone" → **Change package visibility** → **Public**.
3. (Optional) Connect the package to the GoLogX repo so it shows up on the repo sidebar.

This only needs to be done once.

## Verifying a release

After cutting a tag, run through this short checklist:

```bash
TAG=v0.1.2   # the tag you just pushed

# 1. Library
go install github.com/AyoubTadlaoui/GoLogX/cmd/logx@$TAG && logx -version  # should print $TAG (without the leading 'v' in some renderings, that's OK)

# 2. Prebuilt binary
curl -sL https://github.com/AyoubTadlaoui/GoLogX/releases/download/$TAG/checksums.txt | head

# 3. Homebrew
brew update && brew upgrade AyoubTadlaoui/tap/logx && logx -version

# 4. Docker (note: tag is the version WITHOUT the leading 'v')
docker pull ghcr.io/ayoubtadlaoui/logx:${TAG#v}
docker run --rm ghcr.io/ayoubtadlaoui/logx:${TAG#v} -version
```

If all four print the expected version, the release is good.
