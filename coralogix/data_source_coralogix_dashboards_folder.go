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

	"google.golang.org/protobuf/encoding/protojson"

	"terraform-provider-coralogix/coralogix/clientset"
	"terraform-provider-coralogix/coralogix/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var _ datasource.DataSourceWithConfigure = &DashboardsFolderDataSource{}

func NewDashboardsFoldersDataSource() datasource.DataSource {
	return &DashboardsFolderDataSource{}
}

type DashboardsFolderDataSource struct {
	client *cxsdk.DashboardsFoldersClient
}

func (d *DashboardsFolderDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dashboards_folder"
}

func (d *DashboardsFolderDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = clientSet.DashboardsFolders()
}

func (d *DashboardsFolderDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r DashboardsFolderResource
	var resourceResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resourceResp)

	resp.Schema = utils.FrameworkDatasourceSchemaFromFrameworkResourceSchema(resourceResp.Schema)
}

func (d *DashboardsFolderDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data *DashboardsFolderResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed dashboards-folder value from Coralogix
	log.Print("[INFO] Reading dashboards-folders")
	getDashboardsFolders, err := d.client.List(ctx)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error listing dashboards-folders",
			utils.FormatRpcErrors(err, cxsdk.GetDashboardRPC, protojson.Format(&cxsdk.ListDashboardFolderRequest{})),
		)

		return
	}
	log.Printf("[INFO] Received dashboards-folders: %s", protojson.Format(getDashboardsFolders))
	var dashboardsFolder *cxsdk.DashboardFolder
	for _, folder := range getDashboardsFolders.GetFolder() {
		if folder.GetId().GetValue() == data.ID.ValueString() {
			dashboardsFolder = folder
			break
		}
	}
	if dashboardsFolder == nil {
		log.Printf("[ERROR] Could not find created folder with id: %s", data.ID.ValueString())
		resp.Diagnostics.AddError(
			"Error reading dashboards-folders",
			fmt.Sprintf("Could not find created folder with id: %s", data.ID.ValueString()),
		)
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
