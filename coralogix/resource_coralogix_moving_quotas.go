package coralogix

import (
	"context"
	"fmt"
	"strconv"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"terraform-provider-coralogix/coralogix/clientset"
)

func NewTeamQuotaAssignmentResource() resource.Resource {
	return &TeamQuotaAssignmentResource{}
}

type TeamQuotaAssignmentResource struct {
	client *cxsdk.TeamsClient
}

// TeamQuotaAssignmentResourceModel defines the resource model for team quota assignments.
type TeamQuotaAssignmentResourceModel struct {
	ID                types.String  `tfsdk:"id"`
	SourceTeamID      types.String  `tfsdk:"source_team_id"`
	DestinationTeamID types.String  `tfsdk:"destination_team_id"`
	DesiredQuota      types.Float32 `tfsdk:"desired_quota"`
}

func (r *TeamQuotaAssignmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team_quota_assignment"

}

func (r *TeamQuotaAssignmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.Teams()
}

func (r *TeamQuotaAssignmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"source_team_id": schema.StringAttribute{
				Required: true,
			},
			"destination_team_id": schema.StringAttribute{
				Required: true,
			},
			"desired_quota": schema.Float32Attribute{
				Required: true,
			},
		},
		MarkdownDescription: "Coralogix Team.",
		DeprecationMessage:  "This resource is broken and will be removed in an upcoming release.",
	}
}

func (r *TeamQuotaAssignmentResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var plan TeamQuotaAssignmentResourceModel
	diags := request.Plan.Get(ctx, &plan)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	sourceTeamId, err := teamIdStrToTeamId(plan.SourceTeamID)
	if err != nil {
		response.Diagnostics.AddError(
			"Invalid Source Team ID",
			fmt.Sprintf("Source Team ID must be a valid integer: %s", err),
		)
		return
	}

	destinationTeamId, err := teamIdStrToTeamId(plan.DestinationTeamID)
	if err != nil {
		response.Diagnostics.AddError(
			"Invalid Destination Team ID",
			fmt.Sprintf("Destination Team ID must be a valid integer: %s", err),
		)
		return
	}

	sourceTeamQuotas, err := r.client.GetQuota(ctx, &cxsdk.GetTeamQuotaRequest{TeamId: sourceTeamId})
	if err != nil {
		response.Diagnostics.AddError(
			"Error Retrieving Source Team Quota",
			fmt.Sprintf("Could not retrieve source team quota: %s", err),
		)
		return
	}

	destinationTeamQuotas, err := r.client.GetQuota(ctx, &cxsdk.GetTeamQuotaRequest{TeamId: destinationTeamId})
	if err != nil {
		response.Diagnostics.AddError(
			"Error Retrieving Destination Team Quota",
			fmt.Sprintf("Could not retrieve destination team quota: %s", err),
		)
		return
	}

	unitsToMove := plan.DesiredQuota.ValueFloat32() - destinationTeamQuotas.Quota
	if unitsToMove > sourceTeamQuotas.Quota {
		response.Diagnostics.AddError(
			"Insufficient Quota",
			fmt.Sprintf("Not enough quota in source team. Available: %f, Requested: %f", sourceTeamQuotas.Quota, plan.DesiredQuota),
		)
		return
	}

	// Create the quota assignment
	moveQuotaRequest := &cxsdk.MoveQuotaRequest{
		SourceTeam:      sourceTeamId,
		DestinationTeam: destinationTeamId,
		UnitsToMove:     float64(unitsToMove),
	}

	_, err = r.client.MoveQuota(ctx, moveQuotaRequest)
	if err != nil {
		response.Diagnostics.AddError(
			"Error Creating Team Quota Assignment",
			fmt.Sprintf("Could not create team quota assignment: %s", err),
		)
		return
	}

	// Set ID and return
	plan.ID = types.StringValue(fmt.Sprintf("%d-%d", sourceTeamId.Id, destinationTeamId.Id))
	response.State.Set(ctx, &plan)
}

func (r *TeamQuotaAssignmentResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var state TeamQuotaAssignmentResourceModel
	diags := request.State.Get(ctx, &state)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	// Read the quota assignment based on the ID
	destinationTeamId, err := teamIdStrToTeamId(state.DestinationTeamID)
	if err != nil {
		response.Diagnostics.AddError(
			"Invalid Destination Team ID",
			fmt.Sprintf("Destination Team ID must be a valid integer: %s", err),
		)
		return
	}

	quota, err := r.client.GetQuota(ctx, &cxsdk.GetTeamQuotaRequest{TeamId: destinationTeamId})
	if err != nil {
		response.Diagnostics.AddError(
			"Error Reading Team Quota Assignment",
			fmt.Sprintf("Could not read team quota assignment: %s", err),
		)
		return
	}

	state.DesiredQuota = types.Float32Value(quota.Quota)
	response.State.Set(ctx, &state)
}

func (r *TeamQuotaAssignmentResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var plan TeamQuotaAssignmentResourceModel
	diags := request.Plan.Get(ctx, &plan)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	// Update the quota assignment
	sourceTeamId, err := teamIdStrToTeamId(plan.SourceTeamID)
	if err != nil {
		response.Diagnostics.AddError(
			"Invalid Source Team ID",
			fmt.Sprintf("Source Team ID must be a valid integer: %s", err),
		)
		return
	}

	destinationTeamId, err := teamIdStrToTeamId(plan.DestinationTeamID)
	if err != nil {
		response.Diagnostics.AddError(
			"Invalid Destination Team ID",
			fmt.Sprintf("Destination Team ID must be a valid integer: %s", err),
		)
		return
	}

	moveQuotaRequest := &cxsdk.MoveQuotaRequest{
		SourceTeam:      sourceTeamId,
		DestinationTeam: destinationTeamId,
		UnitsToMove:     float64(plan.DesiredQuota.ValueFloat32()),
	}

	_, err = r.client.MoveQuota(ctx, moveQuotaRequest)
	if err != nil {
		response.Diagnostics.AddError(
			"Error Updating Team Quota Assignment",
			fmt.Sprintf("Could not update team quota assignment: %s", err),
		)
		return
	}

	response.State.Set(ctx, &plan)
}

func (r *TeamQuotaAssignmentResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	response.State.RemoveResource(ctx)
}

func teamIdStrToTeamId(teamIdStr types.String) (*cxsdk.TeamID, error) {
	teamId, err := strconv.Atoi(teamIdStr.ValueString())
	if err != nil {
		return nil, fmt.Errorf("invalid team ID: %s", teamIdStr)
	}
	return &cxsdk.TeamID{Id: uint32(teamId)}, nil
}
