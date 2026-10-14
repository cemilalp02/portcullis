# Contributing to Portcullis

Thanks for helping! The fastest way to make Portcullis better is a **well-tested rule** or a
**false-positive / missed-detection report** with a minimal, defanged example.

## Ground rules

- Read [`docs/THREAT_MODEL.md`](docs/THREAT_MODEL.md) and the [ADRs](docs/adr/) first —
  some "obvious" features (inline ignore comments, calling `git`, network lookups) are
  deliberately excluded.
- **Never commit working malicious payloads.** Fixtures use `example.invalid` domains,
  `192.0.2.0/24` addresses and inert commands.
- Be kind: see [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md).

## Development setup

Requirements: Go ≥ 1.26.9, git. Optional: GNU make, [golangci-lint v2](https://golangci-lint.run/).

```bash
go build ./cmd/portcullis     # build
go test ./...            # unit tests
make check               # everything CI runs: tidy, lint, test, govulncheck, actionlint
```

Without make, run the commands listed in the [`Makefile`](Makefile) directly — they are
plain `go` invocations.

## Workflow

1. Open or pick an issue; for larger changes, agree on the approach first.
2. Branch from `main`: `feat/…`, `fix/…`, `rule/…`, `docs/…`, `chore/…`.
3. Write the test first (for rules: the fixtures), then the change.
4. Run `make check` locally.
5. Commit with [Conventional Commits](https://www.conventionalcommits.org/):
   `feat(engine): …`, `fix(jsonc): …`, `rule: add PCL-EXE-007 …`, `docs: …`.
6. Open a PR using the template. CI must be green; one maintainer approval is required.

Significant design changes need an ADR (`docs/adr/0000-template.md`).

## Adding a rule

1. Follow [`docs/RULE_SCHEMA.md`](docs/RULE_SCHEMA.md); one YAML file per rule.
2. Include ≥ 1 positive and ≥ 1 negative fixture — the negative one should be a
   *realistic legitimate* case, not an empty file.
3. Choose severity by attacker gain and an honest confidence level.
4. Complete the review checklist in `RULE_SCHEMA.md` §6.

## Code style

- `golangci-lint run` must pass (formatting included).
- Packages under `internal/` never print; only `internal/cli` writes to the terminal.
- No `os/exec`, `net` or `plugin` in non-test code (ADR-0006) — enforced by lint and tests.
- New runtime dependencies need an ADR (ADR-0007).
