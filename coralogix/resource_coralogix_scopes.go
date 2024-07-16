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

package coralogix

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"

	"terraform-provider-coralogix/coralogix/clientset"
	scopes "terraform-provider-coralogix/coralogix/clientset/grpc/scopes"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"

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
	createScopeURL = "com.coralogixapis.scopes.v1.ScopeService/CreateScopeInOrg"
	updateScopeURL = "com.coralogixapis.scopes.v1.ScopeService/UpdateScope"
	deleteScopeURL = "com.coralogixapis.scopes.v1.ScopeService/DeleteScope"
)

func NewScopeResource() resource.Resource {
	return &ScopeResource{}
}

type ScopeResource struct {
	client *clientset.ScopesClient
}

func (r *ScopeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team"

}

func (r *ScopeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.Scopes()
}

func (r *ScopeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *ScopeResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Scope ID.",
			},
			"display_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Scope display name.",
			},
			"description": schema.StringAttribute{
				Required:            false,
				MarkdownDescription: "Description of the scope. Optional.",
			},
			"team_id": schema.StringAttribute{
				MarkdownDescription: "Associated team.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"filters": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"expression": schema.StringAttribute{
							Required: true,
						},
						"entity_type": schema.StringAttribute{
							Required: true,
						},
					},
				},
				MarkdownDescription: "Filters applied to include data in the scope.",
			},
		},
		MarkdownDescription: "Coralogix Scope.",
	}
}

type ScopeResourceModel struct {
	ID                types.String       `tfsdk:"id"`
	DisplayName       types.String       `tfsdk:"name"`
	Description       types.String       `tfsdk:"description"`
	DefaultExpression types.String       `tfsdk:"default_expression"`
	Filters           []ScopeFilterModel `tfsdk:"filters"`
}

type ScopeFilterModel struct {
	EntityType types.String `tfsdk:"entity_type"`
	Expression types.String `tfsdk:"expression"`
}

func (r *ScopeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *ScopeResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createScopeReq, diags := extractCreateScope(plan)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
	log.Printf("[INFO] Creating new Scope: %s", protojson.Format(createScopeReq))
	createScopeResp, err := r.client.Create(ctx, createScopeReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error creating Scope",
			formatRpcErrors(err, createScopeURL, protojson.Format(createScopeReq)),
		)
		return
	}
	log.Printf("[INFO] Submitted new scope: %s", protojson.Format(createScopeResp))

	getScopeReq := &scopes.GetTeamScopesByIdsRequest{
		Ids: []string{strconv.Itoa(int(createScopeResp.Scope.TeamId))},
	}

	getScopeResp, err := r.client.Get(ctx, getScopeReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error reading Scope",
			formatRpcErrors(err, getScopeURL, protojson.Format(getScopeReq)),
		)
		return
	}
	log.Printf("[INFO] Received Scope: %s", protojson.Format(getScopeResp))
	state := flattenScope(getScopeResp)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func extractCreateScope(plan *ScopeResourceModel) (*scopes.CreateScopeRequest, diag.Diagnostics) {

	return &scopes.CreateScopeRequest{
		DisplayName:       plan.Name.ValueString(),
		Description:       plan.Description.ValueString(),
		Filters:           plan.Filters,
		DefaultExpression: plan.DefaultExpression.ValueString(),
	}, nil
}

func flattenScope(resp *scopes.GetScopeResponse) *ScopeResourceModel {
	return &ScopeResourceModel{
		ID:         types.StringValue(strconv.Itoa(int(resp.GetScopeId().GetId()))),
		Name:       types.StringValue(resp.GetScopeName()),
		Retention:  types.Int64Value(int64(resp.GetRetention())),
		DailyQuota: types.Float64Value(math.Round(resp.GetDailyQuota()*1000) / 1000),
	}
}

func (r *ScopeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var plan *ScopeResourceModel
	diags := req.State.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	intId, err := strconv.Atoi(plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing Scope ID",
			fmt.Sprintf("Error parsing Scope ID: %s", err.Error()),
		)
		return
	}
	getScopeReq := &scopes.GetScopeRequest{
		ScopeId: &scopes.ScopeId{
			Id: uint32(intId),
		},
	}
	log.Printf("[INFO] Reading Scope: %s", protojson.Format(getScopeReq))
	getScopeResp, err := r.client.GetScope(ctx, getScopeReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Scope %q is in state, but no longer exists in Coralogix backend", intId),
				fmt.Sprintf("%q will be recreated when you apply", intId),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Scope",
				formatRpcErrors(err, getScopeURL, protojson.Format(getScopeReq)),
			)
		}
		return
	}
	log.Printf("[INFO] Received Scope: %s", protojson.Format(getScopeResp))

	state := flattenScope(getScopeResp)
	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *ScopeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan *ScopeResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq, diags := extractUpdateScope(plan)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
	log.Printf("[INFO] Updating Scope: %s", protojson.Format(updateReq))

	_, err := r.client.UpdateScope(ctx, updateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.PermissionDenied || status.Code(err) == codes.Unauthenticated {
			resp.Diagnostics.AddError(
				"Error updating Scope",
				fmt.Sprintf("permission denied for url - %s\ncheck your org-key and permissions", updateScopeURL),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error updating Scope",
				formatRpcErrors(err, updateScopeURL, protojson.Format(updateReq)),
			)
		}

		return
	}

	log.Printf("[INFO] Updated team: %s", plan.ID.ValueString())

	getScopeReq := &scopes.GetScopeRequest{
		ScopeId: updateReq.GetScopeId(),
	}
	getScopeResp, err := r.client.GetScope(ctx, getScopeReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error reading Scope",
			formatRpcErrors(err, getScopeURL, protojson.Format(getScopeReq)),
		)
		return
	}
	log.Printf("[INFO] Received Scope: %s", protojson.Format(getScopeResp))
	state := flattenScope(getScopeResp)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func extractUpdateScope(plan *ScopeResourceModel) (*scopes.UpdateScopeRequest, diag.Diagnostics) {
	dailyQuota := new(float64)
	*dailyQuota = plan.DailyQuota.ValueFloat64()

	id, err := strconv.Atoi(plan.ID.ValueString())
	if err != nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error converting team id to int", err.Error())}
	}
	teamId := &scopes.ScopeId{Id: uint32(id)}

	teamName := new(string)
	*teamName = plan.Name.ValueString()

	return &scope.UpdateScopeRequest{}, nil
}

func (r *ScopeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state *ScopeResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	log.Printf("[INFO] Deleting Scope: %s", state.ID.ValueString())
	id, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting Scope",
			fmt.Sprintf("Error converting team id to int: %s", err.Error()),
		)
		return
	}

	deleteReq := &scopes.DeleteScopeRequest{ScopeId: &scopes.ScopeId{Id: uint32(id)}}
	log.Printf("[INFO] Deleting Scope: %s", protojson.Format(deleteReq))
	_, err = r.client.DeleteScope(ctx, deleteReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.PermissionDenied || status.Code(err) == codes.Unauthenticated {
			resp.Diagnostics.AddError(
				"Error deleting Scope",
				fmt.Sprintf("permission denied for url - %s\ncheck your org-key and permissions", deleteScopeURL),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error deleting Scope",
				formatRpcErrors(err, deleteScopeURL, protojson.Format(deleteReq)),
			)
		}
		return
	}
	log.Printf("[INFO] Deleted team: %s", state.ID.ValueString())
}
