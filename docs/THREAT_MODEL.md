# Portcullis — Threat Model (v0.1)

| Field   | Value |
|---------|-------|
| Status  | **Approved** for Phase 0; re-reviewed at every phase gate |
| Scope   | Portcullis v0.1 (`scan`, `lock`, `verify`, GitHub Action, pre-commit hook) |
| Related | [`REQUIREMENTS.md`](REQUIREMENTS.md) |

This document states plainly **what Portcullis protects against, what it does not, and how
Portcullis itself could be attacked.** A security tool that over-promises is itself a risk.

---

## 1. System overview

```
 untrusted repository ──TB1──► [ Portcullis: discover → decode → normalise → detect → report ] ──TB2──► human / CI
         ▲                                     ▲                                               │
         │                                    TB3 (config: only from outside the tree,         │ approve
         │                                         or base commit in PR mode)                  ▼
         └──────────────── AI coding agent reads repo only after approval ◄──── context.lock (pinned state)
```

## 2. Assets

| Asset | Why it matters |
|---|---|
| Developer machine & shell | Hooks, tasks and lifecycle commands execute with the developer's privileges |
| Credentials (SSH, cloud, AI provider tokens, `~/.claude`, MCP configs) | Primary target of observed 2026 campaigns |
| CI/CD secrets and runners | Poisoned context reaches CI through agent-authored PRs |
| Integrity of approved context files | Silent change after approval ("rug pull") |
| Trust in Portcullis's verdict | A false "clean" is worse than no scan |

## 3. Trust boundaries

- **TB1 — Repository content → Portcullis.** Every scanned byte, file name, path and symlink is attacker-controlled.
- **TB2 — Portcullis output → human / CI.** Output must not be able to deceive or attack its consumer.
- **TB3 — Configuration → Portcullis behaviour.** Configuration inside an untrusted tree is attacker-controlled.
- **TB4 — Portcullis release → user machine.** The binary, rule set and Action are part of the user's supply chain.

## 4. Attacker model

| Attacker | Capability | Goal |
|---|---|---|
| **A1 — Malicious repo / skill author** | Full control over repository content | Make the victim's agent run commands or leak data |
| **A2 — Malicious contributor** | Opens PRs against a legitimate repository | Smuggle a hook / hidden instruction past review |
| **A3 — Compromised upstream** (package, skill registry) | Changes content after approval | Rug pull |
| **A4 — Scanner-aware attacker** | Knows Portcullis's rules (they are public) | Evade detection or attack Portcullis itself |

## 5. Threats Portcullis addresses

| ID | Threat | OWASP | Control |
|---|---|---|---|
| T1 | Hidden instructions via invisible Unicode (Tag block, zero-width, bidi) | ASI06, MCP03 | D1 (raw view) |
| T2 | Instruction override / authority claims | ASI01, ASI06 | D2 (normalised view) |
| T3 | Execution payloads (`curl\|sh`, encoded blobs, env / credential exfiltration, reverse shells) | ASI02, ASI05 | D2 |
| T4 | Auto-execution on open / install (Claude hooks, VS Code folder-open tasks, dev-container commands incl. host-side `initializeCommand`, npm install scripts) | ASI04, ASI05 | D4 + auto-exec inventory |
| T5 | Malicious MCP server definitions (shell launchers, remote fetch-and-run) | MCP03, MCP06 | D4 |
| T6 | Refusal bait against LLM-based reviewers | ASI06 | D3 |
| T7 | Skill whose declared purpose mismatches its capabilities | ASI04 | D4 |
| T8 | Silent post-approval modification (rug pull) | ASI04 | `context.lock` + `verify` |
| T9 | Malicious PR adds risky context | ASI04 | GitHub Action + SARIF + lock diff |

## 6. Known limitations (explicitly NOT guaranteed)

- **L1 — Semantic / paraphrased injection.** A malicious instruction in ordinary, polite
  language without known markers may not be detected. Portcullis reduces risk; it does not
  eliminate it. Runtime authority control (*Project 2 — Köken*) is the structural answer.
- **L2 — Novel obfuscation.** Encodings, languages or tricks not yet covered by rules.
  Held-out recall (REQUIREMENTS §8) is published so this gap is measured, not hidden.
- **L3 — Files outside the target list.** An agent may read any file (`README.md`, source
  comments). v0.1 scans the files agents load *automatically* or that execute automatically.
- **L4 — Runtime content.** Web pages, MCP tool *responses* and files fetched at runtime are invisible to a static scan.
- **L5 — Approval quality.** `context.lock` proves *no change since approval*, not that the approved content was safe.
- **L6 — Indirection.** Scripts bundled with a skill are analysed (companion targets), and
  so are the repository scripts that hooks, tasks and lifecycle commands name (`run-script`,
  `run-program`).
  Only one level is followed: a script that the hook's script runs in turn, a path assembled
  at run time (`$DIR/$NAME.sh`) or a script outside the repository is not analysed; the hook
  itself is still reported by PCL-AEX-004.
- **L7 — Wording with a harmless twin.** To keep false positives under 2%, rules skip text
  that reads as documentation: short quoted phrases, descriptions of an attack class,
  negations, options tables, "do not tell the user that …" (which usually forbids a false
  claim). An attacker who phrases an instruction that way can fall into an exclusion. Every
  exclusion is a `none` pattern in the rule file, so the gap is visible and reviewable.

## 7. Threats against Portcullis itself

| ID | Threat | Mitigation |
|---|---|---|
| S1 | Malicious input crashes or hangs the scanner (fail-open in CI) | Fuzzing, including every rule of every target type on arbitrary bytes (`FuzzAnalyze`); size limits; any incomplete analysis ⇒ exit `2`, never "clean" (FR-09, FR-32) |
| S2 | Regex denial of service | RE2 linear-time engine only (FR-22) |
| S3 | Symlink / junction / path traversal outside the repo | Links never followed; resolved component by component (junctions included, which `EvalSymlinks` misses) and reported; all reads through `os.Root` (FR-03) |
| S4 | Terminal / log injection via file names or excerpts | All echoed content escaped (FR-34) |
| S5 | In-repo config or baseline silences findings | Ignored in untrusted mode (FR-40) |
| S6 | Inline "ignore" comments inside scanned files | Not supported by design (FR-43) |
| S7 | PR edits Portcullis config / baseline / lock to weaken its own review | PR mode reads them from the base commit and reports the change (FR-44) |
| S8 | **Padding evasion** — payload placed after many MB of filler | Targets never skipped; oversize ⇒ `high` finding (FR-04) |
| S9 | **Encoding evasion** — instruction file saved as UTF-16 | Decoded and reported (FR-05) |
| S10 | **Normalisation evasion** — zero-width splitting, full-width letters | Dual-view matching (FR-10) |
| S11 | **Parser differential** — duplicate JSON keys, JSONC comments hiding a second definition | JSONC-aware parser, duplicate keys reported (FR-06, FR-08) |
| S12 | **Unparseable config** used to slip past structural rules | Text rules still run; `high` finding (FR-07) |
| S13 | **Case tricks** — `claude.md` instead of `CLAUDE.md` on case-insensitive file systems | Case-insensitive target matching (§5) |
| S14 | **Output flooding** to bury a real finding | Per-file finding cap with summary (FR-14); severity-sorted output |
| S15 | Portcullis executes scanned content | Never: no exec, no templating, no network (NFR-01, NFR-02) |
| S16 | Compromised Portcullis release, dependency or Action | Two runtime dependencies, Dependabot, `govulncheck`; releases built only by the release workflow, `checksums.txt` signed keylessly with Sigstore, SLSA build provenance for every archive and SBOM, published only after the provenance exists; reproducible builds so anyone can rebuild a tag and compare; SHA-pinned actions; OpenSSF Scorecard (`docs/RELEASING.md`) |
| S17 | Malicious samples in Portcullis's own corpus harm contributors | Corpus is defanged: no working payloads, reserved domains only |
| S18 | Unstable normalisation (same text normalises differently on a second pass) lets a pattern match only sometimes | Fuzz-tested idempotency; two real cases (Cherokee folding, dotted İ + combining mark) found and fixed |
| S19 | Hidden characters in Portcullis's own sources or docs (Trojan Source) | Repository hygiene test fails on any format/bidi/Tag/filler character outside `testdata/` |
| S20 | Scanned text injects GitHub workflow commands through annotations (e.g. a message containing a newline followed by `::set-output` or `::add-mask`) | Annotation data and properties are percent-encoded; newlines cannot start a new command (tested) |
| S21 | Scanned text renders as links, images or HTML in the Markdown job summary (tracking pixels, phishing links) | All scanned text is HTML-escaped and Markdown punctuation is backslash-escaped (tested) |
| S22 | Script injection through action inputs | Inputs reach `scripts/action/*.sh` only as environment variables; scripts pass shellcheck |
| S23 | Attack samples in Portcullis's own repository are loaded by agents working on it | Samples are stored inside `.txtar` archives, never as real agent files |
| S24 | Scanned text runs script or loads content in the HTML report (stored XSS, tracking requests when the report is opened) | Contextual escaping by `html/template`; Content Security Policy `default-src 'none'` that allows only the report's own style and script by SHA-256 hash, plus `data:` fonts and icon embedded by Portcullis; no event-handler attributes; no external resources or links other than rule documentation (tested, including a mutation check of the hashes) |
| S25 | **Memory exhaustion** — many agent files near the size limit, analysed in parallel (64 files of 10 MB took 9.5 GB on 32 threads) | Workers share a 32 MiB budget of bytes under analysis; the same input peaks at 1.7 GB (`docs/VALIDATION.md`) |
| S26 | Hidden text in an emoji tag sequence, which UNI-001 exempts so that the flags of Scotland or Wales are not reported | Only U+1F3F4 followed by three to six lowercase tag letters or digits and CANCEL TAG is exempt; longer runs or any other tag character are reported |
| S27 | The GitHub Action runs a replaced or tampered release archive (compromised release asset, download redirect) | The action runs a downloaded binary only if `gh attestation verify` proves it was built by `release.yml` for that exact tag on a GitHub-hosted runner and its SHA-256 matches `checksums.txt`; any failure stops the job instead of falling back. Without a release archive it builds from its own source at the pinned ref (tested offline by `scripts/action/install_test.sh`) |
| S29 | A hook runs a harmless-looking repository script that does the damage (`bash .claude/hooks/setup.sh`) | The script is read through `os.Root` (no path can leave the scan root) and analysed; PCL-EXE-015 (shell scripts) and PCL-EXE-016 (programs) report downloads or encoded commands it executes. At most 256 such scripts are read, sorted by path; the hook command itself is always reported by PCL-AEX-004 |
| S30 | Look-alike letters from another script hide a word from pattern rules (`bаsh`, `ignоre`) | The normalised view folds them to Latin inside mixed words before rules run; PCL-UNI-009 reports the mixed word itself (tested, fuzzed against a reference implementation) |
| S28 | A forged git-crypt header makes Portcullis skip a file as encrypted | A file counts as encrypted only when no 16 readable characters (any script) follow the header; readable text behind a forged header is analysed like any other file (tested) |

## 8. Review cadence

Re-reviewed at the end of every phase and before each release. New attack techniques
observed in the wild are added here first, then become rules + fixtures.
