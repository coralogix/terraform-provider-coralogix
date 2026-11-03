// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an AS IS BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"net/http"
	"sort"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils/schemadump"
)

const (
	tcoLogsTypeName   = "coralogix_tco_policies_logs"
	tcoTracesTypeName = "coralogix_tco_policies_traces"
	tcoRumTypeName    = "coralogix_tco_policies_rum"

	tcoLogsGoldenDir   = "testdata/tco_policies_logs_baseline"
	tcoTracesGoldenDir = "testdata/tco_policies_traces_baseline"
	tcoRumGoldenDir    = "testdata/tco_policies_rum_baseline"
)

// These tests record the behavior of the registered coralogix_tco_policies_logs,
// coralogix_tco_policies_traces, and coralogix_tco_policies_rum. After a change, the golden diff
// must show only listed fixes or message wording.
//
//	UPDATE_GOLDEN=1 go test ./internal/provider -run 'TestTCOPolicies.*Baseline'

type tcoBaseline struct {
	typeName, dir string
	scenarios     map[string]func(h *grHarness, f *tcoFake)
}

var tcoBaselines = map[string]tcoBaseline{
	"logs":   {tcoLogsTypeName, tcoLogsGoldenDir, tcoLogsScenarios},
	"traces": {tcoTracesTypeName, tcoTracesGoldenDir, tcoTracesScenarios},
	"rum":    {tcoRumTypeName, tcoRumGoldenDir, tcoRumScenarios},
}

func TestTCOPoliciesBaselineSchema(t *testing.T) {
	for name, b := range tcoBaselines {
		t.Run(name, func(t *testing.T) {
			checkGoldenAt(t, b.dir, "schema_current.txt", []byte(schemadump.Text(registeredResourceSchema(t, b.typeName))))
		})
	}
}

func TestTCOPoliciesBaselineScenarios(t *testing.T) {
	for name, b := range tcoBaselines {
		for scenario, run := range b.scenarios {
			t.Run(name+"/"+scenario, func(t *testing.T) {
				fake := newTCOFake(t)
				h := newProtocolHarness(t, b.typeName, fake)
				run(h, fake)
				for i := range h.steps {
					// The framework validates nested attributes in map order, so the order of
					// several validation errors is not stable.
					sort.Strings(h.steps[i].ValidateDiags)
				}
				checkGoldenAt(t, b.dir, "scenario_"+scenario+".json", marshalGolden(t, h.steps))
			})
		}
	}
}

// tcoPolicies is the config of a resource with these policies.
func tcoPolicies(policies ...j) j {
	items := make(l, 0, len(policies))
	for _, p := range policies {
		items = append(items, p)
	}
	return j{"policies": items}
}

// tcoPolicy is a policy config: a name, a priority, and the extra attributes.
func tcoPolicy(name, priority string, extra ...j) j {
	out := j{"name": name}
	if priority != "" {
		out["priority"] = priority
	}
	for _, e := range extra {
		for k, v := range e {
			out[k] = v
		}
	}
	return out
}

func tcoRule(ruleType string, names ...string) j {
	items := make(l, 0, len(names))
	for _, n := range names {
		items = append(items, n)
	}
	out := j{"names": items}
	if ruleType != "" {
		out["rule_type"] = ruleType
	}
	return out
}

func tcoTiers(tiers ...j) j {
	items := make(l, 0, len(tiers))
	for _, t := range tiers {
		items = append(items, t)
	}
	return j{"usage_tiers": items}
}

func tcoTier(pct float64, priority string) j {
	return j{"daily_quota_percentage": pct, "priority": priority}
}

func tcoSeverities(s ...string) j {
	items := make(l, 0, len(s))
	for _, v := range s {
		items = append(items, v)
	}
	return j{"severities": items}
}

// tcoLogEntry is an overwrite entry of a log policy, as another client sends it.
func tcoLogEntry(name, priority string, severities ...string) j {
	sev := make(l, 0, len(severities))
	for _, s := range severities {
		sev = append(sev, s)
	}
	return j{"policy": j{"name": name, "priority": priority}, "logRules": j{"severities": sev}}
}

var tcoLogsScenarios = map[string]func(h *grHarness, f *tcoFake){
	"minimal": func(h *grHarness, _ *tcoFake) {
		s := h.Apply("create", h.null(), tcoPolicies(tcoPolicy("first", "high", tcoSeverities("info"))))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy: sends an empty list", s)
	},

	// The resource sends severities: [] when the config has neither severities nor
	// dpxl_expression, and the API refuses a policy without either.
	"without-severities": func(h *grHarness, _ *tcoFake) {
		h.Apply("create: no severities and no dpxl_expression", h.null(), tcoPolicies(tcoPolicy("first", "high")))
	},

	"full-lifecycle": func(h *grHarness, _ *tcoFake) {
		first := tcoPolicy("first", "medium", tcoSeverities("error", "critical"), j{
			"description":  "first policy",
			"applications": tcoRule("", "app-a", "app-b"),
			"subsystems":   tcoRule("starts_with", "sub"),
		})
		second := tcoPolicy("second", "low", tcoSeverities("info"), j{
			"enabled":                       false,
			"applications":                  tcoRule("is_not", "app-c"),
			"quota_based_priority_override": tcoTiers(tcoTier(50, "high"), tcoTier(80.5, "medium")),
		})
		s := h.Apply("create", h.null(), tcoPolicies(first, second))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: reorder", s, tcoPolicies(second, first))
		s = h.Apply("update: change a policy", s, tcoPolicies(second, with(first, j{"priority": "low", "description": "changed"})))
		s = h.Apply("update: remove a policy", s, tcoPolicies(second))
		s = h.Apply("update: add a policy", s, tcoPolicies(second, tcoPolicy("third", "block", tcoSeverities("debug"))))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	},

	"unset-optional": func(h *grHarness, _ *tcoFake) {
		s := h.Apply("create", h.null(), tcoPolicies(tcoPolicy("first", "low", tcoSeverities("info", "warning"), j{
			"description":                   "described",
			"enabled":                       false,
			"applications":                  tcoRule("includes", "app"),
			"subsystems":                    tcoRule("is", "sub-a", "sub-b"),
			"quota_based_priority_override": tcoTiers(tcoTier(10, "medium")),
		})))
		s = h.Apply("update: remove every optional attribute but severities", s, tcoPolicies(tcoPolicy("first", "low", tcoSeverities("info"))))
		h.RefreshPlan("refresh", s)
	},

	"dpxl-expression": func(h *grHarness, _ *tcoFake) {
		s := h.Apply("create", h.null(), tcoPolicies(tcoPolicy("first", "high", j{"dpxl_expression": "<v1> $d.severity == 'INFO'"})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: severities instead of dpxl_expression", s, tcoPolicies(tcoPolicy("first", "high", tcoSeverities("info"))))
		h.Apply("update: dpxl_expression again", s, tcoPolicies(tcoPolicy("first", "high", j{"dpxl_expression": "<v1> true"})))
	},

	"dpxl-without-version": func(h *grHarness, _ *tcoFake) {
		h.Apply("create: the expression has no version tag", h.null(), tcoPolicies(tcoPolicy("first", "high", j{"dpxl_expression": "$d.severity == 'INFO'"})))
	},

	"targets": func(h *grHarness, f *tcoFake) {
		f.datasets = map[string]bool{"audit": true}
		s := h.Apply("create: per-target priorities", h.null(), tcoPolicies(tcoPolicy("first", "", tcoSeverities("info"), j{"targets": l{
			j{"dataset": "logs", "priority": "high"},
			j{"dataset": "audit", "dataspace": "default", "priority": "low", "priority_override": tcoTiers(tcoTier(40, "medium"))},
		}})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: remove a target", s, tcoPolicies(tcoPolicy("first", "", tcoSeverities("info"), j{"targets": l{
			j{"dataset": "logs", "priority": "medium"},
		}})))
		s = h.Apply("update: back to a policy-level priority", s, tcoPolicies(tcoPolicy("first", "high", tcoSeverities("info"))))
		h.RefreshPlan("refresh", s)
	},

	// The API accepts the archive retention of a target and does not store it (recorded in
	// fidelity/cases.json: logs-target-archive-retention), so a target never returns it.
	"target-archive-retention": func(h *grHarness, f *tcoFake) {
		f.archiveRetentions = map[string]bool{"11111111-1111-4111-8111-111111111111": true, "22222222-2222-4222-8222-222222222222": true}
		target := func(retention string) j {
			t := j{"dataset": "logs", "priority": "high"}
			if retention != "" {
				t["archive_retention_id"] = retention
			}
			return tcoPolicies(tcoPolicy("first", "", tcoSeverities("info"), j{"targets": l{t}}))
		}
		s := h.Apply("create: a target with an archive retention", h.null(), target("11111111-1111-4111-8111-111111111111"))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: change it", s, target("22222222-2222-4222-8222-222222222222"))
		s = h.Apply("update: remove it", s, target(""))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	},

	// The schema accepts rule_type "unspecified", and the released extractor sends it as
	// RULE_TYPE_ID_UNSPECIFIED, which the API rejects for every rule (recorded in fidelity/cases.json:
	// logs-unspecified-rule-type and the others).
	"unspecified-rule-type": func(h *grHarness, _ *tcoFake) {
		unspecified := tcoPolicies(tcoPolicy("first", "high", tcoSeverities("info"), j{"applications": tcoRule("unspecified", "web")}))
		h.Apply("create: rejected by the API", h.null(), unspecified)
		s := h.Apply("create with is", h.null(), tcoPolicies(tcoPolicy("first", "high", tcoSeverities("info"), j{"applications": tcoRule("is", "web")})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update to unspecified: rejected by the API", s, unspecified)
		h.RefreshPlan("refresh", s)
	},

	"targets-rejected": func(h *grHarness, _ *tcoFake) {
		h.Apply("create: priority on the policy and on the target", h.null(), tcoPolicies(tcoPolicy("first", "high", tcoSeverities("info"), j{"targets": l{
			j{"dataset": "logs", "priority": "high"},
		}})))
		h.Apply("create: neither a policy priority nor targets", h.null(), tcoPolicies(tcoPolicy("first", "", tcoSeverities("info"))))
		h.Apply("create: a dataset that does not exist", h.null(), tcoPolicies(tcoPolicy("first", "", tcoSeverities("info"), j{"targets": l{
			j{"dataset": "missing", "priority": "low"},
		}})))
	},

	// The API lowercases rule names. The validator refuses only names without any other
	// character than capitals.
	"mixed-case-names": func(h *grHarness, _ *tcoFake) {
		s := h.Apply("create: a mixed-case application name", h.null(), tcoPolicies(tcoPolicy("first", "high", tcoSeverities("info"), j{
			"applications": tcoRule("", "App-A"),
		})))
		h.RefreshPlan("refresh", s)
	},

	"quota-override": func(h *grHarness, _ *tcoFake) {
		s := h.Apply("create: a tier at 0 percent", h.null(), tcoPolicies(tcoPolicy("first", "low", tcoSeverities("info"), j{
			"quota_based_priority_override": tcoTiers(tcoTier(0, "high")),
		})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: a tier at or below the base priority", s, tcoPolicies(tcoPolicy("first", "high", tcoSeverities("info"), j{
			"quota_based_priority_override": tcoTiers(tcoTier(50, "low")),
		})))
		h.RefreshPlan("refresh", s)
	},

	"duplicate-names": func(h *grHarness, _ *tcoFake) {
		h.Apply("create: two policies with one name", h.null(), tcoPolicies(
			tcoPolicy("first", "high", tcoSeverities("info")),
			tcoPolicy("first", "low", tcoSeverities("error")),
		))
	},

	"archive-retention": func(h *grHarness, f *tcoFake) {
		f.archiveRetentions = map[string]bool{"11111111-1111-4111-8111-111111111111": true}
		s := h.Apply("create", h.null(), tcoPolicies(tcoPolicy("first", "high", tcoSeverities("info"), j{"archive_retention_id": "11111111-1111-4111-8111-111111111111"})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: an archive retention that does not exist", s, tcoPolicies(tcoPolicy("first", "high", tcoSeverities("info"), j{"archive_retention_id": missingID})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: remove the archive retention", s, tcoPolicies(tcoPolicy("first", "high", tcoSeverities("info"))))
		h.RefreshPlan("refresh", s)
	},

	"empty-list": func(h *grHarness, _ *tcoFake) {
		s := h.Apply("create: no policies", h.null(), j{"policies": l{}})
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	},

	"existing-policies": func(h *grHarness, f *tcoFake) {
		f.seed("logs", j{"policies": l{tcoLogEntry("made-outside", "PRIORITY_TYPE_LOW", "SEVERITY_DEBUG")}})
		h.Apply("create replaces the policies that the company already has", h.null(), tcoPolicies(tcoPolicy("first", "high", tcoSeverities("info"))))
	},

	"drift": func(h *grHarness, f *tcoFake) {
		s := h.Apply("create", h.null(), tcoPolicies(tcoPolicy("first", "high", tcoSeverities("info"))))
		f.seed("logs", j{"policies": l{
			tcoLogEntry("made-outside", "PRIORITY_TYPE_LOW", "SEVERITY_DEBUG"),
			tcoLogEntry("first", "PRIORITY_TYPE_MEDIUM", "SEVERITY_INFO"),
		}})
		s = h.RefreshPlan("refresh: policies changed outside Terraform", s)
		h.Apply("apply the config again", s, tcoPolicies(tcoPolicy("first", "high", tcoSeverities("info"))))
	},

	"deleted-outside-terraform": func(h *grHarness, f *tcoFake) {
		s := h.Apply("create", h.null(), tcoPolicies(tcoPolicy("first", "high", tcoSeverities("info"))))
		f.seed("logs", j{"policies": l{}})
		h.RefreshPlan("refresh: every policy was deleted outside Terraform", s)
	},

	"import": func(h *grHarness, f *tcoFake) {
		f.seed("logs", j{"policies": l{
			tcoLogEntry("seeded-a", "PRIORITY_TYPE_HIGH", "SEVERITY_INFO", "SEVERITY_ERROR"),
			tcoLogEntry("seeded-b", "PRIORITY_TYPE_LOW", "SEVERITY_DEBUG"),
		}})
		s := h.Import("import", "anything")
		h.Apply("plan the matching config", s, tcoPolicies(
			tcoPolicy("seeded-a", "high", tcoSeverities("info", "error")),
			tcoPolicy("seeded-b", "low", tcoSeverities("debug")),
		))
	},

	"import-empty": func(h *grHarness, _ *tcoFake) {
		h.Import("import when the company has no log policies", "anything")
	},

	"api-error": func(h *grHarness, f *tcoFake) {
		s := h.Apply("create", h.null(), tcoPolicies(tcoPolicy("first", "high", tcoSeverities("info"))))
		f.failGets(http.StatusInternalServerError, "Unexpected server error occurred")
		h.RefreshPlan("refresh: the API fails", s)
	},

	"route-missing": func(h *grHarness, f *tcoFake) {
		s := h.Apply("create", h.null(), tcoPolicies(tcoPolicy("first", "high", tcoSeverities("info"))))
		f.failGets(http.StatusNotFound, "Not Found")
		h.RefreshPlan("refresh: the gateway has no such route", s)
	},

	"update-rejected": func(h *grHarness, f *tcoFake) {
		s := h.Apply("create", h.null(), tcoPolicies(tcoPolicy("first", "high", tcoSeverities("info"))))
		f.failPuts(http.StatusInternalServerError, "Unexpected server error occurred")
		s = h.Apply("update: the API fails", s, tcoPolicies(tcoPolicy("first", "low", tcoSeverities("info"))))
		h.RefreshPlan("refresh", s)
	},

	"destroy-rejected": func(h *grHarness, f *tcoFake) {
		s := h.Apply("create", h.null(), tcoPolicies(tcoPolicy("first", "high", tcoSeverities("info"))))
		f.failPuts(http.StatusInternalServerError, "Unexpected server error occurred")
		h.Destroy("destroy: the API fails", s)
	},

	"config-validation": func(h *grHarness, _ *tcoFake) {
		h.Validate("no policies attribute", j{})
		h.Validate("dpxl_expression and severities", tcoPolicies(tcoPolicy("first", "high", tcoSeverities("info"), j{"dpxl_expression": "<v1> true"})))
		h.Validate("dpxl_expression and applications", tcoPolicies(tcoPolicy("first", "high", j{"dpxl_expression": "<v1> true", "applications": tcoRule("", "app")})))
		h.Validate("unknown priority", tcoPolicies(tcoPolicy("first", "urgent", tcoSeverities("info"))))
		h.Validate("unknown severity", tcoPolicies(tcoPolicy("first", "high", tcoSeverities("fatal"))))
		h.Validate("empty severities", tcoPolicies(tcoPolicy("first", "high", j{"severities": l{}})))
		h.Validate("empty name", tcoPolicies(tcoPolicy("", "high", tcoSeverities("info"))))
		h.Validate("capital application name", tcoPolicies(tcoPolicy("first", "high", tcoSeverities("info"), j{"applications": tcoRule("", "APP")})))
		h.Validate("unknown rule type", tcoPolicies(tcoPolicy("first", "high", tcoSeverities("info"), j{"applications": tcoRule("ends_with", "app")})))
		h.Validate("starts_with with two names", tcoPolicies(tcoPolicy("first", "high", tcoSeverities("info"), j{"subsystems": tcoRule("starts_with", "a", "b")})))
		h.Validate("tier above 100", tcoPolicies(tcoPolicy("first", "low", tcoSeverities("info"), j{"quota_based_priority_override": tcoTiers(tcoTier(150, "high"))})))
		h.Validate("no tiers", tcoPolicies(tcoPolicy("first", "low", tcoSeverities("info"), j{"quota_based_priority_override": tcoTiers()})))
		h.Validate("empty targets", tcoPolicies(tcoPolicy("first", "", tcoSeverities("info"), j{"targets": l{}})))
		h.Validate("target without priority", tcoPolicies(tcoPolicy("first", "", tcoSeverities("info"), j{"targets": l{j{"dataset": "logs"}}})))
		h.Validate("long dataspace", tcoPolicies(tcoPolicy("first", "", tcoSeverities("info"), j{"targets": l{j{"dataset": "logs", "priority": "low", "dataspace": "a-dataspace-name-that-is-longer-than-fifty-characters"}}})))
		h.Validate("empty archive_retention_id", tcoPolicies(tcoPolicy("first", "high", tcoSeverities("info"), j{"archive_retention_id": ""})))
		h.Validate("set id", with(tcoPolicies(tcoPolicy("first", "high", tcoSeverities("info"))), j{"id": "mine"}))
	},
}

// tcoSpanEntry is an overwrite entry of a span policy, as another client sends it.
func tcoSpanEntry(name, priority string, rules j) j {
	return j{"policy": j{"name": name, "priority": priority}, "spanRules": rules}
}

var tcoTracesScenarios = map[string]func(h *grHarness, f *tcoFake){
	"minimal": func(h *grHarness, _ *tcoFake) {
		s := h.Apply("create", h.null(), tcoPolicies(tcoPolicy("first", "high")))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy: sends an empty list", s)
	},

	"full-lifecycle": func(h *grHarness, _ *tcoFake) {
		first := tcoPolicy("first", "medium", j{
			"description":  "first policy",
			"enabled":      false,
			"applications": tcoRule("", "app-a", "app-b"),
			"subsystems":   tcoRule("includes", "sub"),
			"services":     tcoRule("", "svc-a", "svc-b"),
			"actions":      tcoRule("starts_with", "get"),
			"tags": j{
				"tags.http.method": tcoRule("", "GET", "POST"),
				"tags.region":      tcoRule("is_not", "eu"),
			},
		})
		second := tcoPolicy("second", "low", j{"quota_based_priority_override": tcoTiers(tcoTier(60, "medium"))})
		s := h.Apply("create", h.null(), tcoPolicies(first, second))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: reorder", s, tcoPolicies(second, first))
		s = h.Apply("update: remove the tags", s, tcoPolicies(second, with(first, j{"tags": nil})))
		s = h.Apply("update: remove the optional fields", s, tcoPolicies(tcoPolicy("second", "low"), tcoPolicy("first", "medium")))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: remove a policy", s, tcoPolicies(tcoPolicy("second", "low")))
		h.Destroy("destroy", s)
	},

	// The API keeps no order of the tag rules.
	"tag-rules-reordered": func(h *grHarness, f *tcoFake) {
		s := h.Apply("create", h.null(), tcoPolicies(tcoPolicy("first", "high", j{"tags": j{
			"tags.http.method": tcoRule("", "GET"),
			"tags.region":      tcoRule("", "eu"),
		}})))
		f.reverseTagRules = true
		h.RefreshPlan("refresh: the API returns the tag rules in another order", s)
	},

	// The API lowercases service and action names. Services have no validator for capitals.
	"mixed-case-names": func(h *grHarness, _ *tcoFake) {
		s := h.Apply("create: a mixed-case service name", h.null(), tcoPolicies(tcoPolicy("first", "high", j{"services": tcoRule("", "Checkout")})))
		h.RefreshPlan("refresh", s)
	},

	"dpxl-expression": func(h *grHarness, _ *tcoFake) {
		s := h.Apply("create", h.null(), tcoPolicies(tcoPolicy("first", "high", j{"dpxl_expression": "<v1> $l.serviceName == 'checkout'"})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: services instead of dpxl_expression", s, tcoPolicies(tcoPolicy("first", "high", j{"services": tcoRule("", "checkout")})))
		s = h.RefreshPlan("refresh", s)
		structured := tcoPolicy("first", "high", j{
			"services": tcoRule("", "checkout"),
			"actions":  tcoRule("starts_with", "get"),
			"tags":     j{"tags.region": tcoRule("is_not", "eu")},
		})
		s = h.Apply("update: every structured rule", s, tcoPolicies(structured))
		s = h.Apply("update: dpxl_expression instead of the structured rules", s, tcoPolicies(tcoPolicy("first", "high", j{"dpxl_expression": "<v1> $l.serviceName == 'checkout'"})))
		h.RefreshPlan("refresh", s)
	},

	"duplicate-names": func(h *grHarness, _ *tcoFake) {
		h.Apply("create: two policies with one name", h.null(), tcoPolicies(tcoPolicy("first", "high"), tcoPolicy("first", "low")))
	},

	"archive-retention": func(h *grHarness, f *tcoFake) {
		f.archiveRetentions = map[string]bool{"11111111-1111-4111-8111-111111111111": true}
		s := h.Apply("create", h.null(), tcoPolicies(tcoPolicy("first", "high", j{"archive_retention_id": "11111111-1111-4111-8111-111111111111"})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: remove the archive retention", s, tcoPolicies(tcoPolicy("first", "high")))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	},

	"deleted-outside-terraform": func(h *grHarness, f *tcoFake) {
		s := h.Apply("create", h.null(), tcoPolicies(tcoPolicy("first", "high")))
		f.seed("spans", j{"policies": l{}})
		h.RefreshPlan("refresh: every policy was deleted outside Terraform", s)
	},

	"route-missing": func(h *grHarness, f *tcoFake) {
		s := h.Apply("create", h.null(), tcoPolicies(tcoPolicy("first", "high")))
		f.failGets(http.StatusNotFound, "Not Found")
		h.RefreshPlan("refresh: the gateway has no such route", s)
	},

	"update-rejected": func(h *grHarness, f *tcoFake) {
		s := h.Apply("create", h.null(), tcoPolicies(tcoPolicy("first", "high")))
		f.failPuts(http.StatusInternalServerError, "Unexpected server error occurred")
		s = h.Apply("update: the API fails", s, tcoPolicies(with(tcoPolicy("first", "high"), j{"description": "changed"})))
		h.RefreshPlan("refresh", s)
	},

	"destroy-rejected": func(h *grHarness, f *tcoFake) {
		s := h.Apply("create", h.null(), tcoPolicies(tcoPolicy("first", "high")))
		f.failPuts(http.StatusInternalServerError, "Unexpected server error occurred")
		h.Destroy("destroy: the API fails", s)
	},

	"import": func(h *grHarness, f *tcoFake) {
		f.seed("spans", j{"policies": l{
			tcoSpanEntry("seeded", "PRIORITY_TYPE_MEDIUM", j{
				"serviceRule": j{"ruleTypeId": "RULE_TYPE_ID_IS", "name": "svc"},
				"tagRules":    l{j{"tagName": "tags.region", "ruleTypeId": "RULE_TYPE_ID_IS", "tagValue": "eu,us"}},
			}),
		}})
		s := h.Import("import", "anything")
		h.Apply("plan the matching config", s, tcoPolicies(tcoPolicy("seeded", "medium", j{
			"services": tcoRule("", "svc"),
			"tags":     j{"tags.region": tcoRule("", "eu", "us")},
		})))
	},

	"api-error": func(h *grHarness, f *tcoFake) {
		s := h.Apply("create", h.null(), tcoPolicies(tcoPolicy("first", "high")))
		f.failGets(http.StatusInternalServerError, "Unexpected server error occurred")
		h.RefreshPlan("refresh: the API fails", s)
	},

	"config-validation": func(h *grHarness, _ *tcoFake) {
		h.Validate("no priority", tcoPolicies(tcoPolicy("first", "")))
		h.Validate("dpxl_expression and services", tcoPolicies(tcoPolicy("first", "high", j{"dpxl_expression": "<v1> true", "services": tcoRule("", "svc")})))
		h.Validate("capital action name", tcoPolicies(tcoPolicy("first", "high", j{"actions": tcoRule("", "GET")})))
		h.Validate("block tier", tcoPolicies(tcoPolicy("first", "low", j{"quota_based_priority_override": tcoTiers(tcoTier(50, "block"))})))
		h.Validate("tag without names", tcoPolicies(tcoPolicy("first", "high", j{"tags": j{"tags.region": j{"names": l{}}}})))
	},
}

// tcoRumEntry is an overwrite entry of a RUM policy, as another client sends it.
func tcoRumEntry(name, priority string, severities ...string) j {
	sev := make(l, 0, len(severities))
	for _, s := range severities {
		sev = append(sev, s)
	}
	return j{"policy": j{"name": name, "priority": priority}, "rumRules": j{"severities": sev}}
}

var tcoRumScenarios = map[string]func(h *grHarness, f *tcoFake){
	"minimal": func(h *grHarness, _ *tcoFake) {
		s := h.Apply("create", h.null(), tcoPolicies(tcoPolicy("first", "medium", tcoSeverities("error"))))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy: sends an empty list", s)
	},

	"without-severities": func(h *grHarness, _ *tcoFake) {
		h.Apply("create: no severities and no dpxl_expression", h.null(), tcoPolicies(tcoPolicy("first", "medium")))
	},

	"high-priority": func(h *grHarness, _ *tcoFake) {
		h.Apply("create: the API refuses HIGH for RUM", h.null(), tcoPolicies(tcoPolicy("first", "high", tcoSeverities("error"))))
	},

	"full-lifecycle": func(h *grHarness, _ *tcoFake) {
		first := tcoPolicy("first", "low", tcoSeverities("error", "critical"), j{
			"description":                   "first policy",
			"applications":                  tcoRule("", "web"),
			"subsystems":                    tcoRule("is_not", "cx_rum"),
			"quota_based_priority_override": tcoTiers(tcoTier(30, "medium")),
		})
		second := tcoPolicy("second", "block", j{"dpxl_expression": "<v1> true", "enabled": false})
		s := h.Apply("create", h.null(), tcoPolicies(first, second))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: reorder", s, tcoPolicies(second, first))
		s = h.Apply("update: remove a policy", s, tcoPolicies(first))
		s = h.Apply("update: remove the optional fields", s, tcoPolicies(tcoPolicy("first", "low", tcoSeverities("error", "critical"))))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: from severities to dpxl_expression", s, tcoPolicies(tcoPolicy("first", "low", j{"dpxl_expression": "<v1> true"})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: back to severities", s, tcoPolicies(tcoPolicy("first", "low", tcoSeverities("error"))))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	},

	"archive-retention": func(h *grHarness, f *tcoFake) {
		f.archiveRetentions = map[string]bool{"11111111-1111-4111-8111-111111111111": true}
		s := h.Apply("create", h.null(), tcoPolicies(tcoPolicy("first", "medium", tcoSeverities("error"), j{"archive_retention_id": "11111111-1111-4111-8111-111111111111"})))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: remove the archive retention", s, tcoPolicies(tcoPolicy("first", "medium", tcoSeverities("error"))))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	},

	"deleted-outside-terraform": func(h *grHarness, f *tcoFake) {
		s := h.Apply("create", h.null(), tcoPolicies(tcoPolicy("first", "medium", tcoSeverities("error"))))
		f.seed("rum", j{"policies": l{}})
		h.RefreshPlan("refresh: every policy was deleted outside Terraform", s)
	},

	"route-missing": func(h *grHarness, f *tcoFake) {
		s := h.Apply("create", h.null(), tcoPolicies(tcoPolicy("first", "medium", tcoSeverities("error"))))
		f.failGets(http.StatusNotFound, "Not Found")
		h.RefreshPlan("refresh: the gateway has no such route", s)
	},

	"update-rejected": func(h *grHarness, f *tcoFake) {
		s := h.Apply("create", h.null(), tcoPolicies(tcoPolicy("first", "medium", tcoSeverities("error"))))
		f.failPuts(http.StatusInternalServerError, "Unexpected server error occurred")
		s = h.Apply("update: the API fails", s, tcoPolicies(with(tcoPolicy("first", "medium", tcoSeverities("error")), j{"description": "changed"})))
		h.RefreshPlan("refresh", s)
	},

	"destroy-rejected": func(h *grHarness, f *tcoFake) {
		s := h.Apply("create", h.null(), tcoPolicies(tcoPolicy("first", "medium", tcoSeverities("error"))))
		f.failPuts(http.StatusInternalServerError, "Unexpected server error occurred")
		h.Destroy("destroy: the API fails", s)
	},

	"import": func(h *grHarness, f *tcoFake) {
		f.seed("rum", j{"policies": l{tcoRumEntry("seeded", "PRIORITY_TYPE_MEDIUM", "SEVERITY_ERROR")}})
		s := h.Import("import", "anything")
		h.Apply("plan the matching config", s, tcoPolicies(tcoPolicy("seeded", "medium", tcoSeverities("error"))))
	},

	"api-error": func(h *grHarness, f *tcoFake) {
		s := h.Apply("create", h.null(), tcoPolicies(tcoPolicy("first", "medium", tcoSeverities("error"))))
		f.failGets(http.StatusInternalServerError, "Unexpected server error occurred")
		h.RefreshPlan("refresh: the API fails", s)
	},

	"config-validation": func(h *grHarness, _ *tcoFake) {
		h.Validate("no priority", tcoPolicies(tcoPolicy("first", "", tcoSeverities("error"))))
		h.Validate("dpxl_expression and severities", tcoPolicies(tcoPolicy("first", "medium", tcoSeverities("error"), j{"dpxl_expression": "<v1> true"})))
		h.Validate("block tier", tcoPolicies(tcoPolicy("first", "low", tcoSeverities("error"), j{"quota_based_priority_override": tcoTiers(tcoTier(50, "block"))})))
	},
}

// The data sources read every policy of their source type. They take no input.
// tcoFullPolicy is a stored policy with every optional field of the generic policy set.
func tcoFullPolicy(name string) j {
	return j{
		"name":             name,
		"description":      "every field",
		"priority":         "PRIORITY_TYPE_LOW",
		"disabled":         true,
		"applicationRule":  j{"name": "app-a,app-b", "ruleTypeId": "RULE_TYPE_ID_IS"},
		"subsystemRule":    j{"name": "sub", "ruleTypeId": "RULE_TYPE_ID_INCLUDES"},
		"archiveRetention": j{"id": "11111111-1111-4111-8111-111111111111"},
		"priorityOverride": j{"quotaBased": j{"usageTiers": l{j{"dailyQuotaPercentage": 50, "priority": "PRIORITY_TYPE_MEDIUM"}}}},
	}
}

// tcoDataSourceScenarios are the data source scenarios of a source type. full are stored entries
// that together set every optional field, the source rules included.
func tcoDataSourceScenarios(source string, entry func(name, priority string) j, full ...any) map[string]func(h *grHarness) {
	return map[string]func(h *grHarness){
		"read": func(h *grHarness) {
			h.tcoPolicies().seed(source, j{"policies": l{entry("seeded-a", "PRIORITY_TYPE_MEDIUM"), entry("seeded-b", "PRIORITY_TYPE_LOW")}})
			h.ReadDataSource("read the policies", j{})
		},
		"read-every-field": func(h *grHarness) {
			f := h.tcoPolicies()
			f.archiveRetentions = map[string]bool{"11111111-1111-4111-8111-111111111111": true}
			f.seed(source, j{"policies": l(full)})
			h.ReadDataSource("read a policy with every optional field", j{})
		},
		"read-empty": func(h *grHarness) {
			h.ReadDataSource("read when there are no policies", j{})
		},
		"api-error": func(h *grHarness) {
			h.tcoPolicies().failGets(http.StatusInternalServerError, "Unexpected server error occurred")
			h.ReadDataSource("read: the API fails", j{})
		},
		"route-missing": func(h *grHarness) {
			h.tcoPolicies().failGets(http.StatusNotFound, "Not Found")
			h.ReadDataSource("read: the gateway has no such route", j{})
		},
	}
}

var (
	tcoLogsDataSourceScenarios = tcoDataSourceScenarios("logs", func(name, priority string) j {
		return tcoLogEntry(name, priority, "SEVERITY_INFO")
	}, j{"policy": tcoFullPolicy("full"), "logRules": j{"severities": l{"SEVERITY_ERROR", "SEVERITY_CRITICAL"}}},
		// A priority and an override are on the policy or on each target, not both.
		j{"policy": j{"name": "targeted", "priority": "PRIORITY_TYPE_UNSPECIFIED", "targets": l{
			j{"dataset": "logs", "dataspace": "default", "priority": "PRIORITY_TYPE_MEDIUM",
				"priorityOverride": j{"quotaBased": j{"usageTiers": l{j{"dailyQuotaPercentage": 70, "priority": "PRIORITY_TYPE_HIGH"}}}}},
		}}, "logRules": j{"dpxlExpression": "<v1> true"}})
	tcoTracesDataSourceScenarios = tcoDataSourceScenarios("spans", func(name, priority string) j {
		return tcoSpanEntry(name, priority, j{"tagRules": l{j{"tagName": "tags.region", "ruleTypeId": "RULE_TYPE_ID_IS", "tagValue": "eu"}}})
	}, j{"policy": tcoFullPolicy("full"), "spanRules": j{
		"serviceRule": j{"name": "svc-a,svc-b", "ruleTypeId": "RULE_TYPE_ID_IS"},
		"actionRule":  j{"name": "get", "ruleTypeId": "RULE_TYPE_ID_START_WITH"},
		"tagRules": l{
			j{"tagName": "tags.http.method", "ruleTypeId": "RULE_TYPE_ID_IS", "tagValue": "GET,POST"},
			j{"tagName": "tags.region", "ruleTypeId": "RULE_TYPE_ID_IS_NOT", "tagValue": "eu"},
		},
	}})
	tcoRumDataSourceScenarios = tcoDataSourceScenarios("rum", func(name, priority string) j {
		return tcoRumEntry(name, priority, "SEVERITY_ERROR")
	}, j{"policy": tcoFullPolicy("full"), "rumRules": j{"severities": l{"SEVERITY_ERROR", "SEVERITY_CRITICAL"}}})
)

func (h *grHarness) tcoPolicies() *tcoFake {
	h.t.Helper()
	f, ok := h.api.(*tcoFake)
	if !ok {
		h.t.Fatal("expected *tcoFake")
	}
	return f
}
