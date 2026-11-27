package generator

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// A component that one place computes and another place writes needs one Go type for a nested
// object field in all places. The model holds a types.Object, and expand and flatten read and write
// it, so the generated package compiles.
func TestSharedComponentObjectFieldIsOneType(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "placething.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := copiedSyntheticInput(t)
	input.OpenAPI = data
	// The copied synthetic provider gets a client for the new service, in a file of its own.
	client := "package clientset\n\nimport \"example.com/iac-test-sdk/go/openapi/gen/place_things_service\"\n\n" +
		"func (c *ClientSet) PlaceThings() *place_things_service.PlaceThingsServiceAPIService {\n\treturn &place_things_service.PlaceThingsServiceAPIService{}\n}\n"
	if err := os.WriteFile(filepath.Join(loadDir, "internal", "clientset", "place_things.go"), []byte(client), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "placething")
	if err := generateFromInput(Options{Resource: "PlaceThing", OutputDir: out}, input, loadDir); err != nil {
		t.Fatal(err)
	}
	model, err := os.ReadFile(filepath.Join(out, "model.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(model, []byte("Stamp types.Object `tfsdk:\"stamp\"`")) {
		t.Fatalf("the nested object field is not a types.Object in the model:\n%s", model)
	}
	compileGenerated(t, out, input)
}
