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

package enrichment_rules

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"google.golang.org/protobuf/encoding/protojson"

	"google.golang.org/grpc/codes"

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
	log.Printf("[INFO] found %v enrichments for ID %v", len(result), customEnrichmentID)
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
		log.Printf("[INFO] Checking %v", enrichment.GetEnrichmentType().String())

		if strings.Split(enrichment.GetEnrichmentType().String(), ":")[0] == enrichmentType {
			result = append(result, enrichment)
		}
	}
	log.Printf("[INFO] found %v enrichments for type %v", len(result), enrichmentType)

	return result, nil
}

var validEnrichmentTypes = []string{"geo_ip", "suspicious_ip", "aws", "custom"}

func ResourceCoralogixEnrichment() *schema.Resource {
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
		DeprecationMessage: "This resource is deprecated and will be removed in a future version. Please use `coralogix_data_enrichments` instead.",
		Description:        "**DEPRECATED**. Please use `coralogix_data_enrichments` instead.",
		Schema:             EnrichmentSchema(),
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
		return diag.Errorf("%s", utils.FormatRpcErrors(err, cxsdk.AddEnrichmentsRPC, protojson.Format(createReq)))
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
		enrichments, err = EnrichmentsByID(ctx, meta.(*clientset.ClientSet).Enrichments(), utils.StrToUint32(customId))
	}

	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if customId != "" && cxsdk.Code(err) == codes.NotFound {
			d.SetId("")
			return diag.Diagnostics{diag.Diagnostic{
				Severity: diag.Warning,
				Summary:  fmt.Sprintf("Enrichment %q is in state, but no longer exists in Coralogix backend", customId),
				Detail:   fmt.Sprintf("%s will be recreated when you apply", customId),
			}}
		}
		return diag.Errorf("%s", utils.FormatRpcErrors(err, cxsdk.GetEnrichmentsRPC, protojson.Format(&cxsdk.GetEnrichmentsRequest{})))
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

// extractOldIdsFromEnrichment mirrors extractIdsFromEnrichment but sources the
// enrichment fields from the *old* (pre-change) configuration via d.GetChange.
// The Update flow uses these IDs to decide what to delete from the backend:
// deletion must be driven by the pre-change set because emptying or shrinking a
// fields set otherwise makes d.Get return the (empty/smaller) proposed set and
// skip the Delete, leaving the old backend enrichments orphaned.
func extractOldIdsFromEnrichment(d *schema.ResourceData) []uint32 {
	var v interface{}
	if oldGeoIp, _ := d.GetChange("geo_ip"); len(oldGeoIp.([]interface{})) != 0 {
		v = oldGeoIp.([]interface{})[0]
	}
	if oldSuspiciousIp, _ := d.GetChange("suspicious_ip"); len(oldSuspiciousIp.([]interface{})) != 0 {
		v = oldSuspiciousIp.([]interface{})[0]
	}
	if oldAws, _ := d.GetChange("aws"); len(oldAws.([]interface{})) != 0 {
		v = oldAws.([]interface{})[0]
	}
	if oldCustom, _ := d.GetChange("custom"); len(oldCustom.([]interface{})) != 0 {
		v = oldCustom.([]interface{})[0]
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	fields := m["fields"].(*schema.Set).List()
	result := make([]uint32, 0, len(fields))
	for _, field := range fields {
		id := uint32(field.(map[string]interface{})["id"].(int))
		result = append(result, id)
	}
	return result
}

// addEnrichmentsFn and deleteEnrichmentsFn are the seams the update flow calls
// into the Enrichments client through. They default to the real client calls and
// are overridable in unit tests so the remove -> add -> rollback flow can be
// driven without a live backend. This resource predates the plugin-framework
// resource struct used by coralogix_data_enrichments, so the seam is at package
// level rather than on a struct.
var (
	addEnrichmentsFn = func(ctx context.Context, meta interface{}, req *cxsdk.AddEnrichmentsRequest) (any, error) {
		return meta.(*clientset.ClientSet).Enrichments().Add(ctx, req)
	}
	deleteEnrichmentsFn = func(ctx context.Context, meta interface{}, req *cxsdk.DeleteEnrichmentsRequest) error {
		return meta.(*clientset.ClientSet).Enrichments().Delete(ctx, req)
	}
)

// extractOldEnrichmentRequest rebuilds the enrichment request from the *old*
// (pre-change) configuration, using the same block dispatch as
// extractEnrichmentRequest. It is used to best-effort roll back the delete when
// the subsequent add fails, so the resource is not left with all of its
// enrichments removed.
func extractOldEnrichmentRequest(d *schema.ResourceData) []*cxsdk.EnrichmentRequestModel {
	if oldGeoIp, _ := d.GetChange("geo_ip"); len(oldGeoIp.([]interface{})) != 0 {
		return expandGeoIp(oldGeoIp.([]interface{})[0])
	}
	if oldSuspiciousIp, _ := d.GetChange("suspicious_ip"); len(oldSuspiciousIp.([]interface{})) != 0 {
		return expandSuspiciousIp(oldSuspiciousIp.([]interface{})[0])
	}
	if oldAws, _ := d.GetChange("aws"); len(oldAws.([]interface{})) != 0 {
		return expandAws(oldAws.([]interface{})[0])
	}
	if oldCustom, _ := d.GetChange("custom"); len(oldCustom.([]interface{})) != 0 {
		enrichment, _ := expandCustom(oldCustom.([]interface{})[0])
		return enrichment
	}
	return nil
}

func resourceCoralogixEnrichmentUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	oldIds := extractOldIdsFromEnrichment(d)
	enrichmentReq, _, err := extractEnrichmentRequest(d)
	if err != nil {
		return diag.FromErr(err)
	}
	log.Print("[INFO] Updating enrichment")
	// oldReq captures the enrichment set that is about to be deleted, derived
	// from the old (pre-change) configuration. If the subsequent Add fails we
	// best-effort re-add this set so the update is not left in a partially
	// applied (all enrichments deleted) state. Mirrors the coralogix_data_enrichments
	// rollback (#765).
	oldReq := extractOldEnrichmentRequest(d)
	// deleteReq targets the enrichments from the *old* (pre-change) set. The IDs
	// must come from oldIds (d.GetChange) rather than the proposed configuration:
	// when a fields set is emptied or shrunk, d.Get would return the smaller set
	// (len 0 when emptied) and the Delete would be skipped, leaving the removed
	// backend enrichments orphaned. Sourcing from the pre-change set deletes them.
	deleteReq := &cxsdk.DeleteEnrichmentsRequest{EnrichmentIds: utils.Uint32SliceToWrappedUint32Slice(oldIds)}
	if len(oldIds) > 0 {
		if err = deleteEnrichmentsFn(ctx, meta, deleteReq); err != nil {
			log.Printf("[ERROR] Received error: %s", err.Error())
			return diag.Errorf("%s", utils.FormatRpcErrors(err, cxsdk.DeleteEnrichmentsRPC, protojson.Format(deleteReq)))
		}
	}
	createReq := &cxsdk.AddEnrichmentsRequest{RequestEnrichments: enrichmentReq}
	enrichmentResp, err := addEnrichmentsFn(ctx, meta, createReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		// The Add failed after the previous enrichments were already deleted.
		// Best-effort rollback: re-add the old set so we do not leave the
		// resource with all of its enrichments deleted. If the rollback itself
		// fails we log it and report it as a separate diagnostic, without
		// masking the original Add error. No Read/refresh runs on the error path.
		addErr := diag.Errorf("%s", utils.FormatRpcErrors(err, cxsdk.AddEnrichmentsRPC, protojson.Format(createReq)))
		// Only roll back if we actually deleted something; oldIds (the pre-change
		// set) is the same source the Delete above was gated on.
		if len(oldIds) > 0 {
			rollbackReq := &cxsdk.AddEnrichmentsRequest{RequestEnrichments: oldReq}
			if _, rollbackErr := addEnrichmentsFn(ctx, meta, rollbackReq); rollbackErr != nil {
				log.Printf("[ERROR] failed to roll back deleted enrichments after a failed enrichment update: %s (original error: %s)", rollbackErr.Error(), err.Error())
				addErr = append(addErr, diag.Diagnostic{
					Severity: diag.Error,
					Summary:  "Failed to roll back deleted enrichments after a failed enrichment update; the resource may have no enrichments configured until the next apply",
					Detail:   utils.FormatRpcErrors(rollbackErr, cxsdk.AddEnrichmentsRPC, protojson.Format(rollbackReq)),
				})
			}
		}
		return addErr
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
			return diag.Errorf("%s", utils.FormatRpcErrors(err, cxsdk.GetEnrichmentsRPC, protojson.Format(&cxsdk.GetEnrichmentsRequest{})))
		}
		enrichmentIds := make([]*wrapperspb.UInt32Value, 0, len(enrichments))
		for _, enrichment := range enrichments {
			enrichmentIds = append(enrichmentIds, wrapperspb.UInt32(enrichment.GetId()))
		}
		deleteReq := &cxsdk.DeleteEnrichmentsRequest{EnrichmentIds: enrichmentIds}
		if err = meta.(*clientset.ClientSet).Enrichments().Delete(ctx, deleteReq); err != nil {
			log.Printf("[ERROR] Received error: %s", err.Error())
			return diag.Errorf("%s", utils.FormatRpcErrors(err, cxsdk.DeleteEnrichmentsRPC, protojson.Format(deleteReq)))
		}
	} else {
		ids := extractIdsFromEnrichment(d)
		deleteReq := &cxsdk.DeleteEnrichmentsRequest{EnrichmentIds: utils.Uint32SliceToWrappedUint32Slice(ids)}
		if err := meta.(*clientset.ClientSet).Enrichments().Delete(ctx, deleteReq); err != nil {
			log.Printf("[ERROR] Received error: %s", err.Error())
			return diag.Errorf("%s", utils.FormatRpcErrors(err, cxsdk.DeleteEnrichmentsRPC, protojson.Format(deleteReq)))
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
		// The API's custom enrichment ID is a uint32, so parse it as one: this
		// rejects negatives and out-of-range values that strconv.Atoi would
		// accept on 64-bit builds and write into state, while the lookup above
		// resolved a different number via StrToUint32.
		parsedID, err := strconv.ParseUint(d.Id(), 10, 32)
		if err != nil {
			return diag.Errorf("invalid custom enrichment id %q: %s", d.Id(), err)
		}
		// custom_enrichment_id is schema.TypeInt, and int is 32-bit on the
		// 386/arm builds we release, so bound the conversion explicitly.
		if parsedID > math.MaxInt32 {
			return diag.Errorf("custom enrichment id %q is out of representable range", d.Id())
		}
		flattenedEnrichment = map[string]interface{}{
			"custom_enrichment_id": int(parsedID),
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
		resourceType := wrapperspb.String(m["resource"].(string))

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

	return result, utils.Uint32ToStr(uintId)
}
