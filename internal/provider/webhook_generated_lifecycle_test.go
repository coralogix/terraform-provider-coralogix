package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Each type uses the generated resource and data source through the provider registry.
func TestAccCoralogixResourceWebhookGeneratedTypes(t *testing.T) {
	for _, kind := range []string{
		"custom", "slack", "pager_duty", "sendlog", "email_group",
		"microsoft_teams_workflow", "jira", "opsgenie", "demisto", "event_bridge",
	} {
		t.Run(kind, func(t *testing.T) {
			base := generatedWebhookBase(kind)
			jiraToken := acctest.RandomWithPrefix("tf-acc-test")
			pagerDutyKey := acctest.RandomWithPrefix("tf-acc-test")
			config := func(name string) string {
				return generatedWebhookConfig(kind, webhookTestFields{name: name, url: base.url}, jiraToken, pagerDutyKey)
			}

			address := "coralogix_webhook.test"
			name := base.name
			var id string
			captureID := resource.TestCheckResourceAttrWith(address, "id", func(value string) error {
				if value == "" {
					return fmt.Errorf("webhook has no id")
				}
				id = value
				return nil
			})
			steps := []resource.TestStep{
				{Config: config(name), Check: resource.ComposeAggregateTestCheckFunc(captureID, resource.TestCheckResourceAttr(address, "name", name))},
				{ResourceName: address, ImportState: true, ImportStateVerify: true},
				{Config: config(name) + testAccCoralogixDataSourceWebhook_readByID() + testAccCoralogixDataSourceWebhook_readByName(),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttrPair("data.coralogix_webhook.test_by_id", "id", address, "id"),
						resource.TestCheckResourceAttrPair("data.coralogix_webhook.test_by_name", "id", address, "id"),
					)},
				{Config: config(name + "-updated"), Check: resource.ComposeAggregateTestCheckFunc(captureID, resource.TestCheckResourceAttr(address, "name", name+"-updated"))},
				{ResourceName: address, ImportState: true, ImportStateVerify: true},
			}
			if kind == "custom" {
				steps = append(steps, resource.TestStep{
					PreConfig: func() {
						cs, err := testAccNewClientSet()
						if err != nil {
							t.Fatal(err)
						}
						_, _, err = cs.Webhooks().OutgoingWebhooksServiceDeleteOutgoingWebhook(context.Background(), id).Execute()
						if err != nil {
							t.Fatal(err)
						}
					},
					Config: config(name + "-updated"),
					Check: resource.TestCheckResourceAttrWith(address, "id", func(value string) error {
						if value == id {
							return fmt.Errorf("missing webhook was not recreated")
						}
						return nil
					}),
				})
			}
			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				CheckDestroy:             testAccCheckWebhookDestroy,
				Steps:                    steps,
			})
		})
	}
}

func generatedWebhookBase(kind string) *webhookTestFields {
	switch kind {
	case "slack":
		return getRandomWebhookWithCustomTld("slack.com")
	case "jira":
		return getRandomWebhookWithCustomUrl("xyz.atlassian.net")
	default:
		return getRandomWebhook()
	}
}

func generatedWebhookConfig(kind string, w webhookTestFields, jiraToken, pagerDutyKey string) string {
	switch kind {
	case "custom":
		return testAccCoralogixResourceCustomWebhook(&customWebhookTestFields{webhookTestFields: w, method: "post"})
	case "slack":
		return testAccCoralogixResourceSlackWebhook(&slackWebhookTestFields{webhookTestFields: w})
	case "pager_duty":
		return testAccCoralogixResourcePagerdutyWebhook(&pagerDutyWebhookTestFields{webhookTestFields: w, serviceKey: pagerDutyKey})
	case "sendlog":
		return testAccCoralogixResourceSendLogWebhook(&w)
	case "email_group":
		return testAccCoralogixResourceEmailGroupWebhook(&emailGroupWebhookTestFields{webhookTestFields: w, emails: []string{"example@coralogix.com"}})
	case "microsoft_teams_workflow":
		return testAccCoralogixResourceMicrosoftTeamsWorkflowWebhook(&w)
	case "jira":
		return testAccCoralogixResourceJiraWebhook(&jiraWebhookTestFields{webhookTestFields: w, apiToken: jiraToken, email: "example@coralogix.com", projectKey: "TEST"})
	case "opsgenie":
		return testAccCoralogixResourceOpsgenieWebhook(&w)
	case "demisto":
		return testAccCoralogixResourceDemistoWebhook(&w)
	case "event_bridge":
		return testAccCoralogixResourceEventBridgeWebhook(&eventBridgeWebhookTestFields{
			webhookTestFields: w, eventBusArn: "arn:aws:events:us-east-1:123456789012:event-bus/default",
			detail: "detail", detailType: "detailType", source: "source", roleName: "roleName",
		})
	default:
		return ""
	}
}
