package generator

import (
	"os"
	"path/filepath"
	"testing"
)

// The golden output of a singleton like the metrics archive: one call sets it and returns an
// empty object, so Create reads it back with Get. The PUT and the PATCH on its path are not used,
// and the gcs arm of the resource's own oneOf is an unsupported arm.
func TestGoldenOutputStorage(t *testing.T) {
	out := generateStorageThing(t)
	golden := filepath.Join("testdata", "golden", "storagething")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputStorage -update:\n%s", diff)
	}
}

// The generated singleton reads itself back after an empty Create and keeps its id when that read
// fails, and it reports a response with the unsupported arm in Read and in the data source.
func TestStorageRuntimeSemantics(t *testing.T) {
	input, _ := syntheticInput(t)
	out := generateStorageThing(t)
	semantics, err := os.ReadFile(filepath.Join("testdata", "storage_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

// check passes the contract that generation accepts: both run the same model, renderer, and
// template checks. Only generation checks the SDK.
func TestCheckAcceptsTheStorageContract(t *testing.T) {
	err := Check(CheckOptions{
		Resource:      "StorageConfig",
		OpenAPIPath:   filepath.Join("testdata", "storage.yaml"),
		OverridesPath: filepath.Join("testdata", "storage-overrides.yaml"),
	})
	if err != nil {
		t.Fatal(err)
	}
}

// generateStorageThing generates the synthetic singleton like the metrics archive.
func generateStorageThing(t *testing.T) string {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "storage.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "storagething")
	options := Options{Resource: "StorageConfig", OutputDir: out, OverridesPath: filepath.Join("testdata", "storage-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	return out
}
