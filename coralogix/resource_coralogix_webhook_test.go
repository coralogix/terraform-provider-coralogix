package coralogix

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"terraform-provider-coralogix/coralogix/clientset"
)

type webhookTestFields struct {
	name, url string
}

type customWebhookTestFields struct {
	webhookTestFields
	method string
}

type pagerDutyWebhookTestFields struct {
	webhookTestFields
	serviceKey string
}

type emailGroupWebhookTestFields struct {
	webhookTestFields
	emails []string
}

type jiraWebhookTestFields struct {
	webhookTestFields
	apiToken, email, projectKey string
}

func TestAccCoralogixResourceSlackWebhook(t *testing.T) {
	resourceName := "coralogix_webhook.test"
	webhook := getRandomWebhook()
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckWebhookDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceSlackWebhook(webhook),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", webhook.name),
					resource.TestCheckResourceAttr(resourceName, "slack.0.url", webhook.url),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceCustomWebhook(t *testing.T) {
	resourceName := "coralogix_webhook.test"
	webhook := &customWebhookTestFields{
		webhookTestFields: *getRandomWebhook(),
		method:            selectRandomlyFromSlice(validMethods),
	}
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckWebhookDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceCustomWebhook(webhook),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", webhook.name),
					resource.TestCheckResourceAttr(resourceName, "custom.0.url", webhook.url),
					resource.TestCheckResourceAttr(resourceName, "custom.0.method", webhook.method),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourcePagerDutyWebhook(t *testing.T) {
	resourceName := "coralogix_webhook.test"
	webhook := &pagerDutyWebhookTestFields{
		webhookTestFields: *getRandomWebhook(),
		serviceKey:        acctest.RandomWithPrefix("tf-acc-test"),
	}
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckWebhookDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourcePagerdutyWebhook(webhook),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", webhook.name),
					resource.TestCheckResourceAttr(resourceName, "pager_duty.0.service_key", webhook.serviceKey),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceEmailGroupWebhook(t *testing.T) {
	resourceName := "coralogix_webhook.test"
	webhook := &emailGroupWebhookTestFields{
		webhookTestFields: *getRandomWebhook(),
		emails:            []string{"example@coralogix.com"},
	}
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckWebhookDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceEmailGroupWebhook(webhook),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", webhook.name),
					resource.TestCheckResourceAttr(resourceName, "email_group.0.emails.0", webhook.emails[0]),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceJiraWebhook(t *testing.T) {
	resourceName := "coralogix_webhook.test"
	webhook := &jiraWebhookTestFields{
		webhookTestFields: *getRandomWebhook(),
		apiToken:          acctest.RandomWithPrefix("tf-acc-test"),
		email:             "example@coralgox.com",
		projectKey:        acctest.RandomWithPrefix("tf-acc-test"),
	}
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckWebhookDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceJiraWebhook(webhook),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", webhook.name),
					resource.TestCheckResourceAttr(resourceName, "jira.0.url", webhook.url),
					resource.TestCheckResourceAttr(resourceName, "jira.0.api_token", webhook.apiToken),
					resource.TestCheckResourceAttr(resourceName, "jira.0.project_key", webhook.projectKey),
					resource.TestCheckResourceAttr(resourceName, "jira.0.email", webhook.email),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceMicrosoftTeamsWebhook(t *testing.T) {
	resourceName := "coralogix_webhook.test"
	webhook := getRandomWebhook()
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckWebhookDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceMicrosoftTeamsWebhook(webhook),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", webhook.name),
					resource.TestCheckResourceAttr(resourceName, "microsoft_teams.0.url", webhook.url),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceSendLogWebhook(t *testing.T) {
	resourceName := "coralogix_webhook.test"
	webhook := getRandomWebhook()
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckWebhookDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceSendLogWebhook(webhook),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", webhook.name),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceOpsgenieTeamsWebhook(t *testing.T) {
	resourceName := "coralogix_webhook.test"
	webhook := getRandomWebhook()
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckWebhookDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceOpsgenieWebhook(webhook),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", webhook.name),
					resource.TestCheckResourceAttr(resourceName, "opsgenie.0.url", webhook.url),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceDemistoTeamsWebhook(t *testing.T) {
	resourceName := "coralogix_webhook.test"
	webhook := getRandomWebhook()
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckWebhookDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceDemistoWebhook(webhook),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", webhook.name),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckWebhookDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).Webhooks()

	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_webhook" {
			continue
		}

		resp, err := client.GetWebhook(ctx, rs.Primary.ID)
		if err == nil {
			var m map[string]interface{}
			if err = json.Unmarshal([]byte(resp), &m); err != nil {
				return nil
			}
			id := strconv.Itoa(int(m["id"].(float64)))
			if id == rs.Primary.ID {
				return fmt.Errorf("webhook still exists: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}

func getRandomWebhook() *webhookTestFields {
	return &webhookTestFields{
		name: acctest.RandomWithPrefix("tf-acc-test"),
		url:  fmt.Sprintf("https://%s/", acctest.RandomWithPrefix("tf-acc-test")),
	}

}

func testAccCoralogixResourceSlackWebhook(w *webhookTestFields) string {
	return fmt.Sprintf(`resource "coralogix_webhook" "test" {
  	slack {
    	name = "%s"
        url  = "%s"
  	}
}
`,
		w.name, w.url)
}

func testAccCoralogixResourceCustomWebhook(w *customWebhookTestFields) string {
	return fmt.Sprintf(`resource "coralogix_webhook" "test" {
  	 custom {
    	name    = "%s"
    	url     = "%s"
    	method  = "%s"
    	headers = jsonencode({ "custom" : "header" })
  		payload = jsonencode({ "custom" : "payload" })
  	}
}
`,
		w.name, w.url, w.method)
}

func testAccCoralogixResourcePagerdutyWebhook(w *pagerDutyWebhookTestFields) string {
	return fmt.Sprintf(`resource "coralogix_webhook" "test" {
  	pager_duty {
    	name = "%s"
        service_key  = "%s"
  	}
}
`,
		w.name, w.serviceKey)
}

func testAccCoralogixResourceEmailGroupWebhook(w *emailGroupWebhookTestFields) string {
	return fmt.Sprintf(`resource "coralogix_webhook" "test" {
  	email_group {
    	name = "%s"
        emails  = %s
  	}
}
`,
		w.name, sliceToString(w.emails))
}

func testAccCoralogixResourceSendLogWebhook(w *webhookTestFields) string {
	return fmt.Sprintf(`resource "coralogix_webhook" test {
sendlog {
    name    = "%s"
    payload  = jsonencode({ "custom" : "payload" })
  	}
}
`,
		w.name)
}

func testAccCoralogixResourceMicrosoftTeamsWebhook(w *webhookTestFields) string {
	return fmt.Sprintf(`resource "coralogix_webhook" "test" {
  microsoft_teams {
    	name = "%s"
        url  = "%s"
  	}
}
`,
		w.name, w.url)
}

func testAccCoralogixResourceJiraWebhook(w *jiraWebhookTestFields) string {
	return fmt.Sprintf(`resource "coralogix_webhook" "test" {
  	jira {
    name        = "%s"
    url         = "%s"
    api_token   = "%s"
    email       = "%s"
    project_key = "%s"
  }
}
`,
		w.name, w.url, w.apiToken, w.email, w.projectKey)
}

func testAccCoralogixResourceOpsgenieWebhook(w *webhookTestFields) string {
	return fmt.Sprintf(`resource "coralogix_webhook" "test" {
  	opsgenie {
    	name = "%s"
        url  = "%s"
  	}
}
`,
		w.name, w.url)
}

func testAccCoralogixResourceDemistoWebhook(w *webhookTestFields) string {
	return fmt.Sprintf(`resource "coralogix_webhook" "test" {
  	demisto {
    	name = "%s"
        payload = jsonencode({ "custom" : "payload" })
  	}
}
`,
		w.name)
}
