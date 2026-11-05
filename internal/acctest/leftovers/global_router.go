// Copyright 2025 Coralogix Ltd.
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

// Package leftovers deletes what cancelled acceptance runs left behind. It takes the clients as an
// argument and does not import the provider, so that the tests of the provider package can call it.
package leftovers

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
)

const globalRouterType = "coralogix_global_router"

// routingLabels is one set of routing labels. The API refuses a router whose routing labels another
// router already holds.
type routingLabels struct {
	Environment, Service, Team string
}

// GlobalRouters deletes every global router that holds exactly the routing labels of a
// coralogix_global_router in configs. A cancelled acceptance run leaves its router behind, and the
// routing labels of the test are pinned (each new value becomes a meta label that the API never
// deletes), so every later create would fail. A live router of another test cannot hold the same
// labels: the API refuses that.
func GlobalRouters(t *testing.T, clients *clientset.ClientSet, configs ...string) {
	t.Helper()
	wanted, err := globalRouterLabels(configs)
	if err != nil {
		t.Fatalf("read the routing labels of the test configs: %v", err)
	}
	if len(wanted) == 0 {
		return
	}
	routers := clients.GlobalRouters()
	ctx := context.Background()
	list, _, err := routers.GlobalRoutersServiceListGlobalRouters(ctx).Execute()
	if err != nil {
		t.Fatalf("list global routers: %v", err)
	}
	for _, router := range list.Routers {
		held := router.GetRoutingLabels()
		labels := routingLabels{Environment: held.GetEnvironment(), Service: held.GetService(), Team: held.GetTeam()}
		if router.GetId() == "" || !slices.Contains(wanted, labels) {
			continue
		}
		t.Logf("deleting global router %q (%s), left by an earlier run: it holds the routing labels %+v of this test",
			router.GetName(), router.GetId(), labels)
		if _, _, err := routers.GlobalRoutersServiceDeleteGlobalRouter(ctx, router.GetId()).Execute(); err != nil {
			t.Fatalf("delete global router %s: %v", router.GetId(), err)
		}
	}
}

// globalRouterLabels returns the routing labels of every coralogix_global_router in configs. A set
// without a value is left out, so that a router without routing labels is never deleted.
func globalRouterLabels(configs []string) ([]routingLabels, error) {
	var sets []routingLabels
	for _, config := range configs {
		file, diags := hclsyntax.ParseConfig([]byte(config), "config.tf", hcl.InitialPos)
		if diags.HasErrors() {
			return nil, diags
		}
		for _, block := range file.Body.(*hclsyntax.Body).Blocks {
			if block.Type != "resource" || len(block.Labels) == 0 || block.Labels[0] != globalRouterType {
				continue
			}
			attr, ok := block.Body.Attributes["routing_labels"]
			if !ok {
				continue
			}
			value, diags := attr.Expr.Value(nil)
			if diags.HasErrors() {
				return nil, fmt.Errorf("routing_labels of %s.%s must be a literal: %w", globalRouterType, block.Labels[1], diags)
			}
			labels := routingLabels{
				Environment: stringAttr(value, "environment"),
				Service:     stringAttr(value, "service"),
				Team:        stringAttr(value, "team"),
			}
			if labels != (routingLabels{}) && !slices.Contains(sets, labels) {
				sets = append(sets, labels)
			}
		}
	}
	return sets, nil
}

func stringAttr(object cty.Value, name string) string {
	if !object.Type().IsObjectType() || !object.Type().HasAttribute(name) {
		return ""
	}
	value := object.GetAttr(name)
	if value.IsNull() || !value.IsKnown() || value.Type() != cty.String {
		return ""
	}
	return value.AsString()
}
