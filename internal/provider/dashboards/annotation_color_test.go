// Copyright 2026 Coralogix Ltd.
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

package dashboards

import (
	"testing"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_widgets"
)

// An annotation created without a colour reads back as unspecified from the API.
// The attribute is plain optional, so that has to flatten to null: a value where
// the configuration had none fails the apply with "provider produced
// inconsistent result after apply". Writing "unspecified" is not accepted
// either, because omitting the attribute already says the same thing.
func TestAnnotationColorUnspecifiedReadsBackAsNull(t *testing.T) {
	unspecified := dashboardservice.ANNOTATIONCOLOR_ANNOTATION_COLOR_UNSPECIFIED
	flattened := dashboard_widgets.FlattenOptionalEnum(&unspecified, dashboard_widgets.DashboardProtoToSchemaAnnotationColor)
	if !flattened.IsNull() {
		t.Fatalf("an unspecified colour flattened to %q, want null", flattened.ValueString())
	}

	red := dashboardservice.ANNOTATIONCOLOR_ANNOTATION_COLOR_RED
	if got := dashboard_widgets.FlattenOptionalEnum(&red, dashboard_widgets.DashboardProtoToSchemaAnnotationColor); got.ValueString() != "red" {
		t.Fatalf("a red colour flattened to %q, want red", got.ValueString())
	}

	for _, value := range dashboard_widgets.DashboardValidAnnotationColors {
		if value == "unspecified" {
			t.Fatal("the accepted colours include unspecified, so a user can write a value that cannot round trip")
		}
	}
}
