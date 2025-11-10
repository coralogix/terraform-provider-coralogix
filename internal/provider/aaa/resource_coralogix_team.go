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
	"math"
	"strconv"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64planmodifier"
	"google.golang.org/grpc/codes"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NewTeamResource() resource.Resource {
	return &TeamResource{}
}

type TeamResource struct {
	client *cxsdk.TeamsClient
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
				Computed:            true,
				MarkdownDescription: "Team retention.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
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
	createTeamResp, err := r.client.Create(ctx, createTeamReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error creating Team",
			utils.FormatRpcErrors(err, cxsdk.CreateTeamInOrgRPC, protojson.Format(createTeamReq)),
		)

		return
	}
	log.Printf("[INFO] Submitted new team: %s", protojson.Format(createTeamResp.GetTeamId()))

	getTeamReq := &cxsdk.GetTeamRequest{
		TeamId: createTeamResp.GetTeamId(),
	}
	getTeamResp, err := r.client.Get(ctx, getTeamReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error reading Team",
			utils.FormatRpcErrors(err, cxsdk.GetTeamRPC, protojson.Format(getTeamReq)),
		)
		return
	}
	log.Printf("[INFO] Received Team: %s", protojson.Format(getTeamResp))
	state := TeamResourceModel{
		ID:         types.StringValue(strconv.Itoa(int(getTeamResp.GetTeamId().GetId()))),
		Name:       types.StringValue(getTeamResp.GetTeamName()),
		Retention:  types.Int64Value(int64(getTeamResp.GetRetention())),
		DailyQuota: types.Float64Value(math.Round(getTeamResp.GetDailyQuota()*1000) / 1000),
	}

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func extractCreateTeam(plan *TeamResourceModel) (*cxsdk.CreateTeamInOrgRequest, diag.Diagnostics) {
	var dailyQuota *float64
	if !(plan.DailyQuota.IsUnknown() || plan.DailyQuota.IsNull()) {
		dailyQuota = new(float64)
		*dailyQuota = plan.DailyQuota.ValueFloat64()
	}

	return &cxsdk.CreateTeamInOrgRequest{
		TeamName:   plan.Name.ValueString(),
		DailyQuota: dailyQuota,
	}, nil
}

func (r *TeamResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var plan *TeamResourceModel
	diags := req.State.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	intId, err := strconv.Atoi(plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing Team ID",
			fmt.Sprintf("Error parsing Team ID: %s", err.Error()),
		)
		return
	}
	getTeamReq := &cxsdk.GetTeamRequest{
		TeamId: &cxsdk.TeamID{
			Id: uint32(intId),
		},
	}
	log.Printf("[INFO] Reading Team: %s", protojson.Format(getTeamReq))
	getTeamResp, err := r.client.Get(ctx, getTeamReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Team %q is in state, but no longer exists in Coralogix backend", intId),
				fmt.Sprintf("%q will be recreated when you apply", intId),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Team",
				utils.FormatRpcErrors(err, cxsdk.GetTeamRPC, protojson.Format(getTeamReq)),
			)
		}
		return
	}
	log.Printf("[INFO] Received Team: %s", protojson.Format(getTeamResp))

	state := TeamResourceModel{
		ID:         types.StringValue(strconv.Itoa(int(getTeamResp.GetTeamId().GetId()))),
		Name:       types.StringValue(getTeamResp.GetTeamName()),
		Retention:  types.Int64Value(int64(getTeamResp.GetRetention())),
		DailyQuota: types.Float64Value(math.Round(getTeamResp.GetDailyQuota()*1000) / 1000),
	}

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *TeamResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan *TeamResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq, diags := extractUpdateTeam(plan)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
	log.Printf("[INFO] Updating Team: %s", protojson.Format(updateReq))

	_, err := r.client.Update(ctx, updateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating Team",
			utils.FormatRpcErrors(err, cxsdk.UpdateTeamRPC, protojson.Format(updateReq)),
		)

		return
	}

	log.Printf("[INFO] Updated team: %s", plan.ID.ValueString())

	getTeamReq := &cxsdk.GetTeamRequest{
		TeamId: updateReq.GetTeamId(),
	}
	getTeamResp, err := r.client.Get(ctx, getTeamReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error reading Team",
			utils.FormatRpcErrors(err, cxsdk.GetTeamRPC, protojson.Format(getTeamReq)),
		)
		return
	}
	log.Printf("[INFO] Received Team: %s", protojson.Format(getTeamResp))
	state := TeamResourceModel{
		ID:         types.StringValue(strconv.Itoa(int(getTeamResp.GetTeamId().GetId()))),
		Name:       types.StringValue(getTeamResp.GetTeamName()),
		Retention:  types.Int64Value(int64(getTeamResp.GetRetention())),
		DailyQuota: types.Float64Value(math.Round(getTeamResp.GetDailyQuota()*1000) / 1000),
	}

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func extractUpdateTeam(plan *TeamResourceModel) (*cxsdk.UpdateTeamRequest, diag.Diagnostics) {
	dailyQuota := new(float64)
	*dailyQuota = plan.DailyQuota.ValueFloat64()

	id, err := strconv.Atoi(plan.ID.ValueString())
	if err != nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error converting team id to int", err.Error())}
	}
	teamId := &cxsdk.TeamID{Id: uint32(id)}

	teamName := new(string)
	*teamName = plan.Name.ValueString()

	return &cxsdk.UpdateTeamRequest{
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

	deleteReq := &cxsdk.DeleteTeamRequest{TeamId: &cxsdk.TeamID{Id: uint32(id)}}
	log.Printf("[INFO] Deleting Team: %s", protojson.Format(deleteReq))
	_, err = r.client.Delete(ctx, deleteReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error deleting Team",
			utils.FormatRpcErrors(err, cxsdk.DeleteTeamRPC, protojson.Format(deleteReq)),
		)
		return
	}
	log.Printf("[INFO] Deleted team: %s", state.ID.ValueString())
}
