package report

import (
	"bytes"
	"fmt"
	"html"
	"regexp"
	"strings"
	"testing"

	"github.com/cemilalp02/portcullis/internal/engine"
	"github.com/cemilalp02/portcullis/internal/finding"
)

func renderHTML(t *testing.T, res *engine.Result, m Meta) string {
	t.Helper()
	var b bytes.Buffer
	if err := WriteHTML(&b, res, m); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestWriteHTMLEscapesScannedText(t *testing.T) {
	// Every string below comes from a scanned repository in real use.
	const evil = `"><img src=x onerror=alert(1)></pre><script>alert(2)</script><!--`
	res := sample()
	res.Root = `<b onmouseover=alert(3)>repo`
	res.Complete, res.Errors = false, []string{evil}
	res.Findings = append(res.Findings,
		finding.Finding{RuleID: "PCL-AUT-001", Severity: finding.High, Path: "docs/" + evil + "/AGENTS.md", Line: 2, Column: 1,
			Title: evil, Message: evil, Excerpt: evil + "<U+200B>", Remediation: evil, Fingerprint: evil, OWASP: []string{evil}},
		finding.Finding{RuleID: `PCL-x" onclick="alert(4)`, Severity: finding.Low, Path: "CLAUDE.md", Line: 1, Column: 1, Fingerprint: "e"},
	)
	res.Inventory = append(res.Inventory, engine.InventoryItem{Kind: evil, Trigger: evil, Name: evil, Path: evil, Line: 1, Command: evil})
	res.TargetFiles = []engine.TargetFile{{Path: evil, Kind: evil}, {Path: "CLAUDE.md", Kind: "Claude memory file"}}

	out := renderHTML(t, res, metaWithRules(t))
	for _, bad := range []string{"<img", "<b on", "<!--", `onclick="`, "</pre><script"} {
		if strings.Contains(out, bad) {
			t.Errorf("scanned text reached the page unescaped: found %q", bad)
		}
	}
	if n := strings.Count(out, "<script"); n != 1 {
		t.Errorf("page has %d script elements, want only its own", n)
	}
	if n := strings.Count(out, "<svg"); n != 1 {
		t.Errorf("page has %d svg elements, want only the logo", n)
	}
	for _, want := range []string{
		`data-copy="&#34;&gt;&lt;img src=x onerror=alert(1)&gt;`,
		`<title>Portcullis · &lt;b onmouseover=alert(3)&gt;repo</title>`,
		`<mark class="esc" title="1 invisible character">&lt;U&#43;200B&gt;</mark>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q", want)
		}
	}
}

func TestHTMLPolicyCoversOnlyInlineAssets(t *testing.T) {
	out := renderHTML(t, richResult(), metaWithRules(t))

	m := regexp.MustCompile(`<meta http-equiv="Content-Security-Policy" content="([^"]*)">`).FindStringSubmatch(out)
	if m == nil {
		t.Fatal("no Content-Security-Policy meta element")
	}
	csp := html.UnescapeString(m[1])
	if !strings.HasPrefix(csp, "default-src 'none';") {
		t.Errorf("policy must deny by default: %s", csp)
	}
	for _, el := range []string{"style", "script"} {
		body := regexp.MustCompile(`(?s)<` + el + `>(.*?)</` + el + `>`).FindStringSubmatch(out)
		if body == nil {
			t.Fatalf("no inline %s", el)
		}
		if want := fmt.Sprintf("%s-src '%s'", el, cspHash(body[1])); !strings.Contains(csp, want) {
			t.Errorf("policy lacks %q, so the browser would block the inline %s", want, el)
		}
	}

	// Nothing may be loaded: no src attributes, no CSS imports or url()s,
	// and links only to the rule documentation.
	for _, bad := range []string{" src=", "@import", "url(", "http-equiv=\"refresh\""} {
		if strings.Contains(out, bad) {
			t.Errorf("page loads or redirects: found %q", bad)
		}
	}
	for _, h := range regexp.MustCompile(`href="([^"]*)"`).FindAllStringSubmatch(out, -1) {
		if !strings.HasPrefix(h[1], rulesDocURL) {
			t.Errorf("unexpected link %q", h[1])
		}
	}
}

func TestWriteHTMLVerdicts(t *testing.T) {
	onlyMedium := sample()
	onlyMedium.Findings = onlyMedium.Findings[1:]
	empty := sample()
	empty.Findings = nil
	incomplete := sample()
	incomplete.Complete, incomplete.Errors = false, []string{"CLAUDE.md: permission denied"}

	tests := []struct {
		name string
		res  *engine.Result
		m    Meta
		want []string
	}{
		{"fails", sample(), Meta{FailOn: finding.High, Suppressed: 2},
			[]string{"<h1>1 finding at or above high</h1>", `class="status fail"`, "2 suppressed by baseline", "exit code 1"}},
		{"below threshold", onlyMedium, Meta{FailOn: finding.High},
			[]string{"<h1>No findings at or above high</h1>", `class="status pass"`, "exit code 0"}},
		{"threshold disabled", sample(), Meta{NoFail: true},
			[]string{"<h1>2 findings, failure threshold disabled</h1>", "threshold: none", `class="status pass"`}},
		{"empty", empty, Meta{FailOn: finding.High},
			[]string{"<h1>No findings</h1>", "nothing matched a rule", "No agent context files were found."}},
		{"incomplete", incomplete, Meta{FailOn: finding.High},
			[]string{"<h1>Analysis incomplete</h1>", `class="status incomplete"`, `role="alert"`, "CLAUDE.md: permission denied", "exit code 2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := renderHTML(t, tt.res, tt.m)
			for _, w := range tt.want {
				if !strings.Contains(out, w) {
					t.Errorf("output lacks %q", w)
				}
			}
		})
	}
}

func TestWriteHTMLGroupsAndFolds(t *testing.T) {
	res := richResult()
	for i := range foldFilesOver + 1 {
		res.TargetFiles = append(res.TargetFiles, engine.TargetFile{Path: fmt.Sprintf("skills/s%02d/SKILL.md", i), Kind: "Agent skill"})
	}
	res.TargetFiles = append(res.TargetFiles, engine.TargetFile{Path: "CLAUDE.md", Kind: "Claude memory file"})
	out := renderHTML(t, res, Meta{FailOn: finding.High})

	if !strings.Contains(out, `<details class="fold">`) || !strings.Contains(out, "Show all 27 files") {
		t.Error("a long file list must start folded")
	}
	// Groups follow severity order and carry the file type and a tally.
	if i, j := strings.Index(out, `<span class="path">CLAUDE.md</span>`), strings.Index(out, `<span class="path">AGENTS.md</span>`); i < 0 || j < i {
		t.Errorf("critical file must come first (CLAUDE.md at %d, AGENTS.md at %d)", i, j)
	}
	if !strings.Contains(out, `<span class="kind">Claude memory file</span><span class="tally">1 critical</span>`) {
		t.Error("group header lacks kind or tally")
	}
	// Findings at or above the threshold start open, the rest closed.
	if !strings.Contains(out, `<details class="finding sev-critical" data-sev="critical"`) || !strings.Contains(out, `<details class="finding sev-medium below"`) {
		t.Error("threshold classes missing")
	}

	res.TargetFiles = res.TargetFiles[:3]
	if strings.Contains(renderHTML(t, res, Meta{FailOn: finding.High}), `class="fold"`) {
		t.Error("a short file list must not be folded")
	}
}

func TestExcerptSpans(t *testing.T) {
	tag := "<U+E0041>"
	tests := []struct {
		in   string
		want []htmlSpan
	}{
		{"plain text", []htmlSpan{{Text: "plain text"}}},
		{"", nil},
		{"a<U+200B>b", []htmlSpan{{Text: "a"}, {Text: "<U+200B>", Hidden: true, Title: "1 invisible character"}, {Text: "b"}}},
		{"<0xFF><U+202E>x", []htmlSpan{{Text: "<0xFF><U+202E>", Hidden: true, Title: "2 invisible characters"}, {Text: "x"}}},
		{"run " + strings.Repeat(tag, 6), []htmlSpan{{Text: "run "}, {Text: strings.Repeat(tag, 6), Hidden: true, Title: "6 invisible characters"}}},
		{strings.Repeat(tag, 30), []htmlSpan{{Text: strings.Repeat(tag, 4), Hidden: true, Title: "30 invisible characters", More: 26}}},
		{"not <U+12> or <u+200b>", []htmlSpan{{Text: "not <U+12> or <u+200b>"}}},
	}
	for _, tt := range tests {
		got := excerptSpans(tt.in)
		if fmt.Sprint(got) != fmt.Sprint(tt.want) {
			t.Errorf("excerptSpans(%q) =\n %v\nwant\n %v", tt.in, got, tt.want)
		}
	}
}
