# ADR-0008: Line-based `context.lock` format

- Status: Accepted
- Date: 2026-10-13

## Context
`context.lock` is reviewed by humans in PR diffs and parsed by Bekçi from possibly tampered
input. It must be stable across operating systems.

## Decision
A `go.sum`-like text format (see `docs/FILE_FORMATS.md`): header lines, then
`sha256:<hex>  <path>` per target, sorted, paths quoted only when unusual. Hashes are
computed over content with CRLF normalised to LF. Any malformed line is a hard error.

## Consequences
- One line per file ⇒ minimal, readable PR diffs; trivial strict parser.
- CRLF normalisation makes `core.autocrlf` irrelevant; all other byte changes (BOM, invisible
  characters) still change the hash.

## Alternatives considered
- **JSON / YAML** — noisier diffs, needs a richer parser on a tampering-relevant path.
- **Hash the whole set as one digest** — tells you *that* something changed, not *what*.
