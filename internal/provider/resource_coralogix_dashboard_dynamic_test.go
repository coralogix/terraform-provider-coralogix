// Copyright 2024 Coralogix Ltd.
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

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccCoralogixResourceDashboardDynamicWidget(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	statThresholds := `{ from = 0, color = "green" }`
	statThresholdsUpdated := `{ from = 0, color = "green" }, { from = 1000, color = "red" }`
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceDashboardDynamicWidgets(name, "dynamic stat", statThresholds),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.title", "dynamic stat"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.dynamic.query_definitions.0.query.logs.aggregations.0.type", "count"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.dynamic.query_definitions.0.query.logs.group_by.0.keypath.0", "subsystemname"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.dynamic.query_definitions.0.query.logs.group_by.0.scope", "label"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.dynamic.visualization.stat.threshold_type", "absolute"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.title", "dynamic table"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.dynamic.visualization.table.settings.row_style", "one_line"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.dynamic.visualization.table.columns.0.field.keypath.0", "applicationname"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.0.title", "dynamic time series"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.0.definition.dynamic.visualization.time_series_lines_multi.stacked_line", "absolute"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.0.definition.dynamic.query_definitions.0.query.logs.aggregations.0.field", "meta.responseTime.numeric"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				Config: testAccCoralogixResourceDashboardDynamicWidgets(name, "dynamic stat updated", statThresholdsUpdated),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(dashboardResourceName, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.title", "dynamic stat updated"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.dynamic.visualization.stat.thresholds.1.color", "red"),
				),
			},
			{
				ResourceName:      dashboardResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCoralogixResourceDashboardDynamicWidgets(name, statTitle, statThresholds string) string {
	return fmt.Sprintf(`resource "coralogix_dashboard" "test" {
  name        = %q
  description = "dynamic widget acceptance coverage"
  time_frame = {
    relative = {
      duration = "seconds:900"
    }
  }
  layout = {
    sections = [{
      rows = [
        {
          height = 19
          widgets = [
            {
              title = %q
              definition = {
                dynamic = {
                  query_definitions = [{
                    name = "errors"
                    query = {
                      logs = {
                        lucene_query = "coralogix.metadata.severity=\"5\" OR coralogix.metadata.severity=\"6\""
                        group_by = [{
                          keypath = ["subsystemname"]
                          scope   = "label"
                        }]
                        aggregations = [{
                          type = "count"
                        }]
                      }
                    }
                  }]
                  time_frame = {
                    relative = {
                      duration = "seconds:900"
                    }
                  }
                  visualization = {
                    stat = {
                      decimal_precision = 0
                      threshold_type    = "absolute"
                      thresholds        = [%s]
                    }
                  }
                }
              }
            },
            {
              title = "dynamic table"
              definition = {
                dynamic = {
                  query_definitions = [{
                    name = "logs"
                    query = {
                      logs = {
                        lucene_query = "*"
                        group_by = [{
                          keypath = ["applicationname"]
                          scope   = "label"
                        }]
                        aggregations = [{
                          type = "count"
                        }]
                      }
                    }
                  }]
                  visualization = {
                    table = {
                      columns = [
                        { field = { keypath = ["applicationname"], scope = "label" } },
                        { field = { keypath = ["subsystemname"], scope = "label" } },
                      ]
                      settings = {
                        row_style = "one_line"
                      }
                    }
                  }
                }
              }
            },
          ]
        },
        {
          height = 19
          widgets = [{
            title = "dynamic time series"
            definition = {
              dynamic = {
                query_definitions = [{
                  name = "latency"
                  query = {
                    logs = {
                      lucene_query = "*"
                      group_by = [{
                        keypath = ["subsystemname"]
                        scope   = "label"
                      }]
                      aggregations = [{
                        type  = "avg"
                        field = "meta.responseTime.numeric"
                      }]
                    }
                  }
                }]
                visualization = {
                  time_series_lines_multi = {
                    connect_nulls      = true
                    stacked_line       = "absolute"
                    x_axis_time_format = "hh_mm"
                    tooltip = {
                      show_all_series = true
                      show_labels     = false
                    }
                  }
                }
              }
            }
          }]
        },
      ]
    }]
  }
}
`, name, statTitle, statThresholds)
}
