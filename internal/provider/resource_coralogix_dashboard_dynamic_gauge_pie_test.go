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
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
)

const dashboardOpenAPIDynamicGaugePieTestName = "TestAccCoralogixResourceDashboardDynamicGaugeAndPieWidgets"

func TestAccCoralogixResourceDashboardDynamicGaugeAndPieWidgets(t *testing.T) {
	ctx := context.Background()
	var client *dashboardservice.DashboardServiceAPIService
	fixture := t.Name()
	name := dashboardOpenAPIFixtureName(fixture)

	gauge := "layout.sections.0.rows.0.widgets.0.definition.dynamic.visualization.gauge."
	pie := "layout.sections.0.rows.0.widgets.1.definition.dynamic.visualization.pie_chart."

	backendCheck := func(state *terraform.State) error {
		dashboard, err := dashboardOpenAPIFetchDashboard(ctx, client, state, dashboardResourceName, fixture)
		if err != nil {
			return err
		}
		return dashboardOpenAPIAssertDynamicGaugeAndPieWidgets(dashboard, fixture)
	}

	steps := dashboardOpenAPIStructuredLifecycleSteps(
		dashboardOpenAPILifecyclePhase{
			Config: testAccCoralogixResourceDashboardDynamicGaugePieConfig(name, "gauge", true),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),

				resource.TestCheckResourceAttr(dashboardResourceName, gauge+"unit", "seconds"),
				resource.TestCheckResourceAttr(dashboardResourceName, gauge+"min", "0"),
				resource.TestCheckResourceAttr(dashboardResourceName, gauge+"max", "1000"),
				resource.TestCheckResourceAttr(dashboardResourceName, gauge+"show_inner_arc", "true"),
				resource.TestCheckResourceAttr(dashboardResourceName, gauge+"show_outer_arc", "true"),
				resource.TestCheckResourceAttr(dashboardResourceName, gauge+"arc_display.threshold_arc", "true"),
				resource.TestCheckResourceAttr(dashboardResourceName, gauge+"thresholds.0.color", "green"),
				resource.TestCheckResourceAttr(dashboardResourceName, gauge+"legend_by", "thresholds"),

				resource.TestCheckResourceAttr(dashboardResourceName, pie+"max_slices_per_chart", "10"),
				resource.TestCheckResourceAttr(dashboardResourceName, pie+"min_slice_percentage", "2"),
				resource.TestCheckResourceAttr(dashboardResourceName, pie+"show_total", "true"),
				resource.TestCheckResourceAttr(dashboardResourceName, pie+"label_definition.label_source", "inner"),
				resource.TestCheckResourceAttr(dashboardResourceName, pie+"label_definition.show_percentage", "false"),
				resource.TestCheckResourceAttr(dashboardResourceName, pie+"sub_category_fields.0.keypath.0", "severity"),

				backendCheck,
			),
		},
		[]dashboardOpenAPILifecyclePhase{
			{
				Config: testAccCoralogixResourceDashboardDynamicGaugePieConfig(name, "gauge updated", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.title", "gauge updated"),
					backendCheck,
				),
			},
			{
				// Removing the optional enums must reset them rather than keep the old value.
				Config: testAccCoralogixResourceDashboardDynamicGaugePieConfig(name, "gauge updated", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, gauge+"threshold_type", "unspecified"),
					resource.TestCheckResourceAttr(dashboardResourceName, gauge+"legend_by", "unspecified"),
					backendCheck,
				),
			},
		},
		resource.TestStep{
			ResourceName:      dashboardResourceName,
			ImportState:       true,
			ImportStateVerify: true,
			ImportStateCheck: dashboardOpenAPIImportDashboardCheck(ctx, &client, fixture, func(dashboard *dashboardservice.Dashboard) error {
				return dashboardOpenAPIAssertDynamicGaugeAndPieWidgets(dashboard, fixture)
			}),
		},
	)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			client = dashboardOpenAPIAcceptanceClient(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps:                    steps,
	})
}

func dashboardOpenAPIAssertDynamicGaugeAndPieWidgets(dashboard *dashboardservice.Dashboard, fixture string) error {
	sections := dashboard.Layout.Sections
	if len(sections) != 1 {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): sections = %d, want 1", fixture, dashboard.GetId(), len(sections))
	}
	widgets := sections[0].GetRows()[0].GetWidgets()
	if len(widgets) != 2 {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): widgets = %d, want 2", fixture, dashboard.GetId(), len(widgets))
	}

	for index, branch := range []string{"gauge", "pieChart"} {
		definition := widgets[index].Definition
		if definition == nil {
			return fmt.Errorf("dashboard fixture %q (dashboard %q): widget %d has no definition", fixture, dashboard.GetId(), index)
		}
		if err := dashboardOpenAPIAssertOneOfBranch(definition, "WidgetDefinition", "dynamic", dashboard.GetId(), fixture); err != nil {
			return err
		}
		visualization := definition.Dynamic.Visualization
		if visualization == nil {
			return fmt.Errorf("dashboard fixture %q (dashboard %q): widget %d has no visualization", fixture, dashboard.GetId(), index)
		}
		if err := dashboardOpenAPIAssertOneOfBranch(visualization, "Visualization", branch, dashboard.GetId(), fixture); err != nil {
			return err
		}
	}

	return nil
}

// Every list this surface exposes must reject an explicit empty list at plan
// time; otherwise it passes the plan and fails the apply with an
// inconsistent-result error, because the API cannot store an empty list.
func TestAccCoralogixResourceDashboardDynamicGaugePieRejectsEmptyLists(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())

	for attribute, visualization := range map[string]string{
		"gauge_category_fields":   `gauge = { category_fields = [] }`,
		"gauge_value_fields":      `gauge = { value_fields = [] }`,
		"gauge_thresholds":        `gauge = { thresholds = [] }`,
		"pie_category_fields":     `pie_chart = { category_fields = [] }`,
		"pie_sub_category_fields": `pie_chart = { sub_category_fields = [] }`,
	} {
		t.Run(attribute, func(t *testing.T) {
			resource.ParallelTest(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{{
					Config: fmt.Sprintf(`resource "coralogix_dashboard" "test" {
  name = %q
  layout = { sections = [{ rows = [{
    height = 19
    widgets = [{
      title = "gauge"
      definition = { dynamic = {
        query_definitions = [{ query = { logs = { lucene_query = "*" } } }]
        visualization     = { %s }
      }}
    }]
  }] }] }
}
`, name, visualization),
					ExpectError: regexp.MustCompile(`(?s)list must contain at least 1 element`),
				}},
			})
		})
	}
}

func testAccCoralogixResourceDashboardDynamicGaugePieConfig(name, title string, setOptionalEnums bool) string {
	thresholdType, legendBy := "", ""
	if setOptionalEnums {
		thresholdType = `
                      threshold_type = "absolute"`
		legendBy = `
                      legend_by      = "thresholds"`
	}

	return fmt.Sprintf(`resource "coralogix_dashboard" "test" {
  name = %[1]q
  time_frame = { relative = { duration = "seconds:900" } }
  layout = {
    sections = [{
      rows = [{
        height = 19
        widgets = [
          {
            title = %[2]q
            definition = { dynamic = {
              query_definitions = [{
                name  = "rows"
                query = { logs = { lucene_query = "*" } }
              }]
              visualization = {
                gauge = {
                  allow_abbreviation = true
                  decimal_precision  = 2
                  min                = 0
                  max                = 1000
                  show_inner_arc     = true
                  show_outer_arc     = true
                  show_min_max       = false
                  unit               = "seconds"%[3]s%[4]s
                  arc_display = {
                    threshold_arc = true
                    value_arc     = false
                  }
                  thresholds = [
                    { from = 0, color = "green", label = "ok" },
                    { from = 500, color = "red", label = "slow" },
                  ]
                  value_field    = { keypath = ["duration"], scope = "metadata" }
                  category_fields = [{ keypath = ["applicationname"], scope = "label" }]
                }
              }
            }}
          },
          {
            title = "pie chart"
            definition = { dynamic = {
              query_definitions = [{
                name  = "rows"
                query = { logs = { lucene_query = "*" } }
              }]
              visualization = {
                pie_chart = {
                  allow_abbreviation   = false
                  decimal_precision    = 1
                  max_slices_per_chart = 10
                  max_slices_per_stack = 5
                  min_slice_percentage = 2
                  show_total           = true
                  unit                 = "bytes"
                  group_name_template  = "group"
                  stack_name_template  = "stack"
                  label_definition = {
                    is_visible      = true
                    label_source    = "inner"
                    show_name       = true
                    show_percentage = false
                    show_value      = true
                  }
                  value_field         = { keypath = ["duration"], scope = "metadata" }
                  category_fields     = [{ keypath = ["applicationname"], scope = "label" }]
                  sub_category_fields = [{ keypath = ["severity"], scope = "metadata" }]
                }
              }
            }}
          },
        ]
      }]
    }]
  }
}
`, name, title, thresholdType, legendBy)
}

// The proto documents bounds these attributes did not enforce: at least one
// slice per chart and per stack, and a percentage between 0 and 100.
func TestAccCoralogixResourceDashboardDynamicPieRejectsOutOfRangeSliceBounds(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())

	for scenario, visualization := range map[string]string{
		"zero slices per chart":        `pie_chart = { max_slices_per_chart = 0 }`,
		"zero slices per stack":        `pie_chart = { max_slices_per_stack = 0 }`,
		"percentage above one hundred": `pie_chart = { min_slice_percentage = 101 }`,
		"negative percentage":          `pie_chart = { min_slice_percentage = -1 }`,
	} {
		t.Run(scenario, func(t *testing.T) {
			resource.ParallelTest(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{{
					Config: fmt.Sprintf(`resource "coralogix_dashboard" "test" {
  name = %q
  layout = { sections = [{ rows = [{
    height = 19
    widgets = [{
      title = "pie"
      definition = { dynamic = {
        query_definitions = [{ query = { logs = { lucene_query = "*" } } }]
        visualization     = { %s }
      }}
    }]
  }] }] }
}
`, name, visualization),
					ExpectError: regexp.MustCompile(`(?s)Invalid Attribute Value`),
				}},
			})
		})
	}
}
