// This file is handwritten. It tests the generated code for oneOf groups: a
// oneOf with a normal field beside the arms (Interval), an object with two
// groups (Layout: refresh, and time), and a group at the resource root
// (publicLink | privateShare).

package fakeboard

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// withGroups returns withNumbers with an arm set in every group.
func withGroups(t *testing.T) *FakeBoardModel {
	m := withNumbers(t)
	m.PublicLink = &PublicLinkModel{Url: types.StringValue("https://x")}
	m.Layout.RefreshEvery = &EveryModel{Minutes: types.Int32Value(5)}
	m.Layout.Section.Interval = &IntervalModel{Auto: &BoldStyleModel{}, UseLimit: types.BoolValue(true)}
	return m
}

func TestExpandGroups(t *testing.T) {
	body, diags := expandCreate(context.Background(), withGroups(t))
	assertNoDiags(t, diags)
	assertJSON(t, body.PublicLink, `{"url":"https://x"}`)
	assertJSON(t, body.PrivateShare, `null`)
	assertJSON(t, body.Layout.Section.Interval, `{"auto":{},"useLimit":true}`)
	if body.Layout.RefreshEvery == nil || body.Layout.RelativeTime == nil || body.Layout.RefreshOff != nil {
		t.Errorf("layout arms = %+v", body.Layout)
	}
}

func TestRoundTripGroups(t *testing.T) {
	const resp = `{"id":"b1","name":"ops","privateShare":{"team":"sre"},
		"layout":{"title":"T","refreshOff":{},"absoluteTime":{"from":"2026-09-26T08:00:00Z","to":"2026-09-26T09:30:00.5Z"},
			"section":{"interval":{"manual":{"minutes":1},"useLimit":false}}}}`
	m, diags := flatten(context.Background(), unmarshalBoard(t, resp))
	assertNoDiags(t, diags)
	if m.PublicLink != nil || m.PrivateShare == nil || m.Layout.RefreshOff == nil || m.Layout.RefreshEvery != nil {
		t.Fatalf("arms = %+v, %+v, %+v", m.PublicLink, m.PrivateShare, m.Layout)
	}
	body, diags := expandUpdate(context.Background(), m)
	assertNoDiags(t, diags)
	assertJSON(t, body.PrivateShare, `{"team":"sre"}`)
	assertJSON(t, body.Layout.Section.Interval, `{"manual":{"minutes":1},"useLimit":false}`)
}

// TestGroupValidators checks the validator of each group. A nested group is
// checked only when its object is set.
func TestGroupValidators(t *testing.T) {
	cases := []struct {
		name string
		set  func(m *FakeBoardModel)
		want string // a part of the error; "" for no error
	}{
		{"one arm everywhere", func(m *FakeBoardModel) {}, ""},
		{"no arm where none is allowed", func(m *FakeBoardModel) {
			m.PublicLink, m.Layout.RefreshEvery = nil, nil
		}, ""},
		// Interval needs exactly one arm. Without an interval, the check does
		// not run: a resource validator would fail here.
		{"required group, parent not set", func(m *FakeBoardModel) { m.Layout.Section.Interval = nil }, ""},
		{"required group, no arm", func(m *FakeBoardModel) {
			m.Layout.Section.Interval = &IntervalModel{UseLimit: types.BoolValue(true)}
		}, "layout.section.interval.auto"},
		{"two arms beside a normal field", func(m *FakeBoardModel) {
			m.Layout.Section.Interval.Manual = &EveryModel{}
		}, "layout.section.interval.auto"},
		{"two arms in group 1", func(m *FakeBoardModel) { m.Layout.RefreshOff = &BoldStyleModel{} }, "layout.refresh_every"},
		{"no arm in a required group", func(m *FakeBoardModel) { m.Layout.RelativeTime = nil }, "layout.absolute_time"},
		{"two arms at the root", func(m *FakeBoardModel) {
			m.PrivateShare = &PrivateShareModel{Team: types.StringValue("sre")}
		}, "private_share"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := withGroups(t)
			c.set(m)
			diags := validateAll(t, m)
			if c.want == "" {
				assertNoDiags(t, diags)
				return
			}
			if !diags.HasError() || !strings.Contains(diagText(diags), c.want) {
				t.Errorf("diagnostics = %v, want an error about %s", diags, c.want)
			}
		})
	}
}

// validateAll validates m as Terraform does: through a provider server,
// with the resource validators and the attribute validators.
func validateAll(t *testing.T, m *FakeBoardModel) diag.Diagnostics {
	t.Helper()
	ctx := context.Background()
	srv := providerserver.NewProtocol6(&testProvider{})()
	if _, err := srv.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{}); err != nil {
		t.Fatal(err)
	}
	state := toState(t, m)
	config, err := tfprotov6.NewDynamicValue(Schema().Type().TerraformType(ctx), state.Raw)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := srv.ValidateResourceConfig(ctx, &tfprotov6.ValidateResourceConfigRequest{
		TypeName: "test_" + TypeName, Config: &config,
	})
	if err != nil {
		t.Fatal(err)
	}
	var diags diag.Diagnostics
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			diags.AddAttributeError(frameworkPath(d.Attribute), d.Summary, d.Detail+" at "+d.Attribute.String())
		}
	}
	return diags
}

// frameworkPath converts the attribute path of a protocol diagnostic.
func frameworkPath(p *tftypes.AttributePath) path.Path {
	out := path.Empty()
	if p == nil {
		return out
	}
	for _, step := range p.Steps() {
		switch s := step.(type) {
		case tftypes.AttributeName:
			out = out.AtName(string(s))
		case tftypes.ElementKeyInt:
			out = out.AtListIndex(int(s))
		case tftypes.ElementKeyString:
			out = out.AtMapKey(string(s))
		}
	}
	return out
}

// testProvider serves only the generated resource.
type testProvider struct{}

func (p *testProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "test"
}

func (p *testProvider) Schema(context.Context, provider.SchemaRequest, *provider.SchemaResponse) {}

func (p *testProvider) Configure(context.Context, provider.ConfigureRequest, *provider.ConfigureResponse) {
}

func (p *testProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{NewResource}
}

func (p *testProvider) DataSources(context.Context) []func() datasource.DataSource { return nil }

func diagText(diags diag.Diagnostics) string {
	var b strings.Builder
	for _, d := range diags {
		b.WriteString(d.Summary() + " " + d.Detail())
		if p, ok := d.(diag.DiagnosticWithPath); ok {
			b.WriteString(" " + p.Path().String())
		}
		b.WriteString("\n")
	}
	return b.String()
}

func TestLeafMaskGroups(t *testing.T) {
	cases := []struct {
		name   string
		change func(m *FakeBoardModel)
		mask   string
	}{
		{"switch arm beside a normal field", func(m *FakeBoardModel) {
			m.Layout.Section.Interval.Auto, m.Layout.Section.Interval.Manual = nil, &EveryModel{Minutes: types.Int32Value(2)}
		}, "layout.section.interval.manual"},
		{"the normal field beside the arms", func(m *FakeBoardModel) {
			m.Layout.Section.Interval.UseLimit = types.BoolValue(false)
		}, "layout.section.interval.useLimit"},
		{"remove the arm", func(m *FakeBoardModel) { m.Layout.RefreshEvery = nil }, "layout.refreshEvery"},
		{"switch arm in group 2 of 2", func(m *FakeBoardModel) {
			m.Layout.RelativeTime, m.Layout.AbsoluteTime = nil, &AbsoluteTimeModel{From: types.StringValue("2026-09-26T08:00:00Z")}
		}, "layout.absoluteTime"},
		{"value inside an arm", func(m *FakeBoardModel) {
			m.Layout.RefreshEvery = &EveryModel{Minutes: types.Int32Value(6)}
		}, "layout.refreshEvery.minutes"},
		{"switch arm at the root", func(m *FakeBoardModel) {
			m.PublicLink, m.PrivateShare = nil, &PrivateShareModel{Team: types.StringValue("sre")}
		}, "privateShare"},
		{"remove the arm at the root", func(m *FakeBoardModel) { m.PublicLink = nil }, "publicLink"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan := withGroups(t)
			c.change(plan)
			body, diags := updateRequest(context.Background(), toPlan(t, plan), toState(t, withGroups(t)))
			assertNoDiags(t, diags)
			if got := maskOf(body); got != c.mask {
				t.Errorf("mask = %q, want %q", got, c.mask)
			}
		})
	}
}
