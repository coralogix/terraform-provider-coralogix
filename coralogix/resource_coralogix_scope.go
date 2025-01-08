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
	"regexp"
	"strings"
	"terraform-provider-coralogix/coralogix/clientset"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var availableEntityTypes = []string{"logs", "spans", "unspecified"}

func NewScopeResource() resource.Resource {
	return &ScopeResource{}
}

type ScopeResource struct {
	client *cxsdk.ScopesClient
}

func (r *ScopeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_scope"
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
				Optional:            true,
				MarkdownDescription: "Description of the scope. Optional.",
			},
			"default_expression": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Default expression to use when no filter matches the query. Until further notice, this can is limited to `true` (everything is included) or `false` (nothing is included). Use a version tag (e.g `<v1>true` or `<v1>false`)",
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile(`^<v[\d]+>true|false+$`), "Default expression must be in the format `<vX>true` or `<vX>false where X is a version number. E.g. `<v1>true` or `<v1>false"),
				},
			},
			"team_id": schema.StringAttribute{
				MarkdownDescription: "Associated team.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"filters": schema.ListNestedAttribute{
				Required:            true,
				MarkdownDescription: "Filters applied to include data in the scope.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"expression": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Expression to run",
						},
						"entity_type": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Entity type to apply the expression on",
							Validators: []validator.String{
								stringvalidator.OneOf(availableEntityTypes...),
							},
						},
					},
				},
			},
		},
		MarkdownDescription: "Coralogix Scope.",
	}
}

type ScopeResourceModel struct {
	ID                types.String       `tfsdk:"id"`
	DisplayName       types.String       `tfsdk:"display_name"`
	TeamId            types.String       `tfsdk:"team_id"`
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
			formatRpcErrors(err, cxsdk.CreateScopeRPC, protojson.Format(createScopeReq)),
		)
		return
	}
	log.Printf("[INFO] Submitted new scope: %s", protojson.Format(createScopeResp))

	getScopeReq := &cxsdk.GetTeamScopesByIDsRequest{
		Ids: []string{createScopeResp.Scope.Id},
	}
	log.Printf("[INFO] Getting new Scope: %s", protojson.Format(getScopeReq))

	getScopeResp, err := r.client.Get(ctx, getScopeReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error reading Scope",
			formatRpcErrors(err, cxsdk.GetTeamScopesByIDsRPC, protojson.Format(getScopeReq)),
		)
		return
	}
	log.Printf("[INFO] Received Scope: %s", protojson.Format(getScopeResp))
	state := flattenScope(getScopeResp)[0]

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func EntityType(s string) string {
	return strings.ToUpper(fmt.Sprintf("ENTITY_TYPE_%v", s))
}

func extractCreateScope(plan *ScopeResourceModel) (*cxsdk.CreateScopeRequest, diag.Diagnostics) {
	var filters []*cxsdk.ScopeFilter

	for _, filter := range plan.Filters {
		entityType := cxsdk.EntityTypeValueLookup[EntityType(filter.EntityType.ValueString())]

		if entityType == 0 && filter.EntityType.ValueString() != "unspecified" {
			return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid entity type", fmt.Sprintf("Invalid entity type: %s", filter.EntityType.ValueString()))}
		}
		filters = append(filters, &cxsdk.ScopeFilter{
			Expression: filter.Expression.ValueString(),
			EntityType: cxsdk.EntityType(entityType),
		})
	}

	return &cxsdk.CreateScopeRequest{
		DisplayName:       plan.DisplayName.ValueString(),
		Description:       plan.Description.ValueStringPointer(),
		Filters:           filters,
		DefaultExpression: plan.DefaultExpression.ValueString(),
	}, nil
}

func flattenScope(resp *cxsdk.GetScopesResponse) []ScopeResourceModel {
	var scopes []ScopeResourceModel
	for _, scope := range resp.GetScopes() {
		description := types.StringNull()
		if scope.GetDescription() != "" {
			description = types.StringValue(scope.GetDescription())
		}
		scopes = append(scopes, ScopeResourceModel{
			ID:                types.StringValue(scope.GetId()),
			DisplayName:       types.StringValue(scope.GetDisplayName()),
			Description:       description,
			DefaultExpression: types.StringValue(scope.GetDefaultExpression()),
			Filters:           flattenScopeFilters(scope.GetFilters()),
		})
	}
	return scopes
}

func flattenScopeFilters(filters []*cxsdk.ScopeFilter) []ScopeFilterModel {
	var result []ScopeFilterModel
	for _, filter := range filters {

		entityTypeRaw := strings.ToLower(cxsdk.EntityTypeNameLookup[int32(filter.GetEntityType())])
		entityType, _ := strings.CutPrefix(entityTypeRaw, "entity_type_")
		result = append(result, ScopeFilterModel{
			EntityType: types.StringValue(entityType),
			Expression: types.StringValue(filter.GetExpression()),
		})
	}
	return result
}

func (r *ScopeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var plan *ScopeResourceModel
	diags := req.State.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	getScopeReq := &cxsdk.GetTeamScopesByIDsRequest{
		Ids: []string{plan.ID.ValueString()},
	}
	log.Printf("[INFO] Reading Scope: %s", protojson.Format(getScopeReq))
	getScopeResp, err := r.client.Get(ctx, getScopeReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Scope %v is in state, but no longer exists in Coralogix backend", plan.ID.ValueString()),
				fmt.Sprintf("%q will be recreated when you apply", plan.ID.ValueString()),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Scope",
				formatRpcErrors(err, cxsdk.GetTeamScopesByIDsRPC, protojson.Format(getScopeReq)),
			)
		}
		return
	}
	log.Printf("[INFO] Received Scope: %s", protojson.Format(getScopeResp))

	state := flattenScope(getScopeResp)[0]
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

	_, err := r.client.Update(ctx, updateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating Scope",
			formatRpcErrors(err, cxsdk.UpdateScopeRPC, protojson.Format(updateReq)),
		)
		return
	}

	log.Printf("[INFO] Updated scope: %s", plan.ID.ValueString())

	getScopeReq := &cxsdk.GetTeamScopesByIDsRequest{
		Ids: []string{plan.ID.ValueString()},
	}
	getScopeResp, err := r.client.Get(ctx, getScopeReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error reading Scope",
			formatRpcErrors(err, cxsdk.GetTeamScopesByIDsRPC, protojson.Format(getScopeReq)),
		)
		return
	}
	log.Printf("[INFO] Received Scope: %s", protojson.Format(getScopeResp))
	state := flattenScope(getScopeResp)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func extractUpdateScope(plan *ScopeResourceModel) (*cxsdk.UpdateScopeRequest, diag.Diagnostics) {

	var filters []*cxsdk.ScopeFilter

	for _, filter := range plan.Filters {
		entityType := cxsdk.EntityTypeValueLookup[EntityType(filter.EntityType.ValueString())]

		if entityType == 0 && filter.EntityType.ValueString() != "unspecified" {
			return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid entity type", fmt.Sprintf("Invalid entity type: %s", filter.EntityType.ValueString()))}
		}
		filters = append(filters, &cxsdk.ScopeFilter{
			Expression: filter.Expression.ValueString(),
			EntityType: cxsdk.EntityType(entityType),
		})
	}

	return &cxsdk.UpdateScopeRequest{
		Id:                plan.ID.ValueString(),
		DisplayName:       plan.DisplayName.ValueString(),
		Description:       plan.Description.ValueStringPointer(),
		Filters:           filters,
		DefaultExpression: plan.DefaultExpression.ValueString(),
	}, nil
}

func (r *ScopeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state *ScopeResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	log.Printf("[INFO] Deleting Scope: %s", state.ID.ValueString())

	deleteReq := &cxsdk.DeleteScopeRequest{Id: state.ID.ValueString()}
	log.Printf("[INFO] Deleting Scope: %s", protojson.Format(deleteReq))
	_, err := r.client.Delete(ctx, deleteReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error deleting Scope",
			formatRpcErrors(err, cxsdk.DeleteScopeRPC, protojson.Format(deleteReq)),
		)
		return
	}
	log.Printf("[INFO] Deleted scope: %s", state.ID.ValueString())
}
