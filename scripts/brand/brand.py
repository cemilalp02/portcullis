"""Writes the Portcullis brand assets into docs/assets/brand: the mark in two
optical sizes, the two-tone logo lockups (wordmark set in IBM Plex Sans
SemiBold and converted to outlines, so no font is needed to display them),
the app icon, the favicon and the social card. See docs/BRAND.md.

Usage: python scripts/brand/brand.py PLEX_DIR docs/assets/brand
PLEX_DIR holds IBMPlexSans-SemiBold.woff2 and IBMPlexSans-Regular.woff2, the
complete woff2 files from the @ibm/plex-sans npm package (fonts/complete/woff2).
"""
import io
import os
import sys

import uharfbuzz as hb
from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen
from fontTools.ttLib import TTFont

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from mark import LARGE, SMALL, fmt, mark, parts, svg_d  # noqa: E402

INK = "#14181e"
IVORY = "#efe9dc"
# Verdigris, the green of weathered copper: the grille of the two-tone mark.
GRILLE_LIGHT = ("#14b8a6", "#0f766e")  # on light backgrounds
GRILLE_DARK = ("#7ff0dc", "#14b8a6", "#0f766e")  # on dark backgrounds and the icon
TILE = ("#232b36", "#0b0e12")
GLOW = "#2dd4bf"


def gradient(gid, stops, x2=0, y2=1):
    n = len(stops)
    s = "".join(f'<stop offset="{fmt(i / (n - 1))}" stop-color="{c}"/>' for i, c in enumerate(stops))
    return f'<linearGradient id="{gid}" x1="0" y1="0" x2="{x2}" y2="{y2}">{s}</linearGradient>'


def mark_svg(path, title="Portcullis"):
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" role="img" aria-label="{title}">'
            f'<path fill="currentColor" d="{svg_d(path)}"/></svg>\n')


def shape(font_path, text, size, tracking=-0.012):
    """Return (svg path d, advance width, cap height) for `text` at `size`
    px, baseline at y=0."""
    tt = TTFont(font_path)
    tt.flavor = None
    buf = io.BytesIO()
    tt.save(buf)
    data = buf.getvalue()
    tt = TTFont(io.BytesIO(data))
    upem = tt["head"].unitsPerEm
    font = hb.Font(hb.Face(data))
    b = hb.Buffer()
    b.add_str(text)
    b.guess_segment_properties()
    hb.shape(font, b, {"kern": True, "liga": False})
    gs = tt.getGlyphSet()
    order = tt.getGlyphOrder()
    s = size / upem
    pen = SVGPathPen(gs, ntos=fmt)
    x = 0.0
    for info, pos in zip(b.glyph_infos, b.glyph_positions):
        gs[order[info.codepoint]].draw(TransformPen(pen, (s, 0, 0, -s, x + pos.x_offset * s, -pos.y_offset * s)))
        x += pos.x_advance * s + tracking * size
    x -= tracking * size
    return pen.getCommands(), x, tt["OS/2"].sCapHeight * s


def two_tone(frame_fill, grille_stops, gid, transform):
    frame, grille = parts(**LARGE)
    return (f'<defs>{gradient(gid, grille_stops)}</defs>'
            f'<path fill="{frame_fill}" transform="{transform}" d="{svg_d(frame)}"/>'
            f'<path fill="url(#{gid})" transform="{transform}" d="{svg_d(grille)}"/>')


def lockup(semibold, dark):
    size = 40.0
    d, adv, cap = shape(semibold, "Portcullis", size)
    mx0, my0, mx1, my1 = mark(**LARGE).bounds
    mh = cap * 1.42  # the mark stands a little taller than the capitals
    k = mh / (my1 - my0)
    gap = size * 0.34
    mw = (mx1 - mx0) * k
    baseline = (mh + cap) / 2  # centre the capitals on the mark
    width = mw + gap + adv
    color = IVORY if dark else INK
    gid = "pcl-logo-dark" if dark else "pcl-logo"
    t = f"matrix({fmt(k)} 0 0 {fmt(k)} {fmt(-mx0 * k)} {fmt(-my0 * k)})"
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {fmt(width)} {fmt(mh)}" role="img" aria-label="Portcullis">'
            + two_tone(color, GRILLE_DARK if dark else GRILLE_LIGHT, gid, t)
            + f'<path fill="{color}" transform="translate({fmt(mw + gap)} {fmt(baseline)})" d="{d}"/></svg>\n')


def tile(prefix, optical, height=36):
    """App icon: the two-tone mark on a dark tile with a soft glow behind
    the grille and a hairline edge, on a 64-unit grid."""
    frame, grille = parts(**optical)
    x0, y0, x1, y1 = mark(**optical).bounds
    k = height / (y1 - y0)
    tx = 32 - (x0 + x1) / 2 * k
    ty = 32 - (y0 + y1) / 2 * k + 0.6
    t = f"matrix({fmt(k)} 0 0 {fmt(k)} {fmt(tx)} {fmt(ty)})"
    return (f'<defs>{gradient(prefix + "-tile", TILE)}{gradient(prefix + "-grille", GRILLE_DARK)}'
            f'<radialGradient id="{prefix}-glow" cx="0.5" cy="0.56" r="0.5">'
            f'<stop offset="0" stop-color="{GLOW}" stop-opacity="0.38"/><stop offset="1" stop-color="{GLOW}" stop-opacity="0"/>'
            f'</radialGradient></defs>'
            f'<rect width="64" height="64" rx="14" fill="url(#{prefix}-tile)"/>'
            f'<rect width="64" height="64" rx="14" fill="url(#{prefix}-glow)"/>'
            f'<rect x="0.5" y="0.5" width="63" height="63" rx="13.5" fill="none" stroke="#fff" stroke-opacity="0.12"/>'
            f'<path fill="#f3eee3" transform="{t}" d="{svg_d(frame)}"/>'
            f'<path fill="url(#{prefix}-grille)" transform="{t}" d="{svg_d(grille)}"/>')


def icon():
    return f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" role="img" aria-label="Portcullis">{tile("pcl-icon", LARGE)}</svg>\n'


def favicon():
    """Browser tabs: the icon with the small optical size of the mark, which
    stays readable at 16 px. The tile is dark in both colour schemes."""
    return f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64">{tile("pcl-fav", SMALL, height=40)}</svg>\n'


def social(semibold, regular):
    """1280 x 640 card for link previews (GitHub social preview, chat apps)."""
    W, H = 1280, 640
    name, nadv, ncap = shape(semibold, "Portcullis", 112)
    line1, l1adv, _ = shape(regular, "Scans the files AI coding agents trust", 38, tracking=0)
    line2, _, _ = shape(regular, "CLAUDE.md · AGENTS.md · skills · hooks · MCP servers", 27, tracking=0)
    url, _, _ = shape(regular, "github.com/cemilalp02/portcullis", 24, tracking=0)
    s = 4.0  # icon at 256 px
    ix, iy = 120, (H - 64 * s) / 2
    tx = ix + 64 * s + 72
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" role="img" aria-label="Portcullis: scans the files AI coding agents trust">'
            f'<defs><radialGradient id="pcl-sc-glow" cx="0.2" cy="0.5" r="0.6">'
            f'<stop offset="0" stop-color="{GLOW}" stop-opacity="0.22"/><stop offset="1" stop-color="{GLOW}" stop-opacity="0"/></radialGradient>'
            f'<pattern id="pcl-sc-grid" width="40" height="40" patternUnits="userSpaceOnUse"><path d="M40 0H0V40" fill="none" stroke="#fff" stroke-opacity="0.035"/></pattern></defs>'
            f'<rect width="{W}" height="{H}" fill="#0a0c10"/>'
            f'<rect width="{W}" height="{H}" fill="url(#pcl-sc-grid)"/>'
            f'<rect width="{W}" height="{H}" fill="url(#pcl-sc-glow)"/>'
            f'<g transform="translate({fmt(ix)} {fmt(iy)}) scale({fmt(s)})">{tile("pcl-sc", LARGE)}</g>'
            f'<path fill="{IVORY}" transform="translate({tx} 300)" d="{name}"/>'
            f'<path fill="#b9c1cc" transform="translate({tx + 4} 372)" d="{line1}"/>'
            f'<path fill="{GRILLE_DARK[1]}" transform="translate({tx + 4} 426)" d="{line2}"/>'
            f'<path fill="#6b7480" transform="translate({tx + 4} 560)" d="{url}"/>'
            f'</svg>\n')


if __name__ == "__main__":
    plex, out = sys.argv[1], sys.argv[2]
    semibold = os.path.join(plex, "IBMPlexSans-SemiBold.woff2")
    regular = os.path.join(plex, "IBMPlexSans-Regular.woff2")
    os.makedirs(out, exist_ok=True)
    files = {
        "portcullis-mark.svg": mark_svg(mark(**LARGE)),
        "portcullis-mark-small.svg": mark_svg(mark(**SMALL)),
        "portcullis-logo.svg": lockup(semibold, dark=False),
        "portcullis-logo-dark.svg": lockup(semibold, dark=True),
        "portcullis-icon.svg": icon(),
        "favicon.svg": favicon(),
        "portcullis-social.svg": social(semibold, regular),
    }
    for name, text in files.items():
        with open(os.path.join(out, name), "w", encoding="utf-8", newline="\n") as f:
            f.write(text)
        print(f"{name}: {len(text)} bytes")
    # The HTML report embeds the app icon (internal/report/html/icon.svg) and
    # the favicon (internal/report/html/favicon.svg); a test keeps them
    # identical to these files.
