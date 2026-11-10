package typedthing

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"example.com/iac-test-sdk/go/openapi/gen/typed_things_service"
)

func thingSchema(t *testing.T) (*Resource, frameworkresource.SchemaResponse, tftypes.Object) {
	t.Helper()
	r := &Resource{client: &typed_things_service.TypedThingsServiceAPIService{}}
	var schemaResp frameworkresource.SchemaResponse
	r.Schema(context.Background(), frameworkresource.SchemaRequest{}, &schemaResp)
	return r, schemaResp, schemaResp.Schema.Type().TerraformType(context.Background()).(tftypes.Object)
}

// objectValue holds the attributes that the test sets, and null for the others.
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

func num(n float64) tftypes.Value { return tftypes.NewValue(tftypes.Number, n) }

func aggregationType(root tftypes.Object) tftypes.Type { return root.AttributeTypes["aggregation"] }

func fieldsValue(root tftypes.Object, items ...map[string]tftypes.Value) tftypes.Value {
	list := root.AttributeTypes["fields"].(tftypes.List)
	values := []tftypes.Value{}
	for _, item := range items {
		values = append(values, objectValue(list.ElementType, item))
	}
	return tftypes.NewValue(list, values)
}

func create(t *testing.T, aggregation map[string]tftypes.Value, fields ...map[string]tftypes.Value) (*frameworkresource.CreateResponse, tftypes.Object, tftypes.Value) {
	t.Helper()
	typed_things_service.Created = nil
	r, schemaResp, root := thingSchema(t)
	set := map[string]tftypes.Value{"name": str("a"), "fields": fieldsValue(root, fields...)}
	if aggregation != nil {
		set["aggregation"] = objectValue(aggregationType(root), aggregation)
	}
	config := objectValue(root, set)
	plan := objectValue(root, set)
	plan = withID(root, plan)
	resp := &frameworkresource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(root, nil)}}
	r.Create(context.Background(), frameworkresource.CreateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: config},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	return resp, root, config
}

func withID(root tftypes.Object, v tftypes.Value) tftypes.Value {
	var values map[string]tftypes.Value
	if err := v.As(&values); err != nil {
		panic(err)
	}
	values["id"] = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	return tftypes.NewValue(root, values)
}

// Each type sends its arm and no other. An arm with no fields is sent as an empty object, and two
// arms that share the field attribute both read it.
func TestTypeSendsOneArm(t *testing.T) {
	create(t, map[string]tftypes.Value{
		"type": str("percentile"), "field": str("duration"), "percent": num(95),
		"bucket": objectValue(aggregationType(thingRoot(t)).(tftypes.Object).AttributeTypes["bucket"], map[string]tftypes.Value{"size": str("1m")}),
	})
	got := typed_things_service.Created[0].Aggregation
	if got == nil || got.Percentile == nil || got.Average != nil || got.Count != nil {
		t.Fatalf("aggregation = %+v, want only percentile", got)
	}
	if *got.Percentile.Field != "duration" || *got.Percentile.Percent != 95 || *got.Percentile.Bucket.Size != "1m" {
		t.Fatalf("percentile = %+v", got.Percentile)
	}

	create(t, map[string]tftypes.Value{"type": str("avg"), "field": str("duration")})
	got = typed_things_service.Created[0].Aggregation
	if got.Average == nil || *got.Average.Field != "duration" || got.Percentile != nil {
		t.Fatalf("aggregation = %+v, want only average", got)
	}

	create(t, map[string]tftypes.Value{"type": str("count")})
	got = typed_things_service.Created[0].Aggregation
	if got.Count == nil || got.Average != nil || got.Percentile != nil {
		t.Fatalf("aggregation = %+v, want only count", got)
	}
}

// The scalar arms of a list item share the value attribute. The enum arm sends its API value.
func TestScalarArmsShareTheValueAttribute(t *testing.T) {
	create(t, nil,
		map[string]tftypes.Value{"type": str("metadata"), "value": str("service_name")},
		map[string]tftypes.Value{"type": str("tag"), "value": str("env")},
		map[string]tftypes.Value{"type": str("process_tag"), "value": str("pid")},
	)
	got := typed_things_service.Created[0].Fields
	if len(got) != 3 {
		t.Fatalf("sent %d fields, want 3", len(got))
	}
	if got[0].MetadataField == nil || *got[0].MetadataField != typed_things_service.TYPEDMETADATAFIELD_METADATA_FIELD_SERVICE_NAME || got[0].TagField != nil {
		t.Fatalf("field 0 = %+v, want only the enum", got[0])
	}
	if got[1].TagField == nil || *got[1].TagField != "env" || got[1].ProcessTagField != nil {
		t.Fatalf("field 1 = %+v, want only the tag", got[1])
	}
	if got[2].ProcessTagField == nil || *got[2].ProcessTagField != "pid" {
		t.Fatalf("field 2 = %+v, want only the process tag", got[2])
	}
}

// The read gives the type of the arm that the API has, and null for the fields of other arms.
func TestReadGivesTheTypeAndTheMergedFields(t *testing.T) {
	aggregation := map[string]tftypes.Value{"type": str("avg"), "field": str("duration")}
	fields := []map[string]tftypes.Value{
		{"type": str("metadata"), "value": str("application_name")},
		{"type": str("tag"), "value": str("env")},
	}
	resp, root, config := create(t, aggregation, fields...)
	var want, got map[string]tftypes.Value
	if err := config.As(&want); err != nil {
		t.Fatal(err)
	}
	if err := resp.State.Raw.As(&got); err != nil {
		t.Fatal(err)
	}
	if !got["aggregation"].Equal(want["aggregation"]) {
		t.Fatalf("state aggregation = %v, want %v", got["aggregation"], want["aggregation"])
	}
	if !got["fields"].Equal(fieldsValue(root, fields...)) {
		t.Fatalf("state fields = %v", got["fields"])
	}
}

// The check on the type attribute rejects a field that the chosen arm does not have, and a value
// that the enum arm does not accept. It accepts a tag value that is no enum value.
func TestTypeValidatorChecksTheArm(t *testing.T) {
	_, schemaResp, root := thingSchema(t)
	validate := func(attribute string, item map[string]tftypes.Value, aggregation map[string]tftypes.Value) bool {
		t.Helper()
		set := map[string]tftypes.Value{"name": str("a")}
		var typeAttr schema.StringAttribute
		var at path.Path
		if attribute == "aggregation" {
			set["aggregation"] = objectValue(aggregationType(root), aggregation)
			typeAttr = schemaResp.Schema.Attributes["aggregation"].(schema.SingleNestedAttribute).Attributes["type"].(schema.StringAttribute)
			at = path.Root("aggregation").AtName("type")
			item = aggregation
		} else {
			set["fields"] = fieldsValue(root, item)
			typeAttr = schemaResp.Schema.Attributes["fields"].(schema.ListNestedAttribute).NestedObject.Attributes["type"].(schema.StringAttribute)
			at = path.Root("fields").AtListIndex(0).AtName("type")
		}
		config := tfsdk.Config{Schema: schemaResp.Schema, Raw: objectValue(root, set)}
		resp := &validator.StringResponse{}
		for _, v := range typeAttr.Validators {
			v.ValidateString(context.Background(), validator.StringRequest{Path: at, PathExpression: at.Expression(), Config: config, ConfigValue: types.StringValue(*stringOf(item["type"]))}, resp)
		}
		return !resp.Diagnostics.HasError()
	}
	if !validate("aggregation", nil, map[string]tftypes.Value{"type": str("avg"), "field": str("f")}) {
		t.Fatal("avg with a field is valid")
	}
	if validate("aggregation", nil, map[string]tftypes.Value{"type": str("count"), "field": str("f")}) {
		t.Fatal("count with a field must be rejected")
	}
	if validate("aggregation", nil, map[string]tftypes.Value{"type": str("avg"), "percent": num(1)}) {
		t.Fatal("avg with a percent must be rejected")
	}
	if validate("aggregation", nil, map[string]tftypes.Value{"type": str("sum")}) {
		t.Fatal("an unknown type must be rejected")
	}
	if !validate("fields", map[string]tftypes.Value{"type": str("tag"), "value": str("anything")}, nil) {
		t.Fatal("a tag value is free text")
	}
	if validate("fields", map[string]tftypes.Value{"type": str("metadata"), "value": str("anything")}, nil) {
		t.Fatal("a metadata value must be an enum value")
	}
	if !validate("fields", map[string]tftypes.Value{"type": str("metadata"), "value": str("service_name")}, nil) {
		t.Fatal("service_name is a metadata value")
	}
}

func stringOf(v tftypes.Value) *string {
	var s string
	if err := v.As(&s); err != nil {
		panic(err)
	}
	return &s
}

func thingRoot(t *testing.T) tftypes.Object {
	t.Helper()
	_, _, root := thingSchema(t)
	return root
}
