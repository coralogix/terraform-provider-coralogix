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

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

var dashboardResourceName = "coralogix_dashboard.test"
var folderResourceName = "coralogix_dashboards_folder.test_folder"

func testAccDashboardVariablesV2Layout(name string) string {
	return fmt.Sprintf(`
resource "coralogix_dashboard" "test" {
  name = %q
  layout = {
    sections = [{
      rows = [{
        height = 10
        widgets = [{
          definition = {
            markdown = { markdown_text = "variables_v2" }
          }
        }]
      }]
    }]
  }
`, name)
}

func TestAccCoralogixResourceDashboardVariablesV2Static(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// omit → label defaults to value
				Config: testAccDashboardVariablesV2StaticConfig(name, `{ value = "production", is_default = true }`, "production"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.source.static.values.0.value", "production"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.source.static.values.0.label", "production"),
					testAccCheckDashboardVariablesV2StaticLabelOnAPI(dashboardResourceName, "production"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// omit → label set
				Config: testAccDashboardVariablesV2StaticConfig(name, `{ value = "production", label = "Prod", is_default = true }`, "production"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.source.static.values.0.label", "Prod"),
					testAccCheckDashboardVariablesV2StaticLabelOnAPI(dashboardResourceName, "Prod"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// label set → omit (recomputes from value)
				Config: testAccDashboardVariablesV2StaticConfig(name, `{ value = "production", is_default = true }`, "production"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.source.static.values.0.label", "production"),
					testAccCheckDashboardVariablesV2StaticLabelOnAPI(dashboardResourceName, "production"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// omit → label set again (label set → omit → label set)
				Config: testAccDashboardVariablesV2StaticConfig(name, `{ value = "production", label = "Prod", is_default = true }`, "production"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.source.static.values.0.label", "Prod"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// back to omit, then value change while label omitted
				Config: testAccDashboardVariablesV2StaticConfig(name, `{ value = "production", is_default = true }`, "production"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.source.static.values.0.label", "production"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// omit → value change (label follows new value)
				Config: testAccDashboardVariablesV2StaticConfig(name, `{ value = "prod", is_default = true }`, "prod"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.source.static.values.0.value", "prod"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.source.static.values.0.label", "prod"),
					testAccCheckDashboardVariablesV2StaticLabelOnAPI(dashboardResourceName, "prod"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// label set on value A
				Config: testAccDashboardVariablesV2StaticConfig(name, `{ value = "a", label = "Display", is_default = true }`, "a"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.source.static.values.0.value", "a"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.source.static.values.0.label", "Display"),
					testAccCheckDashboardVariablesV2StaticLabelOnAPI(dashboardResourceName, "Display"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// value A → value B with label set (label stays)
				Config: testAccDashboardVariablesV2StaticConfig(name, `{ value = "b", label = "Display", is_default = true }`, "b"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.source.static.values.0.value", "b"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.source.static.values.0.label", "Display"),
					testAccCheckDashboardVariablesV2StaticLabelOnAPI(dashboardResourceName, "Display"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			testAccDashboardImportStep(),
		},
	})
}

func testAccDashboardVariablesV2StaticConfig(name, staticValue, selected string) string {
	return testAccDashboardVariablesV2Layout(name) + `
  variables_v2 = [{
    name         = "environment"
    display_name = "Environment"
    source = {
      static = {
        all_option = { include_all = false }
        values = [` + staticValue + `]
      }
    }
    value = {
      single_string = { value = "` + selected + `", label = "` + selected + `" }
    }
  }]
}`
}

// testAccCheckDashboardVariablesV2StaticLabelOnAPI asserts expand defaulted the
// omitted static label onto the first static value in the live dashboard.
func testAccCheckDashboardVariablesV2StaticLabelOnAPI(resourceName, wantLabel string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found", resourceName)
		}
		client, err := dashboardOpenAPINewAcceptanceClient()
		if err != nil {
			return err
		}
		resp, httpResp, err := client.DashboardsServiceGetDashboard(context.TODO(), rs.Primary.ID).Execute()
		if err != nil {
			return fmt.Errorf("get dashboard: %s", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResp, err), "Get", rs.Primary.ID))
		}
		dashboard := resp.GetDashboard()
		vars := dashboard.GetVariablesV2()
		if len(vars) == 0 || vars[0].Source.Static == nil || len(vars[0].Source.Static.Values) == 0 {
			return fmt.Errorf("dashboard %s has no static variables_v2 values", rs.Primary.ID)
		}
		got := vars[0].Source.Static.Values[0].GetLabel()
		if got != wantLabel {
			return fmt.Errorf("API static values[0].label = %q, want %q", got, wantLabel)
		}
		return nil
	}
}

func TestAccCoralogixResourceDashboardVariablesV2Textbox(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDashboardVariablesV2Layout(name) + `
  variables_v2 = [{
    name             = "search"
    display_name     = "Search"
    display_full_row = true
    source = {
      textbox = {
        default_value = { default_string_value = { value = "hello" } }
      }
    }
    value = { single_string = { value = "hello", label = "hello" } }
  }]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.source.textbox.default_value.default_string_value.value", "hello"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.display_full_row", "true"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccCoralogixResourceDashboardVariablesV2LogsFieldValue(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDashboardVariablesV2Layout(name) + `
  variables_v2 = [{
    name         = "svc"
    display_name = "Service"
    source = {
      query = {
        all_option = { include_all = false }
        logs_query = {
          type = {
            field_value = {
              observation_field = {
                keypath = ["servicename"]
                scope   = "user_data"
              }
            }
          }
        }
      }
    }
    value = { multi_string = { selected_all = {} } }
  }]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.source.query.logs_query.type.field_value.observation_field.keypath.0", "servicename"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccCoralogixResourceDashboardVariablesV2SpansFieldValue(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDashboardVariablesV2Layout(name) + `
  variables_v2 = [{
    name         = "span"
    display_name = "Span"
    source = {
      query = {
        all_option = { include_all = false }
        spans_query = {
          type = {
            field_value = {
              observation_field = {
                keypath = ["service.name"]
                scope   = "user_data"
              }
            }
          }
        }
      }
    }
    value = { multi_string = { selected_all = {} } }
  }]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.source.query.spans_query.type.field_value.observation_field.scope", "user_data"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccCoralogixResourceDashboardVariablesV2Metrics(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDashboardVariablesV2Layout(name) + `
  variables_v2 = [{
    name         = "metric"
    display_name = "Metric"
    source = {
      query = {
        all_option = { include_all = false }
        metrics_query = {
          type = { metric_name = { metric_regex = ".*" } }
        }
      }
    }
    value = { multi_string = { selected_all = {} } }
  }]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.source.query.metrics_query.type.metric_name.metric_regex", ".*"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccCoralogixResourceDashboardVariablesV2ValueDisplayOptions(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDashboardVariablesV2Layout(name) + `
  variables_v2 = [{
    name         = "svc"
    display_name = "Service"
    source = {
      query = {
        all_option = { include_all = false }
        value_display_options = {
          value_regex = ".*"
        }
        logs_query = {
          type = {
            field_value = {
              observation_field = {
                keypath = ["servicename"]
                scope   = "user_data"
              }
            }
          }
        }
      }
    }
    value = { multi_string = { selected_all = {} } }
  }]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.source.query.value_display_options.value_regex", ".*"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccCoralogixResourceDashboardVariablesV2Promql(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDashboardVariablesV2Layout(name) + `
  variables_v2 = [{
    name         = "prom"
    display_name = "Prom"
    source = {
      query = {
        all_option = { include_all = false }
        metrics_query = {
          type = { promql_query = { query = "vector(1)" } }
        }
      }
    }
    value = {
      multi_string = {
        list = {
          values = [{ value = { value = "1", label = "one" } }]
        }
      }
    }
  }]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.source.query.metrics_query.type.promql_query.query", "vector(1)"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.source.query.metrics_query.type.promql_query.promql_query_type", "instant"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: testAccDashboardVariablesV2Layout(name) + `
  variables_v2 = [{
    name         = "prom"
    display_name = "Prom"
    source = {
      query = {
        all_option = { include_all = false }
        metrics_query = {
          type = {
            promql_query = {
              query             = "vector(1)"
              promql_query_type = "range"
            }
          }
        }
      }
    }
    value = {
      multi_string = {
        list = {
          values = [{ value = { value = "1", label = "one" } }]
        }
      }
    }
  }]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.source.query.metrics_query.type.promql_query.promql_query_type", "range"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccCoralogixResourceDashboardVariablesV2Dataprime(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDashboardVariablesV2Layout(name) + `
  variables_v2 = [{
    name         = "dp"
    display_name = "DP"
    source = {
      query = {
        all_option = { include_all = false }
        dataprime_query = {
          type = {
            query_text = { query = "source logs | limit 10" }
          }
        }
      }
    }
    value = { multi_string = { selected_all = {} } }
  }]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.source.query.dataprime_query.type.query_text.query", "source logs | limit 10"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.source.query.dataprime_query.type.query_text.data_mode_type", "high"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccCoralogixResourceDashboardVariablesV2MultiStringAll(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDashboardVariablesV2Layout(name) + `
  variables_v2 = [{
    name         = "svc"
    display_name = "Service"
    source = {
      query = {
        all_option = { include_all = true }
        logs_query = {
          type = {
            field_value = {
              observation_field = {
                keypath = ["servicename"]
                scope   = "user_data"
              }
            }
          }
        }
      }
    }
    value = { multi_string = { all = {} } }
  }]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.value.multi_string.all.%", "0"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			testAccDashboardImportStep(),
		},
	})
}

func TestAccCoralogixResourceDashboardVariablesV2MetricsLabelName(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDashboardVariablesV2Layout(name) + `
  variables_v2 = [{
    name         = "label_names"
    display_name = "Label names"
    source = {
      query = {
        all_option = { include_all = false }
        metrics_query = {
          type = { label_name = { metric_regex = "http_.*" } }
        }
      }
    }
    value = { multi_string = { selected_all = {} } }
  }]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.source.query.metrics_query.type.label_name.metric_regex", "http_.*"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			testAccDashboardImportStep(),
		},
	})
}

func TestAccCoralogixResourceDashboardVariablesV2MetricsLabelValue(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDashboardVariablesV2Layout(name) + `
  variables_v2 = [
    {
      name         = "source_metric"
      display_name = "Source metric"
      source = {
        static = {
          all_option = { include_all = false }
          values     = [{ value = "http_requests_total", is_default = true }]
        }
      }
      value = {
        single_string = { value = "http_requests_total", label = "http_requests_total" }
      }
    },
    {
      name         = "label_value"
      display_name = "Label value"
      source = {
        query = {
          all_option = { include_all = false }
          metrics_query = {
            type = {
              label_value = {
                metric_name = { variable_name = "source_metric" }
                label_name  = { string_value = "service" }
                label_filters = [
                  {
                    metric = { string_value = "http_requests_total" }
                    label  = { string_value = "region" }
                    operator = {
                      type            = "not_equals"
                      selected_values = [{ variable_name = "source_metric" }]
                    }
                  },
                  {
                    metric = { string_value = "http_requests_total" }
                    label  = { string_value = "environment" }
                    operator = {
                      type            = "equals"
                      selected_values = [{ string_value = "production" }]
                    }
                  },
                ]
              }
            }
          }
        }
      }
      value = { multi_string = { selected_all = {} } }
    },
  ]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.1.source.query.metrics_query.type.label_value.metric_name.variable_name", "source_metric"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.1.source.query.metrics_query.type.label_value.label_name.string_value", "service"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.1.source.query.metrics_query.type.label_value.label_filters.0.operator.type", "not_equals"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.1.source.query.metrics_query.type.label_value.label_filters.0.operator.selected_values.0.variable_name", "source_metric"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.1.source.query.metrics_query.type.label_value.label_filters.1.operator.type", "equals"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.1.source.query.metrics_query.type.label_value.label_filters.1.operator.selected_values.0.string_value", "production"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			testAccDashboardImportStep(),
		},
	})
}

func TestAccCoralogixResourceDashboardVariablesV2TextboxValueTypes(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDashboardVariablesV2TextboxValueConfig(name, `
      textbox = {
        default_value = {
          default_numeric_value = { value = 42, min = 1, max = 100, is_integer = true }
        }
      }
    `, `single_numeric = { value = 42, label = "42" }`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.source.textbox.default_value.default_numeric_value.value", "42"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.value.single_numeric.value", "42"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: testAccDashboardVariablesV2TextboxValueConfig(name, `
      textbox = {
        default_value = {
          default_regex_value = { value = "error.*" }
        }
      }
    `, `regex = { value = "error.*", label = "error.*" }`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.source.textbox.default_value.default_regex_value.value", "error.*"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.value.regex.value", "error.*"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: testAccDashboardVariablesV2TextboxValueConfig(name, `
      textbox = {
        default_value = {
          default_lucene_value = { value = "severity:ERROR" }
        }
      }
    `, `lucene = { value = "severity:ERROR", label = "severity:ERROR" }`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.source.textbox.default_value.default_lucene_value.value", "severity:ERROR"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.source.textbox.default_value.default_lucene_value.data_mode_type", "high"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.value.lucene.value", "severity:ERROR"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: testAccDashboardVariablesV2TextboxValueConfig(name, `
      textbox = {
        default_value = {
          default_interval_value = { value = "1m" }
        }
      }
    `, `interval = { value = "1m", label = "1m" }`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.source.textbox.default_value.default_interval_value.value", "1m"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables_v2.0.value.interval.value", "1m"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			testAccDashboardImportStep(),
		},
	})
}

func testAccDashboardVariablesV2TextboxValueConfig(name, source, value string) string {
	return testAccDashboardVariablesV2Layout(name) + fmt.Sprintf(`
  variables_v2 = [{
    name             = "typed"
    display_name     = "Typed"
    display_full_row = true
    source = {
%s
    }
    value = { %s }
  }]
}`, source, value)
}

func TestDashboardLegacyAcceptanceConfigsParse(t *testing.T) {
	t.Parallel()
	name := "tf-acc-dashboard-parse"
	configs := map[string]string{
		"structured":       testAccCoralogixResourceDashboard(name),
		"content-json":     testAccCoralogixResourceDashboardFromJson("/tmp/dashboard.json", name),
		"content-folder":   testAccCoralogixResourceDashboardFromJsonWithFolder("/tmp/dashboard.json", name, name+"-folder"),
		"content-variable": testAccCoralogixResourceDashboardFromJsonWithVar("/tmp/dashboard.json", name),
		"widget":           testAccCoralogixResourceDashboardWithWidget(name, testAccCoralogixResourceDashboardCountWidget()),
		"access-policy":    testAccCoralogixResourceDashboardWithAccessPolicy(name, testAccCoralogixDashboardAccessPolicyPretty()),
		"folder-id":        testAccCoralogixResourceDashboardFolderIDNoDrift(name, name+"-folder"),
	}
	for fixture, config := range configs {
		fixture, config := fixture, config
		t.Run(fixture, func(t *testing.T) {
			t.Parallel()
			_, diagnostics := hclsyntax.ParseConfig([]byte(config), fixture+".tf", hcl.InitialPos)
			if diagnostics.HasErrors() {
				t.Fatalf("legacy dashboard acceptance config is invalid HCL:\n%s", diagnostics.Error())
			}
		})
	}
}

func TestAccCoralogixResourceDashboard(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceDashboard(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
					resource.TestCheckResourceAttr(dashboardResourceName, "name", name),
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
			testAccDashboardImportStep(),
		},
	})
}

func TestAccCoralogixResourceDashboardAccessPolicy(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	updatedName := name + "-updated"
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceDashboardWithAccessPolicy(name, testAccCoralogixDashboardAccessPolicyPretty()) +
					testAccCoralogixDataSourceDashboard_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
					resource.TestCheckResourceAttrSet(dashboardResourceName, "access_policy"),
					testAccCheckDashboardAccessPolicy(dashboardDataSourceName, testAccCoralogixDashboardAccessPolicyPretty()),
				),
			},
			{
				Config:   testAccCoralogixResourceDashboardWithAccessPolicy(name, testAccCoralogixDashboardAccessPolicyPretty()),
				PlanOnly: true,
			},
			{
				Config:   testAccCoralogixResourceDashboardWithAccessPolicy(name, testAccCoralogixDashboardAccessPolicyReorderedObjectKeys()),
				PlanOnly: true,
			},
			{
				Config: testAccCoralogixResourceDashboardWithoutAccessPolicy(updatedName) +
					testAccCoralogixDataSourceDashboard_read(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectKnownValue(
							dashboardResourceName,
							tfjsonpath.New("access_policy"),
							knownvalue.StringFunc(func(got string) error {
								if !utils.JSONStringsEqual(got, testAccCoralogixDashboardAccessPolicyPretty()) {
									return fmt.Errorf("planned access_policy = %q, want JSON equivalent to %q", got, testAccCoralogixDashboardAccessPolicyPretty())
								}

								return nil
							}),
						),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, "name", updatedName),
					testAccCheckDashboardAccessPolicy(dashboardResourceName, testAccCoralogixDashboardAccessPolicyPretty()),
					testAccCheckDashboardAccessPolicy(dashboardDataSourceName, testAccCoralogixDashboardAccessPolicyPretty()),
				),
			},
			testAccDashboardImportStep(),
		},
	})
}

func testAccCheckDashboardAccessPolicy(resourceName, expected string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		resourceState, ok := state.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found", resourceName)
		}

		got := resourceState.Primary.Attributes["access_policy"]
		if !utils.JSONStringsEqual(got, expected) {
			return fmt.Errorf("%s access_policy = %q, want JSON equivalent to %q", resourceName, got, expected)
		}

		return nil
	}
}

func testAccDashboardImportStep(otherIgnoredAttributes ...string) resource.TestStep {
	return testAccDashboardImportStateStep(resource.TestStep{
		ResourceName:      dashboardResourceName,
		ImportState:       true,
		ImportStateVerify: true,
	}, otherIgnoredAttributes...)
}

func testAccDashboardImportStateStep(step resource.TestStep, otherIgnoredAttributes ...string) resource.TestStep {
	var dashboardID string
	var expectedAccessPolicy string
	var expectedAccessPolicySet bool
	originalImportStateIDFunc := step.ImportStateIdFunc
	originalImportStateCheck := step.ImportStateCheck

	step.ImportStateIdFunc = func(state *terraform.State) (string, error) {
		dashboardState, ok := state.RootModule().Resources[dashboardResourceName]
		if !ok {
			return "", fmt.Errorf("resource %s not found", dashboardResourceName)
		}
		if dashboardState.Primary == nil {
			return "", fmt.Errorf("resource %s has no primary state", dashboardResourceName)
		}

		dashboardID = dashboardState.Primary.ID
		expectedAccessPolicy, expectedAccessPolicySet = dashboardState.Primary.Attributes["access_policy"]
		if originalImportStateIDFunc != nil {
			return originalImportStateIDFunc(state)
		}
		return dashboardID, nil
	}
	step.ImportStateVerifyIgnore = append([]string{"access_policy"}, step.ImportStateVerifyIgnore...)
	step.ImportStateVerifyIgnore = append(step.ImportStateVerifyIgnore, otherIgnoredAttributes...)
	step.ImportStateCheck = func(states []*terraform.InstanceState) error {
		for _, state := range states {
			if state.ID != dashboardID {
				continue
			}

			importedAccessPolicy, importedAccessPolicySet := state.Attributes["access_policy"]
			if importedAccessPolicySet != expectedAccessPolicySet {
				return fmt.Errorf("imported access_policy presence = %t, want %t", importedAccessPolicySet, expectedAccessPolicySet)
			}
			if expectedAccessPolicySet && !utils.JSONStringsEqual(importedAccessPolicy, expectedAccessPolicy) {
				return fmt.Errorf("imported access_policy = %q, want JSON equivalent to %q", importedAccessPolicy, expectedAccessPolicy)
			}

			if originalImportStateCheck != nil {
				return originalImportStateCheck(states)
			}
			return nil
		}

		return fmt.Errorf("imported dashboard %q not found in %d state entries", dashboardID, len(states))
	}
	return step
}

func TestAccCoralogixResourceDashboardHexagonWidget(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceDashboardWithWidget(name, `{
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
			testAccDashboardImportStep(),
		},
	})
}

func TestAccCoralogixResourceDashboardLinechartWidget(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceDashboardWithWidget(name, `{
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
			testAccDashboardImportStep(),
		},
	})
}

func TestAccCoralogixResourceDashboardGaugeWidget(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceDashboardWithWidget(name, `{
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
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.gauge.query.metrics.aggregation", utils.UNSPECIFIED),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.gauge.display_series_name", "false"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.gauge.decimal", "2"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.gauge.query.metrics.time_frame.relative.duration", "seconds:900"),
				),
			},
			testAccDashboardImportStep(),
		},
	})
}

func TestAccCoralogixResourceDashboardGaugeWidgetDataPrime(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceDashboardWithWidget(name, `{
  title = "gauge_dataprime"
  definition = {
    gauge = {
      query = {
        data_prime = {
          query = <<-EOT
source logs
| filter 1 == 1
| aggregate count() as c
| choose c
EOT
        }
      }
      min            = 0
      max            = 100
      show_inner_arc = true
      show_outer_arc = true
      unit           = "percent100"
      data_mode_type = "archive"
      threshold_by   = "value"
      thresholds = [{
        from  = 0
        color = "green"
      }]
    }
  }
}`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.title", "gauge_dataprime"),
					resource.TestCheckResourceAttrSet(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.gauge.query.data_prime.query"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.gauge.min", "0"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.gauge.max", "100"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.gauge.unit", "percent100"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.gauge.data_mode_type", "archive"),
				),
			},
			testAccDashboardImportStep(),
		},
	})
}

func TestAccCoralogixResourceDashboardWidgetReference(t *testing.T) {
	sourceName := dashboardOpenAPIFixtureName(t.Name() + "-source")
	consumerName := dashboardOpenAPIFixtureName(t.Name() + "-consumer")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceDashboardWidgetReference(sourceName, consumerName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("coralogix_dashboard.source", "id"),
					resource.TestCheckResourceAttrSet("coralogix_dashboard.source", "layout.sections.0.rows.0.widgets.0.id"),
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
					resource.TestCheckResourceAttrSet(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.reference.dashboard_id"),
					resource.TestCheckResourceAttrSet(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.reference.widget_id"),
					resource.TestCheckResourceAttrPair(
						dashboardResourceName, "layout.sections.0.rows.0.widgets.0.reference.dashboard_id",
						"coralogix_dashboard.source", "id",
					),
					resource.TestCheckResourceAttrPair(
						dashboardResourceName, "layout.sections.0.rows.0.widgets.0.reference.widget_id",
						"coralogix_dashboard.source", "layout.sections.0.rows.0.widgets.0.id",
					),
					resource.TestCheckNoResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			testAccDashboardImportStep(),
		},
	})
}

func testAccCoralogixResourceDashboardWidgetReference(sourceName, consumerName string) string {
	return fmt.Sprintf(`resource "coralogix_dashboard" "source" {
  name        = %q
  description = "source dashboard for widget reference"
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
          definition = {
            markdown = {
              markdown_text = "shared widget"
            }
          }
        }]
      }]
    }]
  }
}

resource "coralogix_dashboard" "test" {
  name        = %q
  description = "consumer dashboard with widget reference"
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
          reference = {
            dashboard_id = coralogix_dashboard.source.id
            widget_id    = coralogix_dashboard.source.layout.sections[0].rows[0].widgets[0].id
          }
        }]
      }]
    }]
  }
}
`, sourceName, consumerName)
}

func TestAccCoralogixResourceDashboardGaugeWidgetThresholdType(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceDashboardWithWidget(name, `{
  title = "gauge_threshold_type"
  definition = {
    gauge = {
      unit           = "milliseconds"
      threshold_type = "absolute"
      thresholds = [{
        from  = 0
        color = "green"
      }]
      query = {
        metrics = {
          promql_query = "vector(1)"
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
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.title", "gauge_threshold_type"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.gauge.threshold_type", "absolute"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			testAccDashboardImportStep(),
		},
	})
}

func TestAccCoralogixResourceDashboardDataTableWidget(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceDashboardWithWidget(name, `{
  title = "data_table"
  definition = {
    data_table = {
      results_per_page = 100
      row_style        = "one_line"
      query = {
        metrics = {
          promql_query = "vector(0)"
          promql_query_type = "instant"
        }
      }
    }
  }
}`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.title", "data_table"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.data_table.query.metrics.promql_query", "vector(0)"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.data_table.query.metrics.promql_query_type", "instant"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.data_table.row_style", "one_line"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.data_table.results_per_page", "100"),
				),
			},
			testAccDashboardImportStep(),
		},
	})
}

func TestAccCoralogixResourceDashboardDataTableWidgetObservationFieldFilter(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "coralogix_dashboard" "test" {
  name        = %q
  description = "Reproducer for #496"
  time_frame = {
    relative = {
      duration = "seconds:900"
    }
  }
  layout = {
    sections = [{
      options = {
        name        = "section-without-color"
        description = "Exercises flattenDashboardOptions when color is unset."
      }
      rows = [{
        height = 19
        widgets = [{
          title = "logs-with-observation-field"
          definition = {
            data_table = {
              results_per_page = 100
              row_style        = "one_line"
              columns = [
                { field = "coralogix.timestamp" },
                { field = "coralogix.text" },
                { field = "coralogix.metadata.subsystemName" },
              ]
              query = {
                logs = {
                  filters = [{
                    observation_field = {
                      keypath = ["subsystemname"]
                      scope   = "label"
                    }
                    operator = {
                      type            = "equals"
                      selected_values = ["pubby-publisher"]
                    }
                  }]
                }
              }
            }
          }
        }]
      }]
    }]
  }
}
`, name),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.options.name", "section-without-color"),
					resource.TestCheckNoResourceAttr(dashboardResourceName, "layout.sections.0.options.color"),
					resource.TestCheckNoResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.data_table.query.logs.filters.0.field"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.data_table.query.logs.filters.0.observation_field.keypath.0", "subsystemname"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.data_table.query.logs.filters.0.observation_field.scope", "label"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.data_table.query.logs.filters.0.operator.type", "equals"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.data_table.query.logs.filters.0.operator.selected_values.0", "pubby-publisher"),
				),
			},
			testAccDashboardImportStep(),
		},
	})
}

func TestAccCoralogixResourceDashboardFromJson(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	parent := filepath.Dir(filepath.Dir(wd))
	filePath := parent + "/examples/resources/coralogix_dashboard/dashboard.json"
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceDashboardFromJson(filePath, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
				),
			},
		},
	})
}

func TestAccCoralogixResourceDashboardFromJsonWithFolder(t *testing.T) {
	dashboardName := dashboardOpenAPIFixtureName(t.Name())
	folderName := dashboardOpenAPIFixtureName(t.Name() + "-folder")
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	parent := filepath.Dir(filepath.Dir(wd))
	filePath := parent + "/examples/resources/coralogix_dashboard/dashboard.json"
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceDashboardFromJsonWithFolder(filePath, dashboardName, folderName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
					resource.TestCheckResourceAttrSet(dashboardResourceName, "folder.id"),
					resource.TestCheckResourceAttrSet(folderResourceName, "id"),
				),
			},
		},
	})
}

func TestAccCoralogixResourceDashboardFolderIDNoDrift(t *testing.T) {
	dashboardName := dashboardOpenAPIFixtureName(t.Name())
	folderName := dashboardOpenAPIFixtureName(t.Name() + "-folder")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceDashboardFolderIDNoDrift(dashboardName, folderName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
					resource.TestCheckResourceAttrSet(dashboardResourceName, "folder.id"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			testAccDashboardImportStep("folder"),
		},
	})
}

func testAccCoralogixResourceDashboardFolderIDNoDrift(dashboardName, folderName string) string {
	return fmt.Sprintf(`
resource "coralogix_dashboards_folder" "test_folder" {
  name = %q
}

resource "coralogix_dashboard" "test" {
  name        = %q
  description = "Dashboard with folder.id should not drift on next plan"

  layout = {
    sections = [
      {
        rows = [
          {
            height = 19
            widgets = [
              {
                title = "placeholder"
                definition = {
                  line_chart = {
                    query_definitions = [
                      {
                        query = {
                          metrics = {
                            promql_query = "up"
                          }
                        }
                      },
                    ]
                  }
                }
              },
            ]
          },
        ]
      },
    ]
  }

  folder = {
    id = coralogix_dashboards_folder.test_folder.id
  }
}
`, folderName, dashboardName)
}

func TestAccCoralogixResourceDashboardFromJsonWithVar(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	parent := filepath.Dir(filepath.Dir(wd))
	filePath := parent + "/examples/resources/coralogix_dashboard/dashboard_with_var_path.json"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceDashboardFromJsonWithVar(filePath, name),
				Check:  resource.ComposeAggregateTestCheckFunc(),
			},
		},
	})
}

func testAccCheckDashboardDestroy(t *testing.T) resource.TestCheckFunc {
	t.Helper()
	return func(s *terraform.State) error {
		client, err := dashboardOpenAPINewAcceptanceClient()
		if err != nil {
			return err
		}

		ctx := context.TODO()

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "coralogix_dashboard" {
				continue
			}

			resp, httpResp, err := client.DashboardsServiceGetDashboard(ctx, rs.Primary.ID).Execute()
			if err == nil {
				dashboard := resp.GetDashboard()
				if dashboard.GetId() == rs.Primary.ID {
					return fmt.Errorf("dashboard still exists: %s", rs.Primary.ID)
				}
				continue
			}

			apiErr := cxsdkOpenapi.NewAPIError(httpResp, err)
			if cxsdkOpenapi.Code(apiErr) == 404 {
				continue
			}
			return fmt.Errorf("error checking dashboard destroy for %s: %s", rs.Primary.ID, utils.FormatOpenAPIErrors(apiErr, "Get", rs.Primary.ID))
		}

		return nil
	}
}

func testAccCoralogixResourceDashboard(name string) string {
	return fmt.Sprintf(`resource "coralogix_dashboard" test {
  name        = %q
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
`, name)
}

func testAccCoralogixResourceDashboardFromJson(jsonFilePath, name string) string {
	return fmt.Sprintf(`resource "coralogix_dashboard" test {
		content_json = jsonencode(merge(jsondecode(file(%q)), { name = %q }))
	}
`, jsonFilePath, name)
}

func testAccCoralogixResourceDashboardFromJsonWithFolder(jsonFilePath, dashboardName, folderName string) string {
	return fmt.Sprintf(`
  resource "coralogix_dashboards_folder" test_folder {
    name = %q
  }
  resource "coralogix_dashboard" test {
      content_json = jsonencode(merge(jsondecode(file(%q)), { name = %q }))
      folder = {
        id = coralogix_dashboards_folder.test_folder.id
      }
  }
`, folderName, jsonFilePath, dashboardName)
}

func testAccCoralogixResourceDashboardFromJsonWithVar(jsonFilePath, name string) string {
	return fmt.Sprintf(`
variable "dashboard_json_path" {
  type    = string
  default = %q
}

resource "coralogix_dashboard" test {
  content_json = jsonencode(merge(jsondecode(file(var.dashboard_json_path)), { name = %q }))
}
`, jsonFilePath, name)
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

func testAccCoralogixResourceDashboardWithWidget(name, widget string) string {
	return fmt.Sprintf(`resource "coralogix_dashboard" test {
name        = %q
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
`, name, widget)
}

func testAccCoralogixResourceDashboardWithAccessPolicy(name, accessPolicy string) string {
	return testAccCoralogixResourceDashboardAccessPolicyConfig(
		name,
		fmt.Sprintf(`  access_policy = <<EOT
%s
EOT
`, accessPolicy),
	)
}

func testAccCoralogixResourceDashboardWithoutAccessPolicy(name string) string {
	return testAccCoralogixResourceDashboardAccessPolicyConfig(name, "")
}

func testAccCoralogixResourceDashboardAccessPolicyConfig(name, accessPolicyBlock string) string {
	return fmt.Sprintf(`resource "coralogix_dashboard" test {
  name = %q

%s

  time_frame = {
    relative = {
      duration = "seconds:900"
    }
  }

  auto_refresh = {
    type = "off"
  }

  layout = {
    sections = [{
      rows = [{
        height = 19
        widgets = [
          %s
        ]
      }]
    }]
  }
}
`, name, accessPolicyBlock, testAccCoralogixResourceDashboardCountWidget())
}

func testAccCoralogixDashboardAccessPolicyPretty() string {
	return `{
  "version": "2025-01-01",
  "default": {
    "permissions": {
      "team-dashboards:UpdateAccessPolicy": "grant",
      "team-dashboards:Read": "grant",
      "team-dashboards:Update": "grant",
      "team-dashboards:ReadAccessPolicy": "grant"
    }
  },
  "rules": []
}`
}

func testAccCoralogixDashboardAccessPolicyReorderedObjectKeys() string {
	return `{"rules":[],"default":{"permissions":{"team-dashboards:Update":"grant","team-dashboards:ReadAccessPolicy":"grant","team-dashboards:UpdateAccessPolicy":"grant","team-dashboards:Read":"grant"}},"version":"2025-01-01"}`
}

func testAccCoralogixResourceDashboardCountWidget() string {
	return `{
  title = "count"
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
        placement  = "unspecified"
      }
    }
  }
  width = 0
}`
}

func TestAccCoralogixResourceDashboardMultiSelectSelectionType(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "coralogix_dashboard" "test" {
  name        = %q
  description = "exercises multi_select selection_type round-trip"
  time_frame = {
    relative = {
      duration = "seconds:900"
    }
  }
  layout = {
    sections = [{
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
                    aggregations = [{ type = "count" }]
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
  variables = [
    {
      name         = "environment"
      display_name = "Environment"
      definition = {
        multi_select = {
          selected_values        = ["staging"]
          values_order_direction = "asc"
          selection_type         = "single"
          source = {
            constant_list = ["staging", "test-trade"]
          }
        }
      }
    },
  ]
}
`, name),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables.0.name", "environment"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables.0.definition.multi_select.selection_type", "single"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables.0.definition.multi_select.values_order_direction", "asc"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables.0.definition.multi_select.source.constant_list.0", "staging"),
					resource.TestCheckResourceAttr(dashboardResourceName, "variables.0.definition.multi_select.source.constant_list.1", "test-trade"),
				),
			},
			testAccDashboardImportStep(),
		},
	})
}

func TestAccCoralogixResourceDashboardLayoutColor(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "coralogix_dashboard" "test" {
  name        = %q
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
				`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.options.name", "Color Test Section"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.options.color", "cyan"),
					resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.options.description", "Checking color"),
				),
			},
			testAccDashboardImportStep(),
		},
	})
}

func TestAccCoralogixResourceDashboardColorsBy(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	config := func(barColorsBy, horizontalColorsBy string) string {
		return fmt.Sprintf(`
resource "coralogix_dashboard" "test" {
  name        = %q
  description = "Testing colors_by branches"
  time_frame = {
    relative = {
      duration = "seconds:900"
    }
  }

  layout = {
    sections = [{
      rows = [{
        height = 19
        widgets = [
          {
            title = "bar chart"
            definition = {
              bar_chart = {
                query     = { logs = { aggregation = { type = "count" } } }
                colors_by = %q
              }
            }
          },
          {
            title = "horizontal bar chart"
            definition = {
              horizontal_bar_chart = {
                query     = { logs = { aggregation = { type = "count" } } }
                colors_by = %q
              }
            }
          },
        ]
      }]
    }]
  }
}
`, name, barColorsBy, horizontalColorsBy)
	}
	checks := func(barColorsBy, horizontalColorsBy string) resource.TestCheckFunc {
		return resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.0.definition.bar_chart.colors_by", barColorsBy),
			resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.horizontal_bar_chart.colors_by", horizontalColorsBy),
		)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: config("query", "category"),
				Check:  checks("query", "category"),
			},
			{
				Config: config("category", "query"),
				Check:  checks("category", "query"),
			},
			{
				Config: config("stack", "aggregation"),
				Check:  checks("stack", "aggregation"),
			},
			testAccDashboardImportStep(),
		},
	})
}

// hash_colors is a plain Optional bool on all four widgets. The last step removes it from the
// config entirely: if the API echoed hashColors: false for an omitted field, that step would
// leave a permanent diff and the attribute would need Optional+Computed with a false default.
func TestAccCoralogixResourceDashboardHashColors(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	config := func(hashColors string) string {
		return fmt.Sprintf(`
resource "coralogix_dashboard" "test" {
  name        = %q
  description = "Testing hash_colors on the classic widgets"
  time_frame = {
    relative = {
      duration = "seconds:900"
    }
  }

  layout = {
    sections = [{
      rows = [{
        height = 19
        widgets = [
          {
            title = "line chart"
            definition = {
              line_chart = {
                query_definitions = [{
                  query = { logs = { aggregations = [{ type = "count" }] } }
                  %[2]s
                }]
              }
            }
          },
          {
            title = "bar chart"
            definition = {
              bar_chart = {
                query = { logs = { aggregation = { type = "count" } } }
                %[2]s
              }
            }
          },
          {
            title = "horizontal bar chart"
            definition = {
              horizontal_bar_chart = {
                query = { logs = { aggregation = { type = "count" } } }
                %[2]s
              }
            }
          },
          {
            title = "pie chart"
            definition = {
              pie_chart = {
                query            = { logs = { aggregation = { type = "count" } } }
                label_definition = {}
                %[2]s
              }
            }
          },
        ]
      }]
    }]
  }
}
`, name, hashColors)
	}

	widgetPaths := []string{
		"layout.sections.0.rows.0.widgets.0.definition.line_chart.query_definitions.0.hash_colors",
		"layout.sections.0.rows.0.widgets.1.definition.bar_chart.hash_colors",
		"layout.sections.0.rows.0.widgets.2.definition.horizontal_bar_chart.hash_colors",
		"layout.sections.0.rows.0.widgets.3.definition.pie_chart.hash_colors",
	}
	checks := func(value string) resource.TestCheckFunc {
		funcs := []resource.TestCheckFunc{resource.TestCheckResourceAttrSet(dashboardResourceName, "id")}
		for _, path := range widgetPaths {
			funcs = append(funcs, resource.TestCheckResourceAttr(dashboardResourceName, path, value))
		}
		return resource.ComposeAggregateTestCheckFunc(funcs...)
	}
	checksUnset := func() resource.TestCheckFunc {
		funcs := []resource.TestCheckFunc{resource.TestCheckResourceAttrSet(dashboardResourceName, "id")}
		for _, path := range widgetPaths {
			funcs = append(funcs, resource.TestCheckNoResourceAttr(dashboardResourceName, path))
		}
		return resource.ComposeAggregateTestCheckFunc(funcs...)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: config("hash_colors = true"),
				Check:  checks("true"),
			},
			{
				Config: config("hash_colors = false"),
				Check:  checks("false"),
			},
			{
				Config: config(""),
				Check:  checksUnset(),
			},
			testAccDashboardImportStep(),
		},
	})
}

// The line chart display and query fields are plain Optional scalars, except the
// three enums, which are Optional+Computed with an "unspecified" default. The
// last step removes every optional field: if the API echoed a value for an
// omitted field, that step would leave a permanent diff.
func TestAccCoralogixResourceDashboardLineChartParityFields(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	config := func(chartFields, definitionFields, metricsFields string) string {
		return fmt.Sprintf(`
resource "coralogix_dashboard" "test" {
  name        = %q
  description = "Testing the line chart display and query fields"
  time_frame = {
    relative = {
      duration = "seconds:900"
    }
  }

  layout = {
    sections = [{
      rows = [{
        height = 19
        widgets = [
          {
            title = "metrics line chart"
            definition = {
              line_chart = {
                %[2]s
                query_definitions = [{
                  query = {
                    metrics = {
                      promql_query = "up"
                      %[4]s
                    }
                  }
                  %[3]s
                }]
              }
            }
          },
          {
            title = "logs line chart"
            definition = {
              line_chart = {
                query_definitions = [{
                  query = {
                    logs = {
                      aggregations = [{ type = "count" }]
                      group_bys = [{
                        keypath = ["log.level"]
                        scope   = "user_data"
                      }]
                    }
                  }
                }]
              }
            }
          },
          {
            title = "spans line chart"
            definition = {
              line_chart = {
                query_definitions = [{
                  query = {
                    spans = {
                      aggregations = [{ type = "metric", aggregation_type = "avg", field = "duration" }]
                      group_bys = [{
                        keypath = ["service", "name"]
                        scope   = "metadata"
                      }]
                    }
                  }
                }]
              }
            }
          },
        ]
      }]
    }]
  }
}
`, name, chartFields, definitionFields, metricsFields)
	}

	const chart = "layout.sections.0.rows.0.widgets.0.definition.line_chart"
	const definition = chart + ".query_definitions.0"
	const metrics = definition + ".query.metrics"

	setChecks := resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
		resource.TestCheckResourceAttr(dashboardResourceName, chart+".connect_nulls", "true"),
		resource.TestCheckResourceAttr(dashboardResourceName, chart+".use_data_time_range", "true"),
		resource.TestCheckResourceAttr(dashboardResourceName, chart+".x_axis_time_format", "dd_mm_hh_mm"),
		resource.TestCheckResourceAttr(dashboardResourceName, definition+".custom_unit", "requests/s"),
		resource.TestCheckResourceAttr(dashboardResourceName, definition+".decimal", "3"),
		resource.TestCheckResourceAttr(dashboardResourceName, definition+".decimal_precision", "true"),
		resource.TestCheckResourceAttr(dashboardResourceName, definition+".y_axis_max", "120"),
		resource.TestCheckResourceAttr(dashboardResourceName, definition+".y_axis_min", "-5"),
		resource.TestCheckResourceAttr(dashboardResourceName, metrics+".editor_mode", "builder"),
		resource.TestCheckResourceAttr(dashboardResourceName, metrics+".series_limit_type", "by_point_count"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.1.definition.line_chart.query_definitions.0.query.logs.group_bys.0.keypath.0", "log.level"),
		resource.TestCheckResourceAttr(dashboardResourceName, "layout.sections.0.rows.0.widgets.2.definition.line_chart.query_definitions.0.query.spans.group_bys.0.scope", "metadata"),
	)
	unsetChecks := resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
		resource.TestCheckNoResourceAttr(dashboardResourceName, chart+".connect_nulls"),
		resource.TestCheckNoResourceAttr(dashboardResourceName, chart+".use_data_time_range"),
		// The new enums are Optional+Computed with no static default, because the
		// API supplies a value when the attribute is omitted. Terraform keeps the
		// prior value rather than reverting it, which is what makes the plan empty.
		resource.TestCheckResourceAttr(dashboardResourceName, chart+".x_axis_time_format", "dd_mm_hh_mm"),
		resource.TestCheckNoResourceAttr(dashboardResourceName, definition+".custom_unit"),
		resource.TestCheckNoResourceAttr(dashboardResourceName, definition+".decimal"),
		resource.TestCheckNoResourceAttr(dashboardResourceName, definition+".decimal_precision"),
		resource.TestCheckNoResourceAttr(dashboardResourceName, definition+".y_axis_max"),
		resource.TestCheckNoResourceAttr(dashboardResourceName, definition+".y_axis_min"),
		resource.TestCheckResourceAttr(dashboardResourceName, metrics+".editor_mode", "builder"),
		resource.TestCheckResourceAttr(dashboardResourceName, metrics+".series_limit_type", "by_point_count"),
	)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: config(`
                connect_nulls       = true
                use_data_time_range = true
                x_axis_time_format  = "dd_mm_hh_mm"`,
					`
                  custom_unit       = "requests/s"
                  decimal           = 3
                  decimal_precision = true
                  y_axis_max        = 120
                  y_axis_min        = -5`,
					`
                      editor_mode       = "builder"
                      series_limit_type = "by_point_count"`),
				Check: setChecks,
			},
			testAccDashboardImportStep(),
			{
				Config: config("", "", ""),
				Check:  unsetChecks,
			},
			// Removing an Optional+Computed attribute cannot clear it: Terraform
			// keeps the last applied value, and the provider cannot tell "the user
			// deleted this line" from "the API chose this value". Writing
			// `unspecified` is the supported reset, so it has to round-trip.
			{
				Config: config(`x_axis_time_format = "unspecified"`, "", `
                      editor_mode       = "unspecified"
                      series_limit_type = "unspecified"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, chart+".x_axis_time_format", utils.UNSPECIFIED),
					resource.TestCheckResourceAttr(dashboardResourceName, metrics+".editor_mode", utils.UNSPECIFIED),
					resource.TestCheckResourceAttr(dashboardResourceName, metrics+".series_limit_type", utils.UNSPECIFIED),
				),
			},
		},
	})
}

// The bar chart and horizontal bar chart share their query models, so the metrics
// and spans query fields are checked on the bar chart alone. The last step removes
// every optional field: an API that echoed a value for an omitted one would leave a
// permanent diff.
func TestAccCoralogixResourceDashboardBarChartParityFields(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	config := func(barFields, horizontalFields, metricsFields string) string {
		return fmt.Sprintf(`
resource "coralogix_dashboard" "test" {
  name        = %q
  description = "Testing the bar chart display and query fields"
  time_frame = {
    relative = {
      duration = "seconds:900"
    }
  }

  layout = {
    sections = [{
      rows = [{
        height = 19
        widgets = [
          {
            title = "bar chart"
            definition = {
              bar_chart = {
                query = {
                  metrics = {
                    promql_query = "up"
                    %[4]s
                  }
                }
                %[2]s
              }
            }
          },
          {
            title = "horizontal bar chart"
            definition = {
              horizontal_bar_chart = {
                query = {
                  spans = {
                    aggregation = { type = "metric", aggregation_type = "avg", field = "duration" }
                    group_names_fields = [{
                      keypath = ["service", "name"]
                      scope   = "metadata"
                    }]
                    stacked_group_name_field = {
                      keypath = ["operation", "name"]
                      scope   = "metadata"
                    }
                  }
                }
                %[3]s
              }
            }
          },
        ]
      }]
    }]
  }
}
`, name, barFields, horizontalFields, metricsFields)
	}

	const bar = "layout.sections.0.rows.0.widgets.0.definition.bar_chart"
	const horizontal = "layout.sections.0.rows.0.widgets.1.definition.horizontal_bar_chart"

	setChecks := resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
		resource.TestCheckResourceAttr(dashboardResourceName, bar+".bar_value_display", "top"),
		resource.TestCheckResourceAttr(dashboardResourceName, bar+".custom_unit", "runs"),
		resource.TestCheckResourceAttr(dashboardResourceName, bar+".decimal", "2"),
		resource.TestCheckResourceAttr(dashboardResourceName, bar+".decimal_precision", "true"),
		resource.TestCheckResourceAttr(dashboardResourceName, bar+".x_axis_time_format", "hh_mm"),
		resource.TestCheckResourceAttr(dashboardResourceName, bar+".y_axis_max", "100"),
		resource.TestCheckResourceAttr(dashboardResourceName, bar+".y_axis_min", "0"),
		resource.TestCheckResourceAttr(dashboardResourceName, bar+".legend.is_visible", "true"),
		resource.TestCheckResourceAttr(dashboardResourceName, bar+".query.metrics.aggregation", "avg"),
		resource.TestCheckResourceAttr(dashboardResourceName, bar+".query.metrics.editor_mode", "text"),
		resource.TestCheckResourceAttr(dashboardResourceName, bar+".query.metrics.promql_query_type", "instant"),
		resource.TestCheckResourceAttr(dashboardResourceName, horizontal+".custom_unit", "errors"),
		resource.TestCheckResourceAttr(dashboardResourceName, horizontal+".decimal", "1"),
		resource.TestCheckResourceAttr(dashboardResourceName, horizontal+".decimal_precision", "true"),
		resource.TestCheckResourceAttr(dashboardResourceName, horizontal+".y_axis_max", "50"),
		resource.TestCheckResourceAttr(dashboardResourceName, horizontal+".y_axis_min", "0"),
		resource.TestCheckResourceAttr(dashboardResourceName, horizontal+".legend.is_visible", "false"),
		resource.TestCheckResourceAttr(dashboardResourceName, horizontal+".query.spans.group_names_fields.0.scope", "metadata"),
		resource.TestCheckResourceAttr(dashboardResourceName, horizontal+".query.spans.stacked_group_name_field.scope", "metadata"),
	)
	unsetChecks := resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
		// The new enums are Optional+Computed with no static default, because the
		// API supplies a value when the attribute is omitted. Terraform keeps the
		// prior value rather than reverting it, which is what makes the plan empty.
		resource.TestCheckResourceAttr(dashboardResourceName, bar+".bar_value_display", "top"),
		resource.TestCheckNoResourceAttr(dashboardResourceName, bar+".custom_unit"),
		resource.TestCheckNoResourceAttr(dashboardResourceName, bar+".decimal"),
		resource.TestCheckNoResourceAttr(dashboardResourceName, bar+".decimal_precision"),
		resource.TestCheckResourceAttr(dashboardResourceName, bar+".x_axis_time_format", "hh_mm"),
		resource.TestCheckNoResourceAttr(dashboardResourceName, bar+".y_axis_max"),
		resource.TestCheckNoResourceAttr(dashboardResourceName, bar+".y_axis_min"),
		resource.TestCheckResourceAttr(dashboardResourceName, bar+".query.metrics.aggregation", "avg"),
		resource.TestCheckResourceAttr(dashboardResourceName, bar+".query.metrics.editor_mode", "text"),
		resource.TestCheckResourceAttr(dashboardResourceName, bar+".query.metrics.promql_query_type", "instant"),
		resource.TestCheckNoResourceAttr(dashboardResourceName, horizontal+".custom_unit"),
		resource.TestCheckNoResourceAttr(dashboardResourceName, horizontal+".decimal"),
		resource.TestCheckNoResourceAttr(dashboardResourceName, horizontal+".decimal_precision"),
		resource.TestCheckNoResourceAttr(dashboardResourceName, horizontal+".y_axis_max"),
		resource.TestCheckNoResourceAttr(dashboardResourceName, horizontal+".y_axis_min"),
	)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: config(`
                bar_value_display  = "top"
                unit               = "custom"
                custom_unit        = "runs"
                decimal            = 2
                decimal_precision  = true
                x_axis_time_format = "hh_mm"
                y_axis_max         = 100
                y_axis_min         = 0
                legend = {
                  is_visible = true
                  columns    = ["sum"]
                }`,
					`
                unit              = "custom"
                custom_unit       = "errors"
                decimal           = 1
                decimal_precision = true
                y_axis_max        = 50
                y_axis_min        = 0
                legend = {
                  is_visible = false
                }`,
					`
                    aggregation       = "avg"
                    editor_mode       = "text"
                    promql_query_type = "instant"`),
				Check: setChecks,
			},
			testAccDashboardImportStep(),
			{
				Config: config("", "", ""),
				Check:  unsetChecks,
			},
		},
	})
}

func TestAccCoralogixResourceDashboardPieChartParityFields(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	config := func(pieFields, metricsFields string) string {
		return fmt.Sprintf(`
resource "coralogix_dashboard" "test" {
  name        = %q
  description = "Testing the pie chart display and query fields"
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
          title = "pie chart"
          definition = {
            pie_chart = {
              query = {
                metrics = {
                  # The API rejects a pie chart metrics query without group_names,
                  # unlike the bar chart, which accepts one.
                  promql_query = "up"
                  group_names  = ["coralogix.metadata.applicationName"]
                  %[3]s
                }
              }
              label_definition = {}
              %[2]s
            }
          }
        }]
      }]
    }]
  }
}
`, name, pieFields, metricsFields)
	}

	const pie = "layout.sections.0.rows.0.widgets.0.definition.pie_chart"

	setChecks := resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
		resource.TestCheckResourceAttr(dashboardResourceName, pie+".custom_unit", "runs"),
		resource.TestCheckResourceAttr(dashboardResourceName, pie+".decimal", "1"),
		resource.TestCheckResourceAttr(dashboardResourceName, pie+".decimal_precision", "true"),
		resource.TestCheckResourceAttr(dashboardResourceName, pie+".show_total", "true"),
		resource.TestCheckResourceAttr(dashboardResourceName, pie+".query.metrics.aggregation", "sum"),
		resource.TestCheckResourceAttr(dashboardResourceName, pie+".query.metrics.editor_mode", "builder"),
		resource.TestCheckResourceAttr(dashboardResourceName, pie+".query.metrics.promql_query_type", "range"),
	)
	unsetChecks := resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
		resource.TestCheckNoResourceAttr(dashboardResourceName, pie+".custom_unit"),
		resource.TestCheckNoResourceAttr(dashboardResourceName, pie+".decimal"),
		resource.TestCheckNoResourceAttr(dashboardResourceName, pie+".decimal_precision"),
		resource.TestCheckNoResourceAttr(dashboardResourceName, pie+".show_total"),
		// The new enums are Optional+Computed with no static default, because the
		// API supplies a value when the attribute is omitted. Terraform keeps the
		// prior value rather than reverting it, which is what makes the plan empty.
		resource.TestCheckResourceAttr(dashboardResourceName, pie+".query.metrics.aggregation", "sum"),
		resource.TestCheckResourceAttr(dashboardResourceName, pie+".query.metrics.editor_mode", "builder"),
		resource.TestCheckResourceAttr(dashboardResourceName, pie+".query.metrics.promql_query_type", "range"),
	)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: config(`
              unit              = "custom"
              custom_unit       = "runs"
              decimal           = 1
              decimal_precision = true
              show_total        = true`,
					`
                  aggregation       = "sum"
                  editor_mode       = "builder"
                  promql_query_type = "range"`),
				Check: setChecks,
			},
			testAccDashboardImportStep(),
			{
				Config: config("", ""),
				Check:  unsetChecks,
			},
		},
	})
}

func TestAccCoralogixResourceDashboardGaugeParityFields(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	config := func(gaugeFields, metricsFields string) string {
		return fmt.Sprintf(`
resource "coralogix_dashboard" "test" {
  name        = %q
  description = "Testing the gauge display and query fields"
  time_frame = {
    relative = {
      duration = "seconds:900"
    }
  }

  layout = {
    sections = [{
      rows = [{
        height = 19
        widgets = [
          {
            title = "metrics gauge"
            definition = {
              gauge = {
                unit = "custom"
                query = {
                  metrics = {
                    promql_query = "vector(1)"
                    %[3]s
                  }
                }
                %[2]s
              }
            }
          },
          {
            title = "spans gauge"
            definition = {
              gauge = {
                unit = "none"
                query = {
                  spans = {
                    spans_aggregation = { type = "metric", aggregation_type = "avg", field = "duration" }
                    group_bys = [{
                      keypath = ["service", "name"]
                      scope   = "metadata"
                    }]
                  }
                }
              }
            }
          },
        ]
      }]
    }]
  }
}
`, name, gaugeFields, metricsFields)
	}

	const gauge = "layout.sections.0.rows.0.widgets.0.definition.gauge"
	const spansGauge = "layout.sections.0.rows.0.widgets.1.definition.gauge"

	setChecks := resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
		resource.TestCheckResourceAttr(dashboardResourceName, gauge+".custom_unit", "ms"),
		resource.TestCheckResourceAttr(dashboardResourceName, gauge+".show_min_max", "true"),
		resource.TestCheckResourceAttr(dashboardResourceName, gauge+".legend_by", "thresholds"),
		resource.TestCheckResourceAttr(dashboardResourceName, gauge+".legend.is_visible", "true"),
		resource.TestCheckResourceAttr(dashboardResourceName, gauge+".query.metrics.editor_mode", "text"),
		resource.TestCheckResourceAttr(dashboardResourceName, gauge+".query.metrics.promql_query_type", "instant"),
		resource.TestCheckResourceAttr(dashboardResourceName, spansGauge+".query.spans.group_bys.0.scope", "metadata"),
	)
	unsetChecks := resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
		resource.TestCheckNoResourceAttr(dashboardResourceName, gauge+".custom_unit"),
		resource.TestCheckNoResourceAttr(dashboardResourceName, gauge+".show_min_max"),
		// The new enums are Optional+Computed with no static default, because the
		// API supplies a value when the attribute is omitted. Terraform keeps the
		// prior value rather than reverting it, which is what makes the plan empty.
		resource.TestCheckResourceAttr(dashboardResourceName, gauge+".legend_by", "thresholds"),
		resource.TestCheckResourceAttr(dashboardResourceName, gauge+".query.metrics.editor_mode", "text"),
		resource.TestCheckResourceAttr(dashboardResourceName, gauge+".query.metrics.promql_query_type", "instant"),
	)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: config(`
                custom_unit  = "ms"
                show_min_max = true
                legend_by    = "thresholds"
                legend = {
                  is_visible = true
                }`,
					`
                    editor_mode       = "text"
                    promql_query_type = "instant"`),
				Check: setChecks,
			},
			testAccDashboardImportStep(),
			{
				Config: config("", ""),
				Check:  unsetChecks,
			},
		},
	})
}

func TestAccCoralogixResourceDashboardDataTableMetricsEditorMode(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	config := func(editorMode string) string {
		return fmt.Sprintf(`
resource "coralogix_dashboard" "test" {
  name        = %q
  description = "Testing the data table metrics editor mode"
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
          title = "data table"
          definition = {
            data_table = {
              results_per_page = 10
              row_style        = "one_line"
              query = {
                metrics = {
                  promql_query = "up"
                  %[2]s
                }
              }
            }
          }
        }]
      }]
    }]
  }
}
`, name, editorMode)
	}

	const editorMode = "layout.sections.0.rows.0.widgets.0.definition.data_table.query.metrics.editor_mode"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: config(`editor_mode = "builder"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
					resource.TestCheckResourceAttr(dashboardResourceName, editorMode, "builder"),
				),
			},
			testAccDashboardImportStep(),
			{
				// editor_mode is Optional+Computed with no static default, so removing
				// it keeps the prior value instead of reverting it. That is what
				// makes the plan empty against a dashboard the API defaulted itself.
				Config: config(""),
				Check:  resource.TestCheckResourceAttr(dashboardResourceName, editorMode, "builder"),
			},
		},
	})
}

func TestAccCoralogixResourceDashboardManualAnnotation(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "coralogix_dashboard" "test" {
  name        = %q
  description = "Testing manual annotation source"
  time_frame = {
    relative = {
      duration = "seconds:900"
    }
  }

  annotations = [{
    name    = "manual threshold band"
    enabled = true
    source = {
      manual = {
        orientation = "horizontal"
        strategy = {
          range = {
            start_value = 45
            end_value   = 80
          }
        }
      }
    }
  }]

  layout = {
    sections = [{
      rows = [{
        height = 10
        widgets = [{
          title = "placeholder"
          width = 0
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
`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
					resource.TestCheckResourceAttr(dashboardResourceName, "annotations.0.name", "manual threshold band"),
					resource.TestCheckResourceAttr(dashboardResourceName, "annotations.0.source.manual.orientation", "horizontal"),
					resource.TestCheckResourceAttr(dashboardResourceName, "annotations.0.source.manual.strategy.range.start_value", "45"),
					resource.TestCheckResourceAttr(dashboardResourceName, "annotations.0.source.manual.strategy.range.end_value", "80"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			testAccDashboardImportStep(),
		},
	})
}

func TestAccCoralogixResourceDashboardDataprimeAnnotation(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "coralogix_dashboard" "test" {
  name        = %q
  description = "Testing dataprime annotation source"
  time_frame = {
    relative = {
      duration = "seconds:900"
    }
  }

  annotations = [
    {
      name    = "dataprime instant"
      enabled = true
      source = {
        dataprime = {
          query = "source logs | filter severity == 'error'"
          strategy = {
            instant = {
              timestamp_field = {
                keypath = ["timestamp"]
                scope   = "metadata"
              }
            }
          }
        }
      }
    },
    {
      name    = "dataprime range"
      enabled = true
      source = {
        dataprime = {
          query = "source logs | limit 10"
          strategy = {
            range = {
              start_timestamp_field = {
                keypath = ["start_time"]
                scope   = "metadata"
              }
              end_timestamp_field = {
                keypath = ["end_time"]
                scope   = "metadata"
              }
            }
          }
        }
      }
    },
    {
      name    = "dataprime duration"
      enabled = true
      source = {
        dataprime = {
          query = "source logs | limit 5"
          strategy = {
            duration = {
              start_timestamp_field = {
                keypath = ["start_time"]
                scope   = "metadata"
              }
              duration_field = {
                keypath = ["duration_ms"]
                scope   = "metadata"
              }
            }
          }
        }
      }
    }
  ]

  layout = {
    sections = [{
      rows = [{
        height = 10
        widgets = [{
          title = "placeholder"
          width = 0
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
`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
					resource.TestCheckResourceAttr(dashboardResourceName, "annotations.0.name", "dataprime instant"),
					resource.TestCheckResourceAttr(dashboardResourceName, "annotations.0.source.dataprime.query", "source logs | filter severity == 'error'"),
					resource.TestCheckResourceAttrSet(dashboardResourceName, "annotations.0.source.dataprime.strategy.instant.timestamp_field.scope"),
					resource.TestCheckResourceAttr(dashboardResourceName, "annotations.1.name", "dataprime range"),
					resource.TestCheckResourceAttr(dashboardResourceName, "annotations.1.source.dataprime.strategy.range.start_timestamp_field.keypath.0", "start_time"),
					resource.TestCheckResourceAttr(dashboardResourceName, "annotations.2.name", "dataprime duration"),
					resource.TestCheckResourceAttr(dashboardResourceName, "annotations.2.source.dataprime.strategy.duration.duration_field.keypath.0", "duration_ms"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			testAccDashboardImportStep(),
		},
	})
}

func TestAccCoralogixResourceDashboardEventRecurrenceAnnotation(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "coralogix_dashboard" "test" {
  name        = %q
  description = "Testing event recurrence annotation source"
  time_frame = {
    relative = {
      duration = "seconds:900"
    }
  }

  annotations = [
    {
      name    = "weekly deployment instant"
      enabled = true
      source = {
        event_recurrence = {
          message_template = "Weekly deployment window"
          recurrence = {
            weekly = {
              days_of_week = ["tuesday", "thursday"]
            }
          }
          strategy = {
            instant = {
              start_time_hour = 9
            }
          }
        }
      }
    },
    {
      name    = "weekly maintenance window"
      enabled = true
      source = {
        event_recurrence = {
          recurrence = {
            weekly = {
              days_of_week = ["sunday"]
            }
          }
          strategy = {
            duration = {
              start_time_hour = 2
              duration        = "7200s"
            }
          }
        }
      }
    }
  ]

  layout = {
    sections = [{
      rows = [{
        height = 10
        widgets = [{
          title = "placeholder"
          width = 0
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
`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
					resource.TestCheckResourceAttr(dashboardResourceName, "annotations.0.name", "weekly deployment instant"),
					resource.TestCheckResourceAttr(dashboardResourceName, "annotations.0.source.event_recurrence.message_template", "Weekly deployment window"),
					resource.TestCheckResourceAttr(dashboardResourceName, "annotations.0.source.event_recurrence.recurrence.weekly.days_of_week.0", "tuesday"),
					resource.TestCheckResourceAttr(dashboardResourceName, "annotations.0.source.event_recurrence.strategy.instant.start_time_hour", "9"),
					resource.TestCheckResourceAttr(dashboardResourceName, "annotations.1.name", "weekly maintenance window"),
					resource.TestCheckResourceAttr(dashboardResourceName, "annotations.1.source.event_recurrence.strategy.duration.duration", "7200s"),
					resource.TestCheckResourceAttr(dashboardResourceName, "annotations.1.source.event_recurrence.strategy.duration.start_time_hour", "2"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			testAccDashboardImportStep(),
		},
	})
}

func TestAccCoralogixResourceDashboardAnnotationScope(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "coralogix_dashboard" "test" {
  name        = %q
  description = "Testing annotation scope"
  time_frame = {
    relative = {
      duration = "seconds:900"
    }
  }

  annotations = [{
    name    = "scoped annotation"
    enabled = true
    source = {
      manual = {
        orientation = "horizontal"
        strategy = {
          range = {
            start_value = 10
            end_value   = 90
          }
        }
      }
    }
    scope = {
      all_widgets = {}
    }
  }]

  layout = {
    sections = [{
      rows = [{
        height = 10
        widgets = [{
          title = "placeholder"
          width = 0
          definition = {
            line_chart = {
              query_definitions = [{
                query = {
                  logs = {
                    aggregations = [{ type = "count" }]
                  }
                }
              }]
              legend = { is_visible = false }
            }
          }
        }]
      }]
    }]
  }
}
`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
					resource.TestCheckResourceAttr(dashboardResourceName, "annotations.0.name", "scoped annotation"),
					resource.TestCheckResourceAttrSet(dashboardResourceName, "annotations.0.scope.all_widgets.%"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: fmt.Sprintf(`
resource "coralogix_dashboard" "test" {
  name        = %q
  description = "Testing annotation scope"
  time_frame = {
    relative = {
      duration = "seconds:900"
    }
  }

  annotations = [{
    name    = "scoped annotation"
    enabled = true
    source = {
      manual = {
        orientation = "horizontal"
        strategy = {
          range = {
            start_value = 10
            end_value   = 90
          }
        }
      }
    }
  }]

  layout = {
    sections = [{
      rows = [{
        height = 10
        widgets = [{
          title = "placeholder"
          width = 0
          definition = {
            line_chart = {
              query_definitions = [{
                query = {
                  logs = {
                    aggregations = [{ type = "count" }]
                  }
                }
              }]
              legend = { is_visible = false }
            }
          }
        }]
      }]
    }]
  }
}
`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dashboardResourceName, "id"),
					resource.TestCheckResourceAttr(dashboardResourceName, "annotations.0.name", "scoped annotation"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			testAccDashboardImportStep(),
		},
	})
}
