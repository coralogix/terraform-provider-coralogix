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

package notifications

import (
	"context"
	"fmt"
	"log"
	"net/http"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	globalRouters "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/global_routers_service"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	globalrouterschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/notifications/global_router_schema"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func NewGlobalRouterResource() resource.Resource {
	return &GlobalRouterResource{}
}

type GlobalRouterResource struct {
	client *globalRouters.GlobalRoutersServiceAPIService
}

type GlobalRouterResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	Name                  types.String `tfsdk:"name"`
	Description           types.String `tfsdk:"description"`
	Rules                 types.List   `tfsdk:"rules"`    // RoutingRuleModel
	FallBack              types.List   `tfsdk:"fallback"` // RoutingTargetModel
	EntityLabels          types.Map    `tfsdk:"entity_labels"`
	MatchingRoutingLabels types.Map    `tfsdk:"matching_routing_labels"`
}

type RoutingRuleModel struct {
	EntityType    types.String `tfsdk:"entity_type"`
	Condition     types.String `tfsdk:"condition"`
	Targets       types.List   `tfsdk:"targets"` // RoutingTargetModel
	CustomDetails types.Map    `tfsdk:"custom_details"`
	Name          types.String `tfsdk:"name"`
}

type RoutingTargetModel struct {
	ConnectorId   types.String `tfsdk:"connector_id"`
	PresetId      types.String `tfsdk:"preset_id"`
	CustomDetails types.Map    `tfsdk:"custom_details"`
}

func (r *GlobalRouterResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_global_router"
}

func (r *GlobalRouterResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	_, r.client, _ = clientSet.GetNotifications()
}

func (r *GlobalRouterResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = globalrouterschema.V1()
}

func (r *GlobalRouterResource) UpgradeState(_ context.Context) map[int64]resource.StateUpgrader {
	v0 := globalrouterschema.V0()
	return map[int64]resource.StateUpgrader{
		1: {
			PriorSchema:   &v0,
			StateUpgrader: r.fetchGlobalRouterFromServer,
		},
	}
}

func (r *GlobalRouterResource) fetchGlobalRouterFromServer(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
	log.Printf("[INFO] Upgrading state from version: %v", req.State.Schema.GetVersion())

	var state *GlobalRouterResourceModel

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	rq := r.client.GlobalRoutersServiceGetGlobalRouter(ctx, id)

	log.Printf("[INFO] Reading coralogix_global_router: %s", utils.FormatJSON(rq))

	result, httpResponse, err := rq.Execute()
	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_global_router %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error reading coralogix_global_router",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
			)
		}
		return
	}
	log.Printf("[INFO] Read coralogix_global_router: %s", utils.FormatJSON(result))

	state, diags = flattenGlobalRouter(ctx, result.Router)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *GlobalRouterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *GlobalRouterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *GlobalRouterResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	router, diags := extractRouter(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	rq := globalRouters.CreateGlobalRouterRequest{
		Router: router,
	}

	log.Printf("[INFO] Creating new coralogix_global_router: %s", utils.FormatJSON(rq))
	result, httpResponse, err := r.client.GlobalRoutersServiceCreateGlobalRouter(ctx).CreateGlobalRouterRequest(rq).Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error creating coralogix_global_router",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", rq),
		)
		return
	}
	log.Printf("[INFO] Created new coralogix_global_router: %s", utils.FormatJSON(result))
	plan, diags = flattenGlobalRouter(ctx, result.Router)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *GlobalRouterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *GlobalRouterResourceModel

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	rq := r.client.GlobalRoutersServiceGetGlobalRouter(ctx, id)

	log.Printf("[INFO] Reading coralogix_global_router: %s", utils.FormatJSON(rq))

	result, httpResponse, err := rq.Execute()
	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_global_router %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error reading coralogix_global_router",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
			)
		}
		return
	}
	log.Printf("[INFO] Read coralogix_global_router: %s", utils.FormatJSON(result))

	state, diags = flattenGlobalRouter(ctx, result.Router)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r GlobalRouterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan *GlobalRouterResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := plan.ID.ValueString()

	router, diags := extractRouter(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	rq := globalRouters.ReplaceGlobalRouterRequest{
		Router: router,
	}
	log.Printf("[INFO] Replacing new coralogix_global_router: %s", utils.FormatJSON(rq))
	result, httpResponse, err := r.client.
		GlobalRoutersServiceReplaceGlobalRouter(ctx).
		ReplaceGlobalRouterRequest(rq).
		Execute()

	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_global_router %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error updating coralogix_global_router", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Update", nil))
		}
		return
	}
	log.Printf("[INFO] Replaced new coralogix_global_router: %s", utils.FormatJSON(result))
	plan, diags = flattenGlobalRouter(ctx, result.Router)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r GlobalRouterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state GlobalRouterResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()

	if _, httpResponse, err := r.client.GlobalRoutersServiceDeleteGlobalRouter(ctx, id).Execute(); err != nil {
		resp.Diagnostics.AddError("Error deleting coralogix_global_router",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Delete", id),
		)
		return
	}
	log.Printf("[INFO] GlobalRouter %s deleted", id)
}

func extractRouter(ctx context.Context, plan *GlobalRouterResourceModel) (*globalRouters.GlobalRouter, diag.Diagnostics) {
	rules, diags := extractGlobalRouterRules(ctx, plan.Rules)
	if diags.HasError() {
		return nil, diags
	}

	fallback, diags := extractRoutingTargets(ctx, plan.FallBack)
	if diags.HasError() {
		return nil, diags
	}

	entityLabels, diags := utils.TypeMapToStringMap(ctx, plan.EntityLabels)
	if diags.HasError() {
		return nil, diags
	}
	entityLabelMatchers, diags := utils.TypeMapToStringMap(ctx, plan.MatchingRoutingLabels)
	if diags.HasError() {
		return nil, diags
	}
	var routerId *string
	if !(plan.ID.IsNull() || plan.ID.IsUnknown()) {
		routerId = plan.ID.ValueStringPointer()
	}

	return &globalRouters.GlobalRouter{
		Id:                 routerId,
		Name:               plan.Name.ValueStringPointer(),
		Description:        plan.Description.ValueStringPointer(),
		Rules:              rules,
		Fallback:           fallback,
		EntityLabels:       &entityLabels,
		EntityLabelMatcher: &entityLabelMatchers,
	}, nil
}

func extractGlobalRouterRules(ctx context.Context, rules types.List) ([]globalRouters.RoutingRule, diag.Diagnostics) {
	var diags diag.Diagnostics
	var rulesObjects []types.Object
	rules.ElementsAs(ctx, &rulesObjects, true)
	extractedRules := make([]globalRouters.RoutingRule, 0, len(rulesObjects))
	for _, rule := range rulesObjects {
		var ruleModel RoutingRuleModel
		if dg := rule.As(ctx, &ruleModel, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		extractedRule, dg := extractRoutingRule(ctx, ruleModel)
		if diags.HasError() {
			diags.Append(dg...)
			continue
		}
		extractedRules = append(extractedRules, *extractedRule)
	}

	if diags.HasError() {
		return nil, diags
	}

	return extractedRules, diags
}

func extractRoutingRule(ctx context.Context, routingModel RoutingRuleModel) (*globalRouters.RoutingRule, diag.Diagnostics) {
	var diags diag.Diagnostics
	targets, dg := extractRoutingTargets(ctx, routingModel.Targets)
	if dg.HasError() {
		diags.Append(dg...)
		return nil, diags
	}

	customDetails, dg := utils.TypeMapToStringMap(ctx, routingModel.CustomDetails)
	if dg.HasError() {
		diags.Append(dg...)
		return nil, diags
	}

	entityType := globalrouterschema.GlobalRouterEntityTypeSchemaToApi[routingModel.EntityType.ValueString()]

	return &globalRouters.RoutingRule{
		Name:          utils.TypeStringToStringPointer(routingModel.Name),
		Condition:     routingModel.Condition.ValueStringPointer(),
		Targets:       targets,
		CustomDetails: &customDetails,
		EntityType:    &entityType,
	}, nil
}

func extractRoutingTargets(ctx context.Context, targets types.List) ([]globalRouters.RoutingTarget, diag.Diagnostics) {
	if targets.IsNull() || targets.IsUnknown() {
		return nil, nil
	}
	var diags diag.Diagnostics
	var targetsObjects []types.Object
	targets.ElementsAs(ctx, &targetsObjects, true)
	extractedTargets := make([]globalRouters.RoutingTarget, 0, len(targetsObjects))
	for _, target := range targetsObjects {
		var targetModel RoutingTargetModel
		if dg := target.As(ctx, &targetModel, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		extractedTarget, dgs := extractRoutingTarget(ctx, targetModel)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		extractedTargets = append(extractedTargets, *extractedTarget)
	}

	if diags.HasError() {
		return nil, diags
	}

	return extractedTargets, diags

}

func extractRoutingTarget(ctx context.Context, routingTargetModel RoutingTargetModel) (*globalRouters.RoutingTarget, diag.Diagnostics) {
	customDetails, diags := utils.TypeMapToStringMap(ctx, routingTargetModel.CustomDetails)
	if diags.HasError() {
		return nil, diags
	}

	return &globalRouters.RoutingTarget{
		ConnectorId:   routingTargetModel.ConnectorId.ValueStringPointer(),
		PresetId:      utils.TypeStringToStringPointer(routingTargetModel.PresetId),
		CustomDetails: &customDetails,
	}, nil
}

func flattenGlobalRouter(ctx context.Context, globalRouter *globalRouters.GlobalRouter) (*GlobalRouterResourceModel, diag.Diagnostics) {
	rules, diags := flattenGlobalRouterRules(ctx, globalRouter.GetRules())
	if diags.HasError() {
		return nil, diags
	}

	matchingRoutingLabels, diags := utils.StringMapToTypeMap(ctx, globalRouter.EntityLabelMatcher)
	if diags.HasError() {
		return nil, diags
	}

	entityLabels, diags := utils.StringMapToTypeMap(ctx, globalRouter.EntityLabels)
	if diags.HasError() {
		return nil, diags
	}

	fallback, diags := flattenFallback(ctx, globalRouter.Fallback)
	if diags.HasError() {
		return nil, diags
	}
	return &GlobalRouterResourceModel{
		ID:                    types.StringValue(globalRouter.GetId()),
		Name:                  types.StringValue(globalRouter.GetName()),
		Description:           types.StringValue(globalRouter.GetDescription()),
		Rules:                 rules,
		FallBack:              fallback,
		EntityLabels:          entityLabels,
		MatchingRoutingLabels: matchingRoutingLabels,
	}, nil
}

func flattenGlobalRouterRules(ctx context.Context, rules []globalRouters.RoutingRule) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	rulesList := make([]types.Object, 0, len(rules))
	for _, rule := range rules {
		ruleModel, dgs := flattenRoutingRule(ctx, &rule)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		rulesList = append(rulesList, ruleModel)
	}

	if diags.HasError() {
		return types.List{}, diags
	}

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: globalrouterschema.RoutingRuleAttr()}, rulesList)
}

func flattenFallback(ctx context.Context, targets []globalRouters.RoutingTarget) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	if targets == nil {
		return types.ListNull(types.ObjectType{AttrTypes: globalrouterschema.RoutingTargetAttr()}), diags
	}
	fallbackTargetList := make([]types.Object, 0, len(targets))
	for _, target := range targets {
		targetModel, dgs := flattenRoutingTarget(ctx, &target)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		fallbackTargetList = append(fallbackTargetList, targetModel)
	}

	if diags.HasError() {
		return types.List{}, diags
	}

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: globalrouterschema.RoutingTargetAttr()}, fallbackTargetList)
}

func flattenRoutingRule(ctx context.Context, rule *globalRouters.RoutingRule) (types.Object, diag.Diagnostics) {
	targets, diags := flattenRoutingTargets(ctx, rule.GetTargets())
	if diags.HasError() {
		return types.ObjectNull(globalrouterschema.RoutingRuleAttr()), diags
	}

	customDetails, diags := flattenCustomDetails(ctx, rule.GetCustomDetails())
	if diags.HasError() {
		return types.ObjectNull(globalrouterschema.RoutingRuleAttr()), diags
	}
	entityType := globalrouterschema.GlobalRouterNotificationCenterEntityTypeApiToSchema[*rule.EntityType]
	ruleModel := RoutingRuleModel{
		Condition:     types.StringValue(rule.GetCondition()),
		Name:          types.StringValue(rule.GetName()),
		Targets:       targets,
		CustomDetails: customDetails,
		EntityType:    types.StringValue(entityType),
	}

	return types.ObjectValueFrom(ctx, globalrouterschema.RoutingRuleAttr(), ruleModel)
}

func flattenCustomDetails(ctx context.Context, details map[string]string) (types.Map, diag.Diagnostics) {
	var diags diag.Diagnostics
	if details == nil {
		return types.MapNull(types.StringType), diags
	}

	detailsMap := make(map[string]types.String)
	for k, v := range details {
		detailsMap[k] = types.StringValue(v)
	}

	return types.MapValueFrom(ctx, types.StringType, detailsMap)
}

func flattenRoutingTargets(ctx context.Context, targets []globalRouters.RoutingTarget) (types.List, diag.Diagnostics) {
	if targets == nil {
		return types.ListNull(types.ObjectType{AttrTypes: globalrouterschema.RoutingTargetAttr()}), nil
	}

	var diags diag.Diagnostics
	targetsList := make([]types.Object, 0, len(targets))
	for _, target := range targets {
		targetModel, dgs := flattenRoutingTarget(ctx, &target)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		targetsList = append(targetsList, targetModel)
	}

	if diags.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: globalrouterschema.RoutingTargetAttr()}), diags
	}

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: globalrouterschema.RoutingTargetAttr()}, targetsList)
}

func flattenRoutingTarget(ctx context.Context, target *globalRouters.RoutingTarget) (types.Object, diag.Diagnostics) {
	customDetails, diags := flattenCustomDetails(ctx, target.GetCustomDetails())
	if diags.HasError() {
		return types.ObjectNull(globalrouterschema.RoutingTargetAttr()), diags
	}

	targetModel := RoutingTargetModel{
		ConnectorId:   types.StringPointerValue(target.ConnectorId),
		PresetId:      utils.StringPointerToTypeString(target.PresetId),
		CustomDetails: customDetails,
	}

	return types.ObjectValueFrom(ctx, globalrouterschema.RoutingTargetAttr(), targetModel)
}
