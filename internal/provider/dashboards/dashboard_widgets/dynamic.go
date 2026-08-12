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
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"

	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
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

	dashboardSchemaToProtoSpanRelationType = map[string]dashboardservice.SpanRelationType{
		utils.UNSPECIFIED: dashboardservice.SPANRELATIONTYPE_SPAN_RELATION_TYPE_NONE_UNSPECIFIED,
		"other":           dashboardservice.SPANRELATIONTYPE_SPAN_RELATION_TYPE_OTHER,
		"parent":          dashboardservice.SPANRELATIONTYPE_SPAN_RELATION_TYPE_PARENT,
		"root":            dashboardservice.SPANRELATIONTYPE_SPAN_RELATION_TYPE_ROOT,
	}
	dashboardProtoToSchemaSpanRelationType = utils.ReverseMap(dashboardSchemaToProtoSpanRelationType)
	dashboardValidSpanRelationTypes        = utils.GetKeys(dashboardSchemaToProtoSpanRelationType)

	dashboardSchemaToProtoStackedLine = map[string]dashboardservice.VisualizationStackedLine{
		utils.UNSPECIFIED: dashboardservice.VISUALIZATIONSTACKEDLINE_STACKED_LINE_UNSPECIFIED,
		"absolute":        dashboardservice.VISUALIZATIONSTACKEDLINE_STACKED_LINE_ABSOLUTE,
		"relative":        dashboardservice.VISUALIZATIONSTACKEDLINE_STACKED_LINE_RELATIVE,
	}
	dashboardProtoToSchemaStackedLine = utils.ReverseMap(dashboardSchemaToProtoStackedLine)
	dashboardValidStackedLine         = utils.GetKeys(dashboardSchemaToProtoStackedLine)

	dashboardSchemaToProtoXAxisTimeFormat = map[string]dashboardservice.XAxisTimeFormat{
		utils.UNSPECIFIED: dashboardservice.XAXISTIMEFORMAT_X_AXIS_TIME_FORMAT_UNSPECIFIED,
		"auto":            dashboardservice.XAXISTIMEFORMAT_X_AXIS_TIME_FORMAT_AUTO,
		"dd_mm":           dashboardservice.XAXISTIMEFORMAT_X_AXIS_TIME_FORMAT_DD_MM,
		"mm_dd":           dashboardservice.XAXISTIMEFORMAT_X_AXIS_TIME_FORMAT_MM_DD,
		"hh_mm":           dashboardservice.XAXISTIMEFORMAT_X_AXIS_TIME_FORMAT_HH_MM,
		"dd_mm_hh_mm":     dashboardservice.XAXISTIMEFORMAT_X_AXIS_TIME_FORMAT_DD_MM_HH_MM,
		"hh_mm_dd_mm":     dashboardservice.XAXISTIMEFORMAT_X_AXIS_TIME_FORMAT_HH_MM_DD_MM,
		"mm_dd_hh_mm":     dashboardservice.XAXISTIMEFORMAT_X_AXIS_TIME_FORMAT_MM_DD_HH_MM,
		"hh_mm_mm_dd":     dashboardservice.XAXISTIMEFORMAT_X_AXIS_TIME_FORMAT_HH_MM_MM_DD,
	}
	dashboardProtoToSchemaXAxisTimeFormat = utils.ReverseMap(dashboardSchemaToProtoXAxisTimeFormat)
	dashboardValidXAxisTimeFormat         = utils.GetKeys(dashboardSchemaToProtoXAxisTimeFormat)

	dashboardSchemaToProtoBarValueDisplay = map[string]dashboardservice.VisualizationBarValueDisplay{
		utils.UNSPECIFIED: dashboardservice.VISUALIZATIONBARVALUEDISPLAY_BAR_VALUE_DISPLAY_UNSPECIFIED,
		"top":             dashboardservice.VISUALIZATIONBARVALUEDISPLAY_BAR_VALUE_DISPLAY_TOP,
		"inside":          dashboardservice.VISUALIZATIONBARVALUEDISPLAY_BAR_VALUE_DISPLAY_INSIDE,
		"both":            dashboardservice.VISUALIZATIONBARVALUEDISPLAY_BAR_VALUE_DISPLAY_BOTH,
	}
	dashboardProtoToSchemaBarValueDisplay = utils.ReverseMap(dashboardSchemaToProtoBarValueDisplay)
	dashboardValidBarValueDisplay         = utils.GetKeys(dashboardSchemaToProtoBarValueDisplay)

	dashboardSchemaToProtoThresholdBy = map[string]dashboardservice.CommonThresholdBy{
		utils.UNSPECIFIED: dashboardservice.COMMONTHRESHOLDBY_THRESHOLD_BY_UNSPECIFIED,
		"value":           dashboardservice.COMMONTHRESHOLDBY_THRESHOLD_BY_VALUE,
		"background":      dashboardservice.COMMONTHRESHOLDBY_THRESHOLD_BY_BACKGROUND,
	}
	dashboardProtoToSchemaThresholdBy = utils.ReverseMap(dashboardSchemaToProtoThresholdBy)
	dashboardValidThresholdBy         = utils.GetKeys(dashboardSchemaToProtoThresholdBy)

	dashboardSchemaToProtoColorApplyTarget = map[string]dashboardservice.ColorApplyTarget{
		utils.UNSPECIFIED: dashboardservice.COLORAPPLYTARGET_COLOR_APPLY_TARGET_UNSPECIFIED,
		"value":           dashboardservice.COLORAPPLYTARGET_COLOR_APPLY_TARGET_VALUE,
		"background":      dashboardservice.COLORAPPLYTARGET_COLOR_APPLY_TARGET_BACKGROUND,
		"row":             dashboardservice.COLORAPPLYTARGET_COLOR_APPLY_TARGET_ROW,
	}
	dashboardProtoToSchemaColorApplyTarget = utils.ReverseMap(dashboardSchemaToProtoColorApplyTarget)
	dashboardValidColorApplyTarget         = utils.GetKeys(dashboardSchemaToProtoColorApplyTarget)

	dashboardSchemaToProtoColorSolidType = map[string]dashboardservice.ColorSolidType{
		utils.UNSPECIFIED: dashboardservice.COLORSOLIDTYPE_COLOR_SOLID_TYPE_UNSPECIFIED,
		"blue":            dashboardservice.COLORSOLIDTYPE_COLOR_SOLID_TYPE_BLUE,
		"green":           dashboardservice.COLORSOLIDTYPE_COLOR_SOLID_TYPE_GREEN,
		"red":             dashboardservice.COLORSOLIDTYPE_COLOR_SOLID_TYPE_RED,
		"purple":          dashboardservice.COLORSOLIDTYPE_COLOR_SOLID_TYPE_PURPLE,
		"cyan":            dashboardservice.COLORSOLIDTYPE_COLOR_SOLID_TYPE_CYAN,
		"magenta":         dashboardservice.COLORSOLIDTYPE_COLOR_SOLID_TYPE_MAGENTA,
		"orange":          dashboardservice.COLORSOLIDTYPE_COLOR_SOLID_TYPE_ORANGE,
		"yellow":          dashboardservice.COLORSOLIDTYPE_COLOR_SOLID_TYPE_YELLOW,
	}
	dashboardProtoToSchemaColorSolidType = utils.ReverseMap(dashboardSchemaToProtoColorSolidType)
	dashboardValidColorSolidType         = utils.GetKeys(dashboardSchemaToProtoColorSolidType)

	dashboardSchemaToProtoTextAlignment = map[string]dashboardservice.TextAlignment{
		utils.UNSPECIFIED: dashboardservice.TEXTALIGNMENT_TEXT_ALIGNMENT_UNSPECIFIED,
		"left":            dashboardservice.TEXTALIGNMENT_TEXT_ALIGNMENT_LEFT,
		"center":          dashboardservice.TEXTALIGNMENT_TEXT_ALIGNMENT_CENTER,
		"right":           dashboardservice.TEXTALIGNMENT_TEXT_ALIGNMENT_RIGHT,
	}
	dashboardProtoToSchemaTextAlignment = utils.ReverseMap(dashboardSchemaToProtoTextAlignment)
	dashboardValidTextAlignment         = utils.GetKeys(dashboardSchemaToProtoTextAlignment)

	dashboardSchemaToProtoValuesMappingType = map[string]dashboardservice.ValuesMappingType{
		utils.UNSPECIFIED: dashboardservice.VALUESMAPPINGTYPE_VALUES_MAPPING_TYPE_UNSPECIFIED,
		"value":           dashboardservice.VALUESMAPPINGTYPE_VALUES_MAPPING_TYPE_VALUE,
		"regex":           dashboardservice.VALUESMAPPINGTYPE_VALUES_MAPPING_TYPE_REGEX,
	}
	dashboardProtoToSchemaValuesMappingType = utils.ReverseMap(dashboardSchemaToProtoValuesMappingType)
	dashboardValidValuesMappingType         = utils.GetKeys(dashboardSchemaToProtoValuesMappingType)

	dashboardSchemaToProtoFieldDataType = map[string]dashboardservice.FieldDataType{
		utils.UNSPECIFIED: dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_UNSPECIFIED,
		"number":          dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_NUMBER,
		"string":          dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_STRING,
		"boolean":         dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_BOOLEAN,
		"timestamp":       dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_TIMESTAMP,
		"object":          dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_OBJECT,
		"array":           dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_ARRAY,
		"regex":           dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_REGEX,
		"union":           dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_UNION,
		"enum":            dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_ENUM,
	}
	dashboardProtoToSchemaFieldDataType = utils.ReverseMap(dashboardSchemaToProtoFieldDataType)
	dashboardValidFieldDataType         = utils.GetKeys(dashboardSchemaToProtoFieldDataType)

	dashboardSchemaToProtoPieChartLabelSource = map[string]dashboardservice.VisualizationPieChartLabelSource{
		utils.UNSPECIFIED: dashboardservice.VISUALIZATIONPIECHARTLABELSOURCE_LABEL_SOURCE_UNSPECIFIED,
		"inner":           dashboardservice.VISUALIZATIONPIECHARTLABELSOURCE_LABEL_SOURCE_INNER,
		"stack":           dashboardservice.VISUALIZATIONPIECHARTLABELSOURCE_LABEL_SOURCE_STACK,
	}
	dashboardProtoToSchemaPieChartLabelSource = utils.ReverseMap(dashboardSchemaToProtoPieChartLabelSource)
	dashboardValidPieChartLabelSource         = utils.GetKeys(dashboardSchemaToProtoPieChartLabelSource)

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
)

type mustBeTrueValidator struct{}

func (v mustBeTrueValidator) Description(context.Context) string {
	return "value must be true when set"
}

func (v mustBeTrueValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v mustBeTrueValidator) ValidateBool(ctx context.Context, req validator.BoolRequest, resp *validator.BoolResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if !req.ConfigValue.ValueBool() {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid value", "this marker must be true when set; omit it otherwise")
	}
}

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
				MarkdownDescription: fmt.Sprintf("The PromQL query type. Valid values are: %s.", strings.Join(DashboardValidPromQLQueryType, ", ")),
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
			ElementType:         types.StringType,
			Required:            true,
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
			},
			"custom_unit": schema.StringAttribute{
				Optional: true,
			},
			"decimal_precision": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.Between(0, math.MaxInt32),
				},
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
				Attributes: ObservationFieldSchema(),
				Optional:   true,
			},
			"value_fields": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: ObservationFieldSchema(),
				},
			},
		},
	}
}

func dynamicThresholdsSchema() schema.Attribute {
	return schema.ListNestedAttribute{
		Optional: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"from": schema.Float64Attribute{
					Optional: true,
				},
				"color": schema.StringAttribute{
					Optional: true,
				},
				"label": schema.StringAttribute{
					Optional: true,
				},
			},
		},
	}
}

func dynamicStatCardSchema() schema.Attribute {
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
			},
			"color_label_mapping": dynamicColorLabelMappingSchema(),
			"custom_unit": schema.StringAttribute{
				Optional: true,
			},
			"decimal_precision": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.Between(0, math.MaxInt32),
				},
			},
			"label":  dynamicStatVisualElementSchema(),
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
			"primary_value": dynamicStatVisualElementSchema(),
			"title":         dynamicStatVisualElementSchema(),
			"unit":          UnitSchema(),
			"value_fields": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: ObservationFieldSchema(),
				},
			},
		},
	}
}

func dynamicStatVisualElementSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"mapped_values": schema.StringAttribute{
				CustomType:          JSONStringType{},
				Optional:            true,
				MarkdownDescription: "Mapped values encoded as a JSON object string.",
			},
			"observation_field": schema.SingleNestedAttribute{
				Attributes: ObservationFieldSchema(),
				Optional:   true,
			},
			"template_text": schema.StringAttribute{
				Optional: true,
			},
			"template_variables": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"mapped_values": schema.StringAttribute{
							CustomType:          JSONStringType{},
							Optional:            true,
							MarkdownDescription: "Mapped values encoded as a JSON object string.",
						},
						"observation_field": schema.SingleNestedAttribute{
							Attributes: ObservationFieldSchema(),
							Optional:   true,
						},
					},
				},
			},
		},
	}
}

func dynamicColorLabelMappingSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"color_by": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidColorApplyTarget...),
				},
				MarkdownDescription: fmt.Sprintf("Which part of the widget the color applies to. Valid values are: %s.", strings.Join(dashboardValidColorApplyTarget, ", ")),
			},
			"range": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"min_max": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"auto": schema.BoolAttribute{
								Optional: true,
								Validators: []validator.Bool{
									mustBeTrueValidator{},
								},
							},
							"custom": schema.SingleNestedAttribute{
								Optional: true,
								Attributes: map[string]schema.Attribute{
									"max": schema.Float64Attribute{
										Optional: true,
									},
									"min": schema.Float64Attribute{
										Optional: true,
									},
								},
							},
						},
						Validators: []validator.Object{
							ExactlyOneOfChildren("auto", "custom"),
						},
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
				},
			},
			"regex": dynamicMappingSectionsSchema(),
			"value": dynamicMappingSectionsSchema(),
		},
	}
}

func dynamicMappingSectionsSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"sections": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"color": schema.StringAttribute{
							Optional: true,
							Computed: true,
							Default:  stringdefault.StaticString(utils.UNSPECIFIED),
							Validators: []validator.String{
								stringvalidator.OneOf(dashboardValidColorSolidType...),
							},
							MarkdownDescription: fmt.Sprintf("The section color. Valid values are: %s.", strings.Join(dashboardValidColorSolidType, ", ")),
						},
						"map_to": schema.StringAttribute{
							Optional: true,
						},
						"value": schema.StringAttribute{
							Optional: true,
						},
					},
				},
			},
		},
	}
}

func dynamicTableSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"columns": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"field": schema.SingleNestedAttribute{
							Attributes: ObservationFieldSchema(),
							Optional:   true,
						},
					},
				},
			},
			"rules": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"description": schema.StringAttribute{
							Optional: true,
						},
						"id": schema.StringAttribute{
							Optional: true,
							Computed: true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseNonNullStateForUnknown(),
							},
						},
						"name": schema.StringAttribute{
							Optional: true,
						},
						"properties": schema.ListNestedAttribute{
							Optional: true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"id": schema.StringAttribute{
										Optional: true,
										Computed: true,
										PlanModifiers: []planmodifier.String{
											stringplanmodifier.UseNonNullStateForUnknown(),
										},
									},
									"definition": dynamicTablePropertyDefinitionSchema(),
								},
							},
						},
						"rule_scope": dynamicTableRuleScopeSchema(),
					},
				},
			},
			"settings": dynamicTableSettingsSchema(),
		},
	}
}

func dynamicTablePropertyDefinitionSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"alignment": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidTextAlignment...),
				},
				MarkdownDescription: fmt.Sprintf("The text alignment. Valid values are: %s.", strings.Join(dashboardValidTextAlignment, ", ")),
			},
			"column_display_name": schema.StringAttribute{
				Optional: true,
			},
			"link": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"actions": schema.ListNestedAttribute{
						Optional: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"id": schema.StringAttribute{
									Optional: true,
									Computed: true,
									PlanModifiers: []planmodifier.String{
										stringplanmodifier.UseNonNullStateForUnknown(),
									},
								},
								"name": schema.StringAttribute{
									Optional: true,
								},
								"should_open_in_new_window": schema.BoolAttribute{
									Optional: true,
								},
								"url": schema.StringAttribute{
									Optional: true,
								},
							},
						},
					},
				},
			},
			"regex_extract": schema.StringAttribute{
				Optional: true,
			},
			"thresholds": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"max": schema.Float64Attribute{
						Optional: true,
					},
					"min": schema.Float64Attribute{
						Optional: true,
					},
					"type": schema.StringAttribute{
						Optional: true,
						Computed: true,
						Default:  stringdefault.StaticString(utils.UNSPECIFIED),
						Validators: []validator.String{
							stringvalidator.OneOf(DashboardValidThresholdTypes...),
						},
						MarkdownDescription: fmt.Sprintf("The threshold type. Valid values are: %s.", strings.Join(DashboardValidThresholdTypes, ", ")),
					},
					"values": dynamicThresholdsSchema(),
				},
			},
			"units": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"allow_abbreviation": schema.BoolAttribute{
						Optional: true,
					},
					"custom_unit": schema.StringAttribute{
						Optional: true,
					},
					"decimal_precision": schema.Int64Attribute{
						Optional: true,
						Validators: []validator.Int64{
							int64validator.Between(0, math.MaxInt32),
						},
					},
					"max": schema.Float64Attribute{
						Optional: true,
					},
					"min": schema.Float64Attribute{
						Optional: true,
					},
					"unit": UnitSchema(),
				},
			},
			"values_alias": schema.StringAttribute{
				Optional: true,
			},
			"values_mapping": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"mappings": schema.ListNestedAttribute{
						Optional: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"input_value": schema.StringAttribute{
									Optional: true,
								},
								"replace_value": schema.StringAttribute{
									Optional: true,
								},
								"type": schema.StringAttribute{
									Optional: true,
									Computed: true,
									Default:  stringdefault.StaticString(utils.UNSPECIFIED),
									Validators: []validator.String{
										stringvalidator.OneOf(dashboardValidValuesMappingType...),
									},
									MarkdownDescription: fmt.Sprintf("The values mapping type. Valid values are: %s.", strings.Join(dashboardValidValuesMappingType, ", ")),
								},
							},
						},
					},
				},
			},
		},
		Validators: []validator.Object{
			ExactlyOneOfChildren(
				"thresholds", "alignment", "units", "regex_extract",
				"link", "values_alias", "values_mapping", "column_display_name",
			),
		},
	}
}

func dynamicTableRuleScopeSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"field": schema.SingleNestedAttribute{
				Attributes: ObservationFieldSchema(),
				Optional:   true,
			},
			"field_type": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidFieldDataType...),
				},
				MarkdownDescription: fmt.Sprintf("The field data type. Valid values are: %s.", strings.Join(dashboardValidFieldDataType, ", ")),
			},
			"regex": schema.StringAttribute{
				Optional: true,
			},
		},
		Validators: []validator.Object{
			ExactlyOneOfChildren("field", "regex", "field_type"),
		},
	}
}

func dynamicTableSettingsSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"column_widths": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"column_name": schema.StringAttribute{
							Optional: true,
						},
						"width": schema.Int64Attribute{
							Optional: true,
							Validators: []validator.Int64{
								int64validator.Between(math.MinInt32, math.MaxInt32),
							},
						},
					},
				},
			},
			"row_style": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidRowStyles...),
				},
				MarkdownDescription: fmt.Sprintf("The row style. Valid values are: %s.", strings.Join(DashboardValidRowStyles, ", ")),
			},
		},
	}
}

func dynamicTimeSeriesLinesSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional:            true,
		DeprecationMessage:  "Deprecated: use time_series_lines_multi instead.",
		MarkdownDescription: "Deprecated: use `time_series_lines_multi` instead. Retained at full fidelity for importing dashboards that still use the singular time series lines visualization.",
		Attributes: map[string]schema.Attribute{
			"allow_abbreviation": schema.BoolAttribute{
				Optional: true,
			},
			"category_fields": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: ObservationFieldSchema(),
				},
			},
			"color_scheme": schema.StringAttribute{
				Optional: true,
			},
			"connect_nulls": schema.BoolAttribute{
				Optional: true,
			},
			"custom_unit": schema.StringAttribute{
				Optional: true,
			},
			"decimal_precision": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.Between(0, math.MaxInt32),
				},
			},
			"hash_colors": schema.BoolAttribute{
				Optional: true,
			},
			"legend": LegendSchema(),
			"scale_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidScaleTypes...),
				},
				MarkdownDescription: fmt.Sprintf("The scale type. Valid values are: %s.", strings.Join(DashboardValidScaleTypes, ", ")),
			},
			"series_count_limit": schema.Int64Attribute{
				Optional: true,
			},
			"series_name_template": schema.StringAttribute{
				Optional: true,
			},
			"stacked_line": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidStackedLine...),
				},
				MarkdownDescription: fmt.Sprintf("How lines are stacked. Valid values are: %s.", strings.Join(dashboardValidStackedLine, ", ")),
			},
			"temporal_field": schema.SingleNestedAttribute{
				Attributes: ObservationFieldSchema(),
				Optional:   true,
			},
			"tooltip": dynamicTimeSeriesTooltipSchema(),
			"unit":    UnitSchema(),
			"use_data_time_range": schema.BoolAttribute{
				Optional: true,
			},
			"value_fields": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: ObservationFieldSchema(),
				},
			},
			"x_axis_time_format": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidXAxisTimeFormat...),
				},
				MarkdownDescription: fmt.Sprintf("The x-axis time format. Valid values are: %s.", strings.Join(dashboardValidXAxisTimeFormat, ", ")),
			},
			"y_axis_max": schema.Float64Attribute{
				Optional:   true,
				CustomType: Float32Type{},
				Validators: []validator.Float64{
					float64validator.Between(-math.MaxFloat32, math.MaxFloat32),
				},
				MarkdownDescription: "The y-axis maximum. Stored at float32 precision by the API.",
			},
			"y_axis_min": schema.Float64Attribute{
				Optional:   true,
				CustomType: Float32Type{},
				Validators: []validator.Float64{
					float64validator.Between(-math.MaxFloat32, math.MaxFloat32),
				},
				MarkdownDescription: "The y-axis minimum. Stored at float32 precision by the API.",
			},
		},
	}
}

func dynamicTimeSeriesLinesMultiSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"connect_nulls": schema.BoolAttribute{
				Optional: true,
			},
			"legend":                 LegendSchema(),
			"query_display_settings": dynamicQueryDisplaySettingsSchema(),
			"stacked_line": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidStackedLine...),
				},
				MarkdownDescription: fmt.Sprintf("How lines are stacked. Valid values are: %s.", strings.Join(dashboardValidStackedLine, ", ")),
			},
			"tooltip": dynamicTimeSeriesTooltipSchema(),
			"use_data_time_range": schema.BoolAttribute{
				Optional: true,
			},
			"x_axis_time_format": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidXAxisTimeFormat...),
				},
				MarkdownDescription: fmt.Sprintf("The x-axis time format. Valid values are: %s.", strings.Join(dashboardValidXAxisTimeFormat, ", ")),
			},
		},
	}
}

func dynamicTimeSeriesBarsSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"allow_abbreviation": schema.BoolAttribute{
				Optional: true,
			},
			"bar_value_display": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidBarValueDisplay...),
				},
				MarkdownDescription: fmt.Sprintf("Where bar values are displayed. Valid values are: %s.", strings.Join(dashboardValidBarValueDisplay, ", ")),
			},
			"category_fields": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: ObservationFieldSchema(),
				},
			},
			"color_scheme": schema.StringAttribute{
				Optional: true,
			},
			"custom_unit": schema.StringAttribute{
				Optional: true,
			},
			"decimal_precision": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.Between(0, math.MaxInt32),
				},
			},
			"hash_colors": schema.BoolAttribute{
				Optional: true,
			},
			"legend": LegendSchema(),
			"max_slices_per_bar": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.Between(0, math.MaxInt32),
				},
			},
			"scale_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidScaleTypes...),
				},
				MarkdownDescription: fmt.Sprintf("The scale type. Valid values are: %s.", strings.Join(DashboardValidScaleTypes, ", ")),
			},
			"series_name_template": schema.StringAttribute{
				Optional: true,
			},
			"sort_by": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidSortBy...),
				},
				MarkdownDescription: fmt.Sprintf("How bars are sorted. Valid values are: %s.", strings.Join(DashboardValidSortBy, ", ")),
			},
			"temporal_field": schema.SingleNestedAttribute{
				Attributes: ObservationFieldSchema(),
				Optional:   true,
			},
			"tooltip": dynamicTimeSeriesTooltipSchema(),
			"unit":    UnitSchema(),
			"value_fields": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: ObservationFieldSchema(),
				},
			},
			"x_axis_time_format": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidXAxisTimeFormat...),
				},
				MarkdownDescription: fmt.Sprintf("The x-axis time format. Valid values are: %s.", strings.Join(dashboardValidXAxisTimeFormat, ", ")),
			},
			"y_axis_max": schema.Float64Attribute{
				Optional:   true,
				CustomType: Float32Type{},
				Validators: []validator.Float64{
					float64validator.Between(-math.MaxFloat32, math.MaxFloat32),
				},
				MarkdownDescription: "The y-axis maximum. Stored at float32 precision by the API.",
			},
			"y_axis_min": schema.Float64Attribute{
				Optional:   true,
				CustomType: Float32Type{},
				Validators: []validator.Float64{
					float64validator.Between(-math.MaxFloat32, math.MaxFloat32),
				},
				MarkdownDescription: "The y-axis minimum. Stored at float32 precision by the API.",
			},
		},
	}
}

func dynamicQueryDisplaySettingsSchema() schema.Attribute {
	return schema.ListNestedAttribute{
		Optional: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"allow_abbreviation": schema.BoolAttribute{
					Optional: true,
				},
				"category_fields": schema.ListNestedAttribute{
					Optional: true,
					NestedObject: schema.NestedAttributeObject{
						Attributes: ObservationFieldSchema(),
					},
				},
				"color_scheme": schema.StringAttribute{
					Optional: true,
				},
				"custom_unit": schema.StringAttribute{
					Optional: true,
				},
				"decimal_precision": schema.Int64Attribute{
					Optional: true,
					Validators: []validator.Int64{
						int64validator.Between(0, math.MaxInt32),
					},
				},
				"hash_colors": schema.BoolAttribute{
					Optional: true,
				},
				"query_id": schema.StringAttribute{
					Required: true,
				},
				"scale_type": schema.StringAttribute{
					Optional: true,
					Computed: true,
					Default:  stringdefault.StaticString(utils.UNSPECIFIED),
					Validators: []validator.String{
						stringvalidator.OneOf(DashboardValidScaleTypes...),
					},
					MarkdownDescription: fmt.Sprintf("The scale type. Valid values are: %s.", strings.Join(DashboardValidScaleTypes, ", ")),
				},
				"series_count_limit": schema.Int64Attribute{
					Optional: true,
				},
				"series_name_template": schema.StringAttribute{
					Optional: true,
				},
				"temporal_field": schema.SingleNestedAttribute{
					Attributes: ObservationFieldSchema(),
					Optional:   true,
				},
				"unit": UnitSchema(),
				"value_fields": schema.ListNestedAttribute{
					Optional: true,
					NestedObject: schema.NestedAttributeObject{
						Attributes: ObservationFieldSchema(),
					},
				},
				"y_axis_max": schema.Float64Attribute{
					Optional:   true,
					CustomType: Float32Type{},
					Validators: []validator.Float64{
						float64validator.Between(-math.MaxFloat32, math.MaxFloat32),
					},
					MarkdownDescription: "The y-axis maximum. Stored at float32 precision by the API.",
				},
				"y_axis_min": schema.Float64Attribute{
					Optional:   true,
					CustomType: Float32Type{},
					Validators: []validator.Float64{
						float64validator.Between(-math.MaxFloat32, math.MaxFloat32),
					},
					MarkdownDescription: "The y-axis minimum. Stored at float32 precision by the API.",
				},
			},
		},
	}
}

func dynamicTimeSeriesTooltipSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"show_all_series": schema.BoolAttribute{
				Optional: true,
			},
			"show_labels": schema.BoolAttribute{
				Optional: true,
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

func dynamicTimeSeriesLinesModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"allow_abbreviation": types.BoolType,
		"category_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"color_scheme":         types.StringType,
		"connect_nulls":        types.BoolType,
		"custom_unit":          types.StringType,
		"decimal_precision":    types.Int64Type,
		"hash_colors":          types.BoolType,
		"legend":               types.ObjectType{AttrTypes: LegendAttr()},
		"scale_type":           types.StringType,
		"series_count_limit":   types.Int64Type,
		"series_name_template": types.StringType,
		"stacked_line":         types.StringType,
		"temporal_field":       ObservationFieldsObject(),
		"tooltip":              types.ObjectType{AttrTypes: dynamicTimeSeriesTooltipModelAttr()},
		"unit":                 types.StringType,
		"use_data_time_range":  types.BoolType,
		"value_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"x_axis_time_format": types.StringType,
		"y_axis_max":         Float32Type{},
		"y_axis_min":         Float32Type{},
	}
}

func dynamicTimeSeriesLinesMultiModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"connect_nulls": types.BoolType,
		"legend":        types.ObjectType{AttrTypes: LegendAttr()},
		"query_display_settings": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicQueryDisplaySettingsModelAttr()},
		},
		"stacked_line":        types.StringType,
		"tooltip":             types.ObjectType{AttrTypes: dynamicTimeSeriesTooltipModelAttr()},
		"use_data_time_range": types.BoolType,
		"x_axis_time_format":  types.StringType,
	}
}

func dynamicTimeSeriesBarsModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"allow_abbreviation": types.BoolType,
		"bar_value_display":  types.StringType,
		"category_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"color_scheme":         types.StringType,
		"custom_unit":          types.StringType,
		"decimal_precision":    types.Int64Type,
		"hash_colors":          types.BoolType,
		"legend":               types.ObjectType{AttrTypes: LegendAttr()},
		"max_slices_per_bar":   types.Int64Type,
		"scale_type":           types.StringType,
		"series_name_template": types.StringType,
		"sort_by":              types.StringType,
		"temporal_field":       ObservationFieldsObject(),
		"tooltip":              types.ObjectType{AttrTypes: dynamicTimeSeriesTooltipModelAttr()},
		"unit":                 types.StringType,
		"value_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"x_axis_time_format": types.StringType,
		"y_axis_max":         Float32Type{},
		"y_axis_min":         Float32Type{},
	}
}

func dynamicQueryDisplaySettingsModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"allow_abbreviation": types.BoolType,
		"category_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"color_scheme":         types.StringType,
		"custom_unit":          types.StringType,
		"decimal_precision":    types.Int64Type,
		"hash_colors":          types.BoolType,
		"query_id":             types.StringType,
		"scale_type":           types.StringType,
		"series_count_limit":   types.Int64Type,
		"series_name_template": types.StringType,
		"temporal_field":       ObservationFieldsObject(),
		"unit":                 types.StringType,
		"value_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"y_axis_max": Float32Type{},
		"y_axis_min": Float32Type{},
	}
}

func dynamicTimeSeriesTooltipModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"show_all_series": types.BoolType,
		"show_labels":     types.BoolType,
	}
}

func dynamicTableModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"columns": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicTableColumnModelAttr()},
		},
		"rules": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicTableRuleModelAttr()},
		},
		"settings": types.ObjectType{AttrTypes: dynamicTableSettingsModelAttr()},
	}
}

func dynamicTableColumnModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field": ObservationFieldsObject(),
	}
}

func dynamicTableRuleModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"description": types.StringType,
		"id":          types.StringType,
		"name":        types.StringType,
		"properties": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicTablePropertyModelAttr()},
		},
		"rule_scope": types.ObjectType{AttrTypes: dynamicTableRuleScopeModelAttr()},
	}
}

func dynamicTablePropertyModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"id":         types.StringType,
		"definition": types.ObjectType{AttrTypes: dynamicTablePropertyDefinitionModelAttr()},
	}
}

func dynamicTablePropertyDefinitionModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"alignment":           types.StringType,
		"column_display_name": types.StringType,
		"link":                types.ObjectType{AttrTypes: dynamicTablePropertyLinkModelAttr()},
		"regex_extract":       types.StringType,
		"thresholds":          types.ObjectType{AttrTypes: dynamicTablePropertyThresholdsModelAttr()},
		"units":               types.ObjectType{AttrTypes: dynamicTablePropertyUnitsModelAttr()},
		"values_alias":        types.StringType,
		"values_mapping":      types.ObjectType{AttrTypes: dynamicTablePropertyValuesMappingModelAttr()},
	}
}

func dynamicTablePropertyLinkModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"actions": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicTableLinkActionModelAttr()},
		},
	}
}

func dynamicTableLinkActionModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"id":                        types.StringType,
		"name":                      types.StringType,
		"should_open_in_new_window": types.BoolType,
		"url":                       types.StringType,
	}
}

func dynamicTablePropertyThresholdsModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"max":  types.Float64Type,
		"min":  types.Float64Type,
		"type": types.StringType,
		"values": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicThresholdAttr()},
		},
	}
}

func dynamicTablePropertyUnitsModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"allow_abbreviation": types.BoolType,
		"custom_unit":        types.StringType,
		"decimal_precision":  types.Int64Type,
		"max":                types.Float64Type,
		"min":                types.Float64Type,
		"unit":               types.StringType,
	}
}

func dynamicTablePropertyValuesMappingModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"mappings": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicTableValueMappingModelAttr()},
		},
	}
}

func dynamicTableValueMappingModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"input_value":   types.StringType,
		"replace_value": types.StringType,
		"type":          types.StringType,
	}
}

func dynamicTableRuleScopeModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field":      ObservationFieldsObject(),
		"field_type": types.StringType,
		"regex":      types.StringType,
	}
}

func dynamicTableSettingsModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"column_widths": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicTableColumnWidthModelAttr()},
		},
		"row_style": types.StringType,
	}
}

func dynamicTableColumnWidthModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"column_name": types.StringType,
		"width":       types.Int64Type,
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

func dynamicStatCardModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"allow_abbreviation": types.BoolType,
		"category_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"color_label_mapping": types.ObjectType{AttrTypes: dynamicColorLabelMappingAttr()},
		"custom_unit":         types.StringType,
		"decimal_precision":   types.Int64Type,
		"label":               types.ObjectType{AttrTypes: dynamicStatVisualElementAttr()},
		"legend":              types.ObjectType{AttrTypes: LegendAttr()},
		"legend_by":           types.StringType,
		"primary_value":       types.ObjectType{AttrTypes: dynamicStatVisualElementAttr()},
		"title":               types.ObjectType{AttrTypes: dynamicStatVisualElementAttr()},
		"unit":                types.StringType,
		"value_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
	}
}

func dynamicStatVisualElementAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"mapped_values":     JSONStringType{},
		"observation_field": ObservationFieldsObject(),
		"template_text":     types.StringType,
		"template_variables": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicTemplateVariableAttr()},
		},
	}
}

func dynamicTemplateVariableAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"mapped_values":     JSONStringType{},
		"observation_field": ObservationFieldsObject(),
	}
}

func dynamicColorLabelMappingAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"color_by": types.StringType,
		"range":    types.ObjectType{AttrTypes: dynamicRangeMappingAttr()},
		"regex":    types.ObjectType{AttrTypes: dynamicSectionsMappingAttr()},
		"value":    types.ObjectType{AttrTypes: dynamicSectionsMappingAttr()},
	}
}

func dynamicRangeMappingAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"min_max":        types.ObjectType{AttrTypes: dynamicMinMaxAttr()},
		"threshold_type": types.StringType,
		"thresholds": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicThresholdAttr()},
		},
	}
}

func dynamicMinMaxAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"auto":   types.BoolType,
		"custom": types.ObjectType{AttrTypes: dynamicMinMaxCustomAttr()},
	}
}

func dynamicMinMaxCustomAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"max": types.Float64Type,
		"min": types.Float64Type,
	}
}

func dynamicSectionsMappingAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"sections": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicMappingSectionAttr()},
		},
	}
}

func dynamicMappingSectionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"color":  types.StringType,
		"map_to": types.StringType,
		"value":  types.StringType,
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
		field, dg := expandSpanObservationFieldModel(ctx, model)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		fields = append(fields, *field)
	}

	return fields, diags
}

func expandSpanObservationFieldObject(ctx context.Context, field types.Object) (*dashboardservice.SpanObservationField, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(field) {
		return nil, nil
	}

	var model SpanObservationFieldModel
	if dg := field.As(ctx, &model, basetypes.ObjectAsOptions{}); dg.HasError() {
		return nil, dg
	}

	return expandSpanObservationFieldModel(ctx, model)
}

func expandSpanObservationFieldModel(ctx context.Context, model SpanObservationFieldModel) (*dashboardservice.SpanObservationField, diag.Diagnostics) {
	keypath, dg := typeStringListToStringSlice(ctx, model.Keypath)
	if dg.HasError() {
		return nil, dg
	}

	return &dashboardservice.SpanObservationField{
		Keypath:      keypath,
		Scope:        OptionalEnumPointer(model.Scope, DashboardSchemaToProtoObservationFieldScope),
		RelationType: OptionalEnumPointer(model.RelationType, dashboardSchemaToProtoSpanRelationType),
	}, nil
}

func expandDynamicVisualization(ctx context.Context, visualization *DynamicVisualizationModel) (*dashboardservice.Visualization, diag.Diagnostics) {
	if visualization == nil {
		return nil, nil
	}

	for _, group := range []func(context.Context, *DynamicVisualizationModel) (*dashboardservice.Visualization, diag.Diagnostics, bool){
		expandDynamicVisualizationStatGroup,
		expandDynamicVisualizationChartGroup,
		expandDynamicVisualizationGeoGroup,
	} {
		if viz, diags, handled := group(ctx, visualization); handled {
			return viz, diags
		}
	}

	return nil, diag.Diagnostics{diag.NewErrorDiagnostic(
		"Unsupported Dashboard Widget Definition",
		"The dynamic widget uses an unknown or unsupported visualization variant.",
	)}
}

func expandDynamicVisualizationStatGroup(ctx context.Context, visualization *DynamicVisualizationModel) (*dashboardservice.Visualization, diag.Diagnostics, bool) {
	switch {
	case visualization.Stat != nil:
		stat, diags := expandDynamicStat(ctx, visualization.Stat)
		if diags.HasError() {
			return nil, diags, true
		}
		return &dashboardservice.Visualization{Stat: stat}, nil, true
	case visualization.StatCard != nil:
		statCard, diags := expandDynamicStatCard(ctx, visualization.StatCard)
		if diags.HasError() {
			return nil, diags, true
		}
		return &dashboardservice.Visualization{StatCard: statCard}, nil, true
	case visualization.Table != nil:
		table, diags := expandDynamicTable(ctx, visualization.Table)
		if diags.HasError() {
			return nil, diags, true
		}
		return &dashboardservice.Visualization{Table: table}, nil, true
	case visualization.Gauge != nil:
		gauge, diags := expandDynamicGauge(ctx, visualization.Gauge)
		if diags.HasError() {
			return nil, diags, true
		}
		return &dashboardservice.Visualization{Gauge: gauge}, nil, true
	case visualization.PieChart != nil:
		pieChart, diags := expandDynamicPieChart(ctx, visualization.PieChart)
		if diags.HasError() {
			return nil, diags, true
		}
		return &dashboardservice.Visualization{PieChart: pieChart}, nil, true
	default:
		return nil, nil, false
	}
}

func expandDynamicVisualizationChartGroup(ctx context.Context, visualization *DynamicVisualizationModel) (*dashboardservice.Visualization, diag.Diagnostics, bool) {
	switch {
	case visualization.TimeSeriesLines != nil:
		lines, diags := expandDynamicTimeSeriesLines(ctx, visualization.TimeSeriesLines)
		if diags.HasError() {
			return nil, diags, true
		}
		return &dashboardservice.Visualization{TimeSeriesLines: lines}, nil, true
	case visualization.TimeSeriesLinesMulti != nil:
		linesMulti, diags := expandDynamicTimeSeriesLinesMulti(ctx, visualization.TimeSeriesLinesMulti)
		if diags.HasError() {
			return nil, diags, true
		}
		return &dashboardservice.Visualization{TimeSeriesLinesMulti: linesMulti}, nil, true
	case visualization.TimeSeriesBars != nil:
		bars, diags := expandDynamicTimeSeriesBars(ctx, visualization.TimeSeriesBars)
		if diags.HasError() {
			return nil, diags, true
		}
		return &dashboardservice.Visualization{TimeSeriesBars: bars}, nil, true
	case visualization.VerticalBars != nil:
		verticalBars, diags := expandDynamicVerticalBars(ctx, visualization.VerticalBars)
		if diags.HasError() {
			return nil, diags, true
		}
		return &dashboardservice.Visualization{VerticalBars: verticalBars}, nil, true
	case visualization.VerticalBarsMulti != nil:
		verticalBarsMulti, diags := expandDynamicVerticalBarsMulti(ctx, visualization.VerticalBarsMulti)
		if diags.HasError() {
			return nil, diags, true
		}
		return &dashboardservice.Visualization{VerticalBarsMulti: verticalBarsMulti}, nil, true
	case visualization.HorizontalBars != nil:
		horizontalBars, diags := expandDynamicHorizontalBars(ctx, visualization.HorizontalBars)
		if diags.HasError() {
			return nil, diags, true
		}
		return &dashboardservice.Visualization{HorizontalBars: horizontalBars}, nil, true
	case visualization.HorizontalBarsMulti != nil:
		horizontalBarsMulti, diags := expandDynamicHorizontalBarsMulti(ctx, visualization.HorizontalBarsMulti)
		if diags.HasError() {
			return nil, diags, true
		}
		return &dashboardservice.Visualization{HorizontalBarsMulti: horizontalBarsMulti}, nil, true
	default:
		return nil, nil, false
	}
}

func expandDynamicVisualizationGeoGroup(ctx context.Context, visualization *DynamicVisualizationModel) (*dashboardservice.Visualization, diag.Diagnostics, bool) {
	switch {
	case visualization.HexagonBins != nil:
		hexagonBins, diags := expandDynamicHexagonBins(ctx, visualization.HexagonBins)
		if diags.HasError() {
			return nil, diags, true
		}
		return &dashboardservice.Visualization{HexagonBins: hexagonBins}, nil, true
	case visualization.Heatmap != nil:
		heatmap, diags := expandDynamicHeatmap(ctx, visualization.Heatmap)
		if diags.HasError() {
			return nil, diags, true
		}
		return &dashboardservice.Visualization{Heatmap: heatmap}, nil, true
	case visualization.Geomap != nil:
		geomap, diags := expandDynamicGeomap(ctx, visualization.Geomap)
		if diags.HasError() {
			return nil, diags, true
		}
		return &dashboardservice.Visualization{Geomap: geomap}, nil, true
	default:
		return nil, nil, false
	}
}

func expandDynamicTimeSeriesLines(ctx context.Context, lines *DynamicTimeSeriesLinesModel) (*dashboardservice.TimeSeriesLines, diag.Diagnostics) {
	if lines == nil {
		return nil, nil
	}

	categoryFields, diags := ExpandObservationFields(ctx, lines.CategoryFields)
	if diags.HasError() {
		return nil, diags
	}

	valueFields, diags := ExpandObservationFields(ctx, lines.ValueFields)
	if diags.HasError() {
		return nil, diags
	}

	temporalField, diags := ExpandObservationFieldObject(ctx, lines.TemporalField)
	if diags.HasError() {
		return nil, diags
	}

	legend, diags := ExpandLegend(ctx, lines.Legend)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.TimeSeriesLines{
		AllowAbbreviation:  lines.AllowAbbreviation.ValueBoolPointer(),
		CategoryFields:     categoryFields,
		ColorScheme:        lines.ColorScheme.ValueStringPointer(),
		ConnectNulls:       lines.ConnectNulls.ValueBoolPointer(),
		CustomUnit:         lines.CustomUnit.ValueStringPointer(),
		DecimalPrecision:   expandInt32Pointer(lines.DecimalPrecision),
		HashColors:         lines.HashColors.ValueBoolPointer(),
		Legend:             legend,
		ScaleType:          OptionalEnumPointer(lines.ScaleType, DashboardSchemaToProtoScaleType),
		SeriesCountLimit:   int64ToStringPointer(lines.SeriesCountLimit),
		SeriesNameTemplate: lines.SeriesNameTemplate.ValueStringPointer(),
		StackedLine:        OptionalEnumPointer(lines.StackedLine, dashboardSchemaToProtoStackedLine),
		TemporalField:      temporalField,
		Tooltip:            expandDynamicTimeSeriesTooltip(lines.Tooltip),
		Unit:               OptionalEnumPointer(lines.Unit, DashboardSchemaToProtoUnit),
		UseDataTimeRange:   lines.UseDataTimeRange.ValueBoolPointer(),
		ValueFields:        valueFields,
		XAxisTimeFormat:    OptionalEnumPointer(lines.XAxisTimeFormat, dashboardSchemaToProtoXAxisTimeFormat),
		YAxisMax:           expandFloat32Pointer(lines.YAxisMax),
		YAxisMin:           expandFloat32Pointer(lines.YAxisMin),
	}, nil
}

func expandDynamicTimeSeriesLinesMulti(ctx context.Context, linesMulti *DynamicTimeSeriesLinesMultiModel) (*dashboardservice.TimeSeriesLinesMulti, diag.Diagnostics) {
	if linesMulti == nil {
		return nil, nil
	}

	legend, diags := ExpandLegend(ctx, linesMulti.Legend)
	if diags.HasError() {
		return nil, diags
	}

	queryDisplaySettings, diags := expandDynamicQueryDisplaySettings(ctx, linesMulti.QueryDisplaySettings)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.TimeSeriesLinesMulti{
		ConnectNulls:         linesMulti.ConnectNulls.ValueBoolPointer(),
		Legend:               legend,
		QueryDisplaySettings: queryDisplaySettings,
		StackedLine:          OptionalEnumPointer(linesMulti.StackedLine, dashboardSchemaToProtoStackedLine),
		Tooltip:              expandDynamicTimeSeriesTooltip(linesMulti.Tooltip),
		UseDataTimeRange:     linesMulti.UseDataTimeRange.ValueBoolPointer(),
		XAxisTimeFormat:      OptionalEnumPointer(linesMulti.XAxisTimeFormat, dashboardSchemaToProtoXAxisTimeFormat),
	}, nil
}

func expandDynamicTimeSeriesBars(ctx context.Context, bars *DynamicTimeSeriesBarsModel) (*dashboardservice.TimeSeriesBars, diag.Diagnostics) {
	if bars == nil {
		return nil, nil
	}

	categoryFields, diags := ExpandObservationFields(ctx, bars.CategoryFields)
	if diags.HasError() {
		return nil, diags
	}

	valueFields, diags := ExpandObservationFields(ctx, bars.ValueFields)
	if diags.HasError() {
		return nil, diags
	}

	temporalField, diags := ExpandObservationFieldObject(ctx, bars.TemporalField)
	if diags.HasError() {
		return nil, diags
	}

	legend, diags := ExpandLegend(ctx, bars.Legend)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.TimeSeriesBars{
		AllowAbbreviation:  bars.AllowAbbreviation.ValueBoolPointer(),
		BarValueDisplay:    OptionalEnumPointer(bars.BarValueDisplay, dashboardSchemaToProtoBarValueDisplay),
		CategoryFields:     categoryFields,
		ColorScheme:        bars.ColorScheme.ValueStringPointer(),
		CustomUnit:         bars.CustomUnit.ValueStringPointer(),
		DecimalPrecision:   expandInt32Pointer(bars.DecimalPrecision),
		HashColors:         bars.HashColors.ValueBoolPointer(),
		Legend:             legend,
		MaxSlicesPerBar:    expandInt32Pointer(bars.MaxSlicesPerBar),
		ScaleType:          OptionalEnumPointer(bars.ScaleType, DashboardSchemaToProtoScaleType),
		SeriesNameTemplate: bars.SeriesNameTemplate.ValueStringPointer(),
		SortBy:             OptionalEnumPointer(bars.SortBy, DashboardSchemaToProtoSortBy),
		TemporalField:      temporalField,
		Tooltip:            expandDynamicTimeSeriesTooltip(bars.Tooltip),
		Unit:               OptionalEnumPointer(bars.Unit, DashboardSchemaToProtoUnit),
		ValueFields:        valueFields,
		XAxisTimeFormat:    OptionalEnumPointer(bars.XAxisTimeFormat, dashboardSchemaToProtoXAxisTimeFormat),
		YAxisMax:           expandFloat32Pointer(bars.YAxisMax),
		YAxisMin:           expandFloat32Pointer(bars.YAxisMin),
	}, nil
}

func expandDynamicQueryDisplaySettings(ctx context.Context, settings types.List) ([]dashboardservice.QueryDisplaySettings, diag.Diagnostics) {
	var models []DynamicQueryDisplaySettingsModel
	diags := settings.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	expanded := make([]dashboardservice.QueryDisplaySettings, 0, len(models))
	for i := range models {
		categoryFields, dg := ExpandObservationFields(ctx, models[i].CategoryFields)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		valueFields, dg := ExpandObservationFields(ctx, models[i].ValueFields)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		temporalField, dg := ExpandObservationFieldObject(ctx, models[i].TemporalField)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expanded = append(expanded, dashboardservice.QueryDisplaySettings{
			AllowAbbreviation:  models[i].AllowAbbreviation.ValueBoolPointer(),
			CategoryFields:     categoryFields,
			ColorScheme:        models[i].ColorScheme.ValueStringPointer(),
			CustomUnit:         models[i].CustomUnit.ValueStringPointer(),
			DecimalPrecision:   expandInt32Pointer(models[i].DecimalPrecision),
			HashColors:         models[i].HashColors.ValueBoolPointer(),
			QueryId:            models[i].QueryID.ValueString(),
			ScaleType:          OptionalEnumPointer(models[i].ScaleType, DashboardSchemaToProtoScaleType),
			SeriesCountLimit:   int64ToStringPointer(models[i].SeriesCountLimit),
			SeriesNameTemplate: models[i].SeriesNameTemplate.ValueStringPointer(),
			TemporalField:      temporalField,
			Unit:               OptionalEnumPointer(models[i].Unit, DashboardSchemaToProtoUnit),
			ValueFields:        valueFields,
			YAxisMax:           expandFloat32Pointer(models[i].YAxisMax),
			YAxisMin:           expandFloat32Pointer(models[i].YAxisMin),
		})
	}

	return expanded, diags
}

func expandDynamicTimeSeriesTooltip(tooltip *DynamicTimeSeriesTooltipModel) *dashboardservice.TimeSeriesTooltip {
	if tooltip == nil {
		return nil
	}

	return &dashboardservice.TimeSeriesTooltip{
		ShowAllSeries: tooltip.ShowAllSeries.ValueBoolPointer(),
		ShowLabels:    tooltip.ShowLabels.ValueBoolPointer(),
	}
}

func expandFloat32Pointer(value Float32Value) *float32 {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	converted := float32(value.ValueFloat64())
	return &converted
}

func expandDynamicTable(ctx context.Context, table *DynamicTableModel) (*dashboardservice.Table, diag.Diagnostics) {
	if table == nil {
		return nil, nil
	}

	columns, diags := expandDynamicTableColumns(ctx, table.Columns)
	if diags.HasError() {
		return nil, diags
	}

	rules, diags := expandDynamicTableRules(ctx, table.Rules)
	if diags.HasError() {
		return nil, diags
	}

	settings, diags := expandDynamicTableSettings(ctx, table.Settings)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.Table{
		Columns:  columns,
		Rules:    rules,
		Settings: settings,
	}, nil
}

func expandDynamicTableColumns(ctx context.Context, columns types.List) ([]dashboardservice.TableColumn, diag.Diagnostics) {
	var models []DynamicTableColumnModel
	diags := columns.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	expanded := make([]dashboardservice.TableColumn, 0, len(models))
	for i := range models {
		field, dg := ExpandObservationFieldObject(ctx, models[i].Field)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expanded = append(expanded, dashboardservice.TableColumn{Field: field})
	}

	return expanded, diags
}

func expandDynamicTableRules(ctx context.Context, rules types.List) ([]dashboardservice.TableRule, diag.Diagnostics) {
	var models []DynamicTableRuleModel
	diags := rules.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	expanded := make([]dashboardservice.TableRule, 0, len(models))
	for i := range models {
		properties, dg := expandDynamicTableProperties(ctx, models[i].Properties)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		ruleScope, dg := expandDynamicTableRuleScope(ctx, models[i].RuleScope)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expanded = append(expanded, dashboardservice.TableRule{
			Description: models[i].Description.ValueStringPointer(),
			Id:          ExpandDashboardUUID(models[i].ID),
			Name:        models[i].Name.ValueStringPointer(),
			Properties:  properties,
			RuleScope:   ruleScope,
		})
	}

	return expanded, diags
}

func expandDynamicTableProperties(ctx context.Context, properties types.List) ([]dashboardservice.Property, diag.Diagnostics) {
	var models []DynamicTablePropertyModel
	diags := properties.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	expanded := make([]dashboardservice.Property, 0, len(models))
	for i := range models {
		definition, dg := expandDynamicTablePropertyDefinition(ctx, models[i].Definition)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expanded = append(expanded, dashboardservice.Property{
			Id:         ExpandDashboardUUID(models[i].ID),
			Definition: definition,
		})
	}

	return expanded, diags
}

func expandDynamicTablePropertyDefinition(ctx context.Context, definition *DynamicTablePropertyDefinitionModel) (*dashboardservice.PropertyDefinition, diag.Diagnostics) {
	if definition == nil {
		return nil, nil
	}

	link, diags := expandDynamicTablePropertyLink(ctx, definition.Link)
	if diags.HasError() {
		return nil, diags
	}

	thresholds, diags := expandDynamicTablePropertyThresholds(ctx, definition.Thresholds)
	if diags.HasError() {
		return nil, diags
	}

	valuesMapping, diags := expandDynamicTablePropertyValuesMapping(ctx, definition.ValuesMapping)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.PropertyDefinition{
		Alignment:         OptionalEnumPointer(definition.Alignment, dashboardSchemaToProtoTextAlignment),
		ColumnDisplayName: definition.ColumnDisplayName.ValueStringPointer(),
		Link:              link,
		RegexExtract:      definition.RegexExtract.ValueStringPointer(),
		Thresholds:        thresholds,
		Units:             expandDynamicTablePropertyUnits(definition.Units),
		ValuesAlias:       definition.ValuesAlias.ValueStringPointer(),
		ValuesMapping:     valuesMapping,
	}, nil
}

func expandDynamicTablePropertyLink(ctx context.Context, link *DynamicTablePropertyLinkModel) (*dashboardservice.PropertyLinks, diag.Diagnostics) {
	if link == nil {
		return nil, nil
	}

	var models []DynamicTableLinkActionModel
	diags := link.Actions.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	actions := make([]dashboardservice.LinkAction, 0, len(models))
	for i := range models {
		actions = append(actions, dashboardservice.LinkAction{
			Id:                    ExpandDashboardUUID(models[i].ID),
			Name:                  models[i].Name.ValueStringPointer(),
			ShouldOpenInNewWindow: models[i].ShouldOpenInNewWindow.ValueBoolPointer(),
			Url:                   models[i].Url.ValueStringPointer(),
		})
	}

	return &dashboardservice.PropertyLinks{Actions: actions}, diags
}

func expandDynamicTablePropertyThresholds(ctx context.Context, thresholds *DynamicTablePropertyThresholdsModel) (*dashboardservice.PropertyThresholds, diag.Diagnostics) {
	if thresholds == nil {
		return nil, nil
	}

	values, diags := expandDynamicThresholds(ctx, thresholds.Values)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.PropertyThresholds{
		Max:    thresholds.Max.ValueFloat64Pointer(),
		Min:    thresholds.Min.ValueFloat64Pointer(),
		Type:   OptionalEnumPointer(thresholds.Type, DashboardSchemaToProtoThresholdType),
		Values: values,
	}, nil
}

func expandDynamicTablePropertyUnits(units *DynamicTablePropertyUnitsModel) *dashboardservice.PropertyUnits {
	if units == nil {
		return nil
	}

	return &dashboardservice.PropertyUnits{
		AllowAbbreviation: units.AllowAbbreviation.ValueBoolPointer(),
		CustomUnit:        units.CustomUnit.ValueStringPointer(),
		DecimalPrecision:  expandInt32Pointer(units.DecimalPrecision),
		Max:               units.Max.ValueFloat64Pointer(),
		Min:               units.Min.ValueFloat64Pointer(),
		Unit:              OptionalEnumPointer(units.Unit, DashboardSchemaToProtoUnit),
	}
}

func expandDynamicTablePropertyValuesMapping(ctx context.Context, valuesMapping *DynamicTablePropertyValuesMappingModel) (*dashboardservice.PropertyValuesMapping, diag.Diagnostics) {
	if valuesMapping == nil {
		return nil, nil
	}

	var models []DynamicTableValueMappingModel
	diags := valuesMapping.Mappings.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	mappings := make([]dashboardservice.PropertyValuesMappingValueMapping, 0, len(models))
	for i := range models {
		mappings = append(mappings, dashboardservice.PropertyValuesMappingValueMapping{
			InputValue:   models[i].InputValue.ValueStringPointer(),
			ReplaceValue: models[i].ReplaceValue.ValueStringPointer(),
			Type:         OptionalEnumPointer(models[i].Type, dashboardSchemaToProtoValuesMappingType),
		})
	}

	return &dashboardservice.PropertyValuesMapping{Mappings: mappings}, diags
}

func expandDynamicTableRuleScope(ctx context.Context, ruleScope *DynamicTableRuleScopeModel) (*dashboardservice.RuleScope, diag.Diagnostics) {
	if ruleScope == nil {
		return nil, nil
	}

	field, diags := ExpandObservationFieldObject(ctx, ruleScope.Field)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.RuleScope{
		Field:     field,
		FieldType: OptionalEnumPointer(ruleScope.FieldType, dashboardSchemaToProtoFieldDataType),
		Regex:     ruleScope.Regex.ValueStringPointer(),
	}, nil
}

func expandDynamicTableSettings(ctx context.Context, settings *DynamicTableSettingsModel) (*dashboardservice.TableSettings, diag.Diagnostics) {
	if settings == nil {
		return nil, nil
	}

	var models []DynamicTableColumnWidthModel
	diags := settings.ColumnWidths.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	columnWidths := make([]dashboardservice.ColumnWidthEntry, 0, len(models))
	for i := range models {
		columnWidths = append(columnWidths, dashboardservice.ColumnWidthEntry{
			ColumnName: models[i].ColumnName.ValueStringPointer(),
			Width:      expandInt32Pointer(models[i].Width),
		})
	}

	return &dashboardservice.TableSettings{
		ColumnWidths: columnWidths,
		RowStyle:     OptionalEnumPointer(settings.RowStyle, DashboardRowStyleSchemaToProto),
	}, diags
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

func expandDynamicStatCard(ctx context.Context, statCard *DynamicStatCardModel) (*dashboardservice.StatCard, diag.Diagnostics) {
	if statCard == nil {
		return nil, nil
	}

	categoryFields, diags := ExpandObservationFields(ctx, statCard.CategoryFields)
	if diags.HasError() {
		return nil, diags
	}

	valueFields, diags := ExpandObservationFields(ctx, statCard.ValueFields)
	if diags.HasError() {
		return nil, diags
	}

	colorLabelMapping, diags := expandDynamicColorLabelMapping(ctx, statCard.ColorLabelMapping)
	if diags.HasError() {
		return nil, diags
	}

	label, diags := expandDynamicStatVisualElement(ctx, statCard.Label)
	if diags.HasError() {
		return nil, diags
	}

	primaryValue, diags := expandDynamicStatVisualElement(ctx, statCard.PrimaryValue)
	if diags.HasError() {
		return nil, diags
	}

	title, diags := expandDynamicStatVisualElement(ctx, statCard.Title)
	if diags.HasError() {
		return nil, diags
	}

	legend, diags := ExpandLegend(ctx, statCard.Legend)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.StatCard{
		AllowAbbreviation: statCard.AllowAbbreviation.ValueBoolPointer(),
		CategoryFields:    categoryFields,
		ColorLabelMapping: colorLabelMapping,
		CustomUnit:        statCard.CustomUnit.ValueStringPointer(),
		DecimalPrecision:  expandInt32Pointer(statCard.DecimalPrecision),
		Label:             label,
		Legend:            legend,
		LegendBy:          OptionalEnumPointer(statCard.LegendBy, DashboardSchemaToProtoLegendBy),
		PrimaryValue:      primaryValue,
		Title:             title,
		Unit:              OptionalEnumPointer(statCard.Unit, DashboardSchemaToProtoUnit),
		ValueFields:       valueFields,
	}, nil
}

func expandDynamicStatVisualElement(ctx context.Context, element *DynamicStatVisualElementModel) (*dashboardservice.StatVisualElement, diag.Diagnostics) {
	if element == nil {
		return nil, nil
	}

	observationField, diags := ExpandObservationFieldObject(ctx, element.ObservationField)
	if diags.HasError() {
		return nil, diags
	}

	mappedValues, diags := expandJSONStringToMap("mapped_values", element.MappedValues)
	if diags.HasError() {
		return nil, diags
	}

	templateVariables, diags := expandDynamicTemplateVariables(ctx, element.TemplateVariables)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.StatVisualElement{
		MappedValues:      mappedValues,
		ObservationField:  observationField,
		TemplateText:      element.TemplateText.ValueStringPointer(),
		TemplateVariables: templateVariables,
	}, nil
}

func expandDynamicTemplateVariables(ctx context.Context, variables types.List) ([]dashboardservice.DisplayNameTemplateVariable, diag.Diagnostics) {
	var models []DynamicTemplateVariableModel
	diags := variables.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	expanded := make([]dashboardservice.DisplayNameTemplateVariable, 0, len(models))
	for i := range models {
		observationField, dg := ExpandObservationFieldObject(ctx, models[i].ObservationField)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		mappedValues, dg := expandJSONStringToMap("mapped_values", models[i].MappedValues)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expanded = append(expanded, dashboardservice.DisplayNameTemplateVariable{
			MappedValues:     mappedValues,
			ObservationField: observationField,
		})
	}

	return expanded, diags
}

func expandDynamicColorLabelMapping(ctx context.Context, mapping *DynamicColorLabelMappingModel) (*dashboardservice.ColorLabelMapping, diag.Diagnostics) {
	if mapping == nil {
		return nil, nil
	}

	rangeMapping, diags := expandDynamicRangeMapping(ctx, mapping.Range)
	if diags.HasError() {
		return nil, diags
	}

	regex, diags := expandDynamicSectionsRegex(ctx, mapping.Regex)
	if diags.HasError() {
		return nil, diags
	}

	value, diags := expandDynamicSectionsValue(ctx, mapping.Value)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.ColorLabelMapping{
		ColorBy: OptionalEnumPointer(mapping.ColorBy, dashboardSchemaToProtoColorApplyTarget),
		Range:   rangeMapping,
		Regex:   regex,
		Value:   value,
	}, nil
}

func expandDynamicRangeMapping(ctx context.Context, rangeMapping *DynamicRangeMappingModel) (*dashboardservice.RangeMapping, diag.Diagnostics) {
	if rangeMapping == nil {
		return nil, nil
	}

	thresholds, diags := expandDynamicThresholds(ctx, rangeMapping.Thresholds)
	if diags.HasError() {
		return nil, diags
	}

	var minMax *dashboardservice.MinMax
	if rangeMapping.MinMax != nil {
		minMax = &dashboardservice.MinMax{}
		if rangeMapping.MinMax.Auto.ValueBool() {
			minMax.Auto = map[string]interface{}{}
		}
		if rangeMapping.MinMax.Custom != nil {
			minMax.Custom = &dashboardservice.MinMaxCustom{
				Max: rangeMapping.MinMax.Custom.Max.ValueFloat64Pointer(),
				Min: rangeMapping.MinMax.Custom.Min.ValueFloat64Pointer(),
			}
		}
	}

	return &dashboardservice.RangeMapping{
		MinMax:        minMax,
		ThresholdType: OptionalEnumPointer(rangeMapping.ThresholdType, DashboardSchemaToProtoThresholdType),
		Thresholds:    thresholds,
	}, nil
}

func expandDynamicSectionsRegex(ctx context.Context, mapping *DynamicSectionsMappingModel) (*dashboardservice.RegexMapping, diag.Diagnostics) {
	if mapping == nil {
		return nil, nil
	}
	sections, diags := expandDynamicMappingSections(ctx, mapping.Sections)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboardservice.RegexMapping{Sections: sections}, nil
}

func expandDynamicSectionsValue(ctx context.Context, mapping *DynamicSectionsMappingModel) (*dashboardservice.ColorLabelMappingValueMapping, diag.Diagnostics) {
	if mapping == nil {
		return nil, nil
	}
	sections, diags := expandDynamicMappingSections(ctx, mapping.Sections)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboardservice.ColorLabelMappingValueMapping{Sections: sections}, nil
}

func expandDynamicMappingSections(ctx context.Context, sections types.List) ([]dashboardservice.MappingSection, diag.Diagnostics) {
	var models []DynamicMappingSectionModel
	diags := sections.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	expanded := make([]dashboardservice.MappingSection, 0, len(models))
	for i := range models {
		expanded = append(expanded, dashboardservice.MappingSection{
			Color: OptionalEnumPointer(models[i].Color, dashboardSchemaToProtoColorSolidType),
			MapTo: models[i].MapTo.ValueStringPointer(),
			Value: models[i].Value.ValueStringPointer(),
		})
	}

	return expanded, diags
}

func expandInt32Pointer(value types.Int64) *int32 {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	converted := int32(value.ValueInt64())
	return &converted
}

func expandJSONStringToMap(attributeName string, value JSONStringValue) (map[string]interface{}, diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return nil, nil
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(value.ValueString()), &parsed); err != nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic(fmt.Sprintf("Invalid %s JSON", attributeName), err.Error())}
	}
	return parsed, nil
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

func flattenSpanObservationFieldObject(ctx context.Context, field *dashboardservice.SpanObservationField) (types.Object, diag.Diagnostics) {
	if field == nil {
		return types.ObjectNull(spanObservationFieldAttr()), nil
	}

	model := &SpanObservationFieldModel{
		Keypath:      utils.StringSliceToTypeStringList(field.GetKeypath()),
		Scope:        flattenOptionalEnum(field.Scope, DashboardProtoToSchemaObservationFieldScope),
		RelationType: flattenOptionalEnum(field.RelationType, dashboardProtoToSchemaSpanRelationType),
	}

	return types.ObjectValueFrom(ctx, spanObservationFieldAttr(), model)
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

	for _, group := range []func(context.Context, *dashboardservice.Visualization) (*DynamicVisualizationModel, diag.Diagnostics, bool){
		flattenDynamicVisualizationStatGroup,
		flattenDynamicVisualizationChartGroup,
		flattenDynamicVisualizationGeoGroup,
	} {
		if model, diags, handled := group(ctx, visualization); handled {
			return model, diags
		}
	}

	return nil, diag.Diagnostics{diag.NewErrorDiagnostic(
		"Unsupported Dashboard Widget Definition",
		"The dynamic widget uses an unknown or unsupported visualization variant.",
	)}
}

func flattenDynamicVisualizationStatGroup(ctx context.Context, visualization *dashboardservice.Visualization) (*DynamicVisualizationModel, diag.Diagnostics, bool) {
	switch {
	case visualization.Stat != nil:
		stat, diags := flattenDynamicStat(ctx, visualization.Stat)
		if diags.HasError() {
			return nil, diags, true
		}
		return &DynamicVisualizationModel{Stat: stat}, nil, true
	case visualization.StatCard != nil:
		statCard, diags := flattenDynamicStatCard(ctx, visualization.StatCard)
		if diags.HasError() {
			return nil, diags, true
		}
		return &DynamicVisualizationModel{StatCard: statCard}, nil, true
	case visualization.Table != nil:
		table, diags := flattenDynamicTable(ctx, visualization.Table)
		if diags.HasError() {
			return nil, diags, true
		}
		return &DynamicVisualizationModel{Table: table}, nil, true
	case visualization.Gauge != nil:
		gauge, diags := flattenDynamicGauge(ctx, visualization.Gauge)
		if diags.HasError() {
			return nil, diags, true
		}
		return &DynamicVisualizationModel{Gauge: gauge}, nil, true
	case visualization.PieChart != nil:
		pieChart, diags := flattenDynamicPieChart(ctx, visualization.PieChart)
		if diags.HasError() {
			return nil, diags, true
		}
		return &DynamicVisualizationModel{PieChart: pieChart}, nil, true
	default:
		return nil, nil, false
	}
}

func flattenDynamicVisualizationChartGroup(ctx context.Context, visualization *dashboardservice.Visualization) (*DynamicVisualizationModel, diag.Diagnostics, bool) {
	switch {
	case visualization.TimeSeriesLines != nil:
		lines, diags := flattenDynamicTimeSeriesLines(ctx, visualization.TimeSeriesLines)
		if diags.HasError() {
			return nil, diags, true
		}
		return &DynamicVisualizationModel{TimeSeriesLines: lines}, nil, true
	case visualization.TimeSeriesLinesMulti != nil:
		linesMulti, diags := flattenDynamicTimeSeriesLinesMulti(ctx, visualization.TimeSeriesLinesMulti)
		if diags.HasError() {
			return nil, diags, true
		}
		return &DynamicVisualizationModel{TimeSeriesLinesMulti: linesMulti}, nil, true
	case visualization.TimeSeriesBars != nil:
		bars, diags := flattenDynamicTimeSeriesBars(ctx, visualization.TimeSeriesBars)
		if diags.HasError() {
			return nil, diags, true
		}
		return &DynamicVisualizationModel{TimeSeriesBars: bars}, nil, true
	case visualization.VerticalBars != nil:
		verticalBars, diags := flattenDynamicVerticalBars(ctx, visualization.VerticalBars)
		if diags.HasError() {
			return nil, diags, true
		}
		return &DynamicVisualizationModel{VerticalBars: verticalBars}, nil, true
	case visualization.VerticalBarsMulti != nil:
		verticalBarsMulti, diags := flattenDynamicVerticalBarsMulti(ctx, visualization.VerticalBarsMulti)
		if diags.HasError() {
			return nil, diags, true
		}
		return &DynamicVisualizationModel{VerticalBarsMulti: verticalBarsMulti}, nil, true
	case visualization.HorizontalBars != nil:
		horizontalBars, diags := flattenDynamicHorizontalBars(ctx, visualization.HorizontalBars)
		if diags.HasError() {
			return nil, diags, true
		}
		return &DynamicVisualizationModel{HorizontalBars: horizontalBars}, nil, true
	case visualization.HorizontalBarsMulti != nil:
		horizontalBarsMulti, diags := flattenDynamicHorizontalBarsMulti(ctx, visualization.HorizontalBarsMulti)
		if diags.HasError() {
			return nil, diags, true
		}
		return &DynamicVisualizationModel{HorizontalBarsMulti: horizontalBarsMulti}, nil, true
	default:
		return nil, nil, false
	}
}

func flattenDynamicVisualizationGeoGroup(ctx context.Context, visualization *dashboardservice.Visualization) (*DynamicVisualizationModel, diag.Diagnostics, bool) {
	switch {
	case visualization.HexagonBins != nil:
		hexagonBins, diags := flattenDynamicHexagonBins(ctx, visualization.HexagonBins)
		if diags.HasError() {
			return nil, diags, true
		}
		return &DynamicVisualizationModel{HexagonBins: hexagonBins}, nil, true
	case visualization.Heatmap != nil:
		heatmap, diags := flattenDynamicHeatmap(ctx, visualization.Heatmap)
		if diags.HasError() {
			return nil, diags, true
		}
		return &DynamicVisualizationModel{Heatmap: heatmap}, nil, true
	case visualization.Geomap != nil:
		geomap, diags := flattenDynamicGeomap(ctx, visualization.Geomap)
		if diags.HasError() {
			return nil, diags, true
		}
		return &DynamicVisualizationModel{Geomap: geomap}, nil, true
	default:
		return nil, nil, false
	}
}

func flattenDynamicTimeSeriesLines(ctx context.Context, lines *dashboardservice.TimeSeriesLines) (*DynamicTimeSeriesLinesModel, diag.Diagnostics) {
	if lines == nil {
		return nil, nil
	}

	categoryFields, diags := FlattenObservationFields(ctx, lines.GetCategoryFields())
	if diags.HasError() {
		return nil, diags
	}

	valueFields, diags := FlattenObservationFields(ctx, lines.GetValueFields())
	if diags.HasError() {
		return nil, diags
	}

	temporalField, diags := FlattenObservationField(ctx, lines.TemporalField)
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicTimeSeriesLinesModel{
		AllowAbbreviation:  types.BoolPointerValue(lines.AllowAbbreviation),
		CategoryFields:     categoryFields,
		ColorScheme:        types.StringPointerValue(lines.ColorScheme),
		ConnectNulls:       types.BoolPointerValue(lines.ConnectNulls),
		CustomUnit:         types.StringPointerValue(lines.CustomUnit),
		DecimalPrecision:   flattenInt32Pointer(lines.DecimalPrecision),
		HashColors:         types.BoolPointerValue(lines.HashColors),
		Legend:             FlattenLegend(lines.Legend),
		ScaleType:          flattenOptionalEnum(lines.ScaleType, DashboardProtoToSchemaScaleType),
		SeriesCountLimit:   stringPointerToInt64(lines.SeriesCountLimit),
		SeriesNameTemplate: types.StringPointerValue(lines.SeriesNameTemplate),
		StackedLine:        flattenOptionalEnum(lines.StackedLine, dashboardProtoToSchemaStackedLine),
		TemporalField:      temporalField,
		Tooltip:            flattenDynamicTimeSeriesTooltip(lines.Tooltip),
		Unit:               flattenOptionalEnum(lines.Unit, DashboardProtoToSchemaUnit),
		UseDataTimeRange:   types.BoolPointerValue(lines.UseDataTimeRange),
		ValueFields:        valueFields,
		XAxisTimeFormat:    flattenOptionalEnum(lines.XAxisTimeFormat, dashboardProtoToSchemaXAxisTimeFormat),
		YAxisMax:           flattenFloat32Pointer(lines.YAxisMax),
		YAxisMin:           flattenFloat32Pointer(lines.YAxisMin),
	}, nil
}

func flattenDynamicTimeSeriesLinesMulti(ctx context.Context, linesMulti *dashboardservice.TimeSeriesLinesMulti) (*DynamicTimeSeriesLinesMultiModel, diag.Diagnostics) {
	if linesMulti == nil {
		return nil, nil
	}

	queryDisplaySettings, diags := flattenDynamicQueryDisplaySettings(ctx, linesMulti.GetQueryDisplaySettings())
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicTimeSeriesLinesMultiModel{
		ConnectNulls:         types.BoolPointerValue(linesMulti.ConnectNulls),
		Legend:               FlattenLegend(linesMulti.Legend),
		QueryDisplaySettings: queryDisplaySettings,
		StackedLine:          flattenOptionalEnum(linesMulti.StackedLine, dashboardProtoToSchemaStackedLine),
		Tooltip:              flattenDynamicTimeSeriesTooltip(linesMulti.Tooltip),
		UseDataTimeRange:     types.BoolPointerValue(linesMulti.UseDataTimeRange),
		XAxisTimeFormat:      flattenOptionalEnum(linesMulti.XAxisTimeFormat, dashboardProtoToSchemaXAxisTimeFormat),
	}, nil
}

func flattenDynamicTimeSeriesBars(ctx context.Context, bars *dashboardservice.TimeSeriesBars) (*DynamicTimeSeriesBarsModel, diag.Diagnostics) {
	if bars == nil {
		return nil, nil
	}

	categoryFields, diags := FlattenObservationFields(ctx, bars.GetCategoryFields())
	if diags.HasError() {
		return nil, diags
	}

	valueFields, diags := FlattenObservationFields(ctx, bars.GetValueFields())
	if diags.HasError() {
		return nil, diags
	}

	temporalField, diags := FlattenObservationField(ctx, bars.TemporalField)
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicTimeSeriesBarsModel{
		AllowAbbreviation:  types.BoolPointerValue(bars.AllowAbbreviation),
		BarValueDisplay:    flattenOptionalEnum(bars.BarValueDisplay, dashboardProtoToSchemaBarValueDisplay),
		CategoryFields:     categoryFields,
		ColorScheme:        types.StringPointerValue(bars.ColorScheme),
		CustomUnit:         types.StringPointerValue(bars.CustomUnit),
		DecimalPrecision:   flattenInt32Pointer(bars.DecimalPrecision),
		HashColors:         types.BoolPointerValue(bars.HashColors),
		Legend:             FlattenLegend(bars.Legend),
		MaxSlicesPerBar:    flattenInt32Pointer(bars.MaxSlicesPerBar),
		ScaleType:          flattenOptionalEnum(bars.ScaleType, DashboardProtoToSchemaScaleType),
		SeriesNameTemplate: types.StringPointerValue(bars.SeriesNameTemplate),
		SortBy:             flattenOptionalEnum(bars.SortBy, DashboardProtoToSchemaSortBy),
		TemporalField:      temporalField,
		Tooltip:            flattenDynamicTimeSeriesTooltip(bars.Tooltip),
		Unit:               flattenOptionalEnum(bars.Unit, DashboardProtoToSchemaUnit),
		ValueFields:        valueFields,
		XAxisTimeFormat:    flattenOptionalEnum(bars.XAxisTimeFormat, dashboardProtoToSchemaXAxisTimeFormat),
		YAxisMax:           flattenFloat32Pointer(bars.YAxisMax),
		YAxisMin:           flattenFloat32Pointer(bars.YAxisMin),
	}, nil
}

func flattenDynamicQueryDisplaySettings(ctx context.Context, settings []dashboardservice.QueryDisplaySettings) (types.List, diag.Diagnostics) {
	if len(settings) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicQueryDisplaySettingsModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	elements := make([]attr.Value, 0, len(settings))
	for i := range settings {
		categoryFields, dg := FlattenObservationFields(ctx, settings[i].GetCategoryFields())
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		valueFields, dg := FlattenObservationFields(ctx, settings[i].GetValueFields())
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		temporalField, dg := FlattenObservationField(ctx, settings[i].TemporalField)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		model := &DynamicQueryDisplaySettingsModel{
			AllowAbbreviation:  types.BoolPointerValue(settings[i].AllowAbbreviation),
			CategoryFields:     categoryFields,
			ColorScheme:        types.StringPointerValue(settings[i].ColorScheme),
			CustomUnit:         types.StringPointerValue(settings[i].CustomUnit),
			DecimalPrecision:   flattenInt32Pointer(settings[i].DecimalPrecision),
			HashColors:         types.BoolPointerValue(settings[i].HashColors),
			QueryID:            types.StringValue(settings[i].QueryId),
			ScaleType:          flattenOptionalEnum(settings[i].ScaleType, DashboardProtoToSchemaScaleType),
			SeriesCountLimit:   stringPointerToInt64(settings[i].SeriesCountLimit),
			SeriesNameTemplate: types.StringPointerValue(settings[i].SeriesNameTemplate),
			TemporalField:      temporalField,
			Unit:               flattenOptionalEnum(settings[i].Unit, DashboardProtoToSchemaUnit),
			ValueFields:        valueFields,
			YAxisMax:           flattenFloat32Pointer(settings[i].YAxisMax),
			YAxisMin:           flattenFloat32Pointer(settings[i].YAxisMin),
		}
		element, dg := types.ObjectValueFrom(ctx, dynamicQueryDisplaySettingsModelAttr(), model)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		elements = append(elements, element)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicQueryDisplaySettingsModelAttr()}), diagnostics
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicQueryDisplaySettingsModelAttr()}, elements)
}

func flattenDynamicTimeSeriesTooltip(tooltip *dashboardservice.TimeSeriesTooltip) *DynamicTimeSeriesTooltipModel {
	if tooltip == nil {
		return nil
	}

	return &DynamicTimeSeriesTooltipModel{
		ShowAllSeries: types.BoolPointerValue(tooltip.ShowAllSeries),
		ShowLabels:    types.BoolPointerValue(tooltip.ShowLabels),
	}
}

func flattenFloat32Pointer(value *float32) Float32Value {
	if value == nil {
		return Float32Value{Float64Value: basetypes.NewFloat64Null()}
	}
	return Float32Value{Float64Value: basetypes.NewFloat64Value(float64(*value))}
}

func flattenDynamicTable(ctx context.Context, table *dashboardservice.Table) (*DynamicTableModel, diag.Diagnostics) {
	if table == nil {
		return nil, nil
	}

	columns, diags := flattenDynamicTableColumns(ctx, table.GetColumns())
	if diags.HasError() {
		return nil, diags
	}

	rules, diags := flattenDynamicTableRules(ctx, table.GetRules())
	if diags.HasError() {
		return nil, diags
	}

	settings, diags := flattenDynamicTableSettings(ctx, table.Settings)
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicTableModel{
		Columns:  columns,
		Rules:    rules,
		Settings: settings,
	}, nil
}

func flattenDynamicTableColumns(ctx context.Context, columns []dashboardservice.TableColumn) (types.List, diag.Diagnostics) {
	if len(columns) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicTableColumnModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	elements := make([]attr.Value, 0, len(columns))
	for i := range columns {
		field, dg := FlattenObservationField(ctx, columns[i].Field)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		element, dg := types.ObjectValueFrom(ctx, dynamicTableColumnModelAttr(), &DynamicTableColumnModel{Field: field})
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		elements = append(elements, element)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicTableColumnModelAttr()}), diagnostics
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicTableColumnModelAttr()}, elements)
}

func flattenDynamicTableRules(ctx context.Context, rules []dashboardservice.TableRule) (types.List, diag.Diagnostics) {
	if len(rules) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicTableRuleModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	elements := make([]attr.Value, 0, len(rules))
	for i := range rules {
		properties, dg := flattenDynamicTableProperties(ctx, rules[i].GetProperties())
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		ruleScope, dg := flattenDynamicTableRuleScope(ctx, rules[i].RuleScope)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		model := &DynamicTableRuleModel{
			Description: types.StringPointerValue(rules[i].Description),
			ID:          flattenDashboardUUID(rules[i].Id),
			Name:        types.StringPointerValue(rules[i].Name),
			Properties:  properties,
			RuleScope:   ruleScope,
		}
		element, dg := types.ObjectValueFrom(ctx, dynamicTableRuleModelAttr(), model)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		elements = append(elements, element)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicTableRuleModelAttr()}), diagnostics
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicTableRuleModelAttr()}, elements)
}

func flattenDynamicTableProperties(ctx context.Context, properties []dashboardservice.Property) (types.List, diag.Diagnostics) {
	if len(properties) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicTablePropertyModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	elements := make([]attr.Value, 0, len(properties))
	for i := range properties {
		definition, dg := flattenDynamicTablePropertyDefinition(ctx, properties[i].Definition)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		model := &DynamicTablePropertyModel{
			ID:         flattenDashboardUUID(properties[i].Id),
			Definition: definition,
		}
		element, dg := types.ObjectValueFrom(ctx, dynamicTablePropertyModelAttr(), model)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		elements = append(elements, element)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicTablePropertyModelAttr()}), diagnostics
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicTablePropertyModelAttr()}, elements)
}

func flattenDynamicTablePropertyDefinition(ctx context.Context, definition *dashboardservice.PropertyDefinition) (*DynamicTablePropertyDefinitionModel, diag.Diagnostics) {
	if definition == nil {
		return nil, nil
	}

	link, diags := flattenDynamicTablePropertyLink(ctx, definition.Link)
	if diags.HasError() {
		return nil, diags
	}

	thresholds, diags := flattenDynamicTablePropertyThresholds(ctx, definition.Thresholds)
	if diags.HasError() {
		return nil, diags
	}

	valuesMapping, diags := flattenDynamicTablePropertyValuesMapping(ctx, definition.ValuesMapping)
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicTablePropertyDefinitionModel{
		Alignment:         flattenOptionalEnum(definition.Alignment, dashboardProtoToSchemaTextAlignment),
		ColumnDisplayName: types.StringPointerValue(definition.ColumnDisplayName),
		Link:              link,
		RegexExtract:      types.StringPointerValue(definition.RegexExtract),
		Thresholds:        thresholds,
		Units:             flattenDynamicTablePropertyUnits(definition.Units),
		ValuesAlias:       types.StringPointerValue(definition.ValuesAlias),
		ValuesMapping:     valuesMapping,
	}, nil
}

func flattenDynamicTablePropertyLink(ctx context.Context, link *dashboardservice.PropertyLinks) (*DynamicTablePropertyLinkModel, diag.Diagnostics) {
	if link == nil {
		return nil, nil
	}

	actions := link.GetActions()
	if len(actions) == 0 {
		return &DynamicTablePropertyLinkModel{
			Actions: types.ListNull(types.ObjectType{AttrTypes: dynamicTableLinkActionModelAttr()}),
		}, nil
	}

	elements := make([]attr.Value, 0, len(actions))
	var diagnostics diag.Diagnostics
	for i := range actions {
		model := &DynamicTableLinkActionModel{
			ID:                    flattenDashboardUUID(actions[i].Id),
			Name:                  types.StringPointerValue(actions[i].Name),
			ShouldOpenInNewWindow: types.BoolPointerValue(actions[i].ShouldOpenInNewWindow),
			Url:                   types.StringPointerValue(actions[i].Url),
		}
		element, dg := types.ObjectValueFrom(ctx, dynamicTableLinkActionModelAttr(), model)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		elements = append(elements, element)
	}
	if diagnostics.HasError() {
		return nil, diagnostics
	}

	list, dg := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicTableLinkActionModelAttr()}, elements)
	if dg.HasError() {
		return nil, dg
	}
	return &DynamicTablePropertyLinkModel{Actions: list}, nil
}

func flattenDynamicTablePropertyThresholds(ctx context.Context, thresholds *dashboardservice.PropertyThresholds) (*DynamicTablePropertyThresholdsModel, diag.Diagnostics) {
	if thresholds == nil {
		return nil, nil
	}

	values, diags := flattenDynamicThresholds(ctx, thresholds.GetValues())
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicTablePropertyThresholdsModel{
		Max:    types.Float64PointerValue(thresholds.Max),
		Min:    types.Float64PointerValue(thresholds.Min),
		Type:   flattenOptionalEnum(thresholds.Type, DashboardProtoToSchemaThresholdType),
		Values: values,
	}, nil
}

func flattenDynamicTablePropertyUnits(units *dashboardservice.PropertyUnits) *DynamicTablePropertyUnitsModel {
	if units == nil {
		return nil
	}

	return &DynamicTablePropertyUnitsModel{
		AllowAbbreviation: types.BoolPointerValue(units.AllowAbbreviation),
		CustomUnit:        types.StringPointerValue(units.CustomUnit),
		DecimalPrecision:  flattenInt32Pointer(units.DecimalPrecision),
		Max:               types.Float64PointerValue(units.Max),
		Min:               types.Float64PointerValue(units.Min),
		Unit:              flattenOptionalEnum(units.Unit, DashboardProtoToSchemaUnit),
	}
}

func flattenDynamicTablePropertyValuesMapping(ctx context.Context, valuesMapping *dashboardservice.PropertyValuesMapping) (*DynamicTablePropertyValuesMappingModel, diag.Diagnostics) {
	if valuesMapping == nil {
		return nil, nil
	}

	mappings := valuesMapping.GetMappings()
	if len(mappings) == 0 {
		return &DynamicTablePropertyValuesMappingModel{
			Mappings: types.ListNull(types.ObjectType{AttrTypes: dynamicTableValueMappingModelAttr()}),
		}, nil
	}

	elements := make([]attr.Value, 0, len(mappings))
	var diagnostics diag.Diagnostics
	for i := range mappings {
		model := &DynamicTableValueMappingModel{
			InputValue:   types.StringPointerValue(mappings[i].InputValue),
			ReplaceValue: types.StringPointerValue(mappings[i].ReplaceValue),
			Type:         flattenOptionalEnum(mappings[i].Type, dashboardProtoToSchemaValuesMappingType),
		}
		element, dg := types.ObjectValueFrom(ctx, dynamicTableValueMappingModelAttr(), model)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		elements = append(elements, element)
	}
	if diagnostics.HasError() {
		return nil, diagnostics
	}

	list, dg := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicTableValueMappingModelAttr()}, elements)
	if dg.HasError() {
		return nil, dg
	}
	return &DynamicTablePropertyValuesMappingModel{Mappings: list}, nil
}

func flattenDynamicTableRuleScope(ctx context.Context, ruleScope *dashboardservice.RuleScope) (*DynamicTableRuleScopeModel, diag.Diagnostics) {
	if ruleScope == nil {
		return nil, nil
	}

	field, diags := FlattenObservationField(ctx, ruleScope.Field)
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicTableRuleScopeModel{
		Field:     field,
		FieldType: flattenOptionalEnum(ruleScope.FieldType, dashboardProtoToSchemaFieldDataType),
		Regex:     types.StringPointerValue(ruleScope.Regex),
	}, nil
}

func flattenDynamicTableSettings(ctx context.Context, settings *dashboardservice.TableSettings) (*DynamicTableSettingsModel, diag.Diagnostics) {
	if settings == nil {
		return nil, nil
	}

	columnWidths := settings.GetColumnWidths()
	widths := types.ListNull(types.ObjectType{AttrTypes: dynamicTableColumnWidthModelAttr()})
	if len(columnWidths) > 0 {
		elements := make([]attr.Value, 0, len(columnWidths))
		var diagnostics diag.Diagnostics
		for i := range columnWidths {
			model := &DynamicTableColumnWidthModel{
				ColumnName: types.StringPointerValue(columnWidths[i].ColumnName),
				Width:      flattenInt32Pointer(columnWidths[i].Width),
			}
			element, dg := types.ObjectValueFrom(ctx, dynamicTableColumnWidthModelAttr(), model)
			if dg.HasError() {
				diagnostics.Append(dg...)
				continue
			}
			elements = append(elements, element)
		}
		if diagnostics.HasError() {
			return nil, diagnostics
		}
		list, dg := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicTableColumnWidthModelAttr()}, elements)
		if dg.HasError() {
			return nil, dg
		}
		widths = list
	}

	return &DynamicTableSettingsModel{
		ColumnWidths: widths,
		RowStyle:     flattenOptionalEnum(settings.RowStyle, DashboardRowStyleProtoToSchema),
	}, nil
}

func flattenDashboardUUID(id *dashboardservice.UUID) types.String {
	if id == nil {
		return types.StringNull()
	}
	return types.StringPointerValue(id.Value)
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

func flattenDynamicStatCard(ctx context.Context, statCard *dashboardservice.StatCard) (*DynamicStatCardModel, diag.Diagnostics) {
	if statCard == nil {
		return nil, nil
	}

	categoryFields, diags := FlattenObservationFields(ctx, statCard.GetCategoryFields())
	if diags.HasError() {
		return nil, diags
	}

	valueFields, diags := FlattenObservationFields(ctx, statCard.GetValueFields())
	if diags.HasError() {
		return nil, diags
	}

	colorLabelMapping, diags := flattenDynamicColorLabelMapping(ctx, statCard.ColorLabelMapping)
	if diags.HasError() {
		return nil, diags
	}

	label, diags := flattenDynamicStatVisualElement(ctx, statCard.Label)
	if diags.HasError() {
		return nil, diags
	}

	primaryValue, diags := flattenDynamicStatVisualElement(ctx, statCard.PrimaryValue)
	if diags.HasError() {
		return nil, diags
	}

	title, diags := flattenDynamicStatVisualElement(ctx, statCard.Title)
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicStatCardModel{
		AllowAbbreviation: types.BoolPointerValue(statCard.AllowAbbreviation),
		CategoryFields:    categoryFields,
		ColorLabelMapping: colorLabelMapping,
		CustomUnit:        types.StringPointerValue(statCard.CustomUnit),
		DecimalPrecision:  flattenInt32Pointer(statCard.DecimalPrecision),
		Label:             label,
		Legend:            FlattenLegend(statCard.Legend),
		LegendBy:          flattenOptionalEnum(statCard.LegendBy, DashboardProtoToSchemaLegendBy),
		PrimaryValue:      primaryValue,
		Title:             title,
		Unit:              flattenOptionalEnum(statCard.Unit, DashboardProtoToSchemaUnit),
		ValueFields:       valueFields,
	}, nil
}

func flattenDynamicStatVisualElement(ctx context.Context, element *dashboardservice.StatVisualElement) (*DynamicStatVisualElementModel, diag.Diagnostics) {
	if element == nil {
		return nil, nil
	}

	observationField, diags := FlattenObservationField(ctx, element.ObservationField)
	if diags.HasError() {
		return nil, diags
	}

	templateVariables, diags := flattenDynamicTemplateVariables(ctx, element.GetTemplateVariables())
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicStatVisualElementModel{
		MappedValues:      flattenMapToJSONString(element.MappedValues),
		ObservationField:  observationField,
		TemplateText:      types.StringPointerValue(element.TemplateText),
		TemplateVariables: templateVariables,
	}, nil
}

func flattenDynamicTemplateVariables(ctx context.Context, variables []dashboardservice.DisplayNameTemplateVariable) (types.List, diag.Diagnostics) {
	if len(variables) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicTemplateVariableAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	elements := make([]attr.Value, 0, len(variables))
	for i := range variables {
		observationField, dg := FlattenObservationField(ctx, variables[i].ObservationField)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		model := &DynamicTemplateVariableModel{
			MappedValues:     flattenMapToJSONString(variables[i].MappedValues),
			ObservationField: observationField,
		}
		element, dg := types.ObjectValueFrom(ctx, dynamicTemplateVariableAttr(), model)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		elements = append(elements, element)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicTemplateVariableAttr()}), diagnostics
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicTemplateVariableAttr()}, elements)
}

func flattenDynamicColorLabelMapping(ctx context.Context, mapping *dashboardservice.ColorLabelMapping) (*DynamicColorLabelMappingModel, diag.Diagnostics) {
	if mapping == nil {
		return nil, nil
	}

	rangeMapping, diags := flattenDynamicRangeMapping(ctx, mapping.Range)
	if diags.HasError() {
		return nil, diags
	}

	var regex *DynamicSectionsMappingModel
	if mapping.Regex != nil {
		sections, dg := flattenDynamicMappingSections(ctx, mapping.Regex.GetSections())
		if dg.HasError() {
			return nil, dg
		}
		regex = &DynamicSectionsMappingModel{Sections: sections}
	}

	var value *DynamicSectionsMappingModel
	if mapping.Value != nil {
		sections, dg := flattenDynamicMappingSections(ctx, mapping.Value.GetSections())
		if dg.HasError() {
			return nil, dg
		}
		value = &DynamicSectionsMappingModel{Sections: sections}
	}

	return &DynamicColorLabelMappingModel{
		ColorBy: flattenOptionalEnum(mapping.ColorBy, dashboardProtoToSchemaColorApplyTarget),
		Range:   rangeMapping,
		Regex:   regex,
		Value:   value,
	}, nil
}

func flattenDynamicRangeMapping(ctx context.Context, rangeMapping *dashboardservice.RangeMapping) (*DynamicRangeMappingModel, diag.Diagnostics) {
	if rangeMapping == nil {
		return nil, nil
	}

	thresholds, diags := flattenDynamicThresholds(ctx, rangeMapping.GetThresholds())
	if diags.HasError() {
		return nil, diags
	}

	var minMax *DynamicMinMaxModel
	if rangeMapping.MinMax != nil {
		minMax = &DynamicMinMaxModel{Auto: types.BoolNull()}
		if rangeMapping.MinMax.Auto != nil {
			minMax.Auto = types.BoolValue(true)
		}
		if rangeMapping.MinMax.Custom != nil {
			minMax.Custom = &DynamicMinMaxCustomModel{
				Max: types.Float64PointerValue(rangeMapping.MinMax.Custom.Max),
				Min: types.Float64PointerValue(rangeMapping.MinMax.Custom.Min),
			}
		}
	}

	return &DynamicRangeMappingModel{
		MinMax:        minMax,
		ThresholdType: flattenOptionalEnum(rangeMapping.ThresholdType, DashboardProtoToSchemaThresholdType),
		Thresholds:    thresholds,
	}, nil
}

func flattenDynamicMappingSections(ctx context.Context, sections []dashboardservice.MappingSection) (types.List, diag.Diagnostics) {
	if len(sections) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicMappingSectionAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	elements := make([]attr.Value, 0, len(sections))
	for i := range sections {
		model := &DynamicMappingSectionModel{
			Color: flattenOptionalEnum(sections[i].Color, dashboardProtoToSchemaColorSolidType),
			MapTo: types.StringPointerValue(sections[i].MapTo),
			Value: types.StringPointerValue(sections[i].Value),
		}
		element, dg := types.ObjectValueFrom(ctx, dynamicMappingSectionAttr(), model)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		elements = append(elements, element)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicMappingSectionAttr()}), diagnostics
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicMappingSectionAttr()}, elements)
}

func flattenInt32Pointer(value *int32) types.Int64 {
	if value == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*value))
}

func flattenMapToJSONString(value map[string]interface{}) JSONStringValue {
	if value == nil {
		return NewJSONStringNull()
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return NewJSONStringNull()
	}
	return NewJSONStringValue(string(encoded))
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
