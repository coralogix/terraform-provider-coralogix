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
  layout {
    section {
      row {
        appearance {
          height = 19
        }
        widget {
          title = "status 4XX"
          definition {
            line_chart {
              query_definition {
                query {
                  metrics {
                    promql_query = "http_requests_total{status!~\"4..\"}"
                  }
                }
              }
              legend {
                is_visible = true
                column    = ["Max", "Last"]
              }
            }
          }
          appearance {
            width = 0
          }
        }
        widget {
          title = "count"
          definition {
            line_chart {
              query_definition {
                query {
                  logs {
                    aggregations {
                      count {
                      }
                    }
                  }
                }
              }
              legend {
                is_visible = true
                column    = ["Min", "Max", "Sum", "Avg", "Last"]
              }
            }
          }
          appearance {
            width = 0
          }
        }
        widget {
          title = "error throwing pods"
          definition {
            line_chart {
              query_definition {
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
              }
              legend {
                is_visible = true
                column    = ["Max", "Last"]
              }
            }
          }
          appearance {
            width = 0
          }
        }
      }
      row {
        appearance {
          height = 28
        }
        widget {
          title       = "dashboards-api logz"
          description = "warnings, errors, criticals"
          definition {
            data_table {
              query {
                logs {
                  filter {
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
              column {
                field = "coralogix.timestamp"
              }
              column {
                field = "textObject.textObject.textObject.kubernetes.pod_id"
              }
              column {
                field = "coralogix.text"
              }
              column {
                field = "coralogix.metadata.applicationName"
              }
              column {
                field = "coralogix.metadata.subsystemName"
              }
              column {
                field = "coralogix.metadata.sdkId"
              }
              column {
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
  variable {
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
  filter{
    source{
      logs{
        field = "coralogix.metadata.applicationName"
        operator {
          equals {
            selection {
              all = true
            }
          }
        }
      }
    }
  }
}

resource "coralogix_dashboard" test {
  name        = "dont drop me!"
  description = "dashboards team is messing with this 🗿"
  layout {
    section {
      row {
        appearance {
          height = 19
        }
        widget {
          title = "status 4XX"
          definition {
            line_chart {
              query_definition {
                query {
                  metrics {
                    promql_query = "http_requests_total{status!~\"4..\"}"
                  }
                }
              }
              legend {
                is_visible = true
                column    = ["Max", "Last"]
              }
            }
          }
          appearance {
            width = 0
          }
        }
        widget {
          title = "count"
          definition {
            line_chart {
              query_definition {
                query {
                  logs {
                    aggregations {
                      count {
                      }
                    }
                  }
                }
              }
              legend {
                is_visible = true
                column    = ["Min", "Max", "Sum", "Avg", "Last"]
              }
            }
          }
          appearance {
            width = 0
          }
        }
        widget {
          title = "error throwing pods"
          definition {
            line_chart {
              query_definition {
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
              }
              legend {
                is_visible = true
                column    = ["Max", "Last"]
              }
            }
          }
          appearance {
            width = 0
          }
        }
      }
      row {
        appearance {
          height = 28
        }
        widget {
          title       = "dashboards-api logz"
          description = "warnings, errors, criticals"
          definition {
            data_table {
              query {
                logs {
                  filter {
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
              column {
                field = "coralogix.timestamp"
              }
              column {
                field = "textObject.textObject.textObject.kubernetes.pod_id"
              }
              column {
                field = "coralogix.text"
              }
              column {
                field = "coralogix.metadata.applicationName"
              }
              column {
                field = "coralogix.metadata.subsystemName"
              }
              column {
                field = "coralogix.metadata.sdkId"
              }
              column {
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
  variable {
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

resource "coralogix_dashboard" "example1" {
  time_frame = {
    type = "absolute"
    from = "14:00" //(only if absolute)
    to = "16:00" //(only if absolute)
  }
  //or
  time_frame = {
    type = "relative"
    duration = "2H"//(only if relative)
  }
}

resource "coralogix_dashboard" "example2" {
  time_frame = {
    //exactly one of one of relative or absolute are required
    relative = {
      duration = "2H"
    }
    absolute = {
      from = "14:00"
      to = "16:00"
    }
  }
}

resource "coralogix_dashboard" "example3" {
  //exactly one of relative_time_frame and absolute_time_frame are required
  relative_time_frame = {
    duration = "2H"
  }

  absolute_time_frame = {
    from = "14:00"
    to = "16:00"
  }
}

resource "coralogix_dashboard" dashboard_from_json {
  content_json = file("./dashboard.json")
}