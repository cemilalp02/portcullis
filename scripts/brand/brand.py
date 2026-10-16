"""Writes the Portcullis brand assets into docs/assets/brand: the mark in two
optical sizes, the logo lockups (wordmark set in IBM Plex Sans SemiBold and
converted to outlines, so no font is needed to display them), the app icon,
the favicon and the social card. Everything is flat: solid colours, no
gradients, glows or shadows. See docs/BRAND.md.

Usage: python scripts/brand/brand.py SANS_DIR MONO_DIR docs/assets/brand
SANS_DIR holds IBMPlexSans-SemiBold.woff2 and IBMPlexSans-Regular.woff2, MONO_DIR
IBMPlexMono-Regular.woff2: the complete woff2 files of the @ibm/plex-sans and
@ibm/plex-mono npm packages (fonts/complete/woff2).
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
PAPER = "#f4f1ea"
SIGNAL = "#c8461b"  # the caught character on light backgrounds
SIGNAL_DARK = "#ee7d4f"  # on dark backgrounds


def mark_color_svg():
    """The two-tone mark on its own: lines in ink, the caught character in
    the signal colour. The HTML report recolours both parts per theme."""
    lines, char = parts(**LARGE)
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" role="img" aria-label="Portcullis">'
            f'<path fill="{INK}" d="{svg_d(lines)}"/><path fill="{SIGNAL}" d="{svg_d(char)}"/></svg>' + chr(10))


def mark_svg(path, title="Portcullis"):
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" role="img" aria-label="{title}">'
            f'<path fill="currentColor" d="{svg_d(path)}"/></svg>\n')


def shape(font_path, text, size, tracking=-0.015):
    """Return (svg path d, advance width, x-height) for `text` at `size`
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
    return pen.getCommands(), x, tt["OS/2"].sxHeight * s


def two_tone(ink, signal, optical, transform):
    lines, char = parts(**optical)
    return (f'<path fill="{ink}" transform="{transform}" d="{svg_d(lines)}"/>'
            f'<path fill="{signal}" transform="{transform}" d="{svg_d(char)}"/>')


def lockup(semibold, dark):
    """Mark and lower-case wordmark; the mark is as tall as the ascenders."""
    size = 40.0
    d, adv, xh = shape(semibold, "portcullis", size)
    x0, y0, x1, y1 = mark(**LARGE).bounds
    mh = size * 0.74
    k = mh / (y1 - y0)
    gap = size * 0.3
    mw = (x1 - x0) * k
    baseline = mh  # the wordmark sits on the mark's bottom line
    width = mw + gap + adv
    descent = size * 0.22
    color = IVORY if dark else INK
    t = f"matrix({fmt(k)} 0 0 {fmt(k)} {fmt(-x0 * k)} {fmt(-y0 * k)})"
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {fmt(width)} {fmt(mh + descent)}" role="img" aria-label="Portcullis">'
            + two_tone(color, SIGNAL_DARK if dark else SIGNAL, LARGE, t)
            + f'<path fill="{color}" transform="translate({fmt(mw + gap)} {fmt(baseline)})" d="{d}"/></svg>\n')


def tile(optical, height):
    """The mark on a flat ink tile, on a 64-unit grid."""
    x0, y0, x1, y1 = mark(**optical).bounds
    k = height / max(x1 - x0, y1 - y0)
    tx = 32 - (x0 + x1) / 2 * k
    ty = 32 - (y0 + y1) / 2 * k
    t = f"matrix({fmt(k)} 0 0 {fmt(k)} {fmt(tx)} {fmt(ty)})"
    return f'<rect width="64" height="64" rx="14" fill="{INK}"/>' + two_tone(IVORY, SIGNAL_DARK, optical, t)


def icon():
    return f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" role="img" aria-label="Portcullis">{tile(LARGE, 38)}</svg>\n'


def favicon():
    """Browser tabs: the small optical size on the tile, readable at 16 px."""
    return f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64">{tile(SMALL, 44)}</svg>\n'


def social(semibold, regular, mono):
    """1280 x 640 card for link previews (GitHub social preview, chat apps)."""
    W, H = 1280, 640
    left = 96
    name, nw, _ = shape(semibold, "portcullis", 88)
    line1, _, _ = shape(regular, "Inspects what AI coding agents read,", 31, tracking=0)
    line2, _, _ = shape(regular, "before they read it.", 31, tracking=0)
    url, _, _ = shape(regular, "github.com/cemilalp02/portcullis", 22, tracking=0)
    x0, y0, x1, y1 = mark(**LARGE).bounds
    k = 66 / (y1 - y0)
    my = 200
    t = f"matrix({fmt(k)} 0 0 {fmt(k)} {fmt(left - x0 * k)} {fmt(my - y0 * k)})"
    mw = (x1 - x0) * k
    # Right: a few lines of a CLAUDE.md, one hiding characters a reader
    # cannot see, marked as the report marks them.
    px0, py0, px1, py1 = 736, 190, 1184, 488
    sx, sy, lh = px0 + 36, py0 + 104, 50
    _, cw, _ = shape(mono, "committing.", 23, tracking=0)
    tag, tw, _ = shape(mono, "U+E0073 +29", 16, tracking=0)
    panel = (f'<rect x="{px0}" y="{py0}" width="{px1 - px0}" height="{py1 - py0}" rx="8" fill="#fbfaf6" stroke="#d8d2c4" stroke-width="1.5"/>'
             f'<rect x="{px0}" y="{py0 + 56}" width="{px1 - px0}" height="1.5" fill="#d8d2c4"/>')
    head, _, _ = shape(mono, "CLAUDE.md", 18, tracking=0)
    panel += f'<path fill="#7b808a" transform="translate({px0 + 36} {py0 + 35})" d="{head}"/>'
    for i, text in enumerate(["# Project", "Run `make test` before", "committing.", "Never commit secrets."]):
        d, _, _ = shape(mono, text, 23, tracking=0)
        panel += f'<path fill="{INK}" transform="translate({sx} {sy + i * lh})" d="{d}"/>'
    bx, by = sx + cw + 12, sy + 2 * lh - 21
    panel += (f'<rect x="{fmt(bx)}" y="{by}" width="{fmt(tw + 20)}" height="28" rx="3" fill="none" stroke="{SIGNAL}" stroke-width="2.5"/>'
              f'<path fill="{SIGNAL}" transform="translate({fmt(bx + 10)} {by + 20})" d="{tag}"/>')
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" role="img" aria-label="Portcullis: inspects what AI coding agents read, before they read it">'
            f'<rect width="{W}" height="{H}" fill="{PAPER}"/>'
            + two_tone(INK, SIGNAL, LARGE, t)
            + f'<path fill="{INK}" transform="translate({fmt(left + mw + 26)} {my + 64})" d="{name}"/>'
            f'<path fill="#3d424b" transform="translate({left} 352)" d="{line1}"/>'
            f'<path fill="#3d424b" transform="translate({left} 396)" d="{line2}"/>'
            f'<rect x="{left}" y="452" width="56" height="4" fill="{SIGNAL}"/>'
            f'<path fill="#6b7079" transform="translate({left} 500)" d="{url}"/>'
            + panel + '</svg>\n')


if __name__ == "__main__":
    plex, plex_mono, out = sys.argv[1], sys.argv[2], sys.argv[3]
    semibold = os.path.join(plex, "IBMPlexSans-SemiBold.woff2")
    regular = os.path.join(plex, "IBMPlexSans-Regular.woff2")
    mono = os.path.join(plex_mono, "IBMPlexMono-Regular.woff2")
    os.makedirs(out, exist_ok=True)
    files = {
        "portcullis-mark.svg": mark_svg(mark(**LARGE)),
        "portcullis-mark-small.svg": mark_svg(mark(**SMALL)),
        "portcullis-mark-color.svg": mark_color_svg(),
        "portcullis-logo.svg": lockup(semibold, dark=False),
        "portcullis-logo-dark.svg": lockup(semibold, dark=True),
        "portcullis-icon.svg": icon(),
        "favicon.svg": favicon(),
        "portcullis-social.svg": social(semibold, regular, mono),
    }
    for name, text in files.items():
        with open(os.path.join(out, name), "w", encoding="utf-8", newline="\n") as f:
            f.write(text)
        print(f"{name}: {len(text)} bytes")
    # The HTML report embeds the two-tone mark (internal/report/html/mark.svg)
    # and the favicon (internal/report/html/favicon.svg); a test keeps them
    # identical to these files.
