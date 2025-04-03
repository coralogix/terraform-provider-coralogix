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
	"terraform-provider-coralogix/coralogix/clientset"
	"terraform-provider-coralogix/coralogix/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var _ datasource.DataSourceWithConfigure = &GroupDataSource{}

func NewGroupDataSource() datasource.DataSource {
	return &GroupDataSource{}
}

type GroupDataSource struct {
	client     *clientset.GroupsClient
	grpcClient *cxsdk.GroupsClient
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

	d.client = clientSet.Groups()
	d.grpcClient = clientSet.GroupGrpc()
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

	var groupID string
	//Get refreshed Group value from Coralogix
	if displayName := data.DisplayName.ValueString(); displayName != "" {
		log.Printf("[INFO] Listing Groups to find by display name: %s", displayName)
		listGroupReq := &cxsdk.GetTeamGroupsRequest{}
		listGroupResp, err := d.grpcClient.List(ctx, listGroupReq)
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
			if group.Name == data.DisplayName.ValueString() {
				groupID = strconv.Itoa(int(group.GroupId.Id))
				break
			}
		}

		if groupID == "" {
			resp.Diagnostics.AddError(fmt.Sprintf("Group with display name %q not found", displayName), "")
			return
		}
	} else if id := data.ID.ValueString(); id != "" {
		groupID = id
	} else {
		resp.Diagnostics.AddError("Group ID or display name must be set", "")
		return
	}

	getGroupResp, err := d.client.GetGroup(ctx, groupID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get group", err.Error())
		return
	}
	if getGroupResp == nil {
		resp.Diagnostics.AddError("Group not found", "Group not found")
		return
	}

	data, diags = flattenSCIMGroup(getGroupResp)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
