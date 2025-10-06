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

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var _ datasource.DataSourceWithConfigure = &GlobalRouterDataSource{}

func NewGlobalRouterDataSource() datasource.DataSource {
	return &GlobalRouterDataSource{}
}

type GlobalRouterDataSource struct {
	client *cxsdk.NotificationsClient
}

func (d *GlobalRouterDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_global_router"
}

func (d *GlobalRouterDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = clientSet.GetNotifications()
}

func (d *GlobalRouterDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r GlobalRouterResource
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

func (d *GlobalRouterDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data *GlobalRouterResourceModel
	diags := req.Config.Get(ctx, &data)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	var globalRouterID string
	//Get refreshed GlobalRouter value from Coralogix
	if name := data.Name.ValueString(); name != "" {
		log.Printf("[INFO] Listing GlobalRouters to find by name: %s", name)
		listGlobalRouterResp, err := d.client.ListGlobalRouters(ctx, &cxsdk.ListGlobalRoutersRequest{})
		if err != nil {
			log.Printf("[ERROR] Received error when listing globalRouters: %s", err.Error())
			listGlobalRouterReqStr, _ := json.Marshal(listGlobalRouterResp)
			resp.Diagnostics.AddError(
				"Error listing GlobalRouters",
				utils.FormatRpcErrors(err, "List", string(listGlobalRouterReqStr)),
			)
			return
		}

		for _, globalRouter := range listGlobalRouterResp.Routers {
			if globalRouter.Name == data.Name.ValueString() {
				globalRouterID = *globalRouter.Id
				break
			}
		}

		if globalRouterID == "" {
			resp.Diagnostics.AddError(fmt.Sprintf("globalRouter with name %q not found", name), "")
			return
		}
	} else if id := data.ID.ValueString(); id != "" {
		globalRouterID = id
	} else {
		resp.Diagnostics.AddError("globalRouter ID or name must be set", "")
		return
	}

	getGlobalRouterResp, err := d.client.GetGlobalRouter(ctx, &cxsdk.GetGlobalRouterRequest{Id: globalRouterID})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get globalRouter", err.Error())
		return
	}
	if getGlobalRouterResp == nil {
		resp.Diagnostics.AddError("globalRouter not found", "globalRouter not found")
		return
	}

	data, diags = flattenGlobalRouter(ctx, getGlobalRouterResp.Router)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
