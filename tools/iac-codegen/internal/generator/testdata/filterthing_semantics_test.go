package filterthing

import (
	"context"
	"testing"

	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"example.com/iac-test-sdk/go/openapi/gen/filter_things_service"
)

func thingSchema(t *testing.T) (*Resource, frameworkresource.SchemaResponse, tftypes.Object) {
	t.Helper()
	r := &Resource{client: &filter_things_service.FilterThingsServiceAPIService{}}
	var schemaResp frameworkresource.SchemaResponse
	r.Schema(context.Background(), frameworkresource.SchemaRequest{}, &schemaResp)
	return r, schemaResp, schemaResp.Schema.Type().TerraformType(context.Background()).(tftypes.Object)
}

func objectValue(typ tftypes.Type, set map[string]tftypes.Value) tftypes.Value {
	object := typ.(tftypes.Object)
	values := map[string]tftypes.Value{}
	for attr, attrType := range object.AttributeTypes {
		values[attr] = tftypes.NewValue(attrType, nil)
	}
	for attr, v := range set {
		values[attr] = v
	}
	return tftypes.NewValue(object, values)
}

func str(s string) tftypes.Value { return tftypes.NewValue(tftypes.String, s) }

// filters is a list with one item for each set of arms.
func filters(root tftypes.Object, items ...map[string]tftypes.Value) tftypes.Value {
	list := root.AttributeTypes["filters"].(tftypes.List)
	values := []tftypes.Value{}
	for _, item := range items {
		values = append(values, objectValue(list.ElementType, item))
	}
	return tftypes.NewValue(list, values)
}

func create(t *testing.T, items ...map[string]tftypes.Value) (*frameworkresource.CreateResponse, tftypes.Object) {
	t.Helper()
	filter_things_service.Created = nil
	r, schemaResp, root := thingSchema(t)
	plan := objectValue(root, map[string]tftypes.Value{
		"id":      tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"name":    str("a"),
		"filters": filters(root, items...),
	})
	config := objectValue(root, map[string]tftypes.Value{"name": str("a"), "filters": filters(root, items...)})
	resp := &frameworkresource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(root, nil)}}
	r.Create(context.Background(), frameworkresource.CreateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: config},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	return resp, root
}

// Each item sends the arm that it sets and no other. A string arm, an object with a nested object,
// and a bool arm for an empty object all work as list items.
func TestListOfOneOfItemsSendOneArmEach(t *testing.T) {
	logsType := thingRoot(t).AttributeTypes["filters"].(tftypes.List).ElementType.(tftypes.Object).AttributeTypes["logs"]
	logsValue := objectValue(logsType, map[string]tftypes.Value{
		"field":    str("severity"),
		"operator": objectValue(logsType.(tftypes.Object).AttributeTypes["operator"], map[string]tftypes.Value{"value": str("equals")}),
	})
	create(t,
		map[string]tftypes.Value{"logs": logsValue},
		map[string]tftypes.Value{"label": str("team")},
		map[string]tftypes.Value{"everything": tftypes.NewValue(tftypes.Bool, true)},
	)
	got := filter_things_service.Created[0].Filters
	if len(got) != 3 {
		t.Fatalf("sent %d filters, want 3", len(got))
	}
	if got[0].Logs == nil || *got[0].Logs.Field != "severity" || *got[0].Logs.Operator.Value != "equals" ||
		got[0].Metrics != nil || got[0].Label != nil || got[0].Everything != nil {
		t.Fatalf("filter 0 = %+v, want only logs", got[0])
	}
	if got[1].Label == nil || *got[1].Label != "team" || got[1].Logs != nil || got[1].Everything != nil {
		t.Fatalf("filter 1 = %+v, want only label", got[1])
	}
	if got[2].Everything == nil || got[2].Logs != nil || got[2].Label != nil {
		t.Fatalf("filter 2 = %+v, want only everything", got[2])
	}
}

// The read puts each API item in the arm that it has, in the order of the API. The other arms
// of the item are null.
func TestListOfOneOfItemsReadByArm(t *testing.T) {
	items := []map[string]tftypes.Value{
		{"label": str("team")},
		{"everything": tftypes.NewValue(tftypes.Bool, true)},
	}
	resp, root := create(t, items...)
	var values map[string]tftypes.Value
	if err := resp.State.Raw.As(&values); err != nil {
		t.Fatal(err)
	}
	if want := filters(root, items...); !values["filters"].Equal(want) {
		t.Fatalf("state filters = %v, want %v", values["filters"], want)
	}
}

func thingRoot(t *testing.T) tftypes.Object {
	t.Helper()
	_, _, root := thingSchema(t)
	return root
}
