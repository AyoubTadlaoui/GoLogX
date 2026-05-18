#!/usr/bin/env sh
#
# gen.sh — regenerate the README screenshots.
#
# Requirements (install once):
#   brew install vhs ffmpeg
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
command -v ffmpeg >/dev/null 2>&1 || { echo "ffmpeg not installed (brew install ffmpeg)" >&2; exit 1; }

echo "→ hero.png (vhs Screenshot, atlas-ragnarok theme)"
vhs "${HERE}/hero.tape"

echo "→ follow.gif (vhs Output, atlas-ragnarok theme)"
vhs "${HERE}/follow.tape"

echo "→ follow.mp4 (ffmpeg, Safari-friendly H.264 + faststart)"
# yuv420p needs even pixel dims, hence the pad filter.
ffmpeg -hide_banner -loglevel error -y \
  -i "${HERE}/follow.gif" \
  -movflags +faststart \
  -pix_fmt yuv420p \
  -vf "pad=ceil(iw/2)*2:ceil(ih/2)*2" \
  -c:v libx264 -preset slow -crf 22 \
  "${HERE}/follow.mp4"

echo "done."
echo "  hero.png   $(du -h ${HERE}/hero.png  | cut -f1)"
echo "  follow.gif $(du -h ${HERE}/follow.gif | cut -f1)"
echo "  follow.mp4 $(du -h ${HERE}/follow.mp4 | cut -f1)"
