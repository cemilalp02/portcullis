package engine

import (
	"strings"
	"testing"

	"github.com/cemilalp02/portcullis/internal/ruleset"
	"github.com/cemilalp02/portcullis/internal/textview"
)

// TestPrefilterIsSound checks that the literal pre-filter never hides a match:
// wherever a text rule's patterns match a unit of a fixture or corpus file,
// MayMatch must allow that unit.
func TestPrefilterIsSound(t *testing.T) {
	e := defaultEngine(t)
	var texts []string
	for _, r := range e.Rules.Rules() {
		if r.Tests != nil {
			for _, c := range append(append([]ruleset.Case{}, r.Tests.Positive...), r.Tests.Negative...) {
				texts = append(texts, c.Content)
			}
		}
	}
	for _, kind := range []string{"known", "clean", "heldout"} {
		for _, path := range corpus(t, kind) {
			_, files := parseTxtar(mustRead(t, path))
			for _, content := range files {
				texts = append(texts, string(content))
			}
		}
	}
	checked := 0
	for _, text := range texts {
		norm := textview.Normalize(text)
		views := map[string]string{
			ruleset.ViewRaw: text, ruleset.ViewNormalized: norm.Text, ruleset.ViewProse: textview.Prose(norm).Text,
		}
		for _, r := range e.Rules.Rules() {
			if r.Match.Scope != ruleset.ScopeText || len(r.Any)+len(r.All) == 0 {
				continue
			}
			s := views[r.Match.View]
			units := []string{s}
			if r.Match.Unit == ruleset.UnitLine {
				units = strings.Split(s, "\n")
			}
			for _, u := range units {
				if r.MatchUnit(u) != nil {
					checked++
					if !r.MayMatch(u) || !r.MayMatch(s) {
						t.Errorf("%s: pre-filter rejects a matching unit %q", r.ID, u)
					}
				}
			}
		}
	}
	if checked < 100 {
		t.Fatalf("only %d matching units checked", checked)
	}
	t.Logf("%d matching units, none rejected by the pre-filter", checked)
}
