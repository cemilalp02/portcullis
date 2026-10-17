package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/cemilalp02/portcullis/internal/engine"
	"github.com/cemilalp02/portcullis/internal/finding"
)

func metaWithRules(t *testing.T) Meta {
	t.Helper()
	e, err := engine.Default()
	if err != nil {
		t.Fatal(err)
	}
	return Meta{Version: "v0.1.0", Ruleset: "2026.10.0", Mode: "trusted", FailOn: finding.High, Rules: e.Rules.Rules()}
}

func richResult() *engine.Result {
	res := sample()
	res.Findings = append(res.Findings,
		finding.Finding{RuleID: "PCL-ENG-008", Severity: finding.Info, Path: "dir x/node_modules", Line: 1, Column: 1, EndLine: 1, EndColumn: 1,
			Message: "3 agent files skipped", Remediation: "Scan it.", Fingerprint: "c"},
		finding.Finding{RuleID: "PCL-AUT-001", Severity: finding.High, Confidence: finding.ConfidenceMedium, Path: "docs/ç/AGENTS.md",
			Line: 2, Column: 5, EndLine: 2, EndColumn: 30, Excerpt: "Ignore all previous", Message: "override\n::warning file=x::injected, title",
			Remediation: "Remove it.", Fingerprint: "d"},
	)
	return res
}

func TestWriteSARIFValidatesAgainstSchema(t *testing.T) {
	m := metaWithRules(t)
	m.PathPrefix = "sub/dir/"
	var b bytes.Buffer
	if err := WriteSARIF(&b, richResult(), m); err != nil {
		t.Fatal(err)
	}

	c := jsonschema.NewCompiler()
	sch, err := c.Compile("testdata/sarif-schema-2.1.0.json")
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if err := sch.Validate(inst); err != nil {
		t.Fatalf("SARIF output violates the SARIF 2.1.0 schema: %v", err)
	}

	var log struct {
		Runs []struct {
			ColumnKind string `json:"columnKind"`
			Tool       struct {
				Driver struct {
					Rules []struct {
						ID         string         `json:"id"`
						HelpURI    string         `json:"helpUri"`
						Properties map[string]any `json:"properties"`
					} `json:"rules"`
				} `json:"driver"`
			} `json:"tool"`
			Results []struct {
				RuleID    string `json:"ruleId"`
				Level     string `json:"level"`
				Locations []struct {
					PhysicalLocation struct {
						ArtifactLocation struct {
							URI string `json:"uri"`
						} `json:"artifactLocation"`
						Region struct {
							StartColumn, EndColumn int
						} `json:"region"`
					} `json:"physicalLocation"`
				} `json:"locations"`
				PartialFingerprints map[string]string `json:"partialFingerprints"`
			} `json:"results"`
			Invocations []struct {
				ExecutionSuccessful bool `json:"executionSuccessful"`
			} `json:"invocations"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(b.Bytes(), &log); err != nil {
		t.Fatal(err)
	}
	run := log.Runs[0]
	if run.ColumnKind != "unicodeCodePoints" || !run.Invocations[0].ExecutionSuccessful {
		t.Errorf("run header = %+v", run)
	}
	if len(run.Tool.Driver.Rules) < 59 {
		t.Errorf("driver lists %d rules", len(run.Tool.Driver.Rules))
	}
	for _, r := range run.Tool.Driver.Rules {
		_, hasSev := r.Properties["security-severity"]
		if r.ID == "PCL-EXE-001" && (!hasSev || r.HelpURI != rulesDocURL+"pcl-exe-001") {
			t.Errorf("EXE-001 rule = %+v", r)
		}
		if r.ID == "PCL-AEX-001" && hasSev {
			t.Error("info rules must not carry a security-severity")
		}
	}
	uris := map[string]string{}
	for _, r := range run.Results {
		uris[r.RuleID] = r.Locations[0].PhysicalLocation.ArtifactLocation.URI
		if r.PartialFingerprints["portcullis/v1"] == "" {
			t.Errorf("%s lacks a fingerprint", r.RuleID)
		}
		if reg := r.Locations[0].PhysicalLocation.Region; reg.EndColumn <= reg.StartColumn && r.RuleID == "PCL-ENG-008" {
			t.Errorf("empty region for %s", r.RuleID)
		}
	}
	if uris["PCL-ENG-008"] != "sub/dir/dir%20x/node_modules" || uris["PCL-AUT-001"] != "sub/dir/docs/%C3%A7/AGENTS.md" {
		t.Errorf("uris = %v", uris)
	}

	b.Reset()
	res := sample()
	res.Complete, res.Errors = false, []string{"CLAUDE.md: permission denied"}
	if err := WriteSARIF(&b, res, metaWithRules(t)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), `"executionSuccessful": false`) || !strings.Contains(b.String(), "permission denied") {
		t.Errorf("incomplete run not reported:\n%s", b.String())
	}
	if err := WriteSARIF(&b, sample(), Meta{}); err == nil {
		t.Error("finding without rule metadata accepted")
	}
}

func TestWriteGitHubResistsInjection(t *testing.T) {
	m := metaWithRules(t)
	m.PathPrefix = "sub"
	var b bytes.Buffer
	if err := WriteGitHub(&b, richResult(), m); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")
	var commands []string
	for _, l := range lines {
		if strings.HasPrefix(l, "::") {
			commands = append(commands, l)
		}
	}
	// sample(): critical + medium, richResult adds info (skipped) + high.
	if len(commands) != 3 {
		t.Fatalf("got %d workflow commands, want 3 (injected text must not create more):\n%s", len(commands), b.String())
	}
	if !strings.HasPrefix(commands[0], "::error file=sub/CLAUDE.md,line=3,col=17,") {
		t.Errorf("first annotation = %q", commands[0])
	}
	if !strings.Contains(b.String(), "override%0A::warning file=x::injected, title%0AFix: Remove it.") {
		t.Errorf("message not escaped:\n%s", b.String())
	}
	if !strings.Contains(commands[1], "::warning ") {
		t.Errorf("medium finding should be a warning: %q", commands[1])
	}
	if !strings.HasPrefix(lines[len(lines)-1], "2 findings at or above 'high'") {
		t.Errorf("summary line = %q", lines[len(lines)-1])
	}

	b.Reset()
	res := sample()
	res.Complete, res.Errors = false, []string{"x: denied"}
	_ = WriteGitHub(&b, res, m)
	if !strings.Contains(b.String(), "::error title=Portcullis analysis incomplete::x: denied") {
		t.Errorf("incomplete analysis not annotated:\n%s", b.String())
	}
	if ghProperty("a:b,c%") != "a%3Ab%2Cc%25" {
		t.Error("ghProperty")
	}
}

func TestWriteMarkdown(t *testing.T) {
	res := richResult()
	res.Findings[0].Message = "click ![x](https://tracker.example.invalid/p.png) <img src=x>"
	m := metaWithRules(t)
	m.Suppressed = 2
	var b bytes.Buffer
	if err := WriteMarkdown(&b, res, m); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{
		"## Portcullis",
		"suppressed by baseline",
		"| 🔴 critical | [PCL-UNI-001](" + rulesDocURL + "pcl-uni-001) | CLAUDE\\.md:3 |",
		"<details><summary>2 findings below the threshold</summary>",
		"Auto-execution inventory: 2 commands",
		"\\!\\[x\\]\\(https:",
		"&lt;img src=x&gt;",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "![x](") || strings.Contains(out, "<img") {
		t.Errorf("scanned text became active markdown/HTML:\n%s", out)
	}

	b.Reset()
	res = sample()
	res.Inventory = res.Inventory[:1]
	_ = WriteMarkdown(&b, res, m)
	for _, want := range []string{"<details><summary>1 finding below the threshold</summary>", "Auto-execution inventory: 1 command runs without asking"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("markdown lacks %q:\n%s", want, b.String())
		}
	}

	b.Reset()
	res = &engine.Result{Complete: false, Errors: []string{"a: denied"}}
	_ = WriteMarkdown(&b, res, Meta{FailOn: finding.High})
	if !strings.Contains(b.String(), "[!WARNING]") || !strings.Contains(b.String(), "No findings at or above the threshold.") {
		t.Errorf("incomplete markdown:\n%s", b.String())
	}
}

func TestColorAndSuppressed(t *testing.T) {
	var b bytes.Buffer
	m := Meta{FailOn: finding.High, Color: true, Suppressed: 1}
	_ = WriteText(&b, sample(), m)
	if !strings.Contains(b.String(), "\x1b[1;31mCRITICAL ") || !strings.Contains(b.String(), "1 suppressed by baseline") {
		t.Errorf("colour/suppressed missing: %q", b.String())
	}
	b.Reset()
	_ = WriteText(&b, sample(), Meta{FailOn: finding.High})
	if strings.Contains(b.String(), "\x1b") {
		t.Error("colour emitted although disabled")
	}
	b.Reset()
	_ = WriteJSON(&b, sample(), m)
	if !strings.Contains(b.String(), `"suppressed": 1`) {
		t.Errorf("JSON lacks suppressed count:\n%s", b.String())
	}
	if ruleName("Download piped to a shell (curl|sh)") != "DownloadPipedToAShellCurlSh" {
		t.Errorf("ruleName = %q", ruleName("Download piped to a shell (curl|sh)"))
	}
}
