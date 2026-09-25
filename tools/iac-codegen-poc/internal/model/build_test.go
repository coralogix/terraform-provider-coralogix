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
		{"oneOf arm is not a field", withField("{$ref: '#/components/schemas/O'}",
			"    O: {type: object, oneOf: [{required: [a]}, {required: [c]}], properties: {a: {type: object}, b: {type: object}}}\n"),
			"oneOf arm c is not a field"},
		{"oneOf arm in two groups", withField("{$ref: '#/components/schemas/O'}",
			"    O: {type: object, oneOf: [{required: [a]}], allOf: [{oneOf: [{required: [a]}, {required: [b]}]}], properties: {a: {type: object}, b: {type: object}}}\n"),
			"oneOf arm a is in two groups"},
		{"oneOf arm is required", withField("{$ref: '#/components/schemas/O'}",
			"    O: {type: object, required: [a], oneOf: [{required: [a]}, {required: [b]}], properties: {a: {type: object}, b: {type: object}, c: {type: string}}}\n"),
			"oneOf arm a has attributes"},
		{"discriminator with a mapping", withField("{$ref: '#/components/schemas/O'}",
			"    O: {type: object, discriminator: {propertyName: k, mapping: {x: '#/components/schemas/X'}}, oneOf: [{required: [a]}], properties: {a: {type: object}, k: {type: string}}}\n"),
			"discriminator is supported only"},
		{"discriminator is not a field", withField("{$ref: '#/components/schemas/O'}",
			"    O: {type: object, discriminator: {propertyName: k}, oneOf: [{required: [a]}], properties: {a: {type: object}, b: {type: string}}}\n"),
			"discriminator is supported only"},
		{"discriminator is not a string", withField("{$ref: '#/components/schemas/O'}",
			"    O: {type: object, discriminator: {propertyName: k}, oneOf: [{required: [a]}], properties: {a: {type: object}, k: {type: boolean}}}\n"),
			"discriminator is supported only"},
		{"no-arm entry lists other arms", withField("{$ref: '#/components/schemas/O'}",
			"    O: {type: object, oneOf: [{required: [a]}, {not: {anyOf: [{required: [b]}]}}], properties: {a: {type: object}, b: {type: object}}}\n"),
			"not.anyOf lists [b], want the arms [a]"},
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

// TestSurveyCollectsAll checks that Survey reports every unsupported shape,
// not only the first, and goes on into the other fields.
func TestSurveyCollectsAll(t *testing.T) {
	const spec = `openapi: 3.1.0
info: {title: t, version: "1"}
paths: {}
components:
  schemas:
    R:
      type: object
      properties:
        a: {anyOf: [{type: string}, {type: number}]}
        b:
          type: object
          properties:
            c: {not: {type: string}}
            d: {type: string}
        e: {type: string}
`
	doc, err := model.Load([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	typ, errs := model.Survey(doc, "R")
	if len(errs) != 2 {
		t.Fatalf("errors = %v, want 2", errs)
	}
	for i, want := range []string{"R.a: anyOf", "R.b.c: not"} {
		if !strings.HasPrefix(errs[i].Error(), want) {
			t.Errorf("error %d = %v, want it to start with %q", i, errs[i], want)
		}
	}
	if typ == nil || len(typ.Fields) != 3 || typ.Fields[2].Name != "e" {
		t.Errorf("type = %+v, want the 3 fields", typ)
	}
}

// TestResponseForms checks how Build reads the resource from a response: the
// resource itself (form B), one field that wraps it (form A), or an error for
// fields beside the resource.
func TestResponseForms(t *testing.T) {
	base := thing{create: "{name: {type: string}}", update: "{name: {type: string}}", get: "{id: {type: string}, name: {type: string}}"}
	build := func(from, to string) (*model.Resource, error) {
		s := base
		spec := strings.ReplaceAll(thingSpec, from, to)
		spec = strings.NewReplacer("CREATE", s.create, "UPDATE", "{updateMask: {type: string}, name: {type: string}}", "GET", s.get, "EXTRA", "").Replace(spec)
		doc, err := model.Load([]byte(spec))
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		return model.Build(doc, "Thing")
	}

	r, err := build("", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Get.Response; got.Field != "thing" || got.Direct {
		t.Errorf("form A: response = %+v, want field thing", got)
	}

	r, err = build("'#/components/schemas/GetThingResponse'", "'#/components/schemas/Thing'")
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Get.Response; !got.Direct || got.Field != "" {
		t.Errorf("form B: response = %+v, want the resource itself", got)
	}

	_, err = build("GetThingResponse: {type: object, properties: {thing:", "GetThingResponse: {type: object, properties: {createdAt: {type: string}, thing:")
	if err == nil || !strings.Contains(err.Error(), "want one that wraps Thing") {
		t.Errorf("fields beside the resource: error = %v", err)
	}
}

// singletonSpec is a singleton Settings: all operations on /settings, with no
// path parameter. VERBS lists the extra operations after get.
const singletonSpec = `openapi: 3.1.0
info: {title: t, version: "1"}
paths:
  /settings:
    get:
      operationId: S_GetSettings
      responses: {"200": {content: {application/json: {schema: {$ref: '#/components/schemas/Wrap'}}}}}
VERBS
components:
  schemas:
    Settings: {type: object, properties: {on: {type: boolean}}}
    Wrap: {type: object, properties: {settings: {$ref: '#/components/schemas/Settings'}}}
    Empty: {type: object}
`

const (
	singletonCreate = `    post:
      operationId: S_CreateSettings
      requestBody: {content: {application/json: {schema: {type: object, properties: {on: {type: boolean}}}}}}
      responses: {"200": {content: {application/json: {schema: {$ref: '#/components/schemas/Wrap'}}}}}
`
	singletonUpdate = `    patch:
      operationId: S_UpdateSettings
      requestBody: {content: {application/json: {schema: {type: object, properties: {on: {type: boolean}, updateMask: {type: string}}}}}}
      responses: {"200": {content: {application/json: {schema: {$ref: '#/components/schemas/Wrap'}}}}}
`
	singletonDelete = `    delete:
      operationId: S_DeleteSettings
      responses: {"200": {content: {application/json: {schema: {$ref: '#/components/schemas/Empty'}}}}}
`
)

func buildSingleton(t *testing.T, verbs string) (*model.Resource, error) {
	t.Helper()
	doc, err := model.Load([]byte(strings.Replace(singletonSpec, "VERBS", verbs, 1)))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return model.Build(doc, "Settings")
}

// TestSingleton checks that a Get with no path parameter is a singleton, and
// that only the Create, Get, Update, and Delete kind on one path builds (D18).
func TestSingleton(t *testing.T) {
	r, err := buildSingleton(t, singletonCreate+singletonUpdate+singletonDelete)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Singleton || r.IDParam != "" {
		t.Errorf("singleton = %t, id param = %q; want true, none", r.Singleton, r.IDParam)
	}
	if !r.Delete.Response.Empty {
		t.Error("the Delete response has no fields, want Empty")
	}

	cases := []struct{ name, verbs, want string }{
		{"Get and Update only", singletonUpdate, "a singleton (Get has no path parameter) needs Create, Get, Update, and Delete"},
		{"no Delete", singletonCreate + singletonUpdate, "needs Create, Get, Update, and Delete"},
		{"Delete on another path", singletonCreate + singletonUpdate + "  /other:\n" + singletonDelete, "a singleton has one path"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := buildSingleton(t, c.verbs)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("error = %v, want it to contain %q", err, c.want)
			}
		})
	}
}
