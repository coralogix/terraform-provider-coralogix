package overrides

import (
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
		"empty file":      {"", "empty"},
		"unknown key":     {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\nbogus: 1\n", "bogus"},
		"unknown nested":  {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\napi:\n  requestWrappr: x\n", "requestWrappr"},
		"no resource":     {"mode: existing\n", "resource is required"},
		"wrong mode":      {"resource: Thing\nmode: new\n", `mode is "new"`},
		"no mode":         {"resource: Thing\n", `mode is ""`},
		"no validators":   {"resource: Thing\nmode: existing\n", "validators.inferred: false is required"},
		"inferred true":   {"resource: Thing\nmode: existing\nvalidators:\n  inferred: true\n", "validators.inferred: false is required"},
		"bad readEmptyAs": {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  T:\n    fields:\n      f: {readEmptyAs: zero}\n", "readEmptyAs"},
		"bad default":     {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  T:\n    fields:\n      f: {default: [a]}\n", "default is"},
		"two validators":  {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  T:\n    fields:\n      f: {validators: [{oneOf: [a], sizeAtLeast: 1}]}\n", "exactly one"},
		"empty line":      {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  T:\n    fields:\n      f: {}\n", "sets nothing"},
		"skip and more":   {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  T:\n    fields:\n      f: {skip: true, computed: true}\n", "skipped field"},
		"required list":   {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  T:\n    required: [a]\n", "only the empty list"},
		"type with no op": {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  T: {}\n", "has no override"},
		"bad extra type":  {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  T:\n    extraAttributes:\n      token_wo:\n        elementType: bool\n        markdownDescription: x\n", "elementType"},
		"unwrap twice":    {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\nunwrap: [A, A]\n", "names A twice"},
		"skip and unwrap": {"resource: Thing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  T:\n    fields:\n      f: {skip: true, unwrap: true}\n", "skipped field"},
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
		"no operation":     {head + "  delete: {}\n", "api.delete.operation is required"},
		"empty operation":  {head + "  delete:\n    operation: \"\"\n", "api.delete.operation is required"},
		"unknown key":      {head + "  delete:\n    operation: Service_ArchiveR\n    before: deactivate\n", "before"},
		"operation scalar": {head + "  delete: Service_ArchiveR\n", "overrides.Delete"},
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
