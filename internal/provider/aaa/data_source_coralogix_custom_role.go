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

package aaa

import (
	"context"
	"fmt"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	roless "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/role_management_service"
)

var _ datasource.DataSourceWithConfigure = &CustomRoleDataSource{}

func NewCustomRoleDataSource() datasource.DataSource {
	return &CustomRoleDataSource{}
}

type CustomRoleDataSource struct {
	client *roless.RoleManagementServiceAPIService
}

func (d *CustomRoleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_role"
}

func (d *CustomRoleDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = clientSet.CustomRoles()
}

func (d *CustomRoleDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r CustomRoleSource
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

func (d *CustomRoleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data *RolesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	id, diags := utils.TypeStringToInt64Pointer(data.ID)

	var customRole *roless.CustomRole
	if !diags.HasError() {
		customRole = getRoleById(ctx, resp, d.client, *id)
	} else if name := data.Name.ValueString(); name != "" {
		customRole = getRoleByName(ctx, resp, d.client, name)
	} else {
		resp.Diagnostics.AddError("Invalid Id or Name", "coralogix_custom_role id or name must be provided")
	}
	if resp.Diagnostics.HasError() {
		return
	}

	model, err := flattenCustomRole(customRole, data)
	if err != nil {
		resp.Diagnostics.AddError("Error flattening coralogix_custom_role during read", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func getRoleById(ctx context.Context, resp *datasource.ReadResponse, client *roless.RoleManagementServiceAPIService, id int64) *roless.CustomRole {
	rq := client.
		RoleManagementServiceGetCustomRole(ctx, id)

	result, httpResponse, err := rq.
		Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error reading coralogix_custom_role", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil))
		return nil
	}
	return result.Role
}

func getRoleByName(ctx context.Context, resp *datasource.ReadResponse, client *roless.RoleManagementServiceAPIService, roleName string) *roless.CustomRole {

	result, httpResponse, err := client.RoleManagementServiceListCustomRoles(ctx).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error reading coralogix_custom_role", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil))
		return nil
	}
	var found bool
	var foundRole *roless.CustomRole
	for _, role := range result.GetRoles() {
		if role.GetName() == roleName {
			if found {
				resp.Diagnostics.AddError(
					"Multiple coralogix_custom_roles found",
					fmt.Sprintf("Multiple coralogix_custom_roles found with name %q", roleName),
				)
				return nil
			}
			found = true
			foundRole = &role
		}
	}

	if !found {
		resp.Diagnostics.AddError(
			"coralogix_custom_role not found",
			fmt.Sprintf("coralogix_custom_role %q not found", roleName),
		)
		return nil
	}

	return foundRole
}
