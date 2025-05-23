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
	"strings"

	"terraform-provider-coralogix/coralogix/clientset"
	"terraform-provider-coralogix/coralogix/utils"

	"google.golang.org/protobuf/encoding/protojson"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

const AWS_TYPE = "aws"
const GEOIP_TYPE = "geo_ip"
const SUSIP_TYPE = "suspicious_ip"
const CUSTOM_TYPE = "custom"

type EnrichmentResourceModel struct {
	aws          *AwsEnrichmentFieldsModel    `tfsdk:"aws"`
	geoIp        *EnrichmentFieldsModel       `tfsdk:"geo_ip"`
	suspiciousIp *EnrichmentFieldsModel       `tfsdk:"suspicious_ip"`
	custom       *CustomEnrichmentFieldsModel `tfsdk:"custom"`
}

type CustomEnrichmentFieldsModel struct {
	customEnrichmentId types.Int64            `tfsdk:"custom_enrichment_id"`
	fields             []EnrichmentFieldModel `tfsdk:"fields"`
}

type EnrichmentFieldsModel struct {
	fields []EnrichmentFieldModel `tfsdk:"fields"`
}

type AwsEnrichmentFieldsModel struct {
	fields []AwsEnrichmentFieldModel `tfsdk:"fields"`
}

type EnrichmentFieldModel struct {
	enrichedFieldName types.String `tfsdk:"enriched_field_name"`
	selectedColumns   []string     `tfsdk:"selected_columns"`
	name              types.String `tfsdk:"name"`
	id                types.Int64  `tfsdk:"id"`
}

type AwsEnrichmentFieldModel struct {
	enrichedFieldName types.String `tfsdk:"enriched_field_name"`
	selectedColumns   []string     `tfsdk:"selected_columns"`
	name              types.String `tfsdk:"name"`
	resource          types.String `tfsdk:"resource"`
	id                types.Int64  `tfsdk:"id"`
}

func (e *EnrichmentResourceModel) GetFields() []CoralogixEnrichment {
	fields := make([]CoralogixEnrichment, 0)
	if e.aws != nil {
		for _, f := range e.aws.fields {
			fields = append(fields, &f)
		}
	}
	if e.geoIp != nil {
		for _, f := range e.geoIp.fields {
			fields = append(fields, &f)
		}
	}
	if e.suspiciousIp != nil {
		for _, f := range e.suspiciousIp.fields {
			fields = append(fields, &f)
		}
	}
	if e.custom != nil {
		for _, f := range e.custom.fields {
			fields = append(fields, &f)
		}
	}
	return fields
}

func (e AwsEnrichmentFieldModel) GetEnrichedFieldName() string {
	return e.enrichedFieldName.ValueString()
}

func (e AwsEnrichmentFieldModel) GetName() string {
	return e.name.ValueString()
}

func (e AwsEnrichmentFieldModel) GetId() uint32 {
	return uint32(e.id.ValueInt64())
}

func (e AwsEnrichmentFieldModel) GetSelectedColumns() []string {
	return e.selectedColumns
}

func (e EnrichmentFieldModel) GetEnrichedFieldName() string {
	return e.enrichedFieldName.ValueString()
}

func (e EnrichmentFieldModel) GetName() string {
	return e.name.ValueString()
}

func (e EnrichmentFieldModel) GetSelectedColumns() []string {
	return e.selectedColumns
}

func (e EnrichmentFieldModel) GetId() uint32 {
	return uint32(e.id.ValueInt64())
}

type CoralogixEnrichment interface {
	GetEnrichedFieldName() string
	GetSelectedColumns() []string
	GetName() string
	GetId() uint32
}

func NewEnrichmentResource() resource.Resource {
	return &EnrichmentResource{}
}

type EnrichmentResource struct {
	client *cxsdk.EnrichmentsClient
}

func (r *EnrichmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_enrichment"
}

func (r *EnrichmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clientSet, ok := req.ProviderData.(*clientset.ClientSet)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *clientset.ClientSet, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = clientSet.Enrichments()
}

func (r *EnrichmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"geo_ip": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"fields": schema.SetNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
							Attributes: enrichmentFieldSchema(),
						},
						Optional:            true,
						MarkdownDescription: "Set of fields to enrich with geo_ip information.",
					},
				},
				Optional: true,
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(
						path.MatchRelative().AtParent().AtName("suspicious_ip"),
						path.MatchRelative().AtParent().AtName("aws"),
					),
				},
				MarkdownDescription: "Coralogix allows you to enrich your logs with location data by automatically converting IPs to Geo-points which can be used to aggregate logs by location and create Map visualizations in Kibana.",
			},
			"suspicious_ip": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"fields": schema.ListNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
							Attributes: enrichmentFieldSchema(),
						},
						Validators: []validator.List{
							listvalidator.SizeAtLeast(1),
						},
						Optional:            true,
						MarkdownDescription: "Set of fields to enrich with suspicious_ip information.",
					},
				},
				Optional: true,
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(
						path.MatchRelative().AtParent().AtName("suspicious_ip"),
						path.MatchRelative().AtParent().AtName("aws"),
					),
				},
				MarkdownDescription: "Coralogix allows you to automatically discover threats on your web servers by enriching your logs with the most updated IP blacklists.",
			},
			"aws": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"fields": schema.ListNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"resource": schema.StringAttribute{
									Required: true,
								},
								"name": schema.StringAttribute{
									Required: true,
								},
								"enriched_field_name": schema.StringAttribute{
									Required: true,
								},
								"selected_columns": schema.SetAttribute{
									ElementType: types.StringType,
									Required:    false,
								},
							},
						},
						Validators: []validator.List{
							listvalidator.SizeAtLeast(1),
						},
						Optional:            true,
						MarkdownDescription: "Set of fields to enrich with aws information.",
					},
				},
				Optional: true,
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(
						path.MatchRelative().AtParent().AtName("suspicious_ip"),
						path.MatchRelative().AtParent().AtName("aws"),
					),
				},
				MarkdownDescription: "Coralogix allows you to enrich your logs with the data from a chosen AWS resource. The feature enriches every log that contains a particular resourceId, associated with the metadata of a chosen AWS resource.",
			},
			"custom": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"custom_enrichment_id": schema.Int64Attribute{
						Required: true,
					},
					"fields": schema.ListNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
							Attributes: enrichmentFieldSchema(),
						},
						Optional:            true,
						MarkdownDescription: "Set of fields to enrich with the custom information.",
					},
				},
				Optional: true,
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(
						path.MatchRelative().AtParent().AtName("suspicious_ip"),
						path.MatchRelative().AtParent().AtName("aws"),
					),
				},
				MarkdownDescription: "Custom Log Enrichment with Coralogix enables you to easily enrich your log data.",
			},
		},
		MarkdownDescription: "Coralogix enrichment. For more info please review - https://coralogix.com/docs/coralogix-enrichment-extension/.",
	}
}

func enrichmentFieldSchema() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name": schema.StringAttribute{
			Required: true,
		},
		"enriched_field_name": schema.StringAttribute{
			Required: true,
		},
		"selected_columns": schema.SetAttribute{
			ElementType: types.StringType,
			Required:    false,
		},
		"id": schema.Int64Attribute{
			Optional: true,
			Computed: true,
		},
	}
}

func (r *EnrichmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *EnrichmentResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	enrichments := extractEnrichments(plan)

	createReq := &cxsdk.AddEnrichmentsRequest{RequestEnrichments: enrichments}
	log.Printf("[INFO] Creating new enrichment: %s", protojson.Format(createReq))
	enrichmentResp, err := r.client.Add(ctx, createReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(utils.FormatRpcErrors(err, cxsdk.AddEnrichmentsRPC, protojson.Format(createReq)), err.Error())
	}
	log.Printf("[INFO] Submitted new enrichment: %s", enrichmentResp)
	plan = flattenEnrichments(enrichmentResp.Enrichments)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *EnrichmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *EnrichmentResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	enrichmentType, customId := getEnrichmentTypeAndId(state)

	log.Printf("[INFO] Reading enrichment of type %v (id: %v)", enrichmentType, customId)
	var enrichments []*cxsdk.Enrichment
	var err error
	if customId == 0 {
		enrichments, err = EnrichmentsByType(ctx, r.client, enrichmentType)
	} else {
		enrichments, err = EnrichmentsByID(ctx, r.client, customId)
	}
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(utils.FormatRpcErrors(err, cxsdk.GetEnrichmentsRPC, fmt.Sprintf("%v(%v)", enrichmentType, customId)), "")
	}
	log.Printf("[INFO] Submitted new enrichment: %s", enrichments)
	state = flattenEnrichments(enrichments)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *EnrichmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan *EnrichmentResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ty, fields := extractEnrichmentUpdates(plan)

	updateReq := &cxsdk.AtomicOverwriteEnrichmentsRequest{
		EnrichmentFields: fields,
		EnrichmentType:   ty,
	}
	log.Printf("[INFO] Updating enrichment: %s", protojson.Format(updateReq))
	enrichmentResp, err := r.client.Update(ctx, updateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(utils.FormatRpcErrors(err, cxsdk.UpdateEnrichmentsRPC, protojson.Format(updateReq)), err.Error())
	}
	log.Printf("[INFO] Submitted new enrichment: %s", enrichmentResp)
	plan = flattenEnrichments(enrichmentResp.Enrichments)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *EnrichmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state *EnrichmentResourceModel
	diags := req.State.Get(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ids := make([]*wrapperspb.UInt32Value, 0)
	for _, id := range extractIdsFromEnrichment(state.GetFields()) {
		ids = append(ids, wrapperspb.UInt32(id))
	}
	deleteReq := &cxsdk.DeleteEnrichmentsRequest{EnrichmentIds: ids}
	if err := r.client.Delete(ctx, deleteReq); err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting enrichment of type %v", ids),
			utils.FormatRpcErrors(err, cxsdk.DeleteActionRPC, protojson.Format(deleteReq)),
		)
		return
	}
	log.Printf("[INFO] Deleted enrichments %v", ids)
}

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

func extractIdsFromEnrichment(fields []CoralogixEnrichment) []uint32 {
	ids := make([]uint32, 0)
	for _, e := range fields {
		ids = append(ids, e.GetId())
	}
	return ids
}

func extractEnrichments(plan *EnrichmentResourceModel) []*cxsdk.EnrichmentRequestModel {
	if plan.aws != nil {
		return extractAwsEnrichment(plan.aws)
	}

	if plan.geoIp != nil {
		return extractGeoIpEnrichment(plan.geoIp)
	}

	if plan.suspiciousIp != nil {
		return extractSuspiciousIpEnrichment(plan.suspiciousIp)
	}

	if plan.custom != nil {
		return extractCustomEnrichment(plan.custom)
	}

	return nil
}

func extractEnrichmentUpdates(plan *EnrichmentResourceModel) (*cxsdk.EnrichmentType, []*cxsdk.EnrichmentFieldDefinition) {
	if plan.aws != nil {
		return extractAwsEnrichmentUpdate(plan.aws)
	}

	if plan.geoIp != nil {
		return extractGeoIpEnrichmentUpdate(plan.geoIp)
	}

	if plan.suspiciousIp != nil {
		return extractSuspiciousIpEnrichmentUpdate(plan.suspiciousIp)
	}

	if plan.custom != nil {
		return extractCustomEnrichmentUpdate(plan.custom)
	}

	return nil, nil
}

func extractCustomEnrichment(enrichments *CustomEnrichmentFieldsModel) []*cxsdk.EnrichmentRequestModel {
	fields := make([]*cxsdk.EnrichmentRequestModel, 0)
	for _, f := range enrichments.fields {
		field := extractUntypedEnrichment(f)
		field.EnrichmentType = &cxsdk.EnrichmentType{
			Type: &cxsdk.EnrichmentTypeCustomEnrichment{
				CustomEnrichment: &cxsdk.CustomEnrichmentType{
					Id: utils.TypeInt64ToWrappedUint32(enrichments.customEnrichmentId),
				},
			},
		}
		fields = append(fields, &field)
	}
	return fields
}

func extractSuspiciousIpEnrichment(enrichments *EnrichmentFieldsModel) []*cxsdk.EnrichmentRequestModel {
	fields := make([]*cxsdk.EnrichmentRequestModel, 0)
	for _, f := range enrichments.fields {
		field := extractUntypedEnrichment(f)
		field.EnrichmentType = &cxsdk.EnrichmentType{
			Type: &cxsdk.EnrichmentTypeSuspiciousIP{},
		}
		fields = append(fields, &field)
	}
	return fields
}

func extractGeoIpEnrichment(enrichments *EnrichmentFieldsModel) []*cxsdk.EnrichmentRequestModel {
	fields := make([]*cxsdk.EnrichmentRequestModel, 0)
	for _, f := range enrichments.fields {
		field := extractUntypedEnrichment(f)
		field.EnrichmentType = &cxsdk.EnrichmentType{
			Type: &cxsdk.EnrichmentTypeGeoIP{},
		}
		fields = append(fields, &field)
	}
	return fields
}

func extractAwsEnrichment(enrichments *AwsEnrichmentFieldsModel) []*cxsdk.EnrichmentRequestModel {
	fields := make([]*cxsdk.EnrichmentRequestModel, 0)
	for _, f := range enrichments.fields {
		field := extractUntypedEnrichment(f)
		field.EnrichmentType = &cxsdk.EnrichmentType{
			Type: &cxsdk.EnrichmentTypeAws{
				Aws: &cxsdk.AwsType{
					ResourceType: wrapperspb.String(f.resource.String()),
				},
			},
		}
		fields = append(fields, &field)
	}
	return fields
}

func extractCustomEnrichmentUpdate(enrichments *CustomEnrichmentFieldsModel) (*cxsdk.EnrichmentType, []*cxsdk.EnrichmentFieldDefinition) {
	fields := make([]*cxsdk.EnrichmentFieldDefinition, 0)
	var enrichmentType *cxsdk.EnrichmentType
	for _, f := range enrichments.fields {
		field := extractUntypedEnrichment(f)
		fields = append(fields, &cxsdk.EnrichmentFieldDefinition{
			FieldName:         field.FieldName,
			EnrichedFieldName: field.EnrichedFieldName,
			SelectedColumns:   field.SelectedColumns,
		})
		if enrichmentType == nil {
			enrichmentType = &cxsdk.EnrichmentType{
				Type: &cxsdk.EnrichmentTypeCustomEnrichment{
					CustomEnrichment: &cxsdk.CustomEnrichmentType{
						Id: utils.TypeInt64ToWrappedUint32(enrichments.customEnrichmentId),
					},
				},
			}
		}
	}
	return enrichmentType, fields
}

func extractSuspiciousIpEnrichmentUpdate(enrichments *EnrichmentFieldsModel) (*cxsdk.EnrichmentType, []*cxsdk.EnrichmentFieldDefinition) {
	fields := make([]*cxsdk.EnrichmentFieldDefinition, 0)
	var enrichmentType *cxsdk.EnrichmentType
	for _, f := range enrichments.fields {
		field := extractUntypedEnrichment(f)
		fields = append(fields, &cxsdk.EnrichmentFieldDefinition{
			FieldName:         field.FieldName,
			EnrichedFieldName: field.EnrichedFieldName,
			SelectedColumns:   field.SelectedColumns,
		})

		if enrichmentType == nil {
			enrichmentType = &cxsdk.EnrichmentType{
				Type: &cxsdk.EnrichmentTypeSuspiciousIP{},
			}
		}
	}
	return enrichmentType, fields
}

func extractGeoIpEnrichmentUpdate(enrichments *EnrichmentFieldsModel) (*cxsdk.EnrichmentType, []*cxsdk.EnrichmentFieldDefinition) {
	fields := make([]*cxsdk.EnrichmentFieldDefinition, 0)
	var enrichmentType *cxsdk.EnrichmentType

	for _, f := range enrichments.fields {
		field := extractUntypedEnrichment(f)
		fields = append(fields, &cxsdk.EnrichmentFieldDefinition{
			FieldName:         field.FieldName,
			EnrichedFieldName: field.EnrichedFieldName,
			SelectedColumns:   field.SelectedColumns,
		})

		field.EnrichmentType = &cxsdk.EnrichmentType{
			Type: &cxsdk.EnrichmentTypeGeoIP{},
		}
	}
	return enrichmentType, fields
}

func extractAwsEnrichmentUpdate(enrichments *AwsEnrichmentFieldsModel) (*cxsdk.EnrichmentType, []*cxsdk.EnrichmentFieldDefinition) {
	fields := make([]*cxsdk.EnrichmentFieldDefinition, 0)
	var enrichmentType *cxsdk.EnrichmentType
	for _, f := range enrichments.fields {
		field := extractUntypedEnrichment(f)
		fields = append(fields, &cxsdk.EnrichmentFieldDefinition{
			FieldName:         field.FieldName,
			EnrichedFieldName: field.EnrichedFieldName,
			SelectedColumns:   field.SelectedColumns,
		})

		field.EnrichmentType = &cxsdk.EnrichmentType{
			Type: &cxsdk.EnrichmentTypeAws{
				Aws: &cxsdk.AwsType{
					ResourceType: wrapperspb.String(f.resource.String()),
				},
			},
		}
	}
	return enrichmentType, fields
}

func extractUntypedEnrichment(e CoralogixEnrichment) cxsdk.EnrichmentRequestModel {
	return cxsdk.EnrichmentRequestModel{
		FieldName:         wrapperspb.String(e.GetName()),
		EnrichedFieldName: wrapperspb.String(e.GetEnrichedFieldName()),
		SelectedColumns:   e.GetSelectedColumns(),
	}
}

func flattenEnrichments(enrichments []*cxsdk.Enrichment) *EnrichmentResourceModel {
	var model EnrichmentResourceModel
	switch t := firstEnrichmentType(enrichments); t {
	case AWS_TYPE:
		model.aws = flattenAwsEnrichment(enrichments)
		break
	case GEOIP_TYPE:
		model.geoIp = &EnrichmentFieldsModel{
			fields: flattenEnrichmentFields(enrichments),
		}
		break
	case SUSIP_TYPE:
		model.suspiciousIp = &EnrichmentFieldsModel{
			fields: flattenEnrichmentFields(enrichments),
		}
		break
	case CUSTOM_TYPE:
		model.custom = flattenCustomEnrichments(enrichments)
		break
	default:
		log.Printf("[ERROR] Unknown enrichment type: %v", t)
	}
	return &model
}

func firstEnrichmentType(enrichments []*cxsdk.Enrichment) string {
	for _, e := range enrichments {
		switch e.EnrichmentType.GetType().(type) {
		case *cxsdk.EnrichmentTypeAws:
			return AWS_TYPE
		case *cxsdk.EnrichmentTypeGeoIP:
			return GEOIP_TYPE
		case *cxsdk.EnrichmentTypeSuspiciousIP:
			return SUSIP_TYPE
		case *cxsdk.EnrichmentTypeCustomEnrichment:
			return CUSTOM_TYPE
		default:
			break
		}
	}
	return ""
}

func extractCustomEnrichmentId(enrichments []*cxsdk.Enrichment) (uint32, bool) {
	var id uint32
	id = 0
	found := false
	for _, e := range enrichments {
		found = e.EnrichmentType.GetCustomEnrichment() == nil
		if !found {
			break
		}
		id = e.EnrichmentType.GetCustomEnrichment().Id.Value
	}
	return id, found
}

func getEnrichmentTypeAndId(model *EnrichmentResourceModel) (string, uint32) {
	if model.aws != nil {
		return AWS_TYPE, 0
	}
	if model.geoIp != nil {
		return GEOIP_TYPE, 0
	}
	if model.suspiciousIp != nil {
		return SUSIP_TYPE, 0
	}
	return CUSTOM_TYPE, uint32(model.custom.customEnrichmentId.ValueInt64())
}

func flattenAwsEnrichment(enrichments []*cxsdk.Enrichment) *AwsEnrichmentFieldsModel {
	fields := make([]AwsEnrichmentFieldModel, 0)
	for _, e := range enrichments {
		fields = append(fields, AwsEnrichmentFieldModel{
			id:                types.Int64Value(int64(e.GetId())),
			name:              types.StringValue(e.GetFieldName()),
			resource:          types.StringValue(e.GetEnrichmentType().GetAws().GetResourceType().GetValue()),
			selectedColumns:   e.GetSelectedColumns(),
			enrichedFieldName: utils.WrapperspbStringToTypeString(e.GetEnrichedFieldName()),
		})
	}
	return &AwsEnrichmentFieldsModel{
		fields: fields,
	}
}

func flattenCustomEnrichments(enrichments []*cxsdk.Enrichment) *CustomEnrichmentFieldsModel {
	fields := make([]EnrichmentFieldModel, 0)
	var customId int64
	for _, e := range enrichments {
		fields = append(fields, EnrichmentFieldModel{
			id:                types.Int64Value(int64(e.GetId())),
			name:              types.StringValue(e.GetFieldName()),
			selectedColumns:   e.GetSelectedColumns(),
			enrichedFieldName: utils.WrapperspbStringToTypeString(e.GetEnrichedFieldName()),
		})
		customId = int64(e.GetEnrichmentType().GetCustomEnrichment().GetId().GetValue())
	}
	return &CustomEnrichmentFieldsModel{
		fields:             fields,
		customEnrichmentId: types.Int64Value(customId),
	}
}

func flattenEnrichmentFields(enrichments []*cxsdk.Enrichment) []EnrichmentFieldModel {
	fields := make([]EnrichmentFieldModel, 0)
	for _, e := range enrichments {
		fields = append(fields, EnrichmentFieldModel{
			id:                types.Int64Value(int64(e.GetId())),
			name:              types.StringValue(e.GetFieldName()),
			selectedColumns:   e.SelectedColumns,
			enrichedFieldName: utils.WrapperspbStringToTypeString(e.GetEnrichedFieldName()),
		})
	}
	return fields
}
