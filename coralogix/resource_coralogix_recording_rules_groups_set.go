package coralogix

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"terraform-provider-coralogix/coralogix/clientset"
	rrgs "terraform-provider-coralogix/coralogix/clientset/grpc/recording-rules-groups-sets/v1"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"gopkg.in/yaml.v3"
)

func resourceCoralogixRecordingRulesGroupsSet() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCoralogixRecordingRulesGroupsSetCreate,
		ReadContext:   resourceCoralogixRecordingRulesGroupsSetRead,
		UpdateContext: resourceCoralogixRecordingRulesGroupsSetUpdate,
		DeleteContext: resourceCoralogixRecordingRulesGroupsSetDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(60 * time.Second),
			Read:   schema.DefaultTimeout(30 * time.Second),
			Update: schema.DefaultTimeout(60 * time.Second),
			Delete: schema.DefaultTimeout(30 * time.Second),
		},

		Schema: RecordingRulesGroupsSetSchema(),

		Description: "Coralogix recording-rules-groups-set. For more information - https://coralogix.com/docs/recording-rules/.",
	}
}

func resourceCoralogixRecordingRulesGroupsSetCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	req, err := expandRecordingRulesGroupsSet(d)
	if err != nil {
		return diag.FromErr(err)
	}

	log.Printf("[INFO] Creating new recording-rule-group-set: %#v", req)
	resp, err := meta.(*clientset.ClientSet).RecordingRuleGroupsSets().CreateRecordingRuleGroupsSet(ctx, req)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcError(err, "recording-rule-group-set")
	}
	log.Printf("[INFO] Submitted new recording-rule-group-set: %#v", resp)

	d.SetId(resp.Id)
	return resourceCoralogixRecordingRulesGroupsSetRead(ctx, d, meta)
}

func expandRecordingRulesGroupsSet(d *schema.ResourceData) (*rrgs.CreateRuleGroupSet, error) {
	if yamlContent, ok := d.GetOk("yaml_content"); ok {
		return expandRecordingRulesGroupsSetFromYaml(yamlContent.(string))
	}

	return expandRecordingRulesGroupSetExplicitly(d), nil
}

func expandRecordingRulesGroupsSetFromYaml(yamlContent string) (*rrgs.CreateRuleGroupSet, error) {
	var result rrgs.CreateRuleGroupSet
	if err := yaml.Unmarshal([]byte(yamlContent), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func expandRecordingRulesGroupSetExplicitly(d *schema.ResourceData) *rrgs.CreateRuleGroupSet {
	var name *string
	if str, ok := d.GetOk("name"); ok && str.(string) != "" {
		name = new(string)
		*name = str.(string)
	}
	groups := expandRecordingRulesGroups(d.Get("group"))

	return &rrgs.CreateRuleGroupSet{
		Name:   name,
		Groups: groups,
	}
}

func expandRecordingRulesGroups(v interface{}) []*rrgs.InRuleGroup {
	groups := v.([]interface{})
	results := make([]*rrgs.InRuleGroup, 0, len(groups))
	for _, g := range groups {
		group := expandRecordingRuleGroup(g)
		results = append(results, group)
	}
	return results

}

func expandRecordingRuleGroup(v interface{}) *rrgs.InRuleGroup {
	m := v.(map[string]interface{})

	name := m["name"].(string)
	interval := uint32(m["interval"].(int))
	limit := uint64(m["limit"].(int))
	rules := expandRecordingRules(m["rule"])

	return &rrgs.InRuleGroup{
		Name:     name,
		Interval: &interval,
		Limit:    &limit,
		Rules:    rules,
	}
}

func expandRecordingRules(v interface{}) []*rrgs.InRule {
	l := v.(*schema.Set).List()
	result := make([]*rrgs.InRule, 0, len(l))
	for _, recordingRule := range l {
		r := expandRecordingRule(recordingRule)
		result = append(result, r)
	}
	return result
}

func expandRecordingRule(v interface{}) *rrgs.InRule {
	m := v.(map[string]interface{})

	record := m["record"].(string)
	expr := m["expr"].(string)
	labels := expandRecordingRuleLabels(m["labels"].(map[string]interface{}))

	return &rrgs.InRule{
		Record: record,
		Expr:   expr,
		Labels: labels,
	}
}

func expandRecordingRuleLabels(m map[string]interface{}) map[string]string {
	form := make(map[string]string)
	for k, v := range m {
		form[k] = v.(string)
	}
	return form
}

func resourceCoralogixRecordingRulesGroupsSetRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	log.Printf("[INFO] Reading recording-rule-group-set %s", id)
	req := &rrgs.FetchRuleGroupSet{
		Id: id,
	}
	resp, err := meta.(*clientset.ClientSet).RecordingRuleGroupsSets().GetRecordingRuleGroupsSet(ctx, req)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if status.Code(err) == codes.NotFound {
			d.SetId("")
			return diag.Diagnostics{diag.Diagnostic{
				Severity: diag.Warning,
				Summary:  fmt.Sprintf("RecordingRuleGroup %q is in state, but no longer exists in Coralogix backend", id),
				Detail:   fmt.Sprintf("%s will be recreated when you apply", id),
			}}
		}
		return handleRpcErrorWithID(err, "recording-rule-group-set", req.Id)
	}

	log.Printf("[INFO] Received recording-rule-group-set: %#v", resp)
	setRecordingRulesGroupsSet(d, resp)
	return nil
}

func resourceCoralogixRecordingRulesGroupsSetUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	createReq, err := expandRecordingRulesGroupsSet(d)
	if err != nil {
		return diag.FromErr(err)
	}
	updateReq := &rrgs.UpdateRuleGroupSet{
		Id:     d.Id(),
		Groups: createReq.Groups,
	}

	log.Printf("[INFO] Updating recording-rule-group-set: %#v", updateReq)
	resp, err := meta.(*clientset.ClientSet).RecordingRuleGroupsSets().UpdateRecordingRuleGroupsSet(ctx, updateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "recording-rule-group-set", updateReq.Id)
	}
	log.Printf("[INFO] Submitted updated recording-rule-group-set: %#v", resp)

	return resourceCoralogixRecordingRulesGroupsSetRead(ctx, d, meta)
}

func resourceCoralogixRecordingRulesGroupsSetDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	req := &rrgs.DeleteRuleGroupSet{Id: id}
	log.Printf("[INFO] Deleting recording-rule-group-set %s", id)
	_, err := meta.(*clientset.ClientSet).RecordingRuleGroupsSets().DeleteRecordingRuleGroupsSet(ctx, req)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "recording-rule-group-set", req.Id)
	}
	log.Printf("[INFO] recording-rule-group-set %s deleted", id)

	d.SetId("")
	return nil
}

func setRecordingRulesGroupsSet(d *schema.ResourceData, set *rrgs.OutRuleGroupSet) diag.Diagnostics {
	if err := d.Set("group", flattenRecordingRulesGroups(set.Groups)); err != nil {
		return diag.FromErr(err)
	}

	if name := set.Name; name != nil {
		if err := d.Set("name", *name); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func flattenRecordingRulesGroups(groups []*rrgs.OutRuleGroup) interface{} {
	result := make([]interface{}, 0, len(groups))
	for _, g := range groups {
		group := flattenRecordingRulesGroup(g)
		result = append(result, group)
	}
	return result
}

func flattenRecordingRulesGroup(group *rrgs.OutRuleGroup) interface{} {
	rules := flattenRecordingRules(group.Rules)
	return []interface{}{
		map[string]interface{}{
			"name":     group.Name,
			"interval": group.Interval,
			"limit":    group.Limit,
			"rule":     rules,
		},
	}
}

func flattenRecordingRules(rules []*rrgs.OutRule) interface{} {
	result := make([]interface{}, 0, len(rules))
	for _, rule := range rules {
		flattenedRecordingRule := flattenRecordingRule(rule)
		result = append(result, flattenedRecordingRule)
	}
	return result
}

func flattenRecordingRule(rule *rrgs.OutRule) interface{} {
	labels := flattenRecordingRuleLabels(rule.Labels)
	return map[string]interface{}{
		"record": rule.Record,
		"expr":   rule.Expr,
		"labels": labels,
	}
}

func flattenRecordingRuleLabels(labels map[string]string) map[string]interface{} {
	form := make(map[string]interface{}, len(labels))
	for k, v := range labels {
		form[k] = v
	}
	return form
}

func RecordingRulesGroupsSetSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"yaml_content": {
			Type:         schema.TypeString,
			Optional:     true,
			ExactlyOneOf: []string{"yaml_content", "group"},
			ValidateFunc: validateRecordingRulesGroupYamlContent,
			Description:  "An option to import recording-rule-group-set from yaml file.",
		},
		"group": {
			Type:         schema.TypeList,
			Optional:     true,
			Computed:     true,
			ExactlyOneOf: []string{"yaml_content", "group"},
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"name": {
						Type:         schema.TypeString,
						Required:     true,
						ValidateFunc: validation.StringIsNotEmpty,
						Description:  "The rule-group name. Have to be unique.",
					},
					"interval": {
						Type:         schema.TypeInt,
						Required:     true,
						ValidateFunc: validation.IntAtLeast(0),
						Description:  "How often rules in the group are evaluated (in seconds).",
					},
					"limit": {
						Type:        schema.TypeInt,
						Optional:    true,
						Description: "Limit the number of alerts an alerting rule and series a recording-rule can produce. 0 is no limit.",
					},
					"rule": {
						Type:     schema.TypeSet,
						Required: true,
						Elem:     recordingRulesSchema(),
						Set:      schema.HashResource(recordingRulesSchema()),
					},
				},
			},
			Description: "An option to define recording-rule-groups explicitly. Will be computed in a case of importing by yaml_content.",
		},
		"name": {
			Type:          schema.TypeString,
			Optional:      true,
			Computed:      true,
			ConflictsWith: []string{"yaml_content"},
			Description:   "recording-rule-groups-set name. Optional in a case of defining the recording-rule-groups ('group') explicitly, and computed in a case of importing by yaml_content",
		},
	}
}

func validateRecordingRulesGroupYamlContent(config interface{}, _ string) ([]string, []error) {
	var set rrgs.CreateRuleGroupSet
	if err := yaml.Unmarshal([]byte(config.(string)), &set); err != nil {
		return nil, []error{err}
	}

	groups := set.Groups
	if len(groups) == 0 {
		return nil, []error{fmt.Errorf("groups list can not be empty")}
	}

	errors := make([]error, 0)
	for i, group := range groups {
		if group.Name == "" {
			errors = append(errors, fmt.Errorf("groups[%d] name can not be empty", i))
		}
		if group.Interval == nil {
			return nil, append(errors, fmt.Errorf("groups[%d] limit have to be set", i))
		}
	}
	if len(errors) != 0 {
		return nil, errors
	}

	return nil, nil
}

func recordingRulesSchema() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"record": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the time series to output to. Must be a valid metric name.",
			},
			"expr": {
				Type:     schema.TypeString,
				Required: true,
				Description: "The PromQL expression to evaluate. " +
					"Every evaluation cycle this is evaluated at the current time," +
					" and the result recorded as a new set of time series with the metric name as given by 'record'.",
			},
			"labels": {
				Type: schema.TypeMap,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional:    true,
				Description: "Labels to add or overwrite before storing the result.",
			},
		},
	}
}
