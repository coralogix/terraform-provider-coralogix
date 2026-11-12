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

// The fake alerts API (with the SLO API that an SLO alert reads) for the alert state upgrade and data source tests.
// It is not a baseline: the tests that use it assert in code, and no golden file is recorded.

package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"
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

// alertKeepRules keep what the real API keeps on Replace when the request does not send it. Each
// file of scenarios adds the rules of its alert types from init.
var alertKeepRules []func(oldProps, newProps map[string]any)

// alertStatusChecks are the rules of the answers that are no 400 with a plain message. A check
// returns a status other than 0 and the body of the answer.
var alertStatusChecks []func(props map[string]any) (int, any)

// alertStoreChecks are status checks that need the stored alerts, for example to look up a reference.
var alertStoreChecks []func(f *alertFake, id string, props map[string]any) (int, any)

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

func msMap(m map[string]any, key string) map[string]any {
	out, _ := m[key].(map[string]any)
	return out
}

// alertPolicyPermissions are the permissions that the API stores in every access policy of an alert.
// The ones that a request leaves out are stored as "deny" (recorded on EU2).
var alertPolicyPermissions = []string{
	"alerts:LogsReadConfig", "alerts:LogsUpdateConfig", "alerts:MetricsReadConfig", "alerts:MetricsUpdateConfig",
	"alerts:ReadAccessPolicy", "alerts:ReadConfig", "alerts:SloMgmtReadConfig", "alerts:SloMgmtUpdateConfig",
	"alerts:SnowbitCreateSecurityAlert", "alerts:SnowbitRead", "alerts:SpansReadConfig", "alerts:SpansUpdateConfig",
	"alerts:UpdateAccessPolicy", "alerts:UpdateConfig",
}

// alertPolicyChange reads the access policy of a Create (a string) or of a Replace (an action).
// It returns nil when the request leaves the policy as it is, a pointer to "" for a delete, and a
// pointer to the text that the API stores for an upsert. msg is the message of a 400.
func alertPolicyChange(raw any) (change *string, msg string) {
	var text string
	switch v := raw.(type) {
	case nil:
		return nil, ""
	case string:
		text = v
	case map[string]any:
		if _, del := v["delete"]; del {
			empty := ""
			return &empty, ""
		}
		upsert, ok := v["upsert"].(string)
		if !ok {
			return nil, ""
		}
		text = upsert
	default:
		return nil, ""
	}
	var policy map[string]any
	if err := json.Unmarshal([]byte(text), &policy); err != nil {

		return nil, "Access policy rejected: Invalid argument: Provided access_policy_raw_json is not a valid JSON: expected ident at line 1 column 2"
	}
	def, _ := policy["default"].(map[string]any)
	if def == nil {
		def = map[string]any{}
		policy["default"] = def
	}
	perms, _ := def["permissions"].(map[string]any)
	if perms == nil {
		perms = map[string]any{}
		def["permissions"] = perms
	}
	for _, name := range alertPolicyPermissions {
		if _, set := perms[name]; !set {
			perms[name] = "deny"
		}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(policy)
	stored := strings.TrimSuffix(buf.String(), "\n")
	return &stored, ""
}

// sloFake returns the SLO service of the fake: the SLO baseline already checked it against the
// real service. Its ids start high, so that they never equal the ids of the alerts.
func (f *alertFake) sloFake() *sloFake {
	if f.sloStore == nil {
		f.sloStore = &sloFake{slos: map[string]map[string]any{}, seq: 5000}
	}
	return f.sloStore
}

// handleSLO answers a request to the SLO service. rest is the path after the base path.
func (f *alertFake) handleSLO(method, rest string, body any) (int, any) {
	path := sloBasePath
	if rest != "" {
		path += "/" + rest
	}
	status, answer := f.sloFake().handle(method, path, body)
	if method == http.MethodPost && status == http.StatusOK {

		slo := cloneMap(answer.(map[string]any)["slo"].(map[string]any))
		setDefault(slo, "description", "")
		answer.(map[string]any)["slo"] = slo
	}
	if method != http.MethodDelete || status != http.StatusOK {
		return status, answer
	}

	gone := []string{}
	for id, stored := range f.alerts {
		props, _ := stored["alertDefProperties"].(map[string]any)
		if sloIDOfAlert(props) == rest {
			gone = append(gone, id)
			delete(f.alerts, id)
		}
	}
	slices.Sort(gone)
	ids := make([]any, len(gone))
	for i, id := range gone {
		ids[i] = id
	}
	return http.StatusOK, map[string]any{"effectedSloAlertIds": ids}
}

func sloIDOfAlert(props map[string]any) string {
	id, _ := msMap(msMap(props, "sloThreshold"), "sloDefinition")["sloId"].(string)
	return id
}

// The REST gateway of the alerts API writes every field that has no presence: a missing list is
// [], a missing map is {}, a missing enum is its zero value, a missing scalar is its zero value.
// The fake reads the shapes from the OpenAPI document of the pinned SDK, so it follows the
// contract that the provider is built against.
var alertSpec struct {
	once    sync.Once
	schemas map[string]any
	err     error
}

func alertSchemas() (map[string]any, error) {
	alertSpec.once.Do(func() {
		out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/coralogix/coralogix-management-sdk").Output()
		if err != nil {
			alertSpec.err = err
			return
		}
		raw, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(out)), "openapi.yaml"))
		if err != nil {
			alertSpec.err = err
			return
		}
		var doc map[string]any
		if alertSpec.err = yaml.Unmarshal(raw, &doc); alertSpec.err != nil {
			return
		}
		components, _ := doc["components"].(map[string]any)
		alertSpec.schemas, _ = components["schemas"].(map[string]any)
	})
	return alertSpec.schemas, alertSpec.err
}

// alertResolve follows $ref and a single allOf to the schema that holds the properties. The
// description and the presence marker of the outer schema win, as the generator reads them.
func alertResolve(schemas map[string]any, schema map[string]any) map[string]any {
	presence := schema["x-coralogix-presence"]
	for i := 0; i < 8; i++ {
		if ref, ok := schema["$ref"].(string); ok {
			next, _ := schemas[strings.TrimPrefix(ref, "#/components/schemas/")].(map[string]any)
			schema = next
			continue
		}
		if all, ok := schema["allOf"].([]any); ok && len(all) == 1 {
			next, _ := all[0].(map[string]any)
			schema = next
			continue
		}
		break
	}
	if presence == true {
		merged := map[string]any{}
		for k, v := range schema {
			merged[k] = v
		}
		merged["x-coralogix-presence"] = true
		return merged
	}
	return schema
}

// alertNoZeroFields are the fields that the gateway does not write when they are missing, in spite
// of having no presence marker in the contract.
var alertNoZeroFields = map[string]bool{"luceneQuery": true}

// alertZeroFill returns value with every field that has no presence written, recursively. A
// field that holds a message and is missing stays missing: the server decides which messages it
// fills in (see alertDefaults).
func alertZeroFill(schemas map[string]any, schema map[string]any, value any) any {
	schema = alertResolve(schemas, schema)
	switch x := value.(type) {
	case map[string]any:
		if props, ok := schema["properties"].(map[string]any); ok {
			for name, raw := range props {
				sub, _ := raw.(map[string]any)
				if sub == nil {
					continue
				}
				field := alertResolve(schemas, sub)
				if existing, present := x[name]; present {
					x[name] = alertZeroFill(schemas, sub, existing)
					continue
				}

				if alertUnionArms(schema)[name] {
					continue
				}

				if alertNoZeroFields[name] {
					continue
				}
				if zero, ok := alertZero(field); ok {
					x[name] = zero
				}
			}
		} else if extra, ok := schema["additionalProperties"].(map[string]any); ok {
			for k, item := range x {
				x[k] = alertZeroFill(schemas, extra, item)
			}
		}
		return x
	case []any:
		items, _ := schema["items"].(map[string]any)
		if items == nil {
			return x
		}
		for i, item := range x {
			x[i] = alertZeroFill(schemas, items, item)
		}
		return x
	}
	return value
}

// alertUnionArms returns the names of the properties that are the arms of the oneOf of schema.
// The other properties of the same object are ordinary fields.
func alertUnionArms(schema map[string]any) map[string]bool {
	arms := map[string]bool{}
	union, _ := schema["oneOf"].([]any)
	for _, raw := range union {
		arm, _ := raw.(map[string]any)
		required, _ := arm["required"].([]any)
		for _, name := range required {
			if n, ok := name.(string); ok {
				arms[n] = true
			}
		}
	}
	return arms
}

// alertZero is the value that the gateway writes for a missing field, and false when it writes
// none: a field with presence and a message.
func alertZero(field map[string]any) (any, bool) {
	if field["x-coralogix-presence"] == true {
		return nil, false
	}
	switch field["type"] {
	case "array":
		return []any{}, true
	case "object":
		if _, isMap := field["additionalProperties"]; isMap {
			return map[string]any{}, true
		}
		return nil, false
	case "string":
		if values, ok := field["enum"].([]any); ok && len(values) > 0 {
			return values[0], true
		}
		return "", true
	case "boolean":
		return false, true
	case "integer", "number":
		return float64(0), true
	}
	return nil, false
}

const (
	alertTypeName  = "coralogix_alert"
	alertGoldenDir = "testdata/alert_baseline"
)

func newAlertHarness(t *testing.T) *grHarness {
	return newProtocolHarness(t, alertTypeName, newAlertFake(t))
}

// alertLogsImmediate is the smallest alert: a name and one type.
func alertLogsImmediate(name string, extra ...j) j {
	out := j{
		"name": name,
		"type_definition": j{"logs_immediate": j{
			"logs_filter": j{"simple_filter": j{"lucene_query": "tfbaseline"}},
		}},
	}
	for _, e := range extra {
		out = with(out, e)
	}
	return out
}

// rawRequestBody decodes the request body again, so the recording shows what the provider sent,
// not what the fake changed while it handled the request.
func rawRequestBody(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	var v any
	_ = json.Unmarshal(raw, &v)
	return v
}

// grError is the error body of the real API: the message holds a JSON "reason".
func grError(status int, reason string) map[string]any {
	prefix := map[int]string{http.StatusBadRequest: "Bad Request", http.StatusNotFound: "Not Found"}[status]
	if prefix == "" {
		prefix = http.StatusText(status)
	}
	raw, _ := json.Marshal(map[string]string{"reason": reason})
	return map[string]any{"code": status, "message": prefix + ": " + string(raw)}
}

func grFail(status int, reason string) (int, any) { return status, grError(status, reason) }

func with(base j, overrides j) j {
	out := maps.Clone(base)
	maps.Copy(out, overrides)
	return out
}

const sloBasePath = "/slo/slos/v1"

// sloFake follows the SLO service as recorded in fidelity/recorded.json, not the Go client.
// Create and replace take the SLO itself as the body. Create returns it as "slo" next to
// exclusionWindowIds, replace next to exclusionWindowIds and effectedSloAlertIds, and delete
// returns effectedSloAlertIds. The server sets id, grouping (the by clause of the query, or
// service_name for an APM SLI), sloType, type, creator, revision (which moves when the SLI
// changes), and the times. labels is {} when the request has none. An omitted
// targetThresholdPercentage or threshold reads as 0, an omitted missingDataStrategy as
// UNCOUNTED, also on a replace, and an omitted productType as UNSPECIFIED. An APM SLI needs
// productType APM, and gets an apmSliMetadata copy. Every APM SLI returns filters and
// groupingKeys, empty when omitted. A written ownership dimension returns staticValues and
// labelKeys as sent, empty when omitted, and an empty resolvedValues; a dimension with no values
// is dropped.
type sloFake struct {
	slos         map[string]map[string]any
	seq          int
	clock        int
	deleteStatus int
}

func (f *sloFake) handle(method, path string, body any) (int, any) {
	_, rest, ok := strings.Cut(path, sloBasePath)
	if !ok {
		return sloFail(http.StatusNotFound, "unknown path "+path)
	}
	in, _ := body.(map[string]any)
	switch {
	case method == http.MethodPost && rest == "":
		return f.write(in, false)
	case method == http.MethodPut && rest == "":
		return f.write(in, true)
	case method == http.MethodGet && strings.HasPrefix(rest, "/"):
		id := strings.TrimPrefix(rest, "/")
		stored, found := f.slos[id]
		if !found {
			return sloFail(http.StatusNotFound, "Slo with id "+id+" not found for company 1")
		}
		return http.StatusOK, map[string]any{"slo": stored}
	case method == http.MethodDelete && strings.HasPrefix(rest, "/"):
		id := strings.TrimPrefix(rest, "/")
		if f.deleteStatus != 0 {
			return sloFail(f.deleteStatus, "Slo with id "+id+" not found for company 1")
		}
		if _, found := f.slos[id]; !found {
			return sloFail(http.StatusNotFound, "Slo with id "+id+" not found for company 1")
		}
		delete(f.slos, id)
		return http.StatusOK, map[string]any{"effectedSloAlertIds": []any{}}
	}
	return sloFail(http.StatusMethodNotAllowed, "unsupported "+method+" "+path)
}

// sloFail is an error answer of the SLO service: the status text, a colon, and the message.
func sloFail(status int, message string) (int, any) {
	return status, map[string]any{"code": status, "message": http.StatusText(status) + ": " + message}
}

// sloValidation is the answer to a request that the validation layer refuses.
func sloValidation(replace bool, reason string) (int, any) {
	method := "CreateSlo"
	if replace {
		method = "ReplaceSlo"
	}
	return sloFail(http.StatusBadRequest, "validation failed for method /com.coralogixapis.slo.v1.SlosService/"+method+": ValidationError: "+reason)
}

var sloArms = []string{"apmSli", "requestBasedMetricSli", "windowBasedMetricSli"}

func (f *sloFake) write(in map[string]any, replace bool) (int, any) {
	if in == nil {
		return sloFail(http.StatusBadRequest, "slo is required")
	}
	if status, resp := f.check(in, replace); status != 0 {
		return status, resp
	}
	stored := map[string]any{
		"name":                      in["name"],
		"targetThresholdPercentage": 0,
		"sloTimeFrame":              in["sloTimeFrame"],
		"productType":               "SLO_PRODUCT_TYPE_UNSPECIFIED",
		"labels":                    map[string]any{},
	}
	if v, ok := in["targetThresholdPercentage"]; ok {
		stored["targetThresholdPercentage"] = v
	}
	if desc, _ := in["description"].(string); desc != "" {
		stored["description"] = desc
	}
	if labels, _ := in["labels"].(map[string]any); len(labels) > 0 {
		stored["labels"] = labels
	}
	if pt, _ := in["productType"].(string); pt != "" {
		stored["productType"] = pt
	}
	if tags := sloOwnershipTags(in["ownershipTags"]); tags != nil {
		stored["ownershipTags"] = tags
	}
	arm := ""
	switch {
	case in["requestBasedMetricSli"] != nil:
		arm = "requestBasedMetricSli"
		sli := in[arm].(map[string]any)
		stored[arm] = sli
		stored["grouping"] = map[string]any{"labels": sloGroupBy(sli["totalEvents"])}
	case in["windowBasedMetricSli"] != nil:
		arm = "windowBasedMetricSli"
		sli := cloneMap(in[arm].(map[string]any))
		if sli["missingDataStrategy"] == nil {
			sli["missingDataStrategy"] = "MISSING_DATA_STRATEGY_UNCOUNTED"
		}
		if sli["threshold"] == nil {
			sli["threshold"] = 0
		}
		stored[arm] = sli
		stored["grouping"] = map[string]any{"labels": sloGroupBy(sli["query"])}
	default:
		arm = "apmSli"
		sli := sloAPMDefaults(in[arm].(map[string]any))
		stored[arm] = sli
		stored["apmSliMetadata"] = cloneMap(sli)
		stored["grouping"] = map[string]any{"labels": []any{"service_name"}}
	}
	stored["sloType"], stored["type"] = sloTypeOf(in), "request"
	if stored["sloType"] == "SLO_TYPE_WINDOW" {
		stored["type"] = "window"
	}

	f.clock++
	now := fmt.Sprintf("2026-01-01T00:00:%02dZ", f.clock)
	id, _ := in["id"].(string)
	var prevDesc any
	if replace {
		prev := f.slos[id]
		prevDesc = prev["description"]
		stored["createTime"], stored["creator"] = prev["createTime"], prev["creator"]
		prevRevision, _ := prev["revision"].(map[string]any)
		revision, _ := prevRevision["revision"].(int)
		if !reflect.DeepEqual(prev[arm], stored[arm]) {
			revision++
		}
		stored["revision"] = map[string]any{"revision": revision, "updateTime": now}
	} else {
		f.seq++
		id = fmt.Sprintf("00000000-0000-4000-8000-%012d", f.seq)
		stored["createTime"], stored["creator"] = now, "terraform@example.com"
		stored["revision"] = map[string]any{"revision": 1, "updateTime": now}
	}
	stored["id"] = id
	stored["updateTime"] = now
	f.slos[id] = stored
	if _, sent := in["description"]; replace && !sent {

		if prevDesc != nil {
			kept := cloneMap(stored)
			kept["description"] = prevDesc
			f.slos[id] = kept
		}
	}
	if replace {
		return http.StatusOK, map[string]any{"slo": stored, "effectedSloAlertIds": []any{}, "exclusionWindowIds": []any{}}
	}
	return http.StatusOK, map[string]any{"slo": stored, "exclusionWindowIds": []any{}}
}

// sloTypeOf is the SLO type that the server derives from the SLI. A window-based metric SLI
// reads SLO_TYPE_REQUEST; an APM latency SLI reads SLO_TYPE_WINDOW.
func sloTypeOf(in map[string]any) string {
	if apm, ok := in["apmSli"].(map[string]any); ok && apm["latencyConfig"] != nil {
		return "SLO_TYPE_WINDOW"
	}
	return "SLO_TYPE_REQUEST"
}

func (f *sloFake) check(in map[string]any, replace bool) (int, any) {
	var set []string
	for _, arm := range sloArms {
		if in[arm] != nil {
			set = append(set, arm)
		}
	}
	if len(set) > 1 {

		return sloFail(http.StatusBadRequest, "proto: (line 1:1): error parsing \""+set[1]+"\", oneof com.coralogixapis.slo.v1.Slo.sli is already set")
	}
	if in["grouping"] != nil {
		return sloValidation(replace, `"slo.grouping" is not allowed`)
	}
	if name, _ := in["name"].(string); name == "" {
		return sloValidation(replace, `"slo.name" is not allowed to be empty`)
	}
	if in["sloTimeFrame"] == nil {
		return sloValidation(replace, `"slo.window" is required`)
	}
	if got, _ := in["sloType"].(string); got != "" && got != sloTypeOf(in) {
		return sloValidation(replace, "sloType must match the SLI type. Expected "+sloTypeOf(in)+", got "+got)
	}
	if in["apmSliMetadata"] != nil {
		return sloValidation(replace, `"slo.apmSliMetadata" is not allowed`)
	}
	if len(set) == 0 {
		return sloValidation(replace, `"slo.sli" is required`)
	}
	if status, resp := sloCheckQueries(in); status != 0 {
		return status, resp
	}
	if window, ok := in["windowBasedMetricSli"].(map[string]any); ok {
		if op, _ := window["comparisonOperator"].(string); op == "" || op == "COMPARISON_OPERATOR_UNSPECIFIED" {
			return sloFail(http.StatusBadRequest, "Invalid comparison operator: 0")
		}
		if w, _ := window["window"].(string); w == "" || w == "WINDOW_SLO_WINDOW_UNSPECIFIED" {
			return sloFail(http.StatusInternalServerError, "failed to create SLO: Not implemented yet: WindowSloWindow.WINDOW_SLO_WINDOW_UNSPECIFIED case")
		}
	}
	if apm, ok := in["apmSli"].(map[string]any); ok {
		if status, resp := sloCheckAPM(in, apm); status != 0 {
			return status, resp
		}
	}
	if replace {
		id, _ := in["id"].(string)
		if _, found := f.slos[id]; !found {
			return sloFail(http.StatusNotFound, "Cannot replace Slo with id "+id+" because it does not exist")
		}
	}
	return 0, nil
}

// sloCheckQueries refuses a metric SLI without a query.
func sloCheckQueries(in map[string]any) (int, any) {
	missing := func(metric any) bool {
		m, _ := metric.(map[string]any)
		q, _ := m["query"].(string)
		return q == ""
	}
	if sli, ok := in["requestBasedMetricSli"].(map[string]any); ok {
		if missing(sli["goodEvents"]) {
			return sloFail(http.StatusBadRequest, "Good events query: Query is missing")
		}
		if missing(sli["totalEvents"]) {
			return sloFail(http.StatusBadRequest, "Total events query: Query is missing")
		}
	}
	if sli, ok := in["windowBasedMetricSli"].(map[string]any); ok && missing(sli["query"]) {
		return sloFail(http.StatusBadRequest, "Window query: Query is missing")
	}
	return 0, nil
}

// sloCheckAPM refuses an APM SLI that the released resource documents as invalid. The messages
// are not recorded: the recording tenant has no APM service.
func sloCheckAPM(in, apm map[string]any) (int, any) {
	if in["productType"] != "SLO_PRODUCT_TYPE_APM" {
		return sloFail(http.StatusBadRequest, "an APM SLI needs productType SLO_PRODUCT_TYPE_APM")
	}
	if services, _ := apm["services"].([]any); len(services) != 1 {
		return sloFail(http.StatusBadRequest, "an APM SLI needs exactly one service")
	}
	if (apm["errorConfig"] == nil) == (apm["latencyConfig"] == nil) {
		return sloFail(http.StatusBadRequest, "an APM SLI needs errorConfig or latencyConfig")
	}
	if latency, ok := apm["latencyConfig"].(map[string]any); ok {
		if latency["timeWindow"] == nil {
			return sloFail(http.StatusBadRequest, "latencyConfig.timeWindow is required")
		}
		if (latency["quantile"] == nil) == (latency["average"] == nil) {
			return sloFail(http.StatusBadRequest, "latencyConfig needs quantile or average")
		}
	}
	return 0, nil
}

// sloAPMDefaults stores the defaults of an APM SLI and the empty arrays that the API returns.
func sloAPMDefaults(in map[string]any) map[string]any {
	sli := cloneMap(in)
	if sli["filters"] == nil {
		sli["filters"] = []any{}
	}

	if filters, ok := sli["filters"].([]any); ok {
		withOperator := make([]any, len(filters))
		for i, f := range filters {
			filter, _ := f.(map[string]any)
			filter = cloneMap(filter)
			if filter["matchOperator"] == nil {
				filter["matchOperator"] = "APM_FILTER_MATCH_OPERATOR_UNSPECIFIED"
			}
			withOperator[i] = filter
		}
		sli["filters"] = withOperator
	}
	if sli["groupingKeys"] == nil {
		sli["groupingKeys"] = []any{}
	}
	if latency, ok := sli["latencyConfig"].(map[string]any); ok {
		latency = cloneMap(latency)
		if latency["threshold"] == nil {
			latency["threshold"] = 0
		}
		if quantile, ok := latency["quantile"].(map[string]any); ok {
			quantile = cloneMap(quantile)
			if quantile["percentile"] == nil {
				quantile["percentile"] = 0
			}
			latency["quantile"] = quantile
		}
		sli["latencyConfig"] = latency
	}
	return sli
}

// sloOwnershipTags drops a dimension with no values, and fills the arrays of the others. The
// static values stay as sent. resolvedValues is empty right after a write: the server resolves it
// later.
func sloOwnershipTags(in any) map[string]any {
	tags, _ := in.(map[string]any)
	out := map[string]any{}
	for _, dim := range []string{"service", "environment", "team"} {
		tag, _ := tags[dim].(map[string]any)
		static, _ := tag["staticValues"].([]any)
		keys, _ := tag["labelKeys"].([]any)
		if len(static) == 0 && len(keys) == 0 {
			continue
		}
		out[dim] = map[string]any{
			"staticValues":   append([]any{}, static...),
			"labelKeys":      append([]any{}, keys...),
			"resolvedValues": []any{},
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

var sloByClause = regexp.MustCompile(`by\s*\(([^)]*)\)`)

// sloGroupBy returns the labels of the by (...) clause of a metric query.
func sloGroupBy(metric any) []any {
	m, _ := metric.(map[string]any)
	query, _ := m["query"].(string)
	match := sloByClause.FindStringSubmatch(query)
	out := []any{}
	if match == nil {
		return out
	}
	for _, label := range strings.Split(match[1], ",") {
		if label = strings.TrimSpace(label); label != "" {
			out = append(out, label)
		}
	}
	return out
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
