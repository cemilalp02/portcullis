package cli

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite docs/RULES.md from the embedded rules")

// TestRulesDocUpToDate keeps docs/RULES.md in sync with the rule files.
func TestRulesDocUpToDate(t *testing.T) {
	code, stdout, stderr := run("rules", "list", "--format", "markdown")
	if code != ExitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	path := filepath.Join("..", "..", "docs", "RULES.md")
	if *update {
		if err := os.WriteFile(path, []byte(stdout), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run: go test ./internal/cli -run TestRulesDoc -update)", err)
	}
	if string(want) != stdout {
		t.Error("docs/RULES.md is out of date; run: go test ./internal/cli -run TestRulesDoc -update")
	}
}
