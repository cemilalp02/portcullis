package engine

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/cemilalp02/portcullis/internal/finding"
)

// The corpus lives in txtar archives (see testdata/corpus/README.md) rather
// than as real agent files, so that agents working on this repository never
// load the defanged attack samples as instructions.

var txtarMarker = regexp.MustCompile(`^-- (.+) --$`)

// parseTxtar splits an archive into its comment and files. File contents are
// kept byte-exact (some samples are UTF-16 or contain control characters).
func parseTxtar(data []byte) (string, map[string][]byte) {
	var comment bytes.Buffer
	files := map[string][]byte{}
	current := ""
	for _, line := range bytes.SplitAfter(data, []byte("\n")) {
		if m := txtarMarker.FindSubmatch(bytes.TrimRight(line, "\n")); m != nil {
			current = string(m[1])
			files[current] = []byte{}
			continue
		}
		if current == "" {
			comment.Write(line)
		} else {
			files[current] = append(files[current], line...)
		}
	}
	return comment.String(), files
}

func unpack(t *testing.T, files map[string][]byte) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func corpus(t *testing.T, kind string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", "testdata", "corpus", kind, "*.txtar"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no %s corpus: %v", kind, err)
	}
	return files
}

// TestKnownCorpus is the Phase 4 gate: every rule expected for every known
// attack sample must fire (100% recall on corpus/known).
func TestKnownCorpus(t *testing.T) {
	e := defaultEngine(t)
	expected, detected := 0, 0
	exercised := map[string]bool{}
	for _, path := range corpus(t, "known") {
		comment, files := parseTxtar(mustRead(t, path))
		var want []string
		for _, line := range strings.Split(comment, "\n") {
			if rest, ok := strings.CutPrefix(line, "expect:"); ok {
				want = append(want, strings.Fields(rest)...)
			}
		}
		if len(want) == 0 {
			t.Errorf("%s: no expect line", path)
		}
		res, err := e.Scan(unpack(t, files), Options{})
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		got := map[string]bool{}
		for _, f := range res.Findings {
			got[f.RuleID] = true
		}
		for _, id := range want {
			if e.Rules.Get(id) == nil {
				t.Errorf("%s: expects unknown rule %s", path, id)
				continue
			}
			expected++
			exercised[id] = true
			if got[id] {
				detected++
			} else {
				t.Errorf("%s: %s did not fire (got %v)", filepath.Base(path), id, keys(got))
			}
		}
	}
	t.Logf("known corpus: %d/%d expected detections (recall %.1f%%)", detected, expected, 100*float64(detected)/float64(expected))

	var untested []string
	for _, r := range e.Rules.Rules() {
		if r.Match.Builtin != builtinEngine && !exercised[r.ID] {
			untested = append(untested, r.ID)
		}
	}
	t.Logf("detection rules without a corpus sample (covered by inline fixtures only): %v", untested)
}

// TestCleanCorpus guards against false positives: realistic legitimate
// repositories must not produce findings at or above the default threshold.
func TestCleanCorpus(t *testing.T) {
	e := defaultEngine(t)
	for _, path := range corpus(t, "clean") {
		_, files := parseTxtar(mustRead(t, path))
		res, err := e.Scan(unpack(t, files), Options{})
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, f := range res.Findings {
			if f.Severity >= finding.High {
				t.Errorf("%s: false positive %s at %s:%d: %s", filepath.Base(path), f.RuleID, f.Path, f.Line, f.Message)
			}
		}
	}
}

func TestParseTxtar(t *testing.T) {
	comment, files := parseTxtar([]byte("desc\nexpect: A B\n-- a/x.md --\nline1\n-- y --\n\x00\x01\n"))
	if comment != "desc\nexpect: A B\n" || string(files["a/x.md"]) != "line1\n" || !bytes.Equal(files["y"], []byte{0, 1, '\n'}) {
		t.Errorf("parseTxtar = %q, %q", comment, files)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
