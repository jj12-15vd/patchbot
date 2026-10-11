package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"patchbot/internal/model"
)

func TestParseScan(t *testing.T) {
	for _, args := range [][]string{{}, {"--format", "yaml", "."}, {"--timeout", "0s", "."}, {"--timeout", "-1s", "."}, {".", "--format", "json"}, {"a", "b"}} {
		if _, err := ParseScan(args, io.Discard); err == nil {
			t.Fatalf("accepted invalid args: %v", args)
		}
	}
	opts, err := ParseScan([]string{"--format", "json", "--timeout", "2s", "path with spaces"}, io.Discard)
	if err != nil || opts.Path != "path with spaces" || opts.Format != "json" || opts.Timeout != 2*time.Second {
		t.Fatalf("options: %+v, %v", opts, err)
	}
}

func TestRunReportsAndExitCodes(t *testing.T) {
	for _, tc := range []struct {
		name, status   string
		findings, code int
	}{{"clean", "complete", 0, 0}, {"findings", "complete", 1, 1}, {"incomplete finding", "partial", 1, 2}, {"failed", "failed", 0, 2}} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			fake := func(ctx context.Context, path string) model.Report {
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("scan has no deadline")
				}
				if path != "project" {
					t.Fatalf("path: %s", path)
				}
				return model.Report{Status: tc.status, Findings: make([]model.Finding, tc.findings)}
			}
			code := Run(context.Background(), []string{"scan", "--format", "json", "project"}, &stdout, &stderr, fake)
			if code != tc.code || stderr.Len() != 0 {
				t.Fatalf("code=%d stderr=%s", code, &stderr)
			}
			var r model.Report
			if err := json.Unmarshal(stdout.Bytes(), &r); err != nil {
				t.Fatalf("stdout is not JSON: %v", err)
			}
			if r.Status != tc.status {
				t.Fatal("wrong report status")
			}
		})
	}
}

func TestHelpAndInvalidArgumentsDoNotScan(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code int
	}{{[]string{"--help"}, 0}, {[]string{"--version"}, 0}, {[]string{"scan", "--help"}, 0}, {[]string{"unknown"}, 2}, {[]string{"scan", "--timeout", "0s", "."}, 2}} {
		code := Run(context.Background(), tc.args, io.Discard, io.Discard, func(context.Context, string) model.Report { t.Fatal("unexpected scan"); return model.Report{} })
		if code != tc.code {
			t.Fatalf("%v: %d", tc.args, code)
		}
	}
}
