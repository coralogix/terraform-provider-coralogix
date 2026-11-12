package unionwrapthing

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"example.com/iac-test-sdk/go/openapi/gen/union_wrap_things_service"
)

// Create and Update send the inlined object, with the common fields and the selected arm in it,
// whether or not the SDK holds the object in a pointer.
func TestRequestsSendTheInlinedObject(t *testing.T) {
	ctx := context.Background()
	m := &UnionWrapThingModel{
		Name:        types.StringValue("n"),
		Description: types.StringNull(),
		Kind:        &UnionWrapThingKindModel{ArmB: &ArmBModel{Count: types.Int32Value(3)}},
	}
	create, diags := expandCreate(ctx, m)
	if diags.HasError() {
		t.Fatal(diags)
	}
	update, diags := expandUpdate(ctx, m)
	if diags.HasError() {
		t.Fatal(diags)
	}
	for name, thing := range map[string]*union_wrap_things_service.Thing{"create": create.Thing, "update": update.Thing} {
		if thing == nil || thing.Name != "n" {
			t.Fatalf("%s thing = %+v, want name n", name, thing)
		}
		if thing.Description != nil {
			t.Fatalf("%s description = %v, want nil", name, *thing.Description)
		}
		if thing.ArmB == nil || thing.ArmB.Count == nil || *thing.ArmB.Count != 3 || thing.ArmA != nil || thing.ArmC != nil {
			t.Fatalf("%s arms = %+v %+v %+v, want only arm b with count 3", name, thing.ArmA, thing.ArmB, thing.ArmC)
		}
	}
}

// A response reads the fields and the arm of the object as top-level attributes.
func TestResponseFlattensTheInlinedObject(t *testing.T) {
	id, description := "thing-1", "d"
	got, diags := flatten(context.Background(), &union_wrap_things_service.UnionWrapThing{
		Id:    &id,
		Thing: &union_wrap_things_service.Thing{Name: "n", Description: &description, ArmA: &union_wrap_things_service.ArmA{Value: "v"}},
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	if got.Id.ValueString() != "thing-1" || got.Name.ValueString() != "n" || got.Description.ValueString() != "d" {
		t.Fatalf("model = %+v, want id thing-1, name n, description d", got)
	}
	if got.Kind == nil || got.Kind.ArmA == nil || got.Kind.ArmA.Value.ValueString() != "v" || got.Kind.ArmB != nil || got.Kind.ArmC != nil {
		t.Fatalf("kind = %+v, want only arm a with value v", got.Kind)
	}
}

// The API always has the object, so a response without it has no resource.
func TestResponseWithoutTheInlinedObjectIsAnError(t *testing.T) {
	id := "thing-1"
	_, diags := flatten(context.Background(), &union_wrap_things_service.UnionWrapThing{Id: &id})
	if !diags.HasError() || !strings.Contains(diags[0].Detail(), "no thing") {
		t.Fatalf("diagnostics = %v, want an error about the missing thing", diags)
	}
}

// The variant of the API follows the arm that is set, in both requests: the arms are in a group of the
// resource, so the line reaches them through it. The response variant is ignored on read.
func TestVariantFollowsTheArmThatIsSet(t *testing.T) {
	ctx := context.Background()
	for name, test := range map[string]struct {
		kind *UnionWrapThingKindModel
		want union_wrap_things_service.ThingVariant
	}{
		"arm a": {&UnionWrapThingKindModel{ArmA: &ArmAModel{Value: types.StringValue("v")}}, union_wrap_things_service.THINGVARIANT_THING_VARIANT_A_OR_UNSPECIFIED},
		"arm b": {&UnionWrapThingKindModel{ArmB: &ArmBModel{Count: types.Int32Value(1)}}, union_wrap_things_service.THINGVARIANT_THING_VARIANT_B},
		"arm c": {&UnionWrapThingKindModel{ArmC: &ArmCModel{Label: types.StringValue("l")}}, union_wrap_things_service.THINGVARIANT_THING_VARIANT_C},
	} {
		t.Run(name, func(t *testing.T) {
			m := &UnionWrapThingModel{Name: types.StringValue("n"), Description: types.StringNull(), Kind: test.kind}
			create, diags := expandCreate(ctx, m)
			if diags.HasError() {
				t.Fatal(diags)
			}
			update, diags := expandUpdate(ctx, m)
			if diags.HasError() {
				t.Fatal(diags)
			}
			for request, thing := range map[string]*union_wrap_things_service.Thing{"create": create.Thing, "update": update.Thing} {
				if thing.Variant == nil || *thing.Variant != test.want {
					t.Errorf("%s variant = %v, want %s", request, thing.Variant, test.want)
				}
			}
		})
	}
	// No arm sends no variant, and a response with another variant reads the same.
	none, diags := expandCreate(ctx, &UnionWrapThingModel{Name: types.StringValue("n"), Description: types.StringNull(), Kind: &UnionWrapThingKindModel{}})
	if diags.HasError() || none.Thing.Variant != nil {
		t.Fatalf("variant without an arm = %v (%v), want none", none.Thing.Variant, diags)
	}
	variant := union_wrap_things_service.THINGVARIANT_THING_VARIANT_C
	id := "thing-1"
	got, diags := flatten(ctx, &union_wrap_things_service.UnionWrapThing{Id: &id, Thing: &union_wrap_things_service.Thing{Name: "n", Variant: &variant, ArmA: &union_wrap_things_service.ArmA{Value: "v"}}})
	if diags.HasError() || got.Kind == nil || got.Kind.ArmA == nil {
		t.Fatalf("flatten = %+v (%v), want the arm of the response, not the variant", got, diags)
	}
}

func itemList(t *testing.T, items ...UnionItemModel) types.List {
	t.Helper()
	for i := range items {
		if items[i].Choices.IsNull() {
			// A list that the test leaves out has no element type yet.
			items[i].Choices = types.ListNull(types.ObjectType{AttrTypes: pickAttrTypes()})
		}
		if items[i].Reminders.IsNull() && items[i].Reminders.ElementType(context.Background()) == nil {
			items[i].Reminders = types.ListNull(types.ObjectType{AttrTypes: unionReminderAttrTypes()})
		}
	}
	list, diags := types.ListValueFrom(context.Background(), types.ObjectType{AttrTypes: unionItemAttrTypes()}, items)
	if diags.HasError() {
		t.Fatal(diags)
	}
	return list
}

func sentItems(t *testing.T, items ...UnionItemModel) []union_wrap_things_service.UnionItem {
	t.Helper()
	m := &UnionWrapThingModel{Name: types.StringValue("n"), Kind: &UnionWrapThingKindModel{ArmC: &ArmCModel{Label: types.StringValue("l")}}, Items: itemList(t, items...)}
	for name, expand := range map[string]func(context.Context, *UnionWrapThingModel) ([]union_wrap_things_service.UnionItem, diagsOf){
		"create": func(ctx context.Context, m *UnionWrapThingModel) ([]union_wrap_things_service.UnionItem, diagsOf) {
			r, d := expandCreate(ctx, m)
			return r.Thing.Items, d
		},
		"update": func(ctx context.Context, m *UnionWrapThingModel) ([]union_wrap_things_service.UnionItem, diagsOf) {
			r, d := expandUpdate(ctx, m)
			return r.Thing.Items, d
		},
	} {
		if got, diags := expand(context.Background(), m); diags.HasError() {
			t.Fatalf("%s: %v", name, diags)
		} else if name == "update" {
			return got
		}
	}
	return nil
}

// An object that the contract does not require is sent only when an attribute of it is set. The
// inlined fields of three objects are attributes of the item: note (optional object), value and
// label (an object that has a field named value), and title and collapsed (an arm of a oneOf).
func TestOptionalInlinedObjectIsSentOnlyWhenAFieldIsSet(t *testing.T) {
	null := types.StringNull()
	got := sentItems(t,
		UnionItemModel{Name: types.StringValue("none"), Note: null, Value: null, Label: null},
		UnionItemModel{Name: types.StringValue("note"), Note: types.StringValue("n"), Value: null, Label: null},
		UnionItemModel{Name: types.StringValue("value"), Note: null, Value: types.StringValue("v"), Label: null},
		UnionItemModel{Name: types.StringValue("label"), Note: null, Value: null, Label: types.StringValue("l")},
	)
	if len(got) != 4 {
		t.Fatalf("sent %d items, want 4", len(got))
	}
	if got[0].Extra != nil || got[0].Value != nil || got[0].Options != nil {
		t.Errorf("item none = %+v, want no object", got[0])
	}
	if got[1].Extra == nil || *got[1].Extra.Note != "n" || got[1].Value != nil {
		t.Errorf("item note = %+v, want only extra", got[1])
	}
	if got[2].Value == nil || *got[2].Value.Value != "v" || got[2].Value.Label != nil || got[2].Extra != nil {
		t.Errorf("item value = %+v, want only the value of the object", got[2])
	}
	if got[3].Value == nil || got[3].Value.Value != nil || *got[3].Value.Label != "l" {
		t.Errorf("item label = %+v, want only the label of the object", got[3])
	}
}

// The arm that an inline line moves up is sent when one of its attributes is set. The other arm is
// sent as it is. Neither set sends no options.
func TestInlinedArmAndKeptArmAreSentSeparately(t *testing.T) {
	null := types.StringNull()
	got := sentItems(t,
		UnionItemModel{Name: types.StringValue("custom"), Note: null, Value: null, Label: null, Options: &ItemOptionsModel{Title: types.StringValue("t"), Collapsed: types.BoolNull()}},
		UnionItemModel{Name: types.StringValue("internal"), Note: null, Value: null, Label: null, Options: &ItemOptionsModel{Title: null, Collapsed: types.BoolNull(), Internal: &InternalOptionsModel{}}},
		UnionItemModel{Name: types.StringValue("neither"), Note: null, Value: null, Label: null, Options: &ItemOptionsModel{Title: null, Collapsed: types.BoolNull()}},
	)
	if got[0].Options == nil || got[0].Options.Custom == nil || *got[0].Options.Custom.Title != "t" || got[0].Options.Internal != nil {
		t.Errorf("custom options = %+v, want only custom", got[0].Options)
	}
	if got[1].Options == nil || got[1].Options.Internal == nil || got[1].Options.Custom != nil {
		t.Errorf("internal options = %+v, want only internal", got[1].Options)
	}
	if got[2].Options == nil || got[2].Options.Custom != nil || got[2].Options.Internal != nil {
		t.Errorf("empty options = %+v, want an empty oneOf", got[2].Options)
	}
}

// A response without an object reads its attributes as null, also after an import, so the second
// plan has nothing to change. An object that the API returns reads as its values.
func TestInlinedObjectsReadAsNullWhenTheAPIHasNone(t *testing.T) {
	ctx := context.Background()
	id, note, v, l, title := "thing-1", "n", "v", "l", "t"
	read := func(items ...union_wrap_things_service.UnionItem) []UnionItemModel {
		got, diags := flatten(ctx, &union_wrap_things_service.UnionWrapThing{Id: &id, Thing: &union_wrap_things_service.Thing{Name: "n", ArmC: &union_wrap_things_service.ArmC{}, Items: items}})
		if diags.HasError() {
			t.Fatal(diags)
		}
		var out []UnionItemModel
		if diags := got.Items.ElementsAs(ctx, &out, false); diags.HasError() {
			t.Fatal(diags)
		}
		return out
	}
	got := read(
		union_wrap_things_service.UnionItem{Name: "bare"},
		union_wrap_things_service.UnionItem{Name: "full", Extra: &union_wrap_things_service.ItemExtra{Note: &note}, Value: &union_wrap_things_service.Labelled{Value: &v, Label: &l},
			Options: &union_wrap_things_service.ItemOptions{Custom: &union_wrap_things_service.CustomOptions{Title: &title}}},
		union_wrap_things_service.UnionItem{Name: "internal", Options: &union_wrap_things_service.ItemOptions{Internal: map[string]interface{}{}}},
		union_wrap_things_service.UnionItem{Name: "empty", Options: &union_wrap_things_service.ItemOptions{}},
	)
	if !got[0].Note.IsNull() || !got[0].Value.IsNull() || !got[0].Label.IsNull() || got[0].Options != nil {
		t.Errorf("bare item = %+v, want null attributes", got[0])
	}
	if got[1].Note.ValueString() != "n" || got[1].Value.ValueString() != "v" || got[1].Label.ValueString() != "l" || got[1].Options.Title.ValueString() != "t" || got[1].Options.Internal != nil {
		t.Errorf("full item = %+v, want the values", got[1])
	}
	if got[2].Options == nil || got[2].Options.Internal == nil || !got[2].Options.Title.IsNull() {
		t.Errorf("internal item options = %+v, want only internal", got[2].Options)
	}
	if got[3].Options == nil || got[3].Options.Internal != nil || !got[3].Options.Title.IsNull() || !got[3].Options.Collapsed.IsNull() {
		t.Errorf("empty item options = %+v, want no value", got[3].Options)
	}
	// The expand of what was read sends what the API had: the round trip changes nothing.
	again := sentItems(t, got...)
	if again[1].Extra == nil || *again[1].Extra.Note != "n" || again[2].Options.Internal == nil || again[0].Extra != nil {
		t.Errorf("sent again = %+v", again)
	}
}

type diagsOf = interface{ HasError() bool }

type testProvider struct{}

func (testProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "test"
}
func (testProvider) Schema(context.Context, provider.SchemaRequest, *provider.SchemaResponse) {}
func (testProvider) Configure(context.Context, provider.ConfigureRequest, *provider.ConfigureResponse) {
}
func (testProvider) Resources(context.Context) []func() frameworkresource.Resource {
	return []func() frameworkresource.Resource{func() frameworkresource.Resource { return NewResource(Hooks{}) }}
}
func (testProvider) DataSources(context.Context) []func() datasource.DataSource { return nil }

// The fields of the inlined arm conflict with the arm that stays: a configuration that sets both is
// an error at plan, as the released resource said.
func TestInlinedArmConflictsWithTheKeptArm(t *testing.T) {
	ctx := context.Background()
	server, err := providerserver.NewProtocol6WithError(testProvider{})()
	if err != nil {
		t.Fatal(err)
	}
	schemaResp, err := server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	objectType := schemaResp.ResourceSchemas["test_union_wrap_thing"].ValueType().(tftypes.Object)
	itemType := objectType.AttributeTypes["items"].(tftypes.List).ElementType.(tftypes.Object)
	optionsType := itemType.AttributeTypes["options"].(tftypes.Object)
	internalType := optionsType.AttributeTypes["internal"]
	options := func(title tftypes.Value, internal bool) tftypes.Value {
		internalValue := tftypes.NewValue(internalType, nil)
		if internal {
			internalValue = tftypes.NewValue(internalType, map[string]tftypes.Value{})
		}
		return tftypes.NewValue(optionsType, map[string]tftypes.Value{"title": title, "collapsed": tftypes.NewValue(tftypes.Bool, nil), "internal": internalValue})
	}
	config := func(opts tftypes.Value) tftypes.Value {
		values := map[string]tftypes.Value{}
		for name, typ := range objectType.AttributeTypes {
			values[name] = tftypes.NewValue(typ, nil)
		}
		nullString := tftypes.NewValue(tftypes.String, nil)
		item := tftypes.NewValue(itemType, map[string]tftypes.Value{"name": tftypes.NewValue(tftypes.String, "i"), "note": nullString, "value": nullString, "label": nullString, "options": opts,
			"first": tftypes.NewValue(itemType.AttributeTypes["first"], nil), "second": tftypes.NewValue(itemType.AttributeTypes["second"], nil),
			"choice": tftypes.NewValue(itemType.AttributeTypes["choice"], nil), "choices": tftypes.NewValue(itemType.AttributeTypes["choices"], nil),
			"action": tftypes.NewValue(itemType.AttributeTypes["action"], nil), "behavior": tftypes.NewValue(itemType.AttributeTypes["behavior"], nil), "reminders": tftypes.NewValue(itemType.AttributeTypes["reminders"], nil)})
		values["name"] = tftypes.NewValue(tftypes.String, "n")
		kindType := objectType.AttributeTypes["kind"].(tftypes.Object)
		armValues := map[string]tftypes.Value{}
		for arm, typ := range kindType.AttributeTypes {
			armValues[arm] = tftypes.NewValue(typ, nil)
		}
		armCType := kindType.AttributeTypes["arm_c"].(tftypes.Object)
		armValues["arm_c"] = tftypes.NewValue(armCType, map[string]tftypes.Value{"label": tftypes.NewValue(tftypes.String, "l")})
		values["kind"] = tftypes.NewValue(kindType, armValues)
		values["items"] = tftypes.NewValue(tftypes.List{ElementType: itemType}, []tftypes.Value{item})
		return tftypes.NewValue(objectType, values)
	}
	title := tftypes.NewValue(tftypes.String, "t")
	none := tftypes.NewValue(tftypes.String, nil)
	for name, test := range map[string]struct {
		opts tftypes.Value
		bad  bool
	}{
		"custom only":   {options(title, false), false},
		"internal only": {options(none, true), false},
		"both":          {options(title, true), true},
	} {
		dv, err := tfprotov6.NewDynamicValue(objectType, config(test.opts))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := server.ValidateResourceConfig(ctx, &tfprotov6.ValidateResourceConfigRequest{TypeName: "test_union_wrap_thing", Config: &dv})
		if err != nil {
			t.Fatal(err)
		}
		var got string
		for _, d := range resp.Diagnostics {
			if d.Severity == tfprotov6.DiagnosticSeverityError {
				got += d.Summary + " " + d.Detail
			}
		}
		if (got != "") != test.bad {
			t.Errorf("%s: errors %q, want an error: %v", name, got, test.bad)
		}
	}
}

// One component in two places: the label of the first place is title, the label of the second is
// label. Each is sent to the label of the API, and read back from it, in its own place.
func TestOneComponentHasAnotherNameInEachPlace(t *testing.T) {
	ctx := context.Background()
	null := types.StringNull()
	got := sentItems(t, UnionItemModel{
		Name: types.StringValue("i"), Note: null, Value: null, Label: null,
		First:  &TagAtUnionItemFirstModel{Label: types.StringValue("one")},
		Second: &TagModel{Label: types.StringValue("two")},
	}, UnionItemModel{Name: types.StringValue("none"), Note: null, Value: null, Label: null})
	if got[0].First == nil || *got[0].First.Label != "one" || got[0].Second == nil || *got[0].Second.Label != "two" {
		t.Fatalf("sent = %+v %+v, want one in the first place and two in the second", got[0].First, got[0].Second)
	}
	if got[1].First != nil || got[1].Second != nil {
		t.Fatalf("an item without tags sent %+v %+v", got[1].First, got[1].Second)
	}
	id, one, two := "thing-1", "one", "two"
	flat, diags := flatten(ctx, &union_wrap_things_service.UnionWrapThing{Id: &id, Thing: &union_wrap_things_service.Thing{
		Name: "n", ArmC: &union_wrap_things_service.ArmC{},
		Items: []union_wrap_things_service.UnionItem{{Name: "i", First: &union_wrap_things_service.Tag{Label: &one}, Second: &union_wrap_things_service.Tag{Label: &two}}, {Name: "none"}},
	}})
	if diags.HasError() {
		t.Fatal(diags)
	}
	var items []UnionItemModel
	if diags := flat.Items.ElementsAs(ctx, &items, false); diags.HasError() {
		t.Fatal(diags)
	}
	if items[0].First == nil || items[0].First.Label.ValueString() != "one" || items[0].Second == nil || items[0].Second.Label.ValueString() != "two" {
		t.Fatalf("read = %+v %+v, want one and two", items[0].First, items[0].Second)
	}
	if items[1].First != nil || items[1].Second != nil {
		t.Fatalf("an item without tags read %+v %+v, want null objects", items[1].First, items[1].Second)
	}
}

func pickList(t *testing.T, picks ...PickModel) types.List {
	t.Helper()
	list, diags := types.ListValueFrom(context.Background(), types.ObjectType{AttrTypes: pickAttrTypes()}, picks)
	if diags.HasError() {
		t.Fatal(diags)
	}
	return list
}

// One component, two places. The single place inlines the value: its fields and its note (named
// remark there) are attributes of the place. The list keeps the object. Each is sent in the API
// object that its place holds.
func TestOnePlaceInlinesTheValueAndTheListKeepsIt(t *testing.T) {
	null := types.StringNull()
	got := sentItems(t,
		UnionItemModel{Name: types.StringValue("both"), Note: null, Value: null, Label: null,
			Choice: &PickAtUnionItemChoiceModel{Value: types.StringValue("v"), Label: types.StringValue("l"), Note: types.StringValue("n")},
			Choices: pickList(t,
				PickModel{Value: &LabelledModel{Value: types.StringValue("lv"), Label: types.StringValue("ll")}, Note: types.StringValue("ln")},
				PickModel{Note: types.StringValue("only a note")})},
		UnionItemModel{Name: types.StringValue("note"), Note: null, Value: null, Label: null,
			Choice: &PickAtUnionItemChoiceModel{Value: null, Label: null, Note: types.StringValue("n")}},
		UnionItemModel{Name: types.StringValue("none"), Note: null, Value: null, Label: null},
	)
	choice := got[0].Choice
	if choice == nil || choice.Value == nil || *choice.Value.Value != "v" || *choice.Value.Label != "l" || *choice.Note != "n" {
		t.Fatalf("choice = %+v, want the value inside its object, and the note", choice)
	}
	list := got[0].Choices
	if len(list) != 2 || list[0].Value == nil || *list[0].Value.Value != "lv" || *list[0].Value.Label != "ll" || *list[0].Note != "ln" {
		t.Fatalf("choices = %+v, want the objects of the list kept", list)
	}
	if list[1].Value != nil || *list[1].Note != "only a note" {
		t.Fatalf("second choice = %+v, want no value object", list[1])
	}
	// The object of the inlined place is sent only when one of its attributes is set.
	if got[1].Choice == nil || got[1].Choice.Value != nil || *got[1].Choice.Note != "n" {
		t.Fatalf("choice with a note only = %+v, want no value object", got[1].Choice)
	}
	if got[2].Choice != nil || got[2].Choices != nil {
		t.Fatalf("item without choices sent %+v %+v", got[2].Choice, got[2].Choices)
	}
}

// A read gives each place its own shape, also an import, which has only the id: the inlined place
// reads null attributes when the API has no object, and what was read is what the next plan sees,
// so the expand of the state sends what the API had.
func TestPlacesReadBackInTheirOwnShape(t *testing.T) {
	ctx := context.Background()
	v, l, n, lv := "v", "l", "n", "lv"
	id := "thing-1"
	union_wrap_things_service.Stored = union_wrap_things_service.UnionWrapThing{Id: &id, Thing: &union_wrap_things_service.Thing{
		Name: "n", ArmC: &union_wrap_things_service.ArmC{},
		Items: []union_wrap_things_service.UnionItem{
			{Name: "full", Choice: &union_wrap_things_service.Pick{Value: &union_wrap_things_service.Labelled{Value: &v, Label: &l}, Note: &n},
				Choices: []union_wrap_things_service.Pick{{Value: &union_wrap_things_service.Labelled{Value: &lv}}}},
			{Name: "no object", Choice: &union_wrap_things_service.Pick{Note: &n}},
			{Name: "bare"},
		},
	}}
	r := &Resource{client: &union_wrap_things_service.UnionWrapThingsServiceAPIService{}}
	var schemaResp frameworkresource.SchemaResponse
	r.Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResp)
	root := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	values := map[string]tftypes.Value{}
	for name, typ := range root.AttributeTypes {
		values[name] = tftypes.NewValue(typ, nil)
	}
	values["id"] = tftypes.NewValue(tftypes.String, id)
	state := tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(root, values)}
	resp := &frameworkresource.ReadResponse{State: state}
	r.Read(ctx, frameworkresource.ReadRequest{State: state}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	var model UnionWrapThingModel
	if diags := resp.State.Get(ctx, &model); diags.HasError() {
		t.Fatal(diags)
	}
	var items []UnionItemModel
	if diags := model.Items.ElementsAs(ctx, &items, false); diags.HasError() {
		t.Fatal(diags)
	}
	if c := items[0].Choice; c == nil || c.Value.ValueString() != "v" || c.Label.ValueString() != "l" || c.Note.ValueString() != "n" {
		t.Fatalf("choice = %+v, want v, l, and the note n", c)
	}
	var picks []PickModel
	if diags := items[0].Choices.ElementsAs(ctx, &picks, false); diags.HasError() {
		t.Fatal(diags)
	}
	if len(picks) != 1 || picks[0].Value == nil || picks[0].Value.Value.ValueString() != "lv" || !picks[0].Value.Label.IsNull() || !picks[0].Note.IsNull() {
		t.Fatalf("choices = %+v, want the object of the list kept", picks)
	}
	if c := items[1].Choice; c == nil || !c.Value.IsNull() || !c.Label.IsNull() || c.Note.ValueString() != "n" {
		t.Fatalf("choice without an object = %+v, want null value and label", c)
	}
	if items[2].Choice != nil || !items[2].Choices.IsNull() {
		t.Fatalf("bare item = %+v %v, want null choice and choices", items[2].Choice, items[2].Choices)
	}
	// The second plan: what was read, sent again, is what the API had.
	again := sentItems(t, items...)
	if again[0].Choice.Value == nil || *again[0].Choice.Value.Value != "v" || again[0].Choices[0].Value == nil || again[1].Choice.Value != nil || again[2].Choice != nil {
		t.Fatalf("sent again = %+v %+v %+v", again[0], again[1], again[2])
	}
}

// A definition is a oneOf of a link and a target. The file inlines the link, whose line says that the
// API always has it, and skips the target. Create and Update send the link object when the item
// holds a definition, also when none of its attributes is set. The target stays unset.
func TestInlinedArmWithRequiredIsAlwaysSent(t *testing.T) {
	null := types.StringNull()
	got := sentItems(t,
		UnionItemModel{Name: types.StringValue("url"), Note: null, Value: null, Label: null, Behavior: &ActionDefinitionModel{Url: types.StringValue("https://example.com")}},
		UnionItemModel{Name: types.StringValue("empty"), Note: null, Value: null, Label: null, Behavior: &ActionDefinitionModel{Url: null}},
		UnionItemModel{Name: types.StringValue("none"), Note: null, Value: null, Label: null},
	)
	if b := got[0].Behavior; b == nil || b.Link == nil || b.Link.Url == nil || *b.Link.Url != "https://example.com" || b.Target != nil {
		t.Errorf("behavior with a url = %+v, want only the link, with the url", got[0].Behavior)
	}
	if b := got[1].Behavior; b == nil || b.Link == nil || b.Link.Url != nil || b.Target != nil {
		t.Errorf("behavior without a url = %+v, want an empty link", got[1].Behavior)
	}
	if got[2].Behavior != nil {
		t.Errorf("item without a behavior sent %+v", got[2].Behavior)
	}
}

// The action inlines its definition too, so the url is an attribute of the action. The request
// holds it in the link of the definition. The definition of an action without a url is left out,
// as for any optional object.
func TestUrlOfAnActionIsSentInTheLinkOfItsDefinition(t *testing.T) {
	null := types.StringNull()
	got := sentItems(t,
		UnionItemModel{Name: types.StringValue("url"), Note: null, Value: null, Label: null, Action: &ItemActionModel{Label: types.StringValue("l"), Url: types.StringValue("https://example.com")}},
		UnionItemModel{Name: types.StringValue("label"), Note: null, Value: null, Label: null, Action: &ItemActionModel{Label: types.StringValue("l"), Url: null}},
	)
	a := got[0].Action
	if a == nil || *a.Label != "l" || a.Definition == nil || a.Definition.Link == nil || *a.Definition.Link.Url != "https://example.com" || a.Definition.Target != nil {
		t.Errorf("action with a url = %+v, want the url in the link of the definition", a)
	}
	if a := got[1].Action; a == nil || *a.Label != "l" || a.Definition != nil {
		t.Errorf("action without a url = %+v, want no definition", a)
	}
}

// A response reads the url from the link, in both places. A response without a definition, or
// with the other arm, reads a null url.
func TestUrlIsReadFromTheLink(t *testing.T) {
	ctx := context.Background()
	id, url, label := "thing-1", "https://example.com", "l"
	page := "d"
	got, diags := flatten(ctx, &union_wrap_things_service.UnionWrapThing{Id: &id, Thing: &union_wrap_things_service.Thing{
		Name: "n", ArmC: &union_wrap_things_service.ArmC{},
		Items: []union_wrap_things_service.UnionItem{
			{Name: "link", Behavior: &union_wrap_things_service.ActionDefinition{Link: &union_wrap_things_service.ActionLink{Url: &url}},
				Action: &union_wrap_things_service.ItemAction{Label: &label, Definition: &union_wrap_things_service.ActionDefinition{Link: &union_wrap_things_service.ActionLink{Url: &url}}}},
			{Name: "target", Behavior: &union_wrap_things_service.ActionDefinition{Target: &union_wrap_things_service.ActionTarget{PageId: &page}},
				Action: &union_wrap_things_service.ItemAction{Label: &label}},
			{Name: "none"},
		},
	}})
	if diags.HasError() {
		t.Fatal(diags)
	}
	var items []UnionItemModel
	if diags := got.Items.ElementsAs(ctx, &items, false); diags.HasError() {
		t.Fatal(diags)
	}
	if items[0].Behavior == nil || items[0].Behavior.Url.ValueString() != url || items[0].Action == nil || items[0].Action.Url.ValueString() != url || items[0].Action.Label.ValueString() != "l" {
		t.Errorf("link item = %+v %+v, want the url in both places", items[0].Behavior, items[0].Action)
	}
	if items[1].Behavior == nil || !items[1].Behavior.Url.IsNull() || items[1].Action == nil || !items[1].Action.Url.IsNull() {
		t.Errorf("target item = %+v %+v, want a null url", items[1].Behavior, items[1].Action)
	}
	if items[2].Behavior != nil || items[2].Action != nil {
		t.Errorf("bare item = %+v %+v, want null objects", items[2].Behavior, items[2].Action)
	}
}

// An import reads the resource with only the id. The url comes back from the link, and the second
// plan sees what the API had: expand sends the same link again.
func TestUrlReadsBackAfterImportAndSendsTheSameLink(t *testing.T) {
	ctx := context.Background()
	id, url, label := "thing-1", "https://example.com", "l"
	union_wrap_things_service.Stored = union_wrap_things_service.UnionWrapThing{Id: &id, Thing: &union_wrap_things_service.Thing{
		Name: "n", ArmC: &union_wrap_things_service.ArmC{},
		Items: []union_wrap_things_service.UnionItem{{Name: "i",
			Behavior: &union_wrap_things_service.ActionDefinition{Link: &union_wrap_things_service.ActionLink{Url: &url}},
			Action:   &union_wrap_things_service.ItemAction{Label: &label, Definition: &union_wrap_things_service.ActionDefinition{Link: &union_wrap_things_service.ActionLink{Url: &url}}}}},
	}}
	r := &Resource{client: &union_wrap_things_service.UnionWrapThingsServiceAPIService{}}
	var schemaResp frameworkresource.SchemaResponse
	r.Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResp)
	root := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	values := map[string]tftypes.Value{}
	for name, typ := range root.AttributeTypes {
		values[name] = tftypes.NewValue(typ, nil)
	}
	values["id"] = tftypes.NewValue(tftypes.String, id)
	state := tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(root, values)}
	resp := &frameworkresource.ReadResponse{State: state}
	r.Read(ctx, frameworkresource.ReadRequest{State: state}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	var model UnionWrapThingModel
	if diags := resp.State.Get(ctx, &model); diags.HasError() {
		t.Fatal(diags)
	}
	var items []UnionItemModel
	if diags := model.Items.ElementsAs(ctx, &items, false); diags.HasError() {
		t.Fatal(diags)
	}
	if items[0].Behavior == nil || items[0].Behavior.Url.ValueString() != url || items[0].Action == nil || items[0].Action.Url.ValueString() != url {
		t.Fatalf("read = %+v %+v, want the url in both places", items[0].Behavior, items[0].Action)
	}
	again := sentItems(t, items...)
	b, a := again[0].Behavior, again[0].Action
	if b == nil || b.Link == nil || *b.Link.Url != url || b.Target != nil || a == nil || a.Definition == nil || a.Definition.Link == nil || *a.Definition.Link.Url != url {
		t.Fatalf("sent again = %+v %+v, want the same link", b, a)
	}
}

func reminderList(t *testing.T, reminders ...UnionReminderModel) types.List {
	t.Helper()
	list, diags := types.ListValueFrom(context.Background(), types.ObjectType{AttrTypes: unionReminderAttrTypes()}, reminders)
	if diags.HasError() {
		t.Fatal(diags)
	}
	return list
}

func reminderOf(label, url string) UnionReminderModel {
	return UnionReminderModel{Label: types.StringValue(label), Url: types.StringValue(url)}
}

func itemWithReminders(name string, reminders types.List) UnionItemModel {
	null := types.StringNull()
	return UnionItemModel{Name: types.StringValue(name), Note: null, Value: null, Label: null, Reminders: reminders}
}

// The reminders are one list of the thing in the API, and each one names its item by the name of the
// item. Create and Update send the reminders of every item in that list, in the order of the items,
// with the link inside the definition, and leave the other kind of definition unset. An item without
// reminders adds none.
func TestRemindersAreSentInOneListWithTheNameOfTheirItem(t *testing.T) {
	ctx := context.Background()
	m := &UnionWrapThingModel{Name: types.StringValue("n"), Kind: &UnionWrapThingKindModel{ArmC: &ArmCModel{Label: types.StringValue("l")}}, Items: itemList(t,
		itemWithReminders("i1", reminderList(t, reminderOf("a", "u1"), reminderOf("b", "u2"))),
		itemWithReminders("i2", reminderList(t, reminderOf("c", "u3"))),
		itemWithReminders("i3", types.ListNull(types.ObjectType{AttrTypes: unionReminderAttrTypes()})),
	)}
	create, diags := expandCreate(ctx, m)
	if diags.HasError() {
		t.Fatal(diags)
	}
	update, diags := expandUpdate(ctx, m)
	if diags.HasError() {
		t.Fatal(diags)
	}
	for name, got := range map[string][]union_wrap_things_service.UnionReminder{"create": create.Thing.Reminders, "update": update.Thing.Reminders} {
		want := []struct{ item, label, url string }{{"i1", "a", "u1"}, {"i1", "b", "u2"}, {"i2", "c", "u3"}}
		if len(got) != len(want) {
			t.Fatalf("%s sent %d reminders, want %d: %+v", name, len(got), len(want), got)
		}
		for i, w := range want {
			r := got[i]
			if r.ItemName != w.item || *r.Label != w.label || r.Definition == nil || r.Definition.Link == nil || *r.Definition.Link.Url != w.url || r.Definition.Target != nil {
				t.Errorf("%s reminder %d = %+v, want item %s, label %s, the link %s", name, i, r, w.item, w.label, w.url)
			}
		}
	}
}

func remindersOf(t *testing.T, item UnionItemModel) []UnionReminderModel {
	t.Helper()
	var out []UnionReminderModel
	if item.Reminders.IsNull() {
		return nil
	}
	if diags := item.Reminders.ElementsAs(context.Background(), &out, false); diags.HasError() {
		t.Fatal(diags)
	}
	return out
}

// The definition of a reminder is an optional object, so a reminder without a url sends no link. A
// response would not show it (only reminders with a link are shown), so the write would lose it.
// Expand refuses it with an error on the reminder and sends nothing; a reminder with a url is sent.
func TestReminderThatAResponseWouldNotShowIsRefused(t *testing.T) {
	m := &UnionWrapThingModel{Name: types.StringValue("n"), Kind: &UnionWrapThingKindModel{ArmC: &ArmCModel{Label: types.StringValue("l")}}, Items: itemList(t,
		itemWithReminders("i1", reminderList(t, reminderOf("a", "u1"), UnionReminderModel{Label: types.StringValue("b"), Url: types.StringNull()})),
	)}
	for call, expand := range map[string]func() diag.Diagnostics{
		"create": func() diag.Diagnostics { _, d := expandCreate(context.Background(), m); return d },
		"update": func() diag.Diagnostics { _, d := expandUpdate(context.Background(), m); return d },
	} {
		diags := expand()
		found := false
		for _, d := range diags.Errors() {
			if withPath, ok := d.(diag.DiagnosticWithPath); ok && withPath.Path().Equal(path.Root("items").AtListIndex(0).AtName("reminders").AtListIndex(1)) && strings.Contains(d.Detail(), "definition.link") {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: the errors %v have none for items[0].reminders[1] that names definition.link", call, diags.Errors())
		}
	}
}

// Flatten gives each item the reminders that name it, in the order of the list. A reminder whose
// definition has no link (the other kind, or none) is in no item, and a reminder that names no item
// is lost, as for any list that is shown inside the items.
func TestOnlyRemindersWithALinkAreShownInTheirItem(t *testing.T) {
	ctx := context.Background()
	id, a, b, c := "thing-1", "u1", "u2", "page"
	link := func(url *string) *union_wrap_things_service.ActionDefinition {
		return &union_wrap_things_service.ActionDefinition{Link: &union_wrap_things_service.ActionLink{Url: url}}
	}
	got, diags := flatten(ctx, &union_wrap_things_service.UnionWrapThing{Id: &id, Thing: &union_wrap_things_service.Thing{
		Name: "n", ArmC: &union_wrap_things_service.ArmC{},
		Items: []union_wrap_things_service.UnionItem{{Name: "i1"}, {Name: "i2"}, {Name: "i3"}},
		Reminders: []union_wrap_things_service.UnionReminder{
			{ItemName: "i1", Definition: link(&a)},
			{ItemName: "i1", Definition: &union_wrap_things_service.ActionDefinition{Target: &union_wrap_things_service.ActionTarget{PageId: &c}}},
			{ItemName: "i2"},
			{ItemName: "i2", Definition: link(nil)},
			{ItemName: "i2", Definition: link(&b)},
			{ItemName: "nobody", Definition: link(&a)},
		},
	}})
	if diags.HasError() {
		t.Fatal(diags)
	}
	var items []UnionItemModel
	if diags := got.Items.ElementsAs(ctx, &items, false); diags.HasError() {
		t.Fatal(diags)
	}
	first, second := remindersOf(t, items[0]), remindersOf(t, items[1])
	if len(first) != 1 || first[0].Url.ValueString() != "u1" {
		t.Errorf("reminders of i1 = %+v, want only the link u1", first)
	}
	if len(second) != 2 || !second[0].Url.IsNull() || second[1].Url.ValueString() != "u2" {
		t.Errorf("reminders of i2 = %+v, want the empty link and the link u2", second)
	}
	if !items[2].Reminders.IsNull() {
		t.Errorf("reminders of i3 = %s, want null", items[2].Reminders)
	}
}

// An import reads the resource with only the id. Each item gets its reminders from the list of the
// thing, and the second plan sees what the API had: expand sends the same list again, and the
// reminder of the other kind is not in it, as it was not shown.
func TestRemindersReadBackAfterImportAndSendTheSameList(t *testing.T) {
	ctx := context.Background()
	id, a, b := "thing-1", "u1", "u2"
	link := func(url *string) *union_wrap_things_service.ActionDefinition {
		return &union_wrap_things_service.ActionDefinition{Link: &union_wrap_things_service.ActionLink{Url: url}}
	}
	union_wrap_things_service.Stored = union_wrap_things_service.UnionWrapThing{Id: &id, Thing: &union_wrap_things_service.Thing{
		Name: "n", ArmC: &union_wrap_things_service.ArmC{},
		Items: []union_wrap_things_service.UnionItem{{Name: "i1"}, {Name: "i2"}},
		Reminders: []union_wrap_things_service.UnionReminder{
			{ItemName: "i1", Definition: link(&a)},
			{ItemName: "i2", Definition: link(&b)},
		},
	}}
	r := &Resource{client: &union_wrap_things_service.UnionWrapThingsServiceAPIService{}}
	var schemaResp frameworkresource.SchemaResponse
	r.Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResp)
	root := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	values := map[string]tftypes.Value{}
	for name, typ := range root.AttributeTypes {
		values[name] = tftypes.NewValue(typ, nil)
	}
	values["id"] = tftypes.NewValue(tftypes.String, id)
	state := tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(root, values)}
	resp := &frameworkresource.ReadResponse{State: state}
	r.Read(ctx, frameworkresource.ReadRequest{State: state}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	var model UnionWrapThingModel
	if diags := resp.State.Get(ctx, &model); diags.HasError() {
		t.Fatal(diags)
	}
	var items []UnionItemModel
	if diags := model.Items.ElementsAs(ctx, &items, false); diags.HasError() {
		t.Fatal(diags)
	}
	if r1, r2 := remindersOf(t, items[0]), remindersOf(t, items[1]); len(r1) != 1 || r1[0].Url.ValueString() != "u1" || len(r2) != 1 || r2[0].Url.ValueString() != "u2" {
		t.Fatalf("read = %+v %+v, want one reminder in each item", r1, r2)
	}
	again, diags := expandUpdate(ctx, &model)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if got := again.Thing.Reminders; len(got) != 2 || got[0].ItemName != "i1" || *got[0].Definition.Link.Url != "u1" || got[1].ItemName != "i2" || *got[1].Definition.Link.Url != "u2" {
		t.Fatalf("sent again = %+v, want the two reminders", got)
	}
}
