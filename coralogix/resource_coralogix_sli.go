package coralogix

import (
	"context"
	"fmt"
	"log"

	"terraform-provider-coralogix/coralogix/clientset"
	sli "terraform-provider-coralogix/coralogix/clientset/grpc/sli"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	_                          resource.ResourceWithConfigure   = &SLIResource{}
	_                          resource.ResourceWithImportState = &SLIResource{}
	sliProtoToSchemaMetricType                                  = map[sli.MetricType]string{
		sli.MetricType_METRIC_TYPE_UNSPECIFIED: "unspecified",
		sli.MetricType_METRIC_TYPE_ERROR:       "error",
		sli.MetricType_METRIC_TYPE_LATENCY:     "latency",
		sli.MetricType_METRIC_TYPE_CUSTOM:      "custom",
	}
	sliSchemaToProtoMetricType = ReverseMap(sliProtoToSchemaMetricType)
	sliValidMetricTypes        = GetKeys(sliSchemaToProtoMetricType)
	sliProtoToSchemaPeriodType = map[sli.SloPeriodType]string{
		sli.SloPeriodType_SLO_PERIOD_TYPE_UNSPECIFIED: "unspecified",
		sli.SloPeriodType_SLO_PERIOD_TYPE_7_DAYS:      "7_days",
		sli.SloPeriodType_SLO_PERIOD_TYPE_14_DAYS:     "14_days",
		sli.SloPeriodType_SLO_PERIOD_TYPE_30_DAYS:     "30_days",
	}
	sliSchemaToProtoPeriodType          = ReverseMap(sliProtoToSchemaPeriodType)
	sliValidPeriodTypes                 = GetKeys(sliSchemaToProtoPeriodType)
	sliProtoToSchemaThresholdSymbolType = map[sli.ThresholdSymbolType]string{
		sli.ThresholdSymbolType_THRESHOLD_SYMBOL_TYPE_UNSPECIFIED:      "unspecified",
		sli.ThresholdSymbolType_THRESHOLD_SYMBOL_TYPE_GREATER:          "greater",
		sli.ThresholdSymbolType_THRESHOLD_SYMBOL_TYPE_GREATER_OR_EQUAL: "greater_or_equal",
		sli.ThresholdSymbolType_THRESHOLD_SYMBOL_TYPE_LESS:             "less",
		sli.ThresholdSymbolType_THRESHOLD_SYMBOL_TYPE_LESS_OR_EQUAL:    "less_or_equal",
		sli.ThresholdSymbolType_THRESHOLD_SYMBOL_TYPE_EQUAL:            "equal",
		sli.ThresholdSymbolType_THRESHOLD_SYMBOL_TYPE_NOT_EQUAL:        "not_equal",
	}
	sliSchemaToProtoThresholdSymbolType = ReverseMap(sliProtoToSchemaThresholdSymbolType)
	sliValidThresholdSymbolTypes        = GetKeys(sliSchemaToProtoThresholdSymbolType)
	sliProtoToSchemaSloStatusType       = map[sli.SloStatusType]string{
		sli.SloStatusType_SLO_STATUS_TYPE_UNSPECIFIED: "unspecified",
		sli.SloStatusType_SLO_STATUS_TYPE_OK:          "ok",
		sli.SloStatusType_SLO_STATUS_TYPE_BREACHED:    "breached",
	}
	sliSchemaToProtoSloStatusType = ReverseMap(sliProtoToSchemaSloStatusType)
	sliProtoToSchemaTimeUnitType  = map[sli.TimeUnitType]string{
		sli.TimeUnitType_TIME_UNIT_TYPE_UNSPECIFIED: "unspecified",
		sli.TimeUnitType_TIME_UNIT_TYPE_MICROSECOND: "microsecond",
		sli.TimeUnitType_TIME_UNIT_TYPE_MILLISECOND: "millisecond",
		sli.TimeUnitType_TIME_UNIT_TYPE_SECOND:      "second",
		sli.TimeUnitType_TIME_UNIT_TYPE_MINUTE:      "minute",
	}
	sliSchemaToProtoTimeUnitType = ReverseMap(sliProtoToSchemaTimeUnitType)
	sliValidTimeUnitTypes        = GetKeys(sliSchemaToProtoTimeUnitType)
	sliProtoToSchemaCompareType  = map[sli.CompareType]string{
		sli.CompareType_COMPARE_TYPE_UNSPECIFIED: "unspecified",
		sli.CompareType_COMPARE_TYPE_IS:          "is",
		sli.CompareType_COMPARE_TYPE_START_WITH:  "start_with",
		sli.CompareType_COMPARE_TYPE_ENDS_WITH:   "ends_with",
		sli.CompareType_COMPARE_TYPE_INCLUDES:    "includes",
	}
	sliSchemaToProtoCompareType = ReverseMap(sliProtoToSchemaCompareType)
	sliValidCompareTypes        = GetKeys(sliSchemaToProtoCompareType)
	createSliURL                = "com.coralogix.catalog.v1.SliService/CreateSli"
	getSliURL                   = "com.coralogix.catalog.v1.SliService/GetSli"
	updateSliURL                = "com.coralogix.catalog.v1.SliService/UpdateSli"
	deleteSliURL                = "com.coralogix.catalog.v1.SliService/DeleteSli"
)

func NewSLIResource() resource.Resource {
	return &SLIResource{}
}

type SLIResource struct {
	client *clientset.SLIClient
}

func (r *SLIResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sli"

}

func (r *SLIResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.SLIs()
}

func (r *SLIResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *SLIResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "SLI ID.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "SLI name.",
			},
			"service_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Service name. This is the name of the service that the SLI is associated with.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
				MarkdownDescription: "Optional SLI description.",
			},
			"metric_name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
				MarkdownDescription: "Metric name. This is the name of the metric that the SLI is associated with.",
			},
			"metric_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf(sliValidMetricTypes...),
				},
				Default:             stringdefault.StaticString("unspecified"),
				MarkdownDescription: fmt.Sprintf("Metric type. This is the type of the metric that the SLI is associated with. Valid values are: %q.", sliValidMetricTypes),
			},
			"slo_percentage": schema.Int64Attribute{
				Required: true,
				Validators: []validator.Int64{
					int64validator.Between(0, 100),
				},
			},
			"slo_period_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf(sliValidPeriodTypes...),
				},
				Default: stringdefault.StaticString("unspecified"),
			},
			"threshold_symbol_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf(sliValidThresholdSymbolTypes...),
				},
				Default: stringdefault.StaticString("unspecified"),
			},
			"threshold_value": schema.Int64Attribute{
				Optional: true,
			},
			"filters": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"field": schema.StringAttribute{
							Required: true,
						},
						"compare_type": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.OneOf(sliValidCompareTypes...),
							},
						},
						"field_values": schema.ListAttribute{
							Required:    true,
							ElementType: types.StringType,
						},
					},
				},
			},
			"slo_status_type": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "SLO status type. This is the status of the SLI. Valid values are: `ok` and `breached`.",
			},
			"error_budget": schema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"label_e2m_id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"total_e2m_id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"time_unit_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf(sliValidTimeUnitTypes...),
				},
				Default:             stringdefault.StaticString("unspecified"),
				MarkdownDescription: fmt.Sprintf("Time unit type. This is the time unit type of the metric that the SLI is associated with. Valid values are: %q.", sliValidTimeUnitTypes),
			},
			"service_names_group": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
		},
		MarkdownDescription: "Coralogix SLI. For more information - https://coralogix.com/docs/service-catalog/#sli-tab",
	}
}

type SLIResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	ServiceName         types.String `tfsdk:"service_name"`
	Description         types.String `tfsdk:"description"`
	MetricName          types.String `tfsdk:"metric_name"`
	MetricType          types.String `tfsdk:"metric_type"`
	SloPercentage       types.Int64  `tfsdk:"slo_percentage"`
	SloPeriodType       types.String `tfsdk:"slo_period_type"`
	ThresholdSymbolType types.String `tfsdk:"threshold_symbol_type"`
	ThresholdValue      types.Int64  `tfsdk:"threshold_value"`
	Filters             types.List   `tfsdk:"filters"` //SLIFilterModel
	SloStatusType       types.String `tfsdk:"slo_status_type"`
	ErrorBudget         types.Int64  `tfsdk:"error_budget"`
	LabelE2MID          types.String `tfsdk:"label_e2m_id"`
	TotalE2MID          types.String `tfsdk:"total_e2m_id"`
	TimeUnitType        types.String `tfsdk:"time_unit_type"`
	ServiceNamesGroup   types.List   `tfsdk:"service_names_group"`
}

type SLIFilterModel struct {
	Field       types.String `tfsdk:"field"`
	CompareType types.String `tfsdk:"compare_type"`
	FieldValues types.List   `tfsdk:"field_values"` //string
}

func (r *SLIResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SLIResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createSLIRequest, diags := extractCreateSLI(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
	log.Printf("[INFO] Creating new SLI: %s", protojson.Format(createSLIRequest))
	createResp, err := r.client.CreateSLI(ctx, createSLIRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		resp.Diagnostics.AddError(
			"Error creating SLI",
			formatRpcErrors(err, createSliURL, protojson.Format(createSLIRequest)),
		)
		return
	}
	sli := createResp.GetSli()
	log.Printf("[INFO] Submitted new SLI: %s", protojson.Format(sli))
	plan.ID = types.StringValue(sli.GetSliId().GetValue())
	plan, diags = flattenSLI(ctx, sli)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func flattenSLI(ctx context.Context, sli *sli.Sli) (SLIResourceModel, diag.Diagnostics) {
	filters, diags := flattenSLIFilters(ctx, sli.GetFilters())
	if diags.HasError() {
		return SLIResourceModel{}, diags
	}

	return SLIResourceModel{
		ID:                  wrapperspbStringToTypeString(sli.GetSliId()),
		Name:                wrapperspbStringToTypeString(sli.GetSliName()),
		ServiceName:         wrapperspbStringToTypeString(sli.GetServiceName()),
		Description:         wrapperspbStringToTypeString(sli.GetSliDescription()),
		MetricName:          wrapperspbStringToTypeString(sli.GetMetricName()),
		MetricType:          types.StringValue(sliProtoToSchemaMetricType[sli.GetMetricType()]),
		SloPercentage:       wrapperspbInt64ToTypeInt64(sli.GetSloPercentage()),
		SloPeriodType:       types.StringValue(sliProtoToSchemaPeriodType[sli.GetSloPeriodType()]),
		ThresholdSymbolType: types.StringValue(sliProtoToSchemaThresholdSymbolType[sli.GetThresholdSymbolType()]),
		ThresholdValue:      wrapperspbInt64ToTypeInt64(sli.GetThresholdValue()),
		Filters:             filters,
		SloStatusType:       types.StringValue(sliProtoToSchemaSloStatusType[sli.GetSloStatusType()]),
		ErrorBudget:         wrapperspbInt64ToTypeInt64(sli.GetErrorBudget()),
		LabelE2MID:          wrapperspbStringToTypeString(sli.GetLabelE2MId()),
		TotalE2MID:          wrapperspbStringToTypeString(sli.GetTotalE2MId()),
		TimeUnitType:        types.StringValue(sliProtoToSchemaTimeUnitType[sli.GetTimeUnitType()]),
		ServiceNamesGroup:   wrappedStringSliceToTypeStringList(sli.GetServiceNamesGroup()),
	}, nil
}

func flattenSLIFilters(ctx context.Context, filters []*sli.SliFilter) (types.List, diag.Diagnostics) {
	if len(filters) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: sliFilterModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0, len(filters))
	for _, filter := range filters {
		flattenedSLIFilter := flattenSLIFilter(filter)
		filterElement, diags := types.ObjectValueFrom(ctx, sliFilterModelAttr(), flattenedSLIFilter)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		filtersElements = append(filtersElements, filterElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: sliFilterModelAttr()}, filtersElements), diagnostics
}

func sliFilterModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field":        types.StringType,
		"compare_type": types.StringType,
		"field_values": types.ListType{
			ElemType: types.StringType,
		},
	}
}

func flattenSLIFilter(filter *sli.SliFilter) *SLIFilterModel {
	if filter == nil {
		return nil
	}

	return &SLIFilterModel{
		Field:       wrapperspbStringToTypeString(filter.GetField()),
		CompareType: types.StringValue(sliProtoToSchemaCompareType[filter.GetCompareType()]),
		FieldValues: wrappedStringSliceToTypeStringList(filter.GetFieldValues()),
	}
}

func extractCreateSLI(ctx context.Context, plan SLIResourceModel) (*sli.CreateSliRequest, diag.Diagnostics) {
	SLI, diags := extractSLI(ctx, plan)
	if diags.HasError() {
		return nil, diags
	}

	return &sli.CreateSliRequest{
		Sli: SLI,
	}, nil
}

func extractSLI(ctx context.Context, plan SLIResourceModel) (*sli.Sli, diag.Diagnostics) {
	serviceNamesGroup, _ := typeStringSliceToWrappedStringSlice(ctx, plan.ServiceNamesGroup.Elements())
	filters, diags := expandSLIFilters(ctx, plan.Filters)
	if diags.HasError() {
		return nil, diags
	}

	return &sli.Sli{
		SliId:               typeStringToWrapperspbString(plan.ID),
		SliName:             typeStringToWrapperspbString(plan.Name),
		SliDescription:      typeStringToWrapperspbString(plan.Description),
		ServiceName:         typeStringToWrapperspbString(plan.ServiceName),
		MetricName:          typeStringToWrapperspbString(plan.MetricName),
		MetricType:          sliSchemaToProtoMetricType[plan.MetricType.ValueString()],
		SloPercentage:       typeInt64ToWrappedInt64(plan.SloPercentage),
		SloPeriodType:       sliSchemaToProtoPeriodType[plan.SloPeriodType.ValueString()],
		ThresholdSymbolType: sliSchemaToProtoThresholdSymbolType[plan.ThresholdSymbolType.ValueString()],
		ThresholdValue:      typeInt64ToWrappedInt64(plan.ThresholdValue),
		Filters:             filters,
		SloStatusType:       sliSchemaToProtoSloStatusType[plan.SloStatusType.ValueString()],
		ErrorBudget:         typeInt64ToWrappedInt64(plan.ErrorBudget),
		LabelE2MId:          typeStringToWrapperspbString(plan.LabelE2MID),
		TotalE2MId:          typeStringToWrapperspbString(plan.TotalE2MID),
		TimeUnitType:        sliSchemaToProtoTimeUnitType[plan.TimeUnitType.ValueString()],
		ServiceNamesGroup:   serviceNamesGroup,
	}, nil
}

func expandSLIFilters(ctx context.Context, filters types.List) ([]*sli.SliFilter, diag.Diagnostics) {
	var diags diag.Diagnostics
	var filtersObjects []types.Object
	var expandedFilters []*sli.SliFilter
	filters.ElementsAs(ctx, &filtersObjects, true)

	for _, fo := range filtersObjects {
		var filter SLIFilterModel
		if dg := fo.As(ctx, &filter, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedFilter, expandDiags := expandSLIFilter(ctx, &filter)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedFilters = append(expandedFilters, expandedFilter)
	}

	return expandedFilters, diags
}

func expandSLIFilter(ctx context.Context, SLI *SLIFilterModel) (*sli.SliFilter, diag.Diagnostics) {
	if SLI == nil {
		return nil, nil
	}

	fieldValues, diags := typeStringSliceToWrappedStringSlice(ctx, SLI.FieldValues.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &sli.SliFilter{
		Field:       typeStringToWrapperspbString(SLI.Field),
		CompareType: sliSchemaToProtoCompareType[SLI.CompareType.ValueString()],
		FieldValues: fieldValues,
	}, nil
}

func (r *SLIResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SLIResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed SLI value from Coralogix
	id := state.ID.ValueString()
	serviceName := state.ServiceName.ValueString()
	log.Printf("[INFO] Reading SLIs of service: %s", serviceName)
	getSliReq := &sli.GetSlisRequest{ServiceName: wrapperspb.String(serviceName)}
	getSLIsResp, err := r.client.GetSLIs(ctx, getSliReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if status.Code(err) == codes.NotFound {
			state.ID = types.StringNull()
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("SLI %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error reading SLI",
				formatRpcErrors(err, getSliURL, protojson.Format(getSliReq)),
			)
		}
		return
	}

	var SLI *sli.Sli
	for _, sli := range getSLIsResp.GetSlis() {
		if sli.GetSliId().GetValue() == id {
			SLI = sli
			break
		}
	}
	if SLI == nil {
		state.ID = types.StringNull()
		resp.Diagnostics.AddWarning(
			fmt.Sprintf("SLI %q is in state, but no longer exists in Coralogix backend", id),
			fmt.Sprintf("%s will be recreated when you apply", id),
		)
		return
	}

	sliStr := protojson.Format(SLI)
	log.Printf("[INFO] Received SLI: %s", sliStr)

	state, diags = flattenSLI(ctx, SLI)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
	//
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *SLIResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan SLIResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	SLI, diags := extractSLI(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}

	updateSliReq := &sli.UpdateSliRequest{Sli: SLI}
	log.Printf("[INFO] Updating SLI: %s", protojson.Format(updateSliReq))
	updateSliResp, err := r.client.UpdateSLI(ctx, updateSliReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		resp.Diagnostics.AddError(
			"Error updating SLI",
			formatRpcErrors(err, updateSliURL, protojson.Format(updateSliReq)),
		)
		return
	}
	log.Printf("[INFO] Submitted updated SLI: %s", updateSliResp)

	// Get refreshed SLI value from Coralogix
	id := plan.ID.ValueString()
	serviceName := plan.ServiceName.ValueString()
	getSliReq := &sli.GetSlisRequest{ServiceName: wrapperspb.String(serviceName)}
	getSLIsResp, err := r.client.GetSLIs(ctx, getSliReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if status.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("SLI %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error reading SLI",
				formatRpcErrors(err, getSliURL, protojson.Format(getSliReq)),
			)
		}
		return
	}

	SLI = nil
	for _, sli := range getSLIsResp.GetSlis() {
		if sli.GetSliId().GetValue() == id {
			SLI = sli
			break
		}
	}
	if SLI == nil {
		resp.Diagnostics.AddWarning(
			fmt.Sprintf("SLI %q is in state, but no longer exists in Coralogix backend", id),
			fmt.Sprintf("%s will be recreated when you apply", id),
		)
		return
	}

	log.Printf("[INFO] Received SLI: %s", protojson.Format(SLI))

	plan, diags = flattenSLI(ctx, SLI)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *SLIResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state SLIResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	log.Printf("[INFO] Deleting SLI %s\n", id)
	deleteReq := &sli.DeleteSliRequest{SliId: wrapperspb.String(id)}
	if _, err := r.client.DeleteSLI(ctx, deleteReq); err != nil {
		reqStr := protojson.Format(deleteReq)
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting SLI %s", state.ID.ValueString()),
			formatRpcErrors(err, deleteSliURL, reqStr),
		)
		return
	}
	log.Printf("[INFO] SLI %s deleted\n", id)
}
