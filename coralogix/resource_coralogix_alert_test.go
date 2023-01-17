package coralogix

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"terraform-provider-coralogix/coralogix/clientset"
	alertsv1 "terraform-provider-coralogix/coralogix/clientset/grpc/alerts/v1"
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
	}
	checks := extractRatioAlertChecks(alert)

	updatedAlert := ratioAlertTestParams{
		alertCommonTestParams: *getRandomAlert(),
		q2Severities:          selectManyRandomlyFromSlice(alertValidLogSeverities),
		timeWindow:            selectRandomlyFromSlice(alertValidTimeFrames),
		ratio:                 randFloat(),
		groupBy:               []string{"EventType"},
		q2SearchQuery:         "remote_addr_enriched:/.*/",
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
	}
	checks := extractTimeRelativeChecks(alert)

	updatedAlert := timeRelativeAlertTestParams{
		alertCommonTestParams: *getRandomAlert(),
		ratioThreshold:        acctest.RandIntRange(0, 1000),
		relativeTimeWindow:    selectRandomlyFromSlice(alertValidRelativeTimeFrames),
		groupBy:               []string{"EventType"},
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

	/*updatedAlert := metricLuceneAlertTestParams{
		alertCommonTestParams: *getRandomAlert(),
		groupBy:               []string{"EventType"},
		metricField:           "subsystem",
		timeWindow:            selectRandomlyFromSlice(alertValidMetricTimeFrames),
		threshold:             acctest.RandIntRange(0, 1000),
		arithmeticOperator:    selectRandomlyFromSlice(alertValidArithmeticOperators),
	}
	if updatedAlert.arithmeticOperator == "Percentile" {
		alert.arithmeticOperatorModifier = acctest.RandIntRange(0, 100)
	}
	updatedAlertChecks := extractLuceneMetricChecks(updatedAlert)*/

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
			/*{
				Config: testAccCoralogixResourceAlertMetricLucene(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},*/
		},
	})
}

func TestAccCoralogixResourceAlert_metricPromql(t *testing.T) {
	alert := metricPromqlAlertTestParams{
		alertCommonTestParams: *getRandomAlert(),
		threshold:             acctest.RandIntRange(0, 1000),
		nonNullPercentage:     acctest.RandIntRange(0, 100),
		timeWindow:            selectRandomlyFromSlice(alertValidMetricTimeFrames),
	}
	checks := extractMetricPromqlAlertChecks(alert)

	/*updatedAlert := metricPromqlAlertTestParams{
		alertCommonTestParams: *getRandomAlert(),
		threshold:             acctest.RandIntRange(0, 1000),
		nonNullPercentage:     acctest.RandIntRange(0, 100),
		timeWindow:            selectRandomlyFromSlice(alertValidMetricTimeFrames),
	}
	updatedAlertChecks := extractMetricPromqlAlertChecks(updatedAlert)*/

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
			/*{
				Config: testAccCoralogixResourceAlertMetricPromql(&updatedAlert),
				Check:  resource.ComposeAggregateTestCheckFunc(updatedAlertChecks...),
			},*/
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
		severity:        selectRandomlyFromSlice(alertValidSeverities),
		activeWhen:      randActiveWhen(),
		notifyEveryMin:  acctest.RandIntRange(1500 /*to avoid notify_every < condition.0.time_window*/, 3600),
	}
	checks := extractFlowAlertChecks(alert)

	updatedAlert := flowAlertTestParams{
		name:            acctest.RandomWithPrefix("tf-acc-test"),
		description:     acctest.RandomWithPrefix("tf-acc-test"),
		emailRecipients: []string{"user@example.com"},
		severity:        selectRandomlyFromSlice(alertValidSeverities),
		activeWhen:      randActiveWhen(),
		notifyEveryMin:  acctest.RandIntRange(1500 /*to avoid notify_every < condition.0.time_window*/, 3600),
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
		emailRecipients: []string{"user@example.com"},
		searchQuery:     "remote_addr_enriched:/.*/",
		severity:        selectRandomlyFromSlice(alertValidSeverities),
		activeWhen:      randActiveWhen(),
		notifyEveryMin:  acctest.RandIntRange(2160 /*to avoid notify_every < condition.0.time_window*/, 3600),
		alertFilters: alertFilters{
			severities: selectManyRandomlyFromSlice(alertValidLogSeverities),
		},
	}
}

func extractStandardAlertChecks(alert standardAlertTestParams) []resource.TestCheckFunc {
	checks := extractCommonChecks(&alert.alertCommonTestParams, "standard")
	checks = append(checks,
		resource.TestCheckResourceAttr(alertResourceName, "notification.0.notify_only_on_triggered_group_by_values", "true"),
		resource.TestCheckResourceAttr(alertResourceName, "meta_labels.0.key", "alert_type"),
		resource.TestCheckResourceAttr(alertResourceName, "meta_labels.0.value", "security"),
		resource.TestCheckResourceAttr(alertResourceName, "meta_labels.1.key", "security_severity"),
		resource.TestCheckResourceAttr(alertResourceName, "meta_labels.1.value", "High"),
		resource.TestCheckResourceAttr(alertResourceName, "standard.0.condition.0.occurrences_threshold", strconv.Itoa(alert.occurrencesThreshold)),
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
		resource.TestCheckResourceAttr(alertResourceName, "notification.0.notify_only_on_triggered_group_by_values", "true"),
		resource.TestCheckResourceAttr(alertResourceName, "ratio.0.query_2.0.search_query", alert.q2SearchQuery),
		resource.TestCheckResourceAttr(alertResourceName, "ratio.0.condition.0.more_than", "true"),
		resource.TestCheckResourceAttr(alertResourceName, "ratio.0.condition.0.queries_ratio", fmt.Sprintf("%f", alert.ratio)),
		resource.TestCheckResourceAttr(alertResourceName, "ratio.0.condition.0.time_window", alert.timeWindow),
		resource.TestCheckResourceAttr(alertResourceName, "ratio.0.condition.0.group_by.0", alert.groupBy[0]),
		resource.TestCheckResourceAttr(alertResourceName, "ratio.0.condition.0.group_by_q1", "true"),
	)
	for i, s := range alert.q2Severities {
		checks = append(checks,
			resource.TestCheckResourceAttr(alertResourceName,
				fmt.Sprintf("ratio.0.query_2.0.severities.%d", i), s))
	}
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
		resource.TestCheckResourceAttr(alertResourceName, "notification.0.notify_only_on_triggered_group_by_values", "true"),
		resource.TestCheckResourceAttr(alertResourceName, "time_relative.0.condition.0.ratio_threshold", strconv.Itoa(alert.ratioThreshold)),
		resource.TestCheckResourceAttr(alertResourceName, "time_relative.0.condition.0.relative_time_window", alert.relativeTimeWindow),
		resource.TestCheckResourceAttr(alertResourceName, "time_relative.0.condition.0.group_by.0", alert.groupBy[0]),
	)
	return checks
}

func extractLuceneMetricChecks(alert metricLuceneAlertTestParams) []resource.TestCheckFunc {
	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttrSet(alertResourceName, "id"),
		resource.TestCheckResourceAttr(alertResourceName, "enabled", "true"),
		resource.TestCheckResourceAttr(alertResourceName, "name", alert.name),
		resource.TestCheckResourceAttr(alertResourceName, "description", alert.description),
		resource.TestCheckResourceAttr(alertResourceName, "alert_severity", alert.severity),
		resource.TestCheckResourceAttr(alertResourceName, "notification.0.notify_only_on_triggered_group_by_values", "true"),
		resource.TestCheckResourceAttr(alertResourceName, "notification.0.recipients.0.emails.0", alert.emailRecipients[0]),
		resource.TestCheckResourceAttr(alertResourceName, "notification.0.notify_every_min", strconv.Itoa(alert.notifyEveryMin)),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.lucene.0.search_query", alert.searchQuery),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.lucene.0.condition.0.metric_field", alert.metricField),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.lucene.0.condition.0.arithmetic_operator", alert.arithmeticOperator),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.lucene.0.condition.0.less_than", "true"),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.lucene.0.condition.0.threshold", strconv.Itoa(alert.threshold)),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.lucene.0.condition.0.arithmetic_operator_modifier", strconv.Itoa(alert.arithmeticOperatorModifier)),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.lucene.0.condition.0.sample_threshold_percentage", strconv.Itoa(alert.sampleThresholdPercentage)),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.lucene.0.condition.0.time_window", alert.timeWindow),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.lucene.0.condition.0.group_by.0", alert.groupBy[0]),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.lucene.0.condition.0.manage_undetected_values.0.disable_triggering_on_undetected_values", "true"),
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
		resource.TestCheckResourceAttr(alertResourceName, "alert_severity", alert.severity),
		resource.TestCheckResourceAttr(alertResourceName, "notification.0.recipients.0.emails.0", alert.emailRecipients[0]),
		resource.TestCheckResourceAttr(alertResourceName, "notification.0.notify_every_min", strconv.Itoa(alert.notifyEveryMin)),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.promql.0.search_query", "http_requests_total{status!~\"4..\"}"),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.promql.0.condition.0.threshold", strconv.Itoa(alert.threshold)),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.promql.0.condition.0.less_than", "true"),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.promql.0.condition.0.sample_threshold_percentage", strconv.Itoa(alert.sampleThresholdPercentage)),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.promql.0.condition.0.min_non_null_values_percentage", strconv.Itoa(alert.nonNullPercentage)),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.promql.0.condition.0.time_window", alert.timeWindow),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.promql.0.condition.0.manage_undetected_values.0.enable_triggering_on_undetected_values", "true"),
		resource.TestCheckResourceAttr(alertResourceName, "metric.0.promql.0.condition.0.manage_undetected_values.0.auto_retire_ratio", "Never"),
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
		resource.TestCheckResourceAttr(alertResourceName, "alert_severity", alert.severity),
		resource.TestCheckResourceAttr(alertResourceName, "notification.0.recipients.0.emails.0", alert.emailRecipients[0]),
		resource.TestCheckResourceAttr(alertResourceName, "notification.0.notify_every_min", strconv.Itoa(alert.notifyEveryMin)),
		resource.TestCheckResourceAttr(alertResourceName, "notification.0.on_trigger_and_resolved", "true"),
		resource.TestCheckResourceAttr(alertResourceName, "tracing.0.latency_threshold_ms", fmt.Sprintf("%.3f", alert.conditionLatencyMs)),
		resource.TestCheckResourceAttr(alertResourceName, "tracing.0.condition.0.more_than", "true"),
		resource.TestCheckResourceAttr(alertResourceName, "tracing.0.condition.0.time_window", alert.timeWindow),
		resource.TestCheckResourceAttr(alertResourceName, "tracing.0.condition.0.occurrences_threshold", strconv.Itoa(alert.occurrencesThreshold)),
		resource.TestCheckResourceAttr(alertResourceName, "tracing.0.field_filters.0.field", "Application"),
		resource.TestCheckResourceAttr(alertResourceName, "tracing.0.field_filters.0.filters.0.operator", "Equals"),
		resource.TestCheckResourceAttr(alertResourceName, "tracing.0.field_filters.0.filters.0.values.0", "nginx"),
	}
	checks = appendSchedulingChecks(checks, alert.daysOfWeek, alert.activityStarts, alert.activityEnds)
	checks = appendSeveritiesCheck(checks, alert.alertFilters.severities, "tracing")
	return checks
}

func extractFlowAlertChecks(alert flowAlertTestParams) []resource.TestCheckFunc {
	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttrSet(alertResourceName, "id"),
		resource.TestCheckResourceAttr(alertResourceName, "enabled", "true"),
		resource.TestCheckResourceAttr(alertResourceName, "name", alert.name),
		resource.TestCheckResourceAttr(alertResourceName, "description", alert.description),
		resource.TestCheckResourceAttr(alertResourceName, "alert_severity", alert.severity),
		resource.TestCheckResourceAttr(alertResourceName, "notification.0.recipients.0.emails.0", alert.emailRecipients[0]),
		resource.TestCheckResourceAttr(alertResourceName, "notification.0.notify_every_min", strconv.Itoa(alert.notifyEveryMin)),
		resource.TestCheckResourceAttr(alertResourceName, "flow.0.stages.1.time_window.0.hours", "0"),
		resource.TestCheckResourceAttr(alertResourceName, "flow.0.stages.1.time_window.0.minutes", "20"),
		resource.TestCheckResourceAttr(alertResourceName, "flow.0.stages.1.time_window.0.seconds", "0"),
		resource.TestCheckResourceAttr(alertResourceName, "flow.0.stages.1.groups.0.operator", "OR"),
	}
	checks = appendSchedulingChecks(checks, alert.daysOfWeek, alert.activityStarts, alert.activityEnds)
	return checks
}

func extractCommonChecks(a *alertCommonTestParams, alertType string) []resource.TestCheckFunc {
	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttrSet(alertResourceName, "id"),
		resource.TestCheckResourceAttr(alertResourceName, "enabled", "true"),
		resource.TestCheckResourceAttr(alertResourceName, "name", a.name),
		resource.TestCheckResourceAttr(alertResourceName, "description", a.description),
		resource.TestCheckResourceAttr(alertResourceName, "alert_severity", a.severity),
		resource.TestCheckResourceAttr(alertResourceName, "notification.0.recipients.0.emails.0", a.emailRecipients[0]),
		resource.TestCheckResourceAttr(alertResourceName, "notification.0.notify_every_min", strconv.Itoa(a.notifyEveryMin)),
		resource.TestCheckResourceAttr(alertResourceName, fmt.Sprintf("%s.0.search_query", alertType), a.searchQuery),
	}

	checks = appendSchedulingChecks(checks, a.daysOfWeek, a.activityStarts, a.activityEnds)

	checks = appendSeveritiesCheck(checks, a.alertFilters.severities, alertType)

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
		checks = append(checks, resource.TestCheckTypeSetElemAttr(alertResourceName, "scheduling.0.time_frames.0.days_enabled.*", d))
	}
	checks = append(checks, resource.TestCheckResourceAttr(alertResourceName, "scheduling.0.time_frames.0.start_time", startTime))
	checks = append(checks, resource.TestCheckResourceAttr(alertResourceName, "scheduling.0.time_frames.0.end_time", endTime))
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
  alert_severity     = "%s"
  
  notification {
    recipients {
      emails      = %s
    }
    notify_every_min = %d
	notify_only_on_triggered_group_by_values = true
  }

  scheduling {
    time_zone =  "%s"
	
	time_frames {
    	days_enabled = %s
    	start_time = "%s"
    	end_time = "%s"
  	}
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
      less_than = true
      occurrences_threshold = %d
      time_window = "%s"
      manage_undetected_values {
			enable_triggering_on_undetected_values = true
			auto_retire_ratio = "%s"
		}
    }
  }
}
`,
		a.name, a.description, a.severity, sliceToString(a.emailRecipients), a.notifyEveryMin, a.timeZone,
		sliceToString(a.daysOfWeek), a.activityStarts, a.activityEnds,
		sliceToString(a.severities), a.searchQuery, sliceToString(a.groupBy), a.occurrencesThreshold, a.timeWindow, a.deadmanRatio)
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
    notify_every_min = %d
    notify_only_on_triggered_group_by_values = true
  }

  scheduling {
    time_zone =  "%s"
	
	time_frames {
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
      queries_ratio = %f
      time_window   = "%s"
      group_by      = %s
      group_by_q1   = true
    }
  }
}`,
		a.name, a.description, a.severity, sliceToString(a.emailRecipients), a.notifyEveryMin, a.timeZone,
		sliceToString(a.daysOfWeek), a.activityStarts, a.activityEnds,
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
    notify_every_min = %d
  }

  scheduling {
    time_zone =  "%s"
	
	time_frames {
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
		a.name, a.description, a.severity, sliceToString(a.emailRecipients), a.notifyEveryMin, a.timeZone,
		sliceToString(a.daysOfWeek), a.activityStarts, a.activityEnds,
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
    notify_every_min = %d
  }

  scheduling {
    time_zone =  "%s"
	
	time_frames {
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
		a.name, a.description, a.severity, sliceToString(a.emailRecipients), a.notifyEveryMin, a.timeZone,
		sliceToString(a.daysOfWeek), a.activityStarts, a.activityEnds, sliceToString(a.severities),
		a.searchQuery, a.uniqueCountKey, a.maxUniqueValues, a.timeWindow, a.groupByKey, a.maxUniqueValuesForGroupBy)
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
    notify_every_min = %d
	notify_only_on_triggered_group_by_values = true
  }

  scheduling {
    time_zone =  "%s"
	
	time_frames {
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
    }
  }
}`,
		a.name, a.description, a.severity, sliceToString(a.emailRecipients), a.notifyEveryMin, a.timeZone,
		sliceToString(a.daysOfWeek), a.activityStarts, a.activityEnds,
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
    notify_every_min = %d
	notify_only_on_triggered_group_by_values = true
  }

  scheduling {
    time_zone =  "%s"
	
	time_frames {
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
			disable_triggering_on_undetected_values = true
		}
      }
    }
  }
}`,
		a.name, a.description, a.severity, sliceToString(a.emailRecipients), a.notifyEveryMin, a.timeZone,
		sliceToString(a.daysOfWeek), a.activityStarts, a.activityEnds, a.searchQuery, a.metricField, a.arithmeticOperator,
		a.threshold, a.arithmeticOperatorModifier, a.sampleThresholdPercentage, a.timeWindow, sliceToString(a.groupBy))
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
    notify_every_min = %d
  }

  scheduling {
    time_zone =  "%s"
	
	time_frames {
    	days_enabled = %s
    	start_time = "%s"
    	end_time = "%s"
  	}
  }

  metric {
    promql {
      search_query = "http_requests_total{status!~\"4..\"}"
      condition {
        less_than                    = true
        threshold                    = %d
        sample_threshold_percentage  = %d
        time_window                  = "%s"
        min_non_null_values_percentage          = %d
      }
    }
  }
}`,
		a.name, a.description, a.severity, sliceToString(a.emailRecipients), a.notifyEveryMin, a.timeZone,
		sliceToString(a.daysOfWeek), a.activityStarts, a.activityEnds, a.threshold, a.sampleThresholdPercentage,
		a.timeWindow, a.nonNullPercentage)
}

func testAccCoralogixResourceAlertTracing(a *tracingAlertTestParams) string {
	return fmt.Sprintf(`resource "coralogix_alert" "test" {
  name               = "%s"
  description        = "%s"
  alert_severity     = "%s"
  notification {
	on_trigger_and_resolved = true
    recipients {
      emails      = %s
    }
    notify_every_min = %d
  }

  scheduling {
    time_zone =  "%s"
	
	time_frames {
    	days_enabled = %s
    	start_time = "%s"
    	end_time = "%s"
  	}
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
		a.name, a.description, a.severity, sliceToString(a.emailRecipients), a.notifyEveryMin, a.timeZone,
		sliceToString(a.daysOfWeek), a.activityStarts, a.activityEnds,
		sliceToString(a.severities), a.conditionLatencyMs, a.timeWindow, a.occurrencesThreshold, sliceToString(a.groupBy))
}

func testAccCoralogixResourceAlertFLow(a *flowAlertTestParams) string {
	return fmt.Sprintf(`resource "coralogix_alert" "standard_alert" {
	name               = "standard"
	alert_severity     = "Info"
	standard {
		condition {
			immediately = true
		}
	}
}

	resource "coralogix_alert" "test" {
  		name               = "%s"
  		description        = "%s"
	  	alert_severity     = "%s"
		notification {
    		recipients {
      			emails      = %s
    		}
    	notify_every_min = %d
  		}

  		scheduling {
    		time_zone =  "%s"
			time_frames {
    			days_enabled = %s
    			start_time = "%s"
    			end_time = "%s"
  			}
		}

  	flow {
    	stages {
      		groups {
        		sub_alerts {
          			user_alert_id = coralogix_alert.standard_alert.id
        		}
        		operator = "OR"
      		}
    	}
    	stages {
      		groups {
        		sub_alerts {
          			user_alert_id = coralogix_alert.standard_alert.id
				}
        		sub_alerts {
          			user_alert_id = coralogix_alert.standard_alert.id
        		}
        		operator = "OR"
      		}
      		time_window {
        		minutes = 20
      		}
		}
  	}
}`,
		a.name, a.description, a.severity, sliceToString(a.emailRecipients), a.notifyEveryMin, a.timeZone,
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
	threshold, nonNullPercentage, sampleThresholdPercentage int
	timeWindow                                              string
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
	notifyEveryMin              int
	activeWhen
}

type alertCommonTestParams struct {
	name, description, severity string
	emailRecipients             []string
	notifyEveryMin              int
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
		timeZone:       selectRandomlyFromSlice(alertValidTimeZones),
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
