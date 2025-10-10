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

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/grpc/codes"
)

func NewUserResource() resource.Resource {
	return &UserResource{}
}

type UserResource struct {
	client *cxsdk.UsersClient
}

func (r *UserResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *UserResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.Users()
}

func (r *UserResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "User ID.",
			},
			"user_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "User name.",
			},
			"name": schema.SingleNestedAttribute{
				Optional: true,
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"given_name": schema.StringAttribute{
						Optional: true,
					},
					"family_name": schema.StringAttribute{
						Optional: true,
					},
				},
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
			},
			"active": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"emails": schema.SetNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"primary": schema.BoolAttribute{
							Computed: true,
							PlanModifiers: []planmodifier.Bool{
								boolplanmodifier.UseStateForUnknown(),
							},
						},
						"value": schema.StringAttribute{
							Computed: true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"type": schema.StringAttribute{
							Computed: true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
					},
					PlanModifiers: []planmodifier.Object{
						objectplanmodifier.UseStateForUnknown(),
					},
				},
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.UseStateForUnknown(),
				},
			},
			"groups": schema.SetAttribute{
				Computed:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.UseStateForUnknown(),
				},
			},
		},
		MarkdownDescription: "Coralogix User.",
	}
}

func (r *UserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *UserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *UserResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createUserRequest, diags := extractCreateUser(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	userStr, _ := json.Marshal(createUserRequest)
	log.Printf("[INFO] Creating new User: %s", string(userStr))
	createResp, err := r.client.Create(ctx, createUserRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error creating User",
			utils.FormatRpcErrors(err, r.client.BaseURL(), string(userStr)),
		)
		return
	}
	userStr, _ = json.Marshal(createResp)
	log.Printf("[INFO] Submitted new User: %s", userStr)

	state, diags := flattenSCIMUser(ctx, createResp)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func flattenSCIMUser(ctx context.Context, user *cxsdk.SCIMUser) (*UserResourceModel, diag.Diagnostics) {
	name, diags := flattenSCIMUserName(ctx, user.Name)
	if diags.HasError() {
		return nil, diags
	}

	emails, diags := flattenSCIMUserEmails(ctx, user.Emails)
	if diags.HasError() {
		return nil, diags
	}

	groups, diags := flattenSCIMUserGroups(ctx, user.Groups)
	if diags.HasError() {
		return nil, diags
	}

	return &UserResourceModel{
		ID:       types.StringValue(*user.ID),
		UserName: types.StringValue(user.UserName),
		Name:     name,
		Active:   types.BoolValue(user.Active),
		Emails:   emails,
		Groups:   groups,
	}, nil
}

func flattenSCIMUserEmails(ctx context.Context, emails []cxsdk.SCIMUserEmail) (types.Set, diag.Diagnostics) {
	emailsIDs := make([]UserEmailModel, 0, len(emails))
	for _, email := range emails {
		emailModel := UserEmailModel{
			Primary: types.BoolValue(email.Primary),
			Value:   types.StringValue(email.Value),
			Type:    types.StringValue(email.Type),
		}
		emailsIDs = append(emailsIDs, emailModel)
	}
	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: SCIMUserEmailAttr()}, emailsIDs)
}

func SCIMUserEmailAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"primary": types.BoolType,
		"value":   types.StringType,
		"type":    types.StringType,
	}
}

func flattenSCIMUserName(ctx context.Context, name *cxsdk.SCIMUserName) (types.Object, diag.Diagnostics) {
	if name == nil {
		return types.ObjectNull(sCIMUserNameAttr()), nil
	}
	return types.ObjectValueFrom(ctx, sCIMUserNameAttr(), &UserNameModel{
		GivenName:  types.StringValue(name.GivenName),
		FamilyName: types.StringValue(name.FamilyName),
	})
}

func sCIMUserNameAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"given_name":  types.StringType,
		"family_name": types.StringType,
	}
}

func flattenSCIMUserGroups(ctx context.Context, groups []cxsdk.SCIMUserGroup) (types.Set, diag.Diagnostics) {
	groupsIDs := make([]string, 0, len(groups))
	for _, group := range groups {
		groupsIDs = append(groupsIDs, group.Value)
	}
	return types.SetValueFrom(ctx, types.StringType, groupsIDs)
}

func (r *UserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *UserResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed User value from Coralogix
	id := state.ID.ValueString()
	getUserResp, err := r.client.Get(ctx, id)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("User %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading User",
				utils.FormatRpcErrors(err, fmt.Sprintf("%s/%s", r.client.BaseURL(), id), ""),
			)
		}
		return
	}
	respStr, _ := json.Marshal(getUserResp)
	log.Printf("[INFO] Received User: %s", string(respStr))

	state, diags = flattenSCIMUser(ctx, getUserResp)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *UserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan, state *UserResourceModel
	diags := req.Plan.Get(ctx, &plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	diags = req.State.Get(ctx, &state)
	if resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	if plan.UserName.ValueString() != state.UserName.ValueString() {
		resp.Diagnostics.AddError(
			"User name cannot be updated",
			"User name is immutable and cannot be updated",
		)
		return
	}

	userUpdateReq, diags := extractCreateUser(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	userStr, _ := json.Marshal(userUpdateReq)
	log.Printf("[INFO] Updating User: %s", string(userStr))
	userID := plan.ID.ValueString()
	userUpdateResp, err := r.client.Update(ctx, userID, userUpdateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating User",
			utils.FormatRpcErrors(err, fmt.Sprintf("%s/%s", r.client.BaseURL(), userID), string(userStr)),
		)
		return
	}
	userStr, _ = json.Marshal(userUpdateResp)
	log.Printf("[INFO] Submitted updated User: %s", string(userStr))

	// Get refreshed User value from Coralogix
	id := plan.ID.ValueString()
	getUserResp, err := r.client.Get(ctx, id)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("User %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading User",
				utils.FormatRpcErrors(err, fmt.Sprintf("%s/%s", r.client.BaseURL(), id), string(userStr)),
			)
		}
		return
	}
	userStr, _ = json.Marshal(getUserResp)
	log.Printf("[INFO] Received User: %s", string(userStr))

	state, diags = flattenSCIMUser(ctx, getUserResp)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *UserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state *UserResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	log.Printf("[INFO] Deleting User %s", id)
	if err := r.client.Delete(ctx, id); err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting User %s", id),
			utils.FormatRpcErrors(err, fmt.Sprintf("%s/%s", r.client.BaseURL(), id), ""),
		)
		return
	}
	log.Printf("[INFO] User %s deleted", id)
}

type UserResourceModel struct {
	ID       types.String `tfsdk:"id"`
	UserName types.String `tfsdk:"user_name"`
	Name     types.Object `tfsdk:"name"` //UserNameModel
	Active   types.Bool   `tfsdk:"active"`
	Emails   types.Set    `tfsdk:"emails"` //UserEmailModel
	Groups   types.Set    `tfsdk:"groups"` //types.String
}

type UserNameModel struct {
	GivenName  types.String `tfsdk:"given_name"`
	FamilyName types.String `tfsdk:"family_name"`
}

type UserEmailModel struct {
	Primary types.Bool   `tfsdk:"primary"`
	Value   types.String `tfsdk:"value"`
	Type    types.String `tfsdk:"type"`
}

func extractCreateUser(ctx context.Context, plan *UserResourceModel) (*cxsdk.SCIMUser, diag.Diagnostics) {
	name, diags := extractUserSCIMName(ctx, plan.Name)
	if diags.HasError() {
		return nil, diags
	}
	emails, diags := extractUserEmails(ctx, plan.Emails)
	if diags.HasError() {
		return nil, diags
	}
	groups, diags := extractUserGroups(ctx, plan.Groups)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.SCIMUser{
		Schemas:  []string{},
		UserName: plan.UserName.ValueString(),
		Name:     name,
		Active:   plan.Active.ValueBool(),
		Emails:   emails,
		Groups:   groups,
	}, nil
}

func extractUserGroups(ctx context.Context, groups types.Set) ([]cxsdk.SCIMUserGroup, diag.Diagnostics) {
	groupsElements := groups.Elements()
	userGroups := make([]cxsdk.SCIMUserGroup, 0, len(groupsElements))
	var diags diag.Diagnostics
	for _, group := range groupsElements {
		val, err := group.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Failed to convert value to Terraform", err.Error())
			continue
		}

		var str string
		if err = val.As(&str); err != nil {
			diags.AddError("Failed to convert value to string", err.Error())
			continue
		}
		userGroups = append(userGroups, cxsdk.SCIMUserGroup{Value: str})
	}
	if diags.HasError() {
		return nil, diags
	}
	return userGroups, nil
}

func extractUserSCIMName(ctx context.Context, name types.Object) (*cxsdk.SCIMUserName, diag.Diagnostics) {
	if name.IsNull() || name.IsUnknown() {
		return nil, nil
	}
	var nameModel UserNameModel
	diags := name.As(ctx, &nameModel, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.SCIMUserName{
		GivenName:  nameModel.GivenName.ValueString(),
		FamilyName: nameModel.FamilyName.ValueString(),
	}, nil
}

func extractUserEmails(ctx context.Context, emails types.Set) ([]cxsdk.SCIMUserEmail, diag.Diagnostics) {
	var diags diag.Diagnostics
	var emailsObjects []types.Object
	var expandedEmails []cxsdk.SCIMUserEmail
	emails.ElementsAs(ctx, &emailsObjects, true)

	for _, eo := range emailsObjects {
		var email UserEmailModel
		if dg := eo.As(ctx, &email, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedEmail := cxsdk.SCIMUserEmail{
			Value:   email.Value.ValueString(),
			Primary: email.Primary.ValueBool(),
			Type:    email.Type.ValueString(),
		}
		expandedEmails = append(expandedEmails, expandedEmail)
	}

	return expandedEmails, diags
}
