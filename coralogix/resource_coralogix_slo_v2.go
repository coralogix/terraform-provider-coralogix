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

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"

	"github.com/hashicorp/terraform-plugin-framework-validators/float32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/encoding/protojson"
)

var (
	_ resource.ResourceWithConfigure   = &SLOV2Resource{}
	_ resource.ResourceWithImportState = &SLOV2Resource{}

	protoToSchemaSloTimeFrame = map[cxsdk.SloTimeframeEnum]string{
		cxsdk.SloTimeframeUnspecified: "unspecified",
		cxsdk.SloTimeframe7Days:       "7_days",
		cxsdk.SloTimeframe14Days:      "14_days",
		cxsdk.SloTimeframe21Days:      "21_days",
		cxsdk.SloTimeframe28Days:      "28_days",
	}
	schemaToProtoSLOTimeFrame = utils.ReverseMap(protoToSchemaSloTimeFrame)
	validSLOTimeFrame         = utils.GetKeys(schemaToProtoSLOTimeFrame)
	protoToSchemaSloWindow    = map[cxsdk.SloWindow]string{
		cxsdk.SloWindowUnspecified: "unspecified",
		cxsdk.SloWindow1Minute:     "1_minute",
		cxsdk.SloWindow5Minutes:    "5_minutes",
	}
	schemaToProtoSLOWindow          = utils.ReverseMap(protoToSchemaSloWindow)
	validWindows                    = utils.GetKeys(schemaToProtoSLOWindow)
	protoToSchemaComparisonOperator = map[cxsdk.SloComparisonOperator]string{
		cxsdk.SloComparisonOperatorUnspecified:         "unspecified",
		cxsdk.SloComparisonOperatorGreaterThan:         "greater_than",
		cxsdk.SloComparisonOperatorLessThan:            "less_than",
		cxsdk.SloComparisonOperatorGreaterThanOrEquals: "greater_than_or_equals",
		cxsdk.SloComparisonOperatorLessThanOrEquals:    "less_than_or_equals",
	}
	schemaToProtoComparisonOperator = utils.ReverseMap(protoToSchemaComparisonOperator)
	validComparisonOperators        = utils.GetKeys(schemaToProtoComparisonOperator)
)

func NewSLOV2Resource() resource.Resource {
	return &SLOV2Resource{}
}

type SLOV2Resource struct {
	client *cxsdk.SLOsClient
}

func (r *SLOV2Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_slo_v2"

}

func (r *SLOV2Resource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.SLOs()
}

func (r *SLOV2Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *SLOV2Resource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
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
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional SLO description.",
			},
			"labels": schema.MapAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "Optional map of labels to attach to the SLO. ",
			},
			"grouping": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"labels": schema.ListAttribute{
						ElementType:         types.StringType,
						Computed:            true,
						MarkdownDescription: "List of labels to group SLO evaluations by.",
					},
				},
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Optional grouping configuration for SLO evaluations.",
			},
			"target_threshold_percentage": schema.Float32Attribute{
				Required:            true,
				MarkdownDescription: "The target threshold percentage.",
				Validators: []validator.Float32{
					float32validator.Between(0, 100),
				},
			},
			"sli": schema.SingleNestedAttribute{
				Required:            true,
				MarkdownDescription: "SLI definition: exactly one of request_based_metric_sli or window_based_metric_sli must be provided.",
				Attributes: map[string]schema.Attribute{
					"request_based_metric_sli": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "SLI based on request metrics.",
						Attributes: map[string]schema.Attribute{
							"good_events": schema.SingleNestedAttribute{
								Required:            true,
								MarkdownDescription: "Query defining good events.",
								Attributes: map[string]schema.Attribute{
									"query": schema.StringAttribute{
										Required:            true,
										MarkdownDescription: "Query string for good events.",
									},
								},
							},
							"total_events": schema.SingleNestedAttribute{
								Required:            true,
								MarkdownDescription: "Query defining total events.",
								Attributes: map[string]schema.Attribute{
									"query": schema.StringAttribute{
										Required:            true,
										MarkdownDescription: "Query string for total events.",
									},
								},
							},
						},
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("window_based_metric_sli")),
						},
					},
					"window_based_metric_sli": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "SLI based on time-window metrics.",
						Attributes: map[string]schema.Attribute{
							"query": schema.SingleNestedAttribute{
								Required:            true,
								MarkdownDescription: "Query used for evaluating the time-window SLI.",
								Attributes: map[string]schema.Attribute{
									"query": schema.StringAttribute{
										Required:            true,
										MarkdownDescription: "Query string for the metric.",
									},
								},
							},
							"window": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: fmt.Sprintf("Time window type for evaluation. One of: %v.", strings.Join(validWindows, ", ")),
								Validators:          []validator.String{stringvalidator.OneOf(validWindows...)},
							},
							"comparison_operator": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: fmt.Sprintf("Comparison operator used to evaluate the threshold. One of: %v", strings.Join(validComparisonOperators, ",")),
								Validators:          []validator.String{stringvalidator.OneOf(validComparisonOperators...)},
							},
							"threshold": schema.Float32Attribute{
								Required:            true,
								MarkdownDescription: "Threshold value for the comparison.",
							},
						},
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("request_based_metric_sli")),
						},
					},
				},
			},
			"window": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"slo_time_frame": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: fmt.Sprintf("SLO time window. One of: %v.", strings.Join(validSLOTimeFrame, ", ")),
						Validators:          []validator.String{stringvalidator.OneOf(validSLOTimeFrame...)},
					},
				},
				MarkdownDescription: fmt.Sprintf("SLO time window. One of: %v.", strings.Join(validSLOTimeFrame, ", ")),
			},
		},
		MarkdownDescription: "Coralogix New SLO.",
	}
}

type SLOV2ResourceModel struct {
	ID                        types.String  `tfsdk:"id"`
	Name                      types.String  `tfsdk:"name"`
	Description               types.String  `tfsdk:"description"`
	Labels                    types.Map     `tfsdk:"labels"`
	Grouping                  types.Object  `tfsdk:"grouping"`
	TargetThresholdPercentage types.Float32 `tfsdk:"target_threshold_percentage"`
	SLI                       types.Object  `tfsdk:"sli"`
	Window                    types.Object  `tfsdk:"window"`
}

type GroupingModel struct {
	Labels types.List `tfsdk:"labels"`
}

type SLIModel struct {
	RequestBasedMetricSli types.Object `tfsdk:"request_based_metric_sli"`
	WindowBasedMetricSli  types.Object `tfsdk:"window_based_metric_sli"`
}

type RequestBasedMetricSliModel struct {
	GoodEvents  types.Object `tfsdk:"good_events"`
	TotalEvents types.Object `tfsdk:"total_events"`
}

type WindowBasedMetricSliModel struct {
	Query              types.Object  `tfsdk:"query"`
	Window             types.String  `tfsdk:"window"`
	ComparisonOperator types.String  `tfsdk:"comparison_operator"`
	Threshold          types.Float32 `tfsdk:"threshold"`
}

type SLOMetricQueryModel struct {
	Query types.String `tfsdk:"query"`
}

type WindowModel struct {
	SloTimeFrame types.String `tfsdk:"slo_time_frame"`
}

func (r *SLOV2Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *SLOV2ResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	slo, diags := extractSLOV2(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
	silenceValidations := true
	createSloReq := &cxsdk.CreateServiceSloRequest{Slo: slo, SilenceDataValidations: &silenceValidations}
	log.Printf("[INFO] Creating new SLO: %s", protojson.Format(createSloReq))
	createResp, err := r.client.Create(ctx, createSloReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error creating SLO",
			utils.FormatRpcErrors(err, cxsdk.SloCreateRPC, protojson.Format(createSloReq)),
		)
		return
	}
	slo = createResp.GetSlo()
	log.Printf("[INFO] Submitted new SLO: %s", protojson.Format(slo))
	plan, diags = flattenSLOV2(ctx, slo)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func extractSLOV2(ctx context.Context, plan *SLOV2ResourceModel) (*cxsdk.Slo, diag.Diagnostics) {
	slo := &cxsdk.Slo{
		Name:                      plan.Name.ValueString(),
		Description:               plan.Description.ValueStringPointer(),
		TargetThresholdPercentage: plan.TargetThresholdPercentage.ValueFloat32(),
	}

	if !plan.ID.IsNull() && plan.ID.ValueString() != "" {
		slo.Id = plan.ID.ValueStringPointer()
	}

	labels, diags := utils.TypeMapToStringMap(ctx, plan.Labels)
	if diags.HasError() {
		return nil, diags
	}
	slo.Labels = labels

	window, diags := extractWindow(ctx, plan.Window)
	if diags.HasError() {
		return nil, diags
	}
	slo.Window = window

	var sliModel SLIModel
	if diags := plan.SLI.As(ctx, &sliModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if reqBased := sliModel.RequestBasedMetricSli; !(reqBased.IsNull() || reqBased.IsUnknown()) {
		sli, diags := extractRequestBasedSLI(ctx, reqBased)
		if diags.HasError() {
			return nil, diags
		}
		slo.Sli = sli
	} else if winBased := sliModel.WindowBasedMetricSli; !(winBased.IsNull() || winBased.IsUnknown()) {
		sli, diags := extractWindowBasedSLI(ctx, winBased)
		if diags.HasError() {
			return nil, diags
		}
		slo.Sli = sli
	} else {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic(
			"Invalid SLI configuration",
			"Exactly one of request_based_metric_sli or window_based_metric_sli must be provided.",
		)}
	}

	return slo, nil
}

func extractRequestBasedSLI(ctx context.Context, reqBased types.Object) (*cxsdk.SloRequestBasedMetricSli, diag.Diagnostics) {
	var requestBasedModel RequestBasedMetricSliModel
	diags := reqBased.As(ctx, &requestBasedModel, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	var goodModel SLOMetricQueryModel
	diags = requestBasedModel.GoodEvents.As(ctx, &goodModel, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	var totalModel SLOMetricQueryModel
	diags = requestBasedModel.TotalEvents.As(ctx, &totalModel, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.SloRequestBasedMetricSli{
		RequestBasedMetricSli: &cxsdk.RequestBasedMetricSli{
			GoodEvents: &cxsdk.Metric{
				Query: goodModel.Query.ValueString(),
			},
			TotalEvents: &cxsdk.Metric{
				Query: totalModel.Query.ValueString(),
			},
		},
	}, nil
}

func extractWindowBasedSLI(ctx context.Context, winBased types.Object) (*cxsdk.SloWindowBasedMetricSli, diag.Diagnostics) {
	var windowBasedModel WindowBasedMetricSliModel
	diags := winBased.As(ctx, &windowBasedModel, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	var queryModel SLOMetricQueryModel
	diags = windowBasedModel.Query.As(ctx, &queryModel, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.SloWindowBasedMetricSli{
		WindowBasedMetricSli: &cxsdk.WindowBasedMetricSli{
			Query: &cxsdk.Metric{
				Query: queryModel.Query.ValueString(),
			},
			Window:             schemaToProtoSLOWindow[windowBasedModel.Window.ValueString()],
			ComparisonOperator: schemaToProtoComparisonOperator[windowBasedModel.ComparisonOperator.ValueString()],
			Threshold:          windowBasedModel.Threshold.ValueFloat32(),
		},
	}, nil
}

func extractWindow(ctx context.Context, rule types.Object) (*cxsdk.SloTimeframe, diag.Diagnostics) {
	if rule.IsNull() || rule.IsUnknown() {
		return nil, nil
	}

	windowModel := &WindowModel{}
	diags := rule.As(ctx, windowModel, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.SloTimeframe{
		SloTimeFrame: schemaToProtoSLOTimeFrame[windowModel.SloTimeFrame.ValueString()],
	}, nil
}

func flattenSLOV2(ctx context.Context, slo *cxsdk.Slo) (*SLOV2ResourceModel, diag.Diagnostics) {
	labels, diags := flattenLabels(ctx, slo.GetLabels())
	if diags.HasError() {
		return nil, diags
	}

	grouping, diags := flattenGrouping(ctx, slo.Grouping)
	if diags.HasError() {
		return nil, diags
	}

	window, diags := flattenWindow(ctx, slo.GetSloTimeFrame())
	if diags.HasError() {
		return nil, diags
	}

	sli, diags := flattenSLI(ctx, slo)
	if diags.HasError() {
		return nil, diags
	}

	model := &SLOV2ResourceModel{
		ID:                        utils.StringPointerToTypeString(slo.Id),
		Name:                      types.StringValue(slo.GetName()),
		Description:               types.StringValue(slo.GetDescription()),
		Labels:                    labels,
		Grouping:                  grouping,
		TargetThresholdPercentage: types.Float32Value(slo.GetTargetThresholdPercentage()),
		SLI:                       sli,
		Window:                    window,
	}

	return model, diags
}

func flattenLabels(ctx context.Context, labels map[string]string) (types.Map, diag.Diagnostics) {
	var diags diag.Diagnostics
	if labels == nil {
		return types.MapNull(types.StringType), diags
	}

	detailsMap := make(map[string]types.String)
	for k, v := range labels {
		detailsMap[k] = types.StringValue(v)
	}

	return types.MapValueFrom(ctx, types.StringType, detailsMap)
}

func flattenGrouping(ctx context.Context, grouping *cxsdk.SloGrouping) (types.Object, diag.Diagnostics) {
	if grouping == nil {
		return types.ObjectNull(map[string]attr.Type{"labels": types.ListType{ElemType: types.StringType}}), nil
	}

	labels, diags := types.ListValueFrom(ctx, types.StringType, grouping.GetLabels())
	if diags.HasError() {
		return types.ObjectNull(map[string]attr.Type{"labels": types.ListType{ElemType: types.StringType}}), diags
	}

	groupingModel := GroupingModel{
		Labels: labels,
	}
	return types.ObjectValueFrom(ctx, map[string]attr.Type{
		"labels": types.ListType{ElemType: types.StringType},
	}, groupingModel)
}

func flattenWindow(ctx context.Context, tf cxsdk.SloTimeframeEnum) (types.Object, diag.Diagnostics) {
	value := protoToSchemaSloTimeFrame[tf]
	model := WindowModel{
		SloTimeFrame: types.StringValue(value),
	}
	return types.ObjectValueFrom(ctx, map[string]attr.Type{
		"slo_time_frame": types.StringType,
	}, model)
}

func flattenSLI(ctx context.Context, slo *cxsdk.Slo) (types.Object, diag.Diagnostics) {
	if rb := slo.GetRequestBasedMetricSli(); rb != nil {
		return flattenRequestBasedSLI(ctx, rb)
	} else if wb := slo.GetWindowBasedMetricSli(); wb != nil {
		return flattenWindowBasedSLI(ctx, wb)
	}
	return types.ObjectNull(map[string]attr.Type{
		"request_based_metric_sli": types.ObjectType{AttrTypes: requestBasedMetricSliAttr()},
		"window_based_metric_sli":  types.ObjectType{AttrTypes: windowBasedMetricSliAttr()},
	}), nil
}

func flattenRequestBasedSLI(ctx context.Context, sli *cxsdk.RequestBasedMetricSli) (types.Object, diag.Diagnostics) {
	goodEvents := SLOMetricQueryModel{
		Query: types.StringValue(sli.GetGoodEvents().GetQuery()),
	}
	totalEvents := SLOMetricQueryModel{
		Query: types.StringValue(sli.GetTotalEvents().GetQuery()),
	}

	goodObj, diags := types.ObjectValueFrom(ctx, sloMetricQueryAttr(), goodEvents)
	if diags.HasError() {
		return types.ObjectNull(sliAttr()), diags
	}
	totalObj, diags := types.ObjectValueFrom(ctx, sloMetricQueryAttr(), totalEvents)
	if diags.HasError() {
		return types.ObjectNull(sliAttr()), diags
	}

	requestSliModel := RequestBasedMetricSliModel{
		GoodEvents:  goodObj,
		TotalEvents: totalObj,
	}
	reqSliObj, diags := types.ObjectValueFrom(ctx, requestBasedMetricSliAttr(), requestSliModel)
	if diags.HasError() {
		return types.ObjectNull(sliAttr()), diags
	}

	return types.ObjectValueFrom(ctx, sliAttr(), SLIModel{
		RequestBasedMetricSli: reqSliObj,
		WindowBasedMetricSli:  types.ObjectNull(windowBasedMetricSliAttr()),
	})
}

func flattenWindowBasedSLI(ctx context.Context, sli *cxsdk.WindowBasedMetricSli) (types.Object, diag.Diagnostics) {
	queryModel := SLOMetricQueryModel{
		Query: types.StringValue(sli.GetQuery().GetQuery()),
	}
	queryObj, diags := types.ObjectValueFrom(ctx, sloMetricQueryAttr(), queryModel)
	if diags.HasError() {
		return types.ObjectNull(sliAttr()), diags
	}

	model := WindowBasedMetricSliModel{
		Query:              queryObj,
		Window:             types.StringValue(protoToSchemaSloWindow[sli.GetWindow()]),
		ComparisonOperator: types.StringValue(protoToSchemaComparisonOperator[sli.GetComparisonOperator()]),
		Threshold:          types.Float32Value(sli.GetThreshold()),
	}
	winObj, diags := types.ObjectValueFrom(ctx, windowBasedMetricSliAttr(), model)
	if diags.HasError() {
		return types.ObjectNull(sliAttr()), diags
	}

	return types.ObjectValueFrom(ctx, sliAttr(), SLIModel{
		RequestBasedMetricSli: types.ObjectNull(requestBasedMetricSliAttr()),
		WindowBasedMetricSli:  winObj,
	})
}

// ---------------------- Attribute Maps ----------------------

func sloMetricQueryAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"query": types.StringType,
	}
}

func requestBasedMetricSliAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"good_events":  types.ObjectType{AttrTypes: sloMetricQueryAttr()},
		"total_events": types.ObjectType{AttrTypes: sloMetricQueryAttr()},
	}
}

func windowBasedMetricSliAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"query":               types.ObjectType{AttrTypes: sloMetricQueryAttr()},
		"window":              types.StringType,
		"comparison_operator": types.StringType,
		"threshold":           types.Float32Type,
	}
}

func sliAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"request_based_metric_sli": types.ObjectType{AttrTypes: requestBasedMetricSliAttr()},
		"window_based_metric_sli":  types.ObjectType{AttrTypes: windowBasedMetricSliAttr()},
	}
}

func (r *SLOV2Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *SLOV2ResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed SLO value from Coralogix
	id := state.ID.ValueString()
	readSloReq := &cxsdk.GetServiceSloRequest{Id: id}
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
				utils.FormatRpcErrors(err, cxsdk.SloGetRPC, protojson.Format(readSloReq)),
			)
		}
		return
	}

	slo := readSloResp.GetSlo()
	log.Printf("[INFO] Received SLO: %s", protojson.Format(slo))
	state, diags = flattenSLOV2(ctx, slo)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}

	//
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *SLOV2Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan *SLOV2ResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	slo, diags := extractSLOV2(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
	silenceValidations := true
	updateSloReq := &cxsdk.ReplaceServiceSloRequest{Slo: slo, SilenceDataValidations: &silenceValidations}
	log.Printf("[INFO] Updating SLO: %s", protojson.Format(updateSloReq))
	updateSloResp, err := r.client.Update(ctx, updateSloReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating SLO",
			utils.FormatRpcErrors(err, cxsdk.SloReplaceRPC, protojson.Format(updateSloReq)),
		)
		return
	}
	log.Printf("[INFO] Submitted updated SLO: %s", updateSloResp)

	// Get refreshed SLO value from Coralogix
	id := plan.ID.ValueString()
	getSloReq := &cxsdk.GetServiceSloRequest{Id: id}
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
				utils.FormatRpcErrors(err, cxsdk.SloGetRPC, protojson.Format(getSloReq)),
			)
		}
		return
	}

	slo = getSloResp.GetSlo()
	log.Printf("[INFO] Received SLO: %s", protojson.Format(slo))
	state, diags := flattenSLOV2(ctx, slo)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *SLOV2Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state *SLOV2ResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	log.Printf("[INFO] Deleting SLO %s\n", id)
	deleteReq := &cxsdk.DeleteServiceSloRequest{Id: id}
	if _, err := r.client.Delete(ctx, deleteReq); err != nil {
		reqStr := protojson.Format(deleteReq)
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting SLO %s", state.ID.ValueString()),
			utils.FormatRpcErrors(err, cxsdk.SloDeleteRPC, reqStr),
		)
		return
	}
	log.Printf("[INFO] SLO %s deleted\n", id)
}
