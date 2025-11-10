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

package alerttypes

import (
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	alerts "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/alert_definitions_service"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	AlertPriorityProtoToSchemaMap = map[alerts.AlertDefPriority]string{
		alerts.ALERTDEFPRIORITY_ALERT_DEF_PRIORITY_P5_OR_UNSPECIFIED: "P5",
		alerts.ALERTDEFPRIORITY_ALERT_DEF_PRIORITY_P4:                "P4",
		alerts.ALERTDEFPRIORITY_ALERT_DEF_PRIORITY_P3:                "P3",
		alerts.ALERTDEFPRIORITY_ALERT_DEF_PRIORITY_P2:                "P2",
		alerts.ALERTDEFPRIORITY_ALERT_DEF_PRIORITY_P1:                "P1",
	}
	AlertPrioritySchemaToProtoMap = utils.ReverseMap(AlertPriorityProtoToSchemaMap)
	ValidAlertPriorities          = utils.GetKeys(AlertPrioritySchemaToProtoMap)

	NotifyOnProtoToSchemaMap = map[alerts.NotifyOn]string{
		alerts.NOTIFYON_NOTIFY_ON_TRIGGERED_ONLY_UNSPECIFIED: "Triggered Only",
		alerts.NOTIFYON_NOTIFY_ON_TRIGGERED_AND_RESOLVED:     "Triggered and Resolved",
	}
	NotifyOnSchemaToProtoMap = utils.ReverseMap(NotifyOnProtoToSchemaMap)
	ValidNotifyOn            = utils.GetKeys(NotifyOnSchemaToProtoMap)

	DaysOfWeekProtoToSchemaMap = map[alerts.DayOfWeek]string{
		alerts.DAYOFWEEK_DAY_OF_WEEK_MONDAY_OR_UNSPECIFIED: "Monday",
		alerts.DAYOFWEEK_DAY_OF_WEEK_TUESDAY:               "Tuesday",
		alerts.DAYOFWEEK_DAY_OF_WEEK_WEDNESDAY:             "Wednesday",
		alerts.DAYOFWEEK_DAY_OF_WEEK_THURSDAY:              "Thursday",
		alerts.DAYOFWEEK_DAY_OF_WEEK_FRIDAY:                "Friday",
		alerts.DAYOFWEEK_DAY_OF_WEEK_SATURDAY:              "Saturday",
		alerts.DAYOFWEEK_DAY_OF_WEEK_SUNDAY:                "Sunday",
	}
	DaysOfWeekSchemaToProtoMap = utils.ReverseMap(DaysOfWeekProtoToSchemaMap)
	ValidDaysOfWeek            = utils.GetKeys(DaysOfWeekSchemaToProtoMap)

	LogFilterOperationTypeProtoToSchemaMap = map[alerts.LogFilterOperationType]string{
		alerts.LOGFILTEROPERATIONTYPE_LOG_FILTER_OPERATION_TYPE_IS_OR_UNSPECIFIED: "IS",
		alerts.LOGFILTEROPERATIONTYPE_LOG_FILTER_OPERATION_TYPE_INCLUDES:          "INCLUDES",
		alerts.LOGFILTEROPERATIONTYPE_LOG_FILTER_OPERATION_TYPE_ENDS_WITH:         "ENDS_WITH",
		alerts.LOGFILTEROPERATIONTYPE_LOG_FILTER_OPERATION_TYPE_STARTS_WITH:       "STARTS_WITH",
	}
	LogFilterOperationTypeSchemaToProtoMap = utils.ReverseMap(LogFilterOperationTypeProtoToSchemaMap)
	ValidLogFilterOperationType            = utils.GetKeys(LogFilterOperationTypeSchemaToProtoMap)

	LogSeverityProtoToSchemaMap = map[alerts.LogSeverity]string{
		alerts.LOGSEVERITY_LOG_SEVERITY_VERBOSE_UNSPECIFIED: "Unspecified",
		alerts.LOGSEVERITY_LOG_SEVERITY_DEBUG:               "Debug",
		alerts.LOGSEVERITY_LOG_SEVERITY_INFO:                "Info",
		alerts.LOGSEVERITY_LOG_SEVERITY_WARNING:             "Warning",
		alerts.LOGSEVERITY_LOG_SEVERITY_ERROR:               "Error",
		alerts.LOGSEVERITY_LOG_SEVERITY_CRITICAL:            "Critical",
	}
	LogSeveritySchemaToProtoMap = utils.ReverseMap(LogSeverityProtoToSchemaMap)
	ValidLogSeverities          = utils.GetKeys(LogSeveritySchemaToProtoMap)

	LogsTimeWindowValueProtoToSchemaMap = map[alerts.LogsTimeWindowValue]string{
		alerts.LOGSTIMEWINDOWVALUE_LOGS_TIME_WINDOW_VALUE_MINUTES_5_OR_UNSPECIFIED: "5_MINUTES",
		alerts.LOGSTIMEWINDOWVALUE_LOGS_TIME_WINDOW_VALUE_MINUTES_10:               "10_MINUTES",
		alerts.LOGSTIMEWINDOWVALUE_LOGS_TIME_WINDOW_VALUE_MINUTES_15:               "15_MINUTES",
		alerts.LOGSTIMEWINDOWVALUE_LOGS_TIME_WINDOW_VALUE_MINUTES_20:               "20_MINUTES",
		alerts.LOGSTIMEWINDOWVALUE_LOGS_TIME_WINDOW_VALUE_MINUTES_30:               "30_MINUTES",
		alerts.LOGSTIMEWINDOWVALUE_LOGS_TIME_WINDOW_VALUE_HOUR_1:                   "1_HOUR",
		alerts.LOGSTIMEWINDOWVALUE_LOGS_TIME_WINDOW_VALUE_HOURS_2:                  "2_HOURS",
		alerts.LOGSTIMEWINDOWVALUE_LOGS_TIME_WINDOW_VALUE_HOURS_4:                  "4_HOURS",
		alerts.LOGSTIMEWINDOWVALUE_LOGS_TIME_WINDOW_VALUE_HOURS_6:                  "6_HOURS",
		alerts.LOGSTIMEWINDOWVALUE_LOGS_TIME_WINDOW_VALUE_HOURS_12:                 "12_HOURS",
		alerts.LOGSTIMEWINDOWVALUE_LOGS_TIME_WINDOW_VALUE_HOURS_24:                 "24_HOURS",
		alerts.LOGSTIMEWINDOWVALUE_LOGS_TIME_WINDOW_VALUE_HOURS_36:                 "36_HOURS",
	}
	LogsTimeWindowValueSchemaToProtoMap = utils.ReverseMap(LogsTimeWindowValueProtoToSchemaMap)
	ValidLogsTimeWindowValues           = utils.GetKeys(LogsTimeWindowValueSchemaToProtoMap)

	AutoRetireTimeframeProtoToSchemaMap = map[alerts.V3AutoRetireTimeframe]string{
		alerts.V3AUTORETIRETIMEFRAME_AUTO_RETIRE_TIMEFRAME_NEVER_OR_UNSPECIFIED: "NEVER",
		alerts.V3AUTORETIRETIMEFRAME_AUTO_RETIRE_TIMEFRAME_MINUTES_5:            "5_MINUTES",
		alerts.V3AUTORETIRETIMEFRAME_AUTO_RETIRE_TIMEFRAME_MINUTES_10:           "10_MINUTES",
		alerts.V3AUTORETIRETIMEFRAME_AUTO_RETIRE_TIMEFRAME_HOUR_1:               "1_HOUR",
		alerts.V3AUTORETIRETIMEFRAME_AUTO_RETIRE_TIMEFRAME_HOURS_2:              "2_HOURS",
		alerts.V3AUTORETIRETIMEFRAME_AUTO_RETIRE_TIMEFRAME_HOURS_6:              "6_HOURS",
		alerts.V3AUTORETIRETIMEFRAME_AUTO_RETIRE_TIMEFRAME_HOURS_12:             "12_HOURS",
		alerts.V3AUTORETIRETIMEFRAME_AUTO_RETIRE_TIMEFRAME_HOURS_24:             "24_HOURS",
	}
	AutoRetireTimeframeSchemaToProtoMap = utils.ReverseMap(AutoRetireTimeframeProtoToSchemaMap)
	ValidAutoRetireTimeframes           = utils.GetKeys(AutoRetireTimeframeSchemaToProtoMap)

	LogsRatioTimeWindowValueProtoToSchemaMap = map[alerts.LogsRatioTimeWindowValue]string{
		alerts.LOGSRATIOTIMEWINDOWVALUE_LOGS_RATIO_TIME_WINDOW_VALUE_MINUTES_5_OR_UNSPECIFIED: "5_MINUTES",
		alerts.LOGSRATIOTIMEWINDOWVALUE_LOGS_RATIO_TIME_WINDOW_VALUE_MINUTES_10:               "10_MINUTES",
		alerts.LOGSRATIOTIMEWINDOWVALUE_LOGS_RATIO_TIME_WINDOW_VALUE_MINUTES_15:               "15_MINUTES",
		alerts.LOGSRATIOTIMEWINDOWVALUE_LOGS_RATIO_TIME_WINDOW_VALUE_MINUTES_30:               "30_MINUTES",
		alerts.LOGSRATIOTIMEWINDOWVALUE_LOGS_RATIO_TIME_WINDOW_VALUE_HOUR_1:                   "1_HOUR",
		alerts.LOGSRATIOTIMEWINDOWVALUE_LOGS_RATIO_TIME_WINDOW_VALUE_HOURS_2:                  "2_HOURS",
		alerts.LOGSRATIOTIMEWINDOWVALUE_LOGS_RATIO_TIME_WINDOW_VALUE_HOURS_4:                  "4_HOURS",
		alerts.LOGSRATIOTIMEWINDOWVALUE_LOGS_RATIO_TIME_WINDOW_VALUE_HOURS_6:                  "6_HOURS",
		alerts.LOGSRATIOTIMEWINDOWVALUE_LOGS_RATIO_TIME_WINDOW_VALUE_HOURS_12:                 "12_HOURS",
		alerts.LOGSRATIOTIMEWINDOWVALUE_LOGS_RATIO_TIME_WINDOW_VALUE_HOURS_24:                 "24_HOURS",
		alerts.LOGSRATIOTIMEWINDOWVALUE_LOGS_RATIO_TIME_WINDOW_VALUE_HOURS_36:                 "36_HOURS",
	}
	LogsRatioTimeWindowValueSchemaToProtoMap = utils.ReverseMap(LogsRatioTimeWindowValueProtoToSchemaMap)
	ValidLogsRatioTimeWindowValues           = utils.GetKeys(LogsRatioTimeWindowValueSchemaToProtoMap)

	LogsRatioGroupByForProtoToSchemaMap = map[alerts.LogsRatioGroupByFor]string{
		alerts.LOGSRATIOGROUPBYFOR_LOGS_RATIO_GROUP_BY_FOR_BOTH_OR_UNSPECIFIED: "Both",
		alerts.LOGSRATIOGROUPBYFOR_LOGS_RATIO_GROUP_BY_FOR_NUMERATOR_ONLY:      "Numerator Only",
		alerts.LOGSRATIOGROUPBYFOR_LOGS_RATIO_GROUP_BY_FOR_DENUMERATOR_ONLY:    "Denominator Only",
	}
	LogsRatioGroupByForSchemaToProtoMap = utils.ReverseMap(LogsRatioGroupByForProtoToSchemaMap)
	ValidLogsRatioGroupByFor            = utils.GetKeys(LogsRatioGroupByForSchemaToProtoMap)

	LogsNewValueTimeWindowValueProtoToSchemaMap = map[alerts.LogsNewValueTimeWindowValue]string{
		alerts.LOGSNEWVALUETIMEWINDOWVALUE_LOGS_NEW_VALUE_TIME_WINDOW_VALUE_HOURS_12_OR_UNSPECIFIED: "12_HOURS",
		alerts.LOGSNEWVALUETIMEWINDOWVALUE_LOGS_NEW_VALUE_TIME_WINDOW_VALUE_HOURS_24:                "24_HOURS",
		alerts.LOGSNEWVALUETIMEWINDOWVALUE_LOGS_NEW_VALUE_TIME_WINDOW_VALUE_HOURS_48:                "48_HOURS",
		alerts.LOGSNEWVALUETIMEWINDOWVALUE_LOGS_NEW_VALUE_TIME_WINDOW_VALUE_HOURS_72:                "72_HOURS",
		alerts.LOGSNEWVALUETIMEWINDOWVALUE_LOGS_NEW_VALUE_TIME_WINDOW_VALUE_WEEK_1:                  "1_WEEK",
		alerts.LOGSNEWVALUETIMEWINDOWVALUE_LOGS_NEW_VALUE_TIME_WINDOW_VALUE_MONTH_1:                 "1_MONTH",
		alerts.LOGSNEWVALUETIMEWINDOWVALUE_LOGS_NEW_VALUE_TIME_WINDOW_VALUE_MONTHS_2:                "2_MONTHS",
		alerts.LOGSNEWVALUETIMEWINDOWVALUE_LOGS_NEW_VALUE_TIME_WINDOW_VALUE_MONTHS_3:                "3_MONTHS",
	}
	LogsNewValueTimeWindowValueSchemaToProtoMap = utils.ReverseMap(LogsNewValueTimeWindowValueProtoToSchemaMap)
	ValidLogsNewValueTimeWindowValues           = utils.GetKeys(LogsNewValueTimeWindowValueSchemaToProtoMap)

	LogsUniqueCountTimeWindowValueProtoToSchemaMap = map[alerts.LogsUniqueValueTimeWindowValue]string{
		alerts.LOGSUNIQUEVALUETIMEWINDOWVALUE_LOGS_UNIQUE_VALUE_TIME_WINDOW_VALUE_MINUTE_1_OR_UNSPECIFIED: "1_MINUTE",
		alerts.LOGSUNIQUEVALUETIMEWINDOWVALUE_LOGS_UNIQUE_VALUE_TIME_WINDOW_VALUE_MINUTES_5:               "5_MINUTES",
		alerts.LOGSUNIQUEVALUETIMEWINDOWVALUE_LOGS_UNIQUE_VALUE_TIME_WINDOW_VALUE_MINUTES_10:              "10_MINUTES",
		alerts.LOGSUNIQUEVALUETIMEWINDOWVALUE_LOGS_UNIQUE_VALUE_TIME_WINDOW_VALUE_MINUTES_15:              "15_MINUTES",
		alerts.LOGSUNIQUEVALUETIMEWINDOWVALUE_LOGS_UNIQUE_VALUE_TIME_WINDOW_VALUE_MINUTES_20:              "20_MINUTES",
		alerts.LOGSUNIQUEVALUETIMEWINDOWVALUE_LOGS_UNIQUE_VALUE_TIME_WINDOW_VALUE_MINUTES_30:              "30_MINUTES",
		alerts.LOGSUNIQUEVALUETIMEWINDOWVALUE_LOGS_UNIQUE_VALUE_TIME_WINDOW_VALUE_HOURS_1:                 "1_HOUR",
		alerts.LOGSUNIQUEVALUETIMEWINDOWVALUE_LOGS_UNIQUE_VALUE_TIME_WINDOW_VALUE_HOURS_2:                 "2_HOURS",
		alerts.LOGSUNIQUEVALUETIMEWINDOWVALUE_LOGS_UNIQUE_VALUE_TIME_WINDOW_VALUE_HOURS_4:                 "4_HOURS",
		alerts.LOGSUNIQUEVALUETIMEWINDOWVALUE_LOGS_UNIQUE_VALUE_TIME_WINDOW_VALUE_HOURS_6:                 "6_HOURS",
		alerts.LOGSUNIQUEVALUETIMEWINDOWVALUE_LOGS_UNIQUE_VALUE_TIME_WINDOW_VALUE_HOURS_12:                "12_HOURS",
		alerts.LOGSUNIQUEVALUETIMEWINDOWVALUE_LOGS_UNIQUE_VALUE_TIME_WINDOW_VALUE_HOURS_24:                "24_HOURS",
		alerts.LOGSUNIQUEVALUETIMEWINDOWVALUE_LOGS_UNIQUE_VALUE_TIME_WINDOW_VALUE_HOURS_36:                "36_HOURS",
	}
	LogsUniqueCountTimeWindowValueSchemaToProtoMap = utils.ReverseMap(LogsUniqueCountTimeWindowValueProtoToSchemaMap)
	ValidLogsUniqueCountTimeWindowValues           = utils.GetKeys(LogsUniqueCountTimeWindowValueSchemaToProtoMap)

	LogsTimeRelativeComparedToProtoToSchemaMap = map[alerts.LogsTimeRelativeComparedTo]string{
		alerts.LOGSTIMERELATIVECOMPAREDTO_LOGS_TIME_RELATIVE_COMPARED_TO_PREVIOUS_HOUR_OR_UNSPECIFIED: "Previous Hour",
		alerts.LOGSTIMERELATIVECOMPAREDTO_LOGS_TIME_RELATIVE_COMPARED_TO_SAME_HOUR_YESTERDAY:          "Same Hour Yesterday",
		alerts.LOGSTIMERELATIVECOMPAREDTO_LOGS_TIME_RELATIVE_COMPARED_TO_SAME_HOUR_LAST_WEEK:          "Same Hour Last Week",
		alerts.LOGSTIMERELATIVECOMPAREDTO_LOGS_TIME_RELATIVE_COMPARED_TO_YESTERDAY:                    "Yesterday",
		alerts.LOGSTIMERELATIVECOMPAREDTO_LOGS_TIME_RELATIVE_COMPARED_TO_SAME_DAY_LAST_WEEK:           "Same Day Last Week",
		alerts.LOGSTIMERELATIVECOMPAREDTO_LOGS_TIME_RELATIVE_COMPARED_TO_SAME_DAY_LAST_MONTH:          "Same Day Last Month",
	}
	LogsTimeRelativeComparedToSchemaToProtoMap = utils.ReverseMap(LogsTimeRelativeComparedToProtoToSchemaMap)
	ValidLogsTimeRelativeComparedTo            = utils.GetKeys(LogsTimeRelativeComparedToSchemaToProtoMap)

	MetricFilterOperationTypeProtoToSchemaMap = map[alerts.MetricTimeWindowValue]string{
		alerts.METRICTIMEWINDOWVALUE_METRIC_TIME_WINDOW_VALUE_MINUTES_1_OR_UNSPECIFIED: "1_MINUTE",
		alerts.METRICTIMEWINDOWVALUE_METRIC_TIME_WINDOW_VALUE_MINUTES_5:                "5_MINUTES",
		alerts.METRICTIMEWINDOWVALUE_METRIC_TIME_WINDOW_VALUE_MINUTES_10:               "10_MINUTES",
		alerts.METRICTIMEWINDOWVALUE_METRIC_TIME_WINDOW_VALUE_MINUTES_15:               "15_MINUTES",
		alerts.METRICTIMEWINDOWVALUE_METRIC_TIME_WINDOW_VALUE_MINUTES_20:               "20_MINUTES",
		alerts.METRICTIMEWINDOWVALUE_METRIC_TIME_WINDOW_VALUE_MINUTES_30:               "30_MINUTES",
		alerts.METRICTIMEWINDOWVALUE_METRIC_TIME_WINDOW_VALUE_HOUR_1:                   "1_HOUR",
		alerts.METRICTIMEWINDOWVALUE_METRIC_TIME_WINDOW_VALUE_HOURS_2:                  "2_HOURS",
		alerts.METRICTIMEWINDOWVALUE_METRIC_TIME_WINDOW_VALUE_HOURS_4:                  "4_HOURS",
		alerts.METRICTIMEWINDOWVALUE_METRIC_TIME_WINDOW_VALUE_HOURS_6:                  "6_HOURS",
		alerts.METRICTIMEWINDOWVALUE_METRIC_TIME_WINDOW_VALUE_HOURS_12:                 "12_HOURS",
		alerts.METRICTIMEWINDOWVALUE_METRIC_TIME_WINDOW_VALUE_HOURS_24:                 "24_HOURS",
		alerts.METRICTIMEWINDOWVALUE_METRIC_TIME_WINDOW_VALUE_HOURS_36:                 "36_HOURS",
	}
	MetricTimeWindowValueSchemaToProtoMap = utils.ReverseMap(MetricFilterOperationTypeProtoToSchemaMap)
	ValidMetricTimeWindowValues           = utils.GetKeys(MetricTimeWindowValueSchemaToProtoMap)

	TracingTimeWindowProtoToSchemaMap = map[alerts.TracingTimeWindowValue]string{
		alerts.TRACINGTIMEWINDOWVALUE_TRACING_TIME_WINDOW_VALUE_MINUTES_5_OR_UNSPECIFIED: "5_MINUTES",
		alerts.TRACINGTIMEWINDOWVALUE_TRACING_TIME_WINDOW_VALUE_MINUTES_10:               "10_MINUTES",
		alerts.TRACINGTIMEWINDOWVALUE_TRACING_TIME_WINDOW_VALUE_MINUTES_15:               "15_MINUTES",
		alerts.TRACINGTIMEWINDOWVALUE_TRACING_TIME_WINDOW_VALUE_MINUTES_20:               "20_MINUTES",
		alerts.TRACINGTIMEWINDOWVALUE_TRACING_TIME_WINDOW_VALUE_MINUTES_30:               "30_MINUTES",
		alerts.TRACINGTIMEWINDOWVALUE_TRACING_TIME_WINDOW_VALUE_HOUR_1:                   "1_HOUR",
		alerts.TRACINGTIMEWINDOWVALUE_TRACING_TIME_WINDOW_VALUE_HOURS_2:                  "2_HOURS",
		alerts.TRACINGTIMEWINDOWVALUE_TRACING_TIME_WINDOW_VALUE_HOURS_4:                  "4_HOURS",
		alerts.TRACINGTIMEWINDOWVALUE_TRACING_TIME_WINDOW_VALUE_HOURS_6:                  "6_HOURS",
		alerts.TRACINGTIMEWINDOWVALUE_TRACING_TIME_WINDOW_VALUE_HOURS_12:                 "12_HOURS",
		alerts.TRACINGTIMEWINDOWVALUE_TRACING_TIME_WINDOW_VALUE_HOURS_24:                 "24_HOURS",
		alerts.TRACINGTIMEWINDOWVALUE_TRACING_TIME_WINDOW_VALUE_HOURS_36:                 "36_HOURS",
	}
	TracingTimeWindowSchemaToProtoMap = utils.ReverseMap(TracingTimeWindowProtoToSchemaMap)
	ValidTracingTimeWindow            = utils.GetKeys(TracingTimeWindowSchemaToProtoMap)

	TracingFilterOperationProtoToSchemaMap = map[alerts.TracingFilterOperationType]string{
		alerts.TRACINGFILTEROPERATIONTYPE_TRACING_FILTER_OPERATION_TYPE_IS_OR_UNSPECIFIED: "IS",
		alerts.TRACINGFILTEROPERATIONTYPE_TRACING_FILTER_OPERATION_TYPE_IS_NOT:            "IS_NOT",
		alerts.TRACINGFILTEROPERATIONTYPE_TRACING_FILTER_OPERATION_TYPE_INCLUDES:          "INCLUDES",
		alerts.TRACINGFILTEROPERATIONTYPE_TRACING_FILTER_OPERATION_TYPE_ENDS_WITH:         "ENDS_WITH",
		alerts.TRACINGFILTEROPERATIONTYPE_TRACING_FILTER_OPERATION_TYPE_STARTS_WITH:       "STARTS_WITH",
	}
	TracingFilterOperationSchemaToProtoMap = utils.ReverseMap(TracingFilterOperationProtoToSchemaMap)
	ValidTracingFilterOperations           = utils.GetKeys(TracingFilterOperationSchemaToProtoMap)
	FlowStageTimeFrameTypeProtoToSchemaMap = map[alerts.TimeframeType]string{
		alerts.TIMEFRAMETYPE_TIMEFRAME_TYPE_UNSPECIFIED: "Unspecified",
		alerts.TIMEFRAMETYPE_TIMEFRAME_TYPE_UP_TO:       "Up To",
	}
	FlowStageTimeFrameTypeSchemaToProtoMap = utils.ReverseMap(FlowStageTimeFrameTypeProtoToSchemaMap)
	ValidFlowStageTimeFrameTypes           = utils.GetKeys(FlowStageTimeFrameTypeSchemaToProtoMap)

	FlowStagesGroupNextOpProtoToSchemaMap = map[alerts.NextOp]string{
		alerts.NEXTOP_NEXT_OP_AND_OR_UNSPECIFIED: "AND",
		alerts.NEXTOP_NEXT_OP_OR:                 "OR",
	}
	FlowStagesGroupNextOpSchemaToProtoMap = utils.ReverseMap(FlowStagesGroupNextOpProtoToSchemaMap)
	ValidFlowStagesGroupNextOps           = utils.GetKeys(FlowStagesGroupNextOpSchemaToProtoMap)

	FlowStagesGroupAlertsOpProtoToSchemaMap = map[alerts.AlertsOp]string{
		alerts.ALERTSOP_ALERTS_OP_AND_OR_UNSPECIFIED: "AND",
		alerts.ALERTSOP_ALERTS_OP_OR:                 "OR",
	}
	FlowStagesGroupAlertsOpSchemaToProtoMap = utils.ReverseMap(FlowStagesGroupAlertsOpProtoToSchemaMap)
	ValidFlowStagesGroupAlertsOps           = utils.GetKeys(FlowStagesGroupAlertsOpSchemaToProtoMap)

	LogsThresholdConditionMap = map[alerts.LogsThresholdConditionType]string{
		alerts.LOGSTHRESHOLDCONDITIONTYPE_LOGS_THRESHOLD_CONDITION_TYPE_MORE_THAN_OR_UNSPECIFIED: "MORE_THAN",
		alerts.LOGSTHRESHOLDCONDITIONTYPE_LOGS_THRESHOLD_CONDITION_TYPE_LESS_THAN:                "LESS_THAN",
	}
	LogsThresholdConditionToProtoMap = utils.ReverseMap(LogsThresholdConditionMap)
	LogsThresholdConditionValues     = utils.GetValues(LogsThresholdConditionMap)

	LogsTimeRelativeConditionMap = map[alerts.LogsTimeRelativeConditionType]string{
		alerts.LOGSTIMERELATIVECONDITIONTYPE_LOGS_TIME_RELATIVE_CONDITION_TYPE_MORE_THAN_OR_UNSPECIFIED: "MORE_THAN",
		alerts.LOGSTIMERELATIVECONDITIONTYPE_LOGS_TIME_RELATIVE_CONDITION_TYPE_LESS_THAN:                "LESS_THAN",
	}
	LogsTimeRelativeConditionToProtoMap = utils.ReverseMap(LogsTimeRelativeConditionMap)
	LogsTimeRelativeConditionValues     = utils.GetValues(LogsTimeRelativeConditionMap)

	LogsRatioConditionMap = map[alerts.LogsRatioConditionType]string{
		alerts.LOGSRATIOCONDITIONTYPE_LOGS_RATIO_CONDITION_TYPE_MORE_THAN_OR_UNSPECIFIED: "MORE_THAN",
		alerts.LOGSRATIOCONDITIONTYPE_LOGS_RATIO_CONDITION_TYPE_LESS_THAN:                "LESS_THAN",
	}
	LogsRatioConditionMapValues        = utils.GetValues(LogsRatioConditionMap)
	LogsRatioConditionSchemaToProtoMap = utils.ReverseMap(LogsRatioConditionMap)

	MetricsThresholdConditionMap = map[alerts.MetricThresholdConditionType]string{
		alerts.METRICTHRESHOLDCONDITIONTYPE_METRIC_THRESHOLD_CONDITION_TYPE_MORE_THAN_OR_UNSPECIFIED: "MORE_THAN",
		alerts.METRICTHRESHOLDCONDITIONTYPE_METRIC_THRESHOLD_CONDITION_TYPE_LESS_THAN:                "LESS_THAN",
		alerts.METRICTHRESHOLDCONDITIONTYPE_METRIC_THRESHOLD_CONDITION_TYPE_MORE_THAN_OR_EQUALS:      "MORE_THAN_OR_EQUALS",
		alerts.METRICTHRESHOLDCONDITIONTYPE_METRIC_THRESHOLD_CONDITION_TYPE_LESS_THAN_OR_EQUALS:      "LESS_THAN_OR_EQUALS",
	}
	MetricsThresholdConditionValues     = utils.GetValues(MetricsThresholdConditionMap)
	MetricsThresholdConditionToProtoMap = utils.ReverseMap(MetricsThresholdConditionMap)

	MetricAnomalyConditionMap = map[alerts.MetricAnomalyConditionType]string{
		alerts.METRICANOMALYCONDITIONTYPE_METRIC_ANOMALY_CONDITION_TYPE_MORE_THAN_USUAL_OR_UNSPECIFIED: "MORE_THAN",
		alerts.METRICANOMALYCONDITIONTYPE_METRIC_ANOMALY_CONDITION_TYPE_LESS_THAN_USUAL:                "LESS_THAN",
	}
	MetricAnomalyConditionValues     = utils.GetValues(MetricAnomalyConditionMap)
	MetricAnomalyConditionToProtoMap = utils.ReverseMap(MetricAnomalyConditionMap)
	LogsAnomalyConditionMap          = map[alerts.LogsAnomalyConditionType]string{
		alerts.LOGSANOMALYCONDITIONTYPE_LOGS_ANOMALY_CONDITION_TYPE_MORE_THAN_USUAL_OR_UNSPECIFIED: "MORE_THAN_USUAL",
	}
	LogsAnomalyConditionSchemaToProtoMap = utils.ReverseMap(LogsAnomalyConditionMap)
	// LogsAnomalyConditionValues           = utils.GetValues(LogsAnomalyConditionMap)

	DurationUnitProtoToSchemaMap = map[alerts.DurationUnit]string{
		alerts.DURATIONUNIT_DURATION_UNIT_UNSPECIFIED: "UNSPECIFIED",
		alerts.DURATIONUNIT_DURATION_UNIT_HOURS:       "HOURS",
	}
	DurationUnitSchemaToProtoMap = utils.ReverseMap(DurationUnitProtoToSchemaMap)
	ValidDurationUnits           = utils.GetKeys(DurationUnitSchemaToProtoMap)
)

type AlertResourceModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Description       types.String `tfsdk:"description"`
	Enabled           types.Bool   `tfsdk:"enabled"`
	Priority          types.String `tfsdk:"priority"`
	Schedule          types.Object `tfsdk:"schedule"`        // AlertScheduleModel
	TypeDefinition    types.Object `tfsdk:"type_definition"` // AlertTypeDefinitionModel
	PhantomMode       types.Bool   `tfsdk:"phantom_mode"`
	Deleted           types.Bool   `tfsdk:"deleted"`
	GroupBy           types.List   `tfsdk:"group_by"`           // []types.String
	IncidentsSettings types.Object `tfsdk:"incidents_settings"` // IncidentsSettingsModel
	NotificationGroup types.Object `tfsdk:"notification_group"` // NotificationGroupModel
	Labels            types.Map    `tfsdk:"labels"`             // map[string]string
}

type AlertScheduleModel struct {
	ActiveOn types.Object `tfsdk:"active_on"` // ActiveOnModel
}

type AlertTypeDefinitionModel struct {
	LogsImmediate             types.Object `tfsdk:"logs_immediate"`               // LogsImmediateModel
	LogsThreshold             types.Object `tfsdk:"logs_threshold"`               // LogsThresholdModel
	LogsAnomaly               types.Object `tfsdk:"logs_anomaly"`                 // LogsAnomalyModel
	LogsRatioThreshold        types.Object `tfsdk:"logs_ratio_threshold"`         // LogsRatioThresholdModel
	LogsNewValue              types.Object `tfsdk:"logs_new_value"`               // LogsNewValueModel
	LogsUniqueCount           types.Object `tfsdk:"logs_unique_count"`            // LogsUniqueCountModel
	LogsTimeRelativeThreshold types.Object `tfsdk:"logs_time_relative_threshold"` // LogsTimeRelativeThresholdModel
	MetricThreshold           types.Object `tfsdk:"metric_threshold"`             // MetricThresholdModel
	MetricAnomaly             types.Object `tfsdk:"metric_anomaly"`               // MetricAnomalyModel
	TracingImmediate          types.Object `tfsdk:"tracing_immediate"`            // TracingImmediateModel
	TracingThreshold          types.Object `tfsdk:"tracing_threshold"`            // TracingThresholdModel
	Flow                      types.Object `tfsdk:"flow"`                         // FlowModel
	SloThreshold              types.Object `tfsdk:"slo_threshold"`                // SloThresholdModel
}

type IncidentsSettingsModel struct {
	NotifyOn           types.String `tfsdk:"notify_on"`
	RetriggeringPeriod types.Object `tfsdk:"retriggering_period"` // RetriggeringPeriodModel
}

type NotificationGroupModel struct {
	Destinations     types.List   `tfsdk:"destinations"`      // NotificationDestinationModel
	Router           types.Object `tfsdk:"router"`            // NotificationRouterModel
	GroupByKeys      types.List   `tfsdk:"group_by_keys"`     // []types.String
	WebhooksSettings types.Set    `tfsdk:"webhooks_settings"` // WebhooksSettingsModel
}

type NotificationRouterModel struct {
	NotifyOn types.String `tfsdk:"notify_on"`
}

type NotificationDestinationModel struct {
	ConnectorId               types.String `tfsdk:"connector_id"`
	PresetId                  types.String `tfsdk:"preset_id"`
	NotifyOn                  types.String `tfsdk:"notify_on"`
	TriggeredRoutingOverrides types.Object `tfsdk:"triggered_routing_overrides"` // SourceOverridesModel
	ResolvedRoutingOverrides  types.Object `tfsdk:"resolved_routing_overrides"`  // SourceOverridesModel
}

type SourceOverridesModel struct {
	ConnectorOverrides types.List   `tfsdk:"connector_overrides"` // []ConfigurationOverrideModel
	PresetOverrides    types.List   `tfsdk:"preset_overrides"`    // []ConfigurationOverrideModel
	PayloadType        types.String `tfsdk:"payload_type"`
}

type ConfigurationOverrideModel struct {
	FieldName types.String `tfsdk:"field_name"`
	Template  types.String `tfsdk:"template"`
}

type NotificationRouter struct {
	NotifyOn types.String `tfsdk:"notify_on"`
}

type WebhooksSettingsModel struct {
	RetriggeringPeriod types.Object `tfsdk:"retriggering_period"` // RetriggeringPeriodModel
	NotifyOn           types.String `tfsdk:"notify_on"`
	IntegrationID      types.String `tfsdk:"integration_id"`
	Recipients         types.Set    `tfsdk:"recipients"` //[]types.String
}

type ActiveOnModel struct {
	DaysOfWeek types.Set    `tfsdk:"days_of_week"` // []types.String
	StartTime  types.String `tfsdk:"start_time"`
	EndTime    types.String `tfsdk:"end_time"`
	UtcOffset  types.String `tfsdk:"utc_offset"`
}

type RetriggeringPeriodModel struct {
	Minutes types.Int64 `tfsdk:"minutes"`
}

// Alert Types:
type LogsImmediateModel struct {
	LogsFilter                types.Object `tfsdk:"logs_filter"`                 // AlertsLogsFilterModel
	NotificationPayloadFilter types.Set    `tfsdk:"notification_payload_filter"` // []types.String
}

type LogsThresholdModel struct {
	Rules                      types.Set    `tfsdk:"rules"`                        // [] LogsThresholdRuleModel
	LogsFilter                 types.Object `tfsdk:"logs_filter"`                  // AlertsLogsFilterModel
	NotificationPayloadFilter  types.Set    `tfsdk:"notification_payload_filter"`  // []types.String
	UndetectedValuesManagement types.Object `tfsdk:"undetected_values_management"` // UndetectedValuesManagementModel
	CustomEvaluationDelay      types.Int32  `tfsdk:"custom_evaluation_delay"`
}

type LogsAnomalyModel struct {
	Rules                     types.Set    `tfsdk:"rules"`                       // [] LogsAnomalyRuleModel
	LogsFilter                types.Object `tfsdk:"logs_filter"`                 // AlertsLogsFilterModel
	NotificationPayloadFilter types.Set    `tfsdk:"notification_payload_filter"` // []types.String
	CustomEvaluationDelay     types.Int32  `tfsdk:"custom_evaluation_delay"`
}

type LogsRatioThresholdModel struct {
	Rules                     types.Set    `tfsdk:"rules"`     // []LogsRatioThresholdRuleModel
	Numerator                 types.Object `tfsdk:"numerator"` // AlertsLogsFilterModel
	NumeratorAlias            types.String `tfsdk:"numerator_alias"`
	Denominator               types.Object `tfsdk:"denominator"` // AlertsLogsFilterModel
	DenominatorAlias          types.String `tfsdk:"denominator_alias"`
	NotificationPayloadFilter types.Set    `tfsdk:"notification_payload_filter"` // []types.String
	GroupByFor                types.String `tfsdk:"group_by_for"`
	CustomEvaluationDelay     types.Int32  `tfsdk:"custom_evaluation_delay"`
}

type LogsNewValueModel struct {
	Rules                     types.Set    `tfsdk:"rules"`                       // []NewValueRuleModel
	LogsFilter                types.Object `tfsdk:"logs_filter"`                 // AlertsLogsFilterModel
	NotificationPayloadFilter types.Set    `tfsdk:"notification_payload_filter"` // []types.String
}

type LogsUniqueCountModel struct {
	Rules                       types.Set    `tfsdk:"rules"`                       // [] LogsUniqueCountRuleModel
	LogsFilter                  types.Object `tfsdk:"logs_filter"`                 // AlertsLogsFilterModel
	NotificationPayloadFilter   types.Set    `tfsdk:"notification_payload_filter"` // []types.String
	MaxUniqueCountPerGroupByKey types.Int64  `tfsdk:"max_unique_count_per_group_by_key"`
	UniqueCountKeypath          types.String `tfsdk:"unique_count_keypath"`
}

type LogsUniqueCountRuleModel struct {
	Condition types.Object `tfsdk:"condition"` // LogsUniqueCountConditionModel
}

type LogsUniqueCountConditionModel struct {
	MaxUniqueCount types.Int64  `tfsdk:"max_unique_count"`
	TimeWindow     types.String `tfsdk:"time_window"`
}

type LogsTimeRelativeThresholdModel struct {
	Rules                      types.Set    `tfsdk:"rules"`                        // [] LogsTimeRelativeRuleModel
	LogsFilter                 types.Object `tfsdk:"logs_filter"`                  // AlertsLogsFilterModel
	NotificationPayloadFilter  types.Set    `tfsdk:"notification_payload_filter"`  // []types.String
	UndetectedValuesManagement types.Object `tfsdk:"undetected_values_management"` // UndetectedValuesManagementModel
	CustomEvaluationDelay      types.Int32  `tfsdk:"custom_evaluation_delay"`
}

type MetricAnomalyRuleModel struct {
	Condition types.Object `tfsdk:"condition"` // MetricAnomalyConditionModel
}

type MetricAnomalyConditionModel struct {
	MinNonNullValuesPct types.Int64   `tfsdk:"min_non_null_values_pct"`
	Threshold           types.Float64 `tfsdk:"threshold"`
	ForOverPct          types.Int64   `tfsdk:"for_over_pct"`
	OfTheLast           types.String  `tfsdk:"of_the_last"`
	ConditionType       types.String  `tfsdk:"condition_type"`
}

type MetricThresholdModel struct {
	Rules                      types.Set    `tfsdk:"rules"`                        // [] MetricThresholdRuleModel
	MetricFilter               types.Object `tfsdk:"metric_filter"`                // MetricFilterModel
	MissingValues              types.Object `tfsdk:"missing_values"`               // MissingValuesModel
	UndetectedValuesManagement types.Object `tfsdk:"undetected_values_management"` // UndetectedValuesManagementModel
	CustomEvaluationDelay      types.Int32  `tfsdk:"custom_evaluation_delay"`
}

type MissingValuesModel struct {
	ReplaceWithZero     types.Bool  `tfsdk:"replace_with_zero"`
	MinNonNullValuesPct types.Int64 `tfsdk:"min_non_null_values_pct"`
}

type MetricThresholdRuleModel struct {
	Condition types.Object `tfsdk:"condition"` // MetricThresholdConditionModel
	Override  types.Object `tfsdk:"override"`  // AlertOverrideModel
}

type MetricThresholdConditionModel struct {
	Threshold     types.Float64 `tfsdk:"threshold"`
	ForOverPct    types.Int64   `tfsdk:"for_over_pct"`
	OfTheLast     types.String  `tfsdk:"of_the_last"`
	ConditionType types.String  `tfsdk:"condition_type"`
}

type MetricAnomalyModel struct {
	MetricFilter          types.Object `tfsdk:"metric_filter"` // MetricFilterModel
	Rules                 types.Set    `tfsdk:"rules"`         // [] MetricAnomalyRuleModel
	CustomEvaluationDelay types.Int32  `tfsdk:"custom_evaluation_delay"`
}

type MetricImmediateModel struct {
	MetricFilter              types.Object `tfsdk:"metric_filter"`               // TracingFilterModel
	NotificationPayloadFilter types.Set    `tfsdk:"notification_payload_filter"` // []types.String
}

type TracingImmediateModel struct {
	TracingFilter             types.Object `tfsdk:"tracing_filter"`              // TracingFilterModel
	NotificationPayloadFilter types.Set    `tfsdk:"notification_payload_filter"` // []types.String
}

type TracingThresholdModel struct {
	TracingFilter             types.Object `tfsdk:"tracing_filter"`              // TracingFilterModel
	NotificationPayloadFilter types.Set    `tfsdk:"notification_payload_filter"` // []types.String
	Rules                     types.Set    `tfsdk:"rules"`                       // [] TracingThresholdRuleModel
}

type TracingThresholdRuleModel struct {
	Condition types.Object `tfsdk:"condition"` // TracingThresholdConditionModel
}

type TracingThresholdConditionModel struct {
	TimeWindow    types.String  `tfsdk:"time_window"`
	SpanAmount    types.Float64 `tfsdk:"span_amount"`
	ConditionType types.String  `tfsdk:"condition_type"`
}

type FlowModel struct {
	Stages             types.List `tfsdk:"stages"` // FlowStageModel
	EnforceSuppression types.Bool `tfsdk:"enforce_suppression"`
}

type FlowStageModel struct {
	FlowStagesGroups types.List   `tfsdk:"flow_stages_groups"` // FlowStagesGroupModel
	TimeframeMs      types.Int64  `tfsdk:"timeframe_ms"`
	TimeframeType    types.String `tfsdk:"timeframe_type"`
}

type FlowStagesGroupModel struct {
	AlertDefs types.Set    `tfsdk:"alert_defs"` // FlowStagesGroupsAlertDefsModel
	NextOp    types.String `tfsdk:"next_op"`
	AlertsOp  types.String `tfsdk:"alerts_op"`
}

type FlowStagesGroupsAlertDefsModel struct {
	Id  types.String `tfsdk:"id"`
	Not types.Bool   `tfsdk:"not"`
}

type AlertsLogsFilterModel struct {
	SimpleFilter types.Object `tfsdk:"simple_filter"` // SimpleFilterModel
}

type SimpleFilterModel struct {
	LuceneQuery  types.String `tfsdk:"lucene_query"`
	LabelFilters types.Object `tfsdk:"label_filters"` // LabelFiltersModel
}

type LabelFiltersModel struct {
	ApplicationName types.Set `tfsdk:"application_name"` // LabelFilterTypeModel
	SubsystemName   types.Set `tfsdk:"subsystem_name"`   // LabelFilterTypeModel
	Severities      types.Set `tfsdk:"severities"`       // []types.String
}

type LabelFilterTypeModel struct {
	Value     types.String `tfsdk:"value"`
	Operation types.String `tfsdk:"operation"`
}

type NotificationPayloadFilterModel struct {
	Filter types.String `tfsdk:"filter"`
}

type UndetectedValuesManagementModel struct {
	TriggerUndetectedValues types.Bool   `tfsdk:"trigger_undetected_values"`
	AutoRetireTimeframe     types.String `tfsdk:"auto_retire_timeframe"`
}

type MetricFilterModel struct {
	Promql types.String `tfsdk:"promql"`
}

type NewValueRuleModel struct {
	Condition types.Object `tfsdk:"condition"` // NewValueConditionModel
}

type NewValueConditionModel struct {
	TimeWindow     types.String `tfsdk:"time_window"`
	KeypathToTrack types.String `tfsdk:"keypath_to_track"`
}

type LogsTimeRelativeRuleModel struct {
	Condition types.Object `tfsdk:"condition"` // LogsTimeRelativeConditionModel
	Override  types.Object `tfsdk:"override"`  // AlertOverrideModel
}

type LogsTimeRelativeConditionModel struct {
	Threshold     types.Float64 `tfsdk:"threshold"`
	ComparedTo    types.String  `tfsdk:"compared_to"`
	ConditionType types.String  `tfsdk:"condition_type"`
}

type LogsRatioThresholdRuleModel struct {
	Condition types.Object `tfsdk:"condition"` // LogsRatioConditionModel
	Override  types.Object `tfsdk:"override"`  // AlertOverrideModel
}

type AlertOverrideModel struct {
	Priority types.String `tfsdk:"priority"`
}

type LogsRatioConditionModel struct {
	Threshold     types.Float64 `tfsdk:"threshold"`
	TimeWindow    types.String  `tfsdk:"time_window"`
	ConditionType types.String  `tfsdk:"condition_type"`
}

type LogsAnomalyRuleModel struct {
	Condition types.Object `tfsdk:"condition"` // LogsAnomalyConditionModel
}

type LogsAnomalyConditionModel struct {
	MinimumThreshold types.Float64 `tfsdk:"minimum_threshold"`
	TimeWindow       types.String  `tfsdk:"time_window"`
	ConditionType    types.String  `tfsdk:"condition_type"`
}

type LogsThresholdRuleModel struct {
	Condition types.Object `tfsdk:"condition"` // LogsThresholdConditionModel
	Override  types.Object `tfsdk:"override"`  // AlertOverrideModel
}

type LogsThresholdConditionModel struct {
	Threshold     types.Float64 `tfsdk:"threshold"`
	TimeWindow    types.String  `tfsdk:"time_window"`
	ConditionType types.String  `tfsdk:"condition_type"`
}

type TracingFilterModel struct {
	LatencyThresholdMs  types.Number `tfsdk:"latency_threshold_ms"`
	TracingLabelFilters types.Object `tfsdk:"tracing_label_filters"` // TracingLabelFiltersModel
}

type TracingLabelFiltersModel struct {
	ApplicationName types.Set `tfsdk:"application_name"` // TracingFilterTypeModel
	SubsystemName   types.Set `tfsdk:"subsystem_name"`   // TracingFilterTypeModel
	ServiceName     types.Set `tfsdk:"service_name"`     // TracingFilterTypeModel
	OperationName   types.Set `tfsdk:"operation_name"`   // TracingFilterTypeModel
	SpanFields      types.Set `tfsdk:"span_fields"`      // TracingSpanFieldsFilterModel
}

type TracingFilterTypeModel struct {
	Values    types.Set    `tfsdk:"values"` // []types.String
	Operation types.String `tfsdk:"operation"`
}

type TracingSpanFieldsFilterModel struct {
	Key        types.String `tfsdk:"key"`
	FilterType types.Object `tfsdk:"filter_type"` // TracingFilterTypeModel
}

type SloThresholdModel struct {
	SloDefinition types.Object `tfsdk:"slo_definition"` // SloDefinitionObject
	ErrorBudget   types.Object `tfsdk:"error_budget"`   // SloThresholdErrorBudgetModel
	BurnRate      types.Object `tfsdk:"burn_rate"`      // SloThresholdBurnRateModel
}

type SloDefinitionObject struct {
	SloId types.String `tfsdk:"slo_id"`
}

type SloThresholdErrorBudgetModel struct {
	Rules types.List `tfsdk:"rules"` // []SloThresholdRuleModel
}

type SloThresholdBurnRateModel struct {
	Rules  types.List   `tfsdk:"rules"`  // []SloThresholdRuleModel
	Dual   types.Object `tfsdk:"dual"`   // SloThresholdDurationWrapperModel
	Single types.Object `tfsdk:"single"` // SloThresholdDurationWrapperModel
}

type SloThresholdRuleModel struct {
	Condition types.Object `tfsdk:"condition"` // SloThresholdConditionModel
	Override  types.Object `tfsdk:"override"`  // AlertOverrideModel
}

type SloThresholdConditionModel struct {
	Threshold types.Float64 `tfsdk:"threshold"`
}

type SloThresholdDurationWrapperModel struct {
	TimeDuration types.Object `tfsdk:"time_duration"` // SloDurationModel
}

type SloDurationModel struct {
	Duration types.Int64  `tfsdk:"duration"`
	Unit     types.String `tfsdk:"unit"`
}
