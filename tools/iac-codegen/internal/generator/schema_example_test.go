package generator

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/acceptance"
)

func strExample(texts ...string) *contractExample {
	e := &contractExample{}
	for _, text := range texts {
		e.Items = append(e.Items, exampleItem{Values: []exampleValue{{Text: text, Tag: "!!str"}}})
	}
	return e
}

func listExample(elements ...string) *contractExample {
	item := exampleItem{Array: true}
	for _, text := range elements {
		item.Values = append(item.Values, exampleValue{Text: text, Tag: "!!str"})
	}
	return &contractExample{Items: []exampleItem{item}}
}

func exampleResource(attrs ...*tfAttr) *tfResource {
	return &tfResource{Package: "p", CRUD: &crudData{TypeName: "thing", IDAttr: "id", Resource: "Thing"}, Attributes: attrs}
}

func exampleAcceptance(t *testing.T, res *tfResource) *acceptanceData {
	t.Helper()
	data, err := buildAcceptance(res, "example.com/provider", &acceptance.File{Resource: "Thing"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// The made-up value of a string, an int64 that the JSON carries as a string, and the elements of a
// list come from the example. The full and the minimal config use the first example, and the
// updated config uses the second one, or the first when there is only one.
func TestAcceptanceUsesExamples(t *testing.T) {
	res := exampleResource(
		&tfAttr{Name: "name", Kind: "String", Required: true},
		&tfAttr{Name: "field", Kind: "String", Required: true, Example: strExample("PLATFORM")},
		&tfAttr{Name: "mode", Kind: "String", Optional: true, Example: strExample("fast", "slow")},
		&tfAttr{Name: "from_ms", Kind: "Int64", Required: true, Uint64: true, Example: strExample("1763251200000")},
		&tfAttr{Name: "names", Kind: "List", ElementType: "types.StringType", Optional: true, Example: listExample("a", "b")},
		&tfAttr{Name: "ids", Kind: "Set", ElementType: "types.StringType", Optional: true, Example: listExample("x", "x", "y")},
		&tfAttr{Name: "note", Kind: "String", Optional: true, Example: strExample("a${b}")},
	)
	data := exampleAcceptance(t, res)
	for _, want := range []string{`field = "PLATFORM"`, `mode = "fast"`, `from_ms = 1763251200000`, `names = ["a", "b"]`, `ids = ["x", "y"]`, `note = "a$${b}"`, `name = "@{run}-name"`} {
		if !strings.Contains(data.Full.Config, want) {
			t.Errorf("full config:\n%s\nwant %s", data.Full.Config, want)
		}
	}
	for _, want := range []string{`field = "PLATFORM"`, `mode = "slow"`, `from_ms = 1763251200000`, `names = ["a", "b"]`, `name = "@{run}-name-updated"`} {
		if !strings.Contains(data.Updated.Config, want) {
			t.Errorf("updated config:\n%s\nwant %s", data.Updated.Config, want)
		}
	}
	wantChecks := map[string]string{"field": "PLATFORM", "from_ms": "1763251200000", "names.#": "2", "names.1": "b", "ids.#": "2", "note": "a${b}"}
	for _, c := range data.Full.Checks {
		if want, ok := wantChecks[c.Path]; ok {
			if c.Value != want {
				t.Errorf("check %s = %q, want %q", c.Path, c.Value, want)
			}
			delete(wantChecks, c.Path)
		}
	}
	if len(wantChecks) != 0 {
		t.Errorf("the full config has no check for %v", wantChecks)
	}
}

// An example never replaces an enum value, and a time keeps the made-up value.
func TestAcceptanceExampleDoesNotReplaceEnumOrTime(t *testing.T) {
	res := exampleResource(
		&tfAttr{Name: "kind", Kind: "String", Required: true, Validators: []string{`stringvalidator.OneOf("alpha", "beta")`}, Example: strExample("beta")},
		&tfAttr{Name: "at", Kind: "String", Required: true, CustomType: rfc3339Type, Example: strExample("2020-01-01T00:00:00+02:00")},
		&tfAttr{Name: "kinds", Kind: "List", ElementType: "types.StringType", Optional: true, Validators: []string{`listvalidator.ValueStringsAre(stringvalidator.OneOf("alpha", "beta"))`}, ElemValidators: []string{`stringvalidator.OneOf("alpha", "beta")`}, Example: listExample("beta")},
	)
	data := exampleAcceptance(t, res)
	for _, want := range []string{`kind = "alpha"`, `at = "2030-01-01T00:00:00Z"`, `kinds = ["alpha"]`} {
		if !strings.Contains(data.Full.Config, want) {
			t.Errorf("full config:\n%s\nwant %s", data.Full.Config, want)
		}
	}
}

// A copy of a step of the sweep numbers a made-up string. It reuses the example as it is.
func TestSweepCopiesReuseExamples(t *testing.T) {
	s := testSynth(t, "")
	s.sweeping, s.element, s.cov, s.perPath = true, 2, map[string]bool{}, true
	made := &tfAttr{Name: "label", Kind: "String", Optional: true}
	example := &tfAttr{Name: "field", Kind: "String", Optional: true, Example: strExample("PLATFORM")}
	madeHCL, _, err := s.makeValue(made, "label", "label", accFull)
	if err != nil {
		t.Fatal(err)
	}
	exampleHCL, checks, err := s.makeValue(example, "field", "field", accFull)
	if err != nil {
		t.Fatal(err)
	}
	if madeHCL != `"@{run}-label-3"` {
		t.Errorf("made-up value = %s, want it numbered", madeHCL)
	}
	if exampleHCL != `"PLATFORM"` || checks[0].Value != "PLATFORM" {
		t.Errorf("example value = %s, checks %v, want PLATFORM unchanged", exampleHCL, checks)
	}
}

// An update with examples changes the attributes that have none. When every attribute that the
// updated config would change has an example, the updated config makes up its values, as it does
// without examples.
func TestUpdatedConfigStillChanges(t *testing.T) {
	res := exampleResource(
		&tfAttr{Name: "name", Kind: "String", Required: true},
		&tfAttr{Name: "field", Kind: "String", Required: true, Example: strExample("PLATFORM")},
	)
	data := exampleAcceptance(t, res)
	if !strings.Contains(data.Updated.Config, `name = "@{run}-name-updated"`) || !strings.Contains(data.Updated.Config, `field = "PLATFORM"`) {
		t.Errorf("updated config:\n%s\nwant name changed and field kept", data.Updated.Config)
	}
	all := exampleResource(
		&tfAttr{Name: "name", Kind: "String", Required: true, Example: strExample("one")},
		&tfAttr{Name: "limit", Kind: "Int64", Optional: true, Example: &contractExample{Items: []exampleItem{{Values: []exampleValue{{Text: "5", Tag: "!!int"}}}}}},
	)
	data = exampleAcceptance(t, all)
	if data.Full.Config == data.Updated.Config {
		t.Errorf("the updated config equals the full config:\n%s", data.Updated.Config)
	}
	if !strings.Contains(data.Full.Config, `name = "one"`) || !strings.Contains(data.Updated.Config, `name = "@{run}-name-updated"`) {
		t.Errorf("full config:\n%s\nupdated config:\n%s\nwant the example in the full config and made-up values in the updated one", data.Full.Config, data.Updated.Config)
	}
}

func TestReadExamples(t *testing.T) {
	spec := `
components:
  schemas:
    Thing:
      properties:
        name: {type: string, examples: [a, b]}
        count: {type: integer, example: 3}
        tags: {type: array, items: {type: string, examples: [t]}}
        names: {type: array, items: {type: string}, examples: [[x, y]]}
        object: {type: object, examples: [{k: v}]}
        none: {type: string}
`
	got := readExamples([]byte(spec))
	if len(got) != 4 || len(got["Thing.name"].Items) != 2 || got["Thing.count"].Items[0].Values[0].Tag != "!!int" ||
		got["Thing.tags"].Items[0].Values[0].Text != "t" || !got["Thing.names"].Items[0].Array || len(got["Thing.names"].Items[0].Values) != 2 {
		t.Errorf("readExamples = %+v, want name, count, tags and names", got)
	}
}

func exampleThingSpec(t *testing.T, edits ...[2]string) []byte {
	t.Helper()
	spec := string(maskThingSpec(t))
	for _, edit := range edits {
		if !strings.Contains(spec, edit[0]) {
			t.Fatalf("the spec has no %q", edit[0])
		}
		spec = strings.Replace(spec, edit[0], edit[1], 1)
	}
	return []byte(spec)
}

// examplesEdits states the examples of the mask thing: a name with two values, a filter value, a fixed
// range with ordered times, a relative range, and an example on an enum, which the test ignores.
var examplesEdits = [][2]string{
	{"        name: {type: string, minLength: 1, description: The name of the thing.}\n        description: {type: string, description: A note that can be cleared.}",
		"        name: {type: string, minLength: 1, examples: [mask-thing-one, mask-thing-two], description: The name of the thing.}\n        description: {type: string, description: A note that can be cleared.}"},
	{"        field: {$ref: '#/components/schemas/MaskThingStringField'}\n        value: {type: string, minLength: 1, description: The text to match.}",
		"        field: {$ref: '#/components/schemas/MaskThingStringField', examples: [MASK_THING_STRING_FIELD_COUNTRY]}\n        value: {type: string, minLength: 1, examples: [match-text], description: The text to match.}"},
	{"        start: {type: string, minLength: 1, description: The first time of the range.}\n        end: {type: string, minLength: 1, description: The last time of the range.}",
		"        start: {type: string, minLength: 1, examples: ['1763251200000'], description: The first time of the range.}\n        end: {type: string, minLength: 1, examples: ['1763337600000'], description: The last time of the range.}"},
	{"        minutes: {type: integer, format: int32, minimum: 1, description: How many minutes back the range goes.}",
		"        minutes: {type: integer, format: int32, minimum: 1, examples: [30, 60], description: How many minutes back the range goes.}"},
}

func generateExampleThing(t *testing.T, spec []byte) (string, error) {
	t.Helper()
	return generateMaskThing(t, spec)
}

// The golden output of a new resource whose contract states examples.
func TestGoldenOutputExampleThing(t *testing.T) {
	out, err := generateExampleThing(t, exampleThingSpec(t, examplesEdits...))
	if err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "golden", "examplething")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputExampleThing -update:\n%s", diff)
	}
	acceptance := readGenerated(t, out, "acceptance_test.go")
	for _, want := range []string{
		`name = "mask-thing-one"`,
		`name = "mask-thing-two"`,
		`value = "match-text"`,
		`start = "1763251200000"`,
		`minutes = 30`,
		`name = "mask-thing-two"`,
		`"string_filter.field": "MASK_THING_STRING_FIELD_PLATFORM"`,
	} {
		if !strings.Contains(acceptance, want) {
			t.Errorf("acceptance_test.go does not contain %q", want)
		}
	}
	if strings.Contains(acceptance, "match-text-") {
		t.Error("a copy of a sweep step numbers the example")
	}
}

// An existing resource ignores the examples of its contract, even one that the schema rules out.
func TestExamplesDoNotChangeAnExistingResource(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "legacy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(spec), "        name:\n          type: string\n", "        name:\n          examples: [12345]\n          type: string\n", 1)
	if edited == string(spec) {
		t.Fatal("no change")
	}
	generate := func(openAPI string) string {
		input, loadDir := syntheticInput(t)
		input.OpenAPI = []byte(openAPI)
		dir := legacyOutputWithAcceptance(t)
		options := Options{Resource: "LegacyThing", OutputDir: dir, OverridesPath: filepath.Join("testdata", "legacy-overrides.yaml")}
		if err := generateFromInput(options, input, loadDir); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	if diff := compareDirectories(generate(string(spec)), generate(edited)); diff != "" {
		t.Fatalf("an example changed the output of an existing resource:\n%s", diff)
	}
}

// check rejects an example that the schema of its own property rules out.
func TestCheckRejectsInvalidExamples(t *testing.T) {
	tests := map[string]struct {
		old, new string
		want     string
	}{
		"below the minimum": {"examples: [30, 60]", "examples: [0]", "below the minimum"},
		"wrong json type":   {"examples: [30, 60]", "examples: ['soon']", "JSON type string"},
		"too short":         {"examples: [mask-thing-one, mask-thing-two]", "examples: ['']", "shorter than minLength"},
		"not in the enum":   {"examples: [MASK_THING_STRING_FIELD_COUNTRY]", "examples: [MASK_THING_STRING_FIELD_NOPE]", "no value of the enum"},
		"a string for text": {"examples: [match-text]", "examples: [5]", "JSON type number"},
		"an array":          {"examples: [match-text]", "examples: [[a, b]]", "is an array"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			spec := exampleThingSpec(t, examplesEdits...)
			spec = []byte(strings.Replace(string(spec), test.old, test.new, 1))
			candidate := filepath.Join(t.TempDir(), "candidate.yaml")
			if err := os.WriteFile(candidate, spec, 0o644); err != nil {
				t.Fatal(err)
			}
			err := Check(CheckOptions{Resource: "MaskThing", OpenAPIPath: candidate})
			var eligibility *EligibilityError
			if !errors.As(err, &eligibility) || !hasReportCode(eligibility.Report, "EXAMPLE_INVALID") || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("check = %v, want EXAMPLE_INVALID with %q", err, test.want)
			}
		})
	}
	t.Run("valid examples pass", func(t *testing.T) {
		candidate := filepath.Join(t.TempDir(), "candidate.yaml")
		if err := os.WriteFile(candidate, exampleThingSpec(t, examplesEdits...), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Check(CheckOptions{Resource: "MaskThing", OpenAPIPath: candidate}); err != nil {
			t.Fatalf("check: %v", err)
		}
	})
}

// The pattern, the int64 carried as a string, and the list limits are part of the check.
func TestExampleProblem(t *testing.T) {
	text := func(s string) *contractExample { return strExample(s) }
	int64Type := &modelType{Kind: "integer", WireString: true}
	tests := map[string]struct {
		typ  *modelType
		ex   *contractExample
		want string
	}{
		"pattern ok":        {&modelType{Kind: "string", Pattern: `^[A-Z]+$`}, text("PLATFORM"), ""},
		"pattern fails":     {&modelType{Kind: "string", Pattern: `^[A-Z]+$`}, text("platform"), "does not match the pattern"},
		"int64 string":      {int64Type, text("1763251200000"), ""},
		"int64 not digits":  {int64Type, text("1.5"), "is no integer"},
		"plain int string":  {&modelType{Kind: "integer"}, text("5"), "JSON type string"},
		"list elements ok":  {&modelType{Kind: "list", Elem: &modelType{Kind: "string"}}, listExample("a", "b"), ""},
		"list one element":  {&modelType{Kind: "list", Elem: &modelType{Kind: "string"}}, text("a"), ""},
		"list too long":     {&modelType{Kind: "list", Elem: &modelType{Kind: "string"}, MaxItems: int64Ptr(1)}, listExample("a", "b"), "2 elements"},
		"element too short": {&modelType{Kind: "list", Elem: &modelType{Kind: "string", MinLength: int64Ptr(2)}}, listExample("a"), "shorter than minLength"},
		"object ignored":    {&modelType{Kind: "object"}, text("x"), ""},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got := exampleProblem(test.typ, test.ex)
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("problem = %q, want %q", got, test.want)
			}
		})
	}
}

func int64Ptr(v int64) *int64 { return &v }

type modelType = model.Type
