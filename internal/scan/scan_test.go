package scan

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"patchbot/internal/cli"
	"patchbot/internal/demo"
	"patchbot/internal/model"
	"patchbot/internal/osv"
)

func fixture(t *testing.T, dynamic bool) string {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("install Node.js to run integration tests")
	}
	_, source, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(source), "..", "..")
	if _, err := os.Stat(filepath.Join(root, "analyzer", "node_modules", "ts-morph")); err != nil {
		t.Fatal("install helper dependencies with npm ci --prefix analyzer --ignore-scripts before testing")
	}
	target := filepath.Join(t.TempDir(), "project with spaces")
	args := []string{filepath.Join(root, "analyzer", "test", "create-demo.mjs"), target}
	if dynamic {
		args = append(args, "--dynamic")
	}
	if output, err := exec.Command("node", args...).CombinedOutput(); err != nil {
		t.Fatalf("create fixture: %v %s", err, output)
	}
	return target
}

func snapshot(t *testing.T, root string) map[string][32]byte {
	t.Helper()
	result := map[string][32]byte{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result[path] = sha256.Sum256(content)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestEndToEndFindingsAndReadOnlyScan(t *testing.T) {
	target := fixture(t, false)
	before := snapshot(t, target)
	server := demo.Server(false)
	defer server.Close()
	s := New()
	s.Lookup = (osv.Client{BaseURL: server.URL}).Lookup
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r := s.Scan(ctx, target)
	if r.Status != "complete" || len(r.Findings) != 5 {
		data, _ := json.MarshalIndent(r, "", "  ")
		t.Fatalf("scan: %s", data)
	}
	if r.Findings[0].Priority != "manual_review" {
		t.Fatal("unknown severity must be visible")
	}
	if r.Analysis.Coverage.Packages != 7 || r.Analysis.Coverage.QueriedVersions != 7 {
		t.Fatalf("coverage: %+v", r.Analysis.Coverage)
	}
	for _, f := range r.Findings {
		if f.Package.Version == "2.0.0" && (f.Package.Direct || f.ImportState == "detected") {
			t.Fatal("duplicate/transitive version acquired root import")
		}
		if f.Package.Name == "demo-build-tool" && f.Priority != "high" {
			t.Fatal("development finding downgraded")
		}
	}
	after := snapshot(t, target)
	if len(before) != len(after) {
		t.Fatal("target file set changed")
	}
	for path, digest := range before {
		if after[path] != digest {
			t.Fatalf("target modified: %s", path)
		}
	}
	var stdout, stderr bytes.Buffer
	code := cli.Run(ctx, []string{"scan", "--format", "json", target}, &stdout, &stderr, s.Scan)
	var decoded model.Report
	if code != 1 || stderr.Len() != 0 || json.Unmarshal(stdout.Bytes(), &decoded) != nil || len(decoded.Findings) != 5 {
		t.Fatalf("CLI JSON round trip: code %d stderr %s", code, &stderr)
	}
}

func TestOutageAndDynamicLoadingProduceIncompleteReports(t *testing.T) {
	for _, tc := range []struct {
		name            string
		outage, dynamic bool
		findings        int
	}{{"outage", true, false, 0}, {"dynamic", false, true, 5}} {
		t.Run(tc.name, func(t *testing.T) {
			target := fixture(t, tc.dynamic)
			server := demo.Server(tc.outage)
			defer server.Close()
			s := New()
			s.Lookup = (osv.Client{BaseURL: server.URL}).Lookup
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			r := s.Scan(ctx, target)
			if r.Status != "partial" || cli.ExitCode(r) != 2 || len(r.Findings) != tc.findings {
				t.Fatalf("scan: %+v", r)
			}
		})
	}
}

func TestPrerequisitesAndFailureJSON(t *testing.T) {
	target := t.TempDir()
	if _, err := ValidateTarget(target); err == nil {
		t.Fatal("accepted missing package.json")
	}
	write := func(name, data string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(target, name), []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("package.json", `{"name":"demo","workspaces":["packages/*"]}`)
	if _, err := ValidateTarget(target); err == nil {
		t.Fatal("accepted workspace")
	}
	write("package.json", `{"name":"demo"}`)
	write("package-lock.json", `{"lockfileVersion":1}`)
	if _, err := ValidateTarget(target); err == nil {
		t.Fatal("accepted lockfile v1")
	}
	write("package-lock.json", `{"lockfileVersion":3,"packages":{}}`)
	if _, err := ValidateTarget(target); err == nil {
		t.Fatal("accepted missing node_modules")
	}
	if err := os.Mkdir(filepath.Join(target, "node_modules"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateTarget(target); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := cli.Run(context.Background(), []string{"scan", "--format", "json", filepath.Join(target, "missing")}, &stdout, &stderr, New().Scan)
	var report model.Report
	if code != 2 || json.Unmarshal(stdout.Bytes(), &report) != nil || report.Status != "failed" || len(report.Errors) == 0 {
		t.Fatal("target error was not a failed JSON report")
	}
}
