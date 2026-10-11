// Package cli owns arguments and exit codes, not scanning logic.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"patchbot/internal/model"
	"patchbot/internal/report"
)

const Version = "0.1.0-prototype"

type Options struct {
	Path    string
	Format  string
	Timeout time.Duration
}

// ScanFunc lets tests substitute a scanner without starting Node or calling OSV.
type ScanFunc func(context.Context, string) model.Report

func Run(ctx context.Context, args []string, stdout, stderr io.Writer, scan ScanFunc) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprintln(stdout, "Usage: patchbot scan [--format text|json] [--timeout 30s] <path>\n       patchbot --version\nPut scan options before the project path.")
		return 0
	}
	if args[0] == "--version" && len(args) == 1 {
		fmt.Fprintln(stdout, "patchbot", Version)
		return 0
	}
	if args[0] != "scan" {
		fmt.Fprintln(stderr, "patchbot: unknown command; use --help")
		return 2
	}
	opts, err := ParseScan(args[1:], stderr)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		fmt.Fprintln(stderr, "patchbot:", err)
		return 2
	}
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	r := scan(ctx, opts.Path)
	if err := report.Write(stdout, opts.Format, r); err != nil {
		fmt.Fprintln(stderr, "patchbot: cannot write report:", err)
		return 2
	}
	return ExitCode(r)
}

func ParseScan(args []string, diagnostics io.Writer) (Options, error) {
	var opts Options
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	fs.SetOutput(diagnostics)
	fs.StringVar(&opts.Format, "format", "text", "report format: text or json")
	fs.DurationVar(&opts.Timeout, "timeout", 30*time.Second, "total scan time limit")
	fs.Usage = func() {
		fmt.Fprintln(diagnostics, "Usage: patchbot scan [options] <path> (options before path)")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	if fs.NArg() != 1 {
		return opts, errors.New("provide exactly one project path; put options before it")
	}
	if opts.Format != "text" && opts.Format != "json" {
		return opts, errors.New("--format must be text or json")
	}
	if opts.Timeout <= 0 {
		return opts, errors.New("--timeout must be greater than zero")
	}
	opts.Path = fs.Arg(0)
	return opts, nil
}

func ExitCode(r model.Report) int {
	if r.Status != "complete" {
		return 2
	}
	if len(r.Findings) > 0 {
		return 1
	}
	return 0
}
