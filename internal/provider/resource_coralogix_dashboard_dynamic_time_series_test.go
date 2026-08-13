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

const dashboardOpenAPIDynamicTimeSeriesTestName = "TestAccCoralogixResourceDashboardDynamicTimeSeriesWidgets"

func TestAccCoralogixResourceDashboardDynamicTimeSeriesWidgets(t *testing.T) {
	ctx := context.Background()
	var client *dashboardservice.DashboardServiceAPIService
	fixture := t.Name()
	name := dashboardOpenAPIFixtureName(fixture)

	linesPrefix := "layout.sections.0.rows.0.widgets.0.definition.dynamic.visualization.time_series_lines."
	multiPrefix := "layout.sections.0.rows.0.widgets.1.definition.dynamic.visualization.time_series_lines_multi."
	barsPrefix := "layout.sections.0.rows.1.widgets.0.definition.dynamic.visualization.time_series_bars."

	backendCheck := func(state *terraform.State) error {
		dashboard, err := dashboardOpenAPIFetchDashboard(ctx, client, state, dashboardResourceName, fixture)
		if err != nil {
			return err
		}
		return dashboardOpenAPIAssertDynamicTimeSeriesWidgets(dashboard, fixture)
	}

	steps := dashboardOpenAPIStructuredLifecycleSteps(
		dashboardOpenAPILifecyclePhase{
			Config: testAccCoralogixResourceDashboardDynamicTimeSeriesConfig(name, "time series lines", true),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),

				// time_series_lines: the deprecated singular variant, kept for import fidelity
				resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.title", "time series lines"),
				resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"allow_abbreviation", "true"),
				resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"connect_nulls", "true"),
				resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"color_scheme", "classic"),
				resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"custom_unit", "req/s"),
				resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"decimal_precision", "2"),
				resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"hash_colors", "true"),
				resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"scale_type", "linear"),
				resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"series_count_limit", "10"),
				resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"series_name_template", "{{severity}}"),
				resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"stacked_line", "absolute"),
				resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"unit", "custom"),
				resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"use_data_time_range", "true"),
				resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"x_axis_time_format", "hh_mm"),
				// float32 on the wire: whole and fractional values must both survive
				resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"y_axis_min", "0"),
				resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"y_axis_max", "99.5"),
				resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"category_fields.0.keypath.0", "applicationname"),
				resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"value_fields.0.keypath.0", "meta.responseTime.numeric"),
				resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"temporal_field.keypath.0", "coralogix.timestamp"),
				resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"tooltip.show_labels", "true"),
				resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"tooltip.show_all_series", "true"),
				resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"legend.is_visible", "true"),
				resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"legend.placement", "bottom"),

				// time_series_lines_multi: per-query display settings
				resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.title", "time series lines multi"),
				resource.TestCheckResourceAttr(dashboardResourceName, multiPrefix+"connect_nulls", "false"),
				resource.TestCheckResourceAttr(dashboardResourceName, multiPrefix+"stacked_line", "relative"),
				resource.TestCheckResourceAttr(dashboardResourceName, multiPrefix+"x_axis_time_format", "auto"),
				resource.TestCheckResourceAttr(dashboardResourceName, multiPrefix+"query_display_settings.0.query_id", "11111111-1111-4111-8111-111111111111"),
				resource.TestCheckResourceAttr(dashboardResourceName, multiPrefix+"query_display_settings.0.scale_type", "logarithmic"),
				resource.TestCheckResourceAttr(dashboardResourceName, multiPrefix+"query_display_settings.0.decimal_precision", "1"),
				resource.TestCheckResourceAttr(dashboardResourceName, multiPrefix+"query_display_settings.0.y_axis_max", "1000.25"),

				// time_series_bars
				resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.0.title", "time series bars"),
				resource.TestCheckResourceAttr(dashboardResourceName, barsPrefix+"bar_value_display", "top"),
				resource.TestCheckResourceAttr(dashboardResourceName, barsPrefix+"max_slices_per_bar", "5"),
				resource.TestCheckResourceAttr(dashboardResourceName, barsPrefix+"scale_type", "linear"),
				resource.TestCheckResourceAttr(dashboardResourceName, barsPrefix+"x_axis_time_format", "dd_mm"),
				resource.TestCheckResourceAttr(dashboardResourceName, barsPrefix+"y_axis_max", "50.5"),

				backendCheck,
			),
		},
		[]dashboardOpenAPILifecyclePhase{
			{
				Config: testAccCoralogixResourceDashboardDynamicTimeSeriesConfig(name, "time series lines updated", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.title", "time series lines updated"),
					backendCheck,
				),
			},
			{
				// Removing the optional enums must reset them rather than keep the old value.
				Config: testAccCoralogixResourceDashboardDynamicTimeSeriesConfig(name, "time series lines updated", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"scale_type", "unspecified"),
					resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"stacked_line", "unspecified"),
					resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"x_axis_time_format", "unspecified"),
					resource.TestCheckResourceAttr(dashboardResourceName, barsPrefix+"bar_value_display", "unspecified"),
					backendCheck,
				),
			},
		},
		resource.TestStep{
			ResourceName:      dashboardResourceName,
			ImportState:       true,
			ImportStateVerify: true,
			ImportStateCheck: dashboardOpenAPIImportDashboardCheck(ctx, &client, fixture, func(dashboard *dashboardservice.Dashboard) error {
				return dashboardOpenAPIAssertDynamicTimeSeriesWidgets(dashboard, fixture)
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

func dashboardOpenAPIAssertDynamicTimeSeriesWidgets(dashboard *dashboardservice.Dashboard, fixture string) error {
	expected := []struct {
		row    int
		widget int
		branch string
	}{
		{row: 0, widget: 0, branch: "timeSeriesLines"},
		{row: 0, widget: 1, branch: "timeSeriesLinesMulti"},
		{row: 1, widget: 0, branch: "timeSeriesBars"},
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

	return nil
}

// Every list this surface exposes must reject an explicit empty list at plan
// time; otherwise it passes the plan and fails the apply with an
// inconsistent-result error, because the API cannot store an empty list.
func TestAccCoralogixResourceDashboardDynamicTimeSeriesRejectsEmptyLists(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())

	for attribute, visualization := range map[string]string{
		"lines_category_fields": `time_series_lines = { category_fields = [] }`,
		"lines_value_fields":    `time_series_lines = { value_fields = [] }`,
		"bars_category_fields":  `time_series_bars = { category_fields = [] }`,
		"bars_value_fields":     `time_series_bars = { value_fields = [] }`,
		"multi_display_value_fields": `time_series_lines_multi = {
          query_display_settings = [{
            query_id     = "11111111-1111-4111-8111-111111111111"
            value_fields = []
          }]
        }`,
		"multi_display_category_fields": `time_series_lines_multi = {
          query_display_settings = [{
            query_id        = "11111111-1111-4111-8111-111111111111"
            category_fields = []
          }]
        }`,
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
      title = "empty list"
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

func testAccCoralogixResourceDashboardDynamicTimeSeriesConfig(name, linesTitle string, setOptionalEnums bool) string {
	linesEnums, barsEnums, multiEnums := "", "", ""
	if setOptionalEnums {
		linesEnums = `
                      scale_type         = "linear"
                      stacked_line       = "absolute"
                      x_axis_time_format = "hh_mm"`
		barsEnums = `
                      bar_value_display  = "top"
                      scale_type         = "linear"
                      x_axis_time_format = "dd_mm"`
		multiEnums = `
                      stacked_line       = "relative"
                      x_axis_time_format = "auto"`
	}

	return fmt.Sprintf(`resource "coralogix_dashboard" "test" {
  name        = %q
  description = "dynamic time series visualizations acceptance coverage"
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
                    name  = "latency"
                    query = { metrics = { promql_query = "vector(1)" } }
                  }]
                  visualization = {
                    time_series_lines = {
                      allow_abbreviation   = true
                      connect_nulls        = true
                      color_scheme         = "classic"
                      custom_unit          = "req/s"
                      decimal_precision    = 2
                      hash_colors          = true
                      series_count_limit   = 10
                      series_name_template = "{{severity}}"
                      unit                 = "custom"
                      use_data_time_range  = true
                      y_axis_min           = 0
                      y_axis_max           = 99.5%s
                      category_fields = [{
                        keypath = ["applicationname"]
                        scope   = "label"
                      }]
                      value_fields = [{
                        keypath = ["meta.responseTime.numeric"]
                        scope   = "user_data"
                      }]
                      temporal_field = {
                        keypath = ["coralogix.timestamp"]
                        scope   = "metadata"
                      }
                      tooltip = {
                        show_labels     = true
                        show_all_series = true
                      }
                      legend = {
                        is_visible = true
                        placement  = "bottom"
                      }
                    }
                  }
                }
              }
            },
            {
              title = "time series lines multi"
              definition = {
                dynamic = {
                  query_definitions = [{
                    id    = "11111111-1111-4111-8111-111111111111"
                    name  = "multi"
                    query = { metrics = { promql_query = "vector(2)" } }
                  }]
                  visualization = {
                    time_series_lines_multi = {
                      connect_nulls       = false
                      use_data_time_range = false%s
                      query_display_settings = [{
                        query_id          = "11111111-1111-4111-8111-111111111111"
                        scale_type        = "logarithmic"
                        decimal_precision = 1
                        y_axis_max        = 1000.25
                      }]
                      tooltip = {
                        show_labels     = false
                        show_all_series = false
                      }
                    }
                  }
                }
              }
            }
          ]
        },
        {
          height = 19
          widgets = [
            {
              title = "time series bars"
              definition = {
                dynamic = {
                  query_definitions = [{
                    name  = "bars"
                    query = { metrics = { promql_query = "vector(3)" } }
                  }]
                  visualization = {
                    time_series_bars = {
                      allow_abbreviation = true
                      max_slices_per_bar = 5
                      y_axis_max         = 50.5%s
                      value_fields = [{
                        keypath = ["meta.responseTime.numeric"]
                        scope   = "user_data"
                      }]
                      tooltip = {
                        show_labels     = true
                        show_all_series = false
                      }
                    }
                  }
                }
              }
            }
          ]
        }
      ]
    }]
  }
}
`, name, linesTitle, linesEnums, multiEnums, barsEnums)
}
