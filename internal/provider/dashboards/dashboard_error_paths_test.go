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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dashboardschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const dashboardErrorPathTestID = "123456789012345678901"

func TestDashboardResourceCreateRejectionDoesNotPoisonState(t *testing.T) {
	requests := make([]string, 0, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":400,"message":"invalid dashboard business rule"}`))
	}))
	t.Cleanup(server.Close)

	ctx := context.Background()
	resourceSchema := dashboardschema.V4()
	plan := dashboardErrorPathPlan(ctx, resourceSchema, "", `{"name":"invalid but serializable","layout":{"sections":[]}}`)
	response := frameworkresource.CreateResponse{State: tfsdk.State{Raw: plan.Raw, Schema: resourceSchema}}
	resource := DashboardResource{openAPIClient: newDashboardOpenAPITestClient(server, "")}

	resource.Create(ctx, frameworkresource.CreateRequest{Plan: plan}, &response)

	if !response.Diagnostics.HasError() {
		t.Fatal("Create() diagnostics have no error, want backend rejection")
	}
	assertDashboardStateID(t, ctx, response.State, "")
	if got, want := strings.Join(requests, ", "), "POST /dashboards/dashboards/v1"; got != want {
		t.Fatalf("requests after rejected create = %q, want %q; create must not continue into read/flatten/state writes; diagnostics: %v", got, want, response.Diagnostics)
	}
}

func TestDashboardResourceFailedPostCreateReadCleansUpPartialDashboard(t *testing.T) {
	requests := make([]string, 0, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPost:
			_, _ = w.Write([]byte(`{"dashboardId":"` + dashboardErrorPathTestID + `"}`))
		case http.MethodGet:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code":500,"message":"read after create failed"}`))
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	ctx := context.Background()
	resourceSchema := dashboardschema.V4()
	plan := dashboardErrorPathPlan(ctx, resourceSchema, "", `{"name":"partial dashboard","layout":{"sections":[]}}`)
	response := frameworkresource.CreateResponse{State: tfsdk.State{Raw: plan.Raw, Schema: resourceSchema}}
	resource := DashboardResource{openAPIClient: newDashboardOpenAPITestClient(server, "")}

	resource.Create(ctx, frameworkresource.CreateRequest{Plan: plan}, &response)

	if !response.Diagnostics.HasError() {
		t.Fatal("Create() diagnostics have no error, want post-create read failure")
	}
	assertDashboardStateID(t, ctx, response.State, "")
	wantRequests := []string{
		"POST /dashboards/dashboards/v1",
		"GET /dashboards/dashboards/v1/" + dashboardErrorPathTestID,
		"DELETE /dashboards/dashboards/v1/" + dashboardErrorPathTestID,
	}
	if got, want := strings.Join(requests, ", "), strings.Join(wantRequests, ", "); got != want {
		t.Fatalf("requests after partial create = %q, want deterministic cleanup sequence %q", got, want)
	}
}

func TestDashboardResourceRejectedReplaceKeepsPriorStateUsableOnRefresh(t *testing.T) {
	requests := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPut:
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"code":422,"message":"replacement rejected"}`))
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"dashboard":{"id":"` + dashboardErrorPathTestID + `","name":"prior remote dashboard","layout":{"sections":[]}}}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	ctx := context.Background()
	resourceSchema := dashboardschema.V4()
	priorState := dashboardErrorPathState(ctx, resourceSchema, dashboardErrorPathTestID, `{"id":"`+dashboardErrorPathTestID+`","name":"prior remote dashboard","layout":{"sections":[]}}`)
	plan := dashboardErrorPathPlan(ctx, resourceSchema, dashboardErrorPathTestID, `{"id":"`+dashboardErrorPathTestID+`","name":"rejected replacement","layout":{"sections":[]}}`)
	config := tfsdk.Config{Raw: plan.Raw, Schema: resourceSchema}
	updateResponse := frameworkresource.UpdateResponse{State: priorState}
	resource := DashboardResource{openAPIClient: newDashboardOpenAPITestClient(server, "")}

	resource.Update(ctx, frameworkresource.UpdateRequest{Config: config, Plan: plan, State: priorState}, &updateResponse)
	if !updateResponse.Diagnostics.HasError() {
		t.Fatal("Update() diagnostics have no error, want backend rejection")
	}
	assertDashboardStateID(t, ctx, updateResponse.State, dashboardErrorPathTestID)
	if len(requests) != 1 || !strings.HasPrefix(requests[0], http.MethodPut+" ") {
		t.Fatalf("requests after rejected replace = %v, want only PUT; diagnostics: %v", requests, updateResponse.Diagnostics)
	}

	readResponse := frameworkresource.ReadResponse{State: priorState}
	resource.Read(ctx, frameworkresource.ReadRequest{State: priorState}, &readResponse)
	if readResponse.Diagnostics.HasError() {
		t.Fatalf("Read() after rejected replace diagnostics = %v", readResponse.Diagnostics)
	}
	assertDashboardStateID(t, ctx, readResponse.State, dashboardErrorPathTestID)
	if len(requests) != 2 || !strings.HasPrefix(requests[1], http.MethodGet+" ") {
		t.Fatalf("requests after refresh = %v, want rejected PUT followed by GET", requests)
	}
}

func TestDashboardResourceReadNotFoundRemovesStateWithWarning(t *testing.T) {
	server := dashboardNotFoundTestServer(t)
	defer server.Close()

	ctx := context.Background()
	resourceSchema := dashboardschema.V4()
	state := dashboardErrorPathState(ctx, resourceSchema, dashboardErrorPathTestID, `{"id":"`+dashboardErrorPathTestID+`"}`)
	response := frameworkresource.ReadResponse{State: state}
	resource := DashboardResource{openAPIClient: newDashboardOpenAPITestClient(server, "")}

	resource.Read(ctx, frameworkresource.ReadRequest{State: state}, &response)

	if response.Diagnostics.HasError() {
		t.Fatalf("Read() diagnostics = %v, want warning only", response.Diagnostics)
	}
	if response.Diagnostics.WarningsCount() != 1 {
		t.Fatalf("Read() warning count = %d, want 1", response.Diagnostics.WarningsCount())
	}
	if !response.State.Raw.IsNull() {
		t.Fatalf("Read() state = %#v, want removed resource", response.State.Raw)
	}
}

func TestDashboardResourceDeleteAlreadyAbsentSucceeds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/dashboards/dashboards/v1/"+dashboardErrorPathTestID {
			t.Fatalf("request = %s %s, want dashboard DELETE", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":404,"message":"dashboard is already absent"}`))
	}))
	t.Cleanup(server.Close)

	ctx := context.Background()
	resourceSchema := dashboardschema.V4()
	state := dashboardErrorPathState(ctx, resourceSchema, dashboardErrorPathTestID, `{"id":"`+dashboardErrorPathTestID+`"}`)
	response := frameworkresource.DeleteResponse{State: state}
	resource := DashboardResource{openAPIClient: newDashboardOpenAPITestClient(server, "")}

	resource.Delete(ctx, frameworkresource.DeleteRequest{State: state}, &response)

	if response.Diagnostics.HasError() {
		t.Fatalf("Delete() already-absent dashboard diagnostics = %v, want success", response.Diagnostics)
	}
}

func TestDashboardStateUpgradeNotFoundRemovesStateWithWarning(t *testing.T) {
	server := dashboardNotFoundTestServer(t)
	defer server.Close()

	ctx := context.Background()
	priorSchema := dashboardschema.V3()
	priorState := dashboardErrorPathState(ctx, priorSchema, dashboardErrorPathTestID, `{"id":"`+dashboardErrorPathTestID+`"}`)
	response := frameworkresource.UpgradeStateResponse{State: tfsdk.State{Schema: dashboardschema.V4()}}

	upgradeDashboardStateV3ToV4(
		ctx,
		frameworkresource.UpgradeStateRequest{State: &priorState},
		&response,
		newDashboardOpenAPITestClient(server, ""),
	)

	if response.Diagnostics.HasError() {
		t.Fatalf("state upgrade diagnostics = %v, want warning only", response.Diagnostics)
	}
	if response.Diagnostics.WarningsCount() != 1 {
		t.Fatalf("state upgrade warning count = %d, want 1", response.Diagnostics.WarningsCount())
	}
	if !response.State.Raw.IsNull() {
		t.Fatalf("state upgrade state = %#v, want removed resource", response.State.Raw)
	}
}

func dashboardNotFoundTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/dashboards/dashboards/v1/"+dashboardErrorPathTestID {
			t.Fatalf("request = %s %s, want dashboard GET", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":404,"message":"dashboard no longer exists"}`))
	}))
}

func dashboardErrorPathPlan(ctx context.Context, resourceSchema schema.Schema, id, contentJSON string) tfsdk.Plan {
	return tfsdk.Plan{
		Raw:    dashboardErrorPathRawValue(ctx, resourceSchema, id, contentJSON),
		Schema: resourceSchema,
	}
}

func dashboardErrorPathState(ctx context.Context, resourceSchema schema.Schema, id, contentJSON string) tfsdk.State {
	return tfsdk.State{
		Raw:    dashboardErrorPathRawValue(ctx, resourceSchema, id, contentJSON),
		Schema: resourceSchema,
	}
}

func dashboardErrorPathRawValue(ctx context.Context, resourceSchema schema.Schema, id, contentJSON string) tftypes.Value {
	terraformType := resourceSchema.Type().TerraformType(ctx)
	objectType, ok := terraformType.(tftypes.Object)
	if !ok {
		panic("dashboard schema Terraform type is not an object")
	}

	attributes := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for name, attributeType := range objectType.AttributeTypes {
		attributes[name] = tftypes.NewValue(attributeType, nil)
	}
	if id != "" {
		attributes["id"] = tftypes.NewValue(objectType.AttributeTypes["id"], id)
	}
	if contentJSON != "" {
		attributes["content_json"] = tftypes.NewValue(objectType.AttributeTypes["content_json"], contentJSON)
	}
	return tftypes.NewValue(terraformType, attributes)
}

func assertDashboardStateID(t *testing.T, ctx context.Context, state tfsdk.State, want string) {
	t.Helper()
	var id types.String
	diagnostics := state.GetAttribute(ctx, path.Root("id"), &id)
	if diagnostics.HasError() {
		t.Fatalf("read state ID diagnostics = %v", diagnostics)
	}
	if want == "" {
		if !id.IsNull() {
			t.Fatalf("state ID = %q, want null", id.ValueString())
		}
		return
	}
	if id.ValueString() != want {
		t.Fatalf("state ID = %q, want %q", id.ValueString(), want)
	}
}
