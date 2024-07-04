package coralogix

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"terraform-provider-coralogix/coralogix/clientset"
)

var alertResourceName = "coralogix_alert.test"

func TestAccCoralogixResourceAlert_logs_immediate(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertStandard(&alert),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_logs_more_than(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertStandard(&alert),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_logs_less_than(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertStandard(&alert),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_logs_more_than_usual(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertStandard(&alert),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_logs_ratio_more_than(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertStandard(&alert),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_logs_ratio_less_than(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertStandard(&alert),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_logs_new_value(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertStandard(&alert),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_logs_unique_count(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertStandard(&alert),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_logs_time_relative_more_than(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertStandard(&alert),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_logs_time_relative_less_than(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertStandard(&alert),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_metric_more_than(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertStandard(&alert),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_metric_less_than(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertStandard(&alert),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_metric_less_than_usual(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertStandard(&alert),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_metric_more_than_usual(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertStandard(&alert),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_metric_more_than_or_equals(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertStandard(&alert),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_metric_less_than_or_equals(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertStandard(&alert),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_tracing_immediate(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertStandard(&alert),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_tracing_more_than(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertStandard(&alert),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_flow(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertStandard(&alert),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertStandard(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},
		},
	})
}

func testAccCheckAlertDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).Alerts()

	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_alert" {
			continue
		}

		req := &alertsv3.GetAlertByUniqueIdRequest{
			Id: wrapperspb.String(rs.Primary.ID),
		}

		resp, err := client.GetAlert(ctx, req)
		if err == nil {
			if resp.Alert.Id.Value == rs.Primary.ID {
				return fmt.Errorf("alert still exists: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}

func testAccCoralogixResourceAlertStandard(a *standardAlertTestParams) string {
	return fmt.Sprintf(`resource "coralogix_alert" "test" {
  name               = "%s"
  description        = "%s"
  severity           = "%s"

  notifications_group {
	notification {
		integration_id       = "%s"
	}
    notification {
        email_recipients             = %s
    }
  }

	incident_settings {
		notify_on = "%s"
		retriggering_period_minutes = %d
	}

  scheduling {
    time_zone =  "%s"
	time_frame {
    	days_enabled = %s
    	start_time = "%s"
    	end_time = "%s"
  	}
  }

   meta_labels = {
   	    alert_type        = "security"
    	security_severity = "high"
   }

  standard {
    severities = %s
    search_query = "%s"
    condition {
	  group_by = %s
      less_than = true
      threshold = %d
      time_window = "%s"
      manage_undetected_values {
			enable_triggering_on_undetected_values = true
			auto_retire_ratio = "%s"
		}
    }
  }
}
`,
		a.name, a.description, a.severity, a.webhookID, sliceToString(a.emailRecipients), a.notifyOn, a.notifyEveryMin, a.timeZone,
		sliceToString(a.daysOfWeek), a.activityStarts, a.activityEnds,
		sliceToString(a.severities), a.searchQuery, sliceToString(a.groupBy), a.occurrencesThreshold, a.timeWindow, a.deadmanRatio)
}

func testAccCoralogixResourceAlertRatio(a *ratioAlertTestParams) string {
	return fmt.Sprintf(`resource "coralogix_alert" "test" {
  name               = "%s"
  description        = "%s"
  severity           = "%s"

  notifications_group {
  	notification {
			integration_id       = "%s"
   }
	notification {
		email_recipients             = %s
	}
  }

	incident_settings {
		notify_on = "%s"
		retriggering_period_minutes = %d
	}	

  scheduling {
    time_zone =  "%s"
	
	time_frame {
    	days_enabled = %s
    	start_time = "%s"
    	end_time = "%s"
  	}
  }

  ratio {
    query_1 {
		severities   = %s
		search_query = "%s"
    }
    query_2 {
      severities   = %s
      search_query = "%s"
    }
    condition {
      more_than     = true
      ratio_threshold = %f
      time_window   = "%s"
      group_by      = %s
      group_by_q1   = true
	  ignore_infinity = %t
    }
  }
}`,
		a.name, a.description, a.severity, a.webhookID, sliceToString(a.emailRecipients), a.notifyOn, a.notifyEveryMin, a.timeZone,
		sliceToString(a.daysOfWeek), a.activityStarts, a.activityEnds,
		sliceToString(a.severities), a.searchQuery, sliceToString(a.q2Severities), a.q2SearchQuery,
		a.ratio, a.timeWindow, sliceToString(a.groupBy), a.ignoreInfinity)
}

func testAccCoralogixResourceAlertNewValue(a *newValueAlertTestParams) string {
	return fmt.Sprintf(`resource "coralogix_alert" "test" {
  name               = "%s"
  description        = "%s"
  severity           = "%s"
  
  notifications_group {
		notification {
        	integration_id       = "%s"
		}
		notification{
     		email_recipients             = %s
     	}
	}

	  incident_settings {
			notify_on = "%s"
			retriggering_period_minutes = %d
		}

  scheduling {
    time_zone =  "%s"
	
	time_frame {
    	days_enabled = %s
    	start_time = "%s"
    	end_time = "%s"
  	}
  }

  new_value {
    severities = %s
	search_query = "%s"
    condition {
      key_to_track = "%s"
      time_window  = "%s"
    }
  }
}`,
		a.name, a.description, a.severity, a.webhookID, sliceToString(a.emailRecipients), a.notifyOn, a.notifyEveryMin, a.timeZone,
		sliceToString(a.daysOfWeek), a.activityStarts, a.activityEnds,
		sliceToString(a.severities), a.searchQuery, a.keyToTrack, a.timeWindow)
}

func testAccCoralogixResourceAlertUniqueCount(a *uniqueCountAlertTestParams) string {
	return fmt.Sprintf(`resource "coralogix_alert" "test" {
  name               = "%s"
  description        = "%s"
  severity           = "%s"
  
  notifications_group {
  		group_by_fields = %s
		notification {
        	integration_id       = "%s"
		}
		notification{
     		email_recipients             = %s
     	}
	}
	
	incident_settings {
    	notify_on = "%s"
    	retriggering_period_minutes = %d
  	}

  scheduling {
    time_zone =  "%s"
	time_frame {
    	days_enabled = %s
    	start_time = "%s"
    	end_time = "%s"
  	}
  }

  unique_count {
    severities = %s
    search_query = "%s"
    condition {
      unique_count_key  = "%s"
      max_unique_values = %d
      time_window       = "%s"
      group_by_key                   = "%s"
      max_unique_values_for_group_by = %d
    }
  }
}`,
		a.name, a.description, a.severity, sliceToString([]string{a.groupByKey}), a.webhookID, sliceToString(a.emailRecipients), a.notifyOn, a.notifyEveryMin, a.timeZone,
		sliceToString(a.daysOfWeek), a.activityStarts, a.activityEnds, sliceToString(a.severities),
		a.searchQuery, a.uniqueCountKey, a.maxUniqueValues, a.timeWindow, a.groupByKey, a.maxUniqueValuesForGroupBy)
}

func testAccCoralogixResourceAlertTimeRelative(a *timeRelativeAlertTestParams) string {
	return fmt.Sprintf(`resource "coralogix_alert" "test" {
  name               = "%s"
  description        = "%s"
  severity           = "%s"
  
  notifications_group {
		notification {
        	integration_id       = "%s"
		}
		notification{
     		email_recipients             = %s
     	}
	}

  incident_settings {
    	notify_on = "%s"
    	retriggering_period_minutes = %d
 }

  scheduling {
    time_zone =  "%s"
	
	time_frame {
    	days_enabled = %s
    	start_time = "%s"
    	end_time = "%s"
  	}
  }

  time_relative {
    severities = %s
    search_query = "%s"
    condition {
      more_than            = true
      group_by             = %s
      ratio_threshold      = %d
      relative_time_window = "%s"
      ignore_infinity = %t
    }
  }
}`,
		a.name, a.description, a.severity, a.webhookID, sliceToString(a.emailRecipients), a.notifyOn, a.notifyEveryMin, a.timeZone,
		sliceToString(a.daysOfWeek), a.activityStarts, a.activityEnds,
		sliceToString(a.severities), a.searchQuery, sliceToString(a.groupBy), a.ratioThreshold, a.relativeTimeWindow, a.ignoreInfinity)
}

func testAccCoralogixResourceAlertMetricLucene(a *metricLuceneAlertTestParams) string {
	return fmt.Sprintf(`resource "coralogix_alert" "test" {
  name               = "%s"
  description        = "%s"
  severity           = "%s"
  
  notifications_group {
		notification {
        	integration_id       = "%s"
		}
		notification{
     		email_recipients             = %s
     	}
	}

	incident_settings {
    	notify_on = "%s"
    	retriggering_period_minutes = %d
 	}

  scheduling {
    time_zone =  "%s"
	
	time_frame {
    	days_enabled = %s
    	start_time = "%s"
    	end_time = "%s"
  	}
  }

  metric {
    lucene {
      search_query = "%s"
      condition {
        metric_field                 = "%s"
        arithmetic_operator          = "%s"
        less_than                    = true
        threshold                    = %d
        arithmetic_operator_modifier = %d
        sample_threshold_percentage  = %d
        time_window                  = "%s"
		group_by = %s
		manage_undetected_values{
			enable_triggering_on_undetected_values = false
		}
      }
    }
  }
}`,
		a.name, a.description, a.severity, a.webhookID, sliceToString(a.emailRecipients), a.notifyOn, a.notifyEveryMin, a.timeZone,
		sliceToString(a.daysOfWeek), a.activityStarts, a.activityEnds, a.searchQuery, a.metricField, a.arithmeticOperator,
		a.threshold, a.arithmeticOperatorModifier, a.sampleThresholdPercentage, a.timeWindow, sliceToString(a.groupBy))
}

func testAccCoralogixResourceAlertMetricPromql(a *metricPromqlAlertTestParams) string {
	return fmt.Sprintf(`resource "coralogix_alert" "test" {
  name               = "%s"
  description        = "%s"
  severity           = "%s"
  
  notifications_group {
		notification {
        	integration_id       = "%s"
		}
		notification{
     		email_recipients             = %s
     	}
	}

  incident_settings {
	notify_on = "%s"
	retriggering_period_minutes = %d	
  }

  scheduling {
    time_zone =  "%s"
	time_frame {
    	days_enabled = %s
    	start_time = "%s"
    	end_time = "%s"
  	}
  }

  metric {
    promql {
      search_query = "http_requests_total{status!~\"4..\"}"
      condition {
        %s                    	     = true
        threshold                    = %d
        sample_threshold_percentage  = %d
        time_window                  = "%s"
        min_non_null_values_percentage = %d
      }
    }
  }
}`,
		a.name, a.description, a.severity, a.webhookID, sliceToString(a.emailRecipients), a.notifyOn, a.notifyEveryMin, a.timeZone,
		sliceToString(a.daysOfWeek), a.activityStarts, a.activityEnds, a.condition, a.threshold, a.sampleThresholdPercentage,
		a.timeWindow, a.nonNullPercentage)
}

func testAccCoralogixResourceAlertTracing(a *tracingAlertTestParams) string {
	return fmt.Sprintf(`resource "coralogix_alert" "test" {
  name               = "%s"
  description        = "%s"
  severity           = "%s"
  
	notifications_group {
		notification {
        	integration_id       = "%s"
		}
		notification{
     		email_recipients             = %s
     	}
	}

 incident_settings {
 	notify_on = "%s"
    retriggering_period_minutes = %d
 }

  scheduling {
    time_zone =  "%s"
	time_frame {
    	days_enabled = %s
    	start_time = "%s"
    	end_time = "%s"
  	}
  }

  tracing {
    latency_threshold_milliseconds = %f
    applications = ["nginx"]
    subsystems = ["subsystem-name"]
	tag_filter {
      field = "Status"
      values = ["filter:contains:400", "500"]
    }

    condition {
      more_than             = true
      time_window           = "%s"
      threshold = %d
    }
  }
}`,
		a.name, a.description, a.severity, a.webhookID, sliceToString(a.emailRecipients), a.notifyOn, a.notifyEveryMin, a.timeZone,
		sliceToString(a.daysOfWeek), a.activityStarts, a.activityEnds,
		a.conditionLatencyMs, a.timeWindow, a.occurrencesThreshold)
}

func testAccCoralogixResourceAlertFLow(a *flowAlertTestParams) string {
	return fmt.Sprintf(`resource "coralogix_alert" "standard_alert" {
	name               = "standard"
	severity           = "Info"

	notifications_group {
    	notification {
      		email_recipients            = ["example@coralogix.com"]
			retriggering_period_minutes = 1
     		notify_on                   = "Triggered_only"
    	}
  	}

	standard {
		condition {
      		more_than         = true
      		threshold         = 5
      		time_window       = "30Min"
      		group_by          = ["coralogix.metadata.sdkId"]
    	}
	}
}

	resource "coralogix_alert" "test" {
  		name               = "%s"
  		description        = "%s"
	  	severity           = "%s"
		
	  notifications_group {
		notification {
        	integration_id       = "%s"
		}
		notification{
     		email_recipients             = %s
     	}
	}

	incident_settings {
			notify_on = "%s"
			retriggering_period_minutes = %d
    }

  	scheduling {
    	time_zone =  "%s"
		time_frame {
    		days_enabled = %s
    		start_time = "%s"
			end_time = "%s"
  		}
	}

  	flow {
    	stage {
      		group {
        		sub_alerts {
          			operator = "OR"
          			flow_alert{
            			user_alert_id = coralogix_alert.standard_alert.id
          			}
        		}
        next_operator = "OR"
      }
      group {
        sub_alerts {
          operator = "AND"
          flow_alert{
            not = true
            user_alert_id = coralogix_alert.standard_alert.id
          }
        }
        next_operator = "AND"
      }
      time_window {
        minutes = 20
      }
    }
    stage {
      group {
        sub_alerts {
          operator = "AND"
          flow_alert {
            user_alert_id = coralogix_alert.standard_alert.id
          }
          flow_alert {
            not = true
            user_alert_id = coralogix_alert.standard_alert.id
          }
        }
        next_operator = "OR"
      }
    }
    group_by          = ["coralogix.metadata.sdkId"]
  }
}`,
		a.name, a.description, a.severity, a.webhookID, sliceToString(a.emailRecipients), a.notifyOn, a.notifyEveryMin, a.timeZone,
		sliceToString(a.daysOfWeek), a.activityStarts, a.activityEnds)
}

type standardAlertTestParams struct {
	groupBy              []string
	occurrencesThreshold int
	timeWindow           string
	deadmanRatio         string
	alertCommonTestParams
}

type ratioAlertTestParams struct {
	q2Severities, groupBy     []string
	ratio                     float64
	timeWindow, q2SearchQuery string
	ignoreInfinity            bool
	alertCommonTestParams
}

type newValueAlertTestParams struct {
	keyToTrack, timeWindow string
	alertCommonTestParams
}

type uniqueCountAlertTestParams struct {
	uniqueCountKey, timeWindow, groupByKey     string
	maxUniqueValues, maxUniqueValuesForGroupBy int
	alertCommonTestParams
}

type timeRelativeAlertTestParams struct {
	alertCommonTestParams
	ratioThreshold     int
	relativeTimeWindow string
	groupBy            []string
	ignoreInfinity     bool
}

type metricLuceneAlertTestParams struct {
	alertCommonTestParams
	groupBy                                                          []string
	metricField, timeWindow, arithmeticOperator                      string
	threshold, arithmeticOperatorModifier, sampleThresholdPercentage int
}

type metricPromqlAlertTestParams struct {
	alertCommonTestParams
	threshold, nonNullPercentage, sampleThresholdPercentage int
	timeWindow                                              string
	condition                                               string
}

type tracingAlertTestParams struct {
	alertCommonTestParams
	occurrencesThreshold int
	conditionLatencyMs   float64
	timeWindow           string
	groupBy              []string
}

type flowAlertTestParams struct {
	name, description, severity string
	emailRecipients             []string
	webhookID                   string
	notifyEveryMin              int
	notifyOn                    string
	activeWhen
}

type alertCommonTestParams struct {
	name, description, severity string
	webhookID                   string
	emailRecipients             []string
	notifyEveryMin              int
	notifyOn                    string
	searchQuery                 string
	alertFilters
	activeWhen
}

type alertFilters struct {
	severities []string
}

type activeWhen struct {
	daysOfWeek                             []string
	activityStarts, activityEnds, timeZone string
}

func randActiveWhen() activeWhen {
	return activeWhen{
		timeZone:       selectRandomlyFromSlice(validTimeZones),
		daysOfWeek:     selectManyRandomlyFromSlice(alertValidDaysOfWeek),
		activityStarts: randHourStr(),
		activityEnds:   randHourStr(),
	}
}

func randHourStr() string {
	return fmt.Sprintf("%s:%s",
		toTwoDigitsFormat(int32(acctest.RandIntRange(0, 24))),
		toTwoDigitsFormat(int32(acctest.RandIntRange(0, 60))))
}
