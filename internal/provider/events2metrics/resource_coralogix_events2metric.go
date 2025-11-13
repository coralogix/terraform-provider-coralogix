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

package events2metrics

import (
	"context"
	"fmt"
	"log"
	"regexp"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	severitySchemaToProto = map[string]cxsdk.L2MSeverity{
		"Unspecified": cxsdk.L2MSeverityUnspecified,
		"Debug":       cxsdk.L2MSeverityDebug,
		"Verbose":     cxsdk.L2MSeverityVerbose,
		"Info":        cxsdk.L2MSeverityInfo,
		"Warning":     cxsdk.L2MSeverityWarning,
		"Error":       cxsdk.L2MSeverityError,
		"Critical":    cxsdk.L2MSeverityCritical,
	}
	severityProtoToSchema = utils.ReverseMap(severitySchemaToProto)
	validSeverities       = utils.GetKeys(severitySchemaToProto)

	protoToSchemaAggregationType = map[cxsdk.E2MAggregationType]string{
		cxsdk.E2MAggregationTypeMin:       "min",
		cxsdk.E2MAggregationTypeMax:       "max",
		cxsdk.E2MAggregationTypeCount:     "count",
		cxsdk.E2MAggregationTypeAvg:       "avg",
		cxsdk.E2MAggregationTypeSum:       "sum",
		cxsdk.E2MAggregationTypeHistogram: "histogram",
		cxsdk.E2MAggregationTypeSamples:   "samples",
	}
	schemaToProtoAggregationSampleType = map[string]cxsdk.E2MAggSampleType{
		"Min": cxsdk.E2MAggSampleTypeMin,
		"Max": cxsdk.E2MAggSampleTypeMax,
	}

	protoToSchemaAggregationSampleType = utils.ReverseMap(schemaToProtoAggregationSampleType)

	validSampleTypes = utils.GetKeys(schemaToProtoAggregationSampleType)
)

var (
	_ resource.ResourceWithConfigure        = &Events2MetricResource{}
	_ resource.ResourceWithConfigValidators = &Events2MetricResource{}
	_ resource.ResourceWithImportState      = &Events2MetricResource{}
	_ resource.ResourceWithUpgradeState     = &Events2MetricResource{}
)

func NewEvents2MetricResource() resource.Resource {
	return &Events2MetricResource{}
}

type Events2MetricResource struct {
	client *cxsdk.Events2MetricsClient
}

type Events2MetricResourceModel struct {
	ID           types.String       `tfsdk:"id"`
	Name         types.String       `tfsdk:"name"`
	Description  types.String       `tfsdk:"description"`
	MetricFields types.Map          `tfsdk:"metric_fields"`
	MetricLabels types.Map          `tfsdk:"metric_labels"`
	Permutations *PermutationsModel `tfsdk:"permutations"`
	SpansQuery   *SpansQueryModel   `tfsdk:"spans_query"`
	LogsQuery    *LogsQueryModel    `tfsdk:"logs_query"`
}

type MetricFieldModel struct {
	SourceField  types.String       `tfsdk:"source_field"`
	Aggregations *AggregationsModel `tfsdk:"aggregations"`
}

type AggregationsModel struct {
	Min       *CommonAggregationModel    `tfsdk:"min"`
	Max       *CommonAggregationModel    `tfsdk:"max"`
	AVG       *CommonAggregationModel    `tfsdk:"avg"`
	Sum       *CommonAggregationModel    `tfsdk:"sum"`
	Count     *CommonAggregationModel    `tfsdk:"count"`
	Samples   *SamplesAggregationModel   `tfsdk:"samples"`
	Histogram *HistogramAggregationModel `tfsdk:"histogram"`
}

type CommonAggregationModel struct {
	Enable           types.Bool   `tfsdk:"enable"`
	TargetMetricName types.String `tfsdk:"target_metric_name"`
}

type SamplesAggregationModel struct {
	Enable           types.Bool   `tfsdk:"enable"`
	TargetMetricName types.String `tfsdk:"target_metric_name"`
	Type             types.String `tfsdk:"type"`
}

type HistogramAggregationModel struct {
	Enable           types.Bool   `tfsdk:"enable"`
	TargetMetricName types.String `tfsdk:"target_metric_name"`
	Buckets          types.List   `tfsdk:"buckets"` //types.Float64
}

type PermutationsModel struct {
	Limit          types.Int64 `tfsdk:"limit"`
	HasExceedLimit types.Bool  `tfsdk:"has_exceed_limit"`
}

type SpansQueryModel struct {
	Lucene       types.String `tfsdk:"lucene"`
	Applications types.Set    `tfsdk:"applications"`
	Subsystems   types.Set    `tfsdk:"subsystems"`
	Actions      types.Set    `tfsdk:"actions"`
	Services     types.Set    `tfsdk:"services"`
}

type LogsQueryModel struct {
	Lucene       types.String `tfsdk:"lucene"`
	Applications types.Set    `tfsdk:"applications"`
	Subsystems   types.Set    `tfsdk:"subsystems"`
	Severities   types.Set    `tfsdk:"severities"`
}

func metricFieldModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"source_field": types.StringType,
		"aggregations": types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"min":       commonAggregationModelAttr(),
				"max":       commonAggregationModelAttr(),
				"avg":       commonAggregationModelAttr(),
				"sum":       commonAggregationModelAttr(),
				"count":     commonAggregationModelAttr(),
				"samples":   samplesAggregationModelAttr(),
				"histogram": histogramAggregationModelAttr(),
			},
		},
	}
}

func commonAggregationModelAttr() attr.Type {
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"enable":             types.BoolType,
			"target_metric_name": types.StringType,
		},
	}
}

func samplesAggregationModelAttr() attr.Type {
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"enable":             types.BoolType,
			"target_metric_name": types.StringType,
			"type":               types.StringType,
		},
	}
}

func histogramAggregationModelAttr() attr.Type {
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"enable":             types.BoolType,
			"target_metric_name": types.StringType,
			"buckets": types.ListType{
				ElemType: types.Float64Type,
			},
		},
	}
}

func (r *Events2MetricResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_events2metric"
}

func (r *Events2MetricResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.Events2Metrics()
}

func (r *Events2MetricResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 1,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile(`^[A-Za-z\d_:-]*$`), "Invalid metric name, name may only contain ASCII letters and digits, as well as underscores and colons."),
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Events2Metric name. Events2Metric names have to be unique per account.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Events2Metric description.",
			},
			"metric_fields": schema.MapNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"source_field": schema.StringAttribute{
							Required: true,
						},
						"aggregations": schema.SingleNestedAttribute{
							Optional: true,
							Computed: true,
							PlanModifiers: []planmodifier.Object{
								objectplanmodifier.UseStateForUnknown(),
							},
							Attributes: map[string]schema.Attribute{
								"min": schema.SingleNestedAttribute{
									Optional: true,
									Computed: true,
									PlanModifiers: []planmodifier.Object{
										objectplanmodifier.UseStateForUnknown(),
									},
									Attributes: map[string]schema.Attribute{
										"enable": schema.BoolAttribute{
											Optional: true,
											Computed: true,
											PlanModifiers: []planmodifier.Bool{
												boolplanmodifier.UseStateForUnknown(),
											},
										},
										"target_metric_name": schema.StringAttribute{
											Computed: true,
											PlanModifiers: []planmodifier.String{
												stringplanmodifier.UseStateForUnknown(),
											},
										},
									},
								},
								"max": schema.SingleNestedAttribute{
									Optional: true,
									Computed: true,
									PlanModifiers: []planmodifier.Object{
										objectplanmodifier.UseStateForUnknown(),
									},
									Attributes: map[string]schema.Attribute{
										"enable": schema.BoolAttribute{
											Optional: true,
											Computed: true,
											PlanModifiers: []planmodifier.Bool{
												boolplanmodifier.UseStateForUnknown(),
											},
										},
										"target_metric_name": schema.StringAttribute{
											Computed: true,
											PlanModifiers: []planmodifier.String{
												stringplanmodifier.UseStateForUnknown(),
											},
										},
									},
								},
								"count": schema.SingleNestedAttribute{
									Optional: true,
									Computed: true,
									PlanModifiers: []planmodifier.Object{
										objectplanmodifier.UseStateForUnknown(),
									},
									Attributes: map[string]schema.Attribute{
										"enable": schema.BoolAttribute{
											Optional: true,
											Computed: true,
											PlanModifiers: []planmodifier.Bool{
												boolplanmodifier.UseStateForUnknown(),
											},
										},
										"target_metric_name": schema.StringAttribute{
											Computed: true,
											PlanModifiers: []planmodifier.String{
												stringplanmodifier.UseStateForUnknown(),
											},
										},
									},
								},
								"avg": schema.SingleNestedAttribute{
									Optional: true,
									Computed: true,
									PlanModifiers: []planmodifier.Object{
										objectplanmodifier.UseStateForUnknown(),
									},
									Attributes: map[string]schema.Attribute{
										"enable": schema.BoolAttribute{
											Optional: true,
											Computed: true,
											PlanModifiers: []planmodifier.Bool{
												boolplanmodifier.UseStateForUnknown(),
											},
										},
										"target_metric_name": schema.StringAttribute{
											Computed: true,
											PlanModifiers: []planmodifier.String{
												stringplanmodifier.UseStateForUnknown(),
											},
										},
									},
								},
								"sum": schema.SingleNestedAttribute{
									Optional: true,
									Computed: true,
									PlanModifiers: []planmodifier.Object{
										objectplanmodifier.UseStateForUnknown(),
									},
									Attributes: map[string]schema.Attribute{
										"enable": schema.BoolAttribute{
											Optional: true,
											Computed: true,
											PlanModifiers: []planmodifier.Bool{
												boolplanmodifier.UseStateForUnknown(),
											},
										},
										"target_metric_name": schema.StringAttribute{
											Computed: true,
											PlanModifiers: []planmodifier.String{
												stringplanmodifier.UseStateForUnknown(),
											},
										},
									},
								},
								"samples": schema.SingleNestedAttribute{
									Optional: true,
									Computed: true,
									PlanModifiers: []planmodifier.Object{
										objectplanmodifier.UseStateForUnknown(),
									},
									Attributes: map[string]schema.Attribute{
										"enable": schema.BoolAttribute{
											Optional: true,
											Computed: true,
											PlanModifiers: []planmodifier.Bool{
												boolplanmodifier.UseStateForUnknown(),
											},
										},
										"target_metric_name": schema.StringAttribute{
											Computed: true,
											PlanModifiers: []planmodifier.String{
												stringplanmodifier.UseStateForUnknown(),
											},
										},
										"type": schema.StringAttribute{
											Required: true,
											Validators: []validator.String{
												stringvalidator.OneOf(validSampleTypes...),
											},
											MarkdownDescription: fmt.Sprintf("Can be one of %q.", validSampleTypes),
										},
									},
								},
								"histogram": schema.SingleNestedAttribute{
									Optional: true,
									Computed: true,
									PlanModifiers: []planmodifier.Object{
										objectplanmodifier.UseStateForUnknown(),
									},
									Attributes: map[string]schema.Attribute{
										"enable": schema.BoolAttribute{
											Optional: true,
											Computed: true,
											PlanModifiers: []planmodifier.Bool{
												boolplanmodifier.UseStateForUnknown(),
											},
										},
										"target_metric_name": schema.StringAttribute{
											Computed: true,
											PlanModifiers: []planmodifier.String{
												stringplanmodifier.UseStateForUnknown(),
											},
										},
										"buckets": schema.ListAttribute{
											ElementType: types.Float64Type,
											Required:    true,
										},
									},
								},
							},
						},
					},
				},
				Validators: []validator.Map{
					mapvalidator.SizeAtLeast(1),
				},
			},
			"metric_labels": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Validators: []validator.Map{
					mapvalidator.SizeAtLeast(1),
				},
			},
			"permutations": schema.SingleNestedAttribute{
				Optional: true,
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"limit": schema.Int64Attribute{
						Optional: true,
						Computed: true,
						Validators: []validator.Int64{
							int64validator.AtLeast(0),
						},
						MarkdownDescription: "Defines the permutations' limit of the events2metric.",
					},
					"has_exceed_limit": schema.BoolAttribute{
						Computed: true,
						PlanModifiers: []planmodifier.Bool{
							boolplanmodifier.UseStateForUnknown(),
						},
						MarkdownDescription: "Notify if the limit permutations' limit of the events2metric has exceed (computed).",
					},
				},
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Defines the permutations' info of the events2metric.",
			},
			"spans_query": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"lucene": schema.StringAttribute{
						Optional:    true,
						Description: "The search_query that we wanted to be notified on.",
					},
					"applications": schema.SetAttribute{
						ElementType: types.StringType,
						Optional:    true,
						Validators: []validator.Set{
							setvalidator.SizeAtLeast(1),
						},
						MarkdownDescription: "An array that contains log’s application names that we want to be alerted on." +
							" Applications can be filtered by prefix, suffix, and contains using the next patterns - filter:startsWith:xxx, filter:endsWith:xxx, filter:contains:xxx",
					},
					"subsystems": schema.SetAttribute{
						ElementType: types.StringType,
						Optional:    true,
						Validators: []validator.Set{
							setvalidator.SizeAtLeast(1),
						},
						MarkdownDescription: "An array that contains log’s subsystem names that we want to be notified on. " +
							" Subsystems can be filtered by prefix, suffix, and contains using the next patterns - filter:startsWith:xxx, filter:endsWith:xxx, filter:contains:xxx",
					},
					"actions": schema.SetAttribute{
						ElementType: types.StringType,
						Optional:    true,
						Validators: []validator.Set{
							setvalidator.SizeAtLeast(1),
						},
						MarkdownDescription: "An array that contains log’s actions names that we want to be notified on. " +
							" Actions can be filtered by prefix, suffix, and contains using the next patterns - filter:startsWith:xxx, filter:endsWith:xxx, filter:contains:xxx",
					},
					"services": schema.SetAttribute{
						ElementType: types.StringType,
						Optional:    true,
						Validators: []validator.Set{
							setvalidator.SizeAtLeast(1),
						},
						MarkdownDescription: "An array that contains log’s services names that we want to be notified on. " +
							" Services can be filtered by prefix, suffix, and contains using the next patterns - filter:startsWith:xxx, filter:endsWith:xxx, filter:contains:xxx",
					},
				},
				MarkdownDescription: "spans-events2metric type. Exactly one of \"spans_query\" or \"logs_query\" should be defined.",
			},
			"logs_query": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"lucene": schema.StringAttribute{
						Optional:    true,
						Description: "The search_query that we wanted to be notified on.",
					},
					"applications": schema.SetAttribute{
						ElementType: types.StringType,
						Optional:    true,
						Validators: []validator.Set{
							setvalidator.SizeAtLeast(1),
						},
						MarkdownDescription: "An array that contains log’s application names that we want to be alerted on." +
							" Applications can be filtered by prefix, suffix, and contains using the next patterns - filter:startsWith:xxx, filter:endsWith:xxx, filter:contains:xxx",
					},
					"subsystems": schema.SetAttribute{
						ElementType: types.StringType,
						Optional:    true,
						Validators: []validator.Set{
							setvalidator.SizeAtLeast(1),
						},
						MarkdownDescription: "An array that contains log’s subsystem names that we want to be notified on. " +
							" Subsystems can be filtered by prefix, suffix, and contains using the next patterns - filter:startsWith:xxx, filter:endsWith:xxx, filter:contains:xxx",
					},
					"severities": schema.SetAttribute{
						Optional:    true,
						ElementType: types.StringType,
						Validators: []validator.Set{
							setvalidator.SizeAtLeast(1),
							setvalidator.ValueStringsAre(stringvalidator.OneOf(validSeverities...)),
						},
						MarkdownDescription: fmt.Sprintf("An array of severities that we interested in. Can be one of %q", validSeverities),
					},
				},
				MarkdownDescription: "logs-events2metric type. Exactly one of \"spans_query\" or \"logs_query\" must be defined.",
			},
		},
	}
}

func (r *Events2MetricResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	schemaV0 := e2mSchemaV0()
	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema:   &schemaV0,
			StateUpgrader: upgradeE2MStateV0ToV1,
		},
	}
}

func upgradeE2MStateV0ToV1(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
	type Events2MetricResourceModelV0 struct {
		ID           types.String `tfsdk:"id"`
		Name         types.String `tfsdk:"name"`
		Description  types.String `tfsdk:"description"`
		MetricFields types.Set    `tfsdk:"metric_fields"`
		MetricLabels types.Set    `tfsdk:"metric_labels"`
		Permutations types.List   `tfsdk:"permutations"`
		SpansQuery   types.List   `tfsdk:"spans_query"`
		LogsQuery    types.List   `tfsdk:"logs_query"`
	}

	var priorStateData Events2MetricResourceModelV0
	resp.Diagnostics.Append(req.State.Get(ctx, &priorStateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	upgradedStateData := Events2MetricResourceModel{
		ID:           priorStateData.ID,
		Description:  priorStateData.Description,
		MetricFields: upgradeE2MMetricFieldsV0ToV1(ctx, priorStateData.MetricFields),
		MetricLabels: upgradeE2MMetricLabelsV0ToV1(ctx, priorStateData.MetricLabels),
		Permutations: upgradeE2MPermutationsV0ToV1(ctx, priorStateData.Permutations),
		SpansQuery:   upgradeE2MSpansQueryV0ToV1(ctx, priorStateData.SpansQuery),
		LogsQuery:    upgradeE2MLogsQueryV0ToV1(ctx, priorStateData.LogsQuery),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, upgradedStateData)...)
}

func upgradeE2MLogsQueryV0ToV1(ctx context.Context, logsQuery types.List) *LogsQueryModel {
	var logsQueryObjects []types.Object
	logsQuery.ElementsAs(ctx, &logsQueryObjects, true)
	if len(logsQueryObjects) == 0 {
		return nil
	}

	var logsQueryObject LogsQueryModel
	logsQueryObjects[0].As(ctx, &logsQueryObject, basetypes.ObjectAsOptions{})
	return &logsQueryObject
}

func upgradeE2MSpansQueryV0ToV1(ctx context.Context, spansQuery types.List) *SpansQueryModel {
	var spansQueryObjects []types.Object
	spansQuery.ElementsAs(ctx, &spansQueryObjects, true)
	if len(spansQueryObjects) == 0 {
		return nil
	}

	var spansQueryObject SpansQueryModel
	spansQueryObjects[0].As(ctx, &spansQueryObject, basetypes.ObjectAsOptions{})
	return &spansQueryObject
}

func upgradeE2MPermutationsV0ToV1(ctx context.Context, permutations types.List) *PermutationsModel {
	var permutationsObjects []types.Object
	permutations.ElementsAs(ctx, &permutationsObjects, true)
	if len(permutationsObjects) == 0 {
		return nil
	}

	var permutationsObject PermutationsModel
	permutationsObjects[0].As(ctx, &permutationsObject, basetypes.ObjectAsOptions{})
	return &permutationsObject
}

func upgradeE2MMetricLabelsV0ToV1(ctx context.Context, labels types.Set) types.Map {
	type MetricLabelV0Model struct {
		TargetLabel types.String `tfsdk:"target_label"`
		SourceField types.String `tfsdk:"source_field"`
	}

	var labelsObjects []types.Object
	labels.ElementsAs(ctx, &labelsObjects, true)
	elements := make(map[string]attr.Value)
	for _, lo := range labelsObjects {
		var metricLabel MetricLabelV0Model
		lo.As(ctx, &metricLabel, basetypes.ObjectAsOptions{})
		elements[metricLabel.TargetLabel.ValueString()] = metricLabel.SourceField
	}

	return types.MapValueMust(types.StringType, elements)
}

func upgradeE2MMetricFieldsV0ToV1(ctx context.Context, fields types.Set) types.Map {
	type MetricFieldV0Model struct {
		TargetBaseMetricName types.String       `tfsdk:"target_base_metric_name"`
		SourceField          types.String       `tfsdk:"source_field"`
		Aggregations         *AggregationsModel `tfsdk:"aggregations"`
	}

	var fieldObjects []types.Object
	fields.ElementsAs(ctx, &fieldObjects, true)
	elements := make(map[string]attr.Value)
	for _, fo := range fieldObjects {
		var metricFieldV0 MetricFieldV0Model
		fo.As(ctx, &metricFieldV0, basetypes.ObjectAsOptions{})
		field := MetricFieldModel{
			SourceField:  metricFieldV0.SourceField,
			Aggregations: metricFieldV0.Aggregations,
		}
		element, _ := types.ObjectValueFrom(ctx, metricFieldModelAttr(), field)
		elements[metricFieldV0.TargetBaseMetricName.ValueString()] = element

	}

	return types.MapValueMust(types.ObjectType{AttrTypes: metricFieldModelAttr()}, elements)
}

func e2mSchemaV0() schema.Schema {
	return schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"name": schema.StringAttribute{
				Required: true,
			},
			"description": schema.StringAttribute{
				Optional: true,
			},
		},
		Blocks: map[string]schema.Block{
			"metric_labels": schema.SetNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"target_label": schema.StringAttribute{
							Required: true,
						},
						"source_field": schema.StringAttribute{
							Required: true,
						},
					},
				},
			},
			"metric_fields": schema.SetNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"source_field": schema.StringAttribute{
							Required: true,
						},
					},
					Blocks: map[string]schema.Block{
						"aggregations": schema.SetNestedBlock{
							NestedObject: schema.NestedBlockObject{
								Blocks: map[string]schema.Block{
									"min":   commonAggregationSchemaV0(),
									"max":   commonAggregationSchemaV0(),
									"count": commonAggregationSchemaV0(),
									"avg":   commonAggregationSchemaV0(),
									"sum":   commonAggregationSchemaV0(),
									"samples": schema.ListNestedBlock{
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												"enable": schema.BoolAttribute{
													Optional: true,
													Computed: true,
												},
												"target_metric_name": schema.StringAttribute{
													Computed: true,
												},
												"type": schema.StringAttribute{
													Required: true,
												},
											},
										},
									},
									"histogram": schema.ListNestedBlock{
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												"enable": schema.BoolAttribute{
													Optional: true,
													Computed: true,
												},
												"target_metric_name": schema.StringAttribute{
													Computed: true,
												},
												"buckets": schema.ListAttribute{
													ElementType: types.Float64Type,
													Required:    true,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"spans_query": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"lucene": schema.StringAttribute{
							Optional: true,
						},
						"applications": schema.SetAttribute{
							ElementType: types.StringType,
							Optional:    true,
						},
						"subsystems": schema.SetAttribute{
							ElementType: types.StringType,
							Optional:    true,
						},
						"actions": schema.SetAttribute{
							ElementType: types.StringType,
							Optional:    true,
						},
						"services": schema.SetAttribute{
							ElementType: types.StringType,
							Optional:    true,
						},
					},
				},
			},
			"logs_query": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"lucene": schema.StringAttribute{
							Optional: true,
						},
						"applications": schema.SetAttribute{
							ElementType: types.StringType,
							Optional:    true,
						},
						"subsystems": schema.SetAttribute{
							ElementType: types.StringType,
							Optional:    true,
						},
						"severities": schema.SetAttribute{
							Optional:    true,
							Computed:    true,
							ElementType: types.StringType,
						},
					},
				},
			},
			"permutations": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"limit":            schema.StringAttribute{},
						"has_exceed_limit": schema.BoolAttribute{},
					},
				},
			},
		},
	}
}

func commonAggregationSchemaV0() schema.ListNestedBlock {
	return schema.ListNestedBlock{
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"enable": schema.BoolAttribute{
					Optional: true,
					Computed: true,
				},
				"target_metric_name": schema.StringAttribute{
					Computed: true,
				},
			},
		},
	}
}

func (r *Events2MetricResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("spans_query"),
			path.MatchRoot("logs_query"),
		),
	}
}

func (r *Events2MetricResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan Events2MetricResourceModel
	diags := req.Plan.Get(ctx, &plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	e2mCreateReq, diags := extractCreateE2M(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	log.Printf("[INFO] Creating new Events2metric: %s", protojson.Format(e2mCreateReq))
	e2mCreateResp, err := r.client.Create(ctx, e2mCreateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error creating Events2Metric",
			utils.FormatRpcErrors(err, cxsdk.E2MCreateRPC, protojson.Format(e2mCreateReq)),
		)
	}
	log.Printf("[INFO] Submitted new Events2metric: %s", protojson.Format(e2mCreateResp))

	plan = flattenE2M(ctx, e2mCreateResp.GetE2M())

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *Events2MetricResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Events2MetricResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed Events2Metric value from Coralogix
	id := state.ID.ValueString()
	log.Printf("[INFO] Reading Events2metric: %s", id)
	getE2MReq := &cxsdk.GetE2MRequest{Id: wrapperspb.String(id)}
	getE2MResp, err := r.client.Get(ctx, getE2MReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Events2Metric %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Events2Metric",
				utils.FormatRpcErrors(err, cxsdk.E2MGetRPC, protojson.Format(getE2MReq)),
			)
		}
		return
	}
	log.Printf("[INFO] Received Events2metric: %s", protojson.Format(getE2MResp))

	state = flattenE2M(ctx, getE2MResp.GetE2M())
	//
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *Events2MetricResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan Events2MetricResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	e2mUpdateReq, diags := extractUpdateE2M(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	log.Printf("[INFO] Updating Events2metric: %s", protojson.Format(e2mUpdateReq))
	e2mUpdateResp, err := r.client.Replace(ctx, e2mUpdateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating Events2Metric",
			utils.FormatRpcErrors(err, cxsdk.E2MReplaceRPC, protojson.Format(e2mUpdateReq)),
		)
		return
	}
	log.Printf("[INFO] Submitted updated Events2metric: %s", protojson.Format(e2mUpdateResp))

	// Get refreshed Events2Metric value from Coralogix
	id := plan.ID.ValueString()
	getE2MResp, err := r.client.Get(ctx, &cxsdk.GetE2MRequest{Id: wrapperspb.String(id)})
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Events2Metric %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Events2Metric",
				utils.FormatRpcErrors(err, cxsdk.E2MGetRPC, protojson.Format(e2mUpdateReq)),
			)
		}
		return
	}
	log.Printf("[INFO] Received Events2metric: %s", protojson.Format(getE2MResp))

	plan = flattenE2M(ctx, e2mUpdateResp.GetE2M())

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *Events2MetricResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Events2MetricResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	deleteReq := &cxsdk.DeleteE2MRequest{Id: wrapperspb.String(id)}
	log.Printf("[INFO] Deleting Events2metric %s\n", id)
	if _, err := r.client.Delete(ctx, deleteReq); err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Events2Metric",
			utils.FormatRpcErrors(err, cxsdk.E2MDeleteRPC, protojson.Format(deleteReq)),
		)
		return
	}
	log.Printf("[INFO] Events2metric %s deleted\n", id)
}

func (r *Events2MetricResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func flattenE2M(ctx context.Context, e2m *cxsdk.E2M) Events2MetricResourceModel {
	return Events2MetricResourceModel{
		ID:           types.StringValue(e2m.GetId().GetValue()),
		Name:         types.StringValue(e2m.GetName().GetValue()),
		Description:  flattenDescription(e2m.GetDescription()),
		MetricFields: flattenE2MMetricFields(ctx, e2m.GetMetricFields()),
		MetricLabels: flattenE2MMetricLabels(e2m.GetMetricLabels()),
		Permutations: flattenE2MPermutations(e2m.GetPermutations()),
		SpansQuery:   flattenSpansQuery(e2m.GetSpansQuery()),
		LogsQuery:    flattenLogsQuery(e2m.GetLogsQuery()),
	}
}

func flattenDescription(e2mDescription *wrapperspb.StringValue) types.String {
	if e2mDescription == nil {
		return types.StringNull()
	}
	return types.StringValue(e2mDescription.GetValue())
}

func extractCreateE2M(ctx context.Context, plan Events2MetricResourceModel) (*cxsdk.CreateE2MRequest, diag.Diagnostics) {
	name := utils.TypeStringToWrapperspbString(plan.Name)
	description := utils.TypeStringToWrapperspbString(plan.Description)
	permutations := expandPermutations(plan.Permutations)
	permutationsLimit := wrapperspb.Int32(permutations.GetLimit())
	metricLabels, diags := expandE2MLabels(ctx, plan.MetricLabels)
	if diags.HasError() {
		return nil, diags
	}
	metricFields, diags := expandE2MFields(ctx, plan.MetricFields)
	if diags.HasError() {
		return nil, diags
	}

	e2mParams := &cxsdk.E2MCreateParams{
		Name:              name,
		Description:       description,
		PermutationsLimit: permutationsLimit,
		MetricLabels:      metricLabels,
		MetricFields:      metricFields,
	}

	if spansQuery := plan.SpansQuery; spansQuery != nil {
		e2mParams.Type = cxsdk.E2MTypeSpans2Metrics
		e2mParams.Query, diags = expandSpansQuery(ctx, spansQuery)
	} else if logsQuery := plan.LogsQuery; logsQuery != nil {
		e2mParams.Type = cxsdk.E2MTypeLogs2Metrics
		e2mParams.Query, diags = expandLogsQuery(ctx, logsQuery)
	}

	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.CreateE2MRequest{
		E2M: e2mParams,
	}, nil
}

func expandPermutations(permutations *PermutationsModel) *cxsdk.E2MPermutations {
	if permutations == nil {
		return nil
	}
	return &cxsdk.E2MPermutations{
		Limit:            int32(permutations.Limit.ValueInt64()),
		HasExceededLimit: permutations.HasExceedLimit.ValueBool(),
	}
}

func extractUpdateE2M(ctx context.Context, plan Events2MetricResourceModel) (*cxsdk.ReplaceE2MRequest, diag.Diagnostics) {
	id := wrapperspb.String(plan.ID.ValueString())
	name := wrapperspb.String(plan.Name.ValueString())
	description := wrapperspb.String(plan.Description.ValueString())
	permutations := expandPermutations(plan.Permutations)
	metricLabels, diags := expandE2MLabels(ctx, plan.MetricLabels)
	if diags.HasError() {
		return nil, diags
	}
	metricFields, diags := expandE2MFields(ctx, plan.MetricFields)
	if diags.HasError() {
		return nil, diags
	}

	e2mParams := &cxsdk.E2M{
		Id:           id,
		Name:         name,
		Description:  description,
		Permutations: permutations,
		MetricLabels: metricLabels,
		MetricFields: metricFields,
	}

	if spansQuery := plan.SpansQuery; spansQuery != nil {
		e2mParams.Type = cxsdk.E2MTypeSpans2Metrics
		e2mParams.Query, diags = expandUpdateSpansQuery(ctx, spansQuery)
	} else if logsQuery := plan.LogsQuery; logsQuery != nil {
		e2mParams.Type = cxsdk.E2MTypeLogs2Metrics
		e2mParams.Query, diags = expandUpdateLogsQuery(ctx, logsQuery)
	}

	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.ReplaceE2MRequest{
		E2M: e2mParams,
	}, nil
}

func expandE2MLabels(ctx context.Context, labels types.Map) ([]*cxsdk.MetricLabel, diag.Diagnostics) {
	labelsMap := labels.Elements()
	result := make([]*cxsdk.MetricLabel, 0, len(labelsMap))
	var diags diag.Diagnostics
	for targetField, value := range labelsMap {
		v, _ := value.ToTerraformValue(ctx)
		var sourceField string
		if err := v.As(&sourceField); err != nil {
			diags.AddError("error expanding metric labels",
				err.Error())
			continue
		}
		label := expandE2MLabel(targetField, sourceField)
		result = append(result, label)
	}
	if diags.HasError() {
		return nil, diags
	}

	return result, nil
}

func expandE2MLabel(targetLabel, sourceField string) *cxsdk.MetricLabel {
	return &cxsdk.MetricLabel{
		TargetLabel: wrapperspb.String(targetLabel),
		SourceField: wrapperspb.String(sourceField),
	}
}

func expandE2MFields(ctx context.Context, fields types.Map) ([]*cxsdk.MetricField, diag.Diagnostics) {
	var fieldsMap map[string]MetricFieldModel
	var diags diag.Diagnostics
	d := fields.ElementsAs(ctx, &fieldsMap, true)
	if d != nil {
		panic(d)
	}
	result := make([]*cxsdk.MetricField, 0, len(fieldsMap))
	for sourceField, metricFieldValue := range fieldsMap {
		field, dgs := expandE2MField(ctx, sourceField, metricFieldValue)
		if dgs.HasError() {
			diags = append(diags, dgs...)
			continue
		}
		result = append(result, field)
	}

	return result, diags
}

func expandE2MField(ctx context.Context, targetField string, metricField MetricFieldModel) (*cxsdk.MetricField, diag.Diagnostics) {
	aggregations, diags := expandE2MAggregations(ctx, metricField.Aggregations)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.MetricField{
		TargetBaseMetricName: wrapperspb.String(targetField),
		SourceField:          wrapperspb.String(metricField.SourceField.ValueString()),
		Aggregations:         aggregations,
	}, nil
}

func expandE2MAggregations(ctx context.Context, aggregationsModel *AggregationsModel) ([]*cxsdk.E2MAggregation, diag.Diagnostics) {
	if aggregationsModel == nil {
		return nil, nil
	}

	aggregations := make([]*cxsdk.E2MAggregation, 0)

	if min := aggregationsModel.Min; min != nil {
		aggregation := &cxsdk.E2MAggregation{AggType: cxsdk.E2MAggregationTypeMin, Enabled: min.Enable.ValueBool(), TargetMetricName: "min"}
		aggregations = append(aggregations, aggregation)
	}
	if max := aggregationsModel.Max; max != nil {
		aggregation := &cxsdk.E2MAggregation{AggType: cxsdk.E2MAggregationTypeMax, Enabled: max.Enable.ValueBool(), TargetMetricName: "max"}
		aggregations = append(aggregations, aggregation)

	}
	if count := aggregationsModel.Count; count != nil {
		aggregation := &cxsdk.E2MAggregation{AggType: cxsdk.E2MAggregationTypeCount, Enabled: count.Enable.ValueBool(), TargetMetricName: "count"}
		aggregations = append(aggregations, aggregation)
	}
	if avg := aggregationsModel.AVG; avg != nil {
		aggregation := &cxsdk.E2MAggregation{AggType: cxsdk.E2MAggregationTypeAvg, Enabled: avg.Enable.ValueBool(), TargetMetricName: "avg"}
		aggregations = append(aggregations, aggregation)

	}
	if sum := aggregationsModel.Sum; sum != nil {
		aggregation := &cxsdk.E2MAggregation{AggType: cxsdk.E2MAggregationTypeSum, Enabled: sum.Enable.ValueBool(), TargetMetricName: "sum"}
		aggregations = append(aggregations, aggregation)

	}
	if samples := aggregationsModel.Samples; samples != nil {
		samplesType := schemaToProtoAggregationSampleType[samples.Type.ValueString()]
		aggregation := &cxsdk.E2MAggregation{AggType: cxsdk.E2MAggregationTypeSamples, Enabled: samples.Enable.ValueBool(), TargetMetricName: "samples", AggMetadata: &cxsdk.E2MAggregationSamples{Samples: &cxsdk.E2MAggSamples{SampleType: samplesType}}}
		aggregations = append(aggregations, aggregation)
	}
	if histogram := aggregationsModel.Histogram; histogram != nil {
		buckets, diags := utils.AttrSliceToFloat32Slice(ctx, histogram.Buckets.Elements())
		if diags.HasError() {
			return nil, diags
		}
		aggregation := &cxsdk.E2MAggregation{AggType: cxsdk.E2MAggregationTypeHistogram, Enabled: histogram.Enable.ValueBool(), TargetMetricName: "histogram", AggMetadata: &cxsdk.E2MAggregationHistogram{Histogram: &cxsdk.E2MAggHistogram{Buckets: buckets}}}
		aggregations = append(aggregations, aggregation)

	}

	return aggregations, nil
}

func expandSpansQuery(ctx context.Context, spansQuery *SpansQueryModel) (*cxsdk.E2MCreateParamsSpansQuery, diag.Diagnostics) {
	lucene := utils.TypeStringToWrapperspbString(spansQuery.Lucene)
	applications, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, spansQuery.Applications.Elements())
	if diags.HasError() {
		return nil, diags
	}
	subsystems, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, spansQuery.Subsystems.Elements())
	if diags.HasError() {
		return nil, diags
	}
	actions, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, spansQuery.Actions.Elements())
	if diags.HasError() {
		return nil, diags
	}
	services, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, spansQuery.Services.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.E2MCreateParamsSpansQuery{
		SpansQuery: &cxsdk.S2MSpansQuery{
			Lucene:                 lucene,
			ApplicationnameFilters: applications,
			SubsystemnameFilters:   subsystems,
			ActionFilters:          actions,
			ServiceFilters:         services,
		},
	}, nil
}

func expandLogsQuery(ctx context.Context, logsQuery *LogsQueryModel) (*cxsdk.E2MCreateParamsLogsQuery, diag.Diagnostics) {
	searchQuery := utils.TypeStringToWrapperspbString(logsQuery.Lucene)
	applications, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, logsQuery.Applications.Elements())
	if diags.HasError() {
		return nil, diags
	}
	subsystems, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, logsQuery.Subsystems.Elements())
	if diags.HasError() {
		return nil, diags
	}
	severities, diags := expandLogsQuerySeverities(ctx, logsQuery.Severities.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.E2MCreateParamsLogsQuery{
		LogsQuery: &cxsdk.L2MLogsQuery{
			Lucene:                 searchQuery,
			ApplicationnameFilters: applications,
			SubsystemnameFilters:   subsystems,
			SeverityFilters:        severities,
		},
	}, nil
}

func expandUpdateSpansQuery(ctx context.Context, spansQuery *SpansQueryModel) (*cxsdk.E2MSpansQuery, diag.Diagnostics) {
	lucene := utils.TypeStringToWrapperspbString(spansQuery.Lucene)
	applications, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, spansQuery.Applications.Elements())
	if diags != nil {
		return nil, diags
	}
	subsystems, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, spansQuery.Subsystems.Elements())
	if diags != nil {
		return nil, diags
	}
	actions, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, spansQuery.Actions.Elements())
	if diags != nil {
		return nil, diags
	}
	services, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, spansQuery.Services.Elements())
	if diags != nil {
		return nil, diags
	}

	return &cxsdk.E2MSpansQuery{
		SpansQuery: &cxsdk.S2MSpansQuery{
			Lucene:                 lucene,
			ApplicationnameFilters: applications,
			SubsystemnameFilters:   subsystems,
			ActionFilters:          actions,
			ServiceFilters:         services,
		},
	}, nil
}

func expandUpdateLogsQuery(ctx context.Context, logsQuery *LogsQueryModel) (*cxsdk.E2MLogsQuery, diag.Diagnostics) {
	searchQuery := wrapperspb.String(logsQuery.Lucene.ValueString())
	applications, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, logsQuery.Applications.Elements())
	if diags.HasError() {
		return nil, diags
	}
	subsystems, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, logsQuery.Subsystems.Elements())
	if diags.HasError() {
		return nil, diags
	}
	severities, diags := expandLogsQuerySeverities(ctx, logsQuery.Severities.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.E2MLogsQuery{
		LogsQuery: &cxsdk.L2MLogsQuery{
			Lucene:                 searchQuery,
			ApplicationnameFilters: applications,
			SubsystemnameFilters:   subsystems,
			SeverityFilters:        severities,
		},
	}, nil
}

func expandLogsQuerySeverities(ctx context.Context, severities []attr.Value) ([]cxsdk.L2MSeverity, diag.Diagnostics) {
	result := make([]cxsdk.L2MSeverity, 0, len(severities))
	var diags diag.Diagnostics
	for _, s := range severities {
		v, err := s.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("error expanding logs query severities",
				err.Error())
			continue
		}
		var str string
		if err = v.As(&str); err != nil {
			diags.AddError("error expanding logs query severities",
				err.Error())
			continue
		}
		severity := cxsdk.L2MSeverity(severitySchemaToProto[str])
		result = append(result, severity)
	}

	if diags.HasError() {
		return nil, diags
	}

	return result, nil
}

func flattenE2MPermutations(permutations *cxsdk.E2MPermutations) *PermutationsModel {
	if permutations == nil {
		return nil
	}
	return &PermutationsModel{
		Limit:          types.Int64Value(int64(permutations.GetLimit())),
		HasExceedLimit: types.BoolValue(permutations.GetHasExceededLimit()),
	}
}

func flattenE2MMetricFields(ctx context.Context, fields []*cxsdk.MetricField) types.Map {
	if len(fields) == 0 {
		return types.MapNull(types.ObjectType{AttrTypes: metricFieldModelAttr()})
	}

	elements := make(map[string]attr.Value)
	for _, f := range fields {
		target, field := flattenE2MMetricField(ctx, f)
		element, _ := types.ObjectValueFrom(ctx, metricFieldModelAttr(), field)
		elements[target] = element
	}
	return types.MapValueMust(types.ObjectType{AttrTypes: metricFieldModelAttr()}, elements)
}

func flattenE2MMetricField(ctx context.Context, field *cxsdk.MetricField) (string, MetricFieldModel) {
	aggregations := flattenE2MAggregations(ctx, field.GetAggregations())
	return field.GetTargetBaseMetricName().GetValue(), MetricFieldModel{
		SourceField:  types.StringValue(field.GetSourceField().GetValue()),
		Aggregations: aggregations,
	}
}

func flattenE2MAggregations(ctx context.Context, aggregations []*cxsdk.E2MAggregation) *AggregationsModel {
	aggregationsSchema := AggregationsModel{}

	for _, aggregation := range aggregations {
		aggTypeStr := protoToSchemaAggregationType[aggregation.GetAggType()]
		switch aggTypeStr {
		case "min":
			aggregationsSchema.Min = flattenE2MCommonAggregation(aggregation)
		case "max":
			aggregationsSchema.Max = flattenE2MCommonAggregation(aggregation)
		case "avg":
			aggregationsSchema.AVG = flattenE2MCommonAggregation(aggregation)
		case "sum":
			aggregationsSchema.Sum = flattenE2MCommonAggregation(aggregation)
		case "count":
			aggregationsSchema.Count = flattenE2MCommonAggregation(aggregation)
		case "samples":
			aggregationsSchema.Samples = flattenE2MSamplesAggregation(aggregation)
		case "histogram":
			aggregationsSchema.Histogram = flattenE2MHistogramAggregation(ctx, aggregation)
		}
	}

	return &aggregationsSchema
}

func flattenE2MCommonAggregation(aggregation *cxsdk.E2MAggregation) *CommonAggregationModel {
	if aggregation == nil {
		return nil
	}

	return &CommonAggregationModel{
		Enable:           types.BoolValue(aggregation.GetEnabled()),
		TargetMetricName: types.StringValue(aggregation.GetTargetMetricName()),
	}
}

func flattenE2MSamplesAggregation(aggregation *cxsdk.E2MAggregation) *SamplesAggregationModel {
	if aggregation == nil {
		return nil
	}

	samplesType := protoToSchemaAggregationSampleType[aggregation.GetSamples().GetSampleType()]
	return &SamplesAggregationModel{
		Enable:           types.BoolValue(aggregation.GetEnabled()),
		TargetMetricName: types.StringValue(aggregation.GetTargetMetricName()),
		Type:             types.StringValue(samplesType),
	}
}

func flattenE2MHistogramAggregation(ctx context.Context, aggregation *cxsdk.E2MAggregation) *HistogramAggregationModel {
	if aggregation == nil {
		return nil
	}

	buckets, diags := utils.Float32SliceTypeList(ctx, aggregation.GetHistogram().GetBuckets())
	if diags.HasError() {
		return nil
	}
	return &HistogramAggregationModel{
		Enable:           types.BoolValue(aggregation.GetEnabled()),
		TargetMetricName: types.StringValue(aggregation.GetTargetMetricName()),
		Buckets:          buckets,
	}
}

func flattenE2MMetricLabels(labels []*cxsdk.MetricLabel) types.Map {
	if len(labels) == 0 {
		return types.MapNull(types.StringType)
	}

	elements := make(map[string]attr.Value)
	for _, l := range labels {
		key, value := l.GetTargetLabel().GetValue(), l.GetSourceField().GetValue()
		elements[key] = types.StringValue(value)
	}

	return types.MapValueMust(types.StringType, elements)
}

func flattenSpansQuery(query *cxsdk.S2MSpansQuery) *SpansQueryModel {
	if query == nil {
		return nil
	}
	return &SpansQueryModel{
		Lucene:       utils.WrapperspbStringToTypeString(query.GetLucene()),
		Applications: utils.WrappedStringSliceToTypeStringSet(query.GetApplicationnameFilters()),
		Subsystems:   utils.WrappedStringSliceToTypeStringSet(query.GetSubsystemnameFilters()),
		Actions:      utils.WrappedStringSliceToTypeStringSet(query.GetActionFilters()),
		Services:     utils.WrappedStringSliceToTypeStringSet(query.GetServiceFilters()),
	}
}

func flattenLogsQuery(query *cxsdk.L2MLogsQuery) *LogsQueryModel {
	if query == nil {
		return nil
	}
	return &LogsQueryModel{
		Lucene:       utils.WrapperspbStringToTypeString(query.GetLucene()),
		Applications: utils.WrappedStringSliceToTypeStringSet(query.GetApplicationnameFilters()),
		Subsystems:   utils.WrappedStringSliceToTypeStringSet(query.GetSubsystemnameFilters()),
		Severities:   flattenLogQuerySeverities(query.GetSeverityFilters()),
	}
}

func flattenLogQuerySeverities(severities []cxsdk.L2MSeverity) types.Set {
	if len(severities) == 0 {
		return types.SetNull(types.StringType)
	}
	elements := make([]attr.Value, 0, len(severities))
	for _, v := range severities {
		log.Println("severity: ", severityProtoToSchema[cxsdk.L2MSeverity(v)])
		severity := types.StringValue(severityProtoToSchema[cxsdk.L2MSeverity(v)])
		elements = append(elements, severity)
	}
	return types.SetValueMust(types.StringType, elements)
}
