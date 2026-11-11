package wrapthing

import (
	"context"
	"reflect"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"example.com/iac-test-sdk/go/openapi/gen/wrap_things_service"
)

func ptr[T any](v T) *T { return &v }

func stringList(values ...string) types.List {
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
		WidgetIds:  stringList("a", "b"),
		Filter:     &SimpleFilterModel{Text: types.StringValue("text"), Limit: types.Int32Value(3), Span: types.Int64Value(-3)},
		Selection:  stringList("x", "y"),
		Tags:       stringSet("p", "q"),
		Panels:     types.ListNull(types.ObjectType{AttrTypes: wrapPanelAttrTypes()}),
		FolderPath: types.StringNull(),
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
		WidgetIds: stringList(), Selection: stringList(), MainLabel: types.StringValue(""),
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
	if want := stringList("a", ""); !got.WidgetIds.Equal(want) {
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

func panelList(t *testing.T, panels ...WrapPanelModel) types.List {
	t.Helper()
	list, diags := types.ListValueFrom(context.Background(), types.ObjectType{AttrTypes: wrapPanelAttrTypes()}, panels)
	if diags.HasError() {
		t.Fatal(diags)
	}
	return list
}

var uuidV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// An id that the client makes: a known id is sent as it is, and an unknown id is a new version 4 UUID
// that differs from the next one. A null id is no id. The id is in a wrapper that the API holds.
func TestClientIDIsMadeOnlyForAnUnknownValue(t *testing.T) {
	body, diags := expandCreate(context.Background(), &WrapThingModel{
		Query: types.StringNull(), Priority: types.StringNull(), Threshold: types.Float64Null(),
		WidgetIds: types.ListNull(types.StringType), Selection: types.ListNull(types.StringType), Tags: types.SetNull(types.StringType), MainLabel: types.StringNull(),
		Panels: panelList(t,
			WrapPanelModel{Id: types.StringValue("fixed-id"), Title: types.StringNull()},
			WrapPanelModel{Id: types.StringUnknown(), Title: types.StringValue("a")},
			WrapPanelModel{Id: types.StringUnknown(), Title: types.StringValue("b")},
			WrapPanelModel{Id: types.StringNull(), Title: types.StringNull()},
		),
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	if len(body.Panels) != 4 {
		t.Fatalf("sent %d panels, want 4", len(body.Panels))
	}
	if got := body.Panels[0].Id.Value; got == nil || *got != "fixed-id" {
		t.Errorf("known id = %v, want fixed-id", got)
	}
	first, second := body.Panels[1].Id.Value, body.Panels[2].Id.Value
	if first == nil || second == nil || !uuidV4.MatchString(*first) || !uuidV4.MatchString(*second) || *first == *second {
		t.Errorf("made ids = %v and %v, want two different version 4 UUIDs", first, second)
	}
	if body.Panels[3].Id.Value != nil {
		t.Errorf("null id = %v, want none", *body.Panels[3].Id.Value)
	}
}

// The read puts the id that the API has in state, also with no prior value (an import), so the plan
// after an apply or an import has nothing to change.
func TestClientIDReadsBackAsTheAPIHasIt(t *testing.T) {
	ctx := context.Background()
	resp := fullResponse()
	resp.Panels = []wrap_things_service.WrapPanel{
		{Id: wrap_things_service.UUID{Value: ptr("id-1")}, Title: ptr("a")},
		{Id: wrap_things_service.UUID{Value: ptr("id-2")}},
	}
	{
		name := "an import, no prior"
		got, diags := flatten(ctx, resp)
		if diags.HasError() {
			t.Fatalf("%s: %v", name, diags)
		}
		var panels []WrapPanelModel
		if diags := got.Panels.ElementsAs(ctx, &panels, false); diags.HasError() {
			t.Fatal(diags)
		}
		if len(panels) != 2 || panels[0].Id.ValueString() != "id-1" || panels[1].Id.ValueString() != "id-2" || panels[0].Title.ValueString() != "a" || !panels[1].Title.IsNull() {
			t.Fatalf("%s: panels = %+v, want the ids and titles of the API", name, panels)
		}
	}
}

type clientIDProvider struct{}

func (clientIDProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "test"
}
func (clientIDProvider) Schema(context.Context, provider.SchemaRequest, *provider.SchemaResponse) {}
func (clientIDProvider) Configure(context.Context, provider.ConfigureRequest, *provider.ConfigureResponse) {
}
func (clientIDProvider) Resources(context.Context) []func() frameworkresource.Resource {
	return []func() frameworkresource.Resource{func() frameworkresource.Resource { return NewResource(Hooks{}) }}
}
func (clientIDProvider) DataSources(context.Context) []func() datasource.DataSource { return nil }

// The plan of an item without an id has the id unknown for a new item, and the id of the state for an
// item that the state has: Terraform plans an id that the configuration leaves out as unknown, and
// UseNonNullStateForUnknown keeps the state value.
func TestClientIDPlansUnknownForANewItemAndKeepsTheStateId(t *testing.T) {
	ctx := context.Background()
	server, err := providerserver.NewProtocol6WithError(clientIDProvider{})()
	if err != nil {
		t.Fatal(err)
	}
	schemaResp, err := server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	root := schemaResp.ResourceSchemas["test_wrap_thing"].ValueType().(tftypes.Object)
	panelType := root.AttributeTypes["panels"].(tftypes.List).ElementType.(tftypes.Object)
	panel := func(id tftypes.Value) tftypes.Value {
		return tftypes.NewValue(panelType, map[string]tftypes.Value{"id": id, "title": tftypes.NewValue(tftypes.String, nil)})
	}
	thing := func(id string, panels ...tftypes.Value) tftypes.Value {
		values := map[string]tftypes.Value{}
		for name, typ := range root.AttributeTypes {
			values[name] = tftypes.NewValue(typ, nil)
		}
		values["id"] = tftypes.NewValue(tftypes.String, id)
		values["query"] = tftypes.NewValue(tftypes.String, "q")
		values["panels"] = tftypes.NewValue(root.AttributeTypes["panels"], panels)
		return tftypes.NewValue(root, values)
	}
	encode := func(v tftypes.Value) *tfprotov6.DynamicValue {
		dv, err := tfprotov6.NewDynamicValue(root, v)
		if err != nil {
			t.Fatal(err)
		}
		return &dv
	}
	null := tftypes.NewValue(tftypes.String, nil)
	prior := thing("thing-1", panel(tftypes.NewValue(tftypes.String, "kept")))
	config := thing("thing-1", panel(null), panel(null))
	proposed := thing("thing-1", panel(tftypes.NewValue(tftypes.String, "kept")), panel(tftypes.NewValue(tftypes.String, tftypes.UnknownValue)))
	plan, err := server.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{TypeName: "test_wrap_thing", PriorState: encode(prior), Config: encode(config), ProposedNewState: encode(proposed)})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range plan.Diagnostics {
		t.Fatalf("plan: %s: %s", d.Summary, d.Detail)
	}
	planned, err := plan.PlannedState.Unmarshal(root)
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]tftypes.Value
	if err := planned.As(&values); err != nil {
		t.Fatal(err)
	}
	var panels []tftypes.Value
	if err := values["panels"].As(&panels); err != nil {
		t.Fatal(err)
	}
	var first, second map[string]tftypes.Value
	if err := panels[0].As(&first); err != nil {
		t.Fatal(err)
	}
	if err := panels[1].As(&second); err != nil {
		t.Fatal(err)
	}
	if !first["id"].Equal(tftypes.NewValue(tftypes.String, "kept")) {
		t.Errorf("the first panel id = %v, want the id of the state", first["id"])
	}
	if second["id"].IsKnown() {
		t.Errorf("the new panel id = %v, want unknown", second["id"])
	}
}

// A list of strings that the API holds in a wrapper is one text: the segments of a folder path are
// "Team/Sub". Expand splits at every slash with no trimming, as the released resource did, and
// flatten joins. A text that is null sends no wrapper and a response without a wrapper reads as null.
func TestJoinedSegmentsAreOneText(t *testing.T) {
	ctx := context.Background()
	for text, want := range map[string][]string{
		"Team/Sub":  {"Team", "Sub"},
		"Team":      {"Team"},
		"":          {""},
		"/Team":     {"", "Team"},
		"Team/":     {"Team", ""},
		"Team//Sub": {"Team", "", "Sub"},
	} {
		body, diags := expandCreate(ctx, &WrapThingModel{
			Query: types.StringNull(), Priority: types.StringNull(), Threshold: types.Float64Null(),
			WidgetIds: types.ListNull(types.StringType), Selection: types.ListNull(types.StringType), Tags: types.SetNull(types.StringType), MainLabel: types.StringNull(),
			FolderPath: types.StringValue(text),
		})
		if diags.HasError() {
			t.Fatal(diags)
		}
		if body.FolderPath == nil || !reflect.DeepEqual(body.FolderPath.Segments, want) {
			t.Errorf("%q sent %+v, want %v", text, body.FolderPath, want)
		}
		resp := fullResponse()
		resp.FolderPath = &wrap_things_service.FolderPath{Segments: want}
		got, diags := flatten(ctx, resp)
		if diags.HasError() || got.FolderPath.ValueString() != text {
			t.Errorf("%v read as %q (%v), want %q", want, got.FolderPath.ValueString(), diags, text)
		}
	}
	for name, folder := range map[string]types.String{"null": types.StringNull(), "unknown": types.StringUnknown()} {
		body, _ := expandCreate(ctx, &WrapThingModel{
			Query: types.StringNull(), Priority: types.StringNull(), Threshold: types.Float64Null(),
			WidgetIds: types.ListNull(types.StringType), Selection: types.ListNull(types.StringType), Tags: types.SetNull(types.StringType), MainLabel: types.StringNull(),
			FolderPath: folder,
		})
		if body.FolderPath != nil {
			t.Errorf("%s sent %+v, want no wrapper", name, body.FolderPath)
		}
	}
	got, _ := flatten(ctx, fullResponse())
	if !got.FolderPath.IsNull() {
		t.Errorf("a response without the path read as %v, want null", got.FolderPath)
	}
}
