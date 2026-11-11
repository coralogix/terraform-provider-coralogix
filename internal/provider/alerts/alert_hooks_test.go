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
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	alertapi "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/alert_definitions_service"
	alertschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/alerts/alert_schema"
	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/alert"
)

func scheduleConfigValue(ctx context.Context, days []string, start, end, offset string) tftypes.Value {
	scheduleType := attributeType(alert.Schema().Type().TerraformType(ctx), "schedule")
	activeOnType := attributeType(scheduleType, "active_on")
	dayValues := make([]tftypes.Value, len(days))
	for i, d := range days {
		dayValues[i] = stringValue(d)
	}
	set := map[string]tftypes.Value{
		"days_of_week": tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, dayValues),
		"start_time":   stringValue(start),
		"end_time":     stringValue(end),
	}
	if offset != "" {
		set["utc_offset"] = stringValue(offset)
	}
	return nullObject(scheduleType, map[string]tftypes.Value{"active_on": nullObject(activeOnType, set)})
}

// The API holds the schedule in UTC. The configured offset moves the time, and the days of the week with it.
func TestScheduleConversion(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name      string
		days      []string
		start     string
		end       string
		offset    string
		wantDays  []alertapi.DayOfWeek
		wantStart [2]int32
		wantEnd   [2]int32
	}{
		{"default offset is UTC", []string{"Monday"}, "08:00", "17:30", "", []alertapi.DayOfWeek{alertapi.DAYOFWEEK_DAY_OF_WEEK_MONDAY_OR_UNSPECIFIED}, [2]int32{8, 0}, [2]int32{17, 30}},
		{"positive offset moves the start into the previous day", []string{"Monday"}, "01:00", "05:00", "+0200", []alertapi.DayOfWeek{alertapi.DAYOFWEEK_DAY_OF_WEEK_SUNDAY}, [2]int32{23, 0}, [2]int32{3, 0}},
		{"negative offset moves the start into the next day", []string{"Sunday"}, "22:00", "23:00", "-0300", []alertapi.DayOfWeek{alertapi.DAYOFWEEK_DAY_OF_WEEK_MONDAY_OR_UNSPECIFIED}, [2]int32{1, 0}, [2]int32{2, 0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := alertConfig(ctx, map[string]tftypes.Value{"schedule": scheduleConfigValue(ctx, tc.days, tc.start, tc.end, tc.offset)})
			body := &alertapi.CreateAlertDefinitionRequest{AlertDefProperties: &alertapi.AlertDefProperties{}}
			if diags := beforeWriteAlert(ctx, config, body); diags.HasError() {
				t.Fatalf("beforeWriteAlert: %v", diags)
			}
			window := body.AlertDefProperties.ActiveOn
			if window == nil {
				t.Fatal("the request has no schedule")
			}
			if !reflect.DeepEqual(window.DayOfWeek, tc.wantDays) {
				t.Errorf("days = %v, want %v", window.DayOfWeek, tc.wantDays)
			}
			if got := [2]int32{*window.StartTime.Hours, *window.StartTime.Minutes}; got != tc.wantStart {
				t.Errorf("start = %v, want %v", got, tc.wantStart)
			}
			if got := [2]int32{*window.EndTime.Hours, *window.EndTime.Minutes}; got != tc.wantEnd {
				t.Errorf("end = %v, want %v", got, tc.wantEnd)
			}

			// Reading the window with the same offset gives the configured schedule back.
			offset := tc.offset
			if offset == "" {
				offset = defaultScheduleOffset
			}
			schedule, diags := flattenActiveOn(window, offset)
			if diags.HasError() {
				t.Fatalf("flattenActiveOn: %v", diags)
			}
			got, ok, _ := readActiveOn(schedule)
			if !ok || got.startTime != tc.start || got.endTime != tc.end || got.utcOffset != offset || len(got.daysOfWeek) != 1 || got.daysOfWeek[0] != tc.days[0] {
				t.Errorf("round trip = %+v, want %v %s-%s %s", got, tc.days, tc.start, tc.end, offset)
			}
		})
	}
}

func TestBeforeWriteWithoutScheduleSendsNone(t *testing.T) {
	ctx := context.Background()
	body := &alertapi.ReplaceAlertDefinitionRequest{AlertDefProperties: &alertapi.AlertDefProperties{ActiveOn: &alertapi.ActivitySchedule{}}}
	if diags := beforeWriteAlert(ctx, alertConfig(ctx, nil), body); diags.HasError() {
		t.Fatalf("beforeWriteAlert: %v", diags)
	}
	if body.AlertDefProperties.ActiveOn != nil {
		t.Errorf("schedule = %v, want none", body.AlertDefProperties.ActiveOn)
	}
	if body.AccessPolicy != nil {
		t.Errorf("access policy action = %v, want none: the configuration leaves the policy unmanaged", body.AccessPolicy)
	}
}

func TestBeforeWriteAccessPolicy(t *testing.T) {
	ctx := context.Background()
	config := func(policy string) tfsdk.Config {
		return alertConfig(ctx, map[string]tftypes.Value{"access_policy": stringValue(policy)})
	}
	create := &alertapi.CreateAlertDefinitionRequest{AlertDefProperties: &alertapi.AlertDefProperties{}}
	if diags := beforeWriteAlert(ctx, config(`{"a":1}`), create); diags.HasError() || create.AccessPolicy == nil || *create.AccessPolicy != `{"a":1}` {
		t.Errorf("create policy = %v, %v", create.AccessPolicy, diags)
	}
	emptyCreate := &alertapi.CreateAlertDefinitionRequest{AlertDefProperties: &alertapi.AlertDefProperties{}}
	if diags := beforeWriteAlert(ctx, config(""), emptyCreate); diags.HasError() || emptyCreate.AccessPolicy != nil {
		t.Errorf("create with an empty policy sends %v, %v, want none", emptyCreate.AccessPolicy, diags)
	}
	update := &alertapi.ReplaceAlertDefinitionRequest{AlertDefProperties: &alertapi.AlertDefProperties{}}
	if diags := beforeWriteAlert(ctx, config(`{"a":1}`), update); diags.HasError() || update.AccessPolicy == nil || update.AccessPolicy.Upsert == nil {
		t.Errorf("update action = %v, %v, want an upsert", update.AccessPolicy, diags)
	}
	deleted := &alertapi.ReplaceAlertDefinitionRequest{AlertDefProperties: &alertapi.AlertDefProperties{}}
	if diags := beforeWriteAlert(ctx, config(""), deleted); diags.HasError() || deleted.AccessPolicy == nil || deleted.AccessPolicy.Delete == nil {
		t.Errorf("update action = %v, %v, want a delete", deleted.AccessPolicy, diags)
	}
	if diags := beforeWriteAlert(ctx, alertConfig(ctx, nil), struct{}{}); !diags.HasError() {
		t.Error("an unexpected request body is no error")
	}
}

func TestBeforeFlattenReadsCaseSettingDefaults(t *testing.T) {
	unspecifiedMode := alertapi.ALERTDEFCASEAUTORESOLVEMODE_ALERT_DEF_CASE_AUTO_RESOLVE_MODE_UNSPECIFIED
	disabled := alertapi.ALERTDEFCASEAUTORESOLVEMODE_ALERT_DEF_CASE_AUTO_RESOLVE_MODE_DISABLED
	unspecifiedType := alertapi.ALERTDEFCASEENRICHMENTQUERYTYPE_ALERT_DEF_CASE_ENRICHMENT_QUERY_TYPE_UNSPECIFIED
	for name, mode := range map[string]*alertapi.AlertDefCaseAutoResolveMode{"missing": nil, "unspecified": &unspecifiedMode} {
		settings := &alertapi.AlertDefCaseSettings{
			AutoResolveMode:   mode,
			EnrichmentQueries: []alertapi.AlertDefCaseEnrichmentQuery{{Query: "q", Type: &unspecifiedType}, {Query: "r"}},
		}
		v := &alertapi.AlertDef{AlertDefProperties: &alertapi.AlertDefProperties{CaseSettings: settings}}
		if diags := beforeFlattenAlert(context.Background(), v); diags.HasError() {
			t.Fatal(diags)
		}
		if *settings.AutoResolveMode != alertapi.ALERTDEFCASEAUTORESOLVEMODE_ALERT_DEF_CASE_AUTO_RESOLVE_MODE_ENABLED {
			t.Errorf("%s: auto resolve mode = %v, want enabled", name, *settings.AutoResolveMode)
		}
		for _, query := range settings.EnrichmentQueries {
			if query.Type == nil || *query.Type != alertapi.ALERTDEFCASEENRICHMENTQUERYTYPE_ALERT_DEF_CASE_ENRICHMENT_QUERY_TYPE_DATAPRIME {
				t.Errorf("%s: query type = %v, want dataprime", name, query.Type)
			}
		}
	}
	settings := &alertapi.AlertDefCaseSettings{AutoResolveMode: &disabled}
	v := &alertapi.AlertDef{AlertDefProperties: &alertapi.AlertDefProperties{CaseSettings: settings}}
	_ = beforeFlattenAlert(context.Background(), v)
	if *settings.AutoResolveMode != disabled {
		t.Errorf("a disabled auto resolve mode reads as %v", *settings.AutoResolveMode)
	}
}

func TestFillReleasedReads(t *testing.T) {
	v := &alertapi.AlertDef{AlertDefProperties: &alertapi.AlertDefProperties{
		LogsThreshold:             &alertapi.LogsThresholdType{LogsFilter: &alertapi.V3LogsFilter{SimpleFilter: &alertapi.LogsSimpleFilter{}}},
		MetricThreshold:           &alertapi.MetricThresholdType{},
		AnalyticsImmediate:        &alertapi.AnalyticsImmediateType{},
		LogsRatioThreshold:        &alertapi.LogsRatioThresholdType{Numerator: &alertapi.V3LogsFilter{SimpleFilter: &alertapi.LogsSimpleFilter{}}},
		LogsTimeRelativeThreshold: &alertapi.LogsTimeRelativeThresholdType{},
	}}
	if diags := beforeFlattenAlert(context.Background(), v); diags.HasError() {
		t.Fatal(diags)
	}
	p := v.AlertDefProperties
	if q := p.LogsThreshold.LogsFilter.SimpleFilter.LuceneQuery; q == nil || *q != "" {
		t.Errorf("a missing lucene query reads as %v, want an empty string", q)
	}
	if q := p.LogsRatioThreshold.Numerator.SimpleFilter.LuceneQuery; q == nil || *q != "" {
		t.Errorf("a missing lucene query of a ratio filter reads as %v, want an empty string", q)
	}
	for name, settings := range map[string]*alertapi.V3UndetectedValuesManagement{
		"logs threshold": p.LogsThreshold.UndetectedValuesManagement, "metric threshold": p.MetricThreshold.UndetectedValuesManagement,
		"logs ratio": p.LogsRatioThreshold.UndetectedValuesManagement, "logs time relative": p.LogsTimeRelativeThreshold.UndetectedValuesManagement,
	} {
		if settings == nil || *settings.TriggerUndetectedValues || *settings.AutoRetireTimeframe != alertapi.V3AUTORETIRETIMEFRAME_AUTO_RETIRE_TIMEFRAME_NEVER_OR_UNSPECIFIED {
			t.Errorf("%s: undetected values = %+v, want the defaults", name, settings)
		}
	}
	if p.LogsThreshold.NoDataPolicy == nil || p.MetricThreshold.NoDataPolicy == nil {
		t.Error("a missing no-data policy of a logs or metric threshold alert does not read as an empty policy")
	}
	if p.AnalyticsImmediate.NoDataPolicy != nil {
		t.Error("a missing no-data policy of an analytics alert must stay missing")
	}
}

func TestEmptyGroupByForm(t *testing.T) {
	empty := types.ListValueMust(types.StringType, []attr.Value{})
	grouped := types.ListValueMust(types.StringType, []attr.Value{types.StringValue("a")})
	alertOf := func(p alertapi.AlertDefProperties) *alertapi.AlertDef {
		return &alertapi.AlertDef{AlertDefProperties: &p}
	}
	cases := map[string]struct {
		in   types.List
		v    *alertapi.AlertDef
		want types.List
	}{
		"empty on a logs alert is null":      {empty, alertOf(alertapi.AlertDefProperties{LogsThreshold: &alertapi.LogsThresholdType{}}), types.ListNull(types.StringType)},
		"empty on a flow alert stays empty":  {types.ListNull(types.StringType), alertOf(alertapi.AlertDefProperties{Flow: &alertapi.FlowType{}}), empty},
		"empty on an slo alert stays empty":  {empty, alertOf(alertapi.AlertDefProperties{SloThreshold: &alertapi.SloThresholdType{}}), empty},
		"empty on a tracing threshold is []": {empty, alertOf(alertapi.AlertDefProperties{TracingThreshold: &alertapi.TracingThresholdType{}}), empty},
		"keys are kept":                      {grouped, alertOf(alertapi.AlertDefProperties{LogsThreshold: &alertapi.LogsThresholdType{}}), grouped},
	}
	for name, tc := range cases {
		if got := emptyGroupByForm(tc.in, tc.v); !got.Equal(tc.want) {
			t.Errorf("%s: %s, want %s", name, got, tc.want)
		}
	}
}

// The case settings arrived with schema version 3: the older schemas stay frozen, so stored state decodes.
func TestCaseSettingsSchemaVersions(t *testing.T) {
	if _, ok := alertschema.V1().Attributes["case_settings"]; ok {
		t.Error("V1 schema must not contain case_settings")
	}
	if _, ok := alertschema.V2().Attributes["case_settings"]; ok {
		t.Error("V2 schema must not contain case_settings")
	}
	attribute, ok := alert.Schema().Attributes["case_settings"]
	if !ok {
		t.Fatal("the schema is missing case_settings")
	}
	if attribute.IsRequired() || attribute.IsComputed() {
		t.Error("case_settings must be plain Optional so removing the block clears it")
	}
	if diags := alert.Schema().ValidateImplementation(context.Background()); diags.HasError() {
		t.Fatalf("the schema is invalid: %v", diags)
	}
}

func TestGetAlertRetriesWithoutAccessPermissionsOnForbidden(t *testing.T) {
	var queries []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("include_access_permissions") == "true" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"code":403,"message":"forbidden"}`))
			return
		}
		_, _ = w.Write([]byte(`{"alertDef":{"id":"3dd36c34-2b3c-4d7d-8b0c-1f6b0c1c0f11"}}`))
	}))
	defer server.Close()
	cfg := alertapi.NewConfiguration()
	cfg.Servers = alertapi.ServerConfigurations{{URL: server.URL}}
	client := alertapi.NewAPIClient(cfg).AlertDefinitionsServiceAPI

	id := "3dd36c34-2b3c-4d7d-8b0c-1f6b0c1c0f11"
	out, _, err := getAlert(context.Background(), func() alertapi.ApiAlertDefsServiceGetAlertDefRequest {
		return client.AlertDefsServiceGetAlertDef(context.Background(), id)
	})
	if err != nil || out == nil || out.AlertDef == nil {
		t.Fatalf("getAlert = %v, %v", out, err)
	}
	if len(queries) != 2 || queries[0] != "include_access_permissions=true" || queries[1] != "" {
		t.Errorf("queries = %q, want the flag first and a retry without it", queries)
	}
	if _, readable := accessPolicyFromGetResponse(out); readable {
		t.Error("a response without access must not be readable")
	}
}

func TestWithFallbackPriority(t *testing.T) {
	overrideType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"priority": tftypes.String}}
	ruleType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"name": tftypes.String, "override": overrideType}}
	rule := func(name string, priority tftypes.Value) tftypes.Value {
		return tftypes.NewValue(ruleType, map[string]tftypes.Value{
			"name": stringValue(name), "override": tftypes.NewValue(overrideType, map[string]tftypes.Value{"priority": priority}),
		})
	}
	unknown := tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	null := tftypes.NewValue(tftypes.String, nil)
	rulesType := tftypes.Set{ElementType: ruleType}
	configType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"type_definition": tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"logs_threshold": tftypes.Object{AttributeTypes: map[string]tftypes.Type{"rules": rulesType}},
	}}}}
	config := func(rules ...tftypes.Value) tftypes.Value {
		return tftypes.NewValue(configType, map[string]tftypes.Value{"type_definition": tftypes.NewValue(configType.AttributeTypes["type_definition"], map[string]tftypes.Value{
			"logs_threshold": tftypes.NewValue(configType.AttributeTypes["type_definition"].(tftypes.Object).AttributeTypes["logs_threshold"], map[string]tftypes.Value{
				"rules": tftypes.NewValue(rulesType, rules),
			}),
		})})
	}
	path := tftypes.NewAttributePath().WithAttributeName("type_definition").WithAttributeName("logs_threshold").WithAttributeName("rules").WithElementKeyValue(rule("a", unknown))
	fallback := stringValue("P2")

	// The configuration has two rules: the first leaves the priority out, the second sets it.
	cfg := config(rule("a", null), rule("b", stringValue("P1")))
	got := withFallbackPriority(cfg, path, rule("a", unknown), fallback)
	if !got.Equal(rule("a", fallback)) {
		t.Errorf("an unconfigured priority plans %v, want P2", got)
	}
	pathB := tftypes.NewAttributePath().WithAttributeName("type_definition").WithAttributeName("logs_threshold").WithAttributeName("rules").WithElementKeyValue(rule("b", unknown))
	if got := withFallbackPriority(cfg, pathB, rule("b", stringValue("P1")), fallback); !got.Equal(rule("b", stringValue("P1"))) {
		t.Errorf("a configured priority plans %v, want P1", got)
	}
	// A known value of the prior state is replaced too: the priority of the alert is the plan.
	if got := withFallbackPriority(cfg, path, rule("a", stringValue("P5")), fallback); !got.Equal(rule("a", fallback)) {
		t.Errorf("an unconfigured priority with a prior value plans %v, want P2", got)
	}
	// A priority that the configuration does not know yet stays unknown.
	unknownCfg := config(rule("a", unknown))
	if got := withFallbackPriority(unknownCfg, path, rule("a", unknown), fallback); !got.Equal(rule("a", unknown)) {
		t.Errorf("an unknown configured priority plans %v, want unknown", got)
	}
	if !isRuleItem(path) || isRuleItem(tftypes.NewAttributePath().WithAttributeName("priority")) {
		t.Error("isRuleItem does not tell a rule from another path")
	}
}

// stateWithDestinations returns a state of the generated schema with one destination for each period.
func stateWithDestinations(ctx context.Context, periods ...tftypes.Value) tfsdk.State {
	s := alert.Schema()
	groupType := attributeType(s.Type().TerraformType(ctx), "notification_group")
	destinationsType := attributeType(groupType, "destinations")
	destinations := make([]tftypes.Value, len(periods))
	for i, period := range periods {
		destinations[i] = nullObject(destinationsType.(tftypes.List).ElementType, map[string]tftypes.Value{
			"connector_id": stringValue("c"), "preset_id": stringValue("p"), "retriggering_period_minutes": period,
		})
	}
	group := nullObject(groupType, map[string]tftypes.Value{"destinations": tftypes.NewValue(destinationsType, destinations)})
	return tfsdk.State{Schema: s, Raw: nullObject(s.Type().TerraformType(ctx), map[string]tftypes.Value{"notification_group": group})}
}

func destinationPeriods(t *testing.T, state tfsdk.State) []types.Int64 {
	t.Helper()
	var destinations types.List
	if diags := state.GetAttribute(context.Background(), destinationsPath, &destinations); diags.HasError() {
		t.Fatal(diags)
	}
	var out []types.Int64
	for _, element := range destinations.Elements() {
		out = append(out, element.(types.Object).Attributes()["retriggering_period_minutes"].(types.Int64))
	}
	return out
}

// The API gives a destination without a period the cadence of the incident. The state keeps the null
// that the plan or the prior state has.
func TestAfterReadKeepsUnconfiguredDestinationPeriodsNull(t *testing.T) {
	ctx := context.Background()
	number := func(n int64) tftypes.Value { return tftypes.NewValue(tftypes.Number, n) }
	null := tftypes.NewValue(tftypes.Number, nil)

	t.Run("an unconfigured period stays null and a configured one stays", func(t *testing.T) {
		state := stateWithDestinations(ctx, number(10), number(600), number(15))
		prior := stateWithDestinations(ctx, null, number(600))
		if diags := afterReadAlert(ctx, &state, prior); diags.HasError() {
			t.Fatal(diags)
		}
		got := destinationPeriods(t, state)
		if !got[0].IsNull() || got[1].ValueInt64() != 600 || got[2].ValueInt64() != 15 {
			t.Errorf("periods = %v, want [null 600 15]: a destination that the prior value does not have keeps the API value", got)
		}
	})
	t.Run("a prior value without destinations leaves the state alone", func(t *testing.T) {
		state := stateWithDestinations(ctx, number(10))
		if diags := afterReadAlert(ctx, &state, tfsdk.State{Schema: alert.Schema(), Raw: nullObject(alert.Schema().Type().TerraformType(ctx), nil)}); diags.HasError() {
			t.Fatal(diags)
		}
		if got := destinationPeriods(t, state); got[0].ValueInt64() != 10 {
			t.Errorf("periods = %v, want [10]", got)
		}
	})
	t.Run("a state upgrade clears every value", func(t *testing.T) {
		state := stateWithDestinations(ctx, number(10), number(600))
		if diags := afterReadAlert(ctx, &state, nil); diags.HasError() {
			t.Fatal(diags)
		}
		if got := destinationPeriods(t, state); !got[0].IsNull() || !got[1].IsNull() {
			t.Errorf("periods = %v, want all null", got)
		}
	})
}

func TestAfterGetSetsAccessPolicy(t *testing.T) {
	ctx := context.Background()
	s := alert.Schema()
	stateWith := func(policy tftypes.Value) tfsdk.State {
		return tfsdk.State{Schema: s, Raw: nullObject(s.Type().TerraformType(ctx), map[string]tftypes.Value{"access_policy": policy})}
	}
	policyOf := func(state tfsdk.State) types.String {
		var policy types.String
		if diags := state.GetAttribute(ctx, accessPolicyPath, &policy); diags.HasError() {
			t.Fatal(diags)
		}
		return policy
	}
	text := `{"version":"1","rules":[]}`
	withPolicy := &alertapi.GetAlertDefResponse{Access: &alertapi.AlertDefAccess{AccessPolicy: &text}}
	readableWithout := &alertapi.GetAlertDefResponse{Access: &alertapi.AlertDefAccess{}}
	notReadable := &alertapi.GetAlertDefResponse{}

	// Import and the data source have no prior value: the policy is stored in the shape of jsonencode.
	state := stateWith(tftypes.NewValue(tftypes.String, nil))
	if diags := afterGetAlert(ctx, &state, withPolicy); diags.HasError() || policyOf(state).ValueString() != `{"rules":[],"version":"1"}` {
		t.Errorf("import: %v, %v", policyOf(state), diags)
	}
	// A configured text that equals the policy of the backend stays as written.
	written := "{\n  \"version\": \"1\",\n  \"rules\": []\n}"
	state = stateWith(stringValue(written))
	if diags := afterGetAlert(ctx, &state, withPolicy); diags.HasError() || policyOf(state).ValueString() != written {
		t.Errorf("an equal text: %v, %v", policyOf(state), diags)
	}
	// A key that cannot read policies keeps the prior value.
	state = stateWith(stringValue(written))
	if diags := afterGetAlert(ctx, &state, notReadable); diags.HasError() || policyOf(state).ValueString() != written {
		t.Errorf("not readable: %v, %v", policyOf(state), diags)
	}
	// A readable alert without a policy clears it.
	state = stateWith(stringValue(written))
	if diags := afterGetAlert(ctx, &state, readableWithout); diags.HasError() || !policyOf(state).IsNull() {
		t.Errorf("no policy: %v, %v", policyOf(state), diags)
	}
}

// The API keeps a stored description that a Replace omits, and clears it only for an explicit "".
func TestBeforeWriteClearsAnOmittedDescriptionOnReplace(t *testing.T) {
	ctx := context.Background()
	text := "kept"
	properties := func() *alertapi.AlertDefProperties { return &alertapi.AlertDefProperties{} }
	withText := func() *alertapi.AlertDefProperties { return &alertapi.AlertDefProperties{Description: &text} }

	t.Run("create omits it", func(t *testing.T) {
		body := &alertapi.CreateAlertDefinitionRequest{AlertDefProperties: properties()}
		if diags := beforeWriteAlert(ctx, alertConfig(ctx, nil), body); diags.HasError() || body.AlertDefProperties.Description != nil {
			t.Errorf("description = %v, %v, want none", body.AlertDefProperties.Description, diags)
		}
	})
	t.Run("replace without a configured description sends an empty one", func(t *testing.T) {
		body := &alertapi.ReplaceAlertDefinitionRequest{AlertDefProperties: properties()}
		if diags := beforeWriteAlert(ctx, alertConfig(ctx, nil), body); diags.HasError() || body.AlertDefProperties.Description == nil || *body.AlertDefProperties.Description != "" {
			t.Errorf("description = %v, %v, want an empty string", body.AlertDefProperties.Description, diags)
		}
	})
	t.Run("replace keeps a configured empty description", func(t *testing.T) {
		empty := ""
		body := &alertapi.ReplaceAlertDefinitionRequest{AlertDefProperties: &alertapi.AlertDefProperties{Description: &empty}}
		config := alertConfig(ctx, map[string]tftypes.Value{"description": stringValue("")})
		if diags := beforeWriteAlert(ctx, config, body); diags.HasError() || body.AlertDefProperties.Description == nil || *body.AlertDefProperties.Description != "" {
			t.Errorf("description = %v, %v, want an empty string", body.AlertDefProperties.Description, diags)
		}
	})
	t.Run("replace keeps a configured description", func(t *testing.T) {
		body := &alertapi.ReplaceAlertDefinitionRequest{AlertDefProperties: withText()}
		config := alertConfig(ctx, map[string]tftypes.Value{"description": stringValue("kept")})
		if diags := beforeWriteAlert(ctx, config, body); diags.HasError() || *body.AlertDefProperties.Description != "kept" {
			t.Errorf("description = %v, %v, want kept", body.AlertDefProperties.Description, diags)
		}
	})
}

// The API returns "" for a cleared description and no value for an alert that never had one.
func TestAfterReadReadsAClearedDescriptionAsNull(t *testing.T) {
	ctx := context.Background()
	s := alert.Schema()
	stateWith := func(description tftypes.Value) tfsdk.State {
		return tfsdk.State{Schema: s, Raw: nullObject(s.Type().TerraformType(ctx), map[string]tftypes.Value{"description": description})}
	}
	descriptionOf := func(state tfsdk.State) types.String {
		var description types.String
		if diags := state.GetAttribute(ctx, descriptionAttrPath, &description); diags.HasError() {
			t.Fatal(diags)
		}
		return description
	}
	null := tftypes.NewValue(tftypes.String, nil)
	cases := []struct {
		name  string
		api   tftypes.Value
		prior any
		want  types.String
	}{
		{"null prior reads null", stringValue(""), stateWith(null), types.StringNull()},
		{"a configured empty prior stays empty", stringValue(""), stateWith(stringValue("")), types.StringValue("")},
		{"no prior (import, upgrade) reads null", stringValue(""), nil, types.StringNull()},
		{"a value stays", stringValue("text"), stateWith(null), types.StringValue("text")},
		{"a missing value stays null", null, stateWith(stringValue("")), types.StringNull()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := stateWith(tc.api)
			if diags := afterReadAlert(ctx, &state, tc.prior); diags.HasError() {
				t.Fatal(diags)
			}
			if got := descriptionOf(state); !got.Equal(tc.want) {
				t.Errorf("description = %v, want %v", got, tc.want)
			}
		})
	}
}

// An API that still fills the evaluation period in returns it for every analytics alert; the state keeps
// the null that the plan or the prior state has, and keeps a value that the plan sets.
func TestAfterReadKeepsAnUnconfiguredEvaluationPeriodNull(t *testing.T) {
	ctx := context.Background()
	s := alert.Schema()
	typeDefinitionType := attributeType(s.Type().TerraformType(ctx), "type_definition")
	armType := func(arm string) tftypes.Type { return attributeType(typeDefinitionType, arm) }
	// stateWith returns a state with the given periods; an absent arm is null.
	stateWith := func(immediate, threshold *tftypes.Value) tfsdk.State {
		arms := map[string]tftypes.Value{}
		for arm, period := range map[string]*tftypes.Value{"analytics_immediate": immediate, "analytics_threshold": threshold} {
			if period != nil {
				arms[arm] = nullObject(armType(arm), map[string]tftypes.Value{"evaluation_period_minutes": *period})
			}
		}
		return tfsdk.State{Schema: s, Raw: nullObject(s.Type().TerraformType(ctx), map[string]tftypes.Value{"type_definition": nullObject(typeDefinitionType, arms)})}
	}
	periodsOf := func(state tfsdk.State) (types.Int32, types.Int32) {
		var immediate, threshold types.Int32
		if diags := state.GetAttribute(ctx, evaluationPeriodPaths[0], &immediate); diags.HasError() {
			t.Fatal(diags)
		}
		if diags := state.GetAttribute(ctx, evaluationPeriodPaths[1], &threshold); diags.HasError() {
			t.Fatal(diags)
		}
		return immediate, threshold
	}
	number := func(n int64) *tftypes.Value { v := tftypes.NewValue(tftypes.Number, n); return &v }
	null := func() *tftypes.Value { v := tftypes.NewValue(tftypes.Number, nil); return &v }
	unknown := func() *tftypes.Value { v := tftypes.NewValue(tftypes.Number, tftypes.UnknownValue); return &v }

	cases := []struct {
		name                         string
		immediate, threshold         *tftypes.Value
		prior                        any
		wantImmediate, wantThreshold types.Int32
	}{
		{"a null prior reads null", number(10), nil, stateWith(null(), nil), types.Int32Null(), types.Int32Null()},
		{"a configured period stays", number(5), nil, stateWith(number(5), nil), types.Int32Value(5), types.Int32Null()},
		{"a period that the plan does not know yet stays", number(10), nil, stateWith(unknown(), nil), types.Int32Value(10), types.Int32Null()},
		{"the threshold arm is handled too", nil, number(10), stateWith(nil, null()), types.Int32Null(), types.Int32Null()},
		{"an arm that the prior value does not have reads null", number(10), nil, stateWith(nil, nil), types.Int32Null(), types.Int32Null()},
		{"an API without a period stays null", null(), nil, stateWith(null(), nil), types.Int32Null(), types.Int32Null()},
		{"an import and a state upgrade read null", number(10), number(15), nil, types.Int32Null(), types.Int32Null()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := stateWith(tc.immediate, tc.threshold)
			if diags := afterReadAlert(ctx, &state, tc.prior); diags.HasError() {
				t.Fatal(diags)
			}
			immediate, threshold := periodsOf(state)
			if !immediate.Equal(tc.wantImmediate) || !threshold.Equal(tc.wantThreshold) {
				t.Errorf("periods = %v, %v, want %v, %v", immediate, threshold, tc.wantImmediate, tc.wantThreshold)
			}
		})
	}
}
