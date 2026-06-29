// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package alerts

import (
	"context"
	"math/big"
	"testing"

	alerttypes "github.com/coralogix/terraform-provider-coralogix/internal/provider/alerts/alert_types"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	alerts "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/alert_definitions_service"
)

// TestFlattenTracingSimpleFilter_LatencyExact ensures the latency_threshold_ms
// returned by the API is preserved exactly through the flatten path. Prior to
// the fix, this used big.ParseFloat with prec=10 which silently rounded values
// outside the [1,1024] significand range — e.g. 50000 → 49984, breaking
// post-apply consistency on v2→v3 migrations.
func TestFlattenTracingSimpleFilter_LatencyExact(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected int64
	}{
		{"low value (exact at all precisions)", "1", 1},
		{"3-digit (exact at prec=10)", "100", 100},
		{"30000 — was rounded UP to 30016 by prec=10 ToNearestAway", "30000", 30000},
		{"50000 — was rounded DOWN to 49984 by prec=10", "50000", 50000},
		{"100000 — was rounded by prec=10", "100000", 100000},
		{"max int32-ish", "2147483647", 2147483647},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			filter := &alerts.TracingSimpleFilter{
				LatencyThresholdMs: &tc.input,
			}
			got, diags := flattenTracingSimpleFilter(context.Background(), filter)
			if diags.HasError() {
				t.Fatalf("flattenTracingSimpleFilter returned diagnostics: %v", diags)
			}

			var model alerttypes.TracingFilterModel
			if diags := got.As(context.Background(), &model, basetypes.ObjectAsOptions{}); diags.HasError() {
				t.Fatalf("As() returned diagnostics: %v", diags)
			}

			f := model.LatencyThresholdMs.ValueBigFloat()
			if f == nil {
				t.Fatalf("LatencyThresholdMs is nil")
			}
			i, acc := f.Int64()
			if acc != big.Exact {
				t.Fatalf("LatencyThresholdMs %v is not an exact int64 (accuracy=%v); precision was insufficient", f, acc)
			}
			if i != tc.expected {
				t.Fatalf("LatencyThresholdMs = %d, want %d (raw big.Float: %v)", i, tc.expected, f)
			}
		})
	}
}

func TestFlattenTracingSimpleFilter_InvalidLatency(t *testing.T) {
	bad := "not-a-number"
	filter := &alerts.TracingSimpleFilter{
		LatencyThresholdMs: &bad,
	}
	_, diags := flattenTracingSimpleFilter(context.Background(), filter)
	if !diags.HasError() {
		t.Fatalf("expected error for non-numeric latency, got nil diagnostics")
	}
	found := false
	for _, d := range diags.Errors() {
		if d.Summary() == "Invalid Latency Threshold Ms" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected diagnostic summary 'Invalid Latency Threshold Ms', got: %v", diags.Errors())
	}
}

func TestExtractCustomEvaluationDelay(t *testing.T) {
	cases := []struct {
		name string
		in   types.Int32
		want *int32
	}{
		{
			name: "null is omitted",
			in:   types.Int32Null(),
			want: nil,
		},
		{
			name: "unknown is omitted",
			in:   types.Int32Unknown(),
			want: nil,
		},
		{
			name: "explicit zero is preserved",
			in:   types.Int32Value(0),
			want: int32Ptr(0),
		},
		{
			name: "explicit non-zero is preserved",
			in:   types.Int32Value(60),
			want: int32Ptr(60),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractCustomEvaluationDelay(tc.in)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("extractCustomEvaluationDelay() = %v, want nil", *got)
				}
				return
			}

			if got == nil {
				t.Fatalf("extractCustomEvaluationDelay() = nil, want %v", *tc.want)
			}
			if *got != *tc.want {
				t.Fatalf("extractCustomEvaluationDelay() = %v, want %v", *got, *tc.want)
			}
		})
	}
}

func int32Ptr(v int32) *int32 {
	return &v
}
