#!/usr/bin/env sh
#
# gen.sh — regenerate the README screenshots.
#
# Requirements (install once):
#   brew install vhs ffmpeg webp
#
# Run from the repo root:
#   sh docs/screenshots/gen.sh
#
# Produces:
#   docs/screenshots/hero.png   — static (vhs Screenshot directive)
#   docs/screenshots/follow.gif — animated GIF (vhs Output)
#   docs/screenshots/follow.mp4 — same animation re-encoded as MP4 (Safari
#                                  renders it reliably even when GIF playback
#                                  is gated by accessibility settings; the
#                                  README uses <video> with the MP4 as the
#                                  primary source and the GIF as fallback)
#
# Theme: atlas-ragnarok — https://github.com/AyoubTadlaoui/atlas-ragnarok
# The same palette is inlined into both hero.tape and follow.tape (vhs's
# Set Theme controls the actual ANSI palette; freeze does not).

set -eu

cd "$(dirname "$0")/../.."   # repo root
HERE="docs/screenshots"

command -v logx   >/dev/null 2>&1 || { echo "logx not on PATH — see DISTRIBUTION.md" >&2; exit 1; }
command -v vhs    >/dev/null 2>&1 || { echo "vhs not installed (brew install vhs)" >&2; exit 1; }
command -v ffmpeg   >/dev/null 2>&1 || { echo "ffmpeg not installed (brew install ffmpeg)" >&2; exit 1; }
command -v gif2webp >/dev/null 2>&1 || { echo "gif2webp not installed (brew install webp)" >&2; exit 1; }

echo "→ hero.png (vhs Screenshot, atlas-ragnarok theme)"
vhs "${HERE}/hero.tape"

echo "→ follow.gif (vhs Output, atlas-ragnarok theme)"
vhs "${HERE}/follow.tape"

echo "→ follow.webp (animated WebP — what the README actually embeds)"
# Animated WebP renders as <img> in markdown, bypassing GitHub's strict
# README sanitizer (which strips <video>), and is widely supported,
# including modern Safari. Much smaller than the source GIF.
# Use gif2webp (from libwebp / `brew install webp`) — ffmpeg's libwebp
# encoder is not always present in macOS bottles.
gif2webp -quiet -q 80 -m 6 -mt -mixed \
  "${HERE}/follow.gif" \
  -o "${HERE}/follow.webp"

echo "→ follow.mp4 (kept as a supplementary download)"
ffmpeg -hide_banner -loglevel error -y \
  -i "${HERE}/follow.gif" \
  -movflags +faststart \
  -pix_fmt yuv420p \
  -vf "pad=ceil(iw/2)*2:ceil(ih/2)*2" \
  -c:v libx264 -preset slow -crf 22 \
  "${HERE}/follow.mp4"

echo "done."
echo "  hero.png    $(du -h ${HERE}/hero.png   | cut -f1)"
echo "  follow.webp $(du -h ${HERE}/follow.webp | cut -f1)  ← embedded in README"
echo "  follow.gif $(du -h ${HERE}/follow.gif | cut -f1)"
echo "  follow.mp4 $(du -h ${HERE}/follow.mp4 | cut -f1)"
