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

const dashboardOpenAPIDynamicTableTestName = "TestAccCoralogixResourceDashboardDynamicTableWidget"

// PropertyDefinition is a union of eight arms and RuleScope of three. The
// coverage manifest classifies all eleven as covered by this test, so each one
// gets its own rule here — a test exercising a subset would let the manifest
// claim support nothing verifies.
func TestAccCoralogixResourceDashboardDynamicTableWidget(t *testing.T) {
	ctx := context.Background()
	var client *dashboardservice.DashboardServiceAPIService
	fixture := t.Name()
	name := dashboardOpenAPIFixtureName(fixture)

	tbl := "layout.sections.0.rows.0.widgets.0.definition.dynamic.visualization.table."

	backendCheck := func(state *terraform.State) error {
		dashboard, err := dashboardOpenAPIFetchDashboard(ctx, client, state, dashboardResourceName, fixture)
		if err != nil {
			return err
		}
		return dashboardOpenAPIAssertDynamicTableWidget(dashboard, fixture)
	}

	steps := dashboardOpenAPIStructuredLifecycleSteps(
		dashboardOpenAPILifecyclePhase{
			Config: testAccCoralogixResourceDashboardDynamicTableConfig(name, "dynamic table", true),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
				resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.title", "dynamic table"),

				resource.TestCheckResourceAttr(dashboardResourceName, tbl+"columns.0.field.keypath.0", "applicationname"),
				resource.TestCheckResourceAttr(dashboardResourceName, tbl+"columns.0.field.scope", "label"),
				resource.TestCheckResourceAttr(dashboardResourceName, tbl+"settings.row_style", "one_line"),
				resource.TestCheckResourceAttr(dashboardResourceName, tbl+"settings.column_widths.0.column_name", "applicationname"),
				resource.TestCheckResourceAttr(dashboardResourceName, tbl+"settings.column_widths.0.width", "120"),

				// one rule per PropertyDefinition arm, in declaration order
				resource.TestCheckResourceAttr(dashboardResourceName, tbl+"rules.0.name", "thresholds arm"),
				resource.TestCheckResourceAttr(dashboardResourceName, tbl+"rules.0.properties.0.definition.thresholds.type", "absolute"),
				resource.TestCheckResourceAttr(dashboardResourceName, tbl+"rules.0.properties.0.definition.thresholds.min", "0"),
				resource.TestCheckResourceAttr(dashboardResourceName, tbl+"rules.0.properties.0.definition.thresholds.max", "100"),
				resource.TestCheckResourceAttr(dashboardResourceName, tbl+"rules.0.properties.0.definition.thresholds.values.0.color", "green"),

				resource.TestCheckResourceAttr(dashboardResourceName, tbl+"rules.1.properties.0.definition.alignment", "center"),
				resource.TestCheckResourceAttr(dashboardResourceName, tbl+"rules.2.properties.0.definition.units.unit", "bytes"),
				resource.TestCheckResourceAttr(dashboardResourceName, tbl+"rules.2.properties.0.definition.units.decimal_precision", "2"),
				resource.TestCheckResourceAttr(dashboardResourceName, tbl+"rules.2.properties.0.definition.units.allow_abbreviation", "true"),
				resource.TestCheckResourceAttr(dashboardResourceName, tbl+"rules.3.properties.0.definition.regex_extract", "^err-(.*)$"),
				resource.TestCheckResourceAttr(dashboardResourceName, tbl+"rules.4.properties.0.definition.link.actions.0.name", "open runbook"),
				resource.TestCheckResourceAttr(dashboardResourceName, tbl+"rules.4.properties.0.definition.link.actions.0.url", "https://example.invalid/runbook"),
				resource.TestCheckResourceAttr(dashboardResourceName, tbl+"rules.4.properties.0.definition.link.actions.0.should_open_in_new_window", "true"),
				resource.TestCheckResourceAttr(dashboardResourceName, tbl+"rules.5.properties.0.definition.values_alias", "alias"),
				resource.TestCheckResourceAttr(dashboardResourceName, tbl+"rules.6.properties.0.definition.values_mapping.mappings.0.input_value", "5"),
				resource.TestCheckResourceAttr(dashboardResourceName, tbl+"rules.6.properties.0.definition.values_mapping.mappings.0.replace_value", "error"),
				resource.TestCheckResourceAttr(dashboardResourceName, tbl+"rules.6.properties.0.definition.values_mapping.mappings.0.type", "value"),
				resource.TestCheckResourceAttr(dashboardResourceName, tbl+"rules.7.properties.0.definition.column_display_name", "Application"),

				// the three RuleScope branches
				resource.TestCheckResourceAttr(dashboardResourceName, tbl+"rules.0.rule_scope.field.keypath.0", "applicationname"),
				resource.TestCheckResourceAttr(dashboardResourceName, tbl+"rules.1.rule_scope.regex", "^app-.*$"),
				resource.TestCheckResourceAttr(dashboardResourceName, tbl+"rules.2.rule_scope.field_type", "string"),

				backendCheck,
			),
		},
		[]dashboardOpenAPILifecyclePhase{
			{
				Config: testAccCoralogixResourceDashboardDynamicTableConfig(name, "dynamic table updated", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.title", "dynamic table updated"),
					backendCheck,
				),
			},
			{
				// Removing the optional enums must reset them rather than keep the old value.
				Config: testAccCoralogixResourceDashboardDynamicTableConfig(name, "dynamic table updated", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, tbl+"settings.row_style", "unspecified"),
					resource.TestCheckResourceAttr(dashboardResourceName, tbl+"rules.0.properties.0.definition.thresholds.type", "unspecified"),
					backendCheck,
				),
			},
		},
		resource.TestStep{
			ResourceName:      dashboardResourceName,
			ImportState:       true,
			ImportStateVerify: true,
			ImportStateCheck: dashboardOpenAPIImportDashboardCheck(ctx, &client, fixture, func(dashboard *dashboardservice.Dashboard) error {
				return dashboardOpenAPIAssertDynamicTableWidget(dashboard, fixture)
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

// Asserts against what the backend stored, not only Terraform state: each rule's
// property definition must come back on the union arm it was sent on, and each
// rule scope on its own branch. A symmetric mapping error would satisfy state
// assertions while sending the wrong arm.
func dashboardOpenAPIAssertDynamicTableWidget(dashboard *dashboardservice.Dashboard, fixture string) error {
	sections := dashboard.Layout.Sections
	if len(sections) != 1 {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): sections = %d, want 1", fixture, dashboard.GetId(), len(sections))
	}
	widgets := sections[0].GetRows()[0].GetWidgets()
	if len(widgets) == 0 || widgets[0].Definition == nil {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): first row has no widget definition", fixture, dashboard.GetId())
	}
	definition := widgets[0].Definition
	if err := dashboardOpenAPIAssertOneOfBranch(definition, "WidgetDefinition", "dynamic", dashboard.GetId(), fixture); err != nil {
		return err
	}
	visualization := definition.Dynamic.Visualization
	if visualization == nil {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): dynamic visualization is nil", fixture, dashboard.GetId())
	}
	if err := dashboardOpenAPIAssertOneOfBranch(visualization, "Visualization", "table", dashboard.GetId(), fixture); err != nil {
		return err
	}

	table := visualization.Table
	wantDefinition := []string{
		"thresholds", "alignment", "units", "regexExtract",
		"link", "valuesAlias", "valuesMapping", "columnDisplayName",
	}
	wantScope := []string{"field", "regex", "fieldType"}

	rules := table.GetRules()
	if len(rules) != len(wantDefinition) {
		return fmt.Errorf("dashboard fixture %q (dashboard %q): table rules = %d, want %d (one per property-definition arm)",
			fixture, dashboard.GetId(), len(rules), len(wantDefinition))
	}
	for i := range rules {
		properties := rules[i].GetProperties()
		if len(properties) != 1 || properties[0].Definition == nil {
			return fmt.Errorf("dashboard fixture %q (dashboard %q): rule %d does not carry exactly one property definition", fixture, dashboard.GetId(), i)
		}
		if err := dashboardOpenAPIAssertOneOfBranch(properties[0].Definition, "PropertyDefinition", wantDefinition[i], dashboard.GetId(), fixture); err != nil {
			return err
		}
		if i < len(wantScope) {
			if rules[i].RuleScope == nil {
				return fmt.Errorf("dashboard fixture %q (dashboard %q): rule %d has no rule scope", fixture, dashboard.GetId(), i)
			}
			if err := dashboardOpenAPIAssertOneOfBranch(rules[i].RuleScope, "RuleScope", wantScope[i], dashboard.GetId(), fixture); err != nil {
				return err
			}
		}
	}
	return nil
}

// Every list this visualization exposes must reject an explicit empty list at
// plan time; otherwise it passes the plan and fails the apply, because the API
// cannot store an empty list.
func TestAccCoralogixResourceDashboardDynamicTableRejectsEmptyLists(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())

	for attribute, body := range map[string]string{
		"columns":       `table = { columns = [] }`,
		"rules":         `table = { rules = [] }`,
		"column_widths": `table = { settings = { column_widths = [] } }`,
		"properties": `table = { rules = [{
          name       = "r"
          properties = []
          rule_scope = { regex = "^a$" }
        }] }`,
		"link_actions": `table = { rules = [{
          name       = "r"
          rule_scope = { regex = "^a$" }
          properties = [{ definition = { link = { actions = [] } } }]
        }] }`,
		"values_mapping_mappings": `table = { rules = [{
          name       = "r"
          rule_scope = { regex = "^a$" }
          properties = [{ definition = { values_mapping = { mappings = [] } } }]
        }] }`,
		"threshold_values": `table = { rules = [{
          name       = "r"
          rule_scope = { regex = "^a$" }
          properties = [{ definition = { thresholds = { values = [] } } }]
        }] }`,
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
`, name, body),
					ExpectError: regexp.MustCompile(`(?s)list must contain at least 1 element`),
				}},
			})
		})
	}
}

// PropertyDefinition is a union: setting two arms must fail at plan time rather
// than reaching the API, which rejects it with a generated-field error naming no
// HCL path. Same for RuleScope.
func TestAccCoralogixResourceDashboardDynamicTableRejectsMultipleUnionArms(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())

	for scenario, body := range map[string]string{
		"two property definition arms": `table = { rules = [{
          name       = "r"
          rule_scope = { regex = "^a$" }
          properties = [{ definition = {
            alignment           = "center"
            column_display_name = "Application"
          } }]
        }] }`,
		"no property definition arm": `table = { rules = [{
          name       = "r"
          rule_scope = { regex = "^a$" }
          properties = [{ definition = {} }]
        }] }`,
		"two rule scope arms": `table = { rules = [{
          name       = "r"
          rule_scope = { regex = "^a$", field_type = "string" }
          properties = [{ definition = { alignment = "center" } }]
        }] }`,
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
      title = "union"
      definition = { dynamic = {
        query_definitions = [{ query = { logs = { lucene_query = "*" } } }]
        visualization     = { %s }
      }}
    }]
  }] }] }
}
`, name, body),
					ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
				}},
			})
		})
	}
}

func testAccCoralogixResourceDashboardDynamicTableConfig(name, title string, setOptionalEnums bool) string {
	rowStyle, thresholdType := "", ""
	if setOptionalEnums {
		rowStyle = `
                        row_style = "one_line"`
		thresholdType = `
                            type = "absolute"`
	}

	return fmt.Sprintf(`resource "coralogix_dashboard" "test" {
  name        = %q
  description = "dynamic table widget acceptance coverage"
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
          title = %q
          definition = {
            dynamic = {
              query_definitions = [{
                name  = "rows"
                query = { logs = { lucene_query = "*" } }
              }]
              visualization = {
                table = {
                  columns = [{
                    field = {
                      keypath = ["applicationname"]
                      scope   = "label"
                    }
                  }]
                  settings = {%s
                    column_widths = [{
                      column_name = "applicationname"
                      width       = 120
                    }]
                  }
                  rules = [
                    {
                      name       = "thresholds arm"
                      rule_scope = { field = { keypath = ["applicationname"], scope = "label" } }
                      properties = [{
                        definition = {
                          thresholds = {%s
                            min = 0
                            max = 100
                            values = [{
                              from  = 0
                              color = "green"
                              label = "ok"
                            }]
                          }
                        }
                      }]
                    },
                    {
                      name       = "alignment arm"
                      rule_scope = { regex = "^app-.*$" }
                      properties = [{ definition = { alignment = "center" } }]
                    },
                    {
                      name       = "units arm"
                      rule_scope = { field_type = "string" }
                      properties = [{
                        definition = {
                          units = {
                            unit               = "bytes"
                            decimal_precision  = 2
                            allow_abbreviation = true
                          }
                        }
                      }]
                    },
                    {
                      name       = "regex extract arm"
                      rule_scope = { regex = "^err-.*$" }
                      properties = [{ definition = { regex_extract = "^err-(.*)$" } }]
                    },
                    {
                      name       = "link arm"
                      rule_scope = { regex = "^link-.*$" }
                      properties = [{
                        definition = {
                          link = {
                            actions = [{
                              name                      = "open runbook"
                              url                       = "https://example.invalid/runbook"
                              should_open_in_new_window = true
                            }]
                          }
                        }
                      }]
                    },
                    {
                      name       = "values alias arm"
                      rule_scope = { regex = "^alias-.*$" }
                      properties = [{ definition = { values_alias = "alias" } }]
                    },
                    {
                      name       = "values mapping arm"
                      rule_scope = { regex = "^map-.*$" }
                      properties = [{
                        definition = {
                          values_mapping = {
                            mappings = [{
                              input_value   = "5"
                              replace_value = "error"
                              type          = "value"
                            }]
                          }
                        }
                      }]
                    },
                    {
                      name       = "column display name arm"
                      rule_scope = { regex = "^name-.*$" }
                      properties = [{ definition = { column_display_name = "Application" } }]
                    },
                  ]
                }
              }
            }
          }
        }]
      }]
    }]
  }
}
`, name, title, rowStyle, thresholdType)
}

// The API stores table columns whose field has no keypath, and this repo
// applies such a fixture through content_json. Typed HCL must express them too.
func TestAccCoralogixResourceDashboardDynamicTableColumnWithoutKeypath(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{{
			Config: fmt.Sprintf(`resource "coralogix_dashboard" "test" {
  name = %q
  layout = { sections = [{ rows = [{
    height = 19
    widgets = [{
      title = "no keypath"
      definition = { dynamic = {
        query_definitions = [{ query = { logs = { lucene_query = "*" } } }]
        visualization = { table = {
          columns = [{ field = { scope = "user_data" } }]
        } }
      }}
    }]
  }] }] }
}
`, name),
			Check: resource.TestCheckNoResourceAttr(dashboardResourceName,
				"layout.sections.0.rows.0.widgets.0.definition.dynamic.visualization.table.columns.0.field.keypath"),
		}},
	})
}
