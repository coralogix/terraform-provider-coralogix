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
			"team_admins_emails": schema.SetAttribute{
				ElementType:         types.StringType,
				Required:            true,
				MarkdownDescription: "Team admins emails.",
			},
			"retention": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Team retention.",
			},
			"send_data_key": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Team send data key. Generated on creation.",
			},
		},
		MarkdownDescription: "Coralogix Team.",
	}
}

type TeamResourceModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	TeamAdminsEmails types.Set    `tfsdk:"team_admins_emails"` //types.String
	Retention        types.Int64  `tfsdk:"retention"`
	SendDataKey      types.String `tfsdk:"send_data_key"`
}

func (r *TeamResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *TeamResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTeamReq, diags := extractCreateTeam(ctx, plan)
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

func extractCreateTeam(ctx context.Context, plan *TeamResourceModel) (*teams.CreateTeamInOrgRequest, diag.Diagnostics) {
	emails, diags := typeStringSliceToStringSlice(ctx, plan.TeamAdminsEmails.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &teams.CreateTeamInOrgRequest{
		TeamName:        plan.Name.ValueString(),
		Retention:       int32(plan.Retention.ValueInt64()),
		TeamAdminsEmail: emails,
	}, nil
}

func flattenTeam(req *teams.CreateTeamInOrgRequest, resp *teams.CreateTeamInOrgResponse) *TeamResourceModel {
	return &TeamResourceModel{
		ID:               types.StringValue(strconv.Itoa(int(resp.GetTeamId().GetId()))),
		Name:             types.StringValue(req.GetTeamName()),
		TeamAdminsEmails: stringSliceToTypeStringSet(req.GetTeamAdminsEmail()),
		Retention:        types.Int64Value(int64(req.GetRetention())),
		SendDataKey:      types.StringValue(resp.GetSendDataKey()),
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

	if plan.Name.ValueString() != state.Name.ValueString() {
		resp.Diagnostics.AddError(
			"Error updating Team",
			"Team name cannot be updated.",
		)
	}

	if plan.Retention.ValueInt64() != state.Retention.ValueInt64() {
		resp.Diagnostics.AddError(
			"Error updating Team",
			"Team retention cannot be updated.",
		)
	}

	if !plan.TeamAdminsEmails.Equal(state.TeamAdminsEmails) {
		resp.Diagnostics.AddError(
			"Error updating Team",
			"Team admins cannot be updated.",
		)
	}
}

func (r *TeamResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddError(
		"Delete not supported",
		"Delete is not supported for this resource.",
	)
}
