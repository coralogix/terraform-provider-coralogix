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
	"strings"
	"time"

	"terraform-provider-coralogix/coralogix/clientset"
	alerts "terraform-provider-coralogix/coralogix/clientset/grpc/alerts/v2"

	"google.golang.org/protobuf/encoding/protojson"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	. "github.com/ahmetalpbalkan/go-linq"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	validAlertTypes = []string{
		"standard", "ratio", "new_value", "unique_count", "time_relative", "metric", "tracing", "flow"}
	alertSchemaSeverityToProtoSeverity = map[string]string{
		"Info":     "ALERT_SEVERITY_INFO_OR_UNSPECIFIED",
		"Warning":  "ALERT_SEVERITY_WARNING",
		"Critical": "ALERT_SEVERITY_CRITICAL",
		"Error":    "ALERT_SEVERITY_ERROR",
		"Low":      "ALERT_SEVERITY_LOW",
	}
	alertProtoSeverityToSchemaSeverity       = reverseMapStrings(alertSchemaSeverityToProtoSeverity)
	alertValidSeverities                     = getKeysStrings(alertSchemaSeverityToProtoSeverity)
	alertSchemaLogSeverityToProtoLogSeverity = map[string]string{
		"Debug":    "LOG_SEVERITY_DEBUG_OR_UNSPECIFIED",
		"Verbose":  "LOG_SEVERITY_VERBOSE",
		"Info":     "LOG_SEVERITY_INFO",
		"Warning":  "LOG_SEVERITY_WARNING",
		"Error":    "LOG_SEVERITY_ERROR",
		"Critical": "LOG_SEVERITY_CRITICAL",
	}
	alertProtoLogSeverityToSchemaLogSeverity = reverseMapStrings(alertSchemaLogSeverityToProtoLogSeverity)
	alertValidLogSeverities                  = getKeysStrings(alertSchemaLogSeverityToProtoLogSeverity)
	alertSchemaDayOfWeekToProtoDayOfWeek     = map[string]string{
		"Monday":    "DAY_OF_WEEK_MONDAY_OR_UNSPECIFIED",
		"Tuesday":   "DAY_OF_WEEK_TUESDAY",
		"Wednesday": "DAY_OF_WEEK_WEDNESDAY",
		"Thursday":  "DAY_OF_WEEK_THURSDAY",
		"Friday":    "DAY_OF_WEEK_FRIDAY",
		"Saturday":  "DAY_OF_WEEK_SATURDAY",
		"Sunday":    "DAY_OF_WEEK_SUNDAY",
	}
	alertProtoDayOfWeekToSchemaDayOfWeek = reverseMapStrings(alertSchemaDayOfWeekToProtoDayOfWeek)
	alertValidDaysOfWeek                 = getKeysStrings(alertSchemaDayOfWeekToProtoDayOfWeek)
	alertSchemaTimeFrameToProtoTimeFrame = map[string]string{
		"5Min":  "TIMEFRAME_5_MIN_OR_UNSPECIFIED",
		"10Min": "TIMEFRAME_10_MIN",
		"15Min": "TIMEFRAME_15_MIN",
		"20Min": "TIMEFRAME_20_MIN",
		"30Min": "TIMEFRAME_30_MIN",
		"1H":    "TIMEFRAME_1_H",
		"2H":    "TIMEFRAME_2_H",
		"4H":    "TIMEFRAME_4_H",
		"6H":    "TIMEFRAME_6_H",
		"12H":   "TIMEFRAME_12_H",
		"24H":   "TIMEFRAME_24_H",
		"36H":   "TIMEFRAME_36_H",
	}
	alertProtoTimeFrameToSchemaTimeFrame            = reverseMapStrings(alertSchemaTimeFrameToProtoTimeFrame)
	alertValidTimeFrames                            = getKeysStrings(alertSchemaTimeFrameToProtoTimeFrame)
	alertSchemaUniqueCountTimeFrameToProtoTimeFrame = map[string]string{
		"1Min":  "TIMEFRAME_1_MIN",
		"5Min":  "TIMEFRAME_5_MIN_OR_UNSPECIFIED",
		"10Min": "TIMEFRAME_10_MIN",
		"15Min": "TIMEFRAME_15_MIN",
		"20Min": "TIMEFRAME_20_MIN",
		"30Min": "TIMEFRAME_30_MIN",
		"1H":    "TIMEFRAME_1_H",
		"2H":    "TIMEFRAME_2_H",
		"4H":    "TIMEFRAME_4_H",
		"6H":    "TIMEFRAME_6_H",
		"12H":   "TIMEFRAME_12_H",
		"24H":   "TIMEFRAME_24_H",
	}
	alertProtoUniqueCountTimeFrameToSchemaTimeFrame = reverseMapStrings(alertSchemaUniqueCountTimeFrameToProtoTimeFrame)
	alertValidUniqueCountTimeFrames                 = getKeysStrings(alertSchemaUniqueCountTimeFrameToProtoTimeFrame)
	alertSchemaNewValueTimeFrameToProtoTimeFrame    = map[string]string{
		"12H":    "TIMEFRAME_12_H",
		"24H":    "TIMEFRAME_24_H",
		"48H":    "TIMEFRAME_48_H",
		"72H":    "TIMEFRAME_72_H",
		"1W":     "TIMEFRAME_1_W",
		"1Month": "TIMEFRAME_1_M",
		"2Month": "TIMEFRAME_2_M",
		"3Month": "TIMEFRAME_3_M",
	}
	alertProtoNewValueTimeFrameToSchemaTimeFrame                     = reverseMapStrings(alertSchemaNewValueTimeFrameToProtoTimeFrame)
	alertValidNewValueTimeFrames                                     = getKeysStrings(alertSchemaNewValueTimeFrameToProtoTimeFrame)
	alertSchemaRelativeTimeFrameToProtoTimeFrameAndRelativeTimeFrame = map[string]protoTimeFrameAndRelativeTimeFrame{
		"Previous_hour":       {timeFrame: alerts.Timeframe_TIMEFRAME_1_H, relativeTimeFrame: alerts.RelativeTimeframe_RELATIVE_TIMEFRAME_HOUR_OR_UNSPECIFIED},
		"Same_hour_yesterday": {timeFrame: alerts.Timeframe_TIMEFRAME_1_H, relativeTimeFrame: alerts.RelativeTimeframe_RELATIVE_TIMEFRAME_DAY},
		"Same_hour_last_week": {timeFrame: alerts.Timeframe_TIMEFRAME_1_H, relativeTimeFrame: alerts.RelativeTimeframe_RELATIVE_TIMEFRAME_WEEK},
		"Yesterday":           {timeFrame: alerts.Timeframe_TIMEFRAME_24_H, relativeTimeFrame: alerts.RelativeTimeframe_RELATIVE_TIMEFRAME_DAY},
		"Same_day_last_week":  {timeFrame: alerts.Timeframe_TIMEFRAME_24_H, relativeTimeFrame: alerts.RelativeTimeframe_RELATIVE_TIMEFRAME_WEEK},
		"Same_day_last_month": {timeFrame: alerts.Timeframe_TIMEFRAME_24_H, relativeTimeFrame: alerts.RelativeTimeframe_RELATIVE_TIMEFRAME_MONTH},
	}
	alertProtoTimeFrameAndRelativeTimeFrameToSchemaRelativeTimeFrame = reverseMapRelativeTimeFrame(alertSchemaRelativeTimeFrameToProtoTimeFrameAndRelativeTimeFrame)
	alertValidRelativeTimeFrames                                     = getKeysRelativeTimeFrame(alertSchemaRelativeTimeFrameToProtoTimeFrameAndRelativeTimeFrame)
	alertSchemaArithmeticOperatorToProtoArithmetic                   = map[string]string{
		"Avg":        "ARITHMETIC_OPERATOR_AVG_OR_UNSPECIFIED",
		"Min":        "ARITHMETIC_OPERATOR_MIN",
		"Max":        "ARITHMETIC_OPERATOR_MAX",
		"Sum":        "ARITHMETIC_OPERATOR_SUM",
		"Count":      "ARITHMETIC_OPERATOR_COUNT",
		"Percentile": "ARITHMETIC_OPERATOR_PERCENTILE",
	}
	alertProtoArithmeticOperatorToSchemaArithmetic   = reverseMapStrings(alertSchemaArithmeticOperatorToProtoArithmetic)
	alertValidArithmeticOperators                    = getKeysStrings(alertSchemaArithmeticOperatorToProtoArithmetic)
	alertValidFlowOperator                           = getKeysInt32(alerts.FlowOperator_value)
	alertSchemaMetricTimeFrameToMetricProtoTimeFrame = map[string]string{
		"1Min":  "TIMEFRAME_1_MIN",
		"5Min":  "TIMEFRAME_5_MIN_OR_UNSPECIFIED",
		"10Min": "TIMEFRAME_10_MIN",
		"15Min": "TIMEFRAME_15_MIN",
		"20Min": "TIMEFRAME_20_MIN",
		"30Min": "TIMEFRAME_30_MIN",
		"1H":    "TIMEFRAME_1_H",
		"2H":    "TIMEFRAME_2_H",
		"4H":    "TIMEFRAME_4_H",
		"6H":    "TIMEFRAME_6_H",
		"12H":   "TIMEFRAME_12_H",
		"24H":   "TIMEFRAME_24_H",
	}
	alertProtoMetricTimeFrameToMetricSchemaTimeFrame = reverseMapStrings(alertSchemaMetricTimeFrameToMetricProtoTimeFrame)
	alertValidMetricTimeFrames                       = getKeysStrings(alertSchemaMetricTimeFrameToMetricProtoTimeFrame)
	alertSchemaDeadmanRatiosToProtoDeadmanRatios     = map[string]string{
		"Never": "CLEANUP_DEADMAN_DURATION_NEVER_OR_UNSPECIFIED",
		"5Min":  "CLEANUP_DEADMAN_DURATION_5MIN",
		"10Min": "CLEANUP_DEADMAN_DURATION_10MIN",
		"1H":    "CLEANUP_DEADMAN_DURATION_1H",
		"2H":    "CLEANUP_DEADMAN_DURATION_2H",
		"6H":    "CLEANUP_DEADMAN_DURATION_6H",
		"12H":   "CLEANUP_DEADMAN_DURATION_12H",
		"24H":   "CLEANUP_DEADMAN_DURATION_24H",
	}
	alertProtoDeadmanRatiosToSchemaDeadmanRatios = reverseMapStrings(alertSchemaDeadmanRatiosToProtoDeadmanRatios)
	alertValidDeadmanRatioValues                 = getKeysStrings(alertSchemaDeadmanRatiosToProtoDeadmanRatios)
	validTimeZones                               = []string{"UTC-11", "UTC-10", "UTC-9", "UTC-8", "UTC-7", "UTC-6", "UTC-5", "UTC-4", "UTC-3", "UTC-2", "UTC-1",
		"UTC+0", "UTC+1", "UTC+2", "UTC+3", "UTC+4", "UTC+5", "UTC+6", "UTC+7", "UTC+8", "UTC+9", "UTC+10", "UTC+11", "UTC+12", "UTC+13", "UTC+14"}
	alertSchemaNotifyOnToProtoNotifyOn = map[string]alerts.NotifyOn{
		"Triggered_only":         alerts.NotifyOn_TRIGGERED_ONLY,
		"Triggered_and_resolved": alerts.NotifyOn_TRIGGERED_AND_RESOLVED,
	}
	alertProtoNotifyOnToSchemaNotifyOn = map[alerts.NotifyOn]string{
		alerts.NotifyOn_TRIGGERED_ONLY:         "Triggered_only",
		alerts.NotifyOn_TRIGGERED_AND_RESOLVED: "Triggered_and_resolved",
	}
	validNotifyOn                      = []string{"Triggered_only", "Triggered_and_resolved"}
	alertSchemaToProtoEvaluationWindow = map[string]alerts.EvaluationWindow{
		"Rolling": alerts.EvaluationWindow_EVALUATION_WINDOW_ROLLING_OR_UNSPECIFIED,
		"Dynamic": alerts.EvaluationWindow_EVALUATION_WINDOW_DYNAMIC,
	}
	alertProtoToSchemaEvaluationWindow = map[alerts.EvaluationWindow]string{
		alerts.EvaluationWindow_EVALUATION_WINDOW_ROLLING_OR_UNSPECIFIED: "Rolling",
		alerts.EvaluationWindow_EVALUATION_WINDOW_DYNAMIC:                "Dynamic",
	}
	validEvaluationWindow = []string{"Rolling", "Dynamic"}
	createAlertURL        = "com.coralogix.alerts.v2.AlertService/CreateAlert"
	getAlertURL           = "com.coralogix.alerts.v2.AlertService/GetAlertByUniqueId"
	updateAlertURL        = "com.coralogix.alerts.v2.AlertService/UpdateAlertByUniqueId"
	deleteAlertURL        = "com.coralogix.alerts.v2.AlertService/DeleteAlertByUniqueId"
)

type alertParams struct {
	Condition *alerts.AlertCondition
	Filters   *alerts.AlertFilters
}

type protoTimeFrameAndRelativeTimeFrame struct {
	timeFrame         alerts.Timeframe
	relativeTimeFrame alerts.RelativeTimeframe
}

func resourceCoralogixAlert() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCoralogixAlertCreate,
		ReadContext:   resourceCoralogixAlertRead,
		UpdateContext: resourceCoralogixAlertUpdate,
		DeleteContext: resourceCoralogixAlertDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(60 * time.Second),
			Read:   schema.DefaultTimeout(30 * time.Second),
			Update: schema.DefaultTimeout(60 * time.Second),
			Delete: schema.DefaultTimeout(30 * time.Second),
		},

		Schema: AlertSchema(),

		Description: "Coralogix alert. More info: https://coralogix.com/docs/alerts-api/ .",
	}
}

func AlertSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"enabled": {
			Type:        schema.TypeBool,
			Optional:    true,
			Default:     true,
			Description: "Determines whether the alert will be active. True by default.",
		},
		"name": {
			Type:         schema.TypeString,
			Required:     true,
			ValidateFunc: validation.StringIsNotEmpty,
			Description:  "Alert name.",
		},
		"description": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "Alert description.",
		},
		"severity": {
			Type:         schema.TypeString,
			Required:     true,
			ValidateFunc: validation.StringInSlice(alertValidSeverities, false),
			Description:  fmt.Sprintf("Determines the alert's severity. Can be one of %q", alertValidSeverities),
		},
		"meta_labels": {
			Type: schema.TypeMap,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
			Optional:         true,
			Description:      "Labels allow you to easily filter by alert type and create views. Insert a new label or use an existing one. You can nest a label using key:value.",
			ValidateDiagFunc: validation.MapKeyMatch(regexp.MustCompile(`^[A-Za-z\d_-]*$`), "not valid key for meta_label"),
		},
		"expiration_date": {
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"day": {
						Type:         schema.TypeInt,
						Required:     true,
						ValidateFunc: validation.IntBetween(1, 31),
						Description:  `Day of a month. Must be from 1 to 31 and valid for the year and month.`,
					},
					"month": {
						Type:         schema.TypeInt,
						Required:     true,
						ValidateFunc: validation.IntBetween(1, 12),
						Description:  `Month of a year. Must be from 1 to 12.`,
					},
					"year": {
						Type:         schema.TypeInt,
						Required:     true,
						ValidateFunc: validation.IntBetween(1, 9999),
						Description:  `Year of the date. Must be from 1 to 9999.`,
					},
				},
			},
			Description: "The expiration date of the alert (if declared).",
		},
		"notifications_group": {
			Type:        schema.TypeSet,
			Optional:    true,
			Computed:    true,
			Elem:        notificationGroupSchema(),
			Set:         schema.HashResource(notificationGroupSchema()),
			Description: "Defines notifications settings over list of group-by keys (or on empty list).",
		},
		"payload_filters": {
			Type:     schema.TypeSet,
			Optional: true,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
			Description: "A list of log fields out of the log example which will be included with the alert notification.",
			Set:         schema.HashString,
		},
		"incident_settings": {
			Type:     schema.TypeList,
			MaxItems: 1,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"retriggering_period_minutes": {
						Type:         schema.TypeInt,
						Required:     true,
						ValidateFunc: validation.IntAtLeast(1),
					},
					"notify_on": {
						Type:         schema.TypeString,
						Optional:     true,
						Default:      "Triggered_only",
						ValidateFunc: validation.StringInSlice(validNotifyOn, false),
						Description:  fmt.Sprintf("Defines the alert's triggering logic. Can be one of %q. Triggered_and_resolved conflicts with new_value, unique_count and flow alerts, and with immediately and more_than_usual conditions", validNotifyOn),
					},
				},
			},
			//AtLeastOneOf: []string{"notifications_group", "show_in_insights", "incident_settings"},
		},
		"scheduling": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: schedulingSchema(),
			},
			MaxItems:    1,
			Description: "Limit the triggering of this alert to specific time frames. Active always by default.",
		},
		"standard": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: standardSchema(),
			},
			MaxItems:     1,
			ExactlyOneOf: validAlertTypes,
			Description:  "Alert based on number of log occurrences.",
		},
		"ratio": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: ratioSchema(),
			},
			MaxItems:     1,
			ExactlyOneOf: validAlertTypes,
			Description:  "Alert based on the ratio between queries.",
		},
		"new_value": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: newValueSchema(),
			},
			MaxItems:     1,
			ExactlyOneOf: validAlertTypes,
			Description:  "Alert on never before seen log value.",
		},
		"unique_count": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: uniqueCountSchema(),
			},
			MaxItems:     1,
			ExactlyOneOf: validAlertTypes,
			Description:  "Alert based on unique value count per key.",
		},
		"time_relative": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: timeRelativeSchema(),
			},
			MaxItems:     1,
			ExactlyOneOf: validAlertTypes,
			Description:  "Alert based on ratio between timeframes.",
		},
		"metric": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: metricSchema(),
			},
			MaxItems:     1,
			ExactlyOneOf: validAlertTypes,
			Description:  "Alert based on arithmetic operators for metrics.",
		},
		"tracing": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: tracingSchema(),
			},
			MaxItems:     1,
			ExactlyOneOf: validAlertTypes,
			Description:  "Alert based on tracing latency.",
		},
		"flow": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: flowSchema(),
			},
			MaxItems:     1,
			ExactlyOneOf: validAlertTypes,
			Description:  "Alert based on a combination of alerts in a specific timeframe.",
		},
	}
}

func notificationGroupSchema() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"group_by_fields": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "List of group-by fields to apply the notification logic on (can be empty). Every notification should contain unique group_by_fields permutation (the order doesn't matter).",
			},
			"notification": {
				Type:        schema.TypeSet,
				Optional:    true,
				Elem:        notificationSubgroupSchema(),
				Set:         schema.HashResource(notificationSubgroupSchema()),
				Description: "Defines notification logic with optional recipients. Can contain single webhook or email recipients list.",
			},
		},
	}
}

func notificationSubgroupSchema() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"retriggering_period_minutes": {
				Type:         schema.TypeInt,
				Optional:     true,
				ValidateFunc: validation.IntAtLeast(1),
				Description: "By default, retriggering_period_minutes will be populated with min for immediate," +
					" more_than and more_than_usual alerts. For less_than alert it will be populated with the chosen time" +
					" frame for the less_than condition (in minutes). You may choose to change the suppress window so the " +
					"alert will be suppressed for a longer period.",
				ExactlyOneOf: []string{"incident_settings"},
			},
			"notify_on": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice(validNotifyOn, false),
				Description:  fmt.Sprintf("Defines the alert's triggering logic. Can be one of %q. Triggered_and_resolved conflicts with new_value, unique_count and flow alerts, and with immediately and more_than_usual conditions", validNotifyOn),
				ExactlyOneOf: []string{"incident_settings"},
			},
			"integration_id": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringIsNotEmpty,
				Description:  "Conflicts with emails.",
			},
			"email_recipients": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
					//ValidateDiagFunc: mailValidationFunc(),
				},
				Set:         schema.HashString,
				Description: "Conflicts with integration_id.",
			},
		},
	}
}

func schedulingSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"time_zone": {
			Type:         schema.TypeString,
			Optional:     true,
			Default:      "UTC+0",
			ValidateFunc: validation.StringInSlice(validTimeZones, false),
			Description:  fmt.Sprintf("Specifies the time zone to be used in interpreting the schedule. Can be one of %q", validTimeZones),
		},
		"time_frame": {
			Type:        schema.TypeSet,
			MaxItems:    1,
			Required:    true,
			Elem:        timeFrames(),
			Set:         hashTimeFrames(),
			Description: "time_frame is a set of days and hours when the alert will be active. ***Currently, supported only for one time_frame***",
		},
	}
}

func timeFrames() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"days_enabled": {
				Type:     schema.TypeSet,
				Required: true,
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: validation.StringInSlice(alertValidDaysOfWeek, false),
				},
				Description: fmt.Sprintf("Days of week. Can be one of %q", alertValidDaysOfWeek),
				Set:         schema.HashString,
			},
			"start_time": timeInDaySchema(`Limit the triggering of this alert to start at specific hour.`),
			"end_time":   timeInDaySchema(`Limit the triggering of this alert to end at specific hour.`),
		},
	}
}

func hashTimeFrames() schema.SchemaSetFunc {
	return schema.HashResource(timeFrames())
}

func commonAlertSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"search_query": searchQuerySchema(),
		"severities": {
			Type:     schema.TypeSet,
			Optional: true,
			Elem: &schema.Schema{
				Type:         schema.TypeString,
				ValidateFunc: validation.StringInSlice(alertValidLogSeverities, false),
			},
			Description: fmt.Sprintf("An array of log severities that we interested in. Can be one of %q", alertValidLogSeverities),
			Set:         schema.HashString,
		},
		"applications": {
			Type:     schema.TypeSet,
			Optional: true,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
			Description: "An array that contains log’s application names that we want to be alerted on." +
				" Applications can be filtered by prefix, suffix, and contains using the next patterns - filter:startsWith:xxx, filter:endsWith:xxx, filter:contains:xxx",
			Set: schema.HashString,
		},
		"subsystems": {
			Type:     schema.TypeSet,
			Optional: true,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
			Description: "An array that contains log’s subsystem names that we want to be notified on. " +
				"Subsystems can be filtered by prefix, suffix, and contains using the next patterns - filter:startsWith:xxx, filter:endsWith:xxx, filter:contains:xxx",
			Set: schema.HashString,
		},
		"categories": {
			Type:     schema.TypeSet,
			Optional: true,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
			Description: "An array that contains log’s categories that we want to be notified on.",
			Set:         schema.HashString,
		},
		"computers": {
			Type:     schema.TypeSet,
			Optional: true,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
			Description: "An array that contains log’s computer names that we want to be notified on.",
			Set:         schema.HashString,
		},
		"classes": {
			Type:     schema.TypeSet,
			Optional: true,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
			Description: "An array that contains log’s class names that we want to be notified on.",
			Set:         schema.HashString,
		},
		"methods": {
			Type:     schema.TypeSet,
			Optional: true,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
			Description: "An array that contains log’s method names that we want to be notified on.",
			Set:         schema.HashString,
		},
		"ip_addresses": {
			Type:     schema.TypeSet,
			Optional: true,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
			Description: "An array that contains log’s IP addresses that we want to be notified on.",
			Set:         schema.HashString,
		},
	}
}

func searchQuerySchema() *schema.Schema {
	return &schema.Schema{
		Type:        schema.TypeString,
		Optional:    true,
		Description: "The search_query that we wanted to be notified on.",
	}
}

func standardSchema() map[string]*schema.Schema {
	standardSchema := commonAlertSchema()
	standardSchema["condition"] = &schema.Schema{
		Type:     schema.TypeList,
		Required: true,
		MaxItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"immediately": {
					Type:     schema.TypeBool,
					Optional: true,
					ExactlyOneOf: []string{"standard.0.condition.0.immediately",
						"standard.0.condition.0.more_than",
						"standard.0.condition.0.less_than",
						"standard.0.condition.0.more_than_usual"},
					Description: "Determines the condition operator." +
						" Must be one of - immediately, less_than, more_than or more_than_usual.",
				},
				"less_than": {
					Type:     schema.TypeBool,
					Optional: true,
					ExactlyOneOf: []string{"standard.0.condition.0.immediately",
						"standard.0.condition.0.more_than",
						"standard.0.condition.0.less_than",
						"standard.0.condition.0.more_than_usual"},
					Description: "Determines the condition operator." +
						" Must be one of - immediately, less_than, more_than or more_than_usual.",
					RequiredWith: []string{"standard.0.condition.0.time_window", "standard.0.condition.0.threshold"},
				},
				"more_than": {
					Type:     schema.TypeBool,
					Optional: true,
					ExactlyOneOf: []string{"standard.0.condition.0.immediately",
						"standard.0.condition.0.more_than",
						"standard.0.condition.0.less_than",
						"standard.0.condition.0.more_than_usual"},
					RequiredWith: []string{"standard.0.condition.0.time_window", "standard.0.condition.0.threshold"},
					Description: "Determines the condition operator." +
						" Must be one of - immediately, less_than, more_than or more_than_usual.",
				},
				"more_than_usual": {
					Type:     schema.TypeBool,
					Optional: true,
					ExactlyOneOf: []string{"standard.0.condition.0.immediately",
						"standard.0.condition.0.more_than",
						"standard.0.condition.0.less_than",
						"standard.0.condition.0.more_than_usual"},
					Description: "Determines the condition operator." +
						" Must be one of - immediately, less_than, more_than or more_than_usual.",
				},
				"threshold": {
					Type:          schema.TypeInt,
					Optional:      true,
					ConflictsWith: []string{"standard.0.condition.0.immediately"},
					Description:   "The number of log occurrences that is needed to trigger the alert.",
				},
				"time_window": {
					Type:          schema.TypeString,
					Optional:      true,
					ValidateFunc:  validation.StringInSlice(alertValidTimeFrames, false),
					ConflictsWith: []string{"standard.0.condition.0.immediately"},
					Description:   fmt.Sprintf("The bounded time frame for the threshold to be occurred within, to trigger the alert. Can be one of %q", alertValidTimeFrames),
				},
				"group_by": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
					ConflictsWith: []string{"standard.0.condition.0.immediately"},
					Description:   "The fields to 'group by' on. In case of immediately = true switch to group_by_key.",
				},
				"group_by_key": {
					Type:          schema.TypeString,
					Optional:      true,
					ConflictsWith: []string{"standard.0.condition.0.more_than", "standard.0.condition.0.less_than", "standard.0.condition.0.more_than_usual"},
					Description:   "The key to 'group by' on. When immediately = true, 'group_by_key' (single string) can be set instead of 'group_by'.",
				},
				"manage_undetected_values": {
					Type:     schema.TypeList,
					Optional: true,
					Computed: true,
					MaxItems: 1,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"enable_triggering_on_undetected_values": {
								Type:        schema.TypeBool,
								Required:    true,
								Description: "Determines whether the deadman-option is enabled. When set to true, auto_retire_ratio is required otherwise auto_retire_ratio should be omitted.",
							},
							"auto_retire_ratio": {
								Type:         schema.TypeString,
								Optional:     true,
								ValidateFunc: validation.StringInSlice(alertValidDeadmanRatioValues, false),
								Description:  fmt.Sprintf("Defines the triggering auto-retire ratio. Can be one of %q", alertValidDeadmanRatioValues),
							},
						},
					},
					RequiredWith: []string{"standard.0.condition.0.less_than", "standard.0.condition.0.group_by"},
					Description:  "Manage your logs undetected values - when relevant, enable/disable triggering on undetected values and change the auto retire interval. By default (when relevant), triggering is enabled with retire-ratio=NEVER.",
				},
				"evaluation_window": {
					Type:         schema.TypeString,
					Optional:     true,
					Computed:     true,
					ValidateFunc: validation.StringInSlice(validEvaluationWindow, false),
					RequiredWith: []string{"standard.0.condition.0.more_than"},
					Description:  fmt.Sprintf("Defines the evaluation-window logic to determine if the threshold has been crossed. Relevant only for more_than condition. Can be one of %q.", validEvaluationWindow),
				},
			},
		},
		Description: "Defines the conditions for triggering and notify by the alert",
	}
	return standardSchema
}

func ratioSchema() map[string]*schema.Schema {
	query1Schema := commonAlertSchema()
	query1Schema["alias"] = &schema.Schema{
		Type:        schema.TypeString,
		Optional:    true,
		Default:     "Query 1",
		Description: "Query1 alias.",
	}

	return map[string]*schema.Schema{
		"query_1": {
			Type:     schema.TypeList,
			Required: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: query1Schema,
			},
		},
		"query_2": {
			Type:     schema.TypeList,
			Required: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"alias": {
						Type:        schema.TypeString,
						Optional:    true,
						Default:     "Query 2",
						Description: "Query2 alias.",
					},
					"search_query": searchQuerySchema(),
					"severities": {
						Type:     schema.TypeSet,
						Optional: true,
						Elem: &schema.Schema{
							Type:         schema.TypeString,
							ValidateFunc: validation.StringInSlice(alertValidLogSeverities, false),
						},
						Description: fmt.Sprintf("An array of log severities that we interested in. Can be one of %q", alertValidLogSeverities),
						Set:         schema.HashString,
					},
					"applications": {
						Type:     schema.TypeSet,
						Optional: true,
						Elem: &schema.Schema{
							Type: schema.TypeString,
						},
						Description: "An array that contains log’s application names that we want to be alerted on." +
							" Applications can be filtered by prefix, suffix, and contains using the next patterns - filter:startsWith:xxx, filter:endsWith:xxx, filter:contains:xxx",
						Set: schema.HashString,
					},
					"subsystems": {
						Type:     schema.TypeSet,
						Optional: true,
						Elem: &schema.Schema{
							Type: schema.TypeString,
						},
						Description: "An array that contains log’s subsystem names that we want to be notified on. " +
							"Subsystems can be filtered by prefix, suffix, and contains using the next patterns - filter:startsWith:xxx, filter:endsWith:xxx, filter:contains:xxx",
						Set: schema.HashString,
					},
				},
			},
		},
		"condition": {
			Type:     schema.TypeList,
			Required: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"more_than": {
						Type:         schema.TypeBool,
						Optional:     true,
						ExactlyOneOf: []string{"ratio.0.condition.0.more_than", "ratio.0.condition.0.less_than"},
						Description: "Determines the condition operator." +
							" Must be one of - less_than or more_than.",
					},
					"less_than": {
						Type:         schema.TypeBool,
						Optional:     true,
						ExactlyOneOf: []string{"ratio.0.condition.0.more_than", "ratio.0.condition.0.less_than"},
					},
					"ratio_threshold": {
						Type:        schema.TypeFloat,
						Required:    true,
						Description: "The ratio(between the queries) threshold that is needed to trigger the alert.",
					},
					"time_window": {
						Type:         schema.TypeString,
						Required:     true,
						ValidateFunc: validation.StringInSlice(alertValidTimeFrames, false),
						Description:  fmt.Sprintf("The bounded time frame for the threshold to be occurred within, to trigger the alert. Can be one of %q", alertValidTimeFrames),
					},
					"ignore_infinity": {
						Type:          schema.TypeBool,
						Optional:      true,
						ConflictsWith: []string{"ratio.0.condition.0.less_than"},
						Description:   "Not triggered when threshold is infinity (divided by zero).",
					},
					"group_by": {
						Type:     schema.TypeList,
						Optional: true,
						Elem: &schema.Schema{
							Type: schema.TypeString,
						},
						Description: "The fields to 'group by' on.",
					},
					"group_by_q1": {
						Type:         schema.TypeBool,
						Optional:     true,
						RequiredWith: []string{"ratio.0.condition.0.group_by"},
						ConflictsWith: []string{"ratio.0.condition.0.group_by_q2",
							"ratio.0.condition.0.group_by_both"},
					},
					"group_by_q2": {
						Type:         schema.TypeBool,
						Optional:     true,
						RequiredWith: []string{"ratio.0.condition.0.group_by"},
						ConflictsWith: []string{"ratio.0.condition.0.group_by_q1",
							"ratio.0.condition.0.group_by_both"},
					},
					"group_by_both": {
						Type:         schema.TypeBool,
						Optional:     true,
						RequiredWith: []string{"ratio.0.condition.0.group_by"},
						ConflictsWith: []string{"ratio.0.condition.0.group_by_q1",
							"ratio.0.condition.0.group_by_q2"},
					},
					"manage_undetected_values": {
						Type:     schema.TypeList,
						Optional: true,
						Computed: true,
						MaxItems: 1,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"enable_triggering_on_undetected_values": {
									Type:        schema.TypeBool,
									Required:    true,
									Description: "Determines whether the deadman-option is enabled. When set to true, auto_retire_ratio is required otherwise auto_retire_ratio should be omitted.",
								},
								"auto_retire_ratio": {
									Type:         schema.TypeString,
									Optional:     true,
									ValidateFunc: validation.StringInSlice(alertValidDeadmanRatioValues, false),
									Description:  fmt.Sprintf("Defines the triggering auto-retire ratio. Can be one of %q", alertValidDeadmanRatioValues),
								},
							},
						},
						RequiredWith: []string{"ratio.0.condition.0.less_than", "ratio.0.condition.0.group_by"},
						Description:  "Manage your logs undetected values - when relevant, enable/disable triggering on undetected values and change the auto retire interval. By default (when relevant), triggering is enabled with retire-ratio=NEVER.",
					},
				},
			},
			Description: "Defines the conditions for triggering and notify by the alert",
		},
	}
}

func newValueSchema() map[string]*schema.Schema {
	newValueSchema := commonAlertSchema()
	newValueSchema["condition"] = &schema.Schema{
		Type:     schema.TypeList,
		Required: true,
		MaxItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"key_to_track": {
					Type:         schema.TypeString,
					Required:     true,
					ValidateFunc: validation.StringIsNotEmpty,
					Description: "Select a key to track. Note, this key needs to have less than 50K unique values in" +
						" the defined timeframe.",
				},
				"time_window": {
					Type:         schema.TypeString,
					Required:     true,
					ValidateFunc: validation.StringInSlice(alertValidNewValueTimeFrames, false),
					Description:  fmt.Sprintf("The bounded time frame for the threshold to be occurred within, to trigger the alert. Can be one of %q", alertValidNewValueTimeFrames),
				},
			},
		},
		Description: "Defines the conditions for triggering and notify by the alert",
	}
	return newValueSchema
}

func uniqueCountSchema() map[string]*schema.Schema {
	uniqueCountSchema := commonAlertSchema()
	uniqueCountSchema["condition"] = &schema.Schema{
		Type:     schema.TypeList,
		Required: true,
		MaxItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"unique_count_key": {
					Type:         schema.TypeString,
					Required:     true,
					ValidateFunc: validation.StringIsNotEmpty,
					Description:  "Defines the key to match to track its unique count.",
				},
				"max_unique_values": {
					Type:     schema.TypeInt,
					Required: true,
				},
				"time_window": {
					Type:         schema.TypeString,
					Required:     true,
					ValidateFunc: validation.StringInSlice(alertValidUniqueCountTimeFrames, false),
					Description:  fmt.Sprintf("The bounded time frame for the threshold to be occurred within, to trigger the alert. Can be one of %q", alertValidUniqueCountTimeFrames),
				},
				"group_by_key": {
					Type:         schema.TypeString,
					Optional:     true,
					RequiredWith: []string{"unique_count.0.condition.0.max_unique_values_for_group_by"},
					Description:  "The key to 'group by' on.",
				},
				"max_unique_values_for_group_by": {
					Type:         schema.TypeInt,
					Optional:     true,
					RequiredWith: []string{"unique_count.0.condition.0.group_by_key"},
				},
			},
		},
		Description: "Defines the conditions for triggering and notify by the alert",
	}
	return uniqueCountSchema
}

func timeRelativeSchema() map[string]*schema.Schema {
	timeRelativeSchema := commonAlertSchema()
	timeRelativeSchema["condition"] = &schema.Schema{
		Type:     schema.TypeList,
		Required: true,
		MaxItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"less_than": {
					Type:     schema.TypeBool,
					Optional: true,
					ExactlyOneOf: []string{"time_relative.0.condition.0.more_than",
						"time_relative.0.condition.0.less_than"},
					Description: "Determines the condition operator." +
						" Must be one of - less_than or more_than.",
				},
				"more_than": {
					Type:     schema.TypeBool,
					Optional: true,
					ExactlyOneOf: []string{"time_relative.0.condition.0.more_than",
						"time_relative.0.condition.0.less_than"},
					Description: "Determines the condition operator." +
						" Must be one of - less_than or more_than.",
				},
				"ratio_threshold": {
					Type:        schema.TypeFloat,
					Required:    true,
					Description: "The ratio threshold that is needed to trigger the alert.",
				},
				"relative_time_window": {
					Type:         schema.TypeString,
					Required:     true,
					ValidateFunc: validation.StringInSlice(alertValidRelativeTimeFrames, false),
					Description:  fmt.Sprintf("Time-window to compare with. Can be one of %q.", alertValidRelativeTimeFrames),
				},
				"ignore_infinity": {
					Type:          schema.TypeBool,
					Optional:      true,
					ConflictsWith: []string{"time_relative.0.condition.0.less_than"},
					Description:   "Not triggered when threshold is infinity (divided by zero).",
				},
				"group_by": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
					Description: "The fields to 'group by' on.",
				},
				"manage_undetected_values": {
					Type:     schema.TypeList,
					Optional: true,
					Computed: true,
					MaxItems: 1,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"enable_triggering_on_undetected_values": {
								Type:        schema.TypeBool,
								Required:    true,
								Description: "Determines whether the deadman-option is enabled. When set to true, auto_retire_ratio is required otherwise auto_retire_ratio should be omitted.",
							},
							"auto_retire_ratio": {
								Type:         schema.TypeString,
								Optional:     true,
								ValidateFunc: validation.StringInSlice(alertValidDeadmanRatioValues, false),
								Description:  fmt.Sprintf("Defines the triggering auto-retire ratio. Can be one of %q", alertValidDeadmanRatioValues),
							},
						},
					},
					RequiredWith: []string{"time_relative.0.condition.0.less_than", "time_relative.0.condition.0.group_by"},
					Description:  "Manage your logs undetected values - when relevant, enable/disable triggering on undetected values and change the auto retire interval. By default (when relevant), triggering is enabled with retire-ratio=NEVER.",
				},
			},
		},
		Description: "Defines the conditions for triggering and notify by the alert",
	}
	return timeRelativeSchema
}

func metricSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"lucene": {
			Type:     schema.TypeList,
			MaxItems: 1,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"search_query": {
						Type:        schema.TypeString,
						Required:    true,
						Description: "Regular expiration. More info: https://coralogix.com/blog/regex-101/",
					},
					"condition": {
						Type:     schema.TypeList,
						Required: true,
						MaxItems: 1,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"metric_field": {
									Type:        schema.TypeString,
									Required:    true,
									Description: "The name of the metric field to alert on.",
								},
								"arithmetic_operator": {
									Type:         schema.TypeString,
									Required:     true,
									ValidateFunc: validation.StringInSlice(alertValidArithmeticOperators, false),
									Description:  fmt.Sprintf("The arithmetic operator to use on the alert. can be one of %q", alertValidArithmeticOperators),
								},
								"arithmetic_operator_modifier": {
									Type:         schema.TypeInt,
									Optional:     true,
									ValidateFunc: validation.IntBetween(0, 100),
									Description:  "When arithmetic_operator = \"Percentile\" you need to supply the value in this property, 0 < value < 100.",
								},
								"less_than": {
									Type:     schema.TypeBool,
									Optional: true,
									ExactlyOneOf: []string{"metric.0.lucene.0.condition.0.less_than",
										"metric.0.lucene.0.condition.0.more_than"},
									Description: "Determines the condition operator." +
										" Must be one of - less_than or more_than.",
								},
								"more_than": {
									Type:     schema.TypeBool,
									Optional: true,
									ExactlyOneOf: []string{"metric.0.lucene.0.condition.0.less_than",
										"metric.0.lucene.0.condition.0.more_than"},
									Description: "Determines the condition operator." +
										" Must be one of - less_than or more_than.",
								},
								"threshold": {
									Type:        schema.TypeFloat,
									Required:    true,
									Description: "The number of log threshold that is needed to trigger the alert.",
								},
								"sample_threshold_percentage": {
									Type:         schema.TypeInt,
									Required:     true,
									ValidateFunc: validation.All(validation.IntDivisibleBy(10), validation.IntBetween(0, 100)),
									Description:  "The metric value must cross the threshold within this percentage of the timeframe (sum and count arithmetic operators do not use this parameter since they aggregate over the entire requested timeframe), increments of 10, 0 <= value <= 100.",
								},
								"time_window": {
									Type:         schema.TypeString,
									Required:     true,
									ValidateFunc: validation.StringInSlice(alertValidMetricTimeFrames, false),
									Description:  fmt.Sprintf("The bounded time frame for the threshold to be occurred within, to trigger the alert. Can be one of %q", alertValidMetricTimeFrames),
								},
								"group_by": {
									Type:     schema.TypeList,
									Optional: true,
									Elem: &schema.Schema{
										Type: schema.TypeString,
									},
									Description: "The fields to 'group by' on.",
								},
								"replace_missing_value_with_zero": {
									Type:          schema.TypeBool,
									Optional:      true,
									ConflictsWith: []string{"metric.0.lucene.0.condition.0.min_non_null_values_percentage"},
									Description:   "If set to true, missing data will be considered as 0, otherwise, it will not be considered at all.",
								},
								"min_non_null_values_percentage": {
									Type:          schema.TypeInt,
									Optional:      true,
									ValidateFunc:  validation.All(validation.IntDivisibleBy(10), validation.IntBetween(0, 100)),
									ConflictsWith: []string{"metric.0.lucene.0.condition.0.replace_missing_value_with_zero"},
									Description:   "The minimum percentage of the timeframe that should have values for this alert to trigger",
								},
								"manage_undetected_values": {
									Type:     schema.TypeList,
									Optional: true,
									Computed: true,
									MaxItems: 1,
									Elem: &schema.Resource{
										Schema: map[string]*schema.Schema{
											"enable_triggering_on_undetected_values": {
												Type:        schema.TypeBool,
												Required:    true,
												Description: "Determines whether the deadman-option is enabled. When set to true, auto_retire_ratio is required otherwise auto_retire_ratio should be omitted.",
											},
											"auto_retire_ratio": {
												Type:         schema.TypeString,
												Optional:     true,
												ValidateFunc: validation.StringInSlice(alertValidDeadmanRatioValues, false),
												Description:  fmt.Sprintf("Defines the triggering auto-retire ratio. Can be one of %q", alertValidDeadmanRatioValues),
											},
										},
									},
									RequiredWith: []string{"metric.0.lucene.0.condition.0.less_than", "metric.0.lucene.0.condition.0.group_by"},
									Description:  "Manage your logs undetected values - when relevant, enable/disable triggering on undetected values and change the auto retire interval. By default (when relevant), triggering is enabled with retire-ratio=NEVER.",
								},
							},
						},
						Description: "Defines the conditions for triggering and notify by the alert",
					},
				},
			},
			ExactlyOneOf: []string{"metric.0.lucene", "metric.0.promql"},
		},
		"promql": {
			Type:     schema.TypeList,
			MaxItems: 1,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"search_query": {
						Type:        schema.TypeString,
						Required:    true,
						Description: "Regular expiration. More info: https://coralogix.com/blog/regex-101/",
					},
					"condition": {
						Type:     schema.TypeList,
						Required: true,
						MaxItems: 1,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"less_than": {
									Type:     schema.TypeBool,
									Optional: true,
									ExactlyOneOf: []string{
										"metric.0.promql.0.condition.0.more_than",
										"metric.0.promql.0.condition.0.more_than_usual",
										"metric.0.promql.0.condition.0.less_than_usual",
										"metric.0.promql.0.condition.0.more_than_or_equal",
										"metric.0.promql.0.condition.0.less_than_or_equal",
									},
									Description: "Determines the condition operator." +
										" Must be one of - immediately, less_than, more_than, more_than_usual, less_than_usual, more_than_or_equal or less_than_or_equal.",
								},
								"more_than": {
									Type:     schema.TypeBool,
									Optional: true,
									Description: "Determines the condition operator." +
										" Must be one of - immediately, less_than, more_than, more_than_usual, less_than_usual, more_than_or_equal or less_than_or_equal.",
								},
								"more_than_usual": {
									Type:     schema.TypeBool,
									Optional: true,
									Description: "Determines the condition operator." +
										" Must be one of - immediately, less_than, more_than, more_than_usual, less_than_usual, more_than_or_equal or less_than_or_equal.",
								},
								"less_than_usual": {
									Type:     schema.TypeBool,
									Optional: true,
									Description: "Determines the condition operator." +
										" Must be one of - immediately, less_than, more_than, more_than_usual, less_than_usual, more_than_or_equal or less_than_or_equal.",
								},
								"more_than_or_equal": {
									Type:     schema.TypeBool,
									Optional: true,
									Description: "Determines the condition operator." +
										" Must be one of - immediately, less_than, more_than, more_than_usual, less_than_usual, more_than_or_equal or less_than_or_equal.",
								},
								"less_than_or_equal": {
									Type:     schema.TypeBool,
									Optional: true,
									Description: "Determines the condition operator." +
										" Must be one of - immediately, less_than, more_than, more_than_usual, less_than_usual, more_than_or_equal or less_than_or_equal.",
								},
								"threshold": {
									Type:        schema.TypeFloat,
									Required:    true,
									Description: "The threshold that is needed to trigger the alert.",
								},
								"time_window": {
									Type:         schema.TypeString,
									Required:     true,
									ValidateFunc: validation.StringInSlice(alertValidMetricTimeFrames, false),
									Description:  fmt.Sprintf("The bounded time frame for the threshold to be occurred within, to trigger the alert. Can be one of %q", alertValidMetricTimeFrames),
								},
								"sample_threshold_percentage": {
									Type:         schema.TypeInt,
									Required:     true,
									ValidateFunc: validation.All(validation.IntDivisibleBy(10), validation.IntBetween(0, 100)),
								},
								"replace_missing_value_with_zero": {
									Type:          schema.TypeBool,
									Optional:      true,
									ConflictsWith: []string{"metric.0.promql.0.condition.0.min_non_null_values_percentage", "metric.0.promql.0.condition.0.more_than_usual"},
									Description:   "If set to true, missing data will be considered as 0, otherwise, it will not be considered at all.",
								},
								"min_non_null_values_percentage": {
									Type:          schema.TypeInt,
									Optional:      true,
									ConflictsWith: []string{"metric.0.promql.0.condition.0.replace_missing_value_with_zero"},
									ValidateFunc:  validation.All(validation.IntDivisibleBy(10), validation.IntBetween(0, 100)),
								},
								"manage_undetected_values": {
									Type:     schema.TypeList,
									Optional: true,
									Computed: true,
									MaxItems: 1,
									Elem: &schema.Resource{
										Schema: map[string]*schema.Schema{
											"enable_triggering_on_undetected_values": {
												Type:        schema.TypeBool,
												Required:    true,
												Description: "Determines whether the deadman-option is enabled. When set to true, auto_retire_ratio is required otherwise auto_retire_ratio should be omitted.",
											},
											"auto_retire_ratio": {
												Type:         schema.TypeString,
												Optional:     true,
												ValidateFunc: validation.StringInSlice(alertValidDeadmanRatioValues, false),
												Description:  fmt.Sprintf("Defines the triggering auto-retire ratio. Can be one of %q", alertValidDeadmanRatioValues),
											},
										},
									},
									ConflictsWith: []string{"metric.0.promql.0.condition.0.more_than", "metric.0.promql.0.condition.0.more_than_or_equal", "metric.0.promql.0.condition.0.more_than_usual", "metric.0.promql.0.condition.0.less_than_usual"},
									Description:   "Manage your logs undetected values - when relevant, enable/disable triggering on undetected values and change the auto retire interval. By default (when relevant), triggering is enabled with retire-ratio=NEVER.",
								},
							},
						},
						Description: "Defines the conditions for triggering and notify by the alert",
					},
				},
			},
			ExactlyOneOf: []string{"metric.0.lucene", "metric.0.promql"},
		},
	}
}

func tracingSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"applications": {
			Type:     schema.TypeSet,
			Optional: true,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
			Description: "An array that contains log’s application names that we want to be alerted on." +
				" Applications can be filtered by prefix, suffix, and contains using the next patterns - filter:notEquals:xxx, filter:startsWith:xxx, filter:endsWith:xxx, filter:contains:xxx",
			Set: schema.HashString,
		},
		"subsystems": {
			Type:     schema.TypeSet,
			Optional: true,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
			Description: "An array that contains log’s subsystems names that we want to be alerted on." +
				" Applications can be filtered by prefix, suffix, and contains using the next patterns - filter:notEquals:xxx, filter:startsWith:xxx, filter:endsWith:xxx, filter:contains:xxx",
			Set: schema.HashString,
		},
		"services": {
			Type:     schema.TypeSet,
			Optional: true,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
			Description: "An array that contains log’s services names that we want to be alerted on." +
				" Applications can be filtered by prefix, suffix, and contains using the next patterns - filter:notEquals:xxx, filter:startsWith:xxx, filter:endsWith:xxx, filter:contains:xxx",
			Set: schema.HashString,
		},
		"tag_filter": {
			Type:     schema.TypeSet,
			Optional: true,
			Elem:     tagFilterSchema(),
			Set:      schema.HashResource(tagFilterSchema()),
		},
		"latency_threshold_milliseconds": {
			Type:         schema.TypeFloat,
			Optional:     true,
			ValidateFunc: validation.FloatAtLeast(0),
		},
		"condition": {
			Type:     schema.TypeList,
			Required: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"immediately": {
						Type:         schema.TypeBool,
						Optional:     true,
						ExactlyOneOf: []string{"tracing.0.condition.0.immediately", "tracing.0.condition.0.more_than"},
						Description: "Determines the condition operator." +
							" Must be one of - immediately or more_than.",
					},
					"more_than": {
						Type:         schema.TypeBool,
						Optional:     true,
						ExactlyOneOf: []string{"tracing.0.condition.0.immediately", "tracing.0.condition.0.more_than"},
						RequiredWith: []string{"tracing.0.condition.0.time_window"},
						Description: "Determines the condition operator." +
							" Must be one of - immediately or more_than.",
					},
					"threshold": {
						Type:          schema.TypeInt,
						Optional:      true,
						ConflictsWith: []string{"tracing.0.condition.0.immediately"},
						Description:   "The number of log occurrences that is needed to trigger the alert.",
					},
					"time_window": {
						Type:          schema.TypeString,
						Optional:      true,
						ForceNew:      true,
						ValidateFunc:  validation.StringInSlice(alertValidTimeFrames, false),
						ConflictsWith: []string{"tracing.0.condition.0.immediately"},
						RequiredWith:  []string{"tracing.0.condition.0.more_than"},
						Description:   fmt.Sprintf("The bounded time frame for the threshold to be occurred within, to trigger the alert. Can be one of %q", alertValidTimeFrames),
					},
					"group_by": {
						Type:     schema.TypeList,
						Optional: true,
						Elem: &schema.Schema{
							Type: schema.TypeString,
						},
						ConflictsWith: []string{"tracing.0.condition.0.immediately"},
						Description:   "The fields to 'group by' on.",
					},
				},
			},
			Description: "Defines the conditions for triggering and notify by the alert",
		},
	}
}

func tagFilterSchema() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"field": {
				Type:     schema.TypeString,
				Required: true,
			},
			"values": {
				Type:     schema.TypeSet,
				Required: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Set:         schema.HashString,
				Description: "Tag filter values can be filtered by prefix, suffix, and contains using the next patterns - filter:notEquals:xxx, filter:startsWith:xxx, filter:endsWith:xxx, filter:contains:xxx",
			},
		},
	}
}

func flowSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"stage": {
			Type:     schema.TypeList,
			Required: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"group": {
						Type:     schema.TypeList,
						Required: true,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"sub_alerts": {
									Type:     schema.TypeList,
									MaxItems: 1,
									Required: true,
									Elem: &schema.Resource{
										Schema: map[string]*schema.Schema{
											"operator": {
												Type:         schema.TypeString,
												Required:     true,
												ValidateFunc: validation.StringInSlice(alertValidFlowOperator, false),
												Description:  fmt.Sprintf("The operator to use on the alert. can be one of %q", alertValidFlowOperator),
											},
											"flow_alert": {
												Type:     schema.TypeList,
												Required: true,
												Elem: &schema.Resource{
													Schema: map[string]*schema.Schema{
														"not": {
															Type:     schema.TypeBool,
															Optional: true,
															Default:  false,
														},
														"user_alert_id": {
															Type:     schema.TypeString,
															Required: true,
														},
													},
												},
											},
										},
									},
								},
								"next_operator": {
									Type:         schema.TypeString,
									Required:     true,
									ValidateFunc: validation.StringInSlice(alertValidFlowOperator, false),
									Description:  fmt.Sprintf("The operator to use on the alert. can be one of %q", alertValidFlowOperator),
								},
							},
						},
					},
					"time_window": timeSchema("Timeframe for flow stage."),
				},
			},
		},
		"group_by": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
		},
	}
}

func resourceCoralogixAlertCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	createAlertRequest, diags := extractCreateAlertRequest(d)
	if len(diags) != 0 {
		return diags
	}

	createAlertStr := protojson.Format(createAlertRequest)
	log.Printf("[INFO] Creating new alert: %s", createAlertStr)
	AlertResp, err := meta.(*clientset.ClientSet).Alerts().CreateAlert(ctx, createAlertRequest)

	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		return diag.Errorf(formatRpcErrors(err, createAlertURL, createAlertStr))
	}

	alert := AlertResp.GetAlert()
	log.Printf("[INFO] Submitted new alert: %s", protojson.Format(alert))
	d.SetId(alert.GetUniqueIdentifier().GetValue())

	return resourceCoralogixAlertRead(ctx, d, meta)
}

func resourceCoralogixAlertRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := wrapperspb.String(d.Id())
	getAlertRequest := &alerts.GetAlertByUniqueIdRequest{
		Id: id,
	}

	log.Printf("[INFO] Reading alert %s", id)
	alertResp, err := meta.(*clientset.ClientSet).Alerts().GetAlert(ctx, getAlertRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.NotFound {
			d.SetId("")
			return diag.Diagnostics{diag.Diagnostic{
				Severity: diag.Warning,
				Summary:  fmt.Sprintf("Alert %q is in state, but no longer exists in Coralogix backend", id),
				Detail:   fmt.Sprintf("%s will be recreated when you apply", id),
			}}
		}
		return diag.Errorf(formatRpcErrors(err, getAlertURL, protojson.Format(getAlertRequest)))
	}
	alert := alertResp.GetAlert()
	alertStr := protojson.Format(alert)
	log.Printf("[INFO] Received alert: %s", alertStr)

	return setAlert(d, alert)
}

func resourceCoralogixAlertUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	req, diags := extractAlert(d)
	if len(diags) != 0 {
		return diags
	}

	updateAlertRequest := &alerts.UpdateAlertByUniqueIdRequest{
		Alert: req,
	}
	updateAlertStr := protojson.Format(updateAlertRequest)
	log.Printf("[INFO] Updating alert %s", updateAlertStr)
	alertResp, err := meta.(*clientset.ClientSet).Alerts().UpdateAlert(ctx, updateAlertRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		return diag.Errorf(formatRpcErrors(err, updateAlertURL, updateAlertStr))
	}
	updateAlertStr = protojson.Format(alertResp)
	log.Printf("[INFO] Submitted updated alert: %s", updateAlertStr)
	d.SetId(alertResp.GetAlert().GetUniqueIdentifier().GetValue())

	return resourceCoralogixAlertRead(ctx, d, meta)
}

func resourceCoralogixAlertDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := wrapperspb.String(d.Id())
	deleteAlertRequest := &alerts.DeleteAlertByUniqueIdRequest{
		Id: id,
	}

	log.Printf("[INFO] Deleting alert %s", id)
	_, err := meta.(*clientset.ClientSet).Alerts().DeleteAlert(ctx, deleteAlertRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		return diag.Errorf(formatRpcErrors(err, deleteAlertURL, protojson.Format(deleteAlertRequest)))
	}
	log.Printf("[INFO] alert %s deleted", id)

	d.SetId("")
	return nil
}

func extractCreateAlertRequest(d *schema.ResourceData) (*alerts.CreateAlertRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	enabled := wrapperspb.Bool(d.Get("enabled").(bool))
	name := wrapperspb.String(d.Get("name").(string))
	description := wrapperspb.String(d.Get("description").(string))
	severity := expandAlertSeverity(d.Get("severity").(string))
	metaLabels := extractMetaLabels(d.Get("meta_labels"))
	expirationDate := expandExpirationDate(d.Get("expiration_date"))
	incidentSettings := expandIncidentSettings(d.Get("incident_settings"))
	notificationGroups, dgs := expandNotificationGroups(d.Get("notifications_group"))
	diags = append(diags, dgs...)
	if len(diags) != 0 {
		return nil, diags
	}
	payloadFilters := expandPayloadFilters(d.Get("payload_filters"))
	scheduling := expandActiveWhen(d.Get("scheduling"))
	alertTypeParams, tracingAlert, dgs := expandAlertType(d)
	diags = append(diags, dgs...)
	if len(diags) != 0 {
		return nil, diags
	}

	return &alerts.CreateAlertRequest{
		Name:                       name,
		Description:                description,
		IsActive:                   enabled,
		Severity:                   severity,
		MetaLabels:                 metaLabels,
		Expiration:                 expirationDate,
		NotificationGroups:         notificationGroups,
		IncidentSettings:           incidentSettings,
		NotificationPayloadFilters: payloadFilters,
		ActiveWhen:                 scheduling,
		Filters:                    alertTypeParams.Filters,
		Condition:                  alertTypeParams.Condition,
		TracingAlert:               tracingAlert,
	}, diags
}

func extractAlert(d *schema.ResourceData) (*alerts.Alert, diag.Diagnostics) {
	var diags diag.Diagnostics
	id := wrapperspb.String(d.Id())
	enabled := wrapperspb.Bool(d.Get("enabled").(bool))
	name := wrapperspb.String(d.Get("name").(string))
	description := wrapperspb.String(d.Get("description").(string))
	severity := expandAlertSeverity(d.Get("severity").(string))
	metaLabels := extractMetaLabels(d.Get("meta_labels"))
	expirationDate := expandExpirationDate(d.Get("expiration_date"))
	incidentSettings := expandIncidentSettings(d.Get("incident_settings"))
	notificationGroups, dgs := expandNotificationGroups(d.Get("notifications_group"))
	diags = append(diags, dgs...)
	payloadFilters := expandPayloadFilters(d.Get("payload_filters"))
	scheduling := expandActiveWhen(d.Get("scheduling"))
	alertTypeParams, tracingAlert, dgs := expandAlertType(d)
	diags = append(diags, dgs...)
	if len(diags) != 0 {
		return nil, diags
	}

	return &alerts.Alert{
		UniqueIdentifier:           id,
		Name:                       name,
		Description:                description,
		IsActive:                   enabled,
		Severity:                   severity,
		MetaLabels:                 metaLabels,
		Expiration:                 expirationDate,
		IncidentSettings:           incidentSettings,
		NotificationGroups:         notificationGroups,
		NotificationPayloadFilters: payloadFilters,
		ActiveWhen:                 scheduling,
		Filters:                    alertTypeParams.Filters,
		Condition:                  alertTypeParams.Condition,
		TracingAlert:               tracingAlert,
	}, diags
}

func expandPayloadFilters(v interface{}) []*wrapperspb.StringValue {
	return interfaceSliceToWrappedStringSlice(v.(*schema.Set).List())
}

func setAlert(d *schema.ResourceData, alert *alerts.Alert) diag.Diagnostics {
	if err := d.Set("name", alert.GetName().GetValue()); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("description", alert.GetDescription().GetValue()); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("enabled", alert.GetIsActive().GetValue()); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("severity", flattenAlertSeverity(alert.GetSeverity().String())); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("meta_labels", flattenMetaLabels(alert.GetMetaLabels())); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("expiration_date", flattenExpirationDate(alert.GetExpiration())); err != nil {
		return diag.FromErr(err)
	}

	incidentSettings := flattenIncidentSettings(alert.GetIncidentSettings())
	if err := d.Set("incident_settings", incidentSettings); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("notifications_group", flattenNotificationGroups(alert.GetNotificationGroups(), incidentSettings != nil)); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("payload_filters", wrappedStringSliceToStringSlice(alert.GetNotificationPayloadFilters())); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("scheduling", flattenScheduling(d, alert.GetActiveWhen())); err != nil {
		return diag.FromErr(err)
	}

	alertType, alertTypeParams := flattenAlertType(alert)
	if err := d.Set(alertType, alertTypeParams); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func flattenIncidentSettings(settings *alerts.AlertIncidentSettings) interface{} {
	if settings == nil {
		return nil
	}
	if !settings.GetUseAsNotificationSettings().GetValue() {
		return nil
	}
	return []interface{}{
		map[string]interface{}{
			"retriggering_period_minutes": int(settings.GetRetriggeringPeriodSeconds().GetValue() / 60),
			"notify_on":                   alertProtoNotifyOnToSchemaNotifyOn[settings.GetNotifyOn()],
		},
	}
}

func flattenAlertSeverity(str string) string {
	return alertProtoSeverityToSchemaSeverity[str]
}

func flattenMetaLabels(labels []*alerts.MetaLabel) interface{} {
	result := make(map[string]interface{})
	for _, l := range labels {
		key := l.GetKey().GetValue()
		val := l.GetValue().GetValue()
		result[key] = val
	}
	return result
}

func flattenNotificationGroups(notificationGroups []*alerts.AlertNotificationGroups, incidentSettingsConfigured bool) interface{} {
	result := make([]interface{}, 0, len(notificationGroups))
	for _, group := range notificationGroups {
		notificationGroup := flattenNotificationGroup(group, incidentSettingsConfigured)
		result = append(result, notificationGroup)
	}
	return result
}

func flattenNotificationGroup(notificationGroup *alerts.AlertNotificationGroups, incidentSettingsConfigured bool) interface{} {
	groupByFields := wrappedStringSliceToStringSlice(notificationGroup.GetGroupByFields())
	notifications := flattenNotifications(notificationGroup.GetNotifications(), incidentSettingsConfigured)
	return map[string]interface{}{
		"group_by_fields": groupByFields,
		"notification":    notifications,
	}
}

func flattenNotifications(notifications []*alerts.AlertNotification, incidentSettingsConfigured bool) interface{} {
	result := make([]interface{}, 0, len(notifications))
	for _, n := range notifications {
		notificationSubgroup := flattenNotificationSubgroup(n, incidentSettingsConfigured)
		result = append(result, notificationSubgroup)
	}
	return result
}

func flattenNotificationSubgroup(notification *alerts.AlertNotification, incidentSettingsConfigured bool) interface{} {
	notificationSchema := map[string]interface{}{}
	if !incidentSettingsConfigured {
		notificationSchema["retriggering_period_minutes"] = int(notification.GetRetriggeringPeriodSeconds().GetValue() / 60)
		notificationSchema["notify_on"] = alertProtoNotifyOnToSchemaNotifyOn[notification.GetNotifyOn()]
	}
	switch integration := notification.GetIntegrationType().(type) {
	case *alerts.AlertNotification_IntegrationId:
		notificationSchema["integration_id"] = strconv.Itoa(int(integration.IntegrationId.GetValue()))
	case *alerts.AlertNotification_Recipients:
		notificationSchema["email_recipients"] = wrappedStringSliceToStringSlice(integration.Recipients.Emails)
	}

	return notificationSchema
}

func flattenScheduling(d *schema.ResourceData, activeWhen *alerts.AlertActiveWhen) interface{} {
	scheduling, ok := d.GetOk("scheduling")
	if !ok || activeWhen == nil {
		return nil
	}

	timeZone := scheduling.([]interface{})[0].(map[string]interface{})["time_zone"].(string)

	timeFrames := flattenTimeFrames(activeWhen, timeZone)

	return []interface{}{
		map[string]interface{}{
			"time_zone":  timeZone,
			"time_frame": timeFrames,
		},
	}
}

func flattenTimeFrames(activeWhen *alerts.AlertActiveWhen, timeZone string) interface{} {
	timeFrames := activeWhen.GetTimeframes()
	utc := flattenUtc(timeZone)
	result := schema.NewSet(hashTimeFrames(), []interface{}{})
	for _, tf := range timeFrames {
		m := flattenTimeFrame(tf, utc)
		result.Add(m)
	}
	return result
}

func flattenUtc(timeZone string) int32 {
	utcStr := strings.Split(timeZone, "UTC")[1]
	utc, _ := strconv.Atoi(utcStr)
	return int32(utc)
}

func flattenTimeFrame(tf *alerts.AlertActiveTimeframe, utc int32) map[string]interface{} {
	tr := tf.GetRange()
	activityStartGMT, activityEndGMT := tr.GetStart(), tr.GetEnd()
	daysOffset := getDaysOffsetFromGMT(activityStartGMT, utc)
	activityStartUTC := flattenTimeInDay(activityStartGMT, utc)
	activityEndUTC := flattenTimeInDay(activityEndGMT, utc)
	daysOfWeek := flattenDaysOfWeek(tf.GetDaysOfWeek(), daysOffset)

	return map[string]interface{}{
		"days_enabled": daysOfWeek,
		"start_time":   activityStartUTC,
		"end_time":     activityEndUTC,
	}
}

func getDaysOffsetFromGMT(activityStartGMT *alerts.Time, utc int32) int32 {
	daysOffset := int32(activityStartGMT.GetHours()+utc) / 24
	if daysOffset < 0 {
		daysOffset += 7
	}

	return daysOffset
}

func flattenTimeInDay(t *alerts.Time, utc int32) string {
	hours := convertGmtToUtc(t.GetHours(), utc)
	hoursStr := toTwoDigitsFormat(hours)
	minStr := toTwoDigitsFormat(t.GetMinutes())
	return fmt.Sprintf("%s:%s", hoursStr, minStr)
}

func flattenDaysOfWeek(daysOfWeek []alerts.DayOfWeek, daysOffset int32) interface{} {
	result := schema.NewSet(schema.HashString, []interface{}{})
	for _, d := range daysOfWeek {
		dayConvertedFromGmtToUtc := alerts.DayOfWeek((int32(d) + daysOffset) % 7)
		day := alertProtoDayOfWeekToSchemaDayOfWeek[dayConvertedFromGmtToUtc.String()]
		result.Add(day)
	}
	return result
}

func flattenAlertType(a *alerts.Alert) (alertType string, alertSchema interface{}) {
	filters := a.GetFilters()
	condition := a.GetCondition().GetCondition()

	switch filters.GetFilterType() {
	case alerts.AlertFilters_FILTER_TYPE_TEXT_OR_UNSPECIFIED:
		if _, ok := condition.(*alerts.AlertCondition_NewValue); ok {
			alertType = "new_value"
			alertSchema = flattenNewValueAlert(filters, condition)
		} else {
			alertType = "standard"
			alertSchema = flattenStandardAlert(filters, condition)
		}
	case alerts.AlertFilters_FILTER_TYPE_RATIO:
		alertType = "ratio"
		alertSchema = flattenRatioAlert(filters, condition)
	case alerts.AlertFilters_FILTER_TYPE_UNIQUE_COUNT:
		alertType = "unique_count"
		alertSchema = flattenUniqueCountAlert(filters, condition)
	case alerts.AlertFilters_FILTER_TYPE_TIME_RELATIVE:
		alertType = "time_relative"
		alertSchema = flattenTimeRelativeAlert(filters, condition)
	case alerts.AlertFilters_FILTER_TYPE_METRIC:
		alertType = "metric"
		alertSchema = flattenMetricAlert(filters, condition)
	case alerts.AlertFilters_FILTER_TYPE_TRACING:
		alertType = "tracing"
		alertSchema = flattenTracingAlert(condition, a.TracingAlert)
	case alerts.AlertFilters_FILTER_TYPE_FLOW:
		alertType = "flow"
		alertSchema = flattenFlowAlert(condition)
	}

	return
}

func flattenNewValueAlert(filters *alerts.AlertFilters, condition interface{}) interface{} {
	alertSchema := flattenCommonAlert(filters)
	conditionMap := flattenNewValueCondition(condition)
	alertSchema["condition"] = []interface{}{conditionMap}
	return []interface{}{alertSchema}
}

func flattenNewValueCondition(condition interface{}) interface{} {
	conditionParams := condition.(*alerts.AlertCondition_NewValue).NewValue.GetParameters()
	return map[string]interface{}{
		"time_window":  alertProtoNewValueTimeFrameToSchemaTimeFrame[conditionParams.GetTimeframe().String()],
		"key_to_track": conditionParams.GetGroupBy()[0].GetValue(),
	}
}

func flattenStandardAlert(filters *alerts.AlertFilters, condition interface{}) interface{} {
	alertSchemaMap := flattenCommonAlert(filters)
	conditionSchema := flattenStandardCondition(condition)
	alertSchemaMap["condition"] = conditionSchema
	return []interface{}{alertSchemaMap}
}

func flattenStandardCondition(condition interface{}) (conditionSchema interface{}) {
	var conditionParams *alerts.ConditionParameters
	switch condition := condition.(type) {
	case *alerts.AlertCondition_Immediate:
		conditionSchema = []interface{}{
			map[string]interface{}{
				"immediately": true,
			},
		}
	case *alerts.AlertCondition_LessThan:
		conditionParams = condition.LessThan.GetParameters()
		groupBy := wrappedStringSliceToStringSlice(conditionParams.GroupBy)
		m := map[string]interface{}{
			"less_than":   true,
			"threshold":   int(conditionParams.GetThreshold().GetValue()),
			"group_by":    groupBy,
			"time_window": alertProtoTimeFrameToSchemaTimeFrame[conditionParams.Timeframe.String()],
		}

		if len(groupBy) > 0 {
			m["manage_undetected_values"] = flattenManageUndetectedValues(conditionParams.GetRelatedExtendedData())
		}

		conditionSchema = []interface{}{m}
	case *alerts.AlertCondition_MoreThan:
		conditionParams = condition.MoreThan.GetParameters()
		conditionSchema = []interface{}{
			map[string]interface{}{
				"more_than":         true,
				"threshold":         int(conditionParams.GetThreshold().GetValue()),
				"group_by":          wrappedStringSliceToStringSlice(conditionParams.GroupBy),
				"time_window":       alertProtoTimeFrameToSchemaTimeFrame[conditionParams.Timeframe.String()],
				"evaluation_window": alertProtoToSchemaEvaluationWindow[condition.MoreThan.GetEvaluationWindow()],
			},
		}
	case *alerts.AlertCondition_MoreThanUsual:
		conditionParams = condition.MoreThanUsual.GetParameters()
		conditionMap := map[string]interface{}{
			"more_than_usual": true,
			"threshold":       int(conditionParams.GetThreshold().GetValue()),
			"time_window":     alertProtoTimeFrameToSchemaTimeFrame[conditionParams.GetTimeframe().String()],
			"group_by":        wrappedStringSliceToStringSlice(conditionParams.GroupBy),
		}
		conditionSchema = []interface{}{
			conditionMap,
		}
	}

	return
}

func flattenManageUndetectedValues(data *alerts.RelatedExtendedData) interface{} {
	if data == nil {
		return []map[string]interface{}{
			{
				"enable_triggering_on_undetected_values": true,
				"auto_retire_ratio":                      flattenDeadmanRatio(alerts.CleanupDeadmanDuration_CLEANUP_DEADMAN_DURATION_NEVER_OR_UNSPECIFIED),
			},
		}
	} else if data.GetShouldTriggerDeadman().GetValue() {
		return []map[string]interface{}{
			{
				"enable_triggering_on_undetected_values": true,
				"auto_retire_ratio":                      flattenDeadmanRatio(data.GetCleanupDeadmanDuration()),
			},
		}
	}

	return []map[string]interface{}{
		{
			"enable_triggering_on_undetected_values": false,
		},
	}
}

func flattenDeadmanRatio(cleanupDeadmanDuration alerts.CleanupDeadmanDuration) string {
	deadmanRatioStr := alerts.CleanupDeadmanDuration_name[int32(cleanupDeadmanDuration)]
	deadmanRatio := alertProtoDeadmanRatiosToSchemaDeadmanRatios[deadmanRatioStr]
	return deadmanRatio
}

func flattenRatioAlert(filters *alerts.AlertFilters, condition interface{}) interface{} {
	query1Map := flattenCommonAlert(filters)
	query1Map["alias"] = filters.GetAlias().GetValue()
	query2 := filters.GetRatioAlerts()[0]
	query2Map := flattenQuery2ParamsMap(query2)
	conditionMap := flattenRatioCondition(condition, query2)

	return []interface{}{
		map[string]interface{}{
			"query_1":   []interface{}{query1Map},
			"query_2":   []interface{}{query2Map},
			"condition": []interface{}{conditionMap},
		},
	}
}

func flattenRatioCondition(condition interface{}, query2 *alerts.AlertFilters_RatioAlert) interface{} {
	var conditionParams *alerts.ConditionParameters
	ratioParamsMap := make(map[string]interface{})

	lessThan := false
	switch condition := condition.(type) {
	case *alerts.AlertCondition_LessThan:
		conditionParams = condition.LessThan.GetParameters()
		ratioParamsMap["less_than"] = true
		lessThan = true
	case *alerts.AlertCondition_MoreThan:
		conditionParams = condition.MoreThan.GetParameters()
		ratioParamsMap["more_than"] = true
	default:
		return nil
	}

	ratioParamsMap["ratio_threshold"] = conditionParams.GetThreshold().GetValue()
	ratioParamsMap["time_window"] = alertProtoTimeFrameToSchemaTimeFrame[conditionParams.GetTimeframe().String()]
	ratioParamsMap["ignore_infinity"] = conditionParams.GetIgnoreInfinity().GetValue()

	groupByQ1 := conditionParams.GetGroupBy()
	groupByQ2 := query2.GetGroupBy()
	var groupBy []string
	if len(groupByQ1) > 0 {
		groupBy = wrappedStringSliceToStringSlice(groupByQ1)
		if len(groupByQ2) > 0 {
			ratioParamsMap["group_by_both"] = true
		} else {
			ratioParamsMap["group_by_q1"] = true
		}
	} else if len(groupByQ2) > 0 {
		groupBy = wrappedStringSliceToStringSlice(groupByQ2)
		ratioParamsMap["group_by_q1"] = true
	}
	ratioParamsMap["group_by"] = groupBy

	if len(groupBy) > 0 && lessThan {
		ratioParamsMap["manage_undetected_values"] = flattenManageUndetectedValues(conditionParams.GetRelatedExtendedData())
	}

	return ratioParamsMap
}

func flattenQuery2ParamsMap(query2 *alerts.AlertFilters_RatioAlert) interface{} {
	return map[string]interface{}{
		"alias":        query2.GetAlias().GetValue(),
		"search_query": query2.GetText().GetValue(),
		"severities":   extractSeverities(query2.GetSeverities()),
		"applications": wrappedStringSliceToStringSlice(query2.GetApplications()),
		"subsystems":   wrappedStringSliceToStringSlice(query2.GetSubsystems()),
	}
}

func flattenUniqueCountAlert(filters *alerts.AlertFilters, condition interface{}) interface{} {
	alertSchema := flattenCommonAlert(filters)
	conditionMap := flattenUniqueCountCondition(condition)
	alertSchema["condition"] = []interface{}{conditionMap}
	return []interface{}{alertSchema}
}

func flattenUniqueCountCondition(condition interface{}) interface{} {
	conditionParams := condition.(*alerts.AlertCondition_UniqueCount).UniqueCount.GetParameters()
	conditionMap := map[string]interface{}{
		"unique_count_key":  conditionParams.GetCardinalityFields()[0].GetValue(),
		"max_unique_values": conditionParams.GetThreshold().GetValue(),
		"time_window":       alertProtoUniqueCountTimeFrameToSchemaTimeFrame[conditionParams.GetTimeframe().String()],
	}

	if groupBy := conditionParams.GetGroupBy(); len(groupBy) > 0 {
		conditionMap["group_by_key"] = conditionParams.GetGroupBy()[0].GetValue()
		conditionMap["max_unique_values_for_group_by"] = conditionParams.GetMaxUniqueCountValuesForGroupByKey().GetValue()
	}

	return conditionMap
}

func flattenTimeRelativeAlert(filters *alerts.AlertFilters, condition interface{}) interface{} {
	alertSchema := flattenCommonAlert(filters)
	conditionMap := flattenTimeRelativeCondition(condition)
	alertSchema["condition"] = []interface{}{conditionMap}
	return []interface{}{alertSchema}
}

func flattenTimeRelativeCondition(condition interface{}) interface{} {
	var conditionParams *alerts.ConditionParameters
	timeRelativeCondition := make(map[string]interface{})
	switch condition := condition.(type) {
	case *alerts.AlertCondition_LessThan:
		conditionParams = condition.LessThan.GetParameters()
		timeRelativeCondition["less_than"] = true
		if len(conditionParams.GroupBy) > 0 {
			timeRelativeCondition["manage_undetected_values"] = flattenManageUndetectedValues(conditionParams.GetRelatedExtendedData())
		}
	case *alerts.AlertCondition_MoreThan:
		conditionParams = condition.MoreThan.GetParameters()
		timeRelativeCondition["more_than"] = true
	default:
		return nil
	}

	timeRelativeCondition["ignore_infinity"] = conditionParams.GetIgnoreInfinity().GetValue()
	timeRelativeCondition["ratio_threshold"] = conditionParams.GetThreshold().GetValue()
	timeRelativeCondition["group_by"] = wrappedStringSliceToStringSlice(conditionParams.GroupBy)
	timeFrame := conditionParams.GetTimeframe()
	relativeTimeFrame := conditionParams.GetRelativeTimeframe()
	timeRelativeCondition["relative_time_window"] = flattenRelativeTimeWindow(timeFrame, relativeTimeFrame)

	return timeRelativeCondition
}

func flattenRelativeTimeWindow(timeFrame alerts.Timeframe, relativeTimeFrame alerts.RelativeTimeframe) string {
	p := protoTimeFrameAndRelativeTimeFrame{timeFrame: timeFrame, relativeTimeFrame: relativeTimeFrame}
	return alertProtoTimeFrameAndRelativeTimeFrameToSchemaRelativeTimeFrame[p]
}

func flattenMetricAlert(filters *alerts.AlertFilters, condition interface{}) interface{} {
	var conditionParams *alerts.ConditionParameters
	var conditionStr string
	switch condition := condition.(type) {
	case *alerts.AlertCondition_LessThan:
		conditionParams = condition.LessThan.GetParameters()
		conditionStr = "less_than"
	case *alerts.AlertCondition_MoreThan:
		conditionParams = condition.MoreThan.GetParameters()
		conditionStr = "more_than"
	case *alerts.AlertCondition_MoreThanUsual:
		conditionParams = condition.MoreThanUsual.GetParameters()
		conditionStr = "more_than_usual"
	case *alerts.AlertCondition_LessThanUsual:
		conditionParams = condition.LessThanUsual.GetParameters()
		conditionStr = "less_than_usual"
	case *alerts.AlertCondition_MoreThanOrEqual:
		conditionParams = condition.MoreThanOrEqual.GetParameters()
		conditionStr = "more_than_or_equal"
	case *alerts.AlertCondition_LessThanOrEqual:
		conditionParams = condition.LessThanOrEqual.GetParameters()
		conditionStr = "less_than_or_equal"
	default:
		return nil
	}

	var metricTypeStr string
	var searchQuery string
	var conditionMap map[string]interface{}
	promqlParams := conditionParams.GetMetricAlertPromqlParameters()
	if promqlParams != nil {
		metricTypeStr = "promql"
		searchQuery = promqlParams.GetPromqlText().GetValue()
		conditionMap = flattenPromQLCondition(conditionParams)
	} else {
		metricTypeStr = "lucene"
		searchQuery = filters.GetText().GetValue()
		conditionMap = flattenLuceneCondition(conditionParams)
	}
	conditionMap[conditionStr] = true
	if conditionStr == "less_than" || conditionStr == "less_than_or_equal" {
		conditionMap["manage_undetected_values"] = flattenManageUndetectedValues(conditionParams.GetRelatedExtendedData())
	}

	metricMap := map[string]interface{}{
		"search_query": searchQuery,
		"condition":    []interface{}{conditionMap},
	}

	return []interface{}{
		map[string]interface{}{
			metricTypeStr: []interface{}{metricMap},
		},
	}
}

func flattenPromQLCondition(params *alerts.ConditionParameters) (promQLConditionMap map[string]interface{}) {
	promqlParams := params.GetMetricAlertPromqlParameters()
	promQLConditionMap =
		map[string]interface{}{
			"threshold":                       params.GetThreshold().GetValue(),
			"time_window":                     alertProtoMetricTimeFrameToMetricSchemaTimeFrame[params.GetTimeframe().String()],
			"sample_threshold_percentage":     promqlParams.GetSampleThresholdPercentage().GetValue(),
			"replace_missing_value_with_zero": promqlParams.GetSwapNullValues().GetValue(),
			"min_non_null_values_percentage":  promqlParams.GetNonNullPercentage().GetValue(),
		}
	return
}

func flattenLuceneCondition(params *alerts.ConditionParameters) map[string]interface{} {
	metricParams := params.GetMetricAlertParameters()
	return map[string]interface{}{
		"metric_field":                    metricParams.GetMetricField().GetValue(),
		"arithmetic_operator":             alertProtoArithmeticOperatorToSchemaArithmetic[metricParams.GetArithmeticOperator().String()],
		"threshold":                       params.GetThreshold().GetValue(),
		"arithmetic_operator_modifier":    metricParams.GetArithmeticOperatorModifier().GetValue(),
		"sample_threshold_percentage":     metricParams.GetSampleThresholdPercentage().GetValue(),
		"time_window":                     alertProtoMetricTimeFrameToMetricSchemaTimeFrame[params.GetTimeframe().String()],
		"group_by":                        wrappedStringSliceToStringSlice(params.GetGroupBy()),
		"replace_missing_value_with_zero": metricParams.GetSwapNullValues().GetValue(),
		"min_non_null_values_percentage":  metricParams.GetNonNullPercentage().GetValue(),
	}
}

func flattenTracingAlert(condition interface{}, tracingAlert *alerts.TracingAlert) interface{} {
	latencyThresholdMS := float64(tracingAlert.GetConditionLatency()) / float64(time.Millisecond.Microseconds())
	applications, subsystems, services := flattenTracingFilters(tracingAlert.GetFieldFilters())
	tagFilters := flattenTagFiltersData(tracingAlert.GetTagFilters())
	conditionSchema := flattenTracingCondition(condition)

	return []interface{}{
		map[string]interface{}{
			"latency_threshold_milliseconds": latencyThresholdMS,
			"applications":                   applications,
			"subsystems":                     subsystems,
			"services":                       services,
			"tag_filter":                     tagFilters,
			"condition":                      conditionSchema,
		},
	}
}

func flattenTracingFilters(tracingFilters []*alerts.FilterData) (applications, subsystems, services interface{}) {
	filtersData := flattenFiltersData(tracingFilters)
	applications = filtersData["applicationName"]
	subsystems = filtersData["subsystemName"]
	services = filtersData["serviceName"]
	return
}

func flattenFlowAlert(condition interface{}) interface{} {
	return []interface{}{flattenFlowAlertsCondition(condition.(*alerts.AlertCondition_Flow))}
}

func flattenFlowAlertsCondition(condition *alerts.AlertCondition_Flow) interface{} {
	stages := flattenStages(condition.Flow.GetStages())

	m := map[string]interface{}{
		"stage": stages,
	}

	if flowParams := condition.Flow.GetParameters(); flowParams != nil {
		groupBy := wrappedStringSliceToStringSlice(flowParams.GetGroupBy())
		if len(groupBy) != 0 {
			m["group_by"] = groupBy
		}
	}

	return m
}

func flattenStages(stages []*alerts.FlowStage) []interface{} {
	result := make([]interface{}, 0, len(stages))
	for _, stage := range stages {
		result = append(result, flattenStage(stage))
	}
	return result
}

func flattenStage(stage *alerts.FlowStage) interface{} {
	timeMS := int(stage.GetTimeframe().GetMs().GetValue())
	return map[string]interface{}{
		"group":       flattenGroups(stage.GetGroups()),
		"time_window": flattenTimeframe(timeMS),
	}
}

func flattenGroups(groups []*alerts.FlowGroup) []interface{} {
	result := make([]interface{}, 0, len(groups))
	for _, g := range groups {
		result = append(result, flattenGroup(g))
	}
	return result
}

func flattenGroup(fg *alerts.FlowGroup) interface{} {
	subAlerts := flattenSubAlerts(fg.GetAlerts())
	operator := fg.GetNextOp().String()
	return map[string]interface{}{
		"sub_alerts":    subAlerts,
		"next_operator": operator,
	}
}

func flattenSubAlerts(subAlerts *alerts.FlowAlerts) interface{} {
	operator := subAlerts.GetOp().String()
	flowAlerts := make([]interface{}, 0, len(subAlerts.GetValues()))
	for _, sa := range subAlerts.GetValues() {
		flowAlerts = append(flowAlerts, flattenInnerFlowAlert(sa))
	}

	return []interface{}{
		map[string]interface{}{
			"operator":   operator,
			"flow_alert": flowAlerts,
		},
	}
}

func flattenInnerFlowAlert(subAlert *alerts.FlowAlert) interface{} {
	return map[string]interface{}{
		"not":           subAlert.GetNot().GetValue(),
		"user_alert_id": subAlert.GetId().GetValue(),
	}
}

func flattenFiltersData(filtersData []*alerts.FilterData) map[string]interface{} {
	result := make(map[string]interface{}, len(filtersData))
	for _, filter := range filtersData {
		field := filter.GetField()
		result[field] = flattenFilters(filter.GetFilters())
	}
	return result
}

func flattenTagFiltersData(filtersData []*alerts.FilterData) interface{} {
	fieldToFilters := flattenFiltersData(filtersData)
	result := make([]interface{}, 0, len(fieldToFilters))
	for field, filters := range fieldToFilters {
		filterSchema := map[string]interface{}{
			"field":  field,
			"values": filters,
		}
		result = append(result, filterSchema)
	}
	return result
}

func flattenFilters(filters []*alerts.Filters) []string {
	result := make([]string, 0)
	for _, f := range filters {
		values := f.GetValues()
		switch operator := f.GetOperator(); operator {
		case "notEquals", "contains", "startsWith", "endsWith":
			for i, val := range values {
				values[i] = fmt.Sprintf("filter:%s:%s", operator, val)
			}
		}
		result = append(result, values...)
	}
	return result
}

func flattenTracingCondition(condition interface{}) interface{} {
	switch condition := condition.(type) {
	case *alerts.AlertCondition_Immediate:
		return []interface{}{
			map[string]interface{}{
				"immediately": true,
			},
		}
	case *alerts.AlertCondition_MoreThan:
		conditionParams := condition.MoreThan.GetParameters()
		return []interface{}{
			map[string]interface{}{
				"more_than":   true,
				"threshold":   conditionParams.GetThreshold().GetValue(),
				"time_window": alertProtoTimeFrameToSchemaTimeFrame[conditionParams.GetTimeframe().String()],
				"group_by":    wrappedStringSliceToStringSlice(conditionParams.GetGroupBy()),
			},
		}
	default:
		return nil
	}
}

func flattenCommonAlert(filters *alerts.AlertFilters) map[string]interface{} {
	metadata := filters.GetMetadata()
	return map[string]interface{}{
		"search_query": filters.GetText().GetValue(),
		"severities":   extractSeverities(filters.GetSeverities()),
		"applications": wrappedStringSliceToStringSlice(metadata.GetApplications()),
		"subsystems":   wrappedStringSliceToStringSlice(metadata.GetSubsystems()),
		"categories":   wrappedStringSliceToStringSlice(metadata.GetCategories()),
		"computers":    wrappedStringSliceToStringSlice(metadata.GetComputers()),
		"classes":      wrappedStringSliceToStringSlice(metadata.GetClasses()),
		"methods":      wrappedStringSliceToStringSlice(metadata.GetMethods()),
		"ip_addresses": wrappedStringSliceToStringSlice(metadata.GetIpAddresses()),
	}
}

func extractSeverities(severities []alerts.AlertFilters_LogSeverity) []string {
	result := make([]string, 0, len(severities))
	for _, s := range severities {
		result = append(result, alertProtoLogSeverityToSchemaLogSeverity[s.String()])
	}
	return result
}

func flattenExpirationDate(expiration *alerts.Date) []map[string]int {
	if expiration == nil {
		return nil
	}
	m := map[string]int{
		"year":  int(expiration.GetYear()),
		"month": int(expiration.GetMonth()),
		"day":   int(expiration.GetDay()),
	}

	return []map[string]int{m}
}

func expandAlertSeverity(severity string) alerts.AlertSeverity {
	severityStr := alertSchemaSeverityToProtoSeverity[severity]
	formatStandardVal := alerts.AlertSeverity_value[severityStr]
	return alerts.AlertSeverity(formatStandardVal)
}

func expandExpirationDate(v interface{}) *alerts.Date {
	l := v.([]interface{})
	if len(l) == 0 || l[0] == nil {
		return nil
	}
	raw := l[0]
	m := raw.(map[string]interface{})
	return &alerts.Date{
		Year:  int32(m["year"].(int)),
		Month: int32(m["month"].(int)),
		Day:   int32(m["day"].(int)),
	}
}

func expandIncidentSettings(v interface{}) *alerts.AlertIncidentSettings {
	l, ok := v.([]interface{})
	if !ok || len(l) == 0 || l[0] == nil {
		return nil
	}
	raw := l[0]
	m := raw.(map[string]interface{})

	retriggeringPeriodSeconds := wrapperspb.UInt32(uint32(m["retriggering_period_minutes"].(int)) * 60)
	notifyOn := alertSchemaNotifyOnToProtoNotifyOn[m["notify_on"].(string)]

	return &alerts.AlertIncidentSettings{
		RetriggeringPeriodSeconds: retriggeringPeriodSeconds,
		NotifyOn:                  notifyOn,
		UseAsNotificationSettings: wrapperspb.Bool(true),
	}

}

func expandNotificationGroups(v interface{}) ([]*alerts.AlertNotificationGroups, diag.Diagnostics) {
	v = v.(*schema.Set).List()
	l := v.([]interface{})
	result := make([]*alerts.AlertNotificationGroups, 0, len(l))
	var diags diag.Diagnostics
	for _, s := range l {
		ml, dgs := expandNotificationGroup(s)
		diags = append(diags, dgs...)
		result = append(result, ml)
	}
	return result, diags
}

func expandNotificationGroup(v interface{}) (*alerts.AlertNotificationGroups, diag.Diagnostics) {
	if v == nil {
		return nil, nil
	}
	m := v.(map[string]interface{})

	groupByFields := interfaceSliceToWrappedStringSlice(m["group_by_fields"].([]interface{}))
	notifications, diags := expandNotificationSubgroups(m["notification"])
	if len(diags) != 0 {
		return nil, diags
	}

	return &alerts.AlertNotificationGroups{
		GroupByFields: groupByFields,
		Notifications: notifications,
	}, nil
}

func expandNotificationSubgroups(v interface{}) ([]*alerts.AlertNotification, diag.Diagnostics) {
	v = v.(*schema.Set).List()
	notifications := v.([]interface{})
	result := make([]*alerts.AlertNotification, 0, len(notifications))
	var diags diag.Diagnostics
	for _, n := range notifications {
		notification, err := expandNotificationSubgroup(n)
		if err != nil {
			diags = append(diags, diag.FromErr(err)...)
		}
		result = append(result, notification)
	}
	return result, diags
}

func expandNotificationSubgroup(v interface{}) (*alerts.AlertNotification, error) {
	if v == nil {
		return nil, nil
	}
	m := v.(map[string]interface{})

	var notifyEverySec *wrapperspb.UInt32Value
	if minutes, ok := m["retriggering_period_minutes"].(int); ok && minutes != 0 {
		notifyEverySec = wrapperspb.UInt32(uint32(minutes) * 60)
	}

	var notifyOn *alerts.NotifyOn
	if notifyOnStr, ok := m["notify_on"].(string); ok {
		notifyOn = new(alerts.NotifyOn)
		*notifyOn = alertSchemaNotifyOnToProtoNotifyOn[notifyOnStr]
	}

	notification := &alerts.AlertNotification{
		RetriggeringPeriodSeconds: notifyEverySec,
		NotifyOn:                  notifyOn,
	}

	var isWebhookIdDefined bool
	if webhookID, ok := m["integration_id"].(string); ok && webhookID != "" {
		isWebhookIdDefined = true
		id := parseNumUint32(webhookID)
		notification.IntegrationType = &alerts.AlertNotification_IntegrationId{
			IntegrationId: wrapperspb.UInt32(id),
		}
	}

	if emails := m["email_recipients"].(*schema.Set).List(); len(emails) != 0 {
		if isWebhookIdDefined {
			return nil, fmt.Errorf("required exactly on of 'integration_id' or 'email_recipients'")
		}

		notification.IntegrationType = &alerts.AlertNotification_Recipients{
			Recipients: &alerts.Recipients{
				Emails: interfaceSliceToWrappedStringSlice(emails),
			},
		}
	}

	return notification, nil
}

func extractMetaLabels(v interface{}) []*alerts.MetaLabel {
	m := v.(map[string]interface{})
	result := make([]*alerts.MetaLabel, 0, len(m))
	for key, val := range m {
		ml := &alerts.MetaLabel{
			Key:   wrapperspb.String(key),
			Value: wrapperspb.String(val.(string)),
		}
		result = append(result, ml)
	}
	return result
}

func expandActiveWhen(v interface{}) *alerts.AlertActiveWhen {
	l := v.([]interface{})
	if len(l) == 0 {
		return nil
	}

	schedulingMap := l[0].(map[string]interface{})
	utc := flattenUtc(schedulingMap["time_zone"].(string))
	timeFrames := schedulingMap["time_frame"].(*schema.Set).List()

	expandedTimeframes := expandActiveTimeframes(timeFrames, utc)

	return &alerts.AlertActiveWhen{
		Timeframes: expandedTimeframes,
	}
}

func expandActiveTimeframes(timeFrames []interface{}, utc int32) []*alerts.AlertActiveTimeframe {
	result := make([]*alerts.AlertActiveTimeframe, 0, len(timeFrames))
	for _, tf := range timeFrames {
		alertActiveTimeframe := expandActiveTimeFrame(tf, utc)
		result = append(result, alertActiveTimeframe)
	}
	return result
}

func expandActiveTimeFrame(timeFrame interface{}, utc int32) *alerts.AlertActiveTimeframe {
	m := timeFrame.(map[string]interface{})
	daysOfWeek := expandDaysOfWeek(m["days_enabled"])
	frameRange := expandRange(m["start_time"], m["end_time"])
	frameRange, daysOfWeek = convertTimeFramesToGMT(frameRange, daysOfWeek, utc)

	alertActiveTimeframe := &alerts.AlertActiveTimeframe{
		DaysOfWeek: daysOfWeek,
		Range:      frameRange,
	}
	return alertActiveTimeframe
}

func convertTimeFramesToGMT(frameRange *alerts.TimeRange, daysOfWeek []alerts.DayOfWeek, utc int32) (*alerts.TimeRange, []alerts.DayOfWeek) {
	daysOfWeekOffset := daysOfWeekOffsetToGMT(frameRange, utc)
	frameRange.Start.Hours = convertUtcToGmt(frameRange.GetStart().GetHours(), utc)
	frameRange.End.Hours = convertUtcToGmt(frameRange.GetEnd().GetHours(), utc)
	if daysOfWeekOffset != 0 {
		for i, d := range daysOfWeek {
			daysOfWeek[i] = alerts.DayOfWeek((int32(d) + daysOfWeekOffset) % 7)
		}
	}

	return frameRange, daysOfWeek
}

func daysOfWeekOffsetToGMT(frameRange *alerts.TimeRange, utc int32) int32 {
	daysOfWeekOffset := int32(frameRange.Start.Hours-utc) / 24
	if daysOfWeekOffset < 0 {
		daysOfWeekOffset += 7
	}
	return daysOfWeekOffset
}

func convertUtcToGmt(hours, utc int32) int32 {
	hours -= utc
	if hours < 0 {
		hours += 24
	} else if hours >= 24 {
		hours -= 24
	}

	return hours
}

func convertGmtToUtc(hours, utc int32) int32 {
	hours += utc
	if hours < 0 {
		hours += 24
	} else if hours >= 24 {
		hours -= 24
	}

	return hours
}

func expandDaysOfWeek(v interface{}) []alerts.DayOfWeek {
	l := v.(*schema.Set).List()
	result := make([]alerts.DayOfWeek, 0, len(l))
	for _, v := range l {
		dayOfWeekStr := alertSchemaDayOfWeekToProtoDayOfWeek[v.(string)]
		dayOfWeekVal := alerts.DayOfWeek_value[dayOfWeekStr]
		result = append(result, alerts.DayOfWeek(dayOfWeekVal))
	}
	return result
}

func expandRange(activityStarts, activityEnds interface{}) *alerts.TimeRange {
	start := expandTimeInDay(activityStarts)
	end := expandTimeInDay(activityEnds)

	return &alerts.TimeRange{
		Start: start,
		End:   end,
	}
}

func expandAlertType(d *schema.ResourceData) (alertTypeParams *alertParams, tracingAlert *alerts.TracingAlert, diags diag.Diagnostics) {
	alertTypeStr := From(validAlertTypes).FirstWith(func(key interface{}) bool {
		return len(d.Get(key.(string)).([]interface{})) > 0
	}).(string)

	alertType := d.Get(alertTypeStr).([]interface{})[0].(map[string]interface{})

	switch alertTypeStr {
	case "standard":
		alertTypeParams, diags = expandStandard(alertType)
	case "ratio":
		alertTypeParams, diags = expandRatio(alertType)
	case "new_value":
		alertTypeParams = expandNewValue(alertType)
	case "unique_count":
		alertTypeParams = expandUniqueCount(alertType)
	case "time_relative":
		alertTypeParams, diags = expandTimeRelative(alertType)
	case "metric":
		alertTypeParams, diags = expandMetric(alertType)
	case "tracing":
		alertTypeParams, tracingAlert = expandTracing(alertType)
	case "flow":
		alertTypeParams = expandFlow(alertType)
	}

	return
}

func expandStandard(m map[string]interface{}) (*alertParams, diag.Diagnostics) {
	conditionMap := extractConditionMap(m)
	condition, err := expandStandardCondition(conditionMap)
	if err != nil {
		return nil, diag.FromErr(err)
	}
	filters := expandStandardFilter(m)
	return &alertParams{
		Condition: condition,
		Filters:   filters,
	}, nil
}

func expandStandardCondition(m map[string]interface{}) (*alerts.AlertCondition, error) {
	if immediately := m["immediately"]; immediately != nil && immediately.(bool) {
		return &alerts.AlertCondition{
			Condition: &alerts.AlertCondition_Immediate{},
		}, nil
	} else if moreThenUsual := m["more_than_usual"]; moreThenUsual != nil && moreThenUsual.(bool) {
		threshold := wrapperspb.Double(float64(m["threshold"].(int)))
		groupBy := interfaceSliceToWrappedStringSlice(m["group_by"].([]interface{}))
		parameters := &alerts.ConditionParameters{
			Threshold: threshold,
			GroupBy:   groupBy,
			Timeframe: expandTimeFrame(m["time_window"].(string)),
		}
		return &alerts.AlertCondition{
			Condition: &alerts.AlertCondition_MoreThanUsual{
				MoreThanUsual: &alerts.MoreThanUsualCondition{Parameters: parameters},
			},
		}, nil
	} else {
		parameters, err := expandStandardConditionParameters(m)
		if err != nil {
			return nil, err
		}
		if lessThan := m["less_than"]; lessThan != nil && lessThan.(bool) {
			return &alerts.AlertCondition{
				Condition: &alerts.AlertCondition_LessThan{
					LessThan: &alerts.LessThanCondition{Parameters: parameters},
				},
			}, nil
		} else if moreThan := m["more_than"]; moreThan != nil && moreThan.(bool) {
			evaluationWindow := expandEvaluationWindow(m)
			return &alerts.AlertCondition{
				Condition: &alerts.AlertCondition_MoreThan{
					MoreThan: &alerts.MoreThanCondition{
						Parameters:       parameters,
						EvaluationWindow: evaluationWindow,
					},
				},
			}, nil
		}
	}

	return nil, fmt.Errorf("immediately, less_than, more_than or more_than_usual have to be true")
}

func expandEvaluationWindow(m map[string]interface{}) *alerts.EvaluationWindow {
	var evaluationWindow *alerts.EvaluationWindow
	if evaluationWindowStr, ok := m["evaluation_window"].(string); ok && evaluationWindowStr != "" {
		evaluationWindow = new(alerts.EvaluationWindow)
		*evaluationWindow = alertSchemaToProtoEvaluationWindow[evaluationWindowStr]
	}
	return evaluationWindow
}

func expandRelatedExtendedData(m map[string]interface{}) (*alerts.RelatedExtendedData, error) {
	if v, ok := m["less_than"]; !(ok && v.(bool)) {
		return nil, nil
	}

	if v, ok := m["manage_undetected_values"]; ok {
		if manageUndetectedValues, ok := v.([]interface{}); ok && len(manageUndetectedValues) != 0 {
			raw := manageUndetectedValues[0].(map[string]interface{})
			if enable, autoRetireRatio := raw["enable_triggering_on_undetected_values"], raw["auto_retire_ratio"]; enable.(bool) {
				if autoRetireRatio == nil || autoRetireRatio.(string) == "" {
					return nil, fmt.Errorf("auto_retire_ratio is required when enable_triggering_on_undetected_values = true")
				}
				cleanupDeadmanDurationStr := alertSchemaDeadmanRatiosToProtoDeadmanRatios[autoRetireRatio.(string)]
				cleanupDeadmanDuration := alerts.CleanupDeadmanDuration(alerts.CleanupDeadmanDuration_value[cleanupDeadmanDurationStr])
				return &alerts.RelatedExtendedData{
					CleanupDeadmanDuration: &cleanupDeadmanDuration,
					ShouldTriggerDeadman:   wrapperspb.Bool(true),
				}, nil
			} else {
				if autoRetireRatio != nil && autoRetireRatio.(string) != "" {
					return nil, fmt.Errorf("auto_retire_ratio is not allowed when enable_triggering_on_undetected_values = false")
				}
				return &alerts.RelatedExtendedData{
					ShouldTriggerDeadman: wrapperspb.Bool(false),
				}, nil
			}
		}
	}

	return nil, nil
}

func expandStandardConditionParameters(m map[string]interface{}) (*alerts.ConditionParameters, error) {
	timeFrame := expandTimeFrame(m["time_window"].(string))
	groupBy := interfaceSliceToWrappedStringSlice(m["group_by"].([]interface{}))
	threshold := wrapperspb.Double(float64(m["threshold"].(int)))
	relatedExtendedData, err := expandRelatedExtendedData(m)
	if err != nil {
		return nil, err
	}

	return &alerts.ConditionParameters{
		Threshold:           threshold,
		Timeframe:           timeFrame,
		GroupBy:             groupBy,
		RelatedExtendedData: relatedExtendedData,
	}, nil
}

func expandTracingConditionParameters(m map[string]interface{}) *alerts.ConditionParameters {
	timeFrame := expandTimeFrame(m["time_window"].(string))
	groupBy := interfaceSliceToWrappedStringSlice(m["group_by"].([]interface{}))
	threshold := wrapperspb.Double(float64(m["threshold"].(int)))

	return &alerts.ConditionParameters{
		Threshold: threshold,
		Timeframe: timeFrame,
		GroupBy:   groupBy,
	}
}

func expandStandardFilter(m map[string]interface{}) *alerts.AlertFilters {
	filters := expandCommonAlertFilter(m)
	filters.FilterType = alerts.AlertFilters_FILTER_TYPE_TEXT_OR_UNSPECIFIED
	return filters
}

func expandRatio(m map[string]interface{}) (*alertParams, diag.Diagnostics) {
	conditionMap := extractConditionMap(m)
	groupBy := interfaceSliceToWrappedStringSlice(conditionMap["group_by"].([]interface{}))
	var groupByQ1, groupByQ2 []*wrapperspb.StringValue
	if len(groupBy) > 0 {
		if conditionMap["group_by_q1"].(bool) {
			groupByQ1 = groupBy
		} else if conditionMap["group_by_q2"].(bool) {
			groupByQ2 = groupBy
		} else if conditionMap["group_by_both"].(bool) {
			groupByQ1 = groupBy
			groupByQ2 = groupBy
		} else {
			return nil, diag.Errorf("group_by is required with one of - group_by_q1/group_by_q1/group_by_both")
		}
	}

	condition, err := expandRatioCondition(conditionMap, groupByQ1)
	if err != nil {
		return nil, diag.FromErr(err)
	}
	filters := expandRatioFilters(m, groupByQ2)

	return &alertParams{
		Condition: condition,
		Filters:   filters,
	}, nil
}

func expandRatioFilters(m map[string]interface{}, groupBy []*wrapperspb.StringValue) *alerts.AlertFilters {
	query1 := m["query_1"].([]interface{})[0].(map[string]interface{})
	filters := expandCommonAlertFilter(query1)
	filters.FilterType = alerts.AlertFilters_FILTER_TYPE_RATIO
	filters.Alias = wrapperspb.String(query1["alias"].(string))
	query2 := expandQuery2(m["query_2"], groupBy)
	filters.RatioAlerts = []*alerts.AlertFilters_RatioAlert{query2}
	return filters
}

func expandRatioCondition(m map[string]interface{}, groupBy []*wrapperspb.StringValue) (*alerts.AlertCondition, error) {
	parameters, err := expandRatioParams(m, groupBy)
	if err != nil {
		return nil, err
	}

	return expandLessThanOrMoreThanAlertCondition(m, parameters)
}

func expandRatioParams(m map[string]interface{}, groupBy []*wrapperspb.StringValue) (*alerts.ConditionParameters, error) {
	threshold := wrapperspb.Double(m["ratio_threshold"].(float64))
	timeFrame := expandTimeFrame(m["time_window"].(string))
	ignoreInfinity := wrapperspb.Bool(m["ignore_infinity"].(bool))
	relatedExtendedData, err := expandRelatedExtendedData(m)
	if err != nil {
		return nil, err
	}

	return &alerts.ConditionParameters{
		Threshold:           threshold,
		Timeframe:           timeFrame,
		GroupBy:             groupBy,
		IgnoreInfinity:      ignoreInfinity,
		RelatedExtendedData: relatedExtendedData,
	}, nil
}

func expandQuery2(v interface{}, groupBy []*wrapperspb.StringValue) *alerts.AlertFilters_RatioAlert {
	m := v.([]interface{})[0].(map[string]interface{})
	alias := wrapperspb.String(m["alias"].(string))
	text := wrapperspb.String(m["search_query"].(string))
	severities := expandAlertFiltersSeverities(m["severities"].(*schema.Set).List())
	applications := interfaceSliceToWrappedStringSlice(m["applications"].(*schema.Set).List())
	subsystems := interfaceSliceToWrappedStringSlice(m["subsystems"].(*schema.Set).List())
	return &alerts.AlertFilters_RatioAlert{
		Alias:        alias,
		Text:         text,
		Severities:   severities,
		Applications: applications,
		Subsystems:   subsystems,
		GroupBy:      groupBy,
	}
}

func expandNewValue(m map[string]interface{}) *alertParams {
	conditionMap := extractConditionMap(m)
	condition := expandNewValueCondition(conditionMap)
	filters := expandNewValueFilters(m)

	return &alertParams{
		Condition: condition,
		Filters:   filters,
	}
}

func expandNewValueCondition(m map[string]interface{}) *alerts.AlertCondition {
	parameters := expandNewValueConditionParameters(m)
	condition := &alerts.AlertCondition{
		Condition: &alerts.AlertCondition_NewValue{
			NewValue: &alerts.NewValueCondition{
				Parameters: parameters,
			},
		},
	}
	return condition
}

func expandNewValueConditionParameters(m map[string]interface{}) *alerts.ConditionParameters {
	timeFrame := expandNewValueTimeFrame(m["time_window"].(string))
	groupBy := []*wrapperspb.StringValue{wrapperspb.String(m["key_to_track"].(string))}
	parameters := &alerts.ConditionParameters{
		Timeframe: timeFrame,
		GroupBy:   groupBy,
	}
	return parameters
}

func expandNewValueFilters(m map[string]interface{}) *alerts.AlertFilters {
	filters := expandCommonAlertFilter(m)
	filters.FilterType = alerts.AlertFilters_FILTER_TYPE_TEXT_OR_UNSPECIFIED
	return filters
}

func expandUniqueCount(m map[string]interface{}) *alertParams {
	conditionMap := extractConditionMap(m)
	condition := expandUniqueCountCondition(conditionMap)
	filters := expandUniqueCountFilters(m)

	return &alertParams{
		Condition: condition,
		Filters:   filters,
	}
}

func expandUniqueCountCondition(m map[string]interface{}) *alerts.AlertCondition {
	parameters := expandUniqueCountConditionParameters(m)
	return &alerts.AlertCondition{
		Condition: &alerts.AlertCondition_UniqueCount{
			UniqueCount: &alerts.UniqueCountCondition{
				Parameters: parameters,
			},
		},
	}
}

func expandUniqueCountConditionParameters(m map[string]interface{}) *alerts.ConditionParameters {
	uniqueCountKey := []*wrapperspb.StringValue{wrapperspb.String(m["unique_count_key"].(string))}
	threshold := wrapperspb.Double(float64(m["max_unique_values"].(int)))
	timeFrame := expandUniqueValueTimeFrame(m["time_window"].(string))

	var groupByThreshold *wrapperspb.UInt32Value
	var groupBy []*wrapperspb.StringValue
	if groupByKey := m["group_by_key"]; groupByKey != nil && groupByKey.(string) != "" {
		groupBy = []*wrapperspb.StringValue{wrapperspb.String(groupByKey.(string))}
		groupByThreshold = wrapperspb.UInt32(uint32(m["max_unique_values_for_group_by"].(int)))
	}

	return &alerts.ConditionParameters{
		CardinalityFields:                 uniqueCountKey,
		Threshold:                         threshold,
		Timeframe:                         timeFrame,
		GroupBy:                           groupBy,
		MaxUniqueCountValuesForGroupByKey: groupByThreshold,
	}
}

func expandUniqueCountFilters(m map[string]interface{}) *alerts.AlertFilters {
	filters := expandCommonAlertFilter(m)
	filters.FilterType = alerts.AlertFilters_FILTER_TYPE_UNIQUE_COUNT
	return filters
}

func expandCommonAlertFilter(m map[string]interface{}) *alerts.AlertFilters {
	severities := expandAlertFiltersSeverities(m["severities"].(*schema.Set).List())
	metadata := expandMetadata(m)
	text := wrapperspb.String(m["search_query"].(string))

	return &alerts.AlertFilters{
		Severities: severities,
		Metadata:   metadata,
		Text:       text,
	}
}

func expandTimeRelative(m map[string]interface{}) (*alertParams, diag.Diagnostics) {
	conditionMap := extractConditionMap(m)
	condition, err := expandTimeRelativeCondition(conditionMap)
	if err != nil {
		return nil, diag.FromErr(err)
	}
	filters := expandTimeRelativeFilters(m)

	return &alertParams{
		Condition: condition,
		Filters:   filters,
	}, nil
}

func expandTimeRelativeCondition(m map[string]interface{}) (*alerts.AlertCondition, error) {
	parameters, err := expandTimeRelativeConditionParameters(m)
	if err != nil {
		return nil, err
	}

	return expandLessThanOrMoreThanAlertCondition(m, parameters)
}

func expandLessThanOrMoreThanAlertCondition(
	m map[string]interface{}, parameters *alerts.ConditionParameters) (*alerts.AlertCondition, error) {
	lessThan, err := trueIfIsLessThanFalseIfMoreThanAndErrorOtherwise(m)
	if err != nil {
		return nil, err
	}

	if lessThan {
		return &alerts.AlertCondition{
			Condition: &alerts.AlertCondition_LessThan{
				LessThan: &alerts.LessThanCondition{Parameters: parameters},
			},
		}, nil
	}

	return &alerts.AlertCondition{
		Condition: &alerts.AlertCondition_MoreThan{
			MoreThan: &alerts.MoreThanCondition{Parameters: parameters},
		},
	}, nil
}

func trueIfIsLessThanFalseIfMoreThanAndErrorOtherwise(m map[string]interface{}) (bool, error) {
	if lessThan := m["less_than"]; lessThan != nil && lessThan.(bool) {
		return true, nil
	} else if moreThan := m["more_than"]; moreThan != nil && moreThan.(bool) {
		return false, nil
	}
	return false, fmt.Errorf("less_than or more_than have to be true")
}

func expandPromqlCondition(m map[string]interface{}, parameters *alerts.ConditionParameters) (*alerts.AlertCondition, error) {
	conditionsStr, err := returnAlertConditionString(m)
	if err != nil {
		return nil, err
	}

	switch conditionsStr {
	case "less_than":
		return &alerts.AlertCondition{
			Condition: &alerts.AlertCondition_LessThan{
				LessThan: &alerts.LessThanCondition{Parameters: parameters},
			},
		}, nil
	case "more_than":
		return &alerts.AlertCondition{
			Condition: &alerts.AlertCondition_MoreThan{
				MoreThan: &alerts.MoreThanCondition{Parameters: parameters},
			},
		}, nil
	case "more_than_usual":
		return &alerts.AlertCondition{
			Condition: &alerts.AlertCondition_MoreThanUsual{
				MoreThanUsual: &alerts.MoreThanUsualCondition{Parameters: parameters},
			},
		}, nil
	case "less_than_usual":
		return &alerts.AlertCondition{
			Condition: &alerts.AlertCondition_LessThanUsual{
				LessThanUsual: &alerts.LessThanUsualCondition{Parameters: parameters},
			},
		}, nil
	case "less_than_or_equal":
		return &alerts.AlertCondition{
			Condition: &alerts.AlertCondition_LessThanOrEqual{
				LessThanOrEqual: &alerts.LessThanOrEqualCondition{Parameters: parameters},
			},
		}, nil
	case "more_than_or_equal":
		return &alerts.AlertCondition{
			Condition: &alerts.AlertCondition_MoreThanOrEqual{
				MoreThanOrEqual: &alerts.MoreThanOrEqualCondition{Parameters: parameters},
			},
		}, nil
	}

	return nil, fmt.Errorf("less_than, more_than, more_than_usual, less_than_usual, less_than_or_equal, or more_than_or_equal must be set to true")
}

func returnAlertConditionString(m map[string]interface{}) (string, error) {
	if lessThan := m["less_than"]; lessThan != nil && lessThan.(bool) {
		return "less_than", nil
	} else if moreThan := m["more_than"]; moreThan != nil && moreThan.(bool) {
		return "more_than", nil
	} else if moreThanUsual := m["more_than_usual"]; moreThanUsual != nil && moreThanUsual.(bool) {
		return "more_than_usual", nil
	} else if lessThanUsual := m["less_than_usual"]; lessThanUsual != nil && lessThanUsual.(bool) {
		return "less_than_usual", nil
	} else if lessThanOrEqual := m["less_than_or_equal"]; lessThanOrEqual != nil && lessThanOrEqual.(bool) {
		return "less_than_or_equal", nil
	} else if moreThanOrEqual := m["more_than_or_equal"]; moreThanOrEqual != nil && moreThanOrEqual.(bool) {
		return "more_than_or_equal", nil
	}

	return "", fmt.Errorf("less_than, more_than, more_than_usual, less_than_usual, less_than_or_equal, or more_than_or_equal must be set to true")
}

func expandTimeRelativeConditionParameters(m map[string]interface{}) (*alerts.ConditionParameters, error) {
	timeFrame, relativeTimeframe := expandTimeFrameAndRelativeTimeframe(m["relative_time_window"].(string))
	ignoreInfinity := wrapperspb.Bool(m["ignore_infinity"].(bool))
	groupBy := interfaceSliceToWrappedStringSlice(m["group_by"].([]interface{}))
	threshold := wrapperspb.Double(m["ratio_threshold"].(float64))
	relatedExtendedData, err := expandRelatedExtendedData(m)
	if err != nil {
		return nil, err
	}

	return &alerts.ConditionParameters{
		Timeframe:           timeFrame,
		RelativeTimeframe:   relativeTimeframe,
		GroupBy:             groupBy,
		Threshold:           threshold,
		IgnoreInfinity:      ignoreInfinity,
		RelatedExtendedData: relatedExtendedData,
	}, nil
}

func expandTimeFrameAndRelativeTimeframe(relativeTimeframeStr string) (alerts.Timeframe, alerts.RelativeTimeframe) {
	p := alertSchemaRelativeTimeFrameToProtoTimeFrameAndRelativeTimeFrame[relativeTimeframeStr]
	return p.timeFrame, p.relativeTimeFrame
}

func expandTimeRelativeFilters(m map[string]interface{}) *alerts.AlertFilters {
	filters := expandCommonAlertFilter(m)
	filters.FilterType = alerts.AlertFilters_FILTER_TYPE_TIME_RELATIVE
	return filters
}

func expandMetric(m map[string]interface{}) (*alertParams, diag.Diagnostics) {
	condition, err := expandMetricCondition(m)
	if err != nil {
		return nil, diag.FromErr(err)
	}
	filters := expandMetricFilters(m)

	return &alertParams{
		Condition: condition,
		Filters:   filters,
	}, nil
}

func expandMetricCondition(m map[string]interface{}) (*alerts.AlertCondition, error) {
	isPromQL := len(m["promql"].([]interface{})) > 0
	var metricType string
	if isPromQL {
		metricType = "promql"
	} else {
		metricType = "lucene"
	}

	metricMap := (m[metricType].([]interface{}))[0].(map[string]interface{})
	text := wrapperspb.String(metricMap["search_query"].(string))
	conditionMap := extractConditionMap(metricMap)
	threshold := wrapperspb.Double(conditionMap["threshold"].(float64))
	sampleThresholdPercentage := wrapperspb.UInt32(uint32(conditionMap["sample_threshold_percentage"].(int)))
	nonNullPercentage := wrapperspb.UInt32(uint32(conditionMap["min_non_null_values_percentage"].(int)))
	swapNullValues := wrapperspb.Bool(conditionMap["replace_missing_value_with_zero"].(bool))
	timeFrame := expandMetricTimeFrame(conditionMap["time_window"].(string))
	relatedExtendedData, err := expandRelatedExtendedData(conditionMap)
	if err != nil {
		return nil, err
	}

	parameters := &alerts.ConditionParameters{
		Threshold:           threshold,
		Timeframe:           timeFrame,
		RelatedExtendedData: relatedExtendedData,
	}

	if isPromQL {
		parameters.MetricAlertPromqlParameters = &alerts.MetricAlertPromqlConditionParameters{
			PromqlText:                text,
			SampleThresholdPercentage: sampleThresholdPercentage,
			NonNullPercentage:         nonNullPercentage,
			SwapNullValues:            swapNullValues,
		}
	} else {
		metricField := wrapperspb.String(conditionMap["metric_field"].(string))
		arithmeticOperator := expandArithmeticOperator(conditionMap["arithmetic_operator"].(string))
		arithmeticOperatorModifier := wrapperspb.UInt32(uint32(conditionMap["arithmetic_operator_modifier"].(int)))
		groupBy := interfaceSliceToWrappedStringSlice(conditionMap["group_by"].([]interface{}))
		parameters.GroupBy = groupBy
		parameters.MetricAlertParameters = &alerts.MetricAlertConditionParameters{
			MetricSource:               alerts.MetricAlertConditionParameters_METRIC_SOURCE_LOGS2METRICS_OR_UNSPECIFIED,
			MetricField:                metricField,
			ArithmeticOperator:         arithmeticOperator,
			ArithmeticOperatorModifier: arithmeticOperatorModifier,
			SampleThresholdPercentage:  sampleThresholdPercentage,
			NonNullPercentage:          nonNullPercentage,
			SwapNullValues:             swapNullValues,
		}
	}

	return expandPromqlCondition(conditionMap, parameters)
}

func expandArithmeticOperator(s string) alerts.MetricAlertConditionParameters_ArithmeticOperator {
	arithmeticStr := alertSchemaArithmeticOperatorToProtoArithmetic[s]
	arithmeticValue := alerts.MetricAlertConditionParameters_ArithmeticOperator_value[arithmeticStr]
	return alerts.MetricAlertConditionParameters_ArithmeticOperator(arithmeticValue)
}

func expandMetricFilters(m map[string]interface{}) *alerts.AlertFilters {
	var text *wrapperspb.StringValue
	if len(m["promql"].([]interface{})) == 0 {
		luceneArr := m["lucene"].([]interface{})
		lucene := luceneArr[0].(map[string]interface{})
		text = wrapperspb.String(lucene["search_query"].(string))
	}

	return &alerts.AlertFilters{
		FilterType: alerts.AlertFilters_FILTER_TYPE_METRIC,
		Text:       text,
	}
}

func expandFlow(m map[string]interface{}) *alertParams {
	stages := expandFlowStages(m["stage"])
	parameters := expandFlowParameters(m["group_by"])
	return &alertParams{
		Condition: &alerts.AlertCondition{
			Condition: &alerts.AlertCondition_Flow{
				Flow: &alerts.FlowCondition{
					Stages:     stages,
					Parameters: parameters,
				},
			},
		},
		Filters: &alerts.AlertFilters{
			FilterType: alerts.AlertFilters_FILTER_TYPE_FLOW,
		},
	}
}

func expandFlowParameters(i interface{}) *alerts.ConditionParameters {
	if i == nil {
		return nil
	}
	groupBy := interfaceSliceToWrappedStringSlice(i.([]interface{}))
	if len(groupBy) == 0 {
		return nil
	}

	return &alerts.ConditionParameters{
		GroupBy: groupBy,
	}
}

func expandFlowStages(i interface{}) []*alerts.FlowStage {
	l := i.([]interface{})
	result := make([]*alerts.FlowStage, 0, len(l))
	for _, v := range l {
		stage := expandFlowStage(v)
		result = append(result, stage)
	}

	return result
}

func expandFlowStage(i interface{}) *alerts.FlowStage {
	m := i.(map[string]interface{})
	groups := expandGroups(m["group"])
	timeFrame := expandFlowTimeFrame(m["time_window"])
	return &alerts.FlowStage{Groups: groups, Timeframe: timeFrame}
}

func expandGroups(v interface{}) []*alerts.FlowGroup {
	groups := v.([]interface{})
	result := make([]*alerts.FlowGroup, 0, len(groups))
	for _, g := range groups {
		group := expandFlowGroup(g)
		result = append(result, group)
	}

	return result
}

func expandFlowGroup(v interface{}) *alerts.FlowGroup {
	m := v.(map[string]interface{})
	subAlerts := expandSubAlerts(m["sub_alerts"])
	operator := expandOperator(m["next_operator"])
	return &alerts.FlowGroup{
		Alerts: subAlerts,
		NextOp: operator,
	}
}

func expandSubAlerts(v interface{}) *alerts.FlowAlerts {
	l := v.([]interface{})
	if len(l) == 0 || l[0] == nil {
		return nil
	}
	raw := l[0]
	m := raw.(map[string]interface{})

	operator := expandOperator(m["operator"])
	values := expandInnerFlowAlerts(m["flow_alert"])

	return &alerts.FlowAlerts{
		Op:     operator,
		Values: values,
	}
}

func expandInnerFlowAlerts(v interface{}) []*alerts.FlowAlert {
	flowAlerts := v.([]interface{})
	result := make([]*alerts.FlowAlert, 0, len(flowAlerts))
	for _, fa := range flowAlerts {
		flowAlert := expandInnerFlowAlert(fa)
		result = append(result, flowAlert)
	}
	return result
}

func expandInnerFlowAlert(v interface{}) *alerts.FlowAlert {
	m := v.(map[string]interface{})
	return &alerts.FlowAlert{
		Id:  wrapperspb.String(m["user_alert_id"].(string)),
		Not: wrapperspb.Bool(m["not"].(bool)),
	}
}

func expandOperator(i interface{}) alerts.FlowOperator {
	operatorStr := i.(string)
	return alerts.FlowOperator(alerts.FlowOperator_value[operatorStr])
}

func expandFlowTimeFrame(i interface{}) *alerts.FlowTimeframe {
	return &alerts.FlowTimeframe{
		Ms: wrapperspb.UInt32(uint32(expandTimeToMS(i))),
	}
}

func expandTracing(m map[string]interface{}) (*alertParams, *alerts.TracingAlert) {
	tracingParams, _ := expandTracingParams(m)
	tracingAlert := expandTracingAlert(m)

	return tracingParams, tracingAlert
}

func expandTracingParams(m map[string]interface{}) (*alertParams, error) {
	conditionMap := extractConditionMap(m)
	condition, err := expandTracingCondition(conditionMap)
	if err != nil {
		return nil, err
	}
	filters := expandTracingFilter()
	return &alertParams{
		Condition: condition,
		Filters:   filters,
	}, nil
}

func expandTracingCondition(m map[string]interface{}) (*alerts.AlertCondition, error) {
	if immediately := m["immediately"]; immediately != nil && immediately.(bool) {
		return &alerts.AlertCondition{
			Condition: &alerts.AlertCondition_Immediate{},
		}, nil
	} else if moreThan := m["more_than"]; moreThan != nil && moreThan.(bool) {
		parameters := expandTracingConditionParameters(m)
		return &alerts.AlertCondition{
			Condition: &alerts.AlertCondition_MoreThan{
				MoreThan: &alerts.MoreThanCondition{Parameters: parameters},
			},
		}, nil
	}

	return nil, fmt.Errorf("immediately or more_than have to be true")
}

func expandTracingFilter() *alerts.AlertFilters {
	return &alerts.AlertFilters{
		FilterType: alerts.AlertFilters_FILTER_TYPE_TRACING,
	}
}

func expandTracingAlert(m map[string]interface{}) *alerts.TracingAlert {
	conditionLatency := uint32(m["latency_threshold_milliseconds"].(float64) * (float64)(time.Millisecond.Microseconds()))
	applications := m["applications"].(*schema.Set).List()
	subsystems := m["subsystems"].(*schema.Set).List()
	services := m["services"].(*schema.Set).List()
	fieldFilters := expandFiltersData(applications, subsystems, services)
	tagFilters := expandTagFilters(m["tag_filter"])
	return &alerts.TracingAlert{
		ConditionLatency: conditionLatency,
		FieldFilters:     fieldFilters,
		TagFilters:       tagFilters,
	}
}

func expandFiltersData(applications, subsystems, services []interface{}) []*alerts.FilterData {
	result := make([]*alerts.FilterData, 0)
	if len(applications) != 0 {
		result = append(result, expandSpecificFilter("applicationName", applications))
	}
	if len(subsystems) != 0 {
		result = append(result, expandSpecificFilter("subsystemName", subsystems))
	}
	if len(services) != 0 {
		result = append(result, expandSpecificFilter("serviceName", services))
	}

	return result
}

func expandTagFilters(i interface{}) []*alerts.FilterData {
	if i == nil {
		return nil
	}
	l := i.(*schema.Set).List()

	result := make([]*alerts.FilterData, 0, len(l))
	for _, v := range l {
		m := v.(map[string]interface{})
		field := m["field"].(string)
		values := m["values"].(*schema.Set).List()
		result = append(result, expandSpecificFilter(field, values))
	}
	return result
}

func expandSpecificFilter(filterName string, values []interface{}) *alerts.FilterData {
	operatorToFilterValues := make(map[string]*alerts.Filters)
	for _, val := range values {
		operator, filterValue := expandFilter(val.(string))
		if _, ok := operatorToFilterValues[operator]; !ok {
			operatorToFilterValues[operator] = new(alerts.Filters)
			operatorToFilterValues[operator].Operator = operator
			operatorToFilterValues[operator].Values = make([]string, 0)
		}
		operatorToFilterValues[operator].Values = append(operatorToFilterValues[operator].Values, filterValue)
	}

	filterResult := make([]*alerts.Filters, 0, len(operatorToFilterValues))
	for _, filters := range operatorToFilterValues {
		filterResult = append(filterResult, filters)
	}

	return &alerts.FilterData{
		Field:   filterName,
		Filters: filterResult,
	}
}

func expandFilter(filterString string) (operator, filterValue string) {
	operator, filterValue = "equals", filterString
	if strings.HasPrefix(filterValue, "filter:") {
		arr := strings.SplitN(filterValue, ":", 3)
		operator, filterValue = arr[1], arr[2]
	}

	return
}

func extractConditionMap(m map[string]interface{}) map[string]interface{} {
	return m["condition"].([]interface{})[0].(map[string]interface{})
}

func expandTimeFrame(s string) alerts.Timeframe {
	protoTimeFrame := alertSchemaTimeFrameToProtoTimeFrame[s]
	return alerts.Timeframe(alerts.Timeframe_value[protoTimeFrame])
}

func expandMetricTimeFrame(s string) alerts.Timeframe {
	protoTimeFrame := alertSchemaMetricTimeFrameToMetricProtoTimeFrame[s]
	return alerts.Timeframe(alerts.Timeframe_value[protoTimeFrame])
}

func expandMetadata(m map[string]interface{}) *alerts.AlertFilters_MetadataFilters {
	categories := interfaceSliceToWrappedStringSlice(m["categories"].(*schema.Set).List())
	applications := interfaceSliceToWrappedStringSlice(m["applications"].(*schema.Set).List())
	subsystems := interfaceSliceToWrappedStringSlice(m["subsystems"].(*schema.Set).List())
	computers := interfaceSliceToWrappedStringSlice(m["computers"].(*schema.Set).List())
	classes := interfaceSliceToWrappedStringSlice(m["classes"].(*schema.Set).List())
	methods := interfaceSliceToWrappedStringSlice(m["methods"].(*schema.Set).List())
	ipAddresses := interfaceSliceToWrappedStringSlice(m["ip_addresses"].(*schema.Set).List())

	return &alerts.AlertFilters_MetadataFilters{
		Categories:   categories,
		Applications: applications,
		Subsystems:   subsystems,
		Computers:    computers,
		Classes:      classes,
		Methods:      methods,
		IpAddresses:  ipAddresses,
	}
}

func expandAlertFiltersSeverities(v interface{}) []alerts.AlertFilters_LogSeverity {
	s := interfaceSliceToStringSlice(v.([]interface{}))
	result := make([]alerts.AlertFilters_LogSeverity, 0, len(s))
	for _, v := range s {
		logSeverityStr := alertSchemaLogSeverityToProtoLogSeverity[v]
		result = append(result, alerts.AlertFilters_LogSeverity(
			alerts.AlertFilters_LogSeverity_value[logSeverityStr]))
	}

	return result
}

func expandNewValueTimeFrame(s string) alerts.Timeframe {
	protoTimeFrame := alertSchemaNewValueTimeFrameToProtoTimeFrame[s]
	return alerts.Timeframe(alerts.Timeframe_value[protoTimeFrame])
}

func expandUniqueValueTimeFrame(s string) alerts.Timeframe {
	protoTimeFrame := alertSchemaUniqueCountTimeFrameToProtoTimeFrame[s]
	return alerts.Timeframe(alerts.Timeframe_value[protoTimeFrame])
}

func expandTimeInDay(v interface{}) *alerts.Time {
	timeArr := strings.Split(v.(string), ":")
	hours := parseNumInt32(timeArr[0])
	minutes := parseNumInt32(timeArr[1])
	return &alerts.Time{
		Hours:   hours,
		Minutes: minutes,
	}
}
