## What and why

<!-- What does this change and which issue / requirement (FR-xx) does it address? -->

## How it was tested

<!-- Tests added, commands run, corpus results. -->

## Checklist

- [ ] `make check` passes locally (or the equivalent `go` commands)
- [ ] Tests added or updated; new rules include positive **and** negative fixtures
- [ ] Fixtures are defanged (no working payloads, only `example.invalid` / `192.0.2.0/24`)
- [ ] No new runtime dependency, or an ADR is included
- [ ] Docs updated (`docs/`, `CHANGELOG.md`) where behaviour changed
- [ ] `THREAT_MODEL.md` updated if this adds, removes or changes a protection
