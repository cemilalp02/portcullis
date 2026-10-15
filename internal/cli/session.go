package cli

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cemilalp02/portcullis/internal/baseline"
	"github.com/cemilalp02/portcullis/internal/config"
	"github.com/cemilalp02/portcullis/internal/discover"
	"github.com/cemilalp02/portcullis/internal/engine"
	"github.com/cemilalp02/portcullis/internal/finding"
	"github.com/cemilalp02/portcullis/internal/lock"
)

// stringList is a repeatable string flag.
type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

// parseArgs parses flags that may appear before or after positional
// arguments and returns the positional ones.
func parseArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return pos, nil
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

// onePath returns the single optional positional path argument.
func onePath(pos []string) (string, error) {
	switch len(pos) {
	case 0:
		return ".", nil
	case 1:
		return pos[0], nil
	}
	return "", fmt.Errorf("expected one path, got %d", len(pos))
}

// protectedFiles are Portcullis's own control files. In pull-request mode
// they are read from the base commit, and changes to them are reported.
var protectedFiles = []string{config.FileName, baseline.FileName, lock.FileName}

// protectedDir holds custom rules and other control files.
const protectedDir = ".portcullis"

// common holds the flags shared by scan, lock, verify and baseline.
type common struct {
	trusted      bool
	configPath   string
	baselinePath string
	snapshot     string
	failOn       string
	maxSize      string
	excludes     stringList
	ruleDirs     stringList
}

func (c *common) register(fs *flag.FlagSet) {
	fs.BoolVar(&c.trusted, "trusted", false, "honour .portcullis.yaml and the baseline inside the scanned repository (only for your own repositories)")
	fs.StringVar(&c.configPath, "config", "", "configuration `file` (must be outside the scanned directory unless -trusted)")
	fs.StringVar(&c.baselinePath, "baseline", "", "baseline `file` of accepted findings")
	fs.StringVar(&c.snapshot, "base-snapshot", "", "`dir` with base-commit copies of Portcullis control files (pull-request mode, implies -trusted)")
	fs.StringVar(&c.failOn, "fail-on", "", "exit 1 if a finding is at or above `severity`: info, low, medium, high, critical or none (default high)")
	fs.StringVar(&c.maxSize, "max-file-size", "", "hard `size` limit per agent file (default 10MiB, at most 1GiB)")
	fs.Var(&c.excludes, "exclude", "skip directories matching `glob` (repeatable)")
	fs.Var(&c.ruleDirs, "rules", "load extra rules from `dir` (repeatable; outside the scanned directory unless -trusted)")
}

// session is the resolved state of one command run.
type session struct {
	root         string // absolute, resolved
	settings     config.Settings
	engine       *engine.Engine
	baseline     *baseline.File
	baselineFile string // path of the baseline in use, for PCL-ENG-013
	snapshot     string // absolute, or ""
	mode         string
	warnings     []string
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// open resolves settings in order: built-in defaults, user configuration,
// --config, the repository's own configuration (trusted mode; from the base
// snapshot in pull-request mode) and finally explicit flags.
func (c *common) open(path string, set map[string]bool) (*session, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	root, err := discover.RealPath(abs)
	if err != nil {
		return nil, fmt.Errorf("scan root: %w", err)
	}
	s := &session{root: root, settings: config.Defaults(), mode: "untrusted"}
	trusted := c.trusted
	if c.snapshot != "" {
		if s.snapshot, err = filepath.Abs(c.snapshot); err != nil {
			return nil, err
		}
		if fi, err := os.Stat(s.snapshot); err != nil || !fi.IsDir() {
			return nil, fmt.Errorf("-base-snapshot %s: not a directory", c.snapshot)
		}
		trusted = true
		s.mode = "pull request (base snapshot)"
	} else if trusted {
		s.mode = "trusted"
	}

	apply := func(p string) error {
		f, err := config.Load(p)
		if err != nil {
			return err
		}
		return s.settings.Apply(f, p, filepath.Dir(p))
	}
	inside := func(p string) bool {
		a, err := filepath.Abs(p)
		if err != nil {
			return true
		}
		if r, err := discover.RealPath(a); err == nil {
			a = r
		}
		return discover.Within(root, a)
	}

	if u := config.UserFile(); u != "" && exists(u) {
		if err := apply(u); err != nil {
			return nil, err
		}
	}
	if c.configPath != "" {
		if !trusted && inside(c.configPath) {
			return nil, fmt.Errorf("-config %s is inside the scanned directory; a scanned repository must not configure its own scanner (use -trusted for your own repositories)", c.configPath)
		}
		if err := apply(c.configPath); err != nil {
			return nil, err
		}
	}
	base := root
	if s.snapshot != "" {
		base = s.snapshot
	}
	if trusted {
		if p := filepath.Join(base, config.FileName); exists(p) {
			if err := apply(p); err != nil {
				return nil, err
			}
		}
	}

	if set["fail-on"] {
		sev, none, err := config.ParseFailOn(c.failOn)
		if err != nil {
			return nil, fmt.Errorf("-fail-on: %w", err)
		}
		s.settings.FailOn, s.settings.NoFail = sev, none
	}
	if set["max-file-size"] {
		n, err := config.ParseSize(c.maxSize)
		if err != nil {
			return nil, fmt.Errorf("-max-file-size: %w", err)
		}
		s.settings.MaxFileSize = n
	}
	s.settings.Exclude = append(s.settings.Exclude, c.excludes...)
	for _, d := range c.ruleDirs {
		if !trusted && inside(d) {
			return nil, fmt.Errorf("-rules %s is inside the scanned directory; use -trusted for your own repositories", d)
		}
		a, err := filepath.Abs(d)
		if err != nil {
			return nil, err
		}
		s.settings.RuleDirs = append(s.settings.RuleDirs, a)
	}

	if s.engine, err = engine.Load(s.settings.RuleDirs...); err != nil {
		return nil, err
	}
	for id := range s.settings.Disabled {
		r := s.engine.Rules.Get(id)
		if r == nil {
			return nil, fmt.Errorf("configuration disables unknown rule %s", id)
		}
		if r.Severity == finding.Critical {
			s.warnings = append(s.warnings, fmt.Sprintf("warning: critical rule %s is disabled by configuration", id))
		}
	}
	for id := range s.settings.Severity {
		if s.engine.Rules.Get(id) == nil {
			return nil, fmt.Errorf("configuration changes the severity of unknown rule %s", id)
		}
	}

	switch {
	case c.baselinePath != "":
		s.baselineFile = c.baselinePath
	case trusted && exists(filepath.Join(base, baseline.FileName)):
		s.baselineFile = filepath.Join(base, baseline.FileName)
	}
	if s.baselineFile != "" {
		if s.baseline, err = baseline.Load(s.baselineFile); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// options returns the engine options for the resolved settings.
func (s *session) options() engine.Options {
	return engine.Options{
		MaxFileSize: s.settings.MaxFileSize,
		SkipDirs:    s.settings.Exclude,
		Disabled:    s.settings.Disabled,
		Severity:    s.settings.Severity,
	}
}

// scan runs the engine and applies trust-mode, base-snapshot and baseline
// handling. It returns the result and the number of suppressed findings.
func (s *session) scan(applyBaseline bool) (*engine.Result, int, error) {
	res, err := s.engine.Scan(s.root, s.options())
	if err != nil {
		return nil, 0, err
	}
	if s.mode != "untrusted" { // in-tree configuration is honoured, not ignored
		res.Findings = slices.DeleteFunc(res.Findings, func(f finding.Finding) bool { return f.RuleID == engine.RuleInTreeConfig })
	}
	if s.snapshot != "" {
		changes, err := protectedChanges(s.root, s.snapshot)
		if err != nil {
			return nil, 0, err
		}
		for _, ch := range changes {
			res.Findings = append(res.Findings, s.engine.RuleFinding(engine.RuleProtected, ch.path, map[string]string{"name": ch.path + " " + ch.state}))
		}
	}
	suppressed := 0
	if applyBaseline && s.baseline != nil {
		var stale []baseline.Entry
		res.Findings, suppressed, stale = s.baseline.Apply(res.Findings)
		where := s.baselineFile
		if rel, err := filepath.Rel(s.root, where); err == nil && !strings.HasPrefix(rel, "..") {
			where = filepath.ToSlash(rel)
		}
		for _, e := range stale {
			res.Findings = append(res.Findings, s.engine.RuleFinding(engine.RuleStaleBaseline, where,
				map[string]string{"name": e.Fingerprint + " (" + e.Rule + " " + e.Path + ")"}))
		}
	}
	finding.Sort(res.Findings)
	return res, suppressed, nil
}

type protectedChange struct{ path, state string }

// protectedChanges compares the control files in the tree with the base
// snapshot (REQUIREMENTS FR-44).
func protectedChanges(root, snapshot string) ([]protectedChange, error) {
	paths := map[string]bool{}
	for _, p := range protectedFiles {
		paths[p] = true
	}
	for _, base := range []string{root, snapshot} {
		dir := filepath.Join(base, protectedDir)
		err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return fs.SkipDir
				}
				return err
			}
			if !d.IsDir() {
				rel, err := filepath.Rel(base, p)
				if err != nil {
					return err
				}
				paths[filepath.ToSlash(rel)] = true
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	var out []protectedChange
	for p := range paths {
		a, aErr := os.ReadFile(filepath.Join(snapshot, filepath.FromSlash(p)))
		b, bErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
		switch {
		case aErr != nil && bErr != nil:
		case aErr != nil:
			out = append(out, protectedChange{p, "added"})
		case bErr != nil:
			out = append(out, protectedChange{p, "removed"})
		case !bytes.Equal(a, b):
			out = append(out, protectedChange{p, "modified"})
		}
	}
	slices.SortFunc(out, func(x, y protectedChange) int { return strings.Compare(x.path, y.path) })
	return out, nil
}

// visited returns the names of the flags set on the command line.
func visited(fs *flag.FlagSet) map[string]bool {
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	return set
}
