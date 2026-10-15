# Portcullis — Brand

A portcullis is the iron grille lowered in front of a castle gate. The mark draws exactly
that: a pointed gateway with the grille hanging inside it, teeth down. It stands for what the
tool does: nothing reaches the agent before it has been inspected.

## Mark

| Asset | Use |
|---|---|
| [`portcullis-logo.svg`](assets/brand/portcullis-logo.svg) | Logo on light backgrounds (mark + wordmark) |
| [`portcullis-logo-dark.svg`](assets/brand/portcullis-logo-dark.svg) | Logo on dark backgrounds |
| [`portcullis-mark.svg`](assets/brand/portcullis-mark.svg) | Mark alone, 32 px and larger (`currentColor`) |
| [`portcullis-mark-small.svg`](assets/brand/portcullis-mark-small.svg) | Mark alone, 16–24 px: fewer, heavier parts (`currentColor`) |
| [`portcullis-icon.svg`](assets/brand/portcullis-icon.svg) | App icon, avatars, social cards |
| [`favicon.svg`](assets/brand/favicon.svg) | Browser tabs; follows the light/dark colour scheme |

The geometry is exact rather than hand-drawn. The arch is two circular arcs centred on the
springing line. The frame, the gap and the grille are concentric offsets of it, so their
widths stay constant all the way round. The parts are merged into one path with boolean
operations.

- Keep clear space of at least the width of the frame around the mark.
- Use the mark in ink or ivory only. Don't recolour, outline, rotate, add effects or rearrange
  the lockup.
- Use the small optical size below 32 px; the large mark fills in at small sizes.

## Colour

| Name | Hex | Use |
|---|---|---|
| Ink | `#14181e` | Mark, text, report masthead |
| Ivory | `#efe9dc` | Mark and text on ink |
| Verdigris | `#1c5d52` (dark theme `#7cc2b2`) | Links, focus, the "Fix" label |
| Paper | `#f4f3ef` | Page background |

Severity colours are reserved for findings: critical `#b42318`, high `#c2500b`, medium
`#946200`, low `#2c58b4`, info `#69707a`. They are never used for decoration.

## Type

**IBM Plex Sans** for text and **IBM Plex Mono** for paths, rule IDs, commands and code
(SIL Open Font License 1.1). The wordmark is Plex Sans SemiBold with slightly tightened
tracking (-0.012 em), converted to outlines.

## Regenerating the assets

The assets are generated, not edited by hand:

```bash
python -m venv .venv && .venv/bin/pip install -r scripts/brand/requirements.txt
npm pack @ibm/plex-sans@1.1.0   # IBMPlexSans-SemiBold.woff2 is in fonts/complete/woff2
.venv/bin/python scripts/brand/brand.py path/to/IBMPlexSans-SemiBold.woff2 docs/assets/brand
```

The HTML report embeds the small mark and the favicon. `TestHTMLBrandMatchesAssets` fails if
they drift from these files.
