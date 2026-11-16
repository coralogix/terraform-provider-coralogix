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
		planned tftypes.Value
		state   []tftypes.Value
		want    tftypes.Value
	}{
		{"both omitted, state has neither: null is kept",
			webhook("1", unknownString, unknownPeriod), []tftypes.Value{webhook("1", nullString, nullPeriod)}, webhook("1", nullString, nullPeriod)},
		{"both omitted, state has both: both are kept",
			webhook("1", unknownString, unknownPeriod), []tftypes.Value{webhook("1", stringValue("Triggered Only"), period(10))}, webhook("1", stringValue("Triggered Only"), period(10))},
		{"both omitted, state has only the period: each is kept as it is",
			webhook("1", unknownString, unknownPeriod), []tftypes.Value{webhook("1", nullString, period(10))}, webhook("1", nullString, period(10))},
		{"notify_on set, period omitted, state has neither: the period stays unknown",
			webhook("1", stringValue("Triggered Only"), unknownPeriod), []tftypes.Value{webhook("1", nullString, nullPeriod)}, webhook("1", stringValue("Triggered Only"), unknownPeriod)},
		{"period set, notify_on omitted, state has neither: notify_on stays unknown",
			webhook("1", unknownString, period(5)), []tftypes.Value{webhook("1", nullString, nullPeriod)}, webhook("1", unknownString, period(5))},
		{"period set, notify_on omitted, state has notify_on: it is kept",
			webhook("1", unknownString, period(5)), []tftypes.Value{webhook("1", stringValue("Triggered Only"), period(10))}, webhook("1", stringValue("Triggered Only"), period(5))},
		{"both set: nothing changes",
			webhook("1", stringValue("Triggered Only"), period(5)), []tftypes.Value{webhook("1", nullString, nullPeriod)}, webhook("1", stringValue("Triggered Only"), period(5))},
		{"no state webhook matches: both stay unknown",
			webhook("2", unknownString, unknownPeriod), []tftypes.Value{webhook("1", nullString, nullPeriod)}, webhook("2", unknownString, unknownPeriod)},
		{"the matching state webhook is picked, not the first one",
			webhook("2", unknownString, unknownPeriod),
			[]tftypes.Value{webhook("1", stringValue("Triggered Only"), period(10)), webhook("2", nullString, nullPeriod)},
			webhook("2", nullString, nullPeriod)},
		{"recipients match",
			nullObject(webhookType, map[string]tftypes.Value{"recipients": tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{stringValue("a@b.c")}), "notify_on": unknownString, "retriggering_period": unknownPeriod}),
			[]tftypes.Value{nullObject(webhookType, map[string]tftypes.Value{"recipients": tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{stringValue("a@b.c")})})},
			nullObject(webhookType, map[string]tftypes.Value{"recipients": tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{stringValue("a@b.c")})})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := withStateAdvancedNotification(tc.planned, tc.state); !got.Equal(tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
