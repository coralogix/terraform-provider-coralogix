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

// The fake connectors API for the connector protocol tests.
// It is not a baseline: the tests that use it assert in code, and no golden file is recorded.

package provider

import (
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const (
	connectorTypeName = "coralogix_connector"
	connectorBasePath = "/notifications/notification-center/v1/connectors"
)

// connectorFake is an in-memory backend for connectors. Like the real API, it returns every
// stored value on a read, including a template or a field value that a client sent as a secret.
type connectorFake struct {
	mu         sync.Mutex
	connectors map[string]map[string]any
	seq        int
	requests   []grRequest
}

func newConnectorFake(t *testing.T) *connectorFake {
	t.Helper()
	f := &connectorFake{connectors: map[string]map[string]any{}}
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

func newConnectorHarness(t *testing.T) *grHarness {
	return newProtocolHarness(t, connectorTypeName, newConnectorFake(t))
}

func (f *connectorFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

func (f *connectorFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

func (f *connectorFake) handle(method, path string, body any) (int, any) {
	_, rest, ok := strings.Cut(path, connectorBasePath)
	if !ok {
		return grFail(http.StatusNotFound, "Not Found")
	}
	id := strings.TrimPrefix(rest, "/")
	in, _ := body.(map[string]any)
	connector, _ := in["connector"].(map[string]any)
	switch {
	case method == http.MethodPost && rest == "":
		f.seq++
		stored := maps.Clone(connector)
		if _, set := stored["id"]; !set {
			stored["id"] = "connector-" + strconv.Itoa(f.seq)
		}
		f.connectors[stored["id"].(string)] = stored
		return http.StatusOK, map[string]any{"connector": stored}
	case method == http.MethodPut && rest == "":
		id, _ = connector["id"].(string)
		if _, found := f.connectors[id]; !found {
			return grFail(http.StatusNotFound, "connector not found")
		}
		f.connectors[id] = maps.Clone(connector)
		return http.StatusOK, map[string]any{"connector": f.connectors[id]}
	case method == http.MethodGet && id != "":
		if c, found := f.connectors[id]; found {
			return http.StatusOK, map[string]any{"connector": c}
		}
		return grFail(http.StatusNotFound, "connector not found")
	case method == http.MethodDelete && id != "":
		delete(f.connectors, id)
		return http.StatusOK, map[string]any{}
	}
	return grFail(http.StatusMethodNotAllowed, "unsupported "+method+" "+path)
}

// sentOverrideTemplates returns the templates of the last write request, keyed by
// "<API entity type>.<field name>".
func sentOverrideTemplates(t *testing.T, requests []grRequest) map[string]string {
	t.Helper()
	for i := len(requests) - 1; i >= 0; i-- {
		r := requests[i]
		if r.Method != "POST" && r.Method != "PUT" {
			continue
		}
		body, _ := r.Body.(map[string]any)
		connector, _ := body["connector"].(map[string]any)
		overrides, _ := connector["configOverrides"].([]any)
		out := map[string]string{}
		for _, o := range overrides {
			override, _ := o.(map[string]any)
			fields, _ := override["fields"].([]any)
			for _, f := range fields {
				field, _ := f.(map[string]any)
				template, _ := field["template"].(string)
				out[override["entityType"].(string)+"."+field["fieldName"].(string)] = template
			}
		}
		return out
	}
	t.Fatalf("no write request in %v", requests)
	return nil
}
