package coralogix

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"terraform-provider-coralogix/coralogix/clientset"
)

func dataSourceCoralogixTCOPolicyOverride() *schema.Resource {
	tcoPolicyOverrideSchema := datasourceSchemaFromResourceSchema(TCOPolicyOverrideSchema())
	tcoPolicyOverrideSchema["id"] = &schema.Schema{
		Type:     schema.TypeString,
		Required: true,
	}

	return &schema.Resource{
		ReadContext: dataSourceCoralogixTCOPolicyOverrideRead,

		Schema: tcoPolicyOverrideSchema,
	}
}

func dataSourceCoralogixTCOPolicyOverrideRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Get("id").(string)

	log.Printf("[INFO] Reading tco-policy-override %s", id)
	tcoPolicyOverride, err := meta.(*clientset.ClientSet).TCOPoliciesOverrides().GetTCOPolicyOverride(ctx, id)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "tco-policy-override", id)
	}
	log.Printf("[INFO] Received tco-policy-override: %#v", tcoPolicyOverride)

	d.SetId(id)

	return setTCOPolicyOverride(d, tcoPolicyOverride)
}
