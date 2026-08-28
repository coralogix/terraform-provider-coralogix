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
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	dashboardschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// A state written by a provider older than v1.13.1 is at schema version 1. That
// version stored layout, variables, filters and annotations with a type the
// current schema cannot hold, so an upgrader that hands those values over
// unchanged fails with a value conversion error and no plan is possible. The
// read has to supply them.
func TestDashboardStateUpgradeFromV1RefreshesChangedAttributes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	server := dashboardStateUpgradeTestServer(t)
	upgrader := dashboardStateUpgraderForVersion(t, ctx, newDashboardOpenAPITestClient(server, ""), 1)
	priorState := tfsdk.State{
		Raw:    dashboardPriorVersionRawValue(ctx, *upgrader.PriorSchema, dashboardErrorPathTestID),
		Schema: *upgrader.PriorSchema,
	}
	response := frameworkresource.UpgradeStateResponse{State: tfsdk.State{Schema: dashboardschema.V4()}}

	upgrader.StateUpgrader(ctx, frameworkresource.UpgradeStateRequest{State: &priorState}, &response)

	if response.Diagnostics.HasError() {
		t.Fatalf("upgrade from schema version 1 diagnostics = %v, want the state the read produced", response.Diagnostics)
	}
	if wantType := dashboardschema.V4().Type().TerraformType(ctx); !response.State.Raw.Type().Equal(wantType) {
		t.Fatalf("upgraded state type = %v, want the v4 resource schema type", response.State.Raw.Type())
	}
	assertDashboardStateID(t, ctx, response.State, dashboardErrorPathTestID)
	assertDashboardStateString(t, ctx, response.State, "name", "upgraded dashboard")
	// The prior state held one filter and one variable at the version 1 type.
	// The read returned neither, so both have to be absent rather than carried
	// over, which is what a value of the old type would be.
	for _, name := range []string{"filters", "variables", "annotations"} {
		var list types.List
		if diagnostics := response.State.GetAttribute(ctx, path.Root(name), &list); diagnostics.HasError() {
			t.Fatalf("read upgraded %s diagnostics = %v", name, diagnostics)
		}
		if !list.IsNull() && len(list.Elements()) != 0 {
			t.Errorf("upgraded %s = %v, want no element, because the read returned none", name, list)
		}
	}
}

// Every wired prior schema has to upgrade into state the framework accepts. A
// new attribute on a type that a prior version shares changes that version's
// stored type, and the upgrader that carries the value over then breaks. This
// test is what turns that into a unit test failure instead of a customer's
// failed plan.
func TestDashboardStateUpgradeFromEveryPriorVersionWritesValidState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	server := dashboardStateUpgradeTestServer(t)
	client := newDashboardOpenAPITestClient(server, "")
	upgraders := DashboardResource{openAPIClient: client}.UpgradeState(ctx)
	if len(upgraders) == 0 {
		t.Fatal("the dashboard resource wires no state upgrader")
	}

	for version := range upgraders {
		t.Run(dashboardPriorVersionName(version), func(t *testing.T) {
			upgrader := dashboardStateUpgraderForVersion(t, ctx, client, version)
			priorState := tfsdk.State{
				Raw:    dashboardPriorVersionRawValue(ctx, *upgrader.PriorSchema, dashboardErrorPathTestID),
				Schema: *upgrader.PriorSchema,
			}
			response := frameworkresource.UpgradeStateResponse{State: tfsdk.State{Schema: dashboardschema.V4()}}

			upgrader.StateUpgrader(ctx, frameworkresource.UpgradeStateRequest{State: &priorState}, &response)

			if response.Diagnostics.HasError() {
				t.Fatalf("upgrade diagnostics = %v, want state at the current schema version", response.Diagnostics)
			}
			if wantType := dashboardschema.V4().Type().TerraformType(ctx); !response.State.Raw.Type().Equal(wantType) {
				t.Fatalf("upgraded state type = %v, want the v4 resource schema type", response.State.Raw.Type())
			}
			assertDashboardStateID(t, ctx, response.State, dashboardErrorPathTestID)
		})
	}
}

// A content_json dashboard keeps its configuration through an upgrade. The read
// cannot rebuild it, because the attribute holds what the user wrote, so it is
// one of the values the seed has to hand over.
func TestDashboardStateUpgradeKeepsContentJSON(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	contentJSON := `{"id":"` + dashboardErrorPathTestID + `","name":"content json dashboard","layout":{"sections":[]}}`
	for _, version := range []int64{1, 2, 3} {
		t.Run(dashboardPriorVersionName(version), func(t *testing.T) {
			server := dashboardStateUpgradeTestServer(t)
			upgrader := dashboardStateUpgraderForVersion(t, ctx, newDashboardOpenAPITestClient(server, ""), version)
			priorState := dashboardErrorPathState(ctx, *upgrader.PriorSchema, dashboardErrorPathTestID, contentJSON)
			response := frameworkresource.UpgradeStateResponse{State: tfsdk.State{Schema: dashboardschema.V4()}}

			upgrader.StateUpgrader(ctx, frameworkresource.UpgradeStateRequest{State: &priorState}, &response)

			if response.Diagnostics.HasError() {
				t.Fatalf("upgrade diagnostics = %v, want the content_json dashboard upgraded", response.Diagnostics)
			}
			if wantType := dashboardschema.V4().Type().TerraformType(ctx); !response.State.Raw.Type().Equal(wantType) {
				t.Fatalf("upgraded state type = %v, want the v4 resource schema type", response.State.Raw.Type())
			}
			assertDashboardStateID(t, ctx, response.State, dashboardErrorPathTestID)
			assertDashboardStateString(t, ctx, response.State, "content_json", contentJSON)
		})
	}
}

func dashboardStateUpgraderForVersion(t *testing.T, ctx context.Context, client *dashboardOpenAPIClient, version int64) frameworkresource.StateUpgrader {
	t.Helper()
	upgrader, ok := DashboardResource{openAPIClient: client}.UpgradeState(ctx)[version]
	if !ok {
		t.Fatalf("the dashboard resource wires no upgrader for schema version %d", version)
	}
	if upgrader.PriorSchema == nil {
		t.Fatalf("the upgrader for schema version %d declares no prior schema", version)
	}
	if upgrader.StateUpgrader == nil {
		t.Fatalf("the upgrader for schema version %d declares no upgrade function", version)
	}
	return upgrader
}

func dashboardPriorVersionName(version int64) string {
	return fmt.Sprintf("prior schema v%d", version)
}

// dashboardPriorVersionRawValue builds state at a prior schema version with a
// value in every attribute that an upgrader could try to carry over: an object
// attribute, and one element in every list attribute. Null alone is not enough,
// because a null value still carries the type of the version that stored it.
func dashboardPriorVersionRawValue(ctx context.Context, priorSchema schema.Schema, id string) tftypes.Value {
	objectType, ok := priorSchema.Type().TerraformType(ctx).(tftypes.Object)
	if !ok {
		panic("dashboard schema Terraform type is not an object")
	}

	attributes := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for name, attributeType := range objectType.AttributeTypes {
		switch typed := attributeType.(type) {
		case tftypes.Object:
			attributes[name] = dashboardNullAttributesValue(typed)
		case tftypes.List:
			elementType, isObject := typed.ElementType.(tftypes.Object)
			if !isObject {
				attributes[name] = tftypes.NewValue(typed, []tftypes.Value{})
				continue
			}
			attributes[name] = tftypes.NewValue(typed, []tftypes.Value{dashboardNullAttributesValue(elementType)})
		default:
			attributes[name] = tftypes.NewValue(attributeType, nil)
		}
	}
	for name, value := range map[string]string{
		"id":          id,
		"name":        "dashboard written by an older provider",
		"description": "state stored at a prior schema version",
	} {
		if attributeType, ok := objectType.AttributeTypes[name]; ok && attributeType.Is(tftypes.String) {
			attributes[name] = tftypes.NewValue(attributeType, value)
		}
	}

	return tftypes.NewValue(objectType, attributes)
}

func dashboardNullAttributesValue(objectType tftypes.Object) tftypes.Value {
	attributes := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for name, attributeType := range objectType.AttributeTypes {
		attributes[name] = tftypes.NewValue(attributeType, nil)
	}
	return tftypes.NewValue(objectType, attributes)
}

func dashboardStateUpgradeTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/dashboards/dashboards/v1/"+dashboardErrorPathTestID {
			t.Errorf("request = %s %s, want dashboard GET", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"dashboard":{"id":"` + dashboardErrorPathTestID + `","name":"upgraded dashboard","layout":{"sections":[]}}}`))
	}))
	t.Cleanup(server.Close)
	return server
}
