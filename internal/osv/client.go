// Package osv queries known advisories; it does not infer exploitability.
package osv

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"patchbot/internal/model"
)

const DefaultURL = "https://api.osv.dev"

type Client struct {
	BaseURL string
	HTTP    *http.Client
}
type Result struct {
	Advisories map[string][]model.Advisory
	Errors     []model.Warning
	Queried    int
	Failed     int
	Withdrawn  int
}

func Key(p model.Package) string { return p.Name + "\x00" + p.Version }

type query struct {
	Package struct {
		Name      string `json:"name"`
		Ecosystem string `json:"ecosystem"`
	} `json:"package"`
	Version   string `json:"version"`
	PageToken string `json:"page_token,omitempty"`
}
type batchResult struct {
	Vulns []struct {
		ID string `json:"id"`
	} `json:"vulns"`
	Next string `json:"next_page_token"`
}

func (c Client) Lookup(ctx context.Context, packages []model.Package) Result {
	result := Result{Advisories: map[string][]model.Advisory{}, Errors: []model.Warning{}}
	unique := map[string]model.Package{}
	for _, p := range packages {
		if p.Queryable {
			unique[Key(p)] = p
		}
	}
	keys := make([]string, 0, len(unique))
	for key := range unique {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result.Queried = len(keys)
	failed := map[string]bool{}
	ids := map[string]map[string]bool{}
	recordFailure := func(key, message string) {
		failed[key] = true
		result.Errors = append(result.Errors, model.Warning{Stage: "vulnerabilities", Code: "OSV_LOOKUP", Message: unique[key].Name + "@" + unique[key].Version + ": " + message})
	}
	for start := 0; start < len(keys); start += 128 {
		end := min(start+128, len(keys))
		pending := append([]string(nil), keys[start:end]...)
		tokens := map[string]string{}
		seenTokens := map[string]map[string]bool{}
		for len(pending) > 0 {
			queries := make([]query, 0, len(pending))
			for _, key := range pending {
				q := query{Version: unique[key].Version, PageToken: tokens[key]}
				q.Package.Name, q.Package.Ecosystem = unique[key].Name, "npm"
				queries = append(queries, q)
			}
			var response struct {
				Results []batchResult `json:"results"`
			}
			err := c.request(ctx, "POST", "/v1/querybatch", map[string]any{"queries": queries}, &response)
			if err != nil || len(response.Results) != len(pending) {
				message := "batch response length does not match query count"
				if err != nil {
					message = err.Error()
				}
				for _, key := range pending {
					recordFailure(key, message)
				}
				break
			}
			next := []string{}
			for i, item := range response.Results {
				key := pending[i]
				if ids[key] == nil {
					ids[key] = map[string]bool{}
				}
				for _, vuln := range item.Vulns {
					if vuln.ID == "" {
						recordFailure(key, "advisory ID missing in batch result")
						continue
					}
					ids[key][vuln.ID] = true
				}
				if item.Next != "" {
					if seenTokens[key] == nil {
						seenTokens[key] = map[string]bool{}
					}
					if seenTokens[key][item.Next] {
						recordFailure(key, "OSV repeated a pagination token")
						continue
					}
					seenTokens[key][item.Next] = true
					tokens[key] = item.Next
					next = append(next, key)
				}
			}
			pending = next
		}
	}
	cache := map[string]rawAdvisory{}
	cacheErrors := map[string]error{}
	withdrawn := map[string]bool{}
	for _, key := range keys {
		advisoryIDs := make([]string, 0, len(ids[key]))
		for id := range ids[key] {
			advisoryIDs = append(advisoryIDs, id)
		}
		sort.Strings(advisoryIDs)
		for _, id := range advisoryIDs {
			raw, exists := cache[id]
			fetchErr, attempted := cacheErrors[id]
			if !exists && !attempted {
				fetchErr = c.request(ctx, "GET", "/v1/vulns/"+url.PathEscape(id), nil, &raw)
				if fetchErr == nil && raw.ID != id {
					fetchErr = fmt.Errorf("advisory identity mismatch")
				}
				cacheErrors[id] = fetchErr
				if fetchErr == nil {
					cache[id] = raw
				}
			}
			if fetchErr != nil {
				recordFailure(key, "fetch "+id+": "+fetchErr.Error())
				continue
			}
			if raw.Withdrawn != "" {
				withdrawn[id] = true
				continue
			}
			matched := false
			for _, affected := range raw.Affected {
				if affected.Package.Name == unique[key].Name && affected.Package.Ecosystem == "npm" {
					matched = true
				}
			}
			if !matched {
				recordFailure(key, "advisory "+id+" lacks matching npm package information")
				continue
			}
			result.Advisories[key] = append(result.Advisories[key], normalize(raw, unique[key].Name))
		}
	}
	result.Failed, result.Withdrawn = len(failed), len(withdrawn)
	return result
}

func (c Client) request(ctx context.Context, method, path string, body any, response any) error {
	base := c.BaseURL
	if base == "" {
		base = DefaultURL
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(base, "/")+path, bytes.NewReader(data))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "PatchBot/0.1.0-prototype")
		res, err := client.Do(req)
		if err != nil {
			if attempt == 2 || ctx.Err() != nil {
				return fmt.Errorf("OSV request failed: %w", err)
			}
			if err := wait(ctx, time.Duration(attempt+1)*100*time.Millisecond); err != nil {
				return err
			}
			continue
		}
		if res.StatusCode == 429 || res.StatusCode >= 500 {
			res.Body.Close()
			if attempt == 2 {
				return fmt.Errorf("OSV HTTP %d after bounded retries", res.StatusCode)
			}
			delay := time.Duration(attempt+1) * 100 * time.Millisecond
			if value := res.Header.Get("Retry-After"); value != "" {
				if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
					delay = time.Duration(min(seconds, 5)) * time.Second
				} else if when, err := http.ParseTime(value); err == nil {
					delay = max(0, min(time.Until(when), 5*time.Second))
				}
			}
			if err := wait(ctx, delay); err != nil {
				return err
			}
			continue
		}
		if res.StatusCode != 200 {
			res.Body.Close()
			return fmt.Errorf("OSV HTTP %d", res.StatusCode)
		}
		payload, err := io.ReadAll(io.LimitReader(res.Body, (8<<20)+1))
		res.Body.Close()
		if err != nil {
			return fmt.Errorf("read OSV response: %w", err)
		}
		if len(payload) > 8<<20 {
			return fmt.Errorf("OSV response exceeds 8 MiB limit")
		}
		if err := json.Unmarshal(payload, response); err != nil {
			return fmt.Errorf("invalid OSV JSON: %w", err)
		}
		return nil
	}
	return fmt.Errorf("OSV retries exhausted")
}

func wait(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
