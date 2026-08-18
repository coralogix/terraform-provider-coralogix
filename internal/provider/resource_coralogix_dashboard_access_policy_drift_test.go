// Copyright 2026 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccCoralogixResourceDashboardAccessPolicyAddedLater covers adopting an
// access policy on a dashboard that already exists: the dashboard is created
// without `access_policy`, so state takes the policy the backend returns, and
// the practitioner writes that policy into the configuration later.
//
// Step 2 uses jsonencode, the form the documented example uses. The provider
// stores a computed policy in the same shape, so the configured text and the
// stored text match and the plan converges. That assertion is the regression
// guard for the perpetual `auto_refresh = (known after apply)` diff.
//
// Step 3 rewrites the same policy as a formatted heredoc. That text cannot
// match the stored one, so the plan may stay busy; the step tolerates either
// outcome rather than asserting the gap.
//
// Every step asserts what must hold in any version: the applied policy is the
// configured one, the dashboard is updated in place and never replaced, and
// `auto_refresh` keeps its value.
func TestAccCoralogixResourceDashboardAccessPolicyAddedLater(t *testing.T) {
	name := dashboardOpenAPIFixtureName("TestAccCoralogixResourceDashboardAccessPolicyAddedLater")
	// jsonencode(jsondecode(...)) re-encodes the fixture in the canonical
	// shape jsonencode produces, which is what the documented example writes.
	canonicalPolicyBlock := fmt.Sprintf("  access_policy = jsonencode(jsondecode(<<EOT\n%s\nEOT\n  ))\n", testAccCoralogixDashboardAccessPolicyPretty())
	prettyPolicyBlock := fmt.Sprintf("  access_policy = <<EOT\n%s\nEOT\n", testAccCoralogixDashboardAccessPolicyPretty())
	autoRefreshBlock := "  auto_refresh = {\n    type = \"off\"\n  }\n"
	var dashboardID, autoRefreshType string

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			// 1. Created without access_policy. State takes the backend's text.
			{
				Config: testAccDashboardAccessPolicyDriftConfig(name, "", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCaptureDashboardAttribute(t, "id", &dashboardID),
					testAccCaptureDashboardAttribute(t, "auto_refresh.type", &autoRefreshType),
					testAccCaptureDashboardAttribute(t, "access_policy", nil),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			// 2. The policy is adopted into the configuration with jsonencode.
			//    The plan must converge.
			{
				Config: testAccDashboardAccessPolicyDriftConfig(name, canonicalPolicyBlock, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckDashboardAccessPolicy(dashboardResourceName, testAccCoralogixDashboardAccessPolicyPretty()),
					testAccCheckDashboardAttributeUnchanged("id", &dashboardID),
					testAccCheckDashboardAttributeUnchanged("auto_refresh.type", &autoRefreshType),
					testAccCaptureDashboardAttribute(t, "access_policy", nil),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			// 3. The same policy written as a formatted heredoc. The stored
			//    text cannot match, so either outcome is accepted.
			{
				Config: testAccDashboardAccessPolicyDriftConfig(name, prettyPolicyBlock, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckDashboardAccessPolicy(dashboardResourceName, testAccCoralogixDashboardAccessPolicyPretty()),
					testAccCheckDashboardAttributeUnchanged("id", &dashboardID),
					testAccCheckDashboardAttributeUnchanged("auto_refresh.type", &autoRefreshType),
				),
				ExpectNonEmptyPlan: true,
			},
			// 4. With auto_refresh in the configuration the plan is empty.
			{
				Config: testAccDashboardAccessPolicyDriftConfig(name, prettyPolicyBlock, autoRefreshBlock),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckDashboardAccessPolicy(dashboardResourceName, testAccCoralogixDashboardAccessPolicyPretty()),
					testAccCheckDashboardAttributeUnchanged("id", &dashboardID),
					resource.TestCheckResourceAttr(dashboardResourceName, "auto_refresh.type", "off"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// testAccCaptureDashboardAttribute records a dashboard attribute for later
// comparison and logs it. Pass a nil target to only log.
func testAccCaptureDashboardAttribute(t *testing.T, attribute string, target *string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		value, err := testAccDashboardAttribute(state, attribute)
		if err != nil {
			return err
		}

		t.Logf("dashboard %s = %q", attribute, value)
		if target != nil {
			*target = value
		}

		return nil
	}
}

func testAccCheckDashboardAttributeUnchanged(attribute string, previous *string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		value, err := testAccDashboardAttribute(state, attribute)
		if err != nil {
			return err
		}
		if value != *previous {
			return fmt.Errorf("dashboard %s = %q, want the previous step's %q", attribute, value, *previous)
		}

		return nil
	}
}

func testAccDashboardAttribute(state *terraform.State, attribute string) (string, error) {
	resourceState, ok := state.RootModule().Resources[dashboardResourceName]
	if !ok || resourceState.Primary == nil {
		return "", fmt.Errorf("resource %s not found", dashboardResourceName)
	}

	return resourceState.Primary.Attributes[attribute], nil
}

func testAccDashboardAccessPolicyDriftConfig(name, accessPolicyBlock, autoRefreshBlock string) string {
	return fmt.Sprintf(`resource "coralogix_dashboard" test {
  name = %q

%s%s
  time_frame = {
    relative = {
      duration = "seconds:900"
    }
  }

  layout = {
    sections = [{
      rows = [{
        height = 19
        widgets = [
          %s
        ]
      }]
    }]
  }
}
`, name, accessPolicyBlock, autoRefreshBlock, testAccCoralogixResourceDashboardCountWidget())
}
