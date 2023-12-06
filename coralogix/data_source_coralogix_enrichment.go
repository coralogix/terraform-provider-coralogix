package coralogix

import (
	"context"
	"log"

	"terraform-provider-coralogix/coralogix/clientset"
	enrichmentv1 "terraform-provider-coralogix/coralogix/clientset/grpc/enrichment/v1"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
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
	log.Print("[INFO] Reading enrichment")
	var enrichmentResp []*enrichmentv1.Enrichment
	var err error
	var enrichmentType string
	if id == "geo_ip" || id == "suspicious_ip" || id == "aws" {
		enrichmentType = id
		enrichmentResp, err = meta.(*clientset.ClientSet).Enrichments().GetEnrichmentsByType(ctx, id)
	} else {
		enrichmentType = "custom"
		enrichmentResp, err = meta.(*clientset.ClientSet).Enrichments().GetCustomEnrichments(ctx, strToUint32(id))
	}
	if err != nil {
		reqStr := protojson.Format(&enrichmentv1.GetEnrichmentsRequest{})
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcError(err, "enrichment", reqStr)
	}
	log.Printf("[INFO] Received enrichment: %#v", enrichmentResp)
	d.SetId(id)
	return setEnrichment(d, enrichmentType, enrichmentResp)
}
