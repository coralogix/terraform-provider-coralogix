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

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	alertapi "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/alert_definitions_service"
	alerttypes "github.com/coralogix/terraform-provider-coralogix/internal/provider/alerts/alert_types"
	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/alert"
)

// beforeFlattenAlert gives the values that the API leaves out the released form, and reads a case setting that the API leaves out, or returns as UNSPECIFIED, as the
// value that the schema defaults to: auto resolve is enabled, and an enrichment query is DataPrime.
func beforeFlattenAlert(_ context.Context, v *alertapi.AlertDef) diag.Diagnostics {
	if v == nil || v.AlertDefProperties == nil {
		return nil
	}
	fillReleasedReads(reflect.ValueOf(v.AlertDefProperties))
	if v.AlertDefProperties.CaseSettings == nil {
		return nil
	}
	settings := v.AlertDefProperties.CaseSettings
	if settings.AutoResolveMode == nil || *settings.AutoResolveMode == alertapi.ALERTDEFCASEAUTORESOLVEMODE_ALERT_DEF_CASE_AUTO_RESOLVE_MODE_UNSPECIFIED {
		enabled := alertapi.ALERTDEFCASEAUTORESOLVEMODE_ALERT_DEF_CASE_AUTO_RESOLVE_MODE_ENABLED
		settings.AutoResolveMode = &enabled
	}
	for i := range settings.EnrichmentQueries {
		query := &settings.EnrichmentQueries[i]
		if query.Type == nil || *query.Type == alertapi.ALERTDEFCASEENRICHMENTQUERYTYPE_ALERT_DEF_CASE_ENRICHMENT_QUERY_TYPE_UNSPECIFIED {
			dataprime := alertapi.ALERTDEFCASEENRICHMENTQUERYTYPE_ALERT_DEF_CASE_ENRICHMENT_QUERY_TYPE_DATAPRIME
			query.Type = &dataprime
		}
	}
	return nil
}

// scheduleAttrTypes is the type of the attributes of schedule.
func scheduleAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{"active_on": types.ObjectType{AttrTypes: activeOnAttrTypes()}}
}

func activeOnAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"days_of_week": types.SetType{ElemType: types.StringType},
		"end_time":     types.StringType,
		"start_time":   types.StringType,
		"utc_offset":   types.StringType,
	}
}

// afterFlattenAlert sets the schedule from the API window. The API holds it in UTC, so the state shows
// it in the offset that the model carries from the plan or the prior state, or +0000 when it has none.
func afterFlattenAlert(_ context.Context, m *alert.AlertDefModel, v *alertapi.AlertDef) diag.Diagnostics {
	m.GroupByKeys = emptyGroupByForm(m.GroupByKeys, v)
	// The schema rejects an empty labels map, so the one that the API returns for an alert without labels is null.
	if !m.EntityLabels.IsUnknown() && len(m.EntityLabels.Elements()) == 0 {
		m.EntityLabels = types.MapNull(types.StringType)
	}
	var window *alertapi.ActivitySchedule
	if v != nil && v.AlertDefProperties != nil {
		window = v.AlertDefProperties.ActiveOn
	}
	if window == nil {
		m.Schedule = types.ObjectNull(scheduleAttrTypes())
		return nil
	}
	offset := defaultScheduleOffset
	if prior, ok, _ := readActiveOn(m.Schedule); ok {
		offset = prior.utcOffset
	}
	schedule, diags := flattenActiveOn(window, offset)
	if diags.HasError() {
		return diags
	}
	m.Schedule = schedule
	return diags
}

// emptyGroupByForm gives an empty group_by the form that the plan has. A slo_threshold, tracing_threshold,
// and flow alert plan it as the empty list (the state value or the API value), any other alert as null.
func emptyGroupByForm(groupBy types.List, v *alertapi.AlertDef) types.List {
	if len(groupBy.Elements()) > 0 || groupBy.IsUnknown() {
		return groupBy
	}
	if v != nil && v.AlertDefProperties != nil {
		p := v.AlertDefProperties
		if p.SloThreshold != nil || p.TracingThreshold != nil || p.Flow != nil {
			return types.ListValueMust(types.StringType, []attr.Value{})
		}
	}
	return types.ListNull(types.StringType)
}

// flattenActiveOn converts the UTC window of the API to the offset of the schedule.
func flattenActiveOn(window *alertapi.ActivitySchedule, utcOffset string) (types.Object, diag.Diagnostics) {
	null := types.ObjectNull(scheduleAttrTypes())
	offsetTime, err := time.Parse(scheduleOffsetFormat, utcOffset)
	if err != nil {
		return null, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid UTC Offset", fmt.Sprintf("UTC Offset %v is not valid", utcOffset))}
	}
	zoneName, offsetSeconds := offsetTime.Zone()
	zone := time.FixedZone(zoneName, offsetSeconds)
	startUTC := timeOfDay(window.StartTime)
	endUTC := timeOfDay(window.EndTime)
	start, end := startUTC.In(zone), endUTC.In(zone)
	days := window.DayOfWeek
	if shift := dayDelta(startUTC, start); shift != 0 {
		days = alerttypes.ShiftDaysOfWeek(days, shift)
	}
	dayValues := make([]attr.Value, 0, len(days))
	for _, day := range days {
		dayValues = append(dayValues, types.StringValue(alerttypes.DaysOfWeekProtoToSchemaMap[day]))
	}
	daySet, diags := types.SetValue(types.StringType, dayValues)
	if diags.HasError() {
		return null, diags
	}
	activeOn, diags := types.ObjectValue(activeOnAttrTypes(), map[string]attr.Value{
		"days_of_week": daySet,
		"end_time":     types.StringValue(end.Format(scheduleTimeFormat)),
		"start_time":   types.StringValue(start.Format(scheduleTimeFormat)),
		"utc_offset":   types.StringValue(utcOffset),
	})
	if diags.HasError() {
		return null, diags
	}
	return types.ObjectValue(scheduleAttrTypes(), map[string]attr.Value{"active_on": activeOn})
}

// timeOfDay returns the UTC time of day in a day that the offset can shift to the next or the
// previous one.
func timeOfDay(t *alertapi.TimeOfDay) time.Time {
	var hours, minutes int
	if t != nil && t.Hours != nil {
		hours = int(*t.Hours)
	}
	if t != nil && t.Minutes != nil {
		minutes = int(*t.Minutes)
	}
	return time.Date(2021, 2, 1, hours, minutes, 0, 0, time.UTC)
}

var destinationsPath = path.Root("notification_group").AtName("destinations")

// afterReadAlert reads an empty description as null (keepDescriptionUnset) and keeps retriggering_period_minutes of a destination null where the plan or the prior
// state has none. The API gives a destination without a period the cadence of the incident and returns
// it: writing that value to the state would fail the apply and pin the inherited value on the next
// update. A state upgrade has no prior value, and a prior schema could not set the period, so every
// returned value is the inherited one.
func afterReadAlert(ctx context.Context, state *tfsdk.State, prior any) diag.Diagnostics {
	diags := keepDescriptionUnset(ctx, state, prior)
	if diags.HasError() {
		return diags
	}
	return append(diags, keepDestinationPeriodsUnset(ctx, state, prior)...)
}

var descriptionAttrPath = path.Root("description")

// keepDescriptionUnset reads the "" that the API returns for a cleared description as null, unless the
// plan or the prior state holds a configured "". The API returns no description for an alert that never
// had one, so an empty one only follows a Replace that cleared it. An import and a state upgrade have no
// prior value, so they read null.
func keepDescriptionUnset(ctx context.Context, state *tfsdk.State, prior any) diag.Diagnostics {
	var description types.String
	if diags := state.GetAttribute(ctx, descriptionAttrPath, &description); diags.HasError() || description.IsNull() || description.IsUnknown() || description.ValueString() != "" {
		return diags
	}
	if data, ok := prior.(priorData); ok && data != nil {
		var kept types.String
		if diags := data.GetAttribute(ctx, descriptionAttrPath, &kept); diags.HasError() {
			return diags
		}
		if !kept.IsNull() && !kept.IsUnknown() && kept.ValueString() == "" {
			return nil
		}
	}
	return state.SetAttribute(ctx, descriptionAttrPath, types.StringNull())
}

// keepDestinationPeriodsUnset clears retriggering_period_minutes where the plan or prior state has none.
func keepDestinationPeriodsUnset(ctx context.Context, state *tfsdk.State, prior any) diag.Diagnostics {
	var destinations types.List
	if diags := state.GetAttribute(ctx, destinationsPath, &destinations); diags.HasError() || destinations.IsNull() || destinations.IsUnknown() {
		return diags
	}
	keep := func(int) bool { return true }
	if data, ok := prior.(priorData); ok && data != nil {
		var kept types.List
		if diags := data.GetAttribute(ctx, destinationsPath, &kept); diags.HasError() || kept.IsNull() || kept.IsUnknown() {
			return nil
		}
		keep = func(i int) bool { return i < len(kept.Elements()) && periodIsUnset(kept.Elements()[i]) }
	}
	cleared, diags := clearDestinationPeriods(destinations, keep)
	if diags.HasError() {
		return diags
	}
	return state.SetAttribute(ctx, destinationsPath, cleared)
}

// periodIsUnset reports whether a destination has no retriggering_period_minutes value.
func periodIsUnset(destination attr.Value) bool {
	object, ok := destination.(types.Object)
	if !ok || object.IsNull() || object.IsUnknown() {
		return false
	}
	period, found := object.Attributes()["retriggering_period_minutes"]
	return !found || period.IsNull() || period.IsUnknown()
}

// clearDestinationPeriods sets retriggering_period_minutes to null on each destination that clear
// selects by its index.
func clearDestinationPeriods(destinations types.List, clear func(int) bool) (types.List, diag.Diagnostics) {
	elements := destinations.Elements()
	out := make([]attr.Value, len(elements))
	for i, element := range elements {
		object, ok := element.(types.Object)
		if !ok || object.IsNull() || object.IsUnknown() || !clear(i) {
			out[i] = element
			continue
		}
		attrs := object.Attributes()
		if _, found := attrs["retriggering_period_minutes"]; !found {
			out[i] = element
			continue
		}
		attrs["retriggering_period_minutes"] = types.Int64Null()
		cleared, diags := types.ObjectValue(object.AttributeTypes(context.Background()), attrs)
		if diags.HasError() {
			return destinations, diags
		}
		out[i] = cleared
	}
	return types.ListValue(destinations.ElementType(context.Background()), out)
}
