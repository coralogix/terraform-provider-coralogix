package coralogix

import (
	"context"
	"log"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"terraform-provider-coralogix/coralogix/clientset"
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

		Description: "Coralogix recording-rules-groups-group. Api-key is required for this resource.",
	}
}

func resourceCoralogixRecordingRulesGroupCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	yamlContent := d.Get("yaml_content").(string)

	log.Printf("[INFO] Creating new recording-rule-groups: %#v", yamlContent)
	resp, err := meta.(*clientset.ClientSet).RecordingRulesGroups().CreateRecordingRuleRules(ctx, yamlContent)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
	}
	log.Printf("[INFO] Submitted new recording-rule-groups: %#v", resp)

	d.SetId("recording-rule-groups")
	return resourceCoralogixRecordingRulesGroupRead(ctx, d, meta)
}

func resourceCoralogixRecordingRulesGroupRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	//log.Print("[INFO] Reading recording-rule-groups")
	//resp, err := meta.(*clientset.ClientSet).RecordingRulesGroups().GetRecordingRuleRules(ctx)
	//if err != nil {
	//	log.Printf("[ERROR] Received error: %#v", err)
	//}
	//log.Printf("[INFO] Received recording-rule-groups: %#v", resp)
	//
	//if err = d.Set("yaml_content", resp); err != nil {
	//	return diag.FromErr(err)
	//}

	return nil
}

func resourceCoralogixRecordingRulesGroupUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	yamlContent := d.Get("yaml_content").(string)

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

func RecordingRulesGroup() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"yaml_content": {
			Type:     schema.TypeString,
			Required: true,
		},
	}
}
