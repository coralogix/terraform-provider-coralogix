package coralogix

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"terraform-provider-coralogix/coralogix/clientset"
)

func dataSourceCoralogixEnrichment() *schema.Resource {
	enrichmentSchema := datasourceSchemaFromResourceSchema(EnrichmentSchema())
	enrichmentSchema["id"] = &schema.Schema{
		Type:     schema.TypeString,
		Required: true,
	}

	return &schema.Resource{
		ReadContext: dataSourceCoralogixEnrichmentRead,

		Schema: enrichmentSchema,
	}
}

func dataSourceCoralogixEnrichmentRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Get("id").(string)

	log.Printf("[INFO] Reading enrichment %s", id)
	enrichmentResp, err := meta.(*clientset.ClientSet).Enrichments().GetEnrichment(ctx, strToUint32(id))
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "enrichment", id)
	}

	log.Printf("[INFO] Received enrichment: %#v", enrichmentResp)

	d.SetId(uint32ToStr(enrichmentResp.GetId()))

	return setEnrichment(d, enrichmentResp)
}
