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

package utils

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// float64ListElements converts configured doubles into the attr.Value slice that
// the events2metrics resource hands to AttrSliceToFloat32Slice on expand.
func float64ListElements(t *testing.T, vals ...float64) []attr.Value {
	t.Helper()
	elems := make([]attr.Value, 0, len(vals))
	for _, v := range vals {
		elems = append(elems, types.Float64Value(v))
	}
	return elems
}

// flattenBuckets runs the flatten side (Float32SliceTypeList) and returns the
// resulting state doubles.
func flattenBuckets(t *testing.T, arr []float32) []float64 {
	t.Helper()
	list, diags := Float32SliceTypeList(context.Background(), arr)
	if diags.HasError() {
		t.Fatalf("Float32SliceTypeList(%v) returned diagnostics: %v", arr, diags)
	}
	elems := list.Elements()
	out := make([]float64, 0, len(elems))
	for _, e := range elems {
		f, ok := e.(types.Float64)
		if !ok {
			t.Fatalf("element %v is not a types.Float64", e)
		}
		out = append(out, f.ValueFloat64())
	}
	return out
}

// TestAttrSliceToFloat32Slice_0_0003 pins the expand side: the double 0.0003 is
// narrowed to float32(0.0003) on the wire, per the []float32 API contract. We do
// NOT try to hide this narrowing on expand.
func TestAttrSliceToFloat32Slice_0_0003(t *testing.T) {
	t.Parallel()

	got, diags := AttrSliceToFloat32Slice(context.Background(), float64ListElements(t, 0.0003))
	if diags.HasError() {
		t.Fatalf("AttrSliceToFloat32Slice returned diagnostics: %v", diags)
	}
	want := []float32{float32(0.0003)}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("AttrSliceToFloat32Slice([0.0003]) = %v, want %v", got, want)
	}
}

// TestFloat32SliceTypeList_0_0003 pins the fix: flattening float32(0.0003) must
// recover the configured double 0.0003 exactly. The old v*10000/10000 branch
// produced 0.0003000000238418579 here, causing a perpetual diff.
func TestFloat32SliceTypeList_0_0003(t *testing.T) {
	t.Parallel()

	got := flattenBuckets(t, []float32{float32(0.0003)})
	if len(got) != 1 {
		t.Fatalf("flatten([float32(0.0003)]) len = %d, want 1", len(got))
	}
	if got[0] != 0.0003 {
		t.Fatalf("flatten([float32(0.0003)])[0] = %.20g, want exactly 0.0003", got[0])
	}
}

// TestE2MHistogramBucketsRoundTrip walks the full expand→flatten path and asserts
// convergence: flatten(expand(x)) must be idempotent for every value, and equal
// the configured double for human-entered decimals.
func TestE2MHistogramBucketsRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   float64
		// want is the converged state double. For most human-entered decimals this
		// equals the input. 1234.5678 is NOT float32-representable and its shortest
		// float32 decimal is 1234.5677, so state settles there — a pre-existing
		// consequence of the []float32 API contract, not of this fix.
		want float64
	}{
		{"one_tenth", 0.1, 0.1},
		{"small_decimal", 0.0003, 0.0003},
		{"five_point_five", 5.5, 5.5},
		{"integer", 100, 100},
		{"non_float32_representable", 1234.5678, 1234.5677},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			expanded, diags := AttrSliceToFloat32Slice(context.Background(), float64ListElements(t, tt.in))
			if diags.HasError() {
				t.Fatalf("expand(%v) diagnostics: %v", tt.in, diags)
			}

			state := flattenBuckets(t, expanded)
			if len(state) != 1 {
				t.Fatalf("flatten(expand(%v)) len = %d, want 1", tt.in, len(state))
			}
			if state[0] != tt.want {
				t.Fatalf("flatten(expand(%v))[0] = %.20g, want %.20g", tt.in, state[0], tt.want)
			}

			// Idempotency: re-expanding the converged state double and flattening
			// again must yield the same value (fixed point — no further drift).
			reExpanded, diags := AttrSliceToFloat32Slice(context.Background(), float64ListElements(t, state[0]))
			if diags.HasError() {
				t.Fatalf("re-expand(%v) diagnostics: %v", state[0], diags)
			}
			state2 := flattenBuckets(t, reExpanded)
			if len(state2) != 1 || state2[0] != state[0] {
				t.Fatalf("round trip not idempotent for %v: first=%.20g second=%.20g", tt.in, state[0], state2[0])
			}
		})
	}
}
