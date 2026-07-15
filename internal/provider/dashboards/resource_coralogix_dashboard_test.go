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
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	dashboardwidgets "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_widgets"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func ptr[T any](v T) *T {
	return &v
}

func TestExtractDashboardContentJSONRestoresAliasesBeforeDiscardingUnknownFields(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "testdata", "dashboards", "content_json_unknown_fields.json"))
	if err != nil {
		t.Fatalf("read content_json unknown-fields fixture: %s", err)
	}

	dashboard, diags := extractDashboard(context.Background(), DashboardResourceModel{
		ContentJson: types.StringValue(string(content)),
		Folder:      types.ObjectNull(dashboardFolderModelAttr()),
	})
	if diags.HasError() {
		t.Fatalf("extract content_json dashboard: %v", diags)
	}
	definition := dashboard.Layout.Sections[0].Rows[0].Widgets[0].Definition
	if definition == nil || definition.DataTable == nil {
		t.Fatal("expected data_table alias to be restored into the typed dataTable field")
	}
	dataTable := definition.DataTable
	if dataTable.ResultsPerPage == nil || *dataTable.ResultsPerPage != 10 {
		t.Fatalf("expected results_per_page alias to be restored, got %v", dataTable.ResultsPerPage)
	}
	if dataTable.RowStyle == nil || *dataTable.RowStyle != dashboardservice.ROWSTYLE_ROW_STYLE_ONE_LINE {
		t.Fatalf("expected row_style alias to be restored, got %v", dataTable.RowStyle)
	}
	if dataTable.Query == nil || dataTable.Query.Metrics == nil || dataTable.Query.Metrics.PromqlQuery == nil {
		t.Fatal("expected nested query and promql_query aliases to be restored into typed fields")
	}

	request := newDashboardOpenAPICreateRequest(*dashboard, nil)
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal normalized content_json request: %s", err)
	}
	for _, unknownField := range []string{
		"unknownRoot", "unknownLayout", "unknownSection", "unknownRow", "unknownWidget",
		"unknownDefinition", "unknownDataTable", "unknownQuery", "unknownMetrics", "unknownPromqlQuery",
	} {
		if strings.Contains(string(encoded), unknownField) {
			t.Fatalf("expected unknown property %q to be discarded from request %s", unknownField, encoded)
		}
	}
}

func TestDashboardAccessPolicyForConfiguredRequest(t *testing.T) {
	policy := types.StringValue(`{"version":"2025-01-01"}`)

	tests := []struct {
		name   string
		config types.String
		plan   types.String
		want   *string
	}{
		{
			name:   "omitted config does not send preserved state",
			config: types.StringNull(),
			plan:   policy,
		},
		{
			name:   "configured policy sends planned value",
			config: policy,
			plan:   policy,
			want:   policy.ValueStringPointer(),
		},
		{
			name:   "configured empty value does not send",
			config: types.StringValue(""),
			plan:   types.StringValue(""),
		},
		{
			name:   "configured unknown sends planned value",
			config: types.StringUnknown(),
			plan:   policy,
			want:   policy.ValueStringPointer(),
		},
		{
			name:   "unknown plan does not send",
			config: types.StringUnknown(),
			plan:   types.StringUnknown(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dashboardAccessPolicyForConfiguredRequest(tt.config, tt.plan)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("expected nil access policy, got %q", *got)
				}
				return
			}
			if got == nil || *got != *tt.want {
				t.Fatalf("expected access policy %q, got %v", *tt.want, got)
			}
		})
	}
}

func TestExpandAnnotationID(t *testing.T) {
	tests := []struct {
		name   string
		id     types.String
		wantID string
	}{
		{name: "generates ID when omitted", id: types.StringNull()},
		{name: "preserves existing ID", id: types.StringValue("existing-id"), wantID: "existing-id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annotation, diags := expandAnnotation(context.Background(), DashboardAnnotationModel{
				ID:      tt.id,
				Name:    types.StringValue("test"),
				Enabled: types.BoolValue(true),
				Source:  types.ObjectNull(annotationSourceModelAttr()),
			})
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if annotation.Id == nil {
				t.Fatal("expected annotation ID, got nil")
			}

			if tt.wantID != "" {
				if *annotation.Id != tt.wantID {
					t.Fatalf("expected ID %q, got %q", tt.wantID, *annotation.Id)
				}
				return
			}
			if _, err := uuid.Parse(*annotation.Id); err != nil {
				t.Fatalf("expected generated UUID, got %q: %s", *annotation.Id, err)
			}
		})
	}
}

func TestExpandDashboardVariableDefinition_ConstantValueDeprecated(t *testing.T) {
	def := &DashboardVariableDefinitionModel{
		ConstantValue: types.StringValue("production"),
	}

	_, diags := expandDashboardVariableDefinition(context.Background(), def)
	if !diags.HasError() {
		t.Fatalf("expected an error for the deprecated constant_value, got none")
	}

	msg := diags.Errors()[0].Summary() + " " + diags.Errors()[0].Detail()
	if !strings.Contains(msg, "constant_value") || !strings.Contains(msg, "multi_select") {
		t.Fatalf("expected the error to direct users from constant_value to multi_select, got: %s", msg)
	}
}

func TestFlattenDashboardVariableDefinition_LegacyConstantBecomesMultiSelect(t *testing.T) {
	def := &dashboardservice.VariableDefinition{
		Constant: &dashboardservice.Constant{Value: ptr("production")},
	}

	got, diags := flattenDashboardVariableDefinition(context.Background(), def)
	if diags.HasError() {
		t.Fatalf("unexpected error flattening legacy constant: %v", diags)
	}
	if got.MultiSelect == nil {
		t.Fatalf("expected legacy Constant to flatten into multi_select, got %+v", got)
	}
	if !got.ConstantValue.IsNull() {
		t.Fatalf("expected constant_value to be null after remap, got %q", got.ConstantValue.ValueString())
	}
}

func TestMultiSelectSpansFieldNameRoundTrip(t *testing.T) {
	ctx := context.Background()
	query := &dashboardservice.QuerySpansQuery{
		Type: &dashboardservice.QuerySpansQueryType{
			FieldName: &dashboardservice.QuerySpansQueryTypeFieldName{
				SpanRegex: ptr(".*"),
			},
		},
	}

	flattened, diags := flattenDashboardVariableDefinitionMultiSelectQuerySpansModel(ctx, query)
	if diags.HasError() {
		t.Fatalf("flattening spans field-name query: %v", diags)
	}

	expanded, diags := expandMultiSelectSpansQuery(ctx, flattened)
	if diags.HasError() {
		t.Fatalf("expanding spans field-name query: %v", diags)
	}
	if expanded == nil || expanded.Type == nil || expanded.Type.FieldName == nil || expanded.Type.FieldName.GetSpanRegex() != ".*" {
		t.Fatalf("expected spans field-name regex to round-trip, got %#v", expanded)
	}
}

func TestMultiSelectAllSelectionRoundTripUsesEmptyList(t *testing.T) {
	expanded, diags := expandMultiSelectSelection(context.Background(), []attr.Value{})
	if diags.HasError() {
		t.Fatalf("expanding all selection: %v", diags)
	}
	if expanded == nil || expanded.All == nil {
		t.Fatalf("expected all selection, got %#v", expanded)
	}

	flattened, diags := flattenDashboardVariableSelectedValues(expanded)
	if diags.HasError() {
		t.Fatalf("flattening all selection: %v", diags)
	}
	if flattened.IsNull() || flattened.IsUnknown() || len(flattened.Elements()) != 0 {
		t.Fatalf("expected a known empty selected_values list, got %s", flattened)
	}
}

func TestAnnotationRangeStrategyRoundTrip(t *testing.T) {
	ctx := context.Background()
	observationField := func(key string) types.Object {
		return types.ObjectValueMust(dashboardwidgets.ObservationFieldAttr(), map[string]attr.Value{
			"keypath": types.ListValueMust(types.StringType, []attr.Value{types.StringValue(key)}),
			"scope":   types.StringValue("metadata"),
		})
	}
	rangeValue := types.ObjectValueMust(rangeStrategyModelAttr(), map[string]attr.Value{
		"start_timestamp_field": observationField("start"),
		"end_timestamp_field":   observationField("end"),
	})

	logs, diags := expandLogsSourceRangeStrategy(ctx, rangeValue)
	if diags.HasError() {
		t.Fatalf("expanding logs range strategy: %v", diags)
	}
	if logs == nil || logs.Range == nil {
		t.Fatalf("expected logs range strategy, got %#v", logs)
	}
	if _, diags := flattenLogsStrategyRange(ctx, logs.Range); diags.HasError() {
		t.Fatalf("flattening logs range strategy: %v", diags)
	}

	spans, diags := expandSpansSourceRangeStrategy(ctx, rangeValue)
	if diags.HasError() {
		t.Fatalf("expanding spans range strategy: %v", diags)
	}
	if spans == nil || spans.Range == nil {
		t.Fatalf("expected spans range strategy, got %#v", spans)
	}
	if _, diags := flattenSpansStrategyRange(ctx, spans.Range); diags.HasError() {
		t.Fatalf("flattening spans range strategy: %v", diags)
	}
}

func TestAnnotationLogsAndSpansExpansionOmitsDataModeType(t *testing.T) {
	ctx := context.Background()
	observationField := types.ObjectValueMust(dashboardwidgets.ObservationFieldAttr(), map[string]attr.Value{
		"keypath": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("timestamp")}),
		"scope":   types.StringValue("metadata"),
	})
	strategy := types.ObjectValueMust(logsAndSpansStrategyModelAttr(), map[string]attr.Value{
		"instant": types.ObjectValueMust(instantStrategyModelAttr(), map[string]attr.Value{
			"timestamp_field": observationField,
		}),
		"range":    types.ObjectNull(rangeStrategyModelAttr()),
		"duration": types.ObjectNull(durationStrategyModelAttr()),
	})
	source := types.ObjectValueMust(annotationsLogsAndSpansSourceModelAttr(), map[string]attr.Value{
		"lucene_query":     types.StringValue("*"),
		"strategy":         strategy,
		"message_template": types.StringNull(),
		"label_fields":     types.ListNull(types.ObjectType{AttrTypes: dashboardwidgets.ObservationFieldAttr()}),
	})

	logs, diags := expandLogsSource(ctx, source)
	if diags.HasError() {
		t.Fatalf("expanding logs annotation source: %v", diags)
	}
	if logs == nil || logs.DataModeType != nil {
		t.Fatalf("expected logs data_mode_type to be omitted from the request, got %#v", logs)
	}

	spans, diags := expandSpansSource(ctx, source)
	if diags.HasError() {
		t.Fatalf("expanding spans annotation source: %v", diags)
	}
	if spans == nil || spans.DataModeType != nil {
		t.Fatalf("expected spans data_mode_type to be omitted from the request, got %#v", spans)
	}
}

func TestFlattenDashboardOptionsColor(t *testing.T) {
	tests := []struct {
		name       string
		color      *dashboardservice.SectionColor
		wantNull   bool
		wantString string
	}{
		{
			name:     "color absent (nil)",
			color:    nil,
			wantNull: true,
		},
		{
			name: "predefined unspecified (zero value) is null",
			color: &dashboardservice.SectionColor{
				Predefined: dashboardservice.SECTIONPREDEFINEDCOLOR_SECTION_PREDEFINED_COLOR_UNSPECIFIED.Ptr(),
			},
			wantNull: true,
		},
		{
			name:     "color wrapper present but value oneof unset is null",
			color:    &dashboardservice.SectionColor{},
			wantNull: true,
		},
		{
			name: "predefined cyan flattens to lowercase string",
			color: &dashboardservice.SectionColor{
				Predefined: dashboardservice.SECTIONPREDEFINEDCOLOR_SECTION_PREDEFINED_COLOR_CYAN.Ptr(),
			},
			wantString: "cyan",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &dashboardservice.SectionOptions{
				Custom: &dashboardservice.CustomSectionOptions{
					Name:  ptr("section"),
					Color: tt.color,
				},
			}
			model, diags := flattenDashboardOptions(context.Background(), opts)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if model == nil {
				t.Fatalf("expected non-nil model, got nil")
			}
			if tt.wantNull {
				if !model.Color.IsNull() {
					t.Fatalf("expected null color, got %q", model.Color.ValueString())
				}
				return
			}
			if model.Color.IsNull() {
				t.Fatalf("expected color %q, got null", tt.wantString)
			}
			if got := model.Color.ValueString(); got != tt.wantString {
				t.Fatalf("expected color %q, got %q", tt.wantString, got)
			}
		})
	}
}
