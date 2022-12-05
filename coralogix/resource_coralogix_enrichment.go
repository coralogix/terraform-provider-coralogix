package coralogix

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
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
					"fields": {
						Type:     schema.TypeSet,
						Optional: true,
						Elem:     fields(),
						Set:      hashFields(),
						//Description: fmt.Sprintf("An array of log severities that we interested in. Can be one of %q", alertValidLogSeverities),
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
					"fields": {
						Type:     schema.TypeSet,
						Optional: true,
						Elem:     fields(),
						Set:      hashFields(),
						//Description: fmt.Sprintf("An array of log severities that we interested in. Can be one of %q", alertValidLogSeverities),
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
					"fields": {
						Type:     schema.TypeSet,
						Optional: true,
						Elem:     awsFields(),
						Set:      hashAwsFields(),
						//Description: fmt.Sprintf("An array of log severities that we interested in. Can be one of %q", alertValidLogSeverities),
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
						Type:     schema.TypeInt,
						Required: true,
					},
					"fields": {
						Type:     schema.TypeSet,
						Optional: true,
						Elem:     fields(),
						Set:      hashFields(),
						//Description: fmt.Sprintf("An array of log severities that we interested in. Can be one of %q", alertValidLogSeverities),
					},
				},
			},
			MaxItems:     1,
			ExactlyOneOf: validEnrichmentTypes,
			Description:  "Custom Log Enrichment with Coralogix enables you to easily enrich your log data.",
		},
	}
}

func fields() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"id": {
				Type:     schema.TypeInt,
				Computed: true,
			},
		},
	}
}

func hashFields() schema.SchemaSetFunc {
	return schema.HashResource(fields())
}

func awsFields() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"resource": {
				Type:     schema.TypeString,
				Required: true,
			},
			"id": {
				Type:     schema.TypeInt,
				Computed: true,
			},
		},
	}
}

func hashAwsFields() schema.SchemaSetFunc {
	return schema.HashResource(awsFields())
}

func resourceCoralogixEnrichmentCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	enrichmentReq, enrichmentTypeOrCustomId, err := extractEnrichmentRequest(d)
	if err != nil {
		return diag.FromErr(err)
	}
	log.Printf("[INFO] Creating new enrichment: %#v", enrichmentReq)
	enrichmentResp, err := meta.(*clientset.ClientSet).Enrichments().CreateEnrichments(ctx, enrichmentReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcError(err, "enrichment")
	}
	log.Printf("[INFO] Submitted new enrichment: %#v", enrichmentResp)
	d.SetId(enrichmentTypeOrCustomId)
	return resourceCoralogixEnrichmentRead(ctx, d, meta)
}

func resourceCoralogixEnrichmentRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	enrichmentType, customId := extractEnrichmentTypeAndCustomId(d)
	log.Print("[INFO] Reading enrichment")
	var enrichmentResp []*enrichmentv1.Enrichment
	var err error
	if customId == "" {
		enrichmentResp, err = meta.(*clientset.ClientSet).Enrichments().GetEnrichmentsByType(ctx, enrichmentType)
	} else {
		enrichmentResp, err = meta.(*clientset.ClientSet).Enrichments().GetCustomEnrichments(ctx, strToUint32(customId))
	}
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcError(err, "enrichment")
	}
	log.Printf("[INFO] Received enrichment: %#v", enrichmentResp)
	return setEnrichment(d, enrichmentType, enrichmentResp)
}

func extractEnrichmentTypeAndCustomId(d *schema.ResourceData) (string, string) {
	if id := d.Id(); id == "geo_ip" || id == "suspicious_ip" || id == "aws" {
		return id, ""
	} else {
		return "custom", id
	}
}

func extractIdsFromEnrichment(d *schema.ResourceData) []uint32 {
	var v interface{}
	if geoIp := d.Get("geo_ip").([]interface{}); len(geoIp) != 0 {
		v = geoIp[0]
	}
	if suspiciousIp := d.Get("suspicious_ip").([]interface{}); len(suspiciousIp) != 0 {
		v = suspiciousIp[0]
	}
	if aws := d.Get("aws").([]interface{}); len(aws) != 0 {
		v = aws[0]
	}
	if custom := d.Get("custom").([]interface{}); len(custom) != 0 {
		v = custom[0]
	}
	m := v.(map[string]interface{})
	fields := m["fields"].(*schema.Set).List()
	result := make([]uint32, 0, len(fields))
	for _, field := range fields {
		id := uint32(field.(map[string]interface{})["id"].(int))
		result = append(result, id)
	}
	return result
}

func resourceCoralogixEnrichmentUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	ids := extractIdsFromEnrichment(d)
	enrichmentReq, _, err := extractEnrichmentRequest(d)
	if err != nil {
		return diag.FromErr(err)
	}
	log.Print("[INFO] Updating enrichment")
	enrichmentResp, err := meta.(*clientset.ClientSet).Enrichments().UpdateEnrichments(ctx, ids, enrichmentReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcError(err, "enrichment")
	}
	log.Printf("[INFO] Received enrichment: %#v", enrichmentResp)
	return resourceCoralogixEnrichmentRead(ctx, d, meta)
}

func resourceCoralogixEnrichmentDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	var err error
	log.Printf("[INFO] Deleting enrichment %s\n", id)
	if id == "geo_ip" || id == "suspicious_ip" || id == "aws" {
		err = meta.(*clientset.ClientSet).Enrichments().DeleteEnrichmentsByType(ctx, id)
	} else {
		ids := extractIdsFromEnrichment(d)
		err = meta.(*clientset.ClientSet).Enrichments().DeleteEnrichments(ctx, ids)
	}
	if err != nil {
		log.Printf("[ERROR] Received error: %#v\n", err)
		return handleRpcError(err, "enrichment")
	}
	log.Printf("[INFO] enrichment %s deleted\n", id)

	d.SetId("")
	return nil
}

func extractEnrichmentRequest(d *schema.ResourceData) ([]*enrichmentv1.EnrichmentRequestModel, string, error) {
	if geoIp := d.Get("geo_ip").([]interface{}); len(geoIp) != 0 {
		return expandGeoIp(geoIp[0]), "geo_ip", nil
	}
	if suspiciousIp := d.Get("suspicious_ip").([]interface{}); len(suspiciousIp) != 0 {
		return expandSuspiciousIp(suspiciousIp[0]), "suspicious_ip", nil
	}
	if aws := d.Get("aws").([]interface{}); len(aws) != 0 {
		return expandAws(aws[0]), "aws", nil
	}
	if custom := d.Get("custom").([]interface{}); len(custom) != 0 {
		enrichment, customId := expandCustom(custom[0])
		return enrichment, customId, nil
	}

	return nil, "", fmt.Errorf("not valid enrichment")
}

func setEnrichment(d *schema.ResourceData, enrichmentType string, enrichments []*enrichmentv1.Enrichment) diag.Diagnostics {
	var flattenedEnrichment interface{}
	switch enrichmentType {
	case "aws":
		flattenedEnrichment =
			map[string]interface{}{
				"fields": flattenAwsEnrichment(enrichments),
			}
	case "geo_ip":
		flattenedEnrichment = map[string]interface{}{
			"fields": flattenEnrichment(enrichments),
		}
	case "suspicious_ip":
		flattenedEnrichment = map[string]interface{}{
			"fields": flattenEnrichment(enrichments),
		}
	case "custom":
		flattenedEnrichment = map[string]interface{}{
			"custom_enrichment_id": int(strToUint32(d.Id())),
			"fields":               flattenEnrichment(enrichments),
		}
	default:
		return diag.Errorf("unexpected enrichment type %s", enrichmentType)
	}

	if err := d.Set(enrichmentType, []interface{}{flattenedEnrichment}); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func flattenAwsEnrichment(enrichments []*enrichmentv1.Enrichment) interface{} {
	result := schema.NewSet(hashAwsFields(), []interface{}{})
	for _, e := range enrichments {
		m := map[string]interface{}{
			"name":     e.GetFieldName(),
			"resource": e.GetEnrichmentType().GetType().(*enrichmentv1.EnrichmentType_Aws).Aws.GetResourceType().GetValue(),
			"id":       int(e.GetId()),
		}
		result.Add(m)
	}
	return result
}

func flattenEnrichment(enrichments []*enrichmentv1.Enrichment) interface{} {
	result := schema.NewSet(hashFields(), []interface{}{})
	for _, e := range enrichments {
		m := map[string]interface{}{
			"name": e.GetFieldName(),
			"id":   int(e.GetId()),
		}
		result.Add(m)
	}
	return result
}

func expandGeoIp(v interface{}) []*enrichmentv1.EnrichmentRequestModel {
	m := v.(map[string]interface{})
	fields := m["fields"].(*schema.Set).List()
	result := make([]*enrichmentv1.EnrichmentRequestModel, 0, len(fields))

	for _, field := range fields {
		fieldName := wrapperspb.String(field.(map[string]interface{})["name"].(string))
		e := &enrichmentv1.EnrichmentRequestModel{
			FieldName: fieldName,
			EnrichmentType: &enrichmentv1.EnrichmentType{
				Type: &enrichmentv1.EnrichmentType_GeoIp{
					GeoIp: &enrichmentv1.GeoIpType{},
				},
			},
		}
		result = append(result, e)
	}

	return result
}

func expandSuspiciousIp(v interface{}) []*enrichmentv1.EnrichmentRequestModel {
	m := v.(map[string]interface{})
	fields := m["fields"].(*schema.Set).List()
	result := make([]*enrichmentv1.EnrichmentRequestModel, 0, len(fields))

	for _, field := range fields {
		fieldName := wrapperspb.String(field.(map[string]interface{})["name"].(string))
		e := &enrichmentv1.EnrichmentRequestModel{
			FieldName: fieldName,
			EnrichmentType: &enrichmentv1.EnrichmentType{
				Type: &enrichmentv1.EnrichmentType_SuspiciousIp{
					SuspiciousIp: &enrichmentv1.SuspiciousIpType{},
				},
			},
		}
		result = append(result, e)
	}

	return result
}

func expandAws(v interface{}) []*enrichmentv1.EnrichmentRequestModel {
	m := v.(map[string]interface{})
	fields := m["fields"].(*schema.Set).List()
	result := make([]*enrichmentv1.EnrichmentRequestModel, 0, len(fields))

	for _, field := range fields {
		m := field.(map[string]interface{})
		fieldName := wrapperspb.String(m["name"].(string))
		resourceType := wrapperspb.String(m["resource_type"].(string))

		e := &enrichmentv1.EnrichmentRequestModel{
			FieldName: fieldName,
			EnrichmentType: &enrichmentv1.EnrichmentType{
				Type: &enrichmentv1.EnrichmentType_Aws{
					Aws: &enrichmentv1.AwsType{
						ResourceType: resourceType,
					},
				},
			},
		}
		result = append(result, e)
	}

	return result
}

func expandCustom(v interface{}) ([]*enrichmentv1.EnrichmentRequestModel, string) {
	m := v.(map[string]interface{})
	fields := m["fields"].(*schema.Set).List()
	uintId := uint32(m["custom_enrichment_id"].(int))
	id := wrapperspb.UInt32(uintId)
	result := make([]*enrichmentv1.EnrichmentRequestModel, 0, len(fields))

	for _, field := range fields {
		m := field.(map[string]interface{})
		fieldName := wrapperspb.String(m["name"].(string))

		e := &enrichmentv1.EnrichmentRequestModel{
			FieldName: fieldName,
			EnrichmentType: &enrichmentv1.EnrichmentType{
				Type: &enrichmentv1.EnrichmentType_CustomEnrichment{
					CustomEnrichment: &enrichmentv1.CustomEnrichmentType{
						Id: id,
					},
				},
			},
		}
		result = append(result, e)
	}

	return result, uint32ToStr(uintId)
}
