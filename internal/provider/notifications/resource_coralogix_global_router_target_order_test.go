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

package notifications

import (
	"context"
	"slices"
	"testing"

	globalRouters "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/global_routers_service"
	"github.com/hashicorp/terraform-plugin-framework/types"

	globalrouterschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/notifications/global_router_schema"
)

// target builds a routing target. An empty preset leaves PresetId unset.
func target(connector, preset string) globalRouters.RoutingTarget {
	t := globalRouters.RoutingTarget{ConnectorId: &connector}
	if preset != "" {
		t.PresetId = &preset
	}
	return t
}

// targetWithDetails builds a routing target whose custom details hold k=v.
func targetWithDetails(connector, preset, v string) globalRouters.RoutingTarget {
	t := target(connector, preset)
	t.CustomDetails = map[string]string{"k": v}
	return t
}

func targetKeys(targets []globalRouters.RoutingTarget) []string {
	keys := make([]string, 0, len(targets))
	for _, t := range targets {
		key := t.GetConnectorId() + "/" + t.GetPresetId()
		if v, ok := t.GetCustomDetails()["k"]; ok {
			key += "{k=" + v + "}"
		}
		keys = append(keys, key)
	}
	return keys
}

func TestOrderTargetsLike(t *testing.T) {
	tests := []struct {
		name   string
		api    []globalRouters.RoutingTarget
		prior  []globalRouters.RoutingTarget
		expect []string
	}{
		{
			name:   "api rotated the targets",
			api:    []globalRouters.RoutingTarget{target("pd", "pd"), target("http", "http"), target("slack", "slack")},
			prior:  []globalRouters.RoutingTarget{target("http", "http"), target("slack", "slack"), target("pd", "pd")},
			expect: []string{"http/http", "slack/slack", "pd/pd"},
		},
		{
			name:   "target added outside terraform goes last",
			api:    []globalRouters.RoutingTarget{target("new", ""), target("pd", "pd"), target("http", "http")},
			prior:  []globalRouters.RoutingTarget{target("http", "http"), target("pd", "pd")},
			expect: []string{"http/http", "pd/pd", "new/"},
		},
		{
			name:   "target removed outside terraform is dropped",
			api:    []globalRouters.RoutingTarget{target("pd", "pd")},
			prior:  []globalRouters.RoutingTarget{target("http", "http"), target("pd", "pd")},
			expect: []string{"pd/pd"},
		},
		{
			name:   "same connector with different presets match on preset",
			api:    []globalRouters.RoutingTarget{target("http", "b"), target("http", ""), target("http", "a")},
			prior:  []globalRouters.RoutingTarget{target("http", "a"), target("http", "b"), target("http", "")},
			expect: []string{"http/a", "http/b", "http/"},
		},
		{
			name:   "duplicate targets are each matched once",
			api:    []globalRouters.RoutingTarget{target("pd", ""), target("http", ""), target("pd", "")},
			prior:  []globalRouters.RoutingTarget{target("pd", ""), target("pd", ""), target("http", "")},
			expect: []string{"pd/", "pd/", "http/"},
		},
		{
			// Probe on eu2: the API accepts this pair and can return it in any order.
			name:   "same connector and preset with different custom details",
			api:    []globalRouters.RoutingTarget{targetWithDetails("http", "", "2"), targetWithDetails("http", "", "1")},
			prior:  []globalRouters.RoutingTarget{targetWithDetails("http", "", "1"), targetWithDetails("http", "", "2")},
			expect: []string{"http/{k=1}", "http/{k=2}"},
		},
		{
			// An exact match must not take a slot that an earlier key-only match would take.
			name:   "exact match wins over an earlier key-only match",
			api:    []globalRouters.RoutingTarget{targetWithDetails("http", "", "1"), targetWithDetails("http", "", "changed")},
			prior:  []globalRouters.RoutingTarget{targetWithDetails("http", "", "2"), targetWithDetails("http", "", "1")},
			expect: []string{"http/{k=changed}", "http/{k=1}"},
		},
		{
			name:   "custom details changed outside terraform keeps position",
			api:    []globalRouters.RoutingTarget{target("pd", ""), targetWithDetails("http", "", "new")},
			prior:  []globalRouters.RoutingTarget{targetWithDetails("http", "", "old"), target("pd", "")},
			expect: []string{"http/{k=new}", "pd/"},
		},
		{
			// Probe on eu2: 1 of 10 creates with mixed connector types came back rotated.
			name:   "api rotated mixed connector types",
			api:    []globalRouters.RoutingTarget{target("http-b", ""), target("http-a", ""), target("pd", "")},
			prior:  []globalRouters.RoutingTarget{target("http-a", ""), target("pd", ""), target("http-b", "")},
			expect: []string{"http-a/", "pd/", "http-b/"},
		},
		{
			name:   "no prior keeps the api order",
			api:    []globalRouters.RoutingTarget{target("pd", ""), target("http", "")},
			expect: []string{"pd/", "http/"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := targetKeys(orderTargetsLike(tt.api, tt.prior))
			if !slices.Equal(got, tt.expect) {
				t.Fatalf("got %v, want %v", got, tt.expect)
			}
		})
	}
}

// A rule without targets must stay nil, so it still flattens to a null list.
func TestOrderTargetsLikeKeepsNil(t *testing.T) {
	if got := orderTargetsLike(nil, []globalRouters.RoutingTarget{target("pd", "")}); got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}

func TestAlignRuleTargetsPairsRulesByIndex(t *testing.T) {
	rules := []globalRouters.RoutingRule{
		{Targets: []globalRouters.RoutingTarget{target("b", ""), target("a", "")}},
		{Targets: []globalRouters.RoutingTarget{target("d", ""), target("c", "")}},
	}
	prior := []globalRouters.RoutingRule{
		{Targets: []globalRouters.RoutingTarget{target("a", ""), target("b", "")}},
	}

	alignRuleTargets(rules, prior)

	if got := targetKeys(rules[0].Targets); !slices.Equal(got, []string{"a/", "b/"}) {
		t.Fatalf("rule 0: got %v, want [a/ b/]", got)
	}
	// Rule 1 has no prior rule, so it keeps the API order.
	if got := targetKeys(rules[1].Targets); !slices.Equal(got, []string{"d/", "c/"}) {
		t.Fatalf("rule 1: got %v, want [d/ c/]", got)
	}
}

// Import starts with only an ID, so Read gets null rules from state.
func TestExtractGlobalRouterRulesFromNullState(t *testing.T) {
	rules, diags := extractGlobalRouterRules(context.Background(), types.ListNull(types.ObjectType{AttrTypes: globalrouterschema.RoutingRuleAttr()}))
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(rules) != 0 {
		t.Fatalf("got %d rules, want 0", len(rules))
	}
}
