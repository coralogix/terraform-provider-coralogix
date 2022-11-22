package coralogix

import (
	"context"
	"fmt"
	"log"
	"time"

	"terraform-provider-coralogix/coralogix/clientset"
	rulesv1 "terraform-provider-coralogix/coralogix/clientset/grpc/com/coralogix/rules/v1"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"golang.org/x/exp/maps"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	rulesSchemaSeverityToProtoSeverity = map[string]string{
		"Debug":    "VALUE_DEBUG_OR_UNSPECIFIED",
		"Verbose":  "VALUE_VERBOSE",
		"Info":     "VALUE_INFO",
		"Warning":  "VALUE_WARNING",
		"Error":    "VALUE_ERROR",
		"Critical": "VALUE_CRITICAL",
	}
	rulesProtoSeverityToSchemaSeverity                 = reverseMap(rulesSchemaSeverityToProtoSeverity)
	rulesValidSeverities                               = maps.Keys(rulesSchemaSeverityToProtoSeverity)
	rulesSchemaDestinationFieldToProtoDestinationField = map[string]string{
		"Category": "DESTINATION_FIELD_CATEGORY_OR_UNSPECIFIED",
		"Class":    "DESTINATION_FIELD_CLASSNAME",
		"Method":   "DESTINATION_FIELD_METHODNAME",
		"ThreadID": "DESTINATION_FIELD_THREADID",
		"Severity": "DESTINATION_FIELD_SEVERITY",
	}
	rulesProtoDestinationFieldToSchemaDestinationField = reverseMap(rulesSchemaDestinationFieldToProtoDestinationField)
	rulesValidDestinationFields                        = maps.Keys(rulesSchemaDestinationFieldToProtoDestinationField)
	rulesSchemaFormatStandardToProtoFormatStandard     = map[string]string{
		"Strftime": "FORMAT_STANDARD_STRFTIME_OR_UNSPECIFIED",
		"JavaSDF":  "FORMAT_STANDARD_JAVASDF",
		"Golang":   "FORMAT_STANDARD_GOLANG",
		"SecondTS": "FORMAT_STANDARD_SECONDSTS",
		"MilliTS":  "FORMAT_STANDARD_MILLITS",
		"MicroTS":  "FORMAT_STANDARD_MICROTS",
		"NanoTS":   "FORMAT_STANDARD_NANOTS",
	}
	rulesProtoFormatStandardToSchemaFormatStandard = reverseMap(rulesSchemaFormatStandardToProtoFormatStandard)
	rulesValidFormatStandards                      = maps.Keys(rulesSchemaFormatStandardToProtoFormatStandard)
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
			Create: schema.DefaultTimeout(60 * time.Minute),
			Read:   schema.DefaultTimeout(30 * time.Minute),
			Update: schema.DefaultTimeout(60 * time.Minute),
			Delete: schema.DefaultTimeout(30 * time.Minute),
		},

		Schema: RulesGroupSchema(),

		Description: "Rule-group is list of rule-subgroups with 'and' (&&) operation between. Api-key is required for this resource.",
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
			Type:        schema.TypeInt,
			Computed:    true,
			Description: "Determines the index of the rule-group between the other rule-groups. By default will be added last.",
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
						Computed: true,
						Description: "Determines the index of the rule-subgroup inside the rule-group." +
							"Will be computed by the order it was declarer.",
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
									Description: "Use a named RegEx group to extract specific values you need as JSON keys without having to parse the entire log.",
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
		Description:  fmt.Sprintf("The format standard you want to use. Can be one of %q", rulesValidDestinationFields),
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
		Default:     true,
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

func commonRulesSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"id": {
			Type:     schema.TypeString,
			Computed: true,
		},
		"name": {
			Type:         schema.TypeString,
			Required:     true,
			ValidateFunc: validation.StringIsNotEmpty,
		},
		"description": {
			Type:     schema.TypeString,
			Optional: true,
			Default:  "",
		},
		"active": {
			Type:     schema.TypeBool,
			Optional: true,
			Default:  true,
		},
		"order": {
			Type:     schema.TypeInt,
			Computed: true,
		},
	}
}

func appendSourceFieldSchema(m map[string]*schema.Schema) map[string]*schema.Schema {
	m["source_field"] = &schema.Schema{
		Type:        schema.TypeString,
		Required:    true,
		Description: "The field on which the Regex will operate on.",
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
		Type:         schema.TypeString,
		Required:     true,
		ValidateFunc: validation.StringIsValidRegExp,
		Description:  "Regular expiration. More info: https://coralogix.com/blog/regex-101/",
	}
	return m
}

func resourceCoralogixRulesGroupCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	createRuleGroupRequest, err := extractCreateRuleGroupRequest(d)
	if err != nil {
		return diag.FromErr(err)
	}

	log.Printf("[INFO] Creating new rule-group: %#v", createRuleGroupRequest)
	ruleGroupResp, err := meta.(*clientset.ClientSet).RuleGroups().CreateRuleGroup(ctx, createRuleGroupRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcError(err)
	}
	ruleGroup := ruleGroupResp.GetRuleGroup()
	log.Printf("[INFO] Submitted new rule-group: %#v", ruleGroup)
	d.SetId(ruleGroup.GetId().GetValue())

	return resourceCoralogixRulesGroupRead(ctx, d, meta)
}

func resourceCoralogixRulesGroupRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	getRuleGroupRequest := &rulesv1.GetRuleGroupRequest{
		GroupId: id,
	}

	log.Printf("[INFO] Reading rule-group %s", id)
	ruleGroupResp, err := meta.(*clientset.ClientSet).RuleGroups().GetRuleGroup(ctx, getRuleGroupRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "rule-group", id)
	}
	ruleGroup := ruleGroupResp.GetRuleGroup()
	log.Printf("[INFO] Received rule-group: %#v", ruleGroup)

	return setRuleGroup(d, ruleGroup)
}

func resourceCoralogixRulesGroupUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	req, err := extractCreateRuleGroupRequest(d)
	if err != nil {
		return diag.FromErr(err)
	}

	id := d.Id()
	updateRuleGroupRequest := &rulesv1.UpdateRuleGroupRequest{
		GroupId:   wrapperspb.String(id),
		RuleGroup: req,
	}

	log.Printf("[INFO] Updating rule-group %s", updateRuleGroupRequest)
	ruleGroupResp, err := meta.(*clientset.ClientSet).RuleGroups().UpdateRuleGroup(ctx, updateRuleGroupRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "rule-group", id)
	}
	log.Printf("[INFO] Submitted updated rule-group: %#v", ruleGroupResp)

	return resourceCoralogixRulesGroupRead(ctx, d, meta)
}

func resourceCoralogixRulesGroupDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	deleteRuleGroupRequest := &rulesv1.DeleteRuleGroupRequest{
		GroupId: id,
	}

	log.Printf("[INFO] Deleting rule-group %s", id)
	_, err := meta.(*clientset.ClientSet).RuleGroups().DeleteRuleGroup(ctx, deleteRuleGroupRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "rule-group", id)
	}
	log.Printf("[INFO] rule-group %s deleted", id)

	d.SetId("")
	return nil
}

func extractCreateRuleGroupRequest(d *schema.ResourceData) (*rulesv1.CreateRuleGroupRequest, error) {
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
	createRuleGroupRequest := &rulesv1.CreateRuleGroupRequest{
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

func setRuleGroup(d *schema.ResourceData, ruleGroup *rulesv1.RuleGroup) diag.Diagnostics {
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

func expandRuleMatcher(d *schema.ResourceData) []*rulesv1.RuleMatcher {
	applications := d.Get("applications").(*schema.Set).List()
	subsystems := d.Get("subsystems").(*schema.Set).List()
	severities := d.Get("severities").(*schema.Set).List()
	ruleMatchers := make([]*rulesv1.RuleMatcher, 0, len(applications)+len(subsystems)+len(severities))

	for _, app := range applications {
		constraintStr := wrapperspb.String(app.(string))
		applicationNameConstraint := rulesv1.ApplicationNameConstraint{Value: constraintStr}
		ruleMatcherApplicationName := rulesv1.RuleMatcher_ApplicationName{ApplicationName: &applicationNameConstraint}
		ruleMatchers = append(ruleMatchers, &rulesv1.RuleMatcher{Constraint: &ruleMatcherApplicationName})
	}

	for _, subSys := range subsystems {
		constraintStr := wrapperspb.String(subSys.(string))
		subsystemNameConstraint := rulesv1.SubsystemNameConstraint{Value: constraintStr}
		ruleMatcherApplicationName := rulesv1.RuleMatcher_SubsystemName{SubsystemName: &subsystemNameConstraint}
		ruleMatchers = append(ruleMatchers, &rulesv1.RuleMatcher{Constraint: &ruleMatcherApplicationName})
	}

	for _, sev := range severities {
		constraintEnum := expandRuledSeverity(sev.(string))
		severityConstraint := rulesv1.SeverityConstraint{Value: constraintEnum}
		ruleMatcherSeverity := rulesv1.RuleMatcher_Severity{Severity: &severityConstraint}
		ruleMatchers = append(ruleMatchers, &rulesv1.RuleMatcher{Constraint: &ruleMatcherSeverity})
	}

	return ruleMatchers
}

func expandRuledSeverity(severity string) rulesv1.SeverityConstraint_Value {
	sevStr := rulesSchemaSeverityToProtoSeverity[severity]
	return rulesv1.SeverityConstraint_Value(rulesv1.SeverityConstraint_Value_value[sevStr])
}

func expandRuleSubgroups(v interface{}) ([]*rulesv1.CreateRuleGroupRequest_CreateRuleSubgroup, error) {
	s := v.([]interface{})
	ruleSubgroups := make([]*rulesv1.CreateRuleGroupRequest_CreateRuleSubgroup, 0, len(s))
	for i, o := range s {
		m := o.(map[string]interface{})
		rsg, err := expandRuleSubgroup(m)
		if err != nil {
			return nil, err
		}
		rsg.Order = wrapperspb.UInt32(uint32(i + 1))
		ruleSubgroups = append(ruleSubgroups, rsg)
	}

	return ruleSubgroups, nil
}

func expandRuleSubgroup(m map[string]interface{}) (*rulesv1.CreateRuleGroupRequest_CreateRuleSubgroup, error) {
	rules, err := expandRules(m["rules"].([]interface{}))
	if err != nil {
		return nil, err
	}
	return &rulesv1.CreateRuleGroupRequest_CreateRuleSubgroup{
		Rules:   rules,
		Enabled: wrapperspb.Bool(m["active"].(bool)),
	}, nil
}

func expandRules(s []interface{}) ([]*rulesv1.CreateRuleGroupRequest_CreateRuleSubgroup_CreateRule, error) {
	rules := make([]*rulesv1.CreateRuleGroupRequest_CreateRuleSubgroup_CreateRule, 0)
	for i, v := range s {
		rule, err := expandRule(v)
		if err != nil {
			return nil, err
		}
		rule.Order = wrapperspb.UInt32(uint32(i + 1))
		rules = append(rules, rule)
	}
	return rules, nil
}

func expandRule(i interface{}) (*rulesv1.CreateRuleGroupRequest_CreateRuleSubgroup_CreateRule, error) {
	m := i.(map[string]interface{})
	var rule *rulesv1.CreateRuleGroupRequest_CreateRuleSubgroup_CreateRule
	for k, v := range m {
		if r, ok := v.([]interface{}); ok && len(r) > 0 {
			if rule == nil {
				rule = expandRuleForSpecificRuleType(k, r[0])
			} else {
				return nil, fmt.Errorf("exactly one of %q must be provided inside rule. more than one rule type where provided.", maps.Keys(m))
			}
		}
	}
	if rule == nil {
		return nil, fmt.Errorf("exactly one of %q must be provided inside rule. no rule type was provided.", maps.Keys(m))
	}
	return rule, nil
}

func expandRuleForSpecificRuleType(rulesType string, i interface{}) *rulesv1.CreateRuleGroupRequest_CreateRuleSubgroup_CreateRule {
	m := i.(map[string]interface{})
	return &rulesv1.CreateRuleGroupRequest_CreateRuleSubgroup_CreateRule{
		Name:        wrapperspb.String(m["name"].(string)),
		Description: wrapperspb.String(m["description"].(string)),
		SourceField: func() *wrapperspb.StringValue {
			if sourceFieldObj, ok := m["source_field"]; ok {
				return wrapperspb.String(sourceFieldObj.(string))
			}
			return wrapperspb.String("text")
		}(),
		Enabled:    wrapperspb.Bool(m["active"].(bool)),
		Order:      wrapperspb.UInt32(uint32(m["order"].(int))),
		Parameters: expandParameters(rulesType, m),
	}
}

func expandParameters(ruleType string, m map[string]interface{}) *rulesv1.RuleParameters {
	var ruleParameters rulesv1.RuleParameters

	switch ruleType {
	case "parse":
		destinationField := wrapperspb.String(m["destination_field"].(string))
		rule := wrapperspb.String(m["regular_expression"].(string))
		parseParameters := rulesv1.ParseParameters{DestinationField: destinationField, Rule: rule}
		ruleParametersParseParameters := rulesv1.RuleParameters_ParseParameters{ParseParameters: &parseParameters}
		ruleParameters.RuleParameters = &ruleParametersParseParameters
	case "extract":
		rule := wrapperspb.String(m["regular_expression"].(string))
		extractParameters := rulesv1.ExtractParameters{Rule: rule}
		ruleParametersExtractParameters := rulesv1.RuleParameters_ExtractParameters{ExtractParameters: &extractParameters}
		ruleParameters.RuleParameters = &ruleParametersExtractParameters
	case "json_extract":
		destinationField := expandDestinationField(m["destination_field"].(string))
		rule := wrapperspb.String(m["json_key"].(string))
		jsonExtractParameters := rulesv1.JsonExtractParameters{DestinationField: destinationField, Rule: rule}
		ruleParametersJsonExtractParameters := rulesv1.RuleParameters_JsonExtractParameters{JsonExtractParameters: &jsonExtractParameters}
		ruleParameters.RuleParameters = &ruleParametersJsonExtractParameters
	case "replace":
		destinationField := wrapperspb.String(m["destination_field"].(string))
		replaceNewVal := wrapperspb.String(m["replacement_string"].(string))
		rule := wrapperspb.String(m["regular_expression"].(string))
		replaceParameters := rulesv1.ReplaceParameters{DestinationField: destinationField, ReplaceNewVal: replaceNewVal, Rule: rule}
		ruleParametersReplaceParameters := rulesv1.RuleParameters_ReplaceParameters{ReplaceParameters: &replaceParameters}
		ruleParameters.RuleParameters = &ruleParametersReplaceParameters
	case "block":
		keepBlockedLogs := wrapperspb.Bool(m["keep_blocked_logs"].(bool))
		rule := wrapperspb.String(m["regular_expression"].(string))
		if m["blocking_all_matching_blocks"].(bool) {
			blockParameters := rulesv1.BlockParameters{KeepBlockedLogs: keepBlockedLogs, Rule: rule}
			ruleParametersBlockParameters := rulesv1.RuleParameters_BlockParameters{BlockParameters: &blockParameters}
			ruleParameters.RuleParameters = &ruleParametersBlockParameters
		} else {
			allowParameters := rulesv1.AllowParameters{KeepBlockedLogs: keepBlockedLogs, Rule: rule}
			ruleParametersAllowParameters := rulesv1.RuleParameters_AllowParameters{AllowParameters: &allowParameters}
			ruleParameters.RuleParameters = &ruleParametersAllowParameters
		}
	case "extract_timestamp":
		standard := expandFieldFormatStandard(m["field_format_standard"].(string))
		format := wrapperspb.String(m["time_format"].(string))
		extractTimestampParameters := rulesv1.ExtractTimestampParameters{Format: format, Standard: standard}
		ruleParametersExtractTimestampParameters := rulesv1.RuleParameters_ExtractTimestampParameters{ExtractTimestampParameters: &extractTimestampParameters}
		ruleParameters.RuleParameters = &ruleParametersExtractTimestampParameters
	case "remove_fields":
		excludedFields := interfaceSliceToStringSlice(m["excluded_fields"].([]interface{}))
		removeFieldsParameters := rulesv1.RemoveFieldsParameters{Fields: excludedFields}
		ruleParametersRemoveFieldsParameters := rulesv1.RuleParameters_RemoveFieldsParameters{RemoveFieldsParameters: &removeFieldsParameters}
		ruleParameters.RuleParameters = &ruleParametersRemoveFieldsParameters
	case "json_stringify":
		destinationField := wrapperspb.String(m["destination_field"].(string))
		deleteSource := wrapperspb.Bool(!m["keep_source_field"].(bool))
		jsonStringifyParameters := rulesv1.JsonStringifyParameters{DestinationField: destinationField, DeleteSource: deleteSource}
		ruleParametersJsonStringifyParameters := rulesv1.RuleParameters_JsonStringifyParameters{JsonStringifyParameters: &jsonStringifyParameters}
		ruleParameters.RuleParameters = &ruleParametersJsonStringifyParameters
	default:
		panic(ruleType)
	}

	return &ruleParameters
}

func expandDestinationField(destinationField string) rulesv1.JsonExtractParameters_DestinationField {
	destinationFieldStr := rulesSchemaDestinationFieldToProtoDestinationField[destinationField]
	destinationFieldVal := rulesv1.JsonExtractParameters_DestinationField_value[destinationFieldStr]
	return rulesv1.JsonExtractParameters_DestinationField(destinationFieldVal)
}

func expandFieldFormatStandard(formatStandard string) rulesv1.ExtractTimestampParameters_FormatStandard {
	formatStandardStr := rulesSchemaFormatStandardToProtoFormatStandard[formatStandard]
	formatStandardVal := rulesv1.ExtractTimestampParameters_FormatStandard_value[formatStandardStr]
	return rulesv1.ExtractTimestampParameters_FormatStandard(formatStandardVal)
}

func flattenRuleMatcher(ruleMatchers []*rulesv1.RuleMatcher) (map[string][]string, error) {
	ruleMatcherMap := map[string][]string{"applications": {}, "subsystems": {}, "severities": {}}
	for _, ruleMatcher := range ruleMatchers {
		switch ruleMatcher.Constraint.(type) {
		case *rulesv1.RuleMatcher_ApplicationName:
			ruleMatcherMap["applications"] = append(ruleMatcherMap["applications"], ruleMatcher.GetApplicationName().
				GetValue().GetValue())
		case *rulesv1.RuleMatcher_SubsystemName:
			ruleMatcherMap["subsystems"] = append(ruleMatcherMap["subsystems"], ruleMatcher.GetSubsystemName().
				GetValue().GetValue())
		case *rulesv1.RuleMatcher_Severity:
			severityStr := ruleMatcher.GetSeverity().GetValue().String()
			ruleMatcherMap["severities"] = append(ruleMatcherMap["severities"], rulesProtoSeverityToSchemaSeverity[severityStr])
		default:
			return nil, fmt.Errorf("unexpected type %T for rule matcher", ruleMatcher)
		}
	}
	return ruleMatcherMap, nil
}

func flattenRuleSubgroups(ruleSubgroups []*rulesv1.RuleSubgroup) ([]interface{}, error) {
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

func flattenRuleGroup(ruleSubgroup *rulesv1.RuleSubgroup) (map[string]interface{}, error) {
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

func flattenRules(ruleSubgroup *rulesv1.RuleSubgroup) ([]interface{}, error) {
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

func flattenRule(r *rulesv1.Rule) (map[string]interface{}, error) {
	rule := flattenCommonRulesParams(r)
	var ruleType string
	ruleParams := r.GetParameters().GetRuleParameters()
	switch ruleParams := ruleParams.(type) {
	case *rulesv1.RuleParameters_ExtractParameters:
		ruleType = "extract"
		extractParameters := ruleParams.ExtractParameters
		rule["regular_expression"] = extractParameters.GetRule().GetValue()
		rule["source_field"] = r.GetSourceField().GetValue()
	case *rulesv1.RuleParameters_JsonExtractParameters:
		ruleType = "json_extract"
		jsonExtractParameters := ruleParams.JsonExtractParameters
		rule["json_key"] = jsonExtractParameters.GetRule().GetValue()
		rule["destination_field"] = rulesProtoDestinationFieldToSchemaDestinationField[jsonExtractParameters.GetDestinationField().String()]
	case *rulesv1.RuleParameters_ReplaceParameters:
		ruleType = "replace"
		replaceParameters := ruleParams.ReplaceParameters
		rule["source_field"] = r.GetSourceField().GetValue()
		rule["destination_field"] = replaceParameters.GetDestinationField().GetValue()
		rule["regular_expression"] = replaceParameters.GetRule().GetValue()
		rule["replacement_string"] = replaceParameters.GetReplaceNewVal().GetValue()
	case *rulesv1.RuleParameters_ParseParameters:
		ruleType = "parse"
		parseParameters := ruleParams.ParseParameters
		rule["source_field"] = r.GetSourceField().GetValue()
		rule["destination_field"] = parseParameters.GetDestinationField().GetValue()
		rule["regular_expression"] = parseParameters.GetRule().GetValue()
	case *rulesv1.RuleParameters_AllowParameters:
		ruleType = "block"
		allowParameters := ruleParams.AllowParameters
		rule["source_field"] = r.GetSourceField().GetValue()
		rule["regular_expression"] = allowParameters.GetRule().GetValue()
		rule["keep_blocked_logs"] = allowParameters.GetKeepBlockedLogs().GetValue()
		rule["blocking_all_matching_blocks"] = false
	case *rulesv1.RuleParameters_BlockParameters:
		ruleType = "block"
		blockParameters := ruleParams.BlockParameters
		rule["source_field"] = r.GetSourceField().GetValue()
		rule["regular_expression"] = blockParameters.GetRule().GetValue()
		rule["keep_blocked_logs"] = blockParameters.GetKeepBlockedLogs().GetValue()
		rule["blocking_all_matching_blocks"] = true
	case *rulesv1.RuleParameters_ExtractTimestampParameters:
		ruleType = "extract_timestamp"
		extractTimestampParameters := ruleParams.ExtractTimestampParameters
		rule["source_field"] = r.GetSourceField().GetValue()
		rule["time_format"] = extractTimestampParameters.GetFormat().GetValue()
		rule["field_format_standard"] = rulesProtoFormatStandardToSchemaFormatStandard[extractTimestampParameters.GetStandard().String()]
	case *rulesv1.RuleParameters_RemoveFieldsParameters:
		ruleType = "remove_fields"
		removeFieldsParameters := ruleParams.RemoveFieldsParameters
		rule["excluded_fields"] = removeFieldsParameters.GetFields()
	case *rulesv1.RuleParameters_JsonStringifyParameters:
		ruleType = "json_stringify"
		jsonStringifyParameters := ruleParams.JsonStringifyParameters
		rule["source_field"] = r.GetSourceField().GetValue()
		rule["destination_field"] = jsonStringifyParameters.GetDestinationField().GetValue()
		rule["keep_source_field"] = !(jsonStringifyParameters.GetDeleteSource().GetValue())
	default:
		return nil, fmt.Errorf("unexpected type %T for r parameters", ruleParams)
	}

	return map[string]interface{}{ruleType: []interface{}{rule}}, nil
}

func flattenCommonRulesParams(rule *rulesv1.Rule) map[string]interface{} {
	return map[string]interface{}{
		"id":          rule.GetId().GetValue(),
		"description": rule.GetDescription().GetValue(),
		"name":        rule.GetName().GetValue(),
		"active":      rule.GetEnabled().GetValue(),
		"order":       rule.GetOrder().GetValue(),
	}
}
