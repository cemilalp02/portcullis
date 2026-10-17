# Portcullis — Requirements Specification (v0.1)

| Field    | Value |
|----------|-------|
| Status   | **Approved** — Phase 0 gate passed (2026-10-13) |
| Owner    | Cemil Alp ([@cemilalp02](https://github.com/cemilalp02)) |
| Module   | `github.com/cemilalp02/portcullis` |
| Related  | [`THREAT_MODEL.md`](THREAT_MODEL.md) · [`ARCHITECTURE.md`](ARCHITECTURE.md) · [`ROADMAP.md`](ROADMAP.md) |

> A *portcullis* is the iron grille dropped in front of a castle gate: nothing passes until it
> has been inspected. Portcullis does the same for what an AI coding agent is about to read.

Requirement keywords **MUST**, **SHOULD**, **MAY** follow RFC 2119.

---

## 1. Problem statement

AI coding agents (Claude Code, Cursor, Copilot, Codex, Gemini CLI, Windsurf, Cline, …)
treat certain repository files as **instructions** or **auto-executed configuration**:
`CLAUDE.md`, `AGENTS.md`, `SKILL.md`, `.cursorrules`, `.claude/settings.json` hooks,
`.mcp.json`, `.vscode/tasks.json`, and others. Opening an untrusted repository with an
agent therefore hands part of the agent's authority — and, through its tools, the
developer's machine and credentials — to the repository's author.

Real campaigns in 2026 (e.g. DUSTMAKER / TeamPCP) planted malicious configuration in
`.claude/`, `.cursor/` and `.vscode/` folders, hid instructions with invisible Unicode, and
embedded "refusal bait" designed to make LLM-based scanners stop analysing a file. A scan
of 3,984 public skills found vulnerabilities in 36.8% and confirmed malicious payloads in 76.

Existing tools cover only part of this surface (MCP-config-only scanners, some of which
send metadata to a cloud service). No widely used open-source tool **locally** scans the
*whole* agent-context surface of a repository, emits standard results for CI, and **pins**
the approved state so that silent changes are caught.

## 2. Goals and non-goals

### 2.1 Goals (v0.1)

- **G1** Detect known malicious patterns and risky auto-execution in agent context files before an agent consumes them.
- **G2** Run fully offline; scanned content never leaves the machine.
- **G3** Fit existing workflows: CLI, GitHub Action, pre-commit, SARIF / GitHub Code Scanning.
- **G4** Pin approved context files (`context.lock`) so any later change fails CI until re-approved.
- **G5** Make rules easy to contribute (declarative YAML, fixture-driven tests).
- **G6** **Fail closed**: anything Portcullis cannot fully analyse is reported, never silently treated as clean.

### 2.2 Non-goals (v0.1)

- Runtime enforcement of agent tool calls (that is *Project 2 — Köken*).
- Guaranteed detection of novel or paraphrased prompt injection. Portcullis is a **pattern and
  structure** scanner, not a semantic classifier (see `THREAT_MODEL.md` §6).
- Automatically fixing or rewriting files.
- General SAST or secret scanning (use gitleaks / trufflehog).

## 3. Users

| Persona | Need | Primary interface |
|---|---|---|
| **P1 — Developer** opening a third-party repo with an AI agent | "Is it safe to let my agent read this?" | `portcullis scan` |
| **P2 — Maintainer** of a repo that ships agent context files | "Did a PR silently add a hook or a hidden instruction?" | GitHub Action, `context.lock` |
| **P3 — Security / platform engineer** | Org-wide visibility in standard formats | SARIF, JSON |
| **P4 — Skill / MCP author** | "Is my skill clean before I publish it?" | `portcullis scan`, pre-commit |

## 4. User stories

| ID | Story | Acceptance criteria |
|---|---|---|
| US-01 | As P1, I scan a cloned repo and see findings ordered by severity with file, line and a fix hint. | `portcullis scan <dir>` prints findings; exit code `1` when any finding ≥ threshold. |
| US-02 | As P1, I scan a repo I do **not** trust, and the repo cannot hide its own findings. | In untrusted mode (default) nothing inside the scanned tree can change Portcullis's behaviour. |
| US-03 | As P1, I immediately see everything that would **run automatically** if I open this repo. | Text output always ends with an *auto-execution inventory* (hooks, tasks, lifecycle commands, MCP servers). |
| US-04 | As P2, I add Portcullis to CI in one step and see results in GitHub's Security tab. | Action uploads SARIF; findings appear in Code Scanning. |
| US-05 | As P2, a PR cannot weaken Portcullis by editing Portcullis's own config. | In PR context, config/baseline are read from the base commit; edits to them are reported. |
| US-06 | As P2, I approve the current context files and get a failing build if any of them changes. | `portcullis lock` writes `context.lock`; `portcullis verify` fails on added / modified / removed targets. |
| US-07 | As P3, I consume results in machine-readable form. | `--format json` / `--format sarif` output validates against its schema in tests. |
| US-08 | As P4, I block my own commits that introduce risky content. | A `.pre-commit-hooks.yaml` definition is provided. |
| US-09 | As any user, I understand why something was flagged. | `portcullis rules list` / `portcullis rules show <id>`: description, severity, OWASP mapping, remediation, references. |
| US-10 | As a contributor, I add a rule with YAML and fixtures only. | No Go code needed; `make test` runs its positive and negative fixtures automatically. |

## 5. Scanned surface

Portcullis recognises **16 target types** in two families (13 in v0.1; v0.2 added Gemini CLI commands and settings and VS Code workspace settings), plus the repository scripts that their commands run (§5.3). File-name matching **MUST** be
case-insensitive, because agents on Windows and macOS read `claude.md` as `CLAUDE.md`.
Every path matches **at any depth** (`apps/web/.vscode/tasks.json` counts like
`.vscode/tasks.json`), because an agent or editor opened on a subfolder treats that folder
as the project root.

### 5.1 Instruction files (natural-language text read by the agent)

| # | Target type | Paths (relative to scan root) |
|---|---|---|
| 1 | Claude memory | `**/CLAUDE.md`, `**/CLAUDE.local.md` |
| 2 | Generic agent instructions | `**/AGENTS.md` |
| 3 | Gemini instructions | `**/GEMINI.md` |
| 4 | Skills | `**/SKILL.md`; every file next to it (any depth) is a companion target: code as `skill-script`, documents as `skill-doc` |
| 5 | Claude commands & sub-agents | `.claude/commands/**/*.md`, `.claude/agents/**/*.md` |
| 6 | Cursor rules | `.cursorrules`, `.cursor/rules/**` |
| 7 | Copilot instructions, prompts, custom agents & chat modes | `.github/copilot-instructions.md`, `.github/instructions/**/*.instructions.md`, `.github/prompts/**/*.prompt.md`, `.github/agents/**/*.md`, `.github/chatmodes/**/*.chatmode.md` |
| 8 | Gemini CLI custom commands | `.gemini/commands/**/*.toml` |
| 9 | Other agent rules | `.windsurfrules`, `.clinerules` (file or folder) |

### 5.2 Configuration files (structured, may cause automatic execution)

| # | Target type | Paths | What is inspected |
|---|---|---|---|
| 10 | Claude settings | `.claude/settings.json`, `.claude/settings.local.json` | `hooks`, `permissions`, `env`, MCP enablement |
| 11 | VS Code tasks | `.vscode/tasks.json` | tasks with `runOptions.runOn: "folderOpen"` |
| 12 | Dev containers | `.devcontainer/devcontainer.json`, `.devcontainer.json`, `.devcontainer/*/devcontainer.json` | `initializeCommand` (runs on the **host**), `onCreate`/`updateContent`/`postCreate`/`postStart`/`postAttach` commands |
| 13 | npm manifests | `**/package.json` | `preinstall`, `install`, `postinstall`, `prepare` scripts |
| 14 | MCP configuration | `.mcp.json`, `**/mcp.json`, `.cursor/mcp.json`, `.vscode/mcp.json` | server `command`, `args`, `env`, `url` |
| 15 | Gemini CLI settings | `.gemini/settings.json` | `mcpServers`, `hooks`, `tools.discoveryCommand` / `callCommand` |
| 16 | VS Code workspace settings | `.vscode/settings.json` | settings that approve agent tools or terminal commands without asking, automatic tasks, MCP servers under `mcp` |

### 5.3 Scripts run by configuration files

Repository scripts named by the commands of §5.2 (`bash .claude/hooks/check.sh`,
`"$CLAUDE_PROJECT_DIR"/.claude/hooks/x.py`, `node scripts/postinstall.js`) are analysed as the
`run-script` target (shell scripts: `.sh`, `.bash`, `.zsh`, `.ps1`, `.psm1`, `.bat`, `.cmd`) or the
`run-program` target (`.py`, `.js`, `.mjs`, `.cjs`, `.ts`, `.rb`, `.pl`), both of family `script`. Paths are resolved inside the scan root against the
project directory and the configuration file's directory; only one level is followed and at
most 256 scripts are read per scan.

Target definitions are **data**, not code, so new agents can be supported by adding a
definition and fixtures.

## 6. Functional requirements

### 6.1 Discovery and input handling

- **FR-01** Portcullis MUST walk the scan root recursively and classify files against §5.
- **FR-02** `.git/` MUST be skipped. `node_modules/` and `vendor/` are skipped by default
  but **target files inside them are still reported in a summary count** (packages there
  can ship skills / MCP configs).
- **FR-03** Symlinks MUST NOT be followed if they resolve outside the scan root; a target
  that is such a symlink is itself reported as a finding.
- **FR-04** **No silent skip of a target file.** Target files are read in full up to a hard
  limit (default 10 MiB). A target above the limit produces a `high` finding
  *"target file too large to analyse"* — a padded instruction file is itself suspicious.
- **FR-05** Encoding: UTF-8 with or without BOM is decoded; UTF-16 LE/BE with BOM is
  decoded **and** reported (`medium`, unusual encoding for instruction files). Invalid
  byte sequences MUST NOT crash the scanner and are reported.
- **FR-06** Structured configs MUST be parsed as **JSONC** (comments and trailing commas),
  matching how VS Code and dev-container tooling read them.
- **FR-07** If a config cannot be parsed, Portcullis MUST still run text-level rules over it and
  report `high` *"unparseable config"* — never skip it.
- **FR-08** **Duplicate JSON keys** MUST be reported (`high`): different consumers resolve
  them differently, which enables "parser differential" smuggling.
- **FR-09** Any read error on a target file is a finding and makes the run exit with `2`.

### 6.2 Text normalisation (anti-evasion)

- **FR-10** Text rules MUST be evaluated against two views of each file:
  1. **raw** — exact text, used for invisible-character and encoding rules;
  2. **normalised** — Unicode NFKC, invisible/format characters removed, case-folded,
     whitespace runs collapsed — used for pattern rules.
  This defeats `ig\u200Bnore previous instructions`, full-width `ｉｇｎｏｒｅ`, and similar tricks.
- **FR-11** Line/column in findings MUST always refer to the **raw** file.

### 6.3 Detectors

| ID | Detector | Examples |
|---|---|---|
| **D1** | Invisible & deceptive characters | Unicode Tag block U+E0000–E007F (always `critical`); zero-width U+200B–U+200D, U+2060, U+FEFF (not at file start); bidi controls U+202A–U+202E, U+2066–U+2069; variation-selector runs |
| **D2** | Authority & execution patterns | instruction-override phrases; `curl …\| sh`, `wget …\| bash`, `iwr …\| iex`; long base64 / hex blobs; commands sending env vars or credential files over the network; reads of `~/.ssh`, `~/.aws`, `~/.claude`, `.env`; reverse-shell idioms |
| **D3** | Refusal bait | Text meant to make an LLM reviewer refuse or stop analysing; flagged `high` on its own |
| **D4** | Structural / consistency | inventory of every auto-execution point (§5.2); severity escalates with content (see §6.4); MCP servers launched through a shell or remote fetch; skill whose name/description does not match the commands or network access it uses |

- **FR-12** Every finding MUST contain: rule ID, severity, confidence (`low`/`medium`/`high`),
  file, line, column, a safe excerpt (escaped, ≤ 200 chars), message, remediation, OWASP
  mapping (ASI / MCP codes) and a stable **fingerprint** (for baselines and SARIF).
- **FR-13** v0.1 MUST ship **≥ 30 rules** spanning D1–D4.
- **FR-14** Per-file finding count is capped (default 50) with a summary line, so a hostile
  file cannot flood output.

### 6.4 Severity model

Severity reflects **what an attacker gains**, not merely what is present:

| Severity | Meaning | Example |
|---|---|---|
| `critical` | Near-certain malicious intent or direct code execution with exfiltration | Unicode Tag smuggling; hook that pipes a download to a shell |
| `high` | Strong attack indicator | refusal bait; instruction override; credential-file access in a hook |
| `medium` | Risky, needs human review | auto-exec command running an unknown local script; UTF-16 instruction file |
| `low` | Notable but commonly legitimate | `postinstall` running a build step; bidi mark in otherwise RTL text |
| `info` | Inventory only | "repository defines 2 Claude hooks" |

*Presence* of auto-execution is `info`/`low`; *suspicious content inside it* escalates.
This keeps legitimate repos (many use `prepare` or `postCreateCommand`) from failing CI
while still showing P1 the full inventory (US-03).

### 6.5 Rules

- **FR-20** Rules are YAML with a versioned schema; the default set is embedded in the binary.
- **FR-21** Every rule MUST ship ≥ 1 positive and ≥ 1 negative fixture; CI fails otherwise.
- **FR-22** Regular expressions MUST use Go `regexp` (RE2, linear time). No backtracking engines.
- **FR-23** Each rule carries metadata: id, title, severity, confidence, targets, OWASP
  mapping, description, remediation, references, and the rule-set version it was introduced in.

### 6.6 Output and exit codes

- **FR-30** Formats: `text` (default; colour only on a TTY), `json`, `sarif` (SARIF 2.1.0);
  added in Phase 5: `github` annotations, `markdown` summary and a self-contained `html` report.
- **FR-31** Output MUST be deterministic for identical input (stable ordering, no timestamps in findings).
- **FR-32** Exit codes: `0` nothing at/above threshold · `1` finding(s) at/above threshold · `2` usage error or incomplete analysis.
- **FR-33** `--fail-on <severity>` sets the threshold; default `high`.
- **FR-34** All scanned content echoed anywhere (terminal, JSON, SARIF, Markdown, HTML) MUST be escaped:
  control characters, ANSI sequences and bidi controls rendered as visible escapes.

### 6.7 Trust modes, configuration and suppression

- **FR-40** **Untrusted mode (default):** nothing inside the scan root can influence Portcullis —
  in-repo `.portcullis.yaml`, baselines and rule directories are ignored (their presence is reported as `info`).
- **FR-41** Configuration from **outside** the scan root is always allowed:
  `--config <file>` or the user config directory (`$XDG_CONFIG_HOME/portcullis`, `%APPDATA%\portcullis`).
- **FR-42** **Trusted mode** (`--trusted`, for a maintainer's own repository) honours in-repo
  config and baseline.
- **FR-43** Suppression is only possible through a **baseline file** keyed by finding
  fingerprint, each entry with a mandatory reason. **No inline "ignore" comments** in scanned files — ever.
- **FR-44** In pull-request context the GitHub Action MUST read config and baseline from the
  **base commit** (supplied via `--base-snapshot`, see ADR-0006). The tree is verified against
  the PR's own `context.lock` (so a context change without a lock update fails), and any
  change to config, baseline or lock relative to the base is reported (`medium`) so
  reviewers see exactly which files are being re-approved.

### 6.8 Context lock

- **FR-50** `portcullis lock` writes `context.lock`: every discovered target with its SHA-256,
  plus lock-format version and rule-set version.
- **FR-51** Hashes are computed over content with **CRLF normalised to LF**, so the same
  commit verifies identically on Windows (`core.autocrlf`) and Linux.
- **FR-52** Paths are stored relative, with forward slashes, sorted — diff-friendly.
- **FR-53** `portcullis verify` exits `1` on any added, modified or removed target and lists each.
- **FR-54** `portcullis lock` refuses to lock while findings ≥ threshold exist, unless `--force` is given (which is recorded in the lock).

### 6.9 Command-line interface

```
portcullis scan    [path] [--format text|json|sarif] [--output file] [--fail-on sev]
                     [--trusted] [--config file] [--baseline file]
portcullis lock    [path] [--force]
portcullis verify  [path]
portcullis rules   list | show <rule-id>
portcullis version
```

### 6.10 Integrations

- **FR-60** GitHub Action (in-repo `action.yml`): runs a scan, uploads SARIF, fails the job
  per threshold. Uses least-privilege permissions (`contents: read`, `security-events: write`),
  documented to run on `pull_request` (never `pull_request_target`), all actions pinned by SHA.
- **FR-61** `.pre-commit-hooks.yaml` definition.

## 7. Non-functional requirements

| ID | Category | Requirement |
|---|---|---|
| NFR-01 | Privacy | No network access during `scan`, `lock`, `verify`. No telemetry. |
| NFR-02 | Safety | Never executes, evaluates or templates scanned content. |
| NFR-03 | Robustness | No panic on any input; parsers and normaliser fuzz-tested (Go native fuzzing) in CI. |
| NFR-04 | Performance | Repo with < 5,000 files: < 1 s. 100,000 files: < 10 s on a mid-range laptop. Memory bounded by the per-file limit. |
| NFR-05 | Portability | Static binaries for Windows, Linux, macOS × amd64, arm64. |
| NFR-06 | Quality | ≥ 80% test coverage on core packages; `golangci-lint`, `go vet`, `govulncheck` clean. |
| NFR-07 | Supply chain | Few, well-maintained dependencies (stdlib + `golang.org/x/text` preferred); signed releases (cosign), SLSA provenance, OpenSSF Scorecard. |
| NFR-08 | Licence | Apache-2.0. |
| NFR-09 | Usability | Every finding states *why* and *what to do*. |

## 8. Success metrics (v0.1 release gate)

| Metric | Target | How measured |
|---|---|---|
| **Recall — known patterns** | **100%** | `testdata/corpus/known/`: hook RCE, Unicode Tag smuggling, DUSTMAKER-style refusal bait, `curl\|sh`, env exfiltration, malicious MCP launchers, … (all defanged) |
| **Recall — held-out evasions** | **≥ 80%**, always reported | `testdata/corpus/heldout/`: variants (homoglyphs, splitting, encoding, JSONC tricks) written *after* rules are frozen, so the number is honest rather than tuned |
| **False-positive rate** | **< 2%** | Pinned list of 1,000 popular public repos containing ≥ 1 target. Every `high`+ finding is triaged; FP rate = repos with ≥ 1 *false* `high`+ finding ÷ repos scanned. Per-rule precision is published; a rule with repeated FPs is demoted or fixed. Script and pinned list live in the repo. |
| **Performance** | NFR-04 | Go benchmarks + timed scan of a large public repo |
| **Robustness** | 0 crashes | Continuous fuzzing in CI |

## 9. Out of scope for v0.1 (v0.2+)

- Optional local LLM judge via Ollama (detector D5).
- Propagation log (recording writes to context files during an agent session).
- `portcullis open <repo>` (clone → scan → open editor if clean).
- Sigstore signing / verification of individual skills.
- Install hooks of other ecosystems (pip, cargo `build.rs`, Gradle, …).
- Confusable / mixed-script identifier detection beyond NFKC.

## 10. Constraints and assumptions

- Language: Go (latest stable). Rationale recorded as an ADR in Phase 1.
- Code, comments and primary docs in English; Turkish README (`README.tr.md`) provided.
- Test corpus contains **only defanged samples**: no working payloads, only reserved
  domains (`example.invalid`) and documentation IP ranges.
- Testing happens on the author's own systems or read-only against public repositories.

## 11. Decision log (Phase 0)

| # | Decision | Rationale |
|---|---|---|
| D-01 | Untrusted mode is the default; only outside-the-tree config is honoured | A scanned repo must never be able to configure its own scanner |
| D-02 | No inline ignore comments; suppression only via fingerprinted baseline with reasons | Same as D-01; makes every suppression reviewable |
| D-03 | Fail closed: incomplete analysis ⇒ exit `2`; oversize / unparseable targets ⇒ `high` finding | Padding and malformed files are common evasion techniques |
| D-04 | RE2-only regex, per-file size and finding caps | Hostile input cannot hang or flood the scanner |
| D-05 | Dual-view matching (raw + normalised) | Defeats zero-width splitting and full-width homoglyph evasions |
| D-06 | Severity reflects attacker gain; presence of auto-exec is `info`/`low` | Keeps FP rate low without hiding the inventory |
| D-07 | Default threshold `high` | Balance between safety and adoption in CI |
| D-08 | PR mode reads config / baseline / lock from base commit | A PR cannot weaken the check that reviews it |
| D-09 | Lock hashes normalise CRLF→LF | Same commit must verify on every OS |
| D-10 | Honest metrics: separate held-out recall, triaged FP rate, per-rule precision | Prevents self-congratulatory numbers |
