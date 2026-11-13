// Copyright 2026 Coralogix Ltd.
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
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
)

const (
	aiceTypeName  = "coralogix_ai_custom_evaluation"
	aiceGoldenDir = "testdata/ai_custom_evaluation_baseline"
	// aiceRoutePath is the route that the steps of cases.json are relative to.
	aiceRoutePath = "/ai"
	aiceListPath  = aiceRoutePath + "/custom-evaluations/v3"
)

// TestAICustomEvaluationFakeMatchesRecordedBackend checks the fake against a live API recording.
func TestAICustomEvaluationFakeMatchesRecordedBackend(t *testing.T) {
	cases, recorded := loadFidelityAt(t, aiceGoldenDir)
	for ci, c := range cases.Cases {
		t.Run(c.Name, func(t *testing.T) {
			rc := recorded.Cases[ci]
			if rc.Name != c.Name || len(rc.Steps) != len(c.Steps) {
				t.Fatalf("recorded.json does not match cases.json for %q: record again", c.Name)
			}
			fake := newAICEFidelityFake()
			// The tenant of the recording has applications. The fake has three, as many as the
			// cases need to tell them apart.
			for _, name := range []string{"first", "second", "third"} {
				fake.seedApp(name, "sub-"+name)
			}
			ids := map[string]string{}
			for si, step := range c.Steps {
				label := fmt.Sprintf("step %d: %s %s", si+1, step.Method, step.Path)
				status, got := aiceFidelitySend(t, step, "fidelity", ids, func(method, path string, body any) (int, any) {
					u, err := url.Parse("/mgmt/openapi/5" + aiceRoutePath + path)
					if err != nil {
						t.Fatal(err)
					}
					return fake.handle(method, u.Path, u.Query(), body)
				})
				compareFidelityStep(t, label, status, got, rc.Steps[si].Status, rc.Steps[si].Response)
			}
		})
	}
}

// aiceFidelitySend sends one step with the run name and the captured ids, captures the ids that
// the step names, and returns the normalized answer.
func aiceFidelitySend(t *testing.T, step fidelityStep, run string, ids map[string]string, send func(method, path string, body any) (int, any)) (int, any) {
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
	if status == http.StatusOK {
		object := asObject(answer)
		switch {
		case step.Method == http.MethodPost && strings.HasSuffix(step.Path, "/custom-evaluations/v3"):
			// Every evaluation that a Create made is kept under a name of its own, captured or not,
			// so that the recorder can delete it.
			id, _ := asObject(object["item"])["id"].(string)
			ids[fmt.Sprintf("created-%d", len(ids))] = id
			if step.Capture != "" {
				ids[step.Capture] = id
			}
		case step.Method == http.MethodGet && strings.HasPrefix(step.Path, "/applications/v3"):
			if _, known := ids["APP1"]; !known {
				apps := asSlice(object["aiApplications"])
				for i, name := range []string{"APP1", "APP2"} {
					if i < len(apps) {
						ids[name], _ = asObject(apps[i])["id"].(string)
					}
				}
			}
		}
	}
	// The ids that the answer holds include those of this step.
	known := map[string]string{"NAME": run}
	for name, id := range ids {
		if !strings.HasPrefix(name, "created-") {
			known[name] = id
		}
	}
	// The real list of applications has no stable order: two calls with the page size 1 can answer
	// with the same application for different pages. A step with that page size compares only the
	// shape, so its application ids are masked as well.
	if strings.HasPrefix(step.Path, "/applications/v3?page_size=1") {
		known = map[string]string{"NAME": run}
	}
	return status, normalizeAICEFidelity(unsubstIDs(t, answer, known), run)
}

// unsubstIDs replaces the captured ids in an answer with their variable names, but not the run name.
func unsubstIDs(t *testing.T, answer any, vars map[string]string) any {
	t.Helper()
	ids := map[string]string{}
	for name, id := range vars {
		if name != "NAME" {
			ids[name] = id
		}
	}
	return unsubstJSON(t, answer, ids)
}

// normalizeAICEFidelity masks what differs in every run: the run name, the company, the times, and the
// names of the applications, which belong to the tenant. A list of evaluations keeps only the
// evaluations of the run, sorted by name, because it holds every evaluation of the tenant. The
// application ids of an evaluation are sorted, because the real API does not keep an order.
func normalizeAICEFidelity(value any, run string) any {
	switch x := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, item := range x {
			switch key {
			case "items":
				out[key] = normalizeAICEList(item, run)
			case "aiApplications":
				out[key] = normalizeAICEApplications(item)
			case "applicationIds":
				out[key] = sortedStrings(item)
			case "message":
				message, _ := item.(string)
				out[key] = strings.ReplaceAll(message, run, "{{NAME}}")
			default:
				out[key] = normalizeAICEFidelity(item, run)
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = normalizeAICEFidelity(item, run)
		}
		return out
	case string:
		return strings.ReplaceAll(x, run, "{{NAME}}")
	default:
		return value
	}
}

func sortedStrings(value any) any {
	items, ok := value.([]any)
	if !ok {
		return value
	}
	out := append([]any{}, items...)
	sort.Slice(out, func(i, j int) bool { return fmt.Sprint(out[i]) < fmt.Sprint(out[j]) })
	return out
}

// normalizeAICEList keeps the evaluations whose name starts with the run name.
func normalizeAICEList(value any, run string) any {
	items, _ := value.([]any)
	kept := []any{}
	for _, item := range items {
		evaluation := asObject(item)
		name, _ := evaluation["name"].(string)
		if !strings.HasPrefix(name, run+"-") {
			continue
		}
		kept = append(kept, normalizeAICEFidelity(evaluation, run))
	}
	sort.Slice(kept, func(i, j int) bool {
		return asObject(kept[i])["name"].(string) < asObject(kept[j])["name"].(string)
	})
	return kept
}

// normalizeAICEApplications masks the names and the metadata of the applications of the tenant. It
// keeps the captured ids (the test replaces them with their variables) and the names of the keys, so that a key that
// the fake lacks shows.
func normalizeAICEApplications(value any) any {
	items, _ := value.([]any)
	out := make([]any, 0, len(items))
	for _, item := range items {
		app := asObject(item)
		masked := map[string]any{}
		for key, v := range app {
			switch key {
			case "id":
				// A captured id was replaced by its variable. Any other id belongs to the tenant.
				if text, _ := v.(string); strings.HasPrefix(text, "{{") {
					masked[key] = v
				} else {
					masked[key] = "<app-id>"
				}
			case "application":
				masked[key] = "<application>"
			case "subsystem":
				masked[key] = "<subsystem>"
			case "companyId":
				masked[key] = "<company-id>"
			case "createdAt", "updatedAt":
				masked[key] = "<time>"
			default:
				masked[key] = "<" + key + ">"
			}
		}
		out = append(out, masked)
	}
	return out
}

// TestRecordAICustomEvaluationFidelity writes real API answers for cases.json. It needs a key that
// can manage AI custom evaluations and read AI applications. The recorder deletes only the
// evaluations that it created.
func TestRecordAICustomEvaluationFidelity(t *testing.T) {
	if os.Getenv("RECORD_AI_CUSTOM_EVALUATION_FIDELITY") != "1" {
		t.Skip("set RECORD_AI_CUSTOM_EVALUATION_FIDELITY=1 and a key to record")
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
	recordAICustomEvaluationFidelity(t, base, apiKey, filepath.Join(aiceGoldenDir, "fidelity", "recorded.json"))
}

func recordAICustomEvaluationFidelity(t *testing.T, base, apiKey, outPath string) {
	t.Helper()
	rawCases, err := os.ReadFile(filepath.Join(aiceGoldenDir, "fidelity", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases fidelityCases
	if err := json.Unmarshal(rawCases, &cases); err != nil {
		t.Fatal(err)
	}
	// Each case has a run name of its own, so that the list of a case shows only its evaluations.
	var runs []string
	created := map[string]bool{}
	t.Cleanup(func() {
		if err := cleanupAICustomEvaluationFidelity(base, apiKey, runs, created); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	send := func(method, path string, body any) (int, any) {
		return sendAICustomEvaluationFidelityRequest(base, apiKey, method, aiceRoutePath+path, body)
	}
	type recordedStep struct {
		Status   int `json:"status"`
		Response any `json:"response"`
	}
	type recordedCase struct {
		Name  string         `json:"name"`
		Steps []recordedStep `json:"steps"`
	}
	out := struct {
		Cases []recordedCase `json:"cases"`
	}{}
	for _, c := range cases.Cases {
		rc := recordedCase{Name: c.Name}
		run := newAICustomEvaluationFidelityRunID(t)
		runs = append(runs, run)
		ids := map[string]string{}
		for _, step := range c.Steps {
			status, answer := aiceFidelitySend(t, step, run, ids, send)
			if status == http.StatusUnauthorized || status == http.StatusForbidden {
				t.Fatalf("the API answered %d: recording needs a key that can manage AI custom evaluations", status)
			}
			for _, id := range ids {
				created[id] = true
			}
			rc.Steps = append(rc.Steps, recordedStep{Status: status, Response: answer})
		}
		out.Cases = append(out.Cases, rc)
	}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outPath, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newAICustomEvaluationFidelityRunID(t *testing.T) string {
	t.Helper()
	unique := make([]byte, 6)
	if _, err := rand.Read(unique); err != nil {
		t.Fatal(err)
	}
	return "tf-ai-ce-fidelity-" + hex.EncodeToString(unique)
}

// cleanupAICustomEvaluationFidelity deletes the evaluations that the runs created: the ids that
// Create returned, and every evaluation whose name starts with a run name. The captured ids include
// the ids of applications, whose Delete answers 404 for an evaluation and does no harm; they are
// not sent, because only the ids of evaluations are in the created set of a Create.
func cleanupAICustomEvaluationFidelity(base, apiKey string, runs []string, created map[string]bool) error {
	status, answer := sendAICustomEvaluationFidelityRequest(base, apiKey, http.MethodGet, aiceListPath, nil)
	if status != http.StatusOK {
		return fmt.Errorf("list evaluations answered %d", status)
	}
	ids := map[string]bool{}
	for _, item := range asSlice(asObject(answer)["items"]) {
		evaluation := asObject(item)
		name, _ := evaluation["name"].(string)
		id, _ := evaluation["id"].(string)
		for _, run := range runs {
			if strings.HasPrefix(name, run+"-") {
				ids[id] = true
			}
		}
		// An evaluation that a case renamed to the empty name has no run name left, so the id that
		// Create returned finds it.
		if created[id] {
			ids[id] = true
		}
	}
	var problems []string
	for id := range ids {
		status, _ := sendAICustomEvaluationFidelityRequest(base, apiKey, http.MethodDelete, aiceListPath+"/"+id, nil)
		if status != http.StatusOK && status != http.StatusNotFound {
			problems = append(problems, fmt.Sprintf("evaluation %s: delete answered %d", id, status))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("evaluations left behind: %s", strings.Join(problems, "; "))
	}
	return nil
}

func sendAICustomEvaluationFidelityRequest(base, apiKey, method, path string, body any) (int, any) {
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
	req.Header.Set("User-Agent", "terraform-provider-coralogix-baseline-recorder")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, map[string]any{"message": err.Error()}
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	var parsed any
	if len(bytes.TrimSpace(raw)) == 0 {
		parsed = map[string]any{}
	} else if json.Unmarshal(raw, &parsed) != nil {
		parsed = map[string]any{"raw": string(raw)}
	}
	return resp.StatusCode, parsed
}

// TestAICustomEvaluationRecorderDryRun runs the recorder against the fake, without credentials, so
// the recorder and its cleanup keep working between recordings.
func TestAICustomEvaluationRecorderDryRun(t *testing.T) {
	fake := newAICEFidelityFake()
	for _, name := range []string{"first", "second", "third"} {
		fake.seedApp(name, "sub-"+name)
	}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	ours := fake.seedEvaluation("not-ours")
	outPath := filepath.Join(t.TempDir(), "recorded.json")
	// The subtest runs the cleanup of the recorder when it ends.
	t.Run("record", func(t *testing.T) {
		recordAICustomEvaluationFidelity(t, srv.URL, "dry-run-key", outPath)
	})
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("recorder did not write output: %v", err)
	}
	if names := fake.sortedNames(); len(names) != 1 || names[0] != "not-ours" {
		t.Errorf("cleanup left or deleted evaluations: %v", names)
	}
	if fake.find(ours) == nil {
		t.Error("cleanup deleted an evaluation that the recorder did not create")
	}
}
