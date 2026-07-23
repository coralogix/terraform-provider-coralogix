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
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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

func TestSupportedWidgetsValidatorDoesNotIncludeEmptyPaths(t *testing.T) {
	description := SupportedWidgetsValidatorWithout("gauge").Description(context.Background())

	if count := strings.Count(description, "<."); count != len(SupportedWidgetTypes)-1 {
		t.Fatalf("widget validator description contains %d relative paths, want %d: %s", count, len(SupportedWidgetTypes)-1, description)
	}
}

func TestExactlyOneOfWidgetsReportsConfiguredBranchesOnce(t *testing.T) {
	ctx := context.Background()
	emptyObjectType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}
	emptyAttributeTypes := map[string]attr.Type{}
	attributeSchemas := make(map[string]schema.Attribute, len(SupportedWidgetTypes))
	terraformTypes := make(map[string]tftypes.Type, len(SupportedWidgetTypes))
	terraformValues := make(map[string]tftypes.Value, len(SupportedWidgetTypes))
	configValues := make(map[string]types.Object, len(SupportedWidgetTypes))

	for _, name := range SupportedWidgetTypes {
		attributeSchemas[name] = schema.SingleNestedAttribute{
			Optional:   true,
			Attributes: map[string]schema.Attribute{},
			Validators: []validator.Object{SupportedWidgetsValidatorWithout(name)},
		}
		terraformTypes[name] = emptyObjectType
		if name == "gauge" || name == "line_chart" {
			terraformValues[name] = tftypes.NewValue(emptyObjectType, map[string]tftypes.Value{})
			configValues[name] = types.ObjectValueMust(emptyAttributeTypes, map[string]attr.Value{})
		} else {
			terraformValues[name] = tftypes.NewValue(emptyObjectType, nil)
			configValues[name] = types.ObjectNull(emptyAttributeTypes)
		}
	}

	config := tfsdk.Config{
		Schema: schema.Schema{Attributes: attributeSchemas},
		Raw:    tftypes.NewValue(tftypes.Object{AttributeTypes: terraformTypes}, terraformValues),
	}
	var diagnostics diag.Diagnostics
	for _, name := range SupportedWidgetTypes {
		request := validator.ObjectRequest{
			Config:         config,
			ConfigValue:    configValues[name],
			Path:           path.Root(name),
			PathExpression: path.MatchRoot(name),
		}
		response := &validator.ObjectResponse{}
		SupportedWidgetsValidatorWithout(name).ValidateObject(ctx, request, response)
		diagnostics.Append(response.Diagnostics...)
	}

	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics count = %d, want 1: %v", len(diagnostics), diagnostics)
	}
	detail := diagnostics[0].Detail()
	if !strings.Contains(detail, "`gauge`, `line_chart`") {
		t.Fatalf("diagnostic does not identify the configured branches: %s", detail)
	}
}

func TestExactlyOneOfAsymmetricSchemaStillReportsConflict(t *testing.T) {
	ctx := context.Background()
	attributeSchemas := map[string]schema.Attribute{
		"constant_list": schema.StringAttribute{Optional: true},
		"logs_path": schema.StringAttribute{
			Optional: true,
			Validators: []validator.String{
				ExactlyOneOfString(path.MatchRelative().AtParent().AtName("constant_list")),
			},
		},
	}
	config := tfsdk.Config{
		Schema: schema.Schema{Attributes: attributeSchemas},
		Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"constant_list": tftypes.String,
			"logs_path":     tftypes.String,
		}}, map[string]tftypes.Value{
			"constant_list": tftypes.NewValue(tftypes.String, "constant"),
			"logs_path":     tftypes.NewValue(tftypes.String, "logs"),
		}),
	}
	request := validator.StringRequest{
		Config:         config,
		ConfigValue:    types.StringValue("logs"),
		Path:           path.Root("logs_path"),
		PathExpression: path.MatchRoot("logs_path"),
	}
	response := &validator.StringResponse{}
	attributeSchemas["logs_path"].(schema.StringAttribute).Validators[0].ValidateString(ctx, request, response)

	if len(response.Diagnostics) != 1 {
		t.Fatalf("diagnostics count = %d, want 1: %v", len(response.Diagnostics), response.Diagnostics)
	}
	if detail := response.Diagnostics[0].Detail(); !strings.Contains(detail, "`constant_list`, `logs_path`") {
		t.Fatalf("diagnostic does not identify the configured branches: %s", detail)
	}
}

// buildOneOfConfig constructs a types.Object matching the shape
// exactlyOneOfChildrenValidator expects: one StringType attribute per name in
// childNames, set to a known value for names in setNames, to unknown for
// names in unknownNames, and to null for everything else.
func buildOneOfConfig(childNames, setNames, unknownNames []string) types.Object {
	set := make(map[string]bool, len(setNames))
	for _, name := range setNames {
		set[name] = true
	}
	unknown := make(map[string]bool, len(unknownNames))
	for _, name := range unknownNames {
		unknown[name] = true
	}

	attrTypes := make(map[string]attr.Type, len(childNames))
	values := make(map[string]attr.Value, len(childNames))
	for _, name := range childNames {
		attrTypes[name] = types.StringType
		switch {
		case unknown[name]:
			values[name] = types.StringUnknown()
		case set[name]:
			values[name] = types.StringValue(name)
		default:
			values[name] = types.StringNull()
		}
	}

	return types.ObjectValueMust(attrTypes, values)
}

func TestExactlyOneOfChildrenValidatesSetCounts(t *testing.T) {
	ctx := context.Background()
	childNames := []string{"a", "b", "c"}

	cases := []struct {
		name       string
		childNames []string
		setNames   []string
		wantErr    string // empty = expect no error
	}{
		{name: "zero_set", childNames: childNames, wantErr: "No attribute was configured"},
		{name: "one_set_first", childNames: childNames, setNames: []string{"a"}},
		{name: "one_set_middle", childNames: childNames, setNames: []string{"b"}},
		{name: "one_set_last", childNames: childNames, setNames: []string{"c"}},
		{name: "two_set", childNames: childNames, setNames: []string{"a", "c"}, wantErr: "Only one of these attributes can be configured"},
		{name: "all_widget_types_set", childNames: SupportedWidgetTypes, setNames: SupportedWidgetTypes, wantErr: "Only one of these attributes can be configured"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := buildOneOfConfig(tc.childNames, tc.setNames, nil)
			req := validator.ObjectRequest{ConfigValue: cfg}
			resp := &validator.ObjectResponse{}
			ExactlyOneOfChildren(tc.childNames...).ValidateObject(ctx, req, resp)

			if tc.wantErr == "" {
				if resp.Diagnostics.HasError() {
					t.Fatalf("expected no error, got: %s", resp.Diagnostics.Errors())
				}
				return
			}

			if !resp.Diagnostics.HasError() {
				t.Fatalf("expected error containing %q, got none", tc.wantErr)
			}
			detail := resp.Diagnostics.Errors()[0].Detail()
			if !strings.Contains(detail, tc.wantErr) {
				t.Fatalf("expected error containing %q, got: %s", tc.wantErr, detail)
			}
			if len(tc.setNames) > 1 {
				for _, name := range tc.setNames {
					if !strings.Contains(detail, "`"+name+"`") {
						t.Fatalf("expected error to name %q, got: %s", name, detail)
					}
				}
			}
		})
	}

	t.Run("parent_null", func(t *testing.T) {
		req := validator.ObjectRequest{ConfigValue: types.ObjectNull(map[string]attr.Type{"a": types.StringType})}
		resp := &validator.ObjectResponse{}
		ExactlyOneOfChildren("a", "b").ValidateObject(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("expected no error for null parent, got: %s", resp.Diagnostics.Errors())
		}
	})

	t.Run("parent_unknown", func(t *testing.T) {
		req := validator.ObjectRequest{ConfigValue: types.ObjectUnknown(map[string]attr.Type{"a": types.StringType})}
		resp := &validator.ObjectResponse{}
		ExactlyOneOfChildren("a", "b").ValidateObject(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("expected no error for unknown parent, got: %s", resp.Diagnostics.Errors())
		}
	})
}

// TestExactlyOneOfChildrenDefersWhenSiblingUnknown is the regression test for
// the false-positive "no attribute configured" error that fired whenever a
// sibling in the group was unknown (e.g. derived from an unresolved
// reference), even though it might resolve to satisfy the oneof once known.
func TestExactlyOneOfChildrenDefersWhenSiblingUnknown(t *testing.T) {
	ctx := context.Background()
	childNames := []string{"a", "b", "c", "d"}

	cases := []struct {
		name         string
		setNames     []string
		unknownNames []string
		wantErr      bool
	}{
		{name: "one_unknown_rest_null", unknownNames: []string{"a"}},
		{name: "two_unknown_rest_null", unknownNames: []string{"a", "b"}},
		{name: "one_unknown_one_set", unknownNames: []string{"a"}, setNames: []string{"b"}},
		{name: "one_unknown_two_set_is_a_definite_conflict", unknownNames: []string{"a"}, setNames: []string{"b", "c"}, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := buildOneOfConfig(childNames, tc.setNames, tc.unknownNames)
			req := validator.ObjectRequest{ConfigValue: cfg}
			resp := &validator.ObjectResponse{}
			ExactlyOneOfChildren(childNames...).ValidateObject(ctx, req, resp)

			if tc.wantErr {
				if !resp.Diagnostics.HasError() {
					t.Fatal("expected an error: two children are known-and-set, no unknown sibling can undo that conflict")
				}
				return
			}
			if resp.Diagnostics.HasError() {
				t.Fatalf("expected no error while a sibling is still unknown, got: %s", resp.Diagnostics.Errors())
			}
		})
	}
}

// TestSupportedWidgetsExactlyOneOfChildrenCoversAllWidgetTypes drives the
// 0/1/2-set matrix through the real SupportedWidgetTypes slice so a future
// 9th widget type is automatically covered without anyone remembering to
// update this test.
func TestSupportedWidgetsExactlyOneOfChildrenCoversAllWidgetTypes(t *testing.T) {
	ctx := context.Background()

	t.Run("zero_set", func(t *testing.T) {
		cfg := buildOneOfConfig(SupportedWidgetTypes, nil, nil)
		req := validator.ObjectRequest{ConfigValue: cfg}
		resp := &validator.ObjectResponse{}
		SupportedWidgetsExactlyOneOfChildren().ValidateObject(ctx, req, resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected an error when no widget type is configured")
		}
		if detail := resp.Diagnostics.Errors()[0].Detail(); !strings.Contains(detail, "No attribute was configured") {
			t.Fatalf("unexpected error detail: %s", detail)
		}
	})

	for _, name := range SupportedWidgetTypes {
		t.Run("one_set_"+name, func(t *testing.T) {
			cfg := buildOneOfConfig(SupportedWidgetTypes, []string{name}, nil)
			req := validator.ObjectRequest{ConfigValue: cfg}
			resp := &validator.ObjectResponse{}
			SupportedWidgetsExactlyOneOfChildren().ValidateObject(ctx, req, resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("expected no error with only %q set, got: %s", name, resp.Diagnostics.Errors())
			}
		})
	}

	t.Run("two_set", func(t *testing.T) {
		if len(SupportedWidgetTypes) < 2 {
			t.Skip("need at least two widget types")
		}
		twoSet := SupportedWidgetTypes[:2]
		cfg := buildOneOfConfig(SupportedWidgetTypes, twoSet, nil)
		req := validator.ObjectRequest{ConfigValue: cfg}
		resp := &validator.ObjectResponse{}
		SupportedWidgetsExactlyOneOfChildren().ValidateObject(ctx, req, resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected an error when two widget types are configured")
		}
		detail := resp.Diagnostics.Errors()[0].Detail()
		for _, name := range twoSet {
			if !strings.Contains(detail, "`"+name+"`") {
				t.Fatalf("expected error to name %q, got: %s", name, detail)
			}
		}
	})
}

// mustType asserts v has static type T, failing the test immediately with a
// message naming what was being asserted. It centralizes the assert-or-fail
// branch so call sites read as a single expression instead of each
// contributing its own branch to cyclomatic complexity.
func mustType[T any](t *testing.T, v any, what string) T {
	t.Helper()
	x, ok := v.(T)
	if !ok {
		t.Fatalf("%s is %T, not %T", what, v, *new(T))
	}
	return x
}

// hexagonQueryChildValue builds a tftypes.Value for one child of Hexagon's
// "query" oneof: null when unset, tftypes.UnknownValue when unknown, or a
// present-but-otherwise-empty object (every grandchild null) when set. The
// validator under test only inspects null/unknown-ness of the direct
// children, so grandchildren don't need to be recursively populated.
func hexagonQueryChildValue(childType tftypes.Type, set, unknown bool) tftypes.Value {
	if unknown {
		return tftypes.NewValue(childType, tftypes.UnknownValue)
	}
	if !set {
		return tftypes.NewValue(childType, nil)
	}
	objType, ok := childType.(tftypes.Object)
	if !ok {
		return tftypes.NewValue(childType, nil)
	}
	inner := make(map[string]tftypes.Value, len(objType.AttributeTypes))
	for innerName, innerType := range objType.AttributeTypes {
		inner[innerName] = tftypes.NewValue(innerType, nil)
	}
	return tftypes.NewValue(childType, inner)
}

// hexagonBuildQueryConfig builds a types.Object for Hexagon's query oneof,
// setting every name in setNames present-but-empty, every name in
// unknownNames to unknown, and leaving the rest null.
func hexagonBuildQueryConfig(ctx context.Context, t *testing.T, query schema.SingleNestedAttribute, queryType tftypes.Object, childNames, setNames, unknownNames []string) types.Object {
	t.Helper()
	set := make(map[string]bool, len(setNames))
	for _, name := range setNames {
		set[name] = true
	}
	unknown := make(map[string]bool, len(unknownNames))
	for _, name := range unknownNames {
		unknown[name] = true
	}
	attrs := make(map[string]tftypes.Value, len(childNames))
	for _, name := range childNames {
		attrs[name] = hexagonQueryChildValue(queryType.AttributeTypes[name], set[name], unknown[name])
	}
	val, err := query.GetType().ValueFromTerraform(ctx, tftypes.NewValue(queryType, attrs))
	if err != nil {
		t.Fatalf("build hexagon query config: %s", err)
	}
	return mustType[types.Object](t, val, "hexagon query config value")
}

// hexagonValidateQuery runs query's validators against cfg and returns the
// resulting diagnostics.
func hexagonValidateQuery(ctx context.Context, query schema.SingleNestedAttribute, cfg types.Object) diag.Diagnostics {
	req := validator.ObjectRequest{ConfigValue: cfg}
	resp := &validator.ObjectResponse{}
	for _, v := range query.Validators {
		v.ValidateObject(ctx, req, resp)
	}
	return resp.Diagnostics
}

// TestHexagonQueryExactlyOneOfChildren drives the same 0-set/2-set/
// unknown-sibling matrix as TestExactlyOneOfChildrenValidatesSetCounts and
// TestExactlyOneOfChildrenDefersWhenSiblingUnknown, but through the actual
// validator wired onto HexagonSchema()'s "query" attribute rather than a
// hand-built one, so a future regression that un-wires it is caught here.
func TestHexagonQueryExactlyOneOfChildren(t *testing.T) {
	ctx := context.Background()

	hexagon := mustType[schema.SingleNestedAttribute](t, HexagonSchema(), "HexagonSchema")
	query := mustType[schema.SingleNestedAttribute](t, hexagon.Attributes["query"], "hexagon query")
	if len(query.Validators) != 1 {
		t.Fatalf("hexagon query validators = %d, want exactly 1", len(query.Validators))
	}

	queryType := mustType[tftypes.Object](t, query.GetType().TerraformType(ctx), "hexagon query terraform type")
	childNames := []string{"logs", "metrics", "spans", "data_prime"}
	for _, name := range childNames {
		if _, ok := queryType.AttributeTypes[name]; !ok {
			t.Fatalf("hexagon query has no %q child in its terraform type", name)
		}
	}

	build := func(setNames, unknownNames []string) types.Object {
		return hexagonBuildQueryConfig(ctx, t, query, queryType, childNames, setNames, unknownNames)
	}
	validate := func(cfg types.Object) diag.Diagnostics {
		return hexagonValidateQuery(ctx, query, cfg)
	}

	t.Run("zero_set", func(t *testing.T) {
		diagnostics := validate(build(nil, nil))
		if !diagnostics.HasError() {
			t.Fatal("expected an error when no query branch is configured")
		}
		if detail := diagnostics.Errors()[0].Detail(); !strings.Contains(detail, "No attribute was configured") {
			t.Fatalf("unexpected error detail: %s", detail)
		}
	})

	for _, name := range childNames {
		t.Run("one_set_"+name, func(t *testing.T) {
			if diagnostics := validate(build([]string{name}, nil)); diagnostics.HasError() {
				t.Fatalf("expected no error with only %q set, got: %s", name, diagnostics.Errors())
			}
		})
	}

	t.Run("two_set", func(t *testing.T) {
		twoSet := childNames[:2]
		diagnostics := validate(build(twoSet, nil))
		if !diagnostics.HasError() {
			t.Fatal("expected an error when two query branches are configured")
		}
		detail := diagnostics.Errors()[0].Detail()
		for _, name := range twoSet {
			if !strings.Contains(detail, "`"+name+"`") {
				t.Fatalf("expected error to name %q, got: %s", name, detail)
			}
		}
	})

	t.Run("one_unknown_rest_null_defers", func(t *testing.T) {
		if diagnostics := validate(build(nil, []string{"logs"})); diagnostics.HasError() {
			t.Fatalf("expected no error while %q is unknown, got: %s", "logs", diagnostics.Errors())
		}
	})

	t.Run("one_unknown_one_set_defers", func(t *testing.T) {
		if diagnostics := validate(build([]string{"metrics"}, []string{"logs"})); diagnostics.HasError() {
			t.Fatalf("expected no error while a sibling is unknown, got: %s", diagnostics.Errors())
		}
	})

	t.Run("one_unknown_two_set_is_a_definite_conflict", func(t *testing.T) {
		cfg := build([]string{"logs", "metrics"}, []string{"spans"})
		if diagnostics := validate(cfg); !diagnostics.HasError() {
			t.Fatal("expected an error: two children are known-and-set, an unknown sibling can't undo that conflict")
		}
	})
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
