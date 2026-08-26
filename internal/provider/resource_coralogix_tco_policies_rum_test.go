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
	"fmt"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/dataplans"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var tcoPoliciesRumResourceName = "coralogix_tco_policies_rum.test"

func TestAccCoralogixResourceTCOPoliciesRumCreate(t *testing.T) {
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
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccTCOPoliciesRumCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceTCOPoliciesRumDpxlExpression(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.0.name", "Example rum tco_policy with DPXL expression"),
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.0.priority", "medium"),
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.0.dpxl_expression", "<v1> $d.severity == 'Error'"),
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.0.severities.#", "0"),
				),
			},
		},
	})
}

// TestAccCoralogixResourceTCOPoliciesRum_quotaOverride covers a policy driven by
// `quota_based_priority_override`, where `priority` is the fallback applied once all tiers
// are exhausted. The second step re-applies the same config to assert idempotency.
func TestAccCoralogixResourceTCOPoliciesRum_quotaOverride(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccTCOPoliciesRumCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceTCOPoliciesRumQuotaOverride(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.0.priority", "block"),
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.0.quota_based_priority_override.usage_tiers.#", "2"),
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.0.quota_based_priority_override.usage_tiers.0.daily_quota_percentage", "50"),
					resource.TestCheckResourceAttr(tcoPoliciesRumResourceName, "policies.0.quota_based_priority_override.usage_tiers.0.priority", "medium"),
				),
			},
			{
				// Same config again must produce no diff.
				Config:   testAccCoralogixResourceTCOPoliciesRumQuotaOverride(),
				PlanOnly: true,
			},
		},
	})
}

// TestAccCoralogixResourceTCOPoliciesRum_dpxl_replaces_severities verifies switching a
// policy's matcher from severities to a DPXL expression.
func TestAccCoralogixResourceTCOPoliciesRum_dpxl_replaces_severities(t *testing.T) {
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
      priority        = "medium"
      dpxl_expression = "<v1> $d.severity == 'Error'"
    },
  ]
}
`
}

func testAccCoralogixResourceTCOPoliciesRumQuotaOverride() string {
	return `resource "coralogix_tco_policies_rum" "test" {
  policies = [
    {
      name       = "Example rum tco_policy with quota-based override"
      priority   = "block"
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
      priority   = "medium"
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
      priority        = "medium"
      dpxl_expression = "<v1> $d.severity == 'Error'"
    },
  ]
}
`
}
