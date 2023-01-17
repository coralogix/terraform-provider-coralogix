package coralogix

import (
	"context"
	"fmt"
	"log"
	"time"

	"terraform-provider-coralogix/coralogix/clientset"

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

		Description: "Coralogix recording-rules-groups-group. " +
			"Api-key is required for this resource. " +
			"For more information - https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/#recording-rules. " +
			"***Please note!*** - all the recording-rule-groups are manged as single resource. " +
			"Therefore, each definition of recording-rule-groups replaces the entire state.",
	}
}

func resourceCoralogixRecordingRulesGroupCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	yamlContent, err := expandYamlContent(d)
	if err != nil {
		return diag.FromErr(err)
	}

	log.Printf("[INFO] Creating new recording-rule-groups: %#v", yamlContent)
	resp, err := meta.(*clientset.ClientSet).RecordingRulesGroups().CreateRecordingRuleRules(ctx, yamlContent)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
	}
	log.Printf("[INFO] Submitted new recording-rule-groups: %#v", resp)

	d.SetId("recording-rule-groups")
	return resourceCoralogixRecordingRulesGroupRead(ctx, d, meta)
}

func expandYamlContent(d *schema.ResourceData) (string, error) {
	if yamlContent, ok := d.GetOk("yaml_content"); ok {
		return yamlContent.(string), nil
	}

	groups := expandRecordingRulesGroups(d.Get("groups"))
	b, err := yaml.Marshal(groups)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func expandRecordingRulesGroups(v interface{}) recordingRulesGroups {
	l := v.(*schema.Set).List()
	result := make([]recordingRulesGroup, 0, len(l))
	for _, recordingRulesGroup := range l {
		r := expandRecordingRulesGroup(recordingRulesGroup)
		result = append(result, r)
	}

	return recordingRulesGroups{
		Groups: result,
	}
}

func expandRecordingRulesGroup(v interface{}) recordingRulesGroup {
	m := v.(map[string]interface{})
	name := m["name"].(string)
	interval := uint(m["interval"].(int))
	limit := uint(m["limit"].(int))
	rules := expandRecordingRules(m["rules"])

	return recordingRulesGroup{
		Name:     name,
		Interval: interval,
		Limit:    limit,
		Rules:    rules,
	}
}

func expandRecordingRules(v interface{}) []recordingRules {
	l := v.(*schema.Set).List()
	result := make([]recordingRules, 0, len(l))
	for _, recordingRule := range l {
		r := expandRecordingRule(recordingRule)
		result = append(result, r)
	}
	return result
}

func expandRecordingRule(v interface{}) recordingRules {
	m := v.(map[string]interface{})

	record := m["record"].(string)
	expr := m["expr"].(string)
	labels := m["labels"].(map[string]interface{})

	return recordingRules{
		Record: record,
		Expr:   expr,
		Labels: labels,
	}
}

func resourceCoralogixRecordingRulesGroupRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	log.Print("[INFO] Reading recording-rule-groups")
	yamlResp, err := meta.(*clientset.ClientSet).RecordingRulesGroups().GetRecordingRuleRules(ctx)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
	}
	log.Printf("[INFO] Received recording-rule-groups: %#v", yamlResp)

	setRecordingRulesGroups(d, yamlResp)
	return nil
}

func resourceCoralogixRecordingRulesGroupUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	yamlContent, err := expandYamlContent(d)
	if err != nil {
		return diag.FromErr(err)
	}

	log.Printf("[INFO] Updating recording-rule-groups: %#v", yamlContent)
	resp, err := meta.(*clientset.ClientSet).RecordingRulesGroups().UpdateRecordingRuleRules(ctx, yamlContent)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
	}
	log.Printf("[INFO] Submitted updated recording-rule-groups: %#v", resp)

	return resourceCoralogixRecordingRulesGroupRead(ctx, d, meta)
}

func resourceCoralogixRecordingRulesGroupDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	log.Print("[INFO] Deleting recording-rule-groups")
	err := meta.(*clientset.ClientSet).RecordingRulesGroups().DeleteRecordingRuleRules(ctx)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
	}
	log.Printf("[INFO] recording-rule-groups deleted")

	d.SetId("")
	return nil
}

func setRecordingRulesGroups(d *schema.ResourceData, yamlResp string) diag.Diagnostics {
	var groups recordingRulesGroups

	if err := yaml.Unmarshal([]byte(yamlResp), &groups.Groups); err != nil {
		panic(fmt.Sprint(groups))
		return diag.FromErr(err)
	}

	if err := d.Set("groups", flattenRecordingRulesGroups(groups)); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func flattenRecordingRulesGroups(groups recordingRulesGroups) interface{} {
	result := make([]interface{}, 0, len(groups.Groups))
	for _, group := range groups.Groups {
		flattenedGroup := flattenRecordingRulesGroup(group)
		result = append(result, flattenedGroup)
	}
	return result
}

func flattenRecordingRulesGroup(group recordingRulesGroup) interface{} {
	rules := flattenRecordingRules(group.Rules)
	return map[string]interface{}{
		"name":     group.Name,
		"interval": group.Interval,
		"limit":    group.Limit,
		"rules":    rules,
	}
}

func flattenRecordingRules(rules []recordingRules) interface{} {
	result := make([]interface{}, 0, len(rules))
	for _, rule := range rules {
		flattenedRecordingRule := flattenRecordingRule(rule)
		result = append(result, flattenedRecordingRule)
	}
	return result
}

func flattenRecordingRule(rule recordingRules) interface{} {
	return map[string]interface{}{
		"record": rule.Record,
		"expr":   rule.Expr,
		"labels": rule.Labels,
	}
}

func RecordingRulesGroup() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"yaml_content": {
			Type:         schema.TypeString,
			Optional:     true,
			ExactlyOneOf: []string{"yaml_content", "groups"},
		},
		"groups": {
			Type:     schema.TypeSet,
			Optional: true,
			Computed: true,
			Elem:     recordingRulesGroupsSchema(),
			Set:      hashRecordingRulesGroups(),
		},
	}
}

func recordingRulesGroupsSchema() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"interval": {
				Type:         schema.TypeInt,
				Required:     true,
				ValidateFunc: validation.IntAtLeast(0),
			},
			"limit": {
				Type:     schema.TypeInt,
				Optional: true,
			},
			"rules": {
				Type:     schema.TypeSet,
				Required: true,
				Elem:     recordingRulesSchema(),
				Set:      hashRecordingRules(),
			},
		},
	}
}

func recordingRulesSchema() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"record": {
				Type:     schema.TypeString,
				Required: true,
			},
			"expr": {
				Type:     schema.TypeString,
				Required: true,
			},
			"labels": {
				Type:     schema.TypeMap,
				Optional: true,
			},
		},
	}
}

func hashRecordingRulesGroups() schema.SchemaSetFunc {
	return schema.HashResource(recordingRulesGroupsSchema())
}

func hashRecordingRules() schema.SchemaSetFunc {
	return schema.HashResource(recordingRulesSchema())
}

type recordingRulesGroups struct {
	Groups []recordingRulesGroup `yaml:"groups"`
}

type recordingRulesGroup struct {
	Name     string           `yaml:"name"`
	Interval uint             `yaml:"interval"`
	Limit    uint             `yaml:"limit"`
	Rules    []recordingRules `yaml:"rules"`
}

type recordingRules struct {
	Record string                 `yaml:"record"`
	Expr   string                 `yaml:"expr"`
	Labels map[string]interface{} `yaml:"labels"`
}
