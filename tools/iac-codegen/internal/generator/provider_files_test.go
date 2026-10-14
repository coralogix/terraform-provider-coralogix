package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/source"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/version"
)

// inProvider returns a copied synthetic provider and the output directory of name in it.
func inProvider(t *testing.T, name string) (source.Input, string, string) {
	t.Helper()
	input, loadDir := copiedSyntheticInput(t)
	return input, loadDir, filepath.Join(input.ProviderRoot, "internal", "provider", "generated", name)
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The golden provider files of the new synthetic resource: the registry, and the docs example and
// import command. A second run writes the same files over the first.
func TestGoldenProviderFiles(t *testing.T) {
	input, loadDir, out := inProvider(t, "thing")
	for range 2 {
		if err := generateFromInput(Options{Resource: "Thing", OutputDir: out}, input, loadDir); err != nil {
			t.Fatal(err)
		}
	}
	examples := filepath.Join(input.ProviderRoot, "examples", "resources", "coralogix_thing")
	got := map[string]string{
		registryFileName: readFile(t, filepath.Join(filepath.Dir(out), registryFileName)),
		"resource.tf":    readFile(t, filepath.Join(examples, "resource.tf")),
		"import.sh":      readFile(t, filepath.Join(examples, "import.sh")),
	}
	golden := filepath.Join("testdata", "golden", "provider")
	for name, text := range got {
		path := filepath.Join(golden, name)
		if *updateGolden {
			writeFile(t, path, text)
		}
		if want := readFile(t, path); text != want {
			t.Errorf("%s differs from %s; run go test ./internal/generator -run TestGoldenProviderFiles -update:\n%s", name, path, text)
		}
	}
	entries, err := os.ReadDir(examples)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("examples directory has %d entries, want resource.tf and import.sh: a staged file stayed", len(entries))
	}
}

// The registry lists every new resource next to the output, and no resource with a
// behavior-overrides file: that one keeps its handwritten registration.
func TestRegistryListsOnlyNewResources(t *testing.T) {
	input, loadDir, out := inProvider(t, "thing")
	generated := filepath.Dir(out)
	header := version.Header + "\n\npackage x\n"
	writeFile(t, filepath.Join(generated, "alpha", "schema.go"), header)
	writeFile(t, filepath.Join(generated, "legacy", "schema.go"), header)
	writeFile(t, filepath.Join(generated, "legacy", overrides.FileName), "mode: existing\n")
	writeFile(t, filepath.Join(generated, "handwritten", "schema.go"), "package handwritten\n")
	if err := generateFromInput(Options{Resource: "Thing", OutputDir: out}, input, loadDir); err != nil {
		t.Fatal(err)
	}
	registry := readFile(t, filepath.Join(generated, registryFileName))
	for _, want := range []string{"alpha.NewResource(alpha.Hooks{})", "thing.NewResource(thing.Hooks{})"} {
		if !strings.Contains(registry, want) {
			t.Errorf("registry lacks %s:\n%s", want, registry)
		}
	}
	for _, unwanted := range []string{"legacy", "handwritten"} {
		if strings.Contains(registry, "generated/"+unwanted+`"`) {
			t.Errorf("registry lists %s:\n%s", unwanted, registry)
		}
	}
}

// A resource that users already have gets no docs files, and the registry does not list it.
func TestExistingResourceGetsNoDocsFiles(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "archived.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir, out := inProvider(t, "archivedthing")
	input.OpenAPI = spec
	writeFile(t, filepath.Join(out, overrides.FileName), readFile(t, filepath.Join("testdata", "archived-overrides.yaml")))
	if err := generateFromInput(Options{Resource: "ArchivedThing", OutputDir: out}, input, loadDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(input.ProviderRoot, "examples")); !os.IsNotExist(err) {
		t.Errorf("examples written for an existing resource: %v", err)
	}
	if registry := readFile(t, filepath.Join(filepath.Dir(out), registryFileName)); strings.Contains(registry, "archivedthing") {
		t.Errorf("registry lists an existing resource:\n%s", registry)
	}
}

// The registry tells a new resource from an existing one by the behavior-overrides file in its
// directory, so an existing resource in the provider must keep the file there.
func TestExistingResourceInProviderNeedsOverridesInOutput(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "archived.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir, out := inProvider(t, "archivedthing")
	input.OpenAPI = spec
	err = generateFromInput(Options{Resource: "ArchivedThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "archived-overrides.yaml")}, input, loadDir)
	if err == nil || !strings.Contains(err.Error(), "must be in it") {
		t.Fatalf("err = %v, want the overrides location rejected", err)
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatal("the generator wrote output although it rejected the run")
	}
}

// A handwritten example stays, and a registry without the generator header stays. In both cases
// the generator writes nothing.
func TestProviderFilesKeepHandwrittenFiles(t *testing.T) {
	tests := map[string]struct {
		path func(root, out string) string
		want string
	}{
		"example": {func(root, _ string) string {
			return filepath.Join(root, "examples", "resources", "coralogix_thing", "resource.tf")
		}, "tfgen did not write it"},
		"registry": {func(_, out string) string {
			return filepath.Join(filepath.Dir(out), registryFileName)
		}, "is not generator-owned"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			input, loadDir, out := inProvider(t, "thing")
			path := test.path(input.ProviderRoot, out)
			writeFile(t, path, "handwritten\n")
			err := generateFromInput(Options{Resource: "Thing", OutputDir: out}, input, loadDir)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
			if got := readFile(t, path); got != "handwritten\n" {
				t.Fatalf("the generator replaced the handwritten file:\n%s", got)
			}
			if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
				t.Fatal("the generator wrote output although it rejected the run")
			}
		})
	}
}

// The example has no value of the test environment: an environment variable becomes a placeholder
// with its name, and the id of the test run becomes "example".
func TestExampleHidesTheTestEnvironment(t *testing.T) {
	acc := &acceptanceData{
		Prerequisites: "resource \"coralogix_connector\" \"slack\" {\n  name = \"@{run}-slack\"\n  integration_id = \"@{env.SLACK_INTEGRATION_ID}\"\n}",
		Full:          &accStep{Config: "resource \"coralogix_thing\" \"test\" {\n  name = \"@{run}-name\"\n  connector_id = coralogix_connector.slack.id\n}"},
	}
	got, err := renderExample(acc, "thing")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`resource "coralogix_thing" "example" {`,
		`integration_id = "<SLACK_INTEGRATION_ID>"`,
		`name           = "example-slack"`,
		`name         = "example-name"`,
		`source = "coralogix/coralogix"`,
	} {
		if !strings.Contains(string(got), want) {
			t.Errorf("example lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(string(got), "@{") {
		t.Errorf("example keeps a placeholder of the test:\n%s", got)
	}
}

// The import command names the id attribute. A singleton has one id per company: its type name.
func TestImportCommand(t *testing.T) {
	tests := map[string]struct {
		crud *crudData
		want string
	}{
		"id":        {&crudData{TypeName: "thing", IDAttr: "id"}, "terraform import coralogix_thing.example <id>\n"},
		"custom id": {&crudData{TypeName: "thing", IDAttr: "key"}, "terraform import coralogix_thing.example <key>\n"},
		"singleton": {&crudData{TypeName: "settings", IDAttr: "id", Singleton: true}, "terraform import coralogix_settings.example settings\n"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if got := string(renderImport(test.crud)); got != test.want {
				t.Fatalf("import = %q, want %q", got, test.want)
			}
		})
	}
}

func TestWithinDir(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "provider")
	for path, want := range map[string]bool{
		root:                                   true,
		filepath.Join(root, "internal", "x"):   true,
		filepath.Join(root, "..", "elsewhere"): false,
		filepath.Join(root+"-other", "x"):      false,
	} {
		if got := withinDir(root, path); got != want {
			t.Errorf("withinDir(%s, %s) = %v, want %v", root, path, got, want)
		}
	}
}
