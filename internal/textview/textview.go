// Package textview builds the normalised view of a document used by pattern
// rules (ADR-0005): invisible characters removed, NFKC, case-folded and
// horizontal whitespace collapsed, with an offset map back to the raw text.
package textview

import (
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// Invisible reports whether r is dropped from the normalised view: format
// characters (Cf, including zero-width, bidi controls and the Tag block),
// variation selectors, the combining grapheme joiner, invisible fillers,
// and control characters other than tab and newline.
func Invisible(r rune) bool {
	switch {
	case r == '\n' || r == '\t':
		return false
	case unicode.IsControl(r), unicode.Is(unicode.Cf, r):
		return true
	case unicode.Is(unicode.Variation_Selector, r):
		return true
	case r == 0x034F, r == 0x115F, r == 0x1160, r == 0x3164, r == 0xFFA0, r == 0x2800:
		return true
	}
	return false
}

// MaxInput is the largest text Normalize accepts; offsets are stored as
// int32 to halve memory use. Longer input is truncated (the engine never
// reads files this large, see the scan --max-file-size limit).
const MaxInput = 1 << 30

// View is the normalised form of a text.
type View struct {
	// Text is the normalised text. Newlines are preserved so line-based rules
	// keep working; runs of other whitespace become a single space.
	Text string
	offs []int32 // offs[i] = raw byte offset for Text[i]; len(offs) == len(Text)+1
}

// Raw maps a byte offset in v.Text (0..len(v.Text)) to a byte offset in the
// raw text the view was built from.
func (v *View) Raw(i int) int {
	i = max(0, min(i, len(v.offs)-1))
	return int(v.offs[i])
}

// Normalize builds the normalised view of s.
func Normalize(s string) *View {
	if len(s) > MaxInput {
		s = s[:MaxInput]
	}
	// Stage 1: drop invisible characters, remembering raw offsets.
	filtered := make([]byte, 0, len(s))
	fmap := make([]int32, 0, len(s)+1)
	for i, r := range s {
		if Invisible(r) {
			continue
		}
		n := len(filtered)
		filtered = utf8.AppendRune(filtered, r)
		for range len(filtered) - n {
			fmap = append(fmap, int32(i)) //nolint:gosec // i < len(s) <= MaxInput < 2^31
		}
	}
	fmap = append(fmap, int32(len(s))) //nolint:gosec // len(s) <= MaxInput < 2^31

	// Stages 2-4: NFKC per normalisation segment, case fold, collapse spaces.
	out := make([]byte, 0, len(filtered))
	offs := make([]int32, 0, len(filtered)+1)
	lastSpace := false
	emit := func(r rune, raw int32) {
		// NFKC can produce a character that stage 1 would have dropped: the
		// norm package inserts U+034F after 30 combining marks (Stream-Safe
		// Text Format). Dropping it here keeps the view idempotent.
		if Invisible(r) {
			return
		}
		if r != '\n' && unicode.IsSpace(r) {
			if lastSpace {
				return
			}
			r = ' '
			lastSpace = true
		} else {
			lastSpace = false
		}
		n := len(out)
		out = utf8.AppendRune(out, r)
		for range len(out) - n {
			offs = append(offs, raw)
		}
	}
	// An ASCII character followed by another ASCII character is its own
	// normalisation segment: NFKC leaves it alone and nothing composes with
	// it, and folding it is lower-casing. Such characters take a fast path;
	// everything else, up to the next such character, goes through NFKC.
	plain := func(i int) bool {
		return filtered[i] < utf8.RuneSelf && (i+1 == len(filtered) || filtered[i+1] < utf8.RuneSelf)
	}
	fold := cases.Fold()
	var it norm.Iter
	for i := 0; i < len(filtered); {
		if plain(i) {
			c := filtered[i]
			if 'A' <= c && c <= 'Z' {
				c += 'a' - 'A'
			}
			emit(rune(c), fmap[i])
			i++
			continue
		}
		j := i + 1
		for j < len(filtered) && !plain(j) {
			j++
		}
		it.Init(norm.NFKC, filtered[i:j])
		for !it.Done() {
			raw := fmap[i+it.Pos()]
			// Full case folding is not idempotent for every script (Cherokee
			// folds lower->upper while upper folds back to lower) and can emit
			// combining marks out of canonical order (Turkish dotted I followed
			// by a mark). Lower-casing and re-normalising gives one stable form.
			for _, r := range norm.NFKC.String(lower(fold.String(string(it.Next())))) {
				emit(r, raw)
			}
		}
		i = j
	}
	offs = append(offs, int32(len(s))) //nolint:gosec // len(s) <= MaxInput < 2^31
	return &View{Text: string(out), offs: offs}
}

func lower(s string) string {
	b := make([]byte, 0, len(s))
	for _, r := range s {
		b = utf8.AppendRune(b, unicode.ToLower(r))
	}
	return string(b)
}
