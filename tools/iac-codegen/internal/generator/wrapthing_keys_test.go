package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// generateWrapThingWith generates the wrapthing contract with its overrides file changed: the first
// from in it becomes to. It returns the error of the generator.
func generateWrapThingWith(t *testing.T, from, to string) error {
	t.Helper()
	base, err := os.ReadFile(filepath.Join("testdata", "wrapthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	spec, err := os.ReadFile(filepath.Join("testdata", "wrapthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(base), from, to, 1)
	if changed == string(base) {
		t.Fatalf("the overrides file has no %q", from)
	}
	dir := t.TempDir()
	overrides := filepath.Join(dir, "wrapthing-overrides.yaml")
	if err := os.WriteFile(overrides, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	return generateFromInput(Options{Resource: "WrapThing", OutputDir: filepath.Join(dir, "out"), OverridesPath: overrides}, input, loadDir)
}

// clientID: uuid needs a string, states computed itself, and takes only the value uuid.
func TestClientIDKeyIsChecked(t *testing.T) {
	tests := map[string]struct{ from, to, want string }{
		"another kind of id":        {"id: {clientID: uuid}", "id: {clientID: guid}", `want "uuid"`},
		"a line that says computed": {"id: {clientID: uuid}", "id: {clientID: uuid, computed: true}", "clientID makes the attribute computed"},
		"a field that is no string": {"      span:\n        terraformType: Int64\n", "      span:\n        terraformType: Int64\n      limit: {clientID: uuid}\n", "clientID: uuid needs a string attribute"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if err := generateWrapThingWith(t, test.from, test.to); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want it to contain %q", err, test.want)
			}
		})
	}
}

// join needs a list of plain strings, and cannot go with the keys that show the list another way.
func TestJoinKeyIsChecked(t *testing.T) {
	tests := map[string]struct{ from, to, want string }{
		"a field that is no list": {"      span:\n        terraformType: Int64\n", "      span:\n        terraformType: Int64\n      text: {join: \"/\"}\n", "join"},
		"with collection":         {`segments: {join: "/"}`, `segments: {join: "/", collection: set}`, "join shows a list of strings as one string"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if err := generateWrapThingWith(t, test.from, test.to); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want it to contain %q", err, test.want)
			}
		})
	}
}

// generateWrapThingSpecWith generates the wrapthing contract with its overrides file as it is and the
// from in the contract changed to to, for each pair of changes.
func generateWrapThingSpecWith(t *testing.T, changes ...[2]string) error {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "wrapthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	changed := string(spec)
	for _, change := range changes {
		if !strings.Contains(changed, change[0]) {
			t.Fatalf("the contract has no %q", change[0])
		}
		changed = strings.ReplaceAll(changed, change[0], change[1])
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = []byte(changed)
	return generateFromInput(Options{Resource: "WrapThing", OutputDir: filepath.Join(t.TempDir(), "out"), OverridesPath: filepath.Join("testdata", "wrapthing-overrides.yaml")}, input, loadDir)
}

// distributeTo names a list of objects of the resource, a component that the resource holds, and two
// string fields that hold the id. The line states only the texts of the attribute, and the list must
// be sent whole by a request that replaces the resource.
func TestDistributeToKeyIsChecked(t *testing.T) {
	line := "          by: {listField: panelId, itemField: id}\n"
	other := "      name: {distributeTo: {item: WrapPanel, as: moved, by: {listField: panelId, itemField: id}}}\n"
	tests := map[string]struct{ from, to, want string }{
		"an item that the resource does not hold":              {"item: WrapPanel", "item: Elsewhere", "item Elsewhere is no component that the resource holds"},
		"the list itself as the item":                          {"item: WrapPanel", "item: WrapNote", "item WrapNote is no component that the resource holds"},
		"a list field that does not exist":                     {"listField: panelId", "listField: nopeId", "by.listField: WrapNote has no field nopeId"},
		"an item field that does not exist":                    {"itemField: id", "itemField: nope", "by.itemField: WrapPanel has no field nope"},
		"a list field that is no string":                       {"listField: panelId", "listField: detail", "the field detail of WrapNote is no plain string"},
		"a name that an attribute has":                         {"as: panel_notes", "as: title", "WrapPanel has the attribute title already"},
		"a name that is no attribute name":                     {"as: panel_notes", "as: Panel-Notes", "is not a Terraform attribute name"},
		"no name":                                              {"          as: panel_notes\n", "", "distributeTo needs item, as, and by"},
		"no field that holds the id":                           {line, "          by: {listField: panelId}\n", "distributeTo needs item, as, and by"},
		"a keep path that does not exist":                      {"keep: detail.url", "keep: detail.nope", "keep: NoteDetail has no field nope"},
		"a keep path that is always set":                       {"keep: detail.url", "keep: id", "keep: id of WrapNote is set in every item"},
		"a keep path through a string":                         {"keep: detail.url", "keep: text.nope", "keep: \"text.nope\" goes through a value that has no fields"},
		"with skip":                                            {"      notes:\n", "      notes:\n        skip: true\n", "distributeTo moves the list into another component"},
		"with readOnly":                                        {"      notes:\n", "      notes:\n        readOnly: true\n", "distributeTo moves the list into another component"},
		"with name":                                            {"      notes:\n", "      notes:\n        name: other\n", "distributeTo moves the list into another component"},
		"on a field that is no list":                           {"      rawQuery:\n", other + "      rawQuery:\n", "distributeTo needs a list of objects of a component, but name is not one"},
		"on a field of another component":                      {"  SimpleFilter:\n    fields:\n", "  SimpleFilter:\n    fields:\n      text: {distributeTo: {item: WrapPanel, as: moved, by: {listField: panelId, itemField: id}}}\n", "distributeTo works on a top-level field of the resource WrapThing"},
		"in the moved items, a key that needs the prior value": {"  WrapNote:\n    fields:\n", "  WrapNote:\n    fields:\n      text: {equality: json}\n", "distributeTo does not support equality"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if err := generateWrapThingWith(t, test.from, test.to); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want it to contain %q", err, test.want)
			}
		})
	}
}

// distributeTo cannot move a list that a request requires, or that another component holds too.
func TestDistributeToNeedsAListThatTheContractAllows(t *testing.T) {
	inBody, inResponse := "                notes:\n                  type: array\n", "        notes:\n          type: array\n"
	tests := map[string]struct {
		changes [][2]string
		want    string
	}{
		"a list that Create requires": {[][2]string{
			{"              required: [query]\n", "              required: [query, notes]\n"},
			{"      required: [id, query]\n      properties:\n        id: {type: string, readOnly: true}", "      required: [id, query, notes]\n      properties:\n        id: {type: string, readOnly: true}"},
			{inBody, inBody + "                  minItems: 1\n"},
			{inResponse, inResponse + "          minItems: 1\n"},
		}, "Create and Update leave optional"},
		"a component that another place holds": {[][2]string{
			{"        title: {type: string, x-coralogix-presence: true}\n", "        title: {type: string, x-coralogix-presence: true}\n        pinned: {$ref: '#/components/schemas/WrapNote'}\n"},
		}, "WrapNote is held in other places too"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if err := generateWrapThingSpecWith(t, test.changes...); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want it to contain %q", err, test.want)
			}
		})
	}
}
