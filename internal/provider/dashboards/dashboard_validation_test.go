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
	"strings"
	"testing"
	"time"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	dashboardschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_schema"
)

func TestAttributePathFromPointer(t *testing.T) {
	for _, tc := range []struct {
		name    string
		pointer string
		want    path.Path
		wantOK  bool
	}{
		{
			name:    "widget location from the check endpoint",
			pointer: "/layout/sections/0/rows/0/widgets/1",
			want: path.Root("layout").AtName("sections").AtListIndex(0).
				AtName("rows").AtListIndex(0).AtName("widgets").AtListIndex(1),
			wantOK: true,
		},
		{
			name:    "section location",
			pointer: "/layout/sections/0",
			want:    path.Root("layout").AtName("sections").AtListIndex(0),
			wantOK:  true,
		},
		{
			name:    "camel case segment becomes snake case",
			pointer: "/variablesV2/3",
			want:    path.Root("variables_v2").AtListIndex(3),
			wantOK:  true,
		},
		{
			// The API keeps the folder id at the root of the dashboard, the
			// schema nests it. Without the alias this would map to the
			// nonexistent "folder_id".
			name:    "folder id is aliased onto the nested attribute",
			pointer: "/folderId",
			want:    path.Root("folder").AtName("id"),
			wantOK:  true,
		},
		{
			name:    "folder path is aliased onto the nested attribute",
			pointer: "/folderPath",
			want:    path.Root("folder").AtName("path"),
			wantOK:  true,
		},
		{name: "root level issue", pointer: "", wantOK: false},
		{name: "root pointer", pointer: "/", wantOK: false},
		{name: "empty segment", pointer: "/layout//sections", wantOK: false},
		{name: "negative index", pointer: "/layout/sections/-1", wantOK: false},
		{name: "escaped name that is not in the schema", pointer: "/layout/~0odd~1name", wantOK: false},
		// Terraform keeps these somewhere else entirely: dashboard level
		// actions live on the widget that owns them, and the refresh interval
		// is a single auto_refresh attribute rather than one field per value.
		{name: "dashboard actions have no schema path", pointer: "/actions/0", wantOK: false},
		{name: "auto refresh field has no schema path", pointer: "/twoMinutes", wantOK: false},
		{name: "unknown field", pointer: "/nonexistent", wantOK: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := attributePathFromPointer(tc.pointer, schemaResolver())
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !tc.wantOK {
				return
			}
			if got.String() != tc.want.String() {
				t.Errorf("path = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestCamelToSnake(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"layout", "layout"},
		{"sections", "sections"},
		{"folderId", "folder_id"},
		{"variablesV2", "variables_v2"},
		{"absoluteTimeFrame", "absolute_time_frame"},
		{"relativeTimeFrame", "relative_time_frame"},
		{"v1", "v1"},
		{"", ""},
	} {
		if got := camelToSnake(tc.in); got != tc.want {
			t.Errorf("camelToSnake(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDashboardValidationSettingsFromEnv(t *testing.T) {
	for _, tc := range []struct {
		name        string
		enabled     string
		timeout     string
		wantEnabled bool
		wantTimeout time.Duration
	}{
		{name: "defaults", wantEnabled: true, wantTimeout: dashboardValidationDefaultTimeout},
		{name: "disabled", enabled: "false", wantEnabled: false, wantTimeout: dashboardValidationDefaultTimeout},
		{name: "disabled with 0", enabled: "0", wantEnabled: false, wantTimeout: dashboardValidationDefaultTimeout},
		{name: "custom timeout", timeout: "12s", wantEnabled: true, wantTimeout: 12 * time.Second},
		{name: "garbage is ignored", enabled: "maybe", timeout: "soon", wantEnabled: true, wantTimeout: dashboardValidationDefaultTimeout},
		{name: "zero timeout is ignored", timeout: "0s", wantEnabled: true, wantTimeout: dashboardValidationDefaultTimeout},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(dashboardValidationEnabledEnvVar, tc.enabled)
			t.Setenv(dashboardValidationTimeoutEnvVar, tc.timeout)

			got := dashboardValidationSettingsFromEnv()
			if got.Enabled != tc.wantEnabled {
				t.Errorf("Enabled = %v, want %v", got.Enabled, tc.wantEnabled)
			}
			if got.Timeout != tc.wantTimeout {
				t.Errorf("Timeout = %s, want %s", got.Timeout, tc.wantTimeout)
			}
		})
	}
}

func TestUnknownUserValuePaths(t *testing.T) {
	objectType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":   tftypes.String,
		"name": tftypes.String,
	}}
	computedOnly := func(attributePath *tftypes.AttributePath) bool {
		return attributePath.String() == tftypes.NewAttributePath().WithAttributeName("id").String()
	}

	for _, tc := range []struct {
		name string
		id   any
		attr any
		want bool
	}{
		{name: "everything known", id: "abc", attr: "my dashboard", want: false},
		{name: "computed id unknown on create", id: tftypes.UnknownValue, attr: "my dashboard", want: false},
		{name: "user value unknown", id: "abc", attr: tftypes.UnknownValue, want: true},
		{name: "both unknown", id: tftypes.UnknownValue, attr: tftypes.UnknownValue, want: true},
		{name: "null user value is known", id: "abc", attr: nil, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := tftypes.NewValue(objectType, map[string]tftypes.Value{
				"id":   tftypes.NewValue(tftypes.String, tc.id),
				"name": tftypes.NewValue(tftypes.String, tc.attr),
			})
			got, convertible := unknownUserValuePaths(plan, computedOnly)
			if !convertible {
				t.Fatal("expected every unknown path to be convertible")
			}
			if (len(got) > 0) != tc.want {
				t.Errorf("unknownUserValuePaths = %v, want any unknown = %v", got, tc.want)
			}
		})
	}
}

func TestAddDashboardIssueWarningsNeverAddsErrors(t *testing.T) {
	var diagnostics diag.Diagnostics
	addDashboardIssueWarnings(&diagnostics, []dashboardservice.Issue{
		issueFixture(dashboardservice.ISSUESEVERITY_SEVERITY_ERROR, "/layout/sections/0", "section id is required"),
		issueFixture(dashboardservice.ISSUESEVERITY_SEVERITY_WARNING, "", "deprecated function"),
	}, nil, schemaResolver(), dashboardConfigShape{})

	if diagnostics.HasError() {
		t.Fatalf("expected warnings only, got errors: %v", diagnostics.Errors())
	}
	if got := len(diagnostics.Warnings()); got != 2 {
		t.Fatalf("warnings = %d, want 2", got)
	}

	withPath := diagnostics.Warnings()[0]
	if withPath.Summary() != "Dashboard validation error" {
		t.Errorf("summary = %q, want %q", withPath.Summary(), "Dashboard validation error")
	}
	attributeDiagnostic, ok := withPath.(diag.DiagnosticWithPath)
	if !ok {
		t.Fatalf("expected a diagnostic carrying an attribute path, got %T", withPath)
	}
	wantPath := path.Root("layout").AtName("sections").AtListIndex(0)
	if attributeDiagnostic.Path().String() != wantPath.String() {
		t.Errorf("path = %s, want %s", attributeDiagnostic.Path(), wantPath)
	}

	// A root level issue has no usable pointer, so it stays on the resource.
	rootIssue := diagnostics.Warnings()[1]
	if _, ok := rootIssue.(diag.DiagnosticWithPath); ok {
		t.Error("root level issue should not carry an attribute path")
	}
	if rootIssue.Summary() != "Dashboard validation warning" {
		t.Errorf("summary = %q, want %q", rootIssue.Summary(), "Dashboard validation warning")
	}
}

func TestAddDashboardIssueWarningsKeepsUnmappedLocationInMessage(t *testing.T) {
	var diagnostics diag.Diagnostics
	addDashboardIssueWarnings(&diagnostics, []dashboardservice.Issue{
		issueFixture(dashboardservice.ISSUESEVERITY_SEVERITY_WARNING, "/layout//sections", "odd pointer"),
	}, nil, schemaResolver(), dashboardConfigShape{})

	if got := len(diagnostics.Warnings()); got != 1 {
		t.Fatalf("warnings = %d, want 1", got)
	}
	if detail := diagnostics.Warnings()[0].Detail(); !strings.Contains(detail, "/layout//sections") {
		t.Errorf("detail = %q, want it to keep the raw pointer", detail)
	}
}

func TestPathsOverlap(t *testing.T) {
	widget0 := path.Root("layout").AtName("sections").AtListIndex(0).
		AtName("rows").AtListIndex(0).AtName("widgets").AtListIndex(0)

	for _, tc := range []struct {
		name        string
		left, right path.Path
		want        bool
	}{
		{name: "same path", left: widget0, right: widget0, want: true},
		{name: "child of unknown", left: widget0, right: widget0.AtName("title"), want: true},
		{name: "parent of unknown", left: widget0.AtName("reference").AtName("dashboard_id"), right: widget0, want: true},
		{
			name:  "sibling widget is unaffected",
			left:  widget0,
			right: widget0.ParentPath().AtListIndex(1),
		},
		{
			name:  "index prefix does not match a longer index",
			left:  path.Root("layout").AtName("sections").AtListIndex(1),
			right: path.Root("layout").AtName("sections").AtListIndex(10),
		},
		{
			name:  "unrelated attributes",
			left:  path.Root("folder").AtName("id"),
			right: path.Root("name"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := pathsOverlap(tc.left, tc.right); got != tc.want {
				t.Errorf("pathsOverlap(%s, %s) = %v, want %v", tc.left, tc.right, got, tc.want)
			}
		})
	}
}

func TestAddDashboardIssueWarningsDropsIssuesCoveringUnknownValues(t *testing.T) {
	widget0 := "/layout/sections/0/rows/0/widgets/0"
	widget1 := "/layout/sections/0/rows/0/widgets/1"
	unknown := []path.Path{
		path.Root("layout").AtName("sections").AtListIndex(0).
			AtName("rows").AtListIndex(0).AtName("widgets").AtListIndex(0).
			AtName("reference").AtName("dashboard_id"),
	}

	var diagnostics diag.Diagnostics
	addDashboardIssueWarnings(&diagnostics, []dashboardservice.Issue{
		issueFixture(dashboardservice.ISSUESEVERITY_SEVERITY_ERROR, widget0, "referenced dashboard does not exist"),
		issueFixture(dashboardservice.ISSUESEVERITY_SEVERITY_ERROR, widget1, "duplicate widget id"),
		issueFixture(dashboardservice.ISSUESEVERITY_SEVERITY_WARNING, "", "dashboard level issue"),
	}, unknown, schemaResolver(), dashboardConfigShape{})

	warnings := diagnostics.Warnings()
	if len(warnings) != 1 {
		t.Fatalf("warnings = %d, want 1: %v", len(warnings), warnings)
	}
	if detail := warnings[0].Detail(); detail != "duplicate widget id" {
		t.Errorf("detail = %q, want the issue about the sibling widget", detail)
	}
}

// An unknown folder.id is the pattern the documentation recommends. The API
// reports folder problems at /folderId, so without the alias the issue would
// map to a path that matches neither the schema nor the unknown value, and the
// warning would survive filtering and point at nothing.
func TestAddDashboardIssueWarningsDropsFolderIssueWhenFolderIDIsUnknown(t *testing.T) {
	unknown := []path.Path{path.Root("folder").AtName("id")}

	var diagnostics diag.Diagnostics
	addDashboardIssueWarnings(&diagnostics, []dashboardservice.Issue{
		issueFixture(dashboardservice.ISSUESEVERITY_SEVERITY_ERROR, "/folderId", "folder id is required"),
	}, unknown, schemaResolver(), dashboardConfigShape{})

	if got := len(diagnostics.Warnings()); got != 0 {
		t.Fatalf("warnings = %d, want 0: %v", got, diagnostics.Warnings())
	}
}

// With content_json the structured attributes are null, so layout.sections[0]
// names nothing the user can edit. The warning has to land on content_json and
// keep the pointer, which is the only way back into the JSON.
func TestAddDashboardIssueWarningsRoutesContentJSONIssues(t *testing.T) {
	var diagnostics diag.Diagnostics
	addDashboardIssueWarnings(&diagnostics, []dashboardservice.Issue{
		issueFixture(dashboardservice.ISSUESEVERITY_SEVERITY_ERROR, "/filters/0", "filter id is required"),
		issueFixture(dashboardservice.ISSUESEVERITY_SEVERITY_ERROR, "/folderId", "folder id is required"),
	}, nil, schemaResolver(), dashboardConfigShape{ContentJSON: true, FolderConfigured: true})

	warnings := diagnostics.Warnings()
	if len(warnings) != 2 {
		t.Fatalf("warnings = %d, want 2", len(warnings))
	}

	fromJSON, ok := warnings[0].(diag.DiagnosticWithPath)
	if !ok {
		t.Fatalf("expected an attribute diagnostic, got %T", warnings[0])
	}
	if got := fromJSON.Path().String(); got != "content_json" {
		t.Errorf("path = %s, want content_json", got)
	}
	if detail := fromJSON.Detail(); !strings.Contains(detail, "/filters/0") {
		t.Errorf("detail = %q, want it to keep the pointer into the JSON", detail)
	}

	// The folder is merged from the folder attribute even in this mode, so it
	// still points at the HCL the user wrote.
	fromHCL, ok := warnings[1].(diag.DiagnosticWithPath)
	if !ok {
		t.Fatalf("expected an attribute diagnostic, got %T", warnings[1])
	}
	if got := fromHCL.Path().String(); got != "folder.id" {
		t.Errorf("path = %s, want folder.id", got)
	}
	if detail := fromHCL.Detail(); strings.Contains(detail, "/folderId") {
		t.Errorf("detail = %q, should not repeat the pointer when the path is configured", detail)
	}
}

// Without a folder attribute, expandOpenAPIDashboardFolder leaves the parsed
// dashboard alone, so the folder in the request came from the JSON. Pointing at
// folder.id would name something the user never wrote.
func TestAddDashboardIssueWarningsRoutesJSONOwnedFolderToContentJSON(t *testing.T) {
	var diagnostics diag.Diagnostics
	addDashboardIssueWarnings(&diagnostics, []dashboardservice.Issue{
		issueFixture(dashboardservice.ISSUESEVERITY_SEVERITY_ERROR, "/folderId", "folder id is required"),
	}, nil, schemaResolver(), dashboardConfigShape{ContentJSON: true, FolderConfigured: false})

	warnings := diagnostics.Warnings()
	if len(warnings) != 1 {
		t.Fatalf("warnings = %d, want 1", len(warnings))
	}

	warning, ok := warnings[0].(diag.DiagnosticWithPath)
	if !ok {
		t.Fatalf("expected an attribute diagnostic, got %T", warnings[0])
	}
	if got := warning.Path().String(); got != "content_json" {
		t.Errorf("path = %s, want content_json", got)
	}
	if detail := warning.Detail(); !strings.Contains(detail, "/folderId") {
		t.Errorf("detail = %q, want it to keep the pointer into the JSON", detail)
	}
}

func TestAddDashboardIssueWarningsKeepsEverythingWhenNothingIsUnknown(t *testing.T) {
	var diagnostics diag.Diagnostics
	addDashboardIssueWarnings(&diagnostics, []dashboardservice.Issue{
		issueFixture(dashboardservice.ISSUESEVERITY_SEVERITY_WARNING, "", "dashboard level issue"),
		issueFixture(dashboardservice.ISSUESEVERITY_SEVERITY_ERROR, "/layout/sections/0", "section issue"),
	}, nil, schemaResolver(), dashboardConfigShape{})

	if got := len(diagnostics.Warnings()); got != 2 {
		t.Fatalf("warnings = %d, want 2", got)
	}
}

func TestAddDashboardIssueWarningsCapsOutput(t *testing.T) {
	issues := make([]dashboardservice.Issue, 0, dashboardValidationMaxReportedIssues+5)
	for i := 0; i < cap(issues); i++ {
		issues = append(issues, issueFixture(
			dashboardservice.ISSUESEVERITY_SEVERITY_WARNING,
			fmt.Sprintf("/layout/sections/%d", i),
			"an issue",
		))
	}

	var diagnostics diag.Diagnostics
	addDashboardIssueWarnings(&diagnostics, issues, nil, schemaResolver(), dashboardConfigShape{})

	// Every reported issue, plus one warning saying the rest were omitted.
	if got, want := len(diagnostics.Warnings()), dashboardValidationMaxReportedIssues+1; got != want {
		t.Fatalf("warnings = %d, want %d", got, want)
	}
	last := diagnostics.Warnings()[len(diagnostics.Warnings())-1]
	if !strings.Contains(last.Detail(), "5 more") {
		t.Errorf("detail = %q, want it to report the omitted count", last.Detail())
	}
}

func TestModifyPlanSkipsDestroyPlan(t *testing.T) {
	requests := 0
	resourceUnderTest := dashboardResourceForTest(t, func(w http.ResponseWriter, _ *http.Request) {
		requests++
		writeIssues(w)
	})

	objectType := dashboardschema.V4().Type().TerraformType(context.Background())
	req := resource.ModifyPlanRequest{
		Plan: tfsdk.Plan{Schema: dashboardschema.V4(), Raw: tftypes.NewValue(objectType, nil)},
	}
	resp := &resource.ModifyPlanResponse{}

	resourceUnderTest.ModifyPlan(context.Background(), req, resp)

	if requests != 0 {
		t.Errorf("made %d validation requests on a destroy plan, want 0", requests)
	}
	if len(resp.Diagnostics) != 0 {
		t.Errorf("diagnostics = %v, want none", resp.Diagnostics)
	}
}

func TestModifyPlanSkipsUnchangedResource(t *testing.T) {
	requests := 0
	resourceUnderTest := dashboardResourceForTest(t, func(w http.ResponseWriter, _ *http.Request) {
		requests++
		writeIssues(w)
	})

	dashboardSchema := dashboardschema.V4()
	unchanged := emptyDashboardObject(t)

	req := resource.ModifyPlanRequest{
		State: tfsdk.State{Schema: dashboardSchema, Raw: unchanged},
		Plan:  tfsdk.Plan{Schema: dashboardSchema, Raw: unchanged},
	}
	resp := &resource.ModifyPlanResponse{}

	resourceUnderTest.ModifyPlan(context.Background(), req, resp)

	if requests != 0 {
		t.Errorf("made %d validation requests for an unchanged dashboard, want 0", requests)
	}
}

// TestModifyPlanValidatesCreate is the control for the test above: the same
// plan value with no prior state must reach the API, otherwise the unchanged
// case would pass for the wrong reason.
func TestModifyPlanValidatesCreate(t *testing.T) {
	requests := 0
	resourceUnderTest := dashboardResourceForTest(t, func(w http.ResponseWriter, _ *http.Request) {
		requests++
		writeIssues(w)
	})

	dashboardSchema := dashboardschema.V4()
	objectType := dashboardSchema.Type().TerraformType(context.Background())

	req := resource.ModifyPlanRequest{
		State: tfsdk.State{Schema: dashboardSchema, Raw: tftypes.NewValue(objectType, nil)},
		Plan:  tfsdk.Plan{Schema: dashboardSchema, Raw: emptyDashboardObject(t)},
	}
	resp := &resource.ModifyPlanResponse{}

	resourceUnderTest.ModifyPlan(context.Background(), req, resp)

	if requests != 1 {
		t.Errorf("made %d validation requests on create, want 1", requests)
	}
	if resp.Diagnostics.HasError() {
		t.Errorf("diagnostics = %v, want no errors", resp.Diagnostics.Errors())
	}
}

func TestModifyPlanWithoutClientDoesNothing(t *testing.T) {
	unconfigured := &DashboardResource{}
	objectType := dashboardschema.V4().Type().TerraformType(context.Background())

	req := resource.ModifyPlanRequest{
		Plan: tfsdk.Plan{Schema: dashboardschema.V4(), Raw: tftypes.NewValue(objectType, nil)},
	}
	resp := &resource.ModifyPlanResponse{}

	unconfigured.ModifyPlan(context.Background(), req, resp)

	if len(resp.Diagnostics) != 0 {
		t.Errorf("diagnostics = %v, want none", resp.Diagnostics)
	}
}

func TestCheckReturnsIssues(t *testing.T) {
	client := dashboardClientForTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dashboards/check/v1" {
			t.Errorf("path = %q, want /dashboards/check/v1", r.URL.Path)
		}
		writeIssues(w,
			`{"severity":"SEVERITY_ERROR","location":"/layout/sections/0/rows/0/widgets/1","message":"duplicate widget id"}`)
	})

	issues, err := client.Check(context.Background(), &dashboardservice.Dashboard{Name: "probe"})
	if err != nil {
		t.Fatalf("Check: %s", err)
	}
	if len(issues) != 1 {
		t.Fatalf("issues = %d, want 1", len(issues))
	}
	if got := issues[0].GetMessage(); got != "duplicate widget id" {
		t.Errorf("message = %q, want %q", got, "duplicate widget id")
	}
}

func TestCheckFailureIsReportedAsError(t *testing.T) {
	client := dashboardClientForTest(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"boom"}`))
	})

	if _, err := client.Check(context.Background(), &dashboardservice.Dashboard{Name: "probe"}); err == nil {
		t.Fatal("expected an error from a failing check call")
	}
}

func issueFixture(severity dashboardservice.IssueSeverity, location, message string) dashboardservice.Issue {
	return dashboardservice.Issue{
		Severity: &severity,
		Location: &location,
		Message:  &message,
	}
}

func writeIssues(w http.ResponseWriter, issues ...string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"issues":[%s]}`, strings.Join(issues, ","))
}

// emptyDashboardObject builds a dashboard object whose attributes are all null.
// The object itself is not null, so it exercises the plan paths that a null
// value would short-circuit.
func emptyDashboardObject(t *testing.T) tftypes.Value {
	t.Helper()

	objectType, ok := dashboardschema.V4().Type().TerraformType(context.Background()).(tftypes.Object)
	if !ok {
		t.Fatal("dashboard schema is not an object type")
	}

	attributes := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for name, attributeType := range objectType.AttributeTypes {
		attributes[name] = tftypes.NewValue(attributeType, nil)
	}

	return tftypes.NewValue(objectType, attributes)
}

// schemaResolver reports whether a path exists in the real dashboard schema,
// the same check ModifyPlan performs.
func schemaResolver() func(*tftypes.AttributePath) bool {
	dashboardSchema := dashboardschema.V4()
	return func(attributePath *tftypes.AttributePath) bool {
		_, err := dashboardSchema.AttributeAtTerraformPath(context.Background(), attributePath)
		return err == nil
	}
}

func dashboardClientForTest(t *testing.T, handler http.HandlerFunc) *dashboardOpenAPIClient {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	configuration := dashboardservice.NewConfiguration()
	configuration.Servers = dashboardservice.ServerConfigurations{{URL: server.URL}}

	return newDashboardOpenAPIClient(dashboardservice.NewAPIClient(configuration).DashboardServiceAPI)
}

func dashboardResourceForTest(t *testing.T, handler http.HandlerFunc) *DashboardResource {
	t.Helper()
	return &DashboardResource{openAPIClient: dashboardClientForTest(t, handler)}
}
