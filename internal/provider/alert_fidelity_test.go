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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
)

const (
	alertRecordedDir = "fidelity"
)

var (
	alertUUIDPattern = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	// alertCompanyPattern matches the company id that some error messages of the API carry. A
	// recording must not hold it.
	alertCompanyPattern = regexp.MustCompile(`CompanyId\(\d+\)`)
	// alertSLOCompanyPattern matches the company id in the "not found" message of an SLO.
	alertSLOCompanyPattern = regexp.MustCompile(`for company \d+`)
	alertTimeKeys          = map[string]bool{
		"createdTime": true, "updatedTime": true, "lastTriggeredTime": true,
		// The times of an SLO.
		"createTime": true, "updateTime": true,
	}
	// alertPersonKeys name the properties that hold a person. An SLO has its creator, an email
	// address. A recording masks them by key, so it never holds a real address.
	alertPersonKeys = map[string]bool{"creator": true}
)

// alertRecorded is the recording of one scenario: every request that the provider sent to the
// real API and the answer, with ids replaced by <id:N> in order of appearance.
type alertRecorded map[string][]alertExchange

// alertNormalizer replaces every UUID by <id:N> and every server time by "<time>".
type alertNormalizer struct {
	ids map[string]string
}

func newAlertNormalizer() *alertNormalizer { return &alertNormalizer{ids: map[string]string{}} }

func (n *alertNormalizer) id(uuid string) string {
	if p, ok := n.ids[uuid]; ok {
		return p
	}
	p := fmt.Sprintf("<id:%d>", len(n.ids)+1)
	n.ids[uuid] = p
	return p
}

func (n *alertNormalizer) text(s string) string {
	s = alertCompanyPattern.ReplaceAllString(s, "CompanyId(<company>)")
	s = alertSLOCompanyPattern.ReplaceAllString(s, "for company <company>")
	return alertUUIDPattern.ReplaceAllStringFunc(s, n.id)
}

func (n *alertNormalizer) value(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			item := x[k]
			if alertTimeKeys[k] {
				out[k] = "<time>"
				continue
			}
			if alertPersonKeys[k] {
				out[k] = "<user>"
				continue
			}
			// The version id is server noise: no request sends it and Terraform never sees it. It is
			// masked, and not numbered, so that it cannot move the numbers of the ids that matter.
			if k == "alertVersionId" {
				out[k] = "<version>"
				continue
			}
			out[k] = n.value(item)
			if k == "access" {
				canonicalizeAlertPolicyText(out[k])
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = n.value(item)
		}
		return out
	case string:
		return n.text(x)
	}
	return v
}

// canonicalizeAlertPolicyText rewrites the access policy of an answer (the "access" block of a Get)
// with sorted keys and no spaces. The API writes the keys of the policy in a different order in
// every answer, and the order carries no meaning. A request keeps the text that the provider sent.
func canonicalizeAlertPolicyText(access any) {
	block, ok := access.(map[string]any)
	if !ok {
		return
	}
	text, ok := block["accessPolicy"].(string)
	if !ok {
		return
	}
	var parsed any
	if json.Unmarshal([]byte(text), &parsed) != nil {
		return
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if enc.Encode(parsed) == nil {
		block["accessPolicy"] = strings.TrimSuffix(buf.String(), "\n")
	}
}

func (n *alertNormalizer) exchange(e alertExchange) alertExchange {
	e.Path = n.text(strings.TrimPrefix(e.Path, "/mgmt/openapi/5"))
	e.Request = n.value(e.Request)
	e.Response = n.value(e.Response)
	return e
}

// denormalize puts the ids of this run back into a recorded request.
func (n *alertNormalizer) denormalize(v any) any {
	reverse := map[string]string{}
	for uuid, p := range n.ids {
		reverse[p] = uuid
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	text := buf.String()
	for p, uuid := range reverse {
		text = strings.ReplaceAll(text, p, uuid)
	}
	var out any
	_ = json.Unmarshal([]byte(text), &out)
	return out
}

// readAlertRecorded reads every recording. Each scenario has a file of its own, fidelity/<name>.json,
// so a group of scenarios records without touching the files of the others.
func readAlertRecorded(t *testing.T) alertRecorded {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(alertGoldenDir, alertRecordedDir, "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no recording in %s: %v", alertRecordedDir, err)
	}
	out := alertRecorded{}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var exchanges []alertExchange
		if err := json.Unmarshal(raw, &exchanges); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		out[strings.TrimSuffix(filepath.Base(file), ".json")] = exchanges
	}
	return out
}

// TestAlertFakeMatchesRecordedBackend replays every request that the provider sent to the real
// API during the recording to the fake, and compares the answers.
func TestAlertFakeMatchesRecordedBackend(t *testing.T) {
	recorded := readAlertRecorded(t)
	names := make([]string, 0, len(recorded))
	for name := range recorded {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			fake := &alertFake{alerts: map[string]map[string]any{}}
			norm := newAlertNormalizer()
			for i, want := range recorded[name] {
				label := fmt.Sprintf("exchange %d: %s %s", i+1, want.Method, want.Path)
				path := fmt.Sprint(norm.denormalize(want.Path))
				var body any
				if want.Request != nil {
					body = norm.denormalize(want.Request)
				}
				path, query, _ := strings.Cut(path, "?")
				values, _ := parseQuery(query)
				status, answer := fake.handle(want.Method, "/mgmt/openapi/5"+path, values, body)
				got := norm.exchange(alertExchange{Method: want.Method, Path: path, Request: want.Request, Status: status, Response: roundTrip(t, answer)})
				if got.Status != want.Status {
					if !alertRecordedPeriodRejection(got.Status, want) {
						t.Errorf("%s: status %d, recorded %d", label, got.Status, want.Status)
					}
					continue
				}
				if !reflect.DeepEqual(got.Response, want.Response) {
					diffs := alertUnrecordedBackendBehavior(alertJSONDiff("", got.Response, want.Response))
					if len(diffs) > 0 {
						t.Errorf("%s: the fake and the API differ at:\n  %s", label, strings.Join(diffs, "\n  "))
					}
				}
			}
		})
	}
}

// alertUnrecordedBackendBehavior drops the differences that come from a backend change that the
// recordings do not have yet. The recordings are answers of the real API, so they cannot be edited
// by hand.
//
// The fake models the backend that keeps the evaluation period of an analytics alert only when
// the request sets it, so it returns no period for a request that omits it. The API that the
// recordings come from still fills the period in, and returns the smaller of 10 and the timeframe.
// Only that one difference is dropped: the fake has no period and the API has one.
//
// When the new backend is live, record the analytics scenarios again (TestRecordAlertFidelity),
// check that the fake matches them, and delete this function and alertRecordedPeriodRejection.
func alertUnrecordedBackendBehavior(diffs []string) []string {
	var out []string
	for _, d := range diffs {
		isPeriod := strings.HasSuffix(strings.SplitN(d, ": the fake has none", 2)[0], ".evaluationPeriodMinutes")
		onAnalytics := strings.Contains(d, ".analyticsImmediate.") || strings.Contains(d, ".analyticsThreshold.")
		if isPeriod && onAnalytics && strings.Contains(d, ": the fake has none, the API has ") {
			continue
		}
		out = append(out, d)
	}
	return out
}

// alertRecordedPeriodRejection reports the other difference of the same backend change: the API
// that the recordings come from rejects a Replace that omits the evaluation period when the stored
// period exceeds the new timeframe, and the fake accepts it, because the new backend stores no
// period for a request that omits it.
func alertRecordedPeriodRejection(gotStatus int, want alertExchange) bool {
	if gotStatus != http.StatusOK || want.Status != http.StatusBadRequest {
		return false
	}
	message, _ := json.Marshal(want.Response)
	return want.Method == http.MethodPut && strings.Contains(string(message), "evaluationPeriodMinutes is omitted, so the alert keeps its current value")
}

func parseQuery(q string) (map[string][]string, error) {
	out := map[string][]string{}
	for _, part := range strings.Split(q, "&") {
		if part == "" {
			continue
		}
		k, v, _ := strings.Cut(part, "=")
		out[k] = append(out[k], v)
	}
	return out, nil
}

func indentJSON(v any) string {
	raw, _ := json.MarshalIndent(v, "", "  ")
	return string(raw)
}

// TestRecordAlertFidelity runs every scenario against a real tenant and writes the exchanges.
// The key is read from the environment and is never written anywhere. The tenant needs a key
// with the alert permissions of an organization.
//
//	RECORD_ALERT_FIDELITY=1 CORALOGIX_API_KEY=... CORALOGIX_ENV=EU2 \
//	  go test ./internal/provider -run TestRecordAlertFidelity
func TestRecordAlertFidelity(t *testing.T) {
	if os.Getenv("RECORD_ALERT_FIDELITY") != "1" {
		t.Skip("set RECORD_ALERT_FIDELITY=1 and an API key to record")
	}
	apiKey, region := os.Getenv("CORALOGIX_API_KEY"), os.Getenv("CORALOGIX_ENV")
	if apiKey == "" || region == "" {
		t.Fatal("CORALOGIX_API_KEY and CORALOGIX_ENV are required to record")
	}
	base, ok := cxsdkOpenapi.URLFromRegion(strings.ToLower(region))
	if !ok {
		t.Fatalf("unknown region %q", region)
	}
	names := alertRecordNames(os.Getenv("RECORD_ALERT_ONLY"))
	if len(names) == 0 {
		t.Fatalf("no scenario matches %q", os.Getenv("RECORD_ALERT_ONLY"))
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) { alertRecordScenario(t, name, base, apiKey) })
	}
}

// alertRecordNames returns the sorted names of the scenarios to record. only is a
// comma-separated list of scenario names. Without it, every scenario records.
func alertRecordNames(only string) []string {
	names := make([]string, 0, len(alertScenarios))
	for name := range alertScenarios {
		if only == "" || slices.Contains(strings.Split(only, ","), name) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

// alertRecordScenario runs one scenario against the real API and writes its exchanges.
func alertRecordScenario(t *testing.T, name, base, apiKey string) {
	live := newAlertLive(t, base, apiKey)
	// The cleanup deletes only what this run created, so recordings may run side by side.
	t.Cleanup(func() { alertCleanup(t, base, apiKey, live.created, live.slos) })
	h := newProtocolHarness(t, alertTypeName, live)
	defer func() {
		// Show what the real API refused, so a scenario that fails live is easy to fix.
		for _, step := range h.steps {
			if len(step.ApplyDiags) > 0 {
				t.Logf("step %q: %s", step.Step, strings.Join(step.ApplyDiags, " | "))
			}
		}
		for _, e := range live.exchangesSoFar() {
			if e.Status >= 400 {
				t.Logf("%s %s answered %d: %s", e.Method, e.Path, e.Status, compactJSON(e.Response))
			}
		}
	}()
	alertScenarios[name](h)
	norm := newAlertNormalizer()
	exchanges := []alertExchange{}
	for _, e := range live.takeExchanges() {
		exchanges = append(exchanges, norm.exchange(e))
	}
	raw, err := json.MarshalIndent(exchanges, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(alertGoldenDir, alertRecordedDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".json"), append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

// alertCleanup deletes the alerts that a run created and left behind. A 404 means the scenario
// deleted it already.
func alertCleanup(t *testing.T, base, apiKey string, created, slos []string) {
	t.Helper()
	// The API refuses to delete an alert that a flow refers to, so go from the newest alert to
	// the oldest, and go again over what is left.
	left := slices.Clone(created)
	slices.Reverse(left)
	for pass := 0; pass < 3 && len(left) > 0; pass++ {
		var next []string
		for _, id := range left {
			status, _ := alertDirect(base, apiKey, http.MethodDelete, alertBasePath+"/"+id, nil)
			if status != http.StatusOK && status != http.StatusNotFound {
				next = append(next, id)
			}
		}
		left = next
	}
	if len(left) > 0 {
		t.Errorf("alerts left behind: %s", strings.Join(left, ", "))
	}
	// The SLOs go after the alerts that refer to them. A 404 means the scenario deleted it already.
	var staleSLOs []string
	for _, id := range slos {
		status, _ := alertDirect(base, apiKey, http.MethodDelete, sloBasePath+"/"+id, nil)
		if status != http.StatusOK && status != http.StatusNotFound {
			staleSLOs = append(staleSLOs, id)
		}
	}
	if len(staleSLOs) > 0 {
		t.Errorf("SLOs left behind: %s", strings.Join(staleSLOs, ", "))
	}
}

func alertDirect(base, apiKey, method, path string, body any) (int, any) {
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

// alertJSONDiff lists the paths where two JSON values differ, with the fake value first.
func alertJSONDiff(path string, fake, api any) []string {
	switch a := fake.(type) {
	case map[string]any:
		b, ok := api.(map[string]any)
		if !ok {
			break
		}
		var out []string
		keys := map[string]bool{}
		for k := range a {
			keys[k] = true
		}
		for k := range b {
			keys[k] = true
		}
		sorted := make([]string, 0, len(keys))
		for k := range keys {
			sorted = append(sorted, k)
		}
		slices.Sort(sorted)
		for _, k := range sorted {
			av, aok := a[k]
			bv, bok := b[k]
			switch {
			case !aok:
				out = append(out, fmt.Sprintf("%s.%s: the fake has none, the API has %s", path, k, compactJSON(bv)))
			case !bok:
				out = append(out, fmt.Sprintf("%s.%s: the fake has %s, the API has none", path, k, compactJSON(av)))
			default:
				out = append(out, alertJSONDiff(path+"."+k, av, bv)...)
			}
		}
		return out
	case []any:
		b, ok := api.([]any)
		if !ok || len(a) != len(b) {
			break
		}
		var out []string
		for i := range a {
			out = append(out, alertJSONDiff(fmt.Sprintf("%s[%d]", path, i), a[i], b[i])...)
		}
		return out
	}
	if reflect.DeepEqual(fake, api) {
		return nil
	}
	return []string{fmt.Sprintf("%s: the fake has %s, the API has %s", path, compactJSON(fake), compactJSON(api))}
}

func compactJSON(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

func TestAlertUnrecordedBackendBehaviorDropsOnlyTheMissingPeriod(t *testing.T) {
	in := []string{
		".alertDef.alertDefProperties.analyticsImmediate.evaluationPeriodMinutes: the fake has none, the API has 10",
		".alertDef.alertDefProperties.analyticsThreshold.evaluationPeriodMinutes: the fake has none, the API has 5",
		// The fake sets a period that the API does not: not the known difference.
		".alertDef.alertDefProperties.analyticsImmediate.evaluationPeriodMinutes: the fake has 10, the API has none",
		// Another field of an analytics alert.
		".alertDef.alertDefProperties.analyticsImmediate.timeframeMinutes: the fake has none, the API has 10",
		// The same name outside an analytics alert.
		".alertDef.alertDefProperties.logsImmediate.evaluationPeriodMinutes: the fake has none, the API has 10",
	}
	want := in[2:]
	if got := alertUnrecordedBackendBehavior(in); !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}
