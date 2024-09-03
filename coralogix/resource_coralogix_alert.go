package coralogix

import (
	"context"
	"fmt"
	"log"
	"strconv"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"

	"github.com/hashicorp/terraform-plugin-framework-validators/boolvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/helpers/validatordiag"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
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

var (
	_              resource.ResourceWithConfigure   = &AlertResource{}
	_              resource.ResourceWithImportState = &AlertResource{}
	createAlertURL                                  = cxsdk.CreateAlertDefRpc
	updateAlertURL                                  = cxsdk.ReplaceAlertDefRpc
	getAlertURL                                     = cxsdk.GetAlertDefRpc
	deleteAlertURL                                  = cxsdk.DeleteAlertDefRpc

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

	// DELETEME
	// evaluationWindowTypeProtoToSchemaMap = map[cxsdk.AlertEvaluationWindow]string{
	// 	cxsdk.AlertEvaluationWindowRollingOrUnspecified: "Rolling",
	// 	cxsdk.AlertEvaluationWindowDynamic:              "Dynamic",
	// }
	// evaluationWindowTypeSchemaToProtoMap = ReverseMap(evaluationWindowTypeProtoToSchemaMap)
	// validEvaluationWindowTypes           = GetKeys(evaluationWindowTypeSchemaToProtoMap)

	logsTimeWindowValueProtoToSchemaMap = map[cxsdk.LogsTimeWindowValue]string{
		cxsdk.LogsTimeWindow5MinutesOrUnspecified: "5_MINUTES",
		cxsdk.LogsTimeWindow10Minutes:             "10_MINUTES",
		cxsdk.LogsTimeWindow15Minutes:             "15_MINUTES",
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
		cxsdk.AutoRetireTimeframeNeverOrUnspecified: "Never",
		cxsdk.AutoRetireTimeframe5Minutes:           "5_Minutes",
		cxsdk.AutoRetireTimeframe10Minutes:          "10_Minutes",
		cxsdk.AutoRetireTimeframe1Hour:              "1_Hour",
		cxsdk.AutoRetireTimeframe2Hours:             "2_Hours",
		cxsdk.AutoRetireTimeframe6Hours:             "6_Hours",
		cxsdk.AutoRetireTimeframe12Hours:            "12_Hours",
		cxsdk.AutoRetireTimeframe24Hours:            "24_Hours",
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
		cxsdk.LogsNewValueTimeWindowValue_3Months:             "3_MONTHS",
	}
	logsNewValueTimeWindowValueSchemaToProtoMap = ReverseMap(logsNewValueTimeWindowValueProtoToSchemaMap)
	validLogsNewValueTimeWindowValues           = GetKeys(logsNewValueTimeWindowValueSchemaToProtoMap)

	logsUniqueCountTimeWindowValueProtoToSchemaMap = map[cxsdk.LogsUniqueValueTimeWindowValue]string{
		cxsdk.LogsUniqueValueTimeWindowValue1MinuteOrUnspecified: "1_MINUTE",
		cxsdk.LogsUniqueValueTimeWindowValue15Minutes:            "5_MINUTES",
		cxsdk.LogsUniqueValueTimeWindowValue20Minutes:            "20_MINUTES",
		cxsdk.LogsUniqueValueTimeWindowValue30Minutes:            "30_MINUTES",
		cxsdk.LogsUniqueValueTimeWindowValue1Hour:                "1_HOUR",
		cxsdk.LogsUniqueValueTimeWindowValue2Hours:               "2_HOURS",
		cxsdk.LogsUniqueValueTimeWindowValue4Hours:               "4_HOURS",
		cxsdk.LogsUniqueValueTimeWindowValue6Hours:               "6_HOURS",
		cxsdk.LogsUniqueValueTimeWindowValue12Hours:              "12_HOURS",
		cxsdk.LogsUniqueValueTimeWindowValue24Hours:              "24_HOURS",
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
		cxsdk.MetricTimeWindowValue30Minutes:            "30_MINUTES",
		cxsdk.MetricTimeWindowValue1Hour:                "1_HOUR",
		cxsdk.MetricTimeWindowValue2Hours:               "2_HOURS",
		cxsdk.MetricTimeWindowValue4Hours:               "4_HOURS",
		cxsdk.MetricTimeWindowValue6Hours:               "6_HOURS",
		cxsdk.MetricTimeWindowValue12Hours:              "12_HOURS",
		cxsdk.MetricTimeWindowValue24Hours:              "24_HOURS",
	}
	metricTimeWindowValueSchemaToProtoMap = ReverseMap(metricFilterOperationTypeProtoToSchemaMap)
	validMetricTimeWindowValues           = GetKeys(metricTimeWindowValueSchemaToProtoMap)

	tracingTimeWindowProtoToSchemaMap = map[cxsdk.TracingTimeWindowValue]string{
		cxsdk.TracingTimeWindowValue5MinutesOrUnspecified: "5_MINUTES",
		cxsdk.TracingTimeWindowValue10Minutes:             "10_MINUTES",
		cxsdk.TracingTimeWindowValue15Minutes:             "15_MINUTES",
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
		cxsdk.TracingFilterOperationTypeIncludes:        "NOT",
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
	logsThresholdConditionValues = GetValues(logsThresholdConditionMap)

	logsRatioConditionConditionMap = map[cxsdk.LogsRatioConditionType]string{
		cxsdk.LogsRatioConditionTypeMoreThanOrUnspecified: "MORE_THAN",
		cxsdk.LogsRatioConditionTypeLessThan:              "LESS_THAN",
	}
	logsRatioConditionConditionMapValues = GetValues(logsRatioConditionConditionMap)
)

func NewAlertResource() resource.Resource {
	return &AlertResource{}
}

type AlertResource struct {
	client *cxsdk.AlertsClient
}

type AlertResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Enabled     types.Bool   `tfsdk:"enabled"`
	Priority    types.String `tfsdk:"priority"`
	Schedule    types.Object `tfsdk:"schedule"` // AlertScheduleModel
	Type        types.Object `tfsdk:"type"`     // AlertTypeDefinitionModel

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
	LogsUnusual               types.Object `tfsdk:"logs_unusual"`                 // LogsUnusualModel
	LogsRatioThreshold        types.Object `tfsdk:"logs_ratio_threshold"`         // LogsRatioThresholdModel
	LogsNewValue              types.Object `tfsdk:"logs_new_value"`               // LogsNewValueModel
	LogsUniqueCount           types.Object `tfsdk:"logs_unique_count"`            // LogsUniqueCountModel
	LogsTimeRelativeThreshold types.Object `tfsdk:"logs_time_relative_threshold"` // LogsTimeRelativeThresholdModel
	LogsMetricThreshold       types.Object `tfsdk:"logs_metric_threshold"`        // LogsMetricThresholdModel
	LogsMetricUnusual         types.Object `tfsdk:"logs_metric_unusual"`          // LogsMetricUnusualModel
	LogsTracingImmediate      types.Object `tfsdk:"logs_tracing_immediate"`       // LogsTracingImmediateModel
	LogsTracingThreshold      types.Object `tfsdk:"logs_tracing_threshold"`       // LogsTracingThresholdModel
}

type IncidentsSettingsModel struct {
	NotifyOn           types.String `tfsdk:"notify_on"`
	RetriggeringPeriod types.Object `tfsdk:"retriggering_period"` // RetriggeringPeriodModel
}

type NotificationGroupModel struct {
	GroupByFields          types.List `tfsdk:"group_by_fields"`          // []types.String
	AdvancedTargetSettings types.Set  `tfsdk:"advanced_target_settings"` // AdvancedTargetSettingsModel
	SimpleTargetSettings   types.Set  `tfsdk:"simple_target_settings"`   // SimpleTargetSettingsModel
}

type AdvancedTargetSettingsModel struct {
	RetriggeringPeriod types.Object `tfsdk:"retriggering_period"` // RetriggeringPeriodModel
	NotifyOn           types.String `tfsdk:"notify_on"`
	IntegrationID      types.String `tfsdk:"integration_id"`
	Recipients         types.Set    `tfsdk:"recipients"` //[]types.String
}

type SimpleTargetSettingsModel struct {
	IntegrationID types.String `tfsdk:"integration_id"`
	Recipients    types.Set    `tfsdk:"recipients"` //[]types.String
}

type ActiveOnModel struct {
	DaysOfWeek types.List   `tfsdk:"days_of_week"` // []types.String
	StartTime  types.Object `tfsdk:"start_time"`   // TimeOfDayModel
	EndTime    types.Object `tfsdk:"end_time"`     // TimeOfDayModel
}

type TimeOfDayModel struct {
	Hours   types.Int64 `tfsdk:"hours"`
	Minutes types.Int64 `tfsdk:"minutes"`
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
	Rules                      types.List   `tfsdk:"rules"`                        // []RuleModel
	LogsFilter                 types.Object `tfsdk:"logs_filter"`                  // AlertsLogsFilterModel
	NotificationPayloadFilter  types.Set    `tfsdk:"notification_payload_filter"`  // []types.String
	UndetectedValuesManagement types.Object `tfsdk:"undetected_values_management"` // UndetectedValuesManagementModel
}

type LogsUnusualModel struct {
	Rules                     types.List   `tfsdk:"rules"`                       // []RuleModel
	LogsFilter                types.Object `tfsdk:"logs_filter"`                 // AlertsLogsFilterModel
	NotificationPayloadFilter types.Set    `tfsdk:"notification_payload_filter"` // []types.String
}

type LogsRatioThresholdModel struct {
	Rules                     types.List   `tfsdk:"rules"`     // []RuleModel
	Numerator                 types.Object `tfsdk:"numerator"` // AlertsLogsFilterModel
	NumeratorAlias            types.String `tfsdk:"numerator_alias"`
	Denominator               types.Object `tfsdk:"denominator"` // AlertsLogsFilterModel
	DenominatorAlias          types.String `tfsdk:"denominator_alias"`
	NotificationPayloadFilter types.Set    `tfsdk:"notification_payload_filter"` // []types.String
	GroupByFor                types.String `tfsdk:"group_by_for"`
}

type LogsNewValueModel struct {
	Rules                     types.List   `tfsdk:"rules"`                       // []NewValueRulesModel
	LogsFilter                types.Object `tfsdk:"logs_filter"`                 // AlertsLogsFilterModel
	NotificationPayloadFilter types.Set    `tfsdk:"notification_payload_filter"` // []types.String
}

type LogsUniqueCountModel struct {
	Rules                     types.List   `tfsdk:"rules"`                       // []LogsUniqueCountRulesModel
	LogsFilter                types.Object `tfsdk:"logs_filter"`                 // AlertsLogsFilterModel
	NotificationPayloadFilter types.Set    `tfsdk:"notification_payload_filter"` // []types.String
}

type LogsUniqueCountRulesModel struct {
	MaxUniqueCountPerGroupByKey types.Int64  `tfsdk:"max_unique_count_per_group_by_key"`
	MaxUniqueCount              types.Int64  `tfsdk:"max_unique_count"`
	TimeWindow                  types.Object `tfsdk:"time_window"` // LogsUniqueCountTimeWindowModel
	UniqueCountKeypath          types.String `tfsdk:"unique_count_keypath"`
}

type LogsTimeRelativeThresholdModel struct {
	Rules                      types.List   `tfsdk:"rules"`                        // []RuleModel
	LogsFilter                 types.Object `tfsdk:"logs_filter"`                  // AlertsLogsFilterModel
	NotificationPayloadFilter  types.Set    `tfsdk:"notification_payload_filter"`  // []types.String
	UndetectedValuesManagement types.Object `tfsdk:"undetected_values_management"` // UndetectedValuesManagementModel
}

type MetricThresholdModel struct {
	Rules                      types.List   `tfsdk:"rules"`                        // []MetricRule
	MetricFilter               types.Object `tfsdk:"metric_filter"`                // MetricFilterModel
	UndetectedValuesManagement types.Object `tfsdk:"undetected_values_management"` // UndetectedValuesManagementModel
}

type MetricRule struct {
	Threshold           types.Float64 `tfsdk:"threshold"`
	ForOverPct          types.Int64   `tfsdk:"for_over_pct"`
	OfTheLast           types.Object  `tfsdk:"of_the_last"` // MetricTimeWindowModel
	Condition           types.String  `tfsdk:"condition"`
	MinNonNullValuesPct types.Int32   `tfsdk:"min_non_null_values_pct"`
	MissingValues       types.Object  `tfsdk:"missing_values"` // MetricMissingValuesModel
}

// DELETEME
// type MetricLessThanModel struct {
// 	MetricFilter               types.Object  `tfsdk:"metric_filter"`  // MetricFilterModel
// 	OfTheLast                  types.Object  `tfsdk:"of_the_last"`    // MetricTimeWindowModel
// 	MissingValues              types.Object  `tfsdk:"missing_values"` // MetricMissingValuesModel
// 	Threshold                  types.Float64 `tfsdk:"threshold"`
// 	ForOverPct                 types.Int64   `tfsdk:"for_over_pct"`
// }

type MetricUnusualModel struct {
	MetricFilter types.Object `tfsdk:"metric_filter"` // MetricFilterModel
	Rules        types.List   `tfsdk:"rules"`         // []MetricRule
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
	TimeWindow                types.Object `tfsdk:"time_window"`                 // TracingTimeWindowModel
	SpanAmount                types.Int64  `tfsdk:"span_amount"`
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
	AlertDefs types.List   `tfsdk:"alert_defs"` // FlowStagesGroupsAlertDefsModel
	NextOp    types.String `tfsdk:"next_op"`
	AlertsOp  types.String `tfsdk:"alerts_op"`
}

type FlowStagesGroupsAlertDefsModel struct {
	Id  types.String `tfsdk:"id"`
	Not types.Bool   `tfsdk:"not"`
}

// DELETEME
// type MetricLessThanUsualModel struct {
// 	MetricFilter        types.Object `tfsdk:"metric_filter"` // MetricFilterModel
// 	OfTheLast           types.Object `tfsdk:"of_the_last"`   // MetricTimeWindowModel
// 	Threshold           types.Int64  `tfsdk:"threshold"`
// 	ForOverPct          types.Int64  `tfsdk:"for_over_pct"`
// 	MinNonNullValuesPct types.Int64  `tfsdk:"min_non_null_values_pct"`
// }

// type MetricMoreThanOrEqualsModel struct {
// 	MetricFilter  types.Object  `tfsdk:"metric_filter"` // MetricFilterModel
// 	Threshold     types.Float64 `tfsdk:"threshold"`
// 	ForOverPct    types.Int64   `tfsdk:"for_over_pct"`
// 	OfTheLast     types.Object  `tfsdk:"of_the_last"`    // MetricTimeWindowModel
// 	MissingValues types.Object  `tfsdk:"missing_values"` // MetricMissingValuesModel
// }

// type MetricLessThanOrEqualsModel struct {
// 	MetricFilter               types.Object  `tfsdk:"metric_filter"`  // MetricFilterModel
// 	OfTheLast                  types.Object  `tfsdk:"of_the_last"`    // MetricTimeWindowModel
// 	MissingValues              types.Object  `tfsdk:"missing_values"` // MetricMissingValuesModel
// 	Threshold                  types.Float64 `tfsdk:"threshold"`
// 	ForOverPct                 types.Int64   `tfsdk:"for_over_pct"`                 // MetricMissingValuesModel
// 	UndetectedValuesManagement types.Object  `tfsdk:"undetected_values_management"` // UndetectedValuesManagementModel
// }

type AlertsLogsFilterModel struct {
	SimpleFilter types.Object `tfsdk:"simple_filter"` // LuceneFilterModel
}

type LogsTimeWindowModel struct {
	SpecificValue types.String `tfsdk:"specific_value"`
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

type LogsRatioTimeWindowModel struct {
	SpecificValue types.String `tfsdk:"specific_value"`
}

type LogsNewValueTimeWindowModel struct {
	SpecificValue types.String `tfsdk:"specific_value"`
}

type LogsUniqueCountTimeWindowModel struct {
	SpecificValue types.String `tfsdk:"specific_value"`
}

type MetricFilterModel struct {
	Promql types.String `tfsdk:"promql"`
}

type MetricTimeWindowModel struct {
	SpecificValue types.String `tfsdk:"specific_value"`
}

type MetricMissingValuesModel struct {
	ReplaceWithZero     types.Bool  `tfsdk:"replace_with_zero"`
	MinNonNullValuesPct types.Int64 `tfsdk:"min_non_null_values_pct"`
}

type NewValueRulesModel struct {
	TimeWindow     types.Object `tfsdk:"time_window"` // LogsTimeWindowModel
	KeypathToTrack types.String `tfsdk:"keypath_to_track"`
}

type RuleModel struct {
	CompareTo      types.String  `tfsdk:"compare_to"`
	Condition      types.String  `tfsdk:"condition"`
	Threshold      types.Float64 `tfsdk:"threshold"`
	TimeWindow     types.Object  `tfsdk:"time_window"` // LogsTimeWindowModel
	IgnoreInfinity types.Bool    `tfsdk:"ignore_infinity"`
}

type TracingFilterModel struct {
	LatencyThresholdMs  types.Int64  `tfsdk:"latency_threshold_ms"`
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

type TracingTimeWindowModel struct {
	SpecificValue types.String `tfsdk:"specific_value"`
}

func (r *AlertResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alert"
}

func (r *AlertResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clientSet, ok := req.ProviderData.(*cxsdk.ClientSet)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *cxsdk.ClientSet, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = clientSet.Alerts()
}

type advancedTargetSettingsPlanModifier struct{}

func (a advancedTargetSettingsPlanModifier) Description(ctx context.Context) string {
	return "Advanced target settings."
}

func (a advancedTargetSettingsPlanModifier) MarkdownDescription(ctx context.Context) string {
	return "Advanced target settings."
}

func (a advancedTargetSettingsPlanModifier) PlanModifyObject(ctx context.Context, request planmodifier.ObjectRequest, response *planmodifier.ObjectResponse) {
	if !request.ConfigValue.IsUnknown() {
		return
	}

	response.PlanValue = request.StateValue
}

type requiredWhenGroupBySet struct {
}

func (r requiredWhenGroupBySet) Description(ctx context.Context) string {
	return "Required when group_by is set."
}

func (r requiredWhenGroupBySet) MarkdownDescription(ctx context.Context) string {
	return "Required when group_by is set."
}

func (r requiredWhenGroupBySet) ValidateInt64(ctx context.Context, req validator.Int64Request, resp *validator.Int64Response) {
	if !req.ConfigValue.IsNull() {
		return
	}

	var groupBy types.Set
	diags := req.Config.GetAttribute(ctx, path.Root("group_by"), &groupBy)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	if !(groupBy.IsNull() || groupBy.IsUnknown()) {
		resp.Diagnostics.Append(validatordiag.InvalidAttributeCombinationDiagnostic(
			req.Path,
			fmt.Sprintf("Attribute %q must be specified when %q is specified", req.Path, "group_by"),
		))
	}
}

func (r *AlertResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 1,
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
							"days_of_week": schema.ListAttribute{
								Required:    true,
								ElementType: types.StringType,
								Validators: []validator.List{
									listvalidator.ValueStringsAre(
										stringvalidator.OneOf(validDaysOfWeek...),
									),
								},
								MarkdownDescription: fmt.Sprintf("Days of the week. Valid values: %q.", validDaysOfWeek),
							},
							"start_time": timeOfDaySchema(),
							"end_time":   timeOfDaySchema(),
						},
					},
				},
				MarkdownDescription: "Alert schedule. Will be activated all the time if not specified.",
			},
			"type_definition": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"logs_immediate": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"logs_filter":                 logsFilterSchema(),
							"notification_payload_filter": notificationPayloadFilterSchema(),
						},
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("logs_threshold"),
								path.MatchRelative().AtParent().AtName("logs_unusual"),
								path.MatchRelative().AtParent().AtName("logs_ratio_threshold"),
								path.MatchRelative().AtParent().AtName("logs_new_value"),
								path.MatchRelative().AtParent().AtName("logs_unique_count"),
								path.MatchRelative().AtParent().AtName("logs_time_relative_threshold"),
								path.MatchRelative().AtParent().AtName("metric_threshold"),
								path.MatchRelative().AtParent().AtName("metric_unusual"),
								path.MatchRelative().AtParent().AtName("tracing_immediate"),
								path.MatchRelative().AtParent().AtName("tracing_threshold"),
								path.MatchRelative().AtParent().AtName("flow"),
							),
						},
					},
					"logs_threshold": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"rules": schema.ListNestedAttribute{
								Required:    true,
								Validators: []validator.List{listvalidator.LengthAtLeast(1)},
								Attributes:  map[string]schema.Attribute{
									"threshold": schema.Float64Attribute{
										Required: true,
									},
									"time_window":                 logsTimeWindowSchema(),
									"condition": schema.StringAttribute{
										Required: true,
										Validators: []validator.String{
											stringvalidator.OneOf(logsThresholdConditionValues...),
										},
										MarkdownDescription: fmt.Sprintf("Condition to evaluate the threshold with. Valid values: %q.", logsThresholdConditionValues),
									},
								},
							"notification_payload_filter": notificationPayloadFilterSchema(),
							"logs_filter":                 logsFilterSchema(),
							// "evaluation_window": schema.StringAttribute{
							// 	Optional: true,
							// 	Computed: true,
							// 	Default:  stringdefault.StaticString("Rolling"),
							// 	Validators: []validator.String{
							// 		stringvalidator.OneOf(validEvaluationWindowTypes...),
							// 	},
							// 	MarkdownDescription: fmt.Sprintf("Evaluation window type. Valid values: %q.", validEvaluationWindowTypes),
							// },
						},
					},
					// "logs_less_than": schema.SingleNestedAttribute{
					// 	Optional: true,
					// 	Attributes: map[string]schema.Attribute{
					// 		"logs_filter":                 logsFilterSchema(),
					// 		"notification_payload_filter": notificationPayloadFilterSchema(),
					// 		"time_window":                 logsTimeWindowSchema(),
					// 		"threshold": schema.Int64Attribute{
					// 			Required: true,
					// 		},
					// 		"undetected_values_management": undetectedValuesManagementSchema(),
					// 	},
					// },
					// "logs_more_than_usual": schema.SingleNestedAttribute{
					// 	Optional: true,
					// 	Attributes: map[string]schema.Attribute{
					// 		"minimum_threshold": schema.Int64Attribute{
					// 			Required: true,
					// 		},
					// 		"time_window":                 logsTimeWindowSchema(),
					// 		"logs_filter":                 logsFilterSchema(),
					// 		"notification_payload_filter": notificationPayloadFilterSchema(),
					// 	},
					// },
					// "logs_ratio_more_than": schema.SingleNestedAttribute{
					// 	Optional: true,
					// 	Attributes: map[string]schema.Attribute{
					// 		"numerator_logs_filter": logsFilterSchema(),
					// 		"numerator_alias": schema.StringAttribute{
					// 			Required: true,
					// 		},
					// 		"denominator_logs_filter": logsFilterSchema(),
					// 		"denominator_alias": schema.StringAttribute{
					// 			Required: true,
					// 		},
					// 		"threshold": schema.Int64Attribute{
					// 			Required: true,
					// 		},
					// 		"time_window": logsRatioTimeWindowSchema(),
					// 		"ignore_infinity": schema.BoolAttribute{
					// 			Optional: true,
					// 			Computed: true,
					// 			Default:  booldefault.StaticBool(false),
					// 		},
					// 		"notification_payload_filter": notificationPayloadFilterSchema(),
					// 		"group_by_for":                logsRatioGroupByForSchema(),
					// 	},
					// },
					// "logs_ratio_less_than": schema.SingleNestedAttribute{
					// 	Optional: true,
					// 	Attributes: map[string]schema.Attribute{
					// 		"numerator_logs_filter": logsFilterSchema(),
					// 		"numerator_alias": schema.StringAttribute{
					// 			Required: true,
					// 		},
					// 		"denominator_logs_filter": logsFilterSchema(),
					// 		"denominator_alias": schema.StringAttribute{
					// 			Required: true,
					// 		},
					// 		"threshold": schema.Int64Attribute{
					// 			Required: true,
					// 		},
					// 		"time_window": logsRatioTimeWindowSchema(),
					// 		"ignore_infinity": schema.BoolAttribute{
					// 			Optional: true,
					// 			Computed: true,
					// 			Default:  booldefault.StaticBool(false),
					// 		},
					// 		"notification_payload_filter":  notificationPayloadFilterSchema(),
					// 		"group_by_for":                 logsRatioGroupByForSchema(),
					// 		"undetected_values_management": undetectedValuesManagementSchema(),
					// 	},
					// },
					// "logs_new_value": schema.SingleNestedAttribute{
					// 	Optional: true,
					// 	Attributes: map[string]schema.Attribute{
					// 		"logs_filter":                 logsFilterSchema(),
					// 		"keypath_to_track":            schema.StringAttribute{Required: true},
					// 		"time_window":                 logsNewValueTimeWindowSchema(),
					// 		"notification_payload_filter": notificationPayloadFilterSchema(),
					// 	},
					// 	Validators: []validator.Object{
					// 		objectvalidator.ConflictsWith(path.MatchRoot("group_by")),
					// 	},
					// },
					// "logs_unique_count": schema.SingleNestedAttribute{
					// 	Optional: true,
					// 	Attributes: map[string]schema.Attribute{
					// 		"logs_filter":                 logsFilterSchema(),
					// 		"notification_payload_filter": notificationPayloadFilterSchema(),
					// 		"time_window":                 logsUniqueCountTimeWindowSchema(),
					// 		"unique_count_keypath":        schema.StringAttribute{Required: true},
					// 		"max_unique_count":            schema.Int64Attribute{Required: true},
					// 		"max_unique_count_per_group_by_key": schema.Int64Attribute{
					// 			Optional: true,
					// 			Validators: []validator.Int64{
					// 				int64validator.AlsoRequires(path.MatchRoot("group_by")),
					// 				requiredWhenGroupBySet{},
					// 			},
					// 		},
					// 	},
					// },
					// "logs_time_relative_more_than": schema.SingleNestedAttribute{
					// 	Optional: true,
					// 	Attributes: map[string]schema.Attribute{
					// 		"logs_filter":                 logsFilterSchema(),
					// 		"notification_payload_filter": notificationPayloadFilterSchema(),
					// 		"threshold":                   schema.Int64Attribute{Required: true},
					// 		"compared_to":                 timeRelativeCompareTo(),
					// 		"ignore_infinity": schema.BoolAttribute{
					// 			Optional: true,
					// 			Computed: true,
					// 			Default:  booldefault.StaticBool(false),
					// 		},
					// 	},
					// },
					// "logs_time_relative_less_than": schema.SingleNestedAttribute{
					// 	Optional: true,
					// 	Attributes: map[string]schema.Attribute{
					// 		"logs_filter":                 logsFilterSchema(),
					// 		"threshold":                   schema.Int64Attribute{Required: true},
					// 		"notification_payload_filter": notificationPayloadFilterSchema(),
					// 		"compared_to": schema.StringAttribute{
					// 			Required: true,
					// 			Validators: []validator.String{
					// 				stringvalidator.OneOf(validLogsTimeRelativeComparedTo...),
					// 			},
					// 			MarkdownDescription: fmt.Sprintf("Compared to. Valid values: %q.", validLogsTimeRelativeComparedTo),
					// 		},
					// 		"ignore_infinity": schema.BoolAttribute{
					// 			Optional: true,
					// 			Computed: true,
					// 			Default:  booldefault.StaticBool(false),
					// 		},
					// 		"undetected_values_management": undetectedValuesManagementSchema(),
					// 	},
					// },
					// "metric_more_than": schema.SingleNestedAttribute{
					// 	Optional: true,
					// 	Attributes: map[string]schema.Attribute{
					// 		"metric_filter": metricFilterSchema(),
					// 		"threshold": schema.Float64Attribute{
					// 			Required: true,
					// 		},
					// 		"for_over_pct": schema.Int64Attribute{
					// 			Required: true,
					// 		},
					// 		"of_the_last":    metricTimeWindowSchema(),
					// 		"missing_values": missingValuesSchema(),
					// 	},
					// },
					// "metric_less_than": schema.SingleNestedAttribute{
					// 	Optional: true,
					// 	Attributes: map[string]schema.Attribute{
					// 		"metric_filter": metricFilterSchema(),
					// 		"threshold": schema.Float64Attribute{
					// 			Required: true,
					// 		},
					// 		"for_over_pct": schema.Int64Attribute{
					// 			Required: true,
					// 		},
					// 		"of_the_last":                  metricTimeWindowSchema(),
					// 		"missing_values":               missingValuesSchema(),
					// 		"undetected_values_management": undetectedValuesManagementSchema(),
					// 	},
					// },
					// "metric_less_than_usual": schema.SingleNestedAttribute{
					// 	Optional: true,
					// 	Attributes: map[string]schema.Attribute{
					// 		"metric_filter": metricFilterSchema(),
					// 		"of_the_last":   metricTimeWindowSchema(),
					// 		"threshold": schema.Int64Attribute{
					// 			Required: true,
					// 		},
					// 		"for_over_pct": schema.Int64Attribute{
					// 			Required: true,
					// 		},
					// 		"min_non_null_values_pct": schema.Int64Attribute{
					// 			Required: true,
					// 		},
					// 	},
					// },
					// "metric_more_than_usual": schema.SingleNestedAttribute{
					// 	Optional: true,
					// 	Attributes: map[string]schema.Attribute{
					// 		"metric_filter": metricFilterSchema(),
					// 		"of_the_last":   metricTimeWindowSchema(),
					// 		"threshold": schema.Int64Attribute{
					// 			Required: true,
					// 		},
					// 		"for_over_pct": schema.Int64Attribute{
					// 			Required: true,
					// 		},
					// 		"min_non_null_values_pct": schema.Int64Attribute{
					// 			Required: true,
					// 		},
					// 	},
					// },
					// "metric_more_than_or_equals": schema.SingleNestedAttribute{
					// 	Optional: true,
					// 	Attributes: map[string]schema.Attribute{
					// 		"metric_filter": metricFilterSchema(),
					// 		"threshold": schema.Float64Attribute{
					// 			Required: true,
					// 		},
					// 		"for_over_pct": schema.Int64Attribute{
					// 			Required: true,
					// 		},
					// 		"of_the_last":    metricTimeWindowSchema(),
					// 		"missing_values": missingValuesSchema(),
					// 	},
					// },
					// "metric_less_than_or_equals": schema.SingleNestedAttribute{
					// 	Optional: true,
					// 	Attributes: map[string]schema.Attribute{
					// 		"metric_filter": metricFilterSchema(),
					// 		"threshold": schema.Float64Attribute{
					// 			Required: true,
					// 		},
					// 		"for_over_pct": schema.Int64Attribute{
					// 			Required: true,
					// 		},
					// 		"of_the_last":                  metricTimeWindowSchema(),
					// 		"missing_values":               missingValuesSchema(),
					// 		"undetected_values_management": undetectedValuesManagementSchema(),
					// 	},
					// },
					// "tracing_immediate": schema.SingleNestedAttribute{
					// 	Optional: true,
					// 	Attributes: map[string]schema.Attribute{
					// 		"tracing_filter":               tracingQuerySchema(),
					// 		"notification_payload_filter": notificationPayloadFilterSchema(),
					// 	},
					// },
					// "tracing_more_than": schema.SingleNestedAttribute{
					// 	Optional: true,
					// 	Attributes: map[string]schema.Attribute{
					// 		"tracing_filter":               tracingQuerySchema(),
					// 		"notification_payload_filter": notificationPayloadFilterSchema(),
					// 		"time_window":                 tracingTimeWindowSchema(),
					// 		"span_amount": schema.Int64Attribute{
					// 			Required: true,
					// 		},
					// 	},
					// },
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
													"alert_defs": schema.ListNestedAttribute{
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
				MarkdownDescription: "Alert type definition. Exactly one of the following must be specified: logs_immediate, logs_more_than, logs_less_than, logs_more_than_usual, logs_ratio_more_than, logs_ratio_less_than, logs_new_value, logs_unique_count, logs_time_relative_more_than, logs_time_relative_less_than, metric_more_than, metric_less_than, metric_more_than_usual, metric_less_than_usual, metric_less_than_or_equals, metric_more_than_or_equals, tracing_immediate, tracing_more_than, flow.",
			},
			"group_by": schema.SetAttribute{
				Optional:            true,
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
					"group_by_fields": types.ListNull(types.StringType),
					"advanced_target_settings": types.SetNull(types.ObjectType{
						AttrTypes: advancedTargetSettingsAttr(),
					}),
					"simple_target_settings": types.SetNull(types.ObjectType{
						AttrTypes: simpleTargetSettingsAttr(),
					}),
				})),
				Attributes: map[string]schema.Attribute{
					"group_by_fields": schema.ListAttribute{
						Optional:    true,
						ElementType: types.StringType,
					},
					"advanced_target_settings": schema.SetNestedAttribute{
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
						Validators: []validator.Set{
							setvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("simple_target_settings"),
							),
						},
					},
					"simple_target_settings": schema.SetNestedAttribute{
						Optional: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
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
						},
					},
				},
			},
			"labels": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
		},
		MarkdownDescription: "Coralogix Alert. For more info please review - https://coralogix.com/docs/getting-started-with-coralogix-alerts/.",
	}
}

func timeRelativeCompareTo() schema.StringAttribute {
	return schema.StringAttribute{
		Required: true,
		Validators: []validator.String{
			stringvalidator.OneOf(validLogsTimeRelativeComparedTo...),
		},
		MarkdownDescription: fmt.Sprintf("Compared to. Valid values: %q.", validLogsTimeRelativeComparedTo),
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

func missingValuesSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Computed: true,
		PlanModifiers: []planmodifier.Object{
			objectplanmodifier.UseStateForUnknown(),
		},
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
	}
}

func tracingQuerySchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"latency_threshold_ms": schema.Int64Attribute{
				Required: true,
			},
			"tracing_label_filters": tracingLabelFiltersSchema(),
		},
	}
}

func tracingTimeWindowSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"specific_value": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(validTracingTimeWindow...),
				},
				MarkdownDescription: fmt.Sprintf("Specific value. Valid values: %q.", validTracingTimeWindow),
			},
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

func metricTimeWindowSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"specific_value": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(validMetricTimeWindowValues...),
				},
				MarkdownDescription: fmt.Sprintf("Specific value. Valid values: %q.", validMetricTimeWindowValues),
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
			"lucene_filter": schema.SingleNestedAttribute{
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

func timeOfDaySchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"hours": schema.Int64Attribute{
				Required: true,
				Validators: []validator.Int64{
					int64validator.Between(0, 23),
				},
			},
			"minutes": schema.Int64Attribute{
				Required: true,
				Validators: []validator.Int64{
					int64validator.Between(0, 59),
				},
			},
		},
	}
}

func logsTimeWindowSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"specific_value": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(validLogsTimeWindowValues...),
				},
				MarkdownDescription: fmt.Sprintf("Time window value. Valid values: %q.", validLogsTimeWindowValues),
			},
		},
	}
}

func logsRatioTimeWindowSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"specific_value": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(validLogsRatioTimeWindowValues...),
				},
				MarkdownDescription: fmt.Sprintf("Time window value. Valid values: %q.", validLogsRatioTimeWindowValues),
			},
		},
	}
}

func logsNewValueTimeWindowSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"specific_value": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(validLogsNewValueTimeWindowValues...),
				},
				MarkdownDescription: fmt.Sprintf("Time window value. Valid values: %q.", validLogsNewValueTimeWindowValues),
			},
		},
	}
}

func logsUniqueCountTimeWindowSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"specific_value": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(validLogsUniqueCountTimeWindowValues...),
				},
				MarkdownDescription: fmt.Sprintf("Time window value. Valid values: %q.", validLogsUniqueCountTimeWindowValues),
			},
		},
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
				Default:  booldefault.StaticBool(true),
			},
			"auto_retire_timeframe": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.OneOf(validAutoRetireTimeframes...),
				},
				MarkdownDescription: fmt.Sprintf("Auto retire timeframe. Valid values: %q.", validAutoRetireTimeframes),
			},
		},
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

	plan, diags = flattenAlert(ctx, alert)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
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
		GroupBy:           groupBy,
		IncidentsSettings: incidentsSettings,
		NotificationGroup: notificationGroup,
		Labels:            labels,
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
	if notificationGroupObject.IsNull() || notificationGroupObject.IsUnknown() {
		return nil, nil
	}

	var notificationGroupModel NotificationGroupModel
	if diags := notificationGroupObject.As(ctx, &notificationGroupModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	groupByFields, diags := typeStringSliceToWrappedStringSlice(ctx, notificationGroupModel.GroupByFields.Elements())
	if diags.HasError() {
		return nil, diags
	}

	notificationGroup := &cxsdk.AlertDefNotificationGroup{
		GroupByFields: groupByFields,
	}
	notificationGroup, diags = expandNotificationTargetSettings(ctx, notificationGroupModel, notificationGroup)
	if diags.HasError() {
		return nil, diags
	}

	return notificationGroup, nil
}

func expandNotificationTargetSettings(ctx context.Context, notificationGroupModel NotificationGroupModel, notificationGroup *cxsdk.AlertDefNotificationGroup) (*cxsdk.AlertDefNotificationGroup, diag.Diagnostics) {
	if advancedTargetSettings := notificationGroupModel.AdvancedTargetSettings; !(advancedTargetSettings.IsNull() || advancedTargetSettings.IsUnknown()) {
		notifications, diags := extractAdvancedTargetSettings(ctx, advancedTargetSettings)
		if diags.HasError() {
			return nil, diags
		}
		notificationGroup.Targets = notifications
	} else if simpleTargetSettings := notificationGroupModel.SimpleTargetSettings; !(simpleTargetSettings.IsNull() || simpleTargetSettings.IsUnknown()) {
		notifications, diags := extractSimpleTargetSettings(ctx, simpleTargetSettings)
		if diags.HasError() {
			return nil, diags
		}
		notificationGroup.Targets = notifications
	}

	return notificationGroup, nil
}

func extractAdvancedTargetSettings(ctx context.Context, advancedTargetSettings types.Set) (*cxsdk.AlertDefNotificationGroupAdvanced, diag.Diagnostics) {
	if advancedTargetSettings.IsNull() || advancedTargetSettings.IsUnknown() {
		return nil, nil
	}

	var advancedTargetSettingsObjects []types.Object
	diags := advancedTargetSettings.ElementsAs(ctx, &advancedTargetSettingsObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	var expandedAdvancedTargetSettings []*cxsdk.AlertDefAdvancedTargetSettings
	for _, ao := range advancedTargetSettingsObjects {
		var advancedTargetSettingsModel AdvancedTargetSettingsModel
		if dg := ao.As(ctx, &advancedTargetSettingsModel, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedAdvancedTargetSetting, expandDiags := extractAdvancedTargetSetting(ctx, advancedTargetSettingsModel)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedAdvancedTargetSettings = append(expandedAdvancedTargetSettings, expandedAdvancedTargetSetting)
	}

	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.AlertDefNotificationGroupAdvanced{
		Advanced: &cxsdk.AlertDefAdvancedTargets{
			AdvancedTargetsSettings: expandedAdvancedTargetSettings,
		},
	}, nil
}

func extractAdvancedTargetSetting(ctx context.Context, advancedTargetSettingsModel AdvancedTargetSettingsModel) (*cxsdk.AlertDefAdvancedTargetSettings, diag.Diagnostics) {
	notifyOn := notifyOnSchemaToProtoMap[advancedTargetSettingsModel.NotifyOn.ValueString()]
	advancedTargetSettings := &cxsdk.AlertDefAdvancedTargetSettings{
		NotifyOn: &notifyOn,
	}
	advancedTargetSettings, diags := expandAlertNotificationByRetriggeringPeriod(ctx, advancedTargetSettings, advancedTargetSettingsModel.RetriggeringPeriod)
	if diags.HasError() {
		return nil, diags
	}

	if !advancedTargetSettingsModel.IntegrationID.IsNull() && !advancedTargetSettingsModel.IntegrationID.IsUnknown() {
		integrationId, diag := typeStringToWrapperspbUint32(advancedTargetSettingsModel.IntegrationID)
		if diag.HasError() {
			return nil, diag
		}
		advancedTargetSettings.Integration = &cxsdk.AlertDefIntegrationType{
			IntegrationType: &cxsdk.AlertDefIntegrationTypeIntegrationID{
				IntegrationId: integrationId,
			},
		}
	} else if !advancedTargetSettingsModel.Recipients.IsNull() && !advancedTargetSettingsModel.Recipients.IsUnknown() {
		emails, diags := typeStringSliceToWrappedStringSlice(ctx, advancedTargetSettingsModel.Recipients.Elements())
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

func expandAlertNotificationByRetriggeringPeriod(ctx context.Context, alertNotification *cxsdk.AlertDefAdvancedTargetSettings, period types.Object) (*cxsdk.AlertDefAdvancedTargetSettings, diag.Diagnostics) {
	if period.IsNull() || period.IsUnknown() {
		return alertNotification, nil
	}

	var periodModel RetriggeringPeriodModel
	if diags := period.As(ctx, &periodModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if !(periodModel.Minutes.IsNull() || periodModel.Minutes.IsUnknown()) {
		alertNotification.RetriggeringPeriod = &cxsdk.AlertDefAdvancedTargetSettingsMinutes{
			Minutes: typeInt64ToWrappedUint32(periodModel.Minutes),
		}
	}

	return alertNotification, nil
}

func extractSimpleTargetSettings(ctx context.Context, simpleTargetSettings types.Set) (*cxsdk.AlertDefNotificationGroupSimple, diag.Diagnostics) {
	if simpleTargetSettings.IsNull() || simpleTargetSettings.IsUnknown() {
		return nil, nil
	}

	var simpleTargetSettingsObjects []types.Object
	diags := simpleTargetSettings.ElementsAs(ctx, &simpleTargetSettingsObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	var expandedSimpleTargetSettings []*cxsdk.AlertDefIntegrationType
	for _, ao := range simpleTargetSettingsObjects {
		var simpleTargetSettingsModel SimpleTargetSettingsModel
		if dg := ao.As(ctx, &simpleTargetSettingsModel, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedSimpleTargetSetting, expandDiags := extractSimpleTargetSetting(ctx, simpleTargetSettingsModel)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedSimpleTargetSettings = append(expandedSimpleTargetSettings, expandedSimpleTargetSetting)
	}

	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.AlertDefNotificationGroupSimple{
		Simple: &cxsdk.AlertDefTargetSimple{
			Integrations: expandedSimpleTargetSettings,
		},
	}, nil

}

func extractSimpleTargetSetting(ctx context.Context, model SimpleTargetSettingsModel) (*cxsdk.AlertDefIntegrationType, diag.Diagnostics) {
	if !model.IntegrationID.IsNull() && !model.IntegrationID.IsUnknown() {
		integrationId, diag := typeStringToWrapperspbUint32(model.IntegrationID)
		if diag.HasError() {
			return nil, diag
		}
		return &cxsdk.AlertDefIntegrationType{
			IntegrationType: &cxsdk.AlertDefIntegrationTypeIntegrationID{
				IntegrationId: integrationId,
			},
		}, nil
	} else if !model.Recipients.IsNull() && !model.Recipients.IsUnknown() {
		emails, diags := typeStringSliceToWrappedStringSlice(ctx, model.Recipients.Elements())
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.AlertDefIntegrationType{
			IntegrationType: &cxsdk.AlertDefIntegrationTypeRecipients{
				Recipients: &cxsdk.AlertDefRecipients{
					Emails: emails,
				},
			},
		}, nil
	}
	return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Integration ID is not set", "Integration ID is not set")}

}

func expandAlertsSchedule(ctx context.Context, alertProperties *cxsdk.AlertDefProperties, scheduleObject types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if scheduleObject.IsNull() || scheduleObject.IsUnknown() {
		return alertProperties, nil
	}

	var scheduleModel AlertScheduleModel
	if diags := scheduleObject.As(ctx, &scheduleModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	var diags diag.Diagnostics
	if activeOn := scheduleModel.ActiveOn; !(activeOn.IsNull() || activeOn.IsUnknown()) {
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
	if activeOnObject.IsNull() || activeOnObject.IsUnknown() {
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

	startTime, diags := extractTimeOfDay(ctx, activeOnModel.StartTime)
	if diags.HasError() {
		return nil, diags
	}

	endTime, diags := extractTimeOfDay(ctx, activeOnModel.EndTime)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.AlertDefScheduleActiveOn{
		ActiveOn: &cxsdk.AlertDefActivitySchedule{
			DayOfWeek: daysOfWeek,
			StartTime: startTime,
			EndTime:   endTime,
		},
	}, nil
}

func extractTimeOfDay(ctx context.Context, timeObject types.Object) (*cxsdk.AlertTimeOfDay, diag.Diagnostics) {
	if timeObject.IsNull() || timeObject.IsUnknown() {
		return nil, nil
	}

	var timeOfDayModel TimeOfDayModel
	if diags := timeObject.As(ctx, &timeOfDayModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	return &cxsdk.AlertTimeOfDay{
		Hours:   int32(timeOfDayModel.Hours.ValueInt64()),
		Minutes: int32(timeOfDayModel.Minutes.ValueInt64()),
	}, nil

}

func extractDaysOfWeek(ctx context.Context, daysOfWeek types.List) ([]cxsdk.AlertDayOfWeek, diag.Diagnostics) {
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
	if alertDefinition.IsNull() || alertDefinition.IsUnknown() {
		return alertProperties, nil
	}

	var alertDefinitionModel AlertTypeDefinitionModel
	if diags := alertDefinition.As(ctx, &alertDefinitionModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	var diags diag.Diagnostics
	if logsImmediate := alertDefinitionModel.LogsImmediate; !(logsImmediate.IsNull() || logsImmediate.IsUnknown()) {
		alertProperties, diags = expandLogsImmediateAlertTypeDefinition(ctx, alertProperties, logsImmediate)
	} else if logsMoreThan := alertDefinitionModel.LogsMoreThan; !(logsMoreThan.IsNull() || logsMoreThan.IsUnknown()) {
		alertProperties, diags = expandLogsMoreThanAlertTypeDefinition(ctx, alertProperties, logsMoreThan)
	} else if logsLessThan := alertDefinitionModel.LogsLessThan; !(logsLessThan.IsNull() || logsLessThan.IsUnknown()) {
		alertProperties, diags = expandLogsLessThanAlertTypeDefinition(ctx, alertProperties, logsLessThan)
	} else if logsMoreThanUsual := alertDefinitionModel.LogsMoreThanUsual; !(logsMoreThanUsual.IsNull() || logsMoreThanUsual.IsUnknown()) {
		alertProperties, diags = expandLogsMoreThanUsualAlertTypeDefinition(ctx, alertProperties, logsMoreThanUsual)
	} else if logsRatioMoreThan := alertDefinitionModel.LogsRatioMoreThan; !(logsRatioMoreThan.IsNull() || logsRatioMoreThan.IsUnknown()) {
		alertProperties, diags = expandLogsRatioMoreThanAlertTypeDefinition(ctx, alertProperties, logsRatioMoreThan)
	} else if logsRatioLessThan := alertDefinitionModel.LogsRatioLessThan; !(logsRatioLessThan.IsNull() || logsRatioLessThan.IsUnknown()) {
		alertProperties, diags = expandLogsRatioLessThanAlertTypeDefinition(ctx, alertProperties, logsRatioLessThan)
	} else if logsNewValue := alertDefinitionModel.LogsNewValue; !(logsNewValue.IsNull() || logsNewValue.IsUnknown()) {
		alertProperties, diags = expandLogsNewValueAlertTypeDefinition(ctx, alertProperties, logsNewValue)
	} else if logsUniqueCount := alertDefinitionModel.LogsUniqueCount; !(logsUniqueCount.IsNull() || logsUniqueCount.IsUnknown()) {
		alertProperties, diags = expandLogsUniqueCountAlertTypeDefinition(ctx, alertProperties, logsUniqueCount)
	} else if logsTimeRelativeMoreThan := alertDefinitionModel.LogsTimeRelativeMoreThan; !(logsTimeRelativeMoreThan.IsNull() || logsTimeRelativeMoreThan.IsUnknown()) {
		alertProperties, diags = expandLogsTimeRelativeMoreThanAlertTypeDefinition(ctx, alertProperties, logsTimeRelativeMoreThan)
	} else if logsTimeRelativeLessThan := alertDefinitionModel.LogsTimeRelativeLessThan; !(logsTimeRelativeLessThan.IsNull() || logsTimeRelativeLessThan.IsUnknown()) {
		alertProperties, diags = expandLogsTimeRelativeLessThanAlertTypeDefinition(ctx, alertProperties, logsTimeRelativeLessThan)
	} else if metricMoreThan := alertDefinitionModel.MetricMoreThan; !(metricMoreThan.IsNull() || metricMoreThan.IsUnknown()) {
		alertProperties, diags = expandMetricMoreThanAlertTypeDefinition(ctx, alertProperties, metricMoreThan)
	} else if metricLessThan := alertDefinitionModel.MetricLessThan; !(metricLessThan.IsNull() || metricLessThan.IsUnknown()) {
		alertProperties, diags = expandMetricLessThanAlertTypeDefinition(ctx, alertProperties, metricLessThan)
	} else if metricMoreThanUsual := alertDefinitionModel.MetricMoreThanUsual; !(metricMoreThanUsual.IsNull() || metricMoreThanUsual.IsUnknown()) {
		alertProperties, diags = expandMetricMoreThanUsualAlertTypeDefinition(ctx, alertProperties, metricMoreThanUsual)
	} else if metricLessThanUsual := alertDefinitionModel.MetricLessThanUsual; !(metricLessThanUsual.IsNull() || metricLessThanUsual.IsUnknown()) {
		alertProperties, diags = expandMetricLessThanUsualAlertTypeDefinition(ctx, alertProperties, metricLessThanUsual)
	} else if metricMoreThanOrEquals := alertDefinitionModel.MetricMoreThanOrEquals; !(metricMoreThanOrEquals.IsNull() || metricMoreThanOrEquals.IsUnknown()) {
		alertProperties, diags = expandMetricMoreThanOrEqualsAlertTypeDefinition(ctx, alertProperties, metricMoreThanOrEquals)
	} else if metricLessThanOrEquals := alertDefinitionModel.MetricLessThanOrEquals; !(metricLessThanOrEquals.IsNull() || metricLessThanOrEquals.IsUnknown()) {
		alertProperties, diags = expandMetricLessThanOrEqualsAlertTypeDefinition(ctx, alertProperties, metricLessThanOrEquals)
	} else if tracingImmediate := alertDefinitionModel.TracingImmediate; !(tracingImmediate.IsNull() || tracingImmediate.IsUnknown()) {
		alertProperties, diags = expandTracingImmediateAlertTypeDefinition(ctx, alertProperties, tracingImmediate)
	} else if tracingMoreThan := alertDefinitionModel.TracingMoreThan; !(tracingMoreThan.IsNull() || tracingMoreThan.IsUnknown()) {
		alertProperties, diags = expandTracingMoreThanAlertTypeDefinition(ctx, alertProperties, tracingMoreThan)
	} else if flow := alertDefinitionModel.Flow; !(flow.IsNull() || flow.IsUnknown()) {
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
	if logsImmediateObject.IsNull() || logsImmediateObject.IsUnknown() {
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
	if !(filterModel.LuceneFilter.IsNull() || filterModel.LuceneFilter.IsUnknown()) {
		logsFilter.FilterType, diags = extractLuceneFilter(ctx, filterModel.LuceneFilter)
	}

	if diags.HasError() {
		return nil, diags
	}

	return logsFilter, nil
}

func extractLuceneFilter(ctx context.Context, luceneFilter types.Object) (*cxsdk.LogsFilterLuceneFilter, diag.Diagnostics) {
	if luceneFilter.IsNull() || luceneFilter.IsUnknown() {
		return nil, nil
	}

	var luceneFilterModel LuceneFilterModel
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

func expandLogsMoreThanAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, moreThanObject types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if moreThanObject.IsNull() || moreThanObject.IsUnknown() {
		return properties, nil
	}

	var moreThanModel LogsMoreThanModel
	if diags := moreThanObject.As(ctx, &moreThanModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	logsFilter, diags := extractLogsFilter(ctx, moreThanModel.LogsFilter)
	if diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := typeStringSliceToWrappedStringSlice(ctx, moreThanModel.NotificationPayloadFilter.Elements())
	if diags.HasError() {
		return nil, diags
	}

	timeWindow, diags := extractLogsTimeWindow(ctx, moreThanModel.TimeWindow)
	if diags.HasError() {
		return nil, diags
	}

	properties.TypeDefinition = &cxsdk.AlertDefPropertiesLogsThreshold{
		LogsThreshold: &cxsdk.LogsThresholdType{
			LogsFilter: logsFilter,
			Rules: []*cxsdk.LogsThresholdRule{
				{Condition: &cxsdk.LogsThresholdCondition{
					Threshold:     typeFloat64ToWrapperspbDouble(moreThanModel.Threshold),
					TimeWindow:    timeWindow,
					ConditionType: cxsdk.LogsThresholdConditionTypeMoreThanOrUnspecified,
					// EvaluationWindow: evaluationWindowTypeSchemaToProtoMap[moreThanModel.EvaluationWindow.ValueString()],
				}},
			},
			NotificationPayloadFilter: notificationPayloadFilter,
		},
	}
	properties.Type = cxsdk.AlertDefTypeLogsThreshold
	return properties, nil
}

func extractLogsTimeWindow(ctx context.Context, timeWindow types.Object) (*cxsdk.LogsTimeWindow, diag.Diagnostics) {
	if timeWindow.IsNull() || timeWindow.IsUnknown() {
		return nil, nil
	}

	var timeWindowModel LogsTimeWindowModel
	if diags := timeWindow.As(ctx, &timeWindowModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if specificValue := timeWindowModel.SpecificValue; !(specificValue.IsNull() || specificValue.IsUnknown()) {
		return &cxsdk.LogsTimeWindow{
			Type: &cxsdk.LogsTimeWindowSpecificValue{
				LogsTimeWindowSpecificValue: logsTimeWindowValueSchemaToProtoMap[specificValue.ValueString()],
			},
		}, nil
	}

	return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Time Window", "Time Window is not valid")}
}

func expandLogsLessThanAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, lessThan types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if lessThan.IsNull() || lessThan.IsUnknown() {
		return properties, nil
	}

	var lessThanModel LogsLessThanModel
	if diags := lessThan.As(ctx, &lessThanModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	logsFilter, diags := extractLogsFilter(ctx, lessThanModel.LogsFilter)
	if diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := typeStringSliceToWrappedStringSlice(ctx, lessThanModel.NotificationPayloadFilter.Elements())
	if diags.HasError() {
		return nil, diags
	}

	timeWindow, diags := extractLogsTimeWindow(ctx, lessThanModel.TimeWindow)
	if diags.HasError() {
		return nil, diags
	}

	undetectedValuesManagement, diags := extractUndetectedValuesManagement(ctx, lessThanModel.UndetectedValuesManagement)
	if diags.HasError() {
		return nil, diags
	}

	properties.TypeDefinition = &cxsdk.AlertDefProperties_LogsLessThan{
		LogsLessThan: &cxsdk.LogsLessThanTypeDefinition{
			LogsFilter:                 logsFilter,
			Threshold:                  typeInt64ToWrappedUint32(lessThanModel.Threshold),
			TimeWindow:                 timeWindow,
			UndetectedValuesManagement: undetectedValuesManagement,
			NotificationPayloadFilter:  notificationPayloadFilter,
		},
	}
	properties.AlertDefType = cxsdk.AlertDefType_ALERT_DEF_TYPE_LOGS_LESS_THAN
	return properties, nil
}

func extractUndetectedValuesManagement(ctx context.Context, management types.Object) (*cxsdk.UndetectedValuesManagement, diag.Diagnostics) {
	if management.IsNull() || management.IsUnknown() {
		return nil, nil
	}

	var managementModel UndetectedValuesManagementModel
	if diags := management.As(ctx, &managementModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
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

func expandLogsMoreThanUsualAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, moreThanUsual types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if moreThanUsual.IsNull() || moreThanUsual.IsUnknown() {
		return properties, nil
	}

	var moreThanUsualModel LogsMoreThanUsualModel
	if diags := moreThanUsual.As(ctx, &moreThanUsualModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	logsFilter, diags := extractLogsFilter(ctx, moreThanUsualModel.LogsFilter)
	if diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := typeStringSliceToWrappedStringSlice(ctx, moreThanUsualModel.NotificationPayloadFilter.Elements())
	if diags.HasError() {
		return nil, diags
	}

	timeWindow, diags := extractLogsTimeWindow(ctx, moreThanUsualModel.TimeWindow)
	if diags.HasError() {
		return nil, diags
	}

	properties.TypeDefinition = &cxsdk.AlertDefProperties_LogsMoreThanUsual{
		LogsMoreThanUsual: &cxsdk.LogsMoreThanUsualTypeDefinition{
			LogsFilter:                logsFilter,
			MinimumThreshold:          typeInt64ToWrappedUint32(moreThanUsualModel.MinimumThreshold),
			TimeWindow:                timeWindow,
			NotificationPayloadFilter: notificationPayloadFilter,
		},
	}
	properties.AlertDefType = cxsdk.AlertDefType_ALERT_DEF_TYPE_LOGS_MORE_THAN_USUAL
	return properties, nil
}

func expandLogsRatioMoreThanAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, moreThan types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if moreThan.IsNull() || moreThan.IsUnknown() {
		return properties, nil
	}

	var moreThanModel LogsRatioMoreThanModel
	if diags := moreThan.As(ctx, &moreThanModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	numeratorLogsFilter, diags := extractLogsFilter(ctx, moreThanModel.NumeratorLogsFilter)
	if diags.HasError() {
		return nil, diags
	}

	denominatorLogsFilter, diags := extractLogsFilter(ctx, moreThanModel.DenominatorLogsFilter)
	if diags.HasError() {
		return nil, diags
	}

	timeWindow, diags := extractLogsRatioTimeWindow(ctx, moreThanModel.TimeWindow)
	if diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := typeStringSliceToWrappedStringSlice(ctx, moreThanModel.NotificationPayloadFilter.Elements())
	if diags.HasError() {
		return nil, diags
	}

	properties.TypeDefinition = &cxsdk.AlertDefPropertiesLogsRatioThreshold{
		LogsRatioThreshold: &cxsdk.LogsRatioThresholdType{
			Numerator:        numeratorLogsFilter,
			NumeratorAlias:   typeStringToWrapperspbString(moreThanModel.NumeratorAlias),
			Denominator:      denominatorLogsFilter,
			DenominatorAlias: typeStringToWrapperspbString(moreThanModel.DenominatorAlias),
			Rules: []*cxsdk.LogsRatioRules{
				{
					Condition: &cxsdk.LogsRatioCondition{
						Threshold:      typeFloat64ToWrapperspbDouble(moreThanModel.Threshold),
						TimeWindow:     timeWindow,
						IgnoreInfinity: typeBoolToWrapperspbBool(moreThanModel.IgnoreInfinity),
						ConditionType:  cxsdk.LogsRatioConditionTypeMoreThanOrUnspecified,
					},
				},
			},
			NotificationPayloadFilter: notificationPayloadFilter,
			GroupByFor:                logsRatioGroupByForSchemaToProtoMap[moreThanModel.GroupByFor.ValueString()],
		},
	}
	properties.Type = cxsdk.AlertDefTypeLogsRatioThreshold
	return properties, nil
}

func extractLogsRatioTimeWindow(ctx context.Context, window types.Object) (*cxsdk.LogsRatioTimeWindow, diag.Diagnostics) {
	if window.IsNull() || window.IsUnknown() {
		return nil, nil
	}

	var windowModel LogsRatioTimeWindowModel
	if diags := window.As(ctx, &windowModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if specificValue := windowModel.SpecificValue; !(specificValue.IsNull() || specificValue.IsUnknown()) {
		return &cxsdk.LogsRatioTimeWindow{
			Type: &cxsdk.LogsRatioTimeWindowSpecificValue{
				LogsRatioTimeWindowSpecificValue: logsRatioTimeWindowValueSchemaToProtoMap[specificValue.ValueString()],
			},
		}, nil
	}

	return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Time Window", "Time Window is not valid")}
}

func expandLogsRatioLessThanAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, ratioLessThan types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if ratioLessThan.IsNull() || ratioLessThan.IsUnknown() {
		return properties, nil
	}

	var ratioLessThanModel LogsRatioLessThanModel
	if diags := ratioLessThan.As(ctx, &ratioLessThanModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	numeratorLogsFilter, diags := extractLogsFilter(ctx, ratioLessThanModel.NumeratorLogsFilter)
	if diags.HasError() {
		return nil, diags
	}

	denominatorLogsFilter, diags := extractLogsFilter(ctx, ratioLessThanModel.DenominatorLogsFilter)
	if diags.HasError() {
		return nil, diags
	}

	timeWindow, diags := extractLogsRatioTimeWindow(ctx, ratioLessThanModel.TimeWindow)
	if diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := typeStringSliceToWrappedStringSlice(ctx, ratioLessThanModel.NotificationPayloadFilter.Elements())
	if diags.HasError() {
		return nil, diags
	}

	undetectedValuesManagement, diags := extractUndetectedValuesManagement(ctx, ratioLessThanModel.UndetectedValuesManagement)
	if diags.HasError() {
		return nil, diags
	}

	properties.TypeDefinition = &cxsdk.AlertDefProperties_LogsRatioLessThan{
		LogsRatioLessThan: &cxsdk.LogsRatioLessThanTypeDefinition{
			NumeratorLogsFilter:        numeratorLogsFilter,
			NumeratorAlias:             typeStringToWrapperspbString(ratioLessThanModel.NumeratorAlias),
			DenominatorLogsFilter:      denominatorLogsFilter,
			DenominatorAlias:           typeStringToWrapperspbString(ratioLessThanModel.DenominatorAlias),
			Threshold:                  typeInt64ToWrappedUint32(ratioLessThanModel.Threshold),
			TimeWindow:                 timeWindow,
			IgnoreInfinity:             typeBoolToWrapperspbBool(ratioLessThanModel.IgnoreInfinity),
			NotificationPayloadFilter:  notificationPayloadFilter,
			GroupByFor:                 logsRatioGroupByForSchemaToProtoMap[ratioLessThanModel.GroupByFor.ValueString()],
			UndetectedValuesManagement: undetectedValuesManagement,
		},
	}
	properties.AlertDefType = cxsdk.AlertDefType_ALERT_DEF_TYPE_LOGS_RATIO_LESS_THAN
	return properties, nil
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

	timeWindow, diags := extractLogsNewValueTimeWindow(ctx, newValueModel.TimeWindow)
	if diags.HasError() {
		return nil, diags
	}

	properties.TypeDefinition = &cxsdk.AlertDefProperties_LogsNewValue{
		LogsNewValue: &cxsdk.LogsNewValueTypeDefinition{
			LogsFilter:                logsFilter,
			KeypathToTrack:            typeStringToWrapperspbString(newValueModel.KeypathToTrack),
			TimeWindow:                timeWindow,
			NotificationPayloadFilter: notificationPayloadFilter,
		},
	}
	properties.AlertDefType = cxsdk.AlertDefType_ALERT_DEF_TYPE_LOGS_NEW_VALUE
	return properties, nil
}

func extractLogsNewValueTimeWindow(ctx context.Context, window types.Object) (*cxsdk.LogsNewValueTimeWindow, diag.Diagnostics) {
	if window.IsNull() || window.IsUnknown() {
		return nil, nil
	}

	var windowModel LogsNewValueTimeWindowModel
	if diags := window.As(ctx, &windowModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if specificValue := windowModel.SpecificValue; !(specificValue.IsNull() || specificValue.IsUnknown()) {
		return &cxsdk.LogsNewValueTimeWindow{
			Type: &cxsdk.LogsNewValueTimeWindow_LogsNewValueTimeWindowSpecificValue{
				LogsNewValueTimeWindowSpecificValue: logsNewValueTimeWindowValueSchemaToProtoMap[specificValue.ValueString()],
			},
		}, nil
	}

	return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Time Window", "Time Window is not valid")}

}

func expandLogsUniqueCountAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, uniqueCount types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if uniqueCount.IsNull() || uniqueCount.IsUnknown() {
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

	timeWindow, diags := extractLogsUniqueCountTimeWindow(ctx, uniqueCountModel.TimeWindow)
	if diags.HasError() {
		return nil, diags
	}

	properties.TypeDefinition = &cxsdk.AlertDefProperties_LogsUniqueCount{
		LogsUniqueCount: &cxsdk.LogsUniqueCountTypeDefinition{
			LogsFilter:                  logsFilter,
			UniqueCountKeypath:          typeStringToWrapperspbString(uniqueCountModel.UniqueCountKeypath),
			MaxUniqueCount:              typeInt64ToWrappedInt64(uniqueCountModel.MaxUniqueCount),
			TimeWindow:                  timeWindow,
			NotificationPayloadFilter:   notificationPayloadFilter,
			MaxUniqueCountPerGroupByKey: typeInt64ToWrappedInt64(uniqueCountModel.MaxUniqueCountPerGroupByKey),
		},
	}
	properties.AlertDefType = cxsdk.AlertDefType_ALERT_DEF_TYPE_LOGS_UNIQUE_COUNT
	return properties, nil
}

func extractLogsUniqueCountTimeWindow(ctx context.Context, window types.Object) (*cxsdk.LogsUniqueValueTimeWindow, diag.Diagnostics) {
	if window.IsNull() || window.IsUnknown() {
		return nil, nil
	}

	var windowModel LogsUniqueCountTimeWindowModel
	if diags := window.As(ctx, &windowModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if specificValue := windowModel.SpecificValue; !(specificValue.IsNull() || specificValue.IsUnknown()) {
		return &cxsdk.LogsUniqueValueTimeWindow{
			Type: &cxsdk.LogsUniqueValueTimeWindow_LogsUniqueValueTimeWindowSpecificValue{
				LogsUniqueValueTimeWindowSpecificValue: logsUniqueCountTimeWindowValueSchemaToProtoMap[specificValue.ValueString()],
			},
		}, nil
	}

	return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Time Window", "Time Window is not valid")}

}

func expandLogsTimeRelativeMoreThanAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, relativeMoreThan types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if relativeMoreThan.IsNull() || relativeMoreThan.IsUnknown() {
		return properties, nil
	}

	var relativeMoreThanModel LogsTimeRelativeMoreThanModel
	if diags := relativeMoreThan.As(ctx, &relativeMoreThanModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	logsFilter, diags := extractLogsFilter(ctx, relativeMoreThanModel.LogsFilter)
	if diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := typeStringSliceToWrappedStringSlice(ctx, relativeMoreThanModel.NotificationPayloadFilter.Elements())
	if diags.HasError() {
		return nil, diags
	}

	properties.TypeDefinition = &cxsdk.AlertDefProperties_LogsTimeRelativeMoreThan{
		LogsTimeRelativeMoreThan: &cxsdk.LogsTimeRelativeMoreThanTypeDefinition{
			LogsFilter:                logsFilter,
			Threshold:                 typeInt64ToWrappedUint32(relativeMoreThanModel.Threshold),
			ComparedTo:                logsTimeRelativeComparedToSchemaToProtoMap[relativeMoreThanModel.ComparedTo.ValueString()],
			IgnoreInfinity:            typeBoolToWrapperspbBool(relativeMoreThanModel.IgnoreInfinity),
			NotificationPayloadFilter: notificationPayloadFilter,
		},
	}
	properties.AlertDefType = cxsdk.AlertDefType_ALERT_DEF_TYPE_LOGS_TIME_RELATIVE_MORE_THAN
	return properties, nil
}

func expandLogsTimeRelativeLessThanAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, timeRelativeLessThan types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if timeRelativeLessThan.IsNull() || timeRelativeLessThan.IsUnknown() {
		return properties, nil
	}

	var timeRelativeLessThanModel LogsTimeRelativeLessThanModel
	if diags := timeRelativeLessThan.As(ctx, &timeRelativeLessThanModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	logsFilter, diags := extractLogsFilter(ctx, timeRelativeLessThanModel.LogsFilter)
	if diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := typeStringSliceToWrappedStringSlice(ctx, timeRelativeLessThanModel.NotificationPayloadFilter.Elements())
	if diags.HasError() {
		return nil, diags
	}

	undetectedValuesManagement, diags := extractUndetectedValuesManagement(ctx, timeRelativeLessThanModel.UndetectedValuesManagement)
	if diags.HasError() {
		return nil, diags
	}

	properties.TypeDefinition = &cxsdk.AlertDefProperties_LogsTimeRelativeLessThan{
		LogsTimeRelativeLessThan: &cxsdk.LogsTimeRelativeLessThanTypeDefinition{
			LogsFilter:                 logsFilter,
			Threshold:                  typeInt64ToWrappedUint32(timeRelativeLessThanModel.Threshold),
			ComparedTo:                 logsTimeRelativeComparedToSchemaToProtoMap[timeRelativeLessThanModel.ComparedTo.ValueString()],
			IgnoreInfinity:             typeBoolToWrapperspbBool(timeRelativeLessThanModel.IgnoreInfinity),
			UndetectedValuesManagement: undetectedValuesManagement,
			NotificationPayloadFilter:  notificationPayloadFilter,
		},
	}
	properties.AlertDefType = cxsdk.AlertDefType_ALERT_DEF_TYPE_LOGS_TIME_RELATIVE_LESS_THAN
	return properties, nil
}

func expandMetricMoreThanAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, metricMoreThan types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if metricMoreThan.IsNull() || metricMoreThan.IsUnknown() {
		return properties, nil
	}

	var metricMoreThanModel MetricMoreThanModel
	if diags := metricMoreThan.As(ctx, &metricMoreThanModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	metricFilter, diags := extractMetricFilter(ctx, metricMoreThanModel.MetricFilter)
	if diags.HasError() {
		return nil, diags
	}

	ofTheLast, diags := extractMetricTimeWindow(ctx, metricMoreThanModel.OfTheLast)
	if diags.HasError() {
		return nil, diags
	}

	missingValues, diags := extractMissingValues(ctx, metricMoreThanModel.MissingValues)
	if diags.HasError() {
		return nil, diags
	}

	properties.TypeDefinition = &cxsdk.AlertDefProperties_MetricMoreThan{
		MetricMoreThan: &cxsdk.MetricMoreThanTypeDefinition{
			MetricFilter:  metricFilter,
			Threshold:     typeFloat64ToWrapperspbFloat(metricMoreThanModel.Threshold),
			ForOverPct:    typeInt64ToWrappedUint32(metricMoreThanModel.ForOverPct),
			OfTheLast:     ofTheLast,
			MissingValues: missingValues,
		},
	}
	properties.AlertDefType = cxsdk.AlertDefType_ALERT_DEF_TYPE_METRIC_MORE_THAN

	return properties, nil
}

func extractMetricFilter(ctx context.Context, filter types.Object) (*cxsdk.MetricFilter, diag.Diagnostics) {
	if filter.IsNull() || filter.IsUnknown() {
		return nil, nil
	}

	var filterModel MetricFilterModel
	if diags := filter.As(ctx, &filterModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if promql := filterModel.Promql; !(promql.IsNull() || promql.IsUnknown()) {
		return &cxsdk.MetricFilter{
			Type: &cxsdk.MetricFilter_Promql{
				Promql: typeStringToWrapperspbString(promql),
			},
		}, nil
	}

	return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Metric Filter", "Metric Filter is not valid")}
}

func extractMetricTimeWindow(ctx context.Context, timeWindow types.Object) (*cxsdk.MetricTimeWindow, diag.Diagnostics) {
	if timeWindow.IsNull() || timeWindow.IsUnknown() {
		return nil, nil
	}

	var timeWindowModel MetricTimeWindowModel
	if diags := timeWindow.As(ctx, &timeWindowModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if specificValue := timeWindowModel.SpecificValue; !(specificValue.IsNull() || specificValue.IsUnknown()) {
		return &cxsdk.MetricTimeWindow{
			Type: &cxsdk.MetricTimeWindow_MetricTimeWindowSpecificValue{
				MetricTimeWindowSpecificValue: metricTimeWindowValueSchemaToProtoMap[specificValue.ValueString()],
			},
		}, nil
	}

	return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Time Window", "Time Window is not valid")}
}

func extractMissingValues(ctx context.Context, missingValues types.Object) (*cxsdk.MetricMissingValues, diag.Diagnostics) {
	if missingValues.IsNull() || missingValues.IsUnknown() {
		return nil, nil
	}

	var missingValuesModel MetricMissingValuesModel
	if diags := missingValues.As(ctx, &missingValuesModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	metricMissingValues := &cxsdk.MetricMissingValues{}
	if replaceWithZero := missingValuesModel.ReplaceWithZero; !(replaceWithZero.IsNull() || replaceWithZero.IsUnknown()) {
		metricMissingValues.MissingValues = &cxsdk.MetricMissingValues_ReplaceWithZero{
			ReplaceWithZero: typeBoolToWrapperspbBool(replaceWithZero),
		}
	} else if minNonNullValuesPct := missingValuesModel.MinNonNullValuesPct; !(minNonNullValuesPct.IsNull() || minNonNullValuesPct.IsUnknown()) {
		metricMissingValues.MissingValues = &cxsdk.MetricMissingValues_MinNonNullValuesPct{
			MinNonNullValuesPct: typeInt64ToWrappedUint32(minNonNullValuesPct),
		}
	} else {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Missing Values", "Missing Values is not valid")}
	}

	return metricMissingValues, nil
}

func expandMetricLessThanAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, metricLessThan types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if metricLessThan.IsNull() || metricLessThan.IsUnknown() {
		return properties, nil
	}

	var metricLessThanModel MetricLessThanModel
	if diags := metricLessThan.As(ctx, &metricLessThanModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	metricFilter, diags := extractMetricFilter(ctx, metricLessThanModel.MetricFilter)
	if diags.HasError() {
		return nil, diags
	}

	ofTheLast, diags := extractMetricTimeWindow(ctx, metricLessThanModel.OfTheLast)
	if diags.HasError() {
		return nil, diags
	}

	missingValues, diags := extractMissingValues(ctx, metricLessThanModel.MissingValues)
	if diags.HasError() {
		return nil, diags
	}

	undetectedValuesManagement, diags := extractUndetectedValuesManagement(ctx, metricLessThanModel.UndetectedValuesManagement)
	if diags.HasError() {
		return nil, diags
	}

	properties.TypeDefinition = &cxsdk.AlertDefProperties_MetricLessThan{
		MetricLessThan: &cxsdk.MetricLessThanTypeDefinition{
			MetricFilter:               metricFilter,
			Threshold:                  typeFloat64ToWrapperspbFloat(metricLessThanModel.Threshold),
			ForOverPct:                 typeInt64ToWrappedUint32(metricLessThanModel.ForOverPct),
			OfTheLast:                  ofTheLast,
			MissingValues:              missingValues,
			UndetectedValuesManagement: undetectedValuesManagement,
		},
	}
	properties.AlertDefType = cxsdk.AlertDefType_ALERT_DEF_TYPE_METRIC_LESS_THAN

	return properties, nil
}

func expandTracingMoreThanAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, tracingMoreThan types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if tracingMoreThan.IsNull() || tracingMoreThan.IsUnknown() {
		return properties, nil
	}

	var tracingMoreThanModel TracingMoreThanModel
	if diags := tracingMoreThan.As(ctx, &tracingMoreThanModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	tracingQuery, diags := extractTracingFilter(ctx, tracingMoreThanModel.TracingFilter)
	if diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := typeStringSliceToWrappedStringSlice(ctx, tracingMoreThanModel.NotificationPayloadFilter.Elements())
	if diags.HasError() {
		return nil, diags
	}

	timeWindow, diags := extractTracingTimeWindow(ctx, tracingMoreThanModel.TimeWindow)
	if diags.HasError() {
		return nil, diags
	}

	properties.TypeDefinition = &cxsdk.AlertDefProperties_TracingMoreThan{
		TracingMoreThan: &cxsdk.TracingMoreThanTypeDefinition{
			TracingFilter:             tracingQuery,
			SpanAmount:                typeInt64ToWrappedUint32(tracingMoreThanModel.SpanAmount),
			TimeWindow:                timeWindow,
			NotificationPayloadFilter: notificationPayloadFilter,
		},
	}
	properties.AlertDefType = cxsdk.AlertDefType_ALERT_DEF_TYPE_TRACING_MORE_THAN

	return properties, nil
}

func extractTracingFilter(ctx context.Context, query types.Object) (*cxsdk.TracingFilter, diag.Diagnostics) {
	if query.IsNull() || query.IsUnknown() {
		return nil, nil
	}

	var queryModel TracingFilterModel
	if diags := query.As(ctx, &queryModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	tracingQuery := &cxsdk.TracingFilter{
		LatencyThresholdMs: typeInt64ToWrappedUint32(queryModel.LatencyThresholdMs),
	}

	tracingQuery, diags := expandTracingFilters(ctx, tracingQuery, &queryModel)
	if diags.HasError() {
		return nil, diags
	}

	return tracingQuery, nil
}

func expandTracingFilters(ctx context.Context, query *cxsdk.TracingFilter, tracingQueryModel *TracingFilterModel) (*cxsdk.TracingFilter, diag.Diagnostics) {
	if tracingQueryModel == nil {
		return query, nil
	}

	var diags diag.Diagnostics
	if tracingLabelFilters := tracingQueryModel.TracingLabelFilters; !(tracingLabelFilters.IsNull() || tracingLabelFilters.IsUnknown()) {
		query, diags = expandTracingLabelFilters(ctx, query, tracingLabelFilters)
	} else {
		diags = diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Tracing Label Filters", "Tracing Label Filters is not valid")}
	}

	return query, diags
}

func expandTracingLabelFilters(ctx context.Context, query *cxsdk.TracingFilter, tracingLabelFilters types.Object) (*cxsdk.TracingFilter, diag.Diagnostics) {
	var filtersModel TracingLabelFiltersModel
	if diags := tracingLabelFilters.As(ctx, &filtersModel, basetypes.ObjectAsOptions{}); diags.HasError() {
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

	spanFields, diags := extractTracingSpanFieldsFilterType(ctx, filtersModel.SpanFields)
	if diags.HasError() {
		return nil, diags
	}

	query.Filters = &cxsdk.TracingFilter_TracingLabelFilters{
		TracingLabelFilters: &cxsdk.TracingLabelFilters{
			ApplicationName: applicationName,
			SubsystemName:   subsystemName,
			OperationName:   operationName,
			SpanFields:      spanFields,
		},
	}

	return query, nil
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
		filter, diags := extractTracingLabelFilter(ctx, filtersObject)
		if diags.HasError() {
			return nil, diags
		}
		filters = append(filters, filter)
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
	diags := spanFields.ElementsAs(ctx, &spanFieldsObjects, true)
	var filters []*cxsdk.TracingSpanFieldsFilterType
	for _, element := range spanFieldsObjects {
		var filterModel TracingSpanFieldsFilterModel
		if diags = element.As(ctx, &filterModel, basetypes.ObjectAsOptions{}); diags.HasError() {
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

func extractTracingTimeWindow(ctx context.Context, window types.Object) (*cxsdk.TracingTimeWindow, diag.Diagnostics) {
	if window.IsNull() || window.IsUnknown() {
		return nil, nil
	}

	var windowModel TracingTimeWindowModel
	if diags := window.As(ctx, &windowModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if specificValue := windowModel.SpecificValue; !(specificValue.IsNull() || specificValue.IsUnknown()) {
		return &cxsdk.TracingTimeWindow{
			Type: &cxsdk.TracingTimeWindow_TracingTimeWindowValue{
				TracingTimeWindowValue: tracingTimeWindowSchemaToProtoMap[specificValue.ValueString()],
			},
		}, nil
	}

	return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Time Window", "Time Window is not valid")}

}

func expandMetricMoreThanUsualAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, metricMoreThanUsual types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if metricMoreThanUsual.IsNull() || metricMoreThanUsual.IsUnknown() {
		return properties, nil
	}

	var metricMoreThanUsualModel MetricMoreThanUsualModel
	if diags := metricMoreThanUsual.As(ctx, &metricMoreThanUsualModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	metricFilter, diags := extractMetricFilter(ctx, metricMoreThanUsualModel.MetricFilter)
	if diags.HasError() {
		return nil, diags
	}

	ofTheLast, diags := extractMetricTimeWindow(ctx, metricMoreThanUsualModel.OfTheLast)
	if diags.HasError() {
		return nil, diags
	}

	properties.TypeDefinition = &cxsdk.AlertDefProperties_MetricMoreThanUsual{
		MetricMoreThanUsual: &cxsdk.MetricMoreThanUsualTypeDefinition{
			MetricFilter:        metricFilter,
			Threshold:           typeInt64ToWrappedUint32(metricMoreThanUsualModel.Threshold),
			ForOverPct:          typeInt64ToWrappedUint32(metricMoreThanUsualModel.ForOverPct),
			OfTheLast:           ofTheLast,
			MinNonNullValuesPct: typeInt64ToWrappedUint32(metricMoreThanUsualModel.MinNonNullValuesPct),
		},
	}
	properties.AlertDefType = cxsdk.AlertDefType_ALERT_DEF_TYPE_METRIC_MORE_THAN_USUAL

	return properties, nil
}

func expandMetricLessThanUsualAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, metricLessThanUsual types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if metricLessThanUsual.IsNull() || metricLessThanUsual.IsUnknown() {
		return properties, nil
	}

	var metricLessThanUsualModel MetricLessThanUsualModel
	if diags := metricLessThanUsual.As(ctx, &metricLessThanUsualModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	metricFilter, diags := extractMetricFilter(ctx, metricLessThanUsualModel.MetricFilter)
	if diags.HasError() {
		return nil, diags
	}

	ofTheLast, diags := extractMetricTimeWindow(ctx, metricLessThanUsualModel.OfTheLast)
	if diags.HasError() {
		return nil, diags
	}

	properties.TypeDefinition = &cxsdk.AlertDefProperties_MetricLessThanUsual{
		MetricLessThanUsual: &cxsdk.MetricLessThanUsualTypeDefinition{
			MetricFilter:        metricFilter,
			Threshold:           typeInt64ToWrappedUint32(metricLessThanUsualModel.Threshold),
			ForOverPct:          typeInt64ToWrappedUint32(metricLessThanUsualModel.ForOverPct),
			OfTheLast:           ofTheLast,
			MinNonNullValuesPct: typeInt64ToWrappedUint32(metricLessThanUsualModel.MinNonNullValuesPct),
		},
	}
	properties.AlertDefType = cxsdk.AlertDefType_ALERT_DEF_TYPE_METRIC_LESS_THAN_USUAL

	return properties, nil
}

func expandMetricMoreThanOrEqualsAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, metricMoreThanOrEquals types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if metricMoreThanOrEquals.IsNull() || metricMoreThanOrEquals.IsUnknown() {
		return properties, nil
	}

	var metricMoreThanOrEqualsModel MetricMoreThanOrEqualsModel
	if diags := metricMoreThanOrEquals.As(ctx, &metricMoreThanOrEqualsModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	metricFilter, diags := extractMetricFilter(ctx, metricMoreThanOrEqualsModel.MetricFilter)
	if diags.HasError() {
		return nil, diags
	}

	ofTheLast, diags := extractMetricTimeWindow(ctx, metricMoreThanOrEqualsModel.OfTheLast)
	if diags.HasError() {
		return nil, diags
	}

	missingValues, diags := extractMissingValues(ctx, metricMoreThanOrEqualsModel.MissingValues)
	if diags.HasError() {
		return nil, diags
	}

	properties.TypeDefinition = &cxsdk.AlertDefProperties_MetricMoreThanOrEquals{
		MetricMoreThanOrEquals: &cxsdk.MetricMoreThanOrEqualsTypeDefinition{
			MetricFilter:  metricFilter,
			Threshold:     typeFloat64ToWrapperspbFloat(metricMoreThanOrEqualsModel.Threshold),
			ForOverPct:    typeInt64ToWrappedUint32(metricMoreThanOrEqualsModel.ForOverPct),
			OfTheLast:     ofTheLast,
			MissingValues: missingValues,
		},
	}
	properties.AlertDefType = cxsdk.AlertDefType_ALERT_DEF_TYPE_METRIC_MORE_THAN_OR_EQUALS
	return properties, nil
}

func expandMetricLessThanOrEqualsAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, equals types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if equals.IsNull() || equals.IsUnknown() {
		return properties, nil
	}

	var equalsModel MetricLessThanOrEqualsModel
	if diags := equals.As(ctx, &equalsModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	metricFilter, diags := extractMetricFilter(ctx, equalsModel.MetricFilter)
	if diags.HasError() {
		return nil, diags
	}

	ofTheLast, diags := extractMetricTimeWindow(ctx, equalsModel.OfTheLast)
	if diags.HasError() {
		return nil, diags
	}

	missingValues, diags := extractMissingValues(ctx, equalsModel.MissingValues)
	if diags.HasError() {
		return nil, diags
	}

	undetectedValuesManagement, diags := extractUndetectedValuesManagement(ctx, equalsModel.UndetectedValuesManagement)
	if diags.HasError() {
		return nil, diags
	}

	properties.TypeDefinition = &cxsdk.AlertDefProperties_MetricLessThanOrEquals{
		MetricLessThanOrEquals: &cxsdk.MetricLessThanOrEqualsTypeDefinition{
			MetricFilter:               metricFilter,
			Threshold:                  typeFloat64ToWrapperspbFloat(equalsModel.Threshold),
			ForOverPct:                 typeInt64ToWrappedUint32(equalsModel.ForOverPct),
			OfTheLast:                  ofTheLast,
			MissingValues:              missingValues,
			UndetectedValuesManagement: undetectedValuesManagement,
		},
	}
	properties.AlertDefType = cxsdk.AlertDefType_ALERT_DEF_TYPE_METRIC_LESS_THAN_OR_EQUALS
	return properties, nil
}

func expandTracingImmediateAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, tracingImmediate types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if tracingImmediate.IsNull() || tracingImmediate.IsUnknown() {
		return properties, nil
	}

	var tracingImmediateModel TracingImmediateModel
	if diags := tracingImmediate.As(ctx, &tracingImmediateModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	tracingQuery, diags := extractTracingFilter(ctx, tracingImmediateModel.TracingFilter)
	if diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := typeStringSliceToWrappedStringSlice(ctx, tracingImmediateModel.NotificationPayloadFilter.Elements())
	if diags.HasError() {
		return nil, diags
	}

	properties.TypeDefinition = &cxsdk.AlertDefProperties_TracingImmediate{
		TracingImmediate: &cxsdk.TracingImmediateTypeDefinition{
			TracingFilter:             tracingQuery,
			NotificationPayloadFilter: notificationPayloadFilter,
		},
	}
	properties.AlertDefType = cxsdk.AlertDefType_ALERT_DEF_TYPE_TRACING_IMMEDIATE

	return properties, nil
}

func expandFlowAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, flow types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if flow.IsNull() || flow.IsUnknown() {
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

	properties.TypeDefinition = &cxsdk.AlertDefProperties_Flow{
		Flow: &cxsdk.FlowTypeDefinition{
			Stages:             stages,
			EnforceSuppression: typeBoolToWrapperspbBool(flowModel.EnforceSuppression),
		},
	}
	properties.AlertDefType = cxsdk.AlertDefType_ALERT_DEF_TYPE_FLOW
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

func extractFlowStagesGroups(ctx context.Context, groups types.List) (*cxsdk.FlowStages_FlowStagesGroups, diag.Diagnostics) {
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

	return &cxsdk.FlowStages_FlowStagesGroups{FlowStagesGroups: &cxsdk.FlowStagesGroups{
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

func extractAlertDefs(ctx context.Context, defs types.List) ([]*cxsdk.FlowStagesGroupsAlertDefs, diag.Diagnostics) {
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

	state, diags = flattenAlert(ctx, alert)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func flattenAlert(ctx context.Context, alert *cxsdk.AlertDef) (*AlertResourceModel, diag.Diagnostics) {
	alertProperties := alert.GetAlertDefProperties()
	alertSchedule, diags := flattenAlertSchedule(ctx, alertProperties)
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

	labels, diags := types.MapValueFrom(ctx, types.StringType, alertProperties.GetLabels())

	return &AlertResourceModel{
		ID:                wrapperspbStringToTypeString(alert.GetId()),
		Name:              wrapperspbStringToTypeString(alertProperties.GetName()),
		Description:       wrapperspbStringToTypeString(alertProperties.GetDescription()),
		Enabled:           wrapperspbBoolToTypeBool(alertProperties.GetEnabled()),
		Priority:          types.StringValue(alertPriorityProtoToSchemaMap[alertProperties.GetPriority()]),
		Schedule:          alertSchedule,
		TypeDefinition:    alertTypeDefinition,
		GroupBy:           wrappedStringSliceToTypeStringSet(alertProperties.GetGroupBy()),
		IncidentsSettings: incidentsSettings,
		NotificationGroup: notificationGroup,
		Labels:            labels,
	}, nil
}

func flattenNotificationGroup(ctx context.Context, notificationGroup *cxsdk.AlertDefNotificationGroup) (types.Object, diag.Diagnostics) {
	if notificationGroup == nil {
		return types.ObjectNull(notificationGroupAttr()), nil
	}

	advancedTargetSettings, diags := flattenAdvancedTargetSettings(ctx, notificationGroup.GetAdvanced())
	if diags.HasError() {
		return types.ObjectNull(notificationGroupAttr()), diags
	}

	simpleTargetSettings, diags := flattenSimpleTargetSettings(ctx, notificationGroup.GetSimple())
	if diags.HasError() {
		return types.ObjectNull(notificationGroupAttr()), diags
	}

	notificationGroupModel := NotificationGroupModel{
		GroupByFields:          wrappedStringSliceToTypeStringList(notificationGroup.GetGroupByFields()),
		AdvancedTargetSettings: advancedTargetSettings,
		SimpleTargetSettings:   simpleTargetSettings,
	}

	return types.ObjectValueFrom(ctx, notificationGroupAttr(), notificationGroupModel)
}

func flattenAdvancedTargetSettings(ctx context.Context, advancedTargetSettings *cxsdk.AlertDefAdvancedTargets) (types.Set, diag.Diagnostics) {
	if advancedTargetSettings == nil {
		return types.SetNull(types.ObjectType{AttrTypes: advancedTargetSettingsAttr()}), nil
	}

	var notificationsModel []*AdvancedTargetSettingsModel
	var diags diag.Diagnostics
	for _, notification := range advancedTargetSettings.GetAdvancedTargetsSettings() {
		retriggeringPeriod, dgs := flattenRetriggeringPeriod(ctx, notification)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		notificationModel := AdvancedTargetSettingsModel{
			NotifyOn:           types.StringValue(notifyOnProtoToSchemaMap[notification.GetNotifyOn()]),
			RetriggeringPeriod: retriggeringPeriod,
			IntegrationID:      types.StringNull(),
			Recipients:         types.SetNull(types.StringType),
		}
		switch integrationType := notification.GetIntegration(); integrationType.GetIntegrationType().(type) {
		case *cxsdk.IntegrationType_IntegrationId:
			notificationModel.IntegrationID = types.StringValue(strconv.Itoa(int(integrationType.GetIntegrationId().GetValue())))
		case *cxsdk.IntegrationType_Recipients:
			notificationModel.Recipients = wrappedStringSliceToTypeStringSet(integrationType.GetRecipients().GetEmails())
		}
		notificationsModel = append(notificationsModel, &notificationModel)
	}

	if diags.HasError() {
		return types.SetNull(types.ObjectType{AttrTypes: advancedTargetSettingsAttr()}), diags
	}

	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: advancedTargetSettingsAttr()}, notificationsModel)
}

func flattenRetriggeringPeriod(ctx context.Context, notifications *cxsdk.AlertDefAdvancedTargetSettings) (types.Object, diag.Diagnostics) {
	switch notificationPeriodType := notifications.RetriggeringPeriod.(type) {
	case *cxsdk.AlertDefAdvancedTargetSettings_Minutes:
		return types.ObjectValueFrom(ctx, retriggeringPeriodAttr(), RetriggeringPeriodModel{
			Minutes: wrapperspbUint32ToTypeInt64(notificationPeriodType.Minutes),
		})
	case nil:
		return types.ObjectNull(retriggeringPeriodAttr()), nil
	default:
		return types.ObjectNull(retriggeringPeriodAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Retriggering Period", fmt.Sprintf("Retriggering Period %v is not supported", notificationPeriodType))}
	}
}

func flattenSimpleTargetSettings(ctx context.Context, simpleTargetSettings *cxsdk.AlertDefTargetSimple) (types.Set, diag.Diagnostics) {
	if simpleTargetSettings == nil {
		return types.SetNull(types.ObjectType{AttrTypes: simpleTargetSettingsAttr()}), nil
	}

	var notificationsModel []SimpleTargetSettingsModel
	for _, notification := range simpleTargetSettings.GetIntegrations() {
		notificationModel := SimpleTargetSettingsModel{
			IntegrationID: types.StringNull(),
			Recipients:    types.SetNull(types.StringType),
		}
		switch notification.GetIntegrationType().(type) {
		case *cxsdk.IntegrationType_IntegrationId:
			notificationModel.IntegrationID = types.StringValue(strconv.Itoa(int(notification.GetIntegrationId().GetValue())))
		case *cxsdk.IntegrationType_Recipients:
			notificationModel.Recipients = wrappedStringSliceToTypeStringSet(notification.GetRecipients().GetEmails())
		}
		notificationsModel = append(notificationsModel, notificationModel)
	}
	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: simpleTargetSettingsAttr()}, notificationsModel)
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
	case *cxsdk.AlertDefIncidentSettings_Minutes:
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
		LogsImmediate:            types.ObjectNull(logsImmediateAttr()),
		LogsMoreThan:             types.ObjectNull(logsMoreThanAttr()),
		LogsLessThan:             types.ObjectNull(logsLessThanAttr()),
		LogsMoreThanUsual:        types.ObjectNull(logsMoreThanUsualAttr()),
		LogsRatioMoreThan:        types.ObjectNull(logsRatioMoreThanAttr()),
		LogsRatioLessThan:        types.ObjectNull(logsRatioLessThanAttr()),
		LogsNewValue:             types.ObjectNull(logsNewValueAttr()),
		LogsUniqueCount:          types.ObjectNull(logsUniqueCountAttr()),
		LogsTimeRelativeMoreThan: types.ObjectNull(logsTimeRelativeMoreThanAttr()),
		LogsTimeRelativeLessThan: types.ObjectNull(logsTimeRelativeLessThanAttr()),
		MetricMoreThan:           types.ObjectNull(metricMoreThanAttr()),
		MetricLessThan:           types.ObjectNull(metricLessThanAttr()),
		MetricMoreThanUsual:      types.ObjectNull(metricMoreThanUsualAttr()),
		MetricLessThanUsual:      types.ObjectNull(metricLessThanUsualAttr()),
		MetricLessThanOrEquals:   types.ObjectNull(metricLessThanOrEqualsAttr()),
		MetricMoreThanOrEquals:   types.ObjectNull(metricMoreThanOrEqualsAttr()),
		TracingImmediate:         types.ObjectNull(tracingImmediateAttr()),
		TracingMoreThan:          types.ObjectNull(tracingMoreThanAttr()),
		Flow:                     types.ObjectNull(flowAttr()),
	}
	var diags diag.Diagnostics
	switch alertTypeDefinition := properties.TypeDefinition.(type) {
	case *cxsdk.AlertDefProperties_LogsImmediate:
		alertTypeDefinitionModel.LogsImmediate, diags = flattenLogsImmediate(ctx, alertTypeDefinition.LogsImmediate)
	case *cxsdk.AlertDefProperties_LogsMoreThan:
		alertTypeDefinitionModel.LogsMoreThan, diags = flattenLogsMoreThan(ctx, alertTypeDefinition.LogsMoreThan)
	case *cxsdk.AlertDefProperties_LogsLessThan:
		alertTypeDefinitionModel.LogsLessThan, diags = flattenLogsLessThan(ctx, alertTypeDefinition.LogsLessThan)
	case *cxsdk.AlertDefProperties_LogsMoreThanUsual:
		alertTypeDefinitionModel.LogsMoreThanUsual, diags = flattenLogsMoreThanUsual(ctx, alertTypeDefinition.LogsMoreThanUsual)
	case *cxsdk.AlertDefProperties_LogsRatioMoreThan:
		alertTypeDefinitionModel.LogsRatioMoreThan, diags = flattenLogsRatioMoreThan(ctx, alertTypeDefinition.LogsRatioMoreThan)
	case *cxsdk.AlertDefProperties_LogsRatioLessThan:
		alertTypeDefinitionModel.LogsRatioLessThan, diags = flattenLogsRatioLessThan(ctx, alertTypeDefinition.LogsRatioLessThan)
	case *cxsdk.AlertDefProperties_LogsNewValue:
		alertTypeDefinitionModel.LogsNewValue, diags = flattenLogsNewValue(ctx, alertTypeDefinition.LogsNewValue)
	case *cxsdk.AlertDefProperties_LogsUniqueCount:
		alertTypeDefinitionModel.LogsUniqueCount, diags = flattenLogsUniqueCount(ctx, alertTypeDefinition.LogsUniqueCount)
	case *cxsdk.AlertDefProperties_LogsTimeRelativeMoreThan:
		alertTypeDefinitionModel.LogsTimeRelativeMoreThan, diags = flattenLogsTimeRelativeMoreThan(ctx, alertTypeDefinition.LogsTimeRelativeMoreThan)
	case *cxsdk.AlertDefProperties_LogsTimeRelativeLessThan:
		alertTypeDefinitionModel.LogsTimeRelativeLessThan, diags = flattenLogsTimeRelativeLessThan(ctx, alertTypeDefinition.LogsTimeRelativeLessThan)
	case *cxsdk.AlertDefProperties_MetricMoreThan:
		alertTypeDefinitionModel.MetricMoreThan, diags = flattenMetricMoreThan(ctx, alertTypeDefinition.MetricMoreThan)
	case *cxsdk.AlertDefProperties_MetricLessThan:
		alertTypeDefinitionModel.MetricLessThan, diags = flattenMetricLessThan(ctx, alertTypeDefinition.MetricLessThan)
	case *cxsdk.AlertDefProperties_MetricMoreThanUsual:
		alertTypeDefinitionModel.MetricMoreThanUsual, diags = flattenMetricMoreThanUsual(ctx, alertTypeDefinition.MetricMoreThanUsual)
	case *cxsdk.AlertDefProperties_MetricLessThanUsual:
		alertTypeDefinitionModel.MetricLessThanUsual, diags = flattenMetricLessThanUsual(ctx, alertTypeDefinition.MetricLessThanUsual)
	case *cxsdk.AlertDefProperties_MetricLessThanOrEquals:
		alertTypeDefinitionModel.MetricLessThanOrEquals, diags = flattenMetricLessThanOrEquals(ctx, alertTypeDefinition.MetricLessThanOrEquals)
	case *cxsdk.AlertDefProperties_MetricMoreThanOrEquals:
		alertTypeDefinitionModel.MetricMoreThanOrEquals, diags = flattenMetricMoreThanOrEquals(ctx, alertTypeDefinition.MetricMoreThanOrEquals)
	case *cxsdk.AlertDefProperties_TracingImmediate:
		alertTypeDefinitionModel.TracingImmediate, diags = flattenTracingImmediate(ctx, alertTypeDefinition.TracingImmediate)
	case *cxsdk.AlertDefProperties_TracingMoreThan:
		alertTypeDefinitionModel.TracingMoreThan, diags = flattenTracingMoreThan(ctx, alertTypeDefinition.TracingMoreThan)
	case *cxsdk.AlertDefProperties_Flow:
		alertTypeDefinitionModel.Flow, diags = flattenFlow(ctx, alertTypeDefinition.Flow)
	default:
		return types.ObjectNull(alertTypeDefinitionAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Alert Type Definition", fmt.Sprintf("Alert Type %v Definition is not valid", alertTypeDefinition))}
	}

	if diags.HasError() {
		return types.ObjectNull(alertTypeDefinitionAttr()), diags
	}

	return types.ObjectValueFrom(ctx, alertTypeDefinitionAttr(), alertTypeDefinitionModel)
}

func flattenLogsImmediate(ctx context.Context, immediate *cxsdk.LogsImmediateTypeDefinition) (types.Object, diag.Diagnostics) {
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
	case *cxsdk.LogsFilter_LuceneFilter:
		logsFilterModer.LuceneFilter, diags = flattenLuceneFilter(ctx, filterType.LuceneFilter)
	default:
		return types.ObjectNull(logsFilterAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Logs Filter", fmt.Sprintf("Logs Filter %v is not supported", filterType))}
	}

	if diags.HasError() {
		return types.ObjectNull(logsFilterAttr()), diags
	}

	return types.ObjectValueFrom(ctx, logsFilterAttr(), logsFilterModer)
}

func flattenLuceneFilter(ctx context.Context, filter *cxsdk.LuceneFilter) (types.Object, diag.Diagnostics) {
	if filter == nil {
		return types.ObjectNull(luceneFilterAttr()), nil
	}

	labelFilters, diags := flattenLabelFilters(ctx, filter.GetLabelFilters())
	if diags.HasError() {
		return types.ObjectNull(luceneFilterAttr()), diags
	}

	return types.ObjectValueFrom(ctx, luceneFilterAttr(), LuceneFilterModel{
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

func flattenLogsMoreThan(ctx context.Context, moreThan *cxsdk.LogsMoreThanTypeDefinition) (types.Object, diag.Diagnostics) {
	if moreThan == nil {
		return types.ObjectNull(logsMoreThanAttr()), nil
	}

	logsFilter, diags := flattenAlertsLogsFilter(ctx, moreThan.GetLogsFilter())
	if diags.HasError() {
		return types.ObjectNull(logsMoreThanAttr()), diags
	}

	timeWindow, diags := flattenLogsTimeWindow(ctx, moreThan.GetTimeWindow())
	if diags.HasError() {
		return types.ObjectNull(logsMoreThanAttr()), diags
	}

	logsMoreThanModel := LogsMoreThanModel{
		LogsFilter:                logsFilter,
		Threshold:                 wrapperspbUint32ToTypeInt64(moreThan.GetThreshold()),
		TimeWindow:                timeWindow,
		EvaluationWindow:          types.StringValue(evaluationWindowTypeProtoToSchemaMap[moreThan.GetEvaluationWindow()]),
		NotificationPayloadFilter: wrappedStringSliceToTypeStringSet(moreThan.GetNotificationPayloadFilter()),
	}
	return types.ObjectValueFrom(ctx, logsMoreThanAttr(), logsMoreThanModel)
}

func flattenLogsTimeWindow(ctx context.Context, timeWindow *cxsdk.LogsTimeWindow) (types.Object, diag.Diagnostics) {
	if timeWindow == nil {
		return types.ObjectNull(logsTimeWindowAttr()), nil
	}

	switch timeWindowType := timeWindow.Type.(type) {
	case *cxsdk.LogsTimeWindow_LogsTimeWindowSpecificValue:
		return types.ObjectValueFrom(ctx, logsTimeWindowAttr(), LogsTimeWindowModel{
			SpecificValue: types.StringValue(logsTimeWindowValueProtoToSchemaMap[timeWindowType.LogsTimeWindowSpecificValue]),
		})
	default:
		return types.ObjectNull(logsTimeWindowAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Time Window", fmt.Sprintf("Time Window %v is not supported", timeWindowType))}
	}

}

func flattenLogsLessThan(ctx context.Context, lessThan *cxsdk.LogsLessThanTypeDefinition) (types.Object, diag.Diagnostics) {
	if lessThan == nil {
		return types.ObjectNull(logsLessThanAttr()), nil
	}

	logsFilter, diags := flattenAlertsLogsFilter(ctx, lessThan.GetLogsFilter())
	if diags.HasError() {
		return types.ObjectNull(logsLessThanAttr()), diags
	}

	timeWindow, diags := flattenLogsTimeWindow(ctx, lessThan.GetTimeWindow())
	if diags.HasError() {
		return types.ObjectNull(logsLessThanAttr()), diags
	}

	undetectedValuesManagement, diags := flattenUndetectedValuesManagement(ctx, lessThan.GetUndetectedValuesManagement())
	if diags.HasError() {
		return types.ObjectNull(logsLessThanAttr()), diags
	}

	logsLessThanModel := LogsLessThanModel{
		LogsFilter:                 logsFilter,
		Threshold:                  wrapperspbUint32ToTypeInt64(lessThan.GetThreshold()),
		TimeWindow:                 timeWindow,
		UndetectedValuesManagement: undetectedValuesManagement,
		NotificationPayloadFilter:  wrappedStringSliceToTypeStringSet(lessThan.GetNotificationPayloadFilter()),
	}
	return types.ObjectValueFrom(ctx, logsLessThanAttr(), logsLessThanModel)
}

func flattenUndetectedValuesManagement(ctx context.Context, undetectedValuesManagement *cxsdk.UndetectedValuesManagement) (types.Object, diag.Diagnostics) {
	if undetectedValuesManagement == nil {
		return types.ObjectNull(undetectedValuesManagementAttr()), nil
	}

	undetectedValuesManagementModel := UndetectedValuesManagementModel{
		TriggerUndetectedValues: wrapperspbBoolToTypeBool(undetectedValuesManagement.GetTriggerUndetectedValues()),
		AutoRetireTimeframe:     types.StringValue(autoRetireTimeframeProtoToSchemaMap[undetectedValuesManagement.GetAutoRetireTimeframe()]),
	}

	return types.ObjectValueFrom(ctx, undetectedValuesManagementAttr(), undetectedValuesManagementModel)
}

func flattenLogsMoreThanUsual(ctx context.Context, moreThanUsual *cxsdk.LogsMoreThanUsualTypeDefinition) (types.Object, diag.Diagnostics) {
	if moreThanUsual == nil {
		return types.ObjectNull(logsMoreThanUsualAttr()), nil
	}

	logsFilter, diags := flattenAlertsLogsFilter(ctx, moreThanUsual.GetLogsFilter())
	if diags.HasError() {
		return types.ObjectNull(logsMoreThanUsualAttr()), diags
	}

	timeWindow, diags := flattenLogsTimeWindow(ctx, moreThanUsual.GetTimeWindow())
	if diags.HasError() {
		return types.ObjectNull(logsMoreThanUsualAttr()), diags
	}

	logsMoreThanUsualModel := LogsMoreThanUsualModel{
		LogsFilter:                logsFilter,
		MinimumThreshold:          wrapperspbUint32ToTypeInt64(moreThanUsual.GetMinimumThreshold()),
		TimeWindow:                timeWindow,
		NotificationPayloadFilter: wrappedStringSliceToTypeStringSet(moreThanUsual.GetNotificationPayloadFilter()),
	}
	return types.ObjectValueFrom(ctx, logsMoreThanUsualAttr(), logsMoreThanUsualModel)
}

func flattenLogsRatioMoreThan(ctx context.Context, ratioMoreThan *cxsdk.LogsRatioMoreThanTypeDefinition) (types.Object, diag.Diagnostics) {
	if ratioMoreThan == nil {
		return types.ObjectNull(logsRatioMoreThanAttr()), nil
	}

	numeratorLogsFilter, diags := flattenAlertsLogsFilter(ctx, ratioMoreThan.GetNumeratorLogsFilter())
	if diags.HasError() {
		return types.ObjectNull(logsRatioMoreThanAttr()), diags
	}

	denominatorLogsFilter, diags := flattenAlertsLogsFilter(ctx, ratioMoreThan.GetDenominatorLogsFilter())
	if diags.HasError() {
		return types.ObjectNull(logsRatioMoreThanAttr()), diags
	}

	timeWindow, diags := flattenLogsRatioTimeWindow(ctx, ratioMoreThan.GetTimeWindow())
	if diags.HasError() {
		return types.ObjectNull(logsRatioMoreThanAttr()), diags
	}

	logsRatioMoreThanModel := LogsRatioMoreThanModel{
		NumeratorLogsFilter:       numeratorLogsFilter,
		NumeratorAlias:            wrapperspbStringToTypeString(ratioMoreThan.GetNumeratorAlias()),
		DenominatorLogsFilter:     denominatorLogsFilter,
		DenominatorAlias:          wrapperspbStringToTypeString(ratioMoreThan.GetDenominatorAlias()),
		Threshold:                 typeFloat64ToWrapperspbDouble(ratioMoreThan.GetThreshold()),
		TimeWindow:                timeWindow,
		IgnoreInfinity:            wrapperspbBoolToTypeBool(ratioMoreThan.GetIgnoreInfinity()),
		NotificationPayloadFilter: wrappedStringSliceToTypeStringSet(ratioMoreThan.GetNotificationPayloadFilter()),
		GroupByFor:                types.StringValue(logsRatioGroupByForProtoToSchemaMap[ratioMoreThan.GetGroupByFor()]),
	}
	return types.ObjectValueFrom(ctx, logsRatioMoreThanAttr(), logsRatioMoreThanModel)
}

func flattenLogsRatioTimeWindow(ctx context.Context, window *cxsdk.LogsRatioTimeWindow) (types.Object, diag.Diagnostics) {
	if window == nil {
		return types.ObjectNull(logsTimeWindowAttr()), nil
	}

	switch timeWindowType := window.Type.(type) {
	case *cxsdk.LogsRatioTimeWindow_LogsRatioTimeWindowSpecificValue:
		return types.ObjectValueFrom(ctx, logsTimeWindowAttr(), LogsRatioTimeWindowModel{
			SpecificValue: types.StringValue(logsRatioTimeWindowValueProtoToSchemaMap[timeWindowType.LogsRatioTimeWindowSpecificValue]),
		})
	default:
		return types.ObjectNull(logsTimeWindowAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Time Window", fmt.Sprintf("Time Window %v is not supported", timeWindowType))}
	}
}

func flattenLogsRatioLessThan(ctx context.Context, ratioLessThan *cxsdk.LogsRatioLessThanTypeDefinition) (types.Object, diag.Diagnostics) {
	if ratioLessThan == nil {
		return types.ObjectNull(logsRatioLessThanAttr()), nil
	}

	numeratorLogsFilter, diags := flattenAlertsLogsFilter(ctx, ratioLessThan.GetNumeratorLogsFilter())
	if diags.HasError() {
		return types.ObjectNull(logsRatioLessThanAttr()), diags
	}

	denominatorLogsFilter, diags := flattenAlertsLogsFilter(ctx, ratioLessThan.GetDenominatorLogsFilter())
	if diags.HasError() {
		return types.ObjectNull(logsRatioLessThanAttr()), diags
	}

	timeWindow, diags := flattenLogsRatioTimeWindow(ctx, ratioLessThan.GetTimeWindow())
	if diags.HasError() {
		return types.ObjectNull(logsRatioLessThanAttr()), diags
	}

	undetectedValuesManagement, diags := flattenUndetectedValuesManagement(ctx, ratioLessThan.GetUndetectedValuesManagement())
	if diags.HasError() {
		return types.ObjectNull(logsRatioLessThanAttr()), diags
	}

	logsRatioLessThanModel := LogsRatioLessThanModel{
		NumeratorLogsFilter:        numeratorLogsFilter,
		NumeratorAlias:             wrapperspbStringToTypeString(ratioLessThan.GetNumeratorAlias()),
		DenominatorLogsFilter:      denominatorLogsFilter,
		DenominatorAlias:           wrapperspbStringToTypeString(ratioLessThan.GetDenominatorAlias()),
		Threshold:                  wrapperspbUint32ToTypeInt64(ratioLessThan.GetThreshold()),
		TimeWindow:                 timeWindow,
		IgnoreInfinity:             wrapperspbBoolToTypeBool(ratioLessThan.GetIgnoreInfinity()),
		NotificationPayloadFilter:  wrappedStringSliceToTypeStringSet(ratioLessThan.GetNotificationPayloadFilter()),
		GroupByFor:                 types.StringValue(logsRatioGroupByForProtoToSchemaMap[ratioLessThan.GetGroupByFor()]),
		UndetectedValuesManagement: undetectedValuesManagement,
	}
	return types.ObjectValueFrom(ctx, logsRatioLessThanAttr(), logsRatioLessThanModel)
}

func flattenLogsUniqueCount(ctx context.Context, uniqueCount *cxsdk.LogsUniqueCountTypeDefinition) (types.Object, diag.Diagnostics) {
	if uniqueCount == nil {
		return types.ObjectNull(logsUniqueCountAttr()), nil
	}

	logsFilter, diags := flattenAlertsLogsFilter(ctx, uniqueCount.GetLogsFilter())
	if diags.HasError() {
		return types.ObjectNull(logsUniqueCountAttr()), diags
	}

	timeWindow, diags := flattenLogsUniqueCountTimeWindow(ctx, uniqueCount.GetTimeWindow())
	if diags.HasError() {
		return types.ObjectNull(logsUniqueCountAttr()), diags
	}

	logsUniqueCountModel := LogsUniqueCountModel{
		LogsFilter:                  logsFilter,
		UniqueCountKeypath:          wrapperspbStringToTypeString(uniqueCount.GetUniqueCountKeypath()),
		MaxUniqueCount:              wrapperspbInt64ToTypeInt64(uniqueCount.GetMaxUniqueCount()),
		TimeWindow:                  timeWindow,
		NotificationPayloadFilter:   wrappedStringSliceToTypeStringSet(uniqueCount.GetNotificationPayloadFilter()),
		MaxUniqueCountPerGroupByKey: wrapperspbInt64ToTypeInt64(uniqueCount.GetMaxUniqueCountPerGroupByKey()),
	}
	return types.ObjectValueFrom(ctx, logsUniqueCountAttr(), logsUniqueCountModel)
}

func flattenLogsUniqueCountTimeWindow(ctx context.Context, timeWindow *cxsdk.LogsUniqueValueTimeWindow) (types.Object, diag.Diagnostics) {
	if timeWindow == nil {
		return types.ObjectNull(logsTimeWindowAttr()), nil
	}

	switch timeWindowType := timeWindow.Type.(type) {
	case *cxsdk.LogsUniqueValueTimeWindow_LogsUniqueValueTimeWindowSpecificValue:
		return types.ObjectValueFrom(ctx, logsTimeWindowAttr(), LogsUniqueCountTimeWindowModel{
			SpecificValue: types.StringValue(logsUniqueCountTimeWindowValueProtoToSchemaMap[timeWindowType.LogsUniqueValueTimeWindowSpecificValue]),
		})
	default:
		return types.ObjectNull(logsTimeWindowAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Time Window", fmt.Sprintf("Time Window %v is not supported", timeWindowType))}
	}

}

func flattenLogsNewValue(ctx context.Context, newValue *cxsdk.LogsNewValueTypeDefinition) (types.Object, diag.Diagnostics) {
	if newValue == nil {
		return types.ObjectNull(logsNewValueAttr()), nil
	}

	logsFilter, diags := flattenAlertsLogsFilter(ctx, newValue.GetLogsFilter())
	if diags.HasError() {
		return types.ObjectNull(logsNewValueAttr()), diags
	}

	timeWindow, diags := flattenLogsNewValueTimeWindow(ctx, newValue.GetTimeWindow())
	if diags.HasError() {
		return types.ObjectNull(logsNewValueAttr()), diags
	}

	logsNewValueModel := LogsNewValueModel{
		LogsFilter:                logsFilter,
		KeypathToTrack:            wrapperspbStringToTypeString(newValue.GetKeypathToTrack()),
		TimeWindow:                timeWindow,
		NotificationPayloadFilter: wrappedStringSliceToTypeStringSet(newValue.GetNotificationPayloadFilter()),
	}
	return types.ObjectValueFrom(ctx, logsNewValueAttr(), logsNewValueModel)
}

func flattenLogsNewValueTimeWindow(ctx context.Context, window *cxsdk.LogsNewValueTimeWindow) (types.Object, diag.Diagnostics) {
	if window == nil {
		return types.ObjectNull(logsTimeWindowAttr()), nil
	}

	switch timeWindowType := window.Type.(type) {
	case *cxsdk.LogsNewValueTimeWindow_LogsNewValueTimeWindowSpecificValue:
		return types.ObjectValueFrom(ctx, logsTimeWindowAttr(), LogsNewValueTimeWindowModel{
			SpecificValue: types.StringValue(logsNewValueTimeWindowValueProtoToSchemaMap[timeWindowType.LogsNewValueTimeWindowSpecificValue]),
		})
	default:
		return types.ObjectNull(logsTimeWindowAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Time Window", fmt.Sprintf("Time Window %v is not supported", timeWindowType))}
	}
}

func flattenAlertSchedule(ctx context.Context, alertProperties *cxsdk.AlertDefProperties) (types.Object, diag.Diagnostics) {
	if alertProperties.Schedule == nil {
		return types.ObjectNull(alertScheduleAttr()), nil
	}

	var alertScheduleModel AlertScheduleModel
	var diags diag.Diagnostics
	switch alertScheduleType := alertProperties.Schedule.(type) {
	case *cxsdk.AlertDefProperties_ActiveOn:
		alertScheduleModel.ActiveOn, diags = flattenActiveOn(ctx, alertScheduleType.ActiveOn)
	default:
		return types.ObjectNull(alertScheduleAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Alert Schedule", fmt.Sprintf("Alert Schedule %v is not supported", alertScheduleType))}
	}

	if diags.HasError() {
		return types.ObjectNull(alertScheduleAttr()), diags
	}

	return types.ObjectValueFrom(ctx, alertScheduleAttr(), alertScheduleModel)
}

func flattenActiveOn(ctx context.Context, activeOn *cxsdk.ActivitySchedule) (types.Object, diag.Diagnostics) {
	if activeOn == nil {
		return types.ObjectNull(alertScheduleActiveOnAttr()), nil
	}

	daysOfWeek, diags := flattenDaysOfWeek(ctx, activeOn.GetDayOfWeek())
	if diags.HasError() {
		return types.ObjectNull(alertScheduleActiveOnAttr()), diags
	}

	startTime, diags := flattenTimeOfDay(ctx, activeOn.GetStartTime())
	if diags.HasError() {
		return types.ObjectNull(alertScheduleActiveOnAttr()), diags
	}

	endTime, diags := flattenTimeOfDay(ctx, activeOn.GetEndTime())
	if diags.HasError() {
		return types.ObjectNull(alertScheduleActiveOnAttr()), diags
	}

	activeOnModel := ActiveOnModel{
		DaysOfWeek: daysOfWeek,
		StartTime:  startTime,
		EndTime:    endTime,
	}
	return types.ObjectValueFrom(ctx, alertScheduleActiveOnAttr(), activeOnModel)
}

func flattenDaysOfWeek(ctx context.Context, daysOfWeek []cxsdk.DayOfWeek) (types.List, diag.Diagnostics) {
	var daysOfWeekStrings []types.String
	for _, dow := range daysOfWeek {
		daysOfWeekStrings = append(daysOfWeekStrings, types.StringValue(daysOfWeekProtoToSchemaMap[dow]))
	}
	return types.ListValueFrom(ctx, types.StringType, daysOfWeekStrings)
}

func flattenTimeOfDay(ctx context.Context, time *cxsdk.TimeOfDay) (types.Object, diag.Diagnostics) {
	if time == nil {
		return types.ObjectNull(timeOfDayAttr()), nil
	}
	return types.ObjectValueFrom(ctx, timeOfDayAttr(), TimeOfDayModel{
		Hours:   types.Int64Value(int64(time.GetHours())),
		Minutes: types.Int64Value(int64(time.GetMinutes())),
	})
}

func flattenLogsTimeRelativeMoreThan(ctx context.Context, logsTimeRelativeMoreThan *cxsdk.LogsTimeRelativeMoreThanTypeDefinition) (types.Object, diag.Diagnostics) {
	if logsTimeRelativeMoreThan == nil {
		return types.ObjectNull(logsTimeRelativeMoreThanAttr()), nil
	}

	logsFilter, diags := flattenAlertsLogsFilter(ctx, logsTimeRelativeMoreThan.GetLogsFilter())
	if diags.HasError() {
		return types.ObjectNull(logsTimeRelativeMoreThanAttr()), diags
	}

	logsTimeRelativeMoreThanModel := LogsTimeRelativeMoreThanModel{
		LogsFilter:                logsFilter,
		NotificationPayloadFilter: wrappedStringSliceToTypeStringSet(logsTimeRelativeMoreThan.GetNotificationPayloadFilter()),
		Threshold:                 wrapperspbUint32ToTypeInt64(logsTimeRelativeMoreThan.GetThreshold()),
		ComparedTo:                types.StringValue(logsTimeRelativeComparedToProtoToSchemaMap[logsTimeRelativeMoreThan.GetComparedTo()]),
		IgnoreInfinity:            wrapperspbBoolToTypeBool(logsTimeRelativeMoreThan.GetIgnoreInfinity()),
	}

	return types.ObjectValueFrom(ctx, logsTimeRelativeMoreThanAttr(), logsTimeRelativeMoreThanModel)
}

func flattenMetricMoreThan(ctx context.Context, metricMoreThan *cxsdk.MetricMoreThanTypeDefinition) (types.Object, diag.Diagnostics) {
	if metricMoreThan == nil {
		return types.ObjectNull(metricMoreThanAttr()), nil
	}

	metricFilter, diags := flattenMetricFilter(ctx, metricMoreThan.GetMetricFilter())
	if diags.HasError() {
		return types.ObjectNull(metricMoreThanAttr()), diags
	}

	ofTheLast, diags := flattenMetricTimeWindow(ctx, metricMoreThan.GetOfTheLast())
	if diags.HasError() {
		return types.ObjectNull(metricMoreThanAttr()), diags
	}

	missingValues, diags := flattenMissingValues(ctx, metricMoreThan.GetMissingValues())
	if diags.HasError() {
		return types.ObjectNull(metricMoreThanAttr()), diags
	}

	metricMoreThanModel := MetricMoreThanModel{
		MetricFilter:  metricFilter,
		Threshold:     wrapperspbFloat64ToTypeFloat64(metricMoreThan.GetThreshold()),
		ForOverPct:    wrapperspbUint32ToTypeInt64(metricMoreThan.GetForOverPct()),
		OfTheLast:     ofTheLast,
		MissingValues: missingValues,
	}
	return types.ObjectValueFrom(ctx, metricMoreThanAttr(), metricMoreThanModel)
}

func flattenMetricFilter(ctx context.Context, filter *cxsdk.MetricFilter) (types.Object, diag.Diagnostics) {
	if filter == nil {
		return types.ObjectNull(metricFilterAttr()), nil
	}

	switch filterType := filter.Type.(type) {
	case *cxsdk.MetricFilter_Promql:
		return types.ObjectValueFrom(ctx, metricFilterAttr(), MetricFilterModel{
			Promql: wrapperspbStringToTypeString(filterType.Promql),
		})
	default:
		return types.ObjectNull(metricFilterAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Metric Filter", fmt.Sprintf("Metric Filter %v is not supported", filterType))}
	}
}

func flattenMetricTimeWindow(ctx context.Context, last *cxsdk.MetricTimeWindow) (types.Object, diag.Diagnostics) {
	if last == nil {
		return types.ObjectNull(metricTimeWindowAttr()), nil
	}

	switch timeWindowType := last.Type.(type) {
	case *cxsdk.MetricTimeWindow_MetricTimeWindowSpecificValue:
		return types.ObjectValueFrom(ctx, metricTimeWindowAttr(), MetricTimeWindowModel{
			SpecificValue: types.StringValue(metricFilterOperationTypeProtoToSchemaMap[timeWindowType.MetricTimeWindowSpecificValue]),
		})
	default:
		return types.ObjectNull(metricTimeWindowAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Time Window", fmt.Sprintf("Time Window %v is not supported", timeWindowType))}
	}
}

func flattenMissingValues(ctx context.Context, missingValues *cxsdk.MetricMissingValues) (types.Object, diag.Diagnostics) {
	if missingValues == nil {
		return types.ObjectNull(metricMissingValuesAttr()), nil
	}

	metricMissingValuesModel := MetricMissingValuesModel{}
	switch missingValuesType := missingValues.MissingValues.(type) {
	case *cxsdk.MetricMissingValues_ReplaceWithZero:
		metricMissingValuesModel.ReplaceWithZero = wrapperspbBoolToTypeBool(missingValuesType.ReplaceWithZero)
	case *cxsdk.MetricMissingValues_MinNonNullValuesPct:
		metricMissingValuesModel.MinNonNullValuesPct = wrapperspbUint32ToTypeInt64(missingValuesType.MinNonNullValuesPct)
	default:
		return types.ObjectNull(metricMissingValuesAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Missing Values", fmt.Sprintf("Missing Values %v is not supported", missingValuesType))}
	}

	return types.ObjectValueFrom(ctx, metricMissingValuesAttr(), metricMissingValuesModel)
}

func flattenMetricLessThan(ctx context.Context, metricLessThan *cxsdk.MetricLessThanTypeDefinition) (types.Object, diag.Diagnostics) {
	if metricLessThan == nil {
		return types.ObjectNull(metricLessThanAttr()), nil
	}

	metricFilter, diags := flattenMetricFilter(ctx, metricLessThan.GetMetricFilter())
	if diags.HasError() {
		return types.ObjectNull(metricLessThanAttr()), diags
	}

	ofTheLast, diags := flattenMetricTimeWindow(ctx, metricLessThan.GetOfTheLast())
	if diags.HasError() {
		return types.ObjectNull(metricLessThanAttr()), diags
	}

	missingValues, diags := flattenMissingValues(ctx, metricLessThan.GetMissingValues())
	if diags.HasError() {
		return types.ObjectNull(metricLessThanAttr()), diags
	}

	undetectedValuesManagement, diags := flattenUndetectedValuesManagement(ctx, metricLessThan.GetUndetectedValuesManagement())
	if diags.HasError() {
		return types.ObjectNull(metricLessThanAttr()), diags
	}

	metricLessThanModel := MetricLessThanModel{
		MetricFilter:               metricFilter,
		Threshold:                  wrapperspbFloat64ToTypeFloat64(metricLessThan.GetThreshold()),
		ForOverPct:                 wrapperspbUint32ToTypeInt64(metricLessThan.GetForOverPct()),
		OfTheLast:                  ofTheLast,
		MissingValues:              missingValues,
		UndetectedValuesManagement: undetectedValuesManagement,
	}
	return types.ObjectValueFrom(ctx, metricLessThanAttr(), metricLessThanModel)
}

func flattenLogsTimeRelativeLessThan(ctx context.Context, timeRelativeLessThan *cxsdk.LogsTimeRelativeLessThanTypeDefinition) (types.Object, diag.Diagnostics) {
	if timeRelativeLessThan == nil {
		return types.ObjectNull(logsTimeRelativeLessThanAttr()), nil
	}

	logsFilter, diags := flattenAlertsLogsFilter(ctx, timeRelativeLessThan.GetLogsFilter())
	if diags.HasError() {
		return types.ObjectNull(logsTimeRelativeLessThanAttr()), diags
	}

	undetectedValuesManagement, diags := flattenUndetectedValuesManagement(ctx, timeRelativeLessThan.GetUndetectedValuesManagement())
	if diags.HasError() {
		return types.ObjectNull(logsTimeRelativeLessThanAttr()), diags
	}

	logsTimeRelativeLessThanModel := LogsTimeRelativeLessThanModel{
		LogsFilter:                 logsFilter,
		NotificationPayloadFilter:  wrappedStringSliceToTypeStringSet(timeRelativeLessThan.GetNotificationPayloadFilter()),
		Threshold:                  wrapperspbUint32ToTypeInt64(timeRelativeLessThan.GetThreshold()),
		ComparedTo:                 types.StringValue(logsTimeRelativeComparedToProtoToSchemaMap[timeRelativeLessThan.GetComparedTo()]),
		IgnoreInfinity:             wrapperspbBoolToTypeBool(timeRelativeLessThan.GetIgnoreInfinity()),
		UndetectedValuesManagement: undetectedValuesManagement,
	}

	return types.ObjectValueFrom(ctx, logsTimeRelativeLessThanAttr(), logsTimeRelativeLessThanModel)
}

func flattenTracingImmediate(ctx context.Context, tracingImmediate *cxsdk.TracingImmediateTypeDefinition) (types.Object, diag.Diagnostics) {
	if tracingImmediate == nil {
		return types.ObjectNull(tracingImmediateAttr()), nil
	}

	tracingQuery, diag := flattenTracingFilter(ctx, tracingImmediate.GetTracingFilter())
	if diag.HasError() {
		return types.ObjectNull(tracingImmediateAttr()), diag
	}

	tracingImmediateModel := TracingImmediateModel{
		TracingFilter:             tracingQuery,
		NotificationPayloadFilter: wrappedStringSliceToTypeStringSet(tracingImmediate.GetNotificationPayloadFilter()),
	}

	return types.ObjectValueFrom(ctx, tracingImmediateAttr(), tracingImmediateModel)
}

func flattenTracingFilter(ctx context.Context, tracingQuery *cxsdk.TracingFilter) (types.Object, diag.Diagnostics) {
	if tracingQuery == nil {
		return types.ObjectNull(tracingQueryAttr()), nil
	}

	tracingQueryModel := &TracingFilterModel{
		LatencyThresholdMs: wrapperspbUint32ToTypeInt64(tracingQuery.GetLatencyThresholdMs()),
	}
	tracingQueryModel, diags := flattenTracingFilterFilters(ctx, tracingQueryModel, tracingQuery)
	if diags.HasError() {
		return types.ObjectNull(tracingQueryAttr()), diags
	}

	return types.ObjectValueFrom(ctx, tracingQueryAttr(), tracingQueryModel)
}

func flattenTracingFilterFilters(ctx context.Context, tracingQueryModel *TracingFilterModel, tracingQuery *cxsdk.TracingFilter) (*TracingFilterModel, diag.Diagnostics) {
	if tracingQuery == nil || tracingQuery.Filters == nil {
		return nil, nil
	}

	var diags diag.Diagnostics
	switch filtersType := tracingQuery.Filters.(type) {
	case *cxsdk.TracingFilter_TracingLabelFilters:
		tracingQueryModel.TracingLabelFilters, diags = flattenTracingLabelFilters(ctx, filtersType.TracingLabelFilters)
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Tracing Query Filters", fmt.Sprintf("Tracing Query Filters %v is not supported", filtersType))}
	}

	return tracingQueryModel, diags
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

func flattenTracingMoreThan(ctx context.Context, tracingMoreThan *cxsdk.TracingMoreThanTypeDefinition) (types.Object, diag.Diagnostics) {
	if tracingMoreThan == nil {
		return types.ObjectNull(tracingMoreThanAttr()), nil
	}

	tracingQuery, diags := flattenTracingFilter(ctx, tracingMoreThan.GetTracingFilter())
	if diags.HasError() {
		return types.ObjectNull(tracingMoreThanAttr()), diags
	}

	timeWindow, diags := flattenTracingTimeWindow(ctx, tracingMoreThan.GetTimeWindow())
	if diags.HasError() {
		return types.ObjectNull(tracingMoreThanAttr()), diags
	}

	tracingMoreThanModel := TracingMoreThanModel{
		TracingFilter:             tracingQuery,
		NotificationPayloadFilter: wrappedStringSliceToTypeStringSet(tracingMoreThan.GetNotificationPayloadFilter()),
		TimeWindow:                timeWindow,
		SpanAmount:                wrapperspbUint32ToTypeInt64(tracingMoreThan.GetSpanAmount()),
	}
	return types.ObjectValueFrom(ctx, tracingMoreThanAttr(), tracingMoreThanModel)
}

func flattenTracingTimeWindow(ctx context.Context, window *cxsdk.TracingTimeWindow) (types.Object, diag.Diagnostics) {
	if window == nil {
		return types.ObjectNull(logsTimeWindowAttr()), nil
	}

	switch timeWindowType := window.Type.(type) {
	case *cxsdk.TracingTimeWindow_TracingTimeWindowValue:
		return types.ObjectValueFrom(ctx, logsTimeWindowAttr(), TracingTimeWindowModel{
			SpecificValue: types.StringValue(tracingTimeWindowProtoToSchemaMap[timeWindowType.TracingTimeWindowValue]),
		})
	default:
		return types.ObjectNull(logsTimeWindowAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Time Window", fmt.Sprintf("Time Window %v is not supported", timeWindowType))}
	}

}

func flattenMetricMoreThanUsual(ctx context.Context, metricMoreThanUsual *cxsdk.MetricMoreThanUsualTypeDefinition) (types.Object, diag.Diagnostics) {
	if metricMoreThanUsual == nil {
		return types.ObjectNull(metricMoreThanUsualAttr()), nil
	}

	metricFilter, diags := flattenMetricFilter(ctx, metricMoreThanUsual.GetMetricFilter())
	if diags.HasError() {
		return types.ObjectNull(metricMoreThanUsualAttr()), diags
	}

	ofTheLast, diags := flattenMetricTimeWindow(ctx, metricMoreThanUsual.GetOfTheLast())
	if diags.HasError() {
		return types.ObjectNull(metricMoreThanUsualAttr()), diags
	}

	metricMoreThanUsualModel := MetricMoreThanUsualModel{
		MetricFilter:        metricFilter,
		OfTheLast:           ofTheLast,
		Threshold:           wrapperspbUint32ToTypeInt64(metricMoreThanUsual.GetThreshold()),
		ForOverPct:          wrapperspbUint32ToTypeInt64(metricMoreThanUsual.GetForOverPct()),
		MinNonNullValuesPct: wrapperspbUint32ToTypeInt64(metricMoreThanUsual.GetMinNonNullValuesPct()),
	}
	return types.ObjectValueFrom(ctx, metricMoreThanUsualAttr(), metricMoreThanUsualModel)
}

func flattenMetricLessThanUsual(ctx context.Context, metricLessThanUsual *cxsdk.MetricLessThanUsualTypeDefinition) (types.Object, diag.Diagnostics) {
	if metricLessThanUsual == nil {
		return types.ObjectNull(metricLessThanUsualAttr()), nil
	}

	metricFilter, diags := flattenMetricFilter(ctx, metricLessThanUsual.GetMetricFilter())
	if diags.HasError() {
		return types.ObjectNull(metricLessThanUsualAttr()), diags
	}

	ofTheLast, diags := flattenMetricTimeWindow(ctx, metricLessThanUsual.GetOfTheLast())
	if diags.HasError() {
		return types.ObjectNull(metricLessThanUsualAttr()), diags
	}

	metricLessThanUsualModel := MetricLessThanUsualModel{
		MetricFilter:        metricFilter,
		OfTheLast:           ofTheLast,
		Threshold:           wrapperspbUint32ToTypeInt64(metricLessThanUsual.GetThreshold()),
		ForOverPct:          wrapperspbUint32ToTypeInt64(metricLessThanUsual.GetForOverPct()),
		MinNonNullValuesPct: wrapperspbUint32ToTypeInt64(metricLessThanUsual.GetMinNonNullValuesPct()),
	}
	return types.ObjectValueFrom(ctx, metricLessThanUsualAttr(), metricLessThanUsualModel)
}

func flattenMetricMoreThanOrEquals(ctx context.Context, equals *cxsdk.MetricMoreThanOrEqualsTypeDefinition) (types.Object, diag.Diagnostics) {
	if equals == nil {
		return types.ObjectNull(metricMoreThanOrEqualsAttr()), nil
	}

	metricFilter, diags := flattenMetricFilter(ctx, equals.GetMetricFilter())
	if diags.HasError() {
		return types.ObjectNull(metricMoreThanOrEqualsAttr()), diags
	}

	ofTheLast, diags := flattenMetricTimeWindow(ctx, equals.GetOfTheLast())
	if diags.HasError() {
		return types.ObjectNull(metricMoreThanOrEqualsAttr()), diags
	}

	missingValues, diags := flattenMissingValues(ctx, equals.GetMissingValues())
	if diags.HasError() {
		return types.ObjectNull(metricMoreThanOrEqualsAttr()), diags
	}

	metricMoreThanOrEqualsModel := MetricMoreThanOrEqualsModel{
		MetricFilter:  metricFilter,
		Threshold:     wrapperspbFloat64ToTypeFloat64(equals.GetThreshold()),
		ForOverPct:    wrapperspbUint32ToTypeInt64(equals.GetForOverPct()),
		OfTheLast:     ofTheLast,
		MissingValues: missingValues,
	}
	return types.ObjectValueFrom(ctx, metricMoreThanOrEqualsAttr(), metricMoreThanOrEqualsModel)
}

func flattenMetricLessThanOrEquals(ctx context.Context, equals *cxsdk.MetricLessThanOrEqualsTypeDefinition) (types.Object, diag.Diagnostics) {
	if equals == nil {
		return types.ObjectNull(metricLessThanOrEqualsAttr()), nil
	}

	metricFilter, diags := flattenMetricFilter(ctx, equals.GetMetricFilter())
	if diags.HasError() {
		return types.ObjectNull(metricLessThanOrEqualsAttr()), diags
	}

	ofTheLast, diags := flattenMetricTimeWindow(ctx, equals.GetOfTheLast())
	if diags.HasError() {
		return types.ObjectNull(metricLessThanOrEqualsAttr()), diags
	}

	missingValues, diags := flattenMissingValues(ctx, equals.GetMissingValues())
	if diags.HasError() {
		return types.ObjectNull(metricLessThanOrEqualsAttr()), diags
	}

	undetectedValuesManagement, diags := flattenUndetectedValuesManagement(ctx, equals.GetUndetectedValuesManagement())
	if diags.HasError() {
		return types.ObjectNull(metricLessThanOrEqualsAttr()), diags
	}

	metricLessThanOrEqualsModel := MetricLessThanOrEqualsModel{
		MetricFilter:               metricFilter,
		Threshold:                  wrapperspbFloat64ToTypeFloat64(equals.GetThreshold()),
		ForOverPct:                 wrapperspbUint32ToTypeInt64(equals.GetForOverPct()),
		OfTheLast:                  ofTheLast,
		MissingValues:              missingValues,
		UndetectedValuesManagement: undetectedValuesManagement,
	}
	return types.ObjectValueFrom(ctx, metricLessThanOrEqualsAttr(), metricLessThanOrEqualsModel)
}

func flattenFlow(ctx context.Context, flow *cxsdk.FlowTypeDefinition) (types.Object, diag.Diagnostics) {
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

func flattenAlertDefs(ctx context.Context, defs []*cxsdk.FlowStagesGroupsAlertDefs) (types.List, diag.Diagnostics) {
	var alertDefs []*FlowStagesGroupsAlertDefsModel
	for _, def := range defs {
		alertDef := &FlowStagesGroupsAlertDefsModel{
			Id:  wrapperspbStringToTypeString(def.GetId()),
			Not: wrapperspbBoolToTypeBool(def.GetNot()),
		}
		alertDefs = append(alertDefs, alertDef)
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: alertDefsAttr()}, alertDefs)
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
		"group_by_fields": types.ListType{
			ElemType: types.StringType,
		},
		"advanced_target_settings": types.SetType{
			ElemType: types.ObjectType{
				AttrTypes: advancedTargetSettingsAttr(),
			},
		},
		"simple_target_settings": types.SetType{
			ElemType: types.ObjectType{
				AttrTypes: simpleTargetSettingsAttr(),
			},
		},
	}
}

func advancedTargetSettingsAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"notify_on": types.StringType,
		"retriggering_period": types.ObjectType{
			AttrTypes: retriggeringPeriodAttr(),
		},
		"integration_id": types.StringType,
		"recipients":     types.SetType{ElemType: types.StringType},
	}
}

func simpleTargetSettingsAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"integration_id": types.StringType,
		"recipients":     types.SetType{ElemType: types.StringType},
	}
}

func alertTypeDefinitionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_immediate": types.ObjectType{
			AttrTypes: logsImmediateAttr(),
		},
		"logs_more_than": types.ObjectType{
			AttrTypes: logsMoreThanAttr(),
		},
		"logs_less_than": types.ObjectType{
			AttrTypes: logsLessThanAttr(),
		},
		"logs_more_than_usual": types.ObjectType{
			AttrTypes: logsMoreThanUsualAttr(),
		},
		"logs_ratio_more_than": types.ObjectType{
			AttrTypes: logsRatioMoreThanAttr(),
		},
		"logs_ratio_less_than": types.ObjectType{
			AttrTypes: logsRatioLessThanAttr(),
		},
		"logs_new_value": types.ObjectType{
			AttrTypes: logsNewValueAttr(),
		},
		"logs_unique_count": types.ObjectType{
			AttrTypes: logsUniqueCountAttr(),
		},
		"logs_time_relative_more_than": types.ObjectType{
			AttrTypes: logsTimeRelativeMoreThanAttr(),
		},
		"logs_time_relative_less_than": types.ObjectType{
			AttrTypes: logsTimeRelativeLessThanAttr(),
		},
		"metric_more_than": types.ObjectType{
			AttrTypes: metricMoreThanAttr(),
		},
		"metric_less_than": types.ObjectType{
			AttrTypes: metricLessThanAttr(),
		},
		"metric_more_than_usual": types.ObjectType{
			AttrTypes: metricMoreThanUsualAttr(),
		},
		"metric_less_than_usual": types.ObjectType{
			AttrTypes: metricLessThanUsualAttr(),
		},
		"metric_more_than_or_equals": types.ObjectType{
			AttrTypes: metricMoreThanOrEqualsAttr(),
		},
		"metric_less_than_or_equals": types.ObjectType{
			AttrTypes: metricLessThanOrEqualsAttr(),
		},
		"tracing_immediate": types.ObjectType{
			AttrTypes: tracingImmediateAttr(),
		},
		"tracing_more_than": types.ObjectType{
			AttrTypes: tracingMoreThanAttr(),
		},
		"flow": types.ObjectType{
			AttrTypes: flowAttr(),
		},
	}
}

func metricLessThanOrEqualsAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"metric_filter": types.ObjectType{
			AttrTypes: metricFilterAttr(),
		},
		"threshold":    types.Int64Type,
		"for_over_pct": types.Int64Type,
		"of_the_last": types.ObjectType{
			AttrTypes: metricTimeWindowAttr(),
		},
		"missing_values": types.ObjectType{
			AttrTypes: metricMissingValuesAttr(),
		},
		"undetected_values_management": types.ObjectType{
			AttrTypes: undetectedValuesManagementAttr(),
		},
	}
}

func metricMoreThanOrEqualsAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"metric_filter": types.ObjectType{
			AttrTypes: metricFilterAttr(),
		},
		"threshold":    types.Int64Type,
		"for_over_pct": types.Int64Type,
		"of_the_last": types.ObjectType{
			AttrTypes: metricTimeWindowAttr(),
		},
		"missing_values": types.ObjectType{
			AttrTypes: metricMissingValuesAttr(),
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
		"lucene_filter": types.ObjectType{
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

func logsMoreThanAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_filter":                 types.ObjectType{AttrTypes: logsFilterAttr()},
		"threshold":                   types.Int64Type,
		"time_window":                 types.ObjectType{AttrTypes: logsTimeWindowAttr()},
		"evaluation_window":           types.StringType,
		"notification_payload_filter": types.SetType{ElemType: types.StringType},
	}
}

func logsTimeWindowAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"specific_value": types.StringType,
	}
}

func logsRatioMoreThanAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"numerator_logs_filter":   types.ObjectType{AttrTypes: logsFilterAttr()},
		"numerator_alias":         types.StringType,
		"denominator_logs_filter": types.ObjectType{AttrTypes: logsFilterAttr()},
		"denominator_alias":       types.StringType,
		"threshold":               types.Int64Type,
		"time_window":             types.ObjectType{AttrTypes: logsTimeWindowAttr()},
		"ignore_infinity":         types.BoolType,
		"notification_payload_filter": types.SetType{
			ElemType: types.StringType,
		},
		"group_by_for": types.StringType,
	}
}

func logsRatioLessThanAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"numerator_logs_filter":   types.ObjectType{AttrTypes: logsFilterAttr()},
		"numerator_alias":         types.StringType,
		"denominator_logs_filter": types.ObjectType{AttrTypes: logsFilterAttr()},
		"denominator_alias":       types.StringType,
		"threshold":               types.Int64Type,
		"time_window":             types.ObjectType{AttrTypes: logsTimeWindowAttr()},
		"ignore_infinity":         types.BoolType,
		"notification_payload_filter": types.SetType{
			ElemType: types.StringType,
		},
		"group_by_for":                 types.StringType,
		"undetected_values_management": types.ObjectType{AttrTypes: undetectedValuesManagementAttr()},
	}
}

func logsMoreThanUsualAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_filter":                 types.ObjectType{AttrTypes: logsFilterAttr()},
		"minimum_threshold":           types.Int64Type,
		"time_window":                 types.ObjectType{AttrTypes: logsTimeWindowAttr()},
		"notification_payload_filter": types.SetType{ElemType: types.StringType},
	}
}

func logsLessThanAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_filter":                  types.ObjectType{AttrTypes: logsFilterAttr()},
		"threshold":                    types.Int64Type,
		"time_window":                  types.ObjectType{AttrTypes: logsTimeWindowAttr()},
		"undetected_values_management": types.ObjectType{AttrTypes: undetectedValuesManagementAttr()},
		"notification_payload_filter":  types.SetType{ElemType: types.StringType},
	}
}

func undetectedValuesManagementAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"trigger_undetected_values": types.BoolType,
		"auto_retire_timeframe":     types.StringType,
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
		"days_of_week": types.ListType{
			ElemType: types.StringType,
		},
		"start_time": types.ObjectType{
			AttrTypes: timeOfDayAttr(),
		},
		"end_time": types.ObjectType{
			AttrTypes: timeOfDayAttr(),
		},
	}
}

func timeOfDayAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"hours":   types.Int64Type,
		"minutes": types.Int64Type,
	}
}

func logsNewValueAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_filter":                 types.ObjectType{AttrTypes: logsFilterAttr()},
		"keypath_to_track":            types.StringType,
		"time_window":                 types.ObjectType{AttrTypes: logsTimeWindowAttr()},
		"notification_payload_filter": types.SetType{ElemType: types.StringType},
	}
}

func logsUniqueCountAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_filter":                       types.ObjectType{AttrTypes: logsFilterAttr()},
		"unique_count_keypath":              types.StringType,
		"max_unique_count":                  types.Int64Type,
		"time_window":                       types.ObjectType{AttrTypes: logsTimeWindowAttr()},
		"notification_payload_filter":       types.SetType{ElemType: types.StringType},
		"max_unique_count_per_group_by_key": types.Int64Type,
	}
}

func metricMoreThanAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"metric_filter":  types.ObjectType{AttrTypes: metricFilterAttr()},
		"threshold":      types.Float64Type,
		"for_over_pct":   types.Int64Type,
		"of_the_last":    types.ObjectType{AttrTypes: metricTimeWindowAttr()},
		"missing_values": types.ObjectType{AttrTypes: metricMissingValuesAttr()},
	}
}

func metricFilterAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"promql": types.StringType,
	}
}

func metricTimeWindowAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"specific_value": types.StringType,
	}
}

func metricMissingValuesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"replace_with_zero":       types.BoolType,
		"min_non_null_values_pct": types.Int64Type,
	}
}

func metricLessThanUsualAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"metric_filter":           types.ObjectType{AttrTypes: metricFilterAttr()},
		"of_the_last":             types.ObjectType{AttrTypes: metricTimeWindowAttr()},
		"threshold":               types.Int64Type,
		"for_over_pct":            types.Int64Type,
		"min_non_null_values_pct": types.Int64Type,
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
		"alert_defs": types.ListType{
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

func tracingMoreThanAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"tracing_filter":              types.ObjectType{AttrTypes: tracingQueryAttr()},
		"notification_payload_filter": types.SetType{ElemType: types.StringType},
		"time_window":                 types.ObjectType{AttrTypes: logsTimeWindowAttr()},
		"span_amount":                 types.Int64Type,
	}
}

func tracingImmediateAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"tracing_filter":              types.ObjectType{AttrTypes: tracingQueryAttr()},
		"notification_payload_filter": types.SetType{ElemType: types.StringType},
	}
}

func metricMoreThanUsualAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"metric_filter":           types.ObjectType{AttrTypes: metricFilterAttr()},
		"of_the_last":             types.ObjectType{AttrTypes: metricTimeWindowAttr()},
		"threshold":               types.Int64Type,
		"for_over_pct":            types.Int64Type,
		"min_non_null_values_pct": types.Int64Type,
	}
}

func metricLessThanAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"metric_filter":                types.ObjectType{AttrTypes: metricFilterAttr()},
		"threshold":                    types.Float64Type,
		"for_over_pct":                 types.Int64Type,
		"of_the_last":                  types.ObjectType{AttrTypes: metricTimeWindowAttr()},
		"missing_values":               types.ObjectType{AttrTypes: metricMissingValuesAttr()},
		"undetected_values_management": types.ObjectType{AttrTypes: undetectedValuesManagementAttr()},
	}
}

func logsTimeRelativeLessThanAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_filter":                  types.ObjectType{AttrTypes: logsFilterAttr()},
		"threshold":                    types.Int64Type,
		"notification_payload_filter":  types.SetType{ElemType: types.StringType},
		"compared_to":                  types.StringType,
		"ignore_infinity":              types.BoolType,
		"undetected_values_management": types.ObjectType{AttrTypes: undetectedValuesManagementAttr()},
	}
}

func logsTimeRelativeMoreThanAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_filter":                 types.ObjectType{AttrTypes: logsFilterAttr()},
		"notification_payload_filter": types.SetType{ElemType: types.StringType},
		"threshold":                   types.Int64Type,
		"compared_to":                 types.StringType,
		"ignore_infinity":             types.BoolType,
	}
}

func tracingQueryAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"latency_threshold_ms":  types.Int64Type,
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

	plan, diags = flattenAlert(ctx, getAlertResp.GetAlertDef())
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
