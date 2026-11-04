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

package aaa

import (
	"context"
	"fmt"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	generatedipaccess "github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/ipaccess"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	ipaccess "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/ip_access_service"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

var _ datasource.DataSourceWithConfigure = &IpAccessDataSource{}

type IpAccessDataSource struct {
	client *ipaccess.IPAccessServiceAPIService
}

func (r *IpAccessDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ip_access"
}

func (r *IpAccessDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resourceSchema := generatedipaccess.Schema()
	attributes := utils.ConvertAttributes(resourceSchema.Attributes)
	if idSchema, ok := resourceSchema.Attributes["id"]; ok {
		attributes["id"] = datasourceschema.StringAttribute{
			Optional:            true,
			Description:         idSchema.GetDescription(),
			MarkdownDescription: idSchema.GetMarkdownDescription(),
		}
	}
	resp.Schema = datasourceschema.Schema{
		Attributes:          attributes,
		Description:         resourceSchema.Description,
		MarkdownDescription: resourceSchema.MarkdownDescription,
		DeprecationMessage:  resourceSchema.DeprecationMessage,
	}
}

func (r *IpAccessDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Decode the full data-source model because its schema includes the resource
	// attributes as computed outputs in addition to the optional lookup id.
	var data generatedipaccess.CompanyIpAccessSettingsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
	rq := r.client.
		IpAccessServiceGetCompanyIpAccessSettings(ctx)
	if !data.Id.IsNull() && !data.Id.IsUnknown() {
		rq = rq.Id(data.Id.ValueString())
	}

	result, httpResponse, err := rq.Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error reading coralogix_ip_access",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
		)
		return
	}

	state, diags := generatedipaccess.Flatten(ctx, result.Settings)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *IpAccessDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	r.client = clientSet.IpAccess()
}

func NewIpAccessDataSource() datasource.DataSource {
	return &IpAccessDataSource{}
}
