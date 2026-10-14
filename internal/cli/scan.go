package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/cemilalp02/portcullis/internal/discover"
	"github.com/cemilalp02/portcullis/internal/engine"
	"github.com/cemilalp02/portcullis/internal/finding"
	"github.com/cemilalp02/portcullis/internal/report"
	"github.com/cemilalp02/portcullis/internal/textview"
	"github.com/cemilalp02/portcullis/internal/version"
)

// stringList is a repeatable string flag.
type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

// parseArgs parses flags that may appear before or after positional
// arguments and returns the positional ones.
func parseArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return pos, nil
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

// parseSize parses sizes such as 10MiB, 512KiB, 2MB or 1048576.
func parseSize(s string) (int64, error) {
	units := []struct {
		suffix string
		mult   int64
	}{{"kib", 1 << 10}, {"mib", 1 << 20}, {"gib", 1 << 30}, {"kb", 1e3}, {"mb", 1e6}, {"gb", 1e9}, {"b", 1}}
	t := strings.ToLower(strings.TrimSpace(s))
	mult := int64(1)
	for _, u := range units {
		if strings.HasSuffix(t, u.suffix) {
			t, mult = strings.TrimSpace(strings.TrimSuffix(t, u.suffix)), u.mult
			break
		}
	}
	n, err := strconv.ParseInt(t, 10, 64)
	if err != nil || n <= 0 || n > textview.MaxInput/mult {
		return 0, fmt.Errorf("invalid size %q (want 1 byte to 1GiB, e.g. 10MiB, 512KiB or a byte count)", s)
	}
	return n * mult, nil
}

func runScan(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "text", "output format: text or json")
	output := fs.String("output", "", "write results to `file` instead of stdout")
	fs.StringVar(output, "o", "", "shorthand for -output")
	failOn := fs.String("fail-on", "high", "exit 1 if a finding is at or above `severity` (info, low, medium, high, critical, none)")
	maxSize := fs.String("max-file-size", "10MiB", "hard `size` limit per agent file")
	noInventory := fs.Bool("no-inventory", false, "hide the auto-execution inventory in text output")
	var excludes stringList
	fs.Var(&excludes, "exclude", "skip directories matching `glob` (repeatable)")
	fs.Usage = func() {
		fmt.Fprint(stderr, "Usage: portcullis scan [flags] [path]\n\nScan a directory (default \".\") for malicious agent context.\n\nFlags:\n")
		fs.PrintDefaults()
	}

	pos, err := parseArgs(fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return ExitOK
		}
		return ExitError
	}
	if len(pos) > 1 {
		fmt.Fprintf(stderr, "portcullis scan: expected one path, got %d\n", len(pos))
		return ExitError
	}
	root := "."
	if len(pos) == 1 {
		root = pos[0]
	}

	meta := report.Meta{Version: version.Get().Version, Ruleset: version.Get().Ruleset, Mode: "untrusted", Inventory: !*noInventory}
	if strings.EqualFold(*failOn, "none") {
		meta.NoFail = true
	} else if meta.FailOn, err = finding.ParseSeverity(*failOn); err != nil {
		fmt.Fprintf(stderr, "portcullis scan: -fail-on: %v\n", err)
		return ExitError
	}
	write := report.WriteText
	switch *format {
	case "text":
	case "json":
		write = report.WriteJSON
	default:
		fmt.Fprintf(stderr, "portcullis scan: unknown format %q (want text or json)\n", *format)
		return ExitError
	}
	size, err := parseSize(*maxSize)
	if err != nil {
		fmt.Fprintf(stderr, "portcullis scan: -max-file-size: %v\n", err)
		return ExitError
	}

	eng, err := engine.Default()
	if err != nil {
		fmt.Fprintf(stderr, "portcullis scan: loading rules: %v\n", err)
		return ExitError
	}
	res, err := eng.Scan(root, engine.Options{
		MaxFileSize: size,
		SkipDirs:    append(append([]string{}, discover.DefaultSkipDirs...), excludes...),
	})
	if err != nil {
		fmt.Fprintf(stderr, "portcullis scan: %v\n", err)
		return ExitError
	}

	out := stdout
	if *output != "" {
		f, err := os.Create(*output)
		if err != nil {
			fmt.Fprintf(stderr, "portcullis scan: %v\n", err)
			return ExitError
		}
		defer f.Close()
		out = f
	}
	if err := write(out, res, meta); err != nil {
		fmt.Fprintf(stderr, "portcullis scan: writing report: %v\n", err)
		return ExitError
	}
	return report.ExitCode(res, meta)
}
