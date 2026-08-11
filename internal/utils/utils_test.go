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

package utils

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type stubOpenAPIError struct {
	msg  string
	body []byte
}

func (s *stubOpenAPIError) Error() string      { return s.msg }
func (s *stubOpenAPIError) Body() []byte       { return s.body }
func (s *stubOpenAPIError) Model() interface{} { return nil }

func openAPIError(statusCode int, msg, body string) error {
	return cxsdkOpenapi.NewAPIError(
		&http.Response{StatusCode: statusCode},
		&stubOpenAPIError{msg: msg, body: []byte(body)},
	)
}

func TestFormatErrorsClassifyAndLabelOperation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		got         string
		wantPrefix  string
		wantSubstrs []string
		notSubstrs  []string
	}{
		{
			name:        "openapi 400 is classified as invalid argument with url and backend body",
			got:         FormatOpenAPIErrors(openAPIError(http.StatusBadRequest, "400 Bad Request", `{"message":"invalid lucene query"}`), "Create", nil),
			wantPrefix:  "invalid argument error.",
			wantSubstrs: []string{"operation - Create", "invalid lucene query"},
		},
		{
			name:        "openapi 422 is classified as invalid argument",
			got:         FormatOpenAPIErrors(openAPIError(http.StatusUnprocessableEntity, "422 Unprocessable Entity", ""), "Update", nil),
			wantPrefix:  "invalid argument error.",
			wantSubstrs: []string{"operation - Update"},
		},
		{
			name:        "openapi 503 is classified as an internal backend error",
			got:         FormatOpenAPIErrors(openAPIError(http.StatusServiceUnavailable, "503 Service Unavailable", ""), "Create", nil),
			wantPrefix:  "internal error in Coralogix backend.",
			wantSubstrs: []string{"operation - Create"},
		},
		{
			name:        "openapi 404 falls to the default branch with url",
			got:         FormatOpenAPIErrors(openAPIError(http.StatusNotFound, "404 Not Found", ""), "Read", nil),
			wantPrefix:  "error - 404 Not Found",
			wantSubstrs: []string{"operation - Read"},
		},
		{
			name:        "a non-APIError falls to the default branch with url",
			got:         FormatOpenAPIErrors(errors.New("transport failure"), "Read", nil),
			wantPrefix:  "error - transport failure",
			wantSubstrs: []string{"operation - Read"},
		},
		{
			name:        "rpc error appends google.rpc details",
			got:         FormatRpcErrors(rpcErrorWithDetails(t, codes.InvalidArgument, "bad query", "unexpected EOF at position 4"), "https://example.com/alerts", ""),
			wantPrefix:  "invalid argument error.",
			wantSubstrs: []string{"operation - https://example.com/alerts", "details - ", "unexpected EOF at position 4"},
		},
		{
			name:        "rpc error without details does not add a details line",
			got:         FormatRpcErrors(rpcError(codes.InvalidArgument, "bad query"), "https://example.com/alerts", ""),
			wantPrefix:  "invalid argument error.",
			wantSubstrs: []string{"operation - https://example.com/alerts"},
			notSubstrs:  []string{"details - "},
		},
		{
			name:        "rpc error with an unclassified code falls to the default branch with url",
			got:         FormatRpcErrors(rpcError(codes.PermissionDenied, "forbidden"), "https://example.com/alerts", ""),
			wantPrefix:  "error - ",
			wantSubstrs: []string{"operation - https://example.com/alerts"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if !strings.HasPrefix(tt.got, tt.wantPrefix) {
				t.Errorf("formatted error = %q, want prefix %q", tt.got, tt.wantPrefix)
			}
			for _, want := range tt.wantSubstrs {
				if !strings.Contains(tt.got, want) {
					t.Errorf("formatted error = %q, want it to contain %q", tt.got, want)
				}
			}
			for _, notWant := range tt.notSubstrs {
				if strings.Contains(tt.got, notWant) {
					t.Errorf("formatted error = %q, want it not to contain %q", tt.got, notWant)
				}
			}
		})
	}
}

// The formatters must never echo the request payload: it can carry credentials
// (connector fields, integration parameters, webhook tokens/urls) that would
// otherwise land in CLI and CI logs on any API failure.
func TestFormatErrorsDoNotEchoRequest(t *testing.T) {
	t.Parallel()

	secretObj := map[string]any{
		"connectorConfig": map[string]any{
			"fields": []any{map[string]any{"fieldName": "integrationKey", "value": "R0234ABCDSECRET"}},
		},
	}

	cases := []struct {
		name, got, secret string
	}{
		{"openapi", FormatOpenAPIErrors(openAPIError(http.StatusBadRequest, "400 Bad Request", ""), "Create", secretObj), "R0234ABCDSECRET"},
		{"rpc", FormatRpcErrors(rpcError(codes.InvalidArgument, "bad request"), "https://example.com/x", `{"apiToken":"jira-secret"}`), "jira-secret"},
	}
	for _, c := range cases {
		if strings.Contains(c.got, c.secret) {
			t.Errorf("%s: diagnostic = %q, want the request secret %q absent", c.name, c.got, c.secret)
		}
		if strings.Contains(c.got, "request -") {
			t.Errorf("%s: diagnostic = %q, want no request line", c.name, c.got)
		}
	}
}

func rpcError(code codes.Code, msg string) error {
	return cxsdk.NewSdkAPIError(status.Error(code, msg), "example-endpoint", "example-feature-group")
}

func rpcErrorWithDetails(t *testing.T, code codes.Code, msg, detail string) error {
	t.Helper()
	st, err := status.New(code, msg).WithDetails(wrapperspb.String(detail))
	if err != nil {
		t.Fatalf("status.WithDetails(%q) = %v", detail, err)
	}
	return cxsdk.NewSdkAPIError(st.Err(), "example-endpoint", "example-feature-group")
}
