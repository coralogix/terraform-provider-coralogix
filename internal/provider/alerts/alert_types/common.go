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

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	AlertPriorityProtoToSchemaMap = map[cxsdk.AlertDefPriority]string{
		cxsdk.AlertDefPriorityP5OrUnspecified: "P5",
		cxsdk.AlertDefPriorityP4:              "P4",
		cxsdk.AlertDefPriorityP3:              "P3",
		cxsdk.AlertDefPriorityP2:              "P2",
		cxsdk.AlertDefPriorityP1:              "P1",
	}
	AlertPrioritySchemaToProtoMap = utils.ReverseMap(AlertPriorityProtoToSchemaMap)
	ValidAlertPriorities          = utils.GetKeys(AlertPrioritySchemaToProtoMap)

	NotifyOnProtoToSchemaMap = map[cxsdk.AlertNotifyOn]string{
		cxsdk.AlertNotifyOnTriggeredOnlyUnspecified: "Triggered Only",
		cxsdk.AlertNotifyOnTriggeredAndResolved:     "Triggered and Resolved",
	}
	NotifyOnSchemaToProtoMap = utils.ReverseMap(NotifyOnProtoToSchemaMap)
	ValidNotifyOn            = utils.GetKeys(NotifyOnSchemaToProtoMap)

	DaysOfWeekProtoToSchemaMap = map[cxsdk.AlertDayOfWeek]string{
		cxsdk.AlertDayOfWeekMonday:    "Monday",
		cxsdk.AlertDayOfWeekTuesday:   "Tuesday",
		cxsdk.AlertDayOfWeekWednesday: "Wednesday",
		cxsdk.AlertDayOfWeekThursday:  "Thursday",
		cxsdk.AlertDayOfWeekFriday:    "Friday",
		cxsdk.AlertDayOfWeekSaturday:  "Saturday",
		cxsdk.AlertDayOfWeekSunday:    "Sunday",
	}
	DaysOfWeekSchemaToProtoMap = utils.ReverseMap(DaysOfWeekProtoToSchemaMap)
	ValidDaysOfWeek            = utils.GetKeys(DaysOfWeekSchemaToProtoMap)

	LogFilterOperationTypeProtoToSchemaMap = map[cxsdk.LogFilterOperationType]string{
		cxsdk.LogFilterOperationIsOrUnspecified: "IS",
		cxsdk.LogFilterOperationIncludes:        "INCLUDES",
		cxsdk.LogFilterOperationEndsWith:        "ENDS_WITH",
		cxsdk.LogFilterOperationStartsWith:      "STARTS_WITH",
	}
	LogFilterOperationTypeSchemaToProtoMap = utils.ReverseMap(LogFilterOperationTypeProtoToSchemaMap)
	ValidLogFilterOperationType            = utils.GetKeys(LogFilterOperationTypeSchemaToProtoMap)

	LogSeverityProtoToSchemaMap = map[cxsdk.LogSeverity]string{
		cxsdk.LogSeverityVerboseUnspecified: "Unspecified",
		cxsdk.LogSeverityDebug:              "Debug",
		cxsdk.LogSeverityInfo:               "Info",
		cxsdk.LogSeverityWarning:            "Warning",
		cxsdk.LogSeverityError:              "Error",
		cxsdk.LogSeverityCritical:           "Critical",
	}
	LogSeveritySchemaToProtoMap = utils.ReverseMap(LogSeverityProtoToSchemaMap)
	ValidLogSeverities          = utils.GetKeys(LogSeveritySchemaToProtoMap)

	LogsTimeWindowValueProtoToSchemaMap = map[cxsdk.LogsTimeWindowValue]string{
		cxsdk.LogsTimeWindow5MinutesOrUnspecified: "5_MINUTES",
		cxsdk.LogsTimeWindow10Minutes:             "10_MINUTES",
		cxsdk.LogsTimeWindow15Minutes:             "15_MINUTES",
		cxsdk.LogsTimeWindow20Minutes:             "20_MINUTES",
		cxsdk.LogsTimeWindow30Minutes:             "30_MINUTES",
		cxsdk.LogsTimeWindow1Hour:                 "1_HOUR",
		cxsdk.LogsTimeWindow2Hours:                "2_HOURS",
		cxsdk.LogsTimeWindow4Hours:                "4_HOURS",
		cxsdk.LogsTimeWindow6Hours:                "6_HOURS",
		cxsdk.LogsTimeWindow12Hours:               "12_HOURS",
		cxsdk.LogsTimeWindow24Hours:               "24_HOURS",
		cxsdk.LogsTimeWindow36Hours:               "36_HOURS",
	}
	LogsTimeWindowValueSchemaToProtoMap = utils.ReverseMap(LogsTimeWindowValueProtoToSchemaMap)
	ValidLogsTimeWindowValues           = utils.GetKeys(LogsTimeWindowValueSchemaToProtoMap)

	AutoRetireTimeframeProtoToSchemaMap = map[cxsdk.AutoRetireTimeframe]string{
		cxsdk.AutoRetireTimeframeNeverOrUnspecified: "NEVER",
		cxsdk.AutoRetireTimeframe5Minutes:           "5_MINUTES",
		cxsdk.AutoRetireTimeframe10Minutes:          "10_MINUTES",
		cxsdk.AutoRetireTimeframe1Hour:              "1_HOUR",
		cxsdk.AutoRetireTimeframe2Hours:             "2_HOURS",
		cxsdk.AutoRetireTimeframe6Hours:             "6_HOURS",
		cxsdk.AutoRetireTimeframe12Hours:            "12_HOURS",
		cxsdk.AutoRetireTimeframe24Hours:            "24_HOURS",
	}
	AutoRetireTimeframeSchemaToProtoMap = utils.ReverseMap(AutoRetireTimeframeProtoToSchemaMap)
	ValidAutoRetireTimeframes           = utils.GetKeys(AutoRetireTimeframeSchemaToProtoMap)

	LogsRatioTimeWindowValueProtoToSchemaMap = map[cxsdk.LogsRatioTimeWindowValue]string{
		cxsdk.LogsRatioTimeWindowValue5MinutesOrUnspecified: "5_MINUTES",
		cxsdk.LogsRatioTimeWindowValue10Minutes:             "10_MINUTES",
		cxsdk.LogsRatioTimeWindowValue15Minutes:             "15_MINUTES",
		cxsdk.LogsRatioTimeWindowValue30Minutes:             "30_MINUTES",
		cxsdk.LogsRatioTimeWindowValue1Hour:                 "1_HOUR",
		cxsdk.LogsRatioTimeWindowValue2Hours:                "2_HOURS",
		cxsdk.LogsRatioTimeWindowValue4Hours:                "4_HOURS",
		cxsdk.LogsRatioTimeWindowValue6Hours:                "6_HOURS",
		cxsdk.LogsRatioTimeWindowValue12Hours:               "12_HOURS",
		cxsdk.LogsRatioTimeWindowValue24Hours:               "24_HOURS",
		cxsdk.LogsRatioTimeWindowValue36Hours:               "36_HOURS",
	}
	LogsRatioTimeWindowValueSchemaToProtoMap = utils.ReverseMap(LogsRatioTimeWindowValueProtoToSchemaMap)
	ValidLogsRatioTimeWindowValues           = utils.GetKeys(LogsRatioTimeWindowValueSchemaToProtoMap)

	LogsRatioGroupByForProtoToSchemaMap = map[cxsdk.LogsRatioGroupByFor]string{
		cxsdk.LogsRatioGroupByForBothOrUnspecified: "Both",
		cxsdk.LogsRatioGroupByForNumeratorOnly:     "Numerator Only",
		cxsdk.LogsRatioGroupByForDenumeratorOnly:   "Denominator Only",
	}
	LogsRatioGroupByForSchemaToProtoMap = utils.ReverseMap(LogsRatioGroupByForProtoToSchemaMap)
	ValidLogsRatioGroupByFor            = utils.GetKeys(LogsRatioGroupByForSchemaToProtoMap)

	LogsNewValueTimeWindowValueProtoToSchemaMap = map[cxsdk.LogsNewValueTimeWindowValue]string{
		cxsdk.LogsNewValueTimeWindowValue12HoursOrUnspecified: "12_HOURS",
		cxsdk.LogsNewValueTimeWindowValue24Hours:              "24_HOURS",
		cxsdk.LogsNewValueTimeWindowValue48Hours:              "48_HOURS",
		cxsdk.LogsNewValueTimeWindowValue72Hours:              "72_HOURS",
		cxsdk.LogsNewValueTimeWindowValue1Week:                "1_WEEK",
		cxsdk.LogsNewValueTimeWindowValue1Month:               "1_MONTH",
		cxsdk.LogsNewValueTimeWindowValue2Months:              "2_MONTHS",
		cxsdk.LogsNewValueTimeWindowValue3Months:              "3_MONTHS",
	}
	LogsNewValueTimeWindowValueSchemaToProtoMap = utils.ReverseMap(LogsNewValueTimeWindowValueProtoToSchemaMap)
	ValidLogsNewValueTimeWindowValues           = utils.GetKeys(LogsNewValueTimeWindowValueSchemaToProtoMap)

	LogsUniqueCountTimeWindowValueProtoToSchemaMap = map[cxsdk.LogsUniqueValueTimeWindowValue]string{
		cxsdk.LogsUniqueValueTimeWindowValue1MinuteOrUnspecified: "1_MINUTE",
		cxsdk.LogsUniqueValueTimeWindowValue5Minutes:             "5_MINUTES",
		cxsdk.LogsUniqueValueTimeWindowValue10Minutes:            "10_MINUTES",
		cxsdk.LogsUniqueValueTimeWindowValue15Minutes:            "15_MINUTES",
		cxsdk.LogsUniqueValueTimeWindowValue20Minutes:            "20_MINUTES",
		cxsdk.LogsUniqueValueTimeWindowValue30Minutes:            "30_MINUTES",
		cxsdk.LogsUniqueValueTimeWindowValue1Hour:                "1_HOUR",
		cxsdk.LogsUniqueValueTimeWindowValue2Hours:               "2_HOURS",
		cxsdk.LogsUniqueValueTimeWindowValue4Hours:               "4_HOURS",
		cxsdk.LogsUniqueValueTimeWindowValue6Hours:               "6_HOURS",
		cxsdk.LogsUniqueValueTimeWindowValue12Hours:              "12_HOURS",
		cxsdk.LogsUniqueValueTimeWindowValue24Hours:              "24_HOURS",
		cxsdk.LogsUniqueValueTimeWindowValue36Hours:              "36_HOURS",
	}
	LogsUniqueCountTimeWindowValueSchemaToProtoMap = utils.ReverseMap(LogsUniqueCountTimeWindowValueProtoToSchemaMap)
	ValidLogsUniqueCountTimeWindowValues           = utils.GetKeys(LogsUniqueCountTimeWindowValueSchemaToProtoMap)

	LogsTimeRelativeComparedToProtoToSchemaMap = map[cxsdk.LogsTimeRelativeComparedTo]string{
		cxsdk.LogsTimeRelativeComparedToPreviousHourOrUnspecified: "Previous Hour",
		cxsdk.LogsTimeRelativeComparedToSameHourYesterday:         "Same Hour Yesterday",
		cxsdk.LogsTimeRelativeComparedToSameHourLastWeek:          "Same Hour Last Week",
		cxsdk.LogsTimeRelativeComparedToYesterday:                 "Yesterday",
		cxsdk.LogsTimeRelativeComparedToSameDayLastWeek:           "Same Day Last Week",
		cxsdk.LogsTimeRelativeComparedToSameDayLastMonth:          "Same Day Last Month",
	}
	LogsTimeRelativeComparedToSchemaToProtoMap = utils.ReverseMap(LogsTimeRelativeComparedToProtoToSchemaMap)
	ValidLogsTimeRelativeComparedTo            = utils.GetKeys(LogsTimeRelativeComparedToSchemaToProtoMap)

	MetricFilterOperationTypeProtoToSchemaMap = map[cxsdk.MetricTimeWindowValue]string{
		cxsdk.MetricTimeWindowValue1MinuteOrUnspecified: "1_MINUTE",
		cxsdk.MetricTimeWindowValue5Minutes:             "5_MINUTES",
		cxsdk.MetricTimeWindowValue10Minutes:            "10_MINUTES",
		cxsdk.MetricTimeWindowValue15Minutes:            "15_MINUTES",
		cxsdk.MetricTimeWindowValue20Minutes:            "20_MINUTES",
		cxsdk.MetricTimeWindowValue30Minutes:            "30_MINUTES",
		cxsdk.MetricTimeWindowValue1Hour:                "1_HOUR",
		cxsdk.MetricTimeWindowValue2Hours:               "2_HOURS",
		cxsdk.MetricTimeWindowValue4Hours:               "4_HOURS",
		cxsdk.MetricTimeWindowValue6Hours:               "6_HOURS",
		cxsdk.MetricTimeWindowValue12Hours:              "12_HOURS",
		cxsdk.MetricTimeWindowValue24Hours:              "24_HOURS",
		cxsdk.MetricTimeWindowValue36Hours:              "36_HOURS",
	}
	MetricTimeWindowValueSchemaToProtoMap = utils.ReverseMap(MetricFilterOperationTypeProtoToSchemaMap)
	ValidMetricTimeWindowValues           = utils.GetKeys(MetricTimeWindowValueSchemaToProtoMap)

	TracingTimeWindowProtoToSchemaMap = map[cxsdk.TracingTimeWindowValue]string{
		cxsdk.TracingTimeWindowValue5MinutesOrUnspecified: "5_MINUTES",
		cxsdk.TracingTimeWindowValue10Minutes:             "10_MINUTES",
		cxsdk.TracingTimeWindowValue15Minutes:             "15_MINUTES",
		cxsdk.TracingTimeWindowValue20Minutes:             "20_MINUTES",
		cxsdk.TracingTimeWindowValue30Minutes:             "30_MINUTES",
		cxsdk.TracingTimeWindowValue1Hour:                 "1_HOUR",
		cxsdk.TracingTimeWindowValue2Hours:                "2_HOURS",
		cxsdk.TracingTimeWindowValue4Hours:                "4_HOURS",
		cxsdk.TracingTimeWindowValue6Hours:                "6_HOURS",
		cxsdk.TracingTimeWindowValue12Hours:               "12_HOURS",
		cxsdk.TracingTimeWindowValue24Hours:               "24_HOURS",
		cxsdk.TracingTimeWindowValue36Hours:               "36_HOURS",
	}
	TracingTimeWindowSchemaToProtoMap = utils.ReverseMap(TracingTimeWindowProtoToSchemaMap)
	ValidTracingTimeWindow            = utils.GetKeys(TracingTimeWindowSchemaToProtoMap)

	TracingFilterOperationProtoToSchemaMap = map[cxsdk.TracingFilterOperationType]string{
		cxsdk.TracingFilterOperationTypeIsOrUnspecified: "IS",
		cxsdk.TracingFilterOperationTypeIsNot:           "IS_NOT",
		cxsdk.TracingFilterOperationTypeIncludes:        "INCLUDES",
		cxsdk.TracingFilterOperationTypeEndsWith:        "ENDS_WITH",
		cxsdk.TracingFilterOperationTypeStartsWith:      "STARTS_WITH",
	}
	TracingFilterOperationSchemaToProtoMap = utils.ReverseMap(TracingFilterOperationProtoToSchemaMap)
	ValidTracingFilterOperations           = utils.GetKeys(TracingFilterOperationSchemaToProtoMap)
	FlowStageTimeFrameTypeProtoToSchemaMap = map[cxsdk.TimeframeType]string{
		cxsdk.TimeframeTypeUnspecified: "Unspecified",
		cxsdk.TimeframeTypeUpTo:        "Up To",
	}
	FlowStageTimeFrameTypeSchemaToProtoMap = utils.ReverseMap(FlowStageTimeFrameTypeProtoToSchemaMap)
	ValidFlowStageTimeFrameTypes           = utils.GetKeys(FlowStageTimeFrameTypeSchemaToProtoMap)

	FlowStagesGroupNextOpProtoToSchemaMap = map[cxsdk.NextOp]string{
		cxsdk.NextOpAndOrUnspecified: "AND",
		cxsdk.NextOpOr:               "OR",
	}
	FlowStagesGroupNextOpSchemaToProtoMap = utils.ReverseMap(FlowStagesGroupNextOpProtoToSchemaMap)
	ValidFlowStagesGroupNextOps           = utils.GetKeys(FlowStagesGroupNextOpSchemaToProtoMap)

	FlowStagesGroupAlertsOpProtoToSchemaMap = map[cxsdk.AlertsOp]string{
		cxsdk.AlertsOpAndOrUnspecified: "AND",
		cxsdk.AlertsOpOr:               "OR",
	}
	FlowStagesGroupAlertsOpSchemaToProtoMap = utils.ReverseMap(FlowStagesGroupAlertsOpProtoToSchemaMap)
	ValidFlowStagesGroupAlertsOps           = utils.GetKeys(FlowStagesGroupAlertsOpSchemaToProtoMap)

	LogsThresholdConditionMap = map[cxsdk.LogsThresholdConditionType]string{
		cxsdk.LogsThresholdConditionTypeMoreThanOrUnspecified: "MORE_THAN",
		cxsdk.LogsThresholdConditionTypeLessThan:              "LESS_THAN",
	}
	LogsThresholdConditionToProtoMap = utils.ReverseMap(LogsThresholdConditionMap)
	LogsThresholdConditionValues     = utils.GetValues(LogsThresholdConditionMap)

	LogsTimeRelativeConditionMap = map[cxsdk.LogsTimeRelativeConditionType]string{
		cxsdk.LogsTimeRelativeConditionTypeMoreThanOrUnspecified: "MORE_THAN",
		cxsdk.LogsTimeRelativeConditionTypeLessThan:              "LESS_THAN",
	}
	LogsTimeRelativeConditionToProtoMap = utils.ReverseMap(LogsTimeRelativeConditionMap)
	LogsTimeRelativeConditionValues     = utils.GetValues(LogsTimeRelativeConditionMap)

	LogsRatioConditionMap = map[cxsdk.LogsRatioConditionType]string{
		cxsdk.LogsRatioConditionTypeMoreThanOrUnspecified: "MORE_THAN",
		cxsdk.LogsRatioConditionTypeLessThan:              "LESS_THAN",
	}
	LogsRatioConditionMapValues        = utils.GetValues(LogsRatioConditionMap)
	LogsRatioConditionSchemaToProtoMap = utils.ReverseMap(LogsRatioConditionMap)

	MetricsThresholdConditionMap = map[cxsdk.MetricThresholdConditionType]string{
		cxsdk.MetricThresholdConditionTypeMoreThanOrUnspecified: "MORE_THAN",
		cxsdk.MetricThresholdConditionTypeLessThan:              "LESS_THAN",
		cxsdk.MetricThresholdConditionTypeMoreThanOrEquals:      "MORE_THAN_OR_EQUALS",
		cxsdk.MetricThresholdConditionTypeLessThanOrEquals:      "LESS_THAN_OR_EQUALS",
	}
	MetricsThresholdConditionValues     = utils.GetValues(MetricsThresholdConditionMap)
	MetricsThresholdConditionToProtoMap = utils.ReverseMap(MetricsThresholdConditionMap)

	MetricAnomalyConditionMap = map[cxsdk.MetricAnomalyConditionType]string{
		cxsdk.MetricAnomalyConditionTypeMoreThanOrUnspecified: "MORE_THAN",
		cxsdk.MetricAnomalyConditionTypeLessThan:              "LESS_THAN",
	}
	MetricAnomalyConditionValues     = utils.GetValues(MetricAnomalyConditionMap)
	MetricAnomalyConditionToProtoMap = utils.ReverseMap(MetricAnomalyConditionMap)
	LogsAnomalyConditionMap          = map[cxsdk.LogsAnomalyConditionType]string{
		cxsdk.LogsAnomalyConditionTypeMoreThanOrUnspecified: "MORE_THAN_USUAL",
	}
	LogsAnomalyConditionSchemaToProtoMap = utils.ReverseMap(LogsAnomalyConditionMap)
	// LogsAnomalyConditionValues           = utils.GetValues(LogsAnomalyConditionMap)

	DurationUnitProtoToSchemaMap = map[cxsdk.SloDurationUnit]string{
		cxsdk.DurationUnitUnspecified: "UNSPECIFIED",
		cxsdk.DurationUnitHours:       "HOURS",
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
