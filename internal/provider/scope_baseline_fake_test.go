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
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

const scopeBasePath = "/aaa/team-scopes/v1"

// scopeFake follows recorded EU2 replies unless a response fault is injected.
type scopeFake struct {
	mu            sync.Mutex
	scopes        map[string]map[string]any
	seq           int
	requests      []grRequest
	status        int
	message       string
	writeResponse map[string]any
	readResponse  map[string]any
}

func newScopeFake(t *testing.T) *scopeFake {
	t.Helper()
	f := &scopeFake{scopes: map[string]map[string]any{}}
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

func (f *scopeFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

func (f *scopeFake) seed() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	id := fmt.Sprintf("00000000-0000-4000-8000-%012d", f.seq)
	f.scopes[id] = map[string]any{"id": id, "teamId": 1234, "displayName": "seeded", "defaultExpression": "<v1>true", "filters": []any{map[string]any{"entityType": "ENTITY_TYPE_LOGS", "expression": "<v1>true"}}}
	return id
}

func (f *scopeFake) forget(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.scopes, id)
}

func (f *scopeFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}
	var body map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &body); err != nil {
			panic(err)
		}
	}
	status, out := f.handle(r, body)
	rec := grRequest{Method: r.Method, Path: r.URL.RequestURI(), Status: status}
	if len(raw) > 0 {
		rec.Body = json.RawMessage(raw)
	}
	f.requests = append(f.requests, rec)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(out)
}

func scopeFailure(status int, message string) (int, any) {
	return status, map[string]any{"code": status, "message": message}
}

func (f *scopeFake) handle(r *http.Request, body map[string]any) (int, any) {
	if f.status != 0 {
		return scopeFailure(f.status, f.message)
	}
	_, rest, ok := strings.Cut(r.URL.Path, scopeBasePath)
	if !ok {
		return scopeFailure(404, "Not Found: Not Found")
	}
	pathID, hasPathID := strings.CutPrefix(rest, "/")
	switch {
	case r.Method == http.MethodGet && rest == "":
		if f.readResponse != nil {
			return 200, f.readResponse
		}
		var found []any
		for _, id := range r.URL.Query()["ids"] {
			if scope, ok := f.scopes[id]; ok {
				found = append(found, scope)
			}
		}
		if len(found) == 0 {
			return scopeFailure(404, "Not Found: No scopes found")
		}
		return 200, map[string]any{"scopes": found}
	case (r.Method == http.MethodPost || r.Method == http.MethodPut) && rest == "":
		id, _ := body["id"].(string)
		if r.Method == http.MethodPut {
			if _, ok := f.scopes[id]; !ok {
				return scopeFailure(404, "Not Found: Scope not found")
			}
		}
		filters, _ := body["filters"].([]any)
		defaultExpression, _ := body["defaultExpression"].(string)
		expressions := []string{defaultExpression}
		for _, filter := range filters {
			fields := filter.(map[string]any)
			if fields["entityType"] == "ENTITY_TYPE_UNSPECIFIED" {
				return scopeFailure(400, "Bad Request: Incoming entity type is unspecified")
			}
			expression, _ := fields["expression"].(string)
			expressions = append(expressions, expression)
		}
		// Only these recorded inputs model expression compilation failures.
		for _, expression := range expressions {
			switch expression {
			case "", "true":
				return scopeFailure(412, fmt.Sprintf("Failed Precondition: Failed to compile expression. Expression: [%s], Errors: [expected version tag. e.g. \"<v1> foo == 'bar'\"]", expression))
			case "<v1>true-whatever":
				return scopeFailure(412, "Failed Precondition: Failed to compile expression. Expression: [<v1>true-whatever], Errors: [expression type could not be inferred, please add a cast, operator cannot be applied to types bool and unknown, expected one of [number and number, timestamp and interval, timestamp and timestamp, interval and interval]]")
			}
		}
		if r.Method == http.MethodPost {
			f.seq++
			id = fmt.Sprintf("00000000-0000-4000-8000-%012d", f.seq)
		}
		scope := maps.Clone(body)
		scope["id"], scope["teamId"] = id, 1234
		if len(filters) == 0 {
			scope["filters"] = []any{}
		}
		f.scopes[id] = scope
		if f.writeResponse != nil {
			return 200, f.writeResponse
		}
		return 200, map[string]any{"scope": scope}
	case r.Method == http.MethodDelete && hasPathID && pathID != "" && !strings.Contains(pathID, "/"):
		delete(f.scopes, pathID)
		return 200, map[string]any{}
	}
	return scopeFailure(404, "Not Found: Not Found")
}

func (f *scopeFake) fail(status int, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.status, f.message = status, message
}

func (f *scopeFake) replyToWrite(response map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writeResponse = response
}

func (f *scopeFake) replyToRead(response map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.readResponse = response
}
