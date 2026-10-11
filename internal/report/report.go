package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"

	"patchbot/internal/model"
)

func Write(w io.Writer, format string, r model.Report) error {
	if format == "json" {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(r)
	}
	// Buffer the text so writer failures are returned, just like JSON failures.
	var b strings.Builder
	fmt.Fprintf(&b, "PatchBot — %s\nTarget: %s\nInventory: installed npm dependencies\nRules: %s\n", r.Status, clean(r.Target), r.RulesVersion)
	c := r.Analysis.Coverage
	fmt.Fprintf(&b, "Coverage: %d packages, %d queried versions, %d source files; %d failed lookups, %d unqueryable packages, %d extraneous packages\n", c.Packages, c.QueriedVersions, c.ScannedFiles, c.FailedQueries, c.Unqueryable, c.Extraneous)
	for _, name := range []string{"dependencies", "source", "vulnerabilities"} {
		stage := r.Analysis.Stages[name]
		fmt.Fprintf(&b, "  %s: %s\n", name, stage.Status)
	}
	for _, f := range r.Findings {
		fmt.Fprintf(&b, "\n[%s] %s@%s — %s\n", f.Priority, clean(f.Package.Name), clean(f.Package.Version), clean(f.Advisory.ID))
		fmt.Fprintf(&b, "  Advisory severity: %s\n  Summary: %s\n", f.Advisory.Severity, clean(f.Advisory.Summary))
		fmt.Fprintf(&b, "  Installed at: %s\n  Dependency path: %s\n", clean(f.Package.Location), clean(strings.Join(f.Package.Path, " -> ")))
		fmt.Fprintf(&b, "  Context: direct=%t production=%t development=%t optional=%t peer=%t unknown=%t\n", f.Package.Direct, f.Package.Context.Production, f.Package.Context.Development, f.Package.Context.Optional, f.Package.Context.Peer, f.Package.Context.Unknown)
		fmt.Fprintf(&b, "  Import evidence: %s\n  Affected-function reachability: %s\n", f.ImportState, f.Reachability)
		for _, evidence := range f.Imports {
			fmt.Fprintf(&b, "    %s:%d (%s, type-only=%t)\n", clean(evidence.File), evidence.Line, evidence.Kind, evidence.TypeOnly)
		}
		for _, reason := range f.Reasons {
			fmt.Fprintf(&b, "  Reason: %s\n", clean(reason))
		}
		fmt.Fprintf(&b, "  Rules: %s\n", clean(strings.Join(f.RuleIDs, ", ")))
		fixFound := false
		for _, affected := range f.Advisory.Affected {
			if affected.Name != f.Package.Name || affected.Ecosystem != "npm" {
				continue
			}
			for _, rng := range affected.Ranges {
				for _, event := range rng.Events {
					if fixed := event["fixed"]; fixed != "" {
						fmt.Fprintf(&b, "  Advisory fixed boundary (%s): %s; review compatibility before upgrading\n", rng.Type, clean(fixed))
						fixFound = true
					}
				}
			}
		}
		if !fixFound {
			fmt.Fprintln(&b, "  Fix information: no fixed boundary supplied for this npm package.")
		}
		for _, ref := range f.Advisory.References {
			fmt.Fprintf(&b, "  Reference: %s\n", clean(ref.URL))
		}
		for _, limit := range f.Limitations {
			fmt.Fprintf(&b, "  Limitation: %s\n", clean(limit))
		}
	}
	for _, warning := range r.Analysis.Warnings {
		fmt.Fprintf(&b, "\nCoverage warning [%s]: %s\n", clean(warning.Code), clean(warning.Message))
	}
	for _, err := range r.Errors {
		fmt.Fprintf(&b, "\nError [%s]: %s\n", clean(err.Stage), clean(err.Message))
	}
	if r.Status == "complete" && len(r.Findings) == 0 {
		fmt.Fprintln(&b, "\nNo known vulnerabilities found in the queried dependencies. This does not establish that the project is secure.")
	} else if r.Status != "complete" {
		fmt.Fprintln(&b, "\nScan incomplete. Missing evidence or lookups must not be treated as a clean security result.")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}
