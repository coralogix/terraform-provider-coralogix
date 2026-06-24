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
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var aiCustomEvaluationResourceName = "coralogix_ai_custom_evaluation.test"

func TestAccCoralogixResourceAICustomEvaluation(t *testing.T) {
	application := &aiEvaluationApplication{}
	name := acctest.RandomWithPrefix("tf-acc-ai-custom-evaluation")
	updatedName := acctest.RandomWithPrefix("tf-acc-ai-custom-evaluation-updated")
	configDir := t.TempDir()
	createConfigFile := filepath.Join(configDir, "create.tf")
	updateConfigFile := filepath.Join(configDir, "update.tf")

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			*application = testAccFirstAICustomEvaluationApplication(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAICustomEvaluationDestroy,
		Steps: []resource.TestStep{
			{
				ConfigFile: testAccAICustomEvaluationConfigFile(
					t,
					createConfigFile,
					application,
					name,
					false,
					testAccAICustomEvaluationCreateCriteria(),
				),
				Check: testAccAICustomEvaluationCheck(
					application,
					name,
					"quality",
					false,
					resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "description", "Flags competitor references in assistant responses."),
					resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "instructions", "Score whether {response} mentions competitor products.\nTreat each assistant answer independently."),
					resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "criteria.acceptable.flags", "Does not mention competitor products.\nAnswer stays focused on our product."),
					resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "criteria.acceptable.examples.0", "User: which tool should I use?\nAssistant: Our product is a strong fit."),
					resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "criteria.prohibited.flags", "Mentions a competitor product.\nNames another vendor as the recommended option."),
					resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "criteria.prohibited.examples.0", "User: which tool should I use?\nAssistant: CompetitorX is a strong fit."),
				),
			},
			{
				ResourceName:      aiCustomEvaluationResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ConfigFile: testAccAICustomEvaluationConfigFile(
					t,
					updateConfigFile,
					application,
					updatedName,
					true,
					testAccAICustomEvaluationUpdateCriteria(),
				),
				Check: testAccAICustomEvaluationCheck(
					application,
					updatedName,
					"security",
					true,
					resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "description", "Flags responses that recommend competitor tools."),
					resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "instructions", "Score whether {response} recommends competitor products.\nOnly evaluate the final assistant response."),
					resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "criteria.acceptable.flags", "Does not recommend competitor products.\nMentions only our product or neutral guidance."),
					resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "criteria.acceptable.examples.0", "User: what should I buy?\nAssistant: Our product covers that workflow."),
					resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "criteria.prohibited.flags", "Recommends a competitor product.\nNames a competitor as the best choice."),
					resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "criteria.prohibited.examples.0", "User: what should I buy?\nAssistant: You should buy CompetitorY."),
				),
			},
		},
	})
}

func TestAccCoralogixResourceAICustomEvaluationMissingApplicationCreate(t *testing.T) {
	missingApplication := &aiEvaluationApplication{
		application: acctest.RandomWithPrefix("tf-acc-missing-ai-application"),
		subsystem:   acctest.RandomWithPrefix("tf-acc-missing-ai-subsystem"),
	}
	name := acctest.RandomWithPrefix("tf-acc-ai-custom-evaluation")
	configDir := t.TempDir()
	createConfigFile := filepath.Join(configDir, "create.tf")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAICustomEvaluationDestroy,
		Steps: []resource.TestStep{
			{
				ConfigFile: testAccAICustomEvaluationConfigFile(
					t,
					createConfigFile,
					missingApplication,
					name,
					false,
					testAccAICustomEvaluationCreateCriteria(),
				),
				ExpectError: regexp.MustCompile(`AI application not found|No AI application named`),
			},
		},
	})
}

func TestAccCoralogixResourceAICustomEvaluationWithoutApplicationsOrCriteria(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-ai-custom-evaluation")
	configDir := t.TempDir()
	createConfigFile := filepath.Join(configDir, "create.tf")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAICustomEvaluationDestroy,
		Steps: []resource.TestStep{
			{
				ConfigFile: testAccAICustomEvaluationMinimalConfigFile(
					t,
					createConfigFile,
					name,
				),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(aiCustomEvaluationResourceName, "id"),
					resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "name", name),
					resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "policy_type", "quality"),
					resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "description", ""),
					resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "instructions", "Score whether {response} matches the policy."),
					resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "should_include_system_prompt", "false"),
					resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "applications.#", "0"),
					resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "application_ids.#", "0"),
					resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "criteria.acceptable.flags", ""),
					resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "criteria.acceptable.examples.#", "0"),
					resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "criteria.prohibited.flags", ""),
					resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "criteria.prohibited.examples.#", "0"),
				),
			},
			{
				ResourceName:      aiCustomEvaluationResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceAICustomEvaluationUnlinkAllApplications(t *testing.T) {
	application := &aiEvaluationApplication{}
	name := acctest.RandomWithPrefix("tf-acc-ai-custom-evaluation")
	configDir := t.TempDir()
	createConfigFile := filepath.Join(configDir, "create.tf")
	updateConfigFile := filepath.Join(configDir, "update.tf")

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			*application = testAccFirstAICustomEvaluationApplication(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAICustomEvaluationDestroy,
		Steps: []resource.TestStep{
			{
				ConfigFile: testAccAICustomEvaluationConfigFile(
					t,
					createConfigFile,
					application,
					name,
					false,
					testAccAICustomEvaluationCreateCriteria(),
				),
				Check: testAccAICustomEvaluationCheck(
					application,
					name,
					"quality",
					false,
				),
			},
			{
				ConfigFile: testAccAICustomEvaluationWithoutApplicationLinksConfigFile(
					t,
					updateConfigFile,
					name,
					testAccAICustomEvaluationCreateCriteria(),
				),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(aiCustomEvaluationResourceName, "id"),
					resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "name", name),
					resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "applications.#", "0"),
					resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "application_ids.#", "0"),
				),
			},
		},
	})
}

func TestAccCoralogixResourceAICustomEvaluationMissingApplicationUpdate(t *testing.T) {
	application := &aiEvaluationApplication{}
	missingApplication := &aiEvaluationApplication{
		application: acctest.RandomWithPrefix("tf-acc-missing-ai-application"),
		subsystem:   acctest.RandomWithPrefix("tf-acc-missing-ai-subsystem"),
	}
	name := acctest.RandomWithPrefix("tf-acc-ai-custom-evaluation")
	configDir := t.TempDir()
	createConfigFile := filepath.Join(configDir, "create.tf")
	updateConfigFile := filepath.Join(configDir, "update.tf")

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			*application = testAccFirstAICustomEvaluationApplication(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAICustomEvaluationDestroy,
		Steps: []resource.TestStep{
			{
				ConfigFile: testAccAICustomEvaluationConfigFile(
					t,
					createConfigFile,
					application,
					name,
					false,
					testAccAICustomEvaluationCreateCriteria(),
				),
				Check: testAccAICustomEvaluationCheck(
					application,
					name,
					"quality",
					false,
				),
			},
			{
				ConfigFile: testAccAICustomEvaluationConfigFile(
					t,
					updateConfigFile,
					missingApplication,
					name,
					true,
					testAccAICustomEvaluationUpdateCriteria(),
				),
				ExpectError: regexp.MustCompile(`AI application not found|No AI application named`),
			},
		},
	})
}

func TestAccCoralogixResourceAICustomEvaluationInvalidInstructions(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-ai-custom-evaluation")
	configDir := t.TempDir()
	createConfigFile := filepath.Join(configDir, "create.tf")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAICustomEvaluationDestroy,
		Steps: []resource.TestStep{
			{
				ConfigFile: testAccAICustomEvaluationInvalidInstructionsConfigFile(
					t,
					createConfigFile,
					name,
				),
				ExpectError: regexp.MustCompile(`instructions must contain at least one of`),
			},
		},
	})
}

func TestAccCoralogixResourceAICustomEvaluationInvalidPolicyType(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-ai-custom-evaluation")
	configDir := t.TempDir()
	createConfigFile := filepath.Join(configDir, "create.tf")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAICustomEvaluationDestroy,
		Steps: []resource.TestStep{
			{
				ConfigFile: testAccAICustomEvaluationInvalidPolicyTypeConfigFile(
					t,
					createConfigFile,
					name,
				),
				ExpectError: regexp.MustCompile(`value must be one of`),
			},
		},
	})
}

func testAccAICustomEvaluationCheck(application *aiEvaluationApplication, name string, policyType string, shouldIncludeSystemPrompt bool, extraChecks ...resource.TestCheckFunc) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		checks := []resource.TestCheckFunc{
			resource.TestCheckResourceAttrSet(aiCustomEvaluationResourceName, "id"),
			resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "name", name),
			resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "policy_type", policyType),
			resource.TestCheckResourceAttr(aiCustomEvaluationResourceName, "should_include_system_prompt", fmt.Sprintf("%t", shouldIncludeSystemPrompt)),
			resource.TestCheckTypeSetElemNestedAttrs(aiCustomEvaluationResourceName, "applications.*", map[string]string{
				"application": application.application,
				"subsystem":   application.subsystem,
			}),
			resource.TestCheckTypeSetElemAttr(aiCustomEvaluationResourceName, "application_ids.*", application.id),
		}
		checks = append(checks, extraChecks...)

		return resource.ComposeAggregateTestCheckFunc(checks...)(s)
	}
}

func testAccFirstAICustomEvaluationApplication(t *testing.T) aiEvaluationApplication {
	t.Helper()

	aiEvaluationApplicationsOnce.Do(func() {
		aiEvaluationApplicationsCache, aiEvaluationApplicationsErr = testAccDiscoverAIApplications()
	})
	if aiEvaluationApplicationsErr != nil {
		t.Fatal(aiEvaluationApplicationsErr)
	}

	for _, application := range aiEvaluationApplicationsCache {
		if application.application == "" || application.id == "" {
			continue
		}
		return application
	}

	t.Fatal("no AI applications found")
	return aiEvaluationApplication{}
}

func testAccCheckAICustomEvaluationDestroy(s *terraform.State) error {
	client := testAccAIEvaluationsClient()
	ctx := context.Background()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_ai_custom_evaluation" {
			continue
		}

		resp, httpResp, err := client.
			AiEvaluationsServiceGetCustomEvaluations(ctx).
			Execute()
		if err != nil {
			apiErr := cxsdkOpenapi.NewAPIError(httpResp, err)
			if cxsdkOpenapi.Code(apiErr) == http.StatusNotFound {
				continue
			}
			return fmt.Errorf("failed to list AI custom evaluations: %s", utils.FormatOpenAPIErrors(apiErr, "List", nil))
		}

		for _, customEvaluation := range resp.GetItems() {
			if customEvaluation.GetId() == rs.Primary.ID {
				return fmt.Errorf("AI custom evaluation still exists: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}

func testAccAICustomEvaluationConfigFile(t *testing.T, filename string, application *aiEvaluationApplication, name string, shouldIncludeSystemPrompt bool, criteria string) config.TestStepConfigFunc {
	return func(config.TestStepConfigRequest) string {
		t.Helper()

		app := *application
		if app.application == "" {
			app.application = "placeholder-ai-application"
		}

		err := os.WriteFile(filename, []byte(testAccCoralogixResourceAICustomEvaluation(app, name, shouldIncludeSystemPrompt, criteria)), 0600)
		if err != nil {
			t.Fatalf("failed to write AI custom evaluation acceptance test config: %s", err)
		}

		return filename
	}
}

func testAccAICustomEvaluationMinimalConfigFile(t *testing.T, filename string, name string) config.TestStepConfigFunc {
	return func(config.TestStepConfigRequest) string {
		t.Helper()

		err := os.WriteFile(filename, []byte(testAccCoralogixResourceAICustomEvaluationMinimal(name)), 0600)
		if err != nil {
			t.Fatalf("failed to write AI custom evaluation acceptance test config: %s", err)
		}

		return filename
	}
}

func testAccAICustomEvaluationWithoutApplicationLinksConfigFile(t *testing.T, filename string, name string, criteria string) config.TestStepConfigFunc {
	return func(config.TestStepConfigRequest) string {
		t.Helper()

		err := os.WriteFile(filename, []byte(testAccCoralogixResourceAICustomEvaluationWithoutApplicationLinks(name, criteria)), 0600)
		if err != nil {
			t.Fatalf("failed to write AI custom evaluation acceptance test config: %s", err)
		}

		return filename
	}
}

func testAccAICustomEvaluationInvalidInstructionsConfigFile(t *testing.T, filename string, name string) config.TestStepConfigFunc {
	return func(config.TestStepConfigRequest) string {
		t.Helper()

		err := os.WriteFile(filename, []byte(testAccCoralogixResourceAICustomEvaluationInvalidInstructions(name)), 0600)
		if err != nil {
			t.Fatalf("failed to write AI custom evaluation acceptance test config: %s", err)
		}

		return filename
	}
}

func testAccAICustomEvaluationInvalidPolicyTypeConfigFile(t *testing.T, filename string, name string) config.TestStepConfigFunc {
	return func(config.TestStepConfigRequest) string {
		t.Helper()

		err := os.WriteFile(filename, []byte(testAccCoralogixResourceAICustomEvaluationInvalidPolicyType(name)), 0600)
		if err != nil {
			t.Fatalf("failed to write AI custom evaluation acceptance test config: %s", err)
		}

		return filename
	}
}

func testAccCoralogixResourceAICustomEvaluation(application aiEvaluationApplication, name string, shouldIncludeSystemPrompt bool, criteria string) string {
	policyType := "quality"
	description := "Flags competitor references in assistant responses."
	instructions := "Score whether {response} mentions competitor products.\nTreat each assistant answer independently."
	if shouldIncludeSystemPrompt {
		policyType = "security"
		description = "Flags responses that recommend competitor tools."
		instructions = "Score whether {response} recommends competitor products.\nOnly evaluate the final assistant response."
	}

	return fmt.Sprintf(`resource "coralogix_ai_custom_evaluation" "test" {
  name                         = %[1]q
  policy_type                  = %[2]q
  description                  = %[3]q
  instructions                 = %[4]q
  should_include_system_prompt = %[5]t
  applications = [{
    application = %[6]q
    subsystem   = %[7]q
  }]

  criteria = {
%[8]s
  }
}
`, name, policyType, description, instructions, shouldIncludeSystemPrompt, application.application, application.subsystem, strings.TrimSuffix(criteria, "\n"))
}

func testAccCoralogixResourceAICustomEvaluationMinimal(name string) string {
	return fmt.Sprintf(`resource "coralogix_ai_custom_evaluation" "test" {
  name         = %[1]q
  policy_type  = "quality"
  instructions = "Score whether {response} matches the policy."
}
`, name)
}

func testAccCoralogixResourceAICustomEvaluationWithoutApplicationLinks(name string, criteria string) string {
	return fmt.Sprintf(`resource "coralogix_ai_custom_evaluation" "test" {
  name                         = %[1]q
  policy_type                  = "quality"
  description                  = "Flags competitor references in assistant responses."
  instructions                 = "Score whether {response} mentions competitor products.\nTreat each assistant answer independently."
  should_include_system_prompt = false
  applications                 = []

  criteria = {
%[2]s
  }
}
`, name, strings.TrimSuffix(criteria, "\n"))
}

func testAccCoralogixResourceAICustomEvaluationInvalidInstructions(name string) string {
	return fmt.Sprintf(`resource "coralogix_ai_custom_evaluation" "test" {
  name         = %[1]q
  policy_type  = "quality"
  instructions = "Score whether the assistant response matches the policy."
}
`, name)
}

func testAccCoralogixResourceAICustomEvaluationInvalidPolicyType(name string) string {
	return fmt.Sprintf(`resource "coralogix_ai_custom_evaluation" "test" {
  name         = %[1]q
  policy_type  = "other"
  instructions = "Score whether {response} matches the policy."
}
`, name)
}

func testAccAICustomEvaluationCreateCriteria() string {
	return `    acceptable = {
      flags = "Does not mention competitor products.\nAnswer stays focused on our product."
      examples = [
        "User: which tool should I use?\nAssistant: Our product is a strong fit.",
      ]
    }
    prohibited = {
      flags = "Mentions a competitor product.\nNames another vendor as the recommended option."
      examples = [
        "User: which tool should I use?\nAssistant: CompetitorX is a strong fit.",
      ]
    }
`
}

func testAccAICustomEvaluationUpdateCriteria() string {
	return `    acceptable = {
      flags = "Does not recommend competitor products.\nMentions only our product or neutral guidance."
      examples = [
        "User: what should I buy?\nAssistant: Our product covers that workflow.",
      ]
    }
    prohibited = {
      flags = "Recommends a competitor product.\nNames a competitor as the best choice."
      examples = [
        "User: what should I buy?\nAssistant: You should buy CompetitorY.",
      ]
    }
`
}
