package generator

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// A singleton that the API cannot create or remove: a GET reads it, and a PATCH with an update mask
// changes it. The generator needs no behavior-overrides file for it. Create sends the PATCH with a
// mask of the configured fields, and Delete makes no call.
func TestGoldenOutputAlwaysThing(t *testing.T) {
	input, loadDir := syntheticInput(t)
	spec, err := os.ReadFile(filepath.Join("testdata", "always-thing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	input.OpenAPI = spec
	out := filepath.Join(t.TempDir(), "alwaysthing")
	if err := generateFromInput(Options{Resource: "AlwaysThing", OutputDir: out}, input, loadDir); err != nil {
		t.Fatal(err)
	}

	golden := filepath.Join("testdata", "golden", "alwaysthing")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("golden output differs; run go test ./internal/generator -run TestGoldenOutputAlwaysThing -update:\n%s", diff)
	}

	resource := readGenerated(t, out, "resource.go")
	for _, want := range []string{
		"createRequest(ctx, req.Config, req.Plan)",
		"v, err = r.update(ctx, SingletonID, body, mask)",
		"func (r *Resource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse)",
	} {
		if !strings.Contains(resource, want) {
			t.Errorf("resource.go does not contain %q", want)
		}
	}
	if strings.Contains(resource, "func (r *Resource) delete(") || strings.Contains(resource, "func (r *Resource) create(") {
		t.Error("resource.go has a create or delete API call, but the API has neither")
	}
	// The singleton keeps the last applied state after the test: values-and-choices ends in the full
	// config, so the singleton ends in the state of the first values.
	acceptance := readGenerated(t, out, "acceptance_test.go")
	endsFull := regexp.MustCompile(`variant\d+Checks\(run\)\},\n\t+// The resource always exists[^\n]*\n\t+\{Config: config\(run, fullConfig\), Check: fullChecks\(run\)\},\n\t+\}\),`)
	if !endsFull.MatchString(acceptance) {
		t.Error("the values-and-choices steps do not end with the full config")
	}
	// A field that the configuration leaves out keeps the value of the API.
	schema := readGenerated(t, out, "schema.go")
	for _, want := range []string{"boolplanmodifier.UseStateForUnknown()", "stringplanmodifier.UseStateForUnknown()"} {
		if !strings.Contains(schema, want) {
			t.Errorf("schema.go does not contain %q", want)
		}
	}
	// The model holds a nested object that the configuration may leave out as an unknown-capable value.
	model := readGenerated(t, out, "model.go")
	for _, want := range []string{`Limits\s+types\.Object`, `Config\s+types\.Object`} {
		if !regexp.MustCompile(want).MatchString(model) {
			t.Errorf("model.go does not match %q", want)
		}
	}
	if err := os.WriteFile(filepath.Join(out, "createrequest_test.go"), []byte(createRequestTest), 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

// createRequestTest runs the generated createRequest. A nested object that the configuration leaves
// out is unknown in the plan, and Create must still build the request. A oneOf names its arm in the
// mask, because the singleton can hold another arm.
const createRequestTest = `package alwaysthing

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func rawOf(t *testing.T, m AlwaysThingModel) tftypes.Value {
	t.Helper()
	ctx := context.Background()
	s := Schema()
	plan := tfsdk.Plan{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}
	if diags := plan.Set(ctx, &m); diags.HasError() {
		t.Fatal(diags)
	}
	return plan.Raw
}

func createMask(t *testing.T, config, plan AlwaysThingModel) string {
	t.Helper()
	s := Schema()
	_, mask, diags := createRequest(context.Background(),
		tfsdk.Config{Schema: s, Raw: rawOf(t, config)}, tfsdk.Plan{Schema: s, Raw: rawOf(t, plan)})
	if diags.HasError() {
		t.Fatal(diags)
	}
	return mask
}

func unknownRest() AlwaysThingModel {
	return AlwaysThingModel{
		Id:     types.StringValue(SingletonID),
		Note:   types.StringUnknown(),
		Limits: types.ObjectUnknown(alwaysThingLimitsAttrTypes()),
		Config: types.ObjectUnknown(alwaysThingConfigAttrTypes()),
	}
}

func TestCreateWithAnOmittedNestedObject(t *testing.T) {
	config := AlwaysThingModel{
		Id:      types.StringNull(),
		Enabled: types.BoolValue(true),
		Note:    types.StringNull(),
		Limits:  types.ObjectNull(alwaysThingLimitsAttrTypes()),
		Config:  types.ObjectNull(alwaysThingConfigAttrTypes()),
	}
	plan := unknownRest()
	plan.Enabled = types.BoolValue(true)
	if got := createMask(t, config, plan); got != "enabled" {
		t.Errorf("mask = %q, want enabled", got)
	}
}

// The arms of a top-level oneOf are the choice of the configuration. They keep no value of the API:
// a kept arm would stay in the plan next to the arm that replaces it.
func TestTopLevelArmsKeepNoValue(t *testing.T) {
	for _, name := range []string{"source_file", "source_url"} {
		if Schema().Attributes[name].IsComputed() {
			t.Errorf("%s is computed, so a switch of arms keeps the old arm in the plan", name)
		}
	}
}

func TestSwitchOfTopLevelArmsSendsOnlyTheNewArm(t *testing.T) {
	s := Schema()
	base := AlwaysThingModel{
		Id:      types.StringValue(SingletonID),
		Enabled: types.BoolValue(true),
		Note:    types.StringValue("n"),
		Limits:  types.ObjectNull(alwaysThingLimitsAttrTypes()),
		Config:  types.ObjectNull(alwaysThingConfigAttrTypes()),
	}
	state, plan := base, base
	state.SourceFile = types.StringValue("f")
	plan.SourceUrl = types.StringValue("u")
	raw := tfsdk.Plan{Schema: s, Raw: rawOf(t, plan)}
	body, mask, diags := updateRequest(context.Background(),
		tfsdk.Config{Schema: s, Raw: raw.Raw}, raw, tfsdk.State{Schema: s, Raw: rawOf(t, state)})
	if diags.HasError() {
		t.Fatal(diags)
	}
	if mask != "sourceUrl" || body.SourceFile != nil || body.SourceUrl == nil {
		t.Errorf("mask = %q, file = %v, url = %v; want only the new arm", mask, body.SourceFile, body.SourceUrl)
	}
}

func TestCreateNamesTheSelectedArm(t *testing.T) {
	armTypes := alwaysThingConfigAttrTypes()
	httpType := armTypes["http"].(types.ObjectType)
	queueType := armTypes["queue"].(types.ObjectType)
	configured := types.ObjectValueMust(alwaysThingConfigAttrTypes(), map[string]attr.Value{
		"http":  types.ObjectNull(httpType.AttrTypes),
		"queue": types.ObjectValueMust(queueType.AttrTypes, map[string]attr.Value{"topic": types.StringValue("t")}),
	})
	config := AlwaysThingModel{
		Id:      types.StringNull(),
		Enabled: types.BoolNull(),
		Note:    types.StringNull(),
		Limits:  types.ObjectNull(alwaysThingLimitsAttrTypes()),
		Config:  configured,
	}
	plan := unknownRest()
	plan.Enabled = types.BoolUnknown()
	plan.Config = configured
	if got := createMask(t, config, plan); got != "config.queue" {
		t.Errorf("mask = %q, want config.queue", got)
	}
}

// A field that the object leaves out is in the mask with no value, so the server clears it and the
// state matches the plan, also when the singleton already holds a value there.
func TestCreateClearsTheFieldsThatAnObjectLeavesOut(t *testing.T) {
	limits := types.ObjectValueMust(alwaysThingLimitsAttrTypes(), map[string]attr.Value{"max": types.Int32Value(3), "min": types.Int32Null()})
	config := AlwaysThingModel{
		Id:      types.StringNull(),
		Enabled: types.BoolNull(),
		Note:    types.StringNull(),
		Limits:  limits,
		Config:  types.ObjectNull(alwaysThingConfigAttrTypes()),
	}
	plan := unknownRest()
	plan.Enabled = types.BoolUnknown()
	plan.Limits = limits
	if got := createMask(t, config, plan); got != "limits.max,limits.min" {
		t.Errorf("mask = %q, want limits.max,limits.min", got)
	}
}
`

// A top-level oneOf that allows no arm cannot keep the value of the API: with no arm configured, the
// singleton may still hold one. The check and the generation both refuse it.
func TestAlwaysThingRefusesAnOptionalTopLevelOneOf(t *testing.T) {
	input, loadDir := syntheticInput(t)
	spec, err := os.ReadFile(filepath.Join("testdata", "always-thing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	old := "- required: [sourceUrl]\n"
	if strings.Count(string(spec), old) != 2 {
		t.Fatal("the fixture has no two top-level oneOf lists")
	}
	spec = regexp.MustCompile(`(?m)^( *)- required: \[sourceUrl\]\n`).ReplaceAll(spec,
		[]byte("${0}${1}- not: {anyOf: [{required: [sourceFile]}, {required: [sourceUrl]}]}\n"))
	input.OpenAPI = spec
	candidate := filepath.Join(t.TempDir(), "candidate.yaml")
	if err := os.WriteFile(candidate, spec, 0o644); err != nil {
		t.Fatal(err)
	}
	checkErr := Check(CheckOptions{Resource: "AlwaysThing", OpenAPIPath: candidate})
	out := filepath.Join(t.TempDir(), "alwaysthing")
	generateErr := generateFromInput(Options{Resource: "AlwaysThing", OutputDir: out}, input, loadDir)
	var eligibility *EligibilityError
	if !errors.As(generateErr, &eligibility) || !strings.Contains(generateErr.Error(), "optional top-level oneOf") {
		t.Fatalf("generate error = %v, want the optional top-level oneOf refusal", generateErr)
	}
	if checkErr == nil || checkErr.Error() != generateErr.Error() {
		t.Fatalf("check and generate decisions differ:\ncheck: %v\ngenerate: %v", checkErr, generateErr)
	}
	if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("generated output exists after the refusal: %v", err)
	}
}

// A singleton with no enum and no oneOf has no values-and-choices subtest. Then minimal-lifecycle is the
// last subtest, and it ends in the full config.
func TestAlwaysThingWithoutVariantsEndsInTheFullConfig(t *testing.T) {
	input, loadDir := syntheticInput(t)
	spec, err := os.ReadFile(filepath.Join("testdata", "always-thing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var kept []string
	lines := strings.Split(string(spec), "\n")
	for i, line := range lines {
		gone := strings.Contains(line, "sourceFile") || strings.Contains(line, "sourceUrl") || strings.Contains(line, "schemas/AlwaysThingConfig'}]")
		next := i+1 < len(lines) && (strings.Contains(lines[i+1], "sourceFile") || strings.Contains(lines[i+1], "sourceUrl"))
		if gone || strings.TrimSpace(line) == "oneOf:" && next {
			continue
		}
		kept = append(kept, line)
	}
	input.OpenAPI = []byte(strings.Join(kept, "\n"))
	out := filepath.Join(t.TempDir(), "alwaysthing")
	if err := generateFromInput(Options{Resource: "AlwaysThing", OutputDir: out}, input, loadDir); err != nil {
		t.Fatal(err)
	}
	acceptance := readGenerated(t, out, "acceptance_test.go")
	if strings.Contains(acceptance, "values-and-choices") {
		t.Fatal("the contract still has a values-and-choices subtest")
	}
	endsFull := regexp.MustCompile(`importStep,\n\t+// The resource always exists[^\n]*\n\t+\{Config: config\(run, fullConfig\), Check: fullChecks\(run\)\},\n\t+\}\),`)
	if !endsFull.MatchString(acceptance) {
		t.Error("minimal-lifecycle does not end with the full config")
	}
}

func readGenerated(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
