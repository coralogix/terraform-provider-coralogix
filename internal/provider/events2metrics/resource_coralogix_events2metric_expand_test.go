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

package events2metrics

import (
	"context"
	"testing"

	e2ms "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/events2metrics_service"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestExpandE2MAggregationsSimple(t *testing.T) {
	aggs, diags := expandE2MAggregations(context.Background(), &AggregationsModel{
		Min:   &CommonAggregationModel{Enable: types.BoolValue(true)},
		Max:   &CommonAggregationModel{Enable: types.BoolValue(false)},
		Count: &CommonAggregationModel{Enable: types.BoolValue(true)},
		AVG:   &CommonAggregationModel{Enable: types.BoolValue(false)},
		Sum:   &CommonAggregationModel{Enable: types.BoolValue(true)},
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(aggs) != 5 {
		t.Fatalf("expected 5 aggregations, got %d", len(aggs))
	}

	byType := map[e2ms.AggType]e2ms.V2Aggregation{}
	for _, agg := range aggs {
		if agg.AggType == nil {
			t.Fatal("expected aggType to be set")
		}
		if agg.None != nil || agg.Samples != nil || agg.Histogram != nil {
			t.Fatalf("simple aggregations must leave none/samples/histogram unset, got %+v", agg)
		}
		if agg.Enabled == nil {
			t.Fatal("expected enabled pointer to be set")
		}
		byType[*agg.AggType] = agg
	}

	assertEnabled(t, byType[e2ms.AGGTYPE_AGG_TYPE_MIN], true, "min")
	assertEnabled(t, byType[e2ms.AGGTYPE_AGG_TYPE_MAX], false, "max")
	assertEnabled(t, byType[e2ms.AGGTYPE_AGG_TYPE_COUNT], true, "count")
	assertEnabled(t, byType[e2ms.AGGTYPE_AGG_TYPE_AVG], false, "avg")
	assertEnabled(t, byType[e2ms.AGGTYPE_AGG_TYPE_SUM], true, "sum")
	assertTarget(t, byType[e2ms.AGGTYPE_AGG_TYPE_MIN], "min")
	assertTarget(t, byType[e2ms.AGGTYPE_AGG_TYPE_MAX], "max")
	assertTarget(t, byType[e2ms.AGGTYPE_AGG_TYPE_COUNT], "count")
	assertTarget(t, byType[e2ms.AGGTYPE_AGG_TYPE_AVG], "avg")
	assertTarget(t, byType[e2ms.AGGTYPE_AGG_TYPE_SUM], "sum")
}

func TestExpandE2MAggregationsSamples(t *testing.T) {
	aggs, diags := expandE2MAggregations(context.Background(), &AggregationsModel{
		Samples: &SamplesAggregationModel{
			Enable: types.BoolValue(true),
			Type:   types.StringValue("Min"),
		},
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(aggs) != 1 {
		t.Fatalf("expected 1 aggregation, got %d", len(aggs))
	}
	agg := aggs[0]
	if agg.AggType == nil || *agg.AggType != e2ms.AGGTYPE_AGG_TYPE_SAMPLES {
		t.Fatalf("expected samples aggType, got %v", agg.AggType)
	}
	if agg.None != nil || agg.Histogram != nil {
		t.Fatalf("samples aggregation must leave none/histogram unset, got %+v", agg)
	}
	if agg.Samples == nil || agg.Samples.SampleType == nil || *agg.Samples.SampleType != e2ms.SAMPLETYPE_SAMPLE_TYPE_MIN {
		t.Fatalf("expected samples.sampleType=Min, got %+v", agg.Samples)
	}
	assertTarget(t, agg, "samples")
}

func TestExpandE2MAggregationsHistogram(t *testing.T) {
	buckets, diags := types.ListValue(types.Float64Type, []attr.Value{
		types.Float64Value(0.1),
		types.Float64Value(5.5),
		types.Float64Value(100),
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	aggs, diags := expandE2MAggregations(context.Background(), &AggregationsModel{
		Histogram: &HistogramAggregationModel{
			Enable:  types.BoolValue(true),
			Buckets: buckets,
		},
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(aggs) != 1 {
		t.Fatalf("expected 1 aggregation, got %d", len(aggs))
	}
	agg := aggs[0]
	if agg.AggType == nil || *agg.AggType != e2ms.AGGTYPE_AGG_TYPE_HISTOGRAM {
		t.Fatalf("expected histogram aggType, got %v", agg.AggType)
	}
	if agg.None != nil || agg.Samples != nil {
		t.Fatalf("histogram aggregation must leave none/samples unset, got %+v", agg)
	}
	if agg.Histogram == nil || len(agg.Histogram.Buckets) != 3 {
		t.Fatalf("expected 3 buckets, got %+v", agg.Histogram)
	}
	assertTarget(t, agg, "histogram")
}

func TestExpandE2MAggregationsEmpty(t *testing.T) {
	aggs, diags := expandE2MAggregations(context.Background(), nil)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if aggs == nil {
		t.Fatal("expected non-nil empty aggregations slice")
	}
	if len(aggs) != 0 {
		t.Fatalf("expected empty aggregations, got %d", len(aggs))
	}
}

func TestExpandE2MAggregationsMixed(t *testing.T) {
	buckets, diags := types.ListValue(types.Float64Type, []attr.Value{types.Float64Value(1)})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	aggs, diags := expandE2MAggregations(context.Background(), &AggregationsModel{
		AVG: &CommonAggregationModel{Enable: types.BoolValue(true)},
		Samples: &SamplesAggregationModel{
			Enable: types.BoolValue(false),
			Type:   types.StringValue("Max"),
		},
		Histogram: &HistogramAggregationModel{
			Enable:  types.BoolValue(true),
			Buckets: buckets,
		},
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(aggs) != 3 {
		t.Fatalf("expected 3 aggregations, got %d", len(aggs))
	}

	byType := map[e2ms.AggType]e2ms.V2Aggregation{}
	for _, agg := range aggs {
		byType[*agg.AggType] = agg
	}
	if byType[e2ms.AGGTYPE_AGG_TYPE_AVG].Samples != nil || byType[e2ms.AGGTYPE_AGG_TYPE_AVG].Histogram != nil || byType[e2ms.AGGTYPE_AGG_TYPE_AVG].None != nil {
		t.Fatalf("avg must leave metadata unset, got %+v", byType[e2ms.AGGTYPE_AGG_TYPE_AVG])
	}
	if byType[e2ms.AGGTYPE_AGG_TYPE_SAMPLES].Samples == nil || byType[e2ms.AGGTYPE_AGG_TYPE_SAMPLES].Histogram != nil {
		t.Fatalf("samples must set only samples metadata, got %+v", byType[e2ms.AGGTYPE_AGG_TYPE_SAMPLES])
	}
	if byType[e2ms.AGGTYPE_AGG_TYPE_HISTOGRAM].Histogram == nil || byType[e2ms.AGGTYPE_AGG_TYPE_HISTOGRAM].Samples != nil {
		t.Fatalf("histogram must set only histogram metadata, got %+v", byType[e2ms.AGGTYPE_AGG_TYPE_HISTOGRAM])
	}
}

func TestExtractCreateUpdateOptionalStringAsymmetry(t *testing.T) {
	ctx := context.Background()
	plan := Events2MetricResourceModel{
		ID:          types.StringValue("11111111-1111-1111-1111-111111111111"),
		Name:        types.StringValue("tf_e2m"),
		Description: types.StringNull(),
		LogsQuery: &LogsQueryModel{
			Lucene:       types.StringNull(),
			Applications: types.SetNull(types.StringType),
			Subsystems:   types.SetNull(types.StringType),
			Severities:   types.SetNull(types.StringType),
		},
		MetricFields: types.MapNull(types.ObjectType{AttrTypes: metricFieldModelAttr()}),
		MetricLabels: types.MapNull(types.StringType),
		Permutations: types.ObjectNull(permutationsModelAttr()),
	}

	createParams, diags := extractCreateE2M(ctx, plan)
	if diags.HasError() {
		t.Fatalf("unexpected create diagnostics: %v", diags)
	}
	if createParams.Description != nil {
		t.Fatalf("create must omit null description, got %q", *createParams.Description)
	}
	if createParams.LogsQuery == nil || createParams.LogsQuery.Lucene != nil {
		t.Fatalf("create must omit null lucene, got %+v", createParams.LogsQuery)
	}
	if createParams.PermutationsLimit == nil || *createParams.PermutationsLimit != 0 {
		t.Fatalf("create must send permutationsLimit=0 when block absent, got %v", createParams.PermutationsLimit)
	}

	replaceParams, diags := extractUpdateE2M(ctx, plan)
	if diags.HasError() {
		t.Fatalf("unexpected update diagnostics: %v", diags)
	}
	if replaceParams.Description == nil || *replaceParams.Description != "" {
		t.Fatalf("update must send empty description pointer for null, got %v", replaceParams.Description)
	}
	if replaceParams.LogsQuery == nil || replaceParams.LogsQuery.Lucene == nil || *replaceParams.LogsQuery.Lucene != "" {
		t.Fatalf("update must send empty lucene pointer for null, got %+v", replaceParams.LogsQuery)
	}
}

func TestFlattenE2MAggregationsUnknownType(t *testing.T) {
	unknown := e2ms.AggType("AGG_TYPE_UNSPECIFIED")
	minType := e2ms.AGGTYPE_AGG_TYPE_MIN
	flattened, diags := flattenE2MAggregations(context.Background(), []e2ms.V2Aggregation{
		{AggType: &unknown, Enabled: ptr(true), TargetMetricName: ptr("x")},
		{AggType: &minType, Enabled: ptr(true), TargetMetricName: ptr("min")},
	})
	if diags.HasError() {
		t.Fatalf("unrecognized aggregation types must be skipped, not fail: %v", diags)
	}
	if flattened == nil || flattened.Min == nil {
		t.Fatal("expected known min aggregation to flatten")
	}
	if flattened.Max != nil || flattened.Histogram != nil || flattened.Samples != nil {
		t.Fatalf("unrecognized aggregation must be skipped, got %+v", flattened)
	}
}

func TestFlattenE2MHistogramAggregationEmptyBuckets(t *testing.T) {
	aggType := e2ms.AGGTYPE_AGG_TYPE_HISTOGRAM
	flattened, diags := flattenE2MAggregations(context.Background(), []e2ms.V2Aggregation{
		{
			AggType:          &aggType,
			Enabled:          ptr(false),
			TargetMetricName: ptr("cx_bucket"),
			Histogram: &e2ms.E2MAggHistogram{
				Buckets: []float32{},
			},
		},
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if flattened == nil || flattened.Histogram == nil {
		t.Fatal("expected histogram aggregation")
	}
	if flattened.Histogram.Buckets.IsNull() {
		t.Fatal("empty buckets must flatten to an empty list, not null")
	}
	if flattened.Histogram.Buckets.IsUnknown() {
		t.Fatal("empty buckets must be known")
	}
	if len(flattened.Histogram.Buckets.Elements()) != 0 {
		t.Fatalf("expected empty buckets list, got %v", flattened.Histogram.Buckets.Elements())
	}
}

func assertEnabled(t *testing.T, agg e2ms.V2Aggregation, want bool, name string) {
	t.Helper()
	if agg.Enabled == nil || *agg.Enabled != want {
		t.Fatalf("%s: expected enabled=%v, got %v", name, want, agg.Enabled)
	}
}

func assertTarget(t *testing.T, agg e2ms.V2Aggregation, want string) {
	t.Helper()
	if agg.TargetMetricName == nil || *agg.TargetMetricName != want {
		t.Fatalf("expected target_metric_name=%q, got %v", want, agg.TargetMetricName)
	}
}
