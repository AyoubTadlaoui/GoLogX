#!/usr/bin/env python3
"""
Generate the atlas-ragnarok storm-fire vignette as a transparent PNG, sized
to match the demo recording. Reproduces the GLSL shader's behavior:

    thunder_blue = vec3(0.055, 0.115, 0.320)   # added in top ~30%
    red_tint     = vec3(0.220, 0.014, 0.028)   # added in bottom ~30%
    vignette     = smoothstep(0.18, 1.00, dist_from_center)
    top_only     = smoothstep(0.65, 0.85, 1.0 - uv.y)
    bottom_only  = smoothstep(0.65, 0.85, uv.y)

Output: screenshots/_vignette.png  (RGBA, same dims as the GIF/MP4).

Used by gen.sh — composited on every frame via ffmpeg overlay filter, so the
animated demo gains the same atmospheric glow the static hero captures from
Ghostty's native GLSL shader.

Usage:
    python3 screenshots/_vignette.py <width> <height> <output.png>
"""

import math
import sys
from PIL import Image


def smoothstep(edge0: float, edge1: float, x: float) -> float:
    t = max(0.0, min(1.0, (x - edge0) / (edge1 - edge0)))
    return t * t * (3.0 - 2.0 * t)


def build_vignette(width: int, height: int) -> Image.Image:
    img = Image.new("RGBA", (width, height), (0, 0, 0, 0))
    px = img.load()

    # Boost the shader's added-color magnitudes so the gradient survives
    # the small README image. We don't have Ghostty's luminance mask (which
    # protects text from being tinted), so keep the boost conservative —
    # the tradeoff is a slightly subtler glow than the live Ghostty render.
    BOOST = 2.2
    thunder_r = min(1.0, 0.055 * BOOST)
    thunder_g = min(1.0, 0.115 * BOOST)
    thunder_b = min(1.0, 0.320 * BOOST)
    red_r = min(1.0, 0.220 * BOOST)
    red_g = min(1.0, 0.014 * BOOST)
    red_b = min(1.0, 0.028 * BOOST)

    # Push the glow further to the edges so the readable middle is wider:
    # original shader uses smoothstep(0.65, 0.85); we tighten to (0.78, 0.97)
    # so the tint only fully kicks in at the outermost ~15% top/bottom.
    GLOW_START, GLOW_END = 0.78, 0.97

    cx, cy = 0.5, 0.5
    for y in range(height):
        uv_y = y / (height - 1)
        top_only = smoothstep(GLOW_START, GLOW_END, 1.0 - uv_y)
        bottom_only = smoothstep(GLOW_START, GLOW_END, uv_y)
        for x in range(width):
            uv_x = x / (width - 1)
            dist = math.sqrt((uv_x - cx) ** 2 + (uv_y - cy) ** 2)
            vignette = smoothstep(0.18, 1.00, dist)

            # Per-channel additive contribution.
            r = thunder_r * vignette * top_only + red_r * vignette * bottom_only
            g = thunder_g * vignette * top_only + red_g * vignette * bottom_only
            b = thunder_b * vignette * top_only + red_b * vignette * bottom_only

            # alpha = strongest contribution at this pixel
            a = max(r, g, b)
            if a <= 0.0:
                continue

            # normalize color by alpha so PIL's RGBA blend reproduces the
            # additive look against pure black; the composite later uses
            # `overlay` so this stays faithful to the shader's intent.
            px[x, y] = (
                int(round(r * 255 / a)),
                int(round(g * 255 / a)),
                int(round(b * 255 / a)),
                int(round(a * 255)),
            )
    return img


def main() -> None:
    if len(sys.argv) != 4:
        print(f"usage: {sys.argv[0]} <width> <height> <output.png>", file=sys.stderr)
        sys.exit(2)
    w, h, out = int(sys.argv[1]), int(sys.argv[2]), sys.argv[3]
    img = build_vignette(w, h)
    img.save(out, optimize=True)


if __name__ == "__main__":
    main()
