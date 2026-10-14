# Bekçi

[![CI](https://github.com/cemilalp02/bekci/actions/workflows/ci.yml/badge.svg)](https://github.com/cemilalp02/bekci/actions/workflows/ci.yml)
[![License: Apache-2.0](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

**Bekçi** (Turkish for *night watchman*) scans the files an AI coding agent reads or runs
automatically — `CLAUDE.md`, `AGENTS.md`, `SKILL.md`, `.cursorrules`, `.claude/settings.json`
hooks, `.mcp.json`, `.vscode/tasks.json`, dev-container and npm lifecycle commands — **before**
the agent sees them. It runs fully offline, emits SARIF for GitHub Code Scanning, and pins
approved context with `context.lock` so silent changes fail CI.

> **Status: pre-alpha, under active development.** Nothing is released yet.
> See the [roadmap](docs/ROADMAP.md).

## Documentation

| Document | What it covers |
|---|---|
| [Requirements](docs/REQUIREMENTS.md) | Scope, scanned surface, functional and non-functional requirements, success metrics |
| [Threat model](docs/THREAT_MODEL.md) | What Bekçi protects against — and what it does not |
| [Architecture](docs/ARCHITECTURE.md) | Pipeline, packages, algorithms, trust model |
| [Rule schema](docs/RULE_SCHEMA.md) | How detection rules are written and tested |
| [CLI](docs/CLI.md) · [File formats](docs/FILE_FORMATS.md) | Commands, exit codes, outputs, `context.lock` |
| [Decision records](docs/adr/) | Why things are the way they are |

## Building from source

```bash
go build ./cmd/bekci
```

Requires Go 1.26.9 or newer. See [CONTRIBUTING.md](CONTRIBUTING.md) for the development workflow.

## Security

Please report vulnerabilities privately — see [SECURITY.md](SECURITY.md).

## License

[Apache-2.0](LICENSE)
