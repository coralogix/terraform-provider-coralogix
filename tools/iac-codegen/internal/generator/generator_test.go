package generator

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/coralogix/coralogix-management-sdk/iac/issue"
	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"github.com/coralogix/coralogix-management-sdk/iac/source"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/version"
)

var updateGolden = flag.Bool("update", false, "replace the committed golden output")

// The golden output of a resource that users already have. It uses every key of the
// behavior-overrides file.
func TestGoldenOutputExistingResource(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "legacy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := legacyOutputWithAcceptance(t)
	options := Options{Resource: "LegacyThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "legacy-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "golden", "legacything")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputExistingResource -update:\n%s", diff)
	}
}

// The golden output of a resource whose API has no DELETE. The behavior-overrides file names
// the archive call, and the generated Delete calls it.
func TestGoldenOutputDeleteOverride(t *testing.T) {
	out := generateArchivedThing(t)
	golden := filepath.Join("testdata", "golden", "archivedthing")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputDeleteOverride -update:\n%s", diff)
	}
}

// generateArchivedThing generates the synthetic resource whose Delete is an archive call.
func generateArchivedThing(t *testing.T) string {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "archived.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "archivedthing")
	options := Options{Resource: "ArchivedThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "archived-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	return out
}

// The golden output of a resource whose responses wrap it in allOf, whose Replace sends the
// required server-set id in its body, and whose Replace echoes a server time and an internal
// flag that the resource skips.
func TestGoldenOutputEchoedValues(t *testing.T) {
	out := generateEchoThing(t)
	golden := filepath.Join("testdata", "golden", "echothing")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputEchoedValues -update:\n%s", diff)
	}
}

// generateEchoThing generates the synthetic resource whose Replace echoes server values.
func generateEchoThing(t *testing.T) string {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "echoed.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "echothing")
	options := Options{Resource: "EchoThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "echoed-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	return out
}

// The golden output of a resource with string fields that hold documents: a JSON field, and a
// YAML field in a list of objects. A change of format alone plans no change.
func TestGoldenOutputDocumentEquality(t *testing.T) {
	out := generateConfigThing(t)
	golden := filepath.Join("testdata", "golden", "configthing")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputDocumentEquality -update:\n%s", diff)
	}
}

// generateConfigThing generates the synthetic resource whose string fields hold documents.
func generateConfigThing(t *testing.T) string {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "configthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "configthing")
	options := Options{Resource: "ConfigThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "configthing-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	return out
}

// The golden output of a resource whose API holds values in wrapper objects. Terraform shows only
// the values: a string, an enum, a required number, a list of wrapped items, an object, and a
// list inside two wrappers. One component collapses in one place and stays an object in another.
func TestGoldenOutputWrappedValues(t *testing.T) {
	out := generateWrapThing(t)
	golden := filepath.Join("testdata", "golden", "wrapthing")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputWrappedValues -update:\n%s", diff)
	}
}

// The golden output of a resource whose lifecycle is custom methods and whose overrides use a
// fixed request value, including one on a nested read-only field, a string read from "" as null,
// a business-first enum, a missing list read as empty, and ExactlyOneOf.
func TestGoldenOutputCustomMethods(t *testing.T) {
	out := generateCustomThing(t)
	golden := filepath.Join("testdata", "golden", "customthing")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputCustomMethods -update:\n%s", diff)
	}
}

// The golden output of a released resource whose Create and Update take the resource itself as
// the body, whose responses hold the resource next to other values, and whose server-set id Get
// does not require.
func TestGoldenOutputSharedBody(t *testing.T) {
	out := generateSharedThing(t)
	golden := filepath.Join("testdata", "golden", "sharedthing")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputSharedBody -update:\n%s", diff)
	}
}

// The generated Create reads the resource next to other response values, Update sends the state
// id in the body, and a read without an id fails.
func TestSharedBodyRuntimeSemantics(t *testing.T) {
	input, _ := syntheticInput(t)
	out := generateSharedThing(t)
	semantics, err := os.ReadFile(filepath.Join("testdata", "sharedthing_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

// generateSharedThing generates the synthetic resource whose Create and Update share one body.
func generateSharedThing(t *testing.T) string {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "sharedthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "sharedthing")
	options := Options{Resource: "SharedThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "sharedthing-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	return out
}

// generateCustomThing generates the synthetic resource with custom-method operations.
func generateCustomThing(t *testing.T) string {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "customthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "customthing")
	options := Options{Resource: "CustomThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "customthing-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	return out
}

// With validators.inferred: true, the contract limit of an enum that an enums line renames lists the
// API values. The validator takes the Terraform values of the line instead, once.
func TestInferredEnumValidatorUsesTheEnumsLine(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "customthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.ReadFile(filepath.Join("testdata", "customthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(file), "inferred: false") {
		t.Fatal("customthing-overrides.yaml no longer has inferred: false")
	}
	overridesPath := filepath.Join(t.TempDir(), "behavior-overrides.yaml")
	if err := os.WriteFile(overridesPath, []byte(strings.Replace(string(file), "inferred: false", "inferred: true", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "customthing")
	if err := generateFromInput(Options{Resource: "CustomThing", OutputDir: out, OverridesPath: overridesPath}, input, loadDir); err != nil {
		t.Fatal(err)
	}
	schema, err := os.ReadFile(filepath.Join(out, "schema.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(schema), `"SYSTEM"`) || strings.Count(string(schema), "stringvalidator.OneOf(") == 0 {
		t.Fatalf("the enum validator must list the Terraform values, not the API values:\n%s", schema)
	}
}

// generateWrapThing generates the synthetic resource whose API holds values in wrapper objects.
func generateWrapThing(t *testing.T) string {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "wrapthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "wrapthing")
	options := Options{Resource: "WrapThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "wrapthing-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestGoldenOutput(t *testing.T) {
	input, sdkDir := syntheticInput(t)
	first := filepath.Join(t.TempDir(), "thing")
	second := filepath.Join(t.TempDir(), "thing")
	options := Options{Resource: "Thing", OutputDir: first}
	if err := generateFromInput(options, input, sdkDir); err != nil {
		t.Fatal(err)
	}
	options.OutputDir = second
	if err := generateFromInput(options, input, sdkDir); err != nil {
		t.Fatal(err)
	}
	if diff := compareDirectories(first, second); diff != "" {
		t.Fatalf("two generations differ:\n%s", diff)
	}
	golden := filepath.Join("testdata", "golden", "thing")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(first, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, first); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutput -update:\n%s", diff)
	}
	entries, err := os.ReadDir(first)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(first, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasPrefix(data, []byte(version.Header+"\n")) {
			t.Errorf("%s lacks exact header", entry.Name())
		}
	}
}

func TestGoldenCanonicalAPIContract(t *testing.T) {
	schemaData, err := os.ReadFile(filepath.Join("testdata", "golden", "thing", "schema.go"))
	if err != nil {
		t.Fatal(err)
	}
	schemaText := string(schemaData)
	for _, fragment := range []string{
		`"description": schema.StringAttribute{`,
		`serverDefaultModifier{value: types.BoolValue(true)}`,
		`stringvalidator.OneOf("THING_KIND_STANDARD", "THING_KIND_ADVANCED", "THING_KIND_P5_OR_UNSPECIFIED")`,
		`stringplanmodifier.RequiresReplace()`,
		`"config": schema.SingleNestedAttribute{`,
		`objectvalidator.ExactlyOneOf`,
		`"destinations": schema.ListAttribute{`,
		`"tags": schema.SetAttribute{`,
		`"details": schema.SetNestedAttribute{`,
		`"create_time": schema.StringAttribute{`,
		`"update_time": schema.StringAttribute{`,
	} {
		if !strings.Contains(schemaText, fragment) {
			t.Errorf("canonical golden schema lacks %q", fragment)
		}
	}
}

func TestGeneratedUpdateMaskUsesQueryParameter(t *testing.T) {
	input, sdkDir := syntheticInput(t)
	out := filepath.Join(t.TempDir(), "thing")
	if err := generateFromInput(Options{Resource: "Thing", OutputDir: out}, input, sdkDir); err != nil {
		t.Fatal(err)
	}
	mask, err := os.ReadFile(filepath.Join(out, "mask.go"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(mask, []byte("body.UpdateMask")) || !bytes.Contains(mask, []byte("return body, strings.Join(mask, \",\"), diags")) {
		t.Fatalf("mask.go does not return the mask separately from the body:\n%s", mask)
	}
	resource, err := os.ReadFile(filepath.Join(out, "resource.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(resource, []byte(".UpdateThingRequest(*body).UpdateMask(mask).Execute()")) {
		t.Fatalf("resource.go does not send updateMask through the query setter:\n%s", resource)
	}
}

// A released PATCH that puts updateMask in the JSON body uses the same update
// as a query-parameter mask, then sets that mask on the body.
func TestBodyUpdateMaskSetsTheSameMask(t *testing.T) {
	input, sdkDir := syntheticInput(t)
	input.OpenAPI = []byte(moveUpdateMaskIntoBody(t, string(input.OpenAPI)))
	out := generateBodyMaskResource(t, input, sdkDir)
	assertBodyUpdateMask(t, out)
	compileGenerated(t, out, input)
}

func moveUpdateMaskIntoBody(t *testing.T, text string) string {
	t.Helper()
	start := strings.Index(text, "operationId: ThingsService_UpdateThing")
	if start < 0 {
		t.Fatal("cannot locate the update operation")
	}
	tail := text[start:]
	params := strings.Index(tail, "      parameters:\n")
	body := strings.Index(tail, "      requestBody:\n")
	if params < 0 || body < 0 || params > body {
		t.Fatal("cannot locate the update mask parameter")
	}
	tail = tail[:params] + tail[body:]
	labels := "                labels:\n                  type: object\n                  additionalProperties: {type: string}\n"
	if !strings.Contains(tail, labels) {
		t.Fatal("cannot locate the update body")
	}
	tail = strings.Replace(tail, labels, labels+"                updateMask:\n                  type: string\n                  pattern: '^[a-z][A-Za-z0-9]*(\\.[a-z][A-Za-z0-9]*)*(,[a-z][A-Za-z0-9]*(\\.[a-z][A-Za-z0-9]*)*)*$'\n", 1)
	return text[:start] + tail
}

func generateBodyMaskResource(t *testing.T, input source.Input, sdkDir string) string {
	t.Helper()
	dir := t.TempDir()
	overrides := filepath.Join(dir, "behavior-overrides.yaml")
	const file = `resource: Thing
mode: existing
validators:
  inferred: false
api:
  updateMaskInBody: true
`
	if err := os.WriteFile(overrides, []byte(file), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "thing")
	if err := generateFromInput(Options{Resource: "Thing", OutputDir: out, OverridesPath: overrides}, input, sdkDir); err != nil {
		t.Fatal(err)
	}
	return out
}

func assertBodyUpdateMask(t *testing.T, out string) {
	t.Helper()
	resource := mustRead(t, filepath.Join(out, "resource.go"))
	if bytes.Contains(resource, []byte("UpdateMask(")) || !bytes.Contains(resource, []byte("body.UpdateMask = &mask")) {
		t.Fatalf("resource.go does not set the mask on the body:\n%s", resource)
	}
	mask := mustRead(t, filepath.Join(out, "mask.go"))
	if bytes.Contains(mask, []byte("keepChanged")) || bytes.Contains(mask, []byte("body.UpdateMask")) {
		t.Fatalf("mask.go does not reuse the query-parameter update:\n%s", mask)
	}
	schema := mustRead(t, filepath.Join(out, "schema.go"))
	if bytes.Contains(schema, []byte("update_mask")) || bytes.Contains(schema, []byte("updateMask")) {
		t.Fatalf("schema.go exposes the mask:\n%s", schema)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// The OpenAPI fork names the mask query parameter with its proto name. The
// SDK method is the same, so the resource must match the golden output.
func TestProtoMaskNameMatchesGolden(t *testing.T) {
	input, sdkDir := syntheticInput(t)
	text := strings.Replace(string(input.OpenAPI), "        - name: updateMask\n", "        - name: update_mask\n", 1)
	if text == string(input.OpenAPI) {
		t.Fatal("cannot locate the updateMask query parameter")
	}
	input.OpenAPI = []byte(text)
	assertGoldenThing(t, input, sdkDir)
}

// The OpenAPI fork writes a response_body field with a description as a single
// allOf. It is the direct resource, so the resource must match the golden output.
func TestSingleAllOfResponsesMatchGolden(t *testing.T) {
	input, sdkDir := syntheticInput(t)
	direct := "              schema:\n                $ref: '#/components/schemas/Thing'\n"
	if strings.Count(string(input.OpenAPI), direct) != 3 {
		t.Fatal("cannot locate the Create, Get, and Update responses")
	}
	input.OpenAPI = []byte(strings.ReplaceAll(string(input.OpenAPI), direct, "              schema:\n                description: The thing.\n                allOf:\n                  - $ref: '#/components/schemas/Thing'\n"))
	assertGoldenThing(t, input, sdkDir)
}

// assertGoldenThing checks that input is eligible and generates exactly the
// golden Thing resource.
func assertGoldenThing(t *testing.T, input source.Input, sdkDir string) {
	t.Helper()
	candidate := filepath.Join(t.TempDir(), "candidate.yaml")
	if err := os.WriteFile(candidate, input.OpenAPI, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Check(CheckOptions{Resource: "Thing", OpenAPIPath: candidate}); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "thing")
	if err := generateFromInput(Options{Resource: "Thing", OutputDir: out}, input, sdkDir); err != nil {
		t.Fatal(err)
	}
	if diff := compareDirectories(filepath.Join("testdata", "golden", "thing"), out); diff != "" {
		t.Fatalf("output differs from the canonical golden output:\n%s", diff)
	}
}

func TestEnumCollectionFlatteningUsesGuardedHelpers(t *testing.T) {
	fields := []*convField{
		{TFName: "statuses", Model: "Statuses", SDK: "Statuses", Conv: convStrings, Collection: "List", Enum: true, EnumZero: "STATUS_UNSPECIFIED"},
		{TFName: "status_set", Model: "StatusSet", SDK: "StatusSet", Conv: convStrings, Collection: "Set", Enum: true, EnumZero: "STATUS_UNSPECIFIED"},
		{TFName: "status_map", Model: "StatusMap", SDK: "StatusMap", Conv: convStringMap, Enum: true, EnumZero: "STATUS_UNSPECIFIED"},
	}
	var rendered bytes.Buffer
	for _, field := range fields {
		if err := templates.ExecuteTemplate(&rendered, "flattenField", field); err != nil {
			t.Fatal(err)
		}
	}
	for _, want := range []string{
		`flattenEnumsList(ctx, p.AtName("statuses"), v.Statuses, "STATUS_UNSPECIFIED", diags)`,
		`flattenEnumsSet(ctx, p.AtName("status_set"), v.StatusSet, "STATUS_UNSPECIFIED", diags)`,
		`flattenEnumMap(ctx, p.AtName("status_map"), v.StatusMap, "STATUS_UNSPECIFIED", diags)`,
	} {
		if !strings.Contains(rendered.String(), want) {
			t.Errorf("generated conversion does not contain %q", want)
		}
	}
}

func TestObjectCollectionAttrTypeFollowsSetOrList(t *testing.T) {
	child := &convObject{
		Model:         "Item",
		AttrTypesFunc: "itemAttrTypes",
		Fields:        []*convField{{TFName: "name", Conv: convString}},
	}
	b := &convBuilder{}
	got, err := b.attrType(&convObject{Model: "Parent"}, &convField{
		TFName: "items", Conv: convObjects, Collection: "Set", Object: child,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "types.SetType{ElemType: types.ObjectType{AttrTypes: itemAttrTypes()}}"
	if got != want {
		t.Fatalf("attr type:\n  got  %s\n  want %s", got, want)
	}
}

func TestEligibilityFailurePreservesOutput(t *testing.T) {
	input, sdkDir := syntheticInput(t)
	input.OpenAPI = bytes.Replace(input.OpenAPI, []byte("default: true\n                  x-coralogix-presence: true"), []byte("default: true"), 1)
	out := filepath.Join(t.TempDir(), "thing")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	prior := []byte(version.Header + "\npackage thing\n\nconst preserved = true\n")
	if err := os.WriteFile(filepath.Join(out, "prior.go"), prior, 0o644); err != nil {
		t.Fatal(err)
	}
	err := generateFromInput(Options{Resource: "Thing", OutputDir: out}, input, sdkDir)
	var eligibility *EligibilityError
	if !errors.As(err, &eligibility) {
		t.Fatalf("got %v, want EligibilityError", err)
	}
	got, readErr := os.ReadFile(filepath.Join(out, "prior.go"))
	if readErr != nil || !bytes.Equal(got, prior) {
		t.Fatalf("prior output changed: data=%q err=%v", got, readErr)
	}
}

func TestCheckUsesGenerateEligibilityAndWritesNothing(t *testing.T) {
	input, sdkDir := syntheticInput(t)
	input.OpenAPI = bytes.Replace(input.OpenAPI, []byte("default: true\n                  x-coralogix-presence: true"), []byte("default: true"), 1)
	work := t.TempDir()
	candidate := filepath.Join(work, "candidate.yaml")
	if err := os.WriteFile(candidate, input.OpenAPI, 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := directoryFiles(work)
	if err != nil {
		t.Fatal(err)
	}
	checkErr := Check(CheckOptions{Resource: "Thing", OpenAPIPath: candidate})
	generated := filepath.Join(work, "generated")
	generateErr := generateFromInput(Options{Resource: "Thing", OutputDir: generated}, input, sdkDir)
	if checkErr == nil || generateErr == nil {
		t.Fatalf("check error = %v, generate error = %v; want eligibility errors", checkErr, generateErr)
	}
	if checkErr.Error() != generateErr.Error() {
		t.Fatalf("eligibility decisions differ:\ncheck:\n%s\ngenerate:\n%s", checkErr, generateErr)
	}
	if _, err := os.Stat(generated); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("generated output exists after rejected check: %v", err)
	}
	after, err := directoryFiles(work)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("check changed files: before=%v after=%v", before, after)
	}
}

func TestContractGapsUseSharedFailClosedValidation(t *testing.T) {
	input, sdkDir := syntheticInput(t)
	base := string(input.OpenAPI)
	tests := map[string]struct {
		spec string
		code string
	}{
		"reused request schema": {
			spec: replaceOperationBodySchema(t, base, "ThingsService_CreateThing", "              $ref: '#/components/schemas/Thing'\n"),
			code: "REQUEST_SCHEMA_REUSED",
		},
		"missing required scalar presence": {
			spec: strings.Replace(base,
				"        endpoint:\n          type: string\n          minLength: 1\n",
				"        endpoint:\n          type: integer\n", 1),
			code: "REQUIRED_SCALAR_PRESENCE_UNKNOWN",
		},
		"invalid enum zero": {
			spec: strings.Replace(base, "THING_KIND_UNSPECIFIED", "THING_KIND_NOT_SET", 1),
			code: "ENUM_ZERO_INVALID",
		},
		"top-level mask for nested oneOf": {
			spec: strings.Replace(base,
				"pattern: '^[a-z][A-Za-z0-9]*(\\.[a-z][A-Za-z0-9]*)*(,[a-z][A-Za-z0-9]*(\\.[a-z][A-Za-z0-9]*)*)*$'",
				"pattern: '^[a-z][A-Za-z0-9]*(,[a-z][A-Za-z0-9]*)*$'", 1),
			code: "UPDATE_MASK_NESTED_ONEOF_UNSUPPORTED",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			work := t.TempDir()
			candidate := filepath.Join(work, "candidate.yaml")
			if err := os.WriteFile(candidate, []byte(test.spec), 0o644); err != nil {
				t.Fatal(err)
			}
			checkErr := Check(CheckOptions{Resource: "Thing", OpenAPIPath: candidate})
			generated := filepath.Join(work, "generated")
			generateInput := input
			generateInput.OpenAPI = []byte(test.spec)
			generateErr := generateFromInput(Options{Resource: "Thing", OutputDir: generated}, generateInput, sdkDir)
			var eligibility *EligibilityError
			if !errors.As(checkErr, &eligibility) || !hasReportCode(eligibility.Report, test.code) {
				t.Fatalf("check error = %v, want %s", checkErr, test.code)
			}
			if generateErr == nil || checkErr.Error() != generateErr.Error() {
				t.Fatalf("check and generate decisions differ:\ncheck: %v\ngenerate: %v", checkErr, generateErr)
			}
			if _, err := os.Stat(generated); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("generated output exists after rejection: %v", err)
			}
		})
	}
}

func TestCheckAndGenerateRejectOptionalResponseID(t *testing.T) {
	input, sdkDir := syntheticInput(t)
	input.OpenAPI = bytes.Replace(input.OpenAPI, []byte("      required: [id, name, enabled, kind, config, status, createTime, updateTime]\n"), []byte("      required: [name, enabled, kind, config, status, createTime, updateTime]\n"), 1)
	work := t.TempDir()
	candidate := filepath.Join(work, "candidate.yaml")
	if err := os.WriteFile(candidate, input.OpenAPI, 0o644); err != nil {
		t.Fatal(err)
	}
	checkErr := Check(CheckOptions{Resource: "Thing", OpenAPIPath: candidate})
	generated := filepath.Join(work, "generated")
	generateErr := generateFromInput(Options{Resource: "Thing", OutputDir: generated}, input, sdkDir)
	if checkErr == nil || generateErr == nil || checkErr.Error() != generateErr.Error() {
		t.Fatalf("check and generate decisions differ:\ncheck: %v\ngenerate: %v", checkErr, generateErr)
	}
	var eligibility *EligibilityError
	if !errors.As(checkErr, &eligibility) || !hasReportCode(eligibility.Report, "RESOURCE_ID_OPTIONAL") {
		t.Fatalf("check error = %v, want RESOURCE_ID_OPTIONAL", checkErr)
	}
	if _, err := os.Stat(generated); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("generated output exists after optional-id rejection: %v", err)
	}
}

func TestCheckAndGenerateRejectWrappedResponse(t *testing.T) {
	input, sdkDir := syntheticInput(t)
	input.OpenAPI = wrappedCreateResponse(input.OpenAPI)
	work := t.TempDir()
	candidate := filepath.Join(work, "candidate.yaml")
	if err := os.WriteFile(candidate, input.OpenAPI, 0o644); err != nil {
		t.Fatal(err)
	}
	checkErr := Check(CheckOptions{Resource: "Thing", OpenAPIPath: candidate})
	generated := filepath.Join(work, "generated")
	generateErr := generateFromInput(Options{Resource: "Thing", OutputDir: generated}, input, sdkDir)
	if checkErr == nil || generateErr == nil || checkErr.Error() != generateErr.Error() {
		t.Fatalf("check and generate decisions differ:\ncheck: %v\ngenerate: %v", checkErr, generateErr)
	}
	var eligibility *EligibilityError
	if !errors.As(checkErr, &eligibility) || !hasReportCode(eligibility.Report, "RESPONSE_WRAPPER_UNSUPPORTED") {
		t.Fatalf("check error = %v, want RESPONSE_WRAPPER_UNSUPPORTED", checkErr)
	}
	if _, err := os.Stat(generated); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("generated output exists after wrapper rejection: %v", err)
	}
}

func TestCheckAndGenerateRejectOptionalCreateRequiredUpdate(t *testing.T) {
	input, sdkDir := syntheticInput(t)
	input.OpenAPI = optionalCreateRequiredUpdateOpenAPI(t, input.OpenAPI)
	work := t.TempDir()
	candidate := filepath.Join(work, "candidate.yaml")
	if err := os.WriteFile(candidate, input.OpenAPI, 0o644); err != nil {
		t.Fatal(err)
	}
	checkErr := Check(CheckOptions{Resource: "Thing", OpenAPIPath: candidate})
	generated := filepath.Join(work, "generated")
	generateErr := generateFromInput(Options{Resource: "Thing", OutputDir: generated}, input, sdkDir)
	if checkErr == nil || generateErr == nil || checkErr.Error() != generateErr.Error() {
		t.Fatalf("check and generate decisions differ:\ncheck: %v\ngenerate: %v", checkErr, generateErr)
	}
	var eligibility *EligibilityError
	if !errors.As(checkErr, &eligibility) || !hasReportCode(eligibility.Report, "FIELD_REQUIREDNESS_UNSUPPORTED") {
		t.Fatalf("check error = %v, want FIELD_REQUIREDNESS_UNSUPPORTED", checkErr)
	}
	if _, err := os.Stat(generated); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("generated output exists after requiredness rejection: %v", err)
	}
}

func TestIntegerResourceIDs(t *testing.T) {
	tests := []struct {
		format string
		goType string
		tfType string
		bits   string
	}{
		{format: "int32", goType: "int32", tfType: "Int32", bits: "32"},
		{format: "int64", goType: "int64", tfType: "Int64", bits: "64"},
	}
	for _, test := range tests {
		t.Run(test.format, func(t *testing.T) {
			input, loadDir := copiedSyntheticInput(t)
			input.OpenAPI = integerIDOpenAPI(t, input.OpenAPI, test.format)
			candidate := filepath.Join(t.TempDir(), "candidate.yaml")
			if err := os.WriteFile(candidate, input.OpenAPI, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := Check(CheckOptions{Resource: "Thing", OpenAPIPath: candidate}); err != nil {
				t.Fatalf("check rejected %s ID: %v", test.format, err)
			}

			sdkPath := filepath.Join(input.SDKDir, "go", "openapi", "gen", "things_service", "things.go")
			sdk, err := os.ReadFile(sdkPath)
			if err != nil {
				t.Fatal(err)
			}
			sdk = bytes.Replace(sdk, []byte("Id           *string"), []byte("Id           *"+test.goType), 1)
			sdk = bytes.ReplaceAll(sdk, []byte("id string"), []byte("id "+test.goType))
			if err := os.WriteFile(sdkPath, sdk, 0o644); err != nil {
				t.Fatal(err)
			}

			out := filepath.Join(t.TempDir(), "thing")
			if err := generateFromInput(Options{Resource: "Thing", OutputDir: out}, input, loadDir); err != nil {
				t.Fatal(err)
			}
			schema, err := os.ReadFile(filepath.Join(out, "schema.go"))
			if err != nil {
				t.Fatal(err)
			}
			resource, err := os.ReadFile(filepath.Join(out, "resource.go"))
			if err != nil {
				t.Fatal(err)
			}
			for _, fragment := range []string{
				"schema." + test.tfType + "Attribute",
				strings.ToLower(test.tfType) + "planmodifier.UseStateForUnknown()",
			} {
				if !bytes.Contains(schema, []byte(fragment)) {
					t.Errorf("schema.go lacks %q", fragment)
				}
			}
			for _, fragment := range []string{
				"strconv.ParseInt(req.ID, 10, " + test.bits + ")",
				"func (r *Resource) get(ctx context.Context, id " + test.goType + ")",
				"var id types." + test.tfType,
				"return id.Value" + test.tfType + "(), diags",
			} {
				if !bytes.Contains(resource, []byte(fragment)) {
					t.Errorf("resource.go lacks %q", fragment)
				}
			}
			compileGenerated(t, out, input)
		})
	}
}

func wrappedCreateResponse(spec []byte) []byte {
	value := strings.Replace(string(spec), "$ref: '#/components/schemas/Thing'", "$ref: '#/components/schemas/CreateThingResponse'", 1)
	value = strings.Replace(value, "    DeleteThingResponse:\n", "    CreateThingResponse:\n      type: object\n      required: [thing]\n      properties:\n        thing:\n          $ref: '#/components/schemas/Thing'\n    DeleteThingResponse:\n", 1)
	return []byte(value)
}

func integerIDOpenAPI(t *testing.T, spec []byte, format string) []byte {
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

func optionalCreateRequiredUpdateOpenAPI(t *testing.T, spec []byte) []byte {
	t.Helper()
	value := strings.Replace(string(spec),
		"              required: [name, kind, config]\n",
		"              required: [kind, config]\n", 1)
	value = strings.Replace(value,
		"              title: UpdateThingRequest\n              type: object\n              required: []\n              properties:\n",
		"              title: UpdateThingRequest\n              type: object\n              required: [name]\n              properties:\n", 1)
	value = strings.Replace(value, "      required: [id, name, enabled, kind, config, status, createTime, updateTime]\n", "      required: [id, enabled, kind, config, status, createTime, updateTime]\n", 1)
	if !strings.Contains(value, "title: UpdateThingRequest\n              type: object\n              required: [name]") {
		t.Fatal("cannot build requiredness fixture")
	}
	return []byte(value)
}

func TestCheckAcceptsEligibleCandidateWithoutWriting(t *testing.T) {
	input, _ := syntheticInput(t)
	work := t.TempDir()
	candidate := filepath.Join(work, "candidate.yaml")
	if err := os.WriteFile(candidate, input.OpenAPI, 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := directoryFiles(work)
	if err != nil {
		t.Fatal(err)
	}
	if err := Check(CheckOptions{Resource: "thing", OpenAPIPath: candidate}); err != nil {
		t.Fatal(err)
	}
	after, err := directoryFiles(work)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("check changed files: before=%v after=%v", before, after)
	}
}

func TestCheckAndGenerateShareSDKShapeEligibility(t *testing.T) {
	input, sdkDir := syntheticInput(t)
	input.OpenAPI = bytes.Replace(input.OpenAPI, []byte("tags: [Things Service]"), []byte("tags: [Other Service]"), 1)
	work := t.TempDir()
	candidate := filepath.Join(work, "candidate.yaml")
	if err := os.WriteFile(candidate, input.OpenAPI, 0o644); err != nil {
		t.Fatal(err)
	}
	checkErr := Check(CheckOptions{Resource: "Thing", OpenAPIPath: candidate})
	generateErr := generateFromInput(Options{Resource: "Thing", OutputDir: filepath.Join(work, "generated")}, input, sdkDir)
	if checkErr == nil || generateErr == nil {
		t.Fatalf("check error = %v, generate error = %v; want tag eligibility errors", checkErr, generateErr)
	}
	if checkErr.Error() != generateErr.Error() {
		t.Fatalf("SDK shape decisions differ:\ncheck:\n%s\ngenerate:\n%s", checkErr, generateErr)
	}
	if !strings.Contains(checkErr.Error(), "OPERATION_TAG_INCOMPATIBLE") {
		t.Fatalf("check error lacks OPERATION_TAG_INCOMPATIBLE: %v", checkErr)
	}
}

func TestCheckRejectsRendererUnsupportedShapes(t *testing.T) {
	input, sdkDir := syntheticInput(t)
	base := string(input.OpenAPI)
	tests := map[string]struct {
		spec string
		code string
	}{
		"invalid generated identifier": {invalidGeneratedIdentifierSpec(t, base), "RENDERER_OUTPUT_INVALID"},
		"acronym name collision":       {acronymNameCollisionSpec(t, base), "TERRAFORM_NAME_COLLISION"},
		"Go field name collision":      {goNameCollisionSpec(t, base), "GO_NAME_COLLISION"},
		"Go component name collision":  {goComponentNameCollisionSpec(t, base), "GO_COMPONENT_NAME_COLLISION"},
		"unbounded uint64":             {unboundedUint64Spec(base), "UINT64_RANGE_UNSUPPORTED"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			work := t.TempDir()
			candidate := filepath.Join(work, "candidate.yaml")
			if err := os.WriteFile(candidate, []byte(test.spec), 0o644); err != nil {
				t.Fatal(err)
			}
			checkErr := Check(CheckOptions{Resource: "Thing", OpenAPIPath: candidate})
			generated := filepath.Join(work, "generated")
			generateInput := input
			generateInput.OpenAPI = []byte(test.spec)
			generateErr := generateFromInput(Options{Resource: "Thing", OutputDir: generated}, generateInput, sdkDir)
			var eligibility *EligibilityError
			if !errors.As(checkErr, &eligibility) {
				t.Fatalf("check error = %v, want EligibilityError", checkErr)
			}
			if !hasReportCode(eligibility.Report, test.code) {
				t.Fatalf("report = %v, want %s", eligibility.Report, test.code)
			}
			if generateErr == nil || checkErr.Error() != generateErr.Error() {
				t.Fatalf("check and generate decisions differ:\ncheck: %v\ngenerate: %v", checkErr, generateErr)
			}
			if _, err := os.Stat(generated); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("generated output exists after renderer rejection: %v", err)
			}
		})
	}
}

func TestCheckAllowsSetOfObjects(t *testing.T) {
	input, _ := syntheticInput(t)
	work := t.TempDir()
	candidate := filepath.Join(work, "candidate.yaml")
	if err := os.WriteFile(candidate, []byte(setOfObjectsSpec(string(input.OpenAPI))), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Check(CheckOptions{Resource: "Thing", OpenAPIPath: candidate}); err != nil {
		t.Fatalf("check set of objects: %v", err)
	}
}

func hasReportCode(report issue.Report, code string) bool {
	return slices.ContainsFunc(report, func(item issue.Issue) bool { return item.Code == code })
}

func TestCheckRejectsMissingSchemasWithoutPanic(t *testing.T) {
	input, _ := syntheticInput(t)
	base := string(input.OpenAPI)
	getWithoutSchema := replaceAfter(t, base, "operationId: ThingsService_GetThing", "            application/json:\n              schema:\n                $ref: '#/components/schemas/Thing'", "            application/json: {}")
	deleteWithoutSchema := replaceAfter(t, base, "operationId: ThingsService_DeleteThing", "            application/json:\n              schema:\n                $ref: '#/components/schemas/DeleteThingResponse'", "            application/json: {}")
	tests := map[string]string{
		"missing create body":   strings.Replace(base, "      requestBody:\n", "      x-removed-request-body:\n", 1),
		"missing update body":   replaceAfter(t, base, "operationId: ThingsService_UpdateThing", "      requestBody:\n", "      x-removed-request-body:\n"),
		"missing get schema":    getWithoutSchema,
		"missing delete schema": deleteWithoutSchema,
	}
	for name, spec := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := validateOpenAPI([]byte(spec), "Thing", model.OperationIDs{}, input.SDKModule, input.Module)
			var eligibility *EligibilityError
			if !errors.As(err, &eligibility) || len(eligibility.Report) == 0 {
				t.Fatalf("error = %v, want a structured eligibility report", err)
			}
		})
	}
}

func unboundedUint64Spec(spec string) string {
	spec = strings.ReplaceAll(spec, "                name:\n                  type: string", "                name:\n                  type: string\n                  format: uint64")
	return strings.Replace(spec, "        name:\n          type: string", "        name:\n          type: string\n          format: uint64", 1)
}

func setOfObjectsSpec(spec string) string {
	spec = strings.ReplaceAll(spec, "x-coralogix-collection: set\n                  items: {type: string}", "x-coralogix-collection: set\n                  items: {$ref: '#/components/schemas/Detail'}")
	spec = strings.Replace(spec, "x-coralogix-collection: set\n          items: {type: string}", "x-coralogix-collection: set\n          items: {$ref: '#/components/schemas/Detail'}", 1)
	return spec + `
    Detail:
      type: object
      required: [value]
      properties:
        value:
          type: string
          minLength: 1
`
}

func invalidGeneratedIdentifierSpec(t *testing.T, spec string) string {
	t.Helper()
	createField := "                name:\n                  type: string\n                  minLength: 1\n                  x-coralogix-presence: true"
	createWithDetail := createField + "\n                detail:\n                  x-coralogix-presence: true\n                  allOf:\n                    - $ref: '#/components/schemas/1FilterOperator'"
	spec = replaceAfter(t, spec, "operationId: ThingsService_CreateThing", createField, createWithDetail)
	updateField := createField
	updateWithDetail := updateField + "\n                detail:\n                  x-coralogix-presence: true\n                  allOf:\n                    - $ref: '#/components/schemas/1FilterOperator'"
	spec = replaceAfter(t, spec, "operationId: ThingsService_UpdateThing", updateField, updateWithDetail)
	responseField := "        name:\n          type: string\n          minLength: 1\n          description: The display name."
	responseWithDetail := responseField + "\n        detail:\n          $ref: '#/components/schemas/1FilterOperator'"
	spec = replaceAfter(t, spec, "    Thing:", responseField, responseWithDetail)
	return spec + `
    1FilterOperator:
      type: object
      required: [value]
      properties:
        value: {type: string, minLength: 1}
`
}

func acronymNameCollisionSpec(t *testing.T, spec string) string {
	t.Helper()
	field := "        name:\n          type: string\n          minLength: 1\n          description: The display name."
	fields := field + "\n        HTTPServer:\n          type: string\n        httpServer:\n          type: string"
	return replaceAfter(t, spec, "    Thing:", field, fields)
}

func goNameCollisionSpec(t *testing.T, spec string) string {
	t.Helper()
	field := "        name:\n          type: string\n          minLength: 1\n          description: The display name."
	fields := field + "\n        foo-bar:\n          type: string\n        foo_bar:\n          type: string"
	return replaceAfter(t, spec, "    Thing:", field, fields)
}

func goComponentNameCollisionSpec(t *testing.T, spec string) string {
	t.Helper()
	field := "        name:\n          type: string\n          minLength: 1\n          description: The display name."
	fields := field + "\n        firstDetail:\n          $ref: '#/components/schemas/FooBar'\n        secondDetail:\n          $ref: '#/components/schemas/fooBar'"
	spec = replaceAfter(t, spec, "    Thing:", field, fields)
	return spec + `
    FooBar:
      type: object
      properties:
        value: {type: string}
    fooBar:
      type: object
      properties:
        value: {type: string}
`
}

func replaceAfter(t *testing.T, value, marker, old, replacement string) string {
	t.Helper()
	index := strings.Index(value, marker)
	if index < 0 {
		t.Fatalf("cannot find marker %q", marker)
	}
	tail := strings.Replace(value[index:], old, replacement, 1)
	if tail == value[index:] {
		t.Fatalf("cannot find %q after %q", old, marker)
	}
	return value[:index] + tail
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

func TestCheckReportsAllIssuesInStableOrder(t *testing.T) {
	input, _ := syntheticInput(t)
	input.OpenAPI = bytes.ReplaceAll(input.OpenAPI, []byte("x-coralogix-presence: true"), []byte("x-removed-presence: true"))
	input.OpenAPI = bytes.ReplaceAll(input.OpenAPI, []byte("x-coralogix-collection: set"), []byte("x-removed-collection: set"))
	candidate := filepath.Join(t.TempDir(), "candidate.yaml")
	if err := os.WriteFile(candidate, input.OpenAPI, 0o644); err != nil {
		t.Fatal(err)
	}
	first := Check(CheckOptions{Resource: "Thing", OpenAPIPath: candidate})
	second := Check(CheckOptions{Resource: "Thing", OpenAPIPath: candidate})
	var eligibility *EligibilityError
	if !errors.As(first, &eligibility) {
		t.Fatalf("got %v, want EligibilityError", first)
	}
	if len(eligibility.Report) < 3 {
		t.Fatalf("got %d issues, want at least 3: %v", len(eligibility.Report), first)
	}
	if first.Error() != second.Error() {
		t.Fatalf("reports differ:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	if !reflect.DeepEqual(eligibility.Report, eligibility.Report.Normalize()) {
		t.Fatalf("report is not normalized: %#v", eligibility.Report)
	}
}

func TestPublicationFailureRestoresOutput(t *testing.T) {
	out := filepath.Join(t.TempDir(), "thing")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	prior := []byte(version.Header + "\npackage thing\n\nconst prior = true\n")
	if err := os.WriteFile(filepath.Join(out, "prior.go"), prior, 0o644); err != nil {
		t.Fatal(err)
	}
	realRename := renamePath
	calls := 0
	renamePath = func(oldPath, newPath string) error {
		calls++
		if calls == 2 {
			return errors.New("injected publication failure")
		}
		return realRename(oldPath, newPath)
	}
	t.Cleanup(func() { renamePath = realRename })
	err := publish(out, map[string][]byte{"next.go": []byte(version.Header + "\npackage thing\n")})
	if err == nil || !strings.Contains(err.Error(), "injected publication failure") {
		t.Fatalf("got %v, want injected failure", err)
	}
	got, readErr := os.ReadFile(filepath.Join(out, "prior.go"))
	if readErr != nil || !bytes.Equal(got, prior) {
		t.Fatalf("prior output was not restored: data=%q err=%v", got, readErr)
	}
}

func TestFormattingFailurePreservesOutput(t *testing.T) {
	input, sdkDir := syntheticInput(t)
	out := filepath.Join(t.TempDir(), "thing")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	prior := []byte(version.Header + "\npackage thing\n\nconst prior = true\n")
	if err := os.WriteFile(filepath.Join(out, "prior.go"), prior, 0o644); err != nil {
		t.Fatal(err)
	}
	realFormat := formatGenerated
	formatGenerated = func(string, []byte) ([]byte, error) { return nil, errors.New("injected formatting failure") }
	t.Cleanup(func() { formatGenerated = realFormat })
	err := generateFromInput(Options{Resource: "Thing", OutputDir: out}, input, sdkDir)
	if err == nil || !strings.Contains(err.Error(), "injected formatting failure") {
		t.Fatalf("got %v, want formatting failure", err)
	}
	got, readErr := os.ReadFile(filepath.Join(out, "prior.go"))
	if readErr != nil || !bytes.Equal(got, prior) {
		t.Fatalf("prior output changed: data=%q err=%v", got, readErr)
	}
}

func TestExistingNonGeneratedDirectoryIsRejected(t *testing.T) {
	out := filepath.Join(t.TempDir(), "thing")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "user.go"), []byte("package thing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := publish(out, map[string][]byte{"next.go": []byte(version.Header + "\npackage thing\n")})
	if err == nil || !strings.Contains(err.Error(), "not generator-owned") {
		t.Fatalf("got %v, want ownership error", err)
	}
}

// assertFragments reports each fragment that the generated file does not contain.
func assertFragments(t *testing.T, file string, data []byte, contract string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		if !bytes.Contains(data, []byte(fragment)) {
			t.Errorf("%s lacks %s contract fragment %q", file, contract, fragment)
		}
	}
}

func TestGeneratedPresenceCollectionAndUpdateContract(t *testing.T) {
	convert, err := os.ReadFile(filepath.Join("testdata", "golden", "thing", "convert.go"))
	if err != nil {
		t.Fatal(err)
	}
	mask, err := os.ReadFile(filepath.Join("testdata", "golden", "thing", "mask.go"))
	if err != nil {
		t.Fatal(err)
	}
	assertFragments(t, "convert.go", convert, "semantic",
		"if v.IsNull() || v.IsUnknown() {\n\t\treturn nil",
		"b := v.ValueBool()\n\treturn &b",
		"s := v.ValueString()\n\treturn &s",
		"out := []T{}",
		"types.ListNull(types.StringType)",
		"types.SetNull(types.StringType)",
		"types.MapNull(types.StringType)",
	)
	assertFragments(t, "mask.go", mask, "update",
		"if p.Equal(s)",
		"return append(mask, at)",
		"pArm == nil:\n\t\treturn append(mask, at+\".\"+sArm.api)",
		"if diags.HasError() || len(mask) == 0",
	)
	resource, err := os.ReadFile(filepath.Join("testdata", "golden", "thing", "resource.go"))
	if err != nil {
		t.Fatal(err)
	}
	assertFragments(t, "resource.go", resource, "runtime",
		`req.ProviderData.(*clientset.ClientSet)`,
		`if cxsdk.IsNotFound(err) {`,
		`resp.Diagnostics.AddWarning("Resource disappeared during read"`,
		`resp.Diagnostics.AddError("Resource no longer exists"`,
		`resp.State.RemoveResource(ctx)`,
	)
	// The framework fails an Update that removes the resource from state.
	start := bytes.Index(resource, []byte("func (r *Resource) Update("))
	end := bytes.Index(resource[start+1:], []byte("\nfunc "))
	if start < 0 || end < 0 {
		t.Fatal("resource.go lacks Update")
	}
	if bytes.Contains(resource[start:start+1+end], []byte("RemoveResource")) {
		t.Error("Update calls RemoveResource")
	}
	// The framework also fails a state upgrade that removes the resource from state.
	legacy, err := os.ReadFile(filepath.Join("testdata", "golden", "legacything", "resource.go"))
	if err != nil {
		t.Fatal(err)
	}
	start = bytes.Index(legacy, []byte("func (r *Resource) upgradeByRefresh("))
	if start < 0 {
		t.Fatal("legacything resource.go lacks upgradeByRefresh")
	}
	end = bytes.Index(legacy[start+1:], []byte("\nfunc "))
	if end < 0 {
		end = len(legacy) - start - 1
	}
	if bytes.Contains(legacy[start:start+1+end], []byte("RemoveResource")) {
		t.Error("upgradeByRefresh calls RemoveResource")
	}
}

func TestDeclaredServerDefaultIsOptionalComputed(t *testing.T) {
	input, loadDir := syntheticInput(t)
	out := filepath.Join(t.TempDir(), "thing")
	if err := generateFromInput(Options{Resource: "Thing", OutputDir: out}, input, loadDir); err != nil {
		t.Fatal(err)
	}
	schema, err := os.ReadFile(filepath.Join(out, "schema.go"))
	if err != nil {
		t.Fatal(err)
	}
	start := bytes.Index(schema, []byte("\"enabled\": schema.BoolAttribute{"))
	end := bytes.Index(schema, []byte("\"kind\": schema.StringAttribute{"))
	if start < 0 || end <= start {
		t.Fatalf("cannot find enabled attribute in schema:\n%s", schema)
	}
	enabled := schema[start:end]
	if !bytes.Contains(enabled, []byte("Optional: true")) || !bytes.Contains(enabled, []byte("Computed: true")) {
		t.Fatalf("schema does not render the declared server default as Optional + Computed:\n%s", schema)
	}
	if bytes.Contains(enabled, []byte("Default:")) {
		t.Fatal("schema renders a Terraform static default")
	}
	if !bytes.Contains(enabled, []byte("serverDefaultModifier{value: types.BoolValue(true)}")) {
		t.Fatal("schema does not render server-default reset planning")
	}
}

func TestOptionalOneOfUsesNestedValidators(t *testing.T) {
	choice := &model.Type{Kind: model.OneOf, Schema: "Choice", Fields: []*model.Field{
		{Name: "first", Type: &model.Type{Kind: model.String}},
		{Name: "second", Type: &model.Type{Kind: model.String}},
	}}
	resource := &model.Resource{Name: "Thing", Fields: []*model.ResourceField{{
		Name: "choice", Type: choice, Behavior: model.Normal,
		Create: &model.Attrs{Presence: true}, Update: &model.Attrs{Presence: true}, Get: &model.Attrs{}, InGet: true,
	}}}
	data, err := buildTFResource(resource, "thing")
	if err != nil {
		t.Fatal(err)
	}
	if len(data.ConfigValidators) != 0 {
		t.Fatalf("optional oneOf has resource validators: %v", data.ConfigValidators)
	}
	for _, child := range data.Attributes[0].Attributes {
		if len(child.Validators) != 1 || !strings.Contains(child.Validators[0], "validator.ExactlyOneOf(path.MatchRelative().AtParent().AtName") {
			t.Errorf("%s validators = %v, want nested ExactlyOneOf", child.Name, child.Validators)
		}
	}
}

func TestComputedObjectDescendantsAreComputed(t *testing.T) {
	leaf := &model.Type{Kind: model.Object, Schema: "ServerLeaf", Fields: []*model.Field{{
		Name: "code", Type: &model.Type{Kind: model.String, MinLength: int64Pointer(1)}, Attrs: model.Attrs{Required: true},
	}}}
	details := &model.Type{Kind: model.Object, Schema: "ServerDetails", Fields: []*model.Field{
		{Name: "state", Type: &model.Type{Kind: model.String, MinLength: int64Pointer(1)}, Attrs: model.Attrs{Required: true}},
		{Name: "items", Type: &model.Type{Kind: model.List, Elem: leaf, MinItems: int64Pointer(1)}, Attrs: model.Attrs{Required: true}},
		{Name: "byKey", Type: &model.Type{Kind: model.Map, Elem: leaf}, Attrs: model.Attrs{Required: true}},
	}}
	resource := &model.Resource{Name: "Thing", Fields: []*model.ResourceField{{
		Name: "details", Type: details, Behavior: model.Computed, Get: &model.Attrs{Required: true}, InGet: true,
	}}}
	data, err := buildTFResource(resource, "thing")
	if err != nil {
		t.Fatal(err)
	}
	assertComputedTree(t, data.Attributes[0])
	if got := data.Models[0].Fields[0].Type; got != "types.Object" {
		t.Fatalf("computed object model type = %q, want types.Object", got)
	}
}

func TestComputedOneOfUsesUnknownCapableModelType(t *testing.T) {
	choice := &model.Type{Kind: model.OneOf, Schema: "ServerChoice", Fields: []*model.Field{
		{Name: "first", Type: &model.Type{Kind: model.String}},
		{Name: "second", Type: &model.Type{Kind: model.String}},
	}}
	resource := &model.Resource{Name: "Thing", Fields: []*model.ResourceField{{
		Name: "choice", Type: choice, Behavior: model.Computed, Get: &model.Attrs{Required: true}, InGet: true,
	}}}
	data, err := buildTFResource(resource, "thing")
	if err != nil {
		t.Fatal(err)
	}
	if got := data.Models[0].Fields[0].Type; got != "types.Object" {
		t.Fatalf("computed oneOf model type = %q, want types.Object", got)
	}
}

// A field line applies to every place of a component. A computed-only place has no
// configuration, so the validators and the document modifier must not be copied there.
// The managed place keeps them. A computed id keeps UseStateForUnknown.
func TestComputedOnlyDropsFieldLineConfig(t *testing.T) {
	doc := &model.Type{Kind: model.Object, Schema: "Doc", Fields: []*model.Field{{
		Name: "raw", Type: &model.Type{Kind: model.String, MinLength: int64Pointer(1)},
		Behavior: model.Normal, Attrs: model.Attrs{Required: true},
		Create: &model.Attrs{Required: true}, Update: &model.Attrs{Required: true},
	}}}
	listType := &model.Type{Kind: model.List, Elem: doc}
	resource := &model.Resource{Name: "Thing", IDParam: "id", Fields: []*model.ResourceField{
		{Name: "id", Type: &model.Type{Kind: model.String}, Behavior: model.Computed, Get: &model.Attrs{Required: true}, InGet: true},
		{Name: "items", Type: listType, Behavior: model.Normal, Create: &model.Attrs{Required: true}, Update: &model.Attrs{Required: true}, Get: &model.Attrs{Required: true}, InGet: true},
		{Name: "generated", Type: listType, Behavior: model.Computed, Get: &model.Attrs{Required: true}, InGet: true},
	}}
	file := mustParse(t, "resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  Doc:\n    fields:\n      raw:\n        equality: yaml\n        validators:\n          - lengthAtLeast: 1\n")
	data, err := buildTFResourceWith(resource, "thing", file)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]*tfAttr{}
	for _, a := range data.Attributes {
		byName[a.Name] = a
	}
	managed := byName["items"].Attributes[0]
	if !slices.Contains(managed.Validators, "stringvalidator.LengthAtLeast(1)") || !slices.Contains(managed.Modifiers, "equivalentDocument{equal: yamlEqual}") {
		t.Fatalf("managed raw = validators %v modifiers %v, want LengthAtLeast and equivalentDocument", managed.Validators, managed.Modifiers)
	}
	computed := byName["generated"].Attributes[0]
	if !computedOnly(computed) || len(computed.Validators) != 0 || len(computed.Modifiers) != 0 {
		t.Fatalf("computed raw flags=(computed-only %t) validators=%v modifiers=%v, want no config checks", computedOnly(computed), computed.Validators, computed.Modifiers)
	}
	id := byName["id"]
	if !computedOnly(id) || !slices.Contains(id.Modifiers, "stringplanmodifier.UseStateForUnknown()") {
		t.Fatalf("id modifiers = %v, want computed-only with UseStateForUnknown", id.Modifiers)
	}
}

func assertComputedTree(t *testing.T, attr *tfAttr) {
	t.Helper()
	if attr.Required || attr.Optional || !attr.Computed || len(attr.Validators) != 0 {
		t.Errorf("%s flags=(required=%t optional=%t computed=%t) validators=%v, want computed-only without validators", attr.Name, attr.Required, attr.Optional, attr.Computed, attr.Validators)
	}
	for _, child := range attr.Attributes {
		assertComputedTree(t, child)
	}
}

func int64Pointer(value int64) *int64 { return &value }

func TestGeneratedRuntimeSemantics(t *testing.T) {
	input, loadDir := syntheticInput(t)
	out := filepath.Join(t.TempDir(), "thing")
	if err := generateFromInput(Options{Resource: "thing", OutputDir: out}, input, loadDir); err != nil {
		t.Fatal(err)
	}
	semantics, err := os.ReadFile(filepath.Join("testdata", "semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

// The golden output is compared as text, and Go does not build testdata. The
// existing-resource output must also compile, for example with an import for
// the static default of a nested attribute.
func TestExistingResourceOutputCompiles(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "legacy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "legacything")
	options := Options{Resource: "LegacyThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "legacy-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

// The generated Delete of a resource with a Delete override calls the archive operation, and
// treats a resource that the API does not find as deleted.
func TestDeleteOverrideRuntimeSemantics(t *testing.T) {
	input, _ := syntheticInput(t)
	out := generateArchivedThing(t)
	semantics, err := os.ReadFile(filepath.Join("testdata", "archived_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

// The generated Update of a resource whose server-set id is in the Update body sends that id.
func TestCustomMethodsRuntimeSemantics(t *testing.T) {
	input, _ := syntheticInput(t)
	out := generateCustomThing(t)
	semantics, err := os.ReadFile(filepath.Join("testdata", "customthing_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

// The generated Replace sends the id from state in its body, and Read takes the resource from a
// response field that holds it as a value.
func TestEchoedValuesRuntimeSemantics(t *testing.T) {
	input, _ := syntheticInput(t)
	out := generateEchoThing(t)
	semantics, err := os.ReadFile(filepath.Join("testdata", "echothing_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

// The generated resource with document fields compares them as documents in its plan and its
// flatten, and plans no change when only the format of a document changes.
func TestDocumentEqualityRuntimeSemantics(t *testing.T) {
	input, _ := syntheticInput(t)
	out := generateConfigThing(t)
	semantics, err := os.ReadFile(filepath.Join("testdata", "configthing_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

// The generated resource whose API holds values in wrapper objects puts each value in its wrappers
// on expand and takes it out on flatten, and keeps null apart from a zero value.
func TestWrappedValuesRuntimeSemantics(t *testing.T) {
	input, _ := syntheticInput(t)
	out := generateWrapThing(t)
	semantics, err := os.ReadFile(filepath.Join("testdata", "wrapthing_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

// requestValue on a field the request schema does not mark read-only is not a
// Terraform attribute. Create and Update must not read it. Reading a missing
// attribute fails every Create and every Update.
func TestWritableRequestValueIsNotChecked(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "customthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	readonly := "        kind:\n          allOf: [{$ref: '#/components/schemas/ThingKind'}]\n          readOnly: true\n"
	writable := "        kind:\n          allOf: [{$ref: '#/components/schemas/ThingKind'}]\n"
	if !strings.Contains(string(spec), readonly) {
		t.Fatal("the root kind field is not read-only in the fixture")
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = []byte(strings.Replace(string(spec), readonly, writable, 1))
	out := filepath.Join(t.TempDir(), "customthing")
	options := Options{Resource: "CustomThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "customthing-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	convert, err := os.ReadFile(filepath.Join(out, "convert.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(convert, []byte("out.Kind = &fixed")) {
		t.Fatal("Create no longer sends the fixed kind value")
	}
	schema, err := os.ReadFile(filepath.Join(out, "schema.go"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(schema, []byte("\t\t\t\"kind\":")) {
		t.Fatal("kind is a Terraform attribute")
	}
	check := []byte(`package customthing

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestRequestValueDoesNotBlockCreateOrUpdate(t *testing.T) {
	ctx := context.Background()
	s := Schema()
	typ := s.Type().TerraformType(ctx).(tftypes.Object)
	values := map[string]tftypes.Value{}
	for name, attrType := range typ.AttributeTypes {
		values[name] = tftypes.NewValue(attrType, nil)
	}
	raw := tftypes.NewValue(typ, values)
	if diags := validateCreate(ctx, tfsdk.Config{Schema: s, Raw: raw}, tfsdk.Plan{Schema: s, Raw: raw}); diags.HasError() {
		t.Fatalf("Create: %v", diags)
	}
	if _, diags := updateChanged(ctx, tfsdk.Config{Schema: s, Raw: raw}, tfsdk.Plan{Schema: s, Raw: raw}, tfsdk.State{Schema: s, Raw: raw}); diags.HasError() {
		t.Fatalf("Update: %v", diags)
	}
}
`)
	if err := os.WriteFile(filepath.Join(out, "request_value_test.go"), check, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

func compileGenerated(t *testing.T, out string, input source.Input) {
	t.Helper()
	// The acceptance test imports the provider package, which the synthetic provider does not have.
	if err := os.Remove(filepath.Join(out, "acceptance_test.go")); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	module := fmt.Sprintf(`module github.com/coralogix/terraform-provider-coralogix/generated-test

go 1.26.0

	require (
	example.com/iac-test-sdk v0.0.0
	github.com/coralogix/terraform-provider-coralogix v0.0.0
	github.com/hashicorp/terraform-plugin-framework v1.17.0
)

replace example.com/iac-test-sdk => %s
replace github.com/coralogix/terraform-provider-coralogix => %s
`, input.SDKDir, input.ModuleRoot)
	if err := os.WriteFile(filepath.Join(out, "go.mod"), []byte(module), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "mod", "tidy")
	command.Dir = out
	command.Env = append(os.Environ(), "GOWORK=off")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("prepare generated resource test module: %v\n%s", err, output)
	}
	command = exec.Command("go", "test", "./...")
	command.Dir = out
	command.Env = append(os.Environ(), "GOWORK=off")
	output, err = command.CombinedOutput()
	if err != nil {
		t.Fatalf("generated resource tests: %v\n%s", err, output)
	}
}

func TestSDKIssuesAreAggregated(t *testing.T) {
	input, loadDir := copiedSyntheticInput(t)
	brokenSDK := input.SDKDir
	path := filepath.Join(brokenSDK, "go", "openapi", "gen", "things_service", "things.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.ReplaceAll(data, []byte("Enabled"), []byte("MissingEnabled"))
	data = bytes.ReplaceAll(data, []byte("Destinations"), []byte("MissingDestinations"))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	input.SDKDir = brokenSDK
	err = generateFromInput(Options{Resource: "Thing", OutputDir: filepath.Join(t.TempDir(), "thing")}, input, loadDir)
	var eligibility *EligibilityError
	if !errors.As(err, &eligibility) {
		t.Fatalf("got %v, want EligibilityError", err)
	}
	count := 0
	for _, item := range eligibility.Report {
		if item.Code == "SDK_SYMBOL_MISSING" {
			count++
		}
	}
	if count < 4 {
		t.Fatalf("got %d SDK issues, want several:\n%s", count, eligibility)
	}
}

func TestOptionalSDKValuesNeedPointers(t *testing.T) {
	input, loadDir := copiedSyntheticInput(t)
	brokenSDK := input.SDKDir
	path := filepath.Join(brokenSDK, "go", "openapi", "gen", "things_service", "things.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.ReplaceAll(data, []byte("*bool"), []byte("bool"))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	input.SDKDir = brokenSDK
	err = generateFromInput(Options{Resource: "Thing", OutputDir: filepath.Join(t.TempDir(), "thing")}, input, loadDir)
	var eligibility *EligibilityError
	if !errors.As(err, &eligibility) || !strings.Contains(eligibility.Error(), "type is bool, want *bool") {
		t.Fatalf("got %v, want pointer-presence eligibility error", err)
	}
}

func TestProviderClientAccessorMustMatchSDKClient(t *testing.T) {
	input, loadDir := copiedSyntheticInput(t)
	path := filepath.Join(input.ModuleRoot, "internal", "clientset", "clientset.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data,
		[]byte("func (c *ClientSet) Things() *things_service.ThingsServiceAPIService { return c.Client }"),
		[]byte("func (*ClientSet) Wrong() string { return \"\" }"), 1)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	err = generateFromInput(Options{Resource: "Thing", OutputDir: filepath.Join(t.TempDir(), "thing")}, input, loadDir)
	var eligibility *EligibilityError
	if !errors.As(err, &eligibility) || !strings.Contains(eligibility.Error(), "SDK_SYMBOL_MISSING") {
		t.Fatalf("got %v, want provider-accessor eligibility error", err)
	}
}

func TestFullReplaceGeneration(t *testing.T) {
	input, loadDir := copiedSyntheticInput(t)
	text := strings.Replace(string(input.OpenAPI), "    patch:\n", "    put:\n", 1)
	text = strings.Replace(text, "ThingsService_UpdateThing", "ThingsService_ReplaceThing", 1)
	start := strings.Index(text, "      parameters:\n")
	end := strings.Index(text[start:], "      requestBody:\n")
	if start < 0 || end < 0 {
		t.Fatal("cannot locate updateMask query parameter")
	}
	input.OpenAPI = []byte(text[:start] + text[start+end:])
	putSDK := input.SDKDir
	path := filepath.Join(putSDK, "go", "openapi", "gen", "things_service", "things.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.ReplaceAll(data, []byte("ApiThingsServiceUpdateThingRequest"), []byte("ApiThingsServiceReplaceThingRequest"))
	data = bytes.ReplaceAll(data, []byte("ThingsServiceUpdateThing"), []byte("ThingsServiceReplaceThing"))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	input.SDKDir = putSDK
	out := filepath.Join(t.TempDir(), "thing")
	if err := generateFromInput(Options{Resource: "thing", OutputDir: out}, input, loadDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "replace.go")); err != nil {
		t.Fatalf("replace.go: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "mask.go")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("mask.go exists for PUT: %v", err)
	}
	replace, err := os.ReadFile(filepath.Join(out, "replace.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(replace, []byte("if !p.Equal(s)")) ||
		!bytes.Contains(replace, []byte("validateRequestValue(path.Root(field.attr), c, p, field.serverDefault, &diags)")) ||
		!bytes.Contains(replace, []byte("if diags.HasError() || !changed")) {
		t.Fatal("replace.go does not suppress an unchanged update")
	}
}

func TestMaskPatternContract(t *testing.T) {
	valid, leaf, err := maskRule(`^[a-z][A-Za-z0-9]*(\.[a-z][A-Za-z0-9]*)*(,[a-z][A-Za-z0-9]*(\.[a-z][A-Za-z0-9]*)*)*$`)
	if err != nil {
		t.Fatal(err)
	}
	if !leaf || !valid("object.field") || valid("*") {
		t.Fatalf("mask rule returned leaf=%t object.field=%t *=%t", leaf, valid("object.field"), valid("*"))
	}
	// The single-path pattern rejects the comma list that a two-field update sends.
	for _, bad := range []string{`^.*$`, `[`, `^[a-z][A-Za-z0-9]*(\.[a-z][A-Za-z0-9]*)*$`} {
		if _, _, err := maskRule(bad); err == nil {
			t.Errorf("maskRule(%q) accepted an unsafe pattern", bad)
		}
	}
}

func syntheticInput(t *testing.T) (source.Input, string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	sdkDir, err := filepath.Abs(filepath.Join("testdata", "sdk"))
	if err != nil {
		t.Fatal(err)
	}
	providerDir, err := filepath.Abs(filepath.Join("testdata", "provider"))
	if err != nil {
		t.Fatal(err)
	}
	return source.Input{ModuleRoot: providerDir, Module: providerModulePath, SDKDir: sdkDir, SDKModule: "example.com/iac-test-sdk", OpenAPI: data}, providerDir
}

func copiedSyntheticInput(t *testing.T) (source.Input, string) {
	t.Helper()
	input, _ := syntheticInput(t)
	root := t.TempDir()
	sdkDir := filepath.Join(root, "sdk")
	providerDir := filepath.Join(root, "provider")
	if err := copyTree(input.SDKDir, sdkDir); err != nil {
		t.Fatal(err)
	}
	if err := copyTree(input.ModuleRoot, providerDir); err != nil {
		t.Fatal(err)
	}
	input.SDKDir = sdkDir
	input.ModuleRoot = providerDir
	return input, providerDir
}

func compareDirectories(wantDir, gotDir string) string {
	want, wantErr := directoryFiles(wantDir)
	got, gotErr := directoryFiles(gotDir)
	if wantErr != nil || gotErr != nil {
		return fmt.Sprintf("read directories: want=%v got=%v", wantErr, gotErr)
	}
	var added, removed, changed []string
	for name, data := range got {
		wantData, ok := want[name]
		switch {
		case !ok:
			added = append(added, name)
		case !bytes.Equal(data, wantData):
			changed = append(changed, name)
		}
	}
	for name := range want {
		if _, ok := got[name]; !ok {
			removed = append(removed, name)
		}
	}
	slices.Sort(added)
	slices.Sort(removed)
	slices.Sort(changed)
	if len(added)+len(removed)+len(changed) == 0 {
		return ""
	}
	return fmt.Sprintf("added: %v\nremoved: %v\nchanged: %v", added, removed, changed)
}

func directoryFiles(dir string) (map[string][]byte, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		out[entry.Name()] = data
	}
	return out, nil
}

func copyDirectory(from, to string) error {
	if err := os.MkdirAll(to, 0o755); err != nil {
		return err
	}
	files, err := directoryFiles(from)
	if err != nil {
		return err
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(to, name), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func copyTree(from, to string) error {
	return filepath.WalkDir(from, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func TestSingletonGeneration(t *testing.T) {
	input, loadDir := syntheticInput(t)
	spec, err := os.ReadFile(filepath.Join("testdata", "singleton.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "settings")
	if err := generateFromInput(Options{Resource: "settings", OutputDir: out}, input, loadDir); err != nil {
		t.Fatal(err)
	}
	schema, err := os.ReadFile(filepath.Join(out, "schema.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(schema), "stringdefault.StaticString(TypeName)") || strings.Contains(string(schema), "UseStateForUnknown") {
		t.Fatalf("singleton id must have a static default and no plan modifier:\n%s", schema)
	}
	resource, err := os.ReadFile(filepath.Join(out, "resource.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(resource), "SettingsServiceGetSettings(ctx).Execute()") {
		t.Fatalf("singleton Get must have no id argument:\n%s", resource)
	}
	semantics := `package settings

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func TestSingletonIDIsStatic(t *testing.T) {
	id, ok := Schema().Attributes["id"].(schema.StringAttribute)
	if !ok || !id.Computed || id.Optional || id.Default == nil {
		t.Fatalf("id attribute = %#v, want computed-only with a default", Schema().Attributes["id"])
	}
	if TypeName != "settings" {
		t.Fatalf("TypeName = %q", TypeName)
	}
	_ = context.Background()
}
`
	if err := os.WriteFile(filepath.Join(out, "singleton_test.go"), []byte(semantics), 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

// A singleton whose API resource has a server-set id holds that id in its id attribute, and
// falls back to the type name when the API returns none.
func TestSingletonWithServerSetID(t *testing.T) {
	input, loadDir := syntheticInput(t)
	spec, err := os.ReadFile(filepath.Join("testdata", "singleton-id.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "settings")
	options := Options{Resource: "settings", OutputDir: out, OverridesPath: filepath.Join("testdata", "singleton-id-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	schema, err := os.ReadFile(filepath.Join(out, "schema.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(schema), "stringdefault.StaticString(TypeName)") {
		t.Fatalf("the id must be the id of the API, not the fixed value:\n%s", schema)
	}
	resource, err := os.ReadFile(filepath.Join(out, "resource.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(resource), "the API returned no id") || !strings.Contains(string(resource), "keep the id that the plan or the state has") || strings.Contains(string(resource), "a singleton has a fixed id") {
		t.Fatalf("flatten must keep the id of the API and fall back to the type name:\n%s", resource)
	}
	semantics := `package settings

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func TestSingletonIDIsFromTheAPI(t *testing.T) {
	id, ok := Schema().Attributes["id"].(schema.StringAttribute)
	if !ok || !id.Computed || id.Optional || id.Default != nil {
		t.Fatalf("id attribute = %#v, want computed-only with no default", Schema().Attributes["id"])
	}
}
`
	if err := os.WriteFile(filepath.Join(out, "singleton_id_test.go"), []byte(semantics), 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

// singletonID needs an id that only the server sets: a Create body that lets the user set it would
// make a value that flatten replaces.
func TestSingletonIDRejectsAWritableID(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "singleton-id.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	writable := strings.Replace(string(spec), "              properties:\n                retentionDays:", "              properties:\n                id: {type: string}\n                retentionDays:", 1)
	if writable == string(spec) {
		t.Fatal("the fixture no longer has the Create body that this test changes")
	}
	overrides, err := os.ReadFile(filepath.Join("testdata", "singleton-id-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	// Without a readOnly line, the id is a field that the Create body sets.
	err = generateSingleton(t, []byte(writable), string(overrides))
	if err == nil || !strings.Contains(err.Error(), "lets a caller set") {
		t.Fatalf("error = %v, want the id to be refused as writable", err)
	}
	// A readOnly line makes the id server-owned, when the contract does not say so.
	open := strings.Replace(writable, "          description: Identifier that the server sets.\n          readOnly: true\n", "          description: Identifier that the server sets.\n", 1)
	if open == writable {
		t.Fatal("the fixture no longer has the read-only id that this test changes")
	}
	if err := generateSingleton(t, []byte(open), string(overrides)); err == nil || !strings.Contains(err.Error(), "lets a caller set") {
		t.Fatalf("error without a readOnly line = %v, want the id to be refused as writable", err)
	}
	withLine := string(overrides) + "\ntypes:\n  Settings:\n    fields:\n      id: {readOnly: true}\n"
	if err := generateSingleton(t, []byte(open), withLine); err != nil {
		t.Fatalf("error with a readOnly line = %v, want none", err)
	}
}

// A field line cannot make the id of a singleton configurable after the checks of the contract.
func TestSingletonIDStaysComputedAfterTheFieldLines(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "singleton-id.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	overrides, err := os.ReadFile(filepath.Join("testdata", "singleton-id-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"{required: true, computed: false}", `{computed: true, default: "fixed"}`} {
		line := string(overrides) + "\ntypes:\n  Settings:\n    fields:\n      id: " + field + "\n"
		err = generateSingleton(t, spec, line)
		if err == nil || !strings.Contains(err.Error(), "must stay computed-only") {
			t.Fatalf("error with %s = %v, want the id to stay computed-only", field, err)
		}
	}
}

// singletonID needs an id field.
func TestSingletonIDNeedsAnIDField(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "singleton.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	overrides, err := os.ReadFile(filepath.Join("testdata", "singleton-id-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	err = generateSingleton(t, spec, string(overrides))
	if err == nil || !strings.Contains(err.Error(), "has none") {
		t.Fatalf("error = %v, want the resource to need an id field", err)
	}
}

// generateSingleton generates the Settings singleton from a spec and an overrides text.
func generateSingleton(t *testing.T, spec []byte, overrides string) error {
	t.Helper()
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	path := filepath.Join(t.TempDir(), "behavior-overrides.yaml")
	if err := os.WriteFile(path, []byte(overrides), 0o644); err != nil {
		t.Fatal(err)
	}
	options := Options{Resource: "settings", OutputDir: filepath.Join(t.TempDir(), "settings"), OverridesPath: path}
	return generateFromInput(options, input, loadDir)
}

// api.singletonID supports a string id: the fallback to the type name and the import are strings.
func TestSingletonIDRejectsANumericID(t *testing.T) {
	input, loadDir := syntheticInput(t)
	spec, err := os.ReadFile(filepath.Join("testdata", "singleton-id.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	numeric := strings.Replace(string(spec), "          type: string\n          description: Identifier that the server sets.", "          type: integer\n          format: int32\n          description: Identifier that the server sets.", 1)
	if numeric == string(spec) {
		t.Fatal("the fixture no longer has the id that this test changes")
	}
	input.OpenAPI = []byte(numeric)
	options := Options{Resource: "settings", OutputDir: filepath.Join(t.TempDir(), "settings"), OverridesPath: filepath.Join("testdata", "singleton-id-overrides.yaml")}
	err = generateFromInput(options, input, loadDir)
	if err == nil || !strings.Contains(err.Error(), "must be a string") {
		t.Fatalf("error = %v, want the id must be a string", err)
	}
}

// An immutable value that holds a value that the server sets compares only the
// request values. The stock RequiresReplace would compare the unknown server
// value and replace the resource on every change.
func TestImmutableServerValuesUseRequestReplace(t *testing.T) {
	settings := &model.Type{Kind: model.Object, Schema: "Settings", CreateSchema: "SettingsCreate", Fields: []*model.Field{
		{Name: "name", Type: &model.Type{Kind: model.String}, Behavior: model.Normal, Create: &model.Attrs{Required: true}},
		{Name: "id", Type: &model.Type{Kind: model.String}, Behavior: model.Computed},
	}}
	resource := &model.Resource{Name: "Thing", Fields: []*model.ResourceField{
		{Name: "settings", Type: settings, Behavior: model.Immutable, Create: &model.Attrs{}, Get: &model.Attrs{}, InGet: true},
		{Name: "region", Type: &model.Type{Kind: model.String}, Behavior: model.Immutable, Create: &model.Attrs{Presence: true}, Get: &model.Attrs{}, InGet: true},
	}}
	data, err := buildTFResource(resource, "thing")
	if err != nil {
		t.Fatal(err)
	}
	if got := data.Attributes[0].Modifiers; !slices.Equal(got, []string{`requestReplaceModifier{at: "settings"}`}) {
		t.Errorf("settings modifiers = %v, want requestReplaceModifier", got)
	}
	if got := data.Attributes[1].Modifiers; !slices.Equal(got, []string{"stringplanmodifier.RequiresReplace()"}) {
		t.Errorf("region modifiers = %v, want the stock RequiresReplace", got)
	}
	if !slices.Equal(data.RequestReplaceKinds, []string{"Object"}) || !slices.Equal(data.ServerPaths, []string{"settings.id"}) {
		t.Errorf("kinds %v, server paths %v", data.RequestReplaceKinds, data.ServerPaths)
	}
}

// A behavior-overrides line can make a nested attribute optional and computed.
// Inside an immutable value, an omitted one would be unknown after any change
// and replace the resource, so the generator rejects it.
func TestImmutableValueRejectsOptionalComputedOverride(t *testing.T) {
	settings := &model.Type{Kind: model.Object, Schema: "Settings", CreateSchema: "Settings", Fields: []*model.Field{
		{Name: "name", Type: &model.Type{Kind: model.String}, Behavior: model.Normal, Create: &model.Attrs{Required: true}},
		{Name: "zone", Type: &model.Type{Kind: model.String}, Behavior: model.Normal, Create: &model.Attrs{}},
	}}
	resource := &model.Resource{Name: "Thing", Fields: []*model.ResourceField{
		{Name: "settings", Type: settings, Behavior: model.Immutable, Create: &model.Attrs{}, Get: &model.Attrs{}, InGet: true},
	}}
	base := "resource: Thing\nmode: existing\nvalidators:\n  inferred: false\n"
	if _, err := buildTFResourceWith(resource, "thing", mustParse(t, base)); err != nil {
		t.Fatalf("immutable value without overrides: %v", err)
	}
	file := mustParse(t, base+"types:\n  Settings:\n    fields:\n      zone: {computed: true}\n")
	_, err := buildTFResourceWith(resource, "thing", file)
	if err == nil || !strings.Contains(err.Error(), "settings: the immutable value holds the optional and computed attribute settings.zone") {
		t.Fatalf("err = %v, want the optional and computed attribute settings.zone", err)
	}
}

// LegacyTarget is one component in the request and the response, and its id is readOnly.
// Without a skip line, the id is a computed attribute: flatten reads it, expand does not send
// it, and the output compiles against the SDK struct that has the field.
func TestSharedComponentReadOnlyFieldIsFlattenedOnly(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "legacy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	overrides, err := os.ReadFile(filepath.Join("testdata", "legacy-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	const skip = "      id: {skip: true}\n"
	if !strings.Contains(string(overrides), skip) {
		t.Fatal("legacy-overrides.yaml has no skip line for LegacyTarget.id: update the test")
	}
	overridesPath := filepath.Join(t.TempDir(), "behavior-overrides.yaml")
	if err := os.WriteFile(overridesPath, []byte(strings.Replace(string(overrides), skip, "", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "legacything")
	options := Options{Resource: "LegacyThing", OutputDir: out, OverridesPath: overridesPath}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	convert, err := os.ReadFile(filepath.Join(out, "convert.go"))
	if err != nil {
		t.Fatal(err)
	}
	expand := functionBody(t, string(convert), "func expandLegacyTarget(")
	flatten := functionBody(t, string(convert), "func flattenLegacyTarget(")
	if strings.Contains(expand, ".Id") {
		t.Errorf("expandLegacyTarget sends the readOnly id:\n%s", expand)
	}
	if !strings.Contains(flatten, "out.Id = ") {
		t.Errorf("flattenLegacyTarget does not read the id:\n%s", flatten)
	}
	schema, err := os.ReadFile(filepath.Join(out, "schema.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`"id": schema\.StringAttribute\{\s*Computed: +true,`).Match(schema) {
		t.Errorf("schema.go has no computed-only target id")
	}
	compileGenerated(t, out, input)
}

// functionBody returns the text of the Go function that starts with signature.
func functionBody(t *testing.T, source, signature string) string {
	t.Helper()
	start := strings.Index(source, signature)
	if start < 0 {
		t.Fatalf("no %s in the output", signature)
	}
	end := strings.Index(source[start:], "\n}\n")
	if end < 0 {
		t.Fatalf("%s has no end", signature)
	}
	return source[start : start+end]
}

func TestFollowUpUpdateSendsTheServerSetID(t *testing.T) {
	out, input := generateSharedWithUpdateOnly(t, func(s string) string { return s })
	resource, err := os.ReadFile(filepath.Join(out, "resource.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(resource)
	start := strings.Index(src, "func (r *Resource) applyUpdateOnly(")
	if start < 0 {
		t.Fatal("resource.go has no applyUpdateOnly")
	}
	end := strings.Index(src[start:], "\n}\n")
	if !strings.Contains(src[start:start+end], "body.Id = &id") {
		t.Fatalf("applyUpdateOnly does not send the state id:\n%s", src[start:start+end])
	}
	compileGenerated(t, out, input)
}

// The golden output of a resource whose id property has another name than the path parameter,
// and whose released attributes have other names than the API.
func TestGoldenOutputIDPropertyAndNames(t *testing.T) {
	out := generateSuppressedThing(t)
	golden := filepath.Join("testdata", "golden", "suppressedthing")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputIDPropertyAndNames -update:\n%s", diff)
	}
}

// generateSuppressedThing generates the synthetic resource modeled on alert suppression rules.
func generateSuppressedThing(t *testing.T) string {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "suppressed.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "suppressedthing")
	options := Options{Resource: "SuppressedThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "suppressed-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	return out
}

// The generated resource whose id property has another name than the path parameter sends no id on
// Create, sends the id property on Update, and reads it back as the id attribute.
func TestIDPropertyRuntimeSemantics(t *testing.T) {
	input, _ := syntheticInput(t)
	out := generateSuppressedThing(t)
	semantics, err := os.ReadFile(filepath.Join("testdata", "suppressed_semantics_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), semantics, 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

// An Update-only field with a default makes Create follow with an Update. When only the server
// sets the id and Update finds the resource by the id in the body, that follow-up Update sends
// the state id as the normal Update does.
// generateSharedWithUpdateOnly generates sharedthing with an Update-only field, compact, that has
// a server default. edit changes the behavior-overrides file.
func generateSharedWithUpdateOnly(t *testing.T, edit func(string) string) (string, source.Input) {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "sharedthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(spec)
	// The Replace body gets its own title and an Update-only field with a server default, and
	// the response component gets the field too.
	put := strings.Index(text, "    put:")
	nameLine := "                name: {type: string}\n"
	i := strings.Index(text[put:], nameLine)
	title := strings.Index(text[put:], "              title: SharedThing\n")
	if put < 0 || i < 0 || title < 0 || title > i {
		t.Fatal("sharedthing.yaml has no Replace body")
	}
	at := put + i + len(nameLine)
	text = text[:at] + "                compact: {type: boolean, default: false, x-coralogix-presence: true}\n" + text[at:]
	text = text[:put+title] + strings.Replace(text[put+title:], "title: SharedThing\n", "title: SharedThingUpdate\n", 1)
	component := "    SharedThing:\n      title: SharedThing\n"
	componentName := "        name: {type: string}\n"
	c := strings.Index(text, component)
	if c < 0 || !strings.Contains(text[c:], componentName) {
		t.Fatal("sharedthing.yaml has no SharedThing component")
	}
	n := c + strings.Index(text[c:], componentName) + len(componentName)
	text = text[:n] + "        compact: {type: boolean, x-coralogix-presence: true}\n" + text[n:]
	overrides, err := os.ReadFile(filepath.Join("testdata", "sharedthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	overridesPath := filepath.Join(t.TempDir(), "overrides.yaml")
	if err := os.WriteFile(overridesPath, []byte(edit(string(overrides))), 0o644); err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = []byte(text)
	out := filepath.Join(t.TempDir(), "sharedthing")
	options := Options{Resource: "SharedThing", OutputDir: out, OverridesPath: overridesPath}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	return out, input
}

// A renamed Update-only field keeps its released name in the follow-up Update.
func TestRenamedUpdateOnlyFieldUsesItsName(t *testing.T) {
	out, input := generateSharedWithUpdateOnly(t, func(s string) string {
		return strings.Replace(s, "  SharedThing:\n    fields:\n", "  SharedThing:\n    fields:\n      compact: {name: compact_mode}\n", 1)
	})
	resource, err := os.ReadFile(filepath.Join(out, "resource.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(resource), `{attr: "compact_mode",`) || strings.Contains(string(resource), `{attr: "compact",`) {
		t.Fatalf("updateOnlyFields does not name compact_mode:\n%s", functionBody(t, string(resource), "var updateOnlyFields"))
	}
	compileGenerated(t, out, input)
}
