package baseline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cemilalp02/portcullis/internal/finding"
)

const fpA = "0123456789abcdef0123456789abcdef"
const fpB = "fedcba9876543210fedcba9876543210"

func TestApply(t *testing.T) {
	f, err := Parse("b", []byte(`version: 1
suppressions:
  - fingerprint: `+fpA+`
    rule: PCL-AEX-004
    path: .vscode/tasks.json
    reason: reviewed the bootstrap script
  - fingerprint: `+fpB+`
    reason: old finding
`))
	if err != nil {
		t.Fatal(err)
	}
	fs := []finding.Finding{{RuleID: "PCL-AEX-004", Fingerprint: fpA}, {RuleID: "PCL-EXE-001", Fingerprint: "ffff0000ffff0000ffff0000ffff0000"}}
	kept, n, stale := f.Apply(fs)
	if n != 1 || len(kept) != 1 || kept[0].RuleID != "PCL-EXE-001" {
		t.Errorf("kept = %+v, suppressed = %d", kept, n)
	}
	if len(stale) != 1 || stale[0].Fingerprint != fpB {
		t.Errorf("stale = %+v", stale)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct{ yaml, want string }{
		{"version: 1\nsuppressions:\n  - fingerprint: xyz\n    reason: r\n", "invalid fingerprint"},
		{"version: 1\nsuppressions:\n  - fingerprint: " + fpA + "\n", "reason is required"},
		{"version: 1\nsuppressions:\n  - fingerprint: " + fpA + "\n    reason: '  '\n", "reason is required"},
		{"version: 1\nsuppressions:\n  - {fingerprint: " + fpA + ", reason: a}\n  - {fingerprint: " + fpA + ", reason: b}\n", "duplicate fingerprint"},
		{"version: 1\nsuppressions:\n  - {fingerprint: " + fpA + ", reason: a, why: b}\n", "field why not found"},
		{"version: 3\n", "unsupported version"},
	}
	for _, tt := range tests {
		if _, err := Parse("b", []byte(tt.yaml)); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%q: error = %v, want %q", tt.yaml, err, tt.want)
		}
	}
}

func TestGenerateRoundTrip(t *testing.T) {
	fs := []finding.Finding{
		{RuleID: "PCL-AEX-004", Path: "a: b/tasks.json", Fingerprint: fpA},
		{RuleID: "PCL-AEX-004", Path: "dup", Fingerprint: fpA},
		{RuleID: "PCL-MCP-002", Path: ".mcp.json", Fingerprint: fpB},
	}
	data, err := Generate(fs, "accepted when the baseline was created")
	if err != nil {
		t.Fatal(err)
	}
	f, err := Parse("gen", data)
	if err != nil {
		t.Fatalf("generated baseline does not parse: %v\n%s", err, data)
	}
	if len(f.Suppressions) != 2 || f.Suppressions[0].Path != "a: b/tasks.json" {
		t.Errorf("generated = %+v", f.Suppressions)
	}
	if _, n, _ := f.Apply(fs); n != 3 {
		t.Errorf("generated baseline suppresses %d of 3", n)
	}
	empty, err := Generate(nil, "x")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse("empty", empty); err != nil {
		t.Errorf("empty baseline: %v", err)
	}
}

func TestLoad(t *testing.T) {
	p := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(p, []byte("version: 1\nsuppressions: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err != nil {
		t.Error(err)
	}
	if _, err := Load(p + ".missing"); err == nil {
		t.Error("missing file loaded")
	}
}
