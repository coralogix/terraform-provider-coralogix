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

const dashboardOpenAPIDynamicStatTestName = "TestAccCoralogixResourceDashboardDynamicStatWidget"

func TestAccCoralogixResourceDashboardDynamicStatWidget(t *testing.T) {
	ctx := context.Background()
	var client *dashboardservice.DashboardServiceAPIService
	fixture := t.Name()
	name := dashboardOpenAPIFixtureName(fixture)
	widgetPrefix := "layout.sections.0.rows.0.widgets.0.definition.dynamic."
	statPrefix := widgetPrefix + "visualization.stat."
	spansPrefix := "layout.sections.0.rows.0.widgets.1.definition.dynamic.query_definitions.0.query.spans."
	metricsExplicitPrefix := "layout.sections.0.rows.1.widgets.0.definition.dynamic.query_definitions.0.query.metrics."
	metricsDefaultPrefix := "layout.sections.0.rows.1.widgets.1.definition.dynamic.query_definitions.0.query.metrics."
	dataPrimePrefix := "layout.sections.0.rows.2.widgets.0.definition.dynamic.query_definitions.0.query.data_prime."

	thresholds := `{ from = 0, color = "green", label = "ok" }`
	thresholdsUpdated := `{ from = 0, color = "green", label = "ok" }, { from = 1000, color = "red", label = "high" }`

	backendCheck := func(state *terraform.State) error {
		dashboard, err := dashboardOpenAPIFetchDashboard(ctx, client, state, dashboardResourceName, fixture)
		if err != nil {
			return err
		}
		return dashboardOpenAPIAssertDynamicStatWidgets(dashboard, fixture)
	}

	steps := dashboardOpenAPIStructuredLifecycleSteps(
		dashboardOpenAPILifecyclePhase{
			Config: testAccCoralogixResourceDashboardDynamicStatConfig(name, "dynamic stat logs", thresholds, true),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
				resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.title", "dynamic stat logs"),
				resource.TestCheckResourceAttrSet(dashboardResourceName, widgetPrefix+"query_definitions.0.id"),
				resource.TestCheckResourceAttr(dashboardResourceName, widgetPrefix+"query_definitions.0.name", "errors"),
				resource.TestCheckResourceAttr(dashboardResourceName, widgetPrefix+"query_definitions.0.query.logs.group_by.0.keypath.0", "subsystemname"),
				resource.TestCheckResourceAttr(dashboardResourceName, widgetPrefix+"query_definitions.0.query.logs.group_by.0.scope", "label"),
				resource.TestCheckResourceAttr(dashboardResourceName, widgetPrefix+"query_definitions.0.query.logs.aggregations.0.type", "count"),
				resource.TestCheckResourceAttr(dashboardResourceName, widgetPrefix+"query_definitions.0.query.logs.filters.0.field", "applicationname"),
				resource.TestCheckResourceAttr(dashboardResourceName, widgetPrefix+"query_definitions.0.query.logs.data_mode_type", "unspecified"),
				resource.TestCheckResourceAttr(dashboardResourceName, widgetPrefix+"interpretation", "single_value_kpi_stat"),
				resource.TestCheckResourceAttr(dashboardResourceName, widgetPrefix+"time_frame.relative.duration", "seconds:900"),

				resource.TestCheckResourceAttr(dashboardResourceName, statPrefix+"allow_abbreviation", "true"),
				resource.TestCheckResourceAttr(dashboardResourceName, statPrefix+"category_fields.0.keypath.0", "applicationname"),
				resource.TestCheckResourceAttr(dashboardResourceName, statPrefix+"category_fields.0.scope", "label"),
				resource.TestCheckResourceAttr(dashboardResourceName, statPrefix+"custom_unit", "widgets"),
				resource.TestCheckResourceAttr(dashboardResourceName, statPrefix+"decimal_precision", "2"),
				resource.TestCheckResourceAttr(dashboardResourceName, statPrefix+"display_series_name", "true"),
				resource.TestCheckResourceAttr(dashboardResourceName, statPrefix+"legend.is_visible", "true"),
				resource.TestCheckResourceAttr(dashboardResourceName, statPrefix+"legend.columns.0", "min"),
				resource.TestCheckResourceAttr(dashboardResourceName, statPrefix+"legend.group_by_query", "true"),
				resource.TestCheckResourceAttr(dashboardResourceName, statPrefix+"legend.placement", "bottom"),
				resource.TestCheckResourceAttr(dashboardResourceName, statPrefix+"legend_by", "thresholds"),
				resource.TestCheckResourceAttr(dashboardResourceName, statPrefix+"max", "100"),
				resource.TestCheckResourceAttr(dashboardResourceName, statPrefix+"min", "0"),
				resource.TestCheckResourceAttr(dashboardResourceName, statPrefix+"threshold_by", "background"),
				resource.TestCheckResourceAttr(dashboardResourceName, statPrefix+"threshold_type", "absolute"),
				resource.TestCheckResourceAttr(dashboardResourceName, statPrefix+"thresholds.0.from", "0"),
				resource.TestCheckResourceAttr(dashboardResourceName, statPrefix+"thresholds.0.color", "green"),
				resource.TestCheckResourceAttr(dashboardResourceName, statPrefix+"thresholds.0.label", "ok"),
				resource.TestCheckResourceAttr(dashboardResourceName, statPrefix+"unit", "custom"),
				resource.TestCheckResourceAttr(dashboardResourceName, statPrefix+"value_field.keypath.0", "meta.responseTime.numeric"),
				resource.TestCheckResourceAttr(dashboardResourceName, statPrefix+"value_field.scope", "user_data"),
				resource.TestCheckResourceAttr(dashboardResourceName, statPrefix+"value_fields.0.keypath.0", "meta.responseTime.numeric"),

				resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.title", "dynamic stat spans"),
				resource.TestCheckResourceAttr(dashboardResourceName, spansPrefix+"lucene_query", "*"),
				resource.TestCheckResourceAttr(dashboardResourceName, spansPrefix+"group_by.0.keypath.0", "service"),
				resource.TestCheckResourceAttr(dashboardResourceName, spansPrefix+"group_by.0.keypath.1", "name"),
				resource.TestCheckResourceAttr(dashboardResourceName, spansPrefix+"group_by.0.scope", "user_data"),
				resource.TestCheckResourceAttr(dashboardResourceName, spansPrefix+"group_by.0.relation_type", "unspecified"),
				resource.TestCheckResourceAttr(dashboardResourceName, spansPrefix+"aggregations.0.type", "count"),
				resource.TestCheckResourceAttr(dashboardResourceName, spansPrefix+"filters.0.field.type", "metadata"),
				resource.TestCheckResourceAttr(dashboardResourceName, spansPrefix+"filters.0.field.value", "application_name"),
				resource.TestCheckResourceAttr(dashboardResourceName, spansPrefix+"filters.0.operator.type", "equals"),
				resource.TestCheckResourceAttr(dashboardResourceName, spansPrefix+"filters.0.operator.selected_values.0", "api"),
				resource.TestCheckResourceAttr(dashboardResourceName, spansPrefix+"data_mode_type", "unspecified"),

				resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.0.title", "dynamic stat metrics explicit"),
				resource.TestCheckResourceAttr(dashboardResourceName, metricsExplicitPrefix+"promql_query", "vector(1)"),
				resource.TestCheckResourceAttr(dashboardResourceName, metricsExplicitPrefix+"promql_query_type", "instant"),
				resource.TestCheckResourceAttr(dashboardResourceName, metricsExplicitPrefix+"editor_mode", "text"),
				resource.TestCheckResourceAttr(dashboardResourceName, metricsExplicitPrefix+"series_limit_type", "by_series_count"),

				resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.1.title", "dynamic stat metrics defaults"),
				resource.TestCheckResourceAttr(dashboardResourceName, metricsDefaultPrefix+"promql_query", "vector(2)"),
				resource.TestCheckResourceAttr(dashboardResourceName, metricsDefaultPrefix+"promql_query_type", "unspecified"),
				resource.TestCheckResourceAttr(dashboardResourceName, metricsDefaultPrefix+"editor_mode", "unspecified"),
				resource.TestCheckResourceAttr(dashboardResourceName, metricsDefaultPrefix+"series_limit_type", "unspecified"),

				resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.2.widgets.0.title", "dynamic stat data prime"),
				resource.TestCheckResourceAttr(dashboardResourceName, dataPrimePrefix+"query", "source logs | limit 10"),
				resource.TestCheckResourceAttr(dashboardResourceName, dataPrimePrefix+"data_mode_type", "unspecified"),

				backendCheck,
			),
		},
		[]dashboardOpenAPILifecyclePhase{
			{
				Config: testAccCoralogixResourceDashboardDynamicStatConfig(name, "dynamic stat logs updated", thresholdsUpdated, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.title", "dynamic stat logs updated"),
					resource.TestCheckResourceAttr(dashboardResourceName, statPrefix+"thresholds.1.from", "1000"),
					resource.TestCheckResourceAttr(dashboardResourceName, statPrefix+"thresholds.1.color", "red"),
					resource.TestCheckResourceAttr(dashboardResourceName, statPrefix+"thresholds.1.label", "high"),
					backendCheck,
				),
			},
			{
				Config: testAccCoralogixResourceDashboardDynamicStatConfig(name, "dynamic stat logs updated", thresholdsUpdated, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, widgetPrefix+"interpretation", "unspecified"),
					resource.TestCheckResourceAttr(dashboardResourceName, statPrefix+"threshold_type", "unspecified"),
					backendCheck,
				),
			},
		},
		resource.TestStep{
			ResourceName:      dashboardResourceName,
			ImportState:       true,
			ImportStateVerify: true,
			ImportStateCheck: dashboardOpenAPIImportDashboardCheck(ctx, &client, fixture, func(dashboard *dashboardservice.Dashboard) error {
				return dashboardOpenAPIAssertDynamicStatWidgets(dashboard, fixture)
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

func dashboardOpenAPIAssertDynamicStatWidgets(dashboard *dashboardservice.Dashboard, fixture string) error {
	expected := []struct {
		row         int
		widget      int
		queryBranch string
	}{
		{row: 0, widget: 0, queryBranch: "logs"},
		{row: 0, widget: 1, queryBranch: "spans"},
		{row: 1, widget: 0, queryBranch: "metrics"},
		{row: 1, widget: 1, queryBranch: "metrics"},
		{row: 2, widget: 0, queryBranch: "dataprime"},
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

		dynamic := definition.Dynamic
		if len(dynamic.GetQueryDefinitions()) != 1 {
			return fmt.Errorf("dashboard fixture %q (dashboard %q): row %d widget %d dynamic queryDefinitions = %d, want 1", fixture, dashboard.GetId(), want.row, want.widget, len(dynamic.GetQueryDefinitions()))
		}
		query := &dynamic.QueryDefinitions[0].Query
		if err := dashboardOpenAPIAssertOneOfBranch(query, "DynamicQuery", want.queryBranch, dashboard.GetId(), fixture); err != nil {
			return err
		}

		if dynamic.Visualization == nil {
			return fmt.Errorf("dashboard fixture %q (dashboard %q): row %d widget %d dynamic visualization is nil", fixture, dashboard.GetId(), want.row, want.widget)
		}
		if err := dashboardOpenAPIAssertOneOfBranch(dynamic.Visualization, "Visualization", "stat", dashboard.GetId(), fixture); err != nil {
			return err
		}
	}

	return nil
}

func testAccCoralogixResourceDashboardDynamicStatConfig(name, statTitle, statThresholds string, setOptionalEnums bool) string {
	interpretation := ""
	thresholdType := ""
	if setOptionalEnums {
		interpretation = `
                  interpretation = "single_value_kpi_stat"`
		thresholdType = `
                      threshold_type      = "absolute"`
	}

	return fmt.Sprintf(`resource "coralogix_dashboard" "test" {
  name        = %q
  description = "dynamic stat widget acceptance coverage"
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
                        lucene_query = "coralogix.metadata.severity=\"5\""
                        group_by = [{
                          keypath = ["subsystemname"]
                          scope   = "label"
                        }]
                        aggregations = [{
                          type = "count"
                        }]
                        filters = [{
                          field = "applicationname"
                          operator = {
                            type            = "equals"
                            selected_values = ["api"]
                          }
                        }]
                      }
                    }
                  }]%s
                  time_frame = {
                    relative = {
                      duration = "seconds:900"
                    }
                  }
                  visualization = {
                    stat = {
                      allow_abbreviation  = true
                      custom_unit         = "widgets"
                      decimal_precision   = 2
                      display_series_name = true
                      legend_by           = "thresholds"
                      max                 = 100
                      min                 = 0
                      threshold_by        = "background"
                      unit                = "custom"%s
                      thresholds          = [%s]
                      legend = {
                        is_visible     = true
                        columns        = ["min"]
                        group_by_query = true
                        placement      = "bottom"
                      }
                      category_fields = [{
                        keypath = ["applicationname"]
                        scope   = "label"
                      }]
                      value_field = {
                        keypath = ["meta.responseTime.numeric"]
                        scope   = "user_data"
                      }
                      value_fields = [{
                        keypath = ["meta.responseTime.numeric"]
                        scope   = "user_data"
                      }]
                    }
                  }
                }
              }
            },
            {
              title = "dynamic stat spans"
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
                          field = {
                            type  = "metadata"
                            value = "application_name"
                          }
                          operator = {
                            type            = "equals"
                            selected_values = ["api"]
                          }
                        }]
                      }
                    }
                  }]
                  visualization = {
                    stat = {
                      unit = "bytes"
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
              title = "dynamic stat metrics explicit"
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
                    stat = {
                      unit = "bytes"
                    }
                  }
                }
              }
            },
            {
              title = "dynamic stat metrics defaults"
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
                    stat = {
                      unit = "bytes"
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
            title = "dynamic stat data prime"
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
                  stat = {
                    unit = "bytes"
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
`, name, statTitle, interpretation, thresholdType, statThresholds)
}

// An explicitly empty list is not the same value as an absent one: expansion
// sends an empty slice, the API omits it, and the refreshed value comes back
// null, which fails the apply with an inconsistent-result error. The size
// validators turn that into a plan-time error naming the attribute.
func TestAccCoralogixResourceDashboardDynamicRejectsEmptyLists(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())

	t.Run("spans_filters", func(t *testing.T) {
		resource.Test(t, resource.TestCase{
			PreCheck:                 func() { testAccPreCheck(t) },
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{{
				Config: fmt.Sprintf(`resource "coralogix_dashboard" "test" {
  name = %q
  layout = { sections = [{ rows = [{
    height = 19
    widgets = [{
      title = "empty spans filters"
      definition = { dynamic = {
        query_definitions = [{ query = { spans = { lucene_query = "*", filters = [] } } }]
        visualization     = { stat = {} }
      }}
    }]
  }] }] }
}
`, name),
				ExpectError: regexp.MustCompile(`(?s)list must contain at least 1 element`),
			}},
		})
	})

	for attribute, visualization := range map[string]string{
		"thresholds":                `stat = { thresholds = [] }`,
		"category_fields":           `stat = { category_fields = [] }`,
		"value_fields":              `stat = { value_fields = [] }`,
		"observation_field_keypath": `stat = { category_fields = [{ keypath = [], scope = "label" }] }`,
	} {
		t.Run(attribute, func(t *testing.T) {
			resource.Test(t, resource.TestCase{
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

// decimal_precision is documented as 0-15 and the API does not enforce it: 16,
// -1, 400 and both int32 extremes are all accepted and read back verbatim
// against a live environment, and the API stores -1. So a value outside the
// documented range applies, exactly as it does for the classic widgets'
// decimal. What stays rejected is a value the API's int32 field cannot hold,
// because the conversion is an unchecked cast that would wrap it silently.
func TestAccCoralogixResourceDashboardDynamicDecimalPrecisionOutsideDocumentedRange(t *testing.T) {
	for _, precision := range []string{"16", "-1"} {
		t.Run(precision, func(t *testing.T) {
			name := dashboardOpenAPIFixtureName(t.Name())
			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{{
					Config: testAccCoralogixResourceDashboardDynamicDecimalPrecisionConfig(name, precision),
					Check: resource.TestCheckResourceAttr(dashboardResourceName,
						"layout.sections.0.rows.0.widgets.0.definition.dynamic.visualization.stat.decimal_precision", precision),
				}},
			})
		})
	}
}

func TestAccCoralogixResourceDashboardDynamicRejectsDecimalPrecisionBeyondInt32(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config:      testAccCoralogixResourceDashboardDynamicDecimalPrecisionConfig(name, "2147483648"),
			ExpectError: regexp.MustCompile(`(?s)must be between -2147483648 and 2147483647`),
		}},
	})
}

func testAccCoralogixResourceDashboardDynamicDecimalPrecisionConfig(name, precision string) string {
	return fmt.Sprintf(`resource "coralogix_dashboard" "test" {
  name = %q
  layout = { sections = [{ rows = [{
    height = 19
    widgets = [{
      title = "decimal precision"
      definition = { dynamic = {
        query_definitions = [{ query = { logs = { lucene_query = "*" } } }]
        visualization     = { stat = { decimal_precision = %s } }
      }}
    }]
  }] }] }
}
`, name, precision)
}
