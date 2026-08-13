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

const dashboardOpenAPIDynamicStatCardTestName = "TestAccCoralogixResourceDashboardDynamicStatCardWidget"

func TestAccCoralogixResourceDashboardDynamicStatCardWidget(t *testing.T) {
	ctx := context.Background()
	var client *dashboardservice.DashboardServiceAPIService
	fixture := t.Name()
	name := dashboardOpenAPIFixtureName(fixture)

	observationPrefix := "layout.sections.0.rows.0.widgets.0.definition.dynamic.visualization.stat_card."
	mappedPrefix := "layout.sections.0.rows.0.widgets.1.definition.dynamic.visualization.stat_card."
	regexPrefix := "layout.sections.0.rows.1.widgets.0.definition.dynamic.visualization.stat_card."

	backendCheck := func(state *terraform.State) error {
		dashboard, err := dashboardOpenAPIFetchDashboard(ctx, client, state, dashboardResourceName, fixture)
		if err != nil {
			return err
		}
		return dashboardOpenAPIAssertDynamicStatCardWidgets(dashboard, fixture)
	}

	steps := dashboardOpenAPIStructuredLifecycleSteps(
		dashboardOpenAPILifecyclePhase{
			Config: testAccCoralogixResourceDashboardDynamicStatCardConfig(name, "stat card observation", true, false),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),

				resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.title", "stat card observation"),
				resource.TestCheckResourceAttr(dashboardResourceName, observationPrefix+"allow_abbreviation", "true"),
				resource.TestCheckResourceAttr(dashboardResourceName, observationPrefix+"category_fields.0.keypath.0", "applicationname"),
				resource.TestCheckResourceAttr(dashboardResourceName, observationPrefix+"category_fields.0.scope", "label"),
				resource.TestCheckResourceAttr(dashboardResourceName, observationPrefix+"custom_unit", "requests"),
				resource.TestCheckResourceAttr(dashboardResourceName, observationPrefix+"decimal_precision", "3"),
				resource.TestCheckResourceAttr(dashboardResourceName, observationPrefix+"legend.is_visible", "true"),
				resource.TestCheckResourceAttr(dashboardResourceName, observationPrefix+"legend.placement", "bottom"),
				resource.TestCheckResourceAttr(dashboardResourceName, observationPrefix+"legend_by", "groups"),
				resource.TestCheckResourceAttr(dashboardResourceName, observationPrefix+"unit", "custom"),
				resource.TestCheckResourceAttr(dashboardResourceName, observationPrefix+"value_fields.0.keypath.0", "meta.responseTime.numeric"),
				resource.TestCheckResourceAttr(dashboardResourceName, observationPrefix+"value_fields.0.scope", "user_data"),

				// title/label/primary_value on the observation_field branch, with template variables
				resource.TestCheckResourceAttr(dashboardResourceName, observationPrefix+"title.template_text", "p99 {{field}}"),
				resource.TestCheckResourceAttr(dashboardResourceName, observationPrefix+"title.observation_field.keypath.0", "subsystemname"),
				resource.TestCheckResourceAttr(dashboardResourceName, observationPrefix+"title.observation_field.scope", "label"),
				resource.TestCheckResourceAttr(dashboardResourceName, observationPrefix+"title.template_variables.0.observation_field.keypath.0", "applicationname"),
				resource.TestCheckResourceAttr(dashboardResourceName, observationPrefix+"title.template_variables.0.observation_field.scope", "label"),
				resource.TestCheckResourceAttr(dashboardResourceName, observationPrefix+"label.observation_field.keypath.0", "severity"),
				resource.TestCheckResourceAttr(dashboardResourceName, observationPrefix+"primary_value.observation_field.keypath.0", "meta.responseTime.numeric"),

				// range colour mapping with an explicit min/max
				resource.TestCheckResourceAttr(dashboardResourceName, observationPrefix+"color_label_mapping.color_by", "value"),
				resource.TestCheckResourceAttr(dashboardResourceName, observationPrefix+"color_label_mapping.range.min_max.custom.min", "0"),
				resource.TestCheckResourceAttr(dashboardResourceName, observationPrefix+"color_label_mapping.range.min_max.custom.max", "500"),
				resource.TestCheckResourceAttr(dashboardResourceName, observationPrefix+"color_label_mapping.range.threshold_type", "absolute"),
				resource.TestCheckResourceAttr(dashboardResourceName, observationPrefix+"color_label_mapping.range.thresholds.0.from", "0"),
				resource.TestCheckResourceAttr(dashboardResourceName, observationPrefix+"color_label_mapping.range.thresholds.0.color", "green"),

				// second widget: the mapped_values marker branch on every visual element
				resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.title", "stat card mapped values"),
				resource.TestCheckResourceAttr(dashboardResourceName, mappedPrefix+"title.mapped_values", "true"),
				resource.TestCheckResourceAttr(dashboardResourceName, mappedPrefix+"primary_value.observation_field.keypath.0", "meta.responseTime.numeric"),
				resource.TestCheckResourceAttr(dashboardResourceName, mappedPrefix+"label.template_variables.0.mapped_values", "true"),
				resource.TestCheckResourceAttr(dashboardResourceName, mappedPrefix+"color_label_mapping.value.sections.0.value", "ok"),
				resource.TestCheckResourceAttr(dashboardResourceName, mappedPrefix+"color_label_mapping.value.sections.0.color", "green"),
				resource.TestCheckResourceAttr(dashboardResourceName, mappedPrefix+"color_label_mapping.value.sections.0.map_to", "Healthy"),

				// third widget: the regex mapping
				resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.0.title", "stat card regex"),
				resource.TestCheckResourceAttr(dashboardResourceName, regexPrefix+"color_label_mapping.regex.sections.0.value", "^err.*"),
				resource.TestCheckResourceAttr(dashboardResourceName, regexPrefix+"color_label_mapping.regex.sections.0.color", "red"),
				resource.TestCheckResourceAttr(dashboardResourceName, regexPrefix+"unit", "bytes"),
				resource.TestCheckResourceAttr(dashboardResourceName, regexPrefix+"custom_unit", "widgets"),

				backendCheck,
			),
		},
		[]dashboardOpenAPILifecyclePhase{
			{
				Config: testAccCoralogixResourceDashboardDynamicStatCardConfig(name, "stat card observation updated", true, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.title", "stat card observation updated"),
					// switching the min/max union arm: custom is replaced by the auto marker
					resource.TestCheckResourceAttr(dashboardResourceName, observationPrefix+"color_label_mapping.range.min_max.auto", "true"),
					resource.TestCheckNoResourceAttr(dashboardResourceName, observationPrefix+"color_label_mapping.range.min_max.custom.min"),
					backendCheck,
				),
			},
			{
				Config: testAccCoralogixResourceDashboardDynamicStatCardConfig(name, "stat card observation updated", false, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, observationPrefix+"legend_by", "unspecified"),
					resource.TestCheckResourceAttr(dashboardResourceName, observationPrefix+"color_label_mapping.color_by", "unspecified"),
					resource.TestCheckResourceAttr(dashboardResourceName, observationPrefix+"color_label_mapping.range.threshold_type", "unspecified"),
					backendCheck,
				),
			},
		},
		resource.TestStep{
			ResourceName:      dashboardResourceName,
			ImportState:       true,
			ImportStateVerify: true,
			ImportStateCheck: dashboardOpenAPIImportDashboardCheck(ctx, &client, fixture, func(dashboard *dashboardservice.Dashboard) error {
				return dashboardOpenAPIAssertDynamicStatCardWidgets(dashboard, fixture)
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

func dashboardOpenAPIAssertDynamicStatCardWidgets(dashboard *dashboardservice.Dashboard, fixture string) error {
	expected := []struct {
		row         int
		widget      int
		mappingType string
	}{
		{row: 0, widget: 0, mappingType: "range"},
		{row: 0, widget: 1, mappingType: "value"},
		{row: 1, widget: 0, mappingType: "regex"},
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
		if err := dashboardOpenAPIAssertOneOfBranch(visualization, "Visualization", "statCard", dashboard.GetId(), fixture); err != nil {
			return err
		}

		statCard := visualization.StatCard
		if statCard.Title == nil || statCard.Label == nil || statCard.PrimaryValue == nil {
			return fmt.Errorf("dashboard fixture %q (dashboard %q): row %d widget %d stat card is missing a visual element", fixture, dashboard.GetId(), want.row, want.widget)
		}
		if statCard.ColorLabelMapping == nil {
			return fmt.Errorf("dashboard fixture %q (dashboard %q): row %d widget %d stat card colorLabelMapping is nil", fixture, dashboard.GetId(), want.row, want.widget)
		}
		if err := dashboardOpenAPIAssertOneOfBranch(statCard.ColorLabelMapping, "ColorLabelMapping", want.mappingType, dashboard.GetId(), fixture); err != nil {
			return err
		}
	}

	return nil
}

func TestAccCoralogixResourceDashboardDynamicStatCardRejectsEmptyLists(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())

	for _, tc := range []struct {
		attribute string
		statCard  string
	}{
		{
			attribute: "category_fields",
			statCard:  `category_fields = []`,
		},
		{
			attribute: "value_fields",
			statCard:  `value_fields = []`,
		},
		{
			attribute: "template_variables",
			statCard: `title = {
                        template_text      = "t"
                        template_variables = []
                      }`,
		},
		{
			attribute: "range thresholds",
			statCard: `color_label_mapping = {
                        range = {
                          thresholds = []
                        }
                      }`,
		},
		{
			attribute: "value sections",
			statCard: `color_label_mapping = {
                        value = {
                          sections = []
                        }
                      }`,
		},
	} {
		t.Run(tc.attribute, func(t *testing.T) {
			resource.ParallelTest(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config:      testAccCoralogixResourceDashboardDynamicStatCardEmptyListConfig(name, tc.statCard),
						ExpectError: regexp.MustCompile(`(?s)(Invalid Attribute Value|list must contain at least 1 element)`),
					},
				},
			})
		})
	}
}

// Same documented 0-15 limit as the stat visualization; see the equivalent test
// there for why this is a plan-time guard rather than a mirror of the API.
func TestAccCoralogixResourceDashboardDynamicStatCardRejectsOutOfRangeDecimalPrecision(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())

	for _, precision := range []string{"16", "-1"} {
		t.Run(precision, func(t *testing.T) {
			resource.ParallelTest(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config:      testAccCoralogixResourceDashboardDynamicStatCardEmptyListConfig(name, "decimal_precision = "+precision),
						ExpectError: regexp.MustCompile(`(?s)must be between 0 and 15`),
					},
				},
			})
		})
	}
}

func TestAccCoralogixResourceDashboardDynamicStatCardRequiresOneColorMappingType(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())

	for _, tc := range []struct {
		scenario string
		mapping  string
	}{
		{
			scenario: "none",
			mapping:  `color_label_mapping = { color_by = "value" }`,
		},
		{
			scenario: "two",
			mapping: `color_label_mapping = {
                        value = { sections = [{ value = "ok", color = "green", map_to = "OK" }] }
                        regex = { sections = [{ value = "^e", color = "red", map_to = "E" }] }
                      }`,
		},
	} {
		t.Run(tc.scenario, func(t *testing.T) {
			resource.ParallelTest(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config:      testAccCoralogixResourceDashboardDynamicStatCardEmptyListConfig(name, tc.mapping),
						ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
					},
				},
			})
		})
	}
}

func TestAccCoralogixResourceDashboardDynamicStatCardRejectsInvalidVisualElements(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())

	for _, tc := range []struct {
		scenario  string
		statCard  string
		expectErr string
	}{
		{
			scenario: "field and mapping on the same element",
			statCard: `title = {
                        mapped_values     = true
                        observation_field = { keypath = ["applicationname"], scope = "label" }
                      }
                      color_label_mapping = {
                        value = { sections = [{ value = "ok", color = "green", map_to = "OK" }] }
                      }`,
			expectErr: `Invalid Attribute Combination`,
		},
		{
			scenario: "field and mapping on the same template variable",
			statCard: `label = {
                        template_text = "l"
                        template_variables = [{
                          mapped_values     = true
                          observation_field = { keypath = ["applicationname"], scope = "label" }
                        }]
                      }
                      color_label_mapping = {
                        value = { sections = [{ value = "ok", color = "green", map_to = "OK" }] }
                      }`,
			expectErr: `Invalid Attribute Combination`,
		},
		{
			scenario: "mapping result as the primary value",
			statCard: `primary_value = {
                        mapped_values = true
                      }
                      color_label_mapping = {
                        value = { sections = [{ value = "ok", color = "green", map_to = "OK" }] }
                      }`,
			expectErr: `primary_value cannot use mapped_values`,
		},
	} {
		t.Run(tc.scenario, func(t *testing.T) {
			resource.ParallelTest(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config:      testAccCoralogixResourceDashboardDynamicStatCardEmptyListConfig(name, tc.statCard),
						ExpectError: regexp.MustCompile(tc.expectErr),
					},
				},
			})
		})
	}
}

func testAccCoralogixResourceDashboardDynamicStatCardEmptyListConfig(name, statCardBody string) string {
	return fmt.Sprintf(`resource "coralogix_dashboard" "test" {
  name = %q
  layout = {
    sections = [{
      rows = [{
        height = 19
        widgets = [{
          title = "empty list"
          definition = {
            dynamic = {
              query_definitions = [{
                query = {
                  logs = {
                    lucene_query = "*"
                  }
                }
              }]
              visualization = {
                stat_card = {
                  %s
                }
              }
            }
          }
        }]
      }]
    }]
  }
}
`, name, statCardBody)
}

func testAccCoralogixResourceDashboardDynamicStatCardConfig(name, observationTitle string, setOptionalEnums, autoMinMax bool) string {
	legendBy, colorBy, thresholdType := "", "", ""
	minMax := `min_max = {
                            custom = {
                              min = 0
                              max = 500
                            }
                          }`
	if autoMinMax {
		minMax = `min_max = {
                            auto = true
                          }`
	}
	if setOptionalEnums {
		legendBy = `
                      legend_by = "groups"`
		colorBy = `
                        color_by = "value"`
		thresholdType = `
                          threshold_type = "absolute"`
	}

	return fmt.Sprintf(`resource "coralogix_dashboard" "test" {
  name        = %q
  description = "dynamic stat card widget acceptance coverage"
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
                    name = "latency"
                    query = {
                      logs = {
                        lucene_query = "*"
                        group_by = [{
                          keypath = ["applicationname"]
                          scope   = "label"
                        }]
                      }
                    }
                  }]
                  visualization = {
                    stat_card = {
                      allow_abbreviation = true
                      custom_unit        = "requests"
                      decimal_precision  = 3
                      unit               = "custom"%s
                      category_fields = [{
                        keypath = ["applicationname"]
                        scope   = "label"
                      }]
                      value_fields = [{
                        keypath = ["meta.responseTime.numeric"]
                        scope   = "user_data"
                      }]
                      legend = {
                        is_visible = true
                        placement  = "bottom"
                      }
                      title = {
                        template_text = "p99 {{field}}"
                        observation_field = {
                          keypath = ["subsystemname"]
                          scope   = "label"
                        }
                        template_variables = [{
                          observation_field = {
                            keypath = ["applicationname"]
                            scope   = "label"
                          }
                        }]
                      }
                      label = {
                        observation_field = {
                          keypath = ["severity"]
                          scope   = "metadata"
                        }
                      }
                      primary_value = {
                        observation_field = {
                          keypath = ["meta.responseTime.numeric"]
                          scope   = "user_data"
                        }
                      }
                      color_label_mapping = {%s
                        range = {%s
                          %s
                          thresholds = [{
                            from  = 0
                            color = "green"
                            label = "ok"
                          }]
                        }
                      }
                    }
                  }
                }
              }
            },
            {
              title = "stat card mapped values"
              definition = {
                dynamic = {
                  query_definitions = [{
                    query = {
                      logs = {
                        lucene_query = "*"
                      }
                    }
                  }]
                  visualization = {
                    stat_card = {
                      title = {
                        mapped_values = true
                      }
                      primary_value = {
                        observation_field = {
                          keypath = ["meta.responseTime.numeric"]
                          scope   = "user_data"
                        }
                      }
                      label = {
                        template_text = "l"
                        template_variables = [{
                          mapped_values = true
                        }]
                      }
                      color_label_mapping = {
                        value = {
                          sections = [{
                            value  = "ok"
                            color  = "green"
                            map_to = "Healthy"
                          }]
                        }
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
              title = "stat card regex"
              definition = {
                dynamic = {
                  query_definitions = [{
                    query = {
                      logs = {
                        lucene_query = "*"
                      }
                    }
                  }]
                  visualization = {
                    stat_card = {
                      # custom_unit is documented as taking effect only with
                      # unit = "custom"; the API stores it either way, so no
                      # validator couples them.
                      unit        = "bytes"
                      custom_unit = "widgets"
                      title = {
                        template_text = "t"
                      }
                      label = {
                        template_text = "l"
                      }
                      primary_value = {
                        template_text = "v"
                      }
                      color_label_mapping = {
                        regex = {
                          sections = [{
                            value  = "^err.*"
                            color  = "red"
                            map_to = "Errors"
                          }]
                        }
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
`, name, observationTitle, legendBy, colorBy, thresholdType, minMax)
}
