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
	"slices"
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/issue"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/model"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/source"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/version"
)

var updateGolden = flag.Bool("update", false, "replace the committed golden output")

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

func TestEnumCollectionFlatteningUsesGuardedHelpers(t *testing.T) {
	fields := []*convField{
		{TFName: "statuses", Model: "Statuses", SDK: "Statuses", Conv: convStrings, Collection: "List", Enum: true},
		{TFName: "status_set", Model: "StatusSet", SDK: "StatusSet", Conv: convStrings, Collection: "Set", Enum: true},
		{TFName: "status_map", Model: "StatusMap", SDK: "StatusMap", Conv: convStringMap, Enum: true},
	}
	var rendered bytes.Buffer
	for _, field := range fields {
		if err := templates.ExecuteTemplate(&rendered, "flattenField", field); err != nil {
			t.Fatal(err)
		}
	}
	for _, want := range []string{
		`flattenEnumsList(ctx, p.AtName("statuses"), v.Statuses, diags)`,
		`flattenEnumsSet(ctx, p.AtName("status_set"), v.StatusSet, diags)`,
		`flattenEnumMap(ctx, p.AtName("status_map"), v.StatusMap, diags)`,
	} {
		if !strings.Contains(rendered.String(), want) {
			t.Errorf("generated conversion does not contain %q", want)
		}
	}
}

func TestEligibilityFailurePreservesOutput(t *testing.T) {
	input, sdkDir := syntheticInput(t)
	input.OpenAPI = bytes.Replace(input.OpenAPI, []byte("x-coralogix-presence: true"), nil, 1)
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
	input.OpenAPI = bytes.Replace(input.OpenAPI, []byte("x-coralogix-presence: true"), nil, 1)
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

func TestCheckAndGenerateRejectOptionalResponseID(t *testing.T) {
	input, sdkDir := syntheticInput(t)
	input.OpenAPI = bytes.Replace(input.OpenAPI, []byte("      required: [id, name, enabled]\n"), []byte("      required: [name, enabled]\n"), 1)
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
			sdk = bytes.Replace(sdk, []byte("Id        *string"), []byte("Id        *"+test.goType), 1)
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
		"request date-time":            {requestDateTimeSpec(base), "RENDERER_SHAPE_UNSUPPORTED"},
		"set of objects":               {setOfObjectsSpec(base), "RENDERER_SHAPE_UNSUPPORTED"},
		"invalid generated identifier": {invalidGeneratedIdentifierSpec(t, base), "RENDERER_OUTPUT_INVALID"},
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
			_, err := validateOpenAPI([]byte(spec), "Thing", model.OperationIDs{}, input.SDKModule, input.ProviderModule)
			var eligibility *EligibilityError
			if !errors.As(err, &eligibility) || len(eligibility.Report) == 0 {
				t.Fatalf("error = %v, want a structured eligibility report", err)
			}
		})
	}
}

func requestDateTimeSpec(spec string) string {
	spec = strings.ReplaceAll(spec, "                name:\n                  type: string", "                name:\n                  type: string\n                  format: date-time")
	return strings.Replace(spec, "        name:\n          type: string", "        name:\n          type: string\n          format: date-time", 1)
}

func setOfObjectsSpec(spec string) string {
	spec = strings.ReplaceAll(spec, "x-coralogix-collection: set\n                  x-coralogix-presence: true\n                  items: {type: string}", "x-coralogix-collection: set\n                  x-coralogix-presence: true\n                  items: {$ref: '#/components/schemas/Detail'}")
	spec = strings.Replace(spec, "x-coralogix-collection: set\n          items: {type: string}", "x-coralogix-collection: set\n          items: {$ref: '#/components/schemas/Detail'}", 1)
	return spec + `
    Detail:
      type: object
      required: [value]
      properties:
        value: {type: string}
`
}

func invalidGeneratedIdentifierSpec(t *testing.T, spec string) string {
	t.Helper()
	createField := "                name:\n                  type: string"
	createWithDetail := createField + "\n                detail:\n                  x-coralogix-presence: true\n                  allOf:\n                    - $ref: '#/components/schemas/v3.FilterOperator'"
	spec = replaceAfter(t, spec, "operationId: ThingsService_CreateThing", createField, createWithDetail)
	updateField := createField + "\n                  x-coralogix-presence: true"
	updateWithDetail := updateField + "\n                detail:\n                  x-coralogix-presence: true\n                  allOf:\n                    - $ref: '#/components/schemas/v3.FilterOperator'"
	spec = replaceAfter(t, spec, "operationId: ThingsService_UpdateThing", updateField, updateWithDetail)
	responseField := "        name:\n          type: string\n          description: The display name."
	responseWithDetail := responseField + "\n        detail:\n          $ref: '#/components/schemas/v3.FilterOperator'"
	spec = replaceAfter(t, spec, "    Thing:", responseField, responseWithDetail)
	return spec + `
    v3.FilterOperator:
      type: object
      required: [value]
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

func TestGeneratedPresenceCollectionAndUpdateContract(t *testing.T) {
	convert, err := os.ReadFile(filepath.Join("testdata", "golden", "thing", "convert.go"))
	if err != nil {
		t.Fatal(err)
	}
	mask, err := os.ReadFile(filepath.Join("testdata", "golden", "thing", "mask.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"if v.IsNull() || v.IsUnknown() {\n\t\treturn nil",
		"b := v.ValueBool()\n\treturn &b",
		"s := v.ValueString()\n\treturn &s",
		"out := []T{}",
		"types.ListNull(types.StringType)",
		"types.SetNull(types.StringType)",
		"types.MapNull(types.StringType)",
	} {
		if !bytes.Contains(convert, []byte(fragment)) {
			t.Errorf("convert.go lacks semantic contract fragment %q", fragment)
		}
	}
	for _, fragment := range []string{
		"if p.Equal(s)",
		"return append(mask, at)",
		"pArm == nil:\n\t\treturn append(mask, at+\".\"+sArm.api)",
		"if diags.HasError() || len(mask) == 0",
	} {
		if !bytes.Contains(mask, []byte(fragment)) {
			t.Errorf("mask.go lacks update contract fragment %q", fragment)
		}
	}
	resource, err := os.ReadFile(filepath.Join("testdata", "golden", "thing", "resource.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		`req.ProviderData.(*clientset.ClientSet)`,
		`if cxsdk.Code(err) == http.StatusNotFound {`,
		`resp.Diagnostics.AddWarning("Resource disappeared during read"`,
		`resp.Diagnostics.AddWarning("Resource disappeared during update"`,
		`resp.State.RemoveResource(ctx)`,
	} {
		if !bytes.Contains(resource, []byte(fragment)) {
			t.Errorf("resource.go lacks runtime contract fragment %q", fragment)
		}
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
	end := bytes.Index(schema, []byte("\"count\": schema.Int64Attribute{"))
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
	if !bytes.Contains(enabled, []byte("serverDefaultModifier{value: types.BoolValue(false)}")) {
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

func compileGenerated(t *testing.T, out string, input source.Input) {
	t.Helper()
	module := fmt.Sprintf(`module github.com/coralogix/terraform-provider-coralogix/generated-test

go 1.26.0

	require (
	example.com/iac-test-sdk v0.0.0
	github.com/coralogix/terraform-provider-coralogix v0.0.0
	github.com/hashicorp/terraform-plugin-framework v1.17.0
)

replace example.com/iac-test-sdk => %s
replace github.com/coralogix/terraform-provider-coralogix => %s
`, input.SDKDir, input.ProviderRoot)
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
	data = bytes.ReplaceAll(data, []byte("Count"), []byte("MissingCount"))
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
	path := filepath.Join(input.ProviderRoot, "internal", "clientset", "clientset.go")
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
	valid, leaf, err := maskRule(`^[a-z][A-Za-z0-9]*(\.[a-z][A-Za-z0-9]*)*$`)
	if err != nil {
		t.Fatal(err)
	}
	if !leaf || !valid("object.field") || valid("*") {
		t.Fatalf("mask rule returned leaf=%t object.field=%t *=%t", leaf, valid("object.field"), valid("*"))
	}
	for _, bad := range []string{`^.*$`, `[`} {
		if _, _, err := maskRule(bad); err == nil {
			t.Errorf("maskRule(%q) accepted an unsafe pattern", bad)
		}
	}
}

func TestSDKLoadingIsOffline(t *testing.T) {
	t.Setenv("GOPROXY", "https://proxy.invalid")
	t.Setenv("GOSUMDB", "sum.invalid")
	env := offlineGoEnv()
	if got := environmentValue(env, "GOPROXY"); got != "off" {
		t.Fatalf("GOPROXY = %q, want off", got)
	}
	if got := environmentValue(env, "GOSUMDB"); got != "off" {
		t.Fatalf("GOSUMDB = %q, want off", got)
	}
}

func environmentValue(env []string, name string) string {
	prefix := name + "="
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			return strings.TrimPrefix(item, prefix)
		}
	}
	return ""
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
	return source.Input{ProviderRoot: providerDir, ProviderModule: source.ProviderModule, SDKDir: sdkDir, SDKModule: "example.com/iac-test-sdk", OpenAPI: data}, providerDir
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
	if err := copyTree(input.ProviderRoot, providerDir); err != nil {
		t.Fatal(err)
	}
	input.SDKDir = sdkDir
	input.ProviderRoot = providerDir
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
