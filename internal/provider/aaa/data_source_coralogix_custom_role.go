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
	"log"
	"strconv"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"google.golang.org/grpc/codes"
)

var _ datasource.DataSourceWithConfigure = &CustomRoleDataSource{}

func NewCustomRoleDataSource() datasource.DataSource {
	return &CustomRoleDataSource{}
}

type CustomRoleDataSource struct {
	client *cxsdk.RolesClient
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
	log.Printf("[INFO] Reading Custom Role")

	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed Action value from Coralogix
	var customRole *cxsdk.CustomRole
	if id := data.ID.ValueString(); id != "" {
		roleId, err := strconv.Atoi(id)
		if err != nil {
			resp.Diagnostics.AddError("Invalid Id", "Custom role id must be an int")
			return
		}
		log.Printf("[INFO] Reading Custom Role: %v", roleId)
		customRole, err = getRoleById(ctx, resp, d.client, roleId)
		if err != nil {
			resp.Diagnostics.AddError("Error reading custom role", err.Error())
			return
		}
	} else if name := data.Name.ValueString(); name != "" {
		var err error
		log.Printf("[INFO] Reading Custom Role: %v", name)
		customRole, err = getRoleByName(ctx, resp, d.client, name)
		if err != nil {
			resp.Diagnostics.AddError("Error reading custom role", err.Error())
			return
		}
	} else {
		resp.Diagnostics.AddError("Invalid Id or Name", "Custom role id or name must be provided")
		return
	}

	model, diags := flattenCustomRole(ctx, customRole)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func getRoleById(ctx context.Context, resp *datasource.ReadResponse, client *cxsdk.RolesClient, roleId int) (*cxsdk.CustomRole, error) {
	getCustomRoleRequest := &cxsdk.GetCustomRoleRequest{
		RoleId: uint32(roleId),
	}
	createCustomRoleResponse, err := client.Get(ctx, getCustomRoleRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				err.Error(),
				fmt.Sprintf("Custom role  %q is in state, but no longer exists in Coralogix backend", roleId),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error reading custom role",
				utils.FormatRpcErrors(err, cxsdk.RolesGetCustomRoleRPC, protojson.Format(getCustomRoleRequest)),
			)
		}
		return nil, err
	}
	log.Printf("[INFO] Received Custom Role: %s", protojson.Format(createCustomRoleResponse))
	return createCustomRoleResponse.GetRole(), nil
}

func getRoleByName(ctx context.Context, resp *datasource.ReadResponse, client *cxsdk.RolesClient, roleName string) (*cxsdk.CustomRole, error) {
	listCustomRolesRequest := &cxsdk.ListCustomRolesRequest{}
	listCustomRolesResponse, err := client.List(ctx, listCustomRolesRequest)
	if err != nil {
		log.Printf("[ERROR] Received error while listing custom roles: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error listing custom roles",
			utils.FormatRpcErrors(err, cxsdk.RolesListCustomRolesRPC, protojson.Format(listCustomRolesRequest)),
		)
		return nil, err
	}

	var found bool
	var result *cxsdk.CustomRole
	for _, role := range listCustomRolesResponse.GetRoles() {
		if role.GetName() == roleName {
			if found {
				resp.Diagnostics.AddError(
					"Multiple custom roles found",
					fmt.Sprintf("Multiple custom roles found with name %q", roleName),
				)
				return nil, fmt.Errorf("multiple custom roles found with name %q", roleName)
			}
			found = true
			result = role
		}
	}

	if !found {
		resp.Diagnostics.AddError(
			"Custom role not found",
			fmt.Sprintf("Custom role %q not found", roleName),
		)
		return nil, fmt.Errorf("custom role %q not found", roleName)
	}

	log.Printf("[INFO] Received Custom Role: %s", protojson.Format(result))
	return result, nil
}
