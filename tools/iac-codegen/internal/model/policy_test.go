package model

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// legacyPolicy is the rule set for testdata/legacy.yaml.
func legacyPolicy() Policy {
	return Policy{
		Existing:       true,
		RequestWrapper: "thing",
		UpdateIDInBody: true,
		ClientSetID:    true,
		EnumAnyPrefix:  []string{"legacy.Kind"},
		Skip:           []string{"LegacyTarget.id"},
		Released: []string{
			"LegacyThing.id", "LegacyThing.name", "LegacyThing.labels", "LegacyThing.rules",
			"LegacyRule.name", "LegacyRule.kind", "LegacyRule.targets",
			"LegacyLabels.env", "LegacyTarget.connectorId", "LegacyTarget.id",
		},
		NoInferredValidators: true,
	}
}

func legacyDoc(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "legacy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func legacyCodes(t *testing.T, p Policy) []string {
	t.Helper()
	doc, err := Load(legacyDoc(t))
	if err != nil {
		t.Fatal(err)
	}
	codes := reportCodes(ValidateWithPolicy(doc, "LegacyThing", OperationIDs{}, p))
	return slices.Compact(codes)
}

// A released resource is refused by the rules for a new resource, for these reasons.
func TestLegacyResourceIsRefusedWithoutPolicy(t *testing.T) {
	got := legacyCodes(t, Policy{})
	for _, want := range []string{
		"ENUM_ZERO_INVALID",
		"FIELD_LIFECYCLE_UNSUPPORTED",
		"RESOURCE_ID_OPTIONAL",
		"RESPONSE_WRAPPER_UNSUPPORTED",
		"UPDATE_ID_IN_BODY_UNSUPPORTED",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("codes %v do not contain %s", got, want)
		}
	}
}

func TestLegacyResourceIsEligibleWithPolicy(t *testing.T) {
	if got := legacyCodes(t, legacyPolicy()); len(got) != 0 {
		t.Fatalf("codes = %v, want none", got)
	}
}

// Each policy field removes its own problem and no other. This keeps a policy field from
// hiding a problem that it does not name.
func TestEachPolicyFieldRelaxesOneRule(t *testing.T) {
	tests := map[string]struct {
		without func(*Policy)
		code    string
	}{
		"request wrapper":   {func(p *Policy) { p.RequestWrapper = "" }, "FIELD_LIFECYCLE_UNSUPPORTED"},
		"update id in body": {func(p *Policy) { p.UpdateIDInBody = false }, "UPDATE_ID_IN_BODY_UNSUPPORTED"},
		"client set id":     {func(p *Policy) { p.ClientSetID = false }, "RESOURCE_ID_OPTIONAL"},
		"enum prefix":       {func(p *Policy) { p.EnumAnyPrefix = nil }, "ENUM_ZERO_INVALID"},
		"released":          {func(p *Policy) { p.Released = nil }, "FIELD_PRESENCE_UNKNOWN"},
		"existing":          {func(p *Policy) { p.Existing = false }, "RESPONSE_WRAPPER_UNSUPPORTED"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			p := legacyPolicy()
			test.without(&p)
			if got := legacyCodes(t, p); !slices.Contains(got, test.code) {
				t.Fatalf("codes = %v, want %s", got, test.code)
			}
		})
	}
}

// A released API can share a nested component between the request and the response, so the
// request component holds a read-only field. Existing mode does not send it: the field is
// computed. A new resource needs its own request component.
func TestExistingResourceComputesANestedReadOnlyRequestField(t *testing.T) {
	p := legacyPolicy()
	p.Skip = nil
	if got := legacyCodes(t, p); len(got) != 0 {
		t.Fatalf("codes = %v, want none", got)
	}
	p.Existing = false
	if got := legacyCodes(t, p); !slices.Contains(got, "FIELD_LIFECYCLE_UNSUPPORTED") {
		t.Fatalf("new resource codes = %v, want FIELD_LIFECYCLE_UNSUPPORTED", got)
	}
	doc, err := Load(legacyDoc(t))
	if err != nil {
		t.Fatal(err)
	}
	p = legacyPolicy()
	p.Skip = nil
	r, err := BuildWithPolicy(doc, "LegacyThing", OperationIDs{}, p)
	if err != nil {
		t.Fatal(err)
	}
	var target *Type
	for _, f := range r.Fields {
		if f.Name == "rules" {
			target = fieldNamed(f.Type.Elem.Fields, "targets").Type.Elem
		}
	}
	id := fieldNamed(target.Fields, "id")
	if id == nil || id.Create != nil || id.Update != nil || id.Behavior != Computed {
		t.Fatalf("targets[].id = %+v, want a computed field that no request sends", id)
	}
	connector := fieldNamed(target.Fields, "connectorId")
	if connector.Create == nil || connector.Update == nil {
		t.Fatalf("targets[].connectorId = %+v, want a field that both requests send", connector)
	}
}

// A released API can share its schema between the request and the response, so the
// contract cannot require the id that only the server sets. serverSetID states that the API
// returns it anyway, and flatten checks the id. Without the line, an optional id is refused.
func TestServerSetIDAcceptsAnOptionalID(t *testing.T) {
	p := legacyPolicy()
	p.ClientSetID = false
	if got := legacyCodes(t, p); !slices.Contains(got, "RESOURCE_ID_OPTIONAL") {
		t.Fatalf("codes without serverSetID = %v, want RESOURCE_ID_OPTIONAL", got)
	}
	// serverSetID states that only the server sets the id, so a request that lets the user set
	// it contradicts the line.
	p.ServerSetID = true
	if got := legacyCodes(t, p); !slices.Equal(got, []string{"SERVER_SET_ID_IN_REQUEST"}) {
		t.Fatalf("codes with serverSetID and a settable id = %v, want SERVER_SET_ID_IN_REQUEST", got)
	}
	p.ReadOnly = []string{"LegacyThing.id"}
	if got := legacyCodes(t, p); len(got) != 0 {
		t.Fatalf("codes with serverSetID and a read-only id = %v, want none", got)
	}
}

// legacyDocWithSiblings is testdata/legacy.yaml with a value next to the resource in the
// Create and Replace responses, as a released API returns the ids of objects the call changed.
func legacyDocWithSiblings(t *testing.T) []byte {
	t.Helper()
	data := string(legacyDoc(t))
	for _, name := range []string{"CreateLegacyThingResponse", "ReplaceLegacyThingResponse"} {
		old := "    " + name + ":\n      type: object\n      required: [thing]\n      properties:\n"
		if !strings.Contains(data, old) {
			t.Fatalf("testdata/legacy.yaml has no %s", name)
		}
		data = strings.Replace(data, old, old+"        changedIds: {type: array, items: {type: string}}\n", 1)
	}
	return []byte(data)
}

func TestExistingResourceReadsTheResourceNextToOtherResponseValues(t *testing.T) {
	doc, err := Load(legacyDocWithSiblings(t))
	if err != nil {
		t.Fatal(err)
	}
	r, err := BuildWithPolicy(doc, "LegacyThing", OperationIDs{}, legacyPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if r.Create.Response.Field != "thing" || r.Update.Response.Field != "thing" {
		t.Fatalf("create response field %q, update response field %q, want thing", r.Create.Response.Field, r.Update.Response.Field)
	}
}

func TestNewResourceRefusesOtherResponseValues(t *testing.T) {
	doc, err := Load(legacyDocWithSiblings(t))
	if err != nil {
		t.Fatal(err)
	}
	p := legacyPolicy()
	p.Existing = false
	if _, err := BuildWithPolicy(doc, "LegacyThing", OperationIDs{}, p); err == nil || !strings.Contains(err.Error(), "want one that wraps LegacyThing") {
		t.Fatalf("err = %v, want a response with one property", err)
	}
}

func TestLegacyResourceBuildsFromTheUnwrappedBody(t *testing.T) {
	doc, err := Load(legacyDoc(t))
	if err != nil {
		t.Fatal(err)
	}
	r, err := BuildWithPolicy(doc, "LegacyThing", OperationIDs{}, legacyPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if !r.Replace || r.IDParam != "id" || r.Update.Path != r.Create.Path {
		t.Fatalf("replace=%t id=%q update path=%q create path=%q", r.Replace, r.IDParam, r.Update.Path, r.Create.Path)
	}
	if r.Create.Body != "inline" || r.Create.BodyTitle != "Create Legacy Thing Request" || r.Create.Response.Field != "thing" {
		t.Fatalf("create body %q title %q response field %q", r.Create.Body, r.Create.BodyTitle, r.Create.Response.Field)
	}
	behaviors := map[string]Behavior{}
	for _, f := range r.Fields {
		behaviors[f.Name] = f.Behavior
	}
	want := map[string]Behavior{"id": Normal, "name": Normal, "labels": Normal, "rules": Normal, "createTime": Computed}
	for name, b := range want {
		if behaviors[name] != b {
			t.Errorf("field %s behavior %q, want %q", name, behaviors[name], b)
		}
	}
	if len(r.Fields) != len(want) {
		t.Errorf("fields = %v", behaviors)
	}
}

// The OpenAPI fork writes a reference with a description as allOf: [$ref]. A request body in
// that form still lets the user set the id, so serverSetID is still refused.
func TestServerSetIDChecksARequestBodyWrittenAsAllOf(t *testing.T) {
	doc, err := Load(legacyDocWithAllOfBodies(t))
	if err != nil {
		t.Fatal(err)
	}
	p := legacyPolicy()
	p.ClientSetID, p.ServerSetID = false, true
	got := slices.Compact(reportCodes(ValidateWithPolicy(doc, "LegacyThing", OperationIDs{}, p)))
	if !slices.Contains(got, "SERVER_SET_ID_IN_REQUEST") {
		t.Fatalf("codes = %v, want SERVER_SET_ID_IN_REQUEST", got)
	}
	p.ReadOnly = []string{"LegacyThing.id"}
	if got := reportCodes(ValidateWithPolicy(doc, "LegacyThing", OperationIDs{}, p)); len(got) != 0 {
		t.Fatalf("codes with a read-only id = %v, want none", got)
	}
}

// legacyDocWithAllOfBodies is testdata/legacy.yaml with the resource in the Create and Replace
// bodies written as allOf: [$ref], as the OpenAPI fork writes a reference with a description.
func legacyDocWithAllOfBodies(t *testing.T) []byte {
	t.Helper()
	old := "                thing: {$ref: '#/components/schemas/LegacyThing'}\n"
	data := string(legacyDoc(t))
	if strings.Count(data, old) != 2 {
		t.Fatalf("testdata/legacy.yaml has %d request bodies with a plain $ref, want 2", strings.Count(data, old))
	}
	return []byte(strings.ReplaceAll(data, old, "                thing: {allOf: [{$ref: '#/components/schemas/LegacyThing'}], description: The thing.}\n"))
}

// Build reads a request body written as allOf: [$ref] as the referenced schema, as validation
// does. Otherwise validation passes and every request field is modeled as not sent.
func TestBuildSendsTheFieldsOfARequestBodyWrittenAsAllOf(t *testing.T) {
	doc, err := Load(legacyDocWithAllOfBodies(t))
	if err != nil {
		t.Fatal(err)
	}
	if got := reportCodes(ValidateWithPolicy(doc, "LegacyThing", OperationIDs{}, legacyPolicy())); len(got) != 0 {
		t.Fatalf("codes = %v, want none", got)
	}
	r, err := BuildWithPolicy(doc, "LegacyThing", OperationIDs{}, legacyPolicy())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"id", "name", "labels", "rules"} {
		i := slices.IndexFunc(r.Fields, func(f *ResourceField) bool { return f.Name == name })
		if i < 0 || r.Fields[i].Create == nil || r.Fields[i].Update == nil {
			t.Errorf("field %s is not sent by Create and Update", name)
		}
	}
}

// An empty marker arm of a group that allows no arm holds no value, so skip can leave it out,
// whether the type has other fields (Object) or only the arms (OneOf).
func TestSkippableEmptyArm(t *testing.T) {
	empty := &Type{Kind: Object, Schema: "Empty"}
	detail := &Type{Kind: Object, Schema: "Detail", Fields: []*Field{{Name: "text", Type: &Type{Kind: String}}}}
	arms := []*Field{{Name: "none", Type: empty}, {Name: "detail", Type: detail}}
	pure := &Type{Kind: OneOf, Schema: "Item", Fields: arms, AllowNone: true}
	if !skippableArm(pure, "none") || skippableArm(pure, "detail") {
		t.Fatal("a pure oneOf that allows no arm: want only the empty arm skippable")
	}
	pure.AllowNone = false
	if skippableArm(pure, "none") {
		t.Fatal("a pure oneOf that needs an arm: want no arm skippable")
	}
	mixed := &Type{Kind: Object, Schema: "Item", Fields: append([]*Field{{Name: "key", Type: &Type{Kind: String}}}, arms...),
		Groups: []OneOfGroup{{Arms: []string{"none", "detail"}, AllowNone: true}}}
	if !skippableArm(mixed, "none") || skippableArm(mixed, "detail") || skippableArm(mixed, "key") {
		t.Fatal("an object with a group that allows no arm: want only the empty arm skippable")
	}
}
