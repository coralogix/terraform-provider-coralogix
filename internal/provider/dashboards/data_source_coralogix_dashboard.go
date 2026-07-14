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
	"context"
	"fmt"
	"log"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var _ datasource.DataSourceWithConfigure = &DashboardDataSource{}

func NewDashboardDataSource() datasource.DataSource {
	return &DashboardDataSource{}
}

type DashboardDataSource struct {
	client *dashboardOpenAPIClient
}

func (d *DashboardDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dashboard"
}

func (d *DashboardDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = newDashboardOpenAPIClient(clientSet.DashboardsOpenAPI())
}

func (d *DashboardDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r DashboardResource
	var resourceResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resourceResp)

	resp.Schema = utils.FrameworkDatasourceSchemaFromFrameworkResourceSchema(resourceResp.Schema)
}

func (d *DashboardDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DashboardResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed Dashboard value from Coralogix
	id := data.ID.ValueString()
	log.Printf("[INFO] Reading Dashboard: %s", id)
	openAPIGetDashboardResp, err := d.client.Get(ctx, id)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error reading Dashboard",
			err.Error(),
		)
		return
	}
	getDashboardResp, err := dashboardOpenAPIGetResponseToProto(openAPIGetDashboardResp)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Dashboard", err.Error())
		return
	}
	log.Printf("[INFO] Received Dashboard: %s", protojson.Format(getDashboardResp))

	dashboard, diags := flattenDashboard(ctx, DashboardResourceModel{}, getDashboardResp)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}

	data = *dashboard
	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
