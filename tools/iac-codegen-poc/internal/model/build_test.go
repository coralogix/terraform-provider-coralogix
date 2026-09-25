package model_test

import (
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/internal/model"
)

// thingSpec is a minimal spec for the resource Thing. The placeholders take
// flow-style YAML: CREATE, UPDATE, and GET are the properties of the Create
// body, the Update body (updateMask is added), and the Thing schema. EXTRA
// adds component schemas.
const thingSpec = `openapi: 3.1.0
info: {title: t, version: "1"}
paths:
  /things:
    post:
      operationId: S_CreateThing
      requestBody: {content: {application/json: {schema: {type: object, properties: CREATE}}}}
      responses: {"200": {content: {application/json: {schema: {$ref: '#/components/schemas/CreateThingResponse'}}}}}
  /things/{id}:
    parameters:
      - {name: id, in: path, required: true, schema: {type: string}}
    get:
      operationId: S_GetThing
      responses: {"200": {content: {application/json: {schema: {$ref: '#/components/schemas/GetThingResponse'}}}}}
    patch:
      operationId: S_UpdateThing
      requestBody: {content: {application/json: {schema: {type: object, properties: UPDATE}}}}
      responses: {"200": {content: {application/json: {schema: {$ref: '#/components/schemas/GetThingResponse'}}}}}
    delete:
      operationId: S_DeleteThing
      responses: {"200": {content: {application/json: {schema: {$ref: '#/components/schemas/DeleteThingResponse'}}}}}
components:
  schemas:
    Thing: {type: object, properties: GET}
    CreateThingResponse: {type: object, properties: {thing: {$ref: '#/components/schemas/Thing'}}}
    GetThingResponse: {type: object, properties: {thing: {allOf: [{$ref: '#/components/schemas/Thing'}]}}}
    DeleteThingResponse: {type: object}
EXTRA`

type thing struct {
	create, update, get, extra string
}

func (s thing) build(t *testing.T) (*model.Resource, error) {
	t.Helper()
	update := "{updateMask: {type: string}}"
	if s.update != "" {
		update = strings.Replace(s.update, "{", "{updateMask: {type: string}, ", 1)
	}
	spec := strings.NewReplacer(
		"CREATE", s.create, "UPDATE", update, "GET", s.get, "EXTRA", s.extra,
	).Replace(thingSpec)
	doc, err := model.Load([]byte(spec))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return model.Build(doc, "Thing")
}

// withField returns a spec with the base fields plus field f of the given
// schema, in Create, Update, and Get.
func withField(schema, extra string) thing {
	return thing{
		create: "{name: {type: string}, f: " + schema + "}",
		update: "{name: {type: string}, f: " + schema + "}",
		get:    "{id: {type: string}, name: {type: string}, f: " + schema + "}",
		extra:  extra,
	}
}

func TestBuildBehaviors(t *testing.T) {
	r, err := thing{
		create: "{name: {type: string}, region: {type: string}}",
		update: "{name: {type: string, x-coralogix-presence: true}}",
		get:    "{id: {type: string}, name: {type: string}, region: {type: string}}",
	}.build(t)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]model.Behavior{}
	for _, f := range r.Fields {
		got[f.Name] = f.Behavior
	}
	want := map[string]model.Behavior{"id": model.Computed, "name": model.Normal, "region": model.Immutable}
	for name, b := range want {
		if got[name] != b {
			t.Errorf("%s: behavior %q, want %q", name, got[name], b)
		}
	}
	if len(got) != len(want) {
		t.Errorf("fields %v, want %v (updateMask is not a field)", got, want)
	}
	if r.IDParam != "id" || r.Get.Response.Field != "thing" || r.Delete.Response.Field != "" {
		t.Errorf("id %q, get field %q, delete field %q", r.IDParam, r.Get.Response.Field, r.Delete.Response.Field)
	}
}

func TestBuildTypes(t *testing.T) {
	for _, c := range []struct {
		name, schema, extra, want string
	}{
		{"uint64 string", "{type: string, format: uint64}", "", "integer uint64 (wire: string)"},
		{"set", "{type: array, uniqueItems: true, x-coralogix-collection: set, items: {type: string}}", "", "set<string>"},
		{"list", "{type: array, items: {type: string}}", "", "list<string>"},
		{"enum without UNSPECIFIED", "{$ref: '#/components/schemas/E'}",
			"    E: {type: string, enum: [E_UNSPECIFIED, A, B]}\n", "enum E [A B]"},
	} {
		t.Run(c.name, func(t *testing.T) {
			r, err := withField(c.schema, c.extra).build(t)
			if err != nil {
				t.Fatal(err)
			}
			dump := model.Dump(r)
			if !strings.Contains(dump, c.want) {
				t.Errorf("dump does not contain %q:\n%s", c.want, dump)
			}
		})
	}
}

func TestBuildRejects(t *testing.T) {
	for _, c := range []struct {
		name    string
		spec    thing
		wantErr string
	}{
		{"update but not get", thing{
			create: "{name: {type: string}}",
			update: "{name: {type: string}, extra: {type: string}}",
			get:    "{id: {type: string}, name: {type: string}}",
		}, "unsupported field location"},
		{"create only", thing{
			create: "{name: {type: string}, secret: {type: string}}",
			update: "{name: {type: string}}",
			get:    "{id: {type: string}, name: {type: string}}",
		}, "unsupported field location"},
		{"type differs", thing{
			create: "{name: {type: string}}",
			update: "{name: {type: boolean}}",
			get:    "{id: {type: string}, name: {type: string}}",
		}, "update type differs"},
		{"no id field", thing{
			create: "{name: {type: string}}",
			update: "{name: {type: string}}",
			get:    "{name: {type: string}}",
		}, `no field "id"`},
		{"free-form map", withField("{type: object, additionalProperties: true}", ""), "without a value schema"},
		{"map with properties", withField("{type: object, properties: {a: {type: string}}, additionalProperties: {type: string}}", ""), "both properties and additionalProperties"},
		{"allOf with two entries", withField("{allOf: [{$ref: '#/components/schemas/A'}, {$ref: '#/components/schemas/A'}]}",
			"    A: {type: object}\n"), "allOf with 2 entries"},
		{"null union", withField("{type: [string, 'null']}", ""), "want exactly one type"},
		{"uniqueItems without set marker", withField("{type: array, uniqueItems: true, items: {type: string}}", ""), "must agree"},
		{"set marker without uniqueItems", withField("{type: array, x-coralogix-collection: set, items: {type: string}}", ""), "must agree"},
		{"recursive", withField("{$ref: '#/components/schemas/R'}",
			"    R: {type: object, properties: {child: {$ref: '#/components/schemas/R'}}}\n"), "recursive"},
		{"oneOf arm missing", withField("{$ref: '#/components/schemas/O'}",
			"    O: {type: object, oneOf: [{required: [a]}], properties: {a: {type: object}, b: {type: object}}}\n"),
			"do not match"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := c.spec.build(t)
			t.Logf("error: %v", err)
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("error = %v, want it to contain %q", err, c.wantErr)
			}
		})
	}
}
