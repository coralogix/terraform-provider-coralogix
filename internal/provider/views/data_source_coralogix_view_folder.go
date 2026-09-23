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

package views

import (
	"context"
	"fmt"
	"log"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	viewsfolders "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/folders_for_views_service"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var _ datasource.DataSourceWithConfigure = &ViewFolderDataSource{}

func NewViewFolderDataSource() datasource.DataSource {
	return &ViewFolderDataSource{}
}

type ViewFolderDataSource struct {
	client *viewsfolders.FoldersForViewsServiceAPIService
}

func (d *ViewFolderDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_view_folder"
}

func (d *ViewFolderDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = clientSet.ViewsFolders()
}

func (d *ViewFolderDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r ViewFolderResource
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

func (d *ViewFolderDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ViewFolderResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	log.Print("[INFO] Listing view folders")
	listResult, httpResponse, err := d.client.ViewsFoldersServiceListViewFolders(ctx).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error listing coralogix_view_folder",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
		)
		return
	}

	// Exactly one of id/name is set, so match on the one the caller actually gave.
	byID := !data.ID.IsNull() && !data.ID.IsUnknown()
	folders := listResult.GetFolders()
	var match *viewsfolders.ViewFolder
	for i := range folders {
		if byID {
			if folders[i].GetId() == data.ID.ValueString() {
				match = &folders[i]
				break
			}
			continue
		}
		if folders[i].GetName() == data.Name.ValueString() {
			match = &folders[i]
			break
		}
	}
	if match == nil {
		if byID {
			resp.Diagnostics.AddError(
				"Error reading coralogix_view_folder",
				fmt.Sprintf("Could not find view folder with id (%s)", data.ID.ValueString()),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error reading coralogix_view_folder",
				fmt.Sprintf("Could not find view folder with name (%s)", data.Name.ValueString()),
			)
		}
		return
	}

	data = flattenViewFolder(match)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
