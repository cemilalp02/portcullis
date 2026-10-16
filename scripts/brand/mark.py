"""Geometry of the Portcullis mark.

The mark shows what Portcullis does: three lines of text from a file an agent
will read, and in the middle line a character that should not be there,
caught and made visible. The lines are one colour (ink or ivory), the caught
character is the accent.

Coordinates are on a 64-unit grid, y pointing down. Corners are true
quarter circles; the parts are combined with boolean operations
(skia-pathops) into clean paths.
"""
import pathops

K = 0.5522847498  # cubic approximation of a quarter circle

# Optical sizes: LARGE for 32 px and up; SMALL keeps the mark readable at
# 16-24 px with heavier lines, a wider gap and a heavier outline.
LARGE = dict(line=8.0, rows=(10, 28, 46), lengths=(52, None, 38), box=(12.5, 17.0), stroke=3.6, gap=4.5)
SMALL = dict(line=10.0, rows=(6, 27, 48), lengths=(56, None, 40), box=(15.0, 20.0), stroke=4.6, gap=4.0)
LEFT = 6.0  # left edge of the text
RIGHT = 58.0  # right edge of the text


def rrect(x0, y0, x1, y1, r):
    """Rounded rectangle with corner radius r."""
    r = min(r, (x1 - x0) / 2, (y1 - y0) / 2)
    k = K * r
    p = pathops.Path()
    p.moveTo(x0 + r, y0)
    p.lineTo(x1 - r, y0)
    p.cubicTo(x1 - r + k, y0, x1, y0 + r - k, x1, y0 + r)
    p.lineTo(x1, y1 - r)
    p.cubicTo(x1, y1 - r + k, x1 - r + k, y1, x1 - r, y1)
    p.lineTo(x0 + r, y1)
    p.cubicTo(x0 + r - k, y1, x0, y1 - r + k, x0, y1 - r)
    p.lineTo(x0, y0 + r)
    p.cubicTo(x0, y0 + r - k, x0 + r - k, y0, x0 + r, y0)
    p.close()
    return p


def _union(paths):
    out = pathops.Path()
    for p in paths:
        out = pathops.op(out, p, pathops.PathOp.UNION)
    out.simplify(fix_winding=True)
    return out


def parts(line, rows, lengths, box, stroke, gap):
    """The text lines and the caught character, as two paths."""
    h = line
    top, mid, bottom = rows
    bw, bh = box
    cx = (LEFT + RIGHT) / 2
    bx0, bx1 = cx - bw / 2, cx + bw / 2
    by0 = mid + h / 2 - bh / 2
    by1 = by0 + bh
    lines = _union([
        rrect(LEFT, top, LEFT + lengths[0], top + h, h / 2),
        rrect(LEFT, mid, bx0 - gap, mid + h, h / 2),
        rrect(bx1 + gap, mid, RIGHT, mid + h, h / 2),
        rrect(LEFT, bottom, LEFT + lengths[2], bottom + h, h / 2),
    ])
    outer = rrect(bx0, by0, bx1, by1, 2.6)
    inner = rrect(bx0 + stroke, by0 + stroke, bx1 - stroke, by1 - stroke, 1.0)
    char = pathops.op(outer, inner, pathops.PathOp.DIFFERENCE)
    char.simplify(fix_winding=True)
    return lines, char


def mark(**kw):
    """The whole mark as one path, for one-colour use."""
    lines, char = parts(**kw)
    return _union([lines, char])


def fmt(v):
    return f"{v:.2f}".rstrip("0").rstrip(".")


def svg_d(p):
    """SVG path data of a pathops path."""
    out = []
    for verb, pts in p:
        if verb == pathops.PathVerb.MOVE:
            out.append("M" + " ".join(fmt(c) for c in pts[0]))
        elif verb == pathops.PathVerb.LINE:
            out.append("L" + " ".join(fmt(c) for c in pts[0]))
        elif verb == pathops.PathVerb.CUBIC:
            out.append("C" + " ".join(fmt(c) for pt in pts for c in pt))
        elif verb == pathops.PathVerb.QUAD:
            out.append("Q" + " ".join(fmt(c) for pt in pts for c in pt))
        elif verb == pathops.PathVerb.CLOSE:
            out.append("Z")
    return "".join(out)
