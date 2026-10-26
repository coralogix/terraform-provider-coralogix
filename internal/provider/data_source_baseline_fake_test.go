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
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
)

// listByID returns the records in the order of their ids, as present shows them. The ids of the
// fakes grow with each write, so a record written later comes later.
func listByID(records map[string]map[string]any, present func(map[string]any) map[string]any) []any {
	out := make([]any, 0, len(records))
	for _, id := range slices.Sorted(maps.Keys(records)) {
		out = append(out, present(records[id]))
	}
	return out
}

// summaries are the system and custom presets as the summaries List shows them: no config overrides.
func (f *presetFake) summaries() []any {
	all := maps.Clone(f.presets)
	maps.Copy(all, f.system)
	return listByID(all, func(stored map[string]any) map[string]any {
		out := map[string]any{}
		for _, key := range []string{"id", "name", "description", "entityType", "connectorType", "presetType", "parentId"} {
			if v, ok := stored[key]; ok {
				out[key] = v
			}
		}
		return out
	})
}

// copyWithNewID stores a copy of the record under a new id, with the same name. A test uses it
// for two records with one name, which the fakes refuse on a write.
func copyWithNewID(records map[string]map[string]any, id, newID string) string {
	stored := maps.Clone(records[id])
	stored["id"] = newID
	records[newID] = stored
	return newID
}

func (f *cxFake) duplicate(id string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return copyWithNewID(f.connectors, id, f.uuidLike())
}

func (f *grFake) duplicate(id string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return copyWithNewID(f.routers, id, f.uuidLike())
}

func (f *presetFake) duplicate(id string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return copyWithNewID(f.presets, id, f.uuidLike())
}

// recordFake serves the reads of a data source for a resource without a protocol fake: Get by id
// and, with listPath, a List. A test seeds the records as the API returns them.
type recordFake struct {
	mu sync.Mutex
	// getPath is the Get path without the id: Get is getPath/{id}. The client adds a prefix to
	// every path.
	getPath string
	// listPath is the path of the List, and listKey the response property of its items. "" when
	// the API has no List.
	listPath, listKey string
	// notFound is the message of a Get of a missing id.
	notFound string
	records  map[string]map[string]any
	requests []grRequest
}

func newRecordFake(t *testing.T, getPath, listPath, listKey, notFound string) *recordFake {
	t.Helper()
	f := &recordFake{getPath: getPath, listPath: listPath, listKey: listKey, notFound: notFound, records: map[string]map[string]any{}}
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

// seed stores the record as is. It returns its id.
func (f *recordFake) seed(record map[string]any) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := record["id"].(string)
	f.records[id] = record
	return id
}

func (f *recordFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

func (f *recordFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	raw, _ := io.ReadAll(r.Body)
	rec := grRequest{Method: r.Method, Path: r.URL.Path}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &rec.Body)
	}
	status, body := f.handle(r.Method, r.URL.Path)
	rec.Status = status
	f.requests = append(f.requests, rec)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (f *recordFake) handle(method, path string) (int, any) {
	if method != http.MethodGet {
		return grFail(http.StatusMethodNotAllowed, "unsupported "+method+" "+path)
	}
	// The client adds a prefix before the path of the operation.
	_, rest, ok := strings.Cut(path, f.getPath)
	if !ok {
		return grFail(http.StatusNotFound, "unknown path "+path)
	}
	if f.listPath != "" && strings.HasSuffix(path, f.listPath) {
		return http.StatusOK, map[string]any{f.listKey: listByID(f.records, maps.Clone)}
	}
	id, ok := strings.CutPrefix(rest, "/")
	if !ok || id == "" || strings.Contains(id, "/") {
		return grFail(http.StatusNotFound, "unknown path "+path)
	}
	stored, found := f.records[id]
	if !found {
		return grFail(http.StatusNotFound, fmt.Sprintf(f.notFound, id))
	}
	return http.StatusOK, maps.Clone(stored)
}
