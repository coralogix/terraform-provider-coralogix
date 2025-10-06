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
	"context"
	"fmt"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type webhookTestFields struct {
	name, url string
}

type customWebhookTestFields struct {
	webhookTestFields
	method string
}

type slackWebhookTestFields struct {
	webhookTestFields
	notifyAbout []string
	attachments []attachmentTestFields
}

type attachmentTestFields struct {
	attachmentType string
	active         bool
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

type eventBridgeWebhookTestFields struct {
	webhookTestFields
	eventBusArn, detail, detailType, source, roleName string
}

func TestAccCoralogixResourceSlackWebhook(t *testing.T) {
	resourceName := "coralogix_webhook.test"
	webhook := &slackWebhookTestFields{
		webhookTestFields: *getRandomWebhook(),
		notifyAbout:       []string{"flow_anomalies"},
		attachments: []attachmentTestFields{
			{
				attachmentType: "metric_snapshot",
				active:         true,
			},
		},
	}
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
					resource.TestCheckResourceAttr(resourceName, "slack.notify_on.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "slack.attachments.0.type", webhook.attachments[0].attachmentType),
					resource.TestCheckResourceAttr(resourceName, "slack.attachments.0.active", fmt.Sprintf("%t", webhook.attachments[0].active)),
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
		method:            "post",
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

func TestAccCoralogixResourceMicrosoftTeamsWorkflowWebhook(t *testing.T) {
	resourceName := "coralogix_webhook.test"
	webhook := getRandomWebhook()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckWebhookDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceMicrosoftTeamsWorkflowWebhook(webhook),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", webhook.name),
					resource.TestCheckResourceAttr(resourceName, "microsoft_teams_workflow.url", webhook.url),
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

func TestAccCoralogixResourceOpsgenieWebhook(t *testing.T) {
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

func TestAccCoralogixResourceDemistoWebhook(t *testing.T) {
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

func TestAccCoralogixResourceEventBridgeWebhook(t *testing.T) {
	resourceName := "coralogix_webhook.test"
	webhook := getRandomWebhook()
	eventBridgeWebhook := &eventBridgeWebhookTestFields{
		webhookTestFields: *webhook,
		eventBusArn:       "arn:aws:events:us-east-1:123456789012:event-bus/default",
		detail:            "detail",
		detailType:        "detailType",
		source:            "source",
		roleName:          "roleName",
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckWebhookDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceEventBridgeWebhook(eventBridgeWebhook),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", eventBridgeWebhook.name),
					resource.TestCheckResourceAttr(resourceName, "event_bridge.event_bus_arn", eventBridgeWebhook.eventBusArn),
					resource.TestCheckResourceAttr(resourceName, "event_bridge.detail", eventBridgeWebhook.detail),
					resource.TestCheckResourceAttr(resourceName, "event_bridge.detail_type", eventBridgeWebhook.detailType),
					resource.TestCheckResourceAttr(resourceName, "event_bridge.source", eventBridgeWebhook.source),
					resource.TestCheckResourceAttr(resourceName, "event_bridge.role_name", eventBridgeWebhook.roleName),
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

		resp, err := client.Get(ctx, &cxsdk.GetOutgoingWebhookRequest{Id: wrapperspb.String(rs.Primary.ID)})
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

func testAccCoralogixResourceSlackWebhook(w *slackWebhookTestFields) string {
	return fmt.Sprintf(`resource "coralogix_webhook" "test" {
name    = "%s"
slack = {
	url  = "%s"
	notify_on = ["flow_anomalies"]
	attachments  = [{
		type  = "metric_snapshot"
		active = true
	}]
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
name       = "%s"
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
		w.name, utils.SliceToString(w.emails))
}

func testAccCoralogixResourceSendLogWebhook(w *webhookTestFields) string {
	return fmt.Sprintf(`resource "coralogix_webhook" test {
name    = "%s"
sendlog = {
	payload  = jsonencode({ "custom" : "payload" })
	url      = "%s"
}
}
`,
		w.name, w.url)
}

func testAccCoralogixResourceMicrosoftTeamsWorkflowWebhook(w *webhookTestFields) string {
	return fmt.Sprintf(`resource "coralogix_webhook" "test" {
name = "%s"
microsoft_teams_workflow = {
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

func testAccCoralogixResourceEventBridgeWebhook(w *eventBridgeWebhookTestFields) string {
	return fmt.Sprintf(`resource "coralogix_webhook" "test" {
  	name = "%s"
	event_bridge = {
		event_bus_arn = "%s"
		detail = "%s"
		detail_type = "%s"
		source = "%s"
		role_name = "%s"
  	}
}
`,
		w.name, w.eventBusArn, w.detail, w.detailType, w.source, w.roleName)
}
