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
	"maps"
	"reflect"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	alertapi "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/alert_definitions_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/alert"
)

// The hooks keep per-arm tables for the facts about an alert type that the schema cannot state. These tests
// derive the arms from the generated schema and fail for a new alert type that no table classifies, so it
// cannot take the default of a table silently.

// schemaArms returns the arms of type_definition of the generated schema.
func schemaArms(t *testing.T) map[string]schema.Attribute {
	t.Helper()
	typeDefinition, ok := alert.Schema().Attributes["type_definition"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("type_definition is not a SingleNestedAttribute")
	}
	return typeDefinition.Attributes
}

// armHasAttribute reports whether the arm has a direct attribute of that name.
func armHasAttribute(arm schema.Attribute, name string) bool {
	object, ok := arm.(schema.SingleNestedAttribute)
	if !ok {
		return false
	}
	_, found := object.Attributes[name]
	return found
}

func inAny(arm string, tables ...[]string) bool {
	for _, table := range tables {
		if slices.Contains(table, arm) {
			return true
		}
	}
	return false
}

// A new alert type must be in groupByArms (the API computes group_by for it) or in armsWithConfiguredGroupBy
// (it does not). Without a line, planGroupBy plans the configured null for an alert type whose read returns
// keys, and the apply fails with "Provider produced inconsistent result ... group_by".
func TestGroupByTablesCoverTheSchemaArms(t *testing.T) {
	arms := schemaArms(t)
	for _, arm := range slices.Sorted(maps.Keys(arms)) {
		inComputed, inConfigured := slices.Contains(groupByArms, arm), slices.Contains(armsWithConfiguredGroupBy, arm)
		switch {
		case inComputed && inConfigured:
			t.Errorf("alert type %q is in groupByArms and armsWithConfiguredGroupBy: it must be in one of them (alert_hooks_plan.go)", arm)
		case !inComputed && !inConfigured:
			t.Errorf("new alert type %q is not in groupByArms, armsWithConfiguredGroupBy: decide whether the API computes group_by for it "+
				"(a read returns keys that the configuration does not have) and add it to one of them (alert_hooks_plan.go); "+
				"if group_by is not allowed for it, add it to armsWithoutGroupBy too (alert_hooks_validate.go)", arm)
		}
	}
	for name, table := range map[string][]string{
		"groupByArms": groupByArms, "armsWithConfiguredGroupBy": armsWithConfiguredGroupBy, "armsWithoutGroupBy": armsWithoutGroupBy,
	} {
		for _, arm := range table {
			if _, ok := arms[arm]; !ok {
				t.Errorf("%s lists %q, which the schema no longer has as an alert type: remove it", name, arm)
			}
		}
		if len(slices.Compact(slices.Sorted(slices.Values(table)))) != len(table) {
			t.Errorf("%s lists an alert type twice", name)
		}
	}
	// An arm that takes no group_by gets the planned null of a configured one, unless the API derives it.
	for _, arm := range armsWithoutGroupBy {
		if !inAny(arm, armsWithConfiguredGroupBy, groupByArms) {
			t.Errorf("alert type %q takes no group_by, so it is in armsWithConfiguredGroupBy or groupByArms (alert_hooks_plan.go)", arm)
		}
	}
}

// groupByInput names the attribute whose change recomputes group_by: it must exist in the schema, or
// valueChanged never sees a change, and the state keeps stale keys.
func TestGroupByInputIsASchemaAttribute(t *testing.T) {
	root := alert.Schema().Attributes
	for arm, names := range groupByInput {
		if !slices.Contains(groupByArms, arm) {
			t.Errorf("groupByInput has a line for %q, which is not in groupByArms (alert_hooks_plan.go)", arm)
		}
		if len(names) < 2 || names[0] != "type_definition" || names[1] != arm {
			t.Errorf("groupByInput[%q] = %v: it must start with type_definition, %s", arm, names, arm)
			continue
		}
		attributes := root
		for i, name := range names {
			attribute, ok := attributes[name]
			if !ok {
				t.Errorf("groupByInput[%q] = %v: the schema has no attribute %q (alert_hooks_plan.go)", arm, names, name)
				break
			}
			if i == len(names)-1 {
				break
			}
			object, ok := attribute.(schema.SingleNestedAttribute)
			if !ok {
				t.Errorf("groupByInput[%q] = %v: %q is not a nested object", arm, names, name)
				break
			}
			attributes = object.Attributes
		}
	}
}

// emptyGroupByForm and apiArmIsSet find an arm in the API alert by its name, so every arm needs a field of
// the same name there.
func TestEveryArmHasAFieldInTheAPIAlert(t *testing.T) {
	apiAlert := reflect.TypeOf(alertapi.AlertDefProperties{})
	for _, arm := range slices.Sorted(maps.Keys(schemaArms(t))) {
		field, ok := apiAlert.FieldByName(goFieldName(arm))
		if !ok || field.Type.Kind() != reflect.Pointer || field.Type.Elem().Kind() != reflect.Struct {
			t.Errorf("alert type %q has no field %s in the API alert: emptyGroupByForm cannot read it (alert_hooks_read.go, apiArmIsSet)", arm, goFieldName(arm))
		}
	}
}

func TestArmsWithReturnedGroupByAreTheOnesWithoutInput(t *testing.T) {
	got := armsWithReturnedGroupBy()
	slices.Sort(got)
	if want := []string{"flow", "slo_threshold", "tracing_threshold"}; !slices.Equal(got, want) {
		t.Errorf("armsWithReturnedGroupBy = %v, want %v: emptyGroupByForm gives these an empty list, so a new one needs a read that returns [] "+
			"for it, and a test case in TestEmptyGroupByForm", got, want)
	}
}

// An alert type has a no_data_policy that the API returns as null when the policy is missing (analyticsArms,
// whose empty block is rejected) or as an object without values (noDataPolicyReadAsEmpty). A new one
// takes the first, unless it is listed.
func TestNoDataPolicyArmsAreClassified(t *testing.T) {
	arms := schemaArms(t)
	readAsEmpty := map[string]bool{}
	for _, candidate := range noDataPolicyReadAsEmpty {
		var matches []string
		for arm := range arms {
			field, ok := reflect.TypeOf(alertapi.AlertDefProperties{}).FieldByName(goFieldName(arm))
			if ok && field.Type.Kind() == reflect.Pointer && field.Type.Elem() == candidate {
				matches = append(matches, arm)
			}
		}
		if len(matches) != 1 {
			t.Errorf("noDataPolicyReadAsEmpty lists %s, which must be the API type of exactly one alert type, found %v (alert_hooks_defaults.go)", candidate, matches)
			continue
		}
		readAsEmpty[matches[0]] = true
	}
	for _, arm := range slices.Sorted(maps.Keys(arms)) {
		hasPolicy := armHasAttribute(arms[arm], "no_data_policy")
		inAnalytics := slices.Contains(analyticsArms, arm)
		switch {
		case hasPolicy && inAnalytics == readAsEmpty[arm]:
			t.Errorf("alert type %q has a no_data_policy: it must be in exactly one of analyticsArms (alert_hooks_validate.go, "+
				"missing policy reads as null) and noDataPolicyReadAsEmpty (alert_hooks_defaults.go, missing policy reads as an object "+
				"without values); in analyticsArms = %v, in noDataPolicyReadAsEmpty = %v", arm, inAnalytics, readAsEmpty[arm])
		case !hasPolicy && (inAnalytics || readAsEmpty[arm]):
			t.Errorf("alert type %q has no no_data_policy in the schema, but a table lists it: remove it", arm)
		}
	}
	for _, arm := range analyticsArms {
		if _, ok := arms[arm]; !ok {
			t.Errorf("analyticsArms lists %q, which the schema no longer has as an alert type: remove it (alert_hooks_validate.go)", arm)
		}
	}
}

// planGroupByFor plans group_by (not configured) for an alert that sets only the arm, with the state
// group_by ["a"] and the same type_definition in the state, as an unchanged alert has.
func planGroupByFor(t *testing.T, arm string) types.List {
	t.Helper()
	ctx := context.Background()
	s := alert.Schema()
	typeDefinition := map[string]map[string]tftypes.Value{}
	if arm != "" {
		typeDefinition[arm] = nil
	}
	groupBy := tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{stringValue("a")})
	raw := func(set map[string]tftypes.Value) tftypes.Value {
		set["type_definition"] = typeDefinitionOf(ctx, typeDefinition)
		return nullObject(s.Type().TerraformType(ctx), set)
	}
	req := resource.ModifyPlanRequest{
		Config: tfsdk.Config{Schema: s, Raw: raw(map[string]tftypes.Value{})},
		State:  tfsdk.State{Schema: s, Raw: raw(map[string]tftypes.Value{"group_by": groupBy})},
	}
	resp := &resource.ModifyPlanResponse{Plan: tfsdk.Plan{Schema: s, Raw: raw(map[string]tftypes.Value{"group_by": groupBy})}}
	if diags := planGroupBy(ctx, req, resp); diags.HasError() {
		t.Fatalf("%s: %v", arm, diags)
	}
	var planned types.List
	if diags := resp.Plan.GetAttribute(ctx, path.Root("group_by"), &planned); diags.HasError() {
		t.Fatalf("%s: %v", arm, diags)
	}
	return planned
}

// An unchanged alert keeps the group_by of its state for every alert type that has one computed, and plans
// the configured null for every other. An alert type that no table knows is the second case, and does not panic.
func TestPlanGroupByPerArm(t *testing.T) {
	kept := types.ListValueMust(types.StringType, []attr.Value{types.StringValue("a")})
	null := types.ListNull(types.StringType)
	for _, arm := range slices.Sorted(maps.Keys(schemaArms(t))) {
		want := null
		if slices.Contains(groupByArms, arm) {
			want = kept
		}
		if got := planGroupByFor(t, arm); !got.Equal(want) {
			t.Errorf("%s: planned group_by = %s, want %s", arm, got, want)
		}
	}
	if got := planGroupByFor(t, ""); !got.Equal(null) {
		t.Errorf("no alert type: planned group_by = %s, want null", got)
	}
}

func TestHooksToleratePlanAndReadOfAnUnknownArm(t *testing.T) {
	// An alert type that no table lists: the attribute of type_definition has an arm the tables do not know.
	unknown := types.ObjectValueMust(map[string]attr.Type{"future_alert_type": types.ObjectType{AttrTypes: map[string]attr.Type{}}},
		map[string]attr.Value{"future_alert_type": types.ObjectValueMust(map[string]attr.Type{}, map[string]attr.Value{})})
	if arm := firstSetArm(unknown, groupByArms); arm != "" {
		t.Errorf("firstSetArm found %q for an alert type that no table lists", arm)
	}
	if anyArmIsSet(unknown, armsWithoutGroupBy) || anyArmIsSet(unknown, analyticsArms) {
		t.Error("an alert type that no table lists is in armsWithoutGroupBy or analyticsArms")
	}
	if anyArmHasOverrideRules(context.Background(), unknown) {
		t.Error("an alert type without rules has override rules")
	}
	empty := types.ListValueMust(types.StringType, []attr.Value{})
	for name, v := range map[string]*alertapi.AlertDef{
		"nil alert": nil, "no properties": {}, "no arm": {AlertDefProperties: &alertapi.AlertDefProperties{}},
	} {
		if got := emptyGroupByForm(empty, v); !got.IsNull() {
			t.Errorf("%s: an empty group_by reads as %s, want null", name, got)
		}
	}
}
