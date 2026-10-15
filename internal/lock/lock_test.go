package lock

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"
)

func hashOf(t *testing.T, s string) string {
	t.Helper()
	h, err := Hash(strings.NewReader(s))
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestHashNormalisesCRLFOnly(t *testing.T) {
	if hashOf(t, "a\r\nb\r\n") != hashOf(t, "a\nb\n") {
		t.Error("CRLF and LF hash differently")
	}
	for _, other := range []string{"a\rb\n", "a\n\rb\n", "a\nb", "a\nb\n\r", "\xef\xbb\xbfa\nb\n"} {
		if hashOf(t, other) == hashOf(t, "a\nb\n") {
			t.Errorf("%q hashes like the LF version; only CRLF may be normalised", other)
		}
	}
	if !strings.HasPrefix(hashOf(t, ""), "sha256:e3b0c44298fc1c149afbf4c8996fb924") {
		t.Error("empty input hash")
	}
	// CR split across read boundaries must behave the same.
	h1, _ := Hash(iotest.OneByteReader(strings.NewReader("x\r\ny\r\rz\r")))
	if h1 != hashOf(t, "x\ny\r\rz\r") {
		t.Error("one-byte reads change the hash")
	}
	if _, err := Hash(iotest.ErrReader(errors.New("boom"))); err == nil {
		t.Error("read error ignored")
	}
}

func TestFormatParseRoundTrip(t *testing.T) {
	f := File{Ruleset: "2026.10.0", Forced: true, Entries: []Entry{
		{Path: "b/AGENTS.md", Hash: hashOf(t, "b")},
		{Path: "CLAUDE.md", Hash: hashOf(t, "a")},
		{Path: "odd\tname/AGENTS.md", Hash: hashOf(t, "c")},
		{Path: " lead.md", Hash: hashOf(t, "d")},
		{Path: "hidden" + string(rune(0x200B)) + "/CLAUDE.md", Hash: hashOf(t, "e")},
	}}
	data := Format(f)
	tabbed := strconv.Quote("odd\tname/AGENTS.md")
	hidden := strconv.Quote("hidden" + string(rune(0x200B)) + "/CLAUDE.md")
	if !bytes.Contains(data, []byte(tabbed)) || !bytes.Contains(data, []byte(hidden)) {
		t.Errorf("unusual paths not quoted:\n%s", data)
	}
	back, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v\n%s", err, data)
	}
	if back.Ruleset != f.Ruleset || !back.Forced || len(back.Entries) != len(f.Entries) {
		t.Fatalf("round trip = %+v", back)
	}
	if !bytes.Equal(Format(back), data) {
		t.Error("Format(Parse(x)) != x")
	}
	crlf := bytes.ReplaceAll(data, []byte("\n"), []byte("\r\n"))
	if _, err := Parse(crlf); err != nil {
		t.Errorf("CRLF checkout rejected: %v", err)
	}
}

func TestParseRejectsTampering(t *testing.T) {
	good := string(Format(File{Ruleset: "2026.10.0", Entries: []Entry{{Path: "A.md", Hash: hashOf(t, "a")}, {Path: "B.md", Hash: hashOf(t, "b")}}}))
	lines := strings.Split(strings.TrimSuffix(good, "\n"), "\n")
	join := func(l []string) []byte { return []byte(strings.Join(l, "\n") + "\n") }
	mod := func(i int, s string) []byte { c := append([]string(nil), lines...); c[i] = s; return join(c) }
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"empty", nil, "truncated"},
		{"wrong header", mod(0, "# something else"), "unexpected header"},
		{"future format", mod(1, "portcullis-lock 2"), "unsupported format"},
		{"missing ruleset", mod(2, "ruleset "), "ruleset"},
		{"bad forced", mod(3, "forced maybe"), "forced"},
		{"bad hash", mod(4, "sha256:xyz  A.md"), "expected 'sha256"},
		{"uppercase hash", mod(4, strings.ToUpper(lines[4][:71])+lines[4][71:]), "expected 'sha256"},
		{"single space", mod(4, strings.Replace(lines[4], "  ", " ", 1)), "expected 'sha256"},
		{"unsorted", join([]string{lines[0], lines[1], lines[2], lines[3], lines[5], lines[4]}), "sorted"},
		{"duplicate", join([]string{lines[0], lines[1], lines[2], lines[3], lines[4], lines[4]}), "sorted"},
		{"traversal", mod(4, lines[4][:71]+"  ../x.md"), "invalid path"},
		{"absolute", mod(4, lines[4][:71]+"  /etc/x.md"), "invalid path"},
		{"unquoted control char", mod(4, lines[4][:71]+"  a\x1bb.md"), "must be quoted"},
		{"bad quoted", mod(4, lines[4][:71]+`  "unterminated`), "invalid quoted path"},
		{"blank entry line", mod(4, ""), "expected 'sha256"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.data)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Parse error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	approved := []Entry{{"a", "h1"}, {"b", "h2"}, {"c", "h3"}}
	current := []Entry{{"a", "h1"}, {"b", "hX"}, {"d", "h4"}}
	d := Compare(approved, current)
	want := Diff{Added: []string{"d"}, Removed: []string{"c"}, Modified: []string{"b"}}
	if !reflect.DeepEqual(d, want) || d.Empty() {
		t.Errorf("Compare = %+v", d)
	}
	if !Compare(approved, approved).Empty() {
		t.Error("identical sets differ")
	}
}

func TestCompute(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "notes.md"), []byte("x\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Compute(root, []Source{{Path: "CLAUDE.md", ReadPath: "docs/notes.md"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "CLAUDE.md" || got[0].Hash != hashOf(t, "x\n") {
		t.Errorf("Compute = %+v", got)
	}
	if _, err := Compute(root, []Source{{Path: "x", ReadPath: "../escape"}}); err == nil {
		t.Error("read outside the root allowed")
	}
	if _, err := Compute(root, []Source{{Path: "x", ReadPath: "missing"}}); err == nil {
		t.Error("missing file accepted")
	}
	if _, err := Compute(filepath.Join(root, "nope"), nil); err == nil {
		t.Error("missing root accepted")
	}
}

func FuzzParse(f *testing.F) {
	f.Add(Format(File{Ruleset: "2026.10.0", Entries: []Entry{{Path: "CLAUDE.md", Hash: "sha256:" + strings.Repeat("a", 64)}}}))
	f.Add([]byte("garbage"))
	f.Fuzz(func(t *testing.T, data []byte) {
		lf, err := Parse(data)
		if err != nil {
			return
		}
		// Anything accepted must survive a round trip unchanged.
		again, err := Parse(Format(lf))
		if err != nil || !reflect.DeepEqual(again, lf) {
			t.Fatalf("round trip of accepted input failed: %v", err)
		}
	})
}

var _ io.Reader = (*strings.Reader)(nil)
