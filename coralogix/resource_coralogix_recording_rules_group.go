package coralogix

import (
	"context"
	"fmt"
	"log"
	"time"

	"terraform-provider-coralogix/coralogix/clientset"
	recordingrules "terraform-provider-coralogix/coralogix/clientset/grpc/recording-rules-groups/v1"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"gopkg.in/yaml.v3"
)

func resourceCoralogixRecordingRulesGroup() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCoralogixRecordingRulesGroupCreate,
		ReadContext:   resourceCoralogixRecordingRulesGroupRead,
		UpdateContext: resourceCoralogixRecordingRulesGroupUpdate,
		DeleteContext: resourceCoralogixRecordingRulesGroupDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(60 * time.Second),
			Read:   schema.DefaultTimeout(30 * time.Second),
			Update: schema.DefaultTimeout(60 * time.Second),
			Delete: schema.DefaultTimeout(30 * time.Second),
		},

		Schema: RecordingRulesGroup(),

		Description: "Coralogix recording-rules-groups-group. For more information - https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/#recording-rules.",
	}
}

func resourceCoralogixRecordingRulesGroupCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	req, err := expandRecordingRulesGroup(d)
	if err != nil {
		return diag.FromErr(err)
	}

	log.Printf("[INFO] Creating new recording-rule-group: %#v", req)
	resp, err := meta.(*clientset.ClientSet).RecordingRuleGroups().CreateRecordingRuleGroup(ctx, req)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "recording-rule-group", req.Name)
	}
	log.Printf("[INFO] Submitted new recording-rule-groups: %#v", resp)

	d.SetId(req.Name)
	return resourceCoralogixRecordingRulesGroupRead(ctx, d, meta)
}

func expandRecordingRulesGroup(d *schema.ResourceData) (*recordingrules.RecordingRuleGroup, error) {
	if yamlContent, ok := d.GetOk("yaml_content"); ok {
		return expandRecordingRulesGroupFromYaml(yamlContent.(string))
	}

	return expandRecordingRulesGroupExplicitly(d.Get("group")), nil
}

func expandRecordingRulesGroupFromYaml(yamlContent string) (*recordingrules.RecordingRuleGroup, error) {
	var result recordingrules.RecordingRuleGroup
	if err := yaml.Unmarshal([]byte(yamlContent), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func expandRecordingRulesGroupExplicitly(v interface{}) *recordingrules.RecordingRuleGroup {
	l := v.([]interface{})
	m := l[0].(map[string]interface{})
	name := m["name"].(string)
	interval := uint32(m["interval"].(int))
	limit := uint64(m["limit"].(int))
	rules := expandRecordingRules(m["rules"])

	return &recordingrules.RecordingRuleGroup{
		Name:     name,
		Interval: &interval,
		Limit:    &limit,
		Rules:    rules,
	}
}

func expandRecordingRules(v interface{}) []*recordingrules.RecordingRule {
	l := v.(*schema.Set).List()
	result := make([]*recordingrules.RecordingRule, 0, len(l))
	for _, recordingRule := range l {
		r := expandRecordingRule(recordingRule)
		result = append(result, r)
	}
	return result
}

func expandRecordingRule(v interface{}) *recordingrules.RecordingRule {
	m := v.(map[string]interface{})

	record := m["record"].(string)
	expr := m["expr"].(string)
	labels := expandRecordingRuleLabels(m["labels"].(map[string]interface{}))

	return &recordingrules.RecordingRule{
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

func resourceCoralogixRecordingRulesGroupRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	name := d.Id()
	log.Printf("[INFO] Reading recording-rule-group %s", name)
	req := &recordingrules.FetchRuleGroup{
		Name: name,
	}
	resp, err := meta.(*clientset.ClientSet).RecordingRuleGroups().GetRecordingRuleGroup(ctx, req)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "recording-rule-group", req.Name)
	}

	log.Printf("[INFO] Received recording-rule-group: %#v", resp)
	setRecordingRulesGroup(d, resp.RuleGroup)
	return nil
}

func resourceCoralogixRecordingRulesGroupUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	req, err := expandRecordingRulesGroup(d)
	if err != nil {
		return diag.FromErr(err)
	}

	log.Printf("[INFO] Updating recording-rule-group: %#v", req)
	resp, err := meta.(*clientset.ClientSet).RecordingRuleGroups().UpdateRecordingRuleGroup(ctx, req)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "recording-rule-group", req.Name)
	}
	log.Printf("[INFO] Submitted updated recording-rule-group: %#v", resp)

	return resourceCoralogixRecordingRulesGroupRead(ctx, d, meta)
}

func resourceCoralogixRecordingRulesGroupDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	name := d.Id()
	req := &recordingrules.DeleteRuleGroup{Name: name}
	log.Printf("[INFO] Deleting recording-rule-group %s", name)
	_, err := meta.(*clientset.ClientSet).RecordingRuleGroups().DeleteRecordingRuleGroup(ctx, req)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "recording-rule-group", req.Name)
	}
	log.Printf("[INFO] recording-rule-group %s deleted", name)

	d.SetId("")
	return nil
}

func setRecordingRulesGroup(d *schema.ResourceData, group *recordingrules.RecordingRuleGroup) diag.Diagnostics {
	if err := d.Set("group", flattenRecordingRulesGroup(group)); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func flattenRecordingRulesGroup(group *recordingrules.RecordingRuleGroup) interface{} {
	rules := flattenRecordingRules(group.Rules)
	return []interface{}{
		map[string]interface{}{
			"name":     group.Name,
			"interval": group.Interval,
			"limit":    group.Limit,
			"rules":    rules,
		},
	}
}

func flattenRecordingRules(rules []*recordingrules.RecordingRule) interface{} {
	result := make([]interface{}, 0, len(rules))
	for _, rule := range rules {
		flattenedRecordingRule := flattenRecordingRule(rule)
		result = append(result, flattenedRecordingRule)
	}
	return result
}

func flattenRecordingRule(rule *recordingrules.RecordingRule) interface{} {
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

func RecordingRulesGroup() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"yaml_content": {
			Type:         schema.TypeString,
			Optional:     true,
			ExactlyOneOf: []string{"yaml_content", "group"},
			ValidateFunc: validateRecordingRulesGroupYamlContent,
			Description:  "An option to import recording-rule-group from yaml file.",
		},
		"group": {
			Type:         schema.TypeList,
			MaxItems:     1,
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
					"rules": {
						Type:     schema.TypeSet,
						Required: true,
						Elem:     recordingRulesSchema(),
						Set:      schema.HashResource(recordingRulesSchema()),
					},
				},
			},
			Description: "An option to defining recording-rule-group explicitly. If not set, will be computed by yaml_content.",
		},
	}
}

func validateRecordingRulesGroupYamlContent(config interface{}, _ string) ([]string, []error) {
	var group recordingrules.RecordingRuleGroup
	if err := yaml.Unmarshal([]byte(config.(string)), &group); err != nil {
		return nil, []error{err}
	}
	if group.Name == "" {
		return nil, []error{fmt.Errorf("groups' name can not be empty")}
	}
	if group.Interval == nil {
		return nil, []error{fmt.Errorf("groups' limit have to be set")}
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
