package coralogix

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
      start_time = {
        hours   = 8
        minutes = 30
      }
      end_time = {
        hours   = 20
        minutes = 30
      }
    }
  }

  type_definition = {
    logs_threshold = {
      rules = [
        { 
	      threshold   = 2.0
          time_window = "10_MINUTES"
          condition = "MORE_THAN" 
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
