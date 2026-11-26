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

// The fake integrations API for the integration protocol tests.
// It is not a baseline: the tests that use it assert in code, and no golden file is recorded.

package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const integrationTypeName = "coralogix_integration"

// integrationFake is an in-memory backend for integrations. Like the real API, it holds a
// parameter of an api key type as an apiKey parameter, and it returns the value on a read.
type integrationFake struct {
	// apiKeyParams are the keys that the definition types as an api key.
	apiKeyParams map[string]bool
	mu           sync.Mutex
	stored       map[string]map[string]any
	seq          int
	requests     []grRequest
}

func newIntegrationFake(t *testing.T) *integrationFake {
	t.Helper()
	f := &integrationFake{stored: map[string]map[string]any{}, apiKeyParams: map[string]bool{"ApiKey": true}}
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

func newIntegrationHarness(t *testing.T) *grHarness {
	return newProtocolHarness(t, integrationTypeName, newIntegrationFake(t))
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
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &rec.Body)
	}
	status, body := f.handle(r.Method, r.URL.Path, rec.Body)
	rec.Status = status
	f.requests = append(f.requests, rec)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (f *integrationFake) handle(method, path string, body any) (int, any) {
	in, _ := body.(map[string]any)
	const instances = "/integrations/integrations/v1/instances/"
	const deployed = "/integrations/integrations/v1/deployed/"
	switch {
	case method == http.MethodPost && strings.HasSuffix(path, "/integrations/metadata/v1/test"):
		return http.StatusOK, map[string]any{}
	case method == http.MethodPost && strings.HasSuffix(path, "/integrations/metadata/v1"):
		f.seq++
		id := "integration-" + strconv.Itoa(f.seq)
		f.stored[id] = f.record(id, in["metadata"])
		return http.StatusOK, map[string]any{"integrationId": id}
	case method == http.MethodPut && strings.HasSuffix(path, "/integrations/metadata/v1"):
		id, _ := in["id"].(string)
		if _, found := f.stored[id]; !found {
			return grFail(http.StatusNotFound, "integration not found")
		}
		f.stored[id] = f.record(id, in["metadata"])
		return http.StatusOK, map[string]any{}
	case method == http.MethodGet && strings.Contains(path, deployed):
		id := path[strings.Index(path, deployed)+len(deployed):]
		if v, found := f.stored[id]; found {
			return http.StatusOK, map[string]any{"integration": v}
		}
		return grFail(http.StatusNotFound, "integration not found")
	case method == http.MethodDelete && strings.Contains(path, instances):
		delete(f.stored, path[strings.Index(path, instances)+len(instances):])
		return http.StatusOK, map[string]any{}
	}
	return grFail(http.StatusMethodNotAllowed, "unsupported "+method+" "+path)
}

// record builds the deployed integration from the metadata of a request.
func (f *integrationFake) record(id string, metadata any) map[string]any {
	m, _ := metadata.(map[string]any)
	generic, _ := m["integrationParameters"].(map[string]any)
	given, _ := generic["parameters"].([]any)
	params := make([]any, 0, len(given))
	for _, p := range given {
		param, _ := p.(map[string]any)
		key, _ := param["key"].(string)
		if value, ok := param["stringValue"].(string); ok && f.apiKeyParams[key] {
			params = append(params, map[string]any{"key": key, "apiKey": map[string]any{"value": value}})
			continue
		}
		params = append(params, param)
	}
	return map[string]any{
		"id":                id,
		"definitionKey":     m["integrationKey"],
		"definitionVersion": m["version"],
		"parameters":        params,
	}
}

// sentParameters returns the parameters of the last write request that the fake took, by key.
func sentParameters(t *testing.T, requests []grRequest) map[string]map[string]any {
	t.Helper()
	for i := len(requests) - 1; i >= 0; i-- {
		r := requests[i]
		if (r.Method != "POST" && r.Method != "PUT") || strings.HasSuffix(r.Path, "/test") {
			continue
		}
		body, _ := r.Body.(map[string]any)
		metadata, _ := body["metadata"].(map[string]any)
		generic, _ := metadata["integrationParameters"].(map[string]any)
		list, _ := generic["parameters"].([]any)
		out := map[string]map[string]any{}
		for _, p := range list {
			param, _ := p.(map[string]any)
			out[param["key"].(string)] = param
		}
		return out
	}
	t.Fatalf("no write request in %v", requests)
	return nil
}
