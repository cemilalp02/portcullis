// Package extract finds the commands a configuration file makes run without
// asking: Claude Code hooks, VS Code folder-open tasks, dev container
// lifecycle commands, npm install scripts and MCP server launchers
// (ARCHITECTURE §5.5).
package extract

import (
	"slices"
	"strings"

	"github.com/cemilalp02/portcullis/internal/jsonc"
)

// Kind classifies an execution point.
type Kind string

// Execution point kinds (docs/RULE_SCHEMA.md `kinds`).
const (
	ClaudeHook   Kind = "claude_hook"
	VSCodeTask   Kind = "vscode_task"
	DevContainer Kind = "devcontainer"
	NPMScript    Kind = "npm_script"
	MCPServer    Kind = "mcp_server"
)

// Kinds lists all execution point kinds.
var Kinds = []Kind{ClaudeHook, VSCodeTask, DevContainer, NPMScript, MCPServer}

// ExecPoint is one command that runs automatically.
type ExecPoint struct {
	Kind    Kind
	Trigger string // when it runs, e.g. "SessionStart", "folderOpen", "postinstall"
	Name    string // hook matcher, task label, script or server name
	Command string // the command line as the consumer would run it
	Start   int    // byte span of the command in the source
	End     int
}

// Func extracts execution points from a parsed configuration.
type Func func(root *jsonc.Node) []ExecPoint

var registry = map[string]Func{
	"claude_settings": claudeSettings,
	"vscode_tasks":    vscodeTasks,
	"devcontainer":    devcontainer,
	"npm_package":     npmPackage,
	"mcp_config":      mcpConfig,
}

// Lookup returns the extractor registered under name.
func Lookup(name string) (Func, bool) {
	f, ok := registry[name]
	return f, ok
}

// Names returns the registered extractor names, sorted.
func Names() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}

func point(k Kind, trigger, name string, n *jsonc.Node, cmd string) ExecPoint {
	return ExecPoint{Kind: k, Trigger: trigger, Name: name, Command: cmd, Start: n.Start, End: n.End}
}

// commandLine renders a string, or an array of strings/{value} objects, as a
// single command line. ok is false for any other shape.
func commandLine(n *jsonc.Node) (string, bool) {
	if s, ok := n.StringValue(); ok {
		return s, true
	}
	if n == nil || n.Kind != jsonc.Array {
		return "", false
	}
	parts := make([]string, 0, len(n.Elems))
	for _, e := range n.Elems {
		switch e.Kind {
		case jsonc.String:
			parts = append(parts, e.Str)
		case jsonc.Object:
			if v, ok := e.Get("value").StringValue(); ok { // VS Code quoted args
				parts = append(parts, v)
			}
		}
	}
	return strings.Join(parts, " "), true
}

// claudeSettings: hooks.<Event>[].hooks[].command plus top-level settings
// whose value is a command Claude Code runs on its own.
func claudeSettings(root *jsonc.Node) []ExecPoint {
	var out []ExecPoint
	if hooks := root.Get("hooks"); hooks != nil && hooks.Kind == jsonc.Object {
		for _, ev := range hooks.Members {
			if ev.Value.Kind != jsonc.Array {
				continue
			}
			for _, group := range ev.Value.Elems {
				matcher, _ := group.Get("matcher").StringValue()
				list := group.Get("hooks")
				if list == nil || list.Kind != jsonc.Array {
					continue
				}
				for _, h := range list.Elems {
					if c := h.Get("command"); c != nil {
						if cmd, ok := c.StringValue(); ok {
							out = append(out, point(ClaudeHook, ev.Key, matcher, c, cmd))
						}
					}
				}
			}
		}
	}
	if sl := root.Get("statusLine"); sl != nil {
		if c := sl.Get("command"); c != nil {
			if cmd, ok := c.StringValue(); ok {
				out = append(out, point(ClaudeHook, "statusLine", "", c, cmd))
			}
		}
	}
	for _, key := range []string{"apiKeyHelper", "awsAuthRefresh", "awsCredentialExport", "otelHeadersHelper"} {
		if c := root.Get(key); c != nil {
			if cmd, ok := c.StringValue(); ok {
				out = append(out, point(ClaudeHook, key, "", c, cmd))
			}
		}
	}
	return out
}

// vscodeTasks: tasks with runOptions.runOn == "folderOpen", including the
// platform-specific variants (windows / linux / osx) that override command
// and args.
func vscodeTasks(root *jsonc.Node) []ExecPoint {
	tasks := root.Get("tasks")
	if tasks == nil || tasks.Kind != jsonc.Array {
		return nil
	}
	var out []ExecPoint
	for _, t := range tasks.Elems {
		if on, _ := t.Get("runOptions").Get("runOn").StringValue(); on != "folderOpen" {
			continue
		}
		label, _ := t.Get("label").StringValue()
		for _, v := range []*jsonc.Node{t, t.Get("windows"), t.Get("linux"), t.Get("osx")} {
			c := v.Get("command")
			if c == nil {
				continue
			}
			cmd, ok := commandLine(c)
			if !ok {
				continue
			}
			if args, ok := commandLine(v.Get("args")); ok && args != "" {
				cmd += " " + args
			}
			out = append(out, point(VSCodeTask, "folderOpen", label, c, cmd))
		}
	}
	return out
}

// devcontainer lifecycle commands; initializeCommand runs on the host.
func devcontainer(root *jsonc.Node) []ExecPoint {
	keys := []struct{ key, trigger string }{
		{"initializeCommand", "initializeCommand (runs on host)"},
		{"onCreateCommand", "onCreateCommand"},
		{"updateContentCommand", "updateContentCommand"},
		{"postCreateCommand", "postCreateCommand"},
		{"postStartCommand", "postStartCommand"},
		{"postAttachCommand", "postAttachCommand"},
	}
	var out []ExecPoint
	for _, k := range keys {
		v := root.Get(k.key)
		if v == nil {
			continue
		}
		if v.Kind == jsonc.Object { // parallel commands: {"name": "cmd" | ["cmd", "arg"]}
			for _, m := range v.Members {
				if cmd, ok := commandLine(m.Value); ok {
					out = append(out, point(DevContainer, k.trigger, m.Key, m.Value, cmd))
				}
			}
			continue
		}
		if cmd, ok := commandLine(v); ok {
			out = append(out, point(DevContainer, k.trigger, "", v, cmd))
		}
	}
	return out
}

// npm scripts that run on `npm install` / `npm ci` without being asked for.
var npmInstallScripts = []string{"preinstall", "install", "postinstall", "prepublish", "preprepare", "prepare", "postprepare"}

func npmPackage(root *jsonc.Node) []ExecPoint {
	scripts := root.Get("scripts")
	if scripts == nil || scripts.Kind != jsonc.Object {
		return nil
	}
	var out []ExecPoint
	for _, m := range scripts.Members {
		if !slices.Contains(npmInstallScripts, m.Key) {
			continue
		}
		if cmd, ok := m.Value.StringValue(); ok {
			out = append(out, point(NPMScript, m.Key, m.Key, m.Value, cmd))
		}
	}
	return out
}

// mcpConfig: "mcpServers" (Claude, Cursor) or "servers" (VS Code). Local
// servers are launched as command + args; remote ones are reported by URL.
func mcpConfig(root *jsonc.Node) []ExecPoint {
	var out []ExecPoint
	for _, key := range []string{"mcpServers", "servers"} {
		servers := root.Get(key)
		if servers == nil || servers.Kind != jsonc.Object {
			continue
		}
		for _, m := range servers.Members {
			s := m.Value
			if c := s.Get("command"); c != nil {
				if cmd, ok := c.StringValue(); ok {
					if args, ok := commandLine(s.Get("args")); ok && args != "" {
						cmd += " " + args
					}
					out = append(out, point(MCPServer, "server start", m.Key, c, cmd))
					continue
				}
			}
			if u := s.Get("url"); u != nil {
				if url, ok := u.StringValue(); ok {
					out = append(out, point(MCPServer, "remote server", m.Key, u, url))
				}
			}
		}
	}
	return out
}
