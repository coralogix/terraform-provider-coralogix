package generator

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
)

// validateOverwriteThing runs the pre-SDK check on the overwritething contract and overrides after
// edit changes them.
func validateOverwriteThing(t *testing.T, edit func(spec, overrides string) (string, string)) (*validatedResource, error) {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "overwritething.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	over, err := os.ReadFile(filepath.Join("testdata", "overwritething-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s, o := edit(string(spec), string(over))
	return validateOpenAPIWith([]byte(s), "ThingList", model.OperationIDs{}, "sdk", "provider", mustParse(t, o))
}

// Each shape that the generated code cannot read or send faithfully is refused.
func TestOverwriteThingRefusesShapes(t *testing.T) {
	tests := map[string]struct {
		edit func(t *testing.T, spec, overrides string) (string, string)
		want string
	}{
		"negate on a field of the resource component": {
			edit: func(t *testing.T, spec, overrides string) (string, string) {
				return spec, replaceOnce(t, overrides, "      things: {required: true}\n", "      things: {required: true, negate: true}\n")
			},
			want: "negate",
		},
		"zeroNull on an enum that a list holds": {
			edit: func(t *testing.T, spec, overrides string) (string, string) {
				return replaceOnce(t, spec, "        priority: {$ref: '#/components/schemas/ThingPriority'}\n        aRules:",
					"        priority: {$ref: '#/components/schemas/ThingPriority'}\n        priorities: {type: array, readOnly: true, items: {$ref: '#/components/schemas/ThingPriority'}}\n        aRules:"), overrides
			},
			want: "zeroNull",
		},
		"a required scalar of an inlined optional object": {
			edit: func(t *testing.T, spec, overrides string) (string, string) {
				spec = replaceOnce(t, spec, "    Rules:\n      type: object\n", "    Rules:\n      type: object\n      required: [values]\n")
				return replaceOnce(t, spec, "values: {type: string, description:", "values: {type: string, minLength: 1, description:"), overrides
			},
			want: "zero value instead of null",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := validateOverwriteThing(t, func(spec, overrides string) (string, string) { return test.edit(t, spec, overrides) })
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want one that says %q", err, test.want)
			}
		})
	}
}

// An inline line on Thing.aRules inlines aRules in Thing and in the request item, its request
// counterpart. Another component of the response with a field of the same name and object, Other,
// keeps it.
func TestNestedInlineStaysOnItsComponent(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "overwritething.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	over, err := os.ReadFile(filepath.Join("testdata", "overwritething-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	// Other, which Thing holds, has its own aRules field of the same object.
	edited := replaceOnce(t, string(spec), "        bRules: {$ref: '#/components/schemas/Rules'}\n",
		"        bRules: {$ref: '#/components/schemas/Rules'}\n        other: {$ref: '#/components/schemas/Other', readOnly: true}\n")
	edited = replaceOnce(t, edited, "    Rules:\n", "    Other:\n      type: object\n      properties:\n        aRules: {$ref: '#/components/schemas/Rules'}\n    Rules:\n")
	file := mustParse(t, string(over))
	if _, _, report := reshape([]byte(edited), "ThingList", model.OperationIDs{}, file, file.Policy()); len(report) != 0 {
		t.Fatal(report)
	}
	var got []string
	for component := range file.Inlined.Nested {
		got = append(got, component)
	}
	slices.Sort(got)
	if !slices.Equal(got, []string{"KindAThingWrite", "Thing"}) {
		t.Fatalf("inlined in %v, want Thing and its request item KindAThingWrite only, not Other", got)
	}
}

// Expand clears the optional request objects that it set no field in, the deepest first.
func TestDropEmptyParentsDeepestFirst(t *testing.T) {
	o := &convObject{Parents: []convParent{
		{SDK: "Policy", DropEmpty: true},
		{SDK: "Inlined"},
		{SDK: "Policy.Details", DropEmpty: true},
	}}
	var got []string
	for _, p := range o.DropEmptyParents() {
		got = append(got, p.SDK)
	}
	if !slices.Equal(got, []string{"Policy.Details", "Policy"}) {
		t.Fatalf("drop order = %v, want Policy.Details then Policy, without the top-level inlined object", got)
	}
}
