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

package coralogix

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"terraform-provider-coralogix/coralogix/clientset"
	teams "terraform-provider-coralogix/coralogix/clientset/grpc/teams"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

var (
	_          datasource.DataSourceWithConfigure = &TeamDataSource{}
	getTeamURL                                    = "com.coralogixapis.aaa.organisations.v2.TeamService/GetTeam"
)

func NewTeamDataSource() datasource.DataSource {
	return &TeamDataSource{}
}

type TeamDataSource struct {
	client *clientset.TeamsClient
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

	resp.Schema = frameworkDatasourceSchemaFromFrameworkResourceSchema(resourceResp.Schema)
}

func (d *TeamDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data *TeamResourceModel
	diags := req.Config.Get(ctx, &data)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	//Get refreshed Team value from Coralogix
	intId, err := strconv.Atoi(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing Team ID",
			fmt.Sprintf("Error parsing Team ID: %s", err.Error()),
		)
		return
	}
	getTeamReq := &teams.GetTeamRequest{
		TeamId: &teams.TeamId{
			Id: uint32(intId),
		},
	}
	log.Printf("[INFO] Reading Team: %s", protojson.Format(getTeamReq))
	getTeamResp, err := d.client.GetTeam(ctx, getTeamReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				err.Error(),
				fmt.Sprintf("Team %q is in state, but no longer exists in Coralogix backend", intId),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Team",
				formatRpcErrors(err, getTeamURL, protojson.Format(getTeamReq)),
			)
		}
		return
	}
	log.Printf("[INFO] Received Team: %s", protojson.Format(getTeamResp))

	data = flattenTeam(getTeamResp)
	// Set state to fully populated data
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
