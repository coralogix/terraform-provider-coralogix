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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
)

// TestIpAccessFakeMatchesRecordedBackend checks the fake against a live API recording.
func TestIpAccessFakeMatchesRecordedBackend(t *testing.T) {
	cases, recorded := loadFidelityAt(t, ipAccessGoldenDir)
	fake := newIpAccessFidelityFake()
	for ci, c := range cases.Cases {
		t.Run(c.Name, func(t *testing.T) {
			rc := recorded.Cases[ci]
			if rc.Name != c.Name || len(rc.Steps) != len(c.Steps) {
				t.Fatalf("recorded.json does not match cases.json for %q: record again", c.Name)
			}
			vars := ipAccessFidelityVars("fidelity")
			for si, step := range c.Steps {
				label := fmt.Sprintf("step %d: %s %s", si+1, step.Method, step.Path)
				status, got := runIpAccessFidelityStep(t, fake, step, vars)
				compareFidelityStep(t, label, status, normalizeIpAccessFidelity(got), rc.Steps[si].Status, normalizeIpAccessFidelity(rc.Steps[si].Response))
			}
		})
	}
}

func newIpAccessFidelityFake() *ipAccessFake {
	return &ipAccessFake{settings: map[string]any{
		"id":                                   ipAccessSettingsID,
		"ipAccess":                             map[string]any{},
		"enableCoralogixCustomerSupportAccess": "CORALOGIX_CUSTOMER_SUPPORT_ACCESS_DISABLED",
	}}
}

func runIpAccessFidelityStep(t *testing.T, fake *ipAccessFake, step fidelityStep, vars map[string]string) (int, any) {
	t.Helper()
	body := step.Body
	if body != nil {
		body = substJSON(t, body, vars)
	}
	fake.mu.Lock()
	status, response := fake.handle(step.Method, ipAccessBasePath+substString(step.Path, vars), body)
	fake.mu.Unlock()
	return status, unsubstJSON(t, roundTrip(t, response), vars)
}

func normalizeIpAccessFidelity(value any) any {
	return normalizeIpAccessValue(value, "")
}

func normalizeIpAccessValue(value any, parent string) any {
	switch x := value.(type) {
	case map[string]any:
		out := map[string]any{}
		if parent == "ipAccess" {
			keys := sortedKeysAny(x)
			for i, key := range keys {
				out[fmt.Sprintf("<entry-%d>", i+1)] = normalizeIpAccessValue(x[key], "entry")
			}
			return out
		}
		for key, item := range x {
			switch {
			case key == "id" && parent == "settings":
				out[key] = "<settings-id>"
			default:
				out[key] = normalizeIpAccessValue(item, key)
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = normalizeIpAccessValue(item, parent)
		}
		return out
	default:
		return value
	}
}

func sortedKeysAny(value map[string]any) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func ipAccessFidelityVars(run string) map[string]string {
	return map[string]string{"NAME": run}
}

// TestRecordIpAccessFidelity writes real API answers for cases.json. The IP access API is a
// singleton: recording creates and deletes team IP access entries. Run only against a disposable
// team with no existing IP access entries and customer-support access disabled. The entries used
// here are disabled and use 0.0.0.0/0, so the range is universal even if the backend unexpectedly
// applies it. The recorder refuses to delete unless the only current entry is its disabled rule.
func TestRecordIpAccessFidelity(t *testing.T) {
	if os.Getenv("RECORD_IP_ACCESS_FIDELITY") != "1" {
		t.Skip("set RECORD_IP_ACCESS_FIDELITY=1 to record against a disposable tenant")
	}
	if os.Getenv("IP_ACCESS_FIDELITY_ALLOW_MUTATION") != "I_UNDERSTAND_THIS_USES_A_DISPOSABLE_TEAM" {
		t.Fatal("recording mutates singleton IP access settings; set IP_ACCESS_FIDELITY_ALLOW_MUTATION=I_UNDERSTAND_THIS_USES_A_DISPOSABLE_TEAM only for a disposable team")
	}
	apiKey := os.Getenv("CORALOGIX_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("CX_API_KEY")
	}
	if apiKey == "" {
		t.Fatal("CORALOGIX_API_KEY is required to record")
	}
	base := os.Getenv("IP_ACCESS_FIDELITY_BASE_URL")
	if base == "" {
		region := os.Getenv("CORALOGIX_ENV")
		if region == "" {
			region = os.Getenv("CX_REGION")
		}
		if region == "" {
			t.Fatal("CORALOGIX_ENV or CX_REGION is required to record")
		}
		var ok bool
		if base, ok = cxsdkOpenapi.URLFromRegion(strings.ToLower(region)); !ok {
			t.Fatalf("unknown region %q", region)
		}
	}
	ensureIpAccessFidelityTeamSafe(t, base, apiKey)
	recordIpAccessFidelity(t, base, apiKey, filepath.Join(ipAccessGoldenDir, "fidelity", "recorded.json"))
}

func ensureIpAccessFidelityTeamSafe(t *testing.T, base, apiKey string) {
	t.Helper()
	status, body := sendIpAccessFidelityRequest(t, base, apiKey, fidelityStep{Method: http.MethodGet})
	if status != http.StatusOK {
		t.Fatalf("preflight Get answered %d; refusing to record against this team: %s", status, compact(body))
	}
	root := asObject(body)
	settings := asObject(root["settings"])
	entries := asObject(settings["ipAccess"])
	access, _ := settings["enableCoralogixCustomerSupportAccess"].(string)
	if len(entries) != 0 || access != "CORALOGIX_CUSTOMER_SUPPORT_ACCESS_DISABLED" {
		t.Fatalf("preflight found IP access entries or non-disabled support access; refusing to modify settings")
	}
}

func ensureIpAccessFidelityEntriesOwned(t *testing.T, base, apiKey, run string) {
	t.Helper()
	status, body := sendIpAccessFidelityRequest(t, base, apiKey, fidelityStep{Method: http.MethodGet})
	if status != http.StatusOK {
		t.Fatalf("pre-delete Get answered %d; refusing to delete IP access entries", status)
	}
	settings := asObject(asObject(body)["settings"])
	access, _ := settings["enableCoralogixCustomerSupportAccess"].(string)
	entries := asObject(settings["ipAccess"])
	if access != "CORALOGIX_CUSTOMER_SUPPORT_ACCESS_DISABLED" || len(entries) != 1 {
		t.Fatalf("pre-delete found changed support access or unexpected entry count; refusing to delete IP access entries")
	}
	for _, raw := range entries {
		entry := asObject(raw)
		name, _ := entry["name"].(string)
		enabled, _ := entry["enabled"].(bool)
		if !strings.HasPrefix(name, run+"-") || enabled {
			t.Fatalf("pre-delete found an entry not created by this run or an enabled rule; refusing to delete IP access entries")
		}
	}
}

func recordIpAccessFidelity(t *testing.T, base, apiKey, outPath string) {
	t.Helper()
	rawCases, err := os.ReadFile(filepath.Join(ipAccessGoldenDir, "fidelity", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases fidelityCases
	if err := json.Unmarshal(rawCases, &cases); err != nil {
		t.Fatal(err)
	}
	vars := ipAccessFidelityVars(fmt.Sprintf("tf-ip-access-fidelity-%d", time.Now().Unix()))
	t.Cleanup(func() {
		if vars["CREATED"] == "" {
			return
		}
		if err := cleanupIpAccessFidelity(base, apiKey, vars["NAME"]); err != nil {
			t.Errorf("could not safely restore disposable IP access team: %v", err)
		}
	})

	out := fidelityRecorded{}
	for _, c := range cases.Cases {
		recordedCase := struct {
			Name  string `json:"name"`
			Steps []struct {
				Status   int `json:"status"`
				Response any `json:"response"`
			} `json:"steps"`
		}{Name: c.Name}
		for _, step := range c.Steps {
			if step.Method == http.MethodDelete {
				ensureIpAccessFidelityEntriesOwned(t, base, apiKey, vars["NAME"])
			}
			status, body := sendIpAccessFidelityRequest(t, base, apiKey, fidelityStep{
				Method: step.Method,
				Path:   substString(step.Path, vars),
				Body:   substJSON(t, step.Body, vars),
			})
			if status == http.StatusUnauthorized || status == http.StatusForbidden {
				t.Fatalf("the API answered %d; recording needs a management key with IP access permissions", status)
			}
			if step.Method == http.MethodPost && status == http.StatusOK {
				vars["CREATED"] = "true"
			}
			body = unsubstJSON(t, body, map[string]string{"NAME": vars["NAME"]})
			recordedCase.Steps = append(recordedCase.Steps, struct {
				Status   int `json:"status"`
				Response any `json:"response"`
			}{Status: status, Response: normalizeIpAccessFidelity(body)})
		}
		out.Cases = append(out.Cases, recordedCase)
	}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outPath, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sendIpAccessFidelityRequest(t *testing.T, base, apiKey string, step fidelityStep) (int, any) {
	t.Helper()
	url := strings.TrimRight(base, "/") + ipAccessBasePath + step.Path
	var body io.Reader
	if step.Body != nil {
		raw, err := json.Marshal(step.Body)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(step.Method, url, body)
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
	var parsed any
	if len(bytes.TrimSpace(raw)) == 0 {
		parsed = map[string]any{}
	} else if err := json.Unmarshal(raw, &parsed); err != nil {
		parsed = map[string]any{"raw": string(raw)}
	}
	return resp.StatusCode, parsed
}

func cleanupIpAccessFidelity(base, apiKey, run string) error {
	if run == "" {
		return nil
	}
	status, body := sendIpAccessFidelityRequestNoTest(base, apiKey, http.MethodGet, "", nil)
	if status != http.StatusOK {
		return fmt.Errorf("cleanup preflight Get answered %d: %s", status, compact(body))
	}
	settings := asObject(asObject(body)["settings"])
	entries := asObject(settings["ipAccess"])
	for _, raw := range entries {
		entry := asObject(raw)
		name, _ := entry["name"].(string)
		if !strings.HasPrefix(name, run+"-") {
			return fmt.Errorf("found an IP access entry not created by this recorder; left it untouched")
		}
	}
	status, body = sendIpAccessFidelityRequestNoTest(base, apiKey, http.MethodPut, "", map[string]any{
		"enableCoralogixCustomerSupportAccess": "CORALOGIX_CUSTOMER_SUPPORT_ACCESS_DISABLED",
		"ipAccess":                             []any{},
	})
	if status != http.StatusOK {
		return fmt.Errorf("cleanup Replace answered %d: %s", status, compact(body))
	}
	status, body = sendIpAccessFidelityRequestNoTest(base, apiKey, http.MethodDelete, "", nil)
	if status != http.StatusOK {
		return fmt.Errorf("cleanup Delete answered %d: %s", status, compact(body))
	}
	return nil
}

func sendIpAccessFidelityRequestNoTest(base, apiKey, method, path string, body any) (int, any) {
	url := strings.TrimRight(base, "/") + ipAccessBasePath + path
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	req, _ := http.NewRequest(method, url, reader)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var parsed any
	if len(bytes.TrimSpace(raw)) == 0 {
		parsed = map[string]any{}
	} else if json.Unmarshal(raw, &parsed) != nil {
		parsed = map[string]any{"raw": string(raw)}
	}
	return resp.StatusCode, parsed
}

// TestIpAccessRecorderDryRun exercises the live recorder pipeline against the fake without credentials.
func TestIpAccessRecorderDryRun(t *testing.T) {
	fake := newIpAccessFidelityFake()
	outPath := filepath.Join(t.TempDir(), "recorded.json")
	// The local server stands in for the region API URL.
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	recordIpAccessFidelity(t, srv.URL, "dry-run-key", outPath)
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("recorder did not write output: %v", err)
	}
	var recorded fidelityRecorded
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &recorded); err != nil {
		t.Fatal(err)
	}
	rawCases, err := os.ReadFile(filepath.Join(ipAccessGoldenDir, "fidelity", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases fidelityCases
	if err := json.Unmarshal(rawCases, &cases); err != nil {
		t.Fatal(err)
	}
	for ci, c := range cases.Cases {
		fake := newIpAccessFidelityFake()
		vars := ipAccessFidelityVars("dry-run")
		for si, step := range c.Steps {
			status, body := runIpAccessFidelityStep(t, fake, step, vars)
			want := recorded.Cases[ci].Steps[si]
			compareFidelityStep(t, fmt.Sprintf("%s step %d", c.Name, si+1), status, normalizeIpAccessFidelity(body), want.Status, normalizeIpAccessFidelity(want.Response))
		}
	}
}
