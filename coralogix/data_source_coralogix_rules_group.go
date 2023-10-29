package coralogix

import (
	"context"
	"log"

	"github.com/coralogix/coralogix-sdk-demo/parsingrules"
	"terraform-provider-coralogix/coralogix/clientset"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceCoralogixRulesGroup() *schema.Resource {
	rulesGroupSchema := datasourceSchemaFromResourceSchema(RulesGroupSchema())
	rulesGroupSchema["id"] = &schema.Schema{
		Type:     schema.TypeString,
		Required: true,
	}

	return &schema.Resource{
		ReadContext: dataSourceCoralogixRulesGroupRead,

		Schema: rulesGroupSchema,
	}
}

func dataSourceCoralogixRulesGroupRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Get("id").(string)
	getRuleGroupRequest := &parsingrules.GetRuleGroupRequest{
		GroupId: id,
	}

	log.Printf("[INFO] Reading rule-group %s", id)
	ruleGroupResp, err := meta.(*clientset.ClientSet).RuleGroups().GetRuleGroup(ctx, getRuleGroupRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "rule-group", id)
	}
	ruleGroup := ruleGroupResp.GetRuleGroup()
	log.Printf("[INFO] Received rule-group: %#v", ruleGroup)

	d.SetId(ruleGroup.GetId().GetValue())

	return setRuleGroup(d, ruleGroup)
}
