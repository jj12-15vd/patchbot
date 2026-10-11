package scan

import (
	"context"
	"time"

	"patchbot/internal/analyzer"
	"patchbot/internal/model"
	"patchbot/internal/osv"
	"patchbot/internal/risk"
)

type Scanner struct {
	Analyze func(context.Context, string) (model.Analysis, error)
	Lookup  func(context.Context, []model.Package) osv.Result
}

func New() Scanner {
	return Scanner{Analyze: (analyzer.Runner{}).Analyze, Lookup: (osv.Client{}).Lookup}
}

func (s Scanner) Scan(ctx context.Context, target string) model.Report {
	r := model.Report{SchemaVersion: model.SchemaVersion, RulesVersion: model.RulesVersion, ScannedAt: time.Now().UTC(),
		Target: target, Status: "failed", Findings: []model.Finding{}, Errors: []model.Warning{},
		Analysis: model.Analysis{SchemaVersion: model.SchemaVersion, InventoryMode: "installed", Packages: []model.Package{},
			Edges: []model.Edge{}, Imports: []model.Import{}, Warnings: []model.Warning{},
			Stages: map[string]model.Stage{"dependencies": {Status: "skipped"}, "source": {Status: "skipped"}, "vulnerabilities": {Status: "skipped"}}}}
	absolute, err := ValidateTarget(target)
	if err != nil {
		r.Errors = append(r.Errors, model.Warning{Stage: "target", Code: "INVALID_TARGET", Message: err.Error()})
		return r
	}
	r.Target = absolute
	a, err := s.Analyze(ctx, absolute)
	if err != nil {
		r.Analysis.Stages["dependencies"] = model.Stage{Status: "failed", Message: err.Error()}
		r.Errors = append(r.Errors, model.Warning{Stage: "analysis", Code: "ANALYZER_FAILED", Message: err.Error()})
		return r
	}
	r.Analysis = a
	result := s.Lookup(ctx, a.Packages)
	r.Errors = append(r.Errors, result.Errors...)
	r.Analysis.Coverage.QueriedVersions = result.Queried
	r.Analysis.Coverage.FailedQueries = result.Failed
	r.Analysis.Coverage.Withdrawn = result.Withdrawn
	lookupStatus := "complete"
	if result.Failed > 0 || len(result.Errors) > 0 || ctx.Err() != nil {
		lookupStatus = "partial"
	}
	if ctx.Err() != nil {
		r.Errors = append(r.Errors, model.Warning{Stage: "scan", Code: "INTERRUPTED", Message: ctx.Err().Error()})
	}
	r.Analysis.Stages["vulnerabilities"] = model.Stage{Status: lookupStatus}
	for _, p := range a.Packages {
		for _, advisory := range result.Advisories[osv.Key(p)] {
			r.Findings = append(r.Findings, risk.Classify(p, advisory, a.Imports, a.Stages["source"].Status))
		}
	}
	risk.Sort(r.Findings)
	r.Status = "complete"
	for _, stage := range r.Analysis.Stages {
		if stage.Status != "complete" {
			r.Status = "partial"
		}
	}
	return r
}
