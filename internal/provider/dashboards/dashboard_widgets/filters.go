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

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func ExpandMetricsFilters(ctx context.Context, metricFilters types.List) ([]dashboardservice.MetricsFilter, diag.Diagnostics) {
	var metricFiltersObjects []types.Object
	var expandedMetricFilters []dashboardservice.MetricsFilter
	diags := metricFilters.ElementsAs(ctx, &metricFiltersObjects, true)
	if diags.HasError() {
		return nil, diags
	}
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
		expandedMetricFilters = append(expandedMetricFilters, *expandedMetricFilter)
	}

	return expandedMetricFilters, diags
}

func expandMetricFilter(ctx context.Context, metricFilter MetricsFilterModel) (*dashboardservice.MetricsFilter, diag.Diagnostics) {
	operator, diags := expandFilterOperator(ctx, metricFilter.Operator)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.MetricsFilter{
		Metric:   utils.TypeStringToStringPointer(metricFilter.Metric),
		Label:    utils.TypeStringToStringPointer(metricFilter.Label),
		Operator: operator,
	}, nil
}

func ExpandLogsFilters(ctx context.Context, logsFilters types.List) ([]dashboardservice.FilterLogsFilter, diag.Diagnostics) {
	var filtersObjects []types.Object
	var expandedFilters []dashboardservice.FilterLogsFilter
	diags := logsFilters.ElementsAs(ctx, &filtersObjects, true)
	if diags.HasError() {
		return nil, diags
	}

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
		expandedFilters = append(expandedFilters, *expandedFilter)
	}

	return expandedFilters, diags
}

func expandLogsFilter(ctx context.Context, logsFilter LogsFilterModel) (*dashboardservice.FilterLogsFilter, diag.Diagnostics) {
	operator, diags := expandFilterOperator(ctx, logsFilter.Operator)
	if diags.HasError() {
		return nil, diags
	}

	observationField, diags := ExpandObservationFieldObject(ctx, logsFilter.ObservationField)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.FilterLogsFilter{
		Field:            utils.TypeStringToStringPointer(logsFilter.Field),
		Operator:         operator,
		ObservationField: observationField,
	}, nil
}

func ExpandLuceneQuery(luceneQuery types.String) *dashboardservice.LuceneQuery {
	if luceneQuery.IsNull() || luceneQuery.IsUnknown() {
		return nil
	}
	return &dashboardservice.LuceneQuery{
		Value: luceneQuery.ValueStringPointer(),
	}
}

func ExpandFilterSource(ctx context.Context, source *DashboardFilterSourceModel) (*dashboardservice.FilterSource, diag.Diagnostics) {
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
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Filter Source", fmt.Sprintf("Unknown filter source type: %#v", source))}
	}
}
func expandFilterSourceLogs(ctx context.Context, logs *FilterSourceLogsModel) (*dashboardservice.FilterSource, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	operator, diags := expandFilterOperator(ctx, logs.Operator)
	if diags.HasError() {
		return nil, diags
	}

	observationField, diags := ExpandObservationFieldObject(ctx, logs.ObservationField)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.FilterSource{
		Logs: &dashboardservice.FilterLogsFilter{
			Field:            utils.TypeStringToStringPointer(logs.Field),
			Operator:         operator,
			ObservationField: observationField,
		},
	}, nil
}

func expandFilterSourceMetrics(ctx context.Context, metrics *FilterSourceMetricsModel) (*dashboardservice.FilterSource, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	operator, diags := expandFilterOperator(ctx, metrics.Operator)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.FilterSource{
		Metrics: &dashboardservice.MetricsFilter{
			Metric:   utils.TypeStringToStringPointer(metrics.MetricName),
			Label:    utils.TypeStringToStringPointer(metrics.MetricLabel),
			Operator: operator,
		},
	}, nil
}

func expandFilterSourceSpans(ctx context.Context, spans *FilterSourceSpansModel) (*dashboardservice.FilterSource, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	field, dg := ExpandSpansField(spans.Field)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	operator, diags := expandFilterOperator(ctx, spans.Operator)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.FilterSource{
		Spans: &dashboardservice.SpansFilter{
			Field:    field,
			Operator: operator,
		},
	}, nil
}

func expandFilterOperator(ctx context.Context, operator *FilterOperatorModel) (*dashboardservice.FilterOperator, diag.Diagnostics) {
	if operator == nil {
		return nil, nil
	}

	selectedValues, diags := typeStringListToStringSlice(ctx, operator.SelectedValues)
	if diags.HasError() {
		return nil, diags
	}

	switch operator.Type.ValueString() {
	case "equals":
		filterOperator := &dashboardservice.FilterOperator{
			Equals: &dashboardservice.FilterEquals{
				Selection: &dashboardservice.EqualsSelection{},
			},
		}
		if len(selectedValues) != 0 {
			filterOperator.Equals.Selection.List = &dashboardservice.EqualsSelectionListSelection{Values: selectedValues}
		} else {
			filterOperator.Equals.Selection.All = map[string]interface{}{}
		}
		return filterOperator, nil
	case "not_equals":
		return &dashboardservice.FilterOperator{
			NotEquals: &dashboardservice.FilterNotEquals{
				Selection: &dashboardservice.NotEqualsSelection{
					List: &dashboardservice.NotEqualsSelectionListSelection{
						Values: selectedValues,
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

func ExpandPromqlQuery(promqlQuery types.String) *dashboardservice.PromQlQuery {
	if promqlQuery.IsNull() || promqlQuery.IsUnknown() {
		return nil
	}

	return &dashboardservice.PromQlQuery{
		Value: promqlQuery.ValueStringPointer(),
	}
}

func ExpandSpansAggregations(ctx context.Context, aggregations types.List) ([]dashboardservice.SpansAggregation, diag.Diagnostics) {
	var aggregationsObjects []types.Object
	var expandedAggregations []dashboardservice.SpansAggregation
	diags := aggregations.ElementsAs(ctx, &aggregationsObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, ao := range aggregationsObjects {
		var aggregation SpansAggregationModel
		if dg := ao.As(ctx, &aggregation, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedAggregation, expandDiag := ExpandSpansAggregation(&aggregation)
		if expandDiag != nil {
			diags.Append(expandDiag)
			continue
		}
		expandedAggregations = append(expandedAggregations, *expandedAggregation)
	}

	return expandedAggregations, diags
}

func ExpandSpansAggregation(spansAggregation *SpansAggregationModel) (*dashboardservice.SpansAggregation, diag.Diagnostic) {
	if spansAggregation == nil {
		return nil, nil
	}

	switch spansAggregation.Type.ValueString() {
	case "metric":
		return &dashboardservice.SpansAggregation{
			MetricAggregation: &dashboardservice.MetricAggregation{
				MetricField:     OptionalEnumPointer(spansAggregation.Field, DashboardSchemaToProtoSpansAggregationMetricField),
				AggregationType: OptionalEnumPointer(spansAggregation.AggregationType, DashboardSchemaToProtoSpansAggregationMetricAggregationType),
			},
		}, nil
	case "dimension":
		return &dashboardservice.SpansAggregation{
			DimensionAggregation: &dashboardservice.DimensionAggregation{
				DimensionField:  OptionalEnumPointer(spansAggregation.Field, DashboardProtoToSchemaSpansAggregationDimensionField),
				AggregationType: OptionalEnumPointer(spansAggregation.AggregationType, DashboardSchemaToProtoSpansAggregationDimensionAggregationType),
			},
		}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Extract Spans Aggregation Error", fmt.Sprintf("Unknown spans aggregation type %#v", spansAggregation))
	}
}

func ExpandSpansFilters(ctx context.Context, spansFilters types.List) ([]dashboardservice.SpansFilter, diag.Diagnostics) {
	var spansFiltersObjects []types.Object
	var expandedSpansFilters []dashboardservice.SpansFilter
	diags := spansFilters.ElementsAs(ctx, &spansFiltersObjects, true)
	if diags.HasError() {
		return nil, diags
	}
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
		expandedSpansFilters = append(expandedSpansFilters, *expandedSpansFilter)
	}

	return expandedSpansFilters, diags
}

func expandSpansFilter(ctx context.Context, spansFilter SpansFilterModel) (*dashboardservice.SpansFilter, diag.Diagnostics) {
	operator, diags := expandFilterOperator(ctx, spansFilter.Operator)
	if diags.HasError() {
		return nil, diags
	}

	field, dg := ExpandSpansField(spansFilter.Field)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &dashboardservice.SpansFilter{
		Field:    field,
		Operator: operator,
	}, nil
}
