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

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var (
	_ basetypes.StringTypable                    = JSONStringType{}
	_ basetypes.StringValuable                   = JSONStringValue{}
	_ basetypes.StringValuableWithSemanticEquals = JSONStringValue{}
)

type JSONStringType struct {
	basetypes.StringType
}

func (t JSONStringType) Equal(o attr.Type) bool {
	other, ok := o.(JSONStringType)
	if !ok {
		return false
	}
	return t.StringType.Equal(other.StringType)
}

func (t JSONStringType) String() string {
	return "JSONStringType"
}

func (t JSONStringType) ValueFromString(_ context.Context, in basetypes.StringValue) (basetypes.StringValuable, diag.Diagnostics) {
	return JSONStringValue{StringValue: in}, nil
}

func (t JSONStringType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	attrValue, err := t.StringType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}
	stringValue, ok := attrValue.(basetypes.StringValue)
	if !ok {
		return nil, fmt.Errorf("unexpected value type %T", attrValue)
	}
	stringValuable, diags := t.ValueFromString(ctx, stringValue)
	if diags.HasError() {
		return nil, fmt.Errorf("unexpected error converting StringValue to StringValuable: %v", diags)
	}
	return stringValuable, nil
}

func (t JSONStringType) ValueType(_ context.Context) attr.Value {
	return JSONStringValue{}
}

type JSONStringValue struct {
	basetypes.StringValue
}

func (v JSONStringValue) Type(_ context.Context) attr.Type {
	return JSONStringType{}
}

func (v JSONStringValue) Equal(o attr.Value) bool {
	other, ok := o.(JSONStringValue)
	if !ok {
		return false
	}
	return v.StringValue.Equal(other.StringValue)
}

func (v JSONStringValue) StringSemanticEquals(_ context.Context, newValuable basetypes.StringValuable) (bool, diag.Diagnostics) {
	nv, ok := newValuable.(JSONStringValue)
	if !ok {
		return false, nil
	}
	if v.IsNull() != nv.IsNull() || v.IsUnknown() != nv.IsUnknown() {
		return false, nil
	}
	if v.IsNull() || v.IsUnknown() {
		return true, nil
	}
	return utils.JSONStringsEqual(v.ValueString(), nv.ValueString()), nil
}

func NewJSONStringNull() JSONStringValue {
	return JSONStringValue{StringValue: basetypes.NewStringNull()}
}

func NewJSONStringValue(value string) JSONStringValue {
	return JSONStringValue{StringValue: basetypes.NewStringValue(value)}
}
