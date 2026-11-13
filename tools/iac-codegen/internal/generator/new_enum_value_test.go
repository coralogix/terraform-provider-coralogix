package generator

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"github.com/coralogix/coralogix-management-sdk/iac/source"
)

// dashboardContract renders the Dashboard resource from a contract with the committed
// behavior-overrides file of coralogix_dashboard. It returns the error text when rendering fails.
func dashboardContract(t *testing.T, spec []byte) (map[string][]byte, string) {
	t.Helper()
	input, err := source.Resolve(".")
	if err != nil {
		t.Skipf("the pinned SDK is not in the module cache: %v", err)
	}
	file, err := readOverrides(filepath.Join(input.ModuleRoot, "internal", "provider", "generated", "dashboard", "behavior-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	validated, err := validateOpenAPIWith(spec, "Dashboard", model.OperationIDs{}, "candidate.invalid/coralogix-management-sdk", providerModulePath, file)
	if err != nil {
		return nil, describeEligibility(err)
	}
	files, err := renderWith(validated.resource, validated.refs, "dashboard", validated.overrides)
	if err != nil {
		return nil, describeEligibility(err)
	}
	return files, ""
}

// editSchemas changes the component schemas of a contract.
func editSchemas(t *testing.T, spec []byte, edit func(schemas map[string]any)) []byte {
	t.Helper()
	var doc map[string]any
	if err := yaml.Unmarshal(spec, &doc); err != nil {
		t.Fatal(err)
	}
	edit(doc["components"].(map[string]any)["schemas"].(map[string]any))
	out, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func describeEligibility(err error) string {
	var eligibility *EligibilityError
	if errors.As(err, &eligibility) {
		var out []string
		for _, i := range eligibility.Report {
			out = append(out, fmt.Sprintf("%s at %s: %s", i.Code, i.Location, i.Message))
		}
		return strings.Join(out, " ;; ")
	}
	return err.Error()
}

// A value that the API adds to an enum that the dashboard already has is accepted without a line
// in behavior-overrides.yaml. The test adds SPAN_RELATION_TYPE_CHILD to the pinned contract and
// renders it with the committed file: the rendering keeps every line of the rendering of the pinned
// contract, and the new value is in the validator and the description of each attribute that
// uses the enum, and in the value map of the conversion.
func TestDashboardTakesANewEnumValueWithoutOverrides(t *testing.T) {
	input, err := source.Resolve(".")
	if err != nil {
		t.Skipf("the pinned SDK is not in the module cache: %v", err)
	}
	before, errText := dashboardContract(t, input.OpenAPI)
	if errText != "" {
		t.Fatalf("pinned contract: %s", errText)
	}
	after, errText := dashboardContract(t, editSchemas(t, input.OpenAPI, func(s map[string]any) {
		e := s["SpanRelationType"].(map[string]any)
		e["enum"] = append(e["enum"].([]any), "SPAN_RELATION_TYPE_CHILD", "SPAN_RELATION_TYPE_NOT_ANY")
	}))
	if errText != "" {
		t.Fatalf("a new enum value must not stop the generator: %s", errText)
	}
	for name, text := range after {
		for _, line := range removedLines(string(before[name]), string(text)) {
			// Only the validator and the description of the enum list its values in one line.
			if !strings.Contains(line, `stringvalidator.OneOf("other", "parent", "root", "unspecified")`) &&
				!strings.Contains(line, `Span relation type. Valid values: [\"other\" \"parent\" \"root\" \"unspecified\"].`) {
				t.Errorf("%s: a new value changed a line that was there before: %s", name, line)
			}
		}
	}
	schema := normalizeSpace(string(after["schema.go"]))
	if want := `stringvalidator.OneOf("child", "not_any", "other", "parent", "root", "unspecified")`; !strings.Contains(schema, want) {
		t.Errorf("schema.go has no validator %s", want)
	}
	if want := `Span relation type. Valid values: [\"child\" \"not_any\" \"other\" \"parent\" \"root\" \"unspecified\"].`; !strings.Contains(schema, want) {
		t.Errorf("schema.go has no description %s", want)
	}
	convert := normalizeSpace(string(after["convert.go"]))
	for _, want := range []string{
		`"child": dashboard_service.SpanRelationType("SPAN_RELATION_TYPE_CHILD"),`,
		`dashboard_service.SpanRelationType("SPAN_RELATION_TYPE_NOT_ANY"): "not_any",`,
	} {
		if !strings.Contains(convert, want) {
			t.Errorf("convert.go has no enum map entry %s", want)
		}
	}
}
