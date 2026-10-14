package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cemilalp02/portcullis/internal/engine"
	"github.com/cemilalp02/portcullis/internal/finding"
)

func sample() *engine.Result {
	return &engine.Result{
		Files: 10, Targets: 3, Complete: true,
		Findings: []finding.Finding{
			{RuleID: "PCL-UNI-001", Severity: finding.Critical, Path: "CLAUDE.md", Line: 3, Column: 17, Message: "6 invisible Unicode Tag characters", Excerpt: "Build.<U+E0069>", Remediation: "Remove them.", Fingerprint: "a"},
			{RuleID: "PCL-ENG-004", Severity: finding.Medium, Path: "AGENTS.md", Line: 1, Column: 1, Message: "File is encoded as utf-16le", Remediation: "Convert.", Fingerprint: "b"},
		},
		Inventory: []engine.InventoryItem{
			{Kind: "claude_hook", Trigger: "SessionStart", Path: ".claude/settings.json", Line: 4, Command: "curl x | sh"},
			{Kind: "npm_script", Trigger: "postinstall", Name: "postinstall", Path: "package.json", Line: 9, Command: "node build.js"},
		},
	}
}

func TestExitCode(t *testing.T) {
	res := sample()
	if got := ExitCode(res, Meta{FailOn: finding.High}); got != ExitFindings {
		t.Errorf("high threshold: %d", got)
	}
	if got := ExitCode(res, Meta{FailOn: finding.Critical}); got != ExitFindings {
		t.Errorf("critical threshold: %d", got)
	}
	res.Findings = res.Findings[1:]
	if got := ExitCode(res, Meta{FailOn: finding.High}); got != ExitOK {
		t.Errorf("only medium, high threshold: %d", got)
	}
	if got := ExitCode(sample(), Meta{NoFail: true}); got != ExitOK {
		t.Errorf("--fail-on none: %d", got)
	}
	res.Complete = false
	if got := ExitCode(res, Meta{NoFail: true}); got != ExitError {
		t.Errorf("incomplete must exit 2 even with --fail-on none: %d", got)
	}
}

func TestWriteText(t *testing.T) {
	var b bytes.Buffer
	if err := WriteText(&b, sample(), Meta{Version: "v0.1.0", Ruleset: "2026.10.0", Mode: "untrusted", FailOn: finding.High, Inventory: true}); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{
		"portcullis v0.1.0 · ruleset 2026.10.0 · mode: untrusted · 3 agent files in 10 files",
		"CRITICAL  PCL-UNI-001  CLAUDE.md:3:17",
		"          > Build.<U+E0069>",
		"          fix: Remove them.",
		"MEDIUM    PCL-ENG-004  AGENTS.md:1:1",
		"Auto-execution inventory (runs without asking):",
		"  claude_hook  SessionStart  .claude/settings.json:4  curl x | sh",
		"  npm_script   postinstall   package.json:9           node build.js",
		"1 finding at or above 'high' (1 critical) · 1 below · exit 1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("text output lacks %q:\n%s", want, out)
		}
	}
}

func TestWriteTextVariants(t *testing.T) {
	res := &engine.Result{Complete: false, Errors: []string{"CLAUDE.md: permission denied"}, Inventory: sample().Inventory}
	var b bytes.Buffer
	if err := WriteText(&b, res, Meta{NoFail: true, Inventory: false}); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{"No findings.", "Analysis incomplete", "  CLAUDE.md: permission denied", "0 findings at or above the threshold (none) · 0 below · exit 2"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "inventory") {
		t.Error("inventory shown although disabled")
	}
	path := &engine.Result{Complete: true, Findings: []finding.Finding{{Path: "a\x1b[2J.md", Severity: finding.Low}}}
	b.Reset()
	_ = WriteText(&b, path, Meta{FailOn: finding.High})
	if strings.Contains(b.String(), "\x1b") || !strings.Contains(b.String(), "a<U+001B>[2J.md") {
		t.Errorf("path not escaped: %q", b.String())
	}
}

func TestWriteJSON(t *testing.T) {
	res := sample()
	res.Findings[0].Message = "bidi " + string(rune(0x202E)) + " and tag " + string(rune(0xE0041))
	var b bytes.Buffer
	if err := WriteJSON(&b, res, Meta{Version: "v0.1.0", Ruleset: "2026.10.0", Mode: "untrusted", FailOn: finding.High}); err != nil {
		t.Fatal(err)
	}
	raw := b.String()
	if strings.ContainsRune(raw, 0x202E) || strings.ContainsRune(raw, 0xE0041) {
		t.Error("raw JSON contains hidden characters")
	}
	if !strings.Contains(raw, `\u202e`) || !strings.Contains(raw, `\udb40\udc41`) {
		t.Errorf("hidden characters not escaped as JSON escapes:\n%s", raw)
	}
	var got map[string]any
	if err := json.Unmarshal(b.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got["schema"] != SchemaURL || got["exitCode"].(float64) != 1 || got["complete"] != true || got["failOn"] != "high" {
		t.Errorf("header fields: %v", got)
	}
	stats := got["stats"].(map[string]any)
	if stats["aboveThreshold"].(float64) != 1 || stats["findings"].(float64) != 2 {
		t.Errorf("stats = %v", stats)
	}
	f0 := got["findings"].([]any)[0].(map[string]any)
	if f0["severity"] != "critical" || f0["message"] != res.Findings[0].Message {
		t.Errorf("finding round-trip = %v", f0)
	}

	b.Reset()
	if err := WriteJSON(&b, &engine.Result{Complete: true}, Meta{NoFail: true}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"findings": []`, `"inventory": []`, `"errors": []`, `"failOn": "none"`} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("empty report lacks %s:\n%s", want, b.String())
		}
	}
}
