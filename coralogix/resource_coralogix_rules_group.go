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
	"strings"
	"time"

	"terraform-provider-coralogix/coralogix/clientset"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"google.golang.org/protobuf/encoding/protojson"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	rulesSchemaSeverityToProtoSeverity = map[string]cxsdk.SeverityConstraintValue{
		"debug":    cxsdk.SeverityConstraintValueDebugOrUnspecified,
		"verbose":  cxsdk.SeverityConstraintValueVerbose,
		"info":     cxsdk.SeverityConstraintValueInfo,
		"warning":  cxsdk.SeverityConstraintValueWarning,
		"error":    cxsdk.SeverityConstraintValueError,
		"critical": cxsdk.SeverityConstraintValueCritical,
	}
	rulesProtoSeverityToSchemaSeverity                 = ReverseMap(rulesSchemaSeverityToProtoSeverity)
	rulesValidSeverities                               = GetKeys(rulesSchemaSeverityToProtoSeverity)
	rulesSchemaDestinationFieldToProtoDestinationField = map[string]cxsdk.JSONExtractParametersDestinationField{
		"Category": cxsdk.JSONExtractParametersDestinationFieldCategoryOrUnspecified,
		"Class":    cxsdk.JSONExtractParametersDestinationFieldClassName,
		"Method":   cxsdk.JSONExtractParametersDestinationFieldMethodName,
		"ThreadID": cxsdk.JSONExtractParametersDestinationFieldThreadID,
		"Severity": cxsdk.JSONExtractParametersDestinationFieldSeverity,
		"Text":     cxsdk.JSONExtractParametersDestinationFieldText,
	}
	rulesProtoDestinationFieldToSchemaDestinationField = ReverseMap(rulesSchemaDestinationFieldToProtoDestinationField)
	rulesValidDestinationFields                        = GetKeys(rulesSchemaDestinationFieldToProtoDestinationField)
	rulesSchemaFormatStandardToProtoFormatStandard     = map[string]cxsdk.ExtractTimestampParametersFormatStandard{
		"strftime": cxsdk.ExtractTimestampParametersFormatStandardStrftimeOrUnspecified,
		"javaSDF":  cxsdk.ExtractTimestampParametersFormatStandardJavasdf,
		"golang":   cxsdk.ExtractTimestampParametersFormatStandardGolang,
		"secondTS": cxsdk.ExtractTimestampParametersFormatStandardSecondsTS,
		"milliTS":  cxsdk.ExtractTimestampParametersFormatStandardMilliTS,
		"microTS":  cxsdk.ExtractTimestampParametersFormatStandardMicroTS,
		"nanoTS":   cxsdk.ExtractTimestampParametersFormatStandardNanoTS,
	}
	rulesProtoFormatStandardToSchemaFormatStandard = ReverseMap(rulesSchemaFormatStandardToProtoFormatStandard)
	rulesValidFormatStandards                      = GetKeys(rulesSchemaFormatStandardToProtoFormatStandard)
)

func resourceCoralogixRulesGroup() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCoralogixRulesGroupCreate,
		ReadContext:   resourceCoralogixRulesGroupRead,
		UpdateContext: resourceCoralogixRulesGroupUpdate,
		DeleteContext: resourceCoralogixRulesGroupDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(60 * time.Second),
			Read:   schema.DefaultTimeout(30 * time.Second),
			Update: schema.DefaultTimeout(60 * time.Second),
			Delete: schema.DefaultTimeout(30 * time.Second),
		},

		Schema: RulesGroupSchema(),

		Description: "Rule-group is list of rule-subgroups with 'and' (&&) operation between. For more info please review - https://coralogix.com/docs/log-parsing-rules/ .",
	}
}

func RulesGroupSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"name": {
			Type:         schema.TypeString,
			Required:     true,
			ValidateFunc: validation.StringIsNotEmpty,
			Description:  "Rule-group name",
		},
		"description": {
			Type:        schema.TypeString,
			Optional:    true,
			Default:     "",
			Description: "Rule-group description",
		},
		"active": {
			Type:        schema.TypeBool,
			Optional:    true,
			Default:     true,
			Description: "Determines whether the rule-group will be active.",
		},
		"applications": {
			Type:     schema.TypeSet,
			Optional: true,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
			Description: "Rules will execute on logs that match the following applications.",
			Set:         schema.HashString,
		},
		"subsystems": {
			Type:     schema.TypeSet,
			Optional: true,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
			Description: "Rules will execute on logs that match the following subsystems.",
			Set:         schema.HashString,
		},
		"severities": {
			Type:     schema.TypeSet,
			Optional: true,
			Elem: &schema.Schema{
				Type:         schema.TypeString,
				ValidateFunc: validation.StringInSlice(rulesValidSeverities, false),
			},
			Description: fmt.Sprintf("Rules will execute on logs that match the following severities. Can be one of %q", rulesValidSeverities),
			Set:         schema.HashString,
		},
		"hidden": {
			Type:     schema.TypeBool,
			Optional: true,
			Default:  false,
		},
		"creator": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "Rule-group creator.",
		},
		"order": {
			Type:         schema.TypeInt,
			Optional:     true,
			Computed:     true,
			Description:  "Determines the index of the rule-group between the other rule-groups. By default, will be added last. (1 based indexing).",
			ValidateFunc: validation.IntAtLeast(1),
		},
		"rule_subgroups": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"id": {
						Type:        schema.TypeString,
						Computed:    true,
						Description: "The ID of thr rule-subgroup. Will be computed by Coralogix endpoint.",
					},
					"active": {
						Type:        schema.TypeBool,
						Optional:    true,
						Default:     true,
						Description: "Determines whether the rule-subgroup will be active.",
					},
					"order": {
						Type:     schema.TypeInt,
						Optional: true,
						Computed: true,
						Description: "Determines the index of the rule-subgroup inside the rule-group." +
							"When not set, will be computed by the order it was declared. (1 based indexing).",
						ValidateFunc: validation.IntAtLeast(1),
					},
					"rules": {
						Type: schema.TypeList,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"parse": {
									Type:     schema.TypeList,
									Optional: true,
									Elem: &schema.Resource{
										Schema: parseSchema(),
									},
									Description: "Parse unstructured logs into JSON format using named Regex groups.",
									MaxItems:    1,
								},
								"block": {
									Type:     schema.TypeList,
									Optional: true,
									Elem: &schema.Resource{
										Schema: blockSchema(),
									},
									Description: "Block rules allow for refined filtering of incoming logs with a RegEx.",
									MaxItems:    1,
								},
								"json_extract": {
									Type:     schema.TypeList,
									Optional: true,
									Elem: &schema.Resource{
										Schema: jsonExtractSchema(),
									},
									Description: "Name a JSON field to extract its value directly into a Coralogix metadata field",
									MaxItems:    1,
								},
								"replace": {
									Type:     schema.TypeList,
									Optional: true,
									Elem: &schema.Resource{
										Schema: replaceSchema(),
									},
									Description: "Replace rules are used to strings in order to fix log structure, change log severity, or obscure information.",
									MaxItems:    1,
								},
								"extract_timestamp": {
									Type:     schema.TypeList,
									Optional: true,
									Elem: &schema.Resource{
										Schema: extractTimestampSchema(),
									},
									Description: "Replace rules are used to replace logs timestamp with JSON field.",
									MaxItems:    1,
								},
								"remove_fields": {
									Type:     schema.TypeList,
									Optional: true,
									Elem: &schema.Resource{
										Schema: removeFieldsSchema(),
									},
									Description: "Remove Fields allows to select fields that will not be indexed.",
									MaxItems:    1,
								},
								"json_stringify": {
									Type:     schema.TypeList,
									Optional: true,
									Elem: &schema.Resource{
										Schema: jsonStringifyFieldsSchema(),
									},
									Description: "Convert JSON object to JSON string.",
									MaxItems:    1,
								},
								"extract": {
									Type:     schema.TypeList,
									Optional: true,
									Elem: &schema.Resource{
										Schema: extractSchema(),
									},
									Description: "Use a named RegEx group to extract specific values you need as JSON getKeysStrings without having to parse the entire log.",
									MaxItems:    1,
								},
								"parse_json_field": {
									Type:     schema.TypeList,
									Optional: true,
									Elem: &schema.Resource{
										Schema: parseJsonFieldSchema(),
									},
									Description: "Convert JSON string to JSON object.",
									MaxItems:    1,
								},
							},
						},
						Required: true,
					},
				},
			},
			Description: "List of rule-subgroups. Every rule-subgroup is list of rules with 'or' (||) operation between.",
		},
	}
}

func parseSchema() map[string]*schema.Schema {
	parseSchema := commonRulesSchema()
	parseSchema = appendSourceFieldSchema(parseSchema)
	parseSchema = appendDestinationFieldSchema(parseSchema)
	parseSchema = appendRegularExpressionSchema(parseSchema)
	return parseSchema
}

func blockSchema() map[string]*schema.Schema {
	blockSchema := commonRulesSchema()
	blockSchema = appendSourceFieldSchema(blockSchema)
	blockSchema = appendRegularExpressionSchema(blockSchema)
	blockSchema["keep_blocked_logs"] = &schema.Schema{
		Type:        schema.TypeBool,
		Optional:    true,
		Default:     false,
		Description: "Determines if to view blocked logs in LiveTail and archive to S3.",
	}
	blockSchema["blocking_all_matching_blocks"] = &schema.Schema{
		Type:        schema.TypeBool,
		Optional:    true,
		Default:     true,
		Description: "Block Logic. If true or nor set - blocking all matching blocks, if false - blocking all non-matching blocks.",
	}
	return blockSchema
}

func jsonExtractSchema() map[string]*schema.Schema {
	jsonExtractSchema := commonRulesSchema()
	jsonExtractSchema["destination_field"] = &schema.Schema{
		Type:         schema.TypeString,
		Required:     true,
		ValidateFunc: validation.StringInSlice(rulesValidDestinationFields, false),
		Description: fmt.Sprintf("The field that will be populated by the results of RegEx operation."+
			"Can be one of %s.", fmt.Sprint(rulesValidDestinationFields)),
	}
	jsonExtractSchema["destination_field_text"] = &schema.Schema{
		Type:        schema.TypeString,
		Optional:    true,
		Description: "Required when destination_field is 'Text'. should be either 'text' or 'text.<some value>'",
	}
	jsonExtractSchema["json_key"] = &schema.Schema{
		Type:        schema.TypeString,
		Required:    true,
		Description: "JSON key to extract its value directly into a Coralogix metadata field.",
	}
	return jsonExtractSchema
}

func replaceSchema() map[string]*schema.Schema {
	replaceSchema := commonRulesSchema()
	replaceSchema = appendRegularExpressionSchema(replaceSchema)
	replaceSchema = appendSourceFieldSchema(replaceSchema)
	replaceSchema = appendDestinationFieldSchema(replaceSchema)
	replaceSchema["replacement_string"] = &schema.Schema{
		Type:        schema.TypeString,
		Optional:    true,
		Default:     "",
		Description: "The string that will replace the matched RegEx",
	}
	return replaceSchema
}

func extractTimestampSchema() map[string]*schema.Schema {
	extractTimestampSchema := commonRulesSchema()
	extractTimestampSchema = appendSourceFieldSchema(extractTimestampSchema)
	extractTimestampSchema["field_format_standard"] = &schema.Schema{
		Type:         schema.TypeString,
		Required:     true,
		ValidateFunc: validation.StringInSlice(rulesValidFormatStandards, false),
		Description:  fmt.Sprintf("The format standard you want to use. Can be one of %q", rulesValidFormatStandards),
	}
	extractTimestampSchema["time_format"] = &schema.Schema{
		Type:        schema.TypeString,
		Required:    true,
		Description: "A time format that matches the field format standard",
	}
	return extractTimestampSchema
}

func removeFieldsSchema() map[string]*schema.Schema {
	removeFieldsSchema := commonRulesSchema()
	removeFieldsSchema["excluded_fields"] = &schema.Schema{
		Type:     schema.TypeList,
		Required: true,
		Elem: &schema.Schema{
			Type: schema.TypeString,
		},
		MinItems:    1,
		Description: "Excluded fields won't be indexed.",
	}
	return removeFieldsSchema
}

func jsonStringifyFieldsSchema() map[string]*schema.Schema {
	jsonStringifySchema := commonRulesSchema()
	jsonStringifySchema = appendSourceFieldSchema(jsonStringifySchema)
	jsonStringifySchema = appendDestinationFieldSchema(jsonStringifySchema)
	jsonStringifySchema["keep_source_field"] = &schema.Schema{
		Type:        schema.TypeBool,
		Optional:    true,
		Default:     false,
		Description: "Determines whether to keep or to delete the source field.",
	}
	return jsonStringifySchema
}

func extractSchema() map[string]*schema.Schema {
	extractSchema := commonRulesSchema()
	extractSchema = appendSourceFieldSchema(extractSchema)
	extractSchema = appendRegularExpressionSchema(extractSchema)
	return extractSchema
}

func parseJsonFieldSchema() map[string]*schema.Schema {
	parseJsonFieldSchema := commonRulesSchema()
	parseJsonFieldSchema = appendSourceFieldSchema(parseJsonFieldSchema)
	parseJsonFieldSchema = appendDestinationFieldSchema(parseJsonFieldSchema)
	parseJsonFieldSchema["keep_source_field"] = &schema.Schema{
		Type:        schema.TypeBool,
		Optional:    true,
		Default:     false,
		Description: "Determines whether to keep or to delete the source field.",
	}
	parseJsonFieldSchema["keep_destination_field"] = &schema.Schema{
		Type:        schema.TypeBool,
		Optional:    true,
		Default:     true,
		Description: "Determines whether to keep or to delete the destination field.",
	}
	return parseJsonFieldSchema
}

func commonRulesSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"id": {
			Type:        schema.TypeString,
			Computed:    true,
			Description: "The rule id.",
		},
		"name": {
			Type:         schema.TypeString,
			Required:     true,
			ValidateFunc: validation.StringIsNotEmpty,
			Description:  "The rule name.",
		},
		"description": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "The rule description.",
		},
		"active": {
			Type:        schema.TypeBool,
			Optional:    true,
			Default:     true,
			Description: "Determines whether to rule will be active or not.",
		},
		"order": {
			Type:     schema.TypeInt,
			Computed: true,
			Optional: true,
			Description: "Determines the index of the rule inside the rule-subgroup." +
				"When not set, will be computed by the order it was declared. (1 based indexing).",
			ValidateFunc: validation.IntAtLeast(1),
		},
	}
}

func appendSourceFieldSchema(m map[string]*schema.Schema) map[string]*schema.Schema {
	m["source_field"] = &schema.Schema{
		Type:        schema.TypeString,
		Required:    true,
		Description: "The field on which the Regex will operate on. Accepts lowercase only.",
	}
	return m
}

func appendDestinationFieldSchema(m map[string]*schema.Schema) map[string]*schema.Schema {
	m["destination_field"] = &schema.Schema{
		Type:        schema.TypeString,
		Required:    true,
		Description: "The field that will be populated by the results of the RegEx operation.",
	}
	return m
}

func appendRegularExpressionSchema(m map[string]*schema.Schema) map[string]*schema.Schema {
	m["regular_expression"] = &schema.Schema{
		Type:        schema.TypeString,
		Required:    true,
		Description: "Regular expiration. More info: https://coralogix.com/blog/regex-101/",
	}
	return m
}

func resourceCoralogixRulesGroupCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	createRuleGroupRequest, err := extractCreateRuleGroupRequest(d)
	if err != nil {
		return diag.FromErr(err)
	}

	log.Printf("[INFO] Creating new rule-group: %s", protojson.Format(createRuleGroupRequest))
	ruleGroupResp, err := meta.(*clientset.ClientSet).RuleGroups().Create(ctx, createRuleGroupRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		return diag.Errorf(formatRpcErrors(err, cxsdk.RuleGroupsCreateRuleGroupRPC, protojson.Format(createRuleGroupRequest)))
	}
	ruleGroup := ruleGroupResp.GetRuleGroup()
	log.Printf("[INFO] Submitted new rule-group: %s", protojson.Format(ruleGroup))
	d.SetId(ruleGroup.GetId().GetValue())

	return resourceCoralogixRulesGroupRead(ctx, d, meta)
}

func resourceCoralogixRulesGroupRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	getRuleGroupRequest := &cxsdk.GetRuleGroupRequest{
		GroupId: id,
	}

	log.Printf("[INFO] Reading rule-group %s", id)
	ruleGroupResp, err := meta.(*clientset.ClientSet).RuleGroups().Get(ctx, getRuleGroupRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.NotFound {
			d.SetId("")
			return diag.Diagnostics{diag.Diagnostic{
				Severity: diag.Warning,
				Summary:  fmt.Sprintf("Rule-Group %q is in state, but no longer exists in Coralogix backend", id),
				Detail:   fmt.Sprintf("%s will be recreated when you apply", id),
			}}
		}
		return diag.Errorf(formatRpcErrors(err, cxsdk.RuleGroupsGetRuleGroupRPC, protojson.Format(getRuleGroupRequest)))
	}
	ruleGroup := ruleGroupResp.GetRuleGroup()
	log.Printf("[INFO] Received rule-group: %s", protojson.Format(ruleGroup))

	return setRuleGroup(d, ruleGroup)
}

func resourceCoralogixRulesGroupUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	req, err := extractCreateRuleGroupRequest(d)
	if err != nil {
		return diag.FromErr(err)
	}

	id := d.Id()
	updateRuleGroupRequest := &cxsdk.UpdateRuleGroupRequest{
		GroupId:   wrapperspb.String(id),
		RuleGroup: req,
	}

	log.Printf("[INFO] Updating rule-group %s to %s", id, protojson.Format(updateRuleGroupRequest))
	ruleGroupResp, err := meta.(*clientset.ClientSet).RuleGroups().Update(ctx, updateRuleGroupRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		return diag.Errorf(formatRpcErrors(err, cxsdk.RuleGroupsUpdateRuleGroupRPC, protojson.Format(updateRuleGroupRequest)))
	}
	log.Printf("[INFO] Submitted updated rule-group: %s", protojson.Format(ruleGroupResp))

	return resourceCoralogixRulesGroupRead(ctx, d, meta)
}

func resourceCoralogixRulesGroupDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	deleteRuleGroupRequest := &cxsdk.DeleteRuleGroupRequest{
		GroupId: id,
	}

	log.Printf("[INFO] Deleting rule-group %s", id)
	_, err := meta.(*clientset.ClientSet).RuleGroups().Delete(ctx, deleteRuleGroupRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		return diag.Errorf(formatRpcErrors(err, cxsdk.RuleGroupsDeleteRuleGroupRPC, protojson.Format(deleteRuleGroupRequest)))
	}
	log.Printf("[INFO] rule-group %s deleted", id)

	d.SetId("")
	return nil
}

func extractCreateRuleGroupRequest(d *schema.ResourceData) (*cxsdk.CreateRuleGroupRequest, error) {
	name := wrapperspb.String(d.Get("name").(string))
	description := wrapperspb.String(d.Get("description").(string))
	creator := wrapperspb.String(d.Get("creator").(string))
	enabled := wrapperspb.Bool(d.Get("active").(bool))
	hidden := wrapperspb.Bool(d.Get("hidden").(bool))
	ruleMatchers := expandRuleMatcher(d)
	rulesSubgroups, err := expandRuleSubgroups(d.Get("rule_subgroups"))
	if err != nil {
		return nil, err
	}
	order := wrapperspb.UInt32(uint32(d.Get("order").(int)))
	createRuleGroupRequest := &cxsdk.CreateRuleGroupRequest{
		Name:          name,
		Description:   description,
		Creator:       creator,
		Enabled:       enabled,
		Hidden:        hidden,
		RuleMatchers:  ruleMatchers,
		RuleSubgroups: rulesSubgroups,
		Order:         order,
	}
	return createRuleGroupRequest, nil
}

func setRuleGroup(d *schema.ResourceData, ruleGroup *cxsdk.RuleGroup) diag.Diagnostics {
	if err := d.Set("active", ruleGroup.GetEnabled().GetValue()); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("name", ruleGroup.GetName().GetValue()); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("description", ruleGroup.GetDescription().GetValue()); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("creator", ruleGroup.GetCreator().GetValue()); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("order", ruleGroup.GetOrder().GetValue()); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("hidden", ruleGroup.GetHidden().GetValue()); err != nil {
		return diag.FromErr(err)
	}

	ruleMatcher, err := flattenRuleMatcher(ruleGroup.GetRuleMatchers())
	if err != nil {
		return diag.FromErr(err)
	}
	if err = d.Set("applications", ruleMatcher["applications"]); err != nil {
		return diag.FromErr(err)
	}
	if err = d.Set("subsystems", ruleMatcher["subsystems"]); err != nil {
		return diag.FromErr(err)
	}
	if err = d.Set("severities", ruleMatcher["severities"]); err != nil {
		return diag.FromErr(err)
	}

	if ruleSubgroups, err := flattenRuleSubgroups(ruleGroup.GetRuleSubgroups()); err != nil {
		return diag.FromErr(err)
	} else if err = d.Set("rule_subgroups", ruleSubgroups); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func expandRuleMatcher(d *schema.ResourceData) []*cxsdk.RuleMatcher {
	applications := d.Get("applications").(*schema.Set).List()
	subsystems := d.Get("subsystems").(*schema.Set).List()
	severities := d.Get("severities").(*schema.Set).List()
	ruleMatchers := make([]*cxsdk.RuleMatcher, 0, len(applications)+len(subsystems)+len(severities))

	for _, app := range applications {
		constraintStr := wrapperspb.String(app.(string))
		applicationNameConstraint := cxsdk.ApplicationNameConstraint{Value: constraintStr}
		ruleMatcherApplicationName := cxsdk.RuleMatcherApplicationName{ApplicationName: &applicationNameConstraint}
		ruleMatchers = append(ruleMatchers, &cxsdk.RuleMatcher{Constraint: &ruleMatcherApplicationName})
	}

	for _, subSys := range subsystems {
		constraintStr := wrapperspb.String(subSys.(string))
		subsystemNameConstraint := cxsdk.SubsystemNameConstraint{Value: constraintStr}
		ruleMatcherApplicationName := cxsdk.RuleMatcherSubsystemName{SubsystemName: &subsystemNameConstraint}
		ruleMatchers = append(ruleMatchers, &cxsdk.RuleMatcher{Constraint: &ruleMatcherApplicationName})
	}

	for _, sev := range severities {
		constraintEnum := expandRuledSeverity(sev.(string))
		severityConstraint := cxsdk.SeverityConstraint{Value: constraintEnum}
		ruleMatcherSeverity := cxsdk.RuleMatcherSeverity{Severity: &severityConstraint}
		ruleMatchers = append(ruleMatchers, &cxsdk.RuleMatcher{Constraint: &ruleMatcherSeverity})
	}

	return ruleMatchers
}

func expandRuledSeverity(severity string) cxsdk.SeverityConstraintValue {
	return cxsdk.SeverityConstraintValue(rulesSchemaSeverityToProtoSeverity[strings.ToLower(severity)])
}

func expandRuleSubgroups(v interface{}) ([]*cxsdk.CreateRuleGroupRequestCreateRuleSubgroup, error) {
	s := v.([]interface{})
	ruleSubgroups := make([]*cxsdk.CreateRuleGroupRequestCreateRuleSubgroup, 0, len(s))
	for i, o := range s {
		m := o.(map[string]interface{})
		rsg, err := expandRuleSubgroup(m)
		if err != nil {
			return nil, err
		}

		if rsg.Order == nil {
			rsg.Order = wrapperspb.UInt32(uint32(i + 1))
		}

		ruleSubgroups = append(ruleSubgroups, rsg)
	}

	return ruleSubgroups, nil
}

func expandRuleSubgroup(m map[string]interface{}) (*cxsdk.CreateRuleGroupRequestCreateRuleSubgroup, error) {
	rules, err := expandRules(m["rules"].([]interface{}))
	if err != nil {
		return nil, err
	}

	active := wrapperspb.Bool(m["active"].(bool))

	var order *wrapperspb.UInt32Value
	if o, ok := m["order"].(int); ok && o != 0 {
		order = wrapperspb.UInt32(uint32(o))
	}

	return &cxsdk.CreateRuleGroupRequestCreateRuleSubgroup{
		Rules:   rules,
		Enabled: active,
		Order:   order,
	}, nil
}

func expandRules(s []interface{}) ([]*cxsdk.CreateRuleGroupRequestCreateRuleSubgroupCreateRule, error) {
	rules := make([]*cxsdk.CreateRuleGroupRequestCreateRuleSubgroupCreateRule, 0)
	for i, v := range s {
		rule, err := expandRule(v)
		if err != nil {
			return nil, err
		}

		if rule.Order == nil {
			rule.Order = wrapperspb.UInt32(uint32(i + 1))
		}

		rules = append(rules, rule)
	}
	return rules, nil
}

func expandRule(i interface{}) (*cxsdk.CreateRuleGroupRequestCreateRuleSubgroupCreateRule, error) {
	m := i.(map[string]interface{})
	var rule *cxsdk.CreateRuleGroupRequestCreateRuleSubgroupCreateRule
	for k, v := range m {
		if r, ok := v.([]interface{}); ok && len(r) > 0 {
			if rule == nil {
				rule = expandRuleForSpecificRuleType(k, r[0])
			} else {
				return nil, fmt.Errorf("exactly one of %q must be provided inside rule. more than one rule type where provided", getKeysInterface(m))
			}
		}
	}
	if rule == nil {
		return nil, fmt.Errorf("exactly one of %q must be provided inside rule. no rule type was provided", getKeysInterface(m))
	}
	return rule, nil
}

func expandRuleForSpecificRuleType(rulesType string, i interface{}) *cxsdk.CreateRuleGroupRequestCreateRuleSubgroupCreateRule {
	m := i.(map[string]interface{})

	var order *wrapperspb.UInt32Value
	if o, ok := m["order"].(int); ok && o != 0 {
		order = wrapperspb.UInt32(uint32(o))
	}

	return &cxsdk.CreateRuleGroupRequestCreateRuleSubgroupCreateRule{
		Name:        wrapperspb.String(m["name"].(string)),
		Description: wrapperspb.String(m["description"].(string)),
		SourceField: func() *wrapperspb.StringValue {
			if sourceFieldObj, ok := m["source_field"]; ok {
				return wrapperspb.String(sourceFieldObj.(string))
			}
			return wrapperspb.String("text")
		}(),
		Enabled:    wrapperspb.Bool(m["active"].(bool)),
		Order:      order,
		Parameters: expandParameters(rulesType, m),
	}
}

func expandParameters(ruleType string, m map[string]interface{}) *cxsdk.RuleParameters {
	var ruleParameters cxsdk.RuleParameters

	switch ruleType {
	case "parse":
		destinationField := wrapperspb.String(m["destination_field"].(string))
		rule := wrapperspb.String(m["regular_expression"].(string))
		parseParameters := cxsdk.ParseParameters{DestinationField: destinationField, Rule: rule}
		ruleParametersParseParameters := cxsdk.RuleParametersParseParameters{ParseParameters: &parseParameters}
		ruleParameters.RuleParameters = &ruleParametersParseParameters
	case "extract":
		rule := wrapperspb.String(m["regular_expression"].(string))
		extractParameters := cxsdk.ExtractParameters{Rule: rule}
		ruleParametersExtractParameters := cxsdk.RuleParametersExtractParameters{ExtractParameters: &extractParameters}
		ruleParameters.RuleParameters = &ruleParametersExtractParameters
	case "json_extract":
		destinationField := rulesSchemaDestinationFieldToProtoDestinationField[m["destination_field"].(string)]
		rule := wrapperspb.String(m["json_key"].(string))
		jsonExtractParameters := cxsdk.JSONExtractParameters{DestinationFieldType: destinationField, Rule: rule}
		if destinationField == cxsdk.JSONExtractParametersDestinationFieldText {
			jsonExtractParameters.DestinationFieldText = wrapperspb.String(m["destination_field_text"].(string))
		}
		ruleParametersJsonExtractParameters := cxsdk.RuleParametersJSONExtractParameters{JsonExtractParameters: &jsonExtractParameters}
		ruleParameters.RuleParameters = &ruleParametersJsonExtractParameters
	case "replace":
		destinationField := wrapperspb.String(m["destination_field"].(string))
		replaceNewVal := wrapperspb.String(m["replacement_string"].(string))
		rule := wrapperspb.String(m["regular_expression"].(string))
		replaceParameters := cxsdk.ReplaceParameters{DestinationField: destinationField, ReplaceNewVal: replaceNewVal, Rule: rule}
		ruleParametersReplaceParameters := cxsdk.RuleParametersReplaceParameters{ReplaceParameters: &replaceParameters}
		ruleParameters.RuleParameters = &ruleParametersReplaceParameters
	case "block":
		keepBlockedLogs := wrapperspb.Bool(m["keep_blocked_logs"].(bool))
		rule := wrapperspb.String(m["regular_expression"].(string))
		if m["blocking_all_matching_blocks"].(bool) {
			blockParameters := cxsdk.BlockParameters{KeepBlockedLogs: keepBlockedLogs, Rule: rule}
			ruleParametersBlockParameters := cxsdk.RuleParametersBlockParameters{BlockParameters: &blockParameters}
			ruleParameters.RuleParameters = &ruleParametersBlockParameters
		} else {
			allowParameters := cxsdk.AllowParameters{KeepBlockedLogs: keepBlockedLogs, Rule: rule}
			ruleParametersAllowParameters := cxsdk.RuleParametersAllowParameters{AllowParameters: &allowParameters}
			ruleParameters.RuleParameters = &ruleParametersAllowParameters
		}
	case "extract_timestamp":
		standard := expandFieldFormatStandard(m["field_format_standard"].(string))
		format := wrapperspb.String(m["time_format"].(string))
		extractTimestampParameters := cxsdk.ExtractTimestampParameters{Format: format, Standard: standard}
		ruleParametersExtractTimestampParameters := cxsdk.RuleParametersExtractTimestampParameters{ExtractTimestampParameters: &extractTimestampParameters}
		ruleParameters.RuleParameters = &ruleParametersExtractTimestampParameters
	case "remove_fields":
		excludedFields := interfaceSliceToStringSlice(m["excluded_fields"].([]interface{}))
		removeFieldsParameters := cxsdk.RemoveFieldsParameters{Fields: excludedFields}
		ruleParametersRemoveFieldsParameters := cxsdk.RuleParametersRemoveFieldsParameters{RemoveFieldsParameters: &removeFieldsParameters}
		ruleParameters.RuleParameters = &ruleParametersRemoveFieldsParameters
	case "json_stringify":
		destinationField := wrapperspb.String(m["destination_field"].(string))
		deleteSource := wrapperspb.Bool(!m["keep_source_field"].(bool))
		jsonStringifyParameters := cxsdk.JSONStringifyParameters{DestinationField: destinationField, DeleteSource: deleteSource}
		ruleParametersJsonStringifyParameters := cxsdk.RuleParametersJSONStringifyParameters{JsonStringifyParameters: &jsonStringifyParameters}
		ruleParameters.RuleParameters = &ruleParametersJsonStringifyParameters
	case "parse_json_field":
		destinationField := wrapperspb.String(m["destination_field"].(string))
		deleteSource := wrapperspb.Bool(!m["keep_source_field"].(bool))
		overrideDest := wrapperspb.Bool(!m["keep_destination_field"].(bool))
		escapedValue := wrapperspb.Bool(true)
		jsonParseParameters := cxsdk.JSONParseParameters{
			DestinationField: destinationField,
			DeleteSource:     deleteSource,
			EscapedValue:     escapedValue,
			OverrideDest:     overrideDest,
		}
		ruleParametersJsonStringifyParameters := cxsdk.RuleParametersJSONParseParameters{JsonParseParameters: &jsonParseParameters}
		ruleParameters.RuleParameters = &ruleParametersJsonStringifyParameters
	default:
		panic(ruleType)
	}

	return &ruleParameters
}

func expandFieldFormatStandard(formatStandard string) cxsdk.ExtractTimestampParametersFormatStandard {
	formatStandardVal := rulesSchemaFormatStandardToProtoFormatStandard[strings.ToLower(formatStandard)]
	return cxsdk.ExtractTimestampParametersFormatStandard(formatStandardVal)
}

func flattenRuleMatcher(ruleMatchers []*cxsdk.RuleMatcher) (map[string][]string, error) {
	ruleMatcherMap := map[string][]string{"applications": {}, "subsystems": {}, "severities": {}}
	for _, ruleMatcher := range ruleMatchers {
		switch ruleMatcher.Constraint.(type) {
		case *cxsdk.RuleMatcherApplicationName:
			ruleMatcherMap["applications"] = append(ruleMatcherMap["applications"], ruleMatcher.GetApplicationName().
				GetValue().GetValue())
		case *cxsdk.RuleMatcherSubsystemName:
			ruleMatcherMap["subsystems"] = append(ruleMatcherMap["subsystems"], ruleMatcher.GetSubsystemName().
				GetValue().GetValue())
		case *cxsdk.RuleMatcherSeverity:
			severityStr := ruleMatcher.GetSeverity().GetValue()
			ruleMatcherMap["severities"] = append(ruleMatcherMap["severities"], rulesProtoSeverityToSchemaSeverity[severityStr])
		default:
			return nil, fmt.Errorf("unexpected type %T for rule matcher", ruleMatcher)
		}
	}
	return ruleMatcherMap, nil
}

func flattenRuleSubgroups(ruleSubgroups []*cxsdk.RuleSubgroup) ([]interface{}, error) {
	result := make([]interface{}, 0, len(ruleSubgroups))
	for _, ruleSubgroup := range ruleSubgroups {
		if rsg, err := flattenRuleGroup(ruleSubgroup); err != nil {
			return nil, err
		} else {
			result = append(result, rsg)
		}
	}
	return result, nil
}

func flattenRuleGroup(ruleSubgroup *cxsdk.RuleSubgroup) (map[string]interface{}, error) {
	rules, err := flattenRules(ruleSubgroup)
	if err != nil {
		return nil, err
	}

	rsg := map[string]interface{}{
		"id":     ruleSubgroup.GetId().GetValue(),
		"order":  ruleSubgroup.GetOrder().GetValue(),
		"active": ruleSubgroup.GetEnabled().GetValue(),
		"rules":  rules,
	}

	return rsg, nil
}

func flattenRules(ruleSubgroup *cxsdk.RuleSubgroup) ([]interface{}, error) {
	rs := ruleSubgroup.GetRules()
	rules := make([]interface{}, 0, len(rs))
	for _, r := range rs {
		var err error
		rule, err := flattenRule(r)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func flattenRule(r *cxsdk.Rule) (map[string]interface{}, error) {
	rule := flattenCommonRulesParams(r)
	var ruleType string
	ruleParams := r.GetParameters().GetRuleParameters()
	switch ruleParams := ruleParams.(type) {
	case *cxsdk.RuleParametersExtractParameters:
		ruleType = "extract"
		extractParameters := ruleParams.ExtractParameters
		rule["regular_expression"] = extractParameters.GetRule().GetValue()
		rule["source_field"] = r.GetSourceField().GetValue()
	case *cxsdk.RuleParametersJSONExtractParameters:
		ruleType = "json_extract"
		jsonExtractParameters := ruleParams.JsonExtractParameters
		rule["json_key"] = jsonExtractParameters.GetRule().GetValue()
		rule["destination_field"] = rulesProtoDestinationFieldToSchemaDestinationField[jsonExtractParameters.GetDestinationFieldType()]
		if jsonExtractParameters.GetDestinationFieldType() == cxsdk.JSONExtractParametersDestinationFieldText {
			rule["destination_field_text"] = jsonExtractParameters.GetDestinationFieldText().GetValue()
		}
	case *cxsdk.RuleParametersReplaceParameters:
		ruleType = "replace"
		replaceParameters := ruleParams.ReplaceParameters
		rule["source_field"] = r.GetSourceField().GetValue()
		rule["destination_field"] = replaceParameters.GetDestinationField().GetValue()
		rule["regular_expression"] = replaceParameters.GetRule().GetValue()
		rule["replacement_string"] = replaceParameters.GetReplaceNewVal().GetValue()
	case *cxsdk.RuleParametersParseParameters:
		ruleType = "parse"
		parseParameters := ruleParams.ParseParameters
		rule["source_field"] = r.GetSourceField().GetValue()
		rule["destination_field"] = parseParameters.GetDestinationField().GetValue()
		rule["regular_expression"] = parseParameters.GetRule().GetValue()
	case *cxsdk.RuleParametersAllowParameters:
		ruleType = "block"
		allowParameters := ruleParams.AllowParameters
		rule["source_field"] = r.GetSourceField().GetValue()
		rule["regular_expression"] = allowParameters.GetRule().GetValue()
		rule["keep_blocked_logs"] = allowParameters.GetKeepBlockedLogs().GetValue()
		rule["blocking_all_matching_blocks"] = false
	case *cxsdk.RuleParametersBlockParameters:
		ruleType = "block"
		blockParameters := ruleParams.BlockParameters
		rule["source_field"] = r.GetSourceField().GetValue()
		rule["regular_expression"] = blockParameters.GetRule().GetValue()
		rule["keep_blocked_logs"] = blockParameters.GetKeepBlockedLogs().GetValue()
		rule["blocking_all_matching_blocks"] = true
	case *cxsdk.RuleParametersExtractTimestampParameters:
		ruleType = "extract_timestamp"
		extractTimestampParameters := ruleParams.ExtractTimestampParameters
		rule["source_field"] = r.GetSourceField().GetValue()
		rule["time_format"] = extractTimestampParameters.GetFormat().GetValue()
		rule["field_format_standard"] = rulesProtoFormatStandardToSchemaFormatStandard[extractTimestampParameters.GetStandard()]
	case *cxsdk.RuleParametersRemoveFieldsParameters:
		ruleType = "remove_fields"
		removeFieldsParameters := ruleParams.RemoveFieldsParameters
		rule["excluded_fields"] = removeFieldsParameters.GetFields()
	case *cxsdk.RuleParametersJSONStringifyParameters:
		ruleType = "json_stringify"
		jsonStringifyParameters := ruleParams.JsonStringifyParameters
		rule["source_field"] = r.GetSourceField().GetValue()
		rule["destination_field"] = jsonStringifyParameters.GetDestinationField().GetValue()
		rule["keep_source_field"] = !(jsonStringifyParameters.GetDeleteSource().GetValue())
	case *cxsdk.RuleParametersJSONParseParameters:
		ruleType = "parse_json_field"
		jsonParseParameters := ruleParams.JsonParseParameters
		rule["source_field"] = r.GetSourceField().GetValue()
		rule["destination_field"] = jsonParseParameters.GetDestinationField().GetValue()
		rule["keep_source_field"] = !(jsonParseParameters.GetDeleteSource().GetValue())
		rule["keep_destination_field"] = !(jsonParseParameters.GetOverrideDest().GetValue())
	default:
		return nil, fmt.Errorf("unexpected type %T for r parameters", ruleParams)
	}

	return map[string]interface{}{ruleType: []interface{}{rule}}, nil
}

func flattenCommonRulesParams(rule *cxsdk.Rule) map[string]interface{} {
	return map[string]interface{}{
		"id":          rule.GetId().GetValue(),
		"description": rule.GetDescription().GetValue(),
		"name":        rule.GetName().GetValue(),
		"active":      rule.GetEnabled().GetValue(),
		"order":       rule.GetOrder().GetValue(),
	}
}
