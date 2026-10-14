// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	https://www.apache.org/licenses/LICENSE-2.0
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
	"regexp"
	"strings"
	"sync"
	"testing"
)

const dashboardsFolderBasePath = "/dashboards/folders/v1"

// dashboardsFolderFake follows the dashboard folders service as recorded in
// fidelity/recorded.json, not the Go client. Create returns only the folder id, and keeps the
// id that the request sets. Get returns {folder: {...}}. Replace returns {}. An omitted
// parentId is left out of the answer.
type dashboardsFolderFake struct {
	mu       sync.Mutex
	folders  map[string]map[string]any
	seq      int
	requests []grRequest
	// seeded are the ids of folders that exist and that a test does not list, so a folder can
	// name them as its parent.
	seeded map[string]bool
}

func newDashboardsFolderFake(t *testing.T) *dashboardsFolderFake {
	t.Helper()
	f := &dashboardsFolderFake{folders: map[string]map[string]any{}, seeded: map[string]bool{}}
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

// seed stores a folder as if it was made outside Terraform. It returns the id.
func (f *dashboardsFolderFake) seed(folder map[string]any) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	status, body := f.write(map[string]any{"folder": folder}, false)
	if status != http.StatusOK {
		panic(fmt.Sprintf("seed failed: %d %v", status, body))
	}
	return body.(map[string]any)["folderId"].(string)
}

// forget deletes a folder without a request, as if it was deleted outside Terraform.
func (f *dashboardsFolderFake) forget(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.folders, id)
}

func (f *dashboardsFolderFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

func (f *dashboardsFolderFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

// dashboardsFolderFail is an error answer of the real API: the status name, then the reason.
func dashboardsFolderFail(status int, reason string) (int, any) {
	prefix := map[int]string{
		http.StatusBadRequest: "Bad Request", http.StatusForbidden: "Permission Denied",
		http.StatusNotFound: "Not Found", http.StatusConflict: "Already Exists",
	}[status]
	return status, map[string]any{"code": status, "message": prefix + ": " + reason}
}

var folderIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func folderMissing(id string) string { return fmt.Sprintf("DashboardFolderId(%s)", id) }

func (f *dashboardsFolderFake) handle(method, path string, body any) (int, any) {
	_, rest, ok := strings.Cut(path, dashboardsFolderBasePath)
	if !ok {
		return dashboardsFolderFail(http.StatusNotFound, "unknown path "+path)
	}
	id := strings.TrimPrefix(rest, "/")
	if id != "" && !folderIDPattern.MatchString(id) {
		return dashboardsFolderFail(http.StatusBadRequest, "invalid uuid")
	}
	switch {
	case method == http.MethodPost && rest == "":
		return f.write(body, false)
	case method == http.MethodPut && rest == "":
		return f.write(body, true)
	case method == http.MethodGet && rest == "":
		list := []any{}
		for _, folder := range f.folders {
			list = append(list, folder)
		}
		return http.StatusOK, map[string]any{"folder": list}
	case method == http.MethodGet && id != "":
		folder, ok := f.folders[id]
		if !ok {
			return dashboardsFolderFail(http.StatusNotFound, "dashboard folder with id "+folderMissing(id)+" does not exist")
		}
		return http.StatusOK, map[string]any{"folder": folder}
	case method == http.MethodDelete && id != "":
		// The API answers 403, not 404, for a folder that does not exist.
		if _, ok := f.folders[id]; !ok {
			return dashboardsFolderFail(http.StatusForbidden, "dont have permissions to delete folder "+folderMissing(id))
		}
		delete(f.folders, id)
		return http.StatusOK, map[string]any{}
	}
	return dashboardsFolderFail(http.StatusNotFound, "unknown route "+method+" "+path)
}

func (f *dashboardsFolderFake) write(body any, replace bool) (int, any) {
	req, _ := body.(map[string]any)
	in, _ := req["folder"].(map[string]any)
	if in == nil {
		in = map[string]any{}
	}
	id, _ := in["id"].(string)
	if id != "" && !folderIDPattern.MatchString(id) {
		return dashboardsFolderFail(http.StatusBadRequest, "invalid uuid")
	}
	name, hasName := in["name"].(string)
	parent, _ := in["parentId"].(string)
	switch {
	case replace && id == "":
		return dashboardsFolderFail(http.StatusBadRequest, "folder id is required")
	case !hasName:
		return dashboardsFolderFail(http.StatusBadRequest, "folder name not found")
	case name == "":
		return dashboardsFolderFail(http.StatusBadRequest, "folder name cannot be empty")
	case len([]rune(name)) > 100:
		return dashboardsFolderFail(http.StatusBadRequest, "folder name length is bigger than 100")
	case parent != "" && !folderIDPattern.MatchString(parent):
		return dashboardsFolderFail(http.StatusBadRequest, "invalid uuid")
	case replace && parent == id:
		return dashboardsFolderFail(http.StatusBadRequest, fmt.Sprintf("folder_id and parent_id can't be equal (%s)", id))
	case replace:
		if _, ok := f.folders[id]; !ok {
			return dashboardsFolderFail(http.StatusBadRequest, "folder with id "+folderMissing(id)+" does not exist")
		}
	}
	if parent != "" {
		if _, ok := f.folders[parent]; !ok && !f.seeded[parent] {
			return dashboardsFolderFail(http.StatusBadRequest, "parent folder id "+folderMissing(parent)+" for company CompanyId(1) does not exist")
		}
	}
	if !replace {
		if id == "" {
			f.seq++
			id = fmt.Sprintf("00000000-0000-4000-8000-%012d", f.seq)
		}
		for existing, folder := range f.folders {
			if existing == id || folder["name"] == name {
				return dashboardsFolderFail(http.StatusConflict, fmt.Sprintf("folder with id %s or name %s already exist in your company", folderMissing(id), name))
			}
		}
	}
	folder := map[string]any{"id": id, "name": name}
	if parent != "" {
		folder["parentId"] = parent
	}
	f.folders[id] = folder
	if replace {
		return http.StatusOK, map[string]any{}
	}
	return http.StatusOK, map[string]any{"folderId": id}
}
