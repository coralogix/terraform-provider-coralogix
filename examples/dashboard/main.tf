terraform {
  required_providers {
    coralogix = {
      version = "~> 1.5"
      source  = "coralogix/coralogix"
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
  layout      = {
    sections = [
      {
        rows = [
          {
            height  = 22
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
                      columns    = ["max", "last"]
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
                              }
                            ]
                          }
                        }
                      },
                    ]
                  }
                  legend = {
                    is_visible = true
                    columns    = ["Min", "Max", "Sum", "Avg", "Last"]
                  }
                }
                width = 0
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
                      columns    = ["max", "last"]
                    }
                  }
                }
                width = 0
              },
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
                      }
                    ]
                  }
                }
                width = 0
              }
            ]
          }
        ]
      },
    ]
  }
  variables = [
    {
      name       = "test_variable"
      definition = {
        multi_select = {
          selected_values = ["1", "2", "3"]
          source          = {
            constant_list = ["1", "2", "3"]
          }
        }
      }
    },
  ]
  filters = [
    {
      source = {
        metrics = {
          metric_name = "http_requests_total"
          label       = "status"
          field       = "coralogix.metadata.applicationName"
          operator    = {
            type            = "equals"
            selected_values = ["staging"]
          }
        }
      }
    },
  ]
}

resource "coralogix_dashboard" test {
  name        = "test"
  description = "dashboards team is messing with this 🗿"
  layout      = {
    sections = [
      {
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
                      columns    = ["Max", "Last"]
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
                  }
                  legend = {
                    is_visible = true
                    columns    = ["Min", "Max", "Sum", "Avg", "Last"]
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
                      columns    = ["Max", "Last"]
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
      name       = "test_variable"
      definition = {
        multi_select = {
          selected_values = ["1", "2", "3"]
          source          = {
            constant_list = ["1", "2", "3"]
          }
        }
      }
    },
  ]
}

resource "coralogix_dashboard" dashboard_from_json {
  content_json = file("./dashboard.json")
}

resource "coralogix_dashboard" dashboard_2 {
  name        = "portal monitoring"
  description = "<insert description>"
  layout      = {
    sections = [
      {
        rows = [
          {
            height  = 15
            widgets = [
              {
                title      = "Avg api response times"
                definition = {
                  line_chart = {
                    query_definitions = [
                      {
                        query = {
                          logs = {
                            lucene_query = "kubernetes.namespace_name:\"hunting-portal\" AND kubernetes.pod_name.keyword:/hunters-api-deployment.*/ AND message:\"HTTP\" AND NOT \"OPTIONS\" AND NOT \"metrics\" AND NOT \"firebase\""
                            aggregations = [
                              {
                                type  = "avg"
                                field = "meta.responseTime.numeric"
                              },
                              {
                                type  = "max"
                                field = "meta.responseTime.numeric"
                              },
                            ]

                            group_by = [
                              "meta.organization.keyword"
                            ]
                          }
                        }
                        scale_type         = "linear"
                        series_count_limit = 100
                        unit               = "milliseconds"
                      },
                    ]
                    legend = {
                      is_visible = true
                      columns    = ["Avg", "Max"]
                    }
                    tooltip = {
                      show_labels = false
                      type        = "all"
                    }
                  }
                }
                width = 0
              },
              {
                title      = "Avg Snowflake query times"
                definition = {
                  line_chart = {
                    query_definitions = [
                      {
                        query = {
                          logs = {
                            lucene_query = "kubernetes.namespace_name:\"hunting-portal\" AND \"Successfully executed\""
                            aggregations = [
                              {
                                type  = "avg"
                                field = "sfResponseTime.numeric"
                              },
                            ]
                            group_by = [
                              "sfDatabase.keyword"
                            ]
                          }
                        }
                        scale_type         = "linear"
                        series_count_limit = 100
                        unit               = "milliseconds"
                      },
                    ]
                    legend = {
                      is_visible = true
                      columns    = ["Avg"]
                    }
                    tooltip = {
                      show_labels = false
                      type        = "all"
                    }
                  }
                }
                width = 0
              },
              {
                title      = "Avg RDS query times"
                definition = {
                  line_chart = {
                    query_definitions = [
                      {
                        query = {
                          logs = {
                            lucene_query = "kubernetes.namespace_name:\"hunting-portal\" AND kubernetes.pod_name.keyword:/hunters-api-deployment.*/ AND \"Postgres successfully\""
                            aggregations = [
                              {
                                type  = "avg"
                                field = "RDSResponseTime.numeric"
                              },
                            ]
                            group_by = [
                              "RDSDatabase.keyword"
                            ]
                          }
                        }
                        scale_type         = "linear"
                        series_count_limit = 100
                        unit               = "milliseconds"
                      },
                    ]
                    legend = {
                      is_visible = true
                      columns    = ["Avg"]
                    }
                    tooltip = {
                      show_labels = false
                      type        = "all"
                    }
                  }
                }
                width = 0
              },
            ]
          },
          {
            height  = 15
            widgets = [
              {
                title      = "OpenAPI - Avg response times"
                definition = {
                  line_chart = {
                    query_definitions = [
                      {
                        query = {
                          logs = {
                            lucene_query = "kubernetes.namespace_name:\"hunting-portal\" AND kubernetes.pod_name.keyword:/hunters-openapi-deployment.*/ AND message:\"HTTP\" AND NOT \"OPTIONS\" AND NOT \"metrics\" AND NOT \"firebase\""
                            aggregations = [
                              {
                                type  = "avg"
                                field = "meta.responseTime.numeric"
                              },
                            ]
                            group_by = [
                              "meta.organization.keyword"
                            ]
                          }
                        }
                        scale_type         = "linear"
                        series_count_limit = 100
                        unit               = "milliseconds"
                      },
                    ]
                    legend = {
                      is_visible = true
                      columns    = ["Avg", "Max"]
                    }
                    tooltip = {
                      show_labels = false
                      type        = "all"
                    }
                  }
                }
                width = 0
              },
            ]
          },
          {
            height  = 15
            widgets = [
              {
                title      = "Open API Requests per organization"
                definition = {
                  line_chart = {
                    query_definitions = [
                      {
                        query = {
                          logs = {
                            lucene_query = "kubernetes.namespace_name:\"hunting-portal\" AND (service:\"api.eu.hunters.ai-production\" OR service:\"api.us.hunters.ai-production\")"
                            aggregations = [
                              {
                                type = "count"
                              },
                            ]
                            group_by = [
                              "meta.organization.keyword"
                            ]
                          }
                        }
                        scale_type         = "linear"
                        series_count_limit = 100
                      },
                    ]
                    legend = {
                      is_visible = true
                      columns    = []
                    }
                    tooltip = {
                      show_labels = false
                      type        = "all"
                    }
                  }
                }
                width = 0
              },
              {
                title      = "Last failed SF queries DBs"
                definition = {
                  line_chart = {
                    query_definitions = [
                      {
                        query = {
                          logs = {
                            lucene_query = "kubernetes.namespace_name:\"hunting-portal\" AND \"Failed to execute statement\""
                            aggregations = [
                              {
                                type = "count"
                              }
                            ]
                            group_by = [
                              "sfDatabase.keyword"
                            ]
                          }
                        }
                        scale_type         = "linear"
                        series_count_limit = 100
                      },
                    ]
                    legend = {
                      is_visible = true
                      columns    = []
                    }
                    tooltip = {
                      show_labels = false
                      type        = "all"
                    }
                  }
                }
                width = 0
              },
              {
                title      = "Avg configuration service query times"
                definition = {
                  line_chart = {
                    query_definitions = [
                      {
                        query = {
                          logs = {
                            lucene_query = "kubernetes.namespace_name:\"hunting-portal\" AND kubernetes.pod_name.keyword:/hunters-api-deployment.*/ AND \"Configuration Service request\""
                            aggregations = [
                              {
                                type  = "avg"
                                field = "configResponseTime.numeric"
                              },
                            ]
                          }
                        }
                        scale_type         = "linear"
                        series_count_limit = 100
                      },
                    ]
                    legend = {
                      is_visible = false
                    }
                    tooltip = {
                      show_labels = false
                      type        = "all"
                    }
                  }
                }
                width = 0
              },
            ]
            height = 15
          },
          # widget {
          #     title = "Last failed API requests"
          #     description = "TBD"
          #     appearance {
          #         width = 0
          #     }
          #   definition {
          #     data_table{
          #         results_per_page = 100
          #         row_style        = "One_Line"
          #         query {
          #                 logs {
          #                     lucene_query = " kubernetes.namespace_name:\"hunting-portal\" AND kubernetes.pod_name.keyword:/hunters-api-deployment.*/ AND message:\"http\" AND meta.res.statusCode:500"
          #                 }
          #         }
          #         column{
          #             field = "GroupBy:meta.req.url"
          #             width = 0
          #         }
          #         column{
          #             field = "Aggregation:543c4bc5-484b-9d70-7b3d-e8f44381baf9"
          #             width = 0
          #         }
          #         order_by{
          #             field = "meta.req.url"
          #             order_direction = "Desc"
          #         }
          #     }
          #                 }
          # }
          # widget {
          #     title = "Last Failed RDS queries DBs"
          #     appearance {
          #     width = 0
          #   }
          #   definition {
          #     # this should be a pie chart!!!
          #                 }
          # }
          # widget {
          #     title = "Alerts notification count per org"
          #     appearance {
          #     width = 0
          #   }
          #   definition {
          #     # this should be a pie chart!!!
          #                 }
          # }
          #          },
          {
            height  = 19
            widgets = [
              {
                title      = "Slowest API requests"
                definition = {
                  line_chart = {
                    query_definitions = [
                      {
                        query = {
                          logs = {
                            lucene_query = " kubernetes.namespace_name:\"hunting-portal\" AND kubernetes.pod_name.keyword:/hunters-api-deployment.*/ AND message:\"http\""
                            aggregations = [
                              {
                                type  = "max"
                                field = "meta.responseTime.numeric"
                              },
                            ]
                            group_by = [
                              "meta.req.url.keyword"
                            ]
                          }
                        }
                        scale_type         = "linear"
                        series_count_limit = 10
                        unit               = "milliseconds"
                      },
                    ]
                    legend = {
                      is_visible = true
                      columns    = ["Max"]
                    }
                    tooltip = {
                      show_labels = false
                      type        = "all"
                    }
                  }
                }
                width = 0
              },
            ]
          },
          {
            height  = 19
            widgets = [
              {
                title      = "Cache warmer runs"
                definition = {
                  line_chart = {
                    query_definitions = [
                      {
                        query = {
                          logs = {
                            lucene_query = "kubernetes.namespace_name:\"hunting-portal\" AND kubernetes.container_name:\"hunters-portal-cache-warmer\" AND message:\"Finish cache warmer run successfully\""
                            aggregations = [
                              {
                                type = "count"
                              },
                            ]
                          }
                        }
                        scale_type         = "linear"
                        series_count_limit = 20
                      },
                    ]
                    legend = {
                      is_visible = true
                    }
                    tooltip = {
                      show_labels = false
                      type        = "all"
                    }
                  }
                }
                width = 0
              },
              {
                title      = "Alerts notification eu runs"
                definition = {
                  line_chart = {
                    query_definitions = [
                      {
                        query = {
                          logs = {
                            lucene_query = "service:\"hunters-portal-eu-notify-alerts-production\" AND \"Finished notify new alerts\""
                            aggregations = [
                              {
                                type = "count"
                              },
                            ]
                          }
                        }
                        scale_type         = "linear"
                        series_count_limit = 20
                      },
                    ]
                    legend = {
                      is_visible = true
                    }
                    tooltip = {
                      show_labels = false
                      type        = "all"
                    }
                  }
                }
                width = 0
              },
              {
                title      = "Alerts notification runs"
                definition = {
                  line_chart = {
                    query_definitions = [
                      {
                        query = {
                          logs = {
                            lucene_query = "service:\"hunters-portal-notify-alerts-production\" AND \"Finished notify new alerts\""
                            aggregations = [
                              {
                                type = "count"
                              },
                            ]
                          }
                        }
                      },
                    ]
                    scale_type         = "linear"
                    series_count_limit = 20
                  }
                  legend = {
                    is_visible = true
                  }
                  tooltip = {
                    show_labels = false
                    type        = "all"
                  }
                }
                width = 0
              },
            ]
          },
        ]
      },
    ]
  }
}