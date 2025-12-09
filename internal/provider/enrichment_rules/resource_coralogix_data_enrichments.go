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
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure   = &DataEnrichmentsResource{}
	_ resource.ResourceWithImportState = &DataEnrichmentsResource{}
)

const (
	AWS_TYPE                     = "aws"
	GEOIP_TYPE                   = "geo_ip"
	SUSIP_TYPE                   = "suspicious_ip"
	CUSTOM_TYPE                  = "custom"
	RESOURCE_ID_DATA_ENRICHMENTS = "data-enrichment-settings"
)

type CoralogixEnrichment interface {
	GetId() uint32
}

type DataEnrichmentsModel struct {
	ID           types.String                 `tfsdk:"id"`
	Aws          *AwsEnrichmentFieldsModel    `tfsdk:"aws"`
	GeoIp        *GeoIpEnrichmentFieldsModel  `tfsdk:"geo_ip"`
	SuspiciousIp *EnrichmentFieldsModel       `tfsdk:"suspicious_ip"`
	Custom       *CustomEnrichmentFieldsModel `tfsdk:"custom"`
}

type GeoIpEnrichmentFieldsModel struct {
	Fields []GeoIpEnrichmentFieldModel `tfsdk:"fields"`
}

type EnrichmentFieldsModel struct {
	Fields []EnrichmentFieldModel `tfsdk:"fields"`
}

type AwsEnrichmentFieldsModel struct {
	Fields []AwsEnrichmentFieldModel `tfsdk:"fields"`
}

type GeoIpEnrichmentFieldModel struct {
	ID                types.Int64  `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Asn               types.Bool   `tfsdk:"with_asn"`
	EnrichedFieldName types.String `tfsdk:"enriched_field_name"`
	SelectedColumns   types.Set    `tfsdk:"selected_columns"`
}

type EnrichmentFieldModel struct {
	ID                types.Int64  `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	EnrichedFieldName types.String `tfsdk:"enriched_field_name"`
	SelectedColumns   types.Set    `tfsdk:"selected_columns"`
}

type AwsEnrichmentFieldModel struct {
	ID                types.Int64  `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	EnrichedFieldName types.String `tfsdk:"enriched_field_name"`
	SelectedColumns   types.Set    `tfsdk:"selected_columns"`
	Resource          types.String `tfsdk:"resource"`
}

type CustomEnrichmentFieldsModel struct {
	CustomEnrichmentId types.Int64            `tfsdk:"custom_enrichment_id"`
	Fields             []EnrichmentFieldModel `tfsdk:"fields"`
}

func (e AwsEnrichmentFieldModel) GetId() uint32 {
	return uint32(e.ID.ValueInt64())
}

func (e EnrichmentFieldModel) GetId() uint32 {
	return uint32(e.ID.ValueInt64())
}

func (e GeoIpEnrichmentFieldModel) GetId() uint32 {
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
	resp.TypeName = req.ProviderTypeName + "_data_enrichments"
}

func (r *DataEnrichmentsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			GEOIP_TYPE: schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"fields": schema.ListNestedAttribute{
						Required: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"with_asn": schema.BoolAttribute{
									Optional: true,
									Computed: true,
									PlanModifiers: []planmodifier.Bool{
										boolplanmodifier.UseStateForUnknown(),
									},
								},
								"name": schema.StringAttribute{
									Required: true,
								},
								"enriched_field_name": schema.StringAttribute{
									Required: true,
								},
								"selected_columns": schema.SetAttribute{
									ElementType: types.StringType,
									Optional:    true,
									Computed:    true,
									PlanModifiers: []planmodifier.Set{
										setplanmodifier.UseStateForUnknown(),
									},
								},
								"id": schema.Int64Attribute{
									Optional: true,
									Computed: true,
								},
							},
						},
						MarkdownDescription: "Set of fields to enrich with geo_ip information.",
					},
				},
				MarkdownDescription: "Coralogix allows you to enrich your logs with location data by automatically converting IPs to Geo-points which can be used to aggregate logs by location and create Map visualizations in Kibana.",
			},
			SUSIP_TYPE: schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"fields": schema.ListNestedAttribute{
						Required: true,
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
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"fields": schema.ListNestedAttribute{
						Required: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"resource": schema.StringAttribute{
									Required: true,
								},
								"name": schema.StringAttribute{
									Required: true,
								},
								"id": schema.Int64Attribute{
									Optional: true,
									Computed: true,
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
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"custom_enrichment_id": schema.Int64Attribute{
						Optional: true,
						Computed: true,
					},
					"fields": schema.ListNestedAttribute{
						Required: true,
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
		resp.Diagnostics.AddError("Error creating coralogix_data_enrichments",
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
		resp.Diagnostics.AddError("Error replacing coralogix_data_enrichments",
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
				"coralogix_data_enrichments is in state, but no longer exists in Coralogix backend",
				"coralogix_data_enrichments will be recreated when you apply",
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error reading coralogix_data_enrichments",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
			)
		}
		return
	}

	state = flattenDataEnrichments(result.Enrichments)
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
		resp.Diagnostics.AddError("Error deleting coralogix_data_enrichments",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
		)
	}
}

func extractDataEnrichments(plan *DataEnrichmentsModel) []ess.EnrichmentRequestModel {
	requestModels := make([]ess.EnrichmentRequestModel, 0)
	ctx := context.TODO()
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
				SelectedColumns:   utils.TypeStringSetToStringSlice(ctx, f.SelectedColumns),
				EnrichmentType:    enrichmentType,
			})
		}
	}

	if plan.GeoIp != nil {
		enrichmentType := ess.EnrichmentType{
			EnrichmentTypeGeoIp: &ess.EnrichmentTypeGeoIp{
				GeoIp: ess.NewGeoIpType(),
			},
		}
		for _, f := range plan.GeoIp.Fields {
			if !(f.Asn.IsNull() || f.Asn.IsUnknown()) {
				enrichmentType.EnrichmentTypeGeoIp.GeoIp.WithAsn = f.Asn.ValueBoolPointer()
			}
			requestModels = append(requestModels, ess.EnrichmentRequestModel{
				EnrichedFieldName: f.EnrichedFieldName.ValueStringPointer(),
				FieldName:         f.Name.ValueString(),
				SelectedColumns:   utils.TypeStringSetToStringSlice(ctx, f.SelectedColumns),
				EnrichmentType:    enrichmentType,
			})
		}
	}

	if plan.SuspiciousIp != nil {
		enrichmentType := ess.EnrichmentType{
			EnrichmentTypeSuspiciousIp: &ess.EnrichmentTypeSuspiciousIp{
				SuspiciousIp: map[string]interface{}{},
			},
		}
		for _, f := range plan.SuspiciousIp.Fields {
			requestModels = append(requestModels, ess.EnrichmentRequestModel{
				EnrichedFieldName: f.EnrichedFieldName.ValueStringPointer(),
				FieldName:         f.Name.ValueString(),
				SelectedColumns:   utils.TypeStringSetToStringSlice(ctx, f.SelectedColumns),
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
				SelectedColumns:   utils.TypeStringSetToStringSlice(ctx, f.SelectedColumns),
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
	model := &DataEnrichmentsModel{
		ID: types.StringValue(RESOURCE_ID_DATA_ENRICHMENTS),
	}
	for _, e := range rgrp {
		if e.EnrichmentType.EnrichmentTypeAws != nil {
			if model.Aws == nil {
				model.Aws = &AwsEnrichmentFieldsModel{}
			}
			model.Aws.Fields = append(model.Aws.Fields, AwsEnrichmentFieldModel{
				EnrichedFieldName: types.StringPointerValue(e.EnrichedFieldName),
				SelectedColumns:   utils.StringSliceToTypeStringSet(e.SelectedColumns),
				Name:              types.StringValue(e.FieldName),
				Resource:          types.StringPointerValue(e.EnrichmentType.EnrichmentTypeAws.Aws.ResourceType),
				ID:                types.Int64Value(e.Id),
			})
		} else if e.EnrichmentType.EnrichmentTypeGeoIp != nil {
			if model.GeoIp == nil {
				model.GeoIp = &GeoIpEnrichmentFieldsModel{}
			}
			model.GeoIp.Fields = append(model.GeoIp.Fields, GeoIpEnrichmentFieldModel{
				EnrichedFieldName: types.StringPointerValue(e.EnrichedFieldName),
				SelectedColumns:   utils.StringSliceToTypeStringSet(e.SelectedColumns),
				Name:              types.StringValue(e.FieldName),
				ID:                types.Int64Value(e.Id),
				Asn:               types.BoolPointerValue(e.EnrichmentType.EnrichmentTypeGeoIp.GeoIp.WithAsn),
			})
		} else if e.EnrichmentType.EnrichmentTypeSuspiciousIp != nil {
			if model.SuspiciousIp == nil {
				model.SuspiciousIp = &EnrichmentFieldsModel{}
			}
			model.SuspiciousIp.Fields = append(model.SuspiciousIp.Fields, EnrichmentFieldModel{
				EnrichedFieldName: types.StringPointerValue(e.EnrichedFieldName),
				SelectedColumns:   utils.StringSliceToTypeStringSet(e.SelectedColumns),
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
				SelectedColumns:   utils.StringSliceToTypeStringSet(e.SelectedColumns),
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
