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
	"errors"
	"fmt"
	"net/http"
	"reflect"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	"github.com/google/uuid"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"
)

const (
	dashboardOpenAPIOperationCreate  = "Create"
	dashboardOpenAPIOperationGet     = "Get"
	dashboardOpenAPIOperationReplace = "Replace"
	dashboardOpenAPIOperationDelete  = "Delete"

	dashboardOpenAPIRequestIDPrefix = "terraform-provider-coralogix-dashboard"
)

var errDashboardOpenAPINotFound = errors.New("dashboard not found")

type dashboardOpenAPIClient struct {
	client *dashboardservice.DashboardServiceAPIService
}

type dashboardOpenAPIReadResult struct {
	Dashboard    *dashboardservice.Dashboard
	AccessPolicy *string
}

func newDashboardOpenAPIClient(client *dashboardservice.DashboardServiceAPIService) *dashboardOpenAPIClient {
	return &dashboardOpenAPIClient{client: client}
}

func (c *dashboardOpenAPIClient) Create(ctx context.Context, dashboard *dashboardservice.Dashboard, accessPolicy *string) (*dashboardservice.CreateDashboardResponse, error) {
	if dashboard == nil {
		return nil, fmt.Errorf("dashboard is required")
	}

	request := newDashboardOpenAPICreateRequest(*dashboard, accessPolicy)
	response, httpResponse, err := c.client.
		DashboardsServiceCreateDashboard(ctx).
		CreateDashboardRequestDataStructure(request).
		Execute()

	return response, formatDashboardOpenAPIError(httpResponse, err, dashboardOpenAPIOperationCreate, request)
}

func (c *dashboardOpenAPIClient) Get(ctx context.Context, id string) (*dashboardOpenAPIReadResult, error) {
	response, httpResponse, err := c.client.
		DashboardsServiceGetDashboard(ctx, id).
		Execute()
	if isDashboardOpenAPINotFound(httpResponse, err) {
		formattedErr := formatDashboardOpenAPIError(httpResponse, err, dashboardOpenAPIOperationGet, id)
		if formattedErr == nil {
			return nil, errDashboardOpenAPINotFound
		}
		return nil, fmt.Errorf("%w: %s", errDashboardOpenAPINotFound, formattedErr)
	}

	if err := formatDashboardOpenAPIError(httpResponse, err, dashboardOpenAPIOperationGet, id); err != nil {
		return nil, err
	}

	if response == nil {
		return nil, fmt.Errorf("dashboard response is required")
	}
	if response.Dashboard == nil {
		return nil, fmt.Errorf("dashboard response did not include dashboard")
	}

	return &dashboardOpenAPIReadResult{
		Dashboard:    response.Dashboard,
		AccessPolicy: response.AccessPolicy,
	}, nil
}

func (c *dashboardOpenAPIClient) Replace(ctx context.Context, dashboard *dashboardservice.Dashboard, accessPolicy *string) error {
	if dashboard == nil {
		return fmt.Errorf("dashboard is required")
	}

	request := newDashboardOpenAPIReplaceRequest(*dashboard, accessPolicy)
	_, httpResponse, err := c.client.
		DashboardsServiceReplaceDashboard(ctx).
		ReplaceDashboardRequestDataStructure(request).
		Execute()

	return formatDashboardOpenAPIError(httpResponse, err, dashboardOpenAPIOperationReplace, request)
}

func (c *dashboardOpenAPIClient) Delete(ctx context.Context, id string) error {
	_, httpResponse, err := c.client.
		DashboardsServiceDeleteDashboard(ctx, id).
		Execute()
	if isDashboardOpenAPINotFound(httpResponse, err) {
		return nil
	}

	return formatDashboardOpenAPIError(httpResponse, err, dashboardOpenAPIOperationDelete, id)
}

func newDashboardOpenAPICreateRequest(dashboard dashboardservice.Dashboard, accessPolicy *string) dashboardservice.CreateDashboardRequestDataStructure {
	discardOpenAPIAdditionalProperties(&dashboard)
	request := dashboardservice.CreateDashboardRequestDataStructure{
		Dashboard: dashboard,
		RequestId: newDashboardOpenAPIRequestID(dashboardOpenAPIOperationCreate),
	}
	if accessPolicy != nil {
		request.AccessPolicy = accessPolicy
	}

	return request
}

func newDashboardOpenAPIReplaceRequest(dashboard dashboardservice.Dashboard, accessPolicy *string) dashboardservice.ReplaceDashboardRequestDataStructure {
	discardOpenAPIAdditionalProperties(&dashboard)
	request := dashboardservice.ReplaceDashboardRequestDataStructure{
		Dashboard: dashboard,
		RequestId: newDashboardOpenAPIRequestID(dashboardOpenAPIOperationReplace),
	}
	if accessPolicy != nil {
		request.AccessPolicy = accessPolicy
	}

	return request
}

// discardOpenAPIAdditionalProperties restores protojson's historical
// DiscardUnknown behavior for content_json. OpenAPI Generator captures unknown
// JSON fields in AdditionalProperties, but the protobuf HTTP endpoint rejects
// them when they are sent back.
func discardOpenAPIAdditionalProperties(value any) {
	discardAdditionalPropertiesValue(reflect.ValueOf(value))
}

func discardAdditionalPropertiesValue(value reflect.Value) {
	if !value.IsValid() {
		return
	}

	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		if !value.IsNil() {
			discardAdditionalPropertiesValue(value.Elem())
		}
	case reflect.Struct:
		if value.Type().PkgPath() != reflect.TypeOf(dashboardservice.Dashboard{}).PkgPath() {
			return
		}
		for i := 0; i < value.NumField(); i++ {
			field := value.Field(i)
			if value.Type().Field(i).Name == "AdditionalProperties" && field.CanSet() {
				field.SetZero()
				continue
			}
			discardAdditionalPropertiesValue(field)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			discardAdditionalPropertiesValue(value.Index(i))
		}
	case reflect.Map:
		for _, key := range value.MapKeys() {
			discardAdditionalPropertiesValue(value.MapIndex(key))
		}
	}
}

func newDashboardOpenAPIRequestID(operation string) string {
	return fmt.Sprintf("%s-%s-%s", dashboardOpenAPIRequestIDPrefix, operation, uuid.NewString())
}

func formatDashboardOpenAPIError(httpResponse *http.Response, err error, operation string, request any) error {
	if err == nil {
		return nil
	}

	apiErr := cxsdkOpenapi.NewAPIError(httpResponse, err)
	return fmt.Errorf("dashboard REST %s failed: %s", operation, utils.FormatOpenAPIErrors(apiErr, operation, request))
}

func isDashboardOpenAPINotFound(httpResponse *http.Response, err error) bool {
	apiErr := cxsdkOpenapi.NewAPIError(httpResponse, err)
	return cxsdkOpenapi.IsNotFound(apiErr)
}
