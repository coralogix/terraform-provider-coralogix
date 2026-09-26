package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/internal/model"
)

// accResource is the AiEvaluation model of the patched spec.
func accResource(t *testing.T) *model.Resource {
	t.Helper()
	data, err := os.ReadFile(patchedSpec)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := model.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	r, err := model.Build(doc, "AiEvaluation")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func accFile(t *testing.T, body string) *accValues {
	t.Helper()
	p := filepath.Join(t.TempDir(), "acc.yaml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	v, err := loadAccValues(p)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

const accValid = `
create:
  application: ${application}
  subsystem: ${subsystem}
  config: {sqlLoad: {joinLimit: "10", allowRecursiveCte: false}}
update:
  threshold: 0.5
  config: {toxicity: {}}
skip:
  isEnabled: "reason"
`

// TestAccConvert checks the conversion of API values to HCL: names become
// snake_case, a uint64 string becomes a number, an empty arm is {}.
func TestAccConvert(t *testing.T) {
	d, err := buildAcc(accResource(t), accFile(t, accValid))
	if err != nil {
		t.Fatal(err)
	}
	create := d.Steps[0].Config
	if !strings.Contains(create, `config = { sql_load = { allow_recursive_cte = false, join_limit = 10 } }`) {
		t.Errorf("create config:\n%s", create)
	}
	var kinds []string
	for _, s := range d.Steps {
		kinds = append(kinds, s.Kind+" "+s.Field)
	}
	// Update steps in spec order; threshold is cleared because it has presence.
	want := "create |import |update config|update threshold|clear threshold"
	if got := strings.Join(kinds, "|"); got != want {
		t.Errorf("steps = %s, want %s", got, want)
	}
	if got := strings.Join(d.Placeholders, ","); got != "application,subsystem" {
		t.Errorf("placeholders = %s", got)
	}
}

func TestAccRejects(t *testing.T) {
	cases := []struct{ name, file, want string }{
		{"unknown field", strings.Replace(accValid, "threshold: 0.5", "thresh: 0.5", 1), "update.thresh: not an Update field"},
		{"computed field", strings.Replace(accValid, "create:\n", "create:\n  companyId: x\n", 1), "create.companyId: not a Create field"},
		{"missing required", strings.Replace(accValid, "  subsystem: ${subsystem}\n", "", 1), "create.subsystem: missing"},
		{"update field not covered", strings.Replace(accValid, "  isEnabled: \"reason\"\n", "", 1), "isEnabled: an Update field needs exactly one of update and skip"},
		{"skip without reason", strings.Replace(accValid, `"reason"`, `""`, 1), "skip.isEnabled: not an Update field, or no reason"},
		{"wrong type", strings.Replace(accValid, "threshold: 0.5", "threshold: high", 1), "threshold: high (string) is not a number"},
		{"uint64 not a string", strings.Replace(accValid, `joinLimit: "10"`, "joinLimit: 10", 1), "joinLimit: 10 (int) is not a decimal string"},
		{"no such nested field", strings.Replace(accValid, "allowRecursiveCte", "recursive", 1), "sqlLoad.recursive: no such field"},
		{"two oneOf arms", strings.Replace(accValid, "{toxicity: {}}", "{toxicity: {}, sexism: {}}", 1), "a oneOf needs exactly one arm, it has 2"},
		{"bad enum", strings.Replace(accValid, "create:\n", "create:\n  target: SIDEWAYS\n", 1), "is not one of [PROMPT RESPONSE CONVERSATION]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := buildAcc(accResource(t), accFile(t, c.file))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("error = %v, want it to contain %q", err, c.want)
			}
		})
	}
}

func TestAccUnknownSection(t *testing.T) {
	p := filepath.Join(t.TempDir(), "acc.yaml")
	if err := os.WriteFile(p, []byte("creat: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadAccValues(p); err == nil {
		t.Error("no error for an unknown section")
	}
}

// TestAccIntegers checks the integer rules: a uint64 is a decimal string, an
// int32 or int64 is a JSON number in its range.
func TestAccIntegers(t *testing.T) {
	cases := []struct {
		typ     model.Type
		value   any
		want    string
		wantErr bool
	}{
		{model.Type{Kind: model.Integer, Format: "uint64", WireString: true}, "10", "10", false},
		{model.Type{Kind: model.Integer, Format: "uint64", WireString: true}, 10, "", true},
		{model.Type{Kind: model.Integer, Format: "int32"}, -5, "-5", false},
		{model.Type{Kind: model.Integer, Format: "int32"}, 1 << 31, "", true},
		{model.Type{Kind: model.Integer, Format: "int32"}, "5", "", true},
		{model.Type{Kind: model.Integer, Format: "int64"}, -1 << 40, "-1099511627776", false},
		{model.Type{Kind: model.Number, Format: "float"}, 0.1, "0.1", false},
	}
	for _, c := range cases {
		got, err := hclValue("x", &c.typ, c.value)
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("%s %s %v: got %q, %v; want %q, error %t", c.typ.Kind, c.typ.Format, c.value, got, err, c.want, c.wantErr)
		}
	}
}

// TestAccGroups checks the oneOf group rule of the values file on the fake
// resource: at most one arm, and exactly one when the group has no "no arm".
func TestAccGroups(t *testing.T) {
	data, err := os.ReadFile("../../spec/fake/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := model.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	r, err := model.Build(doc, "FakeBoard")
	if err != nil {
		t.Fatal(err)
	}
	layout := r.Fields[slices.IndexFunc(r.Fields, func(f *model.ResourceField) bool { return f.Name == "layout" })].Type
	cases := []struct {
		name  string
		value map[string]any
		want  string // "" for no error
	}{
		{"one arm in each group", map[string]any{"title": "t", "refreshOff": map[string]any{}, "absoluteTime": map[string]any{}}, ""},
		{"no arm where none is allowed", map[string]any{"title": "t", "relativeTime": map[string]any{}}, ""},
		{"two arms", map[string]any{"title": "t", "refreshOff": map[string]any{}, "refreshEvery": map[string]any{}, "relativeTime": map[string]any{}}, "at most one arm"},
		{"no arm where one is needed", map[string]any{"title": "t"}, "exactly one arm"},
	}
	for _, c := range cases {
		_, err := hclValue("layout", layout, c.value)
		if (c.want == "") != (err == nil) || (err != nil && !strings.Contains(err.Error(), c.want)) {
			t.Errorf("%s: error = %v, want %q", c.name, err, c.want)
		}
	}
	if err := checkGroups("create", r.Groups, map[string]any{"publicLink": 1, "privateShare": 2}); err == nil {
		t.Error("two root arms: no error")
	}
}

// TestAccReplace checks the values file of a full-replace resource (E11): the
// same steps as for a PATCH resource. The immutable kind cannot have an update
// value, and the id in the Update body and the readOnly createTime are not
// fields of the file.
func TestAccReplace(t *testing.T) {
	data, err := os.ReadFile("../../spec/fake/rules.yaml")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := model.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	r, err := model.Build(doc, "FakeRule")
	if err != nil {
		t.Fatal(err)
	}
	const valid = `
create: {name: n, kind: RULE_KIND_LOGS, description: d, tags: [a], condition: {query: q}}
update: {name: n2, description: d2, enabled: true, priority: 5, tags: [b], condition: {query: q2}}
`
	d, err := buildAcc(r, accFile(t, valid))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range d.Steps {
		got = append(got, s.Kind+" "+s.Field)
	}
	want := []string{"create ", "import ", "update name", "update description", "update enabled", "update priority",
		"update tags", "update condition", "clear description", "clear enabled", "clear priority", "clear tags"}
	if !slices.Equal(got, want) {
		t.Errorf("steps = %v\nwant    %v", got, want)
	}
	for _, bad := range []string{
		"create: {name: n, kind: RULE_KIND_LOGS, condition: {query: q}}\nupdate: {kind: RULE_KIND_SPANS}\n",
		"create: {name: n, kind: RULE_KIND_LOGS, condition: {query: q}, createTime: x}\n",
		"create: {name: n, kind: RULE_KIND_LOGS, condition: {query: q}}\nupdate: {id: x}\n",
	} {
		if _, err := buildAcc(r, accFile(t, bad)); err == nil {
			t.Errorf("no error for %q", bad)
		}
	}
}
