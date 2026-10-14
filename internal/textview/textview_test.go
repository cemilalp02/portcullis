package textview

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// Hidden characters are built from code points so the source file itself
// stays free of them (see internal/archtest).
var (
	zwsp     = string(rune(0x200B))
	rlo      = string(rune(0x202E))
	tagI     = string(rune(0xE0069))
	vs16     = string(rune(0xFE0F))
	nbsp     = string(rune(0x00A0))
	filler   = string(rune(0x3164))
	fullwide = func(s string) string {
		var b strings.Builder
		for _, r := range s {
			if r >= '!' && r <= '~' {
				r += 0xFEE0 // ASCII -> full-width form
			}
			b.WriteRune(r)
		}
		return b.String()
	}
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"lowercase", "Ignore Previous INSTRUCTIONS", "ignore previous instructions"},
		{"zero width split", "ig" + zwsp + "nore", "ignore"},
		{"full width", fullwide("IGNORE"), "ignore"},
		{"bidi and tags dropped", "a" + rlo + "b" + tagI, "ab"},
		{"variation selector dropped", "ok" + vs16, "ok"},
		{"filler dropped", "a" + filler + "b", "ab"},
		{"whitespace collapsed", "curl  \t -s" + nbsp + "x", "curl -s x"},
		{"newlines kept", "a\n\nb", "a\n\nb"},
		{"crlf to lf", "a\r\nb", "a\nb"},
		{"ansi escape dropped", "\x1b[31mred", "[31mred"},
		{"turkish dotted I folds", "İ", "i̇"},
		{"sharp s folds", "STRASSE straße", "strasse strasse"},
		{"ligature", "ﬁle", "file"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Normalize(tt.in).Text; got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRawOffsets(t *testing.T) {
	raw := "line one\nPlease IG" + zwsp + "NORE previous"
	v := Normalize(raw)
	i := strings.Index(v.Text, "ignore")
	if i < 0 {
		t.Fatalf("normalised text %q lacks match", v.Text)
	}
	start, end := v.Raw(i), v.Raw(i+len("ignore"))
	if got := raw[start:end]; got != "IG"+zwsp+"NORE" {
		t.Errorf("raw span = %q", got)
	}
	if v.Raw(-5) != 0 || v.Raw(len(v.Text)+5) != len(raw) {
		t.Error("Raw does not clamp")
	}
}

func TestInvisible(t *testing.T) {
	for _, r := range []rune{0x200B, 0x200D, 0x202E, 0x2066, 0xFEFF, 0xE0041, 0xFE0F, 0x034F, 0x3164, 0x1B, 0x7F} {
		if !Invisible(r) {
			t.Errorf("Invisible(%U) = false", r)
		}
	}
	for _, r := range []rune{'a', ' ', '\n', '\t', 'ç', 0x4E2D} {
		if Invisible(r) {
			t.Errorf("Invisible(%U) = true", r)
		}
	}
}

// Every byte of the normalised text must map into the raw text, offsets must
// never decrease, and re-normalising the mapped raw span of each segment must
// reproduce text containing it.
func FuzzNormalize(f *testing.F) {
	for _, s := range []string{"", "Ignore ALL", "ig" + zwsp + "nore", fullwide("RUN"), "a\r\nb", "\xff\xfe", "İß"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		v := Normalize(s)
		if !utf8.ValidString(v.Text) && utf8.ValidString(s) {
			t.Fatalf("invalid UTF-8 output for valid input %q", s)
		}
		if len(v.offs) != len(v.Text)+1 {
			t.Fatalf("offset map length %d for text length %d", len(v.offs), len(v.Text))
		}
		prev := 0
		for i := range len(v.Text) + 1 {
			o := v.Raw(i)
			if o < prev || o > len(s) {
				t.Fatalf("offset %d at %d out of order/range (prev %d, len %d)", o, i, prev, len(s))
			}
			prev = o
		}
		if again := Normalize(v.Text).Text; again != v.Text && utf8.ValidString(s) {
			// Normalisation must be idempotent so rules see stable input.
			t.Fatalf("not idempotent: %q -> %q -> %q", s, v.Text, again)
		}
	})
}
