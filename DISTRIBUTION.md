# Distribution

How GoLogX is shipped, by audience.

## Install matrix

| Channel | Audience | One-liner | Auto on each tag? |
|---|---|---|---|
| **Go module proxy / pkg.go.dev** | Library users | `go get github.com/AyoubTadlaoui/GoLogX/logx@vX.Y.Z` | ✓ (built into the Go ecosystem) |
| **`go install`** | CLI users on any Go-friendly machine | `go install github.com/AyoubTadlaoui/GoLogX/cmd/logx@vX.Y.Z` | ✓ (built into the Go ecosystem) |
| **Homebrew tap** | macOS + Linux CLI users | `brew install AyoubTadlaoui/tap/logx` | ✓ (goreleaser pushes to [`AyoubTadlaoui/homebrew-tap`](https://github.com/AyoubTadlaoui/homebrew-tap), uses `HOMEBREW_TAP_GITHUB_TOKEN` PAT) |
| **Scoop bucket** | Windows CLI users | `scoop bucket add atlas https://github.com/AyoubTadlaoui/scoop-bucket && scoop install logx` | ✓ (goreleaser pushes to [`AyoubTadlaoui/scoop-bucket`](https://github.com/AyoubTadlaoui/scoop-bucket), reuses the same PAT) |
| **WinGet** | Windows CLI users | `winget install AyoubTadlaoui.logx` | **Manual opt-in** — see [WinGet submission cadence](#winget-submission-cadence). Trigger via Actions → Release → "Run workflow" with `submit_winget=true`. Goreleaser then pushes to your fork [`AyoubTadlaoui/winget-pkgs`](https://github.com/AyoubTadlaoui/winget-pkgs) and opens a PR to [`microsoft/winget-pkgs`](https://github.com/microsoft/winget-pkgs). |
| **GitHub Releases binaries** | All OSes, no Go required | Download from [releases](https://github.com/AyoubTadlaoui/GoLogX/releases) | ✓ (goreleaser, `GITHUB_TOKEN`) |
| **.deb (Debian/Ubuntu)** | Debian-family Linux users | `sudo dpkg -i logx_X.Y.Z_linux_amd64.deb` (download from releases) | ✓ (goreleaser nfpms, attached to GH Release) |
| **.rpm (RHEL/Fedora/SUSE)** | RPM-family Linux users | `sudo rpm -i logx-X.Y.Z-1.x86_64.rpm` (download from releases) | ✓ (goreleaser nfpms, attached to GH Release) |
| **Universal install script** | Any Linux / macOS user, scriptable | `curl -fsSL https://raw.githubusercontent.com/AyoubTadlaoui/GoLogX/main/install.sh \| sh` | ✓ (script lives in repo, always pulls the latest GH Release) |
| **GHCR Docker image** | Container / CI users | `docker run --rm -i ghcr.io/ayoubtadlaoui/logx:X.Y.Z < log.json` | ✓ (multi-arch via goreleaser + buildx, `GITHUB_TOKEN` is enough) |
| **Arch AUR** | Arch Linux users | `yay -S logx-bin` (or `paru -S logx-bin`) | ✓ on each tag goreleaser SSHes to `aur@aur.archlinux.org` and pushes a fresh PKGBUILD to the `logx-bin` repo; requires the `AUR_KEY` repo secret (an SSH private key registered with the maintainer's AUR account) |
| **Nix flake** | Nix / NixOS users | `nix run github:AyoubTadlaoui/GoLogX` | ✓ self-hosted at the repo root (`flake.nix`); consumed directly from GitHub — no separate registry. Bump the `version` string in `flake.nix` on each release (kept in sync with goreleaser's tag) |

Roadmap channels not yet wired: Snap (cross-distro sandboxed), nixpkgs upstream (vs. just the flake), Chocolatey (Windows, paid).

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
5. Updates the Scoop manifest in `AyoubTadlaoui/scoop-bucket` (if the PAT secret is set).
6. Pushes a fresh PKGBUILD to AUR `logx-bin` (if `AUR_KEY` is set).

WinGet is **not** in that list — see below.

### WinGet submission cadence

WinGet submissions go through human review at `microsoft/winget-pkgs`. Auto-submitting on every `v*` tag floods Microsoft's queue with stale PRs that reviewers tend to skip wholesale (a single open PR per package is the norm). The pipeline therefore gates the WinGet pipe on a `WINGET_SUBMIT` env var that is **only set when you explicitly opt in**.

To submit a release to WinGet:

1. Make sure the tag is already cut and the regular release workflow finished cleanly (binaries on GH Release, etc.).
2. Go to **Actions → Release → Run workflow**.
3. Pick the branch or tag.
4. Tick **`submit_winget`**.
5. Click **Run workflow**.

That run sets `WINGET_SUBMIT=1`, goreleaser pushes the manifest to `AyoubTadlaoui/winget-pkgs`, and a PR opens against `microsoft/winget-pkgs`. Microsoft reviews the first PR per publisher (1–3 days); subsequent submissions for the same package often auto-merge.

Cadence rule of thumb: bundle several patch releases and submit one WinGet PR per batch, not one per tag.

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

### AUR SSH key (one-time, optional)

The AUR (Arch User Repository) doesn't use HTTPS+token like GitHub — every push
to `ssh://aur@aur.archlinux.org/<package>.git` is gated on SSH key auth tied
to your AUR account.

1. **Create an AUR account.** Free, no email confirmation needed:
   https://aur.archlinux.org/register

2. **Check the package name is available.** AUR is first-come-first-serve.
   Search for the slug you'll publish:
   https://aur.archlinux.org/packages?K=logx-bin&SB=n
   (As of this writing, `logx-bin` is unclaimed.)

3. **Generate a dedicated SSH keypair** so you can rotate or revoke without
   touching your other keys:

   ```bash
   ssh-keygen -t ed25519 -C "aur" -f ~/.ssh/aur_ed25519
   # No passphrase — goreleaser needs to use the key non-interactively.
   ```

4. **Register the public key.** Log into AUR, open your profile, and paste
   the contents of `~/.ssh/aur_ed25519.pub` into the **SSH Public Key** field.

5. **Verify SSH access:**

   ```bash
   ssh -i ~/.ssh/aur_ed25519 aur@aur.archlinux.org help
   # Should print AUR's command list, not a permission error.
   ```

6. **Store the private key as a GoLogX repo secret** in one paste:

   ```bash
   gh secret set AUR_KEY --repo AyoubTadlaoui/GoLogX < ~/.ssh/aur_ed25519
   ```

7. **First tag after this lands** — goreleaser will SSH to the AUR git
   server, push a fresh PKGBUILD, and end users can `yay -S logx-bin`.

If `AUR_KEY` is missing, the AUR pipe is skipped cleanly (the rest of the
release still ships).

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
TAG=v0.2.0   # the tag you just pushed

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
