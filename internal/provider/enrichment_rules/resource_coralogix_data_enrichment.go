package enrichment_rules

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	ess "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/enrichments_service"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure   = &DataEnrichmentsResource{}
	_ resource.ResourceWithImportState = &DataEnrichmentsResource{}
)

const (
	AWS_TYPE    = "aws"
	GEOIP_TYPE  = "geo_ip"
	SUSIP_TYPE  = "suspicious_ip"
	CUSTOM_TYPE = "custom"
)

type CoralogixEnrichment interface {
	GetEnrichedFieldName() string
	GetSelectedColumns() []string
	GetName() string
	GetId() uint32
}

type DataEnrichmentsModel struct {
	Aws          *AwsEnrichmentFieldsModel    `tfsdk:"aws"`
	GeoIp        *EnrichmentFieldsModel       `tfsdk:"geo_ip"`
	SuspiciousIp *EnrichmentFieldsModel       `tfsdk:"suspicious_ip"`
	Custom       *CustomEnrichmentFieldsModel `tfsdk:"custom"`
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
	ID                types.Int64  `tfsdk:"id"`
}

type AwsEnrichmentFieldModel struct {
	EnrichedFieldName types.String `tfsdk:"enriched_field_name"`
	SelectedColumns   []string     `tfsdk:"selected_columns"`
	Name              types.String `tfsdk:"name"`
	Resource          types.String `tfsdk:"resource"`
	ID                types.Int64  `tfsdk:"id"`
}

type CustomEnrichmentFieldsModel struct {
	CustomEnrichmentId types.Int64            `tfsdk:"custom_enrichment_id"`
	Fields             []EnrichmentFieldModel `tfsdk:"fields"`
}

func (e AwsEnrichmentFieldModel) GetId() uint32 {
	return uint32(e.ID.ValueInt64())
}

func (e AwsEnrichmentFieldModel) GetSelectedColumns() []string {
	return e.SelectedColumns
}

func (e AwsEnrichmentFieldModel) GetEnrichedFieldName() string {
	return e.EnrichedFieldName.ValueString()
}

func (e AwsEnrichmentFieldModel) GetName() string {
	return e.Name.ValueString()
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
	return uint32(e.ID.ValueInt64())
}

func NewDataEnrichmentsResource() resource.Resource {
	return &DataEnrichmentsResource{}
}

type DataEnrichmentsResource struct {
	client *ess.EnrichmentsServiceAPIService
}

func (e *DataEnrichmentsModel) GetFields() []CoralogixEnrichment {
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

func (r *DataEnrichmentsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *DataEnrichmentsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.DataEnrichments()
}

func (r *DataEnrichmentsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_parsing_rules"
}

func (r *DataEnrichmentsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			GEOIP_TYPE: schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"fields": schema.ListNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
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
			SUSIP_TYPE: schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"fields": schema.ListNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
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
			AWS_TYPE: schema.SingleNestedAttribute{
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
			CUSTOM_TYPE: schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"custom_enrichment_id": schema.Int64Attribute{
						Optional: true,
					},
					"fields": schema.ListNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
							Attributes: enrichmentFieldSchema(),
						},
						MarkdownDescription: "Set of fields to enrich with the custom information.",
					},
				},
				MarkdownDescription: "Custom Log Enrichment with Coralogix enables you to easily enrich your log data.",
			},
		},
		MarkdownDescription: "Coralogix enrichment. For more info please check - https://coralogix.com/docs/coralogix-enrichment-extension/.",
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
			Optional:    true,
		},
		"id": schema.Int64Attribute{
			Optional: true,
			Computed: true,
		},
	}
}

func (r *DataEnrichmentsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *DataEnrichmentsModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rq := extractDataEnrichmentsCreate(plan)

	result, httpResponse, err := r.client.
		EnrichmentServiceAddEnrichments(ctx).
		EnrichmentsCreationRequest(*rq).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error creating coralogix_parsing_rules",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", rq),
		)
		return
	}
	state := flattenDataEnrichments(result.Enrichments)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *DataEnrichmentsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan *DataEnrichmentsModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rq := extractDataEnrichmentsUpdate(plan)

	result, httpResponse, err := r.client.
		EnrichmentServiceAtomicOverwriteEnrichments(ctx).
		EnrichmentServiceAtomicOverwriteEnrichmentsRequest(*rq).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error replacing coralogix_parsing_rules",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", rq),
		)
		return
	}
	state := flattenDataEnrichments(result.Enrichments)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *DataEnrichmentsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *DataEnrichmentsModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, httpResponse, err := r.client.
		EnrichmentServiceGetEnrichments(ctx).
		Execute()
	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				"coralogix_parsing_rules is in state, but no longer exists in Coralogix backend",
				"coralogix_parsing_rules will be recreated when you apply",
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error reading coralogix_parsing_rules",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
			)
		}
		return
	}

	state = flattenDataEnrichments(result.Enrichments)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DataEnrichmentsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state *DataEnrichmentsModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ids := make([]int64, 0)
	for _, id := range ExtractIdsFromEnrichment(state.GetFields()) {
		ids = append(ids, int64(id))
	}

	_, httpResponse, err := r.client.EnrichmentServiceRemoveEnrichments(ctx).EnrichmentIds(ids).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error deleting coralogix_parsing_rules",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
		)
	}
}

func extractDataEnrichments(plan *DataEnrichmentsModel) []ess.EnrichmentRequestModel {
	requestModels := make([]ess.EnrichmentRequestModel, 0)
	if plan.Aws != nil {
		for _, f := range plan.Aws.Fields {
			enrichmentType := ess.EnrichmentType{
				EnrichmentTypeAws: &ess.EnrichmentTypeAws{
					Aws: &ess.AwsType{
						ResourceType: f.Resource.ValueStringPointer(),
					},
				},
			}
			requestModels = append(requestModels, ess.EnrichmentRequestModel{
				EnrichedFieldName: f.EnrichedFieldName.ValueStringPointer(),
				FieldName:         f.Name.ValueString(),
				SelectedColumns:   f.SelectedColumns,
				EnrichmentType:    enrichmentType,
			})
		}
	}
	if plan.GeoIp != nil {
		for _, f := range plan.GeoIp.Fields {
			enrichmentType := ess.EnrichmentType{
				EnrichmentTypeGeoIp: &ess.EnrichmentTypeGeoIp{},
			}
			requestModels = append(requestModels, ess.EnrichmentRequestModel{
				EnrichedFieldName: f.EnrichedFieldName.ValueStringPointer(),
				FieldName:         f.Name.ValueString(),
				SelectedColumns:   f.SelectedColumns,
				EnrichmentType:    enrichmentType,
			})
		}
	}
	if plan.SuspiciousIp != nil {
		for _, f := range plan.SuspiciousIp.Fields {
			enrichmentType := ess.EnrichmentType{
				EnrichmentTypeSuspiciousIp: &ess.EnrichmentTypeSuspiciousIp{},
			}
			requestModels = append(requestModels, ess.EnrichmentRequestModel{
				EnrichedFieldName: f.EnrichedFieldName.ValueStringPointer(),
				FieldName:         f.Name.ValueString(),
				SelectedColumns:   f.SelectedColumns,
				EnrichmentType:    enrichmentType,
			})
		}
	}
	if plan.Custom != nil {
		for _, f := range plan.Custom.Fields {
			id := int64(f.GetId())
			enrichmentType := ess.EnrichmentType{
				EnrichmentTypeCustomEnrichment: &ess.EnrichmentTypeCustomEnrichment{
					CustomEnrichment: &ess.CustomEnrichmentType{
						Id: &id,
					},
				},
			}
			requestModels = append(requestModels, ess.EnrichmentRequestModel{
				EnrichedFieldName: f.EnrichedFieldName.ValueStringPointer(),
				FieldName:         f.Name.ValueString(),
				SelectedColumns:   f.SelectedColumns,
				EnrichmentType:    enrichmentType,
			})
		}
	}
	return requestModels
}

func extractDataEnrichmentsCreate(plan *DataEnrichmentsModel) *ess.EnrichmentsCreationRequest {
	req := &ess.EnrichmentsCreationRequest{
		RequestEnrichments: extractDataEnrichments(plan),
	}
	return req
}
func extractDataEnrichmentsUpdate(plan *DataEnrichmentsModel) *ess.EnrichmentServiceAtomicOverwriteEnrichmentsRequest {
	req := &ess.EnrichmentServiceAtomicOverwriteEnrichmentsRequest{
		RequestEnrichments: extractDataEnrichments(plan),
	}
	return req
}

func flattenDataEnrichments(rgrp []ess.Enrichment) *DataEnrichmentsModel {
	model := &DataEnrichmentsModel{}
	for _, e := range rgrp {
		if e.EnrichmentType.EnrichmentTypeAws != nil {
			if model.Aws == nil {
				model.Aws = &AwsEnrichmentFieldsModel{}
			}
			model.Aws.Fields = append(model.Aws.Fields, AwsEnrichmentFieldModel{
				EnrichedFieldName: types.StringPointerValue(e.EnrichedFieldName),
				SelectedColumns:   e.SelectedColumns,
				Name:              types.StringValue(e.FieldName),
				Resource:          types.StringPointerValue(e.EnrichmentType.EnrichmentTypeAws.Aws.ResourceType),
				ID:                types.Int64Value(e.Id),
			})
		} else if e.EnrichmentType.EnrichmentTypeGeoIp != nil {
			if model.GeoIp == nil {
				model.GeoIp = &EnrichmentFieldsModel{}
			}
			model.GeoIp.Fields = append(model.GeoIp.Fields, EnrichmentFieldModel{
				EnrichedFieldName: types.StringPointerValue(e.EnrichedFieldName),
				SelectedColumns:   e.SelectedColumns,
				Name:              types.StringValue(e.FieldName),
				ID:                types.Int64Value(e.Id),
			})
		} else if e.EnrichmentType.EnrichmentTypeSuspiciousIp != nil {
			if model.SuspiciousIp == nil {
				model.SuspiciousIp = &EnrichmentFieldsModel{}
			}
			model.SuspiciousIp.Fields = append(model.SuspiciousIp.Fields, EnrichmentFieldModel{
				EnrichedFieldName: types.StringPointerValue(e.EnrichedFieldName),
				SelectedColumns:   e.SelectedColumns,
				Name:              types.StringValue(e.FieldName),
				ID:                types.Int64Value(e.Id),
			})
		} else if e.EnrichmentType.EnrichmentTypeCustomEnrichment != nil {
			if model.Custom == nil {
				model.Custom = &CustomEnrichmentFieldsModel{}
			}
			model.Custom.CustomEnrichmentId = types.Int64Value(int64(*e.EnrichmentType.EnrichmentTypeCustomEnrichment.CustomEnrichment.Id))
			model.Custom.Fields = append(model.Custom.Fields, EnrichmentFieldModel{
				EnrichedFieldName: types.StringPointerValue(e.EnrichedFieldName),
				SelectedColumns:   e.SelectedColumns,
				Name:              types.StringValue(e.FieldName),
				ID:                types.Int64Value(e.Id),
			})
		}
	}
	return model
}

func DataEnrichmentsByID(ctx context.Context, client *ess.EnrichmentsServiceAPIService, customEnrichmentID uint32) ([]ess.Enrichment, error) {
	result, _, err := client.EnrichmentServiceGetEnrichments(ctx).Execute()
	if err != nil {
		return nil, err
	}

	enrichments := make([]ess.Enrichment, 0)
	for _, enrichment := range result.Enrichments {
		if customEnrichment := enrichment.GetEnrichmentType().EnrichmentTypeCustomEnrichment; customEnrichment != nil && customEnrichment.CustomEnrichment != nil && uint32(*customEnrichment.CustomEnrichment.Id) == customEnrichmentID {
			enrichments = append(enrichments, enrichment)
		}
	}
	log.Printf("[INFO] found %v enrichments for ID %v", len(enrichments), customEnrichmentID)
	return enrichments, nil
}

func DataEnrichmentsByType(ctx context.Context, client *ess.EnrichmentsServiceAPIService, enrichmentType string) ([]ess.Enrichment, error) {
	result, _, err := client.EnrichmentServiceGetEnrichments(ctx).Execute()
	if err != nil {
		return nil, err
	}
	enrichments := make([]ess.Enrichment, 0)
	for _, enrichment := range result.Enrichments {
		log.Printf("[INFO] Checking %v", enrichment.GetEnrichmentType())
		switch enrichmentType {
		case SUSIP_TYPE:
			if t := enrichment.EnrichmentType.EnrichmentTypeSuspiciousIp; t != nil {
				enrichments = append(enrichments, enrichment)
			}
			continue
		case AWS_TYPE:
			if t := enrichment.EnrichmentType.EnrichmentTypeAws; t != nil {
				enrichments = append(enrichments, enrichment)
			}
			continue
		case CUSTOM_TYPE:
			if t := enrichment.EnrichmentType.EnrichmentTypeCustomEnrichment.CustomEnrichment; t != nil {
				enrichments = append(enrichments, enrichment)
			}
			continue
		case GEOIP_TYPE:
			if t := enrichment.EnrichmentType.EnrichmentTypeGeoIp.GeoIp; t != nil {
				enrichments = append(enrichments, enrichment)
			}
			continue
		default:
			log.Printf("[WARNING] Unknown enrichment type: %v", enrichmentType)
		}

	}
	log.Printf("[INFO] found %v enrichments for type %v", len(enrichments), enrichmentType)
	return enrichments, nil
}

func ExtractIdsFromEnrichment(fields []CoralogixEnrichment) []uint32 {
	ids := make([]uint32, 0)
	for _, e := range fields {
		ids = append(ids, e.GetId())
	}
	return ids
}
