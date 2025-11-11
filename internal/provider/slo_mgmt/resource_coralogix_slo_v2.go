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

package slo_mgmt

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	slos "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/slos_service"

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
)

var (
	_ resource.ResourceWithConfigure   = &SLOV2Resource{}
	_ resource.ResourceWithImportState = &SLOV2Resource{}

	protoToSchemaSloTimeFrame = map[slos.SloTimeFrame]string{
		slos.SLOTIMEFRAME_SLO_TIME_FRAME_UNSPECIFIED: utils.UNSPECIFIED,
		slos.SLOTIMEFRAME_SLO_TIME_FRAME_7_DAYS:      "7_days",
		slos.SLOTIMEFRAME_SLO_TIME_FRAME_14_DAYS:     "14_days",
		slos.SLOTIMEFRAME_SLO_TIME_FRAME_21_DAYS:     "21_days",
		slos.SLOTIMEFRAME_SLO_TIME_FRAME_28_DAYS:     "28_days",
	}
	schemaToProtoSLOTimeFrame = utils.ReverseMap(protoToSchemaSloTimeFrame)
	validSLOTimeFrame         = utils.GetKeys(schemaToProtoSLOTimeFrame)
	protoToSchemaSloWindow    = map[slos.WindowSloWindow]string{
		slos.WINDOWSLOWINDOW_WINDOW_SLO_WINDOW_UNSPECIFIED: utils.UNSPECIFIED,
		slos.WINDOWSLOWINDOW_WINDOW_SLO_WINDOW_1_MINUTE:    "1_minute",
		slos.WINDOWSLOWINDOW_WINDOW_SLO_WINDOW_5_MINUTES:   "5_minutes",
	}
	schemaToProtoSLOWindow          = utils.ReverseMap(protoToSchemaSloWindow)
	validWindows                    = utils.GetKeys(schemaToProtoSLOWindow)
	protoToSchemaComparisonOperator = map[slos.ComparisonOperator]string{
		slos.COMPARISONOPERATOR_COMPARISON_OPERATOR_UNSPECIFIED:            utils.UNSPECIFIED,
		slos.COMPARISONOPERATOR_COMPARISON_OPERATOR_GREATER_THAN:           "greater_than",
		slos.COMPARISONOPERATOR_COMPARISON_OPERATOR_LESS_THAN:              "less_than",
		slos.COMPARISONOPERATOR_COMPARISON_OPERATOR_GREATER_THAN_OR_EQUALS: "greater_than_or_equals",
		slos.COMPARISONOPERATOR_COMPARISON_OPERATOR_LESS_THAN_OR_EQUALS:    "less_than_or_equals",
	}
	schemaToProtoComparisonOperator = utils.ReverseMap(protoToSchemaComparisonOperator)
	validComparisonOperators        = utils.GetKeys(schemaToProtoComparisonOperator)
)

func NewSLOV2Resource() resource.Resource {
	return &SLOV2Resource{}
}

type SLOV2Resource struct {
	client *slos.SlosServiceAPIService
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
				MarkdownDescription: "Grouping configuration for SLO evaluations.",
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
		MarkdownDescription: "Coralogix New SLO. Read more about limits and details at https://coralogix.com/docs/user-guides/slos/introduction/",
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
	rq := slos.SlosServiceReplaceSloRequest{
		SloRequestBasedMetricSli: slo.SloRequestBasedMetricSli,
		SloWindowBasedMetricSli:  slo.SloWindowBasedMetricSli,
	}

	log.Printf("[INFO] Creating new coralogix_slo_v2: %s", utils.FormatJSON(rq))
	result, httpResponse, err := r.client.SlosServiceCreateSlo(ctx).SlosServiceReplaceSloRequest(rq).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error creating coralogix_slo_v2",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", rq),
		)
		return
	}
	log.Printf("[INFO] Created new coralogix_slo_v2: %s", utils.FormatJSON(result))
	plan, diags = flattenSLOV2(ctx, &result.Slo)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
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
	rq := r.client.SlosServiceGetSlo(ctx, id)
	log.Printf("[INFO] Reading new coralogix_slo_v2: %s", utils.FormatJSON(rq))
	result, httpResponse, err := rq.Execute()

	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_slo_v2 %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error reading coralogix_slo_v2",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
			)
		}
		return
	}
	log.Printf("[INFO] Read coralogix_slo_v2: %s", utils.FormatJSON(result))

	state, diags = flattenSLOV2(ctx, &result.Slo)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}

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
	// Get refreshed SLO value from Coralogix
	id := plan.ID.ValueString()

	slo, diags := extractSLOV2(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}

	rq := slos.SlosServiceReplaceSloRequest{
		SloRequestBasedMetricSli: slo.SloRequestBasedMetricSli,
		SloWindowBasedMetricSli:  slo.SloWindowBasedMetricSli,
	}
	log.Printf("[INFO] Updating coralogix_slo_v2: %s", utils.FormatJSON(rq))

	result, httpResponse, err := r.client.
		SlosServiceReplaceSlo(ctx).
		SlosServiceReplaceSloRequest(rq).
		Execute()

	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_slo_v2 %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error replacing coralogix_slo_v2", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Replace", rq))
		}
		return
	}
	log.Printf("[INFO] Updated coralogix_slo_v2: %s", utils.FormatJSON(result))

	plan, diags = flattenSLOV2(ctx, &result.Slo)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}

	diags = resp.State.Set(ctx, plan)
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

	log.Printf("[INFO] Deleting coralogix_slo_v2")

	result, httpResponse, err := r.client.
		SlosServiceDeleteSlo(ctx, id).
		Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error deleting coralogix_slo_v2",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Delete", nil),
		)
		return
	}
	log.Printf("[INFO] Deleted coralogix_slo_v2: %s", utils.FormatJSON(result))
}

func extractSLOV2(ctx context.Context, plan *SLOV2ResourceModel) (*slos.Slo, diag.Diagnostics) {
	slo := &slos.Slo{}
	name := plan.Name.ValueStringPointer()
	description := plan.Description.ValueStringPointer()
	targetThresholdPct := plan.TargetThresholdPercentage.ValueFloat32()
	var id *string

	if !plan.ID.IsNull() && plan.ID.ValueString() != "" {
		id = plan.ID.ValueStringPointer()
	}

	labels, diags := utils.TypeMapToStringMap(ctx, plan.Labels)
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := extractWindow(ctx, plan.Window)
	if diags.HasError() {
		return nil, diags
	}

	var sliModel SLIModel
	if diags := plan.SLI.As(ctx, &sliModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if reqBased := sliModel.RequestBasedMetricSli; !(reqBased.IsNull() || reqBased.IsUnknown()) {

		sli, diags := extractRequestBasedSLI(ctx, id, &labels, name, description, targetThresholdPct, timeFrame, reqBased)
		if diags.HasError() {
			return nil, diags
		}
		slo.SloRequestBasedMetricSli = sli
	} else if winBased := sliModel.WindowBasedMetricSli; !(winBased.IsNull() || winBased.IsUnknown()) {
		sli, diags := extractWindowBasedSLI(ctx, id, &labels, name, description, targetThresholdPct, timeFrame, winBased)
		if diags.HasError() {
			return nil, diags
		}
		slo.SloWindowBasedMetricSli = sli
	} else {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic(
			"Invalid SLI configuration",
			"Exactly one of request_based_metric_sli or window_based_metric_sli must be provided.",
		)}
	}

	return slo, nil
}

func extractRequestBasedSLI(ctx context.Context, id *string, labels *map[string]string, name *string, description *string, targetThresholdPct float32, timeFrame *slos.SloTimeFrame, reqBased types.Object) (*slos.SloRequestBasedMetricSli, diag.Diagnostics) {
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

	return &slos.SloRequestBasedMetricSli{
		RequestBasedMetricSli: &slos.RequestBasedMetricSli{
			GoodEvents: slos.Metric{
				Query: goodModel.Query.ValueString(),
			},
			TotalEvents: slos.Metric{
				Query: totalModel.Query.ValueString(),
			},
		},
		Description:               description,
		Id:                        id,
		Labels:                    labels,
		Name:                      *name,
		SloTimeFrame:              timeFrame,
		TargetThresholdPercentage: targetThresholdPct,
	}, nil
}

func extractWindowBasedSLI(ctx context.Context, id *string, labels *map[string]string, name *string, description *string, targetThresholdPct float32, timeFrame *slos.SloTimeFrame, winBased types.Object) (*slos.SloWindowBasedMetricSli, diag.Diagnostics) {
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

	return &slos.SloWindowBasedMetricSli{
		WindowBasedMetricSli: &slos.WindowBasedMetricSli{
			Query: slos.Metric{
				Query: queryModel.Query.ValueString(),
			},
			Window:             schemaToProtoSLOWindow[windowBasedModel.Window.ValueString()],
			ComparisonOperator: schemaToProtoComparisonOperator[windowBasedModel.ComparisonOperator.ValueString()],
			Threshold:          windowBasedModel.Threshold.ValueFloat32(),
		},
		Description:               description,
		Id:                        id,
		Labels:                    labels,
		Name:                      *name,
		SloTimeFrame:              timeFrame,
		TargetThresholdPercentage: targetThresholdPct,
	}, nil
}

func extractWindow(ctx context.Context, rule types.Object) (*slos.SloTimeFrame, diag.Diagnostics) {
	if rule.IsNull() || rule.IsUnknown() {
		return nil, nil
	}

	windowModel := &WindowModel{}
	diags := rule.As(ctx, windowModel, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}
	tf := schemaToProtoSLOTimeFrame[windowModel.SloTimeFrame.ValueString()]
	return &tf, nil
}

func flattenSLOV2(ctx context.Context, slo *slos.Slo) (*SLOV2ResourceModel, diag.Diagnostics) {

	if rb := slo.SloRequestBasedMetricSli; rb != nil {
		return flattenRequestBasedSLI(ctx, rb)
	} else if wb := slo.SloWindowBasedMetricSli; wb != nil {
		return flattenWindowBasedSLI(ctx, wb)
	} else {
		diags := diag.Diagnostics{}
		log.Printf("[ERROR] Response was neither a request nor window based SLO; %s", utils.FormatJSON(slo))
		diags.AddError("Invalid response from server", utils.FormatJSON(slo))
		return nil, diags
	}
}

func flattenGrouping(ctx context.Context, grouping *slos.V1Grouping) (types.Object, diag.Diagnostics) {
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

func flattenWindow(ctx context.Context, tf slos.SloTimeFrame) (types.Object, diag.Diagnostics) {
	value := protoToSchemaSloTimeFrame[tf]
	model := WindowModel{
		SloTimeFrame: types.StringValue(value),
	}
	return types.ObjectValueFrom(ctx, map[string]attr.Type{
		"slo_time_frame": types.StringType,
	}, model)
}

func flattenRequestBasedSLI(ctx context.Context, sli *slos.SloRequestBasedMetricSli) (*SLOV2ResourceModel, diag.Diagnostics) {
	sliObj := types.ObjectNull(sliAttr())

	goodEvents := SLOMetricQueryModel{
		Query: types.StringValue(sli.RequestBasedMetricSli.GoodEvents.Query),
	}

	totalEvents := SLOMetricQueryModel{
		Query: types.StringValue(sli.RequestBasedMetricSli.TotalEvents.Query),
	}

	goodObj, diags := types.ObjectValueFrom(ctx, sloMetricQueryAttr(), goodEvents)
	if diags.HasError() {
		return nil, diags
	}

	totalObj, diags := types.ObjectValueFrom(ctx, sloMetricQueryAttr(), totalEvents)
	if diags.HasError() {
		return nil, diags
	}

	requestSliModel := RequestBasedMetricSliModel{
		GoodEvents:  goodObj,
		TotalEvents: totalObj,
	}

	reqSliObj, diags := types.ObjectValueFrom(ctx, requestBasedMetricSliAttr(), requestSliModel)
	if diags.HasError() {
		return nil, diags
	}

	sliObj, diags = types.ObjectValueFrom(ctx, sliAttr(), SLIModel{
		RequestBasedMetricSli: reqSliObj,
		WindowBasedMetricSli:  types.ObjectNull(windowBasedMetricSliAttr()),
	})
	if diags.HasError() {
		return nil, diags
	}

	labels, diags := utils.StringMapToTypeMap(ctx, sli.Labels)
	if diags.HasError() {
		return nil, diags
	}

	grouping, diags := flattenGrouping(ctx, sli.Grouping)
	if diags.HasError() {
		return nil, diags
	}

	window, diags := flattenWindow(ctx, sli.GetSloTimeFrame())
	if diags.HasError() {
		return nil, diags
	}

	return &SLOV2ResourceModel{
		ID:                        types.StringPointerValue(sli.Id),
		Name:                      types.StringValue(sli.Name),
		Description:               types.StringPointerValue(sli.Description),
		Labels:                    labels,
		Grouping:                  grouping,
		TargetThresholdPercentage: types.Float32Value(sli.TargetThresholdPercentage),
		SLI:                       sliObj,
		Window:                    window,
	}, diags
}

func flattenWindowBasedSLI(ctx context.Context, sli *slos.SloWindowBasedMetricSli) (*SLOV2ResourceModel, diag.Diagnostics) {
	queryModel := SLOMetricQueryModel{
		Query: types.StringValue(sli.WindowBasedMetricSli.Query.Query),
	}
	queryObj, diags := types.ObjectValueFrom(ctx, sloMetricQueryAttr(), queryModel)
	if diags.HasError() {
		return nil, diags
	}

	model := WindowBasedMetricSliModel{
		Query:              queryObj,
		Window:             types.StringValue(protoToSchemaSloWindow[sli.WindowBasedMetricSli.Window]),
		ComparisonOperator: types.StringValue(protoToSchemaComparisonOperator[sli.WindowBasedMetricSli.ComparisonOperator]),
		Threshold:          types.Float32Value(sli.WindowBasedMetricSli.Threshold),
	}
	winObj, diags := types.ObjectValueFrom(ctx, windowBasedMetricSliAttr(), model)
	if diags.HasError() {
		return nil, diags
	}

	sliObj, diags := types.ObjectValueFrom(ctx, sliAttr(), SLIModel{
		RequestBasedMetricSli: types.ObjectNull(requestBasedMetricSliAttr()),
		WindowBasedMetricSli:  winObj,
	})
	if diags.HasError() {
		return nil, diags
	}

	grouping, diags := flattenGrouping(ctx, sli.Grouping)
	if diags.HasError() {
		return nil, diags
	}

	window, diags := flattenWindow(ctx, sli.GetSloTimeFrame())
	if diags.HasError() {
		return nil, diags
	}

	labels, diags := utils.StringMapToTypeMap(ctx, sli.Labels)
	if diags.HasError() {
		return nil, diags
	}

	return &SLOV2ResourceModel{
		ID:                        types.StringPointerValue(sli.Id),
		Name:                      types.StringValue(sli.Name),
		Description:               types.StringPointerValue(sli.Description),
		Grouping:                  grouping,
		TargetThresholdPercentage: types.Float32Value(sli.TargetThresholdPercentage),
		SLI:                       sliObj,
		Window:                    window,
		Labels:                    labels,
	}, diags
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
