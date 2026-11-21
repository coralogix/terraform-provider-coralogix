// Copyright 2026 Coralogix Ltd.
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
	"net/http"
	"strconv"
	"strings"

	"github.com/coralogix/coralogix-management-sdk/go/openapi/gen/users_management_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/user"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// userHooks returns the handwritten steps around the generated coralogix_user. The resource and
// the data source share them. They carry the released behaviors that the generator cannot
// express (the fidelity recording pins them in
// internal/provider/testdata/user_baseline):
//
//   - the name object folds the flat firstName and lastName of the contract into the released
//     name.given_name and name.family_name;
//   - active is the released view of the status enum: ACTIVE is true, every other status is
//     false, and the attribute defaults to true;
//   - groups holds the group ids as their decimal strings, the values SCIM returned;
//   - Create sends the fixed onboarding mode beside the template (api.createValues).
func UserHooks() user.Hooks {
	return user.Hooks{
		BeforeWrite:  userBeforeWrite,
		Get:          userGet,
		AfterFlatten: userAfterFlatten,
		AfterRead:    userAfterRead,
		ModifyPlan:   userUpdatePlan,
	}
}

// NewResource returns the generated user resource wrapped: Update hands the client
// to the hook through the context, for the values only the API knows.
func NewResource(hooks user.Hooks) resource.Resource {
	return &UserResource{Resource: *user.NewResource(hooks).(*user.Resource)}
}

// userUpdatePlan shapes the plan of every update. It keeps the modes of an
// upgraded state unknown: a state that carries none (the schema of 3.26.0 had no
// attribute) plans them null, but the recovery reads the modes the backend has,
// and a known null the read does not answer is an inconsistent result after the
// apply. An unknown plans as (known after apply) and matches what the read
// stores; the access type of an upgraded state is likewise unknown, because the
// PUT response supplies it too. The order of the login modes needs no plan: the
// generator's keepPriorOrder makes the read store them in the order of the plan,
// so a configuration that lists them the way the backend would not still matches.
func userUpdatePlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if resp.Plan.Raw.IsNull() {
		return // A destroy plans no values.
	}
	// The plan the generated steps left: the plan-equality step may have set it to
	// the state, and the hook must shape that plan, not the raw input.
	planRaw := resp.Plan.Raw
	var plan user.RbacV2UserModel
	if d := resp.Plan.Get(ctx, &plan); d.HasError() {
		return
	}
	if plan.Username.IsNull() {
		return // Nothing to recover for.
	}
	attrs := map[string]tftypes.Value{}
	if err := planRaw.As(&attrs); err != nil {
		return
	}
	planned := planRaw
	changed := false
	// The state carries the resource: a legacy-state update. The schema of 3.26.0
	// had no access_type either: a missing value plans null, and the PUT response
	// supplies both the login modes and the access type, which a known-null plan
	// would reject as an inconsistent result. Plan both unknown until the read of
	// the apply fills them.
	if !req.State.Raw.IsNull() {
		for _, name := range []string{"allowed_login_mode", "access_type"} {
			at, ok := attrs[name]
			if ok && at.IsNull() && (name != "allowed_login_mode" || plan.AllowedLoginMode.IsNull()) {
				attrs[name] = tftypes.NewValue(at.Type(), tftypes.UnknownValue)
				changed = true
			}
		}
	}
	if changed {
		resp.Plan.Raw = tftypes.NewValue(planned.Type(), attrs)
	}
}

// userAfterRead reconciles the login modes with the plan or state the read came
// from: the backend answers them in its canonical order (LOCAL before SSO, once)
// while the configuration may list them in any order or repeat one, and Terraform
// compares a list in order — a state in the backend's order under a plan in the
// configured order would fail the apply as an inconsistent result. When the modes
// the API answered hold the same elements as the prior list, the state keeps the
// prior order (a repeated mode stays repeated); the wire value is unaffected, and
// the backend folds the order itself.
func userAfterRead(ctx context.Context, state *tfsdk.State, prior any) diag.Diagnostics {
	var diags diag.Diagnostics
	if state == nil || prior == nil {
		return diags
	}
	getter, ok := prior.(interface {
		Get(ctx context.Context, target any) diag.Diagnostics
	})
	if !ok {
		return diags
	}
	var modes types.List
	if d := state.GetAttribute(ctx, path.Root("allowed_login_mode"), &modes); d.HasError() {
		return diags
	}
	if modes.IsNull() || !modes.IsUnknown() && len(modes.Elements()) == 0 {
		return diags
	}
	var priorModel user.RbacV2UserModel
	if d := getter.Get(ctx, &priorModel); d.HasError() {
		return diags
	}
	priorModes := priorModel.AllowedLoginMode
	if priorModes.IsNull() || priorModes.IsUnknown() {
		return diags
	}
	priorElems := priorModes.Elements()
	elems := modes.Elements()
	if len(priorElems) != len(elems) {
		return diags // A different multiset: the read wins, and the plan must follow.
	}
	used := make([]bool, len(elems))
	for _, want := range priorElems {
		found := false
		for i, got := range elems {
			if !used[i] && want.Equal(got) {
				used[i] = true
				found = true
				break
			}
		}
		if !found {
			return diags // Not the same elements: the read wins.
		}
	}
	if d := state.SetAttribute(ctx, path.Root("allowed_login_mode"), priorModes); d.HasError() {
		return d
	}
	return diags
}

// userSearchKey is the context key of the search the hook can run for the values the
// state of an upgraded resource does not carry.
type userSearchKey struct{}

// NewUserResource returns the wrapped user resource with the released hooks.
func NewUserResource() resource.Resource {
	return NewResource(UserHooks())
}

// UserResource wraps the generated user resource: Update hands the users client to
// the hook through the context, and Configure captures it from the provider data.
type UserResource struct {
	user.Resource
	client *users_management_service.UsersManagementServiceAPIService
}

func (r *UserResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.Resource.Configure(ctx, req, resp)
	if cs, ok := req.ProviderData.(*clientset.ClientSet); ok {
		r.client = cs.Users()
	}
}

func (r *UserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client != nil {
		ctx = context.WithValue(ctx, userSearchKey{}, func(username string) (*users_management_service.SearchUsersResponse, error) {
			out, _, err := r.client.UsersMgmtServiceSearchUsers(ctx).Username(username).PageSize(100).Execute()
			if err != nil {
				return out, err
			}
			// Page through the rest: a user past the first page is a real user,
			// and missing one would send a PUT without its values, which the
			// backend treats as clear-them.
			users := out.GetUsers()
			pageToken := out.GetNextPageToken()
			for page := 1; pageToken != 0 && page < 100; page++ {
				next, _, err := r.client.UsersMgmtServiceSearchUsers(ctx).Username(username).PageSize(100).PageToken(pageToken).Execute()
				if err != nil {
					return out, err
				}
				users = append(users, next.GetUsers()...)
				pageToken = next.GetNextPageToken()
				if len(next.GetUsers()) == 0 {
					break
				}
			}
			out.Users = users
			return out, nil
		})
	}
	r.Resource.Update(ctx, req, resp)
}

// userGet pages the Search at the released page size: the released read sent
// page_size=100 on every page. The username hint of the released read (it
// narrowed the search to the user it read) is a generator gap (V6): the hook
// cannot see the resource it reads, so the generated read pages unfiltered and
// matches the id itself.
func userGet(_ context.Context, request func() users_management_service.ApiUsersMgmtServiceSearchUsersRequest) (*users_management_service.SearchUsersResponse, *http.Response, error) {
	return request().PageSize(100).Execute()
}

// userName is the object of the name extra attribute.
type userName struct {
	FamilyName basetypes.StringValue `tfsdk:"family_name"`
	GivenName  basetypes.StringValue `tfsdk:"given_name"`
}

var userNameAttrTypes = map[string]attr.Type{
	"family_name": basetypes.StringType{},
	"given_name":  basetypes.StringType{},
}

// userBeforeWrite folds the released Terraform shape into the request body. The body is the
// Create item (*CreateUserRequest, with the template and the fixed onboarding mode beside it)
// or the Update item (*UpdateUserRequest); the template in it carries the resource fields.
func userBeforeWrite(ctx context.Context, _ tfsdk.Config, plan tfsdk.Plan, state *tfsdk.State, body any) diag.Diagnostics {
	var diags diag.Diagnostics
	var template *users_management_service.UserTemplate
	switch request := body.(type) {
	case *users_management_service.CreateUserRequest:
		if request == nil {
			return diags
		}
		// Create sends the batch item with its fixed onboarding mode (api.createValues).
		// The generator does not render createValues yet, so the hook sets it.
		mode := users_management_service.ONBOARDINGMODE_ONBOARDING_MODE_NO_INVITE
		request.OnboardingMode = &mode
		template = request.UserTemplate
	case *users_management_service.UpdateUserRequest:
		if request == nil {
			return diags
		}
		template = request.UserTemplate
		// The API ignores a username change: the PUT answers 200 and keeps the old
		// name, and the read then reports a resource that differs from the plan. The
		// released resource refused the change with a clear diagnostic, so the hook
		// does the same. A case-only change is no rename: the backend normalizes the
		// case of an SSO login.
		if state != nil && template != nil && template.GetUsername() != "" {
			var priorModel user.RbacV2UserModel
			if d := state.Get(ctx, &priorModel); d.HasError() {
				diags.Append(d...)
				return diags
			}
			if !priorModel.Username.IsNull() && !strings.EqualFold(template.GetUsername(), priorModel.Username.ValueString()) {
				diags.AddAttributeError(path.Root("user_name"), "Unable to update user",
					fmt.Sprintf("User name cannot be updated after the user is created: the API keeps %q. Remove the resource and create it again with the new name.", priorModel.Username.ValueString()))
				return diags
			}
		}
	default:
		return diags
	}
	if template == nil {
		return diags
	}
	var model user.RbacV2UserModel
	if d := plan.Get(ctx, &model); d.HasError() {
		diags.Append(d...)
		return diags
	}
	// The name object folds into the flat fields. A null object sends neither, as the
	// released resource did for a user without a name. A component the configuration
	// removes sends the empty string: the API stores an omitted name as "" and reads it
	// back as null, so the state keeps the null the plan carries and the update plans no
	// change after the apply.
	if !model.Name.IsNull() && !model.Name.IsUnknown() {
		var name userName
		if d := model.Name.As(ctx, &name, basetypes.ObjectAsOptions{}); d.HasError() {
			diags.Append(d...)
			return diags
		}
		given, family := name.GivenName.ValueString(), name.FamilyName.ValueString()
		template.FirstName = &given
		template.LastName = &family
	}
	// A state upgraded from the released provider carries the login modes and the
	// access type nowhere: the schema of 3.26.0 had no attribute for either, and its
	// private state is a form this framework discards. Without a refresh the plan
	// keeps them unknown, and the expand omits them: the PUT would clear the modes
	// and replace the access type with permanent, discarding a temporary access
	// expiry. The hook searches the API for the prior user and sends the modes and
	// the access type it answers with. A failed search stops the update: continuing
	// would destroy both, and the released resource propagated a lookup failure the
	// same way.
	if (template.AllowedLoginMode == nil || template.AccessType == nil) && state != nil {
		var prior user.RbacV2UserModel
		if d := state.Get(ctx, &prior); !d.HasError() && !prior.Username.IsNull() {
			search, ok := ctx.Value(userSearchKey{}).(func(string) (*users_management_service.SearchUsersResponse, error))
			if !ok {
				diags.AddAttributeError(path.Root("allowed_login_mode"), "Unable to recover the login modes of the user",
					"The state of an upgraded user carries no login modes, and the resource has no users client to search for them. Refresh the resource to read them.")
				return diags
			}
			found, err := search(prior.Username.ValueString())
			if err != nil {
				// A 404 of the search says nothing about the values: the update path
				// reads the user after the write and reports a resource that is gone.
				// Any other failure stops the update, which would destroy the modes
				// and the access type.
				if !strings.Contains(err.Error(), "404") {
					diags.AddAttributeError(path.Root("allowed_login_mode"), "Unable to recover the login modes of the user",
						fmt.Sprintf("The search for %q failed: %s. Refresh the resource to read the modes, then update again.", prior.Username.ValueString(), err))
					return diags
				}
			} else if found != nil {
				for i := range found.Users {
					if strings.EqualFold(found.Users[i].GetUsername(), prior.Username.ValueString()) {
						if template.AllowedLoginMode == nil {
							template.AllowedLoginMode = found.Users[i].AllowedLoginMode
						}
						if template.AccessType == nil {
							// A user without an explicit access type in the API is
							// permanent-access; send it literally so the PUT keeps
							// the user's access rather than relying on the backend
							// default.
							template.AccessType = found.Users[i].AccessType
							if template.AccessType == nil {
								template.AccessType = &users_management_service.AccessType{
									AccessType:      "permanent",
									PermanentAccess: map[string]interface{}{},
								}
							}
						}
					}
				}
			}
		}
	}
	// The schema skips the one-of discriminator of the access type, so the expand sends
	// an empty one beside the chosen arm. The API reads every arm literally, and EU2
	// always answers with the discriminator set, so the hook fills it from the arm.
	if template.AccessType != nil {
		if template.AccessType.TemporaryAccess != nil {
			template.AccessType.AccessType = "temporary"
		} else {
			template.AccessType.AccessType = "permanent"
		}
	}
	// active carries the status: true is ACTIVE, false INACTIVE. The attribute defaults
	// to true, so a create always sends a status, and the batch PUT never clears one.
	status := users_management_service.USERSTATUS_USER_STATUS_ACTIVE
	if !model.Active.IsNull() && !model.Active.IsUnknown() && !model.Active.ValueBool() {
		status = users_management_service.USERSTATUS_USER_STATUS_INACTIVE
	}
	template.Status = &status
	return diags
}

// emptyAsNull reads an API name as null when it is absent or empty: the API
// stores an omitted name as "".
func emptyAsNull(v *string) basetypes.StringValue {
	if v == nil || *v == "" {
		return basetypes.NewStringNull()
	}
	return basetypes.NewStringValue(*v)
}

// userAfterFlatten projects the flat API fields into the extra attributes: the name object,
// the active bool, and the groups set of decimal strings.
func userAfterFlatten(ctx context.Context, m *user.RbacV2UserModel, v *users_management_service.RbacV2User) diag.Diagnostics {
	var diags diag.Diagnostics
	if m == nil || v == nil {
		return diags
	}
	// The API stores an omitted name as the empty string and echoes it back as
	// "": the attribute reads as null, as the released resource recorded. The
	// model carries the kept prior name when the API holds none of it, so the
	// empty API name only replaces an absent prior name.
	name := userName{
		GivenName:  emptyAsNull(v.FirstName),
		FamilyName: emptyAsNull(v.LastName),
	}
	if name.GivenName.IsNull() && name.FamilyName.IsNull() {
		// The API stores no name: the plan or the prior state keeps the name the
		// configuration set, even the empty one (the API cannot tell a null name
		// from an empty one, and the released resource kept the prior). Without a
		// prior, m.Name already is the null object that flatten built.
	} else {
		object, d := basetypes.NewObjectValue(userNameAttrTypes, map[string]attr.Value{
			"given_name":  name.GivenName,
			"family_name": name.FamilyName,
		})
		diags.Append(d...)
		m.Name = object
	}
	// active is ACTIVE; every other status is inactive: an unknown status never reports a
	// user as usable.
	m.Active = basetypes.NewBoolValue(v.GetStatus() == users_management_service.USERSTATUS_USER_STATUS_ACTIVE)
	// groups holds the group ids as their decimal strings, in the order of the API.
	ids := make([]string, 0, len(v.GroupIds))
	for _, id := range v.GroupIds {
		ids = append(ids, strconv.FormatInt(id, 10))
	}
	set, d := basetypes.NewSetValueFrom(ctx, basetypes.StringType{}, ids)
	diags.Append(d...)
	m.Groups = set
	return diags
}
