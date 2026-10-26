package generator

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coralogix/coralogix-management-sdk/iac/source"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/version"
)

// inProvider returns a copied synthetic provider and the output directory of name in it.
func inProvider(t *testing.T, name string) (source.Input, string, string) {
	t.Helper()
	input, loadDir := copiedSyntheticInput(t)
	return input, loadDir, filepath.Join(input.ModuleRoot, "internal", "provider", "generated", name)
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

// The golden provider files of the new synthetic resource: the registry, the docs example, the
// import command, and the docs example of the data source. A second run writes the same files over
// the first.
func TestGoldenProviderFiles(t *testing.T) {
	input, loadDir, out := inProvider(t, "thing")
	for range 2 {
		if err := generateFromInput(Options{Resource: "Thing", OutputDir: out}, input, loadDir); err != nil {
			t.Fatal(err)
		}
	}
	compileRegistry(t, input.ModuleRoot, out)
	examples := filepath.Join(input.ModuleRoot, "examples", "resources", "coralogix_thing")
	dataSourceExamples := filepath.Join(input.ModuleRoot, "examples", "data-sources", "coralogix_thing")
	got := map[string]string{
		registryFileName: readFile(t, filepath.Join(filepath.Dir(out), registryFileName)),
		"resource.tf":    readFile(t, filepath.Join(examples, "resource.tf")),
		"import.sh":      readFile(t, filepath.Join(examples, "import.sh")),
		"data-source.tf": readFile(t, filepath.Join(dataSourceExamples, "data-source.tf")),
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
	if entries, err = os.ReadDir(dataSourceExamples); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("data source examples directory has %d entries, want data-source.tf: a staged file stayed", len(entries))
	}
}

// The docs example of the data source of a new resource with a numeric id sets a number, because
// Terraform rejects a string for an Int64 attribute.
func TestGoldenDataSourceExampleNumericID(t *testing.T) {
	input, loadDir, out := inProvider(t, "thing")
	input.OpenAPI = integerIDOpenAPI(t, input.OpenAPI, "int64")
	integerIDSDK(t, input.SDKDir, "int64")
	if err := generateFromInput(Options{Resource: "Thing", OutputDir: out}, input, loadDir); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, filepath.Join(input.ModuleRoot, "examples", "data-sources", "coralogix_thing", "data-source.tf"))
	path := filepath.Join("testdata", "golden", "provider", "data-source-int64.tf")
	if *updateGolden {
		writeFile(t, path, got)
	}
	if want := readFile(t, path); got != want {
		t.Errorf("data-source.tf differs from %s; run go test ./internal/generator -run TestGoldenDataSourceExampleNumericID -update:\n%s", path, got)
	}
}

// compileRegistry builds the registry with the generated package in the provider, as provider.go
// imports them.
func compileRegistry(t *testing.T, root, out string) {
	t.Helper()
	// The acceptance test imports the provider package, which the synthetic provider does not have.
	if err := os.Remove(filepath.Join(out, "acceptance_test.go")); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"mod", "tidy"}, {"build", "./internal/provider/generated/..."}} {
		command := exec.Command("go", args...)
		command.Dir = root
		command.Env = append(os.Environ(), "GOWORK=off")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, output)
		}
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
	for _, want := range []string{"alpha.NewResource,", "thing.NewResource,"} {
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
	if _, err := os.Stat(filepath.Join(input.ModuleRoot, "examples")); !os.IsNotExist(err) {
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
			path := test.path(input.ModuleRoot, out)
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

// An earlier run that generated another resource in the output directory does not own the example
// of this resource.
func TestExampleOfAnotherResourceStays(t *testing.T) {
	input, loadDir, out := inProvider(t, "thing")
	if err := generateFromInput(Options{Resource: "Thing", OutputDir: out}, input, loadDir); err != nil {
		t.Fatal(err)
	}
	example := filepath.Join(input.ModuleRoot, "examples", "resources", "coralogix_thing", "resource.tf")
	writeFile(t, example, "handwritten\n")
	resource := filepath.Join(out, "resource.go")
	writeFile(t, resource, strings.Replace(readFile(t, resource), `const TypeName = "thing"`, `const TypeName = "other"`, 1))
	err := generateFromInput(Options{Resource: "Thing", OutputDir: out}, input, loadDir)
	if err == nil || !strings.Contains(err.Error(), "tfgen did not write it") {
		t.Fatalf("err = %v, want the example of another resource kept", err)
	}
	if got := readFile(t, example); got != "handwritten\n" {
		t.Fatalf("the generator replaced the handwritten example:\n%s", got)
	}
}

// A run that fails after staging leaves no empty example directory behind: the next run would take
// it for a handwritten one.
func TestFailedRunRemovesTheExampleDirectory(t *testing.T) {
	input, loadDir, out := inProvider(t, "thing")
	writeFile(t, filepath.Join(out, "handwritten.go"), "package thing\n") // publish refuses this output
	err := generateFromInput(Options{Resource: "Thing", OutputDir: out}, input, loadDir)
	if err == nil || !strings.Contains(err.Error(), "not generator-owned") {
		t.Fatalf("err = %v, want the output refused", err)
	}
	if _, err := os.Stat(filepath.Join(input.ModuleRoot, "examples")); !os.IsNotExist(err) {
		t.Fatalf("a failed run left the examples directory: %v", err)
	}
	if err := os.RemoveAll(out); err != nil {
		t.Fatal(err)
	}
	if err := generateFromInput(Options{Resource: "Thing", OutputDir: out}, input, loadDir); err != nil {
		t.Fatalf("the run after the fix failed: %v", err)
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
		"id":        {&crudData{TypeName: "thing", IDAttr: "id"}, "terraform import coralogix_thing.example \"<id>\"\n"},
		"custom id": {&crudData{TypeName: "thing", IDAttr: "key"}, "terraform import coralogix_thing.example \"<key>\"\n"},
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

// The provider imports only the registry in internal/provider/generated, so the output must be a
// direct child of that directory.
func TestOutputLocation(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "internal", "provider", "generated")
	for out, ok := range map[string]bool{
		filepath.Join(generated, "thing"):                             true,
		filepath.Join(root, "internal", "provider", "other", "thing"): false,
		filepath.Join(generated, "a", "thing"):                        false,
		generated:                                                     false,
		filepath.Join(t.TempDir(), "thing"):                           false,
	} {
		err := checkOutputLocation(root, out)
		if ok != (err == nil) {
			t.Errorf("checkOutputLocation(%s) = %v, want ok %v", out, err, ok)
		}
	}
}
