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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	apiKeys "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/api_keys_service"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const apiKeyTestID = "3f5b7c9a-1d2e-4f6a-8b0c-9d1e2f3a4b5c"

// apiKeyResourceForServer builds an ApiKeyResource whose SDK client is pointed at
// the given base URL (an httptest server), so requests are exercised end to end.
func apiKeyResourceForServer(baseURL string) *ApiKeyResource {
	cfg := apiKeys.NewConfiguration()
	cfg.Servers = apiKeys.ServerConfigurations{{URL: baseURL}}
	cfg.HTTPClient = &http.Client{}
	return &ApiKeyResource{client: apiKeys.NewAPIClient(cfg).APIKeysServiceAPI}
}

// apiKeyTestSchema returns the resource schema used to shape plan/state values.
func apiKeyTestSchema(ctx context.Context) schema.Schema {
	response := frameworkresource.SchemaResponse{}
	(&ApiKeyResource{}).Schema(ctx, frameworkresource.SchemaRequest{}, &response)
	if response.Diagnostics.HasError() {
		panic("coralogix_api_key schema could not be built")
	}
	return response.Schema
}

// apiKeyAttributes builds the base attribute map for a fully-null object of the
// resource type, then seeds the always-present fields. Callers override the
// attributes relevant to their scenario.
func apiKeyAttributes(ctx context.Context, resourceSchema schema.Schema) (tftypes.Type, tftypes.Object, map[string]tftypes.Value) {
	terraformType := resourceSchema.Type().TerraformType(ctx)
	objectType, ok := terraformType.(tftypes.Object)
	if !ok {
		panic("coralogix_api_key schema Terraform type is not an object")
	}
	attributes := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for name, attributeType := range objectType.AttributeTypes {
		attributes[name] = tftypes.NewValue(attributeType, nil)
	}
	attributes["id"] = tftypes.NewValue(objectType.AttributeTypes["id"], apiKeyTestID)
	attributes["hashed"] = tftypes.NewValue(objectType.AttributeTypes["hashed"], false)
	attributes["active"] = tftypes.NewValue(objectType.AttributeTypes["active"], true)
	return terraformType, objectType, attributes
}

// apiKeyStringAttr sets a string attribute (nil for null).
func apiKeyStringAttr(objectType tftypes.Object, attributes map[string]tftypes.Value, name string, value *string) {
	if value == nil {
		attributes[name] = tftypes.NewValue(objectType.AttributeTypes[name], nil)
		return
	}
	attributes[name] = tftypes.NewValue(objectType.AttributeTypes[name], *value)
}

func strptr(s string) *string { return &s }

// getApiKeyResponseBody returns a valid GetApiKeyResponse JSON payload so the
// post-update Read inside Update() succeeds.
func getApiKeyResponseBody(name, accessPolicy string) string {
	body := map[string]any{
		"keyInfo": map[string]any{
			"id":     apiKeyTestID,
			"name":   name,
			"active": true,
			"hashed": false,
			"value":  "the-secret-value",
			"keyPermissions": map[string]any{
				"permissions": []string{},
				"presets":     []any{},
			},
			"owner": map[string]any{
				"userId": "user-1",
			},
			"accessPolicy": accessPolicy,
		},
	}
	raw, _ := json.Marshal(body)
	return string(raw)
}

// TestApiKeyResourceUpdate exercises Update() against an httptest mock server and
// asserts the JSON body captured on the PUT request for three plan shapes:
// setting a new access_policy, clearing an existing access_policy, and a
// name-only change (which must not touch access_policy).
func TestApiKeyResourceUpdate(t *testing.T) {
	ctx := context.Background()
	resourceSchema := apiKeyTestSchema(ctx)

	type scenario struct {
		name               string
		stateName          string
		stateAccessPolicy  *string
		planName           string
		planAccessPolicy   *string
		wantNewName        *string
		wantAccessPolicySet bool
		wantAccessPolicy   string
	}

	scenarios := []scenario{
		{
			name:                "new_access_policy",
			stateName:           "key-one",
			stateAccessPolicy:   nil,
			planName:            "key-one",
			planAccessPolicy:    strptr("policy-new"),
			wantNewName:         nil,
			wantAccessPolicySet: true,
			wantAccessPolicy:    "policy-new",
		},
		{
			name:                "cleared_access_policy",
			stateName:           "key-one",
			stateAccessPolicy:   strptr("policy-old"),
			planName:            "key-one",
			planAccessPolicy:    strptr(""),
			wantNewName:         nil,
			wantAccessPolicySet: true,
			wantAccessPolicy:    "", // cleared -> empty string sent to backend
		},
		{
			name:                "name_only_update",
			stateName:           "old-name",
			stateAccessPolicy:   strptr("policy-keep"),
			planName:            "new-name",
			planAccessPolicy:    strptr("policy-keep"),
			wantNewName:         strptr("new-name"),
			wantAccessPolicySet: false,
		},
	}

	for _, tc := range scenarios {
		t.Run(tc.name, func(t *testing.T) {
			var captured apiKeys.UpdateApiKeyRequest
			var updateCalled bool

			mux := http.NewServeMux()
			mux.HandleFunc("/aaa/api-keys/v3/"+apiKeyTestID, func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodPut:
					updateCalled = true
					raw, err := io.ReadAll(r.Body)
					if err != nil {
						t.Fatalf("reading update body: %v", err)
					}
					if err := json.Unmarshal(raw, &captured); err != nil {
						t.Fatalf("unmarshalling update body %q: %v", string(raw), err)
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{}`))
				case http.MethodGet:
					policy := tc.wantAccessPolicy
					if !tc.wantAccessPolicySet {
						policy = "policy-keep"
					}
					name := tc.planName
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(getApiKeyResponseBody(name, policy)))
				default:
					t.Fatalf("unexpected method %q", r.Method)
				}
			})
			server := httptest.NewServer(mux)
			defer server.Close()

			r := apiKeyResourceForServer(server.URL)

			// Build state value.
			terraformType, objectType, stateAttrs := apiKeyAttributes(ctx, resourceSchema)
			apiKeyStringAttr(objectType, stateAttrs, "name", strptr(tc.stateName))
			apiKeyStringAttr(objectType, stateAttrs, "access_policy", tc.stateAccessPolicy)
			apiKeyStringAttr(objectType, stateAttrs, "value", strptr("the-secret-value"))
			state := tfsdk.State{Raw: tftypes.NewValue(terraformType, stateAttrs), Schema: resourceSchema}

			// Build plan value.
			_, _, planAttrs := apiKeyAttributes(ctx, resourceSchema)
			apiKeyStringAttr(objectType, planAttrs, "name", strptr(tc.planName))
			apiKeyStringAttr(objectType, planAttrs, "access_policy", tc.planAccessPolicy)
			apiKeyStringAttr(objectType, planAttrs, "value", strptr("the-secret-value"))
			plan := tfsdk.Plan{Raw: tftypes.NewValue(terraformType, planAttrs), Schema: resourceSchema}

			response := frameworkresource.UpdateResponse{State: state}
			r.Update(ctx, frameworkresource.UpdateRequest{Plan: plan, State: state}, &response)

			if response.Diagnostics.HasError() {
				t.Fatalf("Update() diagnostics = %v, want none", response.Diagnostics)
			}
			if !updateCalled {
				t.Fatal("Update() did not call the backend PUT endpoint")
			}

			// Assert NewName.
			if tc.wantNewName == nil {
				if captured.NewName != nil {
					t.Fatalf("captured NewName = %q, want unset", *captured.NewName)
				}
			} else {
				if captured.NewName == nil {
					t.Fatalf("captured NewName = unset, want %q", *tc.wantNewName)
				}
				if *captured.NewName != *tc.wantNewName {
					t.Fatalf("captured NewName = %q, want %q", *captured.NewName, *tc.wantNewName)
				}
			}

			// Assert AccessPolicy.
			if tc.wantAccessPolicySet {
				if captured.AccessPolicy == nil {
					t.Fatalf("captured AccessPolicy = unset, want %q", tc.wantAccessPolicy)
				}
				if *captured.AccessPolicy != tc.wantAccessPolicy {
					t.Fatalf("captured AccessPolicy = %q, want %q", *captured.AccessPolicy, tc.wantAccessPolicy)
				}
			} else if captured.AccessPolicy != nil {
				t.Fatalf("captured AccessPolicy = %q, want unset for a name-only update", *captured.AccessPolicy)
			}
		})
	}
}

// TestGetKeyInfoErrorResponses covers getKeyInfo's error handling: an HTTP 404,
// an HTTP 500, and a transport error (server closed) which must exercise the
// nil-response guard without panicking.
func TestGetKeyInfoErrorResponses(t *testing.T) {
	ctx := context.Background()
	id := apiKeyTestID

	t.Run("not_found_404", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":5,"message":"api key not found"}`))
		}))
		defer server.Close()

		r := apiKeyResourceForServer(server.URL)
		key, httpResponse, diags := getKeyInfo(ctx, r.client, &id, nil)

		if !diags.HasError() {
			t.Fatal("getKeyInfo() diagnostics = none, want an error for a 404 response")
		}
		if key != nil {
			t.Fatalf("getKeyInfo() key = %#v, want nil on error", key)
		}
		if httpResponse == nil {
			t.Fatal("getKeyInfo() httpResponse = nil, want the 404 response")
		}
		if httpResponse.StatusCode != http.StatusNotFound {
			t.Fatalf("getKeyInfo() status = %d, want %d", httpResponse.StatusCode, http.StatusNotFound)
		}
	})

	t.Run("server_error_500", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code":13,"message":"internal error"}`))
		}))
		defer server.Close()

		r := apiKeyResourceForServer(server.URL)
		key, httpResponse, diags := getKeyInfo(ctx, r.client, &id, nil)

		if !diags.HasError() {
			t.Fatal("getKeyInfo() diagnostics = none, want an error for a 500 response")
		}
		if key != nil {
			t.Fatalf("getKeyInfo() key = %#v, want nil on error", key)
		}
		if httpResponse == nil {
			t.Fatal("getKeyInfo() httpResponse = nil, want the 500 response")
		}
		if httpResponse.StatusCode != http.StatusInternalServerError {
			t.Fatalf("getKeyInfo() status = %d, want %d", httpResponse.StatusCode, http.StatusInternalServerError)
		}
	})

	t.Run("transport_error_nil_response_guard", func(t *testing.T) {
		// Start a server, then immediately close it so the client fails to
		// connect and the SDK returns a nil *http.Response alongside the error.
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		baseURL := server.URL
		server.Close()

		r := apiKeyResourceForServer(baseURL)
		key, httpResponse, diags := getKeyInfo(ctx, r.client, &id, nil)

		if !diags.HasError() {
			t.Fatal("getKeyInfo() diagnostics = none, want an error for a transport failure")
		}
		if key != nil {
			t.Fatalf("getKeyInfo() key = %#v, want nil on error", key)
		}
		if httpResponse != nil {
			t.Fatalf("getKeyInfo() httpResponse = %#v, want nil on a transport failure", httpResponse)
		}
	})
}
