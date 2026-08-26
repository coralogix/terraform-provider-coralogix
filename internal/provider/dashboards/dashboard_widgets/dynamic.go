// Copyright 2025 Coralogix Ltd.
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

package dashboard_widgets

import (
	"context"
	"fmt"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var (
	dashboardSchemaToProtoMetricsEditorMode = map[string]dashboardservice.MetricsQueryEditorMode{
		utils.UNSPECIFIED: dashboardservice.METRICSQUERYEDITORMODE_METRICS_QUERY_EDITOR_MODE_UNSPECIFIED,
		"builder":         dashboardservice.METRICSQUERYEDITORMODE_METRICS_QUERY_EDITOR_MODE_BUILDER,
		"text":            dashboardservice.METRICSQUERYEDITORMODE_METRICS_QUERY_EDITOR_MODE_TEXT,
	}
	dashboardProtoToSchemaMetricsEditorMode = utils.ReverseMap(dashboardSchemaToProtoMetricsEditorMode)
	dashboardValidMetricsEditorModes        = utils.GetKeys(dashboardSchemaToProtoMetricsEditorMode)

	dashboardSchemaToProtoMetricsSeriesLimitType = map[string]dashboardservice.MetricsSeriesLimitType{
		utils.UNSPECIFIED: dashboardservice.METRICSSERIESLIMITTYPE_METRICS_SERIES_LIMIT_TYPE_UNSPECIFIED,
		"by_point_count":  dashboardservice.METRICSSERIESLIMITTYPE_METRICS_SERIES_LIMIT_TYPE_BY_POINT_COUNT,
		"by_series_count": dashboardservice.METRICSSERIESLIMITTYPE_METRICS_SERIES_LIMIT_TYPE_BY_SERIES_COUNT,
	}
	dashboardProtoToSchemaMetricsSeriesLimitType = utils.ReverseMap(dashboardSchemaToProtoMetricsSeriesLimitType)
	dashboardValidMetricsSeriesLimitTypes        = utils.GetKeys(dashboardSchemaToProtoMetricsSeriesLimitType)

	dashboardSchemaToProtoThresholdBy = map[string]dashboardservice.CommonThresholdBy{
		utils.UNSPECIFIED: dashboardservice.COMMONTHRESHOLDBY_THRESHOLD_BY_UNSPECIFIED,
		"value":           dashboardservice.COMMONTHRESHOLDBY_THRESHOLD_BY_VALUE,
		"background":      dashboardservice.COMMONTHRESHOLDBY_THRESHOLD_BY_BACKGROUND,
	}
	dashboardProtoToSchemaThresholdBy = utils.ReverseMap(dashboardSchemaToProtoThresholdBy)
	dashboardValidThresholdBy         = utils.GetKeys(dashboardSchemaToProtoThresholdBy)

	dashboardSchemaToProtoInterpretation = map[string]dashboardservice.Interpretation{
		utils.UNSPECIFIED:                      dashboardservice.INTERPRETATION_INTERPRETATION_UNSPECIFIED,
		"raw_data_table":                       dashboardservice.INTERPRETATION_INTERPRETATION_RAW_DATA_TABLE,
		"trend_over_time_line":                 dashboardservice.INTERPRETATION_INTERPRETATION_TREND_OVER_TIME_LINE,
		"single_value_kpi":                     dashboardservice.INTERPRETATION_INTERPRETATION_SINGLE_VALUE_KPI,
		"multi_value_kpi":                      dashboardservice.INTERPRETATION_INTERPRETATION_MULTI_VALUE_KPI,
		"categorical_analysis_vertical_bars":   dashboardservice.INTERPRETATION_INTERPRETATION_CATEGORICAL_ANALYSIS_VERTICAL_BARS,
		"single_value_kpi_stat":                dashboardservice.INTERPRETATION_INTERPRETATION_SINGLE_VALUE_KPI_STAT,
		"single_value_kpi_gauge":               dashboardservice.INTERPRETATION_INTERPRETATION_SINGLE_VALUE_KPI_GAUGE,
		"multi_value_kpi_stat":                 dashboardservice.INTERPRETATION_INTERPRETATION_MULTI_VALUE_KPI_STAT,
		"multi_value_kpi_gauge":                dashboardservice.INTERPRETATION_INTERPRETATION_MULTI_VALUE_KPI_GAUGE,
		"multi_value_kpi_hexagon_bins":         dashboardservice.INTERPRETATION_INTERPRETATION_MULTI_VALUE_KPI_HEXAGON_BINS,
		"categorical_analysis_pie_chart":       dashboardservice.INTERPRETATION_INTERPRETATION_CATEGORICAL_ANALYSIS_PIE_CHART,
		"categorical_analysis_horizontal_bars": dashboardservice.INTERPRETATION_INTERPRETATION_CATEGORICAL_ANALYSIS_HORIZONTAL_BARS,
		"single_value_kpi_stat_card":           dashboardservice.INTERPRETATION_INTERPRETATION_SINGLE_VALUE_KPI_STAT_CARD,
		"multi_value_kpi_stat_card":            dashboardservice.INTERPRETATION_INTERPRETATION_MULTI_VALUE_KPI_STAT_CARD,
	}
	dashboardProtoToSchemaInterpretation = utils.ReverseMap(dashboardSchemaToProtoInterpretation)
	dashboardValidInterpretation         = utils.GetKeys(dashboardSchemaToProtoInterpretation)

	dashboardSchemaToProtoSpanRelationType = map[string]dashboardservice.SpanRelationType{
		utils.UNSPECIFIED: dashboardservice.SPANRELATIONTYPE_SPAN_RELATION_TYPE_NONE_UNSPECIFIED,
		"other":           dashboardservice.SPANRELATIONTYPE_SPAN_RELATION_TYPE_OTHER,
		"parent":          dashboardservice.SPANRELATIONTYPE_SPAN_RELATION_TYPE_PARENT,
		"root":            dashboardservice.SPANRELATIONTYPE_SPAN_RELATION_TYPE_ROOT,
	}
	dashboardProtoToSchemaSpanRelationType = utils.ReverseMap(dashboardSchemaToProtoSpanRelationType)
	dashboardValidSpanRelationTypes        = utils.GetKeys(dashboardSchemaToProtoSpanRelationType)
)

func DynamicSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"query_definitions": schema.ListNestedAttribute{
				Required: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Optional: true,
							Computed: true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseNonNullStateForUnknown(),
							},
							MarkdownDescription: "Identifier for this query. Generated when omitted. Set it explicitly when a visualization needs to reference the query, as `time_series_lines_multi.query_display_settings[*].query_id` does.",
						},
						"name": schema.StringAttribute{
							Optional: true,
						},
						"query": schema.SingleNestedAttribute{
							Required: true,
							Attributes: map[string]schema.Attribute{
								"logs":       dynamicLogsQuerySchema(),
								"spans":      dynamicSpansQuerySchema(),
								"metrics":    dynamicMetricsQuerySchema(),
								"data_prime": dynamicDataPrimeQuerySchema(),
							},
							Validators: []validator.Object{
								ExactlyOneOfChildren("logs", "spans", "metrics", "data_prime"),
							},
						},
					},
				},
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
			},
			"interpretation": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(utils.UNSPECIFIED),
				DeprecationMessage:  "Deprecated: superseded by the visualization block.",
				MarkdownDescription: fmt.Sprintf("Deprecated: superseded by the `visualization` block. Retained at full fidelity for importing dashboards that still set it. Valid values are: %s.", strings.Join(dashboardValidInterpretation, ", ")),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidInterpretation...),
				},
			},
			"time_frame": TimeFrameSchema(),
			"visualization": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"stat":                    dynamicStatSchema(),
					"stat_card":               dynamicStatCardSchema(),
					"table":                   dynamicTableSchema(),
					"time_series_lines":       dynamicTimeSeriesLinesSchema(),
					"time_series_lines_multi": dynamicTimeSeriesLinesMultiSchema(),
					"time_series_bars":        dynamicTimeSeriesBarsSchema(),
					"vertical_bars":           dynamicVerticalBarsSchema(),
					"vertical_bars_multi":     dynamicVerticalBarsMultiSchema(),
					"horizontal_bars":         dynamicHorizontalBarsSchema(),
					"horizontal_bars_multi":   dynamicHorizontalBarsMultiSchema(),
					"gauge":                   dynamicGaugeSchema(),
					"pie_chart":               dynamicPieChartSchema(),
					"hexagon_bins":            dynamicHexagonBinsSchema(),
					"heatmap":                 dynamicHeatmapSchema(),
					"geomap":                  dynamicGeomapSchema(),
				},
				Validators: []validator.Object{
					ExactlyOneOfChildren("stat", "stat_card", "table", "time_series_lines", "time_series_lines_multi", "time_series_bars", "vertical_bars", "vertical_bars_multi", "horizontal_bars", "horizontal_bars_multi", "gauge", "pie_chart", "hexagon_bins", "heatmap", "geomap"),
				},
			},
		},
		Validators: []validator.Object{
			objectvalidator.AlsoRequires(
				path.MatchRelative().AtParent().AtParent().AtName("title"),
			),
		},
	}
}

func dynamicLogsQuerySchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"lucene_query": schema.StringAttribute{
				Optional: true,
			},
			"group_by": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: ObservationFieldSchema(),
				},
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
			},
			"aggregations":   dynamicAggregationsSchema(),
			"filters":        LogsFiltersSchema(),
			"data_mode_type": dynamicDataModeTypeSchema(),
		},
	}
}

func dynamicSpansQuerySchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"lucene_query": schema.StringAttribute{
				Optional: true,
			},
			"group_by": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: spanObservationFieldSchema(),
				},
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
			},
			"aggregations":   dynamicAggregationsSchema(),
			"filters":        SpansFilterSchema(),
			"data_mode_type": dynamicDataModeTypeSchema(),
		},
	}
}

func dynamicMetricsQuerySchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"promql_query": schema.StringAttribute{
				Required: true,
			},
			"promql_query_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidPromQLQueryType...),
				},
				MarkdownDescription: fmt.Sprintf("The PromQL query type. Use `range` for visualizations that plot values over time, such as the time-series ones; an instant query returns a single point and those charts render empty. Valid values are: %s.", strings.Join(DashboardValidPromQLQueryType, ", ")),
			},
			"editor_mode": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidMetricsEditorModes...),
				},
				MarkdownDescription: fmt.Sprintf("The metrics query editor mode. Valid values are: %s.", strings.Join(dashboardValidMetricsEditorModes, ", ")),
			},
			"series_limit_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidMetricsSeriesLimitTypes...),
				},
				MarkdownDescription: fmt.Sprintf("The metrics series limit type. Valid values are: %s.", strings.Join(dashboardValidMetricsSeriesLimitTypes, ", ")),
			},
		},
	}
}

func dynamicDataPrimeQuerySchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"query": schema.StringAttribute{
				Optional: true,
			},
			"data_mode_type": dynamicDataModeTypeSchema(),
		},
	}
}

func dynamicAggregationsSchema() schema.Attribute {
	return schema.ListNestedAttribute{
		Optional: true,
		Validators: []validator.List{
			listvalidator.SizeAtLeast(1),
		},
		NestedObject: schema.NestedAttributeObject{
			Attributes: LogsAggregationAttributes(),
			Validators: []validator.Object{
				logsAggregationValidator{},
			},
		},
	}
}

func dynamicDataModeTypeSchema() schema.Attribute {
	return schema.StringAttribute{
		Optional: true,
		Computed: true,
		Default:  stringdefault.StaticString(utils.UNSPECIFIED),
		Validators: []validator.String{
			stringvalidator.OneOf(DashboardValidDataModeTypes...),
		},
		MarkdownDescription: fmt.Sprintf("The data mode type. Valid values are: %s.", strings.Join(DashboardValidDataModeTypes, ", ")),
	}
}

func spanObservationFieldSchema() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"keypath": schema.ListAttribute{
			ElementType: types.StringType,
			Required:    true,
			Validators: []validator.List{
				listvalidator.SizeAtLeast(1),
			},
			MarkdownDescription: "Ordered path segments identifying the span field.",
		},
		"scope": schema.StringAttribute{
			Required: true,
			Validators: []validator.String{
				stringvalidator.OneOf(DashboardValidObservationFieldScope...),
			},
			MarkdownDescription: fmt.Sprintf("Where the field lives. Valid values are: %s.", strings.Join(DashboardValidObservationFieldScope, ", ")),
		},
		"relation_type": schema.StringAttribute{
			Optional: true,
			Computed: true,
			Default:  stringdefault.StaticString(utils.UNSPECIFIED),
			Validators: []validator.String{
				stringvalidator.OneOf(dashboardValidSpanRelationTypes...),
			},
			MarkdownDescription: fmt.Sprintf("The span relation type. Valid values are: %s.", strings.Join(dashboardValidSpanRelationTypes, ", ")),
		},
	}
}

func dynamicStatSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"allow_abbreviation": schema.BoolAttribute{
				Optional: true,
			},
			"category_fields": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: ObservationFieldSchema(),
				},
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
			},
			"custom_unit": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Custom unit label. Takes effect only when `unit` is `custom`.",
			},
			"decimal_precision": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.Between(0, 15),
				},
				MarkdownDescription: "How many digits to show after the decimal point. Valid values are 0 to 15.",
			},
			"display_series_name": schema.BoolAttribute{
				Optional: true,
			},
			"legend": LegendSchema(),
			"legend_by": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidLegendBys...),
				},
				MarkdownDescription: fmt.Sprintf("How the legend is grouped. Valid values are: %s.", strings.Join(DashboardValidLegendBys, ", ")),
			},
			"max": schema.Float64Attribute{
				Optional: true,
			},
			"min": schema.Float64Attribute{
				Optional: true,
			},
			"threshold_by": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidThresholdBy...),
				},
				MarkdownDescription: fmt.Sprintf("Which part of the widget the threshold colors. Valid values are: %s.", strings.Join(dashboardValidThresholdBy, ", ")),
			},
			"threshold_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidThresholdTypes...),
				},
				MarkdownDescription: fmt.Sprintf("The threshold type. Valid values are: %s.", strings.Join(DashboardValidThresholdTypes, ", ")),
			},
			"thresholds": dynamicThresholdsSchema(),
			"unit":       UnitSchema(),
			"value_field": schema.SingleNestedAttribute{
				Attributes:          ObservationFieldSchema(),
				Optional:            true,
				DeprecationMessage:  "Deprecated: superseded by value_fields.",
				MarkdownDescription: "Deprecated: superseded by `value_fields`, which holds the same observation fields. Retained at full fidelity so dashboards that still set it can be imported and updated without losing it.",
			},
			"value_fields": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: ObservationFieldSchema(),
				},
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
			},
		},
	}
}

func DynamicType() types.ObjectType {
	return types.ObjectType{
		AttrTypes: dynamicModelAttr(),
	}
}

func dynamicModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"query_definitions": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: dynamicQueryDefinitionModelAttr(),
			},
		},
		"interpretation": types.StringType,
		"time_frame": types.ObjectType{
			AttrTypes: TimeFrameModelAttr(),
		},
		"visualization": types.ObjectType{
			AttrTypes: dynamicVisualizationModelAttr(),
		},
	}
}

func dynamicQueryDefinitionModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"id":   types.StringType,
		"name": types.StringType,
		"query": types.ObjectType{
			AttrTypes: dynamicQueryModelAttr(),
		},
	}
}

func dynamicQueryModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs":       types.ObjectType{AttrTypes: dynamicLogsQueryAttr()},
		"spans":      types.ObjectType{AttrTypes: dynamicSpansQueryAttr()},
		"metrics":    types.ObjectType{AttrTypes: dynamicMetricsQueryAttr()},
		"data_prime": types.ObjectType{AttrTypes: dynamicDataPrimeQueryAttr()},
	}
}

func dynamicLogsQueryAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"lucene_query": types.StringType,
		"group_by": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"aggregations": types.ListType{
			ElemType: types.ObjectType{AttrTypes: AggregationModelAttr()},
		},
		"filters": types.ListType{
			ElemType: types.ObjectType{AttrTypes: LogsFilterModelAttr()},
		},
		"data_mode_type": types.StringType,
	}
}

func dynamicSpansQueryAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"lucene_query": types.StringType,
		"group_by": types.ListType{
			ElemType: types.ObjectType{AttrTypes: spanObservationFieldAttr()},
		},
		"aggregations": types.ListType{
			ElemType: types.ObjectType{AttrTypes: AggregationModelAttr()},
		},
		"filters": types.ListType{
			ElemType: types.ObjectType{AttrTypes: SpansFilterModelAttr()},
		},
		"data_mode_type": types.StringType,
	}
}

func dynamicMetricsQueryAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"promql_query":      types.StringType,
		"promql_query_type": types.StringType,
		"editor_mode":       types.StringType,
		"series_limit_type": types.StringType,
	}
}

func dynamicDataPrimeQueryAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"query":          types.StringType,
		"data_mode_type": types.StringType,
	}
}

func spanObservationFieldAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"keypath": types.ListType{
			ElemType: types.StringType,
		},
		"scope":         types.StringType,
		"relation_type": types.StringType,
	}
}

func dynamicVisualizationModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"stat":                    types.ObjectType{AttrTypes: dynamicStatModelAttr()},
		"stat_card":               types.ObjectType{AttrTypes: dynamicStatCardModelAttr()},
		"table":                   types.ObjectType{AttrTypes: dynamicTableModelAttr()},
		"time_series_lines":       types.ObjectType{AttrTypes: dynamicTimeSeriesLinesModelAttr()},
		"time_series_lines_multi": types.ObjectType{AttrTypes: dynamicTimeSeriesLinesMultiModelAttr()},
		"time_series_bars":        types.ObjectType{AttrTypes: dynamicTimeSeriesBarsModelAttr()},
		"vertical_bars":           types.ObjectType{AttrTypes: dynamicVerticalBarsModelAttr()},
		"vertical_bars_multi":     types.ObjectType{AttrTypes: dynamicVerticalBarsMultiModelAttr()},
		"horizontal_bars":         types.ObjectType{AttrTypes: dynamicHorizontalBarsModelAttr()},
		"horizontal_bars_multi":   types.ObjectType{AttrTypes: dynamicHorizontalBarsMultiModelAttr()},
		"gauge":                   types.ObjectType{AttrTypes: dynamicGaugeModelAttr()},
		"pie_chart":               types.ObjectType{AttrTypes: dynamicPieChartModelAttr()},
		"hexagon_bins":            types.ObjectType{AttrTypes: dynamicHexagonBinsModelAttr()},
		"heatmap":                 types.ObjectType{AttrTypes: dynamicHeatmapModelAttr()},
		"geomap":                  types.ObjectType{AttrTypes: dynamicGeomapModelAttr()},
	}
}

func dynamicStatModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"allow_abbreviation": types.BoolType,
		"category_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"custom_unit":         types.StringType,
		"decimal_precision":   types.Int64Type,
		"display_series_name": types.BoolType,
		"legend": types.ObjectType{
			AttrTypes: LegendAttr(),
		},
		"legend_by":      types.StringType,
		"max":            types.Float64Type,
		"min":            types.Float64Type,
		"threshold_by":   types.StringType,
		"threshold_type": types.StringType,
		"thresholds": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicThresholdAttr()},
		},
		"unit":        types.StringType,
		"value_field": ObservationFieldsObject(),
		"value_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
	}
}

func dynamicThresholdAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"from":  types.Float64Type,
		"color": types.StringType,
		"label": types.StringType,
	}
}

func ExpandDynamic(ctx context.Context, dynamic *DynamicModel) (*dashboardservice.WidgetDefinition, diag.Diagnostics) {
	if dynamic == nil {
		return nil, nil
	}

	queryDefinitions, diags := expandDynamicQueryDefinitions(ctx, dynamic.QueryDefinitions)
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := ExpandTimeFrameSelect(ctx, dynamic.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	visualization, diags := expandDynamicVisualization(ctx, dynamic.Visualization)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.WidgetDefinition{
		Dynamic: &dashboardservice.WidgetsDynamic{
			QueryDefinitions: queryDefinitions,
			Interpretation:   OptionalEnumPointer(dynamic.Interpretation, dashboardSchemaToProtoInterpretation),
			TimeFrame:        timeFrame,
			Visualization:    visualization,
		},
	}, nil
}

func expandDynamicQueryDefinitions(ctx context.Context, queryDefinitions types.List) ([]dashboardservice.DynamicQueryDefinition, diag.Diagnostics) {
	var queryDefinitionsObjects []types.Object
	var expandedQueryDefinitions []dashboardservice.DynamicQueryDefinition
	diags := queryDefinitions.ElementsAs(ctx, &queryDefinitionsObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, qdo := range queryDefinitionsObjects {
		var queryDefinition DynamicQueryDefinitionModel
		if dg := qdo.As(ctx, &queryDefinition, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedQueryDefinition, expandDiags := expandDynamicQueryDefinition(ctx, &queryDefinition)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedQueryDefinitions = append(expandedQueryDefinitions, *expandedQueryDefinition)
	}

	return expandedQueryDefinitions, diags
}

func expandDynamicQueryDefinition(ctx context.Context, queryDefinition *DynamicQueryDefinitionModel) (*dashboardservice.DynamicQueryDefinition, diag.Diagnostics) {
	if queryDefinition == nil {
		return nil, nil
	}

	query, diags := expandDynamicQuery(ctx, queryDefinition.Query)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.DynamicQueryDefinition{
		Id:    *ExpandDashboardIDs(queryDefinition.ID),
		Name:  utils.TypeStringToStringPointer(queryDefinition.Name),
		Query: query,
	}, nil
}

func expandDynamicQuery(ctx context.Context, query *DynamicQueryModel) (dashboardservice.DynamicQuery, diag.Diagnostics) {
	if query == nil {
		return dashboardservice.DynamicQuery{}, nil
	}

	switch {
	case query.Logs != nil:
		logs, diags := expandDynamicLogsQuery(ctx, query.Logs)
		if diags.HasError() {
			return dashboardservice.DynamicQuery{}, diags
		}
		return dashboardservice.DynamicQuery{Logs: logs}, nil
	case query.Spans != nil:
		spans, diags := expandDynamicSpansQuery(ctx, query.Spans)
		if diags.HasError() {
			return dashboardservice.DynamicQuery{}, diags
		}
		return dashboardservice.DynamicQuery{Spans: spans}, nil
	case query.Metrics != nil:
		metrics := expandDynamicMetricsQuery(query.Metrics)
		return dashboardservice.DynamicQuery{Metrics: metrics}, nil
	case query.DataPrime != nil:
		dataPrime := expandDynamicDataPrimeQuery(query.DataPrime)
		return dashboardservice.DynamicQuery{Dataprime: dataPrime}, nil
	default:
		return dashboardservice.DynamicQuery{}, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Dynamic Query", "unknown dynamic query type")}
	}
}

func expandDynamicLogsQuery(ctx context.Context, logs *DynamicQueryLogsModel) (*dashboardservice.Logs, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	groupBy, diags := ExpandObservationFields(ctx, logs.GroupBy)
	if diags.HasError() {
		return nil, diags
	}

	aggregations, diags := ExpandLogsAggregations(ctx, logs.Aggregations)
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := ExpandLogsFilters(ctx, logs.Filters)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.Logs{
		LuceneQuery:  ExpandLuceneQuery(logs.LuceneQuery),
		GroupBy:      groupBy,
		Aggregation:  aggregations,
		Filters:      filters,
		DataModeType: OptionalEnumPointer(logs.DataModeType, DashboardSchemaToProtoDataModeType),
	}, nil
}

func expandDynamicSpansQuery(ctx context.Context, spans *DynamicQuerySpansModel) (*dashboardservice.Spans, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	groupBy, diags := expandSpanObservationFields(ctx, spans.GroupBy)
	if diags.HasError() {
		return nil, diags
	}

	aggregations, diags := ExpandLogsAggregations(ctx, spans.Aggregations)
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := ExpandSpansFilters(ctx, spans.Filters)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.Spans{
		LuceneQuery:  ExpandLuceneQuery(spans.LuceneQuery),
		GroupBy:      groupBy,
		Aggregation:  aggregations,
		Filters:      filters,
		DataModeType: OptionalEnumPointer(spans.DataModeType, DashboardSchemaToProtoDataModeType),
	}, nil
}

func expandDynamicMetricsQuery(metrics *DynamicQueryMetricsModel) *dashboardservice.Metrics {
	if metrics == nil {
		return nil
	}

	return &dashboardservice.Metrics{
		PromqlQuery:     ExpandPromqlQuery(metrics.PromqlQuery),
		PromqlQueryType: OptionalEnumPointer(metrics.PromqlQueryType, DashboardSchemaToProtoPromQLQueryType),
		EditorMode:      OptionalEnumPointer(metrics.EditorMode, dashboardSchemaToProtoMetricsEditorMode),
		SeriesLimitType: OptionalEnumPointer(metrics.SeriesLimitType, dashboardSchemaToProtoMetricsSeriesLimitType),
	}
}

func expandDynamicDataPrimeQuery(dataPrime *DynamicQueryDataPrimeModel) *dashboardservice.Dataprime {
	if dataPrime == nil {
		return nil
	}

	return &dashboardservice.Dataprime{
		DataprimeQuery: &dashboardservice.CommonDataprimeQuery{
			Text: dataPrime.Query.ValueStringPointer(),
		},
		DataModeType: OptionalEnumPointer(dataPrime.DataModeType, DashboardSchemaToProtoDataModeType),
	}
}

func expandSpanObservationFields(ctx context.Context, groupBy types.List) ([]dashboardservice.SpanObservationField, diag.Diagnostics) {
	var objects []types.Object
	diags := groupBy.ElementsAs(ctx, &objects, true)
	if diags.HasError() {
		return nil, diags
	}

	var fields []dashboardservice.SpanObservationField
	for _, obj := range objects {
		var model SpanObservationFieldModel
		if dg := obj.As(ctx, &model, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		keypath, dg := typeStringListToStringSlice(ctx, model.Keypath)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		fields = append(fields, dashboardservice.SpanObservationField{
			Keypath:      keypath,
			Scope:        OptionalEnumPointer(model.Scope, DashboardSchemaToProtoObservationFieldScope),
			RelationType: OptionalEnumPointer(model.RelationType, dashboardSchemaToProtoSpanRelationType),
		})
	}

	return fields, diags
}

func expandDynamicVisualization(ctx context.Context, visualization *DynamicVisualizationModel) (*dashboardservice.Visualization, diag.Diagnostics) {
	if visualization == nil {
		return nil, nil
	}

	// ExactlyOneOfChildren defers while any arm is unknown, so a value only
	// known after apply can arrive with two visualizations set, or with none.
	// The dispatch below would take the first of two and discard the rest, or
	// return no visualization at all, which the read then flattens to null.
	switch selected := dynamicSelectedVisualizations(visualization); {
	case len(selected) > 1:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic(
			"Invalid Attribute Combination",
			fmt.Sprintf("Only one visualization can be configured, but %s are set.", strings.Join(selected, " and ")),
		)}
	case len(selected) == 0:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic(
			"Invalid Attribute Combination",
			"A visualization block must configure exactly one visualization. Remove the block to leave the widget without one.",
		)}
	}

	// One helper per visualization family, so adding a family stays a one-line
	// change here instead of growing a switch past the cyclomatic limit.
	for _, family := range []func(context.Context, *DynamicVisualizationModel) (*dashboardservice.Visualization, diag.Diagnostics){
		expandDynamicVisualizationFamilyStat, expandDynamicVisualizationFamilyTable, expandDynamicVisualizationFamilyTimeSeries, expandDynamicVisualizationFamilyBars, expandDynamicVisualizationFamilyGaugePie, expandDynamicVisualizationFamilySpatial,
	} {
		converted, diags := family(ctx, visualization)
		if diags.HasError() {
			return nil, diags
		}
		if converted != nil {
			return converted, nil
		}
	}

	return nil, nil
}

type dynamicVisualizationArm struct {
	name     string
	selected bool
}

// Listed explicitly rather than discovered by reflection, so every field is
// referenced at compile time and a pointer added to the model for anything
// other than a visualization cannot be mistaken for one.
// TestDynamicVisualizationArmsMatchTheSchema keeps this list and the schema
// from drifting apart.
func dynamicVisualizationArms(visualization *DynamicVisualizationModel) []dynamicVisualizationArm {
	return []dynamicVisualizationArm{
		{"stat", visualization.Stat != nil},
		{"stat_card", visualization.StatCard != nil},
		{"table", visualization.Table != nil},
		{"time_series_lines", visualization.TimeSeriesLines != nil},
		{"time_series_lines_multi", visualization.TimeSeriesLinesMulti != nil},
		{"time_series_bars", visualization.TimeSeriesBars != nil},
		{"vertical_bars", visualization.VerticalBars != nil},
		{"vertical_bars_multi", visualization.VerticalBarsMulti != nil},
		{"horizontal_bars", visualization.HorizontalBars != nil},
		{"horizontal_bars_multi", visualization.HorizontalBarsMulti != nil},
		{"gauge", visualization.Gauge != nil},
		{"pie_chart", visualization.PieChart != nil},
		{"hexagon_bins", visualization.HexagonBins != nil},
		{"heatmap", visualization.Heatmap != nil},
		{"geomap", visualization.Geomap != nil},
	}
}

func dynamicSelectedVisualizations(visualization *DynamicVisualizationModel) []string {
	var selected []string
	for _, arm := range dynamicVisualizationArms(visualization) {
		if arm.selected {
			selected = append(selected, arm.name)
		}
	}
	return selected
}

// The object and conflict validators defer while any child is unknown, so a
// value only known after apply can reach these conversions having selected no
// arm or two. Neither shape has an API representation, so each union re-checks
// here rather than letting the apply fail on the backend's answer.
func dynamicUnionDiagnostic(attribute, requirement string) diag.Diagnostics {
	return diag.Diagnostics{diag.NewErrorDiagnostic(
		"Invalid Attribute Combination",
		fmt.Sprintf("%s requires exactly one of %s.", attribute, requirement),
	)}
}

func expandDynamicVisualizationFamilyStat(ctx context.Context, visualization *DynamicVisualizationModel) (*dashboardservice.Visualization, diag.Diagnostics) {
	switch {
	case visualization.Stat != nil:
		stat, diags := expandDynamicStat(ctx, visualization.Stat)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.Visualization{Stat: stat}, nil
	case visualization.StatCard != nil:
		statCard, diags := expandDynamicStatCard(ctx, visualization.StatCard)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.Visualization{StatCard: statCard}, nil
	}

	return nil, nil
}

func expandDynamicVisualizationFamilyTable(ctx context.Context, visualization *DynamicVisualizationModel) (*dashboardservice.Visualization, diag.Diagnostics) {
	switch {
	case visualization.Table != nil:
		table, diags := expandDynamicTable(ctx, visualization.Table)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.Visualization{Table: table}, nil
	}

	return nil, nil
}

func expandDynamicVisualizationFamilyTimeSeries(ctx context.Context, visualization *DynamicVisualizationModel) (*dashboardservice.Visualization, diag.Diagnostics) {
	switch {
	case visualization.TimeSeriesLines != nil:
		lines, diags := expandDynamicTimeSeriesLines(ctx, visualization.TimeSeriesLines)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.Visualization{TimeSeriesLines: lines}, nil
	case visualization.TimeSeriesLinesMulti != nil:
		linesMulti, diags := expandDynamicTimeSeriesLinesMulti(ctx, visualization.TimeSeriesLinesMulti)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.Visualization{TimeSeriesLinesMulti: linesMulti}, nil
	case visualization.TimeSeriesBars != nil:
		bars, diags := expandDynamicTimeSeriesBars(ctx, visualization.TimeSeriesBars)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.Visualization{TimeSeriesBars: bars}, nil
	}

	return nil, nil
}

func expandDynamicVisualizationFamilyBars(ctx context.Context, visualization *DynamicVisualizationModel) (*dashboardservice.Visualization, diag.Diagnostics) {
	switch {
	case visualization.VerticalBars != nil:
		bars, diags := expandDynamicVerticalBars(ctx, visualization.VerticalBars)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.Visualization{VerticalBars: bars}, nil
	case visualization.VerticalBarsMulti != nil:
		bars, diags := expandDynamicVerticalBarsMulti(ctx, visualization.VerticalBarsMulti)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.Visualization{VerticalBarsMulti: bars}, nil
	case visualization.HorizontalBars != nil:
		bars, diags := expandDynamicHorizontalBars(ctx, visualization.HorizontalBars)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.Visualization{HorizontalBars: bars}, nil
	case visualization.HorizontalBarsMulti != nil:
		bars, diags := expandDynamicHorizontalBarsMulti(ctx, visualization.HorizontalBarsMulti)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.Visualization{HorizontalBarsMulti: bars}, nil
	}

	return nil, nil
}

func expandDynamicVisualizationFamilyGaugePie(ctx context.Context, visualization *DynamicVisualizationModel) (*dashboardservice.Visualization, diag.Diagnostics) {
	switch {
	case visualization.Gauge != nil:
		gauge, diags := expandDynamicGauge(ctx, visualization.Gauge)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.Visualization{Gauge: gauge}, nil
	case visualization.PieChart != nil:
		pieChart, diags := expandDynamicPieChart(ctx, visualization.PieChart)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.Visualization{PieChart: pieChart}, nil
	}

	return nil, nil
}

func expandDynamicVisualizationFamilySpatial(ctx context.Context, visualization *DynamicVisualizationModel) (*dashboardservice.Visualization, diag.Diagnostics) {
	switch {
	case visualization.HexagonBins != nil:
		hexagonBins, diags := expandDynamicHexagonBins(ctx, visualization.HexagonBins)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.Visualization{HexagonBins: hexagonBins}, nil
	case visualization.Heatmap != nil:
		heatmap, diags := expandDynamicHeatmap(ctx, visualization.Heatmap)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.Visualization{Heatmap: heatmap}, nil
	case visualization.Geomap != nil:
		geomap, diags := expandDynamicGeomap(ctx, visualization.Geomap)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.Visualization{Geomap: geomap}, nil
	}

	return nil, nil
}

func expandDynamicStat(ctx context.Context, stat *DynamicStatModel) (*dashboardservice.Stat, diag.Diagnostics) {
	if stat == nil {
		return nil, nil
	}

	valueField, diags := ExpandObservationFieldObject(ctx, stat.ValueField)
	if diags.HasError() {
		return nil, diags
	}

	categoryFields, diags := ExpandObservationFields(ctx, stat.CategoryFields)
	if diags.HasError() {
		return nil, diags
	}

	valueFields, diags := ExpandObservationFields(ctx, stat.ValueFields)
	if diags.HasError() {
		return nil, diags
	}

	thresholds, diags := expandDynamicThresholds(ctx, stat.Thresholds)
	if diags.HasError() {
		return nil, diags
	}

	legend, diags := ExpandLegend(ctx, stat.Legend)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.Stat{
		AllowAbbreviation: stat.AllowAbbreviation.ValueBoolPointer(),
		CategoryFields:    categoryFields,
		CustomUnit:        stat.CustomUnit.ValueStringPointer(),
		DecimalPrecision:  expandInt32Pointer(stat.DecimalPrecision),
		DisplaySeriesName: stat.DisplaySeriesName.ValueBoolPointer(),
		Legend:            legend,
		LegendBy:          OptionalEnumPointer(stat.LegendBy, DashboardSchemaToProtoLegendBy),
		Max:               stat.Max.ValueFloat64Pointer(),
		Min:               stat.Min.ValueFloat64Pointer(),
		ThresholdBy:       OptionalEnumPointer(stat.ThresholdBy, dashboardSchemaToProtoThresholdBy),
		ThresholdType:     OptionalEnumPointer(stat.ThresholdType, DashboardSchemaToProtoThresholdType),
		Thresholds:        thresholds,
		Unit:              OptionalEnumPointer(stat.Unit, DashboardSchemaToProtoUnit),
		ValueField:        valueField,
		ValueFields:       valueFields,
	}, nil
}

func expandInt32Pointer(value types.Int64) *int32 {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	converted := int32(value.ValueInt64())
	return &converted
}

func expandDynamicThresholds(ctx context.Context, thresholds types.List) ([]dashboardservice.CommonThreshold, diag.Diagnostics) {
	var models []DynamicThresholdModel
	diags := thresholds.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	expanded := make([]dashboardservice.CommonThreshold, 0, len(models))
	for _, model := range models {
		expanded = append(expanded, dashboardservice.CommonThreshold{
			From:  model.From.ValueFloat64Pointer(),
			Color: model.Color.ValueStringPointer(),
			Label: model.Label.ValueStringPointer(),
		})
	}

	return expanded, diags
}

func FlattenDynamic(ctx context.Context, dynamic *dashboardservice.WidgetsDynamic) (*WidgetDefinitionModel, diag.Diagnostics) {
	if dynamic == nil {
		return nil, nil
	}

	// The deprecated top-level query has no typed equivalent, and the API stores
	// and returns it verbatim rather than migrating it to query_definitions.
	// Flattening such a widget would drop its only data source, so refuse the
	// read instead of writing state that silently loses it.
	if dynamic.Query != nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic(
			"Unsupported Dashboard Widget Definition",
			"The dynamic widget uses the deprecated top-level `query`, which this provider cannot represent as typed HCL. Manage this dashboard with `content_json`, or move the widget to `query_definitions`, which holds the same queries.",
		)}
	}

	queryDefinitions, diags := flattenDynamicQueryDefinitions(ctx, dynamic.GetQueryDefinitions())
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := FlattenTimeFrameSelect(ctx, dynamic.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	visualization, diags := flattenDynamicVisualization(ctx, dynamic.Visualization)
	if diags.HasError() {
		return nil, diags
	}

	return &WidgetDefinitionModel{
		Dynamic: &DynamicModel{
			QueryDefinitions: queryDefinitions,
			Interpretation:   flattenOptionalEnum(dynamic.Interpretation, dashboardProtoToSchemaInterpretation),
			TimeFrame:        timeFrame,
			Visualization:    visualization,
		},
	}, nil
}

func flattenDynamicQueryDefinitions(ctx context.Context, definitions []dashboardservice.DynamicQueryDefinition) (types.List, diag.Diagnostics) {
	if len(definitions) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicQueryDefinitionModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	definitionsElements := make([]attr.Value, 0, len(definitions))
	for i := range definitions {
		flattenedDefinition, diags := flattenDynamicQueryDefinition(ctx, &definitions[i])
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		definitionElement, diags := types.ObjectValueFrom(ctx, dynamicQueryDefinitionModelAttr(), flattenedDefinition)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		definitionsElements = append(definitionsElements, definitionElement)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicQueryDefinitionModelAttr()}), diagnostics
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicQueryDefinitionModelAttr()}, definitionsElements)
}

func flattenDynamicQueryDefinition(ctx context.Context, definition *dashboardservice.DynamicQueryDefinition) (*DynamicQueryDefinitionModel, diag.Diagnostics) {
	if definition == nil {
		return nil, nil
	}

	query, diags := flattenDynamicQuery(ctx, &definition.Query)
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicQueryDefinitionModel{
		ID:    types.StringValue(definition.GetId()),
		Name:  utils.StringPointerToTypeString(definition.Name),
		Query: query,
	}, nil
}

func flattenDynamicQuery(ctx context.Context, query *dashboardservice.DynamicQuery) (*DynamicQueryModel, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}

	switch {
	case query.Logs != nil:
		return flattenDynamicLogsQuery(ctx, query.Logs)
	case query.Spans != nil:
		return flattenDynamicSpansQuery(ctx, query.Spans)
	case query.Metrics != nil:
		return flattenDynamicMetricsQuery(query.Metrics), nil
	case query.Dataprime != nil:
		return flattenDynamicDataPrimeQuery(query.Dataprime), nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dynamic Query", "unknown dynamic query type")}
	}
}

func flattenDynamicLogsQuery(ctx context.Context, logs *dashboardservice.Logs) (*DynamicQueryModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	groupBy, diags := FlattenObservationFields(ctx, logs.GetGroupBy())
	if diags.HasError() {
		return nil, diags
	}

	aggregations, diags := flattenAggregations(ctx, logs.GetAggregation())
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := FlattenLogsFilters(ctx, logs.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicQueryModel{
		Logs: &DynamicQueryLogsModel{
			LuceneQuery:  flattenLuceneQuery(logs.LuceneQuery),
			GroupBy:      groupBy,
			Aggregations: aggregations,
			Filters:      filters,
			DataModeType: flattenOptionalEnum(logs.DataModeType, DashboardProtoToSchemaDataModeType),
		},
	}, nil
}

func flattenDynamicSpansQuery(ctx context.Context, spans *dashboardservice.Spans) (*DynamicQueryModel, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	groupBy, diags := flattenSpanObservationFields(ctx, spans.GetGroupBy())
	if diags.HasError() {
		return nil, diags
	}

	aggregations, diags := flattenAggregations(ctx, spans.GetAggregation())
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := FlattenSpansFilters(ctx, spans.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicQueryModel{
		Spans: &DynamicQuerySpansModel{
			LuceneQuery:  flattenLuceneQuery(spans.LuceneQuery),
			GroupBy:      groupBy,
			Aggregations: aggregations,
			Filters:      filters,
			DataModeType: flattenOptionalEnum(spans.DataModeType, DashboardProtoToSchemaDataModeType),
		},
	}, nil
}

func flattenDynamicMetricsQuery(metrics *dashboardservice.Metrics) *DynamicQueryModel {
	if metrics == nil {
		return nil
	}

	return &DynamicQueryModel{
		Metrics: &DynamicQueryMetricsModel{
			PromqlQuery:     flattenPromqlQuery(metrics.PromqlQuery),
			PromqlQueryType: flattenOptionalEnum(metrics.PromqlQueryType, DashboardProtoToSchemaPromQLQueryType),
			EditorMode:      flattenOptionalEnum(metrics.EditorMode, dashboardProtoToSchemaMetricsEditorMode),
			SeriesLimitType: flattenOptionalEnum(metrics.SeriesLimitType, dashboardProtoToSchemaMetricsSeriesLimitType),
		},
	}
}

func flattenDynamicDataPrimeQuery(dataPrime *dashboardservice.Dataprime) *DynamicQueryModel {
	if dataPrime == nil {
		return nil
	}

	query := types.StringNull()
	if dataPrime.DataprimeQuery != nil && dataPrime.DataprimeQuery.Text != nil {
		query = types.StringPointerValue(dataPrime.DataprimeQuery.Text)
	}

	return &DynamicQueryModel{
		DataPrime: &DynamicQueryDataPrimeModel{
			Query:        query,
			DataModeType: flattenOptionalEnum(dataPrime.DataModeType, DashboardProtoToSchemaDataModeType),
		},
	}
}

func flattenSpanObservationFields(ctx context.Context, fields []dashboardservice.SpanObservationField) (types.List, diag.Diagnostics) {
	if len(fields) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: spanObservationFieldAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	fieldElements := make([]attr.Value, 0, len(fields))
	for i := range fields {
		model := &SpanObservationFieldModel{
			Keypath:      utils.StringSliceToTypeStringList(fields[i].GetKeypath()),
			Scope:        flattenOptionalEnum(fields[i].Scope, DashboardProtoToSchemaObservationFieldScope),
			RelationType: flattenOptionalEnum(fields[i].RelationType, dashboardProtoToSchemaSpanRelationType),
		}
		fieldElement, diags := types.ObjectValueFrom(ctx, spanObservationFieldAttr(), model)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		fieldElements = append(fieldElements, fieldElement)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: spanObservationFieldAttr()}), diagnostics
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: spanObservationFieldAttr()}, fieldElements)
}

func flattenDynamicVisualization(ctx context.Context, visualization *dashboardservice.Visualization) (*DynamicVisualizationModel, diag.Diagnostics) {
	if visualization == nil {
		return nil, nil
	}

	// One helper per visualization family, so adding a family stays a one-line
	// change here instead of growing a switch past the cyclomatic limit.
	for _, family := range []func(context.Context, *dashboardservice.Visualization) (*DynamicVisualizationModel, diag.Diagnostics){
		flattenDynamicVisualizationFamilyStat, flattenDynamicVisualizationFamilyTable, flattenDynamicVisualizationFamilyTimeSeries, flattenDynamicVisualizationFamilyBars, flattenDynamicVisualizationFamilyGaugePie, flattenDynamicVisualizationFamilySpatial,
	} {
		converted, diags := family(ctx, visualization)
		if diags.HasError() {
			return nil, diags
		}
		if converted != nil {
			return converted, nil
		}
	}

	return nil, diag.Diagnostics{diag.NewErrorDiagnostic(
		"Unsupported Dashboard Widget Definition",
		"The dynamic widget uses a visualization variant this provider version cannot represent as typed HCL. Manage this dashboard with `content_json` until that visualization is supported.",
	)}
}

func flattenDynamicVisualizationFamilyStat(ctx context.Context, visualization *dashboardservice.Visualization) (*DynamicVisualizationModel, diag.Diagnostics) {
	switch {
	case visualization.Stat != nil:
		stat, diags := flattenDynamicStat(ctx, visualization.Stat)
		if diags.HasError() {
			return nil, diags
		}
		return &DynamicVisualizationModel{Stat: stat}, nil
	case visualization.StatCard != nil:
		statCard, diags := flattenDynamicStatCard(ctx, visualization.StatCard)
		if diags.HasError() {
			return nil, diags
		}
		return &DynamicVisualizationModel{StatCard: statCard}, nil
	}

	return nil, nil
}

func flattenDynamicVisualizationFamilyTable(ctx context.Context, visualization *dashboardservice.Visualization) (*DynamicVisualizationModel, diag.Diagnostics) {
	switch {
	case visualization.Table != nil:
		table, diags := flattenDynamicTable(ctx, visualization.Table)
		if diags.HasError() {
			return nil, diags
		}
		return &DynamicVisualizationModel{Table: table}, nil
	}

	return nil, nil
}

func flattenDynamicVisualizationFamilyTimeSeries(ctx context.Context, visualization *dashboardservice.Visualization) (*DynamicVisualizationModel, diag.Diagnostics) {
	switch {
	case visualization.TimeSeriesLines != nil:
		lines, diags := flattenDynamicTimeSeriesLines(ctx, visualization.TimeSeriesLines)
		if diags.HasError() {
			return nil, diags
		}
		return &DynamicVisualizationModel{TimeSeriesLines: lines}, nil
	case visualization.TimeSeriesLinesMulti != nil:
		linesMulti, diags := flattenDynamicTimeSeriesLinesMulti(ctx, visualization.TimeSeriesLinesMulti)
		if diags.HasError() {
			return nil, diags
		}
		return &DynamicVisualizationModel{TimeSeriesLinesMulti: linesMulti}, nil
	case visualization.TimeSeriesBars != nil:
		bars, diags := flattenDynamicTimeSeriesBars(ctx, visualization.TimeSeriesBars)
		if diags.HasError() {
			return nil, diags
		}
		return &DynamicVisualizationModel{TimeSeriesBars: bars}, nil
	}

	return nil, nil
}

func flattenDynamicVisualizationFamilyBars(ctx context.Context, visualization *dashboardservice.Visualization) (*DynamicVisualizationModel, diag.Diagnostics) {
	switch {
	case visualization.VerticalBars != nil:
		bars, diags := flattenDynamicVerticalBars(ctx, visualization.VerticalBars)
		if diags.HasError() {
			return nil, diags
		}
		return &DynamicVisualizationModel{VerticalBars: bars}, nil
	case visualization.VerticalBarsMulti != nil:
		bars, diags := flattenDynamicVerticalBarsMulti(ctx, visualization.VerticalBarsMulti)
		if diags.HasError() {
			return nil, diags
		}
		return &DynamicVisualizationModel{VerticalBarsMulti: bars}, nil
	case visualization.HorizontalBars != nil:
		bars, diags := flattenDynamicHorizontalBars(ctx, visualization.HorizontalBars)
		if diags.HasError() {
			return nil, diags
		}
		return &DynamicVisualizationModel{HorizontalBars: bars}, nil
	case visualization.HorizontalBarsMulti != nil:
		bars, diags := flattenDynamicHorizontalBarsMulti(ctx, visualization.HorizontalBarsMulti)
		if diags.HasError() {
			return nil, diags
		}
		return &DynamicVisualizationModel{HorizontalBarsMulti: bars}, nil
	}

	return nil, nil
}

func flattenDynamicVisualizationFamilyGaugePie(ctx context.Context, visualization *dashboardservice.Visualization) (*DynamicVisualizationModel, diag.Diagnostics) {
	switch {
	case visualization.Gauge != nil:
		gauge, diags := flattenDynamicGauge(ctx, visualization.Gauge)
		if diags.HasError() {
			return nil, diags
		}
		return &DynamicVisualizationModel{Gauge: gauge}, nil
	case visualization.PieChart != nil:
		pieChart, diags := flattenDynamicPieChart(ctx, visualization.PieChart)
		if diags.HasError() {
			return nil, diags
		}
		return &DynamicVisualizationModel{PieChart: pieChart}, nil
	}

	return nil, nil
}

func flattenDynamicVisualizationFamilySpatial(ctx context.Context, visualization *dashboardservice.Visualization) (*DynamicVisualizationModel, diag.Diagnostics) {
	switch {
	case visualization.HexagonBins != nil:
		hexagonBins, diags := flattenDynamicHexagonBins(ctx, visualization.HexagonBins)
		if diags.HasError() {
			return nil, diags
		}
		return &DynamicVisualizationModel{HexagonBins: hexagonBins}, nil
	case visualization.Heatmap != nil:
		heatmap, diags := flattenDynamicHeatmap(ctx, visualization.Heatmap)
		if diags.HasError() {
			return nil, diags
		}
		return &DynamicVisualizationModel{Heatmap: heatmap}, nil
	case visualization.Geomap != nil:
		geomap, diags := flattenDynamicGeomap(ctx, visualization.Geomap)
		if diags.HasError() {
			return nil, diags
		}
		return &DynamicVisualizationModel{Geomap: geomap}, nil
	}

	return nil, nil
}

func flattenDynamicStat(ctx context.Context, stat *dashboardservice.Stat) (*DynamicStatModel, diag.Diagnostics) {
	if stat == nil {
		return nil, nil
	}

	valueField, diags := FlattenObservationField(ctx, stat.ValueField)
	if diags.HasError() {
		return nil, diags
	}

	categoryFields, diags := FlattenObservationFields(ctx, stat.GetCategoryFields())
	if diags.HasError() {
		return nil, diags
	}

	valueFields, diags := FlattenObservationFields(ctx, stat.GetValueFields())
	if diags.HasError() {
		return nil, diags
	}

	thresholds, diags := flattenDynamicThresholds(ctx, stat.GetThresholds())
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicStatModel{
		AllowAbbreviation: types.BoolPointerValue(stat.AllowAbbreviation),
		CategoryFields:    categoryFields,
		CustomUnit:        types.StringPointerValue(stat.CustomUnit),
		DecimalPrecision:  flattenInt32Pointer(stat.DecimalPrecision),
		DisplaySeriesName: types.BoolPointerValue(stat.DisplaySeriesName),
		Legend:            FlattenLegend(stat.Legend),
		LegendBy:          flattenOptionalEnum(stat.LegendBy, DashboardProtoToSchemaLegendBy),
		Max:               types.Float64PointerValue(stat.Max),
		Min:               types.Float64PointerValue(stat.Min),
		ThresholdBy:       flattenOptionalEnum(stat.ThresholdBy, dashboardProtoToSchemaThresholdBy),
		ThresholdType:     flattenOptionalEnum(stat.ThresholdType, DashboardProtoToSchemaThresholdType),
		Thresholds:        thresholds,
		Unit:              flattenOptionalEnum(stat.Unit, DashboardProtoToSchemaUnit),
		ValueField:        valueField,
		ValueFields:       valueFields,
	}, nil
}

func flattenInt32Pointer(value *int32) types.Int64 {
	if value == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*value))
}

func flattenDynamicThresholds(ctx context.Context, thresholds []dashboardservice.CommonThreshold) (types.List, diag.Diagnostics) {
	if len(thresholds) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicThresholdAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	thresholdElements := make([]attr.Value, 0, len(thresholds))
	for i := range thresholds {
		model := &DynamicThresholdModel{
			From:  types.Float64PointerValue(thresholds[i].From),
			Color: types.StringPointerValue(thresholds[i].Color),
			Label: types.StringPointerValue(thresholds[i].Label),
		}
		thresholdElement, diags := types.ObjectValueFrom(ctx, dynamicThresholdAttr(), model)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		thresholdElements = append(thresholdElements, thresholdElement)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicThresholdAttr()}), diagnostics
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicThresholdAttr()}, thresholdElements)
}

func flattenOptionalEnum[T ~string](value *T, mapping map[T]string) types.String {
	if value == nil {
		return types.StringNull()
	}
	if mapped, ok := mapping[*value]; ok {
		return types.StringValue(mapped)
	}
	return types.StringNull()
}
