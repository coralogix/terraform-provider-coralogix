package coralogix

import (
	"context"
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/protobuf/encoding/protojson"
	"terraform-provider-coralogix/coralogix/clientset"
	alerts "terraform-provider-coralogix/coralogix/clientset/grpc/alerts/v3"

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
	_                             resource.ResourceWithConfigure   = &AlertResource{}
	_                             resource.ResourceWithImportState = &AlertResource{}
	createAlertURL                                                 = "com.coralogixapis.alerts.v3.AlertsService/CreateAlert"
	updateAlertURL                                                 = "com.coralogixapis.alerts.v3.AlertsService/ReplaceAlert"
	getAlertURL                                                    = "com.coralogixapis.alerts.v3.AlertsService/GetAlert"
	deleteAlertURL                                                 = "com.coralogixapis.alerts.v3.AlertsService/DeleteAlert"
	alertPriorityProtoToSchemaMap                                  = map[alerts.AlertPriority]string{
		alerts.AlertPriority_ALERT_PRIORITY_P5_OR_UNSPECIFIED: "P5",
		alerts.AlertPriority_ALERT_PRIORITY_P4:                "P4",
		alerts.AlertPriority_ALERT_PRIORITY_P3:                "P3",
		alerts.AlertPriority_ALERT_PRIORITY_P2:                "P2",
		alerts.AlertPriority_ALERT_PRIORITY_P1:                "P1",
	}
	alertPrioritySchemaToProtoMap = ReverseMap(alertPriorityProtoToSchemaMap)
	validAlertPriorities          = GetKeys(alertPrioritySchemaToProtoMap)
	notifyOnProtoToSchemaMap      = map[alerts.NotifyOn]string{
		alerts.NotifyOn_NOTIFY_ON_TRIGGERED_ONLY_UNSPECIFIED: "Triggered Only",
		alerts.NotifyOn_NOTIFY_ON_TRIGGERED_AND_RESOLVED:     "Triggered and Resolved",
	}
	notifyOnSchemaToProtoMap   = ReverseMap(notifyOnProtoToSchemaMap)
	validNotifyOn              = GetKeys(notifyOnSchemaToProtoMap)
	daysOfWeekProtoToSchemaMap = map[alerts.DayOfWeek]string{
		alerts.DayOfWeek_DAY_OF_WEEK_MONDAY_OR_UNSPECIFIED: "Monday",
		alerts.DayOfWeek_DAY_OF_WEEK_TUESDAY:               "Tuesday",
		alerts.DayOfWeek_DAY_OF_WEEK_WEDNESDAY:             "Wednesday",
		alerts.DayOfWeek_DAY_OF_WEEK_THURSDAY:              "Thursday",
		alerts.DayOfWeek_DAY_OF_WEEK_FRIDAY:                "Friday",
		alerts.DayOfWeek_DAY_OF_WEEK_SATURDAY:              "Saturday",
		alerts.DayOfWeek_DAY_OF_WEEK_SUNDAY:                "Sunday",
	}
	daysOfWeekSchemaToProtoMap             = ReverseMap(daysOfWeekProtoToSchemaMap)
	validDaysOfWeek                        = GetKeys(daysOfWeekSchemaToProtoMap)
	logFilterOperationTypeProtoToSchemaMap = map[alerts.LogFilterOperationType]string{
		alerts.LogFilterOperationType_LOG_FILTER_OPERATION_TYPE_IS_OR_UNSPECIFIED: "OR",
		alerts.LogFilterOperationType_LOG_FILTER_OPERATION_TYPE_INCLUDES:          "NOT",
		alerts.LogFilterOperationType_LOG_FILTER_OPERATION_TYPE_ENDS_WITH:         "ENDS_WITH",
		alerts.LogFilterOperationType_LOG_FILTER_OPERATION_TYPE_STARTS_WITH:       "STARTS_WITH",
	}
	logFilterOperationTypeSchemaToProtoMap = ReverseMap(logFilterOperationTypeProtoToSchemaMap)
	validLogFilterOperationType            = GetKeys(logFilterOperationTypeSchemaToProtoMap)
	logSeverityProtoToSchemaMap            = map[alerts.LogSeverity]string{
		alerts.LogSeverity_LOG_SEVERITY_VERBOSE_UNSPECIFIED: "Unspecified",
		alerts.LogSeverity_LOG_SEVERITY_DEBUG:               "Debug",
		alerts.LogSeverity_LOG_SEVERITY_INFO:                "Info",
		alerts.LogSeverity_LOG_SEVERITY_WARNING:             "Warning",
		alerts.LogSeverity_LOG_SEVERITY_ERROR:               "Error",
		alerts.LogSeverity_LOG_SEVERITY_CRITICAL:            "Critical",
	}
	logSeveritySchemaToProtoMap          = ReverseMap(logSeverityProtoToSchemaMap)
	validLogSeverities                   = GetKeys(logSeveritySchemaToProtoMap)
	evaluationWindowTypeProtoToSchemaMap = map[alerts.EvaluationWindow]string{
		alerts.EvaluationWindow_EVALUATION_WINDOW_ROLLING_OR_UNSPECIFIED: "Rolling",
		alerts.EvaluationWindow_EVALUATION_WINDOW_DYNAMIC:                "Dynamic",
	}
	evaluationWindowTypeSchemaToProtoMap = ReverseMap(evaluationWindowTypeProtoToSchemaMap)
	validEvaluationWindowTypes           = GetKeys(evaluationWindowTypeSchemaToProtoMap)
	logsTimeWindowValueProtoToSchemaMap  = map[alerts.LogsTimeWindowValue]string{
		alerts.LogsTimeWindowValue_LOGS_TIME_WINDOW_VALUE_MINUTES_5_OR_UNSPECIFIED: "5_MINUTES",
		alerts.LogsTimeWindowValue_LOGS_TIME_WINDOW_VALUE_MINUTES_10:               "10_MINUTES",
		alerts.LogsTimeWindowValue_LOGS_TIME_WINDOW_VALUE_MINUTES_15:               "15_MINUTES",
		alerts.LogsTimeWindowValue_LOGS_TIME_WINDOW_VALUE_MINUTES_30:               "30_MINUTES",
		alerts.LogsTimeWindowValue_LOGS_TIME_WINDOW_VALUE_HOUR_1:                   "1_HOUR",
		alerts.LogsTimeWindowValue_LOGS_TIME_WINDOW_VALUE_HOURS_2:                  "2_HOURS",
		alerts.LogsTimeWindowValue_LOGS_TIME_WINDOW_VALUE_HOURS_4:                  "4_HOURS",
		alerts.LogsTimeWindowValue_LOGS_TIME_WINDOW_VALUE_HOURS_6:                  "6_HOURS",
		alerts.LogsTimeWindowValue_LOGS_TIME_WINDOW_VALUE_HOURS_12:                 "12_HOURS",
		alerts.LogsTimeWindowValue_LOGS_TIME_WINDOW_VALUE_HOURS_24:                 "24_HOURS",
		alerts.LogsTimeWindowValue_LOGS_TIME_WINDOW_VALUE_HOURS_36:                 "36_HOURS",
	}
	logsTimeWindowValueSchemaToProtoMap = ReverseMap(logsTimeWindowValueProtoToSchemaMap)
	validLogsTimeWindowValues           = GetKeys(logsTimeWindowValueSchemaToProtoMap)
)

func NewAlertResource() resource.Resource {
	return &AlertResource{}
}

type AlertResource struct {
	client *clientset.AlertsClient
}

type AlertResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	Description         types.String `tfsdk:"description"`
	Enabled             types.Bool   `tfsdk:"enabled"`
	AlertPriority       types.String `tfsdk:"alert_priority"`
	AlertSchedule       types.Object `tfsdk:"alert_schedule"`        // AlertScheduleModel
	AlertTypeDefinition types.Object `tfsdk:"alert_type_definition"` // AlertTypeDefinitionModel
	AlertGroupBys       types.List   `tfsdk:"alert_group_bys"`       // []types.String
	IncidentsSettings   types.Object `tfsdk:"incidents_settings"`    // IncidentsSettingsModel
	NotificationGroup   types.Object `tfsdk:"notification_group"`    // NotificationGroupModel
	Labels              types.Map    `tfsdk:"labels"`                // map[string]string
}

type AlertScheduleModel struct {
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

type AlertTypeDefinitionModel struct {
	LogsImmediate            types.Object `tfsdk:"logs_immediate"`               // LogsImmediateModel
	LogsMoreThan             types.Object `tfsdk:"logs_more_than"`               // LogsMoreThanModel
	LogsLessThan             types.Object `tfsdk:"logs_less_than"`               // LogsLessThanModel
	LogsMoreThanUsual        types.Object `tfsdk:"logs_more_than_usual"`         // LogsMoreThanUsualModel
	LogsRatioMoreThan        types.Object `tfsdk:"logs_ratio_more_than"`         // LogsRatioMoreThanModel
	LogsRatioLessThan        types.Object `tfsdk:"logs_ratio_less_than"`         // LogsRatioLessThanModel
	LogsNewValue             types.Object `tfsdk:"logs_new_value"`               // LogsNewValueModel
	LogsUniqueCount          types.Object `tfsdk:"logs_unique_count"`            // LogsUniqueCountModel
	LogsTimeRelativeMoreThan types.Object `tfsdk:"logs_time_relative_more_than"` // LogsTimeRelativeMoreThanModel
	LogsTimeRelativeLessThan types.Object `tfsdk:"logs_time_relative_less_than"` // LogsTimeRelativeLessThanModel
	MetricMoreThan           types.Object `tfsdk:"metric_more_than"`             // MetricMoreThanModel
	MetricLessThan           types.Object `tfsdk:"metric_less_than"`             // MetricLessThanModel
	MetricMoreThanUsual      types.Object `tfsdk:"metric_more_than_usual"`       // MetricMoreThanUsualModel
	TracingImmediate         types.Object `tfsdk:"tracing_immediate"`            // TracingImmediateModel
	TracingMoreThan          types.Object `tfsdk:"tracing_more_than"`            // TracingMoreThanModel
	MetricLessThanUsual      types.Object `tfsdk:"metric_less_than_usual"`       // MetricLessThanUsualModel
	Flow                     types.Object `tfsdk:"flow"`                         // FlowModel
}

type LogsImmediateModel struct {
	LogsFilter                types.Object `tfsdk:"logs_filter"`                 // AlertsLogsFilterModel
	NotificationPayloadFilter types.List   `tfsdk:"notification_payload_filter"` // []types.String
}

type AlertsLogsFilterModel struct {
	LuceneFilter types.Object `tfsdk:"lucene_filter"` // LuceneFilterModel
}

type LuceneFilterModel struct {
	LuceneQuery  types.String `tfsdk:"lucene_query"`
	LabelFilters types.Object `tfsdk:"label_filters"` // LabelFiltersModel
}

type LabelFiltersModel struct {
	ApplicationName types.List `tfsdk:"application_name"` // []LabelFilterTypeModel
	SubsystemName   types.List `tfsdk:"subsystem_name"`   // []LabelFilterTypeModel
	Severities      types.Set  `tfsdk:"severities"`       // []types.String
}

type LabelFilterTypeModel struct {
	Value     types.String `tfsdk:"value"`
	Operation types.String `tfsdk:"operation"`
}

type NotificationPayloadFilterModel struct {
	Filter types.String `tfsdk:"filter"`
}

type LogsMoreThanModel struct {
	Threshold                 types.Int64  `tfsdk:"threshold"`
	TimeWindow                types.Object `tfsdk:"time_window"` // LogsTimeWindowModel
	EvaluationWindow          types.String `tfsdk:"evaluation_window"`
	LogsFilter                types.Object `tfsdk:"logs_filter"`                 // AlertsLogsFilterModel
	NotificationPayloadFilter types.List   `tfsdk:"notification_payload_filter"` // []types.String
}

type LogsTimeWindowModel struct {
	SpecificValue types.String `tfsdk:"specific_value"`
}

type LogsLessThanModel struct {
}

type LogsMoreThanUsualModel struct {
}

type LogsRatioMoreThanModel struct {
}

type LogsRatioLessThanModel struct {
}

type LogsNewValueModel struct {
}

type LogsUniqueCountModel struct {
}

type LogsTimeRelativeMoreThanModel struct {
}

type LogsTimeRelativeLessThanModel struct {
}

type MetricMoreThanModel struct {
}

type MetricLessThanModel struct {
}

type MetricMoreThanUsualModel struct {
}

type TracingImmediateModel struct {
}

type TracingMoreThanModel struct {
}

type FlowModel struct {
}

type MetricLessThanUsualModel struct {
}

type IncidentsSettingsModel struct {
	NotifyOn                  types.String `tfsdk:"notify_on"`
	UseAsNotificationSettings types.Bool   `tfsdk:"use_as_notification_settings"`
	RetriggeringPeriod        types.Object `tfsdk:"retriggering_period"` // RetriggeringPeriodModel
}

type NotificationGroupModel struct {
	GroupByFields types.List `tfsdk:"group_by_fields"` // []types.String
	Notifications types.List `tfsdk:"notifications"`   // AlertNotificationModel
}

type AlertNotificationModel struct {
	RetriggeringPeriod types.Object `tfsdk:"retriggering_period"` // RetriggeringPeriodModel
	NotifyOn           types.String `tfsdk:"notify_on"`
	IntegrationID      types.String `tfsdk:"integration_id"`
	Recipients         types.Set    `tfsdk:"recipients"` //[]types.String
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
			fmt.Sprintf("Expected *clientset.ClientSet, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = clientSet.Alerts()
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
				MarkdownDescription: "Alert enabled status.",
			},
			"alert_priority": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(validAlertPriorities...),
				},
				MarkdownDescription: fmt.Sprintf("Alert priority. Valid values: %q.", validAlertPriorities),
			},
			"alert_schedule": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"days_of_week": schema.ListAttribute{
						Required:    true,
						ElementType: types.StringType,
						Validators: []validator.List{
							listvalidator.ValueStringsAre(
								stringvalidator.OneOf(validDaysOfWeek...),
							),
						},
					},
					"start_time": timeOfDaySchema(),
					"end_time":   timeOfDaySchema(),
				},
			},
			"alert_type_definition": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"logs_immediate": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"logs_filter": logsFilterSchema(),
							"notification_payload_filter": schema.ListAttribute{
								Optional:    true,
								ElementType: types.StringType,
							},
						},
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("logs_more_than"),
								path.MatchRelative().AtParent().AtName("logs_less_than"),
								path.MatchRelative().AtParent().AtName("logs_more_than_usual"),
								path.MatchRelative().AtParent().AtName("logs_ratio_more_than"),
								path.MatchRelative().AtParent().AtName("logs_ratio_less_than"),
								path.MatchRelative().AtParent().AtName("logs_new_value"),
								path.MatchRelative().AtParent().AtName("logs_unique_count"),
								path.MatchRelative().AtParent().AtName("logs_time_relative_more_than"),
								path.MatchRelative().AtParent().AtName("logs_time_relative_less_than"),
								path.MatchRelative().AtParent().AtName("metric_more_than"),
								path.MatchRelative().AtParent().AtName("metric_less_than"),
								path.MatchRelative().AtParent().AtName("metric_more_than_usual"),
								path.MatchRelative().AtParent().AtName("tracing_immediate"),
								path.MatchRelative().AtParent().AtName("tracing_more_than"),
								path.MatchRelative().AtParent().AtName("metric_less_than_usual"),
								path.MatchRelative().AtParent().AtName("flow"),
							),
						},
					},
					"logs_more_than": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"threshold": schema.Int64Attribute{
								Required: true,
							},
							"time_window": schema.SingleNestedAttribute{
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
							},
							"evaluation_window": schema.StringAttribute{
								Optional: true,
								Computed: true,
								Default:  stringdefault.StaticString("Rolling"),
								Validators: []validator.String{
									stringvalidator.OneOf(validEvaluationWindowTypes...),
								},
								MarkdownDescription: fmt.Sprintf("Evaluation window type. Valid values: %q.", validEvaluationWindowTypes),
							},
							"logs_filter": logsFilterSchema(),
						},
					},
					"logs_less_than": schema.SingleNestedAttribute{
						Optional:   true,
						Attributes: map[string]schema.Attribute{},
					},
					"logs_more_than_usual": schema.SingleNestedAttribute{
						Optional:   true,
						Attributes: map[string]schema.Attribute{},
					},
					"logs_ratio_more_than": schema.SingleNestedAttribute{
						Optional:   true,
						Attributes: map[string]schema.Attribute{},
					},
					"logs_ratio_less_than": schema.SingleNestedAttribute{
						Optional:   true,
						Attributes: map[string]schema.Attribute{},
					},
					"logs_new_value": schema.SingleNestedAttribute{
						Optional:   true,
						Attributes: map[string]schema.Attribute{},
					},
					"logs_unique_count": schema.SingleNestedAttribute{
						Optional:   true,
						Attributes: map[string]schema.Attribute{},
					},
					"logs_time_relative_more_than": schema.SingleNestedAttribute{
						Optional:   true,
						Attributes: map[string]schema.Attribute{},
					},
					"logs_time_relative_less_than": schema.SingleNestedAttribute{
						Optional:   true,
						Attributes: map[string]schema.Attribute{},
					},
					"metric_more_than": schema.SingleNestedAttribute{
						Optional:   true,
						Attributes: map[string]schema.Attribute{},
					},
					"metric_less_than": schema.SingleNestedAttribute{
						Optional:   true,
						Attributes: map[string]schema.Attribute{},
					},
					"metric_more_than_usual": schema.SingleNestedAttribute{
						Optional:   true,
						Attributes: map[string]schema.Attribute{},
					},
					"tracing_immediate": schema.SingleNestedAttribute{
						Optional:   true,
						Attributes: map[string]schema.Attribute{},
					},
					"tracing_more_than": schema.SingleNestedAttribute{
						Optional:   true,
						Attributes: map[string]schema.Attribute{},
					},
					"metric_less_than_usual": schema.SingleNestedAttribute{
						Optional:   true,
						Attributes: map[string]schema.Attribute{},
					},
					"flow": schema.SingleNestedAttribute{
						Optional:   true,
						Attributes: map[string]schema.Attribute{},
					},
				},
			},
			"alert_group_bys": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
			"incidents_settings": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"notify_on": schema.StringAttribute{
						Required: true,
						Validators: []validator.String{
							stringvalidator.OneOf(validNotifyOn...),
						},
						MarkdownDescription: fmt.Sprintf("Notify on. Valid values: %q.", validNotifyOn),
					},
					"use_as_notification_settings": schema.BoolAttribute{
						Optional: true,
						Computed: true,
						Default:  booldefault.StaticBool(true),
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
				Attributes: map[string]schema.Attribute{
					"group_by_fields": schema.ListAttribute{
						Optional:    true,
						ElementType: types.StringType,
					},
					"notifications": schema.ListNestedAttribute{
						Optional: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"notify_on": schema.StringAttribute{
									Optional: true,
									Computed: true,
									Validators: []validator.String{
										stringvalidator.OneOf(validNotifyOn...),
										stringvalidator.AtLeastOneOf(path.Root("incidents_settings").AtName("notify_on").Expression()),
									},
								},
								"retriggering_period": schema.SingleNestedAttribute{
									Optional: true,
									Computed: true,
									Attributes: map[string]schema.Attribute{
										"minutes": schema.Int64Attribute{
											Required: true,
										},
									},
									Validators: []validator.Object{
										objectvalidator.AtLeastOneOf(path.Root("incidents_settings").AtName("retriggering_period").Expression()),
									},
								},
								"integration_id": schema.StringAttribute{
									Optional: true,
									Validators: []validator.String{
										stringvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("recipients")),
									},
									MarkdownDescription: "Integration ID.\n" +
										"Exactly one of integration_id or recipients must be set.\n" +
										"Can be linked to an integration by integration_id = coralogix_webhook.<webhook-resource-name>.external_id or setting it explicitly.",
								},
								"recipients": schema.SetAttribute{
									Optional:    true,
									ElementType: types.StringType,
									Validators: []validator.Set{
										setvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("integration_id")),
									},
									MarkdownDescription: "Email recipients.\n" +
										"Exactly one of integration_id or recipients must be set.",
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

func logsFilterSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"lucene_filter": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"lucene_query": schema.StringAttribute{
						Optional: true,
					},
					"label_filters": schema.SingleNestedAttribute{
						Required: true,
						Attributes: map[string]schema.Attribute{
							"application_name": logsAttributeFilterSchema(),
							"subsystem_name":   logsAttributeFilterSchema(),
							"severities": schema.SetAttribute{
								Optional:    true,
								ElementType: types.StringType,
							},
						},
					},
				},
			},
		},
	}
}

func logsAttributeFilterSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Optional: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"value": schema.StringAttribute{
					Required: true,
				},
				"operation": schema.StringAttribute{
					Required: true,
					Validators: []validator.String{
						stringvalidator.OneOf(validLogFilterOperationType...),
					},
				},
			},
		},
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
	createAlertRequest := &alerts.CreateAlertRequest{AlertProperties: alertProperties}
	log.Printf("[INFO] Creating new Alert: %s", protojson.Format(createAlertRequest))
	createResp, err := r.client.CreateAlert(ctx, createAlertRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err)
		resp.Diagnostics.AddError("Error creating Alert",
			formatRpcErrors(err, createAlertURL, protojson.Format(createAlertRequest)),
		)
		return
	}
	alert := createResp.GetAlert()
	log.Printf("[INFO] Submitted new alert: %s", protojson.Format(alert))

	plan, diags = flattenAlert(ctx, alert)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	// Set state to fully populated data
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func extractAlertProperties(ctx context.Context, plan *AlertResourceModel) (*alerts.AlertProperties, diag.Diagnostics) {
	alertGroupBys, diags := typeStringSliceToWrappedStringSlice(ctx, plan.AlertGroupBys.Elements())
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
	alertProperties := &alerts.AlertProperties{
		Name:              typeStringToWrapperspbString(plan.Name),
		Description:       typeStringToWrapperspbString(plan.Description),
		Enabled:           typeBoolToWrapperspbBool(plan.Enabled),
		AlertPriority:     alertPrioritySchemaToProtoMap[plan.AlertPriority.ValueString()],
		AlertGroupBys:     alertGroupBys,
		IncidentsSettings: incidentsSettings,
		NotificationGroup: notificationGroup,
		Labels:            labels,
	}

	alertProperties, diags = expandAlertsSchedule(ctx, alertProperties, plan.AlertSchedule)
	if diags.HasError() {
		return nil, diags
	}

	alertProperties, diags = expandAlertsTypeDefinition(ctx, alertProperties, plan.AlertTypeDefinition)
	if diags.HasError() {
		return nil, diags
	}

	return alertProperties, nil
}

func extractIncidentsSettings(ctx context.Context, incidentsSettingsObject types.Object) (*alerts.AlertIncidentSettings, diag.Diagnostics) {
	if incidentsSettingsObject.IsNull() || incidentsSettingsObject.IsUnknown() {
		return nil, nil
	}

	var incidentsSettingsModel IncidentsSettingsModel
	if diags := incidentsSettingsObject.As(ctx, &incidentsSettingsModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	incidentsSettings := &alerts.AlertIncidentSettings{
		NotifyOn:                  notifyOnSchemaToProtoMap[incidentsSettingsModel.NotifyOn.ValueString()],
		UseAsNotificationSettings: typeBoolToWrapperspbBool(incidentsSettingsModel.UseAsNotificationSettings),
	}

	incidentsSettings, diags := expandIncidentsSettingsByRetriggeringPeriod(ctx, incidentsSettings, incidentsSettingsModel.RetriggeringPeriod)
	if diags.HasError() {
		return nil, diags
	}

	return incidentsSettings, nil
}

func expandIncidentsSettingsByRetriggeringPeriod(ctx context.Context, incidentsSettings *alerts.AlertIncidentSettings, period types.Object) (*alerts.AlertIncidentSettings, diag.Diagnostics) {
	if period.IsNull() || period.IsUnknown() {
		return incidentsSettings, nil
	}

	var periodModel RetriggeringPeriodModel
	if diags := period.As(ctx, &periodModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if !(periodModel.Minutes.IsNull() || periodModel.Minutes.IsUnknown()) {
		incidentsSettings.RetriggeringPeriod = &alerts.AlertIncidentSettings_Minutes{
			Minutes: typeInt64ToWrappedUint32(periodModel.Minutes),
		}
	}

	return incidentsSettings, nil
}

func extractNotificationGroup(ctx context.Context, notificationGroupObject types.Object) (*alerts.AlertNotificationGroup, diag.Diagnostics) {
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

	notifications, diags := extractAlertNotifications(ctx, notificationGroupModel.Notifications)
	if diags.HasError() {
		return nil, diags
	}

	return &alerts.AlertNotificationGroup{
		GroupByFields: groupByFields,
		Notifications: notifications,
	}, nil

}

func extractAlertNotifications(ctx context.Context, notifications types.List) ([]*alerts.AlertNotification, diag.Diagnostics) {
	var notificationsObjects []types.Object
	diags := notifications.ElementsAs(ctx, &notificationsObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	var expandedNotifications []*alerts.AlertNotification
	for _, no := range notificationsObjects {
		var variable AlertNotificationModel
		if dg := no.As(ctx, &variable, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedNotification, expandDiags := extractAlertNotification(ctx, variable)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedNotifications = append(expandedNotifications, expandedNotification)
	}

	if diags.HasError() {
		return nil, diags
	}

	return expandedNotifications, nil
}

func extractAlertNotification(ctx context.Context, variable AlertNotificationModel) (*alerts.AlertNotification, diag.Diagnostics) {
	notifyOn := notifyOnSchemaToProtoMap[variable.NotifyOn.ValueString()]
	alertNotification := &alerts.AlertNotification{
		NotifyOn: &notifyOn,
	}
	alertNotification, diags := expandAlertNotificationByRetriggeringPeriod(ctx, alertNotification, variable.RetriggeringPeriod)
	if diags.HasError() {
		return nil, diags
	}

	if !variable.IntegrationID.IsNull() && !variable.IntegrationID.IsUnknown() {
		integrationId, diag := typeStringToWrapperspbUint32(variable.IntegrationID)
		if diag.HasError() {
			return nil, diag
		}
		alertNotification.IntegrationType = &alerts.AlertNotification_IntegrationId{
			IntegrationId: integrationId,
		}
	} else if !variable.Recipients.IsNull() && !variable.Recipients.IsUnknown() {
		emails, diags := typeStringSliceToWrappedStringSlice(ctx, variable.Recipients.Elements())
		if diags.HasError() {
			return nil, diags
		}
		alertNotification.IntegrationType = &alerts.AlertNotification_Recipients{
			Recipients: &alerts.Recipients{
				Emails: emails,
			},
		}
	}

	return alertNotification, nil
}

func expandAlertNotificationByRetriggeringPeriod(ctx context.Context, alertNotification *alerts.AlertNotification, period types.Object) (*alerts.AlertNotification, diag.Diagnostics) {
	if period.IsNull() || period.IsUnknown() {
		return alertNotification, nil
	}

	var periodModel RetriggeringPeriodModel
	if diags := period.As(ctx, &periodModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if !(periodModel.Minutes.IsNull() || periodModel.Minutes.IsUnknown()) {
		alertNotification.RetriggeringPeriod = &alerts.AlertNotification_Minutes{
			Minutes: typeInt64ToWrappedUint32(periodModel.Minutes),
		}
	}

	return alertNotification, nil
}

func expandAlertsSchedule(ctx context.Context, alertProperties *alerts.AlertProperties, scheduleObject types.Object) (*alerts.AlertProperties, diag.Diagnostics) {
	if scheduleObject.IsNull() || scheduleObject.IsUnknown() {
		return alertProperties, nil
	}

	var scheduleModel AlertScheduleModel
	if diags := scheduleObject.As(ctx, &scheduleModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	daysOfWeek, diags := extractDaysOfWeek(ctx, scheduleModel.DaysOfWeek)
	if diags.HasError() {
		return nil, diags
	}

	startTime, diags := extractTimeOfDay(ctx, scheduleModel.StartTime)
	if diags.HasError() {
		return nil, diags
	}

	endTime, diags := extractTimeOfDay(ctx, scheduleModel.EndTime)
	if diags.HasError() {
		return nil, diags
	}

	alertProperties.AlertSchedule = &alerts.AlertProperties_ActiveOn{
		ActiveOn: &alerts.ActivitySchedule{
			DayOfWeek: daysOfWeek,
			StartTime: startTime,
			EndTime:   endTime,
		},
	}

	return alertProperties, nil
}

func extractTimeOfDay(ctx context.Context, timeObject types.Object) (*alerts.TimeOfDay, diag.Diagnostics) {
	if timeObject.IsNull() || timeObject.IsUnknown() {
		return nil, nil
	}

	var timeOfDayModel TimeOfDayModel
	if diags := timeObject.As(ctx, &timeOfDayModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	return &alerts.TimeOfDay{
		Hours:   int32(timeOfDayModel.Hours.ValueInt64()),
		Minutes: int32(timeOfDayModel.Minutes.ValueInt64()),
	}, nil

}

func extractDaysOfWeek(ctx context.Context, daysOfWeek types.List) ([]alerts.DayOfWeek, diag.Diagnostics) {
	var diags diag.Diagnostics
	daysOfWeekElements := daysOfWeek.Elements()
	result := make([]alerts.DayOfWeek, 0, len(daysOfWeekElements))
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

func expandAlertsTypeDefinition(ctx context.Context, alertProperties *alerts.AlertProperties, alertDefinition types.Object) (*alerts.AlertProperties, diag.Diagnostics) {
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
	} else if tracingImmediate := alertDefinitionModel.TracingImmediate; !(tracingImmediate.IsNull() || tracingImmediate.IsUnknown()) {
		alertProperties, diags = expandTracingImmediateAlertTypeDefinition(ctx, alertProperties, tracingImmediate)
	} else if tracingMoreThan := alertDefinitionModel.TracingMoreThan; !(tracingMoreThan.IsNull() || tracingMoreThan.IsUnknown()) {
		alertProperties, diags = expandTracingMoreThanAlertTypeDefinition(ctx, alertProperties, tracingMoreThan)
	} else if flow := alertDefinitionModel.Flow; !(flow.IsNull() || flow.IsUnknown()) {
		alertProperties, diags = expandFlowAlertTypeDefinition(ctx, alertProperties, flow)
	} else if metricLessThanUsual := alertDefinitionModel.MetricLessThanUsual; !(metricLessThanUsual.IsNull() || metricLessThanUsual.IsUnknown()) {
		alertProperties, diags = expandMetricLessThanUsualAlertTypeDefinition(ctx, alertProperties, metricLessThanUsual)
	} else {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Alert Type Definition", "Alert Type Definition is not valid")}
	}

	if diags.HasError() {
		return nil, diags
	}

	return alertProperties, nil
}

func expandLogsImmediateAlertTypeDefinition(ctx context.Context, properties *alerts.AlertProperties, logsImmediateObject types.Object) (*alerts.AlertProperties, diag.Diagnostics) {
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

	properties.AlertTypeDefinition = &alerts.AlertProperties_LogsImmediate{
		LogsImmediate: &alerts.LogsImmediateAlertTypeDefinition{
			LogsFilter:                logsFilter,
			NotificationPayloadFilter: notificationPayloadFilter,
		},
	}
	properties.AlertType = alerts.AlertType_ALERT_TYPE_LOGS_IMMEDIATE_OR_UNSPECIFIED
	return properties, nil
}

func extractLogsFilter(ctx context.Context, filter types.Object) (*alerts.LogsFilter, diag.Diagnostics) {
	if filter.IsNull() || filter.IsUnknown() {
		return nil, nil
	}

	var filterModel AlertsLogsFilterModel
	if diags := filter.As(ctx, &filterModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	logsFilter := &alerts.LogsFilter{}
	var diags diag.Diagnostics
	if !(filterModel.LuceneFilter.IsNull() || filterModel.LuceneFilter.IsUnknown()) {
		logsFilter.FilterType, diags = extractLuceneFilter(ctx, filterModel.LuceneFilter)
	}

	if diags.HasError() {
		return nil, diags
	}

	return logsFilter, nil
}

func extractLuceneFilter(ctx context.Context, luceneFilter types.Object) (*alerts.LogsFilter_LuceneFilter, diag.Diagnostics) {
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

	return &alerts.LogsFilter_LuceneFilter{
		LuceneFilter: &alerts.LuceneFilter{
			LuceneQuery:  typeStringToWrapperspbString(luceneFilterModel.LuceneQuery),
			LabelFilters: labelFilters,
		},
	}, nil
}

func extractLabelFilters(ctx context.Context, filters types.Object) (*alerts.LabelFilters, diag.Diagnostics) {
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

	return &alerts.LabelFilters{
		ApplicationName: applicationName,
		SubsystemName:   subsystemName,
		Severities:      severities,
	}, nil
}

func extractLabelFilterTypes(ctx context.Context, labelFilterTypes types.List) ([]*alerts.LabelFilterType, diag.Diagnostics) {
	var labelFilterTypesObjects []types.Object
	diags := labelFilterTypes.ElementsAs(ctx, &labelFilterTypesObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	var expandedLabelFilterTypes []*alerts.LabelFilterType
	for _, lft := range labelFilterTypesObjects {
		var labelFilterTypeModel LabelFilterTypeModel
		if dg := lft.As(ctx, &labelFilterTypeModel, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedLabelFilterType := &alerts.LabelFilterType{
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

func extractLogSeverities(ctx context.Context, elements []attr.Value) ([]alerts.LogSeverity, diag.Diagnostics) {
	var diags diag.Diagnostics
	result := make([]alerts.LogSeverity, 0, len(elements))
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

func expandLogsMoreThanAlertTypeDefinition(ctx context.Context, properties *alerts.AlertProperties, moreThanObject types.Object) (*alerts.AlertProperties, diag.Diagnostics) {
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

	properties.AlertTypeDefinition = &alerts.AlertProperties_LogsMoreThan{
		LogsMoreThan: &alerts.LogsMoreThanAlertTypeDefinition{
			LogsFilter:                logsFilter,
			Threshold:                 typeInt64ToWrappedUint32(moreThanModel.Threshold),
			TimeWindow:                timeWindow,
			EvaluationWindow:          evaluationWindowTypeSchemaToProtoMap[moreThanModel.EvaluationWindow.ValueString()],
			NotificationPayloadFilter: notificationPayloadFilter,
		},
	}
	properties.AlertType = alerts.AlertType_ALERT_TYPE_LOGS_MORE_THAN
	return properties, nil
}

func extractLogsTimeWindow(ctx context.Context, timeWindow types.Object) (*alerts.LogsTimeWindow, diag.Diagnostics) {
	if timeWindow.IsNull() || timeWindow.IsUnknown() {
		return nil, nil
	}

	var timeWindowModel LogsTimeWindowModel
	if diags := timeWindow.As(ctx, &timeWindowModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if specificValue := timeWindowModel.SpecificValue; specificValue.IsNull() || specificValue.IsUnknown() {
		return &alerts.LogsTimeWindow{
			Type: &alerts.LogsTimeWindow_LogsTimeWindowSpecificValue{
				LogsTimeWindowSpecificValue: logsTimeWindowValueSchemaToProtoMap[specificValue.ValueString()],
			},
		}, nil
	}

	return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Time Window", "Time Window is not valid")}
}

func expandLogsLessThanAlertTypeDefinition(ctx context.Context, properties *alerts.AlertProperties, than types.Object) (*alerts.AlertProperties, diag.Diagnostics) {
	return nil, nil
}

func expandLogsMoreThanUsualAlertTypeDefinition(ctx context.Context, properties *alerts.AlertProperties, usual types.Object) (*alerts.AlertProperties, diag.Diagnostics) {
	return nil, nil
}

func expandLogsRatioMoreThanAlertTypeDefinition(ctx context.Context, properties *alerts.AlertProperties, than types.Object) (*alerts.AlertProperties, diag.Diagnostics) {
	return nil, nil
}

func expandLogsRatioLessThanAlertTypeDefinition(ctx context.Context, properties *alerts.AlertProperties, than types.Object) (*alerts.AlertProperties, diag.Diagnostics) {
	return nil, nil
}

func expandLogsNewValueAlertTypeDefinition(ctx context.Context, properties *alerts.AlertProperties, value types.Object) (*alerts.AlertProperties, diag.Diagnostics) {
	return nil, nil
}

func expandLogsUniqueCountAlertTypeDefinition(ctx context.Context, properties *alerts.AlertProperties, count types.Object) (*alerts.AlertProperties, diag.Diagnostics) {
	return nil, nil
}

func expandLogsTimeRelativeMoreThanAlertTypeDefinition(ctx context.Context, properties *alerts.AlertProperties, than types.Object) (*alerts.AlertProperties, diag.Diagnostics) {
	return nil, nil
}

func expandLogsTimeRelativeLessThanAlertTypeDefinition(ctx context.Context, properties *alerts.AlertProperties, than types.Object) (*alerts.AlertProperties, diag.Diagnostics) {
	return nil, nil
}

func expandMetricMoreThanAlertTypeDefinition(ctx context.Context, properties *alerts.AlertProperties, than types.Object) (*alerts.AlertProperties, diag.Diagnostics) {
	return nil, nil
}

func expandMetricLessThanAlertTypeDefinition(ctx context.Context, properties *alerts.AlertProperties, than types.Object) (*alerts.AlertProperties, diag.Diagnostics) {
	return nil, nil
}

func expandMetricMoreThanUsualAlertTypeDefinition(ctx context.Context, properties *alerts.AlertProperties, usual types.Object) (*alerts.AlertProperties, diag.Diagnostics) {
	return nil, nil
}

func expandTracingImmediateAlertTypeDefinition(ctx context.Context, properties *alerts.AlertProperties, immediate types.Object) (*alerts.AlertProperties, diag.Diagnostics) {
	return nil, nil
}

func expandTracingMoreThanAlertTypeDefinition(ctx context.Context, properties *alerts.AlertProperties, than types.Object) (*alerts.AlertProperties, diag.Diagnostics) {
	return nil, nil
}

func expandFlowAlertTypeDefinition(ctx context.Context, properties *alerts.AlertProperties, flow types.Object) (*alerts.AlertProperties, diag.Diagnostics) {
	return nil, nil
}

func expandMetricLessThanUsualAlertTypeDefinition(ctx context.Context, properties *alerts.AlertProperties, usual types.Object) (*alerts.AlertProperties, diag.Diagnostics) {
	return nil, nil
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
	getAlertReq := &alerts.GetAlertRequest{Id: wrapperspb.String(id)}
	getAlertResp, err := r.client.GetAlert(ctx, getAlertReq)
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
	alert := getAlertResp.GetAlert()
	log.Printf("[INFO] Received Alert: %s", protojson.Format(alert))

	state, diags = flattenAlert(ctx, alert)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func flattenAlert(ctx context.Context, alert *alerts.Alert) (*AlertResourceModel, diag.Diagnostics) {
	alertProperties := alert.GetProperties()
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
		ID:                  wrapperspbStringToTypeString(alert.GetId()),
		Name:                wrapperspbStringToTypeString(alertProperties.GetName()),
		Description:         wrapperspbStringToTypeString(alertProperties.GetDescription()),
		Enabled:             wrapperspbBoolToTypeBool(alertProperties.GetEnabled()),
		AlertPriority:       types.StringValue(alertPriorityProtoToSchemaMap[alertProperties.GetAlertPriority()]),
		AlertSchedule:       alertSchedule,
		AlertTypeDefinition: alertTypeDefinition,
		AlertGroupBys:       wrappedStringSliceToTypeStringList(alertProperties.GetAlertGroupBys()),
		IncidentsSettings:   incidentsSettings,
		NotificationGroup:   notificationGroup,
		Labels:              labels,
	}, nil
}

func flattenNotificationGroup(ctx context.Context, notificationGroup *alerts.AlertNotificationGroup) (types.Object, diag.Diagnostics) {
	if notificationGroup == nil {
		return types.ObjectNull(notificationGroupAttr()), nil
	}

	notifications, diags := flattenAlertNotifications(ctx, notificationGroup.GetNotifications())
	if diags.HasError() {
		return types.ObjectNull(notificationGroupAttr()), diags
	}

	notificationGroupModel := NotificationGroupModel{
		GroupByFields: wrappedStringSliceToTypeStringList(notificationGroup.GetGroupByFields()),
		Notifications: notifications,
	}
	return types.ObjectValueFrom(ctx, notificationGroupAttr(), notificationGroupModel)
}

func flattenAlertNotifications(ctx context.Context, notifications []*alerts.AlertNotification) (types.List, diag.Diagnostics) {
	var notificationsModel []AlertNotificationModel
	var diags diag.Diagnostics
	for _, notification := range notifications {
		retriggeringPeriod, dgs := flattenRetriggeringPeriod(ctx, notification)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		notificationModel := AlertNotificationModel{
			NotifyOn:           types.StringValue(notifyOnProtoToSchemaMap[notification.GetNotifyOn()]),
			IntegrationID:      WrapperspbUint32ToString(notification.GetIntegrationId()),
			Recipients:         wrappedStringSliceToTypeStringSet(notification.GetRecipients().GetEmails()),
			RetriggeringPeriod: retriggeringPeriod,
		}
		notificationsModel = append(notificationsModel, notificationModel)
	}
	if diags.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: alertNotificationAttr()}), diags
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: alertNotificationAttr()}, notificationsModel)
}

func flattenRetriggeringPeriod(ctx context.Context, notifications *alerts.AlertNotification) (types.Object, diag.Diagnostics) {
	switch notificationPeriodType := notifications.RetriggeringPeriod.(type) {
	case *alerts.AlertNotification_Minutes:
		return types.ObjectValueFrom(ctx, retriggeringPeriodAttr(), RetriggeringPeriodModel{
			Minutes: wrapperspbUint32ToTypeInt64(notificationPeriodType.Minutes),
		})
	default:
		return types.ObjectNull(retriggeringPeriodAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Retriggering Period", fmt.Sprintf("Retriggering Period %v is not valid", notificationPeriodType))}
	}
}

func flattenIncidentsSettings(ctx context.Context, incidentsSettings *alerts.AlertIncidentSettings) (types.Object, diag.Diagnostics) {
	if incidentsSettings == nil {
		return types.ObjectNull(incidentsSettingsAttr()), nil
	}

	retriggeringPeriod, diags := flattenIncidentsSettingsByRetriggeringPeriod(ctx, incidentsSettings)
	if diags.HasError() {
		return types.ObjectNull(incidentsSettingsAttr()), diags
	}

	incidentsSettingsModel := IncidentsSettingsModel{
		NotifyOn:                  types.StringValue(notifyOnProtoToSchemaMap[incidentsSettings.GetNotifyOn()]),
		UseAsNotificationSettings: wrapperspbBoolToTypeBool(incidentsSettings.GetUseAsNotificationSettings()),
		RetriggeringPeriod:        retriggeringPeriod,
	}
	return types.ObjectValueFrom(ctx, incidentsSettingsAttr(), incidentsSettingsModel)
}

func flattenIncidentsSettingsByRetriggeringPeriod(ctx context.Context, settings *alerts.AlertIncidentSettings) (types.Object, diag.Diagnostics) {
	if settings.RetriggeringPeriod == nil {
		return types.ObjectNull(retriggeringPeriodAttr()), nil
	}

	var periodModel RetriggeringPeriodModel
	switch period := settings.RetriggeringPeriod.(type) {
	case *alerts.AlertIncidentSettings_Minutes:
		periodModel.Minutes = wrapperspbUint32ToTypeInt64(period.Minutes)
	default:
		return types.ObjectNull(retriggeringPeriodAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Retriggering Period", fmt.Sprintf("Retriggering Period %v is not valid", period))}
	}

	return types.ObjectValueFrom(ctx, retriggeringPeriodAttr(), periodModel)
}

func flattenAlertTypeDefinition(ctx context.Context, properties *alerts.AlertProperties) (types.Object, diag.Diagnostics) {
	if properties.AlertTypeDefinition == nil {
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
		TracingImmediate:         types.ObjectNull(tracingImmediateAttr()),
		TracingMoreThan:          types.ObjectNull(tracingMoreThanAttr()),
		Flow:                     types.ObjectNull(flowAttr()),
		MetricLessThanUsual:      types.ObjectNull(metricLessThanUsualAttr()),
	}
	var diags diag.Diagnostics
	switch alertTypeDefinition := properties.AlertTypeDefinition.(type) {
	case *alerts.AlertProperties_LogsImmediate:
		alertTypeDefinitionModel.LogsImmediate, diags = flattenLogsImmediate(ctx, alertTypeDefinition.LogsImmediate)
	case *alerts.AlertProperties_LogsMoreThan:
		alertTypeDefinitionModel.LogsMoreThan, diags = flattenLogsMoreThan(ctx, alertTypeDefinition.LogsMoreThan)
	case *alerts.AlertProperties_LogsLessThan:
		alertTypeDefinitionModel.LogsLessThan, diags = flattenLogsLessThan(ctx, alertTypeDefinition.LogsLessThan)
	case *alerts.AlertProperties_LogsMoreThanUsual:
		alertTypeDefinitionModel.LogsMoreThanUsual, diags = flattenLogsMoreThanUsual(ctx, alertTypeDefinition.LogsMoreThanUsual)
	case *alerts.AlertProperties_LogsRatioMoreThan:
		alertTypeDefinitionModel.LogsRatioMoreThan, diags = flattenLogsRatioMoreThan(ctx, alertTypeDefinition.LogsRatioMoreThan)
	case *alerts.AlertProperties_LogsRatioLessThan:
		alertTypeDefinitionModel.LogsRatioLessThan, diags = flattenLogsRatioLessThan(ctx, alertTypeDefinition.LogsRatioLessThan)
	case *alerts.AlertProperties_LogsNewValue:
		alertTypeDefinitionModel.LogsNewValue, diags = flattenLogsNewValue(ctx, alertTypeDefinition.LogsNewValue)
	case *alerts.AlertProperties_LogsUniqueCount:
		alertTypeDefinitionModel.LogsUniqueCount, diags = flattenLogsUniqueCount(ctx, alertTypeDefinition.LogsUniqueCount)
	case *alerts.AlertProperties_LogsTimeRelativeMoreThan:
		alertTypeDefinitionModel.LogsTimeRelativeMoreThan, diags = flattenLogsTimeRelativeMoreThan(ctx, alertTypeDefinition.LogsTimeRelativeMoreThan)
	case *alerts.AlertProperties_LogsTimeRelativeLessThan:
		alertTypeDefinitionModel.LogsTimeRelativeLessThan, diags = flattenLogsTimeRelativeLessThan(ctx, alertTypeDefinition.LogsTimeRelativeLessThan)
	case *alerts.AlertProperties_MetricMoreThan:
		alertTypeDefinitionModel.MetricMoreThan, diags = flattenMetricMoreThan(ctx, alertTypeDefinition.MetricMoreThan)
	case *alerts.AlertProperties_MetricLessThan:
		alertTypeDefinitionModel.MetricLessThan, diags = flattenMetricLessThan(ctx, alertTypeDefinition.MetricLessThan)
	case *alerts.AlertProperties_MetricMoreThanUsual:
		alertTypeDefinitionModel.MetricMoreThanUsual, diags = flattenMetricMoreThanUsual(ctx, alertTypeDefinition.MetricMoreThanUsual)
	case *alerts.AlertProperties_TracingImmediate:
		alertTypeDefinitionModel.TracingImmediate, diags = flattenTracingImmediate(alertTypeDefinition.TracingImmediate)
	case *alerts.AlertProperties_TracingMoreThan:
		alertTypeDefinitionModel.TracingMoreThan, diags = flattenTracingMoreThan(ctx, alertTypeDefinition.TracingMoreThan)
	case *alerts.AlertProperties_Flow:
		alertTypeDefinitionModel.Flow, diags = flattenFlow(ctx, alertTypeDefinition.Flow)
	case *alerts.AlertProperties_MetricLessThanUsual:
		alertTypeDefinitionModel.MetricLessThanUsual, diags = flattenMetricLessThanUsual(ctx, alertTypeDefinition.MetricLessThanUsual)
	default:
		return types.ObjectNull(alertTypeDefinitionAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Alert Type Definition", fmt.Sprintf("Alert Type %v Definition is not valid", alertTypeDefinition))}
	}

	if diags.HasError() {
		return types.ObjectNull(alertTypeDefinitionAttr()), diags
	}

	return types.ObjectValueFrom(ctx, alertTypeDefinitionAttr(), alertTypeDefinitionModel)
}

func flattenLogsImmediate(ctx context.Context, immediate *alerts.LogsImmediateAlertTypeDefinition) (types.Object, diag.Diagnostics) {
	if immediate == nil {
		return types.ObjectNull(logsImmediateAttr()), nil
	}

	logsFilter, diags := flattenAlertsLogsFilter(ctx, immediate.GetLogsFilter())
	if diags.HasError() {
		return types.ObjectNull(logsImmediateAttr()), diags
	}

	logsImmediateModel := LogsImmediateModel{
		LogsFilter:                logsFilter,
		NotificationPayloadFilter: wrappedStringSliceToTypeStringList(immediate.GetNotificationPayloadFilter()),
	}
	return types.ObjectValueFrom(ctx, logsImmediateAttr(), logsImmediateModel)
}

func flattenAlertsLogsFilter(ctx context.Context, filter *alerts.LogsFilter) (types.Object, diag.Diagnostics) {
	if filter == nil {
		return types.ObjectNull(logsFilterAttr()), nil
	}

	var diags diag.Diagnostics
	var logsFilterModer AlertsLogsFilterModel
	switch filterType := filter.FilterType.(type) {
	case *alerts.LogsFilter_LuceneFilter:
		logsFilterModer.LuceneFilter, diags = flattenLuceneFilter(ctx, filterType.LuceneFilter)
	default:
		return types.ObjectNull(logsFilterAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Logs Filter", fmt.Sprintf("Logs Filter %v is not valid", filterType))}
	}

	if diags.HasError() {
		return types.ObjectNull(logsFilterAttr()), diags
	}

	return types.ObjectValueFrom(ctx, logsFilterAttr(), logsFilterModer)
}

func flattenLuceneFilter(ctx context.Context, filter *alerts.LuceneFilter) (types.Object, diag.Diagnostics) {
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

func flattenLabelFilters(ctx context.Context, filters *alerts.LabelFilters) (types.Object, diag.Diagnostics) {
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

func flattenLabelFilterTypes(ctx context.Context, name []*alerts.LabelFilterType) (types.List, diag.Diagnostics) {
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
		return types.ListNull(types.ObjectType{AttrTypes: labelFilterTypesAttr()}), diags
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: labelFilterTypesAttr()}, labelFilterTypes)

}

func flattenLogSeverities(ctx context.Context, severities []alerts.LogSeverity) (types.Set, diag.Diagnostics) {
	var result []attr.Value
	for _, severity := range severities {
		result = append(result, types.StringValue(logSeverityProtoToSchemaMap[severity]))
	}
	return types.SetValueFrom(ctx, types.StringType, result)
}

func flattenLogsMoreThan(ctx context.Context, moreThan *alerts.LogsMoreThanAlertTypeDefinition) (types.Object, diag.Diagnostics) {
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
		NotificationPayloadFilter: wrappedStringSliceToTypeStringList(moreThan.GetNotificationPayloadFilter()),
	}
	return types.ObjectValueFrom(ctx, logsMoreThanAttr(), logsMoreThanModel)
}

func flattenLogsTimeWindow(ctx context.Context, timeWindow *alerts.LogsTimeWindow) (types.Object, diag.Diagnostics) {
	if timeWindow == nil {
		return types.ObjectNull(logsTimeWindowAttr()), nil
	}

	switch timeWindowType := timeWindow.Type.(type) {
	case *alerts.LogsTimeWindow_LogsTimeWindowSpecificValue:
		return types.ObjectValueFrom(ctx, logsTimeWindowAttr(), LogsTimeWindowModel{
			SpecificValue: types.StringValue(logsTimeWindowValueProtoToSchemaMap[timeWindowType.LogsTimeWindowSpecificValue]),
		})
	default:
		return types.ObjectNull(logsTimeWindowAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Time Window", fmt.Sprintf("Time Window %v is not valid", timeWindowType))}
	}

}

func flattenMetricLessThanUsual(ctx context.Context, usual *alerts.MetricLessThanUsualAlertTypeDefinition) (types.Object, diag.Diagnostics) {
	return types.ObjectNull(metricLessThanUsualAttr()), nil
}

func flattenFlow(ctx context.Context, flow *alerts.FlowAlertTypeDefinition) (types.Object, diag.Diagnostics) {
	return types.ObjectNull(flowAttr()), nil
}

func flattenTracingMoreThan(ctx context.Context, than *alerts.TracingMoreThanAlertTypeDefinition) (types.Object, diag.Diagnostics) {
	return types.ObjectNull(tracingMoreThanAttr()), nil
}

func flattenTracingImmediate(immediate *alerts.TracingImmediateAlertTypeDefinition) (types.Object, diag.Diagnostics) {
	return types.ObjectNull(tracingImmediateAttr()), nil
}

func flattenMetricMoreThanUsual(ctx context.Context, usual *alerts.MetricMoreThanUsualAlertTypeDefinition) (types.Object, diag.Diagnostics) {
	return types.ObjectNull(metricMoreThanUsualAttr()), nil
}

func flattenMetricLessThan(ctx context.Context, than *alerts.MetricLessThanAlertTypeDefinition) (types.Object, diag.Diagnostics) {
	return types.ObjectNull(metricLessThanAttr()), nil
}

func flattenMetricMoreThan(ctx context.Context, than *alerts.MetricMoreThanAlertTypeDefinition) (types.Object, diag.Diagnostics) {
	return types.ObjectNull(metricMoreThanAttr()), nil
}

func flattenLogsTimeRelativeLessThan(ctx context.Context, than *alerts.LogsTimeRelativeLessThanAlertTypeDefinition) (types.Object, diag.Diagnostics) {
	return types.ObjectNull(logsTimeRelativeLessThanAttr()), nil
}

func flattenLogsTimeRelativeMoreThan(ctx context.Context, than *alerts.LogsTimeRelativeMoreThanAlertTypeDefinition) (types.Object, diag.Diagnostics) {
	return types.ObjectNull(logsTimeRelativeMoreThanAttr()), nil
}

func flattenLogsUniqueCount(ctx context.Context, count *alerts.LogsUniqueCountAlertTypeDefinition) (types.Object, diag.Diagnostics) {
	return types.ObjectNull(logsUniqueCountAttr()), nil
}

func flattenLogsNewValue(ctx context.Context, value *alerts.LogsNewValueAlertTypeDefinition) (types.Object, diag.Diagnostics) {
	return types.ObjectNull(logsNewValueAttr()), nil

}

func flattenLogsRatioLessThan(ctx context.Context, than *alerts.LogsRatioLessThanAlertTypeDefinition) (types.Object, diag.Diagnostics) {
	return types.ObjectNull(logsRatioLessThanAttr()), nil

}

func flattenLogsRatioMoreThan(ctx context.Context, than *alerts.LogsRatioMoreThanAlertTypeDefinition) (types.Object, diag.Diagnostics) {
	return types.ObjectNull(logsRatioMoreThanAttr()), nil

}

func flattenLogsMoreThanUsual(ctx context.Context, usual *alerts.LogsMoreThanUsualAlertTypeDefinition) (types.Object, diag.Diagnostics) {
	return types.ObjectNull(logsMoreThanUsualAttr()), nil

}

func flattenLogsLessThan(ctx context.Context, than *alerts.LogsLessThanAlertTypeDefinition) (types.Object, diag.Diagnostics) {
	return types.ObjectNull(logsLessThanAttr()), nil
}

func flattenAlertSchedule(ctx context.Context, alertProperties *alerts.AlertProperties) (types.Object, diag.Diagnostics) {
	switch alertScheduleType := alertProperties.AlertSchedule.(type) {
	case *alerts.AlertProperties_ActiveOn:
		activeOn := alertProperties.GetActiveOn()
		daysOfWeek, diags := flattenDaysOfWeek(ctx, activeOn.GetDayOfWeek())
		if diags.HasError() {
			return types.ObjectNull(alertScheduleAttr()), diags
		}
		startTime, diags := flattenTimeOfDay(ctx, activeOn.GetStartTime())
		if diags.HasError() {
			return types.ObjectNull(alertScheduleAttr()), diags
		}
		endTime, diags := flattenTimeOfDay(ctx, activeOn.GetEndTime())
		if diags.HasError() {
			return types.ObjectNull(alertScheduleAttr()), diags
		}
		alertScheduleModel := AlertScheduleModel{
			DaysOfWeek: daysOfWeek,
			StartTime:  startTime,
			EndTime:    endTime,
		}
		return types.ObjectValueFrom(ctx, alertScheduleAttr(), alertScheduleModel)
	default:
		return types.ObjectNull(alertScheduleAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Alert Schedule", fmt.Sprintf("Alert Schedule %v is not supported", alertScheduleType))}
	}
}

func flattenDaysOfWeek(ctx context.Context, daysOfWeek []alerts.DayOfWeek) (types.List, diag.Diagnostics) {
	var daysOfWeekStrings []types.String
	for _, dow := range daysOfWeek {
		daysOfWeekStrings = append(daysOfWeekStrings, types.StringValue(daysOfWeekProtoToSchemaMap[dow]))
	}
	return types.ListValueFrom(ctx, types.StringType, daysOfWeekStrings)
}

func flattenTimeOfDay(ctx context.Context, time *alerts.TimeOfDay) (types.Object, diag.Diagnostics) {
	if time == nil {
		return types.ObjectNull(timeOfDayAttr()), nil
	}
	return types.ObjectValueFrom(ctx, timeOfDayAttr(), TimeOfDayModel{
		Hours:   types.Int64Value(int64(time.GetHours())),
		Minutes: types.Int64Value(int64(time.GetMinutes())),
	})
}

func retriggeringPeriodAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"minutes": types.Int64Type,
	}
}

func incidentsSettingsAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"notify_on":                    types.StringType,
		"use_as_notification_settings": types.BoolType,
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
		"notifications": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: alertNotificationAttr(),
			},
		},
	}
}

func alertNotificationAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"notify_on": types.StringType,
		"retriggering_period": types.ObjectType{
			AttrTypes: retriggeringPeriodAttr(),
		},
		"integration_id": types.StringType,
		"recipients":     types.SetType{ElemType: types.StringType},
	}
}

func labelFilterTypesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"value":     types.StringType,
		"operation": types.StringType,
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
		"tracing_immediate": types.ObjectType{
			AttrTypes: tracingImmediateAttr(),
		},
		"tracing_more_than": types.ObjectType{
			AttrTypes: tracingMoreThanAttr(),
		},
		"flow": types.ObjectType{
			AttrTypes: flowAttr(),
		},
		"metric_less_than_usual": types.ObjectType{
			AttrTypes: metricLessThanUsualAttr(),
		},
	}
}

func logsImmediateAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs_filter": types.ObjectType{
			AttrTypes: logsFilterAttr(),
		},
		"notification_payload_filter": types.ListType{
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
		"application_name": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: labelFilterTypesAttr(),
			},
		},
		"subsystem_name": types.ListType{
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
		"notification_payload_filter": types.ListType{ElemType: types.StringType},
	}
}

func logsTimeWindowAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"specific_value": types.StringType,
	}
}

func metricLessThanUsualAttr() map[string]attr.Type {
	return map[string]attr.Type{}
}

func flowAttr() map[string]attr.Type {
	return map[string]attr.Type{}
}

func tracingMoreThanAttr() map[string]attr.Type {
	return map[string]attr.Type{}
}

func tracingImmediateAttr() map[string]attr.Type {
	return map[string]attr.Type{}
}

func metricMoreThanUsualAttr() map[string]attr.Type {
	return map[string]attr.Type{}
}

func metricLessThanAttr() map[string]attr.Type {
	return map[string]attr.Type{}
}

func metricMoreThanAttr() map[string]attr.Type {
	return map[string]attr.Type{}
}

func logsTimeRelativeLessThanAttr() map[string]attr.Type {
	return map[string]attr.Type{}
}

func logsTimeRelativeMoreThanAttr() map[string]attr.Type {
	return map[string]attr.Type{}
}

func logsUniqueCountAttr() map[string]attr.Type {
	return map[string]attr.Type{}
}

func logsNewValueAttr() map[string]attr.Type {
	return map[string]attr.Type{}
}

func logsRatioLessThanAttr() map[string]attr.Type {
	return map[string]attr.Type{}
}

func logsRatioMoreThanAttr() map[string]attr.Type {
	return map[string]attr.Type{}
}

func logsMoreThanUsualAttr() map[string]attr.Type {
	return map[string]attr.Type{}
}

func logsLessThanAttr() map[string]attr.Type {
	return map[string]attr.Type{}
}

func alertScheduleAttr() map[string]attr.Type {
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
	updateAlertReq := &alerts.ReplaceAlertRequest{
		Id:              typeStringToWrapperspbString(plan.ID),
		AlertProperties: alertProperties,
	}
	log.Printf("[INFO] Updating Alert: %s", protojson.Format(updateAlertReq))
	alertUpdateResp, err := r.client.UpdateAlert(ctx, updateAlertReq)
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
	getAlertReq := &alerts.GetAlertRequest{Id: typeStringToWrapperspbString(plan.ID)}
	getAlertResp, err := r.client.GetAlert(ctx, getAlertReq)
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

	plan, diags = flattenAlert(ctx, getAlertResp.GetAlert())
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
	log.Printf("[INFO] Deleting Alert %s", id)
	deleteReq := &alerts.DeleteAlertRequest{Id: wrapperspb.String(id)}
	log.Printf("[INFO] Deleting Alert: %s", protojson.Format(deleteReq))
	if _, err := r.client.DeleteAlert(ctx, deleteReq); err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting Alert %s", id),
			formatRpcErrors(err, deleteAlertURL, protojson.Format(deleteReq)),
		)
		return
	}
	log.Printf("[INFO] Alert %s deleted", id)
}
