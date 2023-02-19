package coralogix

import (
	"context"
	"log"

	"terraform-provider-coralogix/coralogix/clientset"
	recordingrules "terraform-provider-coralogix/coralogix/clientset/grpc/recording-rules-groups/v1"

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
	id := d.Get("id").(string)
	req := &recordingrules.FetchRuleGroup{
		Name: id,
	}
	log.Printf("[INFO] Reading recording-rule-group %s", id)
	resp, err := meta.(*clientset.ClientSet).RecordingRuleGroups().GetRecordingRuleGroup(ctx, req)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "recording-rule-group", req.Name)
	}

	log.Printf("[INFO] Received recording-rule-group: %#v", resp)

	d.SetId(resp.RuleGroup.Name)
	return setRecordingRulesGroup(d, resp.RuleGroup)
}
