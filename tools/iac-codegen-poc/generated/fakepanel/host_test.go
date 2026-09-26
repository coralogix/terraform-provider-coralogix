// This file is handwritten. It is a small handwritten resource that embeds
// the generated types (D20), as the handwritten dashboard resource would: a
// single nested header, a map of panels, and an interval. It uses only the
// exported names, from another package.

package fakepanel_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	sdk "github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/fakesdk/go/openapi/gen/fake_boards_service"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/generated/fakepanel"
)

// hostModel is the model of the handwritten host resource.
type hostModel struct {
	Header   *fakepanel.HeaderModel       `tfsdk:"header"`
	Panels   types.Map                    `tfsdk:"panels"`
	Interval *fakepanel.IntervalModel     `tfsdk:"interval"`
	Range    *fakepanel.AbsoluteTimeModel `tfsdk:"range"`
}

func hostSchema() schema.Schema {
	return schema.Schema{Attributes: map[string]schema.Attribute{
		"header": schema.SingleNestedAttribute{Optional: true, Attributes: fakepanel.HeaderAttributes()},
		"panels": schema.MapNestedAttribute{Optional: true,
			NestedObject: schema.NestedAttributeObject{Attributes: fakepanel.PanelAttributes()}},
		"interval": schema.SingleNestedAttribute{Optional: true, Attributes: fakepanel.IntervalAttributes()},
		"range":    rangeAttribute(),
	}}
}

func ptr[T any](v T) *T { return &v }

// sdkValues are API values with every shape: a oneOf arm with no fields, a
// oneOf arm with fields, a discriminator, a map, a list of oneOf, a uint64
// string, int32 zero, and float32.
func sdkValues() (*sdk.Header, *sdk.Panel, *sdk.Interval) {
	header := &sdk.Header{Text: "Errors", Color: ptr(sdk.COLOR_RED), Style: &sdk.TextStyle{Bold: map[string]interface{}{}}}
	panel := &sdk.Panel{
		Query:      "level:error",
		Precision:  ptr(int32(0)),
		Thresholds: map[string]float64{"warn": 0.5, "crit": 0.9},
		Unit:       ptr(sdk.UNIT_PERCENT),
		Sort:       &sdk.SortStrategy{ByValue: &sdk.Every{Minutes: ptr(int32(5))}, StrategyType: ptr("BY_VALUE")},
		Style:      &sdk.TextStyle{Font: &sdk.FontStyle{Family: ptr("mono"), Scale: ptr(float32(0.1)), Size: ptr("12")}},
		Filters: []sdk.Filter{
			{Equals: &sdk.Match{Field: "level", Value: ptr("error")}},
			{Contains: &sdk.Match{Field: "text", Value: ptr("timeout")}},
		},
	}
	interval := &sdk.Interval{Manual: &sdk.Every{Minutes: ptr(int32(1))}, UseLimit: ptr(false)}
	return header, panel, interval
}

func jsonOf(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestRoundTrip checks that flatten then expand gives the same API values.
func TestRoundTrip(t *testing.T) {
	ctx := context.Background()
	header, panel, interval := sdkValues()

	hm, diags := fakepanel.FlattenHeader(ctx, path.Root("header"), header)
	h2, d := fakepanel.ExpandHeader(ctx, path.Root("header"), hm)
	diags.Append(d...)
	pm, d := fakepanel.FlattenPanel(ctx, path.Root("panels").AtMapKey("a"), panel)
	diags.Append(d...)
	p2, d := fakepanel.ExpandPanel(ctx, path.Root("panels").AtMapKey("a"), pm)
	diags.Append(d...)
	im, d := fakepanel.FlattenInterval(ctx, path.Root("interval"), interval)
	diags.Append(d...)
	i2, d := fakepanel.ExpandInterval(ctx, path.Root("interval"), im)
	diags.Append(d...)
	if diags.HasError() {
		t.Fatalf("diagnostics: %v", diags)
	}
	for _, c := range []struct{ name, got, want string }{
		{"header", jsonOf(t, h2), jsonOf(t, header)},
		{"panel", jsonOf(t, p2), jsonOf(t, panel)},
		{"interval", jsonOf(t, i2), jsonOf(t, interval)},
	} {
		if c.got != c.want {
			t.Errorf("%s: got %s\nwant %s", c.name, c.got, c.want)
		}
	}
	if !pm.Precision.Equal(types.Int32Value(0)) || !im.UseLimit.Equal(types.BoolValue(false)) {
		t.Errorf("zero values: precision %s, use_limit %s; want 0 and false, not null", pm.Precision, im.UseLimit)
	}
}

// TestHostState checks that the models and the AttrTypes fit the embedded
// schema: a state with every value set is valid and reads back the same.
func TestHostState(t *testing.T) {
	ctx := context.Background()
	m := validModel(t)
	s := toState(t, m)
	var back hostModel
	if diags := s.Get(ctx, &back); diags.HasError() {
		t.Fatalf("get: %v", diags)
	}
	if !back.Panels.Equal(m.Panels) {
		t.Errorf("panels differ after the state:\n%s\n%s", back.Panels, m.Panels)
	}
}

func validModel(t *testing.T) *hostModel {
	t.Helper()
	ctx := context.Background()
	header, panel, interval := sdkValues()
	hm, diags := fakepanel.FlattenHeader(ctx, path.Root("header"), header)
	pm, d := fakepanel.FlattenPanel(ctx, path.Root("panels").AtMapKey("a"), panel)
	diags.Append(d...)
	im, d := fakepanel.FlattenInterval(ctx, path.Root("interval"), interval)
	diags.Append(d...)
	panels, d := types.MapValueFrom(ctx, types.ObjectType{AttrTypes: fakepanel.PanelAttrTypes()}, map[string]fakepanel.PanelModel{"a": *pm})
	diags.Append(d...)
	if diags.HasError() {
		t.Fatalf("diagnostics: %v", diags)
	}
	return &hostModel{Header: hm, Panels: panels, Interval: im}
}

func toState(t *testing.T, m *hostModel) tfsdk.State {
	t.Helper()
	ctx := context.Background()
	s := tfsdk.State{Schema: hostSchema(), Raw: tftypes.NewValue(hostSchema().Type().TerraformType(ctx), nil)}
	if diags := s.Set(ctx, m); diags.HasError() {
		t.Fatalf("set state: %v", diags)
	}
	return s
}

// TestValidators runs the generated oneOf validators through a provider
// server, inside a single nested attribute and inside a map value. The
// handwritten schema wires no validator.
func TestValidators(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		edit func(m *hostModel)
		want string // "" for no error
	}{
		{"valid", func(*hostModel) {}, ""},
		{"no interval: its validators do not run", func(m *hostModel) { m.Interval = nil }, ""},
		{"two style arms", func(m *hostModel) {
			m.Header.Style.Font = &fakepanel.FontStyleModel{Family: types.StringValue("mono"),
				Scale: types.Float32Null(), Size: types.Int64Null()}
		}, "header.style"},
		{"no interval arm, one is required", func(m *hostModel) { m.Interval.Manual = nil }, "interval"},
		{"two interval arms", func(m *hostModel) { m.Interval.Auto = &fakepanel.BoldStyleModel{} }, "interval"},
		{"two sort arms in a map value", func(m *hostModel) {
			var panels map[string]fakepanel.PanelModel
			m.Panels.ElementsAs(ctx, &panels, false)
			p := panels["a"]
			p.Sort.ByName = &fakepanel.BoldStyleModel{}
			panels["a"] = p
			m.Panels, _ = types.MapValueFrom(ctx, types.ObjectType{AttrTypes: fakepanel.PanelAttrTypes()}, panels)
		}, `ElementKeyString("a").AttributeName("sort")`},
		{"two arms in a list item of a map value", func(m *hostModel) {
			var panels map[string]fakepanel.PanelModel
			m.Panels.ElementsAs(ctx, &panels, false)
			p := panels["a"]
			var filters []fakepanel.FilterModel
			p.Filters.ElementsAs(ctx, &filters, false)
			filters[1].Equals = &fakepanel.MatchModel{Field: types.StringValue("x"), Value: types.StringNull()}
			p.Filters, _ = types.ListValueFrom(ctx, types.ObjectType{AttrTypes: fakepanel.FilterAttrTypes()}, filters)
			panels["a"] = p
			m.Panels, _ = types.MapValueFrom(ctx, types.ObjectType{AttrTypes: fakepanel.PanelAttrTypes()}, panels)
		}, `AttributeName("filters").ElementKeyInt(1)`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := validModel(t)
			c.edit(m)
			got := diagText(validate(t, m))
			t.Logf("errors: %s", got)
			switch {
			case c.want == "" && got != "":
				t.Errorf("errors: %s", got)
			case c.want != "" && !strings.Contains(got, c.want):
				t.Errorf("errors = %q, want one at %s", got, c.want)
			}
		})
	}
}

// TestDiagnosticPath checks that an expand error has the path that the
// handwritten code passes, and the path inside the type.
func TestDiagnosticPath(t *testing.T) {
	m := validModel(t)
	m.Header.Style = &fakepanel.TextStyleModel{Font: &fakepanel.FontStyleModel{
		Family: types.StringNull(), Scale: types.Float32Null(), Size: types.Int64Value(-1)}}
	_, diags := fakepanel.ExpandHeader(context.Background(), path.Root("header"), m.Header)
	if got := diagText(diags); !strings.Contains(got, "header.style.font.size") {
		t.Errorf("diagnostics = %q, want the path header.style.font.size", got)
	}
}

func validate(t *testing.T, m *hostModel) diag.Diagnostics {
	t.Helper()
	ctx := context.Background()
	srv := providerserver.NewProtocol6(&hostProvider{})()
	if resp, err := srv.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{}); err != nil || len(resp.Diagnostics) != 0 {
		t.Fatalf("provider schema: %v %v", err, resp.Diagnostics)
	}
	config, err := tfprotov6.NewDynamicValue(hostSchema().Type().TerraformType(ctx), toState(t, m).Raw)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := srv.ValidateResourceConfig(ctx, &tfprotov6.ValidateResourceConfigRequest{TypeName: "test_host", Config: &config})
	if err != nil {
		t.Fatal(err)
	}
	var diags diag.Diagnostics
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			diags.AddAttributeError(path.Empty(), d.Summary, d.Detail+" at "+d.Attribute.String())
		}
	}
	return diags
}

func diagText(diags diag.Diagnostics) string {
	var b strings.Builder
	for _, d := range diags {
		b.WriteString(d.Summary() + " " + d.Detail())
		if p, ok := d.(diag.DiagnosticWithPath); ok && !p.Path().Equal(path.Empty()) {
			b.WriteString(" at " + p.Path().String())
		}
		b.WriteString("\n")
	}
	return b.String()
}

// hostProvider serves only the host resource.
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
