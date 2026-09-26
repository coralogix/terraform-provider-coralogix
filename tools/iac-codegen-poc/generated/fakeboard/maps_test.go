// This file is handwritten. It tests the generated code for maps: a map of
// strings, a map of objects with a oneOf in each value, and a map of 64-bit
// numbers inside a nested object.

package fakeboard

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func labels(kv ...string) types.Map {
	elems := map[string]attr.Value{}
	for i := 0; i < len(kv); i += 2 {
		elems[kv[i]] = types.StringValue(kv[i+1])
	}
	return types.MapValueMust(types.StringType, elems)
}

// panels returns a map of panels. A panel without thresholds gets a typed
// null map: a zero types.Map has no element type.
func panels(t *testing.T, ps map[string]PanelModel) types.Map {
	t.Helper()
	for k, p := range ps {
		if p.Thresholds.ElementType(context.Background()) == nil {
			p.Thresholds = types.MapNull(types.Float64Type)
		}
		if p.Filters.ElementType(context.Background()) == nil {
			p.Filters = types.ListNull(types.ObjectType{AttrTypes: filterAttrTypes()})
		}
		ps[k] = p
	}
	m, diags := types.MapValueFrom(context.Background(), types.ObjectType{AttrTypes: panelAttrTypes()}, ps)
	assertNoDiags(t, diags)
	return m
}

func widths(kv map[string]int64) types.Map {
	elems := map[string]attr.Value{}
	for k, v := range kv {
		elems[k] = types.Int64Value(v)
	}
	return types.MapValueMust(types.Int64Type, elems)
}

// withMaps returns full() with all three maps set.
func withMaps(t *testing.T) *FakeBoardModel {
	m := full(t)
	m.Labels = labels("team", "infra", "env", "")
	m.Panels = panels(t, map[string]PanelModel{
		"cpu": {Query: types.StringValue("avg(cpu)"), Unit: types.StringValue("PERCENT"), Style: bold()},
		"mem": {Query: types.StringValue("avg(mem)")},
	})
	m.Layout.Section.Widths = widths(map[string]int64{"a": 10, "b": 0})
	return m
}

func TestExpandMaps(t *testing.T) {
	body, diags := expandCreate(context.Background(), withMaps(t))
	assertNoDiags(t, diags)
	assertJSON(t, body.Labels, `{"team":"infra","env":""}`)
	assertJSON(t, body.Panels, `{"cpu":{"query":"avg(cpu)","unit":"PERCENT","style":{"bold":{}}},"mem":{"query":"avg(mem)"}}`)
	assertJSON(t, body.Layout.Section.Widths, `{"a":"10","b":"0"}`)
}

// TestExpandMapNullAndEmpty checks that a null map is not sent, and that an
// empty map is sent as {} (contract: {} clears a map).
func TestExpandMapNullAndEmpty(t *testing.T) {
	m := full(t)
	body, diags := expandUpdate(context.Background(), m)
	assertNoDiags(t, diags)
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "labels") || strings.Contains(string(b), "panels") || strings.Contains(string(b), "widths") {
		t.Errorf("null maps were sent: %s", b)
	}

	m.Labels = labels()
	m.Panels = panels(t, map[string]PanelModel{})
	m.Layout.Section.Widths = widths(nil)
	body, diags = expandUpdate(context.Background(), m)
	assertNoDiags(t, diags)
	assertJSON(t, body.Labels, `{}`)
	assertJSON(t, body.Panels, `{}`)
	assertJSON(t, body.Layout.Section.Widths, `{}`)
}

func TestRoundTripMaps(t *testing.T) {
	const resp = `{"id":"b1","name":"ops","labels":{"team":"infra"},
		"panels":{"cpu":{"query":"q","style":{"font":{"size":"8"}}}},
		"layout":{"relativeTime":{},"title":"T","section":{"widths":{"a":"3"}}}}`
	m, diags := flatten(context.Background(), unmarshalBoard(t, resp))
	assertNoDiags(t, diags)
	body, diags := expandUpdate(context.Background(), m)
	assertNoDiags(t, diags)
	assertJSON(t, body.Labels, `{"team":"infra"}`)
	assertJSON(t, body.Panels, `{"cpu":{"query":"q","style":{"font":{"size":"8"}}}}`)
	assertJSON(t, body.Layout.Section.Widths, `{"a":"3"}`)
}

// TestFlattenMapNullAndEmpty checks that a missing map is null, and that {}
// is an empty map, not null.
func TestFlattenMapNullAndEmpty(t *testing.T) {
	m, diags := flatten(context.Background(), unmarshalBoard(t, `{"id":"b1","name":"ops","layout":{"relativeTime":{},"title":"T","section":{}}}`))
	assertNoDiags(t, diags)
	if !m.Labels.IsNull() || !m.Panels.IsNull() || !m.Layout.Section.Widths.IsNull() {
		t.Errorf("labels, panels, widths = %v, %v, %v, want null", m.Labels, m.Panels, m.Layout.Section.Widths)
	}
	m, diags = flatten(context.Background(), unmarshalBoard(t, `{"id":"b1","name":"ops","labels":{},"panels":{},"layout":{"relativeTime":{},"title":"T","section":{"widths":{}}}}`))
	assertNoDiags(t, diags)
	for name, v := range map[string]types.Map{"labels": m.Labels, "panels": m.Panels, "widths": m.Layout.Section.Widths} {
		if v.IsNull() || len(v.Elements()) != 0 {
			t.Errorf("%s = %v, want an empty map", name, v)
		}
	}
}

// TestMapErrorPaths checks that an error in a map value names the key.
func TestMapErrorPaths(t *testing.T) {
	m := withMaps(t)
	m.Layout.Section.Widths = widths(map[string]int64{"a": -1})
	_, diags := expandCreate(context.Background(), m)
	want := path.Root("layout").AtName("section").AtName("widths").AtMapKey("a")
	if d, ok := firstError(diags); !ok || !d.Path().Equal(want) {
		t.Errorf("diagnostics = %v, want an error at %s", diags, want)
	}

	m = withMaps(t)
	both := &TextStyleModel{Bold: &BoldStyleModel{}, Font: &FontStyleModel{Size: types.Int64Value(1)}}
	m.Panels = panels(t, map[string]PanelModel{"cpu": {Query: types.StringValue("q"), Style: both}})
	diags = validateConfig(t, m)
	// The validators are on the arms, so each set arm reports the conflict.
	if !diags.HasError() {
		t.Error("no error, want a conflict at panels[\"cpu\"].style")
	}
	for _, e := range diags.Errors() {
		if d, ok := e.(diag.DiagnosticWithPath); !ok || !strings.HasPrefix(d.Path().String(), `panels["cpu"].style.`) {
			t.Errorf("diagnostics = %v, want conflicts only at panels[\"cpu\"].style", diags)
		}
	}
}

func firstError(diags diag.Diagnostics) (diag.DiagnosticWithPath, bool) {
	if diags.ErrorsCount() != 1 {
		return nil, false
	}
	d, ok := diags.Errors()[0].(diag.DiagnosticWithPath)
	return d, ok
}

// TestUpdateMaskMaps checks that a change anywhere in a map puts the whole
// map in the mask, and that a removed map is in the mask and not in the body.
func TestUpdateMaskMaps(t *testing.T) {
	cases := []struct {
		name   string
		change func(m *FakeBoardModel)
		mask   string
		absent string // a field that must not be in the body
	}{
		{"label value", func(m *FakeBoardModel) { m.Labels = labels("team", "web", "env", "") }, "labels", ""},
		{"new label key", func(m *FakeBoardModel) { m.Labels = labels("team", "infra", "env", "", "x", "y") }, "labels", ""},
		{"value inside a panel", func(m *FakeBoardModel) {
			m.Panels = panels(t, map[string]PanelModel{
				"cpu": {Query: types.StringValue("max(cpu)"), Unit: types.StringValue("PERCENT"), Style: bold()},
				"mem": {Query: types.StringValue("avg(mem)")},
			})
		}, "panels", ""},
		{"remove labels", func(m *FakeBoardModel) { m.Labels = types.MapNull(types.StringType) }, "labels", "labels"},
		{"nested map", func(m *FakeBoardModel) { m.Layout.Section.Widths = widths(map[string]int64{"a": 11, "b": 0}) }, "layout.section.widths", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan := withMaps(t)
			c.change(plan)
			body, diags := updateRequest(context.Background(), toPlan(t, plan), toState(t, withMaps(t)))
			assertNoDiags(t, diags)
			if body == nil || body.UpdateMask == nil || *body.UpdateMask != c.mask {
				t.Fatalf("body = %+v, want mask %q", body, c.mask)
			}
			b, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			var keys map[string]json.RawMessage
			if err := json.Unmarshal(b, &keys); err != nil {
				t.Fatal(err)
			}
			if _, ok := keys[c.absent]; c.absent != "" && ok {
				t.Errorf("body %s has %s, want it cleared", b, c.absent)
			}
		})
	}
}
