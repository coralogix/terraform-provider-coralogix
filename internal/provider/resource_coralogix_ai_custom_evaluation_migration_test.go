// Copyright 2026 Coralogix Ltd.
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

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

const (
	aiCustomEvaluationMigrationAcceptanceEnv  = "CORALOGIX_GENERATED_UPGRADE_ACC"
	aiCustomEvaluationMigrationProviderSource = "registry.terraform.io/coralogix/coralogix"
	// The release before the generated resource. The custom evaluation in it is handwritten.
	aiCustomEvaluationMigrationReleasedVersion = "= 3.25.0"
)

// TestAccCoralogixResourceAICustomEvaluationGeneratedUpgrade creates a custom evaluation with the
// released provider, then plans the same config with this build. A user who upgrades must see no
// change, and a later update and an import must work. The name ends in GeneratedUpgrade, so the
// generated-upgrade shard of the provider-migration job runs it. It needs an environment whose
// API has the read of one custom evaluation, and a tenant that has an AI application.
func TestAccCoralogixResourceAICustomEvaluationGeneratedUpgrade(t *testing.T) {
	if os.Getenv(aiCustomEvaluationMigrationAcceptanceEnv) == "" {
		t.Skipf("set %s=1 to run the registry-backed upgrade test", aiCustomEvaluationMigrationAcceptanceEnv)
	}
	if namespace := os.Getenv(resource.EnvTfAccProviderNamespace); namespace != "coralogix" {
		t.Fatalf("set %s=coralogix to run the registry-backed upgrade test", resource.EnvTfAccProviderNamespace)
	}

	application := testAccFirstAICustomEvaluationApplication(t)
	cases := []struct {
		name    string
		initial func(name string) string
		updated func(name string) string
	}{
		{
			// An application, the criteria with flags and examples, and the system prompt off.
			name: "application-and-criteria",
			initial: func(name string) string {
				return testAccCoralogixResourceAICustomEvaluation(application, name, false, testAccAICustomEvaluationCreateCriteria())
			},
			updated: func(name string) string {
				return testAccCoralogixResourceAICustomEvaluation(application, name, true, testAccAICustomEvaluationUpdateCriteria())
			},
		},
		{
			// Only the required attributes: every default is the one of the released provider.
			name: "minimal",
			initial: func(name string) string {
				return testAccCoralogixResourceAICustomEvaluationMinimal(name)
			},
			updated: func(name string) string {
				return testAccCoralogixResourceAICustomEvaluationWithoutApplicationLinks(name, testAccAICustomEvaluationAcceptableExamplesCriteria())
			},
		},
		{
			// No application links, with the criteria, then the examples removed.
			name: "criteria-without-links",
			initial: func(name string) string {
				return testAccCoralogixResourceAICustomEvaluationWithoutApplicationLinks(name, testAccAICustomEvaluationCreateCriteria())
			},
			updated: func(name string) string {
				return testAccCoralogixResourceAICustomEvaluationWithoutApplicationLinks(name, testAccAICustomEvaluationEmptyExamplesCriteria())
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name := acctest.RandomWithPrefix("tf-acc-ai-custom-evaluation-upgrade")
			initial, updated := tc.initial(name), tc.updated(name)
			resource.Test(t, resource.TestCase{
				PreCheck:     func() { testAccPreCheck(t) },
				CheckDestroy: testAccCheckAICustomEvaluationDestroy,
				Steps: []resource.TestStep{
					{
						Config: initial,
						ExternalProviders: map[string]resource.ExternalProvider{
							"coralogix": {Source: aiCustomEvaluationMigrationProviderSource, VersionConstraint: aiCustomEvaluationMigrationReleasedVersion},
						},
						Check: resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "name", name),
					},
					{
						// This build reads the state of the released provider and plans no change.
						Config:                   initial,
						ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(aiCustomEvaluationResourceName, plancheck.ResourceActionNoop)},
							PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
						},
					},
					{
						Config:                   updated,
						ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(aiCustomEvaluationResourceName, plancheck.ResourceActionUpdate)},
							PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
						},
					},
					{
						ResourceName:             aiCustomEvaluationResourceName,
						ImportState:              true,
						ImportStateVerify:        true,
						ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
					},
				},
			})
		})
	}
}
