package generator

import (
	"errors"
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

// suffixedLines puts lines of fields on the Thing component of enumValuesLines.
func suffixedLines(t *testing.T, fields string) string {
	t.Helper()
	return replaceOnce(t, enumValuesLines, "    fields:\n", "    fields:\n"+fields)
}

// The suffix follows the description of the contract, after one space, and the list of valid values
// when enumValuesInDescription adds one.
func TestDescriptionSuffixFollowsTheContractText(t *testing.T) {
	lines := suffixedLines(t, `      name: {descriptionSuffix: "Terraform adds this."}
      priority: {descriptionSuffix: "Set it with care."}
`)
	got := enumValuesDescriptions(t, "enumValuesInDescription: true", lines)
	want := map[string]string{
		"name":     "The name. Terraform adds this.",
		"priority": `The priority. Valid values: ["P1" "P5" "unspecified"]. Set it with care.`,
		// Lines that the test does not suffix stay as the contract and the list of values give them.
		"tier": `The tier. Valid values: ["gold" "silver"].`,
	}
	for name, text := range want {
		if got[name] != text {
			t.Errorf("description of %s = %q, want %q", name, got[name], text)
		}
	}
}

// Without enumValuesInDescription, the suffix follows the contract text alone.
func TestDescriptionSuffixWithoutEnumValues(t *testing.T) {
	got := enumValuesDescriptions(t, "", suffixedLines(t, "      priority: {descriptionSuffix: \"Set it with care.\"}\n"))
	if want := "The priority. Set it with care."; got["priority"] != want {
		t.Errorf("description of priority = %q, want %q", got["priority"], want)
	}
}

// A field whose contract has no description has nothing to add to. The key is no replacement for
// description or markdownDescription.
func TestDescriptionSuffixNeedsAContractDescription(t *testing.T) {
	lines := suffixedLines(t, "      bare: {descriptionSuffix: \"Terraform adds this.\"}\n")
	_, err := buildTFResourceWith(enumValuesResource(), "thing", mustParse(t, lines))
	if !errors.Is(err, errDescriptionSuffix) || !strings.Contains(err.Error(), "no description") {
		t.Fatalf("err = %v, want the descriptionSuffix error about a missing contract description", err)
	}
	if got := shapeIssue("renderer.schema", err); got.Code != "OVERRIDE_INVALID" {
		t.Errorf("code = %s, want OVERRIDE_INVALID", got.Code)
	}
}

// A contract that already says the text makes the line stale: OVERRIDE_UNUSED, like another key
// that the generator or the contract already states.
func TestDescriptionSuffixStaleWhenTheContractSaysIt(t *testing.T) {
	lines := suffixedLines(t, "      name: {descriptionSuffix: \"The name.\"}\n")
	_, err := buildTFResourceWith(enumValuesResource(), "thing", mustParse(t, lines))
	if !errors.Is(err, errStaleOverrideKey) {
		t.Fatalf("err = %v, want a stale key error", err)
	}
	if got := shapeIssue("renderer.schema", err); got.Code != "OVERRIDE_UNUSED" {
		t.Errorf("code = %s, want OVERRIDE_UNUSED", got.Code)
	}
}

// A place line is laid over the line of the component. When the line of the component has a
// suffix and the place line states the whole text, the combined line sets both.
func TestDescriptionSuffixRefusesALaidOverDescription(t *testing.T) {
	text, suffix := "The whole text.", "Terraform adds this."
	err := applyDescriptionSuffix(&tfAttr{Description: "From the contract.", Component: "Thing", Property: "name"}, overrides.Field{DescriptionSuffix: &suffix, MarkdownDescription: &text})
	if !errors.Is(err, errDescriptionSuffix) {
		t.Fatalf("err = %v, want the descriptionSuffix error", err)
	}
}

// The line itself must be usable: not empty, not padded with white space, not with a description
// or markdownDescription, and on a field that has an attribute.
func TestDescriptionSuffixLineChecks(t *testing.T) {
	tests := map[string]struct {
		line string
		want string
	}{
		"empty":                 {`{descriptionSuffix: ""}`, "is empty"},
		"blank":                 {`{descriptionSuffix: " "}`, "is empty"},
		"leading space":         {`{descriptionSuffix: " Text."}`, "white space"},
		"trailing space":        {`{descriptionSuffix: "Text. "}`, "white space"},
		"with description":      {`{descriptionSuffix: "Text.", description: "Whole."}`, "set one of them"},
		"with markdown":         {`{descriptionSuffix: "Text.", markdownDescription: "Whole."}`, "set one of them"},
		"on an inline field":    {`{descriptionSuffix: "Text.", inline: true}`, "no attribute of its own"},
		"on a skipped field":    {`{descriptionSuffix: "Text.", skip: true}`, "has none"},
		"on a requestValue one": {`{descriptionSuffix: "Text.", requestValue: ALPHA}`, "has none"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := overrides.Parse([]byte("resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  Thing:\n    fields:\n      name: " + test.line + "\n"))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want one with %q", err, test.want)
			}
		})
	}
	if _, err := overrides.Parse([]byte("resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  Thing:\n    fields:\n      name: {descriptionSuffix: \"Text.\"}\n")); err != nil {
		t.Fatalf("a plain line: %v", err)
	}
}
