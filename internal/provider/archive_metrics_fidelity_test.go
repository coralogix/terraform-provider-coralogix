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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
)

// amReplayVars are the token values that the offline test sends to the fake.
func amReplayVars() map[string]string {
	return map[string]string{
		"BUCKET":         "fidelity-bucket",
		"MISSING_BUCKET": "fidelity-missing-bucket",
	}
}

// TestArchiveMetricsFakeMatchesRecordedBackend checks the fake metrics archive backend against
// fidelity/recorded.json. Record that file against a tenant (TestRecordArchiveMetricsFidelity),
// then keep the fake in line with those answers. This test skips when recorded.json does not
// exist.
func TestArchiveMetricsFakeMatchesRecordedBackend(t *testing.T) {
	cases, recorded := loadFidelityAt(t, amGoldenDir)
	vars := amReplayVars()
	fake := &amFake{unwritable: map[string]bool{vars["MISSING_BUCKET"]: true}}
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
				status, resp := fake.handle(s.Method, amPath+s.Path, body)
				fake.mu.Unlock()
				got := unsubstJSON(t, amMaskTenantID(roundTrip(t, resp)), vars)
				compareFidelityStep(t, label, status, got, rc.Steps[si].Status, rc.Steps[si].Response)
			}
		})
	}
}

// TestRecordArchiveMetricsFidelity sends fidelity/cases.json to a real tenant and writes
// fidelity/recorded.json. Set RECORD_ARCHIVE_METRICS_FIDELITY=1 with CORALOGIX_API_KEY,
// CORALOGIX_ENV (or CX_REGION), and ARCHIVE_METRICS_BUCKET.
//
// The tenant must have no metrics archive: the first cases record the answers without one. The
// bucket must let the Coralogix archive role of the environment write, read, and delete objects;
// the API writes and deletes metrics/testfile in it on every successful set. The API cannot
// remove a metrics archive, so cleanup disables it, and the tenant keeps a disabled archive that
// points to the bucket.
func TestRecordArchiveMetricsFidelity(t *testing.T) {
	if os.Getenv("RECORD_ARCHIVE_METRICS_FIDELITY") != "1" {
		t.Skip("set RECORD_ARCHIVE_METRICS_FIDELITY=1 with CORALOGIX_API_KEY, CORALOGIX_ENV, and ARCHIVE_METRICS_BUCKET to record against a tenant without a metrics archive; it leaves a disabled archive")
	}
	url, apiKey, bucket := amRecordingTarget(t)

	rawCases, err := os.ReadFile(filepath.Join(amGoldenDir, "fidelity", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases fidelityCases
	if err := json.Unmarshal(rawCases, &cases); err != nil {
		t.Fatal(err)
	}

	status, original := amSend(t, url, apiKey, http.MethodGet, nil)
	if status != http.StatusNotFound {
		t.Fatalf("the tenant already has a metrics archive (%d: %s). Record against a tenant without one.", status, compact(original))
	}
	t.Cleanup(func() {
		status, resp := amSend(t, url+"/disable", apiKey, http.MethodPost, nil)
		if status != http.StatusOK {
			t.Errorf("DISABLE FAILED (%d): disable the metrics archive of the tenant by hand. API answer: %s", status, compact(resp))
			return
		}
		t.Logf("the metrics archive of the tenant is disabled; it still points to %s", bucket)
	})

	var suffix [4]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	vars := map[string]string{
		"BUCKET":         bucket,
		"MISSING_BUCKET": "cx-tf-fidelity-missing-" + hex.EncodeToString(suffix[:]),
	}

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
			var body any
			if s.Body != nil {
				body = substJSON(t, s.Body, vars)
			}
			status, got := amSend(t, url+s.Path, apiKey, s.Method, body)
			if status == http.StatusForbidden || status == http.StatusUnauthorized {
				t.Fatalf("the API returned %d. Recording needs a management API key with metrics data-setup permissions.", status)
			}
			rec.Steps = append(rec.Steps, struct {
				Status   int `json:"status"`
				Response any `json:"response"`
			}{Status: status, Response: unsubstJSON(t, amMaskTenantID(got), vars)})
		}
		out.Cases = append(out.Cases, rec)
	}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(amGoldenDir, "fidelity", "recorded.json"), append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

// amRecordingTarget returns the URL of the metrics archive API, the API key, and the bucket.
func amRecordingTarget(t *testing.T) (url, apiKey, bucket string) {
	t.Helper()
	apiKey = cmp.Or(os.Getenv("CORALOGIX_API_KEY"), os.Getenv("CX_API_KEY"))
	region := cmp.Or(os.Getenv("CORALOGIX_ENV"), os.Getenv("CX_REGION"))
	bucket = os.Getenv("ARCHIVE_METRICS_BUCKET")
	if apiKey == "" || region == "" || bucket == "" {
		t.Fatal("CORALOGIX_API_KEY, CORALOGIX_ENV (or CX_REGION), and ARCHIVE_METRICS_BUCKET are required to record")
	}
	base, ok := cxsdkOpenapi.URLFromRegion(strings.ToLower(region))
	if !ok {
		t.Fatalf("unknown region %q", region)
	}
	return strings.TrimRight(base, "/") + amPath, apiKey, bucket
}

// amMaskTenantID replaces the company id in a Get answer, so the recording holds no tenant data.
func amMaskTenantID(resp any) any {
	m, _ := resp.(map[string]any)
	config, _ := m["tenantConfig"].(map[string]any)
	if _, ok := config["tenantId"]; ok {
		config["tenantId"] = "<tenantId>"
	}
	return resp
}

// amSend sends one request. A plain-text answer is returned as {"message": text}.
func amSend(t *testing.T, url, apiKey, method string, body any) (int, any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	var parsed any = map[string]any{}
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &parsed); err != nil {
			parsed = map[string]any{"message": strings.TrimSpace(string(raw))}
		}
	}
	return resp.StatusCode, parsed
}
