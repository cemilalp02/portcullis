# ADR-0003: Declarative YAML rules with mandatory inline fixtures

- Status: Accepted
- Date: 2026-10-13

## Context
Community growth for scanners comes from rules. Writing Go to add a pattern is a barrier;
untested rules are the main source of false positives.

## Decision
Rules are YAML (schema in `docs/RULE_SCHEMA.md`), one file per rule, embedded in the binary
with `go:embed`. Every rule contains positive and negative fixtures inline; the test suite
executes them automatically. Logic that cannot be expressed declaratively is a named Go
*built-in* with YAML metadata, so listing and documentation stay uniform.
YAML is decoded with `go.yaml.in/yaml/v3` in strict mode (`KnownFields(true)`).

## Consequences
- Adding a rule = one reviewed file; CI proves it fires and does not over-fire.
- Rule files are trusted input (embedded, or explicitly supplied from outside the scan root),
  so YAML parsing is not on the attacker-controlled path.
- The match language is intentionally small; complex heuristics move into built-ins.

## Alternatives considered
- **Semgrep / Sigma / OPA-Rego rules** — powerful but heavy dependencies and a poor fit for
  invisible-Unicode and exec-point semantics.
- **Rules in Go code** — maximum flexibility, much higher contribution barrier.
- **`gopkg.in/yaml.v3`** — unmaintained; `go.yaml.in/yaml/v3` is its maintained successor.
