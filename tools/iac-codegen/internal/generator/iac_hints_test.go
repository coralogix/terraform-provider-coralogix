package generator

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/acceptance"
)

// iacAcceptance builds the acceptance data of the IacThing fixture, a contract whose fields carry
// x-coralogix-iac knobs. The fixture is the test spec of the iac module.
func iacAcceptance(t *testing.T, file *acceptance.File) *acceptanceData {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", "iacthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := model.Load(spec)
	if err != nil {
		t.Fatal(err)
	}
	r, err := model.Build(doc, "IacThing")
	if err != nil {
		t.Fatal(err)
	}
	res, err := buildTFResource(r, "iacthing")
	if err != nil {
		t.Fatal(err)
	}
	markIaC(res.Attributes, findIaCHints(r), nil)
	res.CRUD = &crudData{TypeName: "iac_thing", IDAttr: "id", Resource: "IacThing"}
	if file == nil {
		file = &acceptance.File{Resource: "IacThing"}
	}
	data, err := buildAcceptance(res, "example.com/provider", file, nil)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func mustContain(t *testing.T, name, config string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(config, want) {
			t.Errorf("%s config:\n%s\nwant %s", name, config, want)
		}
	}
}

func mustNotContain(t *testing.T, name, config string, nots ...string) {
	t.Helper()
	for _, not := range nots {
		if strings.Contains(config, not) {
			t.Errorf("%s config:\n%s\nmust not have %s", name, config, not)
		}
	}
}

// The knobs of the fixture change the generated configs. They never change the schema: the
// attributes keep the Optional and Required flags of the contract.
func TestIaCKnobsInConfigs(t *testing.T) {
	d := iacAcceptance(t, nil)

	// examples: [0] is the full value, [1] the updated one; a field with one example keeps it.
	mustContain(t, "full", d.Full.Config, `name = "alpha"`, `text = "source logs | limit 1"`, `limit = 10`, `count = 5`,
		`serial = "7"`, `ratio = 0.5`, `day = "2026-01-02"`, `tags = ["a", "b"]`)
	mustContain(t, "updated", d.Updated.Config, `name = "beta"`, `text = "source logs | limit 2"`, `limit = 20`, `count = 6`, `serial = "7"`)
	// examples of a map: its key comes from the example, the values stay made up.
	mustContain(t, "full", d.Full.Config, `colors = { cpu = {`)
	mustContain(t, "updated", d.Updated.Config, `colors = { cpu = {`)

	// conflicts_with: the full config sets priority, not note; a sweep step sets note, and none sets both.
	mustContain(t, "full", d.Full.Config, `priority = `)
	mustNotContain(t, "full", d.Full.Config, `note = `)
	sawNote := false
	for _, v := range d.Variants {
		for _, block := range strings.Split(v.Config, "\n\n") {
			if strings.Contains(block, "note = ") {
				sawNote = true
				mustNotContain(t, "variant", block, "priority = ")
			}
		}
	}
	if !sawNote {
		t.Error("no config of the sweep sets note")
	}

	// requires: priority needs mode, set to the API value that equals names.
	mustContain(t, "full", d.Full.Config, `mode = "IAC_MODE_FAST"`)
	for _, v := range d.Variants {
		for _, block := range strings.Split(v.Config, "\n\n") {
			if strings.Contains(block, "priority = ") {
				mustContain(t, "variant", block, `mode = "IAC_MODE_FAST"`)
			}
		}
	}

	// required_by_api: the minimal config has name.
	mustContain(t, "minimal", d.Minimal.Config, `name = "alpha"`)
	mustNotContain(t, "minimal", d.Minimal.Config, `priority`, `search`)
}

// arm_fixed: a sweep step that sets the fixed arm, or that follows a config that did, is a new
// resource, so Terraform creates it and does not update the arm of an existing one.
func TestIaCArmFixedUsesNewResource(t *testing.T) {
	d := iacAcceptance(t, nil)
	if len(d.Variants) == 0 {
		t.Fatal("no sweep step")
	}
	first := d.Variants[0].Config
	// The full config set the fixed arm (logs) on "test". Step 1 sets the other arm on a new resource.
	mustNotContain(t, "step 1", first, `resource "coralogix_iac_thing" "test" {`)
	if !regexp.MustCompile(`"test_1[0-9]"`).MatchString(first) {
		t.Errorf("step 1 has no new resource:\n%s", first)
	}
	mustContain(t, "step 1", first, "metrics = {")
}

// A line of the acceptance file for the same path wins over a knob.
func TestIaCKnobsLoseToAcceptanceFile(t *testing.T) {
	d := iacAcceptance(t, &acceptance.File{Resource: "IacThing", Values: map[string]string{
		"name":        `"from-file"`,
		"search.text": `"file text"`,
	}})
	mustContain(t, "full", d.Full.Config, `name = "from-file"`, `text = "file text"`)
	mustNotContain(t, "full", d.Full.Config, `"alpha"`, `limit 1`)
	// The knobs of the other fields still apply.
	mustContain(t, "full", d.Full.Config, `limit = 10`)
}

// required_by_api puts an optional field in the minimal config, and nothing else changes for
// a field without the knob.
func TestIaCRequiredByAPIInMinimalConfig(t *testing.T) {
	res := exampleResource(
		&tfAttr{Name: "name", Kind: "String", Required: true},
		&tfAttr{Name: "owner", Kind: "String", Optional: true, IaC: &model.IaCHints{RequiredByAPI: true}},
		&tfAttr{Name: "other", Kind: "String", Optional: true},
	)
	data := exampleAcceptance(t, res)
	mustContain(t, "minimal", data.Minimal.Config, `owner = `)
	mustNotContain(t, "minimal", data.Minimal.Config, `other = `)
	mustContain(t, "full", data.Full.Config, `owner = `, `other = `)
}

// A knob that names a sibling that the object does not have is ignored: the iac module and the
// plugin already report it, and the generator must not panic.
func TestIaCKnobsIgnoreUnknownSibling(t *testing.T) {
	res := exampleResource(
		&tfAttr{Name: "name", Kind: "String", Required: true},
		&tfAttr{Name: "a", Property: "a", Kind: "String", Optional: true, IaC: &model.IaCHints{
			ConflictsWith: []string{"gone"}, Requires: []model.IaCRequires{{Field: "gone"}},
		}},
	)
	data := exampleAcceptance(t, res)
	mustContain(t, "full", data.Full.Config, `a = `)
}

// The golden acceptance test of the fixture. It shows each knob in the generated Go test.
func TestGoldenAcceptanceIaCThing(t *testing.T) {
	data := iacAcceptance(t, nil)
	got, err := renderAcceptance(data)
	if err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "golden", "iacthing-acceptance", "acceptance_test.go.golden")
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != string(got) {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenAcceptanceIaCThing -update")
	}
}

// A requires knob of an arm that the config leaves out forces nothing. The config takes the first arm
// of the group, so only the requirement of that arm holds.
func TestIaCRequiresOnlyFromSelectedArm(t *testing.T) {
	enum := func(values ...string) []enumOption {
		var out []enumOption
		for _, v := range values {
			out = append(out, enumOption{API: v, TF: v})
		}
		return out
	}
	res := exampleResource(
		&tfAttr{Name: "name", Kind: "String", Required: true},
		&tfAttr{Name: "mode", Property: "mode", Kind: "String", Optional: true, EnumChoices: enum("x", "y")},
		&tfAttr{Name: "first", Property: "first", Kind: "String", Optional: true, OneOfGroup: "g", IaC: &model.IaCHints{
			Requires: []model.IaCRequires{{Field: "mode", Equals: "x", HasEquals: true}},
		}},
		&tfAttr{Name: "second", Property: "second", Kind: "String", Optional: true, OneOfGroup: "g", IaC: &model.IaCHints{
			Requires: []model.IaCRequires{{Field: "mode", Equals: "y", HasEquals: true}},
		}},
	)
	data := exampleAcceptance(t, res)
	mustContain(t, "full", data.Full.Config, `first = `, `mode = "x"`)
	mustNotContain(t, "full", data.Full.Config, `second = `, `mode = "y"`)
}

// The names of the fields that the schema hides are listed for the recorder, so it redacts them
// whatever their names are (the value of an API key).
func TestSensitiveFieldNamesForTheRecorder(t *testing.T) {
	attrs := []*tfAttr{
		{Name: "name", Property: "name", Kind: "String", Required: true},
		{Name: "value", Property: "value", Kind: "String", Computed: true, Sensitive: true},
		{Name: "nested", Property: "nested", Kind: "SingleNested", Optional: true, Attributes: []*tfAttr{
			{Name: "pass", Property: "pass", Kind: "String", Optional: true, WriteOnly: true},
		}},
	}
	attrs = append(attrs, &tfAttr{Name: "service_key_wo", Kind: "String", Optional: true, WriteOnly: true, Extra: true})
	got := strings.Join(sensitiveFieldNames(attrs), " ")
	// The Terraform name, the name without "_wo", and the camelCase API name that a hook writes.
	for _, want := range []string{"pass", "value", "service_key_wo", "service_key", "serviceKey"} {
		if !strings.Contains(" "+got+" ", " "+want+" ") {
			t.Errorf("sensitive fields = %q, want %s", got, want)
		}
	}
	if strings.Contains(" "+got+" ", " name ") {
		t.Errorf("sensitive fields = %q: a plain field is listed", got)
	}
	golden, err := os.ReadFile(filepath.Join("testdata", "golden", "iacthing-acceptance", "acceptance_test.go.golden"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(golden), "failreport.Install(sensitiveFields...)") {
		t.Error("the generated test does not pass the sensitive fields to the recorder")
	}
}

// A field that conflicts with an arm that the config does not take is still set: the arm is not set,
// so nothing conflicts.
func TestIaCConflictIgnoresArmThatIsNotTaken(t *testing.T) {
	res := exampleResource(
		&tfAttr{Name: "name", Kind: "String", Required: true},
		&tfAttr{Name: "first", Property: "first", Kind: "String", Optional: true, OneOfGroup: "g"},
		&tfAttr{Name: "second", Property: "second", Kind: "String", Optional: true, OneOfGroup: "g", IaC: &model.IaCHints{ConflictsWith: []string{"plain"}}},
		&tfAttr{Name: "plain", Property: "plain", Kind: "String", Optional: true},
	)
	data := exampleAcceptance(t, res)
	// The group takes "first", so "second" is not set, and "plain" does not lose to it.
	mustContain(t, "full", data.Full.Config, `first = `, `plain = `)
	mustNotContain(t, "full", data.Full.Config, `second = `)
}

// A required field always wins a conflict: the optional field that conflicts with it is never set.
func TestIaCConflictWithRequiredField(t *testing.T) {
	res := exampleResource(
		&tfAttr{Name: "name", Property: "name", Kind: "String", Required: true},
		&tfAttr{Name: "legacy", Property: "legacy", Kind: "String", Optional: true, IaC: &model.IaCHints{ConflictsWith: []string{"name"}}},
	)
	data := exampleAcceptance(t, res)
	mustContain(t, "full", data.Full.Config, `name = `)
	mustNotContain(t, "full", data.Full.Config, `legacy = `)
	for _, v := range data.Variants {
		mustNotContain(t, "variant", v.Config, `legacy = `)
	}
}

// A field that a requires knob forces wins over a field that conflicts with it, and the field that
// lost no longer forces anything.
func TestIaCRequiresWinsOverConflict(t *testing.T) {
	res := exampleResource(
		&tfAttr{Name: "name", Kind: "String", Required: true},
		&tfAttr{Name: "owner", Property: "owner", Kind: "String", Optional: true, IaC: &model.IaCHints{Requires: []model.IaCRequires{{Field: "team"}}}},
		&tfAttr{Name: "team", Property: "team", Kind: "String", Optional: true},
		&tfAttr{Name: "group", Property: "group", Kind: "String", Optional: true, IaC: &model.IaCHints{ConflictsWith: []string{"team"}, Requires: []model.IaCRequires{{Field: "extra"}}}},
		&tfAttr{Name: "extra", Property: "extra", Kind: "String", Optional: true},
	)
	data := exampleAcceptance(t, res)
	mustContain(t, "full", data.Full.Config, `owner = `, `team = `)
	mustNotContain(t, "full", data.Full.Config, `group = `)
}

// A requires knob that names an arm of a oneOf group that the config does not take forces nothing.
func TestIaCRequiresIgnoresArmThatIsNotTaken(t *testing.T) {
	res := exampleResource(
		&tfAttr{Name: "name", Kind: "String", Required: true},
		&tfAttr{Name: "plain", Property: "plain", Kind: "String", Optional: true, IaC: &model.IaCHints{Requires: []model.IaCRequires{{Field: "second"}}}},
		&tfAttr{Name: "first", Property: "first", Kind: "String", Optional: true, OneOfGroup: "g"},
		&tfAttr{Name: "second", Property: "second", Kind: "String", Optional: true, OneOfGroup: "g"},
	)
	data := exampleAcceptance(t, res)
	mustContain(t, "full", data.Full.Config, `plain = `, `first = `)
	mustNotContain(t, "full", data.Full.Config, `second = `)
}
