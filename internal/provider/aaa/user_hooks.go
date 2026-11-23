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
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	"github.com/coralogix/coralogix-management-sdk/go/openapi/gen/users_management_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/user"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
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
		AfterGet:     userAfterGet,
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
	// The state carries the resource: a legacy-state update. The schema of 3.26.0
	// had no access_type either: a missing value plans null, and the PUT response
	// supplies both the login modes and the access type, which a known-null plan
	// would reject as an inconsistent result. Plan both unknown until the read of
	// the apply fills them. This runs for an unknown username too (a name built
	// from another resource's attribute): the unknowns are the only plan the
	// apply can correlate with.
	attrs := map[string]tftypes.Value{}
	if err := planRaw.As(&attrs); err != nil {
		return
	}
	planned := planRaw
	changed := false
	if !req.State.Raw.IsNull() {
		for _, name := range []string{"allowed_login_mode", "access_type"} {
			at, ok := attrs[name]
			if ok && at.IsNull() {
				attrs[name] = tftypes.NewValue(at.Type(), tftypes.UnknownValue)
				changed = true
			}
		}
	}
	// emails is the released derived set: exactly one primary work email whose
	// value is the username. The plan carries the set the apply writes, so the
	// plan correlates with the state. An unknown username keeps the set unknown —
	// the read of the apply fills it, and a known plan here would disagree with
	// the state the read writes.
	if plan.Username.IsNull() || plan.Username.IsUnknown() {
		// The schema default would plan a known empty set; the read of the apply
		// writes the derived one-entry set, and a known plan here would disagree
		// with that state as an inconsistent result. Plan the set unknown: the
		// released plan modifier left it (known after apply) in this case too.
		if at, ok := attrs["emails"]; ok && !at.IsNull() {
			attrs["emails"] = tftypes.NewValue(at.Type(), tftypes.UnknownValue)
			changed = true
		}
		if changed {
			resp.Plan.Raw = tftypes.NewValue(planned.Type(), attrs)
		}
		return
	}
	// A case-only change of the username plans the state's spelling: the API
	// ignores a username change, the apply succeeds, and the read keeps the
	// spelling of the configuration, so the released plan showed no change.
	// The state spelling is what the API holds (the read of the last apply
	// wrote it), and the next read keeps it, so the plan correlates.
	if !req.State.Raw.IsNull() {
		var state user.RbacV2UserModel
		if d := req.State.Get(ctx, &state); !d.HasError() && !state.Username.IsNull() {
			if strings.EqualFold(plan.Username.ValueString(), state.Username.ValueString()) {
				if at, ok := attrs["user_name"]; ok {
					attrs["user_name"] = tftypes.NewValue(at.Type(), state.Username.ValueString())
					changed = true
				}
			}
		}
	}
	if at, ok := attrs["emails"]; ok {
		// The username the plan finally carries (the state spelling on a
		// case-only change) decides the email value, because the flatten of the
		// apply reads that username from the plan.
		plannedUserName := plan.Username.ValueString()
		if userAttr, ok := attrs["user_name"]; ok && userAttr.IsKnown() && !userAttr.IsNull() {
			var value string
			if err := userAttr.As(&value); err == nil {
				plannedUserName = value
			}
		}
		objectType := at.Type().(tftypes.Set).ElementType
		objectTypes := objectType.(tftypes.Object).AttributeTypes
		object := tftypes.NewValue(objectType, map[string]tftypes.Value{
			"primary": tftypes.NewValue(objectTypes["primary"], true),
			"type":    tftypes.NewValue(objectTypes["type"], userEmailType),
			"value":   tftypes.NewValue(objectTypes["value"], plannedUserName),
		})
		attrs["emails"] = tftypes.NewValue(at.Type(), []tftypes.Value{object})
		changed = true
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
	// The backend answers each distinct mode once; the configuration may repeat
	// one. The lists reconcile when they hold the same distinct elements: the
	// state keeps the prior list, repetitions and all, and the read's shorter
	// answer never shortens it.
	priorSet := map[string]bool{}
	for _, v := range priorModes.Elements() {
		priorSet[v.String()] = true
	}
	answerSet := map[string]bool{}
	for _, v := range modes.Elements() {
		answerSet[v.String()] = true
	}
	if len(priorSet) != len(answerSet) {
		return diags // A different set of modes: the read wins, and the plan must follow.
	}
	for m := range priorSet {
		if !answerSet[m] {
			return diags
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

// Create restores the released diagnostics of a batch create that did not create
// the user. The generated create() reports "the API did not create the user" with
// the result message, which the backend leaves out for every failed status, so the
// statuses are not tellable apart from the text. The wrapper asks the API instead:
// when the failed create leaves a user with that username in the team, the released
// "already exists" diagnostic names it and points at the import; when it does not,
// the generated error stands (an unspecified status, a domain that is not allowed).
// An error that says the API created the user but returned no resource is the
// invited case: the invitation holds no account yet, so the released message says
// so.
func (r *UserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.Resource.Create(ctx, req, resp)
	if !resp.Diagnostics.HasError() {
		return
	}
	var plan user.RbacV2UserModel
	if d := req.Plan.Get(ctx, &plan); d.HasError() || plan.Username.IsNull() || plan.Username.IsUnknown() {
		return
	}
	for i, d := range resp.Diagnostics {
		if d.Severity() != diag.SeverityError {
			continue
		}
		detail := d.Detail()
		summary := d.Summary()
		switch {
		case strings.Contains(detail, "the API created the user, but returned no resource"):
			resp.Diagnostics[i] = diag.NewErrorDiagnostic(summary,
				fmt.Sprintf("user %q was invited rather than created, so it has no id until the invitation is accepted", plan.Username.ValueString()))
		case summary == "Unable to create user" && strings.Contains(detail, "status CREATE_USER_STATUS_ALREADY_INVITED"):
			// The generated error now names the refusing status; the released
			// resource answered this one with the invitation wording.
			resp.Diagnostics[i] = diag.NewErrorDiagnostic(summary,
				fmt.Sprintf("user %q was invited rather than created, so it has no id until the invitation is accepted", plan.Username.ValueString()))
		case summary == "Unable to create user" && strings.Contains(detail, "status CREATE_USER_STATUS_ALREADY_EXISTS") && r.client != nil:
			// An account with that username already exists: the released
			// diagnostic says to import it.
			resp.Diagnostics[i] = diag.NewErrorDiagnostic(summary,
				fmt.Sprintf("user %q already exists. Import it with `terraform import` instead of creating it.", plan.Username.ValueString()))
		}
	}
}

// Read hands the state's user_name to the Get hook through the context: the read
// then searches with the username filter the released read sent, instead of paging
// the whole team and matching the id itself. When the filtered read misses — a user
// renamed outside Terraform keeps the id, and only the name moved — the read falls
// back to the unfiltered, paged search the released read retried with, so a rename
// outside Terraform is a drift the refresh reports, never a silent removal.
func (r *UserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model user.RbacV2UserModel
	hinted := false
	if d := req.State.Get(ctx, &model); !d.HasError() && !model.Username.IsNull() && !model.Username.IsUnknown() {
		ctx = context.WithValue(ctx, userReadNameKey{}, model.Username.ValueString())
		hinted = true
	}
	r.Resource.Read(ctx, req, resp)
	// The hinted read removed the resource from state: give the released fallback
	// a chance before Terraform recreates the user as a duplicate. The removal
	// is a warning, not an error, and the retry needs a fresh response so the
	// stale "disappeared" warning does not survive it. An unhinted read already
	// scanned unfiltered — a retry would repeat the same scan.
	if !hinted || resp.Diagnostics.HasError() || !resp.State.Raw.IsNull() || !diagHadRemoval(resp) {
		return
	}
	retry := resource.ReadResponse{State: tfsdk.State{Raw: tftypes.Value{}, Schema: resp.State.Schema}, Diagnostics: diag.Diagnostics{}}
	retry.State = tfsdk.State{Raw: tftypes.NewValue(resp.State.Schema.Type().TerraformType(ctx), nil), Schema: resp.State.Schema}
	ctx = context.WithValue(ctx, userReadNameKey{}, "")
	r.Resource.Read(ctx, req, &retry)
	if retry.Diagnostics.HasError() {
		resp.Diagnostics = retry.Diagnostics
		return
	}
	resp.State = retry.State
	// The retry answered: the first read's "removed it from state" warning is
	// stale — the user is in state again.
	resp.Diagnostics = retry.Diagnostics
}

// diagHadRemoval reports whether the read decided the user is gone. The generated
// Read removes the resource from state and warns; the wrapper retries unfiltered
// once before accepting that.
func diagHadRemoval(resp *resource.ReadResponse) bool {
	for _, d := range resp.Diagnostics {
		if d.Severity() == diag.SeverityWarning && strings.Contains(d.Summary(), "disappeared") {
			return true
		}
	}
	return false
}

// ImportState keeps the released email-form import: an import id that carries an
// "@" is a user_name, which the Search resolves to the one user whose username
// equals it case-insensitively; the id the generated import passes through is the
// resolved user id. The Users API has no read by id besides the List, so the
// resolution searches with the username filter the API documents and pages it.
func (r *UserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !strings.Contains(req.ID, "@") {
		r.Resource.ImportState(ctx, req, resp)
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Error importing User",
			"The resource has no users client to resolve the user_name of the import. Import with the user id instead.")
		return
	}
	user, diags := resolveUserByUsername(ctx, r.client, req.ID, "Error importing User")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	// The generated import reads by the id with no hint of its own. The released
	// import left the username in the state, and the read of the refresh filters
	// with it — write it through the passthrough state so the follow-up read is
	// hinted (the context of ImportState does not reach that Read).
	req.ID = user.GetUserId()
	r.Resource.ImportState(ctx, req, resp)
	if resp.Diagnostics.HasError() {
		return
	}
	// The released import stored the username as the API holds it: a lookup in
	// another letter case resolves to the stored spelling, which the reads of the
	// refresh keep.
	_ = resp.State.SetAttribute(ctx, path.Root("user_name"), user.GetUsername())
}

// resolveUserByUsername resolves a user_name to the one user whose username equals
// it case-insensitively: the search narrows server-side with the username filter,
// and the wrapper pages the result. The data-source lookup and the import share
// the semantics the released resource recorded, including the page loop of the
// released search: it stops when the token does not move forward, so a server
// that repeats a token cannot turn one match into several. The summary is the
// diagnostic summary of a failed search, which the caller words for its context.
func resolveUserByUsername(ctx context.Context, client *users_management_service.UsersManagementServiceAPIService, userName, summary string) (*users_management_service.RbacV2User, diag.Diagnostics) {
	var diags diag.Diagnostics
	var matches []*users_management_service.RbacV2User
	pageToken := int64(0)
	for page := 1; ; page++ {
		request := client.UsersMgmtServiceSearchUsers(ctx).Username(userName).PageSize(100)
		if pageToken != 0 {
			request = request.PageToken(pageToken)
		}
		out, httpResp, err := request.Execute()
		if err != nil {
			diags.AddError(summary, cxsdk.NewAPIError(httpResp, err).Error())
			return nil, diags
		}
		for i := range out.GetUsers() {
			if strings.EqualFold(out.Users[i].GetUsername(), userName) {
				matches = append(matches, &out.Users[i])
			}
		}
		next := out.GetNextPageToken()
		if next <= pageToken {
			// The token is an offset into the result set. A missing token, or one
			// that does not move forward, means this was the last page, as the
			// released search recorded.
			break
		}
		if page == userLookupPageCap {
			diags.AddError(summary, fmt.Sprintf("The search for user_name %q did not end within %d pages; give up or report this to the provider developers.", userName, userLookupPageCap))
			return nil, diags
		}
		pageToken = next
		if len(out.GetUsers()) == 0 {
			break
		}
	}
	switch len(matches) {
	case 0:
		diags.AddError(fmt.Sprintf("User with user_name %q not found", userName), "")
	case 1:
		if matches[0].GetUserId() == "" {
			diags.AddError(
				fmt.Sprintf("User with user_name %q was returned without an id", userName),
				"Use the user id instead, or report this to the provider developers.",
			)
			return nil, diags
		}
		return matches[0], diags
	default:
		ids := make([]string, 0, len(matches))
		for _, match := range matches {
			if id := match.GetUserId(); id != "" {
				ids = append(ids, id)
			}
		}
		diags.AddError(
			fmt.Sprintf("Multiple Users found with user_name %q", userName),
			fmt.Sprintf("Matched user ids: %s. Use the user id instead.", strings.Join(ids, ", ")),
		)
	}
	return nil, diags
}

// userLookupPageCap is the most pages of a username lookup. The page loop of the
// released search had no cap; the cap only guards a server that paginates forever.
const userLookupPageCap = 100

// UserResource wraps the generated user resource: Update hands the users client to
// the hook through the context, and Configure captures it from the provider data.
type UserResource struct {
	user.Resource
	client *users_management_service.UsersManagementServiceAPIService
}

// Schema narrows the generated schema to the released shape: emails is read-only
// (the set is derived from the username, and a configuration never sets it). The
// generator's extra attributes are Optional+Computed — its parser refuses a
// computed-only object set — so the wrapper narrows it here.
func (r *UserResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	r.Resource.Schema(ctx, req, resp)
	if resp.Diagnostics.HasError() {
		return
	}
	narrowUserEmails(&resp.Schema)
}

// narrowUserEmails makes the emails set and its every descendant computed-only:
// the released schema marked the set computed-only and the framework requires a
// computed-only nested attribute's children to be computed-only. The generator's
// extra attributes are Optional+Computed with a default — its parser refuses a
// computed-only object set — so the wrapper narrows the whole subtree here.
func narrowUserEmails(schema *resourceschema.Schema) {
	emails, ok := schema.Attributes["emails"].(resourceschema.SetNestedAttribute)
	if !ok {
		return
	}
	emails.Optional = false
	emails.Computed = true
	emails.Default = nil
	nested := emails.NestedObject.Attributes
	if primary, ok := nested["primary"].(resourceschema.BoolAttribute); ok {
		primary.Optional = false
		primary.Computed = true
		primary.Default = nil
		nested["primary"] = primary
	}
	for _, name := range []string{"type", "value"} {
		if at, ok := nested[name].(resourceschema.StringAttribute); ok {
			at.Optional = false
			at.Computed = true
			at.Default = nil
			nested[name] = at
		}
	}
	emails.NestedObject.Attributes = nested
	schema.Attributes["emails"] = emails
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
			// An empty name asks for the unfiltered, paged search: the fallback
			// of a read whose hint missed and of a recovery whose username
			// filter answered nothing.
			request := r.client.UsersMgmtServiceSearchUsers(ctx).PageSize(100)
			if username != "" {
				request = request.Username(username)
			}
			out, raw, err := request.Execute()
			if err != nil {
				return out, searchError(raw, err)
			}
			// Page through the rest: a user past the first page is a real user,
			// and missing one would send a PUT without its values, which the
			// backend treats as clear-them. The loop stops when the token stops
			// moving forward, as the released search did: a server that repeats
			// a token cannot make the same page fill the request twice.
			users := out.GetUsers()
			pageToken := out.GetNextPageToken()
			if pageToken == 0 {
				out.Users = users
				return out, nil
			}
			// A break that a terminal token or an empty page took means the
			// answers ended; only the loop condition running out means the
			// budget is exhausted — the two are not the same, and only the
			// second refuses the update.
			exhausted := true
			for page := 1; page < userLookupPageCap; page++ {
				next, raw, err := request.PageToken(pageToken).Execute()
				if err != nil {
					return out, searchError(raw, err)
				}
				users = append(users, next.GetUsers()...)
				nextToken := next.GetNextPageToken()
				if nextToken <= pageToken {
					exhausted = false
					break
				}
				pageToken = nextToken
				if len(next.GetUsers()) == 0 {
					exhausted = false
					break
				}
			}
			out.Users = users
			if exhausted {
				// The loop ran out of pages with the token still moving: a
				// search this deep is the server misbehaving, and continuing
				// would send a PUT without the values the answers may hold.
				return out, fmt.Errorf("the search did not end within %d pages; give up or report this to the provider developers", userLookupPageCap)
			}
			return out, nil
		})
	}
	r.Resource.Update(ctx, req, resp)
}

// searchError carries the API's response body in the error, as the released
// search did: a 500 with {"code":500,…} tells the operator which call failed,
// and the update refuses to run on that error.
func searchError(raw *http.Response, err error) error {
	if raw == nil {
		return err
	}
	body, readErr := io.ReadAll(raw.Body)
	if readErr != nil || len(body) == 0 {
		return err
	}
	// The raw body ends with the newline the gateway appends; the released
	// messages carried the JSON alone.
	return fmt.Errorf("%w — %s", err, bytes.TrimSpace(body))
}

// userGet pages the Search at the released page size: the released read sent
// page_size=100 on every page. The username hint of the released read (it
// narrowed the search to the user it read) narrows the search too, when the
// wrapper put the state's username into the context: a filtered search answers
// only the user's pages, so a page token that a concurrent create shifts cannot
// drop the user from the result, and a team of thousands of users does not page
// the whole directory for one read.
func userGet(ctx context.Context, request func() users_management_service.ApiUsersMgmtServiceSearchUsersRequest) (*users_management_service.SearchUsersResponse, *http.Response, error) {
	r := request().PageSize(100)
	if username, ok := ctx.Value(userReadNameKey{}).(string); ok && username != "" {
		r = r.Username(username)
	}
	return r.Execute()
}

// userReadNameKey is the context key of the username hint of a read: the state's
// user_name, which the wrapper's Read puts there for the Get hook.
type userReadNameKey struct{}

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
	// access type nowhere: the schema of 3.26.0 had no attribute for either, and
	// the generated resource never reads the private state that Terraform passes
	// through. Without a refresh the plan keeps them unknown, and the expand omits
	// them: the PUT would clear the modes and replace the access type with
	// permanent, discarding a temporary access expiry. The hook searches the API for the prior user and sends the modes and
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
				if !cxsdk.IsNotFound(err) && !strings.Contains(err.Error(), "404") {
					diags.AddAttributeError(path.Root("allowed_login_mode"), "Unable to recover the login modes of the user",
						fmt.Sprintf("The search for %q failed: %s. Refresh the resource to read the modes, then update again.", prior.Username.ValueString(), err))
					return diags
				}
			} else if found != nil && len(found.Users) == 0 {
				// The username the state holds can miss the user the id names — a
				// rename outside Terraform. The released findUserByID then ran the
				// search unfiltered; the filtered search of the recovery does the
				// same before the update gives up: sending the PUT without the
				// modes would clear them.
				found, err = search("")
			}
			if err != nil {
				if !cxsdk.IsNotFound(err) && !strings.Contains(err.Error(), "404") {
					diags.AddAttributeError(path.Root("allowed_login_mode"), "Unable to recover the login modes of the user",
						fmt.Sprintf("The search for %q failed: %s. Refresh the resource to read the modes, then update again.", prior.Username.ValueString(), err))
					return diags
				}
			} else if found != nil {
				// The state id names the user when the state carries it — the
				// only key that survives a rename outside Terraform, so the
				// unfiltered answers match by id, and by username only when the
				// state carries no id (the backend folds the case of an SSO
				// login). A user no answer names means the state no longer
				// describes the API: the update stops, because sending the PUT
				// without the modes would clear them.
				matched := false
				for i := range found.Users {
					idMatch := prior.UserId.IsNull() || found.Users[i].GetUserId() == prior.UserId.ValueString()
					if !idMatch {
						continue
					}
					matched = true
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
				if !matched && !prior.UserId.IsNull() {
					// Neither search named the user the state carries: the state
					// no longer describes the API, and the PUT of a user the API
					// may not hold would clear the modes and the access type.
					diags.AddAttributeError(path.Root("allowed_login_mode"), "Unable to recover the login modes of the user",
						fmt.Sprintf("The state names user %q (id %s), but the API answers no user with that id or name. Refresh the resource, or import the user again, then update.", prior.Username.ValueString(), prior.UserId.ValueString()))
					return diags
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
	// The API stores an omitted name as "" and cannot tell a null component from
	// an empty one. The released preserveUserName kept the prior name only when
	// both of its components said the same thing as the API — an empty string
	// reads the same as a null — so a name the API cleared leaves the state, and
	// a configuration whose component is explicitly empty keeps its own form.
	// m.Name here is the value setState kept from the plan or the prior state,
	// or the flatten's own value when there was no prior.
	apiGiven, apiFamily := emptyAsNull(v.FirstName), emptyAsNull(v.LastName)
	apiCleared := apiGiven.IsNull() && apiFamily.IsNull()
	// The released flattenUserName: with any name present, a missing component
	// reads as ""; with no name at all, the object is null.
	given, family := apiGiven, apiFamily
	if !apiCleared {
		if given.IsNull() {
			given = basetypes.NewStringValue("")
		}
		if family.IsNull() {
			family = basetypes.NewStringValue("")
		}
	}
	name := userName{GivenName: given, FamilyName: family}
	keepName := false
	if !m.Name.IsNull() && !m.Name.IsUnknown() {
		// The released preserveUserName: the prior form survives when both of
		// its components read the same as the API's (a null component reads as
		// ""). A prior the API contradicts follows the API — including a name
		// the API cleared, whose prior form with a real component leaves the
		// state.
		var prior userName
		d := m.Name.As(ctx, &prior, basetypes.ObjectAsOptions{})
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		keepName = prior.GivenName.ValueString() == name.GivenName.ValueString() &&
			prior.FamilyName.ValueString() == name.FamilyName.ValueString()
	}
	if !keepName {
		if apiCleared {
			// The API holds no name at all: the released flattenUserName wrote
			// the null object.
			m.Name = basetypes.NewObjectNull(userNameAttrTypes)
		} else {
			// The released flattenUserName wrote an empty string — not null —
			// for the one component the API does not hold: a read without a
			// prior (an import, a data source) shows the name the released
			// resource showed, so a consumer that passes the component to a
			// string function keeps working.
			object, d := basetypes.NewObjectValue(userNameAttrTypes, map[string]attr.Value{
				"given_name":  name.GivenName,
				"family_name": name.FamilyName,
			})
			diags.Append(d...)
			m.Name = object
		}
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
	// emails is the released view of the username: the set holds exactly one
	// primary work email whose value is the username. The plan carries the same
	// set (userUpdatePlan derives it from the configured username), so the state
	// of a write correlates with the plan, and the read keeps the spelling of the
	// configuration.
	emails, d := deriveUserEmails(ctx, m.Username)
	diags.Append(d...)
	m.Emails = emails
	return diags
}

// userAfterGet repairs the emails of the written state after the plan correlation
// ran, when a read follows the write. The create without a following read writes
// the flatten's set, which the planned set matches; a read writes the set the API
// answer carries.
func userAfterGet(ctx context.Context, state *tfsdk.State, _ *users_management_service.SearchUsersResponse) diag.Diagnostics {
	var diags diag.Diagnostics
	var model user.RbacV2UserModel
	if d := state.Get(ctx, &model); d.HasError() {
		diags.Append(d...)
		return diags
	}
	emails, d := deriveUserEmails(ctx, model.Username)
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	diags.Append(state.SetAttribute(ctx, path.Root("emails"), emails)...)
	return diags
}

// userEmailAttrTypes are the attribute types of one emails entry.
var userEmailAttrTypes = map[string]attr.Type{
	"primary": basetypes.BoolType{},
	"type":    basetypes.StringType{},
	"value":   basetypes.StringType{},
}

// deriveUserEmails builds the computed emails set from the username: exactly one
// primary work email, as the released resource recorded.
func deriveUserEmails(ctx context.Context, username basetypes.StringValue) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics
	if username.IsNull() || username.IsUnknown() {
		return types.SetNull(types.ObjectType{AttrTypes: userEmailAttrTypes}), diags
	}
	object, d := basetypes.NewObjectValue(userEmailAttrTypes, map[string]attr.Value{
		"primary": basetypes.NewBoolValue(true),
		"type":    basetypes.NewStringValue(userEmailType),
		"value":   basetypes.NewStringValue(username.ValueString()),
	})
	diags.Append(d...)
	if diags.HasError() {
		return types.SetNull(types.ObjectType{AttrTypes: userEmailAttrTypes}), diags
	}
	set, d := basetypes.NewSetValueFrom(ctx, types.ObjectType{AttrTypes: userEmailAttrTypes}, []attr.Value{object})
	diags.Append(d...)
	return set, diags
}

// userEmailType is the email type SCIM returned for every user.
const userEmailType = "work"
