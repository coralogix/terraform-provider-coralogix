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
	Aws          *AwsEnrichmentFieldsModel    `tfsdk:"aws"`
	GeoIp        *EnrichmentFieldsModel       `tfsdk:"geo_ip"`
	SuspiciousIp *EnrichmentFieldsModel       `tfsdk:"suspicious_ip"`
	Custom       *CustomEnrichmentFieldsModel `tfsdk:"custom"`
}

type CustomEnrichmentFieldsModel struct {
	CustomEnrichmentId types.Int64            `tfsdk:"custom_enrichment_id"`
	Fields             []EnrichmentFieldModel `tfsdk:"fields"`
}

type EnrichmentFieldsModel struct {
	Fields []EnrichmentFieldModel `tfsdk:"fields"`
}

type AwsEnrichmentFieldsModel struct {
	Fields []AwsEnrichmentFieldModel `tfsdk:"fields"`
}

type EnrichmentFieldModel struct {
	EnrichedFieldName types.String `tfsdk:"enriched_field_name"`
	SelectedColumns   []string     `tfsdk:"selected_columns"`
	Name              types.String `tfsdk:"name"`
	Id                types.Int64  `tfsdk:"id"`
}

type AwsEnrichmentFieldModel struct {
	EnrichedFieldName types.String `tfsdk:"enriched_field_name"`
	SelectedColumns   []string     `tfsdk:"selected_columns"`
	Name              types.String `tfsdk:"name"`
	Resource          types.String `tfsdk:"resource"`
	Id                types.Int64  `tfsdk:"id"`
}

func (e *EnrichmentResourceModel) GetFields() []CoralogixEnrichment {
	fields := make([]CoralogixEnrichment, 0)
	if e.Aws != nil {
		for _, f := range e.Aws.Fields {
			fields = append(fields, &f)
		}
	}
	if e.GeoIp != nil {
		for _, f := range e.GeoIp.Fields {
			fields = append(fields, &f)
		}
	}
	if e.SuspiciousIp != nil {
		for _, f := range e.SuspiciousIp.Fields {
			fields = append(fields, &f)
		}
	}
	if e.Custom != nil {
		for _, f := range e.Custom.Fields {
			fields = append(fields, &f)
		}
	}
	return fields
}

func (e AwsEnrichmentFieldModel) GetEnrichedFieldName() string {
	return e.EnrichedFieldName.ValueString()
}

func (e AwsEnrichmentFieldModel) GetName() string {
	return e.Name.ValueString()
}

func (e AwsEnrichmentFieldModel) GetId() uint32 {
	return uint32(e.Id.ValueInt64())
}

func (e AwsEnrichmentFieldModel) GetSelectedColumns() []string {
	return e.SelectedColumns
}

func (e EnrichmentFieldModel) GetEnrichedFieldName() string {
	return e.EnrichedFieldName.ValueString()
}

func (e EnrichmentFieldModel) GetName() string {
	return e.Name.ValueString()
}

func (e EnrichmentFieldModel) GetSelectedColumns() []string {
	return e.SelectedColumns
}

func (e EnrichmentFieldModel) GetId() uint32 {
	return uint32(e.Id.ValueInt64())
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

func (r *EnrichmentResource) UpgradeState(_ context.Context) map[int64]resource.StateUpgrader {
	schemaV0 := r.schemaV0()
	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema:   &schemaV0,
			StateUpgrader: upgradeFromOldEnrichmentProvider,
		},
	}
}

// Upgrades form the old (plugin-framework) provider to the new
// Since the enriched_field_name is required, the value is set to <field_name>_enriched
func upgradeFromOldEnrichmentProvider(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
	log.Print("[INFO] Enrichment Provider V0 -> V1 Update")
	type EnrichmentFieldModelV0 struct {
		Name types.String `tfsdk:"name"`
		Id   types.Int64  `tfsdk:"id"`
	}

	type AwsEnrichmentFieldModelV0 struct {
		Name     types.String `tfsdk:"name"`
		Resource types.String `tfsdk:"resource"`
		Id       types.Int64  `tfsdk:"id"`
	}

	type AwsEnrichmentFieldsModelV0 struct {
		Fields []AwsEnrichmentFieldModelV0 `tfsdk:"fields"`
	}

	type CustomEnrichmentFieldsModelV0 struct {
		CustomEnrichmentId types.Int64              `tfsdk:"custom_enrichment_id"`
		Fields             []EnrichmentFieldModelV0 `tfsdk:"fields"`
	}

	type EnrichmentFieldsModelV0 struct {
		Fields []EnrichmentFieldModelV0 `tfsdk:"fields"`
	}

	type EnrichmentResourceModelV0 struct {
		Aws          []*AwsEnrichmentFieldsModelV0    `tfsdk:"aws"`
		GeoIp        []*EnrichmentFieldsModelV0       `tfsdk:"geo_ip"`
		SuspiciousIp []*EnrichmentFieldsModelV0       `tfsdk:"suspicious_ip"`
		Custom       []*CustomEnrichmentFieldsModelV0 `tfsdk:"custom"`
	}

	var priorStateData EnrichmentResourceModelV0

	resp.Diagnostics.Append(req.State.Get(ctx, &priorStateData)...)
	if resp.Diagnostics.HasError() {
		log.Print("[ERROR] Couldn't run state upgrade")
		return
	}

	awsFields := make([]AwsEnrichmentFieldModel, 0)

	for a := range priorStateData.Aws {
		for _, f := range priorStateData.Aws[a].Fields {
			awsFields = append(awsFields, AwsEnrichmentFieldModel{
				EnrichedFieldName: types.StringValue(fmt.Sprintf("%v_enriched", f.Name)),
				SelectedColumns:   []string{},
				Resource:          f.Resource,
				Name:              f.Name,
				Id:                f.Id,
			})
		}
	}

	geoIpFields := make([]EnrichmentFieldModel, 0)
	for a := range priorStateData.GeoIp {
		for _, f := range priorStateData.GeoIp[a].Fields {
			geoIpFields = append(geoIpFields, EnrichmentFieldModel{
				EnrichedFieldName: types.StringValue(fmt.Sprintf("%v_enriched", f.Name)),
				SelectedColumns:   []string{},
				Name:              f.Name,
				Id:                f.Id,
			})
		}
	}

	susIpFields := make([]EnrichmentFieldModel, 0)
	for a := range priorStateData.SuspiciousIp {
		for _, f := range priorStateData.SuspiciousIp[a].Fields {
			susIpFields = append(susIpFields, EnrichmentFieldModel{
				EnrichedFieldName: types.StringValue(fmt.Sprintf("%v_enriched", f.Name)),
				SelectedColumns:   []string{},
				Name:              f.Name,
				Id:                f.Id,
			})
		}
	}
	customFields := make([]EnrichmentFieldModel, 0)
	customEnrichmentId := types.Int64Null()
	for a := range priorStateData.Custom {
		if customEnrichmentId.IsNull() {
			customEnrichmentId = priorStateData.Custom[a].CustomEnrichmentId
		}
		for _, f := range priorStateData.Custom[a].Fields {
			customFields = append(customFields, EnrichmentFieldModel{
				EnrichedFieldName: types.StringValue(fmt.Sprintf("%v_enriched", f.Name)),
				SelectedColumns:   []string{},
				Name:              f.Name,
				Id:                f.Id,
			})
		}
	}
	upgradedStateData := EnrichmentResourceModel{
		Aws: &AwsEnrichmentFieldsModel{
			Fields: awsFields,
		},
		GeoIp: &EnrichmentFieldsModel{
			Fields: geoIpFields,
		},
		SuspiciousIp: &EnrichmentFieldsModel{
			Fields: susIpFields,
		},
		Custom: &CustomEnrichmentFieldsModel{
			CustomEnrichmentId: customEnrichmentId,
			Fields:             customFields,
		},
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, upgradedStateData)...)
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

func (r *EnrichmentResource) schemaV1() schema.Schema {
	return schema.Schema{
		Version: 1,
		Blocks: map[string]schema.Block{
			GEOIP_TYPE: schema.SingleNestedBlock{
				Blocks: map[string]schema.Block{
					"fields": schema.ListNestedBlock{
						NestedObject: schema.NestedBlockObject{
							Attributes: enrichmentFieldSchema(),
						},
						MarkdownDescription: "Set of fields to enrich with geo_ip information.",
					},
				},
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(
						path.MatchRelative().AtParent().AtName("suspicious_ip"),
						path.MatchRelative().AtParent().AtName("aws"),
						path.MatchRelative().AtParent().AtName("custom"),
					),
				},
				MarkdownDescription: "Coralogix allows you to enrich your logs with location data by automatically converting IPs to Geo-points which can be used to aggregate logs by location and create Map visualizations in Kibana.",
			},
			SUSIP_TYPE: schema.SingleNestedBlock{
				Blocks: map[string]schema.Block{
					"fields": schema.ListNestedBlock{
						NestedObject: schema.NestedBlockObject{
							Attributes: enrichmentFieldSchema(),
						},
						Validators: []validator.List{
							listvalidator.SizeAtLeast(1),
						},
						MarkdownDescription: "Set of fields to enrich with suspicious_ip information.",
					},
				},
				MarkdownDescription: "Coralogix allows you to automatically discover threats on your web servers by enriching your logs with the most updated IP blacklists.",
			},
			AWS_TYPE: schema.SingleNestedBlock{
				Blocks: map[string]schema.Block{
					"fields": schema.ListNestedBlock{
						NestedObject: schema.NestedBlockObject{
							Attributes: map[string]schema.Attribute{
								"resource": schema.StringAttribute{
									Required: true,
								},
								"name": schema.StringAttribute{
									Required: true,
								},
								"id": schema.Int64Attribute{
									Required: true,
								},
								"enriched_field_name": schema.StringAttribute{
									Required: true,
								},
								"selected_columns": schema.SetAttribute{
									ElementType: types.StringType,
									Optional:    true,
								},
							},
						},
						Validators: []validator.List{
							listvalidator.SizeAtLeast(1),
						},
						MarkdownDescription: "Set of fields to enrich with aws information.",
					},
				},
				MarkdownDescription: "Coralogix allows you to enrich your logs with the data from a chosen AWS resource. The feature enriches every log that contains a particular resourceId, associated with the metadata of a chosen AWS resource.",
			},
			CUSTOM_TYPE: schema.SingleNestedBlock{
				Attributes: map[string]schema.Attribute{
					"custom_enrichment_id": schema.Int64Attribute{
						Optional: true,
					},
				},
				Blocks: map[string]schema.Block{
					"fields": schema.ListNestedBlock{
						NestedObject: schema.NestedBlockObject{
							Attributes: enrichmentFieldSchema(),
						},
						MarkdownDescription: "Set of fields to enrich with the custom information.",
					},
				},
				MarkdownDescription: "Custom Log Enrichment with Coralogix enables you to easily enrich your log data.",
			},
		},
		MarkdownDescription: "Coralogix enrichment. For more info please review - https://coralogix.com/docs/coralogix-enrichment-extension/.",
	}
}

func (r *EnrichmentResource) schemaV0() schema.Schema {
	return schema.Schema{
		Version: 0,
		Blocks: map[string]schema.Block{
			"geo_ip": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"fields": schema.ListNestedAttribute{
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										Required: true,
									},
									"id": schema.Int64Attribute{
										Required: true,
									},
								},
							},
						},
					},
				},
			},
			"suspicious_ip": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"fields": schema.ListNestedAttribute{
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										Required: true,
									},
									"id": schema.Int64Attribute{
										Required: true,
									},
								},
							},
						},
					},
				},
			},
			"aws": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
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
									"id": schema.Int64Attribute{
										Required: true,
									},
								},
								// Optional:            true,
							},
						},
					},
				},
			},
			"custom": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"custom_enrichment_id": schema.Int64Attribute{
							Optional: true,
						},
					},
					Blocks: map[string]schema.Block{
						"fields": schema.ListNestedBlock{
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										Required: true,
									},
									"id": schema.StringAttribute{
										Required: true,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *EnrichmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = r.schemaV1()
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
			Optional:    true,
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
		return
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
		return
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

	enrichments := extractEnrichments(plan)

	updateReq := &cxsdk.AtomicOverwriteEnrichmentsRequest{
		RequestEnrichments: enrichments,
	}
	log.Printf("[INFO] Updating enrichment: %s", protojson.Format(updateReq))
	enrichmentResp, err := r.client.Update(ctx, updateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(utils.FormatRpcErrors(err, cxsdk.UpdateEnrichmentsRPC, protojson.Format(updateReq)), err.Error())
		return
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
	if plan.Aws != nil {
		return extractAwsEnrichment(plan.Aws)
	}

	if plan.GeoIp != nil {
		return extractGeoIpEnrichment(plan.GeoIp)
	}

	if plan.SuspiciousIp != nil {
		return extractSuspiciousIpEnrichment(plan.SuspiciousIp)
	}

	if plan.Custom != nil {
		return extractCustomEnrichment(plan.Custom)
	}

	return nil
}

func extractCustomEnrichment(enrichments *CustomEnrichmentFieldsModel) []*cxsdk.EnrichmentRequestModel {
	fields := make([]*cxsdk.EnrichmentRequestModel, 0)
	for _, f := range enrichments.Fields {
		field := extractUntypedEnrichment(f)
		field.EnrichmentType = &cxsdk.EnrichmentType{
			Type: &cxsdk.EnrichmentTypeCustomEnrichment{
				CustomEnrichment: &cxsdk.CustomEnrichmentType{
					Id: utils.TypeInt64ToWrappedUint32(enrichments.CustomEnrichmentId),
				},
			},
		}
		fields = append(fields, &field)
	}
	return fields
}

func extractSuspiciousIpEnrichment(enrichments *EnrichmentFieldsModel) []*cxsdk.EnrichmentRequestModel {
	fields := make([]*cxsdk.EnrichmentRequestModel, 0)
	for _, f := range enrichments.Fields {
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
	for _, f := range enrichments.Fields {
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
	for _, f := range enrichments.Fields {
		field := extractUntypedEnrichment(f)
		field.EnrichmentType = &cxsdk.EnrichmentType{
			Type: &cxsdk.EnrichmentTypeAws{
				Aws: &cxsdk.AwsType{
					ResourceType: wrapperspb.String(f.Resource.String()),
				},
			},
		}
		fields = append(fields, &field)
	}
	return fields
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
		model.Aws = flattenAwsEnrichment(enrichments)
	case GEOIP_TYPE:
		model.GeoIp = &EnrichmentFieldsModel{
			Fields: flattenEnrichmentFields(enrichments),
		}
	case SUSIP_TYPE:
		model.SuspiciousIp = &EnrichmentFieldsModel{
			Fields: flattenEnrichmentFields(enrichments),
		}
	case CUSTOM_TYPE:
		model.Custom = flattenCustomEnrichments(enrichments)
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

func getEnrichmentTypeAndId(model *EnrichmentResourceModel) (string, uint32) {
	if model.Aws != nil {
		return AWS_TYPE, 0
	}
	if model.GeoIp != nil {
		return GEOIP_TYPE, 0
	}
	if model.SuspiciousIp != nil {
		return SUSIP_TYPE, 0
	}
	if model.Custom != nil {
		return CUSTOM_TYPE, uint32(model.Custom.CustomEnrichmentId.ValueInt64())
	}
	return "", 0
}

func flattenAwsEnrichment(enrichments []*cxsdk.Enrichment) *AwsEnrichmentFieldsModel {
	fields := make([]AwsEnrichmentFieldModel, 0)
	for _, e := range enrichments {
		fields = append(fields, AwsEnrichmentFieldModel{
			Id:                types.Int64Value(int64(e.GetId())),
			Name:              types.StringValue(e.GetFieldName()),
			Resource:          types.StringValue(e.GetEnrichmentType().GetAws().GetResourceType().GetValue()),
			SelectedColumns:   e.GetSelectedColumns(),
			EnrichedFieldName: utils.WrapperspbStringToTypeString(e.GetEnrichedFieldName()),
		})
	}
	return &AwsEnrichmentFieldsModel{
		Fields: fields,
	}
}

func flattenCustomEnrichments(enrichments []*cxsdk.Enrichment) *CustomEnrichmentFieldsModel {
	fields := make([]EnrichmentFieldModel, 0)
	var customId int64
	for _, e := range enrichments {
		fields = append(fields, EnrichmentFieldModel{
			Id:                types.Int64Value(int64(e.GetId())),
			Name:              types.StringValue(e.GetFieldName()),
			SelectedColumns:   e.GetSelectedColumns(),
			EnrichedFieldName: utils.WrapperspbStringToTypeString(e.GetEnrichedFieldName()),
		})
		customId = int64(e.GetEnrichmentType().GetCustomEnrichment().GetId().GetValue())
	}
	return &CustomEnrichmentFieldsModel{
		Fields:             fields,
		CustomEnrichmentId: types.Int64Value(customId),
	}
}

func flattenEnrichmentFields(enrichments []*cxsdk.Enrichment) []EnrichmentFieldModel {
	fields := make([]EnrichmentFieldModel, 0)
	for _, e := range enrichments {
		fields = append(fields, EnrichmentFieldModel{
			Id:                types.Int64Value(int64(e.GetId())),
			Name:              types.StringValue(e.GetFieldName()),
			SelectedColumns:   e.SelectedColumns,
			EnrichedFieldName: utils.WrapperspbStringToTypeString(e.GetEnrichedFieldName()),
		})
	}
	return fields
}
