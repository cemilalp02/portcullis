# ADR-0010: A prose view for natural-language rules

- Status: Accepted
- Date: 2026-10-15

## Context
The first held-out evasion corpus (Phase 6, written after the rules were frozen at 8058add)
reached 41.3% recall. Most misses were not new attacks but ways of writing the same
instruction that the normalised view (ADR-0005) does not undo:

- a phrase split by a soft line break, or by Markdown emphasis (`ig**no**re`);
- look-alike letters from another script inside a Latin word (Cyrillic `о` in `ignоre`), which
  NFKC leaves alone;
- HTML character references (`&#73;gnore`) and leetspeak (`1gn0r3`);
- the instruction written in Spanish, Turkish or another language.

Folding all of this into the normalised view would break rules that need the text as written:
digits in IP addresses and versions, line structure in configuration files, real Cyrillic
words.

## Decision
Add a third view, **prose**, derived from the normalised view (`textview.Prose`) and used only
by rules that match natural-language instructions (`view: prose`). It decodes HTML character
references and drops Markdown emphasis markers. It folds look-alike letters to Latin when the
word also contains Latin letters (or consists only of look-alikes), and turns digits into
letters in words that also contain letters. It joins a line break into a space when the
sentence continues on the next line. Code backticks stay, because rules use them to recognise
quoted examples. The offset map composes with the normalised view's, so positions stay exact.

The instruction rules gain phrasings in the main languages besides English. A separate rule
(PCL-UNI-009) reports words that mix Latin and Cyrillic letters, because the disguise itself
is a signal.

## Consequences
- One tested transformation instead of per-rule workarounds; rules stay readable.
- Genuine Cyrillic or Greek words are left intact, so rules can be written in those
  languages.
- The view is computed lazily, only for files that a prose rule inspects.
- Recall on the first held-out corpus is no longer an honest measure once rules were
  improved with knowledge of it. Phase 6 therefore writes a second held-out corpus after
  the improved rules are frozen, and that one is the gate (docs/VALIDATION.md).

## Alternatives considered
- **Extend the normalised view** — breaks rules that need digits, line structure or real
  non-Latin words.
- **More alternatives in each regex** — every rule would need its own copy of every
  evasion, which is what ADR-0005 set out to avoid.
- **A model-based classifier** — out of scope for an offline, deterministic scanner (ADR-0006).
