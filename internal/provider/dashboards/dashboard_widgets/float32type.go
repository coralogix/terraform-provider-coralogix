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
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var (
	_ basetypes.Float64Typable                    = Float32Type{}
	_ basetypes.Float64Valuable                   = Float32Value{}
	_ basetypes.Float64ValuableWithSemanticEquals = Float32Value{}
)

type Float32Type struct {
	basetypes.Float64Type
}

func (t Float32Type) Equal(o attr.Type) bool {
	other, ok := o.(Float32Type)
	if !ok {
		return false
	}
	return t.Float64Type.Equal(other.Float64Type)
}

func (t Float32Type) String() string {
	return "Float32Type"
}

func (t Float32Type) ValueFromFloat64(_ context.Context, in basetypes.Float64Value) (basetypes.Float64Valuable, diag.Diagnostics) {
	return Float32Value{Float64Value: in}, nil
}

func (t Float32Type) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	attrValue, err := t.Float64Type.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}
	float64Value, ok := attrValue.(basetypes.Float64Value)
	if !ok {
		return nil, fmt.Errorf("unexpected value type %T", attrValue)
	}
	float64Valuable, diags := t.ValueFromFloat64(ctx, float64Value)
	if diags.HasError() {
		return nil, fmt.Errorf("unexpected error converting Float64Value to Float64Valuable: %v", diags)
	}
	return float64Valuable, nil
}

func (t Float32Type) ValueType(_ context.Context) attr.Value {
	return Float32Value{}
}

type Float32Value struct {
	basetypes.Float64Value
}

func (v Float32Value) Type(_ context.Context) attr.Type {
	return Float32Type{}
}

func (v Float32Value) Equal(o attr.Value) bool {
	other, ok := o.(Float32Value)
	if !ok {
		return false
	}
	return v.Float64Value.Equal(other.Float64Value)
}

func (v Float32Value) Float64SemanticEquals(_ context.Context, newValuable basetypes.Float64Valuable) (bool, diag.Diagnostics) {
	nv, ok := newValuable.(Float32Value)
	if !ok {
		return false, nil
	}
	if v.IsNull() != nv.IsNull() || v.IsUnknown() != nv.IsUnknown() {
		return false, nil
	}
	if v.IsNull() || v.IsUnknown() {
		return true, nil
	}
	return float32(v.ValueFloat64()) == float32(nv.ValueFloat64()), nil
}
