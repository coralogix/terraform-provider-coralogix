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
	"encoding/json"
	"fmt"
	"log"

	"terraform-provider-coralogix/coralogix/clientset"
	"terraform-provider-coralogix/coralogix/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"google.golang.org/grpc/codes"
)

var _ datasource.DataSourceWithConfigure = &ViewDataSource{}

func NewViewDataSource() datasource.DataSource {
	return &ViewDataSource{}
}

type ViewDataSource struct {
	client *cxsdk.ViewsClient
}

func (d *ViewDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_view"
}

func (d *ViewDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = clientSet.Views()
}

func (d *ViewDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r ViewResource
	var resourceResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resourceResp)

	resp.Schema = utils.FrameworkDatasourceSchemaFromFrameworkResourceSchema(resourceResp.Schema)
}

func (d *ViewDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data *ViewModel
	diags := req.Config.Get(ctx, &data)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	//Get refreshed View value from Coralogix
	id := data.Id.ValueInt64()
	log.Printf("[INFO] Reading view: %d", id)
	getViewResp, err := d.client.Get(ctx, &cxsdk.GetViewRequest{Id: utils.TypeInt64ToWrappedInt32(data.Id)})
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				err.Error(),
				fmt.Sprintf("View %q is in state, but no longer exists in Coralogix backend", id),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error reading View",
				utils.FormatRpcErrors(err, fmt.Sprintf("%s/%d", cxsdk.GetViewRPC, id), ""),
			)
		}
		return
	}
	respStr, _ := json.Marshal(getViewResp)
	log.Printf("[INFO] Received View: %s", string(respStr))

	data, diags = flattenView(ctx, getViewResp.View)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
