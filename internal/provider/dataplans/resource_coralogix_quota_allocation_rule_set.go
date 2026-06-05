// Copyright 2026 Coralogix Ltd.
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

package dataplans

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	quotaRules "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/quota_allocation_rule_set_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const quotaAllocationRuleSetImportID = "quota-allocation-rule-set"

const (
	quotaAllocationTypeUnspecified = "unspecified"
	quotaAllocationTypePercentage  = "percentage"
	quotaAllocationTypeLockedUnits = "locked_units"
)

var (
	quotaAllocationTypeSchemaToAPI = map[string]quotaRules.QuotaAllocationType{
		quotaAllocationTypeUnspecified: quotaRules.QUOTAALLOCATIONTYPE_QUOTA_ALLOCATION_TYPE_UNSPECIFIED,
		quotaAllocationTypePercentage:  quotaRules.QUOTAALLOCATIONTYPE_QUOTA_ALLOCATION_TYPE_PERCENTAGE,
		quotaAllocationTypeLockedUnits: quotaRules.QUOTAALLOCATIONTYPE_QUOTA_ALLOCATION_TYPE_LOCKED_UNITS,
	}
	quotaAllocationTypeAPIToSchema = map[quotaRules.QuotaAllocationType]string{
		quotaRules.QUOTAALLOCATIONTYPE_QUOTA_ALLOCATION_TYPE_UNSPECIFIED:  quotaAllocationTypeUnspecified,
		quotaRules.QUOTAALLOCATIONTYPE_QUOTA_ALLOCATION_TYPE_PERCENTAGE:   quotaAllocationTypePercentage,
		quotaRules.QUOTAALLOCATIONTYPE_QUOTA_ALLOCATION_TYPE_LOCKED_UNITS: quotaAllocationTypeLockedUnits,
	}
	validQuotaAllocationTypes = []string{
		quotaAllocationTypeUnspecified,
		quotaAllocationTypePercentage,
		quotaAllocationTypeLockedUnits,
	}
)

var (
	_ resource.ResourceWithConfigure      = &QuotaAllocationRuleSetResource{}
	_ resource.ResourceWithImportState    = &QuotaAllocationRuleSetResource{}
	_ resource.ResourceWithValidateConfig = &QuotaAllocationRuleSetResource{}
)

func NewQuotaAllocationRuleSetResource() resource.Resource {
	return &QuotaAllocationRuleSetResource{}
}

type QuotaAllocationRuleSetResource struct {
	client *quotaRules.QuotaAllocationRuleSetServiceAPIService
}

type QuotaAllocationRuleSetModel struct {
	ID    types.String               `tfsdk:"id"`
	Rules []QuotaAllocationRuleModel `tfsdk:"rules"`
}

type QuotaAllocationRuleModel struct {
	EntityType     types.String  `tfsdk:"entity_type"`
	Allocation     types.Float64 `tfsdk:"allocation"`
	AllocationType types.String  `tfsdk:"allocation_type"`
	Enabled        types.Bool    `tfsdk:"enabled"`
	CanOverflow    types.Bool    `tfsdk:"can_overflow"`
}

func (r *QuotaAllocationRuleSetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_quota_allocation_rule_set"
}

func (r *QuotaAllocationRuleSetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.QuotaAllocationRules()
}

func (r *QuotaAllocationRuleSetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the account-level Coralogix quota allocation rule set. This API is a singleton overwrite surface: updates replace the full rule set, and delete removes the account rule set. Requires `team-quota-rules:Read` and `team-quota-rules:Manage` permissions. Known entity types include `logs`, `browserLogs`, `spans`, `metrics`, `sessionRecordings`, `cpuProfiles`, and `olly`, but the API may accept additional values.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The backend identifier for the quota allocation rule set. Import accepts this value or `quota-allocation-rule-set`.",
			},
			"rules": schema.SetNestedAttribute{
				Required: true,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
				MarkdownDescription: "Complete set of quota allocation rules. Because the backend stores a single account-level rule set, Terraform replaces the full set during update.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"entity_type": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.LengthAtLeast(1),
							},
							MarkdownDescription: "Entity type covered by the rule. Known values include `logs`, `browserLogs`, `spans`, `metrics`, `sessionRecordings`, `cpuProfiles`, and `olly`.",
						},
						"allocation": schema.Float64Attribute{
							Required: true,
							Validators: []validator.Float64{
								float64validator.AtLeast(0),
							},
							MarkdownDescription: "Quota allocation value for this entity type. For `percentage`, must be between 0 and 100. For `locked_units`, must be non-negative.",
						},
						"allocation_type": schema.StringAttribute{
							Optional: true,
							Computed: true,
							Default:  stringdefault.StaticString(quotaAllocationTypePercentage),
							Validators: []validator.String{
								stringvalidator.OneOf(validQuotaAllocationTypes...),
							},
							MarkdownDescription: "How the allocation value is interpreted. Valid values are `percentage`, `locked_units`, and `unspecified`.",
						},
						"enabled": schema.BoolAttribute{
							Required:            true,
							MarkdownDescription: "Whether the quota allocation rule is enabled.",
						},
						"can_overflow": schema.BoolAttribute{
							Required:            true,
							MarkdownDescription: "Whether this entity type can overflow beyond its allocation.",
						},
					},
				},
			},
		},
	}
}

func (r *QuotaAllocationRuleSetResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data QuotaAllocationRuleSetModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(validateQuotaAllocationRules(data.Rules)...)
}

func (r *QuotaAllocationRuleSetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *QuotaAllocationRuleSetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan QuotaAllocationRuleSetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ruleSet, diags := expandQuotaAllocationRuleSet(plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	request := quotaRules.CreateQuotaAllocationRuleSetRequest{RuleSet: *ruleSet}
	result, httpResponse, err := r.client.
		QuotaAllocationRuleSetServiceCreateQuotaAllocationRuleSet(ctx).
		CreateQuotaAllocationRuleSetRequest(request).
		Execute()
	if err != nil {
		if responseStatus(httpResponse) == http.StatusConflict {
			existingResult, readResponse, readErr := getQuotaAllocationRuleSet(ctx, r.client, "")
			if readErr != nil {
				resp.Diagnostics.AddError("Error reading existing coralogix_quota_allocation_rule_set after create conflict",
					utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(readResponse, readErr), "Read", nil),
				)
				return
			}
			if !quotaAllocationRuleSetIsEmpty(existingResult) && !quotaAllocationRuleSetHasUserManagedRules(existingResult.RuleSet) {
				ruleSet = mergeManagedQuotaAllocationRules(ruleSet, existingResult.RuleSet)
				request := quotaRules.ReplaceQuotaAllocationRuleSetRequest{RuleSet: *ruleSet}
				replaceResult, replaceResponse, replaceErr := r.client.
					QuotaAllocationRuleSetServiceReplaceQuotaAllocationRuleSet(ctx).
					ReplaceQuotaAllocationRuleSetRequest(request).
					Execute()
				if replaceErr != nil {
					resp.Diagnostics.AddError("Error preserving Coralogix-managed quota allocation rules during create",
						utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(replaceResponse, replaceErr), "Replace", request),
					)
					return
				}

				state, diags := flattenReplaceQuotaAllocationRuleSetResponse(replaceResult)
				if diags.HasError() {
					resp.Diagnostics.Append(diags...)
					return
				}

				resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
				return
			}

			resp.Diagnostics.AddError(
				"Quota allocation rule set already exists",
				"Coralogix already has a quota allocation rule set. Import it with `terraform import coralogix_quota_allocation_rule_set.<name> quota-allocation-rule-set` before managing it with Terraform.",
			)
			return
		}

		resp.Diagnostics.AddError("Error creating coralogix_quota_allocation_rule_set",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", request),
		)
		return
	}

	state, diags := flattenCreateQuotaAllocationRuleSetResponse(result)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *QuotaAllocationRuleSetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state QuotaAllocationRuleSetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, httpResponse, err := getQuotaAllocationRuleSet(ctx, r.client, state.ID.ValueString())
	if err != nil {
		if responseStatus(httpResponse) == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Error reading coralogix_quota_allocation_rule_set",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
		)
		return
	}
	if quotaAllocationRuleSetIsEmpty(result) {
		resp.State.RemoveResource(ctx)
		return
	}

	newState, diags := flattenGetQuotaAllocationRuleSetResponse(result)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	if len(newState.Rules) == 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

func (r *QuotaAllocationRuleSetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan QuotaAllocationRuleSetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ruleSet, diags := expandQuotaAllocationRuleSet(plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	result, httpResponse, err := getQuotaAllocationRuleSet(ctx, r.client, plan.ID.ValueString())
	if err != nil && responseStatus(httpResponse) != http.StatusNotFound {
		resp.Diagnostics.AddError("Error reading coralogix_quota_allocation_rule_set before replace",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
		)
		return
	}
	if err == nil && result != nil && result.RuleSet != nil {
		ruleSet = mergeManagedQuotaAllocationRules(ruleSet, result.RuleSet)
	}

	request := quotaRules.ReplaceQuotaAllocationRuleSetRequest{RuleSet: *ruleSet}
	replaceResult, httpResponse, err := r.client.
		QuotaAllocationRuleSetServiceReplaceQuotaAllocationRuleSet(ctx).
		ReplaceQuotaAllocationRuleSetRequest(request).
		Execute()
	if err != nil {
		if responseStatus(httpResponse) == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Error replacing coralogix_quota_allocation_rule_set",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Replace", request),
		)
		return
	}

	state, diags := flattenReplaceQuotaAllocationRuleSetResponse(replaceResult)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *QuotaAllocationRuleSetResource) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	result, httpResponse, err := getQuotaAllocationRuleSet(ctx, r.client, "")
	if err != nil {
		if responseStatus(httpResponse) == http.StatusNotFound {
			return
		}

		resp.Diagnostics.AddError("Error reading coralogix_quota_allocation_rule_set before delete",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
		)
		return
	}
	if quotaAllocationRuleSetIsEmpty(result) {
		return
	}

	managedRuleSet := managedQuotaAllocationRuleSet(result.RuleSet)
	if len(managedRuleSet.GetRules()) > 0 {
		request := quotaRules.ReplaceQuotaAllocationRuleSetRequest{RuleSet: *managedRuleSet}
		_, httpResponse, err = r.client.
			QuotaAllocationRuleSetServiceReplaceQuotaAllocationRuleSet(ctx).
			ReplaceQuotaAllocationRuleSetRequest(request).
			Execute()
		if err != nil {
			if responseStatus(httpResponse) == http.StatusNotFound {
				return
			}

			resp.Diagnostics.AddError("Error preserving Coralogix-managed quota allocation rules during delete",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Replace", request),
			)
			return
		}
		return
	}

	_, httpResponse, err = r.client.
		QuotaAllocationRuleSetServiceDeleteQuotaAllocationRuleSet(ctx).
		Execute()
	if err != nil {
		if responseStatus(httpResponse) == http.StatusNotFound {
			return
		}

		result, readResponse, readErr := getQuotaAllocationRuleSet(ctx, r.client, "")
		if readErr == nil && quotaAllocationRuleSetIsEmpty(result) {
			return
		}
		if responseStatus(readResponse) == http.StatusNotFound {
			return
		}

		resp.Diagnostics.AddError("Error deleting coralogix_quota_allocation_rule_set",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Delete", nil),
		)
		return
	}
}

func getQuotaAllocationRuleSet(ctx context.Context, client *quotaRules.QuotaAllocationRuleSetServiceAPIService, id string) (*quotaRules.GetQuotaAllocationRuleSetResponse, *http.Response, error) {
	request := client.QuotaAllocationRuleSetServiceGetQuotaAllocationRuleSet(ctx)
	if id != "" && id != quotaAllocationRuleSetImportID {
		request = request.Id(id)
	}

	return request.Execute()
}

func validateQuotaAllocationRules(rules []QuotaAllocationRuleModel) diag.Diagnostics {
	diags := diag.Diagnostics{}
	seen := map[string]struct{}{}
	for _, rule := range rules {
		if rule.EntityType.IsUnknown() {
			continue
		}

		entityType := rule.EntityType.ValueString()
		if entityType == "" {
			continue
		}

		if _, ok := seen[entityType]; ok {
			diags.AddAttributeError(
				path.Root("rules"),
				"Duplicate quota allocation rule entity_type",
				fmt.Sprintf("Only one quota allocation rule can be configured for entity_type %q.", entityType),
			)
			continue
		}
		seen[entityType] = struct{}{}

		if rule.Allocation.IsUnknown() || rule.Allocation.IsNull() {
			continue
		}
		allocationType := quotaAllocationTypePercentage
		allocationTypeKnown := true
		if !rule.AllocationType.IsUnknown() && !rule.AllocationType.IsNull() {
			allocationType = rule.AllocationType.ValueString()
		} else if rule.AllocationType.IsUnknown() {
			allocationTypeKnown = false
		}
		allocation := rule.Allocation.ValueFloat64()
		if allocationTypeKnown && allocationType == quotaAllocationTypePercentage && allocation > 100 {
			diags.AddAttributeError(
				path.Root("rules"),
				"Invalid percentage quota allocation",
				fmt.Sprintf("The quota allocation rule for entity_type %q uses allocation_type %q, so allocation must be between 0 and 100.", entityType, quotaAllocationTypePercentage),
			)
		}
		normalizedAllocation := normalizeQuotaAllocation(allocation)
		if normalizedAllocation != allocation {
			diags.AddAttributeError(
				path.Root("rules"),
				"Invalid quota allocation precision",
				fmt.Sprintf("The quota allocation rule for entity_type %q uses allocation %s, but the Coralogix API stores allocations as float32 and would return %s. Configure %s instead to avoid Terraform state drift.", entityType, formatQuotaAllocation(allocation), formatQuotaAllocation(normalizedAllocation), formatQuotaAllocation(normalizedAllocation)),
			)
		}
	}

	return diags
}

func expandQuotaAllocationRuleSet(plan QuotaAllocationRuleSetModel) (*quotaRules.QuotaAllocationEntityTypeRuleSet, diag.Diagnostics) {
	diags := validateQuotaAllocationRules(plan.Rules)
	if diags.HasError() {
		return nil, diags
	}

	rules := make([]quotaRules.QuotaAllocationEntityTypeRule, 0, len(plan.Rules))
	for _, rule := range plan.Rules {
		sdkRule := quotaRules.QuotaAllocationEntityTypeRule{
			EntityType:  rule.EntityType.ValueString(),
			Allocation:  float32(rule.Allocation.ValueFloat64()),
			Enabled:     rule.Enabled.ValueBool(),
			CanOverflow: rule.CanOverflow.ValueBool(),
		}
		if !rule.AllocationType.IsNull() && !rule.AllocationType.IsUnknown() {
			if allocationType, ok := quotaAllocationTypeSchemaToAPI[rule.AllocationType.ValueString()]; ok {
				sdkRule.SetAllocationType(allocationType)
			}
		}
		rules = append(rules, sdkRule)
	}

	sort.Slice(rules, func(i, j int) bool {
		return rules[i].EntityType < rules[j].EntityType
	})

	ruleSet := quotaRules.QuotaAllocationEntityTypeRuleSet{Rules: rules}
	if !plan.ID.IsNull() && !plan.ID.IsUnknown() && plan.ID.ValueString() != quotaAllocationRuleSetImportID {
		ruleSet.SetId(plan.ID.ValueString())
	}

	return &ruleSet, diags
}

func flattenCreateQuotaAllocationRuleSetResponse(resp *quotaRules.CreateQuotaAllocationRuleSetResponse) (*QuotaAllocationRuleSetModel, diag.Diagnostics) {
	if resp == nil || resp.RuleSet == nil {
		return &QuotaAllocationRuleSetModel{
			ID:    types.StringValue(quotaAllocationRuleSetImportID),
			Rules: []QuotaAllocationRuleModel{},
		}, nil
	}

	return flattenQuotaAllocationRuleSet(resp.RuleSet)
}

func flattenGetQuotaAllocationRuleSetResponse(resp *quotaRules.GetQuotaAllocationRuleSetResponse) (*QuotaAllocationRuleSetModel, diag.Diagnostics) {
	if resp == nil || resp.RuleSet == nil {
		return &QuotaAllocationRuleSetModel{
			ID:    types.StringValue(quotaAllocationRuleSetImportID),
			Rules: []QuotaAllocationRuleModel{},
		}, nil
	}

	return flattenQuotaAllocationRuleSet(resp.RuleSet)
}

func flattenReplaceQuotaAllocationRuleSetResponse(resp *quotaRules.ReplaceQuotaAllocationRuleSetResponse) (*QuotaAllocationRuleSetModel, diag.Diagnostics) {
	if resp == nil || resp.RuleSet == nil {
		return &QuotaAllocationRuleSetModel{
			ID:    types.StringValue(quotaAllocationRuleSetImportID),
			Rules: []QuotaAllocationRuleModel{},
		}, nil
	}

	return flattenQuotaAllocationRuleSet(resp.RuleSet)
}

func quotaAllocationRuleSetIsEmpty(resp *quotaRules.GetQuotaAllocationRuleSetResponse) bool {
	return resp == nil || resp.RuleSet == nil || len(resp.RuleSet.GetRules()) == 0
}

func quotaAllocationRuleSetHasUserManagedRules(ruleSet *quotaRules.QuotaAllocationEntityTypeRuleSet) bool {
	if ruleSet == nil {
		return false
	}
	for _, rule := range ruleSet.GetRules() {
		if !quotaAllocationRuleIsManaged(rule) {
			return true
		}
	}
	return false
}

func mergeManagedQuotaAllocationRules(ruleSet, remoteRuleSet *quotaRules.QuotaAllocationEntityTypeRuleSet) *quotaRules.QuotaAllocationEntityTypeRuleSet {
	if ruleSet == nil {
		ruleSet = &quotaRules.QuotaAllocationEntityTypeRuleSet{}
	}
	if remoteRuleSet == nil {
		return ruleSet
	}
	if !ruleSet.HasId() {
		if id := remoteRuleSet.GetId(); id != "" {
			ruleSet.SetId(id)
		}
	}

	managedRuleSet := managedQuotaAllocationRuleSet(remoteRuleSet)
	rules := append([]quotaRules.QuotaAllocationEntityTypeRule{}, ruleSet.GetRules()...)
	rules = append(rules, managedRuleSet.GetRules()...)
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].EntityType < rules[j].EntityType
	})
	ruleSet.Rules = rules
	return ruleSet
}

func managedQuotaAllocationRuleSet(ruleSet *quotaRules.QuotaAllocationEntityTypeRuleSet) *quotaRules.QuotaAllocationEntityTypeRuleSet {
	managedRuleSet := &quotaRules.QuotaAllocationEntityTypeRuleSet{}
	if ruleSet == nil {
		return managedRuleSet
	}

	if id := ruleSet.GetId(); id != "" {
		managedRuleSet.SetId(id)
	}
	for _, rule := range ruleSet.GetRules() {
		if quotaAllocationRuleIsManaged(rule) {
			managedRuleSet.Rules = append(managedRuleSet.Rules, rule)
		}
	}
	sort.Slice(managedRuleSet.Rules, func(i, j int) bool {
		return managedRuleSet.Rules[i].EntityType < managedRuleSet.Rules[j].EntityType
	})
	return managedRuleSet
}

func flattenQuotaAllocationRuleSet(ruleSet *quotaRules.QuotaAllocationEntityTypeRuleSet) (*QuotaAllocationRuleSetModel, diag.Diagnostics) {
	rules := ruleSet.GetRules()
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].EntityType < rules[j].EntityType
	})

	stateRules := make([]QuotaAllocationRuleModel, 0, len(rules))
	for _, rule := range rules {
		if quotaAllocationRuleIsManaged(rule) {
			continue
		}

		allocationType := types.StringValue(quotaAllocationTypePercentage)
		if sdkAllocationType, ok := rule.GetAllocationTypeOk(); ok {
			if schemaAllocationType, found := quotaAllocationTypeAPIToSchema[*sdkAllocationType]; found {
				allocationType = types.StringValue(schemaAllocationType)
			}
		}

		stateRules = append(stateRules, QuotaAllocationRuleModel{
			EntityType:     types.StringValue(rule.GetEntityType()),
			Allocation:     types.Float64Value(float32ToSchemaFloat64(rule.GetAllocation())),
			AllocationType: allocationType,
			Enabled:        types.BoolValue(rule.GetEnabled()),
			CanOverflow:    types.BoolValue(rule.GetCanOverflow()),
		})
	}

	id := ruleSet.GetId()
	if id == "" {
		id = quotaAllocationRuleSetImportID
	}

	return &QuotaAllocationRuleSetModel{
		ID:    types.StringValue(id),
		Rules: stateRules,
	}, nil
}

func quotaAllocationRuleIsManaged(rule quotaRules.QuotaAllocationEntityTypeRule) bool {
	value, ok := rule.GetCxManagedOk()
	return ok && *value
}

func float32ToSchemaFloat64(value float32) float64 {
	return normalizeQuotaAllocation(float64(value))
}

func normalizeQuotaAllocation(value float64) float64 {
	parsed, err := strconv.ParseFloat(strconv.FormatFloat(float64(float32(value)), 'f', -1, 32), 64)
	if err != nil {
		return value
	}

	return parsed
}

func formatQuotaAllocation(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func responseStatus(response *http.Response) int {
	if response == nil {
		return 0
	}

	return response.StatusCode
}
