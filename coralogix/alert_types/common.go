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
	"terraform-provider-coralogix/coralogix/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
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
		cxsdk.LogFilterOperationIncludes:        "NOT", // includes?
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
