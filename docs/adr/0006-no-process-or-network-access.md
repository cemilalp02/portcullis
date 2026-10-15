# ADR-0006: Portcullis never starts processes or opens network connections

- Status: Accepted
- Date: 2026-10-13

## Context
Portcullis runs on untrusted repositories. Even "harmless" subprocesses are dangerous there:
running `git` inside a hostile checkout can trigger repository-controlled behaviour, and any
network access would break the privacy promise (NFR-01).

## Decision
The `portcullis` binary imports neither `os/exec` nor `net` / `net/http` (outside tests and
standalone scripts under `scripts/`). A CI check fails the build if such an import appears.
Information that would need git (base-commit copies of config, baseline and lock in PRs) is
supplied by the caller through `--base-snapshot <dir>`; the GitHub Action does the `git show`
in its own trusted step.

### Amendment (2026-10-15)
The HTML report uses `html/template`, whose contextual escaping depends on `net/url`, which in
turn uses `net/netip`. Both only parse strings: they open no sockets and resolve no names. They
are therefore allowed by name; every other package under `net/` and `net` itself remain
forbidden, and because the check covers all transitive dependencies, an allowed package can
never bring the socket code in. Our own code still may not import any `net` package
(depguard).

## Consequences
- Whole classes of vulnerabilities (command injection, SSRF, data leakage) are impossible by construction.
- `portcullis open <repo>` (v0.2) will need a separate, carefully reviewed design.
- Slightly more work in the Action script.

## Alternatives considered
- **Call `git` directly** — simpler UX, but reintroduces a process boundary on hostile input.
- **Use a pure-Go git library** — large dependency for a small need.
