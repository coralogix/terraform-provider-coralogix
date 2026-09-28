package source

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveUsesProviderPinnedSDK(t *testing.T) {
	root := t.TempDir()
	version := "v1.2.3"
	providerMod := "module example.com/provider\n\ngo 1.26.0\n\nrequire " + SDKModule + " " + version + "\n"
	writeTestFile(t, filepath.Join(root, "go.mod"), providerMod)
	start := filepath.Join(root, "tools", "iac-codegen")
	writeTestFile(t, filepath.Join(start, "go.mod"), "module example.com/provider/tools/iac-codegen\n\ngo 1.26.0\n")
	cache := filepath.Join(root, "module-cache")
	openAPI := []byte("openapi: 3.0.3\n")
	writeTestFile(t, filepath.Join(cache, SDKModule+"@"+version, "openapi.yaml"), string(openAPI))
	t.Setenv("GOMODCACHE", cache)

	input, err := Resolve(start)
	if err != nil {
		t.Fatal(err)
	}
	if input.ProviderRoot != root || input.SDKVersion != version || string(input.OpenAPI) != string(openAPI) {
		t.Fatalf("resolved %+v", input)
	}
}

func TestResolveDoesNotFetchMissingSDK(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/provider\n\ngo 1.26.0\n\nrequire "+SDKModule+" v1.2.3\n")
	t.Setenv("GOMODCACHE", filepath.Join(root, "empty-cache"))
	_, err := Resolve(root)
	if err == nil {
		t.Fatal("expected missing bundled OpenAPI error")
	}
}

func writeTestFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
