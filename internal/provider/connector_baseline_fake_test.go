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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
)

const cxBasePath = "/notifications/notification-center/v1/connectors"

var cxSelectableTypes = []string{
	"SLACK", "GENERIC_HTTPS", "PAGERDUTY", "SERVICE_NOW", "EMAIL",
	"PAGERDUTY_INCIDENTS", "MICROSOFT_TEAMS", "EVENTBRIDGE", "INCIDENT_IO",
	"IBM_EVENT_NOTIFICATIONS", "CONNECTOR_TYPE_UNSPECIFIED",
}

// cxFake is a small in-memory backend for connectors. It follows the released
// resource's HTTP shape: Create/Replace wrap {connector}, Get/Delete use /{id}.
// Omitted description reads back as "". Config field order is not kept.
type cxFake struct {
	mu           sync.Mutex
	connectors   map[string]map[string]any
	seq          int
	clock        int
	requests     []grRequest
	deleteStatus int
}

func newCXFake(t *testing.T) *cxFake {
	t.Helper()
	f := &cxFake{connectors: map[string]map[string]any{}}
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

func (f *cxFake) seed(connector map[string]any) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	status, body := f.write(map[string]any{"connector": connector}, false)
	if status != http.StatusOK {
		panic(fmt.Sprintf("seed failed: %d %v", status, body))
	}
	return body.(map[string]any)["connector"].(map[string]any)["id"].(string)
}

func (f *cxFake) forget(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.connectors, id)
}

func (f *cxFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

func (f *cxFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	raw, _ := io.ReadAll(r.Body)
	rec := grRequest{Method: r.Method, Path: r.URL.Path}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &rec.Body)
	}
	status, body := f.handle(r.Method, r.URL.Path, rec.Body)
	rec.Body = dropEmptyCollections(rec.Body)
	rec.Status = status
	f.requests = append(f.requests, rec)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (f *cxFake) handle(method, path string, body any) (int, any) {
	_, rest, ok := strings.Cut(path, cxBasePath)
	if !ok {
		return grFail(http.StatusNotFound, "unknown path "+path)
	}
	id := strings.TrimPrefix(rest, "/")

	switch {
	case method == http.MethodPost && id == "":
		return f.write(body, false)
	case method == http.MethodPut && id == "":
		return f.write(body, true)
	case method == http.MethodGet && id != "":
		stored, found := f.connectors[id]
		if !found {
			return grFail(http.StatusNotFound, "Connector with id '"+id+"' not found")
		}
		return http.StatusOK, map[string]any{"connector": f.present(stored)}
	case method == http.MethodDelete && id != "":
		if f.deleteStatus != 0 {
			return grFail(f.deleteStatus, "Connector with id '"+id+"' not found")
		}
		delete(f.connectors, id)
		return http.StatusOK, map[string]any{}
	}
	return grFail(http.StatusMethodNotAllowed, "unsupported "+method+" "+path)
}

func (f *cxFake) uuidLike() string {
	f.seq++
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", f.seq)
}

func (f *cxFake) write(body any, replace bool) (int, any) {
	in, _ := body.(map[string]any)
	connector, _ := in["connector"].(map[string]any)
	if connector == nil {
		return grFail(http.StatusBadRequest, "connector is required")
	}
	if status, resp := f.check(connector, replace); status != 0 {
		return status, resp
	}
	stored := f.normalize(connector)
	id, _ := stored["id"].(string)
	if replace {
		if _, found := f.connectors[id]; !found {
			return grFail(http.StatusNotFound, "Connector with id '"+id+"' not found")
		}
	}
	f.clock++
	stored["updateTime"] = fmt.Sprintf("2026-01-01T00:00:%02dZ", f.clock)
	if _, ok := f.connectors[id]; !ok {
		stored["createTime"] = stored["updateTime"]
	} else {
		stored["createTime"] = f.connectors[id]["createTime"]
	}
	f.connectors[id] = stored
	return http.StatusOK, map[string]any{"connector": f.present(stored)}
}

func (f *cxFake) check(connector map[string]any, replace bool) (int, any) {
	name, _ := connector["name"].(string)
	if name == "" {
		return grFail(http.StatusBadRequest, "name is required")
	}
	typ, _ := connector["type"].(string)
	if typ == "" {
		return grFail(http.StatusBadRequest, "type is required")
	}
	if !slices.Contains(cxSelectableTypes, typ) {
		return grProtoFail(fmt.Sprintf("invalid value for enum field type: %q", typ))
	}
	if replace {
		id, _ := connector["id"].(string)
		if id == "" {
			return grFail(http.StatusBadRequest, "id is required")
		}
	}
	return 0, nil
}

func (f *cxFake) normalize(in map[string]any) map[string]any {
	out := map[string]any{
		"name":            in["name"],
		"type":            in["type"],
		"description":     orDefault(in["description"], ""),
		"configOverrides": orDefault(in["configOverrides"], []any{}),
		"teamId":          int64(1),
	}
	if id, _ := in["id"].(string); id != "" {
		out["id"] = id
	} else {
		out["id"] = f.uuidLike()
	}
	if cfg, ok := in["connectorConfig"].(map[string]any); ok {
		out["connectorConfig"] = map[string]any{"fields": orDefault(cfg["fields"], []any{})}
	} else {
		out["connectorConfig"] = map[string]any{"fields": []any{}}
	}
	out["resolvedConnectorConfig"] = out["connectorConfig"]
	return out
}

func (f *cxFake) present(stored map[string]any) map[string]any {
	out := mapsClone(stored)
	return out
}

func mapsClone(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
