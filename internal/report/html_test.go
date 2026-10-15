package report

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"html"
	"os"
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
	// No element of the page may carry an event handler attribute; the
	// report's own markup never uses them. Quoted attribute values are
	// removed first: escaped text inside them is not an attribute.
	quoted := regexp.MustCompile(`"[^"]*"`)
	handler := regexp.MustCompile(`\son[a-z]+\s*=`)
	for _, tag := range regexp.MustCompile(`<[a-zA-Z][^>]*>`).FindAllString(out, -1) {
		if handler.MatchString(quoted.ReplaceAllString(tag, `""`)) {
			t.Errorf("element with an event handler: %q", tag)
		}
	}
	for _, want := range []string{
		`data-copy="&#34;&gt;&lt;img src=x onerror=alert(1)&gt;`,
		`<title>&lt;b onmouseover=alert(3)&gt;repo · Portcullis report</title>`,
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

	// Fonts and the icon are embedded; nothing else may be loaded.
	for _, d := range []string{"font-src data:;", "img-src data:;"} {
		if !strings.Contains(csp, d) {
			t.Errorf("policy lacks %q", d)
		}
	}
	if n := strings.Count(csp, "http"); n != 0 {
		t.Errorf("policy allows network sources: %s", csp)
	}

	// Nothing may be loaded: no src attributes, no CSS imports, url()s only
	// for the embedded fonts, and links only to in-page anchors, the
	// embedded icon and the rule documentation.
	for _, bad := range []string{" src=", "@import", "http-equiv=\"refresh\""} {
		if strings.Contains(out, bad) {
			t.Errorf("page loads or redirects: found %q", bad)
		}
	}
	if n, fonts := strings.Count(out, "url("), strings.Count(out, "url(data:font/woff2;base64,"); n != fonts || fonts != len(fontFaces) {
		t.Errorf("%d url() references, %d of them embedded fonts; want exactly the %d fonts", n, fonts, len(fontFaces))
	}
	for _, h := range regexp.MustCompile(`href="([^"]*)"`).FindAllStringSubmatch(out, -1) {
		link := html.UnescapeString(h[1])
		if !strings.HasPrefix(link, "#") && !strings.HasPrefix(link, rulesDocURL) && link != htmlFavicon {
			t.Errorf("unexpected link %q", link)
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
			[]string{`<p class="v-text">1 finding at or above high</p>`, `class="verdict v-fail"`, "#i-fail", "2 suppressed by baseline", "exit code 1", `aria-label="1 critical, 1 medium"`}},
		{"below threshold", onlyMedium, Meta{FailOn: finding.High},
			[]string{`<p class="v-text">No findings at or above high</p>`, `class="verdict v-pass"`, "#i-pass", "exit code 0"}},
		{"threshold disabled", sample(), Meta{NoFail: true},
			[]string{`<p class="v-text">2 findings, failure threshold disabled</p>`, "threshold none", `class="verdict v-pass"`}},
		{"empty", empty, Meta{FailOn: finding.High},
			[]string{`<p class="v-text">No findings</p>`, "nothing matched a rule", "No agent context files were found.", `class="dist clean"`, `aria-label="No findings"`}},
		{"incomplete", incomplete, Meta{FailOn: finding.High},
			[]string{`<p class="v-text">Some files could not be analysed</p>`, `class="verdict v-incomplete"`, "#i-warn", `role="alert"`, "CLAUDE.md: permission denied", "exit code 2"}},
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
	if !strings.Contains(out, `<span class="kind">Claude memory file</span><span class="tally"><span class="sev-critical">1 critical</span></span>`) {
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

func TestDistribution(t *testing.T) {
	segs := distribution([]htmlCount{{Key: "critical", N: 1}, {Key: "high"}, {Key: "medium", N: 500}, {Key: "low", N: 3}, {Key: "info"}})
	if len(segs) != 3 || segs[0].Key != "critical" || segs[1].Key != "medium" || segs[2].Key != "low" {
		t.Fatalf("segments = %+v", segs)
	}
	if segs[0].W < 10 || segs[2].W < 10 {
		t.Errorf("small counts must stay visible: %+v", segs)
	}
	last := segs[len(segs)-1]
	if segs[0].X != 0 || last.X+last.W != 1000 {
		t.Errorf("segments must fill the bar exactly: %+v", segs)
	}
	for i := 1; i < len(segs); i++ {
		if segs[i].X <= segs[i-1].X+segs[i-1].W {
			t.Errorf("segments overlap: %+v", segs)
		}
	}
	if got := distribution([]htmlCount{{Key: "critical"}, {Key: "info"}}); len(got) != 1 || got[0].W != 1000 {
		t.Errorf("no findings = %+v", got)
	}
}

func TestFormatInt(t *testing.T) {
	for n, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 1778: "1,778", 1234567: "1,234,567", -1234: "-1,234", -999: "-999"} {
		if got := formatInt(n); got != want {
			t.Errorf("formatInt(%d) = %q, want %q", n, got, want)
		}
	}
}

// The report's logo and icon must stay identical to the published brand
// assets in docs/assets/brand.
func TestHTMLBrandMatchesAssets(t *testing.T) {
	mark, err := os.ReadFile("../../docs/assets/brand/portcullis-mark-small.svg")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mark), `d="`+markPath+`"`) {
		t.Error("markPath differs from docs/assets/brand/portcullis-mark-small.svg")
	}
	icon, err := os.ReadFile("../../docs/assets/brand/favicon.svg")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(icon, mustRead("html/favicon.svg")) {
		t.Error("html/favicon.svg differs from docs/assets/brand/favicon.svg")
	}
	if n := strings.Count(htmlCSS, "@font-face"); n != len(fontFaces) {
		t.Errorf("%d @font-face rules, want %d", n, len(fontFaces))
	}
}

// The embedded fonts are third-party binaries; they must be the files whose
// checksums html/fonts/README.md publishes.
func TestEmbeddedFontsMatchPublishedChecksums(t *testing.T) {
	readme, err := os.ReadFile("html/fonts/README.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fontFaces {
		sum := sha256.Sum256(mustRead("html/fonts/" + f.file))
		want := fmt.Sprintf("`%x`", sum)
		found := false
		for _, line := range strings.Split(string(readme), "\n") {
			if strings.Contains(line, "`"+f.file+"`") && strings.Contains(line, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: SHA-256 %x is not the one listed in html/fonts/README.md", f.file, sum)
		}
	}
}
