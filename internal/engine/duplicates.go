package engine

import (
	"strings"

	"github.com/cemilalp02/portcullis/internal/jsonc"
)

// executingKeys are the configuration keys that decide what runs. A key
// repeated there, or anywhere below them, can show reviewers one command
// while a tool runs the other.
var executingKeys = map[string]bool{
	"scripts": true, "hooks": true, "command": true, "commands": true, "args": true, "statusline": true,
	"apikeyhelper": true, "env": true, "permissions": true, "allow": true, "deny": true, "defaultmode": true,
	"enableallprojectmcpservers": true, "mcpservers": true, "servers": true, "url": true,
	"tasks": true, "runoptions": true, "runon": true, "dependson": true, "label": true,
	"windows": true, "linux": true, "osx": true,
	"initializecommand": true, "oncreatecommand": true, "updatecontentcommand": true,
	"postcreatecommand": true, "poststartcommand": true, "postattachcommand": true,
	"dependencies": true, "devdependencies": true, "optionaldependencies": true, "overrides": true,
	"resolutions": true, "bin": true, "main": true, "exports": true,
}

// executes reports whether a repeated key, or one of the keys above it,
// decides what runs.
func executes(key string, path []string) bool {
	if executingKeys[strings.ToLower(key)] {
		return true
	}
	for _, k := range path {
		if executingKeys[strings.ToLower(k)] {
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
