package discover

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cemilalp02/portcullis/internal/target"
	"github.com/cemilalp02/portcullis/rules"
)

func targets(t *testing.T) *target.Set {
	t.Helper()
	data, err := rules.FS.ReadFile("targets.yaml")
	if err != nil {
		t.Fatal(err)
	}
	s, err := target.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func paths(cs []Candidate) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.Path)
	}
	return out
}

func events(es []Event, k EventKind) []Event {
	var out []Event
	for _, e := range es {
		if e.Kind == k {
			out = append(out, e)
		}
	}
	return out
}

func TestWalkFindsTargets(t *testing.T) {
	root := t.TempDir()
	write(t, root, "CLAUDE.md", "x")
	write(t, root, "docs/claude.md", "x")
	write(t, root, ".vscode/tasks.json", "{}")
	write(t, root, ".github/prompts/a.prompt.md", "x")
	write(t, root, "README.md", "x")
	write(t, root, "src/main.go", "x")
	write(t, root, ".git/CLAUDE.md", "x")                 // never scanned
	write(t, root, "node_modules/pkg/SKILL.md", "x")      // counted, not scanned
	write(t, root, "node_modules/pkg/package.json", "{}") // neither (skip_count)
	write(t, root, "vendor/x/.mcp.json", "{}")            // counted
	write(t, root, "third_party/lib/AGENTS.md", "x")      // excluded by option
	write(t, root, ".portcullis.yaml", "version: 1")      // reported, ignored
	write(t, root, "big/AGENTS.md", strings.Repeat("a", 64))

	res, err := Walk(root, targets(t), Options{MaxFileSize: 32, SkipDirs: []string{"node_modules", "vendor", "third_party/**"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".github/prompts/a.prompt.md", ".vscode/tasks.json", "CLAUDE.md", "docs/claude.md"}
	if got := paths(res.Candidates); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("candidates = %v, want %v", got, want)
	}
	for _, c := range res.Candidates {
		if c.ReadPath != c.Path || c.Target == nil {
			t.Errorf("candidate %+v", c)
		}
	}
	if res.Files != 8 { // excludes .git and skipped dirs
		t.Errorf("Files = %d, want 8", res.Files)
	}
	skipped := events(res.Events, SkippedTargets)
	if len(skipped) != 3 || skipped[0].Path != "node_modules" || skipped[0].Count != 1 ||
		skipped[1].Path != "third_party" || skipped[2].Path != "vendor" {
		t.Errorf("skipped events = %+v", skipped)
	}
	if ev := events(res.Events, Oversize); len(ev) != 1 || ev[0].Path != "big/AGENTS.md" {
		t.Errorf("oversize events = %+v", ev)
	}
	if ev := events(res.Events, InTreeConfig); len(ev) != 1 {
		t.Errorf("in-tree config events = %+v", ev)
	}
}

func TestWalkDefaults(t *testing.T) {
	root := t.TempDir()
	write(t, root, "node_modules/a/CLAUDE.md", "x")
	res, err := Walk(root, targets(t), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 0 || len(events(res.Events, SkippedTargets)) != 1 {
		t.Errorf("default skip dirs not applied: %+v", res)
	}
}

func TestWalkRootErrors(t *testing.T) {
	root := t.TempDir()
	write(t, root, "file.txt", "x")
	if _, err := Walk(filepath.Join(root, "file.txt"), targets(t), Options{}); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("file root error = %v", err)
	}
	if _, err := Walk(filepath.Join(root, "missing"), targets(t), Options{}); err == nil {
		t.Error("missing root accepted")
	}
}

// mkDirLink creates a directory link: a junction on Windows (no privileges
// needed), a symlink elsewhere.
func mkDirLink(t *testing.T, link, dest string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		out, err := exec.Command("cmd", "/c", "mklink", "/J", link, dest).CombinedOutput()
		if err != nil {
			t.Skipf("cannot create junction: %v %s", err, out)
		}
		return
	}
	if err := os.Symlink(dest, link); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}
}

func mkFileLink(t *testing.T, link, dest string) {
	t.Helper()
	if err := os.Symlink(dest, link); err != nil {
		t.Skipf("cannot create file symlink (needs privileges on Windows): %v", err)
	}
}

func TestDirectoryLinks(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "repo")
	outside := filepath.Join(base, "outside")
	write(t, outside, "CLAUDE.md", "secret")
	write(t, root, "inner/AGENTS.md", "x")
	mkDirLink(t, filepath.Join(root, ".claude"), outside)                    // escapes
	mkDirLink(t, filepath.Join(root, "alias"), filepath.Join(root, "inner")) // stays inside

	res, err := Walk(root, targets(t), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := paths(res.Candidates); strings.Join(got, ",") != "inner/AGENTS.md" {
		t.Errorf("candidates = %v (links must not be followed)", got)
	}
	ev := events(res.Events, DirLinkEscape)
	if len(ev) != 1 || ev[0].Path != ".claude" {
		t.Errorf("dir link escape events = %+v", ev)
	}
}

func TestFileLinks(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "repo")
	write(t, base, "outside/creds.txt", "secret")
	write(t, root, "docs/notes.md", "inner")
	mkFileLink(t, filepath.Join(root, "CLAUDE.md"), filepath.Join(base, "outside", "creds.txt"))
	mkFileLink(t, filepath.Join(root, "AGENTS.md"), filepath.Join("docs", "notes.md"))
	mkFileLink(t, filepath.Join(root, "GEMINI.md"), filepath.Join(root, "missing.md"))

	res, err := Walk(root, targets(t), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 1 || res.Candidates[0].Path != "AGENTS.md" || res.Candidates[0].ReadPath != "docs/notes.md" {
		t.Errorf("candidates = %+v, want AGENTS.md read via docs/notes.md", res.Candidates)
	}
	if ev := events(res.Events, LinkEscape); len(ev) != 1 || ev[0].Path != "CLAUDE.md" {
		t.Errorf("link escape events = %+v", ev)
	}
	if ev := events(res.Events, UnresolvedLink); len(ev) != 1 || ev[0].Path != "GEMINI.md" {
		t.Errorf("unresolved link events = %+v", ev)
	}
}

func TestRealPathLoop(t *testing.T) {
	root := t.TempDir()
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	mkFileLink(t, a, b)
	mkFileLink(t, b, a)
	if _, err := RealPath(filepath.Join(root, "a")); err == nil {
		t.Error("link loop resolved")
	}
}

func TestWithin(t *testing.T) {
	root := filepath.Join(string(filepath.Separator)+"x", "repo")
	tests := []struct {
		p    string
		want bool
	}{
		{root, true},
		{filepath.Join(root, "a", "b"), true},
		{filepath.Join(root, "..", "repo2"), false},
		{filepath.Join(root, ".."), false},
		{filepath.Join(root, "..foo"), true}, // a directory literally named "..foo"
	}
	for _, tt := range tests {
		if got := Within(root, tt.p); got != tt.want {
			t.Errorf("Within(%q, %q) = %v, want %v", root, tt.p, got, tt.want)
		}
	}
}
