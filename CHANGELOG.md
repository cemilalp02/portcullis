# Changelog

All notable changes to this project are documented here.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.2.0] - 2026-10-17

Coverage of more agents and of the scripts their hooks run, look-alike letters, fewer wrong
findings below `high`, a VS Code extension and a website. On a fourth held-out set of 500
repositories, 1.2% had a false finding at or above `high` (docs/VALIDATION.md).

### Added

- Gemini CLI settings (`.gemini/settings.json`): its MCP servers, hooks and tool discovery and
  call commands are part of the auto-execution inventory and the execution rules
  (`gemini_hook`, PCL-AEX-011).
- VS Code workspace settings (`.vscode/settings.json`): PCL-AEX-012 (every agent tool approved
  without asking, `chat.tools.autoApprove`), PCL-AEX-013 (a catch-all terminal auto-approve
  rule), PCL-AEX-014 (automatic tasks allowed); MCP servers declared under `mcp`.
- Copilot custom agents and chat modes (`.github/agents/`, `.github/chatmodes/`) and Gemini CLI
  custom commands (`.gemini/commands/*.toml`) are scanned as instructions.
- PCL-ENG-015: a git-crypt encrypted agent file is reported as not analysed instead of being
  matched as text. A forged header in front of readable text is analysed as usual.
- Repository scripts that hooks, tasks, lifecycle commands and MCP servers run
  (`bash "$CLAUDE_PROJECT_DIR"/.claude/hooks/check.sh`, `node scripts/postinstall.js`) are read
  and analysed as the new `run-script` (shell) and `run-program` (Python, JavaScript, Ruby,
  Perl) targets, one level deep and at most 256 per scan, and are covered by `context.lock`.
  PCL-EXE-015 (critical) reports such a shell script that downloads or decodes code and
  executes it; PCL-EXE-016 (critical) a program that passes such a command to a shell or
  executes decoded content. A message that only shows the command to the user is not
  reported.
- Look-alike letters: the normalised view folds Cyrillic, Greek and Armenian letters that look
  like Latin ones back to Latin inside words that mix scripts, so a hook running `bаsh`
  (Cyrillic а) is reported like `bash`. PCL-UNI-009 also covers Greek and Armenian look-alikes.

- VS Code extension (`editors/vscode/`): findings as problems with exact ranges (code-point
  columns converted to the editor's UTF-16), rule links and fixes, rescans on save, a status-bar
  count. It runs in Restricted Mode, and the executable comes from user or machine settings
  only, so a workspace cannot choose what it runs. The `.vsix` is attached to each release and
  covered by the signed checksums; an end-to-end test runs it in an isolated VS Code.
- Project website (`site/`, published by `.github/workflows/pages.yml` to GitHub Pages): a
  single dark static page whose opening shows a file as you see it and as your agent reads it,
  with no external resources and a strict Content-Security-Policy; `scripts/site/build.sh`
  assembles it locally.

### Changed

- PCL-EXE-003, PCL-EXE-005 and PCL-EXE-008 apply to skill scripts only; scripts run by hooks
  are covered by PCL-EXE-015, PCL-EXE-016 and the network rules.
- `context.lock` written by v0.1 does not list the scripts that hooks run; `portcullis verify`
  reports them as new files until the lock is written again.

### Fixed

- PCL-AEX-004 missed hooks that run a repository script through a variable, the usual form
  `"$CLAUDE_PROJECT_DIR"/.claude/hooks/check.sh`; on the development set it now reports 226
  such commands instead of 112.
- PCL-AUT-001 no longer reports a catalogue of injection kinds that names the phrase next to a
  variant of itself ("이전 지시를 무시 / ignore previous instructions") or a short quoted mention
  in Korean or Japanese (false findings of the third held-out set).
- Precision of findings below `high` rose from about 76% to 84% on a sample of the
  development set (docs/VALIDATION.md): PCL-BAI-004 needs an instruction aimed at a security
  tool; PCL-AUT-003 skips chat-template tokens in tokenizer and training code and XML
  elements; PCL-EXE-003 skips commands shown in comments and messages; PCL-AUT-004 no longer
  treats the "Developer mode" of browsers and operating systems as a jailbreak; PCL-UNI-003
  accepts U+200B as the word separator of Khmer, Thai, Lao, Myanmar and Tibetan; PCL-EXE-008
  reports only the home-directory `.npmrc`; PCL-SKL-001 ignores localhost, example.com,
  relative URLs and local `rsync`; PCL-SKL-003 ignores "do not use sudo" and checks for it.
- Messages no longer quote the carriage return of CRLF files.
- The "e.g." exclusion of ten rules never matched (no word boundary after a period).
- Two patterns of one rule matching the same text are reported once (a `curl … | sh` line was
  reported twice by PCL-EXE-001's sibling patterns).

## [0.1.0] - 2026-10-17

The first release.

### Scanning

- `portcullis scan` finds the files AI coding agents read or run on their own — `CLAUDE.md`,
  `AGENTS.md`, `GEMINI.md`, `SKILL.md` and the scripts and documents next to it, Cursor, Windsurf
  and Cline rules, Copilot instructions and prompts, `.claude/settings.json` hooks, `.mcp.json`,
  `.vscode/tasks.json`, dev containers and npm lifecycle scripts — and checks them without ever
  executing, fetching or opening a network connection (ADR-0006, enforced by an architecture
  test).
- 70 rules: 56 detection rules in seven groups — UNI (invisible and deceptive Unicode), AUT
  (authority and concealment), EXE (execution, credentials, exfiltration), BAI (scanner bait),
  AEX (auto-execution), MCP (server definitions), SKL (skills) — and 14 input-integrity rules
  (PCL-ENG-001 to PCL-ENG-014). Every rule ships positive and negative fixtures; the catalogue
  `docs/RULES.md` is generated by `portcullis rules list --format markdown`.
- Text is matched in several views of each file: raw; normalised (invisible characters
  removed, NFKC, case-folded, ADR-0005); prose (Markdown markup and HTML entities undone,
  ADR-0010); and, for auto-executed commands, a shell view with common shell obfuscation
  undone. Findings always point at the text as written; the hidden text of Unicode Tag runs is
  decoded into the finding.
- An auto-execution inventory lists every command that runs without asking: Claude hooks,
  folder-open tasks, dev container commands, npm install scripts and auto-enabled MCP servers.
- Discovery never follows symlinks or Windows junctions; UTF-8 and UTF-16 decoding; a JSONC
  parser that reports duplicate keys; a shared budget of bytes under analysis bounds memory.
- Fail closed: exit `2` whenever anything could not be analysed, never "clean".

### Trust and workflow

- `portcullis lock` / `portcullis verify`: approve agent context files in `context.lock` and fail
  when any of them is added, modified or removed without a new approval.
- `portcullis baseline`: accept current findings by fingerprint, with reasons; stale entries are
  reported (PCL-ENG-013).
- Untrusted mode by default: configuration, baselines and custom rules inside a scanned
  repository are ignored unless `--trusted`. Pull-request mode (`--base-snapshot`) reads them
  from the base commit, and changes to them are reported (PCL-ENG-009).
- Configuration (`.portcullis.yaml`, user config, `--config`), custom rule directories
  (`--rules`), `--exclude`, `--fail-on`, `--color`.

### Output

- Text, JSON, SARIF 2.1.0 (validated against the official schema), GitHub workflow annotations
  and Markdown summaries; `--sarif`, `--summary`, `--path-prefix`.
- Self-contained HTML report (`--format html`, `--html FILE`): one offline file set like a
  printed inspection document, with the verdict, a severity ledger that filters the findings,
  top rules, most affected files, the auto-execution inventory, findings grouped per file with
  excerpts and remedies, search, light and dark themes; no network access, guarded by a
  hash-based Content Security Policy. Styles (Tailwind CSS) and icons (Lucide) are compiled in
  by `web/report`; IBM Plex is embedded (SIL OFL 1.1).

### Integrations

- GitHub Action: annotations on the pull request diff, SARIF upload to code scanning, job
  summary, HTML report artifact, `context.lock` verification. At a release tag it runs the
  release binary only after verifying its build provenance and checksum.
- pre-commit hooks `portcullis` and `portcullis-verify`.

### Releases

- Binaries for Linux, macOS and Windows on amd64 and arm64, built reproducibly by GoReleaser
  in the release workflow, with an SPDX SBOM per archive, `checksums.txt` signed keylessly with
  Sigstore and SLSA build provenance for every file (`docs/RELEASING.md`).

### Validation

- False-positive benchmark on popular public repositories (`scripts/bench/fpbench.py`):
  repositories with a false finding at or above `high` — 0.20% of the 1,000 used for tuning,
  1.60%, 1.40% and 0.80% of three held-out sets of 500 (`docs/VALIDATION.md`).
- Defanged corpus in the test suite: 33 known attacks (all 63 expected detections), 10 clean
  repositories (no finding at or above `high`) and 46 evasion variants.
- Eight fuzz targets (parsers, normalisation, the full analysis); 100,000 files scanned in
  1.5 seconds.

### Documentation

- Requirements, threat model, architecture, rule schema, CLI and file-format specifications,
  architecture decision records ADR-0001 to ADR-0010, validation results, release process,
  brand guide; README in English and Turkish.

[Unreleased]: https://github.com/cemilalp02/portcullis/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/cemilalp02/portcullis/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/cemilalp02/portcullis/releases/tag/v0.1.0
