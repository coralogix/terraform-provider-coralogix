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

func TestFamilyConfigUnchangedIgnoresGroupLevelFields(t *testing.T) {
	family := &FleetConfigurationGroupFamilyModel{
		Active:           types.BoolValue(true),
		CollectorVersion: types.StringValue("0.114.0"),
		Description:      types.StringNull(),
		Metadata:         types.MapNull(types.StringType),
		RemoteConfigurations: []FleetRemoteConfigurationModel{{
			Name:             types.StringValue("default"),
			RawConfiguration: types.StringValue("receivers: [otlp]\n"),
			AgentSelector:    types.MapNull(types.StringType),
		}},
	}
	if !familyConfigUnchanged(family, family) {
		t.Fatal("identical families should compare equal")
	}
	changed := *family
	changed.CollectorVersion = types.StringValue("0.115.0")
	if familyConfigUnchanged(&changed, family) {
		t.Fatal("collector_version change should not compare equal")
	}
}

func TestExpandReplaceRequestOmitsUnchangedFamily(t *testing.T) {
	family := &FleetConfigurationGroupFamilyModel{
		Active:           types.BoolValue(true),
		CollectorVersion: types.StringValue("0.114.0"),
		Description:      types.StringNull(),
		Metadata:         types.MapNull(types.StringType),
		RemoteConfigurations: []FleetRemoteConfigurationModel{{
			Name:             types.StringValue("default"),
			RawConfiguration: types.StringValue("receivers: {}\n"),
			AgentSelector:    types.MapNull(types.StringType),
		}},
	}
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
	req, diags := expandReplaceRequest(context.Background(), plan, prior)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if req.Group.HasFamily() {
		t.Fatal("unchanged family should be omitted from replace")
	}

	req, diags = expandReplaceRequest(context.Background(), plan, nil)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if !req.Group.HasFamily() {
		t.Fatal("replace without prior state should include family")
	}
}
