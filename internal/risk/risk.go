package risk

import (
	"sort"

	"patchbot/internal/model"
)

// Classify is a review policy, not a claim about exploitability.
func Classify(p model.Package, advisory model.Advisory, imports []model.Import, sourceStatus string) model.Finding {
	f := model.Finding{Package: p, Advisory: advisory, Imports: []model.Import{}, Reachability: "unknown",
		ImportState: "unknown", Priority: "review", ContextOrder: 2, RuleIDs: []string{}, Reasons: []string{},
		Limitations: []string{"An import does not prove that affected code executes; missing imports do not prove safety.", "Source analysis covers application files only, not execution inside dependencies."}}
	runtimeImport := false
	for _, evidence := range imports {
		if evidence.NodeID == p.ID {
			f.Imports = append(f.Imports, evidence)
			if !evidence.TypeOnly {
				runtimeImport = true
			}
		}
	}
	if runtimeImport {
		f.ImportState = "detected"
	} else if sourceStatus == "complete" {
		f.ImportState = "not_detected_in_scanned_files"
	}
	if sourceStatus != "complete" {
		f.Limitations = append(f.Limitations, "Source coverage is incomplete; absence of evidence remains unknown.")
	}
	switch advisory.Severity {
	case "critical", "high":
		f.Priority = "high"
		f.RuleIDs = append(f.RuleIDs, "SEVERITY_HIGH")
		f.Reasons = append(f.Reasons, "High or critical advisory severity requires high review priority, including development tooling.")
	case "medium", "low", "none":
		f.RuleIDs = append(f.RuleIDs, "SEVERITY_REVIEW")
		f.Reasons = append(f.Reasons, "Known advisory severity places this finding in the review queue.")
	default:
		f.Priority = "manual_review"
		f.RuleIDs = append(f.RuleIDs, "SEVERITY_UNKNOWN")
		f.Reasons = append(f.Reasons, "Severity is missing or unsupported; manual review is required.")
	}
	if p.Context.Unknown || (!p.Context.Production && !p.Context.Development) {
		f.Priority = "manual_review"
		f.RuleIDs = append(f.RuleIDs, "CONTEXT_UNKNOWN")
		f.Reasons = append(f.Reasons, "Dependency context is unknown; review it explicitly without reducing advisory severity.")
	} else if p.Context.Production && runtimeImport {
		f.ContextOrder = 0
		f.RuleIDs = append(f.RuleIDs, "PRODUCTION_IMPORT")
		f.Reasons = append(f.Reasons, "A production dependency path and a static runtime import were detected; review first within its severity band.")
	} else if p.Context.Production {
		f.ContextOrder = 1
		f.RuleIDs = append(f.RuleIDs, "PRODUCTION_PATH")
		f.Reasons = append(f.Reasons, "A production dependency path exists; affected-function execution remains unknown.")
	} else {
		f.RuleIDs = append(f.RuleIDs, "DEVELOPMENT_PATH")
		f.Reasons = append(f.Reasons, "Dependency paths are development-only; build and CI exposure still need review.")
	}
	if !p.Direct {
		f.Reasons = append(f.Reasons, "This is a transitive dependency; a missing direct import does not rule out use by its parent package.")
	}
	return f
}

func Sort(findings []model.Finding) {
	priority := map[string]int{"manual_review": 0, "high": 1, "review": 2}
	severity := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3, "none": 4, "unknown": 5}
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if priority[a.Priority] != priority[b.Priority] {
			return priority[a.Priority] < priority[b.Priority]
		}
		if severity[a.Advisory.Severity] != severity[b.Advisory.Severity] {
			return severity[a.Advisory.Severity] < severity[b.Advisory.Severity]
		}
		if a.ContextOrder != b.ContextOrder {
			return a.ContextOrder < b.ContextOrder
		}
		if a.Package.Location != b.Package.Location {
			return a.Package.Location < b.Package.Location
		}
		return a.Advisory.ID < b.Advisory.ID
	})
}
