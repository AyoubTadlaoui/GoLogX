#!/usr/bin/env sh
#
# gen.sh — regenerate the README screenshots.
#
# Requirements (install once):
#   brew install vhs
#   go install github.com/charmbracelet/freeze@latest
#   (the freeze binary lands in $(go env GOPATH)/bin — add to your PATH)
#
# Run from the repo root:
#   sh docs/screenshots/gen.sh
#
# Produces:
#   docs/screenshots/hero.png   — static, for README hero shot
#   docs/screenshots/follow.gif — animated, shows `logx -f` tailing live JSON
#
# The CLI used is whichever `logx` is on $PATH (brew/scoop/aur/go-installed —
# all the same binary). Tags/versions are not baked into the images, so the
# same script regenerates them for every release.

set -eu

cd "$(dirname "$0")/../.."   # repo root
HERE="docs/screenshots"

command -v logx   >/dev/null 2>&1 || { echo "logx not on PATH — see DISTRIBUTION.md" >&2; exit 1; }
command -v vhs    >/dev/null 2>&1 || { echo "vhs not installed (brew install vhs)" >&2; exit 1; }
command -v freeze >/dev/null 2>&1 || { echo "freeze not installed (go install github.com/charmbracelet/freeze@latest)" >&2; exit 1; }

echo "→ hero.png (freeze)"
freeze --execute "logx -level=debug ${HERE}/sample.json" \
  --output "${HERE}/hero.png" \
  --window \
  --background "#0f111a" \
  --border.radius 8 \
  --padding 24 \
  --margin 18 \
  --font.size 14 \
  --width 1100 \
  --shadow.blur 24 --shadow.x 0 --shadow.y 14

echo "→ follow.gif (vhs)"
vhs "${HERE}/follow.tape"

echo "done. Inspect ${HERE}/hero.png and ${HERE}/follow.gif"
