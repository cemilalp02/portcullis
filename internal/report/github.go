package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/cemilalp02/portcullis/internal/engine"
	"github.com/cemilalp02/portcullis/internal/finding"
)

// GitHub Actions workflow-command annotations. They show findings on the
// pull request diff without GitHub Advanced Security, also for private
// repositories and pull requests from forks.

// ghData escapes a workflow-command message. Newlines are encoded, so text
// taken from a scanned file can never start a new workflow command.
func ghData(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(s)
}

// ghProperty escapes a workflow-command property value.
func ghProperty(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C").Replace(s)
}

func ghLevel(s finding.Severity) string {
	switch {
	case s >= finding.High:
		return "error"
	case s == finding.Medium:
		return "warning"
	}
	return "notice"
}

// WriteGitHub writes one annotation per finding (info findings are left to
// the summary) and a closing summary line.
func WriteGitHub(w io.Writer, res *engine.Result, m Meta) error {
	var b strings.Builder
	for _, f := range res.Findings {
		if f.Severity == finding.Info {
			continue
		}
		fmt.Fprintf(&b, "::%s file=%s,line=%d,col=%d,endLine=%d,endColumn=%d,title=%s::%s\n",
			ghLevel(f.Severity), ghProperty(repoPath(m, f.Path)), f.Line, f.Column, max(f.EndLine, f.Line), max(f.EndColumn, f.Column),
			ghProperty(f.RuleID+" "+f.Title), ghData(f.Message+"\nFix: "+f.Remediation))
	}
	for _, e := range res.Errors {
		fmt.Fprintf(&b, "::error title=Portcullis analysis incomplete::%s\n", ghData(e))
	}
	b.WriteString(summaryLine(res, m) + "\n")
	_, err := io.WriteString(w, b.String())
	return err
}
