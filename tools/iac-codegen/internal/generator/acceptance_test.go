package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/acceptance"
)

func testSynth(t *testing.T, fileText string) *accSynth {
	t.Helper()
	file, err := acceptance.Parse([]byte("resource: R\n" + fileText))
	if err != nil {
		t.Fatal(err)
	}
	return &accSynth{file: file, idAttr: "id", known: map[string]bool{}, used: map[string]bool{}}
}

func sampleAttrs() []*tfAttr {
	return []*tfAttr{
		{Name: "created", Kind: "String", Computed: true},
		{Name: "enabled", Kind: "Bool", Optional: true},
		{Name: "id", Kind: "String", Optional: true, Computed: true},
		{Name: "kind", Kind: "String", Optional: true, Validators: []string{`stringvalidator.OneOf("unspecified", "alpha", "beta")`}},
		{Name: "labels", Kind: "Map", ElementType: "types.StringType", Optional: true},
		{Name: "limit", Kind: "Int64", Optional: true},
		{Name: "name", Kind: "String", Required: true},
		{Name: "region", Kind: "String", Optional: true, Modifiers: []string{"stringplanmodifier.RequiresReplace()"}},
		{Name: "tags", Kind: "Set", ElementType: "types.StringType", Optional: true},
	}
}

func TestAcceptanceMakesPlainValuesByMode(t *testing.T) {
	tests := map[string]struct {
		mode      accMode
		wantLines []string
		wantNone  []string
		wantCheck []string
	}{
		"full": {
			accFull,
			[]string{`enabled = true`, `kind = "alpha"`, `labels = { key = "@{run}-labels" }`, `limit = 1`, `name = "@{run}-name"`, `region = "@{run}-region"`, `tags = ["@{run}-tags"]`},
			[]string{"id =", "created ="},
			[]string{"enabled=true", "kind=alpha", "labels.key=@{run}-labels", "tags.#=1"},
		},
		"updated": {
			accUpdated,
			[]string{`enabled = false`, `kind = "alpha"`, `limit = 2`, `name = "@{run}-name-updated"`, `region = "@{run}-region"`},
			[]string{"id ="},
			[]string{"enabled=false", "kind=alpha", "limit=2"},
		},
		"minimal": {
			accMinimal,
			[]string{`name = "@{run}-name"`},
			[]string{"enabled", "kind", "labels", "limit", "region", "tags"},
			[]string{"name=@{run}-name"},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			s := testSynth(t, "")
			body, checks, err := s.attrs(sampleAttrs(), "", "", test.mode, true)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range test.wantLines {
				if !strings.Contains(body, want) {
					t.Errorf("config lacks %q:\n%s", want, body)
				}
			}
			for _, none := range test.wantNone {
				if strings.Contains(body, none) {
					t.Errorf("config has %q:\n%s", none, body)
				}
			}
			var got []string
			for _, c := range checks {
				got = append(got, c.Path+"="+c.Value)
			}
			for _, want := range test.wantCheck {
				if !strings.Contains(strings.Join(got, "\n"), want) {
					t.Errorf("checks lack %q: %v", want, got)
				}
			}
		})
	}
}

func TestAcceptanceTakesOneArmOfAOneOfGroup(t *testing.T) {
	s := testSynth(t, "")
	attrs := []*tfAttr{
		{Name: "http", Kind: "SingleNested", Optional: true, GroupValidators: []string{"x"}, Attributes: []*tfAttr{{Name: "endpoint", Kind: "String", Required: true}}},
		{Name: "queue", Kind: "SingleNested", Optional: true, GroupValidators: []string{"x"}, Attributes: []*tfAttr{{Name: "topic", Kind: "String", Required: true}}},
	}
	body, _, err := s.attrs(attrs, "", "", accFull, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "http = {") || strings.Contains(body, "queue") {
		t.Fatalf("config should hold only the first arm:\n%s", body)
	}
}

func TestAcceptanceValuesSkipAndMinimalChangeTheConfig(t *testing.T) {
	s := testSynth(t, "values:\n  name: var.name\nskip: [labels]\nminimal: [limit]\n")
	full, _, err := s.attrs(sampleAttrs(), "", "", accFull, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(full, "name = var.name") || strings.Contains(full, "labels") {
		t.Fatalf("a value replaces the made-up one, and skip leaves the attribute out:\n%s", full)
	}
	minimal, _, err := s.attrs(sampleAttrs(), "", "", accMinimal, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(minimal, "limit = 1") {
		t.Fatalf("a minimal field is set in the minimal config:\n%s", minimal)
	}
}

func TestAcceptanceFailsClosedOnWhatItCannotMake(t *testing.T) {
	s := testSynth(t, "")
	_, _, err := s.attrs([]*tfAttr{{Name: "blob", Kind: "Dynamic", Required: true}}, "", "", accFull, true)
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("err = %v, want an unsupported kind", err)
	}
	_, _, err = s.attrs([]*tfAttr{{Name: "m", Kind: "List", ElementType: "types.ObjectType{}", Required: true}}, "", "", accFull, true)
	if err == nil || !strings.Contains(err.Error(), "set the value") {
		t.Fatalf("err = %v, want a request for a value", err)
	}
}

// The acceptance file may name only things that the schema has, so a stale line cannot stay.
func TestAcceptanceFileMustMatchTheSchema(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("..", "model", "testdata", "legacy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	base, err := os.ReadFile(filepath.Join("testdata", "legacy-acceptance.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]struct {
		edit func(string) string
		want string
	}{
		"value of a missing attribute": {func(s string) string {
			return strings.Replace(s, "coralogix_other.dependency.id\n", "coralogix_other.dependency.id\n  rules[].nope: x\n", 1)
		}, "is not set by the full config"},
		"skip of a missing attribute": {func(s string) string { return strings.Replace(s, "  - labels.env\n", "  - labels.nope\n", 1) }, "not an attribute"},
		"minimal of a missing one":    {func(s string) string { return strings.Replace(s, "  - labels\n", "  - nope\n", 1) }, "not an attribute"},
		"another resource":            {func(s string) string { return strings.Replace(s, "resource: LegacyThing", "resource: Other", 1) }, "is for resource"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			text := test.edit(string(base))
			if text == string(base) {
				t.Fatal("no change")
			}
			input, loadDir := syntheticInput(t)
			input.OpenAPI = spec
			dir := t.TempDir()
			path := filepath.Join(dir, "acceptance.yaml")
			if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
				t.Fatal(err)
			}
			options := Options{Resource: "LegacyThing", OutputDir: filepath.Join(dir, "out"), OverridesPath: filepath.Join("testdata", "legacy-overrides.yaml"), AcceptancePath: path}
			err := generateFromInput(options, input, loadDir)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
			if _, statErr := os.Stat(options.OutputDir); statErr == nil {
				t.Fatal("the generator wrote output although the acceptance file was wrong")
			}
		})
	}
}
