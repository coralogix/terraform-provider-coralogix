package generator

import (
	"fmt"
	"maps"
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
	return &accSynth{file: file, minimal: file.Minimal, idAttr: "id", known: map[string]bool{}, settable: map[string]bool{}, required: map[string]bool{}, used: map[string]bool{}}
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

// A made-up number fits the range validators of the attribute.
func TestAcceptanceNumbersFitTheirRange(t *testing.T) {
	tests := map[string]struct {
		kind, validator string
		full, updated   string
	}{
		"no range":             {"Int64", "", "1", "2"},
		"range around 1 and 2": {"Int64", "int64validator.Between(-3, 10)", "1", "2"},
		"minimum above 1":      {"Int32", "int32validator.Between(5, 10)", "5", "6"},
		"only a minimum":       {"Int64", "int64validator.AtLeast(3)", "3", "4"},
		"maximum below 2":      {"Int64", "int64validator.AtMost(0)", "0", "-1"},
		"one number":           {"Int64", "int64validator.Between(5, 5)", "5", "5"},
		"float default":        {"Float64", "", "1.5", "2.5"},
		"float range":          {"Float64", "float64validator.Between(0, 1)", "0", "1"},
		"narrow float range":   {"Float32", "float32validator.Between(0, 0.5)", "0", "0.25"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			a := &tfAttr{Name: "n", Kind: test.kind}
			if test.validator != "" {
				a.Validators = []string{test.validator}
			}
			full, _ := scalarValue(a, accFull)
			updated, _ := scalarValue(a, accUpdated)
			if full != test.full || updated != test.updated {
				t.Fatalf("values = %s, %s; want %s, %s", full, updated, test.full, test.updated)
			}
		})
	}
}

// A made-up collection has one element. A size validator that rejects one element needs a value
// in the acceptance file.
func TestAcceptanceCollectionsFitTheirSize(t *testing.T) {
	object := []*tfAttr{{Name: "x", Kind: "String", Required: true}}
	tests := map[string]struct {
		attr *tfAttr
		ok   bool
	}{
		"no limit":             {&tfAttr{Kind: "List", ElementType: "types.StringType"}, true},
		"at least one":         {&tfAttr{Kind: "List", ElementType: "types.StringType", Validators: []string{"listvalidator.SizeAtLeast(1)"}}, true},
		"at least two":         {&tfAttr{Kind: "Set", ElementType: "types.StringType", Validators: []string{"setvalidator.SizeAtLeast(2)"}}, false},
		"at most zero":         {&tfAttr{Kind: "Map", ElementType: "types.StringType", Validators: []string{"mapvalidator.SizeAtMost(0)"}}, false},
		"objects at least one": {&tfAttr{Kind: "ListNested", Attributes: object, Validators: []string{"listvalidator.SizeAtLeast(1)"}}, true},
		"objects at least two": {&tfAttr{Kind: "ListNested", Attributes: object, Validators: []string{"listvalidator.SizeAtLeast(2)"}}, false},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			test.attr.Name, test.attr.Required = "c", true
			_, _, err := testSynth(t, "").attrs([]*tfAttr{test.attr}, "", "", accFull, true)
			if test.ok && err != nil {
				t.Fatalf("err = %v, want one made-up element", err)
			}
			if !test.ok && (err == nil || !strings.Contains(err.Error(), "set the value")) {
				t.Fatalf("err = %v, want a request for a value", err)
			}
			if test.ok {
				return
			}
			if _, _, err := testSynth(t, "values:\n  c: '[]'\n").attrs([]*tfAttr{test.attr}, "", "", accFull, true); err != nil {
				t.Fatalf("a value in the file must win: %v", err)
			}
		})
	}
}

// A made-up string fits the length validators of the attribute and keeps @{run} when it can.
// @{run} is 12 characters.
func TestAcceptanceStringsFitTheirLength(t *testing.T) {
	tests := map[string]struct {
		validator     string
		full, updated string
	}{
		"no limit":            {"", "@{run}-name", "@{run}-name-updated"},
		"room for both":       {"stringvalidator.LengthBetween(1, 25)", "@{run}-name", "@{run}-name-updated"},
		"update cut":          {"stringvalidator.LengthAtMost(20)", "@{run}-name", "@{run}uname"},
		"both cut":            {"stringvalidator.LengthAtMost(15)", "@{run}-na", "@{run}una"},
		"padded":              {"stringvalidator.LengthAtLeast(30)", "@{run}-namexxxxxxxxxxxxx", "@{run}-name-updatedxxxxx"},
		"no room for the run": {"stringvalidator.LengthBetween(3, 10)", "aaa", "bbb"},
		"only the run":        {"stringvalidator.LengthAtMost(12)", "@{run}", "@{run}"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			a := &tfAttr{Name: "name", Kind: "String"}
			if test.validator != "" {
				a.Validators = []string{test.validator}
			}
			full, updated := stringValue(a, false), stringValue(a, true)
			if full != test.full || updated != test.updated {
				t.Fatalf("values = %q, %q; want %q, %q", full, updated, test.full, test.updated)
			}
			low, high := lengthRange(a)
			for _, v := range []string{full, updated} {
				n := len(strings.ReplaceAll(v, "@{run}", strings.Repeat("r", runLength)))
				if n < low || n > high {
					t.Fatalf("%q has length %d, want %d to %d", v, n, low, high)
				}
			}
		})
	}
}

// The element of a made-up collection passes the validators of the elements.
func TestAcceptanceCollectionElementsFitTheirValidators(t *testing.T) {
	tests := map[string]struct {
		attr *tfAttr
		want string
	}{
		"enum elements": {&tfAttr{Kind: "List", ElementType: "types.StringType",
			ElemValidators: []string{`stringvalidator.OneOf("ALPHA", "BETA")`}}, `["ALPHA"]`},
		"short strings": {&tfAttr{Kind: "Set", ElementType: "types.StringType",
			ElemValidators: []string{"stringvalidator.LengthAtMost(15)"}}, `["@{run}-ta"]`},
		"numbers in a range": {&tfAttr{Kind: "Map", ElementType: "types.Int64Type",
			ElemValidators: []string{"int64validator.AtLeast(5)"}}, `{ key = 5 }`},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			test.attr.Name, test.attr.Required = "tags", true
			body, _, err := testSynth(t, "").attrs([]*tfAttr{test.attr}, "", "", accFull, true)
			if err != nil {
				t.Fatal(err)
			}
			if want := "tags = " + test.want; body != want {
				t.Fatalf("config = %s, want %s", body, want)
			}
		})
	}
}

// The step after an upgrade expects the action that its configs cause.
func TestAcceptanceNextActionFollowsTheConfigs(t *testing.T) {
	enum := &tfAttr{Name: "kind", Kind: "String", Required: true, Validators: []string{`stringvalidator.OneOf("alpha")`}}
	name := &tfAttr{Name: "name", Kind: "String", Required: true}
	region := &tfAttr{Name: "region", Kind: "String", Optional: true, Modifiers: []string{"stringplanmodifier.RequiresReplace()"}}
	paused := &tfAttr{Name: "paused", Kind: "Bool", Optional: true, Computed: true, Default: "booldefault.StaticBool(false)"}
	note := &tfAttr{Name: "note", Kind: "String", Optional: true, Computed: true}
	label := &tfAttr{Name: "label", Kind: "String", Optional: true}
	zone := &tfAttr{Name: "zone", Kind: "String", Optional: true, Computed: true, Modifiers: []string{"stringplanmodifier.RequiresReplace()"}}
	tests := map[string]struct {
		attrs    []*tfAttr
		from, to accMode
		want     string
	}{
		"a mutable value changes":     {[]*tfAttr{enum, name, region}, accFull, accUpdated, accPlanNoReplace},
		"nothing changes":             {[]*tfAttr{enum, region}, accFull, accUpdated, accPlanNoop},
		"an immutable value is added": {[]*tfAttr{enum, name, region}, accMinimal, accFull, accPlanReplace},
		// A default or an API value in state can equal the next value: update or no change.
		"a left-out value is set to its default": {[]*tfAttr{enum, paused}, accMinimal, accUpdated, accPlanNoReplace},
		"a left-out computed value":              {[]*tfAttr{enum, note}, accMinimal, accFull, accPlanNoReplace},
		"a left-out plain value":                 {[]*tfAttr{enum, label}, accMinimal, accFull, accPlanNoReplace},
		// The API may have set the immutable zone to the value of the next config, or not.
		"a left-out immutable computed value": {[]*tfAttr{enum, name, zone}, accMinimal, accUpdated, accPlanUnknown},
		"a sure replace wins":                 {[]*tfAttr{enum, zone, region}, accMinimal, accFull, accPlanReplace},
	}
	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			s := testSynth(t, "")
			step := func(mode accMode) *accStep {
				body, _, err := s.attrs(test.attrs, "", "", mode, true)
				if err != nil {
					t.Fatal(err)
				}
				return &accStep{Config: body}
			}
			got, err := s.nextAction(test.attrs, step(test.from), step(test.to), test.from, test.to)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("action = %s, want %s", got, test.want)
			}
		})
	}
}

func TestAcceptanceTakesOneArmOfAOneOfGroup(t *testing.T) {
	s := testSynth(t, "")
	attrs := []*tfAttr{
		{Name: "http", Kind: "SingleNested", Optional: true, OneOfGroup: "http,queue", Attributes: []*tfAttr{{Name: "endpoint", Kind: "String", Required: true}}},
		{Name: "queue", Kind: "SingleNested", Optional: true, OneOfGroup: "http,queue", Attributes: []*tfAttr{{Name: "topic", Kind: "String", Required: true}}},
		{Name: "daily", Kind: "SingleNested", Optional: true, OneOfGroup: "daily,weekly", Attributes: []*tfAttr{{Name: "hour", Kind: "Int64", Required: true}}},
		{Name: "weekly", Kind: "SingleNested", Optional: true, OneOfGroup: "daily,weekly", Attributes: []*tfAttr{{Name: "day", Kind: "Int64", Required: true}}},
	}
	body, _, err := s.attrs(attrs, "", "", accFull, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "http = {") || strings.Contains(body, "queue") {
		t.Fatalf("config should hold only the first arm of the first group:\n%s", body)
	}
	if !strings.Contains(body, "daily = {") || strings.Contains(body, "weekly") {
		t.Fatalf("config should hold only the first arm of the second group:\n%s", body)
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
		"skip of a required one":      {func(s string) string { return strings.Replace(s, "  - labels.env\n", "  - name\n", 1) }, `"name" is required`},
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
			dir := filepath.Join(t.TempDir(), "legacything")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "acceptance.yaml")
			if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
				t.Fatal(err)
			}
			options := Options{Resource: "LegacyThing", OutputDir: dir, OverridesPath: filepath.Join("testdata", "legacy-overrides.yaml"), AcceptancePath: path}
			err := generateFromInput(options, input, loadDir)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
			if _, statErr := os.Stat(filepath.Join(dir, "acceptance_test.go")); statErr == nil {
				t.Fatal("the generator wrote output although the acceptance file was wrong")
			}
			// check finds the same error before an SDK exists.
			err = Check(CheckOptions{Resource: "LegacyThing", OpenAPIPath: candidate(t, dir, spec), OverridesPath: options.OverridesPath, AcceptancePath: path})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("check err = %v, want %q", err, test.want)
			}
		})
	}
	t.Run("check accepts a valid file", func(t *testing.T) {
		dir := t.TempDir()
		options := CheckOptions{Resource: "LegacyThing", OpenAPIPath: candidate(t, dir, spec),
			OverridesPath: filepath.Join("testdata", "legacy-overrides.yaml"), AcceptancePath: filepath.Join("testdata", "legacy-acceptance.yaml")}
		if err := Check(options); err != nil {
			t.Fatal(err)
		}
	})
}

// candidate writes the OpenAPI spec as a candidate file for Check.
func candidate(t *testing.T, dir string, spec []byte) string {
	t.Helper()
	path := filepath.Join(dir, "candidate.yaml")
	if err := os.WriteFile(path, spec, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// upgradeFixture is a resource with nested fields and a oneOf, and a builder of its acceptance
// data for an upgrade attributes file. shapes returns the attributes of a release that has the
// paths, in the types of now.
func upgradeFixture(t *testing.T) (shapes func(...string) map[string]acceptance.UpgradeAttribute,
	build func(string, map[string]acceptance.UpgradeAttribute) (*acceptanceData, error)) {
	t.Helper()
	arm := func(name, child string) *tfAttr {
		return &tfAttr{Name: name, Kind: "SingleNested", Optional: true, OneOfGroup: "http,queue",
			Attributes: []*tfAttr{{Name: child, Kind: "String", Required: true}}}
	}
	res := &tfResource{Package: "p", CRUD: &crudData{TypeName: "thing", IDAttr: "id", Resource: "Thing"}, Attributes: []*tfAttr{
		{Name: "name", Kind: "String", Required: true},
		{Name: "labels", Kind: "Map", ElementType: "types.StringType", Optional: true},
		{Name: "rules", Kind: "ListNested", Optional: true, Attributes: []*tfAttr{
			{Name: "name", Kind: "String", Required: true},
			{Name: "kind", Kind: "String", Optional: true},
		}},
		{Name: "delivery", Kind: "SingleNested", Optional: true, Attributes: []*tfAttr{arm("http", "endpoint"), arm("queue", "topic")}},
	}}
	file, err := acceptance.Parse([]byte("resource: Thing\nupgradeFrom: \"1.0.0\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	schema := map[string]*tfAttr{}
	schemaAttrs(schema, "", res.Attributes)
	shapes = func(paths ...string) map[string]acceptance.UpgradeAttribute {
		out := map[string]acceptance.UpgradeAttribute{}
		for _, path := range paths {
			out[path] = upgradeAttribute(schema[path])
		}
		return out
	}
	build = func(from string, attrs map[string]acceptance.UpgradeAttribute) (*acceptanceData, error) {
		return buildAcceptance(res, "example.com/provider", file, &acceptance.UpgradeAttributes{From: from, Attributes: attrs})
	}
	return shapes, build
}

// The upgrade test sends the released provider only the attributes in the upgrade attributes
// file, nested ones and oneOf arms included. This build gets every attribute.
func TestAcceptanceUpgradeLeavesOutNewAttributes(t *testing.T) {
	shapes, build := upgradeFixture(t)
	released := shapes("name", "rules", "rules[].name", "delivery", "delivery.queue", "delivery.queue.topic")
	data, err := build("1.0.0", released)
	if err != nil {
		t.Fatal(err)
	}
	if data.UpgradeFull == nil {
		t.Fatal("the upgrade test needs its own full config")
	}
	for _, want := range []string{"labels", "kind", "http", "endpoint"} {
		if !strings.Contains(data.Full.Config, want) {
			t.Errorf("full config lacks %q:\n%s", want, data.Full.Config)
		}
		if strings.Contains(data.UpgradeFull.Config, want) {
			t.Errorf("upgrade config has %q, which the released provider does not have:\n%s", want, data.UpgradeFull.Config)
		}
	}
	if !strings.Contains(data.UpgradeFull.Config, "queue") {
		t.Errorf("upgrade config should take the oneOf arm that the released provider has:\n%s", data.UpgradeFull.Config)
	}
	if !maps.Equal(data.UpgradeAttributes.Attributes, released) {
		t.Errorf("attributes = %v, want the kept list %v", data.UpgradeAttributes.Attributes, released)
	}

	// A new upgradeFrom takes the list from the schema again.
	data, err = build("0.9.0", released)
	if err != nil {
		t.Fatal(err)
	}
	if data.UpgradeFull != nil || data.UpgradeAttributes.From != "1.0.0" || len(data.UpgradeAttributes.Attributes) != 10 {
		t.Fatalf("upgrade attributes = %+v, want every attribute of the schema for 1.0.0", data.UpgradeAttributes)
	}
}

// An attribute whose type changed since the release is left out like a new one. A required one
// stops generation: no config works with both providers.
func TestAcceptanceUpgradeComparesTypes(t *testing.T) {
	shapes, build := upgradeFixture(t)
	changed := shapes("name", "labels")
	changed["labels"] = acceptance.UpgradeAttribute{Type: "String"} // a string in the release, a map now
	data, err := build("1.0.0", changed)
	if err != nil {
		t.Fatal(err)
	}
	if data.UpgradeFull == nil || strings.Contains(data.UpgradeFull.Config, "labels") {
		t.Errorf("upgrade config should leave out labels, whose type changed:\n%v", data.UpgradeFull)
	}

	// rules was required in the release and is optional now: the released provider still needs it.
	relaxed := shapes("name", "rules", "rules[].name")
	relaxed["rules"] = acceptance.UpgradeAttribute{Type: "ListNested", Required: true}
	data, err = build("1.0.0", relaxed)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(data.Minimal.Config, "rules") || !strings.Contains(data.UpgradeMinimal.Config, "rules = [") {
		t.Errorf("only the minimal config for the release should set rules:\nminimal:\n%s\nupgrade minimal:\n%s",
			data.Minimal.Config, data.UpgradeMinimal.Config)
	}
	for name, test := range map[string]struct {
		attrs map[string]acceptance.UpgradeAttribute
		want  string
	}{
		"a new required attribute": {shapes("name", "rules", "delivery"), `"rules[].name" is required but the released provider`},
		"a required attribute is gone": {map[string]acceptance.UpgradeAttribute{
			"name": {Type: "String", Required: true}, "owner": {Type: "String", Required: true}}, `"owner" is required by the released provider`},
		"a required attribute's new type": {map[string]acceptance.UpgradeAttribute{
			"name": {Type: "Int64", Required: true}}, `"name" is required by the released provider`},
	} {
		if _, err := build("1.0.0", test.attrs); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%s: err = %v, want %q", name, err, test.want)
		}
	}
}

// generate keeps the upgrade attributes file of the last run while upgradeFrom stays the same.
func TestGenerateKeepsTheUpgradeAttributes(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("..", "model", "testdata", "legacy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	out := legacyOutputWithAcceptance(t)
	options := Options{Resource: "LegacyThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "legacy-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(out, acceptance.UpgradeFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// As if the release had no labels: the next run must keep the list and leave labels out.
	older := strings.Replace(string(data), "    labels: {type: SingleNested}\n", "", 1)
	if older == string(data) {
		t.Fatal("no change")
	}
	if err := os.WriteFile(path, []byte(older), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	kept, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(kept) != older {
		t.Fatalf("upgrade attributes = %s, want the kept list", kept)
	}
	test, err := os.ReadFile(filepath.Join(out, "acceptance_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(test), "const upgradeFullConfig") {
		t.Fatal("the upgrade test should have its own full config without labels")
	}
}

// After the minimal config removes an Optional+Computed attribute, the test checks the value that
// the schema states: a default, or the kept value of the full config. Without one, it checks none.
func TestAcceptanceChecksRemovedComputedAttributes(t *testing.T) {
	attrs := []*tfAttr{
		{Name: "name", Kind: "String", Required: true},
		{Name: "disabled", Kind: "Bool", Optional: true, Computed: true, Default: "booldefault.StaticBool(false)"},
		{Name: "tier", Kind: "String", Optional: true, Computed: true, Modifiers: []string{`serverDefaultModifier{value: types.StringValue("basic")}`}},
		{Name: "limit", Kind: "Int64", Optional: true, Computed: true, Modifiers: []string{"int64planmodifier.UseStateForUnknown()"}},
		{Name: "description", Kind: "String", Optional: true, Computed: true},
	}
	s := testSynth(t, "")
	_, full, err := s.attrs(attrs, "", "", accFull, true)
	if err != nil {
		t.Fatal(err)
	}
	defaults, kept := s.removed(attrs, full)
	if got := fmt.Sprint(defaults); got != "[{disabled false} {tier basic}]" {
		t.Errorf("defaults = %s, want disabled and tier", got)
	}
	if got := fmt.Sprint(kept); got != "[{limit 1}]" {
		t.Errorf("kept = %s, want limit with its full value", got)
	}
}

// legacyOutputWithAcceptance returns an output directory that holds the legacy acceptance file, as
// a resource with an upgrade test has it.
func legacyOutputWithAcceptance(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "legacy-acceptance.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "legacything")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, acceptance.FileName), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return out
}

// An acceptance file with upgradeFrom must sit in the output directory, next to the upgrade
// attributes file, so that check and generate read the same list.
func TestUpgradeAcceptanceFileMustBeInTheOutput(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("..", "model", "testdata", "legacy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input, loadDir := syntheticInput(t)
	input.OpenAPI = spec
	options := Options{Resource: "LegacyThing", OutputDir: filepath.Join(t.TempDir(), "legacything"),
		OverridesPath: filepath.Join("testdata", "legacy-overrides.yaml"), AcceptancePath: filepath.Join("testdata", "legacy-acceptance.yaml")}
	err = generateFromInput(options, input, loadDir)
	if err == nil || !strings.Contains(err.Error(), "must be in the output directory") {
		t.Fatalf("err = %v, want the acceptance file in the output directory", err)
	}
}

// A nested field in minimal brings its optional parents, with only their required fields.
func TestAcceptanceMinimalIncludesParentsOfNestedFields(t *testing.T) {
	attrs := []*tfAttr{
		{Name: "name", Kind: "String", Required: true},
		{Name: "rules", Kind: "ListNested", Optional: true, Attributes: []*tfAttr{
			{Name: "name", Kind: "String", Required: true},
			{Name: "kind", Kind: "String", Optional: true},
			{Name: "targets", Kind: "ListNested", Optional: true, Attributes: []*tfAttr{
				{Name: "connector_id", Kind: "String", Required: true},
			}},
		}},
	}
	s := testSynth(t, "minimal:\n  - rules[].targets\n")
	body, _, err := s.attrs(attrs, "", "", accMinimal, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"rules = [", "targets = [", "connector_id = "} {
		if !strings.Contains(body, want) {
			t.Errorf("minimal config lacks %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "kind") {
		t.Errorf("minimal config sets an optional field that is not listed:\n%s", body)
	}
}

// nextAction compares only top-level attributes, so RequiresReplace below them stops generation.
func TestAcceptanceRejectsNestedRequiresReplace(t *testing.T) {
	res := &tfResource{Package: "p", CRUD: &crudData{TypeName: "thing", IDAttr: "id", Resource: "Thing"}, Attributes: []*tfAttr{
		{Name: "name", Kind: "String", Required: true},
		{Name: "rules", Kind: "ListNested", Optional: true, Attributes: []*tfAttr{
			{Name: "region", Kind: "String", Required: true, Modifiers: []string{"stringplanmodifier.RequiresReplace()"}},
		}},
	}}
	file, err := acceptance.Parse([]byte("resource: Thing\n"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = buildAcceptance(res, "example.com/provider", file, nil)
	if err == nil || !strings.Contains(err.Error(), `"rules[].region" has RequiresReplace below the top level`) {
		t.Fatalf("err = %v, want nested RequiresReplace rejected", err)
	}
}

// A minimal field that the server sets cannot be in a config. The walk leaves it out, so the entry
// would promise coverage that the test does not give.
func TestAcceptanceRejectsMinimalFieldThatTheServerSets(t *testing.T) {
	res := &tfResource{Package: "p", CRUD: &crudData{TypeName: "thing", IDAttr: "id", Resource: "Thing"}, Attributes: []*tfAttr{
		{Name: "name", Kind: "String", Required: true},
		{Name: "note", Kind: "String", Optional: true},
		{Name: "created", Kind: "String", Computed: true},
	}}
	for _, key := range []string{"minimal", "upgradeMinimal"} {
		t.Run(key, func(t *testing.T) {
			file, err := acceptance.Parse([]byte("resource: Thing\nupgradeFrom: \"1.0.0\"\n" + key + ":\n  - created\n"))
			if err != nil {
				t.Fatal(err)
			}
			_, err = buildAcceptance(res, "example.com/provider", file, nil)
			if err == nil || !strings.Contains(err.Error(), `"created" is set by the server`) {
				t.Fatalf("err = %v, want the server-set field rejected", err)
			}
		})
	}
	file, err := acceptance.Parse([]byte("resource: Thing\nminimal:\n  - note\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := buildAcceptance(res, "example.com/provider", file, nil); err != nil {
		t.Fatalf("err = %v, want a settable minimal field accepted", err)
	}
}
