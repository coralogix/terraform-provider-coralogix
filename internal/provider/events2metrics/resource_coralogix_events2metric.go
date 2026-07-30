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
	"net/http"
	"regexp"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	e2ms "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/events2metrics_service"

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
)

var (
	severitySchemaToAPI = map[string]e2ms.Logs2metricsV2Severity{
		"Unspecified": e2ms.LOGS2METRICSV2SEVERITY_SEVERITY_UNSPECIFIED,
		"Debug":       e2ms.LOGS2METRICSV2SEVERITY_SEVERITY_DEBUG,
		"Verbose":     e2ms.LOGS2METRICSV2SEVERITY_SEVERITY_VERBOSE,
		"Info":        e2ms.LOGS2METRICSV2SEVERITY_SEVERITY_INFO,
		"Warning":     e2ms.LOGS2METRICSV2SEVERITY_SEVERITY_WARNING,
		"Error":       e2ms.LOGS2METRICSV2SEVERITY_SEVERITY_ERROR,
		"Critical":    e2ms.LOGS2METRICSV2SEVERITY_SEVERITY_CRITICAL,
	}
	severityAPIToSchema = utils.ReverseMap(severitySchemaToAPI)
	validSeverities     = utils.GetKeys(severitySchemaToAPI)

	apiToSchemaAggregationType = map[e2ms.AggType]string{
		e2ms.AGGTYPE_AGG_TYPE_MIN:       "min",
		e2ms.AGGTYPE_AGG_TYPE_MAX:       "max",
		e2ms.AGGTYPE_AGG_TYPE_COUNT:     "count",
		e2ms.AGGTYPE_AGG_TYPE_AVG:       "avg",
		e2ms.AGGTYPE_AGG_TYPE_SUM:       "sum",
		e2ms.AGGTYPE_AGG_TYPE_HISTOGRAM: "histogram",
		e2ms.AGGTYPE_AGG_TYPE_SAMPLES:   "samples",
	}
	schemaToAPIAggregationSampleType = map[string]e2ms.SampleType{
		"Min": e2ms.SAMPLETYPE_SAMPLE_TYPE_MIN,
		"Max": e2ms.SAMPLETYPE_SAMPLE_TYPE_MAX,
	}

	apiToSchemaAggregationSampleType = utils.ReverseMap(schemaToAPIAggregationSampleType)

	validSampleTypes = utils.GetKeys(schemaToAPIAggregationSampleType)
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
	client *e2ms.Events2MetricsServiceAPIService
}

func ptr[T any](v T) *T {
	return &v
}

func nilIfEmpty[T any](s []T) []T {
	if len(s) == 0 {
		return nil
	}
	return s
}

func responseStatus(response *http.Response) int {
	if response == nil {
		return 0
	}
	return response.StatusCode
}

type Events2MetricResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	MetricFields types.Map    `tfsdk:"metric_fields"`
	MetricLabels types.Map    `tfsdk:"metric_labels"`
	// types.Object (not *PermutationsModel): Optional+Computed becomes unknown on
	// create when omitted; Go struct pointers cannot hold unknown values.
	Permutations types.Object     `tfsdk:"permutations"`
	SpansQuery   *SpansQueryModel `tfsdk:"spans_query"`
	LogsQuery    *LogsQueryModel  `tfsdk:"logs_query"`
	DataSource   types.String     `tfsdk:"data_source"`
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

func permutationsModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"limit":            types.Int64Type,
		"has_exceed_limit": types.BoolType,
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
			"data_source": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Data source in `<namespace>/<dataset_name>` format. If not set, defaults to the standard logs/spans stream.",
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
						Computed:    true,
						Description: "The search_query that we wanted to be notified on.",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
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
						Computed:    true,
						Description: "The search_query that we wanted to be notified on.",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
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
		MarkdownDescription: "Coralogix Events2Metrics. Converts logs and spans into metrics for long-term monitoring. For more info please review - https://coralogix.com/docs/user-guides/monitoring-and-insights/events2metrics/.",
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

func upgradeE2MPermutationsV0ToV1(ctx context.Context, permutations types.List) types.Object {
	var permutationsObjects []types.Object
	permutations.ElementsAs(ctx, &permutationsObjects, true)
	if len(permutationsObjects) == 0 {
		return types.ObjectNull(permutationsModelAttr())
	}
	return permutationsObjects[0]
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
	var plan Events2MetricResourceModel
	diags := req.Plan.Get(ctx, &plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	params, diags := extractCreateE2M(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	createResp, httpResponse, err := r.client.Events2MetricServiceCreateE2M(ctx).E2MCreateParams(params).Execute()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating Events2Metric",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", params),
		)
		return
	}
	if createResp == nil || createResp.E2m == nil {
		resp.Diagnostics.AddError(
			"Error creating Events2Metric",
			"Create response did not include an Events2Metric",
		)
		return
	}

	plan, diags = flattenE2M(ctx, createResp.E2m)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *Events2MetricResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Events2MetricResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	getResp, httpResponse, err := r.client.Events2MetricServiceGetE2M(ctx, id).Execute()
	if err != nil {
		if responseStatus(httpResponse) == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Events2Metric %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading Events2Metric",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
		)
		return
	}

	state, diags = flattenE2M(ctx, &getResp.E2m)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *Events2MetricResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan Events2MetricResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	e2m, diags := extractUpdateE2M(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	replaceResp, httpResponse, err := r.client.Events2MetricServiceReplaceE2M(ctx).E2M1(e2m).Execute()
	if err != nil {
		if responseStatus(httpResponse) == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Events2Metric %q is in state, but no longer exists in Coralogix backend", plan.ID.ValueString()),
				fmt.Sprintf("%s will be recreated when you apply", plan.ID.ValueString()),
			)
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error updating Events2Metric",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Replace", e2m),
		)
		return
	}

	plan, diags = flattenE2M(ctx, &replaceResp.E2m)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *Events2MetricResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Events2MetricResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	_, httpResponse, err := r.client.Events2MetricServiceDeleteE2M(ctx, id).Execute()
	if err != nil {
		if responseStatus(httpResponse) == http.StatusNotFound {
			return
		}
		resp.Diagnostics.AddError(
			"Error Deleting Events2Metric",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Delete", nil),
		)
		return
	}
}

func (r *Events2MetricResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func flattenE2M(ctx context.Context, e2m *e2ms.E2M) (Events2MetricResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	metricFields, d := flattenE2MMetricFields(ctx, e2m.GetMetricFields())
	diags.Append(d...)

	perms, permsOk := e2m.GetPermutationsOk()
	permutations, d := flattenE2MPermutations(ctx, perms, permsOk)
	diags.Append(d...)

	model := Events2MetricResourceModel{
		ID:           types.StringValue(e2m.GetId()),
		Name:         types.StringValue(e2m.GetName()),
		Description:  utils.StringPointerToTypeString(e2m.Description),
		MetricFields: metricFields,
		MetricLabels: flattenE2MMetricLabels(e2m.GetMetricLabels()),
		Permutations: permutations,
		DataSource:   utils.StringPointerToTypeString(e2m.DataSource),
	}

	if spansQuery, ok := e2m.GetSpansQueryOk(); ok {
		model.SpansQuery = flattenSpansQuery(spansQuery)
	}
	if logsQuery, ok := e2m.GetLogsQueryOk(); ok {
		model.LogsQuery = flattenLogsQuery(logsQuery)
	}

	return model, diags
}

func extractCreateE2M(ctx context.Context, plan Events2MetricResourceModel) (e2ms.E2MCreateParams, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Preserve gRPC create behavior: absent permutations still sends explicit 0
	// (wrapperspb.Int32(nil.GetLimit())). Nil would change wire shape.
	permutationsLimit := int32(0)
	if !plan.Permutations.IsNull() && !plan.Permutations.IsUnknown() {
		var permutations PermutationsModel
		diags.Append(plan.Permutations.As(ctx, &permutations, basetypes.ObjectAsOptions{})...)
		if !permutations.Limit.IsNull() && !permutations.Limit.IsUnknown() {
			permutationsLimit = int32(permutations.Limit.ValueInt64())
		}
	}

	metricLabels, d := expandE2MLabels(ctx, plan.MetricLabels)
	diags.Append(d...)
	metricFields, d := expandE2MFields(ctx, plan.MetricFields)
	diags.Append(d...)
	if diags.HasError() {
		return e2ms.E2MCreateParams{}, diags
	}

	params := e2ms.E2MCreateParams{
		Name:              plan.Name.ValueString(),
		Description:       utils.TypeStringToStringPointer(plan.Description),
		DataSource:        utils.TypeStringToStringPointer(plan.DataSource),
		PermutationsLimit: ptr(permutationsLimit),
		MetricLabels:      metricLabels,
		MetricFields:      metricFields,
	}

	if spansQuery := plan.SpansQuery; spansQuery != nil {
		params.Type = e2ms.E2MTYPE_E2_M_TYPE_SPANS2_METRICS.Ptr()
		query, d := expandCreateSpansQuery(ctx, spansQuery)
		diags.Append(d...)
		params.SpansQuery = query
	} else if logsQuery := plan.LogsQuery; logsQuery != nil {
		params.Type = e2ms.E2MTYPE_E2_M_TYPE_LOGS2_METRICS.Ptr()
		query, d := expandCreateLogsQuery(ctx, logsQuery)
		diags.Append(d...)
		params.LogsQuery = query
	}

	if diags.HasError() {
		return e2ms.E2MCreateParams{}, diags
	}

	return params, nil
}

func extractUpdateE2M(ctx context.Context, plan Events2MetricResourceModel) (e2ms.E2M1, diag.Diagnostics) {
	var diags diag.Diagnostics

	metricLabels, d := expandE2MLabels(ctx, plan.MetricLabels)
	diags.Append(d...)
	metricFields, d := expandE2MFields(ctx, plan.MetricFields)
	diags.Append(d...)
	if diags.HasError() {
		return e2ms.E2M1{}, diags
	}

	permutations, d := expandPermutations(ctx, plan.Permutations)
	diags.Append(d...)
	if diags.HasError() {
		return e2ms.E2M1{}, diags
	}

	e2m := e2ms.E2M1{
		Id:   ptr(plan.ID.ValueString()),
		Name: plan.Name.ValueString(),
		// Intentional create/update asymmetry (follow-up): update sends "" when description is null.
		Description:  ptr(plan.Description.ValueString()),
		DataSource:   utils.TypeStringToStringPointer(plan.DataSource),
		Permutations: permutations,
		MetricLabels: metricLabels,
		MetricFields: metricFields,
	}

	if spansQuery := plan.SpansQuery; spansQuery != nil {
		e2m.Type = e2ms.E2MTYPE_E2_M_TYPE_SPANS2_METRICS
		query, d := expandUpdateSpansQuery(ctx, spansQuery)
		diags.Append(d...)
		e2m.SpansQuery = query
	} else if logsQuery := plan.LogsQuery; logsQuery != nil {
		e2m.Type = e2ms.E2MTYPE_E2_M_TYPE_LOGS2_METRICS
		query, d := expandUpdateLogsQuery(ctx, logsQuery)
		diags.Append(d...)
		e2m.LogsQuery = query
	}

	if diags.HasError() {
		return e2ms.E2M1{}, diags
	}

	return e2m, nil
}

func expandPermutations(ctx context.Context, permutations types.Object) (*e2ms.E2MPermutations, diag.Diagnostics) {
	if permutations.IsNull() || permutations.IsUnknown() {
		return nil, nil
	}
	var model PermutationsModel
	diags := permutations.As(ctx, &model, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}
	return &e2ms.E2MPermutations{
		Limit:            int32(model.Limit.ValueInt64()),
		HasExceededLimit: model.HasExceedLimit.ValueBool(),
	}, nil
}

func expandE2MLabels(ctx context.Context, labels types.Map) ([]e2ms.MetricLabel, diag.Diagnostics) {
	labelsMap := labels.Elements()
	result := make([]e2ms.MetricLabel, 0, len(labelsMap))
	var diags diag.Diagnostics
	for targetField, value := range labelsMap {
		v, err := value.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("error expanding metric labels", err.Error())
			continue
		}
		var sourceField string
		if err := v.As(&sourceField); err != nil {
			diags.AddError("error expanding metric labels", err.Error())
			continue
		}
		result = append(result, e2ms.MetricLabel{
			TargetLabel: targetField,
			SourceField: sourceField,
		})
	}
	if diags.HasError() {
		return nil, diags
	}

	return nilIfEmpty(result), nil
}

func expandE2MFields(ctx context.Context, fields types.Map) ([]e2ms.V2MetricField, diag.Diagnostics) {
	var fieldsMap map[string]MetricFieldModel
	var diags diag.Diagnostics
	diags.Append(fields.ElementsAs(ctx, &fieldsMap, true)...)
	if diags.HasError() {
		return nil, diags
	}

	result := make([]e2ms.V2MetricField, 0, len(fieldsMap))
	for targetField, metricFieldValue := range fieldsMap {
		field, d := expandE2MField(ctx, targetField, metricFieldValue)
		if d.HasError() {
			diags.Append(d...)
			continue
		}
		result = append(result, field)
	}
	if diags.HasError() {
		return nil, diags
	}

	return nilIfEmpty(result), diags
}

func expandE2MField(ctx context.Context, targetField string, metricField MetricFieldModel) (e2ms.V2MetricField, diag.Diagnostics) {
	aggregations, diags := expandE2MAggregations(ctx, metricField.Aggregations)
	if diags.HasError() {
		return e2ms.V2MetricField{}, diags
	}

	return e2ms.V2MetricField{
		TargetBaseMetricName: targetField,
		SourceField:          metricField.SourceField.ValueString(),
		Aggregations:         aggregations,
	}, nil
}

func expandE2MAggregations(ctx context.Context, aggregationsModel *AggregationsModel) ([]e2ms.V2Aggregation, diag.Diagnostics) {
	// OpenAPI requires the aggregations key; always emit a non-nil slice.
	if aggregationsModel == nil {
		return []e2ms.V2Aggregation{}, nil
	}

	aggregations := make([]e2ms.V2Aggregation, 0)

	if min := aggregationsModel.Min; min != nil {
		aggregations = append(aggregations, e2ms.V2Aggregation{
			AggType:          e2ms.AGGTYPE_AGG_TYPE_MIN.Ptr(),
			Enabled:          ptr(min.Enable.ValueBool()),
			TargetMetricName: ptr("min"),
		})
	}
	if max := aggregationsModel.Max; max != nil {
		aggregations = append(aggregations, e2ms.V2Aggregation{
			AggType:          e2ms.AGGTYPE_AGG_TYPE_MAX.Ptr(),
			Enabled:          ptr(max.Enable.ValueBool()),
			TargetMetricName: ptr("max"),
		})
	}
	if count := aggregationsModel.Count; count != nil {
		aggregations = append(aggregations, e2ms.V2Aggregation{
			AggType:          e2ms.AGGTYPE_AGG_TYPE_COUNT.Ptr(),
			Enabled:          ptr(count.Enable.ValueBool()),
			TargetMetricName: ptr("count"),
		})
	}
	if avg := aggregationsModel.AVG; avg != nil {
		aggregations = append(aggregations, e2ms.V2Aggregation{
			AggType:          e2ms.AGGTYPE_AGG_TYPE_AVG.Ptr(),
			Enabled:          ptr(avg.Enable.ValueBool()),
			TargetMetricName: ptr("avg"),
		})
	}
	if sum := aggregationsModel.Sum; sum != nil {
		aggregations = append(aggregations, e2ms.V2Aggregation{
			AggType:          e2ms.AGGTYPE_AGG_TYPE_SUM.Ptr(),
			Enabled:          ptr(sum.Enable.ValueBool()),
			TargetMetricName: ptr("sum"),
		})
	}
	if samples := aggregationsModel.Samples; samples != nil {
		sampleType := schemaToAPIAggregationSampleType[samples.Type.ValueString()]
		aggregations = append(aggregations, e2ms.V2Aggregation{
			AggType:          e2ms.AGGTYPE_AGG_TYPE_SAMPLES.Ptr(),
			Enabled:          ptr(samples.Enable.ValueBool()),
			TargetMetricName: ptr("samples"),
			Samples: &e2ms.E2MAggSamples{
				SampleType: sampleType.Ptr(),
			},
		})
	}
	if histogram := aggregationsModel.Histogram; histogram != nil {
		buckets, diags := utils.AttrSliceToFloat32Slice(ctx, histogram.Buckets.Elements())
		if diags.HasError() {
			return nil, diags
		}
		aggregations = append(aggregations, e2ms.V2Aggregation{
			AggType:          e2ms.AGGTYPE_AGG_TYPE_HISTOGRAM.Ptr(),
			Enabled:          ptr(histogram.Enable.ValueBool()),
			TargetMetricName: ptr("histogram"),
			Histogram: &e2ms.E2MAggHistogram{
				Buckets: buckets,
			},
		})
	}

	return aggregations, nil
}

func expandCreateSpansQuery(ctx context.Context, spansQuery *SpansQueryModel) (*e2ms.V2SpansQuery, diag.Diagnostics) {
	applications, diags := utils.TypeStringElementsToStringSlice(ctx, spansQuery.Applications.Elements())
	if diags.HasError() {
		return nil, diags
	}
	subsystems, d := utils.TypeStringElementsToStringSlice(ctx, spansQuery.Subsystems.Elements())
	diags.Append(d...)
	actions, d := utils.TypeStringElementsToStringSlice(ctx, spansQuery.Actions.Elements())
	diags.Append(d...)
	services, d := utils.TypeStringElementsToStringSlice(ctx, spansQuery.Services.Elements())
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}

	return &e2ms.V2SpansQuery{
		Lucene:                 utils.TypeStringToStringPointer(spansQuery.Lucene),
		ApplicationnameFilters: nilIfEmpty(applications),
		SubsystemnameFilters:   nilIfEmpty(subsystems),
		ActionFilters:          nilIfEmpty(actions),
		ServiceFilters:         nilIfEmpty(services),
	}, nil
}

func expandCreateLogsQuery(ctx context.Context, logsQuery *LogsQueryModel) (*e2ms.V2LogsQuery, diag.Diagnostics) {
	applications, diags := utils.TypeStringElementsToStringSlice(ctx, logsQuery.Applications.Elements())
	if diags.HasError() {
		return nil, diags
	}
	subsystems, d := utils.TypeStringElementsToStringSlice(ctx, logsQuery.Subsystems.Elements())
	diags.Append(d...)
	severities, d := expandLogsQuerySeverities(ctx, logsQuery.Severities.Elements())
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}

	return &e2ms.V2LogsQuery{
		Lucene:                 utils.TypeStringToStringPointer(logsQuery.Lucene),
		ApplicationnameFilters: nilIfEmpty(applications),
		SubsystemnameFilters:   nilIfEmpty(subsystems),
		SeverityFilters:        nilIfEmpty(severities),
	}, nil
}

func expandUpdateSpansQuery(ctx context.Context, spansQuery *SpansQueryModel) (*e2ms.V2SpansQuery, diag.Diagnostics) {
	applications, diags := utils.TypeStringElementsToStringSlice(ctx, spansQuery.Applications.Elements())
	if diags.HasError() {
		return nil, diags
	}
	subsystems, d := utils.TypeStringElementsToStringSlice(ctx, spansQuery.Subsystems.Elements())
	diags.Append(d...)
	actions, d := utils.TypeStringElementsToStringSlice(ctx, spansQuery.Actions.Elements())
	diags.Append(d...)
	services, d := utils.TypeStringElementsToStringSlice(ctx, spansQuery.Services.Elements())
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}

	return &e2ms.V2SpansQuery{
		Lucene:                 utils.TypeStringToStringPointer(spansQuery.Lucene),
		ApplicationnameFilters: nilIfEmpty(applications),
		SubsystemnameFilters:   nilIfEmpty(subsystems),
		ActionFilters:          nilIfEmpty(actions),
		ServiceFilters:         nilIfEmpty(services),
	}, nil
}

func expandUpdateLogsQuery(ctx context.Context, logsQuery *LogsQueryModel) (*e2ms.V2LogsQuery, diag.Diagnostics) {
	applications, diags := utils.TypeStringElementsToStringSlice(ctx, logsQuery.Applications.Elements())
	if diags.HasError() {
		return nil, diags
	}
	subsystems, d := utils.TypeStringElementsToStringSlice(ctx, logsQuery.Subsystems.Elements())
	diags.Append(d...)
	severities, d := expandLogsQuerySeverities(ctx, logsQuery.Severities.Elements())
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}

	return &e2ms.V2LogsQuery{
		// Intentional create/update asymmetry (follow-up): update sends "" when lucene is null.
		Lucene:                 ptr(logsQuery.Lucene.ValueString()),
		ApplicationnameFilters: nilIfEmpty(applications),
		SubsystemnameFilters:   nilIfEmpty(subsystems),
		SeverityFilters:        nilIfEmpty(severities),
	}, nil
}

func expandLogsQuerySeverities(ctx context.Context, severities []attr.Value) ([]e2ms.Logs2metricsV2Severity, diag.Diagnostics) {
	result := make([]e2ms.Logs2metricsV2Severity, 0, len(severities))
	var diags diag.Diagnostics
	for _, s := range severities {
		v, err := s.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("error expanding logs query severities", err.Error())
			continue
		}
		var str string
		if err = v.As(&str); err != nil {
			diags.AddError("error expanding logs query severities", err.Error())
			continue
		}
		result = append(result, severitySchemaToAPI[str])
	}

	if diags.HasError() {
		return nil, diags
	}

	return result, nil
}

func flattenE2MPermutations(ctx context.Context, permutations *e2ms.E2MPermutations, ok bool) (types.Object, diag.Diagnostics) {
	if !ok || permutations == nil {
		return types.ObjectNull(permutationsModelAttr()), nil
	}
	return types.ObjectValueFrom(ctx, permutationsModelAttr(), PermutationsModel{
		Limit:          types.Int64Value(int64(permutations.GetLimit())),
		HasExceedLimit: types.BoolValue(permutations.GetHasExceededLimit()),
	})
}

func flattenE2MMetricFields(ctx context.Context, fields []e2ms.V2MetricField) (types.Map, diag.Diagnostics) {
	if len(fields) == 0 {
		return types.MapNull(types.ObjectType{AttrTypes: metricFieldModelAttr()}), nil
	}

	var diags diag.Diagnostics
	elements := make(map[string]attr.Value)
	for _, f := range fields {
		target, field, d := flattenE2MMetricField(ctx, f)
		diags.Append(d...)
		if d.HasError() {
			continue
		}
		element, d := types.ObjectValueFrom(ctx, metricFieldModelAttr(), field)
		diags.Append(d...)
		elements[target] = element
	}
	if diags.HasError() {
		return types.MapNull(types.ObjectType{AttrTypes: metricFieldModelAttr()}), diags
	}
	return types.MapValueMust(types.ObjectType{AttrTypes: metricFieldModelAttr()}, elements), diags
}

func flattenE2MMetricField(ctx context.Context, field e2ms.V2MetricField) (string, MetricFieldModel, diag.Diagnostics) {
	aggregations, diags := flattenE2MAggregations(ctx, field.GetAggregations())
	return field.GetTargetBaseMetricName(), MetricFieldModel{
		SourceField:  types.StringValue(field.GetSourceField()),
		Aggregations: aggregations,
	}, diags
}

func flattenE2MAggregations(ctx context.Context, aggregations []e2ms.V2Aggregation) (*AggregationsModel, diag.Diagnostics) {
	aggregationsSchema := AggregationsModel{}
	var diags diag.Diagnostics

	for _, aggregation := range aggregations {
		aggType, ok := aggregation.GetAggTypeOk()
		if !ok || aggType == nil {
			// Match pre-REST behavior: skip unknown/absent aggregation types.
			continue
		}
		aggTypeStr, ok := apiToSchemaAggregationType[*aggType]
		if !ok {
			continue
		}
		switch aggTypeStr {
		case "min":
			aggregationsSchema.Min = flattenE2MCommonAggregation(&aggregation)
		case "max":
			aggregationsSchema.Max = flattenE2MCommonAggregation(&aggregation)
		case "avg":
			aggregationsSchema.AVG = flattenE2MCommonAggregation(&aggregation)
		case "sum":
			aggregationsSchema.Sum = flattenE2MCommonAggregation(&aggregation)
		case "count":
			aggregationsSchema.Count = flattenE2MCommonAggregation(&aggregation)
		case "samples":
			aggregationsSchema.Samples = flattenE2MSamplesAggregation(&aggregation)
		case "histogram":
			histogram, d := flattenE2MHistogramAggregation(ctx, &aggregation)
			diags.Append(d...)
			aggregationsSchema.Histogram = histogram
		}
	}

	return &aggregationsSchema, diags
}

func flattenE2MCommonAggregation(aggregation *e2ms.V2Aggregation) *CommonAggregationModel {
	if aggregation == nil {
		return nil
	}

	return &CommonAggregationModel{
		Enable:           types.BoolValue(aggregation.GetEnabled()),
		TargetMetricName: types.StringValue(aggregation.GetTargetMetricName()),
	}
}

func flattenE2MSamplesAggregation(aggregation *e2ms.V2Aggregation) *SamplesAggregationModel {
	if aggregation == nil {
		return nil
	}

	samplesType := ""
	if samples, ok := aggregation.GetSamplesOk(); ok && samples != nil {
		if sampleType, ok := samples.GetSampleTypeOk(); ok && sampleType != nil {
			samplesType = apiToSchemaAggregationSampleType[*sampleType]
		}
	}
	return &SamplesAggregationModel{
		Enable:           types.BoolValue(aggregation.GetEnabled()),
		TargetMetricName: types.StringValue(aggregation.GetTargetMetricName()),
		Type:             types.StringValue(samplesType),
	}
}

func flattenE2MHistogramAggregation(ctx context.Context, aggregation *e2ms.V2Aggregation) (*HistogramAggregationModel, diag.Diagnostics) {
	if aggregation == nil {
		return nil, nil
	}

	var bucketsList []float32
	if histogram, ok := aggregation.GetHistogramOk(); ok && histogram != nil {
		bucketsList = histogram.GetBuckets()
	}
	buckets, diags := utils.Float32SliceTypeList(ctx, bucketsList)
	if diags.HasError() {
		return nil, diags
	}
	return &HistogramAggregationModel{
		Enable:           types.BoolValue(aggregation.GetEnabled()),
		TargetMetricName: types.StringValue(aggregation.GetTargetMetricName()),
		Buckets:          buckets,
	}, nil
}

func flattenE2MMetricLabels(labels []e2ms.MetricLabel) types.Map {
	if len(labels) == 0 {
		return types.MapNull(types.StringType)
	}

	elements := make(map[string]attr.Value)
	for _, l := range labels {
		elements[l.GetTargetLabel()] = types.StringValue(l.GetSourceField())
	}

	return types.MapValueMust(types.StringType, elements)
}

func flattenSpansQuery(query *e2ms.V2SpansQuery) *SpansQueryModel {
	if query == nil {
		return nil
	}
	return &SpansQueryModel{
		Lucene:       utils.StringPointerToTypeString(query.Lucene),
		Applications: utils.StringSliceToTypeStringSet(query.GetApplicationnameFilters()),
		Subsystems:   utils.StringSliceToTypeStringSet(query.GetSubsystemnameFilters()),
		Actions:      utils.StringSliceToTypeStringSet(query.GetActionFilters()),
		Services:     utils.StringSliceToTypeStringSet(query.GetServiceFilters()),
	}
}

func flattenLogsQuery(query *e2ms.V2LogsQuery) *LogsQueryModel {
	if query == nil {
		return nil
	}
	return &LogsQueryModel{
		Lucene:       utils.StringPointerToTypeString(query.Lucene),
		Applications: utils.StringSliceToTypeStringSet(query.GetApplicationnameFilters()),
		Subsystems:   utils.StringSliceToTypeStringSet(query.GetSubsystemnameFilters()),
		Severities:   flattenLogQuerySeverities(query.GetSeverityFilters()),
	}
}

func flattenLogQuerySeverities(severities []e2ms.Logs2metricsV2Severity) types.Set {
	if len(severities) == 0 {
		return types.SetNull(types.StringType)
	}
	elements := make([]attr.Value, 0, len(severities))
	for _, v := range severities {
		elements = append(elements, types.StringValue(severityAPIToSchema[v]))
	}
	return types.SetValueMust(types.StringType, elements)
}
