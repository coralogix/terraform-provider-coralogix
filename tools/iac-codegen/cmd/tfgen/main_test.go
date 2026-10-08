package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckAcceptsLocalCandidate(t *testing.T) {
	openAPI := filepath.Join("..", "..", "internal", "generator", "testdata", "openapi.yaml")
	if status := run([]string{"check", "--resource", "Thing", "--openapi", openAPI}); status != 0 {
		t.Fatalf("status = %d, want 0", status)
	}
}

func TestGenerateAcceptsOpenAPIInput(t *testing.T) {
	out := filepath.Join(t.TempDir(), "preset")
	stderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	status := run([]string{"generate", "--resource", "Thing", "--out", out, "--openapi", "missing.yaml"})
	w.Close()
	os.Stderr = stderr
	got, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if status != 1 {
		t.Fatalf("status = %d, want 1\n%s", status, got)
	}
	if !strings.Contains(string(got), "read candidate OpenAPI missing.yaml") {
		t.Fatalf("stderr = %q, want the missing candidate file", got)
	}
}

func TestCheckDoesNotAcceptOutput(t *testing.T) {
	if status := run([]string{"check", "--resource", "Thing", "--openapi", "candidate.yaml", "--out", "generated"}); status != 2 {
		t.Fatalf("status = %d, want 2", status)
	}
}
