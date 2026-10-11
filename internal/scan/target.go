package scan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func ValidateTarget(target string) (string, error) {
	absolute, err := filepath.Abs(target)
	if err != nil {
		return target, fmt.Errorf("resolve target path: %w", err)
	}
	absolute, err = filepath.EvalSymlinks(absolute)
	if err != nil {
		return target, fmt.Errorf("target project does not exist or is unreadable: %w", err)
	}
	if info, err := os.Stat(absolute); err != nil || !info.IsDir() {
		return absolute, fmt.Errorf("target must be a project directory")
	}
	var manifest struct {
		Workspaces json.RawMessage `json:"workspaces"`
	}
	if err := readJSON(filepath.Join(absolute, "package.json"), &manifest); err != nil {
		return absolute, fmt.Errorf("target requires a valid package.json: %w", err)
	}
	if len(manifest.Workspaces) > 0 && string(manifest.Workspaces) != "null" {
		return absolute, fmt.Errorf("npm workspaces are outside prototype coverage; choose a single npm project")
	}
	var lock struct {
		Version  int                        `json:"lockfileVersion"`
		Packages map[string]json.RawMessage `json:"packages"`
	}
	if err := readJSON(filepath.Join(absolute, "package-lock.json"), &lock); err != nil {
		return absolute, fmt.Errorf("target requires a valid npm package-lock.json: %w", err)
	}
	if (lock.Version != 2 && lock.Version != 3) || lock.Packages == nil {
		return absolute, fmt.Errorf("target requires package-lock.json version 2 or 3 with package inventory")
	}
	if info, err := os.Stat(filepath.Join(absolute, "node_modules")); err != nil || !info.IsDir() {
		return absolute, fmt.Errorf("target node_modules is missing; prepare the project's dependencies before scanning")
	}
	return absolute, nil
}

func readJSON(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data) > 16<<20 {
		return fmt.Errorf("JSON file exceeds 16 MiB limit")
	}
	data = bytes.TrimSpace(data)
	if !json.Valid(data) || len(data) == 0 || data[0] != '{' {
		return fmt.Errorf("expected a JSON object")
	}
	return json.Unmarshal(data, value)
}
