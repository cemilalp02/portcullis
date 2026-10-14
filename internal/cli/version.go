package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/cemilalp02/bekci/internal/version"
)

func runVersion(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return ExitOK
		}
		return ExitError
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "bekci version: unexpected argument %q\n", fs.Arg(0))
		return ExitError
	}

	info := version.Get()
	switch *format {
	case "text":
		fmt.Fprintf(stdout, "bekci %s\n  commit:  %s\n  built:   %s\n  go:      %s\n  ruleset: %s\n",
			info.Version, info.Commit, info.Date, info.GoVersion, info.Ruleset)
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(info); err != nil {
			fmt.Fprintf(stderr, "bekci version: %v\n", err)
			return ExitError
		}
	default:
		fmt.Fprintf(stderr, "bekci version: unknown format %q (want text or json)\n", *format)
		return ExitError
	}
	return ExitOK
}
