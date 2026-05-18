#!/usr/bin/env python3
"""
_shader.py — apply the atlas-ragnarok GLSL shader to a still image or
animated GIF, pixel-exact.

This is a 2D reimplementation of atlas-ragnarok.glsl. Every input pixel
gets the same treatment Ghostty's GPU shader would give it:

    uv         = pixel_xy / image_size
    dist       = length(uv - 0.5)
    vignette   = smoothstep(0.18, 1.00, dist)
    lum        = dot(rgb, [0.299, 0.587, 0.114])
    bg_mask    = 1.0 - smoothstep(0.04, 0.18, lum)     # text pixels never tinted
    top_only   = smoothstep(0.65, 0.85, 1 - uv.y)      # top ~30%
    bottom_only= smoothstep(0.65, 0.85, uv.y)          # bottom ~30%

    rgb += thunder_blue * vignette * bg_mask * top_only
    rgb += red_tint     * vignette * bg_mask * bottom_only

    thunder_blue = vec3(0.055, 0.115, 0.320)
    red_tint     = vec3(0.220, 0.014, 0.028)

Used by gen.sh on the static hero PNG and every frame of the animated
GIF. The luminance mask is what keeps text crisp — without it, the
naive overlay washes characters in the tinted bands.

Usage:
    python3 screenshots/_shader.py <input> <output>
    # PNG → PNG, or GIF → GIF (frame-by-frame).

Requirements: pillow, numpy.
"""

from __future__ import annotations

import sys
from pathlib import Path

import numpy as np
from PIL import Image, ImageSequence

# --- Shader constants (mirror atlas-ragnarok.glsl) -------------------------
THUNDER_BLUE = np.array([0.055, 0.115, 0.320], dtype=np.float32)
RED_TINT     = np.array([0.220, 0.014, 0.028], dtype=np.float32)
LUMA_COEF    = np.array([0.299, 0.587, 0.114], dtype=np.float32)

VIGNETTE_LO, VIGNETTE_HI = 0.18, 1.00
BAND_LO,     BAND_HI     = 0.65, 0.85
BGMASK_LO,   BGMASK_HI   = 0.04, 0.18


def smoothstep(edge0: float, edge1: float, x: np.ndarray) -> np.ndarray:
    t = np.clip((x - edge0) / (edge1 - edge0), 0.0, 1.0)
    return t * t * (3.0 - 2.0 * t)


def precompute_geometry(width: int, height: int):
    """Compute the four geometry masks that don't depend on pixel color."""
    yy, xx = np.mgrid[0:height, 0:width].astype(np.float32)
    uv_x = xx / max(width - 1, 1)
    uv_y = yy / max(height - 1, 1)
    dist = np.sqrt((uv_x - 0.5) ** 2 + (uv_y - 0.5) ** 2)
    vignette    = smoothstep(VIGNETTE_LO, VIGNETTE_HI, dist)
    top_only    = smoothstep(BAND_LO, BAND_HI, 1.0 - uv_y)
    bottom_only = smoothstep(BAND_LO, BAND_HI, uv_y)
    return vignette, top_only, bottom_only


def apply_shader(img: Image.Image, geom=None) -> Image.Image:
    """Apply the shader to a single PIL image (RGB or RGBA)."""
    mode = img.mode
    arr = np.asarray(img.convert("RGB"), dtype=np.float32) / 255.0
    h, w, _ = arr.shape
    if geom is None:
        geom = precompute_geometry(w, h)
    vignette, top_only, bottom_only = geom

    lum = arr @ LUMA_COEF
    bg_mask = 1.0 - smoothstep(BGMASK_LO, BGMASK_HI, lum)

    common = (vignette * bg_mask)[..., None]
    arr += THUNDER_BLUE * common * top_only[..., None]
    arr += RED_TINT     * common * bottom_only[..., None]

    arr = np.clip(arr * 255.0, 0.0, 255.0).astype(np.uint8)
    out = Image.fromarray(arr, mode="RGB")
    if mode == "RGBA":
        out = out.convert("RGBA")
    return out


def process_gif(in_path: Path, out_dir: Path) -> int:
    """
    Iterate a GIF's frames, shade each, and write each shaded frame as a
    PNG into out_dir as frame_00000.png, frame_00001.png, ...

    Also writes <out_dir>/duration.txt with each frame's duration in
    centiseconds (one per line) so the caller can reconstruct the
    original timing when assembling the GIF.

    Returns the number of frames written.

    Why PNG sequence and not direct GIF? PIL's GIF writer doesn't
    optimize cross-frame deltas well, so a 15s 10fps GIF balloons to
    multiple MB. Letting ffmpeg's palettegen/paletteuse do the encoding
    from a PNG sequence keeps the output around the same size as the
    raw vhs GIF.
    """
    src = Image.open(in_path)
    out_dir.mkdir(parents=True, exist_ok=True)
    geom = None
    durations = []
    count = 0
    for i, f in enumerate(ImageSequence.Iterator(src)):
        if geom is None:
            geom = precompute_geometry(f.width, f.height)
        shaded = apply_shader(f, geom=geom)
        shaded.save(out_dir / f"frame_{i:05d}.png", optimize=False)
        durations.append(f.info.get("duration", 100))
        count = i + 1
    (out_dir / "duration.txt").write_text("\n".join(str(d) for d in durations))
    return count


def main() -> None:
    if len(sys.argv) != 3:
        print(f"usage: {sys.argv[0]} <input.{{png,gif}}> <output>", file=sys.stderr)
        print(f"  PNG → PNG file", file=sys.stderr)
        print(f"  GIF → directory containing frame_NNNNN.png + duration.txt", file=sys.stderr)
        sys.exit(2)
    inp = Path(sys.argv[1])
    out = Path(sys.argv[2])
    suffix = inp.suffix.lower()
    if suffix == ".gif":
        n = process_gif(inp, out)
        print(f"wrote {n} shaded frames to {out}/")
    elif suffix in (".png", ".jpg", ".jpeg", ".webp"):
        img = Image.open(inp)
        apply_shader(img).save(out, optimize=True)
    else:
        print(f"unsupported input format: {suffix}", file=sys.stderr)
        sys.exit(2)


if __name__ == "__main__":
    main()
