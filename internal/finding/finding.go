// Package finding defines the result model shared by the engine and reporters.
package finding

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// Severity reflects what an attacker gains (REQUIREMENTS §6.4).
type Severity int

// Severities in ascending order.
const (
	Info Severity = iota
	Low
	Medium
	High
	Critical
)

var severityNames = [...]string{"info", "low", "medium", "high", "critical"}

func (s Severity) String() string {
	if s < Info || s > Critical {
		return fmt.Sprintf("severity(%d)", int(s))
	}
	return severityNames[s]
}

// MarshalText implements encoding.TextMarshaler.
func (s Severity) MarshalText() ([]byte, error) {
	if s < Info || s > Critical {
		return nil, fmt.Errorf("invalid severity %d", int(s))
	}
	return []byte(s.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (s *Severity) UnmarshalText(b []byte) error {
	v, err := ParseSeverity(string(b))
	if err != nil {
		return err
	}
	*s = v
	return nil
}

// ParseSeverity parses a severity name (case-insensitive).
func ParseSeverity(name string) (Severity, error) {
	for i, n := range severityNames {
		if strings.EqualFold(name, n) {
			return Severity(i), nil
		}
	}
	return Info, fmt.Errorf("unknown severity %q (want one of %s)", name, strings.Join(severityNames[:], ", "))
}

// Confidence is how often a match of a rule is truly malicious.
type Confidence int

// Confidence levels in ascending order.
const (
	ConfidenceLow Confidence = iota
	ConfidenceMedium
	ConfidenceHigh
)

var confidenceNames = [...]string{"low", "medium", "high"}

func (c Confidence) String() string {
	if c < ConfidenceLow || c > ConfidenceHigh {
		return fmt.Sprintf("confidence(%d)", int(c))
	}
	return confidenceNames[c]
}

// MarshalText implements encoding.TextMarshaler.
func (c Confidence) MarshalText() ([]byte, error) {
	if c < ConfidenceLow || c > ConfidenceHigh {
		return nil, fmt.Errorf("invalid confidence %d", int(c))
	}
	return []byte(c.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (c *Confidence) UnmarshalText(b []byte) error {
	for i, n := range confidenceNames {
		if strings.EqualFold(string(b), n) {
			*c = Confidence(i)
			return nil
		}
	}
	return fmt.Errorf("unknown confidence %q (want low, medium or high)", b)
}

// Finding is one reported issue. Line and column are 1-based; columns count
// Unicode code points.
type Finding struct {
	RuleID      string     `json:"ruleId"`
	Title       string     `json:"title"`
	Severity    Severity   `json:"severity"`
	Confidence  Confidence `json:"confidence"`
	Path        string     `json:"path"`
	Line        int        `json:"line"`
	Column      int        `json:"column"`
	EndLine     int        `json:"endLine"`
	EndColumn   int        `json:"endColumn"`
	Excerpt     string     `json:"excerpt"`
	Message     string     `json:"message"`
	Remediation string     `json:"remediation"`
	OWASP       []string   `json:"owasp"`
	Fingerprint string     `json:"fingerprint"`
}

// Compare orders findings by severity (highest first), then path, position,
// rule ID and fingerprint, which makes output deterministic.
func Compare(a, b Finding) int {
	return cmp.Or(
		cmp.Compare(b.Severity, a.Severity),
		cmp.Compare(a.Path, b.Path),
		cmp.Compare(a.Line, b.Line),
		cmp.Compare(a.Column, b.Column),
		cmp.Compare(a.RuleID, b.RuleID),
		cmp.Compare(a.Fingerprint, b.Fingerprint),
	)
}

// Sort sorts findings in place using Compare.
func Sort(fs []Finding) { slices.SortFunc(fs, Compare) }
