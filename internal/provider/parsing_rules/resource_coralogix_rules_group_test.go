package parsing_rules

import (
	"context"
	"regexp"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func extractRuleFixture(name string, order int) map[string]interface{} {
	return map[string]interface{}{
		"extract": []interface{}{map[string]interface{}{
			"name":               name,
			"description":        "",
			"active":             true,
			"order":              order,
			"source_field":       "text",
			"regular_expression": "(?P<field>.*)",
		}},
	}
}

func TestExpandRulesNumbersUnsetOrdersByPosition(t *testing.T) {
	rules, err := expandRules([]interface{}{
		extractRuleFixture("first", 0),
		extractRuleFixture("second", 0),
		extractRuleFixture("third", 0),
	})
	if err != nil {
		t.Fatalf("expandRules: %s", err)
	}
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(rules))
	}
	for i, rule := range rules {
		if rule.Order.GetValue() != uint32(i+1) {
			t.Errorf("rules[%d].Order = %d, want %d", i, rule.Order.GetValue(), i+1)
		}
	}
}

func TestExpandRulesKeepsExplicitOrders(t *testing.T) {
	rules, err := expandRules([]interface{}{
		extractRuleFixture("first", 30),
		extractRuleFixture("second", 10),
	})
	if err != nil {
		t.Fatalf("expandRules: %s", err)
	}
	if got := rules[0].Order.GetValue(); got != 30 {
		t.Errorf("rules[0].Order = %d, want 30", got)
	}
	if got := rules[1].Order.GetValue(); got != 10 {
		t.Errorf("rules[1].Order = %d, want 10", got)
	}
}

func TestExpandRuleSubgroupsNumbersUnsetOrdersByPosition(t *testing.T) {
	subgroups, err := expandRuleSubgroups([]interface{}{
		map[string]interface{}{
			"active": true,
			"order":  0,
			"rules":  []interface{}{extractRuleFixture("first", 0)},
		},
		map[string]interface{}{
			"active": true,
			"order":  0,
			"rules":  []interface{}{extractRuleFixture("second", 0)},
		},
	})
	if err != nil {
		t.Fatalf("expandRuleSubgroups: %s", err)
	}
	if len(subgroups) != 2 {
		t.Fatalf("expected 2 subgroups, got %d", len(subgroups))
	}
	for i, subgroup := range subgroups {
		if subgroup.Order.GetValue() != uint32(i+1) {
			t.Errorf("rule_subgroups[%d].Order = %d, want %d", i, subgroup.Order.GetValue(), i+1)
		}
	}
}

func objectWithNullDefaults(objectType cty.Type, values map[string]cty.Value) cty.Value {
	attributes := make(map[string]cty.Value, len(objectType.AttributeTypes()))
	for name, attributeType := range objectType.AttributeTypes() {
		if value, ok := values[name]; ok {
			attributes[name] = value
			continue
		}
		attributes[name] = cty.NullVal(attributeType)
	}
	return cty.ObjectVal(attributes)
}

type ruleGroupTypes struct {
	resource cty.Type
	subgroup cty.Type
	rule     cty.Type
	extract  cty.Type
}

func newRuleGroupTypes(t *testing.T) ruleGroupTypes {
	t.Helper()
	resourceType := ResourceCoralogixRulesGroup().CoreConfigSchema().ImpliedType()
	subgroupType := resourceType.AttributeTypes()["rule_subgroups"].ElementType()
	ruleType := subgroupType.AttributeTypes()["rules"].ElementType()
	return ruleGroupTypes{
		resource: resourceType,
		subgroup: subgroupType,
		rule:     ruleType,
		extract:  ruleType.AttributeTypes()["extract"].ElementType(),
	}
}

func (types ruleGroupTypes) newRule(name string, ruleOrder cty.Value) cty.Value {
	extract := objectWithNullDefaults(types.extract, map[string]cty.Value{
		"name":               cty.StringVal(name),
		"source_field":       cty.StringVal("text"),
		"regular_expression": cty.StringVal("(?P<field>.*)"),
		"order":              ruleOrder,
	})
	return objectWithNullDefaults(types.rule, map[string]cty.Value{
		"extract": cty.ListVal([]cty.Value{extract}),
	})
}

func (types ruleGroupTypes) newSubgroup(subgroupOrder cty.Value, rules ...cty.Value) cty.Value {
	return objectWithNullDefaults(types.subgroup, map[string]cty.Value{
		"order": subgroupOrder,
		"rules": cty.ListVal(rules),
	})
}

func (types ruleGroupTypes) newConfig(subgroups ...cty.Value) cty.Value {
	return objectWithNullDefaults(types.resource, map[string]cty.Value{
		"name":           cty.StringVal("order-validation"),
		"rule_subgroups": cty.ListVal(subgroups),
	})
}

func orderVal(value int) cty.Value { return cty.NumberIntVal(int64(value)) }

var unsetOrder = cty.NullVal(cty.Number)

// planRuleGroup drives the resource's diff the way PlanResourceChange does:
// the prior state carries the raw config, and the shimmed config carries the
// proposed new state. Returns the error CustomizeDiff produced, if any.
func planRuleGroup(t *testing.T, config, proposed cty.Value) error {
	t.Helper()
	resource := ResourceCoralogixRulesGroup()
	priorState, err := resource.ShimInstanceStateFromValue(cty.NullVal(config.Type()))
	if err != nil {
		t.Fatalf("ShimInstanceStateFromValue: %s", err)
	}
	priorState.RawConfig = config
	_, err = resource.SimpleDiff(
		context.Background(),
		priorState,
		terraform.NewResourceConfigShimmed(proposed, resource.CoreConfigSchema()),
		nil,
	)
	return err
}

func TestRuleGroupOrderValidation(t *testing.T) {
	types := newRuleGroupTypes(t)

	for _, tc := range []struct {
		name      string
		config    cty.Value
		wantError string
	}{
		{
			name: "every rule order unset",
			config: types.newConfig(types.newSubgroup(unsetOrder,
				types.newRule("first", unsetOrder),
				types.newRule("second", unsetOrder))),
		},
		{
			name: "every rule order set and unique",
			config: types.newConfig(types.newSubgroup(unsetOrder,
				types.newRule("first", orderVal(2)),
				types.newRule("second", orderVal(1)))),
		},
		{
			name: "every rule order set, unique and sparse",
			config: types.newConfig(types.newSubgroup(unsetOrder,
				types.newRule("first", orderVal(10)),
				types.newRule("second", orderVal(20)))),
		},
		{
			name: "one rule order unset alongside an explicit order",
			config: types.newConfig(types.newSubgroup(unsetOrder,
				types.newRule("first", unsetOrder),
				types.newRule("second", orderVal(1)))),
			wantError: `rule_subgroups\[0\].rules\[1\] sets order = 1 while rule_subgroups\[0\].rules\[0\] leaves order unset`,
		},
		{
			name: "two rules share an explicit order",
			config: types.newConfig(types.newSubgroup(unsetOrder,
				types.newRule("first", orderVal(3)),
				types.newRule("second", orderVal(3)))),
			wantError: `rule_subgroups\[0\].rules\[0\] and rule_subgroups\[0\].rules\[1\] both set order = 3`,
		},
		{
			name: "an unknown rule order is not a collision",
			config: types.newConfig(types.newSubgroup(unsetOrder,
				types.newRule("first", cty.UnknownVal(cty.Number)),
				types.newRule("second", orderVal(1)))),
		},
		{
			name: "an unknown order does not hide a collision between known orders",
			config: types.newConfig(types.newSubgroup(unsetOrder,
				types.newRule("first", orderVal(1)),
				types.newRule("second", orderVal(1)),
				types.newRule("third", cty.UnknownVal(cty.Number)))),
			wantError: `rule_subgroups\[0\].rules\[0\] and rule_subgroups\[0\].rules\[1\] both set order = 1`,
		},
		{
			name: "an unknown order does not hide a partially set list",
			config: types.newConfig(types.newSubgroup(unsetOrder,
				types.newRule("first", unsetOrder),
				types.newRule("second", orderVal(1)),
				types.newRule("third", cty.UnknownVal(cty.Number)))),
			wantError: `rule_subgroups\[0\].rules\[1\] sets order = 1 while rule_subgroups\[0\].rules\[0\] leaves order unset`,
		},
		{
			name: "an unknown subgroup order does not hide a collision between known orders",
			config: types.newConfig(
				types.newSubgroup(orderVal(2), types.newRule("first", unsetOrder)),
				types.newSubgroup(orderVal(2), types.newRule("second", unsetOrder)),
				types.newSubgroup(cty.UnknownVal(cty.Number), types.newRule("third", unsetOrder))),
			wantError: `rule_subgroups\[0\] and rule_subgroups\[1\] both set order = 2`,
		},
		{
			name: "each subgroup validated independently",
			config: types.newConfig(
				types.newSubgroup(orderVal(1), types.newRule("first", orderVal(1))),
				types.newSubgroup(orderVal(2), types.newRule("second", orderVal(1)))),
		},
		{
			name: "two subgroups share an explicit order",
			config: types.newConfig(
				types.newSubgroup(orderVal(1), types.newRule("first", unsetOrder)),
				types.newSubgroup(orderVal(1), types.newRule("second", unsetOrder))),
			wantError: `rule_subgroups\[0\] and rule_subgroups\[1\] both set order = 1`,
		},
		{
			name: "one subgroup order unset alongside an explicit order",
			config: types.newConfig(
				types.newSubgroup(unsetOrder, types.newRule("first", unsetOrder)),
				types.newSubgroup(orderVal(1), types.newRule("second", unsetOrder))),
			wantError: `rule_subgroups\[1\] sets order = 1 while rule_subgroups\[0\] leaves order unset`,
		},
		{
			name: "an unknown subgroup order is not a collision",
			config: types.newConfig(
				types.newSubgroup(cty.UnknownVal(cty.Number), types.newRule("first", unsetOrder)),
				types.newSubgroup(orderVal(1), types.newRule("second", unsetOrder))),
		},
		{
			name:   "no rule_subgroups at all",
			config: objectWithNullDefaults(types.resource, map[string]cty.Value{"name": cty.StringVal("empty")}),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := planRuleGroup(t, tc.config, tc.config)
			if tc.wantError == "" {
				if err != nil {
					t.Fatalf("expected no error, got %s", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error matching %q, got none", tc.wantError)
			}
			if !regexp.MustCompile(tc.wantError).MatchString(err.Error()) {
				t.Fatalf("error %q does not match %q", err, tc.wantError)
			}
		})
	}
}

// TestRuleGroupOrderValidationCatchesResolvedUnknown covers the apply-time
// backstop: an unknown order is accepted while it is unknown, and the same
// check rejects it once the value resolves, which is what extract runs against
// the resolved config before building the request.
func TestRuleGroupOrderValidationCatchesResolvedUnknown(t *testing.T) {
	types := newRuleGroupTypes(t)

	planTime := types.newConfig(types.newSubgroup(unsetOrder,
		types.newRule("first", orderVal(1)),
		types.newRule("second", cty.UnknownVal(cty.Number))))
	if err := validateRuleGroupOrdersInConfig(planTime); err != nil {
		t.Fatalf("an unknown order must not be rejected while it is unknown, got %s", err)
	}

	resolvedToDuplicate := types.newConfig(types.newSubgroup(unsetOrder,
		types.newRule("first", orderVal(1)),
		types.newRule("second", orderVal(1))))
	if err := validateRuleGroupOrdersInConfig(resolvedToDuplicate); err == nil {
		t.Fatal("an order that resolves to a duplicate must be rejected")
	}

	resolvedToNull := types.newConfig(types.newSubgroup(unsetOrder,
		types.newRule("first", orderVal(1)),
		types.newRule("second", unsetOrder)))
	if err := validateRuleGroupOrdersInConfig(resolvedToNull); err == nil {
		t.Fatal("an order that resolves to null alongside an explicit sibling must be rejected")
	}
}

// TestRuleGroupOrderValidationIgnoresComputedState guards against reading the
// merged plan instead of the raw config: on a second plan the orders the API
// assigned are carried into the proposed new state, and an all-unset config
// must still be accepted rather than read as an explicit collision.
func TestRuleGroupOrderValidationIgnoresComputedState(t *testing.T) {
	types := newRuleGroupTypes(t)

	config := types.newConfig(types.newSubgroup(unsetOrder,
		types.newRule("first", unsetOrder),
		types.newRule("second", unsetOrder)))
	proposed := types.newConfig(types.newSubgroup(orderVal(1),
		types.newRule("first", orderVal(1)),
		types.newRule("second", orderVal(2))))

	if err := planRuleGroup(t, config, proposed); err != nil {
		t.Fatalf("expected no error for an all-unset config on a second plan, got %s", err)
	}
}
