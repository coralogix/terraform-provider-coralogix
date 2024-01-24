package coralogix

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"testing"

	"terraform-provider-coralogix/coralogix/clientset"
	alertsv1 "terraform-provider-coralogix/coralogix/clientset/grpc/alerts/v2"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var alertResourceName = "coralogix_alert.test"

func TestAccCoralogixResourceAlert_standard(t *testing.T) {
	alert := standardAlertTestParams{
		alertCommonTestParams: *getRandomAlert(),
		groupBy:               []string{"EventType"},
		occurrencesThreshold:  acctest.RandIntRange(1, 1000),
		timeWindow:            selectRandomlyFromSlice(alertValidTimeFrames),
		deadmanRatio:          selectRandomlyFromSlice(alertValidDeadmanRatioValues),
	}
	checks := extractStandardAlertChecks(alert)

	updatedAlert := standardAlertTestParams{
		alertCommonTestParams: *getRandomAlert(),
		groupBy:               []string{"EventType"},
		occurrencesThreshold:  acctest.RandIntRange(1, 1000),
		timeWindow:            selectRandomlyFromSlice(alertValidTimeFrames),
		deadmanRatio:          selectRandomlyFromSlice(alertValidDeadmanRatioValues),
	}
	updatedAlertChecks := extractStandardAlertChecks(updatedAlert)

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

func TestAccCoralogixResourceAlert_ratio(t *testing.T) {
	alert := ratioAlertTestParams{
		alertCommonTestParams: *getRandomAlert(),
		q2Severities:          selectManyRandomlyFromSlice(alertValidLogSeverities),
		timeWindow:            selectRandomlyFromSlice(alertValidTimeFrames),
		ratio:                 randFloat(),
		groupBy:               []string{"EventType"},
		q2SearchQuery:         "remote_addr_enriched:/.*/",
		ignoreInfinity:        randBool(),
	}
	checks := extractRatioAlertChecks(alert)

	updatedAlert := ratioAlertTestParams{
		alertCommonTestParams: *getRandomAlert(),
		q2Severities:          selectManyRandomlyFromSlice(alertValidLogSeverities),
		timeWindow:            selectRandomlyFromSlice(alertValidTimeFrames),
		ratio:                 randFloat(),
		groupBy:               []string{"EventType"},
		q2SearchQuery:         "remote_addr_enriched:/.*/",
		ignoreInfinity:        randBool(),
	}
	updatedAlertChecks := extractRatioAlertChecks(updatedAlert)

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
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertRatio(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_newValue(t *testing.T) {
	alert := newValueAlertTestParams{
		alertCommonTestParams: *getRandomAlert(),
		keyToTrack:            "EventType",
		timeWindow:            selectRandomlyFromSlice(alertValidNewValueTimeFrames),
	}
	checks := extractNewValueChecks(alert)

	updatedAlert := newValueAlertTestParams{
		alertCommonTestParams: *getRandomAlert(),
		keyToTrack:            "EventType",
		timeWindow:            selectRandomlyFromSlice(alertValidNewValueTimeFrames),
	}
	updatedAlertChecks := extractNewValueChecks(updatedAlert)

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
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertNewValue(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_uniqueCount(t *testing.T) {
	alert := uniqueCountAlertTestParams{
		alertCommonTestParams:     *getRandomAlert(),
		uniqueCountKey:            "EventType",
		timeWindow:                selectRandomlyFromSlice(alertValidUniqueCountTimeFrames),
		groupByKey:                "metadata.name",
		maxUniqueValues:           2,
		maxUniqueValuesForGroupBy: 20,
	}
	checks := extractUniqueCountAlertChecks(alert)

	updatedAlert := uniqueCountAlertTestParams{
		alertCommonTestParams:     *getRandomAlert(),
		uniqueCountKey:            "EventType",
		timeWindow:                selectRandomlyFromSlice(alertValidUniqueCountTimeFrames),
		groupByKey:                "metadata.name",
		maxUniqueValues:           2,
		maxUniqueValuesForGroupBy: 20,
	}
	updatedAlertChecks := extractUniqueCountAlertChecks(updatedAlert)

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
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertUniqueCount(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_timeRelative(t *testing.T) {
	alert := timeRelativeAlertTestParams{
		alertCommonTestParams: *getRandomAlert(),
		ratioThreshold:        acctest.RandIntRange(0, 1000),
		relativeTimeWindow:    selectRandomlyFromSlice(alertValidRelativeTimeFrames),
		groupBy:               []string{"EventType"},
		ignoreInfinity:        randBool(),
	}
	checks := extractTimeRelativeChecks(alert)

	updatedAlert := timeRelativeAlertTestParams{
		alertCommonTestParams: *getRandomAlert(),
		ratioThreshold:        acctest.RandIntRange(0, 1000),
		relativeTimeWindow:    selectRandomlyFromSlice(alertValidRelativeTimeFrames),
		groupBy:               []string{"EventType"},
		ignoreInfinity:        randBool(),
	}
	updatedAlertChecks := extractTimeRelativeChecks(updatedAlert)

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
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertTimeRelative(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_metricLucene(t *testing.T) {
	alert := metricLuceneAlertTestParams{
		alertCommonTestParams: *getRandomAlert(),
		groupBy:               []string{"EventType"},
		metricField:           "subsystem",
		timeWindow:            selectRandomlyFromSlice(alertValidMetricTimeFrames),
		threshold:             acctest.RandIntRange(0, 1000),
		arithmeticOperator:    selectRandomlyFromSlice(alertValidArithmeticOperators),
	}
	if alert.arithmeticOperator == "Percentile" {
		alert.arithmeticOperatorModifier = acctest.RandIntRange(0, 100)
	}
	checks := extractLuceneMetricChecks(alert)

	updatedAlert := metricLuceneAlertTestParams{
		alertCommonTestParams: *getRandomAlert(),
		groupBy:               []string{"EventType"},
		metricField:           "subsystem",
		timeWindow:            selectRandomlyFromSlice(alertValidMetricTimeFrames),
		threshold:             acctest.RandIntRange(0, 1000),
		arithmeticOperator:    selectRandomlyFromSlice(alertValidArithmeticOperators),
	}
	if updatedAlert.arithmeticOperator == "Percentile" {
		updatedAlert.arithmeticOperatorModifier = acctest.RandIntRange(0, 100)
	}
	updatedAlertChecks := extractLuceneMetricChecks(updatedAlert)

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
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertMetricLucene(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_metricPromql(t *testing.T) {
	alert := metricPromqlAlertTestParams{
		alertCommonTestParams: *getRandomAlert(),
		threshold:             acctest.RandIntRange(0, 1000),
		nonNullPercentage:     10 * acctest.RandIntRange(0, 10),
		timeWindow:            selectRandomlyFromSlice(alertValidMetricTimeFrames),
		condition:             "less_than",
	}
	checks := extractMetricPromqlAlertChecks(alert)

	updatedAlert := metricPromqlAlertTestParams{
		alertCommonTestParams: *getRandomAlert(),
		threshold:             acctest.RandIntRange(0, 1000),
		nonNullPercentage:     10 * acctest.RandIntRange(0, 10),
		timeWindow:            selectRandomlyFromSlice(alertValidMetricTimeFrames),
		condition:             "more_than",
	}
	updatedAlertChecks := extractMetricPromqlAlertChecks(updatedAlert)

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
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertMetricPromql(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_tracing(t *testing.T) {
	alert := tracingAlertTestParams{
		alertCommonTestParams: *getRandomAlert(),
		conditionLatencyMs:    math.Round(randFloat()*1000) / 1000,
		occurrencesThreshold:  acctest.RandIntRange(1, 10000),
		timeWindow:            selectRandomlyFromSlice(alertValidTimeFrames),
		groupBy:               []string{"EventType"},
	}
	checks := extractTracingAlertChecks(alert)

	updatedAlert := tracingAlertTestParams{
		alertCommonTestParams: *getRandomAlert(),
		conditionLatencyMs:    math.Round(randFloat()*1000) / 1000,
		occurrencesThreshold:  acctest.RandIntRange(1, 10000),
		timeWindow:            selectRandomlyFromSlice(alertValidTimeFrames),
		groupBy:               []string{"EventType"},
	}
	updatedAlertChecks := extractTracingAlertChecks(updatedAlert)

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
				ResourceName: alertResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertTracing(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},
		},
	})
}

func TestAccCoralogixResourceAlert_flow(t *testing.T) {
	resourceName := "coralogix_alert.test"

	alert := flowAlertTestParams{
		name:            acctest.RandomWithPrefix("tf-acc-test"),
		description:     acctest.RandomWithPrefix("tf-acc-test"),
		emailRecipients: []string{"user@example.com"},
		webhookID:       "8358",
		severity:        selectRandomlyFromSlice(alertValidSeverities),
		activeWhen:      randActiveWhen(),
		notifyEveryMin:  acctest.RandIntRange(1500 /*to avoid notify_every < condition.0.time_window*/, 3600),
		notifyOn:        selectRandomlyFromSlice(validNotifyOn),
	}
	checks := extractFlowAlertChecks(alert)

	updatedAlert := flowAlertTestParams{
		name:            acctest.RandomWithPrefix("tf-acc-test"),
		description:     acctest.RandomWithPrefix("tf-acc-test"),
		emailRecipients: []string{"user@example.com"},
		webhookID:       "8358",
		severity:        selectRandomlyFromSlice(alertValidSeverities),
		activeWhen:      randActiveWhen(),
		notifyEveryMin:  acctest.RandIntRange(1500 /*to avoid notify_every < condition.0.time_window*/, 3600),
		notifyOn:        selectRandomlyFromSlice(validNotifyOn),
	}
	updatedAlertChecks := extractFlowAlertChecks(updatedAlert)

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
				ResourceName: resourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAlertFLow(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},
		},
	})
}

func getRandomAlert() *alertCommonTestParams {
	return &alertCommonTestParams{
		name:            acctest.RandomWithPrefix("tf-acc-test"),
		description:     acctest.RandomWithPrefix("tf-acc-test"),
		webhookID:       "8358",
		emailRecipients: []string{"user@example.com"},
		searchQuery:     "remote_addr_enriched:/.*/",
		severity:        selectRandomlyFromSlice(alertValidSeverities),
		activeWhen:      randActiveWhen(),
		notifyEveryMin:  acctest.RandIntRange(2160 /*to avoid notify_every < condition.0.time_window*/, 3600),
		notifyOn:        selectRandomlyFromSlice(validNotifyOn),
		alertFilters: alertFilters{
			severities: selectManyRandomlyFromSlice(alertValidLogSeverities),
		},
	}
}

func extractStandardAlertChecks(alert standardAlertTestParams) []resource.TestCheckFunc {
	checks := extractCommonChecks(&alert.alertCommonTestParams, "standard")
	checks = append(checks,
		resource.TestCheckResourceAttr(alertResourceName, "meta_labels.alert_type", "security"),
		resource.TestCheckResourceAttr(alertResourceName, "meta_labels.security_severity", "high"),
		resource.TestCheckResourceAttr(alertResourceName, "standard.0.condition.0.threshold", strconv.Itoa(alert.occurrencesThreshold)),
		resource.TestCheckResourceAttr(alertResourceName, "standard.0.condition.0.time_window", alert.timeWindow),
		resource.TestCheckResourceAttr(alertResourceName, "standard.0.condition.0.group_by.0", alert.groupBy[0]),
		resource.TestCheckResourceAttr(alertResourceName, "standard.0.condition.0.less_than", "true"),
		resource.TestCheckResourceAttr(alertResourceName, "standard.0.condition.0.manage_undetected_values.0.enable_triggering_on_undetected_values", "true"),
		resource.TestCheckResourceAttr(alertResourceName, "standard.0.condition.0.manage_undetected_values.0.auto_retire_ratio", alert.deadmanRatio),
	)
	return checks
}

func extractRatioAlertChecks(alert ratioAlertTestParams) []resource.TestCheckFunc {
	checks := extractCommonChecks(&alert.alertCommonTestParams, "ratio.0.query_1")
	checks = append(checks,
		resource.TestCheckResourceAttr(alertResourceName, "ratio.0.query_2.0.search_query", alert.q2SearchQuery),
		resource.TestCheckResourceAttr(alertResourceName, "ratio.0.condition.0.more_than", "true"),
		resource.TestCheckResourceAttr(alertResourceName, "ratio.0.condition.0.ratio_threshold", fmt.Sprintf("%f", alert.ratio)),
		resource.TestCheckResourceAttr(alertResourceName, "ratio.0.condition.0.time_window", alert.timeWindow),
		resource.TestCheckResourceAttr(alertResourceName, "ratio.0.condition.0.group_by.0", alert.groupBy[0]),
		resource.TestCheckResourceAttr(alertResourceName, "ratio.0.condition.0.group_by_q1", "true"),
		resource.TestCheckResourceAttr(alertResourceName, "ratio.0.condition.0.ignore_infinity", fmt.Sprintf("%t", alert.ignoreInfinity)),
	)
	checks = appendSeveritiesCheck(checks, alert.alertFilters.severities, "ratio.0.query_2")

	return checks
}

func extractNewValueChecks(alert newValueAlertTestParams) []resource.TestCheckFunc {
	checks := extractCommonChecks(&alert.alertCommonTestParams, "new_value")
	checks = append(checks,
		resource.TestCheckResourceAttr(alertResourceName, "new_value.0.condition.0.key_to_track", alert.keyToTrack),
		resource.TestCheckResourceAttr(alertResourceName, "new_value.0.condition.0.time_window", alert.timeWindow),
	)
	return checks
}

func extractUniqueCountAlertChecks(alert uniqueCountAlertTestParams) []resource.TestCheckFunc {
	checks := extractCommonChecks(&alert.alertCommonTestParams, "unique_count")
	checks = append(checks,
		resource.TestCheckResourceAttr(alertResourceName, "unique_count.0.condition.0.unique_count_key", alert.uniqueCountKey),
		resource.TestCheckResourceAttr(alertResourceName, "unique_count.0.condition.0.unique_count_key", alert.uniqueCountKey),
		resource.TestCheckResourceAttr(alertResourceName, "unique_count.0.condition.0.time_window", alert.timeWindow),
		resource.TestCheckResourceAttr(alertResourceName, "unique_count.0.condition.0.max_unique_values", strconv.Itoa(alert.maxUniqueValues)),
		resource.TestCheckResourceAttr(alertResourceName, "unique_count.0.condition.0.max_unique_values_for_group_by", strconv.Itoa(alert.maxUniqueValuesForGroupBy)),
	)
	return checks
}

func extractTimeRelativeChecks(alert timeRelativeAlertTestParams) []resource.TestCheckFunc {
	checks := extractCommonChecks(&alert.alertCommonTestParams, "time_relative")
	checks = append(checks,
		resource.TestCheckResourceAttr(alertResourceName, "time_relative.0.condition.0.ratio_threshold", strconv.Itoa(alert.ratioThreshold)),
		resource.TestCheckResourceAttr(alertResourceName, "time_relative.0.condition.0.relative_time_window", alert.relativeTimeWindow),
		resource.TestCheckResourceAttr(alertResourceName, "time_relative.0.condition.0.group_by.0", alert.groupBy[0]),
		resource.TestCheckResourceAttr(alertResourceName, "time_relative.0.condition.0.ignore_infinity", fmt.Sprintf("%t", alert.ignoreInfinity)),
	)

	return checks
}

func extractLuceneMetricChecks(alert metricLuceneAlertTestParams) []resource.TestCheckFunc {
	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttrSet(alertResourceName, "id"),
		resource.TestCheckResourceAttr(alertResourceName, "enabled", "true"),
		resource.TestCheckResourceAttr(alertResourceName, "name", alert.name),
		resource.TestCheckResourceAttr(alertResourceName, "description", alert.description),
		resource.TestCheckResourceAttr(alertResourceName, "severity", alert.severity),
		resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notifications_group.0.notification.*",
			map[string]string{
				"integration_id": alert.webhookID,
			}),
		resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notifications_group.0.notification.*",
			map[string]string{
				"email_recipients.0": alert.emailRecipients[0],
			}),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.lucene.0.search_query", alert.searchQuery),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.lucene.0.condition.0.metric_field", alert.metricField),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.lucene.0.condition.0.arithmetic_operator", alert.arithmeticOperator),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.lucene.0.condition.0.less_than", "true"),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.lucene.0.condition.0.threshold", strconv.Itoa(alert.threshold)),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.lucene.0.condition.0.arithmetic_operator_modifier", strconv.Itoa(alert.arithmeticOperatorModifier)),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.lucene.0.condition.0.sample_threshold_percentage", strconv.Itoa(alert.sampleThresholdPercentage)),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.lucene.0.condition.0.time_window", alert.timeWindow),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.lucene.0.condition.0.group_by.0", alert.groupBy[0]),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.lucene.0.condition.0.manage_undetected_values.0.enable_triggering_on_undetected_values", "false"),
	}
	checks = appendSchedulingChecks(checks, alert.daysOfWeek, alert.activityStarts, alert.activityEnds)
	return checks
}

func extractMetricPromqlAlertChecks(alert metricPromqlAlertTestParams) []resource.TestCheckFunc {
	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttrSet(alertResourceName, "id"),
		resource.TestCheckResourceAttr(alertResourceName, "enabled", "true"),
		resource.TestCheckResourceAttr(alertResourceName, "name", alert.name),
		resource.TestCheckResourceAttr(alertResourceName, "description", alert.description),
		resource.TestCheckResourceAttr(alertResourceName, "severity", alert.severity),
		resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notifications_group.0.notification.*",
			map[string]string{
				"integration_id": alert.webhookID,
			}),
		resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notifications_group.0.notification.*",
			map[string]string{
				"email_recipients.0": alert.emailRecipients[0],
			}),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.promql.0.search_query", "http_requests_total{status!~\"4..\"}"),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.promql.0.condition.0.threshold", strconv.Itoa(alert.threshold)),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.promql.0.condition.0.sample_threshold_percentage", strconv.Itoa(alert.sampleThresholdPercentage)),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.promql.0.condition.0.min_non_null_values_percentage", strconv.Itoa(alert.nonNullPercentage)),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.promql.0.condition.0.time_window", alert.timeWindow),
	}
	if alert.condition == "less_than" {
		checks = append(checks,
			resource.TestCheckResourceAttr(alertResourceName, "metric.0.promql.0.condition.0.less_than", "true"),
			resource.TestCheckResourceAttr(alertResourceName, "metric.0.promql.0.condition.0.manage_undetected_values.0.enable_triggering_on_undetected_values", "true"),
			resource.TestCheckResourceAttr(alertResourceName, "metric.0.promql.0.condition.0.manage_undetected_values.0.auto_retire_ratio", "Never"),
		)
	} else {
		checks = append(checks,
			resource.TestCheckResourceAttr(alertResourceName, "metric.0.promql.0.condition.0.more_than", "true"),
		)
	}
	checks = appendSchedulingChecks(checks, alert.daysOfWeek, alert.activityStarts, alert.activityEnds)
	return checks
}

func extractTracingAlertChecks(alert tracingAlertTestParams) []resource.TestCheckFunc {
	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttrSet(alertResourceName, "id"),
		resource.TestCheckResourceAttr(alertResourceName, "enabled", "true"),
		resource.TestCheckResourceAttr(alertResourceName, "name", alert.name),
		resource.TestCheckResourceAttr(alertResourceName, "description", alert.description),
		resource.TestCheckResourceAttr(alertResourceName, "severity", alert.severity),
		resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notifications_group.0.notification.*",
			map[string]string{
				"integration_id":              alert.webhookID,
				"retriggering_period_minutes": fmt.Sprintf("%d", alert.notifyEveryMin),
			}),
		resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notifications_group.0.notification.*",
			map[string]string{
				"email_recipients.0":          alert.emailRecipients[0],
				"notify_on":                   "Triggered_and_resolved",
				"retriggering_period_minutes": fmt.Sprintf("%d", alert.notifyEveryMin),
			}),
		resource.TestCheckResourceAttr(alertResourceName, "tracing.0.latency_threshold_milliseconds", fmt.Sprintf("%.3f", alert.conditionLatencyMs)),
		resource.TestCheckResourceAttr(alertResourceName, "tracing.0.condition.0.more_than", "true"),
		resource.TestCheckResourceAttr(alertResourceName, "tracing.0.condition.0.time_window", alert.timeWindow),
		resource.TestCheckResourceAttr(alertResourceName, "tracing.0.condition.0.threshold", strconv.Itoa(alert.occurrencesThreshold)),
		resource.TestCheckResourceAttr(alertResourceName, "tracing.0.applications.0", "nginx"),
		resource.TestCheckResourceAttr(alertResourceName, "tracing.0.subsystems.0", "subsystem-name"),
		resource.TestCheckResourceAttr(alertResourceName, "tracing.0.tag_filter.0.field", "Status"),
		resource.TestCheckTypeSetElemAttr(alertResourceName, "tracing.0.tag_filter.0.values.*", "filter:contains:400"),
		resource.TestCheckTypeSetElemAttr(alertResourceName, "tracing.0.tag_filter.0.values.*", "500"),
	}
	checks = appendSchedulingChecks(checks, alert.daysOfWeek, alert.activityStarts, alert.activityEnds)
	return checks
}

func extractFlowAlertChecks(alert flowAlertTestParams) []resource.TestCheckFunc {
	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttrSet(alertResourceName, "id"),
		resource.TestCheckResourceAttr(alertResourceName, "enabled", "true"),
		resource.TestCheckResourceAttr(alertResourceName, "name", alert.name),
		resource.TestCheckResourceAttr(alertResourceName, "description", alert.description),
		resource.TestCheckResourceAttr(alertResourceName, "severity", alert.severity),
		resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notifications_group.0.notification.*",
			map[string]string{
				"integration_id": alert.webhookID,
			}),
		resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notifications_group.0.notification.*",
			map[string]string{
				"email_recipients.0": alert.emailRecipients[0],
			}),
		resource.TestCheckResourceAttr(alertResourceName, "incident_settings.0.notify_on", alert.notifyOn),
		resource.TestCheckResourceAttr(alertResourceName, "incident_settings.0.retriggering_period_minutes", strconv.Itoa(alert.notifyEveryMin)),
		resource.TestCheckResourceAttr(alertResourceName, "flow.0.stage.0.group.0.sub_alerts.0.operator", "OR"),
		resource.TestCheckResourceAttr(alertResourceName, "flow.0.stage.0.group.0.next_operator", "OR"),
		resource.TestCheckResourceAttr(alertResourceName, "flow.0.stage.0.group.1.sub_alerts.0.operator", "AND"),
		resource.TestCheckResourceAttr(alertResourceName, "flow.0.stage.0.group.1.sub_alerts.0.flow_alert.0.not", "true"),
		resource.TestCheckResourceAttr(alertResourceName, "flow.0.stage.0.group.1.next_operator", "AND"),
		resource.TestCheckResourceAttr(alertResourceName, "flow.0.stage.0.time_window.0.minutes", "20"),
		resource.TestCheckResourceAttr(alertResourceName, "flow.0.group_by.0", "coralogix.metadata.sdkId"),
	}
	checks = appendSchedulingChecks(checks, alert.daysOfWeek, alert.activityStarts, alert.activityEnds)
	return checks
}

func extractCommonChecks(alert *alertCommonTestParams, alertType string) []resource.TestCheckFunc {
	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttrSet(alertResourceName, "id"),
		resource.TestCheckResourceAttr(alertResourceName, "enabled", "true"),
		resource.TestCheckResourceAttr(alertResourceName, "name", alert.name),
		resource.TestCheckResourceAttr(alertResourceName, "description", alert.description),
		resource.TestCheckResourceAttr(alertResourceName, "severity", alert.severity),
		resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notifications_group.0.notification.*",
			map[string]string{
				"integration_id": alert.webhookID,
			}),
		resource.TestCheckTypeSetElemNestedAttrs(alertResourceName, "notifications_group.0.notification.*",
			map[string]string{
				"email_recipients.0": alert.emailRecipients[0],
			}),
		resource.TestCheckResourceAttr(alertResourceName, "incident_settings.0.notify_on", alert.notifyOn),
		resource.TestCheckResourceAttr(alertResourceName, "incident_settings.0.retriggering_period_minutes", strconv.Itoa(alert.notifyEveryMin)),
		resource.TestCheckResourceAttr(alertResourceName, fmt.Sprintf("%s.0.search_query", alertType), alert.searchQuery),
	}

	checks = appendSchedulingChecks(checks, alert.daysOfWeek, alert.activityStarts, alert.activityEnds)

	checks = appendSeveritiesCheck(checks, alert.alertFilters.severities, alertType)

	return checks
}

func appendSeveritiesCheck(checks []resource.TestCheckFunc, severities []string, alertType string) []resource.TestCheckFunc {
	for _, s := range severities {
		checks = append(checks,
			resource.TestCheckTypeSetElemAttr(alertResourceName, fmt.Sprintf("%s.0.severities.*", alertType), s))
	}
	return checks
}

func appendSchedulingChecks(checks []resource.TestCheckFunc, daysOfWeek []string, startTime, endTime string) []resource.TestCheckFunc {
	for _, d := range daysOfWeek {
		checks = append(checks, resource.TestCheckTypeSetElemAttr(alertResourceName, "scheduling.0.time_frame.0.days_enabled.*", d))
	}
	checks = append(checks, resource.TestCheckResourceAttr(alertResourceName, "scheduling.0.time_frame.0.start_time", startTime))
	checks = append(checks, resource.TestCheckResourceAttr(alertResourceName, "scheduling.0.time_frame.0.end_time", endTime))
	return checks
}

func testAccCheckAlertDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).Alerts()

	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_alert" {
			continue
		}

		req := &alertsv1.GetAlertByUniqueIdRequest{
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
