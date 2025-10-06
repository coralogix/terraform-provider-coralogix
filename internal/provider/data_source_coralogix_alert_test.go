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
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var alertDataSourceName = "data." + alertResourceName

func TestAccCoralogixDataSourceAlert(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckActionDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertLogsImmediateForReading(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertDataSourceName, "name", "logs-more-than alert example"),
				),
			},
		},
	})
}

func testAccCoralogixResourceAlertLogsImmediateForReading() string {
	return `resource "coralogix_alert" "dstest" {
  name        = "logs-more-than alert example"
  description = "Example of logs-more-than alert example from terraform"
  priority    = "P2"

  labels = {
    alert_type        = "security"
    security_severity = "high"
  }

  notification_group = {
    simple_target_settings = [
      {
        recipients = ["example@coralogix.com"]
      }
    ]
  }

  incidents_settings = {
    notify_on = "Triggered and Resolved"
    retriggering_period = {
      minutes = 1
    }
  }

  schedule = {
    active_on = {
      days_of_week = ["Wednesday", "Thursday"]
      start_time = "08:30"
      end_time = "20:30"
    }
  }

  type_definition = {
    logs_threshold = {
      rules = [ { 
        condition = {
          threshold   = 2.0
          time_window = "10_MINUTES"
          condition_type = "MORE_THAN" 
        } 
        override = {
            priority = "P2"
          }
        }
      ]
      logs_filter = {
        simple_filter = {
          lucene_query = "message:\"error\""
          label_filters = {
            application_name = [
              {
                operation = "IS"
                value     = "nginx"
              }
            ]
            subsystem_name = [
              {
                operation = "IS"
                value     = "subsystem-name"
              }
            ]
            severities = ["Warning"]
          }
        }
      }
    }
  }
}

data "coralogix_alert" "test" {
	id = coralogix_alert.dstest.id
}
`
}

func TestAccCoralogixAlertWebhooksNotifyOnMandatory(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckActionDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertWebhooksNotifyOnMandatory(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertDataSourceName, "notification_group.webhooks_settings.0.notify_on", "Triggered Only"),
				),
			},
		},
	})
}

func testAccCoralogixResourceAlertWebhooksNotifyOnMandatory() string {
	return `resource "coralogix_alert" "dstest" {
  name        = "logs-more-than alert example"
  description = "Example of logs-more-than alert example from terraform"
  priority    = "P2"

  labels = {
    alert_type        = "security"
    security_severity = "high"
  }

  notification_group = {
    webhooks_settings = [{
      notify_on = "Triggered Only"
      integration_id = "417433"
      retriggering_period = { minutes = 720 }
    }]
    simple_target_settings = [
      {
        recipients = ["example@coralogix.com"]
      }
    ]
  }

  incidents_settings = {
    notify_on = "Triggered and Resolved"
    retriggering_period = {
      minutes = 1
    }
  }

  schedule = {
    active_on = {
      days_of_week = ["Wednesday", "Thursday"]
      start_time = "08:30"
      end_time = "20:30"
    }
  }

  type_definition = {
    logs_threshold = {
      rules = [ { 
        condition = {
          threshold   = 2.0
          time_window = "10_MINUTES"
          condition_type = "MORE_THAN" 
        } 
        override = {
            priority = "P2"
          }
        }
      ]
      logs_filter = {
        simple_filter = {
          lucene_query = "message:\"error\""
          label_filters = {
            application_name = [
              {
                operation = "IS"
                value     = "nginx"
              }
            ]
            subsystem_name = [
              {
                operation = "IS"
                value     = "subsystem-name"
              }
            ]
            severities = ["Warning"]
          }
        }
      }
    }
  }
}

data "coralogix_alert" "test" {
	id = coralogix_alert.dstest.id
}
`
}
