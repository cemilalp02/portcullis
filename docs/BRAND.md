# Portcullis — Brand

A portcullis is the iron grille lowered in front of a castle gate. The mark draws exactly
that: a pointed gateway with the grille hanging inside it, teeth down. It stands for what the
tool does: nothing reaches the agent before it has been inspected.

<p>
  <img src="assets/brand/portcullis-icon.svg" width="72" alt="App icon">&nbsp;
  <img src="assets/brand/portcullis-logo.svg" height="40" alt="Logo on light backgrounds">
</p>

## Mark

The mark has two parts: the **arch** (frame) and the **grille** with its teeth. In colour, the
arch is ink or ivory and the grille is verdigris, the green of weathered copper, with a slight
vertical gradient: the gate is the part that inspects. In one colour, both parts take the same
colour.

| Asset | Use |
|---|---|
| [`portcullis-logo.svg`](assets/brand/portcullis-logo.svg) | Logo on light backgrounds (two-tone mark + wordmark) |
| [`portcullis-logo-dark.svg`](assets/brand/portcullis-logo-dark.svg) | Logo on dark backgrounds |
| [`portcullis-icon.svg`](assets/brand/portcullis-icon.svg) | App icon: the two-tone mark on a dark tile with a soft glow behind the grille. Avatars, the HTML report |
| [`favicon.svg`](assets/brand/favicon.svg) | Browser tabs: the app icon with the small mark |
| [`portcullis-social.svg`](assets/brand/portcullis-social.svg) | 1280 × 640 card for link previews (GitHub social preview, chat apps) |
| [`portcullis-mark.svg`](assets/brand/portcullis-mark.svg) | One-colour mark, 32 px and larger (`currentColor`) |
| [`portcullis-mark-small.svg`](assets/brand/portcullis-mark-small.svg) | One-colour mark, 16–24 px: fewer, heavier parts (`currentColor`) |

The geometry is exact rather than hand-drawn. The arch is two circular arcs centred on the
springing line. The frame, the gap and the grille are concentric offsets of it, so their
widths stay constant all the way round. The parts are built with boolean operations.

- Keep clear space of at least the width of the frame around the mark.
- Use the two-tone mark, or the one-colour mark in ink or ivory. Don't recolour the parts
  otherwise, outline, rotate or rearrange the lockup. The glow belongs to the app icon only.
- Use the small optical size below 32 px; the large mark fills in at small sizes.

## Colour

| Name | Hex | Use |
|---|---|---|
| Ink | `#14181e` | Arch and text on light backgrounds; app-icon tile `#232b36` → `#0b0e12` |
| Ivory | `#efe9dc` | Arch and text on dark backgrounds |
| Verdigris | grille `#14b8a6` → `#0f766e` (light), `#7ff0dc` → `#14b8a6` → `#0f766e` (dark) | The grille; in the report, links, focus and the "How to fix" box (`#0f766e` light, `#2dd4bf` dark) |

The HTML report uses the shadcn/ui token structure (`--background`, `--card`, `--muted`,
`--primary` …) with these colours; see [`web/report/input.css`](../web/report/input.css).
Severity colours are reserved for findings: critical `#dc2626`, high `#ea580c`, medium
`#c27c0e`, low `#2563eb`, info `#64748b` (lighter in the dark theme). They are never used for
decoration.

## Type

**IBM Plex Sans** for text and **IBM Plex Mono** for paths, rule IDs, commands and code
(SIL Open Font License 1.1). The wordmark is Plex Sans SemiBold with slightly tightened
tracking (-0.012 em), converted to outlines. Icons in the report are
[Lucide](https://lucide.dev) at a 2 px stroke.

## Regenerating the assets

The assets are generated, not edited by hand:

```bash
python -m venv .venv && .venv/bin/pip install -r scripts/brand/requirements.txt
npm pack @ibm/plex-sans@1.1.0   # the complete woff2 files are in fonts/complete/woff2
.venv/bin/python scripts/brand/brand.py path/to/fonts/complete/woff2 docs/assets/brand
```

The HTML report embeds the app icon and the favicon (`internal/report/html/icon.svg`,
`favicon.svg`). `TestHTMLBrandMatchesAssets` fails if they drift from these files.
