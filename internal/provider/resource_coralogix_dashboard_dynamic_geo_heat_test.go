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
	"sort"
	"strings"
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

	// Each geomap union must store exactly one arm. Asserted over every arm
	// rather than spot-checking a couple, so an extra arm appearing through a
	// converter or backend change cannot slip past the exactly-one contract
	// this test is what makes the coverage manifest claim.
	first := rows[1].GetWidgets()[0].Definition.Dynamic.Visualization.Geomap
	if first == nil {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): the first geomap was not stored", fixture, dashboard.GetId())
	}
	second := rows[1].GetWidgets()[1].Definition.Dynamic.Visualization.Geomap
	if second == nil {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): the second geomap was not stored", fixture, dashboard.GetId())
	}

	for _, want := range []struct {
		widget string
		union  string
		arm    string
		arms   map[string]bool
	}{
		{"first", "aggregation", "count", geomapAggregationArms(first.Aggregation)},
		{"first", "config", "coordinateConfig", geomapConfigArms(first.Config)},
		{"first", "color", "size", geomapColorArms(first.Color)},
		{"second", "aggregation", "sum", geomapAggregationArms(second.Aggregation)},
		{"second", "config", "awsRegionConfig", geomapConfigArms(second.Config)},
		{"second", "color", "colorRange", geomapColorArms(second.Color)},
	} {
		if err := dashboardOpenAPIAssertExactlyOneArm(want.widget, want.union, want.arm, want.arms, dashboard.GetId(), fixture); err != nil {
			return err
		}
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

// The arms of each geomap union, keyed by their wire names, so the assertion
// below can require exactly one without a caller having to list them.
func geomapAggregationArms(aggregation *dashboardservice.GeomapAggregation) map[string]bool {
	if aggregation == nil {
		return nil
	}
	return map[string]bool{
		"count": aggregation.Count != nil,
		"sum":   aggregation.Sum != nil,
		"min":   aggregation.Min != nil,
		"max":   aggregation.Max != nil,
		"avg":   aggregation.Avg != nil,
	}
}

func geomapConfigArms(config *dashboardservice.GeomapFieldConfig) map[string]bool {
	if config == nil {
		return nil
	}
	return map[string]bool{
		"coordinateConfig": config.CoordinateConfig != nil,
		"awsRegionConfig":  config.AwsRegionConfig != nil,
	}
}

func geomapColorArms(color *dashboardservice.GeomapColor) map[string]bool {
	if color == nil {
		return nil
	}
	return map[string]bool{
		"size":       color.Size != nil,
		"colorRange": color.ColorRange != nil,
	}
}

func dashboardOpenAPIAssertExactlyOneArm(widget, union, want string, arms map[string]bool, id, fixture string) error {
	if arms == nil {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): the %s geomap has no %s", fixture, id, widget, union)
	}

	var set []string
	for arm, selected := range arms {
		if selected {
			set = append(set, arm)
		}
	}
	sort.Strings(set)

	switch {
	case len(set) != 1:
		return fmt.Errorf("dashboard fixture %q (dashboard %q): the %s geomap %s stored %d arms (%s), want exactly 1",
			fixture, id, widget, union, len(set), strings.Join(set, ", "))
	case set[0] != want:
		return fmt.Errorf("dashboard fixture %q (dashboard %q): the %s geomap %s stored %q, want %q",
			fixture, id, widget, union, set[0], want)
	}

	return nil
}

// The converter cannot reach the assertion above: the SDK's MarshalJSON refuses
// two arms of a oneof before the request is sent. It guards the read path
// instead - a backend that returns two arms, or none - so the assertion is
// exercised here directly.
func TestDashboardOpenAPIAssertExactlyOneArm(t *testing.T) {
	for name, tc := range map[string]struct {
		arms      map[string]bool
		want      string
		wantError string
	}{
		"exactly the expected arm": {
			arms: map[string]bool{"count": true, "sum": false, "min": false},
			want: "count",
		},
		"two arms": {
			arms:      map[string]bool{"count": true, "min": true, "sum": false},
			want:      "count",
			wantError: "stored 2 arms (count, min), want exactly 1",
		},
		"no arm": {
			arms:      map[string]bool{"count": false, "sum": false},
			want:      "count",
			wantError: "stored 0 arms (), want exactly 1",
		},
		"the wrong arm": {
			arms:      map[string]bool{"count": false, "sum": true},
			want:      "count",
			wantError: `stored "sum", want "count"`,
		},
		"union absent entirely": {
			arms:      nil,
			want:      "count",
			wantError: "has no aggregation",
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := dashboardOpenAPIAssertExactlyOneArm("first", "aggregation", tc.want, tc.arms, "id", "fixture")
			switch {
			case tc.wantError == "" && err != nil:
				t.Errorf("expected no error, got %v", err)
			case tc.wantError != "" && err == nil:
				t.Errorf("expected an error containing %q, got none", tc.wantError)
			case tc.wantError != "" && !strings.Contains(err.Error(), tc.wantError):
				t.Errorf("expected an error containing %q, got %v", tc.wantError, err)
			}
		})
	}
}

// The API accepts values beyond the documented limits and stores them verbatim
// — confirmed by applying a 129-character unit, a 4097-character template and
// 1001 labels. These validators therefore enforce documentation rather than
// mirroring a backend rejection, so a test is the only thing holding them.
func TestAccCoralogixResourceDashboardDynamicRejectsOverDocumentedLimits(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	labels := strings.TrimSuffix(strings.Repeat(`{ keypath = ["applicationname"], scope = "label" },`, 1001), ",")

	for scenario, visualization := range map[string]string{
		"custom unit over 128": fmt.Sprintf(`heatmap = {
          unit        = "custom"
          custom_unit = %q
        }`, strings.Repeat("u", 129)),
		"message template over 4096": fmt.Sprintf(`heatmap = {
          tooltip = { message_template = %q }
        }`, strings.Repeat("t", 4097)),
		"labels over 1000": fmt.Sprintf(`heatmap = {
          tooltip = { labels = [%s] }
        }`, labels),
	} {
		t.Run(scenario, func(t *testing.T) {
			resource.ParallelTest(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{{
					Config:      testAccCoralogixResourceDashboardDynamicSpatialVisualizationConfig(name, visualization),
					ExpectError: regexp.MustCompile(`(?s)Invalid Attribute Value`),
				}},
			})
		})
	}
}

// A filter's selected_values deliberately allows an empty list, since that
// means "all values", so it carries a maximum-only validator. The documented
// 1000-item cap still applies, and an empty list must still be accepted.
func TestAccCoralogixResourceDashboardFilterSelectedValuesCap(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())

	config := func(values string) string {
		return fmt.Sprintf(`resource "coralogix_dashboard" "test" {
  name = %q
  layout = { sections = [{ rows = [{
    height = 19
    widgets = [{
      title = "filtered"
      definition = { dynamic = {
        query_definitions = [{
          query = { logs = {
            lucene_query = "*"
            filters = [{
              field    = "applicationname"
              operator = {
                type            = "equals"
                selection_type  = "list"
                selected_values = [%s]
              }
            }]
          }}
        }]
        visualization = { heatmap = { value_field = { keypath = ["duration"], scope = "metadata" } } }
      }}
    }]
  }] }] }
}
`, name, values)
	}

	over := strings.TrimSuffix(strings.Repeat(`"v",`, 1001), ",")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config:      config(over),
			ExpectError: regexp.MustCompile(`(?s)Invalid Attribute Value`),
		}},
	})
}
