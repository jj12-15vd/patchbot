package analyzer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"patchbot/internal/model"
)

type Runner struct {
	HelperPath string
	Node       string
}

// FindHelper checks the shipped distribution first, then the compiled source
// location for go run. It never trusts the target project's working directory.
func FindHelper() string {
	var candidates []string
	if executable, err := os.Executable(); err == nil {
		// A command installed on PATH can be a symlink. Locate the helper beside
		// the real binary, not beside that shortcut.
		if resolved, err := filepath.EvalSymlinks(executable); err == nil {
			executable = resolved
		}
		candidates = append(candidates, filepath.Join(filepath.Dir(executable), "analyzer", "src", "index.mjs"))
	}
	if _, source, _, ok := runtime.Caller(0); ok {
		candidates = append(candidates, filepath.Join(filepath.Dir(source), "..", "..", "analyzer", "src", "index.mjs"))
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			absolute, _ := filepath.Abs(candidate)
			return absolute
		}
	}
	return ""
}

type limitedBuffer struct {
	data  bytes.Buffer
	Limit int
}

func (b *limitedBuffer) Bytes() []byte  { return b.data.Bytes() }
func (b *limitedBuffer) String() string { return b.data.String() }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.data.Len()+len(p) > b.Limit {
		return 0, errors.New("analyzer output exceeds size limit")
	}
	return b.data.Write(p)
}

func (r Runner) Analyze(ctx context.Context, target string) (model.Analysis, error) {
	var analysis model.Analysis
	if r.Node == "" {
		r.Node = "node"
	}
	if r.HelperPath == "" {
		r.HelperPath = FindHelper()
	}
	if r.HelperPath == "" {
		return analysis, errors.New("analysis helper missing: rebuild the complete PatchBot distribution")
	}
	if _, err := exec.LookPath(r.Node); err != nil {
		return analysis, errors.New("Node.js missing: install Node.js 24 or newer")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(r.HelperPath), "..", "node_modules", "@npmcli", "arborist", "package.json")); err != nil {
		return analysis, errors.New("helper dependencies missing: run npm ci --ignore-scripts --prefix analyzer in the PatchBot distribution")
	}
	cmd := exec.CommandContext(ctx, r.Node, r.HelperPath, target)
	cmd.WaitDelay = time.Second // Bound waits if a descendant retains output pipes.
	cmd.Dir = filepath.Dir(r.HelperPath)
	stdout := &limitedBuffer{Limit: 16 << 20}
	stderr := &limitedBuffer{Limit: 64 << 10}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return analysis, fmt.Errorf("analysis interrupted: %w", ctx.Err())
		}
		return analysis, fmt.Errorf("analysis helper failed: %w; %s", err, strings.TrimSpace(stderr.String()))
	}
	return Decode(stdout.Bytes())
}

// Decode rejects truncated or incompatible responses rather than inventing a
// successful empty scan. References are checked before evidence is joined.
func Decode(data []byte) (model.Analysis, error) {
	var a model.Analysis
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&a); err != nil {
		return a, fmt.Errorf("invalid analyzer JSON: %w", err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return a, errors.New("analyzer must return exactly one JSON object")
	}
	if a.SchemaVersion != model.SchemaVersion || a.InventoryMode != "installed" {
		return a, errors.New("unsupported analyzer schema or inventory mode")
	}
	if a.Packages == nil || a.Edges == nil || a.Imports == nil || a.Warnings == nil {
		return a, errors.New("analyzer response is missing required arrays")
	}
	for _, name := range []string{"dependencies", "source"} {
		s := a.Stages[name].Status
		if s != "complete" && s != "partial" && s != "failed" {
			return a, fmt.Errorf("invalid or missing %s stage", name)
		}
	}
	ids := map[string]bool{".": true}
	for _, p := range a.Packages {
		if p.ID == "" || ids[p.ID] || p.ID != p.Location || !localPath(p.ID) {
			return a, errors.New("invalid or duplicate package node ID")
		}
		if p.Queryable && (p.Name == "" || p.Version == "") {
			return a, errors.New("queryable package lacks identity")
		}
		ids[p.ID] = true
	}
	for _, e := range a.Edges {
		if !ids[e.From] || (e.To != "" && !ids[e.To]) {
			return a, errors.New("dependency edge references an unknown node")
		}
	}
	for _, p := range a.Packages {
		for _, id := range p.Path {
			if !ids[id] {
				return a, errors.New("dependency path references an unknown node")
			}
		}
	}
	for _, evidence := range a.Imports {
		if !localPath(evidence.File) || evidence.Line < 1 || (evidence.NodeID != "" && !ids[evidence.NodeID]) {
			return a, errors.New("invalid import location or node reference")
		}
	}
	if a.Coverage.Packages != len(a.Packages) {
		return a, errors.New("package coverage count does not match inventory")
	}
	return a, nil
}

func localPath(value string) bool {
	return value != "" && filepath.IsLocal(filepath.FromSlash(value)) && !strings.Contains(value, "\\")
}
