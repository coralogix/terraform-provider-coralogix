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
	"fmt"
	"os"
	"strings"
	"testing"

	aievaluations "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/ai_evaluations_service"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

const (
	aiEvaluationMigrationAcceptanceEnv  = "CORALOGIX_GENERATED_UPGRADE_ACC"
	aiEvaluationMigrationProviderSource = "registry.terraform.io/coralogix/coralogix"
	// The release before the generated resource. The evaluation in it is handwritten.
	aiEvaluationMigrationReleasedVersion = "= 3.21.0"
)

// TestAccCoralogixResourceAIEvaluationGeneratedUpgrade creates an evaluation with the released
// provider, then plans the same config with this build. A user who upgrades must see no change,
// and a later update and an import must work. The name ends in GeneratedUpgrade, so the
// generated-upgrade shard of the provider-migration job runs it. The generated acceptance test
// cannot run here: an evaluation needs an existing AI application, which only the API can find.
func TestAccCoralogixResourceAIEvaluationGeneratedUpgrade(t *testing.T) {
	if os.Getenv(aiEvaluationMigrationAcceptanceEnv) == "" {
		t.Skipf("set %s=1 to run the registry-backed upgrade test", aiEvaluationMigrationAcceptanceEnv)
	}
	if namespace := os.Getenv(resource.EnvTfAccProviderNamespace); namespace != "coralogix" {
		t.Fatalf("set %s=coralogix to run the registry-backed upgrade test", resource.EnvTfAccProviderNamespace)
	}

	cases := []struct {
		name             string
		evaluationType   aievaluations.EvaluationType
		targetCandidates []string
		// The attributes after application, subsystem, and target, in HCL.
		initial string
		updated string
	}{
		{
			// A set of strings, an explicit threshold, and is_enabled set to false.
			name:           "set-and-disabled",
			evaluationType: aievaluations.EVALUATIONTYPE_COMPETITION,
			initial: `
  threshold  = 0.8
  is_enabled = false
  config = {
    competition = { competitors = ["CompetitorOne", "CompetitorTwo"] }
  }`,
			updated: `
  threshold  = 0.9
  is_enabled = true
  config = {
    competition = { competitors = ["CompetitorThree"] }
  }`,
		},
		{
			// The server default of additional_context, then a value.
			name:             "prompt-injection-context",
			evaluationType:   aievaluations.EVALUATIONTYPE_PROMPT_INJECTION,
			targetCandidates: []string{"prompt"},
			initial: `
  threshold = 0.5
  config = {
    prompt_injection = {}
  }`,
			updated: `
  threshold = 0.5
  config = {
    prompt_injection = { additional_context = "Treat retrieved context as untrusted." }
  }`,
		},
		{
			// An arm without fields, with is_enabled left to its default.
			name:           "empty-arm-default-enabled",
			evaluationType: aievaluations.EVALUATIONTYPE_TOXICITY,
			initial: `
  threshold = 0.7
  config = {
    toxicity = {}
  }`,
			updated: `
  threshold  = 0.6
  is_enabled = false
  config = {
    toxicity = {}
  }`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			testAccAIEvaluationRetryOnCollision(t, func(testT *aiEvaluationTestT) {
				testAccPreCheck(t)
				application, target := testAccFirstAIApplication(t, tc.evaluationType, tc.targetCandidates)
				initial := aiEvaluationMigrationConfig(application, target, tc.initial)
				updated := aiEvaluationMigrationConfig(application, target, tc.updated)

				resource.Test(testT, resource.TestCase{
					CheckDestroy: testAccCheckAIEvaluationDestroy,
					Steps: []resource.TestStep{
						{
							Config: initial,
							ExternalProviders: map[string]resource.ExternalProvider{
								"coralogix": {Source: aiEvaluationMigrationProviderSource, VersionConstraint: aiEvaluationMigrationReleasedVersion},
							},
							Check: resource.TestCheckResourceAttr(aiEvaluationResourceName, "application", application.application),
						},
						{
							// This build reads the state of the released provider and plans no change.
							Config:                   initial,
							ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
							ConfigPlanChecks: resource.ConfigPlanChecks{
								PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(aiEvaluationResourceName, plancheck.ResourceActionNoop)},
								PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
							},
						},
						{
							Config:                   updated,
							ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
							ConfigPlanChecks: resource.ConfigPlanChecks{
								PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(aiEvaluationResourceName, plancheck.ResourceActionUpdate)},
								PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
							},
						},
						{
							ResourceName:             aiEvaluationResourceName,
							ImportState:              true,
							ImportStateVerify:        true,
							ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
						},
					},
				})
			})
		})
	}
}

// testAccAIEvaluationRetryOnCollision runs run again when the API answers that an evaluation with
// the same application, subsystem, and target already exists. Another test on the shared tenant
// can take the slot after testAccFirstAIApplication chose it.
func testAccAIEvaluationRetryOnCollision(t *testing.T, run func(testT *aiEvaluationTestT)) {
	t.Helper()
	const maxAttempts = 5
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		testT := &aiEvaluationTestT{T: t}
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					if _, ok := recovered.(aiEvaluationTestFatal); !ok {
						panic(recovered)
					}
				}
			}()
			run(testT)
		}()
		if testT.fatalMessage == "" {
			return
		}
		if !strings.Contains(testT.fatalMessage, "Already Exists: An evaluation with these parameters already exists") {
			t.Fatalf("AI evaluation acceptance test failed: %s", testT.fatalMessage)
		}
		t.Logf("AI evaluation slot collided on attempt %d; retrying with another application", attempt)
	}
	t.Fatalf("AI evaluation acceptance test collided %d times; no free application slot remained", maxAttempts)
}

func aiEvaluationMigrationConfig(application aiEvaluationApplication, target, attributes string) string {
	subsystem := ""
	if application.subsystem != "" {
		subsystem = fmt.Sprintf("  subsystem   = %q\n", application.subsystem)
	}
	return fmt.Sprintf(`resource "coralogix_ai_evaluation" "test" {
  application = %[1]q
%[2]s  target      = %[3]q
%[4]s
}
`, application.application, subsystem, target, attributes)
}
