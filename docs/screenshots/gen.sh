#!/usr/bin/env sh
#
# gen.sh — regenerate the README screenshots.
#
# Requirements (install once):
#   brew install vhs ffmpeg webp
#   pip3 install --user --break-system-packages pillow numpy
#
# Run from the repo root:
#   sh docs/screenshots/gen.sh
#
# Produces:
#   docs/screenshots/hero.png    — static, atlas-ragnarok shader applied
#                                   per pixel (luma mask keeps text crisp).
#   docs/screenshots/follow.gif  — animated GIF, same shader on every frame.
#   docs/screenshots/follow.webp — animated WebP (README embed).
#   docs/screenshots/follow.mp4  — H.264 MP4 (supplementary download).
#
# Theme + shader: atlas-ragnarok
#   https://github.com/AyoubTadlaoui/atlas-ragnarok
# The shader equation (geometry + colors + luminance mask) is
# reimplemented in docs/screenshots/_shader.py and applied to every
# frame in post; vhs/ttyd can't run the GPU shader directly.

set -eu

cd "$(dirname "$0")/../.."   # repo root
HERE="docs/screenshots"

command -v logx     >/dev/null 2>&1 || { echo "logx not on PATH — see DISTRIBUTION.md" >&2; exit 1; }
command -v vhs      >/dev/null 2>&1 || { echo "vhs not installed (brew install vhs)" >&2; exit 1; }
command -v ffmpeg   >/dev/null 2>&1 || { echo "ffmpeg not installed (brew install ffmpeg)" >&2; exit 1; }
command -v gif2webp >/dev/null 2>&1 || { echo "gif2webp not installed (brew install webp)" >&2; exit 1; }
command -v python3  >/dev/null 2>&1 || { echo "python3 not installed" >&2; exit 1; }
python3 -c "import numpy, PIL" 2>/dev/null \
  || { echo "missing python deps (pip3 install --user --break-system-packages pillow numpy)" >&2; exit 1; }

# --- static hero --------------------------------------------------------
echo "→ raw hero.png (vhs Screenshot, atlas-ragnarok base palette)"
vhs "${HERE}/hero.tape"

echo "→ apply GLSL shader to hero (luma mask keeps text crisp)"
python3 "${HERE}/_shader.py" "${HERE}/_hero_raw.png" "${HERE}/hero.png"

# --- animated demo ------------------------------------------------------
echo "→ raw follow.gif (vhs Output, atlas-ragnarok base palette)"
vhs "${HERE}/follow.tape"

echo "→ apply GLSL shader to every frame"
rm -rf "${HERE}/_frames"
python3 "${HERE}/_shader.py" "${HERE}/_follow_raw.gif" "${HERE}/_frames"

# PIL reports per-frame duration in milliseconds. Average → fps for ffmpeg.
AVG_MS=$(awk '{s+=$1; n++} END {printf "%d", (s/n) + 0.5}' "${HERE}/_frames/duration.txt")
FPS=$(awk -v ms="$AVG_MS" 'BEGIN {if (ms <= 0) ms = 100; printf "%g", 1000.0 / ms}')
echo "    avg frame delay ${AVG_MS}ms (${FPS} fps)"

echo "→ reassemble shaded GIF (ffmpeg palettegen for cross-frame delta compression)"
ffmpeg -hide_banner -loglevel error -y \
  -framerate "$FPS" -i "${HERE}/_frames/frame_%05d.png" \
  -filter_complex "split[s0][s1];[s0]palettegen=stats_mode=full[p];[s1][p]paletteuse=dither=bayer:bayer_scale=5" \
  -loop 0 \
  "${HERE}/follow.gif"

echo "→ follow.webp (animated WebP, README embed)"
gif2webp -quiet -q 80 -m 6 -mt -mixed \
  "${HERE}/follow.gif" \
  -o "${HERE}/follow.webp"

echo "→ follow.mp4 (supplementary download)"
ffmpeg -hide_banner -loglevel error -y \
  -framerate "$FPS" -i "${HERE}/_frames/frame_%05d.png" \
  -movflags +faststart \
  -pix_fmt yuv420p \
  -vf "pad=ceil(iw/2)*2:ceil(ih/2)*2" \
  -c:v libx264 -preset slow -crf 22 \
  "${HERE}/follow.mp4"

# Drop intermediates.
rm -rf "${HERE}/_frames"
rm -f "${HERE}/_hero_raw.png" "${HERE}/_follow_raw.gif"

echo "done."
echo "  hero.png    $(du -h ${HERE}/hero.png    | cut -f1)"
echo "  follow.webp $(du -h ${HERE}/follow.webp | cut -f1)  ← embedded in README"
echo "  follow.gif  $(du -h ${HERE}/follow.gif  | cut -f1)"
echo "  follow.mp4  $(du -h ${HERE}/follow.mp4  | cut -f1)"
