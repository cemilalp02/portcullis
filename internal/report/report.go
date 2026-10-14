// Package report renders scan results as text for people and JSON for
// machines (docs/CLI.md §4-§5). It never prints anything itself; callers
// pass the writer.
package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/cemilalp02/portcullis/internal/engine"
	"github.com/cemilalp02/portcullis/internal/escape"
	"github.com/cemilalp02/portcullis/internal/finding"
)

// Exit codes (docs/CLI.md §2).
const (
	ExitOK       = 0
	ExitFindings = 1
	ExitError    = 2
)

// SchemaURL identifies the JSON report format.
const SchemaURL = "https://github.com/cemilalp02/portcullis/schemas/report-v1.json"

// Meta describes the run that produced a result.
type Meta struct {
	Version   string
	Ruleset   string
	Mode      string           // "untrusted" or "trusted"
	FailOn    finding.Severity // threshold for exit code 1
	NoFail    bool             // --fail-on none
	Inventory bool             // show the auto-execution inventory in text output
}

// Above returns how many findings are at or above the threshold.
func Above(res *engine.Result, m Meta) int {
	if m.NoFail {
		return 0
	}
	n := 0
	for _, f := range res.Findings {
		if f.Severity >= m.FailOn {
			n++
		}
	}
	return n
}

// ExitCode returns the process exit code for a result: incomplete analysis
// always wins (fail closed), then findings at or above the threshold.
func ExitCode(res *engine.Result, m Meta) int {
	switch {
	case !res.Complete:
		return ExitError
	case Above(res, m) > 0:
		return ExitFindings
	}
	return ExitOK
}

// WriteText writes the human-readable report.
func WriteText(w io.Writer, res *engine.Result, m Meta) error {
	var b strings.Builder
	fmt.Fprintf(&b, "portcullis %s · ruleset %s · mode: %s · %d agent files in %d files\n\n",
		m.Version, m.Ruleset, m.Mode, res.Targets, res.Files)

	if len(res.Findings) == 0 {
		b.WriteString("No findings.\n\n")
	}
	for _, f := range res.Findings {
		fmt.Fprintf(&b, "%-9s %s  %s:%d:%d\n", strings.ToUpper(f.Severity.String()), f.RuleID, escape.String(f.Path, 0), f.Line, f.Column)
		fmt.Fprintf(&b, "          %s\n", f.Message)
		if f.Excerpt != "" {
			fmt.Fprintf(&b, "          > %s\n", f.Excerpt)
		}
		fmt.Fprintf(&b, "          fix: %s\n\n", f.Remediation)
	}

	if m.Inventory && len(res.Inventory) > 0 {
		b.WriteString("Auto-execution inventory (runs without asking):\n")
		var wk, wt, wl int
		locs := make([]string, len(res.Inventory))
		for i, it := range res.Inventory {
			locs[i] = fmt.Sprintf("%s:%d", it.Path, it.Line)
			wk, wt, wl = max(wk, len(it.Kind)), max(wt, utf8.RuneCountInString(it.Trigger)), max(wl, utf8.RuneCountInString(locs[i]))
		}
		for i, it := range res.Inventory {
			fmt.Fprintf(&b, "  %-*s  %-*s  %-*s  %s\n", wk, it.Kind, wt, it.Trigger, wl, locs[i], it.Command)
		}
		b.WriteString("\n")
	}

	if !res.Complete {
		b.WriteString("Analysis incomplete. These files could not be analysed:\n")
		for _, e := range res.Errors {
			fmt.Fprintf(&b, "  %s\n", e)
		}
		b.WriteString("\n")
	}

	above := Above(res, m)
	threshold := "'" + m.FailOn.String() + "'"
	if m.NoFail {
		threshold = "the threshold (none)"
	}
	fmt.Fprintf(&b, "%d %s at or above %s%s · %d below · exit %d\n",
		above, plural(above, "finding", "findings"), threshold, breakdown(res, m), len(res.Findings)-above, ExitCode(res, m))
	_, err := io.WriteString(w, b.String())
	return err
}

func breakdown(res *engine.Result, m Meta) string {
	if m.NoFail {
		return ""
	}
	counts := map[finding.Severity]int{}
	for _, f := range res.Findings {
		if f.Severity >= m.FailOn {
			counts[f.Severity]++
		}
	}
	var parts []string
	for s := finding.Critical; s >= finding.Info; s-- {
		if counts[s] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[s], s))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

type jsonReport struct {
	Schema    string                 `json:"schema"`
	Tool      jsonTool               `json:"tool"`
	Mode      string                 `json:"mode"`
	FailOn    string                 `json:"failOn"`
	Complete  bool                   `json:"complete"`
	ExitCode  int                    `json:"exitCode"`
	Stats     jsonStats              `json:"stats"`
	Findings  []finding.Finding      `json:"findings"`
	Inventory []engine.InventoryItem `json:"inventory"`
	Errors    []string               `json:"errors"`
}

type jsonTool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Ruleset string `json:"ruleset"`
}

type jsonStats struct {
	Files          int `json:"files"`
	Targets        int `json:"targets"`
	Findings       int `json:"findings"`
	AboveThreshold int `json:"aboveThreshold"`
}

// WriteJSON writes the machine-readable report.
func WriteJSON(w io.Writer, res *engine.Result, m Meta) error {
	failOn := m.FailOn.String()
	if m.NoFail {
		failOn = "none"
	}
	r := jsonReport{
		Schema:    SchemaURL,
		Tool:      jsonTool{Name: "portcullis", Version: m.Version, Ruleset: m.Ruleset},
		Mode:      m.Mode,
		FailOn:    failOn,
		Complete:  res.Complete,
		ExitCode:  ExitCode(res, m),
		Stats:     jsonStats{Files: res.Files, Targets: res.Targets, Findings: len(res.Findings), AboveThreshold: Above(res, m)},
		Findings:  nonNil(res.Findings),
		Inventory: nonNil(res.Inventory),
		Errors:    nonNil(res.Errors),
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		return err
	}
	_, err := w.Write(escapeHidden(buf.Bytes()))
	return err
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// escapeHidden rewrites non-ASCII characters that are not visible (bidi
// controls, zero-width, Tag characters, …) as \uXXXX escapes. Such
// characters only occur inside JSON strings, so the document stays valid and
// decodes to the same values, but viewing the raw file in a terminal can no
// longer be spoofed (FR-34).
func escapeHidden(b []byte) []byte {
	var out bytes.Buffer
	out.Grow(len(b))
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		if r >= utf8.RuneSelf && !escape.Visible(r) {
			if r1, r2 := utf16.EncodeRune(r); r1 != utf8.RuneError {
				fmt.Fprintf(&out, `\u%04x\u%04x`, r1, r2)
			} else {
				fmt.Fprintf(&out, `\u%04x`, r)
			}
		} else {
			out.Write(b[:size])
		}
		b = b[size:]
	}
	return out.Bytes()
}
