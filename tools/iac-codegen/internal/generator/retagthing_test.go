package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coralogix/coralogix-management-sdk/iac/model"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

func TestGoldenOutputRetagThing(t *testing.T) {
	out := generateRetagThing(t)
	golden := filepath.Join("testdata", "golden", "retagthing")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputRetagThing -update:\n%s", diff)
	}
}

// The ids of the tags are computed. Create sends the ids that a BeforeWrite hook reads, Update the ids
// of the state, and both take the state from the write.
func TestRetagThingRuntimeSemantics(t *testing.T) {
	input, _ := syntheticInput(t)
	out := generateRetagThing(t)
	semantics, err := os.ReadFile(filepath.Join("testdata", "retagthing_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

func generateRetagThing(t *testing.T) string {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "retagthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "retagthing")
	options := Options{Resource: "TagList", OutputDir: out, OverridesPath: filepath.Join("testdata", "retagthing-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	return out
}

// computed: true on a field that the request requires is accepted only under the narrow conditions
// of a server-set item id: Get marks the field read-only, the line keeps the state value, and the
// resource is an overwrite resource.
func TestComputedRequestFieldNeedsAServerSetOverwriteID(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "retagthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join("testdata", "retagthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for name, test := range map[string]struct{ from, to, want string }{
		"without useStateForUnknown": {
			from: "id: {computed: true, useStateForUnknown: true}", to: "id: {computed: true}",
			want: "needs useStateForUnknown",
		},
		"on a field that Get does not mark read-only": {
			from: "id: {computed: true, useStateForUnknown: true}", to: "name: {computed: true, useStateForUnknown: true}",
			want: "required: true needs a field that the user sets",
		},
	} {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(string(raw), test.from) {
				t.Fatalf("the overrides have no %q", test.from)
			}
			file := mustParse(t, strings.Replace(string(raw), test.from, test.to, 1))
			_, err := validateOpenAPIWith(spec, "TagList", model.OperationIDs{}, "sdk", "provider", file)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
}

// Without api.overwrite, the line stays an error: only an overwrite resource sends the server id of
// an item back in a write.
func TestServerSetInRequestNeedsOverwrite(t *testing.T) {
	computed := true
	a := &tfAttr{Required: true, ServerSetInRequest: true}
	err := serverSetInRequest(a, overrides.Field{Computed: &computed, UseStateForUnknown: true}, &overrides.File{})
	if err == nil || !strings.Contains(err.Error(), "needs api.overwrite") || !a.Required {
		t.Fatalf("err = %v, required = %t; want the error and the attribute unchanged", err, a.Required)
	}
}
