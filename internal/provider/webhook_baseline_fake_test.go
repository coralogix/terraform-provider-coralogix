package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"sync"
	"testing"
)

const webhookBasePath = "/mgmt/openapi/5/integrations/webhooks/v1"

// webhookFake records the HTTP requests made by the webhook resource.
// Its replies model the create/get/update/delete contract documented for webhooks.
// The fake is deliberately small: the golden records provider behavior, not a
// claim that every backend validation rule is implemented here.
type webhookFake struct {
	mu           sync.Mutex
	webhooks     map[string]map[string]any
	requests     []grRequest
	seq          int
	createStatus int
	updateStatus int
	deleteStatus int
	getStatus    int
	getMessage   string
	listStatus   int
}

func newWebhookFake(t *testing.T) *webhookFake {
	t.Helper()
	f := &webhookFake{webhooks: map[string]map[string]any{}}
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

func (f *webhookFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

func (f *webhookFake) forget(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.webhooks, id)
}

func (f *webhookFake) setReadURL(id, value string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.webhooks[id]["url"] = value
}

func (f *webhookFake) seed(data map[string]any) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	id := fmt.Sprintf("00000000-0000-4000-8000-%012d", f.seq)
	f.webhooks[id] = f.present(data, id)
	return id
}

func (f *webhookFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	raw, _ := io.ReadAll(r.Body)
	rec := grRequest{Method: r.Method, Path: r.URL.Path}
	if len(raw) != 0 {
		_ = json.Unmarshal(raw, &rec.Body)
	}
	status, body := f.handle(r.Method, r.URL.Path, rec.Body)
	rec.Status = status
	f.requests = append(f.requests, rec)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (f *webhookFake) handle(method, path string, body any) (int, any) {
	if !strings.HasPrefix(path, webhookBasePath) {
		return grFail(http.StatusNotFound, "unknown path "+path)
	}
	id := strings.TrimPrefix(strings.TrimPrefix(path, webhookBasePath), "/")
	switch {
	case method == http.MethodPost && id == "":
		return f.create(body)
	case method == http.MethodPut && id == "":
		return f.update(body)
	case method == http.MethodGet && id == "":
		return f.list()
	case method == http.MethodGet && id != "":
		if f.getStatus != 0 {
			message := f.getMessage
			if message == "" {
				message = "Webhook with id " + id + " not found"
			}
			return grFail(f.getStatus, message)
		}
		stored, ok := f.webhooks[id]
		if !ok {
			return grFail(http.StatusNotFound, "Webhook with id "+id+" not found")
		}
		return http.StatusOK, map[string]any{"webhook": stored}
	case method == http.MethodDelete && id != "":
		if f.deleteStatus != 0 {
			return grFail(f.deleteStatus, "Webhook with id "+id+" not found")
		}
		delete(f.webhooks, id)
		return http.StatusOK, map[string]any{}
	}
	return grFail(http.StatusMethodNotAllowed, "unsupported "+method+" "+path)
}

func (f *webhookFake) list() (int, any) {
	if f.listStatus != 0 {
		return grFail(f.listStatus, "list rejected")
	}
	ids := make([]string, 0, len(f.webhooks))
	for id := range f.webhooks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	deployed := make([]any, 0, len(ids))
	for _, id := range ids {
		stored := f.webhooks[id]
		deployed = append(deployed, map[string]any{
			"id": id, "name": stored["name"], "type": stored["type"],
			"createdAt": stored["createdAt"], "updatedAt": stored["updatedAt"],
			"externalId": stored["externalId"],
		})
	}
	return http.StatusOK, map[string]any{"deployed": deployed}
}

func (f *webhookFake) create(body any) (int, any) {
	if f.createStatus != 0 {
		return grFail(f.createStatus, "create rejected")
	}
	data := webhookBodyData(body)
	if data == nil {
		return grFail(http.StatusBadRequest, "data is required")
	}
	if reason := webhookWriteError(data); reason != "" {
		return grFail(http.StatusBadRequest, reason)
	}
	f.seq++
	id := fmt.Sprintf("00000000-0000-4000-8000-%012d", f.seq)
	f.webhooks[id] = f.present(data, id)
	return http.StatusOK, map[string]any{"id": id}
}

func (f *webhookFake) update(body any) (int, any) {
	if f.updateStatus != 0 {
		return grFail(f.updateStatus, "update rejected")
	}
	in, _ := body.(map[string]any)
	id, _ := in["id"].(string)
	data := webhookBodyData(body)
	if id == "" || data == nil {
		return grFail(http.StatusBadRequest, "id and data are required")
	}
	if reason := webhookWriteError(data); reason != "" {
		return grFail(http.StatusBadRequest, reason)
	}
	old, ok := f.webhooks[id]
	if !ok {
		return grFail(http.StatusNotFound, "Webhook with id "+id+" not found")
	}
	if webhookArm(old) != webhookArm(data) {
		return grFail(http.StatusBadRequest, "Webhook type cannot change")
	}
	f.webhooks[id] = f.present(data, id)
	return http.StatusOK, map[string]any{}
}

func webhookBodyData(body any) map[string]any {
	in, _ := body.(map[string]any)
	data, _ := in["data"].(map[string]any)
	return data
}

func webhookWriteError(data map[string]any) string {
	if _, legacy := data["microsoftTeams"]; legacy {
		return "Microsoft Teams webhooks are deprecated"
	}
	if name, _ := data["name"].(string); name == "" {
		return "Webhook name is required"
	}
	switch webhookArm(data) {
	case "genericWebhook", "slack", "msTeamsWorkflow", "jira", "opsgenie", "demisto":
		if url, _ := data["url"].(string); url == "" {
			return "Webhook URL is required"
		}
	}
	if generic, ok := data["genericWebhook"].(map[string]any); ok {
		if method, _ := generic["method"].(string); method == "" || method == "UNKNOWN" {
			return "Unknown generic webhook method"
		}
		if payload, present := generic["payload"].(string); present && payload == "" {
			return "Generic webhook payload cannot be empty"
		}
	}
	if pagerDuty, ok := data["pagerDuty"].(map[string]any); ok {
		if key, _ := pagerDuty["serviceKey"].(string); key == "" {
			return "PagerDuty service key is required"
		}
	}
	if jira, ok := data["jira"].(map[string]any); ok {
		for _, field := range []string{"apiToken", "email", "projectKey"} {
			if value, _ := jira[field].(string); value == "" {
				return "Jira " + field + " is required"
			}
		}
	}
	return ""
}

func webhookArm(data map[string]any) string {
	for _, arm := range []string{"genericWebhook", "slack", "pagerDuty", "sendLog", "emailGroup", "msTeamsWorkflow", "microsoftTeams", "jira", "opsgenie", "demisto", "awsEventBridge"} {
		if _, ok := data[arm]; ok {
			return arm
		}
	}
	return ""
}

func (f *webhookFake) present(data map[string]any, id string) map[string]any {
	raw, _ := json.Marshal(data)
	var stored map[string]any
	_ = json.Unmarshal(raw, &stored)
	// The API keeps the first occurrence of each email address. Slack config
	// keeps one attachment and digest per type. Its read order is not defined.
	if email, ok := stored["emailGroup"].(map[string]any); ok {
		if addresses, ok := email["emailAddresses"].([]any); ok {
			seen := map[string]bool{}
			unique := make([]any, 0, len(addresses))
			for _, address := range addresses {
				key, _ := address.(string)
				if !seen[key] {
					unique = append(unique, address)
					seen[key] = true
				}
			}
			email["emailAddresses"] = unique
		}
	}
	if slack, ok := stored["slack"].(map[string]any); ok {
		for _, field := range []string{"attachments", "digests"} {
			if items, ok := slack[field].([]any); ok {
				byType := map[string]any{}
				for _, item := range items {
					object, _ := item.(map[string]any)
					kind, _ := object["type"].(string)
					byType[kind] = item
				}
				keys := make([]string, 0, len(byType))
				for key := range byType {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				unique := make([]any, 0, len(keys))
				for _, key := range keys {
					unique = append(unique, byType[key])
				}
				slack[field] = unique
			}
		}
	}
	stored["type"] = map[string]string{
		"genericWebhook": "GENERIC", "slack": "SLACK", "pagerDuty": "PAGERDUTY",
		"sendLog": "SEND_LOG", "emailGroup": "EMAIL_GROUP", "msTeamsWorkflow": "MS_TEAMS_WORKFLOW",
		"microsoftTeams": "MICROSOFT_TEAMS", "jira": "JIRA", "opsgenie": "OPSGENIE",
		"demisto": "DEMISTO", "awsEventBridge": "AWS_EVENT_BRIDGE",
	}[webhookArm(stored)]
	stored["id"] = id
	stored["externalId"] = float64(f.seq)
	stored["createdAt"] = "2026-01-01T00:00:00Z"
	stored["updatedAt"] = "2026-01-01T00:00:00Z"
	return stored
}
