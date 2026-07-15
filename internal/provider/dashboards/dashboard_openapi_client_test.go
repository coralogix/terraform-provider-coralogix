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

package dashboards

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	"github.com/google/uuid"
)

func TestNewDashboardOpenAPICreateRequest(t *testing.T) {
	accessPolicy := `{"version":"2025-01-01"}`
	dashboard := dashboardservice.Dashboard{Name: "test"}

	request := newDashboardOpenAPICreateRequest(dashboard, &accessPolicy)

	if request.Dashboard.Name != dashboard.Name {
		t.Fatalf("expected dashboard name %q, got %q", dashboard.Name, request.Dashboard.Name)
	}
	if request.AccessPolicy == nil || *request.AccessPolicy != accessPolicy {
		t.Fatalf("expected access policy %q, got %v", accessPolicy, request.AccessPolicy)
	}
	assertDashboardOpenAPIRequestID(t, request.RequestId, dashboardOpenAPIOperationCreate)
}

func TestNewDashboardOpenAPIRequestDiscardsUnknownProperties(t *testing.T) {
	dashboard := dashboardservice.Dashboard{
		Name: "test",
		Layout: dashboardservice.Layout{
			AdditionalProperties: map[string]interface{}{"unknownNested": true},
		},
		AdditionalProperties: map[string]interface{}{"unknownKey": "should-not-fail"},
	}

	request := newDashboardOpenAPICreateRequest(dashboard, nil)
	content, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("failed to marshal request: %s", err)
	}

	for _, unknownField := range []string{"unknownKey", "unknownNested"} {
		if strings.Contains(string(content), unknownField) {
			t.Fatalf("expected %q to be discarded, got request %s", unknownField, content)
		}
	}
}

func TestRestoreOpenAPIProtoFieldNames(t *testing.T) {
	content := []byte(`{
		"name": "test",
		"layout": {
			"sections": [{
				"rows": [{
					"widgets": [{
						"definition": {
							"data_table": {
								"results_per_page": 10,
								"row_style": "ROW_STYLE_ONE_LINE"
							}
						}
					}]
				}]
			}]
		},
		"unknownKey": "should-not-fail"
	}`)

	dashboard := new(dashboardservice.Dashboard)
	if err := json.Unmarshal(content, dashboard); err != nil {
		t.Fatalf("failed to unmarshal dashboard: %s", err)
	}
	if err := restoreOpenAPIProtoFieldNames(dashboard); err != nil {
		t.Fatalf("failed to restore protobuf field names: %s", err)
	}

	definition := dashboard.Layout.Sections[0].Rows[0].Widgets[0].Definition
	if definition.DataTable == nil {
		t.Fatal("expected data_table to be promoted to dataTable")
	}
	if definition.DataTable.ResultsPerPage == nil || *definition.DataTable.ResultsPerPage != 10 {
		t.Fatalf("expected results_per_page to be promoted, got %v", definition.DataTable.ResultsPerPage)
	}
	if definition.DataTable.RowStyle == nil || *definition.DataTable.RowStyle != dashboardservice.ROWSTYLE_ROW_STYLE_ONE_LINE {
		t.Fatalf("expected row_style to be promoted, got %v", definition.DataTable.RowStyle)
	}

	request := newDashboardOpenAPICreateRequest(*dashboard, nil)
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("failed to marshal request: %s", err)
	}
	if strings.Contains(string(encoded), "unknownKey") {
		t.Fatalf("expected unknown field to be discarded, got request %s", encoded)
	}
	if !strings.Contains(string(encoded), `"dataTable"`) {
		t.Fatalf("expected normalized dataTable definition, got request %s", encoded)
	}
}

func TestNewDashboardOpenAPIReplaceRequest(t *testing.T) {
	dashboard := dashboardservice.Dashboard{Name: "test"}
	accessPolicy := `{"version":"2025-01-01"}`

	tests := []struct {
		name         string
		accessPolicy *string
	}{
		{
			name: "omits nil access policy",
		},
		{
			name:         "includes configured access policy",
			accessPolicy: &accessPolicy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := newDashboardOpenAPIReplaceRequest(dashboard, tt.accessPolicy)

			if request.Dashboard.Name != dashboard.Name {
				t.Fatalf("expected dashboard name %q, got %q", dashboard.Name, request.Dashboard.Name)
			}
			if tt.accessPolicy == nil {
				if request.AccessPolicy != nil {
					t.Fatalf("expected nil access policy, got %q", *request.AccessPolicy)
				}
			} else if request.AccessPolicy == nil || *request.AccessPolicy != *tt.accessPolicy {
				t.Fatalf("expected access policy %q, got %v", *tt.accessPolicy, request.AccessPolicy)
			}
			assertDashboardOpenAPIRequestID(t, request.RequestId, dashboardOpenAPIOperationReplace)
		})
	}
}

func TestFormatDashboardOpenAPIError(t *testing.T) {
	err := formatDashboardOpenAPIError(&http.Response{StatusCode: http.StatusBadRequest}, errors.New("api failed"), dashboardOpenAPIOperationCreate, map[string]string{"name": "test"})
	if err == nil {
		t.Fatal("expected formatted error")
	}
	if !strings.Contains(err.Error(), "api failed") {
		t.Fatalf("expected formatted error to contain original error, got %q", err.Error())
	}

	if err := formatDashboardOpenAPIError(nil, nil, dashboardOpenAPIOperationCreate, nil); err != nil {
		t.Fatalf("expected nil error when SDK returned no error, got %s", err)
	}
}

func TestIsDashboardOpenAPINotFound(t *testing.T) {
	tests := []struct {
		name         string
		httpResponse *http.Response
		err          error
		want         bool
	}{
		{
			name:         "404 response",
			httpResponse: &http.Response{StatusCode: http.StatusNotFound},
			err:          errors.New("not found"),
			want:         true,
		},
		{
			name:         "non-404 response",
			httpResponse: &http.Response{StatusCode: http.StatusInternalServerError},
			err:          errors.New("server error"),
			want:         false,
		},
		{
			name: "nil response",
			err:  errors.New("network error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDashboardOpenAPINotFound(tt.httpResponse, tt.err)
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestDashboardOpenAPIClientCreateRejectionPreservesErrorContext(t *testing.T) {
	const (
		apiKey        = "test-api-key-must-not-leak"
		backendDetail = "dashboard layout violates a backend business rule"
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/dashboards/dashboards/v1" {
			t.Fatalf("request = %s %s, want POST /dashboards/dashboards/v1", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":400,"message":"` + backendDetail + `"}`))
	}))
	t.Cleanup(server.Close)

	client := newDashboardOpenAPITestClient(server, apiKey)
	response, err := client.Create(context.Background(), &dashboardservice.Dashboard{Name: "invalid-but-serializable"}, nil)
	if response != nil {
		t.Fatalf("Create() response = %#v, want nil", response)
	}
	if err == nil {
		t.Fatal("Create() error = nil, want backend rejection")
	}
	for _, context := range []string{dashboardOpenAPIOperationCreate, "400", backendDetail} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("Create() error = %q, want context %q", err, context)
		}
	}
	if strings.Contains(err.Error(), apiKey) {
		t.Fatalf("Create() error exposed API key: %q", err)
	}
}

func TestDashboardOpenAPIClientReplaceRejectionLeavesPriorDashboardReadable(t *testing.T) {
	const (
		dashboardID   = "123456789012345678901"
		backendDetail = "replacement dashboard was rejected"
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/dashboards/dashboards/v1":
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"code":422,"message":"` + backendDetail + `"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/dashboards/dashboards/v1/"+dashboardID:
			_, _ = w.Write([]byte(`{"dashboard":{"id":"` + dashboardID + `","name":"prior dashboard","layout":{"sections":[]}}}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	client := newDashboardOpenAPITestClient(server, "")
	err := client.Replace(context.Background(), &dashboardservice.Dashboard{Id: ptr(dashboardID), Name: "rejected replacement"}, nil)
	if err == nil {
		t.Fatal("Replace() error = nil, want backend rejection")
	}
	for _, context := range []string{dashboardOpenAPIOperationReplace, "422", backendDetail} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("Replace() error = %q, want context %q", err, context)
		}
	}

	readResult, err := client.Get(context.Background(), dashboardID)
	if err != nil {
		t.Fatalf("Get() after rejected replacement error = %v", err)
	}
	if got := readResult.Dashboard.GetName(); got != "prior dashboard" {
		t.Fatalf("Get() after rejected replacement name = %q, want prior dashboard", got)
	}
}

func TestDashboardOpenAPIClientDeleteAlreadyAbsentIsIdempotent(t *testing.T) {
	const dashboardID = "123456789012345678901"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/dashboards/dashboards/v1/"+dashboardID {
			t.Fatalf("request = %s %s, want dashboard DELETE", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":404,"message":"dashboard is already absent"}`))
	}))
	t.Cleanup(server.Close)

	if err := newDashboardOpenAPITestClient(server, "").Delete(context.Background(), dashboardID); err != nil {
		t.Fatalf("Delete() already-absent dashboard error = %v, want nil", err)
	}
}

func TestDashboardOpenAPIClientGetNotFoundRetainsRESTContext(t *testing.T) {
	const dashboardID = "123456789012345678901"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":404,"message":"dashboard no longer exists"}`))
	}))
	t.Cleanup(server.Close)

	_, err := newDashboardOpenAPITestClient(server, "").Get(context.Background(), dashboardID)
	if !errors.Is(err, errDashboardOpenAPINotFound) {
		t.Fatalf("Get() error = %v, want errDashboardOpenAPINotFound", err)
	}
	for _, context := range []string{dashboardOpenAPIOperationGet, "404", "dashboard no longer exists"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("Get() error = %q, want context %q", err, context)
		}
	}
}

func newDashboardOpenAPITestClient(server *httptest.Server, apiKey string) *dashboardOpenAPIClient {
	configuration := dashboardservice.NewConfiguration()
	configuration.HTTPClient = server.Client()
	configuration.Servers = dashboardservice.ServerConfigurations{{URL: server.URL}}
	if apiKey != "" {
		configuration.AddDefaultHeader("Authorization", "Bearer "+apiKey)
	}
	return newDashboardOpenAPIClient(dashboardservice.NewAPIClient(configuration).DashboardServiceAPI)
}

func assertDashboardOpenAPIRequestID(t *testing.T, requestID string, operation string) {
	t.Helper()

	prefix := dashboardOpenAPIRequestIDPrefix + "-" + operation + "-"
	if !strings.HasPrefix(requestID, prefix) {
		t.Fatalf("expected request ID prefix %q, got %q", prefix, requestID)
	}

	uuidPart := strings.TrimPrefix(requestID, prefix)
	if _, err := uuid.Parse(uuidPart); err != nil {
		t.Fatalf("expected request ID to end with UUID, got %q: %s", uuidPart, err)
	}
}
