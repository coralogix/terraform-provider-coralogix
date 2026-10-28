package generator

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"github.com/coralogix/coralogix-management-sdk/iac/sdkcheck"
)

func upsertInputs(t *testing.T) (spec, overrides string) {
	t.Helper()
	specData, err := os.ReadFile(filepath.Join("testdata", "upsert.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	overridesData, err := os.ReadFile(filepath.Join("testdata", "upsert-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(specData), string(overridesData)
}

// An upsert has the Set call as Create and Update, a full replace, and no Delete. The fields of
// the inlined objects are top-level fields that know the object that holds them in each place.
func TestUpsertModel(t *testing.T) {
	spec, overrides := upsertInputs(t)
	validated, err := validateOpenAPIWith([]byte(spec), "Target", model.OperationIDs{}, "sdk", "provider", mustParse(t, overrides))
	if err != nil {
		t.Fatal(err)
	}
	r, file := validated.resource, validated.overrides
	// The model reads the reshaped contract: the Set call is Create, its PUT copy is Update, and a
	// DELETE copy of the Get is Delete. The renderer makes no Delete call.
	if !file.Upserts() || !r.Replace || !r.Singleton || r.Create.OperationID != "UpsertService_SetTarget" || r.Update.OperationID != "UpsertService_SetTarget"+upsertPutSuffix || r.Delete.OperationID != "UpsertService_GetTarget"+upsertDeleteSuffix {
		t.Fatalf("resource = %+v, want an upsert singleton with the Set call as Create and Update", r)
	}
	crud, err := buildCRUDWith(r, validated.refs, file)
	if err != nil {
		t.Fatal(err)
	}
	// Update calls the SDK method of Set: the SDK has no method for the PUT.
	if !crud.Upsert || crud.Delete.Method != "" || crud.Update.Method != "UpsertServiceSetTarget" {
		t.Fatalf("crud = %+v, want an upsert without a Delete call", crud)
	}
	fields := map[string]*model.ResourceField{}
	for _, f := range r.Fields {
		fields[f.Name] = f
	}
	if f := fields["enabled"]; f == nil || terraformName(r.Policy, r.Name, f.Name) != "active" || file.ParentOf("create", f.Name) != nil || file.ParentOf("get", f.Name) == nil || file.ParentOf("get", f.Name).Field != "meta" || file.ParentOf("get", f.Name).Arm {
		t.Fatalf("enabled = %+v, want active, at the top of the request and in meta in the response", f)
	}
	if f := fields["name"]; f == nil || file.ParentOf("create", f.Name) == nil || file.ParentOf("create", f.Name).Field != "spec" || file.ParentOf("get", f.Name) == nil || !file.ParentOf("get", f.Name).Arm {
		t.Fatalf("name = %+v, want it in spec in the request and in the spec arm in the response", f)
	}
	if f := fields["token"]; f == nil || f.Behavior != model.Computed || f.Create != nil {
		t.Fatalf("token = %+v, want a computed field that no request sends", f)
	}
	if _, ok := fields["other"]; ok {
		t.Fatal("the skipped arm is a field")
	}
	for _, path := range []string{"fields.spec", "fields.meta", "create.body.spec"} {
		if !slices.ContainsFunc(validated.refs, func(ref sdkcheck.Ref) bool { return ref.Path == path && ref.Kind == sdkcheck.KindField }) {
			t.Errorf("refs have no field of the inlined object at %s", path)
		}
	}
}

func TestUpsertAndInlineRejectIncompatibleContracts(t *testing.T) {
	tests := map[string]struct {
		spec, overrides func(t *testing.T, spec, overrides string) (string, string)
		ids             model.OperationIDs
		code, location  string
	}{
		"other arm not skipped": {
			overrides: func(t *testing.T, s, o string) (string, string) {
				return s, replaceOnce(t, o, "      other:\n        skip: true\n", "")
			},
			code: "INLINE_UNSUPPORTED", location: "components.schemas.Target",
		},
		"inlined object optional in the request": {
			spec: func(t *testing.T, s, o string) (string, string) {
				return replaceOnce(t, s, "required: [enabled, spec]", "required: [enabled]"), o
			},
			code: "INLINE_UNSUPPORTED", location: "components.schemas.Target",
		},
		"set on another path": {
			spec: func(t *testing.T, s, o string) (string, string) {
				return replaceOnce(t, s, "    post:\n", "  /upsert/v1/target/set:\n    post:\n"), o
			},
			code: "UPSERT_SHAPE_UNSUPPORTED", location: "behavior-overrides.yaml:api.operations.create",
		},
		"operation flag": {
			ids:  model.OperationIDs{Get: "UpsertService_GetTarget"},
			code: "UPSERT_OPERATION_CONFLICT", location: "behavior-overrides.yaml:api.operations",
		},
		"line on no field": {
			overrides: func(t *testing.T, s, o string) (string, string) {
				return s, o + "      region:\n        readOnly: true\n"
			},
			code: "OVERRIDE_UNUSED", location: "behavior-overrides.yaml:types.Target.fields.region",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			spec, overrides := upsertInputs(t)
			if test.spec != nil {
				spec, overrides = test.spec(t, spec, overrides)
			}
			if test.overrides != nil {
				spec, overrides = test.overrides(t, spec, overrides)
			}
			_, err := validateOpenAPIWith([]byte(spec), "Target", test.ids, "sdk", "provider", mustParse(t, overrides))
			if report := eligibilityReport(t, err); !hasIssue(report, test.code, test.location) {
				t.Fatalf("report = %v, want %s at %s", report, test.code, test.location)
			}
		})
	}
}

// A released name cannot be the Terraform name of another attribute.
func TestReleasedNameCollision(t *testing.T) {
	spec, overrides := upsertInputs(t)
	overrides = replaceOnce(t, overrides, "name: active", "name: zone")
	_, err := validateOpenAPIWith([]byte(spec), "Target", model.OperationIDs{}, "sdk", "provider", mustParse(t, overrides))
	if err == nil || !strings.Contains(err.Error(), `name "zone" is the Terraform name of field "zone"`) {
		t.Fatalf("err = %v, want a Terraform name collision", err)
	}
}

// A static default needs a computed attribute: the Framework rejects the schema otherwise.
func TestDefaultNeedsComputed(t *testing.T) {
	spec, overrides := upsertInputs(t)
	overrides = replaceOnce(t, overrides, "        computed: true\n        default: true\n", "        default: true\n")
	_, err := validateOpenAPIWith([]byte(spec), "Target", model.OperationIDs{}, "sdk", "provider", mustParse(t, overrides))
	if report := eligibilityReport(t, err); len(report) != 1 || !strings.Contains(report[0].Message, "add computed: true") {
		t.Fatalf("report = %v, want the default to need computed: true", report)
	}
}

// A default on a field that the contract requires is an exception of the released resource, so
// the line states it with optional: true.
func TestDefaultOnARequiredFieldNeedsOptional(t *testing.T) {
	spec, overrides := upsertInputs(t)
	overrides = replaceOnce(t, overrides, "        optional: true # the contract requires it, and the released attribute has a default\n", "")
	_, err := validateOpenAPIWith([]byte(spec), "Target", model.OperationIDs{}, "sdk", "provider", mustParse(t, overrides))
	if report := eligibilityReport(t, err); len(report) != 1 || !strings.Contains(report[0].Message, "add optional: true") {
		t.Fatalf("report = %v, want the default to need optional: true", report)
	}
}

// An upsert Get with a path parameter is reported, and the checks that follow do not look for
// the Delete that an upsert does not have.
func TestUpsertGetWithPathParameterIsReported(t *testing.T) {
	spec, overrides := upsertInputs(t)
	spec = replaceOnce(t, spec, "  /upsert/v1/target:\n    get:\n", "  /upsert/v1/target/{id}:\n    get:\n      parameters:\n        - {name: id, in: path, required: true, schema: {type: string}}\n")
	spec = replaceOnce(t, spec, "    post:\n", "  /upsert/v1/target:\n    post:\n")
	spec = replaceOnce(t, spec, "        meta: {$ref: '#/components/schemas/Meta'}\n", "        meta: {$ref: '#/components/schemas/Meta'}\n        id: {type: string}\n")
	_, err := validateOpenAPIWith([]byte(spec), "Target", model.OperationIDs{}, "sdk", "provider", mustParse(t, overrides))
	if report := eligibilityReport(t, err); !hasIssue(report, "UPSERT_SHAPE_UNSUPPORTED", "behavior-overrides.yaml:api.operations.get") {
		t.Fatalf("report = %v, want UPSERT_SHAPE_UNSUPPORTED at api.operations.get", report)
	}
}

// An inlined field sits inside its object in the API, and an update mask names API fields, so
// inline needs an Update that sends the whole resource.
func TestInlineNeedsAFullReplace(t *testing.T) {
	spec, overrides := singletonWithUpdate(t, "patch")
	spec = replaceOnce(t, spec, "      operationId: UpsertService_ReplaceTarget\n", "      operationId: UpsertService_ReplaceTarget\n      parameters:\n        - {name: updateMask, in: query, required: true, schema: {type: string, pattern: '^[a-zA-Z_][a-zA-Z0-9_]*(,[a-zA-Z_][a-zA-Z0-9_]*)*$'}}\n")
	_, err := validateOpenAPIWith([]byte(spec), "Target", model.OperationIDs{}, "sdk", "provider", mustParse(t, overrides))
	if err == nil || !strings.Contains(err.Error(), "inline needs an Update that sends the whole resource") {
		t.Fatalf("err = %v, want inline to need a full replace", err)
	}
}

// Only a singleton has a fixed id: singletonID on a resource with an id path parameter is an error.
func TestFixedIDNeedsASingleton(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "sharedthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	overrides, err := os.ReadFile(filepath.Join("testdata", "sharedthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "overrides.yaml")
	if err := os.WriteFile(path, append([]byte("singletonID: {fixed: settings}\n"), overrides...), 0o644); err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	options := Options{Resource: "SharedThing", OutputDir: filepath.Join(t.TempDir(), "sharedthing"), OverridesPath: path}
	if err := generateFromInput(options, input, loadDir); err == nil || !strings.Contains(err.Error(), "singletonID is for a singleton") {
		t.Fatalf("err = %v, want fixedID to need a singleton", err)
	}
}

// inline works with a PUT too: the PUT sends the whole resource, so the inlined fields of its body
// are top-level fields, as for the Set call of an upsert.
func TestInlineWithAPut(t *testing.T) {
	spec, overrides := singletonWithUpdate(t, "put")
	validated, err := validateOpenAPIWith([]byte(spec), "Target", model.OperationIDs{}, "sdk", "provider", mustParse(t, overrides))
	if err != nil {
		t.Fatal(err)
	}
	r, file := validated.resource, validated.overrides
	if file.Upserts() || !r.Replace || r.Update.OperationID != "UpsertService_ReplaceTarget" || r.Delete.OperationID != "UpsertService_DeleteTarget" {
		t.Fatalf("resource = %+v, want a singleton with a PUT and a DELETE", r)
	}
	if parent := file.ParentOf("update", "name"); parent == nil || parent.Field != "spec" {
		t.Fatalf("update parent of name = %+v, want spec", parent)
	}
}

// singletonWithUpdate is the upsert contract as a singleton with a Create, an Update with method,
// and a Delete, and its overrides without the upsert.
func singletonWithUpdate(t *testing.T, method string) (string, string) {
	t.Helper()
	spec, overrides := upsertInputs(t)
	spec = replaceOnce(t, spec, "components:\n", `    `+method+`:
      tags: [Upsert Service]
      operationId: UpsertService_ReplaceTarget
      requestBody:
        required: true
        content:
          application/json:
            schema:
              title: Replace Target Request
              type: object
              required: [enabled, spec]
              properties:
                enabled: {type: boolean}
                spec: {$ref: '#/components/schemas/Spec'}
      responses:
        '200':
          description: ok
          content:
            application/json:
              schema: {$ref: '#/components/schemas/SetTargetResponse'}
    delete:
      tags: [Upsert Service]
      operationId: UpsertService_DeleteTarget
      responses:
        '200':
          description: ok
          content:
            application/json:
              schema: {type: object, properties: {}}
components:
`)
	overrides = replaceOnce(t, overrides, "    update: UpsertService_SetTarget\n    delete: none\n", "    update: UpsertService_ReplaceTarget\n    delete: UpsertService_DeleteTarget\n")
	return spec, overrides
}
