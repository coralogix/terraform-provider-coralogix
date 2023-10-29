package coralogix

import (
	"context"
	"log"

	"github.com/coralogix/coralogix-sdk-demo/recordingrules"
	"terraform-provider-coralogix/coralogix/clientset"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceCoralogixRecordingRulesGroupsSet() *schema.Resource {
	recordingRulesGroupsSetSchema := datasourceSchemaFromResourceSchema(RecordingRulesGroupsSetSchema())
	recordingRulesGroupsSetSchema["id"] = &schema.Schema{
		Type:     schema.TypeString,
		Required: true,
	}

	return &schema.Resource{
		ReadContext: dataSourceCoralogixRecordingRulesGroupsSetRead,

		Schema: recordingRulesGroupsSetSchema,
	}
}

func dataSourceCoralogixRecordingRulesGroupsSetRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Get("id").(string)
	req := &recordingrules.FetchRuleGroupSet{
		Id: id,
	}
	log.Printf("[INFO] Reading recording-rule-group-set %s", id)
	resp, err := meta.(*clientset.ClientSet).RecordingRuleGroupsSets().GetRecordingRuleGroupsSet(ctx, req)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "recording-rule-group-set", req.Id)
	}

	log.Printf("[INFO] Received recording-rule-group-set: %#v", resp)

	d.SetId(resp.Id)
	return setRecordingRulesGroupsSet(d, resp)
}
