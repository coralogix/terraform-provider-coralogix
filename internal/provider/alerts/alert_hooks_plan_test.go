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
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/alert"
)

// The API returns neither notify_on nor retriggering_period for a webhook without Advanced Notification, so
// the state holds null for both. A plan that leaves both out must keep that, not make them unknown.
func TestWithStateAdvancedNotification(t *testing.T) {
	ctx := context.Background()
	webhookType := attributeType(alert.Schema().Type().TerraformType(ctx), "notification_group", "webhooks_settings").(tftypes.Set).ElementType
	periodType := attributeType(webhookType, "retriggering_period")
	period := func(minutes int64) tftypes.Value {
		return tftypes.NewValue(periodType, map[string]tftypes.Value{"minutes": tftypes.NewValue(tftypes.Number, minutes)})
	}
	unknownString := tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	unknownPeriod := tftypes.NewValue(periodType, tftypes.UnknownValue)
	webhook := func(integration string, notify, retriggering tftypes.Value) tftypes.Value {
		return nullObject(webhookType, map[string]tftypes.Value{
			"integration_id": stringValue(integration), "notify_on": notify, "retriggering_period": retriggering,
		})
	}
	nullString := tftypes.NewValue(tftypes.String, nil)
	nullPeriod := tftypes.NewValue(periodType, nil)
	cases := []struct {
		name    string
		config  tftypes.Value
		planned tftypes.Value
		state   []tftypes.Value
		want    tftypes.Value
	}{
		{"both omitted, state has neither: null is kept", webhook("1", nullString, nullPeriod),
			webhook("1", unknownString, unknownPeriod), []tftypes.Value{webhook("1", nullString, nullPeriod)}, webhook("1", nullString, nullPeriod)},
		{"both omitted, state has both: both are kept", webhook("1", nullString, nullPeriod),
			webhook("1", unknownString, unknownPeriod), []tftypes.Value{webhook("1", stringValue("Triggered Only"), period(10))}, webhook("1", stringValue("Triggered Only"), period(10))},
		{"both omitted, state has only the period: each is kept as it is", webhook("1", nullString, nullPeriod),
			webhook("1", unknownString, unknownPeriod), []tftypes.Value{webhook("1", nullString, period(10))}, webhook("1", nullString, period(10))},
		{"notify_on set, period omitted, state has neither: the period stays unknown", webhook("1", stringValue("Triggered Only"), nullPeriod),
			webhook("1", stringValue("Triggered Only"), unknownPeriod), []tftypes.Value{webhook("1", nullString, nullPeriod)}, webhook("1", stringValue("Triggered Only"), unknownPeriod)},
		{"period set, notify_on omitted, state has neither: notify_on stays unknown", webhook("1", nullString, period(5)),
			webhook("1", unknownString, period(5)), []tftypes.Value{webhook("1", nullString, nullPeriod)}, webhook("1", unknownString, period(5))},
		{"period set, notify_on omitted, state has notify_on: it is kept", webhook("1", nullString, period(5)),
			webhook("1", unknownString, period(5)), []tftypes.Value{webhook("1", stringValue("Triggered Only"), period(10))}, webhook("1", stringValue("Triggered Only"), period(5))},
		{"both set: nothing changes", webhook("1", stringValue("Triggered Only"), period(5)),
			webhook("1", stringValue("Triggered Only"), period(5)), []tftypes.Value{webhook("1", nullString, nullPeriod)}, webhook("1", stringValue("Triggered Only"), period(5))},
		{"no state webhook matches: both stay unknown", webhook("2", nullString, nullPeriod),
			webhook("2", unknownString, unknownPeriod), []tftypes.Value{webhook("1", nullString, nullPeriod)}, webhook("2", unknownString, unknownPeriod)},
		{"the matching state webhook is picked, not the first one", webhook("2", nullString, nullPeriod),
			webhook("2", unknownString, unknownPeriod),
			[]tftypes.Value{webhook("1", stringValue("Triggered Only"), period(10)), webhook("2", nullString, nullPeriod)},
			webhook("2", nullString, nullPeriod)},
		{"recipients match", nullObject(webhookType, map[string]tftypes.Value{"recipients": tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{stringValue("a@b.c")})}),
			nullObject(webhookType, map[string]tftypes.Value{"recipients": tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{stringValue("a@b.c")}), "notify_on": unknownString, "retriggering_period": unknownPeriod}),
			[]tftypes.Value{nullObject(webhookType, map[string]tftypes.Value{"recipients": tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{stringValue("a@b.c")})})},
			nullObject(webhookType, map[string]tftypes.Value{"recipients": tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{stringValue("a@b.c")})})},
		{"notify_on is an expression unknown until apply, period omitted, state has neither: both stay unknown",
			webhook("1", unknownString, nullPeriod),
			webhook("1", unknownString, unknownPeriod), []tftypes.Value{webhook("1", nullString, nullPeriod)}, webhook("1", unknownString, unknownPeriod)},
		{"notify_on is an expression unknown until apply, state has both: notify_on stays unknown, the omitted period is kept",
			webhook("1", unknownString, nullPeriod),
			webhook("1", unknownString, unknownPeriod), []tftypes.Value{webhook("1", stringValue("Triggered Only"), period(10))}, webhook("1", unknownString, period(10))},
		{"no configuration webhook matches: both stay unknown",
			webhook("2", nullString, nullPeriod),
			webhook("1", unknownString, unknownPeriod), []tftypes.Value{webhook("1", nullString, nullPeriod)}, webhook("1", unknownString, unknownPeriod)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := withStateAdvancedNotification(tc.planned, tc.state, []tftypes.Value{tc.config}); !got.Equal(tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// The API infers the group_by of an analytics_threshold alert from its DataPrime query, and derives
// use_rows_as_permutations from the keys, so both are planned unknown when the query changes and kept
// otherwise. A configured value is planned as configured.
func TestPlanAnalyticsThresholdGroupBy(t *testing.T) {
	ctx := context.Background()
	s := alert.Schema()
	rootType := s.Type().TerraformType(ctx)
	armType := attributeType(rootType, "type_definition", "analytics_threshold")
	queryType := attributeType(armType, "dataprime_query")
	listType := tftypes.List{ElementType: tftypes.String}
	unknownList := tftypes.NewValue(listType, tftypes.UnknownValue)
	nullList := tftypes.NewValue(listType, nil)
	keys := tftypes.NewValue(listType, []tftypes.Value{stringValue("app")})
	unknownBool := tftypes.NewValue(tftypes.Bool, tftypes.UnknownValue)
	nullBool := tftypes.NewValue(tftypes.Bool, nil)
	boolValue := func(b bool) tftypes.Value { return tftypes.NewValue(tftypes.Bool, b) }
	alertOf := func(name, query string, rows, groupBy tftypes.Value) tftypes.Value {
		arm := nullObject(armType, map[string]tftypes.Value{
			"dataprime_query":          nullObject(queryType, map[string]tftypes.Value{"query": stringValue(query)}),
			"use_rows_as_permutations": rows,
		})
		td := nullObject(attributeType(rootType, "type_definition"), map[string]tftypes.Value{"analytics_threshold": arm})
		return nullObject(rootType, map[string]tftypes.Value{"name": stringValue(name), "type_definition": td, "group_by": groupBy})
	}
	const grouped = "source logs | groupby $l.applicationname as app aggregate count() as cnt"
	const other = "source logs | groupby $l.subsystemname as sub aggregate count() as cnt"
	state := alertOf("a", grouped, boolValue(true), keys)
	cases := []struct {
		name          string
		config, state tftypes.Value
		// plan is the plan of the framework: an unconfigured computed attribute of a changed alert is unknown.
		plan                tftypes.Value
		wantRows, wantGroup tftypes.Value
	}{
		{
			name:   "new alert",
			config: alertOf("a", grouped, nullBool, nullList), state: tftypes.NewValue(rootType, nil),
			plan:     alertOf("a", grouped, unknownBool, unknownList),
			wantRows: unknownBool, wantGroup: unknownList,
		},
		{
			name:   "changed query",
			config: alertOf("a", other, nullBool, nullList), state: state,
			plan:     alertOf("a", other, unknownBool, unknownList),
			wantRows: unknownBool, wantGroup: unknownList,
		},
		{
			name:   "unchanged query keeps the state",
			config: alertOf("b", grouped, nullBool, nullList), state: state,
			plan:     alertOf("b", grouped, unknownBool, unknownList),
			wantRows: boolValue(true), wantGroup: keys,
		},
		{
			name:   "configured group_by",
			config: alertOf("a", other, nullBool, keys), state: state,
			plan:     alertOf("a", other, unknownBool, keys),
			wantRows: unknownBool, wantGroup: keys,
		},
		{
			name:   "configured use_rows_as_permutations that changes",
			config: alertOf("a", grouped, boolValue(false), nullList), state: state,
			plan:     alertOf("a", grouped, boolValue(false), unknownList),
			wantRows: boolValue(false), wantGroup: unknownList,
		},
		{
			name:   "configured use_rows_as_permutations as in the state",
			config: alertOf("b", grouped, boolValue(true), nullList), state: state,
			plan:     alertOf("b", grouped, boolValue(true), unknownList),
			wantRows: boolValue(true), wantGroup: keys,
		},
		{
			name:   "an update of an alert saved before the API derived use_rows_as_permutations",
			config: alertOf("b", grouped, nullBool, nullList), state: alertOf("a", grouped, nullBool, nullList),
			plan:     alertOf("b", grouped, unknownBool, unknownList),
			wantRows: unknownBool, wantGroup: unknownList,
		},
		{
			name:   "no change of an alert saved before the API derived use_rows_as_permutations",
			config: alertOf("a", grouped, nullBool, nullList), state: alertOf("a", grouped, nullBool, nullList),
			plan:     alertOf("a", grouped, nullBool, nullList),
			wantRows: nullBool, wantGroup: nullList,
		},
		{
			// The framework plans unconfigured computed values unknown also for a difference that a plan
			// step undoes before the hook, so an alert without a change can arrive with both unknown.
			name:   "no change of an alert without use_rows_as_permutations that the framework planned unknown",
			config: alertOf("a", grouped, nullBool, nullList), state: alertOf("a", grouped, nullBool, nullList),
			plan:     alertOf("a", grouped, unknownBool, unknownList),
			wantRows: nullBool, wantGroup: nullList,
		},
		{
			name:   "a changed group_by of an alert without use_rows_as_permutations",
			config: alertOf("a", grouped, nullBool, keys), state: alertOf("a", grouped, nullBool, nullList),
			plan:     alertOf("a", grouped, unknownBool, keys),
			wantRows: unknownBool, wantGroup: keys,
		},
		{
			name:   "use_rows_as_permutations from a variable",
			config: alertOf("b", grouped, unknownBool, nullList), state: state,
			plan:     alertOf("b", grouped, unknownBool, unknownList),
			wantRows: unknownBool, wantGroup: unknownList,
		},
	}
	rowsPath := tftypes.NewAttributePath().WithAttributeName("type_definition").WithAttributeName("analytics_threshold").WithAttributeName("use_rows_as_permutations")
	groupByPath := tftypes.NewAttributePath().WithAttributeName("group_by")
	for _, tc := range cases {
		req := resource.ModifyPlanRequest{
			Config: tfsdk.Config{Schema: s, Raw: tc.config},
			State:  tfsdk.State{Schema: s, Raw: tc.state},
			Plan:   tfsdk.Plan{Schema: s, Raw: tc.plan},
		}
		resp := &resource.ModifyPlanResponse{Plan: tfsdk.Plan{Schema: s, Raw: tc.plan}}
		modifyAlertPlan(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("%s: %v", tc.name, resp.Diagnostics)
		}
		for _, check := range []struct {
			p    *tftypes.AttributePath
			want tftypes.Value
		}{{rowsPath, tc.wantRows}, {groupByPath, tc.wantGroup}} {
			got, _, err := tftypes.WalkAttributePath(resp.Plan.Raw, check.p)
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if !got.(tftypes.Value).Equal(check.want) {
				t.Errorf("%s: planned %s = %s, want %s", tc.name, check.p, got, check.want)
			}
		}
	}
}
