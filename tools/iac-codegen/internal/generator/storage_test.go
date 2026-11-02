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

// An empty Create response gives no id, so a singleton whose id comes from the API cannot keep
// its identity when the read after Create fails. check and generation both refuse it, also when
// the resource has the id field that singletonID.fromAPI reads.
func TestEmptyCreateNeedsAFixedSingletonID(t *testing.T) {
	dir := t.TempDir()
	data, err := os.ReadFile(filepath.Join("testdata", "storage-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	overrides := filepath.Join(dir, "behavior-overrides.yaml")
	fromAPI := replaceOnce(t, string(data), "singletonID: {fixed: storage-thing-settings}", "singletonID: {fromAPI: true}")
	if err := os.WriteFile(overrides, []byte(fromAPI), 0o644); err != nil {
		t.Fatal(err)
	}
	spec, err := os.ReadFile(filepath.Join("testdata", "storage.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	spec = []byte(replaceOnce(t, string(spec), "        prefix: {type: string}\n", "        prefix: {type: string}\n        id: {type: string, readOnly: true}\n"))
	specPath := filepath.Join(dir, "storage.yaml")
	if err := os.WriteFile(specPath, spec, 0o644); err != nil {
		t.Fatal(err)
	}
	err = Check(CheckOptions{Resource: "StorageConfig", OpenAPIPath: specPath, OverridesPath: overrides})
	if report := eligibilityReport(t, err); !hasIssue(report, "READ_BACK_SINGLETON_UNSUPPORTED", "behavior-overrides.yaml:singletonID.fromAPI") {
		t.Fatalf("check report = %v, want READ_BACK_SINGLETON_UNSUPPORTED at singletonID.fromAPI", report)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	options := Options{Resource: "StorageConfig", OutputDir: filepath.Join(dir, "storagething"), OverridesPath: overrides}
	if report := eligibilityReport(t, generateFromInput(options, input, loadDir)); !hasIssue(report, "READ_BACK_SINGLETON_UNSUPPORTED", "behavior-overrides.yaml:singletonID.fromAPI") {
		t.Fatalf("generate report = %v, want READ_BACK_SINGLETON_UNSUPPORTED at singletonID.fromAPI", report)
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
