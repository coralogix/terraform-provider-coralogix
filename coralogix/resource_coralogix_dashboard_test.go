package coralogix

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"terraform-provider-coralogix/coralogix/clientset"
	dashboard "terraform-provider-coralogix/coralogix/clientset/grpc/com/coralogix/coralogix-dashboards"
)

func TestAccCoralogixResourceDashboard(t *testing.T) {
	resourceName := "coralogix_dashboard.test"
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckDashboardDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceDashboard(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", "dont drop me!"),
					resource.TestCheckResourceAttr(resourceName, "description", "dashboards team is messing with this 🗿"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.0.appearance.0.height", "19"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.0.widgets.0.title", "status 4XX"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.0.widgets.0.definition.0.line_chart.0.query.0.metrics.0.promql_query", "http_requests_total{status!~\"4..\"}"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.0.widgets.0.definition.0.line_chart.0.legend.0.is_visible", "true"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.0.widgets.0.definition.0.line_chart.0.legend.0.columns.0", "Max"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.0.widgets.0.definition.0.line_chart.0.legend.0.columns.1", "Last"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.0.widgets.0.appearance.0.width", "0"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.0.widgets.1.title", "count"),
					resource.TestCheckResourceAttrSet(resourceName, "layout.0.sections.0.rows.0.widgets.1.definition.0.line_chart.0.query.0.logs.0.aggregations.0.count.0.%"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.0.widgets.1.definition.0.line_chart.0.legend.0.is_visible", "true"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.0.widgets.1.definition.0.line_chart.0.legend.0.columns.0", "Min"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.0.widgets.1.definition.0.line_chart.0.legend.0.columns.1", "Max"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.0.widgets.1.definition.0.line_chart.0.legend.0.columns.2", "Sum"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.0.widgets.1.definition.0.line_chart.0.legend.0.columns.3", "Avg"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.0.widgets.1.definition.0.line_chart.0.legend.0.columns.4", "Last"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.0.widgets.1.appearance.0.width", "0"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.0.widgets.2.title", "error throwing pods"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.0.widgets.2.definition.0.line_chart.0.query.0.logs.0.lucene_query", "coralogix.metadata.severity=\"5\" OR coralogix.metadata.severity=\"6\" OR coralogix.metadata.severity=\"4\""),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.0.widgets.2.definition.0.line_chart.0.query.0.logs.0.group_by.0", "coralogix.metadata.subsystemName"),
					resource.TestCheckResourceAttrSet(resourceName, "layout.0.sections.0.rows.0.widgets.2.definition.0.line_chart.0.query.0.logs.0.aggregations.0.count.0.%"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.0.widgets.2.definition.0.line_chart.0.legend.0.is_visible", "true"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.0.widgets.2.definition.0.line_chart.0.legend.0.columns.0", "Max"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.0.widgets.2.definition.0.line_chart.0.legend.0.columns.1", "Last"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.0.widgets.2.appearance.0.width", "0"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.1.appearance.0.height", "28"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.1.widgets.0.title", "dashboards-api logz"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.1.widgets.0.description", "warnings, errors, criticals"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.1.widgets.0.definition.0.data_table.0.query.0.logs.0.filters.0.name", "coralogix.metadata.severity"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.1.widgets.0.definition.0.data_table.0.query.0.logs.0.filters.0.values.0", "6"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.1.widgets.0.definition.0.data_table.0.query.0.logs.0.filters.0.values.1", "5"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.1.widgets.0.definition.0.data_table.0.query.0.logs.0.filters.0.values.2", "4"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.1.widgets.0.definition.0.data_table.0.query.0.logs.0.filters.1.name", "coralogix.metadata.subsystemName"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.1.widgets.0.definition.0.data_table.0.query.0.logs.0.filters.1.values.0", "coralogix-terraform-provider"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.1.widgets.0.definition.0.data_table.0.results_per_page", "20"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.1.widgets.0.definition.0.data_table.0.row_style", "One_Line"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.1.widgets.0.definition.0.data_table.0.columns.0.field", "coralogix.timestamp"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.1.widgets.0.definition.0.data_table.0.columns.1.field", "textObject.textObject.textObject.kubernetes.pod_id"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.1.widgets.0.definition.0.data_table.0.columns.2.field", "coralogix.text"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.1.widgets.0.definition.0.data_table.0.columns.3.field", "coralogix.metadata.applicationName"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.1.widgets.0.definition.0.data_table.0.columns.4.field", "coralogix.metadata.subsystemName"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.1.widgets.0.definition.0.data_table.0.columns.5.field", "coralogix.metadata.sdkId"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.1.widgets.0.definition.0.data_table.0.columns.6.field", "textObject.log_obj.e2e_test.config"),
					resource.TestCheckResourceAttr(resourceName, "layout.0.sections.0.rows.1.widgets.0.appearance.0.width", "0"),
					resource.TestCheckResourceAttr(resourceName, "variables.0.name", "test_variable"),
					resource.TestCheckResourceAttr(resourceName, "variables.0.definition.0.multi_select.0.selected.0", "1"),
					resource.TestCheckResourceAttr(resourceName, "variables.0.definition.0.multi_select.0.selected.1", "2"),
					resource.TestCheckResourceAttr(resourceName, "variables.0.definition.0.multi_select.0.selected.2", "3"),
					resource.TestCheckResourceAttr(resourceName, "variables.0.definition.0.multi_select.0.source.0.constant_list.0.values.0", "1"),
					resource.TestCheckResourceAttr(resourceName, "variables.0.definition.0.multi_select.0.source.0.constant_list.0.values.1", "2"),
					resource.TestCheckResourceAttr(resourceName, "variables.0.definition.0.multi_select.0.source.0.constant_list.0.values.2", "3"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceDashboardFromJson(t *testing.T) {
	resourceName := "coralogix_dashboard.test"
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
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", "dont drop me!"),
					resource.TestCheckResourceAttr(resourceName, "description", "dashboards team is messing with this 🗿"),
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

		resp, err := client.GetDashboard(ctx, &dashboard.GetDashboardRequest{DashboardId: expandUUID(rs.Primary.ID)})
		if err == nil {
			if resp.GetDashboard().GetId().GetValue() == rs.Primary.ID {
				return fmt.Errorf("dashboard still exists: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}

func testAccCoralogixResourceDashboard() string {
	return fmt.Sprintf(`resource "coralogix_dashboard" test {
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
                  lucene_query = "coralogix.metadata.severity=\"5\" OR coralogix.metadata.severity=\"6\" OR coralogix.metadata.severity=\"4\""
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
                    name   = "coralogix.metadata.severity"
                    values = ["6", "5", "4"]
                  }
                  filters {
                    name   = "coralogix.metadata.subsystemName"
                    values = ["coralogix-terraform-provider"]
                  }
                }
              }
              results_per_page = 20
              row_style        = "One_Line"
              columns {
                field           = "coralogix.timestamp"
              }
              columns {
                field           = "textObject.textObject.textObject.kubernetes.pod_id"
              }
              columns {
                field           = "coralogix.text"
              }
              columns {
                field           = "coralogix.metadata.applicationName"
              }
              columns {
                field           = "coralogix.metadata.subsystemName"
              }
              columns {
                field           = "coralogix.metadata.sdkId"
              }
              columns {
                field           = "textObject.log_obj.e2e_test.config"
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
	definition{
        multi_select{
          selected = ["1", "2", "3"]
          source{
            constant_list{
              values =["1", "2", "3"]
            }
          }
        }
    }
  }
}
`)
}

func testAccCoralogixResourceDashboardFromJson(jsonFilePath string) string {
	return fmt.Sprintf(`resource "coralogix_dashboard" test {
  	name        = "dont drop me!"
    description = "dashboards team is messing with this 🗿"
   	layout_json = file("%s")
	}
`, jsonFilePath)
}
