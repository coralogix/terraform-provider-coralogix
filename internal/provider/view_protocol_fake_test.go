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
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const (
	viewTypeName = "coralogix_view"
	viewBasePath = "/data-exploration/views/v1/views"
)

// viewPresets are the quick time ranges that the service stores, with their captions
// (captionSecondsMap in eng-svc-views).
var viewPresets = map[int]string{
	60: "Last 1 minute", 120: "Last 2 minutes", 300: "Last 5 minutes", 900: "Last 15 minutes",
	1800: "Last 30 minutes", 3600: "Last 1 hour", 7200: "Last 2 hours", 21600: "Last 6 hours",
	43200: "Last 12 hours", 86400: "Last 24 hours", 172800: "Last 2 Days", 259200: "Last 3 Days",
	432000: "Last 5 Days", 604800: "Last 7 Days",
}

// viewFail is an error answer of the service as the gateway writes it: the HTTP status
// text, then the gRPC error as JSON.
func viewFail(status int, grpcCode int, grpcName, message string, details string) (int, any) {
	inner := fmt.Sprintf(`{"message":"%s: %s","status":%d`, grpcName, message, grpcCode)
	if details != "" {
		inner += `,"details":` + details
	}
	return status, map[string]any{"code": status, "message": http.StatusText(status) + ": " + inner + "}"}
}

func viewNotFound(id string) (int, any) {
	return viewFail(http.StatusNotFound, 5, "NOT_FOUND", "view not found", `{"viewId":`+id+`}`)
}

// viewFake is an in-memory backend for views. It follows the request handling of the
// views service:
//   - create stores isCompactMode false, whatever the request sends
//   - replace stores an omitted isCompactMode as false
//   - an omitted or unspecified viewType is stored as VIEW_TYPE_LOGS
//   - an omitted or unspecified syntaxType is stored as SYNTAX_TYPE_LUCENE
//   - seconds is stored as the nearest preset, and caption is the label of that preset
//   - selectedValues keeps only the values set to true
//   - a view saved on another Explore tab reads back without viewType
type viewFake struct {
	// createCompact is what create stores for isCompactMode. The service stores false.
	createCompact bool
	// omitDefaults leaves viewType, isCompactMode, and searchQuery.syntaxType out of a response
	// when they hold the default, as an API may for a value that equals its default.
	omitDefaults bool
	mu           sync.Mutex
	views        map[string]map[string]any
	seq          int
	requests     []grRequest
}

func newViewFake(t *testing.T) *viewFake {
	t.Helper()
	f := &viewFake{views: map[string]map[string]any{}}
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

func newViewHarness(t *testing.T) *grHarness {
	return newProtocolHarness(t, viewTypeName, newViewFake(t))
}

func (h *grHarness) views() *viewFake {
	h.t.Helper()
	f, ok := h.api.(*viewFake)
	if !ok {
		h.t.Fatal("expected *viewFake")
	}
	return f
}

// seedLegacy stores a view saved on another Explore tab: its viewType reads back as absent.
func (f *viewFake) seedLegacy(name string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	id := strconv.Itoa(f.seq)
	f.views[id] = map[string]any{
		"id":            f.seq,
		"name":          name,
		"isCompactMode": false,
		"timeSelection": map[string]any{"quickSelection": map[string]any{"seconds": 3600, "caption": viewPresets[3600]}},
		"searchQuery":   map[string]any{"query": "", "syntaxType": "SYNTAX_TYPE_LUCENE"},
		"filters":       map[string]any{"filters": []any{}},
	}
	return id
}

func (f *viewFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

func (f *viewFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

func (f *viewFake) handle(method, path string, body any) (int, any) {
	_, rest, ok := strings.Cut(path, viewBasePath)
	if !ok {
		return grFail(http.StatusNotFound, "Not Found")
	}
	id := strings.TrimPrefix(rest, "/")
	in, _ := body.(map[string]any)
	switch {
	case method == http.MethodPost && rest == "":
		f.seq++
		id = strconv.Itoa(f.seq)
		return f.store(id, f.seq, in, false)
	case method == http.MethodPut && id != "":
		if _, found := f.views[id]; !found {
			return viewNotFound(id)
		}
		n, _ := strconv.Atoi(id)
		return f.store(id, n, in, true)
	case method == http.MethodGet && id != "":
		if v, found := f.views[id]; found {
			return http.StatusOK, f.response(v)
		}
		return viewNotFound(id)
	case method == http.MethodDelete && id != "":
		if _, found := f.views[id]; !found {
			return viewNotFound(id)
		}
		delete(f.views, id)
		return http.StatusOK, map[string]any{}
	}
	return grFail(http.StatusMethodNotAllowed, "unsupported "+method+" "+path)
}

func (f *viewFake) store(id string, n int, in map[string]any, replace bool) (int, any) {
	name, _ := in["name"].(string)
	if name == "" {
		return viewFail(http.StatusBadRequest, 3, "INVALID_ARGUMENT", "view name is required", "")
	}
	ts, _ := in["timeSelection"].(map[string]any)
	quick, _ := ts["quickSelection"].(map[string]any)
	if quick == nil {
		return viewFail(http.StatusBadRequest, 3, "INVALID_ARGUMENT", "Invalid time selection", "")
	}
	seconds, _ := quick["seconds"].(float64)
	nearest := 60
	for preset := range viewPresets {
		if math.Abs(float64(preset)-seconds) < math.Abs(float64(nearest)-seconds) {
			nearest = preset
		}
	}
	viewType, _ := in["viewType"].(string)
	if viewType == "" || viewType == "VIEW_TYPE_UNSPECIFIED" {
		viewType = "VIEW_TYPE_LOGS"
	}
	sq, _ := in["searchQuery"].(map[string]any)
	query, _ := sq["query"].(string)
	syntax, _ := sq["syntaxType"].(string)
	if syntax == "" || syntax == "SYNTAX_TYPE_UNSPECIFIED" {
		syntax = "SYNTAX_TYPE_LUCENE"
	}
	compact := f.createCompact
	if replace {
		compact, _ = in["isCompactMode"].(bool)
	}
	filters := []any{}
	if fs, _ := in["filters"].(map[string]any); fs != nil {
		items, _ := fs["filters"].([]any)
		for _, item := range items {
			filter, _ := item.(map[string]any)
			selected := map[string]any{}
			values, _ := filter["selectedValues"].(map[string]any)
			for k, v := range values {
				if v == true {
					selected[k] = true
				}
			}
			out := map[string]any{"name": filter["name"]}
			if len(selected) > 0 {
				out["selectedValues"] = selected
			}
			filters = append(filters, out)
		}
	}
	stored := map[string]any{
		"id":            n,
		"name":          name,
		"isCompactMode": compact,
		"viewType":      viewType,
		"timeSelection": map[string]any{"quickSelection": map[string]any{"seconds": nearest, "caption": viewPresets[nearest]}},
		"searchQuery":   map[string]any{"query": query, "syntaxType": syntax},
		"filters":       map[string]any{"filters": filters},
	}
	if folder, _ := in["folderId"].(string); folder != "" {
		stored["folderId"] = folder
	}
	f.views[id] = stored
	return http.StatusOK, f.response(stored)
}

// response is what the service returns for a stored view.
func (f *viewFake) response(stored map[string]any) map[string]any {
	if !f.omitDefaults {
		return stored
	}
	out := map[string]any{}
	for k, v := range stored {
		out[k] = v
	}
	if out["viewType"] == "VIEW_TYPE_LOGS" {
		delete(out, "viewType")
	}
	if out["isCompactMode"] == false {
		delete(out, "isCompactMode")
	}
	if sq, _ := out["searchQuery"].(map[string]any); sq["syntaxType"] == "SYNTAX_TYPE_LUCENE" {
		trimmed := map[string]any{}
		for k, v := range sq {
			if k != "syntaxType" {
				trimmed[k] = v
			}
		}
		out["searchQuery"] = trimmed
	}
	return out
}
