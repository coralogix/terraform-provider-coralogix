// Copyright 2025 Coralogix Ltd.
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
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestDynamicWidgetLogsStatRoundTrip(t *testing.T) {
	ctx := context.Background()

	logsQuery := &DynamicQueryModel{
		Logs: &DynamicQueryLogsModel{
			LuceneQuery: types.StringValue("level:error"),
			GroupBy: types.ListValueMust(ObservationFieldsObject(), []attr.Value{
				types.ObjectValueMust(ObservationFieldAttr(), map[string]attr.Value{
					"keypath": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("service")}),
					"scope":   types.StringValue("user_data"),
				}),
			}),
			Aggregations: types.ListValueMust(types.ObjectType{AttrTypes: AggregationModelAttr()}, []attr.Value{
				types.ObjectValueMust(AggregationModelAttr(), map[string]attr.Value{
					"type":              types.StringValue("count"),
					"field":             types.StringNull(),
					"percent":           types.Float64Null(),
					"observation_field": types.ObjectNull(ObservationFieldAttr()),
				}),
			}),
			Filters:      types.ListNull(types.ObjectType{AttrTypes: LogsFilterModelAttr()}),
			DataModeType: types.StringValue("archive"),
		},
	}

	original := &DynamicModel{
		QueryDefinitions: types.ListValueMust(
			types.ObjectType{AttrTypes: dynamicQueryDefinitionModelAttr()},
			[]attr.Value{
				types.ObjectValueMust(dynamicQueryDefinitionModelAttr(), map[string]attr.Value{
					"id":   types.StringValue("query-1"),
					"name": types.StringValue("errors by service"),
					"query": types.ObjectValueMust(dynamicQueryModelAttr(), map[string]attr.Value{
						"logs":       objectFrom(ctx, t, dynamicLogsQueryAttr(), logsQuery.Logs),
						"spans":      types.ObjectNull(dynamicSpansQueryAttr()),
						"metrics":    types.ObjectNull(dynamicMetricsQueryAttr()),
						"data_prime": types.ObjectNull(dynamicDataPrimeQueryAttr()),
					}),
				}),
			},
		),
		TimeFrame: &TimeFrameModel{
			Relative: &TimeFrameRelativeModel{Duration: types.StringValue("seconds:900")},
		},
		Visualization: &DynamicVisualizationModel{
			Stat: &DynamicStatModel{
				AllowAbbreviation: types.BoolNull(),
				CategoryFields:    types.ListNull(ObservationFieldsObject()),
				CustomUnit:        types.StringNull(),
				DecimalPrecision:  types.Int64Null(),
				DisplaySeriesName: types.BoolNull(),
				Legend:            nil,
				LegendBy:          types.StringValue("unspecified"),
				Max:               types.Float64Null(),
				Min:               types.Float64Null(),
				ThresholdBy:       types.StringValue("unspecified"),
				ThresholdType:     types.StringValue("absolute"),
				Thresholds: types.ListValueMust(types.ObjectType{AttrTypes: dynamicThresholdAttr()}, []attr.Value{
					types.ObjectValueMust(dynamicThresholdAttr(), map[string]attr.Value{
						"from":  types.Float64Value(10),
						"color": types.StringValue("#ff0000"),
						"label": types.StringValue("high"),
					}),
				}),
				Unit: types.StringValue("bytes"),
				ValueField: types.ObjectValueMust(ObservationFieldAttr(), map[string]attr.Value{
					"keypath": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("duration")}),
					"scope":   types.StringValue("metadata"),
				}),
				ValueFields: types.ListNull(ObservationFieldsObject()),
			},
		},
	}

	originalObject, diags := types.ObjectValueFrom(ctx, dynamicModelAttr(), original)
	if diags.HasError() {
		t.Fatalf("normalizing original dynamic model: %v", diags)
	}

	definition, diags := ExpandDynamic(ctx, original)
	if diags.HasError() {
		t.Fatalf("expanding dynamic model: %v", diags)
	}
	if definition == nil || definition.Dynamic == nil {
		t.Fatal("ExpandDynamic returned no dynamic widget")
	}

	if got := len(definition.Dynamic.QueryDefinitions); got != 1 {
		t.Fatalf("expanded query definitions = %d, want 1", got)
	}
	if definition.Dynamic.QueryDefinitions[0].Query.Logs == nil {
		t.Fatal("expanded query definition omitted its logs query")
	}
	if definition.Dynamic.Visualization == nil || definition.Dynamic.Visualization.Stat == nil {
		t.Fatal("expanded dynamic widget omitted its stat visualization")
	}
	if definition.Dynamic.TimeFrame == nil || definition.Dynamic.TimeFrame.RelativeTimeFrame == nil {
		t.Fatal("expanded dynamic widget omitted its relative time frame")
	}

	flattened, diags := FlattenDynamic(ctx, definition.Dynamic)
	if diags.HasError() {
		t.Fatalf("flattening dynamic widget: %v", diags)
	}
	if flattened == nil || flattened.Dynamic == nil {
		t.Fatal("FlattenDynamic returned no dynamic model")
	}

	flattenedObject, diags := types.ObjectValueFrom(ctx, dynamicModelAttr(), flattened.Dynamic)
	if diags.HasError() {
		t.Fatalf("normalizing flattened dynamic model: %v", diags)
	}

	if !originalObject.Equal(flattenedObject) {
		t.Fatalf("dynamic model did not round-trip.\noriginal:  %s\nflattened: %s", originalObject, flattenedObject)
	}
}

func observationFieldObject(keypath, scope string) types.Object {
	return types.ObjectValueMust(ObservationFieldAttr(), map[string]attr.Value{
		"keypath": types.ListValueMust(types.StringType, []attr.Value{types.StringValue(keypath)}),
		"scope":   types.StringValue(scope),
	})
}

func observationFieldList(keypath, scope string) types.List {
	return types.ListValueMust(ObservationFieldsObject(), []attr.Value{observationFieldObject(keypath, scope)})
}

func dynamicThresholdList() types.List {
	return types.ListValueMust(types.ObjectType{AttrTypes: dynamicThresholdAttr()}, []attr.Value{
		types.ObjectValueMust(dynamicThresholdAttr(), map[string]attr.Value{
			"from":  types.Float64Value(10),
			"color": types.StringValue("#ff0000"),
			"label": types.StringValue("high"),
		}),
	})
}

func queryDefinitionsFixture(ctx context.Context, t *testing.T) types.List {
	t.Helper()
	logs := &DynamicQueryLogsModel{
		LuceneQuery:  types.StringValue("level:error"),
		GroupBy:      observationFieldList("service", "user_data"),
		Aggregations: types.ListNull(types.ObjectType{AttrTypes: AggregationModelAttr()}),
		Filters:      types.ListNull(types.ObjectType{AttrTypes: LogsFilterModelAttr()}),
		DataModeType: types.StringValue("archive"),
	}
	return types.ListValueMust(
		types.ObjectType{AttrTypes: dynamicQueryDefinitionModelAttr()},
		[]attr.Value{
			types.ObjectValueMust(dynamicQueryDefinitionModelAttr(), map[string]attr.Value{
				"id":   types.StringValue("query-1"),
				"name": types.StringValue("errors by service"),
				"query": types.ObjectValueMust(dynamicQueryModelAttr(), map[string]attr.Value{
					"logs":       objectFrom(ctx, t, dynamicLogsQueryAttr(), logs),
					"spans":      types.ObjectNull(dynamicSpansQueryAttr()),
					"metrics":    types.ObjectNull(dynamicMetricsQueryAttr()),
					"data_prime": types.ObjectNull(dynamicDataPrimeQueryAttr()),
				}),
			}),
		},
	)
}

func assertDynamicRoundTrip(ctx context.Context, t *testing.T, original *DynamicModel) {
	t.Helper()

	originalObject, diags := types.ObjectValueFrom(ctx, dynamicModelAttr(), original)
	if diags.HasError() {
		t.Fatalf("normalizing original dynamic model: %v", diags)
	}

	definition, diags := ExpandDynamic(ctx, original)
	if diags.HasError() {
		t.Fatalf("expanding dynamic model: %v", diags)
	}
	if definition == nil || definition.Dynamic == nil {
		t.Fatal("ExpandDynamic returned no dynamic widget")
	}

	flattened, diags := FlattenDynamic(ctx, definition.Dynamic)
	if diags.HasError() {
		t.Fatalf("flattening dynamic widget: %v", diags)
	}
	if flattened == nil || flattened.Dynamic == nil {
		t.Fatal("FlattenDynamic returned no dynamic model")
	}

	flattenedObject, diags := types.ObjectValueFrom(ctx, dynamicModelAttr(), flattened.Dynamic)
	if diags.HasError() {
		t.Fatalf("normalizing flattened dynamic model: %v", diags)
	}

	if !originalObject.Equal(flattenedObject) {
		t.Fatalf("dynamic model did not round-trip.\noriginal:  %s\nflattened: %s", originalObject, flattenedObject)
	}
}

func TestDynamicWidgetStatFullFidelityRoundTrip(t *testing.T) {
	ctx := context.Background()

	legend := &LegendModel{
		IsVisible:    types.BoolValue(true),
		Columns:      types.ListValueMust(types.StringType, []attr.Value{types.StringValue("min")}),
		GroupByQuery: types.BoolValue(true),
		Placement:    types.StringValue("bottom"),
	}

	original := &DynamicModel{
		QueryDefinitions: queryDefinitionsFixture(ctx, t),
		TimeFrame: &TimeFrameModel{
			Relative: &TimeFrameRelativeModel{Duration: types.StringValue("seconds:900")},
		},
		Visualization: &DynamicVisualizationModel{
			Stat: &DynamicStatModel{
				AllowAbbreviation: types.BoolValue(true),
				CategoryFields:    observationFieldList("category", "user_data"),
				CustomUnit:        types.StringValue("widgets"),
				DecimalPrecision:  types.Int64Value(3),
				DisplaySeriesName: types.BoolValue(true),
				Legend:            legend,
				LegendBy:          types.StringValue("thresholds"),
				Max:               types.Float64Value(100),
				Min:               types.Float64Value(0),
				ThresholdBy:       types.StringValue("background"),
				ThresholdType:     types.StringValue("absolute"),
				Thresholds:        dynamicThresholdList(),
				Unit:              types.StringValue("bytes"),
				ValueField:        observationFieldObject("duration", "metadata"),
				ValueFields:       observationFieldList("duration2", "metadata"),
			},
		},
	}

	assertDynamicRoundTrip(ctx, t, original)
}

func TestDynamicWidgetStatCardFullFidelityRoundTrip(t *testing.T) {
	ctx := context.Background()

	legend := &LegendModel{
		IsVisible:    types.BoolValue(true),
		Columns:      types.ListValueMust(types.StringType, []attr.Value{types.StringValue("max")}),
		GroupByQuery: types.BoolValue(false),
		Placement:    types.StringValue("bottom"),
	}

	templateVariables := types.ListValueMust(types.ObjectType{AttrTypes: dynamicTemplateVariableAttr()}, []attr.Value{
		types.ObjectValueMust(dynamicTemplateVariableAttr(), map[string]attr.Value{
			"mapped_values":     types.StringValue(`{"a":"1","b":"2"}`),
			"observation_field": observationFieldObject("tvar", "user_data"),
		}),
	})

	visualElement := func(name string) *DynamicStatVisualElementModel {
		return &DynamicStatVisualElementModel{
			MappedValues:      types.StringValue(`{"x":"y"}`),
			ObservationField:  observationFieldObject(name, "user_data"),
			TemplateText:      types.StringValue("text-" + name),
			TemplateVariables: templateVariables,
		}
	}

	sections := types.ListValueMust(types.ObjectType{AttrTypes: dynamicMappingSectionAttr()}, []attr.Value{
		types.ObjectValueMust(dynamicMappingSectionAttr(), map[string]attr.Value{
			"color":  types.StringValue("green"),
			"map_to": types.StringValue("OK"),
			"value":  types.StringValue("200"),
		}),
	})

	original := &DynamicModel{
		QueryDefinitions: queryDefinitionsFixture(ctx, t),
		TimeFrame: &TimeFrameModel{
			Relative: &TimeFrameRelativeModel{Duration: types.StringValue("seconds:900")},
		},
		Visualization: &DynamicVisualizationModel{
			StatCard: &DynamicStatCardModel{
				AllowAbbreviation: types.BoolValue(true),
				CategoryFields:    observationFieldList("category", "user_data"),
				ColorLabelMapping: &DynamicColorLabelMappingModel{
					ColorBy: types.StringValue("background"),
					Range: &DynamicRangeMappingModel{
						MinMax: &DynamicMinMaxModel{
							Auto: types.BoolNull(),
							Custom: &DynamicMinMaxCustomModel{
								Max: types.Float64Value(50),
								Min: types.Float64Value(5),
							},
						},
						ThresholdType: types.StringValue("relative"),
						Thresholds:    dynamicThresholdList(),
					},
					Regex: &DynamicSectionsMappingModel{Sections: sections},
					Value: &DynamicSectionsMappingModel{Sections: sections},
				},
				CustomUnit:       types.StringValue("cards"),
				DecimalPrecision: types.Int64Value(2),
				Label:            visualElement("label"),
				Legend:           legend,
				LegendBy:         types.StringValue("groups"),
				PrimaryValue:     visualElement("primary"),
				Title:            visualElement("title"),
				Unit:             types.StringValue("usd"),
				ValueFields:      observationFieldList("value", "metadata"),
			},
		},
	}

	assertDynamicRoundTrip(ctx, t, original)
}

func TestDynamicWidgetStatCardMinMaxAutoRoundTrip(t *testing.T) {
	ctx := context.Background()

	original := &DynamicModel{
		QueryDefinitions: queryDefinitionsFixture(ctx, t),
		TimeFrame: &TimeFrameModel{
			Relative: &TimeFrameRelativeModel{Duration: types.StringValue("seconds:900")},
		},
		Visualization: &DynamicVisualizationModel{
			StatCard: &DynamicStatCardModel{
				AllowAbbreviation: types.BoolNull(),
				CategoryFields:    types.ListNull(ObservationFieldsObject()),
				ColorLabelMapping: &DynamicColorLabelMappingModel{
					ColorBy: types.StringValue("unspecified"),
					Range: &DynamicRangeMappingModel{
						MinMax: &DynamicMinMaxModel{
							Auto:   types.BoolValue(true),
							Custom: nil,
						},
						ThresholdType: types.StringValue("unspecified"),
						Thresholds:    types.ListNull(types.ObjectType{AttrTypes: dynamicThresholdAttr()}),
					},
					Regex: nil,
					Value: nil,
				},
				CustomUnit:       types.StringNull(),
				DecimalPrecision: types.Int64Null(),
				Label:            nil,
				Legend:           nil,
				LegendBy:         types.StringValue("unspecified"),
				PrimaryValue:     nil,
				Title:            nil,
				Unit:             types.StringValue("unspecified"),
				ValueFields:      types.ListNull(ObservationFieldsObject()),
			},
		},
	}

	assertDynamicRoundTrip(ctx, t, original)
}

func TestDynamicWidgetTableFullFidelityRoundTrip(t *testing.T) {
	ctx := context.Background()

	columns := types.ListValueMust(types.ObjectType{AttrTypes: dynamicTableColumnModelAttr()}, []attr.Value{
		types.ObjectValueMust(dynamicTableColumnModelAttr(), map[string]attr.Value{
			"field": observationFieldObject("status", "user_data"),
		}),
		types.ObjectValueMust(dynamicTableColumnModelAttr(), map[string]attr.Value{
			"field": observationFieldObject("duration", "metadata"),
		}),
	})

	linkActions := types.ListValueMust(types.ObjectType{AttrTypes: dynamicTableLinkActionModelAttr()}, []attr.Value{
		types.ObjectValueMust(dynamicTableLinkActionModelAttr(), map[string]attr.Value{
			"id":                        types.StringValue("11111111-1111-1111-1111-111111111111"),
			"name":                      types.StringValue("open trace"),
			"should_open_in_new_window": types.BoolValue(true),
			"url":                       types.StringValue("https://example.com/{{traceId}}"),
		}),
	})

	valuesMappings := types.ListValueMust(types.ObjectType{AttrTypes: dynamicTableValueMappingModelAttr()}, []attr.Value{
		types.ObjectValueMust(dynamicTableValueMappingModelAttr(), map[string]attr.Value{
			"input_value":   types.StringValue("200"),
			"replace_value": types.StringValue("OK"),
			"type":          types.StringValue("value"),
		}),
	})

	definition := &DynamicTablePropertyDefinitionModel{
		Alignment:         types.StringValue("center"),
		ColumnDisplayName: types.StringValue("Status Code"),
		Link:              &DynamicTablePropertyLinkModel{Actions: linkActions},
		RegexExtract:      types.StringValue(`(\d+)`),
		Thresholds: &DynamicTablePropertyThresholdsModel{
			Max:    types.Float64Value(100),
			Min:    types.Float64Value(0),
			Type:   types.StringValue("absolute"),
			Values: dynamicThresholdList(),
		},
		Units: &DynamicTablePropertyUnitsModel{
			AllowAbbreviation: types.BoolValue(true),
			CustomUnit:        types.StringValue("reqs"),
			DecimalPrecision:  types.Int64Value(2),
			Max:               types.Float64Value(100),
			Min:               types.Float64Value(0),
			Unit:              types.StringValue("bytes"),
		},
		ValuesAlias:   types.StringValue("code"),
		ValuesMapping: &DynamicTablePropertyValuesMappingModel{Mappings: valuesMappings},
	}

	properties := types.ListValueMust(types.ObjectType{AttrTypes: dynamicTablePropertyModelAttr()}, []attr.Value{
		objectFrom(ctx, t, dynamicTablePropertyModelAttr(), &DynamicTablePropertyModel{
			ID:         types.StringValue("22222222-2222-2222-2222-222222222222"),
			Definition: definition,
		}),
	})

	rules := types.ListValueMust(types.ObjectType{AttrTypes: dynamicTableRuleModelAttr()}, []attr.Value{
		objectFrom(ctx, t, dynamicTableRuleModelAttr(), &DynamicTableRuleModel{
			Description: types.StringValue("color errors"),
			ID:          types.StringValue("33333333-3333-3333-3333-333333333333"),
			Name:        types.StringValue("errors rule"),
			Properties:  properties,
			RuleScope: &DynamicTableRuleScopeModel{
				Field:     observationFieldObject("status", "user_data"),
				FieldType: types.StringValue("number"),
				Regex:     types.StringValue("5.."),
			},
		}),
	})

	columnWidths := types.ListValueMust(types.ObjectType{AttrTypes: dynamicTableColumnWidthModelAttr()}, []attr.Value{
		types.ObjectValueMust(dynamicTableColumnWidthModelAttr(), map[string]attr.Value{
			"column_name": types.StringValue("status"),
			"width":       types.Int64Value(120),
		}),
	})

	original := &DynamicModel{
		QueryDefinitions: queryDefinitionsFixture(ctx, t),
		TimeFrame: &TimeFrameModel{
			Relative: &TimeFrameRelativeModel{Duration: types.StringValue("seconds:900")},
		},
		Visualization: &DynamicVisualizationModel{
			Table: &DynamicTableModel{
				Columns: columns,
				Rules:   rules,
				Settings: &DynamicTableSettingsModel{
					ColumnWidths: columnWidths,
					RowStyle:     types.StringValue("two_line"),
				},
			},
		},
	}

	assertDynamicRoundTrip(ctx, t, original)
}

func TestDynamicWidgetTableEmptyListsFlattenToNull(t *testing.T) {
	ctx := context.Background()

	columns, diags := flattenDynamicTableColumns(ctx, nil)
	if diags.HasError() {
		t.Fatalf("flattening empty columns: %v", diags)
	}
	if !columns.IsNull() {
		t.Fatalf("empty columns should flatten to null, got %s", columns)
	}

	rules, diags := flattenDynamicTableRules(ctx, nil)
	if diags.HasError() {
		t.Fatalf("flattening empty rules: %v", diags)
	}
	if !rules.IsNull() {
		t.Fatalf("empty rules should flatten to null, got %s", rules)
	}
}

func TestDynamicMappedValuesPreservesEquivalentJSON(t *testing.T) {
	ctx := context.Background()
	modifier := utils.PreserveStateForEquivalentJSON{}

	state := types.StringValue(`{"a":"1","b":"2"}`)
	equivalentConfig := types.StringValue("{\n  \"b\": \"2\",\n  \"a\": \"1\"\n}")
	resp := &planmodifier.StringResponse{PlanValue: equivalentConfig}
	modifier.PlanModifyString(ctx, planmodifier.StringRequest{
		ConfigValue: equivalentConfig,
		StateValue:  state,
		PlanValue:   equivalentConfig,
	}, resp)
	if !resp.PlanValue.Equal(state) {
		t.Fatalf("equivalent JSON should preserve state.\nstate: %s\nplan:  %s", state, resp.PlanValue)
	}

	differentConfig := types.StringValue(`{"a":"9"}`)
	resp2 := &planmodifier.StringResponse{PlanValue: differentConfig}
	modifier.PlanModifyString(ctx, planmodifier.StringRequest{
		ConfigValue: differentConfig,
		StateValue:  state,
		PlanValue:   differentConfig,
	}, resp2)
	if !resp2.PlanValue.Equal(differentConfig) {
		t.Fatalf("non-equivalent JSON should keep the configured plan value, got %s", resp2.PlanValue)
	}
}

func objectFrom(ctx context.Context, t *testing.T, attrTypes map[string]attr.Type, value any) types.Object {
	t.Helper()
	object, diags := types.ObjectValueFrom(ctx, attrTypes, value)
	if diags.HasError() {
		t.Fatalf("building object value: %v", diags)
	}
	return object
}
