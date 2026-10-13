# ADR-0007: Minimal dependency policy

- Status: Accepted
- Date: 2026-10-13

## Context
A security tool is a supply-chain target (THREAT_MODEL S16). Each dependency is code we
ship into every user's CI.

## Decision
Runtime dependencies are limited to:

| Module | Purpose |
|---|---|
| `golang.org/x/text` | NFKC normalisation, case folding, UTF-16 decoding |
| `go.yaml.in/yaml/v3` | Rule, target, config and baseline decoding |

CLI parsing uses stdlib `flag`; SARIF/JSON use `encoding/json`; glob matching is our own.
Adding a runtime dependency requires a new ADR. Test-only dependencies (e.g. a JSON Schema
validator for SARIF tests) are allowed if they never enter the binary.

## Consequences
Small audit surface, fast builds, stable `govulncheck` results. We write a little more code
(CLI parsing, globbing) ourselves.

## Alternatives considered
- **cobra / urfave-cli** — nicer CLI ergonomics, many transitive dependencies.
- **doublestar** for globs — a fine library, but our target patterns are simple enough.
