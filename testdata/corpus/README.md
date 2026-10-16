# Test corpus

| Directory | Contents | Test | Requirement |
|---|---|---|---|
| `known/` | Defanged samples of known attacks; each lists the rules that must fire | `TestKnownCorpus` | 100% recall (REQUIREMENTS §8, Phase 4 gate) |
| `clean/` | Realistic legitimate repositories | `TestCleanCorpus` | No finding at or above `high` |
| `heldout/` | Evasion variants written in Phase 6 without looking at the rules; each header names the severity a finding must reach (`alert:`) | `TestHeldoutCorpus` | At least 80% recall (REQUIREMENTS §8) |

## Format

Each sample is a [txtar](https://pkg.go.dev/golang.org/x/tools/txtar) archive: a free-text
header, then files introduced by `-- path --` lines. Known samples have an `expect:` line in the
header listing rule IDs:

```
Claude hook pipes a download into bash on every session start.
expect: PCL-EXE-001 PCL-AEX-001

-- .claude/settings.json --
{ "hooks": { ... } }
```

The tests unpack every archive into a temporary directory and run a full scan on it.

**Why archives instead of real files?** A `CLAUDE.md` or `.claude/settings.json` committed here
would be loaded by any AI agent working on this repository. Keeping samples inside `.txtar`
files means no agent ever reads them as instructions, and no editor runs them as tasks.

## Rules for samples

- **Defanged only:** domains under `example.invalid` (or well-known exfiltration services with
  dummy paths), IP addresses from `192.0.2.0/24`, credentials that are obviously fake.
- Samples may contain invisible, bidirectional and control characters on purpose; this directory
  is excluded from the repository hygiene test and is stored byte-exact (`.gitattributes`).
- Every detection rule should be exercised by at least one known sample; `TestKnownCorpus` logs
  the rules that are not.
- `heldout/` was measured once before any rule changed (19 of 46, 41.3%) and then used to
  improve the rules, so its current recall is a development figure, not an independent one
  (see `docs/VALIDATION.md`).
