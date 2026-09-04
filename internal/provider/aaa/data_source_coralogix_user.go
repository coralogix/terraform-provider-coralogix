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
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSourceWithConfigure        = &UserDataSource{}
	_ datasource.DataSourceWithConfigValidators = &UserDataSource{}
)

func NewUserDataSource() datasource.DataSource {
	return &UserDataSource{}
}

type UserDataSource struct {
	clients *userClients
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

	d.clients = newUserClients(clientSet)
}

func (d *UserDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r UserResource
	var resourceResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resourceResp)

	resp.Schema = utils.FrameworkDatasourceSchemaFromFrameworkResourceSchema(resourceResp.Schema)

	resp.Schema.Attributes["id"] = schema.StringAttribute{
		Optional: true,
		Computed: true,
		Validators: []validator.String{
			stringvalidator.LengthAtLeast(1),
		},
		MarkdownDescription: "User ID. Exactly one of `id` or `user_name` must be set.",
	}

	resp.Schema.Attributes["user_name"] = schema.StringAttribute{
		Optional: true,
		Computed: true,
		Validators: []validator.String{
			stringvalidator.LengthAtLeast(1),
		},
		MarkdownDescription: "User name (email). Exactly one of `id` or `user_name` " +
			"must be set. The lookup is case-insensitive, since SSO login can normalize letter " +
			"case in the backend.",
	}
}

func (d *UserDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(
			path.MatchRoot("id"),
			path.MatchRoot("user_name"),
		),
	}
}

func (d *UserDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data *UserResourceModel
	diags := req.Config.Get(ctx, &data)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	configuredUserName := data.UserName
	idSet, userNameSet := isKnownString(data.ID), isKnownString(data.UserName)
	switch {
	case idSet && userNameSet:
		resp.Diagnostics.AddError(
			"Exactly one of id or user_name must be set",
			"Both were set by the time the User data source was read. Remove one of them — "+
				"the two are alternative lookup keys, not filters to combine.",
		)
		return
	case idSet:
		data, diags = d.userByID(ctx, data.ID.ValueString())
	case userNameSet:
		data, diags = d.userByUserName(ctx, data.UserName.ValueString())
	default:
		resp.Diagnostics.AddError("User id or user_name must be set", "")
		return
	}
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	data.UserName = preserveUserNameCase(configuredUserName, data.UserName)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (d *UserDataSource) userByID(ctx context.Context, id string) (*UserResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	log.Printf("[INFO] Reading User: %s", id)

	// The data source has no state, so there is no username to narrow the search with.
	state, err := readUser(ctx, d.clients, id, "")
	if err != nil {
		if isUserNotFoundErr(err) {
			diags.AddError(fmt.Sprintf("User %q not found", id), "")
		} else {
			diags.AddError("Error reading User", err.Error())
		}
		return nil, diags
	}

	return state, diags
}

func (d *UserDataSource) userByUserName(ctx context.Context, userName string) (*UserResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	log.Printf("[INFO] Searching Users to find by user name: %s", userName)

	teamID, err := d.clients.teamID(ctx)
	if err != nil {
		diags.AddError("Error reading User", teamIDErrorDetail(err))
		return nil, diags
	}

	matches, err := findUsersByUsername(ctx, d.clients.users, teamID, userName)
	if err != nil {
		diags.AddError("Error searching Users", err.Error())
		return nil, diags
	}

	switch len(matches) {
	case 0:
		diags.AddError(fmt.Sprintf("User with user_name %q not found", userName), "")
		return nil, diags
	case 1:
		if matches[0].GetUserId() == "" {
			diags.AddError(
				fmt.Sprintf("User with user_name %q was returned without an id", userName),
				"Look the user up by id instead, or report this to the provider developers.",
			)
			return nil, diags
		}
	default:
		diags.AddError(
			fmt.Sprintf("Multiple Users found with user_name %q", userName),
			fmt.Sprintf("Matched user ids: %s. Look the user up by id instead.", strings.Join(userIDs(matches), ", ")),
		)
		return nil, diags
	}

	state, err := flattenUserWithGroups(ctx, d.clients, teamID, &matches[0])
	if err != nil {
		diags.AddError("Error reading User", err.Error())
		return nil, diags
	}

	return state, diags
}

func isKnownString(value types.String) bool {
	return !value.IsNull() && !value.IsUnknown()
}
