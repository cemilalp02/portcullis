# Portcullis — Architecture (v0.1)

| Field   | Value |
|---------|-------|
| Status  | **Approved** — Phase 1 gate passed (2026-10-13) |
| Related | [`REQUIREMENTS.md`](REQUIREMENTS.md) · [`THREAT_MODEL.md`](THREAT_MODEL.md) · [`RULE_SCHEMA.md`](RULE_SCHEMA.md) · [`CLI.md`](CLI.md) · [`FILE_FORMATS.md`](FILE_FORMATS.md) · [`adr/`](adr/) |

---

## 1. Design principles

1. **Scanned content is data, never control.** Nothing inside the scan root can execute,
   configure, suppress or slow down Portcullis beyond linear cost (ADR-0006, D-01).
2. **Fail closed.** Every target is either fully analysed or produces a finding /
   non-zero exit. There is no code path that skips a target silently.
3. **Rules are data, the engine is small.** Detection knowledge lives in YAML; Go code
   provides parsing, normalisation, extraction and a handful of named built-in checks.
4. **Deterministic.** Same input + same rule set ⇒ byte-identical output.
5. **Few dependencies.** Standard library first (ADR-0007).

## 2. Pipeline

```
               ┌────────────┐
 CLI flags ───►│  config    │──► Settings (mode, threshold, limits, rules, baseline, base snapshot)
 out-of-tree   └────────────┘
 config               │
                      ▼
 scan root ──► ┌────────────┐   Candidates (target files)      Engine events
               │ discover   │──────────────────────────────┐  (symlink escape, oversize,
               └────────────┘                              │   read error, skipped-dir targets)
                                                           ▼
                     ┌───────── worker pool (GOMAXPROCS) ─────────┐
                     │  source   : read bytes → decode → Document │
                     │  textview : raw view + normalised view     │
                     │  jsonc    : parse configs (positions, dups)│
                     │  extract  : ExecPoints, MCP servers, Skills│
                     │  engine   : evaluate compiled rules        │
                     └────────────────────────────────────────────┘
                                                           │ []Finding
                                                           ▼
               ┌────────────┐   ┌────────────┐   ┌──────────────────┐
               │ baseline   │──►│  collect   │──►│ report: text /   │──► stdout / file
               │ (trusted)  │   │ sort, cap, │   │ json / sarif     │
               └────────────┘   │ exit code  │   └──────────────────┘
                                └────────────┘
```

`lock` and `verify` reuse `config → discover → source` and then hand the document set to
the `lock` package instead of the engine (`lock` also runs a scan first, FR-54).

## 3. Packages

All packages live under `internal/` — Portcullis exposes a CLI, not a Go API, in v0.1.

| Package | Responsibility | Key types |
|---|---|---|
| `cmd/portcullis` | `main`: wires CLI, maps errors to exit codes | — |
| `internal/cli` | Sub-command parsing (stdlib `flag`), help text | `Command` |
| `internal/config` *(Phase 5)* | Resolve settings from flags + out-of-tree config (+ in-tree / base snapshot in trusted mode) | `Settings`, `Mode` |
| `internal/target` | Embedded target definitions, case-insensitive path matcher | `Def`, `Family`, `Matcher` |
| `internal/discover` | Directory walk, skip policy, symlink policy, size checks | `Candidate`, `Event` |
| `internal/source` | Read bytes, detect BOM / UTF-16, decode, build line index | `Document`, `Encoding` |
| `internal/textview` | Normalised view with offset map back to raw text | `View`, `OffsetMap` |
| `internal/jsonc` | JSONC parser with byte positions and duplicate-key detection | `Node`, `ParseError`, `DupKey` |
| `internal/extract` | Config → facts: Claude hooks, VS Code tasks, dev-container commands, npm scripts, MCP servers, skill metadata | `ExecPoint`, `MCPServer`, `Skill` |
| `internal/ruleset` | Rule YAML schema, strict loading, validation, regex compilation | `Rule`, `Compiled`, `Set` |
| `internal/engine` | Evaluate rules on a document + facts; built-in checks registry; caps; fingerprints | `Engine`, `Builtin` |
| `internal/finding` | Finding model, severity / confidence enums, ordering | `Finding`, `Severity` |
| `internal/baseline` *(Phase 5)* | Load baseline, match by fingerprint | `Baseline` |
| `internal/lock` *(Phase 5)* | Read / write / verify `context.lock` | `File`, `Diff` |
| `internal/report` | `text`, `json`, `sarif` writers | `Writer` |
| `internal/escape` | Safe, truncated excerpts for any output channel | `Excerpt()` |
| `rules` (root package) | Embeds `targets.yaml` and `<category>/<RULE-ID>.yaml`; exports `FS` and the rule-set `Version` | `FS`, `Version` |

Dependency direction is strictly downward in the table; `finding` and `escape` are leaves.
No package below `cli` writes to stdout/stderr.

## 4. Core data model

```go
type Family string // "instruction" | "config"

type Document struct {
    Path     string     // relative, forward slashes, as found on disk
    Target   *target.Def
    Encoding source.Encoding // utf8, utf8-bom, utf16le, utf16be, invalid-utf8
    Text     string     // decoded UTF-8 (invalid sequences → U+FFFD, reported)
    Lines    source.LineIndex
    Norm     textview.View // normalised text + offset map into Text
}

type ExecPoint struct {
    Kind    string // claude_hook | vscode_task | devcontainer | npm_script | mcp_server
    Trigger string // e.g. "PreToolUse", "folderOpen", "initializeCommand (host)", "postinstall"
    Name    string // hook matcher, task label, script name, server name
    Command string // full command line as the consumer would run it
    Span    source.Span // raw byte range in Document.Text
}

type Finding struct {
    RuleID, Title          string
    Severity               Severity   // info < low < medium < high < critical
    Confidence             Confidence // low | medium | high
    Path                   string
    Line, Column           int        // 1-based; column counts Unicode code points
    EndLine, EndColumn     int
    Excerpt                string     // escaped, ≤ 200 code points
    Message, Remediation   string
    OWASP                  []string
    Fingerprint            string     // see §7
}
```

## 5. Key algorithms

### 5.1 Discovery (`discover`)

- `filepath.WalkDir` from the root; `.git` is never entered.
- Each path is matched case-insensitively against `targets.yaml` (FR §5). Non-targets are
  never opened — this is what keeps 100k-file repos under 10 s.
- `node_modules/`, `vendor/` (+ user `--exclude`) are not descended for *scanning*, but a
  cheap name-only pass counts targets inside them ⇒ `PCL-ENG-008` (info).
- Links are never followed. `filepath.EvalSymlinks` does **not** resolve Windows junctions (verified
  on Windows 11 / Go 1.27), so `discover.RealPath` resolves symlinks and junctions component by
  component with `os.Lstat` + `os.Readlink`. A target link leaving the root ⇒ `PCL-ENG-005`; a
  directory link leaving it ⇒ `PCL-ENG-011`; a broken or looping link ⇒ `PCL-ENG-012`. A target
  link inside the root is read at its resolved path.
- All reads go through `os.Root` (Go ≥ 1.24), so a path changed between discovery and reading
  still cannot escape the root.
- Size: checked from the directory entry and again while reading (`io.LimitReader`); above the
  hard limit ⇒ `PCL-ENG-001` (`high`), not analysed. The limit is capped at 1 GiB because the
  normalised view stores offsets as `int32`.
- Target patterns all start with `**/`: an editor or agent opened on a subfolder treats it as the
  project root, so `apps/web/.vscode/tasks.json` is as dangerous as `.vscode/tasks.json`.

### 5.2 Decoding (`source`)

BOM sniffing → UTF-8 / UTF-16LE / UTF-16BE. No BOM: validate UTF-8; if invalid, decode
with replacement and emit `PCL-ENG-004`. UTF-16 also emits `PCL-ENG-004` (`medium`).
A `LineIndex` (sorted line-start offsets) gives O(log n) offset → line/column.

### 5.3 Normalised view (`textview`) — ADR-0005

Built segment by segment with `golang.org/x/text/unicode/norm` iteration:

1. drop code points in the *invisible set* (Cf category, Tag block, variation selectors, bidi controls);
2. NFKC-normalise the segment (full-width → ASCII, ligatures → letters);
3. case-fold (`golang.org/x/text/cases.Fold`), then lower-case and NFKC again — fuzzing showed full
   case folding is not idempotent for Cherokee and can leave combining marks out of canonical order
   after a Turkish dotted İ;
4. collapse runs of horizontal whitespace to one space (newlines kept, so line units still work).

For every output byte the map stores the raw byte offset of the segment it came from, so a
match in the normalised view is reported at the correct raw line/column (FR-11).

### 5.4 JSONC (`jsonc`) — ADR-0004

Hand-written recursive-descent parser (~400 lines): comments (`//`, `/* */`), trailing
commas, byte spans on every node, **duplicate keys recorded** (FR-08), depth limit 256,
errors carry offsets. Fuzzed and differential-tested against `encoding/json` on plain JSON.
On parse failure the document still goes through text rules (FR-07, `PCL-ENG-002`).

### 5.5 Extraction (`extract`)

Pure functions `func(*Document, *jsonc.Node) Facts` per config target:

| Target | ExecPoints produced |
|---|---|
| `.claude/settings*.json` | each `hooks.<Event>[].hooks[].command` (Trigger = event) |
| `.vscode/tasks.json` | tasks with `runOptions.runOn == "folderOpen"`; command + args joined; OS-specific overrides (`windows`, `linux`, `osx`) each produce a point |
| `devcontainer.json` | `initializeCommand` (Trigger marked **host**), `onCreate`, `updateContent`, `postCreate`, `postStart`, `postAttach` — string, array and object forms |
| `package.json` | `preinstall`, `install`, `postinstall`, `prepare` |
| MCP configs | each server: `command` + `args` (Kind `mcp_server`), `url` for remote servers, `env` keys |

Skills: YAML front-matter of `SKILL.md` (`name`, `description`, `allowed-tools`) plus a list
of sibling files — input for the `skill_capability_mismatch` built-in.

### 5.6 Rule evaluation (`engine`)

Rules are compiled once (RE2). Per document:

- **`scope: text`** — unit is each *line* (default) or the whole *document*; view is `raw`
  or `normalized`. A unit matches if ≥ 1 `any` regex matches, all `all` regexes match and
  no `none` regex matches. `codepoints` rules scan the raw view and report each run.
- **`scope: exec`** — unit is each `ExecPoint` whose `Kind` is in `kinds` and whose
  normalised trigger matches `triggers` (if given); matching is on the normalised `Command`.
  `every: true` matches each point (used for the inventory-style rules).
- **`builtin: <name>`** — named Go check (registry in `engine/builtins.go`). v0.1 has one:
  `skill_capability_mismatch` reads the `SKILL.md` front-matter with a minimal line parser (no
  YAML parser ever sees scanned content) and reports network commands in the body when the
  name and description never mention network use. The special builtin `engine` marks rules
  that the engine emits itself (`PCL-ENG-*`).
- **Companion targets** — a target with `companion_of: skill` covers every file with one of its
  extensions below a directory that contains a `SKILL.md`: code becomes `skill-script`
  (family `script`), documents become `skill-doc` (family `instruction`). `discover` adds them
  after the main walk, without following links.
- **Unicode Tag runs** are decoded back to the ASCII text an LLM reads and shown through the
  `{decoded}` placeholder (PCL-UNI-001).

Caps: ≤ 50 findings per file (then one `PCL-ENG-010` summary), ≤ 5 per rule per file.

### 5.7 Collection, exit code

Findings from all workers are merged, baseline-filtered (trusted mode only), sorted by
(severity desc, path, line, column, rule ID). Exit code: `2` if any analysis was
incomplete (`PCL-ENG-006` read error, internal error), else `1` if any finding ≥ threshold, else `0`.

## 6. Trust model in code

```
Settings sources, in order (later wins):
  built-in defaults
  user config dir  (%APPDATA%\portcullis\config.yaml, $XDG_CONFIG_HOME/portcullis/config.yaml)
  --config <file>                                   (must resolve outside scan root, else error)
  [trusted only] <root>/.portcullis.yaml  OR  <base-snapshot>/.portcullis.yaml when --base-snapshot is set
  CLI flags
```

Untrusted mode never opens `.portcullis.yaml`, `.portcullis-baseline.yaml` or rule directories inside
the root; it only notes their presence (`PCL-ENG-007`, info). `--base-snapshot <dir>` is how
the GitHub Action supplies base-commit copies; Portcullis itself **never runs git** or any other
process (ADR-0006). If the tree's copy of a protected file differs from the snapshot ⇒
`PCL-ENG-009` (`medium`) so reviewers see that the PR touches Portcullis's own controls.

## 7. Fingerprints

`hex(sha256(ruleID ‖ 0x00 ‖ path ‖ 0x00 ‖ normalisedMatchText ‖ 0x00 ‖ k))[:32]`, where
`k` is the occurrence index of that same text in the file. Stable across line shifts
(unlike line numbers), unique per occurrence. Emitted to SARIF as
`partialFingerprints["portcullis/v1"]`.

## 8. Concurrency and performance

- Discovery is single-threaded (I/O bound, name matching only).
- Candidates go through a bounded worker pool; each worker owns its document — no shared mutable state.
- Results are sent on a channel and sorted at the end ⇒ determinism regardless of scheduling.
- Memory ≈ workers × max file size (10 MiB default) — bounded.

## 9. GitHub Action flow (FR-44, FR-60)

```
on: pull_request
1. actions/checkout (fetch-depth: 0)                         [SHA-pinned]
2. install portcullis release binary, verify checksum (+ cosign in v0.1 release)
3. for f in .portcullis.yaml .portcullis-baseline.yaml context.lock:
       git show $BASE_SHA:$f > $RUNNER_TEMP/portcullis-base/$f   (if it exists)
4. portcullis scan   --trusted --base-snapshot $RUNNER_TEMP/portcullis-base --format sarif --output portcullis.sarif
5. portcullis verify (tree vs PR's own context.lock; lock diff vs base reported as PCL-ENG-009)
6. github/codeql-action/upload-sarif                          [SHA-pinned, runs even on failure]
7. job result = worst exit code of 4 and 5
permissions: contents: read, security-events: write
```

Effect: a PR cannot weaken config/baseline (base copies are used), cannot change a context
file without updating `context.lock` (verify fails), and any lock update is highlighted for review.

## 10. Requirements traceability

| Requirement | Component(s) |
|---|---|
| FR-01, FR-02, FR-03, FR-04 | `discover`, `target` |
| FR-05, FR-09 | `source` |
| FR-06, FR-07, FR-08 | `jsonc`, `engine` built-ins |
| FR-10, FR-11 | `textview`, `source.LineIndex` |
| FR-12, FR-14 | `finding`, `engine`, `escape` |
| FR-13, FR-20–FR-23 | `rules`, `rules/` embedded set |
| D1–D3 | YAML rules (`scope: text`) |
| D4 | `extract` + YAML rules (`scope: exec`) + built-ins |
| FR-30–FR-34 | `report`, `escape`, `cmd/portcullis` |
| FR-40–FR-44 | `config`, `baseline`, Action (`--base-snapshot`) |
| FR-50–FR-54 | `lock` |
| FR-60, FR-61 | `action.yml`, `.pre-commit-hooks.yaml` |
| NFR-01, NFR-02 | ADR-0006; enforced by a CI check that forbids `os/exec`, `net`, `net/http` imports outside tests |
| NFR-03 | fuzz targets in `jsonc`, `source`, `textview`, `engine` |
| NFR-04 | §5.1, §8; benchmarks in Phase 6 |
| NFR-07 | ADR-0007 |

## 11. Technical risks and how Phase 3 retires them

| # | Risk | Mitigation |
|---|---|---|
| R1 | Offset map wrong after NFKC ⇒ wrong line numbers | Property test: for every match, raw excerpt re-normalises to the matched text; fuzz |
| R2 | JSONC parser disagrees with VS Code on edge cases | Differential tests vs `encoding/json`; corpus of real-world `tasks.json` / `devcontainer.json` |
| R3 | D2 phrase rules noisy on legitimate docs that *discuss* prompt injection | `none` exclusions, confidence levels, early FP sampling in Phase 4 rather than waiting for Phase 6 |
| R4 | Skill mismatch heuristic too vague | Ships as `medium` / `low` confidence in v0.1; precision measured before promotion |
| R5 | Windows junctions / mount points escape the root like symlinks (cf. GO-2026-6604, fixed in Go 1.27.2) | `discover` treats every reparse point (symlink, junction, `ModeIrregular`) as a link and applies FR-03; Windows-only tests in CI; `go.mod` requires a patched toolchain (≥ 1.26.9) |
