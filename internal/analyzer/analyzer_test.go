package analyzer

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"patchbot/internal/model"
)

func envelope() model.Analysis {
	return model.Analysis{SchemaVersion: 1, InventoryMode: "installed", Stages: map[string]model.Stage{"dependencies": {Status: "complete"}, "source": {Status: "complete"}}, Packages: []model.Package{}, Edges: []model.Edge{}, Imports: []model.Import{}, Warnings: []model.Warning{}}
}

func TestDecodeRejectsIncompatibleAndBrokenEvidence(t *testing.T) {
	a := envelope()
	data, _ := json.Marshal(a)
	if _, err := Decode(data); err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{[]byte(`{}`), []byte(`null`), append(append([]byte(nil), data...), data...)} {
		if _, err := Decode(data); err == nil {
			t.Fatalf("accepted broken contract: %s", data)
		}
	}
	a.Imports = []model.Import{{NodeID: "missing", File: "src/index.ts", Line: 1}}
	data, _ = json.Marshal(a)
	if _, err := Decode(data); err == nil {
		t.Fatal("accepted dangling import reference")
	}
}

func TestOutputLimitAppliesToStreamCopies(t *testing.T) {
	buffer := &limitedBuffer{Limit: 4}
	if _, err := io.Copy(buffer, strings.NewReader("oversized")); err == nil {
		t.Fatal("stream copy bypassed analyzer output limit")
	}
}

func TestRunnerHandlesMalformedJSONCancellationAndMissingNode(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node.js is required for helper process tests")
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "node_modules", "@npmcli", "arborist"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "node_modules", "@npmcli", "arborist", "package.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	helper := filepath.Join(root, "src", "index.mjs")
	if err := os.WriteFile(helper, []byte(`console.log("broken json");`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := (Runner{HelperPath: helper}).Analyze(context.Background(), "path with spaces"); err == nil || !strings.Contains(err.Error(), "JSON") {
		t.Fatalf("malformed JSON: %v", err)
	}
	if err := os.WriteFile(helper, []byte(`setInterval(() => {}, 1000);`), 0644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := (Runner{HelperPath: helper}).Analyze(ctx, "path with spaces"); err == nil || !strings.Contains(err.Error(), "interrupted") {
		t.Fatalf("cancellation: %v", err)
	}
	if _, err := (Runner{Node: filepath.Join(root, "missing-node"), HelperPath: helper}).Analyze(context.Background(), root); err == nil || !strings.Contains(err.Error(), "Node.js missing") {
		t.Fatalf("missing Node: %v", err)
	}
}
