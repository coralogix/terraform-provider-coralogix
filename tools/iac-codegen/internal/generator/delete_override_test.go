package generator

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/issue"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/model"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/source"
)

const archiveOperation = "ArchivedThingsService_ArchiveArchivedThing"

const archivedOverrides = "resource: ArchivedThing\nmode: existing\nvalidators:\n  inferred: false\napi:\n  delete:\n    operation: " + archiveOperation + "\n"

func archivedSpec(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "archived.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// archiveCall is the archive operation of the synthetic contract, from its path to its responses.
const archiveCall = `  /archived/v1/things/{id}/archive:
    post:
      tags: [Archived Things Service]
      operationId: ArchivedThingsService_ArchiveArchivedThing
      parameters:
        - {name: id, in: path, required: true, schema: {type: string}}
`

func replaceArchiveCall(t *testing.T, spec, replacement string) string {
	t.Helper()
	if !strings.Contains(spec, archiveCall) {
		t.Fatal("the test contract did not change: update archiveCall")
	}
	return strings.Replace(spec, archiveCall, replacement, 1)
}

func eligibilityReport(t *testing.T, err error) issue.Report {
	t.Helper()
	var eligibility *EligibilityError
	if !errors.As(err, &eligibility) {
		t.Fatalf("err = %v, want an EligibilityError", err)
	}
	return eligibility.Report
}

func hasIssue(report issue.Report, code, location string) bool {
	for _, item := range report {
		if item.Code == code && item.Location == location {
			return true
		}
	}
	return false
}

// The override names a POST on the Get path plus one segment, with the id of Get, no body, and an
// ignored response. The contract is eligible, and the Delete of the model is that operation.
func TestDeleteOverrideAcceptsAnArchiveCall(t *testing.T) {
	validated, err := validateOpenAPIWith([]byte(archivedSpec(t)), "ArchivedThing", model.OperationIDs{}, "sdk", "provider", mustParse(t, archivedOverrides))
	if err != nil {
		t.Fatal(err)
	}
	del := validated.resource.Delete
	if del.OperationID != archiveOperation || del.Method != "POST" || del.Path != "/archived/v1/things/{id}/archive" || del.Body != "" {
		t.Fatalf("delete = %+v, want the archive operation", del)
	}
	for _, ref := range validated.refs {
		if ref.Path == "delete" && ref.Kind == kindMethod && ref.Name == "ArchivedThingsServiceArchiveArchivedThing" {
			return
		}
	}
	t.Fatalf("the SDK symbol checks do not cover the archive method: %+v", validated.refs)
}

// Without the override, the contract has no Delete. The archive call is not a Delete by name.
func TestArchiveCallWithoutOverrideHasNoDelete(t *testing.T) {
	file := mustParse(t, strings.Split(archivedOverrides, "api:")[0])
	_, err := validateOpenAPIWith([]byte(archivedSpec(t)), "ArchivedThing", model.OperationIDs{}, "sdk", "provider", file)
	if report := eligibilityReport(t, err); !hasIssue(report, "OPERATION_NOT_FOUND", "paths.delete") {
		t.Fatalf("report = %v, want OPERATION_NOT_FOUND at paths.delete", report)
	}
}

func TestDeleteOverrideRejectsIncompatibleOperations(t *testing.T) {
	const op = "paths.delete." + archiveOperation
	const line = "behavior-overrides.yaml:api.delete.operation"
	tests := map[string]struct {
		call      string // replaces archiveCall; "" keeps it
		extra     string // appended to the paths of the contract
		overrides string // replaces the operation name in the overrides; "" keeps it
		ids       model.OperationIDs
		code      string // "" when the override is accepted
		location  string
	}{
		"body with fields": {
			call:     archiveCall + "      requestBody:\n        content:\n          application/json:\n            schema: {type: object, required: [], properties: {reason: {type: string}}}\n",
			code:     "DELETE_OVERRIDE_BODY_UNSUPPORTED",
			location: op + ".requestBody",
		},
		"extra required parameter": {
			call:     archiveCall + "        - {name: force, in: query, required: true, schema: {type: boolean}}\n",
			code:     "REQUIRED_PARAMETER_UNSUPPORTED",
			location: op + ".parameters.query.force",
		},
		"extra path parameter": {
			call:     strings.Replace(archiveCall, "/archive:", "/archive/{version}:", 1) + "        - {name: version, in: path, required: true, schema: {type: string}}\n",
			code:     "DELETE_OVERRIDE_ID_INCOMPATIBLE",
			location: op + ".parameters",
		},
		"different id parameter": {
			call:     strings.ReplaceAll(strings.Replace(archiveCall, "{id}", "{thingId}", 1), "name: id,", "name: thingId,"),
			code:     "DELETE_OVERRIDE_ID_INCOMPATIBLE",
			location: op + ".parameters.path.thingId",
		},
		"path deeper than one segment": {
			call:     strings.Replace(archiveCall, "/archive:", "/actions/archive:", 1),
			code:     "DELETE_OVERRIDE_PATH_INCOMPATIBLE",
			location: op,
		},
		"non-POST method": {
			call:     strings.Replace(archiveCall, "    post:", "    patch:", 1),
			code:     "DELETE_OVERRIDE_METHOD_INCOMPATIBLE",
			location: op,
		},
		"operation that does not exist": {
			overrides: "ArchivedThingsService_RetireArchivedThing",
			code:      "DELETE_OVERRIDE_OPERATION_NOT_FOUND",
			location:  line,
		},
		"resource with a DELETE": {
			extra: "    delete:\n      tags: [Archived Things Service]\n      operationId: ArchivedThingsService_DeleteArchivedThing\n" +
				"      responses:\n        '200':\n          description: ok\n          content:\n            application/json:\n              schema: {$ref: '#/components/schemas/ArchiveArchivedThingResponse'}\n",
			code:     "DELETE_OVERRIDE_UNNEEDED",
			location: line,
		},
		"resource with a DELETE and a required parameter": {
			extra: "    delete:\n      tags: [Archived Things Service]\n      operationId: ArchivedThingsService_DeleteArchivedThing\n" +
				"      parameters:\n        - {name: force, in: query, required: true, schema: {type: boolean}}\n" + "      responses:\n        '200':\n          description: ok\n          content:\n            application/json:\n              schema: {$ref: '#/components/schemas/ArchiveArchivedThingResponse'}\n",
			location: line,
		},
		"resource with a DELETE that has a request body": {
			extra: "    delete:\n      tags: [Archived Things Service]\n      operationId: ArchivedThingsService_DeleteArchivedThing\n" +
				"      requestBody:\n        content:\n          application/json:\n            schema: {type: object, required: [], properties: {reason: {type: string}}}\n" + "      responses:\n        '200':\n          description: ok\n          content:\n            application/json:\n              schema: {$ref: '#/components/schemas/ArchiveArchivedThingResponse'}\n",
			location: line,
		},
		"resource with a DELETE without a JSON response": {
			extra:    "    delete:\n      tags: [Archived Things Service]\n      operationId: ArchivedThingsService_DeleteArchivedThing\n" + "      responses:\n        '204':\n          description: deleted\n",
			location: line,
		},
		"flag names another operation": {
			ids:      model.OperationIDs{Delete: "ArchivedThingsService_DeleteArchivedThing"},
			code:     "DELETE_OVERRIDE_CONFLICT",
			location: line,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			spec := archivedSpec(t)
			if test.call != "" {
				spec = replaceArchiveCall(t, spec, test.call)
			}
			if test.extra != "" {
				// Add the extra operation to the Get path item, before its PUT.
				spec = strings.Replace(spec, "    put:\n      tags: [Archived Things Service]\n", test.extra+"    put:\n      tags: [Archived Things Service]\n", 1)
			}
			overrides := archivedOverrides
			if test.overrides != "" {
				overrides = strings.Replace(overrides, archiveOperation, test.overrides, 1)
			}
			err := checkArchived(t, spec, overrides, test.ids)
			if test.code == "" {
				if err != nil {
					t.Fatalf("err = %v, want the override accepted", err)
				}
				return
			}
			if report := eligibilityReport(t, err); !hasIssue(report, test.code, test.location) {
				t.Fatalf("report:\n%s\nwant %s at %s", report, test.code, test.location)
			}
		})
	}
}

// A DELETE with the Delete suffix that discovery could not use does not make the override stale:
// without the override, generation would fail. Here it sits on another path.
func TestDeleteOverrideStaysNeededForAnUnusableDelete(t *testing.T) {
	spec := archivedSpec(t)
	extra := "  /archived/v1/things/{id}/labels/{label}:\n    delete:\n      tags: [Archived Things Service]\n      operationId: ArchivedThingsService_DeleteArchivedThing\n" +
		"      parameters:\n        - {name: id, in: path, required: true, schema: {type: string}}\n        - {name: label, in: path, required: true, schema: {type: string}}\n" +
		"      responses:\n        '200':\n          description: ok\n          content:\n            application/json:\n              schema: {$ref: '#/components/schemas/ArchiveArchivedThingResponse'}\n"
	spec = strings.Replace(spec, "components:\n", extra+"components:\n", 1)
	if err := checkArchived(t, spec, archivedOverrides, model.OperationIDs{}); err != nil {
		t.Fatalf("err = %v, want the override accepted", err)
	}
	// Without the override, the same contract has no usable Delete.
	file := mustParse(t, strings.Split(archivedOverrides, "api:")[0])
	if _, err := validateOpenAPIWith([]byte(spec), "ArchivedThing", model.OperationIDs{}, "sdk", "provider", file); err == nil {
		t.Fatal("the contract is eligible without the override; the test DELETE must be unusable")
	}
}

// checkArchived runs check --overrides on the contract.
func checkArchived(t *testing.T, spec, overridesText string, ids model.OperationIDs) error {
	t.Helper()
	dir := t.TempDir()
	specPath, overridesPath := filepath.Join(dir, "openapi.yaml"), filepath.Join(dir, "overrides.yaml")
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overridesPath, []byte(overridesText), 0o644); err != nil {
		t.Fatal(err)
	}
	return Check(CheckOptions{Resource: "ArchivedThing", OpenAPIPath: specPath, OverridesPath: overridesPath, OperationIDs: ids})
}

// usableDelete is a DELETE on the Get path that the generator can use as the Delete.
const usableDelete = "    delete:\n      tags: [Archived Things Service]\n      operationId: ArchivedThingsService_DeleteArchivedThing\n" +
	"      responses:\n        '200':\n          description: ok\n          content:\n            application/json:\n              schema: {$ref: '#/components/schemas/ArchiveArchivedThingResponse'}\n"

// deleteMethod is the SDK method of usableDelete, for a copied test SDK.
const deleteMethod = `
type ApiArchivedThingsServiceDeleteArchivedThingRequest struct{}

func (*ArchivedThingsServiceAPIService) ArchivedThingsServiceDeleteArchivedThing(ctx context.Context, id string) ApiArchivedThingsServiceDeleteArchivedThingRequest {
	_, _ = ctx, id
	return ApiArchivedThingsServiceDeleteArchivedThingRequest{}
}

func (ApiArchivedThingsServiceDeleteArchivedThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, nil
}
`

// generate decides a stale override after the SDK symbol checks: a DELETE that the pinned SDK does
// not have cannot be the Delete, so the override stays needed. With the SDK method, it is stale.
func TestGenerateDecidesAStaleDeleteOverrideWithTheSDK(t *testing.T) {
	spec := strings.Replace(archivedSpec(t), "    put:\n      tags: [Archived Things Service]\n", usableDelete+"    put:\n      tags: [Archived Things Service]\n", 1)
	overridesPath := filepath.Join(t.TempDir(), "overrides.yaml")
	if err := os.WriteFile(overridesPath, []byte(archivedOverrides), 0o644); err != nil {
		t.Fatal(err)
	}
	generate := func(input source.Input, loadDir string) error {
		input.OpenAPI = []byte(spec)
		return generateFromInput(Options{Resource: "ArchivedThing", OutputDir: filepath.Join(t.TempDir(), "archivedthing"), OverridesPath: overridesPath}, input, loadDir)
	}
	if err := generate(syntheticInput(t)); err != nil {
		t.Fatalf("SDK without the DELETE method: err = %v, want the override accepted", err)
	}
	input, loadDir := copiedSyntheticInput(t)
	path := filepath.Join(input.SDKDir, "go", "openapi", "gen", "archived_things_service", "archived.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, deleteMethod...), 0o644); err != nil {
		t.Fatal(err)
	}
	if report := eligibilityReport(t, generate(input, loadDir)); !hasIssue(report, "DELETE_OVERRIDE_UNNEEDED", "behavior-overrides.yaml:api.delete.operation") {
		t.Fatalf("SDK with the DELETE method: report = %v, want DELETE_OVERRIDE_UNNEEDED", report)
	}
}

// check --overrides and generate share one eligibility path, so both report the same issues.
func TestCheckAndGenerateAgreeOnDeleteOverride(t *testing.T) {
	spec := replaceArchiveCall(t, archivedSpec(t), strings.Replace(archiveCall, "    post:", "    patch:", 1))
	dir := t.TempDir()
	specPath, overridesPath := filepath.Join(dir, "openapi.yaml"), filepath.Join(dir, "overrides.yaml")
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overridesPath, []byte(archivedOverrides), 0o644); err != nil {
		t.Fatal(err)
	}
	checkErr := Check(CheckOptions{Resource: "ArchivedThing", OpenAPIPath: specPath, OverridesPath: overridesPath})
	input, loadDir := syntheticInput(t)
	input.OpenAPI = []byte(spec)
	generateErr := generateFromInput(Options{Resource: "ArchivedThing", OutputDir: filepath.Join(dir, "archivedthing"), OverridesPath: overridesPath}, input, loadDir)
	checkReport, generateReport := eligibilityReport(t, checkErr), eligibilityReport(t, generateErr)
	if checkReport.Error() != generateReport.Error() {
		t.Fatalf("check:\n%s\ngenerate:\n%s", checkReport, generateReport)
	}
	if !hasIssue(checkReport, "DELETE_OVERRIDE_METHOD_INCOMPATIBLE", "paths.delete."+archiveOperation) {
		t.Fatalf("report = %v, want DELETE_OVERRIDE_METHOD_INCOMPATIBLE", checkReport)
	}
}

// The SDK symbol checks cover the method of the Delete override.
func TestDeleteOverrideNeedsTheSDKMethod(t *testing.T) {
	input, loadDir := copiedSyntheticInput(t)
	path := filepath.Join(input.SDKDir, "go", "openapi", "gen", "archived_things_service", "archived.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	broken := bytes.Replace(data, []byte(") ArchivedThingsServiceArchiveArchivedThing("), []byte(") ArchivedThingsServiceRetireArchivedThing("), 1)
	if bytes.Equal(broken, data) {
		t.Fatal("the test SDK did not change: update the replaced text")
	}
	if err := os.WriteFile(path, broken, 0o644); err != nil {
		t.Fatal(err)
	}
	input.OpenAPI = []byte(archivedSpec(t))
	overridesPath := filepath.Join(t.TempDir(), "overrides.yaml")
	if err := os.WriteFile(overridesPath, []byte(archivedOverrides), 0o644); err != nil {
		t.Fatal(err)
	}
	err = generateFromInput(Options{Resource: "ArchivedThing", OutputDir: filepath.Join(t.TempDir(), "archivedthing"), OverridesPath: overridesPath}, input, loadDir)
	if report := eligibilityReport(t, err); !hasIssue(report, "SDK_SYMBOL_MISSING", "delete") {
		t.Fatalf("report = %v, want SDK_SYMBOL_MISSING at delete", report)
	}
}
