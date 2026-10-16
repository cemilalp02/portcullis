package engine

import (
	"strings"

	"github.com/cemilalp02/portcullis/internal/jsonc"
)

// executingKeys are, per configuration format (the target's extractor), the
// keys that decide what runs. A key repeated there, or anywhere below them,
// can show reviewers one command while a tool runs the other. The sets are
// per format because the same name means different things: "commands" runs
// nothing in an extension manifest.
var executingKeys = map[string]map[string]bool{
	"claude_settings": keySet("hooks", "command", "statusline", "apikeyhelper", "env", "permissions", "allow", "deny",
		"defaultmode", "enableallprojectmcpservers", "mcpservers"),
	"vscode_tasks": keySet("tasks", "command", "args", "runoptions", "runon", "dependson", "label",
		"windows", "linux", "osx"),
	"devcontainer": keySet("initializecommand", "oncreatecommand", "updatecontentcommand", "postcreatecommand",
		"poststartcommand", "postattachcommand"),
	"mcp_config": keySet("mcpservers", "servers", "command", "args", "env", "url"),
	"npm_package": keySet("scripts", "dependencies", "devdependencies", "optionaldependencies", "overrides",
		"resolutions", "bin", "main", "exports"),
}

func keySet(keys ...string) map[string]bool {
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[k] = true
	}
	return m
}

// executes reports whether a repeated key, or one of the keys above it,
// decides what runs in a file of the given format.
func executes(format, key string, path []string) bool {
	keys := executingKeys[format]
	if keys[strings.ToLower(key)] {
		return true
	}
	for _, k := range path {
		if keys[strings.ToLower(k)] {
			return true
		}
	}
	return false
}

// keyPath returns the keys of the objects that enclose offset, outermost
// first.
func keyPath(n *jsonc.Node, offset int) []string {
	var path []string
	for n != nil {
		var next *jsonc.Node
		switch n.Kind {
		case jsonc.Object:
			for _, m := range n.Members {
				if m.Value != nil && m.Value.Start <= offset && offset < m.Value.End {
					path = append(path, m.Key)
					next = m.Value
					break
				}
			}
		case jsonc.Array:
			for _, e := range n.Elems {
				if e.Start <= offset && offset < e.End {
					next = e
					break
				}
			}
		}
		n = next
	}
	return path
}
