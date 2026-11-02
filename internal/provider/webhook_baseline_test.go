package provider

import (
	"bytes"
	"context"
	"regexp"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/integrations"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils/schemadump"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

const webhookGoldenDir = "testdata/webhook_baseline"
const webhookUUID = "11111111-1111-4111-8111-111111111111"
const webhookOtherUUID = "22222222-2222-4222-8222-222222222222"

// These recordings exercise the handwritten resource through the Terraform
// protocol. They are the comparison point for a later generated resource.
//
// UPDATE_GOLDEN=1 go test ./internal/provider -run TestWebhookBaseline
func TestWebhookBaselineSchema(t *testing.T) {
	var current resource.SchemaResponse
	integrations.NewWebhookResource().Schema(context.Background(), resource.SchemaRequest{}, &current)
	checkGoldenAt(t, webhookGoldenDir, "schema_current.txt", []byte(schemadump.Text(current.Schema)))
}

func newWebhookHarness(t *testing.T) *grHarness {
	return newProtocolHarness(t, "coralogix_webhook", newWebhookFake(t))
}

func webhookState(t *testing.T, h *grHarness, state any) {
	t.Helper()
	if s, ok := state.(interface{ IsNull() bool }); ok && s.IsNull() {
		t.Fatalf("webhook create returned null state:\n%s", marshalGolden(t, h.steps))
	}
}

var generatedWebhookUUID = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}`)

func webhookGoldenSteps(t *testing.T, h *grHarness) []byte {
	t.Helper()
	raw := marshalGolden(t, h.steps)
	if bytes.Contains(raw, []byte(`"companyId"`)) || bytes.Contains(raw, []byte(`"company_id"`)) {
		t.Fatal("a webhook recording contains a company ID")
	}
	raw = generatedWebhookUUID.ReplaceAllFunc(raw, func(id []byte) []byte {
		if string(id) == webhookUUID || string(id) == webhookOtherUUID || string(id[:8]) == "00000000" {
			return id
		}
		return []byte("<generated-uuid>")
	})
	for marker, replacement := range map[string]string{
		"test-only-a": "<redacted-test-a>",
		"test-only-b": "<redacted-test-b>",
		"test-only-c": "<redacted-test-c>",
	} {
		raw = bytes.ReplaceAll(raw, []byte(marker), []byte(replacement))
	}
	return raw
}

func webhookConfig(name, arm string, fields j) j {
	return j{"name": name, arm: fields}
}

var webhookFullConfigs = map[string]j{
	"custom": webhookConfig("full", "custom", j{
		"uuid": webhookUUID, "method": "post", "url": "https://example.invalid/hook",
		"headers": j{"X-Example": "value"}, "payload": `{"message":"test"}`,
	}),
	"slack": webhookConfig("full", "slack", j{
		"url":         "https://example.invalid/slack",
		"notify_on":   l{"flow_anomalies", "spike_anomalies"},
		"attachments": l{j{"type": "metric_snapshot", "active": true}, j{"type": "logs", "active": false}},
	}),
	"pager_duty":               webhookConfig("full", "pager_duty", j{"service_key": "synthetic-service-key"}),
	"sendlog":                  webhookConfig("full", "sendlog", j{"uuid": webhookUUID, "payload": `{"event":"test"}`}),
	"email_group":              webhookConfig("full", "email_group", j{"emails": l{"one@example.invalid", "two@example.invalid"}}),
	"microsoft_teams_workflow": webhookConfig("full", "microsoft_teams_workflow", j{"url": "https://example.invalid/teams"}),
	"jira": webhookConfig("full", "jira", j{
		"url": "https://example.invalid/jira", "api_token": "synthetic-jira-token",
		"email": "admin@example.invalid", "project_key": "TEST",
	}),
	"opsgenie": webhookConfig("full", "opsgenie", j{"url": "https://example.invalid/opsgenie"}),
	"demisto": webhookConfig("full", "demisto", j{
		"uuid": webhookUUID, "url": "https://example.invalid/demisto", "payload": `{"event":"test"}`,
	}),
	"event_bridge": webhookConfig("full", "event_bridge", j{
		"event_bus_arn": "arn:aws:events:eu-west-1:123456789012:event-bus/example",
		"detail":        `{"event":"test"}`, "detail_type": "test", "source": "example.test", "role_name": "test-role",
	}),
}

func TestWebhookBaselineScenarios(t *testing.T) {
	for _, arm := range []string{"custom", "slack", "pager_duty", "sendlog", "email_group", "microsoft_teams_workflow", "jira", "opsgenie", "demisto", "event_bridge"} {
		t.Run(arm, func(t *testing.T) {
			h := newWebhookHarness(t)
			config := webhookFullConfigs[arm]
			state := h.Apply("create", h.null(), config)
			webhookState(t, h, state)
			state = h.RefreshPlan("refresh and plan", state)
			state = h.Import("import", idOf(state))
			updated := with(config, j{"name": "renamed"})
			state = h.Apply("rename", state, updated)
			h.Destroy("delete", state)
			checkGoldenAt(t, webhookGoldenDir, "scenario_"+arm+".json", webhookGoldenSteps(t, h))
		})
	}
}

func TestWebhookBaselineFieldChanges(t *testing.T) {
	cases := map[string]struct {
		arm   string
		field string
		value any
	}{
		"custom-method":      {"custom", "method", "put"},
		"custom-uuid":        {"custom", "uuid", webhookOtherUUID},
		"custom-url":         {"custom", "url", "https://example.invalid/changed"},
		"custom-headers":     {"custom", "headers", j{"X-Other": "next"}},
		"custom-payload":     {"custom", "payload", `{"message":"next"}`},
		"slack-url":          {"slack", "url", "https://example.invalid/changed"},
		"slack-notify-on":    {"slack", "notify_on", l{"data_usage"}},
		"slack-attachments":  {"slack", "attachments", l{j{"type": "logs", "active": true}}},
		"slack-active":       {"slack", "attachments", l{j{"type": "metric_snapshot", "active": false}, j{"type": "logs", "active": false}}},
		"pager-duty-key":     {"pager_duty", "service_key", "synthetic-next-key"},
		"sendlog-payload":    {"sendlog", "payload", `{"event":"next"}`},
		"sendlog-uuid":       {"sendlog", "uuid", webhookOtherUUID},
		"email-addresses":    {"email_group", "emails", l{"two@example.invalid", "one@example.invalid"}},
		"teams-workflow-url": {"microsoft_teams_workflow", "url", "https://example.invalid/changed"},
		"jira-url":           {"jira", "url", "https://example.invalid/changed"},
		"jira-token":         {"jira", "api_token", "synthetic-next-token"},
		"jira-email":         {"jira", "email", "next@example.invalid"},
		"jira-project":       {"jira", "project_key", "NEXT"},
		"opsgenie-url":       {"opsgenie", "url", "https://example.invalid/changed"},
		"demisto-url":        {"demisto", "url", "https://example.invalid/changed"},
		"demisto-payload":    {"demisto", "payload", `{"event":"next"}`},
		"demisto-uuid":       {"demisto", "uuid", webhookOtherUUID},
		"event-bus-arn":      {"event_bridge", "event_bus_arn", "arn:aws:events:eu-west-1:123456789012:event-bus/next"},
		"event-detail":       {"event_bridge", "detail", `{"event":"next"}`},
		"event-detail-type":  {"event_bridge", "detail_type", "next"},
		"event-source":       {"event_bridge", "source", "example.next"},
		"event-role":         {"event_bridge", "role_name", "next-role"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := newWebhookHarness(t)
			before := webhookFullConfigs[tc.arm]
			state := h.Apply("create", h.null(), before)
			webhookState(t, h, state)
			armFields := with(before[tc.arm].(j), j{tc.field: tc.value})
			state = h.Apply("change "+tc.field, state, webhookConfig("full", tc.arm, armFields))
			h.Destroy("delete", state)
			checkGoldenAt(t, webhookGoldenDir, "field_"+name+".json", webhookGoldenSteps(t, h))
		})
	}
}

func TestWebhookBaselineOptionalFields(t *testing.T) {
	cases := map[string]struct {
		arm   string
		field string
	}{
		"custom-uuid":        {"custom", "uuid"},
		"custom-headers":     {"custom", "headers"},
		"custom-payload":     {"custom", "payload"},
		"custom-url":         {"custom", "url"},
		"slack-notify-on":    {"slack", "notify_on"},
		"slack-attachments":  {"slack", "attachments"},
		"slack-url":          {"slack", "url"},
		"pager-duty-key":     {"pager_duty", "service_key"},
		"sendlog-uuid":       {"sendlog", "uuid"},
		"sendlog-payload":    {"sendlog", "payload"},
		"email-addresses":    {"email_group", "emails"},
		"teams-workflow-url": {"microsoft_teams_workflow", "url"},
		"jira-token":         {"jira", "api_token"},
		"jira-email":         {"jira", "email"},
		"jira-project":       {"jira", "project_key"},
		"jira-url":           {"jira", "url"},
		"opsgenie-url":       {"opsgenie", "url"},
		"demisto-uuid":       {"demisto", "uuid"},
		"demisto-payload":    {"demisto", "payload"},
		"demisto-url":        {"demisto", "url"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := newWebhookHarness(t)
			before := webhookFullConfigs[tc.arm]
			fields := with(before[tc.arm].(j), nil)
			delete(fields, tc.field)
			withoutField := webhookConfig("full", tc.arm, fields)
			state := h.Apply("create with field omitted", h.null(), withoutField)
			if state.IsNull() {
				checkGoldenAt(t, webhookGoldenDir, "optional_"+name+".json", webhookGoldenSteps(t, h))
				return
			}
			state = h.RefreshPlan("refresh and plan", state)
			state = h.Apply("set field", state, before)
			state = h.Apply("remove field", state, withoutField)
			h.Destroy("delete", state)
			checkGoldenAt(t, webhookGoldenDir, "optional_"+name+".json", webhookGoldenSteps(t, h))
		})
	}
}

func TestWebhookBaselineWriteOnly(t *testing.T) {
	cases := map[string]struct {
		arm            string
		before         j
		after          j
		plainField     string
		plainValue     any
		writeOnlyField string
		versionField   string
	}{
		"custom-header": {
			arm: "custom",
			before: webhookConfig("secret", "custom", j{
				"uuid": webhookUUID, "method": "post", "url": "https://example.invalid/hook",
				"headers_wo":          j{"Authorization": "test-only-a"},
				"headers_wo_versions": j{"Authorization": float64(1)},
			}),
			after: webhookConfig("secret", "custom", j{
				"uuid": webhookUUID, "method": "post", "url": "https://example.invalid/hook",
				"headers_wo":          j{"Authorization": "test-only-b"},
				"headers_wo_versions": j{"Authorization": float64(2)},
			}),
			plainField: "headers", plainValue: j{"Authorization": "test-only-c"},
			writeOnlyField: "headers_wo", versionField: "headers_wo_versions",
		},
		"pager-duty-key": {
			arm:        "pager_duty",
			before:     webhookConfig("secret", "pager_duty", j{"service_key_wo": "test-only-a", "service_key_wo_version": float64(1)}),
			after:      webhookConfig("secret", "pager_duty", j{"service_key_wo": "test-only-b", "service_key_wo_version": float64(2)}),
			plainField: "service_key", plainValue: "test-only-c",
			writeOnlyField: "service_key_wo", versionField: "service_key_wo_version",
		},
		"jira-token": {
			arm: "jira",
			before: webhookConfig("secret", "jira", j{
				"url": "https://example.invalid/jira", "email": "admin@example.invalid", "project_key": "TEST",
				"api_token_wo": "test-only-a", "api_token_wo_version": float64(1),
			}),
			after: webhookConfig("secret", "jira", j{
				"url": "https://example.invalid/jira", "email": "admin@example.invalid", "project_key": "TEST",
				"api_token_wo": "test-only-b", "api_token_wo_version": float64(2),
			}),
			plainField: "api_token", plainValue: "test-only-c",
			writeOnlyField: "api_token_wo", versionField: "api_token_wo_version",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := newWebhookHarness(t)
			state := h.Apply("create", h.null(), tc.before)
			webhookState(t, h, state)
			state = h.RefreshPlan("refresh and plan", state)
			state = h.Apply("rotate", state, tc.after)
			plainFields := with(tc.after[tc.arm].(j), j{tc.plainField: tc.plainValue})
			delete(plainFields, tc.writeOnlyField)
			delete(plainFields, tc.versionField)
			state = h.Apply("switch to ordinary field", state, webhookConfig("secret", tc.arm, plainFields))
			state = h.RefreshPlan("refresh after switch and plan", state)
			h.Destroy("delete", state)
			checkGoldenAt(t, webhookGoldenDir, "write_only_"+name+".json", webhookGoldenSteps(t, h))
		})
	}
}

func TestWebhookBaselineEdges(t *testing.T) {
	cases := map[string]func(*grHarness){
		"missing-config": func(h *grHarness) { h.Validate("missing webhook type", j{"name": "invalid"}) },
		"multiple-configs": func(h *grHarness) {
			h.Validate("two webhook types", j{"name": "invalid", "opsgenie": j{"url": "https://example.invalid"}, "email_group": j{}})
		},
		"deprecated-teams": func(h *grHarness) {
			h.Validate("legacy Teams", webhookConfig("legacy", "microsoft_teams", j{"url": "https://example.invalid/teams"}))
		},
		"legacy-teams-create": func(h *grHarness) {
			h.Apply("create legacy Teams", h.null(), webhookConfig("legacy", "microsoft_teams", j{"url": "https://example.invalid/teams"}))
		},
		"missing-name": func(h *grHarness) {
			h.Validate("name omitted", j{"opsgenie": j{"url": "https://example.invalid/opsgenie"}})
		},
		"missing-name-create": func(h *grHarness) {
			h.Apply("create without name", h.null(), j{"opsgenie": j{"url": "https://example.invalid/opsgenie"}})
		},
		"invalid-method": func(h *grHarness) {
			h.Validate("invalid method", webhookConfig("invalid", "custom", j{"method": "patch", "url": "https://example.invalid/hook"}))
		},
		"custom-method-omitted": func(h *grHarness) {
			h.Apply("create without method", h.null(), webhookConfig("missing method", "custom", j{
				"uuid": webhookUUID, "url": "https://example.invalid/hook", "payload": "test",
			}))
		},
		"slack-active-omitted": func(h *grHarness) {
			config := webhookConfig("default active", "slack", j{
				"url": "https://example.invalid/slack", "attachments": l{j{"type": "logs"}},
			})
			state := h.Apply("create without active", h.null(), config)
			if !state.IsNull() {
				h.Destroy("delete", state)
			}
		},
		"sendlog-returned-url": func(h *grHarness) {
			state := h.Apply("create", h.null(), webhookFullConfigs["sendlog"])
			h.api.(*webhookFake).setReadURL(idOf(state), "service-value")
			state = h.RefreshPlan("refresh returned value and plan", state)
			state = h.Import("import returned value", idOf(state))
			h.Destroy("delete", state)
		},
		"duplicate-header": func(h *grHarness) {
			h.Validate("same header sent twice", webhookConfig("invalid", "custom", j{
				"method": "post", "url": "https://example.invalid/hook",
				"headers":             j{"Authorization": "synthetic-plain-value"},
				"headers_wo":          j{"authorization": "synthetic-value"},
				"headers_wo_versions": j{"authorization": float64(1)},
			}))
		},
		"missing-header-version": func(h *grHarness) {
			h.Validate("write-only header has no version", webhookConfig("invalid", "custom", j{
				"method": "post", "url": "https://example.invalid/hook",
				"headers_wo": j{"Authorization": "synthetic-value"},
			}))
		},
		"change-type": func(h *grHarness) {
			s := h.Apply("create", h.null(), webhookFullConfigs["opsgenie"])
			h.Apply("change webhook type", s, webhookFullConfigs["email_group"])
			h.Destroy("delete", s)
		},
		"empty-collections": func(h *grHarness) {
			s := h.Apply("create", h.null(), webhookFullConfigs["slack"])
			empty := webhookConfig("full", "slack", j{"url": "https://example.invalid/slack", "notify_on": l{}, "attachments": l{}})
			s = h.Apply("set both lists empty", s, empty)
			h.RefreshPlan("refresh and plan", s)
		},
		"duplicate-emails": func(h *grHarness) {
			config := webhookConfig("duplicates", "email_group", j{"emails": l{"one@example.invalid", "one@example.invalid"}})
			h.Apply("create with repeated address", h.null(), config)
		},
		"duplicate-attachments": func(h *grHarness) {
			config := webhookConfig("duplicates", "slack", j{
				"url":         "https://example.invalid/slack",
				"attachments": l{j{"type": "logs", "active": true}, j{"type": "logs", "active": false}},
			})
			h.Apply("create with repeated attachment type", h.null(), config)
		},
		"import": func(h *grHarness) {
			id := h.api.(*webhookFake).seed(j{"name": "outside", "type": "GENERIC", "url": "https://example.invalid/hook", "genericWebhook": j{"uuid": webhookUUID, "method": "POST", "headers": j{}, "payload": "hello"}})
			h.Import("import existing", id)
		},
		"import-missing": func(h *grHarness) { h.Import("import missing", "does-not-exist") },
		"import-legacy-teams": func(h *grHarness) {
			id := h.api.(*webhookFake).seed(j{"name": "outside", "type": "MICROSOFT_TEAMS", "url": "https://example.invalid/teams", "microsoftTeams": j{}})
			state := h.Import("import legacy Teams", id)
			h.Apply("update legacy Teams", state, webhookConfig("renamed", "microsoft_teams", j{"url": "https://example.invalid/teams"}))
		},
		"read-missing": func(h *grHarness) {
			s := h.Apply("create", h.null(), webhookFullConfigs["opsgenie"])
			h.api.(*webhookFake).forget(idOf(s))
			h.Refresh("refresh missing", s)
		},
		"read-route-missing": func(h *grHarness) {
			s := h.Apply("create", h.null(), webhookFullConfigs["opsgenie"])
			h.api.(*webhookFake).getStatus = 404
			h.api.(*webhookFake).getMessage = "route not found"
			h.Refresh("refresh when route is missing", s)
		},
		"update-missing": func(h *grHarness) {
			s := h.Apply("create", h.null(), webhookFullConfigs["opsgenie"])
			h.api.(*webhookFake).forget(idOf(s))
			h.Apply("update missing", s, with(webhookFullConfigs["opsgenie"], j{"name": "changed"}))
		},
		"delete-missing": func(h *grHarness) {
			s := h.Apply("create", h.null(), webhookFullConfigs["opsgenie"])
			h.api.(*webhookFake).forget(idOf(s))
			h.api.(*webhookFake).deleteStatus = 404
			h.Destroy("delete missing", s)
		},
		"create-read-error": func(h *grHarness) {
			h.api.(*webhookFake).getStatus = 500
			h.Apply("create then read error", h.null(), webhookFullConfigs["opsgenie"])
		},
		"create-error": func(h *grHarness) {
			h.api.(*webhookFake).createStatus = 400
			h.Apply("create rejected", h.null(), webhookFullConfigs["opsgenie"])
		},
		"update-error": func(h *grHarness) {
			s := h.Apply("create", h.null(), webhookFullConfigs["opsgenie"])
			h.api.(*webhookFake).updateStatus = 400
			h.Apply("update rejected", s, with(webhookFullConfigs["opsgenie"], j{"name": "changed"}))
		},
		"update-read-error": func(h *grHarness) {
			s := h.Apply("create", h.null(), webhookFullConfigs["opsgenie"])
			h.api.(*webhookFake).getStatus = 500
			h.Apply("update then read error", s, with(webhookFullConfigs["opsgenie"], j{"name": "changed"}))
		},
		"read-error": func(h *grHarness) {
			s := h.Apply("create", h.null(), webhookFullConfigs["opsgenie"])
			h.api.(*webhookFake).getStatus = 500
			h.Refresh("refresh error", s)
		},
		"delete-error": func(h *grHarness) {
			s := h.Apply("create", h.null(), webhookFullConfigs["opsgenie"])
			h.api.(*webhookFake).deleteStatus = 500
			h.Destroy("delete error", s)
		},
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			h := newWebhookHarness(t)
			run(h)
			checkGoldenAt(t, webhookGoldenDir, "edge_"+name+".json", webhookGoldenSteps(t, h))
		})
	}
}
