package model

import (
	"testing"
)

// A request scalar without presence or a default gets its zero value as the default in Create,
// Update, and Get when that zero value is valid. Otherwise a read of zero is an omitted value.
// An enum, a field with presence, and a field with a declared default keep their reading.
func TestZeroDefaults(t *testing.T) {
	r := buildThing(t, Policy{})
	for name, want := range map[string][3]string{
		"paused":      {"false", "false", "false"},
		"retries":     {"0", "0", "0"},
		"enabled":     {"true", "true", "<nil>"}, // declared in the requests; Get requires it
		"description": {"<nil>", "<nil>", "<nil>"},
		"priority":    {"<nil>", "<nil>", "<nil>"},
		"limit":       {"<nil>", "<nil>", "<nil>"},
		"label":       {"<nil>", "<nil>", "<nil>"},
	} {
		f := thingField(t, r, name)
		if got := [3]string{defaultText(f.Create.Default), defaultText(f.Update.Default), defaultText(f.Get.Default)}; got != want {
			t.Errorf("%s: Create, Update, Get defaults = %v, want %v", name, got, want)
		}
		if zeroNull := name == "limit" || name == "label"; f.Type.ReadZeroAsNull != zeroNull {
			t.Errorf("%s: ReadZeroAsNull = %t, want %t", name, f.Type.ReadZeroAsNull, zeroNull)
		}
	}
	weight := fieldNamed(thingField(t, r, "spec").Type.Fields, "weight")
	if weight == nil || defaultText(weight.Create.Default) != "0" || defaultText(weight.Update.Default) != "0" || defaultText(weight.Attrs.Default) != "0" {
		t.Fatalf("spec.weight = %+v, want the default 0 in Create, Update, and Get", weight)
	}
}

// A field with a released line in the behavior-overrides file keeps its released reading.
func TestZeroDefaultsKeepReleasedFields(t *testing.T) {
	r := buildThing(t, Policy{Existing: true, Released: []string{"Thing.paused", "Thing.limit"}})
	if f := thingField(t, r, "paused"); f.Create.Default != nil || f.Get.Default != nil {
		t.Errorf("released paused has a default: %v", defaultText(f.Create.Default))
	}
	if thingField(t, r, "limit").Type.ReadZeroAsNull {
		t.Error("released limit reads zero as null")
	}
}

// buildThing builds Thing from the generator fixture under the policy p.
func buildThing(t *testing.T, p Policy) *Resource {
	t.Helper()
	doc, err := Load(validSpec(t))
	if err != nil {
		t.Fatal(err)
	}
	r, err := BuildWithPolicy(doc, "Thing", OperationIDs{}, p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func thingField(t *testing.T, r *Resource, name string) *ResourceField {
	t.Helper()
	for _, f := range r.Fields {
		if f.Name == name {
			return f
		}
	}
	t.Fatalf("no field %s", name)
	return nil
}

func defaultText(d *string) string {
	if d == nil {
		return "<nil>"
	}
	return *d
}

// scalarZero matches settable_zero of the API lint (cx-management-apis), including exclusive
// bounds in the OpenAPI 3.0 and 3.1 forms, and a 64-bit number that the API writes as a digit
// string without a format. Only the exact generated digit patterns make "0" the zero: another
// pattern that accepts "0" keeps "" as the zero of a protobuf string.
func TestScalarZero(t *testing.T) {
	tests := map[string]struct {
		version, schema string
		zero            string
		ok              bool
	}{
		"bool":                             {"3.0.3", "{type: boolean}", "false", true},
		"int":                              {"3.0.3", "{type: integer, format: int32}", "0", true},
		"int minimum 1":                    {"3.0.3", "{type: integer, format: int32, minimum: 1}", "0", false},
		"int maximum -1":                   {"3.0.3", "{type: integer, format: int32, maximum: -1}", "0", false},
		"3.1 exclusiveMinimum 0":           {"3.1.0", "{type: integer, format: int32, exclusiveMinimum: 0}", "0", false},
		"3.1 exclusiveMinimum -1":          {"3.1.0", "{type: integer, format: int32, exclusiveMinimum: -1}", "0", true},
		"3.1 exclusiveMaximum 0":           {"3.1.0", "{type: number, format: double, exclusiveMaximum: 0}", "0", false},
		"3.1 exclusiveMaximum 1":           {"3.1.0", "{type: number, format: double, exclusiveMaximum: 1}", "0", true},
		"3.0 exclusiveMinimum, minimum 0":  {"3.0.3", "{type: integer, format: int32, minimum: 0, exclusiveMinimum: true}", "0", false},
		"3.0 exclusiveMaximum, maximum 0":  {"3.0.3", "{type: integer, format: int32, maximum: 0, exclusiveMaximum: true}", "0", false},
		"3.0 exclusiveMinimum, minimum -1": {"3.0.3", "{type: integer, format: int32, minimum: -1, exclusiveMinimum: true}", "0", true},
		"string":                           {"3.0.3", "{type: string}", "", true},
		"string minLength 1":               {"3.0.3", "{type: string, minLength: 1}", "", false},
		"int64 string":                     {"3.0.3", "{type: string, format: int64}", "0", true},
		"digit string":                     {"3.0.3", "{type: string, minLength: 1, maxLength: 20, pattern: '^[0-9]+$'}", "0", true},
		"signed digit string":              {"3.0.3", "{type: string, pattern: '^-?[0-9]+$'}", "0", true},
		"non-empty string":                 {"3.0.3", `{type: string, minLength: 1, pattern: '^[\s\S]+$'}`, "", false},
		"id pattern":                       {"3.0.3", "{type: string, pattern: '^[a-zA-Z0-9][a-zA-Z0-9_-]*$'}", "", false},
		"upper case and digits":            {"3.0.3", "{type: string, minLength: 1, pattern: '^[A-Z0-9]+$'}", "", false},
		"decimal string":                   {"3.0.3", "{type: string, pattern: '^[+-]?[0-9]+$'}", "", false},
		"digit string from 1":              {"3.0.3", "{type: string, pattern: '^[1-9][0-9]*$'}", "", false},
		"pattern that accepts both":        {"3.0.3", "{type: string, pattern: '^[0-9]*$'}", "", true},
		"date-time":                        {"3.0.3", "{type: string, format: date-time}", "", false},
		"enum":                             {"3.0.3", "{type: string, enum: [LEVEL_UNSPECIFIED, LEVEL_HIGH]}", "", false},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			doc, err := Load([]byte("openapi: " + test.version + "\ninfo: {title: test, version: 1.0.0}\npaths: {}\ncomponents:\n  schemas:\n    Value: " + test.schema + "\n"))
			if err != nil {
				t.Fatal(err)
			}
			schema, err := schemaOf(doc.Components.Schemas.GetOrZero("Value"))
			if err != nil {
				t.Fatal(err)
			}
			if zero, ok := scalarZero(schema); zero != test.zero || ok != test.ok {
				t.Fatalf("scalarZero = %q, %t; want %q, %t", zero, ok, test.zero, test.ok)
			}
		})
	}
}

// A digit string without presence or a default gets "0" as its default, the zero value that the
// API returns for a 64-bit number.
func TestZeroDefaultOfADigitString(t *testing.T) {
	typ := &Type{Kind: String, Zero: "0", ZeroValid: true}
	create, update, get := &Attrs{}, &Attrs{}, &Attrs{}
	zeroDefault(typ, create, update, get)
	for _, a := range []*Attrs{create, update, get} {
		if a.Default == nil || *a.Default != "0" {
			t.Fatalf("default = %v, want \"0\"", a.Default)
		}
	}
}
