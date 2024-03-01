package coralogix

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"terraform-provider-coralogix/coralogix/clientset"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func NewGroupResource() resource.Resource {
	return &GroupResource{}
}

type GroupResource struct {
	client *clientset.GroupsClient
}

func (r *GroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (r *GroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.Groups()
}

func (r *GroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Group ID.",
			},
			"display_name": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Group display name.",
			},
			"members": schema.SetAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
			"role": schema.StringAttribute{
				Required: true,
			},
		},
		MarkdownDescription: "Coralogix group.",
	}
}

func (r *GroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *GroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *GroupResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createGroupRequest, diags := extractGroup(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	groupStr, _ := json.Marshal(createGroupRequest)
	log.Printf("[INFO] Creating new group: %s", string(groupStr))
	createResp, err := r.client.CreateGroup(ctx, createGroupRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error creating Group",
			formatRpcErrors(err, r.client.TargetUrl, string(groupStr)),
		)
		return
	}
	groupStr, _ = json.Marshal(createResp)
	log.Printf("[INFO] Submitted new group: %s", groupStr)

	state, diags := flattenSCIMGroup(createResp)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func flattenSCIMGroup(group *clientset.SCIMGroup) (*GroupResourceModel, diag.Diagnostics) {
	members, diags := flattenSCIMGroupMembers(group.Members)
	if diags.HasError() {
		return nil, diags
	}

	return &GroupResourceModel{
		ID:          types.StringValue(group.ID),
		DisplayName: types.StringValue(group.DisplayName),
		Members:     members,
		Role:        types.StringValue(group.Role),
	}, nil
}

func flattenSCIMGroupMembers(members []clientset.SCIMGroupMember) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics
	membersIDs := make([]attr.Value, 0, len(members))
	for _, member := range members {
		membersIDs = append(membersIDs, types.StringValue(member.Value))
	}
	if diags.HasError() {
		return types.SetNull(types.StringType), diags
	}

	return types.SetValue(types.StringType, membersIDs)
}

func (r *GroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *GroupResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed Group value from Coralogix
	id := state.ID.ValueString()
	log.Printf("[INFO] Reading Group: %s", id)
	getGroupResp, err := r.client.GetGroup(ctx, id)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Group %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Group",
				formatRpcErrors(err, fmt.Sprintf("%s/%s", r.client.TargetUrl, id), ""),
			)
		}
		return
	}
	respStr, _ := json.Marshal(getGroupResp)
	log.Printf("[INFO] Received Group: %s", string(respStr))

	state, diags = flattenSCIMGroup(getGroupResp)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *GroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan *GroupResourceModel
	diags := req.Plan.Get(ctx, &plan)

	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupUpdateReq, diags := extractGroup(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	groupStr, _ := json.Marshal(groupUpdateReq)
	log.Printf("[INFO] Updating Group: %s", string(groupStr))
	groupID := plan.ID.ValueString()
	groupUpdateResp, err := r.client.UpdateGroup(ctx, groupID, groupUpdateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating Group",
			formatRpcErrors(err, fmt.Sprintf("%s/%s", r.client.TargetUrl, groupUpdateReq.ID), string(groupStr)),
		)
		return
	}
	groupStr, _ = json.Marshal(groupUpdateResp)
	log.Printf("[INFO] Submitted updated Group: %s", string(groupStr))

	// Get refreshed Group value from Coralogix
	id := plan.ID.ValueString()
	getGroupResp, err := r.client.GetGroup(ctx, id)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Group %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Group",
				formatRpcErrors(err, fmt.Sprintf("%s/%s", r.client.TargetUrl, id), string(groupStr)),
			)
		}
		return
	}
	groupStr, _ = json.Marshal(getGroupResp)
	log.Printf("[INFO] Received Group: %s", string(groupStr))

	state, diags := flattenSCIMGroup(getGroupResp)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *GroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state *GroupResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	log.Printf("[INFO] Deleting Group %s", id)
	if err := r.client.DeleteGroup(ctx, id); err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting Group %s", id),
			formatRpcErrors(err, fmt.Sprintf("%s/%s", r.client.TargetUrl, id), ""),
		)
		return
	}
	log.Printf("[INFO] Group %s deleted", id)
}

type GroupResourceModel struct {
	ID          types.String `tfsdk:"id"`
	DisplayName types.String `tfsdk:"display_name"`
	Members     types.Set    `tfsdk:"members"` // Set of strings
	Role        types.String `tfsdk:"role"`
}

func extractGroup(ctx context.Context, plan *GroupResourceModel) (*clientset.SCIMGroup, diag.Diagnostics) {

	members, diags := extractGroupMembers(ctx, plan.Members)
	if diags.HasError() {
		return nil, diags
	}

	return &clientset.SCIMGroup{
		DisplayName: plan.DisplayName.ValueString(),
		Members:     members,
		Role:        plan.Role.ValueString(),
	}, nil
}

func extractGroupMembers(ctx context.Context, members types.Set) ([]clientset.SCIMGroupMember, diag.Diagnostics) {
	membersElements := members.Elements()
	groupMembers := make([]clientset.SCIMGroupMember, 0, len(membersElements))
	var diags diag.Diagnostics
	for _, member := range membersElements {
		val, err := member.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Failed to convert value to Terraform", err.Error())
			continue
		}
		var str string

		if err = val.As(&str); err != nil {
			diags.AddError("Failed to convert value to string", err.Error())
			continue
		}
		groupMembers = append(groupMembers, clientset.SCIMGroupMember{Value: str})
	}
	if diags.HasError() {
		return nil, diags
	}
	return groupMembers, nil
}
