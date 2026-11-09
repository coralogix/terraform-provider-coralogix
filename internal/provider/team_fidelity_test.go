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
	"strconv"
	"strings"
	"testing"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
)

// TestTeamFakeMatchesRecordedBackend checks the fake against a live API recording.
func TestTeamFakeMatchesRecordedBackend(t *testing.T) {
	cases, recorded := loadFidelityAt(t, teamGoldenDir)
	for ci, c := range cases.Cases {
		t.Run(c.Name, func(t *testing.T) {
			rc := recorded.Cases[ci]
			if rc.Name != c.Name || len(rc.Steps) != len(c.Steps) {
				t.Fatalf("recorded.json does not match cases.json for %q: record again", c.Name)
			}
			fake := newTeamFidelityFake()
			ids := map[string]string{}
			for si, step := range c.Steps {
				label := fmt.Sprintf("step %d: %s %s", si+1, step.Method, step.Path)
				status, got := teamFidelitySend(t, step, "fidelity", ids, func(method, path string, body any) (int, any) {
					return fake.handle(method, teamBasePath+path, body)
				})
				compareFidelityStep(t, label, status, got, rc.Steps[si].Status, rc.Steps[si].Response)
			}
		})
	}
}

// teamFidelitySend sends one step with the run name and the captured ids, captures the id that a
// Create returns, and returns the normalized answer.
func teamFidelitySend(t *testing.T, step fidelityStep, run string, ids map[string]string, send func(method, path string, body any) (int, any)) (int, any) {
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
	if step.Capture != "" && status == http.StatusOK {
		id, _ := asObject(asObject(answer)["teamId"])["id"].(float64)
		ids[step.Capture] = strconv.FormatInt(int64(id), 10)
	}
	return status, normalizeTeamFidelity(answer, run)
}

var (
	teamErrorIDPattern = regexp.MustCompile(`\nError ID: [0-9a-f]+$`)
	teamFlowIDPattern  = regexp.MustCompile(`, flow_id: [0-9a-f-]+\.$`)
)

// normalizeTeamFidelity masks team ids, the run name, and the request ids that the real API adds
// to some error messages.
func normalizeTeamFidelity(value any, run string) any {
	switch x := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, item := range x {
			switch {
			case key == "teamId":
				out[key] = map[string]any{"id": "<team-id>"}
			case key == "message":
				message, _ := item.(string)
				message = teamErrorIDPattern.ReplaceAllString(message, "")
				message = teamFlowIDPattern.ReplaceAllString(message, "")
				out[key] = strings.ReplaceAll(message, run, "{{NAME}}")
			default:
				out[key] = normalizeTeamFidelity(item, run)
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = normalizeTeamFidelity(item, run)
		}
		return out
	case string:
		return strings.ReplaceAll(x, run, "{{NAME}}")
	default:
		return value
	}
}

// TestRecordTeamFidelity writes real API answers for cases.json. It needs a key whose owner is
// an organization: a team key gets 403. Each case creates teams that move quota from the default
// team of the organization and gives it back when it deletes them. The recorder deletes only the
// teams whose name starts with its run name.
func TestRecordTeamFidelity(t *testing.T) {
	if os.Getenv("RECORD_TEAM_FIDELITY") != "1" {
		t.Skip("set RECORD_TEAM_FIDELITY=1 and an organization key to record")
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
	recordTeamFidelity(t, base, apiKey, filepath.Join(teamGoldenDir, "fidelity", "recorded.json"))
}

func recordTeamFidelity(t *testing.T, base, apiKey, outPath string) {
	t.Helper()
	rawCases, err := os.ReadFile(filepath.Join(teamGoldenDir, "fidelity", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases fidelityCases
	if err := json.Unmarshal(rawCases, &cases); err != nil {
		t.Fatal(err)
	}
	run := newTeamFidelityRunID(t)
	t.Cleanup(func() {
		if err := cleanupTeamFidelity(base, apiKey, run); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	send := func(method, path string, body any) (int, any) {
		return sendTeamFidelityRequest(base, apiKey, method, teamBasePath+path, body)
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
			status, answer := teamFidelitySend(t, step, run, ids, send)
			if status == http.StatusUnauthorized || status == http.StatusForbidden {
				t.Fatalf("the API answered %d: recording needs a key whose owner is an organization", status)
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

func newTeamFidelityRunID(t *testing.T) string {
	t.Helper()
	unique := make([]byte, 6)
	if _, err := rand.Read(unique); err != nil {
		t.Fatal(err)
	}
	return "tf-team-fidelity-" + hex.EncodeToString(unique)
}

// cleanupTeamFidelity deletes every active team whose name starts with the run name. A team with
// the minimum quota cannot be deleted, so it raises the quota first.
func cleanupTeamFidelity(base, apiKey, run string) error {
	status, answer := sendTeamFidelityRequest(base, apiKey, http.MethodGet, teamBasePath, nil)
	if status != http.StatusOK {
		return fmt.Errorf("list teams answered %d", status)
	}
	var problems []string
	for _, item := range asSlice(asObject(answer)["teams"]) {
		team := asObject(item)
		name, _ := team["teamName"].(string)
		if !strings.HasPrefix(name, run+"-") {
			continue
		}
		id, _ := asObject(team["teamId"])["id"].(float64)
		path := teamBasePath + "/" + strconv.FormatInt(int64(id), 10)
		if status, _ := sendTeamFidelityRequest(base, apiKey, http.MethodPut, path, map[string]any{"dailyQuota": 0.5}); status != http.StatusOK {
			problems = append(problems, fmt.Sprintf("%s: raise quota answered %d", name, status))
			continue
		}
		if status, _ := sendTeamFidelityRequest(base, apiKey, http.MethodDelete, path, nil); status != http.StatusOK {
			problems = append(problems, fmt.Sprintf("%s: delete answered %d", name, status))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("teams left behind: %s", strings.Join(problems, "; "))
	}
	return nil
}

func asSlice(value any) []any {
	items, _ := value.([]any)
	return items
}

func sendTeamFidelityRequest(base, apiKey, method, path string, body any) (int, any) {
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

// TestTeamRecorderDryRun runs the recorder against the fake, without credentials, so the recorder
// and its cleanup keep working between recordings.
func TestTeamRecorderDryRun(t *testing.T) {
	fake := newTeamFidelityFake()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, teamBasePath) {
			fake.mu.Lock()
			teams := []any{}
			for id, team := range fake.teams {
				if !team.deleted {
					teams = append(teams, map[string]any{"teamId": map[string]any{"id": id}, "teamName": team.name})
				}
			}
			fake.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"teams": teams})
			return
		}
		fake.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	ours := fake.seed("not-ours", 0.5)
	outPath := filepath.Join(t.TempDir(), "recorded.json")
	// The subtest runs the cleanup of the recorder when it ends.
	t.Run("record", func(t *testing.T) {
		recordTeamFidelity(t, srv.URL, "dry-run-key", outPath)
	})
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("recorder did not write output: %v", err)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	for id, team := range fake.teams {
		switch {
		case id == ours && team.deleted:
			t.Error("cleanup deleted a team that the recorder did not create")
		case id != ours && !team.deleted:
			t.Errorf("cleanup left team %q", team.name)
		}
	}
}
