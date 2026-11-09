package generator

import (
	"strings"
	"testing"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
)

// enumValuesResource is a resource with an enum in every place that the description can list one:
// top level, inside an object, a list of enums, a set of enums, a case-insensitive enum, and an
// enum that the validator takes without its zero value.
func enumValuesResource() *model.Resource {
	enum := func(schema string, values ...string) *model.Type {
		return &model.Type{Kind: model.Enum, Schema: schema, Values: values, EnumZero: strings.ToUpper(schema) + "_UNSPECIFIED"}
	}
	field := func(name string, t *model.Type, description string) *model.ResourceField {
		return &model.ResourceField{Name: name, Type: t, Description: description, Behavior: model.Normal, Create: &model.Attrs{}, Get: &model.Attrs{}, InGet: true}
	}
	detail := &model.Type{Kind: model.Object, Schema: "Detail", CreateSchema: "Detail", Fields: []*model.Field{
		{Name: "mode", Type: enum("Mode", "MODE_FAST", "MODE_SLOW"), Description: "The mode.", Behavior: model.Normal, Create: &model.Attrs{}},
	}}
	return &model.Resource{Name: "Thing", Fields: []*model.ResourceField{
		field("priority", enum("Priority", "PRIORITY_HIGH", "PRIORITY_LOW"), "The priority."),
		field("tier", enum("Tier", "TIER_GOLD", "TIER_SILVER"), "The tier."),
		field("kind", enum("Kind", "KIND_TEXT", "KIND_ID"), "The kind."),
		field("detail", detail, "The detail."),
		field("modes", &model.Type{Kind: model.List, Elem: enum("Mode", "MODE_FAST", "MODE_SLOW")}, "The modes."),
		field("levels", &model.Type{Kind: model.Set, Elem: enum("Level", "LEVEL_LOW", "LEVEL_HIGH")}, "The levels."),
		field("stated", enum("Priority", "PRIORITY_HIGH", "PRIORITY_LOW"), "The stated one."),
		field("already", enum("Priority", "PRIORITY_HIGH", "PRIORITY_LOW"), "Pick one. Valid values: A, B."),
		field("bare", enum("Priority", "PRIORITY_HIGH", "PRIORITY_LOW"), ""),
		field("name", &model.Type{Kind: model.String}, "The name."),
	}}
}

const enumValuesLines = `resource: Thing
mode: existing
validators:
  inferred: true
types:
  Thing:
    fields:
      tier: {validators: [{enumExceptZero: true}]}
      kind: {validators: [{enum: true}]}
      stated: {markdownDescription: "Written by hand."}
enums:
  Priority:
    zero: unspecified
    trimPrefix: PRIORITY_
    values: [PRIORITY_HIGH, PRIORITY_LOW]
    names: {PRIORITY_HIGH: P1, PRIORITY_LOW: P5}
  Tier:
    zero: unspecified
    trimPrefix: TIER_
    values: [TIER_GOLD, TIER_SILVER]
  Kind:
    zero: unspecified
    trimPrefix: KIND_
    values: [KIND_TEXT, KIND_ID]
    caseInsensitive: true
  Mode:
    zero: unspecified
    trimPrefix: MODE_
    values: [MODE_FAST, MODE_SLOW]
  Level:
    zero: unspecified
    trimPrefix: LEVEL_
    values: [LEVEL_LOW, LEVEL_HIGH]
`

func enumValuesDescriptions(t *testing.T, key string, lines string) map[string]string {
	t.Helper()
	text := lines
	if key != "" {
		text = strings.Replace(text, "mode: existing\n", "mode: existing\n"+key+"\n", 1)
	}
	data, err := buildTFResourceWith(enumValuesResource(), "thing", mustParse(t, text))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	var walk func(prefix string, attrs []*tfAttr)
	walk = func(prefix string, attrs []*tfAttr) {
		for _, a := range attrs {
			out[prefix+a.Name] = a.Description
			walk(prefix+a.Name+".", a.Attributes)
		}
	}
	walk("", data.Attributes)
	return out
}

// With enumValuesInDescription, the description lists the Terraform values of the validator: the
// zero value only when the validator accepts it, and the canonical spellings of a case-insensitive
// enum.
func TestEnumValuesInDescription(t *testing.T) {
	got := enumValuesDescriptions(t, "enumValuesInDescription: true", enumValuesLines)
	want := map[string]string{
		"priority":    `The priority. Valid values: ["P1" "P5" "unspecified"].`,
		"tier":        `The tier. Valid values: ["gold" "silver"].`,
		"kind":        `The kind. Valid values: ["id" "text" "unspecified"].`,
		"detail.mode": `The mode. Valid values: ["fast" "slow" "unspecified"].`,
		"modes":       `The modes. Valid values: ["fast" "slow" "unspecified"].`,
		"levels":      `The levels. Valid values: ["high" "low" "unspecified"].`,
		// A text that a field line states is taken as it is.
		"stated": "Written by hand.",
		// A description that has the words already is not extended.
		"already": "Pick one. Valid values: A, B.",
		// No description text: no leading space.
		"bare": `Valid values: ["P1" "P5" "unspecified"].`,
		// No enum validator.
		"name": "The name.",
	}
	for name, text := range want {
		if got[name] != text {
			t.Errorf("description of %s = %q, want %q", name, got[name], text)
		}
	}
}

// The list is the one of the validator. A resource that does not set the key keeps the
// descriptions of the contract, and so does one without an enum validator.
func TestEnumValuesInDescriptionIsOptIn(t *testing.T) {
	off := enumValuesDescriptions(t, "", enumValuesLines)
	for name, text := range off {
		if strings.Contains(text, "Valid values: [") {
			t.Errorf("description of %s = %q without the key, want no list", name, text)
		}
	}
	// Without inferred validators, only a validator of a line gives a list.
	none := enumValuesDescriptions(t, "enumValuesInDescription: true", strings.Replace(enumValuesLines, "inferred: true", "inferred: false", 1))
	for _, name := range []string{"priority", "detail.mode", "modes", "levels", "bare", "stated", "name"} {
		if strings.Contains(none[name], "Valid values") {
			t.Errorf("description of %s = %q without an enum validator, want no list", name, none[name])
		}
	}
	for _, name := range []string{"tier", "kind"} {
		if !strings.Contains(none[name], "Valid values: [") {
			t.Errorf("description of %s = %q, want the list of its enum line", name, none[name])
		}
	}
}

// A resource with no enum keeps its schema when the key is set.
func TestEnumValuesInDescriptionWithoutEnum(t *testing.T) {
	resource := &model.Resource{Name: "Thing", Fields: []*model.ResourceField{
		{Name: "name", Type: &model.Type{Kind: model.String}, Description: "The name.", Behavior: model.Normal, Create: &model.Attrs{Required: true}, Get: &model.Attrs{}, InGet: true},
	}}
	build := func(key string) string {
		data, err := buildTFResourceWith(resource, "thing", mustParse(t, "resource: Thing\nmode: existing\nvalidators:\n  inferred: true\n"+key))
		if err != nil {
			t.Fatal(err)
		}
		return data.Attributes[0].Description
	}
	if on, off := build("enumValuesInDescription: true\n"), build(""); on != off || on != "The name." {
		t.Fatalf("description with the key = %q, without = %q, want both %q", on, off, "The name.")
	}
}
