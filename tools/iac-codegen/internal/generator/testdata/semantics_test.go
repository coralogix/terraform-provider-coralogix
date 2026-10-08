package thing

import (
	"context"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"example.com/iac-test-sdk/go/openapi/gen/things_service"
)

func TestScalarPresence(t *testing.T) {
	if expandBool(types.BoolNull()) != nil || expandBool(types.BoolUnknown()) != nil {
		t.Fatal("null and unknown bool values must be omitted")
	}
	if got := expandBool(types.BoolValue(false)); got == nil || *got {
		t.Fatalf("explicit false = %v", got)
	}
	if got := expandString(types.StringValue("")); got == nil || *got != "" {
		t.Fatalf("explicit empty string = %v", got)
	}
}

func TestUnspecifiedEnumFlattensToNull(t *testing.T) {
	type status string
	unspecified := status("STATUS_UNSPECIFIED")
	if got := flattenEnum(&unspecified, "STATUS_UNSPECIFIED"); !got.IsNull() {
		t.Fatalf("unspecified enum = %v, want null", got)
	}
	active := status("STATUS_ACTIVE")
	if got := flattenEnum(&active, "STATUS_UNSPECIFIED"); got.ValueString() != string(active) {
		t.Fatalf("active enum = %v, want %q", got, active)
	}
	business := status("STATUS_P5_OR_UNSPECIFIED")
	if got := flattenEnum(&business, "STATUS_UNSPECIFIED"); got.ValueString() != string(business) {
		t.Fatalf("business enum = %v, want %q", got, business)
	}
}

func TestUnspecifiedEnumCollectionsReturnDiagnostics(t *testing.T) {
	type status string
	values := []status{"STATUS_ACTIVE", "STATUS_P5_OR_UNSPECIFIED", "STATUS_UNSPECIFIED"}
	ctx := context.Background()
	var diags diag.Diagnostics
	flattenEnumsList(ctx, path.Root("list"), values, "STATUS_UNSPECIFIED", &diags)
	flattenEnumsSet(ctx, path.Root("set"), values, "STATUS_UNSPECIFIED", &diags)
	flattenEnumMap(ctx, path.Root("map"), map[string]status{"value": "STATUS_UNSPECIFIED"}, "STATUS_UNSPECIFIED", &diags)
	if len(diags.Errors()) != 3 {
		t.Fatalf("enum collection diagnostics = %v, want three errors", diags)
	}
}

func TestComputedObjectUnknownDoesNotBlockCreate(t *testing.T) {
	ctx := context.Background()
	s := Schema()
	terraformType := s.Type().TerraformType(ctx)
	root, ok := terraformType.(tftypes.Object)
	if !ok {
		t.Fatalf("schema type = %T, want tftypes.Object", terraformType)
	}
	configValues := nullTFValues(root)
	configValues["name"] = tftypes.NewValue(root.AttributeTypes["name"], "created")
	configValues["kind"] = tftypes.NewValue(root.AttributeTypes["kind"], "THING_KIND_STANDARD")
	configType := root.AttributeTypes["config"].(tftypes.Object)
	httpType := configType.AttributeTypes["http"].(tftypes.Object)
	configValues["config"] = tftypes.NewValue(configType, map[string]tftypes.Value{
		"http": tftypes.NewValue(httpType, map[string]tftypes.Value{
			"endpoint": tftypes.NewValue(httpType.AttributeTypes["endpoint"], "https://example.com"),
		}),
		"queue": tftypes.NewValue(configType.AttributeTypes["queue"], nil),
	})
	planValues := cloneTFValues(configValues)
	planValues["status"] = tftypes.NewValue(root.AttributeTypes["status"], tftypes.UnknownValue)

	client := &things_service.ThingsServiceAPIService{}
	r := &Resource{client: client}
	var response frameworkresource.CreateResponse
	r.Create(ctx, frameworkresource.CreateRequest{
		Config: tfsdk.Config{Schema: s, Raw: tftypes.NewValue(terraformType, configValues)},
		Plan:   tfsdk.Plan{Schema: s, Raw: tftypes.NewValue(terraformType, planValues)},
	}, &response)
	if client.CreateCalls != 1 {
		t.Fatalf("Create API calls = %d, want 1; diagnostics: %v", client.CreateCalls, response.Diagnostics)
	}
}

func nullTFValues(object tftypes.Object) map[string]tftypes.Value {
	values := make(map[string]tftypes.Value, len(object.AttributeTypes))
	for name, valueType := range object.AttributeTypes {
		values[name] = tftypes.NewValue(valueType, nil)
	}
	return values
}

func cloneTFValues(values map[string]tftypes.Value) map[string]tftypes.Value {
	clone := make(map[string]tftypes.Value, len(values))
	for name, value := range values {
		clone[name] = value
	}
	return clone
}

func TestOptionalGetPresenceRoundTrips(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics
	omitted := flattenThing(ctx, path.Root("thing"), &things_service.Thing{}, &diags)
	if !omitted.Description.IsNull() || !omitted.Destinations.IsNull() || !omitted.Tags.IsNull() || !omitted.Details.IsNull() || !omitted.Labels.IsNull() {
		t.Fatalf("omitted optional fields = %#v, want null values", omitted)
	}

	emptyDescription := ""
	explicit := flattenThing(ctx, path.Root("thing"), &things_service.Thing{
		Description:  &emptyDescription,
		Destinations: []string{},
		Tags:         []string{},
		Details:      []things_service.ThingDetail{},
		Labels:       map[string]string{},
	}, &diags)
	if explicit.Description.IsNull() || explicit.Description.ValueString() != "" || explicit.Destinations.IsNull() || explicit.Tags.IsNull() || explicit.Details.IsNull() || explicit.Labels.IsNull() {
		t.Fatalf("explicit empty fields = %#v, want present values", explicit)
	}
	if diags.HasError() {
		t.Fatal(diags)
	}

	values := requestValues()
	state := cloneValues(values)
	state["description"] = types.StringValue("old description")
	mask, updateDiags := updateMask(context.Background(), fakeData{values}, fakeData{values}, fakeData{state})
	body, bodyDiags := expandUpdate(context.Background(), &ThingModel{
		Name:         types.StringNull(),
		Description:  types.StringNull(),
		Enabled:      types.BoolNull(),
		Destinations: types.ListNull(types.StringType),
		Tags:         types.SetNull(types.StringType),
		Details:      types.SetNull(types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType}}),
		Labels:       types.MapNull(types.StringType),
	})
	updateDiags.Append(bodyDiags...)
	if updateDiags.HasError() || body == nil || body.Description != nil || len(mask) != 1 || mask[0] != "description" {
		t.Fatalf("cleared optional field = body %#v, mask %v, diagnostics %v", body, mask, updateDiags)
	}
}

func TestCollectionPresenceAndStability(t *testing.T) {
	t.Run("list presence and order", testListPresenceAndOrder)
	t.Run("set stability", testSetStability)
	t.Run("set of objects", testSetOfObjects)
	t.Run("map and flatten", testMapAndFlatten)
}

func testListPresenceAndOrder(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics
	if got := expandStrings[string](ctx, path.Root("destinations"), types.ListNull(types.StringType), &diags); got != nil {
		t.Fatalf("null list = %#v", got)
	}
	if got := expandStrings[string](ctx, path.Root("destinations"), types.ListUnknown(types.StringType), &diags); got != nil {
		t.Fatalf("unknown list = %#v", got)
	}
	empty := types.ListValueMust(types.StringType, nil)
	if got := expandStrings[string](ctx, path.Root("destinations"), empty, &diags); got == nil || len(got) != 0 {
		t.Fatalf("empty list = %#v", got)
	}
	destinations := types.ListValueMust(types.StringType, []attr.Value{types.StringValue("b"), types.StringValue("a")})
	if got := expandStrings[string](ctx, path.Root("destinations"), destinations, &diags); len(got) != 2 || got[0] != "b" || got[1] != "a" {
		t.Fatalf("destinations list = %#v", got)
	}
	if diags.HasError() {
		t.Fatal(diags)
	}
}

func testSetStability(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics
	first := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("a"), types.StringValue("b")})
	second := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("b"), types.StringValue("a")})
	if !first.Equal(second) {
		t.Fatal("set order changed equality")
	}
	duplicate, duplicateDiags := types.SetValue(types.StringType, []attr.Value{types.StringValue("a"), types.StringValue("a")})
	if duplicateDiags.HasError() {
		t.Fatalf("duplicate set = %#v, %v", duplicate, duplicateDiags)
	}
	if got := expandStringsSet[string](ctx, path.Root("tags"), duplicate, &diags); len(got) != 1 || got[0] != "a" {
		t.Fatalf("expanded duplicate set = %#v", got)
	}
	if diags.HasError() {
		t.Fatal(diags)
	}
}

func testSetOfObjects(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics
	elem := types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType}}
	first := types.SetValueMust(elem, []attr.Value{
		types.ObjectValueMust(elem.AttrTypes, map[string]attr.Value{"name": types.StringValue("a")}),
		types.ObjectValueMust(elem.AttrTypes, map[string]attr.Value{"name": types.StringValue("b")}),
	})
	second := types.SetValueMust(elem, []attr.Value{
		types.ObjectValueMust(elem.AttrTypes, map[string]attr.Value{"name": types.StringValue("b")}),
		types.ObjectValueMust(elem.AttrTypes, map[string]attr.Value{"name": types.StringValue("a")}),
	})
	if !first.Equal(second) {
		t.Fatal("set of objects order changed equality")
	}
	items := expandElements[ThingDetailModel](ctx, path.Root("details"), first, &diags)
	if len(items) != 2 {
		t.Fatalf("expanded details = %#v", items)
	}
	got := expandThingDetail(ctx, path.Root("details"), &items[0], &diags)
	if got == nil || got.Name == "" {
		t.Fatalf("flattened detail = %#v", got)
	}
	if diags.HasError() {
		t.Fatal(diags)
	}
}

func testMapAndFlatten(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics
	emptyMap := types.MapValueMust(types.StringType, nil)
	if got := expandStringMap[string](ctx, path.Root("labels"), emptyMap, &diags); got == nil || len(got) != 0 {
		t.Fatalf("empty map = %#v", got)
	}
	if !flattenStringsList[string](ctx, nil, &diags).IsNull() || flattenStringsList(ctx, []string{}, &diags).IsNull() {
		t.Fatal("list flatten lost the null and empty distinction")
	}
	if diags.HasError() {
		t.Fatal(diags)
	}
}

func TestUpdateClearArmSwitchAndNoOp(t *testing.T) {
	scalar := &maskNode{attr: "enabled", api: "enabled"}
	if got := maskPaths(nil, "", scalar, types.BoolValue(false), types.BoolNull()); len(got) != 1 || got[0] != "enabled" {
		t.Fatalf("set false mask = %v", got)
	}
	if got := maskPaths(nil, "", scalar, types.BoolNull(), types.BoolValue(false)); len(got) != 1 || got[0] != "enabled" {
		t.Fatalf("clear mask = %v", got)
	}
	if got := maskPaths(nil, "", scalar, types.BoolValue(false), types.BoolValue(false)); len(got) != 0 {
		t.Fatalf("unchanged mask = %v", got)
	}

	armA := &maskNode{attr: "a", api: "a"}
	armB := &maskNode{attr: "b", api: "b"}
	union := &maskNode{attr: "choice", api: "choice", oneOf: true, children: []*maskNode{armA, armB}}
	typesByName := map[string]attr.Type{"a": types.StringType, "b": types.StringType}
	plan := types.ObjectValueMust(typesByName, map[string]attr.Value{"a": types.StringNull(), "b": types.StringValue("new")})
	state := types.ObjectValueMust(typesByName, map[string]attr.Value{"a": types.StringValue("old"), "b": types.StringNull()})
	if got := maskPaths(nil, "", union, plan, state); len(got) != 1 || got[0] != "choice.b" {
		t.Fatalf("arm-switch mask = %v", got)
	}

	values := requestValues()
	data := fakeData{values: values}
	body, mask, diags := updateRequest(context.Background(), data, data, data)
	if diags.HasError() || body != nil || mask != "" {
		t.Fatalf("unchanged update = %#v, mask %q, %v", body, mask, diags)
	}
}

func TestServerDefaultResetAndUnknownGuard(t *testing.T) {
	if unknown, preserve := serverDefaultPlan(types.BoolNull(), types.BoolValue(false), types.BoolValue(true)); !unknown || preserve {
		t.Fatalf("removed override = unknown %t, preserve %t", unknown, preserve)
	}
	if unknown, preserve := serverDefaultPlan(types.BoolNull(), types.BoolValue(true), types.BoolValue(true)); unknown || !preserve {
		t.Fatalf("settled default = unknown %t, preserve %t", unknown, preserve)
	}
	if unknown, preserve := serverDefaultPlan(types.BoolUnknown(), types.BoolValue(false), types.BoolValue(true)); unknown || preserve {
		t.Fatalf("unknown configuration = unknown %t, preserve %t", unknown, preserve)
	}

	base := requestValues()
	config, plan, state := cloneValues(base), cloneValues(base), cloneValues(base)
	config["enabled"] = types.BoolNull()
	plan["enabled"] = types.BoolUnknown()
	state["enabled"] = types.BoolValue(false)
	mask, diags := updateMask(context.Background(), fakeData{config}, fakeData{plan}, fakeData{state})
	if diags.HasError() || len(mask) != 1 || mask[0] != "enabled" {
		t.Fatalf("server-default reset mask = %v, %v", mask, diags)
	}

	config["enabled"] = types.BoolUnknown()
	_, diags = updateMask(context.Background(), fakeData{config}, fakeData{plan}, fakeData{state})
	if !diags.HasError() {
		t.Fatal("unknown configured update value was accepted")
	}

	createConfig, createPlan := cloneValues(base), cloneValues(base)
	createConfig["name"], createPlan["name"] = types.StringUnknown(), types.StringUnknown()
	createConfig["description"], createPlan["description"] = types.StringUnknown(), types.StringUnknown()
	createConfig["enabled"], createPlan["enabled"] = types.BoolNull(), types.BoolUnknown()
	if diags := validateCreate(context.Background(), fakeData{createConfig}, fakeData{createPlan}); !diags.HasError() || len(diags) != 2 {
		t.Fatalf("unknown Create diagnostics = %v, want two errors", diags)
	}
}

func requestValues() map[string]attr.Value {
	httpType := types.ObjectType{AttrTypes: map[string]attr.Type{"endpoint": types.StringType}}
	queueType := types.ObjectType{AttrTypes: map[string]attr.Type{"topic": types.StringType}}
	configType := map[string]attr.Type{"http": httpType, "queue": queueType}
	return map[string]attr.Value{
		"name":         types.StringNull(),
		"description":  types.StringNull(),
		"enabled":      types.BoolNull(),
		"kind":         types.StringNull(),
		"config":       types.ObjectNull(configType),
		"spec":         types.ObjectNull(Schema().Attributes["spec"].GetType().(types.ObjectType).AttrTypes),
		"destinations": types.ListNull(types.StringType),
		"tags":         types.SetNull(types.StringType),
		"details":      types.SetNull(types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType}}),
		"labels":       types.MapNull(types.StringType),
	}
}

func cloneValues(values map[string]attr.Value) map[string]attr.Value {
	clone := make(map[string]attr.Value, len(values)+1)
	for name, value := range values {
		clone[name] = value
	}
	return clone
}

type fakeData struct {
	values map[string]attr.Value
}

func (fakeData) Get(context.Context, interface{}) diag.Diagnostics {
	panic("Get must not run for an unchanged plan")
}

func (d fakeData) GetAttribute(_ context.Context, p path.Path, target interface{}) diag.Diagnostics {
	value := target.(*attr.Value)
	*value = d.values[p.String()]
	return nil
}

func TestEmptyValuesKeepPriorForm(t *testing.T) {
	ctx := context.Background()
	s := Schema()
	root := s.Type().TerraformType(ctx).(tftypes.Object)
	listType := root.AttributeTypes["destinations"].(tftypes.List)
	setType := root.AttributeTypes["tags"].(tftypes.Set)
	mapType := root.AttributeTypes["labels"].(tftypes.Map)
	emptyList := tftypes.NewValue(listType, []tftypes.Value{})
	emptySet := tftypes.NewValue(setType, []tftypes.Value{})
	emptyMap := tftypes.NewValue(mapType, map[string]tftypes.Value{})
	oneTag := tftypes.NewValue(setType, []tftypes.Value{tftypes.NewValue(tftypes.String, "a")})
	tests := map[string]struct {
		prior, response, want map[string]tftypes.Value
	}{
		"configured empty, API omits": {
			prior:    map[string]tftypes.Value{"destinations": emptyList, "tags": emptySet, "labels": emptyMap},
			response: map[string]tftypes.Value{},
			want:     map[string]tftypes.Value{"destinations": emptyList, "tags": emptySet, "labels": emptyMap},
		},
		"not configured, API returns empty": {
			prior:    map[string]tftypes.Value{},
			response: map[string]tftypes.Value{"destinations": emptyList, "tags": emptySet, "labels": emptyMap},
			want:     map[string]tftypes.Value{},
		},
		"drift is kept": {
			prior:    map[string]tftypes.Value{"tags": oneTag},
			response: map[string]tftypes.Value{"tags": emptySet},
			want:     map[string]tftypes.Value{"tags": emptySet},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			prior := nullTFValues(root)
			for k, v := range test.prior {
				prior[k] = v
			}
			response := nullTFValues(root)
			for k, v := range test.response {
				response[k] = v
			}
			want := nullTFValues(root)
			for k, v := range test.want {
				want[k] = v
			}
			state := tfsdk.State{Schema: s, Raw: tftypes.NewValue(root, response)}
			diags := keepPriorEmpty(ctx, tfsdk.Plan{Schema: s, Raw: tftypes.NewValue(root, prior)}, &state)
			if diags.HasError() {
				t.Fatal(diags)
			}
			if wantRaw := tftypes.NewValue(root, want); !state.Raw.Equal(wantRaw) {
				t.Fatalf("state = %s, want %s", state.Raw, wantRaw)
			}
		})
	}
}

func TestEmptyObjectRules(t *testing.T) {
	ctx := context.Background()
	httpType := map[string]attr.Type{"endpoint": types.StringType}
	emptyArm := types.ObjectValueMust(httpType, map[string]attr.Value{"endpoint": types.StringNull()})
	if emptyValue("config.http", emptyArm) {
		t.Fatal("an empty oneOf arm selects the arm, so it is not empty")
	}
	statusType := map[string]attr.Type{"health": types.StringType}
	emptyStatus := types.ObjectValueMust(statusType, map[string]attr.Value{"health": types.StringNull()})
	if !emptyValue("status", emptyStatus) {
		t.Fatal("an object outside a oneOf with only empty attributes is empty")
	}
	var diags diag.Diagnostics
	if got := priorEmpty(ctx, "status", types.ObjectNull(statusType), emptyStatus, &diags); !got.Equal(emptyStatus) {
		t.Fatalf("missing object = %v, want the prior empty object", got)
	}
	if got := priorEmpty(ctx, "config.http", types.ObjectNull(httpType), emptyArm, &diags); !got.IsNull() {
		t.Fatalf("missing arm = %v, want null: the API must return an empty arm", got)
	}
	// After an import the prior is null. A computed attribute keeps the API value.
	if got := priorEmpty(ctx, "status", emptyStatus, types.ObjectNull(statusType), &diags); !got.Equal(emptyStatus) {
		t.Fatalf("computed object after import = %v, want the API value", got)
	}
	if diags.HasError() {
		t.Fatal(diags)
	}
}

// nestedFixture builds Terraform values of the resource with a spec. The
// spec has a normal mode, an immutable region, a computed revision and
// source, items with a computed id and an immutable key, and an immutable
// list of targets with a computed id. target is the name of the one target.
type nestedFixture struct {
	root, spec, item, source, targetType tftypes.Object
	items, targets                       tftypes.List
	target                               string
}

func newNestedFixture(ctx context.Context) nestedFixture {
	root := Schema().Type().TerraformType(ctx).(tftypes.Object)
	spec := root.AttributeTypes["spec"].(tftypes.Object)
	items := spec.AttributeTypes["items"].(tftypes.List)
	targets := spec.AttributeTypes["targets"].(tftypes.List)
	return nestedFixture{root: root, spec: spec, items: items, item: items.ElementType.(tftypes.Object),
		source: spec.AttributeTypes["source"].(tftypes.Object), targets: targets,
		targetType: targets.ElementType.(tftypes.Object), target: "t1"}
}

func (f nestedFixture) withTarget(name string) nestedFixture {
	f.target = name
	return f
}

func tfString(v interface{}) tftypes.Value { return tftypes.NewValue(tftypes.String, v) }

func (f nestedFixture) itemValue(id, key interface{}, name string) tftypes.Value {
	return tftypes.NewValue(f.item, map[string]tftypes.Value{"id": tfString(id), "key": tfString(key), "name": tfString(name)})
}

// specValue is the spec in configuration (server false) or in state (server true).
func (f nestedFixture) specValue(server bool, mode, region string, items ...tftypes.Value) tftypes.Value {
	revision, source, targetID := tfString(nil), tftypes.NewValue(f.source, nil), tfString(nil)
	if server {
		revision = tfString("1")
		source = tftypes.NewValue(f.source, map[string]tftypes.Value{"origin": tfString("api")})
		targetID = tfString("target-1")
	}
	target := tftypes.NewValue(f.targetType, map[string]tftypes.Value{"id": targetID, "name": tfString(f.target)})
	return tftypes.NewValue(f.spec, map[string]tftypes.Value{
		"mode": tfString(mode), "region": tfString(region), "revision": revision, "source": source,
		"items": tftypes.NewValue(f.items, items), "targets": tftypes.NewValue(f.targets, []tftypes.Value{target}),
	})
}

// resourceValue is the resource in configuration (server false) or in state (server true).
func (f nestedFixture) resourceValue(server bool, name string, spec tftypes.Value) tftypes.Value {
	values := nullTFValues(f.root)
	values["name"] = tfString(name)
	values["kind"] = tfString("THING_KIND_STANDARD")
	configType := f.root.AttributeTypes["config"].(tftypes.Object)
	httpType := configType.AttributeTypes["http"].(tftypes.Object)
	values["config"] = tftypes.NewValue(configType, map[string]tftypes.Value{
		"http":  tftypes.NewValue(httpType, map[string]tftypes.Value{"endpoint": tfString("https://example.com")}),
		"queue": tftypes.NewValue(configType.AttributeTypes["queue"], nil),
	})
	values["spec"] = spec
	if server {
		statusType := f.root.AttributeTypes["status"].(tftypes.Object)
		values["id"] = tfString("thing-1")
		values["enabled"] = tftypes.NewValue(tftypes.Bool, true)
		values["status"] = tftypes.NewValue(statusType, map[string]tftypes.Value{"health": tfString("ok")})
		values["create_time"] = tfString("2026-01-01T00:00:00Z")
		values["update_time"] = tfString("2026-01-01T00:00:00Z")
	}
	return tftypes.NewValue(f.root, values)
}

type testProvider struct{}

func (testProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "test"
}
func (testProvider) Schema(context.Context, provider.SchemaRequest, *provider.SchemaResponse) {}
func (testProvider) Configure(context.Context, provider.ConfigureRequest, *provider.ConfigureResponse) {
}
func (testProvider) Resources(context.Context) []func() frameworkresource.Resource {
	return []func() frameworkresource.Resource{NewResource}
}
func (testProvider) DataSources(context.Context) []func() datasource.DataSource { return nil }

// planChange runs the Terraform plan of the resource. proposed is the value
// that Terraform core proposes: the configuration, and the prior value of a
// computed attribute that the configuration leaves null.
func (f nestedFixture) planChange(t *testing.T, prior, config, proposed tftypes.Value) (tftypes.Value, []*tftypes.AttributePath) {
	t.Helper()
	ctx := context.Background()
	server, err := providerserver.NewProtocol6WithError(testProvider{})()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{}); err != nil {
		t.Fatal(err)
	}
	encode := func(v tftypes.Value) *tfprotov6.DynamicValue {
		dv, err := tfprotov6.NewDynamicValue(f.root, v)
		if err != nil {
			t.Fatal(err)
		}
		return &dv
	}
	resp, err := server.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{
		TypeName: "test_" + TypeName, PriorState: encode(prior), Config: encode(config), ProposedNewState: encode(proposed),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Fatalf("plan: %s: %s", d.Summary, d.Detail)
		}
	}
	planned, err := resp.PlannedState.Unmarshal(f.root)
	if err != nil {
		t.Fatal(err)
	}
	return planned, resp.RequiresReplace
}

func TestNestedLifecyclePlans(t *testing.T) {
	ctx := context.Background()
	f := newNestedFixture(ctx)
	prior := f.resourceValue(true, "thing", f.specValue(true, "fast", "eu", f.itemValue("item-1", "k1", "a")))
	keyPath := func(i int) *tftypes.AttributePath {
		return tftypes.NewAttributePath().WithAttributeName("spec").WithAttributeName("items").WithElementKeyInt(i).WithAttributeName("key")
	}
	tests := map[string]struct {
		config, proposed tftypes.Value
		replace          *tftypes.AttributePath // nil: an update or no change
		mask             string                 // the update mask of an update
	}{
		"no change": {
			config:   f.resourceValue(false, "thing", f.specValue(false, "fast", "eu", f.itemValue(nil, "k1", "a"))),
			proposed: prior,
		},
		"top-level change leaves the spec out of the mask": {
			config:   f.resourceValue(false, "renamed", f.specValue(false, "fast", "eu", f.itemValue(nil, "k1", "a"))),
			proposed: f.resourceValue(true, "renamed", f.specValue(true, "fast", "eu", f.itemValue("item-1", "k1", "a"))),
			mask:     "name",
		},
		"normal nested change": {
			config:   f.resourceValue(false, "thing", f.specValue(false, "slow", "eu", f.itemValue(nil, "k1", "a"))),
			proposed: f.resourceValue(true, "thing", f.specValue(true, "slow", "eu", f.itemValue("item-1", "k1", "a"))),
			mask:     "spec.mode",
		},
		"immutable nested change": {
			config:   f.resourceValue(false, "thing", f.specValue(false, "fast", "us", f.itemValue(nil, "k1", "a"))),
			proposed: f.resourceValue(true, "thing", f.specValue(true, "fast", "us", f.itemValue("item-1", "k1", "a"))),
			replace:  tftypes.NewAttributePath().WithAttributeName("spec").WithAttributeName("region"),
		},
		"new item with an immutable value": {
			config:   f.resourceValue(false, "thing", f.specValue(false, "fast", "eu", f.itemValue(nil, "k1", "a"), f.itemValue(nil, "k2", "b"))),
			proposed: f.resourceValue(true, "thing", f.specValue(true, "fast", "eu", f.itemValue("item-1", "k1", "a"), f.itemValue(nil, "k2", "b"))),
			replace:  keyPath(1),
		},
		"changed immutable list": {
			config:   f.withTarget("t2").resourceValue(false, "thing", f.withTarget("t2").specValue(false, "fast", "eu", f.itemValue(nil, "k1", "a"))),
			proposed: f.withTarget("t2").resourceValue(true, "thing", f.withTarget("t2").specValue(true, "fast", "eu", f.itemValue("item-1", "k1", "a"))),
			replace:  tftypes.NewAttributePath().WithAttributeName("spec").WithAttributeName("targets"),
		},
		"new item without an immutable value": {
			config:   f.resourceValue(false, "thing", f.specValue(false, "fast", "eu", f.itemValue(nil, "k1", "a"), f.itemValue(nil, nil, "b"))),
			proposed: f.resourceValue(true, "thing", f.specValue(true, "fast", "eu", f.itemValue("item-1", "k1", "a"), f.itemValue(nil, nil, "b"))),
			mask:     "spec.items",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			planned, replace := f.planChange(t, prior, test.config, test.proposed)
			if test.replace != nil {
				if !slices.ContainsFunc(replace, test.replace.Equal) {
					t.Fatalf("requires replace = %v, want %v", replace, test.replace)
				}
				return
			}
			if len(replace) != 0 {
				t.Fatalf("requires replace = %v, want none", replace)
			}
			if test.mask == "" {
				if !planned.Equal(prior) {
					t.Fatalf("planned = %s, want the prior state", planned)
				}
				return
			}
			s := Schema()
			_, mask, diags := updateRequest(ctx, tfsdk.Config{Schema: s, Raw: test.config}, tfsdk.Plan{Schema: s, Raw: planned}, tfsdk.State{Schema: s, Raw: prior})
			if diags.HasError() || mask != test.mask {
				t.Fatalf("update mask = %q, %v; want %q", mask, diags, test.mask)
			}
		})
	}
}

func TestNestedServerValuesDoNotBlockCreate(t *testing.T) {
	ctx := context.Background()
	f := newNestedFixture(ctx)
	s := Schema()
	config := f.resourceValue(false, "thing", f.specValue(false, "fast", "eu", f.itemValue(nil, "k1", "a")))
	plan := f.resourceValue(false, "thing", tftypes.NewValue(f.spec, map[string]tftypes.Value{
		"mode": tfString("fast"), "region": tfString("eu"), "revision": tfString(tftypes.UnknownValue),
		"source": tftypes.NewValue(f.source, tftypes.UnknownValue),
		"items":  tftypes.NewValue(f.items, []tftypes.Value{f.itemValue(tftypes.UnknownValue, "k1", "a")}),
		"targets": tftypes.NewValue(f.targets, []tftypes.Value{tftypes.NewValue(f.targetType, map[string]tftypes.Value{
			"id": tfString(tftypes.UnknownValue), "name": tfString("t1"),
		})}),
	}))
	if diags := validateCreate(ctx, tfsdk.Config{Schema: s, Raw: config}, tfsdk.Plan{Schema: s, Raw: plan}); diags.HasError() {
		t.Fatalf("unknown server values blocked Create: %v", diags)
	}
	plan = f.resourceValue(false, "thing", f.specValue(false, "fast", "eu", f.itemValue(nil, tftypes.UnknownValue, "a")))
	if diags := validateCreate(ctx, tfsdk.Config{Schema: s, Raw: plan}, tfsdk.Plan{Schema: s, Raw: plan}); !diags.HasError() {
		t.Fatal("an unknown configured nested value was accepted")
	}
}
