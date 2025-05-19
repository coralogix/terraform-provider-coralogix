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

package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var (
	connectorResourceName = "coralogix_connector.example"
)

func TestAccCoralogixResourceGenericHttpsConnector(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { TestAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCoralogixGenericHttpsConnector(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(connectorResourceName, "id", "generic_https_terraform_acceptance_test_connector"),
					resource.TestCheckResourceAttr(connectorResourceName, "type", "generic_https"),
					resource.TestCheckResourceAttr(connectorResourceName, "name", "generic-https-connector"),
					resource.TestCheckResourceAttr(connectorResourceName, "description", "generic https connector"),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "method",
						"value":      "post",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "url",
						"value":      "https://httpbin.org/post",
					}),
				),
			},
			{
				ResourceName:      connectorResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccResourceCoralogixGenericHttpsConnectorUpdate(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(connectorResourceName, "id", "generic_https_terraform_acceptance_test_connector"),
					resource.TestCheckResourceAttr(connectorResourceName, "type", "generic_https"),
					resource.TestCheckResourceAttr(connectorResourceName, "name", "generic-https-connector-updated"),
					resource.TestCheckResourceAttr(connectorResourceName, "description", "generic https connector"),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "method",
						"value":      "post",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "url",
						"value":      "https://httpbin.org/post",
					}),
				),
			},
		},
	})
}

func TestAccCoralogixResourceSlackConnector(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { TestAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCoralogixSlackConnector(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(connectorResourceName, "id", "slack_terraform_acceptance_test_connector"),
					resource.TestCheckResourceAttr(connectorResourceName, "type", "slack"),
					resource.TestCheckResourceAttr(connectorResourceName, "name", "test-connector"),
					resource.TestCheckResourceAttr(connectorResourceName, "description", "test connector"),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "integrationId",
						"value":      "luigis-testing-grounds",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "channel",
						"value":      "luigis-testing-grounds",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "fallbackChannel",
						"value":      "luigis-testing-grounds",
					}),
				),
			},
			{
				ResourceName:      connectorResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccResourceCoralogixSlackConnectorUpdate(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(connectorResourceName, "id", "slack_terraform_acceptance_test_connector"),
					resource.TestCheckResourceAttr(connectorResourceName, "type", "slack"),
					resource.TestCheckResourceAttr(connectorResourceName, "name", "test-connector"),
					resource.TestCheckResourceAttr(connectorResourceName, "description", "test connector"),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "integrationId",
						"value":      "luigis-testing-grounds-updated",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "channel",
						"value":      "luigis-testing-grounds-updated",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "fallbackChannel",
						"value":      "luigis-testing-grounds-updated",
					}),
				),
			},
		},
	})
}

func TestAccCoralogixResourcePagerdutyConnector(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { TestAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCoralogixPagerdutyConnector(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(connectorResourceName, "id", "pagerduty_terraform_acceptance_test_connector"),
					resource.TestCheckResourceAttr(connectorResourceName, "type", "pagerduty"),
					resource.TestCheckResourceAttr(connectorResourceName, "name", "test-pagerduty-connector"),
					resource.TestCheckResourceAttr(connectorResourceName, "description", "test pagerduty connector"),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "integrationKey",
						"value":      "integration-key-example",
					}),
				),
			},
			{
				ResourceName:      connectorResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccResourceCoralogixPagerdutyConnectorUpdate(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(connectorResourceName, "id", "pagerduty_terraform_acceptance_test_connector"),
					resource.TestCheckResourceAttr(connectorResourceName, "type", "pagerduty"),
					resource.TestCheckResourceAttr(connectorResourceName, "name", "test-pagerduty-connector"),
					resource.TestCheckResourceAttr(connectorResourceName, "description", "test pagerduty connector updated"),
					resource.TestCheckTypeSetElemNestedAttrs(connectorResourceName, "connector_config.fields.*", map[string]string{
						"field_name": "integrationKey",
						"value":      "integration-key-example",
					}),
				),
			},
		},
	})
}

func testAccResourceCoralogixGenericHttpsConnector() string {
	return `resource "coralogix_connector" "example" {
   id               = "generic_https_terraform_acceptance_test_connector"
   type             = "generic_https"
   name             = "generic-https-connector"
   description      = "generic https connector"
   connector_config = {
     fields = [
	  {
	    field_name = "url"
	 	value      = "https://httpbin.org/post"
	  },
	  {
	 	field_name = "method"
	 	value      = "post"
	  }
     ]
   }
 }
`
}

func testAccResourceCoralogixGenericHttpsConnectorUpdate() string {
	return `resource "coralogix_connector" "example" {
   id               = "generic_https_terraform_acceptance_test_connector"
   type             = "generic_https"
   name             = "generic-https-connector-updated"
   description      = "generic https connector"
   connector_config = {
     fields = [
	  {
	    field_name = "url"
	 	value      = "https://httpbin.org/post"
	  },
	  {
	 	field_name = "method"
	 	value      = "post"
	  }
     ]
   }
 }
`
}

func testAccResourceCoralogixSlackConnector() string {
	return `resource "coralogix_connector" "example" {
   id               = "slack_terraform_acceptance_test_connector"
   type             = "slack"
   name             = "test-connector"
   description      = "test connector"
   connector_config = {
     fields = [
       {
         field_name = "integrationId"
         value      = "luigis-testing-grounds"
       },
	   {
	   	  field_name = "channel"
		  value      = "luigis-testing-grounds"
	   },
	   {
	   	  field_name = "fallbackChannel"
		  value      = "luigis-testing-grounds"
	   },
     ]
   }
 }
`
}

func testAccResourceCoralogixSlackConnectorUpdate() string {
	return `resource "coralogix_connector" "example" {
   id               = "slack_terraform_acceptance_test_connector"
   type             = "slack"
   name             = "test-connector"
   description      = "test connector"
   connector_config = {
     fields = [
       {
         field_name = "integrationId"
         value      = "luigis-testing-grounds-updated"
       },
	   {
	   	  field_name = "channel"
		  value      = "luigis-testing-grounds-updated"
	   },
	   {
	   	  field_name = "fallbackChannel"
		  value      = "luigis-testing-grounds-updated"
	   },
     ]
   }
 }
`
}

func testAccResourceCoralogixPagerdutyConnector() string {
	return `resource "coralogix_connector" "example" {
   id               = "pagerduty_terraform_acceptance_test_connector"
   type             = "pagerduty"
   name             = "test-pagerduty-connector"
   description      = "test pagerduty connector"
   connector_config = {
     fields = [
       {
         field_name = "integrationKey"
         value      = "integration-key-example"
       }
     ]
   }
 }
`
}

func testAccResourceCoralogixPagerdutyConnectorUpdate() string {
	return `resource "coralogix_connector" "example" {
   id               = "pagerduty_terraform_acceptance_test_connector"
   type             = "pagerduty"
   name             = "test-pagerduty-connector"
   description      = "test pagerduty connector updated"
   connector_config = {
     fields = [
       {
         field_name = "integrationKey"
         value      = "integration-key-example"
       }
     ]
   }
 }
`
}
