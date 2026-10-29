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
	"strings"
	"sync"
	"testing"
)

const (
	ipAccessSettingsID = "settings-id"
	ipAccessEntryID    = "entry-%d"
)

// ipAccessFake models the public REST contract and the behavior recorded by the handwritten
// resource tests. The settings ID and entry keys are synthetic; no tenant or company data is used.
type ipAccessFake struct {
	mu       sync.Mutex
	settings map[string]any
	seq      int
	requests []grRequest
}

func newIpAccessFake(t *testing.T) *ipAccessFake {
	t.Helper()
	f := &ipAccessFake{}
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

func (h *grHarness) ipAccess() *ipAccessFake {
	h.t.Helper()
	f, ok := h.api.(*ipAccessFake)
	if !ok {
		h.t.Fatal("expected *ipAccessFake")
	}
	return f
}

func (f *ipAccessFake) seed(settings map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.settings = f.makeSettings(settings)
}

func (f *ipAccessFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

func (f *ipAccessFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

func (f *ipAccessFake) handle(method, path string, body any) (int, any) {
	if !strings.HasSuffix(path, ipAccessBasePath) {
		return http.StatusNotFound, map[string]any{"code": http.StatusNotFound, "message": "Not Found"}
	}
	switch method {
	case http.MethodPost:
		if f.settings != nil {
			return http.StatusConflict, map[string]any{"code": http.StatusConflict, "message": "Already Exists"}
		}
		request := asObject(body)
		f.settings = f.makeSettings(request)
		return http.StatusOK, map[string]any{"settings": f.copySettingsWithAccess(enumValue(request))}
	case http.MethodPut:
		if f.settings == nil {
			return http.StatusNotFound, map[string]any{"code": http.StatusNotFound, "message": "Not Found"}
		}
		request := asObject(body)
		responseAccess := enumValue(request)
		// Replace with UNSPECIFIED preserves the stored support-access setting.
		if request["enableCoralogixCustomerSupportAccess"] == "CORALOGIX_CUSTOMER_SUPPORT_ACCESS_UNSPECIFIED" {
			request["enableCoralogixCustomerSupportAccess"] = f.settings["enableCoralogixCustomerSupportAccess"]
		}
		f.settings = f.makeSettings(request)
		return http.StatusOK, map[string]any{"settings": f.copySettingsWithAccess(responseAccess)}
	case http.MethodGet:
		if f.settings == nil {
			return http.StatusNotFound, map[string]any{"code": http.StatusNotFound, "message": "Not Found"}
		}
		return http.StatusOK, map[string]any{"settings": f.copySettings()}
	case http.MethodDelete:
		f.settings = nil
		return http.StatusOK, map[string]any{}
	default:
		return http.StatusNotFound, map[string]any{"code": http.StatusNotFound, "message": "Not Found"}
	}
}

func (f *ipAccessFake) makeSettings(input map[string]any) map[string]any {
	access := enumValue(input)
	if access == "" || access == "CORALOGIX_CUSTOMER_SUPPORT_ACCESS_UNSPECIFIED" {
		access = "CORALOGIX_CUSTOMER_SUPPORT_ACCESS_DISABLED"
	}
	items := map[string]any{}
	if raw, ok := input["ipAccess"].([]any); ok {
		for _, entry := range raw {
			f.seq++
			items[fmtEntryID(f.seq)] = entry
		}
	} else if raw, ok := input["ipAccess"].(map[string]any); ok {
		for key, entry := range raw {
			items[key] = entry
		}
	}
	return map[string]any{
		"id":                                   ipAccessSettingsID,
		"ipAccess":                             items,
		"enableCoralogixCustomerSupportAccess": access,
	}
}

func (f *ipAccessFake) copySettings() map[string]any {
	encoded, _ := json.Marshal(f.settings)
	var out map[string]any
	_ = json.Unmarshal(encoded, &out)
	return out
}

func (f *ipAccessFake) copySettingsWithAccess(access string) map[string]any {
	out := f.copySettings()
	out["enableCoralogixCustomerSupportAccess"] = access
	return out
}

func enumValue(input map[string]any) string {
	access, _ := input["enableCoralogixCustomerSupportAccess"].(string)
	if access == "" {
		return "CORALOGIX_CUSTOMER_SUPPORT_ACCESS_UNSPECIFIED"
	}
	return access
}

func asObject(value any) map[string]any {
	if object, ok := value.(map[string]any); ok {
		return object
	}
	return map[string]any{}
}

func fmtEntryID(seq int) string {
	return fmt.Sprintf(ipAccessEntryID, seq)
}
