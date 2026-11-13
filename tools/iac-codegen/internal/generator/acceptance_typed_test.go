package generator

import (
	"slices"
	"strings"
	"testing"
)

func walk(t *testing.T, attrs ...*tfAttr) string {
	t.Helper()
	s := testSynth(t, "")
	s.cov = map[string]bool{}
	out, _, err := s.attrs(attrs, "", "", accFull, true)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// typedObject has a type that names the arm, and the fields of the arms: with type "all" the object
// has no field, with type "list" it has items, and with "sum" it has one of field and observation.
func typedObject(first string, rules map[string][]typedRule, checked map[string]map[string][]string) []*tfAttr {
	values := []string{"all", "list", "sum"}
	values = slices.Insert(slices.DeleteFunc(values, func(v string) bool { return v == first }), 0, first)
	quoted := `"` + strings.Join(values, `", "`) + `"`
	kind := &tfAttr{Name: "type", Kind: "String", Required: true, Validators: []string{"stringvalidator.OneOf(" + quoted + ")"}, Typed: &typedInfo{
		Has:      map[string][]string{"all": {}, "list": {"items"}, "sum": {"field", "observation"}},
		Required: map[string][]string{},
		Checked:  checked,
		Rules:    rules,
	}}
	fields := []*tfAttr{
		{Name: "field", Kind: "String", Optional: true, TypedArm: true},
		{Name: "items", Kind: "String", Optional: true, TypedArm: true},
		{Name: "observation", Kind: "String", Optional: true, TypedArm: true},
	}
	kind.Typed.Attrs = fields
	// The type sorts after the fields that it names.
	return append(fields, kind)
}

// A config that sets every field of a typedArms object breaks the rule of its type: with the type
// "all", the items must not be set.
func TestConfigSetsOnlyTheFieldsOfTheArm(t *testing.T) {
	all := walk(t, typedObject("all", nil, nil)...)
	if strings.Contains(all, "items") || strings.Contains(all, "field") {
		t.Errorf("the type all has no field, got:\n%s", all)
	}
	list := walk(t, typedObject("list", nil, nil)...)
	if !strings.Contains(list, "items = ") || strings.Contains(list, "observation") {
		t.Errorf("the type list has items only, got:\n%s", list)
	}
}

// A rule of an arm that wants exactly one of two fields keeps the first.
func TestConfigKeepsOneOfTheFieldsOfARule(t *testing.T) {
	rules := map[string][]typedRule{"sum": {{Kind: "exactlyOneOf", Names: []string{"field", "observation"}}}}
	got := walk(t, typedObject("sum", rules, nil)...)
	if !strings.Contains(got, "field = ") || strings.Contains(got, "observation") {
		t.Errorf("want only field, got:\n%s", got)
	}
}

// The validators of a field that run for the arm give the value: another arm can allow other values.
func TestConfigUsesTheValuesThatTheArmAllows(t *testing.T) {
	checked := map[string]map[string][]string{"list": {"items": {`stringvalidator.OneOf("red", "blue")`}}}
	got := walk(t, typedObject("list", nil, checked)...)
	if !strings.Contains(got, `items = "red"`) {
		t.Errorf("want the first allowed value, got:\n%s", got)
	}
}

// A required arm is the arm of its group, also when an optional arm sorts before it.
func TestConfigKeepsTheRequiredArmOfAGroup(t *testing.T) {
	got := walk(t,
		&tfAttr{Name: "field_name", Kind: "String", Optional: true, OneOfGroup: "field_name,field_value"},
		&tfAttr{Name: "field_value", Kind: "String", Required: true, OneOfGroup: "field_name,field_value"},
	)
	if !strings.Contains(got, "field_value = ") || strings.Contains(got, "field_name") {
		t.Errorf("want only field_value, got:\n%s", got)
	}
}

// An arm that the resource refuses is no arm of a config.
func TestConfigLeavesOutARefusedArm(t *testing.T) {
	got := walk(t,
		&tfAttr{Name: "mapped_values", Kind: "Bool", Optional: true, OneOfGroup: "mapped_values,observation_field", Validators: []string{`unsupportedBoolValidator{Message: "no"}`}},
		&tfAttr{Name: "observation_field", Kind: "String", Optional: true, OneOfGroup: "mapped_values,observation_field"},
	)
	if !strings.Contains(got, "observation_field = ") || strings.Contains(got, "mapped_values") {
		t.Errorf("want only observation_field, got:\n%s", got)
	}
}

// A string that accepts a value of a OneOf or text of a pattern still has the values of the OneOf.
func TestEnumValuesReadAOneOfInsideAny(t *testing.T) {
	a := &tfAttr{Name: "window", Kind: "String", Validators: []string{
		"stringvalidator.Any(stringvalidator.OneOf(\"1_HOUR\", \"2_HOURS\"), stringvalidator.RegexMatches(regexp.MustCompile(`^[0-9]+h$`), \"\"))",
	}}
	if got := enumValues(a); !slices.Equal(got, []string{"1_HOUR", "2_HOURS"}) {
		t.Errorf("got %v", got)
	}
}
