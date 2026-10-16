# Portcullis — Brand

The mark shows what the tool does. Three lines of a file an AI coding agent will read; in the
middle line sits a character that should not be there — caught, and made visible. That is the
job: read what the agent reads, before it does, and point at what does not belong.

<p>
  <img src="assets/brand/portcullis-icon.svg" width="72" alt="App icon">&nbsp;
  <img src="assets/brand/portcullis-logo.svg" height="40" alt="Logo on light backgrounds">
</p>

## Mark

The mark has two parts: the **lines** of text and the **caught character**, a hollow rounded
box set into the middle line. In colour, the lines are ink (ivory on dark backgrounds) and the
character is the signal colour. In one colour, both parts take the same colour. Everything is
flat: no gradients, glows or shadows.

| Asset | Use |
|---|---|
| [`portcullis-logo.svg`](assets/brand/portcullis-logo.svg) | Logo on light backgrounds: two-tone mark and wordmark |
| [`portcullis-logo-dark.svg`](assets/brand/portcullis-logo-dark.svg) | Logo on dark backgrounds |
| [`portcullis-mark-color.svg`](assets/brand/portcullis-mark-color.svg) | Two-tone mark on its own; the HTML report recolours its two parts per theme |
| [`portcullis-icon.svg`](assets/brand/portcullis-icon.svg) | App icon: the mark in ivory and signal on an ink tile. Avatars, profile images |
| [`favicon.svg`](assets/brand/favicon.svg) | Browser tabs: the app icon with the small mark |
| [`portcullis-social.svg`](assets/brand/portcullis-social.svg) | 1280 × 640 card for link previews (GitHub social preview, chat apps) |
| [`portcullis-mark.svg`](assets/brand/portcullis-mark.svg) | One-colour mark, 32 px and larger (`currentColor`) |
| [`portcullis-mark-small.svg`](assets/brand/portcullis-mark-small.svg) | One-colour mark, 16–24 px: heavier lines, a larger character (`currentColor`) |

The geometry is exact rather than hand-drawn ([`scripts/brand/mark.py`](../scripts/brand/mark.py)).
On a 64-unit grid the lines are 8 units tall with fully rounded ends, on rows 10, 28 and 46;
the top line is full width, the bottom one shorter, like the end of a paragraph. The middle
line is split around the character with a 4.5-unit gap on each side. The character is a
12.5 × 17 rounded box drawn with a 3.6-unit outline. Corners are true quarter circles and the
parts are joined with boolean operations.

- Keep clear space of at least one line height (8 units) around the mark.
- The caught character is always the accent: signal on light, light signal on dark, or the
  same colour as the lines in the one-colour mark. Don't recolour the lines, outline the
  mark, rotate it or rearrange the lockup.
- Use the small optical size below 32 px; at that size the large mark's gaps close up.

## Colour

| Name | Light | Dark | Use |
|---|---|---|---|
| Ink | `#14181e` | — | Text and the mark's lines on light backgrounds; the app-icon tile |
| Paper | `#f4f1ea` | — | Background of the report, the social card and the brand board |
| Ivory | — | `#efe9dc` | The mark's lines and text on ink |
| Signal | `#c8461b` | `#ee7d4f` | The caught character; in the report, section numbers, focus rings and the highlight around invisible characters |
| Rule | `#d8d2c4` | `#ece7da` at 14 % | Hairlines |

Severity colours are reserved for findings and never used for decoration:

| Severity | Light | Dark |
|---|---|---|
| Critical | `#b42318` | `#f2846c` |
| High | `#c2560c` | `#f0a050` |
| Medium | `#946300` | `#d8b44c` |
| Low | `#2f5aa8` | `#8fb1ea` |
| Info | `#6b7079` | `#a3a7af` |
| Pass | `#1e7148` | `#70c69c` |

The report's tokens are in [`web/report/input.css`](../web/report/input.css); the dark theme
("night ledger") uses `#111317` for the background and `#ece7da` for text.

## Type

**IBM Plex Sans** for text and **IBM Plex Mono** for paths, rule IDs, commands and code
(SIL Open Font License 1.1). The wordmark is "portcullis" in lower case, Plex Sans SemiBold with
slightly tightened tracking (−0.015 em), converted to outlines. Labels are small capitals: 11 px
SemiBold, upper case, 0.12 em tracking.

## The report

The HTML report is set like a printed inspection document rather than a dashboard: paper, ink
and hairline rules, a 3 px ink rule under the masthead, numbered sections (01 Summary …
05 Rules) with the number in the signal colour, and the verdict as a stamp — `FAIL`, `PASS` or
`INCOMPLETE` in letter-spaced Plex Mono inside a double border in the verdict's colour. The
severity ledger doubles as the findings filter. There are no cards, shadows or gradients; the
only icons are seven [Lucide](https://lucide.dev) glyphs at a 2 px stroke for controls.

## Regenerating the assets

The assets are generated, not edited by hand:

```bash
python -m venv .venv && .venv/bin/pip install -r scripts/brand/requirements.txt
npm pack @ibm/plex-sans@1.1.0 @ibm/plex-mono@2.5.0   # complete woff2 files: fonts/complete/woff2
.venv/bin/python scripts/brand/brand.py path/to/plex-sans/fonts/complete/woff2 \
    path/to/plex-mono/fonts/complete/woff2 docs/assets/brand
```

The HTML report embeds the two-tone mark and the favicon (`internal/report/html/mark.svg`,
`favicon.svg`). `TestHTMLBrandMatchesAssets` fails if they drift from these files.
