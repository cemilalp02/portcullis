package report

import (
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cemilalp02/portcullis/internal/engine"
	"github.com/cemilalp02/portcullis/internal/finding"
)

// Self-contained HTML report: one file, no network access. Text from scanned
// files reaches the page only through html/template's contextual escaping,
// and a Content Security Policy that allows nothing but the report's own
// style and script (by hash) backs that up.

//go:embed html/report.tmpl html/style.css html/app.js
var htmlFS embed.FS

var (
	htmlTmpl = template.Must(template.ParseFS(htmlFS, "html/report.tmpl"))
	htmlCSS  = mustRead("html/style.css")
	htmlJS   = mustRead("html/app.js")
	htmlCSP  = fmt.Sprintf("default-src 'none'; style-src '%s'; script-src '%s'; img-src 'none'; base-uri 'none'; form-action 'none'",
		cspHash(htmlCSS), cspHash(htmlJS))
)

func mustRead(name string) string {
	b, err := htmlFS.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func cspHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return "sha256-" + base64.StdEncoding.EncodeToString(sum[:])
}

type htmlCount struct {
	Key, Label string
	N          int
}

type htmlFinding struct {
	finding.Finding
	SevKey, SevLabel, DocURL, Text string
	Above                          bool
	Spans                          []htmlSpan
}

// htmlSpan is a piece of an excerpt; Hidden marks characters an editor would
// not show, More counts those left out of a long run.
type htmlSpan struct {
	Text, Title string
	Hidden      bool
	More        int
}

type htmlGroup struct {
	Path, Kind, Tally string
	Findings          []htmlFinding
}

type htmlTarget struct {
	Path, Kind string
	Findings   int
	Severities []string
}

type htmlRule struct {
	ID, Title, SevKey, SevLabel, DocURL string
	Count                               int
}

type htmlData struct {
	Name, Version, Ruleset, Mode                string
	Headline, Status, StatusLabel               string
	ThresholdHint                               string
	Targets, Files, Total, Suppressed, ExitCode int
	Counts                                      []htmlCount
	Groups                                      []htmlGroup
	Inventory                                   []engine.InventoryItem
	TargetFiles                                 []htmlTarget
	FoldFiles                                   bool
	Rules                                       []htmlRule
	Errors                                      []string
	CSP                                         string
	CSS                                         template.CSS
	JS                                          template.JS
}

// WriteHTML writes a self-contained HTML report.
func WriteHTML(w io.Writer, res *engine.Result, m Meta) error {
	above := Above(res, m)
	d := htmlData{
		Name: filepath.Base(res.Root), Version: m.Version, Ruleset: m.Ruleset, Mode: m.Mode,
		Targets: res.Targets, Files: res.Files, Total: len(res.Findings), Suppressed: m.Suppressed,
		ExitCode: ExitCode(res, m), Inventory: res.Inventory, Errors: res.Errors,
		CSP: htmlCSP, CSS: template.CSS(htmlCSS), JS: template.JS(htmlJS), //nolint:gosec // embedded, trusted assets
	}
	if d.Name == "." || d.Name == string(filepath.Separator) || d.Name == "" {
		d.Name = "repository"
	}
	threshold := m.FailOn.String()
	if m.NoFail {
		threshold = "none"
	}
	d.ThresholdHint = "threshold: " + threshold
	switch {
	case !res.Complete:
		d.Headline, d.Status, d.StatusLabel = "Analysis incomplete", "incomplete", "Incomplete"
	case above > 0:
		d.Headline, d.Status, d.StatusLabel = fmt.Sprintf("%d %s at or above %s", above, plural(above, "finding", "findings"), threshold), "fail", "Fails"
	case len(res.Findings) == 0:
		d.Headline, d.Status, d.StatusLabel = "No findings", "pass", "Passes"
	case m.NoFail:
		n := len(res.Findings)
		d.Headline, d.Status, d.StatusLabel = fmt.Sprintf("%d %s, failure threshold disabled", n, plural(n, "finding", "findings")), "pass", "Passes"
	default:
		d.Headline, d.Status, d.StatusLabel = fmt.Sprintf("No findings at or above %s", threshold), "pass", "Passes"
	}

	counts := map[finding.Severity]int{}
	perFile := map[string][]htmlFinding{}
	var order []string
	rules := map[string]*htmlRule{}
	for _, f := range res.Findings {
		counts[f.Severity]++
		key := f.Severity.String()
		hf := htmlFinding{
			Finding: f, SevKey: key, SevLabel: key, DocURL: docURL(f.RuleID),
			Above: !m.NoFail && f.Severity >= m.FailOn, Spans: excerptSpans(f.Excerpt),
			Text: strings.ToLower(strings.Join([]string{f.RuleID, f.Title, f.Path, f.Message, f.Excerpt}, " ")),
		}
		if _, ok := perFile[f.Path]; !ok {
			order = append(order, f.Path)
		}
		perFile[f.Path] = append(perFile[f.Path], hf)
		r, ok := rules[f.RuleID]
		if !ok {
			r = &htmlRule{ID: f.RuleID, Title: f.Title, SevKey: key, SevLabel: key, DocURL: docURL(f.RuleID)}
			rules[f.RuleID] = r
		}
		r.Count++
	}
	for s := finding.Critical; s >= finding.Info; s-- {
		d.Counts = append(d.Counts, htmlCount{Key: s.String(), Label: s.String(), N: counts[s]})
	}

	kinds := map[string]string{}
	for _, tf := range res.TargetFiles {
		kinds[tf.Path] = tf.Kind
		t := htmlTarget{Path: tf.Path, Kind: tf.Kind}
		for _, hf := range perFile[tf.Path] {
			t.Findings++
			if !slices.Contains(t.Severities, hf.SevKey) {
				t.Severities = append(t.Severities, hf.SevKey)
			}
		}
		d.TargetFiles = append(d.TargetFiles, t)
	}
	d.FoldFiles = len(d.TargetFiles) > foldFilesOver
	// Findings are sorted by severity, so files appear worst first.
	for _, p := range order {
		fs := perFile[p]
		d.Groups = append(d.Groups, htmlGroup{Path: p, Kind: kinds[p], Tally: tally(fs), Findings: fs})
	}
	for _, r := range rules {
		d.Rules = append(d.Rules, *r)
	}
	slices.SortFunc(d.Rules, func(a, b htmlRule) int {
		sa, _ := finding.ParseSeverity(a.SevKey)
		sb, _ := finding.ParseSeverity(b.SevKey)
		if sa != sb {
			return int(sb - sa)
		}
		return strings.Compare(a.ID, b.ID)
	})
	return htmlTmpl.Execute(w, d)
}

// foldFilesOver is the size above which the file list starts collapsed.
const foldFilesOver = 25

// hiddenRunRE matches a run of characters that escape.String rendered as
// <U+XXXX> or <0xNN>; hiddenOneRE matches one of them.
var (
	hiddenRunRE = regexp.MustCompile(`(?:<U\+[0-9A-F]{4,6}>|<0x[0-9A-F]{2}>)+`)
	hiddenOneRE = regexp.MustCompile(`<U\+[0-9A-F]{4,6}>|<0x[0-9A-F]{2}>`)
)

// hiddenShown is how many escapes of a long run are printed before the rest
// is summarised as "+N more".
const hiddenShown = 4

// excerptSpans splits an escaped excerpt so the report can highlight the
// characters a reader would not see in an editor. Runs longer than
// hiddenShown+2 are shortened; the finding message carries the decoded text.
func excerptSpans(s string) []htmlSpan {
	var out []htmlSpan
	last := 0
	for _, loc := range hiddenRunRE.FindAllStringIndex(s, -1) {
		if loc[0] > last {
			out = append(out, htmlSpan{Text: s[last:loc[0]]})
		}
		parts := hiddenOneRE.FindAllString(s[loc[0]:loc[1]], -1)
		sp := htmlSpan{Text: s[loc[0]:loc[1]], Hidden: true,
			Title: fmt.Sprintf("%d invisible %s", len(parts), plural(len(parts), "character", "characters"))}
		if len(parts) > hiddenShown+2 {
			sp.Text, sp.More = strings.Join(parts[:hiddenShown], ""), len(parts)-hiddenShown
		}
		out = append(out, sp)
		last = loc[1]
	}
	if last < len(s) {
		out = append(out, htmlSpan{Text: s[last:]})
	}
	return out
}

func docURL(id string) string {
	if strings.HasPrefix(id, "PCL-") {
		return rulesDocURL + strings.ToLower(id)
	}
	return ""
}

func tally(fs []htmlFinding) string {
	counts := map[string]int{}
	for _, f := range fs {
		counts[f.SevKey]++
	}
	var parts []string
	for s := finding.Critical; s >= finding.Info; s-- {
		if n := counts[s.String()]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, s))
		}
	}
	return strings.Join(parts, " · ")
}
