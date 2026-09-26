// This file is handwritten. It tests the generated code for number types:
// int32, signed int64, float (float32), lists of int32 and float, a map of
// bools, and a map of doubles.

package fakeboard

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// withNumbers returns withMaps with every number field set.
func withNumbers(t *testing.T) *FakeBoardModel {
	m := withMaps(t)
	m.Flags = types.MapValueMust(types.BoolType, map[string]attr.Value{"beta": types.BoolValue(true), "old": types.BoolValue(false)})
	m.Layout.Section.Header.Style.Font.Scale = types.Float32Value(0.1)
	m.Layout.Section.Columns = types.ListValueMust(types.Int32Type, []attr.Value{types.Int32Value(2), types.Int32Value(1)})
	m.Layout.Section.Ratios = types.ListValueMust(types.Float32Type, []attr.Value{types.Float32Value(0.5), types.Float32Value(0.1)})
	m.Layout.Section.Rows = rows(t, RowModel{Height: types.Int64Value(1), Offset: types.Int64Value(-5)})
	m.Panels = panels(t, map[string]PanelModel{"cpu": {
		Query:      types.StringValue("q"),
		Precision:  types.Int32Value(3),
		Thresholds: types.MapValueMust(types.Float64Type, map[string]attr.Value{"warn": types.Float64Value(0.8)}),
	}})
	return m
}

// TestNumberSchemaTypes checks that each spec format has its own Terraform
// type. A float in a Float64 attribute would read back with other digits.
func TestNumberSchemaTypes(t *testing.T) {
	ctx := context.Background()
	cases := map[string]attr.Type{
		"flags":                                  types.MapType{ElemType: types.BoolType},
		"layout.section.header.style.font.scale": types.Float32Type,
		"layout.section.columns":                 types.ListType{ElemType: types.Int32Type},
		"layout.section.ratios":                  types.ListType{ElemType: types.Float32Type},
	}
	for p, want := range cases {
		steps := path.Empty()
		for _, name := range splitPath(p) {
			steps = steps.AtName(name)
		}
		got, diags := Schema().TypeAtPath(ctx, steps)
		assertNoDiags(t, diags)
		if !got.Equal(want) {
			t.Errorf("%s: type %s, want %s", p, got, want)
		}
	}
	panel := Schema().Attributes["panels"].(schema.MapNestedAttribute).NestedObject.Attributes
	if _, ok := panel["precision"].(schema.Int32Attribute); !ok {
		t.Errorf("panels.precision is %T, want schema.Int32Attribute", panel["precision"])
	}
	if got := panel["thresholds"].GetType(); !got.Equal(types.MapType{ElemType: types.Float64Type}) {
		t.Errorf("panels.thresholds: type %s, want map of Float64", got)
	}
}

func splitPath(p string) []string {
	var out []string
	start := 0
	for i := range len(p) + 1 {
		if i == len(p) || p[i] == '.' {
			out = append(out, p[start:i])
			start = i + 1
		}
	}
	return out
}

func TestExpandNumbers(t *testing.T) {
	body, diags := expandCreate(context.Background(), withNumbers(t))
	assertNoDiags(t, diags)
	assertJSON(t, body.Flags, `{"beta":true,"old":false}`)
	assertJSON(t, body.Layout.Section.Header.Style.Font, `{"family":"mono","size":"12","scale":0.1}`)
	assertJSON(t, body.Layout.Section.Columns, `[2,1]`)
	assertJSON(t, body.Layout.Section.Ratios, `[0.5,0.1]`)
	assertJSON(t, body.Layout.Section.Rows, `[{"height":"1","offset":-5}]`)
	assertJSON(t, body.Panels, `{"cpu":{"query":"q","precision":3,"thresholds":{"warn":0.8}}}`)
}

// TestRoundTripNumbers reads numbers from a response and sends them back. A
// float keeps its digits: 0.1 stays 0.1 in the state.
func TestRoundTripNumbers(t *testing.T) {
	const resp = `{"id":"b1","name":"ops","flags":{"x":false},
		"panels":{"cpu":{"query":"q","precision":0,"thresholds":{}}},
		"layout":{"relativeTime":{},"title":"T","section":{"columns":[],"ratios":[0.1],
			"header":{"text":"h","style":{"font":{"scale":0.1}}},
			"rows":[{"height":"1","offset":-100}]}}}`
	m, diags := flatten(context.Background(), unmarshalBoard(t, resp))
	assertNoDiags(t, diags)
	if got := m.Layout.Section.Header.Style.Font.Scale; !got.Equal(types.Float32Value(0.1)) {
		t.Errorf("scale = %s, want 0.1 exactly", got)
	}
	body, diags := expandUpdate(context.Background(), m)
	assertNoDiags(t, diags)
	assertJSON(t, body.Flags, `{"x":false}`)
	assertJSON(t, body.Panels, `{"cpu":{"query":"q","precision":0,"thresholds":{}}}`)
	assertJSON(t, body.Layout.Section.Columns, `[]`)
	assertJSON(t, body.Layout.Section.Ratios, `[0.1]`)
	assertJSON(t, body.Layout.Section.Header.Style.Font, `{"scale":0.1}`)
	assertJSON(t, body.Layout.Section.Rows, `[{"height":"1","offset":-100}]`)
}

// TestFlattenNumbersNull checks that a missing list or map is null.
func TestFlattenNumbersNull(t *testing.T) {
	m, diags := flatten(context.Background(), unmarshalBoard(t, `{"id":"b1","name":"ops","layout":{"relativeTime":{},"title":"T","section":{}}}`))
	assertNoDiags(t, diags)
	if !m.Flags.IsNull() || !m.Layout.Section.Columns.IsNull() || !m.Layout.Section.Ratios.IsNull() {
		t.Errorf("flags, columns, ratios = %s, %s, %s, want null", m.Flags, m.Layout.Section.Columns, m.Layout.Section.Ratios)
	}
}

// TestNumberRangeValidators checks the spec ranges: precision 0..10 (int32)
// and offset -100..100 (int64).
func TestNumberRangeValidators(t *testing.T) {
	ctx := context.Background()
	panel := Schema().Attributes["panels"].(schema.MapNestedAttribute).NestedObject.Attributes
	precision := panel["precision"].(schema.Int32Attribute)
	for v, wantErr := range map[int32]bool{0: false, 10: false, 11: true, -1: true} {
		var resp validator.Int32Response
		for _, val := range precision.Validators {
			val.ValidateInt32(ctx, validator.Int32Request{ConfigValue: types.Int32Value(v)}, &resp)
		}
		if resp.Diagnostics.HasError() != wantErr {
			t.Errorf("precision %d: error = %t, want %t", v, resp.Diagnostics.HasError(), wantErr)
		}
	}
	section := Schema().Attributes["layout"].(schema.SingleNestedAttribute).Attributes["section"].(schema.SingleNestedAttribute)
	row := section.Attributes["rows"].(schema.ListNestedAttribute).NestedObject.Attributes
	offset := row["offset"].(schema.Int64Attribute)
	for v, wantErr := range map[int64]bool{-100: false, 100: false, -101: true} {
		var resp validator.Int64Response
		for _, val := range offset.Validators {
			val.ValidateInt64(ctx, validator.Int64Request{ConfigValue: types.Int64Value(v)}, &resp)
		}
		if resp.Diagnostics.HasError() != wantErr {
			t.Errorf("offset %d: error = %t, want %t", v, resp.Diagnostics.HasError(), wantErr)
		}
	}
}

func TestLeafMaskNumbers(t *testing.T) {
	cases := []struct {
		name   string
		change func(m *FakeBoardModel)
		mask   string
	}{
		{"float leaf", func(m *FakeBoardModel) {
			m.Layout.Section.Header.Style.Font.Scale = types.Float32Value(0.2)
		}, "layout.section.header.style.font.scale"},
		{"int32 list", func(m *FakeBoardModel) {
			m.Layout.Section.Columns = types.ListValueMust(types.Int32Type, []attr.Value{types.Int32Value(1), types.Int32Value(2)})
		}, "layout.section.columns"},
		{"map of bools", func(m *FakeBoardModel) {
			m.Flags = types.MapValueMust(types.BoolType, map[string]attr.Value{"beta": types.BoolValue(false), "old": types.BoolValue(false)})
		}, "flags"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan := withNumbers(t)
			c.change(plan)
			body, diags := updateRequest(context.Background(), toPlan(t, plan), toState(t, withNumbers(t)))
			assertNoDiags(t, diags)
			if got := maskOf(body); got != c.mask {
				t.Errorf("mask = %q, want %q", got, c.mask)
			}
		})
	}
}
