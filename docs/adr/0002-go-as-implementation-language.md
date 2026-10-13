# ADR-0002: Go as the implementation language

- Status: Accepted
- Date: 2026-10-13

## Context
Bekçi runs in CI on every push and on developer machines before opening a repository.
It must start instantly, ship as one file, cross-compile, and parse hostile input safely.

## Decision
Implement Bekçi in Go (latest stable; `go.mod` declares the minimum supported version).

## Consequences
- Single static binary for 6 OS/arch targets via `goreleaser`; trivial install in CI.
- Memory-safe; RE2 `regexp` in the standard library gives linear-time matching (FR-22).
- Native fuzzing (`go test -fuzz`) for parsers (NFR-03).
- Same language as the planned Project 2 (Köken) proxy, so packages can be shared.

## Alternatives considered
- **Rust** — equally safe and fast; slower contributor onboarding and compile times, and no
  advantage for a mostly I/O + regex workload.
- **Python / TypeScript** — easier rule contribution, but a runtime dependency in CI, slower
  start-up, and a larger supply-chain surface for a security tool.
