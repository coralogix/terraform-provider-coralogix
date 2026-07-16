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

package dashboard_widgets

import (
	"context"
	"strings"
	"testing"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func TestHexagonSpansQueryModelRoundTrip(t *testing.T) {
	ctx := context.Background()
	model := &HexagonModel{
		CustomUnit:    types.StringNull(),
		LegendBy:      types.StringNull(),
		Decimal:       types.NumberNull(),
		DataModeType:  types.StringNull(),
		Thresholds:    types.SetNull(types.ObjectType{AttrTypes: ThresholdAttr()}),
		ThresholdType: types.StringNull(),
		Min:           types.NumberNull(),
		Max:           types.NumberNull(),
		Unit:          types.StringNull(),
		Query: &HexagonQueryModel{
			Spans: &HexagonQuerySpansModel{
				LuceneQuery: types.StringNull(),
				GroupBy:     types.ListNull(types.ObjectType{AttrTypes: SpansFieldModelAttr()}),
				Aggregation: &SpansAggregationModel{
					Type:            types.StringValue("dimension"),
					AggregationType: types.StringValue("unique_count"),
					Field:           types.StringValue("trace_id"),
				},
				Filters: types.ListNull(types.ObjectType{AttrTypes: SpansFilterModelAttr()}),
			},
		},
	}

	value, diags := types.ObjectValueFrom(ctx, HexagonType().AttrTypes, model)
	if diags.HasError() {
		t.Fatalf("converting hexagon model to its Terraform object type: %v", diags)
	}
	var converted HexagonModel
	diags = value.As(ctx, &converted, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("converting Terraform object type back to hexagon model: %v", diags)
	}

	expanded, diags := expandHexagonSpansQuery(ctx, converted.Query.Spans)
	if diags.HasError() {
		t.Fatalf("expanding hexagon spans query: %v", diags)
	}
	if expanded.SpansAggregation == nil || expanded.SpansAggregation.DimensionAggregation == nil {
		t.Fatal("expanded hexagon spans query omitted its dimension aggregation")
	}

	flattened, diags := flattenHexagonSpansQuery(ctx, expanded)
	if diags.HasError() {
		t.Fatalf("flattening hexagon spans query: %v", diags)
	}
	if flattened.Spans == nil || flattened.Spans.Aggregation == nil {
		t.Fatal("flattened hexagon spans query omitted its aggregation")
	}
	if got := flattened.Spans.Aggregation.AggregationType.ValueString(); got != "unique_count" {
		t.Fatalf("flattened aggregation type = %q, want %q", got, "unique_count")
	}
	if got := flattened.Spans.Aggregation.Field.ValueString(); got != "trace_id" {
		t.Fatalf("flattened aggregation field = %q, want %q", got, "trace_id")
	}
}

func TestDataTableSpansAggregationModelRoundTrip(t *testing.T) {
	ctx := context.Background()
	model := &DataTableSpansAggregationModel{
		ID:        types.StringValue("aggregation-id"),
		Name:      types.StringValue("traces"),
		IsVisible: types.BoolValue(true),
		Aggregation: &SpansAggregationModel{
			Type:            types.StringValue("dimension"),
			AggregationType: types.StringValue("unique_count"),
			Field:           types.StringValue("trace_id"),
		},
	}

	expanded, dg := expandDataTableSpansAggregation(model)
	if dg != nil {
		t.Fatalf("expanding data-table spans aggregation: %s", dg.Detail())
	}
	flattened, diags := flattenDataTableSpansQueryAggregations(ctx, []dashboardservice.SpansQueryAggregation{*expanded})
	if diags.HasError() {
		t.Fatalf("flattening data-table spans aggregations: %v", diags)
	}

	var converted []DataTableSpansAggregationModel
	diags = flattened.ElementsAs(ctx, &converted, false)
	if diags.HasError() {
		t.Fatalf("converting flattened aggregations back to the Terraform model: %v", diags)
	}
	if len(converted) != 1 || converted[0].Aggregation == nil {
		t.Fatalf("flattened aggregations = %#v, want one aggregation wrapper", converted)
	}
	if got := converted[0].Aggregation.AggregationType.ValueString(); got != "unique_count" {
		t.Fatalf("flattened aggregation type = %q, want %q", got, "unique_count")
	}
	if got := converted[0].Aggregation.Field.ValueString(); got != "trace_id" {
		t.Fatalf("flattened aggregation field = %q, want %q", got, "trace_id")
	}
}

func TestLogsAggregationValidator(t *testing.T) {
	someObservationField := types.ObjectValueMust(ObservationFieldAttr(), map[string]attr.Value{
		"keypath": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("meta"), types.StringValue("responseTime")}),
		"scope":   types.StringValue("user_data"),
	})
	nullObservationField := types.ObjectNull(ObservationFieldAttr())

	cases := []struct {
		name             string
		aggType          string
		field            types.String
		observationField types.Object
		percent          types.Float64
		wantErr          string // empty = expect no error; otherwise must be substring of diagnostic
	}{
		{name: "count_no_field", aggType: "count", field: types.StringNull(), observationField: nullObservationField, percent: types.Float64Null()},
		{name: "count_with_field", aggType: "count", field: types.StringValue("foo"), observationField: nullObservationField, percent: types.Float64Null(), wantErr: "neither `field` nor `observation_field`"},
		{name: "count_with_observation_field", aggType: "count", field: types.StringNull(), observationField: someObservationField, percent: types.Float64Null(), wantErr: "neither `field` nor `observation_field`"},

		{name: "avg_neither", aggType: "avg", field: types.StringNull(), observationField: nullObservationField, percent: types.Float64Null(), wantErr: "either `field` or `observation_field` must be set"},
		{name: "avg_field_only", aggType: "avg", field: types.StringValue("meta.responseTime.numeric"), observationField: nullObservationField, percent: types.Float64Null()},
		{name: "avg_observation_field_only", aggType: "avg", field: types.StringNull(), observationField: someObservationField, percent: types.Float64Null()},
		{name: "avg_both_set", aggType: "avg", field: types.StringValue("foo"), observationField: someObservationField, percent: types.Float64Null(), wantErr: "mutually exclusive"},

		{name: "sum_observation_field_only", aggType: "sum", field: types.StringNull(), observationField: someObservationField, percent: types.Float64Null()},
		{name: "count_distinct_field_only", aggType: "count_distinct", field: types.StringValue("foo"), observationField: nullObservationField, percent: types.Float64Null()},

		{name: "percentile_field_and_percent", aggType: "percentile", field: types.StringValue("foo"), observationField: nullObservationField, percent: types.Float64Value(95)},
		{name: "percentile_missing_percent", aggType: "percentile", field: types.StringValue("foo"), observationField: nullObservationField, percent: types.Float64Null(), wantErr: "`percent` must be set"},
		{name: "avg_with_percent", aggType: "avg", field: types.StringValue("foo"), observationField: nullObservationField, percent: types.Float64Value(50), wantErr: "`percent` cannot be set"},

		// Unknown values must not trigger false positives — they may resolve to either set or null.
		{name: "avg_unknown_field_null_obs", aggType: "avg", field: types.StringUnknown(), observationField: nullObservationField, percent: types.Float64Null()},
		{name: "avg_null_field_unknown_obs", aggType: "avg", field: types.StringNull(), observationField: types.ObjectUnknown(ObservationFieldAttr()), percent: types.Float64Null()},
		{name: "avg_known_field_unknown_obs", aggType: "avg", field: types.StringValue("foo"), observationField: types.ObjectUnknown(ObservationFieldAttr()), percent: types.Float64Null()},
		{name: "count_unknown_field", aggType: "count", field: types.StringUnknown(), observationField: nullObservationField, percent: types.Float64Null()},
		{name: "count_unknown_obs", aggType: "count", field: types.StringNull(), observationField: types.ObjectUnknown(ObservationFieldAttr()), percent: types.Float64Null()},
		{name: "percentile_unknown_percent", aggType: "percentile", field: types.StringValue("foo"), observationField: nullObservationField, percent: types.Float64Unknown()},
		{name: "avg_unknown_percent", aggType: "avg", field: types.StringValue("foo"), observationField: nullObservationField, percent: types.Float64Unknown()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := types.ObjectValueMust(AggregationModelAttr(), map[string]attr.Value{
				"type":              types.StringValue(tc.aggType),
				"field":             tc.field,
				"percent":           tc.percent,
				"observation_field": tc.observationField,
			})

			req := validator.ObjectRequest{ConfigValue: cfg}
			resp := &validator.ObjectResponse{}
			logsAggregationValidator{}.ValidateObject(context.Background(), req, resp)

			if tc.wantErr == "" {
				if resp.Diagnostics.HasError() {
					t.Fatalf("expected no error, got: %s", resp.Diagnostics.Errors())
				}
				return
			}

			if !resp.Diagnostics.HasError() {
				t.Fatalf("expected error containing %q, got none", tc.wantErr)
			}
			joined := ""
			for _, d := range resp.Diagnostics.Errors() {
				joined += d.Summary() + ": " + d.Detail() + "\n"
			}
			if !strings.Contains(joined, tc.wantErr) {
				t.Fatalf("expected error containing %q, got:\n%s", tc.wantErr, joined)
			}
		})
	}
}

func TestOptionalEnumPointer(t *testing.T) {
	tests := []struct {
		name  string
		value types.String
		want  *dashboardservice.LegendPlacement
	}{
		{name: "null", value: types.StringNull()},
		{name: "unknown", value: types.StringUnknown()},
		{name: "invalid", value: types.StringValue("invalid")},
		{
			name:  "configured",
			value: types.StringValue("auto"),
			want:  dashboardservice.LEGENDPLACEMENT_LEGEND_PLACEMENT_AUTO.Ptr(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := OptionalEnumPointer(tt.value, DashboardLegendPlacementSchemaToProto)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("expected nil enum, got %q", *got)
				}
				return
			}
			if got == nil || *got != *tt.want {
				t.Fatalf("expected enum %q, got %v", *tt.want, got)
			}
		})
	}
}

func TestExpandLegendOmitsUnsetPlacement(t *testing.T) {
	legend, diags := ExpandLegend(context.Background(), &LegendModel{
		IsVisible:    types.BoolValue(true),
		Columns:      types.ListNull(types.StringType),
		GroupByQuery: types.BoolValue(false),
		Placement:    types.StringNull(),
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if legend.Placement != nil {
		t.Fatalf("expected placement to be omitted, got %q", *legend.Placement)
	}
}

func TestFilterOperatorEqualsAllRoundTripUsesEmptyList(t *testing.T) {
	configured := &FilterOperatorModel{
		Type:           types.StringValue("equals"),
		SelectedValues: types.ListValueMust(types.StringType, []attr.Value{}),
	}

	expanded, diags := expandFilterOperator(context.Background(), configured)
	if diags.HasError() {
		t.Fatalf("expanding equals-all filter operator: %v", diags)
	}
	if expanded == nil || expanded.Equals == nil || expanded.Equals.Selection == nil || expanded.Equals.Selection.All == nil {
		t.Fatalf("expected equals-all REST selection, got %#v", expanded)
	}

	flattened, diagnostic := FlattenFilterOperator(expanded)
	if diagnostic != nil {
		t.Fatalf("flattening equals-all filter operator: %s", diagnostic.Detail())
	}
	if flattened.SelectedValues.IsNull() || flattened.SelectedValues.IsUnknown() || len(flattened.SelectedValues.Elements()) != 0 {
		t.Fatalf("expected a known empty selected_values list, got %s", flattened.SelectedValues)
	}
}

func TestFilterOperatorValidatorRejectsEmptyNotEqualsSelection(t *testing.T) {
	for _, selectedValues := range []types.List{
		types.ListNull(types.StringType),
		types.ListValueMust(types.StringType, []attr.Value{}),
	} {
		config := types.ObjectValueMust(FilterOperatorModelAttr(), map[string]attr.Value{
			"type":            types.StringValue("not_equals"),
			"selected_values": selectedValues,
		})
		request := validator.ObjectRequest{ConfigValue: config}
		response := &validator.ObjectResponse{}

		filterOperatorValidator{}.ValidateObject(context.Background(), request, response)

		if !response.Diagnostics.HasError() {
			t.Fatalf("expected an error for not_equals with selected_values %s", selectedValues)
		}
	}
}

func TestAtMostOneOfAttributesValidator(t *testing.T) {
	emptyObject := types.ObjectType{AttrTypes: map[string]attr.Type{}}

	definitionValue := func(selected ...string) types.Object {
		values := make(map[string]attr.Value, len(SupportedWidgetTypes))
		selectedSet := make(map[string]struct{}, len(selected))
		for _, name := range selected {
			selectedSet[name] = struct{}{}
		}
		for _, name := range SupportedWidgetTypes {
			if _, ok := selectedSet[name]; ok {
				values[name] = types.ObjectValueMust(emptyObject.AttrTypes, map[string]attr.Value{})
			} else {
				values[name] = types.ObjectNull(emptyObject.AttrTypes)
			}
		}
		return types.ObjectValueMust(func() map[string]attr.Type {
			attributes := make(map[string]attr.Type, len(SupportedWidgetTypes))
			for _, name := range SupportedWidgetTypes {
				attributes[name] = emptyObject
			}
			return attributes
		}(), values)
	}

	tests := []struct {
		name       string
		value      types.Object
		wantError  bool
		wantDetail string
	}{
		{
			name:  "single branch",
			value: definitionValue("gauge"),
		},
		{
			name:  "no branch",
			value: definitionValue(),
		},
		{
			name:       "two branches",
			value:      definitionValue("line_chart", "gauge"),
			wantError:  true,
			wantDetail: "At most one of " + strings.Join(SupportedWidgetTypes, ", ") + " can be configured; found: gauge, line_chart",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := &validator.ObjectResponse{}
			AtMostOneOfAttributesValidator{AttributeNames: SupportedWidgetTypes}.ValidateObject(context.Background(), validator.ObjectRequest{
				ConfigValue: tt.value,
			}, response)

			if !tt.wantError {
				if response.Diagnostics.HasError() {
					t.Fatalf("expected no error, got: %s", response.Diagnostics.Errors())
				}
				return
			}

			if got := response.Diagnostics.ErrorsCount(); got != 1 {
				t.Fatalf("expected one diagnostic, got %d: %s", got, response.Diagnostics.Errors())
			}
			if detail := response.Diagnostics.Errors()[0].Detail(); !strings.Contains(detail, tt.wantDetail) {
				t.Fatalf("expected diagnostic detail to contain %q, got %q", tt.wantDetail, detail)
			}
		})
	}
}

func TestLegacyDurationOpenAPIRoundTrip(t *testing.T) {
	tests := []struct {
		configured string
		openAPI    string
		state      string
	}{
		{configured: "seconds:900", openAPI: "900s", state: "seconds:900"},
		{configured: "minutes:15", openAPI: "900s", state: "seconds:900"},
		{configured: "seconds:0", openAPI: "0s", state: "seconds:0"},
	}

	for _, tt := range tests {
		t.Run(tt.configured, func(t *testing.T) {
			got, diagnostic := legacyDurationToOpenAPI(tt.configured, "test duration")
			if diagnostic != nil {
				t.Fatalf("unexpected diagnostic: %s", diagnostic.Detail())
			}
			if got == nil || *got != tt.openAPI {
				t.Fatalf("expected OpenAPI duration %q, got %v", tt.openAPI, got)
			}
			if state := openAPIDurationToLegacy(got); state.ValueString() != tt.state {
				t.Fatalf("expected state duration %q, got %q", tt.state, state.ValueString())
			}
		})
	}
}

func TestGoDurationOpenAPIRoundTrip(t *testing.T) {
	got, diagnostic := GoDurationToOpenAPI(types.StringValue("1m0s"), "test interval")
	if diagnostic != nil {
		t.Fatalf("unexpected diagnostic: %s", diagnostic.Detail())
	}
	if got == nil || *got != "60s" {
		t.Fatalf("expected OpenAPI duration %q, got %v", "60s", got)
	}
	if state := OpenAPIDurationToGo(got); state.ValueString() != "1m0s" {
		t.Fatalf("expected state duration %q, got %q", "1m0s", state.ValueString())
	}
}
