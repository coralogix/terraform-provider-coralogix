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
			fake := &tcoFake{archiveRetentions: map[string]bool{}}
			for _, id := range tcoArchiveRetentionPlaceholders {
				fake.archiveRetentions[id] = true
			}
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
				got, want := tcoMask(roundTrip(t, resp)), roundTrip(t, rc.Steps[si].Response)
				sortResponseTagRules(got)
				sortResponseTagRules(want)
				compareFidelityStep(t, label, status, got, rc.Steps[si].Status, want)
			}
		})
	}
}

// TestRecordTCOPoliciesFidelity sends fidelity/cases.json to a real tenant and writes
// fidelity/recorded.json. Set RECORD_TCO_POLICIES_FIDELITY=1 with CORALOGIX_API_KEY and
// CORALOGIX_ENV (or CX_REGION).
//
// The cases replace every log, span, and RUM policy of the tenant. The recorder saves all three
// first and writes them back at the end; they get new ids. When it cannot read one of them, it stops
// before it sends anything.
func TestRecordTCOPoliciesFidelity(t *testing.T) {
	if os.Getenv("RECORD_TCO_POLICIES_FIDELITY") != "1" {
		t.Skip("set RECORD_TCO_POLICIES_FIDELITY=1 with CORALOGIX_API_KEY and CORALOGIX_ENV to record against a tenant; it replaces the TCO policies of the tenant and puts the log, span, and RUM policies back")
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

	retentions := tcoTenantArchiveRetentions(t, base, apiKey)
	restore := map[string]any{}
	for source, overwrite := range map[string]string{"SOURCE_TYPE_LOGS": "/log-policies/v1", "SOURCE_TYPE_SPANS": "/span-policies/v1", "SOURCE_TYPE_RUM": "/rum-policies/v1"} {
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
			status, got := amSend(t, base+s.Path, apiKey, s.Method, tcoReplaceStrings(t, s.Body, retentions))
			got = tcoReplaceStrings(t, got, tcoInverse(retentions))
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

// tcoArchiveRetentionPlaceholders stand for archive retentions of the tenant in fidelity/cases.json.
// The recorder sends the ids of the first two retentions of the tenant in their place and writes the
// placeholders back into the responses, so the files hold no tenant ids. The fake knows them.
var tcoArchiveRetentionPlaceholders = []string{"<archiveRetentionA>", "<archiveRetentionB>"}

// tcoTenantArchiveRetentions maps each placeholder to the id of an archive retention of the tenant.
func tcoTenantArchiveRetentions(t *testing.T, base, apiKey string) map[string]string {
	t.Helper()
	status, got := amSend(t, strings.TrimSuffix(base, tcoBasePath)+"/dataengine/retention-tags/v1", apiKey, http.MethodGet, nil)
	if status != http.StatusOK {
		t.Fatalf("cannot list the archive retentions of the tenant (%d): %s", status, compact(got))
	}
	m, _ := got.(map[string]any)
	list := tcoList(m["retentions"])
	if len(list) < len(tcoArchiveRetentionPlaceholders) {
		t.Fatalf("the tenant has %d archive retentions, the cases need %d", len(list), len(tcoArchiveRetentionPlaceholders))
	}
	out := map[string]string{}
	for i, placeholder := range tcoArchiveRetentionPlaceholders {
		id, _ := list[i]["id"].(string)
		out[placeholder] = id
	}
	return out
}

// tcoReplaceStrings returns v with each key of replace, as a whole JSON string or inside one,
// replaced by its value.
func tcoReplaceStrings(t *testing.T, v any, replace map[string]string) any {
	t.Helper()
	if v == nil || len(replace) == 0 {
		return v
	}
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for from, to := range replace {
		from, to = strings.Trim(compact(from), `"`), strings.Trim(compact(to), `"`)
		text = strings.ReplaceAll(text, from, to)
	}
	var out any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func tcoInverse(m map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range m {
		out[v] = k
	}
	return out
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
		// A policy-level priority means the backend added the targets; the API refuses both. A RUM
		// policy has no targets, and the API refuses them.
		if p, _ := policy["priority"].(string); (p != "" && p != "PRIORITY_TYPE_UNSPECIFIED") || policy["priorityOverride"] != nil || source == "SOURCE_TYPE_RUM" {
			delete(generic, "targets")
		}
		entry := map[string]any{"policy": generic}
		rules := map[string]string{"SOURCE_TYPE_LOGS": "logRules", "SOURCE_TYPE_SPANS": "spanRules", "SOURCE_TYPE_RUM": "rumRules"}[source]
		entry[rules] = cmp.Or[any](policy[rules], map[string]any{})
		policies = append(policies, entry)
	}
	return map[string]any{"policies": policies}
}

// The recorder restores a RUM policy with its RUM rules and without the targets that a Get adds.
func TestTCORestoreBodyRUM(t *testing.T) {
	got := map[string]any{"policies": []any{map[string]any{
		"name": "rum", "priority": "PRIORITY_TYPE_MEDIUM", "enabled": true,
		"rumRules": map[string]any{"severities": []any{"SEVERITY_ERROR"}},
		"targets":  []any{},
	}}}
	body := tcoRestoreBody(t, got, "SOURCE_TYPE_RUM")
	entry := body["policies"].([]any)[0].(map[string]any)
	policy := entry["policy"].(map[string]any)
	if _, ok := entry["rumRules"]; !ok || entry["logRules"] != nil || entry["spanRules"] != nil {
		t.Fatalf("entry = %v, want rumRules only", entry)
	}
	if _, ok := policy["targets"]; ok || policy["disabled"] != false {
		t.Fatalf("policy = %v, want no targets and disabled false", policy)
	}
}

// tcoMask replaces what differs in every run and what belongs to the tenant: policy ids, also inside
// error messages, the company id, and the times.
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
	case string:
		// An error message can name a policy by its id ("DPXL compile failed for policy a, id:...").
		return uuidPattern.ReplaceAllString(x, "<id>")
	}
	return v
}
