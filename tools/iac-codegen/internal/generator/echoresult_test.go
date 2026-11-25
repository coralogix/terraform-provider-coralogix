package generator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoldenOutputEchoResultJoin(t *testing.T) {
	input, loadDir := syntheticInput(t)
	input.OpenAPI = []byte(mustRead(t, filepath.Join("testdata", "echoresult.yaml")))
	out := filepath.Join(t.TempDir(), "echoresult")
	options := Options{Resource: "Member", OutputDir: out, OverridesPath: filepath.Join("testdata", "echoresult-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "golden", "echoresult")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("echoresult golden differs; run with -update:\n%s", diff)
	}
	compileGenerated(t, out, input)
}
