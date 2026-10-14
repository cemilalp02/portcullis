package target

import (
	"strings"
	"testing"

	"github.com/cemilalp02/portcullis/rules"
)

func loadDefault(t *testing.T) *Set {
	t.Helper()
	data, err := rules.FS.ReadFile("targets.yaml")
	if err != nil {
		t.Fatal(err)
	}
	s, err := Load(data)
	if err != nil {
		t.Fatalf("Load(targets.yaml): %v", err)
	}
	return s
}

func TestDefaultTargets(t *testing.T) {
	s := loadDefault(t)
	if n := len(s.Defs()); n != 13 {
		t.Errorf("got %d target types, want 13 (REQUIREMENTS §5)", n)
	}
	tests := []struct {
		path string
		want string // target ID, "" for no match
	}{
		{"CLAUDE.md", "claude-memory"},
		{"claude.md", "claude-memory"}, // case-insensitive (THREAT_MODEL S13)
		{"packages/api/CLAUDE.local.md", "claude-memory"},
		{"AGENTS.md", "agents-md"},
		{"docs/GEMINI.md", "gemini-md"},
		{"skills/pdf/SKILL.md", "skill"},
		{".claude/commands/deploy.md", "claude-commands"},
		{".claude/agents/reviewer/agent.md", "claude-commands"},
		{".cursorrules", "cursor-rules"},
		{".cursor/rules/style.mdc", "cursor-rules"},
		{".github/copilot-instructions.md", "copilot-instructions"},
		{".github/instructions/go.instructions.md", "copilot-instructions"},
		{".github/prompts/review.prompt.md", "copilot-instructions"},
		{".windsurfrules", "other-agent-rules"},
		{".clinerules/01-style.md", "other-agent-rules"},
		{".claude/settings.json", "claude-settings"},
		{".claude/settings.local.json", "claude-settings"},
		{".vscode/tasks.json", "vscode-tasks"},
		{".devcontainer/devcontainer.json", "devcontainer"},
		{".devcontainer.json", "devcontainer"},
		{".devcontainer/python/devcontainer.json", "devcontainer"},
		{".mcp.json", "mcp-config"},
		{".vscode/mcp.json", "mcp-config"},
		{".cursor/mcp.json", "mcp-config"},
		{"package.json", "npm-manifest"},
		{"web/package.json", "npm-manifest"},
		{"README.md", ""},
		{".vscode/settings.json", ""},
		{"CLAUDE.md.bak", ""},
		{"src/claude.go", ""},
		{".github/workflows/ci.yml", ""},
	}
	for _, tt := range tests {
		got := ""
		if d := s.Match(tt.path); d != nil {
			got = d.ID
		}
		if got != tt.want {
			t.Errorf("Match(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
	if s.Get("skill") == nil || s.Get("nope") != nil {
		t.Error("Get")
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pat, path string
		want      bool
	}{
		{"**/a.md", "a.md", true},
		{"**/a.md", "x/y/a.md", true},
		{"x/**", "x/y/z", true},
		{"x/**", "x", true},
		{"x/**/*.md", "x/a.md", true},
		{"x/**/*.md", "x/a/b.md", true},
		{"x/**/*.md", "x/a/b.txt", false},
		{"x/*/c", "x/b/c", true},
		{"x/*/c", "x/b/b/c", false},
		{"a", "a/b", false},
		{"A.MD", "a.md", true},
	}
	for _, tt := range tests {
		if got := MatchPattern(tt.pat, tt.path); got != tt.want {
			t.Errorf("MatchPattern(%q, %q) = %v, want %v", tt.pat, tt.path, got, tt.want)
		}
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct {
		name, yaml, want string
	}{
		{"unknown field", "schema: 1\ntargets:\n  - id: a\n    title: A\n    family: instruction\n    patterns: [a]\n    bogus: 1\n", "field bogus not found"},
		{"bad schema", "schema: 2\ntargets: []\n", "unsupported schema"},
		{"empty", "schema: 1\ntargets: []\n", "no targets"},
		{"bad family", "schema: 1\ntargets:\n  - {id: a, title: A, family: x, patterns: [a]}\n", "unknown family"},
		{"config without extractor", "schema: 1\ntargets:\n  - {id: a, title: A, family: config, format: jsonc, patterns: [a]}\n", "need format jsonc and an extractor"},
		{"instruction with extractor", "schema: 1\ntargets:\n  - {id: a, title: A, family: instruction, extractor: x, patterns: [a]}\n", "plain text"},
		{"no patterns", "schema: 1\ntargets:\n  - {id: a, title: A, family: instruction}\n", "pattern is required"},
		{"absolute pattern", "schema: 1\ntargets:\n  - {id: a, title: A, family: instruction, patterns: [/etc/x]}\n", "relative"},
		{"dotdot pattern", "schema: 1\ntargets:\n  - {id: a, title: A, family: instruction, patterns: [../x]}\n", "'..'"},
		{"bad glob", "schema: 1\ntargets:\n  - {id: a, title: A, family: instruction, patterns: ['[']}\n", "invalid pattern"},
		{"missing id", "schema: 1\ntargets:\n  - {title: A, family: instruction, patterns: [a]}\n", "required"},
		{"duplicate", "schema: 1\ntargets:\n  - {id: a, title: A, family: instruction, patterns: [a]}\n  - {id: a, title: B, family: instruction, patterns: [b]}\n", "duplicate id"},
		{"not yaml", "{{", "targets:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load([]byte(tt.yaml))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Load error = %v, want containing %q", err, tt.want)
			}
		})
	}
}
