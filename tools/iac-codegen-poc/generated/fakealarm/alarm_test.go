// This file is handwritten. It checks the overrides (D21) of
// spec/fake/alarm.overrides.yaml: an enum that names the set rule
// (namesArm), and two fields with handwritten converters (custom, in
// alarmcustom).

package fakealarm_test

import (
	"context"
	"encoding/json"
	"math/big"
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

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/generated/fakealarm"
	sdk "github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/testsdk/go/openapi/gen/fake_boards_service"
)

func decode(t *testing.T, js string) *sdk.Alarm {
	t.Helper()
	var a sdk.Alarm
	if err := json.Unmarshal([]byte(js), &a); err != nil {
		t.Fatal(err)
	}
	return &a
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
func flatten(t *testing.T, js string) *fakealarm.AlarmModel {
	t.Helper()
	ctx := context.Background()
	m, diags := fakealarm.FlattenAlarm(ctx, path.Root("alarm"), decode(t, js))
	if diags.HasError() {
		t.Fatal(diags)
	}
	state := tfsdk.State{Schema: hostSchema(), Raw: tftypes.NewValue(hostSchema().Type().TerraformType(ctx), nil)}
	if diags := state.Set(ctx, &hostModel{Alarm: m}); diags.HasError() {
		t.Fatalf("set: %v", diags)
	}
	var got hostModel
	if diags := state.Get(ctx, &got); diags.HasError() {
		t.Fatalf("get: %v", diags)
	}
	return got.Alarm
}

// TestAlarmSchema checks that type is no attribute and that the custom
// fields have the handwritten attributes.
func TestAlarmSchema(t *testing.T) {
	attrs := fakealarm.AlarmAttributes()
	rule := attrs["rule"].(schema.SingleNestedAttribute).Attributes
	metric := rule["metric_rule"].(schema.SingleNestedAttribute).Attributes
	tracing := rule["tracing_rule"].(schema.SingleNestedAttribute).Attributes
	if _, ok := attrs["type"]; ok {
		t.Error("type is an attribute")
	}
	if _, ok := metric["of_the_last"].(schema.StringAttribute); !ok {
		t.Errorf("of_the_last = %T, want the handwritten StringAttribute", metric["of_the_last"])
	}
	if _, ok := tracing["latency_ms"].(schema.NumberAttribute); !ok {
		t.Errorf("latency_ms = %T, want the handwritten NumberAttribute", tracing["latency_ms"])
	}
}

// TestAlarmRoundTrip checks each rule: it reads into its arm, and expand
// sends it back with the type that names it.
func TestAlarmRoundTrip(t *testing.T) {
	for _, js := range []string{
		`{"logsRule":{"query":"q"},"name":"a","type":"ALARM_TYPE_LOGS_RULE_OR_UNSPECIFIED"}`,
		`{"metricRule":{"ofTheLast":{"specificValue":"TIME_WINDOW_VALUE_HOURS_1"},"threshold":1.5},"type":"ALARM_TYPE_METRIC_RULE"}`,
		`{"metricRule":{"ofTheLast":{"dynamicDuration":"90s"}},"type":"ALARM_TYPE_METRIC_RULE"}`,
		`{"tracingRule":{"latencyMs":"123456789012345678"},"type":"ALARM_TYPE_TRACING_RULE"}`,
		`{"name":"no rule"}`,
	} {
		m := flatten(t, js)
		out, diags := fakealarm.ExpandAlarm(context.Background(), path.Root("alarm"), m)
		if diags.HasError() {
			t.Fatal(diags)
		}
		if got, want := jsonOf(t, out), jsonOf(t, decode(t, js)); got != want {
			t.Errorf("expand:\n got %s\nwant %s", got, want)
		}
	}
	m := flatten(t, `{"metricRule":{"ofTheLast":{"specificValue":"TIME_WINDOW_VALUE_HOURS_1"}}}`)
	if got := m.Rule.MetricRule.OfTheLast.ValueString(); got != "1_HOUR" {
		t.Errorf("of_the_last = %q, want 1_HOUR", got)
	}
}

// TestAlarmTypeFromTheRule checks that expand sets type from the rule, also
// when the model came from a configuration, not from the API.
func TestAlarmTypeFromTheRule(t *testing.T) {
	m := &fakealarm.AlarmModel{Name: types.StringNull(), Rule: &fakealarm.AlarmRuleModel{
		TracingRule: &fakealarm.TracingRuleModel{LatencyMs: types.NumberNull()},
	}}
	out, diags := fakealarm.ExpandAlarm(context.Background(), path.Root("alarm"), m)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if got, want := jsonOf(t, out), `{"tracingRule":{},"type":"ALARM_TYPE_TRACING_RULE"}`; got != want {
		t.Errorf("expand = %s, want %s", got, want)
	}
}

// TestAlarmCustomErrors checks the diagnostics of the handwritten
// converters, with the paths that the generated code gives them.
func TestAlarmCustomErrors(t *testing.T) {
	ctx := context.Background()
	_, diags := fakealarm.FlattenAlarm(ctx, path.Root("alarm"), decode(t, `{"tracingRule":{"latencyMs":"1.5"}}`))
	want := path.Root("alarm").AtName("rule").AtName("tracing_rule").AtName("latency_ms")
	if !diags.HasError() || !diags[0].(interface{ Path() path.Path }).Path().Equal(want) {
		t.Errorf("flatten of latencyMs 1.5: %v, want an error at %v", diags, want)
	}
	m := flatten(t, `{"tracingRule":{"latencyMs":"5"}}`)
	m.Rule.TracingRule.LatencyMs = types.NumberValue(mustFloat("2.5"))
	if _, diags := fakealarm.ExpandAlarm(ctx, path.Root("alarm"), m); !diags.HasError() || !strings.Contains(diags[0].Detail(), "2.5 is not a whole number") {
		t.Errorf("expand of latency_ms 2.5: %v", diags)
	}
}

// TestAlarmValidators runs the generated oneOf validators and the
// handwritten validator of of_the_last through a provider server.
func TestAlarmValidators(t *testing.T) {
	for _, c := range []struct {
		name string
		edit func(m *fakealarm.AlarmModel)
		want string // "" for no error
	}{
		{"valid", func(*fakealarm.AlarmModel) {}, ""},
		{"a fixed window", func(m *fakealarm.AlarmModel) { m.Rule.MetricRule.OfTheLast = types.StringValue("5_MINUTES") }, ""},
		{"a bad window", func(m *fakealarm.AlarmModel) { m.Rule.MetricRule.OfTheLast = types.StringValue("soon") }, `AttributeName("of_the_last")`},
		{"two rules", func(m *fakealarm.AlarmModel) {
			m.Rule.LogsRule = &fakealarm.LogsRuleModel{Query: types.StringValue("q")}
		}, `AttributeName("logs_rule")`},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := flatten(t, `{"metricRule":{"ofTheLast":{"dynamicDuration":"90s"}}}`)
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

func mustFloat(s string) *big.Float {
	f, ok := new(big.Float).SetString(s)
	if !ok {
		panic(s)
	}
	return f
}

func hostSchema() schema.Schema {
	return schema.Schema{Attributes: map[string]schema.Attribute{
		"alarm": schema.SingleNestedAttribute{Required: true, Attributes: fakealarm.AlarmAttributes()},
	}}
}

type hostModel struct {
	Alarm *fakealarm.AlarmModel `tfsdk:"alarm"`
}

// validate returns the validation errors of a configuration with m, and
// their paths.
func validate(t *testing.T, m *fakealarm.AlarmModel) string {
	t.Helper()
	ctx := context.Background()
	state := tfsdk.State{Schema: hostSchema(), Raw: tftypes.NewValue(hostSchema().Type().TerraformType(ctx), nil)}
	if diags := state.Set(ctx, &hostModel{Alarm: m}); diags.HasError() {
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
