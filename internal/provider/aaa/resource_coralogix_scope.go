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
	"net/http"
	"regexp"
	"strconv"
	"strings"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"

	scopess "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/scopes_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

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

var availableEntityTypes = []string{"logs", "spans", utils.UNSPECIFIED}

func NewScopeResource() resource.Resource {
	return &ScopeResource{}
}

type ScopeResource struct {
	client *scopess.ScopesServiceAPIService
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
				MarkdownDescription: "Default expression to use when no filter matches the query. Until further notice, this is limited to `true` (everything is included) or `false` (nothing is included). Use a version tag (e.g `<v1>true` or `<v1>false`)",
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile(`^<v[\d]+>true|false+$`), "Default expression must be in the format `<vX>true` or `<vX>false where X is a version number. E.g. `<v1>true` or `<v1>false"),
				},
			},
			"team_id": schema.StringAttribute{
				MarkdownDescription: "Associated team.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
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

	rq, diags := extractCreateScope(plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	log.Printf("[INFO] Creating new coralogix_scope: %s", utils.FormatJSON(rq))
	result, httpResponse, err := r.client.
		ScopesServiceCreateScope(ctx).
		CreateScopeRequest(*rq).
		Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error creating coralogix_scope",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", rq),
		)
		return
	}
	log.Printf("[INFO] Created new coralogix_scope: %s", utils.FormatJSON(result))
	state := flattenScope(result.Scope)

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func EntityType(s string) string {
	return strings.ToUpper(fmt.Sprintf("ENTITY_TYPE_%v", s))
}

func extractCreateScope(plan *ScopeResourceModel) (*scopess.CreateScopeRequest, diag.Diagnostics) {
	var filters []scopess.ScopesV1Filter

	for _, filter := range plan.Filters {
		et, err := scopess.NewV1EntityTypeFromValue(EntityType(filter.EntityType.ValueString()))
		if err != nil {
			return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid entity type", fmt.Sprintf("Invalid entity type: %s", err))}
		}

		filters = append(filters, scopess.ScopesV1Filter{
			Expression: filter.Expression.ValueStringPointer(),
			EntityType: et,
		})
	}

	return &scopess.CreateScopeRequest{
		DisplayName:       plan.DisplayName.ValueString(),
		Description:       plan.Description.ValueStringPointer(),
		Filters:           filters,
		DefaultExpression: plan.DefaultExpression.ValueStringPointer(),
	}, nil
}

func flattenScope(scope scopess.ScopesV1Scope) ScopeResourceModel {
	description := types.StringNull()
	if scope.GetDescription() != "" {
		description = types.StringValue(scope.GetDescription())
	}
	return ScopeResourceModel{
		ID:                types.StringValue(scope.GetId()),
		DisplayName:       types.StringValue(scope.GetDisplayName()),
		Description:       description,
		DefaultExpression: types.StringValue(scope.GetDefaultExpression()),
		TeamId:            types.StringValue(strconv.Itoa(int(scope.GetTeamId()))),
		Filters:           flattenScopeFilters(scope.GetFilters()),
	}
}

func flattenScopeFilters(filters []scopess.ScopesV1Filter) []ScopeFilterModel {
	var result []ScopeFilterModel
	for _, filter := range filters {
		entityTypeRaw := strings.ToLower(string(filter.GetEntityType()))
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
	id := plan.ID.ValueString()

	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	rq := r.client.
		ScopesServiceGetTeamScopesByIds(ctx).Ids([]string{id})

	log.Printf("[INFO] Reading coralogix_scope: %s", utils.FormatJSON(rq))
	result, httpResponse, err := rq.
		Execute()

	if err != nil && len(result.Scopes) == 0 {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_scope %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error reading coralogix_scope", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil))
		}
		return
	}
	log.Printf("[INFO] Replaced new coralogix_scope: %s", utils.FormatJSON(result))
	state := flattenScope(result.Scopes[0])

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
	id := plan.ID.ValueString()

	rq, diags := extractUpdateScope(plan)
	if diags.HasError() {
		return
	}
	log.Printf("[INFO] Replacing new coralogix_scope: %s", utils.FormatJSON(rq))
	result, httpResponse, err := r.client.
		ScopesServiceUpdateScope(ctx).
		UpdateScopeRequest(*rq).
		Execute()

	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_scope %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error updating coralogix_scope", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Update", nil))
		}
		return
	}
	log.Printf("[INFO] Replaced new coralogix_scope: %s", utils.FormatJSON(result))

	state := flattenScope(result.Scope)

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func extractUpdateScope(plan *ScopeResourceModel) (*scopess.UpdateScopeRequest, diag.Diagnostics) {

	var filters []scopess.ScopesV1Filter

	for _, filter := range plan.Filters {
		et, err := scopess.NewV1EntityTypeFromValue(EntityType(filter.EntityType.ValueString()))
		if err != nil {
			return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid entity type", fmt.Sprintf("Invalid entity type: %s", err))}
		}

		filters = append(filters, scopess.ScopesV1Filter{
			Expression: filter.Expression.ValueStringPointer(),
			EntityType: et,
		})
	}

	return &scopess.UpdateScopeRequest{
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

	id := state.ID.ValueString()

	log.Printf("[INFO] Deleting coralogix_scope")

	result, httpResponse, err := r.client.
		ScopesServiceDeleteScope(ctx, id).
		Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error deleting coralogix_scope",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Delete", nil),
		)
		return
	}
	log.Printf("[INFO] Deleted coralogix_scope: %s", utils.FormatJSON(result))
}
