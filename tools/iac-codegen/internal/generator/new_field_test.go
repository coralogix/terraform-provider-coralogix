package generator

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"github.com/coralogix/coralogix-management-sdk/iac/source"
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
	overridesPath := filepath.Join(input.ModuleRoot, "internal", "provider", "generated", "alertsscheduler", "behavior-overrides.yaml")
	file, err := readOverrides(overridesPath)
	if err != nil {
		t.Fatal(err)
	}
	render := func(spec []byte) map[string][]byte {
		t.Helper()
		validated, err := validateOpenAPIWith(spec, "AlertSchedulerRule", model.OperationIDs{}, input.SDKModule, input.Module, file)
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

// A Terraform-only nested URL does not hide later fields of the API object. The new field still
// reaches schema, model, expand, and flatten without a behavior-overrides line or hook change.
func TestNestedExtraKeepsLaterAPIFieldAutomatic(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "sharedthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, _ := syntheticInput(t)
	file, err := readOverrides(filepath.Join("testdata", "rootunsupported-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := file.Types["SharedCount"].Fields["probeTag"]; ok {
		t.Fatal("the new API field must have no override")
	}
	render := func(contract []byte) map[string][]byte {
		t.Helper()
		validated, err := validateOpenAPIWith(contract, "SharedThing", model.OperationIDs{}, input.SDKModule, input.Module, file)
		if err != nil {
			t.Fatal(err)
		}
		files, err := renderWith(validated.resource, validated.refs, "sharedthing", file)
		if err != nil {
			t.Fatal(err)
		}
		return files
	}
	before := render(spec)
	var doc map[string]any
	if err := yaml.Unmarshal(spec, &doc); err != nil {
		t.Fatal(err)
	}
	schemas := doc["components"].(map[string]any)["schemas"].(map[string]any)
	fields := schemas["SharedCount"].(map[string]any)["properties"].(map[string]any)
	fields["probeTag"] = map[string]any{"type": "string", "x-coralogix-presence": true}
	contract, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	after := render(contract)
	for _, want := range []struct{ file, text string }{
		{"schema.go", `"url": schema.StringAttribute{`},
		{"schema.go", `"probe_tag": schema.StringAttribute{`},
		{"model.go", "Url types.String"},
		{"model.go", "ProbeTag types.String"},
		{"convert.go", "out.ProbeTag = expandString(m.ProbeTag)"},
		{"convert.go", "out.ProbeTag = types.StringPointerValue(v.ProbeTag)"},
	} {
		if !strings.Contains(normalizeSpace(string(after[want.file])), normalizeSpace(want.text)) {
			t.Errorf("%s has no %q", want.file, want.text)
		}
	}
	for name, text := range after {
		if removed := removedLines(string(before[name]), string(text)); len(removed) != 0 {
			t.Errorf("%s: the new field removed generated lines:\n%s", name, strings.Join(removed, "\n"))
		}
	}
}

// A later ordinary webhook field stays automatic beside the Terraform-only URL and secret fields.
func TestWebhookTakesLaterAPIFieldWithoutOverride(t *testing.T) {
	input, err := source.Resolve(".")
	if err != nil {
		t.Fatal(err)
	}
	file, err := readOverrides(filepath.Join(input.ModuleRoot, "internal", "provider", "generated", "webhook", "behavior-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := file.Types["OutgoingWebhook"].Fields["probeNote"]; ok {
		t.Fatal("probeNote must have no override")
	}
	render := func(spec []byte) map[string][]byte {
		t.Helper()
		validated, err := validateOpenAPIWith(spec, "OutgoingWebhook", model.OperationIDs{}, input.SDKModule, input.Module, file)
		if err != nil {
			t.Fatal(err)
		}
		files, err := renderWith(validated.resource, validated.refs, "webhook", file)
		if err != nil {
			t.Fatal(err)
		}
		return files
	}
	before := render(input.OpenAPI)
	var doc map[string]any
	if err := yaml.Unmarshal(input.OpenAPI, &doc); err != nil {
		t.Fatal(err)
	}
	schemas := doc["components"].(map[string]any)["schemas"].(map[string]any)
	for _, component := range []string{"OutgoingWebhook", "OutgoingWebhookInputData"} {
		properties := schemas[component].(map[string]any)["properties"].(map[string]any)
		properties["probeNote"] = map[string]any{"type": "string", "x-coralogix-presence": true, "description": "A future note."}
	}
	contract, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	after := render(contract)
	for _, want := range []struct{ file, text string }{
		{"schema.go", `"probe_note": schema.StringAttribute{`},
		{"model.go", "ProbeNote types.String"},
		{"convert.go", "out.ProbeNote = expandString(m.ProbeNote)"},
		{"convert.go", "out.ProbeNote = types.StringPointerValue(v.ProbeNote)"},
	} {
		if !strings.Contains(normalizeSpace(string(after[want.file])), normalizeSpace(want.text)) {
			t.Errorf("%s has no %q", want.file, want.text)
		}
	}
	for name, text := range after {
		if removed := removedLines(string(before[name]), string(text)); len(removed) != 0 {
			t.Errorf("%s: the new field removed generated lines:\n%s", name, strings.Join(removed, "\n"))
		}
	}
}

// A later webhook config arm gets a schema field and both conversions without an override.
// Existing arms still send type for request parity; the API infers type for the new arm.
func TestWebhookTakesLaterConfigArmWithoutOverride(t *testing.T) {
	input, err := source.Resolve(".")
	if err != nil {
		t.Fatal(err)
	}
	file, err := readOverrides(filepath.Join(input.ModuleRoot, "internal", "provider", "generated", "webhook", "behavior-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := file.Types["OutgoingWebhook"].Fields["discord"]; ok {
		t.Fatal("discord must have no override")
	}
	var doc map[string]any
	if err := yaml.Unmarshal(input.OpenAPI, &doc); err != nil {
		t.Fatal(err)
	}
	schemas := doc["components"].(map[string]any)["schemas"].(map[string]any)
	schemas["DiscordConfig"] = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"channel": map[string]any{"type": "string", "x-coralogix-presence": true},
		},
	}
	for _, component := range []string{"OutgoingWebhook", "OutgoingWebhookInputData"} {
		resource := schemas[component].(map[string]any)
		resource["properties"].(map[string]any)["discord"] = map[string]any{"$ref": "#/components/schemas/DiscordConfig"}
		arm := map[string]any{"required": []any{"discord"}, "type": "object"}
		oneOf := resource["oneOf"].([]any)
		resource["oneOf"] = append(oneOf, arm)
	}
	contract, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	validated, err := validateOpenAPIWith(contract, "OutgoingWebhook", model.OperationIDs{}, input.SDKModule, input.Module, file)
	if err != nil {
		t.Fatal(err)
	}
	files, err := renderWith(validated.resource, validated.refs, "webhook", file)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct{ file, text string }{
		{"schema.go", `"discord": schema.SingleNestedAttribute{`},
		{"schema.go", `"channel": schema.StringAttribute{`},
		{"model.go", "Discord *DiscordConfigModel"},
		{"model.go", "Channel types.String"},
		{"convert.go", `out.Discord = expandDiscordConfig(ctx, p.AtName("discord"), m.Discord, diags)`},
		{"convert.go", `out.Discord = flattenDiscordConfig(ctx, p.AtName("discord"), v.Discord, diags)`},
	} {
		if !strings.Contains(normalizeSpace(string(files[want.file])), normalizeSpace(want.text)) {
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
