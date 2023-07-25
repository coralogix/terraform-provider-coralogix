package coralogix

//
//import (
//	"context"
//	"log"
//
//	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
//	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
//	"terraform-provider-coralogix/coralogix/clientset"
//)
//
//func dataSourceCoralogixTCOPolicy() *schema.Resource {
//	tcoPolicySchema := datasourceSchemaFromResourceSchema(TCOPolicySchema())
//	tcoPolicySchema["id"] = &schema.Schema{
//		Type:     schema.TypeString,
//		Required: true,
//	}
//
//	return &schema.Resource{
//		ReadContext: dataSourceCoralogixTCOPolicyRead,
//
//		Schema: tcoPolicySchema,
//	}
//}
//
//func dataSourceCoralogixTCOPolicyRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
//	id := d.Get("id").(string)
//
//	log.Printf("[INFO] Reading tco-policy %s", id)
//	tcoPolicy, err := meta.(*clientset.ClientSet).TCOPolicies().GetTCOPolicy(ctx, id)
//	if err != nil {
//		log.Printf("[ERROR] Received error: %#v", err)
//		return handleRpcErrorWithID(err, "tco-policy", id)
//	}
//	log.Printf("[INFO] Received tco-policy: %#v", tcoPolicy)
//
//	d.SetId(id)
//
//	return setTCOPolicy(d, tcoPolicy)
//}
