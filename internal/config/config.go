// Package config loads Portcullis configuration files (.portcullis.yaml) and
// merges them into effective settings (docs/FILE_FORMATS.md §2, REQUIREMENTS
// FR-40..FR-42). Where a configuration may come from is decided by the
// caller; this package only parses and validates.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/cemilalp02/portcullis/internal/finding"
	"github.com/cemilalp02/portcullis/internal/target"
)

// FileName is the configuration file looked up in trusted mode.
const FileName = ".portcullis.yaml"

// MaxFileSizeLimit caps max_file_size (offsets in the normalised view are
// 32-bit, see internal/textview).
const MaxFileSizeLimit = 1 << 30

// File is the YAML form of a configuration file.
type File struct {
	Version     int      `yaml:"version"`
	FailOn      string   `yaml:"fail_on"`
	MaxFileSize string   `yaml:"max_file_size"`
	Exclude     []string `yaml:"exclude"`
	Rules       Rules    `yaml:"rules"`
}

// Rules holds rule-related settings.
type Rules struct {
	Disable  []string          `yaml:"disable"`
	Severity map[string]string `yaml:"severity"`
	Dirs     []string          `yaml:"dirs"`
}

// Settings are the effective settings after merging every source.
type Settings struct {
	FailOn      finding.Severity
	NoFail      bool // fail_on: none
	MaxFileSize int64
	Exclude     []string
	Disabled    map[string]bool
	Severity    map[string]finding.Severity
	RuleDirs    []string // absolute
	Sources     []string // configuration files applied, in order
}

// Defaults returns the built-in settings.
func Defaults() Settings {
	return Settings{
		FailOn:      finding.High,
		MaxFileSize: 10 << 20,
		Exclude:     []string{"node_modules", "vendor"},
		Disabled:    map[string]bool{},
		Severity:    map[string]finding.Severity{},
	}
}

// Parse decodes a configuration strictly: unknown keys are errors, so a typo
// cannot silently leave a setting at its default.
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
	return &f, nil
}

// Load reads and parses a configuration file.
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(path, data)
}

// ParseFailOn parses a threshold: a severity name or "none".
func ParseFailOn(s string) (sev finding.Severity, none bool, err error) {
	if strings.EqualFold(strings.TrimSpace(s), "none") {
		return finding.Info, true, nil
	}
	sev, err = finding.ParseSeverity(strings.TrimSpace(s))
	return sev, false, err
}

// ParseSize parses sizes such as 10MiB, 512KiB, 2MB or 1048576 (at most 1 GiB).
func ParseSize(s string) (int64, error) {
	units := []struct {
		suffix string
		mult   int64
	}{{"kib", 1 << 10}, {"mib", 1 << 20}, {"gib", 1 << 30}, {"kb", 1e3}, {"mb", 1e6}, {"gb", 1e9}, {"b", 1}}
	t := strings.ToLower(strings.TrimSpace(s))
	mult := int64(1)
	for _, u := range units {
		if strings.HasSuffix(t, u.suffix) {
			t, mult = strings.TrimSpace(strings.TrimSuffix(t, u.suffix)), u.mult
			break
		}
	}
	n, err := strconv.ParseInt(t, 10, 64)
	if err != nil || n <= 0 || n > MaxFileSizeLimit/mult {
		return 0, fmt.Errorf("invalid size %q (want 1 byte to 1GiB, e.g. 10MiB, 512KiB or a byte count)", s)
	}
	return n * mult, nil
}

// engineRule reports whether id is an input-integrity rule. Those rules make
// Portcullis fail closed and can never be disabled or downgraded by a
// configuration file.
func engineRule(id string) bool { return strings.HasPrefix(strings.ToUpper(id), "PCL-ENG-") }

// Apply merges f into s. dir is the directory of the configuration file;
// relative rule directories are resolved against it.
func (s *Settings) Apply(f *File, source, dir string) error {
	var errs []error
	if f.FailOn != "" {
		sev, none, err := ParseFailOn(f.FailOn)
		if err != nil {
			errs = append(errs, fmt.Errorf("fail_on: %w", err))
		}
		s.FailOn, s.NoFail = sev, none
	}
	if f.MaxFileSize != "" {
		n, err := ParseSize(f.MaxFileSize)
		if err != nil {
			errs = append(errs, fmt.Errorf("max_file_size: %w", err))
		}
		s.MaxFileSize = n
	}
	for _, g := range f.Exclude {
		if err := checkGlob(g); err != nil {
			errs = append(errs, fmt.Errorf("exclude: %w", err))
			continue
		}
		s.Exclude = append(s.Exclude, g)
	}
	for _, id := range f.Rules.Disable {
		if engineRule(id) {
			errs = append(errs, fmt.Errorf("rules.disable: %s is an engine integrity rule and cannot be disabled", id))
			continue
		}
		s.Disabled[strings.ToUpper(id)] = true
	}
	for id, v := range f.Rules.Severity {
		if engineRule(id) {
			errs = append(errs, fmt.Errorf("rules.severity: %s is an engine integrity rule and cannot be changed", id))
			continue
		}
		sev, err := finding.ParseSeverity(v)
		if err != nil {
			errs = append(errs, fmt.Errorf("rules.severity.%s: %w", id, err))
			continue
		}
		s.Severity[strings.ToUpper(id)] = sev
	}
	for _, d := range f.Rules.Dirs {
		if !filepath.IsAbs(d) {
			d = filepath.Join(dir, filepath.FromSlash(d))
		}
		s.RuleDirs = append(s.RuleDirs, filepath.Clean(d))
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("%s: %w", source, err)
	}
	s.Sources = append(s.Sources, source)
	return nil
}

func checkGlob(g string) error {
	if g == "" || strings.HasPrefix(g, "/") || strings.Contains(g, `\`) {
		return fmt.Errorf("invalid pattern %q: use a relative '/'-separated glob", g)
	}
	if !target.ValidPattern(g) {
		return fmt.Errorf("invalid pattern %q", g)
	}
	return nil
}

// UserFile returns the per-user configuration path
// (%APPDATA%\portcullis\config.yaml, $XDG_CONFIG_HOME/portcullis/config.yaml
// or ~/Library/Application Support/portcullis/config.yaml), or "" if the
// user configuration directory is unknown.
func UserFile() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "portcullis", "config.yaml")
}
