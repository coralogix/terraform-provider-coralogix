// Copyright 2025 Coralogix Ltd.
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

package data_exploration

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/coralogix/terraform-provider-coralogix/coralogix/clientset"
	"github.com/coralogix/terraform-provider-coralogix/coralogix/utils"

	"google.golang.org/protobuf/types/known/wrapperspb"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"google.golang.org/grpc/codes"
)

var _ datasource.DataSourceWithConfigure = &ViewsFolderDataSource{}

func NewViewsFolderDataSource() datasource.DataSource {
	return &ViewsFolderDataSource{}
}

type ViewsFolderDataSource struct {
	client *cxsdk.ViewFoldersClient
}

func (d *ViewsFolderDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_views_folder"
}

func (d *ViewsFolderDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ViewsFolderDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r ViewsFolderResource
	var resourceResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resourceResp)

	resp.Schema = utils.FrameworkDatasourceSchemaFromFrameworkResourceSchema(resourceResp.Schema)
}

func (d *ViewsFolderDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data *ViewsFolderModel
	diags := req.Config.Get(ctx, &data)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	//Get refreshed Views-Folder value from Coralogix
	id := data.Id.ValueString()
	log.Printf("[INFO] Reading views-folder: %s", id)
	getViewsFolderResp, err := d.client.Get(ctx, &cxsdk.GetViewFolderRequest{Id: wrapperspb.String(id)})
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				err.Error(),
				fmt.Sprintf("Views-Folder %q is in state, but no longer exists in Coralogix backend", id),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Views-Folder",
				utils.FormatRpcErrors(err, fmt.Sprintf("%s/%s", cxsdk.GetViewFolderRPC, id), ""),
			)
		}
		return
	}
	respStr, _ := json.Marshal(getViewsFolderResp)
	log.Printf("[INFO] Received View: %s", string(respStr))

	data = flattenViewsFolder(getViewsFolderResp.Folder)

	// Set state to fully populated data
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
