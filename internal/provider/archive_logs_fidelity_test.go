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

// alReplayVars are the token values that the offline test sends to the fake. The fake stores
// alServerRegion when a request has no region, and returns this external id.
func alReplayVars() map[string]string {
	return map[string]string{
		"BUCKET":      "fidelity-bucket",
		"REGION":      alServerRegion,
		"EXTERNAL_ID": "external-id-of-the-company",
	}
}

// newRecordingTenantFake returns the fake with the target that the recording tenant had: one with
// enableTags set, which the API keeps on every Set.
func newRecordingTenantFake() *alFake {
	return &alFake{target: map[string]any{
		"s3":         map[string]any{"bucket": "tenant-bucket", "region": alServerRegion},
		"isActive":   false,
		"enableTags": true,
	}}
}

// TestArchiveLogsFakeMatchesRecordedBackend checks the fake archive target backend against
// fidelity/recorded.json. Record that file against a tenant (TestRecordArchiveLogsFidelity),
// then keep the fake in line with those answers. This test skips when recorded.json does not
// exist.
func TestArchiveLogsFakeMatchesRecordedBackend(t *testing.T) {
	cases, recorded := loadFidelityAt(t, alGoldenDir)
	fake := newRecordingTenantFake()
	vars := alReplayVars()
	for ci, c := range cases.Cases {
		t.Run(c.Name, func(t *testing.T) {
			rc := recorded.Cases[ci]
			if rc.Name != c.Name || len(rc.Steps) != len(c.Steps) {
				t.Fatalf("recorded.json does not match cases.json for %q: record again", c.Name)
			}
			for si, s := range c.Steps {
				label := fmt.Sprintf("step %d: %s %s", si+1, s.Method, compact(s.Body))
				status, got := runArchiveLogsFidelityStep(t, fake, s, vars)
				compareFidelityStep(t, label, status, got, rc.Steps[si].Status, rc.Steps[si].Response)
			}
		})
	}
}

func runArchiveLogsFidelityStep(t *testing.T, fake *alFake, s fidelityStep, vars map[string]string) (int, any) {
	t.Helper()
	body := s.Body
	if body != nil {
		body = substJSON(t, body, vars)
	}
	fake.mu.Lock()
	status, resp := fake.handle(s.Method, alPath+substString(s.Path, vars), body)
	fake.mu.Unlock()
	return status, unsubstJSON(t, roundTrip(t, resp), vars)
}

// TestRecordArchiveLogsFidelity sends fidelity/cases.json to a real tenant and writes
// fidelity/recorded.json. Set RECORD_ARCHIVE_LOGS_FIDELITY=1 with CORALOGIX_API_KEY and
// CORALOGIX_ENV (or CX_REGION).
//
// The archive target is one per company, so recording replaces the target of the tenant. Every
// Set of the cases is inactive, so the API does not write to the made-up bucket. Before the cases,
// the recorder reads the target and logs it. Cleanup sets it back. When the company had no S3
// target, the API cannot remove one, and the company keeps an inactive target with the made-up
// bucket. Use a test tenant.
func TestRecordArchiveLogsFidelity(t *testing.T) {
	if os.Getenv("RECORD_ARCHIVE_LOGS_FIDELITY") != "1" {
		t.Skip("set RECORD_ARCHIVE_LOGS_FIDELITY=1 with CORALOGIX_API_KEY and CORALOGIX_ENV to record against a tenant; it replaces the archive target of the tenant and restores it")
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
	url := strings.TrimRight(base, "/") + alPath

	rawCases, err := os.ReadFile(filepath.Join(alGoldenDir, "fidelity", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases fidelityCases
	if err := json.Unmarshal(rawCases, &cases); err != nil {
		t.Fatal(err)
	}

	status, original := alSend(t, url, apiKey, http.MethodGet, nil)
	if status != http.StatusOK {
		t.Fatalf("reading the archive target returned %d: %s", status, compact(original))
	}
	t.Logf("archive target before recording: %s", compact(original))
	t.Cleanup(func() { restoreArchiveTarget(t, url, apiKey, original) })

	var suffix [4]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	vars := map[string]string{"BUCKET": "cx-tf-fidelity-" + hex.EncodeToString(suffix[:])}

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
			status, got := alSend(t, url+substString(s.Path, vars), apiKey, s.Method, body)
			if status == http.StatusForbidden || status == http.StatusUnauthorized {
				t.Fatalf("the API returned %d. Recording needs a management API key with logs data-setup permissions.", status)
			}
			learnArchiveVars(got, vars)
			rec.Steps = append(rec.Steps, struct {
				Status   int `json:"status"`
				Response any `json:"response"`
			}{Status: status, Response: unsubstJSON(t, got, vars)})
		}
		out.Cases = append(out.Cases, rec)
	}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(alGoldenDir, "fidelity", "recorded.json"), append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

// learnArchiveVars sets REGION and EXTERNAL_ID from the first answer that has an S3 target, so the
// recording holds tokens instead of the values of the tenant.
func learnArchiveVars(resp any, vars map[string]string) {
	target, _ := resp.(map[string]any)["target"].(map[string]any)
	s3, _ := target["s3"].(map[string]any)
	if s3 == nil {
		return
	}
	if r, _ := s3["region"].(string); r != "" && vars["REGION"] == "" {
		vars["REGION"] = r
	}
	if id, _ := s3["externalId"].(string); id != "" && vars["EXTERNAL_ID"] == "" {
		vars["EXTERNAL_ID"] = id
	}
}

// restoreArchiveTarget sets the S3 target that the company had before recording. When the restore
// fails, or the company had none, it says what the target is now, so that a person can fix it.
func restoreArchiveTarget(t *testing.T, url, apiKey string, original any) {
	target, _ := original.(map[string]any)["target"].(map[string]any)
	s3, _ := target["s3"].(map[string]any)
	if s3 == nil {
		t.Logf("the company had no S3 archive target. The API cannot remove one, so it keeps an inactive target with the made-up bucket.")
		return
	}
	spec, _ := target["archiveSpec"].(map[string]any)
	active, _ := spec["isActive"].(bool)
	restore := map[string]any{"bucket": s3["bucket"]}
	for _, key := range []string{"region", "roleArn"} {
		if v, ok := s3[key]; ok {
			restore[key] = v
		}
	}
	status, resp := alSend(t, url, apiKey, http.MethodPost, map[string]any{"isActive": active, "s3": restore})
	if status != http.StatusOK {
		t.Errorf("RESTORE FAILED (%d): set the archive target back by hand to isActive=%t, s3=%s. API answer: %s", status, active, compact(restore), compact(resp))
		return
	}
	t.Logf("archive target restored: isActive=%t, s3=%s", active, compact(restore))
}

func alSend(t *testing.T, url, apiKey, method string, body any) (int, any) {
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
			parsed = map[string]any{"raw": string(raw)}
		}
	}
	return resp.StatusCode, parsed
}
