# ADR-0005: Dual-view text matching (raw + normalised)

- Status: Accepted
- Date: 2026-10-13

## Context
Pattern rules on raw text are trivially evaded: zero-width characters inside a word
(`ig​nore`), full-width letters (`ｉｇｎｏｒｅ`), mixed case. Conversely, invisible-character
rules need the untouched text.

## Decision
Every document has two views: **raw** (decoded text) and **normalised** (invisible / format
characters removed → NFKC → case-fold → horizontal whitespace collapsed). Rules choose a view;
pattern rules default to normalised. An offset map from normalised bytes to raw bytes keeps
reported line/column exact.

## Consequences
- Rule authors write simple lowercase patterns; evasion handling is centralised and tested once.
- NFKC can map several code points to one and vice versa — the offset map works per
  normalisation segment; correctness is enforced by property tests (ARCHITECTURE R1).
- Small CPU cost, paid only for target files.

## Alternatives considered
- **Normalise only** — loses invisible-character detection and correct positions.
- **Ask authors to handle variants in each regex** — error-prone, inconsistent coverage.
