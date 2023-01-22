package coralogix

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"terraform-provider-coralogix/coralogix/clientset"
)

var (
	tcoPolicySchemaFilterTypeToTcoPolicyRequestFilterType = map[string]string{
		"starts_with": "Starts With",
		"is":          "Is",
		"is_not":      "Is Not",
		"includes":    "Includes",
	}
	tcoPolicyResponseFilterTypeToTcoPolicySchemaFilterType = reverseMapStrings(tcoPolicySchemaFilterTypeToTcoPolicyRequestFilterType)
	validPolicyFilterTypes                                 = getKeysStrings(tcoPolicySchemaFilterTypeToTcoPolicyRequestFilterType)
	validPolicyPriorities                                  = []string{"high", "medium", "low", "block"}
	tcoPolicySchemaSeverityToTcoPolicyRequestSeverity      = map[string]int{
		"debug":    1,
		"verbose":  2,
		"info":     3,
		"warning":  4,
		"error":    5,
		"critical": 6,
	}
	tcoPolicyResponseSeverityToTcoPolicySchemaSeverity = reverseMapIntToString(tcoPolicySchemaSeverityToTcoPolicyRequestSeverity)
	validPolicySeverities                              = getKeysInt(tcoPolicySchemaSeverityToTcoPolicyRequestSeverity)
)

type tcoPolicyRequest struct {
	Name             string            `json:"name"`
	Enabled          bool              `json:"enabled"`
	Priority         string            `json:"priority"`
	Order            int               `json:"order"`
	ApplicationNames []tcoPolicyFilter `json:"applicationName"`
	SubsystemNames   []tcoPolicyFilter `json:"subsystemName"`
	Severities       []int             `json:"severities"`
}

type tcoPolicyFilter struct {
	Type string   `json:"type"`
	Rule []string `json:"rule"`
}

func resourceCoralogixTCOPolicy() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCoralogixTCOPolicyCreate,
		ReadContext:   resourceCoralogixTCOPolicyRead,
		UpdateContext: resourceCoralogixTCOPolicyUpdate,
		DeleteContext: resourceCoralogixTCOPolicyDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(60 * time.Second),
			Read:   schema.DefaultTimeout(30 * time.Second),
			Update: schema.DefaultTimeout(60 * time.Second),
			Delete: schema.DefaultTimeout(30 * time.Second),
		},

		Schema: TCOPolicySchema(),

		Description: "Coralogix recording-rules-groups-group. " +
			"Api-key is required for this resource. " +
			"For more information - https://coralogix.com/docs/tco-optimizer-api .",
	}
}

func resourceCoralogixTCOPolicyCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	tcoPolicyReq, err := extractTCOPolicyRequest(d)
	if err != nil {
		return diag.FromErr(err)
	}

	log.Printf("[INFO] Creating new tco-policy: %#v", tcoPolicyReq)
	tcoPolicyResp, err := meta.(*clientset.ClientSet).TCOPolicies().CreateTCOPolicy(ctx, tcoPolicyReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcError(err, "tco-policy")
	}

	log.Printf("[INFO] Submitted new tco-policy: %#v", tcoPolicyResp)

	var m map[string]interface{}
	if err = json.Unmarshal([]byte(tcoPolicyResp), &m); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(m["id"].(string))
	return resourceCoralogixTCOPolicyRead(ctx, d, meta)
}

func resourceCoralogixTCOPolicyRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	log.Printf("[INFO] Reading tco-policy %s", id)
	tcoPolicyResp, err := meta.(*clientset.ClientSet).TCOPolicies().GetTCOPolicy(ctx, id)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
	}

	log.Printf("[INFO] Received tco-policy: %#v", tcoPolicyResp)

	return setTCOPolicy(d, tcoPolicyResp)
}

func resourceCoralogixTCOPolicyUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	tcoPolicyReq, err := extractTCOPolicyRequest(d)
	if err != nil {
		return diag.FromErr(err)
	}

	id := d.Id()
	log.Printf("[INFO] Updating tco-policy %s to %s", id, tcoPolicyReq)
	tcoPolicyResp, err := meta.(*clientset.ClientSet).TCOPolicies().UpdateTCOPolicy(ctx, id, tcoPolicyReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcError(err, "tco-policy")
	}

	log.Printf("[INFO] Submitted new tco-policy: %#v", tcoPolicyResp)

	var m map[string]interface{}
	if err = json.Unmarshal([]byte(tcoPolicyResp), &m); err != nil {
		return diag.FromErr(err)
	}

	return resourceCoralogixTCOPolicyRead(ctx, d, meta)
}

func resourceCoralogixTCOPolicyDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()

	log.Printf("[INFO] Deleting tco-policy %s", id)
	err := meta.(*clientset.ClientSet).TCOPolicies().DeleteTCOPolicy(ctx, id)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "tco-policy", id)
	}
	log.Printf("[INFO] tco-policy %s deleted", id)

	d.SetId("")
	return nil
}

func extractTCOPolicyRequest(d *schema.ResourceData) (string, error) {
	name := d.Get("name").(string)
	enable := d.Get("enable").(bool)
	priority := d.Get("priority").(string)
	severities := expandTCOPolicySeverities(d.Get("severities"))
	applicationNames := expandTCOPolicyFilters(d.Get("application_names"))
	subsystemNames := expandTCOPolicyFilters(d.Get("subsystem_names"))

	reqStruct := tcoPolicyRequest{
		Name:             name,
		Enabled:          enable,
		Priority:         priority,
		Severities:       severities,
		ApplicationNames: applicationNames,
		SubsystemNames:   subsystemNames,
	}

	requestJson, err := json.Marshal(reqStruct)
	if err != nil {
		return "", err
	}

	return string(requestJson), nil
}

func expandTCOPolicyFilters(v interface{}) []tcoPolicyFilter {
	filters := v.(*schema.Set).List()
	result := make([]tcoPolicyFilter, 0, len(filters))
	for _, filter := range filters {
		f := expandTCOPolicyFilter(filter)
		result = append(result, f)
	}
	return result
}

func expandTCOPolicyFilter(v interface{}) tcoPolicyFilter {
	m := v.(map[string]interface{})
	filterType := tcoPolicySchemaFilterTypeToTcoPolicyRequestFilterType[m["type"].(string)]
	rules := interfaceSliceToStringSlice(m["rules"].(*schema.Set).List())
	return tcoPolicyFilter{
		Type: filterType,
		Rule: rules,
	}
}

func expandTCOPolicySeverities(v interface{}) []int {
	severities := v.(*schema.Set).List()
	result := make([]int, 0, len(severities))
	for _, severity := range severities {
		s := tcoPolicySchemaSeverityToTcoPolicyRequestSeverity[severity.(string)]
		result = append(result, s)
	}
	return result
}

func setTCOPolicy(d *schema.ResourceData, tcoPolicyResp string) diag.Diagnostics {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(tcoPolicyResp), &m); err != nil {
		return diag.FromErr(err)
	}

	var diags diag.Diagnostics
	if err := d.Set("name", m["name"].(string)); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}
	if err := d.Set("enable", m["enable"].(bool)); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}
	if err := d.Set("priority", m["name"].(string)); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}
	if err := d.Set("severities", flattenTCOPoliciesSeverities(m["severities"])); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}
	if err := d.Set("application_names", flattenTCOPoliciesFilters(m["applicationName"])); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}
	if err := d.Set("subsystem_names", flattenTCOPoliciesFilters(m["subsystemName"])); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}

	return diags
}

func flattenTCOPoliciesSeverities(v interface{}) interface{} {
	if v == nil {
		return []string{}
	}

	severities := v.([]int)
	result := make([]string, 0, len(severities))
	for _, severity := range severities {
		severityStr := tcoPolicyResponseSeverityToTcoPolicySchemaSeverity[severity]
		result = append(result, severityStr)
	}

	return result
}

func flattenTCOPoliciesFilters(v interface{}) interface{} {
	if v == nil {
		return []string{}
	}

	tcoPoliciesFilters := v.([]interface{})
	result := make([]interface{}, 0, len(tcoPoliciesFilters))
	for _, tcoPoliciesFilter := range tcoPoliciesFilters {
		snf := flattenTcoPolicyFilter(tcoPoliciesFilter)
		result = append(result, snf)
	}

	return result
}

func flattenTcoPolicyFilter(filter interface{}) interface{} {
	m := filter.(map[string]interface{})

	filterType := tcoPolicyResponseFilterTypeToTcoPolicySchemaFilterType[m["type"].(string)]

	var rules []string
	if r, ok := m["rules"].([]string); ok {
		rules = r
	} else {
		rules = []string{m["rules"].(string)}
	}

	return map[string]interface{}{
		"type":  filterType,
		"rules": rules,
	}
}

func TCOPolicySchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"name": {
			Type:         schema.TypeString,
			Required:     true,
			ValidateFunc: validation.StringIsNotEmpty,
			Description:  "The policy name.",
		},
		"enable": {
			Type:        schema.TypeBool,
			Optional:    true,
			Default:     true,
			Description: "Determines weather the policy will be enabled. True by default.",
		},
		"priority": {
			Type:         schema.TypeString,
			Required:     true,
			ValidateFunc: validation.StringInSlice(validPolicyPriorities, false),
			Description:  fmt.Sprintf("The policy description. Can be one of %q.", validPolicyPriorities),
		},
		"order": {
			Type:        schema.TypeInt,
			Optional:    true,
			Computed:    true,
			Description: "Determines the policy's order between the other policies.",
		},
		"severities": {
			Type:     schema.TypeSet,
			Required: true,
			Elem: &schema.Schema{
				Type:         schema.TypeString,
				ValidateFunc: validation.StringInSlice(validPolicySeverities, false),
			},
			Set:         schema.HashString,
			MinItems:    1,
			Description: fmt.Sprintf("The severities to apply the policy on. Can be few of %q.", validPolicySeverities),
		},
		"application_names": {
			Type:        schema.TypeSet,
			Optional:    true,
			Elem:        tcoPolicyFiltersSchema(),
			Set:         schema.HashResource(tcoPolicyFiltersSchema()),
			MinItems:    1,
			Description: "The application to apply the policy on.",
		},
		"subsystem_names": {
			Type:        schema.TypeSet,
			Optional:    true,
			Elem:        tcoPolicyFiltersSchema(),
			Set:         schema.HashResource(tcoPolicyFiltersSchema()),
			MinItems:    1,
			Description: "The subsystems to apply the policy on.",
		},
	}
}

func tcoPolicyFiltersSchema() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"type": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringInSlice(validPolicyFilterTypes, false),
				Description:  fmt.Sprintf("the filtering type. Can be one of %q.", validPolicyFilterTypes),
			},
			"rules": {
				Type:     schema.TypeSet,
				Required: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
					Set:  schema.HashString,
				},
				MinItems: 1,
				Description: "In case type = start_with/includes, rules need to contain single string." +
					" Otherwise (is/is_not), rules can contain more strings.",
			},
		},
	}
}
