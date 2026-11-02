package generator

import (
	"os"
	"path/filepath"
	"testing"
)

// The golden output of a released resource like the parsing rule groups: a list of oneOf
// matchers shown as one set per arm (splitArms), and scalars that the released resource sent as
// their zero value when the plan was unknown (sendUnknownAsZero).
func TestGoldenOutputParseThing(t *testing.T) {
	out := generateParseThing(t)
	golden := filepath.Join("testdata", "golden", "parsething")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputParseThing -update:\n%s", diff)
	}
}

// The generated resource runs: see testdata/parsething_semantics_test.go.
func TestParseThingRuntimeSemantics(t *testing.T) {
	input, _ := syntheticInput(t)
	out := generateParseThing(t)
	semantics, err := os.ReadFile(filepath.Join("testdata", "parsething_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

func generateParseThing(t *testing.T) string {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "parsething.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "parsething")
	options := Options{Resource: "ParseThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "parsething-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	return out
}
