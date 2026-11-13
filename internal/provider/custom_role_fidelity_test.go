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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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

// TestCustomRoleFakeMatchesRecordedBackend checks the fake against a live API recording.
func TestCustomRoleFakeMatchesRecordedBackend(t *testing.T) {
	cases, recorded := loadFidelityAt(t, customRoleGoldenDir)
	for ci, c := range cases.Cases {
		t.Run(c.Name, func(t *testing.T) {
			rc := recorded.Cases[ci]
			if rc.Name != c.Name || len(rc.Steps) != len(c.Steps) {
				t.Fatalf("recorded.json does not match cases.json for %q: record again", c.Name)
			}
			fake := newCustomRoleFidelityFake()
			ids := map[string]string{}
			for si, step := range c.Steps {
				label := fmt.Sprintf("step %d: %s %s", si+1, step.Method, step.Path)
				status, got := customRoleFidelitySend(t, step, "fidelity", ids, func(method, path string, body any) (int, any) {
					return fake.handle(method, customRoleBasePath+path, body)
				})
				compareFidelityStep(t, label, status, got, rc.Steps[si].Status, rc.Steps[si].Response)
			}
		})
	}
}

// customRoleFidelitySend sends one step with the run name and the captured ids, captures the id
// that a Create returns, and returns the normalized answer.
func customRoleFidelitySend(t *testing.T, step fidelityStep, run string, ids map[string]string, send func(method, path string, body any) (int, any)) (int, any) {
	t.Helper()
	vars := map[string]string{"NAME": run}
	for name, id := range ids {
		vars[name] = id
	}
	var body any
	if step.Body != nil {
		body = substJSON(t, step.Body, vars)
	}
	status, answer := send(step.Method, substString(step.Path, vars), body)
	answer = roundTrip(t, answer)
	if step.Method == http.MethodPut && status == http.StatusOK {
		// Every role that a Create made is kept under a name of its own, captured or not, so the
		// recorder can delete it.
		id, _ := asObject(answer)["id"].(float64)
		text := strconv.FormatInt(int64(id), 10)
		ids[fmt.Sprintf("created-%d", len(ids))] = text
		if step.Capture != "" {
			ids[step.Capture] = text
		}
	}
	return status, normalizeCustomRoleFidelity(answer, run)
}

var (
	customRoleErrorIDPattern = regexp.MustCompile(`\nError ID: [0-9a-f]+$`)
	customRoleFlowIDPattern  = regexp.MustCompile(`, flow_id: [0-9a-f-]+\.$`)
	customRoleTeamPattern    = regexp.MustCompile(`team [0-9]+`)
)

// customRoleInheritedPermissions marks the permissions that a role shows in the list: the real API
// lists them with the permissions of the parent role, and the parent's permissions change with the
// product, so the recording keeps only the fact.
const customRoleInheritedPermissions = "<permissions of the role and of its parent role>"

// normalizeCustomRoleFidelity masks the role ids, the team id, and the run name. A list keeps only
// the roles of the run, sorted by name, because it holds every role of the team.
func normalizeCustomRoleFidelity(value any, run string) any {
	switch x := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, item := range x {
			switch {
			case key == "id" || key == "roleId":
				out[key] = "<role-id>"
			case key == "teamId":
				out[key] = "<team-id>"
			case key == "roles":
				out[key] = normalizeCustomRoleList(item, run)
			case key == "permissions":
				out[key] = normalizeCustomRolePermissions(item)
			case key == "message":
				message, _ := item.(string)
				message = customRoleErrorIDPattern.ReplaceAllString(message, "")
				message = customRoleFlowIDPattern.ReplaceAllString(message, "")
				message = customRoleTeamPattern.ReplaceAllString(message, "team <team-id>")
				out[key] = strings.ReplaceAll(message, run, "{{NAME}}")
			default:
				out[key] = normalizeCustomRoleFidelity(item, run)
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = normalizeCustomRoleFidelity(item, run)
		}
		return out
	case string:
		return strings.ReplaceAll(x, run, "{{NAME}}")
	default:
		return value
	}
}

// normalizeCustomRolePermissions sorts the permissions of a role. The API returns them in another
// order after Create than after Update, and the resource shows them as a set.
func normalizeCustomRolePermissions(value any) any {
	items, ok := value.([]any)
	if !ok {
		return value
	}
	out := append([]any{}, items...)
	sort.Slice(out, func(i, j int) bool { return fmt.Sprint(out[i]) < fmt.Sprint(out[j]) })
	return out
}

// normalizeCustomRoleList keeps the roles whose name starts with the run name.
func normalizeCustomRoleList(value any, run string) any {
	items, _ := value.([]any)
	kept := []any{}
	for _, item := range items {
		role := asObject(item)
		name, _ := role["name"].(string)
		if !strings.HasPrefix(name, run+"-") {
			continue
		}
		normalized := asObject(normalizeCustomRoleFidelity(role, run))
		if permissions, ok := role["permissions"].([]any); ok && len(permissions) > 20 {
			normalized["permissions"] = customRoleInheritedPermissions
		}
		kept = append(kept, normalized)
	}
	sort.Slice(kept, func(i, j int) bool {
		return asObject(kept[i])["name"].(string) < asObject(kept[j])["name"].(string)
	})
	return kept
}

// TestRecordCustomRoleFidelity writes real API answers for cases.json. It needs a key that can
// manage custom roles. The recorder deletes only the roles that it created.
func TestRecordCustomRoleFidelity(t *testing.T) {
	if os.Getenv("RECORD_CUSTOM_ROLE_FIDELITY") != "1" {
		t.Skip("set RECORD_CUSTOM_ROLE_FIDELITY=1 and a key to record")
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
	recordCustomRoleFidelity(t, base, apiKey, filepath.Join(customRoleGoldenDir, "fidelity", "recorded.json"))
}

func recordCustomRoleFidelity(t *testing.T, base, apiKey, outPath string) {
	t.Helper()
	rawCases, err := os.ReadFile(filepath.Join(customRoleGoldenDir, "fidelity", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases fidelityCases
	if err := json.Unmarshal(rawCases, &cases); err != nil {
		t.Fatal(err)
	}
	// Each case has a run name of its own, so that the list of a case shows only its roles.
	var runs []string
	created := map[string]bool{}
	t.Cleanup(func() {
		if err := cleanupCustomRoleFidelity(base, apiKey, runs, created); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	send := func(method, path string, body any) (int, any) {
		return sendCustomRoleFidelityRequest(base, apiKey, method, customRoleBasePath+path, body)
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
		run := newCustomRoleFidelityRunID(t)
		runs = append(runs, run)
		ids := map[string]string{}
		for _, step := range c.Steps {
			status, answer := customRoleFidelitySend(t, step, run, ids, send)
			if status == http.StatusUnauthorized || status == http.StatusForbidden {
				t.Fatalf("the API answered %d: recording needs a key that can manage custom roles", status)
			}
			for _, id := range ids {
				created[id] = true
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

func newCustomRoleFidelityRunID(t *testing.T) string {
	t.Helper()
	unique := make([]byte, 6)
	if _, err := rand.Read(unique); err != nil {
		t.Fatal(err)
	}
	return "tf-role-fidelity-" + hex.EncodeToString(unique)
}

// cleanupCustomRoleFidelity deletes the roles that the runs created: the ids that Create returned,
// and every role whose name starts with a run name.
func cleanupCustomRoleFidelity(base, apiKey string, runs []string, created map[string]bool) error {
	status, answer := sendCustomRoleFidelityRequest(base, apiKey, http.MethodGet, customRoleBasePath, nil)
	if status != http.StatusOK {
		return fmt.Errorf("list roles answered %d", status)
	}
	ids := map[string]bool{}
	for id := range created {
		if id != "" {
			ids[id] = true
		}
	}
	for _, item := range asSlice(asObject(answer)["roles"]) {
		role := asObject(item)
		name, _ := role["name"].(string)
		for _, run := range runs {
			if strings.HasPrefix(name, run+"-") {
				id, _ := role["roleId"].(float64)
				ids[strconv.FormatInt(int64(id), 10)] = true
			}
		}
	}
	var problems []string
	for id := range ids {
		status, _ := sendCustomRoleFidelityRequest(base, apiKey, http.MethodDelete, customRoleBasePath+"/"+id, nil)
		if status != http.StatusOK && status != http.StatusNotFound {
			problems = append(problems, fmt.Sprintf("role %s: delete answered %d", id, status))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("roles left behind: %s", strings.Join(problems, "; "))
	}
	return nil
}

func sendCustomRoleFidelityRequest(base, apiKey, method, path string, body any) (int, any) {
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, strings.TrimRight(base, "/")+path, reader)
	if err != nil {
		return 0, map[string]any{"message": err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, map[string]any{"message": err.Error()}
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

// TestCustomRoleRecorderDryRun runs the recorder against the fake, without credentials, so the
// recorder and its cleanup keep working between recordings.
func TestCustomRoleRecorderDryRun(t *testing.T) {
	fake := newCustomRoleFidelityFake()
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	ours := fake.seed("not-ours", []string{"spans.events2metrics:ReadConfig"})
	outPath := filepath.Join(t.TempDir(), "recorded.json")
	// The subtest runs the cleanup of the recorder when it ends.
	t.Run("record", func(t *testing.T) {
		recordCustomRoleFidelity(t, srv.URL, "dry-run-key", outPath)
	})
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("recorder did not write output: %v", err)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if _, ok := fake.roles[ours]; !ok {
		t.Error("cleanup deleted a role that the recorder did not create")
	}
	for id, role := range fake.roles {
		if id != ours {
			t.Errorf("cleanup left role %q", role.name)
		}
	}
}
