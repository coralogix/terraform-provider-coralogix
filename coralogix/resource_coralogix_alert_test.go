package coralogix

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"testing"

	"terraform-provider-coralogix/coralogix/clientset"
	alertsv1 "terraform-provider-coralogix/coralogix/clientset/grpc/com/coralogix/alerts/v1"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestAccCoralogixResourceAlert_standard(t *testing.T) {
	resourceName := "coralogix_alert.test"

	alert := standardAlertTestParams{
		alertCommonTestParams: *getRandomAlert(),
		groupBy:               []string{"EventType"},
		occurrencesThreshold:  acctest.RandIntRange(1, 1000),
		timeWindow:            selectRandomlyFromSlice(alertValidTimeFrames),
	}

	checks := extractCommonChecks(&alert.alertCommonTestParams, resourceName, "standard")
	checks = append(checks,
		resource.TestCheckResourceAttr(resourceName, "notification.0.notify_only_on_triggered_group_by_values", "true"),
		resource.TestCheckResourceAttr(resourceName, "meta_labels.0.key", "alert_type"),
		resource.TestCheckResourceAttr(resourceName, "meta_labels.0.value", "security"),
		resource.TestCheckResourceAttr(resourceName, "meta_labels.1.key", "security_severity"),
		resource.TestCheckResourceAttr(resourceName, "meta_labels.1.value", "High"),
		resource.TestCheckResourceAttr(resourceName, "standard.0.condition.0.occurrences_threshold", strconv.Itoa(alert.occurrencesThreshold)),
		resource.TestCheckResourceAttr(resourceName, "standard.0.condition.0.time_window", alert.timeWindow),
		resource.TestCheckResourceAttr(resourceName, "standard.0.condition.0.group_by.0", alert.groupBy[0]),
	)

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
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceAlert_ratio(t *testing.T) {
	resourceName := "coralogix_alert.test"

	alert := ratioAlertTestParams{
		alertCommonTestParams: *getRandomAlert(),
		q2Severities:          selectManyRandomlyFromSlice(alertValidLogSeverities),
		timeWindow:            selectRandomlyFromSlice(alertValidTimeFrames),
		ratio:                 randFloat(),
		groupBy:               []string{"EventType"},
		q2SearchQuery:         "remote_addr_enriched:/.*/",
	}

	checks := extractCommonChecks(&alert.alertCommonTestParams, resourceName, "ratio.0.query_1")
	checks = append(checks,
		resource.TestCheckResourceAttr(resourceName, "notification.0.notify_only_on_triggered_group_by_values", "true"),
		resource.TestCheckResourceAttr(resourceName, "ratio.0.query_2.0.search_query", alert.q2SearchQuery),
		resource.TestCheckResourceAttr(resourceName, "ratio.0.condition.0.more_than", "true"),
		resource.TestCheckResourceAttr(resourceName, "ratio.0.condition.0.queries_ratio", fmt.Sprintf("%f", alert.ratio)),
		resource.TestCheckResourceAttr(resourceName, "ratio.0.condition.0.time_window", alert.timeWindow),
		resource.TestCheckResourceAttr(resourceName, "ratio.0.condition.0.group_by.0", alert.groupBy[0]),
		resource.TestCheckResourceAttr(resourceName, "ratio.0.condition.0.group_by_q1", "true"),
	)

	for i, s := range alert.q2Severities {
		checks = append(checks,
			resource.TestCheckResourceAttr(resourceName,
				fmt.Sprintf("ratio.0.query_2.0.severities.%d", i), s))
	}

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertRatio(&alert),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceAlert_newValue(t *testing.T) {
	resourceName := "coralogix_alert.test"

	alert := newValueAlertTestParams{
		alertCommonTestParams: *getRandomAlert(),
		keyToTrack:            "EventType",
		timeWindow:            selectRandomlyFromSlice(alertValidNewValueTimeFrames),
	}

	checks := extractCommonChecks(&alert.alertCommonTestParams, resourceName, "new_value")
	checks = append(checks,
		resource.TestCheckResourceAttr(resourceName, "new_value.0.condition.0.key_to_track", alert.keyToTrack),
		resource.TestCheckResourceAttr(resourceName, "new_value.0.condition.0.time_window", alert.timeWindow),
	)

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertNewValue(&alert),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceAlert_uniqueCount(t *testing.T) {
	resourceName := "coralogix_alert.test"

	alert := uniqueCountAlertTestParams{
		alertCommonTestParams:     *getRandomAlert(),
		uniqueCountKey:            "EventType",
		timeWindow:                selectRandomlyFromSlice(alertValidUniqueCountTimeFrames),
		groupByKey:                "metadata.name",
		maxUniqueValues:           2,
		maxUniqueValuesForGroupBy: 20,
	}

	checks := extractCommonChecks(&alert.alertCommonTestParams, resourceName, "unique_count")
	checks = append(checks,
		resource.TestCheckResourceAttr(resourceName, "unique_count.0.condition.0.unique_count_key", alert.uniqueCountKey),
		resource.TestCheckResourceAttr(resourceName, "unique_count.0.condition.0.unique_count_key", alert.uniqueCountKey),
		resource.TestCheckResourceAttr(resourceName, "unique_count.0.condition.0.time_window", alert.timeWindow),
		resource.TestCheckResourceAttr(resourceName, "unique_count.0.condition.0.max_unique_values", strconv.Itoa(alert.maxUniqueValues)),
		resource.TestCheckResourceAttr(resourceName, "unique_count.0.condition.0.max_unique_values_for_group_by", strconv.Itoa(alert.maxUniqueValuesForGroupBy)),
	)

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertUniqueCount(&alert),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceAlert_timeRelative(t *testing.T) {
	resourceName := "coralogix_alert.test"

	alert := timeRelativeAlertTestParams{
		alertCommonTestParams: *getRandomAlert(),
		ratioThreshold:        acctest.RandIntRange(0, 1000),
		relativeTimeWindow:    selectRandomlyFromSlice(alertValidRelativeTimeFrames),
		groupBy:               []string{"EventType"},
	}

	checks := extractCommonChecks(&alert.alertCommonTestParams, resourceName, "time_relative")
	checks = append(checks,
		resource.TestCheckResourceAttr(resourceName, "notification.0.notify_only_on_triggered_group_by_values", "true"),
		resource.TestCheckResourceAttr(resourceName, "time_relative.0.condition.0.ratio_threshold", strconv.Itoa(alert.ratioThreshold)),
		resource.TestCheckResourceAttr(resourceName, "time_relative.0.condition.0.relative_time_window", alert.relativeTimeWindow),
		resource.TestCheckResourceAttr(resourceName, "time_relative.0.condition.0.group_by.0", alert.groupBy[0]),
	)

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertTimeRelative(&alert),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceAlert_metricLucene(t *testing.T) {
	resourceName := "coralogix_alert.test"

	alert := metricLuceneAlertTestParams{
		alertCommonTestParams:      *getRandomAlert(),
		groupBy:                    []string{"EventType"},
		metricField:                "subsystem",
		timeWindow:                 selectRandomlyFromSlice(alertValidTimeFrames),
		threshold:                  acctest.RandIntRange(0, 1000),
		arithmeticOperator:         selectRandomlyFromSlice(alertValidArithmeticOperators),
		arithmeticOperatorModifier: acctest.RandIntRange(0, 1000),
	}

	if alert.arithmeticOperator == "Percentile" {
		alert.arithmeticOperatorModifier = acctest.RandIntRange(0, 100)
	}

	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttrSet(resourceName, "id"),
		resource.TestCheckResourceAttr(resourceName, "enabled", "true"),
		resource.TestCheckResourceAttr(resourceName, "name", alert.name),
		resource.TestCheckResourceAttr(resourceName, "description", alert.description),
		resource.TestCheckResourceAttr(resourceName, "alert_severity", alert.severity),
		resource.TestCheckResourceAttr(resourceName, "notification.0.notify_only_on_triggered_group_by_values", "true"),
		resource.TestCheckResourceAttr(resourceName, "notification.0.recipients.0.emails.0", alert.emailRecipients[0]),
		resource.TestCheckResourceAttr(resourceName, "notification.0.notify_every_sec", strconv.Itoa(alert.notifyEverySec)),
		resource.TestCheckResourceAttr(resourceName, "metric.0.lucene.0.search_query", alert.searchQuery),
		resource.TestCheckResourceAttr(resourceName, "metric.0.lucene.0.condition.0.metric_field", alert.metricField),
		resource.TestCheckResourceAttr(resourceName, "metric.0.lucene.0.condition.0.arithmetic_operator", alert.arithmeticOperator),
		resource.TestCheckResourceAttr(resourceName, "metric.0.lucene.0.condition.0.more_than", "true"),
		resource.TestCheckResourceAttr(resourceName, "metric.0.lucene.0.condition.0.threshold", strconv.Itoa(alert.threshold)),
		resource.TestCheckResourceAttr(resourceName, "metric.0.lucene.0.condition.0.arithmetic_operator_modifier", strconv.Itoa(alert.arithmeticOperatorModifier)),
		resource.TestCheckResourceAttr(resourceName, "metric.0.lucene.0.condition.0.sample_threshold_percentage", strconv.Itoa(alert.sampleThresholdPercentage)),
		resource.TestCheckResourceAttr(resourceName, "metric.0.lucene.0.condition.0.time_window", alert.timeWindow),
		resource.TestCheckResourceAttr(resourceName, "metric.0.lucene.0.condition.0.group_by.0", alert.groupBy[0]),
	}

	for _, d := range alert.activeWhen.daysOfWeek {
		checks = append(checks, resource.TestCheckTypeSetElemAttr(resourceName, "scheduling.0.days_enabled.*", d))
	}

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertMetricLucene(&alert),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceAlert_metricPromql(t *testing.T) {
	resourceName := "coralogix_alert.test"

	alert := metricPromqlAlertTestParams{
		alertCommonTestParams:      *getRandomAlert(),
		threshold:                  acctest.RandIntRange(0, 1000),
		arithmeticOperatorModifier: acctest.RandIntRange(0, 1000),
		nonNullPercentage:          acctest.RandIntRange(0, 100),
		timeWindow:                 selectRandomlyFromSlice(alertValidTimeFrames),
	}

	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttrSet(resourceName, "id"),
		resource.TestCheckResourceAttr(resourceName, "enabled", "true"),
		resource.TestCheckResourceAttr(resourceName, "name", alert.name),
		resource.TestCheckResourceAttr(resourceName, "description", alert.description),
		resource.TestCheckResourceAttr(resourceName, "alert_severity", alert.severity),
		resource.TestCheckResourceAttr(resourceName, "notification.0.recipients.0.emails.0", alert.emailRecipients[0]),
		resource.TestCheckResourceAttr(resourceName, "notification.0.notify_every_sec", strconv.Itoa(alert.notifyEverySec)),
		resource.TestCheckResourceAttr(resourceName, "metric.0.promql.0.search_query", alert.searchQuery),
		resource.TestCheckResourceAttr(resourceName, "metric.0.promql.0.condition.0.threshold", strconv.Itoa(alert.threshold)),
		resource.TestCheckResourceAttr(resourceName, "metric.0.promql.0.condition.0.arithmetic_operator_modifier", strconv.Itoa(alert.arithmeticOperatorModifier)),
		resource.TestCheckResourceAttr(resourceName, "metric.0.promql.0.condition.0.more_than", "true"),
		resource.TestCheckResourceAttr(resourceName, "metric.0.promql.0.condition.0.sample_threshold_percentage", strconv.Itoa(alert.sampleThresholdPercentage)),
		resource.TestCheckResourceAttr(resourceName, "metric.0.promql.0.condition.0.min_non_null_values_percentage", strconv.Itoa(alert.nonNullPercentage)),
		resource.TestCheckResourceAttr(resourceName, "metric.0.promql.0.condition.0.time_window", alert.timeWindow),
	}

	checks = appendSchedulingChecks(checks, alert.daysOfWeek, resourceName)

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertMetricPromql(&alert),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceAlert_tracing(t *testing.T) {
	resourceName := "coralogix_alert.test"

	alert := tracingAlertTestParams{
		alertCommonTestParams: *getRandomAlert(),
		conditionLatencyMs:    math.Round(randFloat()*1000) / 1000,
		occurrencesThreshold:  acctest.RandIntRange(1, 10000),
		timeWindow:            selectRandomlyFromSlice(alertValidTimeFrames),
		groupBy:               []string{"EventType"},
	}

	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttrSet(resourceName, "id"),
		resource.TestCheckResourceAttr(resourceName, "enabled", "true"),
		resource.TestCheckResourceAttr(resourceName, "name", alert.name),
		resource.TestCheckResourceAttr(resourceName, "description", alert.description),
		resource.TestCheckResourceAttr(resourceName, "alert_severity", alert.severity),
		resource.TestCheckResourceAttr(resourceName, "notification.0.recipients.0.emails.0", alert.emailRecipients[0]),
		resource.TestCheckResourceAttr(resourceName, "notification.0.notify_every_sec", strconv.Itoa(alert.notifyEverySec)),
		resource.TestCheckResourceAttr(resourceName, "tracing.0.latency_threshold_ms", fmt.Sprintf("%.3f", alert.conditionLatencyMs)),
		resource.TestCheckResourceAttr(resourceName, "tracing.0.condition.0.more_than", "true"),
		resource.TestCheckResourceAttr(resourceName, "tracing.0.condition.0.time_window", alert.timeWindow),
		resource.TestCheckResourceAttr(resourceName, "tracing.0.condition.0.occurrences_threshold", strconv.Itoa(alert.occurrencesThreshold)),
		resource.TestCheckResourceAttr(resourceName, "tracing.0.field_filters.0.field", "Application"),
		resource.TestCheckResourceAttr(resourceName, "tracing.0.field_filters.0.filters.0.operator", "Equals"),
		resource.TestCheckResourceAttr(resourceName, "tracing.0.field_filters.0.filters.0.values.0", "nginx"),
	}

	checks = appendSchedulingChecks(checks, alert.daysOfWeek, resourceName)

	checks = appendSeveritiesCheck(checks, alert.alertFilters.severities, resourceName, "tracing")

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertTracing(&alert),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceAlert_flow(t *testing.T) {
	resourceName := "coralogix_alert.test"

	alert := flowAlertTestParams{
		name:            acctest.RandomWithPrefix("tf-acc-test"),
		description:     acctest.RandomWithPrefix("tf-acc-test"),
		emailRecipients: []string{"or.novogroder@coralogix.com"},
		severity:        selectRandomlyFromSlice(alertValidSeverities),
		activeWhen: activeWhen{
			daysOfWeek: selectManyRandomlyFromSlice(alertValidDaysOfWeek),
			activityStarts: activeHour{
				hour:   acctest.RandIntRange(0, 24),
				minute: acctest.RandIntRange(0, 60),
			},
		},
		notifyEverySec: acctest.RandIntRange(60, 3600),
	}

	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttrSet(resourceName, "id"),
		resource.TestCheckResourceAttr(resourceName, "enabled", "true"),
		resource.TestCheckResourceAttr(resourceName, "name", alert.name),
		resource.TestCheckResourceAttr(resourceName, "description", alert.description),
		resource.TestCheckResourceAttr(resourceName, "alert_severity", alert.severity),
		resource.TestCheckResourceAttr(resourceName, "notification.0.recipients.0.emails.0", alert.emailRecipients[0]),
		resource.TestCheckResourceAttr(resourceName, "notification.0.notify_every_sec", strconv.Itoa(alert.notifyEverySec)),
		resource.TestCheckResourceAttr(resourceName, "flow.0.stages.0.groups.0.sub_alerts.0.user_alert_id", "00bf3eb5-5681-4167-9611-ab0d6b902d84"),
		resource.TestCheckResourceAttr(resourceName, "flow.0.stages.1.time_window.0.hours", "0"),
		resource.TestCheckResourceAttr(resourceName, "flow.0.stages.1.time_window.0.minutes", "20"),
		resource.TestCheckResourceAttr(resourceName, "flow.0.stages.1.time_window.0.seconds", "0"),
		resource.TestCheckResourceAttr(resourceName, "flow.0.stages.1.groups.0.sub_alerts.0.user_alert_id", "d47a5aef-3fa3-4cdd-87df-9e0367372647"),
		resource.TestCheckResourceAttr(resourceName, "flow.0.stages.1.groups.0.sub_alerts.1.user_alert_id", "7a65d9fd-c52a-4eae-953e-6ac24558aa20"),
		resource.TestCheckResourceAttr(resourceName, "flow.0.stages.1.groups.0.operator", "OR"),
	}

	appendSchedulingChecks(checks, alert.daysOfWeek, resourceName)

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertFLow(&alert),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceAlert_update(t *testing.T) {
	resourceName := "coralogix_alert.test"

	alert1 := standardAlertTestParams{
		alertCommonTestParams: *getRandomAlert(),
		groupBy:               []string{"EventType"},
		occurrencesThreshold:  10,
		timeWindow:            selectRandomlyFromSlice(alertValidTimeFrames),
	}

	checks1 := extractCommonChecks(&alert1.alertCommonTestParams, resourceName, "standard")
	checks1 = append(checks1,
		resource.TestCheckResourceAttr(resourceName, "standard.0.condition.0.group_by.0", alert1.groupBy[0]),
	)

	alert2 := standardAlertTestParams{
		alertCommonTestParams: *getRandomAlert(),
		groupBy:               []string{"metadata.uid"},
		occurrencesThreshold:  10,
		timeWindow:            selectRandomlyFromSlice(alertValidTimeFrames),
	}

	checks2 := extractCommonChecks(&alert2.alertCommonTestParams, resourceName, "standard")
	checks2 = append(checks2,
		resource.TestCheckResourceAttr(resourceName, "standard.0.condition.0.group_by.0", alert2.groupBy[0]),
	)

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertStandard(&alert1),
				Check:  resource.ComposeAggregateTestCheckFunc(checks1...),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccCoralogixResourceAlertStandard(&alert2),
				Check:  resource.ComposeAggregateTestCheckFunc(checks2...),
			},
		},
	})
}

func getRandomAlert() *alertCommonTestParams {
	return &alertCommonTestParams{
		name:            acctest.RandomWithPrefix("tf-acc-test"),
		description:     acctest.RandomWithPrefix("tf-acc-test"),
		emailRecipients: []string{"or.novogroder@coralogix.com"},
		searchQuery:     "remote_addr_enriched:/.*/",
		severity:        selectRandomlyFromSlice(alertValidSeverities),
		activeWhen: activeWhen{
			daysOfWeek: selectManyRandomlyFromSlice(alertValidDaysOfWeek),
			activityStarts: activeHour{
				hour:   acctest.RandIntRange(0, 24),
				minute: acctest.RandIntRange(0, 60),
			},
		},
		notifyEverySec: acctest.RandIntRange(60, 3600),
		alertFilters: alertFilters{
			severities: selectManyRandomlyFromSlice(alertValidLogSeverities),
		},
	}
}

func extractCommonChecks(a *alertCommonTestParams, resourceName, alertType string) []resource.TestCheckFunc {
	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttrSet(resourceName, "id"),
		resource.TestCheckResourceAttr(resourceName, "enabled", "true"),
		resource.TestCheckResourceAttr(resourceName, "name", a.name),
		resource.TestCheckResourceAttr(resourceName, "description", a.description),
		resource.TestCheckResourceAttr(resourceName, "alert_severity", a.severity),
		resource.TestCheckResourceAttr(resourceName, "notification.0.recipients.0.emails.0", a.emailRecipients[0]),
		resource.TestCheckResourceAttr(resourceName, "notification.0.notify_every_sec", strconv.Itoa(a.notifyEverySec)),
		resource.TestCheckResourceAttr(resourceName, fmt.Sprintf("%s.0.search_query", alertType), a.searchQuery),
	}

	checks = appendSchedulingChecks(checks, a.daysOfWeek, resourceName)

	checks = appendSeveritiesCheck(checks, a.alertFilters.severities, resourceName, alertType)

	return checks
}

func appendSeveritiesCheck(checks []resource.TestCheckFunc, severities []string, resourceName string, alertType string) []resource.TestCheckFunc {
	for _, s := range severities {
		checks = append(checks,
			resource.TestCheckTypeSetElemAttr(resourceName, fmt.Sprintf("%s.0.severities.*", alertType), s))
	}
	return checks
}

func appendSchedulingChecks(checks []resource.TestCheckFunc, daysOfWeek []string, resourceName string) []resource.TestCheckFunc {
	for _, d := range daysOfWeek {
		checks = append(checks, resource.TestCheckTypeSetElemAttr(resourceName, "scheduling.0.days_enabled.*", d))
	}
	return checks
}

func testAccCheckAlertDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).Alerts()

	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_alert" {
			continue
		}

		req := &alertsv1.GetAlertRequest{
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
  alert_severity     = "%s"
  
  notification {
    recipients {
      emails      = %s
    }
    notify_every_sec = %d
	notify_only_on_triggered_group_by_values = true
  }

  scheduling {
    days_enabled = %s
    start_time = "%d:%d"
    end_time = "%d:%d"
  }

	meta_labels {
    	key   = "alert_type"
    	value = "security"
  	}
  	meta_labels {
    	key   = "security_severity"
    	value = "High"
  	}

  standard {
    severities = %s
    search_query = "%s"
    condition {
	  group_by = %s
      more_than = true
      occurrences_threshold = %d
      time_window = "%s"
    }
  }
}
`,
		a.name, a.description, a.severity, sliceToString(a.emailRecipients), a.notifyEverySec,
		sliceToString(a.daysOfWeek), a.activityStarts.hour, a.activityStarts.minute, a.activityEnds.hour, a.activityEnds.minute,
		sliceToString(a.severities), a.searchQuery, sliceToString(a.groupBy), a.occurrencesThreshold, a.timeWindow)
}

func testAccCoralogixResourceAlertRatio(a *ratioAlertTestParams) string {
	return fmt.Sprintf(`resource "coralogix_alert" "test" {
  name               = "%s"
  description        = "%s"
  alert_severity     = "%s"
  notification {
    recipients {
      emails      = %s
    }
    notify_every_sec = %d
    notify_only_on_triggered_group_by_values = true
  }

  scheduling {
    days_enabled = %s
    start_time = "%d:%d"
    end_time = "%d:%d"
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
      queries_ratio = %f
      time_window   = "%s"
      group_by      = %s
      group_by_q1   = true
    }
  }
}`,
		a.name, a.description, a.severity, sliceToString(a.emailRecipients), a.notifyEverySec,
		sliceToString(a.daysOfWeek), a.activityStarts.hour, a.activityStarts.minute, a.activityEnds.hour, a.activityEnds.minute,
		sliceToString(a.severities), a.searchQuery, sliceToString(a.q2Severities), a.q2SearchQuery,
		a.ratio, a.timeWindow, sliceToString(a.groupBy))
}

func testAccCoralogixResourceAlertNewValue(a *newValueAlertTestParams) string {
	return fmt.Sprintf(`resource "coralogix_alert" "test" {
  name               = "%s"
  description        = "%s"
  alert_severity     = "%s"
  notification {
    recipients {
      emails      = %s
    }
    notify_every_sec = %d
  }

  scheduling {
    days_enabled = %s
    start_time = "%d:%d"
    end_time = "%d:%d"
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
		a.name, a.description, a.severity, sliceToString(a.emailRecipients), a.notifyEverySec,
		sliceToString(a.daysOfWeek), a.activityStarts.hour, a.activityStarts.minute, a.activityEnds.hour, a.activityEnds.minute,
		sliceToString(a.severities), a.searchQuery, a.keyToTrack, a.timeWindow)
}

func testAccCoralogixResourceAlertUniqueCount(a *uniqueCountAlertTestParams) string {
	return fmt.Sprintf(`resource "coralogix_alert" "test" {
  name               = "%s"
  description        = "%s"
  alert_severity     = "%s"
  notification {
    recipients {
      emails      = %s
    }
    notify_every_sec = %d
  }

  scheduling {
    days_enabled = %s
    start_time = "%d:%d"
    end_time = "%d:%d"
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
		a.name, a.description, a.severity, sliceToString(a.emailRecipients), a.notifyEverySec,
		sliceToString(a.daysOfWeek), a.activityStarts.hour, a.activityStarts.minute, a.activityEnds.hour, a.activityEnds.minute,
		sliceToString(a.severities), a.searchQuery, a.uniqueCountKey, a.maxUniqueValues, a.timeWindow, a.groupByKey, a.maxUniqueValuesForGroupBy)
}

func testAccCoralogixResourceAlertTimeRelative(a *timeRelativeAlertTestParams) string {
	return fmt.Sprintf(`resource "coralogix_alert" "test" {
  name               = "%s"
  description        = "%s"
  alert_severity     = "%s"
  notification {
    recipients {
      emails      = %s
    }
    notify_every_sec = %d
	notify_only_on_triggered_group_by_values = true
  }

  scheduling {
    days_enabled = %s
    start_time = "%d:%d"
    end_time = "%d:%d"
  }

  time_relative {
    severities = %s
    search_query = "%s"
    condition {
      more_than            = true
      group_by             = %s
      ratio_threshold      = %d
      relative_time_window = "%s"
    }
  }
}`,
		a.name, a.description, a.severity, sliceToString(a.emailRecipients), a.notifyEverySec,
		sliceToString(a.daysOfWeek), a.activityStarts.hour, a.activityStarts.minute, a.activityEnds.hour, a.activityEnds.minute,
		sliceToString(a.severities), a.searchQuery, sliceToString(a.groupBy), a.ratioThreshold, a.relativeTimeWindow)
}

func testAccCoralogixResourceAlertMetricLucene(a *metricLuceneAlertTestParams) string {
	return fmt.Sprintf(`resource "coralogix_alert" "test" {
  name               = "%s"
  description        = "%s"
  alert_severity     = "%s"
  notification {
    recipients {
      emails      = %s
    }
    notify_every_sec = %d
	notify_only_on_triggered_group_by_values = true
  }

  scheduling {
    days_enabled = %s
    start_time = "%d:%d"
    end_time = "%d:%d"
  }

  metric {
    lucene {
      search_query = "%s"
      condition {
        metric_field                 = "%s"
        arithmetic_operator          = "%s"
        more_than                    = true
        threshold                    = %d
        arithmetic_operator_modifier = %d
        sample_threshold_percentage  = %d
        time_window                  = "%s"
		group_by = %s
      }
    }
  }
}`,
		a.name, a.description, a.severity, sliceToString(a.emailRecipients), a.notifyEverySec,
		sliceToString(a.daysOfWeek), a.activityStarts.hour, a.activityStarts.minute, a.activityEnds.hour,
		a.activityEnds.minute, a.searchQuery, a.metricField, a.arithmeticOperator, a.threshold,
		a.arithmeticOperatorModifier, a.sampleThresholdPercentage, a.timeWindow, sliceToString(a.groupBy))
}

func testAccCoralogixResourceAlertMetricPromql(a *metricPromqlAlertTestParams) string {
	return fmt.Sprintf(`resource "coralogix_alert" "test" {
  name               = "%s"
  description        = "%s"
  alert_severity     = "%s"
  notification {
    recipients {
      emails      = %s
    }
    notify_every_sec = %d
  }

  scheduling {
    days_enabled = %s
    start_time = "%d:%d"
    end_time = "%d:%d"
  }

  metric {
    promql {
      search_query = "%s"
      condition {
        more_than                    = true
        threshold                    = %d
        arithmetic_operator_modifier = %d
        sample_threshold_percentage  = %d
        time_window                  = "%s"
        min_non_null_values_percentage          = %d
      }
    }
  }
}`,
		a.name, a.description, a.severity, sliceToString(a.emailRecipients), a.notifyEverySec,
		sliceToString(a.daysOfWeek), a.activityStarts.hour, a.activityStarts.minute, a.activityEnds.hour, a.activityEnds.minute,
		a.searchQuery, a.threshold, a.arithmeticOperatorModifier, a.sampleThresholdPercentage, a.timeWindow, a.nonNullPercentage)
}

func testAccCoralogixResourceAlertTracing(a *tracingAlertTestParams) string {
	return fmt.Sprintf(`resource "coralogix_alert" "test" {
  name               = "%s"
  description        = "%s"
  alert_severity     = "%s"
  notification {
	//on_trigger_and_resolved = true
    recipients {
      emails      = %s
    }
    notify_every_sec = %d
  }

  scheduling {
    days_enabled = %s
    start_time = "%d:%d"
    end_time = "%d:%d"
  }

  tracing {
    severities           = %s
    latency_threshold_ms = %f
	field_filters {
      field = "Application"
      filters{
        values = ["nginx"]
        operator = "Equals"
      }
    }
    condition {
      more_than             = true
      time_window           = "%s"
      occurrences_threshold = %d
      group_by = %s
    }
  }
}`,
		a.name, a.description, a.severity, sliceToString(a.emailRecipients), a.notifyEverySec,
		sliceToString(a.daysOfWeek), a.activityStarts.hour, a.activityStarts.minute, a.activityEnds.hour, a.activityEnds.minute,
		sliceToString(a.severities), a.conditionLatencyMs, a.timeWindow, a.occurrencesThreshold, sliceToString(a.groupBy))
}

func testAccCoralogixResourceAlertFLow(a *flowAlertTestParams) string {
	return fmt.Sprintf(`resource "coralogix_alert" "test" {
  name               = "%s"
  description        = "%s"
  alert_severity     = "%s"
  notification {
    recipients {
      emails      = %s
    }
    notify_every_sec = %d
  }

  scheduling {
    days_enabled = %s
    start_time = "%d:%d"
    end_time = "%d:%d"
  }

  flow {
    stages {
      groups {
        sub_alerts {
          user_alert_id = "00bf3eb5-5681-4167-9611-ab0d6b902d84"
        }
        operator = "OR"
      }
    }
    stages {
      groups {
        sub_alerts {
          user_alert_id = "d47a5aef-3fa3-4cdd-87df-9e0367372647"
        }
        sub_alerts {
          user_alert_id = "7a65d9fd-c52a-4eae-953e-6ac24558aa20"
        }
        operator = "OR"
      }
      time_window {
        minutes = 20
      }
    }
  }
}`,
		a.name, a.description, a.severity, sliceToString(a.emailRecipients), a.notifyEverySec,
		sliceToString(a.daysOfWeek), a.activityStarts.hour, a.activityStarts.minute, a.activityEnds.hour, a.activityEnds.minute)
}

type standardAlertTestParams struct {
	groupBy              []string
	occurrencesThreshold int
	timeWindow           string
	alertCommonTestParams
}

type ratioAlertTestParams struct {
	q2Severities, groupBy     []string
	ratio                     float64
	timeWindow, q2SearchQuery string
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
}

type metricLuceneAlertTestParams struct {
	alertCommonTestParams
	groupBy                                                          []string
	metricField, timeWindow, arithmeticOperator                      string
	threshold, arithmeticOperatorModifier, sampleThresholdPercentage int
}

type metricPromqlAlertTestParams struct {
	alertCommonTestParams
	threshold, arithmeticOperatorModifier, nonNullPercentage, sampleThresholdPercentage int
	timeWindow                                                                          string
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
	notifyEverySec              int
	activeWhen
}

type alertCommonTestParams struct {
	name, description, severity string
	emailRecipients             []string
	notifyEverySec              int
	searchQuery                 string
	alertFilters
	activeWhen
}

type alertFilters struct {
	severities []string
}

type activeWhen struct {
	daysOfWeek                   []string
	activityStarts, activityEnds activeHour
}

type activeHour struct {
	hour, minute int
}
