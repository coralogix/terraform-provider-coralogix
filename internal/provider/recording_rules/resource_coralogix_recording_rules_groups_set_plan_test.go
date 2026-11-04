// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	https://www.apache.org/licenses/LICENSE-2.0
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
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var (
	planRuleType = tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"evaluation_delay_ms": tftypes.Number,
		"expr":                tftypes.String,
		"labels":              tftypes.Map{ElementType: tftypes.String},
		"record":              tftypes.String,
	}}
	planGroupType = tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"interval": tftypes.Number,
		"limit":    tftypes.Number,
		"name":     tftypes.String,
		"rules":    tftypes.List{ElementType: planRuleType},
	}}
	unknownRule  = tftypes.NewValue(planRuleType, tftypes.UnknownValue)
	unknownGroup = tftypes.NewValue(planGroupType, tftypes.UnknownValue)
)

// unknownLabels marks labels that Terraform plans as unknown.
const unknownLabels = "(unknown)"

// planRule builds a rule. labels is nil for null labels, unknownLabels, or a map.
func planRule(record string, labels any) tftypes.Value {
	labelsType := tftypes.Map{ElementType: tftypes.String}
	value := tftypes.NewValue(labelsType, nil)
	switch l := labels.(type) {
	case string:
		value = tftypes.NewValue(labelsType, tftypes.UnknownValue)
	case map[string]string:
		elems := map[string]tftypes.Value{}
		for k, v := range l {
			elems[k] = tftypes.NewValue(tftypes.String, v)
		}
		value = tftypes.NewValue(labelsType, elems)
	}
	return tftypes.NewValue(planRuleType, map[string]tftypes.Value{
		"evaluation_delay_ms": tftypes.NewValue(tftypes.Number, nil),
		"expr":                tftypes.NewValue(tftypes.String, "vector(1)"),
		"labels":              value,
		"record":              tftypes.NewValue(tftypes.String, record),
	})
}

func planGroup(name string, rules ...tftypes.Value) tftypes.Value {
	return tftypes.NewValue(planGroupType, map[string]tftypes.Value{
		"interval": tftypes.NewValue(tftypes.Number, nil),
		"limit":    tftypes.NewValue(tftypes.Number, nil),
		"name":     tftypes.NewValue(tftypes.String, name),
		"rules":    tftypes.NewValue(tftypes.List{ElementType: planRuleType}, rules),
	})
}

func planRoot(t *testing.T, typ tftypes.Type, groups ...tftypes.Value) tftypes.Value {
	t.Helper()
	object, ok := typ.(tftypes.Object)
	if !ok {
		t.Fatalf("the schema type is %T", typ)
	}
	return tftypes.NewValue(typ, map[string]tftypes.Value{
		"groups":       tftypes.NewValue(object.AttributeTypes["groups"], groups),
		"id":           tftypes.NewValue(tftypes.String, "id-1"),
		"name":         tftypes.NewValue(tftypes.String, "tf-set"),
		"yaml_content": tftypes.NewValue(tftypes.String, nil),
	})
}

// plannedLabels lists the labels of every known planned rule as group/record=labels.
func plannedLabels(t *testing.T, plan tftypes.Value) []string {
	t.Helper()
	var root map[string]tftypes.Value
	var groups []tftypes.Value
	if err := plan.As(&root); err != nil {
		t.Fatal(err)
	}
	if err := root["groups"].As(&groups); err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, group := range groups {
		if !group.IsKnown() {
			out = append(out, unknownLabels)
			continue
		}
		var g map[string]tftypes.Value
		var rules []tftypes.Value
		var name string
		if err := group.As(&g); err != nil {
			t.Fatal(err)
		}
		if err := g["name"].As(&name); err != nil {
			t.Fatal(err)
		}
		if err := g["rules"].As(&rules); err != nil {
			t.Fatal(err)
		}
		for _, rule := range rules {
			if !rule.IsKnown() {
				out = append(out, name+"/"+unknownLabels)
				continue
			}
			var r map[string]tftypes.Value
			var record string
			if err := rule.As(&r); err != nil {
				t.Fatal(err)
			}
			if err := r["record"].As(&record); err != nil {
				t.Fatal(err)
			}
			out = append(out, name+"/"+record+"="+labelsText(t, r["labels"]))
		}
	}
	slices.Sort(out)
	return out
}

func labelsText(t *testing.T, labels tftypes.Value) string {
	t.Helper()
	switch {
	case !labels.IsKnown():
		return unknownLabels
	case labels.IsNull():
		return "null"
	}
	var elems map[string]tftypes.Value
	if err := labels.As(&elems); err != nil {
		t.Fatal(err)
	}
	var pairs []string
	for _, k := range slices.Sorted(maps.Keys(elems)) {
		var v string
		if err := elems[k].As(&v); err != nil {
			t.Fatal(err)
		}
		pairs = append(pairs, k+":"+v)
	}
	return "{" + strings.Join(pairs, ",") + "}"
}

// ModifyPlan keeps the labels of a rule that the configuration leaves out: by group name and record,
// or from the removed rule of a group whose record changed. It skips a rule or a group that is
// unknown, and plans no labels for a rule whose prior rule it cannot tell.
func TestModifyPlanKeepsOmittedLabels(t *testing.T) {
	ctx := context.Background()
	r := NewRecordingRuleGroupSetResource()
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema: %v", schemaResp.Diagnostics)
	}
	s := schemaResp.Schema
	typ := s.Type().TerraformType(ctx)
	a := map[string]string{"a": "1"}
	b := map[string]string{"b": "2"}

	for _, tc := range []struct {
		name                string
		config, plan, state []tftypes.Value
		want                []string
	}{
		{
			name:   "a renamed rule takes the labels of the removed one",
			config: []tftypes.Value{planGroup("g1", planRule("r1_renamed", nil), planRule("r2", nil))},
			plan:   []tftypes.Value{planGroup("g1", planRule("r1_renamed", unknownLabels), planRule("r2", unknownLabels))},
			state:  []tftypes.Value{planGroup("g1", planRule("r1", a), planRule("r2", b))},
			want:   []string{"g1/r1_renamed={a:1}", "g1/r2={b:2}"},
		},
		{
			name:   "a renamed and moved rule keeps its own labels",
			config: []tftypes.Value{planGroup("g1", planRule("r2", nil), planRule("r1_renamed", nil))},
			plan:   []tftypes.Value{planGroup("g1", planRule("r2", unknownLabels), planRule("r1_renamed", unknownLabels))},
			state:  []tftypes.Value{planGroup("g1", planRule("r1", a), planRule("r2", b))},
			want:   []string{"g1/r1_renamed={a:1}", "g1/r2={b:2}"},
		},
		{
			name:   "a new rule next to a renamed one cannot be told apart",
			config: []tftypes.Value{planGroup("g1", planRule("r1_renamed", nil), planRule("r3", nil))},
			plan:   []tftypes.Value{planGroup("g1", planRule("r1_renamed", unknownLabels), planRule("r3", unknownLabels))},
			state:  []tftypes.Value{planGroup("g1", planRule("r1", a))},
			want:   []string{"g1/r1_renamed=" + unknownLabels, "g1/r3=" + unknownLabels},
		},
		{
			name:   "a configured value is kept",
			config: []tftypes.Value{planGroup("g1", planRule("r1_renamed", unknownLabels))},
			plan:   []tftypes.Value{planGroup("g1", planRule("r1_renamed", unknownLabels))},
			state:  []tftypes.Value{planGroup("g1", planRule("r1", a))},
			want:   []string{"g1/r1_renamed=" + unknownLabels},
		},
		{
			name:   "an unknown rule",
			config: []tftypes.Value{planGroup("g1", unknownRule)},
			plan:   []tftypes.Value{planGroup("g1", unknownRule)},
			state:  []tftypes.Value{planGroup("g1", planRule("r1", a))},
			want:   []string{"g1/" + unknownLabels},
		},
		{
			name:   "a known rule next to an unknown one",
			config: []tftypes.Value{planGroup("g1", planRule("r1", nil), unknownRule)},
			plan:   []tftypes.Value{planGroup("g1", planRule("r1", unknownLabels), unknownRule)},
			state:  []tftypes.Value{planGroup("g1", planRule("r1", a), planRule("r2", b))},
			want:   []string{"g1/" + unknownLabels, "g1/r1={a:1}"},
		},
		{
			name:   "an unknown group",
			config: []tftypes.Value{unknownGroup},
			plan:   []tftypes.Value{unknownGroup},
			state:  []tftypes.Value{planGroup("g1", planRule("r1", a))},
			want:   []string{unknownLabels},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := tfsdk.Plan{Schema: s, Raw: planRoot(t, typ, tc.plan...)}
			req := resource.ModifyPlanRequest{
				Config: tfsdk.Config{Schema: s, Raw: planRoot(t, typ, tc.config...)},
				Plan:   plan,
				State:  tfsdk.State{Schema: s, Raw: planRoot(t, typ, tc.state...)},
			}
			resp := resource.ModifyPlanResponse{Plan: plan}
			r.(resource.ResourceWithModifyPlan).ModifyPlan(ctx, req, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("ModifyPlan: %v", resp.Diagnostics)
			}
			if got := plannedLabels(t, resp.Plan.Raw); !slices.Equal(got, tc.want) {
				t.Fatalf("labels = %s, want %s", fmt.Sprint(got), fmt.Sprint(tc.want))
			}
		})
	}
}
