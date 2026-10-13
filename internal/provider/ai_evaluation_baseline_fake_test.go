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
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
)

const aiEvalBasePath = "/ai/evaluations/v3"

// The AI application that the fake knows. The API cannot create one: it
// appears when the tenant sends AI spans.
const (
	aiEvalFakeApplication = "tf-baseline-app"
	aiEvalFakeSubsystem   = "tf-baseline-subsystem"
)

var aiEvalUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// aiEvalConfigLists are the config variants that hold a list, and its name.
var aiEvalConfigLists = map[string]string{
	"allowedTopics":       "topics",
	"restrictedTopics":    "topics",
	"competition":         "competitors",
	"pii":                 "categories",
	"sqlAllowedTables":    "tables",
	"sqlRestrictedTables": "tables",
}

// aiEvalFake follows ws-ai-evaluation-configurations behind the REST facade,
// not the Go client. Create checks the request, then the AI application. A
// second evaluation of the same application, subsystem, type, and target
// conflicts. Update without updateMask changes only the fields in the body;
// a config in the body replaces the stored one, and a variant without fields
// stores {}. A missing prompt-injection context is "". Lists keep the order
// that was sent. Get validates the id as a UUID before it looks it up.
type aiEvalFake struct {
	mu           sync.Mutex
	evaluations  map[string]map[string]any
	applications map[string]bool
	seq          int
	clock        int
	requests     []grRequest
	deleteStatus int
}

func newAIEvalFake(t *testing.T) *aiEvalFake {
	t.Helper()
	f := &aiEvalFake{}
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

func (f *aiEvalFake) reset() {
	f.evaluations = map[string]map[string]any{}
	f.applications = map[string]bool{aiEvalFakeApplication + "/" + aiEvalFakeSubsystem: true}
}

// seed stores an evaluation as if it was made outside Terraform. It returns the id.
func (f *aiEvalFake) seed(body map[string]any) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	status, resp := f.create(body)
	if status != http.StatusOK {
		panic(fmt.Sprintf("seed failed: %d %v", status, resp))
	}
	return resp.(map[string]any)["aiEvaluation"].(map[string]any)["id"].(string)
}

func (f *aiEvalFake) forget(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.evaluations, id)
}

func (f *aiEvalFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

func (f *aiEvalFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

func aiEvalFail(status int, message string) (int, any) {
	prefix := map[int]string{
		http.StatusBadRequest: "Bad Request", http.StatusNotFound: "Not Found", http.StatusConflict: "Already Exists",
	}[status]
	return status, map[string]any{"code": status, "message": prefix + ": " + message}
}

func (f *aiEvalFake) handle(method, path string, body any) (int, any) {
	_, rest, ok := strings.Cut(path, aiEvalBasePath)
	if !ok {
		return aiEvalFail(http.StatusNotFound, "unknown path "+path)
	}
	id := strings.TrimPrefix(rest, "/")
	in, _ := body.(map[string]any)
	switch {
	case method == http.MethodPost && rest == "":
		return f.create(in)
	case method == http.MethodGet && id != "":
		return f.get(id)
	case method == http.MethodPatch && id != "":
		return f.update(id, in)
	case method == http.MethodDelete && id != "":
		return f.delete(id)
	}
	return aiEvalFail(http.StatusMethodNotAllowed, "unsupported "+method+" "+path)
}

func (f *aiEvalFake) find(id string) (map[string]any, int, any) {
	if !aiEvalUUID.MatchString(id) {
		status, resp := aiEvalZodFail([]zodIssue{zodUUID()})
		return nil, status, resp
	}
	stored, found := f.evaluations[id]
	if !found {
		status, resp := aiEvalFail(http.StatusNotFound, "Evaluation not found")
		return nil, status, resp
	}
	return stored, 0, nil
}

func (f *aiEvalFake) get(id string) (int, any) {
	stored, status, resp := f.find(id)
	if stored == nil {
		return status, resp
	}
	return http.StatusOK, map[string]any{"aiEvaluation": present(stored)}
}

func (f *aiEvalFake) delete(id string) (int, any) {
	if f.deleteStatus != 0 {
		return aiEvalFail(f.deleteStatus, "Evaluation not found")
	}
	stored, status, resp := f.find(id)
	if stored == nil {
		return status, resp
	}
	delete(f.evaluations, id)
	return http.StatusOK, map[string]any{"id": id}
}

func (f *aiEvalFake) create(in map[string]any) (int, any) {
	if status, resp := aiEvalProtoCheck(in); status != 0 {
		return status, resp
	}
	if issues := aiEvalCheckCreate(in); len(issues) != 0 {
		return aiEvalZodFail(issues)
	}
	application, subsystem := in["application"].(string), in["subsystem"].(string)
	if !f.applications[application+"/"+subsystem] {
		return aiEvalFail(http.StatusNotFound, "AI application not found")
	}
	config := aiEvalNormalizeConfig(in["config"].(map[string]any))
	target := in["target"].(string)
	for _, other := range f.evaluations {
		if other["application"] == application && other["subsystem"] == subsystem &&
			other["target"] == target && aiEvalVariant(other["config"]) == aiEvalVariant(config) {
			return aiEvalFail(http.StatusConflict, "An evaluation with these parameters already exists")
		}
	}
	f.seq++
	f.clock++
	now := fmt.Sprintf("2026-01-01T00:00:%02dZ", f.clock)
	stored := map[string]any{
		"id":          fmt.Sprintf("00000000-0000-4000-8000-%012d", f.seq),
		"companyId":   "1",
		"application": application,
		"subsystem":   subsystem,
		"target":      target,
		"config":      config,
		"isEnabled":   in["isEnabled"] == true,
		"createdBy":   "00000000-0000-4000-8000-0000000000aa",
		"createdAt":   now,
		"updatedAt":   now,
	}
	if threshold, ok := in["threshold"]; ok {
		stored["threshold"] = threshold
	}
	f.evaluations[stored["id"].(string)] = stored
	return http.StatusOK, map[string]any{"aiEvaluation": present(stored)}
}

func (f *aiEvalFake) update(id string, in map[string]any) (int, any) {
	if status, resp := aiEvalProtoCheck(in); status != 0 {
		return status, resp
	}
	if issues := aiEvalCheckUpdate(id, in); len(issues) != 0 {
		return aiEvalZodFail(issues)
	}
	stored, status, resp := f.find(id)
	if stored == nil {
		return status, resp
	}
	application, _ := in["application"].(string)
	subsystem, _ := in["subsystem"].(string)
	if application != "" || subsystem != "" {
		if application == "" {
			application = stored["application"].(string)
		}
		if subsystem == "" {
			subsystem = stored["subsystem"].(string)
		}
		if !f.applications[application+"/"+subsystem] {
			return aiEvalFail(http.StatusNotFound, "AI application not found")
		}
		stored["application"], stored["subsystem"] = application, subsystem
	}
	for _, k := range []string{"target", "isEnabled", "threshold"} {
		if v, ok := in[k]; ok {
			stored[k] = v
		}
	}
	if config, ok := in["config"].(map[string]any); ok {
		stored["config"] = aiEvalNormalizeConfig(config)
	}
	f.clock++
	stored["updatedAt"] = fmt.Sprintf("2026-01-01T00:00:%02dZ", f.clock)
	return http.StatusOK, map[string]any{"aiEvaluation": present(stored)}
}

var aiEvalMaskPaths = map[string]bool{
	"application": true, "subsystem": true, "target": true, "config": true, "isEnabled": true, "threshold": true,
}

// zodIssue is one issue of a zod error, with its keys in the order zod writes them.
type zodIssue [][2]any

func zodPath(path ...any) [2]any { return [2]any{"path", path} }

func zodRequired(expected string, path ...any) zodIssue {
	if strings.HasPrefix(expected, "'") {
		// A native enum writes expected and received before code.
		return zodIssue{{"expected", expected}, {"received", "undefined"}, {"code", "invalid_type"}, zodPath(path...), {"message", "Required"}}
	}
	return zodIssue{{"code", "invalid_type"}, {"expected", expected}, {"received", "undefined"}, zodPath(path...), {"message", "Required"}}
}

func zodTooSmall(kind string, message string, path ...any) zodIssue {
	minimum := 1
	if kind == "number" {
		minimum = 0
	}
	return zodIssue{{"code", "too_small"}, {"minimum", minimum}, {"type", kind}, {"inclusive", true}, {"exact", false}, {"message", message}, zodPath(path...)}
}

func zodTooBig(path ...any) zodIssue {
	return zodIssue{{"code", "too_big"}, {"maximum", 1}, {"type", "number"}, {"inclusive", true}, {"exact", false}, {"message", "Number must be less than or equal to 1"}, zodPath(path...)}
}

func zodCustom(message string, path ...any) zodIssue {
	return zodIssue{{"code", "custom"}, {"message", message}, zodPath(path...)}
}

func zodUUID() zodIssue {
	return zodIssue{{"validation", "uuid"}, {"code", "invalid_string"}, {"message", "Invalid uuid"}, zodPath("id")}
}

// zodMessage is the text of a zod error: the issues as JSON.stringify(issues, null, 2).
func zodMessage(issues []zodIssue) string {
	var b strings.Builder
	var write func(v any, indent string)
	write = func(v any, indent string) {
		switch x := v.(type) {
		case zodIssue:
			b.WriteString("{")
			for i, kv := range x {
				if i > 0 {
					b.WriteString(",")
				}
				raw, _ := json.Marshal(kv[0])
				b.WriteString("\n" + indent + "  " + string(raw) + ": ")
				write(kv[1], indent+"  ")
			}
			b.WriteString("\n" + indent + "}")
		case []any:
			b.WriteString("[")
			for i, e := range x {
				if i > 0 {
					b.WriteString(",")
				}
				b.WriteString("\n" + indent + "  ")
				write(e, indent+"  ")
			}
			b.WriteString("\n" + indent + "]")
		default:
			raw, _ := json.Marshal(x)
			b.WriteString(string(raw))
		}
	}
	all := make([]any, len(issues))
	for i, issue := range issues {
		all[i] = issue
	}
	write(all, "")
	return b.String()
}

func aiEvalZodFail(issues []zodIssue) (int, any) {
	return aiEvalFail(http.StatusBadRequest, zodMessage(issues))
}

// aiEvalProtoCheck is what the REST facade's JSON decoder refuses before the
// service sees the request: a config with two variants. The SDK writes keys in
// sorted order, and the decoder names the second one.
func aiEvalProtoCheck(in map[string]any) (int, any) {
	config, ok := in["config"].(map[string]any)
	if !ok || len(config) < 2 {
		return 0, nil
	}
	keys := strings.Split(aiEvalVariant(config), ",")
	return aiEvalFail(http.StatusBadRequest, fmt.Sprintf("proto: (line 1:1): error parsing %q, oneof com.coralogixapis.ai.v3.EvaluationConfig.evaluation is already set", keys[1]))
}

// aiEvalCheckCreate follows aiEvaluationCreateSchema: target, config,
// application, subsystem, isEnabled, threshold.
func aiEvalCheckCreate(in map[string]any) []zodIssue {
	var issues []zodIssue
	if _, ok := in["target"].(string); !ok {
		issues = append(issues, zodRequired("'PROMPT' | 'RESPONSE' | 'CONVERSATION'", "target"))
	}
	if config, ok := in["config"].(map[string]any); ok {
		issues = append(issues, aiEvalCheckConfig(config)...)
	} else {
		issues = append(issues, zodRequired("object", "config"))
	}
	for _, k := range []string{"application", "subsystem"} {
		if s, _ := in[k].(string); s == "" {
			issues = append(issues, zodTooSmall("string", "String must contain at least 1 character(s)", k))
		}
	}
	return append(issues, aiEvalCheckThreshold(in)...)
}

func aiEvalCheckThreshold(in map[string]any) []zodIssue {
	t, ok := in["threshold"].(float64)
	switch {
	case !ok:
		return nil
	case t < 0:
		return []zodIssue{zodTooSmall("number", "Number must be greater than or equal to 0", "threshold")}
	case t > 1:
		return []zodIssue{zodTooBig("threshold")}
	}
	return nil
}

func aiEvalCheckConfig(config map[string]any) []zodIssue {
	if len(config) == 0 {
		return []zodIssue{zodRequired("object", "config", "evaluation")}
	}
	variant := aiEvalVariant(config)
	list, ok := aiEvalConfigLists[variant]
	if !ok {
		return nil
	}
	value, _ := config[variant].(map[string]any)
	items, _ := value[list].([]any)
	path := []any{"config", "evaluation", "value", list}
	if len(items) == 0 {
		return []zodIssue{zodTooSmall("array", "Array must contain at least 1 element(s)", path...)}
	}
	seen := map[string]bool{}
	for _, item := range items {
		key := fmt.Sprint(item)
		if seen[key] {
			return []zodIssue{zodCustom("Items in array must be unique, no duplicates allowed.", path...)}
		}
		seen[key] = true
	}
	return nil
}

// aiEvalCheckUpdate follows aiEvaluationUpdateSchema: id, target, config,
// application, subsystem, isEnabled, threshold, updateMask. The mask checks
// run only when the rest is valid.
func aiEvalCheckUpdate(id string, in map[string]any) []zodIssue {
	var issues []zodIssue
	if !aiEvalUUID.MatchString(id) {
		issues = append(issues, zodUUID())
	}
	if config, ok := in["config"].(map[string]any); ok {
		issues = append(issues, aiEvalCheckConfig(config)...)
	}
	for _, k := range []string{"application", "subsystem"} {
		if s, ok := in[k].(string); ok && s == "" {
			issues = append(issues, zodTooSmall("string", "String must contain at least 1 character(s)", k))
		}
	}
	issues = append(issues, aiEvalCheckThreshold(in)...)
	if len(issues) != 0 {
		return issues
	}
	mask, _ := in["updateMask"].(string)
	if mask == "" {
		return nil
	}
	for _, path := range strings.Split(mask, ",") {
		if !aiEvalMaskPaths[path] {
			issues = append(issues, zodCustom(fmt.Sprintf("Unknown field path %q in update_mask.", path), "updateMask"))
		}
	}
	if len(issues) != 0 {
		return issues
	}
	for _, path := range strings.Split(mask, ",") {
		if _, ok := in[path]; !ok && path != "threshold" {
			issues = append(issues, zodCustom(fmt.Sprintf("Field %q is required and cannot be cleared via update_mask.", path), "updateMask"))
		}
	}
	return issues
}

func aiEvalVariant(config any) string {
	m, _ := config.(map[string]any)
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

// aiEvalNormalizeConfig stores the variant as the service does: a variant
// without fields is {}, and prompt injection always has additionalContext.
func aiEvalNormalizeConfig(config map[string]any) map[string]any {
	variant := aiEvalVariant(config)
	value, _ := config[variant].(map[string]any)
	stored := map[string]any{}
	for k, v := range value {
		stored[k] = v
	}
	if variant == "promptInjection" {
		if _, ok := stored["additionalContext"]; !ok {
			stored["additionalContext"] = ""
		}
	}
	return map[string]any{variant: stored}
}

// present writes a stored evaluation as the REST facade does: zero values such
// as false and "" are written, and a threshold that was never set is left out.
func present(stored map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range stored {
		out[k] = v
	}
	return out
}
