package coralogix

import (
	"context"
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
		"Unspecified": "ORDER_DIRECTION_UNSPECIFIED",
		"Asc":         "ORDER_DIRECTION_ASC",
		"Desc":        "ORDER_DIRECTION_DESC",
	}
	dashboardProtoRowStyleToSchemaRowStyle = reverseMapStrings(dashboardSchemaRowStyleToProtoRowStyle)
	dashboardValidRowStyle                 = getKeysStrings(dashboardSchemaRowStyleToProtoRowStyle)
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
	dashboard, err := extractDashboard(d)
	if err != nil {
		return diag.FromErr(err)
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

func extractDashboard(d *schema.ResourceData) (*dashboardv1.Dashboard, error) {
	name := wrapperspb.String(d.Get("name").(string))
	description := wrapperspb.String(d.Get("description").(string))
	layout := expandLayout(d.Get("layout"))
	variables := expandVariables(d.Get("variables"))
	return &dashboardv1.Dashboard{
		Name:        name,
		Description: description,
		Layout:      layout,
		Variables:   variables,
	}, nil
}

func expandLayout(v interface{}) *dashboardv1.Layout {
	var m map[string]interface{}
	if v == nil {
		return nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil
	} else {
		m = l[0].(map[string]interface{})
	}

	sections := expandSections(m["sections"])
	return &dashboardv1.Layout{
		Sections: sections,
	}

}

func expandSections(v interface{}) []*dashboardv1.Section {
	if v == nil {
		return nil
	}
	sections := v.([]interface{})
	result := make([]*dashboardv1.Section, 0, len(sections))
	for _, s := range sections {
		section := expandSection(s)
		result = append(result, section)
	}
	return result
}

func expandSection(v interface{}) *dashboardv1.Section {
	m := v.(map[string]interface{})
	uuid := expandUUID(m["id"])
	rows := expandRows(m["rows"])
	return &dashboardv1.Section{
		Id:   uuid,
		Rows: rows,
	}
}

func expandUUID(v interface{}) *dashboardv1.UUID {
	id := v.(string)
	if id == "" {
		id = uuid.NewString()
	}
	return &dashboardv1.UUID{Value: id}
}

func expandRows(v interface{}) []*dashboardv1.Row {
	if v == nil {
		return nil
	}
	rows := v.([]interface{})
	result := make([]*dashboardv1.Row, 0, len(rows))
	for _, r := range rows {
		row := expandRow(r)
		result = append(result, row)
	}
	return result
}

func expandRow(v interface{}) *dashboardv1.Row {
	m := v.(map[string]interface{})
	uuid := expandUUID(m["id"])
	appearance := expandRowAppearance(m["appearance"])
	widgets := expandWidgets(m["widgets"])
	return &dashboardv1.Row{
		Id:         uuid,
		Appearance: appearance,
		Widgets:    widgets,
	}
}

func expandRowAppearance(v interface{}) *dashboardv1.Row_Appearance {
	m := v.(map[string]interface{})
	height := wrapperspb.Int32(int32(m["height"].(int)))
	return &dashboardv1.Row_Appearance{
		Height: height,
	}
}

func expandWidgets(v interface{}) []*dashboardv1.Widget {
	if v == nil {
		return nil
	}
	widgets := v.([]interface{})
	result := make([]*dashboardv1.Widget, 0, len(widgets))
	for _, w := range widgets {
		widget := expandWidget(w)
		result = append(result, widget)
	}
	return result
}

func expandWidget(v interface{}) *dashboardv1.Widget {
	m := v.(map[string]interface{})
	id := expandUUID(m["id"])
	title := wrapperspb.String(m["title"].(string))
	description := wrapperspb.String(m["description"].(string))
	definition := expandWidgetDefinition(m["definition"])
	appearance := expandWidgetAppearance(m["appearance"])
	return &dashboardv1.Widget{
		Id:          id,
		Title:       title,
		Description: description,
		Definition:  definition,
		Appearance:  appearance,
	}
}

func expandWidgetDefinition(v interface{}) *dashboardv1.Widget_Definition {
	var m map[string]interface{}
	if v == nil {
		return nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil
	} else {
		m = l[0].(map[string]interface{})
	}

	if l, ok := m["line_chart"]; ok && len(l.([]interface{})) != 0 {
		lineChart := expandLineChart(l.([]interface{})[0])
		return &dashboardv1.Widget_Definition{
			Value: lineChart,
		}
	} else if l, ok = m["data_table"]; ok && len(l.([]interface{})) != 0 {
		dataTable := expandDataTable(l.([]interface{})[0])
		return &dashboardv1.Widget_Definition{
			Value: dataTable,
		}
	}

	return nil
}

func expandLineChart(v interface{}) *dashboardv1.Widget_Definition_LineChart {
	m := v.(map[string]interface{})
	query := expandLineChartQuery(m["query"])
	legend := expandLegend(m["legend"])
	seriesNameTemplate := wrapperspb.String(m["series_name_template"].(string))
	return &dashboardv1.Widget_Definition_LineChart{
		LineChart: &dashboardv1.LineChart{
			Query:              query,
			Legend:             legend,
			SeriesNameTemplate: seriesNameTemplate,
		},
	}
}

func expandLineChartQuery(v interface{}) *dashboardv1.LineChart_Query {
	var m map[string]interface{}
	if v == nil {
		return nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil
	} else {
		m = l[0].(map[string]interface{})
	}

	if l, ok := m["logs"]; ok && len(l.([]interface{})) != 0 {
		lineChartQueryLogs := expandLineChartQueryLogs(l.([]interface{})[0])
		return &dashboardv1.LineChart_Query{
			Value: lineChartQueryLogs,
		}
	} else if l, ok = m["metrics"]; ok && len(l.([]interface{})) != 0 {
		lineChartQueryMetrics := expandLineChartQueryMetric(l.([]interface{})[0])
		return &dashboardv1.LineChart_Query{
			Value: lineChartQueryMetrics,
		}
	}

	return nil
}

func expandLineChartQueryLogs(v interface{}) *dashboardv1.LineChart_Query_Logs {
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
	var m map[string]interface{}
	if v == nil {
		return nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil
	} else {
		m = l[0].(map[string]interface{})
	}

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
	legendColumns := v.([]string)
	result := make([]dashboardv1.Legend_LegendColumn, 0, len(legendColumns))
	for _, lc := range legendColumns {
		legend := expandLegendColumn(lc)
		result = append(result, legend)
	}
	return result
}

func expandLegendColumn(s string) dashboardv1.Legend_LegendColumn {
	return dashboardv1.Legend_LEGEND_COLUMN_UNSPECIFIED
}

func expandDataTable(v interface{}) *dashboardv1.Widget_Definition_DataTable {
	m := v.(map[string]interface{})
	query := expandDataTableQuery(m["query"])
	resultsPerPage := wrapperspb.Int32(int32(m["results_per_page"].(int)))
	rowStyle := expandRowStyle(m["row_style"].(string))
	columns := expandDataTableColumns(m["row_style"])

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
	var m map[string]interface{}
	if v == nil {
		return nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil
	} else {
		m = l[0].(map[string]interface{})
	}

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
	logsMap := m["logs"].([]map[string]interface{})[0]

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
	var m map[string]interface{}
	if v == nil {
		return nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil
	} else {
		v = l[0].(map[string]interface{})
	}

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
		v = l[0].(map[string]interface{})
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
		v = l[0].(map[string]interface{})
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
	log.Printf("[INFO] Reading dashboard %s", id)
	resp, err := meta.(*clientset.ClientSet).Dashboards().GetDashboard(ctx, &dashboardv1.GetDashboardRequest{DashboardId: nil})
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "dashboard", id)
	}

	dashboard := resp.GetDashboard()
	log.Printf("[INFO] Received dashboard: %#v", dashboard)

	return setDashboard(d, dashboard)
}

func setDashboard(d *schema.ResourceData, dashboard *dashboardv1.Dashboard) diag.Diagnostics {
	return nil
}

func resourceCoralogixDashboardUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	dashboard, err := extractDashboard(d)
	if err != nil {
		return diag.FromErr(err)
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
									Type:     schema.TypeList,
									MaxItems: 1,
									Elem: &schema.Resource{
										Schema: map[string]*schema.Schema{
											"id": {
												Type:     schema.TypeString,
												Computed: true,
											},
											"appearance": {
												Type:     schema.TypeList,
												Computed: true,
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
																													},
																												},
																											},
																										},
																									},
																								},
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
																							},
																						},
																					},
																				},
																				"legend": {
																					Type:     schema.TypeList,
																					MaxItems: 1,
																					Elem: &schema.Resource{
																						Schema: map[string]*schema.Schema{
																							"is_visible": {
																								Type: schema.TypeBool,
																							},
																							"columns": {
																								Type: schema.TypeList,
																								Elem: &schema.Schema{
																									Type:         schema.TypeString,
																									ValidateFunc: validation.StringInSlice([]string{}, false),
																								},
																							},
																						},
																					},
																				},
																				"series_name_template": {
																					Type:     schema.TypeString,
																					Optional: true,
																				},
																			},
																		},
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
																											Type:     schema.TypeList,
																											MaxItems: 1,
																											Elem: &schema.Resource{
																												Schema: map[string]*schema.Schema{
																													"name": {
																														Type: schema.TypeString,
																													},
																													"values": {
																														Type: schema.TypeList,
																														Elem: &schema.Schema{
																															Type: schema.TypeString,
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
																				},
																				"results_per_page": {
																					Type: schema.TypeInt,
																				},
																				"row_style": {
																					Type:         schema.TypeString,
																					ValidateFunc: validation.StringInSlice(dashboardValidRowStyle, false),
																				},
																				"columns": {
																					Type: schema.TypeList,
																					Elem: &schema.Resource{
																						Schema: map[string]*schema.Schema{
																							"field": {
																								Type: schema.TypeString,
																							},
																							"order_direction": {
																								Type:         schema.TypeString,
																								ValidateFunc: validation.StringInSlice(dashboardValidOrderDirection, false),
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
														"appearance": {
															Type:     schema.TypeList,
															MaxItems: 1,
															Elem: &schema.Resource{
																Schema: map[string]*schema.Schema{
																	"width": {
																		Type: schema.TypeInt,
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
							},
						},
					},
				},
			},
			Optional:    true,
			Description: "Dashboard description.",
		},
		"variables": {
			Type: schema.TypeList,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"name": {
						Type: schema.TypeString,
					},
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
									Elem: &schema.Schema{
										Type: schema.TypeString,
									},
								},
								"source": {
									Type:     schema.TypeList,
									MaxItems: 1,
									Elem: &schema.Resource{
										Schema: map[string]*schema.Schema{
											"logs_path": {
												Type: schema.TypeString,
											},
											"metric_label": {
												Type:     schema.TypeList,
												MaxItems: 1,
												Elem: &schema.Resource{
													Schema: map[string]*schema.Schema{
														"metric_name": {
															Type: schema.TypeString,
														},
														"label": {
															Type: schema.TypeString,
														},
													},
												},
											},
											"constant_list": {
												Type: schema.TypeList,
												Elem: &schema.Schema{
													Type: schema.TypeString,
												},
											},
										},
									},
								},
							},
						},
						Optional: true,
					},
				},
			},
		},
	}
}
