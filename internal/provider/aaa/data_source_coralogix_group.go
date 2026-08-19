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

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	teamGroups "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/team_groups_management_service"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var _ datasource.DataSourceWithConfigure = &GroupDataSource{}

func NewGroupDataSource() datasource.DataSource {
	return &GroupDataSource{}
}

type GroupDataSource struct {
	client *teamGroups.TeamGroupsManagementServiceAPIService
}

func (d *GroupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (d *GroupDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = clientSet.TeamGroups()
}

func (d *GroupDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r GroupResource
	var resourceResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resourceResp)

	resp.Schema = utils.FrameworkDatasourceSchemaFromFrameworkResourceSchema(resourceResp.Schema)

	if idAttr, ok := resp.Schema.Attributes["id"].(schema.StringAttribute); ok {
		idAttr.Required = false
		idAttr.Optional = true
		idAttr.Validators = []validator.String{
			stringvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("display_name")),
		}
		resp.Schema.Attributes["id"] = idAttr
	}

	if nameAttr, ok := resp.Schema.Attributes["display_name"].(schema.StringAttribute); ok {
		nameAttr.Required = false
		nameAttr.Optional = true
		resp.Schema.Attributes["display_name"] = nameAttr
	}
}

func (d *GroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data *GroupResourceModel
	diags := req.Config.Get(ctx, &data)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	group, diags := d.getTeamGroup(ctx, data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	memberIDs, httpResp, err := listGroupUserIDs(ctx, d.client, *group.GroupId)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading Group members",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResp, err), "GetGroupUsers", *group.GroupId),
		)
		return
	}

	data, diags = flattenTeamGroupWithPreferredRole(group, memberIDs, data.Role.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (d *GroupDataSource) getTeamGroup(ctx context.Context, data *GroupResourceModel) (*teamGroups.TeamGroup, diag.Diagnostics) {
	var diags diag.Diagnostics

	if displayName := data.DisplayName.ValueString(); displayName != "" {
		log.Printf("[INFO] Getting Group by display name: %s", displayName)
		getByNameResp, httpResponse, err := d.client.
			GroupsMgmtServiceGetTeamGroupByName(ctx, displayName).
			Execute()
		if err != nil {
			apiErr := cxsdkOpenapi.NewAPIError(httpResponse, err)
			if cxsdkOpenapi.IsNotFound(apiErr) {
				diags.AddError(fmt.Sprintf("Group with display name %q not found", displayName), "")
			} else {
				diags.AddError(
					"Error listing Groups",
					utils.FormatOpenAPIErrors(apiErr, "GetTeamGroupByName", displayName),
				)
			}
			return nil, diags
		}
		if getByNameResp == nil || getByNameResp.Group == nil || getByNameResp.Group.GroupId == nil {
			diags.AddError(fmt.Sprintf("Group with display name %q not found", displayName), "")
			return nil, diags
		}
		return getByNameResp.Group, diags
	}

	if id := data.ID.ValueString(); id != "" {
		groupID, parseDiags := parseGroupID(id)
		diags.Append(parseDiags...)
		if diags.HasError() {
			return nil, diags
		}
		log.Printf("[INFO] Getting Group: %d", groupID)
		getResp, httpResponse, err := d.client.GroupsMgmtServiceGetTeamGroup(ctx, groupID).Execute()
		if err != nil {
			apiErr := cxsdkOpenapi.NewAPIError(httpResponse, err)
			if cxsdkOpenapi.IsNotFound(apiErr) {
				diags.AddError("Group not found", fmt.Sprintf("Group %q not found", id))
			} else {
				diags.AddError("Error reading Group", utils.FormatOpenAPIErrors(apiErr, "GetTeamGroup", id))
			}
			return nil, diags
		}
		if getResp == nil || getResp.Group == nil {
			diags.AddError("Group not found", fmt.Sprintf("Group %q not found", id))
			return nil, diags
		}
		return getResp.Group, diags
	}

	diags.AddError("Group ID or display name must be set", "")
	return nil, diags
}
