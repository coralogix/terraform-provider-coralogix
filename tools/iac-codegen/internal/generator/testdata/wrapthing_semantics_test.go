package wrapthing

import (
	"context"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
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
	for i := range panels {
		if panels[i].PanelNotes.ElementType(context.Background()) == nil {
			panels[i].PanelNotes = types.ListNull(types.ObjectType{AttrTypes: wrapNoteAttrTypes()})
		}
	}
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
		return tftypes.NewValue(panelType, map[string]tftypes.Value{
			"id": id, "title": tftypes.NewValue(tftypes.String, nil), "panel_notes": tftypes.NewValue(panelType.AttributeTypes["panel_notes"], nil),
		})
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

func noteList(t *testing.T, notes ...WrapNoteModel) types.List {
	t.Helper()
	list, diags := types.ListValueFrom(context.Background(), types.ObjectType{AttrTypes: wrapNoteAttrTypes()}, notes)
	if diags.HasError() {
		t.Fatal(diags)
	}
	return list
}

func noteOf(id, text, url string) WrapNoteModel {
	note := WrapNoteModel{Id: types.StringValue(id), Text: types.StringValue(text), Detail: &NoteDetailModel{Url: types.StringValue(url)}}
	if id == "" {
		note.Id = types.StringUnknown()
	}
	return note
}

func apiNote(id, panel, url string) wrap_things_service.WrapNote {
	note := wrap_things_service.WrapNote{Id: id, Text: ptr("text of " + id), PanelId: wrap_things_service.UUID{Value: ptr(panel)}}
	if url != "" {
		note.Detail = &wrap_things_service.NoteDetail{Url: ptr(url)}
	}
	return note
}

func notesModel(t *testing.T, panels ...WrapPanelModel) *WrapThingModel {
	t.Helper()
	return &WrapThingModel{Query: types.StringValue("q"), Panels: panelList(t, panels...)}
}

func panelsOf(t *testing.T, m *WrapThingModel) []WrapPanelModel {
	t.Helper()
	var panels []WrapPanelModel
	if diags := m.Panels.ElementsAs(context.Background(), &panels, false); diags.HasError() {
		t.Fatal(diags)
	}
	return panels
}

// The notes of a panel are in the list of the thing, and each one names its panel by the id of the
// panel. Expand takes them out of the panels in order, and writes the id that the panel has in the
// request: a known id as it is, and the UUID that expand makes for a panel without an id, the same
// in the panel and in its notes. The notes have their own ids, made in the same way. Create and
// Update send the same list.
func TestDistributedItemsAreSentInTheListOfTheThing(t *testing.T) {
	ctx := context.Background()
	model := notesModel(t,
		WrapPanelModel{Id: types.StringValue("p1"), PanelNotes: noteList(t, noteOf("a", "first", "u1"), noteOf("b", "second", "u2"))},
		WrapPanelModel{Id: types.StringValue("p2"), PanelNotes: noteList(t, noteOf("c", "third", "u3"))},
		WrapPanelModel{Id: types.StringUnknown(), PanelNotes: noteList(t, noteOf("", "fourth", "u4"))},
		WrapPanelModel{Id: types.StringValue("p4"), PanelNotes: types.ListNull(types.ObjectType{AttrTypes: wrapNoteAttrTypes()})},
		WrapPanelModel{Id: types.StringValue("p5"), PanelNotes: noteList(t)},
	)
	create, diags := expandCreate(ctx, model)
	if diags.HasError() {
		t.Fatal(diags)
	}
	update, diags := expandUpdate(ctx, model)
	if diags.HasError() {
		t.Fatal(diags)
	}
	for name, notes := range map[string][]wrap_things_service.WrapNote{"create": create.Notes, "update": update.Notes} {
		if len(notes) != 4 {
			t.Fatalf("%s sent %d notes, want 4: %+v", name, len(notes), notes)
		}
		for i, want := range []struct{ id, panel, text, url string }{{"a", "p1", "first", "u1"}, {"b", "p1", "second", "u2"}, {"c", "p2", "third", "u3"}} {
			note := notes[i]
			if note.Id != want.id || *note.PanelId.Value != want.panel || *note.Text != want.text || *note.Detail.Url != want.url {
				t.Errorf("%s note %d = %+v, want %+v", name, i, note, want)
			}
		}
	}
	// The panel without an id gets a made UUID, and its note names that UUID. The note has an id of its own.
	made := create.Panels[2].Id.Value
	last := create.Notes[3]
	if made == nil || !uuidV4.MatchString(*made) {
		t.Fatalf("the panel without an id got %v, want a made UUID", made)
	}
	if last.PanelId.Value == nil || *last.PanelId.Value != *made {
		t.Errorf("the note names the panel %v, want the id that the panel is sent with, %s", last.PanelId.Value, *made)
	}
	if !uuidV4.MatchString(last.Id) || last.Id == *made {
		t.Errorf("the note id = %q, want its own made UUID", last.Id)
	}
	if len(create.Panels) != 5 {
		t.Errorf("sent %d panels, want 5", len(create.Panels))
	}
}

// Without notes there is no list to send, and a request does not keep the notes of the one before.
func TestDistributedItemsAreNotSentWhenThereAreNone(t *testing.T) {
	ctx := context.Background()
	model := notesModel(t, WrapPanelModel{Id: types.StringValue("p1")}, WrapPanelModel{Id: types.StringValue("p2"), PanelNotes: noteList(t)})
	for name, expand := range map[string]func() ([]wrap_things_service.WrapNote, diag.Diagnostics){
		"create": func() ([]wrap_things_service.WrapNote, diag.Diagnostics) {
			b, d := expandCreate(ctx, model)
			return b.Notes, d
		},
		"update": func() ([]wrap_things_service.WrapNote, diag.Diagnostics) {
			b, d := expandUpdate(ctx, model)
			return b.Notes, d
		},
	} {
		notes, diags := expand()
		if diags.HasError() || len(notes) != 0 {
			t.Errorf("%s: notes = %+v (%v), want none", name, notes, diags)
		}
	}
	// A request starts with an empty list: the notes of the one before are not in it.
	with := notesModel(t, WrapPanelModel{Id: types.StringValue("p1"), PanelNotes: noteList(t, noteOf("a", "x", "u"))})
	for range 2 {
		body, diags := expandCreate(ctx, with)
		if diags.HasError() || len(body.Notes) != 1 {
			t.Errorf("notes = %+v (%v), want one", body.Notes, diags)
		}
	}
}

// A note names its panel by the id of the panel, so a panel without one cannot have notes.
func TestDistributedItemsNeedTheIdOfTheirPanel(t *testing.T) {
	model := notesModel(t, WrapPanelModel{Id: types.StringNull(), PanelNotes: noteList(t, noteOf("a", "x", "u"))})
	if _, diags := expandCreate(context.Background(), model); !diags.HasError() {
		t.Fatal("a panel with notes and no id: want an error")
	}
}

// A response shows only the notes that have a url, so a note without one would be missing after the
// write: the read would not return it. Expand refuses such a note, with an error on the note, and
// sends nothing. A note with a url is sent as before.
func TestDistributedItemThatAResponseWouldNotShowIsRefused(t *testing.T) {
	noUrl := noteOf("a", "x", "u")
	noUrl.Detail = nil
	emptyDetail := noteOf("b", "y", "u")
	emptyDetail.Detail = &NoteDetailModel{Url: types.StringNull()}
	for name, note := range map[string]WrapNoteModel{"no detail": noUrl, "a detail without a url": emptyDetail} {
		model := notesModel(t, WrapPanelModel{Id: types.StringValue("p1"), PanelNotes: noteList(t, noteOf("c", "ok", "u"), note)})
		_, createDiags := expandCreate(context.Background(), model)
		_, updateDiags := expandUpdate(context.Background(), model)
		for call, diags := range map[string]diag.Diagnostics{"create": createDiags, "update": updateDiags} {
			if !diags.HasError() {
				t.Fatalf("%s %s: want an error for a note that the response would not show", name, call)
			}
			found := false
			for _, d := range diags.Errors() {
				if withPath, ok := d.(diag.DiagnosticWithPath); ok && withPath.Path().Equal(path.Root("panels").AtListIndex(0).AtName("panel_notes").AtListIndex(1)) && strings.Contains(d.Detail(), "detail.url") {
					found = true
				}
			}
			if !found {
				t.Errorf("%s %s: the errors %v have none for panels[0].panel_notes[1] that names detail.url", name, call, diags.Errors())
			}
		}
	}
}

// Flatten gives each panel the notes that name it, in the order of the list, with the ids that the API
// has. A note without a url is another kind of note, so no panel shows it, and a note of another
// panel is in no panel. A panel with none reads null. A read has no prior value, so this is also the
// read after an import.
func TestDistributedItemsAreReadIntoTheirPanels(t *testing.T) {
	ctx := context.Background()
	response := &wrap_things_service.WrapThing{
		Id: "thing-1",
		Panels: []wrap_things_service.WrapPanel{
			{Id: wrap_things_service.UUID{Value: ptr("p1")}},
			{Id: wrap_things_service.UUID{Value: ptr("p2")}},
			{Id: wrap_things_service.UUID{Value: ptr("p3")}},
		},
		Notes: []wrap_things_service.WrapNote{
			apiNote("a", "p1", "u1"),
			apiNote("b", "p2", "u2"),
			apiNote("c", "p1", "u3"),
			apiNote("d", "p1", ""),
			{Id: "e", PanelId: wrap_things_service.UUID{Value: ptr("p2")}, Detail: &wrap_things_service.NoteDetail{}},
			apiNote("f", "other", "u4"),
			{Id: "g", PanelId: wrap_things_service.UUID{}, Detail: &wrap_things_service.NoteDetail{Url: ptr("u5")}},
		},
	}
	got, diags := flatten(ctx, response)
	if diags.HasError() {
		t.Fatal(diags)
	}
	panels := panelsOf(t, got)
	if len(panels) != 3 {
		t.Fatalf("read %d panels, want 3", len(panels))
	}
	want := []types.List{
		noteList(t, WrapNoteModel{Id: types.StringValue("a"), Text: types.StringValue("text of a"), Detail: &NoteDetailModel{Url: types.StringValue("u1")}},
			WrapNoteModel{Id: types.StringValue("c"), Text: types.StringValue("text of c"), Detail: &NoteDetailModel{Url: types.StringValue("u3")}}),
		noteList(t, WrapNoteModel{Id: types.StringValue("b"), Text: types.StringValue("text of b"), Detail: &NoteDetailModel{Url: types.StringValue("u2")}}),
		types.ListNull(types.ObjectType{AttrTypes: wrapNoteAttrTypes()}),
	}
	for i, panel := range panels {
		if !panel.PanelNotes.Equal(want[i]) {
			t.Errorf("panel %d notes = %s, want %s", i, panel.PanelNotes, want[i])
		}
	}
	// A response without notes gives every panel null.
	response.Notes = nil
	got, diags = flatten(ctx, response)
	if diags.HasError() {
		t.Fatal(diags)
	}
	for i, panel := range panelsOf(t, got) {
		if !panel.PanelNotes.IsNull() {
			t.Errorf("panel %d notes = %s, want null", i, panel.PanelNotes)
		}
	}
}

// The notes of a panel come back as they were sent: expand, then flatten the list that the request
// holds, gives the same panels, and the same notes in them.
func TestDistributedItemsSurviveARoundTrip(t *testing.T) {
	ctx := context.Background()
	model := notesModel(t,
		WrapPanelModel{Id: types.StringValue("p1"), Title: types.StringValue("one"), PanelNotes: noteList(t, noteOf("a", "x", "u1"), noteOf("b", "y", "u2"))},
		WrapPanelModel{Id: types.StringValue("p2"), Title: types.StringNull(), PanelNotes: noteList(t, noteOf("c", "z", "u3"))},
		WrapPanelModel{Id: types.StringValue("p3"), Title: types.StringNull(), PanelNotes: types.ListNull(types.ObjectType{AttrTypes: wrapNoteAttrTypes()})},
	)
	body, diags := expandCreate(ctx, model)
	if diags.HasError() {
		t.Fatal(diags)
	}
	got, diags := flatten(ctx, &wrap_things_service.WrapThing{Id: "thing-1", Query: body.Query, Panels: body.Panels, Notes: body.Notes})
	if diags.HasError() {
		t.Fatal(diags)
	}
	if !got.Panels.Equal(model.Panels) {
		t.Fatalf("panels:\n got %s\nwant %s", got.Panels, model.Panels)
	}
}

// The notes are an attribute of the panel and no attribute of the thing, and the id that names the
// panel is no attribute of the note.
func TestDistributedItemsAreOnlyInsideTheirPanel(t *testing.T) {
	var resp frameworkresource.SchemaResponse
	(&Resource{}).Schema(context.Background(), frameworkresource.SchemaRequest{}, &resp)
	if _, ok := resp.Schema.Attributes["notes"]; ok {
		t.Error("the thing has the attribute notes, want it only in the panels")
	}
	panels, ok := resp.Schema.Attributes["panels"].(schema.ListNestedAttribute)
	if !ok {
		t.Fatalf("panels = %#v", resp.Schema.Attributes["panels"])
	}
	notes, ok := panels.NestedObject.Attributes["panel_notes"].(schema.ListNestedAttribute)
	if !ok || !notes.Optional || notes.Required || notes.Computed {
		t.Fatalf("panel_notes = %#v, want an optional list of objects", panels.NestedObject.Attributes["panel_notes"])
	}
	if notes.MarkdownDescription != "The notes of this panel." {
		t.Errorf("the description = %q, want the text of the line", notes.MarkdownDescription)
	}
	for name := range notes.NestedObject.Attributes {
		if name == "panel_id" {
			t.Error("a note has the attribute panel_id, want the panel to name it")
		}
	}
	if _, ok := reflect.TypeOf(WrapThingModel{}).FieldByName("Notes"); ok {
		t.Error("the model of the thing has Notes")
	}
	if _, ok := reflect.TypeOf(WrapNoteModel{}).FieldByName("PanelId"); ok {
		t.Error("the model of a note has PanelId")
	}
}
