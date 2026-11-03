package parsething

import (
	"context"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"example.com/iac-test-sdk/go/openapi/gen/parse_things_service"
)

func thingSchema(t *testing.T) (*Resource, frameworkresource.SchemaResponse, tftypes.Object) {
	t.Helper()
	r := &Resource{client: &parse_things_service.ParseThingsServiceAPIService{}}
	var schemaResp frameworkresource.SchemaResponse
	r.Schema(context.Background(), frameworkresource.SchemaRequest{}, &schemaResp)
	return r, schemaResp, schemaResp.Schema.Type().TerraformType(context.Background()).(tftypes.Object)
}

// thingValue holds the attributes that the test sets, and null for the others.
func thingValue(root tftypes.Object, set map[string]tftypes.Value) tftypes.Value {
	values := map[string]tftypes.Value{}
	for attr, attrType := range root.AttributeTypes {
		values[attr] = tftypes.NewValue(attrType, nil)
	}
	for attr, v := range set {
		values[attr] = v
	}
	return tftypes.NewValue(root, values)
}

func str(s string) tftypes.Value { return tftypes.NewValue(tftypes.String, s) }

func unknown(typ tftypes.Type) tftypes.Value { return tftypes.NewValue(typ, tftypes.UnknownValue) }

func stringSet(values ...string) tftypes.Value {
	out := []tftypes.Value{}
	for _, v := range values {
		out = append(out, str(v))
	}
	return tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, out)
}

// create runs Create with the plan set; config is the plan with null for the unknown values.
func create(t *testing.T, plan map[string]tftypes.Value) *frameworkresource.CreateResponse {
	t.Helper()
	parse_things_service.Created, parse_things_service.Replaced, parse_things_service.Respond = nil, nil, nil
	r, schemaResp, root := thingSchema(t)
	config := map[string]tftypes.Value{}
	for k, v := range plan {
		if v.IsKnown() {
			config[k] = v
		}
	}
	if _, ok := plan["id"]; !ok {
		plan["id"] = unknown(tftypes.String)
	}
	resp := &frameworkresource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(root, nil)}}
	r.Create(context.Background(), frameworkresource.CreateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: thingValue(root, config)},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: thingValue(root, plan)},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	return resp
}

// An unknown planned active or order is sent as false and 0, as the released resource did. A
// known value is sent as it is.
func TestUnknownScalarsAreSentAsTheirZeroValue(t *testing.T) {
	create(t, map[string]tftypes.Value{
		"name": str("a"), "active": unknown(tftypes.Bool), "order": unknown(tftypes.Number),
		"apps": stringSet(), "severities": stringSet(),
	})
	body := parse_things_service.Created[0]
	if body.Enabled == nil || *body.Enabled || body.Order == nil || *body.Order != 0 {
		t.Fatalf("enabled = %v, order = %v; want false and 0", body.Enabled, body.Order)
	}
	if body.Note != nil {
		t.Fatalf("note = %q; an unknown value without the key is left out", *body.Note)
	}

	create(t, map[string]tftypes.Value{
		"name": str("a"), "active": tftypes.NewValue(tftypes.Bool, true), "order": tftypes.NewValue(tftypes.Number, 7),
		"apps": stringSet(), "severities": stringSet(),
	})
	body = parse_things_service.Created[0]
	if body.Enabled == nil || !*body.Enabled || body.Order == nil || *body.Order != 7 {
		t.Fatalf("enabled = %v, order = %v; want true and 7", body.Enabled, body.Order)
	}
}

// Each value of apps and severities is one matcher with that arm. The read groups the matchers of
// the API by arm, in any order.
func TestSplitArmsSendOneItemPerValueAndReadByArm(t *testing.T) {
	resp := create(t, map[string]tftypes.Value{
		"name": str("a"), "active": tftypes.NewValue(tftypes.Bool, true), "order": tftypes.NewValue(tftypes.Number, 1),
		"apps": stringSet("app-a", "app-b"), "severities": stringSet("debug", "error"),
	})
	var apps, severities []string
	for _, m := range parse_things_service.Created[0].Matchers {
		switch {
		case m.AppName != nil && m.Severity == nil:
			apps = append(apps, *m.AppName.Value)
		case m.Severity != nil && m.AppName == nil:
			severities = append(severities, string(*m.Severity.Value))
		default:
			t.Fatalf("matcher %+v holds no arm or two arms", m)
		}
	}
	sort.Strings(apps)
	sort.Strings(severities)
	if len(apps) != 2 || apps[0] != "app-a" || apps[1] != "app-b" {
		t.Fatalf("apps sent = %v", apps)
	}
	if len(severities) != 2 || severities[0] != "SEVERITY_DEBUG_OR_UNSPECIFIED" || severities[1] != "SEVERITY_ERROR" {
		t.Fatalf("severities sent = %v", severities)
	}

	var state ParseThingModel
	if d := resp.State.Get(context.Background(), &state); d.HasError() {
		t.Fatal(d)
	}
	var gotApps, gotSeverities []string
	state.AppName.ElementsAs(context.Background(), &gotApps, false)
	state.Severity.ElementsAs(context.Background(), &gotSeverities, false)
	sort.Strings(gotApps)
	sort.Strings(gotSeverities)
	if len(gotApps) != 2 || gotApps[1] != "app-b" || len(gotSeverities) != 2 || gotSeverities[0] != "debug" || gotSeverities[1] != "error" {
		t.Fatalf("state apps = %v, severities = %v", gotApps, gotSeverities)
	}
}

// No matcher reads as two empty sets, which the empty default plans.
func TestSplitArmsReadNoItemAsEmptySets(t *testing.T) {
	resp := create(t, map[string]tftypes.Value{
		"name": str("a"), "active": tftypes.NewValue(tftypes.Bool, true), "order": tftypes.NewValue(tftypes.Number, 1),
		"apps": stringSet(), "severities": stringSet(),
	})
	if m := parse_things_service.Created[0].Matchers; m != nil {
		t.Fatalf("matchers sent = %+v, want none", m)
	}
	var state ParseThingModel
	if d := resp.State.Get(context.Background(), &state); d.HasError() {
		t.Fatal(d)
	}
	if state.AppName.IsNull() || len(state.AppName.Elements()) != 0 || state.Severity.IsNull() || len(state.Severity.Elements()) != 0 {
		t.Fatalf("state apps = %v, severities = %v; want empty sets", state.AppName, state.Severity)
	}
}

// A matcher that the API returns with no arm fails the read: the arm sets cannot hold it, and a
// later replace would drop it.
func TestSplitArmsFailTheReadOfAnItemWithoutAnArm(t *testing.T) {
	var diags diag.Diagnostics
	flattenParseThing(context.Background(), path.Empty(), &parse_things_service.ParseThing{Matchers: []parse_things_service.ParseMatcher{{}}}, nil, &diags)
	if !diags.HasError() {
		t.Fatal("reading a matcher without an arm gave no error")
	}
}

// A step that the API returns without params, or with two arms, fails the read: requireOne says
// that a step has exactly one arm.
func TestCarrySiblingsFailTheReadOfAStepWithoutOneArm(t *testing.T) {
	two := &parse_things_service.ParseStep{Params: &parse_things_service.ParseParams{
		KeepParams: &parse_things_service.ParseKeepParams{}, CopyParams: &parse_things_service.ParseCopyParams{},
	}}
	for name, step := range map[string]*parse_things_service.ParseStep{"no arm": {}, "two arms": two} {
		var diags diag.Diagnostics
		flattenParseStep(context.Background(), path.Empty(), step, nil, &diags)
		if !diags.HasError() {
			t.Errorf("%s: the read gave no error", name)
		}
	}
}

// value builds a Terraform value of typ from JSON-like Go values: nil is null, and "?" is unknown.
func value(typ tftypes.Type, v any) tftypes.Value {
	if v == nil {
		return tftypes.NewValue(typ, nil)
	}
	if s, ok := v.(string); ok && s == "?" {
		return tftypes.NewValue(typ, tftypes.UnknownValue)
	}
	switch t := typ.(type) {
	case tftypes.Object:
		in := v.(map[string]any)
		out := map[string]tftypes.Value{}
		for name, at := range t.AttributeTypes {
			out[name] = value(at, in[name])
		}
		return tftypes.NewValue(t, out)
	case tftypes.List:
		var out []tftypes.Value
		for _, e := range v.([]any) {
			out = append(out, value(t.ElementType, e))
		}
		return tftypes.NewValue(t, out)
	case tftypes.Set:
		var out []tftypes.Value
		for _, e := range v.([]any) {
			out = append(out, value(t.ElementType, e))
		}
		return tftypes.NewValue(t, out)
	}
	if n, ok := v.(int); ok {
		return tftypes.NewValue(typ, n)
	}
	return tftypes.NewValue(typ, v)
}

// echo stores what a body sends, as the API does: ids for the thing, the groups, and the steps.
func echo(name *string, enabled *bool, order *int64, note *string, matchers []parse_things_service.ParseMatcher, groups []parse_things_service.CreateParseGroup) parse_things_service.ParseThing {
	id := "thing-1"
	out := parse_things_service.ParseThing{Id: &id, Name: name, Enabled: enabled, Order: order, Note: note, Matchers: matchers}
	for gi, g := range groups {
		gid := "group-" + string(rune('a'+gi))
		group := parse_things_service.ParseGroup{Id: &gid, Enabled: g.Enabled, Order: g.Order}
		for si, s := range g.Steps {
			sid := gid + "-step-" + string(rune('a'+si))
			group.Steps = append(group.Steps, parse_things_service.ParseStep{Id: &sid, Name: s.Name, Enabled: s.Enabled, Order: s.Order, Source: s.Source, Params: s.Params})
		}
		out.Groups = append(out.Groups, group)
	}
	return out
}

// A step is one arm with the fields of the step. Create sends the fields of the step from the
// arm, the position of each group and step, the folded arm for keep_matching = false, the
// negated keep_source, the fixed escaped, and the fixed source of pick. The read shows the
// configured spelling of a case-insensitive enum.
func TestCarrySiblingsSendTheArmWithTheFieldsOfTheStep(t *testing.T) {
	_, _, root := thingSchema(t)
	groups := root.AttributeTypes["groups"]
	plan := map[string]tftypes.Value{
		"name": str("a"), "active": tftypes.NewValue(tftypes.Bool, true), "order": tftypes.NewValue(tftypes.Number, 1),
		"apps": stringSet(), "severities": stringSet(),
		"groups": value(groups, []any{map[string]any{"id": "?", "enabled": true, "order": "?", "steps": []any{
			map[string]any{"keep": map[string]any{"id": "?", "name": "k", "active": true, "order": "?", "source": "text", "rule": "x", "keep_logs": true, "keep_matching": false}},
			map[string]any{"copy": map[string]any{"id": "?", "name": "c", "active": false, "order": "?", "source": "text.a", "target": "text.b", "keep_source": false}},
			map[string]any{"pick": map[string]any{"id": "?", "name": "p", "active": true, "order": "?", "rule": "k", "field": "TEXT"}},
		}}}),
	}
	parse_things_service.Respond = echo
	r, schemaResp, _ := thingSchema(t)
	parse_things_service.Created = nil
	plan["id"] = unknown(tftypes.String)
	resp := &frameworkresource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(root, nil)}}
	r.Create(context.Background(), frameworkresource.CreateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: thingValue(root, plan)},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: thingValue(root, plan)},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}

	body := parse_things_service.Created[0]
	if len(body.Groups) != 1 || *body.Groups[0].Order != 1 || len(body.Groups[0].Steps) != 3 {
		t.Fatalf("groups sent = %+v", body.Groups)
	}
	steps := body.Groups[0].Steps
	for i, s := range steps {
		if s.Order == nil || *s.Order != int64(i+1) {
			t.Fatalf("step %d order = %v, want its position %d", i, s.Order, i+1)
		}
	}
	if steps[0].Params.DropParams == nil || steps[0].Params.KeepParams != nil || *steps[0].Params.DropParams.Rule != "x" || *steps[0].Name != "k" || *steps[0].Source != "text" {
		t.Fatalf("keep_matching = false sent %+v %+v", steps[0], steps[0].Params)
	}
	copyParams := steps[1].Params.CopyParams
	if copyParams == nil || !*copyParams.DeleteSource || !*copyParams.Escaped || *steps[1].Source != "text.a" || *steps[1].Enabled {
		t.Fatalf("copy sent %+v %+v", steps[1], copyParams)
	}
	pick := steps[2].Params.PickParams
	if pick == nil || *steps[2].Source != "text" || *pick.Field != parse_things_service.PARSEPICKFIELD_PICK_FIELD_TEXT {
		t.Fatalf("pick sent %+v %+v", steps[2], pick)
	}

	var state ParseThingModel
	if d := resp.State.Get(context.Background(), &state); d.HasError() {
		t.Fatal(d)
	}
	var stateGroups []ParseGroupModel
	state.Groups.ElementsAs(context.Background(), &stateGroups, false)
	var stateSteps []ParseStepModel
	stateGroups[0].Steps.ElementsAs(context.Background(), &stateSteps, false)
	keep, copied, picked := stateSteps[0].KeepParams, stateSteps[1].CopyParams, stateSteps[2].PickParams
	if keep == nil || keep.KeepMatching.ValueBool() || keep.Name.ValueString() != "k" || keep.Order.ValueInt64() != 1 || keep.Id.ValueString() != "group-a-step-a" {
		t.Fatalf("keep read = %+v", keep)
	}
	if copied == nil || copied.DeleteSource.ValueBool() || copied.Source.ValueString() != "text.a" || copied.Order.ValueInt64() != 2 {
		t.Fatalf("copy read = %+v", copied)
	}
	if picked == nil || picked.Field.ValueString() != "TEXT" || picked.Order.ValueInt64() != 3 {
		t.Fatalf("pick read = %+v, want the configured spelling TEXT", picked)
	}
}
