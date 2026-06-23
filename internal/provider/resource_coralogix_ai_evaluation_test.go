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
	"strings"
	"sync"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	aiapplications "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/ai_applications_service"
	aievaluations "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/ai_evaluations_service"
	testingconfig "github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var (
	aiEvaluationResourceName = "coralogix_ai_evaluation.test"

	aiEvaluationApplicationsOnce  sync.Once
	aiEvaluationApplicationsCache []aiEvaluationApplication
	aiEvaluationApplicationsErr   error
)

type aiEvaluationApplication struct {
	application string
	subsystem   string
}

func TestAccCoralogixResourceAIEvaluation(t *testing.T) {
	testCases := []struct {
		name           string
		evaluationType aievaluations.EvaluationType
		createConfig   string
		updateConfig   string
		createChecks   []resource.TestCheckFunc
		updateChecks   []resource.TestCheckFunc
	}{
		{
			name:           "allowed_topics",
			evaluationType: aievaluations.EVALUATIONTYPE_ALLOWED_TOPICS,
			createConfig: `    allowed_topics = {
      topics = ["billing", "account settings"]
    }`,
			updateConfig: `    allowed_topics = {
      topics = ["observability", "incident response"]
    }`,
			createChecks: testAccAIEvaluationSetChecks("config.allowed_topics.topics.*", "billing", "account settings"),
			updateChecks: testAccAIEvaluationSetChecks("config.allowed_topics.topics.*", "observability", "incident response"),
		},
		{
			name:           "competition",
			evaluationType: aievaluations.EVALUATIONTYPE_COMPETITION,
			createConfig: `    competition = {
      competitors = ["CompetitorOne", "CompetitorTwo"]
    }`,
			updateConfig: `    competition = {
      competitors = ["CompetitorThree", "CompetitorFour"]
    }`,
			createChecks: testAccAIEvaluationSetChecks("config.competition.competitors.*", "CompetitorOne", "CompetitorTwo"),
			updateChecks: testAccAIEvaluationSetChecks("config.competition.competitors.*", "CompetitorThree", "CompetitorFour"),
		},
		{
			name:           "pii",
			evaluationType: aievaluations.EVALUATIONTYPE_PII,
			createConfig: `    pii = {
      categories = ["EMAIL_ADDRESS", "CREDIT_CARD"]
    }`,
			updateConfig: `    pii = {
      categories = ["PHONE_NUMBER", "US_SSN"]
    }`,
			createChecks: testAccAIEvaluationSetChecks("config.pii.categories.*", "EMAIL_ADDRESS", "CREDIT_CARD"),
			updateChecks: testAccAIEvaluationSetChecks("config.pii.categories.*", "PHONE_NUMBER", "US_SSN"),
		},
		{
			name:           "restricted_topics",
			evaluationType: aievaluations.EVALUATIONTYPE_RESTRICTED_TOPICS,
			createConfig: `    restricted_topics = {
      topics = ["competitor mentions", "medical advice"]
    }`,
			updateConfig: `    restricted_topics = {
      topics = ["pricing promises", "legal advice"]
    }`,
			createChecks: testAccAIEvaluationSetChecks("config.restricted_topics.topics.*", "competitor mentions", "medical advice"),
			updateChecks: testAccAIEvaluationSetChecks("config.restricted_topics.topics.*", "pricing promises", "legal advice"),
		},
		{
			name:           "toxicity",
			evaluationType: aievaluations.EVALUATIONTYPE_TOXICITY,
			createConfig:   `    toxicity = {}`,
			updateConfig:   `    toxicity = {}`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			application := &aiEvaluationApplication{}
			target := new(string)
			configDir := t.TempDir()
			createConfigFile := filepath.Join(configDir, "create.tf")
			updateConfigFile := filepath.Join(configDir, "update.tf")

			resource.Test(t, resource.TestCase{
				PreCheck: func() {
					testAccPreCheck(t)
					selectedApplication, selectedTarget := testAccFirstAIApplication(t, testCase.evaluationType)
					*application = selectedApplication
					*target = selectedTarget
				},
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				CheckDestroy:             testAccCheckAIEvaluationDestroy,
				Steps: []resource.TestStep{
					{
						ConfigFile: testAccAIEvaluationConfigFile(t, createConfigFile, application, target, true, testCase.createConfig),
						Check: testAccAIEvaluationCheck(
							application,
							target,
							true,
							testCase.createChecks...,
						),
					},
					{
						ResourceName:      aiEvaluationResourceName,
						ImportState:       true,
						ImportStateVerify: true,
					},
					{
						ConfigFile: testAccAIEvaluationConfigFile(t, updateConfigFile, application, target, false, testCase.updateConfig),
						Check: testAccAIEvaluationCheck(
							application,
							target,
							false,
							testCase.updateChecks...,
						),
					},
				},
			})
		})
	}
}

func testAccAIEvaluationSetChecks(path string, values ...string) []resource.TestCheckFunc {
	checks := make([]resource.TestCheckFunc, 0, len(values))
	for _, value := range values {
		checks = append(checks, resource.TestCheckTypeSetElemAttr(aiEvaluationResourceName, path, value))
	}
	return checks
}

func testAccAIEvaluationCheck(application *aiEvaluationApplication, target *string, isEnabled bool, extraChecks ...resource.TestCheckFunc) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		checks := []resource.TestCheckFunc{
			resource.TestCheckResourceAttrSet(aiEvaluationResourceName, "id"),
			resource.TestCheckResourceAttr(aiEvaluationResourceName, "application", application.application),
			resource.TestCheckResourceAttr(aiEvaluationResourceName, "target", testAccAIEvaluationSelectedTarget(target)),
			resource.TestCheckResourceAttr(aiEvaluationResourceName, "threshold", "0.8"),
			resource.TestCheckResourceAttr(aiEvaluationResourceName, "is_enabled", fmt.Sprintf("%t", isEnabled)),
		}
		if application.subsystem != "" {
			checks = append(checks, resource.TestCheckResourceAttr(aiEvaluationResourceName, "subsystem", application.subsystem))
		}
		checks = append(checks, extraChecks...)

		return resource.ComposeAggregateTestCheckFunc(checks...)(s)
	}
}

func testAccFirstAIApplication(t *testing.T, evaluationType aievaluations.EvaluationType) (aiEvaluationApplication, string) {
	t.Helper()

	aiEvaluationApplicationsOnce.Do(func() {
		aiEvaluationApplicationsCache, aiEvaluationApplicationsErr = testAccDiscoverAIApplications()
	})
	if aiEvaluationApplicationsErr != nil {
		t.Fatal(aiEvaluationApplicationsErr)
	}

	for _, application := range aiEvaluationApplicationsCache {
		target, available, err := testAccAIApplicationTargetForEvaluationType(
			application,
			evaluationType,
			[]string{"response", "conversation", "prompt"},
		)
		if err != nil {
			t.Fatal(err)
		}
		if available {
			return application, target
		}
	}

	t.Fatalf("no AI applications found without existing %s AI evaluation for any target", evaluationType)
	return aiEvaluationApplication{}, ""
}

func testAccDiscoverAIApplications() ([]aiEvaluationApplication, error) {
	resp, httpResp, err := testAccAIApplicationsClient().
		AiApplicationsServiceListAiApplications(context.Background()).
		PageSize(200).
		PageOffset(0).
		Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to list AI applications: %s", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResp, err), "List", nil))
	}

	applications := make([]aiEvaluationApplication, 0, len(resp.GetAiApplications()))
	for _, application := range resp.GetAiApplications() {
		name := application.GetApplication()
		if name == "" {
			continue
		}

		applications = append(applications, aiEvaluationApplication{
			application: name,
			subsystem:   application.GetSubsystem(),
		})
	}
	if len(applications) == 0 {
		return nil, fmt.Errorf("no AI applications found")
	}

	return applications, nil
}

func testAccAIApplicationTargetForEvaluationType(application aiEvaluationApplication, evaluationType aievaluations.EvaluationType, targetCandidates []string) (string, bool, error) {
	ctx := context.Background()
	client := testAccAIEvaluationsClient()

	request := client.
		AiEvaluationsServiceListAiEvaluations(ctx).
		Application(application.application).
		EvaluationType(evaluationType).
		PageSize(200).
		PageOffset(0)
	if application.subsystem != "" {
		request = request.Subsystem(application.subsystem)
	}

	resp, httpResp, err := request.Execute()
	if err != nil {
		return "", false, fmt.Errorf(
			"failed to list AI evaluations for application %q and type %q: %s",
			application.application,
			evaluationType,
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResp, err), "List", nil),
		)
	}

	usedTargets := make(map[string]struct{}, len(resp.GetAiEvaluations()))
	for _, evaluation := range resp.GetAiEvaluations() {
		target := strings.ToLower(string(evaluation.GetTarget()))
		if target == "" {
			return "", false, nil
		}
		usedTargets[target] = struct{}{}
	}

	for _, target := range targetCandidates {
		if _, ok := usedTargets[target]; !ok {
			return target, true, nil
		}
	}

	return "", false, nil
}

func testAccCheckAIEvaluationDestroy(s *terraform.State) error {
	client := testAccAIEvaluationsClient()
	ctx := context.Background()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_ai_evaluation" {
			continue
		}

		resp, httpResp, err := client.
			AiEvaluationsServiceGetAiEvaluation(ctx, rs.Primary.ID).
			Execute()
		if err != nil {
			apiErr := cxsdkOpenapi.NewAPIError(httpResp, err)
			if cxsdkOpenapi.Code(apiErr) == http.StatusNotFound {
				continue
			}
			return apiErr
		}

		evaluation := resp.GetAiEvaluation()
		if evaluation.GetId() == rs.Primary.ID {
			return fmt.Errorf("AI evaluation still exists: %s", rs.Primary.ID)
		}
	}

	return nil
}

func testAccAIApplicationsClient() *aiapplications.AIApplicationsServiceAPIService {
	return testAccAIClientSet().AIApplications()
}

func testAccAIEvaluationsClient() *aievaluations.AIEvaluationsServiceAPIService {
	return testAccAIClientSet().AIEvaluations()
}

func testAccAIClientSet() *clientset.ClientSet {
	apiKey := os.Getenv("CORALOGIX_API_KEY")
	domain := os.Getenv("CORALOGIX_DOMAIN")
	terraformEnvironmentAlias := strings.ToUpper(os.Getenv("CORALOGIX_ENV"))

	if domain != "" {
		targetURL := clientset.GrpcTargetFromDomain(domain)
		return clientset.NewClientSet(domain, apiKey, targetURL)
	}

	targetURL := terraformEnvironmentAliasToGrpcUrl[terraformEnvironmentAlias]
	sdkEnvironment := terraformEnvironmentAliasToSdkEnvironment[terraformEnvironmentAlias]
	return clientset.NewClientSet(sdkEnvironment, apiKey, targetURL)
}

func testAccCoralogixResourceAIEvaluation(application aiEvaluationApplication, target string, isEnabled bool, config string) string {
	subsystem := ""
	if application.subsystem != "" {
		subsystem = fmt.Sprintf("  subsystem   = %q\n", application.subsystem)
	}

	return fmt.Sprintf(`resource "coralogix_ai_evaluation" "test" {
  application = %[1]q
%[2]s  target      = %[3]q
  threshold   = 0.8
  is_enabled  = %[4]t

  config = {
%[5]s
  }
}
`, application.application, subsystem, target, isEnabled, config)
}

func testAccAIEvaluationConfigFile(t *testing.T, filename string, application *aiEvaluationApplication, target *string, isEnabled bool, config string) testingconfig.TestStepConfigFunc {
	return func(testingconfig.TestStepConfigRequest) string {
		t.Helper()

		app := *application
		if app.application == "" {
			app.application = "placeholder-ai-application"
		}

		err := os.WriteFile(filename, []byte(testAccCoralogixResourceAIEvaluation(app, testAccAIEvaluationSelectedTarget(target), isEnabled, config)), 0600)
		if err != nil {
			t.Fatalf("failed to write AI evaluation acceptance test config: %s", err)
		}

		return filename
	}
}

func testAccAIEvaluationSelectedTarget(selected *string) string {
	if selected != nil && *selected != "" {
		return *selected
	}

	return "response"
}
