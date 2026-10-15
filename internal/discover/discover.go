// Package discover walks a scan root and finds agent context files
// (ARCHITECTURE §5.1). It never follows symbolic links or Windows junctions:
// it resolves them itself, reports the ones that leave the root, and hands
// the engine a path inside the root to read.
package discover

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cemilalp02/portcullis/internal/target"
)

// DefaultMaxFileSize is the hard per-file limit (REQUIREMENTS FR-04).
const DefaultMaxFileSize = 10 << 20

// DefaultSkipDirs are directories not descended into by default.
var DefaultSkipDirs = []string{"node_modules", "vendor"}

// EventKind classifies something discovery reports instead of a candidate.
type EventKind int

// Event kinds; each maps to one engine rule (PCL-ENG-xxx).
const (
	Oversize       EventKind = iota // target above the size limit (ENG-001)
	LinkEscape                      // target link resolving outside the root (ENG-005)
	ReadError                       // entry could not be read (ENG-006)
	InTreeConfig                    // Portcullis config inside the tree (ENG-007)
	SkippedTargets                  // targets inside a skipped directory (ENG-008)
	DirLinkEscape                   // directory link resolving outside the root (ENG-011)
	UnresolvedLink                  // broken or looping link (ENG-012)
)

// Event is a reportable observation that is not a file to analyse.
type Event struct {
	Kind     EventKind
	Path     string // '/'-separated, relative to the root
	ReadPath string // Oversize only: where the file can be read, relative to the root
	Detail   string
	Count    int
}

// Candidate is a target file to analyse.
type Candidate struct {
	Path     string // as found, '/'-separated, relative to the root
	ReadPath string // where to read it, relative to the root (differs for links)
	Target   *target.Def
	Size     int64
}

// Options controls the walk.
type Options struct {
	MaxFileSize int64    // 0 means DefaultMaxFileSize
	SkipDirs    []string // directory name or path globs; nil means DefaultSkipDirs
}

// Result is the outcome of a walk.
type Result struct {
	Root       string // absolute, fully resolved scan root
	Files      int    // files seen (excluding skipped directories)
	Candidates []Candidate
	Events     []Event
}

// inTreeConfigFiles are ignored in untrusted mode but reported (FR-40).
var inTreeConfigFiles = []string{".portcullis.yaml", ".portcullis-baseline.yaml"}

// agentDirs are directory names whose links are worth reporting even when
// unresolvable, because agents read configuration through them.
var agentDirs = []string{".claude", ".cursor", ".vscode", ".github", ".devcontainer", ".clinerules"}

type walker struct {
	root    string
	targets *target.Set
	opts    Options
	res     *Result
}

// Walk scans root for targets.
func Walk(root string, targets *target.Set, opts Options) (*Result, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	resolvedRoot, err := RealPath(abs)
	if err != nil {
		return nil, fmt.Errorf("resolve scan root: %w", err)
	}
	fi, err := os.Stat(resolvedRoot)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("scan root %s is not a directory", root)
	}
	if opts.MaxFileSize <= 0 {
		opts.MaxFileSize = DefaultMaxFileSize
	}
	if opts.SkipDirs == nil {
		opts.SkipDirs = DefaultSkipDirs
	}
	w := &walker{root: resolvedRoot, targets: targets, opts: opts, res: &Result{Root: resolvedRoot}}
	if err := filepath.WalkDir(resolvedRoot, w.visit); err != nil {
		return nil, err
	}
	w.addCompanions()
	slices.SortFunc(w.res.Candidates, func(a, b Candidate) int { return strings.Compare(a.Path, b.Path) })
	slices.SortFunc(w.res.Events, func(a, b Event) int {
		return cmp.Or(strings.Compare(a.Path, b.Path), cmp.Compare(a.Kind, b.Kind))
	})
	return w.res, nil
}

func (w *walker) event(k EventKind, rel, detail string, count int) {
	w.res.Events = append(w.res.Events, Event{Kind: k, Path: rel, Detail: detail, Count: count})
}

func (w *walker) visit(p string, d fs.DirEntry, err error) error {
	if p == w.root {
		return err
	}
	rel := relSlash(w.root, p)
	if err != nil {
		w.event(ReadError, rel, err.Error(), 0)
		if d != nil && d.IsDir() {
			return fs.SkipDir
		}
		return nil
	}
	if d.Type()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
		w.res.Files++
		w.link(p, rel, d)
		return nil
	}
	if d.IsDir() {
		if strings.EqualFold(d.Name(), ".git") {
			return fs.SkipDir
		}
		if w.skipped(rel, d.Name()) {
			if n := w.countTargets(p); n > 0 {
				w.event(SkippedTargets, rel, "", n)
			}
			return fs.SkipDir
		}
		return nil
	}
	w.res.Files++
	for _, name := range inTreeConfigFiles {
		if strings.EqualFold(rel, name) {
			w.event(InTreeConfig, rel, "", 0)
		}
	}
	def := w.targets.Match(rel)
	if def == nil {
		return nil
	}
	info, err := d.Info()
	if err != nil {
		w.event(ReadError, rel, err.Error(), 0)
		return nil
	}
	w.add(rel, rel, def, info.Size())
	return nil
}

func (w *walker) add(rel, readRel string, def *target.Def, size int64) {
	if size > w.opts.MaxFileSize {
		w.event(Oversize, rel, fmt.Sprintf("%d bytes > %d bytes", size, w.opts.MaxFileSize), 0)
		w.res.Events[len(w.res.Events)-1].ReadPath = readRel
		return
	}
	w.res.Candidates = append(w.res.Candidates, Candidate{Path: rel, ReadPath: readRel, Target: def, Size: size})
}

// link handles a symbolic link, junction or other reparse point.
func (w *walker) link(abs, rel string, d fs.DirEntry) {
	def := w.targets.Match(rel)
	resolved, err := RealPath(abs)
	if err != nil {
		if def != nil || slices.ContainsFunc(agentDirs, func(n string) bool { return strings.EqualFold(n, d.Name()) }) {
			w.event(UnresolvedLink, rel, err.Error(), 0)
		}
		return
	}
	st, statErr := os.Stat(resolved)
	if !Within(w.root, resolved) {
		switch {
		case def != nil:
			w.event(LinkEscape, rel, resolved, 0)
		case statErr == nil && st.IsDir():
			w.event(DirLinkEscape, rel, resolved, 0)
		}
		return
	}
	if statErr != nil {
		if def != nil {
			w.event(UnresolvedLink, rel, statErr.Error(), 0)
		}
		return
	}
	if def == nil || st.IsDir() {
		return // directories inside the root are scanned at their real location
	}
	w.add(rel, relSlash(w.root, resolved), def, st.Size())
}

// addCompanions adds the files that belong to a found target's directory,
// such as the scripts next to a SKILL.md (target.Def.CompanionOf). Links are
// not followed; the main walk has already reported them.
func (w *walker) addCompanions() {
	seen := make(map[string]bool, len(w.res.Candidates))
	for _, c := range w.res.Candidates {
		seen[c.Path] = true
	}
	doneDirs := map[string]bool{}
	for _, c := range slices.Clone(w.res.Candidates) {
		comps := w.targets.Companions(c.Target.ID)
		dir := path.Dir(c.Path)
		if len(comps) == 0 || doneDirs[dir] {
			continue
		}
		doneDirs[dir] = true
		_ = filepath.WalkDir(filepath.Join(w.root, filepath.FromSlash(dir)), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil //nolint:nilerr // unreadable entries were reported by the main walk
			}
			rel := relSlash(w.root, p)
			if d.IsDir() {
				if p != filepath.Join(w.root, filepath.FromSlash(dir)) && (strings.EqualFold(d.Name(), ".git") || w.skipped(rel, d.Name())) {
					return fs.SkipDir
				}
				return nil
			}
			if seen[rel] || d.Type()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
				return nil
			}
			for _, cd := range comps {
				if !cd.HasExtension(d.Name()) {
					continue
				}
				seen[rel] = true
				if info, err := d.Info(); err == nil {
					w.add(rel, rel, cd, info.Size())
				}
				break
			}
			return nil
		})
	}
}

func (w *walker) skipped(rel, name string) bool {
	for _, g := range w.opts.SkipDirs {
		if ok, _ := path.Match(strings.ToLower(g), strings.ToLower(name)); ok {
			return true
		}
		if target.MatchPattern(g, rel) {
			return true
		}
	}
	return false
}

// countTargets counts agent context files below a skipped directory without
// following links (FR-02).
func (w *walker) countTargets(dir string) int {
	n := 0
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // best-effort count; unreadable entries are skipped
		}
		if d.IsDir() {
			if strings.EqualFold(d.Name(), ".git") {
				return fs.SkipDir
			}
			return nil
		}
		if def := w.targets.Match(relSlash(w.root, p)); def != nil && !def.SkipCount {
			n++
		}
		return nil
	})
	return n
}

func relSlash(root, p string) string {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return filepath.ToSlash(p)
	}
	return filepath.ToSlash(rel)
}

// Within reports whether p is root or inside it. Both must be clean,
// absolute, resolved paths.
func Within(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil || filepath.IsAbs(rel) {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

const maxHops = 40

var errTooManyLinks = errors.New("too many levels of links")

// RealPath resolves every symbolic link and Windows junction in p, component
// by component. filepath.EvalSymlinks does not resolve junctions, which is
// why this exists (ARCHITECTURE R5). Reparse points that are not links (for
// example cloud-file placeholders) are treated as ordinary entries.
func RealPath(p string) (string, error) { return resolve(filepath.Clean(p), 0) }

func resolve(p string, hops int) (string, error) {
	vol := filepath.VolumeName(p)
	parts := strings.FieldsFunc(p[len(vol):], func(r rune) bool { return r == '/' || r == filepath.Separator })
	cur := vol + string(filepath.Separator)
	for i, part := range parts {
		next := filepath.Join(cur, part)
		fi, err := os.Lstat(next)
		if err != nil {
			return "", err
		}
		if fi.Mode()&(fs.ModeSymlink|fs.ModeIrregular) == 0 {
			cur = next
			continue
		}
		dest, err := os.Readlink(next)
		if err != nil {
			if fi.Mode()&fs.ModeIrregular != 0 {
				cur = next
				continue
			}
			return "", err
		}
		if hops >= maxHops {
			return "", errTooManyLinks
		}
		if !filepath.IsAbs(dest) {
			dest = filepath.Join(cur, dest)
		}
		return resolve(filepath.Join(append([]string{dest}, parts[i+1:]...)...), hops+1)
	}
	return cur, nil
}
