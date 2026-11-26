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

// The fake webhooks API for the webhook protocol tests.
// It is not a baseline: the tests that use it assert in code, and no golden file is recorded.

package provider

import (
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const (
	webhookTypeName = "coralogix_webhook"
	webhookBasePath = "/integrations/webhooks/v1"
)

// webhookFake is an in-memory backend for outgoing webhooks. Like the real API, it returns every
// stored value on a read, including a URL or a token that a client sent as a secret.
type webhookFake struct {
	mu       sync.Mutex
	webhooks map[string]map[string]any
	seq      int
	requests []grRequest
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

func newWebhookHarness(t *testing.T) *grHarness {
	return newProtocolHarness(t, webhookTypeName, newWebhookFake(t))
}

func (f *webhookFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

// stored returns a copy of the stored webhook with this id.
func (f *webhookFake) stored(id string) map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return maps.Clone(f.webhooks[id])
}

func (f *webhookFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

func (f *webhookFake) handle(method, path string, body any) (int, any) {
	_, rest, ok := strings.Cut(path, webhookBasePath)
	if !ok {
		return grFail(http.StatusNotFound, "Not Found")
	}
	id := strings.TrimPrefix(rest, "/")
	in, _ := body.(map[string]any)
	switch {
	case method == http.MethodPost && rest == "":
		f.seq++
		id = "webhook-" + strconv.Itoa(f.seq)
		data, _ := in["data"].(map[string]any)
		f.webhooks[id] = f.record(id, f.seq, data)
		return http.StatusOK, map[string]any{"id": id}
	case method == http.MethodPut && rest == "":
		id, _ = in["id"].(string)
		old, found := f.webhooks[id]
		if !found {
			return grFail(http.StatusNotFound, "webhook not found")
		}
		data, _ := in["data"].(map[string]any)
		f.webhooks[id] = f.record(id, int(old["externalId"].(float64)), data)
		return http.StatusOK, map[string]any{}
	case method == http.MethodGet && id != "":
		if w, found := f.webhooks[id]; found {
			return http.StatusOK, map[string]any{"webhook": w}
		}
		return grFail(http.StatusNotFound, "webhook not found")
	case method == http.MethodDelete && id != "":
		if _, found := f.webhooks[id]; !found {
			return grFail(http.StatusNotFound, "webhook not found")
		}
		delete(f.webhooks, id)
		return http.StatusOK, map[string]any{}
	}
	return grFail(http.StatusMethodNotAllowed, "unsupported "+method+" "+path)
}

// seedLegacyTeams stores a deprecated Microsoft Teams webhook, as an older UI or provider made it.
func (f *webhookFake) seedLegacyTeams(id, name, teamsURL string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	f.webhooks[id] = f.record(id, f.seq, map[string]any{"name": name, "url": teamsURL, "microsoftTeams": map[string]any{}})
}

// record builds the stored webhook from the data of a request.
func (f *webhookFake) record(id string, externalID int, data map[string]any) map[string]any {
	out := maps.Clone(data)
	out["id"] = id
	out["externalId"] = float64(externalID)
	out["createdAt"] = "2026-01-01T00:00:00Z"
	out["updatedAt"] = "2026-01-01T00:00:00Z"
	if _, ok := out["type"]; !ok {
		out["type"] = "UNKNOWN_WEBHOOK_TYPE"
		// The API infers the type of the deprecated Microsoft Teams arm from the arm.
		if _, ok := out["microsoftTeams"]; ok {
			out["type"] = "MICROSOFT_TEAMS"
		}
	}
	// The API gives a Slack or Opsgenie webhook an empty config object at least.
	for _, arm := range []string{"opsgenie", "microsoftTeams", "msTeamsWorkflow"} {
		if _, ok := out[arm]; ok && out[arm] == nil {
			out[arm] = map[string]any{}
		}
	}
	return out
}
