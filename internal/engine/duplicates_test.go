package engine

import (
	"testing"
)

func TestDuplicateKeysBySetting(t *testing.T) {
	e := defaultEngine(t)
	pkg := e.Targets.Match("package.json")
	tests := []struct {
		name, src, want string
	}{
		{"script hidden by a second scripts object", `{"scripts": {"test": "jest"}, "scripts": {"postinstall": "node x.js"}}`, RuleDuplicateKey},
		{"repeated script name", `{"scripts": {"test": "jest", "test": "node x.js"}}`, RuleDuplicateKey},
		{"repeated dependency", `{"dependencies": {"left-pad": "1.3.0", "left-pad": "file:../x"}}`, RuleDuplicateKey},
		{"metadata", `{"name": "x", "nonNpm": true, "nonNpm": true}`, RuleDuplicateInert},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := e.Analyze("package.json", pkg, []byte(tt.src))
			if len(byRule(got, tt.want)) != 1 {
				t.Errorf("want one %s, got %+v", tt.want, got)
			}
		})
	}
}
