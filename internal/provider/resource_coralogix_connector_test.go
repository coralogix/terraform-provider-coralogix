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
	"os"
	"regexp"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var (
	connectorResourceName    = "coralogix_connector.example"
	msTeamsIntegrationId     = os.Getenv("MS_TEAMS_INTEGRATION_ID")
	msTeamsTeamId            = os.Getenv("MS_TEAMS_TEAM_ID")
	msTeamsChannelId         = os.Getenv("MS_TEAMS_CHANNEL_ID")
	eventbridgeIntegrationId = os.Getenv("EVENTBRIDGE_INTEGRATION_ID")
	incidentIOAPIKey         = os.Getenv("INCIDENT_IO_API_KEY")
	incidentIOAlertEventsURL = os.Getenv("INCIDENT_IO_ALERT_EVENTS_URL")
	incidentIOAlertSourceTok = os.Getenv("INCIDENT_IO_ALERT_SOURCE_TOKEN")
)

func TestAccCoralogixResourceGenericHttpsConnector(t *testing.T) {
	name := uuid.NewString()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCoralogixGenericHttpsConnector(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(connectorResourceName, "id", name),
					resource.TestCheckResourceAttr(connectorResourceName, "type", "generic_https"),
					resource.TestCheckResourceAttr(connectorResourceName, "name", name),
					resource.TestCheckResourceAttr(connectorResourceName, "description", "generic https connector"),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "method",
						"value":      "post",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "url",
						"value":      "https://api.staging.coralogix.net/mgmt/testing/tools/httpbin/post",
					}),
				),
			},
			{
				ResourceName:      connectorResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccResourceCoralogixGenericHttpsConnectorUpdate(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(connectorResourceName, "id", name),
					resource.TestCheckResourceAttr(connectorResourceName, "type", "generic_https"),
					resource.TestCheckResourceAttr(connectorResourceName, "name", fmt.Sprintf("%v-updated", name)),
					resource.TestCheckResourceAttr(connectorResourceName, "description", "generic https connector"),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "method",
						"value":      "post",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "url",
						"value":      "https://api.staging.coralogix.net/mgmt/testing/tools/httpbin/post",
					}),
				),
			},
		},
	})
}

func TestAccCoralogixResourceSlackConnector(t *testing.T) {
	name := uuid.NewString()
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccRequiredEnvVarsPreCheck(
				t,
				"SLACK_INTEGRATION_ID",
				"SLACK_INTEGRATION_CHANNEL",
				"SLACK_INTEGRATION_CHANNEL_UPDATED",
			)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCoralogixSlackConnector(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(connectorResourceName, "id", name),
					resource.TestCheckResourceAttr(connectorResourceName, "type", "slack"),
					resource.TestCheckResourceAttr(connectorResourceName, "name", name),
					resource.TestCheckResourceAttr(connectorResourceName, "description", "test connector"),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "integrationId",
						"value":      slackIntegrationId,
					}),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "channel",
						"value":      slackIntegrationChannel,
					}),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "fallbackChannel",
						"value":      slackIntegrationChannel,
					}),
				),
			},
			{
				ResourceName:      connectorResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccResourceCoralogixSlackConnectorUpdate(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(connectorResourceName, "id", name),
					resource.TestCheckResourceAttr(connectorResourceName, "type", "slack"),
					resource.TestCheckResourceAttr(connectorResourceName, "name", name),
					resource.TestCheckResourceAttr(connectorResourceName, "description", "test connector"),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "integrationId",
						"value":      slackIntegrationId,
					}),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "channel",
						"value":      slackIntegrationChannelUpdated,
					}),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "fallbackChannel",
						"value":      slackIntegrationChannelUpdated,
					}),
				),
			},
		},
	})
}

func TestAccCoralogixResourcePagerdutyConnector(t *testing.T) {
	name := uuid.NewString()
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccRequiredEnvVarsPreCheck(
				t,
				"PD_INTEGRATION_ID",
			)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCoralogixPagerdutyConnector(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(connectorResourceName, "id", name),
					resource.TestCheckResourceAttr(connectorResourceName, "type", "pagerduty"),
					resource.TestCheckResourceAttr(connectorResourceName, "name", name),
					resource.TestCheckResourceAttr(connectorResourceName, "description", "test pagerduty connector"),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "integrationKey",
						"value":      pagerDutyIntegrationId,
					}),
				),
			},
			{
				ResourceName:      connectorResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccResourceCoralogixPagerdutyConnectorUpdate(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(connectorResourceName, "id", name),
					resource.TestCheckResourceAttr(connectorResourceName, "type", "pagerduty"),
					resource.TestCheckResourceAttr(connectorResourceName, "name", name),
					resource.TestCheckResourceAttr(connectorResourceName, "description", "test pagerduty connector updated"),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "integrationKey",
						"value":      pagerDutyIntegrationId,
					}),
				),
			},
		},
	})
}

func TestAccCoralogixResourcePagerdutyIncidentsConnector(t *testing.T) {
	name := uuid.NewString()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccRequiredEnvVarsPreCheck(t, "PD_INTEGRATION_ID") },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCoralogixPagerdutyIncidentsConnector(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(connectorResourceName, "id", name),
					resource.TestCheckResourceAttr(connectorResourceName, "type", "pagerduty_incidents"),
					resource.TestCheckResourceAttr(connectorResourceName, "name", name),
					resource.TestCheckResourceAttr(connectorResourceName, "description", "test pagerduty incidents connector"),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "integrationId",
						"value":      pagerDutyIntegrationId,
					}),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "service",
						"value":      "PXXXXXX",
					}),
				),
			},
			{
				ResourceName:      connectorResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceMicrosoftTeamsConnector(t *testing.T) {
	name := uuid.NewString()
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			for _, envVar := range []string{
				"MS_TEAMS_INTEGRATION_ID",
				"MS_TEAMS_TEAM_ID",
				"MS_TEAMS_CHANNEL_ID",
			} {
				if os.Getenv(envVar) == "" {
					t.Skipf("%s must be set to run this acceptance test locally", envVar)
				}
			}
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCoralogixMicrosoftTeamsConnector(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(connectorResourceName, "id", name),
					resource.TestCheckResourceAttr(connectorResourceName, "type", "microsoft_teams"),
					resource.TestCheckResourceAttr(connectorResourceName, "name", name),
					resource.TestCheckResourceAttr(connectorResourceName, "description", "test microsoft teams connector"),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "integrationId",
						"value":      msTeamsIntegrationId,
					}),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "teamId",
						"value":      msTeamsTeamId,
					}),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "channelId",
						"value":      msTeamsChannelId,
					}),
				),
			},
			{
				ResourceName:      connectorResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceEventbridgeConnector(t *testing.T) {
	name := uuid.NewString()
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			if os.Getenv("EVENTBRIDGE_INTEGRATION_ID") == "" {
				t.Skipf("EVENTBRIDGE_INTEGRATION_ID must be set to run this acceptance test locally")
			}
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCoralogixEventbridgeConnector(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(connectorResourceName, "id", name),
					resource.TestCheckResourceAttr(connectorResourceName, "type", "eventbridge"),
					resource.TestCheckResourceAttr(connectorResourceName, "name", name),
					resource.TestCheckResourceAttr(connectorResourceName, "description", "test eventbridge connector"),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "integrationId",
						"value":      eventbridgeIntegrationId,
					}),
				),
			},
			{
				ResourceName:      connectorResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceEmailConnector(t *testing.T) {
	name := uuid.NewString()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCoralogixEmailConnector(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(connectorResourceName, "id", name),
					resource.TestCheckResourceAttr(connectorResourceName, "type", "email"),
					resource.TestCheckResourceAttr(connectorResourceName, "name", name),
					resource.TestCheckResourceAttr(connectorResourceName, "description", "email connector example"),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "emailAddresses",
						"value":      `["email1@example.com","email2@example.com"]`,
					}),
				),
			},
			{
				ResourceName:      connectorResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccResourceCoralogixEmailConnectorUpdate(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(connectorResourceName, "id", name),
					resource.TestCheckResourceAttr(connectorResourceName, "type", "email"),
					resource.TestCheckResourceAttr(connectorResourceName, "name", fmt.Sprintf("%s-updated", name)),
					resource.TestCheckResourceAttr(connectorResourceName, "description", "email connector example updated"),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "emailAddresses",
						"value":      `["email1@example.com","email2@example.com","email3@example.com"]`,
					}),
				),
			},
		},
	})
}

func testAccResourceCoralogixGenericHttpsConnector(name string) string {
	return fmt.Sprintf(`resource "coralogix_connector" "example" {
   id               = "%[1]v"
   name             = "%[1]v"
   type             = "generic_https"
   description      = "generic https connector"
   connector_config = {
     fields = [
	  {
	    field_name = "url"
	 	value      = "https://api.staging.coralogix.net/mgmt/testing/tools/httpbin/post"
	  },
	  {
	 	field_name = "method"
	 	value      = "post"
	  }
     ]
   }
 }
`, name)
}

func testAccResourceCoralogixGenericHttpsConnectorUpdate(name string) string {
	return fmt.Sprintf(`resource "coralogix_connector" "example" {
   id               = "%[1]v"
   name             = "%[1]v-updated"
   type             = "generic_https"
   description      = "generic https connector"
   connector_config = {
     fields = [
	  {
	    field_name = "url"
	 	value      = "https://api.staging.coralogix.net/mgmt/testing/tools/httpbin/post"
	  },
	  {
	 	field_name = "method"
	 	value      = "post"
	  }
     ]
   }
}
`, name)
}

func testAccResourceCoralogixSlackConnector(name string) string {
	return fmt.Sprintf(`resource "coralogix_connector" "example" {
   id               = "%[1]v"
   name             = "%[1]v"
   type             = "slack"
   description      = "test connector"
   connector_config = {
     fields = [
       {
         field_name = "integrationId"
         value      = "%[2]v"
       },
	   {
	   	  field_name = "channel"
		  value      = "%[3]v"
	   },
	   {
	   	  field_name = "fallbackChannel"
		  value      = "%[3]v"
	   },
     ]
   }
 }`, name, slackIntegrationId, slackIntegrationChannel)
}

func testAccResourceCoralogixSlackConnectorUpdate(name string) string {
	return fmt.Sprintf(`resource "coralogix_connector" "example" {
   id               = "%[1]v"
   name             = "%[1]v"
   type             = "slack"
   description      = "test connector"
   connector_config = {
     fields = [
       {
         field_name = "integrationId"
         value      = "%[2]v"
       },
	   {
	   	  field_name = "channel"
		  value      = "%[3]v"
	   },
	   {
	   	  field_name = "fallbackChannel"
		  value      = "%[3]v"
	   },
     ]
   }
 }`, name, slackIntegrationId, slackIntegrationChannelUpdated)
}

func testAccResourceCoralogixPagerdutyConnector(name string) string {
	return fmt.Sprintf(`resource "coralogix_connector" "example" {
   id               = "%[1]v"
   type             = "pagerduty"
   name             = "%[1]v"
   description      = "test pagerduty connector"
   connector_config = {
     fields = [
       {
         field_name = "integrationKey"
         value      = "%[2]v"
       }
     ]
   }
 }`, name, pagerDutyIntegrationId)
}

func testAccResourceCoralogixPagerdutyConnectorUpdate(name string) string {
	return fmt.Sprintf(`resource "coralogix_connector" "example" {
   id               = "%[1]v"
   type             = "pagerduty"
   name             = "%[1]v"
   description      = "test pagerduty connector updated"
   connector_config = {
     fields = [
       {
         field_name = "integrationKey"
         value      = "%[2]v"
       }
     ]
   }
 }`, name, pagerDutyIntegrationId)
}

func testAccResourceCoralogixPagerdutyIncidentsConnector(name string) string {
	return fmt.Sprintf(`resource "coralogix_connector" "example" {
   id               = "%[1]v"
   type             = "pagerduty_incidents"
   name             = "%[1]v"
   description      = "test pagerduty incidents connector"
   connector_config = {
     fields = [
       {
         field_name = "integrationId"
         value      = "%[2]v"
       },
       {
         field_name = "service"
         value      = "PXXXXXX"
       }
     ]
   }
 }`, name, pagerDutyIntegrationId)
}

func testAccResourceCoralogixMicrosoftTeamsConnector(name string) string {
	return fmt.Sprintf(`resource "coralogix_connector" "example" {
   id               = "%[1]v"
   type             = "microsoft_teams"
   name             = "%[1]v"
   description      = "test microsoft teams connector"
   connector_config = {
     fields = [
       {
         field_name = "integrationId"
         value      = "%[2]v"
       },
       {
         field_name = "teamId"
         value      = "%[3]v"
       },
       {
         field_name = "channelId"
         value      = "%[4]v"
       }
     ]
   }
 }`, name, msTeamsIntegrationId, msTeamsTeamId, msTeamsChannelId)
}

func TestAccCoralogixResourceIncidentIOConnector(t *testing.T) {
	name := uuid.NewString()
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			if incidentIOAPIKey == "" || incidentIOAlertEventsURL == "" || incidentIOAlertSourceTok == "" {
				t.Skipf("INCIDENT_IO_API_KEY, INCIDENT_IO_ALERT_EVENTS_URL, and INCIDENT_IO_ALERT_SOURCE_TOKEN must be set to run this acceptance test locally")
			}
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCoralogixIncidentIOConnector(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(connectorResourceName, "id", name),
					resource.TestCheckResourceAttr(connectorResourceName, "type", "incident_io"),
					resource.TestCheckResourceAttr(connectorResourceName, "name", name),
					resource.TestCheckResourceAttr(connectorResourceName, "description", "test incident.io connector"),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "apiKey",
						"value":      incidentIOAPIKey,
					}),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "alertEventsUrl",
						"value":      incidentIOAlertEventsURL,
					}),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "alertSourceToken",
						"value":      incidentIOAlertSourceTok,
					}),
				),
			},
			{
				ResourceName:      connectorResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccResourceCoralogixEventbridgeConnector(name string) string {
	return fmt.Sprintf(`resource "coralogix_connector" "example" {
   id               = "%[1]v"
   type             = "eventbridge"
   name             = "%[1]v"
   description      = "test eventbridge connector"
   connector_config = {
     fields = [
       {
         field_name = "integrationId"
         value      = "%[2]v"
       }
     ]
   }
 }`, name, eventbridgeIntegrationId)
}

func testAccResourceCoralogixIncidentIOConnector(name string) string {
	return fmt.Sprintf(`resource "coralogix_connector" "example" {
   id               = "%[1]v"
   type             = "incident_io"
   name             = "%[1]v"
   description      = "test incident.io connector"
   connector_config = {
     fields = [
       {
         field_name = "apiKey"
         value      = "%[2]v"
       },
       {
         field_name = "alertEventsUrl"
         value      = "%[3]v"
       },
       {
         field_name = "alertSourceToken"
         value      = "%[4]v"
       }
     ]
   }
 }`, name, incidentIOAPIKey, incidentIOAlertEventsURL, incidentIOAlertSourceTok)
}

func testAccResourceCoralogixEmailConnector(name string) string {
	return fmt.Sprintf(`resource "coralogix_connector" "example" {
   id               = "%[1]v"
   type             = "email"
   name             = "%[1]v"
   description      = "email connector example"
   connector_config = {
     fields = [
       {
         field_name = "emailAddresses"
         value      = "[\"email1@example.com\",\"email2@example.com\"]"
       }
     ]
   }
   config_overrides = []
 }`, name)
}

func testAccResourceCoralogixEmailConnectorUpdate(name string) string {
	return fmt.Sprintf(`resource "coralogix_connector" "example" {
   id               = "%[1]v"
   type             = "email"
   name             = "%[1]v-updated"
   description      = "email connector example updated"
   connector_config = {
     fields = [
       {
         field_name = "emailAddresses"
         value      = "[\"email1@example.com\",\"email2@example.com\",\"email3@example.com\"]"
       }
     ]
   }
   config_overrides = []
 }`, name)
}

// The secret must reach the API and never reach state. The version map is what
// makes that work on a read, which has no configuration to consult.
func TestAccCoralogixResourceConnectorWriteOnlySecret(t *testing.T) {
	name := uuid.NewString()
	fields := `
      { field_name = "url",                  value = "https://api.staging.coralogix.net/mgmt/testing/tools/httpbin/post" },
      { field_name = "method",               value = "post" },
      { field_name = "additionalBodyFields", value = "{}" },`

	config := func(secret string, version int, description string) string {
		return fmt.Sprintf(`
resource "coralogix_connector" "wo" {
  id          = %[1]q
  name        = %[1]q
  description = %[4]q
  type        = "generic_https"

  connector_config = {
    fields = [%[5]s
    ]
    field_values_wo          = { additionalHeaders = %[2]q }
    field_values_wo_versions = { additionalHeaders = %[3]d }
  }
}
`, name, secret, version, description, fields)
	}

	// A version naming a field that is not supplied write-only would make the
	// read drop that field, so the schema rejects it.
	orphanVersion := fmt.Sprintf(`
resource "coralogix_connector" "wo" {
  id          = %[1]q
  name        = %[1]q
  description = "invalid"
  type        = "generic_https"

  connector_config = {
    fields = [%[2]s
      { field_name = "additionalHeaders",    value = "{}" },
    ]
    field_values_wo_versions = { additionalHeaders = 1 }
  }
}
`, name, fields)

	const rn = "coralogix_connector.wo"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Validation-only steps go first: a trailing invalid
				// configuration is also used for the post-test destroy.
				Config:      orphanVersion,
				ExpectError: regexp.MustCompile(`Version Without a Write-Only Field`),
			},
			{
				Config: config(`{"Authorization":"OAuth SECRET-ONE"}`, 1, "created"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(rn, "connector_config.field_values_wo"),
					resource.TestCheckResourceAttr(rn, "connector_config.field_values_wo_versions.additionalHeaders", "1"),
					// the secret's field is absent from the field list
					resource.TestCheckResourceAttr(rn, "connector_config.fields.#", "3"),
					resource.TestCheckTypeSetElemNestedAttrs(rn, "connector_config.fields.*", map[string]string{
						"field_name": "method", "value": "post",
					}),
					connectorBackendFieldEquals(name, "additionalHeaders", `{"Authorization":"OAuth SECRET-ONE"}`),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// an unrelated change must not wipe the secret: update takes
				// normal values from the plan and the secret from configuration
				Config: config(`{"Authorization":"OAuth SECRET-ONE"}`, 1, "description changed"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rn, "description", "description changed"),
					connectorBackendFieldEquals(name, "additionalHeaders", `{"Authorization":"OAuth SECRET-ONE"}`),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// rotation: the new secret reaches the API when the version moves
				Config: config(`{"Authorization":"OAuth SECRET-TWO"}`, 2, "description changed"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rn, "connector_config.field_values_wo_versions.additionalHeaders", "2"),
					connectorBackendFieldEquals(name, "additionalHeaders", `{"Authorization":"OAuth SECRET-TWO"}`),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// connectorBackendFieldEquals reads the connector from the API. A state
// assertion cannot show whether the secret actually arrived.
func connectorBackendFieldEquals(id, fieldName, want string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		// Built through the provider so it honours whichever of CORALOGIX_ENV
		// or CORALOGIX_DOMAIN the run supplies, as testAccPreCheck accepts
		// either. Reading the environment here would miss a domain-only run.
		clients, err := testAccNewClientSet()
		if err != nil {
			return err
		}
		c, _, _ := clients.GetNotifications()
		res, _, err := c.ConnectorsServiceGetConnector(context.Background(), id).Execute()
		if err != nil {
			return err
		}
		for _, f := range res.Connector.ConnectorConfig.Fields {
			if f.GetFieldName() == fieldName {
				if f.GetValue() != want {
					return fmt.Errorf("backend %s = %q, want %q", fieldName, f.GetValue(), want)
				}
				return nil
			}
		}
		return fmt.Errorf("%s absent from the backend connector", fieldName)
	}
}
