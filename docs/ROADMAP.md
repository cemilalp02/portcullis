# Portcullis — Roadmap to v0.1

Each phase ends with a **gate**: measurable exit criteria that must be met before the next
phase starts. Skipping gates is how security tools end up noisy and unused.

| Phase | Name | Deliverables | Gate | Status |
|---|---|---|---|---|
| **0** | Requirements | `REQUIREMENTS.md`, `THREAT_MODEL.md`, `ROADMAP.md` | Scope, metrics and threat model approved | ✅ Done (2026-10-13) |
| **1** | Architecture & design | `ARCHITECTURE.md`, `RULE_SCHEMA.md`, `CLI.md`, `FILE_FORMATS.md`, ADR-0001…0008, validation spike | Every FR maps to a component; no open design questions | ✅ Done (2026-10-13) |
| **2** | Repository skeleton | Go module, layout, Makefile, `golangci-lint`, CI (lint/test/coverage/vuln), Apache-2.0, `SECURITY.md`, `CONTRIBUTING.md`, templates, Dependabot | Empty project green in CI; `portcullis version` works | ✅ Done (2026-10-14) — local checks green; GitHub CI runs on first push |
| **3** | Core engine | Discovery, decoding, JSONC parsing, normaliser, rule engine, text/JSON output | One rule works end-to-end; coverage ≥ 80% | ✅ Done (2026-10-14) — 2 rules end-to-end, coverage 95.2% |
| **4** | Detectors & rules | D1–D4, ≥ 30 rules with fixtures, defanged known-attack corpus | 100% recall on `corpus/known` | ✅ Done (2026-10-15) — 48 detection rules, recall 64/64, 0 high findings in the clean corpus |
| **5** | Lock & integrations | `lock` / `verify`, SARIF, GitHub Action, pre-commit hook | Poisoned demo repo fails the Action; alert visible in Code Scanning | ✅ Done (2026-10-15) — action scripts run on a malicious pull request (exit 1, base config enforced); SARIF ingested by GitHub code scanning via API (Actions itself is billing-locked) |
| **6** | Validation & hardening | FP benchmark (1,000 repos), held-out corpus, fuzzing, benchmarks, self-scan | FP < 2%, held-out recall ≥ 80%, 0 fuzz crashes, NFR-04 met | ⏳ Next |
| **7** | Release | goreleaser builds, cosign signatures, SLSA provenance, README + demo GIF, `README.tr.md`, `CHANGELOG.md`, `v0.1.0` tag | Ready to publish on GitHub | |

## Working agreement

- One branch per unit of work; [Conventional Commits](https://www.conventionalcommits.org/).
- Test first: a rule's fixtures are written before the rule.
- Significant design decisions are recorded as ADRs in `docs/adr/`.
- `THREAT_MODEL.md` is reviewed at every gate.

## Carried over

| From | Item | Planned in |
|---|---|---|
| Phase 3 | Decode Unicode Tag runs into readable text in PCL-UNI-001 messages | ✅ Phase 4 |
| Phase 3 | Skill sibling files and `SKILL.md` front-matter for the skill-consistency check | ✅ Phase 4 |
| Phase 4 | Confusable / mixed-script identifier detection (homoglyphs beyond NFKC) | v0.2 |
| Phase 4 | Deep analysis of repository scripts called by hooks (L6) | v0.2 |
| Phase 3 | `--trusted`, `--config`, `--baseline`, `--base-snapshot`, `--rules`, `--color`, SARIF | ✅ Phase 5 |
| Phase 5 | Run the action itself on GitHub once Actions can start jobs on the account | when unblocked |
| Phase 5 | Action downloads signed release binaries instead of building from source | Phase 7 |
| Phase 3 | Memory use of the normalised view (~8 bytes per input byte) | Phase 6 |
| Phase 5 | Offline HTML report (`--format html`, Action artifact) | ✅ before Phase 6 |
| Phase 5 | Project website (GitHub Pages: landing page, rule catalogue, sample report) | Phase 7 |
| Phase 5 | VS Code extension showing findings in the editor | v0.2 |
