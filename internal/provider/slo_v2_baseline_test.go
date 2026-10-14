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
	"context"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/slo_mgmt"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils/schemadump"
)

const sloGoldenDir = "testdata/slo_v2_baseline"

// These tests record the behavior of the handwritten coralogix_slo_v2. After
// the generator switch, the golden diff must show only message wording.
//
//	UPDATE_GOLDEN=1 go test ./internal/provider -run TestSLOV2Baseline

func TestSLOV2BaselineSchema(t *testing.T) {
	var current resource.SchemaResponse
	slo_mgmt.NewSLOV2Resource().Schema(context.Background(), resource.SchemaRequest{}, &current)
	checkGoldenAt(t, sloGoldenDir, "schema_current.txt", []byte(schemadump.Text(current.Schema)))
}

func TestSLOV2BaselineScenarios(t *testing.T) {
	for name, run := range sloScenarios {
		t.Run(name, func(t *testing.T) {
			h := newSLOHarness(t)
			run(h)
			for i := range h.steps {
				// The framework validates nested attributes in map order, so the order of
				// several validation errors is not stable.
				sort.Strings(h.steps[i].ValidateDiags)
			}
			checkGoldenAt(t, sloGoldenDir, "scenario_"+name+".json", marshalGolden(t, h.steps))
		})
	}
}

func sloConfig(name string, sli j, extra ...j) j {
	out := j{
		"name":                        name,
		"target_threshold_percentage": 99.5,
		"sli":                         sli,
		"window":                      j{"slo_time_frame": "7_days"},
	}
	for _, e := range extra {
		for k, v := range e {
			out[k] = v
		}
	}
	return out
}

var sloRequestSLI = j{"request_based_metric_sli": j{
	"good_events":  j{"query": `sum(rate(http_requests_total{status="200"}[1m])) by (service)`},
	"total_events": j{"query": `sum(rate(http_requests_total[1m])) by (service)`},
}}

func sloWindowSLI(extra j) j {
	sli := j{
		"query":               j{"query": `sum(rate(up[1m])) by (job)`},
		"window":              "1_minute",
		"comparison_operator": "greater_than",
		"threshold":           0.5,
	}
	for k, v := range extra {
		sli[k] = v
	}
	return j{"window_based_metric_sli": sli}
}

func sloAPMSLI(extra j) j {
	sli := j{"services": l{"checkout"}}
	for k, v := range extra {
		sli[k] = v
	}
	return j{"apm_sli": sli}
}

var sloScenarios = map[string]func(h *grHarness){
	"request-minimal": func(h *grHarness) {
		s := h.Apply("create", h.null(), sloConfig("request", sloRequestSLI))
		s = h.Refresh("refresh", s)
		h.Destroy("destroy", s)
	},

	"request-full-lifecycle": func(h *grHarness) {
		full := sloConfig("request", sloRequestSLI, j{
			"description": "checkout availability",
			"labels":      j{"team": "payments"},
			"ownership_tags": j{
				"service": j{"static_values": l{"checkout", "checkout"}},
				"team":    j{"label_keys": l{"team"}},
			},
		})
		s := h.Apply("create", h.null(), full)
		s = h.Refresh("refresh", s)
		s = h.Apply("update: name, target, time frame, labels", s, with(full, j{
			"name":                        "request-renamed",
			"target_threshold_percentage": 99.9,
			"window":                      j{"slo_time_frame": "28_days"},
			"labels":                      j{"team": "payments", "tier": "1"},
		}))
		s = h.Apply("update: remove description, labels, and ownership tags", s, sloConfig("request-renamed", sloRequestSLI, j{
			"target_threshold_percentage": 99.9,
			"window":                      j{"slo_time_frame": "28_days"},
		}))
		h.Destroy("destroy", s)
	},

	"window-missing-data-strategy": func(h *grHarness) {
		s := h.Apply("create: strategy omitted", h.null(), sloConfig("window", sloWindowSLI(nil)))
		s = h.Apply("update: set strategy bad", s, sloConfig("window", sloWindowSLI(j{"missing_data_strategy": "bad"})))
		s = h.Apply("update: remove strategy", s, sloConfig("window", sloWindowSLI(nil)))
		s = h.Apply("update: change threshold with strategy removed", s, sloConfig("window", sloWindowSLI(j{"threshold": 0.7})))
		h.Refresh("refresh", s)
	},

	"apm-latency": func(h *grHarness) {
		latency := j{"time_window": "5_minutes", "quantile": j{"percentile": 0.95}, "threshold": 300.0}
		cfg := sloConfig("apm-latency", sloAPMSLI(j{
			"latency_config": latency,
			"filters":        l{j{"key": "environment", "values": l{"prod", "staging"}}},
			"grouping_keys":  l{"region"},
		}), j{"product_type": "apm"})
		s := h.Apply("create", h.null(), cfg)
		s = h.Refresh("refresh", s)
		s = h.Apply("update: remove threshold, filters, and grouping keys", s, sloConfig("apm-latency", sloAPMSLI(j{
			"latency_config": j{"time_window": "5_minutes", "quantile": j{"percentile": 0.95}},
		}), j{"product_type": "apm"}))
		s = h.Apply("update: average instead of quantile", s, sloConfig("apm-latency", sloAPMSLI(j{
			"latency_config": j{"time_window": "1_minute", "average": j{}},
		}), j{"product_type": "apm"}))
		h.Destroy("destroy", s)
	},

	"apm-error": func(h *grHarness) {
		cfg := sloConfig("apm-error", sloAPMSLI(j{"error_config": j{}}), j{"product_type": "apm"})
		s := h.Apply("create", h.null(), cfg)
		h.Refresh("refresh", s)
	},

	"product-type-omitted": func(h *grHarness) {
		s := h.Apply("create: product_type omitted", h.null(), sloConfig("pt", sloRequestSLI))
		s = h.Apply("update: product_type unspecified", s, sloConfig("pt", sloRequestSLI, j{"product_type": "unspecified"}))
		h.Apply("update: product_type omitted again", s, sloConfig("pt", sloRequestSLI))
	},

	"switch-sli": func(h *grHarness) {
		s := h.Apply("create: request based", h.null(), sloConfig("switch", sloRequestSLI))
		s = h.Apply("update: window based", s, sloConfig("switch", sloWindowSLI(nil)))
		h.Refresh("refresh", s)
	},

	"config-validation": func(h *grHarness) {
		h.Validate("missing name", j{"target_threshold_percentage": 99.0, "sli": sloRequestSLI, "window": j{"slo_time_frame": "7_days"}})
		h.Validate("missing sli", j{"name": "n", "target_threshold_percentage": 99.0, "window": j{"slo_time_frame": "7_days"}})
		h.Validate("two sli arms", sloConfig("v", j{
			"request_based_metric_sli": sloRequestSLI["request_based_metric_sli"],
			"window_based_metric_sli":  sloWindowSLI(nil)["window_based_metric_sli"],
		}))
		h.Validate("slo_time_frame 1_day", sloConfig("v", sloRequestSLI, j{"window": j{"slo_time_frame": "1_day"}}))
		h.Validate("slo_time_frame in upper case", sloConfig("v", sloRequestSLI, j{"window": j{"slo_time_frame": "7_DAYS"}}))
		h.Validate("window unspecified", sloConfig("v", sloWindowSLI(j{"window": "unspecified"})))
		h.Validate("apm latency without quantile or average", sloConfig("v", sloAPMSLI(j{"latency_config": j{"time_window": "1_minute"}}), j{"product_type": "apm"}))
		h.Validate("apm with error and latency", sloConfig("v", sloAPMSLI(j{"error_config": j{}, "latency_config": j{"time_window": "1_minute", "average": j{}}}), j{"product_type": "apm"}))
		h.Validate("apm without services", sloConfig("v", j{"apm_sli": j{"error_config": j{}}}, j{"product_type": "apm"}))
		h.Validate("empty filters", sloConfig("v", sloAPMSLI(j{"error_config": j{}, "filters": l{}}), j{"product_type": "apm"}))
		h.Validate("ownership tags with no dimension", sloConfig("v", sloRequestSLI, j{"ownership_tags": j{}}))
	},

	"import": func(h *grHarness) {
		id := h.slos().seed(j{
			"name": "seeded", "description": "made outside Terraform", "targetThresholdPercentage": 99,
			"sloTimeFrame": "SLO_TIME_FRAME_14_DAYS",
			"windowBasedMetricSli": j{
				"query": j{"query": "avg(up)"}, "window": "WINDOW_SLO_WINDOW_5_MINUTES",
				"comparisonOperator": "COMPARISON_OPERATOR_LESS_THAN", "threshold": 0.2,
			},
		})
		s := h.Import("import", id)
		h.Apply("plan the matching config", s, sloConfig("seeded", sloWindowSLI(j{
			"query": j{"query": "avg(up)"}, "window": "5_minutes", "comparison_operator": "less_than", "threshold": 0.2,
		}), j{
			"description":                 "made outside Terraform",
			"target_threshold_percentage": 99.0,
			"window":                      j{"slo_time_frame": "14_days"},
		}))
	},

	"import-legacy-filter-value": func(h *grHarness) {
		id := h.slos().seedRaw(j{
			"name": "legacy", "targetThresholdPercentage": 99, "sloTimeFrame": "SLO_TIME_FRAME_7_DAYS",
			"productType": "SLO_PRODUCT_TYPE_APM",
			"apmSli": j{
				"services": l{"checkout"}, "errorConfig": j{}, "groupingKeys": l{},
				"filters": l{j{"key": "environment", "value": "prod"}},
			},
		})
		h.Import("import a filter that has only the deprecated value", id)
	},

	"import-legacy-defaults": func(h *grHarness) {
		id := h.slos().seedRaw(j{
			"name": "legacy-defaults", "targetThresholdPercentage": 99.0, "sloTimeFrame": "SLO_TIME_FRAME_7_DAYS",
			"labels": j{},
			"windowBasedMetricSli": j{
				"query": j{"query": "sum(rate(up[1m])) by (job)"}, "window": "WINDOW_SLO_WINDOW_1_MINUTE",
				"comparisonOperator": "COMPARISON_OPERATOR_GREATER_THAN", "threshold": 0.5,
			},
		})
		s := h.Import("import a record without productType and missingDataStrategy", id)
		h.Apply("plan the matching config with missing_data_strategy set", s, sloConfig("legacy-defaults", sloWindowSLI(j{"missing_data_strategy": "uncounted"}), j{
			"target_threshold_percentage": 99.0,
		}))
	},

	"import-unknown-time-frame": func(h *grHarness) {
		id := h.slos().seed(j{
			"name": "one-day", "targetThresholdPercentage": 99, "sloTimeFrame": "SLO_TIME_FRAME_1_DAY",
			"requestBasedMetricSli": j{"goodEvents": j{"query": "a"}, "totalEvents": j{"query": "b"}},
		})
		h.Import("import a time frame that the released schema does not offer", id)
	},

	"import-unknown-id": func(h *grHarness) {
		h.Import("import an id that does not exist", "does-not-exist")
	},

	"read-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), sloConfig("gone", sloRequestSLI))
		h.slos().forget(idOf(s))
		h.Refresh("refresh: SLO was deleted outside Terraform", s)
	},

	"delete-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), sloConfig("gone", sloRequestSLI))
		h.slos().forget(idOf(s))
		h.slos().deleteStatus = 404
		h.Destroy("destroy: the API answers 404", s)
	},

	"update-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), sloConfig("gone", sloRequestSLI))
		h.slos().forget(idOf(s))
		h.Apply("update: SLO was deleted outside Terraform", s, sloConfig("gone", sloRequestSLI, j{"description": "changed"}))
	},
}
