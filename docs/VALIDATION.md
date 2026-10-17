# Validation

How well Portcullis works, measured against the v0.1 release gate (REQUIREMENTS §8). Every
number here can be reproduced from the repository; the commands are at the end. Phase 6 made
the first measurements; Phase 7 added a third held-out set, measured with the rules of the
v0.1.0 release.

## Summary

| Metric | Gate | Result | Status |
|---|---|---|---|
| False-positive rate, held-out repositories | < 2% | **1.60%** (8 of 500, rules frozen at `0e1e487`), **1.40%** (7 of 500, rules frozen at `e6ffee8`), **0.80%** (4 of 500, the v0.1.0 rules, frozen at `d4471f8`) | ✅ |
| False-positive rate, development set | — | 0.20% (2 of 1,000); optimistic, the rules were tuned on it | — |
| Recall, known attacks (`corpus/known`) | 100% | 63 of 63 expected detections | ✅ |
| Recall, held-out evasions (`corpus/heldout`) | ≥ 80% | 45 of 46 (97.8%) now; **19 of 46 (41.3%) when first measured**, before the rules were improved on it | Met on a development figure; see [Recall](#recall) |
| Performance (NFR-04) | 5,000 files < 1 s; 100,000 files < 10 s | 65 ms; 1.5 s | ✅ |
| Robustness | 0 crashes | 0 crashes in the final run of all 8 fuzz targets; 2 defects found by fuzzing in Phase 6, both fixed | ✅ |
| Self-scan | no findings | 0 findings | ✅ |

## False positives

### Method

`scripts/bench/fpbench.py` selects popular public repositories (GitHub search by stars, no
forks or archived repositories, pushed after 2025-10-01) that have at least one agent context
file at the root, pins each to a commit and fetches only the files a Portcullis target can
match. Scans run in the default untrusted mode.

Every finding at or above `high` (the default `--fail-on` threshold) is read in context and
triaged. A finding is a **true positive** when the content has the property the rule describes,
in a file an agent reads or runs, whatever the author's intent; it is a **false positive** when
it does not. The false-positive rate is the share of repositories with at least one false
finding at or above `high`. Rules in `testdata/benchmark/README.md`.

Four disjoint sets were used:

| Set | Repositories | Used for |
|---|---|---|
| `repos.tsv` | 1,000 most-starred (≥ 15.6k stars) | Development: rules were changed after reading its findings |
| `heldout1-repos.tsv` | next 500 (9.1k–15.6k stars) | One measurement with the rules frozen at `0e1e487` |
| `heldout2-repos.tsv` | next 500 (5.7k–9.1k stars) | One measurement with the rules frozen at `e6ffee8` |
| `heldout3-repos.tsv` | next 500 (3.9k–5.7k stars, plus 15 more-starred repositories that added agent files after the earlier selections) | One measurement with the v0.1.0 rules, frozen at `d4471f8` |

A held-out set gives an independent rate once. Reading its findings and then changing a rule
spends it; the next measurement needs a new set (`fpbench.py select --exclude`).

### Results

| Set | Rules | Findings ≥ high | False | Repositories with a false one | Rate |
|---|---|---|---|---|---|
| heldout1 | frozen `0e1e487` | 24 | 11 | 8 / 500 | **1.60%** |
| heldout2 | frozen `e6ffee8` | 65 | 19 | 7 / 500 | **1.40%** |
| heldout3 | v0.1.0, frozen `d4471f8` | 53 | 10 | 4 / 500 | **0.80%** |
| development | final | 626 | 10 | 2 / 1,000 | 0.20% |

True positives by kind (the note of each triage line):

| Set | `payload` | `risk` | `benign` |
|---|---|---|---|
| development | 530 | 79 | 7 |
| heldout1 | 5 | 5 | 3 |
| heldout2 | 20 | 23 | 3 |
| heldout3 | 31 | 10 | 2 |

- `risk`: a real risky setting or instruction. Agents launched with permission checks off
  (`--dangerously-skip-permissions`, `--yolo`, `bypassPermissions`), every project MCP server
  enabled without approval, unrestricted `Bash` in shared settings, an installer piped into a
  shell from a dev container.
- `payload`: an attack example or test fixture that an agent would read. 4 security-skill
  collections in the development set account for 549 findings (reverse shells, encoded
  execution, prompt-injection catalogues). They are real payloads in agent context; a
  repository that ships them on purpose records them in a baseline.
- `benign`: true, but harmless in context, such as a copilot instruction that tells the model
  to ignore its own PR-description instructions for that repository.

### What the held-out sets found

Per-rule precision at or above `high`, frozen rules:

| Rule | heldout1 (tp / fp) | heldout2 (tp / fp) | heldout3 (tp / fp) | Cause | Fixed in |
|---|---|---|---|---|---|
| PCL-AUT-001 | 5 / 4 | 11 / 1 | 8 / 6 | Detection regexes, descriptions of the attack class, negated checklist items; in heldout3 a detector's catalogue of injection kinds that names the phrase | `e6ffee8`, `01b50df`; heldout3: open |
| PCL-AUT-002 | — | 2 / 4 | — | "Do not tell the user that X" forbids a false claim; it hides nothing | `01b50df` |
| PCL-AUT-005 | — | — | 4 / 3 | Documentation of what a sandbox runs; a negative test that checks a `--yolo` command is rejected | open |
| PCL-AUT-009 | — | 0 / 1 | — | A paper title in a Markdown link | `01b50df` |
| PCL-BAI-003 | 0 / 3 | — | — | "You must refuse to begin work" is policy wording | `e6ffee8` |
| PCL-ENG-003 | — | 1 / 3 | — | `commands` in an extension manifest treated as executing | `84c5fac` |
| PCL-EXE-005 | 0 / 1 | 3 / 0 | 3 / 0 | `eval(atob(...))` named as a pattern to look for | `e6ffee8` |
| PCL-EXE-006 | 0 / 3 | 5 / 0 | — | `echo >/dev/tcp/host/port` is a port check | `e6ffee8` |
| PCL-EXE-009 | — | 0 / 1 | 7 / 0 | `$USER` as the account in a GitHub URL | `01b50df` |
| PCL-EXE-010 | — | 0 / 5 | 5 / 0 | The name paste.rs in the manifest of a paste.rs client | `01b50df` |
| PCL-UNI-001 | — | 1 / 4 | 5 / 0 | Emoji flags of Scotland and Wales are tag sequences | `607e1df` |
| PCL-UNI-006 | — | — | 0 / 1 | Random bytes of a git-crypt encrypted `CLAUDE.md` | open |

heldout3 was measured with the rules of the v0.1.0 release and no rule changed afterwards, so
0.80% is the rate the release ships with. Its false findings come from four repositories: a
security-sweep skill whose detector catalogue and references name "ignore previous
instructions" as a kind of injection, a documentation page that shows the command a sandbox
runs, a test that checks a tampered `--yolo` command is rejected, and an encrypted
`CLAUDE.md`. The last one is not simply fixed by skipping files with a git-crypt header: a
forged header in front of plain text would then hide the text from every rule. These causes
are recorded for v0.2.

After these fixes, re-scanning the spent held-out sets leaves no false finding in heldout2
(46 findings) and none in heldout1 (14 findings; the one line the triage still marks false is a
fingerprint that moved to a true finding on another line of the same file). These are
development figures now.

The fixes cost one true finding in the development set: "never tell the user that evidence was
missing" is concealment, but reads exactly like "do not tell the user that the filing is
guaranteed", which is not. Portcullis now misses the first kind.

The remaining development-set false positives are PCL-AUT-005 in documentation: a CLI cheat
sheet that lists the flag with a comment, a page that documents which mode a wrapper records,
and unit tests of a permission handler that pass `bypassPermissions` to a mock session
(PCL-AUT-005 precision 84%, 54 / 64).

### Limitations

- One person triaged every finding, and the same person wrote the rules.
- Only findings at or above `high` are triaged. Lower severities are informational and
  numerous (for example 1,497 PCL-SKL-003 and 923 PCL-EXE-008 findings in the development
  set); their precision is not measured.
- The sets contain popular repositories. Small or private repositories may look different.

## Recall

| Corpus | Contents | Result |
|---|---|---|
| `known/` | Defanged samples of published attacks; each lists the rules that must fire | 63 / 63 (100%) |
| `heldout/` | Evasion variants written after the rules were frozen in Phase 4 | 45 / 46 (97.8%) |

The held-out corpus was measured once before any Phase 6 change: **19 of 46 (41.3%)**. Most
misses were not new attacks but other ways of writing the same text: HTML entities, Markdown
emphasis inside a word, line breaks, homoglyphs, leetspeak, shell quoting tricks (ADR-0010).
Phase 6 added the prose view, the shell view and new rules for those, and the corpus was
then used to check them. Its 97.8% is therefore a development figure, not an independent
recall measurement. The remaining miss (`wget-pipe-instruction`) is detected only at
`medium`.

An independent recall measurement needs a new evasion corpus written by someone other than
the rule author; it is carried over to the release phase (ROADMAP).

## Performance

Intel Core i9-14900HX, Windows 11, Go 1.27.2, default settings (`go test ./internal/engine
-bench`):

| Benchmark | Result | NFR-04 |
|---|---|---|
| 5,000 files, one agent file in 50 | 65 ms | < 1 s |
| 100,000 files, same mix | 1.5 s | < 10 s |
| One 2.2 MB `CLAUDE.md` | 1.2 s, 85 MB allocated | — |

On the 2,000 benchmark repositories (only agent files fetched), the median scan takes 0.04 s.
The slowest is a collection of 23,271 skill files (25,580 files in all): 10 s.

**Memory.** Analysis holds about 25 bytes per input byte (decoded text, the normalised and
prose views with their offset maps). With one worker per CPU and a 10 MiB per-file limit, a
repository of 64 files of 10 MB peaked at 9.5 GB on 32 threads. Workers now share a 32 MiB
budget of bytes under analysis (`f2f66fa`); the same repository peaks at 1.7 GB and takes 50 s
instead of 17 s. Ordinary repositories are not affected.

## Robustness

Fuzz targets: `FuzzString` (escape), `FuzzParse` (JSONC), `FuzzParse` (lock file),
`FuzzDecode` (encodings), `FuzzNormalize`, `FuzzProse`, `FuzzNormalizeMatchesReference`
(fast paths against a reference implementation) and `FuzzAnalyze` (every rule of every target
type on arbitrary bytes; findings must have a position and printable messages).

Defects found by fuzzing in Phase 6, both fixed with the input kept as a regression seed:

- `Normalize` was not idempotent on long runs of combining marks; it now repeats until the
  text is a fixed point and drops invisible characters NFKC emits.
- The lock-file parser accepted a version with a trailing carriage return that did not
  round-trip; versions are now validated.

Final run (2 minutes per target, 8 workers, on the Phase 6 head): no crash, about 107 million
inputs in all.

| Target | Inputs |
|---|---|
| `escape.FuzzString` | 16.8 M |
| `jsonc.FuzzParse` | 21.6 M |
| `lock.FuzzParse` | 25.3 M |
| `source.FuzzDecode` | 13.9 M |
| `textview.FuzzProse` | 10.7 M |
| `textview.FuzzNormalizeMatchesReference` | 9.1 M |
| `textview.FuzzNormalize` | 9.9 M |
| `engine.FuzzAnalyze` | 0.19 M (every rule runs on each input) |

## Reproduce

```bash
go test ./...                                     # fixtures, corpora, the 5,000-file guard
go test ./internal/engine -run '^$' -bench . -benchmem
python scripts/bench/fpbench.py fetch  --repos testdata/benchmark/heldout3-repos.tsv --cache /tmp/pcl-h3
python scripts/bench/fpbench.py scan   --repos testdata/benchmark/heldout3-repos.tsv --cache /tmp/pcl-h3 --bin ./portcullis --out h3.jsonl
python scripts/bench/fpbench.py report --results h3.jsonl --triage testdata/benchmark/heldout3-triage.tsv
```

Build `./portcullis` at the commit named in the triage header to reproduce a held-out
measurement exactly; a later build gives the development figures above.
