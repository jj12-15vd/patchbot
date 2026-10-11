package risk

import (
	"patchbot/internal/model"
	"testing"
)

func TestPriorityAndEvidenceBoundaries(t *testing.T) {
	dev := model.Package{ID: "node_modules/tool", Location: "node_modules/tool", Context: model.DependencyContext{Development: true}}
	f := Classify(dev, model.Advisory{Severity: "high"}, nil, "complete")
	if f.Priority != "high" || f.Reachability != "unknown" || f.ImportState != "not_detected_in_scanned_files" {
		t.Fatalf("dev finding: %+v", f)
	}
	if f := Classify(dev, model.Advisory{Severity: "unknown"}, nil, "complete"); f.Priority != "manual_review" {
		t.Fatal("unknown severity downgraded")
	}
	if f := Classify(dev, model.Advisory{Severity: "high"}, nil, "partial"); f.ImportState != "unknown" {
		t.Fatal("incomplete source treated as non-use")
	}
	typeOnly := []model.Import{{NodeID: dev.ID, TypeOnly: true}}
	if f := Classify(dev, model.Advisory{Severity: "high"}, typeOnly, "complete"); f.ImportState == "detected" {
		t.Fatal("type-only treated as runtime")
	}
	prod := dev
	prod.Context.Production = true
	if f := Classify(prod, model.Advisory{Severity: "high"}, []model.Import{{NodeID: prod.ID}}, "complete"); f.ContextOrder != 0 {
		t.Fatal("mixed production/import context not ordered first")
	}
}

func TestContextOrdersEqualSeverityWithoutHidingDevelopmentFinding(t *testing.T) {
	prod := model.Package{ID: "node_modules/prod", Location: "node_modules/prod", Context: model.DependencyContext{Production: true}}
	dev := model.Package{ID: "node_modules/dev", Location: "node_modules/dev", Context: model.DependencyContext{Development: true}}
	advisory := model.Advisory{ID: "DEMO", Severity: "high"}
	findings := []model.Finding{Classify(dev, advisory, nil, "complete"), Classify(prod, advisory, []model.Import{{NodeID: prod.ID}}, "complete")}
	Sort(findings)
	if findings[0].Package.ID != prod.ID || findings[1].Priority != "high" {
		t.Fatalf("ordering: %+v", findings)
	}
	Sort(findings)
	if findings[0].Package.ID != prod.ID {
		t.Fatal("unstable ordering")
	}
}
