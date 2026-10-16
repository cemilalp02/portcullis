package ruleset

import (
	"regexp"
	"testing"
)

func TestPrefilterClauses(t *testing.T) {
	tests := []struct {
		pattern, view, unit string
		may, mayNot         []string
	}{
		{`\b(iwr|irm)\b[^|]*\|\s*iex`, ViewNormalized, UnitLine, []string{"irm x | iex"}, []string{"the form is normal"}},
		{`\bne\s+(le\s+)?(dis|dites)\s+pas`, ViewProse, UnitLine, []string{"ne le dis pas"}, []string{"never mind, dis pas"}},
		{`\b(curl|wget)\b[^\n]*\$(token|secret)`, ViewNormalized, UnitLine, []string{"curl -d $token x"}, []string{"never commit a secret"}},
		{`[a-z][\x{0400}-\x{04FF}]`, ViewNormalized, UnitDocument, []string{"aж"}, []string{"plain ascii text"}},
		{`[\x{1F600}-\x{1F64F}]x`, ViewNormalized, UnitLine, []string{"\U0001F642x"}, []string{"x"}},
		{`(?i)ignore`, ViewRaw, UnitLine, []string{"IGNORE", "ignore"}, nil},
		{`\s+`, ViewNormalized, UnitLine, []string{" ", "x y"}, nil},
	}
	for _, tt := range tests {
		m := &Match{Scope: ScopeText, View: tt.view, Unit: tt.unit, Any: []string{tt.pattern}}
		c := &Compiled{lits: prefilter(m)}
		re := regexp.MustCompile(tt.pattern)
		for _, s := range tt.may {
			if re.MatchString(s) && !c.MayMatch(s) {
				t.Errorf("%s: filter rejects matching %q", tt.pattern, s)
			}
		}
		for _, s := range tt.mayNot {
			if c.MayMatch(s) {
				t.Errorf("%s: filter lets %q through (clauses too weak)", tt.pattern, s)
			}
		}
	}
}
