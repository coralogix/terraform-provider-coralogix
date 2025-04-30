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
	connectorResourceName = "coralogix_connector.test"
)

func TestConnector(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { TestAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testConnectorResource(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(customRoleResourceName, "name", "Test Custom Role"),
					resource.TestCheckResourceAttr(customRoleResourceName, "description", "This role is created with terraform!"),
					resource.TestCheckResourceAttr(customRoleResourceName, "parent_role", "Standard User"),
					resource.TestCheckTypeSetElemAttr(customRoleResourceName, "permissions.*", "spans.events2metrics:UpdateConfig"),
				),
			},
			{
				ResourceName:      connectorResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testConnectorUpdateResource(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(customRoleResourceName, "name", "Test Custom Role Renamed"),
					resource.TestCheckResourceAttr(customRoleResourceName, "description", "This role is renamed with terraform!"),
					resource.TestCheckResourceAttr(customRoleResourceName, "parent_role", "Standard User"),
					resource.TestCheckTypeSetElemAttr(customRoleResourceName, "permissions.*", "spans.events2metrics:UpdateConfig"),
					resource.TestCheckTypeSetElemAttr(customRoleResourceName, "permissions.*", "spans.events2metrics:ReadConfig"),
				),
			},
		},
	})
}

func testConnectorResource() string {
	return `resource "coralogix_connector" "example" {
   id               = "custom_id"
   type             = "slack"
   name             = "test-connector"
   description      = "test connector"
   connector_config = {
     fields = [
       {
         field_name = "Slack-Notifications"
         value      = "Slack-Notifications"
       }
     ]
   }
 }
`
}

func testConnectorUpdateResource() string {
	return `resource "coralogix_connector" "example" {
   id               = "custom_id"
   type             = "slack"
   name             = "updated-test-connector"
   description      = "updated test connector"
   connector_config = {
     fields = [
       {
         field_name = "Slack-Notifications"
         value      = "updated Slack-Notifications"
       }
     ]
   }
 }
`
}
