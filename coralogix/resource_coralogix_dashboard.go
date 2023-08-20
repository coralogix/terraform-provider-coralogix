package coralogix

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"golang.org/x/exp/slices"

	"github.com/golang/protobuf/jsonpb"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"terraform-provider-coralogix/coralogix/clientset"
	dashboards "terraform-provider-coralogix/coralogix/clientset/grpc/coralogix-dashboards/v1"
)

var (
	dashboardRowStyleSchemaToProto = map[string]dashboards.RowStyle{
		//"unspecified": dashboards.RowStyle_ROW_STYLE_UNSPECIFIED,
		"one_line":  dashboards.RowStyle_ROW_STYLE_ONE_LINE,
		"two_line":  dashboards.RowStyle_ROW_STYLE_TWO_LINE,
		"condensed": dashboards.RowStyle_ROW_STYLE_CONDENSED,
		"json":      dashboards.RowStyle_ROW_STYLE_JSON,
	}
	dashboardRowStyleProtoToSchema     = ReverseMap(dashboardRowStyleSchemaToProto)
	dashboardValidRowStyles            = GetKeys(dashboardRowStyleSchemaToProto)
	dashboardLegendColumnSchemaToProto = map[string]dashboards.Legend_LegendColumn{
		"unspecified": dashboards.Legend_LEGEND_COLUMN_UNSPECIFIED,
		"min":         dashboards.Legend_LEGEND_COLUMN_MIN,
		"max":         dashboards.Legend_LEGEND_COLUMN_MAX,
		"sum":         dashboards.Legend_LEGEND_COLUMN_SUM,
		"avg":         dashboards.Legend_LEGEND_COLUMN_AVG,
		"last":        dashboards.Legend_LEGEND_COLUMN_LAST,
	}
	dashboardLegendColumnProtoToSchema   = ReverseMap(dashboardLegendColumnSchemaToProto)
	dashboardValidLegendColumns          = GetKeys(dashboardLegendColumnSchemaToProto)
	dashboardOrderDirectionSchemaToProto = map[string]dashboards.OrderDirection{
		"unspecified": dashboards.OrderDirection_ORDER_DIRECTION_UNSPECIFIED,
		"asc":         dashboards.OrderDirection_ORDER_DIRECTION_ASC,
		"desc":        dashboards.OrderDirection_ORDER_DIRECTION_DESC,
	}
	dashboardOrderDirectionProtoToSchema = ReverseMap(dashboardOrderDirectionSchemaToProto)
	dashboardValidOrderDirections        = GetKeys(dashboardOrderDirectionSchemaToProto)
	dashboardSchemaToProtoTooltipType    = map[string]dashboards.LineChart_TooltipType{
		"unspecified": dashboards.LineChart_TOOLTIP_TYPE_UNSPECIFIED,
		"all":         dashboards.LineChart_TOOLTIP_TYPE_ALL,
		"single":      dashboards.LineChart_TOOLTIP_TYPE_SINGLE,
	}
	dashboardProtoToSchemaTooltipType = ReverseMap(dashboardSchemaToProtoTooltipType)
	dashboardValidTooltipTypes        = GetKeys(dashboardSchemaToProtoTooltipType)
	dashboardSchemaToProtoScaleType   = map[string]dashboards.ScaleType{
		"unspecified": dashboards.ScaleType_SCALE_TYPE_UNSPECIFIED,
		"linear":      dashboards.ScaleType_SCALE_TYPE_LINEAR,
		"logarithmic": dashboards.ScaleType_SCALE_TYPE_LOGARITHMIC,
	}
	dashboardProtoToSchemaScaleType = ReverseMap(dashboardSchemaToProtoScaleType)
	dashboardValidScaleTypes        = GetKeys(dashboardSchemaToProtoScaleType)
	dashboardSchemaToProtoUnit      = map[string]dashboards.Unit{
		"unspecified":  dashboards.Unit_UNIT_UNSPECIFIED,
		"microseconds": dashboards.Unit_UNIT_MICROSECONDS,
		"milliseconds": dashboards.Unit_UNIT_MILLISECONDS,
		"seconds":      dashboards.Unit_UNIT_SECONDS,
		"bytes":        dashboards.Unit_UNIT_BYTES,
		"kbytes":       dashboards.Unit_UNIT_KBYTES,
		"mbytes":       dashboards.Unit_UNIT_MBYTES,
		"gbytes":       dashboards.Unit_UNIT_GBYTES,
		"bytes_iec":    dashboards.Unit_UNIT_BYTES_IEC,
		"kibytes":      dashboards.Unit_UNIT_KIBYTES,
		"mibytes":      dashboards.Unit_UNIT_MIBYTES,
		"gibytes":      dashboards.Unit_UNIT_GIBYTES,
	}
	dashboardProtoToSchemaUnit      = ReverseMap(dashboardSchemaToProtoUnit)
	dashboardValidUnits             = GetKeys(dashboardSchemaToProtoUnit)
	dashboardSchemaToProtoGaugeUnit = map[string]dashboards.Gauge_Unit{
		"unspecified":  dashboards.Gauge_UNIT_UNSPECIFIED,
		"microseconds": dashboards.Gauge_UNIT_MICROSECONDS,
		"milliseconds": dashboards.Gauge_UNIT_MILLISECONDS,
		"seconds":      dashboards.Gauge_UNIT_SECONDS,
		"bytes":        dashboards.Gauge_UNIT_BYTES,
		"kbytes":       dashboards.Gauge_UNIT_KBYTES,
		"mbytes":       dashboards.Gauge_UNIT_MBYTES,
		"gbytes":       dashboards.Gauge_UNIT_GBYTES,
		"bytes_iec":    dashboards.Gauge_UNIT_BYTES_IEC,
		"kibytes":      dashboards.Gauge_UNIT_KIBYTES,
		"mibytes":      dashboards.Gauge_UNIT_MIBYTES,
		"gibytes":      dashboards.Gauge_UNIT_GIBYTES,
	}
	dashboardProtoToSchemaGaugeUnit           = ReverseMap(dashboardSchemaToProtoGaugeUnit)
	dashboardValidGaugeUnits                  = GetKeys(dashboardSchemaToProtoGaugeUnit)
	dashboardSchemaToProtoPieChartLabelSource = map[string]dashboards.PieChart_LabelSource{
		"unspecified": dashboards.PieChart_LABEL_SOURCE_UNSPECIFIED,
		"inner":       dashboards.PieChart_LABEL_SOURCE_INNER,
		"stack":       dashboards.PieChart_LABEL_SOURCE_STACK,
	}
	dashboardProtoToSchemaPieChartLabelSource = ReverseMap(dashboardSchemaToProtoPieChartLabelSource)
	dashboardValidPieChartLabelSources        = GetKeys(dashboardSchemaToProtoPieChartLabelSource)
	dashboardSchemaToProtoGaugeAggregation    = map[string]dashboards.Gauge_Aggregation{
		"unspecified": dashboards.Gauge_AGGREGATION_UNSPECIFIED,
		"last":        dashboards.Gauge_AGGREGATION_LAST,
		"min":         dashboards.Gauge_AGGREGATION_MIN,
		"max":         dashboards.Gauge_AGGREGATION_MAX,
		"avg":         dashboards.Gauge_AGGREGATION_AVG,
		"sum":         dashboards.Gauge_AGGREGATION_SUM,
	}
	dashboardProtoToSchemaGaugeAggregation            = ReverseMap(dashboardSchemaToProtoGaugeAggregation)
	dashboardValidGaugeAggregations                   = GetKeys(dashboardSchemaToProtoGaugeAggregation)
	dashboardSchemaToProtoSpansAggregationMetricField = map[string]dashboards.SpansAggregation_MetricAggregation_MetricField{
		"unspecified": dashboards.SpansAggregation_MetricAggregation_METRIC_FIELD_UNSPECIFIED,
		"duration":    dashboards.SpansAggregation_MetricAggregation_METRIC_FIELD_DURATION,
	}
	dashboardProtoToSchemaSpansAggregationMetricField           = ReverseMap(dashboardSchemaToProtoSpansAggregationMetricField)
	dashboardValidSpansAggregationMetricFields                  = GetKeys(dashboardSchemaToProtoSpansAggregationMetricField)
	dashboardSchemaToProtoSpansAggregationMetricAggregationType = map[string]dashboards.SpansAggregation_MetricAggregation_MetricAggregationType{
		"unspecified":   dashboards.SpansAggregation_MetricAggregation_METRIC_AGGREGATION_TYPE_UNSPECIFIED,
		"min":           dashboards.SpansAggregation_MetricAggregation_METRIC_AGGREGATION_TYPE_MIN,
		"max":           dashboards.SpansAggregation_MetricAggregation_METRIC_AGGREGATION_TYPE_MAX,
		"avg":           dashboards.SpansAggregation_MetricAggregation_METRIC_AGGREGATION_TYPE_AVERAGE,
		"sum":           dashboards.SpansAggregation_MetricAggregation_METRIC_AGGREGATION_TYPE_SUM,
		"percentile_99": dashboards.SpansAggregation_MetricAggregation_METRIC_AGGREGATION_TYPE_PERCENTILE_99,
		"percentile_95": dashboards.SpansAggregation_MetricAggregation_METRIC_AGGREGATION_TYPE_PERCENTILE_95,
		"percentile_50": dashboards.SpansAggregation_MetricAggregation_METRIC_AGGREGATION_TYPE_PERCENTILE_50,
	}
	dashboardProtoToSchemaSpansAggregationMetricAggregationType = ReverseMap(dashboardSchemaToProtoSpansAggregationMetricAggregationType)
	dashboardValidSpansAggregationMetricAggregationTypes        = GetKeys(dashboardSchemaToProtoSpansAggregationMetricAggregationType)
	dashboardProtoToSchemaSpansAggregationDimensionField        = map[string]dashboards.SpansAggregation_DimensionAggregation_DimensionField{
		"unspecified": dashboards.SpansAggregation_DimensionAggregation_DIMENSION_FIELD_UNSPECIFIED,
		"trace_id":    dashboards.SpansAggregation_DimensionAggregation_DIMENSION_FIELD_TRACE_ID,
	}
	dashboardSchemaToProtoSpansAggregationDimensionField           = ReverseMap(dashboardProtoToSchemaSpansAggregationDimensionField)
	dashboardValidSpansAggregationDimensionFields                  = GetKeys(dashboardProtoToSchemaSpansAggregationDimensionField)
	dashboardSchemaToProtoSpansAggregationDimensionAggregationType = map[string]dashboards.SpansAggregation_DimensionAggregation_DimensionAggregationType{
		"unspecified":  dashboards.SpansAggregation_DimensionAggregation_DIMENSION_AGGREGATION_TYPE_UNSPECIFIED,
		"unique_count": dashboards.SpansAggregation_DimensionAggregation_DIMENSION_AGGREGATION_TYPE_UNIQUE_COUNT,
		"error_count":  dashboards.SpansAggregation_DimensionAggregation_DIMENSION_AGGREGATION_TYPE_ERROR_COUNT,
	}
	dashboardProtoToSchemaSpansAggregationDimensionAggregationType = ReverseMap(dashboardSchemaToProtoSpansAggregationDimensionAggregationType)
	dashboardValidSpansAggregationDimensionAggregationTypes        = GetKeys(dashboardSchemaToProtoSpansAggregationDimensionAggregationType)
	dashboardSchemaToProtoSpanFieldMetadataField                   = map[string]dashboards.SpanField_MetadataField{
		"unspecified":      dashboards.SpanField_METADATA_FIELD_UNSPECIFIED,
		"application_name": dashboards.SpanField_METADATA_FIELD_APPLICATION_NAME,
		"subsystem_name":   dashboards.SpanField_METADATA_FIELD_SUBSYSTEM_NAME,
		"service_name":     dashboards.SpanField_METADATA_FIELD_SERVICE_NAME,
		"operation_name":   dashboards.SpanField_METADATA_FIELD_OPERATION_NAME,
	}
	dashboardProtoToSchemaSpanFieldMetadataField = ReverseMap(dashboardSchemaToProtoSpanFieldMetadataField)
	dashboardValidSpanFieldMetadataFields        = GetKeys(dashboardSchemaToProtoSpanFieldMetadataField)
	dashboardValidLogsAggregationTypes           = []string{"count", "count_distinct", "sum", "avg", "min", "max"}
	dashboardValidAggregationTypes               = []string{"sum", "avg", "min", "max", "last"}
	dashboardValidSpanFieldTypes                 = []string{"metadata", "tag", "process_tag"}
	dashboardValidSpanAggregationTypes           = []string{"metric", "dimension"}
)

var (
	_ resource.ResourceWithConfigure = &DashboardResource{}
	//_ resource.ResourceWithConfigValidators = &DashboardResource{}
	_ resource.ResourceWithImportState = &DashboardResource{}
)

type DashboardResourceModel struct {
	ID          types.String             `tfsdk:"id"`
	Name        types.String             `tfsdk:"name"`
	Description types.String             `tfsdk:"description"`
	Layout      *DashboardLayoutModel    `tfsdk:"layout"`
	Variables   types.List               `tfsdk:"variables"` //DashboardVariableModel
	Filters     types.List               `tfsdk:"filters"`   //DashboardFilterModel
	TimeFrame   *DashboardTimeFrameModel `tfsdk:"time_frame"`
	ContentJson types.String             `tfsdk:"content_json"`
}

type DashboardLayoutModel struct {
	Sections types.List `tfsdk:"sections"` //SectionModel
}

type SectionModel struct {
	ID   types.String `tfsdk:"id"`
	Rows types.List   `tfsdk:"rows"` //RowModel
}

type RowModel struct {
	ID      types.String `tfsdk:"id"`
	Height  types.Int64  `tfsdk:"height"`
	Widgets types.List   `tfsdk:"widgets"` //WidgetModel
}

type WidgetModel struct {
	ID          types.String           `tfsdk:"id"`
	Title       types.String           `tfsdk:"title"`
	Description types.String           `tfsdk:"description"`
	Definition  *WidgetDefinitionModel `tfsdk:"definition"`
	Width       types.Int64            `tfsdk:"width"`
}

type WidgetDefinitionModel struct {
	LineChart *LineChartModel `tfsdk:"line_chart"`
	DataTable *DataTableModel `tfsdk:"data_table"`
	Gauge     *GaugeModel     `tfsdk:"gauge"`
	PieChart  *PieChartModel  `tfsdk:"pie_chart"`
	BarChart  *BarChartModel  `tfsdk:"bar_chart"`
}

type LineChartModel struct {
	Legend           *LegendModel  `tfsdk:"legend"`
	Tooltip          *TooltipModel `tfsdk:"tooltip"`
	QueryDefinitions types.List    `tfsdk:"query_definitions"` //LineChartQueryDefinitionModel
}

type LegendModel struct {
	IsVisible    types.Bool `tfsdk:"is_visible"`
	Columns      types.List `tfsdk:"columns"` //types.String (dashboardValidLegendColumns)
	GroupByQuery types.Bool `tfsdk:"group_by_query"`
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
}

type LineChartQueryModel struct {
	Logs    *LineChartQueryLogsModel    `tfsdk:"logs"`
	Metrics *LineChartQueryMetricsModel `tfsdk:"metrics"`
	Spans   *LineChartQuerySpansModel   `tfsdk:"spans"`
}

type LineChartQueryLogsModel struct {
	LuceneQuery  types.String `tfsdk:"lucene_query"`
	GroupBy      types.List   `tfsdk:"group_by"`     //types.String
	Aggregations types.List   `tfsdk:"aggregations"` //AggregationModel
	Filters      types.List   `tfsdk:"filters"`      //FilterModel
}

type FilterModel struct {
	Field    types.String         `tfsdk:"field"`
	Operator *FilterOperatorModel `tfsdk:"operator"`
}

type FilterOperatorModel struct {
	Type           types.String `tfsdk:"type"`
	SelectedValues types.List   `tfsdk:"selected_values"` //types.String
}

type LineChartQueryMetricsModel struct {
	PromqlQuery types.String `tfsdk:"promql_query"`
	Filters     types.List   `tfsdk:"filters"` //MetricsFilterModel
}

type QueryMetricFilterModel struct {
	Metric   types.String         `tfsdk:"metric"`
	Label    types.String         `tfsdk:"label"`
	Operator *FilterOperatorModel `tfsdk:"operator"`
}

type LineChartQuerySpansModel struct {
	LuceneQuery  types.String `tfsdk:"lucene_query"`
	GroupBy      types.List   `tfsdk:"group_by"`     //SpansFieldModel
	Aggregations types.List   `tfsdk:"aggregations"` //SpansAggregationModel
	Filters      types.List   `tfsdk:"filters"`      //SpansFilterModel
}

type SpansAggregationModel struct {
	Type            types.String `tfsdk:"type"`
	AggregationType types.String `tfsdk:"aggregation_type"`
	Field           types.String `tfsdk:"field"`
}

type DataTableModel struct {
	Query          *DataTableQueryModel `tfsdk:"query"`
	ResultsPerPage types.Int64          `tfsdk:"results_per_page"`
	RowStyle       types.String         `tfsdk:"row_style"`
	Columns        types.List           `tfsdk:"columns"` //DataTableColumnModel
	OrderBy        *OrderByModel        `tfsdk:"order_by"`
}

type DataTableQueryLogsModel struct {
	LuceneQuery types.String                     `tfsdk:"lucene_query"`
	Filters     types.List                       `tfsdk:"filters"` //LogsFilterModel
	Grouping    *DataTableLogsQueryGroupingModel `tfsdk:"grouping"`
}

type LogsFilterModel struct {
	Field    types.String         `tfsdk:"field"`
	Operator *FilterOperatorModel `tfsdk:"operator"`
}

type DataTableLogsQueryGroupingModel struct {
	GroupBy      types.List `tfsdk:"group_by"`     //types.String
	Aggregations types.List `tfsdk:"aggregations"` //DataTableLogsAggregationModel
}

type DataTableLogsAggregationModel struct {
	ID          types.String          `tfsdk:"id"`
	Name        types.String          `tfsdk:"name"`
	IsVisible   types.Bool            `tfsdk:"is_visible"`
	Aggregation *LogsAggregationModel `tfsdk:"aggregation"`
}

type LogsAggregationModel struct {
	Type  types.String `tfsdk:"type"`
	Field types.String `tfsdk:"field"`
}

type DataTableQueryModel struct {
	Logs    *DataTableQueryLogsModel    `tfsdk:"logs"`
	Metrics *DataTableQueryMetricsModel `tfsdk:"metrics"`
	Spans   *DataTableQuerySpansModel   `tfsdk:"spans"`
}

type DataTableQueryMetricsModel struct {
	PromqlQuery types.String `tfsdk:"promql_query"`
	Filters     types.List   `tfsdk:"filters"` //MetricsFilterModel
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
}

type SpansFilterModel struct {
	Field    *SpansFieldModel     `tfsdk:"field"`
	Operator *FilterOperatorModel `tfsdk:"operator"`
}

type SpansFieldModel struct {
	Type  types.String `tfsdk:"type"`
	Value types.String `tfsdk:"value"`
}

type DataTableSpansQueryGroupingModel struct {
	GroupBy      types.List `tfsdk:"group_by"`     //SpansFieldModel
	Aggregations types.List `tfsdk:"aggregations"` //DataTableSpansAggregationModel
}

type GaugeModel struct {
	Query        *GaugeQueryModel `tfsdk:"query"`
	Min          types.Float64    `tfsdk:"min"`
	Max          types.Float64    `tfsdk:"max"`
	ShowInnerArc types.Bool       `tfsdk:"show_inner_arc"`
	ShowOuterArc types.Bool       `tfsdk:"show_outer_arc"`
	Unit         types.String     `tfsdk:"unit"`
	Thresholds   types.List       `tfsdk:"thresholds"` //GaugeThresholdModel
}

type GaugeQueryModel struct {
	Logs    *GaugeQueryLogsModel    `tfsdk:"logs"`
	Metrics *GaugeQueryMetricsModel `tfsdk:"metrics"`
	Spans   *GaugeQuerySpansModel   `tfsdk:"spans"`
}

type GaugeQueryLogsModel struct {
	LuceneQuery     types.String          `tfsdk:"lucene_query"`
	LogsAggregation *LogsAggregationModel `tfsdk:"logs_aggregation"`
	Aggregation     types.String          `tfsdk:"aggregation"`
	Filters         types.List            `tfsdk:"filters"` //LogsFilterModel
}

type GaugeQueryMetricsModel struct {
	PromqlQuery types.String `tfsdk:"promql_query"`
	Aggregation types.String `tfsdk:"aggregation"`
	Filters     types.List   `tfsdk:"filters"` //MetricsFilterModel
}

type GaugeQuerySpansModel struct {
	LuceneQuery      types.String           `tfsdk:"lucene_query"`
	SpansAggregation *SpansAggregationModel `tfsdk:"spans_aggregation"`
	Aggregation      types.String           `tfsdk:"aggregation"`
	Filters          types.List             `tfsdk:"filters"` //SpansFilterModel
}

type GaugeThresholdModel struct {
	From  types.Float64 `tfsdk:"from"`
	Color types.String  `tfsdk:"color"`
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
}

type PieChartStackDefinitionModel struct {
	MaxSlicesPerStack types.Int64  `tfsdk:"max_slices_per_stack"`
	StackNameTemplate types.String `tfsdk:"stack_name_template"`
}

type PieChartQueryModel struct {
	Logs    *PieChartQueryLogsModel    `tfsdk:"logs"`
	Metrics *PieChartQueryMetricsModel `tfsdk:"metrics"`
	Spans   *PieChartQuerySpansModel   `tfsdk:"spans"`
}

type PieChartQueryLogsModel struct {
	LuceneQuery      types.String          `tfsdk:"lucene_query"`
	Aggregation      *LogsAggregationModel `tfsdk:"aggregation"`
	Filters          types.List            `tfsdk:"filters"`     //LogsFilterModel
	GroupNames       types.List            `tfsdk:"group_names"` //types.String
	StackedGroupName types.String          `tfsdk:"stacked_group_name"`
}

type PieChartQueryMetricsModel struct {
	PromqlQuery      types.String `tfsdk:"promql_query"`
	Filters          types.List   `tfsdk:"filters"`     //MetricsFilterModel
	GroupNames       types.List   `tfsdk:"group_names"` //types.String
	StackedGroupName types.String `tfsdk:"stacked_group_name"`
}

type PieChartQuerySpansModel struct {
	LuceneQuery      types.String           `tfsdk:"lucene_query"`
	Aggregation      *SpansAggregationModel `tfsdk:"aggregation"`
	Filters          types.List             `tfsdk:"filters"`     //SpansFilterModel
	GroupNames       types.List             `tfsdk:"group_names"` //SpansFieldModel
	StackedGroupName *SpansFieldModel       `tfsdk:"stacked_group_name"`
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
}

type BarChartQueryModel struct {
	Logs    *BarChartQueryLogsModel    `tfsdk:"logs"`
	Metrics *BarChartQueryMetricsModel `tfsdk:"metrics"`
	Spans   *BarChartQuerySpansModel   `tfsdk:"spans"`
}

type BarChartQueryLogsModel struct {
	LuceneQuery      types.String          `tfsdk:"lucene_query"`
	Aggregation      *LogsAggregationModel `tfsdk:"aggregation"`
	Filters          types.List            `tfsdk:"filters"`     //LogsFilterModel
	GroupNames       types.List            `tfsdk:"group_names"` //types.String
	StackedGroupName types.String          `tfsdk:"stacked_group_name"`
}

type BarChartQueryMetricsModel struct {
	PromqlQuery      types.String `tfsdk:"promql_query"`
	Filters          types.List   `tfsdk:"filters"`     //MetricsFilterModel
	GroupNames       types.List   `tfsdk:"group_names"` //types.String
	StackedGroupName types.String `tfsdk:"stacked_group_name"`
}

type BarChartQuerySpansModel struct {
	LuceneQuery      types.String           `tfsdk:"lucene_query"`
	Aggregation      *SpansAggregationModel `tfsdk:"aggregation"`
	Filters          types.List             `tfsdk:"filters"`     //SpansFilterModel
	GroupNames       types.List             `tfsdk:"group_names"` //SpansFieldModel
	StackedGroupName *SpansFieldModel       `tfsdk:"stacked_group_name"`
}

type DataTableSpansAggregationModel struct {
	ID          types.String           `json:"id"`
	Name        types.String           `json:"name"`
	IsVisible   types.Bool             `json:"is_visible"`
	Aggregation *SpansAggregationModel `json:"aggregation"`
}

type BarChartStackDefinitionModel struct {
	MaxSlicesPerBar   types.Int64  `tfsdk:"max_slices_per_bar"`
	StackNameTemplate types.String `tfsdk:"stack_name_template"`
}

type BarChartXAxisModel struct {
	Type             types.String `tfsdk:"type"`
	Interval         types.String `tfsdk:"interval"`
	BucketsPresented types.Int64  `tfsdk:"buckets_presented"`
}

type DashboardVariableModel struct {
	Name        types.String                      `tfsdk:"name"`
	Definition  *DashboardVariableDefinitionModel `tfsdk:"definition"`
	DisplayName types.String                      `tfsdk:"display_name"`
}

type MetricMultiSelectSourceModel struct {
	MetricName types.String `tfsdk:"metric_name"`
	Label      types.String `tfsdk:"label"`
}

type DashboardVariableDefinitionModel struct {
	ConstantValue types.String              `tfsdk:"constant_value"`
	MultiSelect   *VariableMultiSelectModel `tfsdk:"multi_select"`
}

type VariableMultiSelectModel struct {
	SelectedValues       types.List                      `tfsdk:"selected_values"` //types.String
	ValuesOrderDirection types.String                    `tfsdk:"values_order_direction"`
	Source               *VariableMultiSelectSourceModel `tfsdk:"source"`
}

type VariableMultiSelectSourceModel struct {
	LogsPath     types.String                  `tfsdk:"logs_path"`
	MetricLabel  *MetricMultiSelectSourceModel `tfsdk:"metric_label"`
	ConstantList types.List                    `tfsdk:"constant_list"` //types.String
	SpanField    *SpansFieldModel              `tfsdk:"span_field"`
}

type DashboardFilterModel struct {
	Source    *DashboardFilterSourceModel `tfsdk:"source"`
	Enabled   types.Bool                  `tfsdk:"enabled"`
	Collapsed types.Bool                  `tfsdk:"collapsed"`
}

type DashboardFilterSourceModel struct {
	Logs    *FilterSourceLogsModel    `tfsdk:"logs"`
	Metrics *FilterSourceMetricsModel `tfsdk:"metrics"`
	Spans   *FilterSourceSpansModel   `tfsdk:"spans"`
}

type FilterSourceLogsModel struct {
	Field    types.String         `tfsdk:"field"`
	Operator *FilterOperatorModel `tfsdk:"operator"`
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

type DashboardTimeFrameModel struct {
	Absolute *DashboardTimeFrameAbsoluteModel `tfsdk:"absolute"`
	Relative *DashboardTimeFrameRelativeModel `tfsdk:"relative"`
}

type DashboardTimeFrameAbsoluteModel struct {
	From types.String `tfsdk:"from"`
	To   types.String `tfsdk:"to"`
}

type DashboardTimeFrameRelativeModel struct {
	Duration types.String `tfsdk:"duration"`
}

func NewDashboardResource() resource.Resource {
	return &DashboardResource{}
}

type DashboardResource struct {
	client *clientset.DashboardsClient
}

func (r DashboardResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

//func (r DashboardResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
//	return []resource.ConfigValidator{
//		&resource.StructFieldValidator{
//			FieldName: "name",
//			Struct:    &DashboardModel{},
//			Err:       resource.NewValidationError("name is required"),
//		},
//	}
//}

func (r DashboardResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dashboard"
}

func (r DashboardResource) Schema(_ context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
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
				Optional:            true,
				MarkdownDescription: "Dashboard name.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Dashboard description.",
			},
			"layout": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"sections": schema.ListNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"id": schema.StringAttribute{
									Computed: true,
									PlanModifiers: []planmodifier.String{
										stringplanmodifier.UseStateForUnknown(),
									},
								},
								"rows": schema.ListNestedAttribute{
									NestedObject: schema.NestedAttributeObject{
										Attributes: map[string]schema.Attribute{
											"id": schema.StringAttribute{
												Computed: true,
												PlanModifiers: []planmodifier.String{
													stringplanmodifier.UseStateForUnknown(),
												},
											},
											"height": schema.Int64Attribute{
												Optional:            true,
												MarkdownDescription: "The height of the row.",
											},
											"widgets": schema.ListNestedAttribute{
												Optional: true,
												NestedObject: schema.NestedAttributeObject{
													Attributes: map[string]schema.Attribute{
														"id": schema.StringAttribute{
															Computed: true,
															PlanModifiers: []planmodifier.String{
																stringplanmodifier.UseStateForUnknown(),
															},
														},
														"title": schema.StringAttribute{
															Optional:            true,
															MarkdownDescription: "Widget title.",
														},
														"description": schema.StringAttribute{
															Optional:            true,
															MarkdownDescription: "Widget description.",
														},
														"definition": schema.SingleNestedAttribute{
															Required: true,
															Attributes: map[string]schema.Attribute{
																"line_chart": schema.SingleNestedAttribute{
																	Attributes: map[string]schema.Attribute{
																		"legend": schema.SingleNestedAttribute{
																			Attributes: map[string]schema.Attribute{
																				"is_visible": schema.BoolAttribute{
																					Optional:            true,
																					Computed:            true,
																					Default:             booldefault.StaticBool(false),
																					MarkdownDescription: "Whether to display the legend. False by default.",
																				},
																				"columns": schema.ListAttribute{
																					ElementType: types.StringType,
																					Optional:    true,
																					Validators: []validator.List{
																						listvalidator.ValueStringsAre(stringvalidator.OneOf(dashboardValidLegendColumns...)),
																						listvalidator.SizeAtLeast(1),
																					},
																					MarkdownDescription: fmt.Sprintf("The columns to display in the legend. Valid values are: %s.", strings.Join(dashboardValidLegendColumns, ", ")),
																				},
																				"group_by_query": schema.BoolAttribute{
																					Optional: true,
																					Computed: true,
																					Default:  booldefault.StaticBool(false),
																				},
																			},
																			Optional: true,
																		},
																		"tooltip": schema.SingleNestedAttribute{
																			Attributes: map[string]schema.Attribute{
																				"show_labels": schema.BoolAttribute{
																					Optional: true,
																					Computed: true,
																					Default:  booldefault.StaticBool(false),
																				},
																				"type": schema.StringAttribute{
																					Optional: true,
																					Validators: []validator.String{
																						stringvalidator.OneOf(dashboardValidTooltipTypes...),
																					},
																					MarkdownDescription: fmt.Sprintf("The tooltip type. Valid values are: %s.", strings.Join(dashboardValidTooltipTypes, ", ")),
																				},
																			},
																			Optional: true,
																		},
																		"query_definitions": schema.ListNestedAttribute{
																			NestedObject: schema.NestedAttributeObject{
																				Attributes: map[string]schema.Attribute{
																					"id": schema.StringAttribute{
																						Computed: true, PlanModifiers: []planmodifier.String{
																							stringplanmodifier.UseStateForUnknown(),
																						},
																					},
																					"query": schema.SingleNestedAttribute{
																						Attributes: map[string]schema.Attribute{
																							"logs": schema.SingleNestedAttribute{
																								Attributes: map[string]schema.Attribute{
																									"lucene_query": schema.StringAttribute{
																										Optional: true,
																									},
																									"group_by": schema.ListAttribute{
																										ElementType: types.StringType,
																										Optional:    true,
																									},
																									"filters":      logsFiltersSchema(),
																									"aggregations": logsAggregationsSchema(),
																								},
																								Optional: true,
																								Validators: []validator.Object{
																									objectvalidator.ExactlyOneOf(
																										path.MatchRelative().AtParent().AtName("metrics"),
																										path.MatchRelative().AtParent().AtName("spans"),
																									),
																								},
																							},
																							"metrics": schema.SingleNestedAttribute{
																								Attributes: map[string]schema.Attribute{
																									"promql_query": schema.StringAttribute{
																										Optional: true,
																									},
																									"filters": metricFiltersSchema(),
																								},
																								Optional: true,
																								Validators: []validator.Object{
																									objectvalidator.ExactlyOneOf(
																										path.MatchRelative().AtParent().AtName("logs"),
																										path.MatchRelative().AtParent().AtName("spans"),
																									),
																								},
																							},
																							"spans": schema.SingleNestedAttribute{
																								Attributes: map[string]schema.Attribute{
																									"lucene_query": schema.StringAttribute{
																										Optional: true,
																									},
																									"group_by":     spansFieldsSchema(),
																									"aggregations": spansAggregationsSchema(),
																									"filters":      spansFilterSchema(),
																								},
																								Optional: true,
																								Validators: []validator.Object{
																									objectvalidator.ExactlyOneOf(
																										path.MatchRelative().AtParent().AtName("metrics"),
																										path.MatchRelative().AtParent().AtName("logs"),
																									),
																								},
																							},
																						},
																						Required: true,
																					},
																					"series_name_template": schema.StringAttribute{
																						Optional: true,
																					},
																					"series_count_limit": schema.Int64Attribute{
																						Optional: true,
																					},
																					"unit": schema.StringAttribute{
																						Optional: true,
																						Computed: true,
																						Default:  stringdefault.StaticString("unspecified"),
																						Validators: []validator.String{
																							stringvalidator.OneOf(dashboardValidUnits...),
																						},
																						MarkdownDescription: fmt.Sprintf("The unit. Valid values are: %s.", strings.Join(dashboardValidUnits, ", ")),
																					},
																					"scale_type": schema.StringAttribute{
																						Optional: true,
																						Computed: true,
																						Validators: []validator.String{
																							stringvalidator.OneOf(dashboardValidScaleTypes...),
																						},
																						Default:             stringdefault.StaticString("unspecified"),
																						MarkdownDescription: fmt.Sprintf("The scale type. Valid values are: %s.", strings.Join(dashboardValidScaleTypes, ", ")),
																					},
																					"name": schema.StringAttribute{
																						Optional: true,
																					},
																					"is_visible": schema.BoolAttribute{
																						Optional: true,
																						Computed: true,
																						Default:  booldefault.StaticBool(false),
																					},
																				},
																			},
																			Required: true,
																			Validators: []validator.List{
																				listvalidator.SizeAtLeast(1),
																			},
																		},
																	},
																	Validators: []validator.Object{
																		objectvalidator.ExactlyOneOf(
																			path.MatchRelative().AtParent().AtName("data_table"),
																			path.MatchRelative().AtParent().AtName("gauge"),
																			path.MatchRelative().AtParent().AtName("pie_chart"),
																			path.MatchRelative().AtParent().AtName("bar_chart"),
																		),
																	},
																	Optional: true,
																},
																"data_table": schema.SingleNestedAttribute{
																	Attributes: map[string]schema.Attribute{
																		"query": schema.SingleNestedAttribute{
																			Attributes: map[string]schema.Attribute{
																				"logs": schema.SingleNestedAttribute{
																					Attributes: map[string]schema.Attribute{
																						"lucene_query": schema.StringAttribute{
																							Optional: true,
																						},
																						"filters": logsFiltersSchema(),
																						"grouping": schema.SingleNestedAttribute{
																							Attributes: map[string]schema.Attribute{
																								"group_by": schema.StringAttribute{
																									Optional: true,
																								},
																								"aggregations": schema.ListNestedAttribute{
																									NestedObject: schema.NestedAttributeObject{
																										Attributes: map[string]schema.Attribute{
																											"id": schema.StringAttribute{
																												Computed: true,
																												PlanModifiers: []planmodifier.String{
																													stringplanmodifier.UseStateForUnknown(),
																												},
																											},
																											"name": schema.StringAttribute{
																												Optional: true,
																											},
																											"is_visible": schema.BoolAttribute{
																												Optional: true,
																											},
																											"aggregation": logsAggregationSchema(),
																										},
																									},
																									Optional: true,
																								},
																							},
																							Optional: true,
																						},
																					},
																					Optional: true,
																				},
																				"spans": schema.SingleNestedAttribute{
																					Attributes: map[string]schema.Attribute{
																						"lucene_query": schema.StringAttribute{
																							Optional: true,
																						},
																						"filters": spansFilterSchema(),
																						"grouping": schema.SingleNestedAttribute{
																							Attributes: map[string]schema.Attribute{
																								"group_by": spansFieldsSchema(),
																								"aggregations": schema.ListNestedAttribute{
																									NestedObject: schema.NestedAttributeObject{
																										Attributes: map[string]schema.Attribute{
																											"id": schema.StringAttribute{
																												Computed: true,
																												PlanModifiers: []planmodifier.String{
																													stringplanmodifier.UseStateForUnknown(),
																												},
																											},
																											"name": schema.StringAttribute{
																												Optional: true,
																											},
																											"is_visible": schema.BoolAttribute{
																												Optional: true,
																												Computed: true,
																												Default:  booldefault.StaticBool(false),
																											},
																											"aggregation": spansAggregationSchema(),
																										},
																									},
																									Optional: true,
																								},
																							},
																							Optional: true,
																						},
																					},
																					Optional: true,
																				},
																				"metrics": schema.SingleNestedAttribute{
																					Attributes: map[string]schema.Attribute{
																						"promql_query": schema.StringAttribute{
																							Optional: true,
																						},
																						"filters": metricFiltersSchema(),
																					},
																					Optional: true,
																				},
																			},
																			Optional: true,
																		},
																		"results_per_page": schema.Int64Attribute{
																			Required:            true,
																			MarkdownDescription: "The number of results to display per page.",
																		},
																		"row_style": schema.StringAttribute{
																			Required: true,
																			Validators: []validator.String{
																				stringvalidator.OneOf(dashboardValidRowStyles...),
																			},
																			MarkdownDescription: fmt.Sprintf("The style of the rows. Can be one of %q.", dashboardValidRowStyles),
																		},
																		"columns": schema.ListNestedAttribute{
																			NestedObject: schema.NestedAttributeObject{
																				Attributes: map[string]schema.Attribute{
																					"field": schema.StringAttribute{
																						Required: true,
																					},
																					"width": schema.Int64Attribute{
																						Optional: true,
																					},
																				},
																			},
																			Required: true,
																			Validators: []validator.List{
																				listvalidator.SizeAtLeast(1),
																			},
																		},
																		"order_by": schema.SingleNestedAttribute{
																			Attributes: map[string]schema.Attribute{
																				"field": schema.StringAttribute{
																					Optional: true,
																				},
																				"order_direction": schema.StringAttribute{
																					Validators: []validator.String{
																						stringvalidator.OneOf(dashboardValidOrderDirections...),
																					},
																					MarkdownDescription: fmt.Sprintf("The order direction. Can be one of %q.", dashboardValidOrderDirections),
																					Optional:            true,
																					Computed:            true,
																					Default:             stringdefault.StaticString("unspecified"),
																				},
																			},
																			Optional: true,
																		},
																	},
																	Validators: []validator.Object{
																		objectvalidator.ExactlyOneOf(
																			path.MatchRelative().AtParent().AtName("line_chart"),
																			path.MatchRelative().AtParent().AtName("gauge"),
																			path.MatchRelative().AtParent().AtName("pie_chart"),
																			path.MatchRelative().AtParent().AtName("bar_chart"),
																		),
																	},
																	Optional: true,
																},
																"gauge": schema.SingleNestedAttribute{
																	Attributes: map[string]schema.Attribute{
																		"query": schema.SingleNestedAttribute{
																			Attributes: map[string]schema.Attribute{
																				"logs": schema.SingleNestedAttribute{
																					Attributes: map[string]schema.Attribute{
																						"lucene_query": schema.StringAttribute{
																							Optional: true,
																						},
																						"aggregation": schema.StringAttribute{
																							Validators: []validator.String{
																								stringvalidator.OneOf(dashboardValidAggregationTypes...),
																							},
																							MarkdownDescription: fmt.Sprintf("The type of aggregation. Can be one of %q.", dashboardValidAggregationTypes),
																							Optional:            true,
																						},
																						"filters":          logsFiltersSchema(),
																						"logs_aggregation": logsAggregationSchema(),
																					},
																					Validators: []validator.Object{
																						objectvalidator.ExactlyOneOf(
																							path.MatchRelative().AtParent().AtName("spans"),
																							path.MatchRelative().AtParent().AtName("metrics"),
																						),
																					},
																					Optional: true,
																				},
																				"metrics": schema.SingleNestedAttribute{
																					Attributes: map[string]schema.Attribute{
																						"promql_query": schema.StringAttribute{
																							Optional: true,
																						},
																						"aggregation": schema.StringAttribute{
																							Validators: []validator.String{
																								stringvalidator.OneOf(dashboardValidAggregationTypes...),
																							},
																							MarkdownDescription: fmt.Sprintf("The type of aggregation. Can be one of %q.", dashboardValidAggregationTypes),
																							Optional:            true,
																						},
																						"filters": metricFiltersSchema(),
																					},
																					Optional: true,
																					Validators: []validator.Object{
																						objectvalidator.ExactlyOneOf(
																							path.MatchRelative().AtParent().AtName("logs"),
																							path.MatchRelative().AtParent().AtName("spans"),
																						),
																					},
																				},
																				"spans": schema.SingleNestedAttribute{
																					Attributes: map[string]schema.Attribute{
																						"lucene_query": schema.StringAttribute{
																							Optional: true,
																						},
																						"spans_aggregation": spansAggregationSchema(),
																						"aggregation": schema.StringAttribute{
																							Validators: []validator.String{
																								stringvalidator.OneOf(dashboardValidGaugeAggregations...),
																							},
																							MarkdownDescription: fmt.Sprintf("The type of aggregation. Can be one of %q.", dashboardValidGaugeAggregations),
																							Optional:            true,
																						},
																						"filters": spansFilterSchema(),
																					},
																					Optional: true,
																					Validators: []validator.Object{
																						objectvalidator.ExactlyOneOf(
																							path.MatchRelative().AtParent().AtName("logs"),
																							path.MatchRelative().AtParent().AtName("metrics"),
																						),
																					},
																				},
																			},
																			Optional: true,
																		},
																		"min": schema.Float64Attribute{
																			Optional: true,
																		},
																		"max": schema.Float64Attribute{
																			Optional: true,
																		},
																		"show_inner_arc": schema.BoolAttribute{
																			Optional: true,
																			Computed: true,
																			Default:  booldefault.StaticBool(false),
																		},
																		"show_outer_arc": schema.BoolAttribute{
																			Optional: true,
																			Computed: true,
																			Default:  booldefault.StaticBool(true),
																		},
																		"unit": schema.StringAttribute{
																			Optional: true,
																			Computed: true,
																			Default:  stringdefault.StaticString("unspecified"),
																			Validators: []validator.String{
																				stringvalidator.OneOf(dashboardValidGaugeUnits...),
																			},
																			MarkdownDescription: fmt.Sprintf("The unit of the gauge. Can be one of %q.", dashboardValidGaugeUnits),
																		},
																		"thresholds": schema.ListNestedAttribute{
																			NestedObject: schema.NestedAttributeObject{
																				Attributes: map[string]schema.Attribute{
																					"color": schema.StringAttribute{
																						Optional: true,
																					},
																					"from": schema.Float64Attribute{
																						Optional: true,
																					},
																				},
																			},
																			Optional: true,
																		},
																	},
																	Validators: []validator.Object{
																		objectvalidator.ExactlyOneOf(
																			path.MatchRelative().AtParent().AtName("line_chart"),
																			path.MatchRelative().AtParent().AtName("data_table"),
																			path.MatchRelative().AtParent().AtName("pie_chart"),
																			path.MatchRelative().AtParent().AtName("bar_chart"),
																		),
																	},
																	Optional: true,
																},
																"pie_chart": schema.SingleNestedAttribute{
																	Attributes: map[string]schema.Attribute{
																		"query": schema.SingleNestedAttribute{
																			Attributes: map[string]schema.Attribute{
																				"logs": schema.SingleNestedAttribute{
																					Attributes: map[string]schema.Attribute{
																						"lucene_query": schema.StringAttribute{
																							Optional: true,
																						},
																						"aggregation": logsAggregationSchema(),
																						"filters":     logsFiltersSchema(),
																						"group_names": schema.ListAttribute{
																							ElementType: types.StringType,
																							Optional:    true,
																						},
																						"stacked_group_name": schema.StringAttribute{
																							Optional: true,
																						},
																					},
																					Optional: true,
																					Validators: []validator.Object{
																						objectvalidator.ExactlyOneOf(
																							path.MatchRelative().AtParent().AtName("spans"),
																							path.MatchRelative().AtParent().AtName("metrics"),
																						),
																					},
																				},
																				"spans": schema.SingleNestedAttribute{
																					Attributes: map[string]schema.Attribute{
																						"lucene_query": schema.StringAttribute{
																							Optional: true,
																						},
																						"aggregation":        spansAggregationSchema(),
																						"filters":            spansFilterSchema(),
																						"group_names":        spansFieldsSchema(),
																						"stacked_group_name": spansFieldSchema(),
																					},
																					Optional: true,
																					Validators: []validator.Object{
																						objectvalidator.ExactlyOneOf(
																							path.MatchRelative().AtParent().AtName("logs"),
																							path.MatchRelative().AtParent().AtName("metrics"),
																						),
																					},
																				},
																				"metrics": schema.SingleNestedAttribute{
																					Attributes: map[string]schema.Attribute{
																						"promql_query": schema.StringAttribute{
																							Optional: true,
																						},
																						"filters": metricFiltersSchema(),
																						"group_names": schema.ListAttribute{
																							ElementType: types.StringType,
																							Optional:    true,
																						},
																						"stacked_group_name": schema.StringAttribute{
																							Optional: true,
																						},
																					},
																					Optional: true,
																					Validators: []validator.Object{
																						objectvalidator.ExactlyOneOf(
																							path.MatchRelative().AtParent().AtName("logs"),
																							path.MatchRelative().AtParent().AtName("spans"),
																						),
																					},
																				},
																			},
																			Optional: true,
																		},
																		"max_slices_per_chart": schema.Int64Attribute{
																			Optional: true,
																		},
																		"min_slice_percentage": schema.Int64Attribute{
																			Optional: true,
																		},
																		"stack_definition": schema.SingleNestedAttribute{
																			Attributes: map[string]schema.Attribute{
																				"max_slices_per_stack": schema.Int64Attribute{
																					Optional: true,
																				},
																				"stack_name_template": schema.StringAttribute{
																					Optional: true,
																				},
																			},
																			Optional: true,
																		},
																		"label_definition": schema.SingleNestedAttribute{
																			Attributes: map[string]schema.Attribute{
																				"label_source": schema.StringAttribute{
																					Optional: true,
																					Computed: true,
																					Default:  stringdefault.StaticString("unspecified"),
																					Validators: []validator.String{
																						stringvalidator.OneOf(dashboardValidPieChartLabelSources...),
																					},
																					MarkdownDescription: fmt.Sprintf("The source of the label. Valid values are: %s", strings.Join(dashboardValidPieChartLabelSources, ", ")),
																				},
																				"is_visible": schema.BoolAttribute{
																					Optional: true,
																				},
																				"show_name": schema.BoolAttribute{
																					Optional: true,
																				},
																				"show_value": schema.BoolAttribute{
																					Optional: true,
																				},
																				"show_percentage": schema.BoolAttribute{
																					Optional: true,
																				},
																			},
																			Optional: true,
																		},
																		"show_legend": schema.BoolAttribute{
																			Optional: true,
																		},
																		"group_name_template": schema.StringAttribute{
																			Optional: true,
																		},
																		"unit": schema.StringAttribute{
																			Optional: true,
																		},
																	},
																	Validators: []validator.Object{
																		objectvalidator.ExactlyOneOf(
																			path.MatchRelative().AtParent().AtName("line_chart"),
																			path.MatchRelative().AtParent().AtName("gauge"),
																			path.MatchRelative().AtParent().AtName("data_table"),
																			path.MatchRelative().AtParent().AtName("bar_chart"),
																		),
																	},
																	Optional: true,
																},
																"bar_chart": schema.SingleNestedAttribute{
																	Attributes: map[string]schema.Attribute{
																		"query": schema.SingleNestedAttribute{
																			Attributes: map[string]schema.Attribute{
																				"logs": schema.SingleNestedAttribute{
																					Attributes: map[string]schema.Attribute{
																						"lucene_query": schema.StringAttribute{
																							Optional: true,
																						},
																						"aggregation": logsAggregationSchema(),
																						"filters":     logsFiltersSchema(),
																						"group_names": schema.ListAttribute{
																							ElementType: types.StringType,
																							Optional:    true,
																						},
																						"stacked_group_name": schema.StringAttribute{
																							Optional: true,
																						},
																					},
																					Optional: true,
																				},
																				"metrics": schema.SingleNestedAttribute{
																					Attributes: map[string]schema.Attribute{
																						"promql_query": schema.StringAttribute{
																							Optional: true,
																						},
																						"filters": metricFiltersSchema(),
																						"group_names": schema.ListAttribute{
																							ElementType: types.StringType,
																							Optional:    true,
																						},
																						"stacked_group_name": schema.StringAttribute{
																							Optional: true,
																						},
																					},
																					Optional: true,
																				},
																				"spans": schema.SingleNestedAttribute{
																					Attributes: map[string]schema.Attribute{
																						"lucene_query": schema.StringAttribute{
																							Optional: true,
																						},
																						"aggregation":        spansAggregationSchema(),
																						"filters":            spansFilterSchema(),
																						"group_names":        spansFieldsSchema(),
																						"stacked_group_name": spansFieldSchema(),
																					},
																					Optional: true,
																				},
																			},
																			Optional: true,
																		},
																		"max_bars_per_chart": schema.Int64Attribute{
																			Optional: true,
																		},
																		"group_name_template": schema.StringAttribute{
																			Optional: true,
																		},
																		"stack_definition": schema.SingleNestedAttribute{
																			Optional: true,
																			Attributes: map[string]schema.Attribute{
																				"max_slices_per_bar": schema.Int64Attribute{
																					Optional: true,
																				},
																				"stack_name_template": schema.StringAttribute{
																					Optional: true,
																				},
																			},
																		},
																		"scale_type": schema.StringAttribute{
																			Optional: true,
																		},
																		"colors_by": schema.StringAttribute{
																			Optional: true,
																		},
																		"xaxis": schema.SingleNestedAttribute{
																			Optional: true,
																			Attributes: map[string]schema.Attribute{
																				"type": schema.StringAttribute{
																					Required: true,
																					Validators: []validator.String{
																						stringvalidator.OneOf("value", "time"),
																					},
																				},
																				"interval": schema.StringAttribute{
																					Optional: true,
																				},
																				"buckets_presented": schema.Int64Attribute{
																					Optional: true,
																				},
																			},
																		},
																		"unit": schema.StringAttribute{
																			Optional: true,
																		},
																	},
																	Validators: []validator.Object{
																		objectvalidator.ExactlyOneOf(
																			path.MatchRelative().AtParent().AtName("data_table"),
																			path.MatchRelative().AtParent().AtName("gauge"),
																			path.MatchRelative().AtParent().AtName("pie_chart"),
																			path.MatchRelative().AtParent().AtName("line_chart"),
																		),
																	},
																	Optional: true,
																},
															},
															MarkdownDescription: "The widget definition. Can contain one of `line_chart`, `bar_chart`, `pie_chart` `data_table` or `gauge`.",
														},
														"width": schema.Int64Attribute{
															Optional:            true,
															MarkdownDescription: "The width of the chart.",
														},
													},
												},
												Validators: []validator.List{
													listvalidator.SizeAtLeast(1),
												},
												MarkdownDescription: "The list of widgets to display in the dashboard.",
											},
										},
									},
									Validators: []validator.List{
										listvalidator.SizeAtLeast(1),
									},
									Optional: true,
								},
							},
						},
						Optional: true,
						Validators: []validator.List{
							listvalidator.SizeAtLeast(1),
						},
					},
				},
			},
			"variables": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Optional: true,
						},
						"definition": schema.SingleNestedAttribute{
							Required: true,
							Attributes: map[string]schema.Attribute{
								"constant_value": schema.StringAttribute{
									Optional: true,
									Validators: []validator.String{
										stringvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("multi_select")),
									},
								},
								"multi_select": schema.SingleNestedAttribute{
									Attributes: map[string]schema.Attribute{
										"selected_values": schema.ListAttribute{
											ElementType: types.StringType,
											Optional:    true,
										},
										"values_order_direction": schema.StringAttribute{
											Optional: true,
											Computed: true,
											Default:  stringdefault.StaticString("unspecified"),
											Validators: []validator.String{
												stringvalidator.OneOf(dashboardValidOrderDirections...),
											},
											MarkdownDescription: fmt.Sprintf("The order direction of the values. Can be one of `%s`.", strings.Join(dashboardValidOrderDirections, "`, `")),
										},
										"source": schema.SingleNestedAttribute{
											Attributes: map[string]schema.Attribute{
												"logs_path": schema.StringAttribute{
													Optional: true,
													Validators: []validator.String{
														stringvalidator.ExactlyOneOf(
															path.MatchRelative().AtParent().AtName("metric_label"),
															path.MatchRelative().AtParent().AtName("constant_list"),
															path.MatchRelative().AtParent().AtName("span_field"),
														),
													},
												},
												"metric_label": schema.SingleNestedAttribute{
													Attributes: map[string]schema.Attribute{
														"metric_name": schema.StringAttribute{
															Required: true,
														},
														"label": schema.StringAttribute{
															Required: true,
														},
													},
													Optional: true,
													Validators: []validator.Object{
														objectvalidator.ExactlyOneOf(
															path.MatchRelative().AtParent().AtName("logs_path"),
															path.MatchRelative().AtParent().AtName("constant_list"),
															path.MatchRelative().AtParent().AtName("span_field"),
														),
													},
												},
												"constant_list": schema.ListAttribute{
													ElementType: types.StringType,
													Optional:    true,
													Validators: []validator.List{
														listvalidator.ExactlyOneOf(
															path.MatchRelative().AtParent().AtName("logs_path"),
															path.MatchRelative().AtParent().AtName("metric_label"),
															path.MatchRelative().AtParent().AtName("span_field"),
														),
													},
												},
												"span_field": schema.SingleNestedAttribute{
													Attributes: spansFieldAttributes(),
													Optional:   true,
													Validators: []validator.Object{
														spansFieldValidator{},
														objectvalidator.ExactlyOneOf(
															path.MatchRelative().AtParent().AtName("logs_path"),
															path.MatchRelative().AtParent().AtName("metric_label"),
															path.MatchRelative().AtParent().AtName("constant_list"),
														),
													},
												},
											},
											Optional: true,
										},
									},
									Optional: true,
									Validators: []validator.Object{
										objectvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("constant_value")),
									},
								},
							},
						},
						"display_name": schema.StringAttribute{
							Optional: true,
						},
					},
				},
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
			},
			"filters": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"source": schema.SingleNestedAttribute{
							Attributes: map[string]schema.Attribute{
								"logs": schema.SingleNestedAttribute{
									Attributes: map[string]schema.Attribute{
										"field": schema.StringAttribute{
											Required: true,
										},
										"operator": filterOperatorSchema(),
									},
									Optional: true,
									Validators: []validator.Object{
										objectvalidator.ExactlyOneOf(
											path.MatchRelative().AtParent().AtName("metrics"),
											path.MatchRelative().AtParent().AtName("spans"),
										),
									},
								},
								"spans": schema.SingleNestedAttribute{
									Attributes: map[string]schema.Attribute{
										"field": schema.SingleNestedAttribute{
											Attributes: spansFieldAttributes(),
											Required:   true,
											Validators: []validator.Object{
												spansFieldValidator{},
											},
										},
										"operator": filterOperatorSchema(),
									},
									Optional: true,
									Validators: []validator.Object{
										objectvalidator.ExactlyOneOf(
											path.MatchRelative().AtParent().AtName("metrics"),
											path.MatchRelative().AtParent().AtName("logs"),
										),
									},
								},
								"metrics": schema.SingleNestedAttribute{
									Attributes: map[string]schema.Attribute{
										"metric_name": schema.StringAttribute{
											Required: true,
										},
										"label": schema.StringAttribute{
											Required: true,
										},
										"operator": filterOperatorSchema(),
									},
									Validators: []validator.Object{
										objectvalidator.ExactlyOneOf(
											path.MatchRelative().AtParent().AtName("spans"),
											path.MatchRelative().AtParent().AtName("logs"),
										),
									},
									Optional: true,
								},
							},
							Required: true,
						},
						"enabled": schema.BoolAttribute{
							Optional: true,
							Computed: true,
							Default:  booldefault.StaticBool(true),
						},
						"collapsed": schema.BoolAttribute{
							Optional: true,
							Computed: true,
							Default:  booldefault.StaticBool(false),
						},
					},
				},
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
			},
			"time_frame": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"absolute": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"start": schema.StringAttribute{
								Required: true,
							},
							"end": schema.StringAttribute{
								Required: true,
							},
						},
						Optional: true,
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("relative")),
						},
					},
					"relative": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"duration": schema.StringAttribute{
								Required: true,
							},
						},
						Optional: true,
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("absolute")),
						},
					},
				},
			},
			"content_json": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(
						path.MatchRelative().AtParent().AtName("id"),
						path.MatchRelative().AtParent().AtName("name"),
						path.MatchRelative().AtParent().AtName("description"),
						path.MatchRelative().AtParent().AtName("layout"),
						path.MatchRelative().AtParent().AtName("variables"),
						path.MatchRelative().AtParent().AtName("filters"),
						path.MatchRelative().AtParent().AtName("time_frame"),
					),
					ContentJsonValidator{},
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIf(JSONStringsEqualPlanModifier, "", ""),
				},
				Description: "an option to set the dashboard content from a json file.",
			},
		},
	}
}

type ContentJsonValidator struct{}

func (c ContentJsonValidator) Description(_ context.Context) string {
	return ""
}

func (c ContentJsonValidator) MarkdownDescription(_ context.Context) string {
	return ""
}

func (c ContentJsonValidator) ValidateString(_ context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() {
		return
	}

	err := protojson.Unmarshal([]byte(request.ConfigValue.ValueString()), &dashboards.Dashboard{})
	if err != nil {
		response.Diagnostics.Append(diag.NewErrorDiagnostic("content_json validation failed", fmt.Sprintf("json content is not matching layout schema. got an err while unmarshalling - %s", err)))
	}
}

func JSONStringsEqualPlanModifier(_ context.Context, plan planmodifier.StringRequest, req *stringplanmodifier.RequiresReplaceIfFuncResponse) {
	if JSONStringsEqual(plan.PlanValue.ValueString(), plan.StateValue.ValueString()) {
		req.RequiresReplace = false
	}
	req.RequiresReplace = true
}

func metricFiltersSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"metric": schema.StringAttribute{
					Optional: true,
				},
				"label": schema.StringAttribute{
					Optional: true,
				},
				"operator": filterOperatorSchema(),
			},
		},
		Validators: []validator.List{
			listvalidator.SizeAtLeast(1),
		},
		Optional: true,
	}
}

func filterOperatorSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf("equals", "not_equals"),
				},
				MarkdownDescription: "The type of the operator. Can be one of `equals` or `not_equals`.",
			},
			"selected_values": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "the values to filter by. When the type is `equals`, this field is optional, the filter will match spans with the selected values, and all the values if not set. When the type is `not_equals`, this field is required, and the filter will match spans without the selected values.",
			},
		},
		Validators: []validator.Object{
			filterOperatorValidator{},
		},
		Required: true,
	}
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
	req.ConfigValue.As(ctx, &filter, basetypes.ObjectAsOptions{})
	if filter.Type.ValueString() == "equals" && filter.SelectedValues.IsNull() {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("filter operator validation failed", "when type is `equals`, `selected_values` must be set"))
	}
}

func logsFiltersSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Optional: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"field": schema.StringAttribute{
					Required: true,
				},
				"operator": filterOperatorSchema(),
			},
		},
		Validators: []validator.List{
			listvalidator.SizeAtLeast(1),
		},
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
	req.ConfigValue.As(ctx, &aggregation, basetypes.ObjectAsOptions{})
	if aggregation.Type.ValueString() == "count" && !aggregation.Field.IsNull() {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("logs aggregation validation failed", "when type is `count`, `field` cannot be set"))
	} else if aggregation.Type.ValueString() != "count" && aggregation.Field.IsNull() {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("logs aggregation validation failed", fmt.Sprintf("when type is `%s`, `field` must be set", aggregation.Type.ValueString())))
	}
}

func logsAggregationSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional:   true,
		Attributes: logsAggregationAttributes(),
		Validators: []validator.Object{
			logsAggregationValidator{},
		},
	}
}

func logsAggregationsSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Optional: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: logsAggregationAttributes(),
			Validators: []validator.Object{
				logsAggregationValidator{},
			},
		},
		Validators: []validator.List{
			listvalidator.SizeAtLeast(1),
		},
	}
}

func logsAggregationAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"type": schema.StringAttribute{
			Required: true,
			Validators: []validator.String{
				stringvalidator.OneOf(dashboardValidLogsAggregationTypes...),
			},
			MarkdownDescription: fmt.Sprintf("The type of the aggregation. Can be one of %q", dashboardValidLogsAggregationTypes),
		},
		"field": schema.StringAttribute{
			Optional: true,
		},
	}
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
	request.ConfigValue.As(ctx, &field, basetypes.ObjectAsOptions{})
	if field.Type.ValueString() == "metadata" && !slices.Contains(dashboardValidSpanFieldMetadataFields, field.Value.ValueString()) {
		response.Diagnostics.Append(diag.NewErrorDiagnostic("spans field validation failed", fmt.Sprintf("when type is `metadata`, `value` must be one of %q", dashboardValidSpanFieldMetadataFields)))
	}
}

func spansFieldSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Attributes: spansFieldAttributes(),
		Optional:   true,
		Validators: []validator.Object{
			spansFieldValidator{},
		},
	}
}

func spansFieldsSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		NestedObject: schema.NestedAttributeObject{
			Attributes: spansFieldAttributes(),
			Validators: []validator.Object{
				spansFieldValidator{},
			},
		},
		Optional: true,
	}
}

func spansFieldAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"type": schema.StringAttribute{
			Required: true,
			Validators: []validator.String{
				stringvalidator.OneOf(dashboardValidSpanFieldTypes...),
			},
			MarkdownDescription: fmt.Sprintf("The type of the field. Can be one of %q", dashboardValidSpanFieldTypes),
		},
		"value": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: fmt.Sprintf("The value of the field. When the field type is `metadata`, can be one of %q", dashboardValidSpanFieldMetadataFields),
		},
	}
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
	request.ConfigValue.As(ctx, &aggregation, basetypes.ObjectAsOptions{})
	if aggregation.Type.ValueString() == "metrics" && !slices.Contains(dashboardValidSpansAggregationMetricAggregationTypes, aggregation.AggregationType.ValueString()) {
		response.Diagnostics.Append(diag.NewErrorDiagnostic("spans aggregation validation failed", fmt.Sprintf("when type is `metrics`, `aggregation_type` must be one of %q", dashboardValidSpansAggregationMetricAggregationTypes)))
	}
	if aggregation.Type.ValueString() == "dimension" && !slices.Contains(dashboardValidSpansAggregationDimensionAggregationTypes, aggregation.AggregationType.ValueString()) {
		response.Diagnostics.Append(diag.NewErrorDiagnostic("spans aggregation validation failed", fmt.Sprintf("when type is `dimension`, `aggregation_type` must be one of %q", dashboardValidSpansAggregationDimensionAggregationTypes)))
	}
}

func spansAggregationSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Attributes: spansAggregationAttributes(),
		Optional:   true,
		Validators: []validator.Object{
			spansAggregationValidator{},
		},
	}
}

func spansAggregationsSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		NestedObject: schema.NestedAttributeObject{
			Attributes: spansAggregationAttributes(),
			Validators: []validator.Object{
				spansAggregationValidator{},
			},
		},
		Optional: true,
	}
}
func spansAggregationAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"type": schema.StringAttribute{
			Required: true,
			Validators: []validator.String{
				stringvalidator.OneOf(dashboardValidSpanAggregationTypes...),
			},
			MarkdownDescription: fmt.Sprintf("Can be one of %q", dashboardValidSpanAggregationTypes),
		},
		"aggregation_type": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: fmt.Sprintf("The type of the aggregation. When the aggregation type is `metrics`, can be one of %q. When When the aggregation type is `dimension`, can be one of %q.", dashboardValidSpansAggregationMetricAggregationTypes, dashboardValidSpansAggregationDimensionAggregationTypes),
		},
		"field": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: fmt.Sprintf("The field to aggregate on. When the aggregation type is `metrics`, can be one of %q. When When the aggregation type is `dimension`, can be one of %q.", dashboardValidSpansAggregationMetricFields, dashboardValidSpansAggregationDimensionFields),
		},
	}
}

func spansFilterSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"field": schema.SingleNestedAttribute{
					Attributes: spansFieldAttributes(),
					Required:   true,
				},
				"operator": filterOperatorSchema(),
			},
		},
		Optional: true,
	}
}

func (r DashboardResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	jsm := &jsonpb.Marshaler{}
	var plan DashboardResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	dashboard, diags := extractDashboard(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	dashboardStr, _ := jsm.MarshalToString(dashboard)
	log.Printf("[INFO] Creating new Dashboard: %#v", dashboardStr)
	createDashboardReq := &dashboards.CreateDashboardRequest{
		Dashboard: dashboard,
	}
	_, err := r.client.CreateDashboard(ctx, createDashboardReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		resp.Diagnostics.AddError(
			"Error creating Dashboard",
			"Could not create Dashboard, unexpected error: "+err.Error(),
		)
		return
	}

	getDashboardReq := &dashboards.GetDashboardRequest{
		DashboardId: dashboard.GetId(),
	}
	getDashboardResp, err := r.client.GetDashboard(ctx, getDashboardReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		resp.Diagnostics.AddError(
			"Error getting Dashboard",
			"Could not create Dashboard, unexpected error: "+err.Error(),
		)
		return
	}
	createDashboardRespStr, _ := jsm.MarshalToString(getDashboardResp.GetDashboard())
	log.Printf("[INFO] Submitted new Dashboard: %#v", createDashboardRespStr)

	flattenedDashboard, diags := flattenDashboard(ctx, plan, getDashboardResp.GetDashboard())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	plan = *flattenedDashboard

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func extractDashboard(ctx context.Context, plan DashboardResourceModel) (*dashboards.Dashboard, diag.Diagnostics) {
	if !plan.ContentJson.IsNull() {
		dashboard := new(dashboards.Dashboard)
		if err := protojson.Unmarshal([]byte(plan.ContentJson.ValueString()), dashboard); err != nil {
			return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error unmarshalling dashboard content json", err.Error())}
		}
		return dashboard, nil
	}

	layout, diags := expandDashboardLayout(ctx, plan.Layout)
	if diags.HasError() {
		return nil, diags
	}

	variables, diags := expandDashboardVariables(ctx, plan.Variables)
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := expandDashboardFilters(ctx, plan.Filters)
	if diags.HasError() {
		return nil, diags
	}

	id := wrapperspb.String(expandDashboardUUID(plan.ID).GetValue())

	dashboard := &dashboards.Dashboard{
		Id:          id,
		Name:        typeStringToWrapperspbString(plan.Name),
		Description: typeStringToWrapperspbString(plan.Description),
		Layout:      layout,
		Variables:   variables,
		Filters:     filters,
	}

	dashboard, dg := expandDashboardTimeFrame(dashboard, plan.TimeFrame)
	if diags.HasError() {
		return nil, diag.Diagnostics{dg}
	}

	return dashboard, nil
}

func expandDashboardTimeFrame(dashboard *dashboards.Dashboard, timeFrame *DashboardTimeFrameModel) (*dashboards.Dashboard, diag.Diagnostic) {
	if timeFrame == nil {
		return dashboard, nil
	}
	var dg diag.Diagnostic
	switch {
	case timeFrame.Relative != nil:
		dashboard.TimeFrame, dg = expandRelativeDashboardTimeFrame(timeFrame.Relative)
	case timeFrame.Absolute != nil:
		dashboard.TimeFrame, dg = expandAbsoluteeDashboardTimeFrame(timeFrame.Absolute)
	default:
		dg = diag.NewErrorDiagnostic("Error Expand Time Frame", "Dashboard TimeFrame must be either Relative or Absolutee")
	}
	return dashboard, dg
}

func expandDashboardLayout(ctx context.Context, layout *DashboardLayoutModel) (*dashboards.Layout, diag.Diagnostics) {
	sections, diags := expandDashboardSections(ctx, layout.Sections)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboards.Layout{
		Sections: sections,
	}, nil
}

func expandDashboardSections(ctx context.Context, sections types.List) ([]*dashboards.Section, diag.Diagnostics) {
	var diags diag.Diagnostics
	var sectionsObjects []types.Object
	var expandedSections []*dashboards.Section
	sections.ElementsAs(ctx, &sectionsObjects, true)

	for _, so := range sectionsObjects {
		var section SectionModel
		if dg := so.As(ctx, &section, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedSection, expandSectionDiags := expandSection(ctx, section)
		if expandSectionDiags.HasError() {
			diags.Append(expandSectionDiags...)
			continue
		}
		expandedSections = append(expandedSections, expandedSection)
	}

	return expandedSections, diags
}

func expandSection(ctx context.Context, section SectionModel) (*dashboards.Section, diag.Diagnostics) {
	id := expandDashboardUUID(section.ID)
	rows, diags := expandDashboardRows(ctx, section.Rows)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Section{
		Id:   id,
		Rows: rows,
	}, nil
}

func expandDashboardRows(ctx context.Context, rows types.List) ([]*dashboards.Row, diag.Diagnostics) {
	var diags diag.Diagnostics
	var rowsObjects []types.Object
	var expandedRows []*dashboards.Row
	rows.ElementsAs(ctx, &rowsObjects, true)

	for _, ro := range rowsObjects {
		var row RowModel
		if dg := ro.As(ctx, &row, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedRow, expandDiags := expandRow(ctx, row)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedRows = append(expandedRows, expandedRow)
	}

	return expandedRows, diags
}

func expandRow(ctx context.Context, row RowModel) (*dashboards.Row, diag.Diagnostics) {
	id := expandDashboardUUID(row.ID)
	appearance := &dashboards.Row_Appearance{
		Height: wrapperspb.Int32(int32(row.Height.ValueInt64())),
	}
	widgets, diags := expandDashboardWidgets(ctx, row.Widgets)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Row{
		Id:         id,
		Appearance: appearance,
		Widgets:    widgets,
	}, nil
}

func expandDashboardWidgets(ctx context.Context, widgets types.List) ([]*dashboards.Widget, diag.Diagnostics) {
	var diags diag.Diagnostics
	var widgetsObjects []types.Object
	var expandedWidgets []*dashboards.Widget
	widgets.ElementsAs(ctx, &widgetsObjects, true)

	for _, wo := range widgetsObjects {
		var widget WidgetModel
		if dg := wo.As(ctx, &widget, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedWidget, expandDiags := expandWidget(ctx, widget)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedWidgets = append(expandedWidgets, expandedWidget)
	}

	return expandedWidgets, diags
}

func expandWidget(ctx context.Context, widget WidgetModel) (*dashboards.Widget, diag.Diagnostics) {
	id := expandDashboardUUID(widget.ID)
	title := typeStringToWrapperspbString(widget.Title)
	description := typeStringToWrapperspbString(widget.Description)
	appearance := &dashboards.Widget_Appearance{
		Width: wrapperspb.Int32(int32(widget.Width.ValueInt64())),
	}
	definition, diags := expandWidgetDefinition(ctx, widget.Definition)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Widget{
		Id:          id,
		Title:       title,
		Description: description,
		Appearance:  appearance,
		Definition:  definition,
	}, nil
}

func expandWidgetDefinition(ctx context.Context, definition *WidgetDefinitionModel) (*dashboards.Widget_Definition, diag.Diagnostics) {
	switch {
	case definition.PieChart != nil:
		return expandPieChart(ctx, definition.PieChart)
	case definition.Gauge != nil:
		return expandGauge(ctx, definition.Gauge)
	case definition.LineChart != nil:
		return expandLineChart(ctx, definition.LineChart)
	case definition.DataTable != nil:
		return expandDataTable(ctx, definition.DataTable)
	case definition.BarChart != nil:
		return expandBarChart(ctx, definition.BarChart)
	default:
		return nil, diag.Diagnostics{
			diag.NewErrorDiagnostic(
				"Extract Dashboard Widget Definition Error",
				fmt.Sprintf("Unknown widget definition type: %#v", definition),
			),
		}
	}
}

func expandPieChart(ctx context.Context, pieChart *PieChartModel) (*dashboards.Widget_Definition, diag.Diagnostics) {
	var diags diag.Diagnostics

	query, diags := expandDashboardQuery(ctx, pieChart.Query)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Widget_Definition{
		Value: &dashboards.Widget_Definition_PieChart{
			PieChart: &dashboards.PieChart{
				Query:              query,
				MaxSlicesPerChart:  typeInt64ToWrappedInt32(pieChart.MaxSlicesPerChart),
				MinSlicePercentage: typeInt64ToWrappedInt32(pieChart.MinSlicePercentage),
				StackDefinition:    expandPieChartStackDefinition(pieChart.StackDefinition),
				LabelDefinition:    expandLabelDefinition(pieChart.LabelDefinition),
				ShowLegend:         typeBoolToWrapperspbBool(pieChart.ShowLegend),
				GroupNameTemplate:  typeStringToWrapperspbString(pieChart.GroupNameTemplate),
				Unit:               dashboardSchemaToProtoUnit[pieChart.Unit.ValueString()],
			},
		},
	}, nil
}

func expandPieChartStackDefinition(stackDefinition *PieChartStackDefinitionModel) *dashboards.PieChart_StackDefinition {
	if stackDefinition == nil {
		return nil
	}

	return &dashboards.PieChart_StackDefinition{
		MaxSlicesPerStack: typeInt64ToWrappedInt32(stackDefinition.MaxSlicesPerStack),
		StackNameTemplate: typeStringToWrapperspbString(stackDefinition.StackNameTemplate),
	}
}

func expandBarChartStackDefinition(stackDefinition *BarChartStackDefinitionModel) *dashboards.BarChart_StackDefinition {
	if stackDefinition == nil {
		return nil
	}

	return &dashboards.BarChart_StackDefinition{
		MaxSlicesPerBar:   typeInt64ToWrappedInt32(stackDefinition.MaxSlicesPerBar),
		StackNameTemplate: typeStringToWrapperspbString(stackDefinition.StackNameTemplate),
	}
}

func expandLabelDefinition(labelDefinition *LabelDefinitionModel) *dashboards.PieChart_LabelDefinition {
	if labelDefinition == nil {
		return nil
	}

	return &dashboards.PieChart_LabelDefinition{
		LabelSource:    dashboardSchemaToProtoPieChartLabelSource[labelDefinition.LabelSource.ValueString()],
		IsVisible:      typeBoolToWrapperspbBool(labelDefinition.IsVisible),
		ShowName:       typeBoolToWrapperspbBool(labelDefinition.ShowName),
		ShowValue:      typeBoolToWrapperspbBool(labelDefinition.ShowValue),
		ShowPercentage: typeBoolToWrapperspbBool(labelDefinition.ShowPercentage),
	}
}

func expandGauge(ctx context.Context, gauge *GaugeModel) (*dashboards.Widget_Definition, diag.Diagnostics) {
	var diags diag.Diagnostics

	query, diags := expandGaugeQuery(ctx, gauge.Query)
	if diags.HasError() {
		return nil, diags
	}

	thresholds, diags := expandGaugeThresholds(ctx, gauge.Thresholds)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Widget_Definition{
		Value: &dashboards.Widget_Definition_Gauge{
			Gauge: &dashboards.Gauge{
				Query:        query,
				Min:          typeFloat64ToWrapperspbDouble(gauge.Min),
				Max:          typeFloat64ToWrapperspbDouble(gauge.Max),
				ShowInnerArc: typeBoolToWrapperspbBool(gauge.ShowInnerArc),
				ShowOuterArc: typeBoolToWrapperspbBool(gauge.ShowOuterArc),
				Unit:         dashboardSchemaToProtoGaugeUnit[gauge.Unit.ValueString()],
				Thresholds:   thresholds,
			},
		},
	}, nil
}

func expandGaugeThresholds(ctx context.Context, gaugeThresholds types.List) ([]*dashboards.Gauge_Threshold, diag.Diagnostics) {
	var diags diag.Diagnostics
	var gaugeThresholdsObjects []types.Object
	var expandedGaugeThresholds []*dashboards.Gauge_Threshold
	gaugeThresholds.ElementsAs(ctx, &gaugeThresholdsObjects, true)

	for _, gto := range gaugeThresholdsObjects {
		var gaugeThreshold GaugeThresholdModel
		if dg := gto.As(ctx, &gaugeThreshold, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedGaugeThreshold := expandGaugeThreshold(&gaugeThreshold)
		expandedGaugeThresholds = append(expandedGaugeThresholds, expandedGaugeThreshold)
	}

	return expandedGaugeThresholds, diags
}

func expandGaugeThreshold(gaugeThresholds *GaugeThresholdModel) *dashboards.Gauge_Threshold {
	if gaugeThresholds == nil {
		return nil
	}
	return &dashboards.Gauge_Threshold{
		From:  typeFloat64ToWrapperspbDouble(gaugeThresholds.From),
		Color: typeStringToWrapperspbString(gaugeThresholds.Color),
	}
}

func expandGaugeQuery(ctx context.Context, gaugeQuery *GaugeQueryModel) (*dashboards.Gauge_Query, diag.Diagnostics) {
	switch {
	case gaugeQuery.Metrics != nil:
		metricQuery, diags := expandGaugeQueryMetrics(ctx, gaugeQuery.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.Gauge_Query{
			Value: &dashboards.Gauge_Query_Metrics{
				Metrics: metricQuery,
			},
		}, nil
	case gaugeQuery.Logs != nil:
		logQuery, diags := expandGaugeQueryLogs(ctx, gaugeQuery.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.Gauge_Query{
			Value: &dashboards.Gauge_Query_Logs{
				Logs: logQuery,
			},
		}, nil
	case gaugeQuery.Spans != nil:
		spanQuery, diags := expandGaugeQuerySpans(ctx, gaugeQuery.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.Gauge_Query{
			Value: &dashboards.Gauge_Query_Spans{
				Spans: spanQuery,
			},
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Extract Gauge Query Error", fmt.Sprintf("Unknown gauge query type %#v", gaugeQuery))}
	}
}

func expandGaugeQuerySpans(ctx context.Context, gaugeQuerySpans *GaugeQuerySpansModel) (*dashboards.Gauge_SpansQuery, diag.Diagnostics) {
	if gaugeQuerySpans == nil {
		return nil, nil
	}
	filters, diags := expandSpansFilters(ctx, gaugeQuerySpans.Filters)
	if diags.HasError() {
		return nil, diags
	}

	spansAggregation, dg := expandSpansAggregation(gaugeQuerySpans.SpansAggregation)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &dashboards.Gauge_SpansQuery{
		LuceneQuery:      expandLuceneQuery(gaugeQuerySpans.LuceneQuery),
		SpansAggregation: spansAggregation,
		Filters:          filters,
		Aggregation:      dashboardSchemaToProtoGaugeAggregation[gaugeQuerySpans.Aggregation.ValueString()],
	}, nil
}

func expandSpansAggregations(ctx context.Context, aggregations types.List) ([]*dashboards.SpansAggregation, diag.Diagnostics) {
	var diags diag.Diagnostics
	var aggregationsObjects []types.Object
	var expandedAggregations []*dashboards.SpansAggregation
	aggregations.ElementsAs(ctx, &aggregationsObjects, true)

	for _, ao := range aggregationsObjects {
		var aggregation SpansAggregationModel
		if dg := ao.As(ctx, &aggregation, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedAggregation, expandDiag := expandSpansAggregation(&aggregation)
		if expandDiag != nil {
			diags.Append(expandDiag)
			continue
		}
		expandedAggregations = append(expandedAggregations, expandedAggregation)
	}

	return expandedAggregations, diags
}

func expandSpansAggregation(spansAggregation *SpansAggregationModel) (*dashboards.SpansAggregation, diag.Diagnostic) {
	if spansAggregation == nil {
		return nil, nil
	}

	switch spansAggregation.Type.ValueString() {
	case "metric":
		return &dashboards.SpansAggregation{
			Aggregation: &dashboards.SpansAggregation_MetricAggregation_{
				MetricAggregation: &dashboards.SpansAggregation_MetricAggregation{
					MetricField:     dashboardSchemaToProtoSpansAggregationMetricField[spansAggregation.Field.ValueString()],
					AggregationType: dashboardSchemaToProtoSpansAggregationMetricAggregationType[spansAggregation.AggregationType.ValueString()],
				},
			},
		}, nil
	case "dimension":
		return &dashboards.SpansAggregation{
			Aggregation: &dashboards.SpansAggregation_DimensionAggregation_{
				DimensionAggregation: &dashboards.SpansAggregation_DimensionAggregation{
					DimensionField:  dashboardProtoToSchemaSpansAggregationDimensionField[spansAggregation.Field.ValueString()],
					AggregationType: dashboardSchemaToProtoSpansAggregationDimensionAggregationType[spansAggregation.AggregationType.ValueString()],
				},
			},
		}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Extract Spans Aggregation Error", fmt.Sprintf("Unknown spans aggregation type %#v", spansAggregation))
	}
}

func expandSpansFilters(ctx context.Context, spansFilters types.List) ([]*dashboards.Filter_SpansFilter, diag.Diagnostics) {
	var diags diag.Diagnostics
	var spansFiltersObjects []types.Object
	var expandedSpansFilters []*dashboards.Filter_SpansFilter
	spansFilters.ElementsAs(ctx, &spansFiltersObjects, true)

	for _, sfo := range spansFiltersObjects {
		var spansFilter SpansFilterModel
		if dg := sfo.As(ctx, &spansFilter, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedSpansFilter, expandDiags := expandSpansFilter(ctx, spansFilter)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedSpansFilters = append(expandedSpansFilters, expandedSpansFilter)
	}

	return expandedSpansFilters, diags
}

func expandSpansFilter(ctx context.Context, spansFilter SpansFilterModel) (*dashboards.Filter_SpansFilter, diag.Diagnostics) {
	operator, diags := expandFilterOperator(ctx, spansFilter.Operator)
	if diags.HasError() {
		return nil, diags
	}

	field, dg := expandSpansField(spansFilter.Field)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &dashboards.Filter_SpansFilter{
		Field:    field,
		Operator: operator,
	}, nil
}

func expandSpansField(spansFilterField *SpansFieldModel) (*dashboards.SpanField, diag.Diagnostic) {
	if spansFilterField == nil {
		return nil, nil
	}

	switch spansFilterField.Type.ValueString() {
	case "metadata":
		return &dashboards.SpanField{
			Value: &dashboards.SpanField_MetadataField_{
				MetadataField: dashboardSchemaToProtoSpanFieldMetadataField[spansFilterField.Value.ValueString()],
			},
		}, nil
	case "tag":
		return &dashboards.SpanField{
			Value: &dashboards.SpanField_TagField{
				TagField: typeStringToWrapperspbString(spansFilterField.Value),
			},
		}, nil
	case "process_tag":
		return &dashboards.SpanField{
			Value: &dashboards.SpanField_ProcessTagField{
				ProcessTagField: typeStringToWrapperspbString(spansFilterField.Value),
			},
		}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Extract Spans Filter Field Error", fmt.Sprintf("Unknown spans filter field type %s", spansFilterField.Type.ValueString()))
	}
}

func expandGaugeQueryMetrics(ctx context.Context, gaugeQueryMetrics *GaugeQueryMetricsModel) (*dashboards.Gauge_MetricsQuery, diag.Diagnostics) {
	filters, diags := expandMetricsFilters(ctx, gaugeQueryMetrics.Filters)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Gauge_MetricsQuery{
		PromqlQuery: expandPromqlQuery(gaugeQueryMetrics.PromqlQuery),
		Aggregation: dashboardSchemaToProtoGaugeAggregation[gaugeQueryMetrics.Aggregation.ValueString()],
		Filters:     filters,
	}, nil
}

func expandMetricsFilters(ctx context.Context, metricFilters types.List) ([]*dashboards.Filter_MetricsFilter, diag.Diagnostics) {
	var diags diag.Diagnostics
	var metricFiltersObjects []types.Object
	var expandedMetricFilters []*dashboards.Filter_MetricsFilter
	metricFilters.ElementsAs(ctx, &metricFiltersObjects, true)

	for _, mfo := range metricFiltersObjects {
		var metricsFilter MetricsFilterModel
		if dg := mfo.As(ctx, &metricsFilter, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedMetricFilter, expandDiags := expandMetricFilter(ctx, metricsFilter)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedMetricFilters = append(expandedMetricFilters, expandedMetricFilter)
	}

	return expandedMetricFilters, diags
}

func expandMetricFilter(ctx context.Context, metricFilter MetricsFilterModel) (*dashboards.Filter_MetricsFilter, diag.Diagnostics) {
	operator, diags := expandFilterOperator(ctx, metricFilter.Operator)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Filter_MetricsFilter{
		Metric:   typeStringToWrapperspbString(metricFilter.Metric),
		Label:    typeStringToWrapperspbString(metricFilter.Label),
		Operator: operator,
	}, nil
}

func expandFilterOperator(ctx context.Context, operator *FilterOperatorModel) (*dashboards.Filter_Operator, diag.Diagnostics) {
	if operator == nil {
		return nil, nil
	}

	selectedValues, diags := typeStringSliceToWrappedStringSlice(ctx, operator.SelectedValues.Elements())
	if diags.HasError() {
		return nil, diags
	}

	switch operator.Type.ValueString() {
	case "equals":
		filterOperator := &dashboards.Filter_Operator{
			Value: &dashboards.Filter_Operator_Equals{
				Equals: &dashboards.Filter_Equals{
					Selection: &dashboards.Filter_Equals_Selection{},
				},
			},
		}
		if len(selectedValues) != 0 {
			filterOperator.GetEquals().Selection.Value = &dashboards.Filter_Equals_Selection_List{
				List: &dashboards.Filter_Equals_Selection_ListSelection{
					Values: selectedValues,
				},
			}
		} else {
			filterOperator.GetEquals().Selection.Value = &dashboards.Filter_Equals_Selection_All{
				All: &dashboards.Filter_Equals_Selection_AllSelection{},
			}
		}
		return filterOperator, nil
	case "not_equals":
		return &dashboards.Filter_Operator{
			Value: &dashboards.Filter_Operator_NotEquals{
				NotEquals: &dashboards.Filter_NotEquals{
					Selection: &dashboards.Filter_NotEquals_Selection{
						Value: &dashboards.Filter_NotEquals_Selection_List{
							List: &dashboards.Filter_NotEquals_Selection_ListSelection{
								Values: selectedValues,
							},
						},
					},
				},
			},
		}, nil
	default:
		diags.Append(diag.NewErrorDiagnostic(
			"Error expand filter operator",
			fmt.Sprintf("unknown filter operator type %s", operator.Type.ValueString())))
		return nil, diags
	}
}

func expandPromqlQuery(promqlQuery types.String) *dashboards.PromQlQuery {
	if promqlQuery.IsNull() || promqlQuery.IsUnknown() {
		return nil
	}

	return &dashboards.PromQlQuery{
		Value: wrapperspb.String(promqlQuery.ValueString()),
	}
}

func expandGaugeQueryLogs(ctx context.Context, gaugeQueryLogs *GaugeQueryLogsModel) (*dashboards.Gauge_LogsQuery, diag.Diagnostics) {
	logsAggregation, dg := expandLogsAggregation(gaugeQueryLogs.LogsAggregation)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	filters, diags := expandLogsFilters(ctx, gaugeQueryLogs.Filters)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Gauge_LogsQuery{
		LuceneQuery:     expandLuceneQuery(gaugeQueryLogs.LuceneQuery),
		LogsAggregation: logsAggregation,
		Filters:         filters,
		Aggregation:     dashboardSchemaToProtoGaugeAggregation[gaugeQueryLogs.Aggregation.ValueString()],
	}, nil
}

func expandLuceneQuery(luceneQuery types.String) *dashboards.LuceneQuery {
	if luceneQuery.IsNull() || luceneQuery.IsUnknown() {
		return nil
	}
	return &dashboards.LuceneQuery{
		Value: wrapperspb.String(luceneQuery.ValueString()),
	}
}

func expandLogsAggregations(ctx context.Context, logsAggregations types.List) ([]*dashboards.LogsAggregation, diag.Diagnostics) {
	var diags diag.Diagnostics
	var logsAggregationsObjects []types.Object
	var expandedLogsAggregations []*dashboards.LogsAggregation
	logsAggregations.ElementsAs(ctx, &logsAggregationsObjects, true)

	for _, qdo := range logsAggregationsObjects {
		var aggregation LogsAggregationModel
		if dg := qdo.As(ctx, &aggregation, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedLogsAggregation, expandDiags := expandLogsAggregation(&aggregation)
		if expandDiags != nil {
			diags.Append(expandDiags)
			continue
		}
		expandedLogsAggregations = append(expandedLogsAggregations, expandedLogsAggregation)
	}

	return expandedLogsAggregations, diags
}

func expandLogsAggregation(logsAggregation *LogsAggregationModel) (*dashboards.LogsAggregation, diag.Diagnostic) {
	if logsAggregation == nil {
		return nil, nil
	}
	switch logsAggregation.Type.ValueString() {
	case "count":
		return &dashboards.LogsAggregation{
			Value: &dashboards.LogsAggregation_Count_{
				Count: &dashboards.LogsAggregation_Count{},
			},
		}, nil
	case "count_distinct":
		return &dashboards.LogsAggregation{
			Value: &dashboards.LogsAggregation_CountDistinct_{
				CountDistinct: &dashboards.LogsAggregation_CountDistinct{
					Field: typeStringToWrapperspbString(logsAggregation.Field),
				},
			},
		}, nil
	case "sum":
		return &dashboards.LogsAggregation{
			Value: &dashboards.LogsAggregation_Sum_{
				Sum: &dashboards.LogsAggregation_Sum{
					Field: typeStringToWrapperspbString(logsAggregation.Field),
				},
			},
		}, nil
	case "avg":
		return &dashboards.LogsAggregation{
			Value: &dashboards.LogsAggregation_Average_{
				Average: &dashboards.LogsAggregation_Average{
					Field: typeStringToWrapperspbString(logsAggregation.Field),
				},
			},
		}, nil
	case "min":
		return &dashboards.LogsAggregation{
			Value: &dashboards.LogsAggregation_Min_{
				Min: &dashboards.LogsAggregation_Min{
					Field: typeStringToWrapperspbString(logsAggregation.Field),
				},
			},
		}, nil
	case "max":
		return &dashboards.LogsAggregation{
			Value: &dashboards.LogsAggregation_Max_{
				Max: &dashboards.LogsAggregation_Max{
					Field: typeStringToWrapperspbString(logsAggregation.Field),
				},
			},
		}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Error expand logs aggregation", fmt.Sprintf("unknown logs aggregation type %s", logsAggregation.Type.ValueString()))
	}
}

func expandLogsFilters(ctx context.Context, logsFilters types.List) ([]*dashboards.Filter_LogsFilter, diag.Diagnostics) {
	var diags diag.Diagnostics
	var filtersObjects []types.Object
	var expandedFilters []*dashboards.Filter_LogsFilter
	logsFilters.ElementsAs(ctx, &filtersObjects, true)

	for _, fo := range filtersObjects {
		var filter LogsFilterModel
		if dg := fo.As(ctx, &filter, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedFilter, expandDiags := expandLogsFilter(ctx, filter)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedFilters = append(expandedFilters, expandedFilter)
	}

	return expandedFilters, diags
}

func expandLogsFilter(ctx context.Context, logsFilter LogsFilterModel) (*dashboards.Filter_LogsFilter, diag.Diagnostics) {
	operator, diags := expandFilterOperator(ctx, logsFilter.Operator)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Filter_LogsFilter{
		Field:    typeStringToWrapperspbString(logsFilter.Field),
		Operator: operator,
	}, nil
}

func expandBarChart(ctx context.Context, chart *BarChartModel) (*dashboards.Widget_Definition, diag.Diagnostics) {
	var diags diag.Diagnostics

	query, diags := expandBarChartQuery(ctx, chart.Query)
	if diags.HasError() {
		return nil, diags
	}

	xaxis, dg := expandXAis(chart.XAxis)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &dashboards.Widget_Definition{
		Value: &dashboards.Widget_Definition_BarChart{
			BarChart: &dashboards.BarChart{
				Query:             query,
				MaxBarsPerChart:   typeInt64ToWrappedInt32(chart.MaxBarsPerChart),
				GroupNameTemplate: typeStringToWrapperspbString(chart.GroupNameTemplate),
				StackDefinition:   expandBarChartStackDefinition(chart.StackDefinition),
				ScaleType:         dashboardSchemaToProtoScaleType[chart.ScaleType.ValueString()],
				ColorsBy:          expandColorsBy(chart.ColorsBy),
				XAxis:             xaxis,
				Unit:              dashboardSchemaToProtoUnit[chart.Unit.ValueString()],
			},
		},
	}, nil
}

func expandColorsBy(colorsBy types.String) *dashboards.BarChart_ColorsBy {
	switch colorsBy.ValueString() {
	case "stack":
		return &dashboards.BarChart_ColorsBy{
			Value: &dashboards.BarChart_ColorsBy_Stack{
				Stack: &dashboards.BarChart_ColorsBy_ColorsByStack{},
			},
		}
	case "group_by":
		return &dashboards.BarChart_ColorsBy{
			Value: &dashboards.BarChart_ColorsBy_GroupBy{
				GroupBy: &dashboards.BarChart_ColorsBy_ColorsByGroupBy{},
			},
		}
	default:
		return nil
	}
}

func expandXAis(xaxis *BarChartXAxisModel) (*dashboards.BarChart_XAxis, diag.Diagnostic) {
	if xaxis == nil {
		return nil, nil
	}

	switch xaxis.Type.ValueString() {
	case "time":
		duration, err := time.ParseDuration(xaxis.Interval.ValueString())
		if err != nil {
			return nil, diag.NewErrorDiagnostic("Error expand bar chart x axis", err.Error())
		}
		return &dashboards.BarChart_XAxis{
			Type: &dashboards.BarChart_XAxis_Time{
				Time: &dashboards.BarChart_XAxis_XAxisByTime{
					Interval:         durationpb.New(duration),
					BucketsPresented: typeInt64ToWrappedInt32(xaxis.BucketsPresented),
				},
			},
		}, nil
	case "value":
		return &dashboards.BarChart_XAxis{
			Type: &dashboards.BarChart_XAxis_Value{
				Value: &dashboards.BarChart_XAxis_XAxisByValue{},
			},
		}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Error expand bar chart x axis", fmt.Sprintf("unknown bar chart x axis type %s", xaxis.Type.ValueString()))
	}
}
func expandBarChartQuery(ctx context.Context, query *BarChartQueryModel) (*dashboards.BarChart_Query, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}
	switch {
	case query.Logs != nil:
		logsQuery, diags := expandBarChartLogsQuery(ctx, query.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.BarChart_Query{
			Value: &dashboards.BarChart_Query_Logs{
				Logs: logsQuery,
			},
		}, nil
	case query.Metrics != nil:
		metricsQuery, diags := expandBarChartMetricsQuery(ctx, query.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.BarChart_Query{
			Value: &dashboards.BarChart_Query_Metrics{
				Metrics: metricsQuery,
			},
		}, nil
	case query.Spans != nil:
		spansQuery, diags := expandBarChartSpansQuery(ctx, query.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.BarChart_Query{
			Value: &dashboards.BarChart_Query_Spans{
				Spans: spansQuery,
			},
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error expand bar chart query", "unknown bar chart query type")}
	}
}

func expandBarChartLogsQuery(ctx context.Context, barChartQueryLogs *BarChartQueryLogsModel) (*dashboards.BarChart_LogsQuery, diag.Diagnostics) {
	if barChartQueryLogs == nil {
		return nil, nil
	}

	aggregation, dg := expandLogsAggregation(barChartQueryLogs.Aggregation)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	filters, diags := expandLogsFilters(ctx, barChartQueryLogs.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := typeStringSliceToWrappedStringSlice(ctx, barChartQueryLogs.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.BarChart_LogsQuery{
		LuceneQuery:      expandLuceneQuery(barChartQueryLogs.LuceneQuery),
		Aggregation:      aggregation,
		Filters:          filters,
		GroupNames:       groupNames,
		StackedGroupName: typeStringToWrapperspbString(barChartQueryLogs.StackedGroupName),
	}, nil
}

func expandBarChartMetricsQuery(ctx context.Context, barChartQueryMetrics *BarChartQueryMetricsModel) (*dashboards.BarChart_MetricsQuery, diag.Diagnostics) {
	if barChartQueryMetrics == nil {
		return nil, nil
	}

	filters, diags := expandMetricsFilters(ctx, barChartQueryMetrics.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := typeStringSliceToWrappedStringSlice(ctx, barChartQueryMetrics.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.BarChart_MetricsQuery{
		PromqlQuery:      expandPromqlQuery(barChartQueryMetrics.PromqlQuery),
		Filters:          filters,
		GroupNames:       groupNames,
		StackedGroupName: typeStringToWrapperspbString(barChartQueryMetrics.StackedGroupName),
	}, nil
}

func expandBarChartSpansQuery(ctx context.Context, barChartQuerySpans *BarChartQuerySpansModel) (*dashboards.BarChart_SpansQuery, diag.Diagnostics) {
	if barChartQuerySpans == nil {
		return nil, nil
	}

	aggregation, dg := expandSpansAggregation(barChartQuerySpans.Aggregation)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	filters, diags := expandSpansFilters(ctx, barChartQuerySpans.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := expandSpansFields(ctx, barChartQuerySpans.GroupNames)
	if diags.HasError() {
		return nil, diags
	}

	expandedFilter, dg := expandSpansField(barChartQuerySpans.StackedGroupName)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &dashboards.BarChart_SpansQuery{
		LuceneQuery:      expandLuceneQuery(barChartQuerySpans.LuceneQuery),
		Aggregation:      aggregation,
		Filters:          filters,
		GroupNames:       groupNames,
		StackedGroupName: expandedFilter,
	}, nil
}

func expandSpansFields(ctx context.Context, spanFields types.List) ([]*dashboards.SpanField, diag.Diagnostics) {
	var diags diag.Diagnostics
	var spanFieldsObjects []types.Object
	var expandedSpanFields []*dashboards.SpanField
	spanFields.ElementsAs(ctx, &spanFieldsObjects, true)

	for _, sfo := range spanFieldsObjects {
		var spansField SpansFieldModel
		if dg := sfo.As(ctx, &spansField, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedSpanField, expandDiag := expandSpansField(&spansField)
		if expandDiag != nil {
			diags.Append(expandDiag)
			continue
		}
		expandedSpanFields = append(expandedSpanFields, expandedSpanField)
	}

	return expandedSpanFields, diags
}

func expandDataTable(ctx context.Context, table *DataTableModel) (*dashboards.Widget_Definition, diag.Diagnostics) {
	query, diags := expandDataTableQuery(ctx, table.Query)
	if diags.HasError() {
		return nil, diags
	}

	columns, diags := expandDataTableColumns(ctx, table.Columns)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Widget_Definition{
		Value: &dashboards.Widget_Definition_DataTable{
			DataTable: &dashboards.DataTable{
				Query:          query,
				ResultsPerPage: typeInt64ToWrappedInt32(table.ResultsPerPage),
				RowStyle:       dashboardRowStyleSchemaToProto[table.RowStyle.ValueString()],
				Columns:        columns,
				OrderBy:        expandOrderBy(table.OrderBy),
			},
		},
	}, nil
}

func expandDataTableQuery(ctx context.Context, dataTableQuery *DataTableQueryModel) (*dashboards.DataTable_Query, diag.Diagnostics) {
	if dataTableQuery == nil {
		return nil, nil
	}
	switch {
	case dataTableQuery.Metrics != nil:
		metrics, diags := expandDataTableMetricsQuery(ctx, dataTableQuery.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.DataTable_Query{
			Value: metrics,
		}, nil
	case dataTableQuery.Logs != nil:
		logs, diags := expandDataTableLogsQuery(ctx, dataTableQuery.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.DataTable_Query{
			Value: logs,
		}, nil
	case dataTableQuery.Spans != nil:
		spans, diags := expandDataTableSpansQuery(ctx, dataTableQuery.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.DataTable_Query{
			Value: spans,
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand DataTable Query", "unknown data table query type")}
	}
}

func expandDataTableMetricsQuery(ctx context.Context, dataTableQueryMetric *DataTableQueryMetricsModel) (*dashboards.DataTable_Query_Metrics, diag.Diagnostics) {
	if dataTableQueryMetric == nil {
		return nil, nil
	}

	filters, diags := expandMetricsFilters(ctx, dataTableQueryMetric.Filters)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.DataTable_Query_Metrics{
		Metrics: &dashboards.DataTable_MetricsQuery{
			PromqlQuery: expandPromqlQuery(dataTableQueryMetric.PromqlQuery),
			Filters:     filters,
		},
	}, nil
}

func expandDataTableLogsQuery(ctx context.Context, dataTableQueryLogs *DataTableQueryLogsModel) (*dashboards.DataTable_Query_Logs, diag.Diagnostics) {
	if dataTableQueryLogs == nil {
		return nil, nil
	}

	filters, diags := expandLogsFilters(ctx, dataTableQueryLogs.Filters)
	if diags.HasError() {
		return nil, diags
	}

	grouping, diags := expandDataTableLogsGrouping(ctx, dataTableQueryLogs.Grouping)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.DataTable_Query_Logs{
		Logs: &dashboards.DataTable_LogsQuery{
			LuceneQuery: expandLuceneQuery(dataTableQueryLogs.LuceneQuery),
			Filters:     filters,
			Grouping:    grouping,
		},
	}, nil
}

func expandDataTableLogsGrouping(ctx context.Context, grouping *DataTableLogsQueryGroupingModel) (*dashboards.DataTable_LogsQuery_Grouping, diag.Diagnostics) {
	if grouping == nil {
		return nil, nil
	}

	groupBy, diags := typeStringSliceToWrappedStringSlice(ctx, grouping.GroupBy.Elements())
	if diags.HasError() {
		return nil, diags
	}

	aggregations, diags := expandDataTableLogsAggregations(ctx, grouping.Aggregations)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.DataTable_LogsQuery_Grouping{
		GroupBy:      groupBy,
		Aggregations: aggregations,
	}, nil

}

func expandDataTableLogsAggregations(ctx context.Context, aggregations types.List) ([]*dashboards.DataTable_LogsQuery_Aggregation, diag.Diagnostics) {
	var diags diag.Diagnostics
	var aggregationsObjects []types.Object
	var expandedAggregations []*dashboards.DataTable_LogsQuery_Aggregation
	aggregations.ElementsAs(ctx, &aggregationsObjects, true)

	for _, ao := range aggregationsObjects {
		var aggregation DataTableLogsAggregationModel
		if dg := ao.As(ctx, &aggregation, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedAggregation, expandDiag := expandDataTableLogsAggregation(&aggregation)
		if expandDiag != nil {
			diags.Append(expandDiag)
			continue
		}
		expandedAggregations = append(expandedAggregations, expandedAggregation)
	}

	return expandedAggregations, diags
}

func expandDataTableLogsAggregation(aggregation *DataTableLogsAggregationModel) (*dashboards.DataTable_LogsQuery_Aggregation, diag.Diagnostic) {
	if aggregation == nil {
		return nil, nil
	}

	logsAggregation, dg := expandLogsAggregation(aggregation.Aggregation)
	if dg != nil {
		return nil, dg
	}

	return &dashboards.DataTable_LogsQuery_Aggregation{
		Id:          typeStringToWrapperspbString(aggregation.ID),
		Name:        typeStringToWrapperspbString(aggregation.Name),
		IsVisible:   typeBoolToWrapperspbBool(aggregation.IsVisible),
		Aggregation: logsAggregation,
	}, nil
}

func expandDataTableSpansQuery(ctx context.Context, dataTableQuerySpans *DataTableQuerySpansModel) (*dashboards.DataTable_Query_Spans, diag.Diagnostics) {
	if dataTableQuerySpans == nil {
		return nil, nil
	}

	filters, diags := expandSpansFilters(ctx, dataTableQuerySpans.Filters)
	if diags.HasError() {
		return nil, diags
	}

	grouping, diags := expandDataTableSpansGrouping(ctx, dataTableQuerySpans.Grouping)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.DataTable_Query_Spans{
		Spans: &dashboards.DataTable_SpansQuery{
			LuceneQuery: expandLuceneQuery(dataTableQuerySpans.LuceneQuery),
			Filters:     filters,
			Grouping:    grouping,
		},
	}, nil
}

func expandDataTableSpansGrouping(ctx context.Context, grouping *DataTableSpansQueryGroupingModel) (*dashboards.DataTable_SpansQuery_Grouping, diag.Diagnostics) {
	if grouping == nil {
		return nil, nil
	}

	groupBy, diags := expandSpansFields(ctx, grouping.GroupBy)
	if diags.HasError() {
		return nil, diags
	}

	aggregations, diags := expandDataTableSpansAggregations(ctx, grouping.Aggregations)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.DataTable_SpansQuery_Grouping{
		GroupBy:      groupBy,
		Aggregations: aggregations,
	}, nil
}

func expandDataTableSpansAggregations(ctx context.Context, spansAggregations types.List) ([]*dashboards.DataTable_SpansQuery_Aggregation, diag.Diagnostics) {
	var diags diag.Diagnostics
	var spansAggregationsObjects []types.Object
	var expandedSpansAggregations []*dashboards.DataTable_SpansQuery_Aggregation
	spansAggregations.ElementsAs(ctx, &spansAggregationsObjects, true)

	for _, sfo := range spansAggregationsObjects {
		var aggregation DataTableSpansAggregationModel
		if dg := sfo.As(ctx, &aggregation, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedSpansAggregation, expandDiag := expandDataTableSpansAggregation(&aggregation)
		if expandDiag != nil {
			diags.Append(expandDiag)
			continue
		}
		expandedSpansAggregations = append(expandedSpansAggregations, expandedSpansAggregation)
	}

	return expandedSpansAggregations, diags
}

func expandDataTableSpansAggregation(aggregation *DataTableSpansAggregationModel) (*dashboards.DataTable_SpansQuery_Aggregation, diag.Diagnostic) {
	if aggregation == nil {
		return nil, nil
	}

	spansAggregation, dg := expandSpansAggregation(aggregation.Aggregation)
	if dg != nil {
		return nil, dg
	}

	return &dashboards.DataTable_SpansQuery_Aggregation{
		Id:          typeStringToWrapperspbString(aggregation.ID),
		Name:        typeStringToWrapperspbString(aggregation.Name),
		IsVisible:   typeBoolToWrapperspbBool(aggregation.IsVisible),
		Aggregation: spansAggregation,
	}, nil
}

func expandDataTableColumns(ctx context.Context, columns types.List) ([]*dashboards.DataTable_Column, diag.Diagnostics) {
	var diags diag.Diagnostics
	var columnsObjects []types.Object
	var expandedColumns []*dashboards.DataTable_Column
	columns.ElementsAs(ctx, &columnsObjects, true)

	for _, co := range columnsObjects {
		var column DataTableColumnModel
		if dg := co.As(ctx, &column, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedColumn := expandDataTableColumn(column)
		expandedColumns = append(expandedColumns, expandedColumn)
	}

	return expandedColumns, diags
}

func expandDataTableColumn(column DataTableColumnModel) *dashboards.DataTable_Column {
	return &dashboards.DataTable_Column{
		Field: typeStringToWrapperspbString(column.Field),
		Width: typeInt64ToWrappedInt32(column.Width),
	}
}

func expandOrderBy(orderBy *OrderByModel) *dashboards.OrderingField {
	if orderBy == nil {
		return nil
	}
	return &dashboards.OrderingField{
		Field:          typeStringToWrapperspbString(orderBy.Field),
		OrderDirection: dashboardOrderDirectionSchemaToProto[orderBy.OrderDirection.ValueString()],
	}
}
func expandLineChart(ctx context.Context, lineChart *LineChartModel) (*dashboards.Widget_Definition, diag.Diagnostics) {
	if lineChart == nil {
		return nil, nil
	}

	legend, diags := expandLineChartLegend(ctx, lineChart.Legend)
	if diags.HasError() {
		return nil, diags
	}

	queryDefinitions, diags := expandLineChartQueryDefinitions(ctx, lineChart.QueryDefinitions)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Widget_Definition{
		Value: &dashboards.Widget_Definition_LineChart{
			LineChart: &dashboards.LineChart{
				Legend:           legend,
				Tooltip:          expandLineChartTooltip(lineChart.Tooltip),
				QueryDefinitions: queryDefinitions,
			},
		},
	}, nil
}

func expandLineChartLegend(ctx context.Context, legend *LegendModel) (*dashboards.Legend, diag.Diagnostics) {
	if legend == nil {
		return nil, nil
	}

	columns, diags := expandLineChartLegendColumns(ctx, legend.Columns.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Legend{
		IsVisible:    typeBoolToWrapperspbBool(legend.IsVisible),
		Columns:      columns,
		GroupByQuery: typeBoolToWrapperspbBool(legend.GroupByQuery),
	}, nil
}

func expandLineChartLegendColumns(ctx context.Context, columns []attr.Value) ([]dashboards.Legend_LegendColumn, diag.Diagnostics) {
	var diags diag.Diagnostics
	expandedColumns := make([]dashboards.Legend_LegendColumn, 0, len(columns))
	for _, s := range columns {
		v, err := s.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Extract LineChart Legend Columns Error", err.Error())
			continue
		}
		var column string
		if err = v.As(&column); err != nil {
			diags.AddError("Extract LineChart Legend Columns Error", err.Error())
			continue
		}

		expandedColumn := dashboardLegendColumnSchemaToProto[column]
		expandedColumns = append(expandedColumns, expandedColumn)
	}

	return expandedColumns, diags
}

func expandLineChartTooltip(tooltip *TooltipModel) *dashboards.LineChart_Tooltip {
	if tooltip == nil {
		return nil
	}

	return &dashboards.LineChart_Tooltip{
		ShowLabels: typeBoolToWrapperspbBool(tooltip.ShowLabels),
		Type:       dashboardSchemaToProtoTooltipType[tooltip.Type.ValueString()],
	}
}

func expandLineChartQueryDefinitions(ctx context.Context, queryDefinitions types.List) ([]*dashboards.LineChart_QueryDefinition, diag.Diagnostics) {
	var diags diag.Diagnostics
	var queryDefinitionsObjects []types.Object
	var expandedQueryDefinitions []*dashboards.LineChart_QueryDefinition
	queryDefinitions.ElementsAs(ctx, &queryDefinitionsObjects, true)

	for _, qdo := range queryDefinitionsObjects {
		var queryDefinition LineChartQueryDefinitionModel
		if dg := qdo.As(ctx, &queryDefinition, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedQueryDefinition, expandDiag := expandLineChartQueryDefinition(ctx, &queryDefinition)
		if expandDiag != nil {
			diags.Append(expandDiag...)
			continue
		}
		expandedQueryDefinitions = append(expandedQueryDefinitions, expandedQueryDefinition)
	}

	return expandedQueryDefinitions, diags
}

func expandLineChartQueryDefinition(ctx context.Context, queryDefinition *LineChartQueryDefinitionModel) (*dashboards.LineChart_QueryDefinition, diag.Diagnostics) {
	if queryDefinition == nil {
		return nil, nil
	}
	query, diags := expandLineChartQuery(ctx, queryDefinition.Query)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.LineChart_QueryDefinition{
		Id:                 typeStringToWrapperspbString(queryDefinition.ID),
		Query:              query,
		SeriesNameTemplate: typeStringToWrapperspbString(queryDefinition.SeriesNameTemplate),
		SeriesCountLimit:   typeInt64ToWrappedInt64(queryDefinition.SeriesCountLimit),
		Unit:               dashboardSchemaToProtoUnit[queryDefinition.Unit.ValueString()],
		ScaleType:          dashboardSchemaToProtoScaleType[queryDefinition.ScaleType.ValueString()],
		Name:               typeStringToWrapperspbString(queryDefinition.Name),
		IsVisible:          typeBoolToWrapperspbBool(queryDefinition.IsVisible),
	}, nil
}

func expandLineChartQuery(ctx context.Context, query *LineChartQueryModel) (*dashboards.LineChart_Query, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}

	switch {
	case query.Logs != nil:
		logs, diags := expandLineChartLogsQuery(ctx, query.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.LineChart_Query{
			Value: logs,
		}, nil
	case query.Metrics != nil:
		metrics, diags := expandLineChartMetricsQuery(ctx, query.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.LineChart_Query{
			Value: metrics,
		}, nil
	case query.Spans != nil:
		spans, diags := expandLineChartSpansQuery(ctx, query.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.LineChart_Query{
			Value: spans,
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand LineChart Query", "Unknown LineChart Query type")}
	}
}

func expandLineChartLogsQuery(ctx context.Context, logs *LineChartQueryLogsModel) (*dashboards.LineChart_Query_Logs, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	groupBy, diags := typeStringSliceToWrappedStringSlice(ctx, logs.GroupBy.Elements())
	if diags.HasError() {
		return nil, diags
	}

	aggregations, diags := expandLogsAggregations(ctx, logs.Aggregations)
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := expandLogsFilters(ctx, logs.Filters)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.LineChart_Query_Logs{
		Logs: &dashboards.LineChart_LogsQuery{
			LuceneQuery:  expandLuceneQuery(logs.LuceneQuery),
			GroupBy:      groupBy,
			Aggregations: aggregations,
			Filters:      filters,
		},
	}, nil
}

func expandLineChartMetricsQuery(ctx context.Context, metrics *LineChartQueryMetricsModel) (*dashboards.LineChart_Query_Metrics, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	filters, diags := expandMetricsFilters(ctx, metrics.Filters)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.LineChart_Query_Metrics{
		Metrics: &dashboards.LineChart_MetricsQuery{
			PromqlQuery: expandPromqlQuery(metrics.PromqlQuery),
			Filters:     filters,
		},
	}, nil
}

func expandLineChartSpansQuery(ctx context.Context, spans *LineChartQuerySpansModel) (*dashboards.LineChart_Query_Spans, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	groupBy, diags := expandSpansFields(ctx, spans.GroupBy)

	aggregations, diags := expandSpansAggregations(ctx, spans.Aggregations)
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := expandSpansFilters(ctx, spans.Filters)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.LineChart_Query_Spans{
		Spans: &dashboards.LineChart_SpansQuery{
			LuceneQuery:  expandLuceneQuery(spans.LuceneQuery),
			GroupBy:      groupBy,
			Aggregations: aggregations,
			Filters:      filters,
		},
	}, nil
}

func expandDashboardQuery(ctx context.Context, pieChartQuery *PieChartQueryModel) (*dashboards.PieChart_Query, diag.Diagnostics) {
	if pieChartQuery == nil {
		return nil, nil
	}

	switch {
	case pieChartQuery.Logs != nil:
		logs, diags := expandPieChartLogsQuery(ctx, pieChartQuery.Logs)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.PieChart_Query{
			Value: logs,
		}, nil
	case pieChartQuery.Metrics != nil:
		metrics, diags := expandPieChartMetricsQuery(ctx, pieChartQuery.Metrics)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.PieChart_Query{
			Value: metrics,
		}, nil
	case pieChartQuery.Spans != nil:
		spans, diags := expandPieChartSpansQuery(ctx, pieChartQuery.Spans)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.PieChart_Query{
			Value: spans,
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand PieChart Query", "Unknown PieChart Query type")}
	}
}

func expandPieChartLogsQuery(ctx context.Context, pieChartQueryLogs *PieChartQueryLogsModel) (*dashboards.PieChart_Query_Logs, diag.Diagnostics) {
	if pieChartQueryLogs == nil {
		return nil, nil
	}

	aggregation, dg := expandLogsAggregation(pieChartQueryLogs.Aggregation)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	filters, diags := expandLogsFilters(ctx, pieChartQueryLogs.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := typeStringSliceToWrappedStringSlice(ctx, pieChartQueryLogs.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.PieChart_Query_Logs{
		Logs: &dashboards.PieChart_LogsQuery{
			LuceneQuery:      expandLuceneQuery(pieChartQueryLogs.LuceneQuery),
			Aggregation:      aggregation,
			Filters:          filters,
			GroupNames:       groupNames,
			StackedGroupName: typeStringToWrapperspbString(pieChartQueryLogs.StackedGroupName),
		},
	}, nil
}

func expandPieChartMetricsQuery(ctx context.Context, pieChartQueryMetrics *PieChartQueryMetricsModel) (*dashboards.PieChart_Query_Metrics, diag.Diagnostics) {
	if pieChartQueryMetrics == nil {
		return nil, nil
	}

	filters, diags := expandMetricsFilters(ctx, pieChartQueryMetrics.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := typeStringSliceToWrappedStringSlice(ctx, pieChartQueryMetrics.GroupNames.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.PieChart_Query_Metrics{
		Metrics: &dashboards.PieChart_MetricsQuery{
			PromqlQuery:      expandPromqlQuery(pieChartQueryMetrics.PromqlQuery),
			GroupNames:       groupNames,
			Filters:          filters,
			StackedGroupName: typeStringToWrapperspbString(pieChartQueryMetrics.StackedGroupName),
		},
	}, nil
}

func expandPieChartSpansQuery(ctx context.Context, pieChartQuerySpans *PieChartQuerySpansModel) (*dashboards.PieChart_Query_Spans, diag.Diagnostics) {
	if pieChartQuerySpans == nil {
		return nil, nil
	}

	aggregation, dg := expandSpansAggregation(pieChartQuerySpans.Aggregation)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	filters, diags := expandSpansFilters(ctx, pieChartQuerySpans.Filters)
	if diags.HasError() {
		return nil, diags
	}

	groupNames, diags := expandSpansFields(ctx, pieChartQuerySpans.GroupNames)
	if diags.HasError() {
		return nil, diags
	}

	stackedGroupName, dg := expandSpansField(pieChartQuerySpans.StackedGroupName)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &dashboards.PieChart_Query_Spans{
		Spans: &dashboards.PieChart_SpansQuery{
			LuceneQuery:      expandLuceneQuery(pieChartQuerySpans.LuceneQuery),
			Aggregation:      aggregation,
			Filters:          filters,
			GroupNames:       groupNames,
			StackedGroupName: stackedGroupName,
		},
	}, nil
}

func expandDashboardVariables(ctx context.Context, variables types.List) ([]*dashboards.Variable, diag.Diagnostics) {
	var diags diag.Diagnostics
	var variablesObjects []types.Object
	var expandedVariables []*dashboards.Variable
	variables.ElementsAs(ctx, &variablesObjects, true)

	for _, vo := range variablesObjects {
		var variable DashboardVariableModel
		if dg := vo.As(ctx, &variable, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedVariable, expandDiags := expandDashboardVariable(ctx, variable)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedVariables = append(expandedVariables, expandedVariable)
	}

	return expandedVariables, diags
}

func expandDashboardVariable(ctx context.Context, variable DashboardVariableModel) (*dashboards.Variable, diag.Diagnostics) {
	definition, diags := expandDashboardVariableDefinition(ctx, variable.Definition)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboards.Variable{
		Name:        typeStringToWrapperspbString(variable.Name),
		DisplayName: typeStringToWrapperspbString(variable.DisplayName),
		Definition:  definition,
	}, nil
}

func expandDashboardVariableDefinition(ctx context.Context, definition *DashboardVariableDefinitionModel) (*dashboards.Variable_Definition, diag.Diagnostics) {
	if definition == nil {
		return nil, nil
	}

	switch {
	case definition.MultiSelect != nil:
		return expandMultiSelect(ctx, definition.MultiSelect)
	case !definition.ConstantValue.IsNull():
		return &dashboards.Variable_Definition{
			Value: &dashboards.Variable_Definition_Constant{
				Constant: &dashboards.Constant{
					Value: typeStringToWrapperspbString(definition.ConstantValue),
				},
			},
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Dashboard Variable", fmt.Sprintf("unknown variable definition type: %T", definition))}
	}
}

func expandMultiSelect(ctx context.Context, multiSelect *VariableMultiSelectModel) (*dashboards.Variable_Definition, diag.Diagnostics) {
	if multiSelect == nil {
		return nil, nil
	}

	source, diags := expandMultiSelectSource(ctx, multiSelect.Source)
	if diags.HasError() {
		return nil, diags
	}

	selection, diags := expandMultiSelectSelection(ctx, multiSelect.SelectedValues.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Variable_Definition{
		Value: &dashboards.Variable_Definition_MultiSelect{
			MultiSelect: &dashboards.MultiSelect{
				Source:               source,
				Selection:            selection,
				ValuesOrderDirection: dashboardOrderDirectionSchemaToProto[multiSelect.ValuesOrderDirection.ValueString()],
			},
		},
	}, nil
}

func expandMultiSelectSelection(ctx context.Context, selectedValues []attr.Value) (*dashboards.MultiSelect_Selection, diag.Diagnostics) {
	if len(selectedValues) == 0 {
		return &dashboards.MultiSelect_Selection{
			Value: &dashboards.MultiSelect_Selection_All{
				All: &dashboards.MultiSelect_Selection_AllSelection{},
			},
		}, nil
	}

	selections, diags := typeStringSliceToWrappedStringSlice(ctx, selectedValues)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboards.MultiSelect_Selection{
		Value: &dashboards.MultiSelect_Selection_List{
			List: &dashboards.MultiSelect_Selection_ListSelection{
				Values: selections,
			},
		},
	}, nil
}

func expandMultiSelectSource(ctx context.Context, source *VariableMultiSelectSourceModel) (*dashboards.MultiSelect_Source, diag.Diagnostics) {
	if source == nil {
		return nil, nil
	}

	switch {
	case !source.LogsPath.IsNull():
		return &dashboards.MultiSelect_Source{
			Value: &dashboards.MultiSelect_Source_LogsPath{
				LogsPath: &dashboards.MultiSelect_LogsPathSource{
					Value: typeStringToWrapperspbString(source.LogsPath),
				},
			},
		}, nil
	case !source.ConstantList.IsNull():
		constantList, diags := typeStringSliceToWrappedStringSlice(ctx, source.ConstantList.Elements())
		if diags.HasError() {
			return nil, diags
		}
		return &dashboards.MultiSelect_Source{
			Value: &dashboards.MultiSelect_Source_ConstantList{
				ConstantList: &dashboards.MultiSelect_ConstantListSource{
					Values: constantList,
				},
			},
		}, nil
	case source.MetricLabel != nil:
		return &dashboards.MultiSelect_Source{
			Value: &dashboards.MultiSelect_Source_MetricLabel{
				MetricLabel: &dashboards.MultiSelect_MetricLabelSource{
					MetricName: typeStringToWrapperspbString(source.MetricLabel.MetricName),
					Label:      typeStringToWrapperspbString(source.MetricLabel.Label),
				},
			},
		}, nil
	case source.SpanField != nil:
		spanField, dg := expandSpansField(source.SpanField)
		if dg != nil {
			return nil, diag.Diagnostics{dg}
		}
		return &dashboards.MultiSelect_Source{
			Value: &dashboards.MultiSelect_Source_SpanField{
				SpanField: &dashboards.MultiSelect_SpanFieldSource{
					Value: spanField,
				},
			},
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Multi Select Source", fmt.Sprintf("unknown multi select source type: %T", source))}
	}
}

func expandDashboardFilters(ctx context.Context, filters types.List) ([]*dashboards.Filter, diag.Diagnostics) {
	var diags diag.Diagnostics
	var filtersObjects []types.Object
	var expandedFilters []*dashboards.Filter
	filters.ElementsAs(ctx, &filtersObjects, true)

	for _, fo := range filtersObjects {
		var filter DashboardFilterModel
		if dg := fo.As(ctx, &filter, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedFilter, expandDiags := expandDashboardFilter(ctx, &filter)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedFilters = append(expandedFilters, expandedFilter)
	}

	return expandedFilters, diags
}

func expandDashboardFilter(ctx context.Context, filter *DashboardFilterModel) (*dashboards.Filter, diag.Diagnostics) {
	if filter == nil {
		return nil, nil
	}

	source, diags := expandFilterSource(ctx, filter)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Filter{
		Source:    source,
		Enabled:   typeBoolToWrapperspbBool(filter.Enabled),
		Collapsed: typeBoolToWrapperspbBool(filter.Collapsed),
	}, nil
}

func expandFilterSource(ctx context.Context, filter *DashboardFilterModel) (*dashboards.Filter_Source, diag.Diagnostics) {
	source := filter.Source
	if source == nil {
		return nil, nil
	}

	switch {
	case source.Logs != nil:
		return expandFilterSourceLogs(ctx, source.Logs)
	case source.Metrics != nil:
		return expandFilterSourceMetrics(ctx, source.Metrics)
	case source.Spans != nil:
		return expandFilterSourceSpans(ctx, source.Spans)
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Filter Source", fmt.Sprintf("Unknown filter source type: %#v", filter))}
	}
}

func expandFilterSourceLogs(ctx context.Context, logs *FilterSourceLogsModel) (*dashboards.Filter_Source, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	operator, diags := expandFilterOperator(ctx, logs.Operator)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Filter_Source{
		Value: &dashboards.Filter_Source_Logs{
			Logs: &dashboards.Filter_LogsFilter{
				Field:    typeStringToWrapperspbString(logs.Field),
				Operator: operator,
			},
		},
	}, nil
}

func expandFilterSourceMetrics(ctx context.Context, metrics *FilterSourceMetricsModel) (*dashboards.Filter_Source, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	operator, diags := expandFilterOperator(ctx, metrics.Operator)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Filter_Source{
		Value: &dashboards.Filter_Source_Metrics{
			Metrics: &dashboards.Filter_MetricsFilter{
				Metric:   typeStringToWrapperspbString(metrics.MetricName),
				Label:    typeStringToWrapperspbString(metrics.MetricLabel),
				Operator: operator,
			},
		},
	}, nil
}

func expandFilterSourceSpans(ctx context.Context, spans *FilterSourceSpansModel) (*dashboards.Filter_Source, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	field, dg := expandSpansField(spans.Field)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	operator, diags := expandFilterOperator(ctx, spans.Operator)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboards.Filter_Source{
		Value: &dashboards.Filter_Source_Spans{
			Spans: &dashboards.Filter_SpansFilter{
				Field:    field,
				Operator: operator,
			},
		},
	}, nil
}

func expandAbsoluteeDashboardTimeFrame(timeFrame *DashboardTimeFrameAbsoluteModel) (*dashboards.Dashboard_AbsoluteTimeFrame, diag.Diagnostic) {
	if timeFrame == nil {
		return nil, nil
	}

	fromTime, err := time.Parse(time.RFC3339, timeFrame.From.ValueString())
	if err != nil {
		return nil, diag.NewErrorDiagnostic("Error Expand Absolutee Dashboard Time Frame", fmt.Sprintf("Error parsing from time: %s", err.Error()))
	}
	toTime, err := time.Parse(time.RFC3339, timeFrame.To.ValueString())
	if err != nil {
		return nil, diag.NewErrorDiagnostic("Error Expand Absolutee Dashboard Time Frame", fmt.Sprintf("Error parsing from time: %s", err.Error()))
	}

	from := timestamppb.New(fromTime)
	to := timestamppb.New(toTime)

	return &dashboards.Dashboard_AbsoluteTimeFrame{
		AbsoluteTimeFrame: &dashboards.TimeFrame{
			From: from,
			To:   to,
		},
	}, nil
}

func expandRelativeDashboardTimeFrame(timeFrame *DashboardTimeFrameRelativeModel) (*dashboards.Dashboard_RelativeTimeFrame, diag.Diagnostic) {
	if timeFrame == nil {
		return nil, nil
	}
	duration, err := time.ParseDuration(timeFrame.Duration.ValueString())
	if err != nil {
		return nil, diag.NewErrorDiagnostic("Error Expand Relative Dashboard Time Frame", fmt.Sprintf("Error parsing duration: %s", err.Error()))
	}
	return &dashboards.Dashboard_RelativeTimeFrame{
		RelativeTimeFrame: durationpb.New(duration),
	}, nil
}

func expandDashboardUUID(id types.String) *dashboards.UUID {
	if id.IsNull() || id.IsUnknown() {
		return &dashboards.UUID{Value: RandStringBytes(21)}
	}
	return &dashboards.UUID{Value: id.ValueString()}
}

func flattenDashboard(ctx context.Context, plan DashboardResourceModel, dashboard *dashboards.Dashboard) (*DashboardResourceModel, diag.Diagnostics) {
	if !plan.ContentJson.IsNull() {
		contentJson, err := protojson.Marshal(dashboard)
		if err != nil {
			return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard", err.Error())}
		}
		if JSONStringsEqual(plan.ContentJson.ValueString(), string(contentJson)) {
			contentJson = []byte(plan.ContentJson.ValueString())
		}

		return &DashboardResourceModel{
			ContentJson: types.StringValue(string(contentJson)),
			ID:          types.StringValue(dashboard.GetId().GetValue()),
			Name:        types.StringNull(),
			Description: types.StringNull(),
			Variables:   types.ListNull(types.ObjectType{AttrTypes: dashboardsVariablesModelAttr()}),
			Filters:     types.ListNull(types.ObjectType{AttrTypes: dashboardsFiltersModelAttr()}),
		}, nil
	}

	layout, diags := flattenDashboardLayout(ctx, dashboard.GetLayout())
	if diags.HasError() {
		log.Printf("[ERROR] ERROR flattenDashboardLayout: %s", diags.Errors())
		return nil, diags
	}

	variables, diags := flattenDashboardVariables(ctx, dashboard.GetVariables())
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := flattenDashboardFilters(ctx, dashboard.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	return &DashboardResourceModel{
		ID:          types.StringValue(dashboard.GetId().GetValue()),
		Name:        types.StringValue(dashboard.GetName().GetValue()),
		Description: types.StringValue(dashboard.GetDescription().GetValue()),
		Layout:      layout,
		Variables:   variables,
		Filters:     filters,
		TimeFrame:   flattenDashboardTimeFrame(dashboard),
		ContentJson: types.StringNull(),
	}, nil
}

func flattenDashboardLayout(ctx context.Context, layout *dashboards.Layout) (*DashboardLayoutModel, diag.Diagnostics) {
	sections, diags := flattenDashboardSections(ctx, layout.GetSections())
	if diags.HasError() {
		return nil, diags
	}

	return &DashboardLayoutModel{
		Sections: sections,
	}, nil
}

func flattenDashboardSections(ctx context.Context, sections []*dashboards.Section) (types.List, diag.Diagnostics) {
	if len(sections) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: sectionModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	sectionsElements := make([]attr.Value, 0, len(sections))
	for _, section := range sections {
		flattenedSection, diags := flattenDashboardSection(ctx, section)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		sectionsElement, diags := types.ObjectValueFrom(ctx, sectionModelAttr(), flattenedSection)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		sectionsElements = append(sectionsElements, sectionsElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: sectionModelAttr()}, sectionsElements), diagnostics
}

func sectionModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"id": types.StringType,
		"rows": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: rowModelAttr(),
			},
		},
	}
}

func rowModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"id":     types.StringType,
		"height": types.Int64Type,
		"widgets": types.ListType{
			ElemType: types.ObjectType{AttrTypes: widgetModelAttr()},
		},
	}
}

func widgetModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"id":          types.StringType,
		"title":       types.StringType,
		"description": types.StringType,
		"definition": types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"line_chart": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"legend": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"is_visible": types.BoolType,
								"columns": types.ListType{
									ElemType: types.StringType,
								},
								"group_by_query": types.BoolType,
							},
						},
						"tooltip": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"show_labels": types.BoolType,
								"type":        types.StringType,
							},
						},
						"query_definitions": types.ListType{
							ElemType: types.ObjectType{
								AttrTypes: map[string]attr.Type{
									"id": types.StringType,
									"query": types.ObjectType{
										AttrTypes: map[string]attr.Type{
											"logs": types.ObjectType{
												AttrTypes: map[string]attr.Type{
													"lucene_query": types.StringType,
													"group_by": types.ListType{
														ElemType: types.StringType,
													},
													"aggregations": types.ListType{
														ElemType: types.ObjectType{
															AttrTypes: aggregationModelAttr(),
														},
													},
													"filters": types.ListType{
														ElemType: types.ObjectType{
															AttrTypes: filterModelAttr(),
														},
													},
												},
											},
											"metrics": types.ObjectType{
												AttrTypes: map[string]attr.Type{
													"promql_query": types.StringType,
													"filters": types.ListType{
														ElemType: types.ObjectType{
															AttrTypes: metricsFilterModelAttr(),
														},
													},
												},
											},
											"spans": types.ObjectType{
												AttrTypes: map[string]attr.Type{
													"lucene_query": types.StringType,
													"group_by": types.ListType{
														ElemType: types.ObjectType{
															AttrTypes: spansFieldModelAttr(),
														},
													},
													"aggregations": types.ListType{
														ElemType: types.ObjectType{
															AttrTypes: spansAggregationModelAttr(),
														},
													},
													"filters": types.ListType{
														ElemType: types.ObjectType{
															AttrTypes: spansFilterModelAttr(),
														},
													},
												},
											},
										},
									},
									"series_name_template": types.StringType,
									"series_count_limit":   types.Int64Type,
									"unit":                 types.StringType,
									"scale_type":           types.StringType,
									"name":                 types.StringType,
									"is_visible":           types.BoolType,
								},
							},
						},
					},
				},
				"data_table": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"query": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"logs": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"lucene_query": types.StringType,
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: filterModelAttr(),
											},
										},
										"grouping": types.ObjectType{
											AttrTypes: map[string]attr.Type{
												"group_by": types.StringType,
												"aggregations": types.ListType{
													ElemType: types.ObjectType{
														AttrTypes: map[string]attr.Type{
															"id":         types.StringType,
															"name":       types.StringType,
															"is_visible": types.BoolType,
															"aggregation": types.ObjectType{
																AttrTypes: aggregationModelAttr(),
															},
														},
													},
												},
											},
										},
									},
								},
								"spans": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"lucene_query": types.StringType,
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: spansFilterModelAttr(),
											},
										},
										"grouping": types.ObjectType{
											AttrTypes: map[string]attr.Type{
												"group_by": types.ListType{
													ElemType: types.ObjectType{
														AttrTypes: spansFieldModelAttr(),
													},
												},
												"aggregations": types.ListType{
													ElemType: types.ObjectType{
														AttrTypes: map[string]attr.Type{
															"id":         types.StringType,
															"name":       types.StringType,
															"is_visible": types.BoolType,
															"aggregation": types.ObjectType{
																AttrTypes: spansAggregationModelAttr(),
															},
														},
													},
												},
											},
										},
									},
								},
								"metrics": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"promql_query": types.StringType,
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: metricsFilterModelAttr(),
											},
										},
									},
								},
							},
						},
						"results_per_page": types.Int64Type,
						"row_style":        types.StringType,
						"columns": types.ListType{
							ElemType: types.ObjectType{
								AttrTypes: dataTableColumnModelAttr(),
							},
						},
						"order_by": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"field":           types.StringType,
								"order_direction": types.StringType,
							},
						},
					},
				},
				"gauge": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"query": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"logs": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"lucene_query": types.StringType,
										"logs_aggregation": types.ObjectType{
											AttrTypes: aggregationModelAttr(),
										},
										"aggregation": types.StringType,
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: filterModelAttr(),
											},
										},
									},
								},
								"metrics": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"promql_query": types.StringType,
										"aggregation":  types.StringType,
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: metricsFilterModelAttr(),
											},
										},
									},
								},
								"spans": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"lucene_query": types.StringType,
										"spans_aggregation": types.ObjectType{
											AttrTypes: spansAggregationModelAttr(),
										},
										"aggregation": types.StringType,
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: spansFilterModelAttr(),
											},
										},
									},
								},
							},
						},
						"min":            types.Float64Type,
						"max":            types.Float64Type,
						"show_inner_arc": types.BoolType,
						"show_outer_arc": types.BoolType,
						"unit":           types.StringType,
						"thresholds": types.ListType{
							ElemType: types.ObjectType{
								AttrTypes: map[string]attr.Type{
									"from":  types.Float64Type,
									"color": types.StringType,
								},
							},
						},
					},
				},
				"pie_chart": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"query": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"logs": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"lucene_query": types.StringType,
										"aggregation": types.ObjectType{
											AttrTypes: aggregationModelAttr(),
										},
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: filterModelAttr(),
											},
										},
										"group_names": types.ListType{
											ElemType: types.StringType,
										},
										"stacked_group_name": types.StringType,
									},
								},
								"metrics": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"promql_query": types.StringType,
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: metricsFilterModelAttr(),
											},
										},
										"group_names": types.ListType{
											ElemType: types.StringType,
										},
										"stacked_group_name": types.StringType,
									},
								},
								"spans": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"lucene_query": types.StringType,
										"aggregation": types.ObjectType{
											AttrTypes: spansAggregationModelAttr(),
										},
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: spansFilterModelAttr(),
											},
										},
										"group_names": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: spansFieldModelAttr(),
											},
										},
										"stacked_group_name": types.ObjectType{
											AttrTypes: spansFieldModelAttr(),
										},
									},
								},
							},
						},
						"max_slices_per_chart": types.Int64Type,
						"min_slice_percentage": types.Int64Type,
						"stack_definition": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"max_slices_per_stack": types.Int64Type,
								"stack_name_template":  types.StringType,
							},
						},
						"label_definition": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"label_source":    types.StringType,
								"is_visible":      types.BoolType,
								"show_name":       types.BoolType,
								"show_value":      types.BoolType,
								"show_percentage": types.BoolType,
							},
						},
						"show_legend":         types.BoolType,
						"group_name_template": types.StringType,
						"unit":                types.StringType,
					},
				},
				"bar_chart": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"query": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"logs": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"lucene_query": types.StringType,
										"aggregation": types.ObjectType{
											AttrTypes: aggregationModelAttr(),
										},
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: filterModelAttr(),
											},
										},
										"group_names": types.ListType{
											ElemType: types.StringType,
										},
										"stacked_group_name": types.StringType,
									},
								},
								"metrics": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"promql_query": types.StringType,
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: metricsFilterModelAttr(),
											},
										},
										"group_names": types.ListType{
											ElemType: types.StringType,
										},
										"stacked_group_name": types.StringType,
									},
								},
								"spans": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"lucene_query": types.StringType,
										"aggregation": types.ObjectType{
											AttrTypes: spansAggregationModelAttr(),
										},
										"filters": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: spansFilterModelAttr(),
											},
										},
										"group_names": types.ListType{
											ElemType: types.ObjectType{
												AttrTypes: spansFieldModelAttr(),
											},
										},
										"stacked_group_name": types.ObjectType{
											AttrTypes: spansFieldModelAttr(),
										},
									},
								},
							},
						},
						"max_bars_per_chart":  types.Int64Type,
						"group_name_template": types.StringType,
						"stack_definition": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"stack_name_template": types.StringType,
								"max_slices_per_bar":  types.Int64Type,
							},
						},
						"scale_type": types.StringType,
						"colors_by":  types.StringType,
						"xaxis": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"type":              types.StringType,
								"interval":          types.StringType,
								"buckets_presented": types.Int64Type,
							},
						},
						"unit": types.StringType,
					},
				},
			},
		},
		"width": types.Int64Type,
	}
}

func lineChartQueryDefinitionModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"id": types.StringType,
		"query": types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"logs": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"lucene_query": types.StringType,
						"group_by": types.ListType{
							ElemType: types.StringType,
						},
						"aggregations": types.ListType{
							ElemType: types.ObjectType{
								AttrTypes: aggregationModelAttr(),
							},
						},
						"filters": types.ListType{
							ElemType: types.ObjectType{
								AttrTypes: filterModelAttr(),
							},
						},
					},
				},
				"metrics": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"promql_query": types.StringType,
						"filters": types.ListType{
							ElemType: types.ObjectType{
								AttrTypes: metricsFilterModelAttr(),
							},
						},
					},
				},
				"spans": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"lucene_query": types.StringType,
						"group_by": types.ListType{
							ElemType: types.ObjectType{
								AttrTypes: spansFieldModelAttr(),
							},
						},
						"aggregations": types.ListType{
							ElemType: types.ObjectType{
								AttrTypes: map[string]attr.Type{
									"type":             types.StringType,
									"aggregation_type": types.StringType,
									"field":            types.StringType,
								},
							},
						},
						"filters": types.ListType{
							ElemType: types.ObjectType{
								AttrTypes: spansFilterModelAttr(),
							},
						},
					},
				},
			},
		},
		"series_name_template": types.StringType,
		"series_count_limit":   types.Int64Type,
		"unit":                 types.StringType,
		"scale_type":           types.StringType,
		"name":                 types.StringType,
		"is_visible":           types.BoolType,
	}
}

func aggregationModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"type":  types.StringType,
		"field": types.StringType,
	}
}

func filterModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field": types.StringType,
		"operator": types.ObjectType{
			AttrTypes: filterOperatorModelAttr(),
		},
	}
}

func filterOperatorModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"type": types.StringType,
		"selected_values": types.ListType{
			ElemType: types.StringType,
		},
	}
}

func metricsFilterModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"metric": types.StringType,
		"label":  types.StringType,
		"operator": types.ObjectType{
			AttrTypes: filterOperatorModelAttr(),
		},
	}
}

func spansFieldModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"type":  types.StringType,
		"value": types.StringType,
	}
}

func groupingAggregationModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"id":         types.StringType,
		"name":       types.StringType,
		"is_visible": types.BoolType,
		"aggregation": types.ObjectType{
			AttrTypes: aggregationModelAttr(),
		},
	}
}

func dataTableColumnModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field": types.StringType,
		"width": types.Int64Type,
	}
}

func spansAggregationModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"type":             types.StringType,
		"aggregation_type": types.StringType,
		"field":            types.StringType,
	}
}

func dashboardsVariablesModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"name":         types.StringType,
		"display_name": types.StringType,
		"definition": types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"constant_value": types.StringType,
				"multi_select": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"selected_values": types.ListType{
							ElemType: types.StringType,
						},
						"values_order_direction": types.StringType,
						"source": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"logs_path": types.StringType,
								"metric_label": types.ObjectType{
									AttrTypes: map[string]attr.Type{
										"metric_name": types.StringType,
										"label":       types.StringType,
									},
								},
								"constant_list": types.ListType{
									ElemType: types.StringType,
								},
								"span_field": types.ObjectType{
									AttrTypes: spansFieldModelAttr(),
								},
							},
						},
					},
				},
			},
		},
	}
}

func dashboardsFiltersModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"source": types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"logs":    types.ObjectType{AttrTypes: filterModelAttr()},
				"metrics": types.ObjectType{AttrTypes: filterSourceMetricsModelAttr()},
				"spans":   types.ObjectType{AttrTypes: filterSourceSpansModelAttr()},
			},
		},
		"enabled":   types.BoolType,
		"collapsed": types.BoolType,
	}
}

func filterSourceSpansModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field": types.ObjectType{
			AttrTypes: spansFieldModelAttr(),
		},
		"operator": types.ObjectType{
			AttrTypes: filterOperatorModelAttr(),
		},
	}
}

func filterSourceMetricsModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"metric_name": types.StringType,
		"label":       types.StringType,
		"operator": types.ObjectType{
			AttrTypes: filterOperatorModelAttr(),
		},
	}
}

func flattenDashboardSection(ctx context.Context, section *dashboards.Section) (*SectionModel, diag.Diagnostics) {
	if section == nil {
		return nil, nil
	}

	rows, diags := flattenDashboardRows(ctx, section.GetRows())
	if diags.HasError() {
		return nil, diags
	}

	return &SectionModel{
		ID:   types.StringValue(section.GetId().GetValue()),
		Rows: rows,
	}, nil
}

func flattenDashboardRows(ctx context.Context, rows []*dashboards.Row) (types.List, diag.Diagnostics) {
	if len(rows) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: rowModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	rowsElements := make([]attr.Value, 0, len(rows))
	for _, row := range rows {
		flattenedRow, diags := flattenDashboardRow(ctx, row)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		rowElement, diags := types.ObjectValueFrom(ctx, rowModelAttr(), flattenedRow)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		rowsElements = append(rowsElements, rowElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: rowModelAttr()}, rowsElements), diagnostics
}

func flattenDashboardRow(ctx context.Context, row *dashboards.Row) (*RowModel, diag.Diagnostics) {
	if row == nil {
		return nil, nil
	}

	widgets, diags := flattenDashboardWidgets(ctx, row.GetWidgets())
	if diags.HasError() {
		return nil, diags
	}
	return &RowModel{
		ID:      types.StringValue(row.GetId().GetValue()),
		Height:  wrapperspbInt32ToTypeInt64(row.GetAppearance().GetHeight()),
		Widgets: widgets,
	}, nil
}

func flattenDashboardWidgets(ctx context.Context, widgets []*dashboards.Widget) (types.List, diag.Diagnostics) {
	if len(widgets) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: widgetModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	widgetsElements := make([]attr.Value, 0, len(widgets))
	for _, widget := range widgets {
		flattenedWidget, diags := flattenDashboardWidget(ctx, widget)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		widgetElement, diags := types.ObjectValueFrom(ctx, widgetModelAttr(), flattenedWidget)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		widgetsElements = append(widgetsElements, widgetElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: widgetModelAttr()}, widgetsElements), diagnostics
}

func flattenDashboardWidget(ctx context.Context, widget *dashboards.Widget) (*WidgetModel, diag.Diagnostics) {
	if widget == nil {
		return nil, nil
	}

	definition, diags := flattenDashboardWidgetDefinition(ctx, widget.GetDefinition())
	if diags.HasError() {
		return nil, diags
	}

	return &WidgetModel{
		ID:          types.StringValue(widget.GetId().GetValue()),
		Title:       wrapperspbStringToTypeString(widget.GetTitle()),
		Description: wrapperspbStringToTypeString(widget.GetDescription()),
		Width:       wrapperspbInt32ToTypeInt64(widget.GetAppearance().GetWidth()),
		Definition:  definition,
	}, nil
}

func flattenDashboardWidgetDefinition(ctx context.Context, definition *dashboards.Widget_Definition) (*WidgetDefinitionModel, diag.Diagnostics) {
	if definition == nil {
		return nil, nil
	}

	switch definition.GetValue().(type) {
	case *dashboards.Widget_Definition_LineChart:
		return flattenLineChart(ctx, definition.GetLineChart())
	case *dashboards.Widget_Definition_DataTable:
		return flattenDataTable(ctx, definition.GetDataTable())
	case *dashboards.Widget_Definition_Gauge:
		return flattenGauge(ctx, definition.GetGauge())
	case *dashboards.Widget_Definition_PieChart:
		return flattenPieChart(ctx, definition.GetPieChart())
	case *dashboards.Widget_Definition_BarChart:
		return flattenBarChart(ctx, definition.GetBarChart())
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Widget Definition", "unknown widget definition type")}
	}
}

func flattenLineChart(ctx context.Context, lineChart *dashboards.LineChart) (*WidgetDefinitionModel, diag.Diagnostics) {
	if lineChart == nil {
		return nil, nil
	}

	queryDefinitions, diags := flattenLineChartQueryDefinitions(ctx, lineChart.GetQueryDefinitions())
	if diags.HasError() {
		return nil, diags
	}

	return &WidgetDefinitionModel{
		LineChart: &LineChartModel{
			Legend:           flattenLegend(lineChart.GetLegend()),
			Tooltip:          flattenTooltip(lineChart.GetTooltip()),
			QueryDefinitions: queryDefinitions,
		},
	}, nil
}

func flattenLegend(legend *dashboards.Legend) *LegendModel {
	if legend == nil {
		return nil
	}

	return &LegendModel{
		IsVisible:    wrapperspbBoolToTypeBool(legend.GetIsVisible()),
		GroupByQuery: wrapperspbBoolToTypeBool(legend.GetGroupByQuery()),
		Columns:      flattenLegendColumns(legend.GetColumns()),
	}
}

func flattenLegendColumns(columns []dashboards.Legend_LegendColumn) types.List {
	if len(columns) == 0 {
		return types.ListNull(types.StringType)
	}

	columnsElements := make([]attr.Value, 0, len(columns))
	for _, column := range columns {
		flattenedColumn := dashboardLegendColumnProtoToSchema[column]
		columnElement := types.StringValue(flattenedColumn)
		columnsElements = append(columnsElements, columnElement)
	}

	return types.ListValueMust(types.StringType, columnsElements)
}

func flattenTooltip(tooltip *dashboards.LineChart_Tooltip) *TooltipModel {
	if tooltip == nil {
		return nil
	}
	return &TooltipModel{
		ShowLabels: wrapperspbBoolToTypeBool(tooltip.GetShowLabels()),
		Type:       types.StringValue(dashboardProtoToSchemaTooltipType[tooltip.GetType()]),
	}
}

func flattenLineChartQueryDefinitions(ctx context.Context, definitions []*dashboards.LineChart_QueryDefinition) (types.List, diag.Diagnostics) {
	if len(definitions) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: lineChartQueryDefinitionModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	definitionsElements := make([]attr.Value, 0, len(definitions))
	for _, definition := range definitions {
		flattenedDefinition, diags := flattenLineChartQueryDefinition(ctx, definition)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		definitionElement, diags := types.ObjectValueFrom(ctx, lineChartQueryDefinitionModelAttr(), flattenedDefinition)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		definitionsElements = append(definitionsElements, definitionElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: lineChartQueryDefinitionModelAttr()}, definitionsElements), diagnostics
}

func flattenLineChartQueryDefinition(ctx context.Context, definition *dashboards.LineChart_QueryDefinition) (*LineChartQueryDefinitionModel, diag.Diagnostics) {
	if definition == nil {
		return nil, nil
	}

	query, diags := flattenLineChartQuery(ctx, definition.GetQuery())
	if diags.HasError() {
		return nil, diags
	}

	return &LineChartQueryDefinitionModel{
		ID:                 wrapperspbStringToTypeString(definition.GetId()),
		Query:              query,
		SeriesNameTemplate: wrapperspbStringToTypeString(definition.GetSeriesNameTemplate()),
		SeriesCountLimit:   wrapperspbInt64ToTypeInt64(definition.GetSeriesCountLimit()),
		Unit:               types.StringValue(dashboardProtoToSchemaUnit[definition.GetUnit()]),
		ScaleType:          types.StringValue(dashboardProtoToSchemaScaleType[definition.GetScaleType()]),
		Name:               wrapperspbStringToTypeString(definition.GetName()),
		IsVisible:          wrapperspbBoolToTypeBool(definition.GetIsVisible()),
	}, nil
}

func flattenLineChartQuery(ctx context.Context, query *dashboards.LineChart_Query) (*LineChartQueryModel, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}

	switch query.GetValue().(type) {
	case *dashboards.LineChart_Query_Logs:
		return flattenLineChartQueryLogs(ctx, query.GetLogs())
	case *dashboards.LineChart_Query_Metrics:
		return flattenLineChartQueryMetrics(ctx, query.GetMetrics())
	case *dashboards.LineChart_Query_Spans:
		return flattenLineChartQuerySpans(ctx, query.GetSpans())
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Line Chart Query", "unknown line chart query type")}
	}
}

func flattenLineChartQueryLogs(ctx context.Context, logs *dashboards.LineChart_LogsQuery) (*LineChartQueryModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	aggregations, diags := flattenAggregations(ctx, logs.GetAggregations())
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := flattenLogsFilters(ctx, logs.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	return &LineChartQueryModel{
		Logs: &LineChartQueryLogsModel{
			LuceneQuery:  wrapperspbStringToTypeString(logs.GetLuceneQuery().GetValue()),
			GroupBy:      wrappedStringSliceToTypeStringList(logs.GetGroupBy()),
			Aggregations: aggregations,
			Filters:      filters,
		},
	}, nil
}

func flattenAggregations(ctx context.Context, aggregations []*dashboards.LogsAggregation) (types.List, diag.Diagnostics) {
	if len(aggregations) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: aggregationModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	aggregationsElements := make([]attr.Value, 0, len(aggregations))
	for _, aggregation := range aggregations {
		flattenedAggregation, dg := flattenLogsAggregation(aggregation)
		if dg != nil {
			diagnostics.Append(dg)
			continue
		}
		aggregationElement, diags := types.ObjectValueFrom(ctx, aggregationModelAttr(), flattenedAggregation)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		aggregationsElements = append(aggregationsElements, aggregationElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: aggregationModelAttr()}, aggregationsElements), diagnostics
}

func flattenLogsAggregation(aggregation *dashboards.LogsAggregation) (*LogsAggregationModel, diag.Diagnostic) {
	if aggregation == nil {
		return nil, nil
	}

	switch aggregationValue := aggregation.GetValue().(type) {
	case *dashboards.LogsAggregation_Count_:
		return &LogsAggregationModel{
			Type: types.StringValue("count"),
		}, nil
	case *dashboards.LogsAggregation_CountDistinct_:
		return &LogsAggregationModel{
			Type:  types.StringValue("count_distinct"),
			Field: wrapperspbStringToTypeString(aggregationValue.CountDistinct.GetField()),
		}, nil
	case *dashboards.LogsAggregation_Sum_:
		return &LogsAggregationModel{
			Type:  types.StringValue("sum"),
			Field: wrapperspbStringToTypeString(aggregationValue.Sum.GetField()),
		}, nil
	case *dashboards.LogsAggregation_Average_:
		return &LogsAggregationModel{
			Type:  types.StringValue("avg"),
			Field: wrapperspbStringToTypeString(aggregationValue.Average.GetField()),
		}, nil
	case *dashboards.LogsAggregation_Min_:
		return &LogsAggregationModel{
			Type:  types.StringValue("min"),
			Field: wrapperspbStringToTypeString(aggregationValue.Min.GetField()),
		}, nil
	case *dashboards.LogsAggregation_Max_:
		return &LogsAggregationModel{
			Type:  types.StringValue("max"),
			Field: wrapperspbStringToTypeString(aggregationValue.Max.GetField()),
		}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Error Flatten Logs Aggregation", "unknown logs aggregation type")
	}
}

func flattenLogsFilters(ctx context.Context, filters []*dashboards.Filter_LogsFilter) (types.List, diag.Diagnostics) {
	if len(filters) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: filterModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0, len(filters))
	for _, filter := range filters {
		flattenedFilter, dg := flattenLogsFilter(filter)
		if dg != nil {
			diagnostics.Append(dg)
			continue
		}
		filterElement, diags := types.ObjectValueFrom(ctx, filterModelAttr(), flattenedFilter)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		filtersElements = append(filtersElements, filterElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: filterModelAttr()}, filtersElements), diagnostics
}

func flattenLogsFilter(filter *dashboards.Filter_LogsFilter) (*FilterModel, diag.Diagnostic) {
	if filter == nil {
		return nil, nil
	}

	operator, dg := flattenFilterOperator(filter.GetOperator())
	if dg != nil {
		return nil, dg
	}

	return &FilterModel{
		Field:    wrapperspbStringToTypeString(filter.GetField()),
		Operator: operator,
	}, nil
}

func flattenFilterOperator(operator *dashboards.Filter_Operator) (*FilterOperatorModel, diag.Diagnostic) {
	switch operator.GetValue().(type) {
	case *dashboards.Filter_Operator_Equals:
		switch operator.GetEquals().GetSelection().GetValue().(type) {
		case *dashboards.Filter_Equals_Selection_All:
			return &FilterOperatorModel{
				Type:           types.StringValue("equals"),
				SelectedValues: types.ListNull(types.StringType),
			}, nil
		case *dashboards.Filter_Equals_Selection_List:
			return &FilterOperatorModel{
				Type:           types.StringValue("equals"),
				SelectedValues: wrappedStringSliceToTypeStringList(operator.GetEquals().GetSelection().GetList().GetValues()),
			}, nil
		default:
			return nil, diag.NewErrorDiagnostic("Error Flatten Logs Filter Operator Equals", "unknown logs filter operator equals selection type")
		}
	case *dashboards.Filter_Operator_NotEquals:
		switch operator.GetNotEquals().GetSelection().GetValue().(type) {
		case *dashboards.Filter_NotEquals_Selection_List:
			return &FilterOperatorModel{
				Type:           types.StringValue("not_equals"),
				SelectedValues: wrappedStringSliceToTypeStringList(operator.GetNotEquals().GetSelection().GetList().GetValues()),
			}, nil
		default:
			return nil, diag.NewErrorDiagnostic("Error Flatten Logs Filter Operator NotEquals", "unknown logs filter operator not_equals selection type")
		}
	default:
		return nil, diag.NewErrorDiagnostic("Error Flatten Logs Filter Operator", "unknown logs filter operator type")
	}
}

func flattenLineChartQueryMetrics(ctx context.Context, metrics *dashboards.LineChart_MetricsQuery) (*LineChartQueryModel, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	filters, diags := flattenMetricsFilters(ctx, metrics.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	return &LineChartQueryModel{
		Metrics: &LineChartQueryMetricsModel{
			PromqlQuery: wrapperspbStringToTypeString(metrics.GetPromqlQuery().GetValue()),
			Filters:     filters,
		},
	}, nil
}

func flattenMetricsFilters(ctx context.Context, filters []*dashboards.Filter_MetricsFilter) (types.List, diag.Diagnostics) {
	if len(filters) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: metricsFilterModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0, len(filters))
	for _, filter := range filters {
		flattenedFilter, dg := flattenMetricsFilter(filter)
		if dg != nil {
			diagnostics.Append(dg)
			continue
		}
		filterElement, diags := types.ObjectValueFrom(ctx, metricsFilterModelAttr(), flattenedFilter)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		filtersElements = append(filtersElements, filterElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: metricsFilterModelAttr()}, filtersElements), diagnostics
}

func flattenMetricsFilter(filter *dashboards.Filter_MetricsFilter) (*MetricsFilterModel, diag.Diagnostic) {
	if filter == nil {
		return nil, nil
	}

	operator, dg := flattenFilterOperator(filter.GetOperator())
	if dg != nil {
		return nil, dg
	}

	return &MetricsFilterModel{
		Metric:   wrapperspbStringToTypeString(filter.GetMetric()),
		Label:    wrapperspbStringToTypeString(filter.GetLabel()),
		Operator: operator,
	}, nil
}

func flattenLineChartQuerySpans(ctx context.Context, spans *dashboards.LineChart_SpansQuery) (*LineChartQueryModel, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	groupBy, diags := flattenSpansFields(ctx, spans.GetGroupBy())
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := flattenSpansFilters(ctx, spans.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	return &LineChartQueryModel{
		Spans: &LineChartQuerySpansModel{
			LuceneQuery: wrapperspbStringToTypeString(spans.GetLuceneQuery().GetValue()),
			GroupBy:     groupBy,
			Filters:     filters,
		},
	}, nil
}

func flattenSpansFilters(ctx context.Context, filters []*dashboards.Filter_SpansFilter) (types.List, diag.Diagnostics) {
	if len(filters) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: spansFilterModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0, len(filters))
	for _, filter := range filters {
		flattenedFilter, dg := flattenSpansFilter(filter)
		if dg != nil {
			diagnostics.Append(dg)
			continue
		}
		filterElement, diags := types.ObjectValueFrom(ctx, spansFilterModelAttr(), flattenedFilter)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		filtersElements = append(filtersElements, filterElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: spansFilterModelAttr()}, filtersElements), diagnostics

}

func spansFilterModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field": types.ObjectType{
			AttrTypes: spansFieldModelAttr(),
		},
		"operator": types.ObjectType{
			AttrTypes: filterOperatorModelAttr(),
		},
	}
}

func flattenSpansFilter(filter *dashboards.Filter_SpansFilter) (*SpansFilterModel, diag.Diagnostic) {
	if filter == nil {
		return nil, nil
	}

	operator, dg := flattenFilterOperator(filter.GetOperator())
	if dg != nil {
		return nil, dg
	}

	field, dg := flattenSpansField(filter.GetField())
	if dg != nil {
		return nil, dg
	}

	return &SpansFilterModel{
		Field:    field,
		Operator: operator,
	}, nil
}

func flattenSpansFields(ctx context.Context, spanFields []*dashboards.SpanField) (types.List, diag.Diagnostics) {
	if len(spanFields) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: spansFieldModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	spanFieldElements := make([]attr.Value, 0, len(spanFields))
	for _, field := range spanFields {
		flattenedField, dg := flattenSpansField(field)
		if dg != nil {
			diagnostics.Append(dg)
			continue
		}
		fieldElement, diags := types.ObjectValueFrom(ctx, spansFieldModelAttr(), flattenedField)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		spanFieldElements = append(spanFieldElements, fieldElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: spansFieldModelAttr()}, spanFieldElements), diagnostics
}

func flattenSpansField(field *dashboards.SpanField) (*SpansFieldModel, diag.Diagnostic) {
	if field == nil {
		return nil, nil
	}

	switch field.GetValue().(type) {
	case *dashboards.SpanField_MetadataField_:
		return &SpansFieldModel{
			Type:  types.StringValue("metadata"),
			Value: types.StringValue(dashboardProtoToSchemaSpanFieldMetadataField[field.GetMetadataField()]),
		}, nil
	case *dashboards.SpanField_TagField:
		return &SpansFieldModel{
			Type:  types.StringValue("tag"),
			Value: wrapperspbStringToTypeString(field.GetTagField()),
		}, nil
	case *dashboards.SpanField_ProcessTagField:
		return &SpansFieldModel{
			Type:  types.StringValue("process_tag"),
			Value: wrapperspbStringToTypeString(field.GetProcessTagField()),
		}, nil

	default:
		return nil, diag.NewErrorDiagnostic("Error Flatten Spans Field", "unknown spans field type")
	}
}

func flattenDataTable(ctx context.Context, table *dashboards.DataTable) (*WidgetDefinitionModel, diag.Diagnostics) {
	if table == nil {
		return nil, nil
	}

	query, diags := flattenDataTableQuery(ctx, table.GetQuery())
	if diags.HasError() {
		return nil, diags
	}

	columns, diags := flattenDataTableColumns(ctx, table.GetColumns())
	if diags.HasError() {
		return nil, diags
	}

	return &WidgetDefinitionModel{
		DataTable: &DataTableModel{
			Query:          query,
			ResultsPerPage: wrapperspbInt32ToTypeInt64(table.GetResultsPerPage()),
			RowStyle:       types.StringValue(dashboardRowStyleProtoToSchema[table.GetRowStyle()]),
			Columns:        columns,
			OrderBy:        flattenOrderBy(table.GetOrderBy()),
		},
	}, nil
}

func flattenDataTableQuery(ctx context.Context, query *dashboards.DataTable_Query) (*DataTableQueryModel, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}

	switch query.GetValue().(type) {
	case *dashboards.DataTable_Query_Logs:
		return flattenDataTableLogsQuery(ctx, query.GetLogs())
	case *dashboards.DataTable_Query_Metrics:
		return flattenDataTableMetricsQuery(ctx, query.GetMetrics())
	case *dashboards.DataTable_Query_Spans:
		return flattenDataTableSpansQuery(ctx, query.GetSpans())
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Data Table Query", "unknown data table query type")}
	}
}

func flattenDataTableLogsQuery(ctx context.Context, logs *dashboards.DataTable_LogsQuery) (*DataTableQueryModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	filters, diags := flattenLogsFilters(ctx, logs.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	grouping, diags := flattenDataTableLogsQueryGrouping(ctx, logs.GetGrouping())
	if diags.HasError() {
		return nil, diags
	}

	return &DataTableQueryModel{
		Logs: &DataTableQueryLogsModel{
			LuceneQuery: wrapperspbStringToTypeString(logs.GetLuceneQuery().GetValue()),
			Filters:     filters,
			Grouping:    grouping,
		},
	}, nil
}

func flattenDataTableLogsQueryGrouping(ctx context.Context, grouping *dashboards.DataTable_LogsQuery_Grouping) (*DataTableLogsQueryGroupingModel, diag.Diagnostics) {
	if grouping == nil {
		return nil, nil
	}

	aggregations, diags := flattenGroupingAggregations(ctx, grouping.GetAggregations())
	if diags.HasError() {
		return nil, diags
	}

	return &DataTableLogsQueryGroupingModel{
		Aggregations: aggregations,
		GroupBy:      wrappedStringSliceToTypeStringList(grouping.GetGroupBy()),
	}, nil
}

func flattenGroupingAggregations(ctx context.Context, aggregations []*dashboards.DataTable_LogsQuery_Aggregation) (types.List, diag.Diagnostics) {
	if len(aggregations) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: groupingAggregationModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	aggregationElements := make([]attr.Value, 0, len(aggregations))
	for _, aggregation := range aggregations {
		flattenedAggregation, dg := flattenGroupingAggregation(aggregation)
		if dg != nil {
			diagnostics.Append(dg)
			continue
		}
		aggregationElement, diags := types.ObjectValueFrom(ctx, groupingAggregationModelAttr(), flattenedAggregation)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		aggregationElements = append(aggregationElements, aggregationElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: groupingAggregationModelAttr()}, aggregationElements), diagnostics
}

func flattenGroupingAggregation(dataTableAggregation *dashboards.DataTable_LogsQuery_Aggregation) (*DataTableLogsAggregationModel, diag.Diagnostic) {
	aggregation, dg := flattenLogsAggregation(dataTableAggregation.GetAggregation())
	if dg != nil {
		return nil, dg
	}

	return &DataTableLogsAggregationModel{
		ID:          wrapperspbStringToTypeString(dataTableAggregation.GetId()),
		Name:        wrapperspbStringToTypeString(dataTableAggregation.GetName()),
		IsVisible:   wrapperspbBoolToTypeBool(dataTableAggregation.GetIsVisible()),
		Aggregation: aggregation,
	}, nil
}

func flattenDataTableMetricsQuery(ctx context.Context, metrics *dashboards.DataTable_MetricsQuery) (*DataTableQueryModel, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	filters, diags := flattenMetricsFilters(ctx, metrics.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	return &DataTableQueryModel{
		Metrics: &DataTableQueryMetricsModel{
			PromqlQuery: wrapperspbStringToTypeString(metrics.GetPromqlQuery().GetValue()),
			Filters:     filters,
		},
	}, nil
}

func flattenDataTableSpansQuery(ctx context.Context, spans *dashboards.DataTable_SpansQuery) (*DataTableQueryModel, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	filters, diags := flattenSpansFilters(ctx, spans.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	grouping, diags := flattenDataTableSpansQueryGrouping(ctx, spans.GetGrouping())
	if diags.HasError() {
		return nil, diags
	}

	return &DataTableQueryModel{
		Spans: &DataTableQuerySpansModel{
			LuceneQuery: wrapperspbStringToTypeString(spans.GetLuceneQuery().GetValue()),
			Filters:     filters,
			Grouping:    grouping,
		},
	}, nil
}

func flattenDataTableSpansQueryGrouping(ctx context.Context, grouping *dashboards.DataTable_SpansQuery_Grouping) (*DataTableSpansQueryGroupingModel, diag.Diagnostics) {
	if grouping == nil {
		return nil, nil
	}

	aggregations, diags := flattenDataTableSpansQueryAggregations(ctx, grouping.GetAggregations())
	if diags.HasError() {
		return nil, diags
	}

	groupBy, diags := flattenSpansFields(ctx, grouping.GetGroupBy())
	return &DataTableSpansQueryGroupingModel{
		Aggregations: aggregations,
		GroupBy:      groupBy,
	}, nil
}

func flattenDataTableSpansQueryAggregations(ctx context.Context, aggregations []*dashboards.DataTable_SpansQuery_Aggregation) (types.List, diag.Diagnostics) {
	if len(aggregations) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: spansAggregationModelAttr()}), nil
	}
	var diagnostics diag.Diagnostics
	aggregationElements := make([]attr.Value, 0, len(aggregations))
	for _, aggregation := range aggregations {
		flattenedAggregation, dg := flattenDataTableSpansQueryAggregation(aggregation)
		if dg != nil {
			diagnostics.Append(dg)
			continue
		}
		aggregationElement, diags := types.ObjectValueFrom(ctx, spansAggregationModelAttr(), flattenedAggregation)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		aggregationElements = append(aggregationElements, aggregationElement)
	}
	return types.ListValueMust(types.ObjectType{AttrTypes: spansAggregationModelAttr()}, aggregationElements), diagnostics
}

func flattenDataTableSpansQueryAggregation(spanAggregation *dashboards.DataTable_SpansQuery_Aggregation) (*DataTableSpansAggregationModel, diag.Diagnostic) {
	aggregation, dg := flattenSpansAggregation(spanAggregation.GetAggregation())
	if dg != nil {
		return nil, dg
	}

	return &DataTableSpansAggregationModel{
		ID:          wrapperspbStringToTypeString(spanAggregation.GetId()),
		Name:        wrapperspbStringToTypeString(spanAggregation.GetName()),
		IsVisible:   wrapperspbBoolToTypeBool(spanAggregation.GetIsVisible()),
		Aggregation: aggregation,
	}, nil
}

func flattenSpansAggregation(aggregation *dashboards.SpansAggregation) (*SpansAggregationModel, diag.Diagnostic) {
	switch aggregation := aggregation.GetAggregation().(type) {
	case *dashboards.SpansAggregation_MetricAggregation_:
		return &SpansAggregationModel{
			Type:            types.StringValue("metric"),
			AggregationType: types.StringValue(dashboardProtoToSchemaSpansAggregationMetricAggregationType[aggregation.MetricAggregation.GetAggregationType()]),
			Field:           types.StringValue(dashboardProtoToSchemaSpansAggregationMetricField[aggregation.MetricAggregation.GetMetricField()]),
		}, nil
	case *dashboards.SpansAggregation_DimensionAggregation_:
		return &SpansAggregationModel{
			Type:            types.StringValue("dimension"),
			AggregationType: types.StringValue(dashboardProtoToSchemaSpansAggregationDimensionAggregationType[aggregation.DimensionAggregation.GetAggregationType()]),
			Field:           types.StringValue(dashboardSchemaToProtoSpansAggregationDimensionField[aggregation.DimensionAggregation.GetDimensionField()]),
		}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Error Flatten Span Aggregation", fmt.Sprintf("unknown aggregation type %T", aggregation))
	}
}

func flattenDataTableColumns(ctx context.Context, columns []*dashboards.DataTable_Column) (types.List, diag.Diagnostics) {
	if len(columns) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dataTableColumnModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	columnElements := make([]attr.Value, 0, len(columns))
	for _, column := range columns {
		flattenedColumn := flattenDataTableColumn(column)
		columnElement, diags := types.ObjectValueFrom(ctx, dataTableColumnModelAttr(), flattenedColumn)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		columnElements = append(columnElements, columnElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: dataTableColumnModelAttr()}, columnElements), diagnostics
}

func flattenDataTableColumn(column *dashboards.DataTable_Column) *DataTableColumnModel {
	if column == nil {
		return nil
	}
	return &DataTableColumnModel{
		Field: wrapperspbStringToTypeString(column.GetField()),
		Width: wrapperspbInt32ToTypeInt64(column.GetWidth()),
	}
}

func flattenOrderBy(orderBy *dashboards.OrderingField) *OrderByModel {
	if orderBy == nil {
		return nil
	}
	return &OrderByModel{
		Field:          wrapperspbStringToTypeString(orderBy.GetField()),
		OrderDirection: types.StringValue(dashboardOrderDirectionProtoToSchema[orderBy.GetOrderDirection()]),
	}
}

func flattenGauge(ctx context.Context, gauge *dashboards.Gauge) (*WidgetDefinitionModel, diag.Diagnostics) {
	if gauge == nil {
		return nil, nil
	}

	query, diags := flattenGaugeQueries(ctx, gauge.GetQuery())
	if diags != nil {
		return nil, diags
	}

	return &WidgetDefinitionModel{
		Gauge: &GaugeModel{
			Query:        query,
			Min:          wrapperspbDoubleToTypeFloat64(gauge.GetMin()),
			Max:          wrapperspbDoubleToTypeFloat64(gauge.GetMax()),
			ShowInnerArc: wrapperspbBoolToTypeBool(gauge.GetShowInnerArc()),
			ShowOuterArc: wrapperspbBoolToTypeBool(gauge.GetShowOuterArc()),
			Unit:         types.StringValue(dashboardProtoToSchemaGaugeUnit[gauge.GetUnit()]),
		},
	}, nil
}

func flattenGaugeQueries(ctx context.Context, query *dashboards.Gauge_Query) (*GaugeQueryModel, diag.Diagnostics) {
	switch query.GetValue().(type) {
	case *dashboards.Gauge_Query_Metrics:
		return flattenGaugeQueryMetrics(ctx, query.GetMetrics())
	case *dashboards.Gauge_Query_Logs:
		return flattenGaugeQueryLogs(ctx, query.GetLogs())
	case *dashboards.Gauge_Query_Spans:
		return flattenGaugeQuerySpans(ctx, query.GetSpans())
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Gauge Query", fmt.Sprintf("unknown query type %T", query))}
	}
}

func flattenGaugeQueryMetrics(ctx context.Context, metrics *dashboards.Gauge_MetricsQuery) (*GaugeQueryModel, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	filters, diags := flattenMetricsFilters(ctx, metrics.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	return &GaugeQueryModel{
		Metrics: &GaugeQueryMetricsModel{
			PromqlQuery: wrapperspbStringToTypeString(metrics.GetPromqlQuery().GetValue()),
			Aggregation: types.StringValue(dashboardProtoToSchemaGaugeAggregation[metrics.GetAggregation()]),
			Filters:     filters,
		},
	}, nil
}

func flattenGaugeQueryLogs(ctx context.Context, logs *dashboards.Gauge_LogsQuery) (*GaugeQueryModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	filters, diags := flattenLogsFilters(ctx, logs.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	logsAggregation, dg := flattenLogsAggregation(logs.GetLogsAggregation())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &GaugeQueryModel{
		Logs: &GaugeQueryLogsModel{
			LuceneQuery:     wrapperspbStringToTypeString(logs.GetLuceneQuery().GetValue()),
			LogsAggregation: logsAggregation,
			Aggregation:     types.StringValue(dashboardProtoToSchemaGaugeAggregation[logs.GetAggregation()]),
			Filters:         filters,
		},
	}, nil
}

func flattenGaugeQuerySpans(ctx context.Context, spans *dashboards.Gauge_SpansQuery) (*GaugeQueryModel, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	filters, diags := flattenSpansFilters(ctx, spans.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	spansAggregation, dg := flattenSpansAggregation(spans.GetSpansAggregation())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &GaugeQueryModel{
		Spans: &GaugeQuerySpansModel{
			LuceneQuery:      wrapperspbStringToTypeString(spans.GetLuceneQuery().GetValue()),
			Filters:          filters,
			Aggregation:      types.StringValue(dashboardProtoToSchemaGaugeAggregation[spans.GetAggregation()]),
			SpansAggregation: spansAggregation,
		},
	}, nil
}

func flattenPieChart(ctx context.Context, pieChart *dashboards.PieChart) (*WidgetDefinitionModel, diag.Diagnostics) {
	if pieChart == nil {
		return nil, nil
	}

	query, diags := flattenPieChartQueries(ctx, pieChart.GetQuery())
	if diags != nil {
		return nil, diags
	}

	return &WidgetDefinitionModel{
		PieChart: &PieChartModel{
			Query:              query,
			MaxSlicesPerChart:  wrapperspbInt32ToTypeInt64(pieChart.GetMaxSlicesPerChart()),
			MinSlicePercentage: wrapperspbInt32ToTypeInt64(pieChart.GetMinSlicePercentage()),
			StackDefinition:    flattenPieChartStackDefinition(pieChart.GetStackDefinition()),
			LabelDefinition:    flattenPieChartLabelDefinition(pieChart.GetLabelDefinition()),
			ShowLegend:         wrapperspbBoolToTypeBool(pieChart.GetShowLegend()),
			GroupNameTemplate:  wrapperspbStringToTypeString(pieChart.GetGroupNameTemplate()),
			Unit:               types.StringValue(dashboardProtoToSchemaUnit[pieChart.GetUnit()]),
		},
	}, nil
}

func flattenPieChartQueries(ctx context.Context, query *dashboards.PieChart_Query) (*PieChartQueryModel, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}

	switch query.GetValue().(type) {
	case *dashboards.PieChart_Query_Metrics:
		return flattenPieChartQueryMetrics(ctx, query.GetMetrics())
	case *dashboards.PieChart_Query_Logs:
		return flattenPieChartQueryLogs(ctx, query.GetLogs())
	case *dashboards.PieChart_Query_Spans:
		return flattenPieChartQuerySpans(ctx, query.GetSpans())
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Pie Chart Query", fmt.Sprintf("unknown query type %T", query))}
	}
}

func flattenPieChartStackDefinition(stackDefinition *dashboards.PieChart_StackDefinition) *PieChartStackDefinitionModel {
	if stackDefinition == nil {
		return nil
	}

	return &PieChartStackDefinitionModel{
		MaxSlicesPerStack: wrapperspbInt32ToTypeInt64(stackDefinition.GetMaxSlicesPerStack()),
		StackNameTemplate: wrapperspbStringToTypeString(stackDefinition.GetStackNameTemplate()),
	}
}

func flattenPieChartLabelDefinition(labelDefinition *dashboards.PieChart_LabelDefinition) *LabelDefinitionModel {
	if labelDefinition == nil {
		return nil
	}
	return &LabelDefinitionModel{
		LabelSource:    types.StringValue(dashboardProtoToSchemaPieChartLabelSource[labelDefinition.GetLabelSource()]),
		IsVisible:      wrapperspbBoolToTypeBool(labelDefinition.GetIsVisible()),
		ShowName:       wrapperspbBoolToTypeBool(labelDefinition.GetShowName()),
		ShowValue:      wrapperspbBoolToTypeBool(labelDefinition.GetShowValue()),
		ShowPercentage: wrapperspbBoolToTypeBool(labelDefinition.GetShowPercentage()),
	}
}

func flattenPieChartQueryMetrics(ctx context.Context, metrics *dashboards.PieChart_MetricsQuery) (*PieChartQueryModel, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	filters, diags := flattenMetricsFilters(ctx, metrics.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	return &PieChartQueryModel{
		Metrics: &PieChartQueryMetricsModel{
			PromqlQuery:      wrapperspbStringToTypeString(metrics.GetPromqlQuery().GetValue()),
			Filters:          filters,
			GroupNames:       wrappedStringSliceToTypeStringList(metrics.GetGroupNames()),
			StackedGroupName: wrapperspbStringToTypeString(metrics.GetStackedGroupName()),
		},
	}, nil
}

func flattenPieChartQueryLogs(ctx context.Context, logs *dashboards.PieChart_LogsQuery) (*PieChartQueryModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	filters, diags := flattenLogsFilters(ctx, logs.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	aggregation, dg := flattenLogsAggregation(logs.GetAggregation())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &PieChartQueryModel{
		Logs: &PieChartQueryLogsModel{
			LuceneQuery:      wrapperspbStringToTypeString(logs.GetLuceneQuery().GetValue()),
			Aggregation:      aggregation,
			Filters:          filters,
			GroupNames:       wrappedStringSliceToTypeStringList(logs.GetGroupNames()),
			StackedGroupName: wrapperspbStringToTypeString(logs.GetStackedGroupName()),
		},
	}, nil
}

func flattenPieChartQuerySpans(ctx context.Context, spans *dashboards.PieChart_SpansQuery) (*PieChartQueryModel, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	filters, diags := flattenSpansFilters(ctx, spans.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	aggregation, dg := flattenSpansAggregation(spans.GetAggregation())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	stackedGroupName, dg := flattenSpansField(spans.GetStackedGroupName())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	groupNames, diags := flattenSpansFields(ctx, spans.GetGroupNames())
	if diags.HasError() {
		return nil, diags
	}

	return &PieChartQueryModel{
		Spans: &PieChartQuerySpansModel{
			LuceneQuery:      wrapperspbStringToTypeString(spans.GetLuceneQuery().GetValue()),
			Filters:          filters,
			Aggregation:      aggregation,
			GroupNames:       groupNames,
			StackedGroupName: stackedGroupName,
		},
	}, nil
}

func flattenBarChart(ctx context.Context, barChart *dashboards.BarChart) (*WidgetDefinitionModel, diag.Diagnostics) {
	if barChart == nil {
		return nil, nil
	}

	query, diags := flattenBarChartQuery(ctx, barChart.GetQuery())
	if diags != nil {
		return nil, diags
	}

	colorsBy, dg := flattenBarChartColorsBy(barChart.GetColorsBy())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &WidgetDefinitionModel{
		BarChart: &BarChartModel{
			Query:             query,
			MaxBarsPerChart:   wrapperspbInt32ToTypeInt64(barChart.GetMaxBarsPerChart()),
			GroupNameTemplate: wrapperspbStringToTypeString(barChart.GetGroupNameTemplate()),
			StackDefinition:   flattenBarChartStackDefinition(barChart.GetStackDefinition()),
			ScaleType:         types.StringValue(dashboardProtoToSchemaScaleType[barChart.GetScaleType()]),
			ColorsBy:          colorsBy,
		},
	}, nil
}

func flattenBarChartQuery(ctx context.Context, query *dashboards.BarChart_Query) (*BarChartQueryModel, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}

	switch query.GetValue().(type) {
	case *dashboards.BarChart_Query_Logs:
		return flattenBarChartQueryLogs(ctx, query.GetLogs())
	case *dashboards.BarChart_Query_Spans:
		return flattenBarChartQuerySpans(ctx, query.GetSpans())
	case *dashboards.BarChart_Query_Metrics:
		return flattenBarChartQueryMetrics(ctx, query.GetMetrics())
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten BarChart Query", fmt.Sprintf("unknown bar chart query type: %T", query.GetValue()))}
	}
}

func flattenBarChartQueryLogs(ctx context.Context, logs *dashboards.BarChart_LogsQuery) (*BarChartQueryModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	filters, diags := flattenLogsFilters(ctx, logs.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	aggregation, dg := flattenLogsAggregation(logs.GetAggregation())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &BarChartQueryModel{
		Logs: &BarChartQueryLogsModel{
			LuceneQuery:      wrapperspbStringToTypeString(logs.GetLuceneQuery().GetValue()),
			Filters:          filters,
			Aggregation:      aggregation,
			GroupNames:       wrappedStringSliceToTypeStringList(logs.GetGroupNames()),
			StackedGroupName: wrapperspbStringToTypeString(logs.GetStackedGroupName()),
		},
	}, nil
}

func flattenBarChartQuerySpans(ctx context.Context, spans *dashboards.BarChart_SpansQuery) (*BarChartQueryModel, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	filters, diags := flattenSpansFilters(ctx, spans.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	aggregation, dg := flattenSpansAggregation(spans.GetAggregation())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	groupNapes, diags := flattenSpansFields(ctx, spans.GetGroupNames())
	if diags.HasError() {
		return nil, diags
	}

	stackedGroupName, dg := flattenSpansField(spans.GetStackedGroupName())
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &BarChartQueryModel{
		Spans: &BarChartQuerySpansModel{
			LuceneQuery:      wrapperspbStringToTypeString(spans.GetLuceneQuery().GetValue()),
			Aggregation:      aggregation,
			Filters:          filters,
			GroupNames:       groupNapes,
			StackedGroupName: stackedGroupName,
		},
	}, nil
}

func flattenBarChartQueryMetrics(ctx context.Context, metrics *dashboards.BarChart_MetricsQuery) (*BarChartQueryModel, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	filters, diags := flattenMetricsFilters(ctx, metrics.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	return &BarChartQueryModel{
		Metrics: &BarChartQueryMetricsModel{
			PromqlQuery:      wrapperspbStringToTypeString(metrics.GetPromqlQuery().GetValue()),
			Filters:          filters,
			GroupNames:       wrappedStringSliceToTypeStringList(metrics.GetGroupNames()),
			StackedGroupName: wrapperspbStringToTypeString(metrics.GetStackedGroupName()),
		},
	}, nil
}

func flattenBarChartStackDefinition(stackDefinition *dashboards.BarChart_StackDefinition) *BarChartStackDefinitionModel {
	if stackDefinition == nil {
		return nil
	}

	return &BarChartStackDefinitionModel{
		MaxSlicesPerBar:   wrapperspbInt32ToTypeInt64(stackDefinition.GetMaxSlicesPerBar()),
		StackNameTemplate: wrapperspbStringToTypeString(stackDefinition.GetStackNameTemplate()),
	}
}

func flattenBarChartColorsBy(colorsBy *dashboards.BarChart_ColorsBy) (types.String, diag.Diagnostic) {
	if colorsBy == nil {
		return types.StringNull(), nil
	}
	switch colorsBy.GetValue().(type) {
	case *dashboards.BarChart_ColorsBy_GroupBy:
		return types.StringValue("group_by"), nil
	case *dashboards.BarChart_ColorsBy_Stack:
		return types.StringValue("stack"), nil
	default:
		return types.StringNull(), diag.NewErrorDiagnostic("", fmt.Sprintf("unknown colors by type %T", colorsBy))
	}
}

func flattenDashboardVariables(ctx context.Context, variables []*dashboards.Variable) (types.List, diag.Diagnostics) {
	if len(variables) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dashboardsVariablesModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	variablesElements := make([]attr.Value, 0, len(variables))
	for _, variable := range variables {
		flattenedVariable, diags := flattenDashboardVariable(variable)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}

		variablesElement, diags := types.ObjectValueFrom(ctx, dashboardsVariablesModelAttr(), flattenedVariable)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		variablesElements = append(variablesElements, variablesElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: dashboardsVariablesModelAttr()}, variablesElements), diagnostics
}

func flattenDashboardVariable(variable *dashboards.Variable) (*DashboardVariableModel, diag.Diagnostics) {
	if variable == nil {
		return nil, nil
	}

	definition, diags := flattenDashboardVariableDefinition(variable.GetDefinition())
	if diags.HasError() {
		return nil, diags
	}

	return &DashboardVariableModel{
		Name:        wrapperspbStringToTypeString(variable.GetName()),
		DisplayName: wrapperspbStringToTypeString(variable.GetDisplayName()),
		Definition:  definition,
	}, nil
}

func flattenDashboardVariableDefinition(variableDefinition *dashboards.Variable_Definition) (*DashboardVariableDefinitionModel, diag.Diagnostics) {
	if variableDefinition == nil {
		return nil, nil
	}

	switch variableDefinition.GetValue().(type) {
	case *dashboards.Variable_Definition_Constant:
		return &DashboardVariableDefinitionModel{
			ConstantValue: wrapperspbStringToTypeString(variableDefinition.GetConstant().GetValue()),
		}, nil
	case *dashboards.Variable_Definition_MultiSelect:
		return flattenDashboardVariableDefinitionMultiSelect(variableDefinition.GetMultiSelect())
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Variable Definition", fmt.Sprintf("unknown variable definition type %T", variableDefinition))}
	}
}

func flattenDashboardVariableDefinitionMultiSelect(multiSelect *dashboards.MultiSelect) (*DashboardVariableDefinitionModel, diag.Diagnostics) {
	if multiSelect == nil {
		return nil, nil
	}

	source, diags := flattenDashboardVariableSource(multiSelect.GetSource())
	if diags.HasError() {
		return nil, diags
	}

	selectedValues, diags := flattenDashboardVariableSelectedValues(multiSelect.GetSelection())
	if diags.HasError() {
		return nil, diags
	}

	return &DashboardVariableDefinitionModel{
		MultiSelect: &VariableMultiSelectModel{
			SelectedValues:       selectedValues,
			ValuesOrderDirection: types.StringValue(dashboardOrderDirectionProtoToSchema[multiSelect.GetValuesOrderDirection()]),
			Source:               source,
		},
	}, nil
}

func flattenDashboardVariableSource(source *dashboards.MultiSelect_Source) (*VariableMultiSelectSourceModel, diag.Diagnostics) {
	if source == nil {
		return nil, nil
	}

	switch source.GetValue().(type) {
	case *dashboards.MultiSelect_Source_LogsPath:
		return &VariableMultiSelectSourceModel{
			LogsPath: wrapperspbStringToTypeString(source.GetLogsPath().GetValue()),
		}, nil
	case *dashboards.MultiSelect_Source_MetricLabel:
		return &VariableMultiSelectSourceModel{
			MetricLabel: &MetricMultiSelectSourceModel{
				MetricName: wrapperspbStringToTypeString(source.GetMetricLabel().GetMetricName()),
				Label:      wrapperspbStringToTypeString(source.GetMetricLabel().GetLabel()),
			},
		}, nil
	case *dashboards.MultiSelect_Source_ConstantList:
		return &VariableMultiSelectSourceModel{
			ConstantList: wrappedStringSliceToTypeStringList(source.GetConstantList().GetValues()),
		}, nil
	case *dashboards.MultiSelect_Source_SpanField:
		spansField, dg := flattenSpansField(source.GetSpanField().GetValue())
		if dg != nil {
			return nil, diag.Diagnostics{dg}
		}
		return &VariableMultiSelectSourceModel{
			SpanField: spansField,
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Variable Definition Multi Select Source", fmt.Sprintf("unknown variable definition multi select source type %T", source))}
	}
}

func flattenDashboardVariableSelectedValues(selection *dashboards.MultiSelect_Selection) (types.List, diag.Diagnostics) {
	switch selection.GetValue().(type) {
	case *dashboards.MultiSelect_Selection_List:
		return wrappedStringSliceToTypeStringList(selection.GetList().GetValues()), nil
	case *dashboards.MultiSelect_Selection_All:
		return types.ListNull(types.StringType), nil
	default:
		return types.ListNull(types.StringType), diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dashboard Variable Definition Multi Select Selection", fmt.Sprintf("unknown variable definition multi select selection type %T", selection))}
	}
}

func flattenDashboardFilters(ctx context.Context, filters []*dashboards.Filter) (types.List, diag.Diagnostics) {
	if len(filters) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dashboardsFiltersModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	filtersElements := make([]attr.Value, 0, len(filters))
	for _, filter := range filters {
		flattenedFilter, dg := flattenDashboardFilter(filter)
		if dg != nil {
			diagnostics = append(diagnostics, dg)
			continue
		}
		filterElement, diags := types.ObjectValueFrom(ctx, dashboardsFiltersModelAttr(), flattenedFilter)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		filtersElements = append(filtersElements, filterElement)
	}

	return types.ListValueMust(types.ObjectType{AttrTypes: dashboardsFiltersModelAttr()}, filtersElements), diagnostics
}

func flattenDashboardFilter(filter *dashboards.Filter) (*DashboardFilterModel, diag.Diagnostic) {
	if filter == nil {
		return nil, nil
	}

	source, diags := flattenDashboardFilterSource(filter.GetSource())
	if diags != nil {
		return nil, diags
	}

	return &DashboardFilterModel{
		Source:    source,
		Enabled:   wrapperspbBoolToTypeBool(filter.GetEnabled()),
		Collapsed: wrapperspbBoolToTypeBool(filter.GetCollapsed()),
	}, nil
}

func flattenDashboardFilterSource(source *dashboards.Filter_Source) (*DashboardFilterSourceModel, diag.Diagnostic) {
	if source == nil {
		return nil, nil
	}

	switch source.GetValue().(type) {
	case *dashboards.Filter_Source_Logs:
		logs, dg := flattenDashboardFilterSourceLogs(source.GetLogs())
		if dg != nil {
			return nil, dg
		}
		return &DashboardFilterSourceModel{Logs: logs}, nil
	case *dashboards.Filter_Source_Spans:
		spans, dg := flattenDashboardFilterSourceSpans(source.GetSpans())
		if dg != nil {
			return nil, dg
		}
		return &DashboardFilterSourceModel{Spans: spans}, nil
	case *dashboards.Filter_Source_Metrics:
		metrics, dg := flattenDashboardFilterSourceMetrics(source.GetMetrics())
		if dg != nil {
			return nil, dg
		}
		return &DashboardFilterSourceModel{Metrics: metrics}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Error Flatten Dashboard Filter Source", fmt.Sprintf("unknown filter source type %T", source))
	}
}

func flattenDashboardFilterSourceLogs(logs *dashboards.Filter_LogsFilter) (*FilterSourceLogsModel, diag.Diagnostic) {
	if logs == nil {
		return nil, nil
	}

	operator, dg := flattenFilterOperator(logs.GetOperator())
	if dg != nil {
		return nil, dg
	}

	return &FilterSourceLogsModel{
		Field:    wrapperspbStringToTypeString(logs.GetField()),
		Operator: operator,
	}, nil
}

func flattenDashboardFilterSourceSpans(spans *dashboards.Filter_SpansFilter) (*FilterSourceSpansModel, diag.Diagnostic) {
	if spans == nil {
		return nil, nil
	}

	field, dg := flattenSpansField(spans.GetField())
	if dg != nil {
		return nil, dg
	}

	operator, dg := flattenFilterOperator(spans.GetOperator())
	if dg != nil {
		return nil, dg
	}

	return &FilterSourceSpansModel{
		Field:    field,
		Operator: operator,
	}, nil
}

func flattenDashboardFilterSourceMetrics(metrics *dashboards.Filter_MetricsFilter) (*FilterSourceMetricsModel, diag.Diagnostic) {
	if metrics == nil {
		return nil, nil
	}

	operator, dg := flattenFilterOperator(metrics.GetOperator())
	if dg != nil {
		return nil, dg
	}

	return &FilterSourceMetricsModel{
		MetricName:  wrapperspbStringToTypeString(metrics.GetMetric()),
		MetricLabel: wrapperspbStringToTypeString(metrics.GetLabel()),
		Operator:    operator,
	}, nil
}

func flattenDashboardTimeFrame(d *dashboards.Dashboard) *DashboardTimeFrameModel {
	switch d.GetTimeFrame().(type) {
	case *dashboards.Dashboard_AbsoluteTimeFrame:
		return flattenAbsoluteDashboardTimeFrame(d.GetAbsoluteTimeFrame())
	case *dashboards.Dashboard_RelativeTimeFrame:
		return flattenRelativeDashboardTimeFrame(d.GetRelativeTimeFrame())
	default:
		return nil
	}
}

func flattenAbsoluteDashboardTimeFrame(timeFrame *dashboards.TimeFrame) *DashboardTimeFrameModel {
	return &DashboardTimeFrameModel{
		Absolute: &DashboardTimeFrameAbsoluteModel{
			From: types.StringValue(timeFrame.GetFrom().String()),
			To:   types.StringValue(timeFrame.GetTo().String()),
		},
	}
}

func flattenRelativeDashboardTimeFrame(timeFrame *durationpb.Duration) *DashboardTimeFrameModel {
	return &DashboardTimeFrameModel{
		Relative: &DashboardTimeFrameRelativeModel{
			Duration: types.StringValue(timeFrame.String()),
		},
	}
}

func (r *DashboardResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DashboardResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed Dashboard value from Coralogix
	id := state.ID.ValueString()
	log.Printf("[INFO] Reading Dashboard: %s", id)
	getDashboardResp, err := r.client.GetDashboard(ctx, &dashboards.GetDashboardRequest{DashboardId: wrapperspb.String(id)})
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if status.Code(err) == codes.NotFound {
			state.ID = types.StringNull()
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Dashboard %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Dashboard",
				handleRpcErrorNewFramework(err, "Dashboard"),
			)
		}
		return
	}
	log.Printf("[INFO] Received Dashboard: %#v", getDashboardResp)

	flattenedDashboard, diags := flattenDashboard(ctx, state, getDashboardResp.GetDashboard())
	if diags != nil {
		resp.Diagnostics.Append(diags...)
		return
	}
	state = *flattenedDashboard

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *DashboardResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan DashboardResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	dashboard, diags := extractDashboard(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	log.Printf("[INFO] Updating Dashboard: %#v", *dashboard)
	_, err := r.client.UpdateDashboard(ctx, &dashboards.ReplaceDashboardRequest{Dashboard: dashboard})
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		resp.Diagnostics.AddError(
			"Error updating Dashboard",
			"Could not update Dashboard, unexpected error: "+err.Error(),
		)
		return
	}

	getDashboardReq := &dashboards.GetDashboardRequest{
		DashboardId: dashboard.GetId(),
	}
	getDashboardResp, err := r.client.GetDashboard(ctx, getDashboardReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		resp.Diagnostics.AddError(
			"Error getting Dashboard",
			"Could not create Dashboard, unexpected error: "+err.Error(),
		)
		return
	}

	updateDashboardRespStr, _ := jsm.MarshalToString(getDashboardResp.GetDashboard())
	log.Printf("[INFO] Submitted updated Dashboard: %#v", updateDashboardRespStr)

	flattenedDashboard, diags := flattenDashboard(ctx, plan, getDashboardResp.GetDashboard())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	plan = *flattenedDashboard

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *DashboardResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DashboardResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	log.Printf("[INFO] Deleting Dashboard %s", id)
	if _, err := r.client.DeleteDashboard(ctx, &dashboards.DeleteDashboardRequest{DashboardId: wrapperspb.String(id)}); err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting Dashboard %s", state.ID.ValueString()),
			handleRpcErrorNewFramework(err, "Dashboard"),
		)
		return
	}
	log.Printf("[INFO] Dashboard %s deleted", id)
}

func (r *DashboardResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.Dashboards()
}
