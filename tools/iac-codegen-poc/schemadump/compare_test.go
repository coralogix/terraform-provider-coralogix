package schemadump_test

import (
	"context"
	"flag"
	"os"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/generated/aievaluation"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/schemadump"
)

var update = flag.Bool("update", false, "rewrite the golden files")

const (
	// handwritten is the dump of the handwritten resource. To rewrite it, run:
	// cd tools/hwschema && go run . > ../../schemadump/testdata/handwritten.txt
	handwritten = "testdata/handwritten.txt"
	generated   = "testdata/generated.golden"
	diffGolden  = "testdata/diff.golden"
)

// TestCompareWithHandwritten compares the generated schema with the
// handwritten one. diff.golden must show only the expected differences
// (README.md, "Differences from the handwritten resource").
// To rewrite the golden files, run:
// go test ./schemadump -update
func TestCompareWithHandwritten(t *testing.T) {
	gen := schemadump.Dump(aievaluation.Schema(), aievaluation.ConfigValidators())
	data, err := os.ReadFile(handwritten)
	if err != nil {
		t.Fatal(err)
	}
	hand, err := schemadump.Parse(string(data))
	if err != nil {
		t.Fatalf("%s: %v", handwritten, err)
	}
	checkGolden(t, generated, schemadump.Text(gen))
	checkGolden(t, diffGolden, schemadump.Diff(hand, gen))
}

// TestSchemaValid runs the framework checks on the generated schema, for
// example that an attribute with a Default is Computed.
func TestSchemaValid(t *testing.T) {
	if diags := aievaluation.Schema().ValidateImplementation(context.Background()); diags.HasError() {
		t.Fatal(diags)
	}
}

func TestParseText(t *testing.T) {
	entries := schemadump.Dump(aievaluation.Schema(), aievaluation.ConfigValidators())
	text := schemadump.Text(entries)
	parsed, err := schemadump.Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	if got := schemadump.Text(parsed); got != text {
		t.Errorf("Parse(Text(x)) changed the dump:\n%s", got)
	}
	if d := schemadump.Diff(entries, parsed); d != "" {
		t.Errorf("Diff of equal dumps is not empty:\n%s", d)
	}
}

func TestDiff(t *testing.T) {
	a := []schemadump.Entry{
		{Path: "gone", Lines: []string{"StringAttribute optional"}},
		{Path: "same", Lines: []string{"BoolAttribute optional"}},
		{Path: "x", Lines: []string{"StringAttribute required", "validator: v1"}},
	}
	b := []schemadump.Entry{
		{Path: "new", Lines: []string{"StringAttribute computed"}},
		{Path: "same", Lines: []string{"BoolAttribute optional"}},
		{Path: "x", Lines: []string{"StringAttribute optional", "validator: v1"}},
	}
	want := `- gone  StringAttribute optional
+ new  StringAttribute computed
~ x
    - StringAttribute required
    + StringAttribute optional
`
	if got := schemadump.Diff(a, b); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func checkGolden(t *testing.T, file, got string) {
	t.Helper()
	if *update {
		if err := os.WriteFile(file, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("%s differs. Run with -update and check the diff.\ngot:\n%s", file, got)
	}
}
