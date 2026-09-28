// This file is handwritten. It checks the overrides (D21) of
// spec/fake/chart.overrides.yaml: two oneOf objects shown by the name of the
// set arm (typeStrings), and two empty objects shown as bools.

package fakechart_test

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/generated/fakechart"
	sdk "github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/testsdk/go/openapi/gen/fake_boards_service"
)

func decode(t *testing.T, js string) *sdk.Chart {
	t.Helper()
	var c sdk.Chart
	if err := json.Unmarshal([]byte(js), &c); err != nil {
		t.Fatal(err)
	}
	return &c
}

func jsonOf(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// flatten reads the API object into the model, and through a Terraform state,
// so the attribute types of the model must match the schema.
func flatten(t *testing.T, js string) *fakechart.ChartModel {
	t.Helper()
	ctx := context.Background()
	m, diags := fakechart.FlattenChart(ctx, path.Root("chart"), decode(t, js))
	if diags.HasError() {
		t.Fatal(diags)
	}
	state := tfsdk.State{Schema: hostSchema(), Raw: tftypes.NewValue(hostSchema().Type().TerraformType(ctx), nil)}
	if diags := state.Set(ctx, &hostModel{Chart: m}); diags.HasError() {
		t.Fatalf("set: %v", diags)
	}
	var got hostModel
	if diags := state.Get(ctx, &got); diags.HasError() {
		t.Fatalf("get: %v", diags)
	}
	return got.Chart
}

func expand(t *testing.T, m *fakechart.ChartModel) string {
	t.Helper()
	out, diags := fakechart.ExpandChart(context.Background(), path.Root("chart"), m)
	if diags.HasError() {
		t.Fatal(diags)
	}
	return jsonOf(t, out)
}

// TestChartSchema checks the Terraform types of the overrides.
func TestChartSchema(t *testing.T) {
	attrs := fakechart.ChartAttributes()
	if _, ok := attrs["colors_by"].(schema.StringAttribute); !ok {
		t.Errorf("colors_by = %T, want a StringAttribute", attrs["colors_by"])
	}
	if _, ok := attrs["mapped_values"].(schema.BoolAttribute); !ok {
		t.Errorf("mapped_values = %T, want a BoolAttribute", attrs["mapped_values"])
	}
	agg := attrs["aggregation"].(schema.SingleNestedAttribute).Attributes
	if typ, ok := agg["type"].(schema.StringAttribute); !ok || !typ.Required {
		t.Errorf("aggregation.type = %#v, want a required StringAttribute", agg["type"])
	}
	for _, name := range []string{"field", "labels", "observation_field", "percent"} {
		if a, ok := agg[name]; !ok || !a.IsOptional() {
			t.Errorf("aggregation.%s is missing or not optional", name)
		}
	}
	if len(agg) != 5 {
		t.Errorf("aggregation has %d attributes, want type and the 4 arm fields", len(agg))
	}
	auto := attrs["min_max"].(schema.SingleNestedAttribute).Attributes["auto"]
	if _, ok := auto.(schema.BoolAttribute); !ok {
		t.Errorf("min_max.auto = %T, want a BoolAttribute", auto)
	}
}

// TestChartRoundTrip checks that each API value reads into the model, and
// expand sends it back the same.
func TestChartRoundTrip(t *testing.T) {
	for _, js := range []string{
		`{"colorsBy":{"stack":{}},"title":"t"}`,
		`{"colorsBy":{"groupBy":{}}}`,
		`{"aggregation":{"count":{}}}`,
		`{"aggregation":{"average":{"field":"f","labels":["a","b"],"observationField":{"keypath":["k"]}}}}`,
		`{"aggregation":{"percentile":{"field":"f","percent":95}}}`,
		`{"aggregation":{"percentile":{"percent":0}}}`,
		`{"aggregations":[{"count":{}},{"average":{"labels":[]}},{"percentile":{"percent":50}}]}`,
		`{"minMax":{"auto":{}}}`,
		`{"minMax":{"custom":{"max":2,"min":1}}}`,
		`{"mappedValues":{}}`,
	} {
		if got, want := expand(t, flatten(t, js)), jsonOf(t, decode(t, js)); got != want {
			t.Errorf("expand:\n got %s\nwant %s", got, want)
		}
	}
}

// TestChartReads checks how flatten reads each form: the fields of the other
// arms are null, a oneOf with no arm is an error (an arm the SDK does not
// know), and a missing empty object shown as a bool is null.
func TestChartReads(t *testing.T) {
	m := flatten(t, `{"aggregation":{"percentile":{"percent":95}}}`)
	a := m.Aggregation
	if a.Type.ValueString() != "percentile" || a.Percent.ValueFloat64() != 95 || !a.Field.IsNull() || !a.Labels.IsNull() || a.ObservationField != nil {
		t.Errorf("percentile = %+v", a)
	}
	a = flatten(t, `{"aggregation":{"count":{}}}`).Aggregation
	if a.Type.ValueString() != "count" || !a.Percent.IsNull() || !a.Labels.IsNull() {
		t.Errorf("count = %+v", a)
	}
	if got := flatten(t, `{"aggregation":{"average":{}}}`).Aggregation.Type.ValueString(); got != "avg" {
		t.Errorf("type of average = %q, want the name avg", got)
	}
	_, diags := fakechart.FlattenChart(context.Background(), path.Root("chart"), decode(t, `{"aggregation":{},"colorsBy":{},"aggregations":[{}]}`))
	var at []string
	for _, d := range diags {
		at = append(at, d.(interface{ Path() path.Path }).Path().String())
	}
	if want := []string{"chart.colors_by", "chart.aggregation", "chart.aggregations[0]"}; !slices.Equal(at, want) {
		t.Errorf("no arm: errors at %v, want %v", at, want)
	}
	m = flatten(t, `{"aggregations":[{"count":{}}]}`)
	m = flatten(t, `{}`)
	if !m.MappedValues.IsNull() || !m.ColorsBy.IsNull() || m.MinMax != nil {
		t.Errorf("empty: mapped_values = %v, colors_by = %v, min_max = %+v", m.MappedValues, m.ColorsBy, m.MinMax)
	}
	if !flatten(t, `{"minMax":{"auto":{}}}`).MinMax.Auto.ValueBool() {
		t.Error("min_max.auto is not true")
	}
}

// TestChartExpand checks expand of a model from a configuration: only the
// fields of the arm that type names are sent, and false is an error (the
// validator misses a value that was unknown at plan time).
func TestChartExpand(t *testing.T) {
	m := flatten(t, `{}`)
	m.Aggregation = &fakechart.ChartAggregationModel{
		Type:    types.StringValue("avg"),
		Field:   types.StringValue("f"),
		Labels:  types.ListNull(types.StringType),
		Percent: types.Float64Value(95),
	}
	m.ColorsBy = types.StringValue("group_by")
	if got, want := expand(t, m), `{"aggregation":{"average":{"field":"f"}},"colorsBy":{"groupBy":{}}}`; got != want {
		t.Errorf("expand:\n got %s\nwant %s", got, want)
	}
	m.ColorsBy, m.Aggregation.Type = types.StringUnknown(), types.StringUnknown()
	if got, want := expand(t, m), `{"aggregation":{}}`; got != want {
		t.Errorf("expand of unknown values:\n got %s\nwant %s", got, want)
	}
	m.MappedValues = types.BoolValue(false)
	_, diags := fakechart.ExpandChart(context.Background(), path.Root("chart"), m)
	if !diags.HasError() || !diags[0].(interface{ Path() path.Path }).Path().Equal(path.Root("chart").AtName("mapped_values")) {
		t.Errorf("expand of mapped_values false: %v, want an error at chart.mapped_values", diags)
	}
}

// TestChartValidators runs the generated validators through a provider
// server.
func TestChartValidators(t *testing.T) {
	for _, c := range []struct {
		name string
		edit func(m *fakechart.ChartModel)
		want string // "" for no error
	}{
		{"valid", func(*fakechart.ChartModel) {}, ""},
		{"a field of another arm", func(m *fakechart.ChartModel) { m.Aggregation.Type = types.StringValue("count") },
			`percent cannot be set when type is "count". at AttributeName("chart").AttributeName("aggregation").AttributeName("percent")`},
		{"a missing required field", func(m *fakechart.ChartModel) { m.Aggregation.Percent = types.Float64Null() },
			`percent is required when type is "percentile". at AttributeName("chart").AttributeName("aggregation").AttributeName("percent")`},
		{"a list item", func(m *fakechart.ChartModel) {
			m.Aggregations = flatten(t, `{"aggregations":[{"count":{}},{"average":{"field":"f"}}]}`).Aggregations
			items := []fakechart.ChartAggregationModel{}
			m.Aggregations.ElementsAs(context.Background(), &items, false)
			items[1].Type = types.StringValue("count")
			m.Aggregations, _ = types.ListValueFrom(context.Background(), types.ObjectType{AttrTypes: fakechart.ChartAggregationAttrTypes()}, items)
		}, `field cannot be set when type is "count". at AttributeName("chart").AttributeName("aggregations").ElementKeyInt(1).AttributeName("field")`},
		{"an unknown type", func(m *fakechart.ChartModel) { m.Aggregation.Type = types.StringValue("median") }, `AttributeName("aggregation").AttributeName("type")`},
		{"an unknown color", func(m *fakechart.ChartModel) { m.ColorsBy = types.StringValue("rainbow") }, `AttributeName("colors_by")`},
		{"false", func(m *fakechart.ChartModel) { m.MappedValues = types.BoolValue(false) },
			`this marker must be true when set; omit it otherwise at AttributeName("chart").AttributeName("mapped_values")`},
		{"auto false (F76)", func(m *fakechart.ChartModel) {
			m.MinMax = &fakechart.ChartMinMaxModel{Auto: types.BoolValue(false), Custom: nil}
		}, `this marker must be true when set; omit it otherwise at AttributeName("chart").AttributeName("min_max").AttributeName("auto")`},
		{"auto and custom", func(m *fakechart.ChartModel) {
			m.MinMax = &fakechart.ChartMinMaxModel{Auto: types.BoolValue(true), Custom: &fakechart.ChartMinMaxCustomModel{}}
		}, `AttributeName("min_max").AttributeName("auto")`},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := flatten(t, `{"aggregation":{"percentile":{"percent":95}},"colorsBy":{"stack":{}}}`)
			c.edit(m)
			got := validate(t, m)
			switch {
			case c.want == "" && got != "":
				t.Errorf("errors: %s", got)
			case c.want != "" && !strings.Contains(got, c.want):
				t.Errorf("errors = %q, want %s", got, c.want)
			}
		})
	}
}

func hostSchema() schema.Schema {
	return schema.Schema{Attributes: map[string]schema.Attribute{
		"chart": schema.SingleNestedAttribute{Required: true, Attributes: fakechart.ChartAttributes()},
	}}
}

type hostModel struct {
	Chart *fakechart.ChartModel `tfsdk:"chart"`
}

// validate returns the validation errors of a configuration with m: each
// detail and its path.
func validate(t *testing.T, m *fakechart.ChartModel) string {
	t.Helper()
	ctx := context.Background()
	state := tfsdk.State{Schema: hostSchema(), Raw: tftypes.NewValue(hostSchema().Type().TerraformType(ctx), nil)}
	if diags := state.Set(ctx, &hostModel{Chart: m}); diags.HasError() {
		t.Fatalf("set: %v", diags)
	}
	srv := providerserver.NewProtocol6(&hostProvider{})()
	if _, err := srv.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{}); err != nil {
		t.Fatal(err)
	}
	config, err := tfprotov6.NewDynamicValue(hostSchema().Type().TerraformType(ctx), state.Raw)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := srv.ValidateResourceConfig(ctx, &tfprotov6.ValidateResourceConfigRequest{TypeName: "test_host", Config: &config})
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			b.WriteString(d.Detail + " at " + d.Attribute.String() + "\n")
		}
	}
	return b.String()
}

type hostProvider struct{}

func (p *hostProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "test"
}
func (p *hostProvider) Schema(context.Context, provider.SchemaRequest, *provider.SchemaResponse) {}
func (p *hostProvider) Configure(context.Context, provider.ConfigureRequest, *provider.ConfigureResponse) {
}
func (p *hostProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{func() resource.Resource { return hostResource{} }}
}
func (p *hostProvider) DataSources(context.Context) []func() datasource.DataSource { return nil }

// hostResource has only the schema. Validation needs no CRUD.
type hostResource struct{}

func (hostResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "test_host"
}
func (hostResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = hostSchema()
}
func (hostResource) Create(context.Context, resource.CreateRequest, *resource.CreateResponse) {}
func (hostResource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse)       {}
func (hostResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {}
func (hostResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {}
