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
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ datasource.DataSourceWithConfigure = &UserDataSource{}

func NewUserDataSource() datasource.DataSource {
	return &UserDataSource{}
}

type UserDataSource struct {
	client *clientset.UsersClient
}

func (d *UserDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *UserDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = clientSet.Users()
}

func (d *UserDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r UserResource
	var resourceResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resourceResp)

	resp.Schema = utils.FrameworkDatasourceSchemaFromFrameworkResourceSchema(resourceResp.Schema)

	if idAttr, ok := resp.Schema.Attributes["id"].(schema.StringAttribute); ok {
		idAttr.Required = false
		idAttr.Optional = true
		idAttr.Computed = true
		idAttr.Validators = []validator.String{
			stringvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("user_name")),
		}
		idAttr.MarkdownDescription = "User ID. Exactly one of `id` or `user_name` must be set."
		resp.Schema.Attributes["id"] = idAttr
	}

	if userNameAttr, ok := resp.Schema.Attributes["user_name"].(schema.StringAttribute); ok {
		userNameAttr.Optional = true
		userNameAttr.MarkdownDescription = "User name (email). Exactly one of `id` or `user_name` " +
			"must be set. The lookup is case-insensitive, since SSO login can normalize letter " +
			"case in the backend."
		resp.Schema.Attributes["user_name"] = userNameAttr
	}
}

func (d *UserDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data *UserResourceModel
	diags := req.Config.Get(ctx, &data)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	var user *cxsdk.SCIMUser
	configuredUserName := data.UserName
	switch {
	case isKnownString(data.ID):
		user, diags = d.userByID(ctx, data.ID.ValueString())
	case isKnownString(data.UserName):
		user, diags = d.userByUserName(ctx, data.UserName.ValueString())
	default:
		resp.Diagnostics.AddError("User id or user_name must be set", "")
		return
	}
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	respStr, _ := json.Marshal(user)
	log.Printf("[INFO] Received User: %s", string(respStr))

	data, diags = flattenSCIMUser(ctx, user)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	if isKnownString(configuredUserName) && strings.EqualFold(configuredUserName.ValueString(), data.UserName.ValueString()) {
		data.UserName = configuredUserName
	}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (d *UserDataSource) userByID(ctx context.Context, id string) (*cxsdk.SCIMUser, diag.Diagnostics) {
	var diags diag.Diagnostics
	log.Printf("[INFO] Reading User: %s", id)
	getUserResp, err := d.client.Get(ctx, id)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.NotFound {
			diags.AddError(fmt.Sprintf("User %q not found", id), "")
		} else {
			diags.AddError(
				"Error reading User",
				utils.FormatRpcErrors(err, fmt.Sprintf("%s/%s", d.client.BaseURL(), id), ""),
			)
		}
		return nil, diags
	}

	return getUserResp, diags
}

func (d *UserDataSource) userByUserName(ctx context.Context, userName string) (*cxsdk.SCIMUser, diag.Diagnostics) {
	var diags diag.Diagnostics
	log.Printf("[INFO] Listing Users to find by user name: %s", userName)
	users, err := d.client.ListByUserName(ctx, userName)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		diags.AddError(
			"Error listing Users",
			utils.FormatRpcErrors(err, d.client.BaseURL(), fmt.Sprintf("user_name: %s", userName)),
		)
		return nil, diags
	}

	matches := matchSCIMUsersByUserName(users, userName)
	switch len(matches) {
	case 0:
		diags.AddError(fmt.Sprintf("User with user_name %q not found", userName), "")
		return nil, diags
	case 1:
		if matches[0].ID == nil {
			diags.AddError(
				fmt.Sprintf("User with user_name %q was returned without an id", userName),
				"Look the user up by id instead, or report this to the provider developers.",
			)
			return nil, diags
		}
		return &matches[0], diags
	default:
		diags.AddError(
			fmt.Sprintf("Multiple Users found with user_name %q", userName),
			fmt.Sprintf("Matched user ids: %s. Look the user up by id instead.", strings.Join(scimUserIDs(matches), ", ")),
		)
		return nil, diags
	}
}

func matchSCIMUsersByUserName(users []cxsdk.SCIMUser, userName string) []cxsdk.SCIMUser {
	matches := make([]cxsdk.SCIMUser, 0, len(users))
	for _, user := range users {
		if strings.EqualFold(user.UserName, userName) {
			matches = append(matches, user)
		}
	}
	return matches
}

func isKnownString(value types.String) bool {
	return !value.IsNull() && !value.IsUnknown()
}

func scimUserIDs(users []cxsdk.SCIMUser) []string {
	ids := make([]string, 0, len(users))
	for _, user := range users {
		if user.ID != nil {
			ids = append(ids, *user.ID)
		}
	}
	return ids
}
