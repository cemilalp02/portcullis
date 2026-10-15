// Package target loads the definitions of files Portcullis scans and matches
// paths against them (REQUIREMENTS §5).
package target

import (
	"bytes"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Family groups targets by how they are analysed.
type Family string

// Target families.
const (
	Instruction Family = "instruction" // natural-language text read by agents
	Config      Family = "config"      // structured files that may execute commands
	Script      Family = "script"      // code shipped with an agent skill and run by the agent
)

// Def describes one target type.
type Def struct {
	ID        string   `yaml:"id"`
	Title     string   `yaml:"title"`
	Family    Family   `yaml:"family"`
	Format    string   `yaml:"format"`    // "text" (default) or "jsonc"
	Extractor string   `yaml:"extractor"` // config targets: name of the exec-point extractor
	Patterns  []string `yaml:"patterns"`
	// SkipCount excludes this target from the count of agent files found in
	// skipped dependency directories (every npm dependency has a package.json).
	SkipCount bool `yaml:"skip_count"`
	// CompanionOf makes this a companion target: instead of matching
	// patterns, it covers the files with one of Extensions that live in the
	// directory (and subdirectories) of a file of the named target, e.g. the
	// scripts bundled next to a SKILL.md.
	CompanionOf string   `yaml:"companion_of"`
	Extensions  []string `yaml:"extensions"`
}

// HasExtension reports whether name ends with one of d.Extensions
// (case-insensitive).
func (d *Def) HasExtension(name string) bool {
	name = strings.ToLower(name)
	for _, e := range d.Extensions {
		if strings.HasSuffix(name, strings.ToLower(e)) {
			return true
		}
	}
	return false
}

// Set is an ordered collection of target definitions.
type Set struct {
	defs []*Def
	byID map[string]*Def
}

// Load decodes and validates target definitions (strictly: unknown fields are
// errors).
func Load(data []byte) (*Set, error) {
	var doc struct {
		Schema  int    `yaml:"schema"`
		Targets []*Def `yaml:"targets"`
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("targets: %w", err)
	}
	if doc.Schema != 1 {
		return nil, fmt.Errorf("targets: unsupported schema %d", doc.Schema)
	}
	s := &Set{byID: make(map[string]*Def)}
	var errs []error
	for i, d := range doc.Targets {
		if err := validate(d); err != nil {
			errs = append(errs, fmt.Errorf("targets[%d] %q: %w", i, d.ID, err))
			continue
		}
		if _, dup := s.byID[d.ID]; dup {
			errs = append(errs, fmt.Errorf("targets[%d]: duplicate id %q", i, d.ID))
			continue
		}
		s.byID[d.ID] = d
		s.defs = append(s.defs, d)
	}
	for _, d := range s.defs {
		if d.CompanionOf == "" {
			continue
		}
		if p := s.byID[d.CompanionOf]; p == nil || p.CompanionOf != "" {
			errs = append(errs, fmt.Errorf("targets %q: companion_of %q is not a pattern-based target", d.ID, d.CompanionOf))
		}
	}
	if len(s.defs) == 0 && len(errs) == 0 {
		errs = append(errs, errors.New("targets: no targets defined"))
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return s, nil
}

func validate(d *Def) error {
	if d.ID == "" || d.Title == "" {
		return errors.New("id and title are required")
	}
	if d.Format == "" {
		d.Format = "text"
	}
	switch d.Family {
	case Instruction, Script:
		if d.Format != "text" || d.Extractor != "" {
			return errors.New("instruction and script targets are plain text without an extractor")
		}
	case Config:
		if d.Format != "jsonc" || d.Extractor == "" {
			return errors.New("config targets need format jsonc and an extractor")
		}
	default:
		return fmt.Errorf("unknown family %q", d.Family)
	}
	if d.CompanionOf != "" {
		if len(d.Patterns) > 0 || len(d.Extensions) == 0 {
			return errors.New("companion targets have extensions instead of patterns")
		}
		for _, e := range d.Extensions {
			if !strings.HasPrefix(e, ".") || strings.ContainsAny(e, "/*?[") {
				return fmt.Errorf("invalid extension %q (want e.g. .py)", e)
			}
		}
		return nil
	}
	if len(d.Extensions) > 0 {
		return errors.New("extensions are only valid with companion_of")
	}
	if len(d.Patterns) == 0 {
		return errors.New("at least one pattern is required")
	}
	for _, p := range d.Patterns {
		if err := checkPattern(p); err != nil {
			return err
		}
	}
	return nil
}

// Defs returns the definitions in file order.
func (s *Set) Defs() []*Def { return slices.Clone(s.defs) }

// Get returns the definition with the given ID, or nil.
func (s *Set) Get(id string) *Def { return s.byID[id] }

// Companions returns the companion targets of the target with the given ID.
func (s *Set) Companions(id string) []*Def {
	var out []*Def
	for _, d := range s.defs {
		if d.CompanionOf == id {
			out = append(out, d)
		}
	}
	return out
}

// Match returns the first definition matching the '/'-separated relative
// path, or nil. Matching is case-insensitive because agents on Windows and
// macOS read claude.md as CLAUDE.md.
func (s *Set) Match(rel string) *Def {
	for _, d := range s.defs {
		for _, p := range d.Patterns {
			if MatchPattern(p, rel) {
				return d
			}
		}
	}
	return nil
}

func checkPattern(p string) error {
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, `\`) {
		return fmt.Errorf("invalid pattern %q: must be a relative '/'-separated glob", p)
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return fmt.Errorf("invalid pattern %q: empty, '.' or '..' segment", p)
		}
		if seg == "**" {
			continue
		}
		if _, err := path.Match(seg, ""); err != nil {
			return fmt.Errorf("invalid pattern %q: %w", p, err)
		}
	}
	return nil
}

// ValidPattern reports whether p is a valid relative '/'-separated glob for
// MatchPattern.
func ValidPattern(p string) bool { return checkPattern(p) == nil }

// MatchPattern reports whether the '/'-separated path rel matches the glob
// pattern, case-insensitively. "**" matches zero or more whole segments;
// other segments use path.Match syntax.
func MatchPattern(pattern, rel string) bool {
	return matchSegs(strings.Split(strings.ToLower(pattern), "/"), strings.Split(strings.ToLower(rel), "/"))
}

func matchSegs(pat, name []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			for i := 0; i <= len(name); i++ {
				if matchSegs(pat[1:], name[i:]) {
					return true
				}
			}
			return false
		}
		if len(name) == 0 {
			return false
		}
		if ok, _ := path.Match(pat[0], name[0]); !ok {
			return false
		}
		pat, name = pat[1:], name[1:]
	}
	return len(name) == 0
}
