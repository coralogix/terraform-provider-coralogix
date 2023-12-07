package coralogix

import (
	"context"
	"fmt"
	"log"
	"regexp"

	"terraform-provider-coralogix/coralogix/clientset"
	e2m "terraform-provider-coralogix/coralogix/clientset/grpc/events2metrics/v2"
	l2m "terraform-provider-coralogix/coralogix/clientset/grpc/logs2metrics/v2"

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
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	validSeverities              = getKeysInt32(l2m.Severity_value)
	protoToSchemaAggregationType = map[e2m.Aggregation_AggType]string{
		e2m.Aggregation_AGG_TYPE_MIN:       "min",
		e2m.Aggregation_AGG_TYPE_MAX:       "max",
		e2m.Aggregation_AGG_TYPE_COUNT:     "count",
		e2m.Aggregation_AGG_TYPE_AVG:       "avg",
		e2m.Aggregation_AGG_TYPE_SUM:       "sum",
		e2m.Aggregation_AGG_TYPE_HISTOGRAM: "histogram",
		e2m.Aggregation_AGG_TYPE_SAMPLES:   "samples",
	}
	schemaToProtoAggregationSampleType = map[string]e2m.E2MAggSamples_SampleType{
		"Min": e2m.E2MAggSamples_SAMPLE_TYPE_MIN,
		"Max": e2m.E2MAggSamples_SAMPLE_TYPE_MAX,
	}
	protoToSchemaAggregationSampleType = map[e2m.E2MAggSamples_SampleType]string{
		e2m.E2MAggSamples_SAMPLE_TYPE_MIN: "Min",
		e2m.E2MAggSamples_SAMPLE_TYPE_MAX: "Max",
	}
	validSampleTypes       = []string{"Min", "Max"}
	createEvents2MetricURL = "com.coralogixapis.events2metrics.v2.Events2MetricService/CreateE2M"
	getEvents2MetricURL    = "com.coralogixapis.events2metrics.v2.Events2MetricService/GetE2M"
	updateEvents2MetricURL = "com.coralogixapis.events2metrics.v2.Events2MetricService/ReplaceE2M"
	deleteEvents2MetricURL = "com.coralogixapis.events2metrics.v2.Events2MetricService/DeleteE2M"
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
	client *clientset.Events2MetricsClient
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
	Enable           types.Bool      `tfsdk:"enable"`
	TargetMetricName types.String    `tfsdk:"target_metric_name"`
	Buckets          []types.Float64 `tfsdk:"buckets"`
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
						"target_base_metric_name": schema.StringAttribute{
							Required: true,
						},
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
	e2mCreateResp, err := r.client.CreateEvents2Metric(ctx, e2mCreateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		resp.Diagnostics.AddError(
			"Error creating Events2Metric",
			formatRpcErrors(err, createEvents2MetricURL, protojson.Format(e2mCreateReq)),
		)
	}
	log.Printf("[INFO] Submitted new Events2metric: %s", protojson.Format(e2mCreateResp))

	plan = flattenE2M(ctx, e2mCreateResp.GetE2M())

	// Set state to fully populated data
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
	getE2MResp, err := r.client.GetEvents2Metric(ctx, &e2m.GetE2MRequest{Id: wrapperspb.String(id)})
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if status.Code(err) == codes.NotFound {
			state.ID = types.StringNull()
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Events2Metric %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Events2Metric",
				formatRpcErrors(err, getEvents2MetricURL, protojson.Format(getE2MResp)),
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
	e2mUpdateResp, err := r.client.UpdateEvents2Metric(ctx, e2mUpdateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		resp.Diagnostics.AddError(
			"Error updating Events2Metric",
			formatRpcErrors(err, updateEvents2MetricURL, protojson.Format(e2mUpdateReq)),
		)
		return
	}
	log.Printf("[INFO] Submitted updated Events2metric: %#v", e2mUpdateResp)

	// Get refreshed Events2Metric value from Coralogix
	id := plan.ID.ValueString()
	getE2MResp, err := r.client.GetEvents2Metric(ctx, &e2m.GetE2MRequest{Id: wrapperspb.String(id)})
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if status.Code(err) == codes.NotFound {
			plan.ID = types.StringNull()
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Events2Metric %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Events2Metric",
				formatRpcErrors(err, getEvents2MetricURL, protojson.Format(e2mUpdateReq)),
			)
		}
		return
	}
	log.Printf("[INFO] Received Events2metric: %s", protojson.Format(getE2MResp))

	plan = flattenE2M(ctx, e2mUpdateResp.GetE2M())

	// Set state to fully populated data
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
	deleteReq := &e2m.DeleteE2MRequest{Id: wrapperspb.String(id)}
	log.Printf("[INFO] Deleting Events2metric %s\n", id)
	if _, err := r.client.DeleteEvents2Metric(ctx, deleteReq); err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Events2Metric",
			formatRpcErrors(err, deleteEvents2MetricURL, protojson.Format(deleteReq)),
		)
		return
	}
	log.Printf("[INFO] Events2metric %s deleted\n", id)
}

func (r *Events2MetricResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func flattenE2M(ctx context.Context, e2m *e2m.E2M) Events2MetricResourceModel {
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

func extractCreateE2M(ctx context.Context, plan Events2MetricResourceModel) (*e2m.CreateE2MRequest, diag.Diagnostics) {
	name := typeStringToWrapperspbString(plan.Name)
	description := typeStringToWrapperspbString(plan.Description)
	permutations := expandPermutations(plan.Permutations)
	permutationsLimit := wrapperspb.Int32(permutations.GetLimit())
	metricLabels, diags := expandE2MLabels(ctx, plan.MetricLabels)
	if diags.HasError() {
		return nil, diags
	}
	metricFields := expandE2MFields(ctx, plan.MetricFields)

	e2mParams := &e2m.E2MCreateParams{
		Name:              name,
		Description:       description,
		PermutationsLimit: permutationsLimit,
		MetricLabels:      metricLabels,
		MetricFields:      metricFields,
	}

	if spansQuery := plan.SpansQuery; spansQuery != nil {
		e2mParams.Type = e2m.E2MType_E2M_TYPE_SPANS2METRICS
		e2mParams.Query, diags = expandSpansQuery(ctx, spansQuery)
	} else if logsQuery := plan.LogsQuery; logsQuery != nil {
		e2mParams.Type = e2m.E2MType_E2M_TYPE_LOGS2METRICS
		e2mParams.Query, diags = expandLogsQuery(ctx, logsQuery)
	}

	if diags.HasError() {
		return nil, diags
	}

	return &e2m.CreateE2MRequest{
		E2M: e2mParams,
	}, nil
}

func expandPermutations(permutations *PermutationsModel) *e2m.E2MPermutations {
	if permutations == nil {
		return nil
	}
	return &e2m.E2MPermutations{
		Limit:            int32(permutations.Limit.ValueInt64()),
		HasExceededLimit: permutations.HasExceedLimit.ValueBool(),
	}
}

func extractUpdateE2M(ctx context.Context, plan Events2MetricResourceModel) (*e2m.ReplaceE2MRequest, diag.Diagnostics) {
	id := wrapperspb.String(plan.ID.ValueString())
	name := wrapperspb.String(plan.Name.ValueString())
	description := wrapperspb.String(plan.Description.ValueString())
	permutations := expandPermutations(plan.Permutations)
	metricLabels, diags := expandE2MLabels(ctx, plan.MetricLabels)
	if diags.HasError() {
		return nil, diags
	}
	metricFields := expandE2MFields(ctx, plan.MetricFields)

	e2mParams := &e2m.E2M{
		Id:           id,
		Name:         name,
		Description:  description,
		Permutations: permutations,
		MetricLabels: metricLabels,
		MetricFields: metricFields,
	}

	if spansQuery := plan.SpansQuery; spansQuery != nil {
		e2mParams.Type = e2m.E2MType_E2M_TYPE_SPANS2METRICS
		e2mParams.Query, diags = expandUpdateSpansQuery(ctx, spansQuery)
	} else if logsQuery := plan.LogsQuery; logsQuery != nil {
		e2mParams.Type = e2m.E2MType_E2M_TYPE_LOGS2METRICS
		e2mParams.Query, diags = expandUpdateLogsQuery(ctx, logsQuery)
	}

	if diags.HasError() {
		return nil, diags
	}

	return &e2m.ReplaceE2MRequest{
		E2M: e2mParams,
	}, nil
}

func expandE2MLabels(ctx context.Context, labels types.Map) ([]*e2m.MetricLabel, diag.Diagnostics) {
	labelsMap := labels.Elements()
	result := make([]*e2m.MetricLabel, 0, len(labelsMap))
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

func expandE2MLabel(targetLabel, sourceField string) *e2m.MetricLabel {
	return &e2m.MetricLabel{
		TargetLabel: wrapperspb.String(targetLabel),
		SourceField: wrapperspb.String(sourceField),
	}
}

func expandE2MFields(ctx context.Context, fields types.Map) []*e2m.MetricField {
	var fieldsMap map[string]MetricFieldModel
	d := fields.ElementsAs(ctx, &fieldsMap, true)
	if d != nil {
		panic(d)
	}
	result := make([]*e2m.MetricField, 0, len(fieldsMap))
	for sourceFiled, metricFieldValue := range fieldsMap {
		field := expandE2MField(sourceFiled, metricFieldValue)
		result = append(result, field)
	}

	return result
}

func expandE2MField(targetField string, metricField MetricFieldModel) *e2m.MetricField {
	return &e2m.MetricField{
		TargetBaseMetricName: wrapperspb.String(targetField),
		SourceField:          wrapperspb.String(metricField.SourceField.ValueString()),
		Aggregations:         expandE2MAggregations(metricField.Aggregations),
	}
}

func expandE2MAggregations(aggregationsModel *AggregationsModel) []*e2m.Aggregation {
	if aggregationsModel == nil {
		return nil
	}

	aggregations := make([]*e2m.Aggregation, 0)

	if min := aggregationsModel.Min; min != nil {
		aggregation := &e2m.Aggregation{AggType: e2m.Aggregation_AGG_TYPE_MIN, Enabled: min.Enable.ValueBool(), TargetMetricName: "min"}
		aggregations = append(aggregations, aggregation)
	}
	if max := aggregationsModel.Max; max != nil {
		aggregation := &e2m.Aggregation{AggType: e2m.Aggregation_AGG_TYPE_MAX, Enabled: max.Enable.ValueBool(), TargetMetricName: "max"}
		aggregations = append(aggregations, aggregation)

	}
	if count := aggregationsModel.Count; count != nil {
		aggregation := &e2m.Aggregation{AggType: e2m.Aggregation_AGG_TYPE_COUNT, Enabled: count.Enable.ValueBool(), TargetMetricName: "count"}
		aggregations = append(aggregations, aggregation)
	}
	if avg := aggregationsModel.AVG; avg != nil {
		aggregation := &e2m.Aggregation{AggType: e2m.Aggregation_AGG_TYPE_AVG, Enabled: avg.Enable.ValueBool(), TargetMetricName: "avg"}
		aggregations = append(aggregations, aggregation)

	}
	if sum := aggregationsModel.Sum; sum != nil {
		aggregation := &e2m.Aggregation{AggType: e2m.Aggregation_AGG_TYPE_SUM, Enabled: sum.Enable.ValueBool(), TargetMetricName: "sum"}
		aggregations = append(aggregations, aggregation)

	}
	if samples := aggregationsModel.Samples; samples != nil {
		samplesType := schemaToProtoAggregationSampleType[samples.Type.ValueString()]
		aggregation := &e2m.Aggregation{AggType: e2m.Aggregation_AGG_TYPE_SAMPLES, Enabled: samples.Enable.ValueBool(), TargetMetricName: "samples", AggMetadata: &e2m.Aggregation_Samples{Samples: &e2m.E2MAggSamples{SampleType: samplesType}}}
		aggregations = append(aggregations, aggregation)
	}
	if histogram := aggregationsModel.Histogram; histogram != nil {
		buckets := expandBuckets(histogram.Buckets)
		aggregation := &e2m.Aggregation{AggType: e2m.Aggregation_AGG_TYPE_HISTOGRAM, Enabled: histogram.Enable.ValueBool(), TargetMetricName: "histogram", AggMetadata: &e2m.Aggregation_Histogram{Histogram: &e2m.E2MAggHistogram{Buckets: buckets}}}
		aggregations = append(aggregations, aggregation)

	}

	return aggregations
}

func expandBuckets(buckets []types.Float64) []float32 {
	result := make([]float32, 0, len(buckets))
	for _, b := range buckets {
		result = append(result, float32(b.ValueFloat64()))
	}

	return result
}

func expandSpansQuery(ctx context.Context, spansQuery *SpansQueryModel) (*e2m.E2MCreateParams_SpansQuery, diag.Diagnostics) {
	lucene := typeStringToWrapperspbString(spansQuery.Lucene)
	applications, diags := typeStringSliceToWrappedStringSlice(ctx, spansQuery.Applications.Elements())
	if diags.HasError() {
		return nil, diags
	}
	subsystems, diags := typeStringSliceToWrappedStringSlice(ctx, spansQuery.Subsystems.Elements())
	if diags.HasError() {
		return nil, diags
	}
	actions, diags := typeStringSliceToWrappedStringSlice(ctx, spansQuery.Actions.Elements())
	if diags.HasError() {
		return nil, diags
	}
	services, diags := typeStringSliceToWrappedStringSlice(ctx, spansQuery.Services.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &e2m.E2MCreateParams_SpansQuery{
		SpansQuery: &e2m.SpansQuery{
			Lucene:                 lucene,
			ApplicationnameFilters: applications,
			SubsystemnameFilters:   subsystems,
			ActionFilters:          actions,
			ServiceFilters:         services,
		},
	}, nil
}

func expandLogsQuery(ctx context.Context, logsQuery *LogsQueryModel) (*e2m.E2MCreateParams_LogsQuery, diag.Diagnostics) {
	searchQuery := typeStringToWrapperspbString(logsQuery.Lucene)
	applications, diags := typeStringSliceToWrappedStringSlice(ctx, logsQuery.Applications.Elements())
	if diags.HasError() {
		return nil, diags
	}
	subsystems, diags := typeStringSliceToWrappedStringSlice(ctx, logsQuery.Subsystems.Elements())
	if diags.HasError() {
		return nil, diags
	}
	severities, diags := expandLogsQuerySeverities(ctx, logsQuery.Severities.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &e2m.E2MCreateParams_LogsQuery{
		LogsQuery: &l2m.LogsQuery{
			Lucene:                 searchQuery,
			ApplicationnameFilters: applications,
			SubsystemnameFilters:   subsystems,
			SeverityFilters:        severities,
		},
	}, nil
}

func expandUpdateSpansQuery(ctx context.Context, spansQuery *SpansQueryModel) (*e2m.E2M_SpansQuery, diag.Diagnostics) {
	lucene := typeStringToWrapperspbString(spansQuery.Lucene)
	applications, diags := typeStringSliceToWrappedStringSlice(ctx, spansQuery.Applications.Elements())
	if diags != nil {
		return nil, diags
	}
	subsystems, diags := typeStringSliceToWrappedStringSlice(ctx, spansQuery.Subsystems.Elements())
	if diags != nil {
		return nil, diags
	}
	actions, diags := typeStringSliceToWrappedStringSlice(ctx, spansQuery.Actions.Elements())
	if diags != nil {
		return nil, diags
	}
	services, diags := typeStringSliceToWrappedStringSlice(ctx, spansQuery.Services.Elements())
	if diags != nil {
		return nil, diags
	}

	return &e2m.E2M_SpansQuery{
		SpansQuery: &e2m.SpansQuery{
			Lucene:                 lucene,
			ApplicationnameFilters: applications,
			SubsystemnameFilters:   subsystems,
			ActionFilters:          actions,
			ServiceFilters:         services,
		},
	}, nil
}

func expandUpdateLogsQuery(ctx context.Context, logsQuery *LogsQueryModel) (*e2m.E2M_LogsQuery, diag.Diagnostics) {
	searchQuery := wrapperspb.String(logsQuery.Lucene.ValueString())
	applications, diags := typeStringSliceToWrappedStringSlice(ctx, logsQuery.Applications.Elements())
	if diags.HasError() {
		return nil, diags
	}
	subsystems, diags := typeStringSliceToWrappedStringSlice(ctx, logsQuery.Subsystems.Elements())
	if diags.HasError() {
		return nil, diags
	}
	severities, diags := expandLogsQuerySeverities(ctx, logsQuery.Severities.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &e2m.E2M_LogsQuery{
		LogsQuery: &l2m.LogsQuery{
			Lucene:                 searchQuery,
			ApplicationnameFilters: applications,
			SubsystemnameFilters:   subsystems,
			SeverityFilters:        severities,
		},
	}, nil
}

func expandLogsQuerySeverities(ctx context.Context, severities []attr.Value) ([]l2m.Severity, diag.Diagnostics) {
	result := make([]l2m.Severity, 0, len(severities))
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
		severity := l2m.Severity(l2m.Severity_value[str])
		result = append(result, severity)
	}

	if diags.HasError() {
		return nil, diags
	}

	return result, nil
}

func flattenE2MPermutations(permutations *e2m.E2MPermutations) *PermutationsModel {
	if permutations == nil {
		return nil
	}
	return &PermutationsModel{
		Limit:          types.Int64Value(int64(permutations.GetLimit())),
		HasExceedLimit: types.BoolValue(permutations.GetHasExceededLimit()),
	}
}

func flattenE2MMetricFields(ctx context.Context, fields []*e2m.MetricField) types.Map {
	if len(fields) == 0 {
		return types.MapNull(types.ObjectType{AttrTypes: metricFieldModelAttr()})
	}

	elements := make(map[string]attr.Value)
	for _, f := range fields {
		target, field := flattenE2MMetricField(f)
		element, _ := types.ObjectValueFrom(ctx, metricFieldModelAttr(), field)
		elements[target] = element
	}
	return types.MapValueMust(types.ObjectType{AttrTypes: metricFieldModelAttr()}, elements)
}

func flattenE2MMetricField(field *e2m.MetricField) (string, MetricFieldModel) {
	aggregations := flattenE2MAggregations(field.GetAggregations())
	return field.GetTargetBaseMetricName().GetValue(), MetricFieldModel{
		SourceField:  types.StringValue(field.GetSourceField().GetValue()),
		Aggregations: aggregations,
	}
}

func flattenE2MAggregations(aggregations []*e2m.Aggregation) *AggregationsModel {
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
			aggregationsSchema.Histogram = flattenE2MHistogramAggregation(aggregation)
		}
	}

	return &aggregationsSchema
}

func flattenE2MCommonAggregation(aggregation *e2m.Aggregation) *CommonAggregationModel {
	if aggregation == nil {
		return nil
	}

	return &CommonAggregationModel{
		Enable:           types.BoolValue(aggregation.GetEnabled()),
		TargetMetricName: types.StringValue(aggregation.GetTargetMetricName()),
	}
}

func flattenE2MSamplesAggregation(aggregation *e2m.Aggregation) *SamplesAggregationModel {
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

func flattenE2MHistogramAggregation(aggregation *e2m.Aggregation) *HistogramAggregationModel {
	if aggregation == nil {
		return nil
	}

	buckets := aggregation.GetHistogram().GetBuckets()
	bucketsModel := make([]types.Float64, 0, len(buckets))
	for _, bucket := range buckets {
		bucketsModel = append(bucketsModel, types.Float64Value(float64(bucket)))
	}

	return &HistogramAggregationModel{
		Enable:           types.BoolValue(aggregation.GetEnabled()),
		TargetMetricName: types.StringValue(aggregation.GetTargetMetricName()),
		Buckets:          bucketsModel,
	}
}

func flattenE2MMetricLabels(labels []*e2m.MetricLabel) types.Map {
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

func flattenSpansQuery(query *e2m.SpansQuery) *SpansQueryModel {
	if query == nil {
		return nil
	}
	return &SpansQueryModel{
		Lucene:       wrapperspbStringToTypeString(query.GetLucene()),
		Applications: wrappedStringSliceToTypeStringSet(query.GetApplicationnameFilters()),
		Subsystems:   wrappedStringSliceToTypeStringSet(query.GetSubsystemnameFilters()),
		Actions:      wrappedStringSliceToTypeStringSet(query.GetActionFilters()),
		Services:     wrappedStringSliceToTypeStringSet(query.GetServiceFilters()),
	}
}

func flattenLogsQuery(query *l2m.LogsQuery) *LogsQueryModel {
	if query == nil {
		return nil
	}
	return &LogsQueryModel{
		Lucene:       wrapperspbStringToTypeString(query.GetLucene()),
		Applications: wrappedStringSliceToTypeStringSet(query.GetApplicationnameFilters()),
		Subsystems:   wrappedStringSliceToTypeStringSet(query.GetSubsystemnameFilters()),
		Severities:   flattenLogQuerySeverities(query.GetSeverityFilters()),
	}
}

func flattenLogQuerySeverities(severities []l2m.Severity) types.Set {
	if len(severities) == 0 {
		return types.SetNull(types.StringType)
	}
	elements := make([]attr.Value, 0, len(severities))
	for _, v := range severities {
		severity := types.StringValue(l2m.Severity_name[int32(v)])
		elements = append(elements, severity)
	}
	return types.SetValueMust(types.StringType, elements)
}
