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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
)

// TestAPIKeyFakeMatchesRecordedBackend checks the fake against a live API recording.
func TestAPIKeyFakeMatchesRecordedBackend(t *testing.T) {
	cases, recorded := loadFidelityAt(t, apiKeyGoldenDir)
	for ci, c := range cases.Cases {
		t.Run(c.Name, func(t *testing.T) {
			rc := recorded.Cases[ci]
			if rc.Name != c.Name || len(rc.Steps) != len(c.Steps) {
				t.Fatalf("recorded.json does not match cases.json for %q: record again", c.Name)
			}
			fake := newAPIKeyFidelityFake()
			ids := map[string]string{}
			for si, step := range c.Steps {
				label := fmt.Sprintf("step %d: %s %s", si+1, step.Method, step.Path)
				status, got := apiKeyFidelitySend(t, step, "fidelity", strconv.Itoa(apiKeyFakeTeamID), ids, func(method, path string, body any) (int, any) {
					return fake.handle(method, apiKeyBasePath+path, body)
				})
				compareFidelityStep(t, label, status, got, rc.Steps[si].Status, normalizeAPIKeyFidelity(rc.Steps[si].Response, "fidelity"))
			}
		})
	}
}

// apiKeyFidelitySend sends one step with the run name, the team id, and the captured key ids, and
// returns the normalized answer. {{TEAM}} in quotes is the team id as a number.
func apiKeyFidelitySend(t *testing.T, step fidelityStep, run, team string, ids map[string]string, send func(method, path string, body any) (int, any)) (int, any) {
	t.Helper()
	vars := map[string]string{"NAME": run}
	for name, id := range ids {
		vars[name] = id
	}
	var body any
	if step.Body != nil {
		raw, err := json.Marshal(step.Body)
		if err != nil {
			t.Fatal(err)
		}
		body = substJSON(t, json.RawMessage(strings.ReplaceAll(string(raw), `"{{TEAM}}"`, team)), vars)
	}
	status, answer := send(step.Method, substString(step.Path, vars), body)
	answer = roundTrip(t, answer)
	if step.Capture != "" && status == http.StatusOK {
		id, _ := asObject(answer)["keyId"].(string)
		ids[step.Capture] = id
	}
	return status, normalizeAPIKeyFidelity(answer, run)
}

var (
	apiKeyUUIDPattern    = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	apiKeyErrorIDPattern = regexp.MustCompile(`\nError ID: [0-9a-f]+$`)
	apiKeyFlowIDPattern  = regexp.MustCompile(`, flow_id: [0-9a-f-]+\.$`)
	apiKeyPolicyList     = regexp.MustCompile(`(?s)\[\[\n(.*?)\]\]`)
)

// normalizeAPIKeyFidelity masks the key ids, the team id, the secret value, and the run name. It
// shows an access policy as the JSON that it holds, because the API writes its keys in any order,
// and it sorts the permissions that an error message lists, for the same reason.
func normalizeAPIKeyFidelity(value any, run string) any {
	switch x := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, item := range x {
			switch {
			case key == "keyId" || key == "id":
				out[key] = "<key-id>"
			case key == "teamId":
				out[key] = "<team-id>"
			case key == "value":
				out[key] = "<secret>"
			case key == "permissions":
				out[key] = normalizeAPIKeyPermissions(item, run)
			case key == "accessPolicy":
				out[key] = normalizeAPIKeyPolicy(item)
			case key == "message":
				message, _ := item.(string)
				out[key] = normalizeAPIKeyMessage(message, run)
			default:
				out[key] = normalizeAPIKeyFidelity(item, run)
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = normalizeAPIKeyFidelity(item, run)
		}
		return out
	case string:
		return strings.ReplaceAll(x, run, "{{NAME}}")
	default:
		return value
	}
}

// normalizeAPIKeyPermissions sorts a list of permission names: the API returns the permissions of
// a key in no stable order.
func normalizeAPIKeyPermissions(value any, run string) any {
	items, ok := value.([]any)
	if !ok {
		return normalizeAPIKeyFidelity(value, run)
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		name, isString := item.(string)
		if !isString {
			return normalizeAPIKeyFidelity(value, run)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]any, len(names))
	for i, name := range names {
		out[i] = name
	}
	return out
}

func normalizeAPIKeyPolicy(value any) any {
	text, ok := value.(string)
	if !ok {
		return value
	}
	var doc any
	if json.Unmarshal([]byte(text), &doc) != nil {
		return value
	}
	return doc
}

func normalizeAPIKeyMessage(message, run string) string {
	message = apiKeyErrorIDPattern.ReplaceAllString(message, "")
	message = apiKeyFlowIDPattern.ReplaceAllString(message, "")
	message = apiKeyUUIDPattern.ReplaceAllString(message, "<key-id>")
	message = apiKeyPolicyList.ReplaceAllStringFunc(message, func(list string) string {
		lines := strings.Split(strings.TrimSpace(apiKeyPolicyList.FindStringSubmatch(list)[1]), "\n")
		for i := range lines {
			lines[i] = strings.TrimSuffix(strings.TrimSpace(lines[i]), ",")
		}
		sort.Strings(lines)
		return "[[" + strings.Join(lines, ", ") + "]]"
	})
	return strings.ReplaceAll(message, run, "{{NAME}}")
}

// TestRecordAPIKeyFidelity writes real API answers for cases.json. It needs a key of a team: the
// keys that a case creates belong to that team. Each case deletes the keys it creates, and the
// recorder deletes every key that a case left behind.
func TestRecordAPIKeyFidelity(t *testing.T) {
	if os.Getenv("RECORD_API_KEY_FIDELITY") != "1" {
		t.Skip("set RECORD_API_KEY_FIDELITY=1 and a team key to record")
	}
	apiKey := os.Getenv("CORALOGIX_API_KEY")
	if apiKey == "" {
		t.Fatal("CORALOGIX_API_KEY is required to record")
	}
	region := os.Getenv("CORALOGIX_ENV")
	if region == "" {
		t.Fatal("CORALOGIX_ENV is required to record")
	}
	base, ok := cxsdkOpenapi.URLFromRegion(strings.ToLower(region))
	if !ok {
		t.Fatalf("unknown region %q", region)
	}
	recordAPIKeyFidelity(t, base, apiKey, filepath.Join(apiKeyGoldenDir, "fidelity", "recorded.json"))
}

func recordAPIKeyFidelity(t *testing.T, base, apiKey, outPath string) {
	t.Helper()
	rawCases, err := os.ReadFile(filepath.Join(apiKeyGoldenDir, "fidelity", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases fidelityCases
	if err := json.Unmarshal(rawCases, &cases); err != nil {
		t.Fatal(err)
	}
	send := func(method, path string, body any) (int, any) {
		return sendTeamFidelityRequest(base, apiKey, method, apiKeyBasePath+path, body)
	}
	team := apiKeyOwnerTeam(t, send)
	run := newAPIKeyFidelityRunID(t)
	var created []string
	t.Cleanup(func() {
		if err := cleanupAPIKeyFidelity(send, run, created); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	recordingSend := func(method, path string, body any) (int, any) {
		status, answer := send(method, path, body)
		if method == http.MethodPost && path == "" && status == http.StatusOK {
			if id, _ := asObject(answer)["keyId"].(string); id != "" {
				created = append(created, id)
			}
		}
		return status, answer
	}
	out := fidelityRecorded{}
	for _, c := range cases.Cases {
		recordedCase := struct {
			Name  string `json:"name"`
			Steps []struct {
				Status   int `json:"status"`
				Response any `json:"response"`
			} `json:"steps"`
		}{Name: c.Name}
		ids := map[string]string{}
		for _, step := range c.Steps {
			status, answer := apiKeyFidelitySend(t, step, run, team, ids, recordingSend)
			if status == http.StatusUnauthorized {
				t.Fatalf("the API answered %d: the key is not valid for this region", status)
			}
			recordedCase.Steps = append(recordedCase.Steps, struct {
				Status   int `json:"status"`
				Response any `json:"response"`
			}{Status: status, Response: answer})
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

// apiKeyOwnerTeam returns the id of the team that owns the recording key: the owner of a key that
// the list shows. The id never reaches recorded.json.
func apiKeyOwnerTeam(t *testing.T, send func(method, path string, body any) (int, any)) string {
	t.Helper()
	status, answer := send(http.MethodGet, "/list/all", nil)
	if status != http.StatusOK {
		t.Fatalf("list keys answered %d", status)
	}
	for _, item := range asSlice(asObject(answer)["keys"]) {
		if team, ok := asObject(asObject(item)["owner"])["teamId"].(float64); ok {
			return strconv.FormatInt(int64(team), 10)
		}
	}
	t.Fatal("no key of a team to take the team id from")
	return ""
}

func newAPIKeyFidelityRunID(t *testing.T) string {
	t.Helper()
	unique := make([]byte, 6)
	if _, err := rand.Read(unique); err != nil {
		t.Fatal(err)
	}
	return "tf-api-key-fidelity-" + hex.EncodeToString(unique)
}

// cleanupAPIKeyFidelity deletes the keys that the recorder created, and every key whose name
// starts with the run name.
func cleanupAPIKeyFidelity(send func(method, path string, body any) (int, any), run string, created []string) error {
	ids := map[string]bool{}
	for _, id := range created {
		ids[id] = true
	}
	if status, answer := send(http.MethodGet, "/list/all", nil); status == http.StatusOK {
		for _, item := range asSlice(asObject(answer)["keys"]) {
			key := asObject(item)
			if name, _ := key["name"].(string); strings.HasPrefix(name, run+"-") {
				if id, _ := key["id"].(string); id != "" {
					ids[id] = true
				}
			}
		}
	}
	var problems []string
	for id := range ids {
		if status, _ := send(http.MethodDelete, "/"+id, nil); status != http.StatusOK && status != http.StatusNotFound {
			problems = append(problems, fmt.Sprintf("%s: delete answered %d", id, status))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("keys left behind: %s", strings.Join(problems, "; "))
	}
	return nil
}

// TestAPIKeyRecorderDryRun runs the recorder against the fake, without credentials, so the
// recorder and its cleanup keep working between recordings.
func TestAPIKeyRecorderDryRun(t *testing.T) {
	fake := newAPIKeyFidelityFake()
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	// A key that is not ours: the recorder must leave it, and the list gives it the team id.
	ours := fake.seed(apiKeySeed{name: "not-ours", permissions: []string{"livetail:Read"}})
	outPath := filepath.Join(t.TempDir(), "recorded.json")
	// The subtest runs the cleanup of the recorder when it ends.
	t.Run("record", func(t *testing.T) {
		recordAPIKeyFidelity(t, srv.URL, "dry-run-key", outPath)
	})
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("recorder did not write output: %v", err)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if _, ok := fake.keys[ours]; !ok {
		t.Error("cleanup deleted a key that the recorder did not create")
	}
	if len(fake.keys) != 1 {
		t.Errorf("cleanup left %d keys, want only the key that is not ours", len(fake.keys)-1)
	}
}
