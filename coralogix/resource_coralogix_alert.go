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
	alertsv1 "terraform-provider-coralogix/coralogix/clientset/grpc/com/coralogix/alerts/v1"

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
		"Previous_hour":       {timeFrame: alertsv1.Timeframe_TIMEFRAME_1_H, relativeTimeFrame: alertsv1.RelativeTimeframe_RELATIVE_TIMEFRAME_HOUR_OR_UNSPECIFIED},
		"Same_hour_yesterday": {timeFrame: alertsv1.Timeframe_TIMEFRAME_1_H, relativeTimeFrame: alertsv1.RelativeTimeframe_RELATIVE_TIMEFRAME_DAY},
		"Same_hour_last_week": {timeFrame: alertsv1.Timeframe_TIMEFRAME_1_H, relativeTimeFrame: alertsv1.RelativeTimeframe_RELATIVE_TIMEFRAME_WEEK},
		"Yesterday":           {timeFrame: alertsv1.Timeframe_TIMEFRAME_24_H, relativeTimeFrame: alertsv1.RelativeTimeframe_RELATIVE_TIMEFRAME_DAY},
		"Same_day_last_week":  {timeFrame: alertsv1.Timeframe_TIMEFRAME_24_H, relativeTimeFrame: alertsv1.RelativeTimeframe_RELATIVE_TIMEFRAME_WEEK},
		"Same_day_last_month": {timeFrame: alertsv1.Timeframe_TIMEFRAME_24_H, relativeTimeFrame: alertsv1.RelativeTimeframe_RELATIVE_TIMEFRAME_MONTH},
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
	alertSchemaTracingOperatorToProtoTracingOperator = map[string]string{
		"Equals":     "equals",
		"Contains":   "contains",
		"Start_with": "startsWith",
		"End_with":   "endsWith"}
	alertProtoTracingOperatorToSchemaTracingOperator       = reverseMapStrings(alertSchemaTracingOperatorToProtoTracingOperator)
	alertValidTracingOperator                              = getKeysStrings(alertSchemaTracingOperatorToProtoTracingOperator)
	alertSchemaTracingFilterFieldToProtoTracingFilterField = map[string]string{
		"Application": "applicationName",
		"Subsystem":   "subsystemName",
		"Service":     "serviceName",
	}
	alertProtoTracingFilterFieldToSchemaTracingFilterField = reverseMapStrings(alertSchemaTracingFilterFieldToProtoTracingFilterField)
	alertValidTracingFilterField                           = getKeysStrings(alertSchemaTracingFilterFieldToProtoTracingFilterField)
	alertValidFlowOperator                                 = getKeysInt(alertsv1.FlowOperator_value)
	alertSchemaMetricTimeFrameToMetricProtoTimeFrame       = map[string]string{
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
	alertValidTimeZones                          = []string{"UTC-11", "UTC-10", "UTC-9", "UTC-8", "UTC-7", "UTC-6", "UTC-5", "UTC-4", "UTC-3", "UTC-2", "UTC-1",
		"UTC+0", "UTC+1", "UTC+2", "UTC+3", "UTC+4", "UTC+5", "UTC+6", "UTC+7", "UTC+8", "UTC+9", "UTC+10", "UTC+11", "UTC+12", "UTC+13", "UTC+14"}
)

type alertParams struct {
	Condition *alertsv1.AlertCondition
	Filters   *alertsv1.AlertFilters
}

type notification struct {
	notifyEverySec                     *wrapperspb.DoubleValue
	ignoreInfinity                     *wrapperspb.BoolValue
	notifyWhenResolved                 *wrapperspb.BoolValue
	notifyOnlyOnTriggeredGroupByValues *wrapperspb.BoolValue
	recipients                         *alertsv1.AlertNotifications
	payloadFields                      []*wrapperspb.StringValue
}

type protoTimeFrameAndRelativeTimeFrame struct {
	timeFrame         alertsv1.Timeframe
	relativeTimeFrame alertsv1.RelativeTimeframe
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
			Create: schema.DefaultTimeout(60 * time.Minute),
			Read:   schema.DefaultTimeout(30 * time.Minute),
			Update: schema.DefaultTimeout(60 * time.Minute),
			Delete: schema.DefaultTimeout(30 * time.Minute),
		},

		Schema: AlertSchema(),

		Description: "Coralogix alert. Api-key is required for this resource." +
			" More info: https://coralogix.com/docs/alerts-api/ .",
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
		"alert_severity": {
			Type:         schema.TypeString,
			Required:     true,
			ValidateFunc: validation.StringInSlice(alertValidSeverities, false),
			Description:  fmt.Sprintf("Determines the alert's severity. Can be one of %q", alertValidSeverities),
		},
		"meta_labels": {
			Type:        schema.TypeSet,
			Optional:    true,
			Elem:        metaLabels(),
			Set:         hashMetaLabels(),
			Description: "Labels allow you to easily filter by alert type and create views. Insert a new label or use an existing one. You can nest a label using key:value.",
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
		"notification": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"on_trigger_and_resolved": {
						Type:          schema.TypeBool,
						Optional:      true,
						ConflictsWith: []string{"new_value", "unique_count", "time_relative", "flow", "standard.0.condition.0.immediately", "standard.0.condition.0.more_than_usual"},
					},
					"ignore_infinity": {
						Type:          schema.TypeBool,
						Optional:      true,
						ConflictsWith: []string{"standard", "new_value", "unique_count", "metric", "tracing", "flow"},
					},
					"notify_only_on_triggered_group_by_values": {
						Type:          schema.TypeBool,
						Optional:      true,
						Default:       false,
						Description:   "Notifications will contain only triggered group-by values.",
						ConflictsWith: []string{"new_value", "unique_count", "metric.0.promql", "tracing", "flow"},
					},
					"recipients": {
						Type:     schema.TypeList,
						Optional: true,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"emails": {
									Type:     schema.TypeSet,
									Optional: true,
									Elem: &schema.Schema{
										Type: schema.TypeString,
										ValidateFunc: validation.StringMatch(
											regexp.MustCompile(`^[a-z/d._%+\-]+@[a-z/d.\-]+\.[a-z]{2,4}$`), "not valid mail address"),
									},
									Description: "The emails for anyone that should receive this alert.",
									Set:         schema.HashString,
								},
								"webhook_ids": {
									Type:     schema.TypeSet,
									Optional: true,
									Elem: &schema.Schema{
										Type: schema.TypeString,
									},
									Description: "The Webhook-integrations to send the alert to.",
									Set:         schema.HashString,
								},
							},
						},
						MaxItems: 1,
					},
					"notify_every_min": {
						Type:         schema.TypeInt,
						Optional:     true,
						Default:      1,
						ValidateFunc: validation.IntAtLeast(1),
						Description: "By default, notify_every_min will be populated with min for immediate," +
							" more_than and more_than_usual alerts. For less_than alert it will be populated with the chosen time" +
							" frame for the less_than condition (in seconds). You may choose to change the suppress window so the " +
							"alert will be suppressed for a longer period.",
					},
					"payload_fields": {
						Type:     schema.TypeSet,
						Optional: true,
						Elem: &schema.Schema{
							Type: schema.TypeString,
						},
						Description: "A list of log fields out of the log example which will be included with the alert notification.",
						Set:         schema.HashString,
					},
				},
			},
			MaxItems:    1,
			Description: "The Alert notification info.",
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
			Description:  "Alert on a never before seen log value.",
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

func schedulingSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"time_zone": {
			Type:         schema.TypeString,
			Optional:     true,
			Default:      "UTC+0",
			ValidateFunc: validation.StringInSlice(alertValidTimeZones, false),
			Description:  fmt.Sprintf("Specifies the time zone to be used in interpreting the schedule. Can be one of %q", alertValidTimeZones),
		},
		"time_frames": {
			Type:        schema.TypeSet,
			Required:    true,
			Elem:        timeFrames(),
			Set:         hashTimeFrames(),
			Description: "time_frames is a set of days and hours when the alert will be active.",
		},
	}
}

func timeFrames() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"days_enabled": {
				Type:     schema.TypeSet,
				Optional: true,
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

func metaLabels() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"key": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringMatch(regexp.MustCompile(`^[A-Za-z\d_-]*$`), "not valid key"),
				Description:  "Label key.",
			},
			"value": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Label value.",
			},
		},
	}
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
			Description: "An array that contains log’s application names that we want to be alerted on.",
			Set:         schema.HashString,
		},
		"subsystems": {
			Type:     schema.TypeSet,
			Optional: true,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
			Description: "An array that contains log’s subsystem names that we want to be notified on.",
			Set:         schema.HashString,
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
		Type:         schema.TypeString,
		Optional:     true,
		ValidateFunc: validation.StringIsValidRegExp,
		Description:  "The search_query that we wanted to be notified on.",
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
				},
				"more_than": {
					Type:     schema.TypeBool,
					Optional: true,
					ExactlyOneOf: []string{"standard.0.condition.0.immediately",
						"standard.0.condition.0.more_than",
						"standard.0.condition.0.less_than",
						"standard.0.condition.0.more_than_usual"},
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
				"occurrences_threshold": {
					Type:          schema.TypeInt,
					Optional:      true,
					ConflictsWith: []string{"standard.0.condition.0.immediately"},
					Description:   "The number of log occurrences that is needed to trigger the alert.",
				},
				"time_window": {
					Type:         schema.TypeString,
					Optional:     true,
					ValidateFunc: validation.StringInSlice(alertValidTimeFrames, false),
					ConflictsWith: []string{"standard.0.condition.0.immediately",
						"standard.0.condition.0.more_than_usual"},
				},
				"group_by": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
					ConflictsWith: []string{"standard.0.condition.0.immediately",
						"standard.0.condition.0.more_than_usual"},
					Description: "The fields to 'group by' on.",
				},
				"group_by_key": {
					Type:     schema.TypeString,
					Optional: true,
					ConflictsWith: []string{"standard.0.condition.0.immediately",
						"standard.0.condition.0.more_than", "standard.0.condition.0.less_than"},
					Description: "The key to 'group by' on.",
				},
				"manage_undetected_values": {
					Type:     schema.TypeList,
					Optional: true,
					MaxItems: 1,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"enable_triggering_on_undetected_values": {
								Type:         schema.TypeBool,
								Optional:     true,
								ExactlyOneOf: []string{"standard.0.condition.0.manage_undetected_values.0.enable_triggering_on_undetected_values", "standard.0.condition.0.manage_undetected_values.0.disable_triggering_on_undetected_values"},
							},
							"disable_triggering_on_undetected_values": {
								Type:         schema.TypeBool,
								Optional:     true,
								ExactlyOneOf: []string{"standard.0.condition.0.manage_undetected_values.0.enable_triggering_on_undetected_values", "standard.0.condition.0.manage_undetected_values.0.disable_triggering_on_undetected_values"},
							},
							"auto_retire_ratio": {
								Type:          schema.TypeString,
								Optional:      true,
								RequiredWith:  []string{"standard.0.condition.0.manage_undetected_values.0.enable_triggering_on_undetected_values"},
								ConflictsWith: []string{"standard.0.condition.0.manage_undetected_values.0.disable_triggering_on_undetected_values"},
								ValidateFunc:  validation.StringInSlice(alertValidDeadmanRatioValues, false),
							},
						},
					},
					RequiredWith: []string{"standard.0.condition.0.less_than", "standard.0.condition.0.group_by"},
				},
			},
		},
		Description: fmt.Sprintf("Target alert by subsystems contained within the logs. Can be one of %q",
			alertValidSeverities),
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
					"applications": {
						Type:     schema.TypeList,
						Optional: true,
						Elem: &schema.Schema{
							Type: schema.TypeString,
						},
						Description: "Target alert by application contained within the logs.",
					},
					"subsystems": {
						Type:     schema.TypeList,
						Optional: true,
						Elem: &schema.Schema{
							Type: schema.TypeString,
						},
						Description: "Target alert by subsystems contained within the logs.",
					},
					"severities": {
						Type:     schema.TypeList,
						Optional: true,
						Elem: &schema.Schema{
							Type:         schema.TypeString,
							ValidateFunc: validation.StringInSlice(alertValidLogSeverities, false),
						},
						Description: fmt.Sprintf("Target alert by severities contained within the logs. Can be one of %q", alertValidLogSeverities),
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
						Computed:     true,
						ExactlyOneOf: []string{"ratio.0.condition.0.more_than", "ratio.0.condition.0.less_than"},
						Description: "Determines the condition operator." +
							" Must be one of - less_than or more_than.",
					},
					"less_than": {
						Type:         schema.TypeBool,
						Optional:     true,
						Computed:     true,
						ExactlyOneOf: []string{"ratio.0.condition.0.more_than", "ratio.0.condition.0.less_than"},
					},
					"queries_ratio": {
						Type:     schema.TypeFloat,
						Required: true,
					},
					"time_window": {
						Type:         schema.TypeString,
						Required:     true,
						ValidateFunc: validation.StringInSlice(alertValidTimeFrames, false),
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
									Type:         schema.TypeBool,
									Optional:     true,
									ExactlyOneOf: []string{"ratio.0.condition.0.manage_undetected_values.0.enable_triggering_on_undetected_values", "ratio.0.condition.0.manage_undetected_values.0.disable_triggering_on_undetected_values"},
								},
								"disable_triggering_on_undetected_values": {
									Type:         schema.TypeBool,
									Optional:     true,
									ExactlyOneOf: []string{"ratio.0.condition.0.manage_undetected_values.0.enable_triggering_on_undetected_values", "ratio.0.condition.0.manage_undetected_values.0.disable_triggering_on_undetected_values"},
								},
								"auto_retire_ratio": {
									Type:          schema.TypeString,
									Optional:      true,
									RequiredWith:  []string{"ratio.0.condition.0.manage_undetected_values.0.enable_triggering_on_undetected_values"},
									ConflictsWith: []string{"ratio.0.condition.0.manage_undetected_values.0.disable_triggering_on_undetected_values"},
									ValidateFunc:  validation.StringInSlice(alertValidDeadmanRatioValues, false),
								},
							},
						},
						RequiredWith: []string{"ratio.0.condition.0.less_than", "ratio.0.condition.0.group_by"},
					},
				},
			},
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
					Type:         schema.TypeInt,
					Required:     true,
					ValidateFunc: validation.IntBetween(1, 1000),
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
					ValidateFunc: validation.IntBetween(1, 1000),
					RequiredWith: []string{"unique_count.0.condition.0.group_by_key"},
				},
			},
		},
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
								Type:         schema.TypeBool,
								Optional:     true,
								ExactlyOneOf: []string{"time_relative.0.condition.0.manage_undetected_values.0.enable_triggering_on_undetected_values", "time_relative.0.condition.0.manage_undetected_values.0.disable_triggering_on_undetected_values"},
							},
							"disable_triggering_on_undetected_values": {
								Type:         schema.TypeBool,
								Optional:     true,
								ExactlyOneOf: []string{"time_relative.0.condition.0.manage_undetected_values.0.enable_triggering_on_undetected_values", "time_relative.0.condition.0.manage_undetected_values.0.disable_triggering_on_undetected_values"},
							},
							"auto_retire_ratio": {
								Type:          schema.TypeString,
								Optional:      true,
								RequiredWith:  []string{"time_relative.0.condition.0.manage_undetected_values.0.enable_triggering_on_undetected_values"},
								ConflictsWith: []string{"time_relative.0.condition.0.manage_undetected_values.0.disable_triggering_on_undetected_values"},
								ValidateFunc:  validation.StringInSlice(alertValidDeadmanRatioValues, false),
							},
						},
					},
					RequiredWith: []string{"time_relative.0.condition.0.less_than", "time_relative.0.condition.0.group_by"},
				},
			},
		},
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
						Type:         schema.TypeString,
						Required:     true,
						ValidateFunc: validation.StringIsValidRegExp,
						Description:  "Regular expiration. More info: https://coralogix.com/blog/regex-101/",
					},
					"condition": {
						Type:     schema.TypeList,
						Required: true,
						MaxItems: 1,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"metric_field": {
									Type:     schema.TypeString,
									Required: true,
								},
								"arithmetic_operator": {
									Type:         schema.TypeString,
									Required:     true,
									ValidateFunc: validation.StringInSlice(alertValidArithmeticOperators, false),
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
								"arithmetic_operator_modifier": {
									Type:     schema.TypeInt,
									Required: true,
								},
								"sample_threshold_percentage": {
									Type:         schema.TypeInt,
									Required:     true,
									ValidateFunc: validation.IntBetween(0, 100),
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
								},
								"min_non_null_values_percentage": {
									Type:          schema.TypeInt,
									Optional:      true,
									ConflictsWith: []string{"metric.0.lucene.0.condition.0.replace_missing_value_with_zero"},
								},
								"manage_undetected_values": {
									Type:     schema.TypeList,
									Optional: true,
									Computed: true,
									MaxItems: 1,
									Elem: &schema.Resource{
										Schema: map[string]*schema.Schema{
											"enable_triggering_on_undetected_values": {
												Type:         schema.TypeBool,
												Optional:     true,
												ExactlyOneOf: []string{"metric.0.lucene.0.condition.0.manage_undetected_values.0.enable_triggering_on_undetected_values", "metric.0.lucene.0.condition.0.manage_undetected_values.0.disable_triggering_on_undetected_values"},
											},
											"disable_triggering_on_undetected_values": {
												Type:         schema.TypeBool,
												Optional:     true,
												ExactlyOneOf: []string{"metric.0.lucene.0.condition.0.manage_undetected_values.0.enable_triggering_on_undetected_values", "metric.0.lucene.0.condition.0.manage_undetected_values.0.disable_triggering_on_undetected_values"},
											},
											"auto_retire_ratio": {
												Type:          schema.TypeString,
												Optional:      true,
												RequiredWith:  []string{"metric.0.lucene.0.condition.0.manage_undetected_values.0.enable_triggering_on_undetected_values"},
												ConflictsWith: []string{"metric.0.lucene.0.condition.0.manage_undetected_values.0.disable_triggering_on_undetected_values"},
												ValidateFunc:  validation.StringInSlice(alertValidDeadmanRatioValues, false),
											},
										},
									},
									RequiredWith: []string{"metric.0.lucene.0.condition.0.less_than", "metric.0.lucene.0.condition.0.group_by"},
								},
							},
						},
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
						Type:         schema.TypeString,
						Required:     true,
						ValidateFunc: validation.StringIsValidRegExp,
						Description:  "Regular expiration. More info: https://coralogix.com/blog/regex-101/",
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
									ExactlyOneOf: []string{"metric.0.promql.0.condition.0.less_than",
										"metric.0.promql.0.condition.0.more_than"},
									Description: "Determines the condition operator." +
										" Must be one of - less_than or more_than.",
								},
								"more_than": {
									Type:     schema.TypeBool,
									Optional: true,
									ExactlyOneOf: []string{"metric.0.promql.0.condition.0.less_than",
										"metric.0.promql.0.condition.0.more_than"},
									Description: "Determines the condition operator." +
										" Must be one of - less_than or more_than.",
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
									ValidateFunc: validation.IntBetween(0, 100),
								},
								"replace_missing_value_with_zero": {
									Type:          schema.TypeBool,
									Optional:      true,
									ConflictsWith: []string{"metric.0.promql.0.condition.0.min_non_null_values_percentage"},
								},
								"min_non_null_values_percentage": {
									Type:          schema.TypeInt,
									Optional:      true,
									ConflictsWith: []string{"metric.0.promql.0.condition.0.replace_missing_value_with_zero"},
								},
								"manage_undetected_values": {
									Type:     schema.TypeList,
									Optional: true,
									Computed: true,
									MaxItems: 1,
									Elem: &schema.Resource{
										Schema: map[string]*schema.Schema{
											"enable_triggering_on_undetected_values": {
												Type:         schema.TypeBool,
												Optional:     true,
												ExactlyOneOf: []string{"metric.0.promql.0.condition.0.manage_undetected_values.0.enable_triggering_on_undetected_values", "metric.0.promql.0.condition.0.manage_undetected_values.0.disable_triggering_on_undetected_values"},
											},
											"disable_triggering_on_undetected_values": {
												Type:         schema.TypeBool,
												Optional:     true,
												ExactlyOneOf: []string{"metric.0.promql.0.condition.0.manage_undetected_values.0.enable_triggering_on_undetected_values", "metric.0.promql.0.condition.0.manage_undetected_values.0.disable_triggering_on_undetected_values"},
											},
											"auto_retire_ratio": {
												Type:          schema.TypeString,
												Optional:      true,
												RequiredWith:  []string{"metric.0.promql.0.condition.0.manage_undetected_values.0.enable_triggering_on_undetected_values"},
												ConflictsWith: []string{"metric.0.promql.0.condition.0.manage_undetected_values.0.disable_triggering_on_undetected_values"},
												ValidateFunc:  validation.StringInSlice(alertValidDeadmanRatioValues, false),
											},
										},
									},
									RequiredWith: []string{"metric.0.promql.0.condition.0.less_than"},
								},
							},
						},
					},
				},
			},
			ExactlyOneOf: []string{"metric.0.lucene", "metric.0.promql"},
		},
	}
}

func tracingSchema() map[string]*schema.Schema {
	tracingSchema := commonAlertSchema()
	tracingSchema["latency_threshold_ms"] = &schema.Schema{
		Type:         schema.TypeFloat,
		Optional:     true,
		ValidateFunc: validation.FloatAtLeast(0),
	}
	tracingSchema["tag_filters"] = filtersSchema(false)
	tracingSchema["field_filters"] = filtersSchema(true)
	tracingSchema["condition"] = &schema.Schema{
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
				"occurrences_threshold": {
					Type:          schema.TypeInt,
					Optional:      true,
					ConflictsWith: []string{"tracing.0.condition.0.immediately"},
					Description:   "The number of log occurrences that is needed to trigger the alert.",
				},
				"time_window": {
					Type:          schema.TypeString,
					Optional:      true,
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
	}

	return tracingSchema
}

func filtersSchema(isFieldFilterSchema bool) *schema.Schema {
	fieldSchema := &schema.Schema{
		Type:     schema.TypeString,
		Required: true,
	}

	if isFieldFilterSchema {
		fieldSchema.ValidateFunc = validation.StringInSlice(alertValidTracingFilterField, false)
	}

	return &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"field": fieldSchema,
				"filters": {
					Type:     schema.TypeList,
					Required: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"values": {
								Type:     schema.TypeList,
								Required: true,
								MinItems: 1,
								Elem: &schema.Schema{
									Type: schema.TypeString,
								},
							},
							"operator": {
								Type:         schema.TypeString,
								Required:     true,
								ValidateFunc: validation.StringInSlice(alertValidTracingOperator, false),
							},
						},
					},
				},
			},
		},
	}
}

func flowSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"stages": {
			Type:     schema.TypeList,
			Required: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"groups": {
						Type:     schema.TypeList,
						Required: true,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"sub_alerts": {
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
								"operator": {
									Type:         schema.TypeString,
									Required:     true,
									ValidateFunc: validation.StringInSlice(alertValidFlowOperator, false),
								},
							},
						},
					},
					"time_window": timeSchema("Timeframe for flow stage."),
				},
			},
		},
	}
}

func resourceCoralogixAlertCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	createAlertRequest, err := extractCreateAlertRequest(d)
	if err != nil {
		return diag.FromErr(err)
	}

	log.Printf("[INFO] Creating new alert: %#v", createAlertRequest)
	AlertResp, err := meta.(*clientset.ClientSet).Alerts().CreateAlert(ctx, createAlertRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcError(err, "alert")
	}
	Alert := AlertResp.GetAlert()
	log.Printf("[INFO] Submitted new alert: %#v", Alert)
	d.SetId(Alert.GetId().GetValue())

	return resourceCoralogixAlertRead(ctx, d, meta)
}

func resourceCoralogixAlertRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := wrapperspb.String(d.Id())
	getAlertRequest := &alertsv1.GetAlertRequest{
		Id: id,
	}

	log.Printf("[INFO] Reading alert %s", id)
	alertResp, err := meta.(*clientset.ClientSet).Alerts().GetAlert(ctx, getAlertRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "alert", id.GetValue())
	}
	alert := alertResp.GetAlert()
	log.Printf("[INFO] Received alert: %#v", alert)

	return setAlert(d, alert)
}

func resourceCoralogixAlertUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	req, err := extractAlert(d)
	if err != nil {
		return diag.FromErr(err)
	}

	id := d.Id()
	updateAlertRequest := &alertsv1.UpdateAlertRequest{
		Alert: req,
	}

	log.Printf("[INFO] Updating alert %s", updateAlertRequest)
	alertResp, err := meta.(*clientset.ClientSet).Alerts().UpdateAlert(ctx, updateAlertRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "alert", id)
	}
	log.Printf("[INFO] Submitted updated alert: %#v", alertResp)
	d.SetId(alertResp.GetAlert().GetId().GetValue())

	return resourceCoralogixAlertRead(ctx, d, meta)
}

func resourceCoralogixAlertDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := wrapperspb.String(d.Id())
	deleteAlertRequest := &alertsv1.DeleteAlertRequest{
		Id: id,
	}

	log.Printf("[INFO] Deleting alert %s\n", id)
	_, err := meta.(*clientset.ClientSet).Alerts().DeleteAlert(ctx, deleteAlertRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v\n", err)
		return handleRpcErrorWithID(err, "alert", id.GetValue())
	}
	log.Printf("[INFO] alert %s deleted\n", id)

	d.SetId("")
	return nil
}

func extractCreateAlertRequest(d *schema.ResourceData) (*alertsv1.CreateAlertRequest, error) {
	enabled := wrapperspb.Bool(d.Get("enabled").(bool))
	name := wrapperspb.String(d.Get("name").(string))
	description := wrapperspb.String(d.Get("description").(string))
	severity := expandAlertSeverity(d.Get("alert_severity").(string))
	metaLabels := extractMetaLabels(d.Get("meta_labels"))
	expirationDate := expandExpirationDate(d.Get("expiration_date"))
	notifications := expandNotification(d.Get("notification"))
	scheduling := expandActiveWhen(d.Get("scheduling"))
	alertTypeParams, tracingAlert, err := expandAlertType(d, notifications)
	if err != nil {
		return nil, err
	}

	createAlertRequest := &alertsv1.CreateAlertRequest{
		Name:                       name,
		Description:                description,
		IsActive:                   enabled,
		Severity:                   severity,
		MetaLabels:                 metaLabels,
		Expiration:                 expirationDate,
		Notifications:              notifications.recipients,
		NotifyEvery:                notifications.notifyEverySec,
		NotificationPayloadFilters: notifications.payloadFields,
		ActiveWhen:                 scheduling,
		Filters:                    alertTypeParams.Filters,
		Condition:                  alertTypeParams.Condition,
		TracingAlert:               tracingAlert,
	}

	return createAlertRequest, nil
}

func extractAlert(d *schema.ResourceData) (*alertsv1.Alert, error) {
	enabled := wrapperspb.Bool(d.Get("enabled").(bool))
	name := wrapperspb.String(d.Get("name").(string))
	description := wrapperspb.String(d.Get("description").(string))
	severity := expandAlertSeverity(d.Get("alert_severity").(string))
	metaLabels := extractMetaLabels(d.Get("meta_labels"))
	expirationDate := expandExpirationDate(d.Get("expiration_date"))
	notifications := expandNotification(d.Get("notification"))
	scheduling := expandActiveWhen(d.Get("scheduling"))
	alertTypeParams, tracingAlert, err := expandAlertType(d, notifications)
	if err != nil {
		return nil, err
	}

	createAlertRequest := &alertsv1.Alert{
		Id:                         wrapperspb.String(d.Id()),
		Name:                       name,
		Description:                description,
		IsActive:                   enabled,
		Severity:                   severity,
		MetaLabels:                 metaLabels,
		Expiration:                 expirationDate,
		Notifications:              notifications.recipients,
		NotifyEvery:                notifications.notifyEverySec,
		NotificationPayloadFilters: notifications.payloadFields,
		ActiveWhen:                 scheduling,
		Filters:                    alertTypeParams.Filters,
		Condition:                  alertTypeParams.Condition,
		TracingAlert:               tracingAlert,
	}

	return createAlertRequest, nil
}

func setAlert(d *schema.ResourceData, alert *alertsv1.Alert) diag.Diagnostics {
	if err := d.Set("name", alert.GetName().GetValue()); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("description", alert.GetDescription().GetValue()); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("enabled", alert.GetIsActive().GetValue()); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("alert_severity", flattenAlertSeverity(alert.GetSeverity().String())); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("meta_labels", flattenMetaLabels(alert.GetMetaLabels())); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("expiration_date", flattenExpirationDate(alert.GetExpiration())); err != nil {
		return diag.FromErr(err)
	}

	alertType, alertTypeParams, ignoreInfinity, notifyWhenResolved, notifyOnlyOnTriggeredGroupByValues := flattenAlertType(alert)

	if err := d.Set("notification", flattenNotification(alert, ignoreInfinity, notifyWhenResolved, notifyOnlyOnTriggeredGroupByValues)); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("scheduling", flattenScheduling(d, alert.GetActiveWhen())); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set(alertType, alertTypeParams); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func flattenAlertSeverity(str string) string {
	return alertProtoSeverityToSchemaSeverity[str]
}

func flattenMetaLabels(labels []*alertsv1.MetaLabel) interface{} {
	result := schema.NewSet(hashMetaLabels(), []interface{}{})
	for _, l := range labels {
		m := make(map[string]interface{})
		m["key"] = l.GetKey().GetValue()
		m["value"] = l.GetValue().GetValue()
		result.Add(m)
	}
	return result
}

func hashMetaLabels() schema.SchemaSetFunc {
	return schema.HashResource(metaLabels())
}

func flattenNotification(alert *alertsv1.Alert, ignoreInfinity, notifyWhenResolved, notifyOnlyOnTriggeredGroupByValues *wrapperspb.BoolValue) interface{} {
	recipients := flattenRecipients(alert.GetNotifications())
	notificationMap := map[string]interface{}{
		"notify_every_min": int(alert.GetNotifyEvery().GetValue() / 60),
		"recipients":       recipients,
		"payload_fields":   wrappedStringSliceToStringSlice(alert.NotificationPayloadFilters),
	}
	if ignoreInfinity != nil {
		notificationMap["ignore_infinity"] = ignoreInfinity.GetValue()
	}
	if notifyWhenResolved != nil {
		notificationMap["on_trigger_and_resolved"] = notifyWhenResolved.GetValue()
	}
	if notifyOnlyOnTriggeredGroupByValues != nil {
		notificationMap["notify_only_on_triggered_group_by_values"] = notifyOnlyOnTriggeredGroupByValues.GetValue()
	}

	return []interface{}{
		notificationMap,
	}
}

func flattenRecipients(notifications *alertsv1.AlertNotifications) interface{} {
	return []interface{}{
		map[string]interface{}{
			"emails":      wrappedStringSliceToStringSlice(notifications.GetEmails()),
			"webhook_ids": wrappedStringSliceToStringSlice(notifications.GetIntegrations()),
		},
	}
}

func flattenScheduling(d *schema.ResourceData, activeWhen *alertsv1.AlertActiveWhen) interface{} {
	scheduling, ok := d.GetOk("scheduling")
	if !ok || activeWhen == nil {
		return nil
	}

	timeZone := scheduling.([]interface{})[0].(map[string]interface{})["time_zone"].(string)

	timeFrames := flattenTimeFrames(activeWhen, timeZone)

	return []interface{}{
		map[string]interface{}{
			"time_zone":   timeZone,
			"time_frames": timeFrames,
		},
	}
}

func flattenTimeFrames(activeWhen *alertsv1.AlertActiveWhen, timeZone string) interface{} {
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

func flattenTimeFrame(tf *alertsv1.AlertActiveTimeframe, utc int32) map[string]interface{} {
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

func getDaysOffsetFromGMT(activityStartGMT *alertsv1.Time, utc int32) int32 {
	daysOffset := int32(activityStartGMT.GetHours()+utc) / 24
	if daysOffset < 0 {
		daysOffset += 7
	}

	return daysOffset
}

func flattenDaysOfWeek(daysOfWeek []alertsv1.DayOfWeek, daysOffset int32) interface{} {
	result := schema.NewSet(schema.HashString, []interface{}{})
	for _, d := range daysOfWeek {
		dayConvertedFromGmtToUtc := alertsv1.DayOfWeek((int32(d) + daysOffset) % 7)
		day := alertProtoDayOfWeekToSchemaDayOfWeek[dayConvertedFromGmtToUtc.String()]
		result.Add(day)
	}
	return result
}

func flattenAlertType(a *alertsv1.Alert) (alertType string, alertSchema interface{}, ignoreInfinity, notifyWhenResolved, notifyOnlyOnTriggeredGroupByValues *wrapperspb.BoolValue) {
	filters := a.GetFilters()
	condition := a.GetCondition().GetCondition()

	switch filters.GetFilterType() {
	case alertsv1.AlertFilters_FILTER_TYPE_TEXT_OR_UNSPECIFIED:
		if _, ok := condition.(*alertsv1.AlertCondition_NewValue); ok {
			alertType = "new_value"
			alertSchema = flattenNewValueAlert(filters, condition)
		} else {
			alertType = "standard"
			alertSchema, notifyWhenResolved, notifyOnlyOnTriggeredGroupByValues = flattenStandardAlert(filters, condition)
		}
	case alertsv1.AlertFilters_FILTER_TYPE_RATIO:
		alertType = "ratio"
		alertSchema, ignoreInfinity, notifyWhenResolved, notifyOnlyOnTriggeredGroupByValues = flattenRatioAlert(filters, condition)
	case alertsv1.AlertFilters_FILTER_TYPE_UNIQUE_COUNT:
		alertType = "unique_count"
		alertSchema = flattenUniqueCountAlert(filters, condition)
	case alertsv1.AlertFilters_FILTER_TYPE_TIME_RELATIVE:
		alertType = "time_relative"
		alertSchema, ignoreInfinity, notifyWhenResolved, notifyOnlyOnTriggeredGroupByValues = flattenTimeRelativeAlert(filters, condition)
	case alertsv1.AlertFilters_FILTER_TYPE_METRIC:
		alertType = "metric"
		alertSchema, notifyWhenResolved, notifyOnlyOnTriggeredGroupByValues = flattenMetricAlert(filters, condition)
	case alertsv1.AlertFilters_FILTER_TYPE_TRACING:
		alertType = "tracing"
		alertSchema, notifyWhenResolved = flattenTracingAlert(filters, condition, a.TracingAlert)
	case alertsv1.AlertFilters_FILTER_TYPE_FLOW:
		alertType = "flow"
		alertSchema = flattenFlowAlert(condition)
	}

	return
}

func flattenNewValueAlert(filters *alertsv1.AlertFilters, condition interface{}) interface{} {
	alertSchema := flattenCommonAlert(filters)
	conditionMap := flattenNewValueCondition(condition)
	alertSchema["condition"] = []interface{}{conditionMap}
	return []interface{}{alertSchema}
}

func flattenNewValueCondition(condition interface{}) interface{} {
	conditionParams := condition.(*alertsv1.AlertCondition_NewValue).NewValue.GetParameters()
	return map[string]interface{}{
		"time_window":  alertProtoNewValueTimeFrameToSchemaTimeFrame[conditionParams.GetTimeframe().String()],
		"key_to_track": conditionParams.GetGroupBy()[0].GetValue(),
	}
}

func flattenStandardAlert(filters *alertsv1.AlertFilters, condition interface{}) (alertSchema interface{}, notifyWhenResolved, notifyOnlyOnTriggeredGroupByValues *wrapperspb.BoolValue) {
	alertSchemaMap := flattenCommonAlert(filters)
	conditionSchema, notifyWhenResolved, notifyOnlyOnTriggeredGroupByValues := flattenStandardCondition(condition)
	alertSchemaMap["condition"] = conditionSchema
	alertSchema = []interface{}{alertSchemaMap}
	return
}

func flattenStandardCondition(condition interface{}) (conditionSchema interface{}, notifyWhenResolved, notifyOnlyOnTriggeredGroupByValues *wrapperspb.BoolValue) {
	var conditionParams *alertsv1.ConditionParameters
	switch condition := condition.(type) {
	case *alertsv1.AlertCondition_Immediate:
		conditionSchema = []interface{}{
			map[string]interface{}{
				"immediately": true,
			},
		}
	case *alertsv1.AlertCondition_LessThan:
		conditionParams = condition.LessThan.GetParameters()
		m := map[string]interface{}{
			"less_than":                true,
			"occurrences_threshold":    int(conditionParams.GetThreshold().GetValue()),
			"group_by":                 wrappedStringSliceToStringSlice(conditionParams.GroupBy),
			"time_window":              alertProtoTimeFrameToSchemaTimeFrame[conditionParams.Timeframe.String()],
			"manage_undetected_values": flattenManageUndetectedValues(conditionParams.GetRelatedExtendedData()),
		}

		conditionSchema = []interface{}{m}
		notifyWhenResolved = conditionParams.GetNotifyOnResolved()
		notifyOnlyOnTriggeredGroupByValues = conditionParams.GetNotifyGroupByOnlyAlerts()
	case *alertsv1.AlertCondition_MoreThan:
		conditionParams = condition.MoreThan.GetParameters()
		conditionSchema = []interface{}{
			map[string]interface{}{
				"more_than":             true,
				"occurrences_threshold": int(conditionParams.GetThreshold().GetValue()),
				"group_by":              wrappedStringSliceToStringSlice(conditionParams.GroupBy),
				"time_window":           alertProtoTimeFrameToSchemaTimeFrame[conditionParams.Timeframe.String()],
			},
		}
		notifyWhenResolved = conditionParams.GetNotifyOnResolved()
		notifyOnlyOnTriggeredGroupByValues = conditionParams.GetNotifyGroupByOnlyAlerts()
	case *alertsv1.AlertCondition_MoreThanUsual:
		conditionParams = condition.MoreThanUsual.GetParameters()
		conditionMap := map[string]interface{}{
			"more_than_usual":       true,
			"occurrences_threshold": int(conditionParams.GetThreshold().GetValue()),
		}
		if groupBy := conditionParams.GetGroupBy(); len(groupBy) > 0 {
			conditionMap["group_by_key"] = groupBy[0].Value
		}
		conditionSchema = []interface{}{
			conditionMap,
		}
	}

	return
}

func flattenManageUndetectedValues(data *alertsv1.RelatedExtendedData) interface{} {
	if data == nil || (data.GetShouldTriggerDeadman() == nil && data.GetShouldTriggerDeadman().GetValue()) {
		return []map[string]interface{}{
			{
				"enable_triggering_on_undetected_values": true,
				"auto_retire_ratio":                      flattenDeadmanRatio(alertsv1.CleanupDeadmanDuration_CLEANUP_DEADMAN_DURATION_NEVER_OR_UNSPECIFIED),
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
			"disable_triggering_on_undetected_values": true,
		},
	}
}

func flattenDeadmanRatio(cleanupDeadmanDuration alertsv1.CleanupDeadmanDuration) string {
	deadmanRatioStr := alertsv1.CleanupDeadmanDuration_name[int32(cleanupDeadmanDuration)]
	deadmanRatio := alertProtoDeadmanRatiosToSchemaDeadmanRatios[deadmanRatioStr]
	return deadmanRatio
}

func flattenRatioAlert(filters *alertsv1.AlertFilters, condition interface{}) (alertSchema interface{}, ignoreInfinity, notifyWhenResolved, notifyOnlyOnTriggeredGroupByValues *wrapperspb.BoolValue) {
	query1Map := flattenCommonAlert(filters)
	query1Map["alias"] = filters.GetAlias().GetValue()
	query2 := filters.GetRatioAlerts()[0]
	query2Map := flattenQuery2ParamsMap(query2)
	conditionMap, ignoreInfinity, notifyWhenResolved, notifyOnlyOnTriggeredGroupByValues := flattenRatioCondition(condition, query2)

	alertSchema = []interface{}{
		map[string]interface{}{
			"query_1":   []interface{}{query1Map},
			"query_2":   []interface{}{query2Map},
			"condition": []interface{}{conditionMap},
		},
	}

	return
}

func flattenRatioCondition(condition interface{}, query2 *alertsv1.AlertFilters_RatioAlert) (ratioParams interface{}, ignoreInfinity, notifyWhenResolved, notifyOnlyOnTriggeredGroupByValues *wrapperspb.BoolValue) {
	var conditionParams *alertsv1.ConditionParameters
	ratioParamsMap := make(map[string]interface{})
	switch condition := condition.(type) {
	case *alertsv1.AlertCondition_LessThan:
		conditionParams = condition.LessThan.GetParameters()
		ratioParamsMap["less_than"] = true
		ratioParamsMap["manage_undetected_values"] = flattenManageUndetectedValues(conditionParams.GetRelatedExtendedData())
	case *alertsv1.AlertCondition_MoreThan:
		conditionParams = condition.MoreThan.GetParameters()
		ratioParamsMap["more_than"] = true
	default:
		return
	}

	ratioParamsMap["queries_ratio"] = conditionParams.GetThreshold().GetValue()
	ratioParamsMap["time_window"] = alertProtoTimeFrameToSchemaTimeFrame[conditionParams.GetTimeframe().String()]

	ignoreInfinity = conditionParams.GetIgnoreInfinity()
	notifyWhenResolved = conditionParams.GetNotifyOnResolved()
	notifyOnlyOnTriggeredGroupByValues = conditionParams.GetNotifyGroupByOnlyAlerts()

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

	ratioParams = ratioParamsMap
	return
}

func flattenQuery2ParamsMap(query2 *alertsv1.AlertFilters_RatioAlert) interface{} {
	return map[string]interface{}{
		"alias":        query2.GetAlias().GetValue(),
		"search_query": query2.GetText().GetValue(),
		"severities":   extractSeverities(query2.GetSeverities()),
		"applications": wrappedStringSliceToStringSlice(query2.GetApplications()),
		"subsystems":   wrappedStringSliceToStringSlice(query2.GetSubsystems()),
	}
}

func flattenUniqueCountAlert(filters *alertsv1.AlertFilters, condition interface{}) interface{} {
	alertSchema := flattenCommonAlert(filters)
	conditionMap := flattenUniqueCountCondition(condition)
	alertSchema["condition"] = []interface{}{conditionMap}
	return []interface{}{alertSchema}
}

func flattenUniqueCountCondition(condition interface{}) interface{} {
	conditionParams := condition.(*alertsv1.AlertCondition_UniqueCount).UniqueCount.GetParameters()
	return map[string]interface{}{
		"unique_count_key":               conditionParams.GetCardinalityFields()[0].GetValue(),
		"max_unique_values":              conditionParams.GetThreshold().GetValue(),
		"time_window":                    alertProtoUniqueCountTimeFrameToSchemaTimeFrame[conditionParams.GetTimeframe().String()],
		"group_by_key":                   conditionParams.GetGroupBy()[0].GetValue(),
		"max_unique_values_for_group_by": conditionParams.GetMaxUniqueCountValuesForGroupByKey().GetValue(),
	}
}

func flattenTimeRelativeAlert(filters *alertsv1.AlertFilters, condition interface{}) (timeRelativeSchema interface{}, ignoreInfinity, notifyWhenResolved, notifyOnlyOnTriggeredGroupByValues *wrapperspb.BoolValue) {
	alertSchema := flattenCommonAlert(filters)
	conditionMap, ignoreInfinity, notifyWhenResolved, notifyOnlyOnTriggeredGroupByValues := flattenTimeRelativeCondition(condition)
	alertSchema["condition"] = []interface{}{conditionMap}
	timeRelativeSchema = []interface{}{alertSchema}
	return
}

func flattenTimeRelativeCondition(condition interface{}) (timeRelativeConditionSchema interface{}, ignoreInfinity, notifyWhenResolved, notifyOnlyOnTriggeredGroupByValues *wrapperspb.BoolValue) {
	var conditionParams *alertsv1.ConditionParameters
	timeRelativeCondition := make(map[string]interface{})
	switch condition := condition.(type) {
	case *alertsv1.AlertCondition_LessThan:
		conditionParams = condition.LessThan.GetParameters()
		timeRelativeCondition["less_than"] = true
		timeRelativeCondition["manage_undetected_values"] = flattenManageUndetectedValues(conditionParams.GetRelatedExtendedData())
	case *alertsv1.AlertCondition_MoreThan:
		conditionParams = condition.MoreThan.GetParameters()
		timeRelativeCondition["more_than"] = true
	default:
		return
	}

	timeRelativeCondition["ratio_threshold"] = int(conditionParams.GetThreshold().GetValue())
	timeRelativeCondition["group_by"] = wrappedStringSliceToStringSlice(conditionParams.GroupBy)
	timeFrame := conditionParams.Timeframe
	relativeTimeFrame := conditionParams.GetRelativeTimeframe()
	timeRelativeCondition["relative_time_window"] = flattenRelativeTimeWindow(timeFrame, relativeTimeFrame)

	ignoreInfinity = conditionParams.GetIgnoreInfinity()
	notifyWhenResolved = conditionParams.GetNotifyOnResolved()
	notifyOnlyOnTriggeredGroupByValues = conditionParams.GetNotifyGroupByOnlyAlerts()
	timeRelativeConditionSchema = timeRelativeCondition

	return
}

func flattenRelativeTimeWindow(timeFrame alertsv1.Timeframe, relativeTimeFrame alertsv1.RelativeTimeframe) string {
	p := protoTimeFrameAndRelativeTimeFrame{timeFrame: timeFrame, relativeTimeFrame: relativeTimeFrame}
	return alertProtoTimeFrameAndRelativeTimeFrameToSchemaRelativeTimeFrame[p]
}

func flattenMetricAlert(filters *alertsv1.AlertFilters, condition interface{}) (metricAlertSchema interface{}, notifyWhenResolved, notifyOnlyOnTriggeredGroupByValues *wrapperspb.BoolValue) {
	var conditionParams *alertsv1.ConditionParameters
	var conditionStr string
	switch condition := condition.(type) {
	case *alertsv1.AlertCondition_LessThan:
		conditionParams = condition.LessThan.GetParameters()
		conditionStr = "less_than"
	case *alertsv1.AlertCondition_MoreThan:
		conditionParams = condition.MoreThan.GetParameters()
		conditionStr = "more_than"
	default:
		return
	}

	var metricTypeStr string
	var searchQuery string
	var conditionMap map[string]interface{}
	promqlParams := conditionParams.GetMetricAlertPromqlParameters()
	if promqlParams != nil {
		metricTypeStr = "promql"
		searchQuery = promqlParams.GetPromqlText().GetValue()
		conditionMap, notifyWhenResolved = flattenPromQLCondition(conditionParams)
		conditionMap["manage_undetected_values"] = flattenManageUndetectedValues(conditionParams.GetRelatedExtendedData())
	} else {
		metricTypeStr = "lucene"
		searchQuery = filters.GetText().GetValue()
		conditionMap, notifyWhenResolved, notifyOnlyOnTriggeredGroupByValues = flattenLuceneCondition(conditionParams)
	}
	conditionMap[conditionStr] = true
	if conditionStr == "less_than" {
		conditionMap["manage_undetected_values"] = flattenManageUndetectedValues(conditionParams.GetRelatedExtendedData())
	}

	metricMap := map[string]interface{}{
		"search_query": searchQuery,
		"condition":    []interface{}{conditionMap},
	}

	metricAlertSchema = []interface{}{
		map[string]interface{}{
			metricTypeStr: []interface{}{metricMap},
		},
	}

	return
}

func flattenPromQLCondition(params *alertsv1.ConditionParameters) (promQLConditionMap map[string]interface{}, notifyWhenResolved *wrapperspb.BoolValue) {
	promqlParams := params.GetMetricAlertPromqlParameters()
	promQLConditionMap =
		map[string]interface{}{
			"threshold":                       params.GetThreshold().GetValue(),
			"time_window":                     alertProtoMetricTimeFrameToMetricSchemaTimeFrame[params.GetTimeframe().String()],
			"sample_threshold_percentage":     promqlParams.GetSampleThresholdPercentage().GetValue(),
			"replace_missing_value_with_zero": promqlParams.GetSwapNullValues().GetValue(),
			"min_non_null_values_percentage":  promqlParams.GetNonNullPercentage().GetValue(),
		}
	notifyWhenResolved = params.GetNotifyOnResolved()
	return
}

func flattenLuceneCondition(params *alertsv1.ConditionParameters) (luceneConditionMap map[string]interface{}, notifyWhenResolved, notifyOnlyOnTriggeredGroupByValues *wrapperspb.BoolValue) {
	metricParams := params.GetMetricAlertParameters()
	luceneConditionMap = map[string]interface{}{
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
	notifyWhenResolved = params.GetNotifyOnResolved()
	notifyOnlyOnTriggeredGroupByValues = params.GetNotifyGroupByOnlyAlerts()
	return
}

func flattenTracingAlert(filters *alertsv1.AlertFilters, condition interface{}, tracingAlert *alertsv1.TracingAlert) (alertSchema interface{}, notifyWhenResolved *wrapperspb.BoolValue) {
	alertMap := flattenCommonAlert(filters)
	conditionSchema, notifyWhenResolved := flattenTracingCondition(condition)
	alertMap["latency_threshold_ms"] = float64(tracingAlert.GetConditionLatency()) / float64(time.Millisecond.Microseconds())
	alertMap["field_filters"] = flattenFiltersData(tracingAlert.GetFieldFilters(), true)
	alertMap["tag_filters"] = flattenFiltersData(tracingAlert.GetTagFilters(), false)
	alertMap["condition"] = conditionSchema
	alertSchema = []interface{}{alertMap}
	return
}

func flattenFlowAlert(condition interface{}) interface{} {
	return []interface{}{flattenFlowAlertsCondition(condition.(*alertsv1.AlertCondition_Flow))}
}

func flattenFlowAlertsCondition(condition *alertsv1.AlertCondition_Flow) interface{} {
	stages := flattenStages(condition.Flow.GetStages())
	return map[string]interface{}{
		"stages": stages,
	}
}

func flattenStages(stages []*alertsv1.FlowStage) []interface{} {
	result := make([]interface{}, 0, len(stages))
	for _, stage := range stages {
		result = append(result, flattenStage(stage))
	}
	return result
}

func flattenStage(stage *alertsv1.FlowStage) interface{} {
	timeMS := int(stage.GetTimeframe().GetMs().GetValue())
	return map[string]interface{}{
		"groups":      flattenGroups(stage.GetGroups()),
		"time_window": flattenTimeframe(timeMS),
	}
}

func flattenGroups(groups []*alertsv1.FlowGroup) []interface{} {
	result := make([]interface{}, 0, len(groups))
	for _, g := range groups {
		result = append(result, flattenGroup(g))
	}
	return result
}

func flattenGroup(g *alertsv1.FlowGroup) interface{} {
	subAlerts := flattenSubAlerts(g.GetAlerts().GetValues())
	operator := g.GetNextOp().String()
	return map[string]interface{}{
		"sub_alerts": subAlerts,
		"operator":   operator,
	}
}

func flattenSubAlerts(subAlerts []*alertsv1.FlowAlert) interface{} {
	result := make([]interface{}, 0, len(subAlerts))
	for _, s := range subAlerts {
		result = append(result, flattenSubAlert(s))
	}
	return result
}

func flattenSubAlert(subAlert *alertsv1.FlowAlert) interface{} {
	return map[string]interface{}{
		"not":           subAlert.GetNot().GetValue(),
		"user_alert_id": subAlert.GetId().GetValue(),
	}
}

func flattenFiltersData(filtersData []*alertsv1.FilterData, isFieldFilters bool) []interface{} {
	result := make([]interface{}, 0, len(filtersData))
	for _, f := range filtersData {
		field := f.GetField()
		if isFieldFilters {
			field = alertProtoTracingFilterFieldToSchemaTracingFilterField[field]
		}
		m := map[string]interface{}{
			"field":   field,
			"filters": flattenFilters(f.GetFilters()),
		}
		result = append(result, m)
	}
	return result
}

func flattenFilters(filters []*alertsv1.Filters) []interface{} {
	result := make([]interface{}, 0, len(filters))
	for _, f := range filters {
		m := map[string]interface{}{
			"values":   f.GetValues(),
			"operator": alertProtoTracingOperatorToSchemaTracingOperator[f.GetOperator()],
		}
		result = append(result, m)
	}
	return result
}

func flattenTracingCondition(condition interface{}) (conditionSchema interface{}, notifyWhenResolved *wrapperspb.BoolValue) {
	switch condition := condition.(type) {
	case *alertsv1.AlertCondition_Immediate:
		conditionSchema = []interface{}{
			map[string]interface{}{
				"immediately": true,
			},
		}
	case *alertsv1.AlertCondition_MoreThan:
		conditionParams := condition.MoreThan.GetParameters()
		conditionSchema = []interface{}{
			map[string]interface{}{
				"more_than":             true,
				"occurrences_threshold": conditionParams.GetThreshold().GetValue(),
				"time_window":           alertProtoTimeFrameToSchemaTimeFrame[conditionParams.GetTimeframe().String()],
				"group_by":              wrappedStringSliceToStringSlice(conditionParams.GetGroupBy()),
			},
		}
		notifyWhenResolved = conditionParams.GetNotifyOnResolved()
	default:
		return
	}
	return
}

func flattenCommonAlert(filters *alertsv1.AlertFilters) map[string]interface{} {
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

func extractSeverities(severities []alertsv1.AlertFilters_LogSeverity) []string {
	result := make([]string, 0, len(severities))
	for _, s := range severities {
		result = append(result, alertProtoLogSeverityToSchemaLogSeverity[s.String()])
	}
	return result
}

func flattenExpirationDate(expiration *alertsv1.Date) []map[string]int {
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

func expandNotification(i interface{}) *notification {
	l := i.([]interface{})
	if len(l) == 0 || l[0] == nil {
		return &notification{}
	}
	raw := l[0]
	m := raw.(map[string]interface{})

	notifyEverySec := wrapperspb.Double(float64(m["notify_every_min"].(int) * 60))
	notifyWhenResolved := wrapperspb.Bool(m["on_trigger_and_resolved"].(bool))
	ignoreInfinity := wrapperspb.Bool(m["ignore_infinity"].(bool))
	notifyOnlyOnTriggeredGroupByValues := wrapperspb.Bool(m["notify_only_on_triggered_group_by_values"].(bool))
	recipients := expandRecipients(m["recipients"])
	payloadFields := interfaceSliceToWrappedStringSlice(m["payload_fields"].(*schema.Set).List())

	return &notification{
		notifyEverySec:                     notifyEverySec,
		notifyWhenResolved:                 notifyWhenResolved,
		notifyOnlyOnTriggeredGroupByValues: notifyOnlyOnTriggeredGroupByValues,
		ignoreInfinity:                     ignoreInfinity,
		recipients:                         recipients,
		payloadFields:                      payloadFields,
	}
}

func expandRecipients(i interface{}) *alertsv1.AlertNotifications {
	l := i.([]interface{})
	if len(l) == 0 || l[0] == nil {
		return &alertsv1.AlertNotifications{}
	}
	raw := l[0]
	m := raw.(map[string]interface{})
	emailRecipients := interfaceSliceToWrappedStringSlice(m["emails"].(*schema.Set).List())
	webhookRecipients := interfaceSliceToWrappedStringSlice(m["webhook_ids"].(*schema.Set).List())
	return &alertsv1.AlertNotifications{
		Emails:       emailRecipients,
		Integrations: webhookRecipients,
	}
}

func expandAlertSeverity(severity string) alertsv1.AlertSeverity {
	severityStr := alertSchemaSeverityToProtoSeverity[severity]
	formatStandardVal := alertsv1.AlertSeverity_value[severityStr]
	return alertsv1.AlertSeverity(formatStandardVal)
}

func expandExpirationDate(v interface{}) *alertsv1.Date {
	l := v.([]interface{})
	if len(l) == 0 || l[0] == nil {
		return nil
	}
	raw := l[0]
	m := raw.(map[string]interface{})
	return &alertsv1.Date{
		Year:  int32(m["year"].(int)),
		Month: int32(m["month"].(int)),
		Day:   int32(m["day"].(int)),
	}
}

func extractMetaLabels(v interface{}) []*alertsv1.MetaLabel {
	v = v.(*schema.Set).List()
	l := v.([]interface{})
	result := make([]*alertsv1.MetaLabel, 0, len(l))
	for _, s := range l {
		ml := expandMetaLabel(s)
		result = append(result, ml)
	}
	return result
}

func expandMetaLabel(v interface{}) *alertsv1.MetaLabel {
	m := v.(map[string]interface{})
	key := wrapperspb.String(m["key"].(string))
	value := wrapperspb.String(m["value"].(string))
	return &alertsv1.MetaLabel{
		Key:   key,
		Value: value,
	}
}

func expandActiveWhen(v interface{}) *alertsv1.AlertActiveWhen {
	l := v.([]interface{})
	if len(l) == 0 {
		return nil
	}

	schedulingMap := l[0].(map[string]interface{})
	utc := flattenUtc(schedulingMap["time_zone"].(string))
	timeFrames := schedulingMap["time_frames"].(*schema.Set).List()

	expandedTimeframes := expandActiveTimeframes(timeFrames, utc)

	return &alertsv1.AlertActiveWhen{
		Timeframes: expandedTimeframes,
	}
}

func expandActiveTimeframes(timeFrames []interface{}, utc int32) []*alertsv1.AlertActiveTimeframe {
	result := make([]*alertsv1.AlertActiveTimeframe, 0, len(timeFrames))
	for _, tf := range timeFrames {
		alertActiveTimeframe := expandActiveTimeFrame(tf, utc)
		result = append(result, alertActiveTimeframe)
	}
	return result
}

func expandActiveTimeFrame(timeFrame interface{}, utc int32) *alertsv1.AlertActiveTimeframe {
	m := timeFrame.(map[string]interface{})
	daysOfWeek := expandDaysOfWeek(m["days_enabled"])
	frameRange := expandRange(m["start_time"], m["end_time"])
	frameRange, daysOfWeek = convertTimeFramesToGMT(frameRange, daysOfWeek, utc)

	alertActiveTimeframe := &alertsv1.AlertActiveTimeframe{
		DaysOfWeek: daysOfWeek,
		Range:      frameRange,
	}
	return alertActiveTimeframe
}

func convertTimeFramesToGMT(frameRange *alertsv1.TimeRange, daysOfWeek []alertsv1.DayOfWeek, utc int32) (*alertsv1.TimeRange, []alertsv1.DayOfWeek) {
	daysOfWeekOffset := daysOfWeekOffsetToGMT(frameRange, utc)
	frameRange.Start.Hours = convertUtcToGmt(frameRange.GetStart().GetHours(), utc)
	frameRange.End.Hours = convertUtcToGmt(frameRange.GetEnd().GetHours(), utc)
	if daysOfWeekOffset != 0 {
		for i, d := range daysOfWeek {
			daysOfWeek[i] = alertsv1.DayOfWeek((int32(d) + daysOfWeekOffset) % 7)
		}
	}

	return frameRange, daysOfWeek
}

func daysOfWeekOffsetToGMT(frameRange *alertsv1.TimeRange, utc int32) int32 {
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

func expandDaysOfWeek(v interface{}) []alertsv1.DayOfWeek {
	l := v.(*schema.Set).List()
	result := make([]alertsv1.DayOfWeek, 0, len(l))
	for _, v := range l {
		dayOfWeekStr := alertSchemaDayOfWeekToProtoDayOfWeek[v.(string)]
		dayOfWeekVal := alertsv1.DayOfWeek_value[dayOfWeekStr]
		result = append(result, alertsv1.DayOfWeek(dayOfWeekVal))
	}
	return result
}

func expandRange(activityStarts, activityEnds interface{}) *alertsv1.TimeRange {
	start := expandTimeInDay(activityStarts)
	end := expandTimeInDay(activityEnds)

	return &alertsv1.TimeRange{
		Start: start,
		End:   end,
	}
}

func expandAlertType(d *schema.ResourceData, notification *notification) (alertTypeParams *alertParams, tracingAlert *alertsv1.TracingAlert, err error) {
	alertTypeStr := From(validAlertTypes).FirstWith(func(key interface{}) bool {
		return len(d.Get(key.(string)).([]interface{})) > 0
	}).(string)

	alertType := d.Get(alertTypeStr).([]interface{})[0].(map[string]interface{})

	switch alertTypeStr {
	case "standard":
		alertTypeParams, err = expandStandard(alertType, notification.notifyWhenResolved, notification.notifyOnlyOnTriggeredGroupByValues)
	case "ratio":
		alertTypeParams, err = expandRatio(alertType, notification.ignoreInfinity, notification.notifyWhenResolved, notification.notifyOnlyOnTriggeredGroupByValues)
	case "new_value":
		alertTypeParams = expandNewValue(alertType)
	case "unique_count":
		alertTypeParams = expandUniqueCount(alertType)
	case "time_relative":
		alertTypeParams, err = expandTimeRelative(alertType, notification.ignoreInfinity, notification.notifyWhenResolved, notification.notifyOnlyOnTriggeredGroupByValues)
	case "metric":
		alertTypeParams, err = expandMetric(alertType, notification.notifyWhenResolved, notification.notifyOnlyOnTriggeredGroupByValues)
	case "tracing":
		alertTypeParams, tracingAlert = expandTracing(alertType, notification.notifyWhenResolved)
	case "flow":
		alertTypeParams = expandFlow(alertType)
	}

	return
}

func expandStandard(m map[string]interface{}, notifyOnResolved, notifyOnlyOnTriggeredGroupByValues *wrapperspb.BoolValue) (*alertParams, error) {
	conditionMap := extractConditionMap(m)
	condition, err := expandStandardCondition(conditionMap, notifyOnResolved, notifyOnlyOnTriggeredGroupByValues)
	if err != nil {
		return nil, err
	}
	filters := expandStandardFilter(m)
	return &alertParams{
		Condition: condition,
		Filters:   filters,
	}, nil
}

func expandStandardCondition(m map[string]interface{}, notifyOnResolved, notifyOnlyOnTriggeredGroupByValues *wrapperspb.BoolValue) (*alertsv1.AlertCondition, error) {
	if immediately := m["immediately"]; immediately != nil && immediately.(bool) {
		return &alertsv1.AlertCondition{
			Condition: &alertsv1.AlertCondition_Immediate{},
		}, nil
	} else if moreThenUsual := m["more_than_usual"]; moreThenUsual != nil && moreThenUsual.(bool) {
		threshold := wrapperspb.Double(float64(m["occurrences_threshold"].(int)))
		groupBy := []*wrapperspb.StringValue{wrapperspb.String(m["group_by_key"].(string))}
		parameters := &alertsv1.ConditionParameters{
			Threshold: threshold,
			GroupBy:   groupBy,
		}
		return &alertsv1.AlertCondition{
			Condition: &alertsv1.AlertCondition_MoreThanUsual{
				MoreThanUsual: &alertsv1.MoreThanUsualCondition{Parameters: parameters},
			},
		}, nil
	} else {
		parameters := expandStandardConditionParameters(m, notifyOnResolved, notifyOnlyOnTriggeredGroupByValues)
		if lessThan := m["less_than"]; lessThan != nil && lessThan.(bool) {
			return &alertsv1.AlertCondition{
				Condition: &alertsv1.AlertCondition_LessThan{
					LessThan: &alertsv1.LessThanCondition{Parameters: parameters},
				},
			}, nil
		} else if moreThan := m["more_than"]; moreThan != nil && moreThan.(bool) {
			return &alertsv1.AlertCondition{
				Condition: &alertsv1.AlertCondition_MoreThan{
					MoreThan: &alertsv1.MoreThanCondition{Parameters: parameters},
				},
			}, nil
		}
	}

	return nil, fmt.Errorf("immediately, less_than, more_than or more_than_usual have to be true")
}

func expandRelatedExtendedData(m map[string]interface{}) *alertsv1.RelatedExtendedData {
	if v, ok := m["manage_undetected_values"]; ok {
		if manageUndetectedValues, ok := v.([]interface{}); ok && len(manageUndetectedValues) != 0 {
			raw := manageUndetectedValues[0].(map[string]interface{})
			if enable, ok := raw["enable_triggering_on_undetected_values"]; ok && enable.(bool) {
				cleanupDeadmanDurationStr := alertSchemaDeadmanRatiosToProtoDeadmanRatios[raw["auto_retire_ratio"].(string)]
				cleanupDeadmanDuration := alertsv1.CleanupDeadmanDuration(alertsv1.CleanupDeadmanDuration_value[cleanupDeadmanDurationStr])
				return &alertsv1.RelatedExtendedData{
					CleanupDeadmanDuration: &cleanupDeadmanDuration,
					ShouldTriggerDeadman:   wrapperspb.Bool(true),
				}
			} else if disable, ok := raw["disable_triggering_on_undetected_values"]; ok && disable.(bool) {
				return &alertsv1.RelatedExtendedData{
					ShouldTriggerDeadman: wrapperspb.Bool(false),
				}
			}

		}
	}

	return nil
}

func expandStandardConditionParameters(m map[string]interface{}, notifyOnResolved, notifyOnlyOnTriggeredGroupByValues *wrapperspb.BoolValue) *alertsv1.ConditionParameters {
	timeFrame := expandTimeFrame(m["time_window"].(string))
	groupBy := interfaceSliceToWrappedStringSlice(m["group_by"].([]interface{}))
	threshold := wrapperspb.Double(float64(m["occurrences_threshold"].(int)))
	relatedExtendedData := expandRelatedExtendedData(m)

	return &alertsv1.ConditionParameters{
		Threshold:               threshold,
		Timeframe:               timeFrame,
		GroupBy:                 groupBy,
		NotifyOnResolved:        notifyOnResolved,
		NotifyGroupByOnlyAlerts: notifyOnlyOnTriggeredGroupByValues,
		RelatedExtendedData:     relatedExtendedData,
	}
}

func expandStandardFilter(m map[string]interface{}) *alertsv1.AlertFilters {
	filters := expandCommonAlertFilter(m)
	filters.FilterType = alertsv1.AlertFilters_FILTER_TYPE_TEXT_OR_UNSPECIFIED
	return filters
}

func expandRatio(m map[string]interface{}, ignoreInfinity, notifyOnResolved, notifyOnlyOnTriggeredGroupByValues *wrapperspb.BoolValue) (*alertParams, error) {
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
			return nil, fmt.Errorf("group_by is required with one of - group_by_q1/group_by_q1/group_by_both")
		}
	}

	condition, err := expandRatioCondition(conditionMap, groupByQ1, ignoreInfinity, notifyOnResolved, notifyOnlyOnTriggeredGroupByValues)
	if err != nil {
		return nil, err
	}
	filters := expandRatioFilters(m, groupByQ2)

	return &alertParams{
		Condition: condition,
		Filters:   filters,
	}, nil
}

func expandRatioFilters(m map[string]interface{}, groupBy []*wrapperspb.StringValue) *alertsv1.AlertFilters {
	query1 := m["query_1"].([]interface{})[0].(map[string]interface{})
	filters := expandCommonAlertFilter(query1)
	filters.FilterType = alertsv1.AlertFilters_FILTER_TYPE_RATIO
	filters.Alias = wrapperspb.String(query1["alias"].(string))
	query2 := expandQuery2(m["query_2"], groupBy)
	filters.RatioAlerts = []*alertsv1.AlertFilters_RatioAlert{query2}
	return filters
}

func expandRatioCondition(m map[string]interface{}, groupBy []*wrapperspb.StringValue, ignoreInfinity, notifyOnResolved, notifyOnlyOnTriggeredGroupByValues *wrapperspb.BoolValue) (*alertsv1.AlertCondition, error) {
	parameters := expandRatioParams(m, groupBy, ignoreInfinity, notifyOnResolved, notifyOnlyOnTriggeredGroupByValues)
	return expandLessThanOrMoreThanAlertCondition(m, parameters)
}

func expandRatioParams(m map[string]interface{}, groupBy []*wrapperspb.StringValue, ignoreInfinity, notifyOnResolved, notifyOnlyOnTriggeredGroupByValues *wrapperspb.BoolValue) *alertsv1.ConditionParameters {
	threshold := wrapperspb.Double(m["queries_ratio"].(float64))
	timeFrame := expandTimeFrame(m["time_window"].(string))
	relatedExtendedData := expandRelatedExtendedData(m)

	return &alertsv1.ConditionParameters{
		Threshold:               threshold,
		Timeframe:               timeFrame,
		GroupBy:                 groupBy,
		NotifyOnResolved:        notifyOnResolved,
		IgnoreInfinity:          ignoreInfinity,
		NotifyGroupByOnlyAlerts: notifyOnlyOnTriggeredGroupByValues,
		RelatedExtendedData:     relatedExtendedData,
	}
}

func expandQuery2(v interface{}, groupBy []*wrapperspb.StringValue) *alertsv1.AlertFilters_RatioAlert {
	m := v.([]interface{})[0].(map[string]interface{})
	alias := wrapperspb.String(m["alias"].(string))
	text := wrapperspb.String(m["search_query"].(string))
	severities := expandAlertFiltersSeverities(m["severities"])
	applications := interfaceSliceToWrappedStringSlice(m["applications"].([]interface{}))
	subsystems := interfaceSliceToWrappedStringSlice(m["subsystems"].([]interface{}))
	return &alertsv1.AlertFilters_RatioAlert{
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

func expandNewValueCondition(m map[string]interface{}) *alertsv1.AlertCondition {
	parameters := expandNewValueConditionParameters(m)
	condition := &alertsv1.AlertCondition{
		Condition: &alertsv1.AlertCondition_NewValue{
			NewValue: &alertsv1.NewValueCondition{
				Parameters: parameters,
			},
		},
	}
	return condition
}

func expandNewValueConditionParameters(m map[string]interface{}) *alertsv1.ConditionParameters {
	timeFrame := expandNewValueTimeFrame(m["time_window"].(string))
	groupBy := []*wrapperspb.StringValue{wrapperspb.String(m["key_to_track"].(string))}
	parameters := &alertsv1.ConditionParameters{
		Timeframe: timeFrame,
		GroupBy:   groupBy,
	}
	return parameters
}

func expandNewValueFilters(m map[string]interface{}) *alertsv1.AlertFilters {
	filters := expandCommonAlertFilter(m)
	filters.FilterType = alertsv1.AlertFilters_FILTER_TYPE_TEXT_OR_UNSPECIFIED
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

func expandUniqueCountCondition(m map[string]interface{}) *alertsv1.AlertCondition {
	parameters := expandUniqueCountConditionParameters(m)
	return &alertsv1.AlertCondition{
		Condition: &alertsv1.AlertCondition_UniqueCount{
			UniqueCount: &alertsv1.UniqueCountCondition{
				Parameters: parameters,
			},
		},
	}
}

func expandUniqueCountConditionParameters(m map[string]interface{}) *alertsv1.ConditionParameters {
	uniqueCountKey := []*wrapperspb.StringValue{wrapperspb.String(m["unique_count_key"].(string))}
	threshold := wrapperspb.Double(float64(m["max_unique_values"].(int)))
	timeFrame := expandUniqueValueTimeFrame(m["time_window"].(string))
	groupBy := []*wrapperspb.StringValue{wrapperspb.String(m["group_by_key"].(string))}
	groupByThreshold := wrapperspb.UInt32(uint32(m["max_unique_values_for_group_by"].(int)))

	return &alertsv1.ConditionParameters{
		CardinalityFields:                 uniqueCountKey,
		Threshold:                         threshold,
		Timeframe:                         timeFrame,
		GroupBy:                           groupBy,
		MaxUniqueCountValuesForGroupByKey: groupByThreshold,
	}
}

func expandUniqueCountFilters(m map[string]interface{}) *alertsv1.AlertFilters {
	filters := expandCommonAlertFilter(m)
	filters.FilterType = alertsv1.AlertFilters_FILTER_TYPE_UNIQUE_COUNT
	return filters
}

func expandCommonAlertFilter(m map[string]interface{}) *alertsv1.AlertFilters {
	severities := expandAlertFiltersSeverities(m["severities"].(*schema.Set).List())
	metadata := expandMetadata(m)
	text := wrapperspb.String(m["search_query"].(string))

	return &alertsv1.AlertFilters{
		Severities: severities,
		Metadata:   metadata,
		Text:       text,
	}
}

func expandTimeRelative(m map[string]interface{}, ignoreInfinity, notifyOnResolved, notifyOnlyOnTriggeredGroupByValues *wrapperspb.BoolValue) (*alertParams, error) {
	conditionMap := extractConditionMap(m)
	condition, err := expandTimeRelativeCondition(conditionMap, ignoreInfinity, notifyOnResolved, notifyOnlyOnTriggeredGroupByValues)
	if err != nil {
		return nil, err
	}
	filters := expandTimeRelativeFilters(m)

	return &alertParams{
		Condition: condition,
		Filters:   filters,
	}, nil
}

func expandTimeRelativeCondition(m map[string]interface{}, ignoreInfinity, notifyOnResolved, notifyOnlyOnTriggeredGroupByValues *wrapperspb.BoolValue) (*alertsv1.AlertCondition, error) {
	parameters := expandTimeRelativeConditionParameters(m, ignoreInfinity, notifyOnResolved, notifyOnlyOnTriggeredGroupByValues)
	return expandLessThanOrMoreThanAlertCondition(m, parameters)
}

func expandLessThanOrMoreThanAlertCondition(
	m map[string]interface{}, parameters *alertsv1.ConditionParameters) (*alertsv1.AlertCondition, error) {
	lessThan, err := trueIfIsLessThanFalseIfMoreThanAndErrorOtherwise(m)
	if err != nil {
		return nil, err
	}

	if lessThan {
		return &alertsv1.AlertCondition{
			Condition: &alertsv1.AlertCondition_LessThan{
				LessThan: &alertsv1.LessThanCondition{Parameters: parameters},
			},
		}, nil
	}

	return &alertsv1.AlertCondition{
		Condition: &alertsv1.AlertCondition_MoreThan{
			MoreThan: &alertsv1.MoreThanCondition{Parameters: parameters},
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

func expandTimeRelativeConditionParameters(m map[string]interface{}, ignoreInfinity, notifyOnResolved, notifyOnlyOnTriggeredGroupByValues *wrapperspb.BoolValue) *alertsv1.ConditionParameters {
	timeFrame, relativeTimeframe := expandTimeFrameAndRelativeTimeframe(m["relative_time_window"].(string))
	groupBy := interfaceSliceToWrappedStringSlice(m["group_by"].([]interface{}))
	threshold := wrapperspb.Double(m["ratio_threshold"].(float64))
	relatedExtendedData := expandRelatedExtendedData(m)
	return &alertsv1.ConditionParameters{
		Timeframe:               timeFrame,
		RelativeTimeframe:       relativeTimeframe,
		GroupBy:                 groupBy,
		Threshold:               threshold,
		IgnoreInfinity:          ignoreInfinity,
		NotifyOnResolved:        notifyOnResolved,
		NotifyGroupByOnlyAlerts: notifyOnlyOnTriggeredGroupByValues,
		RelatedExtendedData:     relatedExtendedData,
	}
}

func expandTimeFrameAndRelativeTimeframe(relativeTimeframeStr string) (alertsv1.Timeframe, alertsv1.RelativeTimeframe) {
	p := alertSchemaRelativeTimeFrameToProtoTimeFrameAndRelativeTimeFrame[relativeTimeframeStr]
	return p.timeFrame, p.relativeTimeFrame
}

func expandTimeRelativeFilters(m map[string]interface{}) *alertsv1.AlertFilters {
	filters := expandCommonAlertFilter(m)
	filters.FilterType = alertsv1.AlertFilters_FILTER_TYPE_TIME_RELATIVE
	return filters
}

func expandMetric(m map[string]interface{}, notifyWhenResolved, notifyOnlyOnTriggeredGroupByValues *wrapperspb.BoolValue) (*alertParams, error) {
	condition, err := expandMetricCondition(m, notifyWhenResolved, notifyOnlyOnTriggeredGroupByValues)
	if err != nil {
		return nil, err
	}
	filters := expandMetricFilters(m)

	return &alertParams{
		Condition: condition,
		Filters:   filters,
	}, nil
}

func expandMetricCondition(m map[string]interface{}, notifyWhenResolved, notifyOnlyOnTriggeredGroupByValues *wrapperspb.BoolValue) (*alertsv1.AlertCondition, error) {
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
	relatedExtendedData := expandRelatedExtendedData(conditionMap)

	parameters := &alertsv1.ConditionParameters{
		Threshold:               threshold,
		NotifyOnResolved:        notifyWhenResolved,
		NotifyGroupByOnlyAlerts: notifyOnlyOnTriggeredGroupByValues,
		Timeframe:               timeFrame,
		RelatedExtendedData:     relatedExtendedData,
	}

	if isPromQL {
		parameters.MetricAlertPromqlParameters = &alertsv1.MetricAlertPromqlConditionParameters{
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
		parameters.MetricAlertParameters = &alertsv1.MetricAlertConditionParameters{
			MetricSource:               alertsv1.MetricAlertConditionParameters_METRIC_SOURCE_LOGS2METRICS_OR_UNSPECIFIED,
			MetricField:                metricField,
			ArithmeticOperator:         arithmeticOperator,
			ArithmeticOperatorModifier: arithmeticOperatorModifier,
			SampleThresholdPercentage:  sampleThresholdPercentage,
			NonNullPercentage:          nonNullPercentage,
			SwapNullValues:             swapNullValues,
		}
	}

	return expandLessThanOrMoreThanAlertCondition(conditionMap, parameters)
}

func expandArithmeticOperator(s string) alertsv1.MetricAlertConditionParameters_ArithmeticOperator {
	arithmeticStr := alertSchemaArithmeticOperatorToProtoArithmetic[s]
	arithmeticValue := alertsv1.MetricAlertConditionParameters_ArithmeticOperator_value[arithmeticStr]
	return alertsv1.MetricAlertConditionParameters_ArithmeticOperator(arithmeticValue)
}

func expandMetricFilters(m map[string]interface{}) *alertsv1.AlertFilters {
	var text *wrapperspb.StringValue
	if len(m["promql"].([]interface{})) == 0 {
		luceneArr := m["lucene"].([]interface{})
		lucene := luceneArr[0].(map[string]interface{})
		text = wrapperspb.String(lucene["search_query"].(string))
	}

	return &alertsv1.AlertFilters{
		FilterType: alertsv1.AlertFilters_FILTER_TYPE_METRIC,
		Text:       text,
	}
}

func expandFlow(m map[string]interface{}) *alertParams {
	stages := expandFlowStages(m["stages"])
	return &alertParams{
		Condition: &alertsv1.AlertCondition{
			Condition: &alertsv1.AlertCondition_Flow{
				Flow: &alertsv1.FlowCondition{
					Stages: stages,
				},
			},
		},
		Filters: &alertsv1.AlertFilters{
			FilterType: alertsv1.AlertFilters_FILTER_TYPE_FLOW,
		},
	}
}

func expandFlowStages(i interface{}) []*alertsv1.FlowStage {
	l := i.([]interface{})
	result := make([]*alertsv1.FlowStage, 0, len(l))
	for _, v := range l {
		stage := expandFlowStage(v)
		result = append(result, stage)
	}

	return result
}

func expandFlowStage(i interface{}) *alertsv1.FlowStage {
	m := i.(map[string]interface{})
	groups := expandGroups(m["groups"])
	timeFrame := expandFlowTimeFrame(m["time_window"])
	return &alertsv1.FlowStage{Groups: groups, Timeframe: timeFrame}
}

func expandGroups(i interface{}) []*alertsv1.FlowGroup {
	l := i.([]interface{})
	result := make([]*alertsv1.FlowGroup, 0, len(l))
	for _, v := range l {
		group := expandFlowGroup(v)
		result = append(result, group)
	}

	return result
}

func expandFlowGroup(i interface{}) *alertsv1.FlowGroup {
	m := i.(map[string]interface{})
	alerts := expandSubAlerts(m["sub_alerts"])
	operator := expandOperator(m["operator"])
	return &alertsv1.FlowGroup{
		Alerts: alerts,
		NextOp: operator,
	}
}

func expandSubAlerts(i interface{}) *alertsv1.FlowAlerts {
	l := i.([]interface{})
	result := make([]*alertsv1.FlowAlert, 0, len(l))
	for _, v := range l {
		subAlert := expandSubAlert(v)
		result = append(result, subAlert)
	}

	return &alertsv1.FlowAlerts{
		Values: result,
	}
}

func expandSubAlert(i interface{}) *alertsv1.FlowAlert {
	m := i.(map[string]interface{})
	return &alertsv1.FlowAlert{
		Id:  wrapperspb.String(m["user_alert_id"].(string)),
		Not: wrapperspb.Bool(m["not"].(bool)),
	}
}

func expandOperator(i interface{}) alertsv1.FlowOperator {
	operatorStr := i.(string)
	return alertsv1.FlowOperator(alertsv1.FlowOperator_value[operatorStr])
}

func expandFlowTimeFrame(i interface{}) *alertsv1.FlowTimeframe {
	return &alertsv1.FlowTimeframe{
		Ms: wrapperspb.UInt32(uint32(expandTimeToMS(i))),
	}
}

func expandTracing(m map[string]interface{}, notifyOnResolved *wrapperspb.BoolValue) (*alertParams, *alertsv1.TracingAlert) {
	tracingParams, _ := expandTracingParams(m, notifyOnResolved)
	tracingAlert := expandTracingAlert(m)

	return tracingParams, tracingAlert
}

func expandTracingParams(m map[string]interface{}, notifyOnResolved *wrapperspb.BoolValue) (*alertParams, error) {
	conditionMap := extractConditionMap(m)
	condition, err := expandTracingCondition(conditionMap, notifyOnResolved)
	if err != nil {
		return nil, err
	}
	filters := expandTracingFilter(m)
	return &alertParams{
		Condition: condition,
		Filters:   filters,
	}, nil
}

func expandTracingCondition(m map[string]interface{}, notifyOnResolved *wrapperspb.BoolValue) (*alertsv1.AlertCondition, error) {
	if immediately := m["immediately"]; immediately != nil && immediately.(bool) {
		return &alertsv1.AlertCondition{
			Condition: &alertsv1.AlertCondition_Immediate{},
		}, nil
	} else if moreThan := m["more_than"]; moreThan != nil && moreThan.(bool) {
		parameters := expandStandardConditionParameters(m, notifyOnResolved, nil)
		return &alertsv1.AlertCondition{
			Condition: &alertsv1.AlertCondition_MoreThan{
				MoreThan: &alertsv1.MoreThanCondition{Parameters: parameters},
			},
		}, nil
	}

	return nil, fmt.Errorf("immediately, less_than, more_than or more_than_usual have to be true")
}

func expandTracingFilter(m map[string]interface{}) *alertsv1.AlertFilters {
	filters := expandCommonAlertFilter(m)
	filters.FilterType = alertsv1.AlertFilters_FILTER_TYPE_TRACING
	return filters
}

func expandTracingAlert(m map[string]interface{}) *alertsv1.TracingAlert {
	conditionLatency := uint32(m["latency_threshold_ms"].(float64) * (float64)(time.Millisecond.Microseconds()))
	fieldFilters := expandFiltersData(m["field_filters"], true)
	tagFilters := expandFiltersData(m["tag_filters"], false)
	return &alertsv1.TracingAlert{
		ConditionLatency: conditionLatency,
		FieldFilters:     fieldFilters,
		TagFilters:       tagFilters,
	}
}

func expandFiltersData(i interface{}, isFieldFilters bool) []*alertsv1.FilterData {
	l := i.([]interface{})
	result := make([]*alertsv1.FilterData, 0, len(l))
	for _, v := range l {
		m := v.(map[string]interface{})
		field := m["field"].(string)
		if isFieldFilters {
			field = alertSchemaTracingFilterFieldToProtoTracingFilterField[field]
		}
		filters := expandFilter(m["filters"])
		fd := &alertsv1.FilterData{
			Field:   field,
			Filters: filters,
		}
		result = append(result, fd)
	}
	return result
}

func expandFilter(i interface{}) []*alertsv1.Filters {
	l := i.([]interface{})
	result := make([]*alertsv1.Filters, 0, len(l))
	for _, v := range l {
		m := v.(map[string]interface{})
		fd := &alertsv1.Filters{
			Values:   interfaceSliceToStringSlice(m["values"].([]interface{})),
			Operator: alertSchemaTracingOperatorToProtoTracingOperator[m["operator"].(string)],
		}
		result = append(result, fd)
	}
	return result
}

func extractConditionMap(m map[string]interface{}) map[string]interface{} {
	return m["condition"].([]interface{})[0].(map[string]interface{})
}

func expandTimeFrame(s string) alertsv1.Timeframe {
	protoTimeFrame := alertSchemaTimeFrameToProtoTimeFrame[s]
	return alertsv1.Timeframe(alertsv1.Timeframe_value[protoTimeFrame])
}

func expandMetricTimeFrame(s string) alertsv1.Timeframe {
	protoTimeFrame := alertSchemaMetricTimeFrameToMetricProtoTimeFrame[s]
	return alertsv1.Timeframe(alertsv1.Timeframe_value[protoTimeFrame])
}

func expandMetadata(m map[string]interface{}) *alertsv1.AlertFilters_MetadataFilters {
	categories := interfaceSliceToWrappedStringSlice(m["categories"].(*schema.Set).List())
	applications := interfaceSliceToWrappedStringSlice(m["applications"].(*schema.Set).List())
	subsystems := interfaceSliceToWrappedStringSlice(m["subsystems"].(*schema.Set).List())
	computers := interfaceSliceToWrappedStringSlice(m["computers"].(*schema.Set).List())
	classes := interfaceSliceToWrappedStringSlice(m["classes"].(*schema.Set).List())
	methods := interfaceSliceToWrappedStringSlice(m["methods"].(*schema.Set).List())
	ipAddresses := interfaceSliceToWrappedStringSlice(m["ip_addresses"].(*schema.Set).List())

	return &alertsv1.AlertFilters_MetadataFilters{
		Categories:   categories,
		Applications: applications,
		Subsystems:   subsystems,
		Computers:    computers,
		Classes:      classes,
		Methods:      methods,
		IpAddresses:  ipAddresses,
	}
}

func expandAlertFiltersSeverities(v interface{}) []alertsv1.AlertFilters_LogSeverity {
	s := interfaceSliceToStringSlice(v.([]interface{}))
	result := make([]alertsv1.AlertFilters_LogSeverity, 0, len(s))
	for _, v := range s {
		logSeverityStr := alertSchemaLogSeverityToProtoLogSeverity[v]
		result = append(result, alertsv1.AlertFilters_LogSeverity(
			alertsv1.AlertFilters_LogSeverity_value[logSeverityStr]))
	}

	return result
}

func expandNewValueTimeFrame(s string) alertsv1.Timeframe {
	protoTimeFrame := alertSchemaNewValueTimeFrameToProtoTimeFrame[s]
	return alertsv1.Timeframe(alertsv1.Timeframe_value[protoTimeFrame])
}

func expandUniqueValueTimeFrame(s string) alertsv1.Timeframe {
	protoTimeFrame := alertSchemaUniqueCountTimeFrameToProtoTimeFrame[s]
	return alertsv1.Timeframe(alertsv1.Timeframe_value[protoTimeFrame])
}
