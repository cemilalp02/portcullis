package textview

import (
	"unicode"
	"unicode/utf8"
)

// foldConfusables replaces look-alike letters of other scripts (the table the
// prose view uses, see prose.go) with their Latin counterparts inside words
// that also contain Latin letters: a word mixing scripts is
// either a mistake or an attempt to hide it from pattern matching. Words in
// one script (Russian, Greek) are left alone, so rules written in those
// languages still match. A letter followed by a combining mark is kept too:
// replacing it would let the mark compose with the new letter and change the
// view on a second normalisation.
func foldConfusables(v *View) *View {
	text := v.Text
	if !hasConfusable(text) {
		return v
	}
	out := make([]byte, 0, len(text))
	offs := make([]int32, 0, len(text)+1)
	for i := 0; i < len(text); {
		j := i
		for j < len(text) {
			r, n := utf8.DecodeRuneInString(text[j:])
			if !wordRune(r) {
				break
			}
			j += n
		}
		if j == i { // not part of a word
			_, n := utf8.DecodeRuneInString(text[i:])
			out = append(out, text[i:i+n]...)
			offs = append(offs, v.offs[i:i+n]...)
			i += n
			continue
		}
		mixed := mixedWord(text[i:j])
		for k := i; k < j; {
			r, n := utf8.DecodeRuneInString(text[k:])
			if c, ok := confusables[r]; ok && mixed {
				next, _ := utf8.DecodeRuneInString(text[k+n:])
				if k+n >= len(text) || !unicode.IsMark(next) {
					out = utf8.AppendRune(out, c)
					offs = append(offs, v.offs[k])
					k += n
					continue
				}
			}
			out = append(out, text[k:k+n]...)
			offs = append(offs, v.offs[k:k+n]...)
			k += n
		}
		i = j
	}
	offs = append(offs, v.offs[len(text)])
	return &View{Text: string(out), offs: offs}
}

func wordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r)
}

// mixedWord reports whether a word has both a Latin letter and a look-alike.
func mixedWord(w string) bool {
	latin, look := false, false
	for _, r := range w {
		switch {
		case r >= 'a' && r <= 'z':
			latin = true
		case confusables[r] != 0:
			look = true
		}
	}
	return latin && look
}

func hasConfusable(s string) bool {
	for _, r := range s {
		if confusables[r] != 0 {
			return true
		}
	}
	return false
}
