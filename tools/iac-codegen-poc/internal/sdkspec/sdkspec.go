// Package sdkspec finds the source OpenAPI spec: the openapi.yaml file at the
// root of the SDK module that go.mod pins. So the spec always matches the SDK
// that the generated code uses.
package sdkspec

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Module is the SDK module. Its root has openapi.yaml.
const Module = "github.com/coralogix/coralogix-management-sdk"

// Path returns the path of openapi.yaml in the pinned SDK module. The module
// must be in the module cache (go mod download).
func Path() (string, error) {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", Module).Output()
	if err != nil {
		return "", fmt.Errorf("go list -m %s: %w", Module, err)
	}
	dir := strings.TrimSpace(string(out))
	if dir == "" {
		return "", fmt.Errorf("module %s is not in the module cache; run go mod download", Module)
	}
	path := filepath.Join(dir, "openapi.yaml")
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	return path, nil
}

// Read returns the content of openapi.yaml in the pinned SDK module.
func Read() ([]byte, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}
