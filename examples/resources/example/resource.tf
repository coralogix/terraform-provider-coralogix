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