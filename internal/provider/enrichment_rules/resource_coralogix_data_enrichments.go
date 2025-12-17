package enrichment_rules

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	cess "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/custom_enrichments_service"
	ess "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/enrichments_service"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
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
	AWS_TYPE    = "aws"
	GEOIP_TYPE  = "geo_ip"
	SUSIP_TYPE  = "suspicious_ip"
	CUSTOM_TYPE = "custom"
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
	CustomEnrichmentDataModel *CustomEnrichmentDataModel `tfsdk:"custom_enrichment_data"`
	Fields                    []EnrichmentFieldModel     `tfsdk:"fields"`
}

type CustomEnrichmentDataModel struct {
	ID          types.Int64  `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Version     types.Int64  `tfsdk:"version"`
	Contents    types.String `tfsdk:"contents"`
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
	client                    *ess.EnrichmentsServiceAPIService
	custom_enrichments_client *cess.CustomEnrichmentsServiceAPIService
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
	idParts := strings.Split(req.ID, ",")
	possibleTypes := []string{AWS_TYPE, SUSIP_TYPE, CUSTOM_TYPE, GEOIP_TYPE}
	if len(idParts) == 0 || len(idParts) > 4 {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with one of %v or 12345 (that's a custom enrichment id). Got: %q", strings.Join(possibleTypes, ","), req.ID),
		)
		return
	}
	isCustomId := false
Outer:
	for _, p := range idParts {
		if !slices.Contains(possibleTypes, strings.ToLower(p)) {
			isCustomId = true
			break Outer
		}
	}

	if isCustomId {
		val, isDataSet := strconv.ParseInt(idParts[0], 10, 64)

		if isDataSet != nil {
			resp.Diagnostics.AddError(
				"Unexpected Import Identifier",
				fmt.Sprintf("Expected import identifier with format: %v or 12345 (that's a custom enrichment id). Got: %q", strings.Join(possibleTypes, ","), req.ID))
			return
		}

		state := DataEnrichmentsModel{
			Custom: &CustomEnrichmentFieldsModel{
				CustomEnrichmentDataModel: &CustomEnrichmentDataModel{
					ID: types.Int64Value(val),
				},
			},
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
	}
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

	r.client, r.custom_enrichments_client = clientSet.DataEnrichments()
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
					"custom_enrichment_data": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"id": schema.Int64Attribute{
								Computed: true,
								PlanModifiers: []planmodifier.Int64{
									int64planmodifier.UseStateForUnknown(),
								},
							},
							"name": schema.StringAttribute{
								Required:    true,
								Description: "A name for the enrichment.",
							},
							"description": schema.StringAttribute{
								Optional:    true,
								Description: "A description.",
							},

							"version": schema.Int64Attribute{
								Computed:    true,
								Description: "The version of the enrichment data.",
							},
							"contents": schema.StringAttribute{
								Required:    true,
								Description: "The file contents to upload. Use Terraform's functions to read from disk.",
							},
						},
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
	// First, upload the custom enrichment (if provided)
	upload := extractCustomEnrichmentsDataCreate(plan)
	var customId *int64 = nil
	var uploadResult *cess.CustomEnrichment
	if upload != nil {
		result, httpResponse, err := r.custom_enrichments_client.
			CustomEnrichmentServiceCreateCustomEnrichment(ctx).
			CreateCustomEnrichmentRequest(*upload).
			Execute()
		if err != nil {
			resp.Diagnostics.AddError("Error uploading custom enrichment coralogix_data_enrichments",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", upload),
			)
			return
		}
		customId = result.CustomEnrichment.Id
		// add ID to the plan for the follow up request
		plan.Custom.CustomEnrichmentDataModel.ID = types.Int64PointerValue(result.CustomEnrichment.Id)
		// store result for "merged flattening"
		uploadResult = result.CustomEnrichment
	}
	rq := extractDataEnrichmentsCreate(plan)
	result, httpResponse, err := r.client.
		EnrichmentServiceAddEnrichments(ctx).
		EnrichmentsCreationRequest(*rq).
		Execute()

	if err != nil {
		if customId != nil {
			r.custom_enrichments_client.
				CustomEnrichmentServiceDeleteCustomEnrichment(ctx, *customId).
				Execute()
		}
		resp.Diagnostics.AddError("Error creating coralogix_data_enrichments",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", rq),
		)
		return
	}
	var content *string = nil
	if plan.Custom != nil && plan.Custom.CustomEnrichmentDataModel != nil {
		content = plan.Custom.CustomEnrichmentDataModel.Contents.ValueStringPointer()
	}
	state := flattenDataEnrichments(result.Enrichments,
		uploadResult,
		// the data isn't actually returned from the request, so we have to keep the state happy like that
		content)

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

	// First, upload/update the custom enrichment (if provided)
	upload := extractCustomEnrichmentsDataUpdate(plan)
	var uploadResult *cess.CustomEnrichment
	if upload != nil {
		result, httpResponse, err := r.custom_enrichments_client.
			CustomEnrichmentServiceUpdateCustomEnrichment(ctx).
			UpdateCustomEnrichmentRequest(*upload).
			Execute()
		if err != nil {
			resp.Diagnostics.AddError("Error uploading custom enrichment coralogix_data_enrichments",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Replace", upload),
			)
			return
		}
		// store result for "merged flattening"
		uploadResult = result.CustomEnrichment
	}

	rq := extractDataEnrichmentsUpdate(plan)

	result, httpResponse, err := r.client.
		EnrichmentServiceAtomicOverwriteEnrichments(ctx).
		EnrichmentServiceAtomicOverwriteEnrichmentsRequest(*rq).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error replacing coralogix_data_enrichments. If custom enrichment data was updated, then this update was executed successfully.",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Replace", rq),
		)
		return
	}
	var content *string = nil
	if plan.Custom != nil && plan.Custom.CustomEnrichmentDataModel != nil {
		content = plan.Custom.CustomEnrichmentDataModel.Contents.ValueStringPointer()
	}
	state := flattenDataEnrichments(result.Enrichments,
		uploadResult,
		content)
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

	id := state.ID.ValueString()
	types := strings.Split(id, ",")

	customEnrichmentId := getCustomEnrichmentId(state)
	if len(types) == 0 && customEnrichmentId == nil {
		resp.Diagnostics.AddError("Error reading coralogix_data_enrichments",
			"No ids found",
		)
		return
	}

	var customEnrichment *cess.CustomEnrichment = nil
	if customEnrichmentId != nil {
		result, httpResponse, err := r.custom_enrichments_client.
			CustomEnrichmentServiceGetCustomEnrichment(ctx, *customEnrichmentId).
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
		customEnrichment = &result.CustomEnrichment
	}
	var enrichments []ess.Enrichment
	if len(types) > 0 {
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
		for _, t := range types {
			enrichments = append(enrichments, FilterEnrichmentByTypes(result.Enrichments, t)...)
		}
	}

	var content *string = nil
	if customEnrichmentId != nil {
		content = state.Custom.CustomEnrichmentDataModel.Contents.ValueStringPointer()
	}
	state = flattenDataEnrichments(enrichments,
		customEnrichment,
		content)

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

	customEnrichmentId := getCustomEnrichmentId(state)
	if customEnrichmentId != nil {
		_, httpResponse, err := r.custom_enrichments_client.
			CustomEnrichmentServiceDeleteCustomEnrichment(ctx, *customEnrichmentId).
			Execute()
		if err != nil {

			resp.Diagnostics.AddError("Error reading coralogix_data_enrichments",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
			)
			return
		}
	}
}

func getCustomEnrichmentId(state *DataEnrichmentsModel) *int64 {
	if state.Custom != nil {
		if state.Custom.CustomEnrichmentDataModel != nil {
			return state.Custom.CustomEnrichmentDataModel.ID.ValueInt64Pointer()
		}
	}
	return nil
}

func extractCustomEnrichmentsDataCreate(plan *DataEnrichmentsModel) *cess.CreateCustomEnrichmentRequest {
	if plan.Custom != nil {
		ext := "csv"
		return &cess.CreateCustomEnrichmentRequest{
			Name:        plan.Custom.CustomEnrichmentDataModel.Name.ValueString(),
			Description: plan.Custom.CustomEnrichmentDataModel.Description.ValueString(),
			File: cess.FileTextualAsFile(&cess.FileTextual{
				Extension: &ext,
				Name:      plan.Custom.CustomEnrichmentDataModel.Name.ValueStringPointer(),
				Textual:   plan.Custom.CustomEnrichmentDataModel.Contents.ValueStringPointer(),
			}),
		}
	}
	return nil
}

func extractCustomEnrichmentsDataUpdate(plan *DataEnrichmentsModel) *cess.UpdateCustomEnrichmentRequest {
	if plan.Custom != nil {
		ext := "csv"
		return &cess.UpdateCustomEnrichmentRequest{
			CustomEnrichmentId: plan.Custom.CustomEnrichmentDataModel.ID.ValueInt64(),
			Name:               plan.Custom.CustomEnrichmentDataModel.Name.ValueString(),
			Description:        plan.Custom.CustomEnrichmentDataModel.Description.ValueString(),
			File: cess.File{
				FileTextual: &cess.FileTextual{
					Extension: &ext,
					Name:      plan.Custom.CustomEnrichmentDataModel.Name.ValueStringPointer(),
					Textual:   plan.Custom.CustomEnrichmentDataModel.Contents.ValueStringPointer(),
				},
			},
		}
	}
	return nil
}

func extractDataEnrichments(plan *DataEnrichmentsModel) []ess.EnrichmentRequestModel {
	requestModels := make([]ess.EnrichmentRequestModel, 0)
	ctx := context.Background()
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
				SuspiciousIp: map[string]any{},
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
		id := plan.Custom.CustomEnrichmentDataModel.ID.ValueInt64Pointer()
		for _, f := range plan.Custom.Fields {

			enrichmentType := ess.EnrichmentType{
				EnrichmentTypeCustomEnrichment: &ess.EnrichmentTypeCustomEnrichment{
					CustomEnrichment: &ess.CustomEnrichmentType{
						Id: id,
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
		// some server side validation wants this
		EnrichmentType: &ess.EnrichmentType{EnrichmentTypeSuspiciousIp: &ess.EnrichmentTypeSuspiciousIp{
			SuspiciousIp: map[string]any{},
		}},
	}
	return req
}

func flattenDataEnrichments(enrichments []ess.Enrichment, uploadResp *cess.CustomEnrichment, customEnrichmentContents *string) *DataEnrichmentsModel {
	id := make([]string, 0)
	model := &DataEnrichmentsModel{}

	if uploadResp != nil {
		model.Custom = &CustomEnrichmentFieldsModel{
			CustomEnrichmentDataModel: &CustomEnrichmentDataModel{
				ID:          types.Int64PointerValue(uploadResp.Id),
				Name:        types.StringPointerValue(uploadResp.Name),
				Description: types.StringPointerValue(uploadResp.Description),
				Version:     types.Int64PointerValue(uploadResp.Version),
				Contents:    types.StringPointerValue(customEnrichmentContents),
			},
			Fields: []EnrichmentFieldModel{},
		}
	}

	for _, e := range enrichments {
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
			id = append(id, AWS_TYPE)
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
			id = append(id, GEOIP_TYPE)
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
			id = append(id, SUSIP_TYPE)
		} else if e.EnrichmentType.EnrichmentTypeCustomEnrichment != nil {
			if model.Custom == nil {
				model.Custom = &CustomEnrichmentFieldsModel{}
			}

			model.Custom.Fields = append(model.Custom.Fields, EnrichmentFieldModel{
				EnrichedFieldName: types.StringPointerValue(e.EnrichedFieldName),
				SelectedColumns:   utils.StringSliceToTypeStringSet(e.SelectedColumns),
				Name:              types.StringValue(e.FieldName),
				ID:                types.Int64Value(e.Id),
			})
			id = append(id, CUSTOM_TYPE)
		}
	}
	model.ID = types.StringValue(strings.Join(id, ","))
	return model
}

func ExtractIdsFromEnrichment(fields []CoralogixEnrichment) []uint32 {
	ids := make([]uint32, 0)
	for _, e := range fields {
		ids = append(ids, e.GetId())
	}
	return ids
}

func FilterEnrichmentByTypes(enrichments []ess.Enrichment, t string) []ess.Enrichment {
	results := make([]ess.Enrichment, 0)
	for _, e := range enrichments {
		if t == AWS_TYPE && e.EnrichmentType.EnrichmentTypeAws != nil {
			results = append(results, e)
		}
		if t == GEOIP_TYPE && e.EnrichmentType.EnrichmentTypeGeoIp != nil {
			results = append(results, e)
		}
		if t == SUSIP_TYPE && e.EnrichmentType.EnrichmentTypeSuspiciousIp != nil {
			results = append(results, e)
		}
		if t == CUSTOM_TYPE && e.EnrichmentType.EnrichmentTypeCustomEnrichment != nil {
			results = append(results, e)
		}
	}
	return results
}
