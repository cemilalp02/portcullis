package ruleset

import (
	"regexp/syntax"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Literal pre-filter. Running every pattern of every rule over every line is
// what makes a large instruction file slow: bounded repetitions such as
// [^.\n]{0,80} make big regular-expression programs. Most patterns cannot
// match without one of a few strings ("ignore", "curl", "ne dis"), so a rule
// is only evaluated on text that contains one of them. The filter never
// changes a result; TestPrefilterIsSound checks that on every fixture and
// corpus file.

// extractor derives the strings. In the normalised and prose views every run
// of horizontal whitespace inside a line is a single space, so for line rules
// on those views \s, \s+ and \s* stand for exactly " " (or nothing), which
// lets short words combine with their neighbours into selective strings.
type extractor struct{ space bool }

const maxExact = 64

// maxClauses limits how many required sets of one pattern are checked.
const maxClauses = 3

// clauses returns sets of strings such that every match of re contains at
// least one string of each set, best filter first, or nil when none is
// known.
func (x extractor) clauses(re *syntax.Regexp) [][]string {
	switch re.Op {
	case syntax.OpCapture, syntax.OpPlus:
		return x.clauses(re.Sub[0])
	case syntax.OpRepeat:
		if re.Min >= 1 {
			return x.clauses(re.Sub[0])
		}
	case syntax.OpConcat:
		// Every part of a concatenation is in the match, so each part's set
		// is a clause. Runs of parts with small exact sets are multiplied
		// out ("i" then "wr|rm" gives "iwr", "irm").
		var cands [][]string
		run := []string{""}
		flush := func() {
			if !slices.Contains(run, "") {
				cands = append(cands, run)
			}
			run = []string{""}
		}
		for _, s := range re.Sub {
			if set := x.exact(s); set != nil {
				if p := product(run, set); p != nil {
					run = p
					continue
				}
				flush()
				run = set
				continue
			}
			flush()
			cands = append(cands, x.clauses(s)...)
		}
		flush()
		if len(cands) == 0 {
			return nil
		}
		slices.SortStableFunc(cands, func(a, b []string) int { return shortest(b) - shortest(a) })
		return cands[:min(len(cands), maxClauses)]
	case syntax.OpAlternate:
		// One branch matches: the union of each branch's best set.
		var out []string
		for _, s := range re.Sub {
			c := x.clauses(s)
			if c == nil {
				return nil
			}
			out = append(out, c[0]...)
		}
		return [][]string{out}
	}
	if set := x.exact(re); set != nil && !slices.Contains(set, "") {
		return [][]string{set}
	}
	if set := leadBytes(re); set != nil {
		return [][]string{set}
	}
	return nil
}

// leadBytes returns, for a character class without ASCII characters, the
// first bytes of the UTF-8 encodings of its characters: a match contains one
// of them. A Cyrillic class gives four bytes, which rule out all-ASCII text.
func leadBytes(re *syntax.Regexp) []string {
	if re.Op != syntax.OpCharClass || len(re.Rune) == 0 || re.Rune[0] < utf8.RuneSelf {
		return nil
	}
	seen := map[byte]bool{}
	var out []string
	var buf [utf8.UTFMax]byte
	for i := 0; i+1 < len(re.Rune); i += 2 {
		for r := re.Rune[i]; r <= re.Rune[i+1]; r++ {
			if r > 0xFFFF {
				// Characters beyond the BMP start with 0xF0..0xF4.
				for b := byte(0xF0); b <= 0xF4; b++ {
					if !seen[b] {
						seen[b] = true
						out = append(out, string([]byte{b}))
					}
				}
				break
			}
			utf8.EncodeRune(buf[:], r)
			if !seen[buf[0]] {
				seen[buf[0]] = true
				out = append(out, string(buf[:1]))
			}
		}
	}
	return out
}

// exact returns every string re can match when that set is small, or nil.
// Zero-width assertions match only the empty string.
func (x extractor) exact(re *syntax.Regexp) []string {
	switch re.Op {
	case syntax.OpLiteral:
		if re.Flags&syntax.FoldCase != 0 {
			return nil
		}
		return []string{string(re.Rune)}
	case syntax.OpEmptyMatch, syntax.OpBeginLine, syntax.OpEndLine, syntax.OpBeginText, syntax.OpEndText,
		syntax.OpWordBoundary, syntax.OpNoWordBoundary:
		return []string{""}
	case syntax.OpCharClass:
		if x.space && isSpaceClass(re) {
			return []string{" "}
		}
		var out []string
		for i := 0; i+1 < len(re.Rune); i += 2 {
			for r := re.Rune[i]; r <= re.Rune[i+1]; r++ {
				if len(out) == 8 {
					return nil
				}
				out = append(out, string(r))
			}
		}
		return out
	case syntax.OpPlus, syntax.OpStar, syntax.OpRepeat:
		if !x.space || !isSpaceClass(re.Sub[0]) {
			if re.Op == syntax.OpRepeat && re.Min == 1 && re.Max == 1 {
				return x.exact(re.Sub[0])
			}
			return nil
		}
		switch {
		case re.Op == syntax.OpPlus || (re.Op == syntax.OpRepeat && re.Min >= 1):
			return []string{" "}
		default:
			return []string{"", " "}
		}
	case syntax.OpCapture:
		return x.exact(re.Sub[0])
	case syntax.OpQuest:
		if set := x.exact(re.Sub[0]); set != nil {
			return append(set, "")
		}
	case syntax.OpAlternate:
		var out []string
		for _, s := range re.Sub {
			set := x.exact(s)
			if set == nil {
				return nil
			}
			out = append(out, set...)
		}
		if len(out) <= maxExact {
			return out
		}
	case syntax.OpConcat:
		out := []string{""}
		for _, s := range re.Sub {
			set := x.exact(s)
			if set == nil {
				return nil
			}
			if out = product(out, set); out == nil {
				return nil
			}
		}
		return out
	}
	return nil
}

// isSpaceClass reports whether a character class contains only white space
// and includes the space character.
func isSpaceClass(re *syntax.Regexp) bool {
	if re.Op != syntax.OpCharClass {
		return false
	}
	hasSpace := false
	for i := 0; i+1 < len(re.Rune); i += 2 {
		lo, hi := re.Rune[i], re.Rune[i+1]
		if hi-lo > 16 {
			return false
		}
		for r := lo; r <= hi; r++ {
			if !unicode.IsSpace(r) {
				return false
			}
			hasSpace = hasSpace || r == ' '
		}
	}
	return hasSpace
}

func product(a, b []string) []string {
	if len(a)*len(b) > maxExact {
		return nil
	}
	out := make([]string, 0, len(a)*len(b))
	for _, x := range a {
		for _, y := range b {
			out = append(out, x+y)
		}
	}
	return out
}

func shortest(set []string) int {
	n := len(set[0])
	for _, s := range set[1:] {
		n = min(n, len(s))
	}
	return n
}

// minimal drops duplicates and strings that contain another string of the
// set, which a Contains check on the shorter one already covers.
func minimal(set []string) []string {
	slices.SortFunc(set, func(a, b string) int { return len(a) - len(b) })
	var out []string
	for _, s := range set {
		if !slices.ContainsFunc(out, func(o string) bool { return strings.Contains(s, o) }) {
			out = append(out, s)
		}
	}
	return out
}

// filter is a rule's pre-filter: a unit can match only if it contains a
// string of every clause of at least one pattern. union holds the first
// clause of every pattern, a cheap check that rejects most units.
type filter struct {
	union *litIndex
	pats  [][]*litIndex
}

func (f *filter) may(s string) bool {
	if !f.union.in(s) {
		return false
	}
	for _, clauses := range f.pats {
		ok := true
		for _, c := range clauses {
			if !c.in(s) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// prefilter builds the filter for a rule, or nil when an any-pattern has no
// required strings (the rule is then always evaluated).
func prefilter(m *Match) *filter {
	x := extractor{space: m.Scope == ScopeText && m.View != ViewRaw && m.Unit == UnitLine}
	parse := func(p string) [][]string {
		re, err := syntax.Parse(p, syntax.Perl)
		if err != nil {
			return nil
		}
		var out [][]string
		for _, c := range x.clauses(re) {
			if shortest(c) > 0 {
				out = append(out, minimal(c))
			}
		}
		return out
	}
	index := func(cs [][]string) []*litIndex {
		out := make([]*litIndex, len(cs))
		for i, c := range cs {
			out[i] = newLitIndex(c)
		}
		return out
	}
	f := &filter{}
	var first []string
	if len(m.Any) > 0 {
		for _, p := range m.Any {
			cs := parse(p)
			if cs == nil {
				return nil
			}
			first = append(first, cs[0]...)
			f.pats = append(f.pats, index(cs))
		}
	} else {
		// All patterns must match, so every clause of every one applies.
		var all [][]string
		for _, p := range m.All {
			all = append(all, parse(p)...)
		}
		if all == nil {
			return nil
		}
		slices.SortStableFunc(all, func(a, b []string) int { return shortest(b) - shortest(a) })
		first = all[0]
		f.pats = [][]*litIndex{index(all)}
	}
	f.union = newLitIndex(minimal(first))
	return f
}

// litIndex finds whether a text contains any of a set of strings in one pass:
// a bitmap of the strings' first two bytes picks the positions worth
// comparing, so the cost does not grow with the number of strings.
type litIndex struct {
	single  []byte // strings of one byte
	bigrams [1 << 16 / 64]uint64
	byPair  map[uint16][]string
}

func newLitIndex(lits []string) *litIndex {
	x := &litIndex{byPair: map[uint16][]string{}}
	for _, l := range lits {
		if len(l) == 1 {
			x.single = append(x.single, l[0])
			continue
		}
		p := uint16(l[0])<<8 | uint16(l[1])
		x.bigrams[p/64] |= 1 << (p % 64)
		x.byPair[p] = append(x.byPair[p], l)
	}
	return x
}

func (x *litIndex) in(s string) bool {
	for _, b := range x.single {
		if strings.IndexByte(s, b) >= 0 {
			return true
		}
	}
	for i := 0; i+1 < len(s); i++ {
		p := uint16(s[i])<<8 | uint16(s[i+1])
		if x.bigrams[p/64]&(1<<(p%64)) == 0 {
			continue
		}
		for _, l := range x.byPair[p] {
			if strings.HasPrefix(s[i:], l) {
				return true
			}
		}
	}
	return false
}

// MayMatch reports whether the rule's patterns can match s at all. A false
// result is certain; a true one still needs MatchUnit.
func (c *Compiled) MayMatch(s string) bool {
	return c.lits == nil || c.lits.may(s)
}
