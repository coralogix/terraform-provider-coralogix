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
	return tftypes.NewValue(root, map[string]tftypes.Value{
		"id":   tftypes.NewValue(tftypes.String, id),
		"name": tftypes.NewValue(tftypes.String, name),
	})
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
