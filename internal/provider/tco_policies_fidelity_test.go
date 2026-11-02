// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an AS IS BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
)

const tcoFidelityDir = "testdata/tco_policies_baseline"

// TestTCOPoliciesFakeMatchesRecordedBackend checks the fake TCO policies backend against
// fidelity/recorded.json. Record that file against a tenant (TestRecordTCOPoliciesFidelity), then
// keep the fake in line with those answers. This test skips when recorded.json does not exist.
func TestTCOPoliciesFakeMatchesRecordedBackend(t *testing.T) {
	cases, recorded := loadFidelityAt(t, tcoFidelityDir)
	for ci, c := range cases.Cases {
		t.Run(c.Name, func(t *testing.T) {
			rc := recorded.Cases[ci]
			if rc.Name != c.Name || len(rc.Steps) != len(c.Steps) {
				t.Fatalf("recorded.json does not match cases.json for %q: record again", c.Name)
			}
			// The recorder ran the cases in order on one tenant, so a case starts with the policies
			// that the cases before it left. Replay them on a new fake, so that -run selects one case.
			fake := &tcoFake{}
			send := func(s fidelityStep) (int, any) {
				fake.mu.Lock()
				defer fake.mu.Unlock()
				return fake.handle(s.Method, tcoBasePath+s.Path, roundTrip(t, s.Body))
			}
			for _, prior := range cases.Cases[:ci] {
				for _, s := range prior.Steps {
					send(s)
				}
			}
			for si, s := range c.Steps {
				label := fmt.Sprintf("step %d: %s %s %s", si+1, s.Method, s.Path, compact(s.Body))
				status, resp := send(s)
				compareFidelityStep(t, label, status, tcoMask(roundTrip(t, resp)), rc.Steps[si].Status, rc.Steps[si].Response)
			}
		})
	}
}

// TestRecordTCOPoliciesFidelity sends fidelity/cases.json to a real tenant and writes
// fidelity/recorded.json. Set RECORD_TCO_POLICIES_FIDELITY=1 with CORALOGIX_API_KEY and
// CORALOGIX_ENV (or CX_REGION).
//
// The cases replace every log, span, and RUM policy of the tenant. The recorder saves the log and
// span policies first and writes them back at the end; they get new ids. The API cannot read RUM
// policies on every region, so RUM policies are not saved: record on a tenant without them.
func TestRecordTCOPoliciesFidelity(t *testing.T) {
	if os.Getenv("RECORD_TCO_POLICIES_FIDELITY") != "1" {
		t.Skip("set RECORD_TCO_POLICIES_FIDELITY=1 with CORALOGIX_API_KEY and CORALOGIX_ENV to record against a tenant; it replaces the TCO policies of the tenant and puts the log and span policies back")
	}
	base, apiKey := tcoRecordingTarget(t)

	rawCases, err := os.ReadFile(filepath.Join(tcoFidelityDir, "fidelity", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases fidelityCases
	if err := json.Unmarshal(rawCases, &cases); err != nil {
		t.Fatal(err)
	}

	restore := map[string]any{}
	for source, overwrite := range map[string]string{"SOURCE_TYPE_LOGS": "/log-policies/v1", "SOURCE_TYPE_SPANS": "/span-policies/v1"} {
		status, got := amSend(t, base+"/policies/v1?source_type="+source, apiKey, http.MethodGet, nil)
		if status != http.StatusOK {
			t.Fatalf("cannot save the %s policies of the tenant (%d): %s", source, status, compact(got))
		}
		restore[overwrite] = tcoRestoreBody(t, got, source)
	}
	t.Cleanup(func() {
		for path, body := range restore {
			if status, got := amSend(t, base+path, apiKey, http.MethodPut, body); status != http.StatusOK {
				t.Errorf("RESTORE FAILED for %s (%d): %s. The saved policies: %s", path, status, compact(got), compact(body))
			}
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
		for _, s := range c.Steps {
			status, got := amSend(t, base+s.Path, apiKey, s.Method, s.Body)
			if status == http.StatusForbidden || status == http.StatusUnauthorized {
				t.Fatalf("the API returned %d. Recording needs a management API key with TCO policy permissions.", status)
			}
			rec.Steps = append(rec.Steps, struct {
				Status   int `json:"status"`
				Response any `json:"response"`
			}{Status: status, Response: tcoMask(got)})
		}
		out.Cases = append(out.Cases, rec)
	}
	// Keep "<id>" and the expressions readable: no HTML escaping.
	var raw bytes.Buffer
	enc := json.NewEncoder(&raw)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tcoFidelityDir, "fidelity", "recorded.json"), raw.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// tcoRecordingTarget returns the URL of the dataplans API and the API key.
func tcoRecordingTarget(t *testing.T) (base, apiKey string) {
	t.Helper()
	apiKey = cmp.Or(os.Getenv("CORALOGIX_API_KEY"), os.Getenv("CX_API_KEY"))
	region := cmp.Or(os.Getenv("CORALOGIX_ENV"), os.Getenv("CX_REGION"))
	if apiKey == "" || region == "" {
		t.Fatal("CORALOGIX_API_KEY and CORALOGIX_ENV (or CX_REGION) are required to record")
	}
	url, ok := cxsdkOpenapi.URLFromRegion(strings.ToLower(region))
	if !ok {
		t.Fatalf("unknown region %q", region)
	}
	return strings.TrimRight(url, "/") + tcoBasePath, apiKey
}

// tcoRestoreBody turns the policies that a Get returned into the overwrite request that creates them
// again.
func tcoRestoreBody(t *testing.T, got any, source string) map[string]any {
	t.Helper()
	m, _ := got.(map[string]any)
	list, _ := m["policies"].([]any)
	policies := []any{}
	for _, p := range list {
		policy, _ := p.(map[string]any)
		generic := map[string]any{"disabled": policy["enabled"] != true}
		for _, k := range []string{"name", "description", "priority", "applicationRule", "subsystemRule", "archiveRetention", "priorityOverride", "targets"} {
			if v, ok := policy[k]; ok {
				generic[k] = v
			}
		}
		if _, ok := generic["description"]; !ok {
			generic["description"] = ""
		}
		// A policy-level priority means the backend added the targets; the API refuses both.
		if p, _ := policy["priority"].(string); (p != "" && p != "PRIORITY_TYPE_UNSPECIFIED") || policy["priorityOverride"] != nil {
			delete(generic, "targets")
		}
		entry := map[string]any{"policy": generic}
		if source == "SOURCE_TYPE_LOGS" {
			entry["logRules"] = cmp.Or[any](policy["logRules"], map[string]any{})
		} else {
			entry["spanRules"] = cmp.Or[any](policy["spanRules"], map[string]any{})
		}
		policies = append(policies, entry)
	}
	return map[string]any{"policies": policies}
}

// tcoMask replaces what differs in every run and what belongs to the tenant: policy ids, the
// company id, and the times.
func tcoMask(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, e := range x {
			switch k {
			case "companyId":
				out[k] = "<companyId>"
			case "createdAt", "updatedAt":
				out[k] = "<time>"
			case "id":
				if s, ok := e.(string); ok && uuidPattern.MatchString(s) && x["name"] != nil {
					out[k] = "<id>"
					continue
				}
				out[k] = tcoMask(e)
			default:
				out[k] = tcoMask(e)
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = tcoMask(e)
		}
		return out
	}
	return v
}
