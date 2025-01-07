// Copyright 2024 Coralogix Ltd.
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

package coralogix

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"time"

	"terraform-provider-coralogix/coralogix/clientset"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"

	"github.com/hashicorp/terraform-plugin-framework-validators/boolvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// Format to parse time from and format to
const TIME_FORMAT = "15:03"

// Format to parse offset from and format to
const OFFSET_FORMAT = "Z0700"

var (
	_              resource.ResourceWithConfigure   = &AlertResource{}
	_              resource.ResourceWithImportState = &AlertResource{}
	createAlertURL                                  = cxsdk.CreateAlertDefRPC
	updateAlertURL                                  = cxsdk.ReplaceAlertDefRPC
	getAlertURL                                     = cxsdk.GetAlertDefRPC
	deleteAlertURL                                  = cxsdk.DeleteAlertDefRPC

	alertPriorityProtoToSchemaMap = map[cxsdk.AlertDefPriority]string{
		cxsdk.AlertDefPriorityP5OrUnspecified: "P5",
		cxsdk.AlertDefPriorityP4:              "P4",
		cxsdk.AlertDefPriorityP3:              "P3",
		cxsdk.AlertDefPriorityP2:              "P2",
		cxsdk.AlertDefPriorityP1:              "P1",
	}
	alertPrioritySchemaToProtoMap = ReverseMap(alertPriorityProtoToSchemaMap)
	validAlertPriorities          = GetKeys(alertPrioritySchemaToProtoMap)

	notifyOnProtoToSchemaMap = map[cxsdk.AlertNotifyOn]string{
		cxsdk.AlertNotifyOnTriggeredOnlyUnspecified: "Triggered Only",
		cxsdk.AlertNotifyOnTriggeredAndResolved:     "Triggered and Resolved",
	}
	notifyOnSchemaToProtoMap = ReverseMap(notifyOnProtoToSchemaMap)
	validNotifyOn            = GetKeys(notifyOnSchemaToProtoMap)

	daysOfWeekProtoToSchemaMap = map[cxsdk.AlertDayOfWeek]string{
		cxsdk.AlertDayOfWeekMonday:    "Monday",
		cxsdk.AlertDayOfWeekTuesday:   "Tuesday",
		cxsdk.AlertDayOfWeekWednesday: "Wednesday",
		cxsdk.AlertDayOfWeekThursday:  "Thursday",
		cxsdk.AlertDayOfWeekFriday:    "Friday",
		cxsdk.AlertDayOfWeekSaturday:  "Saturday",
		cxsdk.AlertDayOfWeekSunday:    "Sunday",
	}
	daysOfWeekSchemaToProtoMap = ReverseMap(daysOfWeekProtoToSchemaMap)
	validDaysOfWeek            = GetKeys(daysOfWeekSchemaToProtoMap)

	logFilterOperationTypeProtoToSchemaMap = map[cxsdk.LogFilterOperationType]string{
		cxsdk.LogFilterOperationIsOrUnspecified: "IS",
		cxsdk.LogFilterOperationIncludes:        "NOT", // includes?
		cxsdk.LogFilterOperationEndsWith:        "ENDS_WITH",
		cxsdk.LogFilterOperationStartsWith:      "STARTS_WITH",
	}
	logFilterOperationTypeSchemaToProtoMap = ReverseMap(logFilterOperationTypeProtoToSchemaMap)
	validLogFilterOperationType            = GetKeys(logFilterOperationTypeSchemaToProtoMap)

	logSeverityProtoToSchemaMap = map[cxsdk.LogSeverity]string{
		cxsdk.LogSeverityVerboseUnspecified: "Unspecified",
		cxsdk.LogSeverityDebug:              "Debug",
		cxsdk.LogSeverityInfo:               "Info",
		cxsdk.LogSeverityWarning:            "Warning",
		cxsdk.LogSeverityError:              "Error",
		cxsdk.LogSeverityCritical:           "Critical",
	}
	logSeveritySchemaToProtoMap = ReverseMap(logSeverityProtoToSchemaMap)
	validLogSeverities          = GetKeys(logSeveritySchemaToProtoMap)

	logsTimeWindowValueProtoToSchemaMap = map[cxsdk.LogsTimeWindowValue]string{
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
	logsTimeWindowValueSchemaToProtoMap = ReverseMap(logsTimeWindowValueProtoToSchemaMap)
	validLogsTimeWindowValues           = GetKeys(logsTimeWindowValueSchemaToProtoMap)

	autoRetireTimeframeProtoToSchemaMap = map[cxsdk.AutoRetireTimeframe]string{
		cxsdk.AutoRetireTimeframeNeverOrUnspecified: "NEVER",
		cxsdk.AutoRetireTimeframe5Minutes:           "5_MINUTES",
		cxsdk.AutoRetireTimeframe10Minutes:          "10_MINUTES",
		cxsdk.AutoRetireTimeframe1Hour:              "1_HOUR",
		cxsdk.AutoRetireTimeframe2Hours:             "2_HOURS",
		cxsdk.AutoRetireTimeframe6Hours:             "6_HOURS",
		cxsdk.AutoRetireTimeframe12Hours:            "12_HOURS",
		cxsdk.AutoRetireTimeframe24Hours:            "24_HOURS",
	}
	autoRetireTimeframeSchemaToProtoMap = ReverseMap(autoRetireTimeframeProtoToSchemaMap)
	validAutoRetireTimeframes           = GetKeys(autoRetireTimeframeSchemaToProtoMap)

	logsRatioTimeWindowValueProtoToSchemaMap = map[cxsdk.LogsRatioTimeWindowValue]string{
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
	logsRatioTimeWindowValueSchemaToProtoMap = ReverseMap(logsRatioTimeWindowValueProtoToSchemaMap)
	validLogsRatioTimeWindowValues           = GetKeys(logsRatioTimeWindowValueSchemaToProtoMap)

	logsRatioGroupByForProtoToSchemaMap = map[cxsdk.LogsRatioGroupByFor]string{
		cxsdk.LogsRatioGroupByForBothOrUnspecified: "Both",
		cxsdk.LogsRatioGroupByForNumeratorOnly:     "Numerator Only",
		cxsdk.LogsRatioGroupByForDenumeratorOnly:   "Denominator Only",
	}
	logsRatioGroupByForSchemaToProtoMap = ReverseMap(logsRatioGroupByForProtoToSchemaMap)
	validLogsRatioGroupByFor            = GetKeys(logsRatioGroupByForSchemaToProtoMap)

	logsNewValueTimeWindowValueProtoToSchemaMap = map[cxsdk.LogsNewValueTimeWindowValue]string{
		cxsdk.LogsNewValueTimeWindowValue12HoursOrUnspecified: "12_HOURS",
		cxsdk.LogsNewValueTimeWindowValue24Hours:              "24_HOURS",
		cxsdk.LogsNewValueTimeWindowValue48Hours:              "48_HOURS",
		cxsdk.LogsNewValueTimeWindowValue72Hours:              "72_HOURS",
		cxsdk.LogsNewValueTimeWindowValue1Week:                "1_WEEK",
		cxsdk.LogsNewValueTimeWindowValue1Month:               "1_MONTH",
		cxsdk.LogsNewValueTimeWindowValue2Months:              "2_MONTHS",
		cxsdk.LogsNewValueTimeWindowValue3Months:              "3_MONTHS",
	}
	logsNewValueTimeWindowValueSchemaToProtoMap = ReverseMap(logsNewValueTimeWindowValueProtoToSchemaMap)
	validLogsNewValueTimeWindowValues           = GetKeys(logsNewValueTimeWindowValueSchemaToProtoMap)

	logsUniqueCountTimeWindowValueProtoToSchemaMap = map[cxsdk.LogsUniqueValueTimeWindowValue]string{
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
	logsUniqueCountTimeWindowValueSchemaToProtoMap = ReverseMap(logsUniqueCountTimeWindowValueProtoToSchemaMap)
	validLogsUniqueCountTimeWindowValues           = GetKeys(logsUniqueCountTimeWindowValueSchemaToProtoMap)

	logsTimeRelativeComparedToProtoToSchemaMap = map[cxsdk.LogsTimeRelativeComparedTo]string{
		cxsdk.LogsTimeRelativeComparedToPreviousHourOrUnspecified: "Previous Hour",
		cxsdk.LogsTimeRelativeComparedToSameHourYesterday:         "Same Hour Yesterday",
		cxsdk.LogsTimeRelativeComparedToSameHourLastWeek:          "Same Hour Last Week",
		cxsdk.LogsTimeRelativeComparedToYesterday:                 "Yesterday",
		cxsdk.LogsTimeRelativeComparedToSameDayLastWeek:           "Same Day Last Week",
		cxsdk.LogsTimeRelativeComparedToSameDayLastMonth:          "Same Day Last Month",
	}
	logsTimeRelativeComparedToSchemaToProtoMap = ReverseMap(logsTimeRelativeComparedToProtoToSchemaMap)
	validLogsTimeRelativeComparedTo            = GetKeys(logsTimeRelativeComparedToSchemaToProtoMap)

	metricFilterOperationTypeProtoToSchemaMap = map[cxsdk.MetricTimeWindowValue]string{
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
	metricTimeWindowValueSchemaToProtoMap = ReverseMap(metricFilterOperationTypeProtoToSchemaMap)
	validMetricTimeWindowValues           = GetKeys(metricTimeWindowValueSchemaToProtoMap)

	tracingTimeWindowProtoToSchemaMap = map[cxsdk.TracingTimeWindowValue]string{
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
	tracingTimeWindowSchemaToProtoMap = ReverseMap(tracingTimeWindowProtoToSchemaMap)
	validTracingTimeWindow            = GetKeys(tracingTimeWindowSchemaToProtoMap)

	tracingFilterOperationProtoToSchemaMap = map[cxsdk.TracingFilterOperationType]string{
		cxsdk.TracingFilterOperationTypeIsOrUnspecified: "IS",
		cxsdk.TracingFilterOperationTypeIsNot:           "IS_NOT",
		cxsdk.TracingFilterOperationTypeIncludes:        "INCLUDES",
		cxsdk.TracingFilterOperationTypeEndsWith:        "ENDS_WITH",
		cxsdk.TracingFilterOperationTypeStartsWith:      "STARTS_WITH",
	}
	tracingFilterOperationSchemaToProtoMap = ReverseMap(tracingFilterOperationProtoToSchemaMap)
	validTracingFilterOperations           = GetKeys(tracingFilterOperationSchemaToProtoMap)
	flowStageTimeFrameTypeProtoToSchemaMap = map[cxsdk.TimeframeType]string{
		cxsdk.TimeframeTypeUnspecified: "Unspecified",
		cxsdk.TimeframeTypeUpTo:        "Up To",
	}
	flowStageTimeFrameTypeSchemaToProtoMap = ReverseMap(flowStageTimeFrameTypeProtoToSchemaMap)
	validFlowStageTimeFrameTypes           = GetKeys(flowStageTimeFrameTypeSchemaToProtoMap)

	flowStagesGroupNextOpProtoToSchemaMap = map[cxsdk.NextOp]string{
		cxsdk.NextOpAndOrUnspecified: "AND",
		cxsdk.NextOpOr:               "OR",
	}
	flowStagesGroupNextOpSchemaToProtoMap = ReverseMap(flowStagesGroupNextOpProtoToSchemaMap)
	validFlowStagesGroupNextOps           = GetKeys(flowStagesGroupNextOpSchemaToProtoMap)

	flowStagesGroupAlertsOpProtoToSchemaMap = map[cxsdk.AlertsOp]string{
		cxsdk.AlertsOpAndOrUnspecified: "AND",
		cxsdk.AlertsOpOr:               "OR",
	}
	flowStagesGroupAlertsOpSchemaToProtoMap = ReverseMap(flowStagesGroupAlertsOpProtoToSchemaMap)
	validFlowStagesGroupAlertsOps           = GetKeys(flowStagesGroupAlertsOpSchemaToProtoMap)

	logsThresholdConditionMap = map[cxsdk.LogsThresholdConditionType]string{
		cxsdk.LogsThresholdConditionTypeMoreThanOrUnspecified: "MORE_THAN",
		cxsdk.LogsThresholdConditionTypeLessThan:              "LESS_THAN",
	}
	logsThresholdConditionToProtoMap = ReverseMap(logsThresholdConditionMap)
	logsThresholdConditionValues     = GetValues(logsThresholdConditionMap)

	logsTimeRelativeConditionMap = map[cxsdk.LogsTimeRelativeConditionType]string{
		cxsdk.LogsTimeRelativeConditionTypeMoreThanOrUnspecified: "MORE_THAN",
		cxsdk.LogsTimeRelativeConditionTypeLessThan:              "LESS_THAN",
	}
	logsTimeRelativeConditionToProtoMap = ReverseMap(logsTimeRelativeConditionMap)
	logsTimeRelativeConditionValues     = GetValues(logsTimeRelativeConditionMap)

	logsRatioConditionMap = map[cxsdk.LogsRatioConditionType]string{
		cxsdk.LogsRatioConditionTypeMoreThanOrUnspecified: "MORE_THAN",
		cxsdk.LogsRatioConditionTypeLessThan:              "LESS_THAN",
	}
	logsRatioConditionMapValues        = GetValues(logsRatioConditionMap)
	logsRatioConditionSchemaToProtoMap = ReverseMap(logsRatioConditionMap)

	metricsThresholdConditionMap = map[cxsdk.MetricThresholdConditionType]string{
		cxsdk.MetricThresholdConditionTypeMoreThanOrUnspecified: "MORE_THAN",
		cxsdk.MetricThresholdConditionTypeLessThan:              "LESS_THAN",
		cxsdk.MetricThresholdConditionTypeMoreThanOrEquals:      "MORE_THAN_OR_EQUALS",
		cxsdk.MetricThresholdConditionTypeLessThanOrEquals:      "LESS_THAN_OR_EQUALS",
	}
	metricsThresholdConditionValues     = GetValues(metricsThresholdConditionMap)
	metricsThresholdConditionToProtoMap = ReverseMap(metricsThresholdConditionMap)

	metricAnomalyConditionMap = map[cxsdk.MetricAnomalyConditionType]string{
		cxsdk.MetricAnomalyConditionTypeMoreThanOrUnspecified: "MORE_THAN",
		cxsdk.MetricAnomalyConditionTypeLessThan:              "LESS_THAN",
	}
	metricAnomalyConditionValues     = GetValues(metricAnomalyConditionMap)
	metricAnomalyConditionToProtoMap = ReverseMap(metricAnomalyConditionMap)
	logsAnomalyConditionMap          = map[cxsdk.LogsAnomalyConditionType]string{
		cxsdk.LogsAnomalyConditionTypeMoreThanOrUnspecified: "MORE_THAN_USUAL",
	}
	logsAnomalyConditionSchemaToProtoMap = ReverseMap(logsAnomalyConditionMap)
	// logsAnomalyConditionValues           = GetValues(logsAnomalyConditionMap)
)

func NewAlertResource() resource.Resource {
	return &AlertResource{}
}

type AlertResource struct {
	client *cxsdk.AlertsClient
}

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
	GroupBy           types.Set    `tfsdk:"group_by"`           // []types.String
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
}

type IncidentsSettingsModel struct {
	NotifyOn           types.String `tfsdk:"notify_on"`
	RetriggeringPeriod types.Object `tfsdk:"retriggering_period"` // RetriggeringPeriodModel
}

type NotificationGroupModel struct {
	GroupByKeys      types.Set `tfsdk:"group_by_keys"`     // []types.String
	WebhooksSettings types.Set `tfsdk:"webhooks_settings"` // WebhooksSettingsModel
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
}

type LogsAnomalyModel struct {
	Rules                     types.Set    `tfsdk:"rules"`                       // [] LogsAnomalyRuleModel
	LogsFilter                types.Object `tfsdk:"logs_filter"`                 // AlertsLogsFilterModel
	NotificationPayloadFilter types.Set    `tfsdk:"notification_payload_filter"` // []types.String
}

type LogsRatioThresholdModel struct {
	Rules                     types.Set    `tfsdk:"rules"`     // []LogsRatioThresholdRuleModel
	Numerator                 types.Object `tfsdk:"numerator"` // AlertsLogsFilterModel
	NumeratorAlias            types.String `tfsdk:"numerator_alias"`
	Denominator               types.Object `tfsdk:"denominator"` // AlertsLogsFilterModel
	DenominatorAlias          types.String `tfsdk:"denominator_alias"`
	NotificationPayloadFilter types.Set    `tfsdk:"notification_payload_filter"` // []types.String
	GroupByFor                types.String `tfsdk:"group_by_for"`
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
	MetricFilter types.Object `tfsdk:"metric_filter"` // MetricFilterModel
	Rules        types.Set    `tfsdk:"rules"`         // [] MetricAnomalyRuleModel
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

func (r *AlertResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alert"
}

func (r *AlertResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clientSet, ok := req.ProviderData.(*clientset.ClientSet)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *cxsdk.ClientSet, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = clientSet.Alerts()
}

func (r *AlertResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:             1,
		MarkdownDescription: "Coralogix Alert. For more info please review - https://coralogix.com/docs/getting-started-with-coralogix-alerts/.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Alert ID.",
			},
			"name": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Alert name.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Alert description.",
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Alert enabled status. True by default.",
			},
			"priority": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(validAlertPriorities...),
				},
				MarkdownDescription: fmt.Sprintf("Alert priority. Valid values: %q.", validAlertPriorities),
			},
			"schedule": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"active_on": schema.SingleNestedAttribute{
						Required: true,
						Attributes: map[string]schema.Attribute{
							"days_of_week": schema.SetAttribute{
								Required:    true,
								ElementType: types.StringType,
								Validators: []validator.Set{
									setvalidator.ValueStringsAre(
										stringvalidator.OneOf(validDaysOfWeek...),
									),
								},
								MarkdownDescription: fmt.Sprintf("Days of the week. Valid values: %q.", validDaysOfWeek),
							},
							"start_time": schema.StringAttribute{
								Required: true,
								Validators: []validator.String{
									stringvalidator.RegexMatches(
										regexp.MustCompile(`^[0-9]{1,2}:[0-9]{1,2}$`),
										"Use 24h time formats like 15:04 or 9:04",
									),
								},
							},
							"end_time": schema.StringAttribute{
								Required: true,
								Validators: []validator.String{
									stringvalidator.RegexMatches(
										regexp.MustCompile(`^[0-9]{1,2}:[0-9]{1,2}$`),
										"Use 24h time formats like 15:04 or 9:04",
									),
								},
							},
							"utc_offset": schema.StringAttribute{
								Optional: true,
								Default:  stringdefault.StaticString("+0000"),
								Computed: true,
								Validators: []validator.String{
									stringvalidator.RegexMatches(
										regexp.MustCompile(`^[-+][0-9]{4}$`),
										"Time zone to interpret the start/end times in, using a UTC offset like -0700",
									),
								},
							},
						},
					},
				},
				MarkdownDescription: "Alert schedule. Will be activated all the time if not specified.",
			},
			// type is being inferred by the type_definition attribute
			"type_definition": schema.SingleNestedAttribute{
				Required:            true,
				MarkdownDescription: "Alert type definition. Exactly one of the following must be specified: logs_immediate, logs_threshold, logs_anomaly, logs_ratio_threshold, logs_new_value, logs_unique_count, logs_time_relative_threshold, metric_threshold, metric_anomaly, tracing_immediate, tracing_threshold flow.",
				Attributes: map[string]schema.Attribute{
					"logs_immediate": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"logs_filter":                 logsFilterSchema(),
							"notification_payload_filter": notificationPayloadFilterSchema(),
						},
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(
								path.MatchRoot("type_definition").AtName("logs_threshold"),
								path.MatchRoot("type_definition").AtName("logs_anomaly"),
								path.MatchRoot("type_definition").AtName("logs_ratio_threshold"),
								path.MatchRoot("type_definition").AtName("logs_unique_count"),
								path.MatchRoot("type_definition").AtName("logs_new_value"),
								path.MatchRoot("type_definition").AtName("logs_time_relative_threshold"),
								path.MatchRoot("type_definition").AtName("metric_threshold"),
								path.MatchRoot("type_definition").AtName("metric_anomaly"),
								path.MatchRoot("type_definition").AtName("tracing_immediate"),
								path.MatchRoot("type_definition").AtName("tracing_threshold"),
								path.MatchRoot("type_definition").AtName("flow"),
							),
						},
					},
					"logs_threshold": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"rules": schema.SetNestedAttribute{
								Required:   true,
								Validators: []validator.Set{setvalidator.SizeAtLeast(1)},
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"condition": schema.SingleNestedAttribute{
											Required: true,
											Attributes: map[string]schema.Attribute{
												"threshold": schema.Float64Attribute{
													Required: true,
												},
												"time_window": logsTimeWindowSchema(validLogsTimeWindowValues),
												"condition_type": schema.StringAttribute{
													Required: true,
													Validators: []validator.String{
														stringvalidator.OneOf(logsThresholdConditionValues...),
													},
													MarkdownDescription: fmt.Sprintf("Condition to evaluate the threshold with. Valid values: %q.", logsThresholdConditionValues),
												},
											},
										},
										"override": overrideAlertSchema(),
									},
								},
							},
							"notification_payload_filter":  notificationPayloadFilterSchema(),
							"logs_filter":                  logsFilterSchema(),
							"undetected_values_management": undetectedValuesManagementSchema(),
						},
					},
					"logs_anomaly": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"logs_filter":                 logsFilterSchema(),
							"notification_payload_filter": notificationPayloadFilterSchema(),
							"rules": schema.SetNestedAttribute{
								Required:   true,
								Validators: []validator.Set{setvalidator.SizeAtLeast(1)},
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"condition": schema.SingleNestedAttribute{
											Required: true,
											Attributes: map[string]schema.Attribute{
												"time_window": logsTimeWindowSchema(validLogsTimeWindowValues),
												"minimum_threshold": schema.Float64Attribute{
													Required: true,
												},
												"condition_type": schema.StringAttribute{
													Computed: true,
													Default:  stringdefault.StaticString("MORE_THAN_USUAL"),
													PlanModifiers: []planmodifier.String{
														stringplanmodifier.UseStateForUnknown(),
													},
												},
											},
										},
									},
								},
							},
						},
					},
					"logs_ratio_threshold": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"rules": schema.SetNestedAttribute{
								Required:   true,
								Validators: []validator.Set{setvalidator.SizeAtLeast(1)},
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"condition": schema.SingleNestedAttribute{
											Required: true,
											Attributes: map[string]schema.Attribute{
												"threshold": schema.Float64Attribute{
													Required: true,
												},
												"time_window": logsTimeWindowSchema(validLogsRatioTimeWindowValues),
												"condition_type": schema.StringAttribute{
													Required: true,
													Validators: []validator.String{
														stringvalidator.OneOf(logsRatioConditionMapValues...),
													},
													MarkdownDescription: fmt.Sprintf("Condition to evaluate the threshold with. Valid values: %q.", logsRatioConditionMapValues),
												},
											},
										},
										"override": overrideAlertSchema(),
									},
								},
							},
							"numerator": logsFilterSchema(),
							"numerator_alias": schema.StringAttribute{
								Required: true,
							},
							"denominator": logsFilterSchema(),
							"denominator_alias": schema.StringAttribute{
								Required: true,
							},
							"notification_payload_filter": notificationPayloadFilterSchema(),
							"group_by_for":                logsRatioGroupByForSchema(),
						},
					},
					"logs_new_value": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"rules": schema.SetNestedAttribute{
								Required:   true,
								Validators: []validator.Set{setvalidator.SizeAtLeast(1)},
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"condition": schema.SingleNestedAttribute{
											Required: true,
											Attributes: map[string]schema.Attribute{
												"keypath_to_track": schema.StringAttribute{Required: true},
												"time_window":      logsTimeWindowSchema(validLogsNewValueTimeWindowValues),
											},
										},
									},
								},
							},
							"logs_filter":                 logsFilterSchema(),
							"notification_payload_filter": notificationPayloadFilterSchema(),
						},
					},
					"logs_unique_count": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"logs_filter":                 logsFilterSchema(),
							"notification_payload_filter": notificationPayloadFilterSchema(),
							"rules": schema.SetNestedAttribute{
								Required:   true,
								Validators: []validator.Set{setvalidator.SizeAtLeast(1)},
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"condition": schema.SingleNestedAttribute{
											Required: true,
											Attributes: map[string]schema.Attribute{
												"time_window":      logsTimeWindowSchema(validLogsUniqueCountTimeWindowValues),
												"max_unique_count": schema.Int64Attribute{Required: true},
											},
										},
									},
								},
							},
							"max_unique_count_per_group_by_key": schema.Int64Attribute{
								Optional: true,
							},
							"unique_count_keypath": schema.StringAttribute{
								Required: true,
							},
						},
					},
					"logs_time_relative_threshold": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"logs_filter":                  logsFilterSchema(),
							"notification_payload_filter":  notificationPayloadFilterSchema(),
							"undetected_values_management": undetectedValuesManagementSchema(),
							"rules": schema.SetNestedAttribute{
								Required:   true,
								Validators: []validator.Set{setvalidator.SizeAtLeast(1)},
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"condition": schema.SingleNestedAttribute{
											Required: true,
											Attributes: map[string]schema.Attribute{
												"condition_type": schema.StringAttribute{
													Required: true,
													Validators: []validator.String{
														stringvalidator.OneOf(logsTimeRelativeConditionValues...),
													},
													MarkdownDescription: fmt.Sprintf("Condition . Valid values: %q.", logsTimeRelativeConditionValues),
												},
												"threshold": schema.Float64Attribute{
													Required: true,
												},
												"compared_to": schema.StringAttribute{
													Required: true,
													Validators: []validator.String{
														stringvalidator.OneOf(validLogsTimeRelativeComparedTo...),
													},
													MarkdownDescription: fmt.Sprintf("Compared to a different time frame. Valid values: %q.", validLogsTimeRelativeComparedTo),
												},
											},
										},
										"override": overrideAlertSchema(),
									},
								},
							},
						},
					},
					// Metrics
					"metric_threshold": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"metric_filter":                metricFilterSchema(),
							"undetected_values_management": undetectedValuesManagementSchema(),
							"rules": schema.SetNestedAttribute{
								Required:   true,
								Validators: []validator.Set{setvalidator.SizeAtLeast(1)},
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"condition": schema.SingleNestedAttribute{
											Required: true,
											Attributes: map[string]schema.Attribute{
												"threshold": schema.Float64Attribute{
													Required: true,
												},
												"for_over_pct": schema.Int64Attribute{
													Required: true,
												},
												"of_the_last": metricTimeWindowSchema(),
												"condition_type": schema.StringAttribute{
													Required: true,
													Validators: []validator.String{
														stringvalidator.OneOf(metricsThresholdConditionValues...),
													},
													MarkdownDescription: fmt.Sprintf("Condition to evaluate the threshold with. Valid values: %q.", metricsThresholdConditionValues),
												},
											},
										},
										"override": overrideAlertSchema(),
									},
								},
							},
							"missing_values": schema.SingleNestedAttribute{
								Required: true,
								Attributes: map[string]schema.Attribute{
									"replace_with_zero": schema.BoolAttribute{
										Optional: true,
										Validators: []validator.Bool{
											boolvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("min_non_null_values_pct")),
										},
									},
									"min_non_null_values_pct": schema.Int64Attribute{
										Optional: true,
									},
								},
							},
						},
					},
					"metric_anomaly": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"metric_filter": metricFilterSchema(),
							"rules": schema.SetNestedAttribute{
								Required:   true,
								Validators: []validator.Set{setvalidator.SizeAtLeast(1)},
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"condition": schema.SingleNestedAttribute{
											Required: true,
											Attributes: map[string]schema.Attribute{
												"min_non_null_values_pct": schema.Int64Attribute{
													Required: true,
												},
												"threshold": schema.Float64Attribute{
													Required: true,
												},
												"for_over_pct": schema.Int64Attribute{
													Required: true,
												},
												"of_the_last": metricTimeWindowSchema(),
												"condition_type": schema.StringAttribute{
													Required: true,
													Validators: []validator.String{
														stringvalidator.OneOf(metricAnomalyConditionValues...),
													},
													MarkdownDescription: fmt.Sprintf("Condition to evaluate the threshold with. Valid values: %q.", metricAnomalyConditionValues),
												},
											},
										},
									},
								},
							},
						},
					},
					// Tracing
					"tracing_immediate": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"tracing_filter":              tracingQuerySchema(),
							"notification_payload_filter": notificationPayloadFilterSchema(),
						},
					},
					"tracing_threshold": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"tracing_filter":              tracingQuerySchema(),
							"notification_payload_filter": notificationPayloadFilterSchema(),
							"rules": schema.SetNestedAttribute{
								Required:   true,
								Validators: []validator.Set{setvalidator.SizeAtLeast(1)},
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"condition": schema.SingleNestedAttribute{
											Required: true,
											Attributes: map[string]schema.Attribute{
												"span_amount": schema.Float64Attribute{
													Required: true,
												},
												"time_window": logsTimeWindowSchema(validTracingTimeWindow),
												"condition_type": schema.StringAttribute{
													Computed: true,
													Default:  stringdefault.StaticString("MORE_THAN"),
													PlanModifiers: []planmodifier.String{
														stringplanmodifier.UseStateForUnknown(),
													},
												},
											},
										},
									},
									// Condition type is missing since there is only a single type to be filled in
								},
							},
						},
					},
					// Flow
					"flow": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"stages": schema.ListNestedAttribute{
								Required: true,
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"flow_stages_groups": schema.ListNestedAttribute{
											Required: true,
											NestedObject: schema.NestedAttributeObject{
												Attributes: map[string]schema.Attribute{
													"alert_defs": schema.SetNestedAttribute{
														Required: true,
														NestedObject: schema.NestedAttributeObject{
															Attributes: map[string]schema.Attribute{
																"id": schema.StringAttribute{
																	Required: true,
																},
																"not": schema.BoolAttribute{
																	Optional: true,
																	Computed: true,
																	Default:  booldefault.StaticBool(false),
																},
															},
														},
													},
													"next_op": schema.StringAttribute{
														Required: true,
														Validators: []validator.String{
															stringvalidator.OneOf(validFlowStagesGroupNextOps...),
														},
														MarkdownDescription: fmt.Sprintf("Next operation. Valid values: %q.", validFlowStagesGroupNextOps),
													},
													"alerts_op": schema.StringAttribute{
														Required: true,
														Validators: []validator.String{
															stringvalidator.OneOf(validFlowStagesGroupAlertsOps...),
														},
														MarkdownDescription: fmt.Sprintf("Alerts operation. Valid values: %q.", validFlowStagesGroupAlertsOps),
													},
												},
											},
										},
										"timeframe_ms": schema.Int64Attribute{
											Optional: true,
											Computed: true,
											Default:  int64default.StaticInt64(0),
										},
										"timeframe_type": schema.StringAttribute{
											Required: true,
											Validators: []validator.String{
												stringvalidator.OneOf(validFlowStageTimeFrameTypes...),
											},
										},
									},
								},
							},
							"enforce_suppression": schema.BoolAttribute{
								Optional: true,
								Computed: true,
								Default:  booldefault.StaticBool(false),
							},
						},
					},
				},
			},
			"phantom_mode": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"deleted": schema.BoolAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"group_by": schema.SetAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Set{
					ComputedForMetricAlerts{},
				},
				Validators: []validator.Set{
					//imidiate, new value, tracing-immidiate,
					GroupByValidator{},
				},
				ElementType:         types.StringType,
				MarkdownDescription: "Group by fields.",
			},
			"incidents_settings": schema.SingleNestedAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]schema.Attribute{
					"notify_on": schema.StringAttribute{
						Required: true,
						Validators: []validator.String{
							stringvalidator.OneOf(validNotifyOn...),
						},
						MarkdownDescription: fmt.Sprintf("Notify on. Valid values: %q.", validNotifyOn),
					},
					"retriggering_period": schema.SingleNestedAttribute{
						Required: true,
						Attributes: map[string]schema.Attribute{
							"minutes": schema.Int64Attribute{
								Required: true,
							},
						},
					},
				},
			},
			"notification_group": schema.SingleNestedAttribute{
				Optional: true,
				Computed: true,
				Default: objectdefault.StaticValue(types.ObjectValueMust(notificationGroupAttr(), map[string]attr.Value{
					"group_by_keys": types.SetNull(types.StringType),
					"webhooks_settings": types.SetNull(types.ObjectType{AttrTypes: map[string]attr.Type{
						"retriggering_period": types.ObjectType{AttrTypes: map[string]attr.Type{
							"minutes": types.Int64Type,
						}},
						"notify_on":      types.StringType,
						"integration_id": types.StringType,
						"recipients":     types.SetType{ElemType: types.StringType},
					}}),
				})),
				Attributes: map[string]schema.Attribute{
					"group_by_keys": schema.SetAttribute{
						Optional:    true,
						ElementType: types.StringType,
					},
					"webhooks_settings": schema.SetNestedAttribute{
						Optional: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"retriggering_period": schema.SingleNestedAttribute{
									Optional: true,
									Computed: true,
									Default: objectdefault.StaticValue(types.ObjectValueMust(retriggeringPeriodAttr(), map[string]attr.Value{
										"minutes": types.Int64Value(10),
									})),
									Attributes: map[string]schema.Attribute{
										"minutes": schema.Int64Attribute{
											Required: true,
										},
									},
									MarkdownDescription: "Retriggering period in minutes. 10 minutes by default.",
								},
								"notify_on": schema.StringAttribute{
									Optional: true,
									Computed: true,
									Default:  stringdefault.StaticString("Triggered Only"),
									Validators: []validator.String{
										stringvalidator.OneOf(validNotifyOn...),
									},
									MarkdownDescription: fmt.Sprintf("Notify on. Valid values: %q. Triggered Only by default.", validNotifyOn),
								},
								"integration_id": schema.StringAttribute{
									Optional: true,
									Validators: []validator.String{
										stringvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("recipients")),
									},
								},
								"recipients": schema.SetAttribute{
									Optional:    true,
									ElementType: types.StringType,
								},
							},
							PlanModifiers: []planmodifier.Object{
								objectplanmodifier.UseStateForUnknown(),
							},
						},
					},
				},
			},
			"labels": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func timeZoneSchema() {
	panic("todo")
}

type GroupByValidator struct {
}

func (g GroupByValidator) Description(ctx context.Context) string {
	return "Group by validator."
}

func (g GroupByValidator) MarkdownDescription(ctx context.Context) string {
	return "Group by validator."
}

func (g GroupByValidator) ValidateSet(ctx context.Context, request validator.SetRequest, response *validator.SetResponse) {
	paths, diags := request.Config.PathMatches(ctx, path.MatchRoot("type_definition"))
	if diags.HasError() {
		response.Diagnostics.Append(diags...)
		return
	}
	var typeDefinition AlertTypeDefinitionModel
	diags = request.Config.GetAttribute(ctx, paths[0], &typeDefinition)
	if diags.HasError() {
		response.Diagnostics.Append(diags...)
		return
	}

	if !objIsNullOrUnknown(typeDefinition.LogsImmediate) || !objIsNullOrUnknown(typeDefinition.LogsNewValue) || !objIsNullOrUnknown(typeDefinition.TracingImmediate) {
		if !(request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown()) {
			response.Diagnostics.AddError("group_by", "Group by is not allowed for logs_immediate, logs_new_value, tracing_immediate alert types.")
		}
	}
}

type ComputedForMetricAlerts struct {
}

func (c ComputedForMetricAlerts) Description(ctx context.Context) string {
	return "Computed for metric alerts."
}

func (c ComputedForMetricAlerts) MarkdownDescription(ctx context.Context) string {
	return "Computed for metric alerts."
}

func (c ComputedForMetricAlerts) PlanModifySet(ctx context.Context, request planmodifier.SetRequest, response *planmodifier.SetResponse) {
	paths, diags := request.Plan.PathMatches(ctx, path.MatchRoot("type_definition"))
	if diags.HasError() {
		response.Diagnostics.Append(diags...)
		return
	}
	var typeDefinition AlertTypeDefinitionModel
	diags = request.Plan.GetAttribute(ctx, paths[0], &typeDefinition)
	if diags.HasError() {
		response.Diagnostics.Append(diags...)
		return
	}

	var typeDefinitionStr string
	if !objIsNullOrUnknown(typeDefinition.MetricThreshold) {
		typeDefinitionStr = "metric_threshold"
	} else if !objIsNullOrUnknown(typeDefinition.MetricAnomaly) {
		typeDefinitionStr = "metric_anomaly"
	}

	if typeDefinitionStr != "" {
		paths, diags = request.Plan.PathMatches(ctx, path.MatchRoot("type_definition").AtName(typeDefinitionStr).AtName("metric_filter").AtName("promql"))
		if diags.HasError() {
			response.Diagnostics.Append(diags...)
			return
		}

		var promqlPlan types.String
		diags = request.Plan.GetAttribute(ctx, paths[0], &promqlPlan)
		if diags.HasError() {
			response.Diagnostics.Append(diags...)
			return
		}

		var promqlState types.String
		diags = request.State.GetAttribute(ctx, paths[0], &promqlState)
		if diags.HasError() {
			response.Diagnostics.Append(diags...)
			return
		}

		if request.ConfigValue.IsUnknown() || request.ConfigValue.IsNull() {
			if !promqlState.Equal(promqlPlan) {
				response.PlanValue = types.SetUnknown(types.StringType)
			} else {
				response.PlanValue = request.StateValue
			}
			return
		}
	}

	response.PlanValue = request.ConfigValue
}

func metricTimeWindowSchema() schema.StringAttribute {
	return schema.StringAttribute{
		Required: true,
		Validators: []validator.String{
			stringvalidator.OneOf(validMetricTimeWindowValues...),
		},
		MarkdownDescription: fmt.Sprintf("Time window to evaluate the threshold with. Valid values: %q.", validMetricTimeWindowValues),
	}
}

func logsTimeWindowSchema(validLogsTimeWindowValues []string) schema.StringAttribute {
	return schema.StringAttribute{
		Required: true,
		Validators: []validator.String{
			stringvalidator.OneOf(validLogsTimeWindowValues...),
		},
	}
}

func overrideAlertSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"priority": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf(validAlertPriorities...),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: fmt.Sprintf("Alert priority. Valid values: %q.", validAlertPriorities),
			},
		},
	}
}

func logsRatioGroupByForSchema() schema.StringAttribute {
	return schema.StringAttribute{
		Optional: true,
		Computed: true,
		Default:  stringdefault.StaticString("Both"),
		Validators: []validator.String{
			stringvalidator.OneOf(validLogsRatioGroupByFor...),
			stringvalidator.AlsoRequires(path.MatchRoot("group_by")),
		},
		MarkdownDescription: fmt.Sprintf("Group by for. Valid values: %q. 'Both' by default.", validLogsRatioGroupByFor),
	}
}

func tracingQuerySchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"latency_threshold_ms": schema.NumberAttribute{
				Required: true,
			},
			"tracing_label_filters": tracingLabelFiltersSchema(),
		},
	}
}

func tracingLabelFiltersSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"application_name": tracingFiltersTypeSchema(),
			"subsystem_name":   tracingFiltersTypeSchema(),
			"service_name":     tracingFiltersTypeSchema(),
			"operation_name":   tracingFiltersTypeSchema(),
			"span_fields":      tracingSpanFieldsFilterSchema(),
		},
	}
}

func tracingFiltersTypeSchema() schema.SetNestedAttribute {
	return schema.SetNestedAttribute{
		Optional: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: tracingFiltersTypeSchemaAttributes(),
		},
	}
}

func tracingFiltersTypeSchemaAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"values": schema.SetAttribute{
			Required:    true,
			ElementType: types.StringType,
		},
		"operation": schema.StringAttribute{
			Optional: true,
			Computed: true,
			Default:  stringdefault.StaticString("IS"),
			Validators: []validator.String{
				stringvalidator.OneOf(validTracingFilterOperations...),
			},
			MarkdownDescription: fmt.Sprintf("Operation. Valid values: %q. 'IS' by default.", validTracingFilterOperations),
		},
	}
}

func tracingSpanFieldsFilterSchema() schema.SetNestedAttribute {
	return schema.SetNestedAttribute{
		Optional: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"key": schema.StringAttribute{
					Required: true,
				},
				"filter_type": schema.SingleNestedAttribute{
					Optional:   true,
					Attributes: tracingFiltersTypeSchemaAttributes(),
				},
			},
		},
	}
}

func metricFilterSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"promql": schema.StringAttribute{
				Required: true,
			},
		},
	}
}

func logsFilterSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Computed: true,
		PlanModifiers: []planmodifier.Object{
			objectplanmodifier.UseStateForUnknown(),
		},
		Attributes: map[string]schema.Attribute{
			"simple_filter": schema.SingleNestedAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]schema.Attribute{
					"lucene_query": schema.StringAttribute{
						Optional: true,
					},
					"label_filters": schema.SingleNestedAttribute{
						Optional: true,
						Computed: true,
						Default: objectdefault.StaticValue(types.ObjectValueMust(labelFiltersAttr(), map[string]attr.Value{
							"application_name": types.SetNull(types.ObjectType{AttrTypes: labelFilterTypesAttr()}),
							"subsystem_name":   types.SetNull(types.ObjectType{AttrTypes: labelFilterTypesAttr()}),
							"severities":       types.SetNull(types.StringType),
						})),
						Attributes: map[string]schema.Attribute{
							"application_name": logsAttributeFilterSchema(),
							"subsystem_name":   logsAttributeFilterSchema(),
							"severities": schema.SetAttribute{
								Optional:    true,
								ElementType: types.StringType,
								Validators: []validator.Set{
									setvalidator.ValueStringsAre(
										stringvalidator.OneOf(validLogSeverities...),
									),
								},
								MarkdownDescription: fmt.Sprintf("Severities. Valid values: %q.", validLogSeverities),
							},
						},
					},
				},
			},
		},
	}
}

func logsAttributeFilterSchema() schema.SetNestedAttribute {
	return schema.SetNestedAttribute{
		Optional: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"value": schema.StringAttribute{
					Required: true,
				},
				"operation": schema.StringAttribute{
					Optional: true,
					Computed: true,
					Default:  stringdefault.StaticString("IS"),
					Validators: []validator.String{
						stringvalidator.OneOf(validLogFilterOperationType...),
					},
					MarkdownDescription: fmt.Sprintf("Operation. Valid values: %q.'IS' by default.", validLogFilterOperationType),
				},
			},
		},
	}
}

func notificationPayloadFilterSchema() schema.SetAttribute {
	return schema.SetAttribute{
		Optional:    true,
		ElementType: types.StringType,
	}
}

func undetectedValuesManagementSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Computed: true,
		PlanModifiers: []planmodifier.Object{
			objectplanmodifier.UseStateForUnknown(),
		},
		Attributes: map[string]schema.Attribute{
			"trigger_undetected_values": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"auto_retire_timeframe": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(autoRetireTimeframeProtoToSchemaMap[cxsdk.AutoRetireTimeframeNeverOrUnspecified]),
				Validators: []validator.String{
					stringvalidator.OneOf(validAutoRetireTimeframes...),
				},
				MarkdownDescription: fmt.Sprintf("Auto retire timeframe. Valid values: %q.", validAutoRetireTimeframes),
			},
		},
		Default: objectdefault.StaticValue(types.ObjectValueMust(undetectedValuesManagementAttr(), map[string]attr.Value{
			"trigger_undetected_values": types.BoolValue(false),
			"auto_retire_timeframe":     types.StringValue(autoRetireTimeframeProtoToSchemaMap[cxsdk.AutoRetireTimeframeNeverOrUnspecified]),
		})),
	}
}

func (r *AlertResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *AlertResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *AlertResourceModel
	if diags := req.Plan.Get(ctx, &plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	alertProperties, diags := extractAlertProperties(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	createAlertRequest := &cxsdk.CreateAlertDefRequest{AlertDefProperties: alertProperties}
	log.Printf("[INFO] Creating new Alert: %s", protojson.Format(createAlertRequest))
	createResp, err := r.client.Create(ctx, createAlertRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err)
		resp.Diagnostics.AddError("Error creating Alert",
			formatRpcErrors(err, createAlertURL, protojson.Format(createAlertRequest)),
		)
		return
	}
	alert := createResp.GetAlertDef()
	log.Printf("[INFO] Submitted new alert: %s", protojson.Format(alert))

	plan, diags = flattenAlert(ctx, alert, &plan.Schedule)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	log.Printf("[INFO] Created Alert: %s", protojson.Format(alert))
	// Set state to fully populated data
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func extractAlertProperties(ctx context.Context, plan *AlertResourceModel) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	groupBy, diags := typeStringSliceToWrappedStringSlice(ctx, plan.GroupBy.Elements())
	if diags.HasError() {
		return nil, diags
	}
	incidentsSettings, diags := extractIncidentsSettings(ctx, plan.IncidentsSettings)
	if diags.HasError() {
		return nil, diags
	}
	notificationGroup, diags := extractNotificationGroup(ctx, plan.NotificationGroup)
	if diags.HasError() {
		return nil, diags
	}
	labels, diags := typeMapToStringMap(ctx, plan.Labels)

	if diags.HasError() {
		return nil, diags
	}
	alertProperties := &cxsdk.AlertDefProperties{
		Name:              typeStringToWrapperspbString(plan.Name),
		Description:       typeStringToWrapperspbString(plan.Description),
		Enabled:           typeBoolToWrapperspbBool(plan.Enabled),
		Priority:          alertPrioritySchemaToProtoMap[plan.Priority.ValueString()],
		GroupByKeys:       groupBy,
		IncidentsSettings: incidentsSettings,
		NotificationGroup: notificationGroup,
		EntityLabels:      labels,
	}

	alertProperties, diags = expandAlertsSchedule(ctx, alertProperties, plan.Schedule)
	if diags.HasError() {
		return nil, diags
	}

	alertProperties, diags = expandAlertsTypeDefinition(ctx, alertProperties, plan.TypeDefinition)
	if diags.HasError() {
		return nil, diags
	}

	return alertProperties, nil
}

func extractIncidentsSettings(ctx context.Context, incidentsSettingsObject types.Object) (*cxsdk.AlertDefIncidentSettings, diag.Diagnostics) {
	if incidentsSettingsObject.IsNull() || incidentsSettingsObject.IsUnknown() {
		return nil, nil
	}

	var incidentsSettingsModel IncidentsSettingsModel
	if diags := incidentsSettingsObject.As(ctx, &incidentsSettingsModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	incidentsSettings := &cxsdk.AlertDefIncidentSettings{
		NotifyOn: notifyOnSchemaToProtoMap[incidentsSettingsModel.NotifyOn.ValueString()],
	}

	incidentsSettings, diags := expandIncidentsSettingsByRetriggeringPeriod(ctx, incidentsSettings, incidentsSettingsModel.RetriggeringPeriod)
	if diags.HasError() {
		return nil, diags
	}

	return incidentsSettings, nil
}

func expandIncidentsSettingsByRetriggeringPeriod(ctx context.Context, incidentsSettings *cxsdk.AlertDefIncidentSettings, period types.Object) (*cxsdk.AlertDefIncidentSettings, diag.Diagnostics) {
	if period.IsNull() || period.IsUnknown() {
		return incidentsSettings, nil
	}

	var periodModel RetriggeringPeriodModel
	if diags := period.As(ctx, &periodModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if !(periodModel.Minutes.IsNull() || periodModel.Minutes.IsUnknown()) {
		incidentsSettings.RetriggeringPeriod = &cxsdk.AlertDefIncidentSettingsMinutes{
			Minutes: typeInt64ToWrappedUint32(periodModel.Minutes),
		}
	}

	return incidentsSettings, nil
}

func extractNotificationGroup(ctx context.Context, notificationGroupObject types.Object) (*cxsdk.AlertDefNotificationGroup, diag.Diagnostics) {
	if objIsNullOrUnknown(notificationGroupObject) {
		return nil, nil
	}

	var notificationGroupModel NotificationGroupModel
	if diags := notificationGroupObject.As(ctx, &notificationGroupModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	groupByFields, diags := typeStringSliceToWrappedStringSlice(ctx, notificationGroupModel.GroupByKeys.Elements())
	if diags.HasError() {
		return nil, diags
	}
	webhooks, diags := extractWebhooksSettings(ctx, notificationGroupModel.WebhooksSettings)
	if diags.HasError() {
		return nil, diags
	}
	notificationGroup := &cxsdk.AlertDefNotificationGroup{
		GroupByKeys: groupByFields,
		Webhooks:    webhooks,
	}

	return notificationGroup, nil
}

func expandNotificationTargetSettings(ctx context.Context, notificationGroupModel NotificationGroupModel, notificationGroup *cxsdk.AlertDefNotificationGroup) (*cxsdk.AlertDefNotificationGroup, diag.Diagnostics) {
	notificationGroup.Webhooks = []*cxsdk.AlertDefWebhooksSettings{}
	if webhooksSettings := notificationGroupModel.WebhooksSettings; !(webhooksSettings.IsNull() || webhooksSettings.IsUnknown()) {
		notifications, diags := extractWebhooksSettings(ctx, webhooksSettings)
		if diags.HasError() {
			return nil, diags
		}
		notificationGroup.Webhooks = notifications
	}

	return notificationGroup, nil
}

func extractWebhooksSettings(ctx context.Context, webhooksSettings types.Set) ([]*cxsdk.AlertDefWebhooksSettings, diag.Diagnostics) {
	if webhooksSettings.IsNull() || webhooksSettings.IsUnknown() {
		return nil, nil
	}

	var webhooksSettingsObject []types.Object
	diags := webhooksSettings.ElementsAs(ctx, &webhooksSettingsObject, true)
	if diags.HasError() {
		return nil, diags
	}
	var expandedWebhooksSettings []*cxsdk.AlertDefWebhooksSettings
	for _, ao := range webhooksSettingsObject {
		var webhooksSettingsModel WebhooksSettingsModel
		if dg := ao.As(ctx, &webhooksSettingsModel, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedAdvancedTargetSetting, expandDiags := extractAdvancedTargetSetting(ctx, webhooksSettingsModel)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedWebhooksSettings = append(expandedWebhooksSettings, expandedAdvancedTargetSetting)
	}

	if diags.HasError() {
		return nil, diags
	}

	return expandedWebhooksSettings, nil
}

func extractAdvancedTargetSetting(ctx context.Context, webhooksSettingsModel WebhooksSettingsModel) (*cxsdk.AlertDefWebhooksSettings, diag.Diagnostics) {
	notifyOn := notifyOnSchemaToProtoMap[webhooksSettingsModel.NotifyOn.ValueString()]
	advancedTargetSettings := &cxsdk.AlertDefWebhooksSettings{
		NotifyOn: &notifyOn,
	}
	advancedTargetSettings, diags := expandAlertNotificationByRetriggeringPeriod(ctx, advancedTargetSettings, webhooksSettingsModel.RetriggeringPeriod)
	if diags.HasError() {
		return nil, diags
	}

	if !webhooksSettingsModel.IntegrationID.IsNull() && !webhooksSettingsModel.IntegrationID.IsUnknown() {
		integrationId, diag := typeStringToWrapperspbUint32(webhooksSettingsModel.IntegrationID)
		if diag.HasError() {
			return nil, diag
		}
		advancedTargetSettings.Integration = &cxsdk.AlertDefIntegrationType{
			IntegrationType: &cxsdk.AlertDefIntegrationTypeIntegrationID{
				IntegrationId: integrationId,
			},
		}
	} else if !webhooksSettingsModel.Recipients.IsNull() && !webhooksSettingsModel.Recipients.IsUnknown() {
		emails, diags := typeStringSliceToWrappedStringSlice(ctx, webhooksSettingsModel.Recipients.Elements())
		if diags.HasError() {
			return nil, diags
		}
		advancedTargetSettings.Integration = &cxsdk.AlertDefIntegrationType{
			IntegrationType: &cxsdk.AlertDefIntegrationTypeRecipients{
				Recipients: &cxsdk.AlertDefRecipients{
					Emails: emails,
				},
			},
		}
	}

	return advancedTargetSettings, nil
}

func expandAlertNotificationByRetriggeringPeriod(ctx context.Context, alertNotification *cxsdk.AlertDefWebhooksSettings, period types.Object) (*cxsdk.AlertDefWebhooksSettings, diag.Diagnostics) {
	if objIsNullOrUnknown(period) {
		return alertNotification, nil
	}

	var periodModel RetriggeringPeriodModel
	if diags := period.As(ctx, &periodModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if !(periodModel.Minutes.IsNull() || periodModel.Minutes.IsUnknown()) {
		alertNotification.RetriggeringPeriod = &cxsdk.AlertDefWebhooksSettingsMinutes{
			Minutes: typeInt64ToWrappedUint32(periodModel.Minutes),
		}
	}

	return alertNotification, nil
}

func expandAlertsSchedule(ctx context.Context, alertProperties *cxsdk.AlertDefProperties, scheduleObject types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if objIsNullOrUnknown(scheduleObject) {
		return alertProperties, nil
	}

	var scheduleModel AlertScheduleModel
	if diags := scheduleObject.As(ctx, &scheduleModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	var diags diag.Diagnostics
	if activeOn := scheduleModel.ActiveOn; !objIsNullOrUnknown(activeOn) {
		alertProperties.Schedule, diags = expandActiveOnSchedule(ctx, activeOn)
	} else {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Schedule object is not valid", "Schedule object is not valid")}
	}

	if diags.HasError() {
		return nil, diags
	}

	return alertProperties, nil
}

func expandActiveOnSchedule(ctx context.Context, activeOnObject types.Object) (*cxsdk.AlertDefPropertiesActiveOn, diag.Diagnostics) {
	if objIsNullOrUnknown(activeOnObject) {
		return nil, nil
	}

	var activeOnModel ActiveOnModel
	if diags := activeOnObject.As(ctx, &activeOnModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	daysOfWeek, diags := extractDaysOfWeek(ctx, activeOnModel.DaysOfWeek)
	if diags.HasError() {
		return nil, diags
	}

	locationTime, e := time.Parse(OFFSET_FORMAT, activeOnModel.UtcOffset.ValueString())
	if e != nil {
		diags.AddError("Failed to parse start time", e.Error())
	}
	_, offset := locationTime.Zone()
	if e != nil {
		diags.AddError("Failed to parse start time", e.Error())
	}
	location := time.FixedZone("", offset)

	startTimeUtc, e := time.ParseInLocation(TIME_FORMAT, activeOnModel.StartTime.ValueString(), time.UTC)
	if e != nil {
		diags.AddError("Failed to parse start time", e.Error())
	}

	endTimeUtc, e := time.ParseInLocation(TIME_FORMAT, activeOnModel.EndTime.ValueString(), time.UTC)
	if e != nil {
		diags.AddError("Failed to parse end time", e.Error())
	}
	if endTimeUtc.Before(startTimeUtc) {
		diags.AddError("End time is before start time", "End time is before start time")
	}

	if diags.HasError() {
		return nil, diags
	}
	// shift the clock
	startTime := startTimeUtc.In(location)
	endTime := endTimeUtc.In(location)

	return &cxsdk.AlertDefScheduleActiveOn{
		ActiveOn: &cxsdk.AlertDefActivitySchedule{
			DayOfWeek: daysOfWeek,
			StartTime: &cxsdk.AlertTimeOfDay{
				Hours:   int32(startTime.UTC().Hour()),
				Minutes: int32(startTime.UTC().Minute()),
			},
			EndTime: &cxsdk.AlertTimeOfDay{
				Hours:   int32(endTime.UTC().Hour()),
				Minutes: int32(endTime.UTC().Minute()),
			},
		},
	}, nil
}

func extractDaysOfWeek(ctx context.Context, daysOfWeek types.Set) ([]cxsdk.AlertDayOfWeek, diag.Diagnostics) {
	var diags diag.Diagnostics
	daysOfWeekElements := daysOfWeek.Elements()
	result := make([]cxsdk.AlertDayOfWeek, 0, len(daysOfWeekElements))
	for _, v := range daysOfWeekElements {
		val, err := v.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Failed to convert value to Terraform", err.Error())
			continue
		}
		var str string

		if err = val.As(&str); err != nil {
			diags.AddError("Failed to convert value to string", err.Error())
			continue
		}
		result = append(result, daysOfWeekSchemaToProtoMap[str])
	}
	return result, diags
}

func expandAlertsTypeDefinition(ctx context.Context, alertProperties *cxsdk.AlertDefProperties, alertDefinition types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if objIsNullOrUnknown(alertDefinition) {
		return alertProperties, nil
	}

	var alertDefinitionModel AlertTypeDefinitionModel
	if diags := alertDefinition.As(ctx, &alertDefinitionModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	var diags diag.Diagnostics

	if logsImmediate := alertDefinitionModel.LogsImmediate; !objIsNullOrUnknown(logsImmediate) {
		// LogsImmediate
		alertProperties, diags = expandLogsImmediateAlertTypeDefinition(ctx, alertProperties, logsImmediate)
	} else if logsThreshold := alertDefinitionModel.LogsThreshold; !objIsNullOrUnknown(logsThreshold) {
		// LogsThreshold
		alertProperties, diags = expandLogsThresholdTypeDefinition(ctx, alertProperties, logsThreshold)
	} else if logsAnomaly := alertDefinitionModel.LogsAnomaly; !objIsNullOrUnknown(logsAnomaly) {
		// LogsAnomaly
		alertProperties, diags = expandLogsAnomalyAlertTypeDefinition(ctx, alertProperties, logsAnomaly)
	} else if logsRatioThreshold := alertDefinitionModel.LogsRatioThreshold; !objIsNullOrUnknown(logsRatioThreshold) {
		// LogsRatioThreshold
		alertProperties, diags = expandLogsRatioThresholdTypeDefinition(ctx, alertProperties, logsRatioThreshold)
	} else if logsNewValue := alertDefinitionModel.LogsNewValue; !objIsNullOrUnknown(logsNewValue) {
		// LogsNewValue
		alertProperties, diags = expandLogsNewValueAlertTypeDefinition(ctx, alertProperties, logsNewValue)
	} else if logsUniqueCount := alertDefinitionModel.LogsUniqueCount; !objIsNullOrUnknown(logsUniqueCount) {
		// LogsUniqueCount
		alertProperties, diags = expandLogsUniqueCountAlertTypeDefinition(ctx, alertProperties, logsUniqueCount)
	} else if logsTimeRelativeThreshold := alertDefinitionModel.LogsTimeRelativeThreshold; !objIsNullOrUnknown(logsTimeRelativeThreshold) {
		// LogsTimeRelativeThreshold
		alertProperties, diags = expandLogsTimeRelativeThresholdAlertTypeDefinition(ctx, alertProperties, logsTimeRelativeThreshold)
	} else if metricThreshold := alertDefinitionModel.MetricThreshold; !objIsNullOrUnknown(metricThreshold) {
		// MetricsThreshold
		alertProperties, diags = expandMetricThresholdAlertTypeDefinition(ctx, alertProperties, metricThreshold)
	} else if metricAnomaly := alertDefinitionModel.MetricAnomaly; !objIsNullOrUnknown(metricAnomaly) {
		// MetricsAnomaly
		alertProperties, diags = expandMetricAnomalyAlertTypeDefinition(ctx, alertProperties, metricAnomaly)
	} else if tracingImmediate := alertDefinitionModel.TracingImmediate; !objIsNullOrUnknown(tracingImmediate) {
		// TracingImmediate
		alertProperties, diags = expandTracingImmediateTypeDefinition(ctx, alertProperties, tracingImmediate)
	} else if tracingThreshold := alertDefinitionModel.TracingThreshold; !objIsNullOrUnknown(tracingThreshold) {
		// TracingThreshold
		alertProperties, diags = expandTracingThresholdTypeDefinition(ctx, alertProperties, tracingThreshold)
	} else if flow := alertDefinitionModel.Flow; !objIsNullOrUnknown(flow) {
		// Flow
		alertProperties, diags = expandFlowAlertTypeDefinition(ctx, alertProperties, flow)
	} else {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Alert Type Definition", "Alert Type Definition is not valid")}
	}

	if diags.HasError() {
		return nil, diags
	}

	return alertProperties, nil
}

func expandLogsImmediateAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, logsImmediateObject types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if objIsNullOrUnknown(logsImmediateObject) {
		return properties, nil
	}

	var immediateModel LogsImmediateModel
	if diags := logsImmediateObject.As(ctx, &immediateModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	logsFilter, diags := extractLogsFilter(ctx, immediateModel.LogsFilter)
	if diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := typeStringSliceToWrappedStringSlice(ctx, immediateModel.NotificationPayloadFilter.Elements())
	if diags.HasError() {
		return nil, diags
	}

	properties.TypeDefinition = &cxsdk.AlertDefPropertiesLogsImmediate{
		LogsImmediate: &cxsdk.LogsImmediateType{
			LogsFilter:                logsFilter,
			NotificationPayloadFilter: notificationPayloadFilter,
		},
	}
	properties.Type = cxsdk.AlertDefTypeLogsImmediateOrUnspecified
	return properties, nil
}

func extractLogsFilter(ctx context.Context, filter types.Object) (*cxsdk.LogsFilter, diag.Diagnostics) {
	if filter.IsNull() || filter.IsUnknown() {
		return nil, nil
	}

	var filterModel AlertsLogsFilterModel
	if diags := filter.As(ctx, &filterModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	logsFilter := &cxsdk.LogsFilter{}
	var diags diag.Diagnostics
	if !(filterModel.SimpleFilter.IsNull() || filterModel.SimpleFilter.IsUnknown()) {
		logsFilter.FilterType, diags = extractLuceneFilter(ctx, filterModel.SimpleFilter)
	}

	if diags.HasError() {
		return nil, diags
	}

	return logsFilter, nil
}

func extractLuceneFilter(ctx context.Context, luceneFilter types.Object) (*cxsdk.LogsFilterSimpleFilter, diag.Diagnostics) {
	if luceneFilter.IsNull() || luceneFilter.IsUnknown() {
		return nil, nil
	}

	var luceneFilterModel SimpleFilterModel
	if diags := luceneFilter.As(ctx, &luceneFilterModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	labelFilters, diags := extractLabelFilters(ctx, luceneFilterModel.LabelFilters)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.LogsFilterSimpleFilter{
		SimpleFilter: &cxsdk.SimpleFilter{
			LuceneQuery:  typeStringToWrapperspbString(luceneFilterModel.LuceneQuery),
			LabelFilters: labelFilters,
		},
	}, nil
}

func extractLabelFilters(ctx context.Context, filters types.Object) (*cxsdk.LabelFilters, diag.Diagnostics) {
	if filters.IsNull() || filters.IsUnknown() {
		return nil, nil
	}

	var filtersModel LabelFiltersModel
	if diags := filters.As(ctx, &filtersModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	applicationName, diags := extractLabelFilterTypes(ctx, filtersModel.ApplicationName)
	if diags.HasError() {
		return nil, diags
	}

	subsystemName, diags := extractLabelFilterTypes(ctx, filtersModel.SubsystemName)
	if diags.HasError() {
		return nil, diags
	}

	severities, diags := extractLogSeverities(ctx, filtersModel.Severities.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.LabelFilters{
		ApplicationName: applicationName,
		SubsystemName:   subsystemName,
		Severities:      severities,
	}, nil
}

func extractLabelFilterTypes(ctx context.Context, labelFilterTypes types.Set) ([]*cxsdk.LabelFilterType, diag.Diagnostics) {
	var labelFilterTypesObjects []types.Object
	diags := labelFilterTypes.ElementsAs(ctx, &labelFilterTypesObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	var expandedLabelFilterTypes []*cxsdk.LabelFilterType
	for _, lft := range labelFilterTypesObjects {
		var labelFilterTypeModel LabelFilterTypeModel
		if dg := lft.As(ctx, &labelFilterTypeModel, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedLabelFilterType := &cxsdk.LabelFilterType{
			Value:     typeStringToWrapperspbString(labelFilterTypeModel.Value),
			Operation: logFilterOperationTypeSchemaToProtoMap[labelFilterTypeModel.Operation.ValueString()],
		}
		expandedLabelFilterTypes = append(expandedLabelFilterTypes, expandedLabelFilterType)
	}

	if diags.HasError() {
		return nil, diags
	}

	return expandedLabelFilterTypes, nil
}

func extractLogSeverities(ctx context.Context, elements []attr.Value) ([]cxsdk.LogSeverity, diag.Diagnostics) {
	var diags diag.Diagnostics
	result := make([]cxsdk.LogSeverity, 0, len(elements))
	for _, v := range elements {
		val, err := v.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Failed to convert value to Terraform", err.Error())
			continue
		}
		var str string

		if err = val.As(&str); err != nil {
			diags.AddError("Failed to convert value to string", err.Error())
			continue
		}
		result = append(result, logSeveritySchemaToProtoMap[str])
	}
	return result, diags
}

func expandLogsThresholdTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, thresholdObject types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if objIsNullOrUnknown(thresholdObject) {
		return properties, nil
	}

	var thresholdModel LogsThresholdModel
	if diags := thresholdObject.As(ctx, &thresholdModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	logsFilter, diags := extractLogsFilter(ctx, thresholdModel.LogsFilter)
	if diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := typeStringSliceToWrappedStringSlice(ctx, thresholdModel.NotificationPayloadFilter.Elements())
	if diags.HasError() {
		return nil, diags
	}

	rules, diags := extractThresholdRules(ctx, thresholdModel.Rules)
	if diags.HasError() {
		return nil, diags
	}
	undetected, diags := extractUndetectedValuesManagement(ctx, thresholdModel.UndetectedValuesManagement)
	if diags.HasError() {
		return nil, diags
	}

	properties.TypeDefinition = &cxsdk.AlertDefPropertiesLogsThreshold{
		LogsThreshold: &cxsdk.LogsThresholdType{
			LogsFilter:                 logsFilter,
			Rules:                      rules,
			NotificationPayloadFilter:  notificationPayloadFilter,
			UndetectedValuesManagement: undetected,
		},
	}

	properties.Type = cxsdk.AlertDefTypeLogsThreshold
	return properties, nil
}

func extractThresholdRules(ctx context.Context, elements types.Set) ([]*cxsdk.LogsThresholdRule, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	rules := make([]*cxsdk.LogsThresholdRule, len(elements.Elements()))
	var objs []types.Object
	elements.ElementsAs(ctx, &objs, false)
	for i, r := range objs {
		var rule LogsThresholdRuleModel
		if dg := r.As(ctx, &rule, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		condition, dg := extractLogsThresholdCondition(ctx, rule.Condition)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}

		override, dg := extractAlertOverride(ctx, rule.Override)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}

		rules[i] = &cxsdk.LogsThresholdRule{
			Condition: condition,
			Override:  override,
		}
	}
	if diags.HasError() {
		return nil, diags
	}
	return rules, nil
}

func extractLogsThresholdCondition(ctx context.Context, condition types.Object) (*cxsdk.LogsThresholdCondition, diag.Diagnostics) {
	if condition.IsNull() || condition.IsUnknown() {
		return nil, nil
	}

	var conditionModel LogsThresholdConditionModel
	if diags := condition.As(ctx, &conditionModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	return &cxsdk.LogsThresholdCondition{
		Threshold: typeFloat64ToWrapperspbDouble(conditionModel.Threshold),
		TimeWindow: &cxsdk.LogsTimeWindow{
			Type: &cxsdk.LogsTimeWindowSpecificValue{
				LogsTimeWindowSpecificValue: logsTimeWindowValueSchemaToProtoMap[conditionModel.TimeWindow.ValueString()],
			},
		},
		ConditionType: logsThresholdConditionToProtoMap[conditionModel.ConditionType.ValueString()],
	}, nil
}

func extractUndetectedValuesManagement(ctx context.Context, management types.Object) (*cxsdk.UndetectedValuesManagement, diag.Diagnostics) {
	if objIsNullOrUnknown(management) {
		return nil, nil
	}
	var managementModel UndetectedValuesManagementModel
	if diags := management.As(ctx, &managementModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if (managementModel.AutoRetireTimeframe.IsNull() || managementModel.AutoRetireTimeframe.IsUnknown()) && (managementModel.TriggerUndetectedValues.IsNull() || managementModel.TriggerUndetectedValues.IsUnknown()) {
		return nil, nil
	}

	var autoRetireTimeframe *cxsdk.AutoRetireTimeframe
	if !(managementModel.AutoRetireTimeframe.IsNull() || managementModel.AutoRetireTimeframe.IsUnknown()) {
		autoRetireTimeframe = new(cxsdk.AutoRetireTimeframe)
		*autoRetireTimeframe = autoRetireTimeframeSchemaToProtoMap[managementModel.AutoRetireTimeframe.ValueString()]
	}

	return &cxsdk.UndetectedValuesManagement{
		TriggerUndetectedValues: typeBoolToWrapperspbBool(managementModel.TriggerUndetectedValues),
		AutoRetireTimeframe:     autoRetireTimeframe,
	}, nil
}

func expandLogsAnomalyAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, anomaly types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if objIsNullOrUnknown(anomaly) {
		return properties, nil
	}

	var anomalyModel LogsAnomalyModel
	if diags := anomaly.As(ctx, &anomalyModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	logsFilter, diags := extractLogsFilter(ctx, anomalyModel.LogsFilter)
	if diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := typeStringSliceToWrappedStringSlice(ctx, anomalyModel.NotificationPayloadFilter.Elements())
	if diags.HasError() {
		return nil, diags
	}

	rules, diags := extractAnomalyRules(ctx, anomalyModel.Rules)
	if diags.HasError() {
		return nil, diags
	}
	properties.TypeDefinition = &cxsdk.AlertDefPropertiesLogsAnomaly{
		LogsAnomaly: &cxsdk.LogsAnomalyType{
			LogsFilter:                logsFilter,
			Rules:                     rules,
			NotificationPayloadFilter: notificationPayloadFilter,
		},
	}

	properties.Type = cxsdk.AlertDefTypeLogsAnomaly
	return properties, nil
}

func extractAnomalyRules(ctx context.Context, elements types.Set) ([]*cxsdk.LogsAnomalyRule, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	rules := make([]*cxsdk.LogsAnomalyRule, len(elements.Elements()))
	var objs []types.Object
	elements.ElementsAs(ctx, &objs, false)
	for i, r := range objs {
		var rule LogsAnomalyRuleModel
		if dg := r.As(ctx, &rule, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}

		var condition LogsAnomalyConditionModel
		if dg := rule.Condition.As(ctx, &condition, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}

		rules[i] = &cxsdk.LogsAnomalyRule{
			Condition: &cxsdk.LogsAnomalyCondition{
				MinimumThreshold: typeFloat64ToWrapperspbDouble(condition.MinimumThreshold),
				TimeWindow: &cxsdk.LogsTimeWindow{
					Type: &cxsdk.LogsTimeWindowSpecificValue{
						LogsTimeWindowSpecificValue: logsTimeWindowValueSchemaToProtoMap[condition.TimeWindow.ValueString()],
					},
				},
				ConditionType: logsAnomalyConditionSchemaToProtoMap[condition.ConditionType.ValueString()],
			},
		}
	}
	if diags.HasError() {
		return nil, diags
	}
	return rules, nil
}

func expandLogsRatioThresholdTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, ratioThreshold types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if objIsNullOrUnknown(ratioThreshold) {
		return properties, nil
	}

	var ratioThresholdModel LogsRatioThresholdModel
	if diags := ratioThreshold.As(ctx, &ratioThresholdModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	numeratorLogsFilter, diags := extractLogsFilter(ctx, ratioThresholdModel.Numerator)
	if diags.HasError() {
		return nil, diags
	}

	denominatorLogsFilter, diags := extractLogsFilter(ctx, ratioThresholdModel.Denominator)
	if diags.HasError() {
		return nil, diags
	}

	rules, diags := extractRatioRules(ctx, ratioThresholdModel.Rules)
	if diags.HasError() {
		return nil, diags
	}
	notificationPayloadFilter, diags := typeStringSliceToWrappedStringSlice(ctx, ratioThresholdModel.NotificationPayloadFilter.Elements())
	if diags.HasError() {
		return nil, diags
	}

	properties.TypeDefinition = &cxsdk.AlertDefPropertiesLogsRatioThreshold{
		LogsRatioThreshold: &cxsdk.LogsRatioThresholdType{
			Numerator:                 numeratorLogsFilter,
			NumeratorAlias:            typeStringToWrapperspbString(ratioThresholdModel.NumeratorAlias),
			Denominator:               denominatorLogsFilter,
			DenominatorAlias:          typeStringToWrapperspbString(ratioThresholdModel.DenominatorAlias),
			Rules:                     rules,
			NotificationPayloadFilter: notificationPayloadFilter,
			GroupByFor:                logsRatioGroupByForSchemaToProtoMap[ratioThresholdModel.GroupByFor.ValueString()],
		},
	}
	properties.Type = cxsdk.AlertDefTypeLogsRatioThreshold
	return properties, nil
}

func extractRatioRules(ctx context.Context, elements types.Set) ([]*cxsdk.LogsRatioRules, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	rules := make([]*cxsdk.LogsRatioRules, len(elements.Elements()))
	var objs []types.Object
	elements.ElementsAs(ctx, &objs, false)
	for i, r := range objs {
		var rule LogsRatioThresholdRuleModel
		if dg := r.As(ctx, &rule, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		condition, dg := extractLogsRatioCondition(ctx, rule.Condition)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		override, dg := extractAlertOverride(ctx, rule.Override)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		rules[i] = &cxsdk.LogsRatioRules{
			Condition: condition,
			Override:  override,
		}
	}
	if diags.HasError() {
		return nil, diags
	}
	return rules, nil
}

func extractAlertOverride(ctx context.Context, override types.Object) (*cxsdk.AlertDefPriorityOverride, diag.Diagnostics) {
	if override.IsNull() || override.IsUnknown() {
		return nil, nil
	}

	var overrideModel AlertOverrideModel
	if diags := override.As(ctx, &overrideModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	return &cxsdk.AlertDefPriorityOverride{
		Priority: alertPrioritySchemaToProtoMap[overrideModel.Priority.ValueString()],
	}, nil
}

func extractLogsRatioCondition(ctx context.Context, condition types.Object) (*cxsdk.LogsRatioCondition, diag.Diagnostics) {
	if condition.IsNull() || condition.IsUnknown() {
		return nil, nil
	}

	var conditionModel LogsRatioConditionModel
	if diags := condition.As(ctx, &conditionModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	return &cxsdk.LogsRatioCondition{
		Threshold: typeFloat64ToWrapperspbDouble(conditionModel.Threshold),
		TimeWindow: &cxsdk.LogsRatioTimeWindow{
			Type: &cxsdk.LogsRatioTimeWindowSpecificValue{
				LogsRatioTimeWindowSpecificValue: logsRatioTimeWindowValueSchemaToProtoMap[conditionModel.TimeWindow.ValueString()],
			},
		},
		ConditionType: logsRatioConditionSchemaToProtoMap[conditionModel.ConditionType.ValueString()],
	}, nil
}

func expandLogsNewValueAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, newValue types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if newValue.IsNull() || newValue.IsUnknown() {
		return properties, nil
	}

	var newValueModel LogsNewValueModel
	if diags := newValue.As(ctx, &newValueModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	logsFilter, diags := extractLogsFilter(ctx, newValueModel.LogsFilter)
	if diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := typeStringSliceToWrappedStringSlice(ctx, newValueModel.NotificationPayloadFilter.Elements())
	if diags.HasError() {
		return nil, diags
	}

	rules, diags := extractNewValueRules(ctx, newValueModel.Rules)
	if diags.HasError() {
		return nil, diags
	}
	properties.TypeDefinition = &cxsdk.AlertDefPropertiesLogsNewValue{
		LogsNewValue: &cxsdk.LogsNewValueType{
			LogsFilter:                logsFilter,
			Rules:                     rules,
			NotificationPayloadFilter: notificationPayloadFilter,
		},
	}
	properties.Type = cxsdk.AlertDefTypeLogsNewValue
	return properties, nil
}

func extractNewValueRules(ctx context.Context, elements types.Set) ([]*cxsdk.LogsNewValueRule, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	rules := make([]*cxsdk.LogsNewValueRule, len(elements.Elements()))
	var objs []types.Object
	elements.ElementsAs(ctx, &objs, false)
	for i, r := range objs {
		var rule NewValueRuleModel
		if dg := r.As(ctx, &rule, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}

		condition, dg := extractNewValueCondition(ctx, rule.Condition)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}

		rules[i] = &cxsdk.LogsNewValueRule{
			Condition: condition,
		}
	}
	if diags.HasError() {
		return nil, diags
	}
	return rules, nil
}

func extractNewValueCondition(ctx context.Context, condition types.Object) (*cxsdk.LogsNewValueCondition, diag.Diagnostics) {
	if condition.IsNull() || condition.IsUnknown() {
		return nil, nil
	}

	var conditionModel NewValueConditionModel
	if diags := condition.As(ctx, &conditionModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	return &cxsdk.LogsNewValueCondition{
		KeypathToTrack: typeStringToWrapperspbString(conditionModel.KeypathToTrack),
		TimeWindow: &cxsdk.LogsNewValueTimeWindow{
			Type: &cxsdk.LogsNewValueTimeWindowSpecificValue{
				LogsNewValueTimeWindowSpecificValue: logsNewValueTimeWindowValueSchemaToProtoMap[conditionModel.TimeWindow.ValueString()],
			},
		},
	}, nil
}

func expandLogsUniqueCountAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, uniqueCount types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if objIsNullOrUnknown(uniqueCount) {
		return properties, nil
	}

	var uniqueCountModel LogsUniqueCountModel
	if diags := uniqueCount.As(ctx, &uniqueCountModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	logsFilter, diags := extractLogsFilter(ctx, uniqueCountModel.LogsFilter)
	if diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := typeStringSliceToWrappedStringSlice(ctx, uniqueCountModel.NotificationPayloadFilter.Elements())
	if diags.HasError() {
		return nil, diags
	}

	rules, diags := extractLogsUniqueCountRules(ctx, uniqueCountModel.Rules)
	if diags.HasError() {
		return nil, diags
	}

	properties.TypeDefinition = &cxsdk.AlertDefPropertiesLogsUniqueCount{
		LogsUniqueCount: &cxsdk.LogsUniqueCountType{
			LogsFilter:                  logsFilter,
			Rules:                       rules,
			NotificationPayloadFilter:   notificationPayloadFilter,
			MaxUniqueCountPerGroupByKey: typeInt64ToWrappedInt64(uniqueCountModel.MaxUniqueCountPerGroupByKey),
			UniqueCountKeypath:          typeStringToWrapperspbString(uniqueCountModel.UniqueCountKeypath),
		},
	}
	properties.Type = cxsdk.AlertDefTypeLogsUniqueCount
	return properties, nil
}

func extractLogsUniqueCountRules(ctx context.Context, elements types.Set) ([]*cxsdk.LogsUniqueCountRule, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	rules := make([]*cxsdk.LogsUniqueCountRule, len(elements.Elements()))
	var objs []types.Object
	elements.ElementsAs(ctx, &objs, false)
	for i, r := range objs {
		var rule LogsUniqueCountRuleModel
		if dg := r.As(ctx, &rule, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		condition, dgs := extractLogsUniqueCountCondition(ctx, rule.Condition)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		rules[i] = &cxsdk.LogsUniqueCountRule{
			Condition: condition,
		}
	}
	if diags.HasError() {
		return nil, diags
	}
	return rules, nil
}

func extractLogsUniqueCountCondition(ctx context.Context, condition types.Object) (*cxsdk.LogsUniqueCountCondition, diag.Diagnostics) {
	if objIsNullOrUnknown(condition) {
		return nil, nil
	}

	var conditionModel LogsUniqueCountConditionModel
	if diags := condition.As(ctx, &conditionModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	return &cxsdk.LogsUniqueCountCondition{
		MaxUniqueCount: typeInt64ToWrappedInt64(conditionModel.MaxUniqueCount),
		TimeWindow: &cxsdk.LogsUniqueValueTimeWindow{
			Type: &cxsdk.LogsUniqueValueTimeWindowSpecificValue{
				LogsUniqueValueTimeWindowSpecificValue: logsUniqueCountTimeWindowValueSchemaToProtoMap[conditionModel.TimeWindow.ValueString()],
			},
		},
	}, nil
}

func expandLogsTimeRelativeThresholdAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, relativeThreshold types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if objIsNullOrUnknown(relativeThreshold) {
		return properties, nil
	}

	var relativeThresholdModel LogsTimeRelativeThresholdModel
	if diags := relativeThreshold.As(ctx, &relativeThresholdModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	logsFilter, diags := extractLogsFilter(ctx, relativeThresholdModel.LogsFilter)
	if diags.HasError() {
		return nil, diags
	}

	undetected, diags := extractUndetectedValuesManagement(ctx, relativeThresholdModel.UndetectedValuesManagement)
	if diags.HasError() {
		return nil, diags
	}
	notificationPayloadFilter, diags := typeStringSliceToWrappedStringSlice(ctx, relativeThresholdModel.NotificationPayloadFilter.Elements())
	if diags.HasError() {
		return nil, diags
	}

	rules, diags := extractTimeRelativeThresholdRules(ctx, relativeThresholdModel.Rules)
	if diags.HasError() {
		return nil, diags
	}
	properties.TypeDefinition = &cxsdk.AlertDefPropertiesLogsTimeRelativeThreshold{
		LogsTimeRelativeThreshold: &cxsdk.LogsTimeRelativeThresholdType{
			LogsFilter:                 logsFilter,
			Rules:                      rules,
			NotificationPayloadFilter:  notificationPayloadFilter,
			UndetectedValuesManagement: undetected,
		},
	}
	properties.Type = cxsdk.AlertDefTypeLogsTimeRelativeThreshold
	return properties, nil
}

func extractTimeRelativeThresholdRules(ctx context.Context, elements types.Set) ([]*cxsdk.LogsTimeRelativeRule, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	rules := make([]*cxsdk.LogsTimeRelativeRule, len(elements.Elements()))
	var objs []types.Object
	elements.ElementsAs(ctx, &objs, false)
	for i, r := range objs {
		var rule LogsTimeRelativeRuleModel
		if dg := r.As(ctx, &rule, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}

		var condition LogsTimeRelativeConditionModel
		if dg := rule.Condition.As(ctx, &condition, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}

		override, dgs := extractAlertOverride(ctx, rule.Override)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}

		rules[i] = &cxsdk.LogsTimeRelativeRule{
			Condition: &cxsdk.LogsTimeRelativeCondition{
				Threshold:     typeFloat64ToWrapperspbDouble(condition.Threshold),
				ComparedTo:    logsTimeRelativeComparedToSchemaToProtoMap[condition.ComparedTo.ValueString()],
				ConditionType: logsTimeRelativeConditionToProtoMap[condition.ConditionType.ValueString()],
			},
			Override: override,
		}
	}
	if diags.HasError() {
		return nil, diags
	}
	return rules, nil
}

func expandMetricThresholdAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, metricThreshold types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if objIsNullOrUnknown(metricThreshold) {
		return properties, nil
	}

	var metricThresholdModel MetricThresholdModel
	if diags := metricThreshold.As(ctx, &metricThresholdModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	metricFilter, diags := extractMetricFilter(ctx, metricThresholdModel.MetricFilter)
	if diags.HasError() {
		return nil, diags
	}

	rules, diags := extractMetricThresholdRules(ctx, metricThresholdModel.Rules)
	if diags.HasError() {
		return nil, diags
	}

	missingValues, diags := extractMetricThresholdMissingValues(ctx, metricThresholdModel.MissingValues)
	if diags.HasError() {
		return nil, diags
	}

	undetected, diags := extractUndetectedValuesManagement(ctx, metricThresholdModel.UndetectedValuesManagement)
	if diags.HasError() {
		return nil, diags
	}
	properties.TypeDefinition = &cxsdk.AlertDefPropertiesMetricThreshold{
		MetricThreshold: &cxsdk.MetricThresholdType{
			MetricFilter:               metricFilter,
			Rules:                      rules,
			MissingValues:              missingValues,
			UndetectedValuesManagement: undetected,
		},
	}
	properties.Type = cxsdk.AlertDefTypeMetricThreshold

	return properties, nil
}

func extractMetricThresholdMissingValues(ctx context.Context, values types.Object) (*cxsdk.MetricMissingValues, diag.Diagnostics) {
	if objIsNullOrUnknown(values) {
		return nil, nil
	}

	var valuesModel MissingValuesModel
	if diags := values.As(ctx, &valuesModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if replaceWithZero := valuesModel.ReplaceWithZero; !(replaceWithZero.IsNull() || replaceWithZero.IsUnknown()) {
		return &cxsdk.MetricMissingValues{
			MissingValues: &cxsdk.MetricMissingValuesReplaceWithZero{
				ReplaceWithZero: typeBoolToWrapperspbBool(replaceWithZero),
			},
		}, nil
	} else if retainMissingValues := valuesModel.MinNonNullValuesPct; !(retainMissingValues.IsNull() || retainMissingValues.IsUnknown()) {
		return &cxsdk.MetricMissingValues{
			MissingValues: &cxsdk.MetricMissingValuesMinNonNullValuesPct{
				MinNonNullValuesPct: typeInt64ToWrappedUint32(retainMissingValues),
			},
		}, nil
	} else {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Metric Missing Values", "Metric Missing Values is not valid")}
	}
}

func extractMetricThresholdRules(ctx context.Context, elements types.Set) ([]*cxsdk.MetricThresholdRule, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	rules := make([]*cxsdk.MetricThresholdRule, len(elements.Elements()))
	var objs []types.Object
	elements.ElementsAs(ctx, &objs, false)
	for i, r := range objs {
		var rule MetricThresholdRuleModel
		if dg := r.As(ctx, &rule, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}

		var condition MetricThresholdConditionModel
		if dg := rule.Condition.As(ctx, &condition, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}

		override, dg := extractAlertOverride(ctx, rule.Override)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}

		rules[i] = &cxsdk.MetricThresholdRule{
			Condition: &cxsdk.MetricThresholdCondition{
				Threshold:  typeFloat64ToWrapperspbDouble(condition.Threshold),
				ForOverPct: typeInt64ToWrappedUint32(condition.ForOverPct),
				OfTheLast: &cxsdk.MetricTimeWindow{
					Type: &cxsdk.MetricTimeWindowSpecificValue{
						MetricTimeWindowSpecificValue: metricTimeWindowValueSchemaToProtoMap[condition.OfTheLast.ValueString()],
					},
				},
				ConditionType: metricsThresholdConditionToProtoMap[condition.ConditionType.ValueString()],
			},
			Override: override,
		}
	}
	if diags.HasError() {
		return nil, diags
	}
	return rules, nil
}

func extractMetricFilter(ctx context.Context, filter types.Object) (*cxsdk.MetricFilter, diag.Diagnostics) {
	if objIsNullOrUnknown(filter) {
		return nil, nil
	}

	var filterModel MetricFilterModel
	if diags := filter.As(ctx, &filterModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if promql := filterModel.Promql; !(promql.IsNull() || promql.IsUnknown()) {
		return &cxsdk.MetricFilter{
			Type: &cxsdk.MetricFilterPromql{
				Promql: typeStringToWrapperspbString(promql),
			},
		}, nil
	}

	return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Metric Filter", "Metric Filter is not valid")}
}

func expandTracingImmediateTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, tracingImmediate types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if objIsNullOrUnknown(tracingImmediate) {
		return properties, nil
	}

	var tracingImmediateModel TracingImmediateModel
	if diags := tracingImmediate.As(ctx, &tracingImmediateModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	tracingQuery, diags := expandTracingFilters(ctx, tracingImmediateModel.TracingFilter)
	if diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := typeStringSliceToWrappedStringSlice(ctx, tracingImmediateModel.NotificationPayloadFilter.Elements())
	if diags.HasError() {
		return nil, diags
	}
	properties.TypeDefinition = &cxsdk.AlertDefPropertiesTracingImmediate{
		TracingImmediate: &cxsdk.TracingImmediateType{
			TracingFilter: &cxsdk.TracingFilter{
				FilterType: tracingQuery,
			},
			NotificationPayloadFilter: notificationPayloadFilter,
		},
	}
	properties.Type = cxsdk.AlertDefTypeTracingImmediate

	return properties, nil
}

func expandTracingThresholdTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, tracingThreshold types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if objIsNullOrUnknown(tracingThreshold) {
		return properties, nil
	}

	var tracingThresholdModel TracingThresholdModel
	if diags := tracingThreshold.As(ctx, &tracingThresholdModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	tracingQuery, diags := expandTracingFilters(ctx, tracingThresholdModel.TracingFilter)
	if diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := typeStringSliceToWrappedStringSlice(ctx, tracingThresholdModel.NotificationPayloadFilter.Elements())
	if diags.HasError() {
		return nil, diags
	}

	rules, diags := extractTracingThresholdRules(ctx, tracingThresholdModel.Rules)
	if diags.HasError() {
		return nil, diags
	}

	properties.TypeDefinition = &cxsdk.AlertDefPropertiesTracingThreshold{
		TracingThreshold: &cxsdk.TracingThresholdType{
			TracingFilter: &cxsdk.TracingFilter{
				FilterType: tracingQuery,
			},
			NotificationPayloadFilter: notificationPayloadFilter,
			Rules:                     rules,
		},
	}
	properties.Type = cxsdk.AlertDefTypeTracingThreshold

	return properties, nil
}

func extractTracingThresholdRules(ctx context.Context, elements types.Set) ([]*cxsdk.TracingThresholdRule, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	rules := make([]*cxsdk.TracingThresholdRule, len(elements.Elements()))
	var objs []types.Object
	elements.ElementsAs(ctx, &objs, false)
	for i, r := range objs {
		var rule TracingThresholdRuleModel
		if dg := r.As(ctx, &rule, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}

		var condition TracingThresholdConditionModel
		if dg := rule.Condition.As(ctx, &condition, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}

		rules[i] = &cxsdk.TracingThresholdRule{
			Condition: &cxsdk.TracingThresholdCondition{
				SpanAmount: typeFloat64ToWrapperspbDouble(condition.SpanAmount),
				TimeWindow: &cxsdk.TracingTimeWindow{
					Type: &cxsdk.TracingTimeWindowSpecificValue{
						TracingTimeWindowValue: tracingTimeWindowSchemaToProtoMap[condition.TimeWindow.ValueString()],
					},
				},
				ConditionType: cxsdk.TracingThresholdConditionTypeMoreThanOrUnspecified,
			},
		}
	}
	if diags.HasError() {
		return nil, diags
	}
	return rules, nil
}

func expandTracingFilters(ctx context.Context, query types.Object) (*cxsdk.TracingFilterSimpleFilter, diag.Diagnostics) {
	if objIsNullOrUnknown(query) {
		return nil, nil
	}
	var labelFilterModel TracingFilterModel
	if diags := query.As(ctx, &labelFilterModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	var filtersModel TracingLabelFiltersModel
	if diags := labelFilterModel.TracingLabelFilters.As(ctx, &filtersModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	applicationName, diags := extractTracingLabelFilters(ctx, filtersModel.ApplicationName)
	if diags.HasError() {
		return nil, diags
	}

	subsystemName, diags := extractTracingLabelFilters(ctx, filtersModel.SubsystemName)
	if diags.HasError() {
		return nil, diags
	}

	operationName, diags := extractTracingLabelFilters(ctx, filtersModel.OperationName)
	if diags.HasError() {
		return nil, diags
	}

	serviceName, diags := extractTracingLabelFilters(ctx, filtersModel.ServiceName)
	if diags.HasError() {
		return nil, diags
	}

	spanFields, diags := extractTracingSpanFieldsFilterType(ctx, filtersModel.SpanFields)
	if diags.HasError() {
		return nil, diags
	}

	filter := &cxsdk.TracingFilterSimpleFilter{
		SimpleFilter: &cxsdk.TracingSimpleFilter{
			TracingLabelFilters: &cxsdk.TracingLabelFilters{
				ApplicationName: applicationName,
				SubsystemName:   subsystemName,
				ServiceName:     serviceName,
				OperationName:   operationName,
				SpanFields:      spanFields,
			},
			LatencyThresholdMs: numberTypeToWrapperspbUInt64(labelFilterModel.LatencyThresholdMs),
		},
	}

	return filter, nil
}

func extractTracingLabelFilters(ctx context.Context, tracingLabelFilters types.Set) ([]*cxsdk.TracingFilterType, diag.Diagnostics) {
	if tracingLabelFilters.IsNull() || tracingLabelFilters.IsUnknown() {
		return nil, nil
	}

	var filtersObjects []types.Object
	diags := tracingLabelFilters.ElementsAs(ctx, &filtersObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	var filters []*cxsdk.TracingFilterType
	for _, filtersObject := range filtersObjects {
		filter, dgs := extractTracingLabelFilter(ctx, filtersObject)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		filters = append(filters, filter)
	}
	if diags.HasError() {
		return nil, diags
	}

	return filters, nil
}

func extractTracingLabelFilter(ctx context.Context, filterModelObject types.Object) (*cxsdk.TracingFilterType, diag.Diagnostics) {
	var filterModel TracingFilterTypeModel
	if diags := filterModelObject.As(ctx, &filterModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	values, diags := typeStringSliceToWrappedStringSlice(ctx, filterModel.Values.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.TracingFilterType{
		Values:    values,
		Operation: tracingFilterOperationSchemaToProtoMap[filterModel.Operation.ValueString()],
	}, nil
}

func extractTracingSpanFieldsFilterType(ctx context.Context, spanFields types.Set) ([]*cxsdk.TracingSpanFieldsFilterType, diag.Diagnostics) {
	if spanFields.IsNull() || spanFields.IsUnknown() {
		return nil, nil
	}

	var spanFieldsObjects []types.Object
	_ = spanFields.ElementsAs(ctx, &spanFieldsObjects, true)
	var filters []*cxsdk.TracingSpanFieldsFilterType
	for _, element := range spanFieldsObjects {
		var filterModel TracingSpanFieldsFilterModel
		if diags := element.As(ctx, &filterModel, basetypes.ObjectAsOptions{}); diags.HasError() {
			return nil, diags
		}

		filterType, diags := extractTracingLabelFilter(ctx, filterModel.FilterType)
		if diags.HasError() {
			return nil, diags
		}

		filters = append(filters, &cxsdk.TracingSpanFieldsFilterType{
			Key:        typeStringToWrapperspbString(filterModel.Key),
			FilterType: filterType,
		})
	}

	return filters, nil
}

func expandMetricAnomalyAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, metricAnomaly types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if objIsNullOrUnknown(metricAnomaly) {
		return properties, nil
	}

	var metricAnomalyModel MetricAnomalyModel
	if diags := metricAnomaly.As(ctx, &metricAnomalyModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	metricFilter, diags := extractMetricFilter(ctx, metricAnomalyModel.MetricFilter)
	if diags.HasError() {
		return nil, diags
	}

	rules, diags := extractMetricAnomalyRules(ctx, metricAnomalyModel.Rules)
	if diags.HasError() {
		return nil, diags
	}

	properties.TypeDefinition = &cxsdk.AlertDefPropertiesMetricAnomaly{
		MetricAnomaly: &cxsdk.MetricAnomalyType{
			MetricFilter: metricFilter,
			Rules:        rules,
		},
	}
	properties.Type = cxsdk.AlertDefTypeMetricAnomaly

	return properties, nil
}

func extractMetricAnomalyRules(ctx context.Context, elements types.Set) ([]*cxsdk.MetricAnomalyRule, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	rules := make([]*cxsdk.MetricAnomalyRule, len(elements.Elements()))
	var objs []types.Object
	elements.ElementsAs(ctx, &objs, false)
	for i, r := range objs {
		var rule MetricAnomalyRuleModel
		if dg := r.As(ctx, &rule, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}

		var condition MetricAnomalyConditionModel
		if dg := rule.Condition.As(ctx, &condition, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}

		rules[i] = &cxsdk.MetricAnomalyRule{
			Condition: &cxsdk.MetricAnomalyCondition{
				Threshold:  typeFloat64ToWrapperspbDouble(condition.Threshold),
				ForOverPct: typeInt64ToWrappedUint32(condition.ForOverPct),
				OfTheLast: &cxsdk.MetricTimeWindow{
					Type: &cxsdk.MetricTimeWindowSpecificValue{
						MetricTimeWindowSpecificValue: metricTimeWindowValueSchemaToProtoMap[condition.OfTheLast.ValueString()],
					},
				},
				ConditionType:       metricAnomalyConditionToProtoMap[condition.ConditionType.ValueString()],
				MinNonNullValuesPct: typeInt64ToWrappedUint32(condition.MinNonNullValuesPct),
			},
		}
	}
	if diags.HasError() {
		return nil, diags
	}
	return rules, nil
}

func expandFlowAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, flow types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if objIsNullOrUnknown(flow) {
		return properties, nil
	}

	var flowModel FlowModel
	if diags := flow.As(ctx, &flowModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	stages, diags := extractFlowStages(ctx, flowModel.Stages)
	if diags.HasError() {
		return nil, diags
	}

	properties.TypeDefinition = &cxsdk.AlertDefPropertiesFlow{
		Flow: &cxsdk.FlowType{
			Stages:             stages,
			EnforceSuppression: typeBoolToWrapperspbBool(flowModel.EnforceSuppression),
		},
	}
	properties.Type = cxsdk.AlertDefTypeFlow
	return properties, nil
}

func extractFlowStages(ctx context.Context, stages types.List) ([]*cxsdk.FlowStages, diag.Diagnostics) {
	if stages.IsNull() || stages.IsUnknown() {
		return nil, nil
	}

	var stagesObjects []types.Object
	diags := stages.ElementsAs(ctx, &stagesObjects, true)
	if diags.HasError() {
		return nil, diags
	}

	var flowStages []*cxsdk.FlowStages
	for _, stageObject := range stagesObjects {
		stage, diags := extractFlowStage(ctx, stageObject)
		if diags.HasError() {
			return nil, diags
		}
		flowStages = append(flowStages, stage)
	}

	return flowStages, nil
}

func extractFlowStage(ctx context.Context, object types.Object) (*cxsdk.FlowStages, diag.Diagnostics) {
	var stageModel FlowStageModel
	if diags := object.As(ctx, &stageModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	flowStage := &cxsdk.FlowStages{
		TimeframeMs:   typeInt64ToWrappedInt64(stageModel.TimeframeMs),
		TimeframeType: flowStageTimeFrameTypeSchemaToProtoMap[stageModel.TimeframeType.ValueString()],
	}

	if flowStagesGroups := stageModel.FlowStagesGroups; !(flowStagesGroups.IsNull() || flowStagesGroups.IsUnknown()) {
		flowStages, diags := extractFlowStagesGroups(ctx, flowStagesGroups)
		if diags.HasError() {
			return nil, diags
		}
		flowStage.FlowStages = flowStages
	}

	return flowStage, nil
}

func extractFlowStagesGroups(ctx context.Context, groups types.List) (*cxsdk.FlowStagesGroups, diag.Diagnostics) {
	if groups.IsNull() || groups.IsUnknown() {
		return nil, nil
	}

	var groupsObjects []types.Object
	diags := groups.ElementsAs(ctx, &groupsObjects, true)
	if diags.HasError() {
		return nil, diags
	}

	var flowStagesGroups []*cxsdk.FlowStagesGroup
	for _, groupObject := range groupsObjects {
		group, diags := extractFlowStagesGroup(ctx, groupObject)
		if diags.HasError() {
			return nil, diags
		}
		flowStagesGroups = append(flowStagesGroups, group)
	}

	return &cxsdk.FlowStagesGroups{
		FlowStagesGroups: &cxsdk.FlowStagesGroupsValue{
			Groups: flowStagesGroups,
		}}, nil

}

func extractFlowStagesGroup(ctx context.Context, object types.Object) (*cxsdk.FlowStagesGroup, diag.Diagnostics) {
	var groupModel FlowStagesGroupModel
	if diags := object.As(ctx, &groupModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	alertDefs, diags := extractAlertDefs(ctx, groupModel.AlertDefs)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.FlowStagesGroup{
		AlertDefs: alertDefs,
		NextOp:    flowStagesGroupNextOpSchemaToProtoMap[groupModel.NextOp.ValueString()],
		AlertsOp:  flowStagesGroupAlertsOpSchemaToProtoMap[groupModel.AlertsOp.ValueString()],
	}, nil

}

func extractAlertDefs(ctx context.Context, defs types.Set) ([]*cxsdk.FlowStagesGroupsAlertDefs, diag.Diagnostics) {
	if defs.IsNull() || defs.IsUnknown() {
		return nil, nil
	}

	var defsObjects []types.Object
	diags := defs.ElementsAs(ctx, &defsObjects, true)
	if diags.HasError() {
		return nil, diags
	}

	var alertDefs []*cxsdk.FlowStagesGroupsAlertDefs
	for _, defObject := range defsObjects {
		def, diags := extractAlertDef(ctx, defObject)
		if diags.HasError() {
			return nil, diags
		}
		alertDefs = append(alertDefs, def)
	}

	return alertDefs, nil

}

func extractAlertDef(ctx context.Context, def types.Object) (*cxsdk.FlowStagesGroupsAlertDefs, diag.Diagnostics) {
	var defModel FlowStagesGroupsAlertDefsModel
	if diags := def.As(ctx, &defModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	return &cxsdk.FlowStagesGroupsAlertDefs{
		Id:  typeStringToWrapperspbString(defModel.Id),
		Not: typeBoolToWrapperspbBool(defModel.Not),
	}, nil

}

func (r *AlertResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *AlertResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed Alert value from Coralogix
	id := state.ID.ValueString()
	log.Printf("[INFO] Reading Alert: %s", id)
	getAlertReq := &cxsdk.GetAlertDefRequest{Id: wrapperspb.String(id)}
	getAlertResp, err := r.client.Get(ctx, getAlertReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Alert %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Alert",
				formatRpcErrors(err, getAlertURL, protojson.Format(getAlertReq)),
			)
		}
		return
	}
	alert := getAlertResp.GetAlertDef()
	log.Printf("[INFO] Received Alert: %s", protojson.Format(alert))

	state, diags = flattenAlert(ctx, alert, &state.Schedule)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func flattenAlert(ctx context.Context, alert *cxsdk.AlertDef, currentSchedule *types.Object) (*AlertResourceModel, diag.Diagnostics) {
	alertProperties := alert.GetAlertDefProperties()

	alertSchedule, diags := flattenAlertSchedule(ctx, alertProperties, currentSchedule)
	if diags.HasError() {
		return nil, diags
	}
	alertTypeDefinition, diags := flattenAlertTypeDefinition(ctx, alertProperties)
	if diags.HasError() {
		return nil, diags
	}
	incidentsSettings, diags := flattenIncidentsSettings(ctx, alertProperties.GetIncidentsSettings())
	if diags.HasError() {
		return nil, diags
	}
	notificationGroup, diags := flattenNotificationGroup(ctx, alertProperties.GetNotificationGroup())
	if diags.HasError() {
		return nil, diags
	}
	labels, diags := types.MapValueFrom(ctx, types.StringType, alertProperties.GetEntityLabels())
	if diags.HasError() {
		return nil, diags
	}
	return &AlertResourceModel{
		ID:                wrapperspbStringToTypeString(alert.GetId()),
		Name:              wrapperspbStringToTypeString(alertProperties.GetName()),
		Description:       wrapperspbStringToTypeString(alertProperties.GetDescription()),
		Enabled:           wrapperspbBoolToTypeBool(alertProperties.GetEnabled()),
		Priority:          types.StringValue(alertPriorityProtoToSchemaMap[alertProperties.GetPriority()]),
		Schedule:          alertSchedule,
		TypeDefinition:    alertTypeDefinition,
		GroupBy:           wrappedStringSliceToTypeStringSet(alertProperties.GetGroupByKeys()),
		IncidentsSettings: incidentsSettings,
		NotificationGroup: notificationGroup,
		Labels:            labels,
		PhantomMode:       wrapperspbBoolToTypeBool(alertProperties.GetPhantomMode()),
		Deleted:           wrapperspbBoolToTypeBool(alertProperties.GetDeleted()),
	}, nil
}

func flattenNotificationGroup(ctx context.Context, notificationGroup *cxsdk.AlertDefNotificationGroup) (types.Object, diag.Diagnostics) {
	if notificationGroup == nil {
		return types.ObjectNull(notificationGroupAttr()), nil
	}

	webhooksSettings, diags := flattenAdvancedTargetSettings(ctx, notificationGroup.GetWebhooks())
	if diags.HasError() {
		return types.ObjectNull(notificationGroupAttr()), diags
	}

	notificationGroupModel := NotificationGroupModel{
		GroupByKeys:      wrappedStringSliceToTypeStringSet(notificationGroup.GetGroupByKeys()),
		WebhooksSettings: webhooksSettings,
	}

	return types.ObjectValueFrom(ctx, notificationGroupAttr(), notificationGroupModel)
}

func flattenAdvancedTargetSettings(ctx context.Context, webhooksSettings []*cxsdk.AlertDefWebhooksSettings) (types.Set, diag.Diagnostics) {
	if webhooksSettings == nil {
		return types.SetNull(types.ObjectType{AttrTypes: webhooksSettingsAttr()}), nil
	}

	var notificationsModel []*WebhooksSettingsModel
	var diags diag.Diagnostics
	for _, notification := range webhooksSettings {
		retriggeringPeriod, dgs := flattenRetriggeringPeriod(ctx, notification)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		notificationModel := WebhooksSettingsModel{
			NotifyOn:           types.StringValue(notifyOnProtoToSchemaMap[notification.GetNotifyOn()]),
			RetriggeringPeriod: retriggeringPeriod,
			IntegrationID:      types.StringNull(),
			Recipients:         types.SetNull(types.StringType),
		}
		switch integrationType := notification.GetIntegration(); integrationType.GetIntegrationType().(type) {
		case *cxsdk.AlertDefIntegrationTypeIntegrationID:
			notificationModel.IntegrationID = types.StringValue(strconv.Itoa(int(integrationType.GetIntegrationId().GetValue())))
		case *cxsdk.AlertDefIntegrationTypeRecipients:
			notificationModel.Recipients = wrappedStringSliceToTypeStringSet(integrationType.GetRecipients().GetEmails())
		}
		notificationsModel = append(notificationsModel, &notificationModel)
	}

	if diags.HasError() {
		return types.SetNull(types.ObjectType{AttrTypes: webhooksSettingsAttr()}), diags
	}

	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: webhooksSettingsAttr()}, notificationsModel)
}

func flattenRetriggeringPeriod(ctx context.Context, notifications *cxsdk.AlertDefWebhooksSettings) (types.Object, diag.Diagnostics) {
	switch notificationPeriodType := notifications.RetriggeringPeriod.(type) {
	case *cxsdk.AlertDefWebhooksSettingsMinutes:
		return types.ObjectValueFrom(ctx, retriggeringPeriodAttr(), RetriggeringPeriodModel{
			Minutes: wrapperspbUint32ToTypeInt64(notificationPeriodType.Minutes),
		})
	case nil:
		return types.ObjectNull(retriggeringPeriodAttr()), nil
	default:
		return types.ObjectNull(retriggeringPeriodAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Retriggering Period", fmt.Sprintf("Retriggering Period %v is not supported", notificationPeriodType))}
	}
}

func flattenIncidentsSettings(ctx context.Context, incidentsSettings *cxsdk.AlertDefIncidentSettings) (types.Object, diag.Diagnostics) {
	if incidentsSettings == nil {
		return types.ObjectNull(incidentsSettingsAttr()), nil
	}

	retriggeringPeriod, diags := flattenIncidentsSettingsByRetriggeringPeriod(ctx, incidentsSettings)
	if diags.HasError() {
		return types.ObjectNull(incidentsSettingsAttr()), diags
	}

	incidentsSettingsModel := IncidentsSettingsModel{
		NotifyOn:           types.StringValue(notifyOnProtoToSchemaMap[incidentsSettings.GetNotifyOn()]),
		RetriggeringPeriod: retriggeringPeriod,
	}
	return types.ObjectValueFrom(ctx, incidentsSettingsAttr(), incidentsSettingsModel)
}

func flattenIncidentsSettingsByRetriggeringPeriod(ctx context.Context, settings *cxsdk.AlertDefIncidentSettings) (types.Object, diag.Diagnostics) {
	if settings.RetriggeringPeriod == nil {
		return types.ObjectNull(retriggeringPeriodAttr()), nil
	}

	var periodModel RetriggeringPeriodModel
	switch period := settings.RetriggeringPeriod.(type) {
	case *cxsdk.AlertDefIncidentSettingsMinutes:
		periodModel.Minutes = wrapperspbUint32ToTypeInt64(period.Minutes)
	default:
		return types.ObjectNull(retriggeringPeriodAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Retriggering Period", fmt.Sprintf("Retriggering Period %v is not supported", period))}
	}

	return types.ObjectValueFrom(ctx, retriggeringPeriodAttr(), periodModel)
}

func flattenAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties) (types.Object, diag.Diagnostics) {
	if properties.TypeDefinition == nil {
		return types.ObjectNull(alertTypeDefinitionAttr()), nil
	}

	alertTypeDefinitionModel := AlertTypeDefinitionModel{
		LogsImmediate:             types.ObjectNull(logsImmediateAttr()),
		LogsThreshold:             types.ObjectNull(logsThresholdAttr()),
		LogsAnomaly:               types.ObjectNull(logsAnomalyAttr()),
		LogsRatioThreshold:        types.ObjectNull(logsRatioThresholdAttr()),
		LogsNewValue:              types.ObjectNull(logsNewValueAttr()),
		LogsUniqueCount:           types.ObjectNull(logsUniqueCountAttr()),
		LogsTimeRelativeThreshold: types.ObjectNull(logsTimeRelativeAttr()),
		MetricThreshold:           types.ObjectNull(metricThresholdAttr()),
		MetricAnomaly:             types.ObjectNull(metricAnomalyAttr()),
		TracingImmediate:          types.ObjectNull(tracingImmediateAttr()),
		TracingThreshold:          types.ObjectNull(tracingThresholdAttr()),
		Flow:                      types.ObjectNull(flowAttr()),
	}
	var diags diag.Diagnostics
	switch alertTypeDefinition := properties.TypeDefinition.(type) {
	case *cxsdk.AlertDefPropertiesLogsImmediate:
		alertTypeDefinitionModel.LogsImmediate, diags = flattenLogsImmediate(ctx, alertTypeDefinition.LogsImmediate)
	case *cxsdk.AlertDefPropertiesLogsThreshold:
		alertTypeDefinitionModel.LogsThreshold, diags = flattenLogsThreshold(ctx, alertTypeDefinition.LogsThreshold)
	case *cxsdk.AlertDefPropertiesLogsAnomaly:
		alertTypeDefinitionModel.LogsAnomaly, diags = flattenLogsAnomaly(ctx, alertTypeDefinition.LogsAnomaly)
	case *cxsdk.AlertDefPropertiesLogsRatioThreshold:
		alertTypeDefinitionModel.LogsRatioThreshold, diags = flattenLogsRatioThreshold(ctx, alertTypeDefinition.LogsRatioThreshold)
	case *cxsdk.AlertDefPropertiesLogsNewValue:
		alertTypeDefinitionModel.LogsNewValue, diags = flattenLogsNewValue(ctx, alertTypeDefinition.LogsNewValue)
	case *cxsdk.AlertDefPropertiesLogsUniqueCount:
		alertTypeDefinitionModel.LogsUniqueCount, diags = flattenLogsUniqueCount(ctx, alertTypeDefinition.LogsUniqueCount)
	case *cxsdk.AlertDefPropertiesLogsTimeRelativeThreshold:
		alertTypeDefinitionModel.LogsTimeRelativeThreshold, diags = flattenLogsTimeRelativeThreshold(ctx, alertTypeDefinition.LogsTimeRelativeThreshold)
	case *cxsdk.AlertDefPropertiesMetricThreshold:
		alertTypeDefinitionModel.MetricThreshold, diags = flattenMetricThreshold(ctx, alertTypeDefinition.MetricThreshold)
	case *cxsdk.AlertDefPropertiesMetricAnomaly:
		alertTypeDefinitionModel.MetricAnomaly, diags = flattenMetricAnomaly(ctx, alertTypeDefinition.MetricAnomaly)
	case *cxsdk.AlertDefPropertiesTracingImmediate:
		alertTypeDefinitionModel.TracingImmediate, diags = flattenTracingImmediate(ctx, alertTypeDefinition.TracingImmediate)
	case *cxsdk.AlertDefPropertiesTracingThreshold:
		alertTypeDefinitionModel.TracingThreshold, diags = flattenTracingThreshold(ctx, alertTypeDefinition.TracingThreshold)
	case *cxsdk.AlertDefPropertiesFlow:
		alertTypeDefinitionModel.Flow, diags = flattenFlow(ctx, alertTypeDefinition.Flow)
	default:
		return types.ObjectNull(alertTypeDefinitionAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Alert Type Definition", fmt.Sprintf("Alert Type '%v' Definition is not valid", alertTypeDefinition))}
	}

	if diags.HasError() {
		return types.ObjectNull(alertTypeDefinitionAttr()), diags
	}

	return types.ObjectValueFrom(ctx, alertTypeDefinitionAttr(), alertTypeDefinitionModel)
}

func flattenLogsImmediate(ctx context.Context, immediate *cxsdk.LogsImmediateType) (types.Object, diag.Diagnostics) {
	if immediate == nil {
		return types.ObjectNull(logsImmediateAttr()), nil
	}

	logsFilter, diags := flattenAlertsLogsFilter(ctx, immediate.GetLogsFilter())
	if diags.HasError() {
		return types.ObjectNull(logsImmediateAttr()), diags
	}

	logsImmediateModel := LogsImmediateModel{
		LogsFilter:                logsFilter,
		NotificationPayloadFilter: wrappedStringSliceToTypeStringSet(immediate.GetNotificationPayloadFilter()),
	}
	return types.ObjectValueFrom(ctx, logsImmediateAttr(), logsImmediateModel)
}

func flattenAlertsLogsFilter(ctx context.Context, filter *cxsdk.LogsFilter) (types.Object, diag.Diagnostics) {
	if filter == nil {
		return types.ObjectNull(logsFilterAttr()), nil
	}

	var diags diag.Diagnostics
	var logsFilterModer AlertsLogsFilterModel
	switch filterType := filter.FilterType.(type) {
	case *cxsdk.LogsFilterSimpleFilter:
		logsFilterModer.SimpleFilter, diags = flattenSimpleFilter(ctx, filterType.SimpleFilter)
	default:
		return types.ObjectNull(logsFilterAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Logs Filter", fmt.Sprintf("Logs Filter %v is not supported", filterType))}
	}

	if diags.HasError() {
		return types.ObjectNull(logsFilterAttr()), diags
	}

	return types.ObjectValueFrom(ctx, logsFilterAttr(), logsFilterModer)
}

func flattenSimpleFilter(ctx context.Context, filter *cxsdk.SimpleFilter) (types.Object, diag.Diagnostics) {
	if filter == nil {
		return types.ObjectNull(luceneFilterAttr()), nil
	}

	labelFilters, diags := flattenLabelFilters(ctx, filter.GetLabelFilters())
	if diags.HasError() {
		return types.ObjectNull(luceneFilterAttr()), diags
	}

	return types.ObjectValueFrom(ctx, luceneFilterAttr(), SimpleFilterModel{
		LuceneQuery:  wrapperspbStringToTypeString(filter.GetLuceneQuery()),
		LabelFilters: labelFilters,
	})
}

func flattenLabelFilters(ctx context.Context, filters *cxsdk.LabelFilters) (types.Object, diag.Diagnostics) {
	if filters == nil {
		return types.ObjectNull(labelFiltersAttr()), nil
	}

	applicationName, diags := flattenLabelFilterTypes(ctx, filters.GetApplicationName())
	if diags.HasError() {
		return types.ObjectNull(labelFiltersAttr()), diags
	}

	subsystemName, diags := flattenLabelFilterTypes(ctx, filters.GetSubsystemName())
	if diags.HasError() {
		return types.ObjectNull(labelFiltersAttr()), diags
	}

	severities, diags := flattenLogSeverities(ctx, filters.GetSeverities())
	if diags.HasError() {
		return types.ObjectNull(labelFiltersAttr()), diags
	}

	return types.ObjectValueFrom(ctx, labelFiltersAttr(), LabelFiltersModel{
		ApplicationName: applicationName,
		SubsystemName:   subsystemName,
		Severities:      severities,
	})
}

func flattenLabelFilterTypes(ctx context.Context, name []*cxsdk.LabelFilterType) (types.Set, diag.Diagnostics) {
	var labelFilterTypes []LabelFilterTypeModel
	var diags diag.Diagnostics
	for _, lft := range name {
		labelFilterType := LabelFilterTypeModel{
			Value:     wrapperspbStringToTypeString(lft.GetValue()),
			Operation: types.StringValue(logFilterOperationTypeProtoToSchemaMap[lft.GetOperation()]),
		}
		labelFilterTypes = append(labelFilterTypes, labelFilterType)
	}
	if diags.HasError() {
		return types.SetNull(types.ObjectType{AttrTypes: labelFilterTypesAttr()}), diags
	}
	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: labelFilterTypesAttr()}, labelFilterTypes)

}

func flattenLogSeverities(ctx context.Context, severities []cxsdk.LogSeverity) (types.Set, diag.Diagnostics) {
	var result []attr.Value
	for _, severity := range severities {
		result = append(result, types.StringValue(logSeverityProtoToSchemaMap[severity]))
	}
	return types.SetValueFrom(ctx, types.StringType, result)
}

func flattenLogsThreshold(ctx context.Context, threshold *cxsdk.LogsThresholdType) (types.Object, diag.Diagnostics) {
	if threshold == nil {
		return types.ObjectNull(logsThresholdAttr()), nil
	}

	logsFilter, diags := flattenAlertsLogsFilter(ctx, threshold.GetLogsFilter())
	if diags.HasError() {
		return types.ObjectNull(logsThresholdAttr()), diags
	}

	rules, diags := flattenLogsThresholdRules(ctx, threshold.Rules)
	if diags.HasError() {
		return types.ObjectNull(logsThresholdAttr()), diags
	}

	undetected, diags := flattenUndetectedValuesManagement(ctx, threshold.GetUndetectedValuesManagement())
	if diags.HasError() {
		return types.ObjectNull(logsThresholdAttr()), diags
	}

	logsMoreThanModel := LogsThresholdModel{
		LogsFilter:                 logsFilter,
		Rules:                      rules,
		NotificationPayloadFilter:  wrappedStringSliceToTypeStringSet(threshold.GetNotificationPayloadFilter()),
		UndetectedValuesManagement: undetected,
	}
	return types.ObjectValueFrom(ctx, logsThresholdAttr(), logsMoreThanModel)
}

func flattenLogsThresholdRules(ctx context.Context, rules []*cxsdk.LogsThresholdRule) (types.Set, diag.Diagnostics) {
	if rules == nil {
		return types.SetNull(types.ObjectType{AttrTypes: flowStageAttr()}), nil
	}
	convertedRules := make([]*LogsThresholdRuleModel, len(rules))
	var diags diag.Diagnostics
	for i, rule := range rules {
		condition, dgs := flattenLogsThresholdRuleCondition(ctx, rule.Condition)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}

		override, dgs := flattenAlertOverride(ctx, rule.Override)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}

		convertedRules[i] = &LogsThresholdRuleModel{
			Condition: condition,
			Override:  override,
		}
	}
	if diags.HasError() {
		return types.SetNull(types.ObjectType{AttrTypes: logsThresholdRulesAttr()}), diags
	}
	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: logsThresholdRulesAttr()}, convertedRules)
}

func flattenLogsThresholdRuleCondition(ctx context.Context, condition *cxsdk.LogsThresholdCondition) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(logsThresholdConditionAttr()), nil
	}

	return types.ObjectValueFrom(ctx, logsThresholdConditionAttr(), LogsThresholdConditionModel{
		Threshold:     wrapperspbDoubleToTypeFloat64(condition.GetThreshold()),
		TimeWindow:    flattenLogsTimeWindow(condition.TimeWindow),
		ConditionType: types.StringValue(logsThresholdConditionMap[condition.GetConditionType()]),
	})
}

func flattenLogsTimeWindow(timeWindow *cxsdk.LogsTimeWindow) types.String {
	if timeWindow == nil {
		return types.StringNull()
	}
	return types.StringValue(logsTimeWindowValueProtoToSchemaMap[timeWindow.GetLogsTimeWindowSpecificValue()])
}

func flattenLogsRatioTimeWindow(timeWindow *cxsdk.LogsRatioTimeWindow) types.String {
	if timeWindow == nil {
		return types.StringNull()
	}
	return types.StringValue(logsRatioTimeWindowValueProtoToSchemaMap[timeWindow.GetLogsRatioTimeWindowSpecificValue()])
}

func flattenLogsNewValueTimeWindow(timeWindow *cxsdk.LogsNewValueTimeWindow) types.String {
	if timeWindow == nil {
		return types.StringNull()
	}
	return types.StringValue(logsNewValueTimeWindowValueProtoToSchemaMap[timeWindow.GetLogsNewValueTimeWindowSpecificValue()])
}

func flattenUndetectedValuesManagement(ctx context.Context, undetectedValuesManagement *cxsdk.UndetectedValuesManagement) (types.Object, diag.Diagnostics) {
	var undetectedValuesManagementModel UndetectedValuesManagementModel
	if undetectedValuesManagement == nil {
		undetectedValuesManagementModel.TriggerUndetectedValues = types.BoolValue(false)
		undetectedValuesManagementModel.AutoRetireTimeframe = types.StringValue(autoRetireTimeframeProtoToSchemaMap[cxsdk.AutoRetireTimeframeNeverOrUnspecified])
	} else {
		undetectedValuesManagementModel.TriggerUndetectedValues = wrapperspbBoolToTypeBool(undetectedValuesManagement.GetTriggerUndetectedValues())
		undetectedValuesManagementModel.AutoRetireTimeframe = types.StringValue(autoRetireTimeframeProtoToSchemaMap[undetectedValuesManagement.GetAutoRetireTimeframe()])
	}
	return types.ObjectValueFrom(ctx, undetectedValuesManagementAttr(), undetectedValuesManagementModel)
}

func flattenLogsAnomaly(ctx context.Context, anomaly *cxsdk.LogsAnomalyType) (types.Object, diag.Diagnostics) {
	if anomaly == nil {
		return types.ObjectNull(logsAnomalyAttr()), nil
	}

	logsFilter, diags := flattenAlertsLogsFilter(ctx, anomaly.GetLogsFilter())
	if diags.HasError() {
		return types.ObjectNull(logsAnomalyAttr()), diags
	}

	rulesRaw := make([]LogsAnomalyRuleModel, len(anomaly.Rules))
	for i, rule := range anomaly.Rules {
		condition, dgs := flattenLogsAnomalyRuleCondition(ctx, rule.Condition)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		rulesRaw[i] = LogsAnomalyRuleModel{
			Condition: condition,
		}
	}
	if diags.HasError() {
		return types.ObjectNull(logsAnomalyAttr()), diags
	}
	rules, diags := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: logsAnomalyRulesAttr()}, rulesRaw)
	if diags.HasError() {
		return types.ObjectNull(logsAnomalyAttr()), diags
	}
	logsMoreThanUsualModel := LogsAnomalyModel{
		LogsFilter:                logsFilter,
		Rules:                     rules,
		NotificationPayloadFilter: wrappedStringSliceToTypeStringSet(anomaly.GetNotificationPayloadFilter()),
	}
	return types.ObjectValueFrom(ctx, logsAnomalyAttr(), logsMoreThanUsualModel)
}

func flattenLogsAnomalyRuleCondition(ctx context.Context, condition *cxsdk.LogsAnomalyCondition) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(logsAnomalyConditionAttr()), nil
	}

	return types.ObjectValueFrom(ctx, logsAnomalyConditionAttr(), LogsAnomalyConditionModel{
		MinimumThreshold: wrapperspbDoubleToTypeFloat64(condition.GetMinimumThreshold()),
		TimeWindow:       flattenLogsTimeWindow(condition.TimeWindow),
		ConditionType:    types.StringValue(logsAnomalyConditionMap[condition.GetConditionType()]),
	})
}

func flattenLogsRatioThreshold(ctx context.Context, ratioThreshold *cxsdk.LogsRatioThresholdType) (types.Object, diag.Diagnostics) {
	if ratioThreshold == nil {
		return types.ObjectNull(logsRatioThresholdAttr()), nil
	}

	numeratorLogsFilter, diags := flattenAlertsLogsFilter(ctx, ratioThreshold.GetNumerator())
	if diags.HasError() {
		return types.ObjectNull(logsRatioThresholdAttr()), diags
	}

	denominatorLogsFilter, diags := flattenAlertsLogsFilter(ctx, ratioThreshold.GetDenominator())
	if diags.HasError() {
		return types.ObjectNull(logsRatioThresholdAttr()), diags
	}

	rules, diags := flattenRatioThresholdRules(ctx, ratioThreshold)
	if diags.HasError() {
		return types.ObjectNull(logsRatioThresholdAttr()), diags
	}

	logsRatioMoreThanModel := LogsRatioThresholdModel{
		Numerator:                 numeratorLogsFilter,
		NumeratorAlias:            wrapperspbStringToTypeString(ratioThreshold.GetNumeratorAlias()),
		Denominator:               denominatorLogsFilter,
		DenominatorAlias:          wrapperspbStringToTypeString(ratioThreshold.GetDenominatorAlias()),
		Rules:                     rules,
		NotificationPayloadFilter: wrappedStringSliceToTypeStringSet(ratioThreshold.GetNotificationPayloadFilter()),
		GroupByFor:                types.StringValue(logsRatioGroupByForProtoToSchemaMap[ratioThreshold.GetGroupByFor()]),
	}
	return types.ObjectValueFrom(ctx, logsRatioThresholdAttr(), logsRatioMoreThanModel)
}

func flattenRatioThresholdRules(ctx context.Context, ratioThreshold *cxsdk.LogsRatioThresholdType) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics
	rulesRaw := make([]LogsRatioThresholdRuleModel, len(ratioThreshold.Rules))
	for i, rule := range ratioThreshold.Rules {
		condition, dgs := flattenLogsRatioThresholdRuleCondition(ctx, rule.Condition)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}

		override, dgs := flattenAlertOverride(ctx, rule.Override)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}

		rulesRaw[i] = LogsRatioThresholdRuleModel{
			Condition: condition,
			Override:  override,
		}
	}

	if diags.HasError() {
		return types.SetNull(types.ObjectType{AttrTypes: logsRatioThresholdRulesAttr()}), diags
	}

	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: logsRatioThresholdRulesAttr()}, rulesRaw)
}

func flattenLogsRatioThresholdRuleCondition(ctx context.Context, condition *cxsdk.LogsRatioCondition) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(logsRatioThresholdRuleConditionAttr()), nil
	}

	return types.ObjectValueFrom(ctx, logsRatioThresholdRuleConditionAttr(), LogsRatioConditionModel{
		Threshold:     wrapperspbDoubleToTypeFloat64(condition.GetThreshold()),
		TimeWindow:    flattenLogsRatioTimeWindow(condition.TimeWindow),
		ConditionType: types.StringValue(logsRatioConditionMap[condition.GetConditionType()]),
	},
	)
}

func flattenAlertOverride(ctx context.Context, override *cxsdk.AlertDefPriorityOverride) (types.Object, diag.Diagnostics) {
	if override == nil {
		return types.ObjectNull(alertOverrideAttr()), nil
	}

	return types.ObjectValueFrom(ctx, alertOverrideAttr(), AlertOverrideModel{
		Priority: types.StringValue(alertPriorityProtoToSchemaMap[override.GetPriority()]),
	})
}

func flattenLogsUniqueCount(ctx context.Context, uniqueCount *cxsdk.LogsUniqueCountType) (types.Object, diag.Diagnostics) {
	if uniqueCount == nil {
		return types.ObjectNull(logsUniqueCountAttr()), nil
	}

	logsFilter, diags := flattenAlertsLogsFilter(ctx, uniqueCount.GetLogsFilter())
	if diags.HasError() {
		return types.ObjectNull(logsUniqueCountAttr()), diags
	}

	rules, diags := flattenLogsUniqueCountRules(ctx, uniqueCount)
	if diags.HasError() {
		return types.ObjectNull(logsUniqueCountAttr()), diags
	}

	logsUniqueCountModel := LogsUniqueCountModel{
		LogsFilter:                  logsFilter,
		Rules:                       rules,
		NotificationPayloadFilter:   wrappedStringSliceToTypeStringSet(uniqueCount.GetNotificationPayloadFilter()),
		MaxUniqueCountPerGroupByKey: wrapperspbInt64ToTypeInt64(uniqueCount.GetMaxUniqueCountPerGroupByKey()),
		UniqueCountKeypath:          wrapperspbStringToTypeString(uniqueCount.GetUniqueCountKeypath()),
	}
	return types.ObjectValueFrom(ctx, logsUniqueCountAttr(), logsUniqueCountModel)
}

func flattenLogsUniqueCountRules(ctx context.Context, uniqueCount *cxsdk.LogsUniqueCountType) (types.Set, diag.Diagnostics) {
	rulesRaw := make([]LogsUniqueCountRuleModel, len(uniqueCount.Rules))
	var diags diag.Diagnostics
	for i, rule := range uniqueCount.Rules {
		condition, dgs := flattenLogsUniqueCountRuleCondition(ctx, rule.Condition)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		rulesRaw[i] = LogsUniqueCountRuleModel{
			Condition: condition,
		}
	}

	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: logsUniqueCountRulesAttr()}, rulesRaw)
}

func flattenLogsUniqueCountRuleCondition(ctx context.Context, condition *cxsdk.LogsUniqueCountCondition) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(logsUniqueCountConditionAttr()), nil
	}

	return types.ObjectValueFrom(ctx, logsUniqueCountConditionAttr(), LogsUniqueCountConditionModel{
		MaxUniqueCount: wrapperspbInt64ToTypeInt64(condition.GetMaxUniqueCount()),
		TimeWindow:     flattenLogsUniqueTimeWindow(condition.TimeWindow),
	})
}

func flattenLogsUniqueTimeWindow(timeWindow *cxsdk.LogsUniqueValueTimeWindow) types.String {
	if timeWindow == nil {
		return types.StringNull()
	}
	return types.StringValue(logsUniqueCountTimeWindowValueProtoToSchemaMap[timeWindow.GetLogsUniqueValueTimeWindowSpecificValue()])
}

func flattenLogsNewValue(ctx context.Context, newValue *cxsdk.LogsNewValueType) (types.Object, diag.Diagnostics) {
	if newValue == nil {
		return types.ObjectNull(logsNewValueAttr()), nil
	}

	logsFilter, diags := flattenAlertsLogsFilter(ctx, newValue.GetLogsFilter())
	if diags.HasError() {
		return types.ObjectNull(logsNewValueAttr()), diags
	}

	rulesRaw := make([]NewValueRuleModel, len(newValue.Rules))
	for i, rule := range newValue.Rules {
		condition, dgs := flattenLogsNewValueCondition(ctx, rule.Condition)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		rulesRaw[i] = NewValueRuleModel{
			Condition: condition,
		}
	}
	if diags.HasError() {
		return types.ObjectNull(logsNewValueAttr()), diags
	}

	rules, diags := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: logsNewValueRulesAttr()}, rulesRaw)
	if diags.HasError() {
		return types.ObjectNull(logsNewValueAttr()), diags
	}

	logsNewValueModel := LogsNewValueModel{
		LogsFilter:                logsFilter,
		Rules:                     rules,
		NotificationPayloadFilter: wrappedStringSliceToTypeStringSet(newValue.GetNotificationPayloadFilter()),
	}
	return types.ObjectValueFrom(ctx, logsNewValueAttr(), logsNewValueModel)
}

func flattenLogsNewValueCondition(ctx context.Context, condition *cxsdk.LogsNewValueCondition) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(logsNewValueConditionAttr()), nil
	}

	return types.ObjectValueFrom(ctx, logsNewValueConditionAttr(), NewValueConditionModel{
		TimeWindow:     flattenLogsNewValueTimeWindow(condition.TimeWindow),
		KeypathToTrack: wrapperspbStringToTypeString(condition.GetKeypathToTrack()),
	})
}

func flattenAlertSchedule(ctx context.Context, alertProperties *cxsdk.AlertDefProperties, currentSchedule *types.Object) (types.Object, diag.Diagnostics) {
	if alertProperties.Schedule == nil {
		return types.ObjectNull(alertScheduleAttr()), nil
	}

	var alertScheduleModel AlertScheduleModel
	var diags diag.Diagnostics
	switch alertScheduleType := alertProperties.Schedule.(type) {
	case *cxsdk.AlertDefPropertiesActiveOn:
		var activeOnModel ActiveOnModel
		if diags := currentSchedule.As(ctx, &activeOnModel, basetypes.ObjectAsOptions{}); diags.HasError() {
			return types.ObjectNull(alertScheduleAttr()), diags
		}

		alertScheduleModel.ActiveOn, diags = flattenActiveOn(ctx, alertScheduleType.ActiveOn, activeOnModel.UtcOffset.ValueString())
	default:
		return types.ObjectNull(alertScheduleAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Alert Schedule", fmt.Sprintf("Alert Schedule %v is not supported", alertScheduleType))}
	}

	if diags.HasError() {
		return types.ObjectNull(alertScheduleAttr()), diags
	}

	return types.ObjectValueFrom(ctx, alertScheduleAttr(), alertScheduleModel)
}

func flattenActiveOn(ctx context.Context, activeOn *cxsdk.AlertDefActivitySchedule, utcOffset string) (types.Object, diag.Diagnostics) {
	if activeOn == nil {
		return types.ObjectNull(alertScheduleActiveOnAttr()), nil
	}

	daysOfWeek, diags := flattenDaysOfWeek(ctx, activeOn.GetDayOfWeek())
	if diags.HasError() {
		return types.ObjectNull(alertScheduleActiveOnAttr()), diags
	}
	offset, err := time.Parse(OFFSET_FORMAT, utcOffset)

	if err != nil {
		return types.ObjectNull(alertScheduleActiveOnAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid UTC Offset", fmt.Sprintf("UTC Offset %v is not valid", utcOffset))}
	}
	zoneName, offsetSecs := offset.Zone() // Name is probably empty
	zone := time.FixedZone(zoneName, offsetSecs)
	startTime := time.Date(2021, 2, 1, int(activeOn.StartTime.Hours), int(activeOn.StartTime.Minutes), 0, 0, zone)

	endTime := time.Date(2021, 2, 1, int(activeOn.EndTime.Hours), int(activeOn.EndTime.Minutes), 0, 0, zone)

	activeOnModel := ActiveOnModel{
		DaysOfWeek: daysOfWeek,
		StartTime:  types.StringValue(startTime.UTC().Format(TIME_FORMAT)),
		EndTime:    types.StringValue(endTime.UTC().Format(TIME_FORMAT)),
		UtcOffset:  types.StringValue(utcOffset),
	}
	return types.ObjectValueFrom(ctx, alertScheduleActiveOnAttr(), activeOnModel)
}

func flattenDaysOfWeek(ctx context.Context, daysOfWeek []cxsdk.AlertDayOfWeek) (types.Set, diag.Diagnostics) {
	var daysOfWeekStrings []types.String
	for _, dow := range daysOfWeek {
		daysOfWeekStrings = append(daysOfWeekStrings, types.StringValue(daysOfWeekProtoToSchemaMap[dow]))
	}
	return types.SetValueFrom(ctx, types.StringType, daysOfWeekStrings)
}

func flattenLogsTimeRelativeThreshold(ctx context.Context, logsTimeRelativeThreshold *cxsdk.LogsTimeRelativeThresholdType) (types.Object, diag.Diagnostics) {
	if logsTimeRelativeThreshold == nil {
		return types.ObjectNull(logsTimeRelativeAttr()), nil
	}

	logsFilter, diags := flattenAlertsLogsFilter(ctx, logsTimeRelativeThreshold.GetLogsFilter())
	if diags.HasError() {
		return types.ObjectNull(logsTimeRelativeAttr()), diags
	}

	rulesRaw := make([]LogsTimeRelativeRuleModel, len(logsTimeRelativeThreshold.Rules))
	for i, rule := range logsTimeRelativeThreshold.Rules {
		condition, dgs := flattenLogsTimeRelativeRuleCondition(ctx, rule.Condition)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}

		override, dgs := flattenAlertOverride(ctx, rule.Override)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}

		rulesRaw[i] = LogsTimeRelativeRuleModel{
			Condition: condition,
			Override:  override,
		}
	}

	if diags.HasError() {
		return types.ObjectNull(logsTimeRelativeAttr()), diags
	}

	rules, diags := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: logsTimeRelativeRulesAttr()}, rulesRaw)
	if diags.HasError() {
		return types.ObjectNull(logsTimeRelativeAttr()), diags
	}

	undetected, diags := flattenUndetectedValuesManagement(ctx, logsTimeRelativeThreshold.UndetectedValuesManagement)
	if diags.HasError() {
		return types.ObjectNull(logsTimeRelativeAttr()), diags
	}

	logsTimeRelativeThresholdModel := LogsTimeRelativeThresholdModel{
		LogsFilter:                 logsFilter,
		Rules:                      rules,
		NotificationPayloadFilter:  wrappedStringSliceToTypeStringSet(logsTimeRelativeThreshold.GetNotificationPayloadFilter()),
		UndetectedValuesManagement: undetected,
	}

	return types.ObjectValueFrom(ctx, logsTimeRelativeAttr(), logsTimeRelativeThresholdModel)
}

func flattenLogsTimeRelativeRuleCondition(ctx context.Context, condition *cxsdk.LogsTimeRelativeCondition) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(logsTimeRelativeConditionAttr()), nil
	}

	return types.ObjectValueFrom(ctx, logsTimeRelativeConditionAttr(), LogsTimeRelativeConditionModel{
		Threshold:     wrapperspbDoubleToTypeFloat64(condition.GetThreshold()),
		ComparedTo:    types.StringValue(logsTimeRelativeComparedToProtoToSchemaMap[condition.GetComparedTo()]),
		ConditionType: types.StringValue(logsTimeRelativeConditionMap[condition.GetConditionType()]),
	})
}

func flattenMetricThreshold(ctx context.Context, metricThreshold *cxsdk.MetricThresholdType) (types.Object, diag.Diagnostics) {
	if metricThreshold == nil {
		return types.ObjectNull(metricThresholdAttr()), nil
	}

	metricFilter, diags := flattenMetricFilter(ctx, metricThreshold.GetMetricFilter())
	if diags.HasError() {
		return types.ObjectNull(metricThresholdAttr()), diags
	}

	undetectedValuesManagement, diags := flattenUndetectedValuesManagement(ctx, metricThreshold.GetUndetectedValuesManagement())
	if diags.HasError() {
		return types.ObjectNull(metricThresholdAttr()), diags
	}

	rulesRaw := make([]MetricThresholdRuleModel, len(metricThreshold.Rules))
	for i, rule := range metricThreshold.Rules {
		condition, dgs := flattenMetricThresholdRuleCondition(ctx, rule.Condition)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}

		override, dgs := flattenAlertOverride(ctx, rule.Override)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}

		rulesRaw[i] = MetricThresholdRuleModel{
			Condition: condition,
			Override:  override,
		}
	}
	rules, diags := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: metricThresholdRulesAttr()}, rulesRaw)
	if diags.HasError() {
		return types.ObjectNull(metricThresholdAttr()), diags
	}

	missingValues, diags := flattenMissingValuesManagement(ctx, metricThreshold.GetMissingValues())
	if diags.HasError() {
		return types.ObjectNull(metricThresholdAttr()), diags
	}

	metricThresholdModel := MetricThresholdModel{
		MetricFilter:               metricFilter,
		Rules:                      rules,
		MissingValues:              missingValues,
		UndetectedValuesManagement: undetectedValuesManagement,
	}
	return types.ObjectValueFrom(ctx, metricThresholdAttr(), metricThresholdModel)
}

func flattenMissingValuesManagement(ctx context.Context, missingValues *cxsdk.MetricMissingValues) (types.Object, diag.Diagnostics) {
	if missingValues == nil {
		return types.ObjectNull(missingValuesAttr()), nil
	}

	switch missingValuesType := missingValues.MissingValues.(type) {
	case *cxsdk.MetricMissingValuesReplaceWithZero:
		return types.ObjectValueFrom(ctx, missingValuesAttr(), MissingValuesModel{
			ReplaceWithZero: wrapperspbBoolToTypeBool(missingValuesType.ReplaceWithZero),
		})
	case *cxsdk.MetricMissingValuesMinNonNullValuesPct:
		return types.ObjectValueFrom(ctx, missingValuesAttr(), MissingValuesModel{
			MinNonNullValuesPct: wrapperspbUint32ToTypeInt64(missingValuesType.MinNonNullValuesPct),
		})
	default:
		return types.ObjectNull(missingValuesAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Missing Values Management", fmt.Sprintf("Missing Values Management %v is not supported", missingValuesType))}
	}
}

func flattenMetricThresholdRuleCondition(ctx context.Context, condition *cxsdk.MetricThresholdCondition) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(metricThresholdConditionAttr()), nil
	}

	return types.ObjectValueFrom(ctx, metricThresholdConditionAttr(), MetricThresholdConditionModel{
		Threshold:     wrapperspbDoubleToTypeFloat64(condition.GetThreshold()),
		ForOverPct:    wrapperspbUint32ToTypeInt64(condition.GetForOverPct()),
		OfTheLast:     flattenMetricTimeWindow(condition.GetOfTheLast()),
		ConditionType: types.StringValue(metricsThresholdConditionMap[condition.GetConditionType()]),
	})
}

func flattenMetricTimeWindow(timeWindow *cxsdk.MetricTimeWindow) types.String {
	if timeWindow == nil {
		return types.StringNull()
	}

	return types.StringValue(metricFilterOperationTypeProtoToSchemaMap[timeWindow.GetMetricTimeWindowSpecificValue()])
}

func flattenMetricFilter(ctx context.Context, filter *cxsdk.MetricFilter) (types.Object, diag.Diagnostics) {
	if filter == nil {
		return types.ObjectNull(metricFilterAttr()), nil
	}

	switch filterType := filter.Type.(type) {
	case *cxsdk.MetricFilterPromql:
		return types.ObjectValueFrom(ctx, metricFilterAttr(), MetricFilterModel{
			Promql: wrapperspbStringToTypeString(filterType.Promql),
		})
	default:
		return types.ObjectNull(metricFilterAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Metric Filter", fmt.Sprintf("Metric Filter %v is not supported", filterType))}
	}
}

func flattenTracingImmediate(ctx context.Context, tracingImmediate *cxsdk.TracingImmediateType) (types.Object, diag.Diagnostics) {
	if tracingImmediate == nil {
		return types.ObjectNull(tracingImmediateAttr()), nil
	}

	var tracingQuery types.Object

	switch filtersType := tracingImmediate.TracingFilter.FilterType.(type) {
	case *cxsdk.TracingFilterSimpleFilter:
		filter, diag := flattenTracingSimpleFilter(ctx, filtersType.SimpleFilter)
		if diag.HasError() {
			return types.ObjectNull(tracingImmediateAttr()), diag
		}
		tracingQuery, diag = types.ObjectValueFrom(ctx, tracingQueryAttr(), filter)
		if diag.HasError() {
			return types.ObjectNull(tracingImmediateAttr()), diag
		}
	default:
		return types.ObjectNull(tracingImmediateAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Tracing Query Filters", fmt.Sprintf("Tracing Query Filters %v is not supported", filtersType))}
	}

	tracingImmediateModel := TracingImmediateModel{
		TracingFilter:             tracingQuery,
		NotificationPayloadFilter: wrappedStringSliceToTypeStringSet(tracingImmediate.GetNotificationPayloadFilter()),
	}

	return types.ObjectValueFrom(ctx, tracingImmediateAttr(), tracingImmediateModel)
}

// Also called query filters
func flattenTracingFilter(ctx context.Context, tracingFilter *cxsdk.TracingFilter) (types.Object, diag.Diagnostics) {
	switch filtersType := tracingFilter.FilterType.(type) {
	case *cxsdk.TracingFilterSimpleFilter:
		filter, diag := flattenTracingSimpleFilter(ctx, filtersType.SimpleFilter)
		if diag.HasError() {
			return types.ObjectNull(tracingQueryAttr()), diag
		}
		tracingQuery, diag := types.ObjectValueFrom(ctx, tracingQueryAttr(), filter)
		if diag.HasError() {
			return types.ObjectNull(tracingQueryAttr()), diag
		}
		return tracingQuery, nil
	default:
		return types.ObjectNull(tracingQueryAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Tracing Query Filters", fmt.Sprintf("Tracing Query Filters %v is not supported", filtersType))}
	}

}

func flattenTracingSimpleFilter(ctx context.Context, tracingQuery *cxsdk.TracingSimpleFilter) (types.Object, diag.Diagnostics) {
	if tracingQuery == nil {
		return types.ObjectNull(tracingQueryAttr()), nil
	}

	labelFilters, diags := flattenTracingLabelFilters(ctx, tracingQuery.TracingLabelFilters)
	if diags.HasError() {
		return types.ObjectNull(tracingQueryAttr()), diags
	}
	tracingQueryModel := &TracingFilterModel{
		LatencyThresholdMs:  wrappedUint64TotypeNumber(tracingQuery.LatencyThresholdMs),
		TracingLabelFilters: labelFilters,
	}
	if diags.HasError() {
		return types.ObjectNull(tracingQueryAttr()), diags
	}

	return types.ObjectValueFrom(ctx, tracingQueryAttr(), tracingQueryModel)
}

func flattenTracingLabelFilters(ctx context.Context, filters *cxsdk.TracingLabelFilters) (types.Object, diag.Diagnostics) {
	if filters == nil {
		return types.ObjectNull(tracingLabelFiltersAttr()), nil
	}

	applicationName, diags := flattenTracingFilterTypes(ctx, filters.GetApplicationName())
	if diags.HasError() {
		return types.ObjectNull(tracingLabelFiltersAttr()), diags
	}

	subsystemName, diags := flattenTracingFilterTypes(ctx, filters.GetSubsystemName())
	if diags.HasError() {
		return types.ObjectNull(tracingLabelFiltersAttr()), diags

	}

	serviceName, diags := flattenTracingFilterTypes(ctx, filters.GetServiceName())
	if diags.HasError() {
		return types.ObjectNull(tracingLabelFiltersAttr()), diags
	}

	operationName, diags := flattenTracingFilterTypes(ctx, filters.GetOperationName())
	if diags.HasError() {
		return types.ObjectNull(tracingLabelFiltersAttr()), diags
	}

	spanFields, diags := flattenTracingSpansFields(ctx, filters.GetSpanFields())
	if diags.HasError() {
		return types.ObjectNull(tracingLabelFiltersAttr()), diags
	}

	return types.ObjectValueFrom(ctx, tracingLabelFiltersAttr(), TracingLabelFiltersModel{
		ApplicationName: applicationName,
		SubsystemName:   subsystemName,
		ServiceName:     serviceName,
		OperationName:   operationName,
		SpanFields:      spanFields,
	})

}

func flattenTracingFilterTypes(ctx context.Context, TracingFilterType []*cxsdk.TracingFilterType) (types.Set, diag.Diagnostics) {
	var tracingFilterTypes []*TracingFilterTypeModel
	for _, tft := range TracingFilterType {
		tracingFilterTypes = append(tracingFilterTypes, flattenTracingFilterType(tft))
	}
	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: tracingFiltersTypeAttr()}, tracingFilterTypes)
}

func flattenTracingFilterType(tracingFilterType *cxsdk.TracingFilterType) *TracingFilterTypeModel {
	if tracingFilterType == nil {
		return nil
	}

	return &TracingFilterTypeModel{
		Values:    wrappedStringSliceToTypeStringSet(tracingFilterType.GetValues()),
		Operation: types.StringValue(tracingFilterOperationProtoToSchemaMap[tracingFilterType.GetOperation()]),
	}
}

func flattenTracingSpansFields(ctx context.Context, spanFields []*cxsdk.TracingSpanFieldsFilterType) (types.Set, diag.Diagnostics) {
	var tracingSpanFields []*TracingSpanFieldsFilterModel
	for _, field := range spanFields {
		tracingSpanField, diags := flattenTracingSpanField(ctx, field)
		if diags.HasError() {
			return types.SetNull(types.ObjectType{AttrTypes: tracingSpanFieldsFilterAttr()}), diags
		}
		tracingSpanFields = append(tracingSpanFields, tracingSpanField)
	}
	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: tracingSpanFieldsFilterAttr()}, tracingSpanFields)
}

func flattenTracingSpanField(ctx context.Context, spanField *cxsdk.TracingSpanFieldsFilterType) (*TracingSpanFieldsFilterModel, diag.Diagnostics) {
	if spanField == nil {
		return nil, nil
	}

	filterType, diags := types.ObjectValueFrom(ctx, tracingFiltersTypeAttr(), flattenTracingFilterType(spanField.GetFilterType()))
	if diags.HasError() {
		return nil, diags
	}

	return &TracingSpanFieldsFilterModel{
		Key:        wrapperspbStringToTypeString(spanField.GetKey()),
		FilterType: filterType,
	}, nil
}

func flattenTracingThreshold(ctx context.Context, tracingThreshold *cxsdk.TracingThresholdType) (types.Object, diag.Diagnostics) {
	if tracingThreshold == nil {
		return types.ObjectNull(tracingThresholdAttr()), nil
	}

	tracingQuery, diags := flattenTracingFilter(ctx, tracingThreshold.GetTracingFilter())
	if diags.HasError() {
		return types.ObjectNull(tracingThresholdAttr()), diags
	}

	rules, diags := flattenTracingThresholdRules(ctx, tracingThreshold, diags)
	if diags.HasError() {
		return types.ObjectNull(tracingThresholdAttr()), diags
	}

	tracingThresholdModel := TracingThresholdModel{
		TracingFilter:             tracingQuery,
		Rules:                     rules,
		NotificationPayloadFilter: wrappedStringSliceToTypeStringSet(tracingThreshold.GetNotificationPayloadFilter()),
	}
	return types.ObjectValueFrom(ctx, tracingThresholdAttr(), tracingThresholdModel)
}

func flattenTracingThresholdRules(ctx context.Context, tracingThreshold *cxsdk.TracingThresholdType, diags diag.Diagnostics) (basetypes.SetValue, diag.Diagnostics) {
	rulesRaw := make([]TracingThresholdRuleModel, len(tracingThreshold.Rules))
	for i, rule := range tracingThreshold.Rules {
		condition, dgs := flattenTracingThresholdRuleCondition(ctx, rule.Condition)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		rulesRaw[i] = TracingThresholdRuleModel{
			Condition: condition,
		}
	}
	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: tracingThresholdRulesAttr()}, rulesRaw)
}

func flattenTracingThresholdRuleCondition(ctx context.Context, condition *cxsdk.TracingThresholdCondition) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(tracingThresholdConditionAttr()), nil
	}

	return types.ObjectValueFrom(ctx, tracingThresholdConditionAttr(), TracingThresholdConditionModel{
		TimeWindow:    flattenTracingTimeWindow(condition.GetTimeWindow()),
		SpanAmount:    wrapperspbDoubleToTypeFloat64(condition.GetSpanAmount()),
		ConditionType: types.StringValue("MORE_THAN"),
	})
}

func flattenTracingTimeWindow(timeWindow *cxsdk.TracingTimeWindow) types.String {
	if timeWindow == nil {
		return types.StringNull()
	}

	return types.StringValue(tracingTimeWindowProtoToSchemaMap[timeWindow.GetTracingTimeWindowValue()])
}

func flattenMetricAnomaly(ctx context.Context, metricMoreThanUsual *cxsdk.MetricAnomalyType) (types.Object, diag.Diagnostics) {
	if metricMoreThanUsual == nil {
		return types.ObjectNull(metricAnomalyAttr()), nil
	}

	metricFilter, diags := flattenMetricFilter(ctx, metricMoreThanUsual.GetMetricFilter())
	if diags.HasError() {
		return types.ObjectNull(metricAnomalyAttr()), diags
	}

	rulesRaw := make([]MetricAnomalyRuleModel, len(metricMoreThanUsual.Rules))
	for i, rule := range metricMoreThanUsual.Rules {
		condition, dgs := flattenMetricAnomalyCondition(ctx, rule.Condition)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		rulesRaw[i] = MetricAnomalyRuleModel{
			Condition: condition,
		}
	}
	if diags.HasError() {
		return types.ObjectNull(metricAnomalyAttr()), diags
	}

	rules, diags := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: metricAnomalyRulesAttr()}, rulesRaw)
	if diags.HasError() {
		return types.ObjectNull(metricAnomalyAttr()), diags
	}
	metricMoreThanUsualModel := MetricAnomalyModel{
		MetricFilter: metricFilter,
		Rules:        rules,
	}
	return types.ObjectValueFrom(ctx, metricAnomalyAttr(), metricMoreThanUsualModel)
}

func flattenMetricAnomalyCondition(ctx context.Context, condition *cxsdk.MetricAnomalyCondition) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(metricAnomalyConditionAttr()), nil
	}

	return types.ObjectValueFrom(ctx, metricAnomalyConditionAttr(), MetricAnomalyConditionModel{
		MinNonNullValuesPct: wrapperspbUint32ToTypeInt64(condition.GetMinNonNullValuesPct()),
		Threshold:           wrapperspbDoubleToTypeFloat64(condition.GetThreshold()),
		ForOverPct:          wrapperspbUint32ToTypeInt64(condition.GetForOverPct()),
		OfTheLast:           flattenMetricTimeWindow(condition.GetOfTheLast()),
		ConditionType:       types.StringValue(metricAnomalyConditionMap[condition.GetConditionType()]),
	},
	)
}

func flattenFlow(ctx context.Context, flow *cxsdk.FlowType) (types.Object, diag.Diagnostics) {
	if flow == nil {
		return types.ObjectNull(flowAttr()), nil
	}

	stages, diags := flattenFlowStages(ctx, flow.GetStages())
	if diags.HasError() {
		return types.ObjectNull(flowAttr()), diags
	}

	flowModel := FlowModel{
		Stages:             stages,
		EnforceSuppression: wrapperspbBoolToTypeBool(flow.GetEnforceSuppression()),
	}
	return types.ObjectValueFrom(ctx, flowAttr(), flowModel)
}

func flattenFlowStages(ctx context.Context, stages []*cxsdk.FlowStages) (types.List, diag.Diagnostics) {
	var flowStages []*FlowStageModel
	for _, stage := range stages {
		flowStage, diags := flattenFlowStage(ctx, stage)
		if diags.HasError() {
			return types.ListNull(types.ObjectType{AttrTypes: flowStageAttr()}), diags
		}
		flowStages = append(flowStages, flowStage)
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: flowStageAttr()}, flowStages)

}

func flattenFlowStage(ctx context.Context, stage *cxsdk.FlowStages) (*FlowStageModel, diag.Diagnostics) {
	if stage == nil {
		return nil, nil
	}

	flowStagesGroups, diags := flattenFlowStagesGroups(ctx, stage)
	if diags.HasError() {
		return nil, diags
	}

	flowStageModel := &FlowStageModel{
		FlowStagesGroups: flowStagesGroups,
		TimeframeMs:      wrapperspbInt64ToTypeInt64(stage.GetTimeframeMs()),
		TimeframeType:    types.StringValue(flowStageTimeFrameTypeProtoToSchemaMap[stage.GetTimeframeType()]),
	}
	return flowStageModel, nil

}

func flattenFlowStagesGroups(ctx context.Context, stage *cxsdk.FlowStages) (types.List, diag.Diagnostics) {
	var flowStagesGroups []*FlowStagesGroupModel
	for _, group := range stage.GetFlowStagesGroups().GetGroups() {
		flowStageGroup, diags := flattenFlowStageGroup(ctx, group)
		if diags.HasError() {
			return types.ListNull(types.ObjectType{AttrTypes: flowStageGroupAttr()}), diags
		}
		flowStagesGroups = append(flowStagesGroups, flowStageGroup)
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: flowStageGroupAttr()}, flowStagesGroups)

}

func flattenFlowStageGroup(ctx context.Context, group *cxsdk.FlowStagesGroup) (*FlowStagesGroupModel, diag.Diagnostics) {
	if group == nil {
		return nil, nil
	}

	alertDefs, diags := flattenAlertDefs(ctx, group.GetAlertDefs())
	if diags.HasError() {
		return nil, diags
	}

	flowStageGroupModel := &FlowStagesGroupModel{
		AlertDefs: alertDefs,
		NextOp:    types.StringValue(flowStagesGroupNextOpProtoToSchemaMap[group.GetNextOp()]),
		AlertsOp:  types.StringValue(flowStagesGroupAlertsOpProtoToSchemaMap[group.GetAlertsOp()]),
	}
	return flowStageGroupModel, nil
}

func flattenAlertDefs(ctx context.Context, defs []*cxsdk.FlowStagesGroupsAlertDefs) (types.Set, diag.Diagnostics) {
	var alertDefs []*FlowStagesGroupsAlertDefsModel
	for _, def := range defs {
		alertDef := &FlowStagesGroupsAlertDefsModel{
			Id:  wrapperspbStringToTypeString(def.GetId()),
			Not: wrapperspbBoolToTypeBool(def.GetNot()),
		}
		alertDefs = append(alertDefs, alertDef)
	}
	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: alertDefsAttr()}, alertDefs)
}

func retriggeringPeriodAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"minutes": types.Int64Type,
	}
}

func incidentsSettingsAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"notify_on": types.StringType,
		"retriggering_period": types.ObjectType{
			AttrTypes: retriggeringPeriodAttr(),
		},
	}
}

func notificationGroupAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"group_by_keys": types.SetType{
			ElemType: types.StringType,
		},
		"webhooks_settings": types.SetType{
			ElemType: types.ObjectType{
				AttrTypes: webhooksSettingsAttr(),
			},
		},
	}
}

func webhooksSettingsAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"notify_on": types.StringType,
		"retriggering_period": types.ObjectType{
			AttrTypes: retriggeringPeriodAttr(),
		},
		"integration_id": types.StringType,
		"recipients":     types.SetType{ElemType: types.StringType},
	}
}

func alertTypeDefinitionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_immediate": types.ObjectType{
			AttrTypes: logsImmediateAttr(),
		},
		"logs_threshold": types.ObjectType{
			AttrTypes: logsThresholdAttr(),
		},
		"logs_anomaly": types.ObjectType{
			AttrTypes: logsAnomalyAttr(),
		},
		"logs_ratio_threshold": types.ObjectType{
			AttrTypes: logsRatioThresholdAttr(),
		},
		"logs_new_value": types.ObjectType{
			AttrTypes: logsNewValueAttr(),
		},
		"logs_unique_count": types.ObjectType{
			AttrTypes: logsUniqueCountAttr(),
		},
		"logs_time_relative_threshold": types.ObjectType{
			AttrTypes: logsTimeRelativeAttr(),
		},
		"metric_threshold": types.ObjectType{
			AttrTypes: metricThresholdAttr(),
		},
		"metric_anomaly": types.ObjectType{
			AttrTypes: metricAnomalyAttr(),
		},
		"tracing_immediate": types.ObjectType{
			AttrTypes: tracingImmediateAttr(),
		},
		"tracing_threshold": types.ObjectType{
			AttrTypes: tracingThresholdAttr(),
		},
		"flow": types.ObjectType{
			AttrTypes: flowAttr(),
		},
	}
}

func logsImmediateAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_filter": types.ObjectType{
			AttrTypes: logsFilterAttr(),
		},
		"notification_payload_filter": types.SetType{
			ElemType: types.StringType,
		},
	}
}

func logsFilterAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"simple_filter": types.ObjectType{
			AttrTypes: luceneFilterAttr(),
		},
	}
}

func luceneFilterAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"lucene_query": types.StringType,
		"label_filters": types.ObjectType{
			AttrTypes: labelFiltersAttr(),
		},
	}
}

func labelFiltersAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"application_name": types.SetType{
			ElemType: types.ObjectType{
				AttrTypes: labelFilterTypesAttr(),
			},
		},
		"subsystem_name": types.SetType{
			ElemType: types.ObjectType{
				AttrTypes: labelFilterTypesAttr(),
			},
		},
		"severities": types.SetType{
			ElemType: types.StringType,
		},
	}
}

func logsThresholdAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_filter":                  types.ObjectType{AttrTypes: logsFilterAttr()},
		"notification_payload_filter":  types.SetType{ElemType: types.StringType},
		"rules":                        types.SetType{ElemType: types.ObjectType{AttrTypes: logsThresholdRulesAttr()}},
		"undetected_values_management": types.ObjectType{AttrTypes: undetectedValuesManagementAttr()},
	}
}

func logsThresholdRulesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{AttrTypes: logsThresholdConditionAttr()},
		"override":  types.ObjectType{AttrTypes: alertOverrideAttr()},
	}
}

func logsThresholdConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"threshold":      types.Float64Type,
		"time_window":    types.StringType,
		"condition_type": types.StringType,
	}
}

func logsAnomalyAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_filter":                 types.ObjectType{AttrTypes: logsFilterAttr()},
		"rules":                       types.SetType{ElemType: types.ObjectType{AttrTypes: logsAnomalyRulesAttr()}},
		"notification_payload_filter": types.SetType{ElemType: types.StringType},
	}
}

func logsAnomalyRulesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{AttrTypes: logsAnomalyConditionAttr()},
	}
}

func logsAnomalyConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"minimum_threshold": types.Float64Type,
		"time_window":       types.StringType,
		"condition_type":    types.StringType,
	}
}

func logsRatioThresholdAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"numerator":         types.ObjectType{AttrTypes: logsFilterAttr()},
		"numerator_alias":   types.StringType,
		"denominator":       types.ObjectType{AttrTypes: logsFilterAttr()},
		"denominator_alias": types.StringType,
		"rules":             types.SetType{ElemType: types.ObjectType{AttrTypes: logsRatioThresholdRulesAttr()}},
		"notification_payload_filter": types.SetType{
			ElemType: types.StringType,
		},
		"group_by_for": types.StringType,
	}
}

func logsRatioThresholdRulesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{AttrTypes: logsRatioThresholdRuleConditionAttr()},
		"override":  types.ObjectType{AttrTypes: alertOverrideAttr()},
	}
}

func logsRatioThresholdRuleConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"threshold":      types.Float64Type,
		"time_window":    types.StringType,
		"condition_type": types.StringType,
	}
}

func alertOverrideAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"priority": types.StringType,
	}
}

func logsNewValueAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_filter":                 types.ObjectType{AttrTypes: logsFilterAttr()},
		"rules":                       types.SetType{ElemType: types.ObjectType{AttrTypes: logsNewValueRulesAttr()}},
		"notification_payload_filter": types.SetType{ElemType: types.StringType},
	}
}

func logsNewValueRulesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{AttrTypes: logsNewValueConditionAttr()},
	}
}

func logsNewValueConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"time_window":      types.StringType,
		"keypath_to_track": types.StringType,
	}
}

func undetectedValuesManagementAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"trigger_undetected_values": types.BoolType,
		"auto_retire_timeframe":     types.StringType,
	}
}

func logsUniqueCountAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_filter":                       types.ObjectType{AttrTypes: logsFilterAttr()},
		"notification_payload_filter":       types.SetType{ElemType: types.StringType},
		"rules":                             types.SetType{ElemType: types.ObjectType{AttrTypes: logsUniqueCountRulesAttr()}},
		"unique_count_keypath":              types.StringType,
		"max_unique_count_per_group_by_key": types.Int64Type,
	}
}

func logsUniqueCountRulesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{AttrTypes: logsUniqueCountConditionAttr()},
	}
}

func logsUniqueCountConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"max_unique_count": types.Int64Type,
		"time_window":      types.StringType,
	}
}

func alertScheduleAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"active_on": types.ObjectType{
			AttrTypes: alertScheduleActiveOnAttr(),
		},
	}
}

func alertScheduleActiveOnAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"days_of_week": types.SetType{
			ElemType: types.StringType,
		},
		"start_time": types.StringType,
		"end_time":   types.StringType,
		"utc_offset": types.StringType,
	}
}

func timeOfDayAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"hours":   types.Int64Type,
		"minutes": types.Int64Type,
	}
}

func logsTimeRelativeAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_filter":                  types.ObjectType{AttrTypes: logsFilterAttr()},
		"notification_payload_filter":  types.SetType{ElemType: types.StringType},
		"undetected_values_management": types.ObjectType{AttrTypes: undetectedValuesManagementAttr()},
		"rules":                        types.SetType{ElemType: types.ObjectType{AttrTypes: logsTimeRelativeRulesAttr()}},
	}
}

func logsTimeRelativeRulesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{
			AttrTypes: logsTimeRelativeConditionAttr(),
		},
		"override": types.ObjectType{
			AttrTypes: alertOverrideAttr(),
		},
	}
}

func logsTimeRelativeConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"threshold":      types.Float64Type,
		"compared_to":    types.StringType,
		"condition_type": types.StringType,
	}
}

func metricThresholdAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"metric_filter":                types.ObjectType{AttrTypes: metricFilterAttr()},
		"undetected_values_management": types.ObjectType{AttrTypes: undetectedValuesManagementAttr()},
		"rules":                        types.SetType{ElemType: types.ObjectType{AttrTypes: metricThresholdRulesAttr()}},
		"missing_values":               types.ObjectType{AttrTypes: missingValuesAttr()},
	}
}

func missingValuesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"replace_with_zero":       types.BoolType,
		"min_non_null_values_pct": types.Int64Type,
	}
}

func metricThresholdRulesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{
			AttrTypes: metricThresholdConditionAttr(),
		},
		"override": types.ObjectType{
			AttrTypes: alertOverrideAttr(),
		},
	}
}

func metricThresholdConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"threshold":      types.Float64Type,
		"for_over_pct":   types.Int64Type,
		"of_the_last":    types.StringType,
		"condition_type": types.StringType,
	}
}

func metricFilterAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"promql": types.StringType,
	}
}

func metricAnomalyAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"metric_filter": types.ObjectType{AttrTypes: metricFilterAttr()},
		"rules":         types.SetType{ElemType: types.ObjectType{AttrTypes: metricAnomalyRulesAttr()}},
	}
}

func metricAnomalyRulesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{AttrTypes: metricAnomalyConditionAttr()},
	}
}

func metricAnomalyConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"min_non_null_values_pct": types.Int64Type,
		"threshold":               types.Float64Type,
		"for_over_pct":            types.Int64Type,
		"of_the_last":             types.StringType,
		"condition_type":          types.StringType,
	}
}

func tracingImmediateAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"tracing_filter":              types.ObjectType{AttrTypes: tracingQueryAttr()},
		"notification_payload_filter": types.SetType{ElemType: types.StringType},
	}
}

func tracingThresholdAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"tracing_filter":              types.ObjectType{AttrTypes: tracingQueryAttr()},
		"notification_payload_filter": types.SetType{ElemType: types.StringType},
		"rules":                       types.SetType{ElemType: types.ObjectType{AttrTypes: tracingThresholdRulesAttr()}},
	}
}

func tracingThresholdRulesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ObjectType{AttrTypes: tracingThresholdConditionAttr()},
	}
}

func tracingThresholdConditionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"span_amount":    types.Float64Type,
		"time_window":    types.StringType,
		"condition_type": types.StringType,
	}
}

func flowAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"stages": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: flowStageAttr(),
			},
		},
		"enforce_suppression": types.BoolType,
	}
}

func flowStageAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"flow_stages_groups": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: flowStageGroupAttr(),
			},
		},
		"timeframe_ms":   types.Int64Type,
		"timeframe_type": types.StringType,
	}
}

func flowStageGroupAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"alert_defs": types.SetType{
			ElemType: types.ObjectType{
				AttrTypes: alertDefsAttr(),
			},
		},
		"next_op":   types.StringType,
		"alerts_op": types.StringType,
	}
}

func alertDefsAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"id":  types.StringType,
		"not": types.BoolType,
	}
}

func tracingQueryAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"latency_threshold_ms":  types.NumberType,
		"tracing_label_filters": types.ObjectType{AttrTypes: tracingLabelFiltersAttr()},
	}
}

func labelFilterTypesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"value":     types.StringType,
		"operation": types.StringType,
	}
}

func tracingLabelFiltersAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"application_name": types.SetType{ElemType: types.ObjectType{AttrTypes: tracingFiltersTypeAttr()}},
		"subsystem_name":   types.SetType{ElemType: types.ObjectType{AttrTypes: tracingFiltersTypeAttr()}},
		"service_name":     types.SetType{ElemType: types.ObjectType{AttrTypes: tracingFiltersTypeAttr()}},
		"operation_name":   types.SetType{ElemType: types.ObjectType{AttrTypes: tracingFiltersTypeAttr()}},
		"span_fields":      types.SetType{ElemType: types.ObjectType{AttrTypes: tracingSpanFieldsFilterAttr()}},
	}
}

func tracingFiltersTypeAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"operation": types.StringType,
		"values":    types.SetType{ElemType: types.StringType},
	}
}

func tracingSpanFieldsFilterAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"key":         types.StringType,
		"filter_type": types.ObjectType{AttrTypes: tracingFiltersTypeAttr()},
	}
}

func (r *AlertResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan *AlertResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	alertProperties, diags := extractAlertProperties(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	updateAlertReq := &cxsdk.ReplaceAlertDefRequest{
		Id:                 typeStringToWrapperspbString(plan.ID),
		AlertDefProperties: alertProperties,
	}
	log.Printf("[INFO] Updating Alert: %s", protojson.Format(updateAlertReq))
	alertUpdateResp, err := r.client.Replace(ctx, updateAlertReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating Alert",
			formatRpcErrors(err, updateAlertURL, protojson.Format(updateAlertReq)),
		)
		return
	}
	log.Printf("[INFO] Submitted updated Alert: %s", protojson.Format(alertUpdateResp))

	// Get refreshed Alert value from Coralogix
	getAlertReq := &cxsdk.GetAlertDefRequest{Id: typeStringToWrapperspbString(plan.ID)}
	getAlertResp, err := r.client.Get(ctx, getAlertReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Alert %q is in state, but no longer exists in Coralogix backend", plan.ID.ValueString()),
				fmt.Sprintf("%s will be recreated when you apply", plan.ID.ValueString()),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Alert",
				formatRpcErrors(err, getAlertURL, protojson.Format(getAlertReq)),
			)
		}
		return
	}
	log.Printf("[INFO] Received Alert: %s", protojson.Format(getAlertResp))

	plan, diags = flattenAlert(ctx, getAlertResp.GetAlertDef(), &plan.Schedule)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *AlertResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state AlertResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	log.Printf("[INFO] Delteting Alert %s", id)
	deleteReq := &cxsdk.DeleteAlertDefRequest{Id: wrapperspb.String(id)}
	log.Printf("[INFO] Deleting Alert: %s", protojson.Format(deleteReq))
	if _, err := r.client.Delete(ctx, deleteReq); err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting Alert %s", id),
			formatRpcErrors(err, deleteAlertURL, protojson.Format(deleteReq)),
		)
		return
	}
	log.Printf("[INFO] Alert %s deleted", id)
}
