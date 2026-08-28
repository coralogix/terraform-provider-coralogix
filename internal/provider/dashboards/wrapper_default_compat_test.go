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
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_schema"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// The three wrapper fields that lost a static default are a gauge min and max,
// and a data table column width. None of them may declare a static default: all
// three are wrapper values in the proto, so an omitted one is absent and not
// zero, and a default sends a value the dashboard never had.
//
// Each one still has to be computed with a plan modifier that copies the prior
// state. The API returns a value for a dashboard created outside Terraform, so a
// configuration that omits the attribute must keep what the read put in state,
// or every plan reports a change. The modifier must copy a null prior state too:
// the API returns no gauge bound at all for a gauge built in the Coralogix UI,
// and a null state left unknown is a plan that is never empty.
func TestWrapperValuesKeepTheirPriorValue(t *testing.T) {
	t.Parallel()
	root := dashboard_schema.V4()
	widget := []string{"layout", "sections", "rows", "widgets", "definition"}

	for name, path := range map[string][]string{
		"gauge min":               append(append([]string{}, widget...), "gauge", "min"),
		"gauge max":               append(append([]string{}, widget...), "gauge", "max"),
		"data table column width": append(append([]string{}, widget...), "data_table", "columns", "width"),
	} {
		t.Run(name, func(t *testing.T) {
			attribute := dashboardResolveAttribute(t, root.Attributes, path...)
			if attribute == nil {
				t.Fatalf("%s is missing", name)
			}
			ctx := context.Background()
			var hasDefault bool
			var optional, computed bool
			var modifiers int
			// Zero is the value to plan with: the gauge min of a dashboard
			// created outside Terraform is zero, and a modifier that treated it
			// as no value would plan a change on every run.
			switch typed := attribute.(type) {
			case schema.Float64Attribute:
				hasDefault, optional, computed, modifiers = typed.Default != nil, typed.Optional, typed.Computed, len(typed.PlanModifiers)
				assertWrapperPlans(t, name,
					planWrapperFloat64(ctx, typed, types.Float64Value(0), wrapperPriorState()), types.Float64Value(0),
					planWrapperFloat64(ctx, typed, types.Float64Null(), wrapperPriorState()),
					planWrapperFloat64(ctx, typed, types.Float64Null(), wrapperCreateState()))
			case schema.Int64Attribute:
				hasDefault, optional, computed, modifiers = typed.Default != nil, typed.Optional, typed.Computed, len(typed.PlanModifiers)
				assertWrapperPlans(t, name,
					planWrapperInt64(ctx, typed, types.Int64Value(0), wrapperPriorState()), types.Int64Value(0),
					planWrapperInt64(ctx, typed, types.Int64Null(), wrapperPriorState()),
					planWrapperInt64(ctx, typed, types.Int64Null(), wrapperCreateState()))
			default:
				t.Fatalf("%s has unexpected kind %T", name, attribute)
			}
			if hasDefault {
				t.Errorf("%s declares a static default, which invents a value the API never had", name)
			}
			if !optional || !computed {
				t.Errorf("%s must be optional and computed, got optional=%t computed=%t", name, optional, computed)
			}
			if modifiers == 0 {
				t.Errorf("%s has no plan modifier, so every plan reports known after apply", name)
			}
		})
	}
}

// assertWrapperPlans checks the three plans that decide whether a configuration
// without the attribute produces an empty plan: the value already in state
// stays, a state with no value plans no value, and a create plans unknown so the
// API decides.
func assertWrapperPlans(t *testing.T, name string, plannedFromState, wantFromState, plannedFromNull, plannedOnCreate attr.Value) {
	t.Helper()
	if !plannedFromState.Equal(wantFromState) {
		t.Errorf("%s planned as %v with %v in state, want the state value", name, plannedFromState, wantFromState)
	}
	if !plannedFromNull.IsNull() {
		t.Errorf("%s planned as %v with no value in state, want no value", name, plannedFromNull)
	}
	if !plannedOnCreate.IsUnknown() {
		t.Errorf("%s planned as %v on create, want unknown", name, plannedOnCreate)
	}
}

// planWrapperFloat64 and planWrapperInt64 run the attribute's plan modifiers for
// a configuration that omits the attribute.
func planWrapperFloat64(ctx context.Context, attribute schema.Float64Attribute, stateValue types.Float64, state tfsdk.State) types.Float64 {
	response := &planmodifier.Float64Response{PlanValue: types.Float64Unknown()}
	for _, modifier := range attribute.PlanModifiers {
		modifier.PlanModifyFloat64(ctx, planmodifier.Float64Request{
			Path:        path.Root("wrapper"),
			ConfigValue: types.Float64Null(),
			PlanValue:   response.PlanValue,
			StateValue:  stateValue,
			State:       state,
		}, response)
	}
	return response.PlanValue
}

func planWrapperInt64(ctx context.Context, attribute schema.Int64Attribute, stateValue types.Int64, state tfsdk.State) types.Int64 {
	response := &planmodifier.Int64Response{PlanValue: types.Int64Unknown()}
	for _, modifier := range attribute.PlanModifiers {
		modifier.PlanModifyInt64(ctx, planmodifier.Int64Request{
			Path:        path.Root("wrapper"),
			ConfigValue: types.Int64Null(),
			PlanValue:   response.PlanValue,
			StateValue:  stateValue,
			State:       state,
		}, response)
	}
	return response.PlanValue
}

// wrapperPriorState is the state of a resource that already exists, and
// wrapperCreateState is the null state of a resource being created. The
// modifiers read the raw state to tell the two apart.
func wrapperPriorState() tfsdk.State {
	return tfsdk.State{Raw: tftypes.NewValue(
		tftypes.Object{AttributeTypes: map[string]tftypes.Type{}},
		map[string]tftypes.Value{},
	)}
}

func wrapperCreateState() tfsdk.State {
	return tfsdk.State{Raw: tftypes.NewValue(
		tftypes.Object{AttributeTypes: map[string]tftypes.Type{}},
		nil,
	)}
}

// The widget highlight is the opposite case. The API returns a value for every
// widget, so plain optional would fail the apply with "was null, but now false".
// It has to be computed, and a computed attribute needs the prior value or every
// plan reports "known after apply". Writing false is how a user stops
// highlighting a widget.
func TestWidgetHighlightedKeepsItsPriorValue(t *testing.T) {
	t.Parallel()
	attribute := dashboardResolveAttribute(t, dashboard_schema.V4().Attributes,
		"layout", "sections", "rows", "widgets", "highlighted")
	highlighted, ok := attribute.(schema.BoolAttribute)
	if !ok {
		t.Fatalf("highlighted is %T, want a bool attribute", attribute)
	}
	if !highlighted.Optional || !highlighted.Computed {
		t.Errorf("highlighted must be optional and computed, got optional=%t computed=%t", highlighted.Optional, highlighted.Computed)
	}
	if highlighted.Default != nil {
		t.Error("highlighted must not declare a static default")
	}
	if len(highlighted.PlanModifiers) == 0 {
		t.Error("highlighted has no plan modifier, so every plan reports known after apply")
	}
}
