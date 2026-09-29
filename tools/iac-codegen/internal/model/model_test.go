package model

import (
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

func TestServerDefaultContract(t *testing.T) {
	base := string(validSpec(t))
	requiredGet := strings.Replace(base, "      required: [id, name]", "      required: [id, name, enabled]", 1)
	doc, err := Load([]byte(requiredGet))
	if err != nil {
		t.Fatal(err)
	}
	if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "FIELD_SERVER_DEFAULT_UNDECLARED") {
		t.Fatalf("codes %v do not contain FIELD_SERVER_DEFAULT_UNDECLARED", codes)
	}

	declared := strings.ReplaceAll(requiredGet,
		"                enabled:\n                  type: boolean\n                  x-coralogix-presence: true",
		"                enabled:\n                  type: boolean\n                  default: false\n                  x-coralogix-presence: true")
	doc, err = Load([]byte(declared))
	if err != nil {
		t.Fatal(err)
	}
	if report := Validate(doc, "Thing", OperationIDs{}); len(report) != 0 {
		t.Fatal(report)
	}

	inconsistent := strings.Replace(requiredGet,
		"                enabled:\n                  type: boolean\n                  x-coralogix-presence: true",
		"                enabled:\n                  type: boolean\n                  default: false\n                  x-coralogix-presence: true", 1)
	doc, err = Load([]byte(inconsistent))
	if err != nil {
		t.Fatal(err)
	}
	if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "FIELD_DEFAULT_CONTRACT_INCONSISTENT") {
		t.Fatalf("codes %v do not contain FIELD_DEFAULT_CONTRACT_INCONSISTENT", codes)
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

func TestRootOneOfMustMatchEveryLifecycle(t *testing.T) {
	base := string(validSpec(t))
	group := "\n              oneOf:\n                - required: [enabled]\n                - required: [count]"
	responseGroup := "\n      oneOf:\n        - required: [enabled]\n        - required: [count]"
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
	if len(resource.Groups) != 1 || !slices.Equal(resource.Groups[0].Arms, []string{"enabled", "count"}) {
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

func TestPatchAndPutUpdateContracts(t *testing.T) {
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

	withoutPattern := strings.Replace(string(data), "                  pattern: '^[a-z][A-Za-z0-9]*(\\.[a-z][A-Za-z0-9]*)*$'\n", "", 1)
	doc, err = Load([]byte(withoutPattern))
	if err != nil {
		t.Fatal(err)
	}
	if codes := reportCodes(Validate(doc, "Thing", OperationIDs{})); !slices.Contains(codes, "UPDATE_MASK_CONTRACT_MISSING") {
		t.Fatalf("codes %v do not contain UPDATE_MASK_CONTRACT_MISSING", codes)
	}

	put := strings.Replace(string(data), "    patch:\n", "    put:\n", 1)
	put = strings.Replace(put, "ThingsService_UpdateThing", "ThingsService_ReplaceThing", 1)
	start := strings.Index(put, "                updateMask:\n")
	end := strings.Index(put[start:], "      responses:\n")
	if start < 0 || end < 0 {
		t.Fatal("cannot locate updateMask block")
	}
	put = put[:start] + put[start+end:]
	doc, err = Load([]byte(put))
	if err != nil {
		t.Fatal(err)
	}
	if report := Validate(doc, "Thing", OperationIDs{}); len(report) != 0 {
		t.Fatal(report)
	}
	resource, err = Build(doc, "Thing")
	if err != nil {
		t.Fatal(err)
	}
	if !resource.Replace || resource.UpdateMask != "" {
		t.Fatalf("PUT resource = replace %t, mask %q", resource.Replace, resource.UpdateMask)
	}
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
