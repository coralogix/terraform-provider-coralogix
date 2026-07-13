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
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var (
	connectorResourceName = "coralogix_connector.example"
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
