package generator

import (
	"os"
	"path/filepath"
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
	// A field that the configuration leaves out keeps the value of the API.
	schema := readGenerated(t, out, "schema.go")
	for _, want := range []string{"boolplanmodifier.UseStateForUnknown()", "stringplanmodifier.UseStateForUnknown()"} {
		if !strings.Contains(schema, want) {
			t.Errorf("schema.go does not contain %q", want)
		}
	}
	// The model holds a nested object that the configuration may leave out as an unknown-capable value.
	model := readGenerated(t, out, "model.go")
	for _, want := range []string{"Limits  types.Object", "Config  types.Object"} {
		if !strings.Contains(model, want) {
			t.Errorf("model.go does not contain %q", want)
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

func TestCreateNamesTheFieldsOfAnObject(t *testing.T) {
	limits := types.ObjectValueMust(alwaysThingLimitsAttrTypes(), map[string]attr.Value{"max": types.Int32Value(3)})
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
	if got := createMask(t, config, plan); got != "limits.max" {
		t.Errorf("mask = %q, want limits.max", got)
	}
}
`

func readGenerated(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
