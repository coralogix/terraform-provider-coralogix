package coralogix

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

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
	"google.golang.org/grpc/status"
	"terraform-provider-coralogix/coralogix/clientset"
)

func NewUserResource() resource.Resource {
	return &UserResource{}
}

type UserResource struct {
	client *clientset.UsersClient
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
	createResp, err := r.client.CreateUser(ctx, createUserRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error creating User",
			formatRpcErrors(err, r.client.TargetUrl, string(userStr)),
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

func flattenSCIMUser(ctx context.Context, user *clientset.SCIMUser) (*UserResourceModel, diag.Diagnostics) {
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

func flattenSCIMUserEmails(ctx context.Context, emails []clientset.SCIMUserEmail) (types.Set, diag.Diagnostics) {
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

func flattenSCIMUserName(ctx context.Context, name *clientset.SCIMUserName) (types.Object, diag.Diagnostics) {
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

func flattenSCIMUserGroups(ctx context.Context, groups []clientset.SCIMUserGroup) (types.Set, diag.Diagnostics) {
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
	getUserResp, err := r.client.GetUser(ctx, id)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("User %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading User",
				formatRpcErrors(err, fmt.Sprintf("%s/%s", r.client.TargetUrl, id), ""),
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
	userUpdateResp, err := r.client.UpdateUser(ctx, userID, userUpdateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating User",
			formatRpcErrors(err, fmt.Sprintf("%s/%s", r.client.TargetUrl, userID), string(userStr)),
		)
		return
	}
	userStr, _ = json.Marshal(userUpdateResp)
	log.Printf("[INFO] Submitted updated User: %s", string(userStr))

	// Get refreshed User value from Coralogix
	id := plan.ID.ValueString()
	getUserResp, err := r.client.GetUser(ctx, id)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("User %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading User",
				formatRpcErrors(err, fmt.Sprintf("%s/%s", r.client.TargetUrl, id), string(userStr)),
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
	if err := r.client.DeleteUser(ctx, id); err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting User %s", id),
			formatRpcErrors(err, fmt.Sprintf("%s/%s", r.client.TargetUrl, id), ""),
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

func extractCreateUser(ctx context.Context, plan *UserResourceModel) (*clientset.SCIMUser, diag.Diagnostics) {
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

	return &clientset.SCIMUser{
		Schemas:  []string{},
		UserName: plan.UserName.ValueString(),
		Name:     name,
		Active:   plan.Active.ValueBool(),
		Emails:   emails,
		Groups:   groups,
	}, nil
}

func extractUserGroups(ctx context.Context, groups types.Set) ([]clientset.SCIMUserGroup, diag.Diagnostics) {
	groupsElements := groups.Elements()
	userGroups := make([]clientset.SCIMUserGroup, 0, len(groupsElements))
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
		userGroups = append(userGroups, clientset.SCIMUserGroup{Value: str})
	}
	if diags.HasError() {
		return nil, diags
	}
	return userGroups, nil
}

func extractUserSCIMName(ctx context.Context, name types.Object) (*clientset.SCIMUserName, diag.Diagnostics) {
	if name.IsNull() || name.IsUnknown() {
		return nil, nil
	}
	var nameModel UserNameModel
	diags := name.As(ctx, &nameModel, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	return &clientset.SCIMUserName{
		GivenName:  nameModel.GivenName.ValueString(),
		FamilyName: nameModel.FamilyName.ValueString(),
	}, nil
}

func extractUserEmails(ctx context.Context, emails types.Set) ([]clientset.SCIMUserEmail, diag.Diagnostics) {
	var diags diag.Diagnostics
	var emailsObjects []types.Object
	var expandedEmails []clientset.SCIMUserEmail
	emails.ElementsAs(ctx, &emailsObjects, true)

	for _, eo := range emailsObjects {
		var email UserEmailModel
		if dg := eo.As(ctx, &email, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedEmail := clientset.SCIMUserEmail{
			Value:   email.Value.ValueString(),
			Primary: email.Primary.ValueBool(),
			Type:    email.Type.ValueString(),
		}
		expandedEmails = append(expandedEmails, expandedEmail)
	}

	return expandedEmails, diags
}
