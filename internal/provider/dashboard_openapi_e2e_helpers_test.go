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

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// dashboardOpenAPIFixtureName returns one unique name to reuse for every step
// of a fixture. Call it once per test; changing the name between update steps
// makes identity-preservation assertions meaningless.
func dashboardOpenAPIFixtureName(fixture string) string {
	prefix := strings.Trim(strings.Map(func(r rune) rune {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			return unicode.ToLower(r)
		default:
			return '-'
		}
	}, fixture), "-")
	if prefix == "" {
		prefix = "fixture"
	}
	if len(prefix) > 32 {
		prefix = prefix[:32]
	}

	return acctest.RandomWithPrefix("tf-acc-dashboard-" + prefix)
}

// dashboardOpenAPIBranchFixture carries the coverage-manifest metadata needed
// to build one branch variant. In particular, API-only branches may still have
// import/data-source hydration support, so callers must not infer hydration
// from Status alone.
type dashboardOpenAPIBranchFixture struct {
	Model               string
	Branch              string
	ProviderPath        string
	Fixture             string
	Status              dashboardOneOfCoverageStatus
	ImportHydration     bool
	DataSourceHydration bool
}

func dashboardOpenAPIBranchFixtureFor(model, branch string) (dashboardOpenAPIBranchFixture, error) {
	modelCoverage, ok := dashboardOpenAPIOneOfCoverage[model]
	if !ok {
		return dashboardOpenAPIBranchFixture{}, fmt.Errorf("dashboard oneof model %q is absent from the coverage manifest", model)
	}
	branchCoverage, ok := modelCoverage.Branches[branch]
	if !ok {
		return dashboardOpenAPIBranchFixture{}, fmt.Errorf("dashboard oneof branch %s.%s is absent from the coverage manifest", model, branch)
	}
	fixture := branchCoverage.FixtureOrTest
	if fixture == "" {
		fixture = model + "-" + branch
	}

	return dashboardOpenAPIBranchFixture{
		Model:               model,
		Branch:              branch,
		ProviderPath:        branchCoverage.ProviderPath,
		Fixture:             fixture,
		Status:              branchCoverage.Status,
		ImportHydration:     branchCoverage.ImportHydration,
		DataSourceHydration: branchCoverage.DataSourceHydration,
	}, nil
}

func (fixture dashboardOpenAPIBranchFixture) UniqueName() string {
	return dashboardOpenAPIFixtureName(fixture.Fixture)
}

func (fixture dashboardOpenAPIBranchFixture) AssertOneOf(carrier any, dashboardID string) error {
	return dashboardOpenAPIAssertOneOfBranch(carrier, fixture.Model, fixture.Branch, dashboardID, fixture.Fixture)
}

// dashboardOpenAPIFetchDashboard reads the Terraform resource ID from state
// and fetches the corresponding API representation. Errors intentionally omit
// response bodies, headers, and request metadata.
func dashboardOpenAPIFetchDashboard(
	ctx context.Context,
	client *dashboardservice.DashboardServiceAPIService,
	state *terraform.State,
	resourceName string,
	fixture string,
) (*dashboardservice.Dashboard, error) {
	id, err := dashboardOpenAPIResourceID(state, resourceName, fixture)
	if err != nil {
		return nil, err
	}
	return dashboardOpenAPIFetchDashboardByID(ctx, client, id, fixture)
}

func dashboardOpenAPIFetchDashboardByID(
	ctx context.Context,
	client *dashboardservice.DashboardServiceAPIService,
	id string,
	fixture string,
) (*dashboardservice.Dashboard, error) {
	if client == nil {
		return nil, fmt.Errorf("dashboard fixture %q (dashboard %q): OpenAPI client is nil", fixture, id)
	}

	response, httpResponse, requestErr := client.DashboardsServiceGetDashboard(ctx, id).Execute()
	if requestErr != nil {
		return nil, dashboardOpenAPISafeRequestError("fetch", fixture, id, httpResponse, requestErr)
	}
	if response == nil || response.Dashboard == nil {
		return nil, fmt.Errorf("dashboard fixture %q (dashboard %q): fetch returned no dashboard", fixture, id)
	}

	return response.Dashboard, nil
}

func dashboardOpenAPIImportDashboardCheck(
	ctx context.Context,
	client **dashboardservice.DashboardServiceAPIService,
	fixture string,
	assert func(*dashboardservice.Dashboard) error,
) resource.ImportStateCheckFunc {
	return func(states []*terraform.InstanceState) error {
		for _, state := range states {
			if state == nil || state.ID == "" || state.Ephemeral.Type != strings.SplitN(dashboardResourceName, ".", 2)[0] {
				continue
			}
			dashboard, err := dashboardOpenAPIFetchDashboardByID(ctx, *client, state.ID, fixture)
			if err != nil {
				return err
			}
			return assert(dashboard)
		}
		return fmt.Errorf("dashboard fixture %q import: dashboard state is absent", fixture)
	}
}

func dashboardOpenAPIComposeImportStateChecks(checks ...resource.ImportStateCheckFunc) resource.ImportStateCheckFunc {
	return func(states []*terraform.InstanceState) error {
		for _, check := range checks {
			if err := check(states); err != nil {
				return err
			}
		}
		return nil
	}
}

// dashboardOpenAPICheckDashboard adapts a fetched-dashboard assertion to a
// resource.TestCheckFunc. The callback should report model/branch details for
// branch-specific assertions by using dashboardOpenAPIAssertOneOfBranch.
func dashboardOpenAPICheckDashboard(
	ctx context.Context,
	client *dashboardservice.DashboardServiceAPIService,
	resourceName string,
	fixture string,
	check func(*dashboardservice.Dashboard) error,
) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		dashboard, err := dashboardOpenAPIFetchDashboard(ctx, client, state, resourceName, fixture)
		if err != nil {
			return err
		}
		if check == nil {
			return fmt.Errorf("dashboard fixture %q (dashboard %q): check callback is nil", fixture, dashboard.GetId())
		}

		return check(dashboard)
	}
}

// dashboardOpenAPIAssertOneOfBranch verifies both sides of a generated union:
// the expected branch must be populated and every sibling in the same protobuf
// oneof must be nil. The generated Dashboard carrier is special because it
// combines the auto_refresh and time_frame protobuf oneofs.
func dashboardOpenAPIAssertOneOfBranch(carrier any, model, expectedBranch, dashboardID, fixture string) error {
	siblings, providerPath, err := dashboardOpenAPIOneOfSiblings(model, expectedBranch)
	if err != nil {
		return dashboardOpenAPIOneOfError(fixture, dashboardID, model, expectedBranch, providerPath, nil, err.Error())
	}

	value := reflect.ValueOf(carrier)
	if !value.IsValid() {
		return dashboardOpenAPIOneOfError(fixture, dashboardID, model, expectedBranch, providerPath, nil, "carrier is nil")
	}
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return dashboardOpenAPIOneOfError(fixture, dashboardID, model, expectedBranch, providerPath, nil, "carrier is nil")
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return dashboardOpenAPIOneOfError(fixture, dashboardID, model, expectedBranch, providerPath, nil, fmt.Sprintf("carrier has type %T, want struct or pointer to struct", carrier))
	}
	if value.Type().Name() != model {
		return dashboardOpenAPIOneOfError(fixture, dashboardID, model, expectedBranch, providerPath, nil, fmt.Sprintf("carrier has generated model name %q", value.Type().Name()))
	}

	branches := append([]string{expectedBranch}, siblings...)
	populated := make([]string, 0, len(branches))
	for _, branch := range branches {
		field, ok := dashboardOpenAPIJSONField(value, branch)
		if !ok {
			return dashboardOpenAPIOneOfError(
				fixture,
				dashboardID,
				model,
				expectedBranch,
				providerPath,
				dashboardOpenAPIPopulatedSiblings(populated, expectedBranch),
				fmt.Sprintf("generated carrier has no %q field", branch),
			)
		}
		if !dashboardOpenAPIIsNil(field) {
			populated = append(populated, branch)
		}
	}
	sort.Strings(populated)

	expectedPopulated := false
	var populatedSiblings []string
	for _, branch := range populated {
		if branch == expectedBranch {
			expectedPopulated = true
		} else {
			populatedSiblings = append(populatedSiblings, branch)
		}
	}
	if !expectedPopulated || len(populatedSiblings) != 0 || len(populated) != 1 {
		detail := "expected branch is nil"
		if expectedPopulated {
			detail = "one or more sibling branches are populated"
		}
		return dashboardOpenAPIOneOfError(fixture, dashboardID, model, expectedBranch, providerPath, populatedSiblings, detail)
	}

	return nil
}

func dashboardOpenAPIPopulatedSiblings(populated []string, expectedBranch string) []string {
	siblings := make([]string, 0, len(populated))
	for _, branch := range populated {
		if branch != expectedBranch {
			siblings = append(siblings, branch)
		}
	}
	sort.Strings(siblings)
	return siblings
}

func dashboardOpenAPIOneOfSiblings(model, expectedBranch string) ([]string, string, error) {
	modelCoverage, ok := dashboardOpenAPIOneOfCoverage[model]
	if !ok {
		return nil, "", fmt.Errorf("model is absent from the dashboard oneof coverage manifest")
	}
	branchCoverage, ok := modelCoverage.Branches[expectedBranch]
	if !ok {
		return nil, "", fmt.Errorf("branch is absent from the dashboard oneof coverage manifest")
	}

	branches := make([]string, 0, len(modelCoverage.Branches)-1)
	for branch := range modelCoverage.Branches {
		if branch != expectedBranch {
			branches = append(branches, branch)
		}
	}
	if model == "Dashboard" {
		autoRefresh := map[string]struct{}{
			"off": {}, "oneMinute": {}, "twoMinutes": {}, "fiveMinutes": {}, "fifteenMinutes": {},
		}
		_, expectedIsAutoRefresh := autoRefresh[expectedBranch]
		branches = branches[:0]
		for branch := range modelCoverage.Branches {
			_, isAutoRefresh := autoRefresh[branch]
			if branch != expectedBranch && isAutoRefresh == expectedIsAutoRefresh {
				branches = append(branches, branch)
			}
		}
	}
	sort.Strings(branches)

	return branches, branchCoverage.ProviderPath, nil
}

func dashboardOpenAPIJSONField(value reflect.Value, name string) (reflect.Value, bool) {
	for i := 0; i < value.NumField(); i++ {
		field := value.Type().Field(i)
		jsonName := strings.Split(field.Tag.Get("json"), ",")[0]
		if jsonName == name {
			return value.Field(i), true
		}
	}

	return reflect.Value{}, false
}

func dashboardOpenAPIIsNil(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func dashboardOpenAPIOneOfError(fixture, dashboardID, model, expectedBranch, providerPath string, populatedSiblings []string, detail string) error {
	if populatedSiblings == nil {
		populatedSiblings = []string{}
	}
	return fmt.Errorf(
		"dashboard fixture %q (dashboard %q): model %s expected branch %q at provider path %q; populated sibling branches=%v: %s",
		fixture, dashboardID, model, expectedBranch, providerPath, populatedSiblings, detail,
	)
}

// dashboardOpenAPIIDTracker captures the Terraform identity after create and
// checks that later update/read steps did not replace the dashboard.
type dashboardOpenAPIIDTracker struct {
	fixture      string
	resourceName string
	id           string
}

func newDashboardOpenAPIIDTracker(resourceName, fixture string) *dashboardOpenAPIIDTracker {
	return &dashboardOpenAPIIDTracker{resourceName: resourceName, fixture: fixture}
}

func (tracker *dashboardOpenAPIIDTracker) Capture() resource.TestCheckFunc {
	return func(state *terraform.State) error {
		id, err := dashboardOpenAPIResourceID(state, tracker.resourceName, tracker.fixture)
		if err != nil {
			return err
		}
		if tracker.id != "" && tracker.id != id {
			return fmt.Errorf("dashboard fixture %q: resource ID changed while recapturing: got %q, want %q", tracker.fixture, id, tracker.id)
		}
		tracker.id = id
		return nil
	}
}

func (tracker *dashboardOpenAPIIDTracker) AssertUnchanged() resource.TestCheckFunc {
	return func(state *terraform.State) error {
		id, err := dashboardOpenAPIResourceID(state, tracker.resourceName, tracker.fixture)
		if err != nil {
			return err
		}
		if tracker.id == "" {
			return fmt.Errorf("dashboard fixture %q (dashboard %q): resource ID was not captured before update", tracker.fixture, id)
		}
		if id != tracker.id {
			return fmt.Errorf("dashboard fixture %q: resource ID changed across update: got %q, want %q", tracker.fixture, id, tracker.id)
		}
		return nil
	}
}

func dashboardOpenAPIResourceID(state *terraform.State, resourceName, fixture string) (string, error) {
	if state == nil || state.RootModule() == nil {
		return "", fmt.Errorf("dashboard fixture %q: Terraform state is nil", fixture)
	}
	resourceState, ok := state.RootModule().Resources[resourceName]
	if !ok {
		return "", fmt.Errorf("dashboard fixture %q: Terraform resource %q is absent from state", fixture, resourceName)
	}
	if resourceState.Primary == nil || resourceState.Primary.ID == "" {
		return "", fmt.Errorf("dashboard fixture %q: Terraform resource %q has no ID", fixture, resourceName)
	}

	return resourceState.Primary.ID, nil
}

// dashboardOpenAPINormalize removes values owned by the dashboard backend so
// request fixtures can be compared semantically with hydrated API models. Only
// exact "id", "createdAt", and "updatedAt" keys are removed; references such
// as folderId remain part of the comparison.
func dashboardOpenAPINormalize(value any) (any, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal dashboard comparison value of type %T: %w", value, err)
	}
	var normalized any
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		return nil, fmt.Errorf("unmarshal dashboard comparison value of type %T: %w", value, err)
	}
	dashboardOpenAPIDeleteBackendFields(normalized)
	return normalized, nil
}

func dashboardOpenAPIDeleteBackendFields(value any) {
	switch typed := value.(type) {
	case map[string]any:
		delete(typed, "id")
		delete(typed, "createdAt")
		delete(typed, "updatedAt")
		for _, child := range typed {
			dashboardOpenAPIDeleteBackendFields(child)
		}
	case []any:
		for _, child := range typed {
			dashboardOpenAPIDeleteBackendFields(child)
		}
	}
}

func dashboardOpenAPIAssertSemanticEqual(expected, actual any, dashboardID, fixture string) error {
	normalizedExpected, err := dashboardOpenAPINormalize(expected)
	if err != nil {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): normalize expected model: %w", fixture, dashboardID, err)
	}
	normalizedActual, err := dashboardOpenAPINormalize(actual)
	if err != nil {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): normalize actual model: %w", fixture, dashboardID, err)
	}
	if !reflect.DeepEqual(normalizedExpected, normalizedActual) {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): normalized dashboard models differ", fixture, dashboardID)
	}

	return nil
}

// dashboardOpenAPICreateDirectFixture registers cleanup before issuing a direct
// API create. If create partially succeeds without returning an ID, cleanup
// falls back to the request's exact, unique dashboard name.
func dashboardOpenAPICreateDirectFixture(
	t *testing.T,
	client *dashboardservice.DashboardServiceAPIService,
	fixture string,
	request dashboardservice.CreateDashboardRequestDataStructure,
) (*dashboardservice.CreateDashboardResponse, error) {
	t.Helper()
	dashboardID := ""
	dashboardOpenAPIRegisterDirectCleanupByName(t, client, fixture, request.Dashboard.Name, &dashboardID)
	if client == nil {
		return nil, fmt.Errorf("dashboard fixture %q: direct create client is nil", fixture)
	}

	response, httpResponse, err := client.DashboardsServiceCreateDashboard(context.Background()).
		CreateDashboardRequestDataStructure(request).
		Execute()
	if response != nil {
		dashboardID = response.GetDashboardId()
	}
	if err != nil {
		return nil, dashboardOpenAPISafeRequestError("direct create", fixture, dashboardID, httpResponse, err)
	}
	if dashboardID == "" {
		return nil, fmt.Errorf("dashboard fixture %q: direct create returned no dashboard ID", fixture)
	}

	return response, nil
}

// dashboardOpenAPIRegisterDirectCleanup registers cleanup before a direct API
// create. fixture must also be the exact unique dashboard name when cleanup may
// need to recover from a partial create that returned no ID.
func dashboardOpenAPIRegisterDirectCleanup(
	t *testing.T,
	client *dashboardservice.DashboardServiceAPIService,
	fixture string,
	dashboardID *string,
) {
	t.Helper()
	dashboardOpenAPIRegisterDirectCleanupByName(t, client, fixture, fixture, dashboardID)
}

func dashboardOpenAPIRegisterDirectCleanupByName(
	t *testing.T,
	client *dashboardservice.DashboardServiceAPIService,
	fixture string,
	dashboardName string,
	dashboardID *string,
) {
	t.Helper()
	t.Cleanup(func() {
		if client == nil {
			t.Errorf("dashboard fixture %q: cleanup client is nil", fixture)
			return
		}

		cleanupID := ""
		if dashboardID != nil {
			cleanupID = *dashboardID
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if cleanupID == "" {
			var err error
			cleanupID, err = dashboardOpenAPIFindDashboardIDByName(ctx, client, fixture, dashboardName)
			if err != nil {
				t.Error(err)
				return
			}
		}
		if cleanupID == "" {
			return
		}

		_, httpResponse, err := client.DashboardsServiceDeleteDashboard(ctx, cleanupID).Execute()
		if err != nil && (httpResponse == nil || httpResponse.StatusCode != http.StatusNotFound) {
			t.Error(dashboardOpenAPISafeRequestError("cleanup", fixture, cleanupID, httpResponse, err))
		}
	})
}

func dashboardOpenAPIFindDashboardIDByName(
	ctx context.Context,
	client *dashboardservice.DashboardServiceAPIService,
	fixture string,
	dashboardName string,
) (string, error) {
	if dashboardName == "" {
		return "", nil
	}
	catalog, httpResponse, err := client.DashboardCatalogServiceGetDashboardCatalog(ctx).Execute()
	if err != nil {
		return "", dashboardOpenAPISafeRequestError("cleanup catalog lookup", fixture, "unknown", httpResponse, err)
	}
	if catalog == nil {
		return "", fmt.Errorf("dashboard fixture %q: cleanup catalog lookup returned no catalog", fixture)
	}

	var matches []string
	for _, item := range catalog.GetItems() {
		if item.GetName() == dashboardName && item.GetId() != "" {
			matches = append(matches, item.GetId())
		}
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("dashboard fixture %q: cleanup found %d dashboards with exact name %q; refusing ambiguous deletion", fixture, len(matches), dashboardName)
	}
	if len(matches) == 1 {
		return matches[0], nil
	}

	return "", nil
}

func dashboardOpenAPISafeRequestError(operation, fixture, dashboardID string, response *http.Response, requestErr error) error {
	if response != nil {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): %s failed with HTTP status %d", fixture, dashboardID, operation, response.StatusCode)
	}
	return fmt.Errorf("dashboard fixture %q (dashboard %q): %s failed without an HTTP response (%T)", fixture, dashboardID, operation, requestErr)
}

// dashboardOpenAPIAcceptanceClient configures a fresh SDKv2 provider so a
// DashboardsOpenAPI client is available to direct fixture setup and
// resource.TestCheckFunc callbacks without mutating the shared test provider.
func dashboardOpenAPIAcceptanceClient(t *testing.T) *dashboardservice.DashboardServiceAPIService {
	t.Helper()
	client, err := dashboardOpenAPINewAcceptanceClient()
	if err != nil {
		t.Fatal(err)
	}

	return client
}

func dashboardOpenAPINewAcceptanceClient() (*dashboardservice.DashboardServiceAPIService, error) {
	clients, err := testAccNewClientSet()
	if err != nil {
		return nil, err
	}

	return clients.DashboardsOpenAPI(), nil
}

func TestDashboardOpenAPIAssertOneOfBranchHelper(t *testing.T) {
	carrier := dashboardservice.GaugeQuery{Metrics: &dashboardservice.GaugeMetricsQuery{}}
	if err := dashboardOpenAPIAssertOneOfBranch(&carrier, "GaugeQuery", "metrics", "dashboard-id", "fixture-name"); err != nil {
		t.Fatalf("assert one populated branch: %s", err)
	}

	carrier.Logs = &dashboardservice.GaugeLogsQuery{}
	err := dashboardOpenAPIAssertOneOfBranch(&carrier, "GaugeQuery", "metrics", "dashboard-id", "fixture-name")
	if err == nil {
		t.Fatal("assert populated sibling: expected error")
	}
	for _, detail := range []string{"GaugeQuery", "metrics", "logs", "dashboard-id", "fixture-name"} {
		if !strings.Contains(err.Error(), detail) {
			t.Errorf("assert populated sibling error %q does not contain %q", err, detail)
		}
	}
}

func TestDashboardOpenAPIAssertOneOfBranchHelperMergedDashboardGroups(t *testing.T) {
	relativeTimeFrame := "PT15M"
	carrier := dashboardservice.Dashboard{
		Off:               map[string]any{},
		RelativeTimeFrame: &relativeTimeFrame,
	}
	if err := dashboardOpenAPIAssertOneOfBranch(&carrier, "Dashboard", "relativeTimeFrame", "dashboard-id", "fixture-name"); err != nil {
		t.Fatalf("assert merged Dashboard time-frame group: %s", err)
	}
	if err := dashboardOpenAPIAssertOneOfBranch(&carrier, "Dashboard", "off", "dashboard-id", "fixture-name"); err != nil {
		t.Fatalf("assert merged Dashboard auto-refresh group: %s", err)
	}
}

func TestDashboardOpenAPIAssertSemanticEqualNormalizesBackendFields(t *testing.T) {
	expected := map[string]any{
		"id":       "request-dashboard-id",
		"folderId": "folder-id",
		"widget": map[string]any{
			"id":        "request-widget-id",
			"createdAt": "request-time",
		},
	}
	actual := map[string]any{
		"id":        "backend-dashboard-id",
		"folderId":  "folder-id",
		"updatedAt": "backend-time",
		"widget": map[string]any{
			"id":        "backend-widget-id",
			"createdAt": "backend-time",
		},
	}
	if err := dashboardOpenAPIAssertSemanticEqual(expected, actual, "dashboard-id", "fixture-name"); err != nil {
		t.Fatalf("compare normalized models: %s", err)
	}

	actual["folderId"] = "different-folder-id"
	if err := dashboardOpenAPIAssertSemanticEqual(expected, actual, "dashboard-id", "fixture-name"); err == nil {
		t.Fatal("compare semantic folder ID: expected mismatch")
	}
}

func TestDashboardOpenAPIBranchFixturePreservesCoverageMetadata(t *testing.T) {
	hydratable, err := dashboardOpenAPIBranchFixtureFor("HorizontalBarChartQuery", "dataprime")
	if err != nil {
		t.Fatalf("load hydratable API-only branch: %s", err)
	}
	if hydratable.ProviderPath == "" || !hydratable.ImportHydration || !hydratable.DataSourceHydration {
		t.Fatalf("hydratable API-only metadata = %#v", hydratable)
	}

	notHydratable, err := dashboardOpenAPIBranchFixtureFor("AnnotationSource", "dataprime")
	if err != nil {
		t.Fatalf("load non-hydratable API-only branch: %s", err)
	}
	if notHydratable.ImportHydration || notHydratable.DataSourceHydration {
		t.Fatalf("non-hydratable API-only metadata = %#v", notHydratable)
	}

	covered, err := dashboardOpenAPIBranchFixtureFor("WidgetDefinition", "gauge")
	if err != nil {
		t.Fatalf("load covered branch: %s", err)
	}
	if covered.Fixture != dashboardOpenAPIBackendHydrationTestName {
		t.Fatalf("covered fixture = %q", covered.Fixture)
	}
}
