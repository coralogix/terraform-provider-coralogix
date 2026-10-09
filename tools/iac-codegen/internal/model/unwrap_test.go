package model

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// wrapPolicy is the rule set of testdata/wrapthing.yaml, as its behavior-overrides file states it.
func wrapPolicy() Policy {
	return Policy{
		Existing:             true,
		NoInferredValidators: true,
		Unwrap:               []string{"LuceneQuery", "PriorityValue", "Threshold", "UUID", "FilterHolder", "Selection", "ListSelection"},
		UnwrapFields:         map[string]bool{"WrapThing.rawQuery": false, "WrapThing.mainLabel": true},
	}
}

// wrapSpec returns testdata/wrapthing.yaml with each pair of old and new text replaced.
func wrapSpec(t *testing.T, replacements ...string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "wrapthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	spec := string(data)
	for i := 0; i < len(replacements); i += 2 {
		if !strings.Contains(spec, replacements[i]) {
			t.Fatalf("the spec has no %q", replacements[i])
		}
		spec = strings.ReplaceAll(spec, replacements[i], replacements[i+1])
	}
	return []byte(spec)
}

func wrapCodes(t *testing.T, spec []byte, p Policy) []string {
	t.Helper()
	doc, err := Load(spec)
	if err != nil {
		t.Fatal(err)
	}
	return slices.Compact(reportCodes(ValidateWithPolicy(doc, "WrapThing", OperationIDs{}, p)))
}

func buildWrapThing(t *testing.T, p Policy) *Resource {
	t.Helper()
	doc, err := Load(wrapSpec(t))
	if err != nil {
		t.Fatal(err)
	}
	r, err := BuildWithPolicy(doc, "WrapThing", OperationIDs{}, p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func wrapField(t *testing.T, r *Resource, name string) *ResourceField {
	t.Helper()
	for _, f := range r.Fields {
		if f.Name == name {
			return f
		}
	}
	t.Fatalf("no field %s", name)
	return nil
}

// Each collapsed field has the type of the value inside its wrappers, and records the wrappers
// outer first. A field that keeps its object is unchanged.
func TestUnwrapReplacesTheTypeWithTheValue(t *testing.T) {
	r := buildWrapThing(t, wrapPolicy())
	tests := []struct {
		field, kind, wrappers, path string
		elem                        bool
	}{
		{field: "query", kind: "string", wrappers: "LuceneQuery", path: "value"},
		{field: "priority", kind: "enum", wrappers: "PriorityValue", path: "value"},
		{field: "threshold", kind: "number", wrappers: "Threshold", path: "value"},
		{field: "filter", kind: "object", wrappers: "FilterHolder", path: "filter"},
		{field: "selection", kind: "list", wrappers: "Selection,ListSelection", path: "list.values"},
		{field: "mainLabel", kind: "string", wrappers: "Label", path: "value"},
		{field: "widgetIds", kind: "string", wrappers: "UUID", path: "value", elem: true},
		{field: "rawQuery", kind: "object"},
		{field: "extraLabel", kind: "object"},
	}
	for _, test := range tests {
		t.Run(test.field, func(t *testing.T) {
			typ := wrapField(t, r, test.field).Type
			if test.elem {
				typ = typ.Elem
			}
			var names []string
			for _, w := range typ.Wrappers {
				names = append(names, w.Schema)
			}
			if string(typ.Kind) != test.kind || strings.Join(names, ",") != test.wrappers || typ.WrapperPath() != test.path {
				t.Fatalf("kind %s, wrappers %v, path %q; want %s, %s, %q", typ.Kind, names, typ.WrapperPath(), test.kind, test.wrappers, test.path)
			}
		})
	}
	used := []string{"unwrap.LuceneQuery", "unwrap.PriorityValue", "unwrap.Threshold", "unwrap.UUID", "unwrap.FilterHolder",
		"unwrap.Selection", "unwrap.ListSelection", "WrapThing.rawQuery", "WrapThing.mainLabel"}
	for _, key := range used {
		if !r.UnwrapUsed[key] {
			t.Errorf("line %s is not used", key)
		}
	}
}

// A request type names the request components of the wrappers.
func TestUnwrapRequestTypeKeepsTheWrappers(t *testing.T) {
	r := buildWrapThing(t, wrapPolicy())
	create := wrapField(t, r, "selection").Type.CreateType()
	if len(create.Wrappers) != 2 || create.Wrappers[0].Schema != "Selection" || create.Wrappers[1].Schema != "ListSelection" {
		t.Fatalf("create wrappers = %+v", create.Wrappers)
	}
}

// A wrapper that collapses states the presence of its property, because Terraform null sends no
// wrapper. The same property in a kept object still needs presence.
func TestUnwrapStatesThePresenceOfTheWrappedProperty(t *testing.T) {
	if codes := wrapCodes(t, wrapSpec(t), wrapPolicy()); len(codes) != 0 {
		t.Fatalf("codes %v, want none", codes)
	}
	without := Policy{Existing: true, NoInferredValidators: true}
	if codes := wrapCodes(t, wrapSpec(t), without); !slices.Contains(codes, "FIELD_PRESENCE_UNKNOWN") {
		t.Fatalf("codes %v, want FIELD_PRESENCE_UNKNOWN without the unwrap lines", codes)
	}
	kept := wrapPolicy()
	kept.UnwrapFields = map[string]bool{"WrapThing.rawQuery": false, "WrapThing.mainLabel": true, "WrapThing.priority": false}
	if codes := wrapCodes(t, wrapSpec(t), kept); !slices.Contains(codes, "FIELD_PRESENCE_UNKNOWN") {
		t.Fatalf("codes %v, want FIELD_PRESENCE_UNKNOWN for the kept priority object", codes)
	}
}

// The response components decide which wrappers collapse, also when a request names its
// components differently.
func TestUnwrapPresenceFollowsTheResponseComponents(t *testing.T) {
	spec := wrapSpec(t,
		"                priority: {$ref: '#/components/schemas/PriorityValue'}",
		"                priority: {$ref: '#/components/schemas/RequestPriorityValue'}",
		"                  items: {$ref: '#/components/schemas/UUID'}",
		"                  items: {$ref: '#/components/schemas/RequestUUID'}",
		"    Label:\n",
		"    RequestPriorityValue:\n      type: object\n      required: []\n      properties:\n        value: {$ref: '#/components/schemas/WrapPriority'}\n"+
			"    RequestUUID:\n      type: object\n      required: []\n      properties:\n        value: {type: string}\n    Label:\n")
	if codes := wrapCodes(t, spec, wrapPolicy()); len(codes) != 0 {
		t.Fatalf("codes %v, want none", codes)
	}
	kept := wrapPolicy()
	kept.Unwrap = slices.DeleteFunc(slices.Clone(kept.Unwrap), func(c string) bool { return c == "PriorityValue" })
	if codes := wrapCodes(t, spec, kept); !slices.Contains(codes, "FIELD_PRESENCE_UNKNOWN") {
		t.Fatalf("codes %v, want FIELD_PRESENCE_UNKNOWN for the kept priority object", codes)
	}
}

func TestUnwrapRejectsWhatIsNotAWrapper(t *testing.T) {
	twoProperties := wrapSpec(t, "        filter: {$ref: '#/components/schemas/SimpleFilter'}\n",
		"        filter: {$ref: '#/components/schemas/SimpleFilter'}\n        note: {type: string, x-coralogix-presence: true}\n")
	innerDefault := wrapSpec(t, "        value: {type: number, format: double}", "        value: {type: number, format: double, default: 1}")
	mapOfWrappers := wrapSpec(t, "                widgetIds:\n                  type: array\n                  items: {$ref: '#/components/schemas/UUID'}",
		"                widgetIds:\n                  type: object\n                  additionalProperties: {$ref: '#/components/schemas/UUID'}",
		"        widgetIds:\n          description: The widgets, each held in a wrapper.\n          type: array\n          items: {$ref: '#/components/schemas/UUID'}",
		"        widgetIds:\n          type: object\n          additionalProperties: {$ref: '#/components/schemas/UUID'}")
	stringLine := wrapPolicy()
	stringLine.UnwrapFields = map[string]bool{"WrapThing.rawQuery": false, "WrapThing.mainLabel": true, "WrapThing.name": true}
	tests := map[string]struct {
		spec   []byte
		policy Policy
		want   string
	}{
		"two properties": {twoProperties, wrapPolicy(), "UNWRAP_COMPONENT_UNSUPPORTED"},
		"inner default":  {innerDefault, wrapPolicy(), "UNWRAP_COMPONENT_UNSUPPORTED"},
		"map of values":  {mapOfWrappers, wrapPolicy(), "UNWRAP_COMPONENT_UNSUPPORTED"},
		"string field":   {wrapSpec(t), stringLine, "UNWRAP_FIELD_UNSUPPORTED"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if codes := wrapCodes(t, test.spec, test.policy); !slices.Contains(codes, test.want) {
				t.Fatalf("codes %v do not contain %s", codes, test.want)
			}
		})
	}
}

// A new resource has no unwrap lines, so a wrapper stays an object.
func TestUnwrapIsOnlyForExistingResources(t *testing.T) {
	p := wrapPolicy()
	p.Existing = false
	if typ := wrapField(t, buildWrapThing(t, p), "query").Type; typ.Kind != Object || len(typ.Wrappers) != 0 {
		t.Fatalf("query = %s with %d wrappers, want the object", typ.Kind, len(typ.Wrappers))
	}
}
