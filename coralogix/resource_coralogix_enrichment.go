// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package coralogix

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"terraform-provider-coralogix/coralogix/clientset"

	"google.golang.org/protobuf/encoding/protojson"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func EnrichmentsByID(ctx context.Context, client *cxsdk.EnrichmentsClient, customEnrichmentID uint32) ([]*cxsdk.Enrichment, error) {
	resp, err := client.List(ctx, &cxsdk.GetEnrichmentsRequest{})
	if err != nil {
		return nil, err
	}

	log.Printf("[INFO] Received custom enrichment: %s", protojson.Format(resp))
	result := make([]*cxsdk.Enrichment, 0)
	for _, enrichment := range resp.GetEnrichments() {
		if customEnrichment := enrichment.GetEnrichmentType().GetCustomEnrichment(); customEnrichment != nil && customEnrichment.GetId().GetValue() == customEnrichmentID {
			result = append(result, enrichment)
		}
	}
	log.Printf("[INFO] found %s enrichments for ID %s", len(result), customEnrichmentID)
	return result, nil
}

func EnrichmentsByType(ctx context.Context, client *cxsdk.EnrichmentsClient, enrichmentType string) ([]*cxsdk.Enrichment, error) {
	resp, err := client.List(ctx, &cxsdk.GetEnrichmentsRequest{})
	if err != nil {
		return nil, err
	}
	log.Printf("[INFO] Received custom enrichment: %s", protojson.Format(resp))

	result := make([]*cxsdk.Enrichment, 0)
	for _, enrichment := range resp.GetEnrichments() {
		if enrichment.GetEnrichmentType().String() == enrichmentType+":{}" {
			result = append(result, enrichment)
		}
	}
	log.Printf("[INFO] found %s enrichments for type %s", len(result), enrichmentType)

	return result, nil
}

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
			Create: schema.DefaultTimeout(60 * time.Second),
			Read:   schema.DefaultTimeout(30 * time.Second),
			Update: schema.DefaultTimeout(60 * time.Second),
			Delete: schema.DefaultTimeout(30 * time.Second),
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
						Type:        schema.TypeSet,
						Optional:    true,
						Elem:        fields(),
						Set:         hashFields(),
						Description: "Set of fields to enrich with geo_ip information.",
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
						Type:        schema.TypeSet,
						Optional:    true,
						Elem:        fields(),
						Set:         hashFields(),
						Description: "Set of fields to enrich with suspicious_ip information.",
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
						Type:        schema.TypeSet,
						Optional:    true,
						Elem:        awsFields(),
						Set:         hashAwsFields(),
						Description: "Set of fields to enrich with aws information.",
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
						Type:        schema.TypeSet,
						Optional:    true,
						Elem:        fields(),
						Set:         hashFields(),
						Description: "Set of fields to enrich with the custom information.",
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
	createReq := &cxsdk.AddEnrichmentsRequest{RequestEnrichments: enrichmentReq}
	log.Printf("[INFO] Creating new enrichment: %s", protojson.Format(createReq))
	enrichmentResp, err := meta.(*clientset.ClientSet).Enrichments().Add(ctx, createReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		return diag.Errorf(formatRpcErrors(err, cxsdk.AddEnrichmentsRPC, protojson.Format(createReq)))
	}
	log.Printf("[INFO] Submitted new enrichment: %s", enrichmentResp)
	d.SetId(enrichmentTypeOrCustomId)
	return resourceCoralogixEnrichmentRead(ctx, d, meta)
}

func resourceCoralogixEnrichmentRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	enrichmentType, customId := extractEnrichmentTypeAndCustomId(d)
	log.Printf("[INFO] Reading enrichment %s", customId)
	var enrichments []*cxsdk.Enrichment
	var err error
	if customId == "" {
		enrichments, err = EnrichmentsByType(ctx, meta.(*clientset.ClientSet).Enrichments(), enrichmentType)
	} else {
		customIdParsed, err := strconv.ParseUint(customId, 10, 32)
		if err != nil {
			return diag.Errorf("failed to parse custom_enrichment_id %s: %s", customId, err)
		}
		enrichments, err = EnrichmentsByID(ctx, meta.(*clientset.ClientSet).Enrichments(), uint32(customIdParsed))
	}

	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if customId != "" && status.Code(err) == codes.NotFound {
			d.SetId("")
			return diag.Diagnostics{diag.Diagnostic{
				Severity: diag.Warning,
				Summary:  fmt.Sprintf("Enrichment %q is in state, but no longer exists in Coralogix backend", customId),
				Detail:   fmt.Sprintf("%s will be recreated when you apply", customId),
			}}
		}
		return diag.Errorf(formatRpcErrors(err, cxsdk.GetEnrichmentsRPC, protojson.Format(&cxsdk.GetEnrichmentsRequest{})))
	}
	return setEnrichment(d, enrichmentType, enrichments)
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
	deleteReq := &cxsdk.DeleteEnrichmentsRequest{EnrichmentIds: uint32SliceToWrappedUint32Slice(ids)}
	if err = meta.(*clientset.ClientSet).Enrichments().Delete(ctx, deleteReq); err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		return diag.Errorf(formatRpcErrors(err, cxsdk.DeleteEnrichmentsRPC, protojson.Format(deleteReq)))
	}
	createReq := &cxsdk.AddEnrichmentsRequest{RequestEnrichments: enrichmentReq}
	enrichmentResp, err := meta.(*clientset.ClientSet).Enrichments().Add(ctx, createReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		return diag.Errorf(formatRpcErrors(err, cxsdk.AddEnrichmentsRPC, protojson.Format(createReq)))
	}
	log.Printf("[INFO] Received enrichment: %s", enrichmentResp)
	return resourceCoralogixEnrichmentRead(ctx, d, meta)
}

func resourceCoralogixEnrichmentDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	log.Printf("[INFO] Deleting enrichment %s", id)
	if id == "geo_ip" || id == "suspicious_ip" || id == "aws" {
		enrichments, err := EnrichmentsByType(ctx, meta.(*clientset.ClientSet).Enrichments(), id)
		if err != nil {
			log.Printf("[ERROR] Received error: %s", err.Error())
			return diag.Errorf(formatRpcErrors(err, cxsdk.GetEnrichmentsRPC, protojson.Format(&cxsdk.GetEnrichmentsRequest{})))
		}
		enrichmentIds := make([]*wrapperspb.UInt32Value, 0, len(enrichments))
		for _, enrichment := range enrichments {
			enrichmentIds = append(enrichmentIds, wrapperspb.UInt32(enrichment.GetId()))
		}
		deleteReq := &cxsdk.DeleteEnrichmentsRequest{EnrichmentIds: enrichmentIds}
		if err = meta.(*clientset.ClientSet).Enrichments().Delete(ctx, deleteReq); err != nil {
			log.Printf("[ERROR] Received error: %s", err.Error())
			return diag.Errorf(formatRpcErrors(err, cxsdk.DeleteEnrichmentsRPC, protojson.Format(deleteReq)))
		}
	} else {
		ids := extractIdsFromEnrichment(d)
		deleteReq := &cxsdk.DeleteEnrichmentsRequest{EnrichmentIds: uint32SliceToWrappedUint32Slice(ids)}
		if err := meta.(*clientset.ClientSet).Enrichments().Delete(ctx, deleteReq); err != nil {
			log.Printf("[ERROR] Received error: %s", err.Error())
			return diag.Errorf(formatRpcErrors(err, cxsdk.DeleteEnrichmentsRPC, protojson.Format(deleteReq)))
		}
	}

	log.Printf("[INFO] enrichment %s deleted", id)

	d.SetId("")
	return nil
}

func extractEnrichmentRequest(d *schema.ResourceData) ([]*cxsdk.EnrichmentRequestModel, string, error) {
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

func setEnrichment(d *schema.ResourceData, enrichmentType string, enrichments []*cxsdk.Enrichment) diag.Diagnostics {
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

func flattenAwsEnrichment(enrichments []*cxsdk.Enrichment) interface{} {
	result := schema.NewSet(hashAwsFields(), []interface{}{})
	for _, e := range enrichments {
		m := map[string]interface{}{
			"name":     e.GetFieldName(),
			"resource": e.GetEnrichmentType().GetType().(*cxsdk.EnrichmentTypeAws).Aws.GetResourceType().GetValue(),
			"id":       int(e.GetId()),
		}
		result.Add(m)
	}
	return result
}

func flattenEnrichment(enrichments []*cxsdk.Enrichment) interface{} {
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

func expandGeoIp(v interface{}) []*cxsdk.EnrichmentRequestModel {
	m := v.(map[string]interface{})
	fields := m["fields"].(*schema.Set).List()
	result := make([]*cxsdk.EnrichmentRequestModel, 0, len(fields))

	for _, field := range fields {
		fieldName := wrapperspb.String(field.(map[string]interface{})["name"].(string))
		e := &cxsdk.EnrichmentRequestModel{
			FieldName: fieldName,
			EnrichmentType: &cxsdk.EnrichmentType{
				Type: &cxsdk.EnrichmentTypeGeoIP{
					GeoIp: &cxsdk.GeoIPType{},
				},
			},
		}
		result = append(result, e)
	}

	return result
}

func expandSuspiciousIp(v interface{}) []*cxsdk.EnrichmentRequestModel {
	m := v.(map[string]interface{})
	fields := m["fields"].(*schema.Set).List()
	result := make([]*cxsdk.EnrichmentRequestModel, 0, len(fields))

	for _, field := range fields {
		fieldName := wrapperspb.String(field.(map[string]interface{})["name"].(string))
		e := &cxsdk.EnrichmentRequestModel{
			FieldName: fieldName,
			EnrichmentType: &cxsdk.EnrichmentType{
				Type: &cxsdk.EnrichmentTypeSuspiciousIP{
					SuspiciousIp: &cxsdk.SuspiciousIPType{},
				},
			},
		}
		result = append(result, e)
	}

	return result
}

func expandAws(v interface{}) []*cxsdk.EnrichmentRequestModel {
	m := v.(map[string]interface{})
	fields := m["fields"].(*schema.Set).List()
	result := make([]*cxsdk.EnrichmentRequestModel, 0, len(fields))

	for _, field := range fields {
		m := field.(map[string]interface{})
		fieldName := wrapperspb.String(m["name"].(string))
		resourceType := wrapperspb.String(m["resource_type"].(string))

		e := &cxsdk.EnrichmentRequestModel{
			FieldName: fieldName,
			EnrichmentType: &cxsdk.EnrichmentType{
				Type: &cxsdk.EnrichmentTypeAws{
					Aws: &cxsdk.AwsType{
						ResourceType: resourceType,
					},
				},
			},
		}
		result = append(result, e)
	}

	return result
}

func expandCustom(v interface{}) ([]*cxsdk.EnrichmentRequestModel, string) {
	m := v.(map[string]interface{})
	fields := m["fields"].(*schema.Set).List()
	uintId := uint32(m["custom_enrichment_id"].(int))
	id := wrapperspb.UInt32(uintId)
	result := make([]*cxsdk.EnrichmentRequestModel, 0, len(fields))

	for _, field := range fields {
		m := field.(map[string]interface{})
		fieldName := wrapperspb.String(m["name"].(string))

		e := &cxsdk.EnrichmentRequestModel{
			FieldName: fieldName,
			EnrichmentType: &cxsdk.EnrichmentType{
				Type: &cxsdk.EnrichmentTypeCustomEnrichment{
					CustomEnrichment: &cxsdk.CustomEnrichmentType{
						Id: id,
					},
				},
			},
		}
		result = append(result, e)
	}

	return result, uint32ToStr(uintId)
}
