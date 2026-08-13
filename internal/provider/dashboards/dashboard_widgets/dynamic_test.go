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

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestDynamicWidgetLogsStatRoundTrip(t *testing.T) {
	ctx := context.Background()
	assertDynamicRoundTrip(ctx, t, logsStatFixture(ctx, t))
}

func TestExpandDynamicPopulatesQueryAndVisualization(t *testing.T) {
	ctx := context.Background()

	definition, diags := ExpandDynamic(ctx, logsStatFixture(ctx, t))
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
}

func logsStatFixture(ctx context.Context, t *testing.T) *DynamicModel {
	t.Helper()

	return &DynamicModel{
		QueryDefinitions: logsQueryDefinitionsFixture(ctx, t),
		TimeFrame: &TimeFrameModel{
			Relative: &TimeFrameRelativeModel{Duration: types.StringValue("seconds:900")},
		},
		Visualization: &DynamicVisualizationModel{
			Stat: statWithUnsetCollections(func(stat *DynamicStatModel) {
				stat.ThresholdType = types.StringValue("absolute")
				stat.Thresholds = dynamicThresholdList()
				stat.Unit = types.StringValue("bytes")
				stat.ValueField = observationFieldObject("duration", "metadata")
			}),
		},
	}
}

// Every SDK field of Stat is set, so a dropped field in either direction fails
// the object comparison instead of silently disappearing on the next read.
func TestDynamicWidgetStatFullFidelityRoundTrip(t *testing.T) {
	ctx := context.Background()

	original := &DynamicModel{
		QueryDefinitions: logsQueryDefinitionsFixture(ctx, t),
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
				Legend: &LegendModel{
					IsVisible:    types.BoolValue(true),
					Columns:      types.ListValueMust(types.StringType, []attr.Value{types.StringValue("min")}),
					GroupByQuery: types.BoolValue(true),
					Placement:    types.StringValue("bottom"),
				},
				LegendBy:      types.StringValue("thresholds"),
				Max:           types.Float64Value(100),
				Min:           types.Float64Value(0),
				ThresholdBy:   types.StringValue("background"),
				ThresholdType: types.StringValue("absolute"),
				Thresholds:    dynamicThresholdList(),
				Unit:          types.StringValue("bytes"),
				ValueField:    observationFieldObject("duration", "metadata"),
				ValueFields:   observationFieldList("duration2", "metadata"),
			},
		},
	}

	assertDynamicRoundTrip(ctx, t, original)
}

func TestDynamicWidgetMetricsQueryFullFidelityRoundTrip(t *testing.T) {
	ctx := context.Background()

	metricsQuery := &DynamicQueryMetricsModel{
		PromqlQuery:     types.StringValue("rate(x[5m])"),
		PromqlQueryType: types.StringValue("instant"),
		EditorMode:      types.StringValue("text"),
		SeriesLimitType: types.StringValue("by_series_count"),
	}

	original := &DynamicModel{
		QueryDefinitions: types.ListValueMust(
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
		),
		Visualization: &DynamicVisualizationModel{
			Stat: statWithUnsetCollections(func(stat *DynamicStatModel) {
				stat.Unit = types.StringValue("bytes")
			}),
		},
	}

	assertDynamicRoundTrip(ctx, t, original)
}

func TestDynamicWidgetSpansAndDataPrimeQueryFullFidelityRoundTrip(t *testing.T) {
	ctx := context.Background()

	spansQuery := &DynamicQuerySpansModel{
		LuceneQuery: types.StringValue("*"),
		GroupBy: types.ListValueMust(types.ObjectType{AttrTypes: spanObservationFieldAttr()}, []attr.Value{
			types.ObjectValueMust(spanObservationFieldAttr(), map[string]attr.Value{
				"keypath":       types.ListValueMust(types.StringType, []attr.Value{types.StringValue("service"), types.StringValue("name")}),
				"scope":         types.StringValue("user_data"),
				"relation_type": types.StringValue("parent"),
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
		Filters: types.ListValueMust(types.ObjectType{AttrTypes: SpansFilterModelAttr()}, []attr.Value{
			types.ObjectValueMust(SpansFilterModelAttr(), map[string]attr.Value{
				"field": types.ObjectValueMust(SpansFieldModelAttr(), map[string]attr.Value{
					"type":  types.StringValue("metadata"),
					"value": types.StringValue("application_name"),
				}),
				"operator": types.ObjectValueMust(FilterOperatorModelAttr(), map[string]attr.Value{
					"type":            types.StringValue("equals"),
					"selected_values": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("api")}),
				}),
			}),
		}),
		DataModeType: types.StringValue("archive"),
	}

	dataPrimeQuery := &DynamicQueryDataPrimeModel{
		Query:        types.StringValue("source logs | limit 10"),
		DataModeType: types.StringValue("archive"),
	}

	original := &DynamicModel{
		QueryDefinitions: types.ListValueMust(
			types.ObjectType{AttrTypes: dynamicQueryDefinitionModelAttr()},
			[]attr.Value{
				types.ObjectValueMust(dynamicQueryDefinitionModelAttr(), map[string]attr.Value{
					"id":   types.StringValue("query-1"),
					"name": types.StringValue("spans"),
					"query": types.ObjectValueMust(dynamicQueryModelAttr(), map[string]attr.Value{
						"logs":       types.ObjectNull(dynamicLogsQueryAttr()),
						"spans":      objectFrom(ctx, t, dynamicSpansQueryAttr(), spansQuery),
						"metrics":    types.ObjectNull(dynamicMetricsQueryAttr()),
						"data_prime": types.ObjectNull(dynamicDataPrimeQueryAttr()),
					}),
				}),
				types.ObjectValueMust(dynamicQueryDefinitionModelAttr(), map[string]attr.Value{
					"id":   types.StringValue("query-2"),
					"name": types.StringValue("data prime"),
					"query": types.ObjectValueMust(dynamicQueryModelAttr(), map[string]attr.Value{
						"logs":       types.ObjectNull(dynamicLogsQueryAttr()),
						"spans":      types.ObjectNull(dynamicSpansQueryAttr()),
						"metrics":    types.ObjectNull(dynamicMetricsQueryAttr()),
						"data_prime": objectFrom(ctx, t, dynamicDataPrimeQueryAttr(), dataPrimeQuery),
					}),
				}),
			},
		),
		Visualization: &DynamicVisualizationModel{
			Stat: statWithUnsetCollections(func(stat *DynamicStatModel) {
				stat.Unit = types.StringValue("bytes")
			}),
		},
	}

	assertDynamicRoundTrip(ctx, t, original)
}

// A visualization variant this provider version does not model must fail the
// read rather than write partial state.
func TestFlattenDynamicRejectsUnmodeledVisualization(t *testing.T) {
	_, diags := flattenDynamicVisualization(context.Background(), &dashboardservice.Visualization{
		Table: &dashboardservice.Table{},
	})
	if !diags.HasError() {
		t.Fatal("flattening an unmodeled visualization returned no error diagnostic")
	}
}

// A zero-value types.List/types.Object carries no element type, which fails
// conversion against the schema's typed attributes, so unset stat collections
// are built explicitly.
func statWithUnsetCollections(set func(*DynamicStatModel)) *DynamicStatModel {
	stat := &DynamicStatModel{
		CategoryFields: types.ListNull(ObservationFieldsObject()),
		Thresholds:     types.ListNull(types.ObjectType{AttrTypes: dynamicThresholdAttr()}),
		ValueField:     types.ObjectNull(ObservationFieldAttr()),
		ValueFields:    types.ListNull(ObservationFieldsObject()),
	}
	set(stat)
	return stat
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

func logsQueryDefinitionsFixture(ctx context.Context, t *testing.T) types.List {
	t.Helper()
	logs := &DynamicQueryLogsModel{
		LuceneQuery: types.StringValue("level:error"),
		GroupBy:     observationFieldList("service", "user_data"),
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

func objectFrom(ctx context.Context, t *testing.T, attrTypes map[string]attr.Type, value any) types.Object {
	t.Helper()
	object, diags := types.ObjectValueFrom(ctx, attrTypes, value)
	if diags.HasError() {
		t.Fatalf("building object value: %v", diags)
	}
	return object
}

// Sets range, regex and value together, which ExactlyOneOfChildren forbids in
// real HCL. That is deliberate: it exercises every mapping branch through
// expand/flatten in one pass. Do not collapse it to a single branch.
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
			"mapped_values":     types.BoolValue(true),
			"observation_field": observationFieldObject("tvar", "user_data"),
		}),
	})

	visualElement := func(name string) *DynamicStatVisualElementModel {
		return &DynamicStatVisualElementModel{
			MappedValues:      types.BoolValue(true),
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
		QueryDefinitions: logsQueryDefinitionsFixture(ctx, t),
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
		QueryDefinitions: logsQueryDefinitionsFixture(ctx, t),
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
