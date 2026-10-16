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
	"os"
	"path/filepath"
	"strings"
	"testing"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
)

// aiEvaluationCase is one case of fidelity/cases.json. A case that needs an
// AI application runs only on a tenant that has one: the API cannot create it.
type aiEvaluationCase struct {
	Name             string         `json:"name"`
	NeedsApplication bool           `json:"needsApplication"`
	Steps            []fidelityStep `json:"steps"`
}

type aiEvaluationRecordedCase struct {
	Name    string `json:"name"`
	Skipped bool   `json:"skipped,omitempty"`
	Steps   []struct {
		Status   int `json:"status"`
		Response any `json:"response"`
	} `json:"steps"`
}

func loadAIEvaluationFidelity(t *testing.T) ([]aiEvaluationCase, []aiEvaluationRecordedCase) {
	t.Helper()
	dir := filepath.Join(aiEvaluationGoldenDir, "fidelity")
	rawRecorded, err := os.ReadFile(filepath.Join(dir, "recorded.json"))
	if os.IsNotExist(err) {
		t.Skip("fidelity/recorded.json does not exist: run the recorder first")
	}
	if err != nil {
		t.Fatal(err)
	}
	var cases struct {
		Cases []aiEvaluationCase `json:"cases"`
	}
	var recorded struct {
		Cases []aiEvaluationRecordedCase `json:"cases"`
	}
	rawCases, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(rawCases, &cases); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(rawRecorded, &recorded); err != nil {
		t.Fatal(err)
	}
	if len(cases.Cases) != len(recorded.Cases) {
		t.Fatalf("cases.json has %d cases and recorded.json has %d: record again", len(cases.Cases), len(recorded.Cases))
	}
	return cases.Cases, recorded.Cases
}

// TestAIEvaluationFakeMatchesRecordedBackend checks the fake evaluation backend
// against recorded.json. Record that file with RECORD_AI_EVALUATION_FIDELITY=1
// against a tenant. A case that needs an AI application is skipped when the
// recording tenant had none.
func TestAIEvaluationFakeMatchesRecordedBackend(t *testing.T) {
	cases, recorded := loadAIEvaluationFidelity(t)
	fake := &aiEvalFake{}
	fake.reset()
	for ci, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			rc := recorded[ci]
			if rc.Name != c.Name {
				t.Fatalf("recorded.json does not match cases.json for %q: record again", c.Name)
			}
			if rc.Skipped {
				t.Skip("the recording tenant had no AI application")
			}
			if len(rc.Steps) != len(c.Steps) {
				t.Fatalf("recorded.json has %d steps for %q, cases.json %d: record again", len(rc.Steps), c.Name, len(c.Steps))
			}
			vars := map[string]string{"application": aiEvalFakeApplication, "subsystem": aiEvalFakeSubsystem}
			for si, s := range c.Steps {
				label := fmt.Sprintf("step %d: %s %s", si+1, s.Method, s.Path)
				status, got := runAIEvaluationFidelityStep(t, fake, s, vars)
				compareFidelityStep(t, label, status, normalizeAIEvaluation(got), rc.Steps[si].Status, normalizeAIEvaluation(rc.Steps[si].Response))
			}
		})
	}
}

func runAIEvaluationFidelityStep(t *testing.T, fake *aiEvalFake, s fidelityStep, vars map[string]string) (int, any) {
	t.Helper()
	body := s.Body
	if body != nil {
		body = substJSON(t, body, vars)
	}
	fake.mu.Lock()
	status, resp := fake.handle(s.Method, aiEvalBasePath+substString(s.Path, vars), body)
	fake.mu.Unlock()
	got := roundTrip(t, resp)
	if s.Capture != "" && status == http.StatusOK {
		vars[s.Capture] = aiEvaluationID(got)
	}
	return status, unsubstAIValues(got, vars)
}

func aiEvaluationID(v any) string {
	m, _ := v.(map[string]any)
	evaluation, _ := m["aiEvaluation"].(map[string]any)
	id, _ := evaluation["id"].(string)
	return id
}

// normalizeAIEvaluation replaces what differs in every run and on every tenant:
// times, the creating user, and the company.
func normalizeAIEvaluation(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, e := range x {
			switch k {
			case "createdAt", "updatedAt":
				out[k] = "<time>"
			case "createdBy":
				out[k] = "<user>"
			case "companyId":
				out[k] = "<company>"
			default:
				out[k] = normalizeAIEvaluation(e)
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = normalizeAIEvaluation(e)
		}
		return out
	}
	return v
}

// redactAITenant writes "<company>" in place of every companyId and "<user>"
// in place of every nonempty createdBy, so that a recording does not publish
// the tenant or the user it was made with.
func redactAITenant(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, e := range x {
			switch {
			case k == "companyId":
				out[k] = "<company>"
			case k == "createdBy" && e != "":
				out[k] = "<user>"
			default:
				out[k] = redactAITenant(e)
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = redactAITenant(e)
		}
		return out
	}
	return v
}

// unsubstAIValues writes "{{name}}" in place of the value of each key "name"
// whose value equals the variable of that name. It leaves other keys and
// strings alone, so a short application name such as "app" cannot rewrite the
// key "application", and an application and subsystem of the same name keep
// their own placeholders.
func unsubstAIValues(v any, vars map[string]string) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, e := range x {
			if value, ok := vars[k]; ok && value != "" && e == value {
				out[k] = "{{" + k + "}}"
				continue
			}
			out[k] = unsubstAIValues(e, vars)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = unsubstAIValues(e, vars)
		}
		return out
	}
	return v
}

// TestRecordAIEvaluationFidelity sends fidelity/cases.json to a real tenant and
// writes fidelity/recorded.json. Set RECORD_AI_EVALUATION_FIDELITY=1 with
// CORALOGIX_API_KEY and CORALOGIX_ENV (or CX_REGION). The cases that create an
// evaluation need an AI application: set CORALOGIX_AI_APPLICATION and
// CORALOGIX_AI_SUBSYSTEM, or the recorder uses the first one the tenant has.
// Without one, those cases are recorded as skipped. Created evaluations are
// deleted afterwards.
func TestRecordAIEvaluationFidelity(t *testing.T) {
	if os.Getenv("RECORD_AI_EVALUATION_FIDELITY") != "1" {
		t.Skip("set RECORD_AI_EVALUATION_FIDELITY=1 with CORALOGIX_API_KEY and CORALOGIX_ENV to record against a tenant")
	}
	apiKey := os.Getenv("CORALOGIX_API_KEY")
	if apiKey == "" {
		t.Fatal("CORALOGIX_API_KEY is required to record")
	}
	region := os.Getenv("CORALOGIX_ENV")
	if region == "" {
		region = os.Getenv("CX_REGION")
	}
	base, ok := cxsdkOpenapi.URLFromRegion(strings.ToLower(region))
	if !ok {
		t.Fatalf("unknown region %q", region)
	}
	rec := aiEvaluationRecorder{t: t, base: strings.TrimRight(base, "/"), apiKey: apiKey}
	application, subsystem := os.Getenv("CORALOGIX_AI_APPLICATION"), os.Getenv("CORALOGIX_AI_SUBSYSTEM")
	if application == "" {
		application, subsystem = rec.firstApplication()
	}
	hasApplication := application != ""
	if !hasApplication {
		// The cases that do not create an evaluation fail before the service
		// looks up the application, so any name records the same answer.
		application, subsystem = aiEvalFakeApplication, aiEvalFakeSubsystem
	}
	t.Cleanup(rec.cleanup)

	rawCases, err := os.ReadFile(filepath.Join(aiEvaluationGoldenDir, "fidelity", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases struct {
		Cases []aiEvaluationCase `json:"cases"`
	}
	if err := json.Unmarshal(rawCases, &cases); err != nil {
		t.Fatal(err)
	}
	var out struct {
		Cases []aiEvaluationRecordedCase `json:"cases"`
	}
	for _, c := range cases.Cases {
		rc := aiEvaluationRecordedCase{Name: c.Name}
		if c.NeedsApplication && !hasApplication {
			rc.Skipped = true
			out.Cases = append(out.Cases, rc)
			continue
		}
		vars := map[string]string{"application": application, "subsystem": subsystem}
		for _, s := range c.Steps {
			status, got := rec.step(s, vars)
			if s.Capture != "" && status != http.StatusOK {
				// Later steps would send an unresolved "{{id}}", and the recording
				// could not match the empty fake. Stop before writing it.
				t.Fatalf("%q: the create step answered %d. The application %q may already have an evaluation of this type and target: delete it, or set CORALOGIX_AI_APPLICATION and CORALOGIX_AI_SUBSYSTEM to an application with free slots", c.Name, status, application)
			}
			if status == http.StatusUnauthorized || status == http.StatusForbidden {
				t.Fatalf("the API answered %d: the key cannot manage AI evaluations", status)
			}
			rc.Steps = append(rc.Steps, struct {
				Status   int `json:"status"`
				Response any `json:"response"`
			}{Status: status, Response: redactAITenant(got)})
		}
		out.Cases = append(out.Cases, rc)
	}
	// Without HTML escaping, "<company>" stays readable in the file.
	var raw bytes.Buffer
	enc := json.NewEncoder(&raw)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(aiEvaluationGoldenDir, "fidelity", "recorded.json"), raw.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

type aiEvaluationRecorder struct {
	t       *testing.T
	base    string
	apiKey  string
	created []string
}

func (r *aiEvaluationRecorder) do(method, path string, body any) (int, any) {
	r.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			r.t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, r.base+path, reader)
	if err != nil {
		r.t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		r.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		r.t.Fatal(err)
	}
	var parsed any = map[string]any{}
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &parsed); err != nil {
			parsed = map[string]any{"raw": string(raw)}
		}
	}
	return resp.StatusCode, parsed
}

func (r *aiEvaluationRecorder) firstApplication() (string, string) {
	status, body := r.do(http.MethodGet, "/ai/applications/v3", nil)
	if status != http.StatusOK {
		r.t.Fatalf("listing AI applications answered %d: %s", status, compact(body))
	}
	m, _ := body.(map[string]any)
	apps, _ := m["aiApplications"].([]any)
	for _, a := range apps {
		app, _ := a.(map[string]any)
		application, _ := app["application"].(string)
		subsystem, _ := app["subsystem"].(string)
		if application != "" && subsystem != "" {
			return application, subsystem
		}
	}
	return "", ""
}

func (r *aiEvaluationRecorder) step(s fidelityStep, vars map[string]string) (int, any) {
	r.t.Helper()
	var body any
	if s.Body != nil {
		body = substJSON(r.t, s.Body, vars)
	}
	status, got := r.do(s.Method, aiEvalBasePath+substString(s.Path, vars), body)
	if s.Capture != "" && status == http.StatusOK {
		if id := aiEvaluationID(got); id != "" {
			vars[s.Capture] = id
			r.created = append(r.created, id)
		}
	}
	return status, unsubstAIValues(got, vars)
}

func (r *aiEvaluationRecorder) cleanup() {
	for _, id := range r.created {
		req, err := http.NewRequest(http.MethodDelete, r.base+aiEvalBasePath+"/"+id, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", "Bearer "+r.apiKey)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
}

func TestAIEvaluationRecorderValues(t *testing.T) {
	got := unsubstAIValues(redactAITenant(map[string]any{
		"aiEvaluation": map[string]any{
			"application": "app",
			"subsystem":   "sub",
			"companyId":   "1234",
			"createdBy":   "someone",
			"message":     "the app was not found",
		},
		"list": []any{map[string]any{"createdBy": ""}},
	}), map[string]string{"application": "app", "subsystem": "sub"})
	want := map[string]any{
		"aiEvaluation": map[string]any{
			"application": "{{application}}",
			"subsystem":   "{{subsystem}}",
			"companyId":   "<company>",
			"createdBy":   "<user>",
			"message":     "the app was not found",
		},
		"list": []any{map[string]any{"createdBy": ""}},
	}
	if got, want := unsubstAIValues(map[string]any{"application": "same", "subsystem": "same"}, map[string]string{"application": "same", "subsystem": "same"}),
		(map[string]any{"application": "{{application}}", "subsystem": "{{subsystem}}"}); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("same names: got %v, want %v", got, want)
	}
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("got %s\nwant %s", gotJSON, wantJSON)
	}
}
