# Distribution

How GoLogX is shipped, by audience.

## Install matrix

| Channel | Audience | One-liner | Auto on each tag? |
|---|---|---|---|
| **Go module proxy / pkg.go.dev** | Library users | `go get github.com/AyoubTadlaoui/GoLogX/logx@vX.Y.Z` | ✓ (built into the Go ecosystem) |
| **`go install`** | CLI users on any Go-friendly machine | `go install github.com/AyoubTadlaoui/GoLogX/cmd/logx@vX.Y.Z` | ✓ (built into the Go ecosystem) |
| **Homebrew tap** | macOS + Linux CLI users | `brew install AyoubTadlaoui/tap/logx` | ✓ (goreleaser pushes to [`AyoubTadlaoui/homebrew-tap`](https://github.com/AyoubTadlaoui/homebrew-tap), uses `HOMEBREW_TAP_GITHUB_TOKEN` PAT) |
| **Scoop bucket** | Windows CLI users | `scoop bucket add atlas https://github.com/AyoubTadlaoui/scoop-bucket && scoop install logx` | ✓ (goreleaser pushes to [`AyoubTadlaoui/scoop-bucket`](https://github.com/AyoubTadlaoui/scoop-bucket), reuses the same PAT) |
| **WinGet** | Windows CLI users | `winget install AyoubTadlaoui.logx` | ✓ on each tag goreleaser pushes to your fork [`AyoubTadlaoui/winget-pkgs`](https://github.com/AyoubTadlaoui/winget-pkgs) and opens a PR to [`microsoft/winget-pkgs`](https://github.com/microsoft/winget-pkgs); Microsoft reviews the first PR (1–3 days), later releases tend to auto-merge |
| **GitHub Releases binaries** | All OSes, no Go required | Download from [releases](https://github.com/AyoubTadlaoui/GoLogX/releases) | ✓ (goreleaser, `GITHUB_TOKEN`) |
| **.deb (Debian/Ubuntu)** | Debian-family Linux users | `sudo dpkg -i logx_X.Y.Z_linux_amd64.deb` (download from releases) | ✓ (goreleaser nfpms, attached to GH Release) |
| **.rpm (RHEL/Fedora/SUSE)** | RPM-family Linux users | `sudo rpm -i logx-X.Y.Z-1.x86_64.rpm` (download from releases) | ✓ (goreleaser nfpms, attached to GH Release) |
| **Universal install script** | Any Linux / macOS user, scriptable | `curl -fsSL https://raw.githubusercontent.com/AyoubTadlaoui/GoLogX/main/install.sh \| sh` | ✓ (script lives in repo, always pulls the latest GH Release) |
| **GHCR Docker image** | Container / CI users | `docker run --rm -i ghcr.io/ayoubtadlaoui/logx:X.Y.Z < log.json` | ✓ (multi-arch via goreleaser + buildx, `GITHUB_TOKEN` is enough) |

Roadmap channels not yet wired: Arch AUR, Nix flake, Snap.

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

The tap auto-update step needs cross-repo write access, which the default
`GITHUB_TOKEN` doesn't have.

1. **Create the PAT.** Open this pre-filled link (sets the description and
   `repo` scope for you):

   https://github.com/settings/tokens/new?description=goreleaser%3Ahomebrew-tap&scopes=repo

   - Pick an expiration (90 days is reasonable; rotate then).
   - Click **Generate token** and copy it — GitHub will only show it once.

2. **Store it as a repo secret on GoLogX** in one paste:

   ```bash
   gh secret set HOMEBREW_TAP_GITHUB_TOKEN \
     --repo AyoubTadlaoui/GoLogX \
     --body "PASTE_PAT_HERE"
   ```

   Or via the web UI:
   Settings → Secrets and variables → Actions → New repository secret.

3. **Verify** by checking the secret is present (the value is not retrievable
   — `gh` only confirms it exists):

   ```bash
   gh secret list --repo AyoubTadlaoui/GoLogX | grep HOMEBREW_TAP_GITHUB_TOKEN
   ```

4. **First tag after this lands** — release workflow's `goreleaser` step
   should push a `logx: bump formula to vX.Y.Z` commit to
   `AyoubTadlaoui/homebrew-tap`.

If the secret is missing, the rest of the release still publishes (GHCR
image + GitHub Release binaries); only the formula-push step is skipped.

### GHCR image visibility

The `ghcr.io/ayoubtadlaoui/logx` package is **public** (one-time flip done in v0.1.2).
Anonymous `docker pull ghcr.io/ayoubtadlaoui/logx:<version>` works without
`docker login`. Verified end-to-end against the v0.1.2 release manifest.

If you ever recreate the package (e.g. after a delete), it will land private
again. To re-flip:

```bash
# via the API (requires `write:packages` scope on your gh token —
# add it with: gh auth refresh -h github.com -s write:packages,read:packages)
gh api -X PATCH /user/packages/container/logx -f visibility=public

# or via the UI
# https://github.com/users/AyoubTadlaoui/packages/container/logx/settings
# → Danger Zone → Change package visibility → Public
```

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
