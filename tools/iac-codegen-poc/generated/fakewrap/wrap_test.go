// This file is handwritten. It checks the wrapper overrides (D21) of
// spec/fake/layout.overrides.yaml: fields of the API type Layout in new
// Terraform objects, with the oneOf validators inside them.

package fakewrap_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	sdk "github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/fakesdk/go/openapi/gen/fake_boards_service"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/generated/fakewrap"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/schemadump"
)

func ptr[T any](v T) *T { return &v }

func jsonOf(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestWrapRoundTrip checks that the wrapped fields are read into the
// wrappers and sent back on the same API object.
func TestWrapRoundTrip(t *testing.T) {
	ctx := context.Background()
	in := &sdk.Layout{
		Title:        "Board",
		RefreshEvery: &sdk.Every{Minutes: ptr(int32(5))},
		AbsoluteTime: &sdk.AbsoluteTime{From: ptr(mustTime(t, "2026-09-26T08:00:00Z"))},
	}
	m, diags := fakewrap.FlattenLayout(ctx, path.Root("layout"), in)
	if diags.HasError() {
		t.Fatal(diags)
	}
	switch {
	case m.Refresh == nil || m.Refresh.RefreshEvery == nil || m.Refresh.RefreshOff != nil:
		t.Errorf("refresh = %+v, want refresh_every only", m.Refresh)
	case m.TimeRange == nil || m.TimeRange.AbsoluteTime == nil:
		t.Errorf("time_range = %+v, want absolute_time", m.TimeRange)
	case m.Heading == nil || m.Heading.Title.ValueString() != "Board":
		t.Errorf("heading = %+v, want the title", m.Heading)
	}
	out, diags := fakewrap.ExpandLayout(ctx, path.Root("layout"), m)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if got, want := jsonOf(t, out), jsonOf(t, in); got != want {
		t.Errorf("expand:\n got %s\nwant %s", got, want)
	}
}

// TestWrapNull checks that a wrapper is null when the API has none of its
// fields, and that heading, which wraps an SDK value, is never null.
func TestWrapNull(t *testing.T) {
	m, diags := fakewrap.FlattenLayout(context.Background(), path.Root("layout"), &sdk.Layout{})
	if diags.HasError() {
		t.Fatal(diags)
	}
	if m.Refresh != nil || m.TimeRange != nil || m.Heading == nil {
		t.Errorf("refresh %v, time_range %v, heading %v: want nil, nil, set", m.Refresh, m.TimeRange, m.Heading)
	}
}

// TestWrapValidators runs the oneOf validators inside the wrappers through a
// provider server.
func TestWrapValidators(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		name string
		edit func(m *fakewrap.LayoutModel)
		want string // "" for no error
	}{
		{"valid", func(*fakewrap.LayoutModel) {}, ""},
		{"two refresh arms", func(m *fakewrap.LayoutModel) { m.Refresh.RefreshOff = &fakewrap.BoldStyleModel{} },
			`AttributeName("refresh").AttributeName("refresh`},
		{"no time_range arm", func(m *fakewrap.LayoutModel) { m.TimeRange.AbsoluteTime = nil },
			`AttributeName("time_range").AttributeName("`},
		{"no time_range", func(m *fakewrap.LayoutModel) { m.TimeRange = nil }, `AttributeName("time_range")`},
	} {
		t.Run(c.name, func(t *testing.T) {
			m, diags := fakewrap.FlattenLayout(ctx, path.Root("layout"), &sdk.Layout{
				Title:        "Board",
				RefreshEvery: &sdk.Every{Minutes: ptr(int32(5))},
				AbsoluteTime: &sdk.AbsoluteTime{From: ptr(mustTime(t, "2026-09-26T08:00:00Z"))},
			})
			if diags.HasError() {
				t.Fatal(diags)
			}
			c.edit(m)
			got := validate(t, m)
			switch {
			case c.want == "" && got != "":
				t.Errorf("errors: %s", got)
			case c.want != "" && !strings.Contains(got, c.want):
				t.Errorf("errors = %q, want one at %s", got, c.want)
			}
		})
	}
}

func hostSchema() schema.Schema {
	return schema.Schema{Attributes: map[string]schema.Attribute{
		"layout": schema.SingleNestedAttribute{Required: true, Attributes: fakewrap.LayoutAttributes()},
	}}
}

type hostModel struct {
	Layout *fakewrap.LayoutModel `tfsdk:"layout"`
}

// validate returns the validation errors of a configuration with m, and
// their paths.
func validate(t *testing.T, m *fakewrap.LayoutModel) string {
	t.Helper()
	ctx := context.Background()
	state := tfsdk.State{Schema: hostSchema(), Raw: tftypes.NewValue(hostSchema().Type().TerraformType(ctx), nil)}
	if diags := state.Set(ctx, &hostModel{Layout: m}); diags.HasError() {
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
			b.WriteString(d.Summary + " at " + d.Attribute.String() + "\n")
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

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// TestReadOnlyTree checks that every attribute inside the read-only section
// is computed only, and that the text keeps a non-null state value.
func TestReadOnlyTree(t *testing.T) {
	var walk func(p string, attrs map[string]schema.Attribute)
	walk = func(p string, attrs map[string]schema.Attribute) {
		for name, a := range attrs {
			if a.IsRequired() || a.IsOptional() || !a.IsComputed() {
				t.Errorf("%s%s: required %v, optional %v, computed %v; want computed only", p, name, a.IsRequired(), a.IsOptional(), a.IsComputed())
			}
			if n, ok := a.(schema.SingleNestedAttribute); ok {
				walk(p+name+".", n.Attributes)
			}
			if n, ok := a.(schema.ListNestedAttribute); ok {
				walk(p+name+"[].", n.NestedObject.Attributes)
			}
		}
	}
	section := fakewrap.LayoutAttributes()["section"].(schema.SingleNestedAttribute)
	if !section.IsComputed() || section.IsOptional() {
		t.Error("section is not computed only")
	}
	walk("section.", section.Attributes)
	text := fakewrap.LayoutAttributes()["heading"].(schema.SingleNestedAttribute).Attributes["text"].(schema.StringAttribute)
	if len(text.PlanModifiers) != 1 || !strings.Contains(text.PlanModifiers[0].Description(context.Background()), "non-null") {
		t.Errorf("heading.text plan modifiers = %v, want UseNonNullStateForUnknown", text.PlanModifiers)
	}
}

// TestPlanWithUnknowns checks that an update plan with every computed
// attribute unknown reads into the model and expands. section is a read-only
// object, so its model field must be a types.Object.
func TestPlanWithUnknowns(t *testing.T) {
	ctx := context.Background()
	m, diags := fakewrap.FlattenLayout(ctx, path.Root("layout"), &sdk.Layout{
		Title:        "Board",
		AbsoluteTime: &sdk.AbsoluteTime{From: ptr(mustTime(t, "2026-09-26T08:00:00Z"))},
		Section:      &sdk.Section{Columns: []int32{1}},
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	state := tfsdk.State{Schema: hostSchema(), Raw: tftypes.NewValue(hostSchema().Type().TerraformType(ctx), nil)}
	if diags := state.Set(ctx, &hostModel{Layout: m}); diags.HasError() {
		t.Fatal(diags)
	}
	raw, err := schemadump.PlanWithUnknowns(ctx, hostSchema(), state.Raw)
	if err != nil {
		t.Fatal(err)
	}
	var plan hostModel
	if diags := (tfsdk.Plan{Schema: hostSchema(), Raw: raw}).Get(ctx, &plan); diags.HasError() {
		t.Fatalf("get plan: %v", diags)
	}
	if !plan.Layout.Section.IsUnknown() {
		t.Errorf("section = %v, want unknown", plan.Layout.Section)
	}
	if _, diags := fakewrap.ExpandLayout(ctx, path.Root("layout"), plan.Layout); diags.HasError() {
		t.Fatalf("expand: %v", diags)
	}
}
