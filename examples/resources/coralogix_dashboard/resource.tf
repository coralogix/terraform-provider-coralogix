terraform {
  required_providers {
    coralogix = {
      version = "~> 1.8"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_dashboard" dashboard {
  name        = "portal monitoring"
  description = "<insert description>"
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
                            lucene_query = "kubernetes.namespace_name:\"portal\" AND kubernetes.pod_name.keyword:/api-deployment.*/ AND message:\"HTTP\" AND NOT \"OPTIONS\" AND NOT \"metrics\" AND NOT \"firebase\""
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
                        resolution         = {
                          interval = "seconds:900"
                        }
                      },
                    ]
                    legend = {
                      is_visible = true
                      columns    = ["avg", "max"]
                    }
                    tooltip = {
                      show_labels = false
                      type        = "all"
                    }
                  }
                }
              },
              {
                title      = "Avg Snowflake query times"
                definition = {
                  line_chart = {
                    query_definitions = [
                      {
                        query = {
                          logs = {
                            lucene_query = "kubernetes.namespace_name:\"portal\" AND \"Successfully executed\""
                            aggregations = [
                              {
                                type    = "percentile"
                                field   = "sfResponseTime.numeric"
                                percent = 95.5
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
                      columns    = ["avg"]
                    }
                    tooltip = {
                      show_labels = false
                      type        = "all"
                    }
                  }
                }
              },
              {
                title      = "Avg RDS query times"
                definition = {
                  line_chart = {
                    query_definitions = [
                      {
                        query = {
                          logs = {
                            lucene_query = "kubernetes.namespace_name:\"portal\" AND kubernetes.pod_name.keyword:/api-deployment.*/ AND \"Postgres successfully\""
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
                        resolution         = {
                          buckets_presented = 10
                        }
                      },
                    ]
                    legend = {
                      is_visible = true
                      columns    = ["avg"]
                    }
                    tooltip = {
                      show_labels = false
                      type        = "all"
                    }
                  }
                }
                width = 10
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
                            lucene_query = "kubernetes.namespace_name:\"portal\" AND kubernetes.pod_name.keyword:/openapi-deployment.*/ AND message:\"HTTP\" AND NOT \"OPTIONS\" AND NOT \"metrics\" AND NOT \"firebase\""
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
                      columns    = ["avg", "max"]
                    }
                    tooltip = {
                      show_labels = false
                      type        = "all"
                    }
                  }
                }
                width = 10
              },
              {
                title      = "gauge"
                definition = {
                  gauge = {
                    unit  = "milliseconds"
                    query = {
                      metrics = {
                        promql_query = "vector(1)"
                        aggregation  = "unspecified"
                      }
                    }
                  }
                }
              }
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
                            lucene_query = "kubernetes.namespace_name:\"portal\" AND (service:\"api.eu.name.ai-production\" OR service:\"api.us.name.ai-production\")"
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
                            lucene_query = "kubernetes.namespace_name:\"portal\" AND \"Failed to execute statement\""
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
                            lucene_query = "kubernetes.namespace_name:\"portal\" AND kubernetes.pod_name.keyword:/api-deployment.*/ AND \"Configuration Service request\""
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
              },
            ]
            height = 15
          },
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
                            lucene_query = " kubernetes.namespace_name:\"portal\" AND kubernetes.pod_name.keyword:/api-deployment.*/ AND message:\"http\""
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
                      columns    = ["max"]
                    }
                    tooltip = {
                      show_labels = false
                      type        = "all"
                    }
                  }
                }
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
                            lucene_query = "kubernetes.namespace_name:\"portal\" AND kubernetes.container_name:\"portal-cache-warmer\" AND message:\"Finish cache warmer run successfully\""
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
              },
              {
                title      = "Alerts notification eu runs"
                definition = {
                  line_chart = {
                    query_definitions = [
                      {
                        query = {
                          logs = {
                            lucene_query = "service:\"portal-eu-notify-alerts-production\" AND \"Finished notify new alerts\""
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
              },
              {
                title      = "Alerts notification runs"
                definition = {
                  line_chart = {
                    query_definitions = [
                      {
                        query = {
                          logs = {
                            lucene_query = "service:\"portal-notify-alerts-production\" AND \"Finished notify new alerts\""
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
              },
              {
                title      = "Alerts notification us runs"
                definition = {
                  pie_chart = {
                    query = {
                      logs = {
                        lucene_query = "service:\"portal-us-notify-alerts-production\" AND \"Finished notify new alerts\""
                        aggregation  = {
                          type = "count"
                        }
                        group_names = [
                          "service.keyword"
                        ]
                      }
                    }
                    label_definition = {
                    }
                  }
                }
              },
              {
                title      = "Alerts notification us runs"
                definition = {
                  bar_chart = {
                    query = {
                      logs = {
                        lucene_query = "service:\"portal-us-notify-alerts-production\" AND \"Finished notify new alerts\""
                        aggregation  = {
                          type = "count"
                        }
                        group_names_fields = [
                          {
                            keypath = ["logid"]
                            scope   = "metadata"
                          },
                        ]
                        stacked_group_name_field = {
                          keypath = ["logid"]
                          scope   = "metadata"
                        }
                      }
                    }
                    xaxis = {
                      time = {
                        interval          = "1h0m5s"
                        buckets_presented = 10
                      }
                    }
                  }
                }
              },
              {
                title      = "Horizontal Bar-Chart"
                definition = {
                  horizontal_bar_chart = {
                    color_scheme   = "cold"
                    colors_by      = "aggregation"
                    display_on_bar = true
                    query          = {
                      logs = {
                        lucene_query = "service:\"portal-us-notify-alerts-production\" AND \"Finished notify new alerts\""
                        aggregation  = {
                          type = "count"
                        }
                        group_names        = ["coralogix.logId.keyword"]
                        stacked_group_name = "coralogix.metadata.severity"
                      }
                    }
                    y_axis_view_by = "value"
                  }
                }
              },
              {
                definition = {
                  markdown = {
                    markdown_text = "## Markdown\n\nThis is a markdown widget"
                    tooltip_text  = "This is a tooltip"
                  }
                }
              },
              {
                title      = "Data Table"
                definition = {
                  data_table = {
                    results_per_page = 10
                    row_style        = "one_line"
                    query            = {
                      data_prime = {
                        query   = "xxx"
                        filters = [
                          {
                            logs = {
                              lucene_query = "service:\"portal-us-notify-alerts-production\" AND \"Finished notify new alerts\""
                              aggregation  = {
                                type = "count"
                              }
                              group_names        = ["coralogix.logId.keyword"]
                              stacked_group_name = "coralogix.metadata.severity"
                              field              = "coralogix.metadata.applicationName"
                              operator           = {
                                type            = "equals"
                                selected_values = ["staging"]
                              }
                            }
                          },
                        ]
                      }
                    }
                  }
                }
              },
            ]
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
            query ={
              query = {
                metrics = {
                  metric_name = {
                    metric_regex = "vector(1)"
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
  annotations = [
    {
      name   = "test_annotation"
      source = {
        metrics = {
          promql_query = "vector(1)"
          strategy     = {
            start_time = {}
          }
          message_template = "test annotation"
          labels           = ["test"]
        }
      }
    },
  ]
  auto_refresh = {
    type = "two_minutes"
  }
  folder = {
    id = coralogix_dashboards_folder.example.id
  }
}

resource "coralogix_dashboards_folder" "example" {
  name     = "example"
}

resource "coralogix_dashboard" dashboard_from_json {
  content_json = file("./dashboard.json")
}