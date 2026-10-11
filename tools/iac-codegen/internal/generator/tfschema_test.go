package generator

import "testing"

// The shape error names the field that needs another model struct, so the
// author does not have to compare the places by hand.
func TestModelFieldDifferenceNamesTheField(t *testing.T) {
	first := []tfModelField{
		{Name: "Name", Type: "types.String", TFName: "name"},
		{Name: "Selector", Type: "*SelectorModel", TFName: "selector"},
	}
	other := []tfModelField{
		{Name: "Name", Type: "types.String", TFName: "name"},
		{Name: "Selector", Type: "types.Object", TFName: "selector"},
	}
	want := "field selector is *SelectorModel in one place and types.Object in another"
	if got := modelFieldDifference(first, other); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got, want := modelFieldDifference(first, first[:1]), "the places have 2 and 1 fields"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
