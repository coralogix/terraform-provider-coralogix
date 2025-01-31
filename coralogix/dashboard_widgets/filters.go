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

package dashboardwidgets

import (
	"context"
	"fmt"
	"terraform-provider-coralogix/coralogix/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func ExpandMetricsFilters(ctx context.Context, metricFilters types.List) ([]*cxsdk.DashboardMetricsFilter, diag.Diagnostics) {
	var metricFiltersObjects []types.Object
	var expandedMetricFilters []*cxsdk.DashboardMetricsFilter
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
		expandedMetricFilters = append(expandedMetricFilters, expandedMetricFilter)
	}

	return expandedMetricFilters, diags
}

func expandMetricFilter(ctx context.Context, metricFilter MetricsFilterModel) (*cxsdk.DashboardMetricsFilter, diag.Diagnostics) {
	operator, diags := expandFilterOperator(ctx, metricFilter.Operator)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.DashboardMetricsFilter{
		Metric:   utils.TypeStringToWrapperspbString(metricFilter.Metric),
		Label:    utils.TypeStringToWrapperspbString(metricFilter.Label),
		Operator: operator,
	}, nil
}

func ExpandLogsFilters(ctx context.Context, logsFilters types.List) ([]*cxsdk.DashboardFilterLogsFilter, diag.Diagnostics) {
	var filtersObjects []types.Object
	var expandedFilters []*cxsdk.DashboardFilterLogsFilter
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
		expandedFilters = append(expandedFilters, expandedFilter)
	}

	return expandedFilters, diags
}

func expandLogsFilter(ctx context.Context, logsFilter LogsFilterModel) (*cxsdk.DashboardFilterLogsFilter, diag.Diagnostics) {
	operator, diags := expandFilterOperator(ctx, logsFilter.Operator)
	if diags.HasError() {
		return nil, diags
	}

	observationField, diags := expandObservationFieldObject(ctx, logsFilter.ObservationField)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.DashboardFilterLogsFilter{
		Field:            utils.TypeStringToWrapperspbString(logsFilter.Field),
		Operator:         operator,
		ObservationField: observationField,
	}, nil
}

func ExpandLuceneQuery(luceneQuery types.String) *cxsdk.DashboardLuceneQuery {
	if luceneQuery.IsNull() || luceneQuery.IsUnknown() {
		return nil
	}
	return &cxsdk.DashboardLuceneQuery{
		Value: wrapperspb.String(luceneQuery.ValueString()),
	}
}

func ExpandFilterSource(ctx context.Context, source *DashboardFilterSourceModel) (*cxsdk.DashboardFilterSource, diag.Diagnostics) {
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
func expandFilterSourceLogs(ctx context.Context, logs *FilterSourceLogsModel) (*cxsdk.DashboardFilterSource, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	operator, diags := expandFilterOperator(ctx, logs.Operator)
	if diags.HasError() {
		return nil, diags
	}

	observationField, diags := expandObservationFieldObject(ctx, logs.ObservationField)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.DashboardFilterSource{
		Value: &cxsdk.DashboardFilterSourceLogs{
			Logs: &cxsdk.DashboardFilterLogsFilter{
				Field:            utils.TypeStringToWrapperspbString(logs.Field),
				Operator:         operator,
				ObservationField: observationField,
			},
		},
	}, nil
}

func expandFilterSourceMetrics(ctx context.Context, metrics *FilterSourceMetricsModel) (*cxsdk.DashboardFilterSource, diag.Diagnostics) {
	if metrics == nil {
		return nil, nil
	}

	operator, diags := expandFilterOperator(ctx, metrics.Operator)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.DashboardFilterSource{
		Value: &cxsdk.DashboardFilterSourceMetrics{
			Metrics: &cxsdk.DashboardFilterMetricsFilter{
				Metric:   utils.TypeStringToWrapperspbString(metrics.MetricName),
				Label:    utils.TypeStringToWrapperspbString(metrics.MetricLabel),
				Operator: operator,
			},
		},
	}, nil
}

func expandFilterSourceSpans(ctx context.Context, spans *FilterSourceSpansModel) (*cxsdk.DashboardFilterSource, diag.Diagnostics) {
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

	return &cxsdk.DashboardFilterSource{
		Value: &cxsdk.DashboardFilterSourceSpans{
			Spans: &cxsdk.DashboardFilterSpansFilter{
				Field:    field,
				Operator: operator,
			},
		},
	}, nil
}

func expandFilterOperator(ctx context.Context, operator *FilterOperatorModel) (*cxsdk.DashboardFilterOperator, diag.Diagnostics) {
	if operator == nil {
		return nil, nil
	}

	selectedValues, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, operator.SelectedValues.Elements())
	if diags.HasError() {
		return nil, diags
	}

	switch operator.Type.ValueString() {
	case "equals":
		filterOperator := &cxsdk.DashboardFilterOperator{
			Value: &cxsdk.DashboardFilterOperatorEquals{
				Equals: &cxsdk.DashboardFilterEquals{
					Selection: &cxsdk.DashboardFilterEqualsSelection{},
				},
			},
		}
		if len(selectedValues) != 0 {
			filterOperator.GetEquals().Selection.Value = &cxsdk.DashboardFilterEqualsSelectionList{
				List: &cxsdk.DashboardFilterEqualsSelectionListSelection{
					Values: selectedValues,
				},
			}
		} else {
			filterOperator.GetEquals().Selection.Value = &cxsdk.DashboardFilterEqualsSelectionAll{
				All: &cxsdk.DashboardFilterEqualsSelectionAllSelection{},
			}
		}
		return filterOperator, nil
	case "not_equals":
		return &cxsdk.DashboardFilterOperator{
			Value: &cxsdk.DashboardFilterOperatorNotEquals{
				NotEquals: &cxsdk.DashboardFilterNotEquals{
					Selection: &cxsdk.DashboardFilterNotEqualsSelection{
						Value: &cxsdk.DashboardFilterNotEqualsSelectionList{
							List: &cxsdk.DashboardFilterNotEqualsSelectionListSelection{
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

func expandPromqlQuery(promqlQuery types.String) *cxsdk.DashboardPromQLQuery {
	if promqlQuery.IsNull() || promqlQuery.IsUnknown() {
		return nil
	}

	return &cxsdk.DashboardPromQLQuery{
		Value: wrapperspb.String(promqlQuery.ValueString()),
	}
}

func ExpandSpansAggregations(ctx context.Context, aggregations types.List) ([]*cxsdk.SpansAggregation, diag.Diagnostics) {
	var aggregationsObjects []types.Object
	var expandedAggregations []*cxsdk.SpansAggregation
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
		expandedAggregations = append(expandedAggregations, expandedAggregation)
	}

	return expandedAggregations, diags
}

func ExpandSpansAggregation(spansAggregation *SpansAggregationModel) (*cxsdk.SpansAggregation, diag.Diagnostic) {
	if spansAggregation == nil {
		return nil, nil
	}

	switch spansAggregation.Type.ValueString() {
	case "metric":
		return &cxsdk.SpansAggregation{
			Aggregation: &cxsdk.SpansAggregationMetricAggregation{
				MetricAggregation: &cxsdk.SpansAggregationMetricAggregationInner{
					MetricField:     DashboardSchemaToProtoSpansAggregationMetricField[spansAggregation.Field.ValueString()],
					AggregationType: DashboardSchemaToProtoSpansAggregationMetricAggregationType[spansAggregation.AggregationType.ValueString()],
				},
			},
		}, nil
	case "dimension":
		return &cxsdk.SpansAggregation{
			Aggregation: &cxsdk.SpansAggregationDimensionAggregation{
				DimensionAggregation: &cxsdk.SpansAggregationDimensionAggregationInner{
					DimensionField:  DashboardProtoToSchemaSpansAggregationDimensionField[spansAggregation.Field.ValueString()],
					AggregationType: DashboardSchemaToProtoSpansAggregationDimensionAggregationType[spansAggregation.AggregationType.ValueString()],
				},
			},
		}, nil
	default:
		return nil, diag.NewErrorDiagnostic("Extract Spans Aggregation Error", fmt.Sprintf("Unknown spans aggregation type %#v", spansAggregation))
	}
}

func ExpandSpansFilters(ctx context.Context, spansFilters types.List) ([]*cxsdk.DashboardFilterSpansFilter, diag.Diagnostics) {
	var spansFiltersObjects []types.Object
	var expandedSpansFilters []*cxsdk.DashboardFilterSpansFilter
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
		expandedSpansFilters = append(expandedSpansFilters, expandedSpansFilter)
	}

	return expandedSpansFilters, diags
}

func expandSpansFilter(ctx context.Context, spansFilter SpansFilterModel) (*cxsdk.DashboardFilterSpansFilter, diag.Diagnostics) {
	operator, diags := expandFilterOperator(ctx, spansFilter.Operator)
	if diags.HasError() {
		return nil, diags
	}

	field, dg := ExpandSpansField(spansFilter.Field)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &cxsdk.DashboardFilterSpansFilter{
		Field:    field,
		Operator: operator,
	}, nil
}
