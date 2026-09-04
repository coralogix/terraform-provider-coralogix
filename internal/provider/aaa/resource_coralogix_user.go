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
	"time"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"

	"github.com/cenkalti/backoff/v5"
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
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func NewUserResource() resource.Resource {
	return &UserResource{}
}

type UserResource struct {
	clients *userClients
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

	r.clients = newUserClients(clientSet)
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

	teamID, err := r.clients.teamID(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error creating User", teamIDErrorDetail(err))
		return
	}

	userName := plan.UserName.ValueString()
	onboardingMode := users.ONBOARDINGMODE_ONBOARDING_MODE_NO_INVITE
	createReq := []users.CreateUserRequest{{
		OnboardingMode: &onboardingMode,
		UserTemplate:   createUserTemplate(userName, name, plan.Active.ValueBool()),
	}}

	log.Printf("[INFO] Creating new User: %s", userName)
	createResp, httpResp, err := r.clients.users.
		UsersMgmtServiceCreateUsers(ctx, teamID).
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
	userID, _, err := createdUserIDs(result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating User", err.Error())
		return
	}
	log.Printf("[INFO] Submitted new User %s", userID)

	state, err := r.readUserAfterWrite(ctx, userID, userName)
	if err != nil {
		resp.Diagnostics.AddError("Error reading User after create", err.Error())
		return
	}

	// The create template carries a status, but the backend does not have to honour it.
	// A second call settles the difference rather than letting the applied state
	// disagree with the plan.
	if state.Active.ValueBool() != plan.Active.ValueBool() {
		user, err := findUserByID(ctx, r.clients.users, teamID, userID, userName)
		if err != nil {
			resp.Diagnostics.AddError("Error setting User status after create", err.Error())
			return
		}
		if err := r.setUserStatus(ctx, teamID, user.GetUserAccountId(), userID, plan.Active.ValueBool()); err != nil {
			resp.Diagnostics.AddError("Error setting User status after create", err.Error())
			return
		}
		if state, err = r.readUserAfterWrite(ctx, userID, userName); err != nil {
			resp.Diagnostics.AddError("Error reading User after create", err.Error())
			return
		}
	}

	state.UserName = preserveUserNameCase(plan.UserName, state.UserName)
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
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
	refreshed, err := readUser(ctx, r.clients, id, state.UserName.ValueString())
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

	teamID, err := r.clients.teamID(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error updating User", teamIDErrorDetail(err))
		return
	}

	userID := state.ID.ValueString()
	userName := state.UserName.ValueString()
	user, err := findUserByID(ctx, r.clients.users, teamID, userID, userName)
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
	userAccountID := user.GetUserAccountId()

	// The name and the status live behind two separate calls, so each change is sent
	// only when it is needed. If the second one fails, state keeps its previous values
	// and the apply reports the error. The next plan refreshes, sees what did land, and
	// sends only what is still missing, so both calls are safe to repeat.
	if err := r.applyUserUpdates(ctx, teamID, userAccountID, user, plan, name); err != nil {
		resp.Diagnostics.AddError("Error updating User", err.Error())
		return
	}

	refreshed, err := r.readUserAfterWrite(ctx, userID, userName)
	if err != nil {
		resp.Diagnostics.AddError("Error reading User after update", err.Error())
		return
	}

	refreshed.UserName = preserveUserNameCase(plan.UserName, refreshed.UserName)
	diags = resp.State.Set(ctx, refreshed)
	resp.Diagnostics.Append(diags...)
}

// applyUserUpdates sends the name change and the status change that the plan asks for.
func (r *UserResource) applyUserUpdates(ctx context.Context, teamID, userAccountID int64, user *users.RbacV2User, plan *UserResourceModel, name *UserNameModel) error {
	if userAccountID == 0 {
		return fmt.Errorf("user %q was returned without a userAccountId, which the update needs", user.GetUserId())
	}

	if userNameChanged(user, name) {
		log.Printf("[INFO] Updating User %s name", user.GetUserId())
		updateReq := []users.UpdateUserRequest{{
			UserAccountId: &userAccountID,
			UserTemplate:  updateUserTemplate(user.GetUsername(), name, plan.Active.ValueBool()),
		}}
		_, httpResp, err := r.clients.users.
			UsersMgmtServiceUpdateUsers(ctx, teamID).
			UpdateUserRequest(updateReq).
			Execute()
		if err != nil {
			return formatUserAPIError(httpResp, err, "UpdateUsers", user.GetUserId())
		}
	}

	if plan.Active.ValueBool() != isUserActive(user) {
		if err := r.setUserStatus(ctx, teamID, userAccountID, user.GetUserId(), plan.Active.ValueBool()); err != nil {
			return err
		}
	}

	return nil
}

func (r *UserResource) setUserStatus(ctx context.Context, teamID, userAccountID int64, userID string, active bool) error {
	if userAccountID == 0 {
		return fmt.Errorf("user %q was returned without a userAccountId, which the status change needs", userID)
	}

	status := userStatusFromActive(active)
	log.Printf("[INFO] Setting User %s status to %s", userID, status)
	_, httpResp, err := r.clients.users.
		UsersMgmtServiceUpdateUsersStatuses(ctx, teamID).
		UpdateUserStatusRequest(users.UpdateUserStatusRequest{
			Status:         &status,
			UserAccountIds: []int64{userAccountID},
		}).
		Execute()
	if err != nil {
		return formatUserAPIError(httpResp, err, "UpdateUsersStatuses", userID)
	}
	return nil
}

func (r *UserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state *UserResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	teamID, err := r.clients.teamID(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error deleting User", teamIDErrorDetail(err))
		return
	}

	id := state.ID.ValueString()
	log.Printf("[INFO] Deleting User %s", id)

	// The Users API has no delete. Deactivating matches what the SCIM delete did: it
	// was always a soft delete that left the user readable with an inactive status.
	user, err := findUserByID(ctx, r.clients.users, teamID, id, state.UserName.ValueString())
	if err != nil {
		if isUserNotFoundErr(err) {
			log.Printf("[INFO] User %s is already gone", id)
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("Error Deleting User %s", id), err.Error())
		return
	}

	if !isUserActive(user) {
		log.Printf("[INFO] User %s is already inactive", id)
		return
	}

	userAccountID := user.GetUserAccountId()
	if userAccountID == 0 {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting User %s", id),
			"The API returned the user without a userAccountId, which the deactivation needs.",
		)
		return
	}

	if err := r.setUserStatus(ctx, teamID, userAccountID, id, false); err != nil {
		if isUserNotFoundErr(err) {
			log.Printf("[INFO] User %s is already gone", id)
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("Error Deleting User %s", id), err.Error())
		return
	}
	log.Printf("[INFO] User %s deactivated", id)
}

// readUserAfterWrite reads the user back so the state written by a create or an update
// is exactly the state a later refresh produces. A write is followed by a short retry,
// because a freshly created user can take a moment to appear in the search index.
func (r *UserResource) readUserAfterWrite(ctx context.Context, userID, userName string) (*UserResourceModel, error) {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = time.Second
	b.MaxInterval = 3 * time.Second

	op := func() (*UserResourceModel, error) {
		state, err := readUser(ctx, r.clients, userID, userName)
		if err != nil {
			if isUserNotFoundErr(err) {
				log.Printf("[INFO] User %s not visible yet (eventual consistency), retrying", userID)
				return nil, err
			}
			return nil, backoff.Permanent(err)
		}
		return state, nil
	}

	return backoff.Retry(ctx, op,
		backoff.WithBackOff(b),
		backoff.WithMaxTries(5),
		backoff.WithMaxElapsedTime(10*time.Second),
	)
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

// userNameChanged reports whether the plan asks for a first or last name the user does
// not already have. A plan without a name block asks for nothing, so it changes nothing.
func userNameChanged(user *users.RbacV2User, name *UserNameModel) bool {
	if name == nil {
		return false
	}
	return user.GetFirstName() != name.GivenName.ValueString() ||
		user.GetLastName() != name.FamilyName.ValueString()
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

func teamIDErrorDetail(err error) string {
	return fmt.Sprintf(
		"Could not resolve the team the API key belongs to. Every Users API call needs it.\nerror - %s\noperation - WhoAmI",
		err,
	)
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
