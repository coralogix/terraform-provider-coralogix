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
	"sort"
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
			values := make([]any, 0, len(x))
			for _, item := range x {
				values = append(values, item)
			}
			// Backend map keys are generated IDs and don't define Terraform set order.
			sort.Slice(values, func(i, j int) bool {
				left, _ := json.Marshal(values[i])
				right, _ := json.Marshal(values[j])
				return bytes.Compare(left, right) < 0
			})
			for i, item := range values {
				out[fmt.Sprintf("<entry-%d>", i+1)] = normalizeIpAccessValue(item, "entry")
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

func ipAccessFidelityVars(run string) map[string]string {
	return map[string]string{"NAME": run}
}

// TestRecordIpAccessFidelity writes real API answers for cases.json. The IP access API is a
// singleton: recording creates and deletes team IP access entries. Run only against a disposable
// team with no existing IP access entries and customer-support access disabled. The two Create
// ranges split IPv4 without overlap. The final enabled safety probe uses 0.0.0.0/0, so it permits
// every IPv4 client. Before Delete and cleanup, the recorder verifies ownership and refuses any
// enabled rule other than that universal probe.
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

func ensureIpAccessFidelityEntriesOwned(t *testing.T, base, apiKey, run string, ownedEntryIDs map[string]struct{}) {
	t.Helper()
	status, body := sendIpAccessFidelityRequest(t, base, apiKey, fidelityStep{Method: http.MethodGet})
	if status != http.StatusOK {
		t.Fatalf("pre-delete Get answered %d; refusing to delete IP access entries", status)
	}
	settings := asObject(asObject(body)["settings"])
	access, _ := settings["enableCoralogixCustomerSupportAccess"].(string)
	entries := asObject(settings["ipAccess"])
	if access != "CORALOGIX_CUSTOMER_SUPPORT_ACCESS_DISABLED" {
		t.Fatalf("pre-delete found changed support access or unexpected entry count; refusing to delete IP access entries")
	}
	for id, raw := range entries {
		entry := asObject(raw)
		name, _ := entry["name"].(string)
		_, knownID := ownedEntryIDs[id]
		knownName := strings.HasPrefix(name, run+"-")
		if !knownID && !knownName {
			t.Fatalf("pre-delete found an entry ID not returned by this run; refusing to delete IP access entries")
		}
		enabled, _ := entry["enabled"].(bool)
		ipRange, _ := entry["ipRange"].(string)
		if enabled && (!knownName || ipRange != "0.0.0.0/0") {
			t.Fatalf("pre-delete found an enabled rule other than this run's universal IPv4 probe; refusing to delete IP access entries")
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
	ownedEntryIDs := map[string]struct{}{}
	t.Cleanup(func() {
		if vars["MUTATION_ATTEMPTED"] == "" {
			return
		}
		if err := cleanupIpAccessFidelity(base, apiKey, vars["NAME"], ownedEntryIDs); err != nil {
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
			if step.Method == http.MethodPost || step.Method == http.MethodPut {
				// Arm cleanup before sending a write: the server may commit it even
				// if the client fails while receiving or reading the response.
				vars["MUTATION_ATTEMPTED"] = "true"
				ensureIpAccessFidelityEntriesOwned(t, base, apiKey, vars["NAME"], ownedEntryIDs)
			} else if step.Method == http.MethodDelete {
				vars["MUTATION_ATTEMPTED"] = "true"
				ensureIpAccessFidelityEntriesOwned(t, base, apiKey, vars["NAME"], ownedEntryIDs)
			}
			status, body := sendIpAccessFidelityRequest(t, base, apiKey, fidelityStep{
				Method: step.Method,
				Path:   substString(step.Path, vars),
				Body:   substJSON(t, step.Body, vars),
			})
			if status == http.StatusUnauthorized || status == http.StatusForbidden {
				t.Fatalf("the API answered %d; recording needs a management key with IP access permissions", status)
			}
			if (step.Method == http.MethodPost || step.Method == http.MethodPut) && status == http.StatusOK {
				captureIpAccessFidelityEntryIDs(ownedEntryIDs, body)
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

func captureIpAccessFidelityEntryIDs(ownedEntryIDs map[string]struct{}, body any) {
	settings := asObject(asObject(body)["settings"])
	entries, ok := settings["ipAccess"].(map[string]any)
	if !ok {
		return
	}
	for id := range ownedEntryIDs {
		delete(ownedEntryIDs, id)
	}
	for id := range entries {
		ownedEntryIDs[id] = struct{}{}
	}
}

func knownUnnamedDisabledRule(entry map[string]any) bool {
	name, _ := entry["name"].(string)
	enabled, _ := entry["enabled"].(bool)
	ipRange, _ := entry["ipRange"].(string)
	return name == "" && !enabled && (ipRange == "128.0.0.0/1" || ipRange == "0.0.0.0/0")
}

func cleanupIpAccessFidelity(base, apiKey, run string, ownedEntryIDs map[string]struct{}) error {
	if ownedEntryIDs == nil {
		return nil
	}
	status, body := sendIpAccessFidelityRequestNoTest(base, apiKey, http.MethodGet, "", nil)
	if status != http.StatusOK {
		return fmt.Errorf("cleanup preflight Get answered %d: %s", status, compact(body))
	}
	settings := asObject(asObject(body)["settings"])
	access, _ := settings["enableCoralogixCustomerSupportAccess"].(string)
	if access != "CORALOGIX_CUSTOMER_SUPPORT_ACCESS_DISABLED" {
		return fmt.Errorf("cleanup found customer-support access changed; left settings untouched")
	}
	entries := asObject(settings["ipAccess"])
	for id, raw := range entries {
		entry := asObject(raw)
		name, _ := entry["name"].(string)
		_, knownID := ownedEntryIDs[id]
		knownName := strings.HasPrefix(name, run+"-")
		if !knownID && !knownName && !knownUnnamedDisabledRule(entry) {
			return fmt.Errorf("found an IP access entry ID not returned by this recorder; left settings untouched")
		}
		enabled, _ := entry["enabled"].(bool)
		ipRange, _ := entry["ipRange"].(string)
		if enabled && (!knownName || ipRange != "0.0.0.0/0") {
			return fmt.Errorf("found an enabled IP access entry other than this recorder's universal IPv4 rule; left settings untouched")
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

func TestIpAccessRecorderCleanupOwnsUnnamedRules(t *testing.T) {
	fake := newIpAccessFidelityFake()
	srv := httptest.NewServer(fake)
	defer srv.Close()

	status, response := sendIpAccessFidelityRequest(t, srv.URL, "dry-run-key", fidelityStep{
		Method: http.MethodPost,
		Body: map[string]any{
			"enableCoralogixCustomerSupportAccess": "CORALOGIX_CUSTOMER_SUPPORT_ACCESS_DISABLED",
			"ipAccess":                             []any{map[string]any{"enabled": false, "ipRange": "0.0.0.0/0"}},
		},
	})
	if status != http.StatusOK {
		t.Fatalf("Create answered %d: %s", status, compact(response))
	}
	ownedEntryIDs := map[string]struct{}{}
	captureIpAccessFidelityEntryIDs(ownedEntryIDs, response)
	if len(ownedEntryIDs) != 1 {
		t.Fatalf("captured %d owned entry IDs, want 1", len(ownedEntryIDs))
	}
	if err := cleanupIpAccessFidelity(srv.URL, "dry-run-key", "dry-run", ownedEntryIDs); err != nil {
		t.Fatalf("cleanup rejected its unnamed rule: %v", err)
	}
	status, response = sendIpAccessFidelityRequest(t, srv.URL, "dry-run-key", fidelityStep{Method: http.MethodGet})
	settings := asObject(asObject(response)["settings"])
	if status != http.StatusOK || len(asObject(settings["ipAccess"])) != 0 || settings["enableCoralogixCustomerSupportAccess"] != "CORALOGIX_CUSTOMER_SUPPORT_ACCESS_DISABLED" {
		t.Fatalf("cleanup left unexpected settings: status=%d response=%s", status, compact(response))
	}
}

func TestIpAccessRecorderCleanupAllowsNamedUniversalEnabledRule(t *testing.T) {
	fake := newIpAccessFidelityFake()
	fake.seed(map[string]any{
		"enableCoralogixCustomerSupportAccess": "CORALOGIX_CUSTOMER_SUPPORT_ACCESS_DISABLED",
		"ipAccess": map[string]any{
			"entry-1": map[string]any{
				"enabled": true,
				"ipRange": "0.0.0.0/0",
				"name":    "cleanup-test-enabled-broad",
			},
		},
	})
	srv := httptest.NewServer(fake)
	defer srv.Close()

	ownedEntryIDs := map[string]struct{}{}
	ensureIpAccessFidelityEntriesOwned(t, srv.URL, "dry-run-key", "cleanup-test", ownedEntryIDs)
	if err := cleanupIpAccessFidelity(srv.URL, "dry-run-key", "cleanup-test", ownedEntryIDs); err != nil {
		t.Fatalf("cleanup rejected its named universal enabled rule: %v", err)
	}
	status, response := sendIpAccessFidelityRequest(t, srv.URL, "dry-run-key", fidelityStep{Method: http.MethodGet})
	settings := asObject(asObject(response)["settings"])
	if status != http.StatusOK || len(asObject(settings["ipAccess"])) != 0 || settings["enableCoralogixCustomerSupportAccess"] != "CORALOGIX_CUSTOMER_SUPPORT_ACCESS_DISABLED" {
		t.Fatalf("cleanup left unexpected settings: status=%d response=%s", status, compact(response))
	}
}

func TestIpAccessRecorderCleanupRecognizesUnnamedRuleAfterLostResponse(t *testing.T) {
	fake := newIpAccessFidelityFake()
	srv := httptest.NewServer(fake)
	defer srv.Close()
	status, response := sendIpAccessFidelityRequest(t, srv.URL, "dry-run-key", fidelityStep{
		Method: http.MethodPost,
		Body: map[string]any{
			"enableCoralogixCustomerSupportAccess": "CORALOGIX_CUSTOMER_SUPPORT_ACCESS_DISABLED",
			"ipAccess":                             []any{map[string]any{"enabled": false, "ipRange": "0.0.0.0/0"}},
		},
	})
	if status != http.StatusOK {
		t.Fatalf("Create answered %d: %s", status, compact(response))
	}
	// Ignore the response IDs to simulate a request that committed before its response was lost.
	if err := cleanupIpAccessFidelity(srv.URL, "dry-run-key", "cleanup-test", map[string]struct{}{}); err != nil {
		t.Fatalf("cleanup did not recognize the known unnamed rule: %v", err)
	}
	status, response = sendIpAccessFidelityRequest(t, srv.URL, "dry-run-key", fidelityStep{Method: http.MethodGet})
	settings := asObject(asObject(response)["settings"])
	if status != http.StatusOK || len(asObject(settings["ipAccess"])) != 0 || settings["enableCoralogixCustomerSupportAccess"] != "CORALOGIX_CUSTOMER_SUPPORT_ACCESS_DISABLED" {
		t.Fatalf("cleanup left unexpected settings: status=%d response=%s", status, compact(response))
	}
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
