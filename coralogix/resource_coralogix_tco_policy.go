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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"terraform-provider-coralogix/coralogix/clientset"
)

var (
	tcoPolicyResponseFilterTypeToTcoPolicySchemaFilterType = map[string]string{
		"Starts With": "starts_with",
		"Is":          "is",
		"Is Not":      "is_not",
		"Includes":    "includes",
	}
	validPolicyPriorities                             = []string{"high", "medium", "low", "block"}
	tcoPolicySchemaSeverityToTcoPolicyRequestSeverity = map[string]int{
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
	Priority         string            `json:"priority"`
	Enabled          bool              `json:"enabled,omitempty"`
	Order            *int              `json:"order,omitempty"`
	ApplicationName  *tcoPolicyFilter  `json:"applicationName,omitempty"`
	SubsystemName    *tcoPolicyFilter  `json:"subsystemName,omitempty"`
	Severities       *[]int            `json:"severities,omitempty"`
	ArchiveRetention *archiveRetention `json:"archiveRetention,omitempty"`
}

type tcoPolicyFilter struct {
	Type string      `json:"type"`
	Rule interface{} `json:"rule"`
}

type archiveRetention struct {
	Id string `json:"archiveRetentionId"`
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

		Description: "Coralogix TCO-Policy. For more information - https://coralogix.com/docs/tco-optimizer-api .",
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
		fmt.Sprintf("Error - %s\nRequest - %s", err.Error(), tcoPolicyReq)
		return handleRpcError(err, "tco-policy")
	}

	log.Printf("[INFO] Submitted new tco-policy: %#v", tcoPolicyResp)

	var m map[string]interface{}
	if err = json.Unmarshal([]byte(tcoPolicyResp), &m); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(m["id"].(string))

	if err = updatePoliciesOrder(ctx, d, meta); err != nil {
		return diag.FromErr(err)
	}

	return resourceCoralogixTCOPolicyRead(ctx, d, meta)
}

func updatePoliciesOrder(ctx context.Context, d *schema.ResourceData, meta interface{}) error {
	tcoPoliciesResp, err := meta.(*clientset.ClientSet).TCOPolicies().GetTCOPolicies(ctx)
	var policies []map[string]interface{}
	if err = json.Unmarshal([]byte(tcoPoliciesResp), &policies); err != nil {
		return err
	}

	policiesOrders := make([]string, len(policies))
	currentIndex := -1
	for i, policy := range policies {
		id := policy["id"].(string)
		policiesOrders[i] = id
		if id == d.Id() {
			currentIndex = i
		}
	}
	desiredIndex := d.Get("order").(int) - 1
	if desiredIndex >= len(policies) {
		desiredIndex = len(policies) - 1
	}
	if currentIndex == desiredIndex {
		return nil
	}
	policiesOrders[currentIndex], policiesOrders[desiredIndex] = policiesOrders[desiredIndex], policiesOrders[currentIndex]

	reorderRequest, err := json.Marshal(policiesOrders)
	if _, err = meta.(*clientset.ClientSet).TCOPolicies().ReorderTCOPolicies(ctx, string(reorderRequest)); err != nil {
		return err
	}

	return nil
}

func resourceCoralogixTCOPolicyRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	log.Printf("[INFO] Reading tco-policy %s", id)
	tcoPolicyResp, err := meta.(*clientset.ClientSet).TCOPolicies().GetTCOPolicy(ctx, id)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if status.Code(err) == codes.NotFound {
			d.SetId("")
			return diag.Diagnostics{diag.Diagnostic{
				Severity: diag.Warning,
				Summary:  fmt.Sprintf("Tco-Policy %q is in state, but no longer exists in Coralogix backend", id),
				Detail:   fmt.Sprintf("%s will be recreated when you apply", id),
			}}
		}
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

	d.SetId(m["id"].(string))

	if err = updatePoliciesOrder(ctx, d, meta); err != nil {
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
	enable := d.Get("enabled").(bool)
	priority := d.Get("priority").(string)
	severities := expandTCOPolicySeverities(d.Get("severities"))
	applicationName := expandTCOPolicyFilter(d.Get("application_name"))
	subsystemName := expandTCOPolicyFilter(d.Get("subsystem_name"))
	archiveRetention := expandActiveRetention(d.Get("archive_retention_id"))

	reqStruct := tcoPolicyRequest{
		Name:             name,
		Enabled:          enable,
		Priority:         priority,
		Severities:       severities,
		ApplicationName:  applicationName,
		SubsystemName:    subsystemName,
		ArchiveRetention: archiveRetention,
	}

	requestJson, err := json.Marshal(reqStruct)
	if err != nil {
		return "", err
	}

	return string(requestJson), nil
}

func expandActiveRetention(v interface{}) *archiveRetention {
	if v == nil || v == "" {
		return nil
	}
	return &archiveRetention{
		Id: v.(string),
	}
}

func expandTCOPolicyFilter(v interface{}) *tcoPolicyFilter {
	l := v.([]interface{})
	if len(l) == 0 {
		return nil
	}
	m := l[0].(map[string]interface{})

	filterType := expandTcoPolicyFilterType(m)
	rule := expandTcoPolicyFilterRule(m)

	return &tcoPolicyFilter{
		Type: filterType,
		Rule: rule,
	}
}

func expandTcoPolicyFilterRule(m map[string]interface{}) interface{} {
	if rules, ok := m["rules"]; ok && rules != nil {
		rulesList := rules.(*schema.Set).List()
		if len(rulesList) == 0 {
			return m["rule"].(string)
		} else {
			return rulesList
		}
	}
	return m["rule"].(string)
}

func expandTcoPolicyFilterType(m map[string]interface{}) string {
	var filterType string
	if is, ok := m["is"]; ok && is.(bool) {
		filterType = "Is"
	} else if isNot, ok := m["is_not"]; ok && isNot.(bool) {
		filterType = "Is Not"
	} else if starsWith, ok := m["starts_with"]; ok && starsWith.(bool) {
		filterType = "Starts With"
	} else {
		filterType = "Includes"
	}
	return filterType
}

func expandTCOPolicySeverities(v interface{}) *[]int {
	if v == nil {
		return nil
	}
	severities := v.(*schema.Set).List()
	result := make([]int, 0, len(severities))
	for _, severity := range severities {
		s := tcoPolicySchemaSeverityToTcoPolicyRequestSeverity[severity.(string)]
		result = append(result, s)
	}
	return &result
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
	if err := d.Set("enabled", m["enabled"].(bool)); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}
	if err := d.Set("order", int(m["order"].(float64))); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}
	if err := d.Set("priority", m["priority"].(string)); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}
	if err := d.Set("severities", flattenTCOPolicySeverities(m["severities"])); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}
	if err := d.Set("application_name", flattenTCOPolicyFilter(m["applicationName"])); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}
	if err := d.Set("subsystem_name", flattenTCOPolicyFilter(m["subsystemName"])); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}
	if err := d.Set("archive_retention_id", flattenArchiveRetention(m["archiveRetention"])); err != nil {

	}
	return diags
}

func flattenArchiveRetention(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	archiveRetention := v.(map[string]interface{})
	return archiveRetention["archiveRetentionId"].(string)
}

func flattenTCOPolicySeverities(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	severities := v.([]interface{})
	result := make([]string, 0, len(severities))
	for _, severity := range severities {
		severityStr := tcoPolicyResponseSeverityToTcoPolicySchemaSeverity[int(severity.(float64))]
		result = append(result, severityStr)
	}

	return result
}

func flattenTCOPolicyFilter(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	filter := v.(map[string]interface{})

	filterType := tcoPolicyResponseFilterTypeToTcoPolicySchemaFilterType[filter["type"].(string)]
	flattenedFilter := map[string]interface{}{
		filterType: true,
	}

	if rules, ok := filter["rule"].([]interface{}); ok {
		flattenedFilter["rules"] = interfaceSliceToStringSlice(rules)
	} else {
		flattenedFilter["rule"] = filter["rule"].(string)
	}

	return []interface{}{flattenedFilter}
}

func TCOPolicySchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"name": {
			Type:         schema.TypeString,
			Required:     true,
			ValidateFunc: validation.StringIsNotEmpty,
			Description:  "The policy name. Have to be unique per policy.",
		},
		"enabled": {
			Type:        schema.TypeBool,
			Optional:    true,
			Default:     true,
			Description: "Determines weather the policy will be enabled. True by default.",
		},
		"priority": {
			Type:         schema.TypeString,
			Required:     true,
			ValidateFunc: validation.StringInSlice(validPolicyPriorities, false),
			Description:  fmt.Sprintf("The policy priority. Can be one of %q.", validPolicyPriorities),
		},
		"order": {
			Type:         schema.TypeInt,
			Required:     true,
			ValidateFunc: validation.IntAtLeast(1),
			Description:  "Determines the policy's order between the other policies. Currently, will be computed by creation order.",
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
		"application_name": {
			Type:        schema.TypeList,
			MaxItems:    1,
			Optional:    true,
			Elem:        tcoPolicyFiltersSchema("application_name"),
			Description: "The applications to apply the policy on. Applies the policy on all the applications by default.",
		},
		"subsystem_name": {
			Type:        schema.TypeList,
			MaxItems:    1,
			Optional:    true,
			Elem:        tcoPolicyFiltersSchema("subsystem_name"),
			Description: "The subsystems to apply the policy on. Applies the policy on all the subsystems by default.",
		},
		"archive_retention_id": {
			Type:         schema.TypeString,
			Optional:     true,
			Description:  "Allowing logs with a specific retention to be tagged.",
			ValidateFunc: validation.StringIsNotEmpty,
		},
	}
}

func tcoPolicyFiltersSchema(filterName string) *schema.Resource {
	filterTypesRoutes := filterTypesRoutes(filterName)
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"is": {
				Type:         schema.TypeBool,
				Optional:     true,
				ExactlyOneOf: filterTypesRoutes,
				RequiredWith: []string{fmt.Sprintf("%s.0.rules", filterName)},
				Description:  "Determines the filter's type. One of is/is_not/starts_with/includes have to be set.",
			},
			"is_not": {
				Type:         schema.TypeBool,
				Optional:     true,
				ExactlyOneOf: filterTypesRoutes,
				RequiredWith: []string{fmt.Sprintf("%s.0.rules", filterName)},
				Description:  "Determines the filter's type. One of is/is_not/starts_with/includes have to be set.",
			},
			"starts_with": {
				Type:         schema.TypeBool,
				Optional:     true,
				ExactlyOneOf: filterTypesRoutes,
				RequiredWith: []string{fmt.Sprintf("%s.0.rule", filterName)},
				Description:  "Determines the filter's type. One of is/is_not/starts_with/includes have to be set.",
			},
			"includes": {
				Type:         schema.TypeBool,
				Optional:     true,
				ExactlyOneOf: filterTypesRoutes,
				RequiredWith: []string{fmt.Sprintf("%s.0.rule", filterName)},
				Description:  "Determines the filter's type. One of is/is_not/starts_with/includes have to be set.",
			},
			"rules": {
				Type:     schema.TypeSet,
				Optional: true,
				MinItems: 1,
				Elem: &schema.Schema{
					Type: schema.TypeString,
					Set:  schema.HashString,
				},
				ExactlyOneOf: []string{fmt.Sprintf("%s.0.rule", filterName), fmt.Sprintf("%s.0.rules", filterName)},
				Description:  "Set of rules to apply the filter on. In case of is=true/is_not=true replace to 'rules' (set of strings).",
			},
			"rule": {
				Type:         schema.TypeString,
				Optional:     true,
				ExactlyOneOf: []string{fmt.Sprintf("%s.0.rule", filterName), fmt.Sprintf("%s.0.rules", filterName)},
				Description:  "Single rule to apply the filter on. In case of start_with=true/includes=true replace to 'rule' (single string).",
			},
		},
	}
}

func filterTypesRoutes(filterName string) []string {
	return []string{
		fmt.Sprintf("%s.0.is", filterName),
		fmt.Sprintf("%s.0.is_not", filterName),
		fmt.Sprintf("%s.0.starts_with", filterName),
		fmt.Sprintf("%s.0.includes", filterName),
	}
}
