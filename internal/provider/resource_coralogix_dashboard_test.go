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
	"os"
	"path/filepath"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"google.golang.org/protobuf/types/known/wrapperspb"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var dashboardResourceName = "coralogix_dashboard.test"
var folderResourceName = "coralogix_dashboards_folder.test_folder"

func TestAccCoralogixResourceDashboard(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceDashboard(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
					resource.TestCheckResourceAttr(dashboardResourceName, "name", "test"),
					resource.TestCheckResourceAttr(dashboardResourceName, "description", "dashboards team is messing with this 🗿"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.options.name", "Status"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.options.color", "blue"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.options.description", "abc"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.options.collapsed", "false"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.height", "19"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.title", "status 4XX"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.metrics.promql_query", "http_requests_total{status!~\"4..\"}"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.legend.is_visible", "true"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.legend.columns.0", "max"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.legend.columns.1", "last"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.width", "0"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.title", "count"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.line_chart.query_definitions.0.query.logs.aggregations.0.type", "count"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.line_chart.legend.is_visible", "true"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.line_chart.legend.columns.0", "min"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.line_chart.legend.columns.1", "max"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.line_chart.legend.columns.2", "sum"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.line_chart.legend.columns.3", "avg"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.line_chart.legend.columns.4", "last"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.width", "10"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.2.title", "error throwing pods"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.2.definition.line_chart.query_definitions.0.query.logs.lucene_query", "coralogix.metadata.severity=5 OR coralogix.metadata.severity=\"6\" OR coralogix.metadata.severity=\"4\""),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.2.definition.line_chart.query_definitions.0.query.logs.group_by.0", "coralogix.metadata.subsystemName"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.2.definition.line_chart.query_definitions.0.query.logs.aggregations.0.type", "count"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.2.definition.line_chart.legend.is_visible", "true"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.2.definition.line_chart.legend.columns.0", "max"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.2.definition.line_chart.legend.columns.1", "last"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.2.width", "0"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.height", "28"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.0.title", "dashboards-api logz"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.0.description", "warnings, errors, criticals"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.0.definition.data_table.query.logs.filters.0.field", "coralogix.metadata.applicationName"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.0.definition.data_table.query.logs.filters.0.operator.type", "equals"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.0.definition.data_table.query.logs.filters.0.operator.selected_values.0", "staging"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.0.definition.data_table.results_per_page", "20"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.0.definition.data_table.row_style", "one_line"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.0.definition.data_table.columns.0.field", "coralogix.timestamp"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.0.definition.data_table.columns.1.field", "textObject.textObject.textObject.kubernetes.pod_id"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.0.definition.data_table.columns.2.field", "coralogix.text"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.0.definition.data_table.columns.3.field", "coralogix.metadata.applicationName"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.0.definition.data_table.columns.4.field", "coralogix.metadata.subsystemName"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.0.definition.data_table.columns.5.field", "coralogix.metadata.sdkId"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.0.definition.data_table.columns.6.field", "textObject.log_obj.e2e_test.config"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.1.widgets.0.width", "0"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables.0.name", "test_variable"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables.0.definition.multi_select.selected_values.0", "1"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables.0.definition.multi_select.selected_values.1", "2"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables.0.definition.multi_select.selected_values.2", "3"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables.0.definition.multi_select.source.constant_list.0", "1"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables.0.definition.multi_select.source.constant_list.1", "2"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables.0.definition.multi_select.source.constant_list.2", "3"),
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

func TestAccCoralogixResourceDashboardHexagonWidget(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceDashboardWithWidget(`{
            title      = "hexagon"
            definition = {
              hexagon = {
                min = 0
                max = 100
                decimal = 2
                threshold_type = "relative"
                thresholds = [{
                  from = 0
                  color = "var(--c-severity-log-verbose)"
                },
                {
                  from = 33
                  color = "var(--c-severity-log-warning)"
                },
                {
                  from = 66
                  color = "var(--c-severity-log-error)"
                }]
                query = {
                  logs = {
				    time_frame = {
					  relative = {
					    duration = "seconds:900" # 15 minutes
					  }
					}
                    aggregation = {
                      type = "count"
                    }
                    group_by = [{
                      keypath = ["subsystemname"]
                      scope = "label"
                    }]
                  }
                }
                legend_by = "groups"
                legend = {
                  is_visible = true
                }
              }
            }
            width = 0
          }`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.title", "hexagon"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.hexagon.query.logs.aggregation.type", "count"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.hexagon.query.logs.group_by.0.keypath.0", "subsystemname"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.hexagon.query.logs.group_by.0.scope", "label"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.hexagon.legend_by", "groups"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.hexagon.min", "0"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.hexagon.max", "100"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.hexagon.decimal", "2"),

					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.hexagon.query.logs.time_frame.relative.duration", "seconds:900"),
					resource.TestCheckTypeSetElemNestedAttrs(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.hexagon.thresholds.*",
						map[string]string{
							"from":  "0",
							"color": "var(--c-severity-log-verbose)",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.hexagon.thresholds.*",
						map[string]string{
							"from":  "33",
							"color": "var(--c-severity-log-warning)",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.hexagon.thresholds.*",
						map[string]string{
							"from":  "66",
							"color": "var(--c-severity-log-error)",
						},
					),
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

func TestAccCoralogixResourceDashboardLinechartWidget(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceDashboardWithWidget(`{
            title      = "line-chart"
            definition = {
              line_chart = {
				        stacked_line = "relative"
                query_definitions = [{
                query = {
                    spans = {
                      aggregations = [{
                        type             = "dimension"
                        field            = "trace_id"
                        aggregation_type = "unique_count"
                      }]
                      filters = [{
                        field = {
                          type  = "metadata"
                          value = "operation_name"
                        }
                        operator = {
                          type            = "equals"
                          selected_values = ["device_status_update"]
                        }
                      },
                      {
                        field = {
                          type  = "tag"
                          value = "deviceStatus"
                        }
                        operator = {
                          type            = "equals"
                          selected_values = ["CANDYBOX_OFFLINE"]
                        }
                      }]
                      group_by = [{
                        type  = "tag"
                        value = "deviceName"
                      }]
					          time_frame = {
                      relative = {
                        duration = "seconds:900" # 15 minutes
                      }
                    }               
                  }
                }
                color_scheme = "classic"
                is_visible   = true
                scale_type   = "linear"
                }, {
                  query = {
                    data_prime = {
                      query = "source logs"
                    }
                  }
                }]
                legend = {
                  is_visible     = true
                  group_by_query = true
                  placement      = "auto"
                },
                tooltip = {
                  show_labels = false
                  type        = "all"
                }
              }
            }
            width = 0
          }`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.title", "line-chart"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.1.query.data_prime.query", "source logs"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.aggregations.0.type", "dimension"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.group_by.0.type", "tag"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.group_by.0.value", "deviceName"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.color_scheme", "classic"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.is_visible", "true"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.scale_type", "linear"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.legend.is_visible", "true"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.legend.group_by_query", "true"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.legend.placement", "auto"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.tooltip.show_labels", "false"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.tooltip.type", "all"),

					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.time_frame.relative.duration", "seconds:900"),

					resource.TestCheckTypeSetElemNestedAttrs(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.filters.*",
						map[string]string{
							"field.type":                 "metadata",
							"field.value":                "operation_name",
							"operator.type":              "equals",
							"operator.selected_values.#": "1",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.query.spans.filters.*",
						map[string]string{
							"field.type":                 "tag",
							"field.value":                "deviceStatus",
							"operator.type":              "equals",
							"operator.selected_values.#": "1",
						},
					),
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

func TestAccCoralogixResourceDashboardGaugeWidget(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceDashboardWithWidget(`{
                title      = "gauge"
                definition = {
                  gauge = {
                    unit  = "milliseconds"
					decimal = 2
					display_series_name = false
					thresholds = [{
						from = 0
						color = "green"
						label = "GREEN!"
					}]
                    query = {
                      metrics = {
                        promql_query = "vector(1)"
                        aggregation  = "unspecified"
						time_frame = {
						  relative = {
						     duration = "seconds:900" 
						  }
						}
                      }
                    }
                  }
                }
          }`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.title", "gauge"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.gauge.query.metrics.promql_query", "vector(1)"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.gauge.query.metrics.aggregation", "unspecified"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.gauge.display_series_name", "false"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.gauge.decimal", "2"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.gauge.query.metrics.time_frame.relative.duration", "seconds:900"),
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
	filePath := parent + "/examples/resources/coralogix_dashboard/dashboard.json"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy,
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

func TestAccCoralogixResourceDashboardFromJsonWithFolder(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	parent := filepath.Dir(wd)
	filePath := parent + "/examples/resources/coralogix_dashboard/dashboard.json"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCoralogixDashboardAndFolderDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceDashboardFromJsonWithFolder(filePath),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
					resource.TestCheckResourceAttrSet(dashboardResourceName, "folder.id"),
					resource.TestCheckResourceAttrSet(folderResourceName, "id"),
				),
			},
		},
	})
}

func TestAccCoralogixResourceDashboardFromJsonWithVar(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	parent := filepath.Dir(wd)
	filePath := parent + "/examples/resources/coralogix_dashboard/dashboard_with_var_path.json"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceDashboardFromJsonWithVar(filePath),
				Check:  resource.ComposeAggregateTestCheckFunc(),
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

		dashboardId := wrapperspb.String(rs.Primary.ID)
		resp, err := client.Get(ctx, &cxsdk.GetDashboardRequest{DashboardId: dashboardId})
		if err == nil {
			if resp.GetDashboard().GetId().GetValue() == rs.Primary.ID {
				return fmt.Errorf("dashboard still exists: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}

func testAccCheckCoralogixDashboardAndFolderDestroy(s *terraform.State) error {
	dashboardsClient := testAccProvider.Meta().(*clientset.ClientSet).Dashboards()
	foldersClient := testAccProvider.Meta().(*clientset.ClientSet).DashboardsFolders()

	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_dashboard" && rs.Type != "coralogix_dashboards_folder" {
			continue
		}

		if rs.Type == "coralogix_dashboard" {
			dashboardId := wrapperspb.String(rs.Primary.ID)
			resp, err := dashboardsClient.Get(ctx, &cxsdk.GetDashboardRequest{DashboardId: dashboardId})
			if err == nil {
				if resp.GetDashboard().GetId().GetValue() == rs.Primary.ID {
					return fmt.Errorf("dashboard still exists: %s", rs.Primary.ID)
				}
			}
		}
		if rs.Type == "coralogix_dashboards_folder" {
			folderId := wrapperspb.String(rs.Primary.ID)
			resp, err := foldersClient.Get(ctx, &cxsdk.GetDashboardFolderRequest{FolderId: folderId})
			if err == nil {
				if resp.GetFolder().GetId().GetValue() == rs.Primary.ID {
					return fmt.Errorf("folder still exists: %s", rs.Primary.ID)
				}
			}
		}
	}

	return nil
}

func testAccCoralogixResourceDashboard() string {
	return `resource "coralogix_dashboard" test {
  name        = "test"
  description = "dashboards team is messing with this 🗿"
  time_frame = {
      relative = {
        duration = "seconds:900" # 15 minutes
      }
  }
  layout      = {
    sections = [
      {
        options = {
          name = "Status"
          description = "abc"
          collapsed = false
          color = "blue"
        }
        rows = [
          {
            height  = 19
            widgets = [
              {
                title      = "status 4XX"
                definition = {
                  line_chart = {
                    query_definitions = [
                      {
                        query = {
                          metrics = {
                            promql_query = "http_requests_total{status!~\"4..\"}"
                          }
                        }
                      },
                    ]
                    legend = {
                      is_visible = true
                      columns     = ["max", "last"]
                    }
                  }
                }
                width = 0
              },
              {
                title      = "count"
                definition = {
                  line_chart = {
                    query_definitions = [
                      {
                        query = {
                          logs = {
                            aggregations = [
                              {
                                type = "count"
                              },
                            ]
                          }
                        }
                      },
                    ]
			      legend = {
                   		is_visible = true
                   		 columns     = ["min", "max", "sum", "avg", "last"]
                  	}
                  } 
                }
                width = 10
              },
              {
                title      = "error throwing pods"
                definition = {
                  line_chart = {
                    query_definitions = [
                      {
                        query = {
                          logs = {
                            lucene_query = "coralogix.metadata.severity=5 OR coralogix.metadata.severity=\"6\" OR coralogix.metadata.severity=\"4\""
                            group_by     = ["coralogix.metadata.subsystemName"]
                            aggregations = [
                              {
                                type = "count"
                              },
                            ]
                          }
                        }
                      },
                    ]
                    legend = {
                      is_visible = true
                      columns     = ["max", "last"]
                    }
                  }
                }
                width = 0
              }
            ]
          },
          {
            height  = 28
            widgets = [
              {
                title       = "dashboards-api logz"
                description = "warnings, errors, criticals"
                definition  = {
                  data_table = {
                    query = {
                      logs = {
                        filters = [
                          {
                            field    = "coralogix.metadata.applicationName"
                            operator = {
                              type            = "equals"
                              selected_values = ["staging"]
                            }
                          }
                        ]
                      }
                    }
                    results_per_page = 20
                    row_style        = "one_line"
                    columns          = [
                      {
                        field = "coralogix.timestamp"
                      },
                      {
                        field = "textObject.textObject.textObject.kubernetes.pod_id"
                      },
                      {
                        field = "coralogix.text"
                      },
                      {
                        field = "coralogix.metadata.applicationName"
                      },
                      {
                        field = "coralogix.metadata.subsystemName"
                      },
                      {
                        field = "coralogix.metadata.sdkId"
                      },
                      {
                        field = "textObject.log_obj.e2e_test.config"
                      },
                    ]
                  }
                }
                width = 0
              }
            ],
          },
        ]
      },
    ]
  }
  variables = [
    {
      name         = "test_variable"
      display_name = "Test Variable"
      definition   = {
        multi_select = {
          selected_values = ["1", "2", "3"]
          source          = {
            constant_list = ["1", "2", "3"]
          }
          values_order_direction = "asc"
        }
      }
    },
    {
      name         = "test_variable2"
      display_name = "Test Variable 2"
      definition = {
        multi_select = {
          source = {
            query = {
              query = {
                metrics = {
                  label_value = {
                    label_filters = [
                      {
                        label = {
                          string_value = "service_name"
                        },
                        operator = {
                          type = "equals"
                          selected_values = [
                            {
                              string_value = "service_name"
                            }
                          ]
                        }
                      }
                    ]
                    metric_name = {
                      string_value = "test_metric"
                    }
                    label_name = {
                      string_value = "region"
                    }
                  }
                }
              }
            }
          }
          values_order_direction = "asc"
        }
      }
    },
  ]
}
`
}

func testAccCoralogixResourceDashboardFromJson(jsonFilePath string) string {
	return fmt.Sprintf(`resource "coralogix_dashboard" test {
   		content_json = file("%s")
	}
`, jsonFilePath)
}

func testAccCoralogixResourceDashboardFromJsonWithFolder(jsonFilePath string) string {
	return fmt.Sprintf(`
  resource "coralogix_dashboards_folder" test_folder {
    name = "test_folder"
  }
  resource "coralogix_dashboard" test {
      content_json = file("%s")
      folder = {
        id = coralogix_dashboards_folder.test_folder.id
      }
  }
`, jsonFilePath)
}

func testAccCoralogixResourceDashboardFromJsonWithVar(jsonFilePath string) string {
	return fmt.Sprintf(`
variable "dashboard_json_path" {
  type    = string
  default = "%s"
}

resource "coralogix_dashboard" test {
  content_json = file(var.dashboard_json_path)
}
`, jsonFilePath)
}

func TestParseRelativeTimeDuration(t *testing.T) {
	res, err := utils.ParseDuration("seconds:900", "relative")
	if err != nil {
		t.Fatal(err)
	}

	if res.Seconds() != 900 {
		t.Fatalf("expected 900 seconds, got %f", res.Seconds())
	}
}

func testAccCoralogixResourceDashboardWithWidget(widget string) string {
	return fmt.Sprintf(`resource "coralogix_dashboard" test {
name        = "test-the-widget"
description = "Widget Tester!"
time_frame = {
  relative = {
    duration = "seconds:900" # 15 minutes
  }
}
layout = {
  sections = [{
    rows = [{
      height = 19
      widgets = [
            %v
      ]
    }]
  }]
}
}
`, widget)
}

func TestAccCoralogixResourceDashboardLayoutColor(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "coralogix_dashboard" "test" {
  name        = "layout-color-test"
  description = "Testing layout section color option"
  time_frame = {
    relative = {
      duration = "seconds:900"
    }
  }

  layout = {
    sections = [{
      options = {
        name        = "Color Test Section"
        description = "Checking color"
        collapsed   = false
        color       = "cyan"
      }
      rows = [{
        height = 10
          widgets = [{
          title      = "placeholder"
          width      = 0
          definition = {
            line_chart = {
              query_definitions = [{
                query = {
                  logs = {
                    aggregations = [{
                      type = "count"
                    }]
                  }
                }
              }]
              legend = {
                is_visible = false
              }
            }
          }
        }]
      }]
    }]
  }
}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.options.name", "Color Test Section"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.options.color", "cyan"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.options.description", "Checking color"),
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
