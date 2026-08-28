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
	"math/big"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/durationpb"
)

var (
	DashboardSchemaToProtoUnit = map[string]dashboardservice.CommonUnit{
		utils.UNSPECIFIED: dashboardservice.COMMONUNIT_UNIT_UNSPECIFIED,
		"microseconds":    dashboardservice.COMMONUNIT_UNIT_MICROSECONDS,
		"milliseconds":    dashboardservice.COMMONUNIT_UNIT_MILLISECONDS,
		"nanoseconds":     dashboardservice.COMMONUNIT_UNIT_NANOSECONDS,
		"seconds":         dashboardservice.COMMONUNIT_UNIT_SECONDS,
		"bytes":           dashboardservice.COMMONUNIT_UNIT_BYTES,
		"kbytes":          dashboardservice.COMMONUNIT_UNIT_KBYTES,
		"mbytes":          dashboardservice.COMMONUNIT_UNIT_MBYTES,
		"gbytes":          dashboardservice.COMMONUNIT_UNIT_GBYTES,
		"bytes_iec":       dashboardservice.COMMONUNIT_UNIT_BYTES_IEC,
		"kibytes":         dashboardservice.COMMONUNIT_UNIT_KIBYTES,
		"mibytes":         dashboardservice.COMMONUNIT_UNIT_MIBYTES,
		"gibytes":         dashboardservice.COMMONUNIT_UNIT_GIBYTES,
		"euro_cents":      dashboardservice.COMMONUNIT_UNIT_EUR_CENTS,
		"euro":            dashboardservice.COMMONUNIT_UNIT_EUR,
		"usd_cents":       dashboardservice.COMMONUNIT_UNIT_USD_CENTS,
		"usd":             dashboardservice.COMMONUNIT_UNIT_USD,
		"custom":          dashboardservice.COMMONUNIT_UNIT_CUSTOM,
		"percent":         dashboardservice.COMMONUNIT_UNIT_PERCENT,
		"percent01":       dashboardservice.COMMONUNIT_UNIT_PERCENT_ZERO_ONE,
		"percent100":      dashboardservice.COMMONUNIT_UNIT_PERCENT_ZERO_HUNDRED,
		"datetime_iso":    dashboardservice.COMMONUNIT_UNIT_DATETIME_ISO,
	}
	DashboardProtoToSchemaUnit = utils.ReverseMap(DashboardSchemaToProtoUnit)
	DashboardValidUnits        = utils.GetKeys(DashboardSchemaToProtoUnit)

	DashboardLegendPlacementSchemaToProto = map[string]dashboardservice.LegendPlacement{
		utils.UNSPECIFIED: dashboardservice.LEGENDPLACEMENT_LEGEND_PLACEMENT_UNSPECIFIED,
		"auto":            dashboardservice.LEGENDPLACEMENT_LEGEND_PLACEMENT_AUTO,
		"bottom":          dashboardservice.LEGENDPLACEMENT_LEGEND_PLACEMENT_BOTTOM,
		"side":            dashboardservice.LEGENDPLACEMENT_LEGEND_PLACEMENT_SIDE,
		"hidden":          dashboardservice.LEGENDPLACEMENT_LEGEND_PLACEMENT_HIDDEN,
	}
	DashboardLegendPlacementProtoToSchema = utils.ReverseMap(DashboardLegendPlacementSchemaToProto)
	DashboardValidLegendPlacements        = utils.GetKeys(DashboardLegendPlacementSchemaToProto)

	DashboardRowStyleSchemaToProto = map[string]dashboardservice.RowStyle{
		utils.UNSPECIFIED: dashboardservice.ROWSTYLE_ROW_STYLE_UNSPECIFIED,
		"one_line":        dashboardservice.ROWSTYLE_ROW_STYLE_ONE_LINE,
		"two_line":        dashboardservice.ROWSTYLE_ROW_STYLE_TWO_LINE,
		"condensed":       dashboardservice.ROWSTYLE_ROW_STYLE_CONDENSED,
		"json":            dashboardservice.ROWSTYLE_ROW_STYLE_JSON,
		"list":            dashboardservice.ROWSTYLE_ROW_STYLE_LIST,
	}
	DashboardRowStyleProtoToSchema     = utils.ReverseMap(DashboardRowStyleSchemaToProto)
	DashboardValidRowStyles            = utils.GetKeys(DashboardRowStyleSchemaToProto)
	DashboardLegendColumnSchemaToProto = map[string]dashboardservice.LegendColumn{
		utils.UNSPECIFIED: dashboardservice.LEGENDCOLUMN_LEGEND_COLUMN_UNSPECIFIED,
		"min":             dashboardservice.LEGENDCOLUMN_LEGEND_COLUMN_MIN,
		"max":             dashboardservice.LEGENDCOLUMN_LEGEND_COLUMN_MAX,
		"sum":             dashboardservice.LEGENDCOLUMN_LEGEND_COLUMN_SUM,
		"avg":             dashboardservice.LEGENDCOLUMN_LEGEND_COLUMN_AVG,
		"last":            dashboardservice.LEGENDCOLUMN_LEGEND_COLUMN_LAST,
		"name":            dashboardservice.LEGENDCOLUMN_LEGEND_COLUMN_NAME,
	}
	DashboardLegendColumnProtoToSchema   = utils.ReverseMap(DashboardLegendColumnSchemaToProto)
	DashboardValidLegendColumns          = utils.GetKeys(DashboardLegendColumnSchemaToProto)
	DashboardOrderDirectionSchemaToProto = map[string]dashboardservice.OrderDirection{
		utils.UNSPECIFIED: dashboardservice.ORDERDIRECTION_ORDER_DIRECTION_UNSPECIFIED,
		"asc":             dashboardservice.ORDERDIRECTION_ORDER_DIRECTION_ASC,
		"desc":            dashboardservice.ORDERDIRECTION_ORDER_DIRECTION_DESC,
	}
	DashboardOrderDirectionProtoToSchema = utils.ReverseMap(DashboardOrderDirectionSchemaToProto)
	DashboardValidOrderDirections        = utils.GetKeys(DashboardOrderDirectionSchemaToProto)
	DashboardValidSortOrderDirections    = utils.GetKeys(DashboardOrderDirectionSchemaToProto)

	// V2 variables accept ORDER_DIRECTION_NONE; keep it off the shared map so
	// legacy variables / widget order_by flatten cannot write "none" into state
	// that OneOf(DashboardValidOrderDirections) would then reject.
	DashboardOrderDirectionSchemaToProtoV2 = map[string]dashboardservice.OrderDirection{
		"asc":  dashboardservice.ORDERDIRECTION_ORDER_DIRECTION_ASC,
		"desc": dashboardservice.ORDERDIRECTION_ORDER_DIRECTION_DESC,
		"none": dashboardservice.ORDERDIRECTION_ORDER_DIRECTION_NONE,
	}
	DashboardOrderDirectionProtoToSchemaV2 = utils.ReverseMap(DashboardOrderDirectionSchemaToProtoV2)
	DashboardValidOrderDirectionsV2        = utils.GetKeys(DashboardOrderDirectionSchemaToProtoV2)

	DashboardSchemaToProtoDisplayTypeV2 = map[string]dashboardservice.VariableDisplayTypeV2{
		"label_value": dashboardservice.VARIABLEDISPLAYTYPEV2_VARIABLE_DISPLAY_TYPE_V2_LABEL_VALUE,
		"value":       dashboardservice.VARIABLEDISPLAYTYPEV2_VARIABLE_DISPLAY_TYPE_V2_VALUE,
		"nothing":     dashboardservice.VARIABLEDISPLAYTYPEV2_VARIABLE_DISPLAY_TYPE_V2_NOTHING,
	}
	DashboardProtoToSchemaDisplayTypeV2 = utils.ReverseMap(DashboardSchemaToProtoDisplayTypeV2)
	DashboardValidDisplayTypesV2        = utils.GetKeys(DashboardSchemaToProtoDisplayTypeV2)

	DashboardSchemaToProtoVariableV2RefreshStrategy = map[string]dashboardservice.VariableSourceV2RefreshStrategy{
		"on_dashboard_load":    dashboardservice.VARIABLESOURCEV2REFRESHSTRATEGY_REFRESH_STRATEGY_ON_DASHBOARD_LOAD,
		"on_time_frame_change": dashboardservice.VARIABLESOURCEV2REFRESHSTRATEGY_REFRESH_STRATEGY_ON_TIME_FRAME_CHANGE,
	}
	DashboardProtoToSchemaVariableV2RefreshStrategy = utils.ReverseMap(DashboardSchemaToProtoVariableV2RefreshStrategy)
	DashboardValidVariableV2RefreshStrategies       = utils.GetKeys(DashboardSchemaToProtoVariableV2RefreshStrategy)

	DashboardSchemaToProtoDataModeTypeV2 = map[string]dashboardservice.V1CommonDataModeType{
		"high":    dashboardservice.V1COMMONDATAMODETYPE_DATA_MODE_TYPE_HIGH_UNSPECIFIED,
		"archive": dashboardservice.V1COMMONDATAMODETYPE_DATA_MODE_TYPE_ARCHIVE,
	}
	DashboardProtoToSchemaDataModeTypeV2 = utils.ReverseMap(DashboardSchemaToProtoDataModeTypeV2)
	DashboardValidDataModeTypesV2        = utils.GetKeys(DashboardSchemaToProtoDataModeTypeV2)

	DashboardValidMultiSelectSelectionTypes = []string{
		"multi",
		"single",
	}
	DashboardSchemaToProtoTooltipType = map[string]dashboardservice.TooltipType{
		utils.UNSPECIFIED: dashboardservice.TOOLTIPTYPE_TOOLTIP_TYPE_UNSPECIFIED,
		"all":             dashboardservice.TOOLTIPTYPE_TOOLTIP_TYPE_ALL,
		"single":          dashboardservice.TOOLTIPTYPE_TOOLTIP_TYPE_SINGLE,
	}
	DashboardProtoToSchemaTooltipType = utils.ReverseMap(DashboardSchemaToProtoTooltipType)
	DashboardValidTooltipTypes        = utils.GetKeys(DashboardSchemaToProtoTooltipType)
	DashboardSchemaToProtoScaleType   = map[string]dashboardservice.ScaleType{
		utils.UNSPECIFIED: dashboardservice.SCALETYPE_SCALE_TYPE_UNSPECIFIED,
		"linear":          dashboardservice.SCALETYPE_SCALE_TYPE_LINEAR,
		"logarithmic":     dashboardservice.SCALETYPE_SCALE_TYPE_LOGARITHMIC,
	}
	DashboardProtoToSchemaScaleType = utils.ReverseMap(DashboardSchemaToProtoScaleType)
	DashboardValidScaleTypes        = utils.GetKeys(DashboardSchemaToProtoScaleType)

	DashboardSchemaToProtoGaugeUnit = map[string]dashboardservice.GaugeUnit{
		utils.UNSPECIFIED: dashboardservice.GAUGEUNIT_UNIT_UNSPECIFIED,
		"none":            dashboardservice.GAUGEUNIT_UNIT_NUMBER,
		"percent":         dashboardservice.GAUGEUNIT_UNIT_PERCENT,
		"microseconds":    dashboardservice.GAUGEUNIT_UNIT_MICROSECONDS,
		"milliseconds":    dashboardservice.GAUGEUNIT_UNIT_MILLISECONDS,
		"nanoseconds":     dashboardservice.GAUGEUNIT_UNIT_NANOSECONDS,
		"seconds":         dashboardservice.GAUGEUNIT_UNIT_SECONDS,
		"bytes":           dashboardservice.GAUGEUNIT_UNIT_BYTES,
		"kbytes":          dashboardservice.GAUGEUNIT_UNIT_KBYTES,
		"mbytes":          dashboardservice.GAUGEUNIT_UNIT_MBYTES,
		"gbytes":          dashboardservice.GAUGEUNIT_UNIT_GBYTES,
		"bytes_iec":       dashboardservice.GAUGEUNIT_UNIT_BYTES_IEC,
		"kibytes":         dashboardservice.GAUGEUNIT_UNIT_KIBYTES,
		"mibytes":         dashboardservice.GAUGEUNIT_UNIT_MIBYTES,
		"gibytes":         dashboardservice.GAUGEUNIT_UNIT_GIBYTES,
		"euro_cents":      dashboardservice.GAUGEUNIT_UNIT_EUR_CENTS,
		"euro":            dashboardservice.GAUGEUNIT_UNIT_EUR,
		"usd_cents":       dashboardservice.GAUGEUNIT_UNIT_USD_CENTS,
		"usd":             dashboardservice.GAUGEUNIT_UNIT_USD,
		"custom":          dashboardservice.GAUGEUNIT_UNIT_CUSTOM,
		"percent01":       dashboardservice.GAUGEUNIT_UNIT_PERCENT_ZERO_ONE,
		"percent100":      dashboardservice.GAUGEUNIT_UNIT_PERCENT_ZERO_HUNDRED,
		"datetime_iso":    dashboardservice.GAUGEUNIT_UNIT_DATETIME_ISO,
	}
	DashboardProtoToSchemaGaugeUnit           = utils.ReverseMap(DashboardSchemaToProtoGaugeUnit)
	DashboardValidGaugeUnits                  = utils.GetKeys(DashboardSchemaToProtoGaugeUnit)
	DashboardSchemaToProtoPieChartLabelSource = map[string]dashboardservice.WidgetsPieChartLabelSource{
		utils.UNSPECIFIED: dashboardservice.WIDGETSPIECHARTLABELSOURCE_LABEL_SOURCE_UNSPECIFIED,
		"inner":           dashboardservice.WIDGETSPIECHARTLABELSOURCE_LABEL_SOURCE_INNER,
		"stack":           dashboardservice.WIDGETSPIECHARTLABELSOURCE_LABEL_SOURCE_STACK,
	}
	DashboardProtoToSchemaPieChartLabelSource = utils.ReverseMap(DashboardSchemaToProtoPieChartLabelSource)
	DashboardValidPieChartLabelSources        = utils.GetKeys(DashboardSchemaToProtoPieChartLabelSource)
	DashboardSchemaToProtoGaugeAggregation    = map[string]dashboardservice.GaugeAggregation{
		utils.UNSPECIFIED: dashboardservice.GAUGEAGGREGATION_AGGREGATION_UNSPECIFIED,
		"last":            dashboardservice.GAUGEAGGREGATION_AGGREGATION_LAST,
		"min":             dashboardservice.GAUGEAGGREGATION_AGGREGATION_MIN,
		"max":             dashboardservice.GAUGEAGGREGATION_AGGREGATION_MAX,
		"avg":             dashboardservice.GAUGEAGGREGATION_AGGREGATION_AVG,
		"sum":             dashboardservice.GAUGEAGGREGATION_AGGREGATION_SUM,
	}
	DashboardProtoToSchemaGaugeAggregation            = utils.ReverseMap(DashboardSchemaToProtoGaugeAggregation)
	DashboardValidGaugeAggregations                   = utils.GetKeys(DashboardSchemaToProtoGaugeAggregation)
	DashboardSchemaToProtoSpansAggregationMetricField = map[string]dashboardservice.MetricAggregationMetricField{
		utils.UNSPECIFIED: dashboardservice.METRICAGGREGATIONMETRICFIELD_METRIC_FIELD_UNSPECIFIED,
		"duration":        dashboardservice.METRICAGGREGATIONMETRICFIELD_METRIC_FIELD_DURATION,
	}
	DashboardProtoToSchemaSpansAggregationMetricField           = utils.ReverseMap(DashboardSchemaToProtoSpansAggregationMetricField)
	DashboardValidSpansAggregationMetricFields                  = utils.GetKeys(DashboardSchemaToProtoSpansAggregationMetricField)
	DashboardSchemaToProtoSpansAggregationMetricAggregationType = map[string]dashboardservice.MetricAggregationType{
		utils.UNSPECIFIED: dashboardservice.METRICAGGREGATIONTYPE_METRIC_AGGREGATION_TYPE_UNSPECIFIED,
		"min":             dashboardservice.METRICAGGREGATIONTYPE_METRIC_AGGREGATION_TYPE_MIN,
		"max":             dashboardservice.METRICAGGREGATIONTYPE_METRIC_AGGREGATION_TYPE_MAX,
		"avg":             dashboardservice.METRICAGGREGATIONTYPE_METRIC_AGGREGATION_TYPE_AVERAGE,
		"sum":             dashboardservice.METRICAGGREGATIONTYPE_METRIC_AGGREGATION_TYPE_SUM,
		"percentile_99":   dashboardservice.METRICAGGREGATIONTYPE_METRIC_AGGREGATION_TYPE_PERCENTILE_99,
		"percentile_95":   dashboardservice.METRICAGGREGATIONTYPE_METRIC_AGGREGATION_TYPE_PERCENTILE_95,
		"percentile_50":   dashboardservice.METRICAGGREGATIONTYPE_METRIC_AGGREGATION_TYPE_PERCENTILE_50,
	}
	DashboardProtoToSchemaSpansAggregationMetricAggregationType = utils.ReverseMap(DashboardSchemaToProtoSpansAggregationMetricAggregationType)
	DashboardValidSpansAggregationMetricAggregationTypes        = utils.GetKeys(DashboardSchemaToProtoSpansAggregationMetricAggregationType)
	DashboardProtoToSchemaSpansAggregationDimensionField        = map[string]dashboardservice.DimensionField{
		utils.UNSPECIFIED: dashboardservice.DIMENSIONFIELD_DIMENSION_FIELD_UNSPECIFIED,
		"trace_id":        dashboardservice.DIMENSIONFIELD_DIMENSION_FIELD_TRACE_ID,
	}
	DashboardSchemaToProtoSpansAggregationDimensionField           = utils.ReverseMap(DashboardProtoToSchemaSpansAggregationDimensionField)
	DashboardValidSpansAggregationDimensionFields                  = utils.GetKeys(DashboardProtoToSchemaSpansAggregationDimensionField)
	DashboardSchemaToProtoSpansAggregationDimensionAggregationType = map[string]dashboardservice.DimensionAggregationType{
		utils.UNSPECIFIED: dashboardservice.DIMENSIONAGGREGATIONTYPE_DIMENSION_AGGREGATION_TYPE_UNSPECIFIED,
		"unique_count":    dashboardservice.DIMENSIONAGGREGATIONTYPE_DIMENSION_AGGREGATION_TYPE_UNIQUE_COUNT,
		"error_count":     dashboardservice.DIMENSIONAGGREGATIONTYPE_DIMENSION_AGGREGATION_TYPE_ERROR_COUNT,
	}
	DashboardProtoToSchemaSpansAggregationDimensionAggregationType = utils.ReverseMap(DashboardSchemaToProtoSpansAggregationDimensionAggregationType)
	DashboardValidSpansAggregationDimensionAggregationTypes        = utils.GetKeys(DashboardSchemaToProtoSpansAggregationDimensionAggregationType)
	DashboardSchemaToProtoSpanFieldMetadataField                   = map[string]dashboardservice.MetadataField{
		utils.UNSPECIFIED:  dashboardservice.METADATAFIELD_METADATA_FIELD_UNSPECIFIED,
		"application_name": dashboardservice.METADATAFIELD_METADATA_FIELD_APPLICATION_NAME,
		"subsystem_name":   dashboardservice.METADATAFIELD_METADATA_FIELD_SUBSYSTEM_NAME,
		"service_name":     dashboardservice.METADATAFIELD_METADATA_FIELD_SERVICE_NAME,
		"operation_name":   dashboardservice.METADATAFIELD_METADATA_FIELD_OPERATION_NAME,
	}
	DashboardProtoToSchemaSpanFieldMetadataField = utils.ReverseMap(DashboardSchemaToProtoSpanFieldMetadataField)
	DashboardValidSpanFieldMetadataFields        = utils.GetKeys(DashboardSchemaToProtoSpanFieldMetadataField)
	DashboardSchemaToProtoSortBy                 = map[string]dashboardservice.SortByType{
		utils.UNSPECIFIED: dashboardservice.SORTBYTYPE_SORT_BY_TYPE_UNSPECIFIED,
		"value":           dashboardservice.SORTBYTYPE_SORT_BY_TYPE_VALUE,
		"name":            dashboardservice.SORTBYTYPE_SORT_BY_TYPE_NAME,
	}
	DashboardProtoToSchemaSortBy                = utils.ReverseMap(DashboardSchemaToProtoSortBy)
	DashboardValidSortBy                        = utils.GetKeys(DashboardSchemaToProtoSortBy)
	DashboardSchemaToProtoObservationFieldScope = map[string]dashboardservice.DatasetScope{
		utils.UNSPECIFIED: dashboardservice.DATASETSCOPE_DATASET_SCOPE_UNSPECIFIED,
		"user_data":       dashboardservice.DATASETSCOPE_DATASET_SCOPE_USER_DATA,
		"label":           dashboardservice.DATASETSCOPE_DATASET_SCOPE_LABEL,
		"metadata":        dashboardservice.DATASETSCOPE_DATASET_SCOPE_METADATA,
	}
	DashboardProtoToSchemaObservationFieldScope = utils.ReverseMap(DashboardSchemaToProtoObservationFieldScope)
	DashboardValidObservationFieldScope         = utils.GetKeys(DashboardSchemaToProtoObservationFieldScope)
	DashboardSchemaToProtoDataModeType          = map[string]dashboardservice.WidgetsCommonDataModeType{
		utils.UNSPECIFIED: dashboardservice.WIDGETSCOMMONDATAMODETYPE_DATA_MODE_TYPE_HIGH_UNSPECIFIED,
		"archive":         dashboardservice.WIDGETSCOMMONDATAMODETYPE_DATA_MODE_TYPE_ARCHIVE,
	}
	DashboardProtoToSchemaDataModeType     = utils.ReverseMap(DashboardSchemaToProtoDataModeType)
	DashboardValidDataModeTypes            = utils.GetKeys(DashboardSchemaToProtoDataModeType)
	DashboardSchemaToProtoGaugeThresholdBy = map[string]dashboardservice.GaugeThresholdBy{
		utils.UNSPECIFIED: dashboardservice.GAUGETHRESHOLDBY_THRESHOLD_BY_UNSPECIFIED,
		"value":           dashboardservice.GAUGETHRESHOLDBY_THRESHOLD_BY_VALUE,
		"background":      dashboardservice.GAUGETHRESHOLDBY_THRESHOLD_BY_BACKGROUND,
	}
	DashboardProtoToSchemaGaugeThresholdBy = utils.ReverseMap(DashboardSchemaToProtoGaugeThresholdBy)
	DashboardValidGaugeThresholdBy         = utils.GetKeys(DashboardSchemaToProtoGaugeThresholdBy)
	DashboardSchemaToProtoRefreshStrategy  = map[string]dashboardservice.MultiSelectRefreshStrategy{
		utils.UNSPECIFIED:      dashboardservice.MULTISELECTREFRESHSTRATEGY_REFRESH_STRATEGY_UNSPECIFIED,
		"on_dashboard_load":    dashboardservice.MULTISELECTREFRESHSTRATEGY_REFRESH_STRATEGY_ON_DASHBOARD_LOAD,
		"on_time_frame_change": dashboardservice.MULTISELECTREFRESHSTRATEGY_REFRESH_STRATEGY_ON_TIME_FRAME_CHANGE,
	}
	DashboardProtoToSchemaRefreshStrategy = utils.ReverseMap(DashboardSchemaToProtoRefreshStrategy)
	DashboardValidRefreshStrategies       = utils.GetKeys(DashboardSchemaToProtoRefreshStrategy)
	DashboardValidLogsAggregationTypes    = []string{"count", "count_distinct", "sum", "avg", "min", "max", "percentile"}
	DashboardValidSpanFieldTypes          = []string{"metadata", "tag", "process_tag"}
	DashboardValidSpanAggregationTypes    = []string{"metric", "dimension"}
	DashboardValidColorSchemes            = []string{"classic", "severity", "cold", "negative", "green", "red", "blue"}
	DashboardValidColorsBy                = []string{"stack", "group_by", "aggregation", "query", "category"}
	SectionValidColors                    = []string{"cyan", "green", "blue", "purple", "magenta", "pink", "orange"}

	// Annotation colours are user data: the Coralogix UI offers a swatch for
	// each one, and an annotation created without a choice reads back as
	// unspecified.
	// An unspecified colour is not in this map on purpose. It is what the API
	// returns for an annotation created without a choice, and the attribute is
	// plain optional, so it has to read back as null: a configuration that omits
	// the colour must not gain a value on apply. Omitting the attribute is how a
	// user says "no choice"; "unspecified" is not accepted as a written value.
	DashboardSchemaToProtoAnnotationColor = map[string]dashboardservice.AnnotationColor{
		"default": dashboardservice.ANNOTATIONCOLOR_ANNOTATION_COLOR_DEFAULT,
		"green":   dashboardservice.ANNOTATIONCOLOR_ANNOTATION_COLOR_GREEN,
		"cyan":    dashboardservice.ANNOTATIONCOLOR_ANNOTATION_COLOR_CYAN,
		"blue":    dashboardservice.ANNOTATIONCOLOR_ANNOTATION_COLOR_BLUE,
		"purple":  dashboardservice.ANNOTATIONCOLOR_ANNOTATION_COLOR_PURPLE,
		"magenta": dashboardservice.ANNOTATIONCOLOR_ANNOTATION_COLOR_MAGENTA,
		"red":     dashboardservice.ANNOTATIONCOLOR_ANNOTATION_COLOR_RED,
		"orange":  dashboardservice.ANNOTATIONCOLOR_ANNOTATION_COLOR_ORANGE,
		"yellow":  dashboardservice.ANNOTATIONCOLOR_ANNOTATION_COLOR_YELLOW,
	}
	DashboardProtoToSchemaAnnotationColor = utils.ReverseMap(DashboardSchemaToProtoAnnotationColor)
	DashboardValidAnnotationColors        = utils.GetKeys(DashboardSchemaToProtoAnnotationColor)

	DashboardSchemaToProtoThresholdType = map[string]dashboardservice.ThresholdType{
		utils.UNSPECIFIED: dashboardservice.THRESHOLDTYPE_THRESHOLD_TYPE_UNSPECIFIED,
		"absolute":        dashboardservice.THRESHOLDTYPE_THRESHOLD_TYPE_ABSOLUTE,
		"relative":        dashboardservice.THRESHOLDTYPE_THRESHOLD_TYPE_RELATIVE,
	}
	DashboardProtoToSchemaThresholdType = utils.ReverseMap(DashboardSchemaToProtoThresholdType)
	DashboardValidThresholdTypes        = utils.GetKeys(DashboardSchemaToProtoThresholdType)
	DashboardSchemaToProtoLegendBy      = map[string]dashboardservice.LegendBy{
		utils.UNSPECIFIED: dashboardservice.LEGENDBY_LEGEND_BY_UNSPECIFIED,
		"thresholds":      dashboardservice.LEGENDBY_LEGEND_BY_THRESHOLDS,
		"groups":          dashboardservice.LEGENDBY_LEGEND_BY_GROUPS,
	}
	DashboardProtoToSchemaLegendBy = utils.ReverseMap(DashboardSchemaToProtoLegendBy)
	DashboardValidLegendBys        = utils.GetKeys(DashboardSchemaToProtoLegendBy)

	DashboardSchemaToProtoCommonAggregation = map[string]dashboardservice.CommonAggregation{
		utils.UNSPECIFIED: dashboardservice.COMMONAGGREGATION_AGGREGATION_UNSPECIFIED,
		"last":            dashboardservice.COMMONAGGREGATION_AGGREGATION_LAST,
		"min":             dashboardservice.COMMONAGGREGATION_AGGREGATION_MIN,
		"max":             dashboardservice.COMMONAGGREGATION_AGGREGATION_MAX,
		"avg":             dashboardservice.COMMONAGGREGATION_AGGREGATION_AVG,
		"sum":             dashboardservice.COMMONAGGREGATION_AGGREGATION_SUM,
	}
	DashboardProtoToSchemaCommonAggregation = utils.ReverseMap(DashboardSchemaToProtoCommonAggregation)
	DashboardValidCommonAggregations        = utils.GetKeys(DashboardSchemaToProtoCommonAggregation)

	DashboardSchemaToProtoBarValueDisplay = map[string]dashboardservice.WidgetsBarValueDisplay{
		utils.UNSPECIFIED: dashboardservice.WIDGETSBARVALUEDISPLAY_BAR_VALUE_DISPLAY_UNSPECIFIED,
		"top":             dashboardservice.WIDGETSBARVALUEDISPLAY_BAR_VALUE_DISPLAY_TOP,
		"inside":          dashboardservice.WIDGETSBARVALUEDISPLAY_BAR_VALUE_DISPLAY_INSIDE,
		"both":            dashboardservice.WIDGETSBARVALUEDISPLAY_BAR_VALUE_DISPLAY_BOTH,
	}
	DashboardProtoToSchemaBarValueDisplay = utils.ReverseMap(DashboardSchemaToProtoBarValueDisplay)
	DashboardValidBarValueDisplays        = utils.GetKeys(DashboardSchemaToProtoBarValueDisplay)

	DashboardSchemaToProtoXAxisTimeFormat = map[string]dashboardservice.XAxisTimeFormat{
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
	DashboardProtoToSchemaXAxisTimeFormat = utils.ReverseMap(DashboardSchemaToProtoXAxisTimeFormat)
	DashboardValidXAxisTimeFormats        = utils.GetKeys(DashboardSchemaToProtoXAxisTimeFormat)

	DashboardSchemaToProtoMetricsEditorMode = map[string]dashboardservice.MetricsQueryEditorMode{
		utils.UNSPECIFIED: dashboardservice.METRICSQUERYEDITORMODE_METRICS_QUERY_EDITOR_MODE_UNSPECIFIED,
		"builder":         dashboardservice.METRICSQUERYEDITORMODE_METRICS_QUERY_EDITOR_MODE_BUILDER,
		"text":            dashboardservice.METRICSQUERYEDITORMODE_METRICS_QUERY_EDITOR_MODE_TEXT,
	}
	DashboardProtoToSchemaMetricsEditorMode = utils.ReverseMap(DashboardSchemaToProtoMetricsEditorMode)
	DashboardValidMetricsEditorModes        = utils.GetKeys(DashboardSchemaToProtoMetricsEditorMode)

	DashboardSchemaToProtoPromQLQueryType = map[string]dashboardservice.PromQLQueryType{
		utils.UNSPECIFIED: dashboardservice.PROMQLQUERYTYPE_PROM_QL_QUERY_TYPE_UNSPECIFIED,
		"range":           dashboardservice.PROMQLQUERYTYPE_PROM_QL_QUERY_TYPE_RANGE,
		"instant":         dashboardservice.PROMQLQUERYTYPE_PROM_QL_QUERY_TYPE_INSTANT,
	}
	DashboardProtoToSchemaPromQLQueryType = utils.ReverseMap(DashboardSchemaToProtoPromQLQueryType)
	DashboardValidPromQLQueryType         = utils.GetKeys(DashboardSchemaToProtoPromQLQueryType)

	DashboardSchemaToProtoWeekday = map[string]dashboardservice.Weekday{
		"monday":    dashboardservice.WEEKDAY_WEEKDAY_MONDAY,
		"tuesday":   dashboardservice.WEEKDAY_WEEKDAY_TUESDAY,
		"wednesday": dashboardservice.WEEKDAY_WEEKDAY_WEDNESDAY,
		"thursday":  dashboardservice.WEEKDAY_WEEKDAY_THURSDAY,
		"friday":    dashboardservice.WEEKDAY_WEEKDAY_FRIDAY,
		"saturday":  dashboardservice.WEEKDAY_WEEKDAY_SATURDAY,
		"sunday":    dashboardservice.WEEKDAY_WEEKDAY_SUNDAY,
	}
	DashboardProtoToSchemaWeekday = utils.ReverseMap(DashboardSchemaToProtoWeekday)
	DashboardValidWeekdays        = utils.GetKeys(DashboardSchemaToProtoWeekday)

	SupportedWidgetTypes = []string{
		"data_table",
		"gauge",
		"hexagon",
		"line_chart",
		"pie_chart",
		"bar_chart",
		"horizontal_bar_chart",
		"markdown",
		"dynamic",
	}
)

// OptionalEnumPointer converts a Terraform string to an OpenAPI enum pointer.
// Null and unknown values must remain absent instead of becoming a pointer to
// the enum's empty Go value, which is not valid protobuf JSON.
func OptionalEnumPointer[T ~string](value types.String, values map[string]T) *T {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}

	converted, ok := values[value.ValueString()]
	if !ok {
		return nil
	}

	return &converted
}

// FlattenEnum converts an API enum to the name the schema uses. A field the API
// did not set arrives as the enum's zero value, which is the empty string and is
// in no mapping; it becomes "unspecified", the schema's own name for that state.
// Writing the empty string instead puts a value in state that the schema does not
// allow, and an attribute that is Optional+Computed then plans as unknown for
// ever, because Terraform cannot reconcile it with an omitted configuration.
func FlattenEnum[T ~string](value T, mapping map[T]string) types.String {
	if name, ok := mapping[value]; ok {
		return types.StringValue(name)
	}

	return types.StringValue(utils.UNSPECIFIED)
}

func legacyDurationToOpenAPI(value, fieldName string) (*string, diag.Diagnostic) {
	duration, diagnostic := utils.ParseDuration(value, fieldName)
	if diagnostic != nil {
		return nil, diagnostic
	}

	return durationToOpenAPI(*duration), nil
}

func durationToOpenAPI(duration time.Duration) *string {
	encoded, err := protojson.Marshal(durationpb.New(duration))
	if err != nil {
		return nil
	}

	value := string(encoded[1 : len(encoded)-1])
	return &value
}

func openAPIDurationToLegacy(value *string) basetypes.StringValue {
	if value == nil {
		return types.StringNull()
	}

	duration := new(durationpb.Duration)
	if err := protojson.Unmarshal([]byte(fmt.Sprintf("%q", *value)), duration); err != nil {
		return types.StringValue(*value)
	}
	if duration.Seconds == 0 && duration.Nanos == 0 {
		return types.StringValue("seconds:0")
	}

	return types.StringValue(duration.String())
}

// GoDurationToOpenAPI converts the Go duration syntax used by bar-chart x-axis
// intervals to the protobuf JSON duration syntax expected by the HTTP API.
func GoDurationToOpenAPI(value types.String, fieldName string) (*string, diag.Diagnostic) {
	if value.IsNull() || value.IsUnknown() {
		return nil, nil
	}

	duration, err := time.ParseDuration(value.ValueString())
	if err != nil {
		return nil, diag.NewErrorDiagnostic(fmt.Sprintf("Error expand %s", fieldName), err.Error())
	}

	return durationToOpenAPI(duration), nil
}

// OpenAPIDurationToGo converts protobuf JSON duration syntax back to the Go
// duration syntax historically stored for bar-chart x-axis intervals.
func OpenAPIDurationToGo(value *string) basetypes.StringValue {
	if value == nil {
		return types.StringNull()
	}

	duration := new(durationpb.Duration)
	if err := protojson.Unmarshal([]byte(fmt.Sprintf("%q", *value)), duration); err != nil {
		return types.StringValue(*value)
	}

	return types.StringValue(duration.AsDuration().String())
}

// QueryMetricsModel is the data table's metrics query. The line chart has its
// own LineChartQueryMetricsModel, because the two queries carry different fields.
type QueryMetricsModel struct {
	PromqlQuery     types.String    `tfsdk:"promql_query"`
	Filters         types.List      `tfsdk:"filters"` //MetricsFilterModel
	PromqlQueryType types.String    `tfsdk:"promql_query_type"`
	EditorMode      types.String    `tfsdk:"editor_mode"`
	TimeFrame       *TimeFrameModel `tfsdk:"time_frame"`
}

type MetricFilterModel struct {
	Metric   types.String         `tfsdk:"metric"`
	Label    types.String         `tfsdk:"label"`
	Operator *FilterOperatorModel `tfsdk:"operator"`
}

type QuerySpansModel struct {
	LuceneQuery  types.String    `tfsdk:"lucene_query"`
	GroupBy      types.List      `tfsdk:"group_by"`     //SpansFieldModel
	Aggregations types.List      `tfsdk:"aggregations"` //SpansAggregationModel
	Filters      types.List      `tfsdk:"filters"`      //SpansFilterModel
	TimeFrame    *TimeFrameModel `tfsdk:"time_frame"`
}

type SpansFieldModel struct {
	Type  types.String `tfsdk:"type"`
	Value types.String `tfsdk:"value"`
}

type LogsAggregationModel struct {
	Type             types.String  `tfsdk:"type"`
	Field            types.String  `tfsdk:"field"`
	Percent          types.Float64 `tfsdk:"percent"`
	ObservationField types.Object  `tfsdk:"observation_field"`
}

type DataPrimeModel struct {
	Query     types.String    `tfsdk:"query"`
	Filters   types.List      `tfsdk:"filters"` //DashboardFilterSourceModel
	TimeFrame *TimeFrameModel `tfsdk:"time_frame"`
}

type SpansAggregationModel struct {
	Type            types.String `tfsdk:"type"`
	AggregationType types.String `tfsdk:"aggregation_type"`
	Field           types.String `tfsdk:"field"`
}

type WidgetDefinitionModel struct {
	LineChart          *LineChartModel          `tfsdk:"line_chart"`
	Hexagon            *HexagonModel            `tfsdk:"hexagon"`
	DataTable          *DataTableModel          `tfsdk:"data_table"`
	Gauge              *GaugeModel              `tfsdk:"gauge"`
	PieChart           *PieChartModel           `tfsdk:"pie_chart"`
	BarChart           *BarChartModel           `tfsdk:"bar_chart"`
	HorizontalBarChart *HorizontalBarChartModel `tfsdk:"horizontal_bar_chart"`
	Markdown           *MarkdownModel           `tfsdk:"markdown"`
	Dynamic            *DynamicModel            `tfsdk:"dynamic"`
}

type DynamicModel struct {
	QueryDefinitions types.List                 `tfsdk:"query_definitions"` //DynamicQueryDefinitionModel
	Interpretation   types.String               `tfsdk:"interpretation"`
	TimeFrame        *TimeFrameModel            `tfsdk:"time_frame"`
	Visualization    *DynamicVisualizationModel `tfsdk:"visualization"`
}

type DynamicQueryDefinitionModel struct {
	ID    types.String       `tfsdk:"id"`
	Name  types.String       `tfsdk:"name"`
	Query *DynamicQueryModel `tfsdk:"query"`
}

type DynamicQueryModel struct {
	Logs      *DynamicQueryLogsModel      `tfsdk:"logs"`
	Spans     *DynamicQuerySpansModel     `tfsdk:"spans"`
	Metrics   *DynamicQueryMetricsModel   `tfsdk:"metrics"`
	DataPrime *DynamicQueryDataPrimeModel `tfsdk:"data_prime"`
}

type DynamicQueryLogsModel struct {
	LuceneQuery  types.String `tfsdk:"lucene_query"`
	GroupBy      types.List   `tfsdk:"group_by"`     //ObservationFieldModel
	Aggregations types.List   `tfsdk:"aggregations"` //LogsAggregationModel
	Filters      types.List   `tfsdk:"filters"`      //LogsFilterModel
	DataModeType types.String `tfsdk:"data_mode_type"`
}

type DynamicQuerySpansModel struct {
	LuceneQuery  types.String `tfsdk:"lucene_query"`
	GroupBy      types.List   `tfsdk:"group_by"`     //SpanObservationFieldModel
	Aggregations types.List   `tfsdk:"aggregations"` //LogsAggregationModel
	Filters      types.List   `tfsdk:"filters"`      //SpansFilterModel
	DataModeType types.String `tfsdk:"data_mode_type"`
}

type DynamicQueryMetricsModel struct {
	PromqlQuery     types.String `tfsdk:"promql_query"`
	PromqlQueryType types.String `tfsdk:"promql_query_type"`
	EditorMode      types.String `tfsdk:"editor_mode"`
	SeriesLimitType types.String `tfsdk:"series_limit_type"`
}

type DynamicQueryDataPrimeModel struct {
	Query        types.String `tfsdk:"query"`
	DataModeType types.String `tfsdk:"data_mode_type"`
}

type SpanObservationFieldModel struct {
	Keypath      types.List   `tfsdk:"keypath"` //types.String
	Scope        types.String `tfsdk:"scope"`
	RelationType types.String `tfsdk:"relation_type"`
}

type DynamicVisualizationModel struct {
	Stat                 *DynamicStatModel                 `tfsdk:"stat"`
	StatCard             *DynamicStatCardModel             `tfsdk:"stat_card"`
	Gauge                *DynamicGaugeModel                `tfsdk:"gauge"`
	PieChart             *DynamicPieChartModel             `tfsdk:"pie_chart"`
	Table                *DynamicTableModel                `tfsdk:"table"`
	TimeSeriesLines      *DynamicTimeSeriesLinesModel      `tfsdk:"time_series_lines"`
	TimeSeriesLinesMulti *DynamicTimeSeriesLinesMultiModel `tfsdk:"time_series_lines_multi"`
	TimeSeriesBars       *DynamicTimeSeriesBarsModel       `tfsdk:"time_series_bars"`
	HexagonBins          *DynamicHexagonBinsModel          `tfsdk:"hexagon_bins"`
	Heatmap              *DynamicHeatmapModel              `tfsdk:"heatmap"`
	Geomap               *DynamicGeomapModel               `tfsdk:"geomap"`
	VerticalBars         *DynamicVerticalBarsModel         `tfsdk:"vertical_bars"`
	VerticalBarsMulti    *DynamicVerticalBarsMultiModel    `tfsdk:"vertical_bars_multi"`
	HorizontalBars       *DynamicHorizontalBarsModel       `tfsdk:"horizontal_bars"`
	HorizontalBarsMulti  *DynamicHorizontalBarsMultiModel  `tfsdk:"horizontal_bars_multi"`
}

type DynamicStatModel struct {
	AllowAbbreviation types.Bool    `tfsdk:"allow_abbreviation"`
	CategoryFields    types.List    `tfsdk:"category_fields"` //ObservationFieldModel
	CustomUnit        types.String  `tfsdk:"custom_unit"`
	DecimalPrecision  types.Int64   `tfsdk:"decimal_precision"`
	DisplaySeriesName types.Bool    `tfsdk:"display_series_name"`
	Legend            *LegendModel  `tfsdk:"legend"`
	LegendBy          types.String  `tfsdk:"legend_by"`
	Max               types.Float64 `tfsdk:"max"`
	Min               types.Float64 `tfsdk:"min"`
	ThresholdBy       types.String  `tfsdk:"threshold_by"`
	ThresholdType     types.String  `tfsdk:"threshold_type"`
	Thresholds        types.List    `tfsdk:"thresholds"` //DynamicThresholdModel
	Unit              types.String  `tfsdk:"unit"`
	ValueField        types.Object  `tfsdk:"value_field"`  //ObservationFieldModel
	ValueFields       types.List    `tfsdk:"value_fields"` //ObservationFieldModel
}

type DynamicThresholdModel struct {
	From  types.Float64 `tfsdk:"from"`
	Color types.String  `tfsdk:"color"`
	Label types.String  `tfsdk:"label"`
}

type LineChartModel struct {
	Legend           *LegendModel  `tfsdk:"legend"`
	Tooltip          *TooltipModel `tfsdk:"tooltip"`
	QueryDefinitions types.List    `tfsdk:"query_definitions"` //LineChartQueryDefinitionModel
	StackedLine      types.String  `tfsdk:"stacked_line"`
	ConnectNulls     types.Bool    `tfsdk:"connect_nulls"`
	UseDataTimeRange types.Bool    `tfsdk:"use_data_time_range"`
	XAxisTimeFormat  types.String  `tfsdk:"x_axis_time_format"`
}

type TooltipModel struct {
	ShowLabels types.Bool   `tfsdk:"show_labels"`
	Type       types.String `tfsdk:"type"`
}

type LineChartQueryDefinitionModel struct {
	ID                 types.String         `tfsdk:"id"`
	Query              *LineChartQueryModel `tfsdk:"query"`
	SeriesNameTemplate types.String         `tfsdk:"series_name_template"`
	SeriesCountLimit   types.Int64          `tfsdk:"series_count_limit"`
	Unit               types.String         `tfsdk:"unit"`
	ScaleType          types.String         `tfsdk:"scale_type"`
	Name               types.String         `tfsdk:"name"`
	IsVisible          types.Bool           `tfsdk:"is_visible"`
	ColorScheme        types.String         `tfsdk:"color_scheme"`
	HashColors         types.Bool           `tfsdk:"hash_colors"`
	Resolution         types.Object         `tfsdk:"resolution"` //LineChartResolutionModel
	DataModeType       types.String         `tfsdk:"data_mode_type"`
	CustomUnit         types.String         `tfsdk:"custom_unit"`
	Decimal            types.Number         `tfsdk:"decimal"`
	DecimalPrecision   types.Bool           `tfsdk:"decimal_precision"`
	YAxisMax           Float32Value         `tfsdk:"y_axis_max"`
	YAxisMin           Float32Value         `tfsdk:"y_axis_min"`
	// IntervalResolution is how the query groups time into buckets. The bar
	// chart carries the same message on its x-axis.
	IntervalResolution *IntervalResolutionModel `tfsdk:"interval_resolution"`
}

type LineChartResolutionModel struct {
	Interval         types.String `tfsdk:"interval"`
	BucketsPresented types.Int64  `tfsdk:"buckets_presented"`
}

type LineChartQueryModel struct {
	Logs      *LineChartQueryLogsModel    `tfsdk:"logs"`
	Metrics   *LineChartQueryMetricsModel `tfsdk:"metrics"`
	Spans     *LineChartQuerySpansModel   `tfsdk:"spans"`
	DataPrime *DataPrimeModel             `tfsdk:"data_prime"`
}

// LineChartQueryMetricsModel is separate from QueryMetricsModel because the
// line chart query carries editor_mode and series_limit_type, which the data
// table query does not have.
type LineChartQueryMetricsModel struct {
	PromqlQuery     types.String    `tfsdk:"promql_query"`
	Filters         types.List      `tfsdk:"filters"` //MetricsFilterModel
	PromqlQueryType types.String    `tfsdk:"promql_query_type"`
	EditorMode      types.String    `tfsdk:"editor_mode"`
	SeriesLimitType types.String    `tfsdk:"series_limit_type"`
	TimeFrame       *TimeFrameModel `tfsdk:"time_frame"`
}

type LineChartQueryLogsModel struct {
	LuceneQuery  types.String    `tfsdk:"lucene_query"`
	GroupBy      types.List      `tfsdk:"group_by"`     //types.String
	GroupBys     types.List      `tfsdk:"group_bys"`    //ObservationFieldModel
	Aggregations types.List      `tfsdk:"aggregations"` //AggregationModel
	Filters      types.List      `tfsdk:"filters"`      //FilterModel
	TimeFrame    *TimeFrameModel `tfsdk:"time_frame"`
}

type QueryMetricFilterModel struct {
	Metric   types.String         `tfsdk:"metric"`
	Label    types.String         `tfsdk:"label"`
	Operator *FilterOperatorModel `tfsdk:"operator"`
}

type LineChartQuerySpansModel struct {
	LuceneQuery  types.String    `tfsdk:"lucene_query"`
	GroupBy      types.List      `tfsdk:"group_by"`     //SpansFieldModel
	GroupBys     types.List      `tfsdk:"group_bys"`    //SpanObservationFieldModel
	Aggregations types.List      `tfsdk:"aggregations"` //SpansAggregationModel
	Filters      types.List      `tfsdk:"filters"`      //SpansFilterModel
	TimeFrame    *TimeFrameModel `tfsdk:"time_frame"`
}

type DataTableModel struct {
	Query          *DataTableQueryModel `tfsdk:"query"`
	ResultsPerPage types.Int64          `tfsdk:"results_per_page"`
	RowStyle       types.String         `tfsdk:"row_style"`
	Columns        types.List           `tfsdk:"columns"` //DataTableColumnModel
	OrderBy        *OrderByModel        `tfsdk:"order_by"`
	DataModeType   types.String         `tfsdk:"data_mode_type"`
}

type DataTableQueryLogsModel struct {
	LuceneQuery types.String                     `tfsdk:"lucene_query"`
	Filters     types.List                       `tfsdk:"filters"` //LogsFilterModel
	Grouping    *DataTableLogsQueryGroupingModel `tfsdk:"grouping"`
	TimeFrame   *TimeFrameModel                  `tfsdk:"time_frame"`
}

type LogsFilterModel struct {
	Field            types.String         `tfsdk:"field"`
	Operator         *FilterOperatorModel `tfsdk:"operator"`
	ObservationField types.Object         `tfsdk:"observation_field"` // ObservationFieldModel
}

type DataTableLogsQueryGroupingModel struct {
	Aggregations types.List `tfsdk:"aggregations"` //DataTableLogsAggregationModel
	GroupBys     types.List `tfsdk:"group_bys"`    //types.String
}

type DataTableLogsAggregationModel struct {
	ID          types.String          `tfsdk:"id"`
	Name        types.String          `tfsdk:"name"`
	IsVisible   types.Bool            `tfsdk:"is_visible"`
	Aggregation *LogsAggregationModel `tfsdk:"aggregation"`
}

type DataTableQueryModel struct {
	Logs      *DataTableQueryLogsModel  `tfsdk:"logs"`
	Metrics   *QueryMetricsModel        `tfsdk:"metrics"`
	Spans     *DataTableQuerySpansModel `tfsdk:"spans"`
	DataPrime *DataPrimeModel           `tfsdk:"data_prime"`
}

type MetricsFilterModel struct {
	Metric   types.String         `tfsdk:"metric"`
	Label    types.String         `tfsdk:"label"`
	Operator *FilterOperatorModel `tfsdk:"operator"`
}

type DataTableColumnModel struct {
	Field types.String `tfsdk:"field"`
	Width types.Int64  `tfsdk:"width"`
}

type OrderByModel struct {
	Field          types.String `tfsdk:"field"`
	OrderDirection types.String `tfsdk:"order_direction"`
}

type DataTableQuerySpansModel struct {
	LuceneQuery types.String                      `tfsdk:"lucene_query"`
	Filters     types.List                        `tfsdk:"filters"` //SpansFilterModel
	Grouping    *DataTableSpansQueryGroupingModel `tfsdk:"grouping"`
	TimeFrame   *TimeFrameModel                   `tfsdk:"time_frame"`
}

type SpansFilterModel struct {
	Field    *SpansFieldModel     `tfsdk:"field"`
	Operator *FilterOperatorModel `tfsdk:"operator"`
}

type DataTableSpansQueryGroupingModel struct {
	GroupBy      types.List `tfsdk:"group_by"`     //SpansFieldModel
	Aggregations types.List `tfsdk:"aggregations"` //DataTableSpansAggregationModel
}

type GaugeModel struct {
	Query             *GaugeQueryModel `tfsdk:"query"`
	Min               types.Float64    `tfsdk:"min"`
	Max               types.Float64    `tfsdk:"max"`
	ShowInnerArc      types.Bool       `tfsdk:"show_inner_arc"`
	ShowOuterArc      types.Bool       `tfsdk:"show_outer_arc"`
	Unit              types.String     `tfsdk:"unit"`
	Thresholds        types.List       `tfsdk:"thresholds"` //GaugeThresholdModel
	DataModeType      types.String     `tfsdk:"data_mode_type"`
	ThresholdBy       types.String     `tfsdk:"threshold_by"`
	ThresholdType     types.String     `tfsdk:"threshold_type"`
	DisplaySeriesName types.Bool       `tfsdk:"display_series_name"`
	Decimal           types.Number     `tfsdk:"decimal"`
	ArcDisplay        *ArcDisplayModel `tfsdk:"arc_display"`
	DecimalPrecision  types.Bool       `tfsdk:"decimal_precision"`
	CustomUnit        types.String     `tfsdk:"custom_unit"`
	Legend            *LegendModel     `tfsdk:"legend"`
	LegendBy          types.String     `tfsdk:"legend_by"`
	ShowMinMax        types.Bool       `tfsdk:"show_min_max"`
}

// ArcDisplayModel replaces the gauge's deprecated show_inner_arc and
// show_outer_arc booleans.
type ArcDisplayModel struct {
	ThresholdArc types.Bool `tfsdk:"threshold_arc"`
	ValueArc     types.Bool `tfsdk:"value_arc"`
}

type GaugeQueryModel struct {
	Logs      *GaugeQueryLogsModel    `tfsdk:"logs"`
	Metrics   *GaugeQueryMetricsModel `tfsdk:"metrics"`
	Spans     *GaugeQuerySpansModel   `tfsdk:"spans"`
	DataPrime *DataPrimeModel         `tfsdk:"data_prime"`
}

type GaugeQueryLogsModel struct {
	LuceneQuery     types.String          `tfsdk:"lucene_query"`
	LogsAggregation *LogsAggregationModel `tfsdk:"logs_aggregation"`
	Filters         types.List            `tfsdk:"filters"`  //LogsFilterModel
	GroupBy         types.List            `tfsdk:"group_by"` //ObservationFieldModel
	TimeFrame       *TimeFrameModel       `tfsdk:"time_frame"`
}

type GaugeQueryMetricsModel struct {
	PromqlQuery     types.String    `tfsdk:"promql_query"`
	Aggregation     types.String    `tfsdk:"aggregation"`
	Filters         types.List      `tfsdk:"filters"` //MetricsFilterModel
	EditorMode      types.String    `tfsdk:"editor_mode"`
	PromqlQueryType types.String    `tfsdk:"promql_query_type"`
	TimeFrame       *TimeFrameModel `tfsdk:"time_frame"`
}

type GaugeQuerySpansModel struct {
	LuceneQuery      types.String           `tfsdk:"lucene_query"`
	SpansAggregation *SpansAggregationModel `tfsdk:"spans_aggregation"`
	Filters          types.List             `tfsdk:"filters"`   //SpansFilterModel
	GroupBy          types.List             `tfsdk:"group_by"`  //SpansFieldModel
	GroupBys         types.List             `tfsdk:"group_bys"` //SpanObservationFieldModel
	TimeFrame        *TimeFrameModel        `tfsdk:"time_frame"`
}

type GaugeThresholdModel struct {
	From  types.Float64 `tfsdk:"from"`
	Color types.String  `tfsdk:"color"`
	Label types.String  `tfsdk:"label"`
}

type PieChartModel struct {
	Query              *PieChartQueryModel           `tfsdk:"query"`
	MaxSlicesPerChart  types.Int64                   `tfsdk:"max_slices_per_chart"`
	MinSlicePercentage types.Int64                   `tfsdk:"min_slice_percentage"`
	StackDefinition    *PieChartStackDefinitionModel `tfsdk:"stack_definition"`
	LabelDefinition    *LabelDefinitionModel         `tfsdk:"label_definition"`
	ShowLegend         types.Bool                    `tfsdk:"show_legend"`
	GroupNameTemplate  types.String                  `tfsdk:"group_name_template"`
	Unit               types.String                  `tfsdk:"unit"`
	ColorScheme        types.String                  `tfsdk:"color_scheme"`
	HashColors         types.Bool                    `tfsdk:"hash_colors"`
	DataModeType       types.String                  `tfsdk:"data_mode_type"`
	CustomUnit         types.String                  `tfsdk:"custom_unit"`
	Decimal            types.Number                  `tfsdk:"decimal"`
	DecimalPrecision   types.Bool                    `tfsdk:"decimal_precision"`
	Legend             *LegendModel                  `tfsdk:"legend"`
	ShowTotal          types.Bool                    `tfsdk:"show_total"`
}

type PieChartStackDefinitionModel struct {
	MaxSlicesPerStack types.Int64  `tfsdk:"max_slices_per_stack"`
	StackNameTemplate types.String `tfsdk:"stack_name_template"`
}

type PieChartQueryModel struct {
	Logs      *PieChartQueryLogsModel      `tfsdk:"logs"`
	Metrics   *PieChartQueryMetricsModel   `tfsdk:"metrics"`
	Spans     *PieChartQuerySpansModel     `tfsdk:"spans"`
	DataPrime *PieChartQueryDataPrimeModel `tfsdk:"data_prime"`
}

type PieChartQueryLogsModel struct {
	LuceneQuery           types.String          `tfsdk:"lucene_query"`
	Aggregation           *LogsAggregationModel `tfsdk:"aggregation"`
	Filters               types.List            `tfsdk:"filters"`     //LogsFilterModel
	GroupNames            types.List            `tfsdk:"group_names"` //types.String
	StackedGroupName      types.String          `tfsdk:"stacked_group_name"`
	GroupNamesFields      types.List            `tfsdk:"group_names_fields"`       //ObservationFieldModel
	StackedGroupNameField types.Object          `tfsdk:"stacked_group_name_field"` //ObservationFieldModel
	TimeFrame             *TimeFrameModel       `tfsdk:"time_frame"`
}

type PieChartQueryMetricsModel struct {
	PromqlQuery      types.String    `tfsdk:"promql_query"`
	Filters          types.List      `tfsdk:"filters"`     //MetricsFilterModel
	GroupNames       types.List      `tfsdk:"group_names"` //types.String
	StackedGroupName types.String    `tfsdk:"stacked_group_name"`
	Aggregation      types.String    `tfsdk:"aggregation"`
	EditorMode       types.String    `tfsdk:"editor_mode"`
	PromqlQueryType  types.String    `tfsdk:"promql_query_type"`
	TimeFrame        *TimeFrameModel `tfsdk:"time_frame"`
}

type PieChartQuerySpansModel struct {
	LuceneQuery           types.String           `tfsdk:"lucene_query"`
	Aggregation           *SpansAggregationModel `tfsdk:"aggregation"`
	Filters               types.List             `tfsdk:"filters"`     //SpansFilterModel
	GroupNames            types.List             `tfsdk:"group_names"` //SpansFieldModel
	StackedGroupName      *SpansFieldModel       `tfsdk:"stacked_group_name"`
	GroupNamesFields      types.List             `tfsdk:"group_names_fields"`       //SpanObservationFieldModel
	StackedGroupNameField types.Object           `tfsdk:"stacked_group_name_field"` //SpanObservationFieldModel
	TimeFrame             *TimeFrameModel        `tfsdk:"time_frame"`
}

type PieChartQueryDataPrimeModel struct {
	Query            types.String    `tfsdk:"query"`
	Filters          types.List      `tfsdk:"filters"`     //DashboardFilterSourceModel
	GroupNames       types.List      `tfsdk:"group_names"` //types.String
	StackedGroupName types.String    `tfsdk:"stacked_group_name"`
	TimeFrame        *TimeFrameModel `tfsdk:"time_frame"`
}

type LabelDefinitionModel struct {
	LabelSource    types.String `tfsdk:"label_source"`
	IsVisible      types.Bool   `tfsdk:"is_visible"`
	ShowName       types.Bool   `tfsdk:"show_name"`
	ShowValue      types.Bool   `tfsdk:"show_value"`
	ShowPercentage types.Bool   `tfsdk:"show_percentage"`
}

type BarChartModel struct {
	Query             *BarChartQueryModel           `tfsdk:"query"`
	MaxBarsPerChart   types.Int64                   `tfsdk:"max_bars_per_chart"`
	GroupNameTemplate types.String                  `tfsdk:"group_name_template"`
	StackDefinition   *BarChartStackDefinitionModel `tfsdk:"stack_definition"`
	ScaleType         types.String                  `tfsdk:"scale_type"`
	ColorsBy          types.String                  `tfsdk:"colors_by"`
	XAxis             *BarChartXAxisModel           `tfsdk:"xaxis"`
	Unit              types.String                  `tfsdk:"unit"`
	SortBy            types.String                  `tfsdk:"sort_by"`
	ColorScheme       types.String                  `tfsdk:"color_scheme"`
	HashColors        types.Bool                    `tfsdk:"hash_colors"`
	DataModeType      types.String                  `tfsdk:"data_mode_type"`
	BarValueDisplay   types.String                  `tfsdk:"bar_value_display"`
	CustomUnit        types.String                  `tfsdk:"custom_unit"`
	Decimal           types.Number                  `tfsdk:"decimal"`
	DecimalPrecision  types.Bool                    `tfsdk:"decimal_precision"`
	Legend            *LegendModel                  `tfsdk:"legend"`
	XAxisTimeFormat   types.String                  `tfsdk:"x_axis_time_format"`
	YAxisMax          Float32Value                  `tfsdk:"y_axis_max"`
	YAxisMin          Float32Value                  `tfsdk:"y_axis_min"`
}

type BarChartQueryModel struct {
	Logs      types.Object `tfsdk:"logs"`       //BarChartQueryLogsModel
	Metrics   types.Object `tfsdk:"metrics"`    //BarChartQueryMetricsModel
	Spans     types.Object `tfsdk:"spans"`      //BarChartQuerySpansModel
	DataPrime types.Object `tfsdk:"data_prime"` //BarChartQueryDataPrimeModel
}

type BarChartQueryLogsModel struct {
	LuceneQuery           types.String          `tfsdk:"lucene_query"`
	Aggregation           *LogsAggregationModel `tfsdk:"aggregation"`
	Filters               types.List            `tfsdk:"filters"`     //LogsFilterModel
	GroupNames            types.List            `tfsdk:"group_names"` //types.String
	StackedGroupName      types.String          `tfsdk:"stacked_group_name"`
	GroupNamesFields      types.List            `tfsdk:"group_names_fields"`       //ObservationFieldModel
	StackedGroupNameField types.Object          `tfsdk:"stacked_group_name_field"` //ObservationFieldModel
	TimeFrame             *TimeFrameModel       `tfsdk:"time_frame"`
}

type ObservationFieldModel struct {
	Keypath types.List   `tfsdk:"keypath"` //types.String
	Scope   types.String `tfsdk:"scope"`
}

type BarChartQueryMetricsModel struct {
	PromqlQuery      types.String    `tfsdk:"promql_query"`
	Filters          types.List      `tfsdk:"filters"`     //MetricsFilterModel
	GroupNames       types.List      `tfsdk:"group_names"` //types.String
	StackedGroupName types.String    `tfsdk:"stacked_group_name"`
	Aggregation      types.String    `tfsdk:"aggregation"`
	EditorMode       types.String    `tfsdk:"editor_mode"`
	PromqlQueryType  types.String    `tfsdk:"promql_query_type"`
	TimeFrame        *TimeFrameModel `tfsdk:"time_frame"`
}

type BarChartQuerySpansModel struct {
	LuceneQuery           types.String           `tfsdk:"lucene_query"`
	Aggregation           *SpansAggregationModel `tfsdk:"aggregation"`
	Filters               types.List             `tfsdk:"filters"`     //SpansFilterModel
	GroupNames            types.List             `tfsdk:"group_names"` //SpansFieldModel
	StackedGroupName      *SpansFieldModel       `tfsdk:"stacked_group_name"`
	GroupNamesFields      types.List             `tfsdk:"group_names_fields"`       //SpanObservationFieldModel
	StackedGroupNameField types.Object           `tfsdk:"stacked_group_name_field"` //SpanObservationFieldModel
	TimeFrame             *TimeFrameModel        `tfsdk:"time_frame"`
}

type BarChartQueryDataPrimeModel struct {
	Query            types.String    `tfsdk:"query"`
	Filters          types.List      `tfsdk:"filters"`     //DashboardFilterSourceModel
	GroupNames       types.List      `tfsdk:"group_names"` //types.String
	StackedGroupName types.String    `tfsdk:"stacked_group_name"`
	TimeFrame        *TimeFrameModel `tfsdk:"time_frame"`
}

type DataTableSpansAggregationModel struct {
	ID          types.String           `tfsdk:"id"`
	Name        types.String           `tfsdk:"name"`
	IsVisible   types.Bool             `tfsdk:"is_visible"`
	Aggregation *SpansAggregationModel `tfsdk:"aggregation"`
}

type BarChartStackDefinitionModel struct {
	MaxSlicesPerBar   types.Int64  `tfsdk:"max_slices_per_bar"`
	StackNameTemplate types.String `tfsdk:"stack_name_template"`
}

type BarChartXAxisModel struct {
	Time        *BarChartXAxisTimeModel  `tfsdk:"time"`
	Value       *BarChartXAxisValueModel `tfsdk:"value"`
	TimeBuckets *IntervalResolutionModel `tfsdk:"time_buckets"`
}

type IntervalResolutionModel struct {
	Auto             *AutoIntervalResolutionModel   `tfsdk:"auto"`
	Manual           *ManualIntervalResolutionModel `tfsdk:"manual"`
	UseAdvancedLimit types.Bool                     `tfsdk:"use_advanced_limit"`
}

type AutoIntervalResolutionModel struct {
	MaximumDataPoints types.Int64  `tfsdk:"maximum_data_points"`
	MinimumInterval   types.String `tfsdk:"minimum_interval"`
}

type ManualIntervalResolutionModel struct {
	Interval          types.String `tfsdk:"interval"`
	MaximumDataPoints types.Int64  `tfsdk:"maximum_data_points"`
	MinimumInterval   types.String `tfsdk:"minimum_interval"`
}

type BarChartXAxisTimeModel struct {
	Interval         types.String `tfsdk:"interval"`
	BucketsPresented types.Int64  `tfsdk:"buckets_presented"`
}

type BarChartXAxisValueModel struct {
}

type HorizontalBarChartModel struct {
	Query             *HorizontalBarChartQueryModel `tfsdk:"query"`
	MaxBarsPerChart   types.Int64                   `tfsdk:"max_bars_per_chart"`
	GroupNameTemplate types.String                  `tfsdk:"group_name_template"`
	StackDefinition   *BarChartStackDefinitionModel `tfsdk:"stack_definition"`
	ScaleType         types.String                  `tfsdk:"scale_type"`
	ColorsBy          types.String                  `tfsdk:"colors_by"`
	Unit              types.String                  `tfsdk:"unit"`
	DisplayOnBar      types.Bool                    `tfsdk:"display_on_bar"`
	YAxisViewBy       types.String                  `tfsdk:"y_axis_view_by"`
	SortBy            types.String                  `tfsdk:"sort_by"`
	ColorScheme       types.String                  `tfsdk:"color_scheme"`
	HashColors        types.Bool                    `tfsdk:"hash_colors"`
	DataModeType      types.String                  `tfsdk:"data_mode_type"`
	CustomUnit        types.String                  `tfsdk:"custom_unit"`
	Decimal           types.Number                  `tfsdk:"decimal"`
	DecimalPrecision  types.Bool                    `tfsdk:"decimal_precision"`
	Legend            *LegendModel                  `tfsdk:"legend"`
	YAxisMax          Float32Value                  `tfsdk:"y_axis_max"`
	YAxisMin          Float32Value                  `tfsdk:"y_axis_min"`
}

type HorizontalBarChartQueryModel struct {
	Logs      types.Object `tfsdk:"logs"`       //BarChartQueryLogsModel
	Metrics   types.Object `tfsdk:"metrics"`    //BarChartQueryMetricsModel
	Spans     types.Object `tfsdk:"spans"`      //BarChartQuerySpansModel
	DataPrime types.Object `tfsdk:"data_prime"` //BarChartQueryDataPrimeModel
}

type MarkdownModel struct {
	MarkdownText types.String `tfsdk:"markdown_text"`
	TooltipText  types.String `tfsdk:"tooltip_text"`
}

type DashboardFilterSourceModel struct {
	Logs    *FilterSourceLogsModel    `tfsdk:"logs"`
	Metrics *FilterSourceMetricsModel `tfsdk:"metrics"`
	Spans   *FilterSourceSpansModel   `tfsdk:"spans"`
}

// TopLevelFilterSourceModel is the source of a dashboard-level filter. Its
// spans branch carries an observation field, which the widget filter source
// cannot: see TopLevelFilterSourceSchema.
type TopLevelFilterSourceModel struct {
	Logs    *FilterSourceLogsModel       `tfsdk:"logs"`
	Metrics *FilterSourceMetricsModel    `tfsdk:"metrics"`
	Spans   *SpansObservationFilterModel `tfsdk:"spans"`
}

type FilterSourceLogsModel struct {
	Field            types.String         `tfsdk:"field"`
	Operator         *FilterOperatorModel `tfsdk:"operator"`
	ObservationField types.Object         `tfsdk:"observation_field"`
}

type FilterSourceMetricsModel struct {
	MetricName  types.String         `tfsdk:"metric_name"`
	MetricLabel types.String         `tfsdk:"label"`
	Operator    *FilterOperatorModel `tfsdk:"operator"`
}

type FilterSourceSpansModel struct {
	Field    *SpansFieldModel     `tfsdk:"field"`
	Operator *FilterOperatorModel `tfsdk:"operator"`
}

type TimeFrameAbsoluteModel struct {
	Start types.String `tfsdk:"start"`
	End   types.String `tfsdk:"end"`
}

type TimeFrameRelativeModel struct {
	Duration types.String `tfsdk:"duration"`
}

type TimeFrameModel struct {
	Absolute *TimeFrameAbsoluteModel `tfsdk:"absolute"` //TimeFrameAbsoluteModel
	Relative *TimeFrameRelativeModel `tfsdk:"relative"` //TimeFrameRelativeModel
}

type spansFieldValidator struct{}

func (s spansFieldValidator) Description(ctx context.Context) string {
	return ""
}

func (s spansFieldValidator) MarkdownDescription(ctx context.Context) string {
	return ""
}

func (s spansFieldValidator) ValidateObject(ctx context.Context, request validator.ObjectRequest, response *validator.ObjectResponse) {
	if request.ConfigValue.IsNull() {
		return
	}

	var field SpansFieldModel
	diags := request.ConfigValue.As(ctx, &field, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		response.Diagnostics.Append(diags...)
		return
	}
	if field.Type.ValueString() == "metadata" && !slices.Contains(DashboardValidSpanFieldMetadataFields, field.Value.ValueString()) {
		response.Diagnostics.Append(diag.NewErrorDiagnostic("spans field validation failed", fmt.Sprintf("when type is `metadata`, `value` must be one of %q", DashboardValidSpanFieldMetadataFields)))
	}
}

type FilterOperatorModel struct {
	Type           types.String `tfsdk:"type"`
	SelectionType  types.String `tfsdk:"selection_type"`
	SelectedValues types.List   `tfsdk:"selected_values"` //types.String
}

type filterOperatorValidator struct{}

func (f filterOperatorValidator) Description(_ context.Context) string {
	return ""
}

func (f filterOperatorValidator) MarkdownDescription(_ context.Context) string {
	return ""
}

func (f filterOperatorValidator) ValidateObject(ctx context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	if req.ConfigValue.IsNull() {
		return
	}

	var filter FilterOperatorModel
	diags := req.ConfigValue.As(ctx, &filter, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	selectionType := knownFilterSelectionType(filter.SelectionType)
	if filter.Type.ValueString() == "not_equals" && selectionType == filterSelectionTypeAll {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("filter operator validation failed", "when type is `not_equals`, `selection_type` must be `list`"))
	}

	if selectionType == filterSelectionTypeAll && filterSelectedValuesAreKnownNonEmpty(filter.SelectedValues) {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("filter operator validation failed", "when selection_type is `all`, `selected_values` must be empty"))
	}

	if filter.Type.ValueString() == "not_equals" && filterSelectedValuesAreMissing(filter.SelectedValues) {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("filter operator validation failed", "when type is `not_equals`, `selected_values` must contain at least one value"))
	}
}

func knownFilterSelectionType(value types.String) string {
	if value.IsNull() || value.IsUnknown() {
		return ""
	}
	return value.ValueString()
}

func filterSelectedValuesAreKnownNonEmpty(value types.List) bool {
	return !value.IsNull() && !value.IsUnknown() && len(value.Elements()) != 0
}

func filterSelectedValuesAreMissing(value types.List) bool {
	return value.IsNull() || (!value.IsUnknown() && len(value.Elements()) == 0)
}

type LegendModel struct {
	IsVisible    types.Bool   `tfsdk:"is_visible"`
	Columns      types.List   `tfsdk:"columns"` //types.String (DashboardValidLegendColumns)
	GroupByQuery types.Bool   `tfsdk:"group_by_query"`
	Placement    types.String `tfsdk:"placement"`
}

type spansAggregationValidator struct{}

func (s spansAggregationValidator) Description(ctx context.Context) string {
	return ""
}

func (s spansAggregationValidator) MarkdownDescription(ctx context.Context) string {
	return ""
}

func (s spansAggregationValidator) ValidateObject(ctx context.Context, request validator.ObjectRequest, response *validator.ObjectResponse) {
	if request.ConfigValue.IsNull() {
		return
	}

	var aggregation SpansAggregationModel
	diags := request.ConfigValue.As(ctx, &aggregation, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		response.Diagnostics.Append(diags...)
		return
	}

	if aggregation.Type.ValueString() == "metrics" && !slices.Contains(DashboardValidSpansAggregationMetricAggregationTypes, aggregation.AggregationType.ValueString()) {
		response.Diagnostics.Append(diag.NewErrorDiagnostic("spans aggregation validation failed", fmt.Sprintf("when type is `metrics`, `aggregation_type` must be one of %q", DashboardValidSpansAggregationMetricAggregationTypes)))
	}
	if aggregation.Type.ValueString() == "dimension" && !slices.Contains(DashboardValidSpansAggregationDimensionAggregationTypes, aggregation.AggregationType.ValueString()) {
		response.Diagnostics.Append(diag.NewErrorDiagnostic("spans aggregation validation failed", fmt.Sprintf("when type is `dimension`, `aggregation_type` must be one of %q", DashboardValidSpansAggregationDimensionAggregationTypes)))
	}
}

type logsAggregationValidator struct{}

func (l logsAggregationValidator) Description(ctx context.Context) string {
	return ""
}

func (l logsAggregationValidator) MarkdownDescription(ctx context.Context) string {
	return ""
}

func (l logsAggregationValidator) ValidateObject(ctx context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	if req.ConfigValue.IsNull() {
		return
	}

	var aggregation LogsAggregationModel
	diags := req.ConfigValue.As(ctx, &aggregation, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	if aggregation.Type.IsNull() || aggregation.Type.IsUnknown() {
		return
	}

	aggregationType := aggregation.Type.ValueString()
	validateLogsAggregationFields(aggregationType, aggregation, resp)
	validateLogsAggregationPercent(aggregationType, aggregation.Percent, resp)
}

func validateLogsAggregationFields(aggregationType string, aggregation LogsAggregationModel, resp *validator.ObjectResponse) {
	fieldKnownSet := !aggregation.Field.IsNull() && !aggregation.Field.IsUnknown()
	fieldKnownUnset := aggregation.Field.IsNull()
	obsKnownSet := !aggregation.ObservationField.IsNull() && !aggregation.ObservationField.IsUnknown()
	obsKnownUnset := aggregation.ObservationField.IsNull()

	if aggregationType == "count" {
		if fieldKnownSet || obsKnownSet {
			resp.Diagnostics.Append(diag.NewErrorDiagnostic("logs aggregation validation failed", "when type is `count`, neither `field` nor `observation_field` can be set"))
		}
	} else {
		if fieldKnownUnset && obsKnownUnset {
			resp.Diagnostics.Append(diag.NewErrorDiagnostic("logs aggregation validation failed", fmt.Sprintf("when type is `%s`, either `field` or `observation_field` must be set", aggregationType)))
		} else if fieldKnownSet && obsKnownSet {
			resp.Diagnostics.Append(diag.NewErrorDiagnostic("logs aggregation validation failed", fmt.Sprintf("when type is `%s`, `field` and `observation_field` are mutually exclusive — set exactly one", aggregationType)))
		}
	}
}

func validateLogsAggregationPercent(aggregationType string, percent types.Float64, resp *validator.ObjectResponse) {
	percentKnownSet := !percent.IsNull() && !percent.IsUnknown()
	percentKnownUnset := percent.IsNull()

	if aggregationType == "percentile" && percentKnownUnset {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("logs aggregation validation failed", "when type is `percentile`, `percent` must be set"))
	} else if aggregationType != "percentile" && percentKnownSet {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("logs aggregation validation failed", fmt.Sprintf("when type is `%s`, `percent` cannot be set", aggregationType)))
	}
}

func FlattenLegend(legend *dashboardservice.Legend) *LegendModel {
	if legend == nil {
		return nil
	}

	return &LegendModel{
		IsVisible:    types.BoolPointerValue(legend.IsVisible),
		GroupByQuery: types.BoolPointerValue(legend.GroupByQuery),
		Columns:      flattenLegendColumns(legend.GetColumns()),
		Placement:    FlattenEnum(legend.GetPlacement(), DashboardLegendPlacementProtoToSchema),
	}
}

func flattenLegendColumns(columns []dashboardservice.LegendColumn) types.List {
	if len(columns) == 0 {
		return types.ListNull(types.StringType)
	}

	columnsElements := make([]attr.Value, 0, len(columns))
	for _, column := range columns {
		flattenedColumn := DashboardLegendColumnProtoToSchema[column]
		columnElement := types.StringValue(flattenedColumn)
		columnsElements = append(columnsElements, columnElement)
	}

	return types.ListValueMust(types.StringType, columnsElements)
}

func ExpandLegend(ctx context.Context, legend *LegendModel) (*dashboardservice.Legend, diag.Diagnostics) {
	if legend == nil {
		return nil, nil
	}

	columns := make([]dashboardservice.LegendColumn, 0, len(legend.Columns.Elements()))
	var columnsParsed []types.String
	if diags := legend.Columns.ElementsAs(ctx, &columnsParsed, true); diags.HasError() {
		return nil, diags
	}
	var diagnostics diag.Diagnostics
	for _, col := range columnsParsed {
		columns = append(columns, DashboardLegendColumnSchemaToProto[col.ValueString()])
	}
	if diagnostics.HasError() {
		return nil, diagnostics
	}

	return &dashboardservice.Legend{
		IsVisible:    legend.IsVisible.ValueBoolPointer(),
		Columns:      columns,
		GroupByQuery: legend.GroupByQuery.ValueBoolPointer(),
		Placement:    OptionalEnumPointer(legend.Placement, DashboardLegendPlacementSchemaToProto),
	}, nil
}

func FlattenSpansFields(ctx context.Context, spanFields []dashboardservice.SpanField) (types.List, diag.Diagnostics) {
	if len(spanFields) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: SpansFieldModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	spanFieldElements := make([]attr.Value, 0, len(spanFields))
	for _, field := range spanFields {
		flattenedField, dg := FlattenSpansField(&field)
		if dg != nil {
			diagnostics.Append(dg)
			continue
		}
		fieldElement, diags := types.ObjectValueFrom(ctx, SpansFieldModelAttr(), flattenedField)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		spanFieldElements = append(spanFieldElements, fieldElement)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: SpansFieldModelAttr()}), diagnostics
	}

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: SpansFieldModelAttr()}, spanFieldElements)
}

func FlattenSpansField(field *dashboardservice.SpanField) (*SpansFieldModel, diag.Diagnostic) {
	if field == nil {
		return nil, nil
	}

	switch {
	case field.MetadataField != nil:
		return &SpansFieldModel{
			Type:  types.StringValue("metadata"),
			Value: types.StringValue(DashboardProtoToSchemaSpanFieldMetadataField[field.GetMetadataField()]),
		}, nil
	case field.TagField != nil:
		return &SpansFieldModel{
			Type:  types.StringValue("tag"),
			Value: types.StringPointerValue(field.TagField),
		}, nil
	case field.ProcessTagField != nil:
		return &SpansFieldModel{
			Type:  types.StringValue("process_tag"),
			Value: types.StringPointerValue(field.ProcessTagField),
		}, nil

	default:
		return nil, diag.NewErrorDiagnostic("Error Flatten Spans Field", "unknown spans field type")
	}
}

func ObservationFieldsObject() types.ObjectType {
	return types.ObjectType{
		AttrTypes: ObservationFieldAttr(),
	}
}

func typeStringListToStringSlice(ctx context.Context, list types.List) ([]string, diag.Diagnostics) {
	if list.IsNull() || list.IsUnknown() {
		return nil, nil
	}
	var values []types.String
	diags := list.ElementsAs(ctx, &values, true)
	if diags.HasError() {
		return nil, diags
	}
	return utils.TypeStringSliceToStringSlice(values), nil
}

func int64ToInt32Pointer(value types.Int64) *int32 {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	converted := int32(value.ValueInt64())
	return &converted
}

func int32PointerToInt64Type(value *int32) types.Int64 {
	if value == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*value))
}

func numberTypeToFloat64Pointer(value types.Number) *float64 {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	converted, _ := value.ValueBigFloat().Float64()
	return &converted
}

func float64PointerToNumberType(value *float64) types.Number {
	if value == nil {
		return types.NumberNull()
	}
	return types.NumberValue(big.NewFloat(*value))
}

func numberTypeToInt32Pointer(value types.Number) *int32 {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	converted, _ := value.ValueBigFloat().Int64()
	result := int32(converted)
	return &result
}

func int32PointerToNumberType(value *int32) types.Number {
	if value == nil {
		return types.NumberNull()
	}
	return types.NumberValue(big.NewFloat(float64(*value)))
}

func FlattenDashboardFiltersSources(ctx context.Context, sources []dashboardservice.FilterSource) (types.List, diag.Diagnostics) {
	if len(sources) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: FilterSourceModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0, len(sources))
	for i := range sources {
		flattenedFilter, diags := FlattenDashboardFilterSource(ctx, &sources[i])
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		filterElement, diags := types.ObjectValueFrom(ctx, FilterSourceModelAttr(), flattenedFilter)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		filtersElements = append(filtersElements, filterElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: FilterSourceModelAttr()}, filtersElements), diagnostics
}

func FlattenDashboardFilterSource(ctx context.Context, source *dashboardservice.FilterSource) (*DashboardFilterSourceModel, diag.Diagnostics) {
	if source == nil {
		return nil, nil
	}

	switch {
	case source.Logs != nil:
		logs, diags := FlattenDashboardFilterSourceLogs(ctx, source.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &DashboardFilterSourceModel{Logs: logs}, nil
	case source.Spans != nil:
		spans, dg := FlattenDashboardFilterSourceSpans(source.Spans)
		if dg != nil {
			return nil, diag.Diagnostics{dg}
		}
		return &DashboardFilterSourceModel{Spans: spans}, nil
	case source.Metrics != nil:
		metrics, dg := FlattenDashboardFilterSourceMetrics(source.Metrics)
		if dg != nil {
			return nil, diag.Diagnostics{dg}
		}
		return &DashboardFilterSourceModel{Metrics: metrics}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Filter Source", fmt.Sprintf("unknown filter source type %T", source))}
	}
}

func FlattenDashboardFilterSourceLogs(ctx context.Context, logs *dashboardservice.FilterLogsFilter) (*FilterSourceLogsModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	operator, dg := FlattenFilterOperator(logs.Operator)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	observationField, diags := FlattenObservationField(ctx, logs.ObservationField)
	if diags.HasError() {
		return nil, diags
	}

	return &FilterSourceLogsModel{
		Field:            utils.StringPointerToTypeString(logs.Field),
		Operator:         operator,
		ObservationField: observationField,
	}, nil
}

func FlattenDashboardFilterSourceSpans(spans *dashboardservice.SpansFilter) (*FilterSourceSpansModel, diag.Diagnostic) {
	if spans == nil {
		return nil, nil
	}

	field, dg := FlattenSpansField(spans.Field)
	if dg != nil {
		return nil, dg
	}

	operator, dg := FlattenFilterOperator(spans.Operator)
	if dg != nil {
		return nil, dg
	}

	return &FilterSourceSpansModel{
		Field:    field,
		Operator: operator,
	}, nil
}

func FlattenDashboardFilterSourceMetrics(metrics *dashboardservice.MetricsFilter) (*FilterSourceMetricsModel, diag.Diagnostic) {
	if metrics == nil {
		return nil, nil
	}

	operator, dg := FlattenFilterOperator(metrics.Operator)
	if dg != nil {
		return nil, dg
	}

	return &FilterSourceMetricsModel{
		MetricName:  utils.StringPointerToTypeString(metrics.Metric),
		MetricLabel: utils.StringPointerToTypeString(metrics.Label),
		Operator:    operator,
	}, nil
}

func FlattenDashboardTimeFrame(ctx context.Context, d *dashboardservice.Dashboard) (*TimeFrameModel, diag.Diagnostics) {
	switch {
	case d == nil:
		return nil, nil
	case d.AbsoluteTimeFrame != nil:
		return flattenAbsoluteTimeFrame(ctx, d.AbsoluteTimeFrame)
	case d.RelativeTimeFrame != nil:
		return flattenRelativeTimeFrame(ctx, d.RelativeTimeFrame)
	default:
		return nil, nil
	}
}

func FlattenTimeFrameSelect(ctx context.Context, d *dashboardservice.TimeFrameSelect) (*TimeFrameModel, diag.Diagnostics) {
	if d == nil {
		return nil, nil
	}
	switch {
	case d.AbsoluteTimeFrame != nil:
		return flattenAbsoluteTimeFrame(ctx, d.AbsoluteTimeFrame)
	case d.RelativeTimeFrame != nil:
		return flattenRelativeTimeFrame(ctx, d.RelativeTimeFrame)
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Time Frame", fmt.Sprintf("unknown time frame type %T", d))}
	}
}

func FlattenObservationField(ctx context.Context, field *dashboardservice.ObservationField) (types.Object, diag.Diagnostics) {
	if field == nil {
		return types.ObjectNull(ObservationFieldAttr()), nil
	}

	return types.ObjectValueFrom(ctx, ObservationFieldAttr(), FlattenLogsFieldModel(field))
}

func FlattenLogsFieldModel(field *dashboardservice.ObservationField) *ObservationFieldModel {
	return &ObservationFieldModel{
		Keypath: utils.StringSliceToTypeStringList(field.GetKeypath()),
		Scope:   types.StringValue(DashboardProtoToSchemaObservationFieldScope[field.GetScope()]),
	}
}

func flattenDuration(timeFrame *string) basetypes.StringValue {
	return openAPIDurationToLegacy(timeFrame)
}

func flattenAbsoluteTimeFrame(ctx context.Context, timeFrame *dashboardservice.TimeFrame) (*TimeFrameModel, diag.Diagnostics) {
	absoluteTimeFrame := &TimeFrameAbsoluteModel{
		Start: types.StringValue(timeFrame.GetFrom().Format(time.RFC3339Nano)),
		End:   types.StringValue(timeFrame.GetTo().Format(time.RFC3339Nano)),
	}

	flattenedTimeFrame := &TimeFrameModel{
		Relative: nil,
		Absolute: absoluteTimeFrame,
	}
	return flattenedTimeFrame, nil
}

func flattenRelativeTimeFrame(ctx context.Context, timeFrame *string) (*TimeFrameModel, diag.Diagnostics) {
	relativeTimeFrame := &TimeFrameRelativeModel{
		Duration: flattenDuration(timeFrame),
	}

	flattenedTimeFrame := &TimeFrameModel{
		Relative: relativeTimeFrame,
		Absolute: nil,
	}
	return flattenedTimeFrame, nil
}

func FlattenSpansFilters(ctx context.Context, filters []dashboardservice.SpansFilter) (types.List, diag.Diagnostics) {
	if len(filters) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: SpansFilterModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0, len(filters))
	for i := range filters {
		flattenedFilter, dg := FlattenSpansFilter(&filters[i])
		if dg != nil {
			diagnostics.Append(dg)
			continue
		}
		filterElement, diags := types.ObjectValueFrom(ctx, SpansFilterModelAttr(), flattenedFilter)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		filtersElements = append(filtersElements, filterElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: SpansFilterModelAttr()}, filtersElements), diagnostics
}

func FlattenSpansFilter(filter *dashboardservice.SpansFilter) (*SpansFilterModel, diag.Diagnostic) {
	if filter == nil {
		return nil, nil
	}

	operator, dg := FlattenFilterOperator(filter.Operator)
	if dg != nil {
		return nil, dg
	}

	field, dg := FlattenSpansField(filter.Field)
	if dg != nil {
		return nil, dg
	}

	return &SpansFilterModel{
		Field:    field,
		Operator: operator,
	}, nil
}

func FlattenFilterOperator(operator *dashboardservice.FilterOperator) (*FilterOperatorModel, diag.Diagnostic) {
	if operator == nil {
		return nil, nil
	}

	switch {
	case operator.Equals != nil:
		switch {
		case operator.Equals.Selection != nil && operator.Equals.Selection.All != nil:
			return &FilterOperatorModel{
				Type:           types.StringValue("equals"),
				SelectionType:  types.StringValue(filterSelectionTypeAll),
				SelectedValues: types.ListValueMust(types.StringType, []attr.Value{}),
			}, nil
		case operator.Equals.Selection != nil && operator.Equals.Selection.List != nil:
			return &FilterOperatorModel{
				Type:           types.StringValue("equals"),
				SelectionType:  types.StringValue(filterSelectionTypeList),
				SelectedValues: flattenFilterSelectedValues(operator.Equals.Selection.List.GetValues()),
			}, nil
		default:
			return nil, diag.NewErrorDiagnostic("Error Flatten Logs Filter Operator Equals", "unknown logs filter operator equals selection type")
		}
	case operator.NotEquals != nil:
		switch {
		case operator.NotEquals.Selection != nil && operator.NotEquals.Selection.List != nil:
			return &FilterOperatorModel{
				Type:           types.StringValue("not_equals"),
				SelectionType:  types.StringValue(filterSelectionTypeList),
				SelectedValues: flattenFilterSelectedValues(operator.NotEquals.Selection.List.GetValues()),
			}, nil
		default:
			return nil, diag.NewErrorDiagnostic("Error Flatten Logs Filter Operator NotEquals", "unknown logs filter operator not_equals selection type")
		}
	default:
		return nil, diag.NewErrorDiagnostic("Error Flatten Logs Filter Operator", "unknown logs filter operator type")
	}
}

func flattenFilterSelectedValues(values []string) types.List {
	if len(values) == 0 {
		return types.ListValueMust(types.StringType, []attr.Value{})
	}
	return utils.StringSliceToTypeStringList(values)
}

func FlattenMetricsFilters(ctx context.Context, filters []dashboardservice.MetricsFilter) (types.List, diag.Diagnostics) {
	if len(filters) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: MetricsFilterModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0, len(filters))
	for i := range filters {
		flattenedFilter, dg := FlattenMetricsFilter(&filters[i])
		if dg != nil {
			diagnostics.Append(dg)
			continue
		}
		filterElement, diags := types.ObjectValueFrom(ctx, MetricsFilterModelAttr(), flattenedFilter)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		filtersElements = append(filtersElements, filterElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: MetricsFilterModelAttr()}, filtersElements), diagnostics
}

func FlattenMetricsFilter(filter *dashboardservice.MetricsFilter) (*MetricsFilterModel, diag.Diagnostic) {
	if filter == nil {
		return nil, nil
	}

	operator, dg := FlattenFilterOperator(filter.Operator)
	if dg != nil {
		return nil, dg
	}

	return &MetricsFilterModel{
		Metric:   utils.StringPointerToTypeString(filter.Metric),
		Label:    utils.StringPointerToTypeString(filter.Label),
		Operator: operator,
	}, nil
}

func FlattenLogsFilters(ctx context.Context, filters []dashboardservice.FilterLogsFilter) (types.List, diag.Diagnostics) {
	if len(filters) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: LogsFilterModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0, len(filters))
	for i := range filters {
		flattenedFilter, diags := flattenLogsFilter(ctx, &filters[i])
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		filterElement, diags := types.ObjectValueFrom(ctx, LogsFilterModelAttr(), flattenedFilter)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		filtersElements = append(filtersElements, filterElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: LogsFilterModelAttr()}, filtersElements), diagnostics
}

func flattenLogsFilter(ctx context.Context, filter *dashboardservice.FilterLogsFilter) (*LogsFilterModel, diag.Diagnostics) {
	if filter == nil {
		return nil, nil
	}

	operator, dg := FlattenFilterOperator(filter.Operator)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	observationField, diags := FlattenObservationField(ctx, filter.ObservationField)
	if diags.HasError() {
		return nil, diags
	}

	return &LogsFilterModel{
		Field:            utils.StringPointerToTypeString(filter.Field),
		Operator:         operator,
		ObservationField: observationField,
	}, nil
}

func FlattenObservationFields(ctx context.Context, namesFields []dashboardservice.ObservationField) (types.List, diag.Diagnostics) {
	if len(namesFields) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: ObservationFieldAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	fieldElements := make([]attr.Value, 0, len(namesFields))
	for i := range namesFields {
		flattenedField, diags := FlattenObservationField(ctx, &namesFields[i])
		if diags != nil {
			diagnostics.Append(diags...)
			continue
		}
		fieldElement, diags := types.ObjectValueFrom(ctx, ObservationFieldAttr(), flattenedField)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		fieldElements = append(fieldElements, fieldElement)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: ObservationFieldAttr()}), diagnostics
	}

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: ObservationFieldAttr()}, fieldElements)
}

func FlattenLogsAggregation(ctx context.Context, aggregation *dashboardservice.LogsAggregation) (*LogsAggregationModel, diag.Diagnostics) {
	if aggregation == nil {
		return nil, nil
	}

	switch {
	case aggregation.Count != nil:
		return &LogsAggregationModel{
			Type:             types.StringValue("count"),
			ObservationField: types.ObjectNull(ObservationFieldAttr()),
		}, nil
	case aggregation.CountDistinct != nil:
		observationField, diags := FlattenObservationField(ctx, aggregation.CountDistinct.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("count_distinct"),
			Field:            utils.StringPointerToTypeString(aggregation.CountDistinct.Field),
			ObservationField: observationField,
		}, nil
	case aggregation.Sum != nil:
		observationField, diags := FlattenObservationField(ctx, aggregation.Sum.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("sum"),
			Field:            utils.StringPointerToTypeString(aggregation.Sum.Field),
			ObservationField: observationField,
		}, nil
	case aggregation.Average != nil:
		observationField, diags := FlattenObservationField(ctx, aggregation.Average.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("avg"),
			Field:            utils.StringPointerToTypeString(aggregation.Average.Field),
			ObservationField: observationField,
		}, nil
	case aggregation.Min != nil:
		observationField, diags := FlattenObservationField(ctx, aggregation.Min.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("min"),
			Field:            utils.StringPointerToTypeString(aggregation.Min.Field),
			ObservationField: observationField,
		}, nil
	case aggregation.Max != nil:
		observationField, diags := FlattenObservationField(ctx, aggregation.Max.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("max"),
			Field:            utils.StringPointerToTypeString(aggregation.Max.Field),
			ObservationField: observationField,
		}, nil
	case aggregation.Percentile != nil:
		observationField, diags := FlattenObservationField(ctx, aggregation.Percentile.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &LogsAggregationModel{
			Type:             types.StringValue("percentile"),
			Field:            utils.StringPointerToTypeString(aggregation.Percentile.Field),
			Percent:          types.Float64PointerValue(aggregation.Percentile.Percent),
			ObservationField: observationField,
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Logs Aggregation", "unknown logs aggregation type")}
	}
}

func ExpandObservationFields(ctx context.Context, namesFields types.List) ([]dashboardservice.ObservationField, diag.Diagnostics) {
	var namesFieldsObjects []types.Object
	var expandedNamesFields []dashboardservice.ObservationField
	diags := namesFields.ElementsAs(ctx, &namesFieldsObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, nfo := range namesFieldsObjects {
		var namesField ObservationFieldModel
		if dg := nfo.As(ctx, &namesField, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedNamesField, expandDiags := expandObservationField(ctx, namesField)
		if expandDiags != nil {
			diags.Append(expandDiags...)
			continue
		}
		expandedNamesFields = append(expandedNamesFields, *expandedNamesField)
	}

	return expandedNamesFields, diags
}

func ExpandObservationFieldObject(ctx context.Context, field types.Object) (*dashboardservice.ObservationField, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(field) {
		return nil, nil
	}

	var observationField ObservationFieldModel
	if dg := field.As(ctx, &observationField, basetypes.ObjectAsOptions{}); dg.HasError() {
		return nil, dg
	}

	return expandObservationField(ctx, observationField)
}

func expandObservationField(ctx context.Context, observationField ObservationFieldModel) (*dashboardservice.ObservationField, diag.Diagnostics) {
	keypath, dg := typeStringListToStringSlice(ctx, observationField.Keypath)
	if dg.HasError() {
		return nil, dg
	}

	scope := DashboardSchemaToProtoObservationFieldScope[observationField.Scope.ValueString()]

	return &dashboardservice.ObservationField{
		Keypath: keypath,
		Scope:   scope.Ptr(),
	}, nil
}

func ExpandSpansField(spansFilterField *SpansFieldModel) (*dashboardservice.SpanField, diag.Diagnostic) {
	if spansFilterField == nil {
		return nil, nil
	}

	switch spansFilterField.Type.ValueString() {
	case "metadata":
		return &dashboardservice.SpanField{
			MetadataField: OptionalEnumPointer(spansFilterField.Value, DashboardSchemaToProtoSpanFieldMetadataField),
		}, nil
	case "tag":
		return &dashboardservice.SpanField{
			TagField: utils.TypeStringToStringPointer(spansFilterField.Value),
		}, nil
	case "process_tag":
		return &dashboardservice.SpanField{
			ProcessTagField: utils.TypeStringToStringPointer(spansFilterField.Value),
		}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Extract Spans Filter Field Error", fmt.Sprintf("Unknown spans filter field type %s", spansFilterField.Type.ValueString()))
	}
}

func ExpandSpansFields(ctx context.Context, spanFields types.List) ([]dashboardservice.SpanField, diag.Diagnostics) {
	var spanFieldsObjects []types.Object
	var expandedSpanFields []dashboardservice.SpanField
	diags := spanFields.ElementsAs(ctx, &spanFieldsObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, sfo := range spanFieldsObjects {
		var spansField SpansFieldModel
		if dg := sfo.As(ctx, &spansField, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedSpanField, expandDiag := ExpandSpansField(&spansField)
		if expandDiag != nil {
			diags.Append(expandDiag)
			continue
		}
		expandedSpanFields = append(expandedSpanFields, *expandedSpanField)
	}

	return expandedSpanFields, diags
}

func ExpandLogsAggregations(ctx context.Context, logsAggregations types.List) ([]dashboardservice.LogsAggregation, diag.Diagnostics) {
	var logsAggregationsObjects []types.Object
	var expandedLogsAggregations []dashboardservice.LogsAggregation
	diags := logsAggregations.ElementsAs(ctx, &logsAggregationsObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, qdo := range logsAggregationsObjects {
		var aggregation LogsAggregationModel
		if dg := qdo.As(ctx, &aggregation, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedLogsAggregation, expandDiags := ExpandLogsAggregation(ctx, &aggregation)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedLogsAggregations = append(expandedLogsAggregations, *expandedLogsAggregation)
	}

	return expandedLogsAggregations, diags
}

func ExpandLogsAggregation(ctx context.Context, logsAggregation *LogsAggregationModel) (*dashboardservice.LogsAggregation, diag.Diagnostics) {
	if logsAggregation == nil {
		return nil, nil
	}
	switch logsAggregation.Type.ValueString() {
	case "count":
		return &dashboardservice.LogsAggregation{
			Count: map[string]interface{}{},
		}, nil
	case "count_distinct":
		observationField, diags := ExpandObservationFieldObject(ctx, logsAggregation.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.LogsAggregation{
			CountDistinct: &dashboardservice.CountDistinct{
				Field:            utils.TypeStringToStringPointer(logsAggregation.Field),
				ObservationField: observationField,
			},
		}, nil
	case "sum":
		observationField, diags := ExpandObservationFieldObject(ctx, logsAggregation.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.LogsAggregation{
			Sum: &dashboardservice.Sum{
				Field:            utils.TypeStringToStringPointer(logsAggregation.Field),
				ObservationField: observationField,
			},
		}, nil
	case "avg":
		observationField, diags := ExpandObservationFieldObject(ctx, logsAggregation.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.LogsAggregation{
			Average: &dashboardservice.Average{
				Field:            utils.TypeStringToStringPointer(logsAggregation.Field),
				ObservationField: observationField,
			},
		}, nil
	case "min":
		observationField, diags := ExpandObservationFieldObject(ctx, logsAggregation.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.LogsAggregation{
			Min: &dashboardservice.Min{
				Field:            utils.TypeStringToStringPointer(logsAggregation.Field),
				ObservationField: observationField,
			},
		}, nil
	case "max":
		observationField, diags := ExpandObservationFieldObject(ctx, logsAggregation.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.LogsAggregation{
			Max: &dashboardservice.Max{
				Field:            utils.TypeStringToStringPointer(logsAggregation.Field),
				ObservationField: observationField,
			},
		}, nil
	case "percentile":
		observationField, diags := ExpandObservationFieldObject(ctx, logsAggregation.ObservationField)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.LogsAggregation{
			Percentile: &dashboardservice.Percentile{
				Field:            utils.TypeStringToStringPointer(logsAggregation.Field),
				Percent:          logsAggregation.Percent.ValueFloat64Pointer(),
				ObservationField: observationField,
			},
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error expand logs aggregation", fmt.Sprintf("unknown logs aggregation type %s", logsAggregation.Type.ValueString()))}
	}
}

func ExpandTimeFrameSelect(ctx context.Context, timeFrame *TimeFrameModel) (*dashboardservice.TimeFrameSelect, diag.Diagnostics) {
	if timeFrame == nil {
		return nil, nil
	}

	tf := dashboardservice.TimeFrameSelect{}

	switch {
	case timeFrame.Relative != nil:
		val, diags := expandRelativeTimeFrame(ctx, timeFrame.Relative)
		if diags.HasError() {
			return nil, diags
		}
		tf.RelativeTimeFrame = val
	case timeFrame.Absolute != nil:
		absoluteTimeFrame, diags := expandAbsoluteTimeFrame(ctx, timeFrame.Absolute)
		if diags.HasError() {
			return nil, diags
		}
		tf.AbsoluteTimeFrame = absoluteTimeFrame
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Time Frame", "Dashboard TimeFrame must be either Relative or Absolute")}
	}
	return &tf, nil
}

func ExpandDashboardTimeFrame(ctx context.Context, dashboard *dashboardservice.Dashboard, timeFrame *TimeFrameModel) (*dashboardservice.Dashboard, diag.Diagnostics) {
	if timeFrame == nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("No time frame received", "time frame was nil")}
	}

	var diags diag.Diagnostics
	switch {
	case timeFrame.Relative != nil:
		relative, diags := expandRelativeTimeFrame(ctx, timeFrame.Relative)
		if diags.HasError() {
			return nil, diags
		}
		dashboard.RelativeTimeFrame = relative
	case timeFrame.Absolute != nil:
		absoluteTimeFrame, diags := expandAbsoluteTimeFrame(ctx, timeFrame.Absolute)
		if diags.HasError() {
			return nil, diags
		}
		dashboard.AbsoluteTimeFrame = absoluteTimeFrame
	default:
		diags = diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Time Frame", "Dashboard TimeFrame must be either Relative or Absolute")}
	}
	return dashboard, diags
}

func expandRelativeTimeFrame(ctx context.Context, timeFrame *TimeFrameRelativeModel) (*string, diag.Diagnostics) {
	duration, dg := legacyDurationToOpenAPI(timeFrame.Duration.ValueString(), "Relative Dashboard Time Frame")
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}
	return duration, nil
}

func expandAbsoluteTimeFrame(ctx context.Context, timeFrame *TimeFrameAbsoluteModel) (*dashboardservice.TimeFrame, diag.Diagnostics) {
	fromTime, err := time.Parse(time.RFC3339, timeFrame.Start.ValueString())
	if err != nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Absolute Dashboard Time Frame", fmt.Sprintf("Error parsing from time: %s", err.Error()))}
	}

	toTime, err := time.Parse(time.RFC3339, timeFrame.End.ValueString())
	if err != nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Absolute Dashboard Time Frame", fmt.Sprintf("Error parsing from time: %s", err.Error()))}
	}

	return &dashboardservice.TimeFrame{
		From: &fromTime,
		To:   &toTime,
	}, nil
}

func SupportedWidgetsValidatorWithout(current string) validator.Object {
	matchers := make([]path.Expression, 0, len(SupportedWidgetTypes)-1)
	for _, name := range SupportedWidgetTypes {
		if name != current {
			matchers = append(matchers, path.MatchRelative().AtParent().AtName(name))
		}
	}
	return ExactlyOneOfObject(matchers...)
}

// ExactlyOneOfChildren validates that exactly one of the named direct child
// attributes of this object is non-null. Unlike ExactlyOneOfObject/
// objectvalidator.ExactlyOneOf, it reads req.ConfigValue directly instead of
// resolving path.Expressions via Config.PathMatches, so it doesn't pay for a
// full config-tree walk per check. Attach it to the parent object of a oneof
// group instead of to each child.
func ExactlyOneOfChildren(childNames ...string) validator.Object {
	return ExactlyOneOfChildrenValidator{ChildNames: childNames}
}

// SupportedWidgetsExactlyOneOfChildren is the cheap, parent-attached
// equivalent of SupportedWidgetsValidatorWithout: attach it once to the
// widget "definition" object instead of once per widget type.
func SupportedWidgetsExactlyOneOfChildren() validator.Object {
	return ExactlyOneOfChildren(SupportedWidgetTypes...)
}

// ExactlyOneOfChildrenValidator is exported (rather than kept as an
// unexported implementation type) so schema-wiring tests in other packages
// can type-assert a validator.Object and inspect ChildNames directly, to
// assert it hasn't silently drifted from the actual attribute map it's
// attached to (ChildNames is a plain string list with no compiler tie back
// to the schema's attribute keys).
type ExactlyOneOfChildrenValidator struct {
	ChildNames []string
}

func (v ExactlyOneOfChildrenValidator) Description(_ context.Context) string {
	return fmt.Sprintf("exactly one of %v must be configured", v.ChildNames)
}

func (v ExactlyOneOfChildrenValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v ExactlyOneOfChildrenValidator) ValidateObject(_ context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	attrs := req.ConfigValue.Attributes()
	var knownChildrenSet []string
	unknownCount := 0
	for _, name := range v.ChildNames {
		val, ok := attrs[name]
		if !ok {
			continue
		}
		if val.IsUnknown() {
			unknownCount++
			continue
		}
		if !val.IsNull() {
			knownChildrenSet = append(knownChildrenSet, name)
		}
	}

	// A second known-and-set child is an unavoidable conflict no matter how
	// any remaining unknown children resolve, so report it immediately.
	if len(knownChildrenSet) > 1 {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid Attribute Combination",
			fmt.Sprintf("Only one of these attributes can be configured: `%s`.", strings.Join(knownChildrenSet, "`, `")))
		return
	}

	// With at most one known-and-set child, an unknown sibling could still
	// resolve to satisfy (or break) "exactly one" — defer instead of
	// reporting a false-positive "none configured" error.
	if unknownCount > 0 {
		return
	}

	if len(knownChildrenSet) == 0 {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid Attribute Combination",
			"No attribute was configured in this one-of group. Configure exactly one value.")
	}
}

// AtLeastOneOfChildren validates that at least one named direct child attribute
// is set when the parent object is configured.
// AtMostOneOfChildren rejects two children set together while allowing none.
// Use it where the API treats the children as alternatives but does not require
// one, so requiring exactly one would reject a shape the API stores.
func AtMostOneOfChildren(childNames ...string) validator.Object {
	return AtMostOneOfChildrenValidator{ChildNames: childNames}
}

type AtMostOneOfChildrenValidator struct {
	ChildNames []string
}

func (v AtMostOneOfChildrenValidator) Description(_ context.Context) string {
	return fmt.Sprintf("at most one of these attributes may be configured: %s", strings.Join(v.ChildNames, ", "))
}

func (v AtMostOneOfChildrenValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v AtMostOneOfChildrenValidator) ValidateObject(_ context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	attrs := req.ConfigValue.Attributes()
	var set []string
	for _, name := range v.ChildNames {
		value, ok := attrs[name]
		if !ok || value.IsUnknown() || value.IsNull() {
			continue
		}
		set = append(set, name)
	}

	if len(set) > 1 {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid Attribute Combination",
			fmt.Sprintf("Only one of these attributes can be configured: `%s`. Omit both to leave the choice to the backend.", strings.Join(set, "`, `")))
	}
}

func AtLeastOneOfChildren(childNames ...string) validator.Object {
	return AtLeastOneOfChildrenValidator{ChildNames: childNames}
}

// AtLeastOneOfChildrenValidator is the object validator behind AtLeastOneOfChildren.
type AtLeastOneOfChildrenValidator struct {
	ChildNames []string
}

func (v AtLeastOneOfChildrenValidator) Description(_ context.Context) string {
	return fmt.Sprintf("At least one of these attributes must be configured: `%s`.", strings.Join(v.ChildNames, "`, `"))
}

func (v AtLeastOneOfChildrenValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v AtLeastOneOfChildrenValidator) ValidateObject(_ context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	attrs := req.ConfigValue.Attributes()
	knownSet := 0
	unknownCount := 0
	for _, name := range v.ChildNames {
		val, ok := attrs[name]
		if !ok {
			continue
		}
		if val.IsUnknown() {
			unknownCount++
			continue
		}
		if !val.IsNull() {
			knownSet++
		}
	}
	if unknownCount > 0 || knownSet > 0 {
		return
	}
	resp.Diagnostics.AddAttributeError(req.Path, "Invalid Attribute Combination",
		fmt.Sprintf("Configure at least one of: `%s`.", strings.Join(v.ChildNames, "`, `")))
}

// FriendlyExactlyOneOfObjectValidator is exported (rather than kept as an
// unexported implementation type) so schema-wiring tests in other packages
// can type-assert a validator.Object and inspect PathExpressions directly,
// to catch old-style, child-attached oneof groups left behind by an
// incomplete migration to the cheaper, parent-attached
// ExactlyOneOfChildrenValidator.
func ExactlyOneOfObject(expressions ...path.Expression) validator.Object {
	return FriendlyExactlyOneOfObjectValidator{
		Object:          objectvalidator.ExactlyOneOf(expressions...),
		PathExpressions: expressions,
	}
}

func ExactlyOneOfString(expressions ...path.Expression) validator.String {
	return friendlyExactlyOneOfStringValidator{
		String:          stringvalidator.ExactlyOneOf(expressions...),
		PathExpressions: expressions,
	}
}

func ExactlyOneOfInt64(expressions ...path.Expression) validator.Int64 {
	return friendlyExactlyOneOfInt64Validator{
		Int64:           int64validator.ExactlyOneOf(expressions...),
		PathExpressions: expressions,
	}
}

type FriendlyExactlyOneOfObjectValidator struct {
	validator.Object
	PathExpressions path.Expressions
}

func (v FriendlyExactlyOneOfObjectValidator) ValidateObject(ctx context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	var delegateResp validator.ObjectResponse
	v.Object.ValidateObject(ctx, req, &delegateResp)
	rewriteExactlyOneOfDiagnostics(ctx, exactlyOneOfDiagnosticRequest{
		Config:          req.Config,
		ConfigValue:     req.ConfigValue,
		Path:            req.Path,
		PathExpression:  req.PathExpression,
		PathExpressions: v.PathExpressions,
	}, delegateResp.Diagnostics, &resp.Diagnostics)
}

type friendlyExactlyOneOfStringValidator struct {
	validator.String
	PathExpressions path.Expressions
}

func (v friendlyExactlyOneOfStringValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	var delegateResp validator.StringResponse
	v.String.ValidateString(ctx, req, &delegateResp)
	rewriteExactlyOneOfDiagnostics(ctx, exactlyOneOfDiagnosticRequest{
		Config:          req.Config,
		ConfigValue:     req.ConfigValue,
		Path:            req.Path,
		PathExpression:  req.PathExpression,
		PathExpressions: v.PathExpressions,
	}, delegateResp.Diagnostics, &resp.Diagnostics)
}

type friendlyExactlyOneOfInt64Validator struct {
	validator.Int64
	PathExpressions path.Expressions
}

func (v friendlyExactlyOneOfInt64Validator) ValidateInt64(ctx context.Context, req validator.Int64Request, resp *validator.Int64Response) {
	var delegateResp validator.Int64Response
	v.Int64.ValidateInt64(ctx, req, &delegateResp)
	rewriteExactlyOneOfDiagnostics(ctx, exactlyOneOfDiagnosticRequest{
		Config:          req.Config,
		ConfigValue:     req.ConfigValue,
		Path:            req.Path,
		PathExpression:  req.PathExpression,
		PathExpressions: v.PathExpressions,
	}, delegateResp.Diagnostics, &resp.Diagnostics)
}

func (v FriendlyExactlyOneOfObjectValidator) exactlyOneOfPathExpressions() path.Expressions {
	return v.PathExpressions
}

func (v friendlyExactlyOneOfStringValidator) exactlyOneOfPathExpressions() path.Expressions {
	return v.PathExpressions
}

func (v friendlyExactlyOneOfInt64Validator) exactlyOneOfPathExpressions() path.Expressions {
	return v.PathExpressions
}

type exactlyOneOfDiagnosticRequest struct {
	Config          tfsdk.Config
	ConfigValue     attr.Value
	Path            path.Path
	PathExpression  path.Expression
	PathExpressions path.Expressions
}

func rewriteExactlyOneOfDiagnostics(ctx context.Context, req exactlyOneOfDiagnosticRequest, delegated diag.Diagnostics, resp *diag.Diagnostics) {
	for _, diagnostic := range delegated {
		if diagnostic.Summary() != "Invalid Attribute Combination" {
			resp.Append(diagnostic)
			continue
		}

		groupPaths, ok := exactlyOneOfGroupPaths(ctx, req.Config, req.Path, req.PathExpression, req.PathExpressions)
		if !ok {
			resp.AddAttributeError(req.Path, diagnostic.Summary(), exactlyOneOfDiagnosticDetail(diagnostic.Detail(), nil))
			continue
		}

		selectedPaths, ok := selectedExactlyOneOfPaths(ctx, req, groupPaths)
		if !ok || !shouldEmitExactlyOneOfDiagnostic(ctx, req.Config, req.Path, groupPaths, selectedPaths) {
			continue
		}

		resp.AddAttributeError(req.Path.ParentPath(), diagnostic.Summary(), exactlyOneOfDiagnosticDetail(diagnostic.Detail(), selectedPaths))
	}
}

func exactlyOneOfGroupPaths(ctx context.Context, config tfsdk.Config, currentPath path.Path, currentExpression path.Expression, expressions path.Expressions) (path.Paths, bool) {
	pathsByName := map[string]path.Path{currentPath.String(): currentPath}
	parentPath := currentPath.ParentPath()

	for _, expression := range currentExpression.MergeExpressions(expressions...) {
		matchedPaths, diags := config.PathMatches(ctx, expression)
		if diags.HasError() {
			return nil, false
		}
		for _, matchedPath := range matchedPaths {
			if matchedPath.ParentPath().Equal(parentPath) {
				pathsByName[matchedPath.String()] = matchedPath
			}
		}
	}

	paths := make(path.Paths, 0, len(pathsByName))
	for _, attributePath := range pathsByName {
		paths = append(paths, attributePath)
	}
	sort.Slice(paths, func(i, j int) bool { return paths[i].String() < paths[j].String() })
	return paths, true
}

func selectedExactlyOneOfPaths(ctx context.Context, req exactlyOneOfDiagnosticRequest, groupPaths path.Paths) (path.Paths, bool) {
	selectedPaths := make(path.Paths, 0, len(groupPaths))
	for _, attributePath := range groupPaths {
		value := req.ConfigValue
		if !attributePath.Equal(req.Path) {
			var configuredValue attr.Value
			if diags := req.Config.GetAttribute(ctx, attributePath, &configuredValue); diags.HasError() {
				return nil, false
			}
			value = configuredValue
		}
		if value != nil && !value.IsNull() && !value.IsUnknown() {
			selectedPaths = append(selectedPaths, attributePath)
		}
	}
	return selectedPaths, true
}

func shouldEmitExactlyOneOfDiagnostic(ctx context.Context, config tfsdk.Config, currentPath path.Path, groupPaths, selectedPaths path.Paths) bool {
	emitterPaths := selectedPaths
	if len(emitterPaths) == 0 {
		emitterPaths = groupPaths
	}

	for _, candidatePath := range emitterPaths {
		if attributeHasMatchingExactlyOneOfValidator(ctx, config, candidatePath, groupPaths) {
			return candidatePath.Equal(currentPath)
		}
	}

	// Schema introspection is only used to de-duplicate diagnostics. If it
	// cannot identify an owner, retain the delegate's validation result.
	return true
}

func attributeHasMatchingExactlyOneOfValidator(ctx context.Context, config tfsdk.Config, attributePath path.Path, groupPaths path.Paths) bool {
	attribute, diags := config.Schema.AttributeAtPath(ctx, attributePath)
	if diags.HasError() {
		return false
	}

	var validators []exactlyOneOfValidator
	if provider, ok := attribute.(interface{ ObjectValidators() []validator.Object }); ok {
		validators = appendExactlyOneOfValidators(validators, provider.ObjectValidators())
	}
	if provider, ok := attribute.(interface{ StringValidators() []validator.String }); ok {
		validators = appendExactlyOneOfValidators(validators, provider.StringValidators())
	}
	if provider, ok := attribute.(interface{ Int64Validators() []validator.Int64 }); ok {
		validators = appendExactlyOneOfValidators(validators, provider.Int64Validators())
	}

	for _, candidateValidator := range validators {
		candidateGroupPaths, ok := exactlyOneOfGroupPaths(ctx, config, attributePath, attributePath.Expression(), candidateValidator.exactlyOneOfPathExpressions())
		if ok && equalPaths(candidateGroupPaths, groupPaths) {
			return true
		}
	}
	return false
}

type exactlyOneOfValidator interface {
	exactlyOneOfPathExpressions() path.Expressions
}

func appendExactlyOneOfValidators[T any](destination []exactlyOneOfValidator, validators []T) []exactlyOneOfValidator {
	for _, candidateValidator := range validators {
		if exactlyOneOf, ok := any(candidateValidator).(exactlyOneOfValidator); ok {
			destination = append(destination, exactlyOneOf)
		}
	}
	return destination
}

func equalPaths(left, right path.Paths) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if !left[i].Equal(right[i]) {
			return false
		}
	}
	return true
}

func exactlyOneOfDiagnosticDetail(detail string, selectedPaths path.Paths) string {
	if strings.HasPrefix(detail, "No attribute specified") {
		return "No attribute was configured in this one-of group. Configure exactly one value."
	}
	if len(selectedPaths) == 0 {
		return "Multiple attributes were configured in this one-of group. Configure only one value."
	}

	selectedNames := make([]string, 0, len(selectedPaths))
	for _, selectedPath := range selectedPaths {
		lastStep, _ := selectedPath.Steps().LastStep()
		if attributeName, ok := lastStep.(path.PathStepAttributeName); ok {
			selectedNames = append(selectedNames, "`"+attributeName.String()+"`")
		} else {
			selectedNames = append(selectedNames, "`"+selectedPath.String()+"`")
		}
	}
	return fmt.Sprintf("Only one of these attributes can be configured: %s.", strings.Join(selectedNames, ", "))
}

func FlattenSpansAggregation(aggregation *dashboardservice.SpansAggregation) (*SpansAggregationModel, diag.Diagnostic) {
	if aggregation == nil {
		return nil, nil
	}
	switch {
	case aggregation.MetricAggregation != nil:
		return &SpansAggregationModel{
			Type:            types.StringValue("metric"),
			AggregationType: types.StringValue(DashboardProtoToSchemaSpansAggregationMetricAggregationType[aggregation.MetricAggregation.GetAggregationType()]),
			Field:           types.StringValue(DashboardProtoToSchemaSpansAggregationMetricField[aggregation.MetricAggregation.GetMetricField()]),
		}, nil
	case aggregation.DimensionAggregation != nil:
		return &SpansAggregationModel{
			Type:            types.StringValue("dimension"),
			AggregationType: types.StringValue(DashboardProtoToSchemaSpansAggregationDimensionAggregationType[aggregation.DimensionAggregation.GetAggregationType()]),
			Field:           types.StringValue(DashboardSchemaToProtoSpansAggregationDimensionField[aggregation.DimensionAggregation.GetDimensionField()]),
		}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Error Flatten Span Aggregation", fmt.Sprintf("unknown aggregation type %T", aggregation))
	}
}

func ExpandResolution(ctx context.Context, resolution types.Object) (*dashboardservice.LineChartResolution, diag.Diagnostics) {
	if resolution.IsNull() || resolution.IsUnknown() {
		return nil, nil
	}

	var resolutionModel LineChartResolutionModel
	if diags := resolution.As(ctx, &resolutionModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if !(resolutionModel.Interval.IsNull() || resolutionModel.Interval.IsUnknown()) {
		interval, dg := legacyDurationToOpenAPI(resolutionModel.Interval.ValueString(), "resolution.interval")
		if dg != nil {
			return nil, diag.Diagnostics{dg}
		}

		return &dashboardservice.LineChartResolution{
			Interval: interval,
		}, nil
	}

	return &dashboardservice.LineChartResolution{
		BucketsPresented: int64ToInt32Pointer(resolutionModel.BucketsPresented),
	}, nil
}

func ExpandDashboardUUID(id types.String) *dashboardservice.UUID {
	if id.IsNull() || id.IsUnknown() {
		value := uuid.NewString()
		return &dashboardservice.UUID{Value: &value}
	}
	return &dashboardservice.UUID{Value: id.ValueStringPointer()}
}

func ExpandDashboardIDs(id types.String) *string {
	if id.IsNull() || id.IsUnknown() {
		value := uuid.NewString()
		return &value
	}
	return id.ValueStringPointer()
}

func ExpandDashboardFiltersSources(ctx context.Context, filters types.List) ([]dashboardservice.FilterSource, diag.Diagnostics) {
	var filtersObjects []types.Object
	var expandedFiltersSources []dashboardservice.FilterSource
	diags := filters.ElementsAs(ctx, &filtersObjects, true)
	if diags.HasError() {
		return nil, diags
	}

	for _, fo := range filtersObjects {
		var filterSource DashboardFilterSourceModel
		if dg := fo.As(ctx, &filterSource, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedFilter, expandDiags := ExpandFilterSource(ctx, &filterSource)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedFiltersSources = append(expandedFiltersSources, *expandedFilter)
	}

	return expandedFiltersSources, diags
}

type DynamicStatCardModel struct {
	AllowAbbreviation types.Bool                     `tfsdk:"allow_abbreviation"`
	CategoryFields    types.List                     `tfsdk:"category_fields"` //ObservationFieldModel
	ColorLabelMapping *DynamicColorLabelMappingModel `tfsdk:"color_label_mapping"`
	CustomUnit        types.String                   `tfsdk:"custom_unit"`
	DecimalPrecision  types.Int64                    `tfsdk:"decimal_precision"`
	Label             *DynamicStatVisualElementModel `tfsdk:"label"`
	Legend            *LegendModel                   `tfsdk:"legend"`
	LegendBy          types.String                   `tfsdk:"legend_by"`
	PrimaryValue      *DynamicStatVisualElementModel `tfsdk:"primary_value"`
	Title             *DynamicStatVisualElementModel `tfsdk:"title"`
	Unit              types.String                   `tfsdk:"unit"`
	ValueFields       types.List                     `tfsdk:"value_fields"` //ObservationFieldModel
}

type DynamicStatVisualElementModel struct {
	MappedValues      types.Bool   `tfsdk:"mapped_values"`
	ObservationField  types.Object `tfsdk:"observation_field"` //ObservationFieldModel
	TemplateText      types.String `tfsdk:"template_text"`
	TemplateVariables types.List   `tfsdk:"template_variables"` //DynamicTemplateVariableModel
}

type DynamicTemplateVariableModel struct {
	MappedValues     types.Bool   `tfsdk:"mapped_values"`
	ObservationField types.Object `tfsdk:"observation_field"` //ObservationFieldModel
}

type DynamicColorLabelMappingModel struct {
	ColorBy types.String                 `tfsdk:"color_by"`
	Range   *DynamicRangeMappingModel    `tfsdk:"range"`
	Regex   *DynamicSectionsMappingModel `tfsdk:"regex"`
	Value   *DynamicSectionsMappingModel `tfsdk:"value"`
}

type DynamicSectionsMappingModel struct {
	Sections types.List `tfsdk:"sections"` //DynamicMappingSectionModel
}

type DynamicRangeMappingModel struct {
	MinMax        *DynamicMinMaxModel `tfsdk:"min_max"`
	ThresholdType types.String        `tfsdk:"threshold_type"`
	Thresholds    types.List          `tfsdk:"thresholds"` //DynamicThresholdModel
}

type DynamicMinMaxModel struct {
	Auto   types.Bool                `tfsdk:"auto"`
	Custom *DynamicMinMaxCustomModel `tfsdk:"custom"`
}

type DynamicMappingSectionModel struct {
	Color types.String `tfsdk:"color"`
	MapTo types.String `tfsdk:"map_to"`
	Value types.String `tfsdk:"value"`
}

type DynamicMinMaxCustomModel struct {
	Max types.Float64 `tfsdk:"max"`
	Min types.Float64 `tfsdk:"min"`
}

type DynamicGaugeModel struct {
	AllowAbbreviation types.Bool              `tfsdk:"allow_abbreviation"`
	ArcDisplay        *DynamicArcDisplayModel `tfsdk:"arc_display"`
	CategoryFields    types.List              `tfsdk:"category_fields"` //ObservationFieldModel
	CustomUnit        types.String            `tfsdk:"custom_unit"`
	DecimalPrecision  types.Int64             `tfsdk:"decimal_precision"`
	DisplaySeriesName types.Bool              `tfsdk:"display_series_name"`
	Legend            *LegendModel            `tfsdk:"legend"`
	LegendBy          types.String            `tfsdk:"legend_by"`
	Max               types.Float64           `tfsdk:"max"`
	Min               types.Float64           `tfsdk:"min"`
	ShowInnerArc      types.Bool              `tfsdk:"show_inner_arc"`
	ShowMinMax        types.Bool              `tfsdk:"show_min_max"`
	ShowOuterArc      types.Bool              `tfsdk:"show_outer_arc"`
	ThresholdType     types.String            `tfsdk:"threshold_type"`
	Thresholds        types.List              `tfsdk:"thresholds"` //DynamicThresholdModel
	Unit              types.String            `tfsdk:"unit"`
	ValueField        types.Object            `tfsdk:"value_field"`  //ObservationFieldModel
	ValueFields       types.List              `tfsdk:"value_fields"` //ObservationFieldModel
}

type DynamicArcDisplayModel struct {
	ThresholdArc types.Bool `tfsdk:"threshold_arc"`
	ValueArc     types.Bool `tfsdk:"value_arc"`
}

type DynamicPieChartModel struct {
	AllowAbbreviation  types.Bool                           `tfsdk:"allow_abbreviation"`
	CategoryFields     types.List                           `tfsdk:"category_fields"` //ObservationFieldModel
	ColorScheme        types.String                         `tfsdk:"color_scheme"`
	CustomUnit         types.String                         `tfsdk:"custom_unit"`
	DecimalPrecision   types.Int64                          `tfsdk:"decimal_precision"`
	GroupNameTemplate  types.String                         `tfsdk:"group_name_template"`
	HashColors         types.Bool                           `tfsdk:"hash_colors"`
	LabelDefinition    *DynamicPieChartLabelDefinitionModel `tfsdk:"label_definition"`
	Legend             *LegendModel                         `tfsdk:"legend"`
	MaxSlicesPerChart  types.Int64                          `tfsdk:"max_slices_per_chart"`
	MaxSlicesPerStack  types.Int64                          `tfsdk:"max_slices_per_stack"`
	MinSlicePercentage types.Int64                          `tfsdk:"min_slice_percentage"`
	ShowTotal          types.Bool                           `tfsdk:"show_total"`
	StackNameTemplate  types.String                         `tfsdk:"stack_name_template"`
	SubCategoryFields  types.List                           `tfsdk:"sub_category_fields"` //ObservationFieldModel
	Unit               types.String                         `tfsdk:"unit"`
	ValueField         types.Object                         `tfsdk:"value_field"` //ObservationFieldModel
}

type DynamicPieChartLabelDefinitionModel struct {
	IsVisible      types.Bool   `tfsdk:"is_visible"`
	LabelSource    types.String `tfsdk:"label_source"`
	ShowName       types.Bool   `tfsdk:"show_name"`
	ShowPercentage types.Bool   `tfsdk:"show_percentage"`
	ShowValue      types.Bool   `tfsdk:"show_value"`
}

type DynamicTimeSeriesLinesModel struct {
	AllowAbbreviation  types.Bool                     `tfsdk:"allow_abbreviation"`
	CategoryFields     types.List                     `tfsdk:"category_fields"` //ObservationFieldModel
	ColorScheme        types.String                   `tfsdk:"color_scheme"`
	ConnectNulls       types.Bool                     `tfsdk:"connect_nulls"`
	CustomUnit         types.String                   `tfsdk:"custom_unit"`
	DecimalPrecision   types.Int64                    `tfsdk:"decimal_precision"`
	HashColors         types.Bool                     `tfsdk:"hash_colors"`
	Legend             *LegendModel                   `tfsdk:"legend"`
	ScaleType          types.String                   `tfsdk:"scale_type"`
	SeriesCountLimit   types.Int64                    `tfsdk:"series_count_limit"`
	SeriesNameTemplate types.String                   `tfsdk:"series_name_template"`
	StackedLine        types.String                   `tfsdk:"stacked_line"`
	TemporalField      types.Object                   `tfsdk:"temporal_field"` //ObservationFieldModel
	Tooltip            *DynamicTimeSeriesTooltipModel `tfsdk:"tooltip"`
	Unit               types.String                   `tfsdk:"unit"`
	UseDataTimeRange   types.Bool                     `tfsdk:"use_data_time_range"`
	ValueFields        types.List                     `tfsdk:"value_fields"` //ObservationFieldModel
	XAxisTimeFormat    types.String                   `tfsdk:"x_axis_time_format"`
	YAxisMax           Float32Value                   `tfsdk:"y_axis_max"`
	YAxisMin           Float32Value                   `tfsdk:"y_axis_min"`
}

type DynamicTimeSeriesLinesMultiModel struct {
	ConnectNulls         types.Bool                     `tfsdk:"connect_nulls"`
	Legend               *LegendModel                   `tfsdk:"legend"`
	QueryDisplaySettings types.List                     `tfsdk:"query_display_settings"` //DynamicQueryDisplaySettingsModel
	StackedLine          types.String                   `tfsdk:"stacked_line"`
	Tooltip              *DynamicTimeSeriesTooltipModel `tfsdk:"tooltip"`
	UseDataTimeRange     types.Bool                     `tfsdk:"use_data_time_range"`
	XAxisTimeFormat      types.String                   `tfsdk:"x_axis_time_format"`
}

type DynamicTimeSeriesBarsModel struct {
	AllowAbbreviation  types.Bool                     `tfsdk:"allow_abbreviation"`
	BarValueDisplay    types.String                   `tfsdk:"bar_value_display"`
	CategoryFields     types.List                     `tfsdk:"category_fields"` //ObservationFieldModel
	ColorScheme        types.String                   `tfsdk:"color_scheme"`
	CustomUnit         types.String                   `tfsdk:"custom_unit"`
	DecimalPrecision   types.Int64                    `tfsdk:"decimal_precision"`
	HashColors         types.Bool                     `tfsdk:"hash_colors"`
	Legend             *LegendModel                   `tfsdk:"legend"`
	MaxSlicesPerBar    types.Int64                    `tfsdk:"max_slices_per_bar"`
	ScaleType          types.String                   `tfsdk:"scale_type"`
	SeriesNameTemplate types.String                   `tfsdk:"series_name_template"`
	SortBy             types.String                   `tfsdk:"sort_by"`
	TemporalField      types.Object                   `tfsdk:"temporal_field"` //ObservationFieldModel
	Tooltip            *DynamicTimeSeriesTooltipModel `tfsdk:"tooltip"`
	Unit               types.String                   `tfsdk:"unit"`
	ValueFields        types.List                     `tfsdk:"value_fields"` //ObservationFieldModel
	XAxisTimeFormat    types.String                   `tfsdk:"x_axis_time_format"`
	YAxisMax           Float32Value                   `tfsdk:"y_axis_max"`
	YAxisMin           Float32Value                   `tfsdk:"y_axis_min"`
}

type DynamicTimeSeriesTooltipModel struct {
	ShowAllSeries types.Bool `tfsdk:"show_all_series"`
	ShowLabels    types.Bool `tfsdk:"show_labels"`
}

type DynamicQueryDisplaySettingsModel struct {
	AllowAbbreviation  types.Bool   `tfsdk:"allow_abbreviation"`
	CategoryFields     types.List   `tfsdk:"category_fields"` //ObservationFieldModel
	ColorScheme        types.String `tfsdk:"color_scheme"`
	CustomUnit         types.String `tfsdk:"custom_unit"`
	DecimalPrecision   types.Int64  `tfsdk:"decimal_precision"`
	HashColors         types.Bool   `tfsdk:"hash_colors"`
	QueryID            types.String `tfsdk:"query_id"`
	ScaleType          types.String `tfsdk:"scale_type"`
	SeriesCountLimit   types.Int64  `tfsdk:"series_count_limit"`
	SeriesNameTemplate types.String `tfsdk:"series_name_template"`
	TemporalField      types.Object `tfsdk:"temporal_field"` //ObservationFieldModel
	Unit               types.String `tfsdk:"unit"`
	ValueFields        types.List   `tfsdk:"value_fields"` //ObservationFieldModel
	YAxisMax           Float32Value `tfsdk:"y_axis_max"`
	YAxisMin           Float32Value `tfsdk:"y_axis_min"`
}

type DynamicSortOrderModel struct {
	OrderDirection types.String              `tfsdk:"order_direction"`
	Strategy       *DynamicSortStrategyModel `tfsdk:"strategy"`
}

type DynamicSortStrategyModel struct {
	Category     types.Bool                    `tfsdk:"category"`
	QueryValue   *DynamicSortByQueryValueModel `tfsdk:"query_value"`
	StrategyType types.String                  `tfsdk:"strategy_type"`
}

type DynamicSortByQueryValueModel struct {
	QueryID types.String `tfsdk:"query_id"`
}

type DynamicBarsQueryFieldSettingsModel struct {
	QueryID    types.String `tfsdk:"query_id"`
	ValueField types.Object `tfsdk:"value_field"` //ObservationFieldModel
}

type DynamicVerticalBarsModel struct {
	AllowAbbreviation types.Bool   `tfsdk:"allow_abbreviation"`
	BarValueDisplay   types.String `tfsdk:"bar_value_display"`
	CategoryFields    types.List   `tfsdk:"category_fields"` //ObservationFieldModel
	ColorScheme       types.String `tfsdk:"color_scheme"`
	ColorsBy          types.String `tfsdk:"colors_by"`
	CustomUnit        types.String `tfsdk:"custom_unit"`
	DecimalPrecision  types.Int64  `tfsdk:"decimal_precision"`
	GroupNameTemplate types.String `tfsdk:"group_name_template"`
	HashColors        types.Bool   `tfsdk:"hash_colors"`
	Legend            *LegendModel `tfsdk:"legend"`
	MaxBarsPerChart   types.Int64  `tfsdk:"max_bars_per_chart"`
	MaxSlicesPerBar   types.Int64  `tfsdk:"max_slices_per_bar"`
	ScaleType         types.String `tfsdk:"scale_type"`
	SortBy            types.String `tfsdk:"sort_by"`
	StackNameTemplate types.String `tfsdk:"stack_name_template"`
	SubCategoryFields types.List   `tfsdk:"sub_category_fields"` //ObservationFieldModel
	Unit              types.String `tfsdk:"unit"`
	ValueField        types.Object `tfsdk:"value_field"` //ObservationFieldModel
	YAxisMax          Float32Value `tfsdk:"y_axis_max"`
	YAxisMin          Float32Value `tfsdk:"y_axis_min"`
}

type DynamicVerticalBarsMultiModel struct {
	AllowAbbreviation  types.Bool             `tfsdk:"allow_abbreviation"`
	BarValueDisplay    types.String           `tfsdk:"bar_value_display"`
	CategoryFields     types.List             `tfsdk:"category_fields"` //ObservationFieldModel
	ColorScheme        types.String           `tfsdk:"color_scheme"`
	ColorsBy           types.String           `tfsdk:"colors_by"`
	CustomUnit         types.String           `tfsdk:"custom_unit"`
	DecimalPrecision   types.Int64            `tfsdk:"decimal_precision"`
	GroupNameTemplate  types.String           `tfsdk:"group_name_template"`
	HashColors         types.Bool             `tfsdk:"hash_colors"`
	Legend             *LegendModel           `tfsdk:"legend"`
	MaxBarsPerChart    types.Int64            `tfsdk:"max_bars_per_chart"`
	QueryFieldSettings types.List             `tfsdk:"query_field_settings"` //DynamicBarsQueryFieldSettingsModel
	ScaleType          types.String           `tfsdk:"scale_type"`
	SortOrder          *DynamicSortOrderModel `tfsdk:"sort_order"`
	Unit               types.String           `tfsdk:"unit"`
	YAxisMax           Float32Value           `tfsdk:"y_axis_max"`
	YAxisMin           Float32Value           `tfsdk:"y_axis_min"`
}

type DynamicHorizontalBarsModel struct {
	AllowAbbreviation types.Bool   `tfsdk:"allow_abbreviation"`
	CategoryFields    types.List   `tfsdk:"category_fields"` //ObservationFieldModel
	ColorScheme       types.String `tfsdk:"color_scheme"`
	ColorsBy          types.String `tfsdk:"colors_by"`
	CustomUnit        types.String `tfsdk:"custom_unit"`
	DecimalPrecision  types.Int64  `tfsdk:"decimal_precision"`
	DisplayOnBar      types.Bool   `tfsdk:"display_on_bar"`
	GroupNameTemplate types.String `tfsdk:"group_name_template"`
	HashColors        types.Bool   `tfsdk:"hash_colors"`
	Legend            *LegendModel `tfsdk:"legend"`
	MaxBarsPerChart   types.Int64  `tfsdk:"max_bars_per_chart"`
	MaxSlicesPerBar   types.Int64  `tfsdk:"max_slices_per_bar"`
	ScaleType         types.String `tfsdk:"scale_type"`
	SortBy            types.String `tfsdk:"sort_by"`
	StackNameTemplate types.String `tfsdk:"stack_name_template"`
	SubCategoryFields types.List   `tfsdk:"sub_category_fields"` //ObservationFieldModel
	Unit              types.String `tfsdk:"unit"`
	ValueField        types.Object `tfsdk:"value_field"` //ObservationFieldModel
	YAxisMax          Float32Value `tfsdk:"y_axis_max"`
	YAxisMin          Float32Value `tfsdk:"y_axis_min"`
	YAxisViewBy       types.String `tfsdk:"y_axis_view_by"`
}

type DynamicHorizontalBarsMultiModel struct {
	AllowAbbreviation  types.Bool             `tfsdk:"allow_abbreviation"`
	CategoryFields     types.List             `tfsdk:"category_fields"` //ObservationFieldModel
	ColorScheme        types.String           `tfsdk:"color_scheme"`
	ColorsBy           types.String           `tfsdk:"colors_by"`
	CustomUnit         types.String           `tfsdk:"custom_unit"`
	DecimalPrecision   types.Int64            `tfsdk:"decimal_precision"`
	DisplayOnBar       types.Bool             `tfsdk:"display_on_bar"`
	GroupNameTemplate  types.String           `tfsdk:"group_name_template"`
	HashColors         types.Bool             `tfsdk:"hash_colors"`
	Legend             *LegendModel           `tfsdk:"legend"`
	MaxBarsPerChart    types.Int64            `tfsdk:"max_bars_per_chart"`
	QueryFieldSettings types.List             `tfsdk:"query_field_settings"` //DynamicBarsQueryFieldSettingsModel
	ScaleType          types.String           `tfsdk:"scale_type"`
	SortOrder          *DynamicSortOrderModel `tfsdk:"sort_order"`
	Unit               types.String           `tfsdk:"unit"`
	YAxisMax           Float32Value           `tfsdk:"y_axis_max"`
	YAxisMin           Float32Value           `tfsdk:"y_axis_min"`
	YAxisViewBy        types.String           `tfsdk:"y_axis_view_by"`
}

func ExpandColorsBy(colorsBy types.String) *dashboardservice.ColorsBy {
	switch colorsBy.ValueString() {
	case "stack":
		return &dashboardservice.ColorsBy{
			Stack: map[string]interface{}{},
		}
	case "group_by":
		return &dashboardservice.ColorsBy{
			GroupBy: map[string]interface{}{},
		}
	case "aggregation":
		return &dashboardservice.ColorsBy{
			Aggregation: map[string]interface{}{},
		}
	case "query":
		return &dashboardservice.ColorsBy{
			Query: map[string]interface{}{},
		}
	case "category":
		return &dashboardservice.ColorsBy{
			Category: map[string]interface{}{},
		}
	default:
		return nil
	}
}

func FlattenColorsBy(colorsBy *dashboardservice.ColorsBy) (types.String, diag.Diagnostic) {
	if colorsBy == nil {
		return types.StringNull(), nil
	}
	switch {
	case colorsBy.GroupBy != nil:
		return types.StringValue("group_by"), nil
	case colorsBy.Stack != nil:
		return types.StringValue("stack"), nil
	case colorsBy.Aggregation != nil:
		return types.StringValue("aggregation"), nil
	case colorsBy.Query != nil:
		return types.StringValue("query"), nil
	case colorsBy.Category != nil:
		return types.StringValue("category"), nil
	default:
		return types.StringNull(), diag.NewErrorDiagnostic("", fmt.Sprintf("unknown colors by type %T", colorsBy))
	}
}

type DynamicTableModel struct {
	Columns  types.List                 `tfsdk:"columns"` //DynamicTableColumnModel
	Rules    types.List                 `tfsdk:"rules"`   //DynamicTableRuleModel
	Settings *DynamicTableSettingsModel `tfsdk:"settings"`
}

type DynamicTableColumnModel struct {
	Field types.Object `tfsdk:"field"` //ObservationFieldModel
}

type DynamicTableRuleModel struct {
	Description types.String                `tfsdk:"description"`
	ID          types.String                `tfsdk:"id"`
	Name        types.String                `tfsdk:"name"`
	Properties  types.List                  `tfsdk:"properties"` //DynamicTablePropertyModel
	RuleScope   *DynamicTableRuleScopeModel `tfsdk:"rule_scope"`
}

type DynamicTablePropertyModel struct {
	ID         types.String                         `tfsdk:"id"`
	Definition *DynamicTablePropertyDefinitionModel `tfsdk:"definition"`
}

type DynamicTablePropertyDefinitionModel struct {
	Alignment         types.String                            `tfsdk:"alignment"`
	ColumnDisplayName types.String                            `tfsdk:"column_display_name"`
	Link              *DynamicTablePropertyLinkModel          `tfsdk:"link"`
	RegexExtract      types.String                            `tfsdk:"regex_extract"`
	Thresholds        *DynamicTablePropertyThresholdsModel    `tfsdk:"thresholds"`
	Units             *DynamicTablePropertyUnitsModel         `tfsdk:"units"`
	ValuesAlias       types.String                            `tfsdk:"values_alias"`
	ValuesMapping     *DynamicTablePropertyValuesMappingModel `tfsdk:"values_mapping"`
}

type DynamicTablePropertyLinkModel struct {
	Actions types.List `tfsdk:"actions"` //DynamicTableLinkActionModel
}

type DynamicTableLinkActionModel struct {
	ID                    types.String `tfsdk:"id"`
	Name                  types.String `tfsdk:"name"`
	ShouldOpenInNewWindow types.Bool   `tfsdk:"should_open_in_new_window"`
	Url                   types.String `tfsdk:"url"`
}

type DynamicTablePropertyThresholdsModel struct {
	Max    types.Float64 `tfsdk:"max"`
	Min    types.Float64 `tfsdk:"min"`
	Type   types.String  `tfsdk:"type"`
	Values types.List    `tfsdk:"values"` //DynamicThresholdModel
}

type DynamicTablePropertyUnitsModel struct {
	AllowAbbreviation types.Bool    `tfsdk:"allow_abbreviation"`
	CustomUnit        types.String  `tfsdk:"custom_unit"`
	DecimalPrecision  types.Int64   `tfsdk:"decimal_precision"`
	Max               types.Float64 `tfsdk:"max"`
	Min               types.Float64 `tfsdk:"min"`
	Unit              types.String  `tfsdk:"unit"`
}

type DynamicTablePropertyValuesMappingModel struct {
	Mappings types.List `tfsdk:"mappings"` //DynamicTableValueMappingModel
}

type DynamicTableValueMappingModel struct {
	InputValue   types.String `tfsdk:"input_value"`
	ReplaceValue types.String `tfsdk:"replace_value"`
	Type         types.String `tfsdk:"type"`
}

type DynamicTableRuleScopeModel struct {
	Field     types.Object `tfsdk:"field"` //ObservationFieldModel
	FieldType types.String `tfsdk:"field_type"`
	Regex     types.String `tfsdk:"regex"`
}

type DynamicTableSettingsModel struct {
	ColumnWidths types.List   `tfsdk:"column_widths"` //DynamicTableColumnWidthModel
	RowStyle     types.String `tfsdk:"row_style"`
}

type DynamicTableColumnWidthModel struct {
	ColumnName types.String `tfsdk:"column_name"`
	Width      types.Int64  `tfsdk:"width"`
}

type DynamicHexagonBinsModel struct {
	AllowAbbreviation types.Bool    `tfsdk:"allow_abbreviation"`
	CategoryFields    types.List    `tfsdk:"category_fields"` //ObservationFieldModel
	CustomUnit        types.String  `tfsdk:"custom_unit"`
	DecimalPrecision  types.Int64   `tfsdk:"decimal_precision"`
	Legend            *LegendModel  `tfsdk:"legend"`
	LegendBy          types.String  `tfsdk:"legend_by"`
	Max               types.Float64 `tfsdk:"max"`
	Min               types.Float64 `tfsdk:"min"`
	ThresholdType     types.String  `tfsdk:"threshold_type"`
	Thresholds        types.List    `tfsdk:"thresholds"` //DynamicThresholdModel
	Unit              types.String  `tfsdk:"unit"`
	ValueField        types.Object  `tfsdk:"value_field"` //ObservationFieldModel
}

type DynamicHeatmapModel struct {
	AllowAbbreviation   types.Bool                  `tfsdk:"allow_abbreviation"`
	ColorAxisMax        Float32Value                `tfsdk:"color_axis_max"`
	ColorAxisMin        Float32Value                `tfsdk:"color_axis_min"`
	ColorRange          types.String                `tfsdk:"color_range"`
	CustomUnit          types.String                `tfsdk:"custom_unit"`
	DecimalPrecision    types.Int64                 `tfsdk:"decimal_precision"`
	HistogramBucketUnit types.String                `tfsdk:"histogram_bucket_unit"`
	Preset              types.String                `tfsdk:"preset"`
	ScaleType           types.String                `tfsdk:"scale_type"`
	ShowNumbers         types.Bool                  `tfsdk:"show_numbers"`
	Tooltip             *DynamicHeatmapTooltipModel `tfsdk:"tooltip"`
	Unit                types.String                `tfsdk:"unit"`
	ValueField          types.Object                `tfsdk:"value_field"`   //ObservationFieldModel
	XAxisFields         types.List                  `tfsdk:"x_axis_fields"` //ObservationFieldModel
	XAxisTimeFormat     types.String                `tfsdk:"x_axis_time_format"`
	YAxisFields         types.List                  `tfsdk:"y_axis_fields"` //ObservationFieldModel
}

type DynamicHeatmapTooltipModel struct {
	Labels          types.List   `tfsdk:"labels"` //ObservationFieldModel
	MessageTemplate types.String `tfsdk:"message_template"`
}

type DynamicGeomapModel struct {
	Aggregation       *DynamicGeomapAggregationModel `tfsdk:"aggregation"`
	AllowAbbreviation types.Bool                     `tfsdk:"allow_abbreviation"`
	Color             *DynamicGeomapColorModel       `tfsdk:"color"`
	Config            *DynamicGeomapFieldConfigModel `tfsdk:"config"`
	CustomUnit        types.String                   `tfsdk:"custom_unit"`
	DecimalPrecision  types.Int64                    `tfsdk:"decimal_precision"`
	MinMax            *DynamicMinMaxModel            `tfsdk:"min_max"`
	Tooltip           *DynamicGeomapTooltipModel     `tfsdk:"tooltip"`
	Unit              types.String                   `tfsdk:"unit"`
}

type DynamicGeomapFieldConfigModel struct {
	AwsRegionConfig  *DynamicGeomapAwsRegionConfigModel  `tfsdk:"aws_region_config"`
	CoordinateConfig *DynamicGeomapCoordinateConfigModel `tfsdk:"coordinate_config"`
}

type DynamicGeomapCoordinateConfigModel struct {
	LatitudeField  types.Object `tfsdk:"latitude_field"`  //ObservationFieldModel
	LongitudeField types.Object `tfsdk:"longitude_field"` //ObservationFieldModel
}

type DynamicGeomapAwsRegionConfigModel struct {
	AwsRegionField types.Object `tfsdk:"aws_region_field"` //ObservationFieldModel
}

type DynamicGeomapAggregationModel struct {
	Avg   *DynamicGeomapAggregationFieldBasedModel `tfsdk:"avg"`
	Count types.Bool                               `tfsdk:"count"`
	Max   *DynamicGeomapAggregationFieldBasedModel `tfsdk:"max"`
	Min   *DynamicGeomapAggregationFieldBasedModel `tfsdk:"min"`
	Sum   *DynamicGeomapAggregationFieldBasedModel `tfsdk:"sum"`
}

type DynamicGeomapAggregationFieldBasedModel struct {
	Field types.Object `tfsdk:"field"` //ObservationFieldModel
}

type DynamicGeomapColorModel struct {
	ColorRange types.String `tfsdk:"color_range"`
	Size       types.String `tfsdk:"size"`
}

type DynamicGeomapTooltipModel struct {
	Labels          types.List   `tfsdk:"labels"` //ObservationFieldModel
	MessageTemplate types.String `tfsdk:"message_template"`
}
