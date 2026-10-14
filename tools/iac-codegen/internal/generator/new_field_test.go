package generator

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/model"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/source"
)

// A field that a later contract adds to a released resource flows from the contract into the
// schema, the model, expand, and flatten without a behavior-overrides line or handwritten code.
// The test adds a top-level and a nested field to the pinned contract of the alerts scheduler and
// renders it with the committed behavior-overrides file: the rendering keeps every line of the
// rendering of the pinned contract, and adds the attribute, the model field, expand, and flatten.
func TestReleasedResourceTakesANewFieldWithoutOverrides(t *testing.T) {
	input, err := source.Resolve(".")
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal(err)
		}
		t.Skipf("the pinned SDK is not in the module cache: %v", err)
	}
	overridesPath := filepath.Join(input.ProviderRoot, "internal", "provider", "generated", "alertsscheduler", "behavior-overrides.yaml")
	file, err := readOverrides(overridesPath)
	if err != nil {
		t.Fatal(err)
	}
	render := func(spec []byte) map[string][]byte {
		t.Helper()
		validated, err := validateOpenAPIWith(spec, "AlertSchedulerRule", model.OperationIDs{}, input.SDKModule, input.ProviderModule, file)
		if err != nil {
			t.Fatal(err)
		}
		files, err := renderWith(validated.resource, validated.refs, "alertsscheduler", file)
		if err != nil {
			t.Fatal(err)
		}
		return files
	}
	before := render(input.OpenAPI)
	after := render(withNewFields(t, input.OpenAPI))

	for name, text := range after {
		if removed := removedLines(string(before[name]), string(text)); len(removed) != 0 {
			t.Errorf("%s: a new field changed lines that were there before:\n%s", name, strings.Join(removed, "\n"))
		}
	}
	for _, want := range []struct{ file, text string }{
		{"schema.go", `"probe_note": schema.StringAttribute{`},
		{"schema.go", `"probe_limit": schema.Int32Attribute{`},
		{"model.go", "ProbeNote types.String"},
		{"model.go", "ProbeLimit types.Int32"},
		{"convert.go", "out.ProbeNote = expandString(m.ProbeNote)"},
		{"convert.go", "out.ProbeNote = types.StringPointerValue(v.ProbeNote)"},
		{"convert.go", "out.ProbeLimit = expandInt32(m.ProbeLimit)"},
		{"convert.go", "out.ProbeLimit = types.Int32PointerValue(v.ProbeLimit)"},
	} {
		if !strings.Contains(normalizeSpace(string(after[want.file])), normalizeSpace(want.text)) {
			t.Errorf("%s has no %q", want.file, want.text)
		}
	}
}

// withNewFields adds the optional fields probeNote to the rule and probeLimit to its time frame.
func withNewFields(t *testing.T, spec []byte) []byte {
	t.Helper()
	var doc map[string]any
	if err := yaml.Unmarshal(spec, &doc); err != nil {
		t.Fatal(err)
	}
	schemas := doc["components"].(map[string]any)["schemas"].(map[string]any)
	add := func(component, name string, property map[string]any) {
		props := schemas[component].(map[string]any)["properties"].(map[string]any)
		props[name] = property
	}
	add("AlertSchedulerRule", "probeNote", map[string]any{"type": "string", "x-coralogix-presence": true, "description": "A note."})
	add("Timeframe", "probeLimit", map[string]any{"type": "integer", "format": "int32", "x-coralogix-presence": true, "description": "A limit."})
	out, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// removedLines returns the lines of before that after does not have. Space is normalized, because
// gofmt aligns the lines next to a new field again.
func removedLines(before, after string) []string {
	count := map[string]int{}
	for _, line := range strings.Split(before, "\n") {
		count[normalizeSpace(line)]++
	}
	for _, line := range strings.Split(after, "\n") {
		count[normalizeSpace(line)]--
	}
	var removed []string
	for line, n := range count {
		for range n {
			removed = append(removed, line)
		}
	}
	slices.Sort(removed)
	return removed
}

func normalizeSpace(s string) string { return strings.Join(strings.Fields(s), " ") }
