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

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestFlattenRemotesPreservesPlanOrderWhenAPIReorders(t *testing.T) {
	plan := []FleetRemoteConfigurationModel{
		remotePlan("otel-agent", "receivers: [otlp]\n"),
		remotePlan("otel-cluster-collector", "receivers: [http]\n"),
	}
	api := []cfggroups.RemoteConfiguration{
		remoteAPI("otel-cluster-collector", "cluster-id", "cluster-hash", "receivers: [http]\n"),
		remoteAPI("otel-agent", "agent-id", "agent-hash", "receivers: [otlp]\n"),
	}

	got, diags := flattenRemotes(context.Background(), plan, api, "", false)
	if diags.HasError() {
		t.Fatalf("flattenRemotes diagnostics: %v", diags)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].Name.ValueString() != "otel-agent" || got[1].Name.ValueString() != "otel-cluster-collector" {
		t.Fatalf("order = [%q, %q], want [otel-agent, otel-cluster-collector]", got[0].Name.ValueString(), got[1].Name.ValueString())
	}
	if got[0].ID.ValueString() != "agent-id" || got[1].ID.ValueString() != "cluster-id" {
		t.Fatalf("ids = [%q, %q], want [agent-id, cluster-id]", got[0].ID.ValueString(), got[1].ID.ValueString())
	}
}

func TestFlattenRemotesSortsByNameWhenPlanIsEmpty(t *testing.T) {
	api := []cfggroups.RemoteConfiguration{
		remoteAPI("otel-cluster-collector", "cluster-id", "cluster-hash", "receivers: [http]\n"),
		remoteAPI("otel-agent", "agent-id", "agent-hash", "receivers: [otlp]\n"),
	}

	got, diags := flattenRemotes(context.Background(), nil, api, "", false)
	if diags.HasError() {
		t.Fatalf("flattenRemotes diagnostics: %v", diags)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].Name.ValueString() != "otel-agent" || got[1].Name.ValueString() != "otel-cluster-collector" {
		t.Fatalf("import order = [%q, %q], want [otel-agent, otel-cluster-collector]", got[0].Name.ValueString(), got[1].Name.ValueString())
	}
}

func remotePlan(name, raw string) FleetRemoteConfigurationModel {
	return FleetRemoteConfigurationModel{
		Name:             types.StringValue(name),
		RawConfiguration: types.StringValue(raw),
	}
}

func remoteAPI(name, id, hash, raw string) cfggroups.RemoteConfiguration {
	remote := cfggroups.NewRemoteConfiguration()
	remote.SetName(name)
	remote.SetId(id)
	remote.SetHash(hash)
	remote.SetRawConfiguration(raw)
	return *remote
}
