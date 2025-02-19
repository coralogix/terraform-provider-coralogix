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
	customRoleResourceName = "coralogix_custom_role.test"
)

func TestCustomRole(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { TestAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testCustomRoleResource(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(customRoleResourceName, "name", "Test Custom Role"),
					resource.TestCheckResourceAttr(customRoleResourceName, "description", "This role is created with terraform!"),
					resource.TestCheckResourceAttr(customRoleResourceName, "parent_role", "Standard User"),
					resource.TestCheckTypeSetElemAttr(customRoleResourceName, "permissions.*", "spans.events2metrics:UpdateConfig"),
				),
			},
			{
				ResourceName:      customRoleResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testCustomRoleUpdateResource(),
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

func testCustomRoleResource() string {
	return `resource "coralogix_custom_role" "test" {
  name  = "Test Custom Role"
  description = "This role is created with terraform!"
  parent_role = "Standard User"
  permissions = ["spans.events2metrics:UpdateConfig"]
}
`
}

func testCustomRoleUpdateResource() string {
	return `resource "coralogix_custom_role" "test" {
  name  = "Test Custom Role Renamed"
  description = "This role is renamed with terraform!"
  parent_role = "Standard User"
  permissions = ["spans.events2metrics:UpdateConfig", "spans.events2metrics:ReadConfig"]
}
`
}
