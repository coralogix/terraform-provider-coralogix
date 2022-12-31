package coralogix

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"terraform-provider-coralogix/coralogix/clientset"
	dashboardv1 "terraform-provider-coralogix/coralogix/clientset/grpc/com/coralogix/coralogix-dashboards"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	dashboardSchemaOrderDirectionToProtoOrderDirection = map[string]string{
		"Unspecified": "ORDER_DIRECTION_UNSPECIFIED",
		"Asc":         "ORDER_DIRECTION_ASC",
		"Desc":        "ORDER_DIRECTION_DESC",
	}
	dashboardProtoOrderDirectionToSchemaOrderDirection = reverseMapStrings(dashboardSchemaOrderDirectionToProtoOrderDirection)
	dashboardValidOrderDirection                       = getKeysStrings(dashboardSchemaOrderDirectionToProtoOrderDirection)
	dashboardSchemaRowStyleToProtoRowStyle             = map[string]string{
		"Unspecified": "ROW_STYLE_UNSPECIFIED",
		"One_Line":    "ROW_STYLE_ONE_LINE",
		"Two_Line":    "ROW_STYLE_TWO_LINE",
		"Condensed":   "ROW_STYLE_CONDENSED",
		"Json":        "ROW_STYLE_JSON",
	}
	dashboardProtoRowStyleToSchemaRowStyle         = reverseMapStrings(dashboardSchemaRowStyleToProtoRowStyle)
	dashboardValidRowStyle                         = getKeysStrings(dashboardSchemaRowStyleToProtoRowStyle)
	dashboardSchemaLegendColumnToProtoLegendColumn = map[string]string{
		"Unspecified": "LEGEND_COLUMN_UNSPECIFIED",
		"Min":         "LEGEND_COLUMN_MIN",
		"Max":         "LEGEND_COLUMN_MAX",
		"Sum":         "LEGEND_COLUMN_SUM",
		"Avg":         "LEGEND_COLUMN_AVG",
		"Last":        "LEGEND_COLUMN_LAST",
	}
	dashboardProtoLegendColumnToSchemaLegendColumn = reverseMapStrings(dashboardSchemaLegendColumnToProtoLegendColumn)
	dashboardValidLegendColumn                     = getKeysStrings(dashboardSchemaLegendColumnToProtoLegendColumn)
)

func resourceCoralogixDashboard() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCoralogixDashboardCreate,
		ReadContext:   resourceCoralogixDashboardRead,
		UpdateContext: resourceCoralogixDashboardUpdate,
		DeleteContext: resourceCoralogixDashboardDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(60 * time.Second),
			Read:   schema.DefaultTimeout(30 * time.Second),
			Update: schema.DefaultTimeout(60 * time.Second),
			Delete: schema.DefaultTimeout(30 * time.Second),
		},

		Schema: DashboardSchema(),

		Description: "Coralogix Dashboard. Api-key is required for this resource.",
	}
}

func resourceCoralogixDashboardCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	dashboard, diags := extractDashboard(d)
	if diags != nil {
		return diags
	}
	createDashboardRequest := &dashboardv1.CreateDashboardRequest{
		Dashboard: dashboard,
	}

	log.Printf("[INFO] Creating new dashboard: %#v", createDashboardRequest)
	DashboardResp, err := meta.(*clientset.ClientSet).Dashboards().CreateDashboard(ctx, createDashboardRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcError(err, "dashboard")
	}

	Dashboard := DashboardResp.ProtoReflect()
	log.Printf("[INFO] Submitted new dashboard: %#v", Dashboard)
	d.SetId(createDashboardRequest.GetDashboard().GetId().GetValue())

	return resourceCoralogixDashboardRead(ctx, d, meta)
}

func extractDashboard(d *schema.ResourceData) (*dashboardv1.Dashboard, diag.Diagnostics) {
	id := expandUUID(d.Id())
	name := wrapperspb.String(d.Get("name").(string))
	description := wrapperspb.String(d.Get("description").(string))
	layout, diags := expandLayout(d.Get("layout"))
	if diags != nil {
		return nil, diags
	}
	variables := expandVariables(d.Get("variables"))
	return &dashboardv1.Dashboard{
		Id:          id,
		Name:        name,
		Description: description,
		Layout:      layout,
		Variables:   variables,
	}, nil
}

func expandLayout(v interface{}) (*dashboardv1.Layout, diag.Diagnostics) {
	var m map[string]interface{}
	if v == nil {
		return nil, nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil, nil
	} else {
		m = l[0].(map[string]interface{})
	}

	sections, diags := expandSections(m["sections"])
	return &dashboardv1.Layout{
		Sections: sections,
	}, diags

}

func expandSections(v interface{}) ([]*dashboardv1.Section, diag.Diagnostics) {
	if v == nil {
		return nil, nil
	}
	sections := v.([]interface{})
	result := make([]*dashboardv1.Section, 0, len(sections))
	var diags diag.Diagnostics
	for _, s := range sections {
		section, ds := expandSection(s)
		if ds != nil {
			diags = append(diags, ds...)
		}
		result = append(result, section)
	}
	return result, diags
}

func expandSection(v interface{}) (*dashboardv1.Section, diag.Diagnostics) {
	m := v.(map[string]interface{})
	uuid := expandUUID(m["id"])
	rows, diags := expandRows(m["rows"])
	return &dashboardv1.Section{
		Id:   uuid,
		Rows: rows,
	}, diags
}

func expandUUID(v interface{}) *dashboardv1.UUID {
	var id string
	if v == nil || v.(string) == "" {
		id = uuid.NewString()
	} else {
		id = v.(string)
	}
	return &dashboardv1.UUID{Value: id}
}

func expandRows(v interface{}) ([]*dashboardv1.Row, diag.Diagnostics) {
	if v == nil {
		return nil, nil
	}
	rows := v.([]interface{})
	result := make([]*dashboardv1.Row, 0, len(rows))
	var diags diag.Diagnostics
	for _, r := range rows {
		row, ds := expandRow(r)
		if ds != nil {
			diags = append(diags, ds...)
		}
		result = append(result, row)
	}
	return result, diags
}

func expandRow(v interface{}) (*dashboardv1.Row, diag.Diagnostics) {
	m := v.(map[string]interface{})
	uuid := expandUUID(m["id"])
	appearance := expandRowAppearance(m["appearance"])
	widgets, diags := expandWidgets(m["widgets"])
	return &dashboardv1.Row{
		Id:         uuid,
		Appearance: appearance,
		Widgets:    widgets,
	}, diags
}

func expandRowAppearance(v interface{}) *dashboardv1.Row_Appearance {
	var m map[string]interface{}
	if v == nil {
		return nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil
	} else {
		m = l[0].(map[string]interface{})
	}

	height := wrapperspb.Int32(int32(m["height"].(int)))
	return &dashboardv1.Row_Appearance{
		Height: height,
	}
}

func expandWidgets(v interface{}) ([]*dashboardv1.Widget, diag.Diagnostics) {
	if v == nil {
		return nil, nil
	}
	widgets := v.([]interface{})
	result := make([]*dashboardv1.Widget, 0, len(widgets))
	var diags diag.Diagnostics
	for _, w := range widgets {
		widget, err := expandWidget(w)
		if err != nil {
			diags = append(diags, diag.FromErr(err)...)
		}
		result = append(result, widget)
	}
	return result, diags
}

func expandWidget(v interface{}) (*dashboardv1.Widget, error) {
	m := v.(map[string]interface{})
	id := expandUUID(m["id"])
	title := wrapperspb.String(m["title"].(string))
	description := wrapperspb.String(m["description"].(string))
	definition, err := expandWidgetDefinition(m["definition"])
	if err != nil {
		return nil, err
	}
	appearance := expandWidgetAppearance(m["appearance"])
	return &dashboardv1.Widget{
		Id:          id,
		Title:       title,
		Description: description,
		Definition:  definition,
		Appearance:  appearance,
	}, nil
}

func expandWidgetDefinition(v interface{}) (*dashboardv1.Widget_Definition, error) {
	var m map[string]interface{}
	if v == nil {
		return nil, nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil, nil
	} else {
		m = l[0].(map[string]interface{})
	}

	if l, ok := m["line_chart"]; ok && len(l.([]interface{})) != 0 {
		lineChart, err := expandLineChart(l.([]interface{})[0])
		if err != nil {
			return nil, err
		}
		return &dashboardv1.Widget_Definition{
			Value: lineChart,
		}, nil
	} else if l, ok = m["data_table"]; ok && len(l.([]interface{})) != 0 {
		dataTable := expandDataTable(l.([]interface{})[0])
		return &dashboardv1.Widget_Definition{
			Value: dataTable,
		}, nil
	}

	return nil, nil
}

func expandLineChart(v interface{}) (*dashboardv1.Widget_Definition_LineChart, error) {
	m := v.(map[string]interface{})
	query, err := expandLineChartQuery(m["query"])
	if err != nil {
		return nil, err
	}
	legend := expandLegend(m["legend"])
	seriesNameTemplate := wrapperspb.String(m["series_name_template"].(string))
	return &dashboardv1.Widget_Definition_LineChart{
		LineChart: &dashboardv1.LineChart{
			Query:              query,
			Legend:             legend,
			SeriesNameTemplate: seriesNameTemplate,
		},
	}, nil
}

func expandLineChartQuery(v interface{}) (*dashboardv1.LineChart_Query, error) {
	var m map[string]interface{}
	if v == nil {
		return nil, fmt.Errorf("line chart query cannot be empty")
	}
	if l := v.([]interface{}); len(l) == 0 || l[0] == nil {
		return nil, fmt.Errorf("line chart query cannot be empty")
	} else {
		m = l[0].(map[string]interface{})
	}

	if l, ok := m["logs"]; ok && len(l.([]interface{})) != 0 {
		lineChartQueryLogs := expandLineChartQueryLogs(l.([]interface{})[0])
		return &dashboardv1.LineChart_Query{
			Value: lineChartQueryLogs,
		}, nil
	} else if l, ok = m["metrics"]; ok && len(l.([]interface{})) != 0 {
		lineChartQueryMetrics := expandLineChartQueryMetric(l.([]interface{})[0])
		return &dashboardv1.LineChart_Query{
			Value: lineChartQueryMetrics,
		}, nil
	}

	return nil, fmt.Errorf("line chart query cannot be empty")
}

func expandLineChartQueryLogs(v interface{}) *dashboardv1.LineChart_Query_Logs {
	if v == nil {
		return &dashboardv1.LineChart_Query_Logs{}
	}
	m := v.(map[string]interface{})
	luceneQuery := &dashboardv1.LuceneQuery{Value: wrapperspb.String(m["lucene_query"].(string))}
	groupBy := interfaceSliceToWrappedStringSlice(m["group_by"].([]interface{}))
	aggregations := expandAggregations(m["aggregations"])
	return &dashboardv1.LineChart_Query_Logs{
		Logs: &dashboardv1.LineChart_LogsQuery{
			LuceneQuery:  luceneQuery,
			GroupBy:      groupBy,
			Aggregations: aggregations,
		},
	}
}

func expandAggregations(v interface{}) []*dashboardv1.LogsAggregation {
	if v == nil {
		return nil
	}
	aggregations := v.([]interface{})
	result := make([]*dashboardv1.LogsAggregation, 0, len(aggregations))
	for _, a := range aggregations {
		aggregation := expandAggregation(a)
		result = append(result, aggregation)
	}
	return result
}

func expandAggregation(v interface{}) *dashboardv1.LogsAggregation {
	if v == nil {
		return nil
	}
	m := v.(map[string]interface{})

	if l, ok := m["count"]; ok && len(l.([]interface{})) != 0 {
		return &dashboardv1.LogsAggregation{
			Value: &dashboardv1.LogsAggregation_Count_{
				Count: &dashboardv1.LogsAggregation_Count{},
			},
		}
	} else if l, ok = m["count_distinct"]; ok && len(l.([]interface{})) != 0 {
		m = l.([]interface{})[0].(map[string]interface{})
		field := wrapperspb.String(m["field"].(string))
		return &dashboardv1.LogsAggregation{
			Value: &dashboardv1.LogsAggregation_CountDistinct_{
				CountDistinct: &dashboardv1.LogsAggregation_CountDistinct{
					Field: field,
				},
			},
		}
	} else if l, ok = m["sum"]; ok && len(l.([]interface{})) != 0 {
		m = l.([]interface{})[0].(map[string]interface{})
		field := wrapperspb.String(m["field"].(string))
		return &dashboardv1.LogsAggregation{
			Value: &dashboardv1.LogsAggregation_Sum_{
				Sum: &dashboardv1.LogsAggregation_Sum{
					Field: field,
				},
			},
		}
	} else if l, ok = m["average"]; ok && len(l.([]interface{})) != 0 {
		m = l.([]interface{})[0].(map[string]interface{})
		field := wrapperspb.String(m["field"].(string))
		return &dashboardv1.LogsAggregation{
			Value: &dashboardv1.LogsAggregation_Average_{
				Average: &dashboardv1.LogsAggregation_Average{
					Field: field,
				},
			},
		}
	} else if l, ok = m["min"]; ok && len(l.([]interface{})) != 0 {
		m = l.([]interface{})[0].(map[string]interface{})
		field := wrapperspb.String(m["field"].(string))
		return &dashboardv1.LogsAggregation{
			Value: &dashboardv1.LogsAggregation_Min_{
				Min: &dashboardv1.LogsAggregation_Min{
					Field: field,
				},
			},
		}
	} else if l, ok = m["max"]; ok && len(l.([]interface{})) != 0 {
		m = l.([]interface{})[0].(map[string]interface{})
		field := wrapperspb.String(m["field"].(string))
		return &dashboardv1.LogsAggregation{
			Value: &dashboardv1.LogsAggregation_Max_{
				Max: &dashboardv1.LogsAggregation_Max{
					Field: field,
				},
			},
		}
	}

	return nil
}

func expandLineChartQueryMetric(v interface{}) *dashboardv1.LineChart_Query_Metrics {
	if v == nil {
		return &dashboardv1.LineChart_Query_Metrics{}
	}
	m := v.(map[string]interface{})
	promqlQuery := wrapperspb.String(m["promql_query"].(string))
	return &dashboardv1.LineChart_Query_Metrics{
		Metrics: &dashboardv1.LineChart_MetricsQuery{
			PromqlQuery: &dashboardv1.PromQlQuery{
				Value: promqlQuery,
			},
		},
	}
}

func expandLegend(v interface{}) *dashboardv1.Legend {
	var m map[string]interface{}
	if v == nil {
		return nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil
	} else {
		m = l[0].(map[string]interface{})
	}

	isVisible := wrapperspb.Bool(m["is_visible"].(bool))
	columns := expandLegendColumns(m["columns"])

	return &dashboardv1.Legend{
		IsVisible: isVisible,
		Columns:   columns,
	}
}

func expandLegendColumns(v interface{}) []dashboardv1.Legend_LegendColumn {
	if v == nil {
		return nil
	}
	legendColumns := v.([]interface{})
	result := make([]dashboardv1.Legend_LegendColumn, 0, len(legendColumns))
	for _, lc := range legendColumns {
		legend := expandLegendColumn(lc.(string))
		result = append(result, legend)
	}
	return result
}

func expandLegendColumn(legendColumn string) dashboardv1.Legend_LegendColumn {
	legendColumnStr := dashboardSchemaLegendColumnToProtoLegendColumn[legendColumn]
	legendColumnValue := dashboardv1.Legend_LegendColumn_value[legendColumnStr]
	return dashboardv1.Legend_LegendColumn(legendColumnValue)
}

func expandDataTable(v interface{}) *dashboardv1.Widget_Definition_DataTable {
	m := v.(map[string]interface{})
	query := expandDataTableQuery(m["query"])
	resultsPerPage := wrapperspb.Int32(int32(m["results_per_page"].(int)))
	rowStyle := expandRowStyle(m["row_style"].(string))
	columns := expandDataTableColumns(m["columns"])

	return &dashboardv1.Widget_Definition_DataTable{
		DataTable: &dashboardv1.DataTable{
			Query:          query,
			ResultsPerPage: resultsPerPage,
			RowStyle:       rowStyle,
			Columns:        columns,
		},
	}
}

func expandDataTableColumns(v interface{}) []*dashboardv1.DataTable_Column {
	if v == nil {
		return nil
	}
	dataTableColumns := v.([]interface{})
	result := make([]*dashboardv1.DataTable_Column, 0, len(dataTableColumns))
	for _, dtc := range dataTableColumns {
		dataTableColumn := expandDataTableColumn(dtc)
		result = append(result, dataTableColumn)
	}
	return result
}

func expandDataTableColumn(v interface{}) *dashboardv1.DataTable_Column {
	if v == nil {
		return nil
	}
	m := v.(map[string]interface{})

	field := wrapperspb.String(m["field"].(string))
	orderDirection := expandOrderDirection(m["order_direction"].(string))
	return &dashboardv1.DataTable_Column{
		Field:          field,
		OrderDirection: orderDirection,
	}

}

func expandOrderDirection(s string) dashboardv1.OrderDirection {
	orderDirectionStr := dashboardSchemaOrderDirectionToProtoOrderDirection[s]
	orderDirectionValue := dashboardv1.OrderDirection_value[orderDirectionStr]
	return dashboardv1.OrderDirection(orderDirectionValue)
}

func expandRowStyle(s string) dashboardv1.RowStyle {
	rowStyleStr := dashboardSchemaRowStyleToProtoRowStyle[s]
	rowStyleValue := dashboardv1.RowStyle_value[rowStyleStr]
	return dashboardv1.RowStyle(rowStyleValue)
}

func expandDataTableQuery(v interface{}) *dashboardv1.DataTable_Query {
	var m map[string]interface{}
	if v == nil {
		return nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil
	} else {
		m = l[0].(map[string]interface{})
	}
	logsMap := m["logs"].([]interface{})[0].(map[string]interface{})

	luceneQuery := expandLuceneQuery(logsMap["lucene_query"])
	filters := expandSearchFilters(logsMap["filters"])
	return &dashboardv1.DataTable_Query{
		Value: &dashboardv1.DataTable_Query_Logs{
			Logs: &dashboardv1.DataTable_LogsQuery{
				LuceneQuery: luceneQuery,
				Filters:     filters,
			},
		},
	}
}

func expandLuceneQuery(v interface{}) *dashboardv1.LuceneQuery {
	query := v.(string)
	return &dashboardv1.LuceneQuery{
		Value: wrapperspb.String(query),
	}
}

func expandSearchFilters(v interface{}) []*dashboardv1.SearchFilter {
	if v == nil {
		return nil
	}
	filters := v.([]interface{})
	result := make([]*dashboardv1.SearchFilter, 0, len(filters))
	for _, f := range filters {
		filter := expandSearchFilter(f)
		result = append(result, filter)
	}
	return result
}

func expandSearchFilter(v interface{}) *dashboardv1.SearchFilter {
	if v == nil {
		return nil
	}
	m := v.(map[string]interface{})
	name := wrapperspb.String(m["name"].(string))
	values := interfaceSliceToWrappedStringSlice(m["values"].([]interface{}))
	return &dashboardv1.SearchFilter{
		Name:   name,
		Values: values,
	}
}

func expandWidgetAppearance(v interface{}) *dashboardv1.Widget_Appearance {
	var m map[string]interface{}
	if v == nil {
		return nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil
	} else {
		m = l[0].(map[string]interface{})
	}

	width := wrapperspb.Int32(int32(m["width"].(int)))
	return &dashboardv1.Widget_Appearance{
		Width: width,
	}
}

func expandVariables(i interface{}) []*dashboardv1.Variable {
	if i == nil {
		return nil
	}
	variables := i.([]interface{})
	result := make([]*dashboardv1.Variable, 0, len(variables))
	for _, v := range variables {
		variable := expandVariable(v)
		result = append(result, variable)
	}
	return result
}

func expandVariable(v interface{}) *dashboardv1.Variable {
	var m map[string]interface{}
	if v == nil {
		return nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil
	} else {
		m = l[0].(map[string]interface{})
	}
	name := wrapperspb.String(m["name"].(string))
	definition := expandDefinition(m)
	return &dashboardv1.Variable{
		Name:       name,
		Definition: definition,
	}
}

func expandDefinition(m map[string]interface{}) *dashboardv1.Variable_Definition {
	if l, ok := m["constant"]; ok && len(l.([]interface{})) != 0 {
		constant := l.([]interface{})[0].(map[string]interface{})
		value := wrapperspb.String(constant["value"].(string))
		return &dashboardv1.Variable_Definition{
			Value: &dashboardv1.Variable_Definition_Constant{
				Constant: &dashboardv1.Constant{
					Value: value,
				},
			},
		}
	} else if l, ok = m["multi_select"]; ok && len(l.([]interface{})) != 0 {
		multiSelect := l.([]interface{})[0].(map[string]interface{})
		selected := interfaceSliceToWrappedStringSlice(multiSelect["selected"].([]interface{}))
		source := expandSource(m["source"])
		return &dashboardv1.Variable_Definition{
			Value: &dashboardv1.Variable_Definition_MultiSelect{
				MultiSelect: &dashboardv1.MultiSelect{
					Selected: selected,
					Source:   source,
				},
			},
		}
	}
	return nil
}

func expandSource(v interface{}) *dashboardv1.MultiSelect_Source {
	var m map[string]interface{}
	if v == nil {
		return nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil
	} else {
		m = l[0].(map[string]interface{})
	}

	if l, ok := m["logs_path"]; ok && len(l.([]interface{})) != 0 {
		logPath := l.([]interface{})[0].(map[string]interface{})
		value := wrapperspb.String(logPath["value"].(string))
		return &dashboardv1.MultiSelect_Source{
			Value: &dashboardv1.MultiSelect_Source_LogsPath{
				LogsPath: &dashboardv1.MultiSelect_LogsPathSource{
					Value: value,
				},
			},
		}
	} else if l, ok = m["metric_label"]; ok && len(l.([]interface{})) != 0 {
		metricLabel := l.([]interface{})[0].(map[string]interface{})
		metricName := wrapperspb.String(metricLabel["metric_name"].(string))
		label := wrapperspb.String(metricLabel["label"].(string))
		return &dashboardv1.MultiSelect_Source{
			Value: &dashboardv1.MultiSelect_Source_MetricLabel{
				MetricLabel: &dashboardv1.MultiSelect_MetricLabelSource{
					MetricName: metricName,
					Label:      label,
				},
			},
		}
	} else if l, ok = m["constant_list"]; ok && len(l.([]interface{})) != 0 {
		constantList := l.([]interface{})[0].(map[string]interface{})
		values := interfaceSliceToWrappedStringSlice(constantList["values"].([]interface{}))
		return &dashboardv1.MultiSelect_Source{
			Value: &dashboardv1.MultiSelect_Source_ConstantList{
				ConstantList: &dashboardv1.MultiSelect_ConstantListSource{
					Values: values,
				},
			},
		}
	}

	return nil
}

func resourceCoralogixDashboardRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	dashboardId := expandUUID(id)
	log.Printf("[INFO] Reading dashboard %s", id)
	resp, err := meta.(*clientset.ClientSet).Dashboards().GetDashboard(ctx, &dashboardv1.GetDashboardRequest{DashboardId: dashboardId})
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "dashboard", id)
	}

	dashboard := resp.GetDashboard()
	log.Printf("[INFO] Received dashboard: %#v", dashboard)

	return setDashboard(d, dashboard)
}

func setDashboard(d *schema.ResourceData, dashboard *dashboardv1.Dashboard) diag.Diagnostics {
	if err := d.Set("name", dashboard.GetName().GetValue()); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("description", dashboard.GetDescription().GetValue()); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("layout", flattenLayout(dashboard.GetLayout())); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("variables", flattenVariables(dashboard.GetVariables())); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func flattenLayout(layout *dashboardv1.Layout) interface{} {
	sections := flattenSections(layout.GetSections())
	return []interface{}{
		map[string]interface{}{
			"sections": sections,
		},
	}
}

func flattenSections(sections []*dashboardv1.Section) interface{} {
	result := make([]interface{}, 0, len(sections))
	for _, s := range sections {
		section := flattenSection(s)
		result = append(result, section)
	}
	return result
}

func flattenSection(section *dashboardv1.Section) interface{} {
	id := section.GetId().GetValue()
	rows := flattenRows(section.GetRows())
	return map[string]interface{}{
		"id":   id,
		"rows": rows,
	}
}

func flattenRows(rows []*dashboardv1.Row) interface{} {
	result := make([]interface{}, 0, len(rows))
	for _, r := range rows {
		row := flattenRow(r)
		result = append(result, row)
	}
	return result
}

func flattenRow(row *dashboardv1.Row) interface{} {
	id := row.GetId().GetValue()
	appearance := flattenRowAppearance(row.GetAppearance())
	widgets := flattenWidgets(row.GetWidgets())
	return map[string]interface{}{
		"id":         id,
		"appearance": appearance,
		"widgets":    widgets,
	}
}

func flattenRowAppearance(appearance *dashboardv1.Row_Appearance) interface{} {
	return []interface{}{
		map[string]interface{}{
			"height": appearance.GetHeight().GetValue(),
		},
	}
}

func flattenWidgets(widgets []*dashboardv1.Widget) interface{} {
	result := make([]interface{}, 0, len(widgets))
	for _, w := range widgets {
		widget := flattenWidget(w)
		result = append(result, widget)
	}
	return result
}

func flattenWidget(widget *dashboardv1.Widget) interface{} {
	id := widget.GetId().GetValue()
	title := widget.GetTitle().GetValue()
	description := widget.GetDescription().GetValue()
	definition := flattenWidgetDefinition(widget.GetDefinition())
	appearance := flattenWidgetAppearance(widget.GetAppearance())
	return map[string]interface{}{
		"id":          id,
		"title":       title,
		"description": description,
		"definition":  definition,
		"appearance":  appearance,
	}
}

func flattenWidgetDefinition(definition *dashboardv1.Widget_Definition) interface{} {
	var widgetDefinition map[string]interface{}
	switch definitionValue := definition.GetValue().(type) {
	case *dashboardv1.Widget_Definition_LineChart:
		lineChart := flattenLineChart(definitionValue.LineChart)
		widgetDefinition = map[string]interface{}{
			"line_chart": lineChart,
		}
	case *dashboardv1.Widget_Definition_DataTable:
		dataTable := flattenDataTable(definitionValue.DataTable)
		widgetDefinition = map[string]interface{}{
			"data_table": dataTable,
		}
	}

	return []interface{}{
		widgetDefinition,
	}
}

func flattenLineChart(lineChart *dashboardv1.LineChart) interface{} {
	query := flattenLineChartQuery(lineChart.GetQuery())
	legend := flattenLegend(lineChart.GetLegend())
	seriesNameTemplate := lineChart.GetSeriesNameTemplate().GetValue()
	return []interface{}{
		map[string]interface{}{
			"query":                query,
			"legend":               legend,
			"series_name_template": seriesNameTemplate,
		},
	}
}

func flattenLineChartQuery(query *dashboardv1.LineChart_Query) interface{} {
	var queryMap interface{}
	switch queryValue := query.GetValue().(type) {
	case *dashboardv1.LineChart_Query_Logs:
		queryMap = map[string]interface{}{
			"logs": flattenLineChartLogsQuery(queryValue.Logs),
		}
	case *dashboardv1.LineChart_Query_Metrics:
		queryMap = map[string]interface{}{
			"metrics": flattenLineChartMetricsQuery(queryValue.Metrics),
		}
	}

	return []interface{}{
		queryMap,
	}
}

func flattenLineChartLogsQuery(logs *dashboardv1.LineChart_LogsQuery) interface{} {
	luceneQuery := logs.GetLuceneQuery().GetValue().GetValue()
	groupBy := wrappedStringSliceToStringSlice(logs.GetGroupBy())
	aggregations := flattenAggregations(logs.GetAggregations())
	return []interface{}{
		map[string]interface{}{
			"lucene_query": luceneQuery,
			"group_by":     groupBy,
			"aggregations": aggregations,
		},
	}
}

func flattenAggregations(aggregations []*dashboardv1.LogsAggregation) interface{} {
	result := make([]interface{}, 0, len(aggregations))
	for _, a := range aggregations {
		aggregation := flattenAggregation(a)
		result = append(result, aggregation)
	}
	return result
}

func flattenAggregation(aggregation *dashboardv1.LogsAggregation) interface{} {
	switch aggregationValue := aggregation.GetValue().(type) {
	case *dashboardv1.LogsAggregation_Count_:
		return map[string]interface{}{
			"count": []interface{}{
				map[string]interface{}{},
			},
		}
	case *dashboardv1.LogsAggregation_CountDistinct_:
		return map[string]interface{}{
			"count_distinct": []interface{}{
				map[string]interface{}{
					"field": aggregationValue.CountDistinct.GetField().GetValue(),
				},
			},
		}
	case *dashboardv1.LogsAggregation_Sum_:
		return map[string]interface{}{
			"sum": []interface{}{
				map[string]interface{}{
					"field": aggregationValue.Sum.GetField().GetValue(),
				},
			},
		}
	case *dashboardv1.LogsAggregation_Average_:
		return map[string]interface{}{
			"average": []interface{}{
				map[string]interface{}{
					"field": aggregationValue.Average.GetField().GetValue(),
				},
			},
		}
	case *dashboardv1.LogsAggregation_Min_:
		return map[string]interface{}{
			"min": []interface{}{
				map[string]interface{}{
					"field": aggregationValue.Min.GetField().GetValue(),
				},
			},
		}
	case *dashboardv1.LogsAggregation_Max_:
		return map[string]interface{}{
			"max": []interface{}{
				map[string]interface{}{
					"field": aggregationValue.Max.GetField().GetValue(),
				},
			},
		}
	}

	return nil
}

func flattenLineChartMetricsQuery(metrics *dashboardv1.LineChart_MetricsQuery) interface{} {
	promqlQuery := metrics.GetPromqlQuery().GetValue().GetValue()
	return []interface{}{
		map[string]interface{}{
			"promql_query": promqlQuery,
		},
	}
}

func flattenLegend(legend *dashboardv1.Legend) interface{} {
	isVisible := legend.IsVisible.GetValue()
	columns := flattenLegendColumns(legend.GetColumns())
	return []interface{}{
		map[string]interface{}{
			"is_visible": isVisible,
			"columns":    columns,
		},
	}
}

func flattenLegendColumns(columns []dashboardv1.Legend_LegendColumn) interface{} {
	result := make([]string, 0, len(columns))
	for _, c := range columns {
		column := flattenLegendColumn(c)
		result = append(result, column)
	}

	return result
}

func flattenLegendColumn(column dashboardv1.Legend_LegendColumn) string {
	columnStr := dashboardv1.Legend_LegendColumn_name[int32(column)]
	return dashboardProtoLegendColumnToSchemaLegendColumn[columnStr]
}

func flattenDataTable(dataTable *dashboardv1.DataTable) interface{} {
	query := flattenDataTableQuery(dataTable.GetQuery())
	resultsPerPage := dataTable.GetResultsPerPage().GetValue()
	rowStyle := flattenRowStyle(dataTable.GetRowStyle())
	columns := flattenDataTableColumns(dataTable.GetColumns())
	return []interface{}{
		map[string]interface{}{
			"query":            query,
			"results_per_page": resultsPerPage,
			"row_style":        rowStyle,
			"columns":          columns,
		},
	}
}

func flattenDataTableColumns(columns []*dashboardv1.DataTable_Column) interface{} {
	result := make([]interface{}, 0, len(columns))
	for _, c := range columns {
		column := flattenDataTableColumn(c)
		result = append(result, column)
	}

	return result
}

func flattenDataTableColumn(column *dashboardv1.DataTable_Column) interface{} {
	field := column.GetField().GetValue()
	orderDirection := flattenOrderDirection(column.GetOrderDirection())
	return map[string]interface{}{
		"field":           field,
		"order_direction": orderDirection,
	}
}

func flattenOrderDirection(orderDirection dashboardv1.OrderDirection) string {
	orderDirectionStr := dashboardv1.OrderDirection_name[int32(orderDirection)]
	return dashboardProtoOrderDirectionToSchemaOrderDirection[orderDirectionStr]
}

func flattenRowStyle(rowStyle dashboardv1.RowStyle) string {
	rowStyleStr := dashboardv1.RowStyle_name[int32(rowStyle)]
	return dashboardProtoRowStyleToSchemaRowStyle[rowStyleStr]
}

func flattenDataTableQuery(query *dashboardv1.DataTable_Query) interface{} {
	logs := flattenDataTableLogsQuery(query.GetLogs())
	return []interface{}{
		map[string]interface{}{
			"logs": logs,
		},
	}
}

func flattenDataTableLogsQuery(logs *dashboardv1.DataTable_LogsQuery) interface{} {
	luceneQuery := logs.GetLuceneQuery().GetValue().GetValue()
	filters := flattenDataTableFilters(logs.GetFilters())
	return []interface{}{
		map[string]interface{}{
			"lucene_query": luceneQuery,
			"filters":      filters,
		},
	}
}

func flattenDataTableFilters(filters []*dashboardv1.SearchFilter) interface{} {
	result := make([]interface{}, 0, len(filters))
	for _, f := range filters {
		filter := flattenDataTableFilter(f)
		result = append(result, filter)
	}
	return result
}

func flattenDataTableFilter(filter *dashboardv1.SearchFilter) interface{} {
	name := filter.GetName().GetValue()
	values := wrappedStringSliceToStringSlice(filter.GetValues())
	return map[string]interface{}{
		"name":   name,
		"values": values,
	}
}

func flattenWidgetAppearance(appearance *dashboardv1.Widget_Appearance) interface{} {
	return []interface{}{
		map[string]interface{}{
			"width": appearance.GetWidth().GetValue(),
		},
	}
}

func flattenVariables(variables []*dashboardv1.Variable) interface{} {
	result := make([]interface{}, 0, len(variables))
	for _, v := range variables {
		variable := flattenVariable(v)
		result = append(result, variable)
	}
	return result
}

func flattenVariable(variable *dashboardv1.Variable) interface{} {
	name := variable.GetName().GetValue()
	definition := flattenVariableDefinition(variable.GetDefinition())
	return map[string]interface{}{
		"name":       name,
		"definition": definition,
	}
}

func flattenVariableDefinition(definition *dashboardv1.Variable_Definition) interface{} {
	var definitionMap map[string]interface{}
	switch definitionValue := definition.GetValue().(type) {
	case *dashboardv1.Variable_Definition_Constant:
		constant := flattenConstant(definitionValue.Constant)
		definitionMap = map[string]interface{}{
			"constant": constant,
		}
	case *dashboardv1.Variable_Definition_MultiSelect:
		multiSelect := flattenMultiSelect(definitionValue.MultiSelect)
		definitionMap = map[string]interface{}{
			"multi_select": multiSelect,
		}
	}
	return []interface{}{
		definitionMap,
	}
}

func flattenConstant(constant *dashboardv1.Constant) interface{} {
	return []interface{}{
		map[string]interface{}{
			"value": constant.GetValue().GetValue(),
		},
	}
}

func flattenMultiSelect(multiSelect *dashboardv1.MultiSelect) interface{} {
	selected := wrappedStringSliceToStringSlice(multiSelect.GetSelected())
	source := flattenMultiSelectSource(multiSelect.GetSource())
	return []interface{}{
		map[string]interface{}{
			"selected": selected,
			"source":   source,
		},
	}
}

func flattenMultiSelectSource(source *dashboardv1.MultiSelect_Source) interface{} {
	var sourceMap map[string]interface{}
	switch sourceValue := source.GetValue().(type) {
	case *dashboardv1.MultiSelect_Source_LogsPath:
		logsPath := flattenLogPathSource(sourceValue.LogsPath)
		sourceMap = map[string]interface{}{
			"log_path": logsPath,
		}
	case *dashboardv1.MultiSelect_Source_MetricLabel:
		metricLabel := flattenMetricLabelSource(sourceValue.MetricLabel)
		sourceMap = map[string]interface{}{
			"metric_label": metricLabel,
		}
	case *dashboardv1.MultiSelect_Source_ConstantList:
		constantList := flattenConstantListSource(sourceValue.ConstantList)
		sourceMap = map[string]interface{}{
			"constant_list": constantList,
		}
	}
	return []interface{}{
		sourceMap,
	}
}

func flattenLogPathSource(logPath *dashboardv1.MultiSelect_LogsPathSource) interface{} {
	value := logPath.GetValue().GetValue()
	return []interface{}{
		map[string]interface{}{
			"value": value,
		},
	}
}

func flattenMetricLabelSource(metricLabel *dashboardv1.MultiSelect_MetricLabelSource) interface{} {
	metricName := metricLabel.GetMetricName().GetValue()
	label := metricLabel.GetLabel().GetValue()
	return []interface{}{
		map[string]interface{}{
			"metric_name": metricName,
			"label":       label,
		},
	}
}

func flattenConstantListSource(constantList *dashboardv1.MultiSelect_ConstantListSource) interface{} {
	values := wrappedStringSliceToStringSlice(constantList.GetValues())
	return []interface{}{
		map[string]interface{}{
			"values": values,
		},
	}
}

func resourceCoralogixDashboardUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	dashboard, diags := extractDashboard(d)
	if diags != nil {
		return diags
	}
	updateDashboardRequest := &dashboardv1.ReplaceDashboardRequest{
		Dashboard: dashboard,
	}

	log.Printf("[INFO] Updating dashboard: %#v", updateDashboardRequest)
	DashboardResp, err := meta.(*clientset.ClientSet).Dashboards().UpdateDashboard(ctx, updateDashboardRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcError(err, "dashboard")
	}

	Dashboard := DashboardResp.ProtoReflect()
	log.Printf("[INFO] Submitted updated dashboard: %#v", Dashboard)
	d.SetId(updateDashboardRequest.GetDashboard().GetId().GetValue())

	return resourceCoralogixDashboardRead(ctx, d, meta)
}

func resourceCoralogixDashboardDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	log.Printf("[INFO] Deleting dashboard %s\n", id)
	deleteAlertRequest := &dashboardv1.DeleteDashboardRequest{DashboardId: &dashboardv1.UUID{Value: id}}
	_, err := meta.(*clientset.ClientSet).Dashboards().DeleteDashboard(ctx, deleteAlertRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v\n", err)
		return handleRpcErrorWithID(err, "dashboard", id)
	}
	log.Printf("[INFO] dashboard %s deleted\n", id)

	d.SetId("")
	return nil
}

func DashboardSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"name": {
			Type:         schema.TypeString,
			Required:     true,
			ValidateFunc: validation.StringIsNotEmpty,
			Description:  "Dashboard name.",
		},
		"description": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "Dashboard description.",
		},
		"layout": {
			Type:     schema.TypeList,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"sections": {
						Type: schema.TypeList,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"id": {
									Type:     schema.TypeString,
									Computed: true,
								},
								"rows": {
									Type: schema.TypeList,
									Elem: &schema.Resource{
										Schema: map[string]*schema.Schema{
											"id": {
												Type:     schema.TypeString,
												Computed: true,
											},
											"appearance": {
												Type:     schema.TypeList,
												Required: true,
												MaxItems: 1,
												Elem: &schema.Resource{
													Schema: map[string]*schema.Schema{
														"height": {
															Type:     schema.TypeInt,
															Required: true,
														},
													},
												},
											},
											"widgets": {
												Type: schema.TypeList,
												Elem: &schema.Resource{
													Schema: map[string]*schema.Schema{
														"id": {
															Type:     schema.TypeString,
															Computed: true,
														},
														"title": {
															Type:     schema.TypeString,
															Optional: true,
														},
														"description": {
															Type:     schema.TypeString,
															Optional: true,
														},
														"definition": {
															Type:     schema.TypeList,
															MaxItems: 1,
															Elem: &schema.Resource{
																Schema: map[string]*schema.Schema{
																	"line_chart": {
																		Type:     schema.TypeList,
																		MaxItems: 1,
																		Elem: &schema.Resource{
																			Schema: map[string]*schema.Schema{
																				"query": {
																					Type:     schema.TypeList,
																					MaxItems: 1,
																					Elem: &schema.Resource{
																						Schema: map[string]*schema.Schema{
																							"logs": {
																								Type:     schema.TypeList,
																								MaxItems: 1,
																								Elem: &schema.Resource{
																									Schema: map[string]*schema.Schema{
																										"lucene_query": {
																											Type:     schema.TypeString,
																											Optional: true,
																										},
																										"group_by": {
																											Type:     schema.TypeList,
																											Optional: true,
																											Elem: &schema.Schema{
																												Type: schema.TypeString,
																											},
																										},
																										"aggregations": {
																											Type: schema.TypeList,
																											Elem: &schema.Resource{
																												Schema: map[string]*schema.Schema{
																													"count": {
																														Type:     schema.TypeList,
																														MaxItems: 1,
																														Elem: &schema.Resource{
																															Schema: map[string]*schema.Schema{},
																														},
																														Optional: true,
																													},
																													"count_distinct": {
																														Type:     schema.TypeList,
																														MaxItems: 1,
																														Elem: &schema.Resource{
																															Schema: map[string]*schema.Schema{
																																"field": {
																																	Type:     schema.TypeString,
																																	Required: true,
																																},
																															},
																														},
																														Optional: true,
																													},
																													"sum": {
																														Type:     schema.TypeList,
																														MaxItems: 1,
																														Elem: &schema.Resource{
																															Schema: map[string]*schema.Schema{
																																"field": {
																																	Type:     schema.TypeString,
																																	Required: true,
																																},
																															},
																														},
																														Optional: true,
																													},
																													"average": {
																														Type:     schema.TypeList,
																														MaxItems: 1,
																														Elem: &schema.Resource{
																															Schema: map[string]*schema.Schema{
																																"field": {
																																	Type:     schema.TypeString,
																																	Required: true,
																																},
																															},
																														},
																														Optional: true,
																													},
																													"min": {
																														Type:     schema.TypeList,
																														MaxItems: 1,
																														Elem: &schema.Resource{
																															Schema: map[string]*schema.Schema{
																																"field": {
																																	Type:     schema.TypeString,
																																	Required: true,
																																},
																															},
																														},
																														Optional: true,
																													},
																													"max": {
																														Type:     schema.TypeList,
																														MaxItems: 1,
																														Elem: &schema.Resource{
																															Schema: map[string]*schema.Schema{
																																"field": {
																																	Type:     schema.TypeString,
																																	Required: true,
																																},
																															},
																														},
																														Optional: true,
																													},
																												},
																											},
																											Optional: true,
																										},
																									},
																								},
																								Optional: true,
																							},
																							"metrics": {
																								Type:     schema.TypeList,
																								MaxItems: 1,
																								Elem: &schema.Resource{
																									Schema: map[string]*schema.Schema{
																										"promql_query": {
																											Type:     schema.TypeString,
																											Required: true,
																										},
																									},
																								},
																								Optional: true,
																							},
																						},
																					},
																					Required: true,
																				},
																				"legend": {
																					Type:     schema.TypeList,
																					MaxItems: 1,
																					Elem: &schema.Resource{
																						Schema: map[string]*schema.Schema{
																							"is_visible": {
																								Type:     schema.TypeBool,
																								Required: true,
																							},
																							"columns": {
																								Type: schema.TypeList,
																								Elem: &schema.Schema{
																									Type:         schema.TypeString,
																									ValidateFunc: validation.StringInSlice(dashboardValidLegendColumn, false),
																								},
																								Required: true,
																							},
																						},
																					},
																					Optional: true,
																				},
																				"series_name_template": {
																					Type:     schema.TypeString,
																					Optional: true,
																				},
																			},
																		},
																		Optional: true,
																	},
																	"data_table": {
																		Type:     schema.TypeList,
																		MaxItems: 1,
																		Elem: &schema.Resource{
																			Schema: map[string]*schema.Schema{
																				"query": {
																					Type:     schema.TypeList,
																					MaxItems: 1,
																					Elem: &schema.Resource{
																						Schema: map[string]*schema.Schema{
																							"logs": {
																								Type:     schema.TypeList,
																								MaxItems: 1,
																								Elem: &schema.Resource{
																									Schema: map[string]*schema.Schema{
																										"lucene_query": {
																											Type:     schema.TypeString,
																											Optional: true,
																										},
																										"filters": {
																											Type: schema.TypeList,
																											Elem: &schema.Resource{
																												Schema: map[string]*schema.Schema{
																													"name": {
																														Type:     schema.TypeString,
																														Required: true,
																													},
																													"values": {
																														Type: schema.TypeList,
																														Elem: &schema.Schema{
																															Type: schema.TypeString,
																														},
																														Required: true,
																													},
																												},
																											},
																											Optional: true,
																										},
																									},
																								},
																								Required: true,
																							},
																						},
																					},
																					Required: true,
																				},
																				"results_per_page": {
																					Type:     schema.TypeInt,
																					Optional: true,
																				},
																				"row_style": {
																					Type:         schema.TypeString,
																					ValidateFunc: validation.StringInSlice(dashboardValidRowStyle, false),
																					Required:     true,
																				},
																				"columns": {
																					Type: schema.TypeList,
																					Elem: &schema.Resource{
																						Schema: map[string]*schema.Schema{
																							"field": {
																								Type:     schema.TypeString,
																								Required: true,
																							},
																							"order_direction": {
																								Type:         schema.TypeString,
																								ValidateFunc: validation.StringInSlice(dashboardValidOrderDirection, false),
																								Required:     true,
																							},
																						},
																					},
																					Optional: true,
																				},
																			},
																		},
																		Optional: true,
																	},
																},
															},
															Required: true,
														},
														"appearance": {
															Type:     schema.TypeList,
															MaxItems: 1,
															Elem: &schema.Resource{
																Schema: map[string]*schema.Schema{
																	"width": {
																		Type:     schema.TypeInt,
																		Required: true,
																	},
																},
															},
															Required: true,
														},
													},
												},
												Optional: true,
											},
										},
									},
									Optional: true,
								},
							},
						},
						Optional: true,
					},
				},
			},
			Optional: true,
		},
		"variables": {
			Type: schema.TypeList,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"name": {
						Type:     schema.TypeString,
						Required: true,
					},
					"definition": {
						Type:     schema.TypeList,
						MaxItems: 1,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"constant": {
									Type:     schema.TypeList,
									MaxItems: 1,
									Elem: &schema.Resource{
										Schema: map[string]*schema.Schema{
											"value": {
												Type:     schema.TypeString,
												Required: true,
											},
										},
									},
									Optional: true,
								},
								"multi_select": {
									Type:     schema.TypeList,
									MaxItems: 1,
									Elem: &schema.Resource{
										Schema: map[string]*schema.Schema{
											"selected": {
												Type: schema.TypeList,
												Elem: &schema.Schema{
													Type: schema.TypeString,
												},
												Required: true,
											},
											"source": {
												Type:     schema.TypeList,
												MaxItems: 1,
												Elem: &schema.Resource{
													Schema: map[string]*schema.Schema{
														"logs_path": {
															Type:     schema.TypeList,
															MaxItems: 1,
															Elem: &schema.Resource{
																Schema: map[string]*schema.Schema{
																	"value": {
																		Type:     schema.TypeString,
																		Required: true,
																	},
																},
															},
															Optional: true,
														},
														"metric_label": {
															Type:     schema.TypeList,
															MaxItems: 1,
															Elem: &schema.Resource{
																Schema: map[string]*schema.Schema{
																	"metric_name": {
																		Type:     schema.TypeString,
																		Required: true,
																	},
																	"label": {
																		Type:     schema.TypeString,
																		Required: true,
																	},
																},
															},
															Optional: true,
														},
														"constant_list": {
															Type:     schema.TypeList,
															MaxItems: 1,
															Elem: &schema.Resource{
																Schema: map[string]*schema.Schema{
																	"values": {
																		Type: schema.TypeList,
																		Elem: &schema.Schema{
																			Type: schema.TypeString,
																		},
																		Required: true,
																	},
																},
															},
															Optional: true,
														},
													},
												},
												Required: true,
											},
										},
									},
									Optional: true,
								},
							},
						},
						Required: true,
					},
				},
			},
			Optional: true,
		},
	}
}
