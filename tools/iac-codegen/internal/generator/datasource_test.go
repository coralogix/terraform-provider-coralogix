package generator

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

// The List of the synthetic things API, and its response component.
const (
	thingsList = `    get:
      tags: [Things Service]
      operationId: ThingsService_ListThings
`
	thingsListResponse = `    ListThingsResponse:
      type: object
      properties:
        things:
          type: array
          items:
            $ref: '#/components/schemas/Thing'
`
)

// A new resource looks up the name only when the List of the API has items with a string name and
// returns every item in one response. Otherwise it looks up the id only, with no error.
func TestNewDataSourceLooksUpTheNameOnlyWithAUsableList(t *testing.T) {
	input, _ := syntheticInput(t)
	spec := string(input.OpenAPI)
	tests := map[string]struct {
		spec string
		name bool
	}{
		"full List with names": {spec, true},
		"paged List": {replaceOnce(t, spec, thingsListResponse,
			thingsListResponse+"        nextPageToken: {type: string}\n"), false},
		"List item without a name": {replaceOnce(t, spec, thingsListResponse,
			strings.Replace(thingsListResponse, "Thing'", "ThingRef'", 1)+"    ThingRef:\n      type: object\n      properties:\n        id: {type: string}\n"), false},
		"no List": {replaceOnce(t, spec, thingsList, "    x-no-list:\n      operationId: ThingsService_ListThings\n"), false},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			validated, err := validateOpenAPI([]byte(test.spec), "Thing", model.OperationIDs{}, "example.com/iac-test-sdk", providerModulePath)
			if err != nil {
				t.Fatal(err)
			}
			files, err := renderWith(validated.resource, validated.refs, "thing", nil)
			if err != nil {
				t.Fatal(err)
			}
			dataSource := string(files[dataSourceFile])
			if got := strings.Contains(dataSource, "func (d *DataSource) idByName("); got != test.name {
				t.Fatalf("name lookup = %v, want %v:\n%s", got, test.name, dataSource)
			}
		})
	}
}

// On a released resource, name in dataSource.lookup needs a List with items that have a string
// name and that returns every item in one response. The error says why the List does not do.
func TestDataSourceNameLookupNeedsAUsableList(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "sharedthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(filepath.Join("testdata", "sharedthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	spec, overridesText := string(data), string(text)
	const summary = "    SharedThingSummary:\n      type: object\n      properties:\n        id: {type: string}\n        name: {type: string}\n"
	const listResponse = "        summaries: {type: array, items: {$ref: '#/components/schemas/SharedThingSummary'}}\n"
	const listLine = "  operations:\n    list: SharedThingsService_ListSharedThingSummaries\n"
	// badList is a GET on the Create path whose response has no items.
	const badList = "    get:\n      tags: [Shared Things Service]\n      operationId: SharedThingsService_ListSharedThings\n" +
		"      responses:\n        '200':\n          description: ok\n          content:\n            application/json:\n" +
		"              schema: {$ref: '#/components/schemas/DeleteSharedThingResponse'}\n"
	tests := map[string]struct {
		spec, overrides, want string
	}{
		"item without a name": {
			replaceOnce(t, spec, summary, strings.Replace(summary, "        name: {type: string}\n", "", 1)), overridesText,
			"the List item SharedThingSummary has no string property name",
		},
		"paged List": {
			replaceOnce(t, spec, listResponse, listResponse+"        nextPageToken: {type: string}\n"), overridesText,
			"the List SharedThingsService_ListSharedThingSummaries pages its result",
		},
		"no List": {
			spec, replaceOnce(t, overridesText, listLine, ""),
			"the API has no List on the Create path; name one with api.operations.list",
		},
		"List that the model cannot describe": {
			replaceOnce(t, spec, "  /shared/v1/things:\n", "  /shared/v1/things:\n"+badList), replaceOnce(t, overridesText, listLine, ""),
			"the List of the API cannot be used: The response DeleteSharedThingResponse has no array of component objects.",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			file := mustParse(t, test.overrides)
			_, err := validateOpenAPIWith([]byte(test.spec), "SharedThing", model.OperationIDs{}, "sdk", "provider", file)
			if got := eligibilityCodes(t, err); !slices.Contains(got, "OVERRIDE_INVALID") || !strings.Contains(err.Error(), test.want) ||
				!strings.Contains(err.Error(), "dataSource.lookup") {
				t.Fatalf("err = %v, want OVERRIDE_INVALID on dataSource.lookup with %q", err, test.want)
			}
		})
	}
}

// The model keeps the List only when the data source looks up the name, so the generator checks the
// SDK names of a List only when the generated code calls it.
func TestUnusedListIsDropped(t *testing.T) {
	usable := func() *model.ListOperation { return &model.ListOperation{NameField: "name"} }
	file := func(t *testing.T, dataSource string) *overrides.File {
		t.Helper()
		f, err := overrides.Parse([]byte("resource: Thing\nmode: existing\nvalidators:\n  inferred: false\n" + dataSource))
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	tests := []struct {
		name string
		r    *model.Resource
		file *overrides.File
		keep bool
	}{
		{"new resource, usable List", &model.Resource{List: usable()}, nil, true},
		{"new resource, paged List", &model.Resource{List: &model.ListOperation{NameField: "name", Paged: true}}, nil, false},
		{"new resource, List without name", &model.Resource{List: &model.ListOperation{}}, nil, false},
		{"released resource, no lookup line", &model.Resource{List: usable()}, file(t, "dataSource: {}\n"), false},
		{"released resource, no data source", &model.Resource{List: usable()}, file(t, ""), false},
		{"released resource, name lookup", &model.Resource{List: usable()}, file(t, "dataSource: {lookup: [id, name]}\n"), true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dropUnusedList(test.r, test.file)
			if kept := test.r.List != nil; kept != test.keep {
				t.Fatalf("List kept = %v, want %v", kept, test.keep)
			}
		})
	}
}

// A show line adds an attribute to the data source that the resource skips. The generator reports a
// line that the file cannot honor, instead of writing a data source that differs from the line.
func TestDataSourceShowChecksTheLine(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "legacy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(filepath.Join("testdata", "legacy-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]struct {
		edit func(string) string
		want string
	}{
		"the resource does not skip the field": {
			func(s string) string { return replaceOnce(t, s, "      managed: {skip: true}\n", "") },
			"does not skip LegacyRule.managed",
		},
		"a field that the component does not have": {
			func(s string) string {
				return replaceOnce(t, s, "        managed:\n          show: true", "        nope:\n          show: true")
			},
			"does not skip LegacyRule.nope",
		},
		"a component that the resource has no list of": {
			func(s string) string {
				return replaceOnce(t, s, "    LegacyRule:\n      fields:\n        name:\n          description: The name of the rule that the data source shows.\n        managed:", "    LegacyLabels:\n      fields:\n        managed:")
			},
			"managed",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			file := mustParse(t, test.edit(string(text)))
			_, err := validateOpenAPIWith(spec, "LegacyThing", model.OperationIDs{}, "sdk", "provider", file)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want one with %q", err, test.want)
			}
		})
	}
}

func TestDataSourceShowNeedsADescription(t *testing.T) {
	text, err := os.ReadFile(filepath.Join("testdata", "legacy-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = overrides.Parse([]byte(replaceOnce(t, string(text), "          description: Whether the rule is managed.\n", "")))
	if err == nil || !strings.Contains(err.Error(), "show needs a description") {
		t.Fatalf("err = %v, want show needs a description", err)
	}
}

// A shown attribute takes a Terraform name that no attribute of the item has: foo-bar and foo_bar are
// both foo_bar, so one would overwrite the other in the data source.
func TestDataSourceShowRejectsANameThatTheItemHas(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "legacy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(filepath.Join("testdata", "legacy-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	// The rule field name is renamed to managed, which is the name of the shown field.
	renamed := replaceOnce(t, string(text), "      name:\n        description: \"The rule name.\"\n        required: true\n", "      name:\n        description: \"The rule name.\"\n        required: true\n        name: managed\n")
	file := mustParse(t, renamed)
	_, err = validateOpenAPIWith(spec, "LegacyThing", model.OperationIDs{}, "sdk", "provider", file)
	if err == nil || !strings.Contains(err.Error(), "already has") {
		t.Fatalf("err = %v, want a Terraform name collision of the shown attribute", err)
	}
}

func TestDataSourceDescriptionNeedsAnAttribute(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "legacy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(filepath.Join("testdata", "legacy-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	edited := replaceOnce(t, string(text), "        name:\n          description: The name of the rule that the data source shows.\n", "        nope:\n          description: x\n")
	_, err = validateOpenAPIWith(spec, "LegacyThing", model.OperationIDs{}, "sdk", "provider", mustParse(t, edited))
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("err = %v, want an error that names the field nope", err)
	}
}
