// This file is handwritten. It tests the generated code for nested objects:
// three levels, a oneOf in two places and inside a list item, a list of
// objects, required nested fields (SDK value types), and null at each level.

package fakeboard

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/testsdk/go/openapi/gen/fake_boards_service"
)

// rows returns a list of rows. With no rows, the list is empty, not null.
func rows(t *testing.T, rs ...RowModel) types.List {
	t.Helper()
	if rs == nil {
		rs = []RowModel{}
	}
	l, diags := types.ListValueFrom(context.Background(), types.ObjectType{AttrTypes: rowAttrTypes()}, rs)
	assertNoDiags(t, diags)
	return l
}

func bold() *TextStyleModel { return &TextStyleModel{Bold: &BoldStyleModel{}} }

func font(family string, size int64) *TextStyleModel {
	return &TextStyleModel{Font: &FontStyleModel{Family: types.StringValue(family), Size: types.Int64Value(size)}}
}

// full returns a board with every nested level set.
func full(t *testing.T) *FakeBoardModel {
	return &FakeBoardModel{
		Name:   types.StringValue("ops"),
		Flags:  types.MapNull(types.BoolType),
		Labels: types.MapNull(types.StringType),
		Panels: types.MapNull(types.ObjectType{AttrTypes: panelAttrTypes()}),
		// Computed, so a types.Object: a plan can hold it unknown.
		Routing: types.ObjectNull(routingAttrTypes()),
		Layout: &LayoutModel{
			// The time group needs exactly one arm.
			RelativeTime: &EveryModel{Minutes: types.Int32Value(15)},
			Title:        types.StringValue("Ops"),
			TitleStyle:   bold(),
			Section: &SectionModel{
				Widths:  types.MapNull(types.Int64Type),
				Columns: types.ListNull(types.Int32Type),
				Ratios:  types.ListNull(types.Float32Type),
				Header:  &HeaderModel{Text: types.StringValue("CPU"), Color: types.StringValue("RED"), Style: font("mono", 12)},
				Rows: rows(t,
					RowModel{Height: types.Int64Value(3), Label: types.StringValue("a")},
					RowModel{Height: types.Int64Value(1), Style: bold()},
				),
			},
		},
	}
}

const fullJSON = `{"name":"ops","layout":{"relativeTime":{"minutes":15},"title":"Ops","titleStyle":{"bold":{}},"section":{
	"header":{"text":"CPU","color":"RED","style":{"font":{"family":"mono","size":"12"}}},
	"rows":[{"height":"3","label":"a"},{"height":"1","style":{"bold":{}}}]}}}`

func assertJSON(t *testing.T, v any, want string) {
	t.Helper()
	got, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("bad want JSON %s: %v", want, err)
	}
	if !reflect.DeepEqual(g, w) {
		t.Errorf("JSON:\n got  %s\n want %s", got, want)
	}
}

func assertNoDiags(t *testing.T, diags diag.Diagnostics) {
	t.Helper()
	if diags.HasError() {
		t.Fatalf("diagnostics: %v", diags)
	}
}

func TestExpandCreateNested(t *testing.T) {
	body, diags := expandCreate(context.Background(), full(t))
	assertNoDiags(t, diags)
	assertJSON(t, body, fullJSON)
}

// TestExpandNullLevels checks that a null object at any level is not sent,
// and that an empty list is sent as [].
func TestExpandNullLevels(t *testing.T) {
	cases := []struct {
		name    string
		section *SectionModel
		want    string
	}{
		{"no section", nil, `{"title":"T"}`},
		{"empty section", &SectionModel{Rows: types.ListNull(types.ObjectType{AttrTypes: rowAttrTypes()})}, `{"title":"T","section":{}}`},
		{"header without style", &SectionModel{
			Header: &HeaderModel{Text: types.StringValue("h")},
			Rows:   types.ListNull(types.ObjectType{AttrTypes: rowAttrTypes()}),
		}, `{"title":"T","section":{"header":{"text":"h"}}}`},
		{"empty rows", &SectionModel{Rows: rows(t)}, `{"title":"T","section":{"rows":[]}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := &FakeBoardModel{Routing: types.ObjectNull(routingAttrTypes()),
				Layout: &LayoutModel{Title: types.StringValue("T"), Section: c.section, RelativeTime: &EveryModel{}}}
			body, diags := expandUpdate(context.Background(), m)
			assertNoDiags(t, diags)
			assertJSON(t, body.Layout, strings.Replace(c.want, `{"title":"T"`, `{"relativeTime":{},"title":"T"`, 1))
		})
	}
}

func unmarshalBoard(t *testing.T, s string) *fake_boards_service.FakeBoard {
	t.Helper()
	var v fake_boards_service.FakeBoard
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatal(err)
	}
	return &v
}

// TestRoundTripNested reads a response, and sends it back. The layout must
// not change on the way.
func TestRoundTripNested(t *testing.T) {
	const layout = `{"absoluteTime":{"from":"2026-09-26T08:00:00Z"},"title":"Ops","titleStyle":{"font":{"size":"9"}},"section":{
		"header":{"text":"CPU","style":{"bold":{}}},
		"rows":[{"height":"2","style":{"font":{"family":"mono"}}}]}}`
	v := unmarshalBoard(t, `{"id":"b1","name":"ops","description":"","layout":`+layout+`}`)
	m, diags := flatten(context.Background(), v)
	assertNoDiags(t, diags)
	if got := m.Description; !got.Equal(types.StringValue("")) {
		t.Errorf("description = %v, want \"\" (a value, not null)", got)
	}
	body, diags := expandUpdate(context.Background(), m)
	assertNoDiags(t, diags)
	assertJSON(t, body.Layout, layout)
	assertJSON(t, body.Description, `""`)
}

// TestFlattenNullLevels checks that a missing object in the response is
// null in the model, at each level.
func TestFlattenNullLevels(t *testing.T) {
	m, diags := flatten(context.Background(), unmarshalBoard(t, `{"id":"b1","name":"ops"}`))
	assertNoDiags(t, diags)
	if m.Layout != nil || !m.Description.IsNull() || !m.UpdatedAt.IsNull() {
		t.Errorf("layout = %v, description = %v, updatedAt = %v, want all null", m.Layout, m.Description, m.UpdatedAt)
	}
	if m.Id.ValueString() != "b1" || m.Name.ValueString() != "ops" {
		t.Errorf("id, name = %v, %v, want b1, ops", m.Id, m.Name)
	}

	m, diags = flatten(context.Background(), unmarshalBoard(t, `{"id":"b1","name":"ops","layout":{"title":"T","section":{"header":{"text":"h"}}}}`))
	assertNoDiags(t, diags)
	s := m.Layout.Section
	if m.Layout.TitleStyle != nil || s.Header.Style != nil || !s.Header.Color.IsNull() || !s.Rows.IsNull() {
		t.Errorf("titleStyle = %v, style = %v, color = %v, rows = %v, want all null", m.Layout.TitleStyle, s.Header.Style, s.Header.Color, s.Rows)
	}
}

// TestExpandErrorPath checks that an error inside a list item names the
// full path with the list index.
func TestExpandErrorPath(t *testing.T) {
	m := full(t)
	m.Layout.Section.Rows = rows(t,
		RowModel{Height: types.Int64Value(1)},
		RowModel{Height: types.Int64Value(-1)},
	)
	_, diags := expandCreate(context.Background(), m)
	want := path.Root("layout").AtName("section").AtName("rows").AtListIndex(1).AtName("height")
	if diags.ErrorsCount() != 1 {
		t.Fatalf("diagnostics = %v, want 1 error", diags)
	}
	d, ok := diags.Errors()[0].(diag.DiagnosticWithPath)
	if !ok || !d.Path().Equal(want) {
		t.Errorf("error = %v, want it at %s", diags.Errors()[0], want)
	}
}

// TestOneOfValidators checks the "at most one choice" validator of each
// oneOf: at level 1, at level 3, and inside a list item.
func TestOneOfValidators(t *testing.T) {
	both := &TextStyleModel{Bold: &BoldStyleModel{}, Font: &FontStyleModel{Size: types.Int64Value(1)}}
	cases := []struct {
		name string
		set  func(m *FakeBoardModel)
		want string // the attribute in the error; "" for no error
	}{
		{"one choice everywhere", func(m *FakeBoardModel) {}, ""},
		{"title style", func(m *FakeBoardModel) { m.Layout.TitleStyle = both }, "layout.title_style"},
		{"header style", func(m *FakeBoardModel) { m.Layout.Section.Header.Style = both }, "layout.section.header.style"},
		{"row style", func(m *FakeBoardModel) {
			m.Layout.Section.Rows = rows(t, RowModel{Height: types.Int64Value(1), Style: both})
		}, "layout.section.rows[0].style"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := full(t)
			c.set(m)
			diags := validateConfig(t, m)
			if c.want == "" {
				assertNoDiags(t, diags)
				return
			}
			if !diags.HasError() {
				t.Fatalf("no error, want a conflict at %s", c.want)
			}
			// The validators are on the arms, so each set arm reports it.
			for _, e := range diags.Errors() {
				d, ok := e.(diag.DiagnosticWithPath)
				if !ok || !strings.HasPrefix(d.Path().String(), c.want+".") {
					t.Errorf("diagnostics = %v, want conflicts only at %s", diags, c.want)
				}
			}
		})
	}
}

func validateConfig(t *testing.T, m *FakeBoardModel) diag.Diagnostics {
	t.Helper()
	return validateAll(t, m)
}
