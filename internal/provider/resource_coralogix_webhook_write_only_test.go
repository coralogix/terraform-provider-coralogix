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
	"regexp"
	"testing"

	webhooks "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/outgoing_webhooks_service"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// A write-only secret must reach Coralogix and stay out of state. Checking only
// the state would pass just as well if the value were dropped on the way out,
// so every apply step is paired with a read of the backend.

func TestAccCoralogixResourceWebhookWriteOnlyJira(t *testing.T) {
	rn := "coralogix_webhook.test"
	name := acctest.RandomWithPrefix("tf-acc-test")
	url := fmt.Sprintf("https://xyz.atlassian.net/?q=%s", acctest.RandomWithPrefix("tf-acc-test"))
	token := acctest.RandomWithPrefix("tf-acc-secret")
	rotated := acctest.RandomWithPrefix("tf-acc-rotated")

	config := func(token string, version int, email string) string {
		return fmt.Sprintf(`
resource "coralogix_webhook" "test" {
  name = %q
  jira = {
    api_token_wo         = %q
    api_token_wo_version = %d
    email                = %q
    project_key          = "ABC"
    url                  = %q
  }
}
`, name, token, version, email, url)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckWebhookDestroyStandalone,
		Steps: []resource.TestStep{
			{
				// A version with no value cannot rotate anything.
				Config: fmt.Sprintf(`
resource "coralogix_webhook" "test" {
  name = %q
  jira = {
    api_token_wo_version = 1
    email                = "a@b.com"
    project_key          = "ABC"
    url                  = %q
  }
}
`, name, url),
				ExpectError: regexp.MustCompile(`(?s)api_token_wo`),
			},
			{
				Config: config(token, 1, "a@b.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rn, "id"),
					resource.TestCheckNoResourceAttr(rn, "jira.api_token_wo"),
					resource.TestCheckResourceAttr(rn, "jira.api_token_wo_version", "1"),
					// The API echoes the token back on read; state must not keep it.
					resource.TestCheckNoResourceAttr(rn, "jira.api_token"),
					webhookBackendJiraTokenEquals(rn, token),
				),
			},
			{
				// An unrelated change must not disturb the stored secret.
				Config: config(token, 1, "changed@b.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rn, "jira.email", "changed@b.com"),
					webhookBackendJiraTokenEquals(rn, token),
				),
			},
			{
				ResourceName:      rn,
				ImportState:       true,
				ImportStateVerify: true,
				// Import has neither configuration nor prior state to say the
				// token is write-only, so it arrives in state and the next apply
				// removes it. Pinned because it is the one place the guarantee
				// does not hold.
				ImportStateVerifyIgnore: []string{"jira"},
			},
			{
				// Terraform holds no copy of the old value, so only the version
				// bump can tell it to send a new one.
				Config: config(rotated, 2, "changed@b.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rn, "jira.api_token_wo_version", "2"),
					webhookBackendJiraTokenEquals(rn, rotated),
				),
			},
		},
	})
}

// An unknown nested block cannot be decoded into a pointer-backed struct, so
// validation has to read each block as an object first. Getting this wrong
// makes any variable-derived block unplannable, which a plan-only step catches.
func TestAccCoralogixResourceWebhookUnknownBlockIsPlannable(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-test")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terraform_data" "credentials" {
  input = {
    api_token   = "token"
    email       = "a@b.com"
    project_key = "ABC"
    url         = "https://example.atlassian.net"
  }
}

resource "coralogix_webhook" "test" {
  name = %q
  jira = terraform_data.credentials.output
}
`, name),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccCoralogixResourceWebhookWriteOnlyPagerDuty(t *testing.T) {
	rn := "coralogix_webhook.test"
	name := acctest.RandomWithPrefix("tf-acc-test")
	key := acctest.RandomWithPrefix("tf-acc-secret")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckWebhookDestroyStandalone,
		Steps: []resource.TestStep{
			{
				// The plain and the write-only attribute are alternatives.
				Config: fmt.Sprintf(`
resource "coralogix_webhook" "test" {
  name = %q
  pager_duty = {
    service_key            = "plain"
    service_key_wo         = "secret"
    service_key_wo_version = 1
  }
}
`, name),
				ExpectError: regexp.MustCompile(`(?s)service_key`),
			},
			{
				Config: fmt.Sprintf(`
resource "coralogix_webhook" "test" {
  name = %q
  pager_duty = {
    service_key_wo         = %q
    service_key_wo_version = 1
  }
}
`, name, key),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(rn, "pager_duty.service_key"),
					resource.TestCheckResourceAttr(rn, "pager_duty.service_key_wo_version", "1"),
					webhookBackendPagerDutyKeyEquals(rn, key),
				),
			},
		},
	})
}

func TestAccCoralogixResourceWebhookWriteOnlyCustomHeaders(t *testing.T) {
	rn := "coralogix_webhook.test"
	name := acctest.RandomWithPrefix("tf-acc-test")
	url := fmt.Sprintf("https://api.staging.coralogix.net/mgmt/testing/tools/httpbin/post/?q=%s", acctest.RandomWithPrefix("tf-acc-test"))
	secret := acctest.RandomWithPrefix("tf-acc-secret")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckWebhookDestroyStandalone,
		Steps: []resource.TestStep{
			{
				// Which of the two values would win is not left to merge order.
				Config: fmt.Sprintf(`
resource "coralogix_webhook" "test" {
  name = %q
  custom = {
    url                 = %q
    method              = "post"
    headers             = { "Authorization" = "plain" }
    headers_wo          = { "Authorization" = "secret" }
    headers_wo_versions = { "Authorization" = 1 }
  }
}
`, name, url),
				ExpectError: regexp.MustCompile(`(?s)collides with another header name`),
			},
			{
				// The per-key check is the only guard here: requiring the
				// versions map to be present would reject an empty headers_wo,
				// which a module passing a variable hits on its default.
				Config: fmt.Sprintf(`
resource "coralogix_webhook" "test" {
  name = %q
  custom = {
    url        = %q
    method     = "post"
    headers_wo = { "Authorization" = "secret" }
  }
}
`, name, url),
				ExpectError: regexp.MustCompile(`(?s)Missing write-only header version`),
			},
			{
				// Elements() cannot tell an unknown map from an empty one, so
				// comparing key sets while either side is unknown invents a
				// mismatch. Both directions have to stay plannable.
				Config: fmt.Sprintf(`
resource "terraform_data" "versions" {
  input = { "Authorization" = 1 }
}

resource "coralogix_webhook" "test" {
  name = %q
  custom = {
    url                 = %q
    method              = "post"
    headers_wo          = { "Authorization" = "secret" }
    headers_wo_versions = terraform_data.versions.output
  }
}
`, name, url),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: fmt.Sprintf(`
resource "terraform_data" "secrets" {
  input = { "Authorization" = "secret" }
}

resource "coralogix_webhook" "test" {
  name = %q
  custom = {
    url                 = %q
    method              = "post"
    headers_wo          = terraform_data.secrets.output
    headers_wo_versions = { "Authorization" = 1 }
  }
}
`, name, url),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				// A null version passes a presence check while recording
				// nothing, so a later change to the secret alone would never
				// be sent.
				Config: fmt.Sprintf(`
variable "versions" {
  type    = map(number)
  default = { Authorization = null }
}

resource "coralogix_webhook" "test" {
  name = %q
  custom = {
    url                 = %q
    method              = "post"
    headers_wo          = { "Authorization" = "secret" }
    headers_wo_versions = var.versions
  }
}
`, name, url),
				ExpectError: regexp.MustCompile(`(?s)Null write-only header version`),
			},
			{
				// An empty write-only map needs no versions map.
				Config: fmt.Sprintf(`
resource "coralogix_webhook" "test" {
  name = %q
  custom = {
    url        = %q
    method     = "post"
    headers    = { "Content-Type" = "application/json" }
    headers_wo = {}
  }
}
`, name, url),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rn, "custom.headers.Content-Type", "application/json"),
				),
			},
			{
				// HTTP header names are case-insensitive and the API stores both
				// spellings, so the recipient would decide which value applies.
				Config: fmt.Sprintf(`
resource "coralogix_webhook" "test" {
  name = %q
  custom = {
    url                 = %q
    method              = "post"
    headers             = { "authorization" = "plain" }
    headers_wo          = { "Authorization" = "secret" }
    headers_wo_versions = { "Authorization" = 1 }
  }
}
`, name, url),
				ExpectError: regexp.MustCompile(`(?s)case-insensitive`),
			},
			{
				// Two spellings within headers_wo are the same collision.
				Config: fmt.Sprintf(`
resource "coralogix_webhook" "test" {
  name = %q
  custom = {
    url                 = %q
    method              = "post"
    headers_wo          = { "Authorization" = "a", "authorization" = "b" }
    headers_wo_versions = { "Authorization" = 1, "authorization" = 1 }
  }
}
`, name, url),
				ExpectError: regexp.MustCompile(`(?s)collides with another header name`),
			},
			{
				Config: fmt.Sprintf(`
resource "coralogix_webhook" "test" {
  name = %q
  custom = {
    url                 = %q
    method              = "post"
    headers             = { "Content-Type" = "application/json" }
    headers_wo          = { "Authorization" = %q }
    headers_wo_versions = { "Authorization" = 1 }
  }
}
`, name, url, secret),
				Check: resource.ComposeAggregateTestCheckFunc(
					// The non-secret header survives; the secret one is removed
					// from what the API echoed back.
					resource.TestCheckResourceAttr(rn, "custom.headers.Content-Type", "application/json"),
					resource.TestCheckNoResourceAttr(rn, "custom.headers.Authorization"),
					resource.TestCheckResourceAttr(rn, "custom.headers_wo_versions.Authorization", "1"),
					webhookBackendHeaderEquals(rn, "Authorization", secret),
					webhookBackendHeaderEquals(rn, "Content-Type", "application/json"),
				),
			},
		},
	})
}

// testAccCheckWebhookDestroy reads the client through testAccProvider.Meta(),
// which is only populated once an SDKv2-backed test has configured it. These
// tests must pass when run on their own.
func testAccCheckWebhookDestroyStandalone(s *terraform.State) error {
	clients, err := testAccNewClientSet()
	if err != nil {
		return err
	}
	ctx := context.TODO()
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_webhook" {
			continue
		}
		if _, _, err := clients.Webhooks().
			OutgoingWebhooksServiceGetOutgoingWebhook(ctx, rs.Primary.ID).Execute(); err == nil {
			return fmt.Errorf("webhook still exists: %s", rs.Primary.ID)
		}
	}
	return nil
}

func webhookFromState(s *terraform.State, resourceName string) (*webhooks.OutgoingWebhook, error) {
	rs, ok := s.RootModule().Resources[resourceName]
	if !ok {
		return nil, fmt.Errorf("resource %s not found in state", resourceName)
	}
	clients, err := testAccNewClientSet()
	if err != nil {
		return nil, err
	}
	result, _, err := clients.Webhooks().
		OutgoingWebhooksServiceGetOutgoingWebhook(context.TODO(), rs.Primary.ID).Execute()
	if err != nil {
		return nil, fmt.Errorf("read webhook %s: %w", rs.Primary.ID, err)
	}
	return result.Webhook, nil
}

func webhookBackendJiraTokenEquals(resourceName, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		webhook, err := webhookFromState(s, resourceName)
		if err != nil {
			return err
		}
		jira := webhook.Jira
		if jira == nil {
			return fmt.Errorf("webhook has no jira configuration")
		}
		if got := jira.GetApiToken(); got != want {
			return fmt.Errorf("backend jira.api_token = %q, want %q", got, want)
		}
		return nil
	}
}

func webhookBackendPagerDutyKeyEquals(resourceName, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		webhook, err := webhookFromState(s, resourceName)
		if err != nil {
			return err
		}
		pagerDuty := webhook.PagerDuty
		if pagerDuty == nil {
			return fmt.Errorf("webhook has no pager_duty configuration")
		}
		if got := pagerDuty.GetServiceKey(); got != want {
			return fmt.Errorf("backend pager_duty.service_key = %q, want %q", got, want)
		}
		return nil
	}
}

func webhookBackendHeaderEquals(resourceName, header, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		webhook, err := webhookFromState(s, resourceName)
		if err != nil {
			return err
		}
		generic := webhook.GenericWebhook
		if generic == nil {
			return fmt.Errorf("webhook has no custom configuration")
		}
		if got := generic.Headers[header]; got != want {
			return fmt.Errorf("backend custom.headers[%q] = %q, want %q", header, got, want)
		}
		return nil
	}
}
