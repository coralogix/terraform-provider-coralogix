package overrides

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/model"
)

const valid = `
resource: Thing
mode: existing
validators:
  inferred: false
api:
  requestWrapper: thing
  updateIDInBody: true
  clientSetID: true
types:
  Labels:
    required: []
  Target:
    fields:
      id: {skip: true}
enums:
  Kind: {zero: unspecified}
`

func TestParseAndPolicy(t *testing.T) {
	f, err := Parse([]byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	want := model.Policy{
		Existing:       true,
		RequestWrapper: "thing",
		UpdateIDInBody: true,
		ClientSetID:    true,
		EnumAnyPrefix:  []string{"Kind"},
		Skip:           []string{"Target.id"},
		Released:       []string{"Target.id"},

		NoInferredValidators: true,
	}
	if got := f.Policy(); !reflect.DeepEqual(got, want) {
		t.Fatalf("policy = %+v, want %+v", got, want)
	}
	var lines []string
	for _, l := range f.Lines() {
		lines = append(lines, l.String())
	}
	if got, want := strings.Join(lines, ","), "enums.Kind,types.Labels.required,types.Target.fields.id"; got != want {
		t.Fatalf("lines = %s, want %s", got, want)
	}
}

func TestPolicyCopiesBodyMask(t *testing.T) {
	text := `
resource: Thing
mode: existing
validators:
  inferred: false
api:
  updateMaskInBody: true
`
	f, err := Parse([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	if !f.Policy().UpdateMaskInBody {
		t.Fatal("policy did not copy updateMaskInBody")
	}
}

func TestParseIsStrict(t *testing.T) {
	tests := map[string]struct {
		text string
		want string
	}{
		"empty file":                        {"", "empty"},
		"unknown key":                       {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\nbogus: 1\n", "bogus"},
		"unknown nested":                    {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\napi:\n  requestWrappr: x\n", "requestWrappr"},
		"no resource":                       {"mode: existing\n", "resource is required"},
		"wrong mode":                        {"resource: Thing\nmode: new\n", `mode is "new"`},
		"no mode":                           {"resource: Thing\n", `mode is ""`},
		"no validators":                     {"resource: Thing\nmode: existing\n", "validators.inferred is required"},
		"bad readEmptyAs":                   {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  T:\n    fields:\n      f: {readEmptyAs: zero}\n", "readEmptyAs"},
		"bad default":                       {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  T:\n    fields:\n      f: {default: [a]}\n", "default is"},
		"two validators":                    {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  T:\n    fields:\n      f: {validators: [{oneOf: [a], sizeAtLeast: 1}]}\n", "exactly one"},
		"empty line":                        {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  T:\n    fields:\n      f: {}\n", "sets nothing"},
		"skip and more":                     {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  T:\n    fields:\n      f: {skip: true, computed: true}\n", "skipped field"},
		"required list":                     {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  T:\n    required: [a]\n", "only the empty list"},
		"type with no op":                   {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  T: {}\n", "has no override"},
		"bad extra type":                    {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  T:\n    extraAttributes:\n      token_wo:\n        elementType: bool\n        markdownDescription: x\n", "elementType"},
		"unwrap twice":                      {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\nunwrap: [A, A]\n", "names A twice"},
		"skip and unwrap":                   {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  T:\n    fields:\n      f: {skip: true, unwrap: true}\n", "skipped field"},
		"skip and name":                     {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  T:\n    fields:\n      f: {skip: true, name: g}\n", "skipped field"},
		"bad name":                          {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  T:\n    fields:\n      f: {name: Time-Frame}\n", "is not a Terraform attribute name"},
		"empty value name":                  {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  T:\n    fields:\n      f: {valueNames: {1: Sunday, 2: \"\"}}\n", "2 has an empty name"},
		"skip and asBool":                   {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  T:\n    fields:\n      f: {skip: true, asBool: true}\n", "skipped field"},
		"same value name":                   {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  T:\n    fields:\n      f: {valueNames: {1: Sunday, 7: Sunday}}\n", "1 and 7 both have the name"},
		"name of a readable rejected value": {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\nenums:\n  E:\n    zero: unspecified\n    values: [E_ONE]\n    rejected: [E_TWO]\n    readRejected: true\n    names: {E_ONE: E_TWO}\n", "have the same Terraform value"},
		"bad nested group name":             {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  T:\n    groups:\n      Bad-Name: {fields: [a]}\n", "types.T.groups.Bad-Name: the name is not a Terraform name"},
		"field in two nested groups":        {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  T:\n    groups:\n      a: {fields: [x]}\n      b: {fields: [x]}\n", "field x is also in types.T.groups.a"},
		"zero spelling of a readable rejected value": {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\nenums:\n  E:\n    zero: E_TWO\n    values: [E_ONE]\n    rejected: [E_TWO]\n    readRejected: true\n", "zero and the rejected \"E_TWO\" have the same Terraform value"},
		"name that is the lower case":                {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\nenums:\n  E:\n    values: [E_ONE]\n    names: {E_ONE: e_one}\n", "already the Terraform value"},
		"name that trimPrefix makes":                 {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\nenums:\n  E:\n    trimPrefix: E_\n    values: [E_ONE]\n    names: {E_ONE: one}\n", "already the Terraform value"},
		"name that verbatim makes":                   {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\nenums:\n  E:\n    verbatim: true\n    values: [E_ONE]\n    names: {E_ONE: E_ONE}\n", "already the Terraform value"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(test.text))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want it to contain %q", err, test.want)
			}
		})
	}
}

// A name line renames the Terraform attribute of a field, and idProperty names the field that
// holds the id. The policy keeps both.
func TestNameAndIDPropertyPolicy(t *testing.T) {
	f, err := Parse([]byte("resource: R\nmode: existing\nvalidators:\n  inferred: false\napi:\n  updateIDInBody: true\n  idProperty: uniqueIdentifier\ntypes:\n  R:\n    fields:\n      uniqueIdentifier: {name: id}\n      scheduleOperation: {name: operation, markdownDescription: The operation.}\n"))
	if err != nil {
		t.Fatal(err)
	}
	p := f.Policy()
	if p.IDProperty != "uniqueIdentifier" {
		t.Fatalf("IDProperty = %q, want uniqueIdentifier", p.IDProperty)
	}
	want := map[string]string{"R.uniqueIdentifier": "id", "R.scheduleOperation": "operation"}
	if !reflect.DeepEqual(p.Names, want) {
		t.Fatalf("Names = %v, want %v", p.Names, want)
	}
	if got := p.TerraformName("R", "scheduleOperation"); got != "operation" {
		t.Fatalf("TerraformName = %q, want operation", got)
	}
	if got := p.TerraformName("Other", "scheduleOperation"); got != "schedule_operation" {
		t.Fatalf("TerraformName of another component = %q, want schedule_operation", got)
	}
	if slices.Contains(p.Released, "R.scheduleOperation") {
		t.Fatal("a name line states no presence: the field must still follow the contract")
	}
}

func TestAValueIsAcceptedOrRejectedNotBoth(t *testing.T) {
	_, err := Parse([]byte("resource: R\nmode: existing\nvalidators:\n  inferred: false\nenums:\n  e.Kind: {values: [A, B], rejected: [B]}\n"))
	if err == nil || !strings.Contains(err.Error(), "values and in rejected") {
		t.Fatalf("err = %v, want an error about a value in both lists", err)
	}
}

func TestAValidatorSetsExactlyOneKey(t *testing.T) {
	_, err := Parse([]byte("resource: R\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  R:\n    fields:\n      f: {validators: [{enum: true, oneOf: [a]}]}\n"))
	if err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("err = %v, want an error about the validator keys", err)
	}
}

// A resource without a released version may keep the limits of the contract as validators.
func TestInferredValidatorsOfAnUnreleasedResource(t *testing.T) {
	for _, inferred := range []bool{true, false} {
		file, err := Parse([]byte(fmt.Sprintf("resource: R\nmode: existing\nvalidators:\n  inferred: %t\n", inferred)))
		if err != nil {
			t.Fatalf("inferred %t: %v", inferred, err)
		}
		if got := file.Policy().NoInferredValidators; got != !inferred {
			t.Fatalf("inferred %t: NoInferredValidators = %t", inferred, got)
		}
	}
}

func TestARequiredFieldCannotBeComputed(t *testing.T) {
	_, err := Parse([]byte("resource: R\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  R:\n    fields:\n      f: {required: true, computed: true}\n"))
	if err == nil || !strings.Contains(err.Error(), "cannot be computed") {
		t.Fatalf("err = %v, want an error about a required and computed field", err)
	}
}

// A released number field, such as an Int64 with a static 0, states its default as a number.
func TestADefaultCanBeANumber(t *testing.T) {
	f, err := Parse([]byte("resource: R\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  R:\n    fields:\n      count: {computed: true, default: 0}\n      ratio: {computed: true, default: 0.5}\n"))
	if err != nil {
		t.Fatal(err)
	}
	fields := f.Types["R"].Fields
	if fields["count"].Default != 0 || fields["ratio"].Default != 0.5 {
		t.Fatalf("defaults = %#v, %#v, want 0 and 0.5", fields["count"].Default, fields["ratio"].Default)
	}
	if got := strings.Join(f.Policy().Defaults, ","); got != "R.count,R.ratio" {
		t.Fatalf("policy defaults = %s, want R.count,R.ratio", got)
	}
}

// A line whose mode a server default cannot have replaces the server default of the contract.
// Another line keeps it.
func TestLinesThatReplaceTheServerDefault(t *testing.T) {
	f, err := Parse([]byte("resource: R\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  R:\n    fields:\n" +
		"      a: {default: 0}\n      b: {required: true}\n      c: {computed: false}\n" +
		"      d: {computed: true}\n      e: {computed: true, useStateForUnknown: true}\n      f: {description: Text.}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(f.Policy().Defaults, ","); got != "R.a,R.c" {
		t.Fatalf("policy defaults = %s, want R.a,R.c", got)
	}
}

// Only a line that says how an omitted value behaves answers the presence question of the
// generator. A line that only changes a text, a validator, or the order does not.
func TestPolicyReleasesOnlyLinesThatStatePresence(t *testing.T) {
	text := valid + "  Words:\n    fields:\n      text: {description: A text.}\n      order: {keepPriorOrder: true}\n      mode: {computed: true}\n      zero: {default: false}\n      empty: {readEmptyAs: \"null\"}\n"
	text = strings.Replace(text, "enums:\n  Kind: {zero: unspecified}\n  Words:", "  Words:", 1)
	text += "enums:\n  Kind: {zero: unspecified}\n"
	f, err := Parse([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(f.Policy().Released, ",")
	if want := "Target.id,Words.empty,Words.mode,Words.zero"; got != want {
		t.Fatalf("released = %s, want %s", got, want)
	}
}

func TestDeleteOperation(t *testing.T) {
	const head = "resource: R\nmode: existing\nvalidators:\n  inferred: false\napi:\n"
	f, err := Parse([]byte(head + "  delete:\n    operation: Service_ArchiveR\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := f.Policy().DeleteOperation; got != "Service_ArchiveR" {
		t.Fatalf("delete operation = %q, want Service_ArchiveR", got)
	}
	if f.Lines() != nil {
		t.Fatalf("lines = %v, want none: the generator checks the operation itself", f.Lines())
	}
	tests := map[string]struct {
		text string
		want string
	}{
		"no operation":       {head + "  delete: {}\n", "api.delete.operation is required"},
		"empty operation":    {head + "  delete:\n    operation: \"\"\n", "api.delete.operation is required"},
		"unknown key":        {head + "  delete:\n    operation: Service_ArchiveR\n    before: deactivate\n", "before"},
		"operation scalar":   {head + "  delete: Service_ArchiveR\n", "overrides.Delete"},
		"also in operations": {head + "  delete:\n    operation: Service_ArchiveR\n  operations:\n    create: Service_CreateR\n    get: Service_GetR\n    update: Service_UpdateR\n    delete: Service_DeleteR\n", "keep only api.operations.delete"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(test.text))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want it to contain %q", err, test.want)
			}
		})
	}
}

func TestEquality(t *testing.T) {
	const head = "resource: R\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  R:\n    fields:\n"
	f, err := Parse([]byte(head + "      config: {equality: yaml}\n      settings: {equality: json, description: A JSON object.}\n"))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, l := range f.Lines() {
		got = append(got, l.String()+"="+l.Equality+":"+strings.Join(l.Keys, "+"))
	}
	if want := "types.R.fields.config=yaml:equality,types.R.fields.settings=json:description+equality"; strings.Join(got, ",") != want {
		t.Fatalf("lines = %s, want %s", strings.Join(got, ","), want)
	}
	if released := f.Policy().Released; len(released) != 0 {
		t.Fatalf("released = %v: equality does not state presence", released)
	}
	tests := map[string]struct {
		line string
		want string
	}{
		"unknown value":  {"{equality: toml}", `equality is "toml"`},
		"upper case":     {"{equality: YAML}", `equality is "YAML"`},
		"skipped field":  {"{skip: true, equality: yaml}", "skipped field"},
		"not a scalar":   {"{equality: [yaml]}", "into string"},
		"empty is unset": {"{equality: \"\"}", "sets nothing"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(head + "      config: " + test.line + "\n"))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want it to contain %q", err, test.want)
			}
		})
	}
}

func TestUnwrapPolicy(t *testing.T) {
	f, err := Parse([]byte("resource: R\nmode: existing\nvalidators:\n  inferred: false\nunwrap: [Query, UUID]\ntypes:\n  R:\n    fields:\n      label: {unwrap: true}\n      raw: {unwrap: false}\n"))
	if err != nil {
		t.Fatal(err)
	}
	p := f.Policy()
	if !reflect.DeepEqual(p.Unwrap, []string{"Query", "UUID"}) {
		t.Errorf("unwrap = %v", p.Unwrap)
	}
	if want := map[string]bool{"R.label": true, "R.raw": false}; !reflect.DeepEqual(p.UnwrapFields, want) {
		t.Errorf("unwrap fields = %v, want %v", p.UnwrapFields, want)
	}
	if len(p.Released) != 0 {
		t.Errorf("released = %v: an unwrap line says nothing about presence", p.Released)
	}
	for _, line := range f.Lines() {
		if !slices.Contains(line.Keys, "unwrap") {
			t.Errorf("line %s keys = %v, want unwrap", line, line.Keys)
		}
	}
}

// The keys that a resource generated in the released shape of a handwritten one needs: its type
// name, groups of top-level fields, and enum values with their prefix removed.
func TestReleasedShapeKeys(t *testing.T) {
	const head = "resource: R\nmode: existing\nvalidators:\n  inferred: false\n"
	f, err := Parse([]byte(head + "typeName: r_v2\ngroups:\n  sli:\n    fields: [a, b]\n    requireOne: true\n  window: {fields: [c]}\nenums:\n  E: {trimPrefix: E_, values: [E_ONE_DAY]}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if f.GroupOf("b") != "sli" || f.GroupOf("c") != "window" || f.GroupOf("d") != "" {
		t.Fatalf("GroupOf = %q, %q, %q, want sli, window, and none", f.GroupOf("b"), f.GroupOf("c"), f.GroupOf("d"))
	}
	if got := f.Enums["E"].TFValue("E_ONE_DAY"); got != "one_day" {
		t.Fatalf("TFValue = %q, want one_day", got)
	}
	tests := map[string]struct{ text, want string }{
		"type name":           {"typeName: R-v2\n", "not a Terraform name"},
		"group name":          {"groups:\n  Sli: {fields: [a]}\n", "not a Terraform name"},
		"empty group":         {"groups:\n  sli: {fields: []}\n", "fields is empty"},
		"field in two groups": {"groups:\n  sli: {fields: [a]}\n  window: {fields: [a]}\n", "also in groups.sli"},
		"prefix of no value":  {"enums:\n  E: {trimPrefix: X_, values: [E_ONE]}\n", "not a prefix"},
		"between order":       {"types:\n  R:\n    fields:\n      f: {validators: [{between: [100, 0]}]}\n", "minimum and a maximum"},
		"two state modifiers": {"types:\n  R:\n    fields:\n      f: {useStateForUnknown: true, useNonNullStateForUnknown: true}\n", "cannot both be set"},
		"two validator kinds": {"types:\n  R:\n    fields:\n      f: {validators: [{exactlyOneOf: [g], atLeastOneOf: [h]}]}\n", "exactly one"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(head + test.text)); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want it to contain %q", err, test.want)
			}
		})
	}
}

func TestRejectUnknownOfAnUnreleasedResource(t *testing.T) {
	for _, reject := range []bool{false, true} {
		file, err := Parse([]byte(fmt.Sprintf("resource: R\nmode: existing\nvalidators:\n  inferred: true\n  rejectUnknown: %t\n", reject)))
		if err != nil {
			t.Fatal(err)
		}
		if got := file.Policy().RejectUnknown; got != reject {
			t.Fatalf("rejectUnknown %t: Policy().RejectUnknown = %t", reject, got)
		}
	}
}

func TestOptionalAndCreateAsPolicy(t *testing.T) {
	f, err := Parse([]byte("resource: R\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  R:\n    fields:\n      quota: {createAs: {limit: limitValue}}\n  Quota:\n    fields:\n      limit: {computed: true}\n  Item:\n    fields:\n      values: {optional: true}\n"))
	if err != nil {
		t.Fatal(err)
	}
	p := f.Policy()
	if want := map[string]map[string]string{"R.quota": {"limit": "limitValue"}}; !reflect.DeepEqual(p.CreateAs, want) {
		t.Errorf("createAs = %v, want %v", p.CreateAs, want)
	}
	if !reflect.DeepEqual(p.Optional, []string{"Item.values"}) {
		t.Errorf("optional = %v", p.Optional)
	}
	if !reflect.DeepEqual(p.Computed, []string{"Quota.limit"}) {
		t.Errorf("computed = %v", p.Computed)
	}
	for _, line := range f.Lines() {
		if line.Field == "values" && !line.Optional {
			t.Errorf("line %s does not say optional", line)
		}
	}
}

func TestOptionalAndCreateAsAreChecked(t *testing.T) {
	for name, field := range map[string]string{
		"required and optional":        "{required: true, optional: true}",
		"skip and optional":            "{skip: true, optional: true}",
		"skip and createAs":            "{skip: true, createAs: {limit: limitValue}}",
		"createAs with an empty field": "{createAs: {limit: ''}}",
		"createAs names a field twice": "{createAs: {limit: limitValue, max: limitValue}}",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte("resource: R\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  R:\n    fields:\n      quota: " + field + "\n")); err == nil {
				t.Fatalf("%s: want an error", field)
			}
		})
	}
}

func TestCollectionAndNameAreChecked(t *testing.T) {
	for name, field := range map[string]string{
		"collection other than set":                 "{collection: list}",
		"name with upper case":                      "{name: Applications}",
		"name with a dash":                          "{name: has-limit}",
		"skip and name":                             "{skip: true, name: limits}",
		"atLeast and regexMatches in one validator": "{validators: [{atLeast: 0, regexMatches: {pattern: x}}]}",
		"requestValue and requestValueBy":           "{requestValue: A, requestValueBy: {b: B}}",
		"requestValueBy with an empty value":        "{requestValueBy: {b: ''}}",
		"skip and requestValueBy":                   "{skip: true, requestValueBy: {b: B}}",
		"keyedBy without a key":                     "{keyedBy: {value: v}}",
		"keyedBy with the key as value":             "{keyedBy: {key: k, value: k}}",
		"keyedBy and collection":                    "{keyedBy: {key: k}, collection: set}",
		"keyedBy object with a value":               "{keyedBy: {key: k, value: v, object: true}}",
		"keyedBy arms without object":               "{keyedBy: {key: k, arms: {A: a}}}",
		"keyedBy keyAs without object":              "{keyedBy: {key: k, keyAs: n}}",
		"regexMatches without a pattern":            "{validators: [{regexMatches: {message: m}}]}",
		"regexMatches that Go cannot compile":       "{validators: [{regexMatches: {pattern: '(?=x)'}}]}",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte("resource: R\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  R:\n    fields:\n      quota: " + field + "\n")); err == nil {
				t.Fatalf("%s: want an error", field)
			}
		})
	}
}

func TestEnumNamesAreChecked(t *testing.T) {
	for name, enum := range map[string]string{
		"a name for a value that is not accepted": "{values: [A_ONE], names: {A_TWO: Two}}",
		"an empty name":                  "{values: [A_ONE], names: {A_ONE: ''}}",
		"two values with one name":       "{values: [A_ONE, A_TWO], names: {A_ONE: Same, A_TWO: Same}}",
		"a name equal to the zero value": "{zero: Unspecified, values: [A_ONE], names: {A_ONE: Unspecified}}",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte("resource: R\nmode: existing\nvalidators:\n  inferred: false\nenums:\n  A: " + enum + "\n")); err == nil {
				t.Fatalf("%s: want an error", enum)
			}
		})
	}
}

func TestCreateAsAndNamesAreCheckedAcrossLines(t *testing.T) {
	for name, types := range map[string]string{
		"createAs on a nested component": "  Quota:\n    fields:\n      limit: {createAs: {max: maxValue}}\n",
		"two fields with one name":       "  R:\n    fields:\n      applicationFilters: {name: apps}\n      appFilters: {name: apps}\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte("resource: R\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n" + types)); err == nil {
				t.Fatal("want an error")
			}
		})
	}
	if _, err := Parse([]byte("resource: R\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  R:\n    fields:\n      quota: {createAs: {limit: limitValue}}\n")); err != nil {
		t.Fatalf("createAs on the resource component: %v", err)
	}
}
