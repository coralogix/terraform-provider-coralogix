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

package aaa

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	apiKeys "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/api_keys_service"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestSetAccessPolicyOnUpdate covers the Update request-building logic for the
// access_policy attribute: it must send the new value when set, send an empty
// string to clear a previously-set value, and be omitted (left nil) when the
// value did not change or is unknown.
func TestSetAccessPolicyOnUpdate(t *testing.T) {
	const policyA = `{"version":"v2025-01-01","rules":[],"default":{}}`
	const policyB = `{"version":"v2025-01-01","rules":[{"name":"r"}],"default":{}}`

	emptyStr := ""

	tests := []struct {
		name    string
		current types.String
		desired types.String
		// want is the expected rq.AccessPolicy: nil means "not sent".
		want *string
	}{
		{
			name:    "unchanged_both_null",
			current: types.StringNull(),
			desired: types.StringNull(),
			want:    nil,
		},
		{
			name:    "unchanged_same_value",
			current: types.StringValue(policyA),
			desired: types.StringValue(policyA),
			want:    nil,
		},
		{
			name:    "set_from_null",
			current: types.StringNull(),
			desired: types.StringValue(policyA),
			want:    strPtr(policyA),
		},
		{
			name:    "changed_value",
			current: types.StringValue(policyA),
			desired: types.StringValue(policyB),
			want:    strPtr(policyB),
		},
		{
			name:    "cleared_to_empty_string_sends_empty_string",
			current: types.StringValue(policyA),
			desired: types.StringValue(""),
			want:    &emptyStr,
		},
		{
			name:    "unknown_desired_is_omitted",
			current: types.StringValue(policyA),
			desired: types.StringUnknown(),
			want:    nil,
		},
		{
			name:    "unknown_desired_from_null_is_omitted",
			current: types.StringNull(),
			desired: types.StringUnknown(),
			want:    nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			current := &ApiKeyModel{AccessPolicy: tc.current}
			desired := &ApiKeyModel{AccessPolicy: tc.desired}
			rq := apiKeys.UpdateApiKeyRequest{}

			setAccessPolicyOnUpdate(current, desired, &rq)

			switch {
			case tc.want == nil && rq.AccessPolicy != nil:
				t.Fatalf("expected AccessPolicy to be omitted (nil), got %q", *rq.AccessPolicy)
			case tc.want != nil && rq.AccessPolicy == nil:
				t.Fatalf("expected AccessPolicy %q, got nil (omitted)", *tc.want)
			case tc.want != nil && rq.AccessPolicy != nil && *tc.want != *rq.AccessPolicy:
				t.Fatalf("expected AccessPolicy %q, got %q", *tc.want, *rq.AccessPolicy)
			}
		})
	}
}

// TestIsNotFoundResponse covers the guard used by Read and Update to decide
// whether an error represents a confirmed 404 (in which case the resource is
// removed from state / recreated). A nil *http.Response must never be treated
// as a 404, and non-404 statuses must not trigger removal.
func TestIsNotFoundResponse(t *testing.T) {
	tests := []struct {
		name string
		resp *http.Response
		want bool
	}{
		{
			name: "nil_response_is_not_404",
			resp: nil,
			want: false,
		},
		{
			name: "confirmed_404",
			resp: &http.Response{StatusCode: http.StatusNotFound},
			want: true,
		},
		{
			name: "500_is_not_404",
			resp: &http.Response{StatusCode: http.StatusInternalServerError},
			want: false,
		},
		{
			name: "401_is_not_404",
			resp: &http.Response{StatusCode: http.StatusUnauthorized},
			want: false,
		},
		{
			name: "200_is_not_404",
			resp: &http.Response{StatusCode: http.StatusOK},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isNotFoundResponse(tc.resp); got != tc.want {
				t.Fatalf("isNotFoundResponse() = %v, want %v", got, tc.want)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}

// TestApiKeyResourceRead drives the framework Read method against an httptest
// backend to verify state-reconciliation behaviour by HTTP status:
//   - 404 Not Found: Read must emit a warning and remove the resource from
//     state, so a silently-recreated key is surfaced to the user.
//   - 500 Internal Server Error (transient): Read must NOT warn and must NOT
//     remove the resource; the error propagates and state is preserved.
func TestApiKeyResourceRead(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name             string
		status           int
		wantWarnings     int
		wantErrors       bool
		wantResourceGone bool
	}{
		{
			name:             "404 emits warning and removes resource",
			status:           http.StatusNotFound,
			wantWarnings:     1,
			wantErrors:       false,
			wantResourceGone: true,
		},
		{
			name:             "500 keeps resource with no warning",
			status:           http.StatusInternalServerError,
			wantWarnings:     0,
			wantErrors:       true,
			wantResourceGone: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"message":"boom"}`))
			}))
			defer srv.Close()

			cfg := apiKeys.NewConfiguration()
			cfg.Servers = apiKeys.ServerConfigurations{{URL: srv.URL}}
			cfg.HTTPClient = srv.Client()
			client := apiKeys.NewAPIClient(cfg).APIKeysServiceAPI

			r := &ApiKeyResource{client: client}

			// Build a fully-populated state from the resource schema.
			schemaResp := &resource.SchemaResponse{}
			r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

			state := tfsdk.State{Schema: schemaResp.Schema}
			model := &ApiKeyModel{
				ID:           types.StringValue("key-123"),
				Name:         types.StringValue("example"),
				Owner:        nil,
				Active:       types.BoolValue(true),
				Hashed:       types.BoolValue(false),
				Permissions:  types.SetValueMust(types.StringType, []attr.Value{}),
				Presets:      types.SetValueMust(types.StringType, []attr.Value{}),
				Value:        types.StringNull(),
				AccessPolicy: types.StringNull(),
			}
			if diags := state.Set(ctx, model); diags.HasError() {
				t.Fatalf("failed to seed state: %v", diags)
			}

			req := resource.ReadRequest{State: state}
			resp := &resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
			// Seed the response state with the current value so a no-op Read
			// leaves the resource present.
			if diags := resp.State.Set(ctx, model); diags.HasError() {
				t.Fatalf("failed to seed response state: %v", diags)
			}

			r.Read(ctx, req, resp)

			if got := resp.Diagnostics.WarningsCount(); got != tc.wantWarnings {
				t.Errorf("warnings = %d, want %d (%v)", got, tc.wantWarnings, resp.Diagnostics.Warnings())
			}
			if got := resp.Diagnostics.HasError(); got != tc.wantErrors {
				t.Errorf("hasError = %v, want %v (%v)", got, tc.wantErrors, resp.Diagnostics.Errors())
			}
			// RemoveResource sets the response state to a null object.
			gone := resp.State.Raw.IsNull()
			if gone != tc.wantResourceGone {
				t.Errorf("resourceGone = %v, want %v", gone, tc.wantResourceGone)
			}
		})
	}
}

func TestFlattenAccessPolicy(t *testing.T) {
	configured := `{ "version": "2025-01-01", "default": { "permissions": { "team-custom-api-keys:ReadConfig": "grant" } }, "rules": [] }`
	compact := `{"version":"2025-01-01","default":{"permissions":{"team-custom-api-keys:ReadConfig":"grant"}},"rules":[]}`
	different := `{"version":"2025-01-01","default":{"permissions":{"team-custom-api-keys:ReadConfig":"deny"}},"rules":[]}`

	t.Run("preserves_configured_text_when_json_equivalent", func(t *testing.T) {
		got := flattenAccessPolicy(types.StringValue(configured), &compact)
		if got.ValueString() != configured {
			t.Fatalf("got %q, want configured text", got.ValueString())
		}
	})
	t.Run("empty_api_policy_is_empty_string", func(t *testing.T) {
		got := flattenAccessPolicy(types.StringNull(), nil)
		if got.ValueString() != "" {
			t.Fatalf("got %q, want empty string", got.ValueString())
		}
	})
	t.Run("keeps_empty_configured_clear", func(t *testing.T) {
		got := flattenAccessPolicy(types.StringValue(""), nil)
		if got.ValueString() != "" {
			t.Fatalf("got %q, want empty string", got.ValueString())
		}
	})
	t.Run("stores_new_policy_when_not_equivalent", func(t *testing.T) {
		got := flattenAccessPolicy(types.StringValue(configured), &different)
		if got.ValueString() == configured {
			t.Fatal("kept configured text for a different policy")
		}
	})
}
