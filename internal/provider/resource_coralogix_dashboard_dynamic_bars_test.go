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

const dashboardOpenAPIDynamicBarsTestName = "TestAccCoralogixResourceDashboardDynamicBarsWidgets"

func TestAccCoralogixResourceDashboardDynamicBarsWidgets(t *testing.T) {
	ctx := context.Background()
	var client *dashboardservice.DashboardServiceAPIService
	fixture := t.Name()
	name := dashboardOpenAPIFixtureName(fixture)

	vertical := "layout.sections.0.rows.0.widgets.0.definition.dynamic.visualization.vertical_bars."
	verticalMulti := "layout.sections.0.rows.0.widgets.1.definition.dynamic.visualization.vertical_bars_multi."
	horizontal := "layout.sections.0.rows.1.widgets.0.definition.dynamic.visualization.horizontal_bars."
	horizontalMulti := "layout.sections.0.rows.1.widgets.1.definition.dynamic.visualization.horizontal_bars_multi."

	backendCheck := func(state *terraform.State) error {
		dashboard, err := dashboardOpenAPIFetchDashboard(ctx, client, state, dashboardResourceName, fixture)
		if err != nil {
			return err
		}
		return dashboardOpenAPIAssertDynamicBarsWidgets(dashboard, fixture)
	}

	steps := dashboardOpenAPIStructuredLifecycleSteps(
		dashboardOpenAPILifecyclePhase{
			Config: testAccCoralogixResourceDashboardDynamicBarsConfig(name, "bars", true),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),

				resource.TestCheckResourceAttr(dashboardResourceName, vertical+"bar_value_display", "top"),
				resource.TestCheckResourceAttr(dashboardResourceName, vertical+"colors_by", "stack"),
				resource.TestCheckResourceAttr(dashboardResourceName, vertical+"max_slices_per_bar", "5"),
				resource.TestCheckResourceAttr(dashboardResourceName, vertical+"sort_by", "value"),
				resource.TestCheckResourceAttr(dashboardResourceName, vertical+"y_axis_max", "99.5"),
				resource.TestCheckResourceAttr(dashboardResourceName, vertical+"category_fields.0.keypath.0", "applicationname"),

				// The sort strategy is a union; this arm is the empty-marker one.
				resource.TestCheckResourceAttr(dashboardResourceName, verticalMulti+"sort_order.order_direction", "asc"),
				resource.TestCheckResourceAttr(dashboardResourceName, verticalMulti+"sort_order.strategy.category", "true"),
				resource.TestCheckResourceAttr(dashboardResourceName, verticalMulti+"query_field_settings.0.query_id", dashboardOpenAPIDynamicBarsQueryID),
				resource.TestCheckResourceAttr(dashboardResourceName, verticalMulti+"scale_type", "logarithmic"),

				resource.TestCheckResourceAttr(dashboardResourceName, horizontal+"y_axis_view_by", "category"),
				resource.TestCheckResourceAttr(dashboardResourceName, horizontal+"display_on_bar", "true"),
				resource.TestCheckResourceAttr(dashboardResourceName, horizontal+"unit", "milliseconds"),

				// ...and this arm carries a query reference instead.
				resource.TestCheckResourceAttr(dashboardResourceName, horizontalMulti+"sort_order.strategy.query_value.query_id", dashboardOpenAPIDynamicBarsQueryID),
				resource.TestCheckResourceAttr(dashboardResourceName, horizontalMulti+"sort_order.order_direction", "desc"),
				resource.TestCheckResourceAttr(dashboardResourceName, horizontalMulti+"y_axis_view_by", "value"),

				backendCheck,
			),
		},
		[]dashboardOpenAPILifecyclePhase{
			{
				Config: testAccCoralogixResourceDashboardDynamicBarsConfig(name, "bars updated", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.title", "bars updated"),
					backendCheck,
				),
			},
			{
				// Removing the optional enums must reset them rather than keep the old value.
				Config: testAccCoralogixResourceDashboardDynamicBarsConfig(name, "bars updated", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, vertical+"scale_type", "unspecified"),
					resource.TestCheckResourceAttr(dashboardResourceName, horizontal+"y_axis_view_by", "unspecified"),
					backendCheck,
				),
			},
		},
		resource.TestStep{
			ResourceName:      dashboardResourceName,
			ImportState:       true,
			ImportStateVerify: true,
			ImportStateCheck: dashboardOpenAPIImportDashboardCheck(ctx, &client, fixture, func(dashboard *dashboardservice.Dashboard) error {
				return dashboardOpenAPIAssertDynamicBarsWidgets(dashboard, fixture)
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

func dashboardOpenAPIAssertDynamicBarsWidgets(dashboard *dashboardservice.Dashboard, fixture string) error {
	expected := []struct {
		row    int
		widget int
		branch string
	}{
		{row: 0, widget: 0, branch: "verticalBars"},
		{row: 0, widget: 1, branch: "verticalBarsMulti"},
		{row: 1, widget: 0, branch: "horizontalBars"},
		{row: 1, widget: 1, branch: "horizontalBarsMulti"},
	}

	sections := dashboard.Layout.Sections
	if len(sections) != 1 {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): sections = %d, want 1", fixture, dashboard.GetId(), len(sections))
	}
	rows := sections[0].GetRows()

	for _, want := range expected {
		if want.row >= len(rows) {
			return fmt.Errorf("dashboard fixture %q (dashboard %q): rows = %d, want row %d", fixture, dashboard.GetId(), len(rows), want.row)
		}
		widgets := rows[want.row].GetWidgets()
		if want.widget >= len(widgets) {
			return fmt.Errorf("dashboard fixture %q (dashboard %q): row %d widgets = %d, want widget %d", fixture, dashboard.GetId(), want.row, len(widgets), want.widget)
		}

		definition := widgets[want.widget].Definition
		if definition == nil {
			return fmt.Errorf("dashboard fixture %q (dashboard %q): row %d widget %d has no definition", fixture, dashboard.GetId(), want.row, want.widget)
		}
		if err := dashboardOpenAPIAssertOneOfBranch(definition, "WidgetDefinition", "dynamic", dashboard.GetId(), fixture); err != nil {
			return err
		}

		visualization := definition.Dynamic.Visualization
		if visualization == nil {
			return fmt.Errorf("dashboard fixture %q (dashboard %q): row %d widget %d dynamic visualization is nil", fixture, dashboard.GetId(), want.row, want.widget)
		}
		if err := dashboardOpenAPIAssertOneOfBranch(visualization, "Visualization", want.branch, dashboard.GetId(), fixture); err != nil {
			return err
		}
	}

	// The sort strategy is a union of an empty marker and a query reference, and
	// the wire form of the marker is an empty object. Assert the backend stored
	// exactly one arm per chart rather than trusting Terraform state.
	multi := rows[0].GetWidgets()[1].Definition.Dynamic.Visualization.VerticalBarsMulti
	if multi == nil || multi.SortOrder == nil || multi.SortOrder.Strategy == nil {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): vertical bars multi has no sort strategy", fixture, dashboard.GetId())
	}
	if multi.SortOrder.Strategy.Category == nil {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): sort strategy category marker was not stored", fixture, dashboard.GetId())
	}
	if multi.SortOrder.Strategy.QueryValue != nil {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): sort strategy stored two arms", fixture, dashboard.GetId())
	}

	return nil
}

const dashboardOpenAPIDynamicBarsQueryID = "9d1b7a4e-0000-4000-8000-00000000ba01"

// Every list this surface exposes must reject an explicit empty list at plan
// time; otherwise it passes the plan and fails the apply with an
// inconsistent-result error, because the API cannot store an empty list.
func TestAccCoralogixResourceDashboardDynamicBarsRejectsEmptyLists(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())

	for attribute, visualization := range map[string]string{
		"vertical_category_fields":         `vertical_bars = { category_fields = [] }`,
		"vertical_sub_category_fields":     `vertical_bars = { sub_category_fields = [] }`,
		"vertical_multi_category_fields":   `vertical_bars_multi = { category_fields = [] }`,
		"horizontal_category_fields":       `horizontal_bars = { category_fields = [] }`,
		"horizontal_sub_category_fields":   `horizontal_bars = { sub_category_fields = [] }`,
		"horizontal_multi_category_fields": `horizontal_bars_multi = { category_fields = [] }`,
	} {
		t.Run(attribute, func(t *testing.T) {
			resource.ParallelTest(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{{
					Config:      testAccCoralogixResourceDashboardDynamicBarsVisualizationConfig(name, visualization),
					ExpectError: regexp.MustCompile(`(?s)list must contain at least 1 element`),
				}},
			})
		})
	}
}

// The sort strategy is a union: exactly one of the marker and the query
// reference. Setting both, or neither, reaches the API as an invalid request
// unless the plan rejects it first.
func TestAccCoralogixResourceDashboardDynamicBarsRejectsInvalidSortStrategy(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())

	for scenario, visualization := range map[string]string{
		"both arms": `vertical_bars_multi = {
          sort_order = {
            strategy = {
              category    = true
              query_value = { query_id = "9d1b7a4e-0000-4000-8000-00000000ba01" }
            }
          }
        }`,
		"no arm": `vertical_bars_multi = {
          sort_order = {
            strategy = {}
          }
        }`,
	} {
		t.Run(scenario, func(t *testing.T) {
			resource.ParallelTest(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{{
					Config:      testAccCoralogixResourceDashboardDynamicBarsVisualizationConfig(name, visualization),
					ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
				}},
			})
		})
	}
}

func testAccCoralogixResourceDashboardDynamicBarsVisualizationConfig(name, visualization string) string {
	return fmt.Sprintf(`resource "coralogix_dashboard" "test" {
  name = %q
  layout = { sections = [{ rows = [{
    height = 19
    widgets = [{
      title = "bars"
      definition = { dynamic = {
        query_definitions = [{ query = { logs = { lucene_query = "*" } } }]
        visualization     = { %s }
      }}
    }]
  }] }] }
}
`, name, visualization)
}

func testAccCoralogixResourceDashboardDynamicBarsConfig(name, title string, setOptionalEnums bool) string {
	scaleType, yAxisViewBy := "", ""
	if setOptionalEnums {
		scaleType = `
                      scale_type = "linear"`
		yAxisViewBy = `
                      y_axis_view_by = "category"`
	}

	return fmt.Sprintf(`resource "coralogix_dashboard" "test" {
  name = %[1]q
  time_frame = { relative = { duration = "seconds:900" } }
  layout = {
    sections = [{
      rows = [
        {
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
                  vertical_bars = {
                    allow_abbreviation = true
                    bar_value_display  = "top"
                    colors_by          = "stack"
                    color_scheme       = "classic"
                    decimal_precision  = 2
                    max_bars_per_chart = 20
                    max_slices_per_bar = 5
                    sort_by            = "value"
                    unit               = "seconds"
                    y_axis_max         = 99.5
                    y_axis_min         = 0%[3]s
                    category_fields = [{ keypath = ["applicationname"], scope = "label" }]
                    value_field     = { keypath = ["duration"], scope = "metadata" }
                  }
                }
              }}
            },
            {
              title = "vertical bars multi"
              definition = { dynamic = {
                query_definitions = [{
                  id    = %[5]q
                  name  = "rows"
                  query = { logs = { lucene_query = "*" } }
                }]
                visualization = {
                  vertical_bars_multi = {
                    colors_by  = "query"
                    scale_type = "logarithmic"
                    unit       = "bytes"
                    sort_order = {
                      order_direction = "asc"
                      strategy        = { category = true }
                    }
                    query_field_settings = [{
                      query_id    = %[5]q
                      value_field = { keypath = ["duration"], scope = "metadata" }
                    }]
                    category_fields = [{ keypath = ["applicationname"], scope = "label" }]
                  }
                }
              }}
            },
          ]
        },
        {
          height = 19
          widgets = [
            {
              title = "horizontal bars"
              definition = { dynamic = {
                query_definitions = [{
                  name  = "rows"
                  query = { logs = { lucene_query = "*" } }
                }]
                visualization = {
                  horizontal_bars = {
                    display_on_bar = true
                    colors_by      = "group_by"
                    unit           = "milliseconds"
                    sort_by        = "name"%[4]s
                    category_fields = [{ keypath = ["applicationname"], scope = "label" }]
                    value_field     = { keypath = ["duration"], scope = "metadata" }
                  }
                }
              }}
            },
            {
              title = "horizontal bars multi"
              definition = { dynamic = {
                query_definitions = [{
                  id    = %[5]q
                  name  = "rows"
                  query = { logs = { lucene_query = "*" } }
                }]
                visualization = {
                  horizontal_bars_multi = {
                    colors_by      = "aggregation"
                    y_axis_view_by = "value"
                    unit           = "usd"
                    sort_order = {
                      order_direction = "desc"
                      strategy = {
                        query_value = { query_id = %[5]q }
                      }
                    }
                    query_field_settings = [{
                      query_id    = %[5]q
                      value_field = { keypath = ["duration"], scope = "metadata" }
                    }]
                    category_fields = [{ keypath = ["applicationname"], scope = "label" }]
                  }
                }
              }}
            },
          ]
        },
      ]
    }]
  }
}
`, name, title, scaleType, yAxisViewBy, dashboardOpenAPIDynamicBarsQueryID)
}
