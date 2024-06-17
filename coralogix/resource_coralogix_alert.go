package coralogix

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/protobuf/encoding/protojson"
	"terraform-provider-coralogix/coralogix/clientset"
	alerts "terraform-provider-coralogix/coralogix/clientset/grpc/alerts/v3"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
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
	validAlertPriorities          = GetKeys(alertPriorityProtoToSchemaMap)
	notifyOnProtoToSchemaMap      = map[alerts.NotifyOn]string{
		alerts.NotifyOn_NOTIFY_ON_TRIGGERED_ONLY_UNSPECIFIED: "Triggered Only",
		alerts.NotifyOn_NOTIFY_ON_TRIGGERED_AND_RESOLVED:     "Triggered and Resolved",
	}
	notifyOnSchemaToProtoMap   = ReverseMap(notifyOnProtoToSchemaMap)
	validNotifyOn              = GetKeys(notifyOnProtoToSchemaMap)
	daysOfWeekProtoToSchemaMap = map[alerts.DayOfWeek]string{
		alerts.DayOfWeek_DAY_OF_WEEK_MONDAY_OR_UNSPECIFIED: "Monday",
		alerts.DayOfWeek_DAY_OF_WEEK_TUESDAY:               "Tuesday",
		alerts.DayOfWeek_DAY_OF_WEEK_WEDNESDAY:             "Wednesday",
		alerts.DayOfWeek_DAY_OF_WEEK_THURSDAY:              "Thursday",
		alerts.DayOfWeek_DAY_OF_WEEK_FRIDAY:                "Friday",
		alerts.DayOfWeek_DAY_OF_WEEK_SATURDAY:              "Saturday",
		alerts.DayOfWeek_DAY_OF_WEEK_SUNDAY:                "Sunday",
	}
	daysOfWeekSchemaToProtoMap = ReverseMap(daysOfWeekProtoToSchemaMap)
	validDaysOfWeek            = GetKeys(daysOfWeekProtoToSchemaMap)
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
	StartTime  types.String `tfsdk:"start_time"`
	EndTime    types.String `tfsdk:"end_time"`
}

type IncidentsSettingsModel struct {
	NotifyOn                  types.String `tfsdk:"notify_on"`
	UseAsNotificationSettings types.Bool   `tfsdk:"use_as_notification_settings"`
	RetriggeringPeriod        types.Object `tfsdk:"retriggering_period"` // RetriggeringPeriodModel
}

type RetriggeringPeriodModel struct {
	Minutes types.Int64 `tfsdk:"minutes"`
}

type NotificationGroupModel struct {
	GroupByFields types.List `tfsdk:"group_by_fields"` // []types.String
	Notifications types.List `tfsdk:"notifications"`   // []AlertNotificationModel
}

type AlertNotificationModel struct {
	RetriggeringPeriod types.Object `tfsdk:"retriggering_period"` // RetriggeringPeriodModel
	NotifyOn           types.String `tfsdk:"notify_on"`
	IntegrationID      types.String `tfsdk:"integration_id"`
	Emails             types.Set    `tfsdk:"recipients"` //[]types.String
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
	Flow                     types.Object `tfsdk:"flow"`                         // FlowModel
	MetricLessThanUsual      types.Object `tfsdk:"metric_less_than_usual"`       // MetricLessThanUsualModel
}

type LogsImmediateModel struct {
	LogsFilter                types.Object `tfsdk:"logs_filter"`                 // AlertsLogsFilterModel
	NotificationPayloadFilter types.List   `tfsdk:"notification_payload_filter"` // []types.String
}

type AlertsLogsFilterModel struct {
	LuceneFilter types.Object `tfsdk:"lucene_filter"` // LuceneFilterModel
}

type LuceneFilterModel struct {
}

type NotificationPayloadFilterModel struct {
	Filter types.String `tfsdk:"filter"`
}

type LogsMoreThanModel struct {
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
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Action ID.",
			},
			"name": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Action name.",
			},
			"url": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					urlValidationFuncFramework{},
				},
				MarkdownDescription: "URL for the external tool.",
			},
			"is_private": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Determines weather the action will be shared with the entire team. Can be set to false only by admin.",
			},
			"is_hidden": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Determines weather the action will be shown at the action menu.",
			},
			"source_type": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(actionValidSourceTypes...),
				},
				MarkdownDescription: fmt.Sprintf("By selecting the data type, you can make sure that the action will be displayed only in the relevant context. Can be one of %q", actionValidSourceTypes),
			},
			"applications": schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
				MarkdownDescription: "Applies the action for specific applications.",
			},
			"subsystems": schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
				MarkdownDescription: "Applies the action for specific subsystems.",
			},
			"created_by": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "The user who created the action.",
			},
		},
		MarkdownDescription: "Coralogix action. For more info please review - https://coralogix.com/docs/coralogix-action-extension/.",
	}
}

func (r *AlertResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *AlertResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
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
	createAlertRequest := &alerts.CreateAlertRequest{AlertProperties: alertProperties}
	log.Printf("[INFO] Creating new Alert: %s", protojson.Format(createAlertRequest))
	createResp, err := r.client.CreateAlert(ctx, createAlertRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err)
		resp.Diagnostics.AddError("Error creating Action",
			formatRpcErrors(err, createAlertURL, protojson.Format(createAlertRequest)),
		)
		return
	}
	alert := createResp.GetAlert()
	log.Printf("[INFO] Submitted new alert: %s", protojson.Format(alert))

	plan, diags = flattenAlert(ctx, alert)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
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
	} else if !variable.Emails.IsNull() && !variable.Emails.IsUnknown() {
		emails, diags := typeStringSliceToWrappedStringSlice(ctx, variable.Emails.Elements())
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

	startTime, dg := stringToTimeOfDay(scheduleModel.StartTime.ValueString())
	if dg != nil {
		diags.Append(dg)
		return nil, diags
	}

	endTime, dg := stringToTimeOfDay(scheduleModel.EndTime.ValueString())
	if diags.HasError() {
		diags.Append(dg)
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

func stringToTimeOfDay(str string) (*alerts.TimeOfDay, diag.Diagnostic) {
	timeArr := strings.Split(str, ":")
	if len(timeArr) != 2 {
		return nil, diag.NewErrorDiagnostic("Invalid time format", "Time should be in HH:MM format")
	}
	hours, err := strconv.Atoi(timeArr[0])
	if err != nil {
		return nil, diag.NewErrorDiagnostic("Invalid time format", "Hours should be a number")
	}
	minutes, err := strconv.Atoi(timeArr[1])
	if err != nil {
		return nil, diag.NewErrorDiagnostic("Invalid time format", "Minutes should be a number")
	}
	return &alerts.TimeOfDay{
		Hours:   int32(hours),
		Minutes: int32(minutes),
	}, nil
}

func extractDaysOfWeek(ctx context.Context, daysOfWeek types.List) ([]alerts.DayOfWeek, diag.Diagnostics) {
	var daysOfWeekObjects []types.Object
	diags := daysOfWeek.ElementsAs(ctx, &daysOfWeekObjects, true)
	if diags.HasError() {
		return nil, diags
	}

	var expandedDaysOfWeek []alerts.DayOfWeek
	for _, dow := range daysOfWeekObjects {
		var variable types.String
		if dg := dow.As(ctx, &variable, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedDaysOfWeek = append(expandedDaysOfWeek, daysOfWeekSchemaToProtoMap[variable.ValueString()])
	}

	if diags.HasError() {
		return nil, diags
	}

	return expandedDaysOfWeek, nil

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
	if !(filterModel.LuceneFilter.IsNull() || filterModel.LuceneFilter.IsUnknown()) {
		luceneFilter, diags := extractLuceneFilter(ctx, filterModel.LuceneFilter)
		if diags.HasError() {
			return nil, diags
		}
		logsFilter.FilterType = &alerts.LogsFilter_LuceneFilter{
			LuceneFilter: luceneFilter,
		}
	}

	return logsFilter, nil
}

func extractLuceneFilter(ctx context.Context, luceneFilter types.Object) (*alerts.LuceneFilter, diag.Diagnostics) {
	if luceneFilter.IsNull() || luceneFilter.IsUnknown() {
		return nil, nil
	}

	var luceneFilterModel LuceneFilterModel
	if diags := luceneFilter.As(ctx, &luceneFilterModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	return &alerts.LuceneFilter{}, nil

}

func expandLogsMoreThanAlertTypeDefinition(ctx context.Context, properties *alerts.AlertProperties, than types.Object) (*alerts.AlertProperties, diag.Diagnostics) {
	return nil, nil
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

	alertTypeDefinition, diags := flattenAlertTypeDefinition(alertProperties)
	if diags.HasError() {
		return nil, diags
	}

	return &AlertResourceModel{
		ID:                  wrapperspbStringToTypeString(alert.GetId()),
		Name:                wrapperspbStringToTypeString(alertProperties.GetName()),
		Description:         wrapperspbStringToTypeString(alertProperties.GetDescription()),
		Enabled:             wrapperspbBoolToTypeBool(alertProperties.GetEnabled()),
		AlertPriority:       types.StringValue(alertPriorityProtoToSchemaMap[alertProperties.GetAlertPriority()]),
		AlertSchedule:       alertSchedule,
		AlertTypeDefinition: alertTypeDefinition,
		AlertGroupBys:       wrappedStringSliceToTypeStringList(alertProperties.GetAlertGroupBys()),
		IncidentsSettings:   flattenIncidentsSettings(alert.GetIncidentsSettings()),
		NotificationGroup:   flattenNotificationGroup(alert.GetNotificationGroup()),
		Labels:              types.Map(labels),
	}, nil
}

func flattenAlertTypeDefinition(properties *alerts.AlertProperties) (types.Object, diag.Diagnostics) {
	if properties.AlertTypeDefinition == nil {
		return types.ObjectNull(alertTypeDefinitionAttr()), nil
	}
}

func alertTypeDefinitionAttr() map[string]attr.Type {

}

func flattenAlertSchedule(ctx context.Context, alertProperties *alerts.AlertProperties) (types.Object, diag.Diagnostics) {
	switch alertProperties.AlertSchedule.(type) {
	case *alerts.AlertProperties_ActiveOn:
		activeOn := alertProperties.GetActiveOn()
		daysOfWeek, diags := flattenDaysOfWeek(ctx, activeOn.GetDayOfWeek())
		if diags.HasError() {
			return types.ObjectNull(alertScheduleAttr()), diags
		}
		alertScheduleModel := AlertScheduleModel{
			DaysOfWeek: daysOfWeek,
			StartTime:  wrapperspbStringToTypeString(activeOn.GetStartTime()),
			EndTime:    wrapperspbStringToTypeString(activeOn.GetEndTime()),
		}
		return types.ObjectValueFrom(alertScheduleModel)
	}
}

func alertScheduleAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"days_of_week": types.ListType(types.StringType),
		"start_time":   types.StringType,
		"end_time":     types.StringType,
	}
}

func flattenDaysOfWeek(ctx context.Context, daysOfWeek []alerts.DayOfWeek) (types.List, diag.Diagnostics) {
	var daysOfWeekStrings []types.String
	for _, dow := range daysOfWeek {
		daysOfWeekStrings = append(daysOfWeekStrings, types.StringValue(daysOfWeekProtoToSchemaMap[dow]))
	}
	return types.ListValueFrom(ctx, types.StringType, daysOfWeekStrings)
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
