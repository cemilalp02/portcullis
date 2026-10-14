// Package escape renders untrusted text safely for terminals, logs and reports
// (REQUIREMENTS FR-34, THREAT_MODEL S4).
package escape

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Visible reports whether r can be shown verbatim. Control, format (Cf),
// bidi, Tag, variation-selector, private-use, unassigned and invisible filler
// characters are not visible.
func Visible(r rune) bool {
	switch {
	case r == utf8.RuneError:
		return false
	case r == ' ':
		return true
	case unicode.IsControl(r), unicode.Is(unicode.Cf, r), unicode.Is(unicode.Co, r):
		return false
	case unicode.Is(unicode.Variation_Selector, r):
		return false
	case r == 0x034F, r == 0x115F, r == 0x1160, r == 0x3164, r == 0xFFA0, r == 0x2800:
		// combining grapheme joiner, Hangul fillers, braille blank
		return false
	case unicode.IsSpace(r):
		// Non-ASCII spaces (NBSP, ideographic space, …) are shown escaped so
		// that look-alike whitespace is obvious.
		return false
	}
	return unicode.IsPrint(r) || unicode.IsGraphic(r)
}

// String escapes every non-visible code point as <U+XXXX> (tab and newline
// as \t and \n) and truncates the result to limit code points of input; limit <= 0
// means no limit. Invalid UTF-8 bytes become <0xNN>.
func String(s string, limit int) string {
	var b strings.Builder
	n := 0
	for i := 0; i < len(s); {
		if limit > 0 && n >= limit {
			b.WriteString("…")
			break
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			fmt.Fprintf(&b, "<0x%02X>", s[i])
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\n':
			b.WriteString(`\n`)
		case Visible(r):
			b.WriteRune(r)
		default:
			fmt.Fprintf(&b, "<U+%04X>", r)
		}
		i += size
		n++
	}
	return b.String()
}
