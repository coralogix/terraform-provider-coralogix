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

package recording_rules

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// A rule group set as written by schema version 0: `group`/`rule` rather than
// `groups`/`rules`, and a rule carrying only record, expr and labels.
const recordingRuleGroupSetStateV0JSON = `{
  "id": "set-id",
  "yaml_content": null,
  "name": "set name",
  "group": [{
    "name": "Foo",
    "interval": 180,
    "limit": 100,
    "rule": [{
      "record": "job:http_requests_total:sum",
      "expr": "sum(rate(http_requests_total[5m])) by (job)",
      "labels": {"team": "backend"}
    }]
  }]
}`

// The V0 upgrader rebuilds every rule at the current type. Widening
// RecordingRuleModel without giving the upgrader its own frozen V0 struct
// breaks only at runtime during a provider upgrade, so this covers the one
// prior version the resource wires.
func TestUpgradeRecordingRuleGroupSetStateV0ToV1(t *testing.T) {
	ctx := context.Background()
	schemaV0 := recordingRuleGroupSetV0()
	currentSchema := recordingRuleGroupSetCurrentSchema(ctx, t)

	priorValue, err := tfprotov6.RawState{JSON: []byte(recordingRuleGroupSetStateV0JSON)}.
		Unmarshal(schemaV0.Type().TerraformType(ctx))
	if err != nil {
		t.Fatalf("decoding V0 state failed: %s", err)
	}

	req := resource.UpgradeStateRequest{
		State: &tfsdk.State{Raw: priorValue, Schema: schemaV0},
	}
	resp := &resource.UpgradeStateResponse{
		State: tfsdk.State{Raw: tftypes.Value{}, Schema: currentSchema},
	}

	upgradeRecordingRuleGroupSetStateV0ToV1(ctx, req, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("upgrade produced errors: %s", resp.Diagnostics)
	}

	var upgraded RecordingRuleGroupSetResourceModel
	if diags := resp.State.Get(ctx, &upgraded); diags.HasError() {
		t.Fatalf("reading upgraded state failed: %s", diags)
	}

	var groups []RecordingRuleGroupModel
	if diags := upgraded.Groups.ElementsAs(ctx, &groups, false); diags.HasError() {
		t.Fatalf("reading upgraded groups failed: %s", diags)
	}
	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}

	var rules []RecordingRuleModel
	if diags := groups[0].Rules.ElementsAs(ctx, &rules, false); diags.HasError() {
		t.Fatalf("reading upgraded rules failed: %s", diags)
	}
	if len(rules) != 1 {
		t.Fatalf("got %d rules, want 1", len(rules))
	}

	if got := rules[0].Record.ValueString(); got != "job:http_requests_total:sum" {
		t.Errorf("record = %q, want %q", got, "job:http_requests_total:sum")
	}
	// V0 state carries no delay, so the upgraded rule must report absent
	// rather than a zero the resource never had.
	if !rules[0].EvaluationDelayMs.IsNull() {
		t.Errorf("evaluation_delay_ms = %v, want null", rules[0].EvaluationDelayMs)
	}
	if got := groups[0].Limit; got.ValueInt64() != 100 {
		t.Errorf("limit = %v, want 100", got)
	}
}

// Absent attributes in stored state decode to null against the current
// schema, which is what lets a new Optional attribute ship without a version
// bump for anyone already on V1.
func TestCurrentSchemaDecodesRuleStateWithoutEvaluationDelay(t *testing.T) {
	ctx := context.Background()
	currentSchema := recordingRuleGroupSetCurrentSchema(ctx, t)

	const stateJSON = `{
      "id": "set-id",
      "yaml_content": null,
      "name": "set name",
      "groups": [{
        "name": "Foo",
        "interval": 180,
        "limit": 100,
        "rules": [{
          "record": "job:http_requests_total:sum",
          "expr": "sum(rate(http_requests_total[5m])) by (job)",
          "labels": {}
        }]
      }]
    }`

	opts := tfprotov6.UnmarshalOpts{
		ValueFromJSONOpts: tftypes.ValueFromJSONOpts{IgnoreUndefinedAttributes: true},
	}
	if _, err := (tfprotov6.RawState{JSON: []byte(stateJSON)}).
		UnmarshalWithOpts(currentSchema.Type().TerraformType(ctx), opts); err != nil {
		t.Fatalf("decoding V1 state without evaluation_delay_ms failed: %s", err)
	}
}

// The rule attr.Type map is the element type for the `rules` list and is
// consumed by the flatten and by the state upgrader, so it has to stay in step
// with the schema. A missing entry surfaces as a value conversion error at
// apply time, not as a compile error.
func TestRecordingRuleAttributesMatchSchema(t *testing.T) {
	attrTypes := recordingRuleAttributes()
	for name := range recordingRulesSchema().Attributes {
		if _, ok := attrTypes[name]; !ok {
			t.Errorf("attribute %q is in recordingRulesSchema() but missing from recordingRuleAttributes()", name)
		}
	}
	for name := range attrTypes {
		if _, ok := recordingRulesSchema().Attributes[name]; !ok {
			t.Errorf("attribute %q is in recordingRuleAttributes() but missing from recordingRulesSchema()", name)
		}
	}
}

// yaml_content is decoded through mirror structs whose yaml tags carry the
// API's camelCase field names, so the only spelling that binds is
// `evaluationDelayMs` — the same key the API documents. yaml.v3 matches tags
// case-sensitively and drops unrecognized keys silently, so lowercase,
// snake_case and other casings send no value with no diagnostic. This locks
// the contract documented in the yaml_content description; if the mirror tag
// or the SDK field changes, this test fails loudly instead of the docs going
// quietly wrong.
func TestExpandRecordingRulesGroupsSetFromYamlEvaluationDelayKey(t *testing.T) {
	yamlFor := func(key string) string {
		return fmt.Sprintf(`name: Example
groups:
  - name: Foo
    interval: 180
    rules:
      - record: job:http_requests_total:sum
        expr: sum(rate(http_requests_total[5m])) by (job)
        %s: 60000
`, key)
	}

	for _, tc := range []struct {
		key       string
		wantBound bool
	}{
		{"evaluationDelayMs", true},
		{"evaluationdelayms", false},
		{"evaluation_delay_ms", false},
		{"EvaluationDelayMs", false},
		{"EVALUATIONDELAYMS", false},
	} {
		t.Run(tc.key, func(t *testing.T) {
			result, diags := expandRecordingRulesGroupsSetFromYaml(yamlFor(tc.key), "")
			if diags.HasError() {
				t.Fatalf("unmarshal failed: %s", diags)
			}
			if len(result.Groups) != 1 || len(result.Groups[0].Rules) != 1 {
				t.Fatalf("got %d groups / %d rules, want 1/1", len(result.Groups), len(result.Groups[0].Rules))
			}

			got, ok := result.Groups[0].Rules[0].GetEvaluationDelayMsOk()
			if tc.wantBound {
				if !ok || got == nil {
					t.Fatalf("key %q: expected evaluationDelayMs to bind, but it was absent", tc.key)
				}
				if *got != 60000 {
					t.Errorf("key %q: evaluationDelayMs = %d, want 60000", tc.key, *got)
				}
			} else if ok {
				t.Errorf("key %q: expected the key to be dropped silently, but evaluationDelayMs bound to %d", tc.key, *got)
			}
		})
	}
}

// The mirror structs that let yaml_content use the API's camelCase spelling
// must not regress any existing field. In particular the SDK's InRuleGroup.Limit
// is a *string, and yaml.v3 coerces an unquoted numeric `limit` into it — a
// behavior a naive YAML->JSON round-trip would break. This parses one fully
// populated set and asserts every field, unquoted limit included, maps through.
func TestExpandRecordingRulesGroupsSetFromYamlPreservesAllFields(t *testing.T) {
	const y = `name: from-yaml
groups:
  - name: Foo
    interval: 180
    limit: 100
    rules:
      - record: job:http_requests_total:sum
        expr: sum(rate(http_requests_total[5m])) by (job)
        labels:
          team: backend
        evaluationDelayMs: 60000
      - record: ts3db_live_ingester_write_latency:3m
        expr: sum(x)
`

	result, diags := expandRecordingRulesGroupsSetFromYaml(y, "")
	if diags.HasError() {
		t.Fatalf("unmarshal failed: %s", diags)
	}
	if result.Name == nil || *result.Name != "from-yaml" {
		t.Fatalf("set name = %v, want from-yaml", result.Name)
	}
	if len(result.Groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(result.Groups))
	}
	g := result.Groups[0]
	if g.Name != "Foo" {
		t.Errorf("group name = %q, want Foo", g.Name)
	}
	if g.Interval == nil || *g.Interval != 180 {
		t.Errorf("interval = %v, want 180", g.Interval)
	}
	// Unquoted numeric limit must still coerce into the *string field.
	if g.Limit == nil || *g.Limit != "100" {
		t.Errorf("limit = %v, want \"100\"", g.Limit)
	}
	if len(g.Rules) != 2 {
		t.Fatalf("got %d rules, want 2", len(g.Rules))
	}
	r0 := g.Rules[0]
	if r0.Record != "job:http_requests_total:sum" || r0.Expr == "" {
		t.Errorf("rule0 record/expr wrong: %q / %q", r0.Record, r0.Expr)
	}
	if r0.Labels["team"] != "backend" {
		t.Errorf("rule0 labels = %v, want team=backend", r0.Labels)
	}
	if d, ok := r0.GetEvaluationDelayMsOk(); !ok || *d != 60000 {
		t.Errorf("rule0 evaluationDelayMs = %v (ok=%v), want 60000", d, ok)
	}
	// The sibling omits the key, so it must stay absent, not collapse to 0.
	if _, ok := g.Rules[1].GetEvaluationDelayMsOk(); ok {
		t.Errorf("rule1 evaluationDelayMs should be absent")
	}
}

func recordingRuleGroupSetCurrentSchema(ctx context.Context, t *testing.T) schema.Schema {
	t.Helper()

	resp := &resource.SchemaResponse{}
	(&RecordingRuleGroupSetResource{}).Schema(ctx, resource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("building current schema failed: %s", resp.Diagnostics)
	}

	return resp.Schema
}
