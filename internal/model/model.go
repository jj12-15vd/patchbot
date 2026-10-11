// Package model defines the versioned JSON contract between Go and Node.
package model

import "time"

const SchemaVersion = 1
const RulesVersion = "prototype-v1"

type Stage struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type Warning struct {
	Stage   string `json:"stage"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type DependencyContext struct {
	Production  bool `json:"production"`
	Development bool `json:"development"`
	Optional    bool `json:"optional"`
	Peer        bool `json:"peer"`
	Unknown     bool `json:"unknown"`
}

type Package struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Version   string            `json:"version"`
	Location  string            `json:"location"`
	Direct    bool              `json:"direct"`
	Context   DependencyContext `json:"context"`
	Path      []string          `json:"dependency_path"`
	Queryable bool              `json:"queryable"`
}

type Edge struct {
	From  string `json:"from"`
	To    string `json:"to"` // Empty when the dependency is missing.
	Name  string `json:"name"`
	Type  string `json:"type"`
	Valid bool   `json:"valid"`
	Error string `json:"error,omitempty"`
}

type Import struct {
	Specifier   string `json:"specifier"`
	PackageName string `json:"package_name"`
	NodeID      string `json:"node_id"` // Empty when resolution is unknown.
	File        string `json:"file"`
	Line        int    `json:"line"`
	Kind        string `json:"kind"`
	TypeOnly    bool   `json:"type_only"`
}

type Coverage struct {
	Packages        int `json:"packages"`
	Extraneous      int `json:"extraneous"`
	Unqueryable     int `json:"unqueryable"`
	ScannedFiles    int `json:"scanned_files"`
	SkippedFiles    int `json:"skipped_files"`
	QueriedVersions int `json:"queried_versions"`
	FailedQueries   int `json:"failed_queries"`
	Withdrawn       int `json:"withdrawn_advisories"`
}

type Analysis struct {
	SchemaVersion int              `json:"schema_version"`
	InventoryMode string           `json:"inventory_mode"`
	Stages        map[string]Stage `json:"stages"`
	Packages      []Package        `json:"packages"`
	Edges         []Edge           `json:"edges"`
	Imports       []Import         `json:"imports"`
	Coverage      Coverage         `json:"coverage"`
	Warnings      []Warning        `json:"warnings"`
}

type SeverityEvidence struct {
	Type   string `json:"type"`
	Value  string `json:"value"`
	Source string `json:"source"`
}

type Reference struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type Affected struct {
	Name      string          `json:"name"`
	Ecosystem string          `json:"ecosystem"`
	Ranges    []AffectedRange `json:"ranges"`
	Versions  []string        `json:"versions"`
}

type AffectedRange struct {
	Type   string              `json:"type"`
	Events []map[string]string `json:"events"`
}

type Advisory struct {
	ID               string             `json:"id"`
	Aliases          []string           `json:"aliases"`
	Summary          string             `json:"summary"`
	Severity         string             `json:"severity"`
	SeverityEvidence []SeverityEvidence `json:"severity_evidence"`
	Affected         []Affected         `json:"affected"`
	References       []Reference        `json:"references"`
	Withdrawn        string             `json:"withdrawn,omitempty"`
}

type Finding struct {
	Package      Package  `json:"package"`
	Advisory     Advisory `json:"advisory"`
	ImportState  string   `json:"import_state"`
	Imports      []Import `json:"imports"`
	Reachability string   `json:"affected_function_reachability"`
	Priority     string   `json:"priority"`
	ContextOrder int      `json:"context_order"`
	RuleIDs      []string `json:"rule_ids"`
	Reasons      []string `json:"reasons"`
	Limitations  []string `json:"limitations"`
}

type Report struct {
	SchemaVersion int       `json:"schema_version"`
	RulesVersion  string    `json:"rules_version"`
	ScannedAt     time.Time `json:"scanned_at"`
	Target        string    `json:"target"`
	Status        string    `json:"status"`
	Analysis      Analysis  `json:"analysis"`
	Findings      []Finding `json:"findings"`
	Errors        []Warning `json:"errors"`
}
