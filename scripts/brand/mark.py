"""Geometry of the Portcullis mark.

The mark is a pointed (two-centred) arch with a portcullis hanging inside it.
Every curve is a true circular arc and every offset is concentric, so the
frame, the gap and the grille keep their widths all the way round. The parts
are combined with boolean operations (skia-pathops) into one clean path.

Coordinates are on a 64-unit grid, y pointing down.
"""
import math

import pathops

W = 64.0
MID = W / 2

# Optical sizes: LARGE for 32 px and up; SMALL keeps the gate readable at
# 16-24 px with fewer, heavier parts.
LARGE = dict(bars=2, bar=3.6, w=3.8, t=7.2, g=2.4, spring=37, crossbars=(28.5, 40), cross=3.8)
SMALL = dict(bars=2, bar=4.4, w=4.6, t=8.5, g=3.0, cross=4.6, spring=37, crossbars=(31,), tooth=9, half=28.5)


def _arc(path, cx, cy, r, a0, a1):
    """Append a circular arc from angle a0 to a1 as cubic Beziers."""
    n = max(1, math.ceil(abs(a1 - a0) / (math.pi / 4)))
    da = (a1 - a0) / n
    k = 4 / 3 * math.tan(da / 4)
    for i in range(n):
        t0, t1 = a0 + i * da, a0 + (i + 1) * da
        x0, y0 = cx + r * math.cos(t0), cy + r * math.sin(t0)
        x3, y3 = cx + r * math.cos(t1), cy + r * math.sin(t1)
        path.cubicTo(x0 - k * r * math.sin(t0), y0 + k * r * math.cos(t0),
                     x3 + k * r * math.sin(t1), y3 - k * r * math.cos(t1), x3, y3)


def arch(half, spring, apex, inset, bottom):
    """Pointed arch region with outer half-span `half`, springing line
    `spring` and outer apex `apex`, offset inwards by `inset`, legs down to
    `bottom`. Both arcs are centred on the springing line."""
    h = spring - apex
    r = (half * half + h * h) / (2 * half)
    cl = MID - half + r  # centre of the left arc
    cr = W - cl
    ri = r - inset
    xl, xr = MID - half + inset, MID + half - inset
    p = pathops.Path()
    p.moveTo(xl, bottom)
    p.lineTo(xl, spring)
    ta = math.atan2(-math.sqrt(ri * ri - (MID - cl) ** 2), MID - cl) % (2 * math.pi)
    _arc(p, cl, spring, ri, math.pi, ta)
    tb = math.atan2(-math.sqrt(ri * ri - (MID - cr) ** 2), MID - cr)
    _arc(p, cr, spring, ri, tb, 0)
    p.lineTo(xr, bottom)
    p.close()
    return p


def _rect(x0, y0, x1, y1):
    return _poly((x0, y0), (x1, y0), (x1, y1), (x0, y1))


def _poly(*pts):
    p = pathops.Path()
    p.moveTo(*pts[0])
    for q in pts[1:]:
        p.lineTo(*q)
    p.close()
    return p


def _union(paths):
    out = pathops.Path()
    for p in paths:
        out = pathops.op(out, p, pathops.PathOp.UNION)
    return out


def mark(**kw):
    """The whole mark as one path."""
    frame, grille = parts(**kw)
    out = _union([frame, grille])
    out.simplify(fix_winding=True)
    return out


def parts(half=27, spring=31, apex=2, ground=62, t=6.5, g=2.2, w=3.6, bars=1, bar=3.6,
          crossbars=(26, 38.5), cross=3.6, tooth=7):
    """The arch frame of thickness t and, after a gap g, the grille with rails
    of width w, `bars` inner bars, horizontal crossbars and pointed teeth.
    The two-tone mark colours them separately."""
    big = ground + 10
    frame = pathops.op(arch(half, spring, apex, 0, ground), arch(half, spring, apex, t, big), pathops.PathOp.DIFFERENCE)
    gx0, gx1 = MID - half + t + g, MID + half - t - g
    last = crossbars[-1] + cross  # bottom edge of the grille body
    body = pathops.op(arch(half, spring, apex, t + g, big), _rect(0, 0, W, last), pathops.PathOp.INTERSECTION)
    inner = arch(half, spring, apex, t + g + w, big)
    ox0, ox1 = gx0 + w, gx1 - w
    gap = (ox1 - ox0 - bars * bar) / (bars + 1)
    cols, xs = [], [(gx0, gx0 + w)]
    x = ox0
    for i in range(bars + 1):
        cols.append(_rect(x, 0, x + gap, last))
        x += gap
        if i < bars:
            xs.append((x, x + bar))
            x += bar
    xs.append((gx1 - w, gx1))
    holes = pathops.op(inner, _union(cols), pathops.PathOp.INTERSECTION)
    holes = pathops.op(holes, _union(_rect(0, y, W, y + cross) for y in crossbars), pathops.PathOp.DIFFERENCE)
    grille = pathops.op(body, holes, pathops.PathOp.DIFFERENCE)
    teeth = _union(_poly((a, last - 0.5), (b, last - 0.5), (b, last + tooth - (b - a) * 0.9),
                         ((a + b) / 2, last + tooth), (a, last + tooth - (b - a) * 0.9)) for a, b in xs)
    grille = _union([grille, teeth])
    frame.simplify(fix_winding=True)
    grille.simplify(fix_winding=True)
    return frame, grille


def fmt(v):
    return f"{v:.2f}".rstrip("0").rstrip(".")


def svg_d(p):
    """SVG path data of a pathops path."""
    parts = []
    for verb, pts in p:
        if verb == pathops.PathVerb.MOVE:
            parts.append("M" + " ".join(fmt(c) for c in pts[0]))
        elif verb == pathops.PathVerb.LINE:
            parts.append("L" + " ".join(fmt(c) for c in pts[0]))
        elif verb == pathops.PathVerb.CUBIC:
            parts.append("C" + " ".join(fmt(c) for pt in pts for c in pt))
        elif verb == pathops.PathVerb.QUAD:
            parts.append("Q" + " ".join(fmt(c) for pt in pts for c in pt))
        elif verb == pathops.PathVerb.CLOSE:
            parts.append("Z")
    return "".join(parts)
