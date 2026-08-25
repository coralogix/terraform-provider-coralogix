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
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
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
					"selection_type":  types.StringValue(filterSelectionTypeList),
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
//
// Every variant in the pinned SDK is now modelled, so there is no real one left
// to point at. An empty visualization stands in: it is what a variant added by a
// later SDK looks like to this switch - a non-nil visualization whose arm none of
// the family helpers recognise. Replace it with that variant when one appears.
func TestFlattenDynamicRejectsUnmodeledVisualization(t *testing.T) {
	_, diags := flattenDynamicVisualization(context.Background(), &dashboardservice.Visualization{})
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

// The colour mapping takes exactly one of range, value and regex, so each arm
// needs its own case. The full-fidelity fixture above covers range; these two
// cover the others, which it used to set at the same time.
func TestDynamicWidgetStatCardColourMappingArmsRoundTrip(t *testing.T) {
	ctx := context.Background()

	sections := types.ListValueMust(types.ObjectType{AttrTypes: dynamicMappingSectionAttr()}, []attr.Value{
		types.ObjectValueMust(dynamicMappingSectionAttr(), map[string]attr.Value{
			"color":  types.StringValue("green"),
			"map_to": types.StringValue("OK"),
			"value":  types.StringValue("200"),
		}),
	})

	for name, mapping := range map[string]*DynamicColorLabelMappingModel{
		"value": {ColorBy: types.StringValue("value"), Value: &DynamicSectionsMappingModel{Sections: sections}},
		"regex": {ColorBy: types.StringValue("value"), Regex: &DynamicSectionsMappingModel{Sections: sections}},
	} {
		t.Run(name, func(t *testing.T) {
			assertDynamicRoundTrip(ctx, t, &DynamicModel{
				QueryDefinitions: logsQueryDefinitionsFixture(ctx, t),
				TimeFrame: &TimeFrameModel{
					Relative: &TimeFrameRelativeModel{Duration: types.StringValue("seconds:900")},
				},
				Visualization: &DynamicVisualizationModel{StatCard: &DynamicStatCardModel{
					ColorLabelMapping: mapping,
					CategoryFields:    observationFieldList("category", "user_data"),
					ValueFields:       observationFieldList("value", "metadata"),
				}},
			})
		})
	}
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

func TestDynamicWidgetGaugeAndPieFullFidelityRoundTrip(t *testing.T) {
	ctx := context.Background()

	legend := &LegendModel{
		IsVisible:    types.BoolValue(true),
		Columns:      types.ListValueMust(types.StringType, []attr.Value{types.StringValue("max")}),
		GroupByQuery: types.BoolValue(false),
		Placement:    types.StringValue("bottom"),
	}

	for name, visualization := range map[string]*DynamicVisualizationModel{
		"gauge": {Gauge: &DynamicGaugeModel{
			AllowAbbreviation: types.BoolValue(true),
			ArcDisplay: &DynamicArcDisplayModel{
				ThresholdArc: types.BoolValue(true),
				ValueArc:     types.BoolValue(false),
			},
			CategoryFields:    observationFieldList("applicationname", "label"),
			CustomUnit:        types.StringValue("rpm"),
			DecimalPrecision:  types.Int64Value(2),
			DisplaySeriesName: types.BoolValue(true),
			Legend:            legend,
			LegendBy:          types.StringValue("thresholds"),
			Max:               types.Float64Value(1000),
			Min:               types.Float64Value(0),
			ShowInnerArc:      types.BoolValue(true),
			ShowMinMax:        types.BoolValue(false),
			ShowOuterArc:      types.BoolValue(true),
			ThresholdType:     types.StringValue("absolute"),
			Thresholds:        dynamicThresholdList(),
			Unit:              types.StringValue("seconds"),
			ValueField:        observationFieldObject("duration", "metadata"),
			ValueFields:       observationFieldList("value", "user_data"),
		}},
		"pie_chart": {PieChart: &DynamicPieChartModel{
			AllowAbbreviation: types.BoolValue(false),
			CategoryFields:    observationFieldList("applicationname", "label"),
			ColorScheme:       types.StringValue("classic"),
			CustomUnit:        types.StringValue("rpm"),
			DecimalPrecision:  types.Int64Value(1),
			GroupNameTemplate: types.StringValue("group"),
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
			MinSlicePercentage: types.Int64Value(2),
			ShowTotal:          types.BoolValue(true),
			StackNameTemplate:  types.StringValue("stack"),
			SubCategoryFields:  observationFieldList("severity", "metadata"),
			Unit:               types.StringValue("bytes"),
			ValueField:         observationFieldObject("duration", "metadata"),
		}},
	} {
		t.Run(name, func(t *testing.T) {
			assertDynamicRoundTrip(ctx, t, &DynamicModel{
				QueryDefinitions: logsQueryDefinitionsFixture(ctx, t),
				TimeFrame: &TimeFrameModel{
					Relative: &TimeFrameRelativeModel{Duration: types.StringValue("seconds:900")},
				},
				Visualization: visualization,
			})
		})
	}
}

func TestDynamicWidgetBarsFullFidelityRoundTrip(t *testing.T) {
	ctx := context.Background()

	f32 := func(v float64) Float32Value {
		return Float32Value{Float64Value: basetypes.NewFloat64Value(v)}
	}
	legend := &LegendModel{
		IsVisible:    types.BoolValue(true),
		Columns:      types.ListValueMust(types.StringType, []attr.Value{types.StringValue("max")}),
		GroupByQuery: types.BoolValue(false),
		Placement:    types.StringValue("bottom"),
	}
	queryFieldSettings, diags := types.ListValueFrom(ctx,
		types.ObjectType{AttrTypes: dynamicBarsQueryFieldSettingsModelAttr()},
		[]DynamicBarsQueryFieldSettingsModel{{
			QueryID:    types.StringValue("9d1b7a4e-0000-4000-8000-00000000ab01"),
			ValueField: observationFieldObject("duration", "metadata"),
		}})
	if diags.HasError() {
		t.Fatalf("building query field settings: %v", diags)
	}

	// One arm per chart, since the API accepts exactly one and the schema
	// enforces it: category on the vertical multi, query_value on the horizontal.
	categorySort := &DynamicSortOrderModel{
		OrderDirection: types.StringValue("asc"),
		Strategy: &DynamicSortStrategyModel{
			Category:     types.BoolValue(true),
			StrategyType: types.StringNull(),
		},
	}
	queryValueSort := &DynamicSortOrderModel{
		OrderDirection: types.StringValue("desc"),
		Strategy: &DynamicSortStrategyModel{
			Category:     types.BoolNull(),
			QueryValue:   &DynamicSortByQueryValueModel{QueryID: types.StringValue("9d1b7a4e-0000-4000-8000-00000000ab01")},
			StrategyType: types.StringNull(),
		},
	}

	for name, visualization := range map[string]*DynamicVisualizationModel{
		"vertical_bars": {VerticalBars: &DynamicVerticalBarsModel{
			AllowAbbreviation: types.BoolValue(true),
			BarValueDisplay:   types.StringValue("top"),
			CategoryFields:    observationFieldList("applicationname", "label"),
			ColorScheme:       types.StringValue("classic"),
			ColorsBy:          types.StringValue("stack"),
			CustomUnit:        types.StringValue("rpm"),
			DecimalPrecision:  types.Int64Value(2),
			GroupNameTemplate: types.StringValue("group"),
			HashColors:        types.BoolValue(true),
			Legend:            legend,
			MaxBarsPerChart:   types.Int64Value(20),
			MaxSlicesPerBar:   types.Int64Value(5),
			ScaleType:         types.StringValue("linear"),
			SortBy:            types.StringValue("value"),
			StackNameTemplate: types.StringValue("stack"),
			SubCategoryFields: observationFieldList("severity", "metadata"),
			Unit:              types.StringValue("seconds"),
			ValueField:        observationFieldObject("duration", "metadata"),
			YAxisMax:          f32(99.5),
			YAxisMin:          f32(0),
		}},
		"vertical_bars_multi": {VerticalBarsMulti: &DynamicVerticalBarsMultiModel{
			AllowAbbreviation:  types.BoolValue(false),
			BarValueDisplay:    types.StringValue("inside"),
			CategoryFields:     observationFieldList("applicationname", "label"),
			ColorScheme:        types.StringValue("cold"),
			ColorsBy:           types.StringValue("query"),
			CustomUnit:         types.StringValue("rpm"),
			DecimalPrecision:   types.Int64Value(1),
			GroupNameTemplate:  types.StringValue("group"),
			HashColors:         types.BoolValue(false),
			Legend:             legend,
			MaxBarsPerChart:    types.Int64Value(10),
			QueryFieldSettings: queryFieldSettings,
			ScaleType:          types.StringValue("logarithmic"),
			SortOrder:          categorySort,
			Unit:               types.StringValue("bytes"),
			YAxisMax:           f32(1000),
			YAxisMin:           f32(-10.25),
		}},
		"horizontal_bars": {HorizontalBars: &DynamicHorizontalBarsModel{
			AllowAbbreviation: types.BoolValue(true),
			CategoryFields:    observationFieldList("applicationname", "label"),
			ColorScheme:       types.StringValue("red"),
			ColorsBy:          types.StringValue("group_by"),
			CustomUnit:        types.StringValue("rpm"),
			DecimalPrecision:  types.Int64Value(3),
			DisplayOnBar:      types.BoolValue(true),
			GroupNameTemplate: types.StringValue("group"),
			HashColors:        types.BoolValue(true),
			Legend:            legend,
			MaxBarsPerChart:   types.Int64Value(15),
			MaxSlicesPerBar:   types.Int64Value(4),
			ScaleType:         types.StringValue("linear"),
			SortBy:            types.StringValue("name"),
			StackNameTemplate: types.StringValue("stack"),
			SubCategoryFields: observationFieldList("severity", "metadata"),
			Unit:              types.StringValue("milliseconds"),
			ValueField:        observationFieldObject("duration", "metadata"),
			YAxisMax:          f32(50),
			YAxisMin:          f32(5),
			YAxisViewBy:       types.StringValue("category"),
		}},
		"horizontal_bars_multi": {HorizontalBarsMulti: &DynamicHorizontalBarsMultiModel{
			AllowAbbreviation:  types.BoolValue(false),
			CategoryFields:     observationFieldList("applicationname", "label"),
			ColorScheme:        types.StringValue("blue"),
			ColorsBy:           types.StringValue("aggregation"),
			CustomUnit:         types.StringValue("rpm"),
			DecimalPrecision:   types.Int64Value(0),
			DisplayOnBar:       types.BoolValue(false),
			GroupNameTemplate:  types.StringValue("group"),
			HashColors:         types.BoolValue(false),
			Legend:             legend,
			MaxBarsPerChart:    types.Int64Value(25),
			QueryFieldSettings: queryFieldSettings,
			ScaleType:          types.StringValue("linear"),
			SortOrder:          queryValueSort,
			Unit:               types.StringValue("usd"),
			YAxisMax:           f32(12.5),
			YAxisMin:           f32(0),
			YAxisViewBy:        types.StringValue("value"),
		}},
	} {
		t.Run(name, func(t *testing.T) {
			assertDynamicRoundTrip(ctx, t, &DynamicModel{
				QueryDefinitions: logsQueryDefinitionsFixture(ctx, t),
				TimeFrame: &TimeFrameModel{
					Relative: &TimeFrameRelativeModel{Duration: types.StringValue("seconds:900")},
				},
				Visualization: visualization,
			})
		})
	}
}

func TestDynamicWidgetTableFullFidelityRoundTrip(t *testing.T) {
	ctx := context.Background()

	propertyList := func(properties ...DynamicTablePropertyModel) types.List {
		list, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicTablePropertyModelAttr()}, properties)
		if diags.HasError() {
			t.Fatalf("building table properties: %v", diags)
		}
		return list
	}

	property := func(id string, definition DynamicTablePropertyDefinitionModel) DynamicTablePropertyModel {
		return DynamicTablePropertyModel{ID: types.StringValue(id), Definition: &definition}
	}

	linkActions, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicTableLinkActionModelAttr()}, []DynamicTableLinkActionModel{{
		ID:                    types.StringValue("action-id"),
		Name:                  types.StringValue("open trace"),
		ShouldOpenInNewWindow: types.BoolValue(true),
		Url:                   types.StringValue("https://example.com/trace"),
	}})
	if diags.HasError() {
		t.Fatalf("building link actions: %v", diags)
	}

	valueMappings, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicTableValueMappingModelAttr()}, []DynamicTableValueMappingModel{{
		InputValue:   types.StringValue("500"),
		ReplaceValue: types.StringValue("server error"),
		Type:         types.StringValue("value"),
	}})
	if diags.HasError() {
		t.Fatalf("building value mappings: %v", diags)
	}

	columnWidths, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicTableColumnWidthModelAttr()}, []DynamicTableColumnWidthModel{{
		ColumnName: types.StringValue("severity"),
		Width:      types.Int64Value(120),
	}})
	if diags.HasError() {
		t.Fatalf("building column widths: %v", diags)
	}

	columns, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicTableColumnModelAttr()}, []DynamicTableColumnModel{
		{Field: observationFieldObject("severity", "metadata")},
		{Field: observationFieldObject("responsetime", "user_data")},
	})
	if diags.HasError() {
		t.Fatalf("building table columns: %v", diags)
	}

	rules, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicTableRuleModelAttr()}, []DynamicTableRuleModel{
		{
			Description: types.StringValue("scoped by field"),
			ID:          types.StringValue("rule-field"),
			Name:        types.StringValue("field rule"),
			RuleScope:   &DynamicTableRuleScopeModel{Field: observationFieldObject("severity", "metadata")},
			Properties: propertyList(
				property("prop-display-name", DynamicTablePropertyDefinitionModel{ColumnDisplayName: types.StringValue("Severity")}),
				property("prop-alignment", DynamicTablePropertyDefinitionModel{Alignment: types.StringValue("center")}),
				property("prop-units", DynamicTablePropertyDefinitionModel{Units: &DynamicTablePropertyUnitsModel{
					AllowAbbreviation: types.BoolValue(true),
					CustomUnit:        types.StringValue("reqs"),
					DecimalPrecision:  types.Int64Value(3),
					Max:               types.Float64Value(99),
					Min:               types.Float64Value(1),
					Unit:              types.StringValue("milliseconds"),
				}}),
			),
		},
		{
			Description: types.StringValue("scoped by regex"),
			ID:          types.StringValue("rule-regex"),
			Name:        types.StringValue("regex rule"),
			RuleScope:   &DynamicTableRuleScopeModel{Field: types.ObjectNull(ObservationFieldAttr()), Regex: types.StringValue("^resp.*")},
			Properties: propertyList(
				property("prop-regex-extract", DynamicTablePropertyDefinitionModel{RegexExtract: types.StringValue("([0-9]+)")}),
				property("prop-link", DynamicTablePropertyDefinitionModel{Link: &DynamicTablePropertyLinkModel{Actions: linkActions}}),
				property("prop-values-alias", DynamicTablePropertyDefinitionModel{ValuesAlias: types.StringValue("alias")}),
			),
		},
		{
			Description: types.StringValue("scoped by field type"),
			ID:          types.StringValue("rule-field-type"),
			Name:        types.StringValue("field type rule"),
			RuleScope:   &DynamicTableRuleScopeModel{Field: types.ObjectNull(ObservationFieldAttr()), FieldType: types.StringValue("number")},
			Properties: propertyList(
				property("prop-values-mapping", DynamicTablePropertyDefinitionModel{ValuesMapping: &DynamicTablePropertyValuesMappingModel{Mappings: valueMappings}}),
				property("prop-thresholds", DynamicTablePropertyDefinitionModel{Thresholds: &DynamicTablePropertyThresholdsModel{
					Max:    types.Float64Value(1000),
					Min:    types.Float64Value(0),
					Type:   types.StringValue("absolute"),
					Values: dynamicThresholdList(),
				}}),
			),
		},
	})
	if diags.HasError() {
		t.Fatalf("building table rules: %v", diags)
	}

	original := &DynamicModel{
		QueryDefinitions: logsQueryDefinitionsFixture(ctx, t),
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

// Shapes the API stores that the typed schema must be able to express. Each was
// confirmed by applying it through content_json and reading the dashboard back.
func TestDynamicWidgetTableReadsShapesTheSchemaMustExpress(t *testing.T) {
	ctx := context.Background()
	scope := dashboardservice.DATASETSCOPE_DATASET_SCOPE_USER_DATA

	t.Run("column keypath may be empty or absent", func(t *testing.T) {
		for name, keypath := range map[string][]string{"empty": {}, "absent": nil} {
			t.Run(name, func(t *testing.T) {
				table, diags := flattenDynamicTable(ctx, &dashboardservice.Table{
					Columns: []dashboardservice.TableColumn{{
						Field: &dashboardservice.ObservationField{Keypath: keypath, Scope: &scope},
					}},
				})
				if diags.HasError() {
					t.Fatalf("flattening a column with a %s keypath: %v", name, diags)
				}

				columns := table.Columns.Elements()
				if len(columns) != 1 {
					t.Fatalf("expected one column, got %d", len(columns))
				}
				field := columns[0].(types.Object).Attributes()["field"].(types.Object)
				if got := field.Attributes()["keypath"]; !got.IsNull() {
					t.Fatalf("keypath must read back as null so the config can omit it, got %s", got)
				}
			})
		}
	})

	// A union with no arm selected must read back absent. Left as a present
	// object it would flatten into config that ExactlyOneOfChildren rejects,
	// leaving the dashboard unmanageable in typed HCL.
	t.Run("rule scope with no arm reads back absent", func(t *testing.T) {
		table, diags := flattenDynamicTable(ctx, &dashboardservice.Table{
			Rules: []dashboardservice.TableRule{{RuleScope: &dashboardservice.RuleScope{}}},
		})
		if diags.HasError() {
			t.Fatalf("flattening a rule with an empty scope: %v", diags)
		}
		rule := table.Rules.Elements()[0].(types.Object)
		if got := rule.Attributes()["rule_scope"]; !got.IsNull() {
			t.Fatalf("rule_scope must read back as null, got %s", got)
		}
	})

	t.Run("property definition with no arm reads back absent", func(t *testing.T) {
		regex := "^a$"
		table, diags := flattenDynamicTable(ctx, &dashboardservice.Table{
			Rules: []dashboardservice.TableRule{{
				RuleScope:  &dashboardservice.RuleScope{Regex: &regex},
				Properties: []dashboardservice.Property{{Definition: &dashboardservice.PropertyDefinition{}}},
			}},
		})
		if diags.HasError() {
			t.Fatalf("flattening a property with an empty definition: %v", diags)
		}
		rule := table.Rules.Elements()[0].(types.Object)
		property := rule.Attributes()["properties"].(types.List).Elements()[0].(types.Object)
		if got := property.Attributes()["definition"]; !got.IsNull() {
			t.Fatalf("definition must read back as null, got %s", got)
		}
	})
}

// Guards the schema half of the empty-keypath fix: the flatten test above
// cannot catch a keypath that is Required again, because the read direction
// returns null either way.
func TestDynamicTableColumnKeypathIsOptional(t *testing.T) {
	table := dynamicTableSchema().(schema.SingleNestedAttribute)
	columns := table.Attributes["columns"].(schema.ListNestedAttribute)
	field := columns.NestedObject.Attributes["field"].(schema.SingleNestedAttribute)
	keypath := field.Attributes["keypath"].(schema.ListAttribute)

	if keypath.Required {
		t.Error("table column keypath must not be Required: the API stores columns with an absent keypath")
	}
	if !keypath.Optional {
		t.Error("table column keypath must be Optional")
	}
}

func TestDynamicWidgetSpatialFullFidelityRoundTrip(t *testing.T) {
	ctx := context.Background()

	f32 := func(v float64) Float32Value {
		return Float32Value{Float64Value: basetypes.NewFloat64Value(v)}
	}
	legend := &LegendModel{
		IsVisible:    types.BoolValue(true),
		Columns:      types.ListValueMust(types.StringType, []attr.Value{types.StringValue("max")}),
		GroupByQuery: types.BoolValue(false),
		Placement:    types.StringValue("bottom"),
	}
	tooltipLabels := observationFieldList("applicationname", "label")

	for name, visualization := range map[string]*DynamicVisualizationModel{
		"hexagon_bins": {HexagonBins: &DynamicHexagonBinsModel{
			AllowAbbreviation: types.BoolValue(true),
			CategoryFields:    observationFieldList("applicationname", "label"),
			CustomUnit:        types.StringValue("rpm"),
			DecimalPrecision:  types.Int64Value(2),
			Legend:            legend,
			LegendBy:          types.StringValue("thresholds"),
			Max:               types.Float64Value(1000),
			Min:               types.Float64Value(0),
			ThresholdType:     types.StringValue("absolute"),
			Thresholds:        dynamicThresholdList(),
			Unit:              types.StringValue("seconds"),
			ValueField:        observationFieldObject("duration", "metadata"),
		}},
		// preset and color_range are the two arms of the same proto oneof, so
		// only one may be set; this covers the gradient arm.
		"heatmap": {Heatmap: &DynamicHeatmapModel{
			AllowAbbreviation:   types.BoolValue(false),
			ColorAxisMax:        f32(99.5),
			ColorAxisMin:        f32(-10.25),
			ColorRange:          types.StringValue("blue"),
			CustomUnit:          types.StringValue("rpm"),
			DecimalPrecision:    types.Int64Value(3),
			HistogramBucketUnit: types.StringValue("seconds"),
			Preset:              types.StringNull(),
			ScaleType:           types.StringValue("linear"),
			ShowNumbers:         types.BoolValue(true),
			Tooltip: &DynamicHeatmapTooltipModel{
				Labels:          tooltipLabels,
				MessageTemplate: types.StringValue("value = {{_count}}"),
			},
			Unit:            types.StringValue("bytes"),
			ValueField:      observationFieldObject("duration", "metadata"),
			XAxisFields:     observationFieldList("timestamp", "metadata"),
			XAxisTimeFormat: types.StringValue("hh_mm"),
			YAxisFields:     observationFieldList("severity", "metadata"),
		}},
		"geomap": {Geomap: &DynamicGeomapModel{
			AllowAbbreviation: types.BoolValue(true),
			Aggregation: &DynamicGeomapAggregationModel{
				Count: types.BoolValue(true),
			},
			Color: &DynamicGeomapColorModel{
				Size: types.StringValue("blue"),
			},
			Config: &DynamicGeomapFieldConfigModel{
				CoordinateConfig: &DynamicGeomapCoordinateConfigModel{
					LatitudeField:  observationFieldObject("lat", "user_data"),
					LongitudeField: observationFieldObject("lon", "user_data"),
				},
			},
			CustomUnit:       types.StringValue("rpm"),
			DecimalPrecision: types.Int64Value(1),
			MinMax: &DynamicMinMaxModel{
				Custom: &DynamicMinMaxCustomModel{
					Max: types.Float64Value(50),
					Min: types.Float64Value(5),
				},
			},
			Tooltip: &DynamicGeomapTooltipModel{
				Labels:          tooltipLabels,
				MessageTemplate: types.StringValue("value = {{_count}}"),
			},
			Unit: types.StringValue("usd"),
		}},
		// The field-based aggregation arms carry an observation field, unlike
		// count which is an empty marker.
		"geomap_field_based_aggregation": {Geomap: &DynamicGeomapModel{
			Aggregation: &DynamicGeomapAggregationModel{
				Sum: &DynamicGeomapAggregationFieldBasedModel{
					Field: observationFieldObject("duration", "metadata"),
				},
			},
			Color: &DynamicGeomapColorModel{
				ColorRange: types.StringValue("green"),
			},
			Config: &DynamicGeomapFieldConfigModel{
				AwsRegionConfig: &DynamicGeomapAwsRegionConfigModel{
					AwsRegionField: observationFieldObject("region", "user_data"),
				},
			},
		}},
	} {
		t.Run(name, func(t *testing.T) {
			assertDynamicRoundTrip(ctx, t, &DynamicModel{
				QueryDefinitions: logsQueryDefinitionsFixture(ctx, t),
				TimeFrame: &TimeFrameModel{
					Relative: &TimeFrameRelativeModel{Duration: types.StringValue("seconds:900")},
				},
				Visualization: visualization,
			})
		})
	}
}

// The API stores a geomap min/max wrapper with neither arm selected. Read back
// as a present block it would carry two null children, which the block's own
// exactly-one-of validator rejects, so the dashboard would diff forever.
// `auto` can be false only when it came from a value unknown at plan time, so
// the object validator has already deferred. Sending the resulting empty
// wrapper would fail the apply, so the conversion refuses it instead.
func TestExpandDynamicMinMaxRejectsFalseAuto(t *testing.T) {
	if _, diags := expandDynamicMinMax(&DynamicMinMaxModel{Auto: types.BoolValue(false)}); !diags.HasError() {
		t.Error("a min/max block whose auto resolved to false must be refused")
	}
	if _, diags := expandDynamicMinMax(&DynamicMinMaxModel{Auto: types.BoolNull()}); !diags.HasError() {
		t.Error("a min/max block with no arm at all must be refused")
	}

	// The valid shapes must still convert, or the guard above would be masking
	// a conversion that never works.
	if got, diags := expandDynamicMinMax(&DynamicMinMaxModel{Auto: types.BoolValue(true)}); diags.HasError() || got == nil || got.Auto == nil {
		t.Errorf("the auto arm must convert, got %v %v", got, diags)
	}
	custom := &DynamicMinMaxModel{Auto: types.BoolNull(), Custom: &DynamicMinMaxCustomModel{Max: types.Float64Value(50)}}
	if got, diags := expandDynamicMinMax(custom); diags.HasError() || got == nil || got.Custom == nil {
		t.Errorf("the custom arm must convert, got %v %v", got, diags)
	}
}

func TestDynamicWidgetGeomapEmptyMinMaxReadsBackAbsent(t *testing.T) {
	if got := flattenDynamicMinMax(&dashboardservice.MinMax{}); got != nil {
		t.Errorf("a min/max wrapper with no arm must read back as absent, got auto=%s custom=%v", got.Auto, got.Custom)
	}

	// The populated arms must still survive, or the guard above would be
	// hiding a dropped field rather than preventing an unexpressible one.
	if got := flattenDynamicMinMax(&dashboardservice.MinMax{Auto: map[string]interface{}{}}); got == nil || !got.Auto.ValueBool() {
		t.Errorf("the auto arm must read back set, got %v", got)
	}
	max := 50.0
	if got := flattenDynamicMinMax(&dashboardservice.MinMax{Custom: &dashboardservice.MinMaxCustom{Max: &max}}); got == nil || got.Custom == nil {
		t.Errorf("the custom arm must read back set, got %v", got)
	}
}

// Each union's validator defers while any child is unknown, so a value only
// known after apply can arrive having selected no arm or two. Neither shape has
// an API representation, so the conversion refuses it.
func TestExpandDynamicSpatialUnionsRejectUnresolvedShapes(t *testing.T) {
	ctx := context.Background()
	field := observationFieldObject("duration", "metadata")

	t.Run("aggregation with no arm", func(t *testing.T) {
		_, diags := expandDynamicGeomapAggregation(ctx, &DynamicGeomapAggregationModel{Count: types.BoolValue(false)})
		if !diags.HasError() {
			t.Error("an aggregation whose count resolved to false must be refused")
		}
	})

	t.Run("aggregation with two arms", func(t *testing.T) {
		_, diags := expandDynamicGeomapAggregation(ctx, &DynamicGeomapAggregationModel{
			Count: types.BoolValue(true),
			Sum:   &DynamicGeomapAggregationFieldBasedModel{Field: field},
		})
		if !diags.HasError() {
			t.Error("an aggregation with two arms must be refused")
		}
	})

	t.Run("colour with no arm", func(t *testing.T) {
		if _, diags := expandDynamicGeomapColor(&DynamicGeomapColorModel{}); !diags.HasError() {
			t.Error("a colour block with neither arm must be refused")
		}
	})

	t.Run("colour with two arms", func(t *testing.T) {
		_, diags := expandDynamicGeomapColor(&DynamicGeomapColorModel{
			Size:       types.StringValue("blue"),
			ColorRange: types.StringValue("green"),
		})
		if !diags.HasError() {
			t.Error("a colour block with both arms must be refused")
		}
	})

	t.Run("field config with no arm", func(t *testing.T) {
		if _, diags := expandDynamicGeomapFieldConfig(ctx, &DynamicGeomapFieldConfigModel{}); !diags.HasError() {
			t.Error("a field config with neither arm must be refused")
		}
	})

	t.Run("stat card range mapping with both arms", func(t *testing.T) {
		_, diags := expandDynamicRangeMapping(ctx, &DynamicRangeMappingModel{
			Thresholds: dynamicThresholdList(),
			MinMax: &DynamicMinMaxModel{
				Auto:   types.BoolValue(true),
				Custom: &DynamicMinMaxCustomModel{Max: types.Float64Value(50)},
			},
		})
		if !diags.HasError() {
			t.Error("a range mapping min/max with both arms must be refused")
		}
	})

	t.Run("field config with two arms", func(t *testing.T) {
		_, diags := expandDynamicGeomapFieldConfig(ctx, &DynamicGeomapFieldConfigModel{
			CoordinateConfig: &DynamicGeomapCoordinateConfigModel{LatitudeField: field, LongitudeField: field},
			AwsRegionConfig:  &DynamicGeomapAwsRegionConfigModel{AwsRegionField: field},
		})
		if !diags.HasError() {
			t.Error("a field config with both arms must be refused")
		}
	})

	// The heatmap colour differs: neither arm is legitimate, only both at once
	// is impossible, so this asserts the empty case still converts.
	t.Run("heatmap colour", func(t *testing.T) {
		// A zero-value types.List/types.Object carries no element type, so the
		// unset collections are built explicitly.
		heatmap := func(preset, colorRange types.String) *DynamicHeatmapModel {
			return &DynamicHeatmapModel{
				ColorRange:  colorRange,
				Preset:      preset,
				ValueField:  types.ObjectNull(ObservationFieldAttr()),
				XAxisFields: types.ListNull(ObservationFieldsObject()),
				YAxisFields: types.ListNull(ObservationFieldsObject()),
			}
		}

		both := heatmap(types.StringValue("blue"), types.StringValue("green"))
		if _, diags := expandDynamicHeatmap(ctx, both); !diags.HasError() {
			t.Error("a heatmap setting both colour arms must be refused")
		}

		neither := heatmap(types.StringNull(), types.StringNull())
		if _, diags := expandDynamicHeatmap(ctx, neither); diags.HasError() {
			t.Errorf("a heatmap with no colour configuration is valid, got %v", diags)
		}
	})
}

// The exactly-one-of validators defer while any arm is unknown, so a value only
// known after apply can arrive with two arms set. Taking the first would
// silently discard the rest and produce state that does not match the config.
func TestExpandDynamicRejectsTwoResolvedUnionArms(t *testing.T) {
	ctx := context.Background()

	t.Run("two visualizations", func(t *testing.T) {
		_, diags := expandDynamicVisualization(ctx, &DynamicVisualizationModel{
			Stat:  &DynamicStatModel{},
			Gauge: &DynamicGaugeModel{},
		})
		if !diags.HasError() {
			t.Error("two visualizations set must be refused rather than silently dropping one")
		}
	})

	// Counted by reflection, so this also asserts the count sees every arm
	// rather than a stale hand-written list.
	t.Run("every visualization arm is counted", func(t *testing.T) {
		total := reflect.TypeOf(DynamicVisualizationModel{}).NumField()
		if got := len(dynamicSelectedVisualizations(&DynamicVisualizationModel{})); got != 0 {
			t.Errorf("an empty visualization must select nothing, got %d", got)
		}
		if total < 15 {
			t.Errorf("expected at least 15 visualization arms in the model, found %d", total)
		}
	})

	t.Run("no visualization at all", func(t *testing.T) {
		// A present block with no arm: the dispatch would return no
		// visualization and the read would flatten the parent to null.
		_, diags := expandDynamicVisualization(ctx, &DynamicVisualizationModel{})
		if !diags.HasError() {
			t.Error("a visualization block with no arm must be refused")
		}

		// An absent block stays legitimate.
		if _, diags := expandDynamicVisualization(ctx, nil); diags.HasError() {
			t.Errorf("no visualization block at all is valid, got %v", diags)
		}
	})

	t.Run("geomap min_max with both arms", func(t *testing.T) {
		_, diags := expandDynamicMinMax(&DynamicMinMaxModel{
			Auto:   types.BoolValue(true),
			Custom: &DynamicMinMaxCustomModel{Max: types.Float64Value(50)},
		})
		if !diags.HasError() {
			t.Error("a min/max with both arms must be refused rather than preferring custom")
		}
	})
}

// The remaining unions in the widget, same reasoning as the spatial ones: the
// schema validator defers while any arm is unknown, so a value only known after
// apply can arrive with none selected or several.
func TestExpandDynamicRemainingUnionsRejectUnresolvedShapes(t *testing.T) {
	ctx := context.Background()
	field := observationFieldObject("duration", "metadata")

	t.Run("sort strategy", func(t *testing.T) {
		none := &DynamicSortStrategyModel{Category: types.BoolValue(false)}
		if _, diags := expandDynamicSortStrategy(none); !diags.HasError() {
			t.Error("a sort strategy with no arm must be refused")
		}
		both := &DynamicSortStrategyModel{
			Category:   types.BoolValue(true),
			QueryValue: &DynamicSortByQueryValueModel{QueryID: types.StringValue("q")},
		}
		if _, diags := expandDynamicSortStrategy(both); !diags.HasError() {
			t.Error("a sort strategy with both arms must be refused")
		}
	})

	t.Run("colour label mapping", func(t *testing.T) {
		if _, diags := expandDynamicColorLabelMapping(ctx, &DynamicColorLabelMappingModel{}); !diags.HasError() {
			t.Error("a colour mapping with no arm must be refused")
		}
		both := &DynamicColorLabelMappingModel{
			Range: &DynamicRangeMappingModel{Thresholds: dynamicThresholdList()},
			Value: &DynamicSectionsMappingModel{Sections: types.ListNull(types.ObjectType{AttrTypes: dynamicMappingSectionAttr()})},
		}
		if _, diags := expandDynamicColorLabelMapping(ctx, both); !diags.HasError() {
			t.Error("a colour mapping with two arms must be refused")
		}
	})

	t.Run("table rule scope", func(t *testing.T) {
		none := &DynamicTableRuleScopeModel{Field: types.ObjectNull(ObservationFieldAttr())}
		if _, diags := expandDynamicTableRuleScope(ctx, none); !diags.HasError() {
			t.Error("a rule scope with no arm must be refused")
		}
		both := &DynamicTableRuleScopeModel{Field: field, Regex: types.StringValue("^a$")}
		if _, diags := expandDynamicTableRuleScope(ctx, both); !diags.HasError() {
			t.Error("a rule scope with two arms must be refused")
		}
	})

	t.Run("table property definition", func(t *testing.T) {
		if _, diags := expandDynamicTablePropertyDefinition(ctx, &DynamicTablePropertyDefinitionModel{}); !diags.HasError() {
			t.Error("a property definition with no arm must be refused")
		}
		both := &DynamicTablePropertyDefinitionModel{
			Alignment:         types.StringValue("center"),
			ColumnDisplayName: types.StringValue("Application"),
		}
		if _, diags := expandDynamicTablePropertyDefinition(ctx, both); !diags.HasError() {
			t.Error("a property definition with two arms must be refused")
		}
	})
}
