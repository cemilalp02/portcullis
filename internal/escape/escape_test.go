package escape

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestString(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		limit int
		want  string
	}{
		{"plain", "make build", 0, "make build"},
		{"turkish letters stay", "Bekçi ğüşıöÇ", 0, "Bekçi ğüşıöÇ"},
		{"tab and newline", "a\tb\nc", 0, `a\tb\nc`},
		{"ansi escape", "\x1b[31mred", 0, "<U+001B>[31mred"},
		{"zero width space", "ig\u200Bnore", 0, "ig<U+200B>nore"},
		{"bidi override", "a\u202Eb", 0, "a<U+202E>b"},
		{"unicode tag", "x\U000E0069", 0, "x<U+E0069>"},
		{"nbsp", "a\u00A0b", 0, "a<U+00A0>b"},
		{"hangul filler", "a\u3164b", 0, "a<U+3164>b"},
		{"variation selector", "a\uFE0Fb", 0, "a<U+FE0F>b"},
		{"invalid utf8", "a\xffb", 0, "a<0xFF>b"},
		{"truncate", "abcdef", 3, "abc…"},
		{"no truncate at limit", "abc", 3, "abc"},
		{"emoji visible", "ok ✅", 0, "ok ✅"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := String(tt.in, tt.limit); got != tt.want {
				t.Errorf("String(%q, %d) = %q, want %q", tt.in, tt.limit, got, tt.want)
			}
		})
	}
}

func FuzzString(f *testing.F) {
	for _, s := range []string{"", "a\x1b[2J", "\U000E0041\u202E", "\xff\xfe"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		out := String(s, 0)
		if !utf8.ValidString(out) {
			t.Fatalf("output is not valid UTF-8: %q", out)
		}
		for _, r := range out {
			if r != ' ' && !Visible(r) && !strings.ContainsRune("…", r) {
				t.Fatalf("output contains non-visible rune %U: %q", r, out)
			}
		}
	})
}
