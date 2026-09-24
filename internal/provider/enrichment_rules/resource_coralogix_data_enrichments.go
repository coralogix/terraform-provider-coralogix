package enrichment_rules

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	cess "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/custom_enrichments_service"
	ess "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/enrichments_service"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-log/tflog"

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
	_ resource.ResourceWithConfigure      = &DataEnrichmentsResource{}
	_ resource.ResourceWithImportState    = &DataEnrichmentsResource{}
	_ resource.ResourceWithValidateConfig = &DataEnrichmentsResource{}
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

	// addEnrichmentsFn and removeEnrichmentsFn are unexported test seams: the
	// generated SDK client is a concrete builder type with no interface, so
	// unit tests substitute these to exercise the Update remove/add/rollback
	// flow without a live backend. Configure wires them to the real builder
	// calls, and Update falls back to the real calls when they are nil.
	addEnrichmentsFn    func(ctx context.Context, rq *ess.EnrichmentsCreationRequest) (*ess.AddEnrichmentsResponse, *http.Response, error)
	removeEnrichmentsFn func(ctx context.Context, ids []int64) (*ess.RemoveEnrichmentsResponse, *http.Response, error)
}

func (r *DataEnrichmentsResource) addEnrichments(ctx context.Context, rq *ess.EnrichmentsCreationRequest) (*ess.AddEnrichmentsResponse, *http.Response, error) {
	if r.addEnrichmentsFn != nil {
		return r.addEnrichmentsFn(ctx, rq)
	}
	return r.client.EnrichmentServiceAddEnrichments(ctx).EnrichmentsCreationRequest(*rq).Execute()
}

func (r *DataEnrichmentsResource) removeEnrichments(ctx context.Context, ids []int64) (*ess.RemoveEnrichmentsResponse, *http.Response, error) {
	if r.removeEnrichmentsFn != nil {
		return r.removeEnrichmentsFn(ctx, ids)
	}
	return r.client.EnrichmentServiceRemoveEnrichments(ctx).EnrichmentIds(ids).Execute()
}

func (e *DataEnrichmentsModel) GetFields() []CoralogixEnrichment {
	fields := make([]CoralogixEnrichment, 0)
	if e.Aws != nil {
		for i := range e.Aws.Fields {
			fields = append(fields, &e.Aws.Fields[i])
		}
	}
	if e.GeoIp != nil {
		for i := range e.GeoIp.Fields {
			fields = append(fields, &e.GeoIp.Fields[i])
		}
	}
	if e.SuspiciousIp != nil {
		for i := range e.SuspiciousIp.Fields {
			fields = append(fields, &e.SuspiciousIp.Fields[i])
		}
	}
	if e.Custom != nil {
		for i := range e.Custom.Fields {
			fields = append(fields, &e.Custom.Fields[i])
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
			ID: types.StringValue(req.ID),
			Custom: &CustomEnrichmentFieldsModel{
				CustomEnrichmentDataModel: &CustomEnrichmentDataModel{
					ID: types.Int64Value(val),
				},
			},
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
	} else {
		resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
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
	r.addEnrichmentsFn = func(ctx context.Context, rq *ess.EnrichmentsCreationRequest) (*ess.AddEnrichmentsResponse, *http.Response, error) {
		return r.client.EnrichmentServiceAddEnrichments(ctx).EnrichmentsCreationRequest(*rq).Execute()
	}
	r.removeEnrichmentsFn = func(ctx context.Context, ids []int64) (*ess.RemoveEnrichmentsResponse, *http.Response, error) {
		return r.client.EnrichmentServiceRemoveEnrichments(ctx).EnrichmentIds(ids).Execute()
	}
}

func (r *DataEnrichmentsResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config *DataEnrichmentsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || config == nil {
		return
	}

	// custom_enrichment_data is required whenever the custom block is set. The
	// nested attribute cannot be marked Required in the schema (that would make
	// the whole custom block mandatory), so enforce the AlsoRequires-style
	// dependency here to avoid a nil dereference in Create/Update.
	if config.Custom != nil && config.Custom.CustomEnrichmentDataModel == nil {
		resp.Diagnostics.AddAttributeError(
			path.Root(CUSTOM_TYPE),
			"Missing custom_enrichment_data",
			"custom_enrichment_data is required when custom is set.",
		)
	}
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
					stringplanmodifier.UseNonNullStateForUnknown(),
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
									Optional: true,
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
									Optional: true,
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
			Optional: true,
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
		cleanupMessage := ""
		if customId != nil {
			_, cleanupHTTPResponse, cleanupErr := r.custom_enrichments_client.
				CustomEnrichmentServiceDeleteCustomEnrichment(ctx, *customId).
				Execute()
			if cleanupErr != nil {
				cleanupMessage = "\nCleanup also failed: " + utils.FormatOpenAPIErrors(
					cxsdkOpenapi.NewAPIError(cleanupHTTPResponse, cleanupErr),
					"Delete",
					customId,
				)
			}
		}
		resp.Diagnostics.AddError("Error creating coralogix_data_enrichments",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", rq)+cleanupMessage,
		)
		return
	}
	var content *string = nil
	if plan.Custom != nil && plan.Custom.CustomEnrichmentDataModel != nil {
		content = plan.Custom.CustomEnrichmentDataModel.Contents.ValueStringPointer()
	}
	state := flattenDataEnrichments(filterDataEnrichmentsForModel(result.Enrichments, plan),
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
	var state *DataEnrichmentsModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	fieldsChanged := !dataEnrichmentFieldsEqual(plan, state)

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
	var content *string
	if plan.Custom != nil && plan.Custom.CustomEnrichmentDataModel != nil {
		content = plan.Custom.CustomEnrichmentDataModel.Contents.ValueStringPointer()
	}

	if !fieldsChanged {
		if uploadResult != nil {
			state.Custom.CustomEnrichmentDataModel = &CustomEnrichmentDataModel{
				ID:          types.Int64PointerValue(uploadResult.Id),
				Name:        types.StringPointerValue(uploadResult.Name),
				Description: types.StringPointerValue(uploadResult.Description),
				Version:     types.Int64PointerValue(uploadResult.Version),
				Contents:    types.StringPointerValue(content),
			}
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
		return
	}

	ids := make([]int64, 0)
	for _, id := range ExtractIdsFromEnrichment(state.GetFields()) {
		ids = append(ids, int64(id))
	}
	// removedRequest captures the exact enrichment set that is about to be
	// removed, derived from the same source (state) the removed ids come from.
	// If the subsequent AddEnrichments fails we best-effort re-add this set so
	// the update is not left in a partially-applied (all enrichments deleted)
	// state.
	removedRequest := extractDataEnrichmentsCreate(state)
	if len(ids) > 0 {
		_, httpResponse, err := r.removeEnrichments(ctx, ids)
		if err != nil {
			resp.Diagnostics.AddError("Error replacing coralogix_data_enrichments",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Delete", ids),
			)
			return
		}
	}

	rq := extractDataEnrichmentsCreate(plan)

	result, httpResponse, err := r.addEnrichments(ctx, rq)
	if err != nil {
		// The add failed after the previous enrichments were already removed.
		// Best-effort rollback: re-add the removed set so we do not leave the
		// resource with all of its enrichments deleted. If the rollback itself
		// fails we log it (without masking the original error) and still return
		// the original error to the user.
		if len(ids) > 0 {
			if _, _, rollbackErr := r.addEnrichments(ctx, removedRequest); rollbackErr != nil {
				tflog.Error(ctx, "failed to roll back removed enrichments after a failed enrichment update; the resource may have no enrichments configured until the next apply", map[string]any{
					"rollback_error": rollbackErr.Error(),
					"original_error": err.Error(),
				})
			}
		}
		resp.Diagnostics.AddError("Error replacing coralogix_data_enrichments. If custom enrichment data was updated, then this update was executed successfully.",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Replace", rq),
		)
		return
	}
	state = flattenDataEnrichments(filterDataEnrichmentsForModel(result.Enrichments, plan),
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

	enrichmentTypes := enrichmentTypesFromModel(state)

	customEnrichmentId := getCustomEnrichmentId(state)
	if len(enrichmentTypes) == 0 && customEnrichmentId == nil {
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
			if httpResponse != nil && httpResponse.StatusCode == http.StatusNotFound {
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
	if len(enrichmentTypes) > 0 {
		result, httpResponse, err := r.client.
			EnrichmentServiceGetEnrichments(ctx).
			Execute()
		if err != nil {
			if httpResponse != nil && httpResponse.StatusCode == http.StatusNotFound {
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
		for _, t := range enrichmentTypes {
			enrichments = append(enrichments, FilterEnrichmentByTypeAndCustomID(result.Enrichments, t, customEnrichmentId)...)
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
		if httpResponse == nil || httpResponse.StatusCode != http.StatusNotFound {
			resp.Diagnostics.AddError("Error deleting coralogix_data_enrichments",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Delete", nil),
			)
		}
	}

	customEnrichmentId := getCustomEnrichmentId(state)
	if customEnrichmentId != nil {
		_, httpResponse, err := r.custom_enrichments_client.
			CustomEnrichmentServiceDeleteCustomEnrichment(ctx, *customEnrichmentId).
			Execute()
		if err != nil {
			if httpResponse != nil && httpResponse.StatusCode == http.StatusNotFound {
				return
			}
			resp.Diagnostics.AddError("Error deleting coralogix_data_enrichments",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Delete", nil),
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
	if plan.Custom != nil && plan.Custom.CustomEnrichmentDataModel != nil {
		ext := "csv"
		return &cess.CreateCustomEnrichmentRequest{
			Name:        plan.Custom.CustomEnrichmentDataModel.Name.ValueString(),
			Description: plan.Custom.CustomEnrichmentDataModel.Description.ValueString(),
			File: cess.File{
				Extension: &ext,
				Name:      plan.Custom.CustomEnrichmentDataModel.Name.ValueStringPointer(),
				Textual:   plan.Custom.CustomEnrichmentDataModel.Contents.ValueStringPointer(),
			},
		}
	}
	return nil
}

func extractCustomEnrichmentsDataUpdate(plan *DataEnrichmentsModel) *cess.UpdateCustomEnrichmentRequest {
	if plan.Custom != nil && plan.Custom.CustomEnrichmentDataModel != nil {
		ext := "csv"
		return &cess.UpdateCustomEnrichmentRequest{
			CustomEnrichmentId: plan.Custom.CustomEnrichmentDataModel.ID.ValueInt64(),
			Name:               plan.Custom.CustomEnrichmentDataModel.Name.ValueString(),
			Description:        plan.Custom.CustomEnrichmentDataModel.Description.ValueString(),
			File: cess.File{
				Extension: &ext,
				Name:      plan.Custom.CustomEnrichmentDataModel.Name.ValueStringPointer(),
				Textual:   plan.Custom.CustomEnrichmentDataModel.Contents.ValueStringPointer(),
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
				Aws: &ess.AwsType{
					ResourceType: f.Resource.ValueStringPointer(),
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
		for _, f := range plan.GeoIp.Fields {
			enrichmentType := ess.EnrichmentType{
				GeoIp: ess.NewGeoIpType(),
			}
			if !(f.Asn.IsNull() || f.Asn.IsUnknown()) {
				enrichmentType.GeoIp.WithAsn = f.Asn.ValueBoolPointer()
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
			SuspiciousIp: map[string]any{},
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

	if plan.Custom != nil && plan.Custom.CustomEnrichmentDataModel != nil {
		id := plan.Custom.CustomEnrichmentDataModel.ID.ValueInt64Pointer()
		for _, f := range plan.Custom.Fields {

			enrichmentType := ess.EnrichmentType{
				CustomEnrichment: &ess.CustomEnrichmentType{
					Id: id,
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

func dataEnrichmentFieldsEqual(plan, state *DataEnrichmentsModel) bool {
	plannedFields := extractDataEnrichments(plan)
	stateFields := extractDataEnrichments(state)
	for i := range plannedFields {
		slices.Sort(plannedFields[i].SelectedColumns)
	}
	for i := range stateFields {
		slices.Sort(stateFields[i].SelectedColumns)
	}
	return reflect.DeepEqual(plannedFields, stateFields)
}

func extractDataEnrichmentsCreate(plan *DataEnrichmentsModel) *ess.EnrichmentsCreationRequest {
	req := &ess.EnrichmentsCreationRequest{
		RequestEnrichments: extractDataEnrichments(plan),
	}
	return req
}

func flattenDataEnrichments(enrichments []ess.Enrichment, uploadResp *cess.CustomEnrichment, customEnrichmentContents *string) *DataEnrichmentsModel {
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
		appendFlattenedDataEnrichment(model, e)
	}
	model.ID = flattenedDataEnrichmentsID(model)
	return model
}

func appendFlattenedDataEnrichment(model *DataEnrichmentsModel, enrichment ess.Enrichment) {
	field := EnrichmentFieldModel{
		EnrichedFieldName: types.StringPointerValue(enrichment.EnrichedFieldName),
		SelectedColumns:   utils.StringSliceToTypeStringSet(enrichment.SelectedColumns),
		Name:              types.StringValue(enrichment.FieldName),
		ID:                types.Int64Value(enrichment.Id),
	}

	switch {
	case enrichment.EnrichmentType.Aws != nil:
		if model.Aws == nil {
			model.Aws = &AwsEnrichmentFieldsModel{}
		}
		model.Aws.Fields = append(model.Aws.Fields, AwsEnrichmentFieldModel{
			EnrichedFieldName: field.EnrichedFieldName,
			SelectedColumns:   field.SelectedColumns,
			Name:              field.Name,
			Resource:          types.StringPointerValue(enrichment.EnrichmentType.Aws.ResourceType),
			ID:                field.ID,
		})
	case enrichment.EnrichmentType.GeoIp != nil:
		if model.GeoIp == nil {
			model.GeoIp = &GeoIpEnrichmentFieldsModel{}
		}
		model.GeoIp.Fields = append(model.GeoIp.Fields, GeoIpEnrichmentFieldModel{
			EnrichedFieldName: field.EnrichedFieldName,
			SelectedColumns:   field.SelectedColumns,
			Name:              field.Name,
			ID:                field.ID,
			Asn:               types.BoolPointerValue(enrichment.EnrichmentType.GeoIp.WithAsn),
		})
	case enrichment.EnrichmentType.SuspiciousIp != nil:
		if model.SuspiciousIp == nil {
			model.SuspiciousIp = &EnrichmentFieldsModel{}
		}
		model.SuspiciousIp.Fields = append(model.SuspiciousIp.Fields, field)
	case enrichment.EnrichmentType.CustomEnrichment != nil:
		if model.Custom == nil {
			model.Custom = &CustomEnrichmentFieldsModel{}
		}
		model.Custom.Fields = append(model.Custom.Fields, field)
	}
}

func flattenedDataEnrichmentsID(model *DataEnrichmentsModel) types.String {
	id := make([]string, 0, 4)
	if model.Aws != nil {
		id = append(id, AWS_TYPE)
	}
	if model.GeoIp != nil {
		id = append(id, GEOIP_TYPE)
	}
	if model.SuspiciousIp != nil {
		id = append(id, SUSIP_TYPE)
	}
	if model.Custom != nil {
		if len(id) == 0 && model.Custom.CustomEnrichmentDataModel != nil && !model.Custom.CustomEnrichmentDataModel.ID.IsNull() {
			id = append(id, strconv.FormatInt(model.Custom.CustomEnrichmentDataModel.ID.ValueInt64(), 10))
		} else {
			id = append(id, CUSTOM_TYPE)
		}
	}
	if len(id) > 0 {
		return types.StringValue(strings.Join(id, ","))
	}
	return types.StringNull()
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
		if t == AWS_TYPE && e.EnrichmentType.Aws != nil {
			results = append(results, e)
		}
		if t == GEOIP_TYPE && e.EnrichmentType.GeoIp != nil {
			results = append(results, e)
		}
		if t == SUSIP_TYPE && e.EnrichmentType.SuspiciousIp != nil {
			results = append(results, e)
		}
		if t == CUSTOM_TYPE && e.EnrichmentType.CustomEnrichment != nil {
			results = append(results, e)
		}
	}
	return results
}

func FilterEnrichmentByTypeAndCustomID(enrichments []ess.Enrichment, enrichmentType string, customEnrichmentID *int64) []ess.Enrichment {
	filtered := FilterEnrichmentByTypes(enrichments, enrichmentType)
	if enrichmentType != CUSTOM_TYPE || customEnrichmentID == nil {
		return filtered
	}

	results := make([]ess.Enrichment, 0, len(filtered))
	for _, enrichment := range filtered {
		id := enrichment.EnrichmentType.CustomEnrichment.Id
		if id != nil && *id == *customEnrichmentID {
			results = append(results, enrichment)
		}
	}
	return results
}

func filterDataEnrichmentsForModel(enrichments []ess.Enrichment, model *DataEnrichmentsModel) []ess.Enrichment {
	customEnrichmentID := getCustomEnrichmentId(model)
	results := make([]ess.Enrichment, 0, len(enrichments))
	if model.Aws != nil {
		results = append(results, FilterEnrichmentByTypeAndCustomID(enrichments, AWS_TYPE, customEnrichmentID)...)
	}
	if model.GeoIp != nil {
		results = append(results, FilterEnrichmentByTypeAndCustomID(enrichments, GEOIP_TYPE, customEnrichmentID)...)
	}
	if model.SuspiciousIp != nil {
		results = append(results, FilterEnrichmentByTypeAndCustomID(enrichments, SUSIP_TYPE, customEnrichmentID)...)
	}
	if model.Custom != nil {
		results = append(results, FilterEnrichmentByTypeAndCustomID(enrichments, CUSTOM_TYPE, customEnrichmentID)...)
	}
	return results
}

func enrichmentTypesFromID(id string) []string {
	if id == "" {
		return nil
	}
	if _, err := strconv.ParseInt(id, 10, 64); err == nil {
		return []string{CUSTOM_TYPE}
	}
	enrichmentTypes := make([]string, 0, 4)
	for _, enrichmentType := range strings.Split(id, ",") {
		if !slices.Contains(enrichmentTypes, enrichmentType) {
			enrichmentTypes = append(enrichmentTypes, enrichmentType)
		}
	}
	return enrichmentTypes
}

func enrichmentTypesFromModel(model *DataEnrichmentsModel) []string {
	if !model.ID.IsNull() && !model.ID.IsUnknown() {
		if enrichmentTypes := enrichmentTypesFromID(model.ID.ValueString()); len(enrichmentTypes) > 0 {
			return enrichmentTypes
		}
	}

	enrichmentTypes := make([]string, 0, 4)
	if model.Aws != nil {
		enrichmentTypes = append(enrichmentTypes, AWS_TYPE)
	}
	if model.GeoIp != nil {
		enrichmentTypes = append(enrichmentTypes, GEOIP_TYPE)
	}
	if model.SuspiciousIp != nil {
		enrichmentTypes = append(enrichmentTypes, SUSIP_TYPE)
	}
	if model.Custom != nil {
		enrichmentTypes = append(enrichmentTypes, CUSTOM_TYPE)
	}
	return enrichmentTypes
}
