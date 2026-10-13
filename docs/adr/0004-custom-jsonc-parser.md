# ADR-0004: Custom JSONC parser

- Status: Accepted
- Date: 2026-10-13

## Context
`.vscode/tasks.json` and `devcontainer.json` are JSON-with-comments and allow trailing
commas. Bekçi needs exact byte positions for every node (line/column in findings) and must
detect duplicate keys (FR-08) — a parser-differential smuggling vector that `encoding/json`
silently resolves (last key wins).

## Decision
Write a small recursive-descent JSONC parser in `internal/jsonc` producing a positioned AST,
recording duplicate keys and enforcing a nesting-depth limit (256).

## Consequences
- ~400 lines of code we own on the hostile-input path: mitigated by Go fuzzing and by
  differential tests against `encoding/json` on plain JSON inputs.
- No third-party dependency on the hottest attack surface.

## Alternatives considered
- **`encoding/json`** — no comments, no per-node positions, hides duplicate keys.
- **`github.com/tailscale/hujson`** — good JSONC support, but we would still need our own
  duplicate-key and position handling on top; adds a dependency for little gain.
