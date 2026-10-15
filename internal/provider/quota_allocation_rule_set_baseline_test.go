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

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/dataplans"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils/schemadump"
)

const quotaGoldenDir = "testdata/quota_allocation_rule_set_baseline"

// These tests record the behavior of the handwritten coralogix_quota_allocation_rule_set. After
// the generator switch, the golden diff must show only message wording and listed fixes.
//
//	UPDATE_GOLDEN=1 go test ./internal/provider -run TestQuotaAllocationRuleSetBaseline

func TestQuotaAllocationRuleSetBaselineSchema(t *testing.T) {
	var current resource.SchemaResponse
	dataplans.NewQuotaAllocationRuleSetResource().Schema(context.Background(), resource.SchemaRequest{}, &current)
	checkGoldenAt(t, quotaGoldenDir, "schema_current.txt", []byte(schemadump.Text(current.Schema)))
}

func TestQuotaAllocationRuleSetBaselineScenarios(t *testing.T) {
	for name, run := range quotaScenarios {
		t.Run(name, func(t *testing.T) {
			h := newQuotaHarness(t)
			run(h)
			for i := range h.steps {
				// The framework validates nested attributes in map order, so the order of
				// several validation errors is not stable.
				sort.Strings(h.steps[i].ValidateDiags)
			}
			checkGoldenAt(t, quotaGoldenDir, "scenario_"+name+".json", marshalGolden(t, h.steps))
		})
	}
}

func quotaRule(entityType string, allocation float64, extra ...j) j {
	out := j{"entity_type": entityType, "allocation": allocation, "enabled": true, "can_overflow": false}
	for _, e := range extra {
		for k, v := range e {
			out[k] = v
		}
	}
	return out
}

func quotaConfig(rules ...j) j {
	items := make(l, 0, len(rules))
	for _, r := range rules {
		items = append(items, r)
	}
	return j{"rules": items}
}

var quotaScenarios = map[string]func(h *grHarness){
	"minimal": func(h *grHarness) {
		s := h.Apply("create: allocation_type omitted", h.null(), quotaConfig(quotaRule("logs", 60)))
		s = h.Refresh("refresh", s)
		h.Destroy("destroy", s)
	},

	"full-lifecycle": func(h *grHarness) {
		s := h.Apply("create", h.null(), quotaConfig(
			quotaRule("logs", 60, j{"can_overflow": true}),
			quotaRule("metrics", 40, j{"allocation_type": "percentage"}),
		))
		s = h.Refresh("refresh", s)
		s = h.Apply("update: change values", s, quotaConfig(
			quotaRule("logs", 55, j{"enabled": false, "can_overflow": true}),
			quotaRule("metrics", 45, j{"allocation_type": "percentage"}),
		))
		s = h.Apply("update: add a locked units rule", s, quotaConfig(
			quotaRule("logs", 55, j{"enabled": false, "can_overflow": true}),
			quotaRule("metrics", 45, j{"allocation_type": "percentage"}),
			quotaRule("spans", 1, j{"allocation_type": "locked_units"}),
		))
		s = h.Apply("update: remove a rule", s, quotaConfig(
			quotaRule("logs", 55, j{"enabled": false, "can_overflow": true}),
			quotaRule("spans", 1, j{"allocation_type": "locked_units"}),
		))
		s = h.Apply("update: allocation_type removed from a rule", s, quotaConfig(
			quotaRule("logs", 55, j{"enabled": false, "can_overflow": true}),
			quotaRule("spans", 1),
		))
		s = h.Apply("update: clear all rules", s, j{"rules": l{}})
		s = h.Refresh("refresh after clear", s)
		s = h.Apply("update: rules again", s, quotaConfig(quotaRule("logs", 10)))
		h.Destroy("destroy", s)
	},

	"allocation-type-unspecified": func(h *grHarness) {
		s := h.Apply("create: unspecified", h.null(), quotaConfig(quotaRule("logs", 60, j{"allocation_type": "unspecified"})))
		s = h.Refresh("refresh", s)
		s = h.Apply("update: percentage", s, quotaConfig(quotaRule("logs", 60, j{"allocation_type": "percentage"})))
		h.Apply("update: unspecified again", s, quotaConfig(quotaRule("logs", 60, j{"allocation_type": "unspecified"})))
	},

	"float-allocation": func(h *grHarness) {
		s := h.Apply("create: 99.2 is a float32 that reads back as 99.2", h.null(), quotaConfig(quotaRule("logs", 99.2)))
		s = h.Refresh("refresh", s)
		h.Apply("update: 0.1 percent", s, quotaConfig(quotaRule("logs", 0.1), quotaRule("metrics", 12.5)))
	},

	// The account already has a rule set with rules that Coralogix manages. A create answers 409,
	// and the provider replaces the rule set. State shows only the customer rules.
	"create-next-to-managed-rules": func(h *grHarness) {
		h.quota().seedManaged(j{"entityType": "alerts", "allocation": 5, "enabled": true, "canOverflow": false, "allocationType": "QUOTA_ALLOCATION_TYPE_PERCENTAGE"})
		s := h.Apply("create: the rule set exists with managed rules only", h.null(), quotaConfig(quotaRule("logs", 60)))
		s = h.Refresh("refresh", s)
		s = h.Apply("update", s, quotaConfig(quotaRule("logs", 50), quotaRule("metrics", 10)))
		h.Destroy("destroy keeps the managed rules", s)
		h.Refresh("refresh after destroy: state is gone", s)
	},

	// The real API replaces the rule set when a create finds customer rules, so there is no conflict.
	"create-over-customer-rules": func(h *grHarness) {
		h.quota().seedCustomer(j{"entityType": "metrics", "allocation": 10, "enabled": true, "canOverflow": false})
		h.Apply("create: the rule set already has customer rules, and the API replaces them", h.null(), quotaConfig(quotaRule("logs", 60)))
	},

	// The provider refuses a percentage above 100 before it calls the API.
	"create-over-100-percent": func(h *grHarness) {
		h.Apply("create: percentage above 100 is refused by the provider", h.null(), quotaConfig(quotaRule("logs", 120, j{"allocation_type": "unspecified"})))
	},

	// The configuration is valid, and the API fails the write.
	"create-api-error": func(h *grHarness) {
		h.quota().writeStatus = 500
		h.Apply("create: the API answers 500, and no state is written", h.null(), quotaConfig(quotaRule("logs", 10)))
	},

	"update-api-error": func(h *grHarness) {
		s := h.Apply("create", h.null(), quotaConfig(quotaRule("logs", 10)))
		h.quota().writeStatus = 500
		s = h.Apply("update: the API answers 500", s, quotaConfig(quotaRule("logs", 20)))
		h.quota().writeStatus = 0
		h.Refresh("refresh: the rules did not change", s)
	},

	"config-validation": func(h *grHarness) {
		h.Validate("missing rules", j{})
		h.Validate("duplicate entity_type", quotaConfig(quotaRule("logs", 10), quotaRule("logs", 20)))
		h.Validate("percentage above 100", quotaConfig(quotaRule("logs", 101)))
		h.Validate("locked units above 100", quotaConfig(quotaRule("logs", 101, j{"allocation_type": "locked_units"})))
		h.Validate("negative allocation", quotaConfig(quotaRule("logs", -1)))
		h.Validate("allocation that is not a float32", quotaConfig(quotaRule("logs", 0.123456789012)))
		h.Validate("empty entity_type", quotaConfig(quotaRule("", 10)))
		h.Validate("allocation_type in upper case", quotaConfig(quotaRule("logs", 10, j{"allocation_type": "PERCENTAGE"})))
		h.Validate("missing enabled", j{"rules": l{j{"entity_type": "logs", "allocation": 10.0, "can_overflow": false}}})
		h.Validate("missing can_overflow", j{"rules": l{j{"entity_type": "logs", "allocation": 10.0, "enabled": true}}})
		h.Validate("missing allocation", j{"rules": l{j{"entity_type": "logs", "enabled": true, "can_overflow": false}}})
	},

	"import": func(h *grHarness) {
		h.quota().seedManaged(j{"entityType": "alerts", "allocation": 5, "enabled": true, "canOverflow": false})
		h.quota().seedCustomer(
			j{"entityType": "logs", "allocation": 60, "enabled": true, "canOverflow": true, "allocationType": "QUOTA_ALLOCATION_TYPE_PERCENTAGE"},
			j{"entityType": "spans", "allocation": 2, "enabled": false, "canOverflow": false, "allocationType": "QUOTA_ALLOCATION_TYPE_LOCKED_UNITS"},
		)
		s := h.Import("import by the synthetic id", "quota-allocation-rule-set")
		h.Apply("plan the matching config", s, quotaConfig(
			quotaRule("logs", 60, j{"can_overflow": true}),
			quotaRule("spans", 2, j{"allocation_type": "locked_units", "enabled": false}),
		))
	},

	"import-backend-id": func(h *grHarness) {
		h.quota().seedCustomer(j{"entityType": "logs", "allocation": 60, "enabled": true, "canOverflow": true})
		h.Import("import by the id of the rule set", "00000000-0000-4000-8000-000000000001")
	},

	"import-unknown-id": func(h *grHarness) {
		h.quota().seedCustomer(j{"entityType": "logs", "allocation": 60, "enabled": true, "canOverflow": true})
		h.Import("import an id that does not exist", "does-not-exist")
	},

	"import-nothing-to-import": func(h *grHarness) {
		h.quota().forget()
		h.Import("import when the account has no rule set", "quota-allocation-rule-set")
	},

	"read-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), quotaConfig(quotaRule("logs", 10)))
		h.quota().forget()
		h.Refresh("refresh: the rule set was deleted outside Terraform", s)
	},

	"update-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), quotaConfig(quotaRule("logs", 10)))
		h.quota().forget()
		h.Apply("update: the rule set was deleted outside Terraform", s, quotaConfig(quotaRule("logs", 20)))
	},

	"delete-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), quotaConfig(quotaRule("logs", 10)))
		h.quota().forget()
		h.Destroy("destroy: the rule set is gone", s)
	},

	"delete-already-empty": func(h *grHarness) {
		s := h.Apply("create", h.null(), quotaConfig(quotaRule("logs", 10)))
		s = h.Apply("update: clear all rules", s, j{"rules": l{}})
		h.Destroy("destroy: the rule set has no customer rules", s)
	},

	"delete-error-but-cleared": func(h *grHarness) {
		s := h.Apply("create", h.null(), quotaConfig(quotaRule("logs", 10)))
		h.quota().deleteStatus, h.quota().deleteClears = 500, true
		h.Destroy("destroy: the API answers 500 and clears the rules", s)
	},

	"delete-error": func(h *grHarness) {
		s := h.Apply("create", h.null(), quotaConfig(quotaRule("logs", 10)))
		h.quota().deleteStatus = 500
		h.Destroy("destroy: the API answers 500 and keeps the rules", s)
	},
}
