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
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/alert"
)

// alertValue is a Terraform value that a test sets inside the configuration of the alert.
type alertValue = tftypes.Value

// nullObject returns an object value of typ whose attributes are null, except the ones in set.
func nullObject(typ tftypes.Type, set map[string]tftypes.Value) tftypes.Value {
	object, ok := typ.(tftypes.Object)
	if !ok {
		panic("not an object type")
	}
	attrs := make(map[string]tftypes.Value, len(object.AttributeTypes))
	for name, attrType := range object.AttributeTypes {
		attrs[name] = tftypes.NewValue(attrType, nil)
	}
	for name, value := range set {
		if _, ok := attrs[name]; !ok {
			panic("no attribute " + name)
		}
		attrs[name] = value
	}
	return tftypes.NewValue(typ, attrs)
}

// attributeType returns the Terraform type of the attribute at the names, from the top of the type.
func attributeType(typ tftypes.Type, names ...string) tftypes.Type {
	for _, name := range names {
		typ = typ.(tftypes.Object).AttributeTypes[name]
	}
	return typ
}

// alertConfig returns a configuration of the generated schema with the given top-level values.
func alertConfig(ctx context.Context, set map[string]tftypes.Value) tfsdk.Config {
	s := alert.Schema()
	return tfsdk.Config{Schema: s, Raw: nullObject(s.Type().TerraformType(ctx), set)}
}

// typeDefinitionOf returns a type_definition value that sets the arms of set (each an object whose
// attributes are the given values or null).
func typeDefinitionOf(ctx context.Context, arms map[string]map[string]tftypes.Value) tftypes.Value {
	typ := attributeType(alert.Schema().Type().TerraformType(ctx), "type_definition")
	set := map[string]tftypes.Value{}
	for arm, values := range arms {
		set[arm] = nullObject(attributeType(typ, arm), values)
	}
	return nullObject(typ, set)
}

func stringValue(s string) tftypes.Value { return tftypes.NewValue(tftypes.String, s) }

func errorSummaries(diags diag.Diagnostics) []string {
	var out []string
	for _, d := range diags.Errors() {
		out = append(out, d.Summary())
	}
	return out
}

func TestArmsWithOverrideMatchSchema(t *testing.T) {
	// An arm whose rules have an override block has a deprecated top-level priority. A new arm of
	// that kind needs a line in armsWithOverride.
	typeDefinition, ok := alert.Schema().Attributes["type_definition"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("type_definition is not a SingleNestedAttribute")
	}
	for arm, attribute := range typeDefinition.Attributes {
		want := hasOverrideRule(attribute)
		got := false
		for _, name := range armsWithOverride {
			got = got || name == arm
		}
		if got != want {
			t.Errorf("%s: in armsWithOverride = %v, rules with an override block = %v", arm, got, want)
		}
	}
}

// hasOverrideRule reports whether an arm has a rule with an override block, directly or in a nested
// object (the slo_threshold rules sit in error_budget and burn_rate).
func hasOverrideRule(attribute schema.Attribute) bool {
	object, ok := attribute.(schema.SingleNestedAttribute)
	if !ok {
		return false
	}
	for name, child := range object.Attributes {
		if name == "rules" && rulesHaveOverride(child) {
			return true
		}
		if hasOverrideRule(child) {
			return true
		}
	}
	return false
}

func rulesHaveOverride(rules schema.Attribute) bool {
	var nested schema.NestedAttributeObject
	switch r := rules.(type) {
	case schema.SetNestedAttribute:
		nested = r.NestedObject
	case schema.ListNestedAttribute:
		nested = r.NestedObject
	default:
		return false
	}
	_, ok := nested.Attributes["override"]
	return ok
}

func TestValidatePriorityDeprecation(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name     string
		arm      string
		priority bool
		wantWarn bool
	}{
		{"metric_threshold warns", "metric_threshold", true, true},
		{"slo_threshold warns", "slo_threshold", true, true},
		{"logs_threshold warns", "logs_threshold", true, true},
		{"logs_ratio_threshold warns", "logs_ratio_threshold", true, true},
		{"logs_time_relative_threshold warns", "logs_time_relative_threshold", true, true},
		{"analytics_threshold warns", "analytics_threshold", true, true},
		{"analytics_immediate does not warn", "analytics_immediate", true, false},
		{"metric_anomaly does not warn", "metric_anomaly", true, false},
		{"logs_immediate does not warn", "logs_immediate", true, false},
		{"tracing_threshold does not warn", "tracing_threshold", true, false},
		{"flow does not warn", "flow", true, false},
		{"null type_definition does not warn", "", true, false},
		{"priority unset does not warn", "metric_threshold", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			set := map[string]tftypes.Value{}
			if tc.priority {
				set["priority"] = stringValue("P1")
			}
			if tc.arm != "" {
				set["type_definition"] = typeDefinitionOf(ctx, map[string]map[string]tftypes.Value{tc.arm: nil})
			}
			diags := validateAlertConfig(ctx, alertConfig(ctx, set))
			if diags.HasError() {
				t.Fatalf("unexpected errors: %v", errorSummaries(diags))
			}
			if got := len(diags.Warnings()); tc.wantWarn && got != 1 || !tc.wantWarn && got != 0 {
				t.Fatalf("warnings = %v, want a deprecation warning: %v", diags.Warnings(), tc.wantWarn)
			}
		})
	}
}

func TestValidateGroupBy(t *testing.T) {
	ctx := context.Background()
	groupBy := tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{stringValue("a")})
	for arm, wantError := range map[string]bool{"logs_immediate": true, "logs_new_value": true, "tracing_immediate": true, "logs_threshold": false, "flow": false} {
		set := map[string]tftypes.Value{
			"group_by":        groupBy,
			"type_definition": typeDefinitionOf(ctx, map[string]map[string]tftypes.Value{arm: nil}),
		}
		diags := validateAlertConfig(ctx, alertConfig(ctx, set))
		if diags.HasError() != wantError {
			t.Errorf("%s: errors = %v, want an error: %v", arm, errorSummaries(diags), wantError)
		}
	}
}

func TestValidateGroupByForNeedsGroupBy(t *testing.T) {
	ctx := context.Background()
	for _, withGroupBy := range []bool{false, true} {
		set := map[string]tftypes.Value{
			"type_definition": typeDefinitionOf(ctx, map[string]map[string]tftypes.Value{
				"logs_ratio_threshold": {"group_by_for": stringValue("Both")},
			}),
		}
		if withGroupBy {
			set["group_by"] = tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{stringValue("a")})
		}
		diags := validateAlertConfig(ctx, alertConfig(ctx, set))
		if diags.HasError() == withGroupBy {
			t.Errorf("group_by set = %v: errors = %v", withGroupBy, errorSummaries(diags))
		}
	}
}

func TestValidateAnalyticsNoDataPolicy(t *testing.T) {
	ctx := context.Background()
	policyType := attributeType(alert.Schema().Type().TerraformType(ctx), "type_definition", "analytics_immediate", "no_data_policy")
	cases := []struct {
		name      string
		policy    tftypes.Value
		wantError bool
	}{
		{"null block is fine", tftypes.NewValue(policyType, nil), false},
		{"unknown block is fine", tftypes.NewValue(policyType, tftypes.UnknownValue), false},
		{"empty block is rejected", nullObject(policyType, nil), true},
		{"state set is fine", nullObject(policyType, map[string]tftypes.Value{"state": stringValue("ALERTING")}), false},
		{"unknown child is fine", nullObject(policyType, map[string]tftypes.Value{"state": tftypes.NewValue(tftypes.String, tftypes.UnknownValue)}), false},
	}
	for _, arm := range analyticsArms {
		for _, tc := range cases {
			t.Run(arm+" "+tc.name, func(t *testing.T) {
				set := map[string]tftypes.Value{
					"type_definition": typeDefinitionOf(ctx, map[string]map[string]tftypes.Value{arm: {"no_data_policy": tc.policy}}),
				}
				diags := validateAlertConfig(ctx, alertConfig(ctx, set))
				if diags.HasError() != tc.wantError {
					t.Errorf("errors = %v, want an error: %v", errorSummaries(diags), tc.wantError)
				}
			})
		}
	}
}

func TestValidateNoDataPolicyUnspecifiedWarning(t *testing.T) {
	ctx := context.Background()
	policyType := attributeType(alert.Schema().Type().TerraformType(ctx), "type_definition", "logs_threshold", "no_data_policy")
	for state, wantWarn := range map[string]bool{"UNSPECIFIED": true, "OK": false} {
		set := map[string]tftypes.Value{
			"type_definition": typeDefinitionOf(ctx, map[string]map[string]tftypes.Value{
				"logs_threshold": {"no_data_policy": nullObject(policyType, map[string]tftypes.Value{"state": stringValue(state)})},
			}),
		}
		diags := validateAlertConfig(ctx, alertConfig(ctx, set))
		if diags.HasError() || (len(diags.Warnings()) == 1) != wantWarn {
			t.Errorf("state %s: diagnostics = %v, want a warning: %v", state, diags, wantWarn)
		}
	}
}

func TestValidateAnalyticsRuleCount(t *testing.T) {
	ctx := context.Background()
	rulesType := attributeType(alert.Schema().Type().TerraformType(ctx), "type_definition", "analytics_threshold", "rules")
	rule := nullObject(rulesType.(tftypes.List).ElementType, nil)
	for count, wantError := range map[int]bool{1: false, 5: false, 6: true} {
		rules := make([]tftypes.Value, count)
		for i := range rules {
			rules[i] = rule
		}
		set := map[string]tftypes.Value{
			"type_definition": typeDefinitionOf(ctx, map[string]map[string]tftypes.Value{
				"analytics_threshold": {"rules": tftypes.NewValue(rulesType, rules)},
			}),
		}
		diags := validateAlertConfig(ctx, alertConfig(ctx, set))
		if diags.HasError() != wantError {
			t.Errorf("%d rules: errors = %v, want an error: %v", count, errorSummaries(diags), wantError)
		}
	}
}
