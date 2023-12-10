package coralogix

import (
	"context"
	"log"

	"terraform-provider-coralogix/coralogix/clientset"
	v1 "terraform-provider-coralogix/coralogix/clientset/grpc/rules-groups/v1"

	"google.golang.org/protobuf/encoding/protojson"

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
	getRuleGroupRequest := &v1.GetRuleGroupRequest{
		GroupId: id,
	}

	log.Printf("[INFO] Reading rule-group %s", id)
	ruleGroupResp, err := meta.(*clientset.ClientSet).RuleGroups().GetRuleGroup(ctx, getRuleGroupRequest)
	if err != nil {
		reqStr := protojson.Format(getRuleGroupRequest)
		log.Printf("[ERROR] Received error: %#v", err)
		return diag.Errorf(formatRpcErrors(err, getParsingRuleURL, reqStr))
	}
	ruleGroup := ruleGroupResp.GetRuleGroup()
	log.Printf("[INFO] Received rule-group: %#v", ruleGroup)

	d.SetId(ruleGroup.GetId().GetValue())

	return setRuleGroup(d, ruleGroup)
}
