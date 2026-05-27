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

package dashboards

import (
	"encoding/json"
	"strings"
	"time"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	dashboardService "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	dashboardschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_schema"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
)

func openAPIDashboardFromProto(dashboard *cxsdk.Dashboard) (dashboardService.Dashboard, diag.Diagnostics) {
	if dashboard == nil {
		return dashboardService.Dashboard{}, diag.Diagnostics{diag.NewErrorDiagnostic("Error converting Dashboard to OpenAPI", "dashboard was nil")}
	}

	dashboardCopy, ok := proto.Clone(dashboard).(*cxsdk.Dashboard)
	if !ok {
		return dashboardService.Dashboard{}, diag.Diagnostics{diag.NewErrorDiagnostic("Error converting Dashboard to OpenAPI", "failed to clone dashboard")}
	}

	if dashboardCopy.GetAutoRefresh() == nil {
		dashboardCopy.AutoRefresh = &cxsdk.DashboardOff{Off: &cxsdk.DashboardAutoRefreshOff{}}
	}

	if dashboardCopy.GetTimeFrame() == nil {
		dashboardCopy.TimeFrame = &cxsdk.DashboardRelativeTimeFrame{RelativeTimeFrame: durationpb.New(15 * time.Minute)}
	}

	data, err := protojson.MarshalOptions{EmitUnpopulated: false}.Marshal(dashboardCopy)
	if err != nil {
		return dashboardService.Dashboard{}, diag.Diagnostics{diag.NewErrorDiagnostic("Error converting Dashboard to OpenAPI", err.Error())}
	}

	var openAPIDashboard dashboardService.Dashboard
	if err := json.Unmarshal(data, &openAPIDashboard); err != nil {
		return dashboardService.Dashboard{}, diag.Diagnostics{openAPIDashboardDiagnostic(err)}
	}

	return openAPIDashboard, nil
}

func protoDashboardFromOpenAPI(dashboard dashboardService.Dashboard) (*cxsdk.Dashboard, diag.Diagnostics) {
	data, err := json.Marshal(dashboard)
	if err != nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error converting OpenAPI Dashboard", err.Error())}
	}

	protoDashboard := new(cxsdk.Dashboard)
	if err := dashboardschema.JSONUnmarshal.Unmarshal(data, protoDashboard); err != nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error converting OpenAPI Dashboard", err.Error())}
	}

	return protoDashboard, nil
}

func newDashboardRequestID() string {
	return uuid.NewString()
}

func openAPIDashboardDiagnostic(err error) diag.Diagnostic {
	if strings.Contains(err.Error(), "oneOf(Dashboard)") {
		return diag.NewErrorDiagnostic(
			"Dashboard does not match OpenAPI dashboard schema",
			"OpenAPI requires exactly one dashboard time frame and exactly one auto-refresh value. Set one of absoluteTimeFrame or relativeTimeFrame, and one of off, twoMinutes, or fiveMinutes. Original error: "+err.Error(),
		)
	}

	return diag.NewErrorDiagnostic("Error converting Dashboard to OpenAPI", err.Error())
}
