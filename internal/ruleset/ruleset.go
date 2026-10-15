// Package ruleset loads, validates and compiles detection rules
// (docs/RULE_SCHEMA.md, ADR-0003). Loading is strict: a rule with a typo, an
// invalid regular expression or missing fixtures is an error, never silently
// skipped.
package ruleset

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"go.yaml.in/yaml/v3"

	"github.com/cemilalp02/portcullis/internal/extract"
	"github.com/cemilalp02/portcullis/internal/finding"
	"github.com/cemilalp02/portcullis/internal/target"
)

// Rule is the YAML form of a detection rule.
type Rule struct {
	Schema      int                `yaml:"schema"`
	ID          string             `yaml:"id"`
	Title       string             `yaml:"title"`
	Severity    finding.Severity   `yaml:"severity"`
	Confidence  finding.Confidence `yaml:"confidence"`
	Targets     []string           `yaml:"targets"`
	OWASP       []string           `yaml:"owasp"`
	Since       string             `yaml:"since"`
	Description string             `yaml:"description"`
	Remediation string             `yaml:"remediation"`
	References  []string           `yaml:"references"`
	Message     string             `yaml:"message"`
	Match       Match              `yaml:"match"`
	Tests       *Tests             `yaml:"tests"`
}

// Match selects what a rule looks at and how it matches.
type Match struct {
	Scope      string   `yaml:"scope"`    // "text" (default) or "exec"
	View       string   `yaml:"view"`     // "normalized" (default) or "raw"; text scope
	Unit       string   `yaml:"unit"`     // "line" (default) or "document"; text scope
	Kinds      []string `yaml:"kinds"`    // exec scope; empty means all kinds
	Triggers   []string `yaml:"triggers"` // exec scope; regexes on the normalised trigger, any must match
	Codepoints []string `yaml:"codepoints"`
	Any        []string `yaml:"any"`
	All        []string `yaml:"all"`
	None       []string `yaml:"none"`
	Every      bool     `yaml:"every"`
	Builtin    string   `yaml:"builtin"`
}

// Tests holds a rule's inline fixtures.
type Tests struct {
	Positive []Case `yaml:"positive"`
	Negative []Case `yaml:"negative"`
}

// Case is one fixture: a file path (deciding its target type) and content.
// Target names the target type explicitly, for companion targets that are
// not matched by path (e.g. skill-script).
type Case struct {
	File    string `yaml:"file"`
	Target  string `yaml:"target"`
	Content string `yaml:"content"`
}

// CaseTarget returns the target type a fixture is analysed as, or nil.
func CaseTarget(ts *target.Set, tc Case) *target.Def {
	if ts == nil {
		return nil
	}
	if tc.Target != "" {
		return ts.Get(tc.Target)
	}
	return ts.Match(tc.File)
}

// Scopes, views and units.
const (
	ScopeText      = "text"
	ScopeExec      = "exec"
	ViewNormalized = "normalized"
	ViewRaw        = "raw"
	UnitLine       = "line"
	UnitDocument   = "document"
)

// Range is an inclusive code point range.
type Range struct{ Lo, Hi rune }

// Compiled is a validated rule ready for evaluation.
type Compiled struct {
	Rule
	Any, All, None []*regexp.Regexp
	TriggerREs     []*regexp.Regexp
	Ranges         []Range
	families       map[target.Family]bool
	targetIDs      map[string]bool
	kinds          map[extract.Kind]bool
}

// AppliesTo reports whether the rule inspects files of target d.
func (c *Compiled) AppliesTo(d *target.Def) bool {
	return d != nil && (c.families[d.Family] || c.targetIDs[d.ID])
}

// HasKind reports whether an exec rule inspects execution points of kind k.
func (c *Compiled) HasKind(k extract.Kind) bool { return len(c.kinds) == 0 || c.kinds[k] }

// MatchTrigger reports whether an exec rule applies to an execution point
// with the given (normalised) trigger.
func (c *Compiled) MatchTrigger(trigger string) bool {
	if len(c.TriggerREs) == 0 {
		return true
	}
	for _, re := range c.TriggerREs {
		if re.MatchString(trigger) {
			return true
		}
	}
	return false
}

// InRanges reports whether r is in one of the rule's code point ranges.
func (c *Compiled) InRanges(r rune) bool {
	for _, rg := range c.Ranges {
		if r >= rg.Lo && r <= rg.Hi {
			return true
		}
	}
	return false
}

// MatchUnit applies the any/all/none patterns to one unit of text. It returns
// the [start, end) byte ranges to report, or nil when the unit does not match.
func (c *Compiled) MatchUnit(s string) [][]int {
	for _, re := range c.None {
		if re.MatchString(s) {
			return nil
		}
	}
	var first []int
	for _, re := range c.All {
		loc := re.FindStringIndex(s)
		if loc == nil {
			return nil
		}
		if first == nil {
			first = loc
		}
	}
	if len(c.Any) == 0 {
		if first == nil {
			return nil
		}
		return [][]int{first}
	}
	var locs [][]int
	for _, re := range c.Any {
		locs = append(locs, re.FindAllStringIndex(s, -1)...)
	}
	if len(locs) == 0 {
		return nil
	}
	slices.SortFunc(locs, func(a, b []int) int { return a[0] - b[0] })
	return slices.CompactFunc(locs, func(a, b []int) bool { return a[0] == b[0] })
}

// Placeholders allowed in messages.
var placeholders = []string{"match", "count", "kind", "trigger", "name", "decoded"}

var (
	idRE       = regexp.MustCompile(`^PCL-(UNI|AUT|EXE|BAI|AEX|MCP|SKL|ENG)-\d{3}$`)
	customRE   = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,15}-[A-Z]{3}-\d{3}$`)
	owaspRE    = regexp.MustCompile(`^(ASI|MCP)\d{2}$`)
	sinceRE    = regexp.MustCompile(`^\d{4}\.\d{1,2}\.\d+$`)
	cpRE       = regexp.MustCompile(`^U\+([0-9A-Fa-f]{4,6})(?:\.\.U\+([0-9A-Fa-f]{4,6}))?$`)
	holderRE   = regexp.MustCompile(`\{([a-zA-Z]+)\}`)
	errNoTests = errors.New("tests: at least one positive and one negative case are required")
)

// Options carries what validation needs to know about the rest of the system.
type Options struct {
	Targets  *target.Set
	Builtins []string // names of registered built-in checks
	// Custom loads user rules: IDs use the user's own prefix (ACME-AUT-001),
	// never PCL-, and files may sit directly in the directory or one level
	// below it.
	Custom bool
}

// Parse decodes and validates one rule file. name is the file name used for
// the ID check and error messages.
func Parse(name string, data []byte, opts Options) (*Compiled, error) {
	var r Rule
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&r); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	c, err := compile(&r, opts)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if want := path.Base(name); want != r.ID+".yaml" {
		return nil, fmt.Errorf("%s: file name must be %s.yaml", name, r.ID)
	}
	return c, nil
}

func compile(r *Rule, opts Options) (*Compiled, error) {
	var errs []error
	fail := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	if r.Schema != 1 {
		fail("unsupported schema %d", r.Schema)
	}
	switch {
	case opts.Custom && strings.HasPrefix(r.ID, "PCL-"):
		fail("custom rule %q must not use the PCL- prefix (use your own, e.g. ACME-AUT-001)", r.ID)
	case opts.Custom && !customRE.MatchString(r.ID):
		fail("invalid id %q (want <PREFIX>-<CAT>-<NNN>)", r.ID)
	case !opts.Custom && !idRE.MatchString(r.ID):
		fail("invalid id %q (want PCL-<CAT>-<NNN>)", r.ID)
	}
	if r.Title == "" || len(r.Title) > 80 {
		fail("title is required and must be at most 80 characters")
	}
	if !sinceRE.MatchString(r.Since) {
		fail("since %q is not a rule-set version (YYYY.M.N)", r.Since)
	}
	for _, f := range []struct{ name, value string }{
		{"description", r.Description}, {"remediation", r.Remediation}, {"message", r.Message},
	} {
		if strings.TrimSpace(f.value) == "" {
			fail("%s is required", f.name)
		}
	}
	for _, m := range holderRE.FindAllStringSubmatch(r.Message, -1) {
		if !slices.Contains(placeholders, m[1]) {
			fail("message placeholder {%s} is not one of %v", m[1], placeholders)
		}
	}
	if len(r.OWASP) == 0 {
		fail("owasp mapping is required")
	}
	for _, o := range r.OWASP {
		if !owaspRE.MatchString(o) {
			fail("invalid owasp code %q", o)
		}
	}

	c := &Compiled{Rule: *r, families: map[target.Family]bool{}, targetIDs: map[string]bool{}, kinds: map[extract.Kind]bool{}}
	if len(r.Targets) == 0 {
		fail("targets is required")
	}
	for _, t := range r.Targets {
		switch {
		case t == string(target.Instruction) || t == string(target.Config) || t == string(target.Script):
			c.families[target.Family(t)] = true
		case opts.Targets != nil && opts.Targets.Get(t) != nil:
			c.targetIDs[t] = true
		default:
			fail("unknown target %q", t)
		}
	}

	errs = append(errs, compileMatch(c, opts)...)
	if r.Match.Builtin == "" {
		if r.Tests == nil || len(r.Tests.Positive) == 0 || len(r.Tests.Negative) == 0 {
			errs = append(errs, errNoTests)
		}
	}
	if r.Tests != nil {
		for _, tc := range append(slices.Clone(r.Tests.Positive), r.Tests.Negative...) {
			if d := CaseTarget(opts.Targets, tc); d == nil || !c.AppliesTo(d) {
				fail("test file %q is not a target this rule inspects", tc.File)
			}
		}
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return c, nil
}

func compileMatch(c *Compiled, opts Options) []error {
	var errs []error
	fail := func(format string, args ...any) { errs = append(errs, fmt.Errorf("match: "+format, args...)) }
	m := &c.Match
	if m.Scope == "" {
		m.Scope = ScopeText
	}
	if m.View == "" {
		m.View = ViewNormalized
	}
	if m.Unit == "" {
		m.Unit = UnitLine
	}

	modes := 0
	for _, used := range []bool{len(m.Codepoints) > 0, len(m.Any)+len(m.All) > 0, m.Every, m.Builtin != ""} {
		if used {
			modes++
		}
	}
	if modes != 1 {
		fail("use exactly one of codepoints, any/all, every or builtin")
	}
	switch m.Scope {
	case ScopeText:
		if len(m.Kinds) > 0 || len(m.Triggers) > 0 || m.Every {
			fail("kinds, triggers and every are only valid with scope exec")
		}
	case ScopeExec:
		if len(m.Codepoints) > 0 {
			fail("codepoints are only valid with scope text")
		}
	default:
		fail("unknown scope %q", m.Scope)
	}
	if m.View != ViewNormalized && m.View != ViewRaw {
		fail("unknown view %q", m.View)
	}
	if m.Unit != UnitLine && m.Unit != UnitDocument {
		fail("unknown unit %q", m.Unit)
	}
	if len(m.Codepoints) > 0 && m.View != ViewRaw {
		fail("codepoints need view: raw (invisible characters are removed from the normalised view)")
	}
	if len(m.None) > 0 && len(m.Any)+len(m.All) == 0 {
		fail("none needs any or all")
	}
	if m.Builtin != "" && !slices.Contains(opts.Builtins, m.Builtin) {
		fail("unknown builtin %q", m.Builtin)
	}
	for _, k := range m.Kinds {
		if !slices.Contains(extract.Kinds, extract.Kind(k)) {
			fail("unknown kind %q", k)
		}
		c.kinds[extract.Kind(k)] = true
	}
	for _, cp := range m.Codepoints {
		rg, err := parseRange(cp)
		if err != nil {
			fail("%v", err)
			continue
		}
		c.Ranges = append(c.Ranges, rg)
	}
	for _, set := range []struct {
		src []string
		dst *[]*regexp.Regexp
	}{{m.Any, &c.Any}, {m.All, &c.All}, {m.None, &c.None}, {m.Triggers, &c.TriggerREs}} {
		for _, p := range set.src {
			re, err := regexp.Compile(p)
			if err != nil {
				fail("invalid pattern %q: %v", p, err)
				continue
			}
			*set.dst = append(*set.dst, re)
		}
	}
	return errs
}

func parseRange(s string) (Range, error) {
	m := cpRE.FindStringSubmatch(s)
	if m == nil {
		return Range{}, fmt.Errorf("invalid code point range %q (want U+XXXX or U+XXXX..U+YYYY)", s)
	}
	lo, _ := strconv.ParseUint(m[1], 16, 32)
	hi := lo
	if m[2] != "" {
		hi, _ = strconv.ParseUint(m[2], 16, 32)
	}
	if lo > hi || hi > unicode.MaxRune {
		return Range{}, fmt.Errorf("invalid code point range %q", s)
	}
	return Range{Lo: rune(lo), Hi: rune(hi)}, nil //nolint:gosec // lo <= hi <= unicode.MaxRune fits in a rune
}

// Set is a loaded rule set, ordered by rule ID.
type Set struct {
	rules []*Compiled
	byID  map[string]*Compiled
}

// Load reads every <category>/<ID>.yaml file in fsys.
func Load(fsys fs.FS, opts Options) (*Set, error) {
	files, err := fs.Glob(fsys, "*/*.yaml")
	if err != nil {
		return nil, err
	}
	if opts.Custom {
		top, err := fs.Glob(fsys, "*.yaml")
		if err != nil {
			return nil, err
		}
		files = append(top, files...)
	}
	s := &Set{byID: map[string]*Compiled{}}
	var errs []error
	for _, f := range files {
		data, err := fs.ReadFile(fsys, f)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		c, err := Parse(f, data, opts)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if _, dup := s.byID[c.ID]; dup {
			errs = append(errs, fmt.Errorf("%s: duplicate rule id %s", f, c.ID))
			continue
		}
		s.byID[c.ID] = c
		s.rules = append(s.rules, c)
	}
	if len(s.rules) == 0 && len(errs) == 0 {
		errs = append(errs, errors.New("no rules found"))
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	slices.SortFunc(s.rules, func(a, b *Compiled) int { return strings.Compare(a.ID, b.ID) })
	return s, nil
}

// Merge adds the rules of other to s. Rule IDs must be unique.
func (s *Set) Merge(other *Set) error {
	for _, r := range other.rules {
		if _, dup := s.byID[r.ID]; dup {
			return fmt.Errorf("rule %s is defined twice", r.ID)
		}
	}
	for _, r := range other.rules {
		s.byID[r.ID] = r
		s.rules = append(s.rules, r)
	}
	slices.SortFunc(s.rules, func(a, b *Compiled) int { return strings.Compare(a.ID, b.ID) })
	return nil
}

// Rules returns all rules ordered by ID.
func (s *Set) Rules() []*Compiled { return slices.Clone(s.rules) }

// Get returns the rule with the given ID, or nil.
func (s *Set) Get(id string) *Compiled { return s.byID[id] }
