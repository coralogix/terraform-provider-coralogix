package overrides

import (
	"reflect"
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
		EmptyRequired:  []string{"Labels"},
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
