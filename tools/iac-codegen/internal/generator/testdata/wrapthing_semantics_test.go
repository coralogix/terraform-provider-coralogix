package wrapthing

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
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
		Filter:     &SimpleFilterModel{Text: types.StringValue("text"), Limit: types.Int32Value(3)},
		Selection:  strings("x", "y"),
		MainLabel:  types.StringValue("main"),
		ExtraLabel: &LabelModel{Value: types.StringValue("extra")},
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
		Filter:     &wrap_things_service.FilterHolder{Filter: &wrap_things_service.SimpleFilter{Text: ptr("text"), Limit: ptr(int32(3))}},
		Selection:  &wrap_things_service.Selection{List: &wrap_things_service.ListSelection{Values: []string{"x", "y"}}},
		MainLabel:  &wrap_things_service.Label{Value: ptr("main")},
		ExtraLabel: &wrap_things_service.Label{Value: ptr("extra")},
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
		Threshold: body.Threshold, WidgetIds: body.WidgetIds, Filter: body.Filter, Selection: body.Selection,
		MainLabel: body.MainLabel, ExtraLabel: body.ExtraLabel,
	}
	if !reflect.DeepEqual(&got, want) {
		t.Fatalf("expandCreate:\n got %+v\nwant %+v", got, *want)
	}
}

func TestExpandSendsNoWrapperForNull(t *testing.T) {
	body, diags := expandCreate(context.Background(), &WrapThingModel{
		Query: types.StringNull(), Priority: types.StringNull(), Threshold: types.Float64Null(),
		WidgetIds: types.ListNull(types.StringType), Selection: types.ListNull(types.StringType), MainLabel: types.StringNull(),
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	if body.Priority != nil || body.Threshold != nil || body.WidgetIds != nil || body.Filter != nil || body.Selection != nil || body.MainLabel != nil {
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
