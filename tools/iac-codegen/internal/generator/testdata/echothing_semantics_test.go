package echothing

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"example.com/iac-test-sdk/go/openapi/gen/echo_things_service"
)

func ptr[T any](v T) *T { return &v }

func thingSchema(t *testing.T) (frameworkresource.SchemaResponse, tftypes.Type) {
	t.Helper()
	ctx := context.Background()
	var schemaResp frameworkresource.SchemaResponse
	(&Resource{}).Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResp)
	return schemaResp, schemaResp.Schema.Type().TerraformType(ctx)
}

func thingValue(root tftypes.Type, id, name string) tftypes.Value {
	return thingWith(root, map[string]tftypes.Value{
		"id":   tftypes.NewValue(tftypes.String, id),
		"name": tftypes.NewValue(tftypes.String, name),
	})
}

// thingWith is a resource value with the given attributes. Every other attribute is null.
func thingWith(root tftypes.Type, set map[string]tftypes.Value) tftypes.Value {
	attrs := map[string]tftypes.Value{}
	for name, typ := range root.(tftypes.Object).AttributeTypes {
		attrs[name] = tftypes.NewValue(typ, nil)
	}
	for name, v := range set {
		attrs[name] = v
	}
	return tftypes.NewValue(root, attrs)
}

// quota is a quota value with a limit and an exceeded flag.
func quota(root tftypes.Type, limit int64, exceeded bool) tftypes.Value {
	typ := root.(tftypes.Object).AttributeTypes["quota"]
	return tftypes.NewValue(typ, map[string]tftypes.Value{
		"limit":        tftypes.NewValue(tftypes.Number, limit),
		"has_exceeded": tftypes.NewValue(tftypes.Bool, exceeded),
	})
}

func createThing(t *testing.T, plan tftypes.Value) echo_things_service.CreateEchoThingRequest {
	t.Helper()
	ctx := context.Background()
	schemaResp, _ := thingSchema(t)
	echo_things_service.Created = nil
	echo_things_service.Answer = echo_things_service.EchoThing{Id: "thing-1", Name: "name", Quota: &echo_things_service.Quota{Limit: 30000}}
	r := &Resource{client: &echo_things_service.EchoThingsServiceAPIService{}}
	resp := &frameworkresource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	r.Create(ctx, frameworkresource.CreateRequest{Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan}, Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: plan}}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if len(echo_things_service.Created) != 1 {
		t.Fatalf("Create calls = %d, want 1", len(echo_things_service.Created))
	}
	return echo_things_service.Created[0]
}

// Create sends quota.limit as the top-level Create field limitValue.
func TestCreateSendsTheFoldedLimitFlat(t *testing.T) {
	_, root := thingSchema(t)
	sent := createThing(t, thingWith(root, map[string]tftypes.Value{
		"id":    tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"name":  tftypes.NewValue(tftypes.String, "name"),
		"quota": quota(root, 5, false),
	}))
	if sent.LimitValue == nil || *sent.LimitValue != 5 {
		t.Fatalf("Create LimitValue = %v, want 5", sent.LimitValue)
	}
}

// Create sends the renamed attributes under their API names, and the tags set as a list.
func TestCreateSendsRenamedAttributesUnderTheirAPINames(t *testing.T) {
	_, root := thingSchema(t)
	entries := root.(tftypes.Object).AttributeTypes["entries"].(tftypes.List)
	entry := entries.ElementType.(tftypes.Object)
	item := map[string]tftypes.Value{}
	for name, typ := range entry.AttributeTypes {
		item[name] = tftypes.NewValue(typ, nil)
	}
	item["key"] = tftypes.NewValue(tftypes.String, "k")
	item["tags"] = tftypes.NewValue(entry.AttributeTypes["tags"], []tftypes.Value{tftypes.NewValue(tftypes.String, "a")})
	item["enable"] = tftypes.NewValue(tftypes.Bool, true)
	sent := createThing(t, thingWith(root, map[string]tftypes.Value{
		"id":      tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"name":    tftypes.NewValue(tftypes.String, "name"),
		"entries": tftypes.NewValue(entries, []tftypes.Value{tftypes.NewValue(entry, item)}),
	}))
	if len(sent.Items) != 1 || sent.Items[0].Key != "k" || len(sent.Items[0].Values) != 1 || sent.Items[0].Values[0] != "a" ||
		sent.Items[0].Enabled == nil || !*sent.Items[0].Enabled {
		t.Fatalf("Create items = %+v, want one item k with values [a] and enabled", sent.Items)
	}
	if _, ok := entry.AttributeTypes["tags"].(tftypes.Set); !ok {
		t.Fatalf("tags is %v, want a set", entry.AttributeTypes["tags"])
	}
}

// Without quota, Create sends no limit, so the server applies its own.
func TestCreateWithoutTheObjectSendsNoLimit(t *testing.T) {
	_, root := thingSchema(t)
	sent := createThing(t, thingWith(root, map[string]tftypes.Value{
		"id":   tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"name": tftypes.NewValue(tftypes.String, "name"),
	}))
	if sent.LimitValue != nil {
		t.Fatalf("Create LimitValue = %d, want none", *sent.LimitValue)
	}
}

// Update sends quota itself, with the limit and without the read-only exceeded flag.
func TestUpdateSendsTheObjectWithoutTheServerFlag(t *testing.T) {
	ctx := context.Background()
	schemaResp, root := thingSchema(t)
	echo_things_service.Replaced = nil
	echo_things_service.Answer = echo_things_service.EchoThing{Id: "thing-1", Name: "name", Quota: &echo_things_service.Quota{Limit: 7, Exceeded: true}}
	r := &Resource{client: &echo_things_service.EchoThingsServiceAPIService{}}
	value := thingWith(root, map[string]tftypes.Value{
		"id":    tftypes.NewValue(tftypes.String, "thing-1"),
		"name":  tftypes.NewValue(tftypes.String, "name"),
		"quota": quota(root, 7, true),
	})
	plan := tfsdk.Plan{Schema: schemaResp.Schema, Raw: value}
	state := tfsdk.State{Schema: schemaResp.Schema, Raw: thingWith(root, map[string]tftypes.Value{
		"id":    tftypes.NewValue(tftypes.String, "thing-1"),
		"name":  tftypes.NewValue(tftypes.String, "old"),
		"quota": quota(root, 7, true),
	})}
	resp := &frameworkresource.UpdateResponse{State: state}
	r.Update(ctx, frameworkresource.UpdateRequest{Plan: plan, State: state, Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: value}}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if len(echo_things_service.Replaced) != 1 {
		t.Fatalf("Replace calls = %d, want 1", len(echo_things_service.Replaced))
	}
	sent := echo_things_service.Replaced[0]
	if sent.Quota == nil || sent.Quota.Limit != 7 || sent.Quota.Exceeded {
		t.Fatalf("Replace quota = %+v, want limit 7 and no exceeded flag", sent.Quota)
	}
}

// Update sends the id that the server set, from state, in the Replace body. It does not send
// the server time or the internal flag that the resource skips.
func TestUpdateSendsTheServerIDInTheBody(t *testing.T) {
	ctx := context.Background()
	schemaResp, root := thingSchema(t)
	echo_things_service.Replaced = nil
	echo_things_service.Answer = echo_things_service.EchoThing{Id: "thing-1", Name: "renamed", CreateTime: ptr("2026-01-01T00:00:00Z"), IsInternal: ptr(false)}

	r := &Resource{client: &echo_things_service.EchoThingsServiceAPIService{}}
	plan := tfsdk.Plan{Schema: schemaResp.Schema, Raw: thingValue(root, "thing-1", "renamed")}
	state := tfsdk.State{Schema: schemaResp.Schema, Raw: thingValue(root, "thing-1", "old")}
	resp := &frameworkresource.UpdateResponse{State: state}
	r.Update(ctx, frameworkresource.UpdateRequest{Plan: plan, State: state, Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: plan.Raw}}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if len(echo_things_service.Replaced) != 1 {
		t.Fatalf("Replace calls = %d, want 1", len(echo_things_service.Replaced))
	}
	sent := echo_things_service.Replaced[0]
	if sent.Id != "thing-1" || sent.Name != "renamed" {
		t.Fatalf("Replace body = %+v, want id thing-1 and name renamed", sent)
	}
	if sent.CreateTime != nil || sent.IsInternal != nil {
		t.Fatalf("Replace sent a skipped field: createTime %v, isInternal %v", sent.CreateTime, sent.IsInternal)
	}
}

// Read takes the resource from a response field that holds it as a value.
func TestReadTakesTheResourceFromAValueField(t *testing.T) {
	ctx := context.Background()
	schemaResp, root := thingSchema(t)
	echo_things_service.Answer = echo_things_service.EchoThing{Id: "thing-1", Name: "name"}

	r := &Resource{client: &echo_things_service.EchoThingsServiceAPIService{}}
	state := tfsdk.State{Schema: schemaResp.Schema, Raw: thingValue(root, "thing-1", "old")}
	resp := &frameworkresource.ReadResponse{State: state}
	r.Read(ctx, frameworkresource.ReadRequest{State: state}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	var name string
	resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("name"), &name)...)
	if name != "name" {
		t.Fatalf("name = %q, want name", name)
	}
}
