package model

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/issue"
	"github.com/pb33f/libopenapi/datamodel/high/base"
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

func TestPresenceContract(t *testing.T) {
	data := validSpec(t)
	doc, err := Load(data)
	if err != nil {
		t.Fatal(err)
	}
	if report := Validate(doc, "Thing", OperationIDs{}); len(report) != 0 {
		t.Fatal(report)
	}
	data = []byte(strings.Replace(string(data), "                  x-coralogix-presence: true\n", "", 1))
	doc, err = Load(data)
	if err != nil {
		t.Fatal(err)
	}
	if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "FIELD_PRESENCE_UNKNOWN") {
		t.Fatalf("codes %v do not contain FIELD_PRESENCE_UNKNOWN", codes)
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
	if codes := reportCodes(nestedPresenceIssues("request.config", proxy, map[*base.Schema]bool{})); !slices.Contains(codes, "FIELD_PRESENCE_UNKNOWN") {
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
	if report := nestedPresenceIssues("request.config", proxy, map[*base.Schema]bool{}); len(report) != 0 {
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
	if report := nestedPresenceIssues("request.choice", proxy, map[*base.Schema]bool{}); len(report) != 0 {
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
	defaultBlock := "                enabled:\n                  type: boolean\n                  default: false\n                  x-coralogix-presence: true"
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

	inconsistent := strings.Replace(base, defaultBlock, noDefaultBlock, 1)
	doc, err = Load([]byte(inconsistent))
	if err != nil {
		t.Fatal(err)
	}
	if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "FIELD_DEFAULT_CONTRACT_INCONSISTENT") {
		t.Fatalf("codes %v do not contain FIELD_DEFAULT_CONTRACT_INCONSISTENT", codes)
	}

	invalid := strings.ReplaceAll(base, "default: false", "default: not-a-boolean")
	doc, err = Load([]byte(invalid))
	if err != nil {
		t.Fatal(err)
	}
	if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "FIELD_DEFAULT_INVALID") {
		t.Fatalf("codes %v do not contain FIELD_DEFAULT_INVALID", codes)
	}
}

func TestWriteOnlyAndPatternAreIneligible(t *testing.T) {
	base := string(validSpec(t))
	tests := map[string]struct {
		spec string
		code string
	}{
		"write only": {
			spec: strings.Replace(base, "        enabled:\n          type: boolean\n", "        enabled:\n          type: boolean\n          writeOnly: true\n", 1),
			code: "FIELD_WRITE_ONLY_UNSUPPORTED",
		},
		"pattern": {
			spec: strings.Replace(base, "        name:\n          type: string\n", "        name:\n          type: string\n          pattern: '^[a-z]+$'\n", 1),
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
	base := string(validSpec(t))
	for _, bound := range []string{"minimum: 0\n                  exclusiveMinimum: true", "maximum: 10\n                  exclusiveMaximum: true"} {
		t.Run(bound, func(t *testing.T) {
			spec := strings.Replace(base, "                  type: integer\n                  format: int64\n", "                  type: integer\n                  format: int64\n                  "+bound+"\n", 1)
			doc, err := Load([]byte(spec))
			if err != nil {
				t.Fatal(err)
			}
			if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "NUMERIC_EXCLUSIVE_BOUND_UNSUPPORTED") {
				t.Fatalf("codes %v do not contain NUMERIC_EXCLUSIVE_BOUND_UNSUPPORTED", codes)
			}
		})
	}
}

func TestRootOneOfMustMatchEveryLifecycle(t *testing.T) {
	base := string(validSpec(t))
	group := "\n              oneOf:\n                - required: [count]\n                - required: [ordered]"
	responseGroup := "\n      oneOf:\n        - required: [count]\n        - required: [ordered]"
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
	if len(resource.Groups) != 1 || !slices.Equal(resource.Groups[0].Arms, []string{"count", "ordered"}) {
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

	withoutPattern := strings.Replace(string(data), "            pattern: '^[a-z][A-Za-z0-9]*(\\.[a-z][A-Za-z0-9]*)*$'\n", "", 1)
	doc, err = Load([]byte(withoutPattern))
	if err != nil {
		t.Fatal(err)
	}
	if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "UPDATE_MASK_CONTRACT_MISSING") {
		t.Fatalf("codes %v do not contain UPDATE_MASK_CONTRACT_MISSING", codes)
	}

	legacyBody := removeMaskQuery(t, string(data))
	legacyBody = strings.Replace(legacyBody,
		"                labels:\n                  type: object\n                  x-coralogix-presence: true\n                  additionalProperties: {type: string}\n",
		"                labels:\n                  type: object\n                  x-coralogix-presence: true\n                  additionalProperties: {type: string}\n                updateMask:\n                  type: string\n                  pattern: '^[a-z][A-Za-z0-9]*(\\.[a-z][A-Za-z0-9]*)*$'\n", 1)
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
	marker := "              type: object\n              properties:\n"
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
