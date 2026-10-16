# Changelog

All notable changes to this project are documented here.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Changed

- Project renamed from Bekçi to **Portcullis** (ADR-0009): module `github.com/cemilalp02/portcullis`,
  binary `portcullis`, rule ID prefix `PCL-`, config `.portcullis.yaml`.
- Messages no longer show an empty name in quotes for unnamed hooks
  (`claude_hook (SessionStart)` instead of `claude_hook '' (SessionStart)`).

### Added

- Brand identity: the Portcullis mark (a pointed gateway with its grille lowered), two-tone
  with a verdigris grille, in two optical sizes; logo lockups for light and dark backgrounds,
  app icon, favicon and a social card, generated from exact geometry by `scripts/brand`; guide
  in `docs/BRAND.md`.
- Self-contained HTML report (`--format html`, `--html FILE`): one offline file with severity
  filters, search, the auto-execution inventory and every scanned agent file; light and dark
  themes; no network access, guarded by a hash-based Content Security Policy. The GitHub Action
  uploads it as a workflow artifact. Report design built with Tailwind CSS, the shadcn/ui
  design tokens and Lucide icons (compiled into the page by `web/report`): sidebar navigation,
  verdict card, severity cards that filter the findings, distribution bar, top rules, most
  affected files and an auto-run summary, findings as cards per file with the excerpt, a
  "How to fix" box and the fingerprint; severity filters and search (`/`), expand/collapse
  all, invisible characters highlighted in excerpts, IBM Plex typography embedded
  (SIL OFL 1.1).
- `portcullis lock` / `portcullis verify`: approve agent context files in `context.lock` and fail
  when any of them is added, modified or removed without a new approval.
- `portcullis baseline`: accept current findings by fingerprint (reasons required and kept).
- Configuration (`.portcullis.yaml`, user config, `--config`), `--trusted` mode, pull-request
  mode with `--base-snapshot`, custom rule directories (`--rules`), `--color`.
- Output formats SARIF 2.1.0 (validated against the official schema), GitHub workflow
  annotations and Markdown summaries; `--sarif`, `--summary`, `--path-prefix`.
- GitHub Action (`action.yml`), pre-commit hooks, and a workflow that scans this repository.
- Rules PCL-ENG-009 (change touches Portcullis control files) and PCL-ENG-013 (stale baseline).
- 46 detection rules (48 in total) across seven groups: UNI (invisible and deceptive Unicode),
  AUT (authority and concealment), EXE (execution, credentials, exfiltration), BAI (scanner
  bait), AEX (auto-execution), MCP (server definitions) and SKL (skills). See `docs/RULES.md`.
- Scripts and documents next to a `SKILL.md` are scanned as `skill-script` / `skill-doc`.
- Exec rules can filter on the trigger; the hidden text of Unicode Tag runs is shown decoded.
- `portcullis rules list --format markdown` and the generated rule catalogue `docs/RULES.md`.
- Defanged corpus: 33 known attacks (100% recall required) and 10 clean repositories.
- `portcullis scan`: scans a directory for agent context files and reports findings as text or
  JSON, with an auto-execution inventory and exit codes 0/1/2 (fail closed).
- `portcullis rules list|show`.
- Core engine: discovery that never follows symlinks or Windows junctions, UTF-8/UTF-16
  decoding, normalised text view, JSONC parser with duplicate-key detection, extraction of
  auto-executed commands from Claude settings, VS Code tasks, dev containers, npm scripts and
  MCP configs, and a YAML rule engine with mandatory fixtures.
- Rules PCL-UNI-001 (Unicode Tag smuggling), PCL-EXE-001 (download piped to a shell in an
  auto-executed command) and engine integrity rules PCL-ENG-001 to PCL-ENG-012.
- Fuzzing of the parsers in CI.
- Requirements, threat model, architecture, rule schema, CLI and file-format specifications.
- Architecture decision records ADR-0001 to ADR-0009.
- Project skeleton: `portcullis version`, CI (lint, tests on Linux/Windows/macOS, govulncheck,
  cross-compilation), architecture test enforcing ADR-0006.
