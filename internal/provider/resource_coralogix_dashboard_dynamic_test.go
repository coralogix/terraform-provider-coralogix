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
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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
				Config: testAccCoralogixResourceDashboardDynamicWidgets(name, "dynamic stat", statThresholds, true),
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
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.dynamic.visualization.table.rules.0.rule_scope.regex", "app.*"),
					resource.TestCheckNoResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.dynamic.visualization.table.rules.0.rule_scope.field_type"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.dynamic.visualization.table.rules.0.properties.0.definition.column_display_name", "App"),
					resource.TestCheckNoResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.dynamic.visualization.table.rules.0.properties.0.definition.alignment"),
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
				Config: testAccCoralogixResourceDashboardDynamicWidgets(name, "dynamic stat updated", statThresholdsUpdated, true),
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
				Config: testAccCoralogixResourceDashboardDynamicWidgets(name, "dynamic stat updated", statThresholdsUpdated, false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(dashboardResourceName, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.0.definition.dynamic.visualization.time_series_lines_multi.stacked_line", "unspecified"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.0.definition.dynamic.visualization.time_series_lines_multi.x_axis_time_format", "unspecified"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.0.definition.dynamic.visualization.time_series_lines_multi.connect_nulls", "true"),
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

func testAccCoralogixResourceDashboardDynamicWidgets(name, statTitle, statThresholds string, setTimeSeriesOptionalEnums bool) string {
	timeSeriesOptionalEnums := ""
	if setTimeSeriesOptionalEnums {
		timeSeriesOptionalEnums = `
                    stacked_line       = "absolute"
                    x_axis_time_format = "hh_mm"`
	}
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
                      rules = [{
                        name = "rename application column"
                        rule_scope = {
                          regex = "app.*"
                        }
                        properties = [{
                          definition = {
                            column_display_name = "App"
                          }
                        }]
                      }]
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
                    connect_nulls      = true%s
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
`, name, statTitle, statThresholds, timeSeriesOptionalEnums)
}

// dashboardDynamicVisualizationSpec describes one dynamic visualization branch
// exercised with a deliberately minimal configuration: every optional enum is
// omitted so the test observes whether the backend echoes the schema default or
// drops the field.
// imported holds attributes whose value-semantic-equality custom type keeps the
// configured spelling in ordinary state but has no prior state to preserve on a
// fresh import, so the backend's canonical spelling lands instead. The two
// spellings compare equal, so plans stay empty; the exact imported strings are
// asserted rather than ignored.
type dashboardDynamicVisualizationSpec struct {
	branch   string
	body     string
	checks   map[string]string
	absent   []string
	imported map[string]string
}

var dashboardDynamicVisualizationGroups = []struct {
	name    string
	widgets []dashboardDynamicVisualizationSpec
}{
	{
		name: "stat-card-and-time-series",
		widgets: []dashboardDynamicVisualizationSpec{
			{
				branch: "stat_card",
				body: `
                  color_label_mapping = {
                    value = {
                      sections = [{
                        value  = "ok"
                        map_to = "OK"
                        color  = "green"
                      }]
                    }
                  }
                  label = {
                    mapped_values = "{   }"
                  }`,
				checks: map[string]string{
					"legend_by":           "unspecified",
					"unit":                "unspecified",
					"label.mapped_values": "{   }",
					"color_label_mapping.value.sections.0.map_to": "OK",
				},
				imported: map[string]string{"label.mapped_values": "{}"},
			},
			{
				branch: "time_series_lines",
				body: `
                  connect_nulls = true`,
				checks: map[string]string{
					"scale_type":         "unspecified",
					"stacked_line":       "unspecified",
					"x_axis_time_format": "unspecified",
					"unit":               "unspecified",
				},
			},
			{
				branch: "time_series_bars",
				body: `
                  y_axis_max = 0.1`,
				checks: map[string]string{
					"bar_value_display":  "unspecified",
					"scale_type":         "unspecified",
					"sort_by":            "unspecified",
					"x_axis_time_format": "unspecified",
					"unit":               "unspecified",
					"y_axis_max":         "0.1",
				},
				imported: map[string]string{"y_axis_max": "0.10000000149011612"},
			},
		},
	},
	{
		name: "vertical-bars",
		widgets: []dashboardDynamicVisualizationSpec{
			{
				branch: "vertical_bars",
				body: `
                  colors_by = "stack"`,
				checks: map[string]string{
					"colors_by":         "stack",
					"bar_value_display": "unspecified",
					"scale_type":        "unspecified",
					"sort_by":           "unspecified",
					"unit":              "unspecified",
				},
			},
			{
				branch: "vertical_bars_multi",
				body: `
                  max_bars_per_chart = 10`,
				checks: map[string]string{
					"bar_value_display": "unspecified",
					"scale_type":        "unspecified",
					"unit":              "unspecified",
				},
			},
			{
				branch: "horizontal_bars",
				body: `
                  max_bars_per_chart = 10`,
				checks: map[string]string{
					"scale_type":     "unspecified",
					"sort_by":        "unspecified",
					"y_axis_view_by": "unspecified",
					"unit":           "unspecified",
				},
			},
		},
	},
	{
		name: "horizontal-bars-multi-gauge-pie",
		widgets: []dashboardDynamicVisualizationSpec{
			{
				branch: "horizontal_bars_multi",
				body: `
                  max_bars_per_chart = 10`,
				checks: map[string]string{
					"scale_type":     "unspecified",
					"y_axis_view_by": "unspecified",
					"unit":           "unspecified",
				},
			},
			{
				branch: "gauge",
				body: `
                  min = 0
                  max = 100`,
				checks: map[string]string{
					"threshold_type": "unspecified",
					"legend_by":      "unspecified",
					"unit":           "unspecified",
				},
			},
			{
				branch: "pie_chart",
				body: `
                  show_total = true`,
				checks: map[string]string{
					"unit": "unspecified",
				},
			},
		},
	},
	{
		name: "hexagon-heatmap-geomap",
		widgets: []dashboardDynamicVisualizationSpec{
			{
				branch: "hexagon_bins",
				body: `
                  min = 0
                  max = 100`,
				checks: map[string]string{
					"threshold_type": "unspecified",
					"legend_by":      "unspecified",
					"unit":           "unspecified",
				},
			},
			{
				branch: "heatmap",
				body: `
                  show_numbers = true`,
				checks: map[string]string{
					"histogram_bucket_unit": "unspecified",
					"scale_type":            "unspecified",
					"x_axis_time_format":    "unspecified",
					"unit":                  "unspecified",
				},
				absent: []string{"color_range", "preset"},
			},
			{
				branch: "geomap",
				body: `
                  aggregation = {
                    count = true
                  }
                  config = {
                    coordinate_config = {
                      latitude_field = {
                        keypath = ["geo", "latitude"]
                        scope   = "user_data"
                      }
                      longitude_field = {
                        keypath = ["geo", "longitude"]
                        scope   = "user_data"
                      }
                    }
                  }`,
				checks: map[string]string{
					"unit": "unspecified",
				},
			},
		},
	},
}

// TestAccCoralogixResourceDashboardDynamicVisualizationsMinimal applies every
// dynamic visualization that the richer dynamic-widget fixture does not cover,
// each with the smallest valid configuration, and asserts the second plan is
// empty. Omitting the optional enums is the point: it is the only way to detect
// a backend that drops a field the schema defaults to "unspecified".
func TestAccCoralogixResourceDashboardDynamicVisualizationsMinimal(t *testing.T) {
	t.Parallel()

	for _, group := range dashboardDynamicVisualizationGroups {
		group := group
		t.Run(group.name, func(t *testing.T) {
			name := dashboardOpenAPIFixtureName(t.Name())
			importCanonical := map[string]string{}
			checks := []resource.TestCheckFunc{
				resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
				resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.#", fmt.Sprintf("%d", len(group.widgets))),
			}
			for index, widget := range group.widgets {
				prefix := fmt.Sprintf("layout.sections.0.rows.0.widgets.%d.definition.dynamic.visualization.%s.", index, widget.branch)
				checks = append(checks, resource.TestCheckResourceAttr(
					dashboardResourceName,
					fmt.Sprintf("layout.sections.0.rows.0.widgets.%d.title", index),
					"dynamic "+widget.branch,
				))
				for attribute, want := range widget.checks {
					checks = append(checks, resource.TestCheckResourceAttr(dashboardResourceName, prefix+attribute, want))
				}
				for _, attribute := range widget.absent {
					checks = append(checks, resource.TestCheckNoResourceAttr(dashboardResourceName, prefix+attribute))
				}
				for attribute, want := range widget.imported {
					importCanonical[prefix+attribute] = want
				}
			}

			importIgnore := make([]string, 0, len(importCanonical))
			for attribute := range importCanonical {
				importIgnore = append(importIgnore, attribute)
			}
			sort.Strings(importIgnore)

			resource.ParallelTest(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				CheckDestroy:             testAccCheckDashboardDestroy(t),
				Steps: []resource.TestStep{
					{
						Config: testAccCoralogixResourceDashboardDynamicVisualizationConfig(name, group.widgets),
						Check:  resource.ComposeAggregateTestCheckFunc(checks...),
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PostApplyPostRefresh: []plancheck.PlanCheck{
								plancheck.ExpectEmptyPlan(),
							},
						},
					},
					{
						ResourceName:            dashboardResourceName,
						ImportState:             true,
						ImportStateVerify:       true,
						ImportStateVerifyIgnore: importIgnore,
						ImportStateCheck:        dashboardDynamicImportedAttributeCheck(group.name, importCanonical),
					},
				},
			})
		})
	}
}

func dashboardDynamicImportedAttributeCheck(fixture string, expected map[string]string) resource.ImportStateCheckFunc {
	return func(states []*terraform.InstanceState) error {
		if len(expected) == 0 {
			return nil
		}
		for _, state := range states {
			if state == nil || state.ID == "" || state.Ephemeral.Type != strings.SplitN(dashboardResourceName, ".", 2)[0] {
				continue
			}
			for attribute, want := range expected {
				got, ok := state.Attributes[attribute]
				if !ok {
					return fmt.Errorf("dashboard fixture %q import: attribute %q is absent from imported state", fixture, attribute)
				}
				if got != want {
					return fmt.Errorf("dashboard fixture %q import: attribute %q = %q, want %q", fixture, attribute, got, want)
				}
			}
			return nil
		}
		return fmt.Errorf("dashboard fixture %q import: dashboard state is absent", fixture)
	}
}

func testAccCoralogixResourceDashboardDynamicVisualizationConfig(name string, widgets []dashboardDynamicVisualizationSpec) string {
	var builder strings.Builder
	for _, widget := range widgets {
		builder.WriteString(fmt.Sprintf(`
            {
              title = "dynamic %s"
              definition = {
                dynamic = {
                  query_definitions = [{
                    name = %q
                    query = {
                      logs = {
                        lucene_query = "*"
                        aggregations = [{
                          type = "count"
                        }]
                      }
                    }
                  }]
                  visualization = {
                    %s = {%s
                    }
                  }
                }
              }
            },`, widget.branch, widget.branch, widget.branch, widget.body))
	}

	return fmt.Sprintf(`resource "coralogix_dashboard" "test" {
  name        = %q
  description = "dynamic visualization minimal-config acceptance coverage"
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
          widgets = [%s
          ]
        },
      ]
    }]
  }
}
`, name, builder.String())
}
