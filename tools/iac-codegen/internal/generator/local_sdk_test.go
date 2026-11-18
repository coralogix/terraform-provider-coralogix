package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coralogix/coralogix-management-sdk/iac/source"
)

func TestResolveLocalSDK(t *testing.T) {
	root := t.TempDir()
	provider := filepath.Join(root, "provider")
	sdk := filepath.Join(root, "sdk")
	for _, dir := range []string{provider, sdk} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(provider, "go.mod"), "module "+providerModulePath+"\n\ngo 1.24\n")
	write(filepath.Join(sdk, "go.mod"), "module "+source.SDKModule+"\n\ngo 1.24\n")
	write(filepath.Join(sdk, "openapi.yaml"), "openapi: 3.0.0\n")
	input, err := resolveLocalSDK(provider, sdk)
	if err != nil {
		t.Fatal(err)
	}
	if input.SDKDir != sdk || input.SDKVersion != "" || input.SDKModule != source.SDKModule || string(input.OpenAPI) != "openapi: 3.0.0\n" {
		t.Fatalf("unexpected local input: %+v", input)
	}
	write(filepath.Join(sdk, "go.mod"), "module example.com/wrong\n")
	if _, err := resolveLocalSDK(provider, sdk); err == nil || !strings.Contains(err.Error(), "must declare module") {
		t.Fatalf("wrong SDK module: %v", err)
	}
}

func TestGenerateWithLocalSDKAndPinnedDefault(t *testing.T) {
	input, _ := copiedSyntheticInput(t)
	for _, path := range []string{
		filepath.Join(input.SDKDir, "go.mod"),
		filepath.Join(input.ModuleRoot, "go.mod"),
		filepath.Join(input.ModuleRoot, "internal/clientset/clientset.go"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		data = []byte(strings.ReplaceAll(string(data), "example.com/iac-test-sdk", source.SDKModule))
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(input.SDKDir, "openapi.yaml"), input.OpenAPI, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(input.ModuleRoot, "internal/provider/generated/thing")
	options := Options{Resource: "Thing", OutputDir: out, SDKDir: input.SDKDir}
	if err := Generate(input.ModuleRoot, options); err != nil {
		t.Fatalf("new resource from local SDK: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "resource.go")); err != nil {
		t.Fatalf("generated resource: %v", err)
	}
	if err := os.WriteFile(filepath.Join(out, "behavior-overrides.yaml"),
		[]byte("resource: Thing\nmode: existing\nmarkdownDescription: A thing.\nschema: {version: 1}\nvalidators: {inferred: false}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Generate(input.ModuleRoot, options); err != nil {
		t.Fatalf("existing resource from local SDK: %v", err)
	}
	if err := Generate(input.ModuleRoot, Options{Resource: "Thing", OutputDir: out}); err == nil ||
		!strings.Contains(err.Error(), "pinned SDK") {
		t.Fatalf("default did not resolve the pinned SDK: %v", err)
	}
	other := filepath.Join(t.TempDir(), "sdk")
	if err := copyTree(input.SDKDir, other); err != nil {
		t.Fatal(err)
	}
	options.SDKDir = other
	if err := Generate(input.ModuleRoot, options); err == nil ||
		!strings.Contains(err.Error(), "SDK_SOURCE_MISMATCH") {
		t.Fatalf("local SDK source mismatch: %v", err)
	}
}
