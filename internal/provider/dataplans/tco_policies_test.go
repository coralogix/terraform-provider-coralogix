// Copyright 2024 Coralogix Ltd.
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

package dataplans

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// ruleObject builds the value of an applications or subsystems rule. A nil ruleType is a null
// rule_type, and an unknownType an unknown one.
func ruleObject(t *testing.T, ruleT tftypes.Type, ruleType *string, unknownType bool, names []string) tftypes.Value {
	t.Helper()
	object := ruleT.(tftypes.Object)
	namesT, ok := object.AttributeTypes["names"]
	if !ok {
		t.Fatalf("the rule has no attribute %q any more: update rejectMultiValueRules", "names")
	}
	ruleTypeT, ok := object.AttributeTypes["rule_type"]
	if !ok {
		t.Fatalf("the rule has no attribute %q any more: update rejectMultiValueRules", "rule_type")
	}
	elements := []tftypes.Value{}
	for _, name := range names {
		elements = append(elements, tftypes.NewValue(tftypes.String, name))
	}
	attributes := map[string]tftypes.Value{
		"names":     tftypes.NewValue(namesT, elements),
		"rule_type": tftypes.NewValue(ruleTypeT, nil),
	}
	if unknownType {
		attributes["rule_type"] = tftypes.NewValue(ruleTypeT, tftypes.UnknownValue)
	} else if ruleType != nil {
		attributes["rule_type"] = tftypes.NewValue(ruleTypeT, *ruleType)
	}
	return nullsExcept(ruleT, attributes)
}

// nullsExcept returns an object value with every attribute null except those in set.
func nullsExcept(objectType tftypes.Type, set map[string]tftypes.Value) tftypes.Value {
	attributes := map[string]tftypes.Value{}
	for name, attributeType := range objectType.(tftypes.Object).AttributeTypes {
		if value, ok := set[name]; ok {
			attributes[name] = value
		} else {
			attributes[name] = tftypes.NewValue(attributeType, nil)
		}
	}
	return tftypes.NewValue(objectType, attributes)
}

func TestRejectMultiValueRules(t *testing.T) {
	ctx := context.Background()
	str := func(s string) *string { return &s }

	// The attributes that hold a rule, for each resource. tags holds a map of rules by tag name.
	resources := map[string]struct {
		resource   resource.Resource
		attributes []string
	}{
		"logs":   {NewTCOPoliciesLogsResource(), []string{"applications", "subsystems"}},
		"traces": {NewTCOPoliciesTracesResource(), []string{"applications", "subsystems", "services", "actions", "tags"}},
		"rum":    {NewTCOPoliciesRumResource(), []string{"applications", "subsystems"}},
	}
	cases := []struct {
		name        string
		ruleType    *string
		unknownType bool
		names       []string
		wantError   bool
	}{
		{name: "starts_with with two names", ruleType: str("starts_with"), names: []string{"one", "two"}, wantError: true},
		{name: "includes with two names", ruleType: str("includes"), names: []string{"one", "two"}, wantError: true},
		{name: "starts_with with one name", ruleType: str("starts_with"), names: []string{"one"}},
		{name: "includes with one name", ruleType: str("includes"), names: []string{"one"}},
		{name: "is with two names", ruleType: str("is"), names: []string{"one", "two"}},
		{name: "is_not with two names", ruleType: str("is_not"), names: []string{"one", "two"}},
		{name: "null rule_type with two names", names: []string{"one", "two"}},
		{name: "unknown rule_type with two names", unknownType: true, names: []string{"one", "two"}},
	}

	for resourceName, entry := range resources {
		r := entry.resource
		var schemaResponse resource.SchemaResponse
		r.Schema(ctx, resource.SchemaRequest{}, &schemaResponse)
		rootT, ok := schemaResponse.Schema.Type().TerraformType(ctx).(tftypes.Object)
		if !ok {
			t.Fatalf("%s: the schema is not an object", resourceName)
		}
		policiesT, ok := rootT.AttributeTypes["policies"].(tftypes.List)
		if !ok {
			t.Fatalf("%s: policies is not a list any more: update rejectMultiValueRules", resourceName)
		}
		policyT := policiesT.ElementType.(tftypes.Object)

		for _, attribute := range entry.attributes {
			attributeT, ok := policyT.AttributeTypes[attribute]
			if !ok {
				t.Fatalf("%s: policies has no attribute %q any more: update rejectMultiValueRules", resourceName, attribute)
			}
			ruleT := attributeT
			if mapT, isMap := attributeT.(tftypes.Map); isMap {
				ruleT = mapT.ElementType
			} else if attribute == "tags" {
				t.Fatalf("%s: tags is not a map any more: update rejectMultiValueRules", resourceName)
			}
			for _, tc := range cases {
				t.Run(fmt.Sprintf("%s/%s/%s", resourceName, attribute, tc.name), func(t *testing.T) {
					value := ruleObject(t, ruleT, tc.ruleType, tc.unknownType, tc.names)
					if mapT, isMap := attributeT.(tftypes.Map); isMap {
						value = tftypes.NewValue(mapT, map[string]tftypes.Value{"tags.http.method": value})
					}
					policy := nullsExcept(policyT, map[string]tftypes.Value{attribute: value})
					root := nullsExcept(rootT, map[string]tftypes.Value{"policies": tftypes.NewValue(policiesT, []tftypes.Value{policy})})

					diags := rejectMultiValueRules(ctx, tfsdk.Config{Raw: root, Schema: schemaResponse.Schema})
					assertRuleDiagnostics(t, diags, tc.wantError)

					// The hook must be the one that the resource runs at validation.
					var validated resource.ValidateConfigResponse
					r.(resource.ResourceWithValidateConfig).ValidateConfig(ctx,
						resource.ValidateConfigRequest{Config: tfsdk.Config{Raw: root, Schema: schemaResponse.Schema}}, &validated)
					assertRuleDiagnostics(t, validated.Diagnostics, tc.wantError)
				})
			}
		}
	}
}

func TestRejectMultiValueRulesNullAndUnknownPolicies(t *testing.T) {
	ctx := context.Background()
	r := NewTCOPoliciesLogsResource()
	var schemaResponse resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResponse)
	rootT := schemaResponse.Schema.Type().TerraformType(ctx).(tftypes.Object)
	policiesT := rootT.AttributeTypes["policies"]

	for name, policies := range map[string]tftypes.Value{
		"null":    tftypes.NewValue(policiesT, nil),
		"unknown": tftypes.NewValue(policiesT, tftypes.UnknownValue),
	} {
		t.Run(name, func(t *testing.T) {
			root := nullsExcept(rootT, map[string]tftypes.Value{"policies": policies})
			diags := rejectMultiValueRules(ctx, tfsdk.Config{Raw: root, Schema: schemaResponse.Schema})
			assertRuleDiagnostics(t, diags, false)
		})
	}
}

func assertRuleDiagnostics(t *testing.T, diags diag.Diagnostics, wantError bool) {
	t.Helper()
	if wantError {
		if !diags.HasError() {
			t.Fatalf("want an error, got %v", diags)
		}
		if len(diags.Errors()) != 1 {
			t.Fatalf("want one error, got %v", diags.Errors())
		}
		return
	}
	if len(diags) != 0 {
		t.Fatalf("want no diagnostics, got %v", diags)
	}
}
