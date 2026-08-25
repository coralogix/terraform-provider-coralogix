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

const dashboardOpenAPIDynamicGeoHeatTestName = "TestAccCoralogixResourceDashboardDynamicSpatialWidgets"

func TestAccCoralogixResourceDashboardDynamicSpatialWidgets(t *testing.T) {
	ctx := context.Background()
	var client *dashboardservice.DashboardServiceAPIService
	fixture := t.Name()
	name := dashboardOpenAPIFixtureName(fixture)

	hexagon := "layout.sections.0.rows.0.widgets.0.definition.dynamic.visualization.hexagon_bins."
	heatmap := "layout.sections.0.rows.0.widgets.1.definition.dynamic.visualization.heatmap."
	geomap := "layout.sections.0.rows.1.widgets.0.definition.dynamic.visualization.geomap."

	backendCheck := func(state *terraform.State) error {
		dashboard, err := dashboardOpenAPIFetchDashboard(ctx, client, state, dashboardResourceName, fixture)
		if err != nil {
			return err
		}
		return dashboardOpenAPIAssertDynamicSpatialWidgets(dashboard, fixture)
	}

	steps := dashboardOpenAPIStructuredLifecycleSteps(
		dashboardOpenAPILifecyclePhase{
			Config: testAccCoralogixResourceDashboardDynamicSpatialConfig(name, "spatial", true),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),

				resource.TestCheckResourceAttr(dashboardResourceName, hexagon+"unit", "seconds"),
				resource.TestCheckResourceAttr(dashboardResourceName, hexagon+"min", "0"),
				resource.TestCheckResourceAttr(dashboardResourceName, hexagon+"max", "1000"),
				resource.TestCheckResourceAttr(dashboardResourceName, hexagon+"thresholds.0.color", "green"),
				resource.TestCheckResourceAttr(dashboardResourceName, hexagon+"legend_by", "thresholds"),

				// color_axis_min/max are float32 on the wire, so a fractional
				// value proves the semantic-equality type is doing its job.
				resource.TestCheckResourceAttr(dashboardResourceName, heatmap+"color_axis_max", "99.5"),
				resource.TestCheckResourceAttr(dashboardResourceName, heatmap+"color_axis_min", "0"),
				resource.TestCheckResourceAttr(dashboardResourceName, heatmap+"color_range", "blue"),
				resource.TestCheckResourceAttr(dashboardResourceName, heatmap+"show_numbers", "true"),
				resource.TestCheckResourceAttr(dashboardResourceName, heatmap+"x_axis_time_format", "hh_mm"),
				resource.TestCheckResourceAttr(dashboardResourceName, heatmap+"tooltip.message_template", "value = {{_count}}"),

				// count is an empty marker on the wire; the other arms carry a field.
				resource.TestCheckResourceAttr(dashboardResourceName, geomap+"aggregation.count", "true"),
				resource.TestCheckResourceAttr(dashboardResourceName, geomap+"color.size", "blue"),
				resource.TestCheckResourceAttr(dashboardResourceName, geomap+"config.coordinate_config.latitude_field.keypath.0", "lat"),
				resource.TestCheckResourceAttr(dashboardResourceName, geomap+"min_max.custom.max", "50"),

				backendCheck,
			),
		},
		[]dashboardOpenAPILifecyclePhase{
			{
				Config: testAccCoralogixResourceDashboardDynamicSpatialConfig(name, "spatial updated", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.title", "spatial updated"),
					backendCheck,
				),
			},
			{
				// Removing the optional enums must reset them rather than keep the old value.
				Config: testAccCoralogixResourceDashboardDynamicSpatialConfig(name, "spatial updated", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, hexagon+"threshold_type", "unspecified"),
					resource.TestCheckResourceAttr(dashboardResourceName, heatmap+"scale_type", "unspecified"),
					backendCheck,
				),
			},
		},
		resource.TestStep{
			ResourceName:      dashboardResourceName,
			ImportState:       true,
			ImportStateVerify: true,
			ImportStateCheck: dashboardOpenAPIImportDashboardCheck(ctx, &client, fixture, func(dashboard *dashboardservice.Dashboard) error {
				return dashboardOpenAPIAssertDynamicSpatialWidgets(dashboard, fixture)
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

func dashboardOpenAPIAssertDynamicSpatialWidgets(dashboard *dashboardservice.Dashboard, fixture string) error {
	expected := []struct {
		row    int
		widget int
		branch string
	}{
		{row: 0, widget: 0, branch: "hexagonBins"},
		{row: 0, widget: 1, branch: "heatmap"},
		{row: 1, widget: 0, branch: "geomap"},
		{row: 1, widget: 1, branch: "geomap"},
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
			return fmt.Errorf("dashboard fixture %q (dashboard %q): row %d widget %d has no visualization", fixture, dashboard.GetId(), want.row, want.widget)
		}
		if err := dashboardOpenAPIAssertOneOfBranch(visualization, "Visualization", want.branch, dashboard.GetId(), fixture); err != nil {
			return err
		}
	}

	// The geomap unions carry an empty marker in one arm and a field in the
	// others, so assert the backend stored exactly one arm of each rather than
	// trusting Terraform state.
	geomap := rows[1].GetWidgets()[0].Definition.Dynamic.Visualization.Geomap
	if geomap == nil || geomap.Aggregation == nil {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): geomap has no aggregation", fixture, dashboard.GetId())
	}
	if geomap.Aggregation.Count == nil {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): geomap aggregation count marker was not stored", fixture, dashboard.GetId())
	}
	if geomap.Aggregation.Sum != nil || geomap.Aggregation.Avg != nil {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): geomap aggregation stored more than one arm", fixture, dashboard.GetId())
	}
	if geomap.Config == nil || geomap.Config.CoordinateConfig == nil {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): geomap coordinate config was not stored", fixture, dashboard.GetId())
	}
	if geomap.Config.AwsRegionConfig != nil {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): geomap field config stored two arms", fixture, dashboard.GetId())
	}

	// The second geomap carries the other arm of each union, so the lifecycle
	// covers all of them rather than only the ones the first widget sets.
	fieldBased := rows[1].GetWidgets()[1].Definition.Dynamic.Visualization.Geomap
	if fieldBased == nil || fieldBased.Aggregation == nil || fieldBased.Aggregation.Sum == nil {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): the field-based geomap aggregation was not stored", fixture, dashboard.GetId())
	}
	if fieldBased.Aggregation.Count != nil {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): the field-based geomap stored two aggregation arms", fixture, dashboard.GetId())
	}
	if fieldBased.Color == nil || fieldBased.Color.ColorRange == nil {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): the geomap gradient colour was not stored", fixture, dashboard.GetId())
	}
	if fieldBased.Config == nil || fieldBased.Config.AwsRegionConfig == nil {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): the geomap AWS region config was not stored", fixture, dashboard.GetId())
	}

	return nil
}

// Every list this surface exposes must reject an explicit empty list at plan
// time; otherwise it passes the plan and fails the apply with an
// inconsistent-result error, because the API cannot store an empty list.
func TestAccCoralogixResourceDashboardDynamicSpatialRejectsEmptyLists(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())

	for attribute, visualization := range map[string]string{
		"hexagon_category_fields": `hexagon_bins = { category_fields = [] }`,
		"hexagon_thresholds":      `hexagon_bins = { thresholds = [] }`,
		"heatmap_x_axis_fields":   `heatmap = { x_axis_fields = [] }`,
		"heatmap_y_axis_fields":   `heatmap = { y_axis_fields = [] }`,
		"heatmap_tooltip_labels":  `heatmap = { tooltip = { labels = [] } }`,
		"geomap_tooltip_labels":   `geomap = { tooltip = { labels = [] } }`,
	} {
		t.Run(attribute, func(t *testing.T) {
			resource.ParallelTest(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{{
					Config:      testAccCoralogixResourceDashboardDynamicSpatialVisualizationConfig(name, visualization),
					ExpectError: regexp.MustCompile(`(?s)list must contain at least 1 element`),
				}},
			})
		})
	}
}

// Both geomap unions and the heatmap colour pair must be resolved at plan time.
func TestAccCoralogixResourceDashboardDynamicSpatialRejectsInvalidUnions(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())

	for scenario, visualization := range map[string]string{
		"two aggregation arms": `geomap = {
          aggregation = {
            count = true
            sum   = { field = { keypath = ["duration"], scope = "metadata" } }
          }
        }`,
		"no aggregation arm": `geomap = {
          aggregation = {}
        }`,
		"two field config arms": `geomap = {
          config = {
            coordinate_config = {
              latitude_field  = { keypath = ["lat"], scope = "user_data" }
              longitude_field = { keypath = ["lon"], scope = "user_data" }
            }
            aws_region_config = {
              aws_region_field = { keypath = ["region"], scope = "user_data" }
            }
          }
        }`,
		"two colour arms": `geomap = {
          color = {
            size        = "blue"
            color_range = "green"
          }
        }`,
		// The heatmap arms sit directly on the visualization rather than in a
		// wrapper, so they are guarded by ConflictsWith instead.
		"heatmap preset and range": `heatmap = {
          preset      = "blue"
          color_range = "green"
        }`,
	} {
		t.Run(scenario, func(t *testing.T) {
			resource.ParallelTest(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{{
					Config:      testAccCoralogixResourceDashboardDynamicSpatialVisualizationConfig(name, visualization),
					ExpectError: regexp.MustCompile(`(?s)Invalid Attribute Combination`),
				}},
			})
		})
	}
}

func testAccCoralogixResourceDashboardDynamicSpatialVisualizationConfig(name, visualization string) string {
	return fmt.Sprintf(`resource "coralogix_dashboard" "test" {
  name = %q
  layout = { sections = [{ rows = [{
    height = 19
    widgets = [{
      title = "spatial"
      definition = { dynamic = {
        query_definitions = [{ query = { logs = { lucene_query = "*" } } }]
        visualization     = { %s }
      }}
    }]
  }] }] }
}
`, name, visualization)
}

func testAccCoralogixResourceDashboardDynamicSpatialConfig(name, title string, setOptionalEnums bool) string {
	thresholdType, scaleType := "", ""
	if setOptionalEnums {
		thresholdType = `
                    threshold_type = "absolute"`
		scaleType = `
                    scale_type = "linear"`
	}

	return fmt.Sprintf(`resource "coralogix_dashboard" "test" {
  name       = %[1]q
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
                query_definitions = [{ name = "rows", query = { logs = { lucene_query = "*" } } }]
                visualization = {
                  hexagon_bins = {
                    allow_abbreviation = true
                    decimal_precision  = 2
                    legend_by          = "thresholds"
                    min                = 0
                    max                = 1000
                    unit               = "seconds"%[3]s
                    thresholds = [
                      { from = 0, color = "green", label = "ok" },
                      { from = 500, color = "red", label = "high" },
                    ]
                    value_field     = { keypath = ["duration"], scope = "metadata" }
                    category_fields = [{ keypath = ["applicationname"], scope = "label" }]
                  }
                }
              }}
            },
            {
              title = "heatmap"
              definition = { dynamic = {
                query_definitions = [{ name = "rows", query = { logs = { lucene_query = "*" } } }]
                visualization = {
                  heatmap = {
                    allow_abbreviation = false
                    color_axis_max     = 99.5
                    color_axis_min     = 0
                    color_range        = "blue"
                    decimal_precision  = 3
                    show_numbers       = true
                    unit               = "bytes"
                    x_axis_time_format = "hh_mm"%[4]s
                    tooltip = {
                      message_template = "value = {{_count}}"
                      labels           = [{ keypath = ["applicationname"], scope = "label" }]
                    }
                    value_field   = { keypath = ["duration"], scope = "metadata" }
                    x_axis_fields = [{ keypath = ["timestamp"], scope = "metadata" }]
                    y_axis_fields = [{ keypath = ["severity"], scope = "metadata" }]
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
            title = "geomap"
            definition = { dynamic = {
              query_definitions = [{ name = "rows", query = { logs = { lucene_query = "*" } } }]
              visualization = {
                geomap = {
                  allow_abbreviation = true
                  decimal_precision  = 1
                  unit               = "usd"
                  aggregation        = { count = true }
                  color              = { size = "blue" }
                  config = {
                    coordinate_config = {
                      latitude_field  = { keypath = ["lat"], scope = "user_data" }
                      longitude_field = { keypath = ["lon"], scope = "user_data" }
                    }
                  }
                  min_max = {
                    custom = {
                      min = 5
                      max = 50
                    }
                  }
                  tooltip = {
                    message_template = "value = {{_count}}"
                    labels           = [{ keypath = ["applicationname"], scope = "label" }]
                  }
                }
              }
            }}
            },
            {
              # The other arm of each geomap union, so the acceptance lifecycle
              # covers all three rather than only the defaults.
              title = "geomap field based"
              definition = { dynamic = {
                query_definitions = [{ name = "rows", query = { logs = { lucene_query = "*" } } }]
                visualization = {
                  geomap = {
                    aggregation = {
                      sum = { field = { keypath = ["duration"], scope = "metadata" } }
                    }
                    color = { color_range = "green" }
                    config = {
                      aws_region_config = {
                        aws_region_field = { keypath = ["region"], scope = "user_data" }
                      }
                    }
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
`, name, title, thresholdType, scaleType)
}
