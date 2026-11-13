package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sweepResource has a oneOf group whose arms hold an enum each, and an enum with more values than
// the group has arms.
//
//	chart: line (unit: bytes, seconds, percent) | pie (slice: small, big) | gauge
//	state: one, two
func sweepResource() *tfResource {
	unit := &tfAttr{Name: "unit", Kind: "String", Optional: true, EnumSchema: "Unit", EnumChoices: []enumOption{
		{"UNIT_BYTES", "bytes"}, {"UNIT_SECONDS", "seconds"}, {"UNIT_PERCENT", "percent"}}}
	slice := &tfAttr{Name: "slice", Kind: "String", Optional: true, EnumSchema: "Slice", EnumChoices: []enumOption{
		{"SLICE_SMALL", "small"}, {"SLICE_BIG", "big"}}}
	const group = "gauge,line,pie"
	return &tfResource{Package: "p", CRUD: &crudData{TypeName: "thing", IDAttr: "id", Resource: "Thing"}, Attributes: []*tfAttr{
		{Name: "name", Kind: "String", Required: true},
		{Name: "line", Kind: "SingleNested", Optional: true, OneOfGroup: group, Component: "Chart", Property: "line", Attributes: []*tfAttr{unit}},
		{Name: "pie", Kind: "SingleNested", Optional: true, OneOfGroup: group, Component: "Chart", Property: "pie", Attributes: []*tfAttr{slice}},
		{Name: "gauge", Kind: "SingleNested", Optional: true, OneOfGroup: group, Component: "Chart", Property: "gauge", Attributes: []*tfAttr{{Name: "max", Kind: "Int64", Optional: true}}},
		{Name: "state", Kind: "String", Optional: true, EnumSchema: "State", EnumChoices: []enumOption{{"STATE_ONE", "one"}, {"STATE_TWO", "two"}}},
	}}
}

func sweep(t *testing.T, fileText string) []*accStep {
	t.Helper()
	s := testSynth(t, fileText)
	res := sweepResource()
	s.cov = map[string]bool{}
	if _, _, err := s.attrs(res.Attributes, "", "", accFull, true); err != nil {
		t.Fatal(err)
	}
	steps, err := s.sweepSteps(res)
	if err != nil {
		t.Fatal(err)
	}
	return steps
}

// Every value of every enum and every arm of the group is set by the full config or by a step, also a
// value below an arm that is not the first one. A step number for each level would never set seconds
// and percent, because the line arm is in no step but the first.
func TestSweepSetsEveryValueAndArm(t *testing.T) {
	steps := sweep(t, "")
	var configs []string
	for _, step := range steps {
		configs = append(configs, step.Config)
	}
	all := strings.Join(configs, "\n")
	for _, want := range []string{`unit = "bytes"`, `unit = "seconds"`, `unit = "percent"`, `slice = "small"`, `slice = "big"`, `gauge = {`, `state = "two"`} {
		if !strings.Contains(all, want) {
			t.Errorf("no step sets %s:\n%s", want, all)
		}
	}
	// The full config sets the first of each: line, bytes, one.
	for i, step := range steps {
		if step.N != i+1 {
			t.Errorf("step %d has number %d", i+1, step.N)
		}
	}
	if len(steps) == 0 || len(steps) > 6 {
		t.Errorf("got %d steps, want a few", len(steps))
	}
}

// A skipped value and a skipped arm are never set. A value that the sweep does not set stays out of
// the steps, and the sweep still ends.
func TestSweepLeavesOutWhatTheFileSkips(t *testing.T) {
	steps := sweep(t, `
skipValues:
  Unit:
    values: [UNIT_SECONDS]
    reason: the tenant refuses it
skipChoices:
  Chart:
    choices: [pie]
    reason: needs a data source
`)
	var all []string
	for _, step := range steps {
		all = append(all, step.Config)
	}
	text := strings.Join(all, "\n")
	for _, none := range []string{`"seconds"`, "pie = {", "slice ="} {
		if strings.Contains(text, none) {
			t.Errorf("a step sets %s:\n%s", none, text)
		}
	}
	for _, want := range []string{`"percent"`, "gauge = {"} {
		if !strings.Contains(text, want) {
			t.Errorf("no step sets %s:\n%s", want, text)
		}
	}
}

// A skip entry that names nothing in the schema fails generation, so the list cannot go stale.
func TestSweepRefusesAStaleSkipEntry(t *testing.T) {
	tests := map[string]struct{ file, want string }{
		"enum":      {"skipValues:\n  Nope:\n    values: [X]\n    reason: r\n", `the schema has no enum "Nope"`},
		"value":     {"skipValues:\n  Unit:\n    values: [UNIT_GONE]\n    reason: r\n", `the enum has no accepted value "UNIT_GONE"`},
		"component": {"skipChoices:\n  Nope:\n    choices: [pie]\n    reason: r\n", `no oneOf group in the component "Nope"`},
		"choice":    {"skipChoices:\n  Chart:\n    choices: [gone]\n    reason: r\n", `the component has no oneOf arm "gone"`},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			s := testSynth(t, test.file)
			if _, err := s.sweepSteps(sweepResource()); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
}

// A value in the acceptance file's values stays as it is, and a skipped path stays out of every step.
func TestSweepKeepsFixedValuesAndSkippedPaths(t *testing.T) {
	steps := sweep(t, "values:\n  state: '\"fixed\"'\nskip: [pie]\n")
	var all []string
	for _, step := range steps {
		all = append(all, step.Config)
	}
	text := strings.Join(all, "\n")
	if strings.Contains(text, `state = "two"`) || strings.Contains(text, "pie = {") {
		t.Errorf("a step changes a fixed value or sets a skipped path:\n%s", text)
	}
}

// A resource with no enum and no oneOf group has no values-and-choices steps.
func TestSweepHasNoStepsWithoutChoices(t *testing.T) {
	s := testSynth(t, "")
	res := &tfResource{Package: "p", CRUD: &crudData{TypeName: "thing", IDAttr: "id", Resource: "Thing"}, Attributes: sampleAttrs()[5:7]}
	s.cov = map[string]bool{}
	if _, _, err := s.attrs(res.Attributes, "", "", accFull, true); err != nil {
		t.Fatal(err)
	}
	steps, err := s.sweepSteps(res)
	if err != nil || len(steps) != 0 {
		t.Fatalf("steps = %d, err = %v, want none", len(steps), err)
	}
}

// The acceptance file of a generated resource skips a value and an arm. The generated test has no
// values-and-choices subtest when the file skips all that it would set.
func TestGeneratedSweepFollowsTheAcceptanceFile(t *testing.T) {
	input, sdkDir := syntheticInput(t)
	generate := func(acceptanceText string) string {
		t.Helper()
		dir := t.TempDir()
		options := Options{Resource: "Thing", OutputDir: filepath.Join(dir, "thing")}
		if acceptanceText != "" {
			options.AcceptancePath = filepath.Join(dir, "acceptance.yaml")
			if err := os.WriteFile(options.AcceptancePath, []byte(acceptanceText), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if err := generateFromInput(options, input, sdkDir); err != nil {
			t.Fatal(err)
		}
		text, err := os.ReadFile(filepath.Join(options.OutputDir, "acceptance_test.go"))
		if err != nil {
			t.Fatal(err)
		}
		return string(text)
	}
	if text := generate(""); !strings.Contains(text, `t.Run("values-and-choices"`) || !strings.Contains(text, "queue = {") {
		t.Fatal("without a file, the test must send the second arm and the second value")
	}
	skipped := generate("resource: Thing\nskipChoices:\n  ThingConfig:\n    choices: [queue]\n    reason: the tenant has no queue\nskipValues:\n  ThingKind:\n    values: [THING_KIND_ADVANCED, THING_KIND_P5_OR_UNSPECIFIED]\n    reason: behind a feature flag\n  ThingPriority:\n    values: [THING_PRIORITY_HIGH]\n    reason: behind a feature flag\n")
	if strings.Contains(skipped, "values-and-choices") || strings.Contains(skipped, "queue = {") {
		t.Fatalf("the file skips every value and arm, so the test has no step:\n%s", skipped)
	}
}

// widgetsResource has a list of widgets. A widget is a oneOf of three kinds, each with an enum, and a
// name that the API wants to be different for each widget.
func widgetsResource(validators ...string) *tfResource {
	kind := func(name string, values ...enumOption) *tfAttr {
		return &tfAttr{Name: name, Kind: "SingleNested", Optional: true, OneOfGroup: "bar,line,pie", Component: "Widget", Property: name,
			Attributes: []*tfAttr{{Name: "mode", Kind: "String", Optional: true, EnumSchema: name + "Mode", EnumChoices: values,
				Validators: []string{`stringvalidator.OneOf("a", "b")`}}}}
	}
	return &tfResource{Package: "p", CRUD: &crudData{TypeName: "board", IDAttr: "id", Resource: "Board"}, Attributes: []*tfAttr{
		{Name: "name", Kind: "String", Required: true},
		{Name: "widgets", Kind: "ListNested", Optional: true, Validators: validators, Attributes: []*tfAttr{
			{Name: "title", Kind: "String", Required: true},
			kind("bar", enumOption{"BAR_A", "a"}, enumOption{"BAR_B", "b"}),
			kind("line", enumOption{"LINE_A", "a"}, enumOption{"LINE_B", "b"}),
			kind("pie", enumOption{"PIE_A", "a"}),
		}},
	}}
}

func sweepWidgets(t *testing.T, res *tfResource) []*accStep {
	t.Helper()
	s := testSynth(t, "")
	s.cov = map[string]bool{}
	if _, _, err := s.attrs(res.Attributes, "", "", accFull, true); err != nil {
		t.Fatal(err)
	}
	steps, err := s.sweepSteps(res)
	if err != nil {
		t.Fatal(err)
	}
	return steps
}

// A step puts an element of each kind in a list, so the kinds do not need a step each. The titles
// differ, because the API can want that.
func TestSweepPutsSeveralElementsInOneStep(t *testing.T) {
	steps := sweepWidgets(t, widgetsResource())
	if len(steps) != 1 {
		t.Fatalf("got %d steps, want one step with all that the full config leaves out", len(steps))
	}
	config := steps[0].Config
	for _, want := range []string{`title = "@{run}-title"`, `title = "@{run}-title-2"`, `title = "@{run}-title-4"`, `mode = "b"`, "line = {", "pie = {"} {
		if !strings.Contains(config, want) {
			t.Errorf("the step lacks %s:\n%s", want, config)
		}
	}
	// line and pie are new arms; the second value of bar and of line needs a widget each.
	if got := strings.Count(config, "title ="); got != 4 {
		t.Errorf("got %d widgets, want 4:\n%s", got, config)
	}
}

// A size validator limits the elements of a list. The widgets that a copy cannot hold go in another
// copy of the resource in the same step.
func TestSweepKeepsTheSizeOfAList(t *testing.T) {
	steps := sweepWidgets(t, widgetsResource("listvalidator.SizeAtMost(1)"))
	if len(steps) != 1 {
		t.Fatalf("got %d steps, want one step with copies", len(steps))
	}
	blocks := strings.Split(steps[0].Config, "\nresource ")
	if len(blocks) < 3 {
		t.Fatalf("got %d copies, want a copy for each widget that a list cannot hold together:\n%s", len(blocks), steps[0].Config)
	}
	for _, block := range blocks {
		if got := strings.Count(block, "title ="); got != 1 {
			t.Errorf("a copy has %d widgets, want 1:\n%s", got, block)
		}
	}
}

// A resource with a oneOf group at its top level sets one arm in a copy, so a step holds a copy for
// each arm, and the copies are named and numbered apart. A singleton has one copy: the API holds one.
func TestSweepMakesCopiesOfTheResource(t *testing.T) {
	steps := sweep(t, "")
	if len(steps) != 1 {
		t.Fatalf("got %d steps, want one step with copies", len(steps))
	}
	config := steps[0].Config
	for _, want := range []string{`resource "coralogix_thing" "test" {`, `resource "coralogix_thing" "test_2" {`, `name = "@{run}-name-2"`, "gauge = {", "pie = {"} {
		if !strings.Contains(config, want) {
			t.Errorf("the step lacks %s:\n%s", want, config)
		}
	}

	s := testSynth(t, "")
	res := sweepResource()
	res.CRUD.Singleton = true
	s.cov = map[string]bool{}
	if _, _, err := s.attrs(res.Attributes, "", "", accFull, true); err != nil {
		t.Fatal(err)
	}
	singleton, err := s.sweepSteps(res)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range singleton {
		if strings.Contains(step.Config, `"test_2"`) {
			t.Fatalf("a singleton has a copy:\n%s", step.Config)
		}
	}
	if len(singleton) < 3 {
		t.Fatalf("got %d steps for a singleton, want a step for what a copy cannot hold together", len(singleton))
	}
}

// A step holds a limited number of extra elements in all of its lists. An element that the limit
// drops must not count as set: a later step or copy sets its value. Else a value is on the checklist
// and in no config. The lists of sections and of widgets in them add up to more than the limit.
func TestSweepSetsTheValuesOfADroppedElementLater(t *testing.T) {
	var options []enumOption
	for i := 1; i <= 100; i++ {
		options = append(options, enumOption{API: fmt.Sprintf("MODE_%d", i), TF: fmt.Sprintf("v%d", i)})
	}
	res := &tfResource{Package: "p", CRUD: &crudData{TypeName: "board", IDAttr: "id", Resource: "Board"}, Attributes: []*tfAttr{
		{Name: "name", Kind: "String", Required: true},
		{Name: "sections", Kind: "ListNested", Optional: true, Attributes: []*tfAttr{
			{Name: "title", Kind: "String", Required: true},
			{Name: "widgets", Kind: "ListNested", Optional: true, Attributes: []*tfAttr{
				{Name: "title", Kind: "String", Required: true},
				{Name: "mode", Kind: "String", Optional: true, EnumSchema: "Mode", EnumChoices: options},
			}},
		}},
	}}
	s := testSynth(t, "")
	s.cov = map[string]bool{}
	full, _, err := s.attrs(res.Attributes, "", "", accFull, true)
	if err != nil {
		t.Fatal(err)
	}
	steps, err := s.sweepSteps(res)
	if err != nil {
		t.Fatal(err)
	}
	all := full
	for _, step := range steps {
		all += "\n" + step.Config
	}
	var missing []string
	for _, option := range options {
		if !strings.Contains(all, `mode = "`+option.TF+`"`) {
			missing = append(missing, option.TF)
		}
	}
	if len(missing) != 0 {
		t.Errorf("no config sets %d modes, and the sweep took them as set: %v", len(missing), missing)
	}
}

// An attribute can accept fewer values than its enum. The sweep sets only the values that the
// validator of the attribute accepts.
func TestSweepSetsOnlyTheValuesThatTheValidatorAccepts(t *testing.T) {
	res := &tfResource{Package: "p", CRUD: &crudData{TypeName: "board", IDAttr: "id", Resource: "Board"}, Attributes: []*tfAttr{
		{Name: "name", Kind: "String", Required: true},
		{Name: "direction", Kind: "String", Optional: true, EnumSchema: "Direction", Validators: []string{`stringvalidator.OneOf("asc", "desc")`},
			EnumChoices: []enumOption{{"DIR_ASC", "asc"}, {"DIR_DESC", "desc"}, {"DIR_NONE", "none"}}},
	}}
	s := testSynth(t, "")
	s.cov = map[string]bool{}
	full, _, err := s.attrs(res.Attributes, "", "", accFull, true)
	if err != nil {
		t.Fatal(err)
	}
	steps, err := s.sweepSteps(res)
	if err != nil {
		t.Fatal(err)
	}
	all := full
	for _, step := range steps {
		all += "\n" + step.Config
	}
	if strings.Contains(all, `"none"`) {
		t.Errorf("a config sets none, and the validator refuses it:\n%s", all)
	}
	if !strings.Contains(all, `direction = "desc"`) {
		t.Errorf("no config sets desc:\n%s", all)
	}
}

func sweepAll(t *testing.T, res *tfResource) (string, error) {
	t.Helper()
	s := testSynth(t, "")
	s.cov = map[string]bool{}
	full, _, err := s.attrs(res.Attributes, "", "", accFull, true)
	if err != nil {
		t.Fatal(err)
	}
	steps, err := s.sweepSteps(res)
	all := full
	for _, step := range steps {
		all += "\n" + step.Config
	}
	return all, err
}

// A branch with an enum value left outranked a branch with only arms below it, so the sweep took the
// first branch again and again, and ended before it set the arms of the other. The arms below a
// branch count as much as an arm at the branch.
func TestSweepReachesTheArmsBelowABranch(t *testing.T) {
	const top, inner = "one,two", "a,b"
	res := &tfResource{Package: "p", CRUD: &crudData{TypeName: "board", IDAttr: "id", Resource: "Board"}, Attributes: []*tfAttr{
		{Name: "name", Kind: "String", Required: true},
		{Name: "one", Kind: "SingleNested", Optional: true, OneOfGroup: top, Component: "Top", Property: "one", Attributes: []*tfAttr{
			{Name: "mode", Kind: "String", Optional: true, EnumSchema: "Mode", EnumChoices: []enumOption{{"M_X", "x"}, {"M_Y", "y"}}},
		}},
		{Name: "two", Kind: "SingleNested", Optional: true, OneOfGroup: top, Component: "Top", Property: "two", Attributes: []*tfAttr{
			{Name: "a", Kind: "SingleNested", Optional: true, OneOfGroup: inner, Component: "Inner", Property: "a", Attributes: []*tfAttr{{Name: "size", Kind: "Int64", Optional: true}}},
			{Name: "b", Kind: "SingleNested", Optional: true, OneOfGroup: inner, Component: "Inner", Property: "b", Attributes: []*tfAttr{{Name: "size", Kind: "Int64", Optional: true}}},
		}},
	}}
	all, err := sweepAll(t, res)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"two = {", "a = {", "b = {", `mode = "y"`} {
		if !strings.Contains(all, want) {
			t.Errorf("no config sets %s:\n%s", want, all)
		}
	}
}

// An arm that a required arm keeps out of its group cannot be set. It must not count as a gain, else
// its branch outranks the others for good and they are never set.
func TestSweepIgnoresArmsThatARequiredArmKeepsOut(t *testing.T) {
	const top, inner = "query,static", "name,value"
	res := &tfResource{Package: "p", CRUD: &crudData{TypeName: "board", IDAttr: "id", Resource: "Board"}, Attributes: []*tfAttr{
		{Name: "title", Kind: "String", Required: true},
		{Name: "query", Kind: "SingleNested", Optional: true, OneOfGroup: top, Component: "Top", Property: "query", Attributes: []*tfAttr{
			{Name: "name", Kind: "SingleNested", Optional: true, OneOfGroup: inner, Component: "Inner", Property: "name", Attributes: []*tfAttr{{Name: "x", Kind: "Int64", Optional: true}}},
			{Name: "value", Kind: "SingleNested", Required: true, OneOfGroup: inner, Component: "Inner", Property: "value", Attributes: []*tfAttr{{Name: "y", Kind: "Int64", Optional: true}}},
		}},
		{Name: "static", Kind: "SingleNested", Optional: true, OneOfGroup: top, Component: "Top", Property: "static", Attributes: []*tfAttr{{Name: "z", Kind: "Int64", Optional: true}}},
	}}
	all, err := sweepAll(t, res)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(all, "static = {") {
		t.Errorf("no config sets static:\n%s", all)
	}
	if strings.Contains(all, "name = {") {
		t.Errorf("a config sets name, and the required arm value owns the group:\n%s", all)
	}
}

// A list or set of enum strings of a new resource keeps the validator in ElemValidators. The sweep
// sets each value of it, one value in each copy.
func TestSweepSetsEveryValueOfAListOfEnums(t *testing.T) {
	res := &tfResource{Package: "p", CRUD: &crudData{TypeName: "board", IDAttr: "id", Resource: "Board"}, Attributes: []*tfAttr{
		{Name: "name", Kind: "String", Required: true},
		{Name: "tags", Kind: "List", ElementType: "types.StringType", Optional: true, ElemValidators: []string{`stringvalidator.OneOf("a", "b", "c")`}},
	}}
	all, err := sweepAll(t, res)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`tags = ["a"]`, `tags = ["b"]`, `tags = ["c"]`} {
		if !strings.Contains(all, want) {
			t.Errorf("no config sets %s:\n%s", want, all)
		}
	}
}

// When the sweep ends, an arm or an enum value that no config set and that no skip names is an error.
func TestSweepFailsWhenAValueStaysOut(t *testing.T) {
	res := sweepResource()
	s := testSynth(t, "")
	s.cov = map[string]bool{}
	if err := s.leftOut(res.Attributes); err == nil || !strings.Contains(err.Error(), `"seconds"`) {
		t.Fatalf("got %v, want an error that names the value seconds", err)
	}
	s = testSynth(t, "skipValues:\n  Unit:\n    values: [UNIT_BYTES, UNIT_SECONDS, UNIT_PERCENT]\n    reason: test\n  Slice:\n    values: [SLICE_SMALL, SLICE_BIG]\n    reason: test\n  State:\n    values: [STATE_ONE, STATE_TWO]\n    reason: test\nskipChoices:\n  Chart:\n    choices: [line, pie, gauge]\n    reason: test\n")
	s.cov = map[string]bool{}
	if err := s.leftOut(res.Attributes); err != nil {
		t.Fatalf("got %v, want no error when the file skips all", err)
	}
}

// stepCopies limits the copies of a step, and gives each step a subtest of its own: it applies the
// full config and then the step. Without the key, one subtest holds all steps.
func TestGeneratedSweepSplitsStepsWithStepCopies(t *testing.T) {
	input, sdkDir := syntheticInput(t)
	generate := func(acceptanceText string) string {
		t.Helper()
		dir := t.TempDir()
		options := Options{Resource: "Thing", OutputDir: filepath.Join(dir, "thing"), AcceptancePath: filepath.Join(dir, "acceptance.yaml")}
		if err := os.WriteFile(options.AcceptancePath, []byte(acceptanceText), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := generateFromInput(options, input, sdkDir); err != nil {
			t.Fatal(err)
		}
		text, err := os.ReadFile(filepath.Join(options.OutputDir, "acceptance_test.go"))
		if err != nil {
			t.Fatal(err)
		}
		return string(text)
	}
	whole := generate("resource: Thing\n")
	if !strings.Contains(whole, `t.Run("values-and-choices",`) || strings.Contains(whole, "values-and-choices-1") {
		t.Fatalf("without stepCopies, one subtest holds all steps:\n%s", whole)
	}
	split := generate("resource: Thing\nstepCopies: 1\n")
	for _, want := range []string{`t.Run("values-and-choices-1",`, `t.Run("values-and-choices-2",`, "{Config: config(run, fullConfig), Check: fullChecks(run)},\n\t\t\t\t{Config: config(run, variant1Config)},"} {
		if !strings.Contains(split, want) {
			t.Errorf("with stepCopies, the test lacks %s:\n%s", want, split)
		}
	}
	if strings.Contains(split, `t.Run("values-and-choices",`) {
		t.Error("with stepCopies, no subtest holds all steps")
	}
}
