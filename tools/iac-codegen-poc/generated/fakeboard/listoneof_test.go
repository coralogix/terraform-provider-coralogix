// This file is handwritten. It tests a list whose items are a oneOf
// (Panel.filters, like the dashboards query filters), and that each list item
// is validated on its own (F49).

package fakeboard

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func match(field string) *MatchModel {
	return &MatchModel{Field: types.StringValue(field), Value: types.StringValue("v")}
}

func withFilters(t *testing.T, filters ...FilterModel) *FakeBoardModel {
	t.Helper()
	m := withGroups(t)
	list, diags := types.ListValueFrom(context.Background(), types.ObjectType{AttrTypes: filterAttrTypes()}, filters)
	assertNoDiags(t, diags)
	m.Panels = panels(t, map[string]PanelModel{"cpu": {Query: types.StringValue("q"), Filters: list}})
	return m
}

// TestListOfOneOf checks expand and flatten of a list of oneOf: each item
// keeps its own arm.
func TestListOfOneOf(t *testing.T) {
	ctx := context.Background()
	m := withFilters(t, FilterModel{Equals: match("level")}, FilterModel{Contains: match("text")})
	body, diags := expandCreate(ctx, m)
	assertNoDiags(t, diags)
	got := body.Panels["cpu"].Filters
	if len(got) != 2 || got[0].Equals == nil || got[0].Contains != nil || got[1].Contains == nil || got[1].Equals != nil {
		t.Fatalf("filters = %+v, want equals then contains", got)
	}
	if got[0].Equals.Field != "level" || got[1].Contains.Field != "text" {
		t.Errorf("filters = %+v", got)
	}
}

// TestListItemValidators checks the oneOf validators of list items through a
// provider server: each item on its own.
func TestListItemValidators(t *testing.T) {
	cases := []struct {
		name    string
		filters []FilterModel
		want    string // the path prefix of every error; "" for no error
	}{
		{"a different arm in each item", []FilterModel{{Equals: match("a")}, {Contains: match("b")}}, ""},
		{"two arms in one item", []FilterModel{{Equals: match("a")}, {Equals: match("b"), Contains: match("c")}},
			`panels["cpu"].filters[1].`},
		{"no arm in one item", []FilterModel{{}, {Equals: match("b")}}, `panels["cpu"].filters[0].`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertErrorsAt(t, validateAll(t, withFilters(t, c.filters...)), c.want)
		})
	}
}

// TestRowsWithDifferentArms is the F49 regression: row 0 bold and row 1 with
// a font is valid. A resource validator with a list wildcard rejected it.
func TestRowsWithDifferentArms(t *testing.T) {
	m := withGroups(t)
	font := &FontStyleModel{Family: types.StringValue("mono"), Scale: types.Float32Null(), Size: types.Int64Null()}
	m.Layout.Section.Rows = rows(t,
		RowModel{Height: types.Int64Value(1), Style: &TextStyleModel{Bold: &BoldStyleModel{}}},
		RowModel{Height: types.Int64Value(1), Style: &TextStyleModel{Font: font}})
	assertErrorsAt(t, validateAll(t, m), "")
}

// assertErrorsAt checks that there is no error when want is "", else that
// there is an error and every error path starts with want.
func assertErrorsAt(t *testing.T, diags diag.Diagnostics, want string) {
	t.Helper()
	if want == "" {
		if diags.HasError() {
			t.Errorf("errors: %s", diagText(diags))
		}
		return
	}
	if !diags.HasError() {
		t.Fatalf("no error, want one at %s", want)
	}
	for _, e := range diags.Errors() {
		if d, ok := e.(diag.DiagnosticWithPath); !ok || !strings.HasPrefix(d.Path().String(), want) {
			t.Errorf("error %q, want it at %s", diagText(diag.Diagnostics{e}), want)
		}
	}
}
