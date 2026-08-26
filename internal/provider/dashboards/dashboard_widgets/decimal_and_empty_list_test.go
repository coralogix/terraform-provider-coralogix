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

package dashboard_widgets

import (
	"math"
	"math/big"
	"testing"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// decimal reaches the API as an int32. A value that type cannot hold would be
// truncated or wrapped on the way out and read back as the changed number, which
// fails the apply with an inconsistent result, so it is rejected at plan time.
func TestDecimalRejectsValuesAnInt32CannotHold(t *testing.T) {
	ctx := t.Context()
	validators := DecimalSchema().Validators

	for name, testCase := range map[string]struct {
		value     types.Number
		wantError bool
	}{
		"whole":           {value: types.NumberValue(big.NewFloat(2)), wantError: false},
		"zero":            {value: types.NumberValue(big.NewFloat(0)), wantError: false},
		"int32_max":       {value: types.NumberValue(big.NewFloat(math.MaxInt32)), wantError: false},
		"int32_min":       {value: types.NumberValue(big.NewFloat(math.MinInt32)), wantError: false},
		"fractional":      {value: types.NumberValue(big.NewFloat(1.5)), wantError: true},
		"above_int32":     {value: types.NumberValue(big.NewFloat(math.MaxInt32 + 1)), wantError: true},
		"below_int32":     {value: types.NumberValue(big.NewFloat(math.MinInt32 - 1)), wantError: true},
		"far_above_int64": {value: types.NumberValue(new(big.Float).SetInt(new(big.Int).Lsh(big.NewInt(1), 96))), wantError: true},
		"null":            {value: types.NumberNull(), wantError: false},
		"unknown":         {value: types.NumberUnknown(), wantError: false},
	} {
		t.Run(name, func(t *testing.T) {
			response := &validator.NumberResponse{}
			for _, v := range validators {
				v.ValidateNumber(ctx, validator.NumberRequest{
					Path:        path.Root("decimal"),
					ConfigValue: testCase.value,
				}, response)
			}
			if got := response.Diagnostics.HasError(); got != testCase.wantError {
				t.Fatalf("decimal = %v produced error %t, want %t: %v", testCase.value, got, testCase.wantError, response.Diagnostics)
			}
		})
	}
}

// FlattenSpansFields turns a zero-length API result into a null list, so an
// explicitly configured empty list cannot round-trip. Every new site rejects one.
func TestNonEmptySpansFieldsRejectsAnEmptyList(t *testing.T) {
	ctx := t.Context()
	validators := NonEmptySpansFieldsSchema().Validators

	empty := types.ListValueMust(types.ObjectType{AttrTypes: SpansFieldModelAttr()}, []attr.Value{})
	response := &validator.ListResponse{}
	for _, v := range validators {
		v.ValidateList(ctx, validator.ListRequest{Path: path.Root("group_by"), ConfigValue: empty}, response)
	}
	if !response.Diagnostics.HasError() {
		t.Fatal("an explicit empty group_by was accepted, want a plan-time error")
	}

	// A zero-length list is what the flatten side cannot represent; a null list
	// means "not configured" and must stay allowed.
	response = &validator.ListResponse{}
	for _, v := range validators {
		v.ValidateList(ctx, validator.ListRequest{
			Path:        path.Root("group_by"),
			ConfigValue: types.ListNull(types.ObjectType{AttrTypes: SpansFieldModelAttr()}),
		}, response)
	}
	if response.Diagnostics.HasError() {
		t.Fatalf("a null group_by was rejected: %v", response.Diagnostics)
	}

	flattened, diags := FlattenSpansFields(ctx, nil)
	if diags.HasError() {
		t.Fatalf("FlattenSpansFields() diagnostics = %v", diags)
	}
	if !flattened.IsNull() {
		t.Fatalf("FlattenSpansFields(nil) = %v, want a null list; the validator above depends on that", flattened)
	}
}

// An enum the API did not set arrives as the enum's zero value, the empty string.
// Flattening it to the empty string puts a value in state the schema does not
// allow, and an Optional+Computed attribute then plans as unknown for ever
// because Terraform cannot reconcile it with an omitted configuration.
func TestFlattenEnumMapsAnAbsentValueToUnspecified(t *testing.T) {
	flattened, diags := FlattenLineChart(t.Context(), &dashboardservice.LineChart{})
	if diags.HasError() {
		t.Fatalf("FlattenLineChart() diagnostics = %v", diags)
	}
	if got := flattened.LineChart.XAxisTimeFormat; got.ValueString() != utils.UNSPECIFIED {
		t.Fatalf("flattened x_axis_time_format = %q, want %q", got.ValueString(), utils.UNSPECIFIED)
	}

	if got := FlattenEnum(dashboardservice.XAXISTIMEFORMAT_X_AXIS_TIME_FORMAT_HH_MM, DashboardProtoToSchemaXAxisTimeFormat); got.ValueString() != "hh_mm" {
		t.Fatalf("FlattenEnum(HH_MM) = %q, want \"hh_mm\"", got.ValueString())
	}
}

// The legend's placement is Optional+Computed with no default, so it has the same
// shape as the enums above: an absent value must flatten to a known name, or the
// attribute plans as unknown for ever. The pre-existing fixtures never caught it
// because they set placement explicitly.
func TestLegendPlacementFlattensAnAbsentValueToUnspecified(t *testing.T) {
	flattened := FlattenLegend(&dashboardservice.Legend{})
	if flattened == nil {
		t.Fatal("FlattenLegend() = nil for a present legend")
	}
	if got := flattened.Placement.ValueString(); got != utils.UNSPECIFIED {
		t.Fatalf("flattened placement = %q, want %q", got, utils.UNSPECIFIED)
	}

	placement, ok := LegendSchema().Attributes["placement"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("legend placement is %T, want schema.StringAttribute", LegendSchema().Attributes["placement"])
	}
	if len(placement.PlanModifiers) == 0 {
		t.Fatal("legend placement has no plan modifier, so an omitted value plans as unknown on every update")
	}
}
