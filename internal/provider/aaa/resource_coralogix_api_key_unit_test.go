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
	"net/http"
	"testing"

	apiKeys "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/api_keys_service"
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
			name:    "cleared_to_null_sends_empty_string",
			current: types.StringValue(policyA),
			desired: types.StringNull(),
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
