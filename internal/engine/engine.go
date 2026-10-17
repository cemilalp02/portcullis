// Package engine runs a scan: discovery, decoding, parsing, extraction and
// rule evaluation, producing findings and the auto-execution inventory
// (ARCHITECTURE §2, §5.6-§5.8).
package engine

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/cemilalp02/portcullis/internal/discover"
	"github.com/cemilalp02/portcullis/internal/escape"
	"github.com/cemilalp02/portcullis/internal/extract"
	"github.com/cemilalp02/portcullis/internal/finding"
	"github.com/cemilalp02/portcullis/internal/jsonc"
	"github.com/cemilalp02/portcullis/internal/ruleset"
	"github.com/cemilalp02/portcullis/internal/source"
	"github.com/cemilalp02/portcullis/internal/target"
	"github.com/cemilalp02/portcullis/internal/textview"
	"github.com/cemilalp02/portcullis/rules"
)

// Limits on reported findings (FR-14).
const (
	MaxPerRulePerFile = 5
	MaxPerFile        = 50
	excerptLimit      = 200
	valueLimit        = 80
)

// Engine rule IDs (rules/eng).
const (
	RuleOversize       = "PCL-ENG-001"
	RuleUnparseable    = "PCL-ENG-002"
	RuleDuplicateKey   = "PCL-ENG-003"
	RuleEncoding       = "PCL-ENG-004"
	RuleLinkEscape     = "PCL-ENG-005"
	RuleReadError      = "PCL-ENG-006"
	RuleInTreeConfig   = "PCL-ENG-007"
	RuleSkippedTargets = "PCL-ENG-008"
	RuleProtected      = "PCL-ENG-009"
	RuleFindingCap     = "PCL-ENG-010"
	RuleDirLinkEscape  = "PCL-ENG-011"
	RuleUnresolvedLink = "PCL-ENG-012"
	RuleStaleBaseline  = "PCL-ENG-013"
	RuleDuplicateInert = "PCL-ENG-014"
	RuleEncrypted      = "PCL-ENG-015"
)

var engineRules = []string{
	RuleOversize, RuleUnparseable, RuleDuplicateKey, RuleEncoding, RuleLinkEscape, RuleReadError,
	RuleInTreeConfig, RuleSkippedTargets, RuleProtected, RuleFindingCap, RuleDirLinkEscape, RuleUnresolvedLink,
	RuleStaleBaseline, RuleDuplicateInert, RuleEncrypted,
}

const builtinEngine = "engine"

// builtins are named Go checks that rules reference with `builtin: <name>`.
// The "engine" builtin is special: those rules are emitted by the engine
// itself (discovery events, decoding and parsing problems).
var builtins = map[string]func(c *collector, r *ruleset.Compiled){
	"skill_capability_mismatch": skillCapabilityMismatch,
	"credential_destination":    credentialDestination,
}

// Builtins lists the built-in check names rules may reference.
func Builtins() []string {
	names := []string{builtinEngine}
	for n := range builtins {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}

// Engine evaluates a rule set against target files. It is safe for
// concurrent use.
type Engine struct {
	Targets *target.Set
	Rules   *ruleset.Set
	rules   []*ruleset.Compiled
}

// New checks that the rule and target sets fit together.
func New(ts *target.Set, rs *ruleset.Set) (*Engine, error) {
	var errs []error
	for _, id := range engineRules {
		if r := rs.Get(id); r == nil || r.Match.Builtin != builtinEngine {
			errs = append(errs, fmt.Errorf("engine rule %s is missing or not 'builtin: engine'", id))
		}
	}
	for _, d := range ts.Defs() {
		if d.Family == target.Config {
			if _, ok := extract.Lookup(d.Extractor); !ok {
				errs = append(errs, fmt.Errorf("target %s: unknown extractor %q", d.ID, d.Extractor))
			}
		}
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return &Engine{Targets: ts, Rules: rs, rules: rs.Rules()}, nil
}

// Default builds an engine from the embedded targets and rules.
func Default() (*Engine, error) { return Load() }

// Load builds an engine from the embedded targets and rules plus the custom
// rules in ruleDirs (each a directory of <ID>.yaml files, optionally one
// level of sub-directories; IDs must not use the PCL- prefix).
func Load(ruleDirs ...string) (*Engine, error) {
	data, err := rules.FS.ReadFile("targets.yaml")
	if err != nil {
		return nil, err
	}
	ts, err := target.Load(data)
	if err != nil {
		return nil, err
	}
	opts := ruleset.Options{Targets: ts, Builtins: Builtins()}
	rs, err := ruleset.Load(rules.FS, opts)
	if err != nil {
		return nil, err
	}
	opts.Custom = true
	for _, dir := range ruleDirs {
		fi, err := os.Stat(dir)
		if err != nil {
			return nil, fmt.Errorf("rules directory: %w", err)
		}
		if !fi.IsDir() {
			return nil, fmt.Errorf("rules directory %s is not a directory", dir)
		}
		extra, err := ruleset.Load(os.DirFS(dir), opts)
		if err != nil {
			return nil, fmt.Errorf("rules directory %s: %w", dir, err)
		}
		if err := rs.Merge(extra); err != nil {
			return nil, fmt.Errorf("rules directory %s: %w", dir, err)
		}
	}
	return New(ts, rs)
}

// Options controls a scan.
type Options struct {
	MaxFileSize int64    // 0 means discover.DefaultMaxFileSize
	SkipDirs    []string // nil means discover.DefaultSkipDirs
	Workers     int      // 0 means GOMAXPROCS
	// Disabled rule IDs are dropped from the result; Severity overrides the
	// severity of the given rule IDs (from a trusted configuration).
	Disabled map[string]bool
	Severity map[string]finding.Severity
}

// TargetFile is an agent context file found by a scan, whether or not it
// could be analysed (oversize files are included, so context.lock covers
// every target).
type TargetFile struct {
	Path     string // as found
	ReadPath string // where to read it, relative to the root
	Kind     string // target type title, e.g. "Claude Code settings"
}

// InventoryItem is one command that would run automatically. All strings
// taken from the scanned file are escaped.
type InventoryItem struct {
	Kind    extract.Kind `json:"kind"`
	Trigger string       `json:"trigger"`
	Name    string       `json:"name"`
	Path    string       `json:"path"`
	Line    int          `json:"line"`
	Command string       `json:"command"`
}

// Result is the outcome of a scan.
type Result struct {
	Root      string
	Files     int // files seen
	Targets   int // agent context files analysed
	Findings  []finding.Finding
	Inventory []InventoryItem
	Errors    []string // what could not be analysed
	Complete  bool     // false means exit code 2 (fail closed)
	// TargetFiles lists every agent context file, sorted by path.
	TargetFiles []TargetFile
}

// Scan scans the directory root.
func (e *Engine) Scan(root string, opts Options) (*Result, error) {
	maxSize := cmp.Or(opts.MaxFileSize, discover.DefaultMaxFileSize)
	dr, err := discover.Walk(root, e.Targets, discover.Options{MaxFileSize: maxSize, SkipDirs: opts.SkipDirs})
	if err != nil {
		return nil, err
	}
	res := &Result{Root: dr.Root, Files: dr.Files, Targets: len(dr.Candidates), Complete: true}
	for _, c := range dr.Candidates {
		res.TargetFiles = append(res.TargetFiles, TargetFile{Path: c.Path, ReadPath: c.ReadPath, Kind: c.Target.Title})
	}
	for _, ev := range dr.Events {
		res.Findings = append(res.Findings, e.eventFinding(ev))
		switch ev.Kind {
		case discover.ReadError:
			res.fail(ev.Path, ev.Detail)
		case discover.Oversize:
			kind := "agent context file"
			if d := e.Targets.Match(ev.Path); d != nil {
				kind = d.Title
			}
			res.TargetFiles = append(res.TargetFiles, TargetFile{Path: ev.Path, ReadPath: ev.ReadPath, Kind: kind})
		}
	}

	r, err := os.OpenRoot(dr.Root)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	workers := cmp.Or(opts.Workers, runtime.GOMAXPROCS(0))
	budget := newByteBudget(analysisBudget)
	jobs := make(chan discover.Candidate)
	results := make(chan fileResult)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range jobs {
				n := min(max(c.Size, 1), analysisBudget)
				budget.acquire(n)
				fr := e.scanCandidate(r, c, maxSize)
				budget.release(n)
				results <- fr
			}
		}()
	}
	go func() {
		for _, c := range dr.Candidates {
			jobs <- c
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()
	var scripts [][]string
	for fr := range results {
		res.Findings = append(res.Findings, fr.findings...)
		res.Inventory = append(res.Inventory, fr.inventory...)
		scripts = append(scripts, fr.scripts...)
		if fr.err != "" {
			res.fail(fr.path, fr.err)
		}
	}
	e.runScripts(r, res, scripts, maxSize)
	slices.SortFunc(res.TargetFiles, func(a, b TargetFile) int { return strings.Compare(a.Path, b.Path) })

	if len(opts.Disabled) > 0 || len(opts.Severity) > 0 {
		kept := res.Findings[:0]
		for _, f := range res.Findings {
			if opts.Disabled[f.RuleID] {
				continue
			}
			if s, ok := opts.Severity[f.RuleID]; ok {
				f.Severity = s
			}
			kept = append(kept, f)
		}
		res.Findings = kept
	}
	finding.Sort(res.Findings)
	slices.SortFunc(res.Inventory, func(a, b InventoryItem) int {
		return cmp.Or(strings.Compare(a.Path, b.Path), cmp.Compare(a.Line, b.Line), strings.Compare(a.Command, b.Command))
	})
	slices.Sort(res.Errors)
	return res, nil
}

func (res *Result) fail(path, detail string) {
	res.Complete = false
	res.Errors = append(res.Errors, escape.String(path, 0)+": "+escape.String(detail, excerptLimit))
}

// analysisBudget bounds the bytes of the files being analysed at once.
// Analysis holds about 25 bytes per input byte (decoded text, the normalised
// and prose views with their offset maps), so without a bound a repository of
// many large files needs workers x 25 x file size: 64 files of 10 MB took
// 9.5 GB with 32 workers. A file larger than the budget is analysed alone.
const analysisBudget = 32 << 20

// byteBudget is a counting semaphore over bytes.
type byteBudget struct {
	mu   sync.Mutex
	cond *sync.Cond
	free int64
}

func newByteBudget(n int64) *byteBudget {
	b := &byteBudget{free: n}
	b.cond = sync.NewCond(&b.mu)
	return b
}

func (b *byteBudget) acquire(n int64) {
	b.mu.Lock()
	for b.free < n {
		b.cond.Wait()
	}
	b.free -= n
	b.mu.Unlock()
}

func (b *byteBudget) release(n int64) {
	b.mu.Lock()
	b.free += n
	b.mu.Unlock()
	b.cond.Broadcast()
}

type fileResult struct {
	path      string
	findings  []finding.Finding
	inventory []InventoryItem
	scripts   [][]string // scripts its commands run, see commandScripts
	err       string
}

// scanCandidate reads a candidate through os.Root, so that even a path
// manipulated between discovery and reading cannot escape the scan root.
func (e *Engine) scanCandidate(r *os.Root, c discover.Candidate, maxSize int64) fileResult {
	fail := func(err error) fileResult {
		f := e.fileFinding(RuleReadError, c.Path, map[string]string{"name": err.Error()})
		return fileResult{path: c.Path, findings: []finding.Finding{f}, err: err.Error()}
	}
	f, err := r.Open(filepath.FromSlash(c.ReadPath))
	if err != nil {
		return fail(err)
	}
	data, err := io.ReadAll(io.LimitReader(f, maxSize+1))
	_ = f.Close()
	if err != nil {
		return fail(err)
	}
	if int64(len(data)) > maxSize { // grew after discovery
		f := e.fileFinding(RuleOversize, c.Path, map[string]string{"name": fmt.Sprintf("more than %d bytes", maxSize)})
		return fileResult{path: c.Path, findings: []finding.Finding{f}}
	}
	findings, inv, points := e.analyze(c.Path, c.Target, data)
	return fileResult{path: c.Path, findings: findings, inventory: inv, scripts: commandScripts(e.Targets, c.Path, points)}
}

// Analyze evaluates every applicable rule against one file's content. path is
// the '/'-separated path reported in findings; def is its target type.
func (e *Engine) Analyze(path string, def *target.Def, raw []byte) ([]finding.Finding, []InventoryItem) {
	findings, inv, _ := e.analyze(path, def, raw)
	return findings, inv
}

func (e *Engine) analyze(path string, def *target.Def, raw []byte) ([]finding.Finding, []InventoryItem, []extract.ExecPoint) {
	doc := source.NewDocument(path, raw)
	col := &collector{e: e, doc: doc, perRule: map[string]int{}, seen: map[string]int{}}

	if encrypted(raw) {
		col.add(e.Rules.Get(RuleEncrypted), 0, 0, "git-crypt", map[string]string{"name": "git-crypt"})
		col.flush()
		return col.findings, nil, nil
	}

	if doc.Encoding.Unusual() {
		col.add(e.Rules.Get(RuleEncoding), 0, 0, string(doc.Encoding), map[string]string{"name": string(doc.Encoding)})
	}

	var points []extract.ExecPoint
	if def.Format == "jsonc" {
		root, dups, err := jsonc.Parse(doc.Text)
		for _, d := range dups {
			id := RuleDuplicateKey
			if root != nil && !executes(def.Extractor, d.Key, keyPath(root, d.Offset)) {
				id = RuleDuplicateInert
			}
			col.add(e.Rules.Get(id), d.Offset, d.Offset+len(d.Key)+2, d.Key, map[string]string{"name": d.Key})
		}
		var se *jsonc.SyntaxError
		switch {
		case errors.As(err, &se):
			col.add(e.Rules.Get(RuleUnparseable), se.Offset, se.Offset, se.Msg, map[string]string{"name": se.Msg})
		case err == nil:
			if fn, ok := extract.Lookup(def.Extractor); ok {
				points = fn(root)
			}
		}
	}

	for _, r := range e.rules {
		if r.Match.Builtin == builtinEngine || !r.AppliesTo(def) {
			continue
		}
		switch {
		case r.Match.Builtin != "":
			builtins[r.Match.Builtin](col, r)
		case len(r.Ranges) > 0:
			col.codepoints(r)
		case r.Match.Scope == ruleset.ScopeExec:
			col.exec(r, points)
		case r.Match.View == ruleset.ViewRaw:
			col.text(r, doc.Text, nil)
		case r.Match.View == ruleset.ViewProse:
			v := col.prose()
			col.text(r, v.Text, v)
		default:
			v := col.normalized()
			col.text(r, v.Text, v)
		}
	}
	col.flush()

	inv := make([]InventoryItem, 0, len(points))
	for _, p := range points {
		line, _ := doc.Lines.Position(p.Start)
		inv = append(inv, InventoryItem{
			Kind:    p.Kind,
			Trigger: escape.String(p.Trigger, valueLimit),
			Name:    escape.String(p.Name, valueLimit),
			Path:    escape.String(path, 0),
			Line:    line,
			Command: escape.String(p.Command, excerptLimit),
		})
	}
	return col.findings, inv, points
}

// eventFinding turns a discovery event into a finding on that path.
func (e *Engine) eventFinding(ev discover.Event) finding.Finding {
	vars := map[string]string{"name": ev.Detail, "count": strconv.Itoa(ev.Count)}
	id := map[discover.EventKind]string{
		discover.Oversize:       RuleOversize,
		discover.LinkEscape:     RuleLinkEscape,
		discover.ReadError:      RuleReadError,
		discover.InTreeConfig:   RuleInTreeConfig,
		discover.SkippedTargets: RuleSkippedTargets,
		discover.DirLinkEscape:  RuleDirLinkEscape,
		discover.UnresolvedLink: RuleUnresolvedLink,
	}[ev.Kind]
	return e.fileFinding(id, ev.Path, vars)
}

// fileFinding reports a rule against a whole file (line 1, column 1).
func (e *Engine) fileFinding(id, path string, vars map[string]string) finding.Finding {
	return e.RuleFinding(id, path, vars)
}

// RuleFinding builds a whole-file finding (line 1, column 1) for the rule
// with the given ID, for checks done outside the engine such as baseline and
// base-snapshot comparisons. vars fill the rule's message placeholders.
func (e *Engine) RuleFinding(id, path string, vars map[string]string) finding.Finding {
	r := e.Rules.Get(id)
	return newFinding(r, path, 1, 1, 1, 1, "", expand(r.Message, vars), fingerprint(id, path, "", 0))
}

// collector accumulates the findings of one file and enforces the caps.
type collector struct {
	e        *Engine
	doc      *source.Document
	findings []finding.Finding
	perRule  map[string]int
	seen     map[string]int
	dropped  int
	norm     *textview.View // built on first use
	prosev   *textview.View
}

// normalized returns the document's normalised view, building it once.
func (c *collector) normalized() *textview.View {
	if c.norm == nil {
		c.norm = textview.Normalize(c.doc.Text)
	}
	return c.norm
}

// prose returns the document's prose view, building it once.
func (c *collector) prose() *textview.View {
	if c.prosev == nil {
		c.prosev = textview.Prose(c.normalized())
	}
	return c.prosev
}

func (c *collector) add(r *ruleset.Compiled, start, end int, match string, vars map[string]string) {
	if c.perRule[r.ID] >= MaxPerRulePerFile || len(c.findings) >= MaxPerFile {
		c.dropped++
		return
	}
	c.perRule[r.ID]++
	norm := textview.Normalize(match).Text
	key := r.ID + "\x00" + norm
	k := c.seen[key]
	c.seen[key]++

	line, col := c.doc.Lines.Position(start)
	endLine, endCol := c.doc.Lines.Position(end)
	c.findings = append(c.findings, newFinding(r, c.doc.Path, line, col, endLine, endCol,
		excerpt(c.doc, line, col), expand(r.Message, vars), fingerprint(r.ID, c.doc.Path, norm, k)))
}

// flush adds the summary finding for findings dropped by the caps.
func (c *collector) flush() {
	if c.dropped > 0 {
		c.findings = append(c.findings, c.e.fileFinding(RuleFindingCap, c.doc.Path, map[string]string{"count": strconv.Itoa(c.dropped)}))
	}
}

// codepoints reports each run of characters in the rule's ranges.
func (c *collector) codepoints(r *ruleset.Compiled) {
	text := c.doc.Text
	start, n := -1, 0
	report := func(end int) {
		if n > 0 && n >= r.Match.MinRun && !emojiTagSequence(text, start, end) && !wordSeparator(text, start, end) {
			c.add(r, start, end, text[start:end], map[string]string{"count": strconv.Itoa(n), "match": text[start:end], "decoded": decodeTags(text[start:end])})
		}
		start, n = -1, 0
	}
	for i, ch := range text {
		if r.InRanges(ch) {
			if start < 0 {
				start = i
			}
			n++
		} else if n > 0 {
			report(i)
		}
	}
	report(len(text))
}

// exec evaluates an exec-scope rule against each execution point.
func (c *collector) exec(r *ruleset.Compiled, points []extract.ExecPoint) {
	for _, p := range points {
		if !r.HasKind(p.Kind) || !r.MatchTrigger(textview.Normalize(p.Trigger).Text) {
			continue
		}
		if !r.Match.Every && r.MatchUnit(shellView(textview.Normalize(p.Command).Text)) == nil {
			continue
		}
		c.add(r, p.Start, p.End, p.Command, map[string]string{
			"kind": string(p.Kind), "trigger": p.Trigger, "name": p.Name, "match": p.Command,
		})
	}
}

// text evaluates a text-scope rule. view is nil for the raw view; otherwise
// offsets in s are mapped back to the raw text through it.
func (c *collector) text(r *ruleset.Compiled, s string, view *textview.View) {
	toRaw := func(i int) int { return i }
	if view != nil {
		toRaw = view.Raw
	}
	report := func(base int, unit string) {
		for _, loc := range r.MatchUnit(unit) {
			start, end := toRaw(base+loc[0]), toRaw(base+loc[1])
			c.add(r, start, end, c.doc.Text[start:end], map[string]string{"match": oneLine(c.doc.Text[start:end])})
		}
	}
	if !r.MayMatch(s) {
		return
	}
	if r.Match.Unit == ruleset.UnitDocument {
		report(0, s)
		return
	}
	for base := 0; base <= len(s); {
		end := strings.IndexByte(s[base:], '\n')
		if end < 0 {
			end = len(s) - base
		}
		// A CRLF file leaves "\r" at the end of each line; it is not part of
		// the text a rule matches or a message quotes.
		if line := strings.TrimSuffix(s[base:base+end], "\r"); r.MayMatch(line) {
			report(base, line)
		}
		base += end + 1
	}
}

func newFinding(r *ruleset.Compiled, path string, line, col, endLine, endCol int, excerpt, msg, fp string) finding.Finding {
	return finding.Finding{
		RuleID:      r.ID,
		Title:       r.Title,
		Severity:    r.Severity,
		Confidence:  r.Confidence,
		Path:        path,
		Line:        line,
		Column:      col,
		EndLine:     endLine,
		EndColumn:   endCol,
		Excerpt:     excerpt,
		Message:     msg,
		Remediation: strings.Join(strings.Fields(r.Remediation), " "),
		OWASP:       slices.Clone(r.OWASP),
		Fingerprint: fp,
	}
}

// oneLine joins a match that spans lines (prose rules) for its message.
func oneLine(s string) string {
	return lineBreaks.ReplaceAllString(s, " ")
}

var lineBreaks = regexp.MustCompile(`[ \t]*[\r\n]+[ \t]*`)

// excerpt returns the escaped source line of a finding; long lines are cut
// to a window around the column.
func excerpt(doc *source.Document, line, col int) string {
	text, _ := doc.Lines.Line(line)
	runes := []rune(text)
	prefix := ""
	if len(runes) > excerptLimit && col > 60 {
		runes = runes[min(col-1-60, len(runes)):]
		prefix = "…"
	}
	return prefix + escape.String(strings.TrimSpace(string(runes)), excerptLimit)
}

var placeholderRE = regexp.MustCompile(` ?'\{[a-z]+\}'|\{[a-z]+\}`)

// expand substitutes {placeholders} in a single pass, so a value taken from a
// scanned file can never inject another placeholder. Values are escaped;
// unknown placeholders become empty. A quoted placeholder with an empty value
// is dropped together with its quotes and leading space, so the message of an
// unnamed hook reads "claude_hook (SessionStart)" without an empty name.
func expand(msg string, vars map[string]string) string {
	return placeholderRE.ReplaceAllStringFunc(msg, func(m string) string {
		i, j := strings.IndexByte(m, '{'), strings.IndexByte(m, '}')
		v := escape.String(vars[m[i+1:j]], valueLimit)
		if i > 0 && v == "" {
			return ""
		}
		return m[:i] + v + m[j+1:]
	})
}

// fingerprint is stable across line shifts and unique per occurrence
// (ARCHITECTURE §7).
func fingerprint(ruleID, path, match string, k int) string {
	h := sha256.New()
	for _, part := range []string{ruleID, path, match, strconv.Itoa(k)} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:32]
}
