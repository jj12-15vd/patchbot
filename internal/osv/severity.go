package osv

import (
	"strings"

	cvss30 "github.com/pandatix/go-cvss/30"
	cvss31 "github.com/pandatix/go-cvss/31"
	cvss40 "github.com/pandatix/go-cvss/40"
	"patchbot/internal/model"
)

type rawSeverity struct {
	Type  string `json:"type"`
	Score string `json:"score"`
}
type rawAdvisory struct {
	ID               string            `json:"id"`
	Aliases          []string          `json:"aliases"`
	Summary          string            `json:"summary"`
	Withdrawn        string            `json:"withdrawn"`
	Severity         []rawSeverity     `json:"severity"`
	DatabaseSpecific map[string]any    `json:"database_specific"`
	References       []model.Reference `json:"references"`
	Affected         []struct {
		Package struct {
			Name      string `json:"name"`
			Ecosystem string `json:"ecosystem"`
		} `json:"package"`
		Ranges            []model.AffectedRange `json:"ranges"`
		Versions          []string              `json:"versions"`
		Severity          []rawSeverity         `json:"severity"`
		DatabaseSpecific  map[string]any        `json:"database_specific"`
		EcosystemSpecific map[string]any        `json:"ecosystem_specific"`
	} `json:"affected"`
}

func normalize(raw rawAdvisory, packageName string) model.Advisory {
	a := model.Advisory{ID: raw.ID, Aliases: raw.Aliases, Summary: raw.Summary, Withdrawn: raw.Withdrawn,
		References: raw.References, Severity: "unknown", SeverityEvidence: []model.SeverityEvidence{}, Affected: []model.Affected{}}
	if a.Aliases == nil {
		a.Aliases = []string{}
	}
	if a.References == nil {
		a.References = []model.Reference{}
	}
	add := func(kind, value, source string) {
		if value == "" {
			return
		}
		a.SeverityEvidence = append(a.SeverityEvidence, model.SeverityEvidence{Type: kind, Value: value, Source: source})
		severity := classify(kind, value)
		if severityRank(severity) > severityRank(a.Severity) {
			a.Severity = severity
		}
	}
	label := func(data map[string]any, source string) {
		if value, ok := data["severity"].(string); ok {
			add("label", value, source)
		}
	}
	for _, s := range raw.Severity {
		add(s.Type, s.Score, "advisory.severity")
	}
	label(raw.DatabaseSpecific, "advisory.database_specific.severity")
	for _, affected := range raw.Affected {
		a.Affected = append(a.Affected, model.Affected{Name: affected.Package.Name, Ecosystem: affected.Package.Ecosystem,
			Ranges: affected.Ranges, Versions: affected.Versions})
		if affected.Package.Name != packageName || affected.Package.Ecosystem != "npm" {
			continue
		}
		for _, s := range affected.Severity {
			add(s.Type, s.Score, "affected.severity")
		}
		label(affected.DatabaseSpecific, "affected.database_specific.severity")
		label(affected.EcosystemSpecific, "affected.ecosystem_specific.severity")
	}
	return a
}

func classify(kind, value string) string {
	if kind == "label" {
		label := strings.ToLower(strings.TrimSpace(value))
		switch label {
		case "critical", "high", "medium", "low", "none":
			return label
		case "moderate":
			return "medium"
		}
		return "unknown"
	}
	score := -1.0
	if kind == "CVSS_V3" && strings.HasPrefix(value, "CVSS:3.1/") {
		if vector, err := cvss31.ParseVector(value); err == nil {
			score = vector.BaseScore()
		}
	} else if kind == "CVSS_V3" && strings.HasPrefix(value, "CVSS:3.0/") {
		if vector, err := cvss30.ParseVector(value); err == nil {
			score = vector.BaseScore()
		}
	} else if kind == "CVSS_V4" && strings.HasPrefix(value, "CVSS:4.0/") {
		// Normalize base metrics only; avoid mixing supplied environmental metrics
		// with our separate project-context policy.
		base := []string{"CVSS:4.0"}
		mandatory := map[string]bool{"AV": true, "AC": true, "AT": true, "PR": true, "UI": true, "VC": true, "VI": true, "VA": true, "SC": true, "SI": true, "SA": true}
		if _, err := cvss40.ParseVector(value); err == nil {
			for _, metric := range strings.Split(value, "/")[1:] {
				if mandatory[strings.SplitN(metric, ":", 2)[0]] {
					base = append(base, metric)
				}
			}
			if vector, err := cvss40.ParseVector(strings.Join(base, "/")); err == nil {
				score = vector.Score()
			}
		}
	}
	switch {
	case score < 0:
		return "unknown"
	case score >= 9:
		return "critical"
	case score >= 7:
		return "high"
	case score >= 4:
		return "medium"
	case score > 0:
		return "low"
	default:
		return "none"
	}
}

func severityRank(s string) int {
	switch s {
	case "critical":
		return 5
	case "high":
		return 4
	case "medium":
		return 3
	case "low":
		return 2
	case "none":
		return 1
	default:
		return 0
	}
}
