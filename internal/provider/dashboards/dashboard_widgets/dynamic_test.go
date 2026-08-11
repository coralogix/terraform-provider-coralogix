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
				ValueField: types.ObjectValueMust(ObservationFieldAttr(), map[string]attr.Value{
					"keypath": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("duration")}),
					"scope":   types.StringValue("metadata"),
				}),
				Unit: types.StringValue("bytes"),
				Thresholds: types.ListValueMust(types.ObjectType{AttrTypes: dynamicThresholdAttr()}, []attr.Value{
					types.ObjectValueMust(dynamicThresholdAttr(), map[string]attr.Value{
						"from":  types.Float64Value(10),
						"color": types.StringValue("#ff0000"),
						"label": types.StringValue("high"),
					}),
				}),
				ThresholdType: types.StringValue("absolute"),
				Legend:        nil,
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

func objectFrom(ctx context.Context, t *testing.T, attrTypes map[string]attr.Type, value any) types.Object {
	t.Helper()
	object, diags := types.ObjectValueFrom(ctx, attrTypes, value)
	if diags.HasError() {
		t.Fatalf("building object value: %v", diags)
	}
	return object
}
