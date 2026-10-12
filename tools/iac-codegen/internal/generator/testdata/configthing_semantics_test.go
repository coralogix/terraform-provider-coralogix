package configthing

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"example.com/iac-test-sdk/go/openapi/gen/config_things_service"
)

const (
	inlineYAML    = "receivers: {otlp: {protocols: [grpc, http]}}\n"
	multilineYAML = "receivers:\n  otlp:\n    protocols:\n      - grpc\n      - http\n"
	changedYAML   = "receivers:\n  otlp:\n    protocols:\n      - grpc\n"
	compactJSON   = `{"a":1,"b":[1,2]}`
	spacedJSON    = "{\n  \"b\": [1, 2],\n  \"a\": 1\n}"
)

// aliasBomb expands to 10^9 values through its aliases.
var aliasBomb = func() string {
	doc := "a0: &a0 [x, x, x, x, x, x, x, x, x, x]\n"
	for i := 1; i < 10; i++ {
		ref := fmt.Sprintf("*a%d", i-1)
		doc += fmt.Sprintf("a%d: &a%d [%s]\n", i, i, strings.Repeat(ref+", ", 9)+ref)
	}
	return doc
}()

// stagingMergeSent and stagingMergeReturned are a collector configuration with a merge key and the
// normalized text that the API returned for it: keys sorted, the merge key expanded, the anchor gone.
const (
	stagingMergeSent     = "receivers:\n  otlp:\n    protocols:\n      grpc: {}\nx-common: &common\n  timeout: 10s\nexporters:\n  otlphttp:\n    <<: *common\n    endpoint: https://a.example.com\nservice:\n  pipelines:\n    traces:\n      receivers: [otlp]\n      exporters: [otlphttp]\n"
	stagingMergeReturned = "exporters:\n  otlphttp:\n    endpoint: https://a.example.com\n    timeout: 10s\nreceivers:\n  otlp:\n    protocols:\n      grpc: {}\nservice:\n  pipelines:\n    traces:\n      exporters:\n      - otlphttp\n      receivers:\n      - otlp\nx-common:\n  timeout: 10s"
)

func TestYAMLEqual(t *testing.T) {
	tests := map[string]struct {
		a, b string
		want bool
	}{
		"inline and multiline":   {inlineYAML, multilineYAML, true},
		"key order":              {"a: 1\nb: 2\n", "b: 2\na: 1\n", true},
		"different value":        {inlineYAML, changedYAML, false},
		"different type":         {"a: 1", "a: '1'", false},
		"invalid":                {"a: [1", "a: [1]", false},
		"invalid itself":         {"a: [1", "a: [1", true},
		"empty":                  {"", "", true},
		"empty and document":     {"", "a: 1", false},
		"empty and blank":        {"", "\n", true},
		"beyond float64":         {"a: 9007199254740992", "a: 9007199254740993", false},
		"beyond int64":           {"a: 18446744073709551616", "a: 18446744073709551617", false},
		"same big integer":       {"a: 18446744073709551617", "{a: 18446744073709551617}", true},
		"decimal beyond":         {"a: 0.10000000000000000001", "a: 0.10000000000000000002", false},
		"same float forms":       {"a: 1.5", "a: 15e-1", true},
		"integer and float":      {"a: 1", "a: 1.0", false},
		"hex integer":            {"a: 0x10", "a: 16", true},
		"alias":                  {"x: &v [1]\ny: *v", "x: [1]\ny: [1]", true},
		"repeated key":           {"a: 1\na: 2", "a: 2", false},
		"number keys":            {"1: a\n2: b", "{2: b, 1: a}", true},
		"holds itself":           {"a: &n [*n]", "a: &n\n  - *n\n", false},
		"holds itself in a map":  {"a: &n {b: *n}", "a: &n\n  b: *n\n", false},
		"reused anchor":          {"x: &v {a: [1]}\ny: *v\nz: *v", "x: {a: [1]}\ny: {a: [1]}\nz: {a: [1]}", true},
		"alias expansion":        {aliasBomb, aliasBomb + "\n", false},
		"merge key":              {"d: &d {t: 1, r: 3}\ne:\n  <<: *d\n  p: a", "d: {t: 1, r: 3}\ne: {p: a, r: 3, t: 1}", true},
		"merge key own key wins": {"d: &d {t: 1}\ne:\n  t: 2\n  <<: *d", "d: {t: 1}\ne: {t: 2}", true},
		"merge key list order":   {"x: &x {t: 1}\ny: &y {t: 2, r: 3}\ne:\n  <<: [*x, *y]", "x: {t: 1}\ny: {t: 2, r: 3}\ne: {t: 1, r: 3}", true},
		"merge key inline map":   {"e:\n  <<: {t: 1}\n  p: a", "e: {t: 1, p: a}", true},
		"merge key nested merge": {"a: &a {t: 1}\nb: &b {<<: *a, r: 2}\ne: {<<: *b}", "a: {t: 1}\nb: {t: 1, r: 2}\ne: {t: 1, r: 2}", true},
		"merge key changed":      {"d: &d {t: 1}\ne: {<<: *d}", "d: {t: 1}\ne: {t: 2}", false},
		"merge key not a map":    {"d: &d [1]\ne: {<<: *d}", "d: [1]\ne: {}", false},
		"two merge keys":         {"d: &d {t: 1}\ne: {<<: *d, <<: *d}", "d: {t: 1}\ne: {t: 1}", false},
		"merge key holds itself": {"e: &e {<<: *e}", "e: {}", false},
		"quoted merge key":       {"e: {'<<': {t: 1}}", "e: {t: 1}", false},
		"staging normalization":  {stagingMergeSent, stagingMergeReturned, true},
	}
	for name, test := range tests {
		if got := yamlEqual(test.a, test.b); got != test.want {
			t.Errorf("%s: yamlEqual(%q, %q) = %v, want %v", name, test.a, test.b, got, test.want)
		}
	}
}

func TestJSONEqual(t *testing.T) {
	tests := map[string]struct {
		a, b string
		want bool
	}{
		"key order and whitespace": {compactJSON, spacedJSON, true},
		"different value":          {compactJSON, `{"a":1,"b":[2,1]}`, false},
		"different type":           {`{"a":1}`, `{"a":"1"}`, false},
		"invalid":                  {`{"a":`, `{"a":1}`, false},
		"invalid itself":           {`{"a":`, `{"a":`, true},
		"empty":                    {"", "", true},
		"empty and document":       {"", "{}", false},
		"beyond float64":           {`{"a":9007199254740992}`, `{"a":9007199254740993}`, false},
		"decimal beyond":           {`[0.10000000000000000001]`, `[0.10000000000000000002]`, false},
		"same number forms":        {`{"a":[1, 1.5]}`, `{"a":[1.0, 15e-1]}`, true},
		"trailing value":           {`{"a":1} {}`, `{"a":1}`, false},
	}
	for name, test := range tests {
		if got := jsonEqual(test.a, test.b); got != test.want {
			t.Errorf("%s: jsonEqual(%q, %q) = %v, want %v", name, test.a, test.b, got, test.want)
		}
	}
}

func TestKeepEquivalentTextHandlesNullAndUnknown(t *testing.T) {
	api := types.StringValue(multilineYAML)
	for name, prior := range map[string]types.String{"null": types.StringNull(), "unknown": types.StringUnknown()} {
		if got := keepEquivalentText(prior, api, yamlEqual); !got.Equal(api) {
			t.Errorf("%s prior: got %v, want the API value", name, got)
		}
	}
	if got := keepEquivalentText(types.StringValue(inlineYAML), types.StringNull(), yamlEqual); !got.IsNull() {
		t.Errorf("null API value: got %v, want null", got)
	}
}

func remotesList(t *testing.T, raw ...string) types.List {
	t.Helper()
	items := make([]ConfigRemoteModel, 0, len(raw))
	for _, r := range raw {
		items = append(items, ConfigRemoteModel{Name: types.StringValue("collector"), RawConfiguration: types.StringValue(r)})
	}
	list, diags := types.ListValueFrom(context.Background(), types.ObjectType{AttrTypes: configRemoteAttrTypes()}, items)
	if diags.HasError() {
		t.Fatal(diags)
	}
	return list
}

func apiThing(settings, raw string) *config_things_service.ConfigThing {
	name := "collector"
	return &config_things_service.ConfigThing{Id: "thing-1", Settings: &settings, Remotes: []config_things_service.ConfigRemote{{Name: &name, RawConfiguration: &raw}}}
}

// An API value that is the prior document in another format keeps the prior text. A different
// document takes the API value, and so does a read with no prior, after an import.
func TestFlattenKeepsPriorTextOfEqualDocuments(t *testing.T) {
	ctx := context.Background()
	prior := &ConfigThingModel{Settings: types.StringValue(compactJSON), Remotes: remotesList(t, inlineYAML)}
	tests := map[string]struct {
		prior              *ConfigThingModel
		api                *config_things_service.ConfigThing
		settings, rawValue string
	}{
		"equal documents":     {prior, apiThing(spacedJSON, multilineYAML), compactJSON, inlineYAML},
		"different documents": {prior, apiThing(`{"a":2}`, changedYAML), `{"a":2}`, changedYAML},
		"no prior":            {nil, apiThing(spacedJSON, multilineYAML), spacedJSON, multilineYAML},
	}
	for name, test := range tests {
		got, diags := flatten(ctx, test.api, test.prior)
		if diags.HasError() {
			t.Fatalf("%s: %v", name, diags)
		}
		if got.Settings.ValueString() != test.settings {
			t.Errorf("%s: settings = %q, want %q", name, got.Settings.ValueString(), test.settings)
		}
		var remotes []ConfigRemoteModel
		if diags := got.Remotes.ElementsAs(ctx, &remotes, false); diags.HasError() || len(remotes) != 1 {
			t.Fatalf("%s: remotes = %v, %v", name, got.Remotes, diags)
		}
		if remotes[0].RawConfiguration.ValueString() != test.rawValue {
			t.Errorf("%s: raw_configuration = %q, want %q", name, remotes[0].RawConfiguration.ValueString(), test.rawValue)
		}
	}
}

// The API returns the remotes in another order, with each document in another format. The items
// still match the prior items as documents, so the state keeps the prior order and text.
func TestFlattenKeepsPriorOrderOfReformattedItems(t *testing.T) {
	ctx := context.Background()
	prior := &ConfigThingModel{Settings: types.StringValue(compactJSON), Remotes: remotesList(t, inlineYAML, "a: [1]")}
	name, first, second := "collector", "a:\n  - 1\n", multilineYAML
	api := &config_things_service.ConfigThing{Id: "thing-1", Settings: ptr(compactJSON), Remotes: []config_things_service.ConfigRemote{
		{Name: &name, RawConfiguration: &first},
		{Name: &name, RawConfiguration: &second},
	}}
	got, diags := flatten(ctx, api, prior)
	if diags.HasError() {
		t.Fatal(diags)
	}
	var remotes []ConfigRemoteModel
	if diags := got.Remotes.ElementsAs(ctx, &remotes, false); diags.HasError() || len(remotes) != 2 {
		t.Fatalf("remotes = %v, %v", got.Remotes, diags)
	}
	if remotes[0].RawConfiguration.ValueString() != inlineYAML || remotes[1].RawConfiguration.ValueString() != "a: [1]" {
		t.Fatalf("raw_configuration = %q, %q, want the prior order and text", remotes[0].RawConfiguration.ValueString(), remotes[1].RawConfiguration.ValueString())
	}
}

func ptr(v string) *string { return &v }

// remote is one remote configuration, as the prior holds it and as the API returns it. A nil name
// is a remote without a name.
type remote struct {
	name     *string
	raw      string
	selector map[string]string
}

// priorRemotes is the prior list of remotes after Create or Update: the plan, in which the server
// sets the id and the hash of each remote later, so they are unknown.
func priorRemotes(t *testing.T, remotes ...remote) types.List {
	t.Helper()
	ctx := context.Background()
	items := make([]ConfigRemoteModel, 0, len(remotes))
	for _, r := range remotes {
		item := ConfigRemoteModel{Id: types.StringUnknown(), Hash: types.StringUnknown(), Name: types.StringPointerValue(r.name), RawConfiguration: types.StringValue(r.raw)}
		if r.selector != nil {
			attributes, diags := types.MapValueFrom(ctx, types.StringType, r.selector)
			if diags.HasError() {
				t.Fatal(diags)
			}
			item.Selector = &ConfigSelectorResponseModel{Attributes: attributes}
		}
		items = append(items, item)
	}
	list, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: configRemoteAttrTypes()}, items)
	if diags.HasError() {
		t.Fatal(diags)
	}
	return list
}

// apiRemotes returns the remotes as the API returns them, each with a server-set id and hash.
func apiRemotes(remotes ...remote) []config_things_service.ConfigRemote {
	out := make([]config_things_service.ConfigRemote, 0, len(remotes))
	for i, r := range remotes {
		item := config_things_service.ConfigRemote{Id: ptr(fmt.Sprintf("id-%d", i)), Hash: ptr(fmt.Sprintf("hash-%d", i)), Name: r.name, RawConfiguration: ptr(r.raw)}
		if r.selector != nil {
			item.Selector = &config_things_service.ConfigSelectorResponse{Attributes: r.selector}
		}
		out = append(out, item)
	}
	return out
}

// The request sends the remotes with their own component, and the response adds an id and a hash.
// The items pair by the fields that the request sends, with the YAML document compared as a
// document, so a reorder that the API does not apply keeps the prior order. The API values stay,
// with their id and hash. When an item changed, or the API added or dropped one, the API order
// stays, so the plan shows the change.
func TestFlattenPairsRemotesByRequestFields(t *testing.T) {
	ctx := context.Background()
	a := remote{name: ptr("a"), raw: inlineYAML, selector: map[string]string{"type": "agent"}}
	b := remote{name: ptr("b"), raw: "b: 1\n", selector: map[string]string{"type": "cluster"}}
	aReformatted := remote{name: a.name, raw: multilineYAML, selector: a.selector}
	bChanged := remote{name: b.name, raw: "b: 2\n", selector: b.selector}
	bOtherSelector := remote{name: b.name, raw: b.raw, selector: map[string]string{"type": "agent"}}
	nameless := remote{raw: "c: 1\n"}
	tests := map[string]struct {
		prior, api []remote
		want       []string // the raw documents of the state, in order
		wantIDs    []string // the ids of the state, from the API
	}{
		"reorder only":              {[]remote{b, a}, []remote{a, b}, []string{"b: 1\n", inlineYAML}, []string{"id-1", "id-0"}},
		"reorder of a reformatted":  {[]remote{b, a}, []remote{aReformatted, b}, []string{"b: 1\n", inlineYAML}, []string{"id-1", "id-0"}},
		"reorder plus one changed":  {[]remote{b, a}, []remote{bChanged, a}, []string{"b: 2\n", inlineYAML}, []string{"id-0", "id-1"}},
		"changed selector":          {[]remote{b, a}, []remote{bOtherSelector, a}, []string{"b: 1\n", inlineYAML}, []string{"id-0", "id-1"}},
		"nameless item":             {[]remote{nameless, a}, []remote{a, nameless}, []string{"c: 1\n", inlineYAML}, []string{"id-1", "id-0"}},
		"duplicate identical items": {[]remote{b, a, a}, []remote{a, a, b}, []string{"b: 1\n", inlineYAML, inlineYAML}, []string{"id-2", "id-0", "id-1"}},
		"same order":                {[]remote{a, b}, []remote{a, b}, []string{inlineYAML, "b: 1\n"}, []string{"id-0", "id-1"}},
		"item that the API added":   {[]remote{b}, []remote{a, b}, []string{inlineYAML, "b: 1\n"}, []string{"id-0", "id-1"}},
		"item that the API dropped": {[]remote{b, a}, []remote{a}, []string{inlineYAML}, []string{"id-0"}},
		"no prior item matches":     {[]remote{bChanged}, []remote{a, b}, []string{inlineYAML, "b: 1\n"}, []string{"id-0", "id-1"}},
	}
	for name, test := range tests {
		prior := &ConfigThingModel{Remotes: priorRemotes(t, test.prior...)}
		got, diags := flatten(ctx, &config_things_service.ConfigThing{Id: "thing-1", Remotes: apiRemotes(test.api...)}, prior)
		if diags.HasError() {
			t.Fatalf("%s: %v", name, diags)
		}
		var remotes []ConfigRemoteModel
		if diags := got.Remotes.ElementsAs(ctx, &remotes, false); diags.HasError() {
			t.Fatalf("%s: %v", name, diags)
		}
		var raws, ids []string
		for _, r := range remotes {
			raws, ids = append(raws, r.RawConfiguration.ValueString()), append(ids, r.Id.ValueString())
		}
		if fmt.Sprint(raws) != fmt.Sprint(test.want) || fmt.Sprint(ids) != fmt.Sprint(test.wantIDs) {
			t.Errorf("%s: raw_configuration = %q, id = %q, want %q, %q", name, raws, ids, test.want, test.wantIDs)
		}
	}
}

// The API returns an unset list or map as an empty one. The state keeps the form of the plan
// after Create, or of the state before Read, so Terraform sees no difference. Without a prior,
// after a state upgrade, the state takes the API value.
func TestSetStateKeepsTheFormOfEmptyCollections(t *testing.T) {
	ctx := context.Background()
	var schemaResp frameworkresource.SchemaResponse
	NewResource(Hooks{}).Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResp)
	objectType := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	remoteType := objectType.AttributeTypes["remotes"].(tftypes.List).ElementType.(tftypes.Object)
	metadataType := objectType.AttributeTypes["metadata"]
	emptyLabels := tftypes.NewValue(objectType.AttributeTypes["labels"], []tftypes.Value{})
	emptyMetadata := tftypes.NewValue(metadataType, map[string]tftypes.Value{})
	null := tftypes.NewValue(tftypes.String, nil)

	// prior is a value of the resource with the labels and the metadata.
	prior := func(labels, metadata tftypes.Value) tftypes.Value {
		remote := tftypes.NewValue(remoteType, map[string]tftypes.Value{
			"name": str("collector"), "raw_configuration": str(inlineYAML), "id": null, "hash": null,
			"selector": tftypes.NewValue(remoteType.AttributeTypes["selector"], nil),
		})
		return tftypes.NewValue(objectType, map[string]tftypes.Value{
			"id": str("thing-1"), "name": null, "settings": null, "template": null, "version": str("v1"),
			"remotes": tftypes.NewValue(tftypes.List{ElementType: remoteType}, []tftypes.Value{remote}),
			"labels":  labels, "metadata": metadata,
		})
	}
	nullLabels, nullMetadata := tftypes.NewValue(objectType.AttributeTypes["labels"], nil), tftypes.NewValue(metadataType, nil)
	name, raw, version := "collector", inlineYAML, "v1"
	api := func(labels []string, metadata map[string]string) *config_things_service.ConfigThing {
		return &config_things_service.ConfigThing{Id: "thing-1", Version: &version, Labels: labels, Metadata: metadata,
			Remotes: []config_things_service.ConfigRemote{{Name: &name, RawConfiguration: &raw}}}
	}
	tests := map[string]struct {
		prior                    *tftypes.Value
		api                      *config_things_service.ConfigThing
		wantLabels, wantMetadata tftypes.Value
	}{
		"unset, API returns empty":    {ptrValue(prior(nullLabels, nullMetadata)), api([]string{}, map[string]string{}), nullLabels, nullMetadata},
		"empty, API returns unset":    {ptrValue(prior(emptyLabels, emptyMetadata)), api(nil, nil), emptyLabels, emptyMetadata},
		"set, API returns empty":      {ptrValue(prior(tftypes.NewValue(objectType.AttributeTypes["labels"], []tftypes.Value{str("a")}), nullMetadata)), api([]string{}, map[string]string{}), emptyLabels, nullMetadata},
		"no prior, API returns empty": {nil, api([]string{}, map[string]string{}), emptyLabels, emptyMetadata},
	}
	for name, test := range tests {
		state := tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objectType, nil)}
		var priorData tfData
		if test.prior != nil {
			priorData = &tfsdk.Plan{Schema: schemaResp.Schema, Raw: *test.prior}
		}
		if diags := NewResource(Hooks{}).(*Resource).setState(ctx, test.api, priorData, &state); diags.HasError() {
			t.Fatalf("%s: %v", name, diags)
		}
		if got := at(t, state.Raw, "labels"); !got.Equal(test.wantLabels) {
			t.Errorf("%s: labels = %v, want %v", name, got, test.wantLabels)
		}
		if got := at(t, state.Raw, "metadata"); !got.Equal(test.wantMetadata) {
			t.Errorf("%s: metadata = %v, want %v", name, got, test.wantMetadata)
		}
	}
}

func ptrValue(v tftypes.Value) *tftypes.Value { return &v }

// testProvider serves the generated resource, so that a test can plan it through the protocol as
// Terraform does. Planning never calls the API.
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

type thing struct {
	settings, name, template tftypes.Value // a zero template is null
	remotes                  []string
	unknownRemoteIDs         bool // the server-set id and hash of each remote are unknown, not null
}

func (th thing) value(objectType tftypes.Object, id, version tftypes.Value) tftypes.Value {
	remoteType := objectType.AttributeTypes["remotes"].(tftypes.List).ElementType.(tftypes.Object)
	items := make([]tftypes.Value, 0, len(th.remotes))
	serverSet := tftypes.NewValue(tftypes.String, nil)
	if th.unknownRemoteIDs {
		serverSet = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	}
	for _, raw := range th.remotes {
		items = append(items, tftypes.NewValue(remoteType, map[string]tftypes.Value{
			"name":              tftypes.NewValue(tftypes.String, "collector"),
			"raw_configuration": tftypes.NewValue(tftypes.String, raw),
			"id":                serverSet,
			"hash":              serverSet,
			"selector":          tftypes.NewValue(remoteType.AttributeTypes["selector"], nil),
		}))
	}
	template := th.template
	if template.Type() == nil {
		template = tftypes.NewValue(tftypes.String, nil)
	}
	return tftypes.NewValue(objectType, map[string]tftypes.Value{
		"id":       id,
		"name":     th.name,
		"settings": th.settings,
		"template": template,
		"version":  version,
		"remotes":  tftypes.NewValue(tftypes.List{ElementType: remoteType}, items),
		"labels":   tftypes.NewValue(objectType.AttributeTypes["labels"], nil),
		"metadata": tftypes.NewValue(objectType.AttributeTypes["metadata"], nil),
	})
}

// plan plans the config against a prior state that the API wrote with the prior documents.
// Like Terraform, the proposed new state takes the config, and the prior value of each computed
// attribute that the config does not set.
func plan(t *testing.T, prior, config thing) (planned, priorValue tftypes.Value, objectType tftypes.Object) {
	t.Helper()
	planned, priorValue, objectType, replace := planReplace(t, prior, config)
	if len(replace) != 0 {
		t.Fatalf("plan requires replace: %v", replace)
	}
	return planned, priorValue, objectType
}

// planReplace is plan, and also returns the attributes whose change replaces the resource.
func planReplace(t *testing.T, prior, config thing) (planned, priorValue tftypes.Value, objectType tftypes.Object, replace []*tftypes.AttributePath) {
	t.Helper()
	ctx := context.Background()
	server, err := providerserver.NewProtocol6WithError(testProvider{})()
	if err != nil {
		t.Fatal(err)
	}
	schemaResp, err := server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	objectType = schemaResp.ResourceSchemas["test_config_thing"].ValueType().(tftypes.Object)
	id, version := tftypes.NewValue(tftypes.String, "thing-1"), tftypes.NewValue(tftypes.String, "v1")
	null := tftypes.NewValue(tftypes.String, nil)
	priorValue = prior.value(objectType, id, version)
	encode := func(v tftypes.Value) *tfprotov6.DynamicValue {
		dv, err := tfprotov6.NewDynamicValue(objectType, v)
		if err != nil {
			t.Fatal(err)
		}
		return &dv
	}
	resp, err := server.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{
		TypeName:         "test_config_thing",
		PriorState:       encode(priorValue),
		Config:           encode(config.value(objectType, null, null)),
		ProposedNewState: encode(config.value(objectType, id, version)),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Fatalf("plan: %s: %s", d.Summary, d.Detail)
		}
	}
	planned, err = resp.PlannedState.Unmarshal(objectType)
	if err != nil {
		t.Fatal(err)
	}
	return planned, priorValue, objectType, resp.RequiresReplace
}

func str(v string) tftypes.Value { return tftypes.NewValue(tftypes.String, v) }

func at(t *testing.T, v tftypes.Value, steps ...any) tftypes.Value {
	t.Helper()
	p := tftypes.NewAttributePath()
	for _, step := range steps {
		switch s := step.(type) {
		case string:
			p = p.WithAttributeName(s)
		case int:
			p = p.WithElementKeyInt(s)
		}
	}
	found, _, err := tftypes.WalkAttributePath(v, p)
	if err != nil {
		t.Fatal(err)
	}
	return found.(tftypes.Value)
}

// The state has the inline YAML and compact JSON that were applied. The config has the same
// documents in another format. The plan is the state: no change, and the computed id and version
// stay known.
func TestEqualDocumentsPlanNoChange(t *testing.T) {
	prior := thing{settings: str(compactJSON), name: str("name"), remotes: []string{inlineYAML}}
	config := thing{settings: str(spacedJSON), name: str("name"), remotes: []string{multilineYAML}}
	planned, priorValue, _ := plan(t, prior, config)
	if !planned.Equal(priorValue) {
		t.Fatalf("planned:\n%v\nwant the prior state:\n%v", planned, priorValue)
	}
}

// A real change of a document plans an update. The other document keeps its state text, and
// Terraform marks the computed version unknown.
func TestChangedDocumentPlansUpdate(t *testing.T) {
	prior := thing{settings: str(compactJSON), name: str("name"), remotes: []string{inlineYAML}}
	tests := map[string]struct{ config, want thing }{
		"yaml": {
			config: thing{settings: str(spacedJSON), name: str("name"), remotes: []string{changedYAML}},
			want:   thing{settings: str(compactJSON), name: str("name"), remotes: []string{changedYAML}, unknownRemoteIDs: true},
		},
		"json": {
			config: thing{settings: str(`{"a":2}`), name: str("name"), remotes: []string{multilineYAML}},
			want:   thing{settings: str(`{"a":2}`), name: str("name"), remotes: []string{inlineYAML}, unknownRemoteIDs: true},
		},
	}
	for name, test := range tests {
		planned, _, objectType := plan(t, prior, test.config)
		want := test.want.value(objectType, str("thing-1"), tftypes.NewValue(tftypes.String, tftypes.UnknownValue))
		if !planned.Equal(want) {
			t.Errorf("%s: planned:\n%v\nwant:\n%v", name, planned, want)
		}
	}
}

// Numbers that one float64 holds are different numbers, so changing one plans an update.
func TestChangedBigNumberPlansUpdate(t *testing.T) {
	prior := thing{settings: str(`{"id":9007199254740992}`), name: str("name"), remotes: []string{"id: 9007199254740992"}}
	config := thing{settings: str(`{"id":9007199254740993}`), name: str("name"), remotes: []string{"id: 9007199254740993"}}
	planned, _, _ := plan(t, prior, config)
	if !at(t, planned, "settings").Equal(str(`{"id":9007199254740993}`)) || !at(t, planned, "remotes", 0, "raw_configuration").Equal(str("id: 9007199254740993")) {
		t.Fatalf("planned = %v, want the configured numbers", planned)
	}
}

// Only Create sets template. The same document in another format plans no change, so it does not
// replace the resource. Another document does.
func TestImmutableDocumentReplacesOnlyOnChange(t *testing.T) {
	prior := thing{settings: str(compactJSON), name: str("name"), template: str(compactJSON), remotes: []string{inlineYAML}}
	same := thing{settings: str(compactJSON), name: str("name"), template: str(spacedJSON), remotes: []string{inlineYAML}}
	planned, priorValue, _, replace := planReplace(t, prior, same)
	if len(replace) != 0 || !planned.Equal(priorValue) {
		t.Fatalf("replace = %v, planned:\n%v\nwant no replace and the prior state", replace, planned)
	}
	changed := thing{settings: str(compactJSON), name: str("name"), template: str(`{"a":2}`), remotes: []string{inlineYAML}}
	_, _, _, replace = planReplace(t, prior, changed)
	if len(replace) != 1 || !replace[0].Equal(tftypes.NewAttributePath().WithAttributeName("template")) {
		t.Fatalf("replace = %v, want template", replace)
	}
}

// A document that does not parse is compared as text, so any change of its text plans a change.
func TestInvalidDocumentComparesText(t *testing.T) {
	prior := thing{settings: str(`{"a":`), name: str("name"), remotes: []string{"a: [1"}}
	config := thing{settings: str(`{"a": `), name: str("name"), remotes: []string{"a: [1"}}
	planned, priorValue, _ := plan(t, prior, config)
	if planned.Equal(priorValue) {
		t.Fatal("a changed text that is not JSON planned no change")
	}
	if got := at(t, planned, "settings"); !got.Equal(str(`{"a": `)) {
		t.Fatalf("settings = %v, want the configured text", got)
	}
}

// An unknown configured value, from a variable for example, stays a change even when the documents
// are equal.
func TestUnknownConfigValueStaysAChange(t *testing.T) {
	prior := thing{settings: str(compactJSON), name: str("name"), remotes: []string{inlineYAML}}
	config := thing{settings: str(spacedJSON), name: tftypes.NewValue(tftypes.String, tftypes.UnknownValue), remotes: []string{multilineYAML}}
	planned, _, _ := plan(t, prior, config)
	if at(t, planned, "name").IsKnown() {
		t.Fatal("the unknown configured name became known")
	}
	if !at(t, planned, "settings").Equal(str(compactJSON)) || !at(t, planned, "remotes", 0, "raw_configuration").Equal(str(inlineYAML)) {
		t.Fatalf("planned documents = %v, want the state text", planned)
	}
}
