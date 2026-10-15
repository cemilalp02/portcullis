package textview

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Prose builds the prose view of a normalised text, for rules that match
// natural-language instructions (ADR-0010). On top of the normalised view it
//
//   - decodes HTML character references (&#73;gnore),
//   - folds look-alike letters from other scripts into Latin (Cyrillic о in
//     an otherwise Latin word) and leetspeak digits inside words (1gn0r3),
//   - drops Markdown emphasis markers (ig**no**re, _all_, ~~prior~~); code
//     backticks stay because rules use them to recognise quoted examples,
//   - joins lines that continue a sentence, so phrases split by a line
//     break still match.
//
// Offsets map back to the raw text like those of the normalised view.
func Prose(v *View) *View {
	toks := decode(v.Text)
	out := make([]byte, 0, len(v.Text))
	offs := make([]int32, 0, len(v.Text)+1)
	emit := func(r rune, src int) {
		n := len(out)
		out = utf8.AppendRune(out, r)
		raw := v.offs[src]
		for range len(out) - n {
			offs = append(offs, raw)
		}
	}
	lastSpace, lastNL := true, true
	space := func(src int) {
		if !lastSpace {
			emit(' ', src)
			lastSpace = true
		}
	}
	for i := 0; i < len(toks); {
		t := toks[i]
		switch {
		case isWordRune(t.r):
			j := i
			for j < len(toks) && isWordRune(toks[j].r) {
				j++
			}
			for _, w := range foldWord(toks[i:j]) {
				emit(w.r, w.src)
			}
			lastSpace, lastNL = false, false
			i = j
			continue
		case t.r == '\n':
			if joinLine(out, toks, i) {
				space(t.src)
			} else {
				trimSpace(&out, &offs)
				emit('\n', t.src)
				lastSpace, lastNL = true, true
			}
		case t.r == '*' || t.r == '~':
			// emphasis and strike-through markers carry no words
		case t.r == '_' && !inWord(toks, i):
			// "_emphasis_" and "__bold__"; snake_case stays intact
		case unicode.IsSpace(t.r):
			if !lastNL {
				space(t.src)
			}
		default:
			emit(t.r, t.src)
			lastSpace, lastNL = false, false
		}
		i++
	}
	offs = append(offs, v.offs[len(v.Text)])
	return &View{Text: string(out), offs: offs}
}

type tok struct {
	r   rune
	src int // byte offset in the normalised text
}

// decode splits s into runes, replacing HTML character references by the
// character they stand for (lower-cased, as the normalised view is).
func decode(s string) []tok {
	toks := make([]tok, 0, len(s))
	for i := 0; i < len(s); {
		if s[i] == '&' {
			if r, n := entity(s[i:]); n > 0 {
				if !Invisible(r) {
					toks = append(toks, tok{unicode.ToLower(r), i})
				}
				i += n
				continue
			}
		}
		r, n := utf8.DecodeRuneInString(s[i:])
		toks = append(toks, tok{r, i})
		i += n
	}
	return toks
}

var named = map[string]rune{"amp": '&', "lt": '<', "gt": '>', "quot": '"', "apos": '\'', "nbsp": ' '}

// entity decodes a character reference at the start of s and returns the
// character and the length of the reference, or 0 if there is none.
func entity(s string) (rune, int) {
	end := strings.IndexByte(s, ';')
	if end < 2 || end > 10 {
		return 0, 0
	}
	body := s[1:end]
	if body[0] != '#' {
		if r, ok := named[body]; ok {
			return r, end + 1
		}
		return 0, 0
	}
	base, digits := 10, body[1:]
	if strings.HasPrefix(digits, "x") {
		base, digits = 16, digits[1:]
	}
	n, err := strconv.ParseUint(digits, base, 32)
	if err != nil || n == 0 || n > unicode.MaxRune || !utf8.ValidRune(rune(n)) {
		return 0, 0
	}
	return rune(n), end + 1
}

// inWord reports whether toks[i] sits between two word characters.
func inWord(toks []tok, i int) bool {
	return i > 0 && isWordRune(toks[i-1].r) && i+1 < len(toks) && isWordRune(toks[i+1].r)
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r)
}

// latinLike are Latin-script letters that stand in for plain ASCII ones.
var latinLike = map[rune]rune{
	0x0131: 'i', // dotless i
	0x0251: 'a', // Latin alpha
	0x0261: 'g', // script g
	0x026A: 'i', // small capital I
	0x1D0F: 'o', // small capital O
}

// confusables are letters of other scripts that look like lower-case Latin
// letters (a subset of Unicode's confusables.txt).
var confusables = map[rune]rune{
	0x0430: 'a', 0x0435: 'e', 0x0450: 'e', 0x0451: 'e', 0x0456: 'i', 0x0457: 'i', 0x0458: 'j',
	0x043E: 'o', 0x0440: 'p', 0x0441: 'c', 0x0443: 'y', 0x0445: 'x', 0x0455: 's', 0x04BB: 'h',
	0x0501: 'd', 0x051B: 'q', 0x051D: 'w', 0x04CF: 'l', // Cyrillic
	0x03BF: 'o', 0x03B1: 'a', 0x03C1: 'p', 0x03B9: 'i', 0x03BA: 'k', 0x03BD: 'v', 0x03C5: 'u', 0x03C7: 'x', // Greek
	0x0585: 'o', 0x057D: 'u', // Armenian
}

var leet = map[rune]rune{'0': 'o', '1': 'i', '3': 'e', '4': 'a', '5': 's', '7': 't'}

// foldWord maps a word to plain Latin where it is written to look like one.
// Letters of another script are folded only when the word also contains
// Latin letters, or consists entirely of look-alikes, so genuine Cyrillic
// or Greek words stay readable for rules written in those languages.
// Digits become letters only in words that also contain letters.
func foldWord(w []tok) []tok {
	latin, foreign, allLook, letters, digits := false, false, true, false, false
	for _, t := range w {
		switch {
		case unicode.IsDigit(t.r):
			digits = true
		case unicode.Is(unicode.Latin, t.r):
			latin, letters = true, true
		case unicode.IsLetter(t.r):
			letters = true
			if _, ok := confusables[t.r]; ok {
				foreign = true
			} else {
				allLook = false
			}
		}
	}
	foldForeign := foreign && (latin || allLook)
	out := make([]tok, 0, len(w))
	for _, t := range w {
		r := t.r
		if m, ok := latinLike[r]; ok {
			r = m
		} else if m, ok := confusables[r]; ok && foldForeign {
			r = m
		} else if m, ok := leet[r]; ok && letters && digits {
			r = m
		}
		out = append(out, tok{r, t.src})
	}
	return out
}

// joinLine reports whether the newline at toks[i] continues a sentence:
// the text so far does not end a sentence and the next line is neither
// blank nor the start of a Markdown block.
func joinLine(out []byte, toks []tok, i int) bool {
	k := len(out)
	for k > 0 && out[k-1] == ' ' {
		k--
	}
	prev, _ := utf8.DecodeLastRune(out[:k])
	if prev == utf8.RuneError || prev == '\n' || strings.ContainsRune(".!?:;。！？", prev) {
		return false
	}
	j := i + 1
	for j < len(toks) && toks[j].r != '\n' && unicode.IsSpace(toks[j].r) {
		j++
	}
	if j >= len(toks) || toks[j].r == '\n' {
		return false
	}
	switch toks[j].r {
	case '#', '-', '*', '+', '>', '|', '`', '<', '[', '{', '}', '"':
		return false
	}
	if unicode.IsDigit(toks[j].r) {
		k := j
		for k < len(toks) && unicode.IsDigit(toks[k].r) {
			k++
		}
		if k < len(toks) && (toks[k].r == '.' || toks[k].r == ')') {
			return false
		}
	}
	return true
}

func trimSpace(out *[]byte, offs *[]int32) {
	for len(*out) > 0 && (*out)[len(*out)-1] == ' ' {
		*out = (*out)[:len(*out)-1]
		*offs = (*offs)[:len(*offs)-1]
	}
}
