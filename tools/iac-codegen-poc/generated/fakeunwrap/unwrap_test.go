// This file is handwritten. It checks the unwrap overrides (D21) of
// spec/fake/query.overrides.yaml: API objects with one field, value, that
// Terraform shows as that field.

package fakeunwrap_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
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

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/generated/fakeunwrap"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/schemadump"
	sdk "github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/testsdk/go/openapi/gen/fake_boards_service"
)

const full = `{"color":{"value":"RED"},"field":{"value":{"keypath":["a","b"],"scope":"user"}},` +
	`"labels":{"value":["x"]},"series":[{"value":{"scope":"s1"}},{"value":{"scope":"s2"}}],` +
	`"source":{"luceneQuery":{"value":"status:500"}},"widgetIds":[{"value":"00000000-0000-0000-0000-000000000001"}]}`

func decode(t *testing.T, js string) *sdk.Query {
	t.Helper()
	var q sdk.Query
	if err := json.Unmarshal([]byte(js), &q); err != nil {
		t.Fatal(err)
	}
	return &q
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
func flatten(t *testing.T, js string) *fakeunwrap.QueryModel {
	t.Helper()
	ctx := context.Background()
	m, diags := fakeunwrap.FlattenQuery(ctx, path.Root("query"), decode(t, js))
	if diags.HasError() {
		t.Fatal(diags)
	}
	state := tfsdk.State{Schema: hostSchema(), Raw: tftypes.NewValue(hostSchema().Type().TerraformType(ctx), nil)}
	if diags := state.Set(ctx, &hostModel{Query: m}); diags.HasError() {
		t.Fatalf("set: %v", diags)
	}
	var got hostModel
	if diags := state.Get(ctx, &got); diags.HasError() {
		t.Fatalf("get: %v", diags)
	}
	return got.Query
}

// TestUnwrapRoundTrip checks that each value object is read as its value
// and sent back as the same object.
func TestUnwrapRoundTrip(t *testing.T) {
	m := flatten(t, full)
	switch {
	case m.Source.LuceneQuery.ValueString() != "status:500" || !m.Source.PromqlQuery.IsNull():
		t.Errorf("source = %+v", m.Source)
	case m.Color.ValueString() != "RED":
		t.Errorf("color = %v", m.Color)
	case m.Field == nil || m.Field.Scope.ValueString() != "user":
		t.Errorf("field = %+v", m.Field)
	case len(m.WidgetIds.Elements()) != 1 || len(m.Labels.Elements()) != 1 || len(m.Series.Elements()) != 2:
		t.Errorf("widget_ids %v, labels %v, series %v", m.WidgetIds, m.Labels, m.Series)
	}
	out, diags := fakeunwrap.ExpandQuery(context.Background(), path.Root("query"), m)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if got, want := jsonOf(t, out), jsonOf(t, decode(t, full)); got != want {
		t.Errorf("expand:\n got %s\nwant %s", got, want)
	}
}

// TestUnwrapReads checks that a missing object and an object without a value
// read the same, with the read overrides of the value.
func TestUnwrapReads(t *testing.T) {
	for _, c := range []struct {
		name, json string
		check      func(m *fakeunwrap.QueryModel) bool
	}{
		{"missing objects are null", `{"field":{}}`, func(m *fakeunwrap.QueryModel) bool {
			return m.Source == nil && m.Field == nil && m.Color.IsNull() && m.Labels.IsNull() && m.WidgetIds.IsNull() && m.Series.IsNull()
		}},
		{"an object without a value is null", `{"field":{},"source":{"luceneQuery":{}},"color":{},"labels":{}}`, func(m *fakeunwrap.QueryModel) bool {
			return m.Source.LuceneQuery.IsNull() && m.Color.IsNull() && m.Labels.IsNull()
		}},
		{"emptyAsNull on the value", `{"field":{},"labels":{"value":[]}}`, func(m *fakeunwrap.QueryModel) bool {
			return m.Labels.IsNull()
		}},
		{"missingAsZero on the value", `{"field":{},"widgetIds":[{}]}`, func(m *fakeunwrap.QueryModel) bool {
			return m.WidgetIds.Equal(strSet(""))
		}},
		{"an empty value is a value", `{"field":{},"source":{"luceneQuery":{"value":""}}}`, func(m *fakeunwrap.QueryModel) bool {
			return !m.Source.LuceneQuery.IsNull() && m.Source.LuceneQuery.ValueString() == ""
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if m := flatten(t, c.json); !c.check(m) {
				t.Errorf("%s read as %+v", c.json, m)
			}
		})
	}
}

// TestUnwrapExpand checks that a null value sends no object, and that "" and
// an empty set are sent as values.
func TestUnwrapExpand(t *testing.T) {
	for _, c := range []struct {
		name string
		m    *fakeunwrap.QueryModel
		want string
	}{
		{"null", nullQuery(), `{"field":{}}`},
		{"empty values", func() *fakeunwrap.QueryModel {
			m := nullQuery()
			m.Source = &fakeunwrap.QuerySourceModel{LuceneQuery: types.StringValue(""), PromqlQuery: types.StringNull()}
			m.Labels = strSet()
			return m
		}(), `{"field":{},"labels":{"value":[]},"source":{"luceneQuery":{"value":""}}}`},
		{"unknown", func() *fakeunwrap.QueryModel {
			m := nullQuery()
			m.Color = types.StringUnknown()
			return m
		}(), `{"field":{}}`},
	} {
		t.Run(c.name, func(t *testing.T) {
			out, diags := fakeunwrap.ExpandQuery(context.Background(), path.Root("query"), c.m)
			if diags.HasError() {
				t.Fatal(diags)
			}
			if got := jsonOf(t, out); got != c.want {
				t.Errorf("expand = %s, want %s", got, c.want)
			}
		})
	}
}

func nullQuery() *fakeunwrap.QueryModel {
	return &fakeunwrap.QueryModel{
		WidgetIds: types.SetNull(types.StringType),
		Labels:    types.SetNull(types.StringType),
		Color:     types.StringNull(),
		Series:    types.ListNull(types.ObjectType{AttrTypes: fakeunwrap.ObservationFieldAttrTypes()}),
	}
}

// TestUnwrapValidators runs the validators of the values, and the oneOf
// validators on the unwrapped arms, through a provider server.
func TestUnwrapValidators(t *testing.T) {
	for _, c := range []struct {
		name string
		edit func(m *fakeunwrap.QueryModel)
		want string // "" for no error
	}{
		{"valid", func(*fakeunwrap.QueryModel) {}, ""},
		{"two arms", func(m *fakeunwrap.QueryModel) { m.Source.PromqlQuery = types.StringValue("up") },
			`AttributeName("source").AttributeName("`},
		{"empty promql", func(m *fakeunwrap.QueryModel) {
			m.Source.LuceneQuery, m.Source.PromqlQuery = types.StringNull(), types.StringValue("")
		}, `AttributeName("promql_query")`},
		{"short widget id", func(m *fakeunwrap.QueryModel) {
			m.WidgetIds = strSet("w1")
		}, `AttributeName("widget_ids")`},
		{"no field", func(m *fakeunwrap.QueryModel) { m.Field = nil }, `AttributeName("field")`},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := flatten(t, full)
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

// TestUnwrapPlanWithUnknowns checks that an update plan with the computed
// color unknown reads into the model and expands without the color.
func TestUnwrapPlanWithUnknowns(t *testing.T) {
	ctx := context.Background()
	state := tfsdk.State{Schema: hostSchema(), Raw: tftypes.NewValue(hostSchema().Type().TerraformType(ctx), nil)}
	if diags := state.Set(ctx, &hostModel{Query: flatten(t, full)}); diags.HasError() {
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
	if !plan.Query.Color.IsUnknown() {
		t.Errorf("color = %v, want unknown", plan.Query.Color)
	}
	out, diags := fakeunwrap.ExpandQuery(ctx, path.Root("query"), plan.Query)
	if diags.HasError() {
		t.Fatalf("expand: %v", diags)
	}
	if out.Color != nil {
		t.Errorf("color = %v, want not sent", out.Color)
	}
}

// strSet is a set of strings.
func strSet(vs ...string) types.Set {
	elems := make([]attr.Value, 0, len(vs))
	for _, v := range vs {
		elems = append(elems, types.StringValue(v))
	}
	return types.SetValueMust(types.StringType, elems)
}

func hostSchema() schema.Schema {
	return schema.Schema{Attributes: map[string]schema.Attribute{
		"query": schema.SingleNestedAttribute{Required: true, Attributes: fakeunwrap.QueryAttributes()},
	}}
}

type hostModel struct {
	Query *fakeunwrap.QueryModel `tfsdk:"query"`
}

// validate returns the validation errors of a configuration with m, and
// their paths.
func validate(t *testing.T, m *fakeunwrap.QueryModel) string {
	t.Helper()
	ctx := context.Background()
	state := tfsdk.State{Schema: hostSchema(), Raw: tftypes.NewValue(hostSchema().Type().TerraformType(ctx), nil)}
	if diags := state.Set(ctx, &hostModel{Query: m}); diags.HasError() {
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
