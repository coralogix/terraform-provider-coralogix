package groupsetthing

import (
	"context"
	"math/big"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

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

// Inside a set item the server default reads the plan, which Terraform proposes from the prior item
// that it paired with the configured one. The framework gives it the state of the item at the same
// index, here another group with interval 120.
func TestServerDefaultInASetReadsThePlan(t *testing.T) {
	ctx := context.Background()
	m := serverDefaultModifier{value: types.Int64Value(60), fromPlan: true}
	for _, test := range []struct {
		name        string
		plan        types.Int64
		wantUnknown bool
	}{
		{name: "the paired item has the default", plan: types.Int64Value(60)},
		{name: "the paired item has another value that the config removes", plan: types.Int64Value(90), wantUnknown: true},
	} {
		_, schemaResp, root := setSchema(t)
		config := thingWithGroups(root)
		req := planmodifier.Int64Request{
			Path:        path.Root("groups").AtSetValue(types.StringValue("g")).AtName("interval"),
			Config:      tfsdk.Config{Schema: schemaResp.Schema, Raw: config},
			ConfigValue: types.Int64Null(),
			StateValue:  types.Int64Value(120),
			PlanValue:   test.plan,
			Plan:        tfsdk.Plan{Schema: schemaResp.Schema, Raw: config},
		}
		resp := &planmodifier.Int64Response{PlanValue: test.plan}
		m.PlanModifyInt64(ctx, req, resp)
		if resp.PlanValue.IsUnknown() != test.wantUnknown {
			t.Errorf("%s: plan = %v, want unknown %t", test.name, resp.PlanValue, test.wantUnknown)
		}
	}
}

// thingWithGroups is a configuration that sets the groups.
func thingWithGroups(root tftypes.Object) tftypes.Value {
	return plannedSet(root)
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
