package typedthing

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
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
	return createWith(t, nil, aggregation, fields...)
}

// createWith runs Create with extra attributes of the thing in the plan, for example the selection.
func createWith(t *testing.T, extra map[string]tftypes.Value, aggregation map[string]tftypes.Value, fields ...map[string]tftypes.Value) (*frameworkresource.CreateResponse, tftypes.Object, tftypes.Value) {
	t.Helper()
	typed_things_service.Created = nil
	r, schemaResp, root := thingSchema(t)
	set := map[string]tftypes.Value{"name": str("a"), "fields": fieldsValue(root, fields...)}
	for k, v := range extra {
		set[k] = v
	}
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

func stringList(values ...string) tftypes.Value {
	items := []tftypes.Value{}
	for _, v := range values {
		items = append(items, str(v))
	}
	return tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, items)
}

// selection makes the selection object of a plan. A nil type is the type that the plan modifier
// has not filled in yet: unknown, as it is when Create runs.
func selection(root tftypes.Object, kind *string, values tftypes.Value) tftypes.Value {
	typeValue := tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	if kind != nil {
		typeValue = str(*kind)
	}
	return objectValue(root.AttributeTypes["selection"], map[string]tftypes.Value{"selection_type": typeValue, "selected_values": values})
}

func ptr(s string) *string { return &s }

// A type that the configuration leaves out is picked from the fields that are set: values select
// a list, and no values select all. A type that is set stays, also a list with no values.
func TestOmittedTypeIsInferredFromTheFields(t *testing.T) {
	root := thingRoot(t)
	cases := []struct {
		name   string
		kind   *string
		values tftypes.Value
		all    bool
		list   []string
		state  string
	}{
		{"values select a list", nil, stringList("a", "b"), false, []string{"a", "b"}, "list"},
		{"no values select all", nil, stringList(), true, nil, "all"},
		{"an empty list on purpose", ptr("list"), stringList(), false, []string{}, "list"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp, _, _ := createWith(t, map[string]tftypes.Value{"selection": selection(root, c.kind, c.values)}, nil)
			got := typed_things_service.Created[0].Selection
			if got == nil || (got.All != nil) != c.all || (got.List != nil) == c.all {
				t.Fatalf("selection = %+v, want all = %v", got, c.all)
			}
			if !c.all && len(got.List.Values) != len(c.list) {
				t.Fatalf("values = %v, want %v", got.List.Values, c.list)
			}
			var state map[string]tftypes.Value
			if err := resp.State.Raw.As(&state); err != nil {
				t.Fatal(err)
			}
			want := selection(root, ptr(c.state), stringList(c.list...))
			if !state["selection"].Equal(want) {
				t.Fatalf("state selection = %v, want %v", state["selection"], want)
			}
		})
	}
}

// The plan modifier fills the type from the planned values, and keeps a configured type.
func TestPlanModifierInfersTheType(t *testing.T) {
	_, schemaResp, root := thingSchema(t)
	attr := schemaResp.Schema.Attributes["selection"].(schema.SingleNestedAttribute).Attributes["selection_type"].(schema.StringAttribute)
	if len(attr.PlanModifiers) != 1 || !attr.Computed || !attr.Optional {
		t.Fatalf("selection_type = %+v, want optional and computed with one plan modifier", attr)
	}
	plan := func(kind *string, values tftypes.Value) types.String {
		set := map[string]tftypes.Value{"name": str("a"), "selection": selection(root, kind, values)}
		raw := objectValue(root, set)
		config := objectValue(root, map[string]tftypes.Value{"name": str("a"), "selection": selection(root, kind, values)})
		at := path.Root("selection").AtName("selection_type")
		resp := &planmodifier.StringResponse{PlanValue: types.StringUnknown()}
		attr.PlanModifiers[0].PlanModifyString(context.Background(), planmodifier.StringRequest{
			Path: at, Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: config}, Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: raw},
			ConfigValue: types.StringNull(), PlanValue: types.StringUnknown(),
		}, resp)
		return resp.PlanValue
	}
	if got := plan(nil, stringList("a")); got.ValueString() != "list" {
		t.Fatalf("values: type = %v, want list", got)
	}
	if got := plan(nil, stringList()); got.ValueString() != "all" {
		t.Fatalf("no values: type = %v, want all", got)
	}
	if got := plan(nil, tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, tftypes.UnknownValue)); !got.IsUnknown() {
		t.Fatalf("unknown values: type = %v, want unknown", got)
	}
}

// operator makes the operator object of a plan. A nil selection type is unknown, as it is when
// Create runs and the configuration leaves it out.
func operator(root tftypes.Object, kind string, selectionType *string, values tftypes.Value) tftypes.Value {
	selection := tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	if selectionType != nil {
		selection = str(*selectionType)
	}
	return objectValue(root.AttributeTypes["operator"], map[string]tftypes.Value{"type": str(kind), "selection_type": selection, "selected_values": values})
}

// An operator is one object in Terraform and two levels in the API: equals or notEquals, and then a
// selection. Each combination is sent in its own place and read back from there.
func TestOperatorIsFlatInTerraformAndNestedInTheAPI(t *testing.T) {
	root := thingRoot(t)
	cases := []struct {
		name          string
		kind          string
		selectionType *string
		values        tftypes.Value
		check         func(*typed_things_service.TypedOperator) bool
		state         string
		stateValues   []string
	}{
		{"equals with values", "equals", nil, stringList("a"), func(o *typed_things_service.TypedOperator) bool {
			return o.Equals != nil && o.Equals.Selection.List != nil && o.Equals.Selection.List.Values[0] == "a"
		}, "list", []string{"a"}},
		{"equals without values selects all", "equals", nil, stringList(), func(o *typed_things_service.TypedOperator) bool {
			return o.Equals != nil && o.Equals.Selection.All != nil
		}, "all", nil},
		{"equals with an empty list on purpose", "equals", ptr("list"), stringList(), func(o *typed_things_service.TypedOperator) bool {
			return o.Equals != nil && o.Equals.Selection.List != nil && len(o.Equals.Selection.List.Values) == 0
		}, "list", nil},
		{"not_equals with values", "not_equals", nil, stringList("a"), func(o *typed_things_service.TypedOperator) bool {
			return o.NotEquals != nil && o.NotEquals.Selection.List != nil && o.NotEquals.Selection.List.Values[0] == "a"
		}, "list", []string{"a"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp, _, _ := createWith(t, map[string]tftypes.Value{"operator": operator(root, c.kind, c.selectionType, c.values)}, nil)
			got := typed_things_service.Created[0].Operator
			if got == nil || !c.check(got) {
				t.Fatalf("operator = %+v", got)
			}
			var state map[string]tftypes.Value
			if err := resp.State.Raw.As(&state); err != nil {
				t.Fatal(err)
			}
			want := operator(root, c.kind, ptr(c.state), stringList(c.stateValues...))
			if !state["operator"].Equal(want) {
				t.Fatalf("state operator = %v, want %v", state["operator"], want)
			}
		})
	}
}

// The checks on the operator: not_equals has no all selection, and all has no values.
func TestOperatorValidatorsRejectTheImpossibleCombinations(t *testing.T) {
	_, schemaResp, root := thingSchema(t)
	attrs := schemaResp.Schema.Attributes["operator"].(schema.SingleNestedAttribute).Attributes
	validate := func(kind string, selectionType *string, values tftypes.Value) bool {
		t.Helper()
		config := tfsdk.Config{Schema: schemaResp.Schema, Raw: objectValue(root, map[string]tftypes.Value{
			"name": str("a"),
			"operator": objectValue(root.AttributeTypes["operator"], map[string]tftypes.Value{
				"type": str(kind), "selection_type": nullableString(selectionType), "selected_values": values,
			}),
		})}
		resp := &validator.StringResponse{}
		typeAt := path.Root("operator").AtName("type")
		for _, v := range attrs["type"].(schema.StringAttribute).Validators {
			v.ValidateString(context.Background(), validator.StringRequest{Path: typeAt, PathExpression: typeAt.Expression(), Config: config, ConfigValue: types.StringValue(kind)}, resp)
		}
		if selectionType != nil {
			selectionAt := path.Root("operator").AtName("selection_type")
			for _, v := range attrs["selection_type"].(schema.StringAttribute).Validators {
				v.ValidateString(context.Background(), validator.StringRequest{Path: selectionAt, PathExpression: selectionAt.Expression(), Config: config, ConfigValue: types.StringValue(*selectionType)}, resp)
			}
		}
		return !resp.Diagnostics.HasError()
	}
	if !validate("equals", ptr("all"), stringList()) || !validate("equals", ptr("list"), stringList("a")) || !validate("not_equals", ptr("list"), stringList("a")) {
		t.Fatal("equals with all, equals with a list, and not_equals with a list are valid")
	}
	if validate("not_equals", ptr("all"), stringList()) {
		t.Fatal("not_equals with all must be rejected")
	}
	if validate("equals", ptr("all"), stringList("a")) {
		t.Fatal("all with values must be rejected")
	}
}

func nullableString(s *string) tftypes.Value {
	if s == nil {
		return tftypes.NewValue(tftypes.String, nil)
	}
	return str(*s)
}

// A text that the user set to "" stays "" when it is in an arm: the read of the arm needs the prior
// value, because the line reads an empty text as null when the user left it out.
func TestPriorValuesReachTheSelectedArm(t *testing.T) {
	root := thingRoot(t)

	// An object arm of a single field.
	resp, _, _ := create(t, map[string]tftypes.Value{"type": str("avg"), "field": str("")})
	var state map[string]tftypes.Value
	if err := resp.State.Raw.As(&state); err != nil {
		t.Fatal(err)
	}
	want := objectValue(aggregationType(root), map[string]tftypes.Value{"type": str("avg"), "field": str("")})
	if !state["aggregation"].Equal(want) {
		t.Fatalf("aggregation = %v, want the configured empty field to stay", state["aggregation"])
	}

	// An arm of a typedArms object that is in another typedArms object: the operator.
	withNote := func(kind string, selectionType *string) tftypes.Value {
		return objectValue(root.AttributeTypes["operator"], map[string]tftypes.Value{
			"type": str(kind), "selection_type": nullableString(selectionType), "selected_values": stringList("a"), "note": str(""),
		})
	}
	for name, plan := range map[string]tftypes.Value{
		"selection": objectValue(root.AttributeTypes["selection"], map[string]tftypes.Value{
			"selection_type": str("list"), "selected_values": stringList("a"), "note": str(""),
		}),
		"operator": withNote("equals", ptr("list")),
	} {
		resp, _, _ := createWith(t, map[string]tftypes.Value{name: plan}, nil)
		if err := resp.State.Raw.As(&state); err != nil {
			t.Fatal(err)
		}
		var attrs map[string]tftypes.Value
		if err := state[name].As(&attrs); err != nil {
			t.Fatal(err)
		}
		if !attrs["note"].Equal(str("")) {
			t.Fatalf("%s note = %v, want the configured empty text to stay", name, attrs["note"])
		}
	}
}
