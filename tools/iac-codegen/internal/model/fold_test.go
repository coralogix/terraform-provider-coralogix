package model

import (
	"slices"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/orderedmap"
)

// createBody is a Create body with integer properties, of which required are required.
func createBody(names []string, required ...string) *base.Schema {
	props := orderedmap.New[string, *base.SchemaProxy]()
	for _, n := range names {
		props.Set(n, base.CreateSchemaProxy(&base.Schema{Type: []string{"integer"}, Format: "int32"}))
	}
	return &base.Schema{Type: []string{"object"}, Properties: props, Required: required}
}

// A required flat field makes the object that holds it required: Terraform does not check the
// required attributes of an object that the configuration leaves out, so Create would send zero.
func TestFoldedObjectOfARequiredFieldIsRequired(t *testing.T) {
	p := Policy{CreateAs: map[string]map[string]string{"Thing.quota": {"limit": "limitValue"}}}
	out, err := p.foldCreateBody("Thing", createBody([]string{"name", "limitValue"}, "limitValue"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(out.Required, "quota") || slices.Contains(out.Required, "limitValue") {
		t.Fatalf("required = %v, want quota and not limitValue", out.Required)
	}

	out, err = p.foldCreateBody("Thing", createBody([]string{"name", "limitValue"}))
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(out.Required, "quota") {
		t.Fatalf("required = %v, want quota optional when Create does not require limitValue", out.Required)
	}
}

// Two createAs lines cannot send the same flat field: one of the values would be lost.
func TestTwoFoldsOfOneFlatFieldAreRefused(t *testing.T) {
	p := Policy{CreateAs: map[string]map[string]string{
		"Thing.quota": {"limit": "limitValue"},
		"Thing.cap":   {"max": "limitValue"},
	}}
	_, err := p.foldCreateBody("Thing", createBody([]string{"limitValue"}))
	if err == nil || !strings.Contains(err.Error(), `already sends "limitValue"`) {
		t.Fatalf("err = %v, want an error about a flat field that two lines send", err)
	}
}
