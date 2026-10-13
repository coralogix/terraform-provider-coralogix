package model

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func gadgetSpec(t *testing.T, replacements ...string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "gadget.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	spec := string(data)
	for i := 0; i < len(replacements); i += 2 {
		if !strings.Contains(spec, replacements[i]) {
			t.Fatalf("the spec has no %q", replacements[i])
		}
		spec = strings.Replace(spec, replacements[i], replacements[i+1], 1)
	}
	return []byte(spec)
}

func gadgetCodes(t *testing.T, spec []byte) []string {
	t.Helper()
	doc, err := Load(spec)
	if err != nil {
		t.Fatal(err)
	}
	return slices.Compact(reportCodes(ValidateWithPolicy(doc, "Gadget", OperationIDs{}, Policy{})))
}

func TestGadgetServerDefaults(t *testing.T) {
	if codes := gadgetCodes(t, gadgetSpec(t)); len(codes) != 0 {
		t.Fatalf("codes = %v, want none", codes)
	}
	doc, err := Load(gadgetSpec(t))
	if err != nil {
		t.Fatal(err)
	}
	r, err := Build(doc, "Gadget")
	if err != nil {
		t.Fatal(err)
	}
	field := func(name string) *ResourceField {
		i := slices.IndexFunc(r.Fields, func(f *ResourceField) bool { return f.Name == name })
		if i < 0 {
			t.Fatalf("no field %s", name)
		}
		return r.Fields[i]
	}
	// A default next to an enum $ref keeps the enum type and reads the default.
	mode := field("mode")
	if mode.Description != "Gadget mode." {
		t.Errorf("mode description = %q, want the description of the GadgetMode component", mode.Description)
	}
	if mode.Type.Kind != Enum || mode.Type.Schema != "GadgetMode" || mode.Create.Default == nil || *mode.Create.Default != "GADGET_MODE_BASIC" {
		t.Fatalf("mode = %+v create %+v, want the GadgetMode enum with its default", mode.Type, mode.Create)
	}
	// Only replace sets compact: it is managed, and Create does not send it.
	compact := field("compact")
	if compact.Behavior != Normal || compact.Create != nil || compact.Update == nil {
		t.Fatalf("compact: behavior %q create %v update %v, want normal and only in Update", compact.Behavior, compact.Create, compact.Update)
	}
	syntax := fieldNamed(field("search").Type.Fields, "syntax")
	if syntax.Behavior != Normal || syntax.Create == nil || syntax.Create.Default == nil || *syntax.Create.Default != "GADGET_SYNTAX_PLAIN" {
		t.Fatalf("search.syntax: behavior %q create %+v, want normal with its default", syntax.Behavior, syntax.Create)
	}
}

// Omitting an Update field resets it to its server default. That needs no presence marker only
// when a configured value cannot look like omission: false next to a default of true would.
func TestServerDefaultUpdatePresence(t *testing.T) {
	createActive := "                name: {type: string, minLength: 1, x-coralogix-presence: true}\n                mode:\n"
	tests := map[string]struct {
		def  string
		want bool
	}{
		"default false": {"false", false},
		"default true":  {"true", true},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			active := "                active: {type: boolean, default: " + test.def
			spec := gadgetSpec(t,
				createActive, strings.Replace(createActive, "\n                mode:\n", "\n"+active+", x-coralogix-presence: true}\n                mode:\n", 1),
				"                mode: {$ref: '#/components/schemas/GadgetMode'}\n", "                mode: {$ref: '#/components/schemas/GadgetMode'}\n"+active+"}\n",
				"\n        mode: {$ref: '#/components/schemas/GadgetMode'}\n", "\n        mode: {$ref: '#/components/schemas/GadgetMode'}\n        active: {type: boolean, default: "+test.def+", x-coralogix-presence: true}\n",
			)
			doc, err := Load(spec)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, item := range ValidateWithPolicy(doc, "Gadget", OperationIDs{}, Policy{}) {
				if item.Location == "components.schemas.Gadget.active.update" && item.Code == "FIELD_PRESENCE_UNKNOWN" {
					found = true
				}
			}
			if found != test.want {
				t.Fatalf("FIELD_PRESENCE_UNKNOWN at active.update = %t, want %t", found, test.want)
			}
		})
	}
}

func TestGadgetServerDefaultContract(t *testing.T) {
	tests := map[string]struct {
		replacements []string
		code         string
	}{
		"replace-only field without a default": {
			[]string{"compact: {type: boolean, default: false, x-coralogix-presence: true}\n                search", "compact: {type: boolean, x-coralogix-presence: true}\n                search"},
			"FIELD_LIFECYCLE_UNSUPPORTED",
		},
		"response declares another default": {
			[]string{"        mode: {$ref: '#/components/schemas/GadgetMode'}\n        compact", "        mode:\n          $ref: '#/components/schemas/GadgetMode'\n          default: GADGET_MODE_EXPERT\n        compact"},
			"FIELD_DEFAULT_CONTRACT_INCONSISTENT",
		},
		"replace declares another default": {
			[]string{"                mode: {$ref: '#/components/schemas/GadgetMode'}\n", "                mode:\n                  $ref: '#/components/schemas/GadgetMode'\n                  default: GADGET_MODE_EXPERT\n"},
			"FIELD_DEFAULT_CONTRACT_INCONSISTENT",
		},
		"required nested field with a default": {
			[]string{"required: [text]", "required: [text, syntax]"},
			"NESTED_FIELD_DEFAULT_UNSUPPORTED",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if codes := gadgetCodes(t, gadgetSpec(t, test.replacements...)); !slices.Contains(codes, test.code) {
				t.Fatalf("codes = %v, want %s", codes, test.code)
			}
		})
	}
}

// The type of a $ref with siblings comes from the target, so a sibling constraint would be
// dropped. It is an error instead.
func TestGadgetRefSiblingConstraintIsAnError(t *testing.T) {
	doc, err := Load(gadgetSpec(t, "                  default: GADGET_MODE_BASIC\n", "                  default: GADGET_MODE_BASIC\n                  pattern: '^GADGET_MODE_B'\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Build(doc, "Gadget"); err == nil || !strings.Contains(err.Error(), "the sibling pattern") {
		t.Fatalf("err = %v, want the sibling pattern error", err)
	}
	if codes := slices.Compact(reportCodes(ValidateWithPolicy(doc, "Gadget", OperationIDs{}, Policy{}))); len(codes) == 0 {
		t.Fatal("validation passed, want an issue for the sibling pattern")
	}
}

// A keyword outside the allowlist is an error even when the generator has no other check for it.
func TestGadgetUnknownRefSiblingIsAnError(t *testing.T) {
	for _, sibling := range []string{"prefixItems: [{type: string}]", "patternProperties: {'^a': {type: string}}"} {
		doc, err := Load(gadgetSpec(t, "                  default: GADGET_MODE_BASIC\n", "                  default: GADGET_MODE_BASIC\n                  "+sibling+"\n"))
		if err != nil {
			t.Fatal(err)
		}
		keyword := strings.SplitN(sibling, ":", 2)[0]
		if _, err := Build(doc, "Gadget"); err == nil || !strings.Contains(err.Error(), "the sibling "+keyword) {
			t.Fatalf("%s: err = %v, want the sibling error", keyword, err)
		}
	}
	// Descriptive siblings and extensions keep working.
	doc, err := Load(gadgetSpec(t, "                  default: GADGET_MODE_BASIC\n", "                  default: GADGET_MODE_BASIC\n                  description: The mode.\n                  x-note: kept\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Build(doc, "Gadget"); err != nil {
		t.Fatal(err)
	}
}

// x-coralogix-presence describes the field, so it can sit next to a $ref. Another Coralogix
// extension, such as x-coralogix-collection, describes the type, which comes from the target.
func TestGadgetCoralogixRefSiblings(t *testing.T) {
	for sibling, wantErr := range map[string]bool{"x-coralogix-presence: true": false, "x-coralogix-collection: set": true} {
		doc, err := Load(gadgetSpec(t, "                  default: GADGET_MODE_BASIC\n", "                  default: GADGET_MODE_BASIC\n                  "+sibling+"\n"))
		if err != nil {
			t.Fatal(err)
		}
		_, err = Build(doc, "Gadget")
		if got := err != nil && strings.Contains(err.Error(), "the sibling x-coralogix-"); got != wantErr {
			t.Errorf("%s: err = %v, want the sibling error %t", sibling, err, wantErr)
		}
	}
}

// A default next to a $ref is checked against the type and the limits of the target.
func TestGadgetRefSiblingDefaultMustFitTheTarget(t *testing.T) {
	codes := gadgetCodes(t, gadgetSpec(t, "                  default: GADGET_MODE_BASIC\n", "                  default: GADGET_MODE_MISSING\n"))
	if !slices.Contains(codes, "FIELD_DEFAULT_INVALID") {
		t.Fatalf("codes = %v, want FIELD_DEFAULT_INVALID", codes)
	}
}
