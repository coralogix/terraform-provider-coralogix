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

package fleet

import (
	"context"
	"fmt"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	cfggroups "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/fleet_manager_configuration_groups"
	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/configurationgroup"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var _ datasource.DataSourceWithConfigure = &FleetConfigurationGroupDataSource{}

func NewFleetConfigurationGroupDataSource() datasource.DataSource {
	return &FleetConfigurationGroupDataSource{}
}

// FleetConfigurationGroupDataSource reads a configuration group with the
// schema and conversion of the generated resource.
type FleetConfigurationGroupDataSource struct {
	client *cfggroups.FleetManagerConfigurationGroupsAPIService
}

func (d *FleetConfigurationGroupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + configurationgroup.TypeName
}

func (d *FleetConfigurationGroupDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	clientSet, ok := req.ProviderData.(*clientset.ClientSet)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *clientset.ClientSet, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	d.client = clientSet.ConfigurationGroups()
}

func (d *FleetConfigurationGroupDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var resourceResp resource.SchemaResponse
	configurationgroup.NewResource(configurationgroup.Hooks{}).Schema(ctx, resource.SchemaRequest{}, &resourceResp)
	resp.Schema = utils.FrameworkDatasourceSchemaFromFrameworkResourceSchema(resourceResp.Schema)

	if idAttr, ok := resp.Schema.Attributes["id"].(schema.StringAttribute); ok {
		idAttr.Required = true
		idAttr.Optional = false
		idAttr.Computed = false
		resp.Schema.Attributes["id"] = idAttr
	}
}

func (d *FleetConfigurationGroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data *configurationgroup.ConfigurationGroupModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, httpResponse, err := d.client.ConfigurationGroupServiceGetConfigurationGroup(ctx, data.Id.ValueString()).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error reading coralogix_"+configurationgroup.TypeName,
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
		)
		return
	}

	data, diags := configurationgroup.Flatten(ctx, result)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}
