package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/cemilalp02/portcullis/internal/engine"
	"github.com/cemilalp02/portcullis/internal/finding"
)

// Markdown summary, e.g. for $GITHUB_STEP_SUMMARY.

// mdText makes scanned text inert in Markdown: HTML is escaped and every
// Markdown punctuation character is backslash-escaped, so a file cannot turn
// its content into links, images or formatting in the summary.
func mdText(s string) string {
	s = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\r", " ", "\n", " ").Replace(s)
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune("\\`*_{}[]()#+-.!|~\"'", r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

var severityIcon = map[finding.Severity]string{
	finding.Critical: "🔴", finding.High: "🟠", finding.Medium: "🟡", finding.Low: "🔵", finding.Info: "⚪",
}

// WriteMarkdown writes a Markdown summary: findings at or above the
// threshold in a table, the rest and the inventory in collapsed sections.
func WriteMarkdown(w io.Writer, res *engine.Result, m Meta) error {
	var b strings.Builder
	b.WriteString("## Portcullis\n\n")
	fmt.Fprintf(&b, "%s\n\n", mdText(summaryLine(res, m)))
	if !res.Complete {
		b.WriteString("> [!WARNING]\n> Analysis incomplete; these files could not be analysed:\n")
		for _, e := range res.Errors {
			fmt.Fprintf(&b, "> - %s\n", mdText(e))
		}
		b.WriteString("\n")
	}

	var above, below []finding.Finding
	for _, f := range res.Findings {
		if !m.NoFail && f.Severity >= m.FailOn {
			above = append(above, f)
		} else {
			below = append(below, f)
		}
	}
	table := func(fs []finding.Finding) {
		b.WriteString("| Severity | Rule | Location | Finding |\n|---|---|---|---|\n")
		for _, f := range fs {
			fmt.Fprintf(&b, "| %s %s | [%s](%s) | %s:%d | %s |\n", severityIcon[f.Severity], f.Severity, f.RuleID,
				ruleLink(f.RuleID), mdText(repoPath(m, f.Path)), f.Line, mdText(f.Message))
		}
		b.WriteString("\n")
	}
	if len(above) > 0 {
		table(above)
	} else {
		b.WriteString("No findings at or above the threshold.\n\n")
	}
	if len(below) > 0 {
		fmt.Fprintf(&b, "<details><summary>%d findings below the threshold</summary>\n\n", len(below))
		table(below)
		b.WriteString("</details>\n\n")
	}
	if len(res.Inventory) > 0 {
		fmt.Fprintf(&b, "<details><summary>Auto-execution inventory: %d commands run without asking</summary>\n\n", len(res.Inventory))
		b.WriteString("| Kind | Trigger | Location | Command |\n|---|---|---|---|\n")
		for _, it := range res.Inventory {
			fmt.Fprintf(&b, "| %s | %s | %s:%d | %s |\n", it.Kind, mdText(it.Trigger), mdText(repoPath(m, it.Path)), it.Line, mdText(it.Command))
		}
		b.WriteString("\n</details>\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func ruleLink(id string) string {
	if strings.HasPrefix(id, "PCL-") {
		return rulesDocURL + strings.ToLower(id)
	}
	return repoURL
}
