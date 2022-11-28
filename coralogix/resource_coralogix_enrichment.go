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
	alertsv1 "terraform-provider-coralogix/coralogix/clientset/grpc/com/coralogix/alerts/v1"
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
					"field_name": {
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
	enrichmentsResp, err := meta.(*clientset.ClientSet).Enrichments().CreateEnrichment(ctx, enrichmentReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcError(err)
	}
	enrichmentResp := enrichmentsResp.GetEnrichments()[0]
	log.Printf("[INFO] Submitted new enrichment: %#v", enrichmentResp)
	id := extractEnrichmentId(enrichmentResp)
	d.SetId(id)

	return resourceCoralogixEnrichmentRead(ctx, d, meta)
}

func extractEnrichmentId(enrichment *enrichmentv1.Enrichment) string {
	enrichmentTypeStr := enrichmentTypeStr(enrichment.GetEnrichmentType())
	return fmt.Sprintf("%s:%s", enrichmentTypeStr, enrichment.FieldName)
}

func enrichmentTypeStr(enrichmentType *enrichmentv1.EnrichmentType) string {
	switch enrichmentType.GetType().(type) {
	case *enrichmentv1.EnrichmentType_GeoIp:
		return "GeoIp"
	case *enrichmentv1.EnrichmentType_SuspiciousIp:
		return "SuspiciousIp"
	case *enrichmentv1.EnrichmentType_Aws:
		return "Aws"
	case *enrichmentv1.EnrichmentType_CustomEnrichment:
		return "Custom"
	}
	return ""
}

func resourceCoralogixEnrichmentRead(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := wrapperspb.String(data.Id())
	log.Print("[INFO] Reading enrichments")
	enrichmentsResp, err := meta.(*clientset.ClientSet).Enrichments().GetEnrichments(ctx, &enrichmentv1.GetEnrichmentsRequest{})
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcError(err, "enrichments")
	}
	enrichments := enrichmentsResp.GetEnrichments()
	log.Printf("[INFO] Received enrichments: %#v", enrichments)

	return setEnrichment(data, enrichment)
}

func extractEnrichment(d *schema.ResourceData) (*enrichmentv1.EnrichmentRequestModel, error) {
	if geoIp := d.Get("geo_ip").([]interface{}); len(geoIp) != 0 {
		return expandGeoIp(geoIp[0])
	}
	if suspiciousIp := d.Get("suspicious_ip").([]interface{}); len(suspiciousIp) != 0 {
		return expandSuspiciousIp(suspiciousIp[0])
	}
	if aws := d.Get("aws").([]interface{}); len(aws) != 0 {
		return expandAws(aws[0])
	}
	if custom := d.Get("custom").([]interface{}); len(custom) != 0 {
		return expandCustom(custom[0])
	}
	return nil, fmt.Errorf("not valid enrichment")
}

func setEnrichment(data *schema.ResourceData, enrichment *enrichmentv1.Enrichment) diag.Diagnostics {

}

func expandGeoIp(v interface{}) (*enrichmentv1.EnrichmentRequestModel, error) {
	m := v.(map[string]interface{})
	fieldName := wrapperspb.String(m["field_name"].(string))
	return &enrichmentv1.EnrichmentRequestModel{
		FieldName: fieldName,
		EnrichmentType: &enrichmentv1.EnrichmentType{
			Type: &enrichmentv1.EnrichmentType_GeoIp{
				GeoIp: &enrichmentv1.GeoIpType{},
			},
		},
	}, nil
}

func expandSuspiciousIp(v interface{}) (*enrichmentv1.EnrichmentRequestModel, error) {
	m := v.(map[string]interface{})
	fieldName := wrapperspb.String(m["field_name"].(string))
	return &enrichmentv1.EnrichmentRequestModel{
		FieldName: fieldName,
		EnrichmentType: &enrichmentv1.EnrichmentType{
			Type: &enrichmentv1.EnrichmentType_SuspiciousIp{
				SuspiciousIp: &enrichmentv1.SuspiciousIpType{},
			},
		},
	}, nil
}

func expandAws(v interface{}) (*enrichmentv1.EnrichmentRequestModel, error) {
	m := v.(map[string]interface{})
	fieldName := wrapperspb.String(m["field_name"].(string))
	return &enrichmentv1.EnrichmentRequestModel{
		FieldName: fieldName,
		EnrichmentType: &enrichmentv1.EnrichmentType{
			Type: &enrichmentv1.EnrichmentType_Aws{
				Aws: &enrichmentv1.AwsType{},
			},
		},
	}, nil
}

func expandCustom(v interface{}) (*enrichmentv1.EnrichmentRequestModel, error) {
	m := v.(map[string]interface{})
	fieldName := wrapperspb.String(m["field_name"].(string))
	id := wrapperspb.UInt32(uint32(m["id"].(int)))
	return &enrichmentv1.EnrichmentRequestModel{
		FieldName: fieldName,
		EnrichmentType: &enrichmentv1.EnrichmentType{
			Type: &enrichmentv1.EnrichmentType_CustomEnrichment{
				CustomEnrichment: &enrichmentv1.CustomEnrichmentType{
					Id: id,
				},
			},
		},
	}, nil
}

func resourceCoralogixEnrichmentDelete(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := wrapperspb.String(d.Id())
	deleteAlertRequest := &alertsv1.DeleteAlertRequest{
		Id: id,
	}

	log.Printf("[INFO] Deleting alert %s\n", id)
	_, err := meta.(*clientset.ClientSet).Alerts().DeleteAlert(ctx, deleteAlertRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v\n", err)
		return handleRpcErrorWithID(err, "alert", id.GetValue())
	}
	log.Printf("[INFO] alert %s deleted\n", id)

	d.SetId("")
	return nil
}

func resourceCoralogixEnrichmentUpdate(ctx context.Context, data *schema.ResourceData, i interface{}) diag.Diagnostics {
	req, err := extractAlert(d)
	if err != nil {
		return diag.FromErr(err)
	}

	id := d.Id()
	updateAlertRequest := &alertsv1.UpdateAlertRequest{
		Alert: req,
	}

	log.Printf("[INFO] Updating alert %s", updateAlertRequest)
	alertResp, err := meta.(*clientset.ClientSet).Alerts().UpdateAlert(ctx, updateAlertRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "alert", id)
	}
	log.Printf("[INFO] Submitted updated alert: %#v", alertResp)
	d.SetId(alertResp.GetAlert().GetId().GetValue())

	return resourceCoralogixAlertRead(ctx, d, meta)
}
