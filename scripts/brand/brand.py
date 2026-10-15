"""Writes the Portcullis brand assets into docs/assets/brand: the mark in
two optical sizes, the logo lockups (wordmark set in IBM Plex Sans SemiBold
and converted to outlines, so no font is needed to display them), the app
icon and the favicon. See docs/BRAND.md.

Usage: python scripts/brand/brand.py IBMPlexSans-SemiBold.woff2 docs/assets/brand
(the font is the complete woff2 from the @ibm/plex-sans npm package)
"""
import io
import os
import sys

import uharfbuzz as hb
from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen
from fontTools.ttLib import TTFont

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from mark import LARGE, SMALL, fmt, mark, svg_d  # noqa: E402

INK = "#14181e"
IVORY = "#efe9dc"


def mark_svg(path, color=None, title="Portcullis"):
    fill = f' fill="{color}"' if color else ' fill="currentColor"'
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" role="img" aria-label="{title}">'
            f'<path{fill} d="{svg_d(path)}"/></svg>\n')


def wordmark(font_path, size, tracking=-0.012):
    """Return (svg path d, advance width) for 'Portcullis' at `size` px,
    baseline at y=0."""
    tt = TTFont(font_path)
    tt.flavor = None
    buf = io.BytesIO()
    tt.save(buf)
    data = buf.getvalue()
    tt = TTFont(io.BytesIO(data))
    upem = tt["head"].unitsPerEm
    face = hb.Face(data)
    font = hb.Font(face)
    b = hb.Buffer()
    b.add_str("Portcullis")
    b.guess_segment_properties()
    hb.shape(font, b, {"kern": True, "liga": False})
    gs = tt.getGlyphSet()
    order = tt.getGlyphOrder()
    s = size / upem
    pen = SVGPathPen(gs, ntos=fmt)
    x = 0.0
    for info, pos in zip(b.glyph_infos, b.glyph_positions):
        name = order[info.codepoint]
        gs[name].draw(TransformPen(pen, (s, 0, 0, -s, x + pos.x_offset * s, -pos.y_offset * s)))
        x += pos.x_advance * s + tracking * size
    x -= tracking * size
    cap = tt["OS/2"].sCapHeight * s
    return pen.getCommands(), x, cap


def lockup(font_path, color):
    size = 40.0
    d, adv, cap = wordmark(font_path, size)
    m = mark(**LARGE)
    mx0, my0, mx1, my1 = m.bounds
    mh = cap * 1.42  # the mark stands a little taller than the capitals
    k = mh / (my1 - my0)
    gap = size * 0.34
    mw = (mx1 - mx0) * k
    height = mh
    baseline = (height + cap) / 2  # centre the capitals on the mark
    width = mw + gap + adv
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {fmt(width)} {fmt(height)}" role="img" aria-label="Portcullis">'
            f'<g fill="{color}"><path transform="matrix({fmt(k)} 0 0 {fmt(k)} {fmt(-mx0 * k)} {fmt(-my0 * k)})" d="{svg_d(m)}"/>'
            f'<path transform="translate({fmt(mw + gap)} {fmt(baseline)})" d="{d}"/></g></svg>\n')


def icon():
    m = mark(**LARGE)
    x0, y0, x1, y1 = m.bounds
    k = 36 / (y1 - y0)
    tx = 32 - (x0 + x1) / 2 * k
    ty = 32 - (y0 + y1) / 2 * k + 0.6
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" role="img" aria-label="Portcullis">'
            f'<rect width="64" height="64" rx="14" fill="{INK}"/>'
            f'<path fill="{IVORY}" transform="matrix({fmt(k)} 0 0 {fmt(k)} {fmt(tx)} {fmt(ty)})" d="{svg_d(m)}"/></svg>\n')


def favicon():
    """Small mark that follows the browser's colour scheme."""
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64">'
            f'<style>path{{fill:{INK}}}@media (prefers-color-scheme:dark){{path{{fill:{IVORY}}}}}</style>'
            f'<path d="{svg_d(mark(**SMALL))}"/></svg>\n')


if __name__ == "__main__":
    font_path, out = sys.argv[1], sys.argv[2]
    os.makedirs(out, exist_ok=True)
    files = {
        "portcullis-mark.svg": mark_svg(mark(**LARGE)),
        "portcullis-mark-small.svg": mark_svg(mark(**SMALL)),
        "portcullis-logo.svg": lockup(font_path, INK),
        "portcullis-logo-dark.svg": lockup(font_path, IVORY),
        "portcullis-icon.svg": icon(),
        "favicon.svg": favicon(),
    }
    for name, text in files.items():
        with open(os.path.join(out, name), "w", encoding="utf-8", newline="\n") as f:
            f.write(text)
        print(f"{name}: {len(text)} bytes")
    # The HTML report embeds the small mark (internal/report/html.go,
    # markPath) and the favicon (internal/report/html/favicon.svg); a test
    # keeps them identical to these files.
