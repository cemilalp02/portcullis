# ADR-0001: Record architecture decisions

- Status: Accepted
- Date: 2026-10-13

## Context
Portcullis is a security tool; users and contributors need to understand *why* it behaves as it
does, especially where safety was traded against convenience.

## Decision
Significant decisions are recorded as short ADRs (Michael Nygard format) in `docs/adr/`,
numbered sequentially, never deleted — superseded instead. Template: `0000-template.md`.

## Consequences
Design rationale survives contributor turnover; reviewers can point to an ADR instead of
re-arguing settled questions. Small overhead per decision.
