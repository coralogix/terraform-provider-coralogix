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

const (
	arGoldenDir = "testdata/archive_retentions_baseline"
	arPath      = "/dataengine/retention-tags/v1"
)

// arReplayVars are the token values that the offline test sends to the fake.
func arReplayVars() map[string]string {
	vars := map[string]string{}
	for i, id := range arTagIDs {
		vars[fmt.Sprintf("ID%d", i+1)] = id
		if i > 0 {
			vars[fmt.Sprintf("NAME%d", i+1)] = arDefaultNames[i]
		}
	}
	return vars
}

// TestArchiveRetentionsFakeMatchesRecordedBackend checks the fake retention tags backend against
// fidelity/recorded.json. Record that file against a tenant (TestRecordArchiveRetentionsFidelity),
// then keep the fake in line with those answers.
func TestArchiveRetentionsFakeMatchesRecordedBackend(t *testing.T) {
	cases, recorded := loadFidelityAt(t, arGoldenDir)
	vars := arReplayVars()
	fake := &arFake{}
	fake.reset()
	for ci, c := range cases.Cases {
		t.Run(c.Name, func(t *testing.T) {
			rc := recorded.Cases[ci]
			if rc.Name != c.Name || len(rc.Steps) != len(c.Steps) {
				t.Fatalf("recorded.json does not match cases.json for %q: record again", c.Name)
			}
			for si, s := range c.Steps {
				label := fmt.Sprintf("step %d: %s %s %s", si+1, s.Method, s.Path, compact(s.Body))
				body := s.Body
				if body != nil {
					body = substJSON(t, body, vars)
				}
				fake.mu.Lock()
				status, resp := fake.handle(s.Method, arPath+s.Path, body)
				fake.mu.Unlock()
				got := unsubstJSON(t, roundTrip(t, resp), vars)
				compareFidelityStep(t, label, status, got, rc.Steps[si].Status, rc.Steps[si].Response)
			}
		})
	}
}

// TestRecordArchiveRetentionsFidelity sends fidelity/cases.json to a real tenant and writes
// fidelity/recorded.json. Set RECORD_ARCHIVE_RETENTIONS_FIDELITY=1 with CORALOGIX_API_KEY and
// CORALOGIX_ENV (or CX_REGION).
//
// The API has no delete, and every write replaces tag names, so the cases rename tags of the
// tenant for a moment. Each case restores the starting names, and cleanup restores them again.
// Tag ids never change.
func TestRecordArchiveRetentionsFidelity(t *testing.T) {
	if os.Getenv("RECORD_ARCHIVE_RETENTIONS_FIDELITY") != "1" {
		t.Skip("set RECORD_ARCHIVE_RETENTIONS_FIDELITY=1 with CORALOGIX_API_KEY and CORALOGIX_ENV to record; it renames retention tags of the tenant for a moment and restores them")
	}
	url, apiKey := arRecordingTarget(t)

	rawCases, err := os.ReadFile(filepath.Join(arGoldenDir, "fidelity", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases fidelityCases
	if err := json.Unmarshal(rawCases, &cases); err != nil {
		t.Fatal(err)
	}

	status, start := amSend(t, url, apiKey, http.MethodGet, nil)
	if status != http.StatusOK {
		t.Fatalf("Get returned %d: %s", status, compact(start))
	}
	tags := arTags(start)
	if len(tags) != 4 || tags[0]["name"] != "Default" {
		t.Fatalf("want 4 retention tags with Default first, got %s", compact(start))
	}
	vars := map[string]string{}
	restore := []any{}
	for i, tag := range tags {
		id, _ := tag["id"].(string)
		name, _ := tag["name"].(string)
		if strings.HasPrefix(name, "tf_probe") || strings.HasPrefix(name, "aaaa") {
			t.Fatalf("tag %d is named %q, left by an earlier probe. Rename it by hand first.", i+1, name)
		}
		vars[fmt.Sprintf("ID%d", i+1)] = id
		if i > 0 {
			vars[fmt.Sprintf("NAME%d", i+1)] = name
		}
		restore = append(restore, map[string]any{"id": id, "name": name})
	}
	t.Logf("starting tags: %s", compact(start))
	t.Cleanup(func() {
		status, resp := amSend(t, url, apiKey, http.MethodPost, map[string]any{"retentionUpdateElements": restore})
		_, now := amSend(t, url, apiKey, http.MethodGet, nil)
		if status != http.StatusOK || !arSameNames(tags, arTags(now)) {
			t.Errorf("RESTORE FAILED (%d: %s). Rename the tags by hand to: %s. Now: %s", status, compact(resp), compact(restore), compact(now))
			return
		}
		t.Log("the retention tags have their starting names")
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
		var last any
		for _, s := range c.Steps {
			var body any
			if s.Body != nil {
				body = substJSON(t, s.Body, vars)
			}
			status, got := amSend(t, url+s.Path, apiKey, s.Method, body)
			if status == http.StatusUnauthorized {
				t.Fatalf("the API returned %d. Recording needs a management API key with retention permissions.", status)
			}
			if s.Method == http.MethodGet {
				last = got
			}
			rec.Steps = append(rec.Steps, struct {
				Status   int `json:"status"`
				Response any `json:"response"`
			}{Status: status, Response: unsubstJSON(t, got, vars)})
		}
		if !arSameNames(tags, arTags(last)) {
			t.Fatalf("case %s left the tags as %s; stopping", c.Name, compact(last))
		}
		out.Cases = append(out.Cases, rec)
	}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(arGoldenDir, "fidelity", "recorded.json"), append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

// arRecordingTarget returns the URL of the retention tags API and the API key.
func arRecordingTarget(t *testing.T) (url, apiKey string) {
	t.Helper()
	apiKey = cmp.Or(os.Getenv("CORALOGIX_API_KEY"), os.Getenv("CX_API_KEY"))
	region := cmp.Or(os.Getenv("CORALOGIX_ENV"), os.Getenv("CX_REGION"))
	if apiKey == "" || region == "" {
		t.Fatal("CORALOGIX_API_KEY and CORALOGIX_ENV (or CX_REGION) are required to record")
	}
	base, ok := cxsdkOpenapi.URLFromRegion(strings.ToLower(region))
	if !ok {
		t.Fatalf("unknown region %q", region)
	}
	return strings.TrimRight(base, "/") + arPath, apiKey
}

// arTags returns the tags of a Get answer.
func arTags(resp any) []map[string]any {
	m, _ := resp.(map[string]any)
	items, _ := m["retentions"].([]any)
	var tags []map[string]any
	for _, item := range items {
		if tag, ok := item.(map[string]any); ok {
			tags = append(tags, tag)
		}
	}
	return tags
}

// arSameNames reports whether two tag lists have the same ids and names in the same order.
func arSameNames(want, got []map[string]any) bool {
	if len(want) != len(got) {
		return false
	}
	for i := range want {
		if want[i]["id"] != got[i]["id"] || want[i]["name"] != got[i]["name"] {
			return false
		}
	}
	return true
}
