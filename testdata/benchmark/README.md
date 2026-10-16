# False-positive benchmark

| File | Contents |
|---|---|
| `repos.tsv` | The 1,000 repositories, pinned to a commit: the most-starred public repositories (forks and archived repositories excluded) that have at least one agent context file at the root. Selected on 2026-10-15 with `scripts/bench/fpbench.py select`. |
| `triage.tsv` | One line per finding at or above `high`: repository, fingerprint, rule, verdict (`tp` or `fp`) and a note. |
| `heldout1-repos.tsv` | 500 further repositories, selected on 2026-10-16 with `--exclude repos.tsv` after the rules were frozen at commit `0e1e487`. |
| `heldout1-triage.tsv` | Triage of that scan, same format. |

`repos.tsv` is the development set: rules were changed after reading its findings, so its
false-positive rate is optimistic. A held-out set gives an independent rate exactly once:
a rule change made after reading its triage spends it, and the next measurement needs a new
set selected with `--exclude` for every earlier list.

Run it (Python 3, `git` and an authenticated `gh` on the PATH):

```bash
python scripts/bench/fpbench.py fetch  --repos testdata/benchmark/repos.tsv --cache /tmp/pcl-bench
python scripts/bench/fpbench.py scan   --repos testdata/benchmark/repos.tsv --cache /tmp/pcl-bench --bin ./portcullis --out results.jsonl
python scripts/bench/fpbench.py report --results results.jsonl --triage testdata/benchmark/triage.tsv
```

`fetch` downloads only the files a Portcullis target can match (a sparse, blob-less clone at
the pinned commit, plus the folders of every `SKILL.md`). Scans run in the default untrusted
mode, as for a freshly cloned repository. `report` fails while any finding at or above `high`
is untriaged.

## Triage rules

A finding is a **true positive** when the matched content has the property the rule
describes, in a file an agent reads or runs, whatever the author's intent. A reverse shell in
a penetration-testing skill is a reverse shell: an agent can be told to run it.

The note of a true positive starts with what it is: `risk` (a real risky setting or
instruction), `payload` (an attack example or test fixture that an agent would read) or
`benign` (true, but harmless in context).

A finding is a **false positive** when the content does not have that property. Examples:
a token sent to its own API reported as exfiltration, CSS precedence reported as an
instruction override, or a bug description reported as concealment.

The false-positive rate is the share of scanned repositories with at least one false finding
at or above `high` (REQUIREMENTS §8). Per-rule precision is reported for every rule with
findings at or above `high`.
