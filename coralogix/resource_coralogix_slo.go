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

	"terraform-provider-coralogix/coralogix/clientset"
	"terraform-provider-coralogix/coralogix/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"

	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
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
	_ resource.ResourceWithConfigure   = &SLOResource{}
	_ resource.ResourceWithImportState = &SLOResource{}
	//_                         resource.ResourceWithConfigValidators = &SLOResource{}

	protoToSchemaSloTimeFrame = map[cxsdk.SloTimeframeEnum]string{
		cxsdk.SloTimeframeUnspecified: "unspecified",
		cxsdk.SloTimeframe7Days:       "7_days",
		cxsdk.SloTimeframe14Days:      "14_days",
		cxsdk.SloTimeframe21Days:      "21_days",
		cxsdk.SloTimeframe28Days:      "28_days",
		cxsdk.SloTimeframe90Days:      "90_days",
	}
	schemaToProtoSLOTimeFrame = utils.ReverseMap(protoToSchemaSloTimeFrame)
	validSLOTimeFrame         = utils.GetKeys(schemaToProtoSLOTimeFrame)
)

func NewSLOResource() resource.Resource {
	return &SLOResource{}
}

type SLOResource struct {
	client *cxsdk.SLOsClient
}

//func (r *SLOResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
//	return []resource.ConfigValidator{
//		SLOResourceValidator{},
//	}
//}
//
//type SLOResourceValidator struct {
//}
//
//func (S SLOResourceValidator) Description(ctx context.Context) string {
//	return "Coralogix SLO resource validator."
//}
//
//func (S SLOResourceValidator) MarkdownDescription(ctx context.Context) string {
//	return "Coralogix SLO resource validator."
//}
//
//func (S SLOResourceValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
//	var config *SLOResourceModel
//	diags := req.Config.Get(ctx, &config)
//	if diags.HasError() {
//		resp.Diagnostics = diags
//		return
//	}
//	if config.Type.ValueString() == "latency" && config.ThresholdMicroseconds.IsNull() {
//		resp.Diagnostics.AddError(
//			"ThresholdMicroseconds is required when type is latency",
//			"ThresholdMicroseconds is required when type is latency",
//		)
//		return
//	}
//	if config.Type.ValueString() == "latency" && config.ThresholdSymbolType.IsNull() {
//		resp.Diagnostics.AddError(
//			"ThresholdSymbolType is required when type is latency",
//			"ThresholdSymbolType is required when type is latency",
//		)
//		return
//	}
//	if config.Type.ValueString() == "error" && !config.ThresholdMicroseconds.IsNull() {
//		resp.Diagnostics.AddError(
//			"ThresholdMicroseconds is not allowed when type is error",
//			"ThresholdMicroseconds is not allowed when type is error",
//		)
//		return
//	}
//	if config.Type.ValueString() == "error" && !config.ThresholdSymbolType.IsNull() {
//		resp.Diagnostics.AddError(
//			"ThresholdSymbolType is not allowed when type is error",
//			"ThresholdSymbolType is not allowed when type is error",
//		)
//		return
//	}
//}

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

	r.client = clientSet.SLOs()
}

func (r *SLOResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func sloResourceSchemaV1() schema.Schema {
	return schema.Schema{
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
			//"creator": schema.StringAttribute{
			//	Required:            true,
			//	MarkdownDescription: "Creator. This is the name of the user that created the SLO.",
			//},
			"labels": schema.MapAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "Optional map of labels to attach to the SLO. ",
			},
			"grouping": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "Optional list of labels to group SLO evaluations by.",
			},
			"target_threshold_percentage": schema.Float64Attribute{
				Required:            true,
				MarkdownDescription: "The target threshold percentage.",
				Validators: []validator.Float64{
					float64validator.Between(0, 100),
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
								MarkdownDescription: "Time window type for evaluation. One of: `1_minute`, `5_minutes`.",
							},
							"comparison_operator": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "Comparison operator used to evaluate the threshold. One of: `greater_than`, `less_than`, `greater_than_or_equals`, `less_than_or_equals`.",
							},
							"threshold": schema.Float64Attribute{
								Required:            true,
								MarkdownDescription: "Threshold value for the comparison.",
							},
						},
					},
				},

				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(
						path.MatchRoot("request_based_metric_sli"),
						path.MatchRoot("window_based_metric_sli"),
					),
				},
			},
			"window": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"slo_time_frame": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Time window for the SLO. One of: 7_days, 14_days, 21_days, 28_days, 90_days.",
						Validators:          []validator.String{stringvalidator.OneOf(validSLOTimeFrame...)},
					},
				},
				MarkdownDescription: "SLO time window. Currently only time frame is supported.",
			},
		},
		MarkdownDescription: "Coralogix SLO.",
	}
}

func sloResourceSchemaV0() schema.Schema {
	return schema.Schema{
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
				Optional: true,
				//Validators:          []validator.String{stringvalidator.OneOf(validThresholdSymbolTypes...)},
				//MarkdownDescription: fmt.Sprintf("Threshold symbol type. Required when `type` is `latency`. Valid values are: %q", validThresholdSymbolTypes),
			},
			"filters": schema.SetNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"field": schema.StringAttribute{
							Required: true,
						},
						"compare_type": schema.StringAttribute{
							Required: true,
							//Validators:          []validator.String{stringvalidator.OneOf(validSLOCompareTypes...)},
							//MarkdownDescription: fmt.Sprintf("Compare type. This is the compare type of the SLO. Valid values are: %q", validSLOCompareTypes),
						},
						"field_values": schema.SetAttribute{
							Optional:    true,
							ElementType: types.StringType,
						},
					},
				},
			},
			"period": schema.StringAttribute{
				Required: true,
				//Validators:          []validator.String{stringvalidator.OneOf(validSLOPeriods...)},
				//MarkdownDescription: fmt.Sprintf("Period. This is the period of the SLO. Valid values are: %q", validSLOPeriods),
			},
		},
		MarkdownDescription: "Coralogix SLO.",
	}
}

func (r *SLOResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = sloResourceSchemaV1()
}

type SLOResourceModel struct {
	ID                        types.String  `tfsdk:"id"`
	Name                      types.String  `tfsdk:"name"`
	Description               types.String  `tfsdk:"description"`
	Labels                    types.Map     `tfsdk:"labels"`
	Grouping                  types.List    `tfsdk:"grouping"`
	TargetThresholdPercentage types.Float64 `tfsdk:"target_threshold_percentage"`
	SLI                       types.Object  `tfsdk:"sli"`
	Window                    types.Object  `tfsdk:"window"`
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
	Threshold          types.Float64 `tfsdk:"threshold"`
}

type SLOMetricQueryModel struct {
	Query types.String `tfsdk:"query"`
}

type WindowModel struct {
	SloTimeFrame types.String `tfsdk:"slo_time_frame"`
}

func extractSLO(ctx context.Context, plan *SLOResourceModel) (*cxsdk.Slo, diag.Diagnostics) {
	labels, diags := utils.TypeMapToStringMap(ctx, plan.Labels)
	if diags.HasError() {
		return nil, diags
	}

	window, diags := extractWindow(ctx, plan.Window)
	if diags.HasError() {
		return nil, diags
	}

	sli, diags := extractSli(ctx, plan.SLI)
	if diags.HasError() {
		return nil, diags
	}

	slo := &cxsdk.Slo{
		Id:                        plan.ID.ValueString(),
		Name:                      plan.Name.ValueString(),
		Description:               plan.Description.ValueStringPointer(),
		Labels:                    labels,
		TargetThresholdPercentage: plan.TargetThresholdPercentage.ValueInt32(),
		Window:                    window,
		Sli:                       sli,
	}
	return slo, nil
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

func extractSli(ctx context.Context, sli types.Set) (*cxsdk.SloRequestBasedMetricSli, diag.Diagnostics) {
	if sli.IsNull() || sli.IsUnknown() {
		return nil, nil
	}

	var diags diag.Diagnostics
	var sliList []MetricSliModel
	diags = sli.ElementsAs(ctx, &sliList, true)
	if diags.HasError() {
		return nil, diags
	}
	if len(sliList) == 0 {
		return nil, nil
	}

	sliModel := sliList[0]

	var groupByLabels []string
	if !sliModel.GroupByLabels.IsNull() && !sliModel.GroupByLabels.IsUnknown() {
		var elements []types.String
		diags = sliModel.GroupByLabels.ElementsAs(ctx, &elements, true)
		if diags.HasError() {
			return nil, diags
		}
		for _, e := range elements {
			groupByLabels = append(groupByLabels, e.ValueString())
		}
	}

	return &cxsdk.SloMetricSli{
		MetricSli: &cxsdk.MetricSli{
			GoodEvents: &cxsdk.Metric{
				Query: sliModel.GoodEvents.Query.ValueString(),
			},
			TotalEvents: &cxsdk.Metric{
				Query: sliModel.TotalEvents.Query.ValueString(),
			},
			GroupByLabels: groupByLabels,
		},
	}, diags
}

func flattenSLO(ctx context.Context, slo *cxsdk.Slo) (*SLOResourceModel, diag.Diagnostics) {
	sli, diags := flattenSLOMetricSli(ctx, slo.GetMetricSli())
	if diags.HasError() {
		return nil, diags
	}

	window, diags := flattenSLOWindow(ctx, slo.GetSloTimeFrame())
	if diags.HasError() {
		return nil, diags
	}

	labels, diags := types.MapValueFrom(ctx, types.StringType, slo.GetLabels())
	if diags.HasError() {
		return nil, diags
	}

	model := &SLOResourceModel{
		ID:                        types.StringValue(slo.GetId()),
		Name:                      types.StringValue(slo.GetName()),
		Description:               types.StringValue(slo.GetDescription()),
		Labels:                    labels,
		SLI:                       sli,
		Window:                    window,
		TargetThresholdPercentage: types.Int32Value(slo.GetTargetThresholdPercentage()),
	}
	return model, nil
}

func flattenSLOMetricSli(ctx context.Context, sli *cxsdk.MetricSli) (types.Set, diag.Diagnostics) {
	groupByLabels, diags := types.ListValueFrom(ctx, types.StringType, sli.GetGroupByLabels())
	if diags.HasError() {
		return types.SetNull(types.ObjectType{}), diags
	}

	sliBlock := MetricSliModel{
		GoodEvents: SLOMetricQueryModel{
			Query: types.StringValue(sli.GetGoodEvents().GetQuery()),
		},
		TotalEvents: SLOMetricQueryModel{
			Query: types.StringValue(sli.GetTotalEvents().GetQuery()),
		},
		GroupByLabels: groupByLabels,
	}

	sliSet, diags := types.SetValueFrom(ctx, types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"good_events":     types.ObjectType{AttrTypes: map[string]attr.Type{"query": types.StringType}},
			"total_events":    types.ObjectType{AttrTypes: map[string]attr.Type{"query": types.StringType}},
			"group_by_labels": types.ListType{ElemType: types.StringType},
		},
	}, []any{sliBlock})
	return sliSet, diags
}

func flattenSLOWindow(ctx context.Context, frame cxsdk.SloTimeframeEnum) (types.Object, diag.Diagnostics) {
	window := WindowModel{
		SloTimeFrame: types.StringValue(protoToSchemaSloTimeFrame[frame]),
	}
	return types.ObjectValueFrom(ctx, map[string]attr.Type{
		"slo_time_frame": types.StringType,
	}, window)
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
	createSloReq := &cxsdk.CreateServiceSloRequest{Slo: slo}
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
	log.Printf("[INFO] Submitted new SLO: %s", protojson.Format(createSloReq))
	plan, diags = flattenSLO(ctx, slo)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
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
	updateSloReq := &cxsdk.ReplaceServiceSloRequest{Slo: slo}
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
	log.Printf("[INFO] Received SLO: %s", protojson.Format(getSloResp))
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
