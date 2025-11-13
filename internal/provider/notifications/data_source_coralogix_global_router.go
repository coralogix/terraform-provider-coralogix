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

package notifications

import (
	"context"
	"fmt"
	"log"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	globalRouters "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/global_routers_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var _ datasource.DataSourceWithConfigure = &GlobalRouterDataSource{}

func NewGlobalRouterDataSource() datasource.DataSource {
	return &GlobalRouterDataSource{}
}

type GlobalRouterDataSource struct {
	client *globalRouters.GlobalRoutersServiceAPIService
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

	_, d.client, _ = clientSet.GetNotifications()
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

	var routerID string
	//Get refreshed connector value from Coralogix
	if name := data.Name.ValueString(); name != "" {
		log.Printf("[INFO] Listing coralogix_global_router to find by name: %s", name)
		listResult, httpResponse, err := d.client.
			GlobalRoutersServiceListGlobalRouters(ctx).
			Execute()

		if err != nil {
			resp.Diagnostics.AddError(
				"Error listing coralogix_global_router",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "List", nil),
			)
			return
		}

		for _, router := range listResult.Routers {
			if *router.Name == data.Name.ValueString() {
				routerID = *router.Id
				break
			}
		}

		if routerID == "" {
			resp.Diagnostics.AddError(fmt.Sprintf("coralogix_global_router with name %q not found", name), "")
			return
		}
	} else if id := data.ID.ValueString(); id != "" {
		routerID = id
	} else {
		resp.Diagnostics.AddError("ID or name must be set", "")
		return
	}
	rq := d.client.GlobalRoutersServiceGetGlobalRouter(ctx, routerID)

	log.Printf("[INFO] Reading coralogix_global_router: %s", utils.FormatJSON(rq))

	result, httpResponse, err := rq.
		Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error reading coralogix_global_router",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
		)
		return
	}

	log.Printf("[INFO] Read coralogix_global_router: %s", utils.FormatJSON(result))

	data, diags = flattenGlobalRouter(ctx, result.Router)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
