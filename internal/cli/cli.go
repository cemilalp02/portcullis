// Package cli implements the bekci command-line interface.
//
// Run is the only entry point; it never calls os.Exit and writes only to the
// writers it is given, so every command is testable in-process.
package cli

import (
	"fmt"
	"io"
)

// Exit codes, see docs/CLI.md §2.
const (
	// ExitOK means the analysis completed with nothing at or above the threshold.
	ExitOK = 0
	// ExitFindings means the analysis completed with findings at or above the threshold.
	ExitFindings = 1
	// ExitError means a usage or configuration error, or an incomplete analysis.
	ExitError = 2
)

type command struct {
	name    string
	summary string
	run     func(args []string, stdout, stderr io.Writer) int
}

func commands() []command {
	return []command{
		{name: "version", summary: "Print version and build information", run: runVersion},
	}
}

// Run executes the command line args (without the program name) and returns
// the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return ExitError
	}
	switch args[0] {
	case "help", "-h", "-help", "--help":
		usage(stdout)
		return ExitOK
	case "--version":
		return runVersion(nil, stdout, stderr)
	}
	for _, c := range commands() {
		if c.name == args[0] {
			return c.run(args[1:], stdout, stderr)
		}
	}
	fmt.Fprintf(stderr, "bekci: unknown command %q\n\n", args[0])
	usage(stderr)
	return ExitError
}

func usage(w io.Writer) {
	fmt.Fprint(w, "Bekçi scans AI agent context files before an agent reads them.\n\n")
	fmt.Fprint(w, "Usage:\n  bekci <command> [flags] [path]\n\nCommands:\n")
	for _, c := range commands() {
		fmt.Fprintf(w, "  %-10s %s\n", c.name, c.summary)
	}
	fmt.Fprint(w, "\nRun 'bekci <command> -h' for command flags.\n")
}
