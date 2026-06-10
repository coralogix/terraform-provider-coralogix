// Copyright 2025 Coralogix Ltd.
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

package alerttypes

import (
	"testing"

	alerts "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/alert_definitions_service"
)

func TestShiftDaysOfWeek(t *testing.T) {
	mon := alerts.DAYOFWEEK_DAY_OF_WEEK_MONDAY_OR_UNSPECIFIED
	tue := alerts.DAYOFWEEK_DAY_OF_WEEK_TUESDAY
	wed := alerts.DAYOFWEEK_DAY_OF_WEEK_WEDNESDAY
	fri := alerts.DAYOFWEEK_DAY_OF_WEEK_FRIDAY
	sat := alerts.DAYOFWEEK_DAY_OF_WEEK_SATURDAY
	sun := alerts.DAYOFWEEK_DAY_OF_WEEK_SUNDAY

	cases := []struct {
		name  string
		in    []alerts.DayOfWeek
		shift int
		want  []alerts.DayOfWeek
	}{
		{"no shift returns input unchanged", []alerts.DayOfWeek{mon, fri}, 0, []alerts.DayOfWeek{mon, fri}},
		{"shift +1 Mon→Tue", []alerts.DayOfWeek{mon}, 1, []alerts.DayOfWeek{tue}},
		{"shift +1 Sun→Mon (wrap)", []alerts.DayOfWeek{sun}, 1, []alerts.DayOfWeek{mon}},
		{"shift -1 Mon→Sun (wrap)", []alerts.DayOfWeek{mon}, -1, []alerts.DayOfWeek{sun}},
		{"shift -1 Tue→Mon", []alerts.DayOfWeek{tue}, -1, []alerts.DayOfWeek{mon}},
		{"shift +1 multiple [Mon,Wed,Fri]→[Tue,Thu,Sat]", []alerts.DayOfWeek{mon, wed, fri}, 1, []alerts.DayOfWeek{tue, alerts.DAYOFWEEK_DAY_OF_WEEK_THURSDAY, sat}},
		{"shift +7 wraps to identity", []alerts.DayOfWeek{mon, wed}, 7, []alerts.DayOfWeek{mon, wed}},
		{"shift -7 wraps to identity", []alerts.DayOfWeek{tue, sat}, -7, []alerts.DayOfWeek{tue, sat}},
		{"empty input", []alerts.DayOfWeek{}, 1, []alerts.DayOfWeek{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ShiftDaysOfWeek(tc.in, tc.shift)
			if len(got) != len(tc.want) {
				t.Fatalf("length mismatch: got %d, want %d", len(got), len(tc.want))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("idx %d: got %v, want %v", i, got[i], tc.want[i])
				}
			}
		})
	}
}
