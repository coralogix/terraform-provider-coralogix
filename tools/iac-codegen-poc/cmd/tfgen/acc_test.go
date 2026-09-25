package main

import (
	"os"
	"path/filepath"
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
