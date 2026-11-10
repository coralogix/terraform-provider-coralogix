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
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
)

const alertBasePath = "/alerts/alerts/v3"

// alertFake is a small in-memory backend for alert definitions. It is checked against recorded
// answers of the real API (alert_fidelity_test.go), so it follows what was recorded on a real
// tenant and nothing else:
//   - Create, Get, and Replace answer {"alertDef": {...}} with the server fields id,
//     alertVersionId, createdTime, updatedTime, and status. Delete answers {}.
//   - The REST gateway writes every field without presence, so an omitted list reads back as
//     [], an omitted map as {}, an omitted enum as its zero value, and an omitted bool as false.
//     The rules of alertDefaults say which messages the server fills in.
//   - A deleted, unknown, or malformed id answers 404 "Not Found: The requested Alert was not
//     found", on Get, Replace, and Delete.
//   - With include_access_permissions=true, Get also answers the access permissions of the key, and
//     the access policy of the alert when it has one. A Create takes the policy as a string, a Replace
//     as an action ({"upsert": text} or {"delete": {}}). A Replace without the action keeps the policy.
//     The API stores the policy with every permission it knows: the ones the request omits are "deny".
type alertFake struct {
	mu       sync.Mutex
	alerts   map[string]map[string]any
	policies map[string]string // alert id -> the stored access policy text
	sloStore *sloFake
	seq      int
	requests []grRequest
}

func init() {
	// The simple filter of a logs alert always has label filters.
	addAlertFill(func(props map[string]any) {
		if arm, ok := props["logsImmediate"].(map[string]any); ok {
			alertFillLogsFilter(arm)
		}
	})
}

func alertFillLogsFilter(arm map[string]any) {
	filter, _ := arm["logsFilter"].(map[string]any)
	if filter == nil {
		return
	}
	simple, _ := filter["simpleFilter"].(map[string]any)
	if simple == nil {
		return
	}
	setDefault(simple, "labelFilters", map[string]any{})
}

func newAlertFake(t *testing.T) *alertFake {
	t.Helper()
	f := &alertFake{alerts: map[string]map[string]any{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)

	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	original := http.DefaultTransport
	http.DefaultTransport = redirectTransport{target: target, next: original}
	t.Cleanup(func() { http.DefaultTransport = original })
	return f
}

func (f *alertFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

// seed stores an alert as if it was made outside Terraform. It returns the id.
func (f *alertFake) seed(props map[string]any) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	status, body := f.write(map[string]any{"alertDefProperties": props}, false)
	if status != http.StatusOK {
		panic(fmt.Sprintf("seed failed: %d %v", status, body))
	}
	return body.(map[string]any)["alertDef"].(map[string]any)["id"].(string)
}

// forget deletes an alert without a request, as if it was deleted outside Terraform.
func (f *alertFake) forget(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.alerts, id)
}

func (f *alertFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	raw, _ := io.ReadAll(r.Body)
	rec := grRequest{Method: r.Method, Path: r.URL.Path}
	if r.URL.RawQuery != "" {
		rec.Path += "?" + r.URL.RawQuery
	}
	var parsed any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &parsed)
	}
	status, body := f.handle(r.Method, r.URL.Path, r.URL.Query(), parsed)
	rec.Body = rawRequestBody(raw)
	rec.Status = status
	f.requests = append(f.requests, rec)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (f *alertFake) handle(method, path string, query url.Values, body any) (int, any) {
	if _, rest, ok := strings.Cut(path, sloBasePath); ok {
		return f.handleSLO(method, strings.TrimPrefix(rest, "/"), body)
	}
	_, rest, ok := strings.Cut(path, alertBasePath)
	if !ok {
		return grFail(http.StatusNotFound, "unknown path "+path)
	}
	id := strings.TrimPrefix(rest, "/")
	in, _ := body.(map[string]any)

	switch {
	case method == http.MethodPost && id == "":
		return f.write(in, false)
	case method == http.MethodPut && id == "":
		return f.write(in, true)
	case method == http.MethodGet && id != "":
		stored, found := f.alerts[id]
		if !found {
			return alertNotFound()
		}
		out := map[string]any{"alertDef": stored}
		if query.Get("include_access_permissions") == "true" {
			out["access"] = map[string]any{"permissions": map[string]any{
				"canRead": true, "canManage": true, "canReadAccessPolicy": true, "canUpdateAccessPolicy": true,
			}}
			if policy, has := f.policies[id]; has {
				out["access"].(map[string]any)["accessPolicy"] = policy
			}
		}
		return http.StatusOK, out
	case method == http.MethodDelete && id != "":
		if _, found := f.alerts[id]; !found {
			return alertNotFound()
		}
		delete(f.alerts, id)
		delete(f.policies, id)
		return http.StatusOK, map[string]any{}
	}
	return grFail(http.StatusMethodNotAllowed, "unsupported "+method+" "+path)
}

func alertNotFound() (int, any) {
	return http.StatusNotFound, map[string]any{"code": http.StatusNotFound, "message": "Not Found: The requested Alert was not found"}
}

func alertBadRequest(message string) (int, any) {
	return http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "Bad Request: " + message}
}

func (f *alertFake) uuidLike() string {
	f.seq++
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", f.seq)
}

// storePolicy applies the change that alertPolicyChange read from a request: nil leaves the policy,
// a pointer to "" deletes it, and any other text replaces it.
func (f *alertFake) storePolicy(id string, change *string) {
	if change == nil {
		return
	}
	if *change == "" {
		delete(f.policies, id)
		return
	}
	if f.policies == nil {
		f.policies = map[string]string{}
	}
	f.policies[id] = *change
}

func (f *alertFake) write(in map[string]any, update bool) (int, any) {
	props, _ := in["alertDefProperties"].(map[string]any)
	if props == nil {
		return alertBadRequest("alertDefProperties is required")
	}
	var id string
	if update {
		id, _ = in["id"].(string)
		if _, found := f.alerts[id]; !found {
			return alertNotFound()
		}
	}
	if status, body := alertStatusCheck(f, id, props); status != 0 {
		return status, body
	}
	if msg := alertCheck(props); msg != "" {
		return alertBadRequest(msg)
	}
	policy, msg := alertPolicyChange(in["accessPolicy"])
	if msg != "" {
		return alertBadRequest(msg)
	}
	if !update {
		id = f.uuidLike()
	}
	f.storePolicy(id, policy)
	stored := map[string]any{
		"alertDefProperties": alertDefaults(props),
		"id":                 id,
		"alertVersionId":     "",
		"createdTime":        "2026-01-01T00:00:00Z",
		"updatedTime":        "2026-01-01T00:00:00Z",
		"status":             "ALERT_DEF_STATUS_UNSPECIFIED",
	}
	for _, fill := range alertStoreFills {
		fill(f, stored["alertDefProperties"].(map[string]any))
	}
	if old, found := f.alerts[id]; found {
		stored["createdTime"], stored["alertVersionId"] = old["createdTime"], old["alertVersionId"]
		oldProps, _ := old["alertDefProperties"].(map[string]any)
		newProps, _ := stored["alertDefProperties"].(map[string]any)
		// Recorded and probed: a Replace without description keeps the stored one (only "" clears it),
		// unless the same Replace changes the definition of the alert (the type or the content of a type arm).
		for _, key := range alertKeptWhenOmitted {
			if _, sent := props[key]; !sent && !alertDefinitionChanged(oldProps, newProps) {
				if kept, had := oldProps[key]; had {
					newProps[key] = kept
				}
			}
		}
		for _, keep := range alertKeepRules {
			keep(oldProps, newProps)
		}
		if alertDefinitionChanged(oldProps, newProps) {
			stored["alertVersionId"] = f.uuidLike()
		}
	} else {
		stored["alertVersionId"] = f.uuidLike()
	}
	f.alerts[id] = stored
	return http.StatusOK, map[string]any{"alertDef": stored}
}

// alertChecks and alertFills are the rules that the recordings show. Each file of scenarios adds
// the rules of its alert types from init, next to the scenarios that need them.
//
//   - A check returns the message of a validation error, or "". The first message wins.
//   - A fill adds what the server writes into the stored properties although the request has
//     none, for example a message that the server creates. It runs before the zero values that
//     the gateway writes.
var (
	alertChecks []func(props map[string]any) string
	alertFills  []func(props map[string]any)
)

// alertStoreFills are fills that need the stored objects, for example what the API copies from an
// SLO that an alert refers to. They run on the stored properties, after the zero values.
var alertStoreFills []func(f *alertFake, props map[string]any)

func addAlertStoreFill(fill func(f *alertFake, props map[string]any)) {
	alertStoreFills = append(alertStoreFills, fill)
}

// alertDefinitionChanged is true when the type or the content of a type arm differs. Probed on a
// real tenant: a new name, priority, enabled flag, description, incident settings, or router keeps
// alertVersionId, and a new query changes it.
func alertDefinitionChanged(oldProps, newProps map[string]any) bool {
	if oldProps["type"] != newProps["type"] {
		return true
	}
	if schemas, err := alertSchemas(); err == nil {
		if schema, ok := schemas["AlertDefProperties"].(map[string]any); ok {
			for arm := range alertUnionArms(schema) {
				if !reflect.DeepEqual(oldProps[arm], newProps[arm]) {
					return true
				}
			}
		}
	}
	return false
}

// alertKeptWhenOmitted are the properties that the real API keeps on Replace when the request
// omits them. The recordings show it.
var alertKeptWhenOmitted = []string{"description"}

func addAlertCheck(check func(props map[string]any) string) { alertChecks = append(alertChecks, check) }
func addAlertFill(fill func(props map[string]any))          { alertFills = append(alertFills, fill) }

// alertCheck is the validation that the recordings show. It returns the message of the first
// failure.
func alertCheck(props map[string]any) string {
	if name, _ := props["name"].(string); name == "" {
		return "name is required"
	}
	for _, check := range alertChecks {
		if msg := check(props); msg != "" {
			return msg
		}
	}
	return ""
}

// alertDefaults returns what the server stores for a request: the request, the fields the server
// fills in, and the zero values that the REST gateway writes.
func alertDefaults(in map[string]any) map[string]any {
	out := cloneJSON(in).(map[string]any)
	setDefault(out, "enabled", true)
	setDefault(out, "priority", "ALERT_DEF_PRIORITY_P5_OR_UNSPECIFIED")
	setDefault(out, "type", "ALERT_DEF_TYPE_LOGS_IMMEDIATE_OR_UNSPECIFIED")
	setDefault(out, "incidentsSettings", map[string]any{"minutes": float64(10), "notifyOn": "NOTIFY_ON_TRIGGERED_ONLY_UNSPECIFIED"})
	setDefault(out, "notificationGroup", map[string]any{})
	setDefault(out, "deleted", false)
	for _, fill := range alertFills {
		fill(out)
	}
	if schemas, err := alertSchemas(); err == nil {
		if schema, ok := schemas["AlertDefProperties"].(map[string]any); ok {
			out = alertZeroFill(schemas, schema, out).(map[string]any)
		}
	}
	return out
}

func setDefault(m map[string]any, key string, value any) {
	if _, ok := m[key]; !ok {
		m[key] = value
	}
}

func cloneJSON(v any) any {
	raw, _ := json.Marshal(v)
	var out any
	_ = json.Unmarshal(raw, &out)
	return out
}

// alertLive forwards every request of the provider to a real tenant and records the exchanges.
// It replaces the placeholder key of the harness with the real key, which only lives in the
// environment of the recording run.
type alertLive struct {
	mu        sync.Mutex
	base      *url.URL
	next      http.RoundTripper
	apiKey    string
	requests  []grRequest
	exchanges []alertExchange
	// created holds the id of every alert that a request of this run created, for the cleanup.
	created []string
	// slos holds the id of every SLO that a request of this run created, for the cleanup, and
	// label names the scenario in the names of those SLOs.
	slos  []string
	label string
}

// alertExchange is one request and the answer of the real API.
type alertExchange struct {
	Method   string `json:"method"`
	Path     string `json:"path"`
	Request  any    `json:"request,omitempty"`
	Status   int    `json:"status"`
	Response any    `json:"response,omitempty"`
}

func newAlertLive(t *testing.T, base, apiKey string) *alertLive {
	t.Helper()
	target, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	original := http.DefaultTransport
	l := &alertLive{base: target, next: original, apiKey: apiKey, label: alertSLOLabel(t.Name())}
	http.DefaultTransport = l
	t.Cleanup(func() { http.DefaultTransport = original })
	return l
}

func (l *alertLive) takeRequests() []grRequest {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := l.requests
	l.requests = nil
	return out
}

func (l *alertLive) exchangesSoFar() []alertExchange {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]alertExchange(nil), l.exchanges...)
}

func (l *alertLive) takeExchanges() []alertExchange {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := l.exchanges
	l.exchanges = nil
	return out
}

func (l *alertLive) RoundTrip(req *http.Request) (*http.Response, error) {
	var raw []byte
	if req.Body != nil {
		raw, _ = io.ReadAll(req.Body)
	}
	out := req.Clone(req.Context())
	out.URL.Scheme, out.URL.Host, out.Host = l.base.Scheme, l.base.Host, l.base.Host
	out.Header.Set("Authorization", "Bearer "+l.apiKey)
	out.Body = io.NopCloser(bytes.NewReader(raw))
	out.ContentLength = int64(len(raw))
	resp, err := l.next.RoundTrip(out)
	if err != nil {
		return nil, err
	}
	answer, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(answer))

	path := req.URL.Path
	if req.URL.RawQuery != "" {
		path += "?" + req.URL.RawQuery
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.requests = append(l.requests, grRequest{Method: req.Method, Path: path, Body: rawRequestBody(raw), Status: resp.StatusCode})
	ex := alertExchange{Method: req.Method, Path: path, Status: resp.StatusCode}
	if len(bytes.TrimSpace(raw)) > 0 {
		ex.Request = rawRequestBody(raw)
	}
	if len(bytes.TrimSpace(answer)) > 0 {
		var parsed any
		if json.Unmarshal(answer, &parsed) == nil {
			ex.Response = parsed
		} else {
			ex.Response = map[string]any{"raw": string(answer)}
		}
	}
	l.exchanges = append(l.exchanges, ex)
	if req.Method == http.MethodPost && resp.StatusCode == http.StatusOK {
		var made struct {
			AlertDef struct {
				ID string `json:"id"`
			} `json:"alertDef"`
		}
		if json.Unmarshal(answer, &made) == nil && made.AlertDef.ID != "" {
			l.created = append(l.created, made.AlertDef.ID)
		}
	}
	if req.Method == http.MethodPost && resp.StatusCode == http.StatusOK && strings.HasSuffix(req.URL.Path, sloBasePath) {
		var made struct {
			SLO struct {
				ID string `json:"id"`
			} `json:"slo"`
		}
		if json.Unmarshal(answer, &made) == nil && made.SLO.ID != "" {
			l.slos = append(l.slos, made.SLO.ID)
		}
	}
	return resp, nil
}

// seed creates an alert directly, as if it was made outside Terraform.
func (l *alertLive) seed(props map[string]any) string {
	status, answer := alertDirectVia(l, http.MethodPost, alertBasePath, map[string]any{"alertDefProperties": props})
	id, _ := asObject(asObject(answer)["alertDef"])["id"].(string)
	if status != http.StatusOK || id == "" {
		panic(fmt.Sprintf("seed failed: %d %v", status, answer))
	}
	return id
}

// forget deletes an alert directly, as if it was deleted outside Terraform.
func (l *alertLive) forget(id string) {
	if status, answer := alertDirectVia(l, http.MethodDelete, alertBasePath+"/"+id, nil); status != http.StatusOK {
		panic(fmt.Sprintf("forget failed: %d %v", status, answer))
	}
}

func alertDirectVia(l *alertLive, method, path string, body any) (int, any) {
	return alertDirect(l.base.String(), l.apiKey, method, path, body)
}

// alertKeepRules keep what the real API keeps on Replace when the request does not send it. Each
// file of scenarios adds the rules of its alert types from init.
var alertKeepRules []func(oldProps, newProps map[string]any)

func addAlertKeepRule(keep func(oldProps, newProps map[string]any)) {
	alertKeepRules = append(alertKeepRules, keep)
}

// alertStatusChecks are the rules of the answers that are no 400 with a plain message. A check
// returns a status other than 0 and the body of the answer.
var alertStatusChecks []func(props map[string]any) (int, any)

// alertStoreChecks are status checks that need the stored alerts, for example to look up a reference.
var alertStoreChecks []func(f *alertFake, id string, props map[string]any) (int, any)

func addAlertStatusCheck(check func(props map[string]any) (int, any)) {
	alertStatusChecks = append(alertStatusChecks, check)
}

func addAlertStoreCheck(check func(f *alertFake, id string, props map[string]any) (int, any)) {
	alertStoreChecks = append(alertStoreChecks, check)
}

// alertStatusCheck is called by the fake before the plain checks.
func alertStatusCheck(f *alertFake, id string, props map[string]any) (int, any) {
	for _, check := range alertStatusChecks {
		if status, body := check(props); status != 0 {
			return status, body
		}
	}
	for _, check := range alertStoreChecks {
		if status, body := check(f, id, props); status != 0 {
			return status, body
		}
	}
	return 0, nil
}
