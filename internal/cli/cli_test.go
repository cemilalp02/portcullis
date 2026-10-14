package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cemilalp02/portcullis/internal/version"
)

func run(args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = Run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string // substring; "" means stdout must be empty
		wantStderr string // substring; "" means stderr must be empty
	}{
		{name: "no args prints usage to stderr", args: nil, wantCode: ExitError, wantStderr: "Usage:"},
		{name: "help", args: []string{"help"}, wantCode: ExitOK, wantStdout: "version"},
		{name: "--help", args: []string{"--help"}, wantCode: ExitOK, wantStdout: "Usage:"},
		{name: "unknown command", args: []string{"frobnicate"}, wantCode: ExitError, wantStderr: `unknown command "frobnicate"`},
		{name: "version text", args: []string{"version"}, wantCode: ExitOK, wantStdout: "portcullis "},
		{name: "--version alias", args: []string{"--version"}, wantCode: ExitOK, wantStdout: "ruleset:"},
		{name: "version bad format", args: []string{"version", "--format", "xml"}, wantCode: ExitError, wantStderr: `unknown format "xml"`},
		{name: "version unexpected arg", args: []string{"version", "extra"}, wantCode: ExitError, wantStderr: `unexpected argument "extra"`},
		{name: "version unknown flag", args: []string{"version", "--nope"}, wantCode: ExitError, wantStderr: "flag provided but not defined"},
		{name: "version -h", args: []string{"version", "-h"}, wantCode: ExitOK, wantStderr: "-format"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := run(tt.args...)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d", code, tt.wantCode)
			}
			checkStream(t, "stdout", stdout, tt.wantStdout)
			checkStream(t, "stderr", stderr, tt.wantStderr)
		})
	}
}

func checkStream(t *testing.T, name, got, want string) {
	t.Helper()
	if want == "" && got != "" {
		t.Errorf("%s = %q, want empty", name, got)
	}
	if !strings.Contains(got, want) {
		t.Errorf("%s = %q, want substring %q", name, got, want)
	}
}

func TestVersionJSON(t *testing.T) {
	code, stdout, stderr := run("version", "--format", "json")
	if code != ExitOK || stderr != "" {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr)
	}
	var got version.Info
	dec := json.NewDecoder(strings.NewReader(stdout))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("invalid JSON %q: %v", stdout, err)
	}
	if got != version.Get() {
		t.Errorf("got %+v, want %+v", got, version.Get())
	}
}
