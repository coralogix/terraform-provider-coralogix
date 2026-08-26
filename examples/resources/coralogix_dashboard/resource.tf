terraform {
  required_providers {
    coralogix = {
      version = "~> 3.0"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_dashboard" "dashboard" {
  name        = "portal monitoring"
  description = "<insert description>"
  access_policy = jsonencode({
    version = "2025-01-01"
    default = {
      permissions = {
        "team-dashboards:Read"               = "grant"
        "team-dashboards:ReadAccessPolicy"   = "grant"
        "team-dashboards:Update"             = "grant"
        "team-dashboards:UpdateAccessPolicy" = "grant"
      }
    }
    rules = []
  })
  layout = {
    sections = [
      {
        options = {
          name        = "Status"
          description = "abc"
          collapsed   = false
          color       = "blue"
        }
        rows = [
          {
            height = 15
            widgets = [
              {
                title = "Avg api response times"
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
                        hash_colors        = true
                        resolution = {
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
                title = "Avg Snowflake query times"
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
                title = "Avg RDS query times"
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
                        resolution = {
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
              },
            ]
          },
          {
            height = 15
            widgets = [
              {
                title = "OpenAPI - Avg response times"
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
              },
              {
                title = "gauge"
                definition = {
                  gauge = {
                    unit = "milliseconds"
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
            height = 15
            widgets = [
              {
                title = "Open API Requests per organization"
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
              },
              {
                title = "Last failed SF queries DBs"
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
              },
              {
                title = "Avg configuration service query times"
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
            height = 19
            widgets = [
              {
                title = "Slowest API requests"
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
            height = 19
            widgets = [
              {
                title = "Cache warmer runs"
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
                title = "Alerts notification eu runs"
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
                title = "Alerts notification runs"
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
                title = "Alerts notification us runs"
                definition = {
                  pie_chart = {
                    query = {
                      logs = {
                        lucene_query = "service:\"portal-us-notify-alerts-production\" AND \"Finished notify new alerts\""
                        aggregation = {
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
                title = "Alerts notification us runs"
                definition = {
                  bar_chart = {
                    query = {
                      logs = {
                        lucene_query = "service:\"portal-us-notify-alerts-production\" AND \"Finished notify new alerts\""
                        aggregation = {
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
                    hash_colors = true
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
                title = "Horizontal Bar-Chart"
                definition = {
                  horizontal_bar_chart = {
                    color_scheme   = "cold"
                    colors_by      = "aggregation"
                    display_on_bar = true
                    query = {
                      logs = {
                        lucene_query = "service:\"portal-us-notify-alerts-production\" AND \"Finished notify new alerts\""
                        aggregation = {
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
                title = "Data Table"
                definition = {
                  data_table = {
                    results_per_page = 10
                    row_style        = "one_line"
                    query = {
                      data_prime = {
                        query = "xxx"
                        filters = [
                          {
                            logs = {
                              lucene_query = "service:\"portal-us-notify-alerts-production\" AND \"Finished notify new alerts\""
                              aggregation = {
                                type = "count"
                              }
                              group_names        = ["coralogix.logId.keyword"]
                              stacked_group_name = "coralogix.metadata.severity"
                              field              = "coralogix.metadata.applicationName"
                              operator = {
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
  # Prefer variables_v2 for new dashboards. Legacy `variables` remains available during migration.
  variables_v2 = [
    {
      name         = "environment"
      display_name = "Environment"
      source = {
        static = {
          values_order_direction = "none"
          all_option = {
            include_all = false
          }
          values = [
            {
              value      = "production"
              is_default = true
            },
            {
              value = "staging"
              label = "Staging"
            },
          ]
        }
      }
      value = {
        single_string = {
          value = "production"
          label = "production"
        }
      }
    },
    {
      name             = "search"
      display_name     = "Search"
      display_full_row = true
      source = {
        textbox = {
          default_value = {
            default_string_value = {
              value = "error"
            }
          }
        }
      }
      value = {
        single_string = {
          value = "error"
          label = "error"
        }
      }
    },
    {
      name         = "service"
      display_name = "Service"
      source = {
        query = {
          all_option             = { include_all = false }
          values_order_direction = "asc"
          refresh_strategy       = "on_dashboard_load"
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
      value = {
        multi_string = {
          selected_all = {}
        }
      }
    },
    {
      name         = "span_service"
      display_name = "Span service"
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
      value = {
        multi_string = {
          selected_all = {}
        }
      }
    },
    {
      name         = "metric_name"
      display_name = "Metric name"
      source = {
        query = {
          all_option = { include_all = false }
          metrics_query = {
            type = {
              metric_name = {
                metric_regex = ".*"
              }
            }
          }
        }
      }
      value = {
        multi_string = {
          selected_all = {}
        }
      }
    },
    {
      name         = "promql_values"
      display_name = "PromQL values"
      source = {
        query = {
          all_option = { include_all = false }
          metrics_query = {
            type = {
              promql_query = {
                query             = "vector(1)"
                promql_query_type = "instant"
              }
            }
          }
        }
      }
      value = {
        multi_string = {
          list = {
            values = [
              {
                value = {
                  value = "1"
                  label = "one"
                }
              },
            ]
          }
        }
      }
    },
    {
      name         = "dataprime_values"
      display_name = "DataPrime values"
      source = {
        query = {
          all_option = { include_all = false }
          dataprime_query = {
            type = {
              query_text = {
                query = "source logs | limit 10"
              }
            }
          }
        }
      }
      value = {
        multi_string = {
          selected_all = {}
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
          operator = {
            type            = "equals"
            selected_values = ["staging"]
          }
        }
      }
    },
  ]
  annotations = [
    {
      name = "test_annotation"
      source = {
        metrics = {
          promql_query = "vector(1)"
          strategy = {
            start_time = {}
          }
          message_template = "test annotation"
          labels           = ["test"]
        }
      }
    },
    {
      name = "manual_threshold_band"
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
      # scope restricts this annotation to all widgets; omit to not apply a scope.
      # Use specific_widgets = { widget_ids = ["<uuid>"] } to target individual widgets.
      scope = {
        all_widgets = {}
      }
    },
    {
      name = "dataprime_annotation"
      source = {
        dataprime = {
          query = "source logs | limit 10"
          strategy = {
            instant = {
              timestamp_field = {
                keypath = ["timestamp"]
                scope   = "metadata"
              }
            }
          }
          message_template = "dataprime event"
          orientation      = "vertical"
          data_mode_type   = "unspecified"
        }
      }
    },
    {
      name = "weekly_event_recurrence"
      source = {
        event_recurrence = {
          message_template = "weekly maintenance window"
          recurrence = {
            weekly = {
              days_of_week = ["monday", "wednesday"]
            }
          }
          strategy = {
            duration = {
              start_time_hour = 2
              duration        = "3600s"
            }
          }
        }
      }
    },
  ]
  auto_refresh = {
    type = "two_minutes"
  }
  # Recommended: reference a sibling coralogix_dashboards_folder resource via
  # folder.id so the folder's lifecycle is owned by Terraform. The shorthand
  # `folder = { path = "Some/Folder" }` is accepted but implicitly creates any
  # missing folders server-side and will not destroy them with the dashboard.
  folder = {
    id = coralogix_dashboards_folder.example.id
  }
}

resource "coralogix_dashboards_folder" "example" {
  name = "example"
}

resource "coralogix_dashboard" "widgets" {
  name        = "widget-examples"
  description = "Widget testing"
  time_frame = {
    relative = {
      duration = "seconds:900" # 15 minutes
    }
  }
  layout = {
    sections = [{
      rows = [{
        height = 19
        widgets = [{
          title = "hexagon"
          definition = {
            hexagon = {
              min            = 0
              max            = 100
              decimal        = 2
              threshold_type = "relative"
              thresholds = [{
                from  = 0
                color = "var(--c-severity-log-verbose)"
                },
                {
                  from  = 33
                  color = "var(--c-severity-log-warning)"
                },
                {
                  from  = 66
                  color = "var(--c-severity-log-error)"
              }]
              query = {
                logs = {
                  aggregation = {
                    type = "count"
                  }
                  group_by = [{
                    keypath = ["subsystemname"]
                    scope   = "label"
                  }]
                }
              }
              legend_by = "groups"
              legend = {
                is_visible = true
              }
            }
          }
        }]
      }]
      },
      {
        # A `dynamic` widget pairs one or more `query_definitions` with a single
        # `visualization`. Each query picks exactly one source (logs here), and the
        # widget picks exactly one visualization.
        rows = [{
          height = 19
          widgets = [{
            title = "dynamic stat - error volume"
            definition = {
              dynamic = {
                query_definitions = [{
                  name = "errors"
                  query = {
                    logs = {
                      lucene_query = "coralogix.metadata.severity=\"5\" OR coralogix.metadata.severity=\"6\""
                      group_by = [{
                        keypath = ["subsystemname"]
                        scope   = "label"
                      }]
                      aggregations = [{
                        type = "count"
                      }]
                    }
                  }
                }]
                time_frame = {
                  relative = {
                    duration = "seconds:900"
                  }
                }
                visualization = {
                  stat = {
                    decimal_precision = 0
                    threshold_type    = "absolute"
                    thresholds = [
                      { from = 0, color = "green" },
                      { from = 1000, color = "red" },
                    ]
                  }
                }
              }
            }
            },
            {
              # `stat_card` shows a headline value with its own title and label.
              # Each of the three text elements is read from a field
              # (`observation_field`) or templated (`template_text`). `title` and
              # `label` may instead set `mapped_values = true` to show the result
              # of `color_label_mapping`; `primary_value` cannot, because it is
              # the value that mapping reads from.
              title = "dynamic stat card - p99 latency"
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
                      unit              = "milliseconds"
                      decimal_precision = 1
                      title = {
                        template_text = "p99 latency"
                      }
                      label = {
                        observation_field = {
                          keypath = ["applicationname"]
                          scope   = "label"
                        }
                      }
                      primary_value = {
                        observation_field = {
                          keypath = ["meta.responseTime.numeric"]
                          scope   = "user_data"
                        }
                      }
                      # Colour the value by range. Exactly one of `range`, `value`
                      # or `regex` may be set.
                      color_label_mapping = {
                        color_by = "value"
                        range = {
                          threshold_type = "absolute"
                          min_max = {
                            custom = {
                              min = 0
                              max = 1000
                            }
                          }
                          thresholds = [
                            { from = 0, color = "green", label = "fast" },
                            { from = 500, color = "red", label = "slow" },
                          ]
                        }
                      }
                    }
                  }
                }
              }
            },
            {
              # `time_series_lines_multi` plots one line per query over time.
              # `query_display_settings` styles an individual query, so that
              # query needs an explicit `id` to reference.
              title = "dynamic time series - latency by app"
              definition = {
                dynamic = {
                  query_definitions = [{
                    id   = "9d1b7a4e-0000-4000-8000-00000000ab01"
                    name = "p99"
                    query = {
                      metrics = {
                        promql_query = "histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))"
                        # Time-series charts need a range query. An instant query
                        # returns a single point and the chart renders empty.
                        promql_query_type = "range"
                      }
                    }
                  }]
                  visualization = {
                    time_series_lines_multi = {
                      connect_nulls      = true
                      stacked_line       = "absolute"
                      x_axis_time_format = "hh_mm"
                      query_display_settings = [{
                        query_id          = "9d1b7a4e-0000-4000-8000-00000000ab01"
                        scale_type        = "linear"
                        unit              = "seconds"
                        decimal_precision = 2
                        y_axis_min        = 0
                      }]
                      legend = {
                        is_visible = true
                        placement  = "bottom"
                      }
                    }
                  }
                }
              }
            },
            {
              # A `table` lists raw rows. `rules` style individual columns:
              # each rule picks columns with `rule_scope` and applies exactly
              # one `definition` — rename, align, format, map values, and so on.
              title = "dynamic table - recent errors"
              definition = {
                dynamic = {
                  query_definitions = [{
                    name = "rows"
                    query = {
                      logs = {
                        lucene_query = "coralogix.metadata.severity=\"5\""
                      }
                    }
                  }]
                  visualization = {
                    table = {
                      columns = [
                        { field = { keypath = ["applicationname"], scope = "label" } },
                        { field = { keypath = ["severity"], scope = "metadata" } },
                      ]
                      settings = {
                        row_style = "one_line"
                        column_widths = [{
                          column_name = "applicationname"
                          width       = 200
                        }]
                      }
                      rules = [
                        {
                          name       = "rename the application column"
                          rule_scope = { field = { keypath = ["applicationname"], scope = "label" } }
                          properties = [{ definition = { column_display_name = "Application" } }]
                        },
                        {
                          name       = "show severity names instead of numbers"
                          rule_scope = { field = { keypath = ["severity"], scope = "metadata" } }
                          properties = [{
                            definition = {
                              values_mapping = {
                                mappings = [
                                  { input_value = "5", replace_value = "ERROR", type = "value" },
                                  { input_value = "6", replace_value = "CRITICAL", type = "value" },
                                ]
                              }
                            }
                          }]
                        },
                      ]
                    }
                  }
                }
              }
            },
            {
              # `vertical_bars_multi` plots one bar group per query. `sort_order`
              # picks the sort key: `category` is a marker with no settings of
              # its own, or `query_value` names the query to sort by.
              title = "dynamic bars - errors by application"
              definition = {
                dynamic = {
                  query_definitions = [{
                    id   = "9d1b7a4e-0000-4000-8000-00000000ba01"
                    name = "errors"
                    query = {
                      logs = {
                        lucene_query = "coralogix.metadata.severity=\"5\""
                        group_by = [{
                          keypath = ["applicationname"]
                          scope   = "label"
                        }]
                        # A bar chart needs a value to plot, so the query has to
                        # aggregate. The result column is named after the
                        # aggregation and its index, hence `count_0` below.
                        aggregations = [{
                          type = "count"
                        }]
                      }
                    }
                  }]
                  visualization = {
                    vertical_bars_multi = {
                      unit               = "seconds"
                      scale_type         = "linear"
                      colors_by          = "query"
                      max_bars_per_chart = 20
                      bar_value_display  = "top"
                      sort_order = {
                        order_direction = "desc"
                        strategy = {
                          query_value = { query_id = "9d1b7a4e-0000-4000-8000-00000000ba01" }
                        }
                      }
                      query_field_settings = [{
                        query_id    = "9d1b7a4e-0000-4000-8000-00000000ba01"
                        value_field = { keypath = ["count_0"], scope = "user_data" }
                      }]
                      # The grouped field arrives as a data column, so it is
                      # read from `user_data` here rather than from `label`.
                      category_fields = [{ keypath = ["applicationname"], scope = "user_data" }]
                      legend = {
                        is_visible = true
                        placement  = "bottom"
                      }
                    }
                  }
                }
              }
            },
            {
              # `gauge` shows one aggregated value on an arc, coloured by
              # `thresholds`. The query has to aggregate, and the result column
              # is named after the aggregation and its index.
              title = "dynamic gauge - error rate"
              definition = {
                dynamic = {
                  query_definitions = [{
                    name = "errors"
                    query = {
                      logs = {
                        lucene_query = "coralogix.metadata.severity=\"5\""
                        aggregations = [{
                          type = "count"
                        }]
                      }
                    }
                  }]
                  visualization = {
                    gauge = {
                      unit               = "percent"
                      min                = 0
                      max                = 1000
                      show_inner_arc     = true
                      show_outer_arc     = true
                      allow_abbreviation = true
                      threshold_type     = "absolute"
                      arc_display = {
                        threshold_arc = true
                      }
                      thresholds = [
                        { from = 0, color = "green", label = "ok" },
                        { from = 500, color = "red", label = "high" },
                      ]
                      value_field = { keypath = ["count_0"], scope = "user_data" }
                    }
                  }
                }
              }
            }
          ]
        }]
    }]
  }
}

# Cross-dashboard widget reference: reuse a widget from another dashboard by ID.
resource "coralogix_dashboard" "dashboard_with_widget_reference" {
  name        = "portal monitoring shared"
  description = "Dashboard that reuses a widget from coralogix_dashboard.dashboard"
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
            dashboard_id = coralogix_dashboard.dashboard.id
            widget_id    = coralogix_dashboard.dashboard.layout.sections[0].rows[0].widgets[0].id
          }
        }]
      }]
    }]
  }
}

resource "coralogix_dashboard" "dashboard_from_json_with_folder" {
  content_json = file("./dashboard.json")
  folder = {
    id = coralogix_dashboards_folder.example.id
  }
}
