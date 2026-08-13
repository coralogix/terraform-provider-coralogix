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

package dashboard_widgets

import (
	"sort"
	"testing"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
)

// An enum value the API can return but the map does not contain flattens to
// null, and the schema's default then writes "unspecified" on the next apply —
// silently replacing whatever the user or the UI had set. So every value the
// pinned SDK accepts must be mapped, not just the ones a test happens to use.
//
// This is the check that would have caught UNIT_PERCENT and UNIT_DATETIME_ISO
// being absent while the gauge map already had percent.
func TestUnitMapsCoverEverySDKValue(t *testing.T) {
	t.Run("CommonUnit", func(t *testing.T) {
		mapped := make(map[dashboardservice.CommonUnit]struct{}, len(DashboardSchemaToProtoUnit))
		for _, v := range DashboardSchemaToProtoUnit {
			mapped[v] = struct{}{}
		}
		var missing []string
		for _, v := range dashboardservice.AllowedCommonUnitEnumValues {
			if _, ok := mapped[v]; !ok {
				missing = append(missing, string(v))
			}
		}
		sort.Strings(missing)
		if len(missing) > 0 {
			t.Errorf("DashboardSchemaToProtoUnit does not map %d SDK value(s): %v.\n"+
				"Reads of those values flatten to null and the default overwrites them on the next apply.", len(missing), missing)
		}
	})

	t.Run("GaugeUnit", func(t *testing.T) {
		mapped := make(map[dashboardservice.GaugeUnit]struct{}, len(DashboardSchemaToProtoGaugeUnit))
		for _, v := range DashboardSchemaToProtoGaugeUnit {
			mapped[v] = struct{}{}
		}
		var missing []string
		for _, v := range dashboardservice.AllowedGaugeUnitEnumValues {
			if _, ok := mapped[v]; !ok {
				missing = append(missing, string(v))
			}
		}
		sort.Strings(missing)
		if len(missing) > 0 {
			t.Errorf("DashboardSchemaToProtoGaugeUnit does not map %d SDK value(s): %v.", len(missing), missing)
		}
	})
}
