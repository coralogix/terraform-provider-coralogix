package sdkspec

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

// TestPathIsPinnedVersion checks that Path uses the SDK version that go.mod pins.
func TestPathIsPinnedVersion(t *testing.T) {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Version}}", Module).Output()
	if err != nil {
		t.Fatal(err)
	}
	version := strings.TrimSpace(string(out))
	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(path, "@"+version) {
		t.Errorf("path = %q, want it to contain @%s", path, version)
	}
}

func TestRead(t *testing.T) {
	data, err := Read()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("openapi: 3.1.0")) {
		t.Errorf("spec starts with %q, want openapi: 3.1.0", data[:min(len(data), 20)])
	}
}
