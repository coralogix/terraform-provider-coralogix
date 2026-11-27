package generator

import "testing"

// The set of an arm that holds another typedArms object has the attributes of both objects. An
// attribute that the inner object merged follows the inner type, and a field of the arm itself, next
// to the inner object, follows the outer type.
func TestTypedArmSetNestedFollowsTheObjectThatMergedTheAttribute(t *testing.T) {
	outerAttrs := []*tfAttr{{Name: "selection_type"}, {Name: "selected_values"}, {Name: "extra"}}
	outer := &typedInfo{
		Has:   map[string][]string{"equals": {"selection_type", "selected_values", "extra"}},
		Attrs: outerAttrs,
	}
	inner := &typedInfo{
		Has:   map[string][]string{"all": {}, "list": {"selected_values"}},
		Attrs: []*tfAttr{{Name: "selected_values"}},
	}
	outerSet := newTypedArmSet(outer, `"equals"`, nil, false)
	for _, tc := range []struct {
		inner string
		name  string
		want  bool
	}{
		{`"list"`, "selected_values", true},
		{`"list"`, "extra", true},
		{`"all"`, "selected_values", false},
		{`"all"`, "extra", true},
		{`"all"`, "unknown", false},
	} {
		set := newTypedArmSet(inner, tc.inner, outerSet, true)
		if got := set.allows(tc.name); got != tc.want {
			t.Errorf("inner %s: allows(%s) = %v, want %v", tc.inner, tc.name, got, tc.want)
		}
	}
	if got := newTypedArmSet(inner, `"list"`, outerSet, false).allows("extra"); got {
		t.Error("a type that is no attribute of the outer object must not take its fields")
	}
}
