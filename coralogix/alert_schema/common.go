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

package alertschema

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NotificationGroupAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"group_by_keys": types.ListType{
			ElemType: types.StringType,
		},
		"webhooks_settings": types.SetType{
			ElemType: types.ObjectType{
				AttrTypes: WebhooksSettingsAttr(),
			},
		},
		"destinations": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: NotificationDestinationsAttr(),
			},
		},
		"router": types.ObjectType{
			AttrTypes: NotificationRouterAttr(),
		},
	}
}

func WebhooksSettingsAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"notify_on": types.StringType,
		"retriggering_period": types.ObjectType{
			AttrTypes: RetriggeringPeriodAttr(),
		},
		"integration_id": types.StringType,
		"recipients":     types.SetType{ElemType: types.StringType},
	}
}

func NotificationDestinationsAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"connector_id": types.StringType,
		"preset_id":    types.StringType,
		"notify_on":    types.StringType,
		"triggered_routing_overrides": types.ObjectType{
			AttrTypes: RoutingOverridesAttr(),
		},
		"resolved_routing_overrides": types.ObjectType{
			AttrTypes: RoutingOverridesAttr(),
		},
	}
}

func RoutingOverridesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"connector_overrides": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: ConfigurationOverridesAttr(),
			},
		},
		"preset_overrides": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: ConfigurationOverridesAttr(),
			},
		},
		"payload_type": types.StringType,
	}
}

func ConfigurationOverridesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field_name": types.StringType,
		"template":   types.StringType,
	}
}

func NotificationRouterAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"notify_on": types.StringType,
	}
}

func AlertTypeDefinitionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_immediate": types.ObjectType{
			AttrTypes: LogsImmediateAttr(),
		},
		"logs_threshold": types.ObjectType{
			AttrTypes: LogsThresholdAttr(),
		},
		"logs_anomaly": types.ObjectType{
			AttrTypes: LogsAnomalyAttr(),
		},
		"logs_ratio_threshold": types.ObjectType{
			AttrTypes: LogsRatioThresholdAttr(),
		},
		"logs_new_value": types.ObjectType{
			AttrTypes: LogsNewValueAttr(),
		},
		"logs_unique_count": types.ObjectType{
			AttrTypes: LogsUniqueCountAttr(),
		},
		"logs_time_relative_threshold": types.ObjectType{
			AttrTypes: LogsTimeRelativeAttr(),
		},
		"metric_threshold": types.ObjectType{
			AttrTypes: MetricThresholdAttr(),
		},
		"metric_anomaly": types.ObjectType{
			AttrTypes: MetricAnomalyAttr(),
		},
		"tracing_immediate": types.ObjectType{
			AttrTypes: TracingImmediateAttr(),
		},
		"tracing_threshold": types.ObjectType{
			AttrTypes: TracingThresholdAttr(),
		},
		"flow": types.ObjectType{
			AttrTypes: FlowAttr(),
		},
		"slo_threshold": types.ObjectType{
			AttrTypes: SloThresholdAttr(),
		},
	}
}

func LogsImmediateAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_filter": types.ObjectType{
			AttrTypes: LogsFilterAttr(),
		},
		"notification_payload_filter": types.SetType{
			ElemType: types.StringType,
		},
	}
}

func LogsFilterAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"simple_filter": types.ObjectType{
			AttrTypes: LuceneFilterAttr(),
		},
	}
}

func LuceneFilterAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"lucene_query": types.StringType,
		"label_filters": types.ObjectType{
			AttrTypes: LabelFiltersAttr(),
		},
	}
}

func LabelFiltersAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"application_name": types.SetType{
			ElemType: types.ObjectType{
				AttrTypes: LabelFilterTypesAttr(),
			},
		},
		"subsystem_name": types.SetType{
			ElemType: types.ObjectType{
				AttrTypes: LabelFilterTypesAttr(),
			},
		},
		"severities": types.SetType{
			ElemType: types.StringType,
		},
	}
}

func LogsThresholdAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_filter":                  types.ObjectType{AttrTypes: LogsFilterAttr()},
		"notification_payload_filter":  types.SetType{ElemType: types.StringType},
		"rules":                        types.SetType{ElemType: types.ObjectType{AttrTypes: LogsThresholdRulesAttr()}},
		"undetected_values_management": types.ObjectType{AttrTypes: UndetectedValuesManagementAttr()},
		"custom_evaluation_delay":      types.Int32Type,
	}
}

func LogsThresholdRulesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{AttrTypes: LogsThresholdConditionAttr()},
		"override":  types.ObjectType{AttrTypes: AlertOverrideAttr()},
	}
}

func LogsThresholdConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"threshold":      types.Float64Type,
		"time_window":    types.StringType,
		"condition_type": types.StringType,
	}
}

func LogsAnomalyAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_filter":                 types.ObjectType{AttrTypes: LogsFilterAttr()},
		"rules":                       types.SetType{ElemType: types.ObjectType{AttrTypes: LogsAnomalyRulesAttr()}},
		"notification_payload_filter": types.SetType{ElemType: types.StringType},
		"custom_evaluation_delay":     types.Int32Type,
	}
}

func LogsAnomalyRulesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{AttrTypes: LogsAnomalyConditionAttr()},
	}
}

func LogsAnomalyConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"minimum_threshold": types.Float64Type,
		"time_window":       types.StringType,
		"condition_type":    types.StringType,
	}
}

func LogsRatioThresholdAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"numerator":         types.ObjectType{AttrTypes: LogsFilterAttr()},
		"numerator_alias":   types.StringType,
		"denominator":       types.ObjectType{AttrTypes: LogsFilterAttr()},
		"denominator_alias": types.StringType,
		"rules":             types.SetType{ElemType: types.ObjectType{AttrTypes: LogsRatioThresholdRulesAttr()}},
		"notification_payload_filter": types.SetType{
			ElemType: types.StringType,
		},
		"group_by_for":            types.StringType,
		"custom_evaluation_delay": types.Int32Type,
	}
}

func LogsRatioThresholdRulesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{AttrTypes: LogsRatioThresholdRuleConditionAttr()},
		"override":  types.ObjectType{AttrTypes: AlertOverrideAttr()},
	}
}

func LogsRatioThresholdRuleConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"threshold":      types.Float64Type,
		"time_window":    types.StringType,
		"condition_type": types.StringType,
	}
}

func AlertOverrideAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"priority": types.StringType,
	}
}

func LogsNewValueAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_filter":                 types.ObjectType{AttrTypes: LogsFilterAttr()},
		"rules":                       types.SetType{ElemType: types.ObjectType{AttrTypes: LogsNewValueRulesAttr()}},
		"notification_payload_filter": types.SetType{ElemType: types.StringType},
	}
}

func LogsNewValueRulesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{AttrTypes: LogsNewValueConditionAttr()},
	}
}

func LogsNewValueConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"time_window":      types.StringType,
		"keypath_to_track": types.StringType,
	}
}

func UndetectedValuesManagementAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"trigger_undetected_values": types.BoolType,
		"auto_retire_timeframe":     types.StringType,
	}
}

func LogsUniqueCountAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_filter":                       types.ObjectType{AttrTypes: LogsFilterAttr()},
		"notification_payload_filter":       types.SetType{ElemType: types.StringType},
		"rules":                             types.SetType{ElemType: types.ObjectType{AttrTypes: LogsUniqueCountRulesAttr()}},
		"unique_count_keypath":              types.StringType,
		"max_unique_count_per_group_by_key": types.Int64Type,
	}
}

func LogsUniqueCountRulesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{AttrTypes: LogsUniqueCountConditionAttr()},
	}
}

func LogsUniqueCountConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"max_unique_count": types.Int64Type,
		"time_window":      types.StringType,
	}
}

func AlertScheduleAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"active_on": types.ObjectType{
			AttrTypes: AlertScheduleActiveOnAttr(),
		},
	}
}

func AlertScheduleActiveOnAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"days_of_week": types.SetType{
			ElemType: types.StringType,
		},
		"start_time": types.StringType,
		"end_time":   types.StringType,
		"utc_offset": types.StringType,
	}
}

func LogsTimeRelativeAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_filter":                  types.ObjectType{AttrTypes: LogsFilterAttr()},
		"notification_payload_filter":  types.SetType{ElemType: types.StringType},
		"undetected_values_management": types.ObjectType{AttrTypes: UndetectedValuesManagementAttr()},
		"rules":                        types.SetType{ElemType: types.ObjectType{AttrTypes: LogsTimeRelativeRulesAttr()}},
		"custom_evaluation_delay":      types.Int32Type,
	}
}

func LogsTimeRelativeRulesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{
			AttrTypes: LogsTimeRelativeConditionAttr(),
		},
		"override": types.ObjectType{
			AttrTypes: AlertOverrideAttr(),
		},
	}
}

func LogsTimeRelativeConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"threshold":      types.Float64Type,
		"compared_to":    types.StringType,
		"condition_type": types.StringType,
	}
}

func MetricThresholdAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"metric_filter":                types.ObjectType{AttrTypes: MetricFilterAttr()},
		"undetected_values_management": types.ObjectType{AttrTypes: UndetectedValuesManagementAttr()},
		"rules":                        types.SetType{ElemType: types.ObjectType{AttrTypes: MetricThresholdRulesAttr()}},
		"missing_values":               types.ObjectType{AttrTypes: MissingValuesAttr()},
		"custom_evaluation_delay":      types.Int32Type,
	}
}

func MissingValuesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"replace_with_zero":       types.BoolType,
		"min_non_null_values_pct": types.Int64Type,
	}
}

func MetricThresholdRulesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{
			AttrTypes: MetricThresholdConditionAttr(),
		},
		"override": types.ObjectType{
			AttrTypes: AlertOverrideAttr(),
		},
	}
}

func MetricThresholdConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"threshold":      types.Float64Type,
		"for_over_pct":   types.Int64Type,
		"of_the_last":    types.StringType,
		"condition_type": types.StringType,
	}
}

func MetricFilterAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"promql": types.StringType,
	}
}

func MetricAnomalyAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"metric_filter":           types.ObjectType{AttrTypes: MetricFilterAttr()},
		"rules":                   types.SetType{ElemType: types.ObjectType{AttrTypes: MetricAnomalyRulesAttr()}},
		"custom_evaluation_delay": types.Int32Type,
	}
}

func MetricAnomalyRulesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{AttrTypes: MetricAnomalyConditionAttr()},
	}
}

func MetricAnomalyConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"min_non_null_values_pct": types.Int64Type,
		"threshold":               types.Float64Type,
		"for_over_pct":            types.Int64Type,
		"of_the_last":             types.StringType,
		"condition_type":          types.StringType,
	}
}

func TracingImmediateAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"tracing_filter":              types.ObjectType{AttrTypes: TracingQueryAttr()},
		"notification_payload_filter": types.SetType{ElemType: types.StringType},
	}
}

func TracingThresholdAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"tracing_filter":              types.ObjectType{AttrTypes: TracingQueryAttr()},
		"notification_payload_filter": types.SetType{ElemType: types.StringType},
		"rules":                       types.SetType{ElemType: types.ObjectType{AttrTypes: TracingThresholdRulesAttr()}},
	}
}

func TracingThresholdRulesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{AttrTypes: TracingThresholdConditionAttr()},
	}
}

func TracingThresholdConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"span_amount":    types.Float64Type,
		"time_window":    types.StringType,
		"condition_type": types.StringType,
	}
}

func FlowAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"stages": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: FlowStageAttr(),
			},
		},
		"enforce_suppression": types.BoolType,
	}
}

func FlowStageAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"flow_stages_groups": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: FlowStageGroupAttr(),
			},
		},
		"timeframe_ms":   types.Int64Type,
		"timeframe_type": types.StringType,
	}
}

func FlowStageGroupAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"alert_defs": types.SetType{
			ElemType: types.ObjectType{
				AttrTypes: AlertDefsAttr(),
			},
		},
		"next_op":   types.StringType,
		"alerts_op": types.StringType,
	}
}

func AlertDefsAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"id":  types.StringType,
		"not": types.BoolType,
	}
}

func SloThresholdAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"slo_definition": types.ObjectType{AttrTypes: SloDefinitionAttr()},
		"error_budget":   types.ObjectType{AttrTypes: SloErrorBudgetAttr()},
		"burn_rate":      types.ObjectType{AttrTypes: SloBurnRateAttr()},
	}
}

func SloDefinitionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"slo_id": types.StringType,
	}
}

func SloErrorBudgetAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"rules": types.ListType{
			ElemType: types.ObjectType{AttrTypes: SloThresholdRuleAttr()},
		},
	}
}

func SloBurnRateAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"rules": types.ListType{
			ElemType: types.ObjectType{AttrTypes: SloThresholdRuleAttr()},
		},
		"dual":   types.ObjectType{AttrTypes: SloDurationWrapperAttr()},
		"single": types.ObjectType{AttrTypes: SloDurationWrapperAttr()},
	}
}

func SloDurationWrapperAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"time_duration": types.ObjectType{AttrTypes: SloDurationAttr()},
	}
}

func SloDurationAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"duration": types.Int64Type,
		"unit":     types.StringType,
	}
}

func SloThresholdRuleAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{AttrTypes: SloThresholdConditionAttr()},
		"override":  types.ObjectType{AttrTypes: AlertOverrideAttr()},
	}
}

func SloThresholdConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"threshold": types.Float64Type,
	}
}

func TracingQueryAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"latency_threshold_ms":  types.NumberType,
		"tracing_label_filters": types.ObjectType{AttrTypes: TracingLabelFiltersAttr()},
	}
}

func LabelFilterTypesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"value":     types.StringType,
		"operation": types.StringType,
	}
}

func TracingLabelFiltersAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"application_name": types.SetType{ElemType: types.ObjectType{AttrTypes: TracingFiltersTypeAttr()}},
		"subsystem_name":   types.SetType{ElemType: types.ObjectType{AttrTypes: TracingFiltersTypeAttr()}},
		"service_name":     types.SetType{ElemType: types.ObjectType{AttrTypes: TracingFiltersTypeAttr()}},
		"operation_name":   types.SetType{ElemType: types.ObjectType{AttrTypes: TracingFiltersTypeAttr()}},
		"span_fields":      types.SetType{ElemType: types.ObjectType{AttrTypes: TracingSpanFieldsFilterAttr()}},
	}
}

func TracingFiltersTypeAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"operation": types.StringType,
		"values":    types.SetType{ElemType: types.StringType},
	}
}

func TracingSpanFieldsFilterAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"key":         types.StringType,
		"filter_type": types.ObjectType{AttrTypes: TracingFiltersTypeAttr()},
	}
}

func RetriggeringPeriodAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"minutes": types.Int64Type,
	}
}

func IncidentsSettingsAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"notify_on": types.StringType,
		"retriggering_period": types.ObjectType{
			AttrTypes: RetriggeringPeriodAttr(),
		},
	}
}
