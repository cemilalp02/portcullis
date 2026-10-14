# Changelog

All notable changes to this project are documented here.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Changed

- Project renamed from Bekçi to **Portcullis** (ADR-0009): module `github.com/cemilalp02/portcullis`,
  binary `portcullis`, rule ID prefix `PCL-`, config `.portcullis.yaml`.

### Added

- Requirements, threat model, architecture, rule schema, CLI and file-format specifications.
- Architecture decision records ADR-0001 to ADR-0009.
- Project skeleton: `portcullis version`, CI (lint, tests on Linux/Windows/macOS, govulncheck,
  cross-compilation), architecture test enforcing ADR-0006.
