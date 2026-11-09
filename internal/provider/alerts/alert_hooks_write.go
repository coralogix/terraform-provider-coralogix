// Copyright 2026 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
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
	"reflect"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	alertapi "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/alert_definitions_service"
	alerttypes "github.com/coralogix/terraform-provider-coralogix/internal/provider/alerts/alert_types"
)

const (
	// scheduleTimeFormat is the format of the start and the end time of the schedule.
	scheduleTimeFormat = "15:04"
	// scheduleOffsetFormat is the format of the UTC offset of the schedule.
	scheduleOffsetFormat = "Z0700"
	// defaultScheduleOffset is the offset of a schedule that sets none.
	defaultScheduleOffset = "+0000"
)

// beforeWriteAlert adds what the generated request does not hold: the schedule, which the API
// stores in UTC, and the access policy.
func beforeWriteAlert(ctx context.Context, config tfsdk.Config, body any) diag.Diagnostics {
	switch b := body.(type) {
	case *alertapi.CreateAlertDefinitionRequest:
		return beforeCreateAlert(ctx, config, b)
	case *alertapi.ReplaceAlertDefinitionRequest:
		return beforeUpdateAlert(ctx, config, b)
	default:
		return diag.Diagnostics{diag.NewErrorDiagnostic("Unexpected request body", fmt.Sprintf("The alert request body is %T.", body))}
	}
}

func beforeCreateAlert(ctx context.Context, config tfsdk.Config, body *alertapi.CreateAlertDefinitionRequest) diag.Diagnostics {
	diags := setActiveOn(ctx, config, body.AlertDefProperties)
	if diags.HasError() {
		return diags
	}
	policy, policyDiags := configuredAccessPolicy(ctx, config)
	diags.Append(policyDiags...)
	body.AccessPolicy = accessPolicyForCreate(policy)
	return diags
}

func beforeUpdateAlert(ctx context.Context, config tfsdk.Config, body *alertapi.ReplaceAlertDefinitionRequest) diag.Diagnostics {
	diags := setActiveOn(ctx, config, body.AlertDefProperties)
	if diags.HasError() {
		return diags
	}
	policy, policyDiags := configuredAccessPolicy(ctx, config)
	diags.Append(policyDiags...)
	body.AccessPolicy = accessPolicyActionForUpdate(policy)
	diags.Append(clearOmittedDescription(ctx, config, body.AlertDefProperties)...)
	return diags
}

var descriptionPath = path.Root("description")

// clearOmittedDescription sends description "" on Replace when the configuration leaves it out. The
// API keeps a stored description that a Replace omits and clears it only for an explicit "", so
// without this a removed description would stay on the alert. Create omits it, because a new alert has
// none to clear; a configured value, "" too, is already in the request.
func clearOmittedDescription(ctx context.Context, config tfsdk.Config, properties *alertapi.AlertDefProperties) diag.Diagnostics {
	if properties == nil {
		return nil
	}
	var description types.String
	diags := config.GetAttribute(ctx, descriptionPath, &description)
	if diags.HasError() || !description.IsNull() {
		return diags
	}
	empty := ""
	properties.Description = &empty
	return diags
}

// setActiveOn sends the configured schedule, and drops what the released request did not hold. A configuration
// without a schedule sends none.
func setActiveOn(ctx context.Context, config tfsdk.Config, properties *alertapi.AlertDefProperties) diag.Diagnostics {
	if properties == nil {
		return nil
	}
	var schedule types.Object
	diags := config.GetAttribute(ctx, path.Root("schedule"), &schedule)
	if diags.HasError() {
		return diags
	}
	activeOn, expandDiags := expandActiveOn(schedule)
	diags.Append(expandDiags...)
	if diags.HasError() {
		return diags
	}
	properties.ActiveOn = activeOn
	dropEmptyNoDataPolicies(reflect.ValueOf(properties))
	return diags
}

// configuredAccessPolicy returns the access_policy of the configuration.
func configuredAccessPolicy(ctx context.Context, config tfsdk.Config) (types.String, diag.Diagnostics) {
	var policy types.String
	diags := config.GetAttribute(ctx, path.Root("access_policy"), &policy)
	return policy, diags
}

// accessPolicyForCreate maps the configured access_policy to the plain-string field of the Create
// request: an absent or empty policy is omitted, so the alert is created without one.
func accessPolicyForCreate(policy types.String) *string {
	if policy.IsNull() || policy.IsUnknown() || policy.ValueString() == "" {
		return nil
	}
	return policy.ValueStringPointer()
}

// accessPolicyActionForUpdate maps the configured access_policy to the access policy action of the
// Replace request: a configured value upserts, an explicit "" deletes the remote policy, and an
// attribute that the configuration leaves out sends no action and leaves the remote policy unchanged.
func accessPolicyActionForUpdate(policy types.String) *alertapi.AccessPolicyAction {
	if policy.IsNull() || policy.IsUnknown() {
		return nil
	}
	if policy.ValueString() == "" {
		return &alertapi.AccessPolicyAction{Delete: map[string]interface{}{}}
	}
	return &alertapi.AccessPolicyAction{Upsert: policy.ValueStringPointer()}
}

// dayDelta is the number of calendar days from one time to the other.
func dayDelta(from, to time.Time) int {
	fromDay := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	toDay := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC)
	return int(toDay.Sub(fromDay).Hours() / 24)
}

// scheduleActiveOn is the active_on object of the schedule.
type scheduleActiveOn struct {
	daysOfWeek []string
	startTime  string
	endTime    string
	utcOffset  string
}

// readActiveOn returns the active_on object of a schedule, and false when the schedule is null,
// unknown, or has no active_on.
func readActiveOn(schedule types.Object) (scheduleActiveOn, bool, diag.Diagnostics) {
	var diags diag.Diagnostics
	if schedule.IsNull() || schedule.IsUnknown() {
		return scheduleActiveOn{}, false, nil
	}
	activeOn, ok := schedule.Attributes()["active_on"].(types.Object)
	if !ok || activeOn.IsNull() || activeOn.IsUnknown() {
		return scheduleActiveOn{}, false, nil
	}
	attrs := activeOn.Attributes()
	out := scheduleActiveOn{utcOffset: defaultScheduleOffset}
	out.startTime = stringAttr(attrs["start_time"])
	out.endTime = stringAttr(attrs["end_time"])
	if offset := stringAttr(attrs["utc_offset"]); offset != "" {
		out.utcOffset = offset
	}
	if days, ok := attrs["days_of_week"].(types.Set); ok {
		for _, element := range days.Elements() {
			out.daysOfWeek = append(out.daysOfWeek, stringAttr(element))
		}
	}
	return out, true, diags
}

func stringAttr(v any) string {
	if s, ok := v.(types.String); ok && !s.IsNull() && !s.IsUnknown() {
		return s.ValueString()
	}
	return ""
}

// expandActiveOn converts the schedule of the configuration to the UTC window of the API. The
// start and the end time are in the configured offset, so the days of the week move with the day
// that the start time falls on in UTC.
func expandActiveOn(schedule types.Object) (*alertapi.ActivitySchedule, diag.Diagnostics) {
	activeOn, ok, diags := readActiveOn(schedule)
	if !ok || diags.HasError() {
		return nil, diags
	}
	offsetTime, err := time.Parse(scheduleOffsetFormat, activeOn.utcOffset)
	if err != nil {
		diags.AddError("Failed to parse start time", err.Error())
		return nil, diags
	}
	_, offset := offsetTime.Zone()
	location := time.FixedZone("", offset)
	startTime, err := time.ParseInLocation(scheduleTimeFormat, activeOn.startTime, location)
	if err != nil {
		diags.AddError("Failed to parse start time", err.Error())
	}
	endTime, err := time.ParseInLocation(scheduleTimeFormat, activeOn.endTime, location)
	if err != nil {
		diags.AddError("Failed to parse end time", err.Error())
	}
	if diags.HasError() {
		return nil, diags
	}
	days := make([]alertapi.DayOfWeek, 0, len(activeOn.daysOfWeek))
	for _, day := range activeOn.daysOfWeek {
		days = append(days, alerttypes.DaysOfWeekSchemaToProtoMap[day])
	}
	startUTC, endUTC := startTime.UTC(), endTime.UTC()
	if shift := dayDelta(startTime, startUTC); shift != 0 {
		days = alerttypes.ShiftDaysOfWeek(days, shift)
	}
	startHour, startMinute := int32(startUTC.Hour()), int32(startUTC.Minute())
	endHour, endMinute := int32(endUTC.Hour()), int32(endUTC.Minute())
	return &alertapi.ActivitySchedule{
		DayOfWeek: days,
		StartTime: &alertapi.TimeOfDay{Hours: &startHour, Minutes: &startMinute},
		EndTime:   &alertapi.TimeOfDay{Hours: &endHour, Minutes: &endMinute},
	}, diags
}
