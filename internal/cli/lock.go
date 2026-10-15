package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/cemilalp02/portcullis/internal/baseline"
	"github.com/cemilalp02/portcullis/internal/engine"
	"github.com/cemilalp02/portcullis/internal/escape"
	"github.com/cemilalp02/portcullis/internal/finding"
	"github.com/cemilalp02/portcullis/internal/lock"
	"github.com/cemilalp02/portcullis/internal/report"
	"github.com/cemilalp02/portcullis/internal/version"
)

func sources(res *engine.Result) []lock.Source {
	out := make([]lock.Source, 0, len(res.TargetFiles))
	for _, tf := range res.TargetFiles {
		out = append(out, lock.Source{Path: tf.Path, ReadPath: tf.ReadPath})
	}
	return out
}

func parseCommand(name, usage string, args []string, stderr io.Writer, setup func(*flag.FlagSet)) (*flag.FlagSet, string, int, bool) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	setup(fs)
	fs.Usage = func() {
		fmt.Fprint(stderr, usage+"\nFlags:\n")
		fs.PrintDefaults()
	}
	pos, err := parseArgs(fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, "", ExitOK, false
		}
		return nil, "", ExitError, false
	}
	path, err := onePath(pos)
	if err != nil {
		fmt.Fprintf(stderr, "portcullis %s: %v\n", name, err)
		return nil, "", ExitError, false
	}
	return fs, path, 0, true
}

func runLock(args []string, stdout, stderr io.Writer) int {
	var c common
	var force bool
	var output string
	fs, path, code, ok := parseCommand("lock", "Usage: portcullis lock [flags] [path]\n\nScan the repository and, if nothing at or above the threshold is found, approve the\ncurrent agent context files by writing their hashes to context.lock.\n", args, stderr, func(fs *flag.FlagSet) {
		c.register(fs)
		fs.BoolVar(&force, "force", false, "approve even if findings at or above the threshold exist (recorded in the lock)")
		fs.StringVar(&output, "output", "", "write the lock to `file` (default <path>/context.lock)")
		fs.StringVar(&output, "o", "", "shorthand for -output")
	})
	if !ok {
		return code
	}
	fail := func(err error) int {
		fmt.Fprintf(stderr, "portcullis lock: %v\n", err)
		return ExitError
	}
	s, err := c.open(path, visited(fs))
	if err != nil {
		return fail(err)
	}
	res, suppressed, err := s.scan(true)
	if err != nil {
		return fail(err)
	}
	meta := report.Meta{Version: version.Get().Version, Ruleset: version.Get().Ruleset, Mode: s.mode,
		FailOn: s.settings.FailOn, NoFail: s.settings.NoFail, Suppressed: suppressed}
	if !res.Complete {
		_ = report.WriteText(stderr, res, meta)
		return fail(errors.New("analysis incomplete; nothing was approved"))
	}
	above := report.Above(res, meta)
	if above > 0 && !force {
		_ = report.WriteText(stdout, res, meta)
		fmt.Fprintf(stderr, "portcullis lock: refusing to approve: %d findings at or above '%s'. Fix or baseline them, or use -force.\n", above, s.settings.FailOn)
		return ExitFindings
	}
	entries, err := lock.Compute(res.Root, sources(res))
	if err != nil {
		return fail(err)
	}
	if output == "" {
		output = filepath.Join(s.root, lock.FileName)
	}
	data := lock.Format(lock.File{Ruleset: version.Get().Ruleset, Forced: above > 0, Entries: entries})
	if err := os.WriteFile(output, data, 0o644); err != nil { //nolint:gosec // context.lock is committed to the repository
		return fail(err)
	}
	fmt.Fprintf(stdout, "Approved %d agent context files in %s\n", len(entries), escape.String(output, 0))
	if above > 0 {
		fmt.Fprintf(stdout, "Forced despite %d findings at or above '%s' (recorded as 'forced true').\n", above, s.settings.FailOn)
	}
	return ExitOK
}

type verifyJSON struct {
	Match      bool     `json:"match"`
	Approved   int      `json:"approved"`
	Ruleset    string   `json:"ruleset"`
	Forced     bool     `json:"forced"`
	Added      []string `json:"added"`
	Modified   []string `json:"modified"`
	Removed    []string `json:"removed"`
	Reapproved []string `json:"reapproved"`
}

func runVerify(args []string, stdout, stderr io.Writer) int {
	var c common
	var lockPath, format string
	fs, path, code, ok := parseCommand("verify", "Usage: portcullis verify [flags] [path]\n\nCheck that the agent context files still match context.lock. Exit 1 if any file was\nadded, modified or removed since it was approved.\n", args, stderr, func(fs *flag.FlagSet) {
		c.register(fs)
		fs.StringVar(&lockPath, "lock", "", "lock `file` (default <path>/context.lock)")
		fs.StringVar(&format, "format", "text", "output `format`: text or json")
	})
	if !ok {
		return code
	}
	fail := func(err error) int {
		fmt.Fprintf(stderr, "portcullis verify: %v\n", err)
		return ExitError
	}
	if format != "text" && format != "json" {
		return fail(fmt.Errorf("unknown format %q (want text or json)", format))
	}
	s, err := c.open(path, visited(fs))
	if err != nil {
		return fail(err)
	}
	if lockPath == "" {
		lockPath = filepath.Join(s.root, lock.FileName)
	}
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return fail(fmt.Errorf("%w (create it with 'portcullis lock')", err))
	}
	approved, err := lock.Parse(data)
	if err != nil {
		return fail(err)
	}
	res, err := s.engine.Scan(s.root, s.options())
	if err != nil {
		return fail(err)
	}
	current, err := lock.Compute(res.Root, sources(res))
	if err != nil {
		return fail(err)
	}
	d := lock.Compare(approved.Entries, current)

	var reapproved []string
	if s.snapshot != "" {
		if old, err := os.ReadFile(filepath.Join(s.snapshot, lock.FileName)); err == nil {
			if base, err := lock.Parse(old); err == nil {
				rd := lock.Compare(base.Entries, approved.Entries)
				reapproved = append(append(append(reapproved, rd.Added...), rd.Modified...), rd.Removed...)
			}
		}
	}

	if format == "json" {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(verifyJSON{Match: d.Empty(), Approved: len(approved.Entries), Ruleset: approved.Ruleset, Forced: approved.Forced,
			Added: nonNil(d.Added), Modified: nonNil(d.Modified), Removed: nonNil(d.Removed), Reapproved: nonNil(reapproved)})
	} else {
		fmt.Fprintf(stdout, "context.lock: %d agent context files approved with rule set %s\n", len(approved.Entries), escape.String(approved.Ruleset, 40))
		if approved.Forced {
			fmt.Fprintln(stdout, "warning: the lock was created with -force despite findings")
		}
		if len(reapproved) > 0 {
			fmt.Fprintf(stdout, "This change updates context.lock and re-approves %d files:\n", len(reapproved))
			for _, p := range reapproved {
				fmt.Fprintf(stdout, "  %s\n", escape.String(p, 0))
			}
		}
		if d.Empty() {
			fmt.Fprintln(stdout, "OK: no agent context file changed since it was approved.")
		} else {
			fmt.Fprintf(stdout, "MISMATCH: %d agent context files changed since they were approved:\n", len(d.Added)+len(d.Modified)+len(d.Removed))
			for _, x := range []struct {
				label string
				paths []string
			}{{"added   ", d.Added}, {"modified", d.Modified}, {"removed ", d.Removed}} {
				for _, p := range x.paths {
					fmt.Fprintf(stdout, "  %s  %s\n", x.label, escape.String(p, 0))
				}
			}
			fmt.Fprintln(stdout, "Review the changes with 'portcullis scan', then approve them with 'portcullis lock'.")
		}
	}
	if !d.Empty() {
		return ExitFindings
	}
	return ExitOK
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func runBaseline(args []string, stdout, stderr io.Writer) int {
	var c common
	var output, reason string
	fs, path, code, ok := parseCommand("baseline", "Usage: portcullis baseline [flags] [path]\n\nWrite a baseline that accepts every current finding, so that only new findings are\nreported from now on. Reasons of entries already in the baseline are kept.\n", args, stderr, func(fs *flag.FlagSet) {
		c.register(fs)
		fs.StringVar(&output, "output", "", "write the baseline to `file` (default <path>/.portcullis-baseline.yaml)")
		fs.StringVar(&output, "o", "", "shorthand for -output")
		fs.StringVar(&reason, "reason", "", "`reason` recorded for new entries (default: accepted when the baseline was created)")
	})
	if !ok {
		return code
	}
	fail := func(err error) int {
		fmt.Fprintf(stderr, "portcullis baseline: %v\n", err)
		return ExitError
	}
	s, err := c.open(path, visited(fs))
	if err != nil {
		return fail(err)
	}
	res, _, err := s.scan(false)
	if err != nil {
		return fail(err)
	}
	if !res.Complete {
		return fail(errors.New("analysis incomplete; refusing to write a baseline"))
	}
	if output == "" {
		output = filepath.Join(s.root, baseline.FileName)
	}
	if reason == "" {
		reason = "accepted when the baseline was created on " + time.Now().UTC().Format("2006-01-02")
	}
	keep := map[string]string{}
	if old, err := baseline.Load(output); err == nil {
		for _, e := range old.Suppressions {
			keep[e.Fingerprint] = e.Reason
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fail(err)
	}
	var accept []finding.Finding
	for _, f := range res.Findings {
		if f.RuleID != engine.RuleStaleBaseline && f.RuleID != engine.RuleProtected && f.RuleID != engine.RuleReadError {
			accept = append(accept, f)
		}
	}
	data, err := baseline.Generate(accept, reason)
	if err != nil {
		return fail(err)
	}
	if len(keep) > 0 {
		f, err := baseline.Parse(output, data)
		if err != nil {
			return fail(err)
		}
		for i, e := range f.Suppressions {
			if r, ok := keep[e.Fingerprint]; ok {
				f.Suppressions[i].Reason = r
			}
		}
		if data, err = baseline.Encode(f); err != nil {
			return fail(err)
		}
	}
	if err := os.WriteFile(output, data, 0o644); err != nil { //nolint:gosec // the baseline is committed to the repository
		return fail(err)
	}
	fmt.Fprintf(stdout, "Wrote %d accepted findings to %s\nReview every entry and replace generic reasons with real ones before committing.\n",
		len(accept), escape.String(output, 0))
	return ExitOK
}
