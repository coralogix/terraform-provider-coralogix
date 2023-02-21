package coralogix

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/types/known/wrapperspb"
	"terraform-provider-coralogix/coralogix/clientset"
	dashboard "terraform-provider-coralogix/coralogix/clientset/grpc/coralogix-dashboards/v1"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

var dashboardResourceName = "coralogix_dashboard.test"

func TestAccCoralogixResourceDashboard(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckDashboardDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceDashboard(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
					resource.TestCheckResourceAttr(dashboardResourceName, "name", "dont drop me!"),
					resource.TestCheckResourceAttr(dashboardResourceName, "description", "dashboards team is messing with this 🗿"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.0.appearance.0.height", "19"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.0.widgets.0.title", "status 4XX"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.0.widgets.0.definition.0.line_chart.0.query.0.metrics.0.promql_query", "http_requests_total{status!~\"4..\"}"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.0.widgets.0.definition.0.line_chart.0.legend.0.is_visible", "true"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.0.widgets.0.definition.0.line_chart.0.legend.0.columns.0", "Max"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.0.widgets.0.definition.0.line_chart.0.legend.0.columns.1", "Last"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.0.widgets.0.appearance.0.width", "0"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.0.widgets.1.title", "count"),
					resource.TestCheckResourceAttrSet(dashboardResourceName, "layout.0.sections.0.rows.0.widgets.1.definition.0.line_chart.0.query.0.logs.0.aggregations.0.count.0.%"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.0.widgets.1.definition.0.line_chart.0.legend.0.is_visible", "true"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.0.widgets.1.definition.0.line_chart.0.legend.0.columns.0", "Min"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.0.widgets.1.definition.0.line_chart.0.legend.0.columns.1", "Max"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.0.widgets.1.definition.0.line_chart.0.legend.0.columns.2", "Sum"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.0.widgets.1.definition.0.line_chart.0.legend.0.columns.3", "Avg"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.0.widgets.1.definition.0.line_chart.0.legend.0.columns.4", "Last"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.0.widgets.1.appearance.0.width", "0"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.0.widgets.2.title", "error throwing pods"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.0.widgets.2.definition.0.line_chart.0.query.0.logs.0.lucene_query", "coralogix.metadata.severity=5 OR coralogix.metadata.severity=\"6\" OR coralogix.metadata.severity=\"4\""),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.0.widgets.2.definition.0.line_chart.0.query.0.logs.0.group_by.0", "coralogix.metadata.subsystemName"),
					resource.TestCheckResourceAttrSet(dashboardResourceName, "layout.0.sections.0.rows.0.widgets.2.definition.0.line_chart.0.query.0.logs.0.aggregations.0.count.0.%"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.0.widgets.2.definition.0.line_chart.0.legend.0.is_visible", "true"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.0.widgets.2.definition.0.line_chart.0.legend.0.columns.0", "Max"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.0.widgets.2.definition.0.line_chart.0.legend.0.columns.1", "Last"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.0.widgets.2.appearance.0.width", "0"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.1.appearance.0.height", "28"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.1.widgets.0.title", "dashboards-api logz"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.1.widgets.0.description", "warnings, errors, criticals"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.1.widgets.0.definition.0.data_table.0.query.0.logs.0.filters.0.field", "coralogix.metadata.applicationName"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.1.widgets.0.definition.0.data_table.0.query.0.logs.0.filters.0.operator.0.equals.0.selection.0.list.0", "staging"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.1.widgets.0.definition.0.data_table.0.results_per_page", "20"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.1.widgets.0.definition.0.data_table.0.row_style", "One_Line"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.1.widgets.0.definition.0.data_table.0.columns.0.field", "coralogix.timestamp"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.1.widgets.0.definition.0.data_table.0.columns.1.field", "textObject.textObject.textObject.kubernetes.pod_id"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.1.widgets.0.definition.0.data_table.0.columns.2.field", "coralogix.text"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.1.widgets.0.definition.0.data_table.0.columns.3.field", "coralogix.metadata.applicationName"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.1.widgets.0.definition.0.data_table.0.columns.4.field", "coralogix.metadata.subsystemName"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.1.widgets.0.definition.0.data_table.0.columns.5.field", "coralogix.metadata.sdkId"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.1.widgets.0.definition.0.data_table.0.columns.6.field", "textObject.log_obj.e2e_test.config"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.0.sections.0.rows.1.widgets.0.appearance.0.width", "0"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables.0.name", "test_variable"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables.0.definition.0.multi_select.0.selection.0.list.0", "1"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables.0.definition.0.multi_select.0.selection.0.list.1", "2"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables.0.definition.0.multi_select.0.selection.0.list.2", "3"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables.0.definition.0.multi_select.0.source.0.constant_list.0", "1"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables.0.definition.0.multi_select.0.source.0.constant_list.1", "2"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables.0.definition.0.multi_select.0.source.0.constant_list.2", "3"),
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

func TestAccCoralogixResourceDashboardFromJson(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	parent := filepath.Dir(wd)
	filePath := parent + "/examples/dashboard/dashboard.json"
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckDashboardDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceDashboardFromJson(filePath),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
				),
			},
		},
	})
}

func testAccCheckDashboardDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).Dashboards()

	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_dashboard" {
			continue
		}

		dashboardId := wrapperspb.String(expandUUID(rs.Primary.ID))
		resp, err := client.GetDashboard(ctx, &dashboard.GetDashboardRequest{DashboardId: dashboardId})
		if err == nil {
			if resp.GetDashboard().GetId().GetValue() == rs.Primary.ID {
				return fmt.Errorf("dashboard still exists: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}

func testAccCoralogixResourceDashboard() string {
	return `resource "coralogix_dashboard" test {
  	name        = "dont drop me!"
  	description = "dashboards team is messing with this 🗿"
    layout {
    sections {
      rows {
        appearance {
          height = 19
        }
        widgets {
          title = "status 4XX"
          definition {
            line_chart {
              query {
                metrics {
                  promql_query = "http_requests_total{status!~\"4..\"}"
                }
              }
              legend {
                is_visible = true
                columns    = ["Max", "Last"]
              }
            }
          }
          appearance {
            width = 0
          }
        }
        widgets {
          title = "count"
          definition {
            line_chart {
              query {
                logs {
                  aggregations {
                    count {
                    }
                  }
                }
              }
              legend {
                is_visible = true
                columns    = ["Min", "Max", "Sum", "Avg", "Last"]
              }
            }
          }
          appearance {
            width = 0
          }
        }
        widgets {
          title = "error throwing pods"
          definition {
            line_chart {
              query {
                logs {
                  lucene_query = "coralogix.metadata.severity=5 OR coralogix.metadata.severity=\"6\" OR coralogix.metadata.severity=\"4\""
                  group_by     = ["coralogix.metadata.subsystemName"]
                  aggregations {
                    count {
                    }
                  }
                }
              }
              legend {
                is_visible = true
                columns    = ["Max", "Last"]
              }
            }
          }
          appearance {
            width = 0
          }
        }
      }
      rows {
        appearance {
          height = 28
        }
        widgets {
          title       = "dashboards-api logz"
          description = "warnings, errors, criticals"
          definition {
            data_table {
              query {
                logs {
                  filters {
                    field = "coralogix.metadata.applicationName"
                    operator {
                      equals {
                        selection {
                          list = ["staging"]
                        }
                      }
                    }
                  }
                }
              }
              results_per_page = 20
              row_style        = "One_Line"
              columns {
                field = "coralogix.timestamp"
              }
              columns {
                field = "textObject.textObject.textObject.kubernetes.pod_id"
              }
              columns {
                field = "coralogix.text"
              }
              columns {
                field = "coralogix.metadata.applicationName"
              }
              columns {
                field = "coralogix.metadata.subsystemName"
              }
              columns {
                field = "coralogix.metadata.sdkId"
              }
              columns {
                field = "textObject.log_obj.e2e_test.config"
              }
            }
          }
          appearance {
            width = 0
          }
        }
      }
    }
  }
  variables {
    name = "test_variable"
    definition {
      multi_select {
        selection {
          list = ["1", "2", "3"]
        }
        source {
          constant_list = ["1", "2", "3"]
        }
      }
    }
  }
}
`
}

func testAccCoralogixResourceDashboardFromJson(jsonFilePath string) string {
	return fmt.Sprintf(`resource "coralogix_dashboard" test {
   		content_json = file("%s")
	}
`, jsonFilePath)
}
