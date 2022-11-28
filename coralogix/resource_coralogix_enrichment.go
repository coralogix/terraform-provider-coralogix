package coralogix

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"terraform-provider-coralogix/coralogix/clientset"
	enrichmentv1 "terraform-provider-coralogix/coralogix/clientset/grpc/com/coralogix/enrichment/v1"
)

var validEnrichmentTypes = []string{"geo_ip", "suspicious_ip", "aws", "custom"}

func resourceCoralogixEnrichment() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCoralogixEnrichmentCreate,
		ReadContext:   resourceCoralogixEnrichmentRead,
		UpdateContext: resourceCoralogixEnrichmentUpdate,
		DeleteContext: resourceCoralogixEnrichmentDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(60 * time.Minute),
			Read:   schema.DefaultTimeout(30 * time.Minute),
			Update: schema.DefaultTimeout(60 * time.Minute),
			Delete: schema.DefaultTimeout(30 * time.Minute),
		},

		Schema: EnrichmentSchema(),
	}
}

func EnrichmentSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"geo_ip": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"key": {
						Type:         schema.TypeString,
						Required:     true,
						ValidateFunc: validation.StringIsNotEmpty,
					},
				},
			},
			MaxItems:     1,
			ExactlyOneOf: validEnrichmentTypes,
			Description:  "Coralogix allows you to enrich your logs with location data by automatically converting IPs to Geo-points which can be used to aggregate logs by location and create Map visualizations in Kibana.",
		},
		"suspicious_ip": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"key": {
						Type:         schema.TypeString,
						Required:     true,
						ValidateFunc: validation.StringIsNotEmpty,
					},
				},
			},
			MaxItems:     1,
			ExactlyOneOf: validEnrichmentTypes,
		},
		"aws": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"key": {
						Type:         schema.TypeString,
						Required:     true,
						ValidateFunc: validation.StringIsNotEmpty,
					},
				},
			},
			MaxItems:     1,
			ExactlyOneOf: validEnrichmentTypes,
		},
		"custom": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"key": {
						Type:         schema.TypeString,
						Required:     true,
						ValidateFunc: validation.StringIsNotEmpty,
					},
				},
			},
			MaxItems:     1,
			ExactlyOneOf: validEnrichmentTypes,
		},
	}
}

func resourceCoralogixEnrichmentCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	enrichment, err := extractEnrichment(d)
	if err != nil {
		return diag.FromErr(err)
	}

	enrichmentReq := &enrichmentv1.AddEnrichmentsRequest{
		RequestEnrichments: []*enrichmentv1.EnrichmentRequestModel{enrichment},
	}

	log.Printf("[INFO] Creating new enrichment: %#v", enrichmentReq)
	enrichmentsResp, err := meta.(*clientset.ClientSet).Enrichment().CreateEnrichment(ctx, enrichmentReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcError(err)
	}
	enrichmentResp := enrichmentsResp.GetEnrichments()[0]
	log.Printf("[INFO] Submitted new enrichment: %#v", enrichmentResp)
	id := strconv.FormatUint(uint64(enrichmentResp.GetId()), 10)
	d.SetId(id)

	return resourceCoralogixEnrichmentRead(ctx, d, meta)
}

func extractEnrichment(d *schema.ResourceData) (*enrichmentv1.EnrichmentRequestModel, error) {
	d.Get("name")
	switch d.Get("") {

	}
}

func resourceCoralogixEnrichmentDelete(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {

}

func resourceCoralogixEnrichmentUpdate(ctx context.Context, data *schema.ResourceData, i interface{}) diag.Diagnostics {

}

func resourceCoralogixEnrichmentRead(ctx context.Context, data *schema.ResourceData, i interface{}) diag.Diagnostics {

}
