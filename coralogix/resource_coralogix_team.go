package coralogix

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"terraform-provider-coralogix/coralogix/clientset"
	teams "terraform-provider-coralogix/coralogix/clientset/grpc/teams"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64planmodifier"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	createTeamURL = "com.coralogixapis.aaa.organisations.v2.TeamService/CreateTeamInOrg"
	updateTeamURL = "com.coralogixapis.aaa.organisations.v2.TeamService/UpdateTeam"
	deleteTeamURL = "com.coralogixapis.aaa.organisations.v2.TeamService/DeleteTeam"
)

func NewTeamResource() resource.Resource {
	return &TeamResource{}
}

type TeamResource struct {
	client *clientset.TeamsClient
}

func (r *TeamResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team"

}

func (r *TeamResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *TeamResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *TeamResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
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
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Team name.",
			},
			"retention": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Team retention.",
			},
			"daily_quota": schema.Float64Attribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Team quota. Optional, Default daily quota is 0.01 units/day.",
				PlanModifiers: []planmodifier.Float64{
					float64planmodifier.UseStateForUnknown(),
				},
			},
		},
		MarkdownDescription: "Coralogix Team.",
	}
}

type TeamResourceModel struct {
	ID         types.String  `tfsdk:"id"`
	Name       types.String  `tfsdk:"name"`
	Retention  types.Int64   `tfsdk:"retention"`
	DailyQuota types.Float64 `tfsdk:"daily_quota"`
}

func (r *TeamResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *TeamResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTeamReq, diags := extractCreateTeam(plan)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
	log.Printf("[INFO] Creating new Team: %s", protojson.Format(createTeamReq))
	createTeamResp, err := r.client.CreateTeam(ctx, createTeamReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.PermissionDenied || status.Code(err) == codes.Unauthenticated {
			resp.Diagnostics.AddError(
				"Error creating Team",
				fmt.Sprintf("permission denied for url - %s\ncheck your org-key and permissions", createTeamURL),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error creating Team",
				formatRpcErrors(err, createTeamURL, protojson.Format(createTeamReq)),
			)
		}

		return
	}
	log.Printf("[INFO] Submitted new team: %s", protojson.Format(createTeamResp.GetTeamId()))
	plan = flattenTeam(createTeamReq, createTeamResp)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func extractCreateTeam(plan *TeamResourceModel) (*teams.CreateTeamInOrgRequest, diag.Diagnostics) {
	var dailyQuota *float64
	if !(plan.DailyQuota.IsUnknown() || plan.DailyQuota.IsNull()) {
		dailyQuota = new(float64)
		*dailyQuota = plan.DailyQuota.ValueFloat64()
	}

	return &teams.CreateTeamInOrgRequest{
		TeamName:   plan.Name.ValueString(),
		Retention:  int32(plan.Retention.ValueInt64()),
		DailyQuota: dailyQuota,
	}, nil
}

func flattenTeam(req *teams.CreateTeamInOrgRequest, resp *teams.CreateTeamInOrgResponse) *TeamResourceModel {
	return &TeamResourceModel{
		ID:         types.StringValue(strconv.Itoa(int(resp.GetTeamId().GetId()))),
		Name:       types.StringValue(req.GetTeamName()),
		Retention:  types.Int64Value(int64(req.GetRetention())),
		DailyQuota: types.Float64Value(req.GetDailyQuota()),
	}
}

func (r *TeamResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var plan *TeamResourceModel
	diags := req.State.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *TeamResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state *TeamResourceModel
	diags := req.State.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	diags = req.Plan.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.Retention.ValueInt64() != state.Retention.ValueInt64() {
		resp.Diagnostics.AddError(
			"Error updating Team",
			"Team retention cannot be updated.",
		)
	}

	updateReq, diags := extractUpdateTeam(plan)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
	log.Printf("[INFO] Updating Team: %s", protojson.Format(updateReq))

	_, err := r.client.UpdateTeam(ctx, updateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.PermissionDenied || status.Code(err) == codes.Unauthenticated {
			resp.Diagnostics.AddError(
				"Error updating Team",
				fmt.Sprintf("permission denied for url - %s\ncheck your org-key and permissions", updateTeamURL),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error updating Team",
				formatRpcErrors(err, updateTeamURL, protojson.Format(updateReq)),
			)
		}

		return
	}

	log.Printf("[INFO] Updated team: %s", plan.ID.ValueString())

	// Set state to fully populated data
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func extractUpdateTeam(plan *TeamResourceModel) (*teams.UpdateTeamRequest, diag.Diagnostics) {
	var dailyQuota *float64
	if !(plan.DailyQuota.IsUnknown() || plan.DailyQuota.IsNull()) {
		dailyQuota = new(float64)
		*dailyQuota = plan.DailyQuota.ValueFloat64()
	}

	id, err := strconv.Atoi(plan.ID.ValueString())
	if err != nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error converting team id to int", err.Error())}
	}
	teamId := &teams.TeamId{Id: uint32(id)}

	teamName := new(string)
	*teamName = plan.Name.ValueString()

	return &teams.UpdateTeamRequest{
		TeamId:     teamId,
		TeamName:   teamName,
		DailyQuota: dailyQuota,
	}, nil
}

func (r *TeamResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state *TeamResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	log.Printf("[INFO] Deleting Team: %s", state.ID.ValueString())
	id, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting Team",
			fmt.Sprintf("Error converting team id to int: %s", err.Error()),
		)
		return
	}

	deleteReq := &teams.DeleteTeamRequest{TeamId: &teams.TeamId{Id: uint32(id)}}
	log.Printf("[INFO] Deleting Team: %s", protojson.Format(deleteReq))
	_, err = r.client.DeleteTeam(ctx, deleteReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.PermissionDenied || status.Code(err) == codes.Unauthenticated {
			resp.Diagnostics.AddError(
				"Error deleting Team",
				fmt.Sprintf("permission denied for url - %s\ncheck your org-key and permissions", deleteTeamURL),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error deleting Team",
				formatRpcErrors(err, deleteTeamURL, protojson.Format(deleteReq)),
			)
		}
		return
	}
	log.Printf("[INFO] Deleted team: %s", state.ID.ValueString())
}
