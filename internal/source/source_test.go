package source

import (
	"testing"
	"unicode/utf8"
)

func utf16le(s string, bom bool) []byte {
	var b []byte
	if bom {
		b = append(b, 0xFF, 0xFE)
	}
	for _, r := range s {
		b = append(b, byte(r), byte(r>>8))
	}
	return b
}

func utf16be(s string, bom bool) []byte {
	var b []byte
	if bom {
		b = append(b, 0xFE, 0xFF)
	}
	for _, r := range s {
		b = append(b, byte(r>>8), byte(r))
	}
	return b
}

func TestDecode(t *testing.T) {
	tests := []struct {
		name     string
		in       []byte
		wantText string
		wantEnc  Encoding
	}{
		{"utf8", []byte("Run make.\n"), "Run make.\n", UTF8},
		{"utf8 turkish", []byte("Çalıştır"), "Çalıştır", UTF8},
		{"utf8 bom stripped", append([]byte{0xEF, 0xBB, 0xBF}, "hi"...), "hi", UTF8BOM},
		{"utf8 bom invalid rest", []byte{0xEF, 0xBB, 0xBF, 'a', 0xFF}, "a" + string(utf8.RuneError), InvalidUTF8},
		{"utf16le bom", utf16le("ignore all", true), "ignore all", UTF16LE},
		{"utf16be bom", utf16be("ignore all", true), "ignore all", UTF16BE},
		{"utf16le no bom", utf16le("ignore previous instructions", false), "ignore previous instructions", UTF16LENoBOM},
		{"utf16be no bom", utf16be("ignore previous instructions", false), "ignore previous instructions", UTF16BENoBOM},
		{"utf16 odd trailing byte", append(utf16le("ab", true), 'x'), "ab" + string(utf8.RuneError), UTF16LE},
		{"invalid utf8", []byte{'a', 0xC3, 'b'}, "a" + string(utf8.RuneError) + "b", InvalidUTF8},
		{"empty", nil, "", UTF8},
		{"short with nul", []byte{'a', 0}, "a\x00", UTF8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, enc := Decode(tt.in)
			if text != tt.wantText || enc != tt.wantEnc {
				t.Errorf("Decode = %q, %q; want %q, %q", text, enc, tt.wantText, tt.wantEnc)
			}
		})
	}
}

func TestEncodingUnusual(t *testing.T) {
	if UTF8.Unusual() || UTF8BOM.Unusual() {
		t.Error("UTF-8 reported as unusual")
	}
	for _, e := range []Encoding{UTF16LE, UTF16BE, UTF16LENoBOM, UTF16BENoBOM, InvalidUTF8} {
		if !e.Unusual() {
			t.Errorf("%s not reported as unusual", e)
		}
	}
}

func TestLineIndex(t *testing.T) {
	text := "first\r\nçok güzel\n\nlast"
	li := NewLineIndex(text)
	if li.Count() != 4 {
		t.Fatalf("Count = %d, want 4", li.Count())
	}
	pos := []struct{ off, line, col int }{
		{0, 1, 1},
		{5, 1, 6},
		{7, 2, 1},
		{7 + len("çok "), 2, 5}, // multi-byte 'ç' counts as one column
		{len(text), 4, 5},
		{-3, 1, 1},
		{len(text) + 10, 4, 5},
	}
	for _, p := range pos {
		if l, c := li.Position(p.off); l != p.line || c != p.col {
			t.Errorf("Position(%d) = %d:%d, want %d:%d", p.off, l, c, p.line, p.col)
		}
	}
	lines := []string{"first", "çok güzel", "", "last"}
	for i, want := range lines {
		if got, _ := li.Line(i + 1); got != want {
			t.Errorf("Line(%d) = %q, want %q", i+1, got, want)
		}
	}
	if got, start := li.Line(0); got != "" || start != -1 {
		t.Errorf("Line(0) = %q, %d", got, start)
	}
	if _, start := li.Line(2); start != 7 {
		t.Errorf("Line(2) start = %d, want 7", start)
	}
}

func TestNewDocument(t *testing.T) {
	d := NewDocument("CLAUDE.md", utf16le("a\nb", true))
	if d.Path != "CLAUDE.md" || d.Text != "a\nb" || d.Encoding != UTF16LE || d.Lines.Count() != 2 {
		t.Errorf("NewDocument = %+v", d)
	}
}

func FuzzDecode(f *testing.F) {
	f.Add([]byte("plain"))
	f.Add(utf16le("x", true))
	f.Add([]byte{0xFF, 0xFE, 0x00, 0xD8}) // lone surrogate
	f.Fuzz(func(t *testing.T, b []byte) {
		text, _ := Decode(b)
		if !utf8.ValidString(text) {
			t.Fatalf("Decode produced invalid UTF-8 for %x", b)
		}
		li := NewLineIndex(text)
		for off := 0; off <= len(text); off++ {
			if l, c := li.Position(off); l < 1 || c < 1 {
				t.Fatalf("Position(%d) = %d:%d", off, l, c)
			}
		}
	})
}
