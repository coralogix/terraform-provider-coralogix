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
	"errors"
	"net/http"
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

func TestDashboardOpenAPIGetResponseToReadResult(t *testing.T) {
	accessPolicy := `{"version":"2025-01-01"}`
	openAPIResponse := &dashboardservice.GetDashboardResponse{
		AccessPolicy: &accessPolicy,
		Dashboard: &dashboardservice.Dashboard{
			Id:          ptr("dashboard-id"),
			Name:        "dashboard-name",
			Description: ptr("migration bridge"),
			Layout:      *dashboardservice.NewLayout(),
		},
	}

	got, err := dashboardOpenAPIGetResponseToReadResult(openAPIResponse)
	if err != nil {
		t.Fatalf("unexpected error converting dashboard response: %s", err)
	}

	if got.Dashboard.GetId() != openAPIResponse.Dashboard.GetId() {
		t.Fatalf("expected id %q, got %q", openAPIResponse.Dashboard.GetId(), got.Dashboard.GetId())
	}
	if got.Dashboard.GetName() != openAPIResponse.Dashboard.GetName() {
		t.Fatalf("expected name %q, got %q", openAPIResponse.Dashboard.GetName(), got.Dashboard.GetName())
	}
	if got.AccessPolicy == nil || *got.AccessPolicy != accessPolicy {
		t.Fatalf("expected access policy %q, got %v", accessPolicy, got.AccessPolicy)
	}
}

func TestDashboardOpenAPIGetResponseToReadResultRequiresResponse(t *testing.T) {
	if _, err := dashboardOpenAPIGetResponseToReadResult(nil); err == nil {
		t.Fatal("expected an error for nil response")
	}
}

func TestDashboardOpenAPIGetResponseToReadResultRequiresDashboard(t *testing.T) {
	if _, err := dashboardOpenAPIGetResponseToReadResult(&dashboardservice.GetDashboardResponse{}); err == nil {
		t.Fatal("expected an error for missing dashboard")
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

func ptr[T any](v T) *T {
	return &v
}
