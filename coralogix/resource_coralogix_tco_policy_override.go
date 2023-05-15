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
	"k8s.io/apimachinery/pkg/api/errors"
	"terraform-provider-coralogix/coralogix/clientset"
)

type tcoPolicyOverrideRequest struct {
	Priority        string  `json:"priority"`
	ApplicationName *string `json:"applicationName,omitempty"`
	SubsystemName   *string `json:"subsystemName,omitempty"`
	Severity        int     `json:"severity,omitempty"`
}

func resourceCoralogixTCOPolicyOverride() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCoralogixTCOPolicyOverrideCreate,
		ReadContext:   resourceCoralogixTCOPolicyOverrideRead,
		UpdateContext: resourceCoralogixTCOPolicyOverrideUpdate,
		DeleteContext: resourceCoralogixTCOPolicyOverrideDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(60 * time.Second),
			Read:   schema.DefaultTimeout(30 * time.Second),
			Update: schema.DefaultTimeout(60 * time.Second),
			Delete: schema.DefaultTimeout(30 * time.Second),
		},

		Schema: TCOPolicyOverrideSchema(),

		Description: "Coralogix TCO-Policy-Override. For more information - https://coralogix.com/docs/tco-optimizer-api/#policy-overrides .",
	}
}

func resourceCoralogixTCOPolicyOverrideCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	tcoPolicyReq, err := extractTCOPolicyOverrideRequest(d)
	if err != nil {
		return diag.FromErr(err)
	}

	log.Printf("[INFO] Creating new tco-policy-override: %#v", tcoPolicyReq)
	tcoPolicyOverrideResp, err := meta.(*clientset.ClientSet).TCOPoliciesOverrides().CreateTCOPolicyOverride(ctx, tcoPolicyReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcError(err, "tco-policy-override")
	}

	log.Printf("[INFO] Submitted new tco-policy-override: %#v", tcoPolicyOverrideResp)

	var m map[string]interface{}
	if err = json.Unmarshal([]byte(tcoPolicyOverrideResp), &m); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(m["id"].(string))
	return resourceCoralogixTCOPolicyOverrideRead(ctx, d, meta)
}

func resourceCoralogixTCOPolicyOverrideRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	log.Printf("[INFO] Reading tco-policy-override %s", id)
	tcoPolicyResp, err := meta.(*clientset.ClientSet).TCOPoliciesOverrides().GetTCOPolicyOverride(ctx, id)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if errors.IsNotFound(err) {
			d.SetId("")
			return diag.Diagnostics{diag.Diagnostic{
				Severity: diag.Warning,
				Summary:  fmt.Sprintf("Tco-Policy-override %q is in state, but no longer exists in Coralogix backend", id),
				Detail:   fmt.Sprintf("%s will be recreated when you apply", id),
			}}
		}
		return diag.FromErr(err)
	}

	log.Printf("[INFO] Received tco-policy-override: %#v", tcoPolicyResp)

	return setTCOPolicyOverride(d, tcoPolicyResp)
}

func resourceCoralogixTCOPolicyOverrideUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	tcoPolicyOverrideReq, err := extractTCOPolicyOverrideRequest(d)
	if err != nil {
		return diag.FromErr(err)
	}

	id := d.Id()
	log.Printf("[INFO] Updating tco-policy-override %s to %s", id, tcoPolicyOverrideReq)
	tcoPolicyOverrideResp, err := meta.(*clientset.ClientSet).TCOPoliciesOverrides().UpdateTCOPolicyOverride(ctx, id, tcoPolicyOverrideReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcError(err, "tco-policy-override")
	}

	log.Printf("[INFO] Submitted new tco-policy-override: %#v", tcoPolicyOverrideResp)

	var m map[string]interface{}
	if err = json.Unmarshal([]byte(tcoPolicyOverrideResp), &m); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(m["id"].(string))
	return resourceCoralogixTCOPolicyOverrideRead(ctx, d, meta)
}

func resourceCoralogixTCOPolicyOverrideDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()

	log.Printf("[INFO] Deleting tco-policy %s", id)
	err := meta.(*clientset.ClientSet).TCOPoliciesOverrides().DeleteTCOPolicyOverride(ctx, id)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "tco-policy-override", id)
	}
	log.Printf("[INFO] tco-policy-override %s deleted", id)

	d.SetId("")
	return nil
}

func extractTCOPolicyOverrideRequest(d *schema.ResourceData) (string, error) {
	priority := d.Get("priority").(string)
	var applicationName *string
	if s := d.Get("application_name").(string); s != "" {
		applicationName = new(string)
		*applicationName = s
	}

	var subsystemName *string
	if s := d.Get("subsystem_name").(string); s != "" {
		subsystemName = new(string)
		*subsystemName = s
	}

	severity := tcoPolicySchemaSeverityToTcoPolicyRequestSeverity[d.Get("severity").(string)]
	reqStruct := tcoPolicyOverrideRequest{
		Priority:        priority,
		ApplicationName: applicationName,
		SubsystemName:   subsystemName,
		Severity:        severity,
	}

	requestJson, err := json.Marshal(reqStruct)
	if err != nil {
		return "", err
	}

	return string(requestJson), nil
}

func setTCOPolicyOverride(d *schema.ResourceData, tcoPolicyOverrideResp string) diag.Diagnostics {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(tcoPolicyOverrideResp), &m); err != nil {
		return diag.FromErr(err)
	}

	var diags diag.Diagnostics
	if err := d.Set("priority", m["priority"].(string)); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}
	if err := d.Set("severity", tcoPolicyResponseSeverityToTcoPolicySchemaSeverity[int(m["severity"].(float64))]); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}
	if err := d.Set("application_name", m["applicationName"].(string)); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}
	if err := d.Set("subsystem_name", m["subsystemName"].(string)); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}

	return diags
}

func TCOPolicyOverrideSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"priority": {
			Type:         schema.TypeString,
			Required:     true,
			ValidateFunc: validation.StringInSlice(validPolicyPriorities, false),
			Description:  fmt.Sprintf("The policy-override priority. Can be one of %q.", validPolicyPriorities),
		},
		"severity": {
			Type:         schema.TypeString,
			Required:     true,
			ValidateFunc: validation.StringInSlice(validPolicySeverities, false),
			Description:  fmt.Sprintf("The severity to apply the policy on. Can be one of %q.", validPolicySeverities),
		},
		"application_name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The application to apply the policy on. Applies the policy on all the applications by default.",
		},
		"subsystem_name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The subsystem to apply the policy on. Applies the policy on all the subsystems by default.",
		},
	}
}
