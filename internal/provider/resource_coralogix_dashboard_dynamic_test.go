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
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.0.definition.dynamic.interpretation", "trend_over_time_line"),
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
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.0.definition.dynamic.interpretation", "unspecified"),
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
	dynamicInterpretation := ""
	if setTimeSeriesOptionalEnums {
		timeSeriesOptionalEnums = `
                    stacked_line       = "absolute"
                    x_axis_time_format = "hh_mm"`
		dynamicInterpretation = `
                interpretation = "trend_over_time_line"`
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
                }]%s
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
`, name, statTitle, statThresholds, dynamicInterpretation, timeSeriesOptionalEnums)
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

func TestAccCoralogixResourceDashboardDynamicQuerySources(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	spansPrefix := "layout.sections.0.rows.0.widgets.0.definition.dynamic.query_definitions.0.query.spans."
	dataPrimePrefix := "layout.sections.0.rows.0.widgets.1.definition.dynamic.query_definitions.0.query.data_prime."
	metricsExplicitPrefix := "layout.sections.0.rows.1.widgets.0.definition.dynamic.query_definitions.0.query.metrics."
	metricsDefaultPrefix := "layout.sections.0.rows.1.widgets.1.definition.dynamic.query_definitions.0.query.metrics."

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceDashboardDynamicQuerySourcesConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),

					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.title", "dynamic spans"),
					resource.TestCheckResourceAttr(dashboardResourceName, spansPrefix+"lucene_query", "*"),
					resource.TestCheckResourceAttr(dashboardResourceName, spansPrefix+"group_by.0.keypath.0", "service"),
					resource.TestCheckResourceAttr(dashboardResourceName, spansPrefix+"group_by.0.keypath.1", "name"),
					resource.TestCheckResourceAttr(dashboardResourceName, spansPrefix+"group_by.0.scope", "user_data"),
					resource.TestCheckResourceAttr(dashboardResourceName, spansPrefix+"group_by.0.relation_type", "unspecified"),
					resource.TestCheckResourceAttr(dashboardResourceName, spansPrefix+"aggregations.0.type", "count"),
					resource.TestCheckResourceAttr(dashboardResourceName, spansPrefix+"filters.0.observation_field.keypath.0", "status"),
					resource.TestCheckResourceAttr(dashboardResourceName, spansPrefix+"filters.0.observation_field.keypath.1", "code"),
					resource.TestCheckResourceAttr(dashboardResourceName, spansPrefix+"filters.0.observation_field.scope", "user_data"),
					resource.TestCheckResourceAttr(dashboardResourceName, spansPrefix+"filters.0.observation_field.relation_type", "unspecified"),
					resource.TestCheckNoResourceAttr(dashboardResourceName, spansPrefix+"filters.0.field"),
					resource.TestCheckResourceAttr(dashboardResourceName, spansPrefix+"filters.0.operator.type", "equals"),
					resource.TestCheckResourceAttr(dashboardResourceName, spansPrefix+"filters.0.operator.selected_values.0", "error"),
					resource.TestCheckResourceAttr(dashboardResourceName, spansPrefix+"data_mode_type", "unspecified"),

					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.title", "dynamic data prime"),
					resource.TestCheckResourceAttr(dashboardResourceName, dataPrimePrefix+"query", "source logs | limit 10"),
					resource.TestCheckResourceAttr(dashboardResourceName, dataPrimePrefix+"data_mode_type", "unspecified"),

					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.0.title", "dynamic metrics explicit"),
					resource.TestCheckResourceAttr(dashboardResourceName, metricsExplicitPrefix+"promql_query", "vector(1)"),
					resource.TestCheckResourceAttr(dashboardResourceName, metricsExplicitPrefix+"promql_query_type", "instant"),
					resource.TestCheckResourceAttr(dashboardResourceName, metricsExplicitPrefix+"editor_mode", "text"),
					resource.TestCheckResourceAttr(dashboardResourceName, metricsExplicitPrefix+"series_limit_type", "by_series_count"),

					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.1.title", "dynamic metrics defaults"),
					resource.TestCheckResourceAttr(dashboardResourceName, metricsDefaultPrefix+"promql_query", "vector(2)"),
					resource.TestCheckResourceAttr(dashboardResourceName, metricsDefaultPrefix+"promql_query_type", "unspecified"),
					resource.TestCheckResourceAttr(dashboardResourceName, metricsDefaultPrefix+"editor_mode", "unspecified"),
					resource.TestCheckResourceAttr(dashboardResourceName, metricsDefaultPrefix+"series_limit_type", "unspecified"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:      dashboardResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCoralogixResourceDashboardDynamicQuerySourcesConfig(name string) string {
	return fmt.Sprintf(`resource "coralogix_dashboard" "test" {
  name        = %q
  description = "dynamic query source acceptance coverage"
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
              title = "dynamic spans"
              definition = {
                dynamic = {
                  query_definitions = [{
                    name = "spans errors"
                    query = {
                      spans = {
                        lucene_query = "*"
                        group_by = [{
                          keypath = ["service", "name"]
                          scope   = "user_data"
                        }]
                        aggregations = [{
                          type = "count"
                        }]
                        filters = [{
                          observation_field = {
                            keypath = ["status", "code"]
                            scope   = "user_data"
                          }
                          operator = {
                            type            = "equals"
                            selected_values = ["error"]
                          }
                        }]
                      }
                    }
                  }]
                  visualization = {
                    pie_chart = {
                      show_total = true
                    }
                  }
                }
              }
            },
            {
              title = "dynamic data prime"
              definition = {
                dynamic = {
                  query_definitions = [{
                    name = "dataprime logs"
                    query = {
                      data_prime = {
                        query = "source logs | limit 10"
                      }
                    }
                  }]
                  visualization = {
                    pie_chart = {
                      show_total = true
                    }
                  }
                }
              }
            },
          ]
        },
        {
          height = 19
          widgets = [
            {
              title = "dynamic metrics explicit"
              definition = {
                dynamic = {
                  query_definitions = [{
                    name = "metrics explicit"
                    query = {
                      metrics = {
                        promql_query      = "vector(1)"
                        promql_query_type = "instant"
                        editor_mode       = "text"
                        series_limit_type = "by_series_count"
                      }
                    }
                  }]
                  visualization = {
                    time_series_lines = {
                      connect_nulls = true
                    }
                  }
                }
              }
            },
            {
              title = "dynamic metrics defaults"
              definition = {
                dynamic = {
                  query_definitions = [{
                    name = "metrics defaults"
                    query = {
                      metrics = {
                        promql_query = "vector(2)"
                      }
                    }
                  }]
                  visualization = {
                    time_series_lines = {
                      connect_nulls = true
                    }
                  }
                }
              }
            },
          ]
        },
      ]
    }]
  }
}
`, name)
}

const (
	dashboardDynamicSeriesLimitQueryID     = "40000000-0000-4000-8000-000000000001"
	dashboardDynamicSeriesLimitDisplayUnit = "datetime_iso"
)

func TestAccCoralogixResourceDashboardOrderDirectionNone(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	barsPrefix := "layout.sections.0.rows.0.widgets.0.definition.dynamic.visualization.vertical_bars_multi."

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceDashboardOrderDirectionNoneConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
					resource.TestCheckResourceAttr(dashboardResourceName, barsPrefix+"sort_order.order_direction", "none"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables.0.definition.multi_select.values_order_direction", "none"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:      dashboardResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

var dashboardDynamicSortStrategyArms = []struct {
	name         string
	body         string
	strategyType string
}{
	{
		name:         "category",
		body:         `category = "{}"`,
		strategyType: "",
	},
	{
		name: "category-explicit-discriminator",
		body: `category      = "{}"
                        strategy_type = "STRATEGY_TYPE_CATEGORY"`,
		strategyType: "STRATEGY_TYPE_CATEGORY",
	},
	{
		name: "query-value",
		body: `query_value = {
                          query_id = "` + dashboardDynamicSortStrategyQueryID + `"
                        }`,
		strategyType: "",
	},
}

const dashboardDynamicSortStrategyQueryID = "40000000-0000-4000-8000-000000000002"

func TestAccCoralogixResourceDashboardDynamicBarsSortOrderStrategy(t *testing.T) {
	t.Parallel()

	for _, arm := range dashboardDynamicSortStrategyArms {
		arm := arm
		t.Run(arm.name, func(t *testing.T) {
			name := dashboardOpenAPIFixtureName(t.Name())
			prefix := "layout.sections.0.rows.0.widgets.0.definition.dynamic.visualization.vertical_bars_multi.sort_order."
			resource.ParallelTest(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				CheckDestroy:             testAccCheckDashboardDestroy(t),
				Steps: []resource.TestStep{
					{
						Config: testAccCoralogixResourceDashboardDynamicSortStrategyConfig(name, arm.body),
						Check: resource.ComposeAggregateTestCheckFunc(
							resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
							resource.TestCheckResourceAttr(dashboardResourceName, prefix+"order_direction", "desc"),
							resource.TestCheckResourceAttr(dashboardResourceName, prefix+"strategy.strategy_type", arm.strategyType),
						),
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PostApplyPostRefresh: []plancheck.PlanCheck{
								plancheck.ExpectEmptyPlan(),
							},
						},
					},
					{
						ResourceName:      dashboardResourceName,
						ImportState:       true,
						ImportStateVerify: true,
					},
				},
			})
		})
	}
}

func testAccCoralogixResourceDashboardDynamicSortStrategyConfig(name, strategy string) string {
	return fmt.Sprintf(`resource "coralogix_dashboard" "test" {
  name        = %q
  description = "dynamic bars sort order strategy acceptance coverage"
  time_frame = {
    relative = {
      duration = "seconds:900"
    }
  }
  layout = {
    sections = [{
      rows = [{
        height = 19
        widgets = [{
          title = "dynamic vertical bars multi sort strategy"
          definition = {
            dynamic = {
              query_definitions = [{
                id   = %q
                name = "errors"
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
                vertical_bars_multi = {
                  max_bars_per_chart = 10
                  sort_order = {
                    order_direction = "desc"
                    strategy = {
                      %s
                    }
                  }
                }
              }
            }
          }
        }]
      }]
    }]
  }
}
`, name, dashboardDynamicSortStrategyQueryID, strategy)
}

func testAccCoralogixResourceDashboardOrderDirectionNoneConfig(name string) string {
	return fmt.Sprintf(`resource "coralogix_dashboard" "test" {
  name        = %q
  description = "order direction none acceptance coverage"
  time_frame = {
    relative = {
      duration = "seconds:900"
    }
  }
  variables = [{
    name         = "subsystem"
    display_name = "Subsystem"
    definition = {
      multi_select = {
        selected_values        = ["staging"]
        values_order_direction = "none"
        source = {
          constant_list = ["staging", "production"]
        }
      }
    }
  }]
  layout = {
    sections = [{
      rows = [
        {
          height = 19
          widgets = [
            {
              title = "dynamic vertical bars multi sort order none"
              definition = {
                dynamic = {
                  query_definitions = [{
                    name = "errors"
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
                    vertical_bars_multi = {
                      max_bars_per_chart = 10
                      sort_order = {
                        order_direction = "none"
                      }
                    }
                  }
                }
              }
            },
          ]
        },
      ]
    }]
  }
}
`, name)
}

func TestAccCoralogixResourceDashboardDynamicSeriesCountLimit(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	linesPrefix := "layout.sections.0.rows.0.widgets.0.definition.dynamic.visualization.time_series_lines."
	multiPrefix := "layout.sections.0.rows.0.widgets.1.definition.dynamic.visualization.time_series_lines_multi."

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceDashboardDynamicSeriesCountLimitConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),

					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.title", "dynamic lines series count limit"),
					resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"series_count_limit", "100"),
					resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"unit", "percent"),
					resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"legend.columns.0", "simple_value"),
					resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"legend.columns.1", "max"),
					resource.TestCheckResourceAttr(dashboardResourceName, linesPrefix+"legend.is_visible", "true"),

					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.title", "dynamic lines multi series count limit"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.dynamic.query_definitions.0.id", dashboardDynamicSeriesLimitQueryID),
					resource.TestCheckResourceAttr(dashboardResourceName, multiPrefix+"query_display_settings.0.query_id", dashboardDynamicSeriesLimitQueryID),
					resource.TestCheckResourceAttr(dashboardResourceName, multiPrefix+"query_display_settings.0.series_count_limit", "250"),
					resource.TestCheckResourceAttr(dashboardResourceName, multiPrefix+"query_display_settings.0.unit", dashboardDynamicSeriesLimitDisplayUnit),
					resource.TestCheckResourceAttr(dashboardResourceName, multiPrefix+"legend.columns.0", "simple_value"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:      dashboardResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCoralogixResourceDashboardDynamicSeriesCountLimitConfig(name string) string {
	return fmt.Sprintf(`resource "coralogix_dashboard" "test" {
  name        = %q
  description = "dynamic series count limit acceptance coverage"
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
              title = "dynamic lines series count limit"
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
                    time_series_lines = {
                      connect_nulls      = true
                      series_count_limit = 100
                      unit               = "percent"
                      legend = {
                        is_visible = true
                        columns    = ["simple_value", "max"]
                      }
                    }
                  }
                }
              }
            },
            {
              title = "dynamic lines multi series count limit"
              definition = {
                dynamic = {
                  query_definitions = [{
                    id   = %q
                    name = "errors"
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
                    time_series_lines_multi = {
                      connect_nulls = true
                      legend = {
                        is_visible = true
                        columns    = ["simple_value"]
                      }
                      query_display_settings = [{
                        query_id           = %q
                        series_count_limit = 250
                        unit               = %q
                      }]
                    }
                  }
                }
              }
            },
          ]
        },
      ]
    }]
  }
}
`, name, dashboardDynamicSeriesLimitQueryID, dashboardDynamicSeriesLimitQueryID, dashboardDynamicSeriesLimitDisplayUnit)
}
