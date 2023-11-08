package coralogix

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"terraform-provider-coralogix/coralogix/clientset"
	webhooks "terraform-provider-coralogix/coralogix/clientset/grpc/webhooks"
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
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckWebhookDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceSlackWebhook(webhook),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", webhook.name),
					resource.TestCheckResourceAttr(resourceName, "slack.url", webhook.url),
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
		method:            selectRandomlyFromSlice(webhooksValidMethods),
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckWebhookDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceCustomWebhook(webhook),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", webhook.name),
					resource.TestCheckResourceAttr(resourceName, "custom.url", webhook.url),
					resource.TestCheckResourceAttr(resourceName, "custom.method", webhook.method),
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
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckWebhookDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourcePagerdutyWebhook(webhook),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", webhook.name),
					resource.TestCheckResourceAttr(resourceName, "pager_duty.service_key", webhook.serviceKey),
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
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckWebhookDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceEmailGroupWebhook(webhook),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", webhook.name),
					resource.TestCheckResourceAttr(resourceName, "email_group.emails.0", webhook.emails[0]),
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
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckWebhookDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceJiraWebhook(webhook),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", webhook.name),
					resource.TestCheckResourceAttr(resourceName, "jira.url", webhook.url),
					resource.TestCheckResourceAttr(resourceName, "jira.api_token", webhook.apiToken),
					resource.TestCheckResourceAttr(resourceName, "jira.project_key", webhook.projectKey),
					resource.TestCheckResourceAttr(resourceName, "jira.email", webhook.email),
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
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckWebhookDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceMicrosoftTeamsWebhook(webhook),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", webhook.name),
					resource.TestCheckResourceAttr(resourceName, "microsoft_teams.url", webhook.url),
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
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckWebhookDestroy,
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
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckWebhookDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceOpsgenieWebhook(webhook),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", webhook.name),
					resource.TestCheckResourceAttr(resourceName, "opsgenie.url", webhook.url),
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
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckWebhookDestroy,
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

		resp, err := client.GetWebhook(ctx, &webhooks.GetOutgoingWebhookRequest{Id: wrapperspb.String(rs.Primary.ID)})
		if err == nil {
			if resp.GetWebhook().GetId().GetValue() == rs.Primary.ID {
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
  	name    = "%s"
	slack = {
        url  = "%s"
  	}
}
`,
		w.name, w.url)
}

func testAccCoralogixResourceCustomWebhook(w *customWebhookTestFields) string {
	return fmt.Sprintf(`resource "coralogix_webhook" "test" {
  	 name    = "%s"
     custom = {
    	url     = "%s"
    	method  = "%s"
    	headers = { "Content-Type" : "application/json" }
  		payload = jsonencode({ "custom" : "payload" })
  	}
}
`,
		w.name, w.url, w.method)
}

func testAccCoralogixResourcePagerdutyWebhook(w *pagerDutyWebhookTestFields) string {
	return fmt.Sprintf(`resource "coralogix_webhook" "test" {
  	name    = "%s"
	pager_duty = {
        service_key  = "%s"
  	}
}
`,
		w.name, w.serviceKey)
}

func testAccCoralogixResourceEmailGroupWebhook(w *emailGroupWebhookTestFields) string {
	return fmt.Sprintf(`resource "coralogix_webhook" "test" {
  	name = "%s"
	email_group = {
        emails  = %s
  	}
}
`,
		w.name, sliceToString(w.emails))
}

func testAccCoralogixResourceSendLogWebhook(w *webhookTestFields) string {
	return fmt.Sprintf(`resource "coralogix_webhook" test {
	name    = "%s"
	sendlog = {
    payload  = jsonencode({ "custom" : "payload" })
  	}
}
`,
		w.name)
}

func testAccCoralogixResourceMicrosoftTeamsWebhook(w *webhookTestFields) string {
	return fmt.Sprintf(`resource "coralogix_webhook" "test" {
  	name = "%s"
	microsoft_teams = {
        url  = "%s"
  	}
}
`,
		w.name, w.url)
}

func testAccCoralogixResourceJiraWebhook(w *jiraWebhookTestFields) string {
	return fmt.Sprintf(`resource "coralogix_webhook" "test" {
  	name        = "%s"
	jira = {
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
  	name = "%s"
	opsgenie = {
        url  = "%s"
  	}
}
`,
		w.name, w.url)
}

func testAccCoralogixResourceDemistoWebhook(w *webhookTestFields) string {
	return fmt.Sprintf(`resource "coralogix_webhook" "test" {
  	name = "%s"
	demisto = {
        payload = jsonencode({ "custom" : "payload" })
		url = "%s"
  	}
}
`,
		w.name, w.url)
}
