package textview

import (
	"testing"
)

// Look-alike letters are built from code points so the source stays readable:
// cyr(0x0456) is Cyrillic і, which looks like Latin i.
func cyr(r rune) string { return string(r) }

func TestConfusablesInMixedWords(t *testing.T) {
	i, o, e, a := cyr(0x0456), cyr(0x043E), cyr(0x0435), cyr(0x0430) // Cyrillic і о е а
	gOmicron := cyr(0x03BF)                                          // Greek ο
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"cyrillic letters inside latin words", i + "gnore prev" + i + "ous " + i + "nstruct" + i + o + "ns", "ignore previous instructions"},
		{"greek omicron inside a latin word", "f" + gOmicron + "llow", "follow"},
		{"upper-case look-alikes fold first", cyr(0x0406) + "GNORE", "ignore"},
		{"command name", "curl -s x | b" + a + "sh", "curl -s x | bash"},
		{"plain russian is kept", "игнорируй все", "игнорируй все"},
		{"russian word of look-alikes only is kept", cyr(0x0441) + o + cyr(0x0440), cyr(0x0441) + o + cyr(0x0440)},
		{"greek text is kept", "λόγος", "λόγοσ"}, // final sigma folds, as before
		{"mixed with a combining mark is kept", "x" + e + string(rune(0x0301)), "x" + e + string(rune(0x0301))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Normalize(tt.in).Text; got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestConfusableOffsets(t *testing.T) {
	in := "x " + cyr(0x0456) + "gnore"
	v := Normalize(in)
	if v.Text != "x ignore" {
		t.Fatalf("text = %q", v.Text)
	}
	// "i" (one byte in the view) maps back to the two-byte Cyrillic letter.
	if v.Raw(2) != 2 || v.Raw(3) != 4 || v.Raw(len(v.Text)) != len(in) {
		t.Errorf("offsets: Raw(2)=%d Raw(3)=%d Raw(end)=%d", v.Raw(2), v.Raw(3), v.Raw(len(v.Text)))
	}
}
