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
	"fmt"
	"net/http"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"

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
	Dashboard    *cxsdk.Dashboard
	AccessPolicy *string
}

func newDashboardOpenAPIClient(client *dashboardservice.DashboardServiceAPIService) *dashboardOpenAPIClient {
	return &dashboardOpenAPIClient{client: client}
}

func (c *dashboardOpenAPIClient) Create(ctx context.Context, dashboard *cxsdk.Dashboard, accessPolicy *string) (*dashboardservice.CreateDashboardResponse, error) {
	request, err := newDashboardOpenAPICreateRequestFromProto(dashboard, accessPolicy)
	if err != nil {
		return nil, err
	}

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

	return dashboardOpenAPIGetResponseToReadResult(response)
}

func (c *dashboardOpenAPIClient) Replace(ctx context.Context, dashboard *cxsdk.Dashboard, accessPolicy *string) error {
	request, err := newDashboardOpenAPIReplaceRequestFromProto(dashboard, accessPolicy)
	if err != nil {
		return err
	}

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

	return formatDashboardOpenAPIError(httpResponse, err, dashboardOpenAPIOperationDelete, id)
}

func newDashboardOpenAPICreateRequestFromProto(dashboard *cxsdk.Dashboard, accessPolicy *string) (dashboardservice.CreateDashboardRequestDataStructure, error) {
	openAPIDashboard, err := dashboardProtoToOpenAPI(dashboard)
	if err != nil {
		return dashboardservice.CreateDashboardRequestDataStructure{}, err
	}

	return newDashboardOpenAPICreateRequest(openAPIDashboard, accessPolicy), nil
}

func newDashboardOpenAPIReplaceRequestFromProto(dashboard *cxsdk.Dashboard, accessPolicy *string) (dashboardservice.ReplaceDashboardRequestDataStructure, error) {
	openAPIDashboard, err := dashboardProtoToOpenAPI(dashboard)
	if err != nil {
		return dashboardservice.ReplaceDashboardRequestDataStructure{}, err
	}

	return newDashboardOpenAPIReplaceRequest(openAPIDashboard, accessPolicy), nil
}

func newDashboardOpenAPICreateRequest(dashboard dashboardservice.Dashboard, accessPolicy *string) dashboardservice.CreateDashboardRequestDataStructure {
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
	request := dashboardservice.ReplaceDashboardRequestDataStructure{
		Dashboard: dashboard,
		RequestId: newDashboardOpenAPIRequestID(dashboardOpenAPIOperationReplace),
	}
	if accessPolicy != nil {
		request.AccessPolicy = accessPolicy
	}

	return request
}

func newDashboardOpenAPIRequestID(operation string) string {
	return fmt.Sprintf("%s-%s-%s", dashboardOpenAPIRequestIDPrefix, operation, uuid.NewString())
}

// dashboardProtoToOpenAPI is a migration-only bridge while the dashboard schema,
// expanders, and flatteners still use the protobuf SDK model. Remove it once the
// resource boundary is fully OpenAPI-native.
func dashboardProtoToOpenAPI(dashboard *cxsdk.Dashboard) (dashboardservice.Dashboard, error) {
	if dashboard == nil {
		return dashboardservice.Dashboard{}, fmt.Errorf("dashboard is required")
	}

	protoJSON, err := protojson.Marshal(dashboard)
	if err != nil {
		return dashboardservice.Dashboard{}, fmt.Errorf("marshal protobuf dashboard for OpenAPI request: %w", err)
	}

	var openAPIDashboard dashboardservice.Dashboard
	if err := json.Unmarshal(protoJSON, &openAPIDashboard); err != nil {
		return dashboardservice.Dashboard{}, fmt.Errorf("unmarshal protobuf dashboard into OpenAPI request: %w", err)
	}

	return openAPIDashboard, nil
}

// dashboardOpenAPIGetResponseToReadResult is a migration-only bridge while the
// dashboard schema, widget helpers, and flatteners still use the protobuf SDK
// dashboard model. Remove it once the resource boundary is fully OpenAPI-native.
func dashboardOpenAPIGetResponseToReadResult(response *dashboardservice.GetDashboardResponse) (*dashboardOpenAPIReadResult, error) {
	if response == nil {
		return nil, fmt.Errorf("dashboard response is required")
	}
	if response.Dashboard == nil {
		return nil, fmt.Errorf("dashboard response did not include dashboard")
	}

	openAPIJSON, err := json.Marshal(response.Dashboard)
	if err != nil {
		return nil, fmt.Errorf("marshal OpenAPI dashboard for protobuf flattener: %w", err)
	}

	var dashboard cxsdk.Dashboard
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(openAPIJSON, &dashboard); err != nil {
		return nil, fmt.Errorf("unmarshal OpenAPI dashboard into protobuf flattener: %w", err)
	}

	return &dashboardOpenAPIReadResult{
		Dashboard:    &dashboard,
		AccessPolicy: response.AccessPolicy,
	}, nil
}

func formatDashboardOpenAPIError(httpResponse *http.Response, err error, operation string, request any) error {
	if err == nil {
		return nil
	}

	apiErr := cxsdkOpenapi.NewAPIError(httpResponse, err)
	return fmt.Errorf("%s", utils.FormatOpenAPIErrors(apiErr, operation, request))
}

func isDashboardOpenAPINotFound(httpResponse *http.Response, err error) bool {
	if responseStatusCode(httpResponse) == http.StatusNotFound {
		return true
	}

	apiErr := cxsdkOpenapi.NewAPIError(httpResponse, err)
	return cxsdkOpenapi.Code(apiErr) == http.StatusNotFound
}

func responseStatusCode(response *http.Response) int {
	if response == nil {
		return 0
	}

	return response.StatusCode
}
