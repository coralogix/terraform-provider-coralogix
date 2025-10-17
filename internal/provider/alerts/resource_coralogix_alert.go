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

package alerts

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	alertschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/alerts/alert_schema"
	alerttypes "github.com/coralogix/terraform-provider-coralogix/internal/provider/alerts/alert_types"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// Format to parse time from and format to
const TIME_FORMAT = "15:04"

// Format to parse offset from and format to
const OFFSET_FORMAT = "Z0700"

const DEFAULT_TIMEZONE_OFFSET = "+0000"

var (
	_              resource.ResourceWithConfigure   = &AlertResource{}
	_              resource.ResourceWithImportState = &AlertResource{}
	createAlertURL                                  = cxsdk.CreateAlertDefRPC
	updateAlertURL                                  = cxsdk.ReplaceAlertDefRPC
	getAlertURL                                     = cxsdk.GetAlertDefRPC
	deleteAlertURL                                  = cxsdk.DeleteAlertDefRPC
)

func NewAlertResource() resource.Resource {
	return &AlertResource{}
}

type AlertResource struct {
	client *cxsdk.AlertsClient
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
	resp.Schema = alertschema.V3()
}

func (r AlertResource) UpgradeState(_ context.Context) map[int64]resource.StateUpgrader {
	alertSchemaV1 := alertschema.V1()
	alertSchemaV2 := alertschema.V2()
	return map[int64]resource.StateUpgrader{
		1: {
			PriorSchema:   &alertSchemaV1,
			StateUpgrader: r.GenericUpgradeState(alertSchemaV1),
		},
		2: {
			PriorSchema:   &alertSchemaV2,
			StateUpgrader: r.GenericUpgradeState(alertSchemaV2),
		},
	}
}

func (r AlertResource) GenericUpgradeState(_ any) func(context.Context, resource.UpgradeStateRequest, *resource.UpgradeStateResponse) {
	return func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
		// Generic state upgrade, simply fetches the alert again and updates the state
		var state types.String
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &state)...)
		//Get refreshed Alert value from Coralogix
		id := state.ValueString()
		log.Printf("[INFO] Reading Alert: %s", id)
		if resp.Diagnostics.HasError() {
			return
		}

		if id == "" {
			resp.Diagnostics.AddError("Missing ID in prior state", "Upgrade requires the prior state's ID attribute.")
			return
		}

		var schedule types.Object
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("schedule"), &schedule)...)
		if resp.Diagnostics.HasError() {
			return
		}

		getAlertReq := cxsdk.GetAlertDefRequest{Id: wrapperspb.String(id)}
		getAlertResp, err := r.client.Get(ctx, &getAlertReq)
		if err != nil {
			log.Printf("[ERROR] Received error: %s", err.Error())
			if cxsdk.Code(err) == codes.NotFound {
				resp.Diagnostics.AddWarning(
					fmt.Sprintf("Alert %q is in state, but no longer exists in Coralogix backend", id),
					fmt.Sprintf("%s will be recreated when you apply", id),
				)
				resp.State.RemoveResource(ctx)
			} else {
				resp.Diagnostics.AddError(
					"Error reading Alert",
					utils.FormatRpcErrors(err, getAlertURL, protojson.Format(&getAlertReq)),
				)
			}
			return
		}

		alert := getAlertResp.GetAlertDef()
		log.Printf("[INFO] Received Alert for %q: %s", id, protojson.Format(alert))

		newState, diags := flattenAlert(ctx, alert, &schedule)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}

		resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
	}
}
func (r *AlertResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *AlertResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *alerttypes.AlertResourceModel
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
			utils.FormatRpcErrors(err, createAlertURL, protojson.Format(createAlertRequest)),
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

func extractAlertProperties(ctx context.Context, plan *alerttypes.AlertResourceModel) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	groupBy, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, plan.GroupBy.Elements())
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

	labels, diags := utils.TypeMapToStringMap(ctx, plan.Labels)

	if diags.HasError() {
		return nil, diags
	}
	alertProperties := &cxsdk.AlertDefProperties{
		Name:              utils.TypeStringToWrapperspbString(plan.Name),
		Description:       utils.TypeStringToWrapperspbString(plan.Description),
		Enabled:           utils.TypeBoolToWrapperspbBool(plan.Enabled),
		Priority:          alerttypes.AlertPrioritySchemaToProtoMap[plan.Priority.ValueString()],
		GroupByKeys:       groupBy,
		IncidentsSettings: incidentsSettings,
		NotificationGroup: notificationGroup,
		EntityLabels:      labels,
		PhantomMode:       utils.TypeBoolToWrapperspbBool(plan.PhantomMode),
		// Schedule is set in the next step
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

	var incidentsSettingsModel alerttypes.IncidentsSettingsModel
	if diags := incidentsSettingsObject.As(ctx, &incidentsSettingsModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	incidentsSettings := &cxsdk.AlertDefIncidentSettings{
		NotifyOn: alerttypes.NotifyOnSchemaToProtoMap[incidentsSettingsModel.NotifyOn.ValueString()],
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

	var periodModel alerttypes.RetriggeringPeriodModel
	if diags := period.As(ctx, &periodModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if !(periodModel.Minutes.IsNull() || periodModel.Minutes.IsUnknown()) {
		incidentsSettings.RetriggeringPeriod = &cxsdk.AlertDefIncidentSettingsMinutes{
			Minutes: utils.TypeInt64ToWrappedUint32(periodModel.Minutes),
		}
	}

	return incidentsSettings, nil
}

func extractNotificationGroup(ctx context.Context, notificationGroupObject types.Object) (*cxsdk.AlertDefNotificationGroup, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(notificationGroupObject) {
		return nil, nil
	}

	var notificationGroupModel alerttypes.NotificationGroupModel
	if diags := notificationGroupObject.As(ctx, &notificationGroupModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	groupByFields, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, notificationGroupModel.GroupByKeys.Elements())
	if diags.HasError() {
		return nil, diags
	}
	webhooks, diags := extractWebhooksSettings(ctx, notificationGroupModel.WebhooksSettings)
	if diags.HasError() {
		return nil, diags
	}
	destinations, diags := extractDestinations(ctx, notificationGroupModel.Destinations)
	if diags.HasError() {
		return nil, diags
	}
	router, diags := extractNotificationRouter(ctx, notificationGroupModel.Router)
	if diags.HasError() {
		return nil, diags
	}
	notificationGroup := &cxsdk.AlertDefNotificationGroup{
		Destinations: destinations,
		Router:       router,
		GroupByKeys:  groupByFields,
		Webhooks:     webhooks,
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
		var webhooksSettingsModel alerttypes.WebhooksSettingsModel
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

func extractDestinations(ctx context.Context, notificationDestinations types.List) ([]*cxsdk.NotificationDestination, diag.Diagnostics) {
	if notificationDestinations.IsNull() || notificationDestinations.IsUnknown() {
		return nil, nil
	}

	var notificationDestinationsObject []types.Object
	diags := notificationDestinations.ElementsAs(ctx, &notificationDestinationsObject, true)
	if diags.HasError() {
		return nil, diags
	}
	var expandedDestinations []*cxsdk.NotificationDestination
	for _, destination := range notificationDestinationsObject {
		var destinationModel alerttypes.NotificationDestinationModel
		if diags := destination.As(ctx, &destinationModel, basetypes.ObjectAsOptions{}); diags.HasError() {
			return nil, diags
		}
		presetId := destinationModel.PresetId.ValueString()
		triggeredRoutingOverrides, diags := extractRoutingOverrides(ctx, destinationModel.TriggeredRoutingOverrides)
		if diags.HasError() {
			return nil, diags
		}
		resolvedRoutingOverrides, diags := extractRoutingOverrides(ctx, destinationModel.ResolvedRoutingOverrides)
		if diags.HasError() {
			return nil, diags
		}
		destination := &cxsdk.NotificationDestination{
			ConnectorId: destinationModel.ConnectorId.ValueString(),
			PresetId:    &presetId,
			NotifyOn:    alerttypes.NotifyOnSchemaToProtoMap[destinationModel.NotifyOn.ValueString()],
			TriggeredRoutingOverrides: &cxsdk.NotificationRouting{
				ConfigOverrides: triggeredRoutingOverrides,
			},
			ResolvedRouteOverrides: &cxsdk.NotificationRouting{
				ConfigOverrides: resolvedRoutingOverrides,
			},
		}
		expandedDestinations = append(expandedDestinations, destination)
	}

	return expandedDestinations, nil
}

func extractRoutingOverrides(ctx context.Context, overridesObject types.Object) (*cxsdk.SourceOverrides, diag.Diagnostics) {
	if overridesObject.IsNull() || overridesObject.IsUnknown() {
		return nil, nil
	}

	var routingOverridesModel alerttypes.SourceOverridesModel
	if diags := overridesObject.As(ctx, &routingOverridesModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}
	connectorOverrides, diags := extractConnectorOverrides(ctx, routingOverridesModel.ConnectorOverrides)
	if diags.HasError() {
		return nil, diags
	}
	presetOverrides, diags := extractPresetOverrides(ctx, routingOverridesModel.PresetOverrides)
	if diags.HasError() {
		return nil, diags
	}
	sourceOverrides := &cxsdk.SourceOverrides{
		ConnectorConfigFields: connectorOverrides,
		MessageConfigFields:   presetOverrides,
		PayloadType:           routingOverridesModel.PayloadType.ValueString(),
	}

	return sourceOverrides, nil
}

func extractConnectorOverrides(ctx context.Context, overridesObject types.List) ([]*cxsdk.ConnectorOverride, diag.Diagnostics) {
	if overridesObject.IsNull() || overridesObject.IsUnknown() {
		return nil, nil
	}

	var configurationOverridesModel []types.Object
	diags := overridesObject.ElementsAs(ctx, &configurationOverridesModel, true)
	if diags.HasError() {
		return nil, diags
	}
	var connectorOverrides []*cxsdk.ConnectorOverride
	for _, override := range configurationOverridesModel {
		var connectorOverrideModel alerttypes.ConfigurationOverrideModel
		if diags := override.As(ctx, &connectorOverrideModel, basetypes.ObjectAsOptions{}); diags.HasError() {
			return nil, diags
		}
		connectorOverride := &cxsdk.ConnectorOverride{
			FieldName: connectorOverrideModel.FieldName.ValueString(),
			Template:  connectorOverrideModel.Template.ValueString(),
		}
		connectorOverrides = append(connectorOverrides, connectorOverride)
	}

	return connectorOverrides, nil
}

func extractPresetOverrides(ctx context.Context, overridesObject types.List) ([]*cxsdk.PresetOverride, diag.Diagnostics) {
	if overridesObject.IsNull() || overridesObject.IsUnknown() {
		return nil, nil
	}

	var configurationOverridesModel []types.Object
	diags := overridesObject.ElementsAs(ctx, &configurationOverridesModel, true)
	if diags.HasError() {
		return nil, diags
	}
	var connectorOverrides []*cxsdk.PresetOverride
	for _, override := range configurationOverridesModel {
		var connectorOverrideModel alerttypes.ConfigurationOverrideModel
		if diags := override.As(ctx, &connectorOverrideModel, basetypes.ObjectAsOptions{}); diags.HasError() {
			return nil, diags
		}
		connectorOverride := &cxsdk.PresetOverride{
			FieldName: connectorOverrideModel.FieldName.ValueString(),
			Template:  connectorOverrideModel.Template.ValueString(),
		}
		connectorOverrides = append(connectorOverrides, connectorOverride)
	}

	return connectorOverrides, nil
}

func extractNotificationRouter(ctx context.Context, routerObject types.Object) (*cxsdk.NotificationRouter, diag.Diagnostics) {
	if routerObject.IsNull() || routerObject.IsUnknown() {
		return nil, nil
	}

	var routerModel alerttypes.NotificationRouterModel
	if diags := routerObject.As(ctx, &routerModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	notifyOn := alerttypes.NotifyOnSchemaToProtoMap[routerModel.NotifyOn.ValueString()]

	router := &cxsdk.NotificationRouter{
		Id:       "router_default",
		NotifyOn: &notifyOn,
	}

	return router, nil
}

func extractAdvancedTargetSetting(ctx context.Context, webhooksSettingsModel alerttypes.WebhooksSettingsModel) (*cxsdk.AlertDefWebhooksSettings, diag.Diagnostics) {
	notifyOn := alerttypes.NotifyOnSchemaToProtoMap[webhooksSettingsModel.NotifyOn.ValueString()]
	advancedTargetSettings := &cxsdk.AlertDefWebhooksSettings{
		NotifyOn: &notifyOn,
	}
	advancedTargetSettings, diags := expandAlertNotificationByRetriggeringPeriod(ctx, advancedTargetSettings, webhooksSettingsModel.RetriggeringPeriod)
	if diags.HasError() {
		return nil, diags
	}

	if !webhooksSettingsModel.IntegrationID.IsNull() && !webhooksSettingsModel.IntegrationID.IsUnknown() {
		integrationId, diag := utils.TypeStringToWrapperspbUint32(webhooksSettingsModel.IntegrationID)
		if diag.HasError() {
			return nil, diag
		}
		advancedTargetSettings.Integration = &cxsdk.AlertDefIntegrationType{
			IntegrationType: &cxsdk.AlertDefIntegrationTypeIntegrationID{
				IntegrationId: integrationId,
			},
		}
	} else if !webhooksSettingsModel.Recipients.IsNull() && !webhooksSettingsModel.Recipients.IsUnknown() {
		emails, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, webhooksSettingsModel.Recipients.Elements())
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
	if utils.ObjIsNullOrUnknown(period) {
		return alertNotification, nil
	}

	var periodModel alerttypes.RetriggeringPeriodModel
	if diags := period.As(ctx, &periodModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if !(periodModel.Minutes.IsNull() || periodModel.Minutes.IsUnknown()) {
		alertNotification.RetriggeringPeriod = &cxsdk.AlertDefWebhooksSettingsMinutes{
			Minutes: utils.TypeInt64ToWrappedUint32(periodModel.Minutes),
		}
	}

	return alertNotification, nil
}

func expandAlertsSchedule(ctx context.Context, alertProperties *cxsdk.AlertDefProperties, scheduleObject types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(scheduleObject) {
		return alertProperties, nil
	}

	var scheduleModel alerttypes.AlertScheduleModel
	if diags := scheduleObject.As(ctx, &scheduleModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	var diags diag.Diagnostics
	if activeOn := scheduleModel.ActiveOn; !utils.ObjIsNullOrUnknown(activeOn) {
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
	if utils.ObjIsNullOrUnknown(activeOnObject) {
		return nil, nil
	}

	var activeOnModel alerttypes.ActiveOnModel
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

	startTime, e := time.ParseInLocation(TIME_FORMAT, activeOnModel.StartTime.ValueString(), location)
	if e != nil {
		diags.AddError("Failed to parse start time", e.Error())
	}

	endTime, e := time.ParseInLocation(TIME_FORMAT, activeOnModel.EndTime.ValueString(), location)
	if e != nil {
		diags.AddError("Failed to parse end time", e.Error())
	}
	if endTime.Before(startTime) {
		diags.AddError("End time is before start time", "End time is before start time")
	}

	if diags.HasError() {
		return nil, diags
	}

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
		result = append(result, alerttypes.DaysOfWeekSchemaToProtoMap[str])
	}
	return result, diags
}

func expandAlertsTypeDefinition(ctx context.Context, alertProperties *cxsdk.AlertDefProperties, alertDefinition types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(alertDefinition) {
		return alertProperties, nil
	}

	var alertDefinitionModel alerttypes.AlertTypeDefinitionModel
	if diags := alertDefinition.As(ctx, &alertDefinitionModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	var diags diag.Diagnostics

	if logsImmediate := alertDefinitionModel.LogsImmediate; !utils.ObjIsNullOrUnknown(logsImmediate) {
		// LogsImmediate
		alertProperties, diags = expandLogsImmediateAlertTypeDefinition(ctx, alertProperties, logsImmediate)
	} else if logsThreshold := alertDefinitionModel.LogsThreshold; !utils.ObjIsNullOrUnknown(logsThreshold) {
		// LogsThreshold
		alertProperties, diags = expandLogsThresholdTypeDefinition(ctx, alertProperties, logsThreshold)
	} else if logsAnomaly := alertDefinitionModel.LogsAnomaly; !utils.ObjIsNullOrUnknown(logsAnomaly) {
		// LogsAnomaly
		alertProperties, diags = expandLogsAnomalyAlertTypeDefinition(ctx, alertProperties, logsAnomaly)
	} else if logsRatioThreshold := alertDefinitionModel.LogsRatioThreshold; !utils.ObjIsNullOrUnknown(logsRatioThreshold) {
		// LogsRatioThreshold
		alertProperties, diags = expandLogsRatioThresholdTypeDefinition(ctx, alertProperties, logsRatioThreshold)
	} else if logsNewValue := alertDefinitionModel.LogsNewValue; !utils.ObjIsNullOrUnknown(logsNewValue) {
		// LogsNewValue
		alertProperties, diags = expandLogsNewValueAlertTypeDefinition(ctx, alertProperties, logsNewValue)
	} else if logsUniqueCount := alertDefinitionModel.LogsUniqueCount; !utils.ObjIsNullOrUnknown(logsUniqueCount) {
		// LogsUniqueCount
		alertProperties, diags = expandLogsUniqueCountAlertTypeDefinition(ctx, alertProperties, logsUniqueCount)
	} else if logsTimeRelativeThreshold := alertDefinitionModel.LogsTimeRelativeThreshold; !utils.ObjIsNullOrUnknown(logsTimeRelativeThreshold) {
		// LogsTimeRelativeThreshold
		alertProperties, diags = expandLogsTimeRelativeThresholdAlertTypeDefinition(ctx, alertProperties, logsTimeRelativeThreshold)
	} else if metricThreshold := alertDefinitionModel.MetricThreshold; !utils.ObjIsNullOrUnknown(metricThreshold) {
		// MetricsThreshold
		alertProperties, diags = expandMetricThresholdAlertTypeDefinition(ctx, alertProperties, metricThreshold)
	} else if metricAnomaly := alertDefinitionModel.MetricAnomaly; !utils.ObjIsNullOrUnknown(metricAnomaly) {
		// MetricsAnomaly
		alertProperties, diags = expandMetricAnomalyAlertTypeDefinition(ctx, alertProperties, metricAnomaly)
	} else if tracingImmediate := alertDefinitionModel.TracingImmediate; !utils.ObjIsNullOrUnknown(tracingImmediate) {
		// TracingImmediate
		alertProperties, diags = expandTracingImmediateTypeDefinition(ctx, alertProperties, tracingImmediate)
	} else if tracingThreshold := alertDefinitionModel.TracingThreshold; !utils.ObjIsNullOrUnknown(tracingThreshold) {
		// TracingThreshold
		alertProperties, diags = expandTracingThresholdTypeDefinition(ctx, alertProperties, tracingThreshold)
	} else if flow := alertDefinitionModel.Flow; !utils.ObjIsNullOrUnknown(flow) {
		// Flow
		alertProperties, diags = expandFlowAlertTypeDefinition(ctx, alertProperties, flow)
	} else if sloThreshold := alertDefinitionModel.SloThreshold; !utils.ObjIsNullOrUnknown(sloThreshold) {
		// SLOThreshold
		alertProperties, diags = expandSloThresholdAlertTypeDefinition(ctx, alertProperties, sloThreshold)
	} else {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Alert Type Definition", "Alert Type Definition is not valid")}
	}

	if diags.HasError() {
		return nil, diags
	}

	return alertProperties, nil
}

func expandLogsImmediateAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, logsImmediateObject types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(logsImmediateObject) {
		return properties, nil
	}

	var immediateModel alerttypes.LogsImmediateModel
	if diags := logsImmediateObject.As(ctx, &immediateModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	logsFilter, diags := extractLogsFilter(ctx, immediateModel.LogsFilter)
	if diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, immediateModel.NotificationPayloadFilter.Elements())
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

	var filterModel alerttypes.AlertsLogsFilterModel
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

	var luceneFilterModel alerttypes.SimpleFilterModel
	if diags := luceneFilter.As(ctx, &luceneFilterModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	labelFilters, diags := extractLabelFilters(ctx, luceneFilterModel.LabelFilters)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.LogsFilterSimpleFilter{
		SimpleFilter: &cxsdk.SimpleFilter{
			LuceneQuery:  utils.TypeStringToWrapperspbString(luceneFilterModel.LuceneQuery),
			LabelFilters: labelFilters,
		},
	}, nil
}

func extractLabelFilters(ctx context.Context, filters types.Object) (*cxsdk.LabelFilters, diag.Diagnostics) {
	if filters.IsNull() || filters.IsUnknown() {
		return nil, nil
	}

	var filtersModel alerttypes.LabelFiltersModel
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
		var labelFilterTypeModel alerttypes.LabelFilterTypeModel
		if dg := lft.As(ctx, &labelFilterTypeModel, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedLabelFilterType := &cxsdk.LabelFilterType{
			Value:     utils.TypeStringToWrapperspbString(labelFilterTypeModel.Value),
			Operation: alerttypes.LogFilterOperationTypeSchemaToProtoMap[labelFilterTypeModel.Operation.ValueString()],
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
		result = append(result, alerttypes.LogSeveritySchemaToProtoMap[str])
	}
	return result, diags
}

func expandLogsThresholdTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, thresholdObject types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(thresholdObject) {
		return properties, nil
	}

	var thresholdModel alerttypes.LogsThresholdModel
	if diags := thresholdObject.As(ctx, &thresholdModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	logsFilter, diags := extractLogsFilter(ctx, thresholdModel.LogsFilter)
	if diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, thresholdModel.NotificationPayloadFilter.Elements())
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
			EvaluationDelayMs:          wrapperspb.Int32(thresholdModel.CustomEvaluationDelay.ValueInt32()),
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
		var rule alerttypes.LogsThresholdRuleModel
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

	var conditionModel alerttypes.LogsThresholdConditionModel
	if diags := condition.As(ctx, &conditionModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	return &cxsdk.LogsThresholdCondition{
		Threshold: utils.TypeFloat64ToWrapperspbDouble(conditionModel.Threshold),
		TimeWindow: &cxsdk.LogsTimeWindow{
			Type: &cxsdk.LogsTimeWindowSpecificValue{
				LogsTimeWindowSpecificValue: alerttypes.LogsTimeWindowValueSchemaToProtoMap[conditionModel.TimeWindow.ValueString()],
			},
		},
		ConditionType: alerttypes.LogsThresholdConditionToProtoMap[conditionModel.ConditionType.ValueString()],
	}, nil
}

func extractUndetectedValuesManagement(ctx context.Context, management types.Object) (*cxsdk.UndetectedValuesManagement, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(management) {
		return nil, nil
	}
	var managementModel alerttypes.UndetectedValuesManagementModel
	if diags := management.As(ctx, &managementModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if (managementModel.AutoRetireTimeframe.IsNull() || managementModel.AutoRetireTimeframe.IsUnknown()) && (managementModel.TriggerUndetectedValues.IsNull() || managementModel.TriggerUndetectedValues.IsUnknown()) {
		return nil, nil
	}

	var autoRetireTimeframe *cxsdk.AutoRetireTimeframe
	if !(managementModel.AutoRetireTimeframe.IsNull() || managementModel.AutoRetireTimeframe.IsUnknown()) {
		autoRetireTimeframe = new(cxsdk.AutoRetireTimeframe)
		*autoRetireTimeframe = alerttypes.AutoRetireTimeframeSchemaToProtoMap[managementModel.AutoRetireTimeframe.ValueString()]
	}

	return &cxsdk.UndetectedValuesManagement{
		TriggerUndetectedValues: utils.TypeBoolToWrapperspbBool(managementModel.TriggerUndetectedValues),
		AutoRetireTimeframe:     autoRetireTimeframe,
	}, nil
}

func expandLogsAnomalyAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, anomaly types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(anomaly) {
		return properties, nil
	}

	var anomalyModel alerttypes.LogsAnomalyModel
	if diags := anomaly.As(ctx, &anomalyModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	logsFilter, diags := extractLogsFilter(ctx, anomalyModel.LogsFilter)
	if diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, anomalyModel.NotificationPayloadFilter.Elements())
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
			EvaluationDelayMs:         wrapperspb.Int32(anomalyModel.CustomEvaluationDelay.ValueInt32()),
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
		var rule alerttypes.LogsAnomalyRuleModel
		if dg := r.As(ctx, &rule, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}

		var condition alerttypes.LogsAnomalyConditionModel
		if dg := rule.Condition.As(ctx, &condition, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}

		rules[i] = &cxsdk.LogsAnomalyRule{
			Condition: &cxsdk.LogsAnomalyCondition{
				MinimumThreshold: utils.TypeFloat64ToWrapperspbDouble(condition.MinimumThreshold),
				TimeWindow: &cxsdk.LogsTimeWindow{
					Type: &cxsdk.LogsTimeWindowSpecificValue{
						LogsTimeWindowSpecificValue: alerttypes.LogsTimeWindowValueSchemaToProtoMap[condition.TimeWindow.ValueString()],
					},
				},
				ConditionType: alerttypes.LogsAnomalyConditionSchemaToProtoMap[condition.ConditionType.ValueString()],
			},
		}
	}
	if diags.HasError() {
		return nil, diags
	}
	return rules, nil
}

func expandLogsRatioThresholdTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, ratioThreshold types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(ratioThreshold) {
		return properties, nil
	}

	var ratioThresholdModel alerttypes.LogsRatioThresholdModel
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
	notificationPayloadFilter, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, ratioThresholdModel.NotificationPayloadFilter.Elements())
	if diags.HasError() {
		return nil, diags
	}

	properties.TypeDefinition = &cxsdk.AlertDefPropertiesLogsRatioThreshold{
		LogsRatioThreshold: &cxsdk.LogsRatioThresholdType{
			Numerator:                 numeratorLogsFilter,
			NumeratorAlias:            utils.TypeStringToWrapperspbString(ratioThresholdModel.NumeratorAlias),
			Denominator:               denominatorLogsFilter,
			DenominatorAlias:          utils.TypeStringToWrapperspbString(ratioThresholdModel.DenominatorAlias),
			Rules:                     rules,
			NotificationPayloadFilter: notificationPayloadFilter,
			GroupByFor:                alerttypes.LogsRatioGroupByForSchemaToProtoMap[ratioThresholdModel.GroupByFor.ValueString()],
			EvaluationDelayMs:         wrapperspb.Int32(ratioThresholdModel.CustomEvaluationDelay.ValueInt32()),
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
		var rule alerttypes.LogsRatioThresholdRuleModel
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
	if utils.ObjIsNullOrUnknown(override) {
		return nil, nil
	}

	var overrideModel alerttypes.AlertOverrideModel
	if diags := override.As(ctx, &overrideModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	return &cxsdk.AlertDefPriorityOverride{
		Priority: alerttypes.AlertPrioritySchemaToProtoMap[overrideModel.Priority.ValueString()],
	}, nil
}

func extractLogsRatioCondition(ctx context.Context, condition types.Object) (*cxsdk.LogsRatioCondition, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(condition) {
		return nil, nil
	}

	var conditionModel alerttypes.LogsRatioConditionModel
	if diags := condition.As(ctx, &conditionModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	return &cxsdk.LogsRatioCondition{
		Threshold: utils.TypeFloat64ToWrapperspbDouble(conditionModel.Threshold),
		TimeWindow: &cxsdk.LogsRatioTimeWindow{
			Type: &cxsdk.LogsRatioTimeWindowSpecificValue{
				LogsRatioTimeWindowSpecificValue: alerttypes.LogsRatioTimeWindowValueSchemaToProtoMap[conditionModel.TimeWindow.ValueString()],
			},
		},
		ConditionType: alerttypes.LogsRatioConditionSchemaToProtoMap[conditionModel.ConditionType.ValueString()],
	}, nil
}

func expandLogsNewValueAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, newValue types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if newValue.IsNull() || newValue.IsUnknown() {
		return properties, nil
	}

	var newValueModel alerttypes.LogsNewValueModel
	if diags := newValue.As(ctx, &newValueModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	logsFilter, diags := extractLogsFilter(ctx, newValueModel.LogsFilter)
	if diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, newValueModel.NotificationPayloadFilter.Elements())
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
		var rule alerttypes.NewValueRuleModel
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

	var conditionModel alerttypes.NewValueConditionModel
	if diags := condition.As(ctx, &conditionModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	return &cxsdk.LogsNewValueCondition{
		KeypathToTrack: utils.TypeStringToWrapperspbString(conditionModel.KeypathToTrack),
		TimeWindow: &cxsdk.LogsNewValueTimeWindow{
			Type: &cxsdk.LogsNewValueTimeWindowSpecificValue{
				LogsNewValueTimeWindowSpecificValue: alerttypes.LogsNewValueTimeWindowValueSchemaToProtoMap[conditionModel.TimeWindow.ValueString()],
			},
		},
	}, nil
}

func expandLogsUniqueCountAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, uniqueCount types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(uniqueCount) {
		return properties, nil
	}

	var uniqueCountModel alerttypes.LogsUniqueCountModel
	if diags := uniqueCount.As(ctx, &uniqueCountModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	logsFilter, diags := extractLogsFilter(ctx, uniqueCountModel.LogsFilter)
	if diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, uniqueCountModel.NotificationPayloadFilter.Elements())
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
			MaxUniqueCountPerGroupByKey: utils.TypeInt64ToWrappedInt64(uniqueCountModel.MaxUniqueCountPerGroupByKey),
			UniqueCountKeypath:          utils.TypeStringToWrapperspbString(uniqueCountModel.UniqueCountKeypath),
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
		var rule alerttypes.LogsUniqueCountRuleModel
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
	if utils.ObjIsNullOrUnknown(condition) {
		return nil, nil
	}

	var conditionModel alerttypes.LogsUniqueCountConditionModel
	if diags := condition.As(ctx, &conditionModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	return &cxsdk.LogsUniqueCountCondition{
		MaxUniqueCount: utils.TypeInt64ToWrappedInt64(conditionModel.MaxUniqueCount),
		TimeWindow: &cxsdk.LogsUniqueValueTimeWindow{
			Type: &cxsdk.LogsUniqueValueTimeWindowSpecificValue{
				LogsUniqueValueTimeWindowSpecificValue: alerttypes.LogsUniqueCountTimeWindowValueSchemaToProtoMap[conditionModel.TimeWindow.ValueString()],
			},
		},
	}, nil
}

func expandLogsTimeRelativeThresholdAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, relativeThreshold types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(relativeThreshold) {
		return properties, nil
	}

	var relativeThresholdModel alerttypes.LogsTimeRelativeThresholdModel
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
	notificationPayloadFilter, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, relativeThresholdModel.NotificationPayloadFilter.Elements())
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
			EvaluationDelayMs:          wrapperspb.Int32(relativeThresholdModel.CustomEvaluationDelay.ValueInt32()),
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
		var rule alerttypes.LogsTimeRelativeRuleModel
		if dg := r.As(ctx, &rule, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}

		var condition alerttypes.LogsTimeRelativeConditionModel
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
				Threshold:     utils.TypeFloat64ToWrapperspbDouble(condition.Threshold),
				ComparedTo:    alerttypes.LogsTimeRelativeComparedToSchemaToProtoMap[condition.ComparedTo.ValueString()],
				ConditionType: alerttypes.LogsTimeRelativeConditionToProtoMap[condition.ConditionType.ValueString()],
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
	if utils.ObjIsNullOrUnknown(metricThreshold) {
		return properties, nil
	}

	var metricThresholdModel alerttypes.MetricThresholdModel
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
			EvaluationDelayMs:          wrapperspb.Int32(metricThresholdModel.CustomEvaluationDelay.ValueInt32()),
		},
	}
	properties.Type = cxsdk.AlertDefTypeMetricThreshold

	return properties, nil
}

func extractMetricThresholdMissingValues(ctx context.Context, values types.Object) (*cxsdk.MetricMissingValues, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(values) {
		return nil, nil
	}

	var valuesModel alerttypes.MissingValuesModel
	if diags := values.As(ctx, &valuesModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if replaceWithZero := valuesModel.ReplaceWithZero; !(replaceWithZero.IsNull() || replaceWithZero.IsUnknown()) {
		return &cxsdk.MetricMissingValues{
			MissingValues: &cxsdk.MetricMissingValuesReplaceWithZero{
				ReplaceWithZero: utils.TypeBoolToWrapperspbBool(replaceWithZero),
			},
		}, nil
	} else if retainMissingValues := valuesModel.MinNonNullValuesPct; !(retainMissingValues.IsNull() || retainMissingValues.IsUnknown()) {
		return &cxsdk.MetricMissingValues{
			MissingValues: &cxsdk.MetricMissingValuesMinNonNullValuesPct{
				MinNonNullValuesPct: utils.TypeInt64ToWrappedUint32(retainMissingValues),
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
		var rule alerttypes.MetricThresholdRuleModel
		if dg := r.As(ctx, &rule, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}

		var condition alerttypes.MetricThresholdConditionModel
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
				Threshold:     utils.TypeFloat64ToWrapperspbDouble(condition.Threshold),
				ForOverPct:    utils.TypeInt64ToWrappedUint32(condition.ForOverPct),
				OfTheLast:     expandMetricTimeWindow(condition.OfTheLast),
				ConditionType: alerttypes.MetricsThresholdConditionToProtoMap[condition.ConditionType.ValueString()],
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
	if utils.ObjIsNullOrUnknown(filter) {
		return nil, nil
	}

	var filterModel alerttypes.MetricFilterModel
	if diags := filter.As(ctx, &filterModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if promql := filterModel.Promql; !(promql.IsNull() || promql.IsUnknown()) {
		return &cxsdk.MetricFilter{
			Type: &cxsdk.MetricFilterPromql{
				Promql: utils.TypeStringToWrapperspbString(promql),
			},
		}, nil
	}

	return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Metric Filter", "Metric Filter is not valid")}
}

func expandTracingImmediateTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, tracingImmediate types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(tracingImmediate) {
		return properties, nil
	}

	var tracingImmediateModel alerttypes.TracingImmediateModel
	if diags := tracingImmediate.As(ctx, &tracingImmediateModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	tracingQuery, diags := expandTracingFilters(ctx, tracingImmediateModel.TracingFilter)
	if diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, tracingImmediateModel.NotificationPayloadFilter.Elements())
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
	if utils.ObjIsNullOrUnknown(tracingThreshold) {
		return properties, nil
	}

	var tracingThresholdModel alerttypes.TracingThresholdModel
	if diags := tracingThreshold.As(ctx, &tracingThresholdModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	tracingQuery, diags := expandTracingFilters(ctx, tracingThresholdModel.TracingFilter)
	if diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, tracingThresholdModel.NotificationPayloadFilter.Elements())
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
		var rule alerttypes.TracingThresholdRuleModel
		if dg := r.As(ctx, &rule, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}

		var condition alerttypes.TracingThresholdConditionModel
		if dg := rule.Condition.As(ctx, &condition, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}

		rules[i] = &cxsdk.TracingThresholdRule{
			Condition: &cxsdk.TracingThresholdCondition{
				SpanAmount: utils.TypeFloat64ToWrapperspbDouble(condition.SpanAmount),
				TimeWindow: &cxsdk.TracingTimeWindow{
					Type: &cxsdk.TracingTimeWindowSpecificValue{
						TracingTimeWindowValue: alerttypes.TracingTimeWindowSchemaToProtoMap[condition.TimeWindow.ValueString()],
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
	if utils.ObjIsNullOrUnknown(query) {
		return nil, nil
	}
	var labelFilterModel alerttypes.TracingFilterModel
	if diags := query.As(ctx, &labelFilterModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	var filtersModel alerttypes.TracingLabelFiltersModel
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
			LatencyThresholdMs: utils.NumberTypeToWrapperspbUInt64(labelFilterModel.LatencyThresholdMs),
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
	var filterModel alerttypes.TracingFilterTypeModel
	if diags := filterModelObject.As(ctx, &filterModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	values, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, filterModel.Values.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.TracingFilterType{
		Values:    values,
		Operation: alerttypes.TracingFilterOperationSchemaToProtoMap[filterModel.Operation.ValueString()],
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
		var filterModel alerttypes.TracingSpanFieldsFilterModel
		if diags := element.As(ctx, &filterModel, basetypes.ObjectAsOptions{}); diags.HasError() {
			return nil, diags
		}

		filterType, diags := extractTracingLabelFilter(ctx, filterModel.FilterType)
		if diags.HasError() {
			return nil, diags
		}

		filters = append(filters, &cxsdk.TracingSpanFieldsFilterType{
			Key:        utils.TypeStringToWrapperspbString(filterModel.Key),
			FilterType: filterType,
		})
	}

	return filters, nil
}

func expandMetricAnomalyAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, metricAnomaly types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(metricAnomaly) {
		return properties, nil
	}

	var metricAnomalyModel alerttypes.MetricAnomalyModel
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
			MetricFilter:      metricFilter,
			Rules:             rules,
			EvaluationDelayMs: wrapperspb.Int32(metricAnomalyModel.CustomEvaluationDelay.ValueInt32()),
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
		var rule alerttypes.MetricAnomalyRuleModel
		if dg := r.As(ctx, &rule, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}

		var condition alerttypes.MetricAnomalyConditionModel
		if dg := rule.Condition.As(ctx, &condition, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}

		rules[i] = &cxsdk.MetricAnomalyRule{
			Condition: &cxsdk.MetricAnomalyCondition{
				Threshold:  utils.TypeFloat64ToWrapperspbDouble(condition.Threshold),
				ForOverPct: utils.TypeInt64ToWrappedUint32(condition.ForOverPct),
				OfTheLast: &cxsdk.MetricTimeWindow{
					Type: &cxsdk.MetricTimeWindowSpecificValue{
						MetricTimeWindowSpecificValue: alerttypes.MetricTimeWindowValueSchemaToProtoMap[condition.OfTheLast.ValueString()],
					},
				},
				ConditionType:       alerttypes.MetricAnomalyConditionToProtoMap[condition.ConditionType.ValueString()],
				MinNonNullValuesPct: utils.TypeInt64ToWrappedUint32(condition.MinNonNullValuesPct),
			},
		}
	}
	if diags.HasError() {
		return nil, diags
	}
	return rules, nil
}

func expandMetricTimeWindow(metricTimeWindow types.String) *cxsdk.MetricTimeWindow {
	if metricTimeWindow.IsNull() || metricTimeWindow.IsUnknown() {
		return nil
	}

	timeWindowStr := metricTimeWindow.ValueString()
	if timeWindowVal, ok := alerttypes.MetricTimeWindowValueSchemaToProtoMap[timeWindowStr]; ok {
		return &cxsdk.MetricTimeWindow{
			Type: &cxsdk.MetricTimeWindowSpecificValue{
				MetricTimeWindowSpecificValue: timeWindowVal,
			},
		}
	} else {
		return &cxsdk.MetricTimeWindow{
			Type: &cxsdk.MetricTimeWindowDynamicDuration{
				MetricTimeWindowDynamicDuration: wrapperspb.String(timeWindowStr),
			},
		}
	}
}

func expandFlowAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, flow types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(flow) {
		return properties, nil
	}

	var flowModel alerttypes.FlowModel
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
			EnforceSuppression: utils.TypeBoolToWrapperspbBool(flowModel.EnforceSuppression),
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
	var stageModel alerttypes.FlowStageModel
	if diags := object.As(ctx, &stageModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	flowStage := &cxsdk.FlowStages{
		TimeframeMs:   utils.TypeInt64ToWrappedInt64(stageModel.TimeframeMs),
		TimeframeType: alerttypes.FlowStageTimeFrameTypeSchemaToProtoMap[stageModel.TimeframeType.ValueString()],
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
	var groupModel alerttypes.FlowStagesGroupModel
	if diags := object.As(ctx, &groupModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	alertDefs, diags := extractAlertDefs(ctx, groupModel.AlertDefs)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.FlowStagesGroup{
		AlertDefs: alertDefs,
		NextOp:    alerttypes.FlowStagesGroupNextOpSchemaToProtoMap[groupModel.NextOp.ValueString()],
		AlertsOp:  alerttypes.FlowStagesGroupAlertsOpSchemaToProtoMap[groupModel.AlertsOp.ValueString()],
	}, nil

}

func expandSloThresholdAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties, sloThreshold types.Object) (*cxsdk.AlertDefProperties, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(sloThreshold) {
		return properties, nil
	}

	var model alerttypes.SloThresholdModel
	if diags := sloThreshold.As(ctx, &model, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	sloDef, diags := extractSloDefinition(ctx, model.SloDefinition)
	if diags.HasError() {
		return nil, diags
	}

	sloThresholdType := &cxsdk.SloThresholdType{
		SloDefinition: sloDef,
	}

	if !utils.ObjIsNullOrUnknown(model.ErrorBudget) {
		errorBudget, diags := extractSloErrorBudgetThreshold(ctx, model.ErrorBudget)
		if diags.HasError() {
			return nil, diags
		}
		sloThresholdType.Threshold = &cxsdk.SloErrorBudgetThresholdType{ErrorBudget: errorBudget}
	} else if !utils.ObjIsNullOrUnknown(model.BurnRate) {
		burnRate, diags := extractSloBurnRateThreshold(ctx, model.BurnRate)
		if diags.HasError() {
			return nil, diags
		}
		sloThresholdType.Threshold = &cxsdk.SloBurnRateThresholdType{BurnRate: burnRate}
	} else {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid SLO Threshold Type", "SLO Threshold must have either ErrorBudget or BurnRate defined")}
	}

	properties.TypeDefinition = &cxsdk.AlertDefPropertiesSlo{
		SloThreshold: sloThresholdType,
	}
	properties.Type = cxsdk.AlertDefTypeSloThreshold
	return properties, nil
}

func extractSloDefinition(ctx context.Context, obj types.Object) (*cxsdk.AlertSloDefinition, diag.Diagnostics) {
	var model alerttypes.SloDefinitionObject
	if diags := obj.As(ctx, &model, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	return &cxsdk.AlertSloDefinition{
		SloId: wrapperspb.String(model.SloId.ValueString()),
	}, nil
}

func extractSloErrorBudgetThreshold(ctx context.Context, obj types.Object) (*cxsdk.SloErrorBudgetThreshold, diag.Diagnostics) {
	var model alerttypes.SloThresholdErrorBudgetModel
	if diags := obj.As(ctx, &model, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	rules, diags := extractSloThresholdRules(ctx, model.Rules)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.SloErrorBudgetThreshold{Rules: rules}, nil
}

func extractSloBurnRateThreshold(ctx context.Context, obj types.Object) (*cxsdk.SloBurnRateThreshold, diag.Diagnostics) {
	var model alerttypes.SloThresholdBurnRateModel
	if diags := obj.As(ctx, &model, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	rules, diags := extractSloThresholdRules(ctx, model.Rules)
	if diags.HasError() {
		return nil, diags
	}

	burnRate := &cxsdk.SloBurnRateThreshold{Rules: rules}

	if !utils.ObjIsNullOrUnknown(model.Dual) {
		timeDuration, diags := extractSloTimeDuration(ctx, model.Dual)
		if diags.HasError() {
			return nil, diags
		}
		burnRate.Type = &cxsdk.DualBurnRateThresholdType{Dual: &cxsdk.DualBurnRateThreshold{TimeDuration: timeDuration}}
	} else if !utils.ObjIsNullOrUnknown(model.Single) {
		timeDuration, diags := extractSloTimeDuration(ctx, model.Single)
		if diags.HasError() {
			return nil, diags
		}
		burnRate.Type = &cxsdk.SingleBurnRateThresholdType{Single: &cxsdk.SingleBurnRateThreshold{TimeDuration: timeDuration}}
	} else {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid SLO Burn Rate Type", "SLO Burn Rate must have either Dual or Single defined")}
	}

	return burnRate, nil
}

func extractSloThresholdRules(ctx context.Context, rules types.List) ([]*cxsdk.SloThresholdRule, diag.Diagnostics) {
	if rules.IsNull() || rules.IsUnknown() {
		return nil, nil
	}

	var ruleObjs []types.Object
	diags := rules.ElementsAs(ctx, &ruleObjs, true)
	if diags.HasError() {
		return nil, diags
	}

	var result []*cxsdk.SloThresholdRule
	for _, obj := range ruleObjs {
		var model alerttypes.SloThresholdRuleModel
		if diags := obj.As(ctx, &model, basetypes.ObjectAsOptions{}); diags.HasError() {
			return nil, diags
		}

		var condModel alerttypes.SloThresholdConditionModel
		if diags := model.Condition.As(ctx, &condModel, basetypes.ObjectAsOptions{}); diags.HasError() {
			return nil, diags
		}

		override, diags := extractAlertOverride(ctx, model.Override)
		if diags.HasError() {
			return nil, diags
		}

		result = append(result, &cxsdk.SloThresholdRule{
			Condition: &cxsdk.SloThresholdCondition{
				Threshold: wrapperspb.Double(condModel.Threshold.ValueFloat64()),
			},
			Override: override,
		})
	}

	return result, nil
}

func extractSloTimeDuration(ctx context.Context, obj types.Object) (*cxsdk.TimeDuration, diag.Diagnostics) {
	var model alerttypes.SloThresholdDurationWrapperModel
	if diags := obj.As(ctx, &model, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	var durationModel alerttypes.SloDurationModel
	if diags := model.TimeDuration.As(ctx, &durationModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	return &cxsdk.TimeDuration{
		Duration: wrapperspb.UInt64(uint64(durationModel.Duration.ValueInt64())),
		Unit:     alerttypes.DurationUnitSchemaToProtoMap[durationModel.Unit.ValueString()],
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
	var defModel alerttypes.FlowStagesGroupsAlertDefsModel
	if diags := def.As(ctx, &defModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	return &cxsdk.FlowStagesGroupsAlertDefs{
		Id:  utils.TypeStringToWrapperspbString(defModel.Id),
		Not: utils.TypeBoolToWrapperspbBool(defModel.Not),
	}, nil

}

func (r *AlertResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *alerttypes.AlertResourceModel
	diags := req.State.Get(ctx, &state)
	//Get refreshed Alert value from Coralogix
	id := state.ID.ValueString()
	log.Printf("[INFO] Reading Alert: %s", id)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	getAlertReq := &cxsdk.GetAlertDefRequest{Id: wrapperspb.String(id)}
	getAlertResp, err := r.client.Get(ctx, getAlertReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Alert %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Alert",
				utils.FormatRpcErrors(err, getAlertURL, protojson.Format(getAlertReq)),
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

func flattenAlert(ctx context.Context, alert *cxsdk.AlertDef, currentSchedule *types.Object) (*alerttypes.AlertResourceModel, diag.Diagnostics) {
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
	return &alerttypes.AlertResourceModel{
		ID:                utils.WrapperspbStringToTypeString(alert.GetId()),
		Name:              utils.WrapperspbStringToTypeString(alertProperties.GetName()),
		Description:       utils.WrapperspbStringToTypeString(alertProperties.GetDescription()),
		Enabled:           utils.WrapperspbBoolToTypeBool(alertProperties.GetEnabled()),
		Priority:          types.StringValue(alerttypes.AlertPriorityProtoToSchemaMap[alertProperties.GetPriority()]),
		Schedule:          alertSchedule,
		TypeDefinition:    alertTypeDefinition,
		GroupBy:           utils.WrappedStringSliceToTypeStringList(alertProperties.GetGroupByKeys()),
		IncidentsSettings: incidentsSettings,
		NotificationGroup: notificationGroup,
		Labels:            labels,
		PhantomMode:       utils.WrapperspbBoolToTypeBool(alertProperties.GetPhantomMode()),
		Deleted:           utils.WrapperspbBoolToTypeBool(alertProperties.GetDeleted()),
	}, nil
}

func flattenNotificationGroup(ctx context.Context, notificationGroup *cxsdk.AlertDefNotificationGroup) (types.Object, diag.Diagnostics) {
	if notificationGroup == nil {
		return types.ObjectNull(alertschema.NotificationGroupV2Attr()), nil
	}

	webhooksSettings, diags := flattenAdvancedTargetSettings(ctx, notificationGroup.GetWebhooks())
	if diags.HasError() {
		return types.ObjectNull(alertschema.NotificationGroupV2Attr()), diags
	}
	destinations, diags := flattenNotificationDestinations(ctx, notificationGroup.GetDestinations())
	if diags.HasError() {
		return types.ObjectNull(alertschema.NotificationGroupV2Attr()), diags
	}

	router, diags := flattenNotificationRouter(ctx, notificationGroup.GetRouter())
	if diags.HasError() {
		return types.ObjectNull(alertschema.NotificationGroupV2Attr()), diags
	}

	notificationGroupModel := alerttypes.NotificationGroupModel{
		GroupByKeys:      utils.WrappedStringSliceToTypeStringList(notificationGroup.GetGroupByKeys()),
		WebhooksSettings: webhooksSettings,
		Destinations:     destinations,
		Router:           router,
	}

	return types.ObjectValueFrom(ctx, alertschema.NotificationGroupV2Attr(), notificationGroupModel)
}

func flattenAdvancedTargetSettings(ctx context.Context, webhooksSettings []*cxsdk.AlertDefWebhooksSettings) (types.Set, diag.Diagnostics) {
	if webhooksSettings == nil {
		return types.SetNull(types.ObjectType{AttrTypes: alertschema.WebhooksSettingsAttr()}), nil
	}

	var notificationsModel []*alerttypes.WebhooksSettingsModel
	var diags diag.Diagnostics
	for _, notification := range webhooksSettings {
		retriggeringPeriod, dgs := flattenRetriggeringPeriod(ctx, notification)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		notificationModel := alerttypes.WebhooksSettingsModel{
			NotifyOn:           types.StringValue(alerttypes.NotifyOnProtoToSchemaMap[notification.GetNotifyOn()]),
			RetriggeringPeriod: retriggeringPeriod,
			IntegrationID:      types.StringNull(),
			Recipients:         types.SetNull(types.StringType),
		}
		switch integrationType := notification.GetIntegration(); integrationType.GetIntegrationType().(type) {
		case *cxsdk.AlertDefIntegrationTypeIntegrationID:
			notificationModel.IntegrationID = types.StringValue(strconv.Itoa(int(integrationType.GetIntegrationId().GetValue())))
		case *cxsdk.AlertDefIntegrationTypeRecipients:
			notificationModel.Recipients = utils.WrappedStringSliceToTypeStringSet(integrationType.GetRecipients().GetEmails())
		}
		notificationsModel = append(notificationsModel, &notificationModel)
	}

	if diags.HasError() {
		return types.SetNull(types.ObjectType{AttrTypes: alertschema.WebhooksSettingsAttr()}), diags
	}

	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.WebhooksSettingsAttr()}, notificationsModel)
}

func flattenNotificationDestinations(ctx context.Context, destinations []*cxsdk.NotificationDestination) (types.List, diag.Diagnostics) {
	if destinations == nil {
		return types.ListNull(types.ObjectType{AttrTypes: alertschema.NotificationDestinationsV2Attr()}), nil
	}
	var destinationModels []*alerttypes.NotificationDestinationModel
	for _, destination := range destinations {
		var triggeredRoutingOverrides *cxsdk.SourceOverrides
		if destination.TriggeredRoutingOverrides != nil {
			triggeredRoutingOverrides = destination.TriggeredRoutingOverrides.ConfigOverrides
		}
		var resolvedRoutingOverrides *cxsdk.SourceOverrides
		if destination.ResolvedRouteOverrides != nil {
			resolvedRoutingOverrides = destination.ResolvedRouteOverrides.ConfigOverrides
		}
		flattenedTriggeredRoutingOverrides, diags := flattenRoutingOverrides(ctx, triggeredRoutingOverrides)
		if diags.HasError() {
			return types.ListNull(types.ObjectType{AttrTypes: alertschema.NotificationDestinationsV2Attr()}), diags
		}
		flattenedResolvedRoutingOverrides, diags := flattenRoutingOverrides(ctx, resolvedRoutingOverrides)
		if diags.HasError() {
			return types.ListNull(types.ObjectType{AttrTypes: alertschema.NotificationDestinationsV2Attr()}), diags
		}
		destinationModel := alerttypes.NotificationDestinationModel{
			ConnectorId:               types.StringValue(destination.GetConnectorId()),
			PresetId:                  types.StringValue(destination.GetPresetId()),
			NotifyOn:                  types.StringValue(alerttypes.NotifyOnProtoToSchemaMap[destination.GetNotifyOn()]),
			TriggeredRoutingOverrides: flattenedTriggeredRoutingOverrides,
			ResolvedRoutingOverrides:  flattenedResolvedRoutingOverrides,
		}
		destinationModels = append(destinationModels, &destinationModel)
	}
	flattenedDestinations, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.NotificationDestinationsV2Attr()}, destinationModels)
	if diags.HasError() {
		return types.ListNull(types.ListType{ElemType: types.ObjectType{AttrTypes: alertschema.NotificationDestinationsV2Attr()}}), diags
	}
	return flattenedDestinations, nil
}

func flattenRoutingOverrides(ctx context.Context, overrides *cxsdk.SourceOverrides) (types.Object, diag.Diagnostics) {
	if overrides == nil {
		return types.ObjectNull(alertschema.RoutingOverridesV2Attr()), nil
	}

	var connectorOverrideModels []*alerttypes.ConfigurationOverrideModel
	var presetOverrideModels []*alerttypes.ConfigurationOverrideModel
	for _, connectorOverride := range overrides.ConnectorConfigFields {
		connectorOverrideModel := alerttypes.ConfigurationOverrideModel{
			FieldName: types.StringValue(connectorOverride.GetFieldName()),
			Template:  types.StringValue(connectorOverride.GetTemplate()),
		}
		connectorOverrideModels = append(connectorOverrideModels, &connectorOverrideModel)
	}
	for _, presetOverride := range overrides.MessageConfigFields {
		presetOverrideModel := alerttypes.ConfigurationOverrideModel{
			FieldName: types.StringValue(presetOverride.GetFieldName()),
			Template:  types.StringValue(presetOverride.GetTemplate()),
		}
		presetOverrideModels = append(presetOverrideModels, &presetOverrideModel)
	}
	flattenedConnectorOverrides, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.ConfigurationOverridesAttr()}, connectorOverrideModels)
	if diags.HasError() {
		return types.ObjectNull(alertschema.RoutingOverridesV2Attr()), diags
	}
	flattenedPresetOverrides, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.ConfigurationOverridesAttr()}, presetOverrideModels)
	if diags.HasError() {
		return types.ObjectNull(alertschema.RoutingOverridesV2Attr()), diags
	}
	overridesModel := alerttypes.SourceOverridesModel{
		PayloadType:        types.StringValue(overrides.GetPayloadType()),
		ConnectorOverrides: flattenedConnectorOverrides,
		PresetOverrides:    flattenedPresetOverrides,
	}
	return types.ObjectValueFrom(ctx, alertschema.RoutingOverridesV2Attr(), overridesModel)

}

func flattenNotificationRouter(ctx context.Context, notificationRouter *cxsdk.NotificationRouter) (types.Object, diag.Diagnostics) {
	if notificationRouter == nil {
		return types.ObjectNull(alertschema.NotificationRouterAttr()), nil
	}

	notificationRouterModel := alerttypes.NotificationRouterModel{
		NotifyOn: types.StringValue(alerttypes.NotifyOnProtoToSchemaMap[notificationRouter.GetNotifyOn()]),
	}
	return types.ObjectValueFrom(ctx, alertschema.NotificationRouterAttr(), notificationRouterModel)
}

func flattenRetriggeringPeriod(ctx context.Context, notifications *cxsdk.AlertDefWebhooksSettings) (types.Object, diag.Diagnostics) {
	switch notificationPeriodType := notifications.RetriggeringPeriod.(type) {
	case *cxsdk.AlertDefWebhooksSettingsMinutes:
		return types.ObjectValueFrom(ctx, alertschema.RetriggeringPeriodAttr(), alerttypes.RetriggeringPeriodModel{
			Minutes: utils.WrapperspbUint32ToTypeInt64(notificationPeriodType.Minutes),
		})
	case nil:
		return types.ObjectNull(alertschema.RetriggeringPeriodAttr()), nil
	default:
		return types.ObjectNull(alertschema.RetriggeringPeriodAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Retriggering Period", fmt.Sprintf("Retriggering Period %v is not supported", notificationPeriodType))}
	}
}

func flattenIncidentsSettings(ctx context.Context, incidentsSettings *cxsdk.AlertDefIncidentSettings) (types.Object, diag.Diagnostics) {
	if incidentsSettings == nil {
		return types.ObjectNull(alertschema.IncidentsSettingsAttr()), nil
	}

	retriggeringPeriod, diags := flattenIncidentsSettingsByRetriggeringPeriod(ctx, incidentsSettings)
	if diags.HasError() {
		return types.ObjectNull(alertschema.IncidentsSettingsAttr()), diags
	}

	incidentsSettingsModel := alerttypes.IncidentsSettingsModel{
		NotifyOn:           types.StringValue(alerttypes.NotifyOnProtoToSchemaMap[incidentsSettings.GetNotifyOn()]),
		RetriggeringPeriod: retriggeringPeriod,
	}
	return types.ObjectValueFrom(ctx, alertschema.IncidentsSettingsAttr(), incidentsSettingsModel)
}

func flattenIncidentsSettingsByRetriggeringPeriod(ctx context.Context, settings *cxsdk.AlertDefIncidentSettings) (types.Object, diag.Diagnostics) {
	if settings.RetriggeringPeriod == nil {
		return types.ObjectNull(alertschema.RetriggeringPeriodAttr()), nil
	}

	var periodModel alerttypes.RetriggeringPeriodModel
	switch period := settings.RetriggeringPeriod.(type) {
	case *cxsdk.AlertDefIncidentSettingsMinutes:
		periodModel.Minutes = utils.WrapperspbUint32ToTypeInt64(period.Minutes)
	default:
		return types.ObjectNull(alertschema.RetriggeringPeriodAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Retriggering Period", fmt.Sprintf("Retriggering Period %v is not supported", period))}
	}

	return types.ObjectValueFrom(ctx, alertschema.RetriggeringPeriodAttr(), periodModel)
}

func flattenAlertTypeDefinition(ctx context.Context, properties *cxsdk.AlertDefProperties) (types.Object, diag.Diagnostics) {
	if properties.TypeDefinition == nil {
		return types.ObjectNull(alertschema.AlertTypeDefinitionAttr()), nil
	}

	alertTypeDefinitionModel := alerttypes.AlertTypeDefinitionModel{
		LogsImmediate:             types.ObjectNull(alertschema.LogsImmediateAttr()),
		LogsThreshold:             types.ObjectNull(alertschema.LogsThresholdAttr()),
		LogsAnomaly:               types.ObjectNull(alertschema.LogsAnomalyAttr()),
		LogsRatioThreshold:        types.ObjectNull(alertschema.LogsRatioThresholdAttr()),
		LogsNewValue:              types.ObjectNull(alertschema.LogsNewValueAttr()),
		LogsUniqueCount:           types.ObjectNull(alertschema.LogsUniqueCountAttr()),
		LogsTimeRelativeThreshold: types.ObjectNull(alertschema.LogsTimeRelativeAttr()),
		MetricThreshold:           types.ObjectNull(alertschema.MetricThresholdAttr()),
		MetricAnomaly:             types.ObjectNull(alertschema.MetricAnomalyAttr()),
		TracingImmediate:          types.ObjectNull(alertschema.TracingImmediateAttr()),
		TracingThreshold:          types.ObjectNull(alertschema.TracingThresholdAttr()),
		Flow:                      types.ObjectNull(alertschema.FlowAttr()),
		SloThreshold:              types.ObjectNull(alertschema.SloThresholdAttr()),
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
	case *cxsdk.AlertDefPropertiesSlo:
		alertTypeDefinitionModel.SloThreshold, diags = flattenSloThreshold(ctx, alertTypeDefinition.SloThreshold)
	default:
		return types.ObjectNull(alertschema.AlertTypeDefinitionAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Alert Type Definition", fmt.Sprintf("Alert Type '%v' Definition is not valid", alertTypeDefinition))}
	}

	if diags.HasError() {
		return types.ObjectNull(alertschema.AlertTypeDefinitionAttr()), diags
	}

	return types.ObjectValueFrom(ctx, alertschema.AlertTypeDefinitionAttr(), alertTypeDefinitionModel)
}

func flattenLogsImmediate(ctx context.Context, immediate *cxsdk.LogsImmediateType) (types.Object, diag.Diagnostics) {
	if immediate == nil {
		return types.ObjectNull(alertschema.LogsImmediateAttr()), nil
	}

	logsFilter, diags := flattenAlertsLogsFilter(ctx, immediate.GetLogsFilter())
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsImmediateAttr()), diags
	}

	logsImmediateModel := alerttypes.LogsImmediateModel{
		LogsFilter:                logsFilter,
		NotificationPayloadFilter: utils.WrappedStringSliceToTypeStringSet(immediate.GetNotificationPayloadFilter()),
	}
	return types.ObjectValueFrom(ctx, alertschema.LogsImmediateAttr(), logsImmediateModel)
}

func flattenAlertsLogsFilter(ctx context.Context, filter *cxsdk.LogsFilter) (types.Object, diag.Diagnostics) {
	if filter == nil {
		return types.ObjectNull(alertschema.LogsFilterAttr()), nil
	}

	var diags diag.Diagnostics
	var logsFilterModer alerttypes.AlertsLogsFilterModel
	switch filterType := filter.FilterType.(type) {
	case *cxsdk.LogsFilterSimpleFilter:
		logsFilterModer.SimpleFilter, diags = flattenSimpleFilter(ctx, filterType.SimpleFilter)
	default:
		return types.ObjectNull(alertschema.LogsFilterAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Logs Filter", fmt.Sprintf("Logs Filter %v is not supported", filterType))}
	}

	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsFilterAttr()), diags
	}

	return types.ObjectValueFrom(ctx, alertschema.LogsFilterAttr(), logsFilterModer)
}

func flattenSimpleFilter(ctx context.Context, filter *cxsdk.SimpleFilter) (types.Object, diag.Diagnostics) {
	if filter == nil {
		return types.ObjectNull(alertschema.LuceneFilterAttr()), nil
	}

	labelFilters, diags := flattenLabelFilters(ctx, filter.GetLabelFilters())
	if diags.HasError() {
		return types.ObjectNull(alertschema.LuceneFilterAttr()), diags
	}

	return types.ObjectValueFrom(ctx, alertschema.LuceneFilterAttr(), alerttypes.SimpleFilterModel{
		LuceneQuery:  utils.WrapperspbStringToTypeString(filter.GetLuceneQuery()),
		LabelFilters: labelFilters,
	})
}

func flattenLabelFilters(ctx context.Context, filters *cxsdk.LabelFilters) (types.Object, diag.Diagnostics) {
	if filters == nil {
		return types.ObjectNull(alertschema.LabelFiltersAttr()), nil
	}

	applicationName, diags := flattenLabelFilterTypes(ctx, filters.GetApplicationName())
	if diags.HasError() {
		return types.ObjectNull(alertschema.LabelFiltersAttr()), diags
	}

	subsystemName, diags := flattenLabelFilterTypes(ctx, filters.GetSubsystemName())
	if diags.HasError() {
		return types.ObjectNull(alertschema.LabelFiltersAttr()), diags
	}

	severities, diags := flattenLogSeverities(ctx, filters.GetSeverities())
	if diags.HasError() {
		return types.ObjectNull(alertschema.LabelFiltersAttr()), diags
	}

	return types.ObjectValueFrom(ctx, alertschema.LabelFiltersAttr(), alerttypes.LabelFiltersModel{
		ApplicationName: applicationName,
		SubsystemName:   subsystemName,
		Severities:      severities,
	})
}

func flattenLabelFilterTypes(ctx context.Context, name []*cxsdk.LabelFilterType) (types.Set, diag.Diagnostics) {
	var labelFilterTypes []alerttypes.LabelFilterTypeModel
	var diags diag.Diagnostics
	for _, lft := range name {
		labelFilterType := alerttypes.LabelFilterTypeModel{
			Value:     utils.WrapperspbStringToTypeString(lft.GetValue()),
			Operation: types.StringValue(alerttypes.LogFilterOperationTypeProtoToSchemaMap[lft.GetOperation()]),
		}
		labelFilterTypes = append(labelFilterTypes, labelFilterType)
	}
	if diags.HasError() {
		return types.SetNull(types.ObjectType{AttrTypes: alertschema.LabelFilterTypesAttr()}), diags
	}
	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.LabelFilterTypesAttr()}, labelFilterTypes)

}

func flattenLogSeverities(ctx context.Context, severities []cxsdk.LogSeverity) (types.Set, diag.Diagnostics) {
	var result []attr.Value
	for _, severity := range severities {
		result = append(result, types.StringValue(alerttypes.LogSeverityProtoToSchemaMap[severity]))
	}
	return types.SetValueFrom(ctx, types.StringType, result)
}

func flattenLogsThreshold(ctx context.Context, threshold *cxsdk.LogsThresholdType) (types.Object, diag.Diagnostics) {
	if threshold == nil {
		return types.ObjectNull(alertschema.LogsThresholdAttr()), nil
	}

	logsFilter, diags := flattenAlertsLogsFilter(ctx, threshold.GetLogsFilter())
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsThresholdAttr()), diags
	}

	rules, diags := flattenLogsThresholdRules(ctx, threshold.Rules)
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsThresholdAttr()), diags
	}

	undetected, diags := flattenUndetectedValuesManagement(ctx, threshold.GetUndetectedValuesManagement())
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsThresholdAttr()), diags
	}

	logsMoreThanModel := alerttypes.LogsThresholdModel{
		LogsFilter:                 logsFilter,
		Rules:                      rules,
		NotificationPayloadFilter:  utils.WrappedStringSliceToTypeStringSet(threshold.GetNotificationPayloadFilter()),
		UndetectedValuesManagement: undetected,
		CustomEvaluationDelay:      utils.WrapperspbInt32ToTypeInt32(threshold.GetEvaluationDelayMs()),
	}
	return types.ObjectValueFrom(ctx, alertschema.LogsThresholdAttr(), logsMoreThanModel)
}

func flattenLogsThresholdRules(ctx context.Context, rules []*cxsdk.LogsThresholdRule) (types.Set, diag.Diagnostics) {
	if rules == nil {
		return types.SetNull(types.ObjectType{AttrTypes: alertschema.FlowStageAttr()}), nil
	}
	convertedRules := make([]*alerttypes.LogsThresholdRuleModel, len(rules))
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

		convertedRules[i] = &alerttypes.LogsThresholdRuleModel{
			Condition: condition,
			Override:  override,
		}
	}
	if diags.HasError() {
		return types.SetNull(types.ObjectType{AttrTypes: alertschema.LogsThresholdRulesAttr()}), diags
	}
	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.LogsThresholdRulesAttr()}, convertedRules)
}

func flattenLogsThresholdRuleCondition(ctx context.Context, condition *cxsdk.LogsThresholdCondition) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(alertschema.LogsThresholdConditionAttr()), nil
	}

	return types.ObjectValueFrom(ctx, alertschema.LogsThresholdConditionAttr(), alerttypes.LogsThresholdConditionModel{
		Threshold:     utils.WrapperspbDoubleToTypeFloat64(condition.GetThreshold()),
		TimeWindow:    flattenLogsTimeWindow(condition.TimeWindow),
		ConditionType: types.StringValue(alerttypes.LogsThresholdConditionMap[condition.GetConditionType()]),
	})
}

func flattenLogsTimeWindow(timeWindow *cxsdk.LogsTimeWindow) types.String {
	if timeWindow == nil {
		return types.StringNull()
	}
	return types.StringValue(alerttypes.LogsTimeWindowValueProtoToSchemaMap[timeWindow.GetLogsTimeWindowSpecificValue()])
}

func flattenLogsRatioTimeWindow(timeWindow *cxsdk.LogsRatioTimeWindow) types.String {
	if timeWindow == nil {
		return types.StringNull()
	}
	return types.StringValue(alerttypes.LogsRatioTimeWindowValueProtoToSchemaMap[timeWindow.GetLogsRatioTimeWindowSpecificValue()])
}

func flattenLogsNewValueTimeWindow(timeWindow *cxsdk.LogsNewValueTimeWindow) types.String {
	if timeWindow == nil {
		return types.StringNull()
	}
	return types.StringValue(alerttypes.LogsNewValueTimeWindowValueProtoToSchemaMap[timeWindow.GetLogsNewValueTimeWindowSpecificValue()])
}

func flattenUndetectedValuesManagement(ctx context.Context, undetectedValuesManagement *cxsdk.UndetectedValuesManagement) (types.Object, diag.Diagnostics) {
	var undetectedValuesManagementModel alerttypes.UndetectedValuesManagementModel
	if undetectedValuesManagement == nil {
		undetectedValuesManagementModel.TriggerUndetectedValues = types.BoolValue(false)
		undetectedValuesManagementModel.AutoRetireTimeframe = types.StringValue(alerttypes.AutoRetireTimeframeProtoToSchemaMap[cxsdk.AutoRetireTimeframeNeverOrUnspecified])
	} else {
		undetectedValuesManagementModel.TriggerUndetectedValues = utils.WrapperspbBoolToTypeBool(undetectedValuesManagement.GetTriggerUndetectedValues())
		undetectedValuesManagementModel.AutoRetireTimeframe = types.StringValue(alerttypes.AutoRetireTimeframeProtoToSchemaMap[undetectedValuesManagement.GetAutoRetireTimeframe()])
	}
	return types.ObjectValueFrom(ctx, alertschema.UndetectedValuesManagementAttr(), undetectedValuesManagementModel)
}

func flattenLogsAnomaly(ctx context.Context, anomaly *cxsdk.LogsAnomalyType) (types.Object, diag.Diagnostics) {
	if anomaly == nil {
		return types.ObjectNull(alertschema.LogsAnomalyAttr()), nil
	}

	logsFilter, diags := flattenAlertsLogsFilter(ctx, anomaly.GetLogsFilter())
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsAnomalyAttr()), diags
	}

	rulesRaw := make([]alerttypes.LogsAnomalyRuleModel, len(anomaly.Rules))
	for i, rule := range anomaly.Rules {
		condition, dgs := flattenLogsAnomalyRuleCondition(ctx, rule.Condition)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		rulesRaw[i] = alerttypes.LogsAnomalyRuleModel{
			Condition: condition,
		}
	}
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsAnomalyAttr()), diags
	}
	rules, diags := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.LogsAnomalyRulesAttr()}, rulesRaw)
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsAnomalyAttr()), diags
	}
	logsMoreThanUsualModel := alerttypes.LogsAnomalyModel{
		LogsFilter:                logsFilter,
		Rules:                     rules,
		NotificationPayloadFilter: utils.WrappedStringSliceToTypeStringSet(anomaly.GetNotificationPayloadFilter()),
		CustomEvaluationDelay:     utils.WrapperspbInt32ToTypeInt32(anomaly.GetEvaluationDelayMs()),
	}
	return types.ObjectValueFrom(ctx, alertschema.LogsAnomalyAttr(), logsMoreThanUsualModel)
}

func flattenLogsAnomalyRuleCondition(ctx context.Context, condition *cxsdk.LogsAnomalyCondition) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(alertschema.LogsAnomalyConditionAttr()), nil
	}

	return types.ObjectValueFrom(ctx, alertschema.LogsAnomalyConditionAttr(), alerttypes.LogsAnomalyConditionModel{
		MinimumThreshold: utils.WrapperspbDoubleToTypeFloat64(condition.GetMinimumThreshold()),
		TimeWindow:       flattenLogsTimeWindow(condition.TimeWindow),
		ConditionType:    types.StringValue(alerttypes.LogsAnomalyConditionMap[condition.GetConditionType()]),
	})
}

func flattenLogsRatioThreshold(ctx context.Context, ratioThreshold *cxsdk.LogsRatioThresholdType) (types.Object, diag.Diagnostics) {
	if ratioThreshold == nil {
		return types.ObjectNull(alertschema.LogsRatioThresholdAttr()), nil
	}

	numeratorLogsFilter, diags := flattenAlertsLogsFilter(ctx, ratioThreshold.GetNumerator())
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsRatioThresholdAttr()), diags
	}

	denominatorLogsFilter, diags := flattenAlertsLogsFilter(ctx, ratioThreshold.GetDenominator())
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsRatioThresholdAttr()), diags
	}

	rules, diags := flattenRatioThresholdRules(ctx, ratioThreshold)
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsRatioThresholdAttr()), diags
	}

	logsRatioMoreThanModel := alerttypes.LogsRatioThresholdModel{
		Numerator:                 numeratorLogsFilter,
		NumeratorAlias:            utils.WrapperspbStringToTypeString(ratioThreshold.GetNumeratorAlias()),
		Denominator:               denominatorLogsFilter,
		DenominatorAlias:          utils.WrapperspbStringToTypeString(ratioThreshold.GetDenominatorAlias()),
		Rules:                     rules,
		NotificationPayloadFilter: utils.WrappedStringSliceToTypeStringSet(ratioThreshold.GetNotificationPayloadFilter()),
		GroupByFor:                types.StringValue(alerttypes.LogsRatioGroupByForProtoToSchemaMap[ratioThreshold.GetGroupByFor()]),
		CustomEvaluationDelay:     utils.WrapperspbInt32ToTypeInt32(ratioThreshold.GetEvaluationDelayMs()),
	}
	return types.ObjectValueFrom(ctx, alertschema.LogsRatioThresholdAttr(), logsRatioMoreThanModel)
}

func flattenRatioThresholdRules(ctx context.Context, ratioThreshold *cxsdk.LogsRatioThresholdType) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics
	rulesRaw := make([]alerttypes.LogsRatioThresholdRuleModel, len(ratioThreshold.Rules))
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

		rulesRaw[i] = alerttypes.LogsRatioThresholdRuleModel{
			Condition: condition,
			Override:  override,
		}
	}

	if diags.HasError() {
		return types.SetNull(types.ObjectType{AttrTypes: alertschema.LogsRatioThresholdRulesAttr()}), diags
	}

	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.LogsRatioThresholdRulesAttr()}, rulesRaw)
}

func flattenLogsRatioThresholdRuleCondition(ctx context.Context, condition *cxsdk.LogsRatioCondition) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(alertschema.LogsRatioThresholdRuleConditionAttr()), nil
	}

	return types.ObjectValueFrom(ctx, alertschema.LogsRatioThresholdRuleConditionAttr(), alerttypes.LogsRatioConditionModel{
		Threshold:     utils.WrapperspbDoubleToTypeFloat64(condition.GetThreshold()),
		TimeWindow:    flattenLogsRatioTimeWindow(condition.TimeWindow),
		ConditionType: types.StringValue(alerttypes.LogsRatioConditionMap[condition.GetConditionType()]),
	},
	)
}

func flattenAlertOverride(ctx context.Context, override *cxsdk.AlertDefPriorityOverride) (types.Object, diag.Diagnostics) {
	if override == nil {
		return types.ObjectNull(alertschema.AlertOverrideAttr()), nil
	}

	return types.ObjectValueFrom(ctx, alertschema.AlertOverrideAttr(), alerttypes.AlertOverrideModel{
		Priority: types.StringValue(alerttypes.AlertPriorityProtoToSchemaMap[override.GetPriority()]),
	})
}

func flattenLogsUniqueCount(ctx context.Context, uniqueCount *cxsdk.LogsUniqueCountType) (types.Object, diag.Diagnostics) {
	if uniqueCount == nil {
		return types.ObjectNull(alertschema.LogsUniqueCountAttr()), nil
	}

	logsFilter, diags := flattenAlertsLogsFilter(ctx, uniqueCount.GetLogsFilter())
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsUniqueCountAttr()), diags
	}

	rules, diags := flattenLogsUniqueCountRules(ctx, uniqueCount)
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsUniqueCountAttr()), diags
	}

	logsUniqueCountModel := alerttypes.LogsUniqueCountModel{
		LogsFilter:                  logsFilter,
		Rules:                       rules,
		NotificationPayloadFilter:   utils.WrappedStringSliceToTypeStringSet(uniqueCount.GetNotificationPayloadFilter()),
		MaxUniqueCountPerGroupByKey: utils.WrapperspbInt64ToTypeInt64(uniqueCount.GetMaxUniqueCountPerGroupByKey()),
		UniqueCountKeypath:          utils.WrapperspbStringToTypeString(uniqueCount.GetUniqueCountKeypath()),
	}
	return types.ObjectValueFrom(ctx, alertschema.LogsUniqueCountAttr(), logsUniqueCountModel)
}

func flattenLogsUniqueCountRules(ctx context.Context, uniqueCount *cxsdk.LogsUniqueCountType) (types.Set, diag.Diagnostics) {
	rulesRaw := make([]alerttypes.LogsUniqueCountRuleModel, len(uniqueCount.Rules))
	var diags diag.Diagnostics
	for i, rule := range uniqueCount.Rules {
		condition, dgs := flattenLogsUniqueCountRuleCondition(ctx, rule.Condition)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		rulesRaw[i] = alerttypes.LogsUniqueCountRuleModel{
			Condition: condition,
		}
	}

	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.LogsUniqueCountRulesAttr()}, rulesRaw)
}

func flattenLogsUniqueCountRuleCondition(ctx context.Context, condition *cxsdk.LogsUniqueCountCondition) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(alertschema.LogsUniqueCountConditionAttr()), nil
	}

	return types.ObjectValueFrom(ctx, alertschema.LogsUniqueCountConditionAttr(), alerttypes.LogsUniqueCountConditionModel{
		MaxUniqueCount: utils.WrapperspbInt64ToTypeInt64(condition.GetMaxUniqueCount()),
		TimeWindow:     flattenLogsUniqueTimeWindow(condition.TimeWindow),
	})
}

func flattenLogsUniqueTimeWindow(timeWindow *cxsdk.LogsUniqueValueTimeWindow) types.String {
	if timeWindow == nil {
		return types.StringNull()
	}
	return types.StringValue(alerttypes.LogsUniqueCountTimeWindowValueProtoToSchemaMap[timeWindow.GetLogsUniqueValueTimeWindowSpecificValue()])
}

func flattenLogsNewValue(ctx context.Context, newValue *cxsdk.LogsNewValueType) (types.Object, diag.Diagnostics) {
	if newValue == nil {
		return types.ObjectNull(alertschema.LogsNewValueAttr()), nil
	}

	logsFilter, diags := flattenAlertsLogsFilter(ctx, newValue.GetLogsFilter())
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsNewValueAttr()), diags
	}

	rulesRaw := make([]alerttypes.NewValueRuleModel, len(newValue.Rules))
	for i, rule := range newValue.Rules {
		condition, dgs := flattenLogsNewValueCondition(ctx, rule.Condition)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		rulesRaw[i] = alerttypes.NewValueRuleModel{
			Condition: condition,
		}
	}
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsNewValueAttr()), diags
	}

	rules, diags := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.LogsNewValueRulesAttr()}, rulesRaw)
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsNewValueAttr()), diags
	}

	logsNewValueModel := alerttypes.LogsNewValueModel{
		LogsFilter:                logsFilter,
		Rules:                     rules,
		NotificationPayloadFilter: utils.WrappedStringSliceToTypeStringSet(newValue.GetNotificationPayloadFilter()),
	}
	return types.ObjectValueFrom(ctx, alertschema.LogsNewValueAttr(), logsNewValueModel)
}

func flattenLogsNewValueCondition(ctx context.Context, condition *cxsdk.LogsNewValueCondition) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(alertschema.LogsNewValueConditionAttr()), nil
	}

	return types.ObjectValueFrom(ctx, alertschema.LogsNewValueConditionAttr(), alerttypes.NewValueConditionModel{
		TimeWindow:     flattenLogsNewValueTimeWindow(condition.TimeWindow),
		KeypathToTrack: utils.WrapperspbStringToTypeString(condition.GetKeypathToTrack()),
	})
}

func flattenAlertSchedule(ctx context.Context, alertProperties *cxsdk.AlertDefProperties, currentSchedule *types.Object) (types.Object, diag.Diagnostics) {
	if alertProperties.Schedule == nil {
		return types.ObjectNull(alertschema.AlertScheduleAttr()), nil
	}

	var alertScheduleModel alerttypes.AlertScheduleModel
	var diags diag.Diagnostics
	switch alertScheduleType := alertProperties.Schedule.(type) {
	case *cxsdk.AlertDefPropertiesActiveOn:
		utcOffset := DEFAULT_TIMEZONE_OFFSET
		// Set the offset according to the previous state, if possible
		// Note that there is a default value set on the schema so it should work for new resources, but old/generated states could run into this
		var scheduleModel alerttypes.AlertScheduleModel
		if diags := currentSchedule.As(ctx, &scheduleModel, basetypes.ObjectAsOptions{}); !diags.HasError() {
			if !utils.ObjIsNullOrUnknown(scheduleModel.ActiveOn) {
				var activeOnModel alerttypes.ActiveOnModel
				if diags := scheduleModel.ActiveOn.As(ctx, &activeOnModel, basetypes.ObjectAsOptions{}); !diags.HasError() {
					utcOffset = activeOnModel.UtcOffset.ValueString()
				}
			}
		}

		alertScheduleModel.ActiveOn, diags = flattenActiveOn(ctx, alertScheduleType.ActiveOn, utcOffset)
	default:
		return types.ObjectNull(alertschema.AlertScheduleAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Alert Schedule", fmt.Sprintf("Alert Schedule %v is not supported", alertScheduleType))}
	}

	if diags.HasError() {
		return types.ObjectNull(alertschema.AlertScheduleAttr()), diags
	}

	return types.ObjectValueFrom(ctx, alertschema.AlertScheduleAttr(), alertScheduleModel)
}

func flattenActiveOn(ctx context.Context, activeOn *cxsdk.AlertDefActivitySchedule, utcOffset string) (types.Object, diag.Diagnostics) {
	if activeOn == nil {
		return types.ObjectNull(alertschema.AlertScheduleActiveOnAttr()), nil
	}

	daysOfWeek, diags := flattenDaysOfWeek(ctx, activeOn.GetDayOfWeek())
	if diags.HasError() {
		return types.ObjectNull(alertschema.AlertScheduleActiveOnAttr()), diags
	}
	offset, err := time.Parse(OFFSET_FORMAT, utcOffset)

	if err != nil {
		return types.ObjectNull(alertschema.AlertScheduleActiveOnAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid UTC Offset", fmt.Sprintf("UTC Offset %v is not valid", utcOffset))}
	}
	zoneName, offsetSecs := offset.Zone() // Name is probably empty
	zone := time.FixedZone(zoneName, offsetSecs)
	startTime := time.Date(2021, 2, 1, int(activeOn.StartTime.Hours), int(activeOn.StartTime.Minutes), 0, 0, time.UTC).In(zone)
	endTime := time.Date(2021, 2, 1, int(activeOn.EndTime.Hours), int(activeOn.EndTime.Minutes), 0, 0, time.UTC).In(zone)

	activeOnModel := alerttypes.ActiveOnModel{
		DaysOfWeek: daysOfWeek,
		StartTime:  types.StringValue(startTime.Format(TIME_FORMAT)),
		EndTime:    types.StringValue(endTime.Format(TIME_FORMAT)),
		UtcOffset:  types.StringValue(utcOffset),
	}
	return types.ObjectValueFrom(ctx, alertschema.AlertScheduleActiveOnAttr(), activeOnModel)
}

func flattenDaysOfWeek(ctx context.Context, daysOfWeek []cxsdk.AlertDayOfWeek) (types.Set, diag.Diagnostics) {
	var daysOfWeekStrings []types.String
	for _, dow := range daysOfWeek {
		daysOfWeekStrings = append(daysOfWeekStrings, types.StringValue(alerttypes.DaysOfWeekProtoToSchemaMap[dow]))
	}
	return types.SetValueFrom(ctx, types.StringType, daysOfWeekStrings)
}

func flattenLogsTimeRelativeThreshold(ctx context.Context, logsTimeRelativeThreshold *cxsdk.LogsTimeRelativeThresholdType) (types.Object, diag.Diagnostics) {
	if logsTimeRelativeThreshold == nil {
		return types.ObjectNull(alertschema.LogsTimeRelativeAttr()), nil
	}

	logsFilter, diags := flattenAlertsLogsFilter(ctx, logsTimeRelativeThreshold.GetLogsFilter())
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsTimeRelativeAttr()), diags
	}

	rulesRaw := make([]alerttypes.LogsTimeRelativeRuleModel, len(logsTimeRelativeThreshold.Rules))
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

		rulesRaw[i] = alerttypes.LogsTimeRelativeRuleModel{
			Condition: condition,
			Override:  override,
		}
	}

	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsTimeRelativeAttr()), diags
	}

	rules, diags := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.LogsTimeRelativeRulesAttr()}, rulesRaw)
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsTimeRelativeAttr()), diags
	}

	undetected, diags := flattenUndetectedValuesManagement(ctx, logsTimeRelativeThreshold.UndetectedValuesManagement)
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsTimeRelativeAttr()), diags
	}

	logsTimeRelativeThresholdModel := alerttypes.LogsTimeRelativeThresholdModel{
		LogsFilter:                 logsFilter,
		Rules:                      rules,
		NotificationPayloadFilter:  utils.WrappedStringSliceToTypeStringSet(logsTimeRelativeThreshold.GetNotificationPayloadFilter()),
		UndetectedValuesManagement: undetected,
		CustomEvaluationDelay:      utils.WrapperspbInt32ToTypeInt32(logsTimeRelativeThreshold.GetEvaluationDelayMs()),
	}

	return types.ObjectValueFrom(ctx, alertschema.LogsTimeRelativeAttr(), logsTimeRelativeThresholdModel)
}

func flattenLogsTimeRelativeRuleCondition(ctx context.Context, condition *cxsdk.LogsTimeRelativeCondition) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(alertschema.LogsTimeRelativeConditionAttr()), nil
	}

	return types.ObjectValueFrom(ctx, alertschema.LogsTimeRelativeConditionAttr(), alerttypes.LogsTimeRelativeConditionModel{
		Threshold:     utils.WrapperspbDoubleToTypeFloat64(condition.GetThreshold()),
		ComparedTo:    types.StringValue(alerttypes.LogsTimeRelativeComparedToProtoToSchemaMap[condition.GetComparedTo()]),
		ConditionType: types.StringValue(alerttypes.LogsTimeRelativeConditionMap[condition.GetConditionType()]),
	})
}

func flattenMetricThreshold(ctx context.Context, metricThreshold *cxsdk.MetricThresholdType) (types.Object, diag.Diagnostics) {
	if metricThreshold == nil {
		return types.ObjectNull(alertschema.MetricThresholdAttr()), nil
	}

	metricFilter, diags := flattenMetricFilter(ctx, metricThreshold.GetMetricFilter())
	if diags.HasError() {
		return types.ObjectNull(alertschema.MetricThresholdAttr()), diags
	}

	undetectedValuesManagement, diags := flattenUndetectedValuesManagement(ctx, metricThreshold.GetUndetectedValuesManagement())
	if diags.HasError() {
		return types.ObjectNull(alertschema.MetricThresholdAttr()), diags
	}

	rulesRaw := make([]alerttypes.MetricThresholdRuleModel, len(metricThreshold.Rules))
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

		rulesRaw[i] = alerttypes.MetricThresholdRuleModel{
			Condition: condition,
			Override:  override,
		}
	}
	rules, diags := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.MetricThresholdRulesAttr()}, rulesRaw)
	if diags.HasError() {
		return types.ObjectNull(alertschema.MetricThresholdAttr()), diags
	}

	missingValues, diags := flattenMissingValuesManagement(ctx, metricThreshold.GetMissingValues())
	if diags.HasError() {
		return types.ObjectNull(alertschema.MetricThresholdAttr()), diags
	}

	metricThresholdModel := alerttypes.MetricThresholdModel{
		MetricFilter:               metricFilter,
		Rules:                      rules,
		MissingValues:              missingValues,
		UndetectedValuesManagement: undetectedValuesManagement,
		CustomEvaluationDelay:      utils.WrapperspbInt32ToTypeInt32(metricThreshold.GetEvaluationDelayMs()),
	}
	return types.ObjectValueFrom(ctx, alertschema.MetricThresholdAttr(), metricThresholdModel)
}

func flattenMissingValuesManagement(ctx context.Context, missingValues *cxsdk.MetricMissingValues) (types.Object, diag.Diagnostics) {
	if missingValues == nil {
		return types.ObjectNull(alertschema.MissingValuesAttr()), nil
	}

	switch missingValuesType := missingValues.MissingValues.(type) {
	case *cxsdk.MetricMissingValuesReplaceWithZero:
		return types.ObjectValueFrom(ctx, alertschema.MissingValuesAttr(), alerttypes.MissingValuesModel{
			ReplaceWithZero: utils.WrapperspbBoolToTypeBool(missingValuesType.ReplaceWithZero),
		})
	case *cxsdk.MetricMissingValuesMinNonNullValuesPct:
		return types.ObjectValueFrom(ctx, alertschema.MissingValuesAttr(), alerttypes.MissingValuesModel{
			MinNonNullValuesPct: utils.WrapperspbUint32ToTypeInt64(missingValuesType.MinNonNullValuesPct),
		})
	default:
		return types.ObjectNull(alertschema.MissingValuesAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Missing Values Management", fmt.Sprintf("Missing Values Management %v is not supported", missingValuesType))}
	}
}

func flattenMetricThresholdRuleCondition(ctx context.Context, condition *cxsdk.MetricThresholdCondition) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(alertschema.MetricThresholdConditionAttr()), nil
	}

	return types.ObjectValueFrom(ctx, alertschema.MetricThresholdConditionAttr(), alerttypes.MetricThresholdConditionModel{
		Threshold:     utils.WrapperspbDoubleToTypeFloat64(condition.GetThreshold()),
		ForOverPct:    utils.WrapperspbUint32ToTypeInt64(condition.GetForOverPct()),
		OfTheLast:     flattenMetricTimeWindow(condition.GetOfTheLast()),
		ConditionType: types.StringValue(alerttypes.MetricsThresholdConditionMap[condition.GetConditionType()]),
	})
}

func flattenMetricTimeWindow(timeWindow *cxsdk.MetricTimeWindow) types.String {
	if timeWindow == nil {
		return types.StringNull()
	}

	switch timeWindowType := timeWindow.GetType().(type) {
	case *cxsdk.MetricTimeWindowSpecificValue:
		return types.StringValue(alerttypes.MetricFilterOperationTypeProtoToSchemaMap[timeWindowType.MetricTimeWindowSpecificValue])
	case *cxsdk.MetricTimeWindowDynamicDuration:
		return types.StringValue(timeWindowType.MetricTimeWindowDynamicDuration.GetValue())
	}
	return types.StringValue(alerttypes.MetricFilterOperationTypeProtoToSchemaMap[timeWindow.GetMetricTimeWindowSpecificValue()])
}

func flattenMetricFilter(ctx context.Context, filter *cxsdk.MetricFilter) (types.Object, diag.Diagnostics) {
	if filter == nil {
		return types.ObjectNull(alertschema.MetricFilterAttr()), nil
	}

	switch filterType := filter.Type.(type) {
	case *cxsdk.MetricFilterPromql:
		return types.ObjectValueFrom(ctx, alertschema.MetricFilterAttr(), alerttypes.MetricFilterModel{
			Promql: utils.WrapperspbStringToTypeString(filterType.Promql),
		})
	default:
		return types.ObjectNull(alertschema.MetricFilterAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Metric Filter", fmt.Sprintf("Metric Filter %v is not supported", filterType))}
	}
}

func flattenTracingImmediate(ctx context.Context, tracingImmediate *cxsdk.TracingImmediateType) (types.Object, diag.Diagnostics) {
	if tracingImmediate == nil {
		return types.ObjectNull(alertschema.TracingImmediateAttr()), nil
	}

	var tracingQuery types.Object

	switch filtersType := tracingImmediate.TracingFilter.FilterType.(type) {
	case *cxsdk.TracingFilterSimpleFilter:
		filter, diag := flattenTracingSimpleFilter(ctx, filtersType.SimpleFilter)
		if diag.HasError() {
			return types.ObjectNull(alertschema.TracingImmediateAttr()), diag
		}
		tracingQuery, diag = types.ObjectValueFrom(ctx, alertschema.TracingQueryAttr(), filter)
		if diag.HasError() {
			return types.ObjectNull(alertschema.TracingImmediateAttr()), diag
		}
	default:
		return types.ObjectNull(alertschema.TracingImmediateAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Tracing Query Filters", fmt.Sprintf("Tracing Query Filters %v is not supported", filtersType))}
	}

	tracingImmediateModel := alerttypes.TracingImmediateModel{
		TracingFilter:             tracingQuery,
		NotificationPayloadFilter: utils.WrappedStringSliceToTypeStringSet(tracingImmediate.GetNotificationPayloadFilter()),
	}

	return types.ObjectValueFrom(ctx, alertschema.TracingImmediateAttr(), tracingImmediateModel)
}

// Also called query filters
func flattenTracingFilter(ctx context.Context, tracingFilter *cxsdk.TracingFilter) (types.Object, diag.Diagnostics) {
	switch filtersType := tracingFilter.FilterType.(type) {
	case *cxsdk.TracingFilterSimpleFilter:
		filter, diag := flattenTracingSimpleFilter(ctx, filtersType.SimpleFilter)
		if diag.HasError() {
			return types.ObjectNull(alertschema.TracingQueryAttr()), diag
		}
		tracingQuery, diag := types.ObjectValueFrom(ctx, alertschema.TracingQueryAttr(), filter)
		if diag.HasError() {
			return types.ObjectNull(alertschema.TracingQueryAttr()), diag
		}
		return tracingQuery, nil
	default:
		return types.ObjectNull(alertschema.TracingQueryAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Tracing Query Filters", fmt.Sprintf("Tracing Query Filters %v is not supported", filtersType))}
	}

}

func flattenTracingSimpleFilter(ctx context.Context, tracingQuery *cxsdk.TracingSimpleFilter) (types.Object, diag.Diagnostics) {
	if tracingQuery == nil {
		return types.ObjectNull(alertschema.TracingQueryAttr()), nil
	}

	labelFilters, diags := flattenTracingLabelFilters(ctx, tracingQuery.TracingLabelFilters)
	if diags.HasError() {
		return types.ObjectNull(alertschema.TracingQueryAttr()), diags
	}
	tracingQueryModel := &alerttypes.TracingFilterModel{
		LatencyThresholdMs:  utils.WrappedUint64TotypeNumber(tracingQuery.LatencyThresholdMs),
		TracingLabelFilters: labelFilters,
	}
	if diags.HasError() {
		return types.ObjectNull(alertschema.TracingQueryAttr()), diags
	}

	return types.ObjectValueFrom(ctx, alertschema.TracingQueryAttr(), tracingQueryModel)
}

func flattenTracingLabelFilters(ctx context.Context, filters *cxsdk.TracingLabelFilters) (types.Object, diag.Diagnostics) {
	if filters == nil {
		return types.ObjectNull(alertschema.TracingLabelFiltersAttr()), nil
	}

	applicationName, diags := flattenTracingFilterTypes(ctx, filters.GetApplicationName())
	if diags.HasError() {
		return types.ObjectNull(alertschema.TracingLabelFiltersAttr()), diags
	}

	subsystemName, diags := flattenTracingFilterTypes(ctx, filters.GetSubsystemName())
	if diags.HasError() {
		return types.ObjectNull(alertschema.TracingLabelFiltersAttr()), diags

	}

	serviceName, diags := flattenTracingFilterTypes(ctx, filters.GetServiceName())
	if diags.HasError() {
		return types.ObjectNull(alertschema.TracingLabelFiltersAttr()), diags
	}

	operationName, diags := flattenTracingFilterTypes(ctx, filters.GetOperationName())
	if diags.HasError() {
		return types.ObjectNull(alertschema.TracingLabelFiltersAttr()), diags
	}

	spanFields, diags := flattenTracingSpansFields(ctx, filters.GetSpanFields())
	if diags.HasError() {
		return types.ObjectNull(alertschema.TracingLabelFiltersAttr()), diags
	}

	return types.ObjectValueFrom(ctx, alertschema.TracingLabelFiltersAttr(), alerttypes.TracingLabelFiltersModel{
		ApplicationName: applicationName,
		SubsystemName:   subsystemName,
		ServiceName:     serviceName,
		OperationName:   operationName,
		SpanFields:      spanFields,
	})

}

func flattenTracingFilterTypes(ctx context.Context, TracingFilterType []*cxsdk.TracingFilterType) (types.Set, diag.Diagnostics) {
	var tracingFilterTypes []*alerttypes.TracingFilterTypeModel
	for _, tft := range TracingFilterType {
		tracingFilterTypes = append(tracingFilterTypes, flattenTracingFilterType(tft))
	}
	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.TracingFiltersTypeAttr()}, tracingFilterTypes)
}

func flattenTracingFilterType(tracingFilterType *cxsdk.TracingFilterType) *alerttypes.TracingFilterTypeModel {
	if tracingFilterType == nil {
		return nil
	}

	return &alerttypes.TracingFilterTypeModel{
		Values:    utils.WrappedStringSliceToTypeStringSet(tracingFilterType.GetValues()),
		Operation: types.StringValue(alerttypes.TracingFilterOperationProtoToSchemaMap[tracingFilterType.GetOperation()]),
	}
}

func flattenTracingSpansFields(ctx context.Context, spanFields []*cxsdk.TracingSpanFieldsFilterType) (types.Set, diag.Diagnostics) {
	var tracingSpanFields []*alerttypes.TracingSpanFieldsFilterModel
	for _, field := range spanFields {
		tracingSpanField, diags := flattenTracingSpanField(ctx, field)
		if diags.HasError() {
			return types.SetNull(types.ObjectType{AttrTypes: alertschema.TracingSpanFieldsFilterAttr()}), diags
		}
		tracingSpanFields = append(tracingSpanFields, tracingSpanField)
	}
	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.TracingSpanFieldsFilterAttr()}, tracingSpanFields)
}

func flattenTracingSpanField(ctx context.Context, spanField *cxsdk.TracingSpanFieldsFilterType) (*alerttypes.TracingSpanFieldsFilterModel, diag.Diagnostics) {
	if spanField == nil {
		return nil, nil
	}

	filterType, diags := types.ObjectValueFrom(ctx, alertschema.TracingFiltersTypeAttr(), flattenTracingFilterType(spanField.GetFilterType()))
	if diags.HasError() {
		return nil, diags
	}

	return &alerttypes.TracingSpanFieldsFilterModel{
		Key:        utils.WrapperspbStringToTypeString(spanField.GetKey()),
		FilterType: filterType,
	}, nil
}

func flattenTracingThreshold(ctx context.Context, tracingThreshold *cxsdk.TracingThresholdType) (types.Object, diag.Diagnostics) {
	if tracingThreshold == nil {
		return types.ObjectNull(alertschema.TracingThresholdAttr()), nil
	}

	tracingQuery, diags := flattenTracingFilter(ctx, tracingThreshold.GetTracingFilter())
	if diags.HasError() {
		return types.ObjectNull(alertschema.TracingThresholdAttr()), diags
	}

	rules, diags := flattenTracingThresholdRules(ctx, tracingThreshold, diags)
	if diags.HasError() {
		return types.ObjectNull(alertschema.TracingThresholdAttr()), diags
	}

	tracingThresholdModel := alerttypes.TracingThresholdModel{
		TracingFilter:             tracingQuery,
		Rules:                     rules,
		NotificationPayloadFilter: utils.WrappedStringSliceToTypeStringSet(tracingThreshold.GetNotificationPayloadFilter()),
	}
	return types.ObjectValueFrom(ctx, alertschema.TracingThresholdAttr(), tracingThresholdModel)
}

func flattenTracingThresholdRules(ctx context.Context, tracingThreshold *cxsdk.TracingThresholdType, diags diag.Diagnostics) (basetypes.SetValue, diag.Diagnostics) {
	rulesRaw := make([]alerttypes.TracingThresholdRuleModel, len(tracingThreshold.Rules))
	for i, rule := range tracingThreshold.Rules {
		condition, dgs := flattenTracingThresholdRuleCondition(ctx, rule.Condition)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		rulesRaw[i] = alerttypes.TracingThresholdRuleModel{
			Condition: condition,
		}
	}
	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.TracingThresholdRulesAttr()}, rulesRaw)
}

func flattenTracingThresholdRuleCondition(ctx context.Context, condition *cxsdk.TracingThresholdCondition) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(alertschema.TracingThresholdConditionAttr()), nil
	}

	return types.ObjectValueFrom(ctx, alertschema.TracingThresholdConditionAttr(), alerttypes.TracingThresholdConditionModel{
		TimeWindow:    flattenTracingTimeWindow(condition.GetTimeWindow()),
		SpanAmount:    utils.WrapperspbDoubleToTypeFloat64(condition.GetSpanAmount()),
		ConditionType: types.StringValue("MORE_THAN"),
	})
}

func flattenTracingTimeWindow(timeWindow *cxsdk.TracingTimeWindow) types.String {
	if timeWindow == nil {
		return types.StringNull()
	}

	return types.StringValue(alerttypes.TracingTimeWindowProtoToSchemaMap[timeWindow.GetTracingTimeWindowValue()])
}

func flattenMetricAnomaly(ctx context.Context, anomaly *cxsdk.MetricAnomalyType) (types.Object, diag.Diagnostics) {
	if anomaly == nil {
		return types.ObjectNull(alertschema.MetricAnomalyAttr()), nil
	}

	metricFilter, diags := flattenMetricFilter(ctx, anomaly.GetMetricFilter())
	if diags.HasError() {
		return types.ObjectNull(alertschema.MetricAnomalyAttr()), diags
	}

	rulesRaw := make([]alerttypes.MetricAnomalyRuleModel, len(anomaly.Rules))
	for i, rule := range anomaly.Rules {
		condition, dgs := flattenMetricAnomalyCondition(ctx, rule.Condition)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		rulesRaw[i] = alerttypes.MetricAnomalyRuleModel{
			Condition: condition,
		}
	}
	if diags.HasError() {
		return types.ObjectNull(alertschema.MetricAnomalyAttr()), diags
	}

	rules, diags := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.MetricAnomalyRulesAttr()}, rulesRaw)
	if diags.HasError() {
		return types.ObjectNull(alertschema.MetricAnomalyAttr()), diags
	}
	anomalyModel := alerttypes.MetricAnomalyModel{
		MetricFilter:          metricFilter,
		Rules:                 rules,
		CustomEvaluationDelay: utils.WrapperspbInt32ToTypeInt32(anomaly.GetEvaluationDelayMs()),
	}
	return types.ObjectValueFrom(ctx, alertschema.MetricAnomalyAttr(), anomalyModel)
}

func flattenMetricAnomalyCondition(ctx context.Context, condition *cxsdk.MetricAnomalyCondition) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(alertschema.MetricAnomalyConditionAttr()), nil
	}

	return types.ObjectValueFrom(ctx, alertschema.MetricAnomalyConditionAttr(), alerttypes.MetricAnomalyConditionModel{
		MinNonNullValuesPct: utils.WrapperspbUint32ToTypeInt64(condition.GetMinNonNullValuesPct()),
		Threshold:           utils.WrapperspbDoubleToTypeFloat64(condition.GetThreshold()),
		ForOverPct:          utils.WrapperspbUint32ToTypeInt64(condition.GetForOverPct()),
		OfTheLast:           flattenMetricTimeWindow(condition.GetOfTheLast()),
		ConditionType:       types.StringValue(alerttypes.MetricAnomalyConditionMap[condition.GetConditionType()]),
	},
	)
}

func flattenFlow(ctx context.Context, flow *cxsdk.FlowType) (types.Object, diag.Diagnostics) {
	if flow == nil {
		return types.ObjectNull(alertschema.FlowAttr()), nil
	}

	stages, diags := flattenFlowStages(ctx, flow.GetStages())
	if diags.HasError() {
		return types.ObjectNull(alertschema.FlowAttr()), diags
	}

	flowModel := alerttypes.FlowModel{
		Stages:             stages,
		EnforceSuppression: utils.WrapperspbBoolToTypeBool(flow.GetEnforceSuppression()),
	}
	return types.ObjectValueFrom(ctx, alertschema.FlowAttr(), flowModel)
}

func flattenFlowStages(ctx context.Context, stages []*cxsdk.FlowStages) (types.List, diag.Diagnostics) {
	var flowStages []*alerttypes.FlowStageModel
	for _, stage := range stages {
		flowStage, diags := flattenFlowStage(ctx, stage)
		if diags.HasError() {
			return types.ListNull(types.ObjectType{AttrTypes: alertschema.FlowStageAttr()}), diags
		}
		flowStages = append(flowStages, flowStage)
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.FlowStageAttr()}, flowStages)

}

func flattenFlowStage(ctx context.Context, stage *cxsdk.FlowStages) (*alerttypes.FlowStageModel, diag.Diagnostics) {
	if stage == nil {
		return nil, nil
	}

	flowStagesGroups, diags := flattenFlowStagesGroups(ctx, stage)
	if diags.HasError() {
		return nil, diags
	}

	flowStageModel := &alerttypes.FlowStageModel{
		FlowStagesGroups: flowStagesGroups,
		TimeframeMs:      utils.WrapperspbInt64ToTypeInt64(stage.GetTimeframeMs()),
		TimeframeType:    types.StringValue(alerttypes.FlowStageTimeFrameTypeProtoToSchemaMap[stage.GetTimeframeType()]),
	}
	return flowStageModel, nil

}

func flattenFlowStagesGroups(ctx context.Context, stage *cxsdk.FlowStages) (types.List, diag.Diagnostics) {
	var flowStagesGroups []*alerttypes.FlowStagesGroupModel
	for _, group := range stage.GetFlowStagesGroups().GetGroups() {
		flowStageGroup, diags := flattenFlowStageGroup(ctx, group)
		if diags.HasError() {
			return types.ListNull(types.ObjectType{AttrTypes: alertschema.FlowStageGroupAttr()}), diags
		}
		flowStagesGroups = append(flowStagesGroups, flowStageGroup)
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.FlowStageGroupAttr()}, flowStagesGroups)

}

func flattenFlowStageGroup(ctx context.Context, group *cxsdk.FlowStagesGroup) (*alerttypes.FlowStagesGroupModel, diag.Diagnostics) {
	if group == nil {
		return nil, nil
	}

	alertDefs, diags := flattenAlertDefs(ctx, group.GetAlertDefs())
	if diags.HasError() {
		return nil, diags
	}

	flowStageGroupModel := &alerttypes.FlowStagesGroupModel{
		AlertDefs: alertDefs,
		NextOp:    types.StringValue(alerttypes.FlowStagesGroupNextOpProtoToSchemaMap[group.GetNextOp()]),
		AlertsOp:  types.StringValue(alerttypes.FlowStagesGroupAlertsOpProtoToSchemaMap[group.GetAlertsOp()]),
	}
	return flowStageGroupModel, nil
}

func flattenAlertDefs(ctx context.Context, defs []*cxsdk.FlowStagesGroupsAlertDefs) (types.Set, diag.Diagnostics) {
	var alertDefs []*alerttypes.FlowStagesGroupsAlertDefsModel
	for _, def := range defs {
		alertDef := &alerttypes.FlowStagesGroupsAlertDefsModel{
			Id:  utils.WrapperspbStringToTypeString(def.GetId()),
			Not: utils.WrapperspbBoolToTypeBool(def.GetNot()),
		}
		alertDefs = append(alertDefs, alertDef)
	}
	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.AlertDefsAttr()}, alertDefs)
}

func flattenSloThreshold(ctx context.Context, slo *cxsdk.SloThresholdType) (types.Object, diag.Diagnostics) {
	if slo == nil {
		return types.ObjectNull(alertschema.SloThresholdAttr()), nil
	}

	sloDefinition := types.ObjectValueMust(alertschema.SloDefinitionAttr(), map[string]attr.Value{
		"slo_id": utils.WrapperspbStringToTypeString(slo.GetSloDefinition().GetSloId()),
	})

	sloModel := alerttypes.SloThresholdModel{
		SloDefinition: sloDefinition,
		ErrorBudget:   types.ObjectNull(alertschema.SloErrorBudgetAttr()),
		BurnRate:      types.ObjectNull(alertschema.SloBurnRateAttr()),
	}

	switch t := slo.GetThreshold().(type) {
	case *cxsdk.SloErrorBudgetThresholdType:
		errBudget, diags := flattenSloErrorBudget(ctx, t.ErrorBudget)
		if diags.HasError() {
			return types.ObjectNull(alertschema.SloThresholdAttr()), diags
		}
		sloModel.ErrorBudget = errBudget
	case *cxsdk.SloBurnRateThresholdType:
		burnRate, diags := flattenSloBurnRate(ctx, t.BurnRate)
		if diags.HasError() {
			return types.ObjectNull(alertschema.SloThresholdAttr()), diags
		}
		sloModel.BurnRate = burnRate
	}

	return types.ObjectValueFrom(ctx, alertschema.SloThresholdAttr(), sloModel)
}

func flattenSloErrorBudget(ctx context.Context, errBudget *cxsdk.SloErrorBudgetThreshold) (types.Object, diag.Diagnostics) {
	rules, diags := flattenSloThresholdRules(ctx, errBudget.GetRules())
	if diags.HasError() {
		return types.ObjectNull(alertschema.SloErrorBudgetAttr()), diags
	}
	return types.ObjectValueFrom(ctx, alertschema.SloErrorBudgetAttr(), alerttypes.SloThresholdErrorBudgetModel{Rules: rules})
}

func flattenSloBurnRate(ctx context.Context, burnRate *cxsdk.SloBurnRateThreshold) (types.Object, diag.Diagnostics) {
	rules, diags := flattenSloThresholdRules(ctx, burnRate.GetRules())
	if diags.HasError() {
		return types.ObjectNull(alertschema.SloBurnRateAttr()), diags
	}

	burnRateModel := alerttypes.SloThresholdBurnRateModel{
		Rules:  rules,
		Dual:   types.ObjectNull(alertschema.SloDurationWrapperAttr()),
		Single: types.ObjectNull(alertschema.SloDurationWrapperAttr()),
	}

	switch bt := burnRate.GetType().(type) {
	case *cxsdk.DualBurnRateThresholdType:
		td, diags := flattenSloTimeDuration(ctx, bt.Dual.GetTimeDuration())
		if diags.HasError() {
			return types.ObjectNull(alertschema.SloBurnRateAttr()), diags
		}
		burnRateModel.Dual = td
	case *cxsdk.SingleBurnRateThresholdType:
		td, diags := flattenSloTimeDuration(ctx, bt.Single.GetTimeDuration())
		if diags.HasError() {
			return types.ObjectNull(alertschema.SloBurnRateAttr()), diags
		}
		burnRateModel.Single = td
	}

	return types.ObjectValueFrom(ctx, alertschema.SloBurnRateAttr(), burnRateModel)
}

func flattenSloThresholdRules(ctx context.Context, rules []*cxsdk.SloThresholdRule) (types.List, diag.Diagnostics) {
	var models []alerttypes.SloThresholdRuleModel
	for _, rule := range rules {
		override, diags := flattenAlertOverride(ctx, rule.GetOverride())
		if diags.HasError() {
			return types.ListNull(types.ObjectType{AttrTypes: alertschema.SloThresholdRuleAttr()}), diags
		}
		ruleModel := alerttypes.SloThresholdRuleModel{
			Condition: types.ObjectValueMust(alertschema.SloThresholdConditionAttr(), map[string]attr.Value{
				"threshold": types.Float64Value(rule.GetCondition().GetThreshold().GetValue()),
			}),
			Override: override,
		}
		models = append(models, ruleModel)
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.SloThresholdRuleAttr()}, models)
}

func flattenSloTimeDuration(ctx context.Context, td *cxsdk.TimeDuration) (types.Object, diag.Diagnostics) {
	return types.ObjectValueFrom(ctx, alertschema.SloDurationWrapperAttr(), alerttypes.SloThresholdDurationWrapperModel{
		TimeDuration: types.ObjectValueMust(alertschema.SloDurationAttr(), map[string]attr.Value{
			"duration": types.Int64Value(int64(td.GetDuration().GetValue())),
			"unit":     types.StringValue(alerttypes.DurationUnitProtoToSchemaMap[td.GetUnit()]),
		}),
	})
}

func (r *AlertResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan *alerttypes.AlertResourceModel
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
		Id:                 utils.TypeStringToWrapperspbString(plan.ID),
		AlertDefProperties: alertProperties,
	}
	log.Printf("[INFO] Updating Alert: %s", protojson.Format(updateAlertReq))
	alertUpdateResp, err := r.client.Replace(ctx, updateAlertReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating Alert",
			utils.FormatRpcErrors(err, updateAlertURL, protojson.Format(updateAlertReq)),
		)
		return
	}
	log.Printf("[INFO] Submitted updated Alert: %s", protojson.Format(alertUpdateResp))

	// Get refreshed Alert value from Coralogix
	getAlertReq := &cxsdk.GetAlertDefRequest{Id: utils.TypeStringToWrapperspbString(plan.ID)}
	getAlertResp, err := r.client.Get(ctx, getAlertReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Alert %q is in state, but no longer exists in Coralogix backend", plan.ID.ValueString()),
				fmt.Sprintf("%s will be recreated when you apply", plan.ID.ValueString()),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Alert",
				utils.FormatRpcErrors(err, getAlertURL, protojson.Format(getAlertReq)),
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
	var state alerttypes.AlertResourceModel
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
			utils.FormatRpcErrors(err, deleteAlertURL, protojson.Format(deleteReq)),
		)
		return
	}
	log.Printf("[INFO] Alert %s deleted", id)
}
