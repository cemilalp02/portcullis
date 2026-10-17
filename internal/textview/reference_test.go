package textview

import (
	"slices"
	"testing"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// normalizeReference is Normalize without the ASCII fast path: every
// segment goes through NFKC and case folding, repeated to a fixed point the
// same way. The fast path must give the same text and offsets.
func normalizeReference(s string) *View {
	v := referenceOnce(s)
	for range 3 {
		if !hasMarkRun(v.Text) {
			break
		}
		w := referenceOnce(v.Text)
		if w.Text == v.Text {
			break
		}
		for i, o := range w.offs {
			w.offs[i] = v.offs[o]
		}
		v = w
	}
	return foldConfusables(v)
}

func referenceOnce(s string) *View {
	filtered := make([]byte, 0, len(s))
	fmap := make([]int32, 0, len(s)+1)
	for i, r := range s {
		if Invisible(r) {
			continue
		}
		n := len(filtered)
		filtered = utf8.AppendRune(filtered, r)
		for range len(filtered) - n {
			fmap = append(fmap, int32(i))
		}
	}
	fold := cases.Fold()
	var out []byte
	var offs []int32
	lastSpace := false
	var it norm.Iter
	it.Init(norm.NFKC, filtered)
	for !it.Done() {
		raw := fmap[it.Pos()]
		for _, r := range norm.NFKC.String(lower(fold.String(string(it.Next())))) {
			if Invisible(r) {
				continue
			}
			if r != '\n' && unicode.IsSpace(r) {
				if lastSpace {
					continue
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
	}
	offs = append(offs, int32(len(s)))
	return &View{Text: string(out), offs: offs}
}

func FuzzNormalizeMatchesReference(f *testing.F) {
	for _, s := range []string{"", "Ignore ALL", "e" + string(rune(0x0301)) + " cafe" + string(rune(0x0301)), "A" + string(rune(0x030A)), "x" + string(rune(0x200B)) + "y",
		"\t a  b\n", string(rune(0x0130)) + string(rune(0x0131)), string(rune(0xFF21)) + "B", "\xff\xfe a"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got, want := Normalize(s), normalizeReference(s)
		if got.Text != want.Text || !slices.Equal(got.offs, want.offs) {
			t.Fatalf("Normalize(%q) = %q %v, reference %q %v", s, got.Text, got.offs, want.Text, want.offs)
		}
	})
}
