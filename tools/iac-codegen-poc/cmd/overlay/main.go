// Command overlay applies a list of set/remove entries to an OpenAPI YAML file.
//
//	go run ./cmd/overlay --overlay spec/overlay.yaml --out spec/openapi.patched.yaml
//
// Without --in, the source is openapi.yaml in the SDK module that go.mod pins.
// The output keeps every line that the overlay does not change byte for byte.
// See overlay.go for the entry format and the path syntax.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/internal/sdkspec"
)

func main() {
	in := flag.String("in", "", "source OpenAPI YAML file (default: openapi.yaml in the pinned SDK module)")
	overlayPath := flag.String("overlay", "", "overlay YAML file")
	out := flag.String("out", "", "output OpenAPI YAML file")
	flag.Parse()

	if err := run(*in, *overlayPath, *out); err != nil {
		fmt.Fprintln(os.Stderr, "overlay:", err)
		os.Exit(1)
	}
}

func run(in, overlayPath, out string) error {
	if overlayPath == "" || out == "" {
		return fmt.Errorf("--overlay and --out are required")
	}
	src, err := readSource(in)
	if err != nil {
		return err
	}
	overlay, err := os.ReadFile(overlayPath)
	if err != nil {
		return err
	}
	entries, err := parseEntries(overlay)
	if err != nil {
		return fmt.Errorf("%s: %w", overlayPath, err)
	}
	patched, err := apply(src, entries)
	if err != nil {
		return fmt.Errorf("%s: %w", overlayPath, err)
	}
	return os.WriteFile(out, patched, 0o644)
}

// readSource reads in, or the pinned SDK spec when in is empty.
func readSource(in string) ([]byte, error) {
	if in == "" {
		return sdkspec.Read()
	}
	return os.ReadFile(in)
}
