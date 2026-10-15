package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/cemilalp02/portcullis/internal/engine"
	"github.com/cemilalp02/portcullis/internal/finding"
	"github.com/cemilalp02/portcullis/internal/ruleset"
)

// SARIF 2.1.0 output for GitHub code scanning and other SARIF consumers
// (docs/CLI.md §6).

const (
	sarifSchema  = "https://json.schemastore.org/sarif-2.1.0.json"
	sarifVersion = "2.1.0"
	repoURL      = "https://github.com/cemilalp02/portcullis"
	rulesDocURL  = repoURL + "/blob/main/docs/RULES.md#"
)

type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool        sarifTool         `json:"tool"`
	Invocations []sarifInvocation `json:"invocations"`
	ColumnKind  string            `json:"columnKind"`
	Results     []sarifResult     `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version"`
	InformationURI string      `json:"informationUri"`
	Rules          []sarifRule `json:"rules"`
}

type sarifText struct {
	Text     string `json:"text"`
	Markdown string `json:"markdown,omitempty"`
}

type sarifRule struct {
	ID                   string         `json:"id"`
	Name                 string         `json:"name"`
	ShortDescription     sarifText      `json:"shortDescription"`
	FullDescription      sarifText      `json:"fullDescription"`
	Help                 sarifText      `json:"help"`
	HelpURI              string         `json:"helpUri,omitempty"`
	DefaultConfiguration sarifRuleLevel `json:"defaultConfiguration"`
	Properties           map[string]any `json:"properties"`
}

type sarifRuleLevel struct {
	Level string `json:"level"`
}

type sarifInvocation struct {
	ExecutionSuccessful        bool                `json:"executionSuccessful"`
	ToolExecutionNotifications []sarifNotification `json:"toolExecutionNotifications,omitempty"`
}

type sarifNotification struct {
	Level   string    `json:"level"`
	Message sarifText `json:"message"`
}

type sarifResult struct {
	RuleID              string            `json:"ruleId"`
	RuleIndex           int               `json:"ruleIndex"`
	Level               string            `json:"level"`
	Message             sarifText         `json:"message"`
	Locations           []sarifLocation   `json:"locations"`
	PartialFingerprints map[string]string `json:"partialFingerprints"`
	Properties          map[string]any    `json:"properties"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysical `json:"physicalLocation"`
}

type sarifPhysical struct {
	ArtifactLocation sarifArtifact `json:"artifactLocation"`
	Region           sarifRegion   `json:"region"`
}

type sarifArtifact struct {
	URI       string `json:"uri"`
	URIBaseID string `json:"uriBaseId"`
}

type sarifRegion struct {
	StartLine   int        `json:"startLine"`
	StartColumn int        `json:"startColumn"`
	EndLine     int        `json:"endLine"`
	EndColumn   int        `json:"endColumn"`
	Snippet     *sarifText `json:"snippet,omitempty"`
}

// sarifLevel maps severities to SARIF levels.
func sarifLevel(s finding.Severity) string {
	switch {
	case s >= finding.High:
		return "error"
	case s == finding.Medium:
		return "warning"
	}
	return "note"
}

// securitySeverity drives GitHub's Critical/High/Medium/Low labels. Info
// findings carry none, so GitHub lists them as notes, not vulnerabilities.
var securitySeverity = map[finding.Severity]string{
	finding.Critical: "9.5", finding.High: "8.0", finding.Medium: "5.5", finding.Low: "2.0",
}

// ruleName turns a title into a PascalCase SARIF rule name.
func ruleName(title string) string {
	var b strings.Builder
	for _, w := range strings.FieldsFunc(title, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		r := []rune(w)
		if r[0] < unicode.MaxASCII {
			b.WriteRune(unicode.ToUpper(r[0]))
			b.WriteString(string(r[1:]))
		}
	}
	return b.String()
}

// uriPath percent-encodes a '/'-separated relative path for a SARIF URI
// (RFC 3986), keeping '/' and unreserved characters.
func uriPath(p string) string {
	var b strings.Builder
	for _, c := range []byte(p) {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', strings.IndexByte("-._~/", c) >= 0:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// repoPath prefixes a finding path with Meta.PathPrefix (the scan root
// relative to the repository root).
func repoPath(m Meta, p string) string {
	prefix := strings.Trim(m.PathPrefix, "/")
	if prefix == "" || prefix == "." {
		return p
	}
	return prefix + "/" + p
}

// WriteSARIF writes a SARIF 2.1.0 log. Rule metadata comes from m.Rules.
func WriteSARIF(w io.Writer, res *engine.Result, m Meta) error {
	index := map[string]int{}
	driver := sarifDriver{Name: "Portcullis", Version: m.Version, InformationURI: repoURL, Rules: []sarifRule{}}
	for _, r := range m.Rules {
		index[r.ID] = len(driver.Rules)
		driver.Rules = append(driver.Rules, sarifRuleFor(r))
	}
	run := sarifRun{
		Tool:        sarifTool{Driver: driver},
		Invocations: []sarifInvocation{{ExecutionSuccessful: res.Complete}},
		ColumnKind:  "unicodeCodePoints",
		Results:     []sarifResult{},
	}
	for _, e := range res.Errors {
		run.Invocations[0].ToolExecutionNotifications = append(run.Invocations[0].ToolExecutionNotifications,
			sarifNotification{Level: "error", Message: sarifText{Text: e}})
	}
	for _, f := range res.Findings {
		i, ok := index[f.RuleID]
		if !ok {
			return fmt.Errorf("sarif: no metadata for rule %s", f.RuleID)
		}
		endCol := f.EndColumn
		if f.EndLine < f.Line || (f.EndLine == f.Line && endCol <= f.Column) {
			endCol = f.Column + 1 // SARIF regions must not be empty or inverted
		}
		region := sarifRegion{StartLine: f.Line, StartColumn: f.Column, EndLine: max(f.EndLine, f.Line), EndColumn: endCol}
		if f.Excerpt != "" {
			region.Snippet = &sarifText{Text: f.Excerpt}
		}
		run.Results = append(run.Results, sarifResult{
			RuleID:    f.RuleID,
			RuleIndex: i,
			Level:     sarifLevel(f.Severity),
			Message:   sarifText{Text: f.Message},
			Locations: []sarifLocation{{PhysicalLocation: sarifPhysical{
				ArtifactLocation: sarifArtifact{URI: uriPath(repoPath(m, f.Path)), URIBaseID: "%SRCROOT%"},
				Region:           region,
			}}},
			PartialFingerprints: map[string]string{"portcullis/v1": f.Fingerprint},
			Properties:          map[string]any{"severity": f.Severity.String(), "confidence": f.Confidence.String()},
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(sarifLog{Schema: sarifSchema, Version: sarifVersion, Runs: []sarifRun{run}})
}

func sarifRuleFor(r *ruleset.Compiled) sarifRule {
	oneLine := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	tags := append([]string{"security", "ai-agents"}, r.OWASP...)
	props := map[string]any{"tags": tags, "precision": r.Confidence.String()}
	if s, ok := securitySeverity[r.Severity]; ok {
		props["security-severity"] = s
	}
	rule := sarifRule{
		ID:                   r.ID,
		Name:                 ruleName(r.Title),
		ShortDescription:     sarifText{Text: r.Title},
		FullDescription:      sarifText{Text: oneLine(r.Description)},
		Help:                 sarifText{Text: oneLine(r.Remediation), Markdown: "**Fix:** " + oneLine(r.Remediation)},
		DefaultConfiguration: sarifRuleLevel{Level: sarifLevel(r.Severity)},
		Properties:           props,
	}
	if strings.HasPrefix(r.ID, "PCL-") {
		rule.HelpURI = rulesDocURL + strings.ToLower(r.ID)
		rule.Help.Markdown += fmt.Sprintf("\n\n[Rule documentation](%s)", rule.HelpURI)
	}
	return rule
}
