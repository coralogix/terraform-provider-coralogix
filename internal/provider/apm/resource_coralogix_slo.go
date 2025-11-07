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

package apm

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	_                                resource.ResourceWithConfigure        = &SLOResource{}
	_                                resource.ResourceWithImportState      = &SLOResource{}
	_                                resource.ResourceWithConfigValidators = &SLOResource{}
	protoToSchemaThresholdSymbolType                                       = map[cxsdk.ThresholdSymbol]string{
		cxsdk.SloThresholdSymbolGreaterOrEqual: "greater_or_equal",
		cxsdk.SloThresholdSymbolGreater:        "greater",
		cxsdk.SloThresholdSymbolLess:           "less",
		cxsdk.SloThresholdSymbolLessOrEqual:    "less_or_equal",
		cxsdk.SloThresholdSymbolEqual:          "equal",
	}
	schemaToProtoThresholdSymbolType = utils.ReverseMap(protoToSchemaThresholdSymbolType)
	validThresholdSymbolTypes        = utils.GetKeys(schemaToProtoThresholdSymbolType)
	protoToSchemaSLOCompareType      = map[cxsdk.CompareType]string{
		cxsdk.SloCompareTypeUnspecified: utils.UNSPECIFIED,
		cxsdk.SloCompareTypeIs:          "is",
		cxsdk.SloCompareTypeStartsWith:  "starts_with",
		cxsdk.SloCompareTypeEndsWith:    "ends_with",
		cxsdk.SloCompareTypeIncludes:    "includes",
	}
	schemaToProtoSLOCompareType = utils.ReverseMap(protoToSchemaSLOCompareType)
	validSLOCompareTypes        = utils.GetKeys(schemaToProtoSLOCompareType)
	protoToSchemaSLOPeriod      = map[cxsdk.SloPeriod]string{
		cxsdk.SloPeriodUnspecified: utils.UNSPECIFIED,
		cxsdk.SloPeriod7Days:       "7_days",
		cxsdk.SloPeriod14Days:      "14_days",
		cxsdk.SloPeriod30Days:      "30_days",
	}
	schemaToProtoSLOPeriod = utils.ReverseMap(protoToSchemaSLOPeriod)
	validSLOPeriods        = utils.GetKeys(schemaToProtoSLOPeriod)
	protoToSchemaSLOStatus = map[cxsdk.SloStatus]string{
		cxsdk.LegacySloStatusUnspecified: utils.UNSPECIFIED,
		cxsdk.LegacySloStatusOk:          "ok",
		cxsdk.LegacySloStatusBreached:    "breached",
	}
)

func NewSLOResource() resource.Resource {
	return &SLOResource{}
}

type SLOResource struct {
	client *cxsdk.LegacySLOsClient
}

func (r *SLOResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		SLOResourceValidator{},
	}
}

type SLOResourceValidator struct {
}

func (S SLOResourceValidator) Description(ctx context.Context) string {
	return "Coralogix SLO resource validator."
}

func (S SLOResourceValidator) MarkdownDescription(ctx context.Context) string {
	return "Coralogix SLO resource validator."
}

func (S SLOResourceValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config *SLOResourceModel
	diags := req.Config.Get(ctx, &config)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
	if config.Type.ValueString() == "latency" && config.ThresholdMicroseconds.IsNull() {
		resp.Diagnostics.AddError(
			"ThresholdMicroseconds is required when type is latency",
			"ThresholdMicroseconds is required when type is latency",
		)
		return
	}
	if config.Type.ValueString() == "latency" && config.ThresholdSymbolType.IsNull() {
		resp.Diagnostics.AddError(
			"ThresholdSymbolType is required when type is latency",
			"ThresholdSymbolType is required when type is latency",
		)
		return
	}
	if config.Type.ValueString() == "error" && !config.ThresholdMicroseconds.IsNull() {
		resp.Diagnostics.AddError(
			"ThresholdMicroseconds is not allowed when type is error",
			"ThresholdMicroseconds is not allowed when type is error",
		)
		return
	}
	if config.Type.ValueString() == "error" && !config.ThresholdSymbolType.IsNull() {
		resp.Diagnostics.AddError(
			"ThresholdSymbolType is not allowed when type is error",
			"ThresholdSymbolType is not allowed when type is error",
		)
		return
	}
}

func (r *SLOResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_slo"

}

func (r *SLOResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.LegacySLOs()
}

func (r *SLOResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *SLOResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "SLO ID.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "SLO name.",
			},
			"service_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Service name. This is the name of the service that the SLO is associated with.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional SLO description.",
			},
			"status": schema.StringAttribute{
				Computed: true,
			},
			"target_percentage": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Target percentage. This is the target percentage of the SLO.",
				Validators: []validator.Int64{
					int64validator.Between(0, 100),
				},
			},
			"remaining_error_budget_percentage": schema.Int64Attribute{
				Computed: true,
			},
			"type": schema.StringAttribute{
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf("error", "latency")},
				MarkdownDescription: `Type. This is the type of the SLO. Valid values are: "error", "latency".`,
			},
			"threshold_microseconds": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Threshold in microseconds. Required when `type` is `latency`.",
			},
			"threshold_symbol_type": schema.StringAttribute{
				Optional:            true,
				Validators:          []validator.String{stringvalidator.OneOf(validThresholdSymbolTypes...)},
				MarkdownDescription: fmt.Sprintf("Threshold symbol type. Required when `type` is `latency`. Valid values are: %q", validThresholdSymbolTypes),
			},
			"filters": schema.SetNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"field": schema.StringAttribute{
							Required: true,
						},
						"compare_type": schema.StringAttribute{
							Required:            true,
							Validators:          []validator.String{stringvalidator.OneOf(validSLOCompareTypes...)},
							MarkdownDescription: fmt.Sprintf("Compare type. This is the compare type of the SLO. Valid values are: %q", validSLOCompareTypes),
						},
						"field_values": schema.SetAttribute{
							Optional:    true,
							ElementType: types.StringType,
						},
					},
				},
			},
			"period": schema.StringAttribute{
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf(validSLOPeriods...)},
				MarkdownDescription: fmt.Sprintf("Period. This is the period of the SLO. Valid values are: %q", validSLOPeriods),
			},
		},
		MarkdownDescription: "Coralogix SLO.",
		DeprecationMessage:  "This resource is deprecated in favor of coralogix_slo_v2.",
	}
}

type SLOResourceModel struct {
	ID                             types.String `tfsdk:"id"`
	Name                           types.String `tfsdk:"name"`
	ServiceName                    types.String `tfsdk:"service_name"`
	Description                    types.String `tfsdk:"description"`
	Status                         types.String `tfsdk:"status"`
	TargetPercentage               types.Int64  `tfsdk:"target_percentage"`
	RemainingErrorBudgetPercentage types.Int64  `tfsdk:"remaining_error_budget_percentage"`
	Type                           types.String `tfsdk:"type"`
	ThresholdMicroseconds          types.Int64  `tfsdk:"threshold_microseconds"`
	ThresholdSymbolType            types.String `tfsdk:"threshold_symbol_type"`
	Filters                        types.Set    `tfsdk:"filters"` //types.Object
	Period                         types.String `tfsdk:"period"`
}

type SLOFilterModel struct {
	Field       types.String `tfsdk:"field"`
	CompareType types.String `tfsdk:"compare_type"`
	FieldValues types.Set    `tfsdk:"field_values"` //types.String
}

func (r *SLOResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *SLOResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	slo, diags := extractSLO(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
	createSloReq := &cxsdk.CreateLegacySloRequest{Slo: slo}
	log.Printf("[INFO] Creating new SLO: %s", protojson.Format(createSloReq))
	createResp, err := r.client.Create(ctx, createSloReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error creating SLO",
			utils.FormatRpcErrors(err, cxsdk.LegacySloCreateRPC, protojson.Format(createSloReq)),
		)
		return
	}
	slo = createResp.GetSlo()
	log.Printf("[INFO] Submitted new SLO: %s", protojson.Format(slo))
	plan, diags = flattenSLO(ctx, slo)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func flattenSLO(ctx context.Context, slo *cxsdk.ServiceSlo) (*SLOResourceModel, diag.Diagnostics) {
	filters, diags := flattenSLOFilters(ctx, slo.GetFilters())
	if diags != nil {
		return nil, diags
	}
	flattenedSlo := &SLOResourceModel{
		ID:                             utils.WrapperspbStringToTypeString(slo.GetId()),
		Name:                           utils.WrapperspbStringToTypeString(slo.GetName()),
		ServiceName:                    utils.WrapperspbStringToTypeString(slo.GetServiceName()),
		Description:                    utils.WrapperspbStringToTypeString(slo.GetDescription()),
		Status:                         types.StringValue(protoToSchemaSLOStatus[slo.GetStatus()]),
		TargetPercentage:               utils.WrapperspbUint32ToTypeInt64(slo.GetTargetPercentage()),
		RemainingErrorBudgetPercentage: utils.WrapperspbUint32ToTypeInt64(slo.GetRemainingErrorBudgetPercentage()),
		Period:                         types.StringValue(protoToSchemaSLOPeriod[slo.GetPeriod()]),
		Filters:                        filters,
	}
	flattenedSlo, dg := flattenSLOType(flattenedSlo, slo)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}
	return flattenedSlo, nil
}

func flattenSLOFilters(ctx context.Context, filters []*cxsdk.SliFilter) (types.Set, diag.Diagnostics) {
	if len(filters) == 0 {
		return types.SetNull(types.ObjectType{AttrTypes: sloFilterModelAttr()}), nil
	}
	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0, len(filters))
	for _, filter := range filters {
		flattenedFilter := SLOFilterModel{
			Field:       utils.WrapperspbStringToTypeString(filter.GetField()),
			CompareType: types.StringValue(protoToSchemaSLOCompareType[filter.GetCompareType()]),
			FieldValues: utils.WrappedStringSliceToTypeStringSet(filter.GetFieldValues()),
		}
		filtersElement, diags := types.ObjectValueFrom(ctx, sloFilterModelAttr(), flattenedFilter)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		filtersElements = append(filtersElements, filtersElement)
	}
	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: sloFilterModelAttr()}, filtersElements)
}

func sloFilterModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field":        types.StringType,
		"compare_type": types.StringType,
		"field_values": types.SetType{
			ElemType: types.StringType,
		},
	}
}

func flattenSLOType(flattenedSlo *SLOResourceModel, slo *cxsdk.ServiceSlo) (*SLOResourceModel, diag.Diagnostic) {
	switch sliType := slo.SliType.(type) {
	case *cxsdk.ServiceSloErrorSli:
		flattenedSlo.Type = types.StringValue("error")
	case *cxsdk.ServiceSloLatencySli:
		flattenedSlo.Type = types.StringValue("latency")
		latency, err := strconv.Atoi(sliType.LatencySli.GetThresholdMicroseconds().GetValue())
		if err != nil {
			return nil, diag.NewErrorDiagnostic("Error converting latency threshold to int", err.Error())
		}
		flattenedSlo.ThresholdMicroseconds = types.Int64Value(int64(latency))
		flattenedSlo.ThresholdSymbolType = types.StringValue(protoToSchemaThresholdSymbolType[sliType.LatencySli.GetThresholdSymbol()])
	}
	return flattenedSlo, nil
}

func extractSLO(ctx context.Context, plan *SLOResourceModel) (*cxsdk.ServiceSlo, diag.Diagnostics) {
	filters, diags := extractSLOFilters(ctx, plan.Filters)
	if diags.HasError() {
		return nil, diags
	}
	slo := &cxsdk.ServiceSlo{
		Id:               utils.TypeStringToWrapperspbString(plan.ID),
		Name:             utils.TypeStringToWrapperspbString(plan.Name),
		ServiceName:      utils.TypeStringToWrapperspbString(plan.ServiceName),
		Description:      utils.TypeStringToWrapperspbString(plan.Description),
		TargetPercentage: utils.TypeInt64ToWrappedUint32(plan.TargetPercentage),
		Period:           schemaToProtoSLOPeriod[plan.Period.ValueString()],
		Filters:          filters,
	}
	slo = expandSLIType(slo, plan)

	return slo, nil
}

func expandSLIType(slo *cxsdk.ServiceSlo, plan *SLOResourceModel) *cxsdk.ServiceSlo {
	switch plan.Type.ValueString() {
	case "error":
		slo.SliType = &cxsdk.ServiceSloErrorSli{ErrorSli: &cxsdk.ErrorSli{}}
	case "latency":

		slo.SliType = &cxsdk.ServiceSloLatencySli{
			LatencySli: &cxsdk.LatencySli{
				ThresholdMicroseconds: wrapperspb.String(strconv.Itoa(int(plan.ThresholdMicroseconds.ValueInt64()))),
				ThresholdSymbol:       schemaToProtoThresholdSymbolType[plan.ThresholdSymbolType.ValueString()],
			},
		}
	}

	return slo
}

func extractSLOFilters(ctx context.Context, filters types.Set) ([]*cxsdk.SliFilter, diag.Diagnostics) {
	var diags diag.Diagnostics
	var filtersObjects []types.Object
	var expandedLabels []*cxsdk.SliFilter
	filters.ElementsAs(ctx, &filtersObjects, true)

	for _, fo := range filtersObjects {
		var label SLOFilterModel
		if dg := fo.As(ctx, &label, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		fieldValues, dgs := utils.TypeStringSliceToWrappedStringSlice(ctx, label.FieldValues.Elements())
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		expandedLabel := &cxsdk.SliFilter{
			Field:       utils.TypeStringToWrapperspbString(label.Field),
			CompareType: schemaToProtoSLOCompareType[label.CompareType.ValueString()],
			FieldValues: fieldValues,
		}
		expandedLabels = append(expandedLabels, expandedLabel)
	}

	return expandedLabels, diags
}

func (r *SLOResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *SLOResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed SLO value from Coralogix
	id := state.ID.ValueString()
	readSloReq := &cxsdk.GetLegacySloRequest{Id: wrapperspb.String(id)}
	readSloResp, err := r.client.Get(ctx, readSloReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("SLO %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading SLO",
				utils.FormatRpcErrors(err, cxsdk.LegacySloGetRPC, protojson.Format(readSloReq)),
			)
		}
		return
	}

	slo := readSloResp.GetSlo()
	log.Printf("[INFO] Received SLO: %s", protojson.Format(slo))
	state, diags = flattenSLO(ctx, slo)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}

	//
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *SLOResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan *SLOResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	slo, diags := extractSLO(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
	updateSloReq := &cxsdk.ReplaceLegacySloRequest{Slo: slo}
	log.Printf("[INFO] Updating SLO: %s", protojson.Format(updateSloReq))
	updateSloResp, err := r.client.Update(ctx, updateSloReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating SLO",
			utils.FormatRpcErrors(err, cxsdk.LegacySloReplaceRPC, protojson.Format(updateSloReq)),
		)
		return
	}
	log.Printf("[INFO] Submitted updated SLO: %s", updateSloResp)

	// Get refreshed SLO value from Coralogix
	id := plan.ID.ValueString()
	getSloReq := &cxsdk.GetLegacySloRequest{Id: wrapperspb.String(id)}
	getSloResp, err := r.client.Get(ctx, getSloReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("SLO %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading SLO",
				utils.FormatRpcErrors(err, cxsdk.LegacySloGetRPC, protojson.Format(getSloReq)),
			)
		}
		return
	}

	slo = getSloResp.GetSlo()
	log.Printf("[INFO] Received SLO: %s", protojson.Format(slo))
	state, diags := flattenSLO(ctx, slo)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *SLOResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state *SLOResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	log.Printf("[INFO] Deleting SLO %s\n", id)
	deleteReq := &cxsdk.DeleteLegacySloRequest{Id: wrapperspb.String(id)}
	if _, err := r.client.Delete(ctx, deleteReq); err != nil {
		reqStr := protojson.Format(deleteReq)
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting SLO %s", state.ID.ValueString()),
			utils.FormatRpcErrors(err, cxsdk.LegacySloDeleteRPC, reqStr),
		)
		return
	}
	log.Printf("[INFO] SLO %s deleted\n", id)
}
