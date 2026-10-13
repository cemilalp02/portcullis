# Bekçi — Threat Model (v0.1)

| Field   | Value |
|---------|-------|
| Status  | **Approved** for Phase 0; re-reviewed at every phase gate |
| Scope   | Bekçi v0.1 (`scan`, `lock`, `verify`, GitHub Action, pre-commit hook) |
| Related | [`REQUIREMENTS.md`](REQUIREMENTS.md) |

This document states plainly **what Bekçi protects against, what it does not, and how
Bekçi itself could be attacked.** A security tool that over-promises is itself a risk.

---

## 1. System overview

```
 untrusted repository ──TB1──► [ Bekçi: discover → decode → normalise → detect → report ] ──TB2──► human / CI
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
| Trust in Bekçi's verdict | A false "clean" is worse than no scan |

## 3. Trust boundaries

- **TB1 — Repository content → Bekçi.** Every scanned byte, file name, path and symlink is attacker-controlled.
- **TB2 — Bekçi output → human / CI.** Output must not be able to deceive or attack its consumer.
- **TB3 — Configuration → Bekçi behaviour.** Configuration inside an untrusted tree is attacker-controlled.
- **TB4 — Bekçi release → user machine.** The binary, rule set and Action are part of the user's supply chain.

## 4. Attacker model

| Attacker | Capability | Goal |
|---|---|---|
| **A1 — Malicious repo / skill author** | Full control over repository content | Make the victim's agent run commands or leak data |
| **A2 — Malicious contributor** | Opens PRs against a legitimate repository | Smuggle a hook / hidden instruction past review |
| **A3 — Compromised upstream** (package, skill registry) | Changes content after approval | Rug pull |
| **A4 — Scanner-aware attacker** | Knows Bekçi's rules (they are public) | Evade detection or attack Bekçi itself |

## 5. Threats Bekçi addresses

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
  language without known markers may not be detected. Bekçi reduces risk; it does not
  eliminate it. Runtime authority control (*Project 2 — Köken*) is the structural answer.
- **L2 — Novel obfuscation.** Encodings, languages or tricks not yet covered by rules.
  Held-out recall (REQUIREMENTS §8) is published so this gap is measured, not hidden.
- **L3 — Files outside the target list.** An agent may read any file (`README.md`, source
  comments). v0.1 scans the files agents load *automatically* or that execute automatically.
- **L4 — Runtime content.** Web pages, MCP tool *responses* and files fetched at runtime are invisible to a static scan.
- **L5 — Approval quality.** `context.lock` proves *no change since approval*, not that the approved content was safe.
- **L6 — Indirection.** A benign-looking hook that runs a local script (`./scripts/setup.sh`)
  is reported as `medium`, but the script's own content is not deeply analysed in v0.1.

## 7. Threats against Bekçi itself

| ID | Threat | Mitigation |
|---|---|---|
| S1 | Malicious input crashes or hangs the scanner (fail-open in CI) | Fuzzing; size limits; any incomplete analysis ⇒ exit `2`, never "clean" (FR-09, FR-32) |
| S2 | Regex denial of service | RE2 linear-time engine only (FR-22) |
| S3 | Symlink / path traversal outside the repo | Not followed; reported (FR-03) |
| S4 | Terminal / log injection via file names or excerpts | All echoed content escaped (FR-34) |
| S5 | In-repo config or baseline silences findings | Ignored in untrusted mode (FR-40) |
| S6 | Inline "ignore" comments inside scanned files | Not supported by design (FR-43) |
| S7 | PR edits Bekçi config / baseline / lock to weaken its own review | PR mode reads them from the base commit and reports the change (FR-44) |
| S8 | **Padding evasion** — payload placed after many MB of filler | Targets never skipped; oversize ⇒ `high` finding (FR-04) |
| S9 | **Encoding evasion** — instruction file saved as UTF-16 | Decoded and reported (FR-05) |
| S10 | **Normalisation evasion** — zero-width splitting, full-width letters | Dual-view matching (FR-10) |
| S11 | **Parser differential** — duplicate JSON keys, JSONC comments hiding a second definition | JSONC-aware parser, duplicate keys reported (FR-06, FR-08) |
| S12 | **Unparseable config** used to slip past structural rules | Text rules still run; `high` finding (FR-07) |
| S13 | **Case tricks** — `claude.md` instead of `CLAUDE.md` on case-insensitive file systems | Case-insensitive target matching (§5) |
| S14 | **Output flooding** to bury a real finding | Per-file finding cap with summary (FR-14); severity-sorted output |
| S15 | Bekçi executes scanned content | Never: no exec, no templating, no network (NFR-01, NFR-02) |
| S16 | Compromised Bekçi release, dependency or Action | Minimal deps, Dependabot, `govulncheck`, cosign-signed releases, SLSA provenance, SHA-pinned actions, OpenSSF Scorecard |
| S17 | Malicious samples in Bekçi's own corpus harm contributors | Corpus is defanged: no working payloads, reserved domains only |

## 8. Review cadence

Re-reviewed at the end of every phase and before each release. New attack techniques
observed in the wild are added here first, then become rules + fixtures.
