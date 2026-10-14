# Changelog

All notable changes to this project are documented here.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Changed

- Project renamed from Bekçi to **Portcullis** (ADR-0009): module `github.com/cemilalp02/portcullis`,
  binary `portcullis`, rule ID prefix `PCL-`, config `.portcullis.yaml`.

### Added

- `portcullis scan`: scans a directory for agent context files and reports findings as text or
  JSON, with an auto-execution inventory and exit codes 0/1/2 (fail closed).
- `portcullis rules list|show`.
- Core engine: discovery that never follows symlinks or Windows junctions, UTF-8/UTF-16
  decoding, normalised text view, JSONC parser with duplicate-key detection, extraction of
  auto-executed commands from Claude settings, VS Code tasks, dev containers, npm scripts and
  MCP configs, and a YAML rule engine with mandatory fixtures.
- Rules PCL-UNI-001 (Unicode Tag smuggling), PCL-EXE-001 (download piped to a shell in an
  auto-executed command) and engine integrity rules PCL-ENG-001 to PCL-ENG-012.
- Fuzzing of the parsers in CI.
- Requirements, threat model, architecture, rule schema, CLI and file-format specifications.
- Architecture decision records ADR-0001 to ADR-0009.
- Project skeleton: `portcullis version`, CI (lint, tests on Linux/Windows/macOS, govulncheck,
  cross-compilation), architecture test enforcing ADR-0006.
