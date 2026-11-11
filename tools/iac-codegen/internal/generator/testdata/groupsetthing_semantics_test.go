package groupsetthing

import (
	"context"
	"fmt"
	"math/big"
	"regexp"
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
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

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
		"period":   tftypes.NewValue(group.AttributeTypes["period"], tftypes.UnknownValue),
		"items":    tftypes.NewValue(items, []tftypes.Value{itemValue}),
	})
	return tftypes.NewValue(root, map[string]tftypes.Value{
		"id":       tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"name":     tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"groups":   tftypes.NewValue(groups, []tftypes.Value{groupValue}),
		"document": tftypes.NewValue(tftypes.String, "doc"),
		"schedule": tftypes.NewValue(root.AttributeTypes["schedule"], nil),
		"window":   tftypes.NewValue(root.AttributeTypes["window"], nil),
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
		Name     string       `tfsdk:"name"`
		Interval int64        `tfsdk:"interval"`
		Limit    int64        `tfsdk:"limit"`
		Items    types.List   `tfsdk:"items"`
		Period   types.Object `tfsdk:"period"`
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
		"period":   tftypes.NewValue(group.AttributeTypes["period"], nil),
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
		"schedule": tftypes.NewValue(root.AttributeTypes["schedule"], nil),
		"window":   tftypes.NewValue(root.AttributeTypes["window"], nil),
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

func (setProvider) Configure(context.Context, provider.ConfigureRequest, *provider.ConfigureResponse) {
}

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

// planGroupIntervals is a ModifyPlan hook that plans a value inside the items of the set of groups.
// The items of the plan, the configuration, and the state are sorted apart, so it does not read the
// configuration or the state at the position of an item. It rebuilds the plan with tftypes.Transform,
// which hands it each item as a whole, and sets the interval of the items that leave it unknown.
func planGroupIntervals(interval int64) func(context.Context, frameworkresource.ModifyPlanRequest, *frameworkresource.ModifyPlanResponse) {
	return func(_ context.Context, _ frameworkresource.ModifyPlanRequest, resp *frameworkresource.ModifyPlanResponse) {
		planned, err := tftypes.Transform(resp.Plan.Raw, func(p *tftypes.AttributePath, v tftypes.Value) (tftypes.Value, error) {
			object, ok := v.Type().(tftypes.Object)
			steps := p.Steps()
			if !ok || !v.IsKnown() || v.IsNull() || len(steps) != 2 || steps[0] != tftypes.AttributeName("groups") {
				return v, nil
			}
			if _, isItem := steps[1].(tftypes.ElementKeyValue); !isItem {
				return v, nil
			}
			var fields map[string]tftypes.Value
			if err := v.As(&fields); err != nil {
				return v, err
			}
			if fields["interval"].IsKnown() {
				return v, nil
			}
			fields["interval"] = num(interval)
			return tftypes.NewValue(object, fields), nil
		})
		if err != nil {
			resp.Diagnostics.AddError("plan", err.Error())
			return
		}
		resp.Plan.Raw = planned
	}
}

// The hook of a resource without an equality line is called by the generated ModifyPlan, and it
// plans a value inside a set item. A destroy plan and a resource without the hook plan nothing.
func TestModifyPlanHookPlansAValueInASetItem(t *testing.T) {
	ctx := context.Background()
	r, schemaResp, root := setSchema(t)
	r.hooks.ModifyPlan = planGroupIntervals(90)
	plan := plannedSet(root)
	state := tftypes.NewValue(root, nil)
	req := frameworkresource.ModifyPlanRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: plan},
		State:  tfsdk.State{Schema: schemaResp.Schema, Raw: state},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
	}
	resp := &frameworkresource.ModifyPlanResponse{Plan: req.Plan}
	r.ModifyPlan(ctx, req, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	var groups []struct {
		Name     string       `tfsdk:"name"`
		Interval int64        `tfsdk:"interval"`
		Limit    int64        `tfsdk:"limit"`
		Items    types.List   `tfsdk:"items"`
		Period   types.Object `tfsdk:"period"`
	}
	if d := resp.Plan.GetAttribute(ctx, pathRoot("groups"), &groups); d.HasError() {
		t.Fatal(d)
	}
	if len(groups) != 1 || groups[0].Interval != 90 || groups[0].Limit != 5 {
		t.Fatalf("planned groups = %+v, want the interval of the hook and the limit of the plan", groups)
	}

	destroy := frameworkresource.ModifyPlanRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: state},
		State:  tfsdk.State{Schema: schemaResp.Schema, Raw: plan},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: state},
	}
	called := false
	r.hooks.ModifyPlan = func(context.Context, frameworkresource.ModifyPlanRequest, *frameworkresource.ModifyPlanResponse) {
		called = true
	}
	r.ModifyPlan(ctx, destroy, &frameworkresource.ModifyPlanResponse{Plan: destroy.Plan})
	r.hooks.ModifyPlan = nil
	r.ModifyPlan(ctx, req, &frameworkresource.ModifyPlanResponse{Plan: req.Plan})
	if called {
		t.Fatal("the hook ran on destroy")
	}
}

// An equality line of an extra attribute adds the plan modifier of the attribute, and no ModifyPlan
// of its own: the resource has a server default, which that ModifyPlan would lose. The same JSON in
// another format keeps the state text, and another document does not.
func TestExtraDocumentKeepsTheStateOfEqualJSON(t *testing.T) {
	ctx := context.Background()
	_, schemaResp, _ := setSchema(t)
	document, ok := schemaResp.Schema.Attributes["document"].(resourceschema.StringAttribute)
	if !ok || len(document.PlanModifiers) == 0 {
		t.Fatalf("document = %#v, want a string attribute with the plan modifier of the equality line", schemaResp.Schema.Attributes["document"])
	}
	for _, test := range []struct{ config, want string }{
		{"{ \"a\": 1 }", "{\"a\":1}"},
		{"{\"a\": 2}", "{\"a\": 2}"},
	} {
		req := planmodifier.StringRequest{ConfigValue: types.StringValue(test.config), StateValue: types.StringValue("{\"a\":1}"), PlanValue: types.StringValue(test.config)}
		resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}
		document.PlanModifiers[0].PlanModifyString(ctx, req, resp)
		if resp.PlanValue.ValueString() != test.want {
			t.Errorf("config %s: plan %s, want %s", test.config, resp.PlanValue, test.want)
		}
	}
}

// periodOf finds the group with the name in the flat state of the set, and returns the minutes of its
// period. ok is false when the group has no period.
func periodOf(state *terraform.State, name string) (minutes string, found, ok bool) {
	attrs := state.RootModule().Resources["test_group_set_thing.s"].Primary.Attributes
	for key, value := range attrs {
		if strings.HasPrefix(key, "groups.") && strings.HasSuffix(key, ".name") && value == name {
			found = true
			minutes, ok = attrs[strings.TrimSuffix(key, ".name")+".period.period_minutes"]
		}
	}
	return minutes, found, ok
}

// checkPeriod checks the minutes of the period of a group, "" for a group with no period.
func checkPeriod(name, want string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		minutes, found, ok := periodOf(state, name)
		switch {
		case !found:
			return fmt.Errorf("the state has no group %q", name)
		case want == "" && ok:
			return fmt.Errorf("group %q has the period %s, want none", name, minutes)
		case want != "" && minutes != want:
			return fmt.Errorf("group %q has the period %q, want %q", name, minutes, want)
		}
		return nil
	}
}

// With real Terraform, the optional period of a group is an object that the server fills in. A group
// that leaves it out sits next to one that sets it, and a group that changes sits next to another
// that changes or keeps its period. The framework gives a modifier inside a set item the state of the
// item at the same index, which can be another item, so the built-in UseStateForUnknown gave a
// group the period of its neighbour. The generated modifier takes the state item that the planned
// item matches, so a group that changes keeps an unknown period, which the API then answers. The
// test framework fails a step whose plan after the apply is not empty. Each case runs for several
// pairs of names, so that some pair sorts the plan and the state apart.
func TestOptionalPeriodInASetWithRealTerraform(t *testing.T) {
	factories := map[string]func() (tfprotov6.ProviderServer, error){"test": providerserver.NewProtocol6WithError(setProvider{})}
	period := func(minutes int) string { return fmt.Sprintf(", period = { period_minutes = %d }", minutes) }
	for _, names := range [][2]string{{"a", "b"}, {"b", "a"}, {"x", "y"}, {"first", "second"}} {
		a, b := names[0], names[1]
		for _, test := range []struct {
			name  string
			steps []resource.TestStep
		}{
			{"a group without the period next to one with it", []resource.TestStep{{
				Config: setConfig(map[string]string{a: "", b: period(7)}),
				Check:  resource.ComposeTestCheckFunc(checkPeriod(a, ""), checkPeriod(b, "7")),
			}}},
			{"each group sets its own period", []resource.TestStep{{
				Config: setConfig(map[string]string{a: period(3), b: period(7)}),
				Check:  resource.ComposeTestCheckFunc(checkPeriod(a, "3"), checkPeriod(b, "7")),
			}}},
			{"a group changes its period next to one that keeps it", []resource.TestStep{
				{Config: setConfig(map[string]string{a: period(3), b: period(7)})},
				{
					Config: setConfig(map[string]string{a: period(3), b: period(9)}),
					Check:  resource.ComposeTestCheckFunc(checkPeriod(a, "3"), checkPeriod(b, "9")),
				},
			}},
			{"two groups that change and leave the period out never take the period of each other", []resource.TestStep{
				{Config: setConfig(map[string]string{a: period(3), b: period(7)})},
				{
					Config: setConfig(map[string]string{a: ", limit = 5", b: ", limit = 6"}),
					Check:  resource.ComposeTestCheckFunc(checkPeriod(a, ""), checkPeriod(b, "")),
				},
			}},
			{"a group that changes next to one that sets its period", []resource.TestStep{
				{Config: setConfig(map[string]string{a: period(3), b: period(7)})},
				{
					Config: setConfig(map[string]string{a: ", limit = 5", b: period(7)}),
					Check:  resource.ComposeTestCheckFunc(checkPeriod(a, ""), checkPeriod(b, "7")),
				},
			}},
			{"a group that is new next to one that keeps its period", []resource.TestStep{
				{Config: setConfig(map[string]string{a: period(3)})},
				{
					Config: setConfig(map[string]string{a: period(3), b: ""}),
					Check:  resource.ComposeTestCheckFunc(checkPeriod(a, "3"), checkPeriod(b, "")),
				},
			}},
		} {
			t.Run(a+"_"+b+"/"+test.name, func(t *testing.T) {
				resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: factories, Steps: test.steps})
			})
		}
	}
}

// setItemStateModifier takes the state of the item that the planned item matches, not the item at the
// same index: the state holds the groups a (period 3) and b (period 7), and the plan lists them in
// the other order. A planned item that changed matches no state item and keeps its unknown period.
func TestSetItemStateModifierTakesTheMatchingItem(t *testing.T) {
	ctx := context.Background()
	_, schemaResp, root := setSchema(t)
	group := root.AttributeTypes["groups"].(tftypes.Set).ElementType.(tftypes.Object)
	items := group.AttributeTypes["items"].(tftypes.List)
	item := items.ElementType.(tftypes.Object)
	period := group.AttributeTypes["period"]
	stateGroup := func(name string, minutes int64, limit int64) tftypes.Value {
		return tftypes.NewValue(group, map[string]tftypes.Value{
			"name": tftypes.NewValue(tftypes.String, name), "interval": num(60), "limit": num(limit),
			"period": tftypes.NewValue(period, map[string]tftypes.Value{"period_minutes": num(minutes)}),
			"items": tftypes.NewValue(items, []tftypes.Value{tftypes.NewValue(item, map[string]tftypes.Value{
				"key": tftypes.NewValue(tftypes.String, "k"), "labels": tftypes.NewValue(item.AttributeTypes["labels"], map[string]tftypes.Value{}),
			})}),
		})
	}
	planned := func(name string, limit int64) tftypes.Value {
		return tftypes.NewValue(group, map[string]tftypes.Value{
			"name": tftypes.NewValue(tftypes.String, name), "interval": tftypes.NewValue(tftypes.Number, tftypes.UnknownValue), "limit": num(limit),
			"period": tftypes.NewValue(period, tftypes.UnknownValue),
			"items": tftypes.NewValue(items, []tftypes.Value{tftypes.NewValue(item, map[string]tftypes.Value{
				"key": tftypes.NewValue(tftypes.String, "k"), "labels": tftypes.NewValue(item.AttributeTypes["labels"], tftypes.UnknownValue),
			})}),
		})
	}
	state := tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(root, map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, "set-1"), "name": tftypes.NewValue(tftypes.String, "s"), "document": tftypes.NewValue(tftypes.String, nil),
		"schedule": tftypes.NewValue(root.AttributeTypes["schedule"], nil),
		"window":   tftypes.NewValue(root.AttributeTypes["window"], nil),
		"groups":   tftypes.NewValue(root.AttributeTypes["groups"], []tftypes.Value{stateGroup("b", 7, 0), stateGroup("a", 3, 0)}),
	})}
	for _, test := range []struct {
		name  string
		group tftypes.Value
		want  tftypes.Value
	}{
		{"a", planned("a", 0), tftypes.NewValue(period, map[string]tftypes.Value{"period_minutes": num(3)})},
		{"b", planned("b", 0), tftypes.NewValue(period, map[string]tftypes.Value{"period_minutes": num(7)})},
		{"a with another limit", planned("a", 5), tftypes.NewValue(period, tftypes.UnknownValue)},
		{"a group that the state lacks", planned("c", 0), tftypes.NewValue(period, tftypes.UnknownValue)},
	} {
		element, err := schemaResp.Schema.Attributes["groups"].GetType().(types.SetType).ElemType.ValueFromTerraform(ctx, test.group)
		if err != nil {
			t.Fatal(err)
		}
		plan := types.ObjectUnknown(map[string]attr.Type{"period_minutes": types.Int64Type})
		req := planmodifier.ObjectRequest{State: state, PlanValue: plan, Path: path.Root("groups").AtSetValue(element).AtName("period")}
		resp := &planmodifier.ObjectResponse{PlanValue: plan}
		setItemStateModifier{}.PlanModifyObject(ctx, req, resp)
		got, err := resp.PlanValue.ToTerraformValue(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !got.Equal(test.want) {
			t.Errorf("%s: planned period = %v, want %v", test.name, got, test.want)
		}
	}
}

// A group that the configuration sets from an unknown value stays unknown, also when the state item
// that the planned item matches is complete: once the value is known, it can differ from the state.
// The configured item is the one that the planned item matches, not the item at the same index, and a
// group that the configuration omits still takes the state.
func TestSetItemStateModifierKeepsAnUnknownConfigurationUnknown(t *testing.T) {
	ctx := context.Background()
	_, schemaResp, root := setSchema(t)
	group := root.AttributeTypes["groups"].(tftypes.Set).ElementType.(tftypes.Object)
	items := group.AttributeTypes["items"].(tftypes.List)
	item := items.ElementType.(tftypes.Object)
	period := group.AttributeTypes["period"]
	unknownPeriod := tftypes.NewValue(period, tftypes.UnknownValue)
	oneItem := func(labels tftypes.Value) tftypes.Value {
		return tftypes.NewValue(items, []tftypes.Value{tftypes.NewValue(item, map[string]tftypes.Value{
			"key": tftypes.NewValue(tftypes.String, "k"), "labels": labels,
		})})
	}
	groupWith := func(name string, interval tftypes.Value, p tftypes.Value, labels tftypes.Value) tftypes.Value {
		return tftypes.NewValue(group, map[string]tftypes.Value{
			"name": tftypes.NewValue(tftypes.String, name), "interval": interval, "limit": num(0), "period": p, "items": oneItem(labels),
		})
	}
	emptyLabels := tftypes.NewValue(item.AttributeTypes["labels"], map[string]tftypes.Value{})
	statePeriod := func(minutes int64) tftypes.Value {
		return tftypes.NewValue(period, map[string]tftypes.Value{"period_minutes": num(minutes)})
	}
	top := func(groups ...tftypes.Value) tftypes.Value {
		return tftypes.NewValue(root, map[string]tftypes.Value{
			"id": tftypes.NewValue(tftypes.String, "set-1"), "name": tftypes.NewValue(tftypes.String, "s"), "document": tftypes.NewValue(tftypes.String, nil),
			"schedule": tftypes.NewValue(root.AttributeTypes["schedule"], nil),
			"window":   tftypes.NewValue(root.AttributeTypes["window"], nil),
			"groups":   tftypes.NewValue(root.AttributeTypes["groups"], groups),
		})
	}
	state := tfsdk.State{Schema: schemaResp.Schema, Raw: top(
		groupWith("b", num(60), statePeriod(7), emptyLabels), groupWith("a", num(60), statePeriod(3), emptyLabels),
	)}
	nullNumber := tftypes.NewValue(tftypes.Number, nil)
	nullLabels := tftypes.NewValue(item.AttributeTypes["labels"], nil)
	unknownNumber := tftypes.NewValue(tftypes.Number, tftypes.UnknownValue)
	for _, test := range []struct {
		name   string
		config tftypes.Value
		group  string
		want   tftypes.Value
	}{
		// The configuration lists the groups in the other order than the state does.
		{"a with an unknown period", top(
			groupWith("b", nullNumber, statePeriod(7), nullLabels), groupWith("a", nullNumber, unknownPeriod, nullLabels),
		), "a", unknownPeriod},
		{"b next to an unknown period of a", top(
			groupWith("b", nullNumber, tftypes.NewValue(period, nil), nullLabels), groupWith("a", nullNumber, unknownPeriod, nullLabels),
		), "b", statePeriod(7)},
		{"a with an omitted period", top(
			groupWith("a", nullNumber, tftypes.NewValue(period, nil), nullLabels),
		), "a", statePeriod(3)},
		{"a with a period that has an unknown value", top(
			groupWith("a", nullNumber, tftypes.NewValue(period, map[string]tftypes.Value{"period_minutes": tftypes.NewValue(tftypes.Number, tftypes.UnknownValue)}), nullLabels),
		), "a", unknownPeriod},
	} {
		planned := groupWith(test.group, unknownNumber, unknownPeriod, tftypes.NewValue(item.AttributeTypes["labels"], tftypes.UnknownValue))
		element, err := schemaResp.Schema.Attributes["groups"].GetType().(types.SetType).ElemType.ValueFromTerraform(ctx, planned)
		if err != nil {
			t.Fatal(err)
		}
		plan := types.ObjectUnknown(map[string]attr.Type{"period_minutes": types.Int64Type})
		req := planmodifier.ObjectRequest{
			Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: test.config}, State: state, PlanValue: plan,
			Path: path.Root("groups").AtSetValue(element).AtName("period"),
		}
		resp := &planmodifier.ObjectResponse{PlanValue: plan}
		setItemStateModifier{}.PlanModifyObject(ctx, req, resp)
		got, err := resp.PlanValue.ToTerraformValue(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !got.Equal(test.want) {
			t.Errorf("%s: planned period = %v, want %v", test.name, got, test.want)
		}
	}
}

// An optional group is null when the response has none of its fields, and an object when it has one,
// also when the value is 0. The real Terraform tests above cover what Create sends of a group.
func TestOptionalGroupIsNullWhenTheResponseHasNoField(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics
	if got := flattenOutGroupThingPeriod(ctx, path.Root("period"), &group_set_things_service.OutGroupThing{}, &diags); got != nil {
		t.Fatalf("flatten without the field = %+v, want a null group", got)
	}
	minutes := int64(0)
	got := flattenOutGroupThingPeriod(ctx, path.Root("period"), &group_set_things_service.OutGroupThing{PeriodMinutes: &minutes}, &diags)
	if got == nil || got.PeriodMinutes.ValueInt64() != 0 || got.PeriodMinutes.IsNull() {
		t.Fatalf("flatten with minutes 0 = %+v, want a group that holds 0", got)
	}
	if diags.HasError() {
		t.Fatal(diags)
	}
}

// With real Terraform, the same JSON document in another format plans no change, also when the
// items of a set hold a computed value that the configuration leaves out.
func TestEquivalentDocumentPlansNoChangeWithRealTerraform(t *testing.T) {
	factories := map[string]func() (tfprotov6.ProviderServer, error){"test": providerserver.NewProtocol6WithError(setProvider{})}
	config := func(document string) string {
		return "resource \"test_group_set_thing\" \"s\" {\n  document = " + strconv.Quote(document) + "\n  groups = [{ name = \"a\", items = [{ key = \"k\" }] }]\n}\n"
	}
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: factories, Steps: []resource.TestStep{
		{Config: config("{\"a\":1}")},
		{Config: config("{ \"a\": 1 }"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}

func scheduleConfig(body string) string {
	return "resource \"test_group_set_thing\" \"s\" {\n  groups = [{ name = \"g\", items = [{ key = \"k\" }] }]\n" + body + "}\n"
}

// The relative arm, a plain duration, is an object of its own beside the absolute arm: schedule =
// { relative = { duration } }. Each arm reads back as it was sent, the plan after each apply is
// empty (the test framework checks it), a change of arm replaces the other, removing the schedule
// sends none, and both arms are an error at plan.
func TestSingleArmGroupIsTheObjectOfItsArm(t *testing.T) {
	factories := map[string]func() (tfprotov6.ProviderServer, error){"test": providerserver.NewProtocol6WithError(setProvider{})}
	const res = "test_group_set_thing.s"
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: scheduleConfig("  schedule = { relative = { duration = \"15m\" } }\n"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(res, "schedule.relative.duration", "15m"),
					resource.TestCheckNoResourceAttr(res, "schedule.absolute.start"),
					func(*terraform.State) error {
						body := group_set_things_service.Created[len(group_set_things_service.Created)-1]
						if body.Schedule == nil || body.Schedule.RelativeWindow == nil || *body.Schedule.RelativeWindow != "15m" || body.Schedule.AbsoluteWindow != nil {
							return fmt.Errorf("sent schedule = %+v, want only the relative window 15m", body.Schedule)
						}
						return nil
					},
				),
			},
			{
				Config: scheduleConfig("  schedule = { absolute = { start = \"a\", end = \"b\" } }\n"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(res, "schedule.absolute.start", "a"),
					resource.TestCheckResourceAttr(res, "schedule.absolute.end", "b"),
					resource.TestCheckNoResourceAttr(res, "schedule.relative.duration"),
					func(*terraform.State) error {
						body := group_set_things_service.Updated[len(group_set_things_service.Updated)-1]
						if body.Schedule == nil || body.Schedule.AbsoluteWindow == nil || body.Schedule.RelativeWindow != nil {
							return fmt.Errorf("sent schedule = %+v, want only the absolute window", body.Schedule)
						}
						return nil
					},
				),
			},
			{
				Config: scheduleConfig("  schedule = {}\n"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr(res, "schedule.relative.duration"),
					resource.TestCheckNoResourceAttr(res, "schedule.absolute.start"),
				),
			},
			{Config: scheduleConfig(""), Check: resource.TestCheckNoResourceAttr(res, "schedule.relative.duration")},
			{
				Config:      scheduleConfig("  schedule = { absolute = { start = \"a\" }, relative = { duration = \"15m\" } }\n"),
				ExpectError: regexp.MustCompile(`(?s)absolute.*relative|relative.*absolute`),
			},
			{
				Config:      scheduleConfig("  schedule = { relative = {} }\n"),
				ExpectError: regexp.MustCompile(`duration`),
			},
			// An import reads the arm that the API has, so the state matches the configuration.
			{Config: scheduleConfig("  schedule = { relative = { duration = \"15m\" } }\n")},
			{ResourceName: res, ImportState: true, ImportStateVerify: true},
		},
	})
}

// The window is an optional group of the resource. It holds the absolute arm of a oneOf, and the
// group relative stands for the other arm, a plain duration: window = { relative = { duration } }.
// Each arm reads back as it was sent, the plan after each apply is empty (the test framework checks
// it), a change of arm replaces the other, removing the window sends no arm, and two arms, or none
// in a window, are an error at plan. An import reads the arm that the API has.
func TestGroupInAnOptionalTopLevelGroupHoldsOneArm(t *testing.T) {
	factories := map[string]func() (tfprotov6.ProviderServer, error){"test": providerserver.NewProtocol6WithError(setProvider{})}
	const res = "test_group_set_thing.s"
	group_set_things_service.Created, group_set_things_service.Updated = nil, nil
	sent := func(check func(absolute *group_set_things_service.AbsoluteWindow, relative *string) bool, what string) resource.TestCheckFunc {
		return func(*terraform.State) error {
			var absolute *group_set_things_service.AbsoluteWindow
			var relative *string
			if n := len(group_set_things_service.Updated); n > 0 {
				absolute, relative = group_set_things_service.Updated[n-1].AbsoluteWindow, group_set_things_service.Updated[n-1].RelativeWindow
			} else {
				created := group_set_things_service.Created[len(group_set_things_service.Created)-1]
				absolute, relative = created.AbsoluteWindow, created.RelativeWindow
			}
			if !check(absolute, relative) {
				return fmt.Errorf("sent absolute = %+v, relative = %v, want %s", absolute, relative, what)
			}
			return nil
		}
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: scheduleConfig("  window = { relative = { duration = \"15m\" } }\n"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(res, "window.relative.duration", "15m"),
					resource.TestCheckNoResourceAttr(res, "window.absolute.start"),
					sent(func(a *group_set_things_service.AbsoluteWindow, r *string) bool {
						return a == nil && r != nil && *r == "15m"
					}, "only the relative 15m"),
				),
			},
			{
				Config: scheduleConfig("  window = { absolute = { start = \"a\", end = \"b\" } }\n"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(res, "window.absolute.start", "a"),
					resource.TestCheckResourceAttr(res, "window.absolute.end", "b"),
					resource.TestCheckNoResourceAttr(res, "window.relative.duration"),
					sent(func(a *group_set_things_service.AbsoluteWindow, r *string) bool {
						return a != nil && a.Start == "a" && r == nil
					}, "only the absolute window"),
				),
			},
			// An import reads the arm that the API has, so the state matches the configuration.
			{ResourceName: res, ImportState: true, ImportStateVerify: true},
			{
				Config: scheduleConfig(""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr(res, "window.absolute.start"),
					resource.TestCheckNoResourceAttr(res, "window.relative.duration"),
					sent(func(a *group_set_things_service.AbsoluteWindow, r *string) bool { return a == nil && r == nil }, "no arm"),
				),
			},
			{
				Config:      scheduleConfig("  window = { absolute = { start = \"a\" }, relative = { duration = \"15m\" } }\n"),
				ExpectError: regexp.MustCompile(`(?s)absolute.*relative|relative.*absolute`),
			},
			{Config: scheduleConfig("  window = {}\n"), ExpectError: regexp.MustCompile(`(?s)absolute.*relative|relative.*absolute`)},
			{Config: scheduleConfig("  window = { relative = {} }\n"), ExpectError: regexp.MustCompile(`duration`)},
			{Config: scheduleConfig("  window = { relative = { duration = \"1h\" } }\n")},
			{ResourceName: res, ImportState: true, ImportStateVerify: true},
		},
	})
}

// A response that has both arms is an error, and one that has none is a null window.
func TestWindowOfAResponseHasOneArmOrNone(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics
	if got := flattenGroupSetThingWindow(ctx, path.Root("window"), &group_set_things_service.GroupSetThing{}, &diags); got != nil || diags.HasError() {
		t.Fatalf("flatten without an arm = %+v, %v, want a null window", got, diags)
	}
	relative := "15m"
	got := flattenGroupSetThingWindow(ctx, path.Root("window"), &group_set_things_service.GroupSetThing{RelativeWindow: &relative}, &diags)
	if got == nil || got.AbsoluteWindow != nil || got.Relative == nil || got.Relative.RelativeWindow.ValueString() != "15m" || diags.HasError() {
		t.Fatalf("flatten with the relative arm = %+v, %v", got, diags)
	}
	flattenGroupSetThingWindow(ctx, path.Root("window"), &group_set_things_service.GroupSetThing{RelativeWindow: &relative,
		AbsoluteWindow: &group_set_things_service.AbsoluteWindow{Start: "a"}}, &diags)
	if !diags.HasError() {
		t.Fatal("flatten with both arms gave no error")
	}
}

// A oneOf of arms without fields is a string that names the arm that is set. A name line gives the
// released spelling (balanced for balancedMode). The arm reads back as its name, a text that names
// no arm is an error at plan, and an omitted mode sends no arm.
func TestEmptyArmsAreAStringWithRealTerraform(t *testing.T) {
	factories := map[string]func() (tfprotov6.ProviderServer, error){"test": providerserver.NewProtocol6WithError(setProvider{})}
	const res = "test_group_set_thing.s"
	group_set_things_service.Created, group_set_things_service.Updated = nil, nil
	sentMode := func(check func(*group_set_things_service.Mode) bool, what string) resource.TestCheckFunc {
		return func(*terraform.State) error {
			var schedule *group_set_things_service.Schedule
			if n := len(group_set_things_service.Updated); n > 0 {
				schedule = group_set_things_service.Updated[n-1].Schedule
			} else {
				schedule = group_set_things_service.Created[len(group_set_things_service.Created)-1].Schedule
			}
			if schedule == nil || !check(schedule.Mode) {
				return fmt.Errorf("sent schedule = %+v, want %s", schedule, what)
			}
			return nil
		}
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: scheduleConfig("  schedule = { mode = \"fast\" }\n"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(res, "schedule.mode", "fast"),
					sentMode(func(m *group_set_things_service.Mode) bool {
						return m != nil && m.Fast != nil && m.Slow == nil && m.BalancedMode == nil
					}, "only the arm fast"),
				),
			},
			{
				Config: scheduleConfig("  schedule = { mode = \"balanced\" }\n"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(res, "schedule.mode", "balanced"),
					sentMode(func(m *group_set_things_service.Mode) bool { return m != nil && m.BalancedMode != nil && m.Fast == nil }, "only the arm balanced"),
				),
			},
			{
				Config: scheduleConfig("  schedule = { relative = { duration = \"1m\" } }\n"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr(res, "schedule.mode"),
					sentMode(func(m *group_set_things_service.Mode) bool { return m == nil }, "no mode"),
				),
			},
			{Config: scheduleConfig("  schedule = { mode = \"bold\" }\n"), ExpectError: regexp.MustCompile(`mode`)},
			{Config: scheduleConfig("  schedule = { mode = \"slow\" }\n")},
			// An import reads the arm that the API has, so the state matches the configuration.
			{ResourceName: res, ImportState: true, ImportStateVerify: true},
		},
	})
}
