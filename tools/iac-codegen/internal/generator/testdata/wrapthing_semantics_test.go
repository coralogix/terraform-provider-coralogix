package wrapthing

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"example.com/iac-test-sdk/go/openapi/gen/wrap_things_service"
)

func ptr[T any](v T) *T { return &v }

func strings(values ...string) types.List {
	elems := make([]attr.Value, 0, len(values))
	for _, v := range values {
		elems = append(elems, types.StringValue(v))
	}
	return types.ListValueMust(types.StringType, elems)
}

func stringSet(values ...string) types.Set {
	elems := make([]attr.Value, 0, len(values))
	for _, v := range values {
		elems = append(elems, types.StringValue(v))
	}
	return types.SetValueMust(types.StringType, elems)
}

// fullModel sets every attribute. Terraform shows the wrapped values without their wrappers.
func fullModel() *WrapThingModel {
	return &WrapThingModel{
		Id:         types.StringValue("thing-1"),
		Name:       types.StringValue("name"),
		Query:      types.StringValue("error"),
		RawQuery:   &LuceneQueryModel{Value: types.StringValue("raw")},
		Priority:   types.StringValue("WRAP_PRIORITY_P1"),
		Threshold:  types.Float64Value(0.5),
		WidgetIds:  strings("a", "b"),
		Filter:     &SimpleFilterModel{Text: types.StringValue("text"), Limit: types.Int32Value(3), Span: types.Int64Value(-3)},
		Selection:  strings("x", "y"),
		Tags:       stringSet("p", "q"),
		MainLabel:  types.StringValue("main"),
		ExtraLabel: &LabelModel{Value: types.StringValue("extra")},
		Offset:     types.Int64Value(-9007199254740993),
		Window:     types.StringValue("2_HOURS"),
	}
}

// fullResponse is the API form of fullModel.
func fullResponse() *wrap_things_service.WrapThing {
	return &wrap_things_service.WrapThing{
		Id:         "thing-1",
		Name:       ptr("name"),
		Query:      wrap_things_service.LuceneQuery{Value: ptr("error")},
		RawQuery:   &wrap_things_service.LuceneQuery{Value: ptr("raw")},
		Priority:   &wrap_things_service.PriorityValue{Value: ptr(wrap_things_service.WRAPPRIORITY_WRAP_PRIORITY_P1)},
		Threshold:  &wrap_things_service.Threshold{Value: 0.5},
		WidgetIds:  []wrap_things_service.UUID{{Value: ptr("a")}, {Value: ptr("b")}},
		Filter:     &wrap_things_service.FilterHolder{Filter: &wrap_things_service.SimpleFilter{Text: ptr("text"), Limit: ptr(int32(3)), Span: ptr("-3")}},
		Selection:  &wrap_things_service.Selection{List: &wrap_things_service.ListSelection{Values: []string{"x", "y"}}},
		Tags:       &wrap_things_service.Tags{Values: []string{"p", "q"}},
		MainLabel:  &wrap_things_service.Label{Value: ptr("main")},
		ExtraLabel: &wrap_things_service.Label{Value: ptr("extra")},
		Offset:     ptr("-9007199254740993"),
		Window:     &wrap_things_service.Window{FixedValue: ptr(wrap_things_service.WINDOWVALUE_WINDOW_VALUE_HOURS_2)},
	}
}

func TestExpandPutsValuesInTheirWrappers(t *testing.T) {
	body, diags := expandCreate(context.Background(), fullModel())
	if diags.HasError() {
		t.Fatal(diags)
	}
	want := fullResponse()
	got := wrap_things_service.WrapThing{
		Id: want.Id, Name: body.Name, Query: body.Query, RawQuery: body.RawQuery, Priority: body.Priority,
		Threshold: body.Threshold, WidgetIds: body.WidgetIds, Filter: body.Filter, Selection: body.Selection, Tags: body.Tags, Window: body.Window,
		MainLabel: body.MainLabel, ExtraLabel: body.ExtraLabel, Offset: body.Offset,
	}
	if !reflect.DeepEqual(&got, want) {
		t.Fatalf("expandCreate:\n got %+v\nwant %+v", got, *want)
	}
}

func TestExpandSendsNoWrapperForNull(t *testing.T) {
	body, diags := expandCreate(context.Background(), &WrapThingModel{
		Query: types.StringNull(), Priority: types.StringNull(), Threshold: types.Float64Null(),
		WidgetIds: types.ListNull(types.StringType), Selection: types.ListNull(types.StringType), Tags: types.SetNull(types.StringType), MainLabel: types.StringNull(),
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	if body.Priority != nil || body.Threshold != nil || body.WidgetIds != nil || body.Filter != nil || body.Selection != nil || body.Tags != nil || body.MainLabel != nil {
		t.Fatalf("a null value sent a wrapper: %+v", body)
	}
	// The SDK holds the required query as a value, so a null query is an empty wrapper.
	if body.Query.Value != nil {
		t.Fatalf("query = %+v, want an empty wrapper", body.Query)
	}
}

func TestExpandSendsZeroValuesInTheirWrappers(t *testing.T) {
	body, diags := expandCreate(context.Background(), &WrapThingModel{
		Query: types.StringValue(""), Priority: types.StringNull(), Threshold: types.Float64Value(0),
		WidgetIds: strings(), Selection: strings(), MainLabel: types.StringValue(""),
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	if body.Query.Value == nil || *body.Query.Value != "" {
		t.Fatalf("query = %+v, want the empty string", body.Query)
	}
	if body.Threshold == nil || body.Threshold.Value != 0 {
		t.Fatalf("threshold = %+v, want 0", body.Threshold)
	}
	if body.WidgetIds == nil || len(body.WidgetIds) != 0 {
		t.Fatalf("widget ids = %#v, want an empty list", body.WidgetIds)
	}
	if body.Selection == nil || body.Selection.List == nil || len(body.Selection.List.Values) != 0 {
		t.Fatalf("selection = %+v, want an empty list in its wrappers", body.Selection)
	}
	if body.MainLabel == nil || body.MainLabel.Value == nil || *body.MainLabel.Value != "" {
		t.Fatalf("main label = %+v, want the empty string", body.MainLabel)
	}
}

func TestFlattenTakesValuesOutOfTheirWrappers(t *testing.T) {
	got, diags := flatten(context.Background(), fullResponse())
	if diags.HasError() {
		t.Fatal(diags)
	}
	if !reflect.DeepEqual(got, fullModel()) {
		t.Fatalf("flatten:\n got %+v\nwant %+v", *got, *fullModel())
	}
}

func TestFlattenReadsAMissingValueAsNull(t *testing.T) {
	got, diags := flatten(context.Background(), &wrap_things_service.WrapThing{
		Id:        "thing-1",
		Priority:  &wrap_things_service.PriorityValue{},
		Selection: &wrap_things_service.Selection{},
		MainLabel: &wrap_things_service.Label{},
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	for name, value := range map[string]attr.Value{
		"query": got.Query, "priority": got.Priority, "threshold": got.Threshold,
		"widget_ids": got.WidgetIds, "selection": got.Selection, "main_label": got.MainLabel,
	} {
		if !value.IsNull() {
			t.Errorf("%s = %s, want null", name, value)
		}
	}
	if got.Filter != nil || got.RawQuery != nil || got.ExtraLabel != nil {
		t.Errorf("objects = %+v %+v %+v, want nil", got.Filter, got.RawQuery, got.ExtraLabel)
	}
}

func TestFlattenKeepsAnItemWithoutValue(t *testing.T) {
	got, diags := flatten(context.Background(), &wrap_things_service.WrapThing{
		Id:        "thing-1",
		WidgetIds: []wrap_things_service.UUID{{Value: ptr("a")}, {}},
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	if want := strings("a", ""); !got.WidgetIds.Equal(want) {
		t.Fatalf("widget ids = %s, want %s", got.WidgetIds, want)
	}
}

// A signed 64-bit number that the API sends as a string is an Int64 in Terraform: expand writes its
// decimal text, a negative value and one past 2^53 included, and a null value sends nothing.
func TestSignedDigitStringIsSentAsDecimalText(t *testing.T) {
	for name, test := range map[string]struct {
		value types.Int64
		want  *string
	}{
		"negative":      {types.Int64Value(-5), ptr("-5")},
		"zero":          {types.Int64Value(0), ptr("0")},
		"int64 minimum": {types.Int64Value(-1 << 63), ptr("-9223372036854775808")},
		"int64 maximum": {types.Int64Value(1<<63 - 1), ptr("9223372036854775807")},
		"null":          {types.Int64Null(), nil},
		"unknown":       {types.Int64Unknown(), nil},
	} {
		t.Run(name, func(t *testing.T) {
			body, diags := expandCreate(context.Background(), &WrapThingModel{Offset: test.value})
			if diags.HasError() {
				t.Fatal(diags)
			}
			if !reflect.DeepEqual(body.Offset, test.want) {
				t.Fatalf("offset = %v, want %v", body.Offset, test.want)
			}
		})
	}
}

// Flatten parses the text of the API. A text that is no 64-bit number is an error, not a null.
func TestSignedDigitStringIsReadAsAnInteger(t *testing.T) {
	for text, want := range map[string]int64{"-5": -5, "0": 0, "42": 42, "-9223372036854775808": -1 << 63} {
		got, diags := flatten(context.Background(), &wrap_things_service.WrapThing{Id: "thing-1", Offset: ptr(text)})
		if diags.HasError() || got.Offset.ValueInt64() != want {
			t.Fatalf("offset of %q = %v (%v), want %d", text, got.Offset, diags, want)
		}
	}
	got, diags := flatten(context.Background(), &wrap_things_service.WrapThing{Id: "thing-1"})
	if diags.HasError() || !got.Offset.IsNull() {
		t.Fatalf("a missing offset = %v (%v), want null", got.Offset, diags)
	}
	for _, text := range []string{"", "abc", "1.5", "9223372036854775808", "-9223372036854775809"} {
		if _, diags := flatten(context.Background(), &wrap_things_service.WrapThing{Id: "thing-1", Offset: ptr(text)}); !diags.HasError() {
			t.Errorf("offset %q: want an error", text)
		}
	}
}

// A list inside a wrapper can be a set: expand writes the elements of the set, in the order that
// Terraform gives them, inside the wrapper, and an empty set is an empty list in its wrapper.
func TestWrappedSetIsSentInItsWrapper(t *testing.T) {
	for name, test := range map[string]struct {
		value types.Set
		want  *wrap_things_service.Tags
	}{
		"elements": {stringSet("a", "b"), &wrap_things_service.Tags{Values: []string{"a", "b"}}},
		"empty":    {stringSet(), &wrap_things_service.Tags{Values: []string{}}},
		"null":     {types.SetNull(types.StringType), nil},
	} {
		t.Run(name, func(t *testing.T) {
			body, diags := expandCreate(context.Background(), &WrapThingModel{Tags: test.value})
			if diags.HasError() {
				t.Fatal(diags)
			}
			if !reflect.DeepEqual(body.Tags, test.want) {
				t.Fatalf("tags = %+v, want %+v", body.Tags, test.want)
			}
		})
	}
}

// Flatten builds the set from the list in the wrapper, in any order. A missing wrapper, and a
// wrapper without values, are null.
func TestWrappedSetIsReadFromItsWrapper(t *testing.T) {
	got, diags := flatten(context.Background(), &wrap_things_service.WrapThing{Id: "thing-1", Tags: &wrap_things_service.Tags{Values: []string{"b", "a"}}})
	if diags.HasError() || !got.Tags.Equal(stringSet("a", "b")) {
		t.Fatalf("tags = %s (%v), want [a b] as a set", got.Tags, diags)
	}
	got, diags = flatten(context.Background(), &wrap_things_service.WrapThing{Id: "thing-1", Tags: &wrap_things_service.Tags{Values: []string{}}})
	if diags.HasError() || got.Tags.IsNull() || len(got.Tags.Elements()) != 0 {
		t.Fatalf("tags = %s (%v), want an empty set", got.Tags, diags)
	}
	for name, response := range map[string]*wrap_things_service.WrapThing{
		"no wrapper":           {Id: "thing-1"},
		"a wrapper of no list": {Id: "thing-1", Tags: &wrap_things_service.Tags{}},
	} {
		got, diags = flatten(context.Background(), response)
		if diags.HasError() || !got.Tags.IsNull() {
			t.Errorf("%s: tags = %s (%v), want null", name, got.Tags, diags)
		}
	}
}

// A oneOf of an enum and a string is one Terraform string. A spelling of the enum selects the enum
// arm, the zero value of the enum included, and any other text is the string arm.
func TestFoldedOneOfIsSentAsTheArmOfItsValue(t *testing.T) {
	for name, test := range map[string]struct {
		value types.String
		want  *wrap_things_service.Window
	}{
		"a spelling of the enum": {types.StringValue("5_MINUTES"), &wrap_things_service.Window{FixedValue: ptr(wrap_things_service.WINDOWVALUE_WINDOW_VALUE_MINUTES_5)}},
		"the zero value":         {types.StringValue("1_MINUTE"), &wrap_things_service.Window{FixedValue: ptr(wrap_things_service.WINDOWVALUE_WINDOW_VALUE_MINUTES_1_OR_UNSPECIFIED)}},
		"a duration":             {types.StringValue("90m"), &wrap_things_service.Window{CustomValue: ptr("90m")}},
		"the API spelling":       {types.StringValue("WINDOW_VALUE_MINUTES_5"), &wrap_things_service.Window{CustomValue: ptr("WINDOW_VALUE_MINUTES_5")}},
		"null":                   {types.StringNull(), nil},
		"unknown":                {types.StringUnknown(), nil},
	} {
		t.Run(name, func(t *testing.T) {
			body, diags := expandCreate(context.Background(), &WrapThingModel{Window: test.value})
			if diags.HasError() {
				t.Fatal(diags)
			}
			if !reflect.DeepEqual(body.Window, test.want) {
				t.Fatalf("window = %+v, want %+v", body.Window, test.want)
			}
		})
	}
}

// Flatten reads the arm that the response has, and a response with no arm, or with two, is null or
// an error, as for a oneOf.
func TestFoldedOneOfIsReadFromTheArmOfTheResponse(t *testing.T) {
	for name, test := range map[string]struct {
		api  *wrap_things_service.Window
		want types.String
	}{
		"the enum arm":    {&wrap_things_service.Window{FixedValue: ptr(wrap_things_service.WINDOWVALUE_WINDOW_VALUE_MINUTES_5)}, types.StringValue("5_MINUTES")},
		"the zero value":  {&wrap_things_service.Window{FixedValue: ptr(wrap_things_service.WINDOWVALUE_WINDOW_VALUE_MINUTES_1_OR_UNSPECIFIED)}, types.StringValue("1_MINUTE")},
		"the string arm":  {&wrap_things_service.Window{CustomValue: ptr("1h30m")}, types.StringValue("1h30m")},
		"an empty string": {&wrap_things_service.Window{CustomValue: ptr("")}, types.StringValue("")},
		"no arm":          {&wrap_things_service.Window{}, types.StringNull()},
		"no object":       {nil, types.StringNull()},
	} {
		t.Run(name, func(t *testing.T) {
			got, diags := flatten(context.Background(), &wrap_things_service.WrapThing{Id: "thing-1", Window: test.api})
			if diags.HasError() || !got.Window.Equal(test.want) {
				t.Fatalf("window = %s (%v), want %s", got.Window, diags, test.want)
			}
		})
	}
	both := &wrap_things_service.Window{FixedValue: ptr(wrap_things_service.WINDOWVALUE_WINDOW_VALUE_MINUTES_5), CustomValue: ptr("5m")}
	if _, diags := flatten(context.Background(), &wrap_things_service.WrapThing{Id: "thing-1", Window: both}); !diags.HasError() {
		t.Fatal("a response with both arms: want an error")
	}
	unknown := &wrap_things_service.Window{FixedValue: ptr(wrap_things_service.WindowValue("WINDOW_VALUE_HOURS_99"))}
	if _, diags := flatten(context.Background(), &wrap_things_service.WrapThing{Id: "thing-1", Window: unknown}); !diags.HasError() {
		t.Fatal("an enum value that the enums line lacks: want an error")
	}
}

// The attribute takes a spelling of the enum or a match of the pattern, and nothing else.
func TestFoldedOneOfValidatorAcceptsTheUnionOfBothArms(t *testing.T) {
	var resp frameworkresource.SchemaResponse
	(&Resource{}).Schema(context.Background(), frameworkresource.SchemaRequest{}, &resp)
	attribute, ok := resp.Schema.Attributes["window"].(schema.StringAttribute)
	if !ok || len(attribute.Validators) != 1 {
		t.Fatalf("window = %#v, want a string attribute with one validator", resp.Schema.Attributes["window"])
	}
	for value, valid := range map[string]bool{
		"1_MINUTE": true, "5_MINUTES": true, "2_HOURS": true, "90m": true, "30s": true, "2h": true,
		"WINDOW_VALUE_HOURS_2": false, "five": false, "": false, "1.5h": false, "5_minutes": false,
	} {
		var diags validator.StringResponse
		attribute.Validators[0].ValidateString(context.Background(), validator.StringRequest{ConfigValue: types.StringValue(value)}, &diags)
		if diags.Diagnostics.HasError() == valid {
			t.Errorf("window %q: valid = %v, want %v (%v)", value, !diags.Diagnostics.HasError(), valid, diags.Diagnostics)
		}
	}
}
