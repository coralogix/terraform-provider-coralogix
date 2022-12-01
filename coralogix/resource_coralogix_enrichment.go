package coralogix

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"google.golang.org/protobuf/types/known/wrapperspb"
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
					"field_name": {
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
					"field_name": {
						Type:         schema.TypeString,
						Required:     true,
						ValidateFunc: validation.StringIsNotEmpty,
					},
				},
			},
			MaxItems:     1,
			ExactlyOneOf: validEnrichmentTypes,
			Description:  "Coralogix allows you to automatically discover threats on your web servers by enriching your logs with the most updated IP blacklists.",
		},
		"aws": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"field_name": {
						Type:         schema.TypeString,
						Required:     true,
						ValidateFunc: validation.StringIsNotEmpty,
					},
					"resource_type": {
						Type:         schema.TypeString,
						Required:     true,
						ValidateFunc: validation.StringIsNotEmpty,
					},
				},
			},
			MaxItems:     1,
			ExactlyOneOf: validEnrichmentTypes,
			Description:  "Coralogix allows you to enrich your logs with the data from a chosen AWS resource. The feature enriches every log that contains a particular resourceId, associated with the metadata of a chosen AWS resource.",
		},
		"custom": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"custom_enrichment_id": {
						Type:         schema.TypeString,
						Required:     true,
						ValidateFunc: validation.StringIsNotEmpty,
					},
					"field_name": {
						Type:         schema.TypeString,
						Required:     true,
						ValidateFunc: validation.StringIsNotEmpty,
					},
				},
			},
			MaxItems:     1,
			ExactlyOneOf: validEnrichmentTypes,
			Description:  "Custom Log Enrichment with Coralogix enables you to easily enrich your log data.",
		},
	}
}

func resourceCoralogixEnrichmentCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	enrichmentReq, err := extractEnrichmentRequest(d)
	if err != nil {
		return diag.FromErr(err)
	}
	log.Printf("[INFO] Creating new enrichment: %#v", enrichmentReq)
	enrichmentResp, err := meta.(*clientset.ClientSet).Enrichments().CreateEnrichment(ctx, enrichmentReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcError(err, "enrichment")
	}
	log.Printf("[INFO] Submitted new enrichment: %#v", enrichmentResp)
	id := uint32ToStr(enrichmentResp.GetId())
	d.SetId(id)
	return resourceCoralogixEnrichmentRead(ctx, d, meta)
}

func resourceCoralogixEnrichmentRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	log.Print("[INFO] Reading enrichment")
	enrichmentResp, err := meta.(*clientset.ClientSet).Enrichments().GetEnrichment(ctx, strToUint32(id))
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "enrichment", id)
	}
	log.Printf("[INFO] Received enrichment: %#v", enrichmentResp)
	return setEnrichment(d, enrichmentResp)
}

func resourceCoralogixEnrichmentUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	enrichmentReq, err := extractEnrichmentRequest(d)
	if err != nil {
		return diag.FromErr(err)
	}
	log.Print("[INFO] Updating enrichment")
	enrichmentResp, err := meta.(*clientset.ClientSet).Enrichments().UpdateEnrichment(ctx, strToUint32(id), enrichmentReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "enrichment", id)
	}
	log.Printf("[INFO] Received enrichment: %#v", enrichmentResp)
	return setEnrichment(d, enrichmentResp)
}

func resourceCoralogixEnrichmentDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	log.Printf("[INFO] Deleting enrichment %s\n", id)
	err := meta.(*clientset.ClientSet).Enrichments().DeleteEnrichment(ctx, strToUint32(id))
	if err != nil {
		log.Printf("[ERROR] Received error: %#v\n", err)
		return handleRpcErrorWithID(err, "enrichment", id)
	}
	log.Printf("[INFO] enrichment %s deleted\n", id)

	d.SetId("")
	return nil
}

func extractEnrichmentRequest(d *schema.ResourceData) (*enrichmentv1.EnrichmentRequestModel, error) {
	if geoIp := d.Get("geo_ip").([]interface{}); len(geoIp) != 0 {
		return expandGeoIp(geoIp[0]), nil
	}
	if suspiciousIp := d.Get("suspicious_ip").([]interface{}); len(suspiciousIp) != 0 {
		return expandSuspiciousIp(suspiciousIp[0]), nil
	}
	if aws := d.Get("aws").([]interface{}); len(aws) != 0 {
		return expandAws(aws[0]), nil
	}
	if custom := d.Get("custom").([]interface{}); len(custom) != 0 {
		return expandCustom(custom[0]), nil
	}

	return nil, fmt.Errorf("not valid enrichment")
}

func setEnrichment(d *schema.ResourceData, enrichment *enrichmentv1.Enrichment) diag.Diagnostics {
	enrichmentType := enrichment.GetEnrichmentType().GetType()
	fieldName := enrichment.GetFieldName()
	var enrichmentTypeStr string
	var flattenedEnrichment interface{}
	switch enrichmentType.(type) {
	case *enrichmentv1.EnrichmentType_Aws:
		enrichmentTypeStr = "aws"
		flattenedEnrichment = flattenAwsEnrichment(enrichmentType.(*enrichmentv1.EnrichmentType_Aws).Aws, fieldName)
	case *enrichmentv1.EnrichmentType_GeoIp:
		enrichmentTypeStr = "geo_ip"
		flattenedEnrichment = flattenGeoIpEnrichment(fieldName)
	case *enrichmentv1.EnrichmentType_SuspiciousIp:
		enrichmentTypeStr = "suspicious_ip"
		flattenedEnrichment = flattenSuspiciousIpEnrichment(fieldName)
	case *enrichmentv1.EnrichmentType_CustomEnrichment:
		enrichmentTypeStr = "custom"
		flattenedEnrichment = flattenCustomEnrichment(enrichmentType.(*enrichmentv1.EnrichmentType_CustomEnrichment).CustomEnrichment, fieldName)
	default:
		return diag.Errorf("unexpected enrichment type %s", enrichment.GetEnrichmentType().String())
	}

	if err := d.Set(enrichmentTypeStr, []interface{}{flattenedEnrichment}); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func flattenAwsEnrichment(e *enrichmentv1.AwsType, fieldName string) interface{} {
	return map[string]interface{}{
		"field_name":    fieldName,
		"resource_type": e.GetResourceType().GetValue(),
	}
}

func flattenGeoIpEnrichment(fieldName string) interface{} {
	return map[string]interface{}{
		"field_name": fieldName,
	}
}

func flattenSuspiciousIpEnrichment(fieldName string) interface{} {
	return map[string]interface{}{
		"field_name": fieldName,
	}
}

func flattenCustomEnrichment(e *enrichmentv1.CustomEnrichmentType, fieldName string) interface{} {
	return map[string]interface{}{
		"custom_enrichment_id": uint32ToStr(e.GetId().GetValue()),
		"field_name":           fieldName,
	}
}

func expandGeoIp(v interface{}) *enrichmentv1.EnrichmentRequestModel {
	m := v.(map[string]interface{})
	fieldName := wrapperspb.String(m["field_name"].(string))
	return &enrichmentv1.EnrichmentRequestModel{
		FieldName: fieldName,
		EnrichmentType: &enrichmentv1.EnrichmentType{
			Type: &enrichmentv1.EnrichmentType_GeoIp{
				GeoIp: &enrichmentv1.GeoIpType{},
			},
		},
	}
}

func expandSuspiciousIp(v interface{}) *enrichmentv1.EnrichmentRequestModel {
	m := v.(map[string]interface{})
	fieldName := wrapperspb.String(m["field_name"].(string))
	return &enrichmentv1.EnrichmentRequestModel{
		FieldName: fieldName,
		EnrichmentType: &enrichmentv1.EnrichmentType{
			Type: &enrichmentv1.EnrichmentType_SuspiciousIp{
				SuspiciousIp: &enrichmentv1.SuspiciousIpType{},
			},
		},
	}
}

func expandAws(v interface{}) *enrichmentv1.EnrichmentRequestModel {
	m := v.(map[string]interface{})
	fieldName := wrapperspb.String(m["field_name"].(string))
	return &enrichmentv1.EnrichmentRequestModel{
		FieldName: fieldName,
		EnrichmentType: &enrichmentv1.EnrichmentType{
			Type: &enrichmentv1.EnrichmentType_Aws{
				Aws: &enrichmentv1.AwsType{},
			},
		},
	}
}

func expandCustom(v interface{}) *enrichmentv1.EnrichmentRequestModel {
	m := v.(map[string]interface{})
	fieldName := wrapperspb.String(m["field_name"].(string))
	id := wrapperspb.UInt32(strToUint32(m["custom_enrichment_id"].(string)))
	return &enrichmentv1.EnrichmentRequestModel{
		FieldName: fieldName,
		EnrichmentType: &enrichmentv1.EnrichmentType{
			Type: &enrichmentv1.EnrichmentType_CustomEnrichment{
				CustomEnrichment: &enrichmentv1.CustomEnrichmentType{
					Id: id,
				},
			},
		},
	}
}
