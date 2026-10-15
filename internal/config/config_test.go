package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cemilalp02/portcullis/internal/finding"
)

func TestApply(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "rules")
	f, err := Parse("cfg", []byte(`version: 1
fail_on: medium
max_file_size: 2MiB
exclude: [third_party/**, generated]
rules:
  disable: [pcl-aex-001]
  severity:
    PCL-AEX-002: high
  dirs: [rules-extra, '`+filepath.ToSlash(abs)+`']
`))
	if err != nil {
		t.Fatal(err)
	}
	s := Defaults()
	if err := s.Apply(f, "cfg", filepath.Join("base", "dir")); err != nil {
		t.Fatal(err)
	}
	if s.FailOn != finding.Medium || s.NoFail || s.MaxFileSize != 2<<20 {
		t.Errorf("threshold/size = %v %v %d", s.FailOn, s.NoFail, s.MaxFileSize)
	}
	if strings.Join(s.Exclude, ",") != "node_modules,vendor,third_party/**,generated" {
		t.Errorf("exclude = %v", s.Exclude)
	}
	if !s.Disabled["PCL-AEX-001"] || s.Severity["PCL-AEX-002"] != finding.High {
		t.Errorf("rules = %v %v", s.Disabled, s.Severity)
	}
	if s.RuleDirs[0] != filepath.Join("base", "dir", "rules-extra") || s.RuleDirs[1] != abs {
		t.Errorf("rule dirs = %v", s.RuleDirs)
	}
	if len(s.Sources) != 1 {
		t.Errorf("sources = %v", s.Sources)
	}

	none, _ := Parse("cfg", []byte("version: 1\nfail_on: none\n"))
	if err := s.Apply(none, "cfg2", "."); err != nil || !s.NoFail {
		t.Errorf("fail_on none: %v %v", s.NoFail, err)
	}
}

func TestParseAndApplyErrors(t *testing.T) {
	tests := []struct{ yaml, want string }{
		{"version: 1\nfailon: high\n", "field failon not found"},
		{"version: 2\n", "unsupported version"},
		{"fail_on: high\n", "unsupported version 0"},
		{"version: 1\nfail_on: severe\n", "fail_on"},
		{"version: 1\nmax_file_size: 2GiB\n", "max_file_size"},
		{"version: 1\nexclude: [/abs]\n", "invalid pattern"},
		{"version: 1\nexclude: ['a/../b']\n", "invalid pattern"},
		{"version: 1\nrules: {disable: [PCL-ENG-006]}\n", "cannot be disabled"},
		{"version: 1\nrules: {severity: {pcl-eng-001: info}}\n", "cannot be changed"},
		{"version: 1\nrules: {severity: {PCL-AEX-001: urgent}}\n", "unknown severity"},
	}
	for _, tt := range tests {
		f, err := Parse("cfg", []byte(tt.yaml))
		if err == nil {
			s := Defaults()
			err = s.Apply(f, "cfg", ".")
		}
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%q: error = %v, want %q", tt.yaml, err, tt.want)
		}
	}
}

func TestLoad(t *testing.T) {
	p := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(p, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err != nil {
		t.Error(err)
	}
	if _, err := Load(p + ".missing"); err == nil {
		t.Error("missing file loaded")
	}
}

func TestParseSize(t *testing.T) {
	for in, want := range map[string]int64{"10MiB": 10 << 20, "512kib": 512 << 10, "2MB": 2e6, "1048576": 1 << 20, "7 b": 7, "1GiB": 1 << 30} {
		if got, err := ParseSize(in); err != nil || got != want {
			t.Errorf("ParseSize(%q) = %d, %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "0", "-1", "ten", "10XB", "2GiB", "1073741825"} {
		if _, err := ParseSize(bad); err == nil {
			t.Errorf("ParseSize(%q) succeeded", bad)
		}
	}
}

func TestUserFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AppData", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	if got := UserFile(); !strings.HasSuffix(filepath.ToSlash(got), "portcullis/config.yaml") {
		t.Errorf("UserFile = %q", got)
	}
}
