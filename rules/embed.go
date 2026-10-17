// Package rules embeds Portcullis's default target definitions and detection
// rules (ADR-0003). The YAML files next to this file are the source of truth;
// see docs/RULE_SCHEMA.md for the format.
package rules

import "embed"

// Version is the version of the embedded rule set. Bump it whenever a rule or
// target definition changes; it is recorded in context.lock files.
const Version = "2026.10.3"

// FS holds targets.yaml and every rule file (<category>/<RULE-ID>.yaml).
//
//go:embed targets.yaml */*.yaml
var FS embed.FS
