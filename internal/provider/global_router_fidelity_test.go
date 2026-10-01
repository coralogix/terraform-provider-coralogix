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
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
)

type fidelityStep struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Body    any    `json:"body"`
	Capture string `json:"capture"`
}

type fidelityCases struct {
	Cases []struct {
		Name  string         `json:"name"`
		Steps []fidelityStep `json:"steps"`
	} `json:"cases"`
}

type fidelityRecorded struct {
	Cases []struct {
		Name  string `json:"name"`
		Steps []struct {
			Status   int `json:"status"`
			Response any `json:"response"`
		} `json:"steps"`
	} `json:"cases"`
}

// loadFidelity reads the cases and the recorded answers. It skips the test when there is no recording.
func loadFidelity(t *testing.T) (fidelityCases, fidelityRecorded) {
	t.Helper()
	dir := filepath.Join(grGoldenDir, "fidelity")
	rawRecorded, err := os.ReadFile(filepath.Join(dir, "recorded.json"))
	if errors.Is(err, fs.ErrNotExist) {
		t.Skip("fidelity/recorded.json does not exist: run the recorder first")
	}
	if err != nil {
		t.Fatal(err)
	}
	rawCases, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases fidelityCases
	var recorded fidelityRecorded
	if err := json.Unmarshal(rawCases, &cases); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(rawRecorded, &recorded); err != nil {
		t.Fatal(err)
	}
	if len(cases.Cases) != len(recorded.Cases) {
		t.Fatalf("cases.json has %d cases and recorded.json has %d: record again", len(cases.Cases), len(recorded.Cases))
	}
	return cases, recorded
}

// TestGlobalRouterFakeMatchesRecordedBackend checks the fake backend against
// the answers of the real API. The answers are in fidelity/recorded.json. A
// recorder made that file by sending the requests in fidelity/cases.json to a
// tenant. This test sends the same requests to the fake and compares the
// status codes and the response bodies. It skips when recorded.json does not exist.
//
// A difference means that the fake does not behave like the real API, so the
// golden files of the baseline may not show what the provider does for real.
func TestGlobalRouterFakeMatchesRecordedBackend(t *testing.T) {
	cases, recorded := loadFidelity(t)
	fake := &grFake{routers: map[string]map[string]any{}}
	for ci, c := range cases.Cases {
		t.Run(c.Name, func(t *testing.T) {
			rc := recorded.Cases[ci]
			if rc.Name != c.Name || len(rc.Steps) != len(c.Steps) {
				t.Fatalf("recorded.json does not match cases.json for %q: record again", c.Name)
			}
			vars := map[string]string{}
			for si, s := range c.Steps {
				label := fmt.Sprintf("step %d: %s %s", si+1, s.Method, s.Path)
				status, got := runFidelityStep(t, fake, s, vars)
				compareFidelityStep(t, label, status, got, rc.Steps[si].Status, rc.Steps[si].Response)
			}
		})
	}
}

// runFidelityStep sends one step to the fake. It returns the status and the response, with the
// id that the server made replaced by its token.
func runFidelityStep(t *testing.T, fake *grFake, s fidelityStep, vars map[string]string) (int, any) {
	t.Helper()
	body := s.Body
	if body != nil {
		body = substJSON(t, body, vars)
	}
	fake.mu.Lock()
	status, resp := fake.handle(s.Method, grBasePath+substString(s.Path, vars), body)
	fake.mu.Unlock()
	got := roundTrip(t, resp)
	if s.Capture != "" && status == 200 {
		router, _ := got.(map[string]any)["router"].(map[string]any)
		vars[s.Capture], _ = router["id"].(string)
	}
	return status, unsubstJSON(t, got, vars)
}

func compareFidelityStep(t *testing.T, label string, status int, got any, wantStatus int, want any) {
	t.Helper()
	if status != wantStatus {
		t.Errorf("%s: status %d, real API %d\n  fake response: %s\n  real response: %s", label, status, wantStatus, compact(got), compact(want))
		return
	}
	if status != 200 {
		// The provider shows this text to the user, so it must be the same.
		if g, w := normalizeMessage(errorMessage(got)), normalizeMessage(errorMessage(want)); g != w {
			t.Errorf("%s: same status %d, different message\n  fake: %s\n  real: %s", label, status, g, w)
		}
		return
	}
	if diff := firstJSONDifference("", normalizeFidelity(got), normalizeFidelity(want)); diff != "" {
		t.Errorf("%s: response differs from the real API at %s\n  fake: %s\n  real: %s", label, diff, compact(got), compact(want))
	}
}

func roundTrip(t *testing.T, v any) any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func substString(s string, vars map[string]string) string {
	for name, value := range vars {
		s = strings.ReplaceAll(s, "{{"+name+"}}", value)
	}
	return s
}

func substJSON(t *testing.T, v any, vars map[string]string) any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal([]byte(substString(string(raw), vars)), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func unsubstJSON(t *testing.T, v any, vars map[string]string) any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for name, value := range vars {
		if value != "" {
			s = strings.ReplaceAll(s, value, "{{"+name+"}}")
		}
	}
	var out any
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// normalizeFidelity replaces the values that are different in every run:
// timestamps, and the ids that the server makes for targets.
func normalizeFidelity(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		_, isTarget := x["connectorId"]
		for k, e := range x {
			switch {
			case k == "id" && isTarget:
				out[k] = "<id>"
			case k == "createTime" || k == "updateTime":
				out[k] = "<time>"
			default:
				out[k] = normalizeFidelity(e)
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = normalizeFidelity(e)
		}
		return out
	}
	return v
}

// firstJSONDifference returns the path of the first difference, or "".
func firstJSONDifference(path string, a, b any) string {
	switch x := a.(type) {
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok {
			return pathOrRoot(path)
		}
		keys := slices.Sorted(maps.Keys(x))
		for k := range y {
			if _, ok := x[k]; !ok {
				keys = append(keys, k)
			}
		}
		slices.Sort(keys)
		for _, k := range slices.Compact(keys) {
			av, aok := x[k]
			bv, bok := y[k]
			sub := path + "." + k
			switch {
			case aok != bok:
				return sub + " (key only in the " + map[bool]string{true: "fake", false: "real response"}[aok] + ")"
			default:
				if d := firstJSONDifference(sub, av, bv); d != "" {
					return d
				}
			}
		}
		return ""
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return pathOrRoot(path) + " (list length)"
		}
		for i := range x {
			if d := firstJSONDifference(fmt.Sprintf("%s[%d]", path, i), x[i], y[i]); d != "" {
				return d
			}
		}
		return ""
	}
	if !reflect.DeepEqual(a, b) {
		return pathOrRoot(path)
	}
	return ""
}

func pathOrRoot(p string) string {
	if p == "" {
		return "(root)"
	}
	return p
}

func compact(v any) string {
	raw, _ := json.Marshal(v)
	if len(raw) > 900 {
		return string(raw[:900]) + "..."
	}
	return string(raw)
}

var (
	uuidPattern    = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	linePosPattern = regexp.MustCompile(`\(line \d+:\d+\)`)
)

// normalizeMessage removes what differs in every run: generated ids and the position in the request text.
func normalizeMessage(s string) string {
	s = uuidPattern.ReplaceAllString(s, "<uuid>")
	return linePosPattern.ReplaceAllString(s, "(line L:C)")
}

func errorMessage(v any) string {
	m, _ := v.(map[string]any)
	s, _ := m["message"].(string)
	return s
}
