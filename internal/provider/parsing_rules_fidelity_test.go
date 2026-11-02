// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
)

// TestParsingRulesFakeMatchesRecordedBackend checks parsingRulesFake against
// fidelity/recorded.json. Record that file with RECORD_PARSING_RULES_FIDELITY=1 against a tenant,
// then keep the fake in line with those answers. This test skips when recorded.json does not exist.
func TestParsingRulesFakeMatchesRecordedBackend(t *testing.T) {
	cases, recorded := loadFidelityAt(t, parsingRulesGoldenDir)
	for ci, c := range cases.Cases {
		t.Run(c.Name, func(t *testing.T) {
			fake := newParsingRulesFakeState()
			rc := recorded.Cases[ci]
			if rc.Name != c.Name || len(rc.Steps) != len(c.Steps) {
				t.Fatalf("recorded.json does not match cases.json for %q: record again", c.Name)
			}
			vars, tokens := map[string]string{}, uuidTokens{}
			for si, s := range c.Steps {
				label := fmt.Sprintf("step %d: %s %s", si+1, s.Method, s.Path)
				body := s.Body
				if body != nil {
					body = substJSON(t, body, vars)
				}
				fake.mu.Lock()
				status, resp := fake.handle(s.Method, parsingRulesBasePath+substString(s.Path, vars), body)
				fake.mu.Unlock()
				got := roundTrip(t, resp)
				if s.Capture != "" && status == http.StatusOK {
					vars[s.Capture] = parsingRulesIDOf(got)
				}
				got = parsingRulesNormalize(unsubstJSON(t, got, vars), tokens)
				compareFidelityStep(t, label, status, got, rc.Steps[si].Status, rc.Steps[si].Response)
			}
		})
	}
}

// TestRecordParsingRulesFidelity sends fidelity/cases.json to a real tenant and writes
// fidelity/recorded.json. Set RECORD_PARSING_RULES_FIDELITY=1 with CORALOGIX_API_KEY and
// CORALOGIX_ENV (or CX_REGION). Each case scopes its groups to an application that sends no logs,
// and the rule groups that the cases create are deleted afterwards. A Delete lowers the order of
// every group above the deleted one, so the cleanup first moves a group to the end.
func TestRecordParsingRulesFidelity(t *testing.T) {
	if os.Getenv("RECORD_PARSING_RULES_FIDELITY") != "1" {
		t.Skip("set RECORD_PARSING_RULES_FIDELITY=1 with CORALOGIX_API_KEY and CORALOGIX_ENV to record against a tenant")
	}
	apiKey := os.Getenv("CORALOGIX_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("CX_API_KEY")
	}
	if apiKey == "" {
		t.Fatal("CORALOGIX_API_KEY is required to record")
	}
	region := os.Getenv("CORALOGIX_ENV")
	if region == "" {
		region = os.Getenv("CX_REGION")
	}
	if region == "" {
		t.Fatal("CORALOGIX_ENV or CX_REGION is required to record")
	}
	base, ok := cxsdkOpenapi.URLFromRegion(strings.ToLower(region))
	if !ok {
		t.Fatalf("unknown region %q", region)
	}

	rawCases, err := os.ReadFile(filepath.Join(parsingRulesGoldenDir, "fidelity", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases fidelityCases
	if err := json.Unmarshal(rawCases, &cases); err != nil {
		t.Fatal(err)
	}

	var created []string
	t.Cleanup(func() {
		if err := cleanupRecordedRuleGroups(base, apiKey, created); err != nil {
			t.Error(err)
		}
	})

	out := fidelityRecorded{}
	for _, c := range cases.Cases {
		rec := struct {
			Name  string `json:"name"`
			Steps []struct {
				Status   int `json:"status"`
				Response any `json:"response"`
			} `json:"steps"`
		}{Name: c.Name}
		vars, tokens := map[string]string{}, uuidTokens{}
		for _, s := range c.Steps {
			status, got := recordParsingRulesStep(t, base, apiKey, s, vars, tokens, &created)
			if (status == http.StatusForbidden || status == http.StatusUnauthorized) && s.Method == http.MethodPost {
				t.Fatalf("the rule groups API returned %d on %s %s. Recording needs a management API key that can manage parsing rules.", status, s.Method, s.Path)
			}
			rec.Steps = append(rec.Steps, struct {
				Status   int `json:"status"`
				Response any `json:"response"`
			}{Status: status, Response: got})
		}
		out.Cases = append(out.Cases, rec)
	}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parsingRulesGoldenDir, "fidelity", "recorded.json"), append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func recordParsingRulesStep(t *testing.T, base, apiKey string, s fidelityStep, vars map[string]string, tokens uuidTokens, created *[]string) (int, any) {
	t.Helper()
	var body any
	if s.Body != nil {
		body = substJSON(t, s.Body, vars)
	}
	status, parsed, err := parsingRulesCall(base, apiKey, s.Method, substString(s.Path, vars), body)
	if err != nil {
		t.Fatal(err)
	}
	if s.Capture != "" && status == http.StatusOK {
		if id := parsingRulesIDOf(parsed); id != "" {
			vars[s.Capture] = id
			*created = append(*created, id)
		}
	}
	return status, parsingRulesNormalize(unsubstJSON(t, parsed, vars), tokens)
}

func parsingRulesCall(base, apiKey, method, path string, body any) (int, any, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, strings.TrimRight(base, "/")+parsingRulesBasePath+path, reader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}
	var parsed any = map[string]any{}
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &parsed); err != nil {
			parsed = map[string]any{"raw": string(raw)}
		}
	}
	return resp.StatusCode, parsed, nil
}

var parsingRulesCompany = regexp.MustCompile(`(?i)(company ?id:? ?)\d+`)

// uuidTokens gives each id of one case a token, in the order of first appearance, so that a
// comparison still sees whether a step keeps an id or makes a new one.
type uuidTokens map[string]string

func (u uuidTokens) token(id string) string {
	if t, ok := u[id]; ok {
		return t
	}
	t := fmt.Sprintf("<uuid-%d>", len(u)+1)
	u[id] = t
	return t
}

// parsingRulesNormalize replaces what differs in every run: the ids that every write gives the
// subgroups and rules (a token per id of the case), and a company id in a message. It walks the
// keys in order, so the tokens do not depend on the order of a map.
func parsingRulesNormalize(v any, tokens uuidTokens) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for _, k := range slices.Sorted(maps.Keys(x)) {
			e := x[k]
			switch s, _ := e.(string); {
			case k == "id" && uuidPattern.MatchString(s):
				out[k] = tokens.token(s)
			case k == "message":
				s = uuidPattern.ReplaceAllStringFunc(s, tokens.token)
				out[k] = parsingRulesCompany.ReplaceAllString(s, "${1}<company>")
			default:
				out[k] = parsingRulesNormalize(e, tokens)
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = parsingRulesNormalize(e, tokens)
		}
		return out
	}
	return v
}

// cleanupRecordedRuleGroups deletes the groups that the recording made. It moves each group to
// the end first, because a Delete lowers the order of every group above it.
func cleanupRecordedRuleGroups(base, apiKey string, ids []string) error {
	var failed []string
	for _, id := range ids {
		status, got, err := parsingRulesCall(base, apiKey, http.MethodGet, "/"+id, nil)
		if err != nil || status == http.StatusNotFound {
			continue
		}
		if status != http.StatusOK {
			failed = append(failed, id)
			continue
		}
		group, _ := got.(map[string]any)["ruleGroup"].(map[string]any)
		group["order"] = 0
		delete(group, "id")
		// Deleting a group that did not move to the end would lower the order of the groups above it.
		if status, _, err := parsingRulesCall(base, apiKey, http.MethodPut, "/"+id, parsingRulesRequestOf(group)); err != nil || status != http.StatusOK {
			failed = append(failed, id)
			continue
		}
		if status, _, err := parsingRulesCall(base, apiKey, http.MethodDelete, "/"+id, nil); err != nil || status != http.StatusOK {
			failed = append(failed, id)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("could not move to the end and delete the recorded rule groups %v: move each one last (order 0) and delete it by hand", failed)
	}
	return nil
}

// parsingRulesRequestOf turns a rule group of a Get into a request: the ids go.
func parsingRulesRequestOf(group map[string]any) map[string]any {
	for _, raw := range asList(group["ruleSubgroups"]) {
		sg, _ := raw.(map[string]any)
		delete(sg, "id")
		for _, raw := range asList(sg["rules"]) {
			r, _ := raw.(map[string]any)
			delete(r, "id")
			if p, ok := r["parameters"].(map[string]any)["jsonParseParameters"].(map[string]any); ok {
				p["escapedValue"] = true
			}
		}
	}
	return group
}
