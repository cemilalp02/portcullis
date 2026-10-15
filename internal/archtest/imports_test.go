// Package archtest enforces architectural rules that the compiler cannot.
package archtest

import (
	"os/exec"
	"strings"
	"testing"
)

// forbidden lists packages that must never be linked into the portcullis binary,
// directly or transitively (ADR-0006: no processes, no network). A prefix also
// forbids its sub-packages.
var forbidden = []string{"os/exec", "net", "plugin"}

// allowed are pure parsing packages under net/ that open no sockets and
// resolve no names; html/template needs net/url to escape URLs. The socket
// package net itself stays forbidden, and the check is transitive, so neither
// can bring it in.
var allowed = map[string]bool{"net/url": true, "net/netip": true}

func TestBinaryHasNoProcessOrNetworkDependencies(t *testing.T) {
	if testing.Short() {
		t.Skip("runs 'go list'")
	}
	out, err := exec.Command("go", "list", "-deps", "github.com/cemilalp02/portcullis/cmd/portcullis").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	deps := strings.Fields(string(out))
	if len(deps) == 0 {
		t.Fatal("go list returned no dependencies")
	}
	for _, dep := range deps {
		if allowed[dep] {
			continue
		}
		for _, bad := range forbidden {
			if dep == bad || strings.HasPrefix(dep, bad+"/") {
				t.Errorf("portcullis binary depends on %q (forbidden by ADR-0006)", dep)
			}
		}
	}
}
