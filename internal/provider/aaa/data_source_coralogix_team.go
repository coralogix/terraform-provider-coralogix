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

package aaa

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	teamsservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/teams_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSourceWithConfigure = &TeamDataSource{}
)

func NewTeamDataSource() datasource.DataSource {
	return &TeamDataSource{}
}

type TeamDataSource struct {
	client *teamsservice.TeamsServiceAPIService
}

func (d *TeamDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team"
}

func (d *TeamDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clientSet, ok := req.ProviderData.(*clientset.ClientSet)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *clientset.ClientSet, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = clientSet.Teams()
}

func (d *TeamDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r TeamResource
	var resourceResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resourceResp)

	resp.Schema = utils.FrameworkDatasourceSchemaFromFrameworkResourceSchema(resourceResp.Schema)
}

func (d *TeamDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data *TeamResourceModel
	diags := req.Config.Get(ctx, &data)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	//Get refreshed Team value from Coralogix
	teamId, err := strconv.ParseInt(data.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing Team ID",
			fmt.Sprintf("Error parsing Team ID: %s", err.Error()),
		)
		return
	}
	log.Printf("[INFO] Reading Team: %d", teamId)
	getTeamResp, httpResponse, err := d.client.TeamServiceGetTeam(ctx, teamId).Execute()
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdkOpenapi.IsNotFound(cxsdkOpenapi.NewAPIError(httpResponse, err)) {
			resp.Diagnostics.AddWarning(
				err.Error(),
				fmt.Sprintf("Team %d is in state, but no longer exists in Coralogix backend", teamId),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Team",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
			)
		}
		return
	}
	log.Printf("[INFO] Received Team: %s", utils.FormatJSON(getTeamResp))

	data = &TeamResourceModel{
		ID:         types.StringValue(strconv.FormatInt(getTeamResp.TeamId.GetId(), 10)),
		Name:       types.StringValue(getTeamResp.GetTeamName()),
		Retention:  types.Int64Value(int64(getTeamResp.GetRetention())),
		DailyQuota: types.Float64Value(math.Round(getTeamResp.GetDailyQuota()*1000) / 1000),
	}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
