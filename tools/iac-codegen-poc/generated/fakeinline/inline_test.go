// This file is handwritten. It checks the overrides (D21) of
// spec/fake/key.overrides.yaml: the fields of nested API objects in the
// parent (inline, F67), 64-bit numbers as strings (int64 and string, F68),
// and a secret (sensitive).

package fakeinline_test

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

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/generated/fakeinline"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/schemadump"
	sdk "github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/testsdk/go/openapi/gen/fake_boards_service"
)

// full has every writable field. maxCount is above 2^53, so a float64 would
// lose it.
const full = `{"backup":{"permissions":["b"]},"keyPermissions":{"permissions":["read"],"presets":["p1"]},` +
	`"limits":{"burst":"-5","perMinute":60},"maxCount":"9007199254740993","name":"k","ownerTeamId":42,` +
	`"rotation":{"everyDays":30,"permissions":{"presets":["p2"]}},"secret":"s","ttlSeconds":"18446744073709551"}`

func decode(t *testing.T, js string) *sdk.Key {
	t.Helper()
	var k sdk.Key
	if err := json.Unmarshal([]byte(js), &k); err != nil {
		t.Fatal(err)
	}
	return &k
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
func flatten(t *testing.T, js string) *fakeinline.KeyModel {
	t.Helper()
	ctx := context.Background()
	m, diags := fakeinline.FlattenKey(ctx, path.Root("key"), decode(t, js))
	if diags.HasError() {
		t.Fatal(diags)
	}
	state := tfsdk.State{Schema: hostSchema(), Raw: tftypes.NewValue(hostSchema().Type().TerraformType(ctx), nil)}
	if diags := state.Set(ctx, &hostModel{Key: m}); diags.HasError() {
		t.Fatalf("set: %v", diags)
	}
	var got hostModel
	if diags := state.Get(ctx, &got); diags.HasError() {
		t.Fatalf("get: %v", diags)
	}
	return got.Key
}

func expand(t *testing.T, m *fakeinline.KeyModel) string {
	t.Helper()
	out, diags := fakeinline.ExpandKey(context.Background(), path.Root("key"), m)
	if diags.HasError() {
		t.Fatal(diags)
	}
	return jsonOf(t, out)
}

// TestInlineSchema checks the attributes that the overrides change.
func TestInlineSchema(t *testing.T) {
	attrs := fakeinline.KeyAttributes()
	for _, name := range []string{"key_permissions", "limits"} {
		if _, ok := attrs[name]; ok {
			t.Errorf("%s: the holding field is an attribute", name)
		}
	}
	rotation := attrs["rotation"].(schema.SingleNestedAttribute).Attributes
	backup := attrs["backup"].(schema.SingleNestedAttribute).Attributes
	for _, c := range []struct {
		name string
		ok   bool
	}{
		{"secret is sensitive", attrs["secret"].IsSensitive()},
		{"name is not sensitive", !attrs["name"].IsSensitive()},
		{"permissions is a set", isType[schema.SetAttribute](attrs["permissions"])},
		{"per_minute is required", attrs["per_minute"].IsRequired()},
		{"updated_by is computed only", attrs["updated_by"].IsComputed() && !attrs["updated_by"].IsOptional()},
		{"owner_team_id is a string", isType[schema.StringAttribute](attrs["owner_team_id"])},
		{"max_count is an Int64", isType[schema.Int64Attribute](attrs["max_count"])},
		{"burst is an Int64", isType[schema.Int64Attribute](attrs["burst"])},
		{"ttl_seconds is an Int64", isType[schema.Int64Attribute](attrs["ttl_seconds"])},
		{"rotation inlines the permissions", rotation["presets"] != nil && rotation["permissions"] != nil && len(rotation) == 4},
		{"backup keeps the object", backup["presets"] != nil && len(backup) == 3},
	} {
		if !c.ok {
			t.Error(c.name + ": no")
		}
	}
}

func isType[T schema.Attribute](a schema.Attribute) bool {
	_, ok := a.(T)
	return ok
}

// TestInlineRoundTrip checks that the inlined fields read into the parent
// and are sent back in their objects, and that the 64-bit numbers keep every
// digit.
func TestInlineRoundTrip(t *testing.T) {
	m := flatten(t, full)
	switch {
	case !m.Permissions.Equal(strSet("read")) || !m.Presets.Equal(strList("p1")):
		t.Errorf("permissions %v, presets %v", m.Permissions, m.Presets)
	case m.PerMinute.ValueInt32() != 60 || m.Burst.ValueInt64() != -5:
		t.Errorf("per_minute %v, burst %v", m.PerMinute, m.Burst)
	case m.MaxCount.ValueInt64() != 9007199254740993 || m.TtlSeconds.ValueInt64() != 18446744073709551:
		t.Errorf("max_count %v, ttl_seconds %v", m.MaxCount, m.TtlSeconds)
	case m.OwnerTeamId.ValueString() != "42":
		t.Errorf("owner_team_id = %v", m.OwnerTeamId)
	case m.Rotation == nil || !m.Rotation.Presets.Equal(strList("p2")) || !m.Rotation.Permissions.IsNull():
		t.Errorf("rotation = %+v", m.Rotation)
	case m.Backup == nil || !m.Backup.Permissions.Equal(strSet("b")):
		t.Errorf("backup = %+v", m.Backup)
	}
	if got, want := expand(t, m), jsonOf(t, decode(t, full)); got != want {
		t.Errorf("expand:\n got %s\nwant %s", got, want)
	}
}

// TestInlineReads checks that a missing object reads as null inlined fields,
// with the read overrides of each field.
func TestInlineReads(t *testing.T) {
	for _, c := range []struct {
		name, json string
		check      func(m *fakeinline.KeyModel) bool
	}{
		{"a missing object is null fields", `{"limits":{"perMinute":1}}`, func(m *fakeinline.KeyModel) bool {
			return m.Permissions.IsNull() && m.Presets.IsNull() && m.UpdatedBy.IsNull() && m.Burst.IsNull() && m.Rotation == nil
		}},
		{"an empty object is null fields", `{"keyPermissions":{},"limits":{"perMinute":1},"rotation":{}}`, func(m *fakeinline.KeyModel) bool {
			return m.Permissions.IsNull() && m.Presets.IsNull() && m.Rotation != nil && m.Rotation.Permissions.IsNull()
		}},
		{"emptyAsNull on an inlined field", `{"keyPermissions":{"permissions":[],"presets":[]},"limits":{"perMinute":1}}`,
			func(m *fakeinline.KeyModel) bool { return m.Presets.IsNull() && m.Permissions.Equal(strSet()) }},
		{"a read-only inlined field", `{"keyPermissions":{"updatedBy":"u"},"limits":{"perMinute":1}}`,
			func(m *fakeinline.KeyModel) bool { return m.UpdatedBy.ValueString() == "u" }},
		{"the same object, not inlined", `{"backup":{"presets":[]},"limits":{"perMinute":1}}`,
			func(m *fakeinline.KeyModel) bool { return m.Backup != nil && m.Backup.Presets.IsNull() }},
	} {
		t.Run(c.name, func(t *testing.T) {
			if m := flatten(t, c.json); !c.check(m) {
				t.Errorf("%s read as %+v", c.json, m)
			}
		})
	}
}

// TestNumberTextErrors checks the diagnostics of a value that is not a
// number, in both directions.
func TestNumberTextErrors(t *testing.T) {
	ctx := context.Background()
	_, diags := fakeinline.FlattenKey(ctx, path.Root("key"), decode(t, `{"limits":{"perMinute":1},"maxCount":"12x"}`))
	if !diags.HasError() || !strings.Contains(diags[0].Summary(), "Invalid API value") {
		t.Errorf("flatten of maxCount 12x: %v", diags)
	}
	m := nullKey()
	m.OwnerTeamId = types.StringValue("team-a")
	_, diags = fakeinline.ExpandKey(ctx, path.Root("key"), m)
	if !diags.HasError() || !strings.Contains(diags[0].Detail(), `"team-a" is not a 64-bit number`) {
		t.Errorf("expand of owner_team_id team-a: %v", diags)
	}
	if want := path.Root("key").AtName("owner_team_id"); !diags[0].(interface{ Path() path.Path }).Path().Equal(want) {
		t.Errorf("path = %v, want %v", diags[0], want)
	}
}

// TestInlineExpand checks when expand sends an inlined object: when at least
// one of its writable fields is set.
func TestInlineExpand(t *testing.T) {
	for _, c := range []struct {
		name string
		edit func(m *fakeinline.KeyModel)
		want string
	}{
		{"all null", func(*fakeinline.KeyModel) {}, `{"limits":{"perMinute":0}}`},
		{"an empty set is a value", func(m *fakeinline.KeyModel) { m.Permissions = strSet() },
			`{"keyPermissions":{"permissions":[]},"limits":{"perMinute":0}}`},
		{"a read-only field is not sent", func(m *fakeinline.KeyModel) { m.UpdatedBy = types.StringValue("u") },
			`{"limits":{"perMinute":0}}`},
		{"an unknown field is not sent", func(m *fakeinline.KeyModel) { m.Presets = types.ListUnknown(types.StringType) },
			`{"limits":{"perMinute":0}}`},
		{"a required value object", func(m *fakeinline.KeyModel) { m.PerMinute = types.Int32Value(5) },
			`{"limits":{"perMinute":5}}`},
		{"in a nested parent", func(m *fakeinline.KeyModel) {
			m.Rotation = &fakeinline.KeyRotationModel{EveryDays: types.Int32Null(), Permissions: strSet("w"),
				Presets: types.ListNull(types.StringType), UpdatedBy: types.StringNull()}
		}, `{"limits":{"perMinute":0},"rotation":{"permissions":{"permissions":["w"]}}}`},
		{"negative and large numbers", func(m *fakeinline.KeyModel) {
			m.MaxCount, m.Burst, m.OwnerTeamId = types.Int64Value(-9007199254740993), types.Int64Value(0), types.StringValue("-7")
		}, `{"limits":{"burst":"0","perMinute":0},"maxCount":"-9007199254740993","ownerTeamId":-7}`},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := nullKey()
			c.edit(m)
			if got := expand(t, m); got != c.want {
				t.Errorf("expand = %s, want %s", got, c.want)
			}
		})
	}
}

func nullKey() *fakeinline.KeyModel {
	return &fakeinline.KeyModel{
		Name: types.StringNull(), Secret: types.StringNull(),
		Permissions: types.SetNull(types.StringType), Presets: types.ListNull(types.StringType), UpdatedBy: types.StringNull(),
		PerMinute: types.Int32Null(), Burst: types.Int64Null(),
		OwnerTeamId: types.StringNull(), MaxCount: types.Int64Null(), TtlSeconds: types.Int64Null(),
	}
}

// TestInlineValidators runs the schema checks through a provider server: an
// inlined field that is required in a required object is required in the
// parent.
func TestInlineValidators(t *testing.T) {
	for _, c := range []struct {
		name string
		edit func(m *fakeinline.KeyModel)
		want string // "" for no error
	}{
		{"valid", func(*fakeinline.KeyModel) {}, ""},
		{"no per_minute", func(m *fakeinline.KeyModel) { m.PerMinute = types.Int32Null() }, `AttributeName("per_minute")`},
		{"negative unsigned", func(m *fakeinline.KeyModel) { m.TtlSeconds = types.Int64Value(-1) }, `AttributeName("ttl_seconds")`},
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

// TestInlinePlanWithUnknowns checks that an update plan with the computed
// updated_by unknown reads into the model, and that expand sends no object
// for it: only the unknown read-only field is set.
func TestInlinePlanWithUnknowns(t *testing.T) {
	ctx := context.Background()
	state := tfsdk.State{Schema: hostSchema(), Raw: tftypes.NewValue(hostSchema().Type().TerraformType(ctx), nil)}
	m := flatten(t, `{"keyPermissions":{"updatedBy":"u"},"limits":{"perMinute":1},"rotation":{"permissions":{"updatedBy":"v"}}}`)
	if diags := state.Set(ctx, &hostModel{Key: m}); diags.HasError() {
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
	if !plan.Key.UpdatedBy.IsUnknown() || !plan.Key.Rotation.UpdatedBy.IsUnknown() {
		t.Errorf("updated_by = %v, rotation.updated_by = %v, want unknown", plan.Key.UpdatedBy, plan.Key.Rotation.UpdatedBy)
	}
	if got, want := expand(t, plan.Key), `{"limits":{"perMinute":1},"rotation":{}}`; got != want {
		t.Errorf("expand = %s, want %s", got, want)
	}
}

func strSet(vs ...string) types.Set {
	return types.SetValueMust(types.StringType, strValues(vs))
}

func strList(vs ...string) types.List {
	return types.ListValueMust(types.StringType, strValues(vs))
}

func strValues(vs []string) []attr.Value {
	elems := make([]attr.Value, 0, len(vs))
	for _, v := range vs {
		elems = append(elems, types.StringValue(v))
	}
	return elems
}

func hostSchema() schema.Schema {
	return schema.Schema{Attributes: map[string]schema.Attribute{
		"key": schema.SingleNestedAttribute{Required: true, Attributes: fakeinline.KeyAttributes()},
	}}
}

type hostModel struct {
	Key *fakeinline.KeyModel `tfsdk:"key"`
}

// validate returns the validation errors of a configuration with m, and
// their paths.
func validate(t *testing.T, m *fakeinline.KeyModel) string {
	t.Helper()
	ctx := context.Background()
	state := tfsdk.State{Schema: hostSchema(), Raw: tftypes.NewValue(hostSchema().Type().TerraformType(ctx), nil)}
	if diags := state.Set(ctx, &hostModel{Key: m}); diags.HasError() {
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
