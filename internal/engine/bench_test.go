package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cemilalp02/portcullis/internal/textview"
)

// Performance (REQUIREMENTS NFR-04): a repository with fewer than 5,000
// files scans in under a second. The benchmarks build synthetic trees: source
// files that are walked but not read, plus agent context files every 50th
// file (a high share; real repositories have far fewer).

const claudeMD = "# Project\n\nRun `make test` before committing. Use Go 1.27.\n" +
	"Never commit secrets; read them from the environment.\n"

const settingsJSON = `{
  "permissions": {"allow": ["Bash(npm test)", "Bash(go test ./...)"]},
  "hooks": {"PostToolUse": [{"matcher": "Edit", "hooks": [{"type": "command", "command": "npx prettier --write ."}]}]}
}
`

func makeTree(tb testing.TB, files int) string {
	tb.Helper()
	root := tb.TempDir()
	for i := range files {
		dir := filepath.Join(root, fmt.Sprintf("pkg%03d", i/100), fmt.Sprintf("mod%02d", i%100/10))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			tb.Fatal(err)
		}
		name, body := fmt.Sprintf("file%05d.go", i), "package x\n"
		switch i % 100 {
		case 0:
			name, body = "CLAUDE.md", claudeMD
		case 50:
			dir = filepath.Join(dir, ".claude")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				tb.Fatal(err)
			}
			name, body = "settings.json", settingsJSON
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			tb.Fatal(err)
		}
	}
	return root
}

func BenchmarkScan5000Files(b *testing.B) {
	e, err := Default()
	if err != nil {
		b.Fatal(err)
	}
	root := makeTree(b, 5000)
	b.ResetTimer()
	for b.Loop() {
		res, err := e.Scan(root, Options{})
		if err != nil || res.Files != 5000 {
			b.Fatalf("scan: %v, %d files", err, res.Files)
		}
	}
}

// TestScan5000FilesUnderOneSecond guards NFR-04 in the normal test run, with
// a generous margin for slow CI machines.
func TestScan5000FilesUnderOneSecond(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a 5,000-file tree")
	}
	e := defaultEngine(t)
	root := makeTree(t, 5000)
	if _, err := e.Scan(root, Options{}); err != nil { // warm the file-system cache
		t.Fatal(err)
	}
	best := time.Duration(1 << 62)
	for range 3 {
		start := time.Now()
		if _, err := e.Scan(root, Options{}); err != nil {
			t.Fatal(err)
		}
		best = min(best, time.Since(start))
	}
	t.Logf("5,000 files: best of 3 scans %v", best)
	if best > time.Second {
		t.Errorf("scan of 5,000 files took %v, want < 1s (NFR-04)", best)
	}
}

func BenchmarkNormalize(b *testing.B) {
	text := strings.Repeat(claudeMD, 2000) // ~220 KB
	b.SetBytes(int64(len(text)))
	for b.Loop() {
		textview.Normalize(text)
	}
}

func BenchmarkProse(b *testing.B) {
	v := textview.Normalize(strings.Repeat(claudeMD, 2000))
	b.SetBytes(int64(len(v.Text)))
	for b.Loop() {
		textview.Prose(v)
	}
}

func BenchmarkScanLargeInstructionFile(b *testing.B) {
	e, err := Default()
	if err != nil {
		b.Fatal(err)
	}
	root := b.TempDir()
	text := strings.Repeat(claudeMD, 20000) // ~2.2 MB, close to the default size limit
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte(text), 0o644); err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(text)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := e.Scan(root, Options{}); err != nil {
			b.Fatal(err)
		}
	}
}
