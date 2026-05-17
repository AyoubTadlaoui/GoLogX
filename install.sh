#!/usr/bin/env sh
#
# install.sh — download and install the logx CLI from GoLogX releases.
#
#   curl -sSL https://raw.githubusercontent.com/AyoubTadlaoui/GoLogX/main/install.sh | sh
#
# Env knobs:
#   VERSION       Tag to install (default: latest). Example: VERSION=v0.1.5
#   INSTALL_DIR   Where to drop the binary (default: /usr/local/bin if writable,
#                 else $HOME/.local/bin). Example: INSTALL_DIR=$HOME/bin
#
# Verifies the SHA256 checksum from the release before installing. POSIX sh —
# works on bash, dash, busybox ash. Linux + macOS, amd64 + arm64.

set -eu

REPO='AyoubTadlaoui/GoLogX'
BIN='logx'

# -------- helpers ----------------------------------------------------------

err() { printf 'install.sh: %s\n' "$*" >&2; exit 1; }
info() { printf '%s\n' "$*" >&2; }

need() {
  command -v "$1" >/dev/null 2>&1 || err "missing required tool: $1"
}

# Pick the first available SHA256 implementation.
sha256_of() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  elif command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    err 'no SHA256 tool found (need shasum or sha256sum)'
  fi
}

# -------- detect OS and arch ----------------------------------------------

uname_s=$(uname -s)
uname_m=$(uname -m)

case "$uname_s" in
  Linux)  OS=linux ;;
  Darwin) OS=darwin ;;
  *)      err "unsupported OS: $uname_s (linux and darwin only — Windows users see DISTRIBUTION.md)" ;;
esac

case "$uname_m" in
  x86_64|amd64)   ARCH=x86_64 ;;
  arm64|aarch64)  ARCH=arm64 ;;
  *)              err "unsupported arch: $uname_m (amd64 and arm64 only)" ;;
esac

# -------- resolve version -------------------------------------------------

need curl
need tar
need mktemp
need awk

VERSION=${VERSION:-latest}
if [ "$VERSION" = latest ]; then
  info "resolving latest release..."
  VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | awk -F'"' '/"tag_name":/ {print $4; exit}')
  [ -n "${VERSION}" ] || err 'could not resolve latest tag'
fi
# strip leading v if present, then re-prepend (handles both "v0.1.5" and "0.1.5")
VER_NUMERIC=${VERSION#v}
TAG="v${VER_NUMERIC}"

# -------- resolve install dir ---------------------------------------------

if [ -z "${INSTALL_DIR:-}" ]; then
  if [ -w /usr/local/bin ] || ([ "$(id -u)" = 0 ] && [ -d /usr/local/bin ]); then
    INSTALL_DIR=/usr/local/bin
  else
    INSTALL_DIR="$HOME/.local/bin"
    mkdir -p "$INSTALL_DIR"
    case ":$PATH:" in
      *:"$INSTALL_DIR":*) ;;
      *) info "note: $INSTALL_DIR is not on \$PATH. Add it to your shell profile." ;;
    esac
  fi
fi

# -------- download + verify ----------------------------------------------

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT INT TERM

ARCHIVE="GoLogX_${VER_NUMERIC}_${OS}_${ARCH}.tar.gz"
URL_BASE="https://github.com/${REPO}/releases/download/${TAG}"
ARCHIVE_URL="${URL_BASE}/${ARCHIVE}"
CHECKSUMS_URL="${URL_BASE}/checksums.txt"

info "downloading ${ARCHIVE_URL}"
curl -fsSL -o "${TMP}/${ARCHIVE}" "${ARCHIVE_URL}" \
  || err "download failed: ${ARCHIVE_URL}"

info "verifying SHA256..."
curl -fsSL -o "${TMP}/checksums.txt" "${CHECKSUMS_URL}" \
  || err "could not download ${CHECKSUMS_URL}"
EXPECTED=$(awk -v file="${ARCHIVE}" '$2 == file {print $1; exit}' "${TMP}/checksums.txt")
[ -n "${EXPECTED}" ] || err "no checksum entry for ${ARCHIVE}"
ACTUAL=$(sha256_of "${TMP}/${ARCHIVE}")
[ "${EXPECTED}" = "${ACTUAL}" ] \
  || err "checksum mismatch for ${ARCHIVE}: expected ${EXPECTED}, got ${ACTUAL}"

# -------- extract + install ----------------------------------------------

info "extracting..."
tar -xzf "${TMP}/${ARCHIVE}" -C "${TMP}"
[ -f "${TMP}/${BIN}" ] || err "archive did not contain ${BIN}"

DEST="${INSTALL_DIR}/${BIN}"
info "installing to ${DEST}"
# Prefer atomic install via mv when possible; cp+chmod as fallback.
if mv "${TMP}/${BIN}" "${DEST}" 2>/dev/null; then
  :
else
  cp "${TMP}/${BIN}" "${DEST}"
fi
chmod +x "${DEST}"

# -------- done -----------------------------------------------------------

info "installed ${BIN} ${TAG} to ${DEST}"
"${DEST}" -version
