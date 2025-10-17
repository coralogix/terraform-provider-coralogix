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
						"value":      "https://httpbin.org/post",
					}),
				),
			},
		},
	})
}

func TestAccCoralogixResourceSlackConnector(t *testing.T) {
	name := uuid.NewString()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
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
				Config: testAccResourceCoralogixSlackConnectorUpdate(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(connectorResourceName, "id", name),
					resource.TestCheckResourceAttr(connectorResourceName, "type", "slack"),
					resource.TestCheckResourceAttr(connectorResourceName, "name", name),
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
	name := uuid.NewString()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
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
				Config: testAccResourceCoralogixPagerdutyConnectorUpdate(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(connectorResourceName, "id", name),
					resource.TestCheckResourceAttr(connectorResourceName, "type", "pagerduty"),
					resource.TestCheckResourceAttr(connectorResourceName, "name", name),
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
	 	value      = "https://httpbin.org/post"
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
	 	value      = "https://httpbin.org/post"
	  },
	  {
	 	field_name = "method"
	 	value      = "post"
	  }
     ]
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
 }`, name)
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
 }`, name)
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
         value      = "integration-key-example"
       }
     ]
   }
 }`, name)
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
         value      = "integration-key-example"
       }
     ]
   }
 }`, name)
}
