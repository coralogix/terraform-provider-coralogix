package model

import (
	"os"
	"path/filepath"
	"slices"
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
		"skip":              {func(p *Policy) { p.Skip = nil }, "FIELD_LIFECYCLE_UNSUPPORTED"},
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
