package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A singleton that the API cannot create or remove: a GET reads it, and a PATCH with an update mask
// changes it. The generator needs no behavior-overrides file for it. Create sends the PATCH with a
// mask of the configured fields, and Delete makes no call.
func TestGoldenOutputAlwaysThing(t *testing.T) {
	input, loadDir := syntheticInput(t)
	spec, err := os.ReadFile(filepath.Join("testdata", "always-thing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "alwaysthing")
	if err := generateFromInput(Options{Resource: "AlwaysThing", OutputDir: out}, input, loadDir); err != nil {
		t.Fatal(err)
	}

	golden := filepath.Join("testdata", "golden", "alwaysthing")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputAlwaysThing -update:\n%s", diff)
	}

	resource := readGenerated(t, out, "resource.go")
	for _, want := range []string{
		"createRequest(ctx, req.Config, req.Plan)",
		"v, err = r.update(ctx, SingletonID, body, mask)",
		"func (r *Resource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse)",
	} {
		if !strings.Contains(resource, want) {
			t.Errorf("resource.go does not contain %q", want)
		}
	}
	if strings.Contains(resource, "func (r *Resource) delete(") || strings.Contains(resource, "func (r *Resource) create(") {
		t.Error("resource.go has a create or delete API call, but the API has neither")
	}
	// A field that the configuration leaves out keeps the value of the API.
	schema := readGenerated(t, out, "schema.go")
	for _, want := range []string{"boolplanmodifier.UseStateForUnknown()", "stringplanmodifier.UseStateForUnknown()"} {
		if !strings.Contains(schema, want) {
			t.Errorf("schema.go does not contain %q", want)
		}
	}
	compileGenerated(t, out, input)
}

func readGenerated(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
