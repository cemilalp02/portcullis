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
	"strconv"
	"strings"

	"github.com/cemilalp02/portcullis/internal/engine"
	"github.com/cemilalp02/portcullis/internal/finding"
)

// Self-contained HTML report: one file, no network access. Text from scanned
// files reaches the page only through html/template's contextual escaping,
// and a Content Security Policy that allows nothing but the report's own
// style and script (by hash) and its embedded fonts and icon backs that up.

//go:embed html/report.tmpl html/style.css html/app.js html/favicon.svg html/fonts/*.woff2
var htmlFS embed.FS

// fontFaces are IBM's own Latin-1 subsets of IBM Plex (html/fonts/README.md).
// Other characters fall back to the system fonts named in style.css.
var fontFaces = []struct {
	family, file string
	weight       int
}{
	{"IBM Plex Sans", "IBMPlexSans-Regular-Latin1.woff2", 400},
	{"IBM Plex Sans", "IBMPlexSans-SemiBold-Latin1.woff2", 600},
	{"IBM Plex Mono", "IBMPlexMono-Regular-Latin1.woff2", 400},
	{"IBM Plex Mono", "IBMPlexMono-Medium-Latin1.woff2", 500},
}

const latin1Range = "U+0000,U+000D,U+0020-007E,U+00A0-00FF,U+0131,U+0152-0153,U+02C6,U+02DA,U+02DC," +
	"U+2013-2014,U+2018-201A,U+201C-201E,U+2020-2022,U+2026,U+2030,U+2039-203A,U+2044,U+20AC,U+2122,U+2212,U+FB01-FB02"

// markPath is the small optical size of the Portcullis mark on a 64-unit
// grid, as in docs/assets/brand/portcullis-mark-small.svg.
const markPath = "M3.5 62L3.5 37C3.5 28.78 6.33 20.81 11.53 14.43C16.72 8.06 23.95 3.67 32 2C40.05 3.67 47.28 8.06 52.47 14.43" +
	"C57.67 20.81 60.5 28.78 60.5 37L60.5 62L52 62L52 37C52 31.04 50.04 25.24 46.43 20.5C42.82 15.75 37.75 12.32 32 10.74" +
	"C26.25 12.32 21.18 15.75 17.57 20.5C13.96 25.24 12 31.04 12 37L12 62Z" +
	"M15 35.1L15.07 35.1C15.43 30.61 17.03 26.29 19.71 22.65C22.76 18.48 27.07 15.41 32 13.87C36.93 15.41 41.24 18.48 44.29 22.65" +
	"C46.97 26.29 48.57 30.61 48.93 35.1L49 35.1L49 40.46L46.7 44.6L44.4 40.46L44.4 35.6L39.07 35.6L39.07 40.64L36.87 44.6" +
	"L34.67 40.64L34.67 35.6L29.33 35.6L29.33 40.64L27.13 44.6L24.93 40.64L24.93 35.6L19.6 35.6L19.6 40.46L17.3 44.6L15 40.46Z" +
	"M29.33 20.04L29.33 31L34.67 31L34.67 20.04C33.82 19.54 32.92 19.11 32 18.74C31.08 19.11 30.18 19.54 29.33 20.04Z" +
	"M39.07 23.54L39.07 31L43.46 31C42.89 29.22 42.07 27.53 41.01 25.96C40.42 25.1 39.77 24.3 39.07 23.54Z" +
	"M24.93 23.54C24.23 24.3 23.58 25.1 22.99 25.96C21.93 27.53 21.11 29.22 20.54 31L24.93 31Z"

var (
	htmlTmpl = template.Must(template.New("report.tmpl").Funcs(template.FuncMap{
		"num": formatInt, "plural": plural,
	}).ParseFS(htmlFS, "html/report.tmpl"))
	htmlCSS     = fontCSS() + string(mustRead("html/style.css"))
	htmlJS      = string(mustRead("html/app.js"))
	htmlFavicon = "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(mustRead("html/favicon.svg"))
	htmlCSP     = fmt.Sprintf("default-src 'none'; style-src '%s'; script-src '%s'; font-src data:; img-src data:; base-uri 'none'; form-action 'none'",
		cspHash(htmlCSS), cspHash(htmlJS))
)

func mustRead(name string) []byte {
	b, err := htmlFS.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return b
}

func fontCSS() string {
	var b strings.Builder
	b.WriteString("/* IBM Plex: Copyright IBM Corp., SIL Open Font License 1.1, Reserved Font Name \"Plex\". */\n")
	for _, f := range fontFaces {
		fmt.Fprintf(&b, "@font-face{font-family:%q;font-style:normal;font-weight:%d;font-display:swap;"+
			"src:url(data:font/woff2;base64,%s) format(\"woff2\");unicode-range:%s}\n",
			f.family, f.weight, base64.StdEncoding.EncodeToString(mustRead("html/fonts/"+f.file)), latin1Range)
	}
	return b.String()
}

func cspHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return "sha256-" + base64.StdEncoding.EncodeToString(sum[:])
}

type htmlCount struct {
	Key, Label string
	N          int
}

// htmlSeg is one segment of the severity bar, in units of a 1000-wide box.
type htmlSeg struct {
	Key  string
	X, W int
}

type htmlFinding struct {
	finding.Finding
	SevKey, DocURL, Text string
	Above                bool
	Spans                []htmlSpan
}

// htmlSpan is a piece of an excerpt; Hidden marks characters an editor would
// not show, More counts those left out of a long run.
type htmlSpan struct {
	Text, Title string
	Hidden      bool
	More        int
}

type htmlTally struct {
	Key string
	N   int
}

type htmlGroup struct {
	Path, Kind string
	Tally      []htmlTally
	Findings   []htmlFinding
}

type htmlTarget struct {
	Path, Kind string
	Findings   int
	Severities []string
}

type htmlRule struct {
	ID, Title, SevKey, DocURL string
	Count                     int
}

type htmlData struct {
	Name, Version, Ruleset, Mode                string
	Headline, Status, StatusLabel, StatusIcon   string
	Threshold, DistLabel                        string
	Targets, Files, Total, Suppressed, ExitCode int
	Counts                                      []htmlCount
	Dist                                        []htmlSeg
	Groups                                      []htmlGroup
	Inventory                                   []engine.InventoryItem
	TargetFiles                                 []htmlTarget
	FoldFiles                                   bool
	Rules                                       []htmlRule
	Errors                                      []string
	CSP, MarkPath                               string
	Favicon                                     template.URL
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
		CSP: htmlCSP, MarkPath: markPath,
		// Embedded, trusted assets; nothing from the scanned repository.
		Favicon: template.URL(htmlFavicon), CSS: template.CSS(htmlCSS), JS: template.JS(htmlJS), //nolint:gosec // see above
	}
	if d.Name == "." || d.Name == string(filepath.Separator) || d.Name == "" {
		d.Name = "repository"
	}
	d.Threshold = m.FailOn.String()
	if m.NoFail {
		d.Threshold = "none"
	}
	switch {
	case !res.Complete:
		d.Headline, d.Status, d.StatusLabel, d.StatusIcon = "Some files could not be analysed", "incomplete", "Incomplete", "i-warn"
	case above > 0:
		d.Headline, d.Status, d.StatusLabel, d.StatusIcon = fmt.Sprintf("%d %s at or above %s", above, plural(above, "finding", "findings"), d.Threshold), "fail", "Fail", "i-fail"
	case len(res.Findings) == 0:
		d.Headline, d.Status, d.StatusLabel, d.StatusIcon = "No findings", "pass", "Pass", "i-pass"
	case m.NoFail:
		n := len(res.Findings)
		d.Headline, d.Status, d.StatusLabel, d.StatusIcon = fmt.Sprintf("%d %s, failure threshold disabled", n, plural(n, "finding", "findings")), "pass", "Pass", "i-pass"
	default:
		d.Headline, d.Status, d.StatusLabel, d.StatusIcon = fmt.Sprintf("No findings at or above %s", d.Threshold), "pass", "Pass", "i-pass"
	}

	counts := map[finding.Severity]int{}
	perFile := map[string][]htmlFinding{}
	var order []string
	rules := map[string]*htmlRule{}
	for _, f := range res.Findings {
		counts[f.Severity]++
		key := f.Severity.String()
		hf := htmlFinding{
			Finding: f, SevKey: key, DocURL: docURL(f.RuleID),
			Above: !m.NoFail && f.Severity >= m.FailOn, Spans: excerptSpans(f.Excerpt),
			Text: strings.ToLower(strings.Join([]string{f.RuleID, f.Title, f.Path, f.Message, f.Excerpt}, " ")),
		}
		if _, ok := perFile[f.Path]; !ok {
			order = append(order, f.Path)
		}
		perFile[f.Path] = append(perFile[f.Path], hf)
		r, ok := rules[f.RuleID]
		if !ok {
			r = &htmlRule{ID: f.RuleID, Title: f.Title, SevKey: key, DocURL: docURL(f.RuleID)}
			rules[f.RuleID] = r
		}
		r.Count++
	}
	var label []string
	for s := finding.Critical; s >= finding.Info; s-- {
		key := s.String()
		d.Counts = append(d.Counts, htmlCount{Key: key, Label: strings.ToUpper(key[:1]) + key[1:], N: counts[s]})
		if counts[s] > 0 {
			label = append(label, fmt.Sprintf("%d %s", counts[s], key))
		}
	}
	d.Dist = distribution(d.Counts)
	d.DistLabel = "No findings"
	if len(label) > 0 {
		d.DistLabel = strings.Join(label, ", ")
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

// distribution lays out the severity bar: one segment per severity that has
// findings, proportional to its count but never thinner than minW so a
// single critical finding stays visible next to hundreds of others.
func distribution(counts []htmlCount) []htmlSeg {
	const width, gap, minW = 1000, 4, 10
	var nz []htmlCount
	total := 0
	for _, c := range counts {
		if c.N > 0 {
			nz = append(nz, c)
			total += c.N
		}
	}
	if total == 0 {
		return []htmlSeg{{Key: "none", W: width}}
	}
	avail := width - gap*(len(nz)-1)
	ws := make([]int, len(nz))
	sum, big := 0, 0
	for i, c := range nz {
		ws[i] = max(avail*c.N/total, minW)
		sum += ws[i]
		if ws[i] > ws[big] {
			big = i
		}
	}
	ws[big] += avail - sum
	segs := make([]htmlSeg, 0, len(nz))
	x := 0
	for i, c := range nz {
		segs = append(segs, htmlSeg{Key: c.Key, X: x, W: ws[i]})
		x += ws[i] + gap
	}
	return segs
}

// formatInt writes n with thousands separators: 1778 becomes "1,778".
func formatInt(n int) string {
	s := strconv.Itoa(n)
	start := 0
	if n < 0 {
		start = 1
	}
	for i := len(s) - 3; i > start; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

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

func tally(fs []htmlFinding) []htmlTally {
	counts := map[string]int{}
	for _, f := range fs {
		counts[f.SevKey]++
	}
	var out []htmlTally
	for s := finding.Critical; s >= finding.Info; s-- {
		if n := counts[s.String()]; n > 0 {
			out = append(out, htmlTally{Key: s.String(), N: n})
		}
	}
	return out
}
