package overwritething

import (
	"context"
	"math/big"
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"example.com/iac-test-sdk/go/openapi/gen/overwrite_things_service"
)

// unknown is a value that Terraform plans.
var unknown = struct{}{}

// value converts a Go value to a Terraform value of typ: a map for an object (a missing attribute is
// null), a slice for a list or set, nil for null, and unknown.
func value(t *testing.T, typ tftypes.Type, v any) tftypes.Value {
	t.Helper()
	if v == nil {
		return tftypes.NewValue(typ, nil)
	}
	if v == unknown {
		return tftypes.NewValue(typ, tftypes.UnknownValue)
	}
	switch typ := typ.(type) {
	case tftypes.Object:
		m := v.(map[string]any)
		attrs := map[string]tftypes.Value{}
		for name, at := range typ.AttributeTypes {
			attrs[name] = value(t, at, m[name])
		}
		return tftypes.NewValue(typ, attrs)
	case tftypes.List:
		var elems []tftypes.Value
		for _, e := range v.([]any) {
			elems = append(elems, value(t, typ.ElementType, e))
		}
		return tftypes.NewValue(typ, elems)
	case tftypes.Set:
		var elems []tftypes.Value
		for _, e := range v.([]any) {
			elems = append(elems, value(t, typ.ElementType, e))
		}
		return tftypes.NewValue(typ, elems)
	}
	if n, ok := v.(int); ok {
		return tftypes.NewValue(typ, big.NewFloat(float64(n)))
	}
	return tftypes.NewValue(typ, v)
}

func setup(t *testing.T) (*Resource, frameworkresource.SchemaResponse, tftypes.Type) {
	t.Helper()
	overwrite_things_service.Stored = map[overwrite_things_service.ThingKind][]overwrite_things_service.Thing{}
	overwrite_things_service.Writes, overwrite_things_service.Kinds = nil, nil
	overwrite_things_service.WriteErr, overwrite_things_service.ReadErr = nil, nil
	r := &Resource{client: &overwrite_things_service.OverwriteThingsServiceAPIService{}}
	var s frameworkresource.SchemaResponse
	r.Schema(context.Background(), frameworkresource.SchemaRequest{}, &s)
	return r, s, s.Schema.Type().TerraformType(context.Background())
}

func noErrors(t *testing.T, diags diag.Diagnostics) {
	t.Helper()
	if diags.HasError() {
		t.Fatal(diags)
	}
}

// thing is the config of one thing, and plan is the same thing as Terraform plans it.
func thing(name string, names ...any) map[string]any {
	return map[string]any{"name": name, "names": names}
}

func planned(name string, enabled bool, names ...any) map[string]any {
	return map[string]any{"name": name, "description": "", "enabled": enabled, "id": unknown, "order": unknown, "names": names, "priority": nil}
}

// things reads the things of the state.
func things(t *testing.T, state tfsdk.State) []ThingModel {
	t.Helper()
	var m ThingListModel
	noErrors(t, state.Get(context.Background(), &m))
	var out []ThingModel
	noErrors(t, m.Things.ElementsAs(context.Background(), &out, false))
	return out
}

// Create sends the things in their wrapper, negates enabled, and joins the rule values. It then
// reads the things with the query of their kind.
func TestCreateWritesAndReadsBack(t *testing.T) {
	r, s, typ := setup(t)
	disabled := planned("second", false, "b")
	disabled["enabled"] = false
	config := value(t, typ, map[string]any{"things": []any{thing("first", "x", "y"), map[string]any{"name": "second", "enabled": false, "names": []any{"b"}}}})
	plan := value(t, typ, map[string]any{"id": "overwrite-things", "things": []any{planned("first", true, "x", "y"), disabled}})
	resp := &frameworkresource.CreateResponse{State: tfsdk.State{Schema: s.Schema}}
	r.Create(context.Background(), frameworkresource.CreateRequest{Config: tfsdk.Config{Schema: s.Schema, Raw: config}, Plan: tfsdk.Plan{Schema: s.Schema, Raw: plan}}, resp)
	noErrors(t, resp.Diagnostics)
	if len(overwrite_things_service.Writes) != 1 {
		t.Fatalf("writes = %d, want 1", len(overwrite_things_service.Writes))
	}
	sent := overwrite_things_service.Writes[0].Things
	if len(sent) != 2 || sent[0].Thing.Name != "first" || sent[0].Thing.Disabled == nil || *sent[0].Thing.Disabled || sent[1].Thing.Disabled == nil || !*sent[1].Thing.Disabled {
		t.Fatalf("written things = %+v, want first enabled and second disabled, in the wrapper", sent)
	}
	if v := sent[0].ARules.Values; v == nil || (*v != "x,y" && *v != "y,x") {
		t.Fatalf("rule values = %v, want x and y joined by a comma", v)
	}
	if !slices.Equal(overwrite_things_service.Kinds, []overwrite_things_service.ThingKind{overwrite_things_service.THINGKIND_THING_KIND_A}) {
		t.Fatalf("Get queries = %v, want one with kind A", overwrite_things_service.Kinds)
	}
	got := things(t, resp.State)
	if len(got) != 2 || got[0].Id.ValueString() != "thing-a" || got[1].Order.ValueInt32() != 2 || got[1].Enabled.ValueBool() || !got[0].Enabled.ValueBool() {
		t.Fatalf("state = %+v, want the ids, the order, and enabled read back", got)
	}
	if n := len(got[0].Values.Elements()); n != 2 {
		t.Fatalf("names = %v, want the two values split", got[0].Values)
	}
	var id string
	noErrors(t, resp.State.GetAttribute(context.Background(), path.Root("id"), &id))
	if id != "overwrite-things" {
		t.Fatalf("id = %q, want the fixed id", id)
	}
}

// A thing that the API returns without its rules reads its values as null.
func TestFlattenWithoutRulesReadsNull(t *testing.T) {
	var diags diag.Diagnostics
	got := flattenThing(context.Background(), path.Empty(), &overwrite_things_service.Thing{Name: "a"}, &diags)
	noErrors(t, diags)
	if !got.Values.IsNull() {
		t.Fatalf("names = %v, want null", got.Values)
	}
}

// A write that succeeds and a Get after it that fails keep the fixed id in the state, so Terraform
// marks the resource as tainted instead of losing the things that the write replaced.
func TestCreateKeepsTheIDWhenTheReadFails(t *testing.T) {
	r, s, typ := setup(t)
	overwrite_things_service.ReadErr = notFound{}
	config := value(t, typ, map[string]any{"things": []any{thing("first", "x")}})
	plan := value(t, typ, map[string]any{"id": "overwrite-things", "things": []any{planned("first", true, "x")}})
	resp := &frameworkresource.CreateResponse{State: tfsdk.State{Schema: s.Schema}}
	r.Create(context.Background(), frameworkresource.CreateRequest{Config: tfsdk.Config{Schema: s.Schema, Raw: config}, Plan: tfsdk.Plan{Schema: s.Schema, Raw: plan}}, resp)
	if !resp.Diagnostics.HasError() || len(overwrite_things_service.Writes) != 1 {
		t.Fatalf("diagnostics = %v, writes = %d; want an error after one write", resp.Diagnostics, len(overwrite_things_service.Writes))
	}
	var id string
	noErrors(t, resp.State.GetAttribute(context.Background(), path.Root("id"), &id))
	if id != "overwrite-things" {
		t.Fatalf("id = %q, want the fixed id kept", id)
	}
}

// The API holds the values of an empty set as "": they read back as an empty set, not as a set of
// one empty string.
func TestSplitEmptyReadsEmpty(t *testing.T) {
	var diags diag.Diagnostics
	empty := ""
	got := flattenThing(context.Background(), path.Empty(), &overwrite_things_service.Thing{Name: "a", ARules: &overwrite_things_service.Rules{Values: &empty}}, &diags)
	noErrors(t, diags)
	if got.Values.IsNull() || len(got.Values.Elements()) != 0 {
		t.Fatalf("names = %v, want an empty set", got.Values)
	}
}

// Read lists only the things of the kind of the resource.
func TestReadUsesTheQuery(t *testing.T) {
	r, s, typ := setup(t)
	name := "other"
	overwrite_things_service.Stored[overwrite_things_service.THINGKIND_THING_KIND_B] = []overwrite_things_service.Thing{{Name: name}}
	state := value(t, typ, map[string]any{"id": "overwrite-things", "things": []any{}})
	resp := &frameworkresource.ReadResponse{State: tfsdk.State{Schema: s.Schema, Raw: state}}
	r.Read(context.Background(), frameworkresource.ReadRequest{State: tfsdk.State{Schema: s.Schema, Raw: state}}, resp)
	noErrors(t, resp.Diagnostics)
	if got := things(t, resp.State); len(got) != 0 {
		t.Fatalf("state = %+v, want no things of kind B", got)
	}
}

// Delete sends the write with no things.
func TestDeleteWritesNoThings(t *testing.T) {
	r, s, typ := setup(t)
	state := value(t, typ, map[string]any{"id": "overwrite-things", "things": []any{}})
	resp := &frameworkresource.DeleteResponse{State: tfsdk.State{Schema: s.Schema, Raw: state}}
	r.Delete(context.Background(), frameworkresource.DeleteRequest{State: tfsdk.State{Schema: s.Schema, Raw: state}}, resp)
	noErrors(t, resp.Diagnostics)
	if len(overwrite_things_service.Writes) != 1 || overwrite_things_service.Writes[0].Things != nil {
		t.Fatalf("writes = %+v, want one write without things", overwrite_things_service.Writes)
	}
}

// notFound is a 404 whose body says that a resource is missing.
type notFound struct{}

func (notFound) Error() string   { return "404 Not Found" }
func (notFound) StatusCode() int { return 404 }
func (notFound) Body() []byte    { return []byte("Not Found: thing kind") }

// The write never answers 404 for things that are gone already, so a 404 of Delete is an error.
func TestDelete404IsAnError(t *testing.T) {
	r, s, typ := setup(t)
	overwrite_things_service.WriteErr = notFound{}
	state := value(t, typ, map[string]any{"id": "overwrite-things", "things": []any{}})
	resp := &frameworkresource.DeleteResponse{State: tfsdk.State{Schema: s.Schema, Raw: state}}
	r.Delete(context.Background(), frameworkresource.DeleteRequest{State: tfsdk.State{Schema: s.Schema, Raw: state}}, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("Delete accepted a 404, want an error")
	}
}

// The data source reads the things of the kind of the resource, with the same query.
func TestDataSourceReadsTheKind(t *testing.T) {
	ctx := context.Background()
	setup(t)
	id, order, enabled := "stored", int32(1), true
	overwrite_things_service.Stored[overwrite_things_service.THINGKIND_THING_KIND_A] = []overwrite_things_service.Thing{{Id: &id, Order: &order, Name: "a", Enabled: &enabled}}
	overwrite_things_service.Stored[overwrite_things_service.THINGKIND_THING_KIND_B] = []overwrite_things_service.Thing{{Name: "b"}}
	d := &DataSource{resource: Resource{client: &overwrite_things_service.OverwriteThingsServiceAPIService{}}}
	var s datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &s)
	config := value(t, s.Schema.Type().TerraformType(ctx), map[string]any{})
	resp := &datasource.ReadResponse{State: tfsdk.State{Schema: s.Schema, Raw: config}}
	d.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: s.Schema, Raw: config}}, resp)
	noErrors(t, resp.Diagnostics)
	if !slices.Equal(overwrite_things_service.Kinds, []overwrite_things_service.ThingKind{overwrite_things_service.THINGKIND_THING_KIND_A}) {
		t.Fatalf("Get queries = %v, want one with kind A", overwrite_things_service.Kinds)
	}
	if got := things(t, resp.State); len(got) != 1 || got[0].Name.ValueString() != "a" {
		t.Fatalf("data source things = %+v, want only the thing of kind A", got)
	}
}

// The description conflicts with the rule values and has at most 50 characters.
func TestDescriptionValidators(t *testing.T) {
	_, s, _ := setup(t)
	things := s.Schema.Attributes["things"].(schema.ListNestedAttribute)
	validators := things.NestedObject.Attributes["description"].(schema.StringAttribute).Validators
	var got []string
	for _, v := range validators {
		got = append(got, v.Description(context.Background()))
	}
	if len(got) != 2 {
		t.Fatalf("description validators = %v, want the length and the conflict", got)
	}
}

// An element with the separator would read back as two elements, and a set of one empty element
// would read back empty, so a validator rejects both.
func TestSplitRejectsTheSeparator(t *testing.T) {
	_, s, _ := setup(t)
	things := s.Schema.Attributes["things"].(schema.ListNestedAttribute)
	names := things.NestedObject.Attributes["names"].(schema.SetAttribute)
	for _, v := range names.Validators {
		if strings.Contains(v.Description(context.Background()), "must not be empty or contain") {
			return
		}
	}
	t.Fatal("names has no validator against the separator")
}

// The API returns the zero priority for a thing without one, and it reads back as null.
func TestZeroPriorityReadsNull(t *testing.T) {
	var diags diag.Diagnostics
	zero := overwrite_things_service.THINGPRIORITY_THING_PRIORITY_UNSPECIFIED
	got := flattenThing(context.Background(), path.Empty(), &overwrite_things_service.Thing{Name: "a", Priority: &zero}, &diags)
	noErrors(t, diags)
	if !got.Priority.IsNull() {
		t.Fatalf("priority = %v, want null", got.Priority)
	}
	high := overwrite_things_service.THINGPRIORITY_THING_PRIORITY_HIGH
	got = flattenThing(context.Background(), path.Empty(), &overwrite_things_service.Thing{Name: "a", Priority: &high}, &diags)
	noErrors(t, diags)
	if got.Priority.ValueString() != "high" {
		t.Fatalf("priority = %v, want high", got.Priority)
	}
}
