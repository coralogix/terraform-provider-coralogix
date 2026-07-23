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
	"math/big"
	"net/http"
	"strconv"
	"time"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	alertschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/alerts/alert_schema"
	alerttypes "github.com/coralogix/terraform-provider-coralogix/internal/provider/alerts/alert_types"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	alerts "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/alert_definitions_service"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Format to parse time from and format to
const TIME_FORMAT = "15:04"

// Format to parse offset from and format to
const OFFSET_FORMAT = "Z0700"

const DEFAULT_TIMEZONE_OFFSET = "+0000"

var (
	_ resource.ResourceWithConfigure   = &AlertResource{}
	_ resource.ResourceWithImportState = &AlertResource{}
)

func NewAlertResource() resource.Resource {
	return &AlertResource{}
}

type AlertResource struct {
	client *alerts.AlertDefinitionsServiceAPIService
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
			fmt.Sprintf("Expected *alerts.ClientSet, got: %T. Please report this issue to the provider developers.", req.ProviderData),
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

		getAlertResp, httpResponse, err := r.client.AlertDefsServiceGetAlertDef(ctx, id).Execute()
		if err != nil {
			resp.Diagnostics.AddError("Error creating coralogix_alert",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", id),
			)
			return
		}

		alert := getAlertResp.GetAlertDef()

		newState, diags := flattenAlert(ctx, alert, &schedule, nil)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}

		// Prior schema versions could not configure retriggering_period_minutes,
		// so any value echoed by the backend is the inherited default and must
		// not be pinned into the upgraded state.
		notificationGroup, diags := clearDestinationRetriggering(ctx, newState.NotificationGroup)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		newState.NotificationGroup = notificationGroup

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
	rq := alerts.CreateAlertDefinitionRequest{AlertDefProperties: alertProperties}
	result, httpResponse, err := r.client.AlertDefsServiceCreateAlertDef(ctx).CreateAlertDefinitionRequest(rq).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error creating coralogix_alert",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", rq),
		)
		return
	}
	plan, diags = flattenAlert(ctx, result.GetAlertDef(), &plan.Schedule, &plan.NotificationGroup)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
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
	id := plan.ID.ValueString()
	rq := &alerts.ReplaceAlertDefinitionRequest{
		Id:                 &id,
		AlertDefProperties: alertProperties,
	}
	result, httpResponse, err := r.client.
		AlertDefsServiceReplaceAlertDef(ctx).
		ReplaceAlertDefinitionRequest(*rq).Execute()
	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_alert %v is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%v will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error replacing coralogix_alert", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Replace", rq))
		}
		return
	}
	plan, diags = flattenAlert(ctx, result.GetAlertDef(), &plan.Schedule, &plan.NotificationGroup)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

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
	_, httpResponse, err := r.client.
		AlertDefsServiceDeleteAlertDef(ctx, id).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error reading alert",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Delete", id),
		)
		return
	}
}

func (r *AlertResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *alerttypes.AlertResourceModel
	diags := req.State.Get(ctx, &state)

	id := state.ID.ValueString()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	rq := r.client.AlertDefsServiceGetAlertDef(ctx, id)

	result, httpResponse, err := rq.Execute()
	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_alert %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error reading coralogix_alert",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
			)
		}
		return
	}

	state, diags = flattenAlert(ctx, result.GetAlertDef(), &state.Schedule, &state.NotificationGroup)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func extractAlertProperties(ctx context.Context, plan *alerttypes.AlertResourceModel) (*alerts.AlertDefProperties, diag.Diagnostics) {
	alertProperties := &alerts.AlertDefProperties{}

	alertProperties, diags := expandAlertsTypeDefinition(ctx, alertProperties, *plan)
	if diags.HasError() {
		return nil, diags
	}

	dataSources, diags := extractDataSources(ctx, plan.DataSources)
	if diags.HasError() {
		return nil, diags
	}
	alertProperties.DataSources = dataSources

	return alertProperties, nil
}

func extractDataSources(ctx context.Context, dataSources types.List) ([]alerts.AlertDefDataSource, diag.Diagnostics) {
	if dataSources.IsNull() || dataSources.IsUnknown() {
		return nil, nil
	}

	var dataSourceObjects []types.Object
	diags := dataSources.ElementsAs(ctx, &dataSourceObjects, true)
	if diags.HasError() {
		return nil, diags
	}

	var extractedDataSources []alerts.AlertDefDataSource
	for _, dataSourceObject := range dataSourceObjects {
		var dataSourceModel alerttypes.DataSourceModel
		if diags := dataSourceObject.As(ctx, &dataSourceModel, basetypes.ObjectAsOptions{}); diags.HasError() {
			return nil, diags
		}
		extractedDataSources = append(extractedDataSources, alerts.AlertDefDataSource{
			DataSpace: dataSourceModel.DataSpace.ValueStringPointer(),
			DataSet:   dataSourceModel.DataSet.ValueStringPointer(),
		})
	}

	return extractedDataSources, nil
}

func extractCustomEvaluationDelay(delay types.Int32) *int32 {
	if delay.IsNull() || delay.IsUnknown() {
		return nil
	}

	return delay.ValueInt32Pointer()
}

func extractIncidentsSettings(ctx context.Context, incidentsSettingsObject types.Object) (*alerts.AlertDefIncidentSettings, diag.Diagnostics) {
	if incidentsSettingsObject.IsNull() || incidentsSettingsObject.IsUnknown() {
		return nil, nil
	}

	var incidentsSettingsModel alerttypes.IncidentsSettingsModel
	if diags := incidentsSettingsObject.As(ctx, &incidentsSettingsModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	incidentsSettings := &alerts.AlertDefIncidentSettings{}

	if !incidentsSettingsModel.NotifyOn.IsNull() && !incidentsSettingsModel.NotifyOn.IsUnknown() {
		incidentsSettings.NotifyOn = alerttypes.NotifyOnSchemaToProtoMap[incidentsSettingsModel.NotifyOn.ValueString()].Ptr()
	} else {
		incidentsSettings.NotifyOn = alerts.NOTIFYON_NOTIFY_ON_TRIGGERED_ONLY_UNSPECIFIED.Ptr()
	}

	incidentsSettings, diags := expandIncidentsSettingsByRetriggeringPeriod(ctx, incidentsSettings, incidentsSettingsModel.RetriggeringPeriod)
	if diags.HasError() {
		return nil, diags
	}

	return incidentsSettings, nil
}

func expandIncidentsSettingsByRetriggeringPeriod(ctx context.Context, incidentsSettings *alerts.AlertDefIncidentSettings, period types.Object) (*alerts.AlertDefIncidentSettings, diag.Diagnostics) {
	if period.IsNull() || period.IsUnknown() {
		return incidentsSettings, nil
	}

	var periodModel alerttypes.RetriggeringPeriodModel
	if diags := period.As(ctx, &periodModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if !(periodModel.Minutes.IsNull() || periodModel.Minutes.IsUnknown()) {
		incidentsSettings.Minutes = periodModel.Minutes.ValueInt64Pointer()
	}

	return incidentsSettings, nil
}

func extractNotificationGroup(ctx context.Context, notificationGroupObject types.Object) (*alerts.AlertDefNotificationGroup, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(notificationGroupObject) {
		return nil, nil
	}

	var notificationGroupModel alerttypes.NotificationGroupModel
	if diags := notificationGroupObject.As(ctx, &notificationGroupModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	groupByFields, diags := utils.TypeStringElementsToStringSlice(ctx, notificationGroupModel.GroupByKeys.Elements())
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
	notificationGroup := &alerts.AlertDefNotificationGroup{
		Destinations: destinations,
		Router:       router,
		GroupByKeys:  groupByFields,
		Webhooks:     webhooks,
	}

	return notificationGroup, nil
}

func extractWebhooksSettings(ctx context.Context, webhooksSettings types.Set) ([]alerts.AlertDefWebhooksSettings, diag.Diagnostics) {
	if webhooksSettings.IsNull() || webhooksSettings.IsUnknown() {
		return nil, nil
	}

	var webhooksSettingsObject []types.Object
	diags := webhooksSettings.ElementsAs(ctx, &webhooksSettingsObject, true)
	if diags.HasError() {
		return nil, diags
	}
	var expandedWebhooksSettings []alerts.AlertDefWebhooksSettings
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
		if expandedAdvancedTargetSetting != nil {
			expandedWebhooksSettings = append(expandedWebhooksSettings, *expandedAdvancedTargetSetting)
		}
	}

	if diags.HasError() {
		return nil, diags
	}

	return expandedWebhooksSettings, nil
}

func extractDestinations(ctx context.Context, notificationDestinations types.List) ([]alerts.NotificationDestination, diag.Diagnostics) {
	if notificationDestinations.IsNull() || notificationDestinations.IsUnknown() {
		return nil, nil
	}

	var notificationDestinationsObject []types.Object
	diags := notificationDestinations.ElementsAs(ctx, &notificationDestinationsObject, true)
	if diags.HasError() {
		return nil, diags
	}
	var expandedDestinations []alerts.NotificationDestination
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
		destination := alerts.NotificationDestination{
			ConnectorId: destinationModel.ConnectorId.ValueStringPointer(),
			PresetId:    &presetId,
			TriggeredRoutingOverrides: &alerts.NotificationRouting{
				ConfigOverrides: triggeredRoutingOverrides,
			},
			ResolvedRouteOverrides: &alerts.NotificationRouting{
				ConfigOverrides: resolvedRoutingOverrides,
			},
		}

		if !destinationModel.RetriggeringPeriodMinutes.IsNull() && !destinationModel.RetriggeringPeriodMinutes.IsUnknown() {
			destination.RetriggeringPeriodMinutes = destinationModel.RetriggeringPeriodMinutes.ValueInt64Pointer()
		}

		if !destinationModel.NotifyOn.IsNull() && !destinationModel.NotifyOn.IsUnknown() {
			destination.NotifyOn = alerttypes.NotifyOnSchemaToProtoMap[destinationModel.NotifyOn.ValueString()].Ptr()
		} else {
			destination.NotifyOn = alerts.NOTIFYON_NOTIFY_ON_TRIGGERED_ONLY_UNSPECIFIED.Ptr()
		}
		expandedDestinations = append(expandedDestinations, destination)
	}

	return expandedDestinations, nil
}

func extractRoutingOverrides(ctx context.Context, overridesObject types.Object) (*alerts.V3SourceOverrides, diag.Diagnostics) {
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
	sourceOverrides := &alerts.V3SourceOverrides{
		ConnectorConfigFields: connectorOverrides,
		MessageConfigFields:   presetOverrides,
		PayloadType:           routingOverridesModel.PayloadType.ValueStringPointer(),
	}

	return sourceOverrides, nil
}

func extractConnectorOverrides(ctx context.Context, overridesObject types.List) ([]alerts.V3ConnectorConfigField, diag.Diagnostics) {
	if overridesObject.IsNull() || overridesObject.IsUnknown() {
		return nil, nil
	}

	var configurationOverridesModel []types.Object
	diags := overridesObject.ElementsAs(ctx, &configurationOverridesModel, true)
	if diags.HasError() {
		return nil, diags
	}
	var connectorOverrides []alerts.V3ConnectorConfigField
	for _, override := range configurationOverridesModel {
		var connectorOverrideModel alerttypes.ConfigurationOverrideModel
		if diags := override.As(ctx, &connectorOverrideModel, basetypes.ObjectAsOptions{}); diags.HasError() {
			return nil, diags
		}
		connectorOverride := alerts.V3ConnectorConfigField{
			FieldName: connectorOverrideModel.FieldName.ValueStringPointer(),
			Template:  connectorOverrideModel.Template.ValueStringPointer(),
		}
		connectorOverrides = append(connectorOverrides, connectorOverride)
	}

	return connectorOverrides, nil
}

func extractPresetOverrides(ctx context.Context, overridesObject types.List) ([]alerts.V3MessageConfigField, diag.Diagnostics) {
	if overridesObject.IsNull() || overridesObject.IsUnknown() {
		return nil, nil
	}

	var configurationOverridesModel []types.Object
	diags := overridesObject.ElementsAs(ctx, &configurationOverridesModel, true)
	if diags.HasError() {
		return nil, diags
	}
	var connectorOverrides []alerts.V3MessageConfigField
	for _, override := range configurationOverridesModel {
		var connectorOverrideModel alerttypes.ConfigurationOverrideModel
		if diags := override.As(ctx, &connectorOverrideModel, basetypes.ObjectAsOptions{}); diags.HasError() {
			return nil, diags
		}
		connectorOverride := alerts.V3MessageConfigField{
			FieldName: connectorOverrideModel.FieldName.ValueStringPointer(),
			Template:  connectorOverrideModel.Template.ValueStringPointer(),
		}
		connectorOverrides = append(connectorOverrides, connectorOverride)
	}

	return connectorOverrides, nil
}

func extractNotificationRouter(ctx context.Context, routerObject types.Object) (*alerts.NotificationRouter, diag.Diagnostics) {
	if routerObject.IsNull() || routerObject.IsUnknown() {
		return nil, nil
	}

	var routerModel alerttypes.NotificationRouterModel
	if diags := routerObject.As(ctx, &routerModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	router := &alerts.NotificationRouter{}

	if !routerModel.NotifyOn.IsNull() && !routerModel.NotifyOn.IsUnknown() {
		router.NotifyOn = alerttypes.NotifyOnSchemaToProtoMap[routerModel.NotifyOn.ValueString()].Ptr()
	} else {
		router.NotifyOn = alerts.NOTIFYON_NOTIFY_ON_TRIGGERED_ONLY_UNSPECIFIED.Ptr()
	}

	return router, nil
}

func extractAdvancedTargetSetting(ctx context.Context, webhooksSettingsModel alerttypes.WebhooksSettingsModel) (*alerts.AlertDefWebhooksSettings, diag.Diagnostics) {
	advancedTargetSettings := &alerts.AlertDefWebhooksSettings{}

	notifyOnIsEmpty := webhooksSettingsModel.NotifyOn.IsNull() || webhooksSettingsModel.NotifyOn.IsUnknown()
	retriggeringPeriodIsEmpty := utils.ObjIsNullOrUnknown(webhooksSettingsModel.RetriggeringPeriod)

	if notifyOnIsEmpty && retriggeringPeriodIsEmpty {
		log.Printf("[WARN] Advanced notifications disabled for webhook - both notify_on and retriggering_period are not set")
	}

	if !notifyOnIsEmpty {
		advancedTargetSettings.NotifyOn = alerttypes.NotifyOnSchemaToProtoMap[webhooksSettingsModel.NotifyOn.ValueString()].Ptr()
	}
	advancedTargetSettings, diags := expandAlertNotificationByRetriggeringPeriod(ctx, advancedTargetSettings, webhooksSettingsModel.RetriggeringPeriod)
	if diags.HasError() {
		return nil, diags
	}

	if !webhooksSettingsModel.IntegrationID.IsNull() && !webhooksSettingsModel.IntegrationID.IsUnknown() {
		integrationId, diag := utils.TypeStringToInt64Pointer(webhooksSettingsModel.IntegrationID)
		if diag.HasError() {
			return nil, diag
		}
		advancedTargetSettings.Integration = &alerts.V3IntegrationType{
			IntegrationId: integrationId,
		}
	} else if !webhooksSettingsModel.Recipients.IsNull() && !webhooksSettingsModel.Recipients.IsUnknown() {
		emails, diags := utils.TypeStringElementsToStringSlice(ctx, webhooksSettingsModel.Recipients.Elements())
		if diags.HasError() {
			return nil, diags
		}
		advancedTargetSettings.Integration = &alerts.V3IntegrationType{
			Recipients: &alerts.Recipients{
				Emails: emails,
			},
		}
	}

	return advancedTargetSettings, nil
}

func expandAlertNotificationByRetriggeringPeriod(ctx context.Context, alertNotification *alerts.AlertDefWebhooksSettings, period types.Object) (*alerts.AlertDefWebhooksSettings, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(period) {
		return alertNotification, nil
	}

	var periodModel alerttypes.RetriggeringPeriodModel
	if diags := period.As(ctx, &periodModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if !(periodModel.Minutes.IsNull() || periodModel.Minutes.IsUnknown()) {
		alertNotification.Minutes = periodModel.Minutes.ValueInt64Pointer()
	}

	return alertNotification, nil
}

func dayDelta(from, to time.Time) int {
	fromDay := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	toDay := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC)
	return int(toDay.Sub(fromDay).Hours() / 24)
}

func expandActiveOnSchedule(ctx context.Context, scheduleObject types.Object) (*alerts.ActivitySchedule, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(scheduleObject) {
		return nil, nil
	}
	var scheduleModel alerttypes.AlertScheduleModel
	if diags := scheduleObject.As(ctx, &scheduleModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}
	if utils.ObjIsNullOrUnknown(scheduleModel.ActiveOn) {
		return nil, nil
	}

	var activeOnModel alerttypes.ActiveOnModel
	if diags := scheduleModel.ActiveOn.As(ctx, &activeOnModel, basetypes.ObjectAsOptions{}); diags.HasError() {
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

	if diags.HasError() {
		return nil, diags
	}

	startTimeUtc := startTime.UTC()
	endTimeUtc := endTime.UTC()
	startHour := int32(startTimeUtc.Hour())
	startMinute := int32(startTimeUtc.Minute())
	endHour := int32(endTimeUtc.Hour())
	endMinute := int32(endTimeUtc.Minute())

	if dayShift := dayDelta(startTime, startTimeUtc); dayShift != 0 {
		daysOfWeek = alerttypes.ShiftDaysOfWeek(daysOfWeek, dayShift)
	}
	return &alerts.ActivitySchedule{
		DayOfWeek: daysOfWeek,
		StartTime: &alerts.TimeOfDay{
			Hours:   &startHour,
			Minutes: &startMinute,
		},
		EndTime: &alerts.TimeOfDay{
			Hours:   &endHour,
			Minutes: &endMinute,
		},
	}, nil
}

func extractDaysOfWeek(ctx context.Context, daysOfWeek types.Set) ([]alerts.DayOfWeek, diag.Diagnostics) {
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
		result = append(result, alerttypes.DaysOfWeekSchemaToProtoMap[str])
	}
	return result, diags
}

func expandAlertsTypeDefinition(ctx context.Context, alertProperties *alerts.AlertDefProperties, alertResourceModel alerttypes.AlertResourceModel) (*alerts.AlertDefProperties, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(alertResourceModel.TypeDefinition) {
		return alertProperties, nil
	}

	var alertDefinitionModel alerttypes.AlertTypeDefinitionModel
	if diags := alertResourceModel.TypeDefinition.As(ctx, &alertDefinitionModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	updatedProperties, handled, diags := expandLogsAlertsTypeDefinition(ctx, alertProperties, alertDefinitionModel, alertResourceModel)
	if !handled {
		updatedProperties, handled, diags = expandNonLogsAlertsTypeDefinition(ctx, alertProperties, alertDefinitionModel, alertResourceModel)
	}
	if !handled {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Alert Type Definition", "Alert Type Definition is not valid")}
	}
	if diags.HasError() {
		return nil, diags
	}

	return updatedProperties, nil
}

func expandLogsAlertsTypeDefinition(ctx context.Context, alertProperties *alerts.AlertDefProperties, alertDefinitionModel alerttypes.AlertTypeDefinitionModel, alertResourceModel alerttypes.AlertResourceModel) (*alerts.AlertDefProperties, bool, diag.Diagnostics) {
	switch {
	case !utils.ObjIsNullOrUnknown(alertDefinitionModel.LogsImmediate):
		properties, diags := expandLogsImmediateAlertTypeDefinition(ctx, alertProperties, alertDefinitionModel.LogsImmediate, alertResourceModel)
		return properties, true, diags
	case !utils.ObjIsNullOrUnknown(alertDefinitionModel.LogsThreshold):
		properties, diags := expandLogsThresholdTypeDefinition(ctx, alertProperties, alertDefinitionModel.LogsThreshold, alertResourceModel)
		return properties, true, diags
	case !utils.ObjIsNullOrUnknown(alertDefinitionModel.LogsAnomaly):
		properties, diags := expandLogsAnomalyAlertTypeDefinition(ctx, alertProperties, alertDefinitionModel.LogsAnomaly, alertResourceModel)
		return properties, true, diags
	case !utils.ObjIsNullOrUnknown(alertDefinitionModel.LogsRatioThreshold):
		properties, diags := expandLogsRatioThresholdTypeDefinition(ctx, alertProperties, alertDefinitionModel.LogsRatioThreshold, alertResourceModel)
		return properties, true, diags
	case !utils.ObjIsNullOrUnknown(alertDefinitionModel.LogsNewValue):
		properties, diags := expandLogsNewValueAlertTypeDefinition(ctx, alertProperties, alertDefinitionModel.LogsNewValue, alertResourceModel)
		return properties, true, diags
	case !utils.ObjIsNullOrUnknown(alertDefinitionModel.LogsUniqueCount):
		properties, diags := expandLogsUniqueCountAlertTypeDefinition(ctx, alertProperties, alertDefinitionModel.LogsUniqueCount, alertResourceModel)
		return properties, true, diags
	case !utils.ObjIsNullOrUnknown(alertDefinitionModel.LogsTimeRelativeThreshold):
		properties, diags := expandLogsTimeRelativeThresholdAlertTypeDefinition(ctx, alertProperties, alertDefinitionModel.LogsTimeRelativeThreshold, alertResourceModel)
		return properties, true, diags
	default:
		return alertProperties, false, nil
	}
}

func expandNonLogsAlertsTypeDefinition(ctx context.Context, alertProperties *alerts.AlertDefProperties, alertDefinitionModel alerttypes.AlertTypeDefinitionModel, alertResourceModel alerttypes.AlertResourceModel) (*alerts.AlertDefProperties, bool, diag.Diagnostics) {
	switch {
	case !utils.ObjIsNullOrUnknown(alertDefinitionModel.MetricThreshold):
		properties, diags := expandMetricThresholdAlertTypeDefinition(ctx, alertProperties, alertDefinitionModel.MetricThreshold, alertResourceModel)
		return properties, true, diags
	case !utils.ObjIsNullOrUnknown(alertDefinitionModel.MetricAnomaly):
		properties, diags := expandMetricAnomalyAlertTypeDefinition(ctx, alertProperties, alertDefinitionModel.MetricAnomaly, alertResourceModel)
		return properties, true, diags
	case !utils.ObjIsNullOrUnknown(alertDefinitionModel.TracingImmediate):
		properties, diags := expandTracingImmediateTypeDefinition(ctx, alertProperties, alertDefinitionModel.TracingImmediate, alertResourceModel)
		return properties, true, diags
	case !utils.ObjIsNullOrUnknown(alertDefinitionModel.TracingThreshold):
		properties, diags := expandTracingThresholdTypeDefinition(ctx, alertProperties, alertDefinitionModel.TracingThreshold, alertResourceModel)
		return properties, true, diags
	case !utils.ObjIsNullOrUnknown(alertDefinitionModel.Flow):
		properties, diags := expandFlowAlertTypeDefinition(ctx, alertProperties, alertDefinitionModel.Flow, alertResourceModel)
		return properties, true, diags
	case !utils.ObjIsNullOrUnknown(alertDefinitionModel.SloThreshold):
		properties, diags := expandSloThresholdAlertTypeDefinition(ctx, alertProperties, alertDefinitionModel.SloThreshold, alertResourceModel)
		return properties, true, diags
	default:
		return alertProperties, false, nil
	}
}

func expandLogsImmediateAlertTypeDefinition(ctx context.Context, properties *alerts.AlertDefProperties, logsImmediateObject types.Object, alertResourceModel alerttypes.AlertResourceModel) (*alerts.AlertDefProperties, diag.Diagnostics) {
	var immediateModel alerttypes.LogsImmediateModel
	if diags := logsImmediateObject.As(ctx, &immediateModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	logsFilter, diags := extractLogsFilter(ctx, immediateModel.LogsFilter)
	if diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := utils.TypeStringElementsToStringSlice(ctx, immediateModel.NotificationPayloadFilter.Elements())
	if diags.HasError() {
		return nil, diags
	}

	groupBy, diags := utils.TypeStringElementsToStringSlice(ctx, alertResourceModel.GroupBy.Elements())
	if diags.HasError() {
		return nil, diags
	}

	incidentsSettings, diags := extractIncidentsSettings(ctx, alertResourceModel.IncidentsSettings)
	if diags.HasError() {
		return nil, diags
	}
	notificationGroup, diags := extractNotificationGroup(ctx, alertResourceModel.NotificationGroup)
	if diags.HasError() {
		return nil, diags
	}

	labels, diags := utils.TypeMapToStringMap(ctx, alertResourceModel.Labels)
	if diags.HasError() {
		return nil, diags
	}
	schedule, diags := expandActiveOnSchedule(ctx, alertResourceModel.Schedule)
	if diags.HasError() {
		return nil, diags
	}
	properties.Name = alertResourceModel.Name.ValueStringPointer()
	properties.Description = alertResourceModel.Description.ValueStringPointer()
	properties.Enabled = alertResourceModel.Enabled.ValueBoolPointer()
	properties.Priority = alerttypes.AlertPrioritySchemaToProtoMap[extractAlertPriority(alertResourceModel.Priority)].Ptr()
	properties.GroupByKeys = groupBy
	properties.IncidentsSettings = incidentsSettings
	properties.NotificationGroup = notificationGroup
	properties.EntityLabels = &labels
	properties.PhantomMode = alertResourceModel.PhantomMode.ValueBoolPointer()
	properties.ActiveOn = schedule

	properties.LogsImmediate = &alerts.LogsImmediateType{
		LogsFilter:                logsFilter,
		NotificationPayloadFilter: notificationPayloadFilter,
	}
	properties.Type = alerts.ALERTDEFTYPE_ALERT_DEF_TYPE_LOGS_IMMEDIATE_OR_UNSPECIFIED.Ptr()
	return properties, nil
}

func extractAlertPriority(priority types.String) string {
	if priority.IsNull() || priority.IsUnknown() {
		return alerttypes.AlertPriorityProtoToSchemaMap[alerts.ALERTDEFPRIORITY_ALERT_DEF_PRIORITY_P5_OR_UNSPECIFIED]
	} else {
		return priority.ValueString()
	}
}

func extractLogsFilter(ctx context.Context, filter types.Object) (*alerts.V3LogsFilter, diag.Diagnostics) {
	if filter.IsNull() || filter.IsUnknown() {
		return nil, nil
	}

	var filterModel alerttypes.AlertsLogsFilterModel
	if diags := filter.As(ctx, &filterModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	logsFilter := &alerts.V3LogsFilter{}
	var diags diag.Diagnostics
	if !(filterModel.SimpleFilter.IsNull() || filterModel.SimpleFilter.IsUnknown()) {
		logsFilter.SimpleFilter, diags = extractLuceneFilter(ctx, filterModel.SimpleFilter)
	}

	if diags.HasError() {
		return nil, diags
	}

	return logsFilter, nil
}

func extractLuceneFilter(ctx context.Context, luceneFilter types.Object) (*alerts.LogsSimpleFilter, diag.Diagnostics) {
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

	return &alerts.LogsSimpleFilter{
		LuceneQuery:  utils.TypeStringToStringPointer(luceneFilterModel.LuceneQuery),
		LabelFilters: labelFilters,
	}, nil
}

func extractLabelFilters(ctx context.Context, filters types.Object) (*alerts.LabelFilters, diag.Diagnostics) {
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

	return &alerts.LabelFilters{
		ApplicationName: applicationName,
		SubsystemName:   subsystemName,
		Severities:      severities,
	}, nil
}

func extractLabelFilterTypes(ctx context.Context, labelFilterTypes types.Set) ([]alerts.LabelFilterType, diag.Diagnostics) {
	var labelFilterTypesObjects []types.Object
	diags := labelFilterTypes.ElementsAs(ctx, &labelFilterTypesObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	var expandedLabelFilterTypes []alerts.LabelFilterType
	for _, lft := range labelFilterTypesObjects {
		var labelFilterTypeModel alerttypes.LabelFilterTypeModel
		if dg := lft.As(ctx, &labelFilterTypeModel, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		operation := labelFilterTypeModel.Operation.ValueString()
		if operation == "" {
			operation = alerttypes.LogFilterOperationTypeProtoToSchemaMap[alerts.LOGFILTEROPERATIONTYPE_LOG_FILTER_OPERATION_TYPE_IS_OR_UNSPECIFIED]
		}
		expandedLabelFilterType := alerts.LabelFilterType{
			Value:     labelFilterTypeModel.Value.ValueStringPointer(),
			Operation: alerttypes.LogFilterOperationTypeSchemaToProtoMap[operation].Ptr(),
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
		result = append(result, alerttypes.LogSeveritySchemaToProtoMap[str])
	}
	return result, diags
}

func expandLogsThresholdTypeDefinition(ctx context.Context, properties *alerts.AlertDefProperties, thresholdObject types.Object, alertResourceModel alerttypes.AlertResourceModel) (*alerts.AlertDefProperties, diag.Diagnostics) {
	var thresholdModel alerttypes.LogsThresholdModel
	if diags := thresholdObject.As(ctx, &thresholdModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	logsFilter, diags := extractLogsFilter(ctx, thresholdModel.LogsFilter)
	if diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := utils.TypeStringElementsToStringSlice(ctx, thresholdModel.NotificationPayloadFilter.Elements())
	if diags.HasError() {
		return nil, diags
	}

	groupBy, diags := utils.TypeStringElementsToStringSlice(ctx, alertResourceModel.GroupBy.Elements())
	if diags.HasError() {
		return nil, diags
	}

	incidentsSettings, diags := extractIncidentsSettings(ctx, alertResourceModel.IncidentsSettings)
	if diags.HasError() {
		return nil, diags
	}
	notificationGroup, diags := extractNotificationGroup(ctx, alertResourceModel.NotificationGroup)
	if diags.HasError() {
		return nil, diags
	}

	labels, diags := utils.TypeMapToStringMap(ctx, alertResourceModel.Labels)
	if diags.HasError() {
		return nil, diags
	}
	schedule, diags := expandActiveOnSchedule(ctx, alertResourceModel.Schedule)
	if diags.HasError() {
		return nil, diags
	}
	properties.Name = alertResourceModel.Name.ValueStringPointer()
	properties.Description = alertResourceModel.Description.ValueStringPointer()
	properties.Enabled = alertResourceModel.Enabled.ValueBoolPointer()
	properties.Priority = alerttypes.AlertPrioritySchemaToProtoMap[extractAlertPriority(alertResourceModel.Priority)].Ptr()
	properties.GroupByKeys = groupBy
	properties.IncidentsSettings = incidentsSettings
	properties.NotificationGroup = notificationGroup
	properties.EntityLabels = &labels
	properties.PhantomMode = alertResourceModel.PhantomMode.ValueBoolPointer()
	properties.ActiveOn = schedule
	if utils.ObjIsNullOrUnknown(thresholdObject) {
		return properties, nil
	}

	rules, diags := extractThresholdRules(ctx, thresholdModel.Rules)
	if diags.HasError() {
		return nil, diags
	}
	undetected, diags := extractUndetectedValuesManagement(ctx, thresholdModel.UndetectedValuesManagement)
	if diags.HasError() {
		return nil, diags
	}
	noDataPolicy, diags := extractNoDataPolicy(ctx, thresholdModel.NoDataPolicy)
	if diags.HasError() {
		return nil, diags
	}

	properties.LogsThreshold = &alerts.LogsThresholdType{
		LogsFilter:                 logsFilter,
		Rules:                      rules,
		NotificationPayloadFilter:  notificationPayloadFilter,
		UndetectedValuesManagement: undetected,
		NoDataPolicy:               noDataPolicy,
		EvaluationDelayMs:          extractCustomEvaluationDelay(thresholdModel.CustomEvaluationDelay),
	}

	properties.Type = alerts.ALERTDEFTYPE_ALERT_DEF_TYPE_LOGS_THRESHOLD.Ptr()
	return properties, nil
}

func extractThresholdRules(ctx context.Context, elements types.Set) ([]alerts.LogsThresholdRule, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	rules := make([]alerts.LogsThresholdRule, len(elements.Elements()))
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

		rules[i] = alerts.LogsThresholdRule{
			Condition: condition,
			Override:  override,
		}
	}
	if diags.HasError() {
		return nil, diags
	}
	return rules, nil
}

func extractLogsThresholdCondition(ctx context.Context, condition types.Object) (*alerts.LogsThresholdCondition, diag.Diagnostics) {
	if condition.IsNull() || condition.IsUnknown() {
		return nil, nil
	}

	var conditionModel alerttypes.LogsThresholdConditionModel
	if diags := condition.As(ctx, &conditionModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}
	conditionType := alerts.LOGSTHRESHOLDCONDITIONTYPE_LOGS_THRESHOLD_CONDITION_TYPE_MORE_THAN_OR_UNSPECIFIED
	if !conditionModel.ConditionType.IsNull() && !conditionModel.ConditionType.IsUnknown() {
		conditionType = alerttypes.LogsThresholdConditionToProtoMap[conditionModel.ConditionType.ValueString()]
	}
	timeWindow := conditionModel.TimeWindow.ValueString()
	if timeWindow == "" {
		timeWindow = alerttypes.LogsTimeWindowValueProtoToSchemaMap[alerts.LOGSTIMEWINDOWVALUE_LOGS_TIME_WINDOW_VALUE_MINUTES_5_OR_UNSPECIFIED]
	}
	return &alerts.LogsThresholdCondition{
		Threshold: conditionModel.Threshold.ValueFloat64Pointer(),
		TimeWindow: &alerts.LogsTimeWindow{
			LogsTimeWindowSpecificValue: alerttypes.LogsTimeWindowValueSchemaToProtoMap[timeWindow].Ptr(),
		},
		ConditionType: conditionType.Ptr(),
	}, nil
}

func extractUndetectedValuesManagement(ctx context.Context, management types.Object) (*alerts.V3UndetectedValuesManagement, diag.Diagnostics) {
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

	var autoRetireTimeframe *alerts.V3AutoRetireTimeframe
	if !(managementModel.AutoRetireTimeframe.IsNull() || managementModel.AutoRetireTimeframe.IsUnknown()) {
		autoRetireTimeframe = new(alerts.V3AutoRetireTimeframe)
		autoRetireTimeFrameModel := managementModel.AutoRetireTimeframe.ValueString()
		if autoRetireTimeFrameModel == "" {
			autoRetireTimeFrameModel = alerttypes.AutoRetireTimeframeProtoToSchemaMap[alerts.V3AUTORETIRETIMEFRAME_AUTO_RETIRE_TIMEFRAME_NEVER_OR_UNSPECIFIED]
		}
		*autoRetireTimeframe = alerttypes.AutoRetireTimeframeSchemaToProtoMap[autoRetireTimeFrameModel]
	}

	return &alerts.V3UndetectedValuesManagement{
		TriggerUndetectedValues: managementModel.TriggerUndetectedValues.ValueBoolPointer(),
		AutoRetireTimeframe:     autoRetireTimeframe,
	}, nil
}

func extractNoDataPolicy(ctx context.Context, noDataPolicy types.Object) (*alerts.NoDataPolicy, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(noDataPolicy) {
		return nil, nil
	}
	var policyModel alerttypes.NoDataPolicyModel
	if diags := noDataPolicy.As(ctx, &policyModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}
	if policyModel.State.IsNull() && policyModel.AutoRetireSeconds.IsNull() {
		return nil, nil
	}
	out := &alerts.NoDataPolicy{}
	if !(policyModel.AutoRetireSeconds.IsNull() || policyModel.AutoRetireSeconds.IsUnknown()) {
		sec := int32(policyModel.AutoRetireSeconds.ValueInt64())
		out.AutoRetireSeconds = &sec
	}
	if !(policyModel.State.IsNull() || policyModel.State.IsUnknown()) {
		stateStr := policyModel.State.ValueString()
		if stateStr != "" {
			out.State = alerttypes.NoDataPolicyStateSchemaToProtoMap[stateStr].Ptr()
		}
	}
	return out, nil
}

func expandLogsAnomalyAlertTypeDefinition(ctx context.Context, properties *alerts.AlertDefProperties, anomaly types.Object, alertResourceModel alerttypes.AlertResourceModel) (*alerts.AlertDefProperties, diag.Diagnostics) {
	var anomalyModel alerttypes.LogsAnomalyModel
	if diags := anomaly.As(ctx, &anomalyModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	logsFilter, diags := extractLogsFilter(ctx, anomalyModel.LogsFilter)
	if diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := utils.TypeStringElementsToStringSlice(ctx, anomalyModel.NotificationPayloadFilter.Elements())
	if diags.HasError() {
		return nil, diags
	}

	groupBy, diags := utils.TypeStringElementsToStringSlice(ctx, alertResourceModel.GroupBy.Elements())
	if diags.HasError() {
		return nil, diags
	}

	incidentsSettings, diags := extractIncidentsSettings(ctx, alertResourceModel.IncidentsSettings)
	if diags.HasError() {
		return nil, diags
	}
	notificationGroup, diags := extractNotificationGroup(ctx, alertResourceModel.NotificationGroup)
	if diags.HasError() {
		return nil, diags
	}

	labels, diags := utils.TypeMapToStringMap(ctx, alertResourceModel.Labels)
	if diags.HasError() {
		return nil, diags
	}
	schedule, diags := expandActiveOnSchedule(ctx, alertResourceModel.Schedule)
	if diags.HasError() {
		return nil, diags
	}
	properties.Name = alertResourceModel.Name.ValueStringPointer()
	properties.Description = alertResourceModel.Description.ValueStringPointer()
	properties.Enabled = alertResourceModel.Enabled.ValueBoolPointer()
	properties.Priority = alerttypes.AlertPrioritySchemaToProtoMap[extractAlertPriority(alertResourceModel.Priority)].Ptr()
	properties.GroupByKeys = groupBy
	properties.IncidentsSettings = incidentsSettings
	properties.NotificationGroup = notificationGroup
	properties.EntityLabels = &labels
	properties.PhantomMode = alertResourceModel.PhantomMode.ValueBoolPointer()
	properties.ActiveOn = schedule
	if utils.ObjIsNullOrUnknown(anomaly) {
		return properties, nil
	}

	rules, diags := extractAnomalyRules(ctx, anomalyModel.Rules)
	if diags.HasError() {
		return nil, diags
	}

	var anomalyAlertSettings *alerts.AnomalyAlertSettings
	if !anomalyModel.PercentageOfDeviation.IsNull() && !anomalyModel.PercentageOfDeviation.IsUnknown() {
		percentageValue := float32(anomalyModel.PercentageOfDeviation.ValueFloat64())
		anomalyAlertSettings = &alerts.AnomalyAlertSettings{
			PercentageOfDeviation: &percentageValue,
		}
	}

	properties.LogsAnomaly = &alerts.LogsAnomalyType{
		LogsFilter:                logsFilter,
		Rules:                     rules,
		NotificationPayloadFilter: notificationPayloadFilter,
		EvaluationDelayMs:         extractCustomEvaluationDelay(anomalyModel.CustomEvaluationDelay),
		AnomalyAlertSettings:      anomalyAlertSettings,
	}

	properties.Type = alerts.ALERTDEFTYPE_ALERT_DEF_TYPE_LOGS_ANOMALY.Ptr()
	return properties, nil
}

func extractAnomalyRules(ctx context.Context, elements types.Set) ([]alerts.LogsAnomalyRule, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	rules := make([]alerts.LogsAnomalyRule, len(elements.Elements()))
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

		conditionType := alerts.LOGSANOMALYCONDITIONTYPE_LOGS_ANOMALY_CONDITION_TYPE_MORE_THAN_USUAL_OR_UNSPECIFIED
		if !condition.ConditionType.IsNull() && !condition.ConditionType.IsUnknown() {
			conditionType = alerttypes.LogsAnomalyConditionSchemaToProtoMap[condition.ConditionType.ValueString()]
		}

		timeWindow := condition.TimeWindow.ValueString()
		if timeWindow == "" {
			timeWindow = alerttypes.LogsTimeWindowValueProtoToSchemaMap[alerts.LOGSTIMEWINDOWVALUE_LOGS_TIME_WINDOW_VALUE_MINUTES_5_OR_UNSPECIFIED]
		}
		rules[i] = alerts.LogsAnomalyRule{
			Condition: &alerts.LogsAnomalyCondition{
				MinimumThreshold: condition.MinimumThreshold.ValueFloat64Pointer(),
				TimeWindow: &alerts.LogsTimeWindow{
					LogsTimeWindowSpecificValue: alerttypes.LogsTimeWindowValueSchemaToProtoMap[timeWindow].Ptr(),
				},
				ConditionType: conditionType.Ptr(),
			},
		}
	}
	if diags.HasError() {
		return nil, diags
	}
	return rules, nil
}

func expandLogsRatioThresholdTypeDefinition(ctx context.Context, properties *alerts.AlertDefProperties, ratioThreshold types.Object, alertResourceModel alerttypes.AlertResourceModel) (*alerts.AlertDefProperties, diag.Diagnostics) {
	var ratioThresholdModel alerttypes.LogsRatioThresholdModel
	if diags := ratioThreshold.As(ctx, &ratioThresholdModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := utils.TypeStringElementsToStringSlice(ctx, ratioThresholdModel.NotificationPayloadFilter.Elements())
	if diags.HasError() {
		return nil, diags
	}

	groupBy, diags := utils.TypeStringElementsToStringSlice(ctx, alertResourceModel.GroupBy.Elements())
	if diags.HasError() {
		return nil, diags
	}

	incidentsSettings, diags := extractIncidentsSettings(ctx, alertResourceModel.IncidentsSettings)
	if diags.HasError() {
		return nil, diags
	}
	notificationGroup, diags := extractNotificationGroup(ctx, alertResourceModel.NotificationGroup)
	if diags.HasError() {
		return nil, diags
	}

	labels, diags := utils.TypeMapToStringMap(ctx, alertResourceModel.Labels)
	if diags.HasError() {
		return nil, diags
	}
	schedule, diags := expandActiveOnSchedule(ctx, alertResourceModel.Schedule)
	if diags.HasError() {
		return nil, diags
	}
	properties.Name = alertResourceModel.Name.ValueStringPointer()
	properties.Description = alertResourceModel.Description.ValueStringPointer()
	properties.Enabled = alertResourceModel.Enabled.ValueBoolPointer()
	properties.Priority = alerttypes.AlertPrioritySchemaToProtoMap[extractAlertPriority(alertResourceModel.Priority)].Ptr()
	properties.GroupByKeys = groupBy
	properties.IncidentsSettings = incidentsSettings
	properties.NotificationGroup = notificationGroup
	properties.EntityLabels = &labels
	properties.PhantomMode = alertResourceModel.PhantomMode.ValueBoolPointer()
	properties.ActiveOn = schedule
	if utils.ObjIsNullOrUnknown(ratioThreshold) {
		return properties, nil
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

	groupByFor := ratioThresholdModel.GroupByFor.ValueString()
	if groupByFor == "" {
		groupByFor = alerttypes.LogsRatioGroupByForProtoToSchemaMap[alerts.LOGSRATIOGROUPBYFOR_LOGS_RATIO_GROUP_BY_FOR_BOTH_OR_UNSPECIFIED]
	}
	undetectedValuesManagement, diags := extractUndetectedValuesManagement(ctx, ratioThresholdModel.UndetectedValuesManagement)
	if diags.HasError() {
		return nil, diags
	}
	properties.LogsRatioThreshold = &alerts.LogsRatioThresholdType{
		Numerator:                  numeratorLogsFilter,
		NumeratorAlias:             ratioThresholdModel.NumeratorAlias.ValueStringPointer(),
		Denominator:                denominatorLogsFilter,
		DenominatorAlias:           ratioThresholdModel.DenominatorAlias.ValueStringPointer(),
		Rules:                      rules,
		NotificationPayloadFilter:  notificationPayloadFilter,
		GroupByFor:                 alerttypes.LogsRatioGroupByForSchemaToProtoMap[groupByFor].Ptr(),
		EvaluationDelayMs:          extractCustomEvaluationDelay(ratioThresholdModel.CustomEvaluationDelay),
		IgnoreInfinity:             ratioThresholdModel.IgnoreInfinity.ValueBoolPointer(),
		UndetectedValuesManagement: undetectedValuesManagement,
	}
	properties.Type = alerts.ALERTDEFTYPE_ALERT_DEF_TYPE_LOGS_RATIO_THRESHOLD.Ptr()
	return properties, nil
}

func extractRatioRules(ctx context.Context, elements types.Set) ([]alerts.LogsRatioRules, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	rules := make([]alerts.LogsRatioRules, len(elements.Elements()))
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
		rules[i] = alerts.LogsRatioRules{
			Condition: condition,
			Override:  override,
		}
	}
	if diags.HasError() {
		return nil, diags
	}
	return rules, nil
}

func extractAlertOverride(ctx context.Context, override types.Object) (*alerts.AlertDefOverride, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(override) {
		return nil, nil
	}

	var overrideModel alerttypes.AlertOverrideModel
	if diags := override.As(ctx, &overrideModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	return &alerts.AlertDefOverride{
		Priority: alerttypes.AlertPrioritySchemaToProtoMap[extractAlertPriority(overrideModel.Priority)].Ptr(),
	}, nil
}

func extractLogsRatioCondition(ctx context.Context, condition types.Object) (*alerts.LogsRatioCondition, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(condition) {
		return nil, nil
	}

	var conditionModel alerttypes.LogsRatioConditionModel
	if diags := condition.As(ctx, &conditionModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	conditionType := alerts.LOGSRATIOCONDITIONTYPE_LOGS_RATIO_CONDITION_TYPE_MORE_THAN_OR_UNSPECIFIED
	if !conditionModel.ConditionType.IsNull() && !conditionModel.ConditionType.IsUnknown() {
		conditionType = alerttypes.LogsRatioConditionSchemaToProtoMap[conditionModel.ConditionType.ValueString()]
	}
	logsRatioTimeWindowSpecificValue := conditionModel.TimeWindow.ValueString()
	if logsRatioTimeWindowSpecificValue == "" {
		logsRatioTimeWindowSpecificValue = alerttypes.LogsRatioTimeWindowValueProtoToSchemaMap[alerts.LOGSRATIOTIMEWINDOWVALUE_LOGS_RATIO_TIME_WINDOW_VALUE_MINUTES_5_OR_UNSPECIFIED]
	}
	return &alerts.LogsRatioCondition{
		Threshold: conditionModel.Threshold.ValueFloat64Pointer(),
		TimeWindow: &alerts.LogsRatioTimeWindow{
			LogsRatioTimeWindowSpecificValue: alerttypes.LogsRatioTimeWindowValueSchemaToProtoMap[logsRatioTimeWindowSpecificValue].Ptr(),
		},
		ConditionType: conditionType.Ptr(),
	}, nil
}

func expandLogsNewValueAlertTypeDefinition(ctx context.Context, properties *alerts.AlertDefProperties, newValue types.Object, alertResourceModel alerttypes.AlertResourceModel) (*alerts.AlertDefProperties, diag.Diagnostics) {
	var newValueModel alerttypes.LogsNewValueModel
	if diags := newValue.As(ctx, &newValueModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := utils.TypeStringElementsToStringSlice(ctx, newValueModel.NotificationPayloadFilter.Elements())
	if diags.HasError() {
		return nil, diags
	}

	logsFilter, diags := extractLogsFilter(ctx, newValueModel.LogsFilter)
	if diags.HasError() {
		return nil, diags
	}

	groupBy, diags := utils.TypeStringElementsToStringSlice(ctx, alertResourceModel.GroupBy.Elements())
	if diags.HasError() {
		return nil, diags
	}

	incidentsSettings, diags := extractIncidentsSettings(ctx, alertResourceModel.IncidentsSettings)
	if diags.HasError() {
		return nil, diags
	}
	notificationGroup, diags := extractNotificationGroup(ctx, alertResourceModel.NotificationGroup)
	if diags.HasError() {
		return nil, diags
	}

	labels, diags := utils.TypeMapToStringMap(ctx, alertResourceModel.Labels)
	if diags.HasError() {
		return nil, diags
	}
	schedule, diags := expandActiveOnSchedule(ctx, alertResourceModel.Schedule)
	if diags.HasError() {
		return nil, diags
	}
	properties.Name = alertResourceModel.Name.ValueStringPointer()
	properties.Description = alertResourceModel.Description.ValueStringPointer()
	properties.Enabled = alertResourceModel.Enabled.ValueBoolPointer()
	properties.Priority = alerttypes.AlertPrioritySchemaToProtoMap[extractAlertPriority(alertResourceModel.Priority)].Ptr()
	properties.GroupByKeys = groupBy
	properties.IncidentsSettings = incidentsSettings
	properties.NotificationGroup = notificationGroup
	properties.EntityLabels = &labels
	properties.PhantomMode = alertResourceModel.PhantomMode.ValueBoolPointer()
	properties.ActiveOn = schedule
	if newValue.IsNull() || newValue.IsUnknown() {
		return properties, nil
	}

	rules, diags := extractNewValueRules(ctx, newValueModel.Rules)
	if diags.HasError() {
		return nil, diags
	}
	properties.LogsNewValue = &alerts.LogsNewValueType{
		LogsFilter:                logsFilter,
		Rules:                     rules,
		NotificationPayloadFilter: notificationPayloadFilter,
	}
	properties.Type = alerts.ALERTDEFTYPE_ALERT_DEF_TYPE_LOGS_NEW_VALUE.Ptr()
	return properties, nil
}

func extractNewValueRules(ctx context.Context, elements types.Set) ([]alerts.LogsNewValueRule, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	rules := make([]alerts.LogsNewValueRule, len(elements.Elements()))
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

		rules[i] = alerts.LogsNewValueRule{
			Condition: condition,
		}
	}
	if diags.HasError() {
		return nil, diags
	}
	return rules, nil
}

func extractNewValueCondition(ctx context.Context, condition types.Object) (*alerts.LogsNewValueCondition, diag.Diagnostics) {
	if condition.IsNull() || condition.IsUnknown() {
		return nil, nil
	}

	var conditionModel alerttypes.NewValueConditionModel
	if diags := condition.As(ctx, &conditionModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}
	timeWindowValue := conditionModel.TimeWindow.ValueString()
	if timeWindowValue == "" {
		timeWindowValue = alerttypes.LogsNewValueTimeWindowValueProtoToSchemaMap[alerts.LOGSNEWVALUETIMEWINDOWVALUE_LOGS_NEW_VALUE_TIME_WINDOW_VALUE_HOURS_12_OR_UNSPECIFIED]
	}

	return &alerts.LogsNewValueCondition{
		KeypathToTrack: conditionModel.KeypathToTrack.ValueStringPointer(),
		TimeWindow: &alerts.LogsNewValueTimeWindow{
			LogsNewValueTimeWindowSpecificValue: alerttypes.LogsNewValueTimeWindowValueSchemaToProtoMap[timeWindowValue].Ptr(),
		},
	}, nil
}

func expandLogsUniqueCountAlertTypeDefinition(ctx context.Context, properties *alerts.AlertDefProperties, uniqueCount types.Object, alertResourceModel alerttypes.AlertResourceModel) (*alerts.AlertDefProperties, diag.Diagnostics) {
	var uniqueCountModel alerttypes.LogsUniqueCountModel
	if diags := uniqueCount.As(ctx, &uniqueCountModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := utils.TypeStringElementsToStringSlice(ctx, uniqueCountModel.NotificationPayloadFilter.Elements())
	if diags.HasError() {
		return nil, diags
	}

	logsFilter, diags := extractLogsFilter(ctx, uniqueCountModel.LogsFilter)
	if diags.HasError() {
		return nil, diags
	}

	groupBy, diags := utils.TypeStringElementsToStringSlice(ctx, alertResourceModel.GroupBy.Elements())
	if diags.HasError() {
		return nil, diags
	}

	incidentsSettings, diags := extractIncidentsSettings(ctx, alertResourceModel.IncidentsSettings)
	if diags.HasError() {
		return nil, diags
	}
	notificationGroup, diags := extractNotificationGroup(ctx, alertResourceModel.NotificationGroup)
	if diags.HasError() {
		return nil, diags
	}

	labels, diags := utils.TypeMapToStringMap(ctx, alertResourceModel.Labels)
	if diags.HasError() {
		return nil, diags
	}
	schedule, diags := expandActiveOnSchedule(ctx, alertResourceModel.Schedule)
	if diags.HasError() {
		return nil, diags
	}
	properties.Name = alertResourceModel.Name.ValueStringPointer()
	properties.Description = alertResourceModel.Description.ValueStringPointer()
	properties.Enabled = alertResourceModel.Enabled.ValueBoolPointer()
	properties.Priority = alerttypes.AlertPrioritySchemaToProtoMap[extractAlertPriority(alertResourceModel.Priority)].Ptr()
	properties.GroupByKeys = groupBy
	properties.IncidentsSettings = incidentsSettings
	properties.NotificationGroup = notificationGroup
	properties.EntityLabels = &labels
	properties.PhantomMode = alertResourceModel.PhantomMode.ValueBoolPointer()
	properties.ActiveOn = schedule
	if utils.ObjIsNullOrUnknown(uniqueCount) {
		return properties, nil
	}

	rules, diags := extractLogsUniqueCountRules(ctx, uniqueCountModel.Rules)
	if diags.HasError() {
		return nil, diags
	}

	var maxUniqueCountPerGroupByKey *string
	if !uniqueCountModel.MaxUniqueCountPerGroupByKey.IsNull() && !uniqueCountModel.MaxUniqueCountPerGroupByKey.IsUnknown() {
		val := strconv.FormatInt(uniqueCountModel.MaxUniqueCountPerGroupByKey.ValueInt64(), 10)
		maxUniqueCountPerGroupByKey = &val
	}
	properties.LogsUniqueCount = &alerts.LogsUniqueCountType{
		LogsFilter:                  logsFilter,
		Rules:                       rules,
		NotificationPayloadFilter:   notificationPayloadFilter,
		MaxUniqueCountPerGroupByKey: maxUniqueCountPerGroupByKey,
		UniqueCountKeypath:          uniqueCountModel.UniqueCountKeypath.ValueStringPointer(),
	}
	properties.Type = alerts.ALERTDEFTYPE_ALERT_DEF_TYPE_LOGS_UNIQUE_COUNT.Ptr()
	return properties, nil
}

func extractLogsUniqueCountRules(ctx context.Context, elements types.Set) ([]alerts.LogsUniqueCountRule, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	rules := make([]alerts.LogsUniqueCountRule, len(elements.Elements()))
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
		rules[i] = alerts.LogsUniqueCountRule{
			Condition: condition,
		}
	}
	if diags.HasError() {
		return nil, diags
	}
	return rules, nil
}

func extractLogsUniqueCountCondition(ctx context.Context, condition types.Object) (*alerts.LogsUniqueCountCondition, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(condition) {
		return nil, nil
	}

	var conditionModel alerttypes.LogsUniqueCountConditionModel
	if diags := condition.As(ctx, &conditionModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	maxUniqueCount := strconv.Itoa(int(conditionModel.MaxUniqueCount.ValueInt64()))
	timeWindow := conditionModel.TimeWindow.ValueString()
	if timeWindow == "" {
		timeWindow = alerttypes.LogsUniqueCountTimeWindowValueProtoToSchemaMap[alerts.LOGSUNIQUEVALUETIMEWINDOWVALUE_LOGS_UNIQUE_VALUE_TIME_WINDOW_VALUE_MINUTE_1_OR_UNSPECIFIED]
	}
	return &alerts.LogsUniqueCountCondition{
		MaxUniqueCount: &maxUniqueCount,
		TimeWindow: &alerts.LogsUniqueValueTimeWindow{
			LogsUniqueValueTimeWindowSpecificValue: alerttypes.LogsUniqueCountTimeWindowValueSchemaToProtoMap[timeWindow].Ptr(),
		},
	}, nil
}

func expandLogsTimeRelativeThresholdAlertTypeDefinition(ctx context.Context, properties *alerts.AlertDefProperties, relativeThreshold types.Object, alertResourceModel alerttypes.AlertResourceModel) (*alerts.AlertDefProperties, diag.Diagnostics) {
	var relativeThresholdModel alerttypes.LogsTimeRelativeThresholdModel
	if diags := relativeThreshold.As(ctx, &relativeThresholdModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := utils.TypeStringElementsToStringSlice(ctx, relativeThresholdModel.NotificationPayloadFilter.Elements())
	if diags.HasError() {
		return nil, diags
	}

	logsFilter, diags := extractLogsFilter(ctx, relativeThresholdModel.LogsFilter)
	if diags.HasError() {
		return nil, diags
	}

	groupBy, diags := utils.TypeStringElementsToStringSlice(ctx, alertResourceModel.GroupBy.Elements())
	if diags.HasError() {
		return nil, diags
	}

	incidentsSettings, diags := extractIncidentsSettings(ctx, alertResourceModel.IncidentsSettings)
	if diags.HasError() {
		return nil, diags
	}
	notificationGroup, diags := extractNotificationGroup(ctx, alertResourceModel.NotificationGroup)
	if diags.HasError() {
		return nil, diags
	}

	labels, diags := utils.TypeMapToStringMap(ctx, alertResourceModel.Labels)
	if diags.HasError() {
		return nil, diags
	}
	schedule, diags := expandActiveOnSchedule(ctx, alertResourceModel.Schedule)
	if diags.HasError() {
		return nil, diags
	}
	properties.Name = alertResourceModel.Name.ValueStringPointer()
	properties.Description = alertResourceModel.Description.ValueStringPointer()
	properties.Enabled = alertResourceModel.Enabled.ValueBoolPointer()
	properties.Priority = alerttypes.AlertPrioritySchemaToProtoMap[extractAlertPriority(alertResourceModel.Priority)].Ptr()
	properties.GroupByKeys = groupBy
	properties.IncidentsSettings = incidentsSettings
	properties.NotificationGroup = notificationGroup
	properties.EntityLabels = &labels
	properties.PhantomMode = alertResourceModel.PhantomMode.ValueBoolPointer()
	properties.ActiveOn = schedule
	if utils.ObjIsNullOrUnknown(relativeThreshold) {
		return properties, nil
	}

	undetected, diags := extractUndetectedValuesManagement(ctx, relativeThresholdModel.UndetectedValuesManagement)
	if diags.HasError() {
		return nil, diags
	}

	rules, diags := extractTimeRelativeThresholdRules(ctx, relativeThresholdModel.Rules)
	if diags.HasError() {
		return nil, diags
	}
	properties.LogsTimeRelativeThreshold = &alerts.LogsTimeRelativeThresholdType{
		LogsFilter:                 logsFilter,
		Rules:                      rules,
		NotificationPayloadFilter:  notificationPayloadFilter,
		UndetectedValuesManagement: undetected,
		EvaluationDelayMs:          extractCustomEvaluationDelay(relativeThresholdModel.CustomEvaluationDelay),
		IgnoreInfinity:             relativeThresholdModel.IgnoreInfinity.ValueBoolPointer(),
	}
	properties.Type = alerts.ALERTDEFTYPE_ALERT_DEF_TYPE_LOGS_TIME_RELATIVE_THRESHOLD.Ptr()
	return properties, nil
}

func extractTimeRelativeThresholdRules(ctx context.Context, elements types.Set) ([]alerts.LogsTimeRelativeRule, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	rules := make([]alerts.LogsTimeRelativeRule, len(elements.Elements()))
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

		conditionType := condition.ConditionType.ValueString()
		if conditionType == "" {
			conditionType = alerttypes.LogsTimeRelativeConditionMap[alerts.LOGSTIMERELATIVECONDITIONTYPE_LOGS_TIME_RELATIVE_CONDITION_TYPE_MORE_THAN_OR_UNSPECIFIED]
		}
		comparedTo := condition.ComparedTo.ValueString()
		if comparedTo == "" {
			comparedTo = alerttypes.LogsTimeRelativeComparedToProtoToSchemaMap[alerts.LOGSTIMERELATIVECOMPAREDTO_LOGS_TIME_RELATIVE_COMPARED_TO_PREVIOUS_HOUR_OR_UNSPECIFIED]
		}
		rules[i] = alerts.LogsTimeRelativeRule{
			Condition: &alerts.LogsTimeRelativeCondition{
				Threshold:     condition.Threshold.ValueFloat64Pointer(),
				ComparedTo:    alerttypes.LogsTimeRelativeComparedToSchemaToProtoMap[comparedTo].Ptr(),
				ConditionType: alerttypes.LogsTimeRelativeConditionToProtoMap[conditionType].Ptr(),
			},
			Override: override,
		}
	}
	if diags.HasError() {
		return nil, diags
	}
	return rules, nil
}

func expandMetricThresholdAlertTypeDefinition(ctx context.Context, properties *alerts.AlertDefProperties, metricThreshold types.Object, alertResourceModel alerttypes.AlertResourceModel) (*alerts.AlertDefProperties, diag.Diagnostics) {
	var metricThresholdModel alerttypes.MetricThresholdModel
	if diags := metricThreshold.As(ctx, &metricThresholdModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	groupBy, diags := utils.TypeStringElementsToStringSlice(ctx, alertResourceModel.GroupBy.Elements())
	if diags.HasError() {
		return nil, diags
	}

	incidentsSettings, diags := extractIncidentsSettings(ctx, alertResourceModel.IncidentsSettings)
	if diags.HasError() {
		return nil, diags
	}
	notificationGroup, diags := extractNotificationGroup(ctx, alertResourceModel.NotificationGroup)
	if diags.HasError() {
		return nil, diags
	}

	labels, diags := utils.TypeMapToStringMap(ctx, alertResourceModel.Labels)
	if diags.HasError() {
		return nil, diags
	}
	schedule, diags := expandActiveOnSchedule(ctx, alertResourceModel.Schedule)
	if diags.HasError() {
		return nil, diags
	}
	properties.Name = alertResourceModel.Name.ValueStringPointer()
	properties.Description = alertResourceModel.Description.ValueStringPointer()
	properties.Enabled = alertResourceModel.Enabled.ValueBoolPointer()
	properties.Priority = alerttypes.AlertPrioritySchemaToProtoMap[extractAlertPriority(alertResourceModel.Priority)].Ptr()
	properties.GroupByKeys = groupBy
	properties.IncidentsSettings = incidentsSettings
	properties.NotificationGroup = notificationGroup
	properties.EntityLabels = &labels
	properties.PhantomMode = alertResourceModel.PhantomMode.ValueBoolPointer()
	properties.ActiveOn = schedule
	if utils.ObjIsNullOrUnknown(metricThreshold) {
		return properties, nil
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
	noDataPolicy, diags := extractNoDataPolicy(ctx, metricThresholdModel.NoDataPolicy)
	if diags.HasError() {
		return nil, diags
	}
	properties.MetricThreshold = &alerts.MetricThresholdType{
		MetricFilter:               metricFilter,
		Rules:                      rules,
		MissingValues:              missingValues,
		UndetectedValuesManagement: undetected,
		NoDataPolicy:               noDataPolicy,
		EvaluationDelayMs:          extractCustomEvaluationDelay(metricThresholdModel.CustomEvaluationDelay),
	}
	properties.Type = alerts.ALERTDEFTYPE_ALERT_DEF_TYPE_METRIC_THRESHOLD.Ptr()

	return properties, nil
}

func extractMetricThresholdMissingValues(ctx context.Context, values types.Object) (*alerts.MetricMissingValues, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(values) {
		return nil, nil
	}

	var valuesModel alerttypes.MissingValuesModel
	if diags := values.As(ctx, &valuesModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if replaceWithZero := valuesModel.ReplaceWithZero; !(replaceWithZero.IsNull() || replaceWithZero.IsUnknown()) {
		return &alerts.MetricMissingValues{
			ReplaceWithZero: replaceWithZero.ValueBoolPointer(),
		}, nil
	} else if retainMissingValues := valuesModel.MinNonNullValuesPct; !(retainMissingValues.IsNull() || retainMissingValues.IsUnknown()) {
		return &alerts.MetricMissingValues{
			MinNonNullValuesPct: retainMissingValues.ValueInt64Pointer(),
		}, nil
	} else {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Metric Missing Values", "Metric Missing Values is not valid")}
	}
}

func extractMetricThresholdRules(ctx context.Context, elements types.Set) ([]alerts.MetricThresholdRule, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	rules := make([]alerts.MetricThresholdRule, len(elements.Elements()))
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

		conditionType := alerts.METRICTHRESHOLDCONDITIONTYPE_METRIC_THRESHOLD_CONDITION_TYPE_MORE_THAN_OR_UNSPECIFIED
		if !condition.ConditionType.IsNull() && !condition.ConditionType.IsUnknown() {
			conditionType = alerttypes.MetricsThresholdConditionToProtoMap[condition.ConditionType.ValueString()]
		}
		rules[i] = alerts.MetricThresholdRule{
			Condition: &alerts.MetricThresholdCondition{
				Threshold:     condition.Threshold.ValueFloat64Pointer(),
				ForOverPct:    condition.ForOverPct.ValueInt64Pointer(),
				OfTheLast:     expandMetricTimeWindow(condition.OfTheLast),
				ConditionType: conditionType.Ptr(),
			},
			Override: override,
		}
	}
	if diags.HasError() {
		return nil, diags
	}
	return rules, nil
}

func extractMetricFilter(ctx context.Context, filter types.Object) (*alerts.MetricFilter, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(filter) {
		return nil, nil
	}

	var filterModel alerttypes.MetricFilterModel
	if diags := filter.As(ctx, &filterModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if promql := filterModel.Promql; !(promql.IsNull() || promql.IsUnknown()) {
		return &alerts.MetricFilter{
			Promql: promql.ValueStringPointer(),
		}, nil
	}

	return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Metric Filter", "Metric Filter is not valid")}
}

func expandTracingImmediateTypeDefinition(ctx context.Context, properties *alerts.AlertDefProperties, tracingImmediate types.Object, alertResourceModel alerttypes.AlertResourceModel) (*alerts.AlertDefProperties, diag.Diagnostics) {
	var tracingImmediateModel alerttypes.TracingImmediateModel
	if diags := tracingImmediate.As(ctx, &tracingImmediateModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	groupBy, diags := utils.TypeStringElementsToStringSlice(ctx, alertResourceModel.GroupBy.Elements())
	if diags.HasError() {
		return nil, diags
	}

	incidentsSettings, diags := extractIncidentsSettings(ctx, alertResourceModel.IncidentsSettings)
	if diags.HasError() {
		return nil, diags
	}
	notificationGroup, diags := extractNotificationGroup(ctx, alertResourceModel.NotificationGroup)
	if diags.HasError() {
		return nil, diags
	}

	labels, diags := utils.TypeMapToStringMap(ctx, alertResourceModel.Labels)
	if diags.HasError() {
		return nil, diags
	}
	schedule, diags := expandActiveOnSchedule(ctx, alertResourceModel.Schedule)
	if diags.HasError() {
		return nil, diags
	}
	properties.Name = alertResourceModel.Name.ValueStringPointer()
	properties.Description = alertResourceModel.Description.ValueStringPointer()
	properties.Enabled = alertResourceModel.Enabled.ValueBoolPointer()
	properties.Priority = alerttypes.AlertPrioritySchemaToProtoMap[extractAlertPriority(alertResourceModel.Priority)].Ptr()
	properties.GroupByKeys = groupBy
	properties.IncidentsSettings = incidentsSettings
	properties.NotificationGroup = notificationGroup
	properties.EntityLabels = &labels
	properties.PhantomMode = alertResourceModel.PhantomMode.ValueBoolPointer()
	properties.ActiveOn = schedule
	if utils.ObjIsNullOrUnknown(tracingImmediate) {
		return properties, nil
	}

	tracingQuery, diags := expandTracingFilters(ctx, tracingImmediateModel.TracingFilter)
	if diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := utils.TypeStringElementsToStringSlice(ctx, tracingImmediateModel.NotificationPayloadFilter.Elements())
	if diags.HasError() {
		return nil, diags
	}
	properties.TracingImmediate = &alerts.TracingImmediateType{
		TracingFilter: &alerts.TracingFilter{
			SimpleFilter: tracingQuery,
		},
		NotificationPayloadFilter: notificationPayloadFilter,
	}
	properties.Type = alerts.ALERTDEFTYPE_ALERT_DEF_TYPE_TRACING_IMMEDIATE.Ptr()

	return properties, nil
}

func expandTracingThresholdTypeDefinition(ctx context.Context, properties *alerts.AlertDefProperties, tracingThreshold types.Object, alertResourceModel alerttypes.AlertResourceModel) (*alerts.AlertDefProperties, diag.Diagnostics) {
	var tracingThresholdModel alerttypes.TracingThresholdModel
	if diags := tracingThreshold.As(ctx, &tracingThresholdModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	groupBy, diags := utils.TypeStringElementsToStringSlice(ctx, alertResourceModel.GroupBy.Elements())
	if diags.HasError() {
		return nil, diags
	}

	incidentsSettings, diags := extractIncidentsSettings(ctx, alertResourceModel.IncidentsSettings)
	if diags.HasError() {
		return nil, diags
	}
	notificationGroup, diags := extractNotificationGroup(ctx, alertResourceModel.NotificationGroup)
	if diags.HasError() {
		return nil, diags
	}

	labels, diags := utils.TypeMapToStringMap(ctx, alertResourceModel.Labels)
	if diags.HasError() {
		return nil, diags
	}
	schedule, diags := expandActiveOnSchedule(ctx, alertResourceModel.Schedule)
	if diags.HasError() {
		return nil, diags
	}
	properties.Name = alertResourceModel.Name.ValueStringPointer()
	properties.Description = alertResourceModel.Description.ValueStringPointer()
	properties.Enabled = alertResourceModel.Enabled.ValueBoolPointer()
	properties.Priority = alerttypes.AlertPrioritySchemaToProtoMap[extractAlertPriority(alertResourceModel.Priority)].Ptr()
	properties.GroupByKeys = groupBy
	properties.IncidentsSettings = incidentsSettings
	properties.NotificationGroup = notificationGroup
	properties.EntityLabels = &labels
	properties.PhantomMode = alertResourceModel.PhantomMode.ValueBoolPointer()
	properties.ActiveOn = schedule
	if utils.ObjIsNullOrUnknown(tracingThreshold) {
		return properties, nil
	}

	tracingQuery, diags := expandTracingFilters(ctx, tracingThresholdModel.TracingFilter)
	if diags.HasError() {
		return nil, diags
	}

	notificationPayloadFilter, diags := utils.TypeStringElementsToStringSlice(ctx, tracingThresholdModel.NotificationPayloadFilter.Elements())
	if diags.HasError() {
		return nil, diags
	}

	rules, diags := extractTracingThresholdRules(ctx, tracingThresholdModel.Rules)
	if diags.HasError() {
		return nil, diags
	}

	properties.TracingThreshold = &alerts.TracingThresholdType{
		TracingFilter: &alerts.TracingFilter{
			SimpleFilter: tracingQuery,
		},
		NotificationPayloadFilter: notificationPayloadFilter,
		Rules:                     rules,
	}
	properties.Type = alerts.ALERTDEFTYPE_ALERT_DEF_TYPE_TRACING_THRESHOLD.Ptr()

	return properties, nil
}

func extractTracingThresholdRules(ctx context.Context, elements types.Set) ([]alerts.TracingThresholdRule, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	rules := make([]alerts.TracingThresholdRule, len(elements.Elements()))
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

		timeWindow := condition.TimeWindow.ValueString()
		if timeWindow == "" {
			timeWindow = alerttypes.TracingTimeWindowProtoToSchemaMap[alerts.TRACINGTIMEWINDOWVALUE_TRACING_TIME_WINDOW_VALUE_MINUTES_5_OR_UNSPECIFIED]
		}

		rules[i] = alerts.TracingThresholdRule{
			Condition: &alerts.TracingThresholdCondition{
				SpanAmount: condition.SpanAmount.ValueFloat64Pointer(),
				TimeWindow: &alerts.TracingTimeWindow{
					TracingTimeWindowValue: alerttypes.TracingTimeWindowSchemaToProtoMap[timeWindow].Ptr(),
				},
				ConditionType: alerts.TRACINGTHRESHOLDCONDITIONTYPE_TRACING_THRESHOLD_CONDITION_TYPE_MORE_THAN_OR_UNSPECIFIED.Ptr(),
			},
		}
	}
	if diags.HasError() {
		return nil, diags
	}
	return rules, nil
}

func expandTracingFilters(ctx context.Context, query types.Object) (*alerts.TracingSimpleFilter, diag.Diagnostics) {
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

	latencyThresholdMs := labelFilterModel.LatencyThresholdMs.ValueBigFloat().String()
	filter := &alerts.TracingSimpleFilter{
		TracingLabelFilters: &alerts.TracingLabelFilters{
			ApplicationName: applicationName,
			SubsystemName:   subsystemName,
			ServiceName:     serviceName,
			OperationName:   operationName,
			SpanFields:      spanFields,
		},
		LatencyThresholdMs: &latencyThresholdMs,
	}

	return filter, nil
}

func extractTracingLabelFilters(ctx context.Context, tracingLabelFilters types.Set) ([]alerts.TracingFilterType, diag.Diagnostics) {
	if tracingLabelFilters.IsNull() || tracingLabelFilters.IsUnknown() {
		return nil, nil
	}

	var filtersObjects []types.Object
	diags := tracingLabelFilters.ElementsAs(ctx, &filtersObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	var filters []alerts.TracingFilterType
	for _, filtersObject := range filtersObjects {
		filter, dgs := extractTracingLabelFilter(ctx, filtersObject)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}
		if filter != nil {
			filters = append(filters, *filter)
		}
	}
	if diags.HasError() {
		return nil, diags
	}

	return filters, nil
}

func extractTracingLabelFilter(ctx context.Context, filterModelObject types.Object) (*alerts.TracingFilterType, diag.Diagnostics) {
	var filterModel alerttypes.TracingFilterTypeModel
	if diags := filterModelObject.As(ctx, &filterModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	values, diags := utils.TypeStringElementsToStringSlice(ctx, filterModel.Values.Elements())
	if diags.HasError() {
		return nil, diags
	}

	operation := filterModel.Operation.ValueString()
	if operation == "" {
		operation = alerttypes.TracingFilterOperationProtoToSchemaMap[alerts.TRACINGFILTEROPERATIONTYPE_TRACING_FILTER_OPERATION_TYPE_IS_OR_UNSPECIFIED]
	}

	tracingTypeFilter := &alerts.TracingFilterType{
		Values:    values,
		Operation: alerttypes.TracingFilterOperationSchemaToProtoMap[operation].Ptr(),
	}
	return tracingTypeFilter, nil
}

func extractTracingSpanFieldsFilterType(ctx context.Context, spanFields types.Set) ([]alerts.TracingSpanFieldsFilterType, diag.Diagnostics) {
	if spanFields.IsNull() || spanFields.IsUnknown() {
		return nil, nil
	}

	var spanFieldsObjects []types.Object
	_ = spanFields.ElementsAs(ctx, &spanFieldsObjects, true)
	var filters []alerts.TracingSpanFieldsFilterType
	for _, element := range spanFieldsObjects {
		var filterModel alerttypes.TracingSpanFieldsFilterModel
		if diags := element.As(ctx, &filterModel, basetypes.ObjectAsOptions{}); diags.HasError() {
			return nil, diags
		}

		filterType, diags := extractTracingLabelFilter(ctx, filterModel.FilterType)
		if diags.HasError() {
			return nil, diags
		}

		filters = append(filters, alerts.TracingSpanFieldsFilterType{
			Key:        filterModel.Key.ValueStringPointer(),
			FilterType: filterType,
		})
	}

	return filters, nil
}

func expandMetricAnomalyAlertTypeDefinition(ctx context.Context, properties *alerts.AlertDefProperties, metricAnomaly types.Object, alertResourceModel alerttypes.AlertResourceModel) (*alerts.AlertDefProperties, diag.Diagnostics) {
	var metricAnomalyModel alerttypes.MetricAnomalyModel
	if diags := metricAnomaly.As(ctx, &metricAnomalyModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	groupBy, diags := utils.TypeStringElementsToStringSlice(ctx, alertResourceModel.GroupBy.Elements())
	if diags.HasError() {
		return nil, diags
	}

	incidentsSettings, diags := extractIncidentsSettings(ctx, alertResourceModel.IncidentsSettings)
	if diags.HasError() {
		return nil, diags
	}
	notificationGroup, diags := extractNotificationGroup(ctx, alertResourceModel.NotificationGroup)
	if diags.HasError() {
		return nil, diags
	}

	labels, diags := utils.TypeMapToStringMap(ctx, alertResourceModel.Labels)
	if diags.HasError() {
		return nil, diags
	}
	schedule, diags := expandActiveOnSchedule(ctx, alertResourceModel.Schedule)
	if diags.HasError() {
		return nil, diags
	}
	properties.Name = alertResourceModel.Name.ValueStringPointer()
	properties.Description = alertResourceModel.Description.ValueStringPointer()
	properties.Enabled = alertResourceModel.Enabled.ValueBoolPointer()
	properties.Priority = alerttypes.AlertPrioritySchemaToProtoMap[extractAlertPriority(alertResourceModel.Priority)].Ptr()
	properties.GroupByKeys = groupBy
	properties.IncidentsSettings = incidentsSettings
	properties.NotificationGroup = notificationGroup
	properties.EntityLabels = &labels
	properties.PhantomMode = alertResourceModel.PhantomMode.ValueBoolPointer()
	properties.ActiveOn = schedule
	if utils.ObjIsNullOrUnknown(metricAnomaly) {
		return properties, nil
	}

	metricFilter, diags := extractMetricFilter(ctx, metricAnomalyModel.MetricFilter)
	if diags.HasError() {
		return nil, diags
	}

	rules, diags := extractMetricAnomalyRules(ctx, metricAnomalyModel.Rules)
	if diags.HasError() {
		return nil, diags
	}

	var anomalyAlertSettings *alerts.AnomalyAlertSettings
	if !metricAnomalyModel.PercentageOfDeviation.IsNull() && !metricAnomalyModel.PercentageOfDeviation.IsUnknown() {
		percentageValue := float32(metricAnomalyModel.PercentageOfDeviation.ValueFloat64())
		anomalyAlertSettings = &alerts.AnomalyAlertSettings{
			PercentageOfDeviation: &percentageValue,
		}
	}

	properties.MetricAnomaly = &alerts.MetricAnomalyType{
		MetricFilter:         metricFilter,
		Rules:                rules,
		EvaluationDelayMs:    extractCustomEvaluationDelay(metricAnomalyModel.CustomEvaluationDelay),
		AnomalyAlertSettings: anomalyAlertSettings,
	}
	properties.Type = alerts.ALERTDEFTYPE_ALERT_DEF_TYPE_METRIC_ANOMALY.Ptr()

	return properties, nil
}

func extractMetricAnomalyRules(ctx context.Context, elements types.Set) ([]alerts.MetricAnomalyRule, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	rules := make([]alerts.MetricAnomalyRule, len(elements.Elements()))
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

		conditionType := alerts.METRICANOMALYCONDITIONTYPE_METRIC_ANOMALY_CONDITION_TYPE_MORE_THAN_USUAL_OR_UNSPECIFIED
		if !condition.ConditionType.IsNull() && !condition.ConditionType.IsUnknown() {
			conditionType = alerttypes.MetricAnomalyConditionToProtoMap[condition.ConditionType.ValueString()]
		}
		ofTheLast := condition.OfTheLast.ValueString()
		if ofTheLast == "" {
			ofTheLast = alerttypes.MetricFilterOperationTypeProtoToSchemaMap[alerts.METRICTIMEWINDOWVALUE_METRIC_TIME_WINDOW_VALUE_MINUTES_1_OR_UNSPECIFIED]
		}
		rules[i] = alerts.MetricAnomalyRule{
			Condition: &alerts.MetricAnomalyCondition{
				Threshold:  condition.Threshold.ValueFloat64Pointer(),
				ForOverPct: condition.ForOverPct.ValueInt64Pointer(),
				OfTheLast: &alerts.MetricTimeWindow{
					MetricTimeWindowSpecificValue: alerttypes.MetricTimeWindowValueSchemaToProtoMap[ofTheLast].Ptr(),
				},
				ConditionType:       conditionType.Ptr(),
				MinNonNullValuesPct: condition.MinNonNullValuesPct.ValueInt64Pointer(),
			},
		}
	}
	if diags.HasError() {
		return nil, diags
	}
	return rules, nil
}

func expandMetricTimeWindow(metricTimeWindow types.String) *alerts.MetricTimeWindow {
	if metricTimeWindow.IsNull() || metricTimeWindow.IsUnknown() {
		return &alerts.MetricTimeWindow{
			MetricTimeWindowSpecificValue: alerts.METRICTIMEWINDOWVALUE_METRIC_TIME_WINDOW_VALUE_MINUTES_1_OR_UNSPECIFIED.Ptr(),
		}
	}
	timeWindowStr := metricTimeWindow.ValueString()
	if timeWindowStr == "" {
		timeWindowStr = alerttypes.MetricFilterOperationTypeProtoToSchemaMap[alerts.METRICTIMEWINDOWVALUE_METRIC_TIME_WINDOW_VALUE_MINUTES_1_OR_UNSPECIFIED]
		return &alerts.MetricTimeWindow{
			MetricTimeWindowSpecificValue: alerttypes.MetricTimeWindowValueSchemaToProtoMap[timeWindowStr].Ptr(),
		}
	} else if timeWindow, ok := alerttypes.MetricTimeWindowValueSchemaToProtoMap[timeWindowStr]; ok {
		return &alerts.MetricTimeWindow{
			MetricTimeWindowSpecificValue: timeWindow.Ptr(),
		}

	} else {
		return &alerts.MetricTimeWindow{
			MetricTimeWindowDynamicDuration: &timeWindowStr,
		}
	}
}

func expandFlowAlertTypeDefinition(ctx context.Context, properties *alerts.AlertDefProperties, flow types.Object, alertResourceModel alerttypes.AlertResourceModel) (*alerts.AlertDefProperties, diag.Diagnostics) {
	var flowModel alerttypes.FlowModel
	if diags := flow.As(ctx, &flowModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	groupBy, diags := utils.TypeStringElementsToStringSlice(ctx, alertResourceModel.GroupBy.Elements())
	if diags.HasError() {
		return nil, diags
	}

	incidentsSettings, diags := extractIncidentsSettings(ctx, alertResourceModel.IncidentsSettings)
	if diags.HasError() {
		return nil, diags
	}
	notificationGroup, diags := extractNotificationGroup(ctx, alertResourceModel.NotificationGroup)
	if diags.HasError() {
		return nil, diags
	}

	labels, diags := utils.TypeMapToStringMap(ctx, alertResourceModel.Labels)
	if diags.HasError() {
		return nil, diags
	}
	schedule, diags := expandActiveOnSchedule(ctx, alertResourceModel.Schedule)
	if diags.HasError() {
		return nil, diags
	}
	properties.Name = alertResourceModel.Name.ValueStringPointer()
	properties.Description = alertResourceModel.Description.ValueStringPointer()
	properties.Enabled = alertResourceModel.Enabled.ValueBoolPointer()
	properties.Priority = alerttypes.AlertPrioritySchemaToProtoMap[extractAlertPriority(alertResourceModel.Priority)].Ptr()
	properties.GroupByKeys = groupBy
	properties.IncidentsSettings = incidentsSettings
	properties.NotificationGroup = notificationGroup
	properties.EntityLabels = &labels
	properties.PhantomMode = alertResourceModel.PhantomMode.ValueBoolPointer()
	properties.ActiveOn = schedule
	if utils.ObjIsNullOrUnknown(flow) {
		return properties, nil
	}

	stages, diags := extractFlowStages(ctx, flowModel.Stages)
	if diags.HasError() {
		return nil, diags
	}

	properties.Flow = &alerts.FlowType{
		Stages:             stages,
		EnforceSuppression: flowModel.EnforceSuppression.ValueBoolPointer(),
	}
	properties.Type = alerts.ALERTDEFTYPE_ALERT_DEF_TYPE_FLOW.Ptr()
	return properties, nil
}

func extractFlowStages(ctx context.Context, stages types.List) ([]alerts.FlowStages, diag.Diagnostics) {
	if stages.IsNull() || stages.IsUnknown() {
		return nil, nil
	}

	var stagesObjects []types.Object
	diags := stages.ElementsAs(ctx, &stagesObjects, true)
	if diags.HasError() {
		return nil, diags
	}

	var flowStages []alerts.FlowStages
	for _, stageObject := range stagesObjects {
		stage, diags := extractFlowStage(ctx, stageObject)
		if diags.HasError() {
			return nil, diags
		}
		if stage != nil {
			flowStages = append(flowStages, *stage)
		}
	}

	return flowStages, nil
}

func extractFlowStage(ctx context.Context, object types.Object) (*alerts.FlowStages, diag.Diagnostics) {
	var stageModel alerttypes.FlowStageModel
	if diags := object.As(ctx, &stageModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	timeFrameMs := strconv.FormatInt(stageModel.TimeframeMs.ValueInt64(), 10)
	timeFrameType := stageModel.TimeframeType.ValueString()
	if timeFrameType == "" {
		timeFrameType = alerttypes.FlowStageTimeFrameTypeProtoToSchemaMap[alerts.TIMEFRAMETYPE_TIMEFRAME_TYPE_UNSPECIFIED]
	}
	flowStage := &alerts.FlowStages{
		TimeframeMs:   &timeFrameMs,
		TimeframeType: alerttypes.FlowStageTimeFrameTypeSchemaToProtoMap[timeFrameType].Ptr(),
	}

	if flowStagesGroups := stageModel.FlowStagesGroups; !(flowStagesGroups.IsNull() || flowStagesGroups.IsUnknown()) {
		flowStages, diags := extractFlowStagesGroups(ctx, flowStagesGroups)
		if diags.HasError() {
			return nil, diags
		}
		flowStage.FlowStagesGroups = flowStages
	}

	return flowStage, nil
}

func extractFlowStagesGroups(ctx context.Context, groups types.List) (*alerts.FlowStagesGroups, diag.Diagnostics) {
	if groups.IsNull() || groups.IsUnknown() {
		return nil, nil
	}

	var groupsObjects []types.Object
	diags := groups.ElementsAs(ctx, &groupsObjects, true)
	if diags.HasError() {
		return nil, diags
	}

	var flowStagesGroups []alerts.FlowStagesGroup
	for _, groupObject := range groupsObjects {
		group, diags := extractFlowStagesGroup(ctx, groupObject)
		if diags.HasError() {
			return nil, diags
		}
		if group != nil {
			flowStagesGroups = append(flowStagesGroups, *group)
		}
	}

	return &alerts.FlowStagesGroups{
		Groups: flowStagesGroups,
	}, nil

}

func extractFlowStagesGroup(ctx context.Context, object types.Object) (*alerts.FlowStagesGroup, diag.Diagnostics) {
	var groupModel alerttypes.FlowStagesGroupModel
	if diags := object.As(ctx, &groupModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	alertDefs, diags := extractAlertDefs(ctx, groupModel.AlertDefs)
	if diags.HasError() {
		return nil, diags
	}

	nextOp := groupModel.NextOp.ValueString()
	if nextOp == "" {
		nextOp = alerttypes.FlowStagesGroupNextOpProtoToSchemaMap[alerts.NEXTOP_NEXT_OP_AND_OR_UNSPECIFIED]
	}
	alertsOp := groupModel.AlertsOp.ValueString()
	if alertsOp == "" {
		alertsOp = alerttypes.FlowStagesGroupAlertsOpProtoToSchemaMap[alerts.ALERTSOP_ALERTS_OP_AND_OR_UNSPECIFIED]
	}
	return &alerts.FlowStagesGroup{
		AlertDefs: alertDefs,
		NextOp:    alerttypes.FlowStagesGroupNextOpSchemaToProtoMap[nextOp].Ptr(),
		AlertsOp:  alerttypes.FlowStagesGroupAlertsOpSchemaToProtoMap[alertsOp].Ptr(),
	}, nil

}

func expandSloThresholdAlertTypeDefinition(ctx context.Context, properties *alerts.AlertDefProperties, sloThreshold types.Object, alertResourceModel alerttypes.AlertResourceModel) (*alerts.AlertDefProperties, diag.Diagnostics) {
	var sloThresholdModel alerttypes.SloThresholdModel
	if diags := sloThreshold.As(ctx, &sloThresholdModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	groupBy, diags := utils.TypeStringElementsToStringSlice(ctx, alertResourceModel.GroupBy.Elements())
	if diags.HasError() {
		return nil, diags
	}

	incidentsSettings, diags := extractIncidentsSettings(ctx, alertResourceModel.IncidentsSettings)
	if diags.HasError() {
		return nil, diags
	}
	notificationGroup, diags := extractNotificationGroup(ctx, alertResourceModel.NotificationGroup)
	if diags.HasError() {
		return nil, diags
	}

	labels, diags := utils.TypeMapToStringMap(ctx, alertResourceModel.Labels)
	if diags.HasError() {
		return nil, diags
	}
	schedule, diags := expandActiveOnSchedule(ctx, alertResourceModel.Schedule)
	if diags.HasError() {
		return nil, diags
	}
	properties.Name = alertResourceModel.Name.ValueStringPointer()
	properties.Description = alertResourceModel.Description.ValueStringPointer()
	properties.Enabled = alertResourceModel.Enabled.ValueBoolPointer()
	properties.Priority = alerttypes.AlertPrioritySchemaToProtoMap[extractAlertPriority(alertResourceModel.Priority)].Ptr()
	properties.GroupByKeys = groupBy
	properties.IncidentsSettings = incidentsSettings
	properties.NotificationGroup = notificationGroup
	properties.EntityLabels = &labels
	properties.PhantomMode = alertResourceModel.PhantomMode.ValueBoolPointer()
	properties.ActiveOn = schedule
	if utils.ObjIsNullOrUnknown(sloThreshold) {
		return properties, nil
	}

	sloDef, diags := extractSloDefinition(ctx, sloThresholdModel.SloDefinition)
	if diags.HasError() {
		return nil, diags
	}

	sloThresholdType := alerts.SloThresholdType{}

	if !utils.ObjIsNullOrUnknown(sloThresholdModel.ErrorBudget) {
		errorBudget, diags := extractSloErrorBudgetThreshold(ctx, sloThresholdModel.ErrorBudget)
		if diags.HasError() {
			return nil, diags
		}
		sloThresholdType.ErrorBudget = errorBudget
		sloThresholdType.SloDefinition = sloDef
	} else if !utils.ObjIsNullOrUnknown(sloThresholdModel.BurnRate) {
		burnRate, diags := extractSloBurnRateThreshold(ctx, sloThresholdModel.BurnRate)
		if diags.HasError() {
			return nil, diags
		}
		sloThresholdType.BurnRate = burnRate
		sloThresholdType.SloDefinition = sloDef
	} else {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid SLO Threshold Type", "SLO Threshold must have either ErrorBudget or BurnRate defined")}
	}

	properties.SloThreshold = &sloThresholdType
	properties.Type = alerts.ALERTDEFTYPE_ALERT_DEF_TYPE_SLO_THRESHOLD.Ptr()
	return properties, nil
}

func extractSloDefinition(ctx context.Context, obj types.Object) (*alerts.V3SloDefinition, diag.Diagnostics) {
	var model alerttypes.SloDefinitionObject
	if diags := obj.As(ctx, &model, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	return &alerts.V3SloDefinition{
		SloId: model.SloId.ValueStringPointer(),
	}, nil
}

func extractSloErrorBudgetThreshold(ctx context.Context, obj types.Object) (*alerts.ErrorBudgetThreshold, diag.Diagnostics) {
	var model alerttypes.SloThresholdErrorBudgetModel
	if diags := obj.As(ctx, &model, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	rules, diags := extractSloThresholdRules(ctx, model.Rules)
	if diags.HasError() {
		return nil, diags
	}

	return &alerts.ErrorBudgetThreshold{Rules: rules}, nil
}

func extractSloBurnRateThreshold(ctx context.Context, obj types.Object) (*alerts.BurnRateThreshold, diag.Diagnostics) {
	var model alerttypes.SloThresholdBurnRateModel
	if diags := obj.As(ctx, &model, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	rules, diags := extractSloThresholdRules(ctx, model.Rules)
	if diags.HasError() {
		return nil, diags
	}

	burnRate := &alerts.BurnRateThreshold{}

	if !utils.ObjIsNullOrUnknown(model.Dual) {
		timeDuration, diags := extractSloTimeDuration(ctx, model.Dual)
		if diags.HasError() {
			return nil, diags
		}
		burnRate.Dual = &alerts.BurnRateTypeDual{TimeDuration: timeDuration}
		burnRate.Rules = rules
	} else if !utils.ObjIsNullOrUnknown(model.Single) {
		timeDuration, diags := extractSloTimeDuration(ctx, model.Single)
		if diags.HasError() {
			return nil, diags
		}
		burnRate.Single = &alerts.BurnRateTypeSingle{TimeDuration: timeDuration}
		burnRate.Rules = rules
	} else {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid SLO Burn Rate Type", "SLO Burn Rate must have either Dual or Single defined")}
	}

	return burnRate, nil
}

func extractSloThresholdRules(ctx context.Context, rules types.List) ([]alerts.SloThresholdRule, diag.Diagnostics) {
	if rules.IsNull() || rules.IsUnknown() {
		return nil, nil
	}

	var ruleObjs []types.Object
	diags := rules.ElementsAs(ctx, &ruleObjs, true)
	if diags.HasError() {
		return nil, diags
	}

	var result []alerts.SloThresholdRule
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

		result = append(result, alerts.SloThresholdRule{
			Condition: &alerts.SloThresholdCondition{
				Threshold: condModel.Threshold.ValueFloat64Pointer(),
			},
			Override: override,
		})
	}

	return result, nil
}

func extractSloTimeDuration(ctx context.Context, obj types.Object) (*alerts.TimeDuration, diag.Diagnostics) {
	var model alerttypes.SloThresholdDurationWrapperModel
	if diags := obj.As(ctx, &model, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	var durationModel alerttypes.SloDurationModel
	if diags := model.TimeDuration.As(ctx, &durationModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	duration := strconv.FormatInt(durationModel.Duration.ValueInt64(), 10)
	return &alerts.TimeDuration{
		Duration: &duration,
		Unit:     alerttypes.DurationUnitSchemaToProtoMap[durationModel.Unit.ValueString()].Ptr(),
	}, nil
}

func extractAlertDefs(ctx context.Context, defs types.Set) ([]alerts.FlowStagesGroupsAlertDefs, diag.Diagnostics) {
	if defs.IsNull() || defs.IsUnknown() {
		return nil, nil
	}

	var defsObjects []types.Object
	diags := defs.ElementsAs(ctx, &defsObjects, true)
	if diags.HasError() {
		return nil, diags
	}

	var alertDefs []alerts.FlowStagesGroupsAlertDefs
	for _, defObject := range defsObjects {
		def, diags := extractAlertDef(ctx, defObject)
		if diags.HasError() {
			return nil, diags
		}
		if def != nil {
			alertDefs = append(alertDefs, *def)
		}
	}

	return alertDefs, nil

}

func extractAlertDef(ctx context.Context, def types.Object) (*alerts.FlowStagesGroupsAlertDefs, diag.Diagnostics) {
	var defModel alerttypes.FlowStagesGroupsAlertDefsModel
	if diags := def.As(ctx, &defModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	return &alerts.FlowStagesGroupsAlertDefs{
		Id:  defModel.Id.ValueStringPointer(),
		Not: defModel.Not.ValueBoolPointer(),
	}, nil

}

func flattenAlert(ctx context.Context, alert alerts.AlertDef, currentSchedule *types.Object, currentNotificationGroup *types.Object) (*alerttypes.AlertResourceModel, diag.Diagnostics) {
	alertProperties := alert.AlertDefProperties

	alertSchedule, diags := flattenAlertSchedule(ctx, *alertProperties, currentSchedule)
	if diags.HasError() {
		return nil, diags
	}
	alertTypeDefinition, diags := flattenAlertTypeDefinition(ctx, alertProperties)
	if diags.HasError() {
		return nil, diags
	}
	incidentsSettings, diags := flattenIncidentsSettings(ctx, getAlertIncidentSettings(alertProperties))
	if diags.HasError() {
		return nil, diags
	}
	notificationGroup, diags := flattenNotificationGroup(ctx, getAlertNotificationGroup(alertProperties))
	if diags.HasError() {
		return nil, diags
	}
	notificationGroup, diags = preserveDestinationRetriggeringNulls(ctx, currentNotificationGroup, notificationGroup)
	if diags.HasError() {
		return nil, diags
	}
	labels, diags := types.MapValueFrom(ctx, types.StringType, getAlertEntityLabels(alertProperties))
	if diags.HasError() {
		return nil, diags
	}
	alertPriority := getAlertPriority(alertProperties)
	if alertPriority == nil {
		alertPriority = alerts.ALERTDEFPRIORITY_ALERT_DEF_PRIORITY_P5_OR_UNSPECIFIED.Ptr()
	}
	groupByKeys := getAlertGroupByKeys(alertProperties)
	groupBy := groupByKeysToStateValue(groupByKeys, alertProperties)
	dataSources, diags := flattenDataSources(ctx, alertProperties.DataSources)
	if diags.HasError() {
		return nil, diags
	}
	return &alerttypes.AlertResourceModel{
		ID:                types.StringPointerValue(alert.Id),
		Name:              types.StringPointerValue(getAlertName(alertProperties)),
		Description:       types.StringPointerValue(getAlertDescription(alertProperties)),
		Enabled:           types.BoolPointerValue(getAlertEnabled(alertProperties)),
		Priority:          types.StringValue(alerttypes.AlertPriorityProtoToSchemaMap[*alertPriority]),
		Schedule:          alertSchedule,
		TypeDefinition:    alertTypeDefinition,
		GroupBy:           groupBy,
		IncidentsSettings: incidentsSettings,
		NotificationGroup: notificationGroup,
		Labels:            labels,
		PhantomMode:       types.BoolPointerValue(getAlertPhantomMode(alertProperties)),
		Deleted:           types.BoolPointerValue(getAlertDeleted(alertProperties)),
		DataSources:       dataSources,
	}, nil
}

// preserveDestinationRetriggeringNulls keeps retriggering_period_minutes null on
// destinations where it wasn't configured. When the field is omitted the backend
// assigns the incident retriggering cadence (e.g. incidents_settings minutes) and
// echoes it back; writing that value into state would fail apply consistency and
// pin the inherited value on the next update, preventing a return to inheritance.
func preserveDestinationRetriggeringNulls(ctx context.Context, current *types.Object, flattened types.Object) (types.Object, diag.Diagnostics) {
	if current == nil || utils.ObjIsNullOrUnknown(*current) {
		return flattened, nil
	}

	var currentModel alerttypes.NotificationGroupModel
	if diags := current.As(ctx, &currentModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return flattened, diags
	}
	if currentModel.Destinations.IsNull() || currentModel.Destinations.IsUnknown() {
		return flattened, nil
	}
	var currentDestinations []alerttypes.NotificationDestinationModel
	if diags := currentModel.Destinations.ElementsAs(ctx, &currentDestinations, false); diags.HasError() {
		return flattened, diags
	}

	return clearDestinationRetriggeringWhere(ctx, flattened, func(i int) bool {
		return i < len(currentDestinations) &&
			(currentDestinations[i].RetriggeringPeriodMinutes.IsNull() || currentDestinations[i].RetriggeringPeriodMinutes.IsUnknown())
	})
}

// clearDestinationRetriggering nulls retriggering_period_minutes on every
// destination. Used on state upgrades: prior schema versions could not
// configure the field, so any echoed value is the backend's inherited default.
func clearDestinationRetriggering(ctx context.Context, flattened types.Object) (types.Object, diag.Diagnostics) {
	return clearDestinationRetriggeringWhere(ctx, flattened, func(int) bool { return true })
}

func clearDestinationRetriggeringWhere(ctx context.Context, flattened types.Object, shouldClear func(int) bool) (types.Object, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(flattened) {
		return flattened, nil
	}
	var flattenedModel alerttypes.NotificationGroupModel
	if diags := flattened.As(ctx, &flattenedModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return flattened, diags
	}
	if flattenedModel.Destinations.IsNull() || flattenedModel.Destinations.IsUnknown() {
		return flattened, nil
	}
	var flattenedDestinations []alerttypes.NotificationDestinationModel
	if diags := flattenedModel.Destinations.ElementsAs(ctx, &flattenedDestinations, false); diags.HasError() {
		return flattened, diags
	}

	for i := range flattenedDestinations {
		if shouldClear(i) {
			flattenedDestinations[i].RetriggeringPeriodMinutes = types.Int64Null()
		}
	}

	destinations, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.NotificationDestinationsV3Attr()}, flattenedDestinations)
	if diags.HasError() {
		return flattened, diags
	}
	flattenedModel.Destinations = destinations
	return types.ObjectValueFrom(ctx, alertschema.NotificationGroupV3Attr(), flattenedModel)
}

func flattenDataSources(ctx context.Context, dataSources []alerts.AlertDefDataSource) (types.List, diag.Diagnostics) {
	if len(dataSources) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: alertschema.DataSourcesAttr()}), nil
	}

	dataSourceModels := make([]alerttypes.DataSourceModel, 0, len(dataSources))
	for _, dataSource := range dataSources {
		dataSourceModels = append(dataSourceModels, alerttypes.DataSourceModel{
			DataSpace: types.StringPointerValue(dataSource.DataSpace),
			DataSet:   types.StringPointerValue(dataSource.DataSet),
		})
	}

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.DataSourcesAttr()}, dataSourceModels)
}

func getAlertName(alertDefProperties *alerts.AlertDefProperties) *string {
	if alertDefProperties.Flow != nil {
		return alertDefProperties.Name
	} else if alertDefProperties.LogsImmediate != nil {
		return alertDefProperties.Name
	} else if alertDefProperties.MetricAnomaly != nil {
		return alertDefProperties.Name
	} else if alertDefProperties.SloThreshold != nil {
		return alertDefProperties.Name
	} else if alertDefProperties.TracingThreshold != nil {
		return alertDefProperties.Name
	} else if alertDefProperties.LogsUniqueCount != nil {
		return alertDefProperties.Name
	} else if alertDefProperties.LogsThreshold != nil {
		return alertDefProperties.Name
	} else if alertDefProperties.MetricThreshold != nil {
		return alertDefProperties.Name
	} else if alertDefProperties.LogsTimeRelativeThreshold != nil {
		return alertDefProperties.Name
	} else if alertDefProperties.LogsNewValue != nil {
		return alertDefProperties.Name
	} else if alertDefProperties.LogsRatioThreshold != nil {
		return alertDefProperties.Name
	} else if alertDefProperties.LogsAnomaly != nil {
		return alertDefProperties.Name
	} else if alertDefProperties.TracingImmediate != nil {
		return alertDefProperties.Name
	} else {
		return nil
	}
}

func getAlertDescription(alertDefProperties *alerts.AlertDefProperties) *string {
	if alertDefProperties.Flow != nil {
		return alertDefProperties.Description
	} else if alertDefProperties.LogsImmediate != nil {
		return alertDefProperties.Description
	} else if alertDefProperties.MetricAnomaly != nil {
		return alertDefProperties.Description
	} else if alertDefProperties.SloThreshold != nil {
		return alertDefProperties.Description
	} else if alertDefProperties.TracingThreshold != nil {
		return alertDefProperties.Description
	} else if alertDefProperties.LogsUniqueCount != nil {
		return alertDefProperties.Description
	} else if alertDefProperties.LogsThreshold != nil {
		return alertDefProperties.Description
	} else if alertDefProperties.MetricThreshold != nil {
		return alertDefProperties.Description
	} else if alertDefProperties.LogsTimeRelativeThreshold != nil {
		return alertDefProperties.Description
	} else if alertDefProperties.LogsNewValue != nil {
		return alertDefProperties.Description
	} else if alertDefProperties.LogsRatioThreshold != nil {
		return alertDefProperties.Description
	} else if alertDefProperties.LogsAnomaly != nil {
		return alertDefProperties.Description
	} else if alertDefProperties.TracingImmediate != nil {
		return alertDefProperties.Description
	} else {
		return nil
	}
}

func getAlertEnabled(alertDefProperties *alerts.AlertDefProperties) *bool {
	if alertDefProperties.Flow != nil {
		return alertDefProperties.Enabled
	} else if alertDefProperties.LogsImmediate != nil {
		return alertDefProperties.Enabled
	} else if alertDefProperties.MetricAnomaly != nil {
		return alertDefProperties.Enabled
	} else if alertDefProperties.SloThreshold != nil {
		return alertDefProperties.Enabled
	} else if alertDefProperties.TracingThreshold != nil {
		return alertDefProperties.Enabled
	} else if alertDefProperties.LogsUniqueCount != nil {
		return alertDefProperties.Enabled
	} else if alertDefProperties.LogsThreshold != nil {
		return alertDefProperties.Enabled
	} else if alertDefProperties.MetricThreshold != nil {
		return alertDefProperties.Enabled
	} else if alertDefProperties.LogsTimeRelativeThreshold != nil {
		return alertDefProperties.Enabled
	} else if alertDefProperties.LogsNewValue != nil {
		return alertDefProperties.Enabled
	} else if alertDefProperties.LogsRatioThreshold != nil {
		return alertDefProperties.Enabled
	} else if alertDefProperties.LogsAnomaly != nil {
		return alertDefProperties.Enabled
	} else if alertDefProperties.TracingImmediate != nil {
		return alertDefProperties.Enabled
	} else {
		return nil
	}
}
func getAlertIncidentSettings(alertDefProperties *alerts.AlertDefProperties) *alerts.AlertDefIncidentSettings {
	if alertDefProperties.Flow != nil {
		return alertDefProperties.IncidentsSettings
	} else if alertDefProperties.LogsImmediate != nil {
		return alertDefProperties.IncidentsSettings
	} else if alertDefProperties.MetricAnomaly != nil {
		return alertDefProperties.IncidentsSettings
	} else if alertDefProperties.SloThreshold != nil {
		return alertDefProperties.IncidentsSettings
	} else if alertDefProperties.TracingThreshold != nil {
		return alertDefProperties.IncidentsSettings
	} else if alertDefProperties.LogsUniqueCount != nil {
		return alertDefProperties.IncidentsSettings
	} else if alertDefProperties.LogsThreshold != nil {
		return alertDefProperties.IncidentsSettings
	} else if alertDefProperties.MetricThreshold != nil {
		return alertDefProperties.IncidentsSettings
	} else if alertDefProperties.LogsTimeRelativeThreshold != nil {
		return alertDefProperties.IncidentsSettings
	} else if alertDefProperties.LogsNewValue != nil {
		return alertDefProperties.IncidentsSettings
	} else if alertDefProperties.LogsRatioThreshold != nil {
		return alertDefProperties.IncidentsSettings
	} else if alertDefProperties.LogsAnomaly != nil {
		return alertDefProperties.IncidentsSettings
	} else if alertDefProperties.TracingImmediate != nil {
		return alertDefProperties.IncidentsSettings
	} else {
		return nil
	}
}

func getAlertEntityLabels(alertDefProperties *alerts.AlertDefProperties) *map[string]string {
	if alertDefProperties.Flow != nil {
		return alertDefProperties.EntityLabels
	} else if alertDefProperties.LogsImmediate != nil {
		return alertDefProperties.EntityLabels
	} else if alertDefProperties.MetricAnomaly != nil {
		return alertDefProperties.EntityLabels
	} else if alertDefProperties.SloThreshold != nil {
		return alertDefProperties.EntityLabels
	} else if alertDefProperties.TracingThreshold != nil {
		return alertDefProperties.EntityLabels
	} else if alertDefProperties.LogsUniqueCount != nil {
		return alertDefProperties.EntityLabels
	} else if alertDefProperties.LogsThreshold != nil {
		return alertDefProperties.EntityLabels
	} else if alertDefProperties.MetricThreshold != nil {
		return alertDefProperties.EntityLabels
	} else if alertDefProperties.LogsTimeRelativeThreshold != nil {
		return alertDefProperties.EntityLabels
	} else if alertDefProperties.LogsNewValue != nil {
		return alertDefProperties.EntityLabels
	} else if alertDefProperties.LogsRatioThreshold != nil {
		return alertDefProperties.EntityLabels
	} else if alertDefProperties.LogsAnomaly != nil {
		return alertDefProperties.EntityLabels
	} else if alertDefProperties.TracingImmediate != nil {
		return alertDefProperties.EntityLabels
	} else {
		return nil
	}
}

func getAlertNotificationGroup(alertDefProperties *alerts.AlertDefProperties) *alerts.AlertDefNotificationGroup {
	if alertDefProperties.Flow != nil {
		return alertDefProperties.NotificationGroup
	} else if alertDefProperties.LogsImmediate != nil {
		return alertDefProperties.NotificationGroup
	} else if alertDefProperties.MetricAnomaly != nil {
		return alertDefProperties.NotificationGroup
	} else if alertDefProperties.SloThreshold != nil {
		return alertDefProperties.NotificationGroup
	} else if alertDefProperties.TracingThreshold != nil {
		return alertDefProperties.NotificationGroup
	} else if alertDefProperties.LogsUniqueCount != nil {
		return alertDefProperties.NotificationGroup
	} else if alertDefProperties.LogsThreshold != nil {
		return alertDefProperties.NotificationGroup
	} else if alertDefProperties.MetricThreshold != nil {
		return alertDefProperties.NotificationGroup
	} else if alertDefProperties.LogsTimeRelativeThreshold != nil {
		return alertDefProperties.NotificationGroup
	} else if alertDefProperties.LogsNewValue != nil {
		return alertDefProperties.NotificationGroup
	} else if alertDefProperties.LogsRatioThreshold != nil {
		return alertDefProperties.NotificationGroup
	} else if alertDefProperties.LogsAnomaly != nil {
		return alertDefProperties.NotificationGroup
	} else if alertDefProperties.TracingImmediate != nil {
		return alertDefProperties.NotificationGroup
	} else {
		return nil
	}
}

func getAlertPriority(alertDefProperties *alerts.AlertDefProperties) *alerts.AlertDefPriority {
	if alertDefProperties.Flow != nil {
		return alertDefProperties.Priority
	} else if alertDefProperties.LogsImmediate != nil {
		return alertDefProperties.Priority
	} else if alertDefProperties.MetricAnomaly != nil {
		return alertDefProperties.Priority
	} else if alertDefProperties.SloThreshold != nil {
		return alertDefProperties.Priority
	} else if alertDefProperties.TracingThreshold != nil {
		return alertDefProperties.Priority
	} else if alertDefProperties.LogsUniqueCount != nil {
		return alertDefProperties.Priority
	} else if alertDefProperties.LogsThreshold != nil {
		return alertDefProperties.Priority
	} else if alertDefProperties.MetricThreshold != nil {
		return alertDefProperties.Priority
	} else if alertDefProperties.LogsTimeRelativeThreshold != nil {
		return alertDefProperties.Priority
	} else if alertDefProperties.LogsNewValue != nil {
		return alertDefProperties.Priority
	} else if alertDefProperties.LogsRatioThreshold != nil {
		return alertDefProperties.Priority
	} else if alertDefProperties.LogsAnomaly != nil {
		return alertDefProperties.Priority
	} else if alertDefProperties.TracingImmediate != nil {
		return alertDefProperties.Priority
	} else {
		return alerts.ALERTDEFPRIORITY_ALERT_DEF_PRIORITY_P5_OR_UNSPECIFIED.Ptr()
	}
}

func groupByKeysToStateValue(keys []string, alertDefProperties *alerts.AlertDefProperties) types.List {
	// For alert types that use the group_by plan modifier (slo_threshold, tracing_threshold, flow),
	// use empty list instead of null when there are no keys so plan and read are consistent.
	// Other types plan group_by as null when unset, so we must return null for empty to avoid
	// "was null, but now cty.ListValEmpty" on apply.
	if len(keys) == 0 {
		if alertDefProperties != nil && alertTypeUsesGroupByPlanModifier(alertDefProperties) {
			return types.ListValueMust(types.StringType, []attr.Value{})
		}
		return types.ListNull(types.StringType)
	}
	return utils.StringSliceToTypeStringList(keys)
}

// alertTypeUsesGroupByPlanModifier returns true for alert types that have the special group_by
// plan modifier (unknown when state null, state value when state set). Only for these do we
// normalize empty group_by to [] instead of null on read.
func alertTypeUsesGroupByPlanModifier(alertDefProperties *alerts.AlertDefProperties) bool {
	return alertDefProperties.SloThreshold != nil ||
		alertDefProperties.TracingThreshold != nil ||
		alertDefProperties.Flow != nil
}

func getAlertGroupByKeys(alertDefProperties *alerts.AlertDefProperties) []string {
	if alertDefProperties.Flow != nil {
		return alertDefProperties.GroupByKeys
	} else if alertDefProperties.LogsImmediate != nil {
		return alertDefProperties.GroupByKeys
	} else if alertDefProperties.MetricAnomaly != nil {
		return alertDefProperties.GroupByKeys
	} else if alertDefProperties.SloThreshold != nil {
		return alertDefProperties.GroupByKeys
	} else if alertDefProperties.TracingThreshold != nil {
		return alertDefProperties.GroupByKeys
	} else if alertDefProperties.LogsUniqueCount != nil {
		return alertDefProperties.GroupByKeys
	} else if alertDefProperties.LogsThreshold != nil {
		return alertDefProperties.GroupByKeys
	} else if alertDefProperties.MetricThreshold != nil {
		return alertDefProperties.GroupByKeys
	} else if alertDefProperties.LogsTimeRelativeThreshold != nil {
		return alertDefProperties.GroupByKeys
	} else if alertDefProperties.LogsNewValue != nil {
		return alertDefProperties.GroupByKeys
	} else if alertDefProperties.LogsRatioThreshold != nil {
		return alertDefProperties.GroupByKeys
	} else if alertDefProperties.LogsAnomaly != nil {
		return alertDefProperties.GroupByKeys
	} else if alertDefProperties.TracingImmediate != nil {
		return alertDefProperties.GroupByKeys
	} else {
		return nil
	}
}

func getAlertPhantomMode(alertDefProperties *alerts.AlertDefProperties) *bool {
	if alertDefProperties.Flow != nil {
		return alertDefProperties.PhantomMode
	} else if alertDefProperties.LogsImmediate != nil {
		return alertDefProperties.PhantomMode
	} else if alertDefProperties.MetricAnomaly != nil {
		return alertDefProperties.PhantomMode
	} else if alertDefProperties.SloThreshold != nil {
		return alertDefProperties.PhantomMode
	} else if alertDefProperties.TracingThreshold != nil {
		return alertDefProperties.PhantomMode
	} else if alertDefProperties.LogsUniqueCount != nil {
		return alertDefProperties.PhantomMode
	} else if alertDefProperties.LogsThreshold != nil {
		return alertDefProperties.PhantomMode
	} else if alertDefProperties.MetricThreshold != nil {
		return alertDefProperties.PhantomMode
	} else if alertDefProperties.LogsTimeRelativeThreshold != nil {
		return alertDefProperties.PhantomMode
	} else if alertDefProperties.LogsNewValue != nil {
		return alertDefProperties.PhantomMode
	} else if alertDefProperties.LogsRatioThreshold != nil {
		return alertDefProperties.PhantomMode
	} else if alertDefProperties.LogsAnomaly != nil {
		return alertDefProperties.PhantomMode
	} else if alertDefProperties.TracingImmediate != nil {
		return alertDefProperties.PhantomMode
	} else {
		return nil
	}
}

func getAlertDeleted(alertDefProperties *alerts.AlertDefProperties) *bool {
	if alertDefProperties.Flow != nil {
		return alertDefProperties.Deleted
	} else if alertDefProperties.LogsImmediate != nil {
		return alertDefProperties.Deleted
	} else if alertDefProperties.MetricAnomaly != nil {
		return alertDefProperties.Deleted
	} else if alertDefProperties.SloThreshold != nil {
		return alertDefProperties.Deleted
	} else if alertDefProperties.TracingThreshold != nil {
		return alertDefProperties.Deleted
	} else if alertDefProperties.LogsUniqueCount != nil {
		return alertDefProperties.Deleted
	} else if alertDefProperties.LogsThreshold != nil {
		return alertDefProperties.Deleted
	} else if alertDefProperties.MetricThreshold != nil {
		return alertDefProperties.Deleted
	} else if alertDefProperties.LogsTimeRelativeThreshold != nil {
		return alertDefProperties.Deleted
	} else if alertDefProperties.LogsNewValue != nil {
		return alertDefProperties.Deleted
	} else if alertDefProperties.LogsRatioThreshold != nil {
		return alertDefProperties.Deleted
	} else if alertDefProperties.LogsAnomaly != nil {
		return alertDefProperties.Deleted
	} else if alertDefProperties.TracingImmediate != nil {
		return alertDefProperties.Deleted
	} else {
		return nil
	}
}

func flattenNotificationGroup(ctx context.Context, notificationGroup *alerts.AlertDefNotificationGroup) (types.Object, diag.Diagnostics) {
	if notificationGroup == nil {
		return types.ObjectNull(alertschema.NotificationGroupV3Attr()), nil
	}

	webhooksSettings, diags := flattenAdvancedTargetSettings(ctx, notificationGroup.Webhooks)
	if diags.HasError() {
		return types.ObjectNull(alertschema.NotificationGroupV3Attr()), diags
	}
	destinations, diags := flattenNotificationDestinations(ctx, notificationGroup.Destinations)
	if diags.HasError() {
		return types.ObjectNull(alertschema.NotificationGroupV3Attr()), diags
	}

	router, diags := flattenNotificationRouter(ctx, notificationGroup.Router)
	if diags.HasError() {
		return types.ObjectNull(alertschema.NotificationGroupV3Attr()), diags
	}

	notificationGroupModel := alerttypes.NotificationGroupModel{
		GroupByKeys:      utils.StringSliceToTypeStringList(notificationGroup.GetGroupByKeys()),
		WebhooksSettings: webhooksSettings,
		Destinations:     destinations,
		Router:           router,
	}

	return types.ObjectValueFrom(ctx, alertschema.NotificationGroupV3Attr(), notificationGroupModel)
}

func flattenAdvancedTargetSettings(ctx context.Context, webhooksSettings []alerts.AlertDefWebhooksSettings) (types.Set, diag.Diagnostics) {
	if webhooksSettings == nil {
		return types.SetNull(types.ObjectType{AttrTypes: alertschema.WebhooksSettingsAttr()}), nil
	}

	var notificationsModel []*alerttypes.WebhooksSettingsModel
	var diags diag.Diagnostics
	for _, notification := range webhooksSettings {
		retriggeringPeriod, dgs := flattenRetriggeringPeriod(ctx, &notification)
		if dgs.HasError() {
			diags.Append(dgs...)
			continue
		}

		var notifyOnValue types.String
		if notification.NotifyOn != nil {
			notifyOnValue = types.StringValue(alerttypes.NotifyOnProtoToSchemaMap[*notification.NotifyOn])
		} else {
			notifyOnValue = types.StringNull()
		}
		notificationModel := alerttypes.WebhooksSettingsModel{
			NotifyOn:           notifyOnValue,
			RetriggeringPeriod: retriggeringPeriod,
			IntegrationID:      types.StringNull(),
			Recipients:         types.SetNull(types.StringType),
		}

		integration := notification.Integration
		if integration != nil {
			if integration.IntegrationId != nil {
				integrationID := strconv.FormatInt(*integration.IntegrationId, 10)
				notificationModel.IntegrationID = types.StringValue(integrationID)
			} else if integration.Recipients != nil {
				notificationModel.Recipients = utils.StringSliceToTypeStringSet(integration.Recipients.Emails)
			}
		}
		notificationsModel = append(notificationsModel, &notificationModel)
	}

	if diags.HasError() {
		return types.SetNull(types.ObjectType{AttrTypes: alertschema.WebhooksSettingsAttr()}), diags
	}

	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.WebhooksSettingsAttr()}, notificationsModel)
}

func flattenNotificationDestinations(ctx context.Context, destinations []alerts.NotificationDestination) (types.List, diag.Diagnostics) {
	if destinations == nil {
		return types.ListNull(types.ObjectType{AttrTypes: alertschema.NotificationDestinationsV3Attr()}), nil
	}
	var destinationModels []*alerttypes.NotificationDestinationModel
	for _, destination := range destinations {
		var triggeredRoutingOverrides *alerts.V3SourceOverrides
		if destination.TriggeredRoutingOverrides != nil {
			triggeredRoutingOverrides = destination.TriggeredRoutingOverrides.ConfigOverrides
		}
		var resolvedRoutingOverrides *alerts.V3SourceOverrides
		if destination.ResolvedRouteOverrides != nil {
			resolvedRoutingOverrides = destination.ResolvedRouteOverrides.ConfigOverrides
		}
		flattenedTriggeredRoutingOverrides, diags := flattenRoutingOverrides(ctx, triggeredRoutingOverrides)
		if diags.HasError() {
			return types.ListNull(types.ObjectType{AttrTypes: alertschema.NotificationDestinationsV3Attr()}), diags
		}
		flattenedResolvedRoutingOverrides, diags := flattenRoutingOverrides(ctx, resolvedRoutingOverrides)
		if diags.HasError() {
			return types.ListNull(types.ObjectType{AttrTypes: alertschema.NotificationDestinationsV3Attr()}), diags
		}

		var notifyOn alerts.NotifyOn
		if destination.NotifyOn != nil {
			notifyOn = *destination.NotifyOn
		} else {
			notifyOn = alerts.NOTIFYON_NOTIFY_ON_TRIGGERED_ONLY_UNSPECIFIED
		}
		destinationModel := alerttypes.NotificationDestinationModel{
			ConnectorId:               types.StringValue(destination.GetConnectorId()),
			PresetId:                  types.StringValue(destination.GetPresetId()),
			NotifyOn:                  types.StringValue(alerttypes.NotifyOnProtoToSchemaMap[notifyOn]),
			TriggeredRoutingOverrides: flattenedTriggeredRoutingOverrides,
			ResolvedRoutingOverrides:  flattenedResolvedRoutingOverrides,
			RetriggeringPeriodMinutes: types.Int64PointerValue(destination.RetriggeringPeriodMinutes),
		}
		destinationModels = append(destinationModels, &destinationModel)
	}
	flattenedDestinations, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.NotificationDestinationsV3Attr()}, destinationModels)
	if diags.HasError() {
		return types.ListNull(types.ListType{ElemType: types.ObjectType{AttrTypes: alertschema.NotificationDestinationsV3Attr()}}), diags
	}
	return flattenedDestinations, nil
}

func flattenRoutingOverrides(ctx context.Context, overrides *alerts.V3SourceOverrides) (types.Object, diag.Diagnostics) {
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

func flattenNotificationRouter(ctx context.Context, notificationRouter *alerts.NotificationRouter) (types.Object, diag.Diagnostics) {
	if notificationRouter == nil {
		return types.ObjectNull(alertschema.NotificationRouterAttr()), nil
	}

	var notifyOn alerts.NotifyOn
	if notificationRouter.NotifyOn != nil {
		notifyOn = *notificationRouter.NotifyOn
	} else {
		notifyOn = alerts.NOTIFYON_NOTIFY_ON_TRIGGERED_ONLY_UNSPECIFIED
	}
	notificationRouterModel := alerttypes.NotificationRouterModel{
		NotifyOn: types.StringValue(alerttypes.NotifyOnProtoToSchemaMap[notifyOn]),
	}
	return types.ObjectValueFrom(ctx, alertschema.NotificationRouterAttr(), notificationRouterModel)
}

func flattenRetriggeringPeriod(ctx context.Context, notifications *alerts.AlertDefWebhooksSettings) (types.Object, diag.Diagnostics) {
	if notifications.Minutes == nil {
		return types.ObjectNull(alertschema.RetriggeringPeriodAttr()), nil
	} else {
		return types.ObjectValueFrom(ctx, alertschema.RetriggeringPeriodAttr(), alerttypes.RetriggeringPeriodModel{
			Minutes: types.Int64PointerValue(notifications.Minutes),
		})
	}
}

func flattenIncidentsSettings(ctx context.Context, incidentsSettings *alerts.AlertDefIncidentSettings) (types.Object, diag.Diagnostics) {
	if incidentsSettings == nil {
		return types.ObjectNull(alertschema.IncidentsSettingsAttr()), nil
	}

	retriggeringPeriod, diags := flattenIncidentsSettingsByRetriggeringPeriod(ctx, incidentsSettings)
	if diags.HasError() {
		return types.ObjectNull(alertschema.IncidentsSettingsAttr()), diags
	}

	var notifyOn alerts.NotifyOn
	if incidentsSettings.NotifyOn != nil {
		notifyOn = *incidentsSettings.NotifyOn
	} else {
		notifyOn = alerts.NOTIFYON_NOTIFY_ON_TRIGGERED_ONLY_UNSPECIFIED
	}
	incidentsSettingsModel := alerttypes.IncidentsSettingsModel{
		NotifyOn:           types.StringValue(alerttypes.NotifyOnProtoToSchemaMap[notifyOn]),
		RetriggeringPeriod: retriggeringPeriod,
	}
	return types.ObjectValueFrom(ctx, alertschema.IncidentsSettingsAttr(), incidentsSettingsModel)
}

func flattenIncidentsSettingsByRetriggeringPeriod(ctx context.Context, settings *alerts.AlertDefIncidentSettings) (types.Object, diag.Diagnostics) {
	if settings.Minutes == nil {
		return types.ObjectNull(alertschema.RetriggeringPeriodAttr()), nil
	}

	var periodModel alerttypes.RetriggeringPeriodModel
	periodModel.Minutes = types.Int64PointerValue(settings.Minutes)

	return types.ObjectValueFrom(ctx, alertschema.RetriggeringPeriodAttr(), periodModel)
}

func flattenAlertTypeDefinition(ctx context.Context, properties *alerts.AlertDefProperties) (types.Object, diag.Diagnostics) {
	if properties == nil {
		return types.ObjectNull(alertschema.AlertTypeDefinitionAttr()), nil
	}

	alertTypeDefinitionModel := emptyAlertTypeDefinitionModel()
	diags, handled := flattenLogsAlertTypeDefinition(ctx, properties, &alertTypeDefinitionModel)
	if !handled {
		diags, handled = flattenNonLogsAlertTypeDefinition(ctx, properties, &alertTypeDefinitionModel)
	}
	if !handled {
		return types.ObjectNull(alertschema.AlertTypeDefinitionAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Alert Type Definition", "Alert Type Definition is not valid")}
	}
	if diags.HasError() {
		return types.ObjectNull(alertschema.AlertTypeDefinitionAttr()), diags
	}

	return types.ObjectValueFrom(ctx, alertschema.AlertTypeDefinitionAttr(), alertTypeDefinitionModel)
}

func emptyAlertTypeDefinitionModel() alerttypes.AlertTypeDefinitionModel {
	return alerttypes.AlertTypeDefinitionModel{
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
}

func flattenLogsAlertTypeDefinition(ctx context.Context, properties *alerts.AlertDefProperties, model *alerttypes.AlertTypeDefinitionModel) (diag.Diagnostics, bool) {
	switch {
	case properties.LogsImmediate != nil:
		diags := diag.Diagnostics(nil)
		model.LogsImmediate, diags = flattenLogsImmediate(ctx, properties.LogsImmediate)
		return diags, true
	case properties.LogsThreshold != nil:
		diags := diag.Diagnostics(nil)
		model.LogsThreshold, diags = flattenLogsThreshold(ctx, properties.LogsThreshold)
		return diags, true
	case properties.LogsAnomaly != nil:
		diags := diag.Diagnostics(nil)
		model.LogsAnomaly, diags = flattenLogsAnomaly(ctx, properties.LogsAnomaly)
		return diags, true
	case properties.LogsRatioThreshold != nil:
		diags := diag.Diagnostics(nil)
		model.LogsRatioThreshold, diags = flattenLogsRatioThreshold(ctx, properties.LogsRatioThreshold)
		return diags, true
	case properties.LogsNewValue != nil:
		diags := diag.Diagnostics(nil)
		model.LogsNewValue, diags = flattenLogsNewValue(ctx, properties.LogsNewValue)
		return diags, true
	case properties.LogsUniqueCount != nil:
		diags := diag.Diagnostics(nil)
		model.LogsUniqueCount, diags = flattenLogsUniqueCount(ctx, properties.LogsUniqueCount)
		return diags, true
	case properties.LogsTimeRelativeThreshold != nil:
		diags := diag.Diagnostics(nil)
		model.LogsTimeRelativeThreshold, diags = flattenLogsTimeRelativeThreshold(ctx, properties.LogsTimeRelativeThreshold)
		return diags, true
	default:
		return nil, false
	}
}

func flattenNonLogsAlertTypeDefinition(ctx context.Context, properties *alerts.AlertDefProperties, model *alerttypes.AlertTypeDefinitionModel) (diag.Diagnostics, bool) {
	switch {
	case properties.MetricThreshold != nil:
		diags := diag.Diagnostics(nil)
		model.MetricThreshold, diags = flattenMetricThreshold(ctx, properties.MetricThreshold)
		return diags, true
	case properties.MetricAnomaly != nil:
		diags := diag.Diagnostics(nil)
		model.MetricAnomaly, diags = flattenMetricAnomaly(ctx, properties.MetricAnomaly)
		return diags, true
	case properties.TracingImmediate != nil:
		diags := diag.Diagnostics(nil)
		model.TracingImmediate, diags = flattenTracingImmediate(ctx, properties.TracingImmediate)
		return diags, true
	case properties.TracingThreshold != nil:
		diags := diag.Diagnostics(nil)
		model.TracingThreshold, diags = flattenTracingThreshold(ctx, properties.TracingThreshold)
		return diags, true
	case properties.Flow != nil:
		diags := diag.Diagnostics(nil)
		model.Flow, diags = flattenFlow(ctx, properties.Flow)
		return diags, true
	case properties.SloThreshold != nil:
		diags := diag.Diagnostics(nil)
		model.SloThreshold, diags = flattenSloThreshold(ctx, properties.SloThreshold)
		return diags, true
	default:
		return nil, false
	}
}

func flattenLogsImmediate(ctx context.Context, immediate *alerts.LogsImmediateType) (types.Object, diag.Diagnostics) {
	if immediate == nil {
		return types.ObjectNull(alertschema.LogsImmediateAttr()), nil
	}

	logsFilter, _ := immediate.GetLogsFilterOk()
	logsFilterModel, diags := flattenAlertsLogsFilter(ctx, logsFilter)
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsImmediateAttr()), diags
	}

	notificationPayloadFilter, _ := immediate.GetNotificationPayloadFilterOk()
	logsImmediateModel := alerttypes.LogsImmediateModel{
		LogsFilter:                logsFilterModel,
		NotificationPayloadFilter: utils.StringSliceToTypeStringSet(notificationPayloadFilter),
	}
	return types.ObjectValueFrom(ctx, alertschema.LogsImmediateAttr(), logsImmediateModel)
}

func flattenAlertsLogsFilter(ctx context.Context, filter *alerts.V3LogsFilter) (types.Object, diag.Diagnostics) {
	if filter == nil {
		return types.ObjectNull(alertschema.LogsFilterAttr()), nil
	}

	var diags diag.Diagnostics
	var logsFilterModel alerttypes.AlertsLogsFilterModel
	if simpleFilter := filter.SimpleFilter; simpleFilter != nil {
		logsFilterModel.SimpleFilter, diags = flattenSimpleFilter(ctx, simpleFilter)
	} else {
		return types.ObjectNull(alertschema.LogsFilterAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Logs Filter", "Only simple filter is supported, and it came back null")}
	}

	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsFilterAttr()), diags
	}

	return types.ObjectValueFrom(ctx, alertschema.LogsFilterAttr(), logsFilterModel)
}

func flattenSimpleFilter(ctx context.Context, filter *alerts.LogsSimpleFilter) (types.Object, diag.Diagnostics) {
	if filter == nil {
		return types.ObjectNull(alertschema.LuceneFilterAttr()), nil
	}

	labelFilters, _ := filter.GetLabelFiltersOk()
	luceneQuery, _ := filter.GetLuceneQueryOk()
	labelFiltersModel, diags := flattenLabelFilters(ctx, labelFilters)
	if diags.HasError() {
		return types.ObjectNull(alertschema.LuceneFilterAttr()), diags
	}

	// Normalize nil or empty lucene_query to "" so state matches config after apply.
	// API returns null when lucene_query is empty, causing "Provider produced inconsistent result after apply".
	luceneQueryVal := ""
	if luceneQuery != nil && *luceneQuery != "" {
		luceneQueryVal = *luceneQuery
	}

	return types.ObjectValueFrom(ctx, alertschema.LuceneFilterAttr(), alerttypes.SimpleFilterModel{
		LuceneQuery:  types.StringValue(luceneQueryVal),
		LabelFilters: labelFiltersModel,
	})
}

func flattenLabelFilters(ctx context.Context, filters *alerts.LabelFilters) (types.Object, diag.Diagnostics) {
	if filters == nil {
		return types.ObjectNull(alertschema.LabelFiltersAttr()), nil
	}

	applicationName, _ := filters.GetApplicationNameOk()
	applicationNameModel, diags := flattenLabelFilterTypes(ctx, applicationName)
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
		ApplicationName: applicationNameModel,
		SubsystemName:   subsystemName,
		Severities:      severities,
	})
}

func flattenLabelFilterTypes(ctx context.Context, name []alerts.LabelFilterType) (types.Set, diag.Diagnostics) {
	var labelFilterTypes []alerttypes.LabelFilterTypeModel
	var diags diag.Diagnostics
	for _, lft := range name {
		labelFilterType := alerttypes.LabelFilterTypeModel{
			Value: utils.StringPointerToTypeString(lft.Value),
		}
		if lft.Operation != nil {
			labelFilterType.Operation = types.StringValue(alerttypes.LogFilterOperationTypeProtoToSchemaMap[lft.GetOperation()])
		} else {
			labelFilterType.Operation = types.StringValue(alerttypes.LogFilterOperationTypeProtoToSchemaMap[alerts.LOGFILTEROPERATIONTYPE_LOG_FILTER_OPERATION_TYPE_IS_OR_UNSPECIFIED])
		}
		labelFilterTypes = append(labelFilterTypes, labelFilterType)
	}
	if diags.HasError() {
		return types.SetNull(types.ObjectType{AttrTypes: alertschema.LabelFilterTypesAttr()}), diags
	}
	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.LabelFilterTypesAttr()}, labelFilterTypes)

}

func flattenLogSeverities(ctx context.Context, severities []alerts.LogSeverity) (types.Set, diag.Diagnostics) {
	var result []attr.Value
	for _, severity := range severities {
		result = append(result, types.StringValue(alerttypes.LogSeverityProtoToSchemaMap[severity]))
	}
	return types.SetValueFrom(ctx, types.StringType, result)
}

func flattenLogsThreshold(ctx context.Context, threshold *alerts.LogsThresholdType) (types.Object, diag.Diagnostics) {
	if threshold == nil {
		return types.ObjectNull(alertschema.LogsThresholdAttr()), nil
	}

	logsFilter, diags := flattenAlertsLogsFilter(ctx, threshold.LogsFilter)
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsThresholdAttr()), diags
	}

	rules, diags := flattenLogsThresholdRules(ctx, threshold.Rules)
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsThresholdAttr()), diags
	}

	undetected, diags := flattenUndetectedValuesManagement(ctx, threshold.UndetectedValuesManagement)
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsThresholdAttr()), diags
	}
	noDataPolicy, diags := flattenNoDataPolicy(ctx, threshold.NoDataPolicy)
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsThresholdAttr()), diags
	}

	logsMoreThanModel := alerttypes.LogsThresholdModel{
		LogsFilter:                 logsFilter,
		Rules:                      rules,
		NotificationPayloadFilter:  utils.StringSliceToTypeStringSet(threshold.NotificationPayloadFilter),
		UndetectedValuesManagement: undetected,
		NoDataPolicy:               noDataPolicy,
		CustomEvaluationDelay:      types.Int32PointerValue(threshold.EvaluationDelayMs),
	}
	return types.ObjectValueFrom(ctx, alertschema.LogsThresholdAttr(), logsMoreThanModel)
}

func flattenLogsThresholdRules(ctx context.Context, rules []alerts.LogsThresholdRule) (types.Set, diag.Diagnostics) {
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

func flattenLogsThresholdRuleCondition(ctx context.Context, condition *alerts.LogsThresholdCondition) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(alertschema.LogsThresholdConditionAttr()), nil
	}

	conditionType := alerts.LOGSTHRESHOLDCONDITIONTYPE_LOGS_THRESHOLD_CONDITION_TYPE_MORE_THAN_OR_UNSPECIFIED
	if condition.ConditionType != nil {
		conditionType = *condition.ConditionType
	}
	return types.ObjectValueFrom(ctx, alertschema.LogsThresholdConditionAttr(), alerttypes.LogsThresholdConditionModel{
		Threshold:     types.Float64PointerValue(condition.Threshold),
		TimeWindow:    flattenLogsTimeWindow(condition.TimeWindow),
		ConditionType: types.StringValue(alerttypes.LogsThresholdConditionMap[conditionType]),
	})
}

func flattenLogsTimeWindow(timeWindow *alerts.LogsTimeWindow) types.String {
	if timeWindow == nil {
		return types.StringNull()
	}
	logsTimeWindowValue := alerts.LOGSTIMEWINDOWVALUE_LOGS_TIME_WINDOW_VALUE_MINUTES_5_OR_UNSPECIFIED
	if timeWindow.LogsTimeWindowSpecificValue != nil {
		logsTimeWindowValue = *timeWindow.LogsTimeWindowSpecificValue
	}
	return types.StringValue(alerttypes.LogsTimeWindowValueProtoToSchemaMap[logsTimeWindowValue])
}

func flattenLogsRatioTimeWindow(timeWindow *alerts.LogsRatioTimeWindow) types.String {
	if timeWindow == nil {
		return types.StringNull()
	}
	timeWindowValue := timeWindow.LogsRatioTimeWindowSpecificValue
	if timeWindowValue == nil {
		timeWindowValue = alerts.LOGSRATIOTIMEWINDOWVALUE_LOGS_RATIO_TIME_WINDOW_VALUE_MINUTES_5_OR_UNSPECIFIED.Ptr()
	}
	return types.StringValue(alerttypes.LogsRatioTimeWindowValueProtoToSchemaMap[*timeWindowValue])
}

func flattenLogsNewValueTimeWindow(timeWindow *alerts.LogsNewValueTimeWindow) types.String {
	if timeWindow == nil {
		return types.StringNull()
	}
	timeWindowValue := timeWindow.LogsNewValueTimeWindowSpecificValue
	if timeWindowValue == nil {
		timeWindowValue = alerts.LOGSNEWVALUETIMEWINDOWVALUE_LOGS_NEW_VALUE_TIME_WINDOW_VALUE_HOURS_12_OR_UNSPECIFIED.Ptr().Ptr()
	}
	return types.StringValue(alerttypes.LogsNewValueTimeWindowValueProtoToSchemaMap[*timeWindowValue])
}

func flattenUndetectedValuesManagement(ctx context.Context, undetectedValuesManagement *alerts.V3UndetectedValuesManagement) (types.Object, diag.Diagnostics) {
	var undetectedValuesManagementModel alerttypes.UndetectedValuesManagementModel
	if undetectedValuesManagement == nil {
		undetectedValuesManagementModel.TriggerUndetectedValues = types.BoolValue(false)
		undetectedValuesManagementModel.AutoRetireTimeframe = types.StringValue(alerttypes.AutoRetireTimeframeProtoToSchemaMap[alerts.V3AUTORETIRETIMEFRAME_AUTO_RETIRE_TIMEFRAME_NEVER_OR_UNSPECIFIED])
	} else {
		autoRetireTimeFrame := undetectedValuesManagement.AutoRetireTimeframe
		if autoRetireTimeFrame == nil {
			autoRetireTimeFrame = alerts.V3AUTORETIRETIMEFRAME_AUTO_RETIRE_TIMEFRAME_NEVER_OR_UNSPECIFIED.Ptr()
		}
		undetectedValuesManagementModel.TriggerUndetectedValues = types.BoolPointerValue(undetectedValuesManagement.TriggerUndetectedValues)
		undetectedValuesManagementModel.AutoRetireTimeframe = types.StringValue(alerttypes.AutoRetireTimeframeProtoToSchemaMap[*autoRetireTimeFrame])
	}
	return types.ObjectValueFrom(ctx, alertschema.UndetectedValuesManagementAttr(), undetectedValuesManagementModel)
}

func flattenNoDataPolicy(ctx context.Context, noDataPolicy *alerts.NoDataPolicy) (types.Object, diag.Diagnostics) {
	var model alerttypes.NoDataPolicyModel
	if noDataPolicy == nil {
		return types.ObjectValueFrom(ctx, alertschema.NoDataPolicyAttr(), model)
	}
	if autoRetireSeconds, ok := noDataPolicy.GetAutoRetireSecondsOk(); ok {
		model.AutoRetireSeconds = types.Int64Value(int64(*autoRetireSeconds))
	}
	if noDataPolicy.State != nil {
		model.State = types.StringValue(alerttypes.NoDataPolicyStateProtoToSchemaMap[*noDataPolicy.State])
	}
	return types.ObjectValueFrom(ctx, alertschema.NoDataPolicyAttr(), model)
}

func flattenLogsAnomaly(ctx context.Context, anomaly *alerts.LogsAnomalyType) (types.Object, diag.Diagnostics) {
	if anomaly == nil {
		return types.ObjectNull(alertschema.LogsAnomalyAttr()), nil
	}

	logsFilter, diags := flattenAlertsLogsFilter(ctx, anomaly.LogsFilter)
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

	var percentageOfDeviation types.Float64
	if anomaly.AnomalyAlertSettings != nil && anomaly.AnomalyAlertSettings.PercentageOfDeviation != nil {
		percentageOfDeviation = types.Float64Value(float64(*anomaly.AnomalyAlertSettings.PercentageOfDeviation))
	} else {
		percentageOfDeviation = types.Float64Null()
	}

	logsMoreThanUsualModel := alerttypes.LogsAnomalyModel{
		LogsFilter:                logsFilter,
		Rules:                     rules,
		NotificationPayloadFilter: utils.StringSliceToTypeStringSet(anomaly.GetNotificationPayloadFilter()),
		CustomEvaluationDelay:     types.Int32PointerValue(anomaly.EvaluationDelayMs),
		PercentageOfDeviation:     percentageOfDeviation,
	}
	return types.ObjectValueFrom(ctx, alertschema.LogsAnomalyAttr(), logsMoreThanUsualModel)
}

func flattenLogsAnomalyRuleCondition(ctx context.Context, condition *alerts.LogsAnomalyCondition) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(alertschema.LogsAnomalyConditionAttr()), nil
	}

	logsAnomalyConditionModel := alerttypes.LogsAnomalyConditionModel{
		MinimumThreshold: types.Float64PointerValue(condition.MinimumThreshold),
		TimeWindow:       flattenLogsTimeWindow(condition.TimeWindow),
		ConditionType:    types.StringValue(alerttypes.LogsAnomalyConditionMap[condition.GetConditionType()]),
	}
	if condition.ConditionType != nil {
		logsAnomalyConditionModel.ConditionType = types.StringValue(alerttypes.LogsAnomalyConditionMap[condition.GetConditionType()])
	} else {
		logsAnomalyConditionModel.ConditionType = types.StringValue(alerttypes.LogsAnomalyConditionMap[alerts.LOGSANOMALYCONDITIONTYPE_LOGS_ANOMALY_CONDITION_TYPE_MORE_THAN_USUAL_OR_UNSPECIFIED])
	}
	return types.ObjectValueFrom(ctx, alertschema.LogsAnomalyConditionAttr(), logsAnomalyConditionModel)
}

func flattenLogsRatioThreshold(ctx context.Context, ratioThreshold *alerts.LogsRatioThresholdType) (types.Object, diag.Diagnostics) {
	if ratioThreshold == nil {
		return types.ObjectNull(alertschema.LogsRatioThresholdAttr()), nil
	}

	numeratorLogsFilter, diags := flattenAlertsLogsFilter(ctx, ratioThreshold.Numerator)
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsRatioThresholdAttr()), diags
	}

	denominatorLogsFilter, diags := flattenAlertsLogsFilter(ctx, ratioThreshold.Denominator)
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsRatioThresholdAttr()), diags
	}

	rules, diags := flattenRatioThresholdRules(ctx, ratioThreshold)
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsRatioThresholdAttr()), diags
	}

	groupByFor := ratioThreshold.GroupByFor
	if groupByFor == nil {
		groupByFor = alerts.LOGSRATIOGROUPBYFOR_LOGS_RATIO_GROUP_BY_FOR_BOTH_OR_UNSPECIFIED.Ptr()
	}

	undetectedValuesManagement, diags := flattenUndetectedValuesManagement(ctx, ratioThreshold.UndetectedValuesManagement)
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsRatioThresholdAttr()), diags
	}

	logsRatioMoreThanModel := alerttypes.LogsRatioThresholdModel{
		Numerator:                  numeratorLogsFilter,
		NumeratorAlias:             types.StringPointerValue(ratioThreshold.NumeratorAlias),
		Denominator:                denominatorLogsFilter,
		DenominatorAlias:           types.StringPointerValue(ratioThreshold.DenominatorAlias),
		Rules:                      rules,
		NotificationPayloadFilter:  utils.StringSliceToTypeStringSet(ratioThreshold.GetNotificationPayloadFilter()),
		GroupByFor:                 types.StringValue(alerttypes.LogsRatioGroupByForProtoToSchemaMap[*groupByFor]),
		CustomEvaluationDelay:      types.Int32PointerValue(ratioThreshold.EvaluationDelayMs),
		IgnoreInfinity:             types.BoolPointerValue(ratioThreshold.IgnoreInfinity),
		UndetectedValuesManagement: undetectedValuesManagement,
	}
	return types.ObjectValueFrom(ctx, alertschema.LogsRatioThresholdAttr(), logsRatioMoreThanModel)
}

func flattenRatioThresholdRules(ctx context.Context, ratioThreshold *alerts.LogsRatioThresholdType) (types.Set, diag.Diagnostics) {
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

func flattenLogsRatioThresholdRuleCondition(ctx context.Context, condition *alerts.LogsRatioCondition) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(alertschema.LogsRatioThresholdRuleConditionAttr()), nil
	}
	conditionType := condition.ConditionType
	if conditionType == nil {
		conditionType = alerts.LOGSRATIOCONDITIONTYPE_LOGS_RATIO_CONDITION_TYPE_MORE_THAN_OR_UNSPECIFIED.Ptr()
	}

	return types.ObjectValueFrom(ctx, alertschema.LogsRatioThresholdRuleConditionAttr(), alerttypes.LogsRatioConditionModel{
		Threshold:     types.Float64PointerValue(condition.Threshold),
		TimeWindow:    flattenLogsRatioTimeWindow(condition.TimeWindow),
		ConditionType: types.StringValue(alerttypes.LogsRatioConditionMap[*conditionType]),
	},
	)
}

func flattenAlertOverride(ctx context.Context, override *alerts.AlertDefOverride) (types.Object, diag.Diagnostics) {
	if override == nil {
		return types.ObjectNull(alertschema.AlertOverrideAttr()), nil
	}
	priority := alerts.ALERTDEFPRIORITY_ALERT_DEF_PRIORITY_P5_OR_UNSPECIFIED
	if override.Priority != nil {
		priority = *override.Priority
	}
	return types.ObjectValueFrom(ctx, alertschema.AlertOverrideAttr(), alerttypes.AlertOverrideModel{
		Priority: types.StringValue(alerttypes.AlertPriorityProtoToSchemaMap[priority]),
	})
}

func flattenLogsUniqueCount(ctx context.Context, uniqueCount *alerts.LogsUniqueCountType) (types.Object, diag.Diagnostics) {
	if uniqueCount == nil {
		return types.ObjectNull(alertschema.LogsUniqueCountAttr()), nil
	}

	logsFilter, diags := flattenAlertsLogsFilter(ctx, uniqueCount.LogsFilter)
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsUniqueCountAttr()), diags
	}

	rules, diags := flattenLogsUniqueCountRules(ctx, uniqueCount)
	if diags.HasError() {
		return types.ObjectNull(alertschema.LogsUniqueCountAttr()), diags
	}

	var maxUniqueCountPerGroupByKey *int64
	if uniqueCount.MaxUniqueCountPerGroupByKey != nil {
		parsed, err := strconv.ParseInt(*uniqueCount.MaxUniqueCountPerGroupByKey, 10, 64)
		if err != nil {
			diags.AddError("Invalid Max Unique Count Per Group By Key", fmt.Sprintf("Could not parse Max Unique Count Per Group By Key value '%s' to int64: %s", *uniqueCount.MaxUniqueCountPerGroupByKey, err.Error()))
			return types.ObjectNull(alertschema.LogsUniqueCountAttr()), diags
		}
		maxUniqueCountPerGroupByKey = &parsed
	}
	logsUniqueCountModel := alerttypes.LogsUniqueCountModel{
		LogsFilter:                  logsFilter,
		Rules:                       rules,
		NotificationPayloadFilter:   utils.StringSliceToTypeStringSet(uniqueCount.NotificationPayloadFilter),
		MaxUniqueCountPerGroupByKey: types.Int64PointerValue(maxUniqueCountPerGroupByKey),
		UniqueCountKeypath:          types.StringPointerValue(uniqueCount.UniqueCountKeypath),
	}
	return types.ObjectValueFrom(ctx, alertschema.LogsUniqueCountAttr(), logsUniqueCountModel)
}

func flattenLogsUniqueCountRules(ctx context.Context, uniqueCount *alerts.LogsUniqueCountType) (types.Set, diag.Diagnostics) {
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

func flattenLogsUniqueCountRuleCondition(ctx context.Context, condition *alerts.LogsUniqueCountCondition) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(alertschema.LogsUniqueCountConditionAttr()), nil
	}

	if condition.MaxUniqueCount == nil {
		diags := diag.Diagnostics{}
		diags.AddError("Missing Max Unique Count", "The API response is missing the required max_unique_count field")
		return types.ObjectNull(alertschema.LogsUniqueCountConditionAttr()), diags
	}
	maxUniqueCount, err := strconv.ParseInt(*condition.MaxUniqueCount, 10, 64)
	if err != nil {
		diags := diag.Diagnostics{}
		diags.AddError("Invalid Max Unique Count", fmt.Sprintf("Could not parse Max Unique Count value '%s' to int64: %s", *condition.MaxUniqueCount, err.Error()))
		return types.ObjectNull(alertschema.LogsUniqueCountConditionAttr()), diags
	}
	return types.ObjectValueFrom(ctx, alertschema.LogsUniqueCountConditionAttr(), alerttypes.LogsUniqueCountConditionModel{
		MaxUniqueCount: types.Int64Value(maxUniqueCount),
		TimeWindow:     flattenLogsUniqueTimeWindow(condition.TimeWindow),
	})
}

func flattenLogsUniqueTimeWindow(timeWindow *alerts.LogsUniqueValueTimeWindow) types.String {
	if timeWindow == nil {
		return types.StringNull()
	}
	timeWindowValue := timeWindow.LogsUniqueValueTimeWindowSpecificValue
	if timeWindowValue == nil {
		timeWindowValue = alerts.LOGSUNIQUEVALUETIMEWINDOWVALUE_LOGS_UNIQUE_VALUE_TIME_WINDOW_VALUE_MINUTE_1_OR_UNSPECIFIED.Ptr()
	}
	return types.StringValue(alerttypes.LogsUniqueCountTimeWindowValueProtoToSchemaMap[*timeWindowValue])
}

func flattenLogsNewValue(ctx context.Context, newValue *alerts.LogsNewValueType) (types.Object, diag.Diagnostics) {
	if newValue == nil {
		return types.ObjectNull(alertschema.LogsNewValueAttr()), nil
	}

	logsFilter, diags := flattenAlertsLogsFilter(ctx, newValue.LogsFilter)
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
		NotificationPayloadFilter: utils.StringSliceToTypeStringSet(newValue.GetNotificationPayloadFilter()),
	}
	return types.ObjectValueFrom(ctx, alertschema.LogsNewValueAttr(), logsNewValueModel)
}

func flattenLogsNewValueCondition(ctx context.Context, condition *alerts.LogsNewValueCondition) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(alertschema.LogsNewValueConditionAttr()), nil
	}

	return types.ObjectValueFrom(ctx, alertschema.LogsNewValueConditionAttr(), alerttypes.NewValueConditionModel{
		TimeWindow:     flattenLogsNewValueTimeWindow(condition.TimeWindow),
		KeypathToTrack: types.StringPointerValue(condition.KeypathToTrack),
	})
}

func flattenAlertSchedule(ctx context.Context, alertProperties alerts.AlertDefProperties, currentSchedule *types.Object) (types.Object, diag.Diagnostics) {
	var alertScheduleModel alerttypes.AlertScheduleModel
	var diags diag.Diagnostics
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

	activeOn, diags := getActiveOn(alertProperties)
	if diags.HasError() {
		return types.ObjectNull(alertschema.AlertScheduleAttr()), diags
	}
	if activeOn == nil {
		return types.ObjectNull(alertschema.AlertScheduleAttr()), nil
	}
	alertScheduleModel.ActiveOn, diags = flattenActiveOn(ctx, *activeOn, utcOffset)
	if diags.HasError() {
		return types.ObjectNull(alertschema.AlertScheduleAttr()), diags
	}

	return types.ObjectValueFrom(ctx, alertschema.AlertScheduleAttr(), alertScheduleModel)
}

func getActiveOn(alertProperties alerts.AlertDefProperties) (*alerts.ActivitySchedule, diag.Diagnostics) {
	if alertProperties.Flow != nil {
		return alertProperties.ActiveOn, nil
	} else if alertProperties.LogsAnomaly != nil {
		return alertProperties.ActiveOn, nil
	} else if alertProperties.MetricAnomaly != nil {
		return alertProperties.ActiveOn, nil
	} else if alertProperties.LogsNewValue != nil {
		return alertProperties.ActiveOn, nil
	} else if alertProperties.LogsUniqueCount != nil {
		return alertProperties.ActiveOn, nil
	} else if alertProperties.LogsRatioThreshold != nil {
		return alertProperties.ActiveOn, nil
	} else if alertProperties.LogsTimeRelativeThreshold != nil {
		return alertProperties.ActiveOn, nil
	} else if alertProperties.MetricThreshold != nil {
		return alertProperties.ActiveOn, nil
	} else if alertProperties.TracingThreshold != nil {
		return alertProperties.ActiveOn, nil
	} else if alertProperties.LogsThreshold != nil {
		return alertProperties.ActiveOn, nil
	} else if alertProperties.TracingImmediate != nil {
		return alertProperties.ActiveOn, nil
	} else if alertProperties.LogsImmediate != nil {
		return alertProperties.ActiveOn, nil
	} else if alertProperties.SloThreshold != nil {
		return alertProperties.ActiveOn, nil
	}
	return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Unsupported Alert Type", "Received an unsupported alert type from the server.")}
}

func flattenActiveOn(ctx context.Context, activeOn alerts.ActivitySchedule, utcOffset string) (types.Object, diag.Diagnostics) {
	offset, err := time.Parse(OFFSET_FORMAT, utcOffset)
	if err != nil {
		return types.ObjectNull(alertschema.AlertScheduleActiveOnAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid UTC Offset", fmt.Sprintf("UTC Offset %v is not valid", utcOffset))}
	}
	zoneName, offsetSecs := offset.Zone() // Name is probably empty
	zone := time.FixedZone(zoneName, offsetSecs)
	startTimeUtc := time.Date(2021, 2, 1, int(*activeOn.StartTime.Hours), int(*activeOn.StartTime.Minutes), 0, 0, time.UTC)
	endTimeUtc := time.Date(2021, 2, 1, int(*activeOn.EndTime.Hours), int(*activeOn.EndTime.Minutes), 0, 0, time.UTC)
	startTime := startTimeUtc.In(zone)
	endTime := endTimeUtc.In(zone)

	daysOfWeekProto := activeOn.DayOfWeek
	if dayShift := dayDelta(startTimeUtc, startTime); dayShift != 0 {
		daysOfWeekProto = alerttypes.ShiftDaysOfWeek(daysOfWeekProto, dayShift)
	}
	daysOfWeek, diags := flattenDaysOfWeek(ctx, daysOfWeekProto)
	if diags.HasError() {
		return types.ObjectNull(alertschema.AlertScheduleActiveOnAttr()), diags
	}

	activeOnModel := alerttypes.ActiveOnModel{
		DaysOfWeek: daysOfWeek,
		StartTime:  types.StringValue(startTime.Format(TIME_FORMAT)),
		EndTime:    types.StringValue(endTime.Format(TIME_FORMAT)),
		UtcOffset:  types.StringValue(utcOffset),
	}
	return types.ObjectValueFrom(ctx, alertschema.AlertScheduleActiveOnAttr(), activeOnModel)
}

func flattenDaysOfWeek(ctx context.Context, daysOfWeek []alerts.DayOfWeek) (types.Set, diag.Diagnostics) {
	var daysOfWeekStrings []types.String
	for _, dow := range daysOfWeek {
		daysOfWeekStrings = append(daysOfWeekStrings, types.StringValue(alerttypes.DaysOfWeekProtoToSchemaMap[dow]))
	}
	return types.SetValueFrom(ctx, types.StringType, daysOfWeekStrings)
}

func flattenLogsTimeRelativeThreshold(ctx context.Context, logsTimeRelativeThreshold *alerts.LogsTimeRelativeThresholdType) (types.Object, diag.Diagnostics) {
	if logsTimeRelativeThreshold == nil {
		return types.ObjectNull(alertschema.LogsTimeRelativeAttr()), nil
	}

	logsFilter, diags := flattenAlertsLogsFilter(ctx, logsTimeRelativeThreshold.LogsFilter)
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
		NotificationPayloadFilter:  utils.StringSliceToTypeStringSet(logsTimeRelativeThreshold.GetNotificationPayloadFilter()),
		UndetectedValuesManagement: undetected,
		CustomEvaluationDelay:      types.Int32PointerValue(logsTimeRelativeThreshold.EvaluationDelayMs),
		IgnoreInfinity:             types.BoolPointerValue(logsTimeRelativeThreshold.IgnoreInfinity),
	}

	return types.ObjectValueFrom(ctx, alertschema.LogsTimeRelativeAttr(), logsTimeRelativeThresholdModel)
}

func flattenLogsTimeRelativeRuleCondition(ctx context.Context, condition *alerts.LogsTimeRelativeCondition) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(alertschema.LogsTimeRelativeConditionAttr()), nil
	}
	comparedTo := condition.ComparedTo
	if comparedTo == nil {
		comparedTo = alerts.LOGSTIMERELATIVECOMPAREDTO_LOGS_TIME_RELATIVE_COMPARED_TO_PREVIOUS_HOUR_OR_UNSPECIFIED.Ptr().Ptr()
	}

	conditionType := condition.ConditionType
	if conditionType == nil {
		conditionType = alerts.LOGSTIMERELATIVECONDITIONTYPE_LOGS_TIME_RELATIVE_CONDITION_TYPE_MORE_THAN_OR_UNSPECIFIED.Ptr()
	}

	return types.ObjectValueFrom(ctx, alertschema.LogsTimeRelativeConditionAttr(), alerttypes.LogsTimeRelativeConditionModel{
		Threshold:     types.Float64PointerValue(condition.Threshold),
		ComparedTo:    types.StringValue(alerttypes.LogsTimeRelativeComparedToProtoToSchemaMap[*comparedTo]),
		ConditionType: types.StringValue(alerttypes.LogsTimeRelativeConditionMap[*conditionType]),
	})
}

func flattenMetricThreshold(ctx context.Context, metricThreshold *alerts.MetricThresholdType) (types.Object, diag.Diagnostics) {
	if metricThreshold == nil {
		return types.ObjectNull(alertschema.MetricThresholdAttr()), nil
	}

	metricFilter, diags := flattenMetricFilter(ctx, metricThreshold.MetricFilter)
	if diags.HasError() {
		return types.ObjectNull(alertschema.MetricThresholdAttr()), diags
	}

	undetectedValuesManagement, diags := flattenUndetectedValuesManagement(ctx, metricThreshold.UndetectedValuesManagement)
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

	missingValues, diags := flattenMissingValuesManagement(ctx, metricThreshold.MissingValues)
	if diags.HasError() {
		return types.ObjectNull(alertschema.MetricThresholdAttr()), diags
	}
	noDataPolicy, diags := flattenNoDataPolicy(ctx, metricThreshold.NoDataPolicy)
	if diags.HasError() {
		return types.ObjectNull(alertschema.MetricThresholdAttr()), diags
	}

	metricThresholdModel := alerttypes.MetricThresholdModel{
		MetricFilter:               metricFilter,
		Rules:                      rules,
		MissingValues:              missingValues,
		UndetectedValuesManagement: undetectedValuesManagement,
		NoDataPolicy:               noDataPolicy,
		CustomEvaluationDelay:      types.Int32PointerValue(metricThreshold.EvaluationDelayMs),
	}
	return types.ObjectValueFrom(ctx, alertschema.MetricThresholdAttr(), metricThresholdModel)
}

func flattenMissingValuesManagement(ctx context.Context, missingValues *alerts.MetricMissingValues) (types.Object, diag.Diagnostics) {
	if missingValues == nil {
		return types.ObjectNull(alertschema.MissingValuesAttr()), nil
	}
	if replaceWithZero := missingValues.ReplaceWithZero; replaceWithZero != nil {
		return types.ObjectValueFrom(ctx, alertschema.MissingValuesAttr(), alerttypes.MissingValuesModel{
			ReplaceWithZero: types.BoolPointerValue(replaceWithZero),
		})
	} else if minNonNullValuesPct := missingValues.MinNonNullValuesPct; minNonNullValuesPct != nil {
		return types.ObjectValueFrom(ctx, alertschema.MissingValuesAttr(), alerttypes.MissingValuesModel{
			MinNonNullValuesPct: types.Int64PointerValue(minNonNullValuesPct),
		})
	} else {
		return types.ObjectNull(alertschema.MissingValuesAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Missing Values Management", "Missing Values Management strategy not supported")}
	}
}

func flattenMetricThresholdRuleCondition(ctx context.Context, condition *alerts.MetricThresholdCondition) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(alertschema.MetricThresholdConditionAttr()), nil
	}

	conditionType := condition.ConditionType
	if conditionType == nil {
		conditionType = alerts.METRICTHRESHOLDCONDITIONTYPE_METRIC_THRESHOLD_CONDITION_TYPE_MORE_THAN_OR_UNSPECIFIED.Ptr()
	}
	return types.ObjectValueFrom(ctx, alertschema.MetricThresholdConditionAttr(), alerttypes.MetricThresholdConditionModel{
		Threshold:     types.Float64PointerValue(condition.Threshold),
		ForOverPct:    types.Int64PointerValue(condition.ForOverPct),
		OfTheLast:     flattenMetricTimeWindow(condition.OfTheLast),
		ConditionType: types.StringValue(alerttypes.MetricsThresholdConditionMap[*conditionType]),
	})
}

func flattenMetricTimeWindow(timeWindow *alerts.MetricTimeWindow) types.String {
	if timeWindow == nil {
		return types.StringValue(alerttypes.MetricFilterOperationTypeProtoToSchemaMap[alerts.METRICTIMEWINDOWVALUE_METRIC_TIME_WINDOW_VALUE_MINUTES_1_OR_UNSPECIFIED])
	}
	if specificValue := timeWindow.MetricTimeWindowSpecificValue; specificValue != nil {
		metricTimeWindowSpecificValue := *specificValue
		if metricTimeWindowSpecificValue == "" {
			metricTimeWindowSpecificValue = alerts.METRICTIMEWINDOWVALUE_METRIC_TIME_WINDOW_VALUE_MINUTES_1_OR_UNSPECIFIED
		}
		return types.StringValue(alerttypes.MetricFilterOperationTypeProtoToSchemaMap[metricTimeWindowSpecificValue])
	} else if dynamicDuration := timeWindow.MetricTimeWindowDynamicDuration; dynamicDuration != nil {
		return types.StringPointerValue(dynamicDuration)
	} else {
		return types.StringValue(alerttypes.MetricFilterOperationTypeProtoToSchemaMap[alerts.METRICTIMEWINDOWVALUE_METRIC_TIME_WINDOW_VALUE_MINUTES_1_OR_UNSPECIFIED])
	}
}

func flattenMetricFilter(ctx context.Context, filter *alerts.MetricFilter) (types.Object, diag.Diagnostics) {
	if filter == nil {
		return types.ObjectNull(alertschema.MetricFilterAttr()), nil
	}
	if filter.Promql != nil {
		return types.ObjectValueFrom(ctx, alertschema.MetricFilterAttr(), alerttypes.MetricFilterModel{
			Promql: types.StringPointerValue(filter.Promql),
		})
	} else {
		return types.ObjectNull(alertschema.MetricFilterAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Metric Filter", "Metric Filter type is not supported")}
	}
}

func flattenTracingImmediate(ctx context.Context, tracingImmediate *alerts.TracingImmediateType) (types.Object, diag.Diagnostics) {
	if tracingImmediate == nil {
		return types.ObjectNull(alertschema.TracingImmediateAttr()), nil
	}

	var tracingQuery types.Object
	if simpleFilter := tracingImmediate.TracingFilter.SimpleFilter; simpleFilter != nil {
		filter, diag := flattenTracingSimpleFilter(ctx, simpleFilter)
		if diag.HasError() {
			return types.ObjectNull(alertschema.TracingImmediateAttr()), diag
		}
		tracingQuery, diag = types.ObjectValueFrom(ctx, alertschema.TracingQueryAttr(), filter)
		if diag.HasError() {
			return types.ObjectNull(alertschema.TracingImmediateAttr()), diag
		}
	} else {
		return types.ObjectNull(alertschema.TracingImmediateAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Tracing Query Filters", "Tracing Query Filters type is not supported")}
	}

	tracingImmediateModel := alerttypes.TracingImmediateModel{
		TracingFilter:             tracingQuery,
		NotificationPayloadFilter: utils.StringSliceToTypeStringSet(tracingImmediate.GetNotificationPayloadFilter()),
	}

	return types.ObjectValueFrom(ctx, alertschema.TracingImmediateAttr(), tracingImmediateModel)
}

// Also called query filters
func flattenTracingFilter(ctx context.Context, tracingFilter *alerts.TracingFilter) (types.Object, diag.Diagnostics) {
	if simpleFilter := tracingFilter.SimpleFilter; simpleFilter != nil {
		filter, diag := flattenTracingSimpleFilter(ctx, simpleFilter)
		if diag.HasError() {
			return types.ObjectNull(alertschema.TracingQueryAttr()), diag
		}
		tracingQuery, diag := types.ObjectValueFrom(ctx, alertschema.TracingQueryAttr(), filter)
		if diag.HasError() {
			return types.ObjectNull(alertschema.TracingQueryAttr()), diag
		}
		return tracingQuery, nil
	} else {
		return types.ObjectNull(alertschema.TracingQueryAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Tracing Query Filters", "Tracing Query Filter type is not supported")}
	}

}

func flattenTracingSimpleFilter(ctx context.Context, tracingQuery *alerts.TracingSimpleFilter) (types.Object, diag.Diagnostics) {
	if tracingQuery == nil {
		return types.ObjectNull(alertschema.TracingQueryAttr()), nil
	}

	labelFilters, diags := flattenTracingLabelFilters(ctx, tracingQuery.TracingLabelFilters)
	if diags.HasError() {
		return types.ObjectNull(alertschema.TracingQueryAttr()), diags
	}
	latencyRaw, err := strconv.ParseInt(*tracingQuery.LatencyThresholdMs, 10, 64)
	if err != nil {
		return types.ObjectNull(alertschema.TracingQueryAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Latency Threshold Ms", fmt.Sprintf("Could not parse Latency Threshold Ms value '%s' to int64: %s", *tracingQuery.LatencyThresholdMs, err.Error()))}
	}
	latencyThresholdMs := new(big.Float).SetInt64(latencyRaw)
	tracingQueryModel := &alerttypes.TracingFilterModel{
		LatencyThresholdMs:  types.NumberValue(latencyThresholdMs),
		TracingLabelFilters: labelFilters,
	}
	if diags.HasError() {
		return types.ObjectNull(alertschema.TracingQueryAttr()), diags
	}

	return types.ObjectValueFrom(ctx, alertschema.TracingQueryAttr(), tracingQueryModel)
}

func flattenTracingLabelFilters(ctx context.Context, filters *alerts.TracingLabelFilters) (types.Object, diag.Diagnostics) {
	if filters == nil {
		return types.ObjectNull(alertschema.TracingLabelFiltersAttr()), nil
	}

	applicationName, diags := flattenTracingFilterTypes(ctx, filters.ApplicationName)
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

	spanFields, diags := flattenTracingSpansFields(ctx, filters.SpanFields)
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

func flattenTracingFilterTypes(ctx context.Context, TracingFilterType []alerts.TracingFilterType) (types.Set, diag.Diagnostics) {
	var tracingFilterTypes []*alerttypes.TracingFilterTypeModel
	for _, tft := range TracingFilterType {
		tracingFilterTypes = append(tracingFilterTypes, flattenTracingFilterType(&tft))
	}
	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.TracingFiltersTypeAttr()}, tracingFilterTypes)
}

func flattenTracingFilterType(tracingFilterType *alerts.TracingFilterType) *alerttypes.TracingFilterTypeModel {
	if tracingFilterType == nil {
		return nil
	}

	tracingFilterTypeModel := &alerttypes.TracingFilterTypeModel{
		Values: utils.StringSliceToTypeStringSet(tracingFilterType.GetValues()),
	}
	if tracingFilterType.Operation != nil && *tracingFilterType.Operation != "" {
		tracingFilterTypeModel.Operation = types.StringValue(alerttypes.TracingFilterOperationProtoToSchemaMap[*tracingFilterType.Operation])
	} else {
		tracingFilterTypeModel.Operation = types.StringValue(alerttypes.TracingFilterOperationProtoToSchemaMap[alerts.TRACINGFILTEROPERATIONTYPE_TRACING_FILTER_OPERATION_TYPE_IS_OR_UNSPECIFIED])
	}
	return tracingFilterTypeModel
}

func flattenTracingSpansFields(ctx context.Context, spanFields []alerts.TracingSpanFieldsFilterType) (types.Set, diag.Diagnostics) {
	var tracingSpanFields []*alerttypes.TracingSpanFieldsFilterModel
	for _, field := range spanFields {
		tracingSpanField, diags := flattenTracingSpanField(ctx, &field)
		if diags.HasError() {
			return types.SetNull(types.ObjectType{AttrTypes: alertschema.TracingSpanFieldsFilterAttr()}), diags
		}
		tracingSpanFields = append(tracingSpanFields, tracingSpanField)
	}
	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.TracingSpanFieldsFilterAttr()}, tracingSpanFields)
}

func flattenTracingSpanField(ctx context.Context, spanField *alerts.TracingSpanFieldsFilterType) (*alerttypes.TracingSpanFieldsFilterModel, diag.Diagnostics) {
	if spanField == nil {
		return nil, nil
	}

	filterType, diags := types.ObjectValueFrom(ctx, alertschema.TracingFiltersTypeAttr(), flattenTracingFilterType(spanField.FilterType))
	if diags.HasError() {
		return nil, diags
	}

	return &alerttypes.TracingSpanFieldsFilterModel{
		Key:        types.StringPointerValue(spanField.Key),
		FilterType: filterType,
	}, nil
}

func flattenTracingThreshold(ctx context.Context, tracingThreshold *alerts.TracingThresholdType) (types.Object, diag.Diagnostics) {
	if tracingThreshold == nil {
		return types.ObjectNull(alertschema.TracingThresholdAttr()), nil
	}

	tracingQuery, diags := flattenTracingFilter(ctx, tracingThreshold.TracingFilter)
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
		NotificationPayloadFilter: utils.StringSliceToTypeStringSet(tracingThreshold.GetNotificationPayloadFilter()),
	}
	return types.ObjectValueFrom(ctx, alertschema.TracingThresholdAttr(), tracingThresholdModel)
}

func flattenTracingThresholdRules(ctx context.Context, tracingThreshold *alerts.TracingThresholdType, diags diag.Diagnostics) (basetypes.SetValue, diag.Diagnostics) {
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

func flattenTracingThresholdRuleCondition(ctx context.Context, condition *alerts.TracingThresholdCondition) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(alertschema.TracingThresholdConditionAttr()), nil
	}

	return types.ObjectValueFrom(ctx, alertschema.TracingThresholdConditionAttr(), alerttypes.TracingThresholdConditionModel{
		TimeWindow:    flattenTracingTimeWindow(condition.TimeWindow),
		SpanAmount:    types.Float64PointerValue(condition.SpanAmount),
		ConditionType: types.StringValue("MORE_THAN"),
	})
}

func flattenTracingTimeWindow(timeWindow *alerts.TracingTimeWindow) types.String {
	if timeWindow == nil {
		return types.StringNull()
	}
	timeWindowValue := timeWindow.TracingTimeWindowValue
	if timeWindowValue == nil {
		timeWindowValue = alerts.TRACINGTIMEWINDOWVALUE_TRACING_TIME_WINDOW_VALUE_MINUTES_5_OR_UNSPECIFIED.Ptr()
	}

	return types.StringValue(alerttypes.TracingTimeWindowProtoToSchemaMap[*timeWindowValue])
}

func flattenMetricAnomaly(ctx context.Context, anomaly *alerts.MetricAnomalyType) (types.Object, diag.Diagnostics) {
	if anomaly == nil {
		return types.ObjectNull(alertschema.MetricAnomalyAttr()), nil
	}

	metricFilter, diags := flattenMetricFilter(ctx, anomaly.MetricFilter)
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

	var percentageOfDeviation types.Float64
	if anomaly.AnomalyAlertSettings != nil && anomaly.AnomalyAlertSettings.PercentageOfDeviation != nil {
		percentageOfDeviation = types.Float64Value(float64(*anomaly.AnomalyAlertSettings.PercentageOfDeviation))
	} else {
		percentageOfDeviation = types.Float64Null()
	}

	anomalyModel := alerttypes.MetricAnomalyModel{
		MetricFilter:          metricFilter,
		Rules:                 rules,
		CustomEvaluationDelay: types.Int32PointerValue(anomaly.EvaluationDelayMs),
		PercentageOfDeviation: percentageOfDeviation,
	}
	return types.ObjectValueFrom(ctx, alertschema.MetricAnomalyAttr(), anomalyModel)
}

func flattenMetricAnomalyCondition(ctx context.Context, condition *alerts.MetricAnomalyCondition) (types.Object, diag.Diagnostics) {
	if condition == nil {
		return types.ObjectNull(alertschema.MetricAnomalyConditionAttr()), nil
	}

	conditionType := condition.ConditionType
	if conditionType == nil {
		conditionType = alerts.METRICANOMALYCONDITIONTYPE_METRIC_ANOMALY_CONDITION_TYPE_MORE_THAN_USUAL_OR_UNSPECIFIED.Ptr()
	}

	return types.ObjectValueFrom(ctx, alertschema.MetricAnomalyConditionAttr(), alerttypes.MetricAnomalyConditionModel{
		MinNonNullValuesPct: types.Int64PointerValue(condition.MinNonNullValuesPct),
		Threshold:           types.Float64PointerValue(condition.Threshold),
		ForOverPct:          types.Int64PointerValue(condition.ForOverPct),
		OfTheLast:           flattenMetricTimeWindow(condition.OfTheLast),
		ConditionType:       types.StringValue(alerttypes.MetricAnomalyConditionMap[*conditionType]),
	},
	)
}

func flattenFlow(ctx context.Context, flow *alerts.FlowType) (types.Object, diag.Diagnostics) {
	if flow == nil {
		return types.ObjectNull(alertschema.FlowAttr()), nil
	}

	stages, diags := flattenFlowStages(ctx, flow.Stages)
	if diags.HasError() {
		return types.ObjectNull(alertschema.FlowAttr()), diags
	}

	flowModel := alerttypes.FlowModel{
		Stages:             stages,
		EnforceSuppression: types.BoolPointerValue(flow.EnforceSuppression),
	}
	return types.ObjectValueFrom(ctx, alertschema.FlowAttr(), flowModel)
}

func flattenFlowStages(ctx context.Context, stages []alerts.FlowStages) (types.List, diag.Diagnostics) {
	var flowStages []*alerttypes.FlowStageModel
	for _, stage := range stages {
		flowStage, diags := flattenFlowStage(ctx, &stage)
		if diags.HasError() {
			return types.ListNull(types.ObjectType{AttrTypes: alertschema.FlowStageAttr()}), diags
		}
		flowStages = append(flowStages, flowStage)
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.FlowStageAttr()}, flowStages)

}

func flattenFlowStage(ctx context.Context, stage *alerts.FlowStages) (*alerttypes.FlowStageModel, diag.Diagnostics) {
	if stage == nil {
		return nil, nil
	}

	flowStagesGroups, diags := flattenFlowStagesGroups(ctx, stage)
	if diags.HasError() {
		return nil, diags
	}
	timeFrameMs, err := strconv.ParseInt(*stage.TimeframeMs, 10, 64)
	if err != nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Timeframe Ms", fmt.Sprintf("Could not parse Timeframe Ms value '%s' to int64: %s", *stage.TimeframeMs, err.Error()))}
	}
	timeFrameType := stage.TimeframeType
	if timeFrameType == nil {
		timeFrameType = alerts.TIMEFRAMETYPE_TIMEFRAME_TYPE_UNSPECIFIED.Ptr()
	}
	flowStageModel := &alerttypes.FlowStageModel{
		FlowStagesGroups: flowStagesGroups,
		TimeframeMs:      types.Int64PointerValue(&timeFrameMs),
		TimeframeType:    types.StringValue(alerttypes.FlowStageTimeFrameTypeProtoToSchemaMap[*timeFrameType]),
	}
	return flowStageModel, nil

}

func flattenFlowStagesGroups(ctx context.Context, stage *alerts.FlowStages) (types.List, diag.Diagnostics) {
	var flowStagesGroups []*alerttypes.FlowStagesGroupModel
	for _, group := range stage.GetFlowStagesGroups().Groups {
		flowStageGroup, diags := flattenFlowStageGroup(ctx, &group)
		if diags.HasError() {
			return types.ListNull(types.ObjectType{AttrTypes: alertschema.FlowStageGroupAttr()}), diags
		}
		flowStagesGroups = append(flowStagesGroups, flowStageGroup)
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.FlowStageGroupAttr()}, flowStagesGroups)

}

func flattenFlowStageGroup(ctx context.Context, group *alerts.FlowStagesGroup) (*alerttypes.FlowStagesGroupModel, diag.Diagnostics) {
	if group == nil {
		return nil, nil
	}

	alertDefs, diags := flattenAlertDefs(ctx, group.AlertDefs)
	if diags.HasError() {
		return nil, diags
	}

	flowStageGroupModel := &alerttypes.FlowStagesGroupModel{
		AlertDefs: alertDefs,
	}

	if group.NextOp != nil {
		flowStageGroupModel.NextOp = types.StringValue(alerttypes.FlowStagesGroupNextOpProtoToSchemaMap[*group.NextOp])
	} else {
		flowStageGroupModel.NextOp = types.StringValue(alerttypes.FlowStagesGroupNextOpProtoToSchemaMap[alerts.NEXTOP_NEXT_OP_AND_OR_UNSPECIFIED])
	}
	if group.AlertsOp != nil {
		flowStageGroupModel.AlertsOp = types.StringValue(alerttypes.FlowStagesGroupAlertsOpProtoToSchemaMap[*group.AlertsOp])
	} else {
		flowStageGroupModel.AlertsOp = types.StringValue(alerttypes.FlowStagesGroupAlertsOpProtoToSchemaMap[alerts.ALERTSOP_ALERTS_OP_AND_OR_UNSPECIFIED])
	}
	return flowStageGroupModel, nil
}

func flattenAlertDefs(ctx context.Context, defs []alerts.FlowStagesGroupsAlertDefs) (types.Set, diag.Diagnostics) {
	var alertDefs []*alerttypes.FlowStagesGroupsAlertDefsModel
	for _, def := range defs {
		alertDef := &alerttypes.FlowStagesGroupsAlertDefsModel{
			Id:  types.StringPointerValue(def.Id),
			Not: types.BoolPointerValue(def.Not),
		}
		alertDefs = append(alertDefs, alertDef)
	}
	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.AlertDefsAttr()}, alertDefs)
}

func flattenSloThreshold(ctx context.Context, slo *alerts.SloThresholdType) (types.Object, diag.Diagnostics) {
	if slo == nil {
		return types.ObjectNull(alertschema.SloThresholdAttr()), nil
	}

	sloDefinition := types.ObjectValueMust(alertschema.SloDefinitionAttr(), map[string]attr.Value{
		"slo_id": types.StringPointerValue(getSloId(slo)),
	})

	sloModel := alerttypes.SloThresholdModel{
		SloDefinition: sloDefinition,
		ErrorBudget:   types.ObjectNull(alertschema.SloErrorBudgetAttr()),
		BurnRate:      types.ObjectNull(alertschema.SloBurnRateAttr()),
	}

	if burnRate := slo.BurnRate; burnRate != nil {
		burnRate, diags := flattenSloBurnRate(ctx, burnRate)
		if diags.HasError() {
			return types.ObjectNull(alertschema.SloThresholdAttr()), diags
		}
		sloModel.BurnRate = burnRate

	} else if errorBudget := slo.ErrorBudget; errorBudget != nil {
		errBudget, diags := flattenSloErrorBudget(ctx, errorBudget)
		if diags.HasError() {
			return types.ObjectNull(alertschema.SloThresholdAttr()), diags
		}
		sloModel.ErrorBudget = errBudget
	}

	return types.ObjectValueFrom(ctx, alertschema.SloThresholdAttr(), sloModel)
}

func getSloId(slo *alerts.SloThresholdType) *string {
	if slo.SloDefinition != nil {
		return slo.SloDefinition.SloId
	}
	return nil
}

func flattenSloErrorBudget(ctx context.Context, errBudget *alerts.ErrorBudgetThreshold) (types.Object, diag.Diagnostics) {
	rules, diags := flattenSloThresholdRules(ctx, errBudget.Rules)
	if diags.HasError() {
		return types.ObjectNull(alertschema.SloErrorBudgetAttr()), diags
	}
	return types.ObjectValueFrom(ctx, alertschema.SloErrorBudgetAttr(), alerttypes.SloThresholdErrorBudgetModel{Rules: rules})
}

func flattenSloBurnRate(ctx context.Context, burnRate *alerts.BurnRateThreshold) (types.Object, diag.Diagnostics) {
	burnRateRules := getBurnRateThresholdRules(burnRate)
	rules, diags := flattenSloThresholdRules(ctx, burnRateRules)
	if diags.HasError() {
		return types.ObjectNull(alertschema.SloBurnRateAttr()), diags
	}

	burnRateModel := alerttypes.SloThresholdBurnRateModel{
		Rules:  rules,
		Dual:   types.ObjectNull(alertschema.SloDurationWrapperAttr()),
		Single: types.ObjectNull(alertschema.SloDurationWrapperAttr()),
	}

	if dual := burnRate.Dual; dual != nil {
		td, diags := flattenSloTimeDuration(ctx, dual.TimeDuration)
		if diags.HasError() {
			return types.ObjectNull(alertschema.SloBurnRateAttr()), diags
		}
		burnRateModel.Dual = td

	} else if single := burnRate.Single; single != nil {
		td, diags := flattenSloTimeDuration(ctx, single.TimeDuration)
		if diags.HasError() {
			return types.ObjectNull(alertschema.SloBurnRateAttr()), diags
		}
		burnRateModel.Single = td

	}

	return types.ObjectValueFrom(ctx, alertschema.SloBurnRateAttr(), burnRateModel)
}

func getBurnRateThresholdRules(slo *alerts.BurnRateThreshold) []alerts.SloThresholdRule {
	return slo.Rules
}

func flattenSloThresholdRules(ctx context.Context, rules []alerts.SloThresholdRule) (types.List, diag.Diagnostics) {
	var models []alerttypes.SloThresholdRuleModel
	for _, rule := range rules {
		override, diags := flattenAlertOverride(ctx, rule.Override)
		if diags.HasError() {
			return types.ListNull(types.ObjectType{AttrTypes: alertschema.SloThresholdRuleAttr()}), diags
		}
		ruleModel := alerttypes.SloThresholdRuleModel{
			Condition: types.ObjectValueMust(alertschema.SloThresholdConditionAttr(), map[string]attr.Value{
				"threshold": types.Float64Value(*rule.GetCondition().Threshold),
			}),
			Override: override,
		}
		models = append(models, ruleModel)
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: alertschema.SloThresholdRuleAttr()}, models)
}

func flattenSloTimeDuration(ctx context.Context, td *alerts.TimeDuration) (types.Object, diag.Diagnostics) {
	duration, err := strconv.ParseInt(*td.Duration, 10, 64)
	if err != nil {
		return types.ObjectNull(alertschema.SloDurationAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("Invalid Duration", fmt.Sprintf("Could not parse Duration value '%s' to int64: %s", *td.Duration, err.Error()))}
	}
	unit := td.Unit
	if unit == nil {
		unit = alerts.DURATIONUNIT_DURATION_UNIT_UNSPECIFIED.Ptr()
	}
	return types.ObjectValueFrom(ctx, alertschema.SloDurationWrapperAttr(), alerttypes.SloThresholdDurationWrapperModel{
		TimeDuration: types.ObjectValueMust(alertschema.SloDurationAttr(), map[string]attr.Value{
			"duration": types.Int64Value(duration),
			"unit":     types.StringValue(alerttypes.DurationUnitProtoToSchemaMap[*unit]),
		}),
	})
}
