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
	"maps"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/events2metrics"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils/schemadump"
)

const e2mTypeName = "coralogix_events2metric"

const e2mGoldenDir = "testdata/events2metric_baseline"

// These tests record the behavior of coralogix_events2metric through the plugin
// protocol, against a fake backend that follows the server code. When the resource
// changes, the golden diff must show only listed fixes or message wording.
//
//	UPDATE_GOLDEN=1 go test ./internal/provider -run TestEvents2MetricBaseline

func newE2MHarness(t *testing.T) *grHarness {
	return newProtocolHarness(t, e2mTypeName, newE2MFake(t))
}

func (h *grHarness) events2metrics() *e2mFake {
	h.t.Helper()
	f, ok := h.api.(*e2mFake)
	if !ok {
		h.t.Fatal("expected *e2mFake")
	}
	return f
}

func TestEvents2MetricBaselineSchema(t *testing.T) {
	var current resource.SchemaResponse
	events2metrics.NewEvents2MetricResource().Schema(context.Background(), resource.SchemaRequest{}, &current)
	checkGoldenAt(t, e2mGoldenDir, "schema_current.txt", []byte(schemadump.Text(current.Schema)))
}

func TestEvents2MetricBaselineScenarios(t *testing.T) {
	for name, run := range e2mScenarios {
		t.Run(name, func(t *testing.T) {
			h := newE2MHarness(t)
			run(h)
			checkGoldenAt(t, e2mGoldenDir, "scenario_"+name+".json", marshalGolden(t, h.steps))
		})
	}
}

// logs2metric is a logs events2metric with a name and an empty logs query.
func logs2metric(name string, extra ...j) j {
	out := j{"name": name, "logs_query": j{}}
	for _, e := range extra {
		maps.Copy(out, e)
	}
	return out
}

// e2mFullLogs sets every attribute that a logs events2metric can have.
var e2mFullLogs = logs2metric("full", j{
	"description": "logs2metric",
	"data_source": "system/poc.error_tracking",
	"logs_query": j{
		"lucene":       "remote_addr_enriched:/.*/",
		"applications": l{"filter:startsWith:nginx"},
		"subsystems":   l{"api"},
		"severities":   l{"Debug", "Error"},
	},
	"metric_fields": j{
		"method": j{"source_field": "method"},
		"geo_point": j{
			"source_field": "location_geopoint",
			"aggregations": j{
				"max":       j{"enable": false},
				"min":       j{"enable": false},
				"avg":       j{"enable": true},
				"samples":   j{"enable": true, "type": "Min"},
				"histogram": j{"enable": true, "buckets": l{1.5, 2.0}},
			},
		},
	},
	"metric_labels": j{"Status": "status", "Path": "http_referer"},
	"permutations":  j{"limit": float64(20000)},
})

var e2mFullSpans = j{
	"name":        "spans",
	"description": "spans2metric",
	"spans_query": j{
		"lucene":       "remote_addr_enriched:/.*/",
		"applications": l{"filter:startsWith:nginx"},
		"subsystems":   l{"api"},
		"actions":      l{"action-name"},
		"services":     l{"service-name"},
	},
	"metric_fields": j{"duration": j{"source_field": "duration"}},
	"metric_labels": j{"Service": "tags.service"},
	"permutations":  j{"limit": float64(100)},
}

var e2mScenarios = map[string]func(h *grHarness){
	// Only the required attributes. The API fills lucene, the permutations, and no fields.
	"minimal": func(h *grHarness) {
		s := h.Apply("create", h.null(), logs2metric("minimal"))
		s = h.Refresh("refresh", s)
		h.Destroy("destroy", s)
	},

	"logs-full-lifecycle": func(h *grHarness) {
		s := h.Apply("create", h.null(), e2mFullLogs)
		s = h.Refresh("refresh", s)
		s = h.Apply("update: change description, a label, and an aggregation", s, with(e2mFullLogs, j{
			"description":   "changed",
			"metric_labels": j{"Status": "status_code", "Path": "http_referer"},
			"metric_fields": j{
				"method": j{"source_field": "method", "aggregations": j{"count": j{"enable": false}}},
				"geo_point": j{
					"source_field": "location_geopoint",
					"aggregations": j{"max": j{"enable": true}},
				},
			},
		}))
		h.Import("import", idOf(s))
		h.Destroy("destroy", s)
	},

	"spans-full-lifecycle": func(h *grHarness) {
		s := h.Apply("create", h.null(), e2mFullSpans)
		s = h.Refresh("refresh", s)
		s = h.Apply("update: change filters", s, with(e2mFullSpans, j{
			"spans_query": j{"actions": l{"other-action"}, "services": l{"a", "b"}},
		}))
		h.Destroy("destroy", s)
	},

	// Removing optional attributes. Update sends description "" to clear it, and the API
	// reads "" back, so the apply is inconsistent with the planned null. permutations and
	// the aggregations keep the values in state.
	"unset-optional-attributes": func(h *grHarness) {
		s := h.Apply("create", h.null(), e2mFullLogs)
		s = h.Apply("update: only the name and an empty logs query are left", s, logs2metric("full"))
		h.Refresh("refresh", s)
	},

	// A new metric field next to one in state: its aggregations are unknown, not
	// copied from a null prior value.
	"add-metric-field": func(h *grHarness) {
		cfg := logs2metric("grow", j{"metric_fields": j{"a": j{"source_field": "a"}}})
		s := h.Apply("create", h.null(), cfg)
		s = h.Apply("update: add a field", s, with(cfg, j{"metric_fields": j{
			"a": j{"source_field": "a"},
			"b": j{"source_field": "b", "aggregations": j{"sum": j{"enable": false}}},
		}}))
		s = h.Apply("update: remove the first field", s, with(cfg, j{"metric_fields": j{
			"b": j{"source_field": "b", "aggregations": j{"sum": j{"enable": false}}},
		}}))
		h.Refresh("refresh", s)
	},

	// An aggregation left out of the config keeps the value in state, also after it was set.
	"aggregation-removed-from-config": func(h *grHarness) {
		cfg := logs2metric("aggs", j{"metric_fields": j{"f": j{"source_field": "f", "aggregations": j{
			"min": j{"enable": false},
		}}}})
		s := h.Apply("create", h.null(), cfg)
		h.Apply("update: min left out", s, logs2metric("aggs", j{"metric_fields": j{"f": j{"source_field": "f"}}}))
	},

	"severities-unspecified": func(h *grHarness) {
		s := h.Apply("create", h.null(), logs2metric("sev", j{"logs_query": j{"severities": l{"Unspecified", "Critical"}}}))
		h.Refresh("refresh", s)
	},

	// permutations left out sends a limit of 0, which the API refuses next to labels.
	"permutations-omitted": func(h *grHarness) {
		h.Apply("create without labels", h.null(), logs2metric("nolabels"))
		h.Apply("create with labels", h.null(), logs2metric("labels", j{"metric_labels": j{"L": "l"}}))
	},

	"permutations-change": func(h *grHarness) {
		cfg := logs2metric("perm", j{"permutations": j{"limit": float64(10)}})
		s := h.Apply("create", h.null(), cfg)
		s = h.Apply("update: limit 20", s, with(cfg, j{"permutations": j{"limit": float64(20)}}))
		h.Apply("update: permutations left out", s, logs2metric("perm"))
	},

	"has-exceeded-limit": func(h *grHarness) {
		s := h.Apply("create", h.null(), logs2metric("exceeded", j{"permutations": j{"limit": float64(5)}}))
		h.events2metrics().exceeded[idOf(s)] = true
		s = h.Refresh("refresh: the limit was exceeded", s)
		h.Apply("plan the same config", s, logs2metric("exceeded", j{"permutations": j{"limit": float64(5)}}))
	},

	// The new query block plans lucene from a null prior value (UseStateForUnknown), and
	// the API reads "" back, so the apply is inconsistent.
	"switch-query-type": func(h *grHarness) {
		s := h.Apply("create logs", h.null(), logs2metric("switch"))
		h.Apply("update: spans query instead", s, j{"name": "switch", "spans_query": j{}})
	},

	"config-validation": func(h *grHarness) {
		h.Validate("no query", j{"name": "n"})
		h.Validate("both queries", j{"name": "n", "logs_query": j{}, "spans_query": j{}})
		h.Validate("name with a dot", logs2metric("a.b"))
		h.Validate("empty name", logs2metric(""))
		h.Validate("unknown severity", logs2metric("n", j{"logs_query": j{"severities": l{"debug"}}}))
		h.Validate("empty severities", logs2metric("n", j{"logs_query": j{"severities": l{}}}))
		h.Validate("empty applications", logs2metric("n", j{"logs_query": j{"applications": l{}}}))
		h.Validate("unknown sample type", logs2metric("n", j{"metric_fields": j{"f": j{"source_field": "f", "aggregations": j{
			"samples": j{"type": "Avg"},
		}}}}))
		h.Validate("empty metric fields", logs2metric("n", j{"metric_fields": j{}}))
		h.Validate("empty metric labels", logs2metric("n", j{"metric_labels": j{}}))
		h.Validate("negative limit", logs2metric("n", j{"permutations": j{"limit": float64(-1)}}))
	},

	"import": func(h *grHarness) {
		id := h.events2metrics().seed(j{
			"name": "seeded", "type": "E2M_TYPE_LOGS2METRICS", "description": "made outside Terraform",
			"logsQuery":         j{"lucene": "x", "severityFilters": l{"SEVERITY_INFO"}},
			"metricFields":      l{j{"targetBaseMetricName": "f", "sourceField": "f", "aggregations": l{}}},
			"metricLabels":      l{j{"targetLabel": "L", "sourceField": "l"}},
			"permutationsLimit": float64(7),
		})
		s := h.Import("import", id)
		h.Apply("plan the matching config", s, logs2metric("seeded", j{
			"description":   "made outside Terraform",
			"logs_query":    j{"lucene": "x", "severities": l{"Info"}},
			"metric_fields": j{"f": j{"source_field": "f"}},
			"metric_labels": j{"L": "l"},
			"permutations":  j{"limit": float64(7)},
		}))
	},

	"import-unknown-id": func(h *grHarness) {
		h.Import("import an id that does not exist", "00000000-0000-4000-8000-999999999999")
	},

	"read-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), logs2metric("gone"))
		h.events2metrics().forget(idOf(s))
		h.Refresh("refresh: deleted outside Terraform", s)
	},

	// The API answers 400, not 404, to a Replace or Delete of a missing id.
	"update-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), logs2metric("gone"))
		h.events2metrics().forget(idOf(s))
		h.Apply("update: deleted outside Terraform", s, logs2metric("gone", j{"description": "changed"}))
	},

	"delete-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), logs2metric("gone"))
		h.events2metrics().forget(idOf(s))
		h.Destroy("destroy: deleted outside Terraform", s)
	},

	"upgrade-from-v0": func(h *grHarness) {
		h.UpgradeV0("upgrade", `{
			"id": "00000000-0000-4000-8000-000000000001", "name": "old", "description": "v0",
			"metric_labels": [{"target_label": "L", "source_field": "l"}],
			"metric_fields": [{"target_base_metric_name": "f", "source_field": "f", "aggregations": []}],
			"permutations": [{"limit": "10", "has_exceed_limit": false}],
			"logs_query": [{"lucene": "x", "applications": ["a"], "subsystems": null, "severities": ["Info"]}],
			"spans_query": []
		}`)
	},
}
