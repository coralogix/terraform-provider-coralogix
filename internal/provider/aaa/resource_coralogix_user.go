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

	users "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/users_management_service"
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
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func NewUserResource() resource.Resource {
	return &UserResource{}
}

type UserResource struct {
	client *users.UsersManagementServiceAPIService
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
				Required: true,
				PlanModifiers: []planmodifier.String{
					caseInsensitiveStringPlanModifier{},
				},
				MarkdownDescription: "User name (email). Comparison is case-insensitive: SSO " +
					"login can normalize letter case in the backend, and that normalization " +
					"will not trigger drift in subsequent plans.",
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
		MarkdownDescription: "Coralogix User. For more info please review - https://coralogix.com/docs/user-guides/account-management/user-management/manage-team-members/.",
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

	name, diags := extractUserName(ctx, plan.Name)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	userName := plan.UserName.ValueString()
	onboardingMode := users.ONBOARDINGMODE_ONBOARDING_MODE_NO_INVITE
	createReq := []users.CreateUserRequest{{
		OnboardingMode: &onboardingMode,
		UserTemplate:   createUserTemplate(userName, name, plan.Active.ValueBool()),
	}}

	log.Printf("[INFO] Creating new User: %s", userName)
	createResp, httpResp, err := r.client.
		UsersMgmtServiceCreateUsers(ctx).
		CreateUserRequest(createReq).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error creating User", formatUserAPIError(httpResp, err, "CreateUsers", userName).Error())
		return
	}

	result, err := createUserResultFor(createResp, userName)
	if err != nil {
		resp.Diagnostics.AddError("Error creating User", err.Error())
		return
	}
	user, err := createdUser(result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating User", err.Error())
		return
	}
	log.Printf("[INFO] Created new User %s", user.GetUserId())

	r.setUserState(ctx, plan, user, &resp.State, resp.Private, &resp.Diagnostics)
}

func (r *UserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *UserResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	log.Printf("[INFO] Reading User: %s", id)
	user, err := findUserByID(ctx, r.client, id, state.UserName.ValueString())
	if err != nil {
		if isUserNotFoundErr(err) {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("User %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading User", err.Error())
		return
	}

	refreshed, err := flattenUserToState(ctx, user)
	if err != nil {
		resp.Diagnostics.AddError("Error reading User", err.Error())
		return
	}
	refreshed.Name = preserveUserName(ctx, state.Name, refreshed.Name)

	resp.Diagnostics.Append(setUserEcho(ctx, resp.Private, user)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The username is stored exactly as the backend spells it, which is what the SCIM
	// read did. The plan modifier absorbs a case-only difference from the configuration,
	// so this cannot produce a diff, and it keeps `user_name` and `emails[].value` in
	// agreement.
	diags = resp.State.Set(ctx, refreshed)
	resp.Diagnostics.Append(diags...)
}

func (r *UserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state *UserResourceModel
	diags := req.Plan.Get(ctx, &plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	diags = req.State.Get(ctx, &state)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	if !strings.EqualFold(plan.UserName.ValueString(), state.UserName.ValueString()) {
		resp.Diagnostics.AddError(
			"User name cannot be updated",
			fmt.Sprintf(
				"Cannot change user_name from %q to %q. user_name is set at creation and "+
					"cannot be updated in place. To assign this resource to a different user, "+
					"recreate it (terraform state rm + apply, or `terraform state mv`).",
				state.UserName.ValueString(), plan.UserName.ValueString(),
			),
		)
		return
	}

	name, diags := extractUserName(ctx, plan.Name)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	userID := state.ID.ValueString()
	echo, err := r.userEcho(ctx, req.Private, userID, state.UserName.ValueString())
	if err != nil {
		if isUserNotFoundErr(err) {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("User %q is in state, but no longer exists in Coralogix backend", userID),
				fmt.Sprintf("%s will be recreated when you apply", userID),
			)
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error updating User", err.Error())
		return
	}

	log.Printf("[INFO] Updating User %s", userID)
	user, err := putUser(ctx, r.client, userID, updateUserTemplate(name, plan.Active.ValueBool(), echo))
	if err != nil {
		if isUserNotFoundErr(err) {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("User %q is in state, but no longer exists in Coralogix backend", userID),
				fmt.Sprintf("%s will be recreated when you apply", userID),
			)
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error updating User", err.Error())
		return
	}

	r.setUserState(ctx, plan, user, &resp.State, resp.Private, &resp.Diagnostics)
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

	name, diags := extractUserName(ctx, state.Name)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	echo, err := r.userEcho(ctx, req.Private, id, state.UserName.ValueString())
	if err != nil {
		if isUserNotFoundErr(err) {
			log.Printf("[INFO] User %s is already gone", id)
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("Error Deleting User %s", id), err.Error())
		return
	}

	// The Users API has no delete. Deactivating matches what the SCIM delete did: it
	// was always a soft delete that left the user readable with an inactive status.
	// Deactivating a user that is already inactive succeeds, so no read is needed first.
	if _, err := putUser(ctx, r.client, id, updateUserTemplate(name, false, echo)); err != nil {
		if isUserNotFoundErr(err) {
			log.Printf("[INFO] User %s is already gone", id)
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("Error Deleting User %s", id), err.Error())
		return
	}
	log.Printf("[INFO] User %s deactivated", id)
}

// setUserState writes the user a create or an update returned. Both responses carry the
// full user, so no read follows.
func (r *UserResource) setUserState(ctx context.Context, plan *UserResourceModel, user *users.RbacV2User, state *tfsdk.State, private userPrivateState, diags *diag.Diagnostics) {
	applied, err := flattenUserToState(ctx, user)
	if err != nil {
		diags.AddError("Error reading User", err.Error())
		return
	}
	applied.UserName = preserveUserNameCase(plan.UserName, applied.UserName)
	applied.Name = preserveUserName(ctx, plan.Name, applied.Name)

	diags.Append(setUserEcho(ctx, private, user)...)
	if diags.HasError() {
		return
	}
	diags.Append(state.Set(ctx, applied)...)
}

// userEcho returns the fields an update has to send back unchanged. They come from
// private state, which every read and write refreshes. Private state is empty only when
// no read has run since the upgrade from the SCIM provider, for example on
// `apply -refresh=false`. Only then does this search for the user.
func (r *UserResource) userEcho(ctx context.Context, private userPrivateState, userID, userName string) (userEcho, error) {
	raw, diags := private.GetKey(ctx, userEchoPrivateKey)
	if diags.HasError() {
		first := diags.Errors()[0]
		return userEcho{}, fmt.Errorf("%s: %s", first.Summary(), first.Detail())
	}
	echo, err := decodeUserEcho(raw)
	if err != nil {
		return userEcho{}, err
	}
	if echo != nil {
		return *echo, nil
	}

	log.Printf("[INFO] No stored login modes for User %s, reading them first", userID)
	user, err := findUserByID(ctx, r.client, userID, userName)
	if err != nil {
		return userEcho{}, err
	}
	return userEchoFrom(user), nil
}

// userPrivateState is the part of the framework's private state the user resource uses.
type userPrivateState interface {
	GetKey(ctx context.Context, key string) ([]byte, diag.Diagnostics)
	SetKey(ctx context.Context, key string, value []byte) diag.Diagnostics
}

func setUserEcho(ctx context.Context, private userPrivateState, user *users.RbacV2User) diag.Diagnostics {
	raw, err := encodeUserEcho(user)
	if err != nil {
		return diag.Diagnostics{diag.NewErrorDiagnostic("Error storing User login modes", err.Error())}
	}
	return private.SetKey(ctx, userEchoPrivateKey, raw)
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

func extractUserName(ctx context.Context, name types.Object) (*UserNameModel, diag.Diagnostics) {
	if name.IsNull() || name.IsUnknown() {
		return nil, nil
	}
	var nameModel UserNameModel
	if diags := name.As(ctx, &nameModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}
	return &nameModel, nil
}

// preserveUserNameCase keeps the username the configuration wrote, so a backend that
// normalizes letter case does not produce an inconsistent-result error or a diff.
func preserveUserNameCase(configured, fromAPI types.String) types.String {
	if configured.IsNull() || configured.IsUnknown() {
		return fromAPI
	}
	if strings.EqualFold(configured.ValueString(), fromAPI.ValueString()) {
		return configured
	}
	return fromAPI
}

type caseInsensitiveStringPlanModifier struct{}

func (caseInsensitiveStringPlanModifier) Description(_ context.Context) string {
	return "Treats the value as case-insensitive — suppresses diffs that only change letter case."
}

func (m caseInsensitiveStringPlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (caseInsensitiveStringPlanModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.StateValue.IsNull() || req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}
	if strings.EqualFold(req.StateValue.ValueString(), req.PlanValue.ValueString()) {
		resp.PlanValue = req.StateValue
	}
}
