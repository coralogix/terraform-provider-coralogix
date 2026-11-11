package configthing

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
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
		return withNulls(objectType, map[string]tftypes.Value{
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
	policy                   tftypes.Value // a zero policy is null
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
	policy := th.policy
	if policy.Type() == nil {
		policy = tftypes.NewValue(tftypes.String, nil)
	}
	return withNulls(objectType, map[string]tftypes.Value{
		"id":       id,
		"name":     th.name,
		"settings": th.settings,
		"template": template,
		"version":  version,
		"remotes":  tftypes.NewValue(tftypes.List{ElementType: remoteType}, items),
		"labels":   tftypes.NewValue(objectType.AttributeTypes["labels"], nil),
		"metadata": tftypes.NewValue(objectType.AttributeTypes["metadata"], nil),
		"policy":   policy,
	})
}

// withNulls is the value of the resource with the given attributes, and null for the others, the
// Terraform-only ones included.
func withNulls(objectType tftypes.Object, values map[string]tftypes.Value) tftypes.Value {
	out := map[string]tftypes.Value{}
	for name, typ := range objectType.AttributeTypes {
		out[name] = tftypes.NewValue(typ, nil)
	}
	for name, v := range values {
		out[name] = v
	}
	return tftypes.NewValue(objectType, out)
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

// planFixture is the state of a thing with compact settings, the configuration that spells the same
// settings with another format, and the plan that Terraform proposes after the plan modifiers: the
// settings of the state, and the version that the server sets, unknown.
func planFixture(t *testing.T) (schemaResp frameworkresource.SchemaResponse, state, config, plan tftypes.Value) {
	t.Helper()
	ctx := context.Background()
	NewResource(Hooks{}).Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResp)
	objectType := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	values := func(settings string, version tftypes.Value) tftypes.Value {
		out := map[string]tftypes.Value{}
		for name, typ := range objectType.AttributeTypes {
			out[name] = tftypes.NewValue(typ, nil)
		}
		out["id"], out["settings"], out["version"] = str("thing-1"), str(settings), version
		return tftypes.NewValue(objectType, out)
	}
	version := tftypes.NewValue(tftypes.String, "v1")
	return schemaResp, values(compactJSON, version), values(spacedJSON, tftypes.NewValue(tftypes.String, nil)), values(compactJSON, tftypes.NewValue(tftypes.String, tftypes.UnknownValue))
}

// The ModifyPlan hook runs after the equality steps: it sees the plan that they leave, which is the
// state for a change of format alone, and what it sets stays in the plan. It never runs on destroy,
// and a resource without the hook plans as before.
func TestModifyPlanHookRunsAfterTheEqualitySteps(t *testing.T) {
	ctx := context.Background()
	schemaResp, state, config, plan := planFixture(t)
	var sawPlan tftypes.Value
	calls := 0
	hook := func(ctx context.Context, req frameworkresource.ModifyPlanRequest, resp *frameworkresource.ModifyPlanResponse) {
		calls++
		sawPlan = resp.Plan.Raw
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("name"), "from-hook")...)
	}
	r := NewResource(Hooks{ModifyPlan: hook}).(frameworkresource.ResourceWithModifyPlan)
	req := frameworkresource.ModifyPlanRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: config},
		State:  tfsdk.State{Schema: schemaResp.Schema, Raw: state},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
	}
	resp := &frameworkresource.ModifyPlanResponse{Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan}}
	r.ModifyPlan(ctx, req, resp)
	if resp.Diagnostics.HasError() || calls != 1 {
		t.Fatalf("calls = %d, diagnostics = %v, want one call and no error", calls, resp.Diagnostics)
	}
	if !sawPlan.Equal(state) {
		t.Fatalf("the hook saw %v, want the state that the equality steps leave", sawPlan)
	}
	var name types.String
	resp.Diagnostics.Append(resp.Plan.GetAttribute(ctx, path.Root("name"), &name)...)
	var settings types.String
	resp.Diagnostics.Append(resp.Plan.GetAttribute(ctx, path.Root("settings"), &settings)...)
	if name.ValueString() != "from-hook" || settings.ValueString() != compactJSON {
		t.Fatalf("planned name %q and settings %q, want the value of the hook and the state's settings", name, settings)
	}

	// Destroy: the plan is null, and the hook does not run.
	destroy := frameworkresource.ModifyPlanRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(config.Type(), nil)},
		State:  tfsdk.State{Schema: schemaResp.Schema, Raw: state},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: tftypes.NewValue(config.Type(), nil)},
	}
	r.ModifyPlan(ctx, destroy, &frameworkresource.ModifyPlanResponse{Plan: destroy.Plan})
	if calls != 1 {
		t.Fatalf("the hook ran on destroy")
	}

	// No hook: the equality steps alone plan no change.
	plain := NewResource(Hooks{}).(frameworkresource.ResourceWithModifyPlan)
	plainResp := &frameworkresource.ModifyPlanResponse{Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan}}
	plain.ModifyPlan(ctx, req, plainResp)
	if plainResp.Diagnostics.HasError() || !plainResp.Plan.Raw.Equal(state) {
		t.Fatalf("without a hook the plan is %v, want the state", plainResp.Plan.Raw)
	}
}

// An error of the hook stays in the response.
func TestModifyPlanHookErrorIsReported(t *testing.T) {
	ctx := context.Background()
	schemaResp, state, config, plan := planFixture(t)
	r := NewResource(Hooks{ModifyPlan: func(_ context.Context, _ frameworkresource.ModifyPlanRequest, resp *frameworkresource.ModifyPlanResponse) {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("rejected", "no"))
	}}).(frameworkresource.ResourceWithModifyPlan)
	resp := &frameworkresource.ModifyPlanResponse{Plan: tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan}}
	r.ModifyPlan(ctx, frameworkresource.ModifyPlanRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: config},
		State:  tfsdk.State{Schema: schemaResp.Schema, Raw: state},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("the error of the hook is lost")
	}
}

// protocolServer serves the generated resource, and returns the type of its value.
func protocolServer(t *testing.T) (tfprotov6.ProviderServer, tftypes.Object) {
	t.Helper()
	server, err := providerserver.NewProtocol6WithError(testProvider{})()
	if err != nil {
		t.Fatal(err)
	}
	schemaResp, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	return server, schemaResp.ResourceSchemas["test_config_thing"].ValueType().(tftypes.Object)
}

func encodeValue(t *testing.T, objectType tftypes.Object, v tftypes.Value) *tfprotov6.DynamicValue {
	t.Helper()
	dv, err := tfprotov6.NewDynamicValue(objectType, v)
	if err != nil {
		t.Fatal(err)
	}
	return &dv
}

// planOf plans a change of the thing from prior to the proposed new state, with config.
func planOf(t *testing.T, prior, config, proposed tftypes.Value) tftypes.Value {
	t.Helper()
	server, objectType := protocolServer(t)
	resp, err := server.PlanResourceChange(context.Background(), &tfprotov6.PlanResourceChangeRequest{
		TypeName:         "test_config_thing",
		PriorState:       encodeValue(t, objectType, prior),
		Config:           encodeValue(t, objectType, config),
		ProposedNewState: encodeValue(t, objectType, proposed),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Fatalf("plan: %s: %s", d.Summary, d.Detail)
		}
	}
	planned, err := resp.PlannedState.Unmarshal(objectType)
	if err != nil {
		t.Fatal(err)
	}
	return planned
}

// An Optional and Computed string with an equality line and UseStateForUnknown: the same document
// in another format plans no change, an omitted value keeps the state, another document is a
// change, and a new resource plans it unknown, for a hook to fill in.
func TestExtraPolicyPlansLikeADocument(t *testing.T) {
	_, objectType := protocolServer(t)
	unknown := tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	null := tftypes.NewValue(tftypes.String, nil)
	value := func(settings string, policy tftypes.Value) tftypes.Value {
		return withNulls(objectType, map[string]tftypes.Value{"id": str("thing-1"), "name": str("n"), "settings": str(settings), "version": str("v1"), "policy": policy})
	}
	prior := value(compactJSON, str(compactJSON))
	otherSettings := `{"a":2}`
	tests := map[string]struct{ prior, config, proposed tftypes.Value }{
		// no change at all
		"same document": {prior, value(compactJSON, str(spacedJSON)), value(compactJSON, str(spacedJSON))},
		"omitted":       {prior, value(compactJSON, null), value(compactJSON, str(compactJSON))},
	}
	for name, test := range tests {
		if planned := planOf(t, test.prior, test.config, test.proposed); !planned.Equal(prior) {
			t.Errorf("%s: planned\n%v\nwant the prior state", name, planned)
		}
	}
	// A change of another attribute marks the computed attributes unknown. The policy keeps its
	// state value, in the format of the state, unless the configuration sets another document. A null
	// state value stays null: UseStateForUnknown copies the state, and a hook sets the value.
	changed := map[string]struct {
		prior, config, proposed tftypes.Value
		want                    tftypes.Value
	}{
		"omitted and kept":  {prior, value(otherSettings, null), value(otherSettings, str(compactJSON)), str(compactJSON)},
		"same document":     {prior, value(otherSettings, str(spacedJSON)), value(otherSettings, str(spacedJSON)), str(compactJSON)},
		"another document":  {prior, value(otherSettings, str(`{"a":3}`)), value(otherSettings, str(`{"a":3}`)), str(`{"a":3}`)},
		"null in the state": {value(compactJSON, null), value(otherSettings, null), value(otherSettings, null), null},
	}
	for name, test := range changed {
		planned := planOf(t, test.prior, test.config, test.proposed)
		if got := at(t, planned, "policy"); !got.Equal(test.want) {
			t.Errorf("%s: policy = %v, want %v; planned %v", name, got, test.want, planned)
		}
	}
	created := planOf(t, tftypes.NewValue(objectType, nil), withNulls(objectType, map[string]tftypes.Value{"name": str("n")}), withNulls(objectType, map[string]tftypes.Value{"name": str("n")}))
	if got := at(t, created, "policy"); !got.Equal(unknown) {
		t.Errorf("create: policy = %v, want unknown", got)
	}
	if got := at(t, created, "limits"); got.IsKnown() {
		t.Errorf("create: limits = %v, want an unknown object", got)
	}
}

// The state keeps the policy of the plan when the API holds none, and a read without a plan or state
// starts it as null. The Terraform-only objects start as typed nulls: flatten writes no value.
func TestExtraAttributesStartNullAndKeepThePlanValue(t *testing.T) {
	ctx := context.Background()
	var schemaResp frameworkresource.SchemaResponse
	NewResource(Hooks{}).Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResp)
	objectType := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	r := NewResource(Hooks{}).(*Resource)
	api := apiThing(compactJSON, inlineYAML)

	state := tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objectType, nil)}
	if diags := r.setState(ctx, api, nil, &state); diags.HasError() {
		t.Fatal(diags)
	}
	for _, name := range []string{"policy", "schedule", "limits"} {
		if got := at(t, state.Raw, name); !got.IsNull() {
			t.Errorf("without a prior, %s = %v, want null", name, got)
		}
	}

	prior := withNulls(objectType, map[string]tftypes.Value{"policy": str(spacedJSON)})
	state = tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objectType, nil)}
	if diags := r.setState(ctx, api, &tfsdk.Plan{Schema: schemaResp.Schema, Raw: prior}, &state); diags.HasError() {
		t.Fatal(diags)
	}
	if got := at(t, state.Raw, "policy"); !got.Equal(str(spacedJSON)) {
		t.Errorf("with a prior, policy = %v, want the planned text", got)
	}
}

// The schedule is a Terraform-only object: its required attributes and validators check the
// configuration, and a default fills the optional ones in the plan.
func TestExtraScheduleChecksAndDefaults(t *testing.T) {
	ctx := context.Background()
	server, objectType := protocolServer(t)
	schedule := func(daysOfWeek []string, start, offset tftypes.Value) tftypes.Value {
		return withNulls(objectType, map[string]tftypes.Value{"name": str("n"), "schedule": scheduleOf(objectType, daysOfWeek, start, offset)})
	}
	null := tftypes.NewValue(tftypes.String, nil)
	tests := map[string]struct {
		config tftypes.Value
		want   string // a part of the error, or "" for none
	}{
		"valid":                 {schedule([]string{"Monday"}, str("07:30"), null), ""},
		"bad time":              {schedule([]string{"Monday"}, str("7:30"), null), "use 24h time like 15:04"},
		"bad day":               {schedule([]string{"Funday"}, str("07:30"), null), "Funday"},
		"missing days":          {schedule(nil, str("07:30"), null), "days_of_week"},
		"missing start":         {schedule([]string{"Monday"}, null, null), "start_time"},
		"bad offset":            {schedule([]string{"Monday"}, str("07:30"), str("0700")), "use an offset like -0700"},
		"a good offset is kept": {schedule([]string{"Monday"}, str("07:30"), str("-0700")), ""},
	}
	for name, test := range tests {
		resp, err := server.ValidateResourceConfig(ctx, &tfprotov6.ValidateResourceConfigRequest{
			TypeName: "test_config_thing", Config: encodeValue(t, objectType, test.config),
		})
		if err != nil {
			t.Fatal(err)
		}
		var got string
		for _, d := range resp.Diagnostics {
			if d.Severity == tfprotov6.DiagnosticSeverityError {
				got += d.Summary + " " + d.Detail + " " + fmt.Sprint(d.Attribute)
			}
		}
		if (test.want == "") != (got == "") || !strings.Contains(got, test.want) {
			t.Errorf("%s: errors %q, want one with %q", name, got, test.want)
		}
	}

	// The plan fills the defaults of the optional attributes of the window, and a schedule that the
	// configuration leaves out stays null.
	config := schedule([]string{"Monday"}, str("07:30"), null)
	planned := planOf(t, tftypes.NewValue(objectType, nil), config, config)
	window := func(name string) tftypes.Value { return at(t, planned, "schedule", "active_on", name) }
	if !window("utc_offset").Equal(str("+0000")) || !window("enabled").Equal(tftypes.NewValue(tftypes.Bool, true)) || window("limit").IsNull() {
		t.Errorf("planned window utc_offset %v, enabled %v, limit %v, want the defaults", window("utc_offset"), window("enabled"), window("limit"))
	}
	if !window("tags").IsNull() {
		t.Errorf("planned tags = %v, want null: it has no default", window("tags"))
	}
	none := planOf(t, tftypes.NewValue(objectType, nil), withNulls(objectType, map[string]tftypes.Value{"name": str("n")}), withNulls(objectType, map[string]tftypes.Value{"name": str("n")}))
	if got := at(t, none, "schedule"); !got.IsNull() {
		t.Errorf("planned schedule = %v, want null", got)
	}
}

// A change of only the schedule makes an Update, because the attribute says inUpdate: a hook sends it.
// The object that the user may set and the server may fill in is Optional and Computed.
func TestExtraScheduleIsAnUpdateField(t *testing.T) {
	var schemaResp frameworkresource.SchemaResponse
	NewResource(Hooks{}).Schema(context.Background(), frameworkresource.SchemaRequest{}, &schemaResp)
	if _, ok := schemaResp.Schema.Attributes["schedule"]; !ok {
		t.Fatal("the schedule is not an attribute")
	}
	found := false
	for _, f := range updateFields {
		found = found || f.attr == "schedule"
	}
	if !found {
		t.Fatal("updateFields has no schedule: a change of only the schedule sends no Update")
	}
	if limits := schemaResp.Schema.Attributes["limits"]; !limits.IsOptional() || !limits.IsComputed() {
		t.Fatal("limits is not Optional and Computed")
	}
}

// scheduleOf is the value of the schedule attribute with one window. A nil list of days is null.
func scheduleOf(objectType tftypes.Object, daysOfWeek []string, start, offset tftypes.Value) tftypes.Value {
	scheduleType := objectType.AttributeTypes["schedule"].(tftypes.Object)
	activeOn := scheduleType.AttributeTypes["active_on"].(tftypes.Object)
	var dayValues any
	if daysOfWeek != nil {
		values := make([]tftypes.Value, 0, len(daysOfWeek))
		for _, d := range daysOfWeek {
			values = append(values, str(d))
		}
		dayValues = values
	}
	window := map[string]tftypes.Value{}
	for name, typ := range activeOn.AttributeTypes {
		window[name] = tftypes.NewValue(typ, nil)
	}
	window["days_of_week"], window["start_time"], window["utc_offset"] = tftypes.NewValue(activeOn.AttributeTypes["days_of_week"], dayValues), start, offset
	return tftypes.NewValue(scheduleType, map[string]tftypes.Value{"active_on": tftypes.NewValue(activeOn, window)})
}

// The state keeps the object of the plan, so a hook finds it there. A value inside it that the plan
// left unknown cannot enter state: AfterFlatten must set it, and without that the write is an
// error. A new read starts the object as null.
func TestExtraObjectKeepsTheObjectOfThePlan(t *testing.T) {
	ctx := context.Background()
	var schemaResp frameworkresource.SchemaResponse
	NewResource(Hooks{}).Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResp)
	objectType := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	scheduleType := objectType.AttributeTypes["schedule"]
	known := scheduleOf(objectType, []string{"Monday"}, str("07:30"), str("+0000"))
	unknownOffset := scheduleOf(objectType, []string{"Monday"}, str("07:30"), tftypes.NewValue(tftypes.String, tftypes.UnknownValue))
	// fillOffset is a hook that sets the offset that the plan left unknown, and keeps the rest.
	fillOffset := func(ctx context.Context, m *ConfigThingModel, _ *config_things_service.ConfigThing) diag.Diagnostics {
		var diags diag.Diagnostics
		attrs := m.Schedule.Attributes()
		activeOn, _ := attrs["active_on"].(types.Object)
		window := activeOn.Attributes()
		window["utc_offset"] = types.StringValue("+0200")
		newActive, d := types.ObjectValue(activeOn.AttributeTypes(ctx), window)
		diags.Append(d...)
		attrs["active_on"] = newActive
		filled, d := types.ObjectValue(m.Schedule.AttributeTypes(ctx), attrs)
		diags.Append(d...)
		m.Schedule = filled
		return diags
	}
	tests := map[string]struct {
		plan    tftypes.Value
		hooks   Hooks
		want    tftypes.Value
		wantErr bool
	}{
		"known":                      {plan: known, want: known},
		"unknown inside, no hook":    {plan: unknownOffset, wantErr: true},
		"unknown inside, hook fills": {plan: unknownOffset, hooks: Hooks{AfterFlatten: fillOffset}, want: scheduleOf(objectType, []string{"Monday"}, str("07:30"), str("+0200"))},
		"null in the plan":           {plan: tftypes.NewValue(scheduleType, nil), want: tftypes.NewValue(scheduleType, nil)},
		"unknown in general":         {plan: tftypes.NewValue(scheduleType, tftypes.UnknownValue), want: tftypes.NewValue(scheduleType, nil)},
	}
	for name, test := range tests {
		r := NewResource(test.hooks).(*Resource)
		prior := withNulls(objectType, map[string]tftypes.Value{"schedule": test.plan})
		state := tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objectType, nil)}
		diags := r.setState(ctx, apiThing(compactJSON, inlineYAML), &tfsdk.Plan{Schema: schemaResp.Schema, Raw: prior}, &state)
		if test.wantErr {
			if !diags.HasError() {
				t.Errorf("%s: want an error for the unknown value inside the object", name)
			}
			continue
		}
		if diags.HasError() {
			t.Fatalf("%s: %v", name, diags)
		}
		if got := at(t, state.Raw, "schedule"); !got.Equal(test.want) {
			t.Errorf("%s: schedule = %v, want %v", name, got, test.want)
		}
	}
}

func number(v float64) types.Number { return types.NumberValue(big.NewFloat(v)) }

// A float32 field is a Float64 attribute, and an int32 and a double field are Number attributes, as
// the released resource had them. Each value goes to the API as the type that the API holds and
// reads back as the value that the configuration has: set, change, remove, and import.
func TestFloatAndNumberAttributesKeepTheReleasedTypes(t *testing.T) {
	ctx := context.Background()
	var schemaResp frameworkresource.SchemaResponse
	NewResource(Hooks{}).Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResp)
	objectType := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	for name, want := range map[string]tftypes.Type{"ratio": tftypes.Number, "retries": tftypes.Number, "scale": tftypes.Number} {
		if got := objectType.AttributeTypes[name]; !got.Equal(want) {
			t.Fatalf("%s has the type %v, want %v", name, got, want)
		}
	}
	if _, ok := schemaResp.Schema.Attributes["ratio"].(schema.Float64Attribute); !ok {
		t.Fatalf("ratio is %T, want a Float64Attribute", schemaResp.Schema.Attributes["ratio"])
	}

	set := &ConfigThingModel{Ratio: types.Float64Value(0.3), Retries: number(5), Scale: number(2.5)}
	req, diags := expandCreate(ctx, set)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if req.Ratio == nil || *req.Ratio != float32(0.3) || req.Retries == nil || *req.Retries != 5 || req.Scale == nil || *req.Scale != 2.5 {
		t.Fatalf("request = %+v, want 0.3, 5, 2.5", req)
	}

	// The API holds the float32 0.3, which is 0.30000001192092896 as a float64: it reads back as 0.3.
	api := &config_things_service.ConfigThing{Id: "thing-1", Ratio: req.Ratio, Retries: req.Retries, Scale: req.Scale}
	for name, prior := range map[string]*ConfigThingModel{"with the prior": set, "import, no prior": nil} {
		got, diags := flatten(ctx, api, prior)
		if diags.HasError() {
			t.Fatalf("%s: %v", name, diags)
		}
		if !got.Ratio.Equal(set.Ratio) || !got.Retries.Equal(set.Retries) || !got.Scale.Equal(set.Scale) {
			t.Errorf("%s: read %v, %v, %v, want %v, %v, %v", name, got.Ratio, got.Retries, got.Scale, set.Ratio, set.Retries, set.Scale)
		}
	}

	// A configured value that the API stores as another float32 stays as configured.
	long := float32(0.123456789)
	got, diags := flatten(ctx, &config_things_service.ConfigThing{Id: "thing-1", Ratio: &long}, &ConfigThingModel{Ratio: types.Float64Value(0.123456789)})
	if diags.HasError() || got.Ratio.ValueFloat64() != 0.123456789 {
		t.Errorf("ratio = %v, %v, want the configured 0.123456789 to stay", got.Ratio, diags)
	}

	// Remove: a null value sends nothing, and an absent value reads as null. So does an unknown one.
	for name, m := range map[string]*ConfigThingModel{
		"null":    {Ratio: types.Float64Null(), Retries: types.NumberNull(), Scale: types.NumberNull()},
		"unknown": {Ratio: types.Float64Unknown(), Retries: types.NumberUnknown(), Scale: types.NumberUnknown()},
	} {
		req, diags := expandCreate(ctx, m)
		if diags.HasError() || req.Ratio != nil || req.Retries != nil || req.Scale != nil {
			t.Errorf("%s: request = %+v, %v, want no value", name, req, diags)
		}
	}
	got, diags = flatten(ctx, &config_things_service.ConfigThing{Id: "thing-1"}, set)
	if diags.HasError() || !got.Ratio.IsNull() || !got.Retries.IsNull() || !got.Scale.IsNull() {
		t.Errorf("read = %v, %v, %v, %v, want null", got.Ratio, got.Retries, got.Scale, diags)
	}
}

// Terraform parses a decimal such as 0.1 with more bits than a double holds. The API stores the
// double, and reading it back as a double would plan one number and store another, which fails the
// apply (inconsistent result) and shows a diff on every plan. A Number that rounds to the stored
// double stays as configured. The test builds the number from text, as Terraform does, and not
// from a double.
func TestDoubleNumberKeepsTheConfiguredDecimal(t *testing.T) {
	ctx := context.Background()
	decimal := func(text string) types.Number {
		f, _, err := big.ParseFloat(text, 10, 512, big.ToNearestEven)
		if err != nil {
			t.Fatal(err)
		}
		return types.NumberValue(f)
	}
	for _, text := range []string{"0.1", "2.5", "0.123456789012345678901"} {
		configured := decimal(text)
		req, diags := expandCreate(ctx, &ConfigThingModel{Scale: configured})
		if diags.HasError() || req.Scale == nil {
			t.Fatalf("%s: expand = %v, %v", text, req, diags)
		}
		api := &config_things_service.ConfigThing{Id: "thing-1", Scale: req.Scale}
		got, diags := flatten(ctx, api, &ConfigThingModel{Scale: configured})
		if diags.HasError() {
			t.Fatalf("%s: %v", text, diags)
		}
		if !got.Scale.Equal(configured) {
			t.Errorf("%s: read %v, want the configured %v to stay", text, got.Scale.ValueBigFloat().Text('g', 30), configured.ValueBigFloat().Text('g', 30))
		}
		// Import has no prior: the state takes the double of the API, which the next plan shows.
		imported, diags := flatten(ctx, api, nil)
		if diags.HasError() || imported.Scale.IsNull() {
			t.Fatalf("%s: import read = %v, %v", text, imported.Scale, diags)
		}
	}

	// A configured value that is another double than the API's changes the state.
	other := 0.2
	got, diags := flatten(ctx, &config_things_service.ConfigThing{Id: "thing-1", Scale: &other}, &ConfigThingModel{Scale: decimal("0.1")})
	if diags.HasError() || got.Scale.Equal(decimal("0.1")) {
		t.Errorf("read = %v, %v, want the API value 0.2, not the prior", got.Scale, diags)
	}
}

// An int32 Number holds a whole number in the int32 range. Another value is an error at apply, not
// a cut value that reads back as another number.
func TestNumberOverAnInt32RejectsWhatTheAPICannotHold(t *testing.T) {
	for name, v := range map[string]float64{"a fraction": 2.5, "too large": 3e10, "too small": -3e10} {
		if _, diags := expandCreate(context.Background(), &ConfigThingModel{Retries: number(v)}); !diags.HasError() {
			t.Errorf("%s: no error for %v", name, v)
		}
	}
}

// validationErrors validates the config of the resource and returns the errors as one text.
func validationErrors(t *testing.T, objectType tftypes.Object, server tfprotov6.ProviderServer, config tftypes.Value) string {
	t.Helper()
	resp, err := server.ValidateResourceConfig(context.Background(), &tfprotov6.ValidateResourceConfigRequest{
		TypeName: "test_config_thing", Config: encodeValue(t, objectType, config),
	})
	if err != nil {
		t.Fatal(err)
	}
	var got string
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			got += d.Summary + " " + d.Detail + " " + fmt.Sprint(d.Attribute)
		}
	}
	return got
}

// A marker bool accepts true, and rejects a configured false at plan. A value that is not set is
// no error.
func TestEqualsTrueRejectsFalseAtPlan(t *testing.T) {
	server, objectType := protocolServer(t)
	tests := map[string]struct {
		marker tftypes.Value
		bad    bool
	}{
		"true":    {tftypes.NewValue(tftypes.Bool, true), false},
		"false":   {tftypes.NewValue(tftypes.Bool, false), true},
		"not set": {tftypes.NewValue(tftypes.Bool, nil), false},
		"unknown": {tftypes.NewValue(tftypes.Bool, tftypes.UnknownValue), false},
	}
	for name, test := range tests {
		got := validationErrors(t, objectType, server, withNulls(objectType, map[string]tftypes.Value{"name": str("n"), "marker": test.marker}))
		if (got != "") != test.bad {
			t.Errorf("%s: errors %q, want an error: %v", name, got, test.bad)
		}
	}
	// The bool reads back as set, so the second plan has no change.
	true_ := true
	got, diags := flatten(context.Background(), &config_things_service.ConfigThing{Id: "thing-1", Marker: &true_}, &ConfigThingModel{Marker: types.BoolValue(true)})
	if diags.HasError() || !got.Marker.ValueBool() {
		t.Errorf("marker = %v, %v, want true", got.Marker, diags)
	}
}

// The selector attributes need the name of their remote, which is an attribute of the object above
// the selector: the target "../name" goes up one level.
func TestAlsoRequiresReachesAnAttributeAboveTheParent(t *testing.T) {
	server, objectType := protocolServer(t)
	remoteType := objectType.AttributeTypes["remotes"].(tftypes.List).ElementType.(tftypes.Object)
	selectorType := remoteType.AttributeTypes["selector"].(tftypes.Object)
	remote := func(name tftypes.Value, attributes tftypes.Value) tftypes.Value {
		return withNulls(objectType, map[string]tftypes.Value{"name": str("n"), "remotes": tftypes.NewValue(tftypes.List{ElementType: remoteType}, []tftypes.Value{
			tftypes.NewValue(remoteType, map[string]tftypes.Value{
				"name": name, "raw_configuration": str(inlineYAML), "id": tftypes.NewValue(tftypes.String, nil), "hash": tftypes.NewValue(tftypes.String, nil),
				"selector": tftypes.NewValue(selectorType, map[string]tftypes.Value{"attributes": attributes}),
			}),
		})})
	}
	attrs := tftypes.NewValue(selectorType.AttributeTypes["attributes"], map[string]tftypes.Value{"app": str("web")})
	noAttrs := tftypes.NewValue(selectorType.AttributeTypes["attributes"], nil)
	tests := map[string]struct {
		config tftypes.Value
		bad    bool
	}{
		"both set":         {remote(str("collector"), attrs), false},
		"attributes alone": {remote(tftypes.NewValue(tftypes.String, nil), attrs), true},
		"name alone":       {remote(str("collector"), noAttrs), false},
	}
	for name, test := range tests {
		got := validationErrors(t, objectType, server, test.config)
		if (got != "") != test.bad || test.bad && !strings.Contains(got, "name") {
			t.Errorf("%s: errors %q, want an error naming the remote name: %v", name, got, test.bad)
		}
	}
}
