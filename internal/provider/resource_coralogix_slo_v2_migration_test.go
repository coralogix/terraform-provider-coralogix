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
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

const (
	// The generated upgrade tests of every resource share this variable and run in one CI shard.
	sloV2UpgradeAcceptanceEnv = "CORALOGIX_GENERATED_UPGRADE_ACC"
	sloV2UpgradeSource        = "registry.terraform.io/coralogix/coralogix"
	// The release before the generated resource. The SLO in it is handwritten.
	sloV2UpgradeReleasedVersion = "= 3.21.0"
)

// TestAccCoralogixResourceSloV2SLIsGeneratedUpgrade creates a window-based SLO and APM SLOs with
// the released provider, then plans the same config with this build. A user who upgrades must see
// no change, and a later update and an import must work. The generated upgrade test covers the
// request-based SLI.
func TestAccCoralogixResourceSloV2SLIsGeneratedUpgrade(t *testing.T) {
	requireSLOV2UpgradeAcceptance(t)
	service := os.Getenv(sloV2APMServiceEnvVar)

	cases := []struct {
		name             string
		initial, updated string
		apm              bool
	}{
		{
			name:    "window-based",
			initial: testAccCoralogixSLOV2WindowSLO(`missing_data_strategy = "good"`, `labels = { team = "a" }`),
			updated: testAccCoralogixSLOV2WindowSLO(`missing_data_strategy = "bad"`, `labels = { team = "b" }`),
		},
		{
			name:    "apm-error",
			initial: testAccCoralogixSLOV2APMSLI(service, `error_config = {}`, ""),
			updated: testAccCoralogixSLOV2APMSLI(service, `error_config = {}`, `
    filters = [{
      key    = "http.status_code"
      values = ["500", "503"]
    }]`),
			apm: true,
		},
		{
			name: "apm-latency",
			initial: testAccCoralogixSLOV2APMSLI(service, `
      latency_config = {
        time_window = "5_minutes"
        threshold   = 500.0
        quantile = {
          percentile = 0.95
        }
      }`, ""),
			updated: testAccCoralogixSLOV2APMSLI(service, `
      latency_config = {
        time_window = "5_minutes"
        threshold   = 250
        average     = {}
      }`, ""),
			apm: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resource.Test(t, resource.TestCase{
				PreCheck: func() {
					if tc.apm {
						testAccSLOV2APMPreCheck(t)
					} else {
						testAccPreCheck(t)
					}
				},
				CheckDestroy: testAccSLOV2CheckDestroy,
				Steps: []resource.TestStep{
					{
						Config: tc.initial,
						ExternalProviders: map[string]resource.ExternalProvider{
							"coralogix": {Source: sloV2UpgradeSource, VersionConstraint: sloV2UpgradeReleasedVersion},
						},
					},
					{
						// This build reads the state of the released provider and plans no change.
						Config:                   tc.initial,
						ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(sloV2ResourceName, plancheck.ResourceActionNoop)},
							PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
						},
					},
					{
						Config:                   tc.updated,
						ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(sloV2ResourceName, plancheck.ResourceActionUpdate)},
							PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
						},
					},
					{
						ResourceName:             sloV2ResourceName,
						ImportState:              true,
						ImportStateVerify:        true,
						ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
					},
				},
			})
		})
	}
}

func requireSLOV2UpgradeAcceptance(t *testing.T) {
	t.Helper()
	if os.Getenv(sloV2UpgradeAcceptanceEnv) == "" {
		t.Skipf("set %s=1 to run the registry-backed coralogix_slo_v2 upgrade tests", sloV2UpgradeAcceptanceEnv)
	}
	if namespace := os.Getenv(resource.EnvTfAccProviderNamespace); namespace != "coralogix" {
		t.Fatalf("set %s=coralogix to run the registry-backed coralogix_slo_v2 upgrade tests", resource.EnvTfAccProviderNamespace)
	}
}
