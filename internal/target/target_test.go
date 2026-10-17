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
	if n := len(s.Defs()); n != 18 {
		t.Errorf("got %d target types, want 18 (REQUIREMENTS §5: 16, plus the two skill companions)", n)
	}
	comp := s.Companions("skill")
	if len(comp) != 2 || comp[0].ID != "skill-script" || comp[0].Family != Script || comp[1].ID != "skill-doc" || comp[1].Family != Instruction {
		t.Fatalf("skill companions = %+v", comp)
	}
	if !comp[0].HasExtension("scripts/RUN.PY") || comp[0].HasExtension("logo.png") {
		t.Error("HasExtension")
	}
	if s.Match("skills/pdf/scripts/run.py") != nil {
		t.Error("companion targets must not match by pattern")
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
		{"apps/web/.vscode/tasks.json", "vscode-tasks"}, // subfolder opened as a project
		{"services/api/.claude/settings.json", "claude-settings"},
		{"tools/.mcp.json", "mcp-config"},
		{".devcontainer/devcontainer.json", "devcontainer"},
		{".devcontainer.json", "devcontainer"},
		{".devcontainer/python/devcontainer.json", "devcontainer"},
		{".mcp.json", "mcp-config"},
		{".vscode/mcp.json", "mcp-config"},
		{".cursor/mcp.json", "mcp-config"},
		{"package.json", "npm-manifest"},
		{"web/package.json", "npm-manifest"},
		{"README.md", ""},
		{".vscode/settings.json", "vscode-settings"},
		{".vscode/launch.json", ""},
		{".gemini/settings.json", "gemini-settings"},
		{"tools/.gemini/commands/git/review.toml", "gemini-commands"},
		{".gemini/config.toml", ""},
		{".github/agents/reviewer.md", "copilot-instructions"},
		{".github/chatmodes/plan.chatmode.md", "copilot-instructions"},
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
		{"companion with patterns", "schema: 1\ntargets:\n  - {id: a, title: A, family: instruction, patterns: [a]}\n  - {id: b, title: B, family: script, companion_of: a, extensions: [.py], patterns: [x]}\n", "extensions instead of patterns"},
		{"companion without extensions", "schema: 1\ntargets:\n  - {id: a, title: A, family: instruction, patterns: [a]}\n  - {id: b, title: B, family: script, companion_of: a}\n", "extensions instead of patterns"},
		{"bad extension", "schema: 1\ntargets:\n  - {id: a, title: A, family: instruction, patterns: [a]}\n  - {id: b, title: B, family: script, companion_of: a, extensions: [py]}\n", "invalid extension"},
		{"unknown companion parent", "schema: 1\ntargets:\n  - {id: b, title: B, family: script, companion_of: nope, extensions: [.py]}\n", "not a pattern-based target"},
		{"extensions without companion", "schema: 1\ntargets:\n  - {id: a, title: A, family: instruction, patterns: [a], extensions: [.py]}\n", "only valid with companion_of"},
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
