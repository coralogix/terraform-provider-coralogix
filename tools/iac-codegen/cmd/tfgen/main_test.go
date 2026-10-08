package main

import (
	"path/filepath"
	"testing"
)

func TestCheckAcceptsLocalCandidate(t *testing.T) {
	openAPI := filepath.Join("..", "..", "internal", "generator", "testdata", "openapi.yaml")
	if status := run([]string{"check", "--resource", "Thing", "--openapi", openAPI}); status != 0 {
		t.Fatalf("status = %d, want 0", status)
	}
}

func TestGenerateDoesNotAcceptOpenAPIInput(t *testing.T) {
	if status := run([]string{"generate", "--resource", "Thing", "--out", t.TempDir(), "--openapi", "missing.yaml"}); status != 2 {
		t.Fatalf("status = %d, want 2", status)
	}
}

func TestCheckDoesNotAcceptOutput(t *testing.T) {
	if status := run([]string{"check", "--resource", "Thing", "--openapi", "candidate.yaml", "--out", "generated"}); status != 2 {
		t.Fatalf("status = %d, want 2", status)
	}
}
