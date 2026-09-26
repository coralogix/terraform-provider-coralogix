// This file is handwritten. It checks the overrides (D21): with
// spec/fake/routing.overrides.yaml, the generated Routing types have the
// schema of a handwritten resource that differs from the API, and they
// convert the API values to that schema and back.

package fakerouting_test

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/generated/fakerouting"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/schemadump"
	sdk "github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/testsdk/go/openapi/gen/fake_boards_service"
)

// handwrittenAttributes is the schema that a handwritten resource would have
// for Routing. It differs from the API: other names (name, id), a required
// field, defaults, sets instead of lists, Int64 and Float64 instead of int32
// and float, and server fields: one keeps its state value.
func handwrittenAttributes() map[string]schema.Attribute {
	deliveries := []string{"disabled", "errors", "unspecified"}
	return map[string]schema.Attribute{
		"name": schema.StringAttribute{Required: true,
			Validators: []validator.String{stringvalidator.LengthBetween(1, 50)}},
		"delivery": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("unspecified"),
			Validators: []validator.String{stringvalidator.OneOf(deliveries...)}},
		"channels": schema.SetAttribute{Optional: true, ElementType: types.StringType,
			Validators: []validator.Set{setvalidator.ValueStringsAre(stringvalidator.OneOf(deliveries...))}},
		"targets": schema.SetNestedAttribute{Optional: true, NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"connector_id": schema.StringAttribute{Required: true},
				"tags":         schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType},
			}}},
		"priority": schema.Int64Attribute{Optional: true,
			PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}},
		"weight":   schema.Float64Attribute{Optional: true},
		"disabled": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
		"id": schema.StringAttribute{Computed: true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"create_time": schema.StringAttribute{Computed: true},
	}
}

// TestSchemaMatchesHandwritten checks that the generated schema is the
// handwritten one: same names, kinds, flags, defaults, validators, and plan
// modifiers.
func TestSchemaMatchesHandwritten(t *testing.T) {
	dump := func(attrs map[string]schema.Attribute) []schemadump.Entry {
		return schemadump.Dump(schema.Schema{Attributes: attrs}, nil)
	}
	if d := schemadump.Diff(dump(handwrittenAttributes()), dump(fakerouting.RoutingAttributes())); d != "" {
		t.Errorf("generated schema differs from the handwritten one:\n%s", d)
	}
}

func ptr[T any](v T) *T { return &v }

func sdkRouting() *sdk.Routing {
	return &sdk.Routing{
		RoutingName: ptr("on-call"),
		Delivery:    ptr(sdk.DELIVERY_ERRORS_ONLY),
		Channels:    []sdk.Delivery{sdk.DELIVERY_DISABLED, sdk.DELIVERY_DELIVERY_UNSPECIFIED},
		Targets:     []sdk.Target{{ConnectorId: ptr("c1"), Tags: []string{"a", "b"}}, {ConnectorId: ptr("c2")}},
		Priority:    ptr(int32(7)),
		Weight:      ptr(float32(0.1)),
		Disabled:    ptr(false),
		RouterId:    ptr("r1"),
		CreateTime:  ptr(time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)),
	}
}

func jsonOf(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestRoundTrip checks the Terraform values of the overrides, and that
// expand gives back the API values. The spec marks create_time readOnly:
// it is read, and not sent.
func TestRoundTrip(t *testing.T) {
	ctx := context.Background()
	in := sdkRouting()
	m, diags := fakerouting.FlattenRouting(ctx, path.Root("routing"), in)
	if diags.HasError() {
		t.Fatalf("flatten: %v", diags)
	}
	channels, _ := types.SetValueFrom(ctx, types.StringType, []string{"disabled", "unspecified"})
	for _, c := range []struct {
		name      string
		got, want attr.Value
	}{
		{"name", m.RoutingName, types.StringValue("on-call")},
		{"delivery", m.Delivery, types.StringValue("errors")},
		{"channels", m.Channels, channels},
		{"priority", m.Priority, types.Int64Value(7)},
		{"weight", m.Weight, types.Float64Value(0.1)},
		{"disabled", m.Disabled, types.BoolValue(false)},
		{"id", m.RouterId, types.StringValue("r1")},
		{"create_time", m.CreateTime, types.StringValue("2026-09-26T08:00:00Z")},
	} {
		if !c.got.Equal(c.want) {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
	out, diags := fakerouting.ExpandRouting(ctx, path.Root("routing"), m)
	if diags.HasError() {
		t.Fatalf("expand: %v", diags)
	}
	want := sdkRouting()
	want.CreateTime = nil
	// missingAsZero: the missing tags are an empty set in the state, so
	// expand sends them as [].
	want.Targets[1].Tags = []string{}
	if got := jsonOf(t, out); got != jsonOf(t, want) {
		t.Errorf("expand:\n got %s\nwant %s", got, jsonOf(t, want))
	}
}

// TestState checks that the models and the attribute types fit the schema:
// a state with the flattened values is valid and reads back the same.
func TestState(t *testing.T) {
	ctx := context.Background()
	s := schema.Schema{Attributes: map[string]schema.Attribute{
		"routing": schema.SingleNestedAttribute{Optional: true, Attributes: fakerouting.RoutingAttributes()},
	}}
	type host struct {
		Routing *fakerouting.RoutingModel `tfsdk:"routing"`
	}
	m, diags := fakerouting.FlattenRouting(ctx, path.Root("routing"), sdkRouting())
	if diags.HasError() {
		t.Fatal(diags)
	}
	state := tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}
	if diags := state.Set(ctx, &host{Routing: m}); diags.HasError() {
		t.Fatalf("set state: %v", diags)
	}
	var back host
	if diags := state.Get(ctx, &back); diags.HasError() {
		t.Fatalf("get state: %v", diags)
	}
	if !back.Routing.Targets.Equal(m.Targets) || !back.Routing.Channels.Equal(m.Channels) {
		t.Errorf("sets differ after the state:\n%s\n%s", back.Routing.Targets, m.Targets)
	}
}

// TestEnumValues checks the enum names both ways: the zero value has a name,
// an unknown API value is an error, and a name that is not valid is an error.
func TestEnumValues(t *testing.T) {
	ctx := context.Background()
	in := &sdk.Routing{Delivery: ptr(sdk.DELIVERY_DELIVERY_UNSPECIFIED)}
	m, diags := fakerouting.FlattenRouting(ctx, path.Root("routing"), in)
	if diags.HasError() || m.Delivery.ValueString() != "unspecified" {
		t.Errorf("zero value: %s %v, want \"unspecified\"", m.Delivery, diags)
	}
	_, diags = fakerouting.FlattenRouting(ctx, path.Root("routing"), &sdk.Routing{Delivery: ptr(sdk.Delivery("NEW_VALUE"))})
	if got := diagsText(diags); !strings.Contains(got, `"NEW_VALUE"`) || !strings.Contains(got, "routing.delivery") {
		t.Errorf("unknown API value: diagnostics = %q", got)
	}
	m.Delivery = types.StringValue("ERRORS_ONLY")
	if _, diags := fakerouting.ExpandRouting(ctx, path.Root("routing"), m); !strings.Contains(diagsText(diags), "routing.delivery") {
		t.Errorf("API value as a name: diagnostics = %q", diagsText(diags))
	}
}

// TestWideNumbers checks that an Int64 value out of the int32 range is an
// error with its path, and that the limits themselves pass.
func TestWideNumbers(t *testing.T) {
	ctx := context.Background()
	m, _ := fakerouting.FlattenRouting(ctx, path.Root("routing"), sdkRouting())
	for _, n := range []int64{math.MinInt32, math.MaxInt32} {
		m.Priority = types.Int64Value(n)
		out, diags := fakerouting.ExpandRouting(ctx, path.Root("routing"), m)
		if diags.HasError() || int64(*out.Priority) != n {
			t.Errorf("%d: %v", n, diags)
		}
	}
	m.Priority = types.Int64Value(math.MaxInt32 + 1)
	if _, diags := fakerouting.ExpandRouting(ctx, path.Root("routing"), m); !strings.Contains(diagsText(diags), "routing.priority") {
		t.Errorf("out of range: diagnostics = %q", diagsText(diags))
	}
}

func diagsText(diags diag.Diagnostics) string {
	var b strings.Builder
	for _, d := range diags {
		b.WriteString(d.Summary() + " " + d.Detail())
		if p, ok := d.(diag.DiagnosticWithPath); ok {
			b.WriteString(" at " + p.Path().String())
		}
		b.WriteString("\n")
	}
	return b.String()
}

// TestReadRules checks missingAsZero and emptyAsNull: a missing weight
// reads as 0, a missing delivery as its proto zero value, missing tags as an
// empty set, and empty channels as null.
func TestReadRules(t *testing.T) {
	ctx := context.Background()
	in := &sdk.Routing{Channels: []sdk.Delivery{}, Targets: []sdk.Target{{ConnectorId: ptr("c1")}}}
	m, diags := fakerouting.FlattenRouting(ctx, path.Root("routing"), in)
	if diags.HasError() {
		t.Fatal(diags)
	}
	var targets []fakerouting.TargetModel
	m.Targets.ElementsAs(ctx, &targets, false)
	switch {
	case !m.Weight.Equal(types.Float64Value(0)):
		t.Errorf("weight = %s, want 0", m.Weight)
	case !m.Channels.IsNull():
		t.Errorf("channels = %s, want null", m.Channels)
	case len(targets) != 1 || targets[0].Tags.IsNull() || len(targets[0].Tags.Elements()) != 0:
		t.Errorf("targets = %s, want one with empty tags", m.Targets)
	case !m.Priority.IsNull():
		t.Errorf("priority = %s, want null: it has no read override", m.Priority)
	case m.Delivery.ValueString() != "unspecified":
		t.Errorf("delivery = %s, want the proto zero value \"unspecified\"", m.Delivery)
	}
	if in.Weight != nil || in.Channels == nil {
		t.Errorf("flatten changed the API value: %+v", in)
	}
}
