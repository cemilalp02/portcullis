# Validation

How well Portcullis works, measured against the v0.1 release gate (REQUIREMENTS §8). Every
number here can be reproduced from the repository; the commands are at the end. Phase 6 made
the first measurements; Phase 7 added a third held-out set, measured with the rules of the
v0.1.0 release; Phase 10 estimated the precision of findings below `high`; Phase 12 measured a fourth held-out set with the rules of
the v0.2.0 release.

## Summary

| Metric | Gate | Result | Status |
|---|---|---|---|
| False-positive rate, held-out repositories | < 2% | **1.60%** (8 of 500, rules frozen at `0e1e487`), **1.40%** (7 of 500, rules frozen at `e6ffee8`), **0.80%** (4 of 500, the v0.1.0 rules, frozen at `d4471f8`), **1.20%** (6 of 500, the v0.2.0 rules, frozen at `1a31198`) | ✅ |
| False-positive rate, development set | — | 0.20% (2 of 1,000); optimistic, the rules were tuned on it | — |
| Recall, known attacks (`corpus/known`) | 100% | 63 of 63 expected detections | ✅ |
| Recall, held-out evasions (`corpus/heldout`) | ≥ 80% | 45 of 46 (97.8%) now; **19 of 46 (41.3%) when first measured**, before the rules were improved on it | Met on a development figure; see [Recall](#recall) |
| Performance (NFR-04) | 5,000 files < 1 s; 100,000 files < 10 s | 65 ms; 1.5 s | ✅ |
| Robustness | 0 crashes | 0 crashes in the final run of all 8 fuzz targets; 2 defects found by fuzzing in Phase 6, both fixed | ✅ |
| Self-scan | no findings | 0 findings | ✅ |
| Precision of `medium` and `low` findings (sampled) | — | 76% before Phase 10, **84%** after; see [below](#precision-below-high) | — |

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

### The fourth held-out set (v0.2.0)

heldout4 was selected after all v0.2 work was merged, excluding every earlier list, and
scanned with the rules frozen at `1a31198`; its fetch also downloaded the scripts that hooks
and tasks run. 6 of 500 repositories (1.20%) had a false finding at or above `high`, within
the gate but above the 0.80% of v0.1.0 on heldout3. The new targets and rules held up: both
findings of PCL-EXE-015 were real installers piped into a shell in dev container scripts.

| Rule | tp / fp | False findings |
|---|---|---|
| PCL-AUT-002 | 1 / 6 | "without the user noticing" in a design reference (four copies of one file), a bug-class definition, a code comment |
| PCL-AUT-001 | 22 / 4 | Code that filters the phrase out; a heading naming the "system override" prompt technique |
| PCL-BAI-003 | 0 / 2 | A canned refusal returned by a guardrail example |
| PCL-AUT-005 | 23 / 2 | A documentation page named `agents.md` that lists agent commands a user can configure |
| PCL-ENG-003 | 0 / 1 | A duplicated key in a task that is not executed (`problemMatcher__`) |
| PCL-EXE-007 | 0 / 1 | A host command that only creates `~/.ssh` and `~/.gnupg` directories for mounts |

These causes are recorded for the next release; changing a rule because of them spends this
set, so the next measurement needs a fifth one.

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
- Only findings at or above `high` are triaged for the false-positive rate. The precision of
  lower severities is estimated from a sample in [Precision below `high`](#precision-below-high).
- The sets contain popular repositories. Small or private repositories may look different.

## Precision below `high`

Findings below `high` are not part of the false-positive gate, but they are what a user reads
first in a large repository. Phase 10 measured how often they are right.

### Method

From the development-set scan, up to ten `medium` and `low` findings of each rule were drawn
at random, one per repository where possible (210 findings). A finding is **right** when what
it states is true of the text: the skill does use the network, the file does name a credential
path, the word does mix scripts. Whether the text is harmful is a separate question; most right
findings at these severities are benign. Rules whose precision changed were sampled again
after the change with a new random seed. The estimate weights each rule's sample precision by
its number of findings.

### Results

Estimated precision rose from **76.2% to 84.4%**; the estimated number of wrong
findings fell from about 1,292 to about 743, while no finding at or
above `high` changed on the development set and every known attack is still detected.

| Rule | Findings before | Precision before | Findings after | Precision after |
|---|---:|---:|---:|---:|
| PCL-SKL-003 | 1,489 | 80% | 1,410 | 80% |
| PCL-SKL-001 | 1,060 | 60% | 936 | 60% |
| PCL-EXE-008 | 921 | 80% | 774 | 100% |
| PCL-EXE-002 | 381 | 90% | 381 | 90% |
| PCL-SKL-002 | 368 | 100% | 368 | 100% |
| PCL-AEX-004 | 226 | 100% | 226 | 100% |
| PCL-AUT-003 | 211 | 10% | 56 | 50% |
| PCL-UNI-003 | 158 | 80% | 113 | 100% |
| PCL-AUT-007 | 99 | 100% | 99 | 100% |
| PCL-AUT-008 | 80 | 80% | 67 | 90% |
| PCL-AUT-006 | 71 | 90% | 71 | 90% |
| PCL-AUT-004 | 59 | 50% | 48 | 100% |
| PCL-UNI-009 | 54 | 90% | 54 | 90% |
| PCL-EXE-003 | 46 | 20% | 9 | 89% |
| PCL-AEX-005 | 43 | 100% | 43 | 100% |
| PCL-MCP-002 | 43 | 100% | 43 | 100% |
| PCL-ENG-002 | 37 | 100% | 37 | 100% |
| PCL-BAI-004 | 32 | 0% | 0 | — |
| PCL-AEX-003 | 20 | 100% | 20 | 100% |
| PCL-ENG-014 | 11 | 100% | 11 | 100% |
| PCL-AEX-002 | 8 | 100% | 8 | 100% |
| PCL-UNI-005 | 1 | 100% | 1 | 100% |
| PCL-ENG-004 | 1 | 100% | 1 | 100% |
| **All** | **5,419** | **76.2%** | **4,776** | **84.4%** |

What was wrong, and the fix:

- **PCL-BAI-004** (0 of 10): words such as "bypass" or "disable" near a tool name in
  catalogues of vulnerability classes, lint rule names and "do not disable the guardrails".
  The rule now needs an instruction aimed at the tool ("disable gitleaks", "if the scanner
  complains, bypass it").
- **PCL-AUT-003** (1 of 10): chat-template tokens in tokenizer and fine-tuning code, `<system>`
  as a placeholder or an XML element. A start token now counts only when a role follows it,
  an end token only when text follows it, and tokens assigned to variables are skipped.
  Chat-format examples in fine-tuning documentation still match; by their text alone they
  cannot be told from an injected conversation.
- **PCL-EXE-003** (2 of 10): install commands shown in comments, error messages and help
  texts, and code that searches for the pattern.
- **PCL-AUT-004** (5 of 10): "Developer mode" of browsers, phones, Windows and ChatGPT. It
  now needs a claim that rules no longer apply.
- **PCL-UNI-003**: Khmer, Thai, Lao, Myanmar and Tibetan use U+200B as the word separator.
- **PCL-EXE-008**: a project `.npmrc` is configuration; only the one in the home directory
  holds tokens.
- **PCL-SKL-001**, **PCL-SKL-003**, **PCL-AUT-008**: requests to localhost or example.com,
  relative `fetch('/api')`, local `rsync`; "do not use sudo", guards that check for `sudo`;
  `card.profile` in Python.
- Messages of CRLF files quoted the carriage return (`<U+000D>`).
- An exclusion written as `\b(e\.g\.|for example|…)\b` never matched "e.g. " because no word
  boundary follows a period; it was fixed in ten rules.

### Limitations

- Ten findings per rule give a wide interval (a rule at 80% may be anywhere from about 45%
  to 95%); the totals are estimates, not counts.
- PCL-SKL-001 stays at about 60%: a skill that teaches HTTP code (`fetch(url)` in an example)
  reads like a skill that uses the network.
- The same person wrote the rules and judged the findings.

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
