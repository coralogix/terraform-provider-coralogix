// Copyright 2026 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fleet

import (
	"context"
	"testing"

	cfggroups "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/fleet_manager_configuration_groups"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func presetFamily(integrationVersion types.String, features string) *FleetConfigurationGroupFamilyModel {
	return &FleetConfigurationGroupFamilyModel{
		Active:      types.BoolValue(true),
		Description: types.StringNull(),
		Preset: &FleetPresetFamilyModel{
			ChartName:             types.StringValue("otel_integration"),
			ChartVersion:          types.StringValue("0.0.200"),
			IntegrationVersion:    integrationVersion,
			Metadata:              types.MapValueMust(types.StringType, map[string]attr.Value{"ClusterName": types.StringValue("prod")}),
			ObservabilityFeatures: types.StringValue(features),
			RemoteConfigurations:  types.ListUnknown(types.ObjectType{AttrTypes: remoteConfigurationAttrTypes()}),
		},
	}
}

func TestSchemaIsValid(t *testing.T) {
	var r FleetConfigurationGroupResource
	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)
	if diags := resp.Schema.ValidateImplementation(context.Background()); diags.HasError() {
		t.Fatalf("schema is invalid: %v", diags)
	}
}

func TestChartNameRoundTrip(t *testing.T) {
	values := chartNameSchemaValues()
	if len(values) != len(cfggroups.AllowedChartNameEnumValues)-1 {
		t.Fatalf("schema values = %v, want every SDK chart name except unspecified", values)
	}
	for _, value := range values {
		if value == "unspecified" {
			t.Fatal("unspecified must not be a valid chart_name")
		}
		api := chartNameToAPI(value)
		if !api.IsValid() {
			t.Fatalf("chartNameToAPI(%q) = %q, not a valid SDK value", value, api)
		}
		if got := chartNameFromAPI(&api).ValueString(); got != value {
			t.Fatalf("round trip of %q = %q", value, got)
		}
	}
	unspecified := cfggroups.CHARTNAME_CHART_NAME_UNSPECIFIED
	if !chartNameFromAPI(&unspecified).IsNull() || !chartNameFromAPI(nil).IsNull() {
		t.Fatal("unspecified or missing chart name should flatten to null")
	}
}

func TestJSONStringsEqualIgnoresKeyOrderAndWhitespace(t *testing.T) {
	if !jsonStringsEqual(`{"logs":{"enabled":true},"metrics":1}`, "{\n  \"metrics\": 1,\n  \"logs\": {\"enabled\": true}\n}") {
		t.Fatal("reordered JSON should compare equal")
	}
	if jsonStringsEqual(`{"logs":{"enabled":true}}`, `{"logs":{"enabled":false}}`) {
		t.Fatal("different JSON should not compare equal")
	}
	if jsonStringsEqual(`not json`, `{}`) {
		t.Fatal("invalid JSON should not compare equal")
	}
}

func TestExpandFamilyCreateSendsOnlyTheConfiguredType(t *testing.T) {
	ctx := context.Background()
	preset, diags := expandFamilyCreate(ctx, presetFamily(types.StringUnknown(), `{"logs":{}}`))
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if !preset.HasPreset() || preset.HasRaw() {
		t.Fatal("preset family should send preset only")
	}
	if preset.Preset.GetChartName() != cfggroups.CHARTNAME_CHART_NAME_OTEL_INTEGRATION {
		t.Fatalf("chartName = %q", preset.Preset.GetChartName())
	}
	if preset.Preset.HasIntegrationVersion() {
		t.Fatal("unknown integration_version should be omitted so the API picks the default")
	}
	if preset.Preset.Metadata["ClusterName"] != "prod" {
		t.Fatalf("metadata = %v", preset.Preset.Metadata)
	}

	raw, diags := expandFamilyCreate(ctx, rawFamily(types.StringValue("0.114.0"), "receivers: {}\n"))
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if raw.HasPreset() || !raw.HasRaw() {
		t.Fatal("raw family should send raw only")
	}
	if raw.Raw.GetCollectorVersion() != "0.114.0" || len(raw.Raw.RemoteConfigurations) != 1 {
		t.Fatalf("raw = %+v", raw.Raw)
	}

	neither := presetFamily(types.StringNull(), `{}`)
	neither.Preset = nil
	if _, diags := expandFamilyCreate(ctx, neither); !diags.HasError() {
		t.Fatal("a family with neither preset nor raw should fail")
	}
}

func TestExpandUpdateRequestSendsChangedPreset(t *testing.T) {
	prior := presetFamily(types.StringValue("1.0.0"), `{"a":1}`)
	planned := presetFamily(types.StringUnknown(), `{"a":2}`)
	planned.Preset.Metadata = types.MapNull(types.StringType)
	group := func(f *FleetConfigurationGroupFamilyModel) *FleetConfigurationGroupResourceModel {
		return &FleetConfigurationGroupResourceModel{Name: types.StringValue("g"), Description: types.StringNull(), Tags: types.ListNull(types.StringType), PriorityOrder: types.Int64Value(0), Family: f}
	}
	req, mask, diags := expandUpdateRequest(context.Background(), group(planned), group(prior))
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if mask[len(mask)-1] != "family.preset" || !req.Family.HasPreset() || req.Family.HasRaw() {
		t.Fatalf("changed preset should be masked and sent, mask %v", mask)
	}
	if req.Family.Preset.HasIntegrationVersion() {
		t.Fatal("unknown integration_version should be omitted so the API re-resolves it")
	}
	if req.Family.Preset.HasMetadata() {
		t.Fatal("removed metadata should be omitted so the masked arm clears it")
	}
	if req.Family.Preset.GetObservabilityFeatures() != `{"a":2}` {
		t.Fatalf("observabilityFeatures = %q", req.Family.Preset.GetObservabilityFeatures())
	}
}

func TestFlattenFamilyPreset(t *testing.T) {
	chart := cfggroups.CHARTNAME_CHART_NAME_OTEL_LINUX_STANDALONE
	apiFamily := cfggroups.NewConfigurationFamily("family-id", "3")
	apiFamily.SetActive(true)
	apiFamily.SetCollectorVersion("0.120.0")
	preset := cfggroups.NewPresetConfigurationFamily("0.0.200", "1.2.0", `{"metrics":1,"logs":{"enabled":true}}`)
	preset.ChartName = &chart
	preset.SetRemoteConfigurations([]cfggroups.RemoteConfiguration{
		remoteAPI("otel-cluster-collector", "cluster-id", "cluster-hash", "receivers: [http]\n"),
		remoteAPI("otel-agent", "agent-id", "agent-hash", "receivers: [otlp]\n"),
	})
	apiFamily.SetPreset(*preset)

	configured := `{"logs": {"enabled": true}, "metrics": 1}`
	plan := presetFamily(types.StringUnknown(), configured)
	plan.Preset.Metadata = types.MapNull(types.StringType)
	got, diags := flattenFamily(context.Background(), plan, []cfggroups.ConfigurationFamily{*apiFamily}, true)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if got.Raw != nil || got.Preset == nil {
		t.Fatal("preset family should flatten into preset only")
	}
	if got.Preset.ChartName.ValueString() != "otel_linux_standalone" {
		t.Fatalf("chart_name = %q", got.Preset.ChartName.ValueString())
	}
	if got.Preset.IntegrationVersion.ValueString() != "1.2.0" {
		t.Fatalf("integration_version = %q", got.Preset.IntegrationVersion.ValueString())
	}
	if got.Preset.ObservabilityFeatures.ValueString() != configured {
		t.Fatalf("equivalent observability_features should echo the configured text, got %q", got.Preset.ObservabilityFeatures.ValueString())
	}
	if !got.Preset.Metadata.IsNull() {
		t.Fatalf("unset metadata should stay null, got %v", got.Preset.Metadata)
	}
	var remotes []FleetRemoteConfigurationModel
	if diags := got.Preset.RemoteConfigurations.ElementsAs(context.Background(), &remotes, false); diags.HasError() {
		t.Fatalf("remote_configuration: %v", diags)
	}
	if len(remotes) != 2 || remotes[0].Name.ValueString() != "otel-agent" || remotes[1].ID.ValueString() != "cluster-id" {
		t.Fatalf("generated remotes should be sorted by name, got %+v", remotes)
	}
}

func TestPresetConfigUnchanged(t *testing.T) {
	state := presetFamily(types.StringValue("1.2.0"), `{"metrics":1,"logs":{}}`)
	if !familyConfigUnchanged(presetFamily(types.StringUnknown(), `{"logs":{},"metrics":1}`), state) {
		t.Fatal("omitted integration_version and reordered JSON should not change the family")
	}
	changed := presetFamily(types.StringValue("1.2.0"), `{"metrics":2,"logs":{}}`)
	if familyConfigUnchanged(changed, state) {
		t.Fatal("changed observability_features should change the family")
	}
	bumped := presetFamily(types.StringValue("1.2.0"), `{"metrics":1,"logs":{}}`)
	bumped.Preset.ChartVersion = types.StringValue("0.0.201")
	if familyConfigUnchanged(bumped, state) {
		t.Fatal("chart_version change should change the family")
	}
	if familyConfigUnchanged(rawFamily(types.StringValue("0.114.0"), "receivers: {}\n"), state) {
		t.Fatal("switching from preset to raw should change the family")
	}
}

func TestUpgradeStateV0MovesFamilyFieldsUnderRaw(t *testing.T) {
	ctx := context.Background()
	schemaV0 := fleetConfigurationGroupSchemaV0()
	var current resource.SchemaResponse
	(&FleetConfigurationGroupResource{}).Schema(ctx, resource.SchemaRequest{}, &current)

	prior := tfsdk.State{Schema: schemaV0, Raw: tftypes.NewValue(schemaV0.Type().TerraformType(ctx), nil)}
	selector := types.MapValueMust(types.StringType, map[string]attr.Value{"cx.agent.type": types.StringValue("agent")})
	diags := prior.Set(ctx, &fleetConfigurationGroupResourceModelV0{
		ID:            types.StringValue("group-id"),
		Name:          types.StringValue("group"),
		Description:   types.StringNull(),
		Tags:          types.ListNull(types.StringType),
		PriorityOrder: types.Int64Value(10),
		Family: &fleetConfigurationGroupFamilyModelV0{
			ID:               types.StringValue("family-id"),
			Version:          types.StringValue("2"),
			Active:           types.BoolValue(true),
			Description:      types.StringNull(),
			CollectorVersion: types.StringValue("0.114.0"),
			Metadata:         types.MapValueMust(types.StringType, map[string]attr.Value{"team": types.StringValue("o11y")}),
			RemoteConfigurations: []FleetRemoteConfigurationModel{
				{ID: types.StringValue("b-id"), Hash: types.StringValue("b-hash"), Name: types.StringValue("b"), RawConfiguration: types.StringValue("receivers: {}\n"), AgentSelector: selector},
				{ID: types.StringValue("a-id"), Hash: types.StringValue("a-hash"), Name: types.StringValue("a"), RawConfiguration: types.StringValue("receivers: {}\n"), AgentSelector: types.MapNull(types.StringType)},
			},
		},
	})
	if diags.HasError() {
		t.Fatalf("setting v0 state: %v", diags)
	}

	req := resource.UpgradeStateRequest{State: &prior}
	resp := resource.UpgradeStateResponse{State: tfsdk.State{Schema: current.Schema, Raw: tftypes.NewValue(current.Schema.Type().TerraformType(ctx), nil)}}
	upgradeFleetConfigurationGroupStateV0(ctx, req, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("upgrade: %v", resp.Diagnostics)
	}

	var got FleetConfigurationGroupResourceModel
	if diags := resp.State.Get(ctx, &got); diags.HasError() {
		t.Fatalf("reading upgraded state: %v", diags)
	}
	if got.ID.ValueString() != "group-id" || got.PriorityOrder.ValueInt64() != 10 {
		t.Fatalf("group fields not carried over: %+v", got)
	}
	if got.Family == nil || got.Family.Preset != nil || got.Family.Raw == nil {
		t.Fatalf("family should be upgraded into raw, got %+v", got.Family)
	}
	if got.Family.ID.ValueString() != "family-id" || got.Family.Version.ValueString() != "2" {
		t.Fatalf("family id/version not carried over: %+v", got.Family)
	}
	raw := got.Family.Raw
	if raw.CollectorVersion.ValueString() != "0.114.0" || raw.Metadata.Elements()["team"].(types.String).ValueString() != "o11y" {
		t.Fatalf("raw collector_version/metadata not carried over: %+v", raw)
	}
	if len(raw.RemoteConfigurations) != 2 || raw.RemoteConfigurations[0].Name.ValueString() != "b" || raw.RemoteConfigurations[0].ID.ValueString() != "b-id" {
		t.Fatalf("remotes should keep their order and ids, got %+v", raw.RemoteConfigurations)
	}
}
