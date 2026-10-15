package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/cemilalp02/portcullis/internal/engine"
	"github.com/cemilalp02/portcullis/internal/report"
	"github.com/cemilalp02/portcullis/internal/version"
)

var writers = map[string]func(io.Writer, *engine.Result, report.Meta) error{
	"text":     report.WriteText,
	"json":     report.WriteJSON,
	"sarif":    report.WriteSARIF,
	"github":   report.WriteGitHub,
	"markdown": report.WriteMarkdown,
}

// useColor decides whether text output gets ANSI colours.
func useColor(mode string, out io.Writer) (bool, error) {
	switch mode {
	case "always":
		return true, nil
	case "never":
		return false, nil
	case "auto":
	default:
		return false, fmt.Errorf("-color: unknown value %q (want auto, always or never)", mode)
	}
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false, nil
	}
	f, ok := out.(*os.File)
	if !ok {
		return false, nil
	}
	fi, err := f.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return false, nil
	}
	if runtime.GOOS == "windows" && os.Getenv("WT_SESSION") == "" && os.Getenv("TERM") == "" && os.Getenv("ANSICON") == "" {
		return false, nil // legacy console without ANSI support
	}
	return true, nil
}

// writeFile writes a report to path (append for GitHub step summaries).
func writeFile(path string, appendMode bool, write func(io.Writer) error) error {
	flags := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	if appendMode {
		flags = os.O_CREATE | os.O_WRONLY | os.O_APPEND
	}
	f, err := os.OpenFile(path, flags, 0o644) //nolint:gosec // reports are meant to be shared, like any build output
	if err != nil {
		return err
	}
	if err := write(f); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func runScan(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var c common
	c.register(fs)
	format := fs.String("format", "text", "output `format`: text, json, sarif, github (workflow annotations) or markdown")
	output := fs.String("output", "", "write results to `file` instead of stdout")
	fs.StringVar(output, "o", "", "shorthand for -output")
	sarifOut := fs.String("sarif", "", "also write SARIF 2.1.0 to `file`")
	summaryOut := fs.String("summary", "", "also append a Markdown summary to `file` (e.g. $GITHUB_STEP_SUMMARY)")
	prefix := fs.String("path-prefix", "", "`prefix` for file paths in SARIF and annotations when scanning a subdirectory of a repository")
	noInventory := fs.Bool("no-inventory", false, "hide the auto-execution inventory in text output")
	color := fs.String("color", "auto", "colour text output: `auto`, always or never (NO_COLOR is honoured)")
	fs.Usage = func() {
		fmt.Fprint(stderr, "Usage: portcullis scan [flags] [path]\n\nScan a directory (default \".\") for malicious or risky AI agent context.\nExit codes: 0 clean, 1 findings at or above the threshold, 2 error or incomplete analysis.\n\nFlags:\n")
		fs.PrintDefaults()
	}
	pos, err := parseArgs(fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return ExitOK
		}
		return ExitError
	}
	fail := func(err error) int {
		fmt.Fprintf(stderr, "portcullis scan: %v\n", err)
		return ExitError
	}
	path, err := onePath(pos)
	if err != nil {
		return fail(err)
	}
	write, ok := writers[*format]
	if !ok {
		return fail(fmt.Errorf("unknown format %q (want text, json, sarif, github or markdown)", *format))
	}
	out := stdout
	var outFile *os.File
	if *output != "" {
		if outFile, err = os.Create(*output); err != nil {
			return fail(err)
		}
		defer outFile.Close()
		out = outFile
	}
	colored, err := useColor(*color, out)
	if err != nil {
		return fail(err)
	}

	s, err := c.open(path, visited(fs))
	if err != nil {
		return fail(err)
	}
	for _, w := range s.warnings {
		fmt.Fprintln(stderr, w)
	}
	res, suppressed, err := s.scan(true)
	if err != nil {
		return fail(err)
	}
	meta := report.Meta{
		Version: version.Get().Version, Ruleset: version.Get().Ruleset, Mode: s.mode,
		FailOn: s.settings.FailOn, NoFail: s.settings.NoFail, Inventory: !*noInventory,
		Color: colored && *format == "text", Suppressed: suppressed, PathPrefix: *prefix, Rules: s.engine.Rules.Rules(),
	}
	if err := write(out, res, meta); err != nil {
		return fail(fmt.Errorf("writing report: %w", err))
	}
	if *sarifOut != "" {
		if err := writeFile(*sarifOut, false, func(w io.Writer) error { return report.WriteSARIF(w, res, meta) }); err != nil {
			return fail(fmt.Errorf("writing SARIF: %w", err))
		}
	}
	if *summaryOut != "" {
		if err := writeFile(*summaryOut, true, func(w io.Writer) error { return report.WriteMarkdown(w, res, meta) }); err != nil {
			return fail(fmt.Errorf("writing summary: %w", err))
		}
	}
	return report.ExitCode(res, meta)
}
