#!/usr/bin/env sh
#
# gen.sh — regenerate the README screenshots.
#
# Requirements (install once):
#   brew install vhs ffmpeg webp
#   pip3 install pillow            # for the vignette overlay generator
#
# Run from the repo root:
#   sh docs/screenshots/gen.sh
#
# Produces:
#   docs/screenshots/hero.png   — static, atlas-ragnarok shader vignette
#                                  composited on top (matches the author's
#                                  real Ghostty render).
#   docs/screenshots/follow.gif — animated GIF, same vignette on every frame.
#   docs/screenshots/follow.webp — animated WebP (README embed).
#   docs/screenshots/follow.mp4  — H.264 MP4 (supplementary download).
#
# Theme + vignette: atlas-ragnarok
#   https://github.com/AyoubTadlaoui/atlas-ragnarok
# The vignette PNG (_vignette.py) reproduces the GLSL shader's geometry
# and colors in 2D; vhs/ttyd can't run the live shader, so we composite
# the equivalent gradient on every frame in post.

set -eu

cd "$(dirname "$0")/../.."   # repo root
HERE="docs/screenshots"

command -v logx     >/dev/null 2>&1 || { echo "logx not on PATH — see DISTRIBUTION.md" >&2; exit 1; }
command -v vhs      >/dev/null 2>&1 || { echo "vhs not installed (brew install vhs)" >&2; exit 1; }
command -v ffmpeg   >/dev/null 2>&1 || { echo "ffmpeg not installed (brew install ffmpeg)" >&2; exit 1; }
command -v ffprobe  >/dev/null 2>&1 || { echo "ffprobe not installed (comes with ffmpeg)" >&2; exit 1; }
command -v gif2webp >/dev/null 2>&1 || { echo "gif2webp not installed (brew install webp)" >&2; exit 1; }
command -v python3  >/dev/null 2>&1 || { echo "python3 not installed" >&2; exit 1; }

# --- static hero --------------------------------------------------------
echo "→ raw hero.png (vhs Screenshot, atlas-ragnarok base palette)"
vhs "${HERE}/hero.tape"

echo "→ build vignette overlay matching hero dimensions"
DIMS=$(python3 -c "from PIL import Image; im=Image.open('${HERE}/_hero_raw.png'); print(f'{im.width}x{im.height}')")
HW=$(echo "$DIMS" | cut -dx -f1)
HH=$(echo "$DIMS" | cut -dx -f2)
echo "    hero dims: ${HW}x${HH}"
python3 "${HERE}/_vignette.py" "$HW" "$HH" "${HERE}/_vignette_hero.png"

echo "→ composite vignette onto hero.png"
ffmpeg -hide_banner -loglevel error -y \
  -i "${HERE}/_hero_raw.png" \
  -i "${HERE}/_vignette_hero.png" \
  -filter_complex "[0:v][1:v]overlay=0:0" \
  "${HERE}/hero.png"

# --- animated demo ------------------------------------------------------
echo "→ raw follow.gif (vhs Output, atlas-ragnarok base palette)"
vhs "${HERE}/follow.tape"

echo "→ build vignette overlay matching follow dimensions"
DIMS=$(ffprobe -hide_banner -loglevel error -select_streams v:0 \
  -show_entries stream=width,height -of csv=p=0:s=x "${HERE}/_follow_raw.gif")
FW=$(echo "$DIMS" | cut -dx -f1)
FH=$(echo "$DIMS" | cut -dx -f2)
echo "    follow dims: ${FW}x${FH}"
python3 "${HERE}/_vignette.py" "$FW" "$FH" "${HERE}/_vignette_follow.png"

echo "→ composite vignette over every frame"
ffmpeg -hide_banner -loglevel error -y \
  -i "${HERE}/_follow_raw.gif" \
  -i "${HERE}/_vignette_follow.png" \
  -filter_complex "[0:v][1:v]overlay=0:0,split[s0][s1];[s0]palettegen=stats_mode=full[p];[s1][p]paletteuse=dither=bayer:bayer_scale=5" \
  -loop 0 \
  "${HERE}/follow.gif"

echo "→ follow.webp (animated WebP, README embed)"
gif2webp -quiet -q 80 -m 6 -mt -mixed \
  "${HERE}/follow.gif" \
  -o "${HERE}/follow.webp"

echo "→ follow.mp4 (supplementary download)"
ffmpeg -hide_banner -loglevel error -y \
  -i "${HERE}/follow.gif" \
  -movflags +faststart \
  -pix_fmt yuv420p \
  -vf "pad=ceil(iw/2)*2:ceil(ih/2)*2" \
  -c:v libx264 -preset slow -crf 22 \
  "${HERE}/follow.mp4"

# Drop intermediates.
rm -f "${HERE}/_hero_raw.png" "${HERE}/_follow_raw.gif" \
      "${HERE}/_vignette_hero.png" "${HERE}/_vignette_follow.png"

echo "done."
echo "  hero.png    $(du -h ${HERE}/hero.png    | cut -f1)"
echo "  follow.webp $(du -h ${HERE}/follow.webp | cut -f1)  ← embedded in README"
echo "  follow.gif  $(du -h ${HERE}/follow.gif  | cut -f1)"
echo "  follow.mp4  $(du -h ${HERE}/follow.mp4  | cut -f1)"
