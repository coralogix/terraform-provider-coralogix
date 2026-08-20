// Copyright 2026 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestDashboardFilterSelectionConfigParses(t *testing.T) {
	_, diagnostics := hclsyntax.ParseConfig([]byte(dashboardFilterSelectionConfig("dashboard")), "filter-selection.tf", hcl.InitialPos)
	if diagnostics.HasErrors() {
		t.Fatalf("filter selection config does not parse: %s", diagnostics.Error())
	}
}

func TestAccCoralogixResourceDashboardFilterSelectionTypes(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{{
			Config: dashboardFilterSelectionConfig(name),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
			},
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
				resource.TestCheckResourceAttr(dashboardResourceName, "filters.0.source.metrics.operator.selection_type", "all"),
				resource.TestCheckResourceAttr(dashboardResourceName, "filters.0.source.metrics.operator.selected_values.#", "0"),
				resource.TestCheckResourceAttr(dashboardResourceName, "filters.1.source.metrics.operator.selection_type", "list"),
				resource.TestCheckResourceAttr(dashboardResourceName, "filters.1.source.metrics.operator.selected_values.#", "0"),
			),
		}},
	})
}

func dashboardFilterSelectionConfig(name string) string {
	return fmt.Sprintf(`
resource "coralogix_dashboard" "test" {
  name        = %q
  description = "Filter selection round-trip"
  time_frame  = { relative = { duration = "seconds:900" } }
  layout = {
    sections = [{ rows = [{
      height = 10
      widgets = [{
        title = "filter-selection"
        definition = { line_chart = {
          query_definitions = [{
            query = { logs = { aggregations = [{ type = "count" }] } }
          }]
          legend = { is_visible = false }
        } }
      }]
    }] }]
  }
  filters = [
    {
      source = { metrics = {
        metric_name = "http_requests_total"
        label       = "service"
        operator    = { type = "equals", selected_values = [] }
      } }
    },
    {
      source = { metrics = {
        metric_name = "http_requests_total"
        label       = "service"
        operator    = { type = "equals", selection_type = "list", selected_values = [] }
      } }
    },
  ]
}
`, name)
}
