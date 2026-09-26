// This file is handwritten. It tests the generated leaf update masks: the
// spec pattern accepts nested paths, so the mask names the changed leaves.

package fakeboard

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/testsdk/go/openapi/gen/fake_boards_service"
)

// TestLeafMask changes the plan and checks the update mask. The state is
// withMaps. gone is a JSON path that must not be in the body: the mask
// clears it.
func TestLeafMask(t *testing.T) {
	cases := []struct {
		name   string
		change func(m *FakeBoardModel)
		mask   string
		gone   []string
	}{
		{"leaf 3 levels deep", func(m *FakeBoardModel) {
			m.Layout.Section.Header.Text = types.StringValue("Memory")
		}, "layout.section.header.text", nil},
		{"two leaves", func(m *FakeBoardModel) {
			m.Layout.Section.Header.Text = types.StringValue("Memory")
			m.Layout.Section.Header.Color = types.StringValue("BLUE")
		}, "layout.section.header.text,layout.section.header.color", nil},
		{"clear a leaf", func(m *FakeBoardModel) {
			m.Layout.Section.Header.Color = types.StringNull()
		}, "layout.section.header.color", []string{"layout", "section", "header", "color"}},
		{"top level and a leaf", func(m *FakeBoardModel) {
			m.Description = types.StringValue("new")
			m.Layout.Title = types.StringValue("New")
		}, "description,layout.title", nil},
		{"value inside the same oneOf arm", func(m *FakeBoardModel) {
			m.Layout.Section.Header.Style = font("mono", 14)
		}, "layout.section.header.style.font.size", nil},
		{"new oneOf arm (2.6)", func(m *FakeBoardModel) {
			m.Layout.Section.Header.Style = bold()
		}, "layout.section.header.style.bold", nil},
		{"remove a oneOf (2.7)", func(m *FakeBoardModel) {
			m.Layout.Section.Header.Style = nil
		}, "layout.section.header.style.font", []string{"layout", "section", "header", "style"}},
		{"remove a oneOf at level 1", func(m *FakeBoardModel) {
			m.Layout.TitleStyle = nil
		}, "layout.titleStyle.bold", []string{"layout", "titleStyle"}},
		{"remove an object", func(m *FakeBoardModel) {
			m.Layout.Section.Header = nil
		}, "layout.section.header", []string{"layout", "section", "header"}},
		{"value inside a list item: the whole list", func(m *FakeBoardModel) {
			m.Layout.Section.Rows = rows(t,
				RowModel{Height: types.Int64Value(4), Label: types.StringValue("a")},
				RowModel{Height: types.Int64Value(1), Style: bold()},
			)
		}, "layout.section.rows", nil},
		{"nested map: the whole map", func(m *FakeBoardModel) {
			m.Layout.Section.Widths = widths(map[string]int64{"a": 10})
		}, "layout.section.widths", nil},
		{"top-level map: the whole map", func(m *FakeBoardModel) {
			m.Labels = labels("team", "web", "env", "")
		}, "labels", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan := withMaps(t)
			c.change(plan)
			body, diags := updateRequest(context.Background(), toPlan(t, plan), toState(t, withMaps(t)))
			assertNoDiags(t, diags)
			if body == nil || body.UpdateMask == nil || *body.UpdateMask != c.mask {
				t.Fatalf("mask = %v, want %q", maskOf(body), c.mask)
			}
			if c.gone != nil && hasPath(t, body, c.gone) {
				t.Errorf("the body has %v, want it left out, so the server clears it", c.gone)
			}
		})
	}
}

// TestLeafMaskAddObject checks that a new object is masked as a whole.
func TestLeafMaskAddObject(t *testing.T) {
	state := withMaps(t)
	state.Layout.Section = nil
	body, diags := updateRequest(context.Background(), toPlan(t, withMaps(t)), toState(t, state))
	assertNoDiags(t, diags)
	if got := maskOf(body); got != "layout.section" {
		t.Errorf("mask = %q, want layout.section", got)
	}
}

// TestLeafMaskNoChange checks that no request is made when nothing changed,
// also when a Set or a map is written in another order.
func TestLeafMaskNoChange(t *testing.T) {
	body, diags := updateRequest(context.Background(), toPlan(t, withMaps(t)), toState(t, withMaps(t)))
	assertNoDiags(t, diags)
	if body != nil {
		t.Errorf("body = %+v, want nil", body)
	}
}

// maskOf returns the update mask of body. It is "" for a nil body.
func maskOf(body *fake_boards_service.FakeBoardsServiceUpdateFakeBoardRequest) string {
	return body.GetUpdateMask()
}

// hasPath reports whether the JSON of v has the object path keys.
func hasPath(t *testing.T, v any, keys []string) bool {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var cur any
	if err := json.Unmarshal(b, &cur); err != nil {
		t.Fatal(err)
	}
	for _, k := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return false
		}
		if cur, ok = m[k]; !ok {
			return false
		}
	}
	return true
}
