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

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
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

func TestDynamicWidgetMetricsStatFullFidelityRoundTrip(t *testing.T) {
	ctx := context.Background()

	metricsQuery := &DynamicQueryMetricsModel{
		PromqlQuery:     types.StringValue("rate(x[5m])"),
		PromqlQueryType: types.StringValue("instant"),
		EditorMode:      types.StringValue("text"),
		SeriesLimitType: types.StringValue("by_series_count"),
	}

	queryDefinitions := types.ListValueMust(
		types.ObjectType{AttrTypes: dynamicQueryDefinitionModelAttr()},
		[]attr.Value{
			types.ObjectValueMust(dynamicQueryDefinitionModelAttr(), map[string]attr.Value{
				"id":   types.StringValue("query-1"),
				"name": types.StringValue("rate by service"),
				"query": types.ObjectValueMust(dynamicQueryModelAttr(), map[string]attr.Value{
					"logs":       types.ObjectNull(dynamicLogsQueryAttr()),
					"spans":      types.ObjectNull(dynamicSpansQueryAttr()),
					"metrics":    objectFrom(ctx, t, dynamicMetricsQueryAttr(), metricsQuery),
					"data_prime": types.ObjectNull(dynamicDataPrimeQueryAttr()),
				}),
			}),
		},
	)

	original := &DynamicModel{
		QueryDefinitions: queryDefinitions,
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
				Thresholds:        dynamicThresholdList(),
				Unit:              types.StringValue("bytes"),
				ValueField:        observationFieldObject("duration", "metadata"),
				ValueFields:       types.ListNull(ObservationFieldsObject()),
			},
		},
	}

	assertDynamicRoundTrip(ctx, t, original)
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
		Interpretation:   types.StringValue("single_value_kpi_stat"),
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
			"mapped_values":     jsonStringTestValue(`{"a":"1","b":"2"}`),
			"observation_field": observationFieldObject("tvar", "user_data"),
		}),
	})

	visualElement := func(name string) *DynamicStatVisualElementModel {
		return &DynamicStatVisualElementModel{
			MappedValues:      jsonStringTestValue(`{"x":"y"}`),
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

func TestDynamicWidgetTimeSeriesLinesMultiFullFidelityRoundTrip(t *testing.T) {
	ctx := context.Background()

	legend := &LegendModel{
		IsVisible:    types.BoolValue(true),
		Columns:      types.ListValueMust(types.StringType, []attr.Value{types.StringValue("avg")}),
		GroupByQuery: types.BoolValue(true),
		Placement:    types.StringValue("side"),
	}

	queryDisplaySettings := types.ListValueMust(types.ObjectType{AttrTypes: dynamicQueryDisplaySettingsModelAttr()}, []attr.Value{
		objectFrom(ctx, t, dynamicQueryDisplaySettingsModelAttr(), &DynamicQueryDisplaySettingsModel{
			AllowAbbreviation:  types.BoolValue(true),
			CategoryFields:     observationFieldList("category", "user_data"),
			ColorScheme:        types.StringValue("classic"),
			CustomUnit:         types.StringValue("reqs"),
			DecimalPrecision:   types.Int64Value(2),
			HashColors:         types.BoolValue(true),
			QueryID:            types.StringValue("query-1"),
			ScaleType:          types.StringValue("logarithmic"),
			SeriesCountLimit:   types.Int64Value(100),
			SeriesNameTemplate: types.StringValue("{{label}}"),
			TemporalField:      observationFieldObject("timestamp", "metadata"),
			Unit:               types.StringValue("bytes"),
			ValueFields:        observationFieldList("value", "metadata"),
			YAxisMax:           float32TestValue(100),
			YAxisMin:           float32TestValue(0),
		}),
	})

	original := &DynamicModel{
		QueryDefinitions: queryDefinitionsFixture(ctx, t),
		TimeFrame: &TimeFrameModel{
			Relative: &TimeFrameRelativeModel{Duration: types.StringValue("seconds:900")},
		},
		Visualization: &DynamicVisualizationModel{
			TimeSeriesLinesMulti: &DynamicTimeSeriesLinesMultiModel{
				ConnectNulls:         types.BoolValue(true),
				Legend:               legend,
				QueryDisplaySettings: queryDisplaySettings,
				StackedLine:          types.StringValue("absolute"),
				Tooltip: &DynamicTimeSeriesTooltipModel{
					ShowAllSeries: types.BoolValue(true),
					ShowLabels:    types.BoolValue(false),
				},
				UseDataTimeRange: types.BoolValue(true),
				XAxisTimeFormat:  types.StringValue("dd_mm_hh_mm"),
			},
		},
	}

	assertDynamicRoundTrip(ctx, t, original)
}

func TestDynamicWidgetTimeSeriesLinesFullFidelityRoundTrip(t *testing.T) {
	ctx := context.Background()

	legend := &LegendModel{
		IsVisible:    types.BoolValue(true),
		Columns:      types.ListValueMust(types.StringType, []attr.Value{types.StringValue("avg")}),
		GroupByQuery: types.BoolValue(true),
		Placement:    types.StringValue("side"),
	}

	original := &DynamicModel{
		QueryDefinitions: queryDefinitionsFixture(ctx, t),
		TimeFrame: &TimeFrameModel{
			Relative: &TimeFrameRelativeModel{Duration: types.StringValue("seconds:900")},
		},
		Visualization: &DynamicVisualizationModel{
			TimeSeriesLines: &DynamicTimeSeriesLinesModel{
				AllowAbbreviation:  types.BoolValue(true),
				CategoryFields:     observationFieldList("category", "user_data"),
				ColorScheme:        types.StringValue("classic"),
				ConnectNulls:       types.BoolValue(true),
				CustomUnit:         types.StringValue("reqs"),
				DecimalPrecision:   types.Int64Value(4),
				HashColors:         types.BoolValue(true),
				Legend:             legend,
				ScaleType:          types.StringValue("logarithmic"),
				SeriesCountLimit:   types.Int64Value(100),
				SeriesNameTemplate: types.StringValue("{{label}}"),
				StackedLine:        types.StringValue("absolute"),
				TemporalField:      observationFieldObject("timestamp", "metadata"),
				Tooltip: &DynamicTimeSeriesTooltipModel{
					ShowAllSeries: types.BoolValue(true),
					ShowLabels:    types.BoolValue(false),
				},
				Unit:             types.StringValue("bytes"),
				UseDataTimeRange: types.BoolValue(true),
				ValueFields:      observationFieldList("value", "metadata"),
				XAxisTimeFormat:  types.StringValue("dd_mm_hh_mm"),
				YAxisMax:         float32TestValue(100),
				YAxisMin:         float32TestValue(0),
			},
		},
	}

	assertDynamicRoundTrip(ctx, t, original)
}

func TestDynamicWidgetTimeSeriesBarsFullFidelityRoundTrip(t *testing.T) {
	ctx := context.Background()

	legend := &LegendModel{
		IsVisible:    types.BoolValue(true),
		Columns:      types.ListValueMust(types.StringType, []attr.Value{types.StringValue("sum")}),
		GroupByQuery: types.BoolValue(false),
		Placement:    types.StringValue("bottom"),
	}

	original := &DynamicModel{
		QueryDefinitions: queryDefinitionsFixture(ctx, t),
		TimeFrame: &TimeFrameModel{
			Relative: &TimeFrameRelativeModel{Duration: types.StringValue("seconds:900")},
		},
		Visualization: &DynamicVisualizationModel{
			TimeSeriesBars: &DynamicTimeSeriesBarsModel{
				AllowAbbreviation:  types.BoolValue(true),
				BarValueDisplay:    types.StringValue("both"),
				CategoryFields:     observationFieldList("category", "user_data"),
				ColorScheme:        types.StringValue("classic"),
				CustomUnit:         types.StringValue("reqs"),
				DecimalPrecision:   types.Int64Value(3),
				HashColors:         types.BoolValue(true),
				Legend:             legend,
				MaxSlicesPerBar:    types.Int64Value(10),
				ScaleType:          types.StringValue("linear"),
				SeriesNameTemplate: types.StringValue("{{label}}"),
				SortBy:             types.StringValue("value"),
				TemporalField:      observationFieldObject("timestamp", "metadata"),
				Tooltip: &DynamicTimeSeriesTooltipModel{
					ShowAllSeries: types.BoolValue(false),
					ShowLabels:    types.BoolValue(true),
				},
				Unit:            types.StringValue("usd"),
				ValueFields:     observationFieldList("value", "metadata"),
				XAxisTimeFormat: types.StringValue("hh_mm"),
				YAxisMax:        float32TestValue(100),
				YAxisMin:        float32TestValue(0),
			},
		},
	}

	assertDynamicRoundTrip(ctx, t, original)
}

func barsColorsBy() types.String {
	return types.StringValue("stack")
}

func barsQueryFieldSettingsList() types.List {
	return types.ListValueMust(types.ObjectType{AttrTypes: dynamicBarsQueryFieldSettingsModelAttr()}, []attr.Value{
		types.ObjectValueMust(dynamicBarsQueryFieldSettingsModelAttr(), map[string]attr.Value{
			"query_id":    types.StringValue("query-1"),
			"value_field": observationFieldObject("value", "metadata"),
		}),
	})
}

func barsSortOrder() *DynamicSortOrderModel {
	return &DynamicSortOrderModel{
		OrderDirection: types.StringValue("asc"),
		Strategy: &DynamicSortStrategyModel{
			Category:     jsonStringTestValue(`{"c":"x"}`),
			QueryValue:   &DynamicSortByQueryValueModel{QueryID: types.StringValue("query-1")},
			StrategyType: types.StringValue("by_value"),
		},
	}
}

func barsLegend() *LegendModel {
	return &LegendModel{
		IsVisible:    types.BoolValue(true),
		Columns:      types.ListValueMust(types.StringType, []attr.Value{types.StringValue("sum")}),
		GroupByQuery: types.BoolValue(false),
		Placement:    types.StringValue("bottom"),
	}
}

func TestDynamicWidgetVerticalBarsFullFidelityRoundTrip(t *testing.T) {
	ctx := context.Background()

	original := &DynamicModel{
		QueryDefinitions: queryDefinitionsFixture(ctx, t),
		TimeFrame: &TimeFrameModel{
			Relative: &TimeFrameRelativeModel{Duration: types.StringValue("seconds:900")},
		},
		Visualization: &DynamicVisualizationModel{
			VerticalBars: &DynamicVerticalBarsModel{
				AllowAbbreviation: types.BoolValue(true),
				BarValueDisplay:   types.StringValue("both"),
				CategoryFields:    observationFieldList("category", "user_data"),
				ColorScheme:       types.StringValue("classic"),
				ColorsBy:          barsColorsBy(),
				CustomUnit:        types.StringValue("reqs"),
				DecimalPrecision:  types.Int64Value(3),
				GroupNameTemplate: types.StringValue("{{group}}"),
				HashColors:        types.BoolValue(true),
				Legend:            barsLegend(),
				MaxBarsPerChart:   types.Int64Value(50),
				MaxSlicesPerBar:   types.Int64Value(10),
				ScaleType:         types.StringValue("linear"),
				SortBy:            types.StringValue("value"),
				StackNameTemplate: types.StringValue("{{stack}}"),
				SubCategoryFields: observationFieldList("sub", "label"),
				Unit:              types.StringValue("usd"),
				ValueField:        observationFieldObject("value", "metadata"),
				YAxisMax:          float32TestValue(100),
				YAxisMin:          float32TestValue(0),
			},
		},
	}

	assertDynamicRoundTrip(ctx, t, original)
}

func TestDynamicWidgetVerticalBarsMultiFullFidelityRoundTrip(t *testing.T) {
	ctx := context.Background()

	original := &DynamicModel{
		QueryDefinitions: queryDefinitionsFixture(ctx, t),
		TimeFrame: &TimeFrameModel{
			Relative: &TimeFrameRelativeModel{Duration: types.StringValue("seconds:900")},
		},
		Visualization: &DynamicVisualizationModel{
			VerticalBarsMulti: &DynamicVerticalBarsMultiModel{
				AllowAbbreviation:  types.BoolValue(true),
				BarValueDisplay:    types.StringValue("top"),
				CategoryFields:     observationFieldList("category", "user_data"),
				ColorScheme:        types.StringValue("classic"),
				ColorsBy:           barsColorsBy(),
				CustomUnit:         types.StringValue("reqs"),
				DecimalPrecision:   types.Int64Value(2),
				GroupNameTemplate:  types.StringValue("{{group}}"),
				HashColors:         types.BoolValue(false),
				Legend:             barsLegend(),
				MaxBarsPerChart:    types.Int64Value(25),
				QueryFieldSettings: barsQueryFieldSettingsList(),
				ScaleType:          types.StringValue("logarithmic"),
				SortOrder:          barsSortOrder(),
				Unit:               types.StringValue("bytes"),
				YAxisMax:           float32TestValue(100),
				YAxisMin:           float32TestValue(0),
			},
		},
	}

	assertDynamicRoundTrip(ctx, t, original)
}

func TestDynamicWidgetHorizontalBarsFullFidelityRoundTrip(t *testing.T) {
	ctx := context.Background()

	original := &DynamicModel{
		QueryDefinitions: queryDefinitionsFixture(ctx, t),
		TimeFrame: &TimeFrameModel{
			Relative: &TimeFrameRelativeModel{Duration: types.StringValue("seconds:900")},
		},
		Visualization: &DynamicVisualizationModel{
			HorizontalBars: &DynamicHorizontalBarsModel{
				AllowAbbreviation: types.BoolValue(true),
				CategoryFields:    observationFieldList("category", "user_data"),
				ColorScheme:       types.StringValue("classic"),
				ColorsBy:          barsColorsBy(),
				CustomUnit:        types.StringValue("reqs"),
				DecimalPrecision:  types.Int64Value(3),
				DisplayOnBar:      types.BoolValue(true),
				GroupNameTemplate: types.StringValue("{{group}}"),
				HashColors:        types.BoolValue(true),
				Legend:            barsLegend(),
				MaxBarsPerChart:   types.Int64Value(50),
				MaxSlicesPerBar:   types.Int64Value(10),
				ScaleType:         types.StringValue("linear"),
				SortBy:            types.StringValue("name"),
				StackNameTemplate: types.StringValue("{{stack}}"),
				SubCategoryFields: observationFieldList("sub", "label"),
				Unit:              types.StringValue("usd"),
				ValueField:        observationFieldObject("value", "metadata"),
				YAxisMax:          float32TestValue(100),
				YAxisMin:          float32TestValue(0),
				YAxisViewBy:       types.StringValue("category"),
			},
		},
	}

	assertDynamicRoundTrip(ctx, t, original)
}

func TestDynamicWidgetHorizontalBarsMultiFullFidelityRoundTrip(t *testing.T) {
	ctx := context.Background()

	original := &DynamicModel{
		QueryDefinitions: queryDefinitionsFixture(ctx, t),
		TimeFrame: &TimeFrameModel{
			Relative: &TimeFrameRelativeModel{Duration: types.StringValue("seconds:900")},
		},
		Visualization: &DynamicVisualizationModel{
			HorizontalBarsMulti: &DynamicHorizontalBarsMultiModel{
				AllowAbbreviation:  types.BoolValue(true),
				CategoryFields:     observationFieldList("category", "user_data"),
				ColorScheme:        types.StringValue("classic"),
				ColorsBy:           barsColorsBy(),
				CustomUnit:         types.StringValue("reqs"),
				DecimalPrecision:   types.Int64Value(2),
				DisplayOnBar:       types.BoolValue(false),
				GroupNameTemplate:  types.StringValue("{{group}}"),
				HashColors:         types.BoolValue(false),
				Legend:             barsLegend(),
				MaxBarsPerChart:    types.Int64Value(25),
				QueryFieldSettings: barsQueryFieldSettingsList(),
				ScaleType:          types.StringValue("logarithmic"),
				SortOrder:          barsSortOrder(),
				Unit:               types.StringValue("bytes"),
				YAxisMax:           float32TestValue(100),
				YAxisMin:           float32TestValue(0),
				YAxisViewBy:        types.StringValue("value"),
			},
		},
	}

	assertDynamicRoundTrip(ctx, t, original)
}

func TestDynamicWidgetGaugeFullFidelityRoundTrip(t *testing.T) {
	ctx := context.Background()

	legend := &LegendModel{
		IsVisible:    types.BoolValue(true),
		Columns:      types.ListValueMust(types.StringType, []attr.Value{types.StringValue("avg")}),
		GroupByQuery: types.BoolValue(true),
		Placement:    types.StringValue("bottom"),
	}

	original := &DynamicModel{
		QueryDefinitions: queryDefinitionsFixture(ctx, t),
		TimeFrame: &TimeFrameModel{
			Relative: &TimeFrameRelativeModel{Duration: types.StringValue("seconds:900")},
		},
		Visualization: &DynamicVisualizationModel{
			Gauge: &DynamicGaugeModel{
				AllowAbbreviation: types.BoolValue(true),
				ArcDisplay: &DynamicArcDisplayModel{
					ThresholdArc: types.BoolValue(true),
					ValueArc:     types.BoolValue(false),
				},
				CategoryFields:    observationFieldList("category", "user_data"),
				CustomUnit:        types.StringValue("gauges"),
				DecimalPrecision:  types.Int64Value(2),
				DisplaySeriesName: types.BoolValue(true),
				Legend:            legend,
				LegendBy:          types.StringValue("thresholds"),
				Max:               types.Float64Value(100),
				Min:               types.Float64Value(0),
				ShowInnerArc:      types.BoolValue(true),
				ShowMinMax:        types.BoolValue(true),
				ShowOuterArc:      types.BoolValue(false),
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

func TestDynamicWidgetPieChartFullFidelityRoundTrip(t *testing.T) {
	ctx := context.Background()

	legend := &LegendModel{
		IsVisible:    types.BoolValue(true),
		Columns:      types.ListValueMust(types.StringType, []attr.Value{types.StringValue("sum")}),
		GroupByQuery: types.BoolValue(false),
		Placement:    types.StringValue("side"),
	}

	original := &DynamicModel{
		QueryDefinitions: queryDefinitionsFixture(ctx, t),
		TimeFrame: &TimeFrameModel{
			Relative: &TimeFrameRelativeModel{Duration: types.StringValue("seconds:900")},
		},
		Visualization: &DynamicVisualizationModel{
			PieChart: &DynamicPieChartModel{
				AllowAbbreviation: types.BoolValue(true),
				CategoryFields:    observationFieldList("category", "user_data"),
				ColorScheme:       types.StringValue("classic"),
				CustomUnit:        types.StringValue("slices"),
				DecimalPrecision:  types.Int64Value(2),
				GroupNameTemplate: types.StringValue("{{group}}"),
				HashColors:        types.BoolValue(true),
				LabelDefinition: &DynamicPieChartLabelDefinitionModel{
					IsVisible:      types.BoolValue(true),
					LabelSource:    types.StringValue("inner"),
					ShowName:       types.BoolValue(true),
					ShowPercentage: types.BoolValue(false),
					ShowValue:      types.BoolValue(true),
				},
				Legend:             legend,
				MaxSlicesPerChart:  types.Int64Value(10),
				MaxSlicesPerStack:  types.Int64Value(5),
				MinSlicePercentage: types.Int64Value(1),
				ShowTotal:          types.BoolValue(true),
				StackNameTemplate:  types.StringValue("{{stack}}"),
				SubCategoryFields:  observationFieldList("sub", "label"),
				Unit:               types.StringValue("usd"),
				ValueField:         observationFieldObject("value", "metadata"),
			},
		},
	}

	assertDynamicRoundTrip(ctx, t, original)
}

func TestDynamicWidgetHexagonBinsFullFidelityRoundTrip(t *testing.T) {
	ctx := context.Background()

	legend := &LegendModel{
		IsVisible:    types.BoolValue(true),
		Columns:      types.ListValueMust(types.StringType, []attr.Value{types.StringValue("avg")}),
		GroupByQuery: types.BoolValue(true),
		Placement:    types.StringValue("bottom"),
	}

	original := &DynamicModel{
		QueryDefinitions: queryDefinitionsFixture(ctx, t),
		TimeFrame: &TimeFrameModel{
			Relative: &TimeFrameRelativeModel{Duration: types.StringValue("seconds:900")},
		},
		Visualization: &DynamicVisualizationModel{
			HexagonBins: &DynamicHexagonBinsModel{
				AllowAbbreviation: types.BoolValue(true),
				CategoryFields:    observationFieldList("category", "user_data"),
				CustomUnit:        types.StringValue("hexes"),
				DecimalPrecision:  types.Int64Value(2),
				Legend:            legend,
				LegendBy:          types.StringValue("thresholds"),
				Max:               types.Float64Value(100),
				Min:               types.Float64Value(0),
				ThresholdType:     types.StringValue("absolute"),
				Thresholds:        dynamicThresholdList(),
				Unit:              types.StringValue("bytes"),
				ValueField:        observationFieldObject("duration", "metadata"),
			},
		},
	}

	assertDynamicRoundTrip(ctx, t, original)
}

func TestDynamicWidgetHeatmapFullFidelityRoundTrip(t *testing.T) {
	ctx := context.Background()

	original := &DynamicModel{
		QueryDefinitions: queryDefinitionsFixture(ctx, t),
		TimeFrame: &TimeFrameModel{
			Relative: &TimeFrameRelativeModel{Duration: types.StringValue("seconds:900")},
		},
		Visualization: &DynamicVisualizationModel{
			Heatmap: &DynamicHeatmapModel{
				AllowAbbreviation:   types.BoolValue(true),
				ColorAxisMax:        float32TestValue(100),
				ColorAxisMin:        float32TestValue(0),
				ColorRange:          types.StringValue("blue_reversed"),
				CustomUnit:          types.StringValue("cells"),
				DecimalPrecision:    types.Int64Value(3),
				HistogramBucketUnit: types.StringValue("milliseconds"),
				Preset:              types.StringValue("green_reversed"),
				ScaleType:           types.StringValue("linear"),
				ShowNumbers:         types.BoolValue(true),
				Tooltip: &DynamicHeatmapTooltipModel{
					Labels:          observationFieldList("tip", "user_data"),
					MessageTemplate: types.StringValue("{{value}}"),
				},
				Unit:            types.StringValue("usd"),
				ValueField:      observationFieldObject("value", "metadata"),
				XAxisFields:     observationFieldList("x", "user_data"),
				XAxisTimeFormat: types.StringValue("hh_mm"),
				YAxisFields:     observationFieldList("y", "user_data"),
			},
		},
	}

	assertDynamicRoundTrip(ctx, t, original)
}

func TestDynamicWidgetGeomapFullFidelityRoundTrip(t *testing.T) {
	ctx := context.Background()

	fieldBased := func(name string) *DynamicGeomapAggregationFieldBasedModel {
		return &DynamicGeomapAggregationFieldBasedModel{
			Field: observationFieldObject(name, "metadata"),
		}
	}

	original := &DynamicModel{
		QueryDefinitions: queryDefinitionsFixture(ctx, t),
		TimeFrame: &TimeFrameModel{
			Relative: &TimeFrameRelativeModel{Duration: types.StringValue("seconds:900")},
		},
		Visualization: &DynamicVisualizationModel{
			Geomap: &DynamicGeomapModel{
				Aggregation: &DynamicGeomapAggregationModel{
					Avg: fieldBased("avg_field"),
				},
				AllowAbbreviation: types.BoolValue(true),
				Color: &DynamicGeomapColorModel{
					ColorRange: types.StringValue("red_reversed"),
				},
				Config: &DynamicGeomapFieldConfigModel{
					CoordinateConfig: &DynamicGeomapCoordinateConfigModel{
						LatitudeField:  observationFieldObject("lat", "metadata"),
						LongitudeField: observationFieldObject("lon", "metadata"),
					},
				},
				CustomUnit:       types.StringValue("points"),
				DecimalPrecision: types.Int64Value(2),
				MinMax: &DynamicMinMaxModel{
					Auto: types.BoolNull(),
					Custom: &DynamicMinMaxCustomModel{
						Max: types.Float64Value(50),
						Min: types.Float64Value(5),
					},
				},
				Tooltip: &DynamicGeomapTooltipModel{
					Labels:          observationFieldList("tip", "user_data"),
					MessageTemplate: types.StringValue("{{lat}},{{lon}}"),
				},
				Unit: types.StringValue("usd"),
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

func jsonStringTestValue(s string) JSONStringValue {
	return NewJSONStringValue(s)
}

func TestJSONStringValueSemanticEquals(t *testing.T) {
	ctx := context.Background()

	for _, tc := range []struct {
		name  string
		left  JSONStringValue
		right JSONStringValue
		want  bool
	}{
		{
			name:  "key order differs",
			left:  jsonStringTestValue(`{"b":1,"a":2}`),
			right: jsonStringTestValue(`{"a":2,"b":1}`),
			want:  true,
		},
		{
			name:  "pretty printed",
			left:  jsonStringTestValue("{\n  \"a\": 1\n}"),
			right: jsonStringTestValue(`{"a":1}`),
			want:  true,
		},
		{
			name:  "number renormalized",
			left:  jsonStringTestValue(`{"a":1.0}`),
			right: jsonStringTestValue(`{"a":1}`),
			want:  true,
		},
		{
			name:  "different values",
			left:  jsonStringTestValue(`{"a":1}`),
			right: jsonStringTestValue(`{"a":2}`),
			want:  false,
		},
		{
			name:  "null and null",
			left:  NewJSONStringNull(),
			right: NewJSONStringNull(),
			want:  true,
		},
		{
			name:  "null and known",
			left:  NewJSONStringNull(),
			right: jsonStringTestValue(`{"a":1}`),
			want:  false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			equal, diags := tc.left.StringSemanticEquals(ctx, tc.right)
			if diags.HasError() {
				t.Fatalf("semantic equals returned diagnostics: %v", diags)
			}
			if equal != tc.want {
				t.Fatalf("StringSemanticEquals(%s, %s) = %t, want %t", tc.left, tc.right, equal, tc.want)
			}
		})
	}
}

func TestDynamicMappedValuesNonCanonicalJSONRoundTrip(t *testing.T) {
	ctx := context.Background()

	nonCanonicalElement := "{\n  \"warn\": \"2\",\n  \"ok\": 1.0\n}"
	nonCanonicalVariable := "{ \"z\": \"last\", \"a\": \"first\" }"

	original := &DynamicModel{
		QueryDefinitions: queryDefinitionsFixture(ctx, t),
		TimeFrame: &TimeFrameModel{
			Relative: &TimeFrameRelativeModel{Duration: types.StringValue("seconds:900")},
		},
		Visualization: &DynamicVisualizationModel{
			StatCard: &DynamicStatCardModel{
				CategoryFields: types.ListNull(ObservationFieldsObject()),
				ValueFields:    types.ListNull(ObservationFieldsObject()),
				PrimaryValue: &DynamicStatVisualElementModel{
					MappedValues:     jsonStringTestValue(nonCanonicalElement),
					ObservationField: observationFieldObject("primary", "user_data"),
					TemplateVariables: types.ListValueMust(types.ObjectType{AttrTypes: dynamicTemplateVariableAttr()}, []attr.Value{
						types.ObjectValueMust(dynamicTemplateVariableAttr(), map[string]attr.Value{
							"mapped_values":     jsonStringTestValue(nonCanonicalVariable),
							"observation_field": observationFieldObject("tvar", "user_data"),
						}),
					}),
				},
			},
		},
	}

	definition, diags := ExpandDynamic(ctx, original)
	if diags.HasError() {
		t.Fatalf("expanding dynamic model: %v", diags)
	}

	flattened, diags := FlattenDynamic(ctx, definition.Dynamic)
	if diags.HasError() {
		t.Fatalf("flattening dynamic widget: %v", diags)
	}

	got := flattened.Dynamic.Visualization.StatCard.PrimaryValue
	if got.MappedValues.ValueString() == nonCanonicalElement {
		t.Fatal("expected the API round-trip to renormalize the JSON; fixture is no longer a regression guard")
	}

	equal, diags := jsonStringTestValue(nonCanonicalElement).StringSemanticEquals(ctx, got.MappedValues)
	if diags.HasError() {
		t.Fatalf("semantic equals returned diagnostics: %v", diags)
	}
	if !equal {
		t.Fatalf("non-canonical mapped_values must be semantically equal after round-trip.\nconfig:    %s\nflattened: %s", nonCanonicalElement, got.MappedValues)
	}

	var variables []DynamicTemplateVariableModel
	if diags := got.TemplateVariables.ElementsAs(ctx, &variables, true); diags.HasError() {
		t.Fatalf("reading flattened template variables: %v", diags)
	}
	if len(variables) != 1 {
		t.Fatalf("expected 1 template variable, got %d", len(variables))
	}

	equal, diags = jsonStringTestValue(nonCanonicalVariable).StringSemanticEquals(ctx, variables[0].MappedValues)
	if diags.HasError() {
		t.Fatalf("semantic equals returned diagnostics: %v", diags)
	}
	if !equal {
		t.Fatalf("non-canonical template variable mapped_values must be semantically equal after round-trip.\nconfig:    %s\nflattened: %s", nonCanonicalVariable, variables[0].MappedValues)
	}
}

func float32TestValue(f float64) Float32Value {
	return Float32Value{Float64Value: basetypes.NewFloat64Value(f)}
}

func TestFloat32ValueSemanticEquals(t *testing.T) {
	ctx := context.Background()

	config := float32TestValue(0.1)
	afterRoundTrip := float32TestValue(float64(float32(0.1)))

	equal, diags := config.Float64SemanticEquals(ctx, afterRoundTrip)
	if diags.HasError() {
		t.Fatalf("semantic equals returned diagnostics: %v", diags)
	}
	if !equal {
		t.Fatalf("expected 0.1 and float64(float32(0.1)) to be semantically equal, got not equal")
	}

	different := float32TestValue(0.2)
	equal, diags = config.Float64SemanticEquals(ctx, different)
	if diags.HasError() {
		t.Fatalf("semantic equals returned diagnostics: %v", diags)
	}
	if equal {
		t.Fatalf("expected 0.1 and 0.2 to be semantically unequal, got equal")
	}

	nullValue := Float32Value{Float64Value: basetypes.NewFloat64Null()}
	equal, diags = config.Float64SemanticEquals(ctx, nullValue)
	if diags.HasError() {
		t.Fatalf("semantic equals returned diagnostics: %v", diags)
	}
	if equal {
		t.Fatalf("expected a known value and null to be semantically unequal, got equal")
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

func TestDynamicWidgetGeomapUnionArmsRoundTrip(t *testing.T) {
	ctx := context.Background()

	fieldBased := func(name string) *DynamicGeomapAggregationFieldBasedModel {
		return &DynamicGeomapAggregationFieldBasedModel{
			Field: observationFieldObject(name, "metadata"),
		}
	}

	geomapWith := func(mutate func(*DynamicGeomapModel)) *DynamicModel {
		geomap := &DynamicGeomapModel{
			Aggregation: &DynamicGeomapAggregationModel{Avg: fieldBased("avg_field")},
			Config: &DynamicGeomapFieldConfigModel{
				CoordinateConfig: &DynamicGeomapCoordinateConfigModel{
					LatitudeField:  observationFieldObject("lat", "metadata"),
					LongitudeField: observationFieldObject("lon", "metadata"),
				},
			},
		}
		mutate(geomap)
		return &DynamicModel{
			QueryDefinitions: queryDefinitionsFixture(ctx, t),
			Visualization:    &DynamicVisualizationModel{Geomap: geomap},
		}
	}

	for name, mutate := range map[string]func(*DynamicGeomapModel){
		"aggregation_avg": func(g *DynamicGeomapModel) { g.Aggregation = &DynamicGeomapAggregationModel{Avg: fieldBased("f")} },
		"aggregation_count": func(g *DynamicGeomapModel) {
			g.Aggregation = &DynamicGeomapAggregationModel{Count: types.BoolValue(true)}
		},
		"aggregation_max": func(g *DynamicGeomapModel) { g.Aggregation = &DynamicGeomapAggregationModel{Max: fieldBased("f")} },
		"aggregation_min": func(g *DynamicGeomapModel) { g.Aggregation = &DynamicGeomapAggregationModel{Min: fieldBased("f")} },
		"aggregation_sum": func(g *DynamicGeomapModel) { g.Aggregation = &DynamicGeomapAggregationModel{Sum: fieldBased("f")} },
		"color_range": func(g *DynamicGeomapModel) {
			g.Color = &DynamicGeomapColorModel{ColorRange: types.StringValue("red_reversed")}
		},
		"color_size": func(g *DynamicGeomapModel) { g.Color = &DynamicGeomapColorModel{Size: types.StringValue("orange")} },
		"config_aws_region": func(g *DynamicGeomapModel) {
			g.Config = &DynamicGeomapFieldConfigModel{
				AwsRegionConfig: &DynamicGeomapAwsRegionConfigModel{AwsRegionField: observationFieldObject("region", "metadata")},
			}
		},
		"min_max_auto": func(g *DynamicGeomapModel) {
			g.MinMax = &DynamicMinMaxModel{Auto: types.BoolValue(true)}
		},
	} {
		t.Run(name, func(t *testing.T) {
			assertDynamicRoundTrip(ctx, t, geomapWith(mutate))
		})
	}
}

func TestObjectValidatorsDeferOnUnknownConfig(t *testing.T) {
	ctx := context.Background()

	for name, tc := range map[string]struct {
		validate  func(context.Context, validator.ObjectRequest, *validator.ObjectResponse)
		attrTypes map[string]attr.Type
	}{
		"logs_aggregation":  {validate: logsAggregationValidator{}.ValidateObject, attrTypes: AggregationModelAttr()},
		"spans_aggregation": {validate: spansAggregationValidator{}.ValidateObject, attrTypes: SpansAggregationModelAttr()},
		"filter_operator":   {validate: filterOperatorValidator{}.ValidateObject, attrTypes: FilterOperatorModelAttr()},
		"spans_field":       {validate: spansFieldValidator{}.ValidateObject, attrTypes: SpansFieldModelAttr()},
	} {
		t.Run(name, func(t *testing.T) {
			resp := &validator.ObjectResponse{}
			tc.validate(ctx, validator.ObjectRequest{
				ConfigValue: types.ObjectUnknown(tc.attrTypes),
			}, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("unknown object produced diagnostics instead of deferring: %v", resp.Diagnostics.Errors())
			}
		})
	}
}
