package osv

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"patchbot/internal/model"
)

func testPackage(name, version string) model.Package {
	return model.Package{Name: name, Version: version, Queryable: true}
}

func TestLookupPaginationDeduplicationAndWithdrawal(t *testing.T) {
	var batches, details atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/querybatch" {
			batches.Add(1)
			var body struct {
				Queries []query `json:"queries"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
				http.Error(w, "bad", 400)
				return
			}
			if len(body.Queries) != 1 {
				t.Errorf("expected one deduplicated query, got %d", len(body.Queries))
			}
			q := body.Queries[0]
			if q.Package.Ecosystem != "npm" || q.Version != "1.0.0" {
				t.Errorf("wrong query: %+v", q)
			}
			if q.PageToken == "" {
				fmt.Fprint(w, `{"results":[{"vulns":[{"id":"DEMO-A"}],"next_page_token":"page-2"}]}`)
			} else {
				fmt.Fprint(w, `{"results":[{"vulns":[{"id":"DEMO-A"},{"id":"DEMO-WITHDRAWN"}]}]}`)
			}
			return
		}
		details.Add(1)
		if strings.HasSuffix(r.URL.Path, "DEMO-WITHDRAWN") {
			fmt.Fprint(w, `{"id":"DEMO-WITHDRAWN","withdrawn":"2026-01-01T00:00:00Z"}`)
		} else {
			fmt.Fprint(w, `{"id":"DEMO-A","aliases":["CVE-DEMO"],"database_specific":{"severity":"HIGH"},"affected":[{"package":{"name":"demo","ecosystem":"npm"}}]}`)
		}
	}))
	defer server.Close()
	p := testPackage("demo", "1.0.0")
	result := (Client{BaseURL: server.URL}).Lookup(context.Background(), []model.Package{p, p})
	if result.Failed != 0 || result.Queried != 1 || result.Withdrawn != 1 || batches.Load() != 2 || details.Load() != 2 {
		t.Fatalf("result %+v batches=%d details=%d", result, batches.Load(), details.Load())
	}
	a := result.Advisories[Key(p)]
	if len(a) != 1 || a[0].Severity != "high" || len(a[0].Aliases) != 1 {
		t.Fatalf("advisories: %+v", a)
	}
}

func TestBatchFailureIsNeverAnEmptySuccessfulLookup(t *testing.T) {
	for _, payload := range []string{`not json`, `{"results":[]}`, `{"results":[{"vulns":[{}]}]}`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, payload) }))
		result := (Client{BaseURL: server.URL}).Lookup(context.Background(), []model.Package{testPackage("demo", "1.0.0")})
		server.Close()
		if result.Failed != 1 || len(result.Errors) == 0 {
			t.Fatalf("malformed response considered success: %s %+v", payload, result)
		}
	}
}

func TestRateLimitRetryAndCancellation(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			return
		}
		fmt.Fprint(w, `{"results":[{}]}`)
	}))
	defer server.Close()
	client := Client{BaseURL: server.URL}
	result := client.Lookup(context.Background(), []model.Package{testPackage("demo", "1.0.0")})
	if result.Failed != 0 || requests.Load() != 2 {
		t.Fatalf("retry result: %+v requests=%d", result, requests.Load())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result = client.Lookup(ctx, []model.Package{testPackage("demo", "1.0.0")})
	if result.Failed != 1 {
		t.Fatal("cancellation considered complete")
	}
}

func TestKeepSuccessfulFindingsWhenOtherDetailsFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/querybatch":
			fmt.Fprint(w, `{"results":[{"vulns":[{"id":"GOOD"},{"id":"BAD"}]}]}`)
		case "/v1/vulns/GOOD":
			fmt.Fprint(w, `{"id":"GOOD","affected":[{"package":{"name":"demo","ecosystem":"npm"}}]}`)
		default:
			http.Error(w, "missing", 404)
		}
	}))
	defer server.Close()
	p := testPackage("demo", "1.0.0")
	result := (Client{BaseURL: server.URL}).Lookup(context.Background(), []model.Package{p})
	if result.Failed != 1 || len(result.Advisories[Key(p)]) != 1 {
		t.Fatalf("lost partial evidence: %+v", result)
	}
}

func TestTimeoutAndRepeatedPaginationAreIncomplete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"results":[{"next_page_token":"same"}]}`)
	}))
	defer server.Close()
	result := (Client{BaseURL: server.URL}).Lookup(context.Background(), []model.Package{testPackage("demo", "1.0.0")})
	if result.Failed != 1 {
		t.Fatal("repeated token not detected")
	}
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bound the handler too: a client cancellation before it reads the request
		// body must not leave the test server's shutdown waiting indefinitely.
		select {
		case <-r.Context().Done():
		case <-time.After(100 * time.Millisecond):
		}
	}))
	defer slow.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	result = (Client{BaseURL: slow.URL}).Lookup(ctx, []model.Package{testPackage("demo", "1.0.0")})
	if result.Failed != 1 {
		t.Fatal("timeout considered successful")
	}
}

func TestSeverityVectorsAndUnknownData(t *testing.T) {
	for _, tc := range []struct{ kind, value, want string }{
		{"label", "MODERATE", "medium"}, {"label", "invented", "unknown"},
		{"CVSS_V3", "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H", "critical"},
		{"CVSS_V3", "CVSS:3.0/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H", "critical"},
		{"CVSS_V4", "CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:H/VI:H/VA:H/SC:H/SI:H/SA:H", "critical"},
		{"CVSS_V3", "CVSS:3.1/broken", "unknown"}, {"CVSS_V2", "AV:N/AC:L", "unknown"},
	} {
		if got := classify(tc.kind, tc.value); got != tc.want {
			t.Errorf("%s: got %s want %s", tc.value, got, tc.want)
		}
	}
}
