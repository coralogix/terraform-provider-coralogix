package coralogix

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"terraform-provider-coralogix/coralogix/clientset"
	teams "terraform-provider-coralogix/coralogix/clientset/grpc/teams"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	getTeamQuotaURL = "com.coralogixapis.aaa.organisations.v2.TeamService/GetTeamQuota"
	movingQuotaURL  = "com.coralogixapis.aaa.organisations.v2.TeamService/MoveQuota"
)

func NewMovingQuotaResource() resource.Resource {
	return &MovingQuotaResource{}
}

type MovingQuotaResource struct {
	client *clientset.TeamsClient
}

func (r *MovingQuotaResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_moving_quota"

}

func (r *MovingQuotaResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *MovingQuotaResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Team ID.",
			},
			"source_team_id": schema.StringAttribute{
				Required: true,
			},
			"source_team_quota": schema.Float64Attribute{
				Computed: true,
			},
			"destination_team_id": schema.StringAttribute{
				Required: true,
			},
			"destination_team_quota": schema.Float64Attribute{
				Computed: true,
			},
			"desired_source_team_quota": schema.Float64Attribute{
				Required: true,
			},
		},
		MarkdownDescription: "This resource is used to move quota from one team to another.",
	}
}

type MovingQuotaResourceModel struct {
	ID                          types.String  `tfsdk:"id"`
	SourceTeamID                types.String  `tfsdk:"source_team_id"`
	SourceTeamQuota             types.Float64 `tfsdk:"source_team_quota"`
	DestinationTeamID           types.String  `tfsdk:"destination_team_id"`
	DestinationTeamQuota        types.Float64 `tfsdk:"destination_team_quota"`
	DesiredDestinationTeamQuota types.Float64 `tfsdk:"desired_source_team_quota"`
}

func (r *MovingQuotaResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *MovingQuotaResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state, diag := r.UpdateOrCreate(ctx, plan)
	if diag != nil {
		resp.Diagnostics.Append(diag)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *MovingQuotaResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var plan *MovingQuotaResourceModel
	diags := req.State.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	sourceTeamIdInt, err := strconv.Atoi(plan.SourceTeamID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error getting Team quota",
			err.Error(),
		)
		return
	}
	destinationTeamIdInt, err := strconv.Atoi(plan.DestinationTeamID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error getting Team quota",
			err.Error(),
		)
		return
	}

	getSourceTeamQuotaReq := &teams.GetTeamQuotaRequest{
		TeamId: &teams.TeamId{
			Id: uint32(sourceTeamIdInt),
		},
	}
	log.Printf("[INFO] Getting Team quota: %s", protojson.Format(getSourceTeamQuotaReq))
	getSourceTeamQuotaResp, err := r.client.GetTeamQuota(ctx, getSourceTeamQuotaReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error getting Team quota",
			formatRpcErrors(err, getTeamQuotaURL, protojson.Format(getSourceTeamQuotaReq)),
		)
		return
	}
	log.Printf("[INFO] Retrieved Team quota: %s", protojson.Format(getSourceTeamQuotaResp))

	getDestinationTeamQuotaReq := &teams.GetTeamQuotaRequest{
		TeamId: &teams.TeamId{
			Id: uint32(destinationTeamIdInt),
		},
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Error getting Team quota",
			err.Error(),
		)
		return
	}
	log.Printf("[INFO] Getting Team quota: %s", protojson.Format(getDestinationTeamQuotaReq))
	getDestinationTeamQuotaResp, err := r.client.GetTeamQuota(ctx, getDestinationTeamQuotaReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error getting Team quota",
			formatRpcErrors(err, getTeamQuotaURL, protojson.Format(getSourceTeamQuotaReq)),
		)
		return
	}
	log.Printf("[INFO] Retrieved Team quota: %s", protojson.Format(getDestinationTeamQuotaResp))

	state := &MovingQuotaResourceModel{
		ID:                   types.StringValue(""),
		SourceTeamID:         plan.SourceTeamID,
		SourceTeamQuota:      types.Float64Value(float64(getSourceTeamQuotaResp.GetQuota())),
		DestinationTeamID:    plan.DestinationTeamID,
		DestinationTeamQuota: types.Float64Value(float64(getDestinationTeamQuotaResp.GetQuota())),
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *MovingQuotaResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan *MovingQuotaResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state, diag := r.UpdateOrCreate(ctx, plan)
	if diag != nil {
		resp.Diagnostics.Append(diag)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *MovingQuotaResource) UpdateOrCreate(ctx context.Context, plan *MovingQuotaResourceModel) (*MovingQuotaResourceModel, diag.Diagnostic) {
	sourceTeamIdInt, err := strconv.Atoi(plan.SourceTeamID.ValueString())
	if err != nil {
		return nil, diag.NewErrorDiagnostic(fmt.Sprintf("Error parsing Team %s quota", plan.SourceTeamID.ValueString()), err.Error())
	}
	destinationTeamIdInt, err := strconv.Atoi(plan.DestinationTeamID.ValueString())
	if err != nil {
		return nil, diag.NewErrorDiagnostic(fmt.Sprintf("Error getting Team %s quota", plan.DestinationTeamID.ValueString()), err.Error())
	}

	getSourceTeamQuotaReq := &teams.GetTeamQuotaRequest{
		TeamId: &teams.TeamId{
			Id: uint32(sourceTeamIdInt),
		},
	}
	log.Printf("[INFO] Getting Team quota: %s", protojson.Format(getSourceTeamQuotaReq))
	getSourceTeamQuotaResp, err := r.client.GetTeamQuota(ctx, getSourceTeamQuotaReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		return nil, diag.NewErrorDiagnostic("Error getting Team quota", formatRpcErrors(err, getTeamQuotaURL, protojson.Format(getSourceTeamQuotaReq)))
	}
	log.Printf("[INFO] Retrieved Team quota: %s", protojson.Format(getSourceTeamQuotaResp))

	getDestinationTeamQuotaReq := &teams.GetTeamQuotaRequest{
		TeamId: &teams.TeamId{
			Id: uint32(destinationTeamIdInt),
		},
	}
	log.Printf("[INFO] Getting Team quota: %s", protojson.Format(getDestinationTeamQuotaReq))
	getDestinationTeamQuotaResp, err := r.client.GetTeamQuota(ctx, getDestinationTeamQuotaReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		return nil, diag.NewErrorDiagnostic("Error getting Team quota", formatRpcErrors(err, getTeamQuotaURL, protojson.Format(getSourceTeamQuotaReq)))
	}
	log.Printf("[INFO] Retrieved Team quota: %s", protojson.Format(getDestinationTeamQuotaResp))

	unitsToMove := float32(plan.DesiredDestinationTeamQuota.ValueFloat64()) - getDestinationTeamQuotaResp.GetQuota()
	if unitsToMove < 0 {
		return nil, diag.NewErrorDiagnostic("Error moving Team quota", fmt.Sprintf("Desired source team quota (%f) is less than destination team quota (%f).", plan.DesiredDestinationTeamQuota.ValueFloat64(), getDestinationTeamQuotaResp.GetQuota()))
	} else if unitsToMove == 0 {
		return &MovingQuotaResourceModel{
			ID:                          types.StringValue(""),
			SourceTeamID:                plan.SourceTeamID,
			SourceTeamQuota:             types.Float64Value(float64(getSourceTeamQuotaResp.GetQuota())),
			DestinationTeamID:           plan.DestinationTeamID,
			DestinationTeamQuota:        types.Float64Value(float64(getDestinationTeamQuotaResp.GetQuota())),
			DesiredDestinationTeamQuota: plan.DesiredDestinationTeamQuota,
		}, nil
	}
	if unitsToMove > getSourceTeamQuotaResp.GetQuota() {
		return nil, diag.NewErrorDiagnostic("Error moving Team quota", fmt.Sprintf("Desired source team quota (%f) is greater than source team quota (%f).", plan.DesiredDestinationTeamQuota.ValueFloat64(), getSourceTeamQuotaResp.GetQuota()))
	}

	moveQuotaReq := &teams.MoveQuotaRequest{
		SourceTeam:      &teams.TeamId{Id: uint32(sourceTeamIdInt)},
		DestinationTeam: &teams.TeamId{Id: uint32(destinationTeamIdInt)},
		UnitsToMove:     unitsToMove,
	}
	log.Printf("[INFO] Moving Team quota: %s", protojson.Format(moveQuotaReq))
	moveQuotaResp, err := r.client.MoveQuota(ctx, moveQuotaReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.PermissionDenied || status.Code(err) == codes.Unauthenticated {
			return nil, diag.NewErrorDiagnostic(
				"Error moving Team quota",
				fmt.Sprintf("permission denied for url - %s\ncheck your org-key and permissions", movingQuotaURL),
			)
		}
		return nil, diag.NewErrorDiagnostic("Error moving Team quota", formatRpcErrors(err, movingQuotaURL, protojson.Format(moveQuotaReq)))
	}
	log.Printf("[INFO] Moved Team quota: %s", protojson.Format(moveQuotaResp))
	return &MovingQuotaResourceModel{
		ID:                          types.StringValue(""),
		SourceTeamID:                plan.SourceTeamID,
		SourceTeamQuota:             types.Float64Value(float64(moveQuotaResp.GetSourceTeamQuota())),
		DestinationTeamID:           plan.DestinationTeamID,
		DestinationTeamQuota:        types.Float64Value(float64(moveQuotaResp.GetDestinationTeamQuota())),
		DesiredDestinationTeamQuota: plan.DesiredDestinationTeamQuota,
	}, nil
}

func (r *MovingQuotaResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {

}
