// Package baseline handles .portcullis-baseline.yaml: findings that were
// reviewed and accepted, keyed by fingerprint, each with a reason
// (docs/FILE_FORMATS.md §3, REQUIREMENTS FR-43). Suppression is only ever
// possible here; scanned files can never suppress their own findings.
package baseline

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/cemilalp02/portcullis/internal/finding"
)

// FileName is the baseline file looked up in trusted mode.
const FileName = ".portcullis-baseline.yaml"

// Entry is one accepted finding. Rule and Path are informational; matching
// is by fingerprint only.
type Entry struct {
	Fingerprint string `yaml:"fingerprint"`
	Rule        string `yaml:"rule"`
	Path        string `yaml:"path"`
	Reason      string `yaml:"reason"`
}

// File is a parsed baseline.
type File struct {
	Version      int     `yaml:"version"`
	Suppressions []Entry `yaml:"suppressions"`
}

var fingerprintRE = regexp.MustCompile(`^[0-9a-f]{32}$`)

// Parse decodes and validates a baseline strictly.
func Parse(name string, data []byte) (*File, error) {
	var f File
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if f.Version != 1 {
		return nil, fmt.Errorf("%s: unsupported version %d (want version: 1)", name, f.Version)
	}
	var errs []error
	seen := map[string]bool{}
	for i, e := range f.Suppressions {
		switch {
		case !fingerprintRE.MatchString(e.Fingerprint):
			errs = append(errs, fmt.Errorf("suppressions[%d]: invalid fingerprint %q", i, e.Fingerprint))
		case strings.TrimSpace(e.Reason) == "":
			errs = append(errs, fmt.Errorf("suppressions[%d] (%s): a reason is required", i, e.Fingerprint))
		case seen[e.Fingerprint]:
			errs = append(errs, fmt.Errorf("suppressions[%d]: duplicate fingerprint %s", i, e.Fingerprint))
		}
		seen[e.Fingerprint] = true
	}
	if err := errors.Join(errs...); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return &f, nil
}

// Load reads and parses a baseline file.
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(path, data)
}

// Apply removes accepted findings. It returns the remaining findings, how
// many were suppressed, and the entries that no longer match anything
// (stale entries should be removed so the file shrinks over time).
func (f *File) Apply(fs []finding.Finding) (kept []finding.Finding, suppressed int, stale []Entry) {
	byFP := make(map[string]bool, len(f.Suppressions))
	for _, e := range f.Suppressions {
		byFP[e.Fingerprint] = true
	}
	used := map[string]bool{}
	for _, x := range fs {
		if byFP[x.Fingerprint] {
			suppressed++
			used[x.Fingerprint] = true
			continue
		}
		kept = append(kept, x)
	}
	for _, e := range f.Suppressions {
		if !used[e.Fingerprint] {
			stale = append(stale, e)
		}
	}
	return kept, suppressed, stale
}

// Generate renders a baseline accepting every given finding with reason.
func Generate(fs []finding.Finding, reason string) ([]byte, error) {
	f := File{Version: 1, Suppressions: []Entry{}}
	seen := map[string]bool{}
	for _, x := range fs {
		if seen[x.Fingerprint] {
			continue
		}
		seen[x.Fingerprint] = true
		f.Suppressions = append(f.Suppressions, Entry{Fingerprint: x.Fingerprint, Rule: x.RuleID, Path: x.Path, Reason: reason})
	}
	return Encode(&f)
}

// Encode renders a baseline file with its explanatory header.
func Encode(f *File) ([]byte, error) {
	var b bytes.Buffer
	b.WriteString("# Portcullis baseline: reviewed findings that are accepted. Every entry needs a reason.\n")
	b.WriteString("# Matching is by fingerprint; rule and path are for reviewers. Remove entries that are reported as stale.\n")
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(f); err != nil {
		return nil, err
	}
	return b.Bytes(), enc.Close()
}
