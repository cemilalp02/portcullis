// Package archtest enforces architectural rules that the compiler cannot.
package archtest

import (
	"os/exec"
	"strings"
	"testing"
)

// forbidden lists packages that must never be linked into the portcullis binary,
// directly or transitively (ADR-0006: no processes, no network).
var forbidden = []string{"os/exec", "net", "plugin"}

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
		for _, bad := range forbidden {
			if dep == bad || strings.HasPrefix(dep, bad+"/") {
				t.Errorf("portcullis binary depends on %q (forbidden by ADR-0006)", dep)
			}
		}
	}
}
