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

package dataengine

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/tfsdk"

	retss "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/retentions_service"
)

func tfsdkConfig() tfsdk.Config { return tfsdk.Config{} }

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("dial tcp 127.0.0.1:1: connect: connection refused")
}

// retentionsHooks returns hooks whose client reads the tags from serve, or fails when serve is nil.
func retentionsHooks(t *testing.T, serve http.HandlerFunc) *archiveRetentionsHooks {
	t.Helper()
	cfg := retss.NewConfiguration()
	cfg.HTTPClient = &http.Client{Transport: failingTransport{}}
	cfg.Servers = retss.ServerConfigurations{{URL: "https://example.com"}}
	if serve != nil {
		srv := httptest.NewServer(serve)
		t.Cleanup(srv.Close)
		cfg.HTTPClient = srv.Client()
		cfg.Servers = retss.ServerConfigurations{{URL: srv.URL}}
	}
	return &archiveRetentionsHooks{client: retss.NewAPIClient(cfg).RetentionsServiceAPI}
}

func serveTags(ids ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tags := []map[string]any{}
		for i, id := range ids {
			tags = append(tags, map[string]any{"id": id, "order": i + 1, "name": "n" + id, "editable": i > 0})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"retentions": tags})
	}
}

func renameRequest(names ...string) *retss.UpdateRetentionsRequest {
	req := &retss.UpdateRetentionsRequest{}
	for _, name := range names {
		req.RetentionUpdateElements = append(req.RetentionUpdateElements, retss.RetentionUpdateElement{Name: name})
	}
	return req
}

// Create reads the tags and sends their ids by position, and the first name is always Default.
func TestBeforeWriteFillsTheIDsOfCreate(t *testing.T) {
	req := renameRequest("", "warm", "cold", "frozen")
	diags := retentionsHooks(t, serveTags("a", "b", "c", "d")).beforeWrite(context.Background(), tfsdkConfig(), req)
	if diags.HasError() {
		t.Fatal(diags)
	}
	for i, want := range []string{"a", "b", "c", "d"} {
		if got := req.RetentionUpdateElements[i].Id; got != want {
			t.Errorf("item %d id = %q, want %q", i, got, want)
		}
	}
	if got := req.RetentionUpdateElements[0].Name; got != "Default" {
		t.Errorf("first name = %q, want Default", got)
	}
}

// Update has the ids of the state, so it does not read the tags.
func TestBeforeWriteKeepsTheIDsOfUpdate(t *testing.T) {
	req := renameRequest("", "warm")
	req.RetentionUpdateElements[0].Id, req.RetentionUpdateElements[1].Id = "a", "b"
	diags := retentionsHooks(t, nil).beforeWrite(context.Background(), tfsdkConfig(), req)
	if diags.HasError() {
		t.Fatalf("Update read the tags: %v", diags)
	}
	if req.RetentionUpdateElements[1].Id != "b" || req.RetentionUpdateElements[0].Name != "Default" {
		t.Fatalf("request = %+v, want the state ids and Default first", req.RetentionUpdateElements)
	}
}

// A read that fails, or that returns fewer tags than the configuration, stops Create before the write.
func TestBeforeWriteStopsCreate(t *testing.T) {
	for name, test := range map[string]struct {
		serve http.HandlerFunc
		want  string
	}{
		"transport failure":         {serve: nil, want: "connection refused"},
		"fewer tags than the items": {serve: serveTags("a", "b"), want: "returned 2 archive retentions, but the configuration defines 4"},
		"no tags":                   {serve: serveTags(), want: "returned 0 archive retentions"},
	} {
		t.Run(name, func(t *testing.T) {
			req := renameRequest("", "warm", "cold", "frozen")
			diags := retentionsHooks(t, test.serve).beforeWrite(context.Background(), tfsdkConfig(), req)
			if !diags.HasError() {
				t.Fatal("beforeWrite succeeded, want an error")
			}
			var text strings.Builder
			for _, d := range diags {
				text.WriteString(d.Summary() + " " + d.Detail())
			}
			if !strings.Contains(text.String(), test.want) {
				t.Fatalf("diagnostics = %q, want %q", text.String(), test.want)
			}
		})
	}
}

// A name that the configuration leaves out arrives as "", which the API rejects; it keeps the
// current name of the tag, also on Update.
func TestBeforeWriteKeepsALeftOutName(t *testing.T) {
	req := renameRequest("", "warm", "", "frozen")
	for i, id := range []string{"a", "b", "c", "d"} {
		req.RetentionUpdateElements[i].Id = id
	}
	diags := retentionsHooks(t, serveTags("a", "b", "c", "d")).beforeWrite(context.Background(), tfsdkConfig(), req)
	if diags.HasError() {
		t.Fatal(diags)
	}
	for i, want := range []string{"Default", "warm", "nc", "frozen"} {
		if got := req.RetentionUpdateElements[i].Name; got != want {
			t.Errorf("item %d name = %q, want %q", i, got, want)
		}
	}
}
