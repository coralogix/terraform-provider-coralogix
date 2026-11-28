package generator

import (
	"testing"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
	iacoverrides "github.com/coralogix/coralogix-management-sdk/iac/overrides"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

const asListSpec = `openapi: 3.0.3
info: {title: t, version: "1"}
paths: {}
components:
  schemas:
    Sel:
      type: object
      properties:
        all: {$ref: '#/components/schemas/Sel.All'}
        list: {allOf: [{$ref: '#/components/schemas/Sel.List'}]}
    Sel.All: {type: object, additionalProperties: false}
    Sel.List:
      type: object
      properties:
        labels: {type: array, items: {type: string}}
        values: {type: array, items: {type: string}}
        newText: {type: string}
        newCount: {type: integer, format: int32}
        newThing: {$ref: '#/components/schemas/Sel.Thing'}
        described: {type: boolean}
    Sel.Thing: {type: object, properties: {x: {type: string}}}
`

// The list arm of a typedArms.asList oneOf is shown as its one list of strings. Every other field
// of the arm that has no line is skipped, so a field that the API adds does not stop generation.
// A field with a line keeps it.
func TestSkipListArmFieldsSkipsWhatTheShownListDoesNotHold(t *testing.T) {
	doc, err := model.Load([]byte(asListSpec))
	if err != nil {
		t.Fatal(err)
	}
	file := &overrides.File{Types: map[string]overrides.Type{
		"Sel":      {TypedArms: &overrides.TypedArms{AsList: true}},
		"Sel.List": {Fields: map[string]overrides.Field{"labels": {Field: iacoverrides.Field{Skip: true}}, "described": {Required: true}}},
	}}
	skipListArmFields(doc, file)
	fields := file.Types["Sel.List"].Fields
	for _, name := range []string{"newText", "newCount", "newThing", "labels"} {
		if !fields[name].Skip {
			t.Errorf("%s is not skipped", name)
		}
	}
	if fields["values"].Skip {
		t.Error("the shown list must not be skipped")
	}
	if fields["described"].Skip || !fields["described"].Required {
		t.Error("a field with a line keeps its line")
	}
	skipListArmFields(doc, file) // a second run changes nothing
	if len(file.Types["Sel.List"].Fields) != 5 {
		t.Errorf("fields = %d, want 5", len(file.Types["Sel.List"].Fields))
	}
}

func TestSkipListArmFieldsLeavesAOneOfWithoutOneListArm(t *testing.T) {
	doc, err := model.Load([]byte(asListSpec))
	if err != nil {
		t.Fatal(err)
	}
	file := &overrides.File{Types: map[string]overrides.Type{"Sel": {TypedArms: &overrides.TypedArms{AsList: true}}, "Sel.List": {Fields: map[string]overrides.Field{"values": {Field: iacoverrides.Field{Skip: true}}, "labels": {Field: iacoverrides.Field{Skip: true}}}}}}
	skipListArmFields(doc, file)
	if len(file.Types["Sel.List"].Fields) != 2 {
		t.Error("an arm whose list is skipped has no shown list: no field is skipped for it")
	}
}
