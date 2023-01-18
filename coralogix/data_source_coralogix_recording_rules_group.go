package coralogix

import (
	"context"
	"log"

	"terraform-provider-coralogix/coralogix/clientset"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceCoralogixRecordingRulesGroup() *schema.Resource {
	recordingRulesGroupSchema := datasourceSchemaFromResourceSchema(RecordingRulesGroup())
	recordingRulesGroupSchema["id"] = &schema.Schema{
		Type:     schema.TypeString,
		Required: true,
	}

	return &schema.Resource{
		ReadContext: dataSourceCoralogixRecordingRulesGroupRead,

		Schema: recordingRulesGroupSchema,
	}
}

func dataSourceCoralogixRecordingRulesGroupRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	log.Print("[INFO] Reading recording-rule-groups")
	yamlResp, err := meta.(*clientset.ClientSet).RecordingRulesGroups().GetRecordingRuleRules(ctx)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
	}

	log.Printf("[INFO] Received recording-rule-groups: %#v", yamlResp)

	d.SetId("recording-rule-groups")

	return setRecordingRulesGroups(d, yamlResp)
}
