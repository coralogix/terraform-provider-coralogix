terraform {
  required_providers {
    coralogix = {
      version = "~> 1.3"
      source  = "locally/debug/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_dashboard" dashboard {
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
                order_direction = "Unspecified"
              }
              columns {
                field           = "textObject.textObject.textObject.kubernetes.pod_id"
                order_direction = "Unspecified"
              }
              columns {
                field           = "coralogix.text"
                order_direction = "Unspecified"
              }
              columns {
                field           = "coralogix.metadata.applicationName"
                order_direction = "Unspecified"
              }
              columns {
                field           = "coralogix.metadata.subsystemName"
                order_direction = "Unspecified"
              }
              columns {
                field           = "coralogix.metadata.sdkId"
                order_direction = "Unspecified"
              }
              columns {
                field           = "textObject.textObject.textObject.log_obj.e2e_test.config"
                order_direction = "Unspecified"
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
}

resource "coralogix_dashboard" dashboard_from_json {
  name = "dont drop me!"
  description = "dashboards team is messing with this 🗿"
  layout_json = file("./dashboard.json")
}

