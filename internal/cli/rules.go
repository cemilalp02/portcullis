package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/cemilalp02/portcullis/internal/engine"
	"github.com/cemilalp02/portcullis/internal/ruleset"
)

type ruleInfo struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Severity    string   `json:"severity"`
	Confidence  string   `json:"confidence"`
	Targets     []string `json:"targets"`
	OWASP       []string `json:"owasp"`
	Since       string   `json:"since"`
	Description string   `json:"description"`
	Remediation string   `json:"remediation"`
	References  []string `json:"references"`
}

func info(r *ruleset.Compiled) ruleInfo {
	oneLine := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	refs := r.References
	if refs == nil {
		refs = []string{}
	}
	return ruleInfo{
		ID: r.ID, Title: r.Title, Severity: r.Severity.String(), Confidence: r.Confidence.String(),
		Targets: r.Targets, OWASP: r.OWASP, Since: r.Since,
		Description: oneLine(r.Description), Remediation: oneLine(r.Remediation), References: refs,
	}
}

func runRules(args []string, stdout, stderr io.Writer) int {
	usage := "Usage: portcullis rules list [-format text|json]\n       portcullis rules show <rule-id>\n"
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return ExitError
	}
	eng, err := engine.Default()
	if err != nil {
		fmt.Fprintf(stderr, "portcullis rules: %v\n", err)
		return ExitError
	}
	switch args[0] {
	case "list":
		return rulesList(eng, args[1:], stdout, stderr)
	case "show":
		if len(args) != 2 {
			fmt.Fprint(stderr, usage)
			return ExitError
		}
		r := eng.Rules.Get(strings.ToUpper(args[1]))
		if r == nil {
			fmt.Fprintf(stderr, "portcullis rules: unknown rule %q\n", args[1])
			return ExitError
		}
		rulesShow(info(r), stdout)
		return ExitOK
	case "-h", "-help", "--help", "help":
		fmt.Fprint(stdout, usage)
		return ExitOK
	}
	fmt.Fprintf(stderr, "portcullis rules: unknown subcommand %q\n%s", args[0], usage)
	return ExitError
}

func rulesList(eng *engine.Engine, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("rules list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return ExitOK
		}
		return ExitError
	}
	var list []ruleInfo
	for _, r := range eng.Rules.Rules() {
		list = append(list, info(r))
	}
	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(list); err != nil {
			return ExitError
		}
	case "text":
		tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "ID\tSEVERITY\tCONFIDENCE\tTITLE")
		for _, r := range list {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.ID, r.Severity, r.Confidence, r.Title)
		}
		if err := tw.Flush(); err != nil {
			return ExitError
		}
	default:
		fmt.Fprintf(stderr, "portcullis rules list: unknown format %q (want text or json)\n", *format)
		return ExitError
	}
	return ExitOK
}

func rulesShow(r ruleInfo, w io.Writer) {
	fmt.Fprintf(w, "%s  %s\n", r.ID, r.Title)
	fmt.Fprintf(w, "severity: %s   confidence: %s   since: %s\n", r.Severity, r.Confidence, r.Since)
	fmt.Fprintf(w, "targets: %s\nowasp: %s\n\n", strings.Join(r.Targets, ", "), strings.Join(r.OWASP, ", "))
	fmt.Fprintf(w, "Description\n  %s\n\nRemediation\n  %s\n", r.Description, r.Remediation)
	if len(r.References) > 0 {
		fmt.Fprint(w, "\nReferences\n")
		for _, ref := range r.References {
			fmt.Fprintf(w, "  - %s\n", ref)
		}
	}
}
