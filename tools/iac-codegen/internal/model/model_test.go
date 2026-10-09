package model

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/issue"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

func TestClassifyFieldLifecycle(t *testing.T) {
	tests := []struct {
		create bool
		update bool
		get    bool
		want   Behavior
		valid  bool
	}{
		{true, true, true, Normal, true},
		{true, false, true, Immutable, true},
		{false, false, true, Computed, true},
		{false, true, true, "", false},
		{true, true, false, "", false},
		{true, false, false, "", false},
	}
	for _, test := range tests {
		got, err := Classify(test.create, test.update, test.get)
		if (err == nil) != test.valid || got != test.want {
			t.Errorf("Classify(%t, %t, %t) = %q, %v; want %q, valid=%t", test.create, test.update, test.get, got, err, test.want, test.valid)
		}
	}
}

func TestResolveResourceName(t *testing.T) {
	doc, err := Load(validSpec(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, selection := range []string{"Thing", "thing"} {
		name, report := ResolveResource(doc, selection)
		if len(report) != 0 || name != "Thing" {
			t.Errorf("ResolveResource(%q) = %q, %v", selection, name, report)
		}
	}
}

func TestTerraformNameKeepsAcronymsTogether(t *testing.T) {
	for input, want := range map[string]string{
		"HTTPServer":  "http_server",
		"httpServer":  "http_server",
		"sqlReadOnly": "sql_read_only",
		"foo-bar":     "foo_bar",
		"foo.bar":     "foo_bar",
		"1st":         "_1st",
		"_private":    "_private",
		"café":        "caf_",
	} {
		if got := TerraformName(input); got != want {
			t.Errorf("TerraformName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSeparatorTerraformNameCollisionIsIneligible(t *testing.T) {
	doc := loadComponent(t, `
    Collision:
      type: object
      properties:
        foo-bar: {type: string}
        foo_bar: {type: string}
`)
	typeValue, problems := Survey(doc, "Collision")
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	if codes := reportCodes(nameCollisions("components.schemas.Collision", typeValue)); !slices.Contains(codes, "TERRAFORM_NAME_COLLISION") {
		t.Fatalf("codes %v do not contain TERRAFORM_NAME_COLLISION", codes)
	}
}

func TestAcronymTerraformNameCollisionIsIneligible(t *testing.T) {
	doc := loadComponent(t, `
    Collision:
      type: object
      properties:
        HTTPServer: {type: string}
        httpServer: {type: string}
`)
	typeValue, problems := Survey(doc, "Collision")
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	if codes := reportCodes(nameCollisions("components.schemas.Collision", typeValue)); !slices.Contains(codes, "TERRAFORM_NAME_COLLISION") {
		t.Fatalf("codes %v do not contain TERRAFORM_NAME_COLLISION", codes)
	}
}

func TestGoNameCollisionIsIneligible(t *testing.T) {
	doc := loadComponent(t, `
    Collision:
      type: object
      properties:
        foo-bar: {type: string}
        foo_bar: {type: string}
`)
	typeValue, problems := Survey(doc, "Collision")
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	if codes := reportCodes(nameCollisions("components.schemas.Collision", typeValue)); !slices.Contains(codes, "GO_NAME_COLLISION") {
		t.Fatalf("codes %v do not contain GO_NAME_COLLISION", codes)
	}
	if GoName("foo-bar") != "FooBar" || GoName("foo_bar") != "FooBar" {
		t.Fatal("GoName does not match generated field naming")
	}
}

func TestComponentGoNameCollisionIsIneligible(t *testing.T) {
	doc := loadComponent(t, `
    Root:
      type: object
      properties:
        first: {$ref: '#/components/schemas/FooBar'}
        second: {$ref: '#/components/schemas/fooBar'}
    FooBar:
      type: object
      properties:
        value: {type: string}
    fooBar:
      type: object
      properties:
        value: {type: string}
`)
	typeValue, problems := Survey(doc, "Root")
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	if codes := reportCodes(componentNameCollisions(typeValue)); !slices.Contains(codes, "GO_COMPONENT_NAME_COLLISION") {
		t.Fatalf("codes %v do not contain GO_COMPONENT_NAME_COLLISION", codes)
	}
}

func TestUint64RequiresTerraformInt64Bound(t *testing.T) {
	tests := map[string]struct {
		maxLength string
		eligible  bool
	}{
		"missing bound": {eligible: false},
		"18 digits":     {maxLength: "      maxLength: 18\n", eligible: true},
		"19 digits":     {maxLength: "      maxLength: 19\n", eligible: false},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			doc := loadComponent(t, fmt.Sprintf(`
    Counter:
      type: string
      format: uint64
%s`, test.maxLength))
			_, problems := Survey(doc, "Counter")
			if (len(problems) == 0) != test.eligible {
				t.Fatalf("problems = %v, eligible = %t", problems, test.eligible)
			}
		})
	}
}

func TestPresenceContract(t *testing.T) {
	data := validSpec(t)
	doc, err := Load(data)
	if err != nil {
		t.Fatal(err)
	}
	if report := Validate(doc, "Thing", OperationIDs{}); len(report) != 0 {
		t.Fatal(report)
	}
	data = []byte(strings.Replace(string(data),
		"                description:\n                  type: string\n                  x-coralogix-presence: true\n",
		"                description:\n                  type: string\n", 1))
	doc, err = Load(data)
	if err != nil {
		t.Fatal(err)
	}
	if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "FIELD_PRESENCE_UNKNOWN") {
		t.Fatalf("codes %v do not contain FIELD_PRESENCE_UNKNOWN", codes)
	}
}

func TestRequiredScalarPresenceContract(t *testing.T) {
	spec := strings.Replace(string(validSpec(t)),
		"        endpoint:\n          type: string\n          minLength: 1\n",
		"        endpoint:\n          type: integer\n", 1)
	doc, err := Load([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "REQUIRED_SCALAR_PRESENCE_UNKNOWN") {
		t.Fatalf("codes %v do not contain REQUIRED_SCALAR_PRESENCE_UNKNOWN", codes)
	}
}

func TestRequiredValueCannotBeEmpty(t *testing.T) {
	tests := map[string]struct {
		old string
		new string
	}{
		"string": {
			old: "        endpoint:\n          type: string\n          minLength: 1\n",
			new: "        endpoint:\n          type: string\n",
		},
		"string with presence": {
			old: "        endpoint:\n          type: string\n          minLength: 1\n",
			new: "        endpoint:\n          type: string\n          x-coralogix-presence: true\n",
		},
		"list": {
			old: "        endpoint:\n          type: string\n          minLength: 1\n",
			new: "        endpoint:\n          type: array\n          items: {type: string}\n",
		},
		"map": {
			old: "        endpoint:\n          type: string\n          minLength: 1\n",
			new: "        endpoint:\n          type: object\n          additionalProperties: {type: string}\n",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			spec := strings.Replace(string(validSpec(t)), test.old, test.new, 1)
			doc, err := Load([]byte(spec))
			if err != nil {
				t.Fatal(err)
			}
			if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "REQUIRED_VALUE_CAN_BE_EMPTY") {
				t.Fatalf("codes %v do not contain REQUIRED_VALUE_CAN_BE_EMPTY", codes)
			}
		})
	}
}

func TestRequiredValueWithMinimumIsEligible(t *testing.T) {
	tests := map[string]string{
		"list": "        endpoint:\n          type: array\n          minItems: 1\n          items: {type: string}\n",
		"map":  "        endpoint:\n          type: object\n          minProperties: 1\n          maxProperties: 5\n          additionalProperties: {type: string}\n",
	}
	for name, field := range tests {
		t.Run(name, func(t *testing.T) {
			spec := strings.Replace(string(validSpec(t)), "        endpoint:\n          type: string\n          minLength: 1\n", field, 1)
			doc, err := Load([]byte(spec))
			if err != nil {
				t.Fatal(err)
			}
			if report := Validate(doc, "Thing", OperationIDs{}); len(report) != 0 {
				t.Fatalf("report = %v, want none", report)
			}
		})
	}
}

func TestPropertyCountOnlyOnMaps(t *testing.T) {
	spec := strings.Replace(string(validSpec(t)),
		"    HttpThingConfig:\n      type: object\n",
		"    HttpThingConfig:\n      type: object\n      minProperties: 1\n", 1)
	doc, err := Load([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "OBJECT_PROPERTY_COUNT_UNSUPPORTED") {
		t.Fatalf("codes %v do not contain OBJECT_PROPERTY_COUNT_UNSUPPORTED", codes)
	}
}

// TestCollectionsAndObjectsNeedNoPresence covers the real OpenAPI generator
// output: it writes x-coralogix-presence only on scalars and enums.
func TestCollectionsAndObjectsNeedNoPresence(t *testing.T) {
	spec := string(validSpec(t))
	if strings.Contains(spec, "type: array\n                  x-coralogix-presence") {
		t.Fatal("the synthetic spec must not mark collections with presence")
	}
	doc, err := Load([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	if report := Validate(doc, "Thing", OperationIDs{}); len(report) != 0 {
		t.Fatalf("report = %v, want none", report)
	}
}

func TestMissingRequiredListMeansNoneRequired(t *testing.T) {
	tests := map[string]struct {
		old string
		new string
	}{
		"request root": {
			old: "              title: UpdateThingRequest\n              type: object\n              required: []\n              properties:\n",
			new: "              title: UpdateThingRequest\n              type: object\n              properties:\n",
		},
		"nested object": {
			old: "    ThingConfig:\n      type: object\n      required: []\n      properties:\n",
			new: "    ThingConfig:\n      type: object\n      properties:\n",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			spec := strings.Replace(string(validSpec(t)), test.old, test.new, 1)
			doc, err := Load([]byte(spec))
			if err != nil {
				t.Fatal(err)
			}
			// The OpenAPI fork cannot write an empty list, so a missing list
			// means that no field is required.
			if report := Validate(doc, "Thing", OperationIDs{}); len(report) != 0 {
				t.Fatalf("a missing required list is ineligible: %v", report)
			}
		})
	}
}

func TestRequestSchemasMustBeSeparate(t *testing.T) {
	spec := replaceOperationBodySchema(t, string(validSpec(t)), "ThingsService_CreateThing", "              $ref: '#/components/schemas/Thing'\n")
	doc, err := Load([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "REQUEST_SCHEMA_REUSED") {
		t.Fatalf("codes %v do not contain REQUEST_SCHEMA_REUSED", codes)
	}
}

func TestEnumZeroIsExact(t *testing.T) {
	doc, err := Load(validSpec(t))
	if err != nil {
		t.Fatal(err)
	}
	resource, err := Build(doc, "Thing")
	if err != nil {
		t.Fatal(err)
	}
	index := slices.IndexFunc(resource.Fields, func(field *ResourceField) bool { return field.Name == "kind" })
	if index < 0 {
		t.Fatal("kind field not found")
	}
	typeValue := resource.Fields[index].Type
	if typeValue.EnumZero != "THING_KIND_UNSPECIFIED" || !slices.Contains(typeValue.Values, "THING_KIND_P5_OR_UNSPECIFIED") {
		t.Fatalf("enum = zero %q, values %v", typeValue.EnumZero, typeValue.Values)
	}

	spec := strings.Replace(string(validSpec(t)), "THING_KIND_UNSPECIFIED", "THING_KIND_NOT_SET", 1)
	doc, err = Load([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	_, problems := Survey(doc, "Thing")
	if len(problems) == 0 || !strings.Contains(fmt.Sprint(problems), "must be <PREFIX>_UNSPECIFIED") {
		t.Fatalf("problems = %v, want exact enum zero error", problems)
	}
}

func TestUpdateMaskPatternMustAcceptSeveralPaths(t *testing.T) {
	spec := strings.Replace(string(validSpec(t)),
		"pattern: '^[a-z][A-Za-z0-9]*(\\.[a-z][A-Za-z0-9]*)*(,[a-z][A-Za-z0-9]*(\\.[a-z][A-Za-z0-9]*)*)*$'",
		"pattern: '^[a-z][A-Za-z0-9]*(\\.[a-z][A-Za-z0-9]*)*$'", 1)
	doc, err := Load([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "UPDATE_MASK_CONTRACT_INVALID") {
		t.Fatalf("codes %v do not contain UPDATE_MASK_CONTRACT_INVALID", codes)
	}
}

func TestNestedOneOfNeedsDottedUpdateMask(t *testing.T) {
	spec := strings.Replace(string(validSpec(t)),
		"pattern: '^[a-z][A-Za-z0-9]*(\\.[a-z][A-Za-z0-9]*)*(,[a-z][A-Za-z0-9]*(\\.[a-z][A-Za-z0-9]*)*)*$'",
		"pattern: '^[a-z][A-Za-z0-9]*(,[a-z][A-Za-z0-9]*)*$'", 1)
	doc, err := Load([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "UPDATE_MASK_NESTED_ONEOF_UNSUPPORTED") {
		t.Fatalf("codes %v do not contain UPDATE_MASK_NESTED_ONEOF_UNSUPPORTED", codes)
	}
}

func TestNestedPresenceContract(t *testing.T) {
	doc := loadComponent(t, `
    Nested:
      type: object
      properties:
        enabled:
          type: boolean
`)
	proxy := doc.Components.Schemas.GetOrZero("Nested")
	if codes := reportCodes(nestedPresenceIssues(Policy{}, "request.config", proxy, false, map[presenceVisit]bool{})); !slices.Contains(codes, "FIELD_PRESENCE_UNKNOWN") {
		t.Fatalf("codes %v do not contain FIELD_PRESENCE_UNKNOWN", codes)
	}

	doc = loadComponent(t, `
    Nested:
      type: object
      properties:
        enabled:
          type: boolean
          x-coralogix-presence: true
`)
	proxy = doc.Components.Schemas.GetOrZero("Nested")
	if report := nestedPresenceIssues(Policy{}, "request.config", proxy, false, map[presenceVisit]bool{}); len(report) != 0 {
		t.Fatal(report)
	}

	doc = loadComponent(t, `
    Choice:
      type: object
      properties:
        a: {type: string}
        b: {type: string}
      oneOf:
        - required: [a]
        - required: [b]
`)
	proxy = doc.Components.Schemas.GetOrZero("Choice")
	if report := nestedPresenceIssues(Policy{}, "request.choice", proxy, false, map[presenceVisit]bool{}); len(report) != 0 {
		t.Fatalf("oneOf arms use their union presence and need no annotation: %v", report)
	}
}

func TestRequiredLifecycleParametersAreIneligible(t *testing.T) {
	spec := strings.Replace(string(validSpec(t)),
		"      operationId: ThingsService_CreateThing\n",
		"      operationId: ThingsService_CreateThing\n      parameters:\n        - name: tenantId\n          in: query\n          required: true\n          schema: {type: string}\n        - name: If-Match\n          in: header\n          required: true\n          schema: {type: string}\n", 1)
	doc, err := Load([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	report := Validate(doc, "Thing", OperationIDs{})
	count := 0
	for _, item := range report {
		if item.Code == "REQUIRED_PARAMETER_UNSUPPORTED" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("got %d required-parameter issues, want 2: %v", count, report)
	}
	if _, err := Build(doc, "Thing"); err == nil || !strings.Contains(err.Error(), "required query parameter \"tenantId\"") {
		t.Fatalf("Build error = %v, want unsupported required parameter", err)
	}
}

func TestServerDefaultContract(t *testing.T) {
	base := string(validSpec(t))
	defaultBlock := "                enabled:\n                  type: boolean\n                  default: true\n                  x-coralogix-presence: true"
	noDefaultBlock := "                enabled:\n                  type: boolean\n                  x-coralogix-presence: true"
	undeclared := strings.ReplaceAll(base, defaultBlock, noDefaultBlock)
	doc, err := Load([]byte(undeclared))
	if err != nil {
		t.Fatal(err)
	}
	if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "FIELD_SERVER_DEFAULT_UNDECLARED") {
		t.Fatalf("codes %v do not contain FIELD_SERVER_DEFAULT_UNDECLARED", codes)
	}

	doc, err = Load([]byte(base))
	if err != nil {
		t.Fatal(err)
	}
	if report := Validate(doc, "Thing", OperationIDs{}); len(report) != 0 {
		t.Fatal(report)
	}

	// The first block is the Create field and the last block is the Update field.
	updateBlock := strings.LastIndex(base, defaultBlock)
	withUpdate := func(block string) string {
		return base[:updateBlock] + block + base[updateBlock+len(defaultBlock):]
	}
	doc, err = Load([]byte(withUpdate(noDefaultBlock)))
	if err != nil {
		t.Fatal(err)
	}
	if report := Validate(doc, "Thing", OperationIDs{}); len(report) != 0 {
		t.Fatalf("Update without the Create default: %v", report)
	}

	for name, spec := range map[string]string{
		"Update default only":      strings.Replace(base, defaultBlock, noDefaultBlock, 1),
		"different Update default": withUpdate(strings.Replace(defaultBlock, "default: true", "default: false", 1)),
	} {
		doc, err = Load([]byte(spec))
		if err != nil {
			t.Fatal(err)
		}
		if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "FIELD_DEFAULT_CONTRACT_INCONSISTENT") {
			t.Fatalf("%s: codes %v do not contain FIELD_DEFAULT_CONTRACT_INCONSISTENT", name, codes)
		}
	}

	invalid := strings.ReplaceAll(base, "default: true", "default: not-a-boolean")
	doc, err = Load([]byte(invalid))
	if err != nil {
		t.Fatal(err)
	}
	if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "FIELD_DEFAULT_INVALID") {
		t.Fatalf("codes %v do not contain FIELD_DEFAULT_INVALID", codes)
	}
}

// In existing mode, only a default line replaces the declared server default. Another line
// keeps the generated resource on the declared default, so the contract of it is still checked.
func TestExistingModeServerDefaultContract(t *testing.T) {
	base := string(validSpec(t))
	defaultBlock := "                enabled:\n                  type: boolean\n                  default: true\n                  x-coralogix-presence: true"
	noDefaultBlock := "                enabled:\n                  type: boolean\n                  x-coralogix-presence: true"
	updateBlock := strings.LastIndex(base, defaultBlock)
	different := base[:updateBlock] + strings.Replace(defaultBlock, "default: true", "default: false", 1) + base[updateBlock+len(defaultBlock):]
	released := Policy{Existing: true, Released: []string{"Thing.enabled"}}
	withDefault := Policy{Existing: true, Released: []string{"Thing.enabled"}, Defaults: []string{"Thing.enabled"}}
	tests := map[string]struct {
		spec   string
		policy Policy
		want   string // "" means no default issue
	}{
		"other line, different Update default":   {different, released, "FIELD_DEFAULT_CONTRACT_INCONSISTENT"},
		"default line, different Update default": {different, withDefault, ""},
		"other line, no declared default":        {strings.ReplaceAll(base, defaultBlock, noDefaultBlock), released, ""},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			doc, err := Load([]byte(test.spec))
			if err != nil {
				t.Fatal(err)
			}
			codes := reportCodes(ValidateWithPolicy(doc, "Thing", OperationIDs{}, test.policy))
			hasDefaultIssue := slices.ContainsFunc(codes, func(c string) bool { return strings.Contains(c, "DEFAULT") })
			if test.want == "" && hasDefaultIssue || test.want != "" && !slices.Contains(codes, test.want) {
				t.Fatalf("codes = %v, want %q", codes, test.want)
			}
		})
	}
}

func TestWriteOnlyAndLookaroundPatternAreIneligible(t *testing.T) {
	base := string(validSpec(t))
	tests := map[string]struct {
		spec string
		code string
	}{
		"write only": {
			spec: strings.Replace(base, "        enabled:\n          type: boolean\n", "        enabled:\n          type: boolean\n          writeOnly: true\n", 1),
			code: "FIELD_WRITE_ONLY_UNSUPPORTED",
		},
		"pattern on a request enum": {
			spec: strings.Replace(base, "    ThingKind:\n      type: string\n", "    ThingKind:\n      type: string\n      pattern: '^[a-z]+$'\n", 1),
			code: "STRING_PATTERN_UNSUPPORTED",
		},
		"request pattern that Go cannot compile": {
			spec: strings.Replace(base, "                name:\n                  type: string\n", "                name:\n                  type: string\n                  pattern: '^(?!x)[a-z]+$'\n", 1),
			code: "STRING_PATTERN_UNSUPPORTED",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			doc, err := Load([]byte(test.spec))
			if err != nil {
				t.Fatal(err)
			}
			if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, test.code) {
				t.Fatalf("codes %v do not contain %s", codes, test.code)
			}
		})
	}
}

// A pattern becomes a validator of the configuration. The configuration never
// sets a value that only the response has, so a pattern there is no issue.
func TestResponseOnlyPatternIsEligible(t *testing.T) {
	spec := strings.Replace(string(validSpec(t)),
		"        id:\n          type: string\n          description: The server-assigned identifier.\n",
		"        id:\n          type: string\n          pattern: '^[0-9a-f-]+$'\n          description: The server-assigned identifier.\n", 1)
	if !strings.Contains(spec, "pattern: '^[0-9a-f-]+$'") {
		t.Fatal("fixture text not found")
	}
	doc, err := Load([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	if report := Validate(doc, "Thing", OperationIDs{}); len(report) != 0 {
		t.Fatalf("a pattern on the response-only id is ineligible: %v", report)
	}
}

// withNamePattern sets pattern on name in the Create body, the Update body, and
// Thing. Only Thing when requests is false.
func withNamePattern(t *testing.T, pattern string, requests bool) string {
	t.Helper()
	spec := string(validSpec(t))
	if requests {
		spec = strings.Replace(spec, "                name:\n                  type: string\n", "                name:\n                  type: string\n                  pattern: '"+pattern+"'\n", 2)
	}
	return strings.Replace(spec, "        name:\n          type: string\n", "        name:\n          type: string\n          pattern: '"+pattern+"'\n", 1)
}

func namePattern(t *testing.T, spec string, policy Policy) string {
	t.Helper()
	doc, err := Load([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	if report := ValidateWithPolicy(doc, "Thing", OperationIDs{}, policy); len(report) != 0 {
		t.Fatalf("ineligible: %v", report)
	}
	r, err := BuildWithPolicy(doc, "Thing", OperationIDs{}, policy)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range r.Fields {
		if f.Name == "name" {
			return f.Type.Pattern
		}
	}
	t.Fatal("no name field")
	return ""
}

func TestRequestPatternIsTheFieldPattern(t *testing.T) {
	const pattern = "^[a-z][a-z0-9-]*$"
	if got := namePattern(t, withNamePattern(t, pattern, true), Policy{}); got != pattern {
		t.Errorf("pattern = %q, want %q", got, pattern)
	}
	if got := namePattern(t, withNamePattern(t, `^[\s\S]*$`, true), Policy{}); got != "" {
		t.Errorf("free-text pattern = %q, want none", got)
	}
	// An existing resource keeps the released behavior: no pattern validator.
	if got := namePattern(t, withNamePattern(t, pattern, true), Policy{Existing: true}); got != "" {
		t.Errorf("existing-mode pattern = %q, want none", got)
	}
}

func TestRequestPatternMustMatchTheResponse(t *testing.T) {
	doc, err := Load([]byte(withNamePattern(t, "^[a-z]+$", false)))
	if err != nil {
		t.Fatal(err)
	}
	if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "FIELD_TYPE_INCONSISTENT") {
		t.Fatalf("codes %v do not contain FIELD_TYPE_INCONSISTENT", codes)
	}
}

func TestObjectPropertyCountsAreIneligible(t *testing.T) {
	base := string(validSpec(t))
	for _, keyword := range []string{"minProperties: 1", "maxProperties: 2"} {
		t.Run(keyword, func(t *testing.T) {
			spec := strings.Replace(base, "    Thing:\n      type: object\n", "    Thing:\n      type: object\n      "+keyword+"\n", 1)
			doc, err := Load([]byte(spec))
			if err != nil {
				t.Fatal(err)
			}
			if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "OBJECT_PROPERTY_COUNT_UNSUPPORTED") {
				t.Fatalf("codes %v do not contain OBJECT_PROPERTY_COUNT_UNSUPPORTED", codes)
			}
		})
	}
}

func TestExclusiveNumericBoundsAreIneligible(t *testing.T) {
	tests := map[string]string{
		"exclusive minimum": "      minimum: 0\n      exclusiveMinimum: true\n",
		"exclusive maximum": "      maximum: 10\n      exclusiveMaximum: true\n",
	}
	for name, bound := range tests {
		t.Run(name, func(t *testing.T) {
			doc := loadComponent(t, fmt.Sprintf(`
    Counter:
      type: integer
      format: int64
%s`, bound))
			_, problems := Survey(doc, "Counter")
			var report issue.Report
			for _, problem := range problems {
				report = append(report, schemaIssue(problem))
			}
			if codes := reportCodes(report); !slices.Contains(codes, "NUMERIC_EXCLUSIVE_BOUND_UNSUPPORTED") {
				t.Fatalf("codes %v do not contain NUMERIC_EXCLUSIVE_BOUND_UNSUPPORTED", codes)
			}
		})
	}
}

func TestRootOneOfMustMatchEveryLifecycle(t *testing.T) {
	base := string(validSpec(t))
	group := "\n              oneOf:\n                - required: [description]\n                - required: [destinations]"
	responseGroup := "\n      oneOf:\n        - required: [description]\n        - required: [destinations]"
	mismatch := strings.Replace(base, "    Thing:\n      type: object", "    Thing:\n      type: object"+responseGroup, 1)
	doc, err := Load([]byte(mismatch))
	if err != nil {
		t.Fatal(err)
	}
	if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "ROOT_ONEOF_LIFECYCLE_INCONSISTENT") {
		t.Fatalf("codes %v do not contain ROOT_ONEOF_LIFECYCLE_INCONSISTENT", codes)
	}

	matching := strings.Replace(base, "              title: CreateThingRequest\n              type: object", "              title: CreateThingRequest\n              type: object"+group, 1)
	matching = strings.Replace(matching, "              title: UpdateThingRequest\n              type: object", "              title: UpdateThingRequest\n              type: object"+group, 1)
	matching = strings.Replace(matching, "    Thing:\n      type: object", "    Thing:\n      type: object"+responseGroup, 1)
	doc, err = Load([]byte(matching))
	if err != nil {
		t.Fatal(err)
	}
	if report := Validate(doc, "Thing", OperationIDs{}); len(report) != 0 {
		t.Fatal(report)
	}
	resource, err := Build(doc, "Thing")
	if err != nil {
		t.Fatal(err)
	}
	if len(resource.Groups) != 1 || !slices.Equal(resource.Groups[0].Arms, []string{"description", "destinations"}) {
		t.Fatalf("root groups = %#v", resource.Groups)
	}
}

func TestCollectionContracts(t *testing.T) {
	doc := loadComponent(t, `
    Collections:
      type: object
      properties:
        ordered:
          type: array
          items: {type: string}
        unordered:
          type: array
          uniqueItems: true
          x-coralogix-collection: set
          items: {type: string}
        labels:
          type: object
          additionalProperties: {type: string}
`)
	typeValue, problems := Survey(doc, "Collections")
	if len(problems) != 0 {
		t.Fatalf("unexpected problems: %v", problems)
	}
	want := []Kind{List, Set, Map}
	for index, field := range typeValue.Fields {
		if field.Type.Kind != want[index] {
			t.Errorf("%s kind = %s, want %s", field.Name, field.Type.Kind, want[index])
		}
	}

	for name, schema := range map[string]string{
		"uniqueItems alone": `
    Bad:
      type: array
      uniqueItems: true
      items: {type: string}
`,
		"untyped map": `
    Bad:
      type: object
      additionalProperties: true
`,
	} {
		t.Run(name, func(t *testing.T) {
			_, problems := Survey(loadComponent(t, schema), "Bad")
			if len(problems) == 0 {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestPatchUpdateContract(t *testing.T) {
	data := validSpec(t)
	doc, err := Load(data)
	if err != nil {
		t.Fatal(err)
	}
	resource, err := Build(doc, "Thing")
	if err != nil {
		t.Fatal(err)
	}
	if resource.Replace || resource.UpdateMask != "updateMask" {
		t.Fatalf("PATCH resource = replace %t, mask %q", resource.Replace, resource.UpdateMask)
	}

	withoutPattern := strings.Replace(string(data), "            pattern: '^[a-z][A-Za-z0-9]*(\\.[a-z][A-Za-z0-9]*)*(,[a-z][A-Za-z0-9]*(\\.[a-z][A-Za-z0-9]*)*)*$'\n", "", 1)
	doc, err = Load([]byte(withoutPattern))
	if err != nil {
		t.Fatal(err)
	}
	if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "UPDATE_MASK_CONTRACT_MISSING") {
		t.Fatalf("codes %v do not contain UPDATE_MASK_CONTRACT_MISSING", codes)
	}

	legacyBody := removeMaskQuery(t, string(data))
	withoutMask := legacyBody
	legacyBody = strings.Replace(legacyBody,
		"                labels:\n                  type: object\n                  additionalProperties: {type: string}\n",
		"                labels:\n                  type: object\n                  additionalProperties: {type: string}\n                updateMask:\n                  type: string\n                  pattern: '^[a-z][A-Za-z0-9]*(\\.[a-z][A-Za-z0-9]*)*(,[a-z][A-Za-z0-9]*(\\.[a-z][A-Za-z0-9]*)*)*$'\n", 1)
	if legacyBody == withoutMask {
		t.Fatal("the legacy body-mask fixture did not change the spec")
	}
	doc, err = Load([]byte(legacyBody))
	if err != nil {
		t.Fatal(err)
	}
	if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "CLEAR_BEHAVIOR_UNKNOWN") {
		t.Fatalf("legacy body-mask codes %v do not contain CLEAR_BEHAVIOR_UNKNOWN", codes)
	}

	optionalQuery := strings.Replace(string(data), "          required: true\n", "", 1)
	doc, err = Load([]byte(optionalQuery))
	if err != nil {
		t.Fatal(err)
	}
	if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "CLEAR_BEHAVIOR_UNKNOWN") {
		t.Fatalf("optional query-mask codes %v do not contain CLEAR_BEHAVIOR_UNKNOWN", codes)
	}
}

func TestUpdateMaskProtoName(t *testing.T) {
	data := string(validSpec(t))
	protoName := strings.Replace(data, "        - name: updateMask\n", "        - name: update_mask\n", 1)
	if protoName == data {
		t.Fatal("cannot rename the updateMask query parameter")
	}
	doc, err := Load([]byte(protoName))
	if err != nil {
		t.Fatal(err)
	}
	resource, err := Build(doc, "Thing")
	if err != nil {
		t.Fatal(err)
	}
	if resource.UpdateMask != "update_mask" || resource.UpdateMaskPattern == "" {
		t.Fatalf("mask %q, pattern %q", resource.UpdateMask, resource.UpdateMaskPattern)
	}

	bothNames := strings.Replace(data, "        - name: updateMask\n", "        - name: update_mask\n          in: query\n          schema: {type: string}\n        - name: updateMask\n", 1)
	doc, err = Load([]byte(bothNames))
	if err != nil {
		t.Fatal(err)
	}
	if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "CLEAR_BEHAVIOR_UNKNOWN") {
		t.Fatalf("codes %v do not contain CLEAR_BEHAVIOR_UNKNOWN", codes)
	}
}

func TestSingleAllOfResponseIsDirect(t *testing.T) {
	direct := "              schema:\n                $ref: '#/components/schemas/Thing'\n"
	allOf := "              schema:\n                description: The thing.\n                allOf:\n                  - $ref: '#/components/schemas/%s'\n"
	spec := strings.ReplaceAll(string(validSpec(t)), direct, fmt.Sprintf(allOf, "Thing"))
	doc, err := Load([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	if report := Validate(doc, "Thing", OperationIDs{}); len(report) != 0 {
		t.Fatalf("allOf responses are ineligible: %v", report)
	}
	resource, err := Build(doc, "Thing")
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range []Operation{resource.Create, resource.Get, resource.Update} {
		if !op.Response.Direct || op.Response.Schema != "Thing" {
			t.Errorf("%s response = %+v, want the direct Thing", op.OperationID, op.Response)
		}
	}

	wrapped := strings.Replace(string(validSpec(t)), direct, fmt.Sprintf(allOf, "CreateThingResponse"), 1)
	wrapped = strings.Replace(wrapped, "    DeleteThingResponse:\n", "    CreateThingResponse:\n      type: object\n      required: [thing]\n      properties:\n        thing:\n          $ref: '#/components/schemas/Thing'\n    DeleteThingResponse:\n", 1)
	doc, err = Load([]byte(wrapped))
	if err != nil {
		t.Fatal(err)
	}
	if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "RESPONSE_WRAPPER_UNSUPPORTED") {
		t.Fatalf("codes %v do not contain RESPONSE_WRAPPER_UNSUPPORTED", codes)
	}
}

func TestDeleteResponseCanBeInlineEmptyObject(t *testing.T) {
	ref := "                $ref: '#/components/schemas/DeleteThingResponse'\n"
	for name, c := range map[string]struct {
		schema   string
		eligible bool
	}{
		"empty object, as for google.protobuf.Empty": {"                type: object\n", true},
		"object with fields":                         {"                type: object\n                properties:\n                  ok:\n                    type: boolean\n", false},
	} {
		spec := strings.Replace(string(validSpec(t)), ref, c.schema, 1)
		doc, err := Load([]byte(spec))
		if err != nil {
			t.Fatal(err)
		}
		codes := reportCodes(Validate(doc, "Thing", OperationIDs{}))
		if c.eligible != (len(codes) == 0) {
			t.Fatalf("%s: codes %v, want eligible=%v", name, codes, c.eligible)
		}
		if !c.eligible {
			continue
		}
		resource, err := Build(doc, "Thing")
		if err != nil {
			t.Fatal(err)
		}
		if got := resource.Delete.Response; !got.Empty || got.Schema != "" {
			t.Errorf("%s: Delete response = %+v, want an empty response with no component", name, got)
		}
	}
}

func TestPutUpdateContract(t *testing.T) {
	data := validSpec(t)
	put := strings.Replace(string(data), "    patch:\n", "    put:\n", 1)
	put = strings.Replace(put, "ThingsService_UpdateThing", "ThingsService_ReplaceThing", 1)
	put = removeMaskQuery(t, put)
	doc, err := Load([]byte(put))
	if err != nil {
		t.Fatal(err)
	}
	if report := Validate(doc, "Thing", OperationIDs{}); len(report) != 0 {
		t.Fatal(report)
	}
	resource, err := Build(doc, "Thing")
	if err != nil {
		t.Fatal(err)
	}
	if !resource.Replace || resource.UpdateMask != "" {
		t.Fatalf("PUT resource = replace %t, mask %q", resource.Replace, resource.UpdateMask)
	}
}

func TestUpdateIDInBodyIsIneligible(t *testing.T) {
	for _, method := range []string{"patch", "put"} {
		t.Run(method, func(t *testing.T) {
			spec := collectionUpdateSpec(t, string(validSpec(t)), method)
			doc, err := Load([]byte(spec))
			if err != nil {
				t.Fatal(err)
			}
			codes := reportCodes(Validate(doc, "Thing", OperationIDs{}))
			if !slices.Contains(codes, "UPDATE_ID_IN_BODY_UNSUPPORTED") {
				t.Fatalf("codes %v do not contain UPDATE_ID_IN_BODY_UNSUPPORTED", codes)
			}
		})
	}
}

func TestGetResponseIDMustBeRequired(t *testing.T) {
	spec := strings.Replace(string(validSpec(t)), "      required: [id, name, enabled, kind, config, status, createTime, updateTime]\n", "      required: [name, enabled, kind, config, status, createTime, updateTime]\n", 1)
	doc, err := Load([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "RESOURCE_ID_OPTIONAL") {
		t.Fatalf("codes %v do not contain RESOURCE_ID_OPTIONAL", codes)
	}
}

func TestRequestRequirednessContract(t *testing.T) {
	doc, err := Load(validSpec(t))
	if err != nil {
		t.Fatal(err)
	}
	if report := Validate(doc, "Thing", OperationIDs{}); len(report) != 0 {
		t.Fatalf("Create-required and Update-optional must be eligible: %v", report)
	}

	spec := optionalCreateRequiredUpdateSpec(t, string(validSpec(t)))
	doc, err = Load([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "FIELD_REQUIREDNESS_UNSUPPORTED") {
		t.Fatalf("codes %v do not contain FIELD_REQUIREDNESS_UNSUPPORTED", codes)
	}
}

func optionalCreateRequiredUpdateSpec(t *testing.T, spec string) string {
	t.Helper()
	spec = strings.Replace(spec,
		"              required: [name, kind, config]\n",
		"              required: [kind, config]\n", 1)
	spec = strings.Replace(spec,
		"              title: UpdateThingRequest\n              type: object\n              required: []\n              properties:\n",
		"              title: UpdateThingRequest\n              type: object\n              required: [name]\n              properties:\n", 1)
	spec = strings.Replace(spec, "      required: [id, name, enabled, kind, config, status, createTime, updateTime]\n", "      required: [id, enabled, kind, config, status, createTime, updateTime]\n", 1)
	if !strings.Contains(spec, "title: UpdateThingRequest\n              type: object\n              required: [name]") {
		t.Fatal("cannot build requiredness fixture")
	}
	return spec
}

func replaceOperationBodySchema(t *testing.T, spec, operationID, replacement string) string {
	t.Helper()
	operation := strings.Index(spec, "operationId: "+operationID)
	if operation < 0 {
		t.Fatalf("operation %s not found", operationID)
	}
	const schemaMarker = "            schema:\n"
	schema := strings.Index(spec[operation:], schemaMarker)
	if schema < 0 {
		t.Fatalf("request schema for %s not found", operationID)
	}
	schema += operation + len(schemaMarker)
	responses := strings.Index(spec[schema:], "      responses:\n")
	if responses < 0 {
		t.Fatalf("responses for %s not found", operationID)
	}
	responses += schema
	return spec[:schema] + replacement + spec[responses:]
}

func TestResourceIDTypes(t *testing.T) {
	for _, format := range []string{"int32", "int64"} {
		t.Run(format, func(t *testing.T) {
			doc, err := Load(integerIDSpec(t, validSpec(t), format))
			if err != nil {
				t.Fatal(err)
			}
			if report := Validate(doc, "Thing", OperationIDs{}); len(report) != 0 {
				t.Fatal(report)
			}
			resource, err := Build(doc, "Thing")
			if err != nil {
				t.Fatal(err)
			}
			if resource.IDType == nil || resource.IDType.Kind != Integer || resource.IDType.Format != format {
				t.Fatalf("ID type = %#v, want integer/%s", resource.IDType, format)
			}
		})
	}

	t.Run("unsupported", func(t *testing.T) {
		spec := string(validSpec(t))
		spec = strings.Replace(spec, "        schema:\n          type: string\n    get:\n", "        schema:\n          type: boolean\n    get:\n", 1)
		spec = strings.Replace(spec, "        id:\n          type: string\n          description: The server-assigned identifier.\n", "        id:\n          type: boolean\n          description: The server-assigned identifier.\n", 1)
		doc, err := Load([]byte(spec))
		if err != nil {
			t.Fatal(err)
		}
		if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "RESOURCE_ID_TYPE_UNSUPPORTED") {
			t.Fatalf("codes %v do not contain RESOURCE_ID_TYPE_UNSUPPORTED", codes)
		}
	})

	t.Run("inconsistent", func(t *testing.T) {
		spec := integerIDSpec(t, validSpec(t), "int32")
		spec = []byte(strings.Replace(string(spec), "        id:\n          type: integer\n          format: int32\n", "        id:\n          type: integer\n          format: int64\n", 1))
		doc, err := Load(spec)
		if err != nil {
			t.Fatal(err)
		}
		if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "RESOURCE_ID_TYPE_INCONSISTENT") {
			t.Fatalf("codes %v do not contain RESOURCE_ID_TYPE_INCONSISTENT", codes)
		}
	})
}

func TestWrappedResourceResponsesAreIneligible(t *testing.T) {
	spec := strings.Replace(string(validSpec(t)), "$ref: '#/components/schemas/Thing'", "$ref: '#/components/schemas/CreateThingResponse'", 1)
	spec = strings.Replace(spec, "    DeleteThingResponse:\n", "    CreateThingResponse:\n      type: object\n      required: [thing]\n      properties:\n        thing:\n          $ref: '#/components/schemas/Thing'\n    DeleteThingResponse:\n", 1)
	doc, err := Load([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "RESPONSE_WRAPPER_UNSUPPORTED") {
		t.Fatalf("codes %v do not contain RESPONSE_WRAPPER_UNSUPPORTED", codes)
	}
}

func integerIDSpec(t *testing.T, spec []byte, format string) []byte {
	t.Helper()
	value := string(spec)
	pathID := "        schema:\n          type: string\n    get:\n"
	pathIntegerID := fmt.Sprintf("        schema:\n          type: integer\n          format: %s\n    get:\n", format)
	value = strings.Replace(value, pathID, pathIntegerID, 1)
	responseID := "        id:\n          type: string\n          description: The server-assigned identifier.\n"
	responseIntegerID := fmt.Sprintf("        id:\n          type: integer\n          format: %s\n          description: The server-assigned identifier.\n", format)
	value = strings.Replace(value, responseID, responseIntegerID, 1)
	if !strings.Contains(value, pathIntegerID) || !strings.Contains(value, responseIntegerID) {
		t.Fatalf("cannot convert synthetic ID to %s", format)
	}
	return []byte(value)
}

func collectionUpdateSpec(t *testing.T, spec, method string) string {
	t.Helper()
	if method == "put" {
		spec = strings.Replace(spec, "    patch:\n", "    put:\n", 1)
		spec = strings.Replace(spec, "ThingsService_UpdateThing", "ThingsService_ReplaceThing", 1)
		spec = removeMaskQuery(t, spec)
	}
	start := strings.Index(spec, "    "+method+":\n")
	end := strings.Index(spec[start:], "    delete:\n")
	item := strings.Index(spec, "  /things/{id}:\n")
	if start < 0 || end < 0 || item < 0 {
		t.Fatal("cannot locate Update operation")
	}
	end += start
	update := spec[start:end]
	spec = spec[:start] + spec[end:]
	spec = spec[:item] + update + spec[item:]
	marker := "              type: object\n              required: []\n              properties:\n"
	replacement := "              type: object\n              required: [id]\n              properties:\n                id:\n                  type: string\n"
	operation := "operationId: ThingsService_" + map[string]string{"patch": "Update", "put": "Replace"}[method] + "Thing"
	position := strings.Index(spec, operation)
	if position < 0 {
		t.Fatalf("cannot locate %s", operation)
	}
	tail := strings.Replace(spec[position:], marker, replacement, 1)
	if tail == spec[position:] {
		t.Fatal("cannot add Update body id")
	}
	return spec[:position] + tail
}

func removeMaskQuery(t *testing.T, spec string) string {
	t.Helper()
	start := strings.Index(spec, "      parameters:\n")
	if start < 0 {
		t.Fatal("cannot locate updateMask query parameter")
	}
	end := strings.Index(spec[start:], "      requestBody:\n")
	if end < 0 {
		t.Fatal("cannot locate updateMask query parameter")
	}
	return spec[:start] + spec[start+end:]
}

func TestSchemaIssuesAreAggregatedAndSorted(t *testing.T) {
	doc := loadComponent(t, `
    Broken:
      type: object
      properties:
        fooBar: {type: string}
        foo_bar: {type: string}
        children:
          type: array
          items: {$ref: '#/components/schemas/Broken'}
        free:
          type: object
          additionalProperties: true
        variant:
          anyOf:
            - {type: string}
            - {type: integer, format: int64}
`)
	report := Validate(doc, "Broken", OperationIDs{})
	codes := reportCodes(report)
	for _, want := range []string{"MAP_VALUE_TYPE_UNSUPPORTED", "SCHEMA_RECURSIVE", "TERRAFORM_NAME_COLLISION", "UNION_SHAPE_UNSUPPORTED"} {
		if !slices.Contains(codes, want) {
			t.Errorf("codes %v do not contain %s", codes, want)
		}
	}
	if got := report.Normalize(); !slices.Equal(report, got) {
		t.Fatalf("report is not normalized:\n%v\nwant:\n%v", report, got)
	}
}

func TestOperationDiscoveryAndExplicitSelection(t *testing.T) {
	data := string(validSpec(t))
	extra := `  /other-things:
    post:
      tags: [Things Service]
      operationId: Other_CreateThing
      requestBody:
        required: true
        content:
          application/json:
            schema:
              title: OtherCreateThingRequest
              type: object
              required: [name]
              properties:
                name: {type: string}
                enabled: {type: boolean, x-coralogix-presence: true}
                count: {type: integer, format: int64, x-coralogix-presence: true}
      responses:
        '200':
          description: Created
          content:
            application/json:
              schema: {$ref: '#/components/schemas/Thing'}
`
	data = strings.Replace(data, "components:\n", extra+"components:\n", 1)
	doc, err := Load([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "OPERATION_AMBIGUOUS") {
		t.Fatalf("codes %v do not contain OPERATION_AMBIGUOUS", codes)
	}
	ids := OperationIDs{Create: "ThingsService_CreateThing"}
	if report := Validate(doc, "Thing", ids); len(report) != 0 {
		t.Fatal(report)
	}
	resource, err := BuildWithOperationIDs(doc, "Thing", ids)
	if err != nil {
		t.Fatal(err)
	}
	if resource.Create.OperationID != ids.Create {
		t.Fatalf("selected %q, want %q", resource.Create.OperationID, ids.Create)
	}
}

func TestOperationDocumentOrderDoesNotChangeSelection(t *testing.T) {
	original := string(validSpec(t))
	collectionStart := strings.Index(original, "  /things:\n")
	itemStart := strings.Index(original, "  /things/{id}:\n")
	componentsStart := strings.Index(original, "components:\n")
	if collectionStart < 0 || itemStart < 0 || componentsStart < 0 {
		t.Fatal("cannot locate path blocks")
	}
	reordered := original[:collectionStart] + original[itemStart:componentsStart] + original[collectionStart:itemStart] + original[componentsStart:]
	var resources []*Resource
	for _, data := range []string{original, reordered} {
		doc, err := Load([]byte(data))
		if err != nil {
			t.Fatal(err)
		}
		resource, err := Build(doc, "Thing")
		if err != nil {
			t.Fatal(err)
		}
		resources = append(resources, resource)
	}
	if !reflect.DeepEqual(resources[0], resources[1]) {
		t.Fatalf("operation order changed selection:\n%#v\n%#v", resources[0], resources[1])
	}
}

func validSpec(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "generator", "testdata", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func loadComponent(t *testing.T, schemas string) *DocumentAlias {
	t.Helper()
	doc, err := Load([]byte("openapi: 3.0.3\ninfo: {title: test, version: 1.0.0}\npaths: {}\ncomponents:\n  schemas:\n" + schemas))
	if err != nil {
		t.Fatal(err)
	}
	return (*DocumentAlias)(doc)
}

// DocumentAlias keeps the helper signature short without changing model APIs.
type DocumentAlias = v3.Document

func reportCodes(report issue.Report) []string {
	normalized := report.Normalize()
	codes := make([]string, 0, len(normalized))
	for _, item := range normalized {
		codes = append(codes, item.Code)
	}
	return codes
}

// specType returns the type of the spec field of the fixture, with the
// request components that Build merged into it.
func specType(t *testing.T) *Type {
	t.Helper()
	doc, err := Load(validSpec(t))
	if err != nil {
		t.Fatal(err)
	}
	r, err := Build(doc, "Thing")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range r.Fields {
		if f.Name == "spec" {
			return f.Type
		}
	}
	t.Fatal("no spec field")
	return nil
}

func TestNestedFieldLifecycle(t *testing.T) {
	spec := specType(t)
	if spec.CreateSchema != "ThingSpecCreate" || spec.UpdateSchema != "ThingSpecUpdate" {
		t.Fatalf("request components = %s, %s", spec.CreateSchema, spec.UpdateSchema)
	}
	item := fieldNamed(spec.Fields, "items").Type.Elem
	source := fieldNamed(spec.Fields, "source").Type
	target := fieldNamed(spec.Fields, "targets").Type.Elem
	for _, test := range []struct {
		object *Type
		field  string
		want   Behavior
	}{
		{spec, "mode", Normal},
		{spec, "region", Immutable},
		{spec, "revision", Computed},
		{spec, "source", Computed},
		{spec, "items", Normal},
		{spec, "targets", Immutable},
		{target, "id", Computed}, // inside an immutable value, Create does not send it
		{target, "name", Normal},
		{item, "id", Computed},
		{item, "key", Immutable},
		{item, "name", Normal},
		{source, "origin", ""}, // inside a computed value
	} {
		if got := fieldNamed(test.object.Fields, test.field).Behavior; got != test.want {
			t.Errorf("%s.%s = %q, want %q", test.object.Schema, test.field, got, test.want)
		}
	}
}

func TestNestedRequestTypes(t *testing.T) {
	spec := specType(t)
	create, update := spec.CreateType(), spec.UpdateType()
	if create.Schema != "ThingSpecCreate" || create.Model != "ThingSpec" || !slices.Equal(fieldNames(create), []string{"mode", "region", "items", "targets"}) {
		t.Errorf("Create type = %s for %s with %v", create.Schema, create.Model, fieldNames(create))
	}
	if update.Schema != "ThingSpecUpdate" || !slices.Equal(fieldNames(update), []string{"mode", "items"}) {
		t.Errorf("Update type = %s with %v", update.Schema, fieldNames(update))
	}
	updateItem := fieldNamed(update.Fields, "items").Type.Elem
	if updateItem.Schema != "ThingItemUpdate" || updateItem.Model != "ThingItem" || !slices.Equal(fieldNames(updateItem), []string{"name"}) {
		t.Errorf("Update item type = %s for %s with %v", updateItem.Schema, updateItem.Model, fieldNames(updateItem))
	}
}

func TestNestedFieldContract(t *testing.T) {
	const updateItem = "    ThingItemUpdate:\n      type: object\n      required: [name]\n      properties:\n        name:\n          type: string\n          minLength: 1\n"
	tests := map[string]struct {
		old, new, code, location string
	}{
		"different kind": {
			updateItem, strings.Replace(updateItem, "type: string\n          minLength: 1", "type: boolean\n          x-coralogix-presence: true", 1),
			"FIELD_TYPE_INCONSISTENT", "components.schemas.Thing.spec.items[].name",
		},
		"different limits": {
			updateItem, strings.Replace(updateItem, "minLength: 1", "minLength: 2", 1),
			"FIELD_TYPE_INCONSISTENT", "components.schemas.Thing.spec.items[].name",
		},
		"request field missing from the response": {
			updateItem, updateItem + "        extra:\n          type: string\n          x-coralogix-presence: true\n",
			"FIELD_LIFECYCLE_UNSUPPORTED", "components.schemas.Thing.spec.items[].extra",
		},
		"Update field missing from Create": {
			updateItem, updateItem + "        id:\n          type: string\n          x-coralogix-presence: true\n",
			"FIELD_LIFECYCLE_UNSUPPORTED", "components.schemas.Thing.spec.items[].id",
		},
		"optional in Create, required in Update": {
			"    ThingSpecUpdate:\n      type: object\n      required: []\n", "    ThingSpecUpdate:\n      type: object\n      required: [mode]\n",
			"FIELD_REQUIREDNESS_UNSUPPORTED", "components.schemas.Thing.spec.mode",
		},
		"different oneOf arms": {
			"                config:\n                  allOf:\n                    - $ref: '#/components/schemas/ThingConfig'\n",
			"                config:\n                  $ref: '#/components/schemas/ThingConfigUpdate'\n",
			"FIELD_TYPE_INCONSISTENT", "components.schemas.Thing.config",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			spec := string(validSpec(t))
			if !strings.Contains(spec, test.old) {
				t.Fatal("fixture text not found")
			}
			spec = strings.Replace(spec, test.old, test.new, 1)
			spec = strings.Replace(spec, "    DeleteThingResponse:\n",
				"    ThingConfigUpdate:\n      type: object\n      required: []\n      properties:\n        http:\n          $ref: '#/components/schemas/HttpThingConfig'\n      oneOf:\n        - required: [http]\n    DeleteThingResponse:\n", 1)
			doc, err := Load([]byte(spec))
			if err != nil {
				t.Fatal(err)
			}
			var found []issue.Issue
			for _, item := range Validate(doc, "Thing", OperationIDs{}) {
				if item.Location == test.location {
					found = append(found, item)
				}
			}
			// Validate and Build report the same issue once.
			if len(found) != 1 || found[0].Code != test.code {
				t.Fatalf("issues at %s = %v, want one %s", test.location, found, test.code)
			}
		})
	}
}

// looseConfigSpec returns the fixture with the config oneOf of the given
// request and response schemas replaced by ThingConfigLoose, a copy that
// allows no arm. Create, Update, and the response are "create", "update", and
// "get".
func looseConfigSpec(t *testing.T, loose ...string) string {
	t.Helper()
	spec := string(validSpec(t))
	refs := map[string][2]string{
		"create": {"                config:\n                  $ref: '#/components/schemas/ThingConfig'\n                spec:\n", "                config:\n                  $ref: '#/components/schemas/ThingConfigLoose'\n                spec:\n"},
		"update": {"                config:\n                  allOf:\n                    - $ref: '#/components/schemas/ThingConfig'\n", "                config:\n                  $ref: '#/components/schemas/ThingConfigLoose'\n"},
		"get":    {"        config:\n          $ref: '#/components/schemas/ThingConfig'\n        status:\n", "        config:\n          $ref: '#/components/schemas/ThingConfigLoose'\n        status:\n"},
	}
	for _, op := range loose {
		if !strings.Contains(spec, refs[op][0]) {
			t.Fatalf("fixture text for %s not found", op)
		}
		spec = strings.Replace(spec, refs[op][0], refs[op][1], 1)
	}
	return strings.Replace(spec, "    DeleteThingResponse:\n", `    ThingConfigLoose:
      type: object
      required: []
      properties:
        http:
          $ref: '#/components/schemas/HttpThingConfig'
        queue:
          $ref: '#/components/schemas/QueueThingConfig'
      oneOf:
        - required: [http]
        - required: [queue]
        - not:
            anyOf:
              - required: [http]
              - required: [queue]
    DeleteThingResponse:
`, 1)
}

func TestOneOfNoArmRule(t *testing.T) {
	tests := map[string]struct {
		loose []string
		issue string // "" when eligible
	}{
		"Update and response allow no arm":      {loose: []string{"update", "get"}},
		"only Update allows no arm":             {loose: []string{"update"}},
		"Create allows no arm, response not":    {loose: []string{"create"}, issue: "the resource response requires one"},
		"Create and response allow, Update not": {loose: []string{"create", "get"}, issue: "the Update request requires one"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			doc, err := Load([]byte(looseConfigSpec(t, test.loose...)))
			if err != nil {
				t.Fatal(err)
			}
			var found []issue.Issue
			for _, item := range Validate(doc, "Thing", OperationIDs{}) {
				if item.Location == "components.schemas.Thing.config" {
					found = append(found, item)
				}
			}
			if test.issue == "" {
				if len(found) != 0 {
					t.Fatalf("issues = %v, want none", found)
				}
				r, err := Build(doc, "Thing")
				if err != nil {
					t.Fatal(err)
				}
				for _, f := range r.Fields {
					// The configuration follows Create, which requires an arm.
					if f.Name == "config" && f.Type.AllowNone {
						t.Fatal("config allows no arm, want the Create rule")
					}
				}
				return
			}
			if len(found) != 1 || found[0].Code != "FIELD_TYPE_INCONSISTENT" || !strings.Contains(found[0].Message, test.issue) {
				t.Fatalf("issues = %v, want one FIELD_TYPE_INCONSISTENT with %q", found, test.issue)
			}
		})
	}
}
