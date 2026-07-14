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
)

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
