// This development-only command injects a local fake OSV server. The production
// CLI always uses OSV; demo endpoint configuration is not a production option.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"patchbot/internal/cli"
	"patchbot/internal/demo"
	"patchbot/internal/osv"
	"patchbot/internal/scan"
)

func main() { os.Exit(run()) }

func run() int {
	outage := flag.Bool("outage", false, "simulate OSV unavailable")
	dynamic := flag.Bool("dynamic", false, "add unsupported dynamic-loading examples")
	format := flag.String("format", "text", "text or json")
	flag.Parse()
	_, source, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(source), "..", "..")
	target, err := os.MkdirTemp("", "patchbot demo ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	defer os.RemoveAll(target)
	args := []string{filepath.Join(root, "analyzer", "test", "create-demo.mjs"), target}
	if *dynamic {
		args = append(args, "--dynamic")
	}
	cmd := exec.Command("node", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "fixture setup: %v %s\n", err, output)
		return 2
	}
	server := demo.Server(*outage)
	defer server.Close()
	scanner := scan.New()
	scanner.Lookup = (osv.Client{BaseURL: server.URL}).Lookup
	fmt.Fprintln(os.Stderr, "DEMO: fictional packages/advisories, local mock OSV server; no live vulnerability lookup.")
	return cli.Run(context.Background(), []string{"scan", "--format", *format, target}, os.Stdout, os.Stderr, scanner.Scan)
}
