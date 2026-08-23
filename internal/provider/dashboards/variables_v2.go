// Copyright 2026 Coralogix Ltd.
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

package dashboards

import (
	"context"
	"fmt"
	"strconv"

	dashboardschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_schema"
	dashboardwidgets "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_widgets"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

type DashboardVariableV2Model struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	DisplayName    types.String `tfsdk:"display_name"`
	Description    types.String `tfsdk:"description"`
	DisplayType    types.String `tfsdk:"display_type"`
	DisplayFullRow types.Bool   `tfsdk:"display_full_row"`
	Source         types.Object `tfsdk:"source"`
	Value          types.Object `tfsdk:"value"`
}

type variableSourceV2Model struct {
	Static  *staticSourceV2Model  `tfsdk:"static"`
	Textbox *textboxSourceV2Model `tfsdk:"textbox"`
	Query   *querySourceV2Model   `tfsdk:"query"`
}

type staticSourceV2Model struct {
	ValuesOrderDirection types.String      `tfsdk:"values_order_direction"`
	AllOption            *allOptionV2Model `tfsdk:"all_option"`
	Values               types.List        `tfsdk:"values"`
}

type allOptionV2Model struct {
	IncludeAll types.Bool   `tfsdk:"include_all"`
	Label      types.String `tfsdk:"label"`
}

type staticValueV2Model struct {
	Value     types.String `tfsdk:"value"`
	Label     types.String `tfsdk:"label"`
	IsDefault types.Bool   `tfsdk:"is_default"`
}

type textboxSourceV2Model struct {
	DefaultValue *textboxDefaultValueV2Model `tfsdk:"default_value"`
}

type textboxDefaultValueV2Model struct {
	DefaultStringValue   *textboxStringValueV2Model  `tfsdk:"default_string_value"`
	DefaultNumericValue  *textboxNumericValueV2Model `tfsdk:"default_numeric_value"`
	DefaultRegexValue    *textboxStringValueV2Model  `tfsdk:"default_regex_value"`
	DefaultLuceneValue   *textboxLuceneValueV2Model  `tfsdk:"default_lucene_value"`
	DefaultIntervalValue *textboxStringValueV2Model  `tfsdk:"default_interval_value"`
}

type textboxStringValueV2Model struct {
	Value types.String `tfsdk:"value"`
}

type textboxNumericValueV2Model struct {
	Value     types.Float64 `tfsdk:"value"`
	Min       types.Float64 `tfsdk:"min"`
	Max       types.Float64 `tfsdk:"max"`
	IsInteger types.Bool    `tfsdk:"is_integer"`
}

type textboxLuceneValueV2Model struct {
	Value        types.String `tfsdk:"value"`
	DataModeType types.String `tfsdk:"data_mode_type"`
}

type querySourceV2Model struct {
	ValuesOrderDirection types.String                `tfsdk:"values_order_direction"`
	AllOption            *allOptionV2Model           `tfsdk:"all_option"`
	RefreshStrategy      types.String                `tfsdk:"refresh_strategy"`
	ValueDisplayOptions  *valueDisplayOptionsV2Model `tfsdk:"value_display_options"`
	LogsQuery            *logsQueryV2Model           `tfsdk:"logs_query"`
	SpansQuery           *spansQueryV2Model          `tfsdk:"spans_query"`
	MetricsQuery         *metricsQueryV2Model        `tfsdk:"metrics_query"`
	DataprimeQuery       *dataprimeQueryV2Model      `tfsdk:"dataprime_query"`
}

type valueDisplayOptionsV2Model struct {
	ValueRegex types.String `tfsdk:"value_regex"`
	LabelRegex types.String `tfsdk:"label_regex"`
}

type logsQueryV2Model struct {
	Type *logsQueryTypeV2Model `tfsdk:"type"`
}

type logsQueryTypeV2Model struct {
	FieldValue *logsFieldValueV2Model `tfsdk:"field_value"`
}

type logsFieldValueV2Model struct {
	ObservationField types.Object `tfsdk:"observation_field"`
}

type spansQueryV2Model struct {
	Type *spansQueryTypeV2Model `tfsdk:"type"`
}

type spansQueryTypeV2Model struct {
	FieldValue *spansFieldValueV2Model `tfsdk:"field_value"`
}

type spansFieldValueV2Model struct {
	Value            *dashboardwidgets.SpansFieldModel `tfsdk:"value"`
	ObservationField types.Object                      `tfsdk:"observation_field"`
}

type metricsQueryV2Model struct {
	Type *metricsQueryTypeV2Model `tfsdk:"type"`
}

type metricsQueryTypeV2Model struct {
	MetricName  *metricRegexV2Model       `tfsdk:"metric_name"`
	LabelName   *metricRegexV2Model       `tfsdk:"label_name"`
	LabelValue  *metricsLabelValueV2Model `tfsdk:"label_value"`
	PromqlQuery *promqlQueryV2Model       `tfsdk:"promql_query"`
}

type metricRegexV2Model struct {
	MetricRegex types.String `tfsdk:"metric_regex"`
}

type metricsLabelValueV2Model struct {
	MetricName   types.Object `tfsdk:"metric_name"`
	LabelName    types.Object `tfsdk:"label_name"`
	LabelFilters types.List   `tfsdk:"label_filters"`
}

type metricsLabelFilterV2Model struct {
	Metric   types.Object `tfsdk:"metric"`
	Label    types.Object `tfsdk:"label"`
	Operator types.Object `tfsdk:"operator"`
}

type metricsLabelFilterOperatorV2Model struct {
	Type           types.String `tfsdk:"type"`
	SelectedValues types.List   `tfsdk:"selected_values"`
}

type stringOrVariableV2Model struct {
	StringValue  types.String `tfsdk:"string_value"`
	VariableName types.String `tfsdk:"variable_name"`
}

type promqlQueryV2Model struct {
	Query           types.String `tfsdk:"query"`
	PromqlQueryType types.String `tfsdk:"promql_query_type"`
}

type dataprimeQueryV2Model struct {
	Type *dataprimeQueryTypeV2Model `tfsdk:"type"`
}

type dataprimeQueryTypeV2Model struct {
	QueryText *dataprimeQueryTextV2Model `tfsdk:"query_text"`
}

type dataprimeQueryTextV2Model struct {
	Query        types.String `tfsdk:"query"`
	DataModeType types.String `tfsdk:"data_mode_type"`
}

type variableValueV2Model struct {
	SingleString  *stringValueLabelFlatModel  `tfsdk:"single_string"`
	SingleNumeric *numericValueLabelFlatModel `tfsdk:"single_numeric"`
	Regex         *stringValueLabelFlatModel  `tfsdk:"regex"`
	Lucene        *stringValueLabelFlatModel  `tfsdk:"lucene"`
	Interval      *stringValueLabelFlatModel  `tfsdk:"interval"`
	MultiString   *multiStringValueV2Model    `tfsdk:"multi_string"`
}

type stringValueLabelFlatModel struct {
	Value types.String `tfsdk:"value"`
	Label types.String `tfsdk:"label"`
}

type numericValueLabelFlatModel struct {
	Value types.Float64 `tfsdk:"value"`
	Label types.String  `tfsdk:"label"`
}

type multiStringValueV2Model struct {
	SelectedAll types.Object            `tfsdk:"selected_all"`
	All         types.Object            `tfsdk:"all"`
	List        *multiStringListV2Model `tfsdk:"list"`
}

type multiStringListV2Model struct {
	Values types.List `tfsdk:"values"`
}

type multiStringListItemV2Model struct {
	Value *stringValueLabelFlatModel `tfsdk:"value"`
}

func expandDashboardVariablesV2(ctx context.Context, variables types.List) ([]dashboardservice.VariableV2, diag.Diagnostics) {
	if variables.IsNull() || variables.IsUnknown() {
		return nil, nil
	}

	var models []DashboardVariableV2Model
	diags := variables.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	result := make([]dashboardservice.VariableV2, 0, len(models))
	for _, model := range models {
		variable, variableDiags := expandDashboardVariableV2(ctx, model)
		diags.Append(variableDiags...)
		if variableDiags.HasError() {
			continue
		}
		result = append(result, *variable)
	}
	return result, diags
}

func expandDashboardVariableV2(ctx context.Context, model DashboardVariableV2Model) (*dashboardservice.VariableV2, diag.Diagnostics) {
	source, diags := expandVariableSourceV2(ctx, model.Source)
	if diags.HasError() {
		return nil, diags
	}
	value, valueDiags := expandVariableValueV2(ctx, model.Value)
	diags.Append(valueDiags...)
	if diags.HasError() {
		return nil, diags
	}

	result := &dashboardservice.VariableV2{
		Id:          *dashboardwidgets.ExpandDashboardUUID(model.ID),
		Name:        model.Name.ValueString(),
		DisplayName: model.DisplayName.ValueString(),
		DisplayType: requiredEnumValue(model.DisplayType, dashboardwidgets.DashboardSchemaToProtoDisplayTypeV2),
		Source:      derefOrZero(source),
		Value:       derefOrZero(value),
	}
	if !model.Description.IsNull() && !model.Description.IsUnknown() {
		result.Description = model.Description.ValueStringPointer()
	}
	if !model.DisplayFullRow.IsNull() && !model.DisplayFullRow.IsUnknown() {
		result.DisplayFullRow = model.DisplayFullRow.ValueBoolPointer()
	}
	return result, diags
}

func expandVariableSourceV2(ctx context.Context, source types.Object) (*dashboardservice.VariableSourceV2, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(source) {
		return nil, nil
	}
	var model variableSourceV2Model
	diags := source.As(ctx, &model, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	result := &dashboardservice.VariableSourceV2{}
	switch {
	case model.Static != nil:
		static, staticDiags := expandStaticSourceV2(ctx, model.Static)
		diags.Append(staticDiags...)
		if diags.HasError() {
			return nil, diags
		}
		result.Static = static
	case model.Textbox != nil:
		textbox, textboxDiags := expandTextboxSourceV2(ctx, model.Textbox)
		diags.Append(textboxDiags...)
		if diags.HasError() {
			return nil, diags
		}
		result.Textbox = textbox
	case model.Query != nil:
		query, queryDiags := expandQuerySourceV2(ctx, model.Query)
		diags.Append(queryDiags...)
		if diags.HasError() {
			return nil, diags
		}
		result.Query = query
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error expanding variables_v2 source", "source must set exactly one of static, textbox, or query")}
	}
	return result, diags
}

func expandStaticSourceV2(ctx context.Context, model *staticSourceV2Model) (*dashboardservice.StaticSource, diag.Diagnostics) {
	values, diags := expandStaticValuesV2(ctx, model.Values)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboardservice.StaticSource{
		ValuesOrderDirection: requiredEnumValue(model.ValuesOrderDirection, dashboardwidgets.DashboardOrderDirectionSchemaToProtoV2),
		AllOption:            expandAllOptionV2(model.AllOption),
		Values:               values,
	}, nil
}

func expandStaticValuesV2(ctx context.Context, values types.List) ([]dashboardservice.ValueLabel, diag.Diagnostics) {
	if values.IsNull() || values.IsUnknown() {
		return nil, nil
	}
	var models []staticValueV2Model
	diags := values.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}
	result := make([]dashboardservice.ValueLabel, 0, len(models))
	for _, model := range models {
		label := model.Label
		if label.IsNull() || label.IsUnknown() {
			label = model.Value
		}
		entry := dashboardservice.ValueLabel{
			Value: model.Value.ValueString(),
			Label: label.ValueString(),
		}
		if !model.IsDefault.IsNull() && !model.IsDefault.IsUnknown() {
			entry.IsDefault = model.IsDefault.ValueBoolPointer()
		}
		result = append(result, entry)
	}
	return result, nil
}

func expandAllOptionV2(model *allOptionV2Model) dashboardservice.AllOption {
	if model == nil {
		return dashboardservice.AllOption{}
	}
	result := dashboardservice.AllOption{}
	if !model.IncludeAll.IsNull() && !model.IncludeAll.IsUnknown() {
		result.IncludeAll = model.IncludeAll.ValueBool()
	}
	if !model.Label.IsNull() && !model.Label.IsUnknown() {
		result.Label = model.Label.ValueStringPointer()
	}
	return result
}

func expandTextboxSourceV2(ctx context.Context, model *textboxSourceV2Model) (*dashboardservice.TextboxSource, diag.Diagnostics) {
	_ = ctx
	if model.DefaultValue == nil {
		return &dashboardservice.TextboxSource{}, nil
	}
	defaultValue, diags := expandTextboxDefaultValueV2(model.DefaultValue)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboardservice.TextboxSource{DefaultValue: defaultValue}, nil
}

func expandTextboxDefaultValueV2(model *textboxDefaultValueV2Model) (*dashboardservice.TextboxDefaultValue, diag.Diagnostics) {
	result := &dashboardservice.TextboxDefaultValue{}
	switch {
	case model.DefaultStringValue != nil:
		result.DefaultStringValue = &dashboardservice.TextboxDefaultStringValue{
			Value: utils.TypeStringToStringPointer(model.DefaultStringValue.Value),
		}
	case model.DefaultNumericValue != nil:
		result.DefaultNumericValue = &dashboardservice.TextboxDefaultNumericValue{
			Value:     typeFloat64ToFloat32Pointer(model.DefaultNumericValue.Value),
			Min:       typeFloat64ToFloat32Pointer(model.DefaultNumericValue.Min),
			Max:       typeFloat64ToFloat32Pointer(model.DefaultNumericValue.Max),
			IsInteger: boolPointerIfSet(model.DefaultNumericValue.IsInteger),
		}
	case model.DefaultRegexValue != nil:
		result.DefaultRegexValue = &dashboardservice.TextboxDefaultRegexValue{
			Value: utils.TypeStringToStringPointer(model.DefaultRegexValue.Value),
		}
	case model.DefaultLuceneValue != nil:
		result.DefaultLuceneValue = &dashboardservice.TextboxDefaultLuceneValue{
			Value:        utils.TypeStringToStringPointer(model.DefaultLuceneValue.Value),
			DataModeType: dashboardwidgets.OptionalEnumPointer(model.DefaultLuceneValue.DataModeType, dashboardwidgets.DashboardSchemaToProtoDataModeTypeV2),
		}
	case model.DefaultIntervalValue != nil:
		result.DefaultIntervalValue = &dashboardservice.TextboxDefaultIntervalValue{
			Value: utils.TypeStringToStringPointer(model.DefaultIntervalValue.Value),
		}
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error expanding variables_v2 textbox", "default_value must set exactly one arm")}
	}
	return result, nil
}

func expandQuerySourceV2(ctx context.Context, model *querySourceV2Model) (*dashboardservice.VariableSourceV2QuerySource, diag.Diagnostics) {
	result := &dashboardservice.VariableSourceV2QuerySource{
		ValuesOrderDirection: requiredEnumValue(model.ValuesOrderDirection, dashboardwidgets.DashboardOrderDirectionSchemaToProtoV2),
		AllOption:            expandAllOptionV2(model.AllOption),
		ValueDisplayOptions:  expandValueDisplayOptionsV2(model.ValueDisplayOptions),
		RefreshStrategy:      dashboardwidgets.OptionalEnumPointer(model.RefreshStrategy, dashboardwidgets.DashboardSchemaToProtoVariableV2RefreshStrategy),
	}

	var diags diag.Diagnostics
	switch {
	case model.LogsQuery != nil:
		logs, logsDiags := expandLogsQueryV2(ctx, model.LogsQuery)
		diags.Append(logsDiags...)
		result.LogsQuery = logs
	case model.SpansQuery != nil:
		spans, spansDiags := expandSpansQueryV2(ctx, model.SpansQuery)
		diags.Append(spansDiags...)
		result.SpansQuery = spans
	case model.MetricsQuery != nil:
		metrics, metricsDiags := expandMetricsQueryV2(ctx, model.MetricsQuery)
		diags.Append(metricsDiags...)
		result.MetricsQuery = metrics
	case model.DataprimeQuery != nil:
		dataprime, dataprimeDiags := expandDataprimeQueryV2(model.DataprimeQuery)
		diags.Append(dataprimeDiags...)
		result.DataprimeQuery = dataprime
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error expanding variables_v2 query", "query must set exactly one of logs_query, spans_query, metrics_query, or dataprime_query")}
	}
	if diags.HasError() {
		return nil, diags
	}
	return result, diags
}

func expandValueDisplayOptionsV2(model *valueDisplayOptionsV2Model) *dashboardservice.VariableSourceV2ValueDisplayOptions {
	if model == nil {
		return nil
	}
	result := &dashboardservice.VariableSourceV2ValueDisplayOptions{}
	if !model.ValueRegex.IsNull() && !model.ValueRegex.IsUnknown() {
		result.ValueRegex = model.ValueRegex.ValueStringPointer()
	}
	if !model.LabelRegex.IsNull() && !model.LabelRegex.IsUnknown() {
		result.LabelRegex = model.LabelRegex.ValueStringPointer()
	}
	return result
}

func expandLogsQueryV2(ctx context.Context, model *logsQueryV2Model) (*dashboardservice.QuerySourceLogsQuery, diag.Diagnostics) {
	if model.Type == nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error expanding variables_v2 logs_query", "type is required")}
	}
	if model.Type.FieldValue == nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error expanding variables_v2 logs_query", "type must set field_value")}
	}
	var diags diag.Diagnostics
	observationField, obsDiags := dashboardwidgets.ExpandObservationFieldObject(ctx, model.Type.FieldValue.ObservationField)
	diags.Append(obsDiags...)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboardservice.QuerySourceLogsQuery{
		Type: &dashboardservice.QuerySourceLogsQueryType{
			FieldValue: &dashboardservice.QuerySourceLogsQueryTypeFieldValue{
				ObservationField: derefOrZero(observationField),
			},
		},
	}, diags
}

func expandSpansQueryV2(ctx context.Context, model *spansQueryV2Model) (*dashboardservice.QuerySourceSpansQuery, diag.Diagnostics) {
	if model.Type == nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error expanding variables_v2 spans_query", "type is required")}
	}
	if model.Type.FieldValue == nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error expanding variables_v2 spans_query", "type must set field_value")}
	}
	var diags diag.Diagnostics
	fieldValue := &dashboardservice.QuerySourceSpansQueryTypeFieldValue{}
	if model.Type.FieldValue.Value != nil {
		spanField, dg := dashboardwidgets.ExpandSpansField(model.Type.FieldValue.Value)
		if dg != nil {
			diags.Append(dg)
			return nil, diags
		}
		fieldValue.Value = spanField
	}
	if !utils.ObjIsNullOrUnknown(model.Type.FieldValue.ObservationField) {
		observationField, obsDiags := dashboardwidgets.ExpandObservationFieldObject(ctx, model.Type.FieldValue.ObservationField)
		diags.Append(obsDiags...)
		if diags.HasError() {
			return nil, diags
		}
		fieldValue.ObservationField = observationField
	}
	return &dashboardservice.QuerySourceSpansQuery{
		Type: &dashboardservice.QuerySourceSpansQueryType{
			FieldValue: fieldValue,
		},
	}, diags
}

func expandMetricsQueryV2(ctx context.Context, model *metricsQueryV2Model) (*dashboardservice.QuerySourceMetricsQuery, diag.Diagnostics) {
	if model.Type == nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error expanding variables_v2 metrics_query", "type is required")}
	}
	queryType := &dashboardservice.QuerySourceMetricsQueryType{}
	var diags diag.Diagnostics
	switch {
	case model.Type.MetricName != nil:
		queryType.MetricName = &dashboardservice.QuerySourceMetricsQueryTypeMetricName{
			MetricRegex: utils.TypeStringToStringPointer(model.Type.MetricName.MetricRegex),
		}
	case model.Type.LabelName != nil:
		queryType.LabelName = &dashboardservice.QuerySourceMetricsQueryTypeLabelName{
			MetricRegex: utils.TypeStringToStringPointer(model.Type.LabelName.MetricRegex),
		}
	case model.Type.LabelValue != nil:
		labelValue, labelDiags := expandMetricsLabelValueV2(ctx, model.Type.LabelValue)
		diags.Append(labelDiags...)
		if diags.HasError() {
			return nil, diags
		}
		queryType.LabelValue = labelValue
	case model.Type.PromqlQuery != nil:
		queryType.PromqlQuery = &dashboardservice.PromqlQuery{
			Query:           dashboardwidgets.ExpandPromqlQuery(model.Type.PromqlQuery.Query),
			PromqlQueryType: dashboardwidgets.OptionalEnumPointer(model.Type.PromqlQuery.PromqlQueryType, dashboardwidgets.DashboardSchemaToProtoPromQLQueryType),
		}
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error expanding variables_v2 metrics_query", "type must set metric_name, label_name, label_value, or promql_query")}
	}
	return &dashboardservice.QuerySourceMetricsQuery{Type: queryType}, diags
}

func expandMetricsLabelValueV2(ctx context.Context, model *metricsLabelValueV2Model) (*dashboardservice.QuerySourceMetricsQueryTypeLabelValue, diag.Diagnostics) {
	metricName, diags := expandStringOrVariableV2(ctx, model.MetricName)
	if diags.HasError() {
		return nil, diags
	}
	labelName, labelDiags := expandStringOrVariableV2(ctx, model.LabelName)
	diags.Append(labelDiags...)
	if diags.HasError() {
		return nil, diags
	}
	labelFilters, filterDiags := expandMetricsLabelFiltersV2(ctx, model.LabelFilters)
	diags.Append(filterDiags...)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboardservice.QuerySourceMetricsQueryTypeLabelValue{
		MetricName:   metricName,
		LabelName:    derefOrZero(labelName),
		LabelFilters: labelFilters,
	}, diags
}

func expandMetricsLabelFiltersV2(ctx context.Context, filters types.List) ([]dashboardservice.QuerySourceMetricsQueryMetricsLabelFilter, diag.Diagnostics) {
	if filters.IsNull() || filters.IsUnknown() || len(filters.Elements()) == 0 {
		return nil, nil
	}
	var models []metricsLabelFilterV2Model
	diags := filters.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}
	result := make([]dashboardservice.QuerySourceMetricsQueryMetricsLabelFilter, 0, len(models))
	for _, model := range models {
		filter, filterDiags := expandMetricsLabelFilterV2(ctx, model)
		diags.Append(filterDiags...)
		if filterDiags.HasError() {
			continue
		}
		result = append(result, *filter)
	}
	if diags.HasError() {
		return nil, diags
	}
	return result, nil
}

func expandMetricsLabelFilterV2(ctx context.Context, model metricsLabelFilterV2Model) (*dashboardservice.QuerySourceMetricsQueryMetricsLabelFilter, diag.Diagnostics) {
	metric, diags := expandStringOrVariableV2(ctx, model.Metric)
	if diags.HasError() {
		return nil, diags
	}
	label, labelDiags := expandStringOrVariableV2(ctx, model.Label)
	diags.Append(labelDiags...)
	if diags.HasError() {
		return nil, diags
	}
	operator, operatorDiags := expandMetricsLabelFilterOperatorV2(ctx, model.Operator)
	diags.Append(operatorDiags...)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboardservice.QuerySourceMetricsQueryMetricsLabelFilter{
		Metric:   metric,
		Label:    label,
		Operator: operator,
	}, diags
}

func expandMetricsLabelFilterOperatorV2(ctx context.Context, operator types.Object) (*dashboardservice.QuerySourceMetricsQueryOperator, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(operator) {
		return nil, nil
	}
	var model metricsLabelFilterOperatorV2Model
	diags := operator.As(ctx, &model, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}
	values, valuesDiags := expandStringOrVariablesV2(ctx, model.SelectedValues)
	diags.Append(valuesDiags...)
	if diags.HasError() {
		return nil, diags
	}
	selection := &dashboardservice.QuerySourceMetricsQuerySelection{
		List: &dashboardservice.QuerySourceMetricsQuerySelectionListSelection{Values: values},
	}
	switch model.Type.ValueString() {
	case "equals":
		return &dashboardservice.QuerySourceMetricsQueryOperator{
			Equals: &dashboardservice.QuerySourceMetricsQueryEquals{Selection: selection},
		}, nil
	case "not_equals":
		return &dashboardservice.QuerySourceMetricsQueryOperator{
			NotEquals: &dashboardservice.QuerySourceMetricsQueryNotEquals{Selection: selection},
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error expanding variables_v2 metrics operator", fmt.Sprintf("unknown operator type %q", model.Type.ValueString()))}
	}
}

func expandStringOrVariablesV2(ctx context.Context, values types.List) ([]dashboardservice.QuerySourceMetricsQueryStringOrVariable, diag.Diagnostics) {
	if values.IsNull() || values.IsUnknown() {
		return nil, nil
	}
	var objects []types.Object
	diags := values.ElementsAs(ctx, &objects, true)
	if diags.HasError() {
		return nil, diags
	}
	result := make([]dashboardservice.QuerySourceMetricsQueryStringOrVariable, 0, len(objects))
	for _, object := range objects {
		value, valueDiags := expandStringOrVariableV2(ctx, object)
		diags.Append(valueDiags...)
		if valueDiags.HasError() {
			continue
		}
		if value != nil {
			result = append(result, *value)
		}
	}
	if diags.HasError() {
		return nil, diags
	}
	return result, nil
}

func expandStringOrVariableV2(ctx context.Context, value types.Object) (*dashboardservice.QuerySourceMetricsQueryStringOrVariable, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(value) {
		return nil, nil
	}
	var model stringOrVariableV2Model
	diags := value.As(ctx, &model, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}
	switch {
	case !model.VariableName.IsNull() && !model.VariableName.IsUnknown():
		return &dashboardservice.QuerySourceMetricsQueryStringOrVariable{
			VariableName: utils.TypeStringToStringPointer(model.VariableName),
		}, nil
	case !model.StringValue.IsNull() && !model.StringValue.IsUnknown():
		return &dashboardservice.QuerySourceMetricsQueryStringOrVariable{
			StringValue: utils.TypeStringToStringPointer(model.StringValue),
		}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error expanding variables_v2 string_or_variable", "must set string_value or variable_name")}
	}
}

func expandDataprimeQueryV2(model *dataprimeQueryV2Model) (*dashboardservice.QuerySourceDataprimeQuery, diag.Diagnostics) {
	if model.Type == nil || model.Type.QueryText == nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error expanding variables_v2 dataprime_query", "type.query_text is required")}
	}
	return &dashboardservice.QuerySourceDataprimeQuery{
		Type: &dashboardservice.DataprimeQueryType{
			QueryText: &dashboardservice.QueryText{
				Query: &dashboardservice.CommonDataprimeQuery{
					Text: utils.TypeStringToStringPointer(model.Type.QueryText.Query),
				},
				DataModeType: dashboardwidgets.OptionalEnumPointer(model.Type.QueryText.DataModeType, dashboardwidgets.DashboardSchemaToProtoDataModeTypeV2),
			},
		},
	}, nil
}

func expandVariableValueV2(ctx context.Context, value types.Object) (*dashboardservice.VariableValueV2, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(value) {
		return nil, nil
	}
	var model variableValueV2Model
	diags := value.As(ctx, &model, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}
	result := &dashboardservice.VariableValueV2{}
	switch {
	case model.SingleString != nil:
		result.SingleString = &dashboardservice.SingleStringValue{Value: expandStringValueLabel(model.SingleString)}
	case model.SingleNumeric != nil:
		result.SingleNumeric = &dashboardservice.VariableValueV2SingleNumericValue{Value: expandNumericValueLabel(model.SingleNumeric)}
	case model.Regex != nil:
		result.Regex = &dashboardservice.RegexValue{Value: expandStringValueLabel(model.Regex)}
	case model.Lucene != nil:
		result.Lucene = &dashboardservice.LuceneQueryValue{Value: expandStringValueLabel(model.Lucene)}
	case model.Interval != nil:
		result.Interval = &dashboardservice.IntervalValue{Value: expandStringValueLabel(model.Interval)}
	case model.MultiString != nil:
		multi, multiDiags := expandMultiStringValueV2(ctx, model.MultiString)
		diags.Append(multiDiags...)
		if diags.HasError() {
			return nil, diags
		}
		result.MultiString = multi
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error expanding variables_v2 value", "value must set exactly one arm")}
	}
	return result, diags
}

func expandStringValueLabel(model *stringValueLabelFlatModel) *dashboardservice.StringValueLabel {
	if model == nil {
		return nil
	}
	return &dashboardservice.StringValueLabel{
		Value: utils.TypeStringToStringPointer(model.Value),
		Label: utils.TypeStringToStringPointer(model.Label),
	}
}

func expandNumericValueLabel(model *numericValueLabelFlatModel) *dashboardservice.NumericValueLabel {
	if model == nil {
		return nil
	}
	return &dashboardservice.NumericValueLabel{
		Value: typeFloat64ToFloat32Pointer(model.Value),
		Label: utils.TypeStringToStringPointer(model.Label),
	}
}

func expandMultiStringValueV2(ctx context.Context, model *multiStringValueV2Model) (*dashboardservice.MultiStringValue, diag.Diagnostics) {
	result := &dashboardservice.MultiStringValue{}
	switch {
	case !utils.ObjIsNullOrUnknown(model.SelectedAll):
		result.SelectedAll = map[string]interface{}{}
	case !utils.ObjIsNullOrUnknown(model.All):
		result.All = map[string]interface{}{}
	case model.List != nil:
		values, diags := expandMultiStringListValuesV2(ctx, model.List.Values)
		if diags.HasError() {
			return nil, diags
		}
		result.List = &dashboardservice.ListValue{Values: values}
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error expanding variables_v2 multi_string", "must set selected_all, all, or list")}
	}
	return result, nil
}

func expandMultiStringListValuesV2(ctx context.Context, values types.List) ([]dashboardservice.SingleStringValue, diag.Diagnostics) {
	if values.IsNull() || values.IsUnknown() {
		return nil, nil
	}
	var models []multiStringListItemV2Model
	diags := values.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}
	result := make([]dashboardservice.SingleStringValue, 0, len(models))
	for _, model := range models {
		result = append(result, dashboardservice.SingleStringValue{Value: expandStringValueLabel(model.Value)})
	}
	return result, nil
}

func flattenDashboardVariablesV2(ctx context.Context, variables []dashboardservice.VariableV2, configured types.List) (types.List, diag.Diagnostics) {
	elementType, ok := configured.ElementType(ctx).(basetypes.ObjectType)
	if !ok {
		elementType = dashboardVariablesV2ElementType()
	}
	if len(variables) == 0 {
		return types.ListNull(elementType), nil
	}

	elements := make([]attr.Value, 0, len(variables))
	var diags diag.Diagnostics
	for i := range variables {
		element, elementDiags := flattenDashboardVariableV2(ctx, &variables[i], elementType)
		diags.Append(elementDiags...)
		if elementDiags.HasError() {
			continue
		}
		elements = append(elements, element)
	}
	if diags.HasError() {
		return types.ListNull(elementType), diags
	}
	return types.ListValueMust(elementType, elements), nil
}

func flattenDashboardVariableV2(ctx context.Context, variable *dashboardservice.VariableV2, elementType basetypes.ObjectType) (types.Object, diag.Diagnostics) {
	sourceType := elementType.AttrTypes["source"].(types.ObjectType)
	valueType := elementType.AttrTypes["value"].(types.ObjectType)

	source, diags := flattenVariableSourceV2(ctx, &variable.Source, sourceType)
	if diags.HasError() {
		return types.ObjectNull(elementType.AttrTypes), diags
	}
	value, valueDiags := flattenVariableValueV2(ctx, &variable.Value, valueType)
	diags.Append(valueDiags...)
	if diags.HasError() {
		return types.ObjectNull(elementType.AttrTypes), diags
	}

	attrs := map[string]attr.Value{
		"id":               flattenVariableID(variable.Id),
		"name":             types.StringValue(variable.Name),
		"display_name":     types.StringValue(variable.DisplayName),
		"description":      types.StringPointerValue(variable.Description),
		"display_full_row": types.BoolPointerValue(variable.DisplayFullRow),
		"source":           source,
		"value":            value,
	}
	if variable.DisplayType != "" && variable.DisplayType != dashboardservice.VARIABLEDISPLAYTYPEV2_VARIABLE_DISPLAY_TYPE_V2_UNSPECIFIED {
		if mapped, ok := dashboardwidgets.DashboardProtoToSchemaDisplayTypeV2[variable.DisplayType]; ok {
			attrs["display_type"] = types.StringValue(mapped)
		} else {
			attrs["display_type"] = types.StringNull()
		}
	} else {
		attrs["display_type"] = types.StringNull()
	}
	return types.ObjectValueMust(elementType.AttrTypes, attrs), diags
}

func flattenVariableID(id dashboardservice.UUID) types.String {
	if id.Value == nil {
		return types.StringNull()
	}
	return types.StringValue(*id.Value)
}

func flattenVariableSourceV2(ctx context.Context, source *dashboardservice.VariableSourceV2, objectType types.ObjectType) (types.Object, diag.Diagnostics) {
	if source == nil {
		return types.ObjectNull(objectType.AttrTypes), nil
	}
	attrs := map[string]attr.Value{
		"static":  nullValueForType(objectType.AttrTypes["static"]),
		"textbox": nullValueForType(objectType.AttrTypes["textbox"]),
		"query":   nullValueForType(objectType.AttrTypes["query"]),
	}
	var diags diag.Diagnostics
	switch {
	case source.Static != nil:
		static, staticDiags := flattenStaticSourceV2(ctx, source.Static, objectType.AttrTypes["static"].(types.ObjectType))
		diags.Append(staticDiags...)
		attrs["static"] = static
	case source.Textbox != nil:
		textbox, textboxDiags := flattenTextboxSourceV2(ctx, source.Textbox, objectType.AttrTypes["textbox"].(types.ObjectType))
		diags.Append(textboxDiags...)
		attrs["textbox"] = textbox
	case source.Query != nil:
		query, queryDiags := flattenQuerySourceV2(ctx, source.Query, objectType.AttrTypes["query"].(types.ObjectType))
		diags.Append(queryDiags...)
		attrs["query"] = query
	}
	if diags.HasError() {
		return types.ObjectNull(objectType.AttrTypes), diags
	}
	return types.ObjectValueMust(objectType.AttrTypes, attrs), nil
}

func flattenStaticSourceV2(ctx context.Context, source *dashboardservice.StaticSource, objectType types.ObjectType) (types.Object, diag.Diagnostics) {
	valuesType := objectType.AttrTypes["values"].(types.ListType)
	values, diags := flattenStaticValuesV2(ctx, source.Values, valuesType)
	if diags.HasError() {
		return types.ObjectNull(objectType.AttrTypes), diags
	}
	allOption, allDiags := flattenAllOptionV2(source.AllOption, objectType.AttrTypes["all_option"].(types.ObjectType))
	diags.Append(allDiags...)
	if diags.HasError() {
		return types.ObjectNull(objectType.AttrTypes), diags
	}
	attrs := map[string]attr.Value{
		"values":                 values,
		"all_option":             allOption,
		"values_order_direction": flattenOrderDirectionV2(source.ValuesOrderDirection),
	}
	return types.ObjectValueMust(objectType.AttrTypes, attrs), diags
}

func flattenStaticValuesV2(ctx context.Context, values []dashboardservice.ValueLabel, listType types.ListType) (types.List, diag.Diagnostics) {
	_ = ctx
	if len(values) == 0 {
		return types.ListNull(listType.ElemType), nil
	}
	entryType := listType.ElemType.(types.ObjectType)
	elements := make([]attr.Value, 0, len(values))
	for _, value := range values {
		// Optional+Computed label uses StaticValueLabelFromValue so omitted config
		// plans as value. Always store the API label (including when label==value).
		elements = append(elements, types.ObjectValueMust(entryType.AttrTypes, map[string]attr.Value{
			"value":      types.StringValue(value.Value),
			"label":      types.StringValue(value.Label),
			"is_default": types.BoolPointerValue(value.IsDefault),
		}))
	}
	return types.ListValueMust(entryType, elements), nil
}

func flattenAllOptionV2(option dashboardservice.AllOption, objectType types.ObjectType) (types.Object, diag.Diagnostics) {
	return types.ObjectValueMust(objectType.AttrTypes, map[string]attr.Value{
		"include_all": types.BoolValue(option.IncludeAll),
		"label":       types.StringPointerValue(option.Label),
	}), nil
}

func flattenTextboxSourceV2(ctx context.Context, source *dashboardservice.TextboxSource, objectType types.ObjectType) (types.Object, diag.Diagnostics) {
	defaultValueType := objectType.AttrTypes["default_value"].(types.ObjectType)
	defaultValue, diags := flattenTextboxDefaultValueV2(source.DefaultValue, defaultValueType)
	if diags.HasError() {
		return types.ObjectNull(objectType.AttrTypes), diags
	}
	return types.ObjectValueMust(objectType.AttrTypes, map[string]attr.Value{
		"default_value": defaultValue,
	}), nil
}

func flattenTextboxDefaultValueV2(value *dashboardservice.TextboxDefaultValue, objectType types.ObjectType) (types.Object, diag.Diagnostics) {
	if value == nil {
		return types.ObjectNull(objectType.AttrTypes), nil
	}
	attrs := map[string]attr.Value{
		"default_string_value":   nullValueForType(objectType.AttrTypes["default_string_value"]),
		"default_numeric_value":  nullValueForType(objectType.AttrTypes["default_numeric_value"]),
		"default_regex_value":    nullValueForType(objectType.AttrTypes["default_regex_value"]),
		"default_lucene_value":   nullValueForType(objectType.AttrTypes["default_lucene_value"]),
		"default_interval_value": nullValueForType(objectType.AttrTypes["default_interval_value"]),
	}
	switch {
	case value.DefaultStringValue != nil:
		attrs["default_string_value"] = types.ObjectValueMust(
			objectType.AttrTypes["default_string_value"].(types.ObjectType).AttrTypes,
			map[string]attr.Value{"value": types.StringPointerValue(value.DefaultStringValue.Value)},
		)
	case value.DefaultNumericValue != nil:
		attrs["default_numeric_value"] = types.ObjectValueMust(
			objectType.AttrTypes["default_numeric_value"].(types.ObjectType).AttrTypes,
			map[string]attr.Value{
				"value":      float32PointerToTypeFloat64(value.DefaultNumericValue.Value),
				"min":        float32PointerToTypeFloat64(value.DefaultNumericValue.Min),
				"max":        float32PointerToTypeFloat64(value.DefaultNumericValue.Max),
				"is_integer": types.BoolPointerValue(value.DefaultNumericValue.IsInteger),
			},
		)
	case value.DefaultRegexValue != nil:
		attrs["default_regex_value"] = types.ObjectValueMust(
			objectType.AttrTypes["default_regex_value"].(types.ObjectType).AttrTypes,
			map[string]attr.Value{"value": types.StringPointerValue(value.DefaultRegexValue.Value)},
		)
	case value.DefaultLuceneValue != nil:
		dataMode := types.StringNull()
		if value.DefaultLuceneValue.DataModeType != nil {
			dataMode = types.StringValue(dashboardwidgets.DashboardProtoToSchemaDataModeTypeV2[*value.DefaultLuceneValue.DataModeType])
		}
		attrs["default_lucene_value"] = types.ObjectValueMust(
			objectType.AttrTypes["default_lucene_value"].(types.ObjectType).AttrTypes,
			map[string]attr.Value{
				"value":          types.StringPointerValue(value.DefaultLuceneValue.Value),
				"data_mode_type": dataMode,
			},
		)
	case value.DefaultIntervalValue != nil:
		attrs["default_interval_value"] = types.ObjectValueMust(
			objectType.AttrTypes["default_interval_value"].(types.ObjectType).AttrTypes,
			map[string]attr.Value{"value": types.StringPointerValue(value.DefaultIntervalValue.Value)},
		)
	}
	return types.ObjectValueMust(objectType.AttrTypes, attrs), nil
}

func flattenQuerySourceV2(ctx context.Context, source *dashboardservice.VariableSourceV2QuerySource, objectType types.ObjectType) (types.Object, diag.Diagnostics) {
	attrs := map[string]attr.Value{
		"logs_query":            nullValueForType(objectType.AttrTypes["logs_query"]),
		"spans_query":           nullValueForType(objectType.AttrTypes["spans_query"]),
		"metrics_query":         nullValueForType(objectType.AttrTypes["metrics_query"]),
		"dataprime_query":       nullValueForType(objectType.AttrTypes["dataprime_query"]),
		"value_display_options": nullValueForType(objectType.AttrTypes["value_display_options"]),
	}
	var diags diag.Diagnostics

	allOption, allDiags := flattenAllOptionV2(source.AllOption, objectType.AttrTypes["all_option"].(types.ObjectType))
	diags.Append(allDiags...)
	attrs["all_option"] = allOption
	attrs["values_order_direction"] = flattenOrderDirectionV2(source.ValuesOrderDirection)

	if source.RefreshStrategy != nil && *source.RefreshStrategy != dashboardservice.VARIABLESOURCEV2REFRESHSTRATEGY_REFRESH_STRATEGY_UNSPECIFIED {
		attrs["refresh_strategy"] = types.StringValue(dashboardwidgets.DashboardProtoToSchemaVariableV2RefreshStrategy[*source.RefreshStrategy])
	} else {
		attrs["refresh_strategy"] = types.StringNull()
	}
	if source.ValueDisplayOptions != nil {
		attrs["value_display_options"] = types.ObjectValueMust(
			objectType.AttrTypes["value_display_options"].(types.ObjectType).AttrTypes,
			map[string]attr.Value{
				"value_regex": types.StringPointerValue(source.ValueDisplayOptions.ValueRegex),
				"label_regex": types.StringPointerValue(source.ValueDisplayOptions.LabelRegex),
			},
		)
	}

	switch {
	case source.LogsQuery != nil:
		logs, logsDiags := flattenLogsQueryV2(ctx, source.LogsQuery, objectType.AttrTypes["logs_query"].(types.ObjectType))
		diags.Append(logsDiags...)
		attrs["logs_query"] = logs
	case source.SpansQuery != nil:
		spans, spansDiags := flattenSpansQueryV2(ctx, source.SpansQuery, objectType.AttrTypes["spans_query"].(types.ObjectType))
		diags.Append(spansDiags...)
		attrs["spans_query"] = spans
	case source.MetricsQuery != nil:
		metrics, metricsDiags := flattenMetricsQueryV2(ctx, source.MetricsQuery, objectType.AttrTypes["metrics_query"].(types.ObjectType))
		diags.Append(metricsDiags...)
		attrs["metrics_query"] = metrics
	case source.DataprimeQuery != nil:
		dataprime, dataprimeDiags := flattenDataprimeQueryV2(source.DataprimeQuery, objectType.AttrTypes["dataprime_query"].(types.ObjectType))
		diags.Append(dataprimeDiags...)
		attrs["dataprime_query"] = dataprime
	}
	if diags.HasError() {
		return types.ObjectNull(objectType.AttrTypes), diags
	}
	return types.ObjectValueMust(objectType.AttrTypes, attrs), nil
}

func flattenLogsQueryV2(ctx context.Context, query *dashboardservice.QuerySourceLogsQuery, objectType types.ObjectType) (types.Object, diag.Diagnostics) {
	typeType := objectType.AttrTypes["type"].(types.ObjectType)
	typeAttrs := map[string]attr.Value{
		"field_value": nullValueForType(typeType.AttrTypes["field_value"]),
	}
	var diags diag.Diagnostics
	if query.Type != nil && query.Type.FieldValue != nil {
		observationField, obsDiags := dashboardwidgets.FlattenObservationField(ctx, &query.Type.FieldValue.ObservationField)
		diags.Append(obsDiags...)
		typeAttrs["field_value"] = types.ObjectValueMust(
			typeType.AttrTypes["field_value"].(types.ObjectType).AttrTypes,
			map[string]attr.Value{"observation_field": observationField},
		)
	}
	if diags.HasError() {
		return types.ObjectNull(objectType.AttrTypes), diags
	}
	return types.ObjectValueMust(objectType.AttrTypes, map[string]attr.Value{
		"type": types.ObjectValueMust(typeType.AttrTypes, typeAttrs),
	}), nil
}

func flattenSpansQueryV2(ctx context.Context, query *dashboardservice.QuerySourceSpansQuery, objectType types.ObjectType) (types.Object, diag.Diagnostics) {
	typeType := objectType.AttrTypes["type"].(types.ObjectType)
	typeAttrs := map[string]attr.Value{
		"field_value": nullValueForType(typeType.AttrTypes["field_value"]),
	}
	var diags diag.Diagnostics
	if query.Type != nil && query.Type.FieldValue != nil {
		fieldValueType := typeType.AttrTypes["field_value"].(types.ObjectType)
		fieldValueAttrs := map[string]attr.Value{
			"value":             nullValueForType(fieldValueType.AttrTypes["value"]),
			"observation_field": nullValueForType(fieldValueType.AttrTypes["observation_field"]),
		}
		if query.Type.FieldValue.Value != nil {
			spanModel, dg := dashboardwidgets.FlattenSpansField(query.Type.FieldValue.Value)
			if dg != nil {
				diags.Append(dg)
			} else {
				spanObj, spanDiags := types.ObjectValueFrom(ctx, fieldValueType.AttrTypes["value"].(types.ObjectType).AttrTypes, spanModel)
				diags.Append(spanDiags...)
				fieldValueAttrs["value"] = spanObj
			}
		}
		if query.Type.FieldValue.ObservationField != nil {
			observationField, obsDiags := dashboardwidgets.FlattenObservationField(ctx, query.Type.FieldValue.ObservationField)
			diags.Append(obsDiags...)
			fieldValueAttrs["observation_field"] = observationField
		}
		typeAttrs["field_value"] = types.ObjectValueMust(fieldValueType.AttrTypes, fieldValueAttrs)
	}
	if diags.HasError() {
		return types.ObjectNull(objectType.AttrTypes), diags
	}
	return types.ObjectValueMust(objectType.AttrTypes, map[string]attr.Value{
		"type": types.ObjectValueMust(typeType.AttrTypes, typeAttrs),
	}), nil
}

func flattenMetricsQueryV2(ctx context.Context, query *dashboardservice.QuerySourceMetricsQuery, objectType types.ObjectType) (types.Object, diag.Diagnostics) {
	typeType := objectType.AttrTypes["type"].(types.ObjectType)
	typeAttrs := map[string]attr.Value{
		"metric_name":  nullValueForType(typeType.AttrTypes["metric_name"]),
		"label_name":   nullValueForType(typeType.AttrTypes["label_name"]),
		"label_value":  nullValueForType(typeType.AttrTypes["label_value"]),
		"promql_query": nullValueForType(typeType.AttrTypes["promql_query"]),
	}
	var diags diag.Diagnostics
	if query.Type != nil {
		switch {
		case query.Type.MetricName != nil:
			typeAttrs["metric_name"] = types.ObjectValueMust(
				typeType.AttrTypes["metric_name"].(types.ObjectType).AttrTypes,
				map[string]attr.Value{"metric_regex": types.StringPointerValue(query.Type.MetricName.MetricRegex)},
			)
		case query.Type.LabelName != nil:
			typeAttrs["label_name"] = types.ObjectValueMust(
				typeType.AttrTypes["label_name"].(types.ObjectType).AttrTypes,
				map[string]attr.Value{"metric_regex": types.StringPointerValue(query.Type.LabelName.MetricRegex)},
			)
		case query.Type.LabelValue != nil:
			labelValue, labelDiags := flattenMetricsLabelValueV2(ctx, query.Type.LabelValue, typeType.AttrTypes["label_value"].(types.ObjectType))
			diags.Append(labelDiags...)
			typeAttrs["label_value"] = labelValue
		case query.Type.PromqlQuery != nil:
			queryText := types.StringNull()
			if query.Type.PromqlQuery.Query != nil {
				queryText = types.StringPointerValue(query.Type.PromqlQuery.Query.Value)
			}
			promqlType := types.StringValue("instant")
			if query.Type.PromqlQuery.PromqlQueryType != nil {
				if mapped, ok := dashboardwidgets.DashboardProtoToSchemaPromQLQueryType[*query.Type.PromqlQuery.PromqlQueryType]; ok && mapped != utils.UNSPECIFIED {
					promqlType = types.StringValue(mapped)
				}
			}
			typeAttrs["promql_query"] = types.ObjectValueMust(
				typeType.AttrTypes["promql_query"].(types.ObjectType).AttrTypes,
				map[string]attr.Value{
					"query":             queryText,
					"promql_query_type": promqlType,
				},
			)
		}
	}
	if diags.HasError() {
		return types.ObjectNull(objectType.AttrTypes), diags
	}
	return types.ObjectValueMust(objectType.AttrTypes, map[string]attr.Value{
		"type": types.ObjectValueMust(typeType.AttrTypes, typeAttrs),
	}), nil
}

func flattenMetricsLabelValueV2(ctx context.Context, value *dashboardservice.QuerySourceMetricsQueryTypeLabelValue, objectType types.ObjectType) (types.Object, diag.Diagnostics) {
	metricName, diags := flattenStringOrVariableV2(ctx, value.MetricName, objectType.AttrTypes["metric_name"].(types.ObjectType))
	if diags.HasError() {
		return types.ObjectNull(objectType.AttrTypes), diags
	}
	labelName, labelDiags := flattenStringOrVariableV2(ctx, &value.LabelName, objectType.AttrTypes["label_name"].(types.ObjectType))
	diags.Append(labelDiags...)
	if diags.HasError() {
		return types.ObjectNull(objectType.AttrTypes), diags
	}
	filtersType := objectType.AttrTypes["label_filters"].(types.ListType)
	filters, filterDiags := flattenMetricsLabelFiltersV2(ctx, value.LabelFilters, filtersType)
	diags.Append(filterDiags...)
	if diags.HasError() {
		return types.ObjectNull(objectType.AttrTypes), diags
	}
	return types.ObjectValueMust(objectType.AttrTypes, map[string]attr.Value{
		"metric_name":   metricName,
		"label_name":    labelName,
		"label_filters": filters,
	}), nil
}

func flattenMetricsLabelFiltersV2(ctx context.Context, filters []dashboardservice.QuerySourceMetricsQueryMetricsLabelFilter, listType types.ListType) (types.List, diag.Diagnostics) {
	if len(filters) == 0 {
		return types.ListNull(listType.ElemType), nil
	}
	entryType := listType.ElemType.(types.ObjectType)
	elements := make([]attr.Value, 0, len(filters))
	var diags diag.Diagnostics
	for i := range filters {
		element, elementDiags := flattenMetricsLabelFilterV2(ctx, &filters[i], entryType)
		diags.Append(elementDiags...)
		if elementDiags.HasError() {
			continue
		}
		elements = append(elements, element)
	}
	if diags.HasError() {
		return types.ListNull(listType.ElemType), diags
	}
	return types.ListValueMust(entryType, elements), nil
}

func flattenMetricsLabelFilterV2(ctx context.Context, filter *dashboardservice.QuerySourceMetricsQueryMetricsLabelFilter, objectType types.ObjectType) (types.Object, diag.Diagnostics) {
	metric, diags := flattenStringOrVariableV2(ctx, filter.Metric, objectType.AttrTypes["metric"].(types.ObjectType))
	if diags.HasError() {
		return types.ObjectNull(objectType.AttrTypes), diags
	}
	label, labelDiags := flattenStringOrVariableV2(ctx, filter.Label, objectType.AttrTypes["label"].(types.ObjectType))
	diags.Append(labelDiags...)
	if diags.HasError() {
		return types.ObjectNull(objectType.AttrTypes), diags
	}
	operator, operatorDiags := flattenMetricsLabelFilterOperatorV2(ctx, filter.Operator, objectType.AttrTypes["operator"].(types.ObjectType))
	diags.Append(operatorDiags...)
	if diags.HasError() {
		return types.ObjectNull(objectType.AttrTypes), diags
	}
	return types.ObjectValueMust(objectType.AttrTypes, map[string]attr.Value{
		"metric":   metric,
		"label":    label,
		"operator": operator,
	}), nil
}

func flattenMetricsLabelFilterOperatorV2(ctx context.Context, operator *dashboardservice.QuerySourceMetricsQueryOperator, objectType types.ObjectType) (types.Object, diag.Diagnostics) {
	if operator == nil {
		return types.ObjectNull(objectType.AttrTypes), nil
	}
	var values []dashboardservice.QuerySourceMetricsQueryStringOrVariable
	operatorType := ""
	switch {
	case operator.Equals != nil:
		operatorType = "equals"
		if operator.Equals.Selection != nil && operator.Equals.Selection.List != nil {
			values = operator.Equals.Selection.List.Values
		}
	case operator.NotEquals != nil:
		operatorType = "not_equals"
		if operator.NotEquals.Selection != nil && operator.NotEquals.Selection.List != nil {
			values = operator.NotEquals.Selection.List.Values
		}
	default:
		return types.ObjectNull(objectType.AttrTypes), nil
	}
	selectedValues, diags := flattenStringOrVariablesV2(ctx, values, objectType.AttrTypes["selected_values"].(types.ListType))
	if diags.HasError() {
		return types.ObjectNull(objectType.AttrTypes), diags
	}
	return types.ObjectValueMust(objectType.AttrTypes, map[string]attr.Value{
		"type":            types.StringValue(operatorType),
		"selected_values": selectedValues,
	}), nil
}

func flattenStringOrVariablesV2(ctx context.Context, values []dashboardservice.QuerySourceMetricsQueryStringOrVariable, listType types.ListType) (types.List, diag.Diagnostics) {
	if len(values) == 0 {
		return types.ListNull(listType.ElemType), nil
	}
	entryType := listType.ElemType.(types.ObjectType)
	elements := make([]attr.Value, 0, len(values))
	var diags diag.Diagnostics
	for i := range values {
		element, elementDiags := flattenStringOrVariableV2(ctx, &values[i], entryType)
		diags.Append(elementDiags...)
		if elementDiags.HasError() {
			continue
		}
		elements = append(elements, element)
	}
	if diags.HasError() {
		return types.ListNull(listType.ElemType), diags
	}
	return types.ListValueMust(entryType, elements), nil
}

func flattenStringOrVariableV2(ctx context.Context, value *dashboardservice.QuerySourceMetricsQueryStringOrVariable, objectType types.ObjectType) (types.Object, diag.Diagnostics) {
	_ = ctx
	if value == nil {
		return types.ObjectNull(objectType.AttrTypes), nil
	}
	return types.ObjectValueMust(objectType.AttrTypes, map[string]attr.Value{
		"string_value":  types.StringPointerValue(value.StringValue),
		"variable_name": types.StringPointerValue(value.VariableName),
	}), nil
}

func flattenDataprimeQueryV2(query *dashboardservice.QuerySourceDataprimeQuery, objectType types.ObjectType) (types.Object, diag.Diagnostics) {
	typeType := objectType.AttrTypes["type"].(types.ObjectType)
	queryTextType := typeType.AttrTypes["query_text"].(types.ObjectType)
	queryTextAttrs := map[string]attr.Value{
		"query":          types.StringNull(),
		"data_mode_type": types.StringNull(),
	}
	if query.Type != nil && query.Type.QueryText != nil {
		if query.Type.QueryText.Query != nil {
			queryTextAttrs["query"] = types.StringPointerValue(query.Type.QueryText.Query.Text)
		}
		if query.Type.QueryText.DataModeType != nil {
			queryTextAttrs["data_mode_type"] = types.StringValue(dashboardwidgets.DashboardProtoToSchemaDataModeTypeV2[*query.Type.QueryText.DataModeType])
		}
	}
	return types.ObjectValueMust(objectType.AttrTypes, map[string]attr.Value{
		"type": types.ObjectValueMust(typeType.AttrTypes, map[string]attr.Value{
			"query_text": types.ObjectValueMust(queryTextType.AttrTypes, queryTextAttrs),
		}),
	}), nil
}

func flattenVariableValueV2(ctx context.Context, value *dashboardservice.VariableValueV2, objectType types.ObjectType) (types.Object, diag.Diagnostics) {
	if value == nil {
		return types.ObjectNull(objectType.AttrTypes), nil
	}
	attrs := map[string]attr.Value{
		"single_string":  nullValueForType(objectType.AttrTypes["single_string"]),
		"single_numeric": nullValueForType(objectType.AttrTypes["single_numeric"]),
		"regex":          nullValueForType(objectType.AttrTypes["regex"]),
		"lucene":         nullValueForType(objectType.AttrTypes["lucene"]),
		"interval":       nullValueForType(objectType.AttrTypes["interval"]),
		"multi_string":   nullValueForType(objectType.AttrTypes["multi_string"]),
	}
	var diags diag.Diagnostics
	switch {
	case value.SingleString != nil:
		attrs["single_string"] = flattenStringValueLabelFlat(value.SingleString.Value, objectType.AttrTypes["single_string"].(types.ObjectType))
	case value.SingleNumeric != nil:
		attrs["single_numeric"] = flattenNumericValueLabelFlat(value.SingleNumeric.Value, objectType.AttrTypes["single_numeric"].(types.ObjectType))
	case value.Regex != nil:
		attrs["regex"] = flattenStringValueLabelFlat(value.Regex.Value, objectType.AttrTypes["regex"].(types.ObjectType))
	case value.Lucene != nil:
		attrs["lucene"] = flattenStringValueLabelFlat(value.Lucene.Value, objectType.AttrTypes["lucene"].(types.ObjectType))
	case value.Interval != nil:
		attrs["interval"] = flattenStringValueLabelFlat(value.Interval.Value, objectType.AttrTypes["interval"].(types.ObjectType))
	case value.MultiString != nil:
		multi, multiDiags := flattenMultiStringValueV2(ctx, value.MultiString, objectType.AttrTypes["multi_string"].(types.ObjectType))
		diags.Append(multiDiags...)
		attrs["multi_string"] = multi
	}
	if diags.HasError() {
		return types.ObjectNull(objectType.AttrTypes), diags
	}
	return types.ObjectValueMust(objectType.AttrTypes, attrs), nil
}

func flattenStringValueLabelFlat(value *dashboardservice.StringValueLabel, objectType types.ObjectType) types.Object {
	if value == nil {
		return types.ObjectNull(objectType.AttrTypes)
	}
	return types.ObjectValueMust(objectType.AttrTypes, map[string]attr.Value{
		"value": types.StringPointerValue(value.Value),
		"label": types.StringPointerValue(value.Label),
	})
}

func flattenNumericValueLabelFlat(value *dashboardservice.NumericValueLabel, objectType types.ObjectType) types.Object {
	if value == nil {
		return types.ObjectNull(objectType.AttrTypes)
	}
	return types.ObjectValueMust(objectType.AttrTypes, map[string]attr.Value{
		"value": float32PointerToTypeFloat64(value.Value),
		"label": types.StringPointerValue(value.Label),
	})
}

func flattenMultiStringValueV2(ctx context.Context, value *dashboardservice.MultiStringValue, objectType types.ObjectType) (types.Object, diag.Diagnostics) {
	attrs := map[string]attr.Value{
		"selected_all": nullValueForType(objectType.AttrTypes["selected_all"]),
		"all":          nullValueForType(objectType.AttrTypes["all"]),
		"list":         nullValueForType(objectType.AttrTypes["list"]),
	}
	switch {
	case value.SelectedAll != nil:
		attrs["selected_all"] = types.ObjectValueMust(objectType.AttrTypes["selected_all"].(types.ObjectType).AttrTypes, map[string]attr.Value{})
	case value.All != nil:
		attrs["all"] = types.ObjectValueMust(objectType.AttrTypes["all"].(types.ObjectType).AttrTypes, map[string]attr.Value{})
	case value.List != nil:
		listType := objectType.AttrTypes["list"].(types.ObjectType)
		valuesType := listType.AttrTypes["values"].(types.ListType)
		values, diags := flattenMultiStringListValuesV2(ctx, value.List.Values, valuesType)
		if diags.HasError() {
			return types.ObjectNull(objectType.AttrTypes), diags
		}
		attrs["list"] = types.ObjectValueMust(listType.AttrTypes, map[string]attr.Value{"values": values})
	}
	return types.ObjectValueMust(objectType.AttrTypes, attrs), nil
}

func flattenMultiStringListValuesV2(ctx context.Context, values []dashboardservice.SingleStringValue, listType types.ListType) (types.List, diag.Diagnostics) {
	_ = ctx
	entryType := listType.ElemType.(types.ObjectType)
	valueType := entryType.AttrTypes["value"].(types.ObjectType)
	// Keep empty lists as known empty (not null) so values = [] round-trips.
	elements := make([]attr.Value, 0, len(values))
	for _, value := range values {
		elements = append(elements, types.ObjectValueMust(entryType.AttrTypes, map[string]attr.Value{
			"value": flattenStringValueLabelFlat(value.Value, valueType),
		}))
	}
	return types.ListValueMust(entryType, elements), nil
}

func dashboardVariablesV2ElementType() basetypes.ObjectType {
	root := dashboardschema.V4().Type().(basetypes.ObjectType)
	list := root.AttrTypes["variables_v2"].(basetypes.ListType)
	return list.ElemType.(basetypes.ObjectType)
}

func nullValueForType(attrType attr.Type) attr.Value {
	switch {
	case attrType.Equal(types.StringType):
		return types.StringNull()
	case attrType.Equal(types.BoolType):
		return types.BoolNull()
	case attrType.Equal(types.Float64Type):
		return types.Float64Null()
	case attrType.Equal(types.Int64Type):
		return types.Int64Null()
	}

	switch typed := attrType.(type) {
	case types.ObjectType:
		return types.ObjectNull(typed.AttrTypes)
	case types.ListType:
		return types.ListNull(typed.ElemType)
	default:
		return types.DynamicNull()
	}
}

func typeFloat64ToFloat32Pointer(value types.Float64) *float32 {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	converted := float32(value.ValueFloat64())
	return &converted
}

func float32PointerToTypeFloat64(value *float32) types.Float64 {
	if value == nil {
		return types.Float64Null()
	}
	return types.Float64Value(float32ToSchemaFloat64(*value))
}

func float32ToSchemaFloat64(value float32) float64 {
	parsed, err := strconv.ParseFloat(strconv.FormatFloat(float64(value), 'f', -1, 32), 64)
	if err != nil {
		return float64(value)
	}
	return parsed
}

func boolPointerIfSet(value types.Bool) *bool {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	return value.ValueBoolPointer()
}

// requiredEnumValue converts a Terraform string to a required OpenAPI enum value.
// Null/unknown/unmapped values become the zero value (empty string). Schema defaults
// keep real plans off this path; zero is only for incomplete expand fixtures.
func requiredEnumValue[T ~string](value types.String, values map[string]T) T {
	if p := dashboardwidgets.OptionalEnumPointer(value, values); p != nil {
		return *p
	}
	var zero T
	return zero
}

func derefOrZero[T any](value *T) T {
	if value == nil {
		var zero T
		return zero
	}
	return *value
}

func flattenOrderDirectionV2(direction dashboardservice.OrderDirection) types.String {
	if mapped, ok := dashboardwidgets.DashboardOrderDirectionProtoToSchemaV2[direction]; ok {
		return types.StringValue(mapped)
	}
	return types.StringNull()
}
