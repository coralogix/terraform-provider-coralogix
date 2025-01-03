# Create an import file like this

tee -a import.tf <<EOF
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
#
#import {
#  to = coralogix_alert.logs_immediate_alert
#  id = "19e27a6d-470d-47e9-9447-d1a1bb512eb6"
#}
#
#import {
#  to = coralogix_alert.flow_alert_example
#  id = "41544404-db3c-4d6e-b039-b8cc3efd51f8"
#}
#
#import {
#  to = coralogix_alert.logs_new_value
#  id = "4c760ad4-2eb4-444b-9285-8a86f3eda7cb"
#}
#
#import {
#  to = coralogix_alert.tracing_more_than
#  id = "b8529327-87e2-4140-89df-3541d3171f1a"
#}
#
#import {
#  to = coralogix_alert.logs-ratio-more-than
#  id = "187f3ea4-caa7-46e1-82c0-2dfd1e67a680"
#}
EOF

## Follow the Migration Guide to obtain the following:


# __generated__ by Terraform
# Please review these resources and move them into your main configuration files.

# __generated__ by Terraform from "4c760ad4-2eb4-444b-9285-8a86f3eda7cb"
#resource "coralogix_alert" "logs_new_value" {
#  deleted     = false
#  description = "Example of logs-new-value alert from terraform"
#  enabled     = true
#  group_by    = null
#  incidents_settings = {
#    notify_on = "Triggered Only"
#    retriggering_period = {
#      minutes = 10
#    }
#  }
#  labels = null
#  name   = "logs-new-value alert example"
#  notification_group = {
#    group_by_keys     = null
#    webhooks_settings = null
#  }
#  phantom_mode = false
#  priority     = "P2"
#  schedule     = null
#  type_definition = {
#    flow           = null
#    logs_anomaly   = null
#    logs_immediate = null
#    logs_new_value = {
#      logs_filter = {
#        simple_filter = {
#          label_filters = {
#            application_name = null
#            severities       = null
#            subsystem_name   = null
#          }
#          lucene_query = null
#        }
#      }
#      notification_payload_filter = ["coralogix.metadata.sdkId", "coralogix.metadata.sdkName", "coralogix.metadata.sdkVersion"]
#      rules = [
#        {
#          condition = {
#            keypath_to_track = "remote_addr_geoip.country_name"
#            time_window = {
#              specific_value = "12_HOURS"
#            }
#          }
#        },
#      ]
#    }
#    logs_ratio_threshold         = null
#    logs_threshold               = null
#    logs_time_relative_threshold = null
#    logs_unique_count            = null
#    metric_anomaly               = null
#    metric_threshold             = null
#    tracing_immediate            = null
#    tracing_threshold            = null
#  }
#}
#
## __generated__ by Terraform from "41544404-db3c-4d6e-b039-b8cc3efd51f8"
#resource "coralogix_alert" "flow_alert_example" {
#  deleted     = false
#  description = "Example of flow alert from terraform"
#  enabled     = true
#  group_by    = null
#  incidents_settings = {
#    notify_on = "Triggered Only"
#    retriggering_period = {
#      minutes = 10
#    }
#  }
#  labels = null
#  name   = "flow alert example"
#  notification_group = {
#    group_by_keys     = null
#    webhooks_settings = null
#  }
#  phantom_mode = false
#  priority     = "P3"
#  schedule     = null
#  type_definition = {
#    flow = {
#      enforce_suppression = false
#      stages = [
#        {
#          flow_stages_groups = [
#            {
#              alert_defs = [
#                {
#                  id  = "5c197c44-a51d-4c70-a90a-77a4a21ae3d8"
#                  not = false
#                },
#                {
#                  id  = "f8a782a1-a503-4987-884f-7dac4b834b03"
#                  not = false
#                },
#              ]
#              alerts_op = "OR"
#              next_op   = "AND"
#            },
#            {
#              alert_defs = [
#                {
#                  id  = "81bba4f8-332c-4bc4-b5d2-bd074a4f969e"
#                  not = false
#                },
#                {
#                  id  = "f8a782a1-a503-4987-884f-7dac4b834b03"
#                  not = false
#                },
#              ]
#              alerts_op = "AND"
#              next_op   = "OR"
#            },
#          ]
#          timeframe_ms   = 10
#          timeframe_type = "Up To"
#        },
#      ]
#    }
#    logs_anomaly                 = null
#    logs_immediate               = null
#    logs_new_value               = null
#    logs_ratio_threshold         = null
#    logs_threshold               = null
#    logs_time_relative_threshold = null
#    logs_unique_count            = null
#    metric_anomaly               = null
#    metric_threshold             = null
#    tracing_immediate            = null
#    tracing_threshold            = null
#  }
#}
#
## __generated__ by Terraform from "19e27a6d-470d-47e9-9447-d1a1bb512eb6"
#resource "coralogix_alert" "logs_immediate_alert" {
#  deleted     = false
#  description = "Example of logs immediate alert from terraform"
#  enabled     = true
#  group_by    = null
#  incidents_settings = {
#    notify_on = "Triggered and Resolved"
#    retriggering_period = {
#      minutes = 10
#    }
#  }
#  labels = {
#    alert_type        = "security"
#    security_severity = "high"
#  }
#  name = "logs immediate alert"
#  notification_group = {
#    group_by_keys     = null
#    webhooks_settings = null
#  }
#  phantom_mode = false
#  priority     = "P2"
#  schedule = {
#    active_on = {
#      days_of_week = ["Wednesday", "Thursday"]
#      end_time = {
#        hours   = 20
#        minutes = 30
#      }
#      start_time = {
#        hours   = 8
#        minutes = 30
#      }
#    }
#  }
#  type_definition = {
#    flow         = null
#    logs_anomaly = null
#    logs_immediate = {
#      logs_filter = {
#        simple_filter = {
#          label_filters = {
#            application_name = null
#            severities       = null
#            subsystem_name   = null
#          }
#          lucene_query = "message:\"error\""
#        }
#      }
#      notification_payload_filter = null
#    }
#    logs_new_value               = null
#    logs_ratio_threshold         = null
#    logs_threshold               = null
#    logs_time_relative_threshold = null
#    logs_unique_count            = null
#    metric_anomaly               = null
#    metric_threshold             = null
#    tracing_immediate            = null
#    tracing_threshold            = null
#  }
#}
#
## __generated__ by Terraform from "187f3ea4-caa7-46e1-82c0-2dfd1e67a680"
#resource "coralogix_alert" "logs-ratio-more-than" {
#  deleted     = false
#  description = "Example of logs-ratio-more-than alert from terraform"
#  enabled     = true
#  group_by    = ["coralogix.metadata.alert_id", "coralogix.metadata.alert_name"]
#  incidents_settings = {
#    notify_on = "Triggered Only"
#    retriggering_period = {
#      minutes = 10
#    }
#  }
#  labels = null
#  name   = "logs-ratio-more-than alert example"
#  notification_group = {
#    group_by_keys     = ["coralogix.metadata.alert_id", "coralogix.metadata.alert_name"]
#    webhooks_settings = null
#  }
#  phantom_mode = false
#  priority     = "P1"
#  schedule     = null
#  type_definition = {
#    flow           = null
#    logs_anomaly   = null
#    logs_immediate = null
#    logs_new_value = null
#    logs_ratio_threshold = {
#      denominator = {
#        simple_filter = {
#          label_filters = {
#            application_name = [
#              {
#                operation = "IS"
#                value     = "nginx"
#              },
#            ]
#            severities = ["Warning"]
#            subsystem_name = [
#              {
#                operation = "IS"
#                value     = "subsystem-name"
#              },
#            ]
#          }
#          lucene_query = "mod_date:[20020101 TO 20030101]"
#        }
#      }
#      denominator_alias           = "denominator"
#      group_by_for                = "Both"
#      notification_payload_filter = null
#      numerator = {
#        simple_filter = {
#          label_filters = {
#            application_name = [
#              {
#                operation = "IS"
#                value     = "nginx"
#              },
#            ]
#            severities = ["Error"]
#            subsystem_name = [
#              {
#                operation = "IS"
#                value     = "subsystem-name"
#              },
#            ]
#          }
#          lucene_query = "mod_date:[20030101 TO 20040101]"
#        }
#      }
#      numerator_alias = "numerator"
#      rules = [
#        {
#          condition = {
#            condition_type = "MORE_THAN"
#            threshold      = 2
#            time_window = {
#              specific_value = "10_MINUTES"
#            }
#          }
#          override = {
#            priority = "P2"
#          }
#        },
#      ]
#    }
#    logs_threshold               = null
#    logs_time_relative_threshold = null
#    logs_unique_count            = null
#    metric_anomaly               = null
#    metric_threshold             = null
#    tracing_immediate            = null
#    tracing_threshold            = null
#  }
#}
#
## __generated__ by Terraform
#resource "coralogix_alert" "tracing_more_than" {
#  deleted     = false
#  description = "Example of tracing_more_than alert from terraform"
#  enabled     = true
#  group_by    = null
#  incidents_settings = {
#    notify_on = "Triggered Only"
#    retriggering_period = {
#      minutes = 10
#    }
#  }
#  labels = null
#  name   = "tracing_more_than alert example"
#  notification_group = {
#    group_by_keys     = null
#    webhooks_settings = null
#  }
#  phantom_mode = false
#  priority     = "P2"
#  schedule     = null
#  type_definition = {
#    flow                         = null
#    logs_anomaly                 = null
#    logs_immediate               = null
#    logs_new_value               = null
#    logs_ratio_threshold         = null
#    logs_threshold               = null
#    logs_time_relative_threshold = null
#    logs_unique_count            = null
#    metric_anomaly               = null
#    metric_threshold             = null
#    tracing_immediate            = null
#    tracing_threshold = {
#      notification_payload_filter = null
#      rules = [
#        {
#          condition = {
#            span_amount = 5
#            time_window = {
#              specific_value = "10_MINUTES"
#            }
#          }
#        },
#      ]
#      tracing_filter = {
#        latency_threshold_ms = 100
#        tracing_label_filters = {
#          application_name = [
#            {
#              operation = "IS"
#              values    = ["apache", "nginx"]
#            },
#            {
#              operation = "STARTS_WITH"
#              values    = ["application-name:"]
#            },
#          ]
#          operation_name = null
#          service_name   = null
#          span_fields    = null
#          subsystem_name = null
#        }
#      }
#    }
#  }
#}
#