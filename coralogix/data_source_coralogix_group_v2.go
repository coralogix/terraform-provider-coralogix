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
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"google.golang.org/grpc/codes"
	"terraform-provider-coralogix/coralogix/clientset"
	"terraform-provider-coralogix/coralogix/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var _ datasource.DataSourceWithConfigure = &GroupDataSource{}

func NewGroupV2DataSource() datasource.DataSource {
	return &GroupV2DataSource{}
}

type GroupV2DataSource struct {
	client *cxsdk.GroupsClient
}

func (d *GroupV2DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_v2"
}

func (d *GroupV2DataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = clientSet.GroupGrpc()
}

func (d *GroupV2DataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r GroupV2Resource
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

func (d *GroupV2DataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data *GroupV2ResourceModel
	diags := req.Config.Get(ctx, &data)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	var groupID string
	//Get refreshed Group value from Coralogix
	if name := data.Name.ValueString(); name != "" {
		log.Printf("[INFO] Listing Groups to find by name: %s", name)
		listGroupReq := &cxsdk.GetTeamGroupsRequest{}
		listGroupResp, err := d.client.List(ctx, listGroupReq)
		if err != nil {
			log.Printf("[ERROR] Received error when listing groups: %s", err.Error())
			listGroupReqStr, _ := json.Marshal(listGroupReq)
			resp.Diagnostics.AddError(
				"Error listing Groups",
				utils.FormatRpcErrors(err, "List", string(listGroupReqStr)),
			)
			return
		}

		for _, group := range listGroupResp.Groups {
			if group.Name == data.Name.ValueString() {
				groupID = strconv.Itoa(int(group.GroupId.Id))
				break
			}
		}

		if groupID == "" {
			resp.Diagnostics.AddError(fmt.Sprintf("Group with name %q not found", name), "")
			return
		}
	} else if id := data.ID.ValueString(); id != "" {
		groupID = id
	} else {
		resp.Diagnostics.AddError("Group ID or display name must be set", "")
		return
	}

	id, err := strconv.Atoi(groupID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Group ID", fmt.Sprintf("Group ID must be an integer, got: %s", groupID))
		return
	}
	groupId := &cxsdk.TeamGroupID{Id: uint32(id)}
	getGroupResp, err := d.client.Get(ctx, &cxsdk.GetTeamGroupRequest{GroupId: groupId})
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				err.Error(),
				fmt.Sprintf("Group %q is in state, but no longer exists in Coralogix backend", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Group",
				utils.FormatRpcErrors(err, "Get", fmt.Sprintf("Group ID: %d", id)),
			)
		}
		resp.Diagnostics.AddError("Failed to get group", err.Error())
		return
	}

	users, diag := getGroupUsers(ctx, d.client, groupId)
	if diag != nil {
		resp.Diagnostics.Append(diag)
		return
	}

	data, diags = flattenGroupV2(ctx, getGroupResp.Group, users)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
