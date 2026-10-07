package configthing

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
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

func TestYAMLEqual(t *testing.T) {
	tests := map[string]struct {
		a, b string
		want bool
	}{
		"inline and multiline": {inlineYAML, multilineYAML, true},
		"key order":            {"a: 1\nb: 2\n", "b: 2\na: 1\n", true},
		"different value":      {inlineYAML, changedYAML, false},
		"different type":       {"a: 1", "a: '1'", false},
		"invalid":              {"a: [1", "a: [1]", false},
		"invalid itself":       {"a: [1", "a: [1", true},
		"empty":                {"", "", true},
		"empty and document":   {"", "a: 1", false},
		"empty and blank":      {"", "\n", true},
		"beyond float64":       {"a: 9007199254740992", "a: 9007199254740993", false},
		"beyond int64":         {"a: 18446744073709551616", "a: 18446744073709551617", false},
		"same big integer":     {"a: 18446744073709551617", "{a: 18446744073709551617}", true},
		"decimal beyond":       {"a: 0.10000000000000000001", "a: 0.10000000000000000002", false},
		"same float forms":     {"a: 1.5", "a: 15e-1", true},
		"integer and float":    {"a: 1", "a: 1.0", false},
		"hex integer":          {"a: 0x10", "a: 16", true},
		"alias":                {"x: &v [1]\ny: *v", "x: [1]\ny: [1]", true},
		"repeated key":         {"a: 1\na: 2", "a: 2", false},
		"number keys":          {"1: a\n2: b", "{2: b, 1: a}", true},
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
	return []func() frameworkresource.Resource{NewResource}
}
func (testProvider) DataSources(context.Context) []func() datasource.DataSource { return nil }

type thing struct {
	settings, name tftypes.Value
	remotes        []string
}

func (th thing) value(objectType tftypes.Object, id, version tftypes.Value) tftypes.Value {
	remoteType := objectType.AttributeTypes["remotes"].(tftypes.List).ElementType.(tftypes.Object)
	items := make([]tftypes.Value, 0, len(th.remotes))
	for _, raw := range th.remotes {
		items = append(items, tftypes.NewValue(remoteType, map[string]tftypes.Value{
			"name":              tftypes.NewValue(tftypes.String, "collector"),
			"raw_configuration": tftypes.NewValue(tftypes.String, raw),
		}))
	}
	return tftypes.NewValue(objectType, map[string]tftypes.Value{
		"id":       id,
		"name":     th.name,
		"settings": th.settings,
		"version":  version,
		"remotes":  tftypes.NewValue(tftypes.List{ElementType: remoteType}, items),
	})
}

// plan plans the config against a prior state that the API wrote with the prior documents.
// Like Terraform, the proposed new state takes the config, and the prior value of each computed
// attribute that the config does not set.
func plan(t *testing.T, prior, config thing) (planned, priorValue tftypes.Value, objectType tftypes.Object) {
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
	if len(resp.RequiresReplace) != 0 {
		t.Fatalf("plan requires replace: %v", resp.RequiresReplace)
	}
	planned, err = resp.PlannedState.Unmarshal(objectType)
	if err != nil {
		t.Fatal(err)
	}
	return planned, priorValue, objectType
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
			want:   thing{settings: str(compactJSON), name: str("name"), remotes: []string{changedYAML}},
		},
		"json": {
			config: thing{settings: str(`{"a":2}`), name: str("name"), remotes: []string{multilineYAML}},
			want:   thing{settings: str(`{"a":2}`), name: str("name"), remotes: []string{inlineYAML}},
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
