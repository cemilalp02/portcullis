package archtest

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// hidden reports characters that must never appear literally in the
// project's own source and docs: they are the very techniques Portcullis
// detects (Trojan Source, invisible prompt injection). Test inputs that need
// them live under testdata/ or are written with numeric escapes.
func hidden(r rune) bool {
	switch {
	case unicode.Is(unicode.Cf, r), unicode.Is(unicode.Co, r):
		return true
	case unicode.Is(unicode.Variation_Selector, r):
		return true
	case r == 0x00A0, r == 0x034F, r == 0x115F, r == 0x1160, r == 0x3164, r == 0xFFA0, r == 0x2800:
		return true
	case unicode.IsControl(r) && r != '\t' && r != '\n' && r != '\r':
		return true
	}
	return false
}

func TestRepositoryHasNoHiddenCharacters(t *testing.T) {
	root := filepath.Join("..", "..")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			switch name {
			case ".git", "testdata", "bin", "dist", "node_modules", "__pycache__", ".venv":
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || strings.HasSuffix(name, ".exe") || name == "coverage.out" {
			return nil
		}
		if strings.HasSuffix(name, ".woff2") || strings.HasSuffix(name, ".png") || strings.HasSuffix(name, ".gif") {
			// Published font binaries (internal/report/html/fonts, see its
			// README for their checksums), screenshots and the README demo;
			// never text.
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !utf8.Valid(data) {
			t.Errorf("%s: not valid UTF-8", path)
			return nil
		}
		line := 1
		for i, r := range string(data) {
			if r == '\n' {
				line++
			}
			if hidden(r) && (r != 0xFEFF || i != 0) { // a leading BOM is allowed
				t.Errorf("%s:%d: hidden character U+%04X", path, line, r)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
