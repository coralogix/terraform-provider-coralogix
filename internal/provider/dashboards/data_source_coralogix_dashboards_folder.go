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

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	dbfs "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_folders_service"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var _ datasource.DataSourceWithConfigure = &DashboardsFolderDataSource{}

func NewDashboardsFoldersDataSource() datasource.DataSource {
	return &DashboardsFolderDataSource{}
}

type DashboardsFolderDataSource struct {
	client *dbfs.DashboardFoldersServiceAPIService
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

	if idAttr, ok := resp.Schema.Attributes["id"].(schema.StringAttribute); ok {
		idAttr.Required = false
		idAttr.Optional = true
		idAttr.Validators = []validator.String{
			stringvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("name")),
		}
		resp.Schema.Attributes["id"] = idAttr
	}

	if nameAttr, ok := resp.Schema.Attributes["name"].(schema.StringAttribute); ok {
		nameAttr.Required = false
		nameAttr.Optional = true
		resp.Schema.Attributes["name"] = nameAttr
	}
}

func (d *DashboardsFolderDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DashboardsFolderResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed dashboards-folder value from Coralogix
	log.Print("[INFO] Reading dashboards-folders")
	listResult, httpResponse, err := d.client.DashboardFoldersServiceListDashboardFolders(ctx).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error listing coralogix_dashboard_folder", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil))
		return
	}
	var dashboardsFolder dbfs.DashboardFolder
	var found = false
	for _, folder := range listResult.GetFolder() {
		found = folder.GetId() == data.ID.ValueString() ||
			*folder.Name == data.Name.ValueString()
		if found {
			dashboardsFolder = folder
			break
		}
	}
	if !found {
		resp.Diagnostics.AddError(
			"Error reading coralogix_dashboard_folders",
			fmt.Sprintf("Could not find folder with id (%s) or name (%s)", data.ID.ValueString(), data.Name.ValueString()),
		)
	}
	data = flattenDashboardsFolder(&dashboardsFolder)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
