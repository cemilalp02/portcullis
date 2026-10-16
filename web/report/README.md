# HTML report assets

The HTML report (`portcullis scan --format html`, `--html FILE`) is one self-contained file.
Its style sheet and icons are built here and committed to `internal/report/html`, so building
Portcullis needs no Node.js.

| Output | Built from |
|---|---|
| `internal/report/html/style.css` | [`input.css`](input.css): [Tailwind CSS](https://tailwindcss.com) v4, with semantic design tokens named after those of [shadcn/ui](https://ui.shadcn.com) and set in the [Portcullis palette](../../docs/BRAND.md). Only the classes used in `report.tmpl`, `app.js` and `html.go` are generated. |
| `internal/report/html/icons.svg` | [Lucide](https://lucide.dev) icons: every `"#i-name"` in `report.tmpl` and `"i-name"` in `html.go` becomes a `<symbol>`. |

```bash
cd web/report
npm ci
npm run build
```

`build.mjs` uses Tailwind's Node API rather than its CLI, which pulls in a file watcher this
build does not need. CI rebuilds both files and fails if they differ from the committed ones;
`TestIconsAreInTheSprite` fails if the template refers to an icon the sprite lacks.

The report's Content Security Policy allows its own style and script by hash and nothing from
the network, so everything stays inline: no web fonts, scripts or images are fetched. Licences
of the bundled projects are in [THIRD_PARTY_LICENSES.md](THIRD_PARTY_LICENSES.md).
