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
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/dataplans"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	tcoPolicys "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/policies_service"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var tcoPoliciesRumResourceName = "coralogix_tco_policies_rum.test"

// skipIfRumPoliciesDisabled skips the test when the RUM TCO policies feature is not
// enabled for the account. The feature is gated per company; a gated account answers the
// atomic-overwrite write with a 400 whose body carries a FAILED_PRECONDITION mentioning
// that RUM quota policies are not enabled. The gate is enforced per policy
// ("policies[0] failed custom validation"), so the probe must submit at least one policy —
// an empty overwrite passes even when the feature is off. On success the probe clears the
// collection again, which is harmless in the acceptance account (every test here overwrites
// the whole collection anyway).
func skipIfRumPoliciesDisabled(t *testing.T) {
	t.Helper()
	// Only probe under acceptance runs; otherwise let resource.Test perform its standard
	// TF_ACC skip.
	if os.Getenv(resource.EnvTfAcc) == "" {
		return
	}
	clients, err := testAccNewClientSet()
	if err != nil {
		t.Fatalf("failed to build acceptance client: %s", err)
	}
	ctx := context.Background()
	probeName := "tf-acc-rum-feature-probe"
	probe := tcoPolicys.AtomicOverwriteRumPoliciesRequest{
		Policies: []tcoPolicys.CreateRumPolicyRequest{
			{
				Policy:   tcoPolicys.CreateGenericPolicyRequest{Name: probeName, Priority: tcoPolicys.QUOTAV1PRIORITY_PRIORITY_TYPE_LOW},
				RumRules: tcoPolicys.LogRules{Severities: []tcoPolicys.QuotaV1Severity{tcoPolicys.QUOTAV1SEVERITY_SEVERITY_ERROR}},
			},
		},
	}
	_, _, err = clients.TCOPolicies().
		PoliciesServiceAtomicOverwriteRumPolicies(ctx).
		AtomicOverwriteRumPoliciesRequest(probe).
		Execute()
	if err == nil {
		// Feature is on — clean up the probe policy before the test runs.
		_, _, _ = clients.TCOPolicies().
			PoliciesServiceAtomicOverwriteRumPolicies(ctx).
			AtomicOverwriteRumPoliciesRequest(*tcoPolicys.NewAtomicOverwriteRumPoliciesRequestWithDefaults()).
			Execute()
		return
	}

	var apiErr *tcoPolicys.GenericOpenAPIError
	if errors.As(err, &apiErr) {
		body := string(apiErr.Body())
		if strings.Contains(body, "RUM quota policies are not enabled") || strings.Contains(body, "FAILED_PRECONDITION") {
			t.Skipf("RUM TCO policies feature is not enabled for this account; skipping. Backend said: %s", body)
		}
	}
	t.Fatalf("unexpected error probing RUM TCO policies availability: %s", err)
}

func TestAccCoralogixResourceTCOPoliciesRumCreate(t *testing.T) {
	skipIfRumPoliciesDisabled(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccTCOPoliciesRumCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceTCOPoliciesRum(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.0.name", "Example rum tco_policy 1"),
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.0.priority", "low"),
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.0.order", "1"),
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.0.severities.#", "3"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesRumResourceName, "policies.0.severities.*", "error"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesRumResourceName, "policies.0.severities.*", "warning"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesRumResourceName, "policies.0.severities.*", "info"),
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.0.applications.rule_type", "starts_with"),
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.0.applications.names.0", "prod"),
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.0.subsystems.rule_type", "is"),
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.0.subsystems.names.#", "2"),
					// archive_retention_id is sourced from the archive-retentions data source, so
					// the fixture is portable across accounts; assert it round-tripped rather than a
					// hard-coded value.
					resource.TestCheckResourceAttrSet(tcoPoliciesRumResourceName, "policies.0.archive_retention_id"),
					// RUM policies have no dataset routing, so targets must never appear in state.
					resource.TestCheckNoResourceAttr(tcoPoliciesRumResourceName, "policies.0.targets"),

					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.1.name", "Example rum tco_policy 2"),
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.1.priority", "medium"),
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.1.order", "2"),
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.1.severities.#", "2"),
				),
			},
		},
	})
}

// TestAccCoralogixResourceTCOPoliciesRum_dpxl_expression covers a RUM policy whose matcher
// is a DataPrime expression instead of severities. The two are mutually exclusive at the
// API, so this fixture omits severities entirely.
func TestAccCoralogixResourceTCOPoliciesRum_dpxl_expression(t *testing.T) {
	skipIfRumPoliciesDisabled(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccTCOPoliciesRumCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceTCOPoliciesRumDpxlExpression(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.0.name", "Example rum tco_policy with DPXL expression"),
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.0.priority", "high"),
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.0.dpxl_expression", "<v1> $d.severity == 'Error'"),
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.0.severities.#", "0"),
				),
			},
		},
	})
}

// TestAccCoralogixResourceTCOPoliciesRum_quotaOverrideNoPriority is the key regression: a
// policy that omits `priority` and relies on `quota_based_priority_override`. The backend
// injects a `block` fallback priority; `priority` being Optional+Computed must absorb it so
// the follow-up plan is empty rather than perpetually diffing.
func TestAccCoralogixResourceTCOPoliciesRum_quotaOverrideNoPriority(t *testing.T) {
	skipIfRumPoliciesDisabled(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccTCOPoliciesRumCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceTCOPoliciesRumQuotaOverrideNoPriority(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.0.quota_based_priority_override.usage_tiers.#", "2"),
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.0.quota_based_priority_override.usage_tiers.0.daily_quota_percentage", "50"),
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.0.quota_based_priority_override.usage_tiers.0.priority", "medium"),
					// Server injected the fallback priority; Computed absorbed it.
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.0.priority", "block"),
				),
			},
			{
				// Same config again must produce no diff.
				Config:   testAccCoralogixResourceTCOPoliciesRumQuotaOverrideNoPriority(),
				PlanOnly: true,
			},
		},
	})
}

// TestAccCoralogixResourceTCOPoliciesRum_dpxl_replaces_severities verifies switching a
// policy's matcher from severities to a DPXL expression.
func TestAccCoralogixResourceTCOPoliciesRum_dpxl_replaces_severities(t *testing.T) {
	skipIfRumPoliciesDisabled(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccTCOPoliciesRumCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceTCOPoliciesRumSeveritiesOnly(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.0.severities.#", "1"),
					resource.TestCheckTypeSetElemAttr(tcoPoliciesRumResourceName, "policies.0.severities.*", "info"),
					resource.TestCheckNoResourceAttr(tcoPoliciesRumResourceName, "policies.0.dpxl_expression"),
				),
			},
			{
				Config: testAccCoralogixResourceTCOPoliciesRumDpxlOnly(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.0.dpxl_expression", "<v1> $d.severity == 'Error'"),
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.0.severities.#", "0"),
				),
			},
		},
	})
}

func testAccTCOPoliciesRumCheckDestroy(s *terraform.State) error {
	clients, err := testAccNewClientSet()
	if err != nil {
		return fmt.Errorf("failed to build acceptance client: %w", err)
	}
	client := clients.TCOPolicies()
	ctx := context.TODO()
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_tco_policies_rum" {
			continue
		}

		if result, _, err := client.PoliciesServiceGetCompanyPolicies(ctx).SourceType(dataplans.RumSource).Execute(); err == nil {
			if len(result.GetPolicies()) > 0 {
				return fmt.Errorf("rum tco-policies still exist: %s", utils.FormatJSON(result))
			}
		}
	}

	return nil
}

func testAccCoralogixResourceTCOPoliciesRum() string {
	return `data "coralogix_archive_retentions" "all" {}

resource "coralogix_tco_policies_rum" "test" {
  policies = [
    {
      name                 = "Example rum tco_policy 1"
      priority             = "low"
      severities           = ["error", "warning", "info"]
      applications = {
        rule_type = "starts_with"
        names     = ["prod"]
      }
      subsystems = {
        rule_type = "is"
        names     = ["mobile", "web"]
      }
      archive_retention_id = data.coralogix_archive_retentions.all.retentions[1].id
    },
    {
      name       = "Example rum tco_policy 2"
      priority   = "medium"
      severities = ["error", "critical"]
      subsystems = {
        rule_type = "is"
        names     = ["checkout"]
      }
    },
  ]
}
`
}

func testAccCoralogixResourceTCOPoliciesRumDpxlExpression() string {
	return `resource "coralogix_tco_policies_rum" "test" {
  policies = [
    {
      name            = "Example rum tco_policy with DPXL expression"
      description     = "DPXL-based matcher for the RUM policy"
      priority        = "high"
      dpxl_expression = "<v1> $d.severity == 'Error'"
    },
  ]
}
`
}

func testAccCoralogixResourceTCOPoliciesRumQuotaOverrideNoPriority() string {
	return `resource "coralogix_tco_policies_rum" "test" {
  policies = [
    {
      name       = "Example rum tco_policy with quota-based override"
      severities = ["info", "warning"]
      quota_based_priority_override = {
        usage_tiers = [
          { daily_quota_percentage = 50, priority = "medium" },
          { daily_quota_percentage = 80, priority = "low" },
        ]
      }
    },
  ]
}
`
}

func testAccCoralogixResourceTCOPoliciesRumSeveritiesOnly() string {
	return `resource "coralogix_tco_policies_rum" "test" {
  policies = [
    {
      name       = "Example rum tco_policy migration"
      priority   = "high"
      severities = ["info"]
    },
  ]
}
`
}

func testAccCoralogixResourceTCOPoliciesRumDpxlOnly() string {
	return `resource "coralogix_tco_policies_rum" "test" {
  policies = [
    {
      name            = "Example rum tco_policy migration"
      priority        = "high"
      dpxl_expression = "<v1> $d.severity == 'Error'"
    },
  ]
}
`
}
