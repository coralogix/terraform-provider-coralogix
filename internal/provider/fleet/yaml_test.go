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
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"gopkg.in/yaml.v3"
)

func TestExampleCollectorYAMLFilesParse(t *testing.T) {
	exampleDir := filepath.Join("..", "..", "..", "examples", "resources", "coralogix_fleet_configuration_group")
	for _, name := range []string{"otel-agent.yaml", "otel-cluster-collector.yaml"} {
		path := filepath.Join(exampleDir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var doc any
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if doc == nil {
			t.Fatalf("%s decoded to nil", path)
		}
	}
}

func TestYAMLStringsEqualTreatsInlineAndMultilineListsAsTheSame(t *testing.T) {
	inline := "receivers: [otlp]\n"
	multiline := "receivers:\n  - otlp\n"
	if !yamlStringsEqual(inline, multiline) {
		t.Fatal("inline and multiline YAML lists should compare equal")
	}
}

func TestYAMLStringsEqualRejectsDifferentDocuments(t *testing.T) {
	if yamlStringsEqual("receivers: [otlp]\n", "receivers: [http]\n") {
		t.Fatal("different YAML documents should not compare equal")
	}
}

func TestFlattenConfiguredStringPreservesEmptyPlan(t *testing.T) {
	empty := ""
	got := flattenConfiguredString(&empty, types.StringValue(""))
	if got.IsNull() || got.ValueString() != "" {
		t.Fatalf("configured empty string should stay empty, got %#v", got)
	}
	got = flattenConfiguredString(nil, types.StringNull())
	if !got.IsNull() {
		t.Fatalf("omitted description should stay null, got %#v", got)
	}
}

func TestFlattenConfiguredStringDoesNotMaskRemoteClear(t *testing.T) {
	empty := ""
	got := flattenConfiguredString(&empty, types.StringValue("kept by terraform"))
	if !got.IsNull() {
		t.Fatalf("remotely cleared description should become null, got %#v", got)
	}
	got = flattenConfiguredString(nil, types.StringValue("kept by terraform"))
	if !got.IsNull() {
		t.Fatalf("nil API description should not echo prior nonempty state, got %#v", got)
	}
}

func TestSelectorAttrsForStateDropsInjectedCollectorVersion(t *testing.T) {
	api := map[string]string{
		"cx.agent.type":   "agent",
		"service.version": "0.114.0",
	}
	got := selectorAttrsForState(api, types.MapNull(types.StringType), "0.114.0", true)
	if _, ok := got["service.version"]; ok {
		t.Fatal("injected service.version should be dropped when it matches collector_version")
	}
	if got["cx.agent.type"] != "agent" {
		t.Fatalf("kept selector attr = %q", got["cx.agent.type"])
	}
}

func TestSelectorAttrsForStateKeepsExplicitServiceVersion(t *testing.T) {
	api := map[string]string{
		"cx.agent.type":   "agent",
		"service.version": "1.2.3",
	}
	got := selectorAttrsForState(api, types.MapNull(types.StringType), "", true)
	if got["service.version"] != "1.2.3" {
		t.Fatalf("explicit service.version without collector_version should be kept, got %#v", got)
	}
	got = selectorAttrsForState(api, types.MapNull(types.StringType), "0.114.0", false)
	if got["service.version"] != "1.2.3" {
		t.Fatalf("data-source reads should keep remote service.version, got %#v", got)
	}
}

func rawFamily(collectorVersion types.String, rawConfiguration string) *FleetConfigurationGroupFamilyModel {
	return &FleetConfigurationGroupFamilyModel{
		Active:      types.BoolValue(true),
		Description: types.StringNull(),
		Raw: &FleetRawFamilyModel{
			CollectorVersion: collectorVersion,
			Metadata:         types.MapNull(types.StringType),
			RemoteConfigurations: []FleetRemoteConfigurationModel{{
				Name:             types.StringValue("default"),
				RawConfiguration: types.StringValue(rawConfiguration),
				AgentSelector:    types.MapNull(types.StringType),
			}},
		},
	}
}

func TestFamilyConfigUnchangedIgnoresGroupLevelFields(t *testing.T) {
	family := rawFamily(types.StringValue("0.114.0"), "receivers: [otlp]\n")
	if !familyConfigUnchanged(family, family) {
		t.Fatal("identical families should compare equal")
	}
	changed := rawFamily(types.StringValue("0.115.0"), "receivers: [otlp]\n")
	if familyConfigUnchanged(changed, family) {
		t.Fatal("collector_version change should not compare equal")
	}
	omitted := rawFamily(types.StringNull(), "receivers: [otlp]\n")
	if familyConfigUnchanged(omitted, family) {
		t.Fatal("removing collector_version should change the family so the replace clears it")
	}
}

func TestExpandReplaceRequestOmitsUnchangedFamily(t *testing.T) {
	family := rawFamily(types.StringValue("0.114.0"), "receivers: {}\n")
	plan := &FleetConfigurationGroupResourceModel{
		Name:          types.StringValue("new-name"),
		Description:   types.StringNull(),
		Tags:          types.ListNull(types.StringType),
		PriorityOrder: types.Int64Value(0),
		Family:        family,
	}
	prior := &FleetConfigurationGroupResourceModel{
		Name:          types.StringValue("old-name"),
		Description:   types.StringNull(),
		Tags:          types.ListNull(types.StringType),
		PriorityOrder: types.Int64Value(0),
		Family:        family,
	}
	req, mask, diags := expandUpdateRequest(context.Background(), plan, prior)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if req.HasFamily() || slices.ContainsFunc(mask, func(p string) bool { return strings.HasPrefix(p, "family") }) {
		t.Fatalf("unchanged family should be left out of the update, mask %v", mask)
	}
	if !slices.Equal(mask, []string{"name", "description", "tags", "priorityOrder"}) {
		t.Fatalf("group fields should always be masked, got %v", mask)
	}

	plan.Family = rawFamily(types.StringNull(), "receivers: {}\n")
	req, mask, diags = expandUpdateRequest(context.Background(), plan, prior)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if !slices.Equal(mask[4:], []string{"family.raw"}) || !req.Family.HasRaw() || req.Family.Raw.HasCollectorVersion() {
		t.Fatalf("removed collector_version should send family.raw without collectorVersion, mask %v", mask)
	}

	activeOnly := rawFamily(types.StringValue("0.114.0"), "receivers: {}\n")
	activeOnly.Active = types.BoolValue(false)
	plan.Family = activeOnly
	req, mask, diags = expandUpdateRequest(context.Background(), plan, prior)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if !slices.Equal(mask[4:], []string{"family.active"}) || req.Family.GetActive() || req.Family.HasRaw() {
		t.Fatalf("an active-only change should mask only family.active, mask %v", mask)
	}

	plan.Family = family
	_, mask, diags = expandUpdateRequest(context.Background(), plan, nil)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if !slices.Contains(mask, "family.raw") || !slices.Contains(mask, "family.active") {
		t.Fatalf("update without prior state should send the whole family, mask %v", mask)
	}
}
