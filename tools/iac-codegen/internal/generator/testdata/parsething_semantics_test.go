package parsething

import (
	"context"
	"sort"
	"testing"

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

func strings(values ...string) tftypes.Value {
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
		"apps": strings(), "severities": strings(),
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
		"apps": strings(), "severities": strings(),
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
		"apps": strings("app-a", "app-b"), "severities": strings("debug", "error"),
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
		"apps": strings(), "severities": strings(),
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
