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
	"strings"
	"sync"
	"testing"
)

const presetBasePath = "/notifications/notification-center/v1/presets"

const presetSystemParent = "preset_system_generic_https_alerts_empty"

// presetFake follows notification-config-hub PresetsService, not the Go client.
// Custom create requires a system parent whose entity and connector types match.
// presetType is ignored on input and stored as CUSTOM. An empty description and
// an AUTO attachment policy are omitted on read (proto3 defaults). configOverrides
// keep their order and a replace stores exactly the list that was sent. Message
// fields are a set ordered by fieldName; a repeated fieldName keeps the first.
type presetFake struct {
	mu           sync.Mutex
	presets      map[string]map[string]any
	system       map[string]map[string]any
	seq          int
	clock        int
	requests     []grRequest
	deleteStatus int
}

func newPresetFake(t *testing.T) *presetFake {
	t.Helper()
	f := &presetFake{}
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

func (f *presetFake) reset() {
	f.presets = map[string]map[string]any{}
	f.system = map[string]map[string]any{
		presetSystemParent: {
			"id":            presetSystemParent,
			"name":          "Generic HTTPS empty",
			"entityType":    "ALERTS",
			"connectorType": "GENERIC_HTTPS",
			"presetType":    "SYSTEM",
			"configOverrides": []any{map[string]any{
				"conditionType": map[string]any{"matchEntityType": map[string]any{}},
				"payloadType":   "generic_https_empty",
				"messageConfig": map[string]any{"fields": []any{
					map[string]any{"fieldName": "body", "template": "{}"},
				}},
			}},
		},
	}
}

func (f *presetFake) seed(preset map[string]any) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	status, body := f.write(map[string]any{"preset": preset}, false)
	if status != http.StatusOK {
		panic(fmt.Sprintf("seed failed: %d %v", status, body))
	}
	return body.(map[string]any)["preset"].(map[string]any)["id"].(string)
}

func (f *presetFake) forget(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.presets, id)
}

func (f *presetFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

func (f *presetFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

func (f *presetFake) handle(method, path string, body any) (int, any) {
	_, rest, ok := strings.Cut(path, presetBasePath)
	if !ok {
		return grFail(http.StatusNotFound, "unknown path "+path)
	}
	switch {
	case method == http.MethodPost && rest == ":createCustom":
		return f.write(body, false)
	case method == http.MethodPut && rest == ":replaceCustom":
		return f.write(body, true)
	case method == http.MethodGet && strings.HasPrefix(rest, "/") && !strings.Contains(rest[1:], "/"):
		return f.get(strings.TrimPrefix(rest, "/"))
	case method == http.MethodDelete && strings.HasPrefix(rest, "/custom/"):
		return f.delete(strings.TrimPrefix(rest, "/custom/"))
	}
	return grFail(http.StatusMethodNotAllowed, "unsupported "+method+" "+path)
}

func (f *presetFake) get(id string) (int, any) {
	if stored, found := f.presets[id]; found {
		return http.StatusOK, map[string]any{"preset": f.present(stored)}
	}
	if stored, found := f.system[id]; found {
		return http.StatusOK, map[string]any{"preset": f.present(stored)}
	}
	return grFail(http.StatusNotFound, "Preset with id '"+id+"' not found")
}

func (f *presetFake) delete(id string) (int, any) {
	if f.deleteStatus != 0 {
		return grFail(f.deleteStatus, "Preset with id '"+id+"' not found")
	}
	if _, system := f.system[id]; system {
		return grFail(http.StatusNotFound, "Preset with id '"+id+"' not found")
	}
	if _, found := f.presets[id]; !found {
		return grFail(http.StatusNotFound, "Preset with id '"+id+"' not found")
	}
	delete(f.presets, id)
	return http.StatusOK, map[string]any{}
}

func (f *presetFake) uuidLike() string {
	f.seq++
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", f.seq)
}

func (f *presetFake) write(body any, replace bool) (int, any) {
	in, _ := body.(map[string]any)
	preset, _ := in["preset"].(map[string]any)
	if preset == nil {
		return grFail(http.StatusBadRequest, "preset is required")
	}
	if status, resp := f.check(preset, replace); status != 0 {
		return status, resp
	}
	parent := f.system[preset["parentId"].(string)]
	overrides, status, resp := normalizeOverrides(preset["configOverrides"], parent)
	if status != 0 {
		return status, resp
	}
	stored := map[string]any{
		"name":            preset["name"],
		"entityType":      preset["entityType"],
		"connectorType":   preset["connectorType"],
		"parentId":        preset["parentId"],
		"presetType":      "CUSTOM",
		"configOverrides": overrides,
	}
	if id, _ := preset["id"].(string); id != "" {
		stored["id"] = id
	} else if replace {
		return grFail(http.StatusBadRequest, "id is required")
	} else {
		stored["id"] = f.uuidLike()
	}
	if desc, _ := preset["description"].(string); desc != "" {
		stored["description"] = desc
	}
	if policy := attachmentPolicy(preset["attachmentConfig"]); policy != "" {
		stored["attachmentPolicy"] = policy
	}
	id := stored["id"].(string)
	if replace {
		if _, found := f.presets[id]; !found {
			return grFail(http.StatusNotFound, "Preset with id '"+id+"' not found")
		}
	} else if _, found := f.presets[id]; found {
		return grFail(http.StatusBadRequest, "Preset with id "+id+" already exists")
	}
	if !replace || f.presets[id]["name"] != stored["name"] {
		for other, existing := range f.presets {
			if other != id && existing["name"] == stored["name"] {
				return grFail(http.StatusBadRequest, "Preset with name "+stored["name"].(string)+" already exists")
			}
		}
	}
	f.clock++
	stored["updateTime"] = fmt.Sprintf("2026-01-01T00:00:%02dZ", f.clock)
	if prev, ok := f.presets[id]; ok {
		stored["createTime"] = prev["createTime"]
	} else {
		stored["createTime"] = stored["updateTime"]
	}
	f.presets[id] = stored
	return http.StatusOK, map[string]any{"preset": f.present(stored)}
}

func (f *presetFake) check(preset map[string]any, replace bool) (int, any) {
	name, _ := preset["name"].(string)
	if name == "" {
		return grFail(http.StatusBadRequest, "name is required")
	}
	connector, _ := preset["connectorType"].(string)
	if connector == "" || connector == "CONNECTOR_TYPE_UNSPECIFIED" {
		return grFail(http.StatusBadRequest, "connector_type is required")
	}
	entity, _ := preset["entityType"].(string)
	if entity == "" || entity == "ENTITY_TYPE_UNSPECIFIED" {
		return grFail(http.StatusBadRequest, "entity_type is required")
	}
	parentID, _ := preset["parentId"].(string)
	if parentID == "" {
		return grFail(http.StatusBadRequest, "Parent id is required for custom preset")
	}
	parent, found := f.system[parentID]
	if !found {
		return grFail(http.StatusBadRequest, "Parent preset not found")
	}
	if parent["entityType"] != entity || parent["connectorType"] != connector {
		return grFail(http.StatusBadRequest, "Parent preset entity type and connector type must match with child preset")
	}
	if replace {
		id, _ := preset["id"].(string)
		if id == "" {
			return grFail(http.StatusBadRequest, "id is required")
		}
		existing, found := f.presets[id]
		if !found {
			if _, system := f.system[id]; system {
				return grFail(http.StatusNotFound, "Preset with id '"+id+"' not found")
			}
			return grFail(http.StatusNotFound, "Preset with id '"+id+"' not found")
		}
		if existing["connectorType"] != connector {
			return grFail(http.StatusBadRequest, "Changing connector type is not allowed")
		}
		if existing["entityType"] != entity {
			return grFail(http.StatusBadRequest, "Changing entity type is not allowed")
		}
	}
	return 0, nil
}

func (f *presetFake) present(stored map[string]any) map[string]any {
	out := map[string]any{
		"id":            stored["id"],
		"name":          stored["name"],
		"entityType":    stored["entityType"],
		"connectorType": stored["connectorType"],
		"presetType":    stored["presetType"],
	}
	if parent, ok := stored["parentId"].(string); ok && parent != "" {
		out["parentId"] = parent
	}
	if desc, ok := stored["description"].(string); ok && desc != "" {
		out["description"] = desc
	}
	if policy, _ := stored["attachmentPolicy"].(string); policy != "" && policy != "AUTO" {
		out["attachmentConfig"] = map[string]any{"policy": policy}
	}
	if overrides, ok := stored["configOverrides"].([]any); ok && len(overrides) > 0 {
		out["configOverrides"] = overrides
	}
	if t, ok := stored["createTime"]; ok {
		out["createTime"] = t
		out["updateTime"] = stored["updateTime"]
	}
	return out
}

func attachmentPolicy(v any) string {
	cfg, _ := v.(map[string]any)
	if cfg == nil {
		return ""
	}
	policy, _ := cfg["policy"].(string)
	return policy
}

func parentPayload(parent map[string]any) string {
	overrides, _ := parent["configOverrides"].([]any)
	for _, item := range overrides {
		m, _ := item.(map[string]any)
		if payload, _ := m["payloadType"].(string); payload != "" {
			return payload
		}
	}
	return ""
}

func normalizeOverrides(in any, parent map[string]any) ([]any, int, any) {
	if in == nil {
		return nil, 0, nil
	}
	arr, ok := in.([]any)
	if !ok {
		status, body := grFail(http.StatusBadRequest, "configOverrides must be a list")
		return nil, status, body
	}
	payload := parentPayload(parent)
	out := make([]any, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok || m["conditionType"] == nil {
			status, body := grFail(http.StatusBadRequest, "condition_type is required for config override")
			return nil, status, body
		}
		mc, _ := m["messageConfig"].(map[string]any)
		if mc == nil {
			status, body := grFail(http.StatusBadRequest, "message_config is required for config override")
			return nil, status, body
		}
		fields, _ := mc["fields"].([]any)
		override := map[string]any{
			"conditionType": m["conditionType"],
			"messageConfig": map[string]any{"fields": normalizeMessageFields(fields)},
		}
		if got, _ := m["payloadType"].(string); got != "" {
			override["payloadType"] = got
		} else if payload == "" {
			status, body := grFail(http.StatusBadRequest, "Can't resolve output schema for entity subtype")
			return nil, status, body
		} else {
			override["payloadType"] = payload
		}
		out = append(out, override)
	}
	return out, 0, nil
}

func normalizeMessageFields(fields []any) []any {
	seen := map[string]bool{}
	out := make([]any, 0, len(fields))
	for _, field := range fields {
		m, _ := field.(map[string]any)
		if m == nil {
			continue
		}
		name, _ := m["fieldName"].(string)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, map[string]any{"fieldName": name, "template": m["template"]})
	}
	slices.SortFunc(out, func(a, b any) int {
		return strings.Compare(a.(map[string]any)["fieldName"].(string), b.(map[string]any)["fieldName"].(string))
	})
	return out
}
