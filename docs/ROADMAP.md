# Bekçi — Roadmap to v0.1

Each phase ends with a **gate**: measurable exit criteria that must be met before the next
phase starts. Skipping gates is how security tools end up noisy and unused.

| Phase | Name | Deliverables | Gate | Status |
|---|---|---|---|---|
| **0** | Requirements | `REQUIREMENTS.md`, `THREAT_MODEL.md`, `ROADMAP.md` | Scope, metrics and threat model approved | ✅ Done (2026-10-13) |
| **1** | Architecture & design | `ARCHITECTURE.md`, rule YAML schema, `context.lock` format, CLI spec, ADRs | Every FR maps to a component; no open design questions | ⏳ Next |
| **2** | Repository skeleton | Go module, layout, Makefile, `golangci-lint`, CI (lint/test/coverage/vuln), Apache-2.0, `SECURITY.md`, `CONTRIBUTING.md`, templates, Dependabot | Empty project green in CI; `bekci version` works | |
| **3** | Core engine | Discovery, decoding, JSONC parsing, normaliser, rule engine, text/JSON output | One rule works end-to-end; coverage ≥ 80% | |
| **4** | Detectors & rules | D1–D4, ≥ 30 rules with fixtures, defanged known-attack corpus | 100% recall on `corpus/known` | |
| **5** | Lock & integrations | `lock` / `verify`, SARIF, GitHub Action, pre-commit hook | Poisoned demo repo fails the Action; alert visible in Code Scanning | |
| **6** | Validation & hardening | FP benchmark (1,000 repos), held-out corpus, fuzzing, benchmarks, self-scan | FP < 2%, held-out recall ≥ 80%, 0 fuzz crashes, NFR-04 met | |
| **7** | Release | goreleaser builds, cosign signatures, SLSA provenance, README + demo GIF, `README.tr.md`, `CHANGELOG.md`, `v0.1.0` tag | Ready to publish on GitHub | |

## Working agreement

- One branch per unit of work; [Conventional Commits](https://www.conventionalcommits.org/).
- Test first: a rule's fixtures are written before the rule.
- Significant design decisions are recorded as ADRs in `docs/adr/`.
- `THREAT_MODEL.md` is reviewed at every gate.
