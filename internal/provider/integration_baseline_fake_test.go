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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
)

const integrationTypeName = "coralogix_integration"

const integrationGoldenDir = "testdata/integration_baseline"

const integrationBasePath = "/integrations"

// integrationRoleFailsTest is the role that the real Test call rejects. The recording shows the
// answer for this role. Any other role passes the test.
const integrationRoleFailsTest = "arn:aws:iam::123456789012:role/example-role"

// integrationDefinitions lists the definitions that the recording covers: the version, the
// required parameters, and every parameter that the definition knows.
var integrationDefinitions = map[string]struct {
	version  string
	required []string
	known    []string
}{
	"aws-resource-catalog": {
		version:  "0.1.0",
		required: []string{"AwsRoleArn", "IntegrationName"},
		known:    []string{"AwsRoleArn", "IntegrationName"},
	},
	"aws-metrics-collector": {
		version:  "0.1.0",
		required: []string{"AwsRoleArn", "IntegrationName"},
		known:    []string{"ApplicationName", "AwsRegion", "AwsRoleArn", "EnrichWithTags", "IntegrationName", "MetricNamespaces", "SubsystemName", "WithAggregations"},
	},
}

func newIntegrationHarness(t *testing.T) *grHarness {
	return newProtocolHarness(t, integrationTypeName, newIntegrationFake(t))
}

func (h *grHarness) integrations() *integrationFake {
	h.t.Helper()
	f, ok := h.api.(*integrationFake)
	if !ok {
		h.t.Fatal("expected *integrationFake")
	}
	return f
}

// integrationFake follows the integrations service as recorded in fidelity/recorded.json, not the
// Go client. A Test call checks the same things as a Save and then tries the role: the answer is a
// success or a failure result, with status 200. A Save answers the id that the server makes. A
// read answers the definition key and version and every parameter in no fixed order. An update
// merges the parameters that it sends into the stored ones and keeps the others, answers an empty
// object, and answers 404 for an id that does not exist. A delete answers an empty object, and
// 404 for an id that does not exist.
//
// Not recorded, because the tenant never answered them: notFound makes every call answer 404, and
// status makes a Save, an Update, or a Delete answer that status.
type integrationFake struct {
	mu       sync.Mutex
	items    map[string]map[string]any
	seq      int
	requests []grRequest
	// failStatus is the status of a Save, Update, or Delete that fails. 0 means that it works.
	failStatus int
	// testStatus is the status of a Test that fails. 0 means that it works.
	testStatus int
	// notFound makes a read, update, or delete answer 404 for every id.
	notFound bool
	// readStatus is the status of a read that fails. 0 means that it works.
	readStatus int
}

func newIntegrationFakeState() *integrationFake {
	return &integrationFake{items: map[string]map[string]any{}}
}

func newIntegrationFake(t *testing.T) *integrationFake {
	t.Helper()
	f := newIntegrationFakeState()
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

// seed stores an integration that exists outside Terraform, and returns its id.
func (f *integrationFake) seed(key, version string, parameters ...map[string]any) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	id := fmt.Sprintf("00000000-0000-4000-8000-%012d", f.seq)
	list := []any{}
	for _, p := range parameters {
		list = append(list, p)
	}
	f.items[id] = map[string]any{"definitionKey": key, "definitionVersion": version, "parameters": list}
	return id
}

// modifyOutside sets the string value of a parameter, as if somebody changed it outside Terraform.
func (f *integrationFake) modifyOutside(id, key, value string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	params := asList(f.items[id]["parameters"])
	for i, p := range params {
		if p.(map[string]any)["key"] == key {
			params[i] = map[string]any{"key": key, "stringValue": value}
		}
	}
}

// sortRecordedParameters sorts every list of parameters in a request body by key.
func sortRecordedParameters(v any) {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			if list, ok := e.([]any); ok && k == "parameters" {
				sort.Slice(list, func(i, j int) bool {
					a, _ := list[i].(map[string]any)["key"].(string)
					b, _ := list[j].(map[string]any)["key"].(string)
					return a < b
				})
			}
			sortRecordedParameters(e)
		}
	case []any:
		for _, e := range x {
			sortRecordedParameters(e)
		}
	}
}

// deleteOutside removes an integration, as if somebody deleted it outside Terraform.
func (f *integrationFake) deleteOutside(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.items, id)
}

// onlyID returns the id of the only integration. It is the id of the integration that a scenario made.
func (f *integrationFake) onlyID() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id := range f.items {
		return id
	}
	return ""
}

func (f *integrationFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

func (f *integrationFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	raw, _ := io.ReadAll(r.Body)
	rec := grRequest{Method: r.Method, Path: r.URL.Path}
	if q := r.URL.RawQuery; q != "" {
		rec.Path += "?" + q
	}
	rec.Body = rawRequestBody(raw)
	status, body := f.handle(r.Method, r.URL.Path, rec.Body)
	// The provider lists the parameters in map order. The golden files list them by key.
	sortRecordedParameters(rec.Body)
	rec.Status = status
	f.requests = append(f.requests, rec)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func integrationFail(status int, message string) (int, any) {
	return status, map[string]any{"code": status, "message": http.StatusText(status) + ": " + message}
}

func integrationNotFound(id string) (int, any) {
	return integrationFail(http.StatusNotFound, "Integration not found: "+id)
}

func (f *integrationFake) handle(method, path string, body any) (int, any) {
	_, rest, ok := strings.Cut(path, integrationBasePath)
	if !ok {
		return integrationFail(http.StatusNotFound, "unknown path "+path)
	}
	in, _ := body.(map[string]any)
	switch {
	case method == http.MethodPost && rest == "/metadata/v1/test":
		if f.testStatus != 0 {
			return integrationFail(f.testStatus, "the integration could not be tested")
		}
		metadata, _ := in["integrationData"].(map[string]any)
		if status, answer := integrationValidate(metadata); status != 0 {
			return status, answer
		}
		for _, p := range asList(integrationParams(metadata)) {
			param, _ := p.(map[string]any)
			if param["key"] == "AwsRoleArn" && param["stringValue"] == integrationRoleFailsTest {
				return http.StatusOK, map[string]any{"result": map[string]any{"failure": map[string]any{
					"errorMessage": "Failed to get sdk config: Wrong IAM role and or external id combination provided. RoleArn: " + integrationRoleFailsTest,
				}}}
			}
		}
		return http.StatusOK, map[string]any{"result": map[string]any{"success": map[string]any{}}}
	case method == http.MethodPost && rest == "/metadata/v1":
		if f.failStatus != 0 {
			return integrationFail(f.failStatus, "the integration could not be saved")
		}
		metadata, _ := in["metadata"].(map[string]any)
		if status, answer := integrationValidate(metadata); status != 0 {
			return status, answer
		}
		f.seq++
		id := fmt.Sprintf("00000000-0000-4000-8000-%012d", f.seq)
		f.items[id] = map[string]any{
			"definitionKey":     metadata["integrationKey"],
			"definitionVersion": metadata["version"],
			"parameters":        integrationParams(metadata),
		}
		return http.StatusOK, map[string]any{"integrationId": id}
	case method == http.MethodPut && rest == "/metadata/v1":
		if f.failStatus != 0 {
			return integrationFail(f.failStatus, "the integration could not be updated")
		}
		id, _ := in["id"].(string)
		item, ok := f.items[id]
		if !ok || f.notFound {
			return integrationNotFound(id)
		}
		metadata, _ := in["metadata"].(map[string]any)
		if status, answer := integrationValidate(metadata); status != 0 {
			return status, answer
		}
		// The stored parameters keep the keys that the update leaves out.
		merged := map[string]any{}
		for _, p := range asList(item["parameters"]) {
			merged[p.(map[string]any)["key"].(string)] = p
		}
		for _, p := range asList(integrationParams(metadata)) {
			merged[p.(map[string]any)["key"].(string)] = p
		}
		keys := make([]string, 0, len(merged))
		for k := range merged {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		list := []any{}
		for _, k := range keys {
			list = append(list, merged[k])
		}
		item["definitionVersion"] = metadata["version"]
		item["parameters"] = list
		return http.StatusOK, map[string]any{}
	case method == http.MethodGet && strings.HasPrefix(rest, "/integrations/v1/deployed/"):
		id := strings.TrimPrefix(rest, "/integrations/v1/deployed/")
		if f.readStatus != 0 {
			return integrationFail(f.readStatus, "the integration could not be read")
		}
		item, ok := f.items[id]
		if !ok || f.notFound {
			return integrationNotFound(id)
		}
		return http.StatusOK, map[string]any{"integration": map[string]any{
			"id":                id,
			"definitionKey":     item["definitionKey"],
			"definitionVersion": item["definitionVersion"],
			"parameters":        item["parameters"],
			"integrationStatus": map[string]any{"connectionStatus": "ACTIVE", "details": map[string]any{}, "messages": []any{}},
		}}
	case method == http.MethodDelete && strings.HasPrefix(rest, "/integrations/v1/instances/"):
		id := strings.TrimPrefix(rest, "/integrations/v1/instances/")
		if f.failStatus != 0 {
			return integrationFail(f.failStatus, "the integration could not be deleted")
		}
		if _, ok := f.items[id]; !ok || f.notFound {
			return integrationNotFound(id)
		}
		delete(f.items, id)
		return http.StatusOK, map[string]any{}
	}
	return integrationFail(http.StatusMethodNotAllowed, "unsupported "+method+" "+path)
}

func integrationParams(metadata map[string]any) any {
	parameters, _ := metadata["integrationParameters"].(map[string]any)
	return parameters["parameters"]
}

// integrationValidate answers what a Save or a Test answers for a definition: 404 for an unknown
// key, and 400 for a version, a missing required parameter, or a parameter that the definition
// does not know. It returns status 0 when the metadata is valid.
func integrationValidate(metadata map[string]any) (int, any) {
	key, _ := metadata["integrationKey"].(string)
	def, ok := integrationDefinitions[key]
	if !ok {
		return integrationFail(http.StatusNotFound, "Integration type by integration key not found: "+key)
	}
	if metadata["version"] != def.version {
		return integrationFail(http.StatusBadRequest, "Request errors:\n* Invalid version")
	}
	given := []string{}
	for _, p := range asList(integrationParams(metadata)) {
		given = append(given, p.(map[string]any)["key"].(string))
	}
	for _, name := range def.required {
		if !slices.Contains(given, name) {
			return integrationFail(http.StatusBadRequest, "Request errors:\n* Required field "+name+" not provided")
		}
	}
	for _, name := range given {
		if !slices.Contains(def.known, name) {
			return integrationFail(http.StatusBadRequest, "Request errors:\n* Unknown field "+name+" provided")
		}
	}
	return 0, nil
}
