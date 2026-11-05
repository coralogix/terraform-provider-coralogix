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
	"regexp"
	"strings"
	"sync"
	"testing"
)

// arTagIDs are the ids of the retention tags of the fake company, in order.
var arTagIDs = []string{
	"11111111-1111-4111-8111-111111111111",
	"22222222-2222-4222-8222-222222222222",
	"33333333-3333-4333-8333-333333333333",
	"44444444-4444-4444-8444-444444444444",
}

// arDefaultNames are the names that the service gives the retention tags of a new company.
var arDefaultNames = []string{"Default", "Short", "Intermediate", "Long"}

var (
	arNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	arUUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

// arFake is a small in-memory backend of the archive retention tags (GET and POST
// /dataengine/retention-tags/v1), as ws-archive serves it. A company has a fixed number of tags,
// ordered from 1; only the first, Default, is not editable. Get returns every tag in order, and
// creates the default tags when the company has none. Update validates each item (id a uuid,
// name required, ^[a-zA-Z0-9_-]+$, at most 255 characters), the list size (at most the tag
// count), and unique names within the request (validate-retentions.ts). Then an unknown id is a
// 404 and a new name for a tag that is not editable is a 403 (retention.service.ts). It changes
// only the names of the tags in the request; the last item wins when an id repeats. It returns
// the items of the request, in request order, with the stored order and editable set from the new
// name. Errors are JSON with the gRPC status text; the 500 messages leave out the company id that
// the service adds. It follows the answers recorded on EU2
// (archive_retentions_fidelity_test.go).
type arFake struct {
	mu       sync.Mutex
	count    int              // the retention tag count of the company
	tags     []map[string]any // nil until the first read creates the defaults
	requests []grRequest
	// getFail, when set, answers every GET instead of the tags.
	getFail func() (int, any)
	// setFail, when set, answers every POST instead of renaming.
	setFail func() (int, any)
	// getFailAfterSet makes every GET fail once a POST has renamed tags.
	getFailAfterSet func() (int, any)
}

func newARFake(t *testing.T) *arFake {
	t.Helper()
	f := &arFake{}
	f.reset()
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

// reset gives the company its default tags: Default, Short, Intermediate, Long, then Tag_4 and up
// when the tag count is larger (retention.service.ts, addRetentions).
func (f *arFake) reset() {
	if f.count == 0 {
		f.count = len(arTagIDs)
	}
	f.tags = nil
	for i := 0; i < f.count; i++ {
		id := fmt.Sprintf("%d%d%d%d%d%d%d%d-0000-4000-8000-000000000000", i+1, i+1, i+1, i+1, i+1, i+1, i+1, i+1)
		name := fmt.Sprintf("Tag_%d", i)
		if i < len(arTagIDs) {
			id, name = arTagIDs[i], arDefaultNames[i]
		}
		f.tags = append(f.tags, map[string]any{"id": id, "order": i + 1, "name": name, "editable": i > 0})
	}
}

// setCount gives the company count tags with their default names.
func (f *arFake) setCount(count int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.count = count
	f.reset()
}

// rename changes the stored name of tag i (from 0), as another client would.
func (f *arFake) rename(i int, name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tags[i]["name"] = name
}

// forget removes the tags, as a company that never read them.
func (f *arFake) forget() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tags = nil
}

// failGets makes every GET answer with the given error.
func (f *arFake) failGets(status int, prefix, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getFail = func() (int, any) { return amFail(status, prefix, message) }
}

// failGetsAfterSet makes every GET answer with the given error once a POST has succeeded.
func (f *arFake) failGetsAfterSet(status int, prefix, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getFailAfterSet = func() (int, any) { return amFail(status, prefix, message) }
}

// healGets makes GET answer from the stored tags again.
func (f *arFake) healGets() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getFail, f.getFailAfterSet = nil, nil
}

// failSets makes every POST answer with the given error.
func (f *arFake) failSets(status int, prefix, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setFail = func() (int, any) { return amFail(status, prefix, message) }
}

func (f *arFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

func (f *arFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

func (f *arFake) handle(method, path string, body any) (int, any) {
	if !strings.HasSuffix(path, arPath) {
		return grFail(http.StatusNotFound, "unknown path "+path)
	}
	switch method {
	case http.MethodGet:
		if f.getFail != nil {
			return f.getFail()
		}
		if f.tags == nil {
			f.reset()
		}
		out := []any{}
		for _, tag := range f.tags {
			out = append(out, mapsClone(tag))
		}
		return http.StatusOK, map[string]any{"retentions": out}
	case http.MethodPost:
		if f.setFail != nil {
			return f.setFail()
		}
		status, resp := f.update(body)
		if status == http.StatusOK && f.getFailAfterSet != nil {
			f.getFail = f.getFailAfterSet
		}
		return status, resp
	}
	return grFail(http.StatusMethodNotAllowed, "unsupported "+method+" "+path)
}

func (f *arFake) update(body any) (int, any) {
	// Update reads the stored tags without creating the defaults, so a company that never read
	// its tags has none, and any id is unknown.
	in, _ := body.(map[string]any)
	items, _ := in["retentionUpdateElements"].([]any)
	count := f.count
	if count == 0 {
		count = len(arTagIDs)
	}
	if msg := arValidate(items, count); msg != "" {
		return amFail(http.StatusBadRequest, "Bad Request", msg)
	}
	byID := map[string]map[string]any{}
	for _, tag := range f.tags {
		byID[tag["id"].(string)] = tag
	}
	for _, item := range items {
		m := item.(map[string]any)
		if byID[m["id"].(string)] == nil {
			return amFail(http.StatusNotFound, "Not Found", "Could not set archive retentions, Non existing uuid present on request.")
		}
	}
	for _, item := range items {
		m := item.(map[string]any)
		tag := byID[m["id"].(string)]
		if tag["editable"] == false && tag["name"] != m["name"] {
			return amFail(http.StatusForbidden, "Permission Denied", "Could not set archive retentions. Could not edit uneditable retention name.")
		}
	}
	out := []any{}
	for _, item := range items {
		m := item.(map[string]any)
		tag := byID[m["id"].(string)]
		tag["name"] = m["name"]
		out = append(out, map[string]any{"id": m["id"], "order": tag["order"], "name": m["name"], "editable": m["name"] != "Default"})
	}
	return http.StatusOK, map[string]any{"retentions": out}
}

// arValidate returns the first message of the request validator, or "".
func arValidate(items []any, count int) string {
	for i, item := range items {
		m, _ := item.(map[string]any)
		id, hasID := m["id"].(string)
		switch {
		case !hasID:
			return fmt.Sprintf(`"[%d].id" is required`, i)
		case id == "":
			return fmt.Sprintf(`"[%d].id" is not allowed to be empty`, i)
		case !arUUIDPattern.MatchString(id):
			return fmt.Sprintf(`"[%d].id" must be a valid GUID`, i)
		}
		name, hasName := m["name"].(string)
		switch {
		case !hasName:
			return fmt.Sprintf(`"[%d].name" is required`, i)
		case name == "":
			return fmt.Sprintf(`"[%d].name" is not allowed to be empty`, i)
		case !arNamePattern.MatchString(name):
			return fmt.Sprintf(`"[%d].name" with value "%s" fails to match the required pattern: /^[a-zA-Z0-9_-]+$/`, i, name)
		case len(name) > 255:
			return fmt.Sprintf(`"[%d].name" length must be less than or equal to 255 characters long`, i)
		}
	}
	if len(items) > count {
		return fmt.Sprintf(`"value" must contain less than or equal to %d items`, count)
	}
	seen := map[any]bool{}
	for i, item := range items {
		name := item.(map[string]any)["name"]
		if seen[name] {
			return fmt.Sprintf(`"[%d]" contains a duplicate value`, i)
		}
		seen[name] = true
	}
	return ""
}
