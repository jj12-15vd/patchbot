// Package demo supplies fictional advisories for repeatable offline demos/tests.
package demo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
)

func Server(outage bool) *httptest.Server {
	ids := map[string]string{"demo-parser@1.0.0": "DEMO-PARSER-1", "demo-parser@2.0.0": "DEMO-PARSER-2", "demo-build-tool@1.0.0": "DEMO-BUILD", "demo-shared@1.0.0": "DEMO-SHARED", "@demo/scoped@1.0.0": "DEMO-UNKNOWN"}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if outage {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if r.URL.Path == "/v1/querybatch" {
			var payload struct {
				Queries []struct {
					Version string `json:"version"`
					Package struct {
						Name string `json:"name"`
					} `json:"package"`
				} `json:"queries"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, "bad request", 400)
				return
			}
			results := []map[string]any{}
			for _, q := range payload.Queries {
				vulns := []map[string]string{}
				if id := ids[q.Package.Name+"@"+q.Version]; id != "" {
					vulns = append(vulns, map[string]string{"id": id})
				}
				results = append(results, map[string]any{"vulns": vulns})
			}
			json.NewEncoder(w).Encode(map[string]any{"results": results})
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/v1/vulns/")
		name, severity, fixed := "", "HIGH", ""
		switch id {
		case "DEMO-PARSER-1":
			name, fixed = "demo-parser", "1.0.1"
		case "DEMO-PARSER-2":
			name, fixed = "demo-parser", "2.0.1"
		case "DEMO-BUILD":
			name = "demo-build-tool"
		case "DEMO-SHARED":
			name, severity = "demo-shared", "MODERATE"
		case "DEMO-UNKNOWN":
			name, severity = "@demo/scoped", ""
		default:
			http.Error(w, "missing", 404)
			return
		}
		events := []map[string]string{{"introduced": "0"}}
		if fixed != "" {
			events = append(events, map[string]string{"fixed": fixed})
		}
		json.NewEncoder(w).Encode(map[string]any{"id": id, "summary": "Fictional demonstration advisory; not a real vulnerability.",
			"database_specific": map[string]string{"severity": severity}, "affected": []any{map[string]any{
				"package": map[string]string{"name": name, "ecosystem": "npm"},
				"ranges":  []any{map[string]any{"type": "SEMVER", "events": events}},
			}}})
	}))
}
