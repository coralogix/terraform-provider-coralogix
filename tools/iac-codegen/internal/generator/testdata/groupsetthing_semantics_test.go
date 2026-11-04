package groupsetthing

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"example.com/iac-test-sdk/go/openapi/gen/group_set_things_service"
)

func setSchema(t *testing.T) (*Resource, frameworkresource.SchemaResponse, tftypes.Object) {
	t.Helper()
	r := &Resource{client: &group_set_things_service.GroupSetThingsServiceAPIService{}}
	var schemaResp frameworkresource.SchemaResponse
	r.Schema(context.Background(), frameworkresource.SchemaRequest{}, &schemaResp)
	return r, schemaResp, schemaResp.Schema.Type().TerraformType(context.Background()).(tftypes.Object)
}

func pathRoot(name string) path.Path { return path.Root(name) }

func num(n int64) tftypes.Value { return tftypes.NewValue(tftypes.Number, big.NewFloat(float64(n))) }

// plannedSet is the plan of a set with one group: an unknown id, name, and interval, a limit of 5,
// and one item with unknown labels, which the server fills in.
func plannedSet(root tftypes.Object) tftypes.Value {
	groups := root.AttributeTypes["groups"].(tftypes.Set)
	group := groups.ElementType.(tftypes.Object)
	items := group.AttributeTypes["items"].(tftypes.List)
	item := items.ElementType.(tftypes.Object)
	labels := item.AttributeTypes["labels"].(tftypes.Map)
	itemValue := tftypes.NewValue(item, map[string]tftypes.Value{
		"key":    tftypes.NewValue(tftypes.String, "k"),
		"labels": tftypes.NewValue(labels, tftypes.UnknownValue),
	})
	groupValue := tftypes.NewValue(group, map[string]tftypes.Value{
		"name":     tftypes.NewValue(tftypes.String, "g"),
		"interval": tftypes.NewValue(tftypes.Number, tftypes.UnknownValue),
		"limit":    num(5),
		"items":    tftypes.NewValue(items, []tftypes.Value{itemValue}),
	})
	return tftypes.NewValue(root, map[string]tftypes.Value{
		"id":       tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"name":     tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"groups":   tftypes.NewValue(groups, []tftypes.Value{groupValue}),
		"document": tftypes.NewValue(tftypes.String, "doc"),
	})
}

func TestCreateSendsTheLimitAsAStringAndReadsItBack(t *testing.T) {
	group_set_things_service.Created = nil
	r, schemaResp, root := setSchema(t)
	plan := plannedSet(root)
	resp := &frameworkresource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(root, nil)}}
	r.Create(context.Background(), frameworkresource.CreateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: plan},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if len(group_set_things_service.Created) != 1 || len(group_set_things_service.Created[0].Groups) != 1 {
		t.Fatalf("create bodies = %+v, want one with one group", group_set_things_service.Created)
	}
	sent := group_set_things_service.Created[0].Groups[0]
	if sent.Limit == nil || *sent.Limit != "5" {
		t.Fatalf("sent limit = %v, want the string 5", sent.Limit)
	}
	if sent.Interval != nil {
		t.Fatalf("sent interval = %d, want none: the server stores its default", *sent.Interval)
	}
	if body := group_set_things_service.Created[0]; body.Name != nil {
		t.Fatalf("sent name = %q, want none for an unknown name", *body.Name)
	}

	var state struct {
		Name     string `tfsdk:"name"`
		Document string `tfsdk:"document"`
	}
	var groups []struct {
		Name     string     `tfsdk:"name"`
		Interval int64      `tfsdk:"interval"`
		Limit    int64      `tfsdk:"limit"`
		Items    types.List `tfsdk:"items"`
	}
	if d := resp.State.GetAttribute(context.Background(), pathRoot("name"), &state.Name); d.HasError() {
		t.Fatal(d)
	}
	if d := resp.State.GetAttribute(context.Background(), pathRoot("document"), &state.Document); d.HasError() {
		t.Fatal(d)
	}
	if d := resp.State.GetAttribute(context.Background(), pathRoot("groups"), &groups); d.HasError() {
		t.Fatal(d)
	}
	if state.Name != "set-1" || state.Document != "doc" {
		t.Fatalf("state name %q and document %q, want the id as the name and the planned document", state.Name, state.Document)
	}
	if len(groups) != 1 || groups[0].Interval != 60 || groups[0].Limit != 5 {
		t.Fatalf("state groups = %+v, want interval 60 from the server and limit 5", groups)
	}
}

// A state upgrade reads the set from the API, and keeps the document of the prior state: the API
// holds none.
func TestUpgradeKeepsTheDocumentOfThePriorState(t *testing.T) {
	ctx := context.Background()
	limit, interval := "0", int64(60)
	group_set_things_service.Stored = group_set_things_service.GroupSetThing{Id: "set-1", Name: "s", Groups: []group_set_things_service.OutGroupThing{{Name: "g", Interval: &interval, Limit: &limit}}}
	r, schemaResp, root := setSchema(t)
	upgrader := r.UpgradeState(ctx)[0]
	priorType := upgrader.PriorSchema.Type().TerraformType(ctx).(tftypes.Object)
	prior := tftypes.NewValue(priorType, map[string]tftypes.Value{
		"id":       tftypes.NewValue(tftypes.String, "set-1"),
		"document": tftypes.NewValue(tftypes.String, "doc"),
		"group":    tftypes.NewValue(tftypes.String, "old"),
	})
	resp := &frameworkresource.UpgradeStateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(root, nil)}}
	upgrader.StateUpgrader(ctx, frameworkresource.UpgradeStateRequest{State: &tfsdk.State{Schema: *upgrader.PriorSchema, Raw: prior}}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	var document, name string
	if d := resp.State.GetAttribute(ctx, pathRoot("document"), &document); d.HasError() {
		t.Fatal(d)
	}
	if d := resp.State.GetAttribute(ctx, pathRoot("name"), &name); d.HasError() {
		t.Fatal(d)
	}
	if document != "doc" || name != "s" {
		t.Fatalf("upgraded document %q and name %q, want the prior document and the name of the API", document, name)
	}
}

// groupValue is a group with one item and an interval and a limit that are null when they are nil.
func groupValue(root tftypes.Object, name string, interval, limit *int64) tftypes.Value {
	group := root.AttributeTypes["groups"].(tftypes.Set).ElementType.(tftypes.Object)
	items := group.AttributeTypes["items"].(tftypes.List)
	item := items.ElementType.(tftypes.Object)
	number := func(n *int64) tftypes.Value {
		if n == nil {
			return tftypes.NewValue(tftypes.Number, nil)
		}
		return num(*n)
	}
	return tftypes.NewValue(group, map[string]tftypes.Value{
		"name":     tftypes.NewValue(tftypes.String, name),
		"interval": number(interval),
		"limit":    number(limit),
		"items": tftypes.NewValue(items, []tftypes.Value{tftypes.NewValue(item, map[string]tftypes.Value{
			"key":    tftypes.NewValue(tftypes.String, "k"),
			"labels": tftypes.NewValue(item.AttributeTypes["labels"], nil),
		})}),
	})
}

// thingWithGroups is a configuration that sets the groups.
func thingWithGroups(root tftypes.Object, groups ...tftypes.Value) tftypes.Value {
	return tftypes.NewValue(root, map[string]tftypes.Value{
		"id":       tftypes.NewValue(tftypes.String, nil),
		"name":     tftypes.NewValue(tftypes.String, nil),
		"groups":   tftypes.NewValue(root.AttributeTypes["groups"], groups),
		"document": tftypes.NewValue(tftypes.String, nil),
	})
}

// groupPath is the path of the interval of the planned group.
func groupPath(t *testing.T, schemaResp frameworkresource.SchemaResponse, planned tftypes.Value) path.Path {
	t.Helper()
	elem, err := schemaResp.Schema.Attributes["groups"].GetType().(types.SetType).ElemType.ValueFromTerraform(context.Background(), planned)
	if err != nil {
		t.Fatal(err)
	}
	return path.Root("groups").AtSetValue(elem).AtName("interval")
}

func int64p(n int64) *int64 { return &n }

// Inside a set item the server default reads the plan, which Terraform proposes from the prior item
// that it paired with the configured one, and the configuration of the item that the planned item
// matches. The framework gives it the state and the configuration of the item at the same index,
// here another group with interval 120 that the configuration sets.
func TestServerDefaultInASetReadsThePlan(t *testing.T) {
	ctx := context.Background()
	m := serverDefaultModifier{value: types.Int64Value(60), fromPlan: true}
	for _, test := range []struct {
		name        string
		plan        int64
		wantUnknown bool
	}{
		{name: "the paired item has the default", plan: 60},
		{name: "the paired item has another value that the config removes", plan: 90, wantUnknown: true},
	} {
		_, schemaResp, root := setSchema(t)
		config := thingWithGroups(root, groupValue(root, "g", nil, nil), groupValue(root, "other", int64p(120), nil))
		req := planmodifier.Int64Request{
			Path:        groupPath(t, schemaResp, groupValue(root, "g", int64p(test.plan), int64p(0))),
			Config:      tfsdk.Config{Schema: schemaResp.Schema, Raw: config},
			ConfigValue: types.Int64Value(120),
			StateValue:  types.Int64Value(120),
			PlanValue:   types.Int64Value(test.plan),
			Plan:        tfsdk.Plan{Schema: schemaResp.Schema, Raw: config},
		}
		resp := &planmodifier.Int64Response{PlanValue: req.PlanValue}
		m.PlanModifyInt64(ctx, req, resp)
		if resp.PlanValue.IsUnknown() != test.wantUnknown {
			t.Errorf("%s: plan = %v, want unknown %t", test.name, resp.PlanValue, test.wantUnknown)
		}
	}
}

// A planned item whose interval comes from its configuration keeps it, even when the framework
// gives the modifier the configuration of another item that leaves the interval out.
func TestServerDefaultInASetKeepsAConfiguredValue(t *testing.T) {
	ctx := context.Background()
	m := serverDefaultModifier{value: types.Int64Value(60), fromPlan: true}
	_, schemaResp, root := setSchema(t)
	config := thingWithGroups(root, groupValue(root, "a", nil, nil), groupValue(root, "b", int64p(90), nil))
	req := planmodifier.Int64Request{
		Path:        groupPath(t, schemaResp, groupValue(root, "b", int64p(90), int64p(0))),
		Config:      tfsdk.Config{Schema: schemaResp.Schema, Raw: config},
		ConfigValue: types.Int64Null(),
		StateValue:  types.Int64Value(60),
		PlanValue:   types.Int64Value(90),
		Plan:        tfsdk.Plan{Schema: schemaResp.Schema, Raw: config},
	}
	resp := &planmodifier.Int64Response{PlanValue: req.PlanValue}
	m.PlanModifyInt64(ctx, req, resp)
	if !resp.PlanValue.Equal(types.Int64Value(90)) {
		t.Fatalf("plan = %v, want the configured 90", resp.PlanValue)
	}
}

// A set that the configuration leaves out, which the document sets instead, keeps the values of its
// items: a null configuration value of an item is no removal there.
func TestServerDefaultInAnUnconfiguredSetKeepsThePlan(t *testing.T) {
	ctx := context.Background()
	_, schemaResp, root := setSchema(t)
	values := map[string]tftypes.Value{}
	for name, typ := range root.AttributeTypes {
		values[name] = tftypes.NewValue(typ, nil)
	}
	values["document"] = tftypes.NewValue(tftypes.String, "doc")
	config := tftypes.NewValue(root, values)
	m := serverDefaultModifier{value: types.Int64Value(60), fromPlan: true}
	req := planmodifier.Int64Request{
		Path:        path.Root("groups").AtSetValue(types.StringValue("g")).AtName("interval"),
		Config:      tfsdk.Config{Schema: schemaResp.Schema, Raw: config},
		ConfigValue: types.Int64Null(),
		PlanValue:   types.Int64Value(120),
		Plan:        tfsdk.Plan{Schema: schemaResp.Schema, Raw: config},
	}
	resp := &planmodifier.Int64Response{PlanValue: req.PlanValue}
	m.PlanModifyInt64(ctx, req, resp)
	if !resp.PlanValue.Equal(types.Int64Value(120)) {
		t.Fatalf("plan = %v, want the planned 120 of a set that the document sets", resp.PlanValue)
	}
}

// A state upgrade restores the document of the prior state before AfterFlatten, so a hook that
// derives the document from the API value wins over the prior one.
func TestUpgradeRunsAfterFlattenOnTheKeptDocument(t *testing.T) {
	ctx := context.Background()
	limit, interval := "0", int64(60)
	group_set_things_service.Stored = group_set_things_service.GroupSetThing{Id: "set-1", Name: "s", Groups: []group_set_things_service.OutGroupThing{{Name: "g", Interval: &interval, Limit: &limit}}}
	r, schemaResp, root := setSchema(t)
	r.hooks.AfterFlatten = func(_ context.Context, m *GroupSetThingModel, v *group_set_things_service.GroupSetThing) diag.Diagnostics {
		if m.Document.ValueString() == "doc" {
			m.Document = types.StringValue("from " + v.Name)
		}
		return nil
	}
	upgrader := r.UpgradeState(ctx)[0]
	priorType := upgrader.PriorSchema.Type().TerraformType(ctx).(tftypes.Object)
	prior := tftypes.NewValue(priorType, map[string]tftypes.Value{
		"id":       tftypes.NewValue(tftypes.String, "set-1"),
		"document": tftypes.NewValue(tftypes.String, "doc"),
		"group":    tftypes.NewValue(tftypes.String, "old"),
	})
	resp := &frameworkresource.UpgradeStateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(root, nil)}}
	upgrader.StateUpgrader(ctx, frameworkresource.UpgradeStateRequest{State: &tfsdk.State{Schema: *upgrader.PriorSchema, Raw: prior}}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	var document string
	if d := resp.State.GetAttribute(ctx, pathRoot("document"), &document); d.HasError() {
		t.Fatal(d)
	}
	if document != "from s" {
		t.Fatalf("upgraded document %q, want the value of AfterFlatten", document)
	}
}

// setItemConfig follows the path down to the set: a set inside a configured object that the
// configuration leaves out is not configured.
func TestSetItemConfigChecksTheContainingSet(t *testing.T) {
	ctx := context.Background()
	itemType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"interval": tftypes.Number}}
	settingsType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"other": tftypes.String, "groups": tftypes.Set{ElementType: itemType}}}
	rootType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"settings": settingsType}}
	element := tftypes.NewValue(itemType, map[string]tftypes.Value{"interval": tftypes.NewValue(tftypes.Number, nil)})
	planned := types.ObjectValueMust(map[string]attr.Type{"interval": types.Int64Type}, map[string]attr.Value{"interval": types.Int64Value(60)})
	at := path.Root("settings").AtName("groups").AtSetValue(planned).AtName("interval")
	for _, test := range []struct {
		name     string
		settings tftypes.Value
		want     bool
	}{
		{"no settings", tftypes.NewValue(settingsType, nil), false},
		{"settings without groups", tftypes.NewValue(settingsType, map[string]tftypes.Value{"other": tftypes.NewValue(tftypes.String, "x"), "groups": tftypes.NewValue(tftypes.Set{ElementType: itemType}, nil)}), false},
		{"settings with groups", tftypes.NewValue(settingsType, map[string]tftypes.Value{"other": tftypes.NewValue(tftypes.String, nil), "groups": tftypes.NewValue(tftypes.Set{ElementType: itemType}, []tftypes.Value{element})}), true},
	} {
		config := tftypes.NewValue(rootType, map[string]tftypes.Value{"settings": test.settings})
		if value, got := setItemConfig(ctx, config, at); got != test.want || (got && !value.IsNull()) {
			t.Errorf("%s: setItemConfig = %v, %t, want null, %t", test.name, value, got, test.want)
		}
	}
}

type setProvider struct{}

func (setProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "test"
}

func (setProvider) Schema(context.Context, provider.SchemaRequest, *provider.SchemaResponse) {}

func (setProvider) Configure(context.Context, provider.ConfigureRequest, *provider.ConfigureResponse) {}

func (setProvider) Resources(context.Context) []func() frameworkresource.Resource {
	return []func() frameworkresource.Resource{func() frameworkresource.Resource {
		return NewResourceWithClient(&group_set_things_service.GroupSetThingsServiceAPIService{}, Hooks{})
	}}
}

func (setProvider) DataSources(context.Context) []func() datasource.DataSource { return nil }

// setConfig is a group set whose groups have the given HCL attributes next to their name and item.
func setConfig(groups map[string]string) string {
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	b.WriteString("resource \"test_group_set_thing\" \"s\" {\n  groups = [\n")
	for _, name := range names {
		fmt.Fprintf(&b, "    { name = %q, items = [{ key = \"k\" }]%s },\n", name, groups[name])
	}
	b.WriteString("  ]\n}\n")
	return b.String()
}

func checkGroup(name string, interval, limit int64) resource.TestCheckFunc {
	return resource.TestCheckTypeSetElemNestedAttrs("test_group_set_thing.s", "groups.*", map[string]string{
		"name":     name,
		"interval": strconv.FormatInt(interval, 10),
		"limit":    strconv.FormatInt(limit, 10),
	})
}

// With real Terraform, a set item that leaves out a field with a server default sits next to one
// that sets it. The framework pairs the configuration and the plan of set items by index, and the
// two sort their items apart, so the modifier must not read the configuration of another item.
// The test framework fails a step whose plan after the apply is not empty. Each case runs for
// several pairs of names, so that some pair sorts the plan and the configuration apart.
func TestServerDefaultsInASetWithRealTerraform(t *testing.T) {
	factories := map[string]func() (tfprotov6.ProviderServer, error){"test": providerserver.NewProtocol6WithError(setProvider{})}
	for _, names := range [][2]string{{"a", "b"}, {"b", "a"}, {"x", "y"}, {"first", "second"}} {
		a, b := names[0], names[1]
		for _, test := range []struct {
			name  string
			steps []resource.TestStep
		}{
			{"an item without the interval next to one with it", []resource.TestStep{{
				Config: setConfig(map[string]string{a: "", b: ", interval = 90"}),
				Check:  resource.ComposeTestCheckFunc(checkGroup(a, 60, 0), checkGroup(b, 90, 0)),
			}}},
			{"an item without the limit next to one with it", []resource.TestStep{{
				Config: setConfig(map[string]string{a: "", b: ", limit = 5"}),
				Check:  resource.ComposeTestCheckFunc(checkGroup(a, 60, 0), checkGroup(b, 60, 5)),
			}}},
			{"each item sets the field that the other leaves out", []resource.TestStep{{
				Config: setConfig(map[string]string{a: ", limit = 5", b: ", interval = 90"}),
				Check:  resource.ComposeTestCheckFunc(checkGroup(a, 60, 5), checkGroup(b, 90, 0)),
			}}},
			{"three items", []resource.TestStep{{
				Config: setConfig(map[string]string{a: "", b: ", interval = 90, limit = 5", "c": ", interval = 120"}),
				Check:  resource.ComposeTestCheckFunc(checkGroup(a, 60, 0), checkGroup(b, 90, 5), checkGroup("c", 120, 0)),
			}}},
			{"an item removes its interval next to one that keeps it", []resource.TestStep{
				{Config: setConfig(map[string]string{a: ", interval = 120", b: ", interval = 90"})},
				{
					Config: setConfig(map[string]string{a: "", b: ", interval = 90"}),
					Check:  resource.ComposeTestCheckFunc(checkGroup(a, 60, 0), checkGroup(b, 90, 0)),
				},
			}},
			{"an item sets the interval that it left out", []resource.TestStep{
				{Config: setConfig(map[string]string{a: "", b: ""})},
				{
					Config: setConfig(map[string]string{a: "", b: ", interval = 90"}),
					Check:  resource.ComposeTestCheckFunc(checkGroup(a, 60, 0), checkGroup(b, 90, 0)),
				},
			}},
		} {
			t.Run(a+"_"+b+"/"+test.name, func(t *testing.T) {
				resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: factories, Steps: test.steps})
			})
		}
	}
}
