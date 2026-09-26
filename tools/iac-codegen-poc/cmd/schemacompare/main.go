// Command schemacompare compares the schema dump of a handwritten resource
// with the dump of the generated schema that should replace it (D21). It
// prints each difference, breaking ones first, and exits with status 1 when
// one is breaking.
//
//	schemacompare handwritten.txt generated.txt
//
// The dumps are the text form of package schemadump. The code that writes
// them runs where both schemas compile, for example in a copy of the
// provider (integration/globalrouter).
package main

import (
	"fmt"
	"os"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/schemadump"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: schemacompare <handwritten dump> <generated dump>")
		os.Exit(2)
	}
	hw, err := read(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "schemacompare:", err)
		os.Exit(2)
	}
	gen, err := read(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, "schemacompare:", err)
		os.Exit(2)
	}
	diffs := schemadump.Compare(hw, gen)
	fmt.Print(schemadump.ReportText(diffs))
	for _, d := range diffs {
		if d.Breaking {
			os.Exit(1)
		}
	}
}

func read(path string) ([]schemadump.Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	entries, err := schemadump.Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return entries, nil
}
