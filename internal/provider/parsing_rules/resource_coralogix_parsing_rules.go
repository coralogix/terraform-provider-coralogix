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

package parsing_rules

import (
	"context"
	"fmt"
	"net/http"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	prgs "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/rule_groups_service"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure   = &ParsingRulesResource{}
	_ resource.ResourceWithImportState = &ParsingRulesResource{}

	rulesSchemaSeverityToApiSeverity = map[string]prgs.Value{
		"debug":    prgs.VALUE_VALUE_DEBUG_OR_UNSPECIFIED,
		"verbose":  prgs.VALUE_VALUE_VERBOSE,
		"info":     prgs.VALUE_VALUE_INFO,
		"warning":  prgs.VALUE_VALUE_WARNING,
		"error":    prgs.VALUE_VALUE_ERROR,
		"critical": prgs.VALUE_VALUE_CRITICAL,
	}
	rulesApiSeverityToSchemaSeverity                 = utils.ReverseMap(rulesSchemaSeverityToApiSeverity)
	parsingRulesValidSeverities                      = utils.GetKeys(rulesSchemaSeverityToApiSeverity)
	rulesSchemaDestinationFieldToApiDestinationField = map[string]prgs.DestinationField{
		"category": prgs.DESTINATIONFIELD_DESTINATION_FIELD_CATEGORY_OR_UNSPECIFIED,
		"class":    prgs.DESTINATIONFIELD_DESTINATION_FIELD_CLASSNAME,
		"method":   prgs.DESTINATIONFIELD_DESTINATION_FIELD_METHODNAME,
		"threadID": prgs.DESTINATIONFIELD_DESTINATION_FIELD_THREADID,
		"severity": prgs.DESTINATIONFIELD_DESTINATION_FIELD_SEVERITY,
		"text":     prgs.DESTINATIONFIELD_DESTINATION_FIELD_TEXT,
	}
	rulesApiDestinationFieldToSchemaDestinationField = utils.ReverseMap(rulesSchemaDestinationFieldToApiDestinationField)
	parsingRulesValidDestinationFields               = utils.GetKeys(rulesSchemaDestinationFieldToApiDestinationField)
	rulesSchemaFormatStandardToApiFormatStandard     = map[string]prgs.FormatStandard{
		"strftime": prgs.FORMATSTANDARD_FORMAT_STANDARD_STRFTIME_OR_UNSPECIFIED,
		"javaSDF":  prgs.FORMATSTANDARD_FORMAT_STANDARD_JAVASDF,
		"golang":   prgs.FORMATSTANDARD_FORMAT_STANDARD_GOLANG,
		"secondTS": prgs.FORMATSTANDARD_FORMAT_STANDARD_SECONDSTS,
		"milliTS":  prgs.FORMATSTANDARD_FORMAT_STANDARD_MILLITS,
		"microTS":  prgs.FORMATSTANDARD_FORMAT_STANDARD_MICROTS,
		"nanoTS":   prgs.FORMATSTANDARD_FORMAT_STANDARD_NANOTS,
	}
	rulesApiFormatStandardToSchemaFormatStandard = utils.ReverseMap(rulesSchemaFormatStandardToApiFormatStandard)
	parsingRulesValidFormatStandards             = utils.GetKeys(rulesSchemaFormatStandardToApiFormatStandard)

	defaultSourceFieldName = "text"
)

type ParsingRulesModel struct {
	ID          types.String `tfsdk:"id"`
	Creator     types.String `tfsdk:"creator"`
	Description types.String `tfsdk:"description"`
	Active      types.Bool   `tfsdk:"active"`
	Hidden      types.Bool   `tfsdk:"hidden"`
	Name        types.String `tfsdk:"name"`
	Order       types.Int64  `tfsdk:"order"`

	Applications []types.String `tfsdk:"applications"`
	Subsystems   []types.String `tfsdk:"subsystems"`
	Severities   []types.String `tfsdk:"severities"`

	RuleSubgroups []RuleSubgroupsModel `tfsdk:"rule_subgroups"`
}

type RuleSubgroupsModel struct {
	ID     types.String        `tfsdk:"id"`
	Active types.Bool          `tfsdk:"active"`
	Order  types.Int64         `tfsdk:"order"`
	Rules  []RuleSubgroupModel `tfsdk:"rules"`
}

type RuleSubgroupModel struct {
	Parse            *ParseModel            `tfsdk:"parse"`
	Block            *BlockModel            `tfsdk:"block"`
	JsonExtract      *JsonExtractModel      `tfsdk:"json_extract"`
	Replace          *ReplaceModel          `tfsdk:"replace"`
	ExtractTimestamp *ExtractTimestampModel `tfsdk:"extract_timestamp"`
	RemoveFields     *RemoveFieldsModel     `tfsdk:"remove_fields"`
	JsonStringify    *JsonStringifyModel    `tfsdk:"json_stringify"`
	Extract          *ExtractModel          `tfsdk:"extract"`
	ParseJsonField   *ParseJsonFieldModel   `tfsdk:"parse_json_field"`
}

type ParseModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Active      types.Bool   `tfsdk:"active"`
	Order       types.Int64  `tfsdk:"order"`

	DestinationField  types.String `tfsdk:"destination_field"`
	SourceField       types.String `tfsdk:"source_field"`
	RegularExpression types.String `tfsdk:"regular_expression"`
}

type BlockModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Active      types.Bool   `tfsdk:"active"`
	Order       types.Int64  `tfsdk:"order"`

	SourceField       types.String `tfsdk:"source_field"`
	RegularExpression types.String `tfsdk:"regular_expression"`
	KeepBlockedLogs   types.Bool   `tfsdk:"keep_blocked_logs"`
	BlockMatchingLogs types.Bool   `tfsdk:"block_all_matching_blocks"`
}

type JsonExtractModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Active      types.Bool   `tfsdk:"active"`
	Order       types.Int64  `tfsdk:"order"`

	DestinationField     types.String `tfsdk:"destination_field"`
	DestinationFieldText types.String `tfsdk:"destination_field_text"`
	JsonKey              types.String `tfsdk:"json_key"`
}

type ReplaceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Active      types.Bool   `tfsdk:"active"`
	Order       types.Int64  `tfsdk:"order"`

	SourceField       types.String `tfsdk:"source_field"`
	DestinationField  types.String `tfsdk:"destination_field"`
	RegularExpression types.String `tfsdk:"regular_expression"`
	ReplacementString types.String `tfsdk:"replacement_string"`
}

type ExtractTimestampModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Active      types.Bool   `tfsdk:"active"`
	Order       types.Int64  `tfsdk:"order"`

	SourceField         types.String `tfsdk:"source_field"`
	FieldFormatStandard types.String `tfsdk:"field_format_standard"`
	TimeFormat          types.String `tfsdk:"time_format"`
}

type RemoveFieldsModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Active      types.Bool   `tfsdk:"active"`
	Order       types.Int64  `tfsdk:"order"`

	ExcludedFields []types.String `tfsdk:"excluded_fields"`
}

type JsonStringifyModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Active      types.Bool   `tfsdk:"active"`
	Order       types.Int64  `tfsdk:"order"`

	SourceField      types.String `tfsdk:"source_field"`
	DestinationField types.String `tfsdk:"destination_field"`
	KeepSourceField  types.Bool   `tfsdk:"keep_source_field"`
}

type ExtractModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Active      types.Bool   `tfsdk:"active"`
	Order       types.Int64  `tfsdk:"order"`

	SourceField       types.String `tfsdk:"source_field"`
	RegularExpression types.String `tfsdk:"regular_expression"`
}

type ParseJsonFieldModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Active      types.Bool   `tfsdk:"active"`
	Order       types.Int64  `tfsdk:"order"`

	SourceField          types.String `tfsdk:"source_field"`
	DestinationField     types.String `tfsdk:"destination_field"`
	KeepSourceField      types.Bool   `tfsdk:"keep_source_field"`
	KeepDestinationField types.Bool   `tfsdk:"keep_destination_field"`
}

func NewParsingRulesResource() resource.Resource {
	return &ParsingRulesResource{}
}

type ParsingRulesResource struct {
	client *prgs.RuleGroupsServiceAPIService
}

func (r *ParsingRulesResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *ParsingRulesResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.ParsingRuleGroups()
}

func (r *ParsingRulesResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_parsing_rules"
}

func (r *ParsingRulesResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Rule-group name",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Rule-group description",
			},
			"active": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Determines whether the rule-group will be active.",
			},
			"applications": schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Default:     setdefault.StaticValue(types.SetValueMust(types.StringType, []attr.Value{})),
				Description: "Rules will execute on logs that match the following applications.",
			},
			"subsystems": schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Default:     setdefault.StaticValue(types.SetValueMust(types.StringType, []attr.Value{})),
				Description: "Rules will execute on logs that match the following subsystems.",
			},
			"severities": schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Description: fmt.Sprintf("Rules will execute on logs that match the these severities. Can be one of %q", parsingRulesValidSeverities),
				Default:     setdefault.StaticValue(types.SetValueMust(types.StringType, []attr.Value{})),
				Validators: []validator.Set{
					setvalidator.ValueStringsAre(stringvalidator.OneOf(parsingRulesValidSeverities...)),
				},
			},
			"hidden": schema.BoolAttribute{
				Optional: true,
				Computed: true,
			},
			"creator": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Rule-group creator.",
			},
			"order": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Determines the index of the rule-group between the other rule-groups. By default, will be added last. (1 based indexing).",
			},
			"rule_subgroups": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed: true,
						},
						"active": schema.BoolAttribute{
							Optional: true,
							Computed: true,
							Default:  booldefault.StaticBool(true),
						},
						"order": schema.Int64Attribute{
							Computed: true,
						},
						"rules": schema.ListNestedAttribute{
							Required: true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"parse": schema.SingleNestedAttribute{
										Optional:   true,
										Attributes: parseSchemaAttrs(),
										Validators: []validator.Object{
											objectvalidator.ExactlyOneOf(
												path.MatchRelative().
													AtParent().
													AtName("parse"),
												path.MatchRelative().
													AtParent().
													AtName("block"),
												path.MatchRelative().
													AtParent().
													AtName("json_extract"),
												path.MatchRelative().
													AtParent().
													AtName("replace"),
												path.MatchRelative().
													AtParent().
													AtName("extract_timestamp"),
												path.MatchRelative().
													AtParent().
													AtName("remove_fields"),
												path.MatchRelative().
													AtParent().
													AtName("json_stringify"),
												path.MatchRelative().
													AtParent().
													AtName("extract"),
												path.MatchRelative().
													AtParent().
													AtName("parse_json_field"),
											),
										},
									},
									"block": schema.SingleNestedAttribute{
										Optional:   true,
										Attributes: blockAttrs()},
									"json_extract": schema.SingleNestedAttribute{
										Optional:   true,
										Attributes: jsonExtractAttrs()},
									"replace": schema.SingleNestedAttribute{
										Optional:   true,
										Attributes: replaceAttrs()},
									"extract_timestamp": schema.SingleNestedAttribute{
										Optional:   true,
										Attributes: extractTimestampAttrs()},
									"remove_fields": schema.SingleNestedAttribute{
										Optional:   true,
										Attributes: removeFieldsAttrs()},
									"json_stringify": schema.SingleNestedAttribute{
										Optional:   true,
										Attributes: jsonStringifyFieldsAttrs()},
									"extract": schema.SingleNestedAttribute{
										Optional:   true,
										Attributes: extractAttrs()},
									"parse_json_field": schema.SingleNestedAttribute{
										Optional:   true,
										Attributes: parseJsonFieldAttrs()},
								},
							},
						},
					},
				},
				Description: "List of rule-subgroups. Every rule-subgroup is a list of rules linked with a logical 'OR' (||) operation.",
			},
		},
	}
}

func (r *ParsingRulesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *ParsingRulesModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rq := extractParsingRules(plan)

	result, httpResponse, err := r.client.
		RuleGroupsServiceCreateRuleGroup(ctx).
		RuleGroupsServiceCreateRuleGroupRequest(*rq).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error creating coralogix_parsing_rules",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", rq),
		)
		return
	}
	state := flattenParsingRules(result.RuleGroup)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *ParsingRulesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan *ParsingRulesModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rq := extractParsingRules(plan)

	result, httpResponse, err := r.client.
		RuleGroupsServiceCreateRuleGroup(ctx).
		RuleGroupsServiceCreateRuleGroupRequest(*rq).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error replacing coralogix_parsing_rules",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", rq),
		)
		return
	}
	state := flattenParsingRules(result.RuleGroup)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *ParsingRulesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *ParsingRulesModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	rq := r.client.RuleGroupsServiceGetRuleGroup(ctx, id)
	result, httpResponse, err := rq.Execute()
	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				"coralogix_parsing_rules is in state, but no longer exists in Coralogix backend",
				"coralogix_parsing_rules will be recreated when you apply",
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error reading coralogix_parsing_rules",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
			)
		}
		return
	}

	state = flattenParsingRules(result.RuleGroup)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ParsingRulesResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state *ParsingRulesModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	rq := r.client.RuleGroupsServiceDeleteRuleGroup(ctx, id)

	_, httpResponse, err := rq.Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error deleting coralogix_parsing_rules",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
		)
	}
}

func extractParsingRules(plan *ParsingRulesModel) *prgs.RuleGroupsServiceCreateRuleGroupRequest {
	ruleMatchers := extractRuleMatchers(plan.Applications, plan.Subsystems, plan.Severities)
	ruleSubGroups := extractRuleSubGroups(plan.RuleSubgroups)
	return &prgs.RuleGroupsServiceCreateRuleGroupRequest{
		Creator:       plan.Creator.ValueStringPointer(),
		Description:   plan.Description.ValueStringPointer(),
		Enabled:       plan.Active.ValueBoolPointer(),
		Hidden:        plan.Hidden.ValueBoolPointer(),
		Name:          plan.Name.ValueStringPointer(),
		Order:         plan.Order.ValueInt64Pointer(),
		RuleMatchers:  ruleMatchers,
		RuleSubgroups: ruleSubGroups,
	}
}

func extractRuleMatchers(apps []types.String, subs []types.String, sevs []types.String) []prgs.RuleMatcher {
	if len(apps) == 0 && len(subs) == 0 && len(sevs) == 0 {
		return nil
	}

	ruleMatchers := make([]prgs.RuleMatcher, len(apps)+len(subs)+len(sevs))
	for i, a := range apps {
		ruleMatchers[i] = prgs.RuleMatcher{
			RuleMatcherApplicationName: &prgs.RuleMatcherApplicationName{
				ApplicationName: &prgs.ApplicationNameConstraint{Value: a.ValueStringPointer()},
			},
		}
	}

	for i, s := range subs {
		ruleMatchers[len(apps)+i] = prgs.RuleMatcher{
			RuleMatcherSubsystemName: &prgs.RuleMatcherSubsystemName{
				SubsystemName: &prgs.SubsystemNameConstraint{Value: s.ValueStringPointer()},
			},
		}
	}

	for i, s := range sevs {
		if !(s.IsNull() && s.IsUnknown()) {
			val := rulesSchemaSeverityToApiSeverity[s.ValueString()]
			ruleMatchers[len(apps)+len(subs)+i] = prgs.RuleMatcher{
				RuleMatcherSeverity: &prgs.RuleMatcherSeverity{
					Severity: &prgs.SeverityConstraint{Value: &val},
				},
			}
		}
	}
	return ruleMatchers
}

func flattenParsingRules(rgrp *prgs.RuleGroup) *ParsingRulesModel {
	applications, subsystems, severities := flattenParsingRuleMatcher(rgrp.RuleMatchers)
	ruleSubgroups := flattenRuleSubGroups(rgrp.RuleSubgroups)
	return &ParsingRulesModel{
		ID:            types.StringPointerValue(rgrp.Id),
		Creator:       types.StringPointerValue(rgrp.Creator),
		Description:   types.StringPointerValue(rgrp.Description),
		Active:        types.BoolPointerValue(rgrp.Enabled),
		Hidden:        types.BoolPointerValue(rgrp.Hidden),
		Name:          types.StringPointerValue(rgrp.Name),
		Order:         types.Int64PointerValue(rgrp.Order),
		Applications:  utils.StringSliceToTypeStringSlice(applications),
		Subsystems:    utils.StringSliceToTypeStringSlice(subsystems),
		Severities:    utils.StringSliceToTypeStringSlice(severities),
		RuleSubgroups: ruleSubgroups,
	}
}

func parseSchemaAttrs() map[string]schema.Attribute {
	parseSchema := commonRulesAttrs()
	parseSchema = appendSourceFieldAttrs(parseSchema)
	parseSchema = appendDestinationFieldAttrs(parseSchema)
	parseSchema = appendRegularExpressionAttrs(parseSchema)
	return parseSchema
}

func blockAttrs() map[string]schema.Attribute {
	blockSchema := commonRulesAttrs()
	blockSchema = appendSourceFieldAttrs(blockSchema)
	blockSchema = appendRegularExpressionAttrs(blockSchema)
	blockSchema["keep_blocked_logs"] = schema.BoolAttribute{
		Optional:            true,
		MarkdownDescription: "Determines if to view blocked logs in LiveTail and archive to S3.",
	}
	blockSchema["block_all_matching_blocks"] = schema.BoolAttribute{
		Optional:            true,
		Computed:            true,
		Default:             booldefault.StaticBool(true),
		MarkdownDescription: "Block Logic. If true or nor set - blocking all matching blocks, if false - blocking all non-matching blocks.",
	}
	return blockSchema
}

func jsonExtractAttrs() map[string]schema.Attribute {
	jsonExtractSchema := commonRulesAttrs()
	jsonExtractSchema["destination_field"] = schema.StringAttribute{
		Required: true,
		Validators: []validator.String{
			stringvalidator.OneOf(parsingRulesValidDestinationFields...),
		},
		MarkdownDescription: fmt.Sprintf("The field that will be populated by the results of RegEx operation."+
			"Can be one of %s.", fmt.Sprint(parsingRulesValidDestinationFields)),
	}
	jsonExtractSchema["destination_field_text"] = schema.StringAttribute{
		Optional:            true,
		MarkdownDescription: "Required when destination_field is 'Text'. should be either 'text' or 'text.<some value>'",
	}
	jsonExtractSchema["json_key"] = schema.StringAttribute{
		Required:    true,
		Description: "JSON key to extract its value directly into a Coralogix metadata field.",
	}
	return jsonExtractSchema
}

func replaceAttrs() map[string]schema.Attribute {
	replaceSchema := commonRulesAttrs()
	replaceSchema = appendRegularExpressionAttrs(replaceSchema)
	replaceSchema = appendSourceFieldAttrs(replaceSchema)
	replaceSchema = appendDestinationFieldAttrs(replaceSchema)
	replaceSchema["replacement_string"] = schema.StringAttribute{
		Optional:    true,
		Description: "The string that will replace the matched RegEx",
	}
	return replaceSchema
}

func extractTimestampAttrs() map[string]schema.Attribute {
	extractTimestampSchema := commonRulesAttrs()
	extractTimestampSchema = appendSourceFieldAttrs(extractTimestampSchema)
	extractTimestampSchema["field_format_standard"] = schema.StringAttribute{
		Required: true,
		Validators: []validator.String{
			stringvalidator.OneOf(parsingRulesValidFormatStandards...),
		},
		Description: fmt.Sprintf("The format standard you want to use. Can be one of %q", parsingRulesValidFormatStandards),
	}
	extractTimestampSchema["time_format"] = schema.StringAttribute{
		Required:    true,
		Description: "A time format that matches the field format standard",
	}
	return extractTimestampSchema
}

func removeFieldsAttrs() map[string]schema.Attribute {
	removeFieldsSchema := commonRulesAttrs()
	removeFieldsSchema["excluded_fields"] = schema.ListAttribute{
		Required:    true,
		ElementType: types.StringType,
		Validators: []validator.List{
			listvalidator.SizeAtLeast(1),
		},
		Description: "Excluded fields won't be indexed.",
	}
	return removeFieldsSchema
}

func jsonStringifyFieldsAttrs() map[string]schema.Attribute {
	jsonStringifySchema := commonRulesAttrs()
	jsonStringifySchema = appendSourceFieldAttrs(jsonStringifySchema)
	jsonStringifySchema = appendDestinationFieldAttrs(jsonStringifySchema)
	jsonStringifySchema["keep_source_field"] = schema.BoolAttribute{
		Optional:    true,
		Computed:    true,
		Default:     booldefault.StaticBool(false),
		Description: "Determines whether to keep or to delete the source field.",
	}
	return jsonStringifySchema
}

func extractAttrs() map[string]schema.Attribute {
	extractSchema := commonRulesAttrs()
	extractSchema = appendSourceFieldAttrs(extractSchema)
	extractSchema = appendRegularExpressionAttrs(extractSchema)
	return extractSchema
}

func parseJsonFieldAttrs() map[string]schema.Attribute {
	parseJsonFieldSchema := commonRulesAttrs()
	parseJsonFieldSchema = appendSourceFieldAttrs(parseJsonFieldSchema)
	parseJsonFieldSchema = appendDestinationFieldAttrs(parseJsonFieldSchema)
	parseJsonFieldSchema["keep_source_field"] = schema.BoolAttribute{
		Optional:    true,
		Computed:    true,
		Default:     booldefault.StaticBool(false),
		Description: "Determines whether to keep or to delete the source field.",
	}
	parseJsonFieldSchema["keep_destination_field"] = schema.BoolAttribute{
		Optional:    true,
		Computed:    true,
		Default:     booldefault.StaticBool(true),
		Description: "Determines whether to keep or to delete the destination field.",
	}
	return parseJsonFieldSchema
}

func commonRulesAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "The rule id.",
		},
		"name": schema.StringAttribute{
			Required:    true,
			Description: "The rule name.",
		},
		"description": schema.StringAttribute{
			Optional:    true,
			Description: "The rule description.",
		},
		"active": schema.BoolAttribute{
			Optional:    true,
			Computed:    true,
			Default:     booldefault.StaticBool(true),
			Description: "Determines whether the rule will be active or not.",
		},
		"order": schema.Int64Attribute{
			Computed:    true,
			Description: "Determines the index of the rule inside the rule-subgroup. Will be computed by the order it was declared (1-based indexing).",
		},
	}
}

func appendSourceFieldAttrs(m map[string]schema.Attribute) map[string]schema.Attribute {
	m["source_field"] = schema.StringAttribute{
		Required:            true,
		MarkdownDescription: "The field on which the Regex will operate on. Accepts lowercase only.",
	}
	return m
}

func appendDestinationFieldAttrs(m map[string]schema.Attribute) map[string]schema.Attribute {
	m["destination_field"] = schema.StringAttribute{
		Required:            true,
		MarkdownDescription: "The field that will be populated by the results of the RegEx operation.",
	}
	return m
}

func appendRegularExpressionAttrs(m map[string]schema.Attribute) map[string]schema.Attribute {
	m["regular_expression"] = schema.StringAttribute{
		Required:            true,
		MarkdownDescription: "Regular expiration. More info: https://coralogix.com/blog/regex-101/",
	}
	return m
}

func extractRuleSubGroups(subgroups []RuleSubgroupsModel) []prgs.CreateRuleGroupRequestCreateRuleSubgroup {
	if len(subgroups) == 0 {
		return nil
	}
	subgroupRules := make([]prgs.CreateRuleGroupRequestCreateRuleSubgroup, len(subgroups))
	for g, groups := range subgroups {
		order := int64(g) + 1
		rules := make([]prgs.CreateRuleGroupRequestCreateRuleSubgroupCreateRule, 0)

		subgroupRules[g] = prgs.CreateRuleGroupRequestCreateRuleSubgroup{
			Enabled: groups.Active.ValueBoolPointer(),
			Order:   &order,
		}

		for i, rule := range groups.Rules {
			order := int64(i) + 1
			if r := rule.Block; r != nil {
				var params prgs.RuleParameters
				if r.BlockMatchingLogs.ValueBool() {
					params = prgs.RuleParameters{
						RuleParametersBlockParameters: &prgs.RuleParametersBlockParameters{
							BlockParameters: &prgs.BlockParameters{
								KeepBlockedLogs: r.KeepBlockedLogs.ValueBoolPointer(),
								Rule:            r.RegularExpression.ValueStringPointer(),
							},
						},
					}
				} else {
					params = prgs.RuleParameters{
						RuleParametersAllowParameters: &prgs.RuleParametersAllowParameters{
							AllowParameters: &prgs.AllowParameters{
								KeepBlockedLogs: r.KeepBlockedLogs.ValueBoolPointer(),
								Rule:            r.RegularExpression.ValueStringPointer(),
							},
						},
					}
				}
				rules = append(rules, prgs.CreateRuleGroupRequestCreateRuleSubgroupCreateRule{
					Description: r.Description.ValueStringPointer(),
					Enabled:     r.Active.ValueBoolPointer(),
					Name:        r.Name.ValueStringPointer(),
					Order:       &order,
					Parameters:  &params,
					SourceField: r.SourceField.ValueStringPointer(),
				})

			}
			if r := rule.JsonExtract; r != nil {
				destinationField := rulesSchemaDestinationFieldToApiDestinationField[r.DestinationField.ValueString()]
				rules = append(rules, prgs.CreateRuleGroupRequestCreateRuleSubgroupCreateRule{
					Description: r.Description.ValueStringPointer(),
					Enabled:     r.Active.ValueBoolPointer(),
					Name:        r.Name.ValueStringPointer(),
					Order:       &order,
					SourceField: &defaultSourceFieldName,
					Parameters: &prgs.RuleParameters{
						RuleParametersJsonExtractParameters: &prgs.RuleParametersJsonExtractParameters{
							JsonExtractParameters: &prgs.JsonExtractParameters{
								DestinationFieldText: r.DestinationFieldText.ValueStringPointer(),
								DestinationFieldType: &destinationField,
								Rule:                 r.JsonKey.ValueStringPointer(),
							},
						},
					},
				})
			}
			if r := rule.Replace; r != nil {
				rules = append(rules, prgs.CreateRuleGroupRequestCreateRuleSubgroupCreateRule{
					Description: r.Description.ValueStringPointer(),
					Enabled:     r.Active.ValueBoolPointer(),
					Name:        r.Name.ValueStringPointer(),
					Order:       &order,
					SourceField: r.SourceField.ValueStringPointer(),
					Parameters: &prgs.RuleParameters{
						RuleParametersReplaceParameters: &prgs.RuleParametersReplaceParameters{
							ReplaceParameters: &prgs.ReplaceParameters{
								DestinationField: r.DestinationField.ValueStringPointer(),
								ReplaceNewVal:    r.ReplacementString.ValueStringPointer(),
								Rule:             r.RegularExpression.ValueStringPointer(),
							},
						},
					},
				})
			}

			if r := rule.ExtractTimestamp; r != nil {
				fmtStd := rulesSchemaFormatStandardToApiFormatStandard[r.FieldFormatStandard.ValueString()]
				rules = append(rules, prgs.CreateRuleGroupRequestCreateRuleSubgroupCreateRule{
					Description: r.Description.ValueStringPointer(),
					Enabled:     r.Active.ValueBoolPointer(),
					Name:        r.Name.ValueStringPointer(),
					Order:       &order,
					SourceField: r.SourceField.ValueStringPointer(),
					Parameters: &prgs.RuleParameters{
						RuleParametersExtractTimestampParameters: &prgs.RuleParametersExtractTimestampParameters{
							ExtractTimestampParameters: &prgs.ExtractTimestampParameters{
								Format:   r.TimeFormat.ValueStringPointer(),
								Standard: &fmtStd,
							},
						},
					},
				})
			}

			if r := rule.RemoveFields; r != nil {
				rules = append(rules, prgs.CreateRuleGroupRequestCreateRuleSubgroupCreateRule{
					Description: r.Description.ValueStringPointer(),
					Enabled:     r.Active.ValueBoolPointer(),
					Name:        r.Name.ValueStringPointer(),
					Order:       &order,
					SourceField: &defaultSourceFieldName,
					Parameters: &prgs.RuleParameters{
						RuleParametersRemoveFieldsParameters: &prgs.RuleParametersRemoveFieldsParameters{
							RemoveFieldsParameters: &prgs.RemoveFieldsParameters{
								Fields: utils.TypeStringSliceToStringSlice(r.ExcludedFields),
							},
						},
					},
				})
			}

			if r := rule.JsonStringify; r != nil {
				deleteSource := !r.KeepSourceField.ValueBool()
				rules = append(rules, prgs.CreateRuleGroupRequestCreateRuleSubgroupCreateRule{
					Description: r.Description.ValueStringPointer(),
					Enabled:     r.Active.ValueBoolPointer(),
					Name:        r.Name.ValueStringPointer(),
					Order:       &order,
					SourceField: r.SourceField.ValueStringPointer(),
					Parameters: &prgs.RuleParameters{
						RuleParametersJsonStringifyParameters: &prgs.RuleParametersJsonStringifyParameters{
							JsonStringifyParameters: &prgs.JsonStringifyParameters{
								DeleteSource:     &deleteSource,
								DestinationField: r.DestinationField.ValueStringPointer(),
							},
						},
					},
				})
			}

			if r := rule.Extract; r != nil {
				rules = append(rules, prgs.CreateRuleGroupRequestCreateRuleSubgroupCreateRule{
					Description: r.Description.ValueStringPointer(),
					Enabled:     r.Active.ValueBoolPointer(),
					Name:        r.Name.ValueStringPointer(),
					Order:       &order,
					SourceField: r.SourceField.ValueStringPointer(),
					Parameters: &prgs.RuleParameters{
						RuleParametersExtractParameters: &prgs.RuleParametersExtractParameters{
							ExtractParameters: &prgs.ExtractParameters{
								Rule: r.RegularExpression.ValueStringPointer(),
							},
						},
					},
				})
			}

			if r := rule.ParseJsonField; r != nil {
				deleteSource := !r.KeepSourceField.ValueBool()
				overrideDestination := !r.KeepDestinationField.ValueBool()
				escapeValue := true
				rules = append(rules, prgs.CreateRuleGroupRequestCreateRuleSubgroupCreateRule{
					Description: r.Description.ValueStringPointer(),
					Enabled:     r.Active.ValueBoolPointer(),
					Name:        r.Name.ValueStringPointer(),
					Order:       &order,
					SourceField: r.SourceField.ValueStringPointer(),
					Parameters: &prgs.RuleParameters{
						RuleParametersJsonParseParameters: &prgs.RuleParametersJsonParseParameters{
							JsonParseParameters: &prgs.JsonParseParameters{
								DeleteSource:     &deleteSource,
								DestinationField: r.DestinationField.ValueStringPointer(),
								EscapedValue:     &escapeValue,
								OverrideDest:     &overrideDestination,
							},
						},
					},
				})
			}

			if r := rule.Parse; r != nil {
				rules = append(rules, prgs.CreateRuleGroupRequestCreateRuleSubgroupCreateRule{
					Description: r.Description.ValueStringPointer(),
					Enabled:     r.Active.ValueBoolPointer(),
					Name:        r.Name.ValueStringPointer(),
					Order:       &order,
					SourceField: r.SourceField.ValueStringPointer(),
					Parameters: &prgs.RuleParameters{
						RuleParametersParseParameters: &prgs.RuleParametersParseParameters{
							ParseParameters: &prgs.ParseParameters{
								DestinationField: r.DestinationField.ValueStringPointer(),
								Rule:             r.RegularExpression.ValueStringPointer(),
							},
						},
					},
				})
			}
		}
		subgroupRules[g].Rules = rules
	}
	return subgroupRules
}

func flattenParsingRuleMatcher(ruleMatchers []prgs.RuleMatcher) ([]string, []string, []string) {
	applications := make([]string, 0)
	subsystems := make([]string, 0)
	severities := make([]string, 0)
	for _, ruleMatcher := range ruleMatchers {

		if ruleMatcher.RuleMatcherApplicationName != nil {
			applications = append(applications, *ruleMatcher.RuleMatcherApplicationName.ApplicationName.Value)
		}
		if ruleMatcher.RuleMatcherSubsystemName != nil {
			subsystems = append(subsystems, *ruleMatcher.RuleMatcherSubsystemName.SubsystemName.Value)

		}
		if ruleMatcher.RuleMatcherSeverity != nil {
			severities = append(severities, rulesApiSeverityToSchemaSeverity[*ruleMatcher.RuleMatcherSeverity.Severity.Value])

		}
	}
	return applications, subsystems, severities
}

func flattenRuleSubGroups(subgroups []prgs.RuleSubgroup) []RuleSubgroupsModel {
	if subgroups == nil {
		return nil
	}
	subgroupRules := make([]RuleSubgroupsModel, len(subgroups))
	for g, groups := range subgroups {
		rules := make([]RuleSubgroupModel, 0)

		subgroupRules[g] = RuleSubgroupsModel{
			Active: types.BoolPointerValue(groups.Enabled),
			Order:  types.Int64PointerValue(groups.Order),
		}
		for _, rule := range groups.Rules {
			params := rule.Parameters

			if p := params.RuleParametersAllowParameters; p != nil {
				rules = append(rules, RuleSubgroupModel{
					Block: &BlockModel{
						ID:                types.StringPointerValue(rule.Id),
						Name:              types.StringPointerValue(rule.Name),
						Description:       types.StringPointerValue(rule.Description),
						Active:            types.BoolPointerValue(rule.Enabled),
						Order:             types.Int64PointerValue(rule.Order),
						SourceField:       types.StringPointerValue(rule.SourceField),
						RegularExpression: types.StringPointerValue(p.AllowParameters.Rule),
						KeepBlockedLogs:   types.BoolPointerValue(p.AllowParameters.KeepBlockedLogs),
						BlockMatchingLogs: types.BoolValue(false),
					},
				})
			}
			if p := params.RuleParametersBlockParameters; p != nil {
				rules = append(rules, RuleSubgroupModel{
					Block: &BlockModel{
						ID:                types.StringPointerValue(rule.Id),
						Name:              types.StringPointerValue(rule.Name),
						Description:       types.StringPointerValue(rule.Description),
						Active:            types.BoolPointerValue(rule.Enabled),
						Order:             types.Int64PointerValue(rule.Order),
						SourceField:       types.StringPointerValue(rule.SourceField),
						RegularExpression: types.StringPointerValue(p.BlockParameters.Rule),
						KeepBlockedLogs:   types.BoolPointerValue(p.BlockParameters.KeepBlockedLogs),
						BlockMatchingLogs: types.BoolValue(true),
					},
				})
			}
			if p := params.RuleParametersExtractParameters; p != nil {
				rules = append(rules, RuleSubgroupModel{
					Extract: &ExtractModel{
						ID:                types.StringPointerValue(rule.Id),
						Name:              types.StringPointerValue(rule.Name),
						Description:       types.StringPointerValue(rule.Description),
						Active:            types.BoolPointerValue(rule.Enabled),
						Order:             types.Int64PointerValue(rule.Order),
						SourceField:       types.StringPointerValue(rule.SourceField),
						RegularExpression: types.StringPointerValue(p.ExtractParameters.Rule),
					},
				})

			}
			if p := params.RuleParametersExtractTimestampParameters; p != nil {
				fmtStd := rulesApiFormatStandardToSchemaFormatStandard[*p.ExtractTimestampParameters.Standard]
				rules = append(rules, RuleSubgroupModel{
					ExtractTimestamp: &ExtractTimestampModel{
						ID:                  types.StringPointerValue(rule.Id),
						Name:                types.StringPointerValue(rule.Name),
						Description:         types.StringPointerValue(rule.Description),
						Active:              types.BoolPointerValue(rule.Enabled),
						Order:               types.Int64PointerValue(rule.Order),
						SourceField:         types.StringPointerValue(rule.SourceField),
						FieldFormatStandard: types.StringValue(fmtStd),
						TimeFormat:          types.StringPointerValue(p.ExtractTimestampParameters.Format),
					},
				})
			}
			if p := params.RuleParametersJsonExtractParameters; p != nil {
				destinationField := rulesApiDestinationFieldToSchemaDestinationField[*p.JsonExtractParameters.DestinationFieldType]
				rules = append(rules, RuleSubgroupModel{
					JsonExtract: &JsonExtractModel{
						ID:                   types.StringPointerValue(rule.Id),
						Name:                 types.StringPointerValue(rule.Name),
						Description:          types.StringPointerValue(rule.Description),
						Active:               types.BoolPointerValue(rule.Enabled),
						Order:                types.Int64PointerValue(rule.Order),
						DestinationField:     types.StringValue(destinationField),
						DestinationFieldText: types.StringPointerValue(p.JsonExtractParameters.DestinationFieldText),
						JsonKey:              types.StringPointerValue(p.JsonExtractParameters.Rule),
					},
				})
			}
			if p := params.RuleParametersJsonParseParameters; p != nil {
				keepSourceField := !*p.JsonParseParameters.DeleteSource
				keepDestinationField := !*p.JsonParseParameters.OverrideDest

				rules = append(rules, RuleSubgroupModel{
					ParseJsonField: &ParseJsonFieldModel{
						ID:                   types.StringPointerValue(rule.Id),
						Name:                 types.StringPointerValue(rule.Name),
						Description:          types.StringPointerValue(rule.Description),
						Active:               types.BoolPointerValue(rule.Enabled),
						Order:                types.Int64PointerValue(rule.Order),
						SourceField:          types.StringPointerValue(rule.SourceField),
						DestinationField:     types.StringPointerValue(p.JsonParseParameters.DestinationField),
						KeepSourceField:      types.BoolValue(keepSourceField),
						KeepDestinationField: types.BoolValue(keepDestinationField),
					},
				})
			}
			if p := params.RuleParametersJsonStringifyParameters; p != nil {
				keepSourceField := !*params.RuleParametersJsonStringifyParameters.JsonStringifyParameters.DeleteSource
				rules = append(rules, RuleSubgroupModel{
					JsonStringify: &JsonStringifyModel{
						ID:               types.StringPointerValue(rule.Id),
						Name:             types.StringPointerValue(rule.Name),
						Description:      types.StringPointerValue(rule.Description),
						Active:           types.BoolPointerValue(rule.Enabled),
						Order:            types.Int64PointerValue(rule.Order),
						SourceField:      types.StringPointerValue(rule.SourceField),
						DestinationField: types.StringPointerValue(p.JsonStringifyParameters.DestinationField),
						KeepSourceField:  types.BoolValue(keepSourceField),
					},
				})
			}
			if p := params.RuleParametersParseParameters; p != nil {
				rules = append(rules, RuleSubgroupModel{
					Parse: &ParseModel{
						ID:                types.StringPointerValue(rule.Id),
						Name:              types.StringPointerValue(rule.Name),
						Description:       types.StringPointerValue(rule.Description),
						Active:            types.BoolPointerValue(rule.Enabled),
						Order:             types.Int64PointerValue(rule.Order),
						SourceField:       types.StringPointerValue(rule.SourceField),
						DestinationField:  types.StringPointerValue(p.ParseParameters.DestinationField),
						RegularExpression: types.StringPointerValue(p.ParseParameters.Rule),
					},
				})
			}
			if p := params.RuleParametersRemoveFieldsParameters; p != nil {
				rules = append(rules, RuleSubgroupModel{
					RemoveFields: &RemoveFieldsModel{
						ID:             types.StringPointerValue(rule.Id),
						Name:           types.StringPointerValue(rule.Name),
						Description:    types.StringPointerValue(rule.Description),
						Active:         types.BoolPointerValue(rule.Enabled),
						Order:          types.Int64PointerValue(rule.Order),
						ExcludedFields: utils.StringSliceToTypeStringSlice(p.RemoveFieldsParameters.Fields),
					},
				})
			}
			if p := params.RuleParametersReplaceParameters; p != nil {
				rules = append(rules, RuleSubgroupModel{
					Replace: &ReplaceModel{
						ID:                types.StringPointerValue(rule.Id),
						Name:              types.StringPointerValue(rule.Name),
						Description:       types.StringPointerValue(rule.Description),
						Active:            types.BoolPointerValue(rule.Enabled),
						Order:             types.Int64PointerValue(rule.Order),
						SourceField:       types.StringPointerValue(rule.SourceField),
						DestinationField:  types.StringPointerValue(p.ReplaceParameters.DestinationField),
						RegularExpression: types.StringPointerValue(p.ReplaceParameters.Rule),
						ReplacementString: types.StringPointerValue(p.ReplaceParameters.ReplaceNewVal),
					},
				})
			}
		}
		subgroupRules[g].Rules = rules
	}

	return subgroupRules
}
