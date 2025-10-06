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
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var (
	customRoleSourceByID   = "data." + customRoleResourceName + "_by_id"
	customRoleSourceByName = "data." + customRoleResourceName + "_by_name"
)

func TestAccCoralogixDataSourceCustomRole(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testCustomRoleResource() +
					testCustomRoleDataSourceByID(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(customRoleSourceByID, "name", "Test Custom Role"),
					resource.TestCheckResourceAttr(customRoleSourceByID, "description", "This role is created with terraform!"),
					resource.TestCheckResourceAttr(customRoleSourceByID, "parent_role", "Standard User"),
				),
			},
			{
				Config: testCustomRoleResource() +
					testCustomRoleDataSourceByName(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(customRoleSourceByName, "name", "Test Custom Role"),
					resource.TestCheckResourceAttr(customRoleSourceByName, "description", "This role is created with terraform!"),
					resource.TestCheckResourceAttr(customRoleSourceByName, "parent_role", "Standard User"),
				),
			},
		},
	})
}

func testCustomRoleDataSourceByID() string {
	return `
data "coralogix_custom_role" "test_by_id" {
  id = coralogix_custom_role.test.id
}
`
}

func testCustomRoleDataSourceByName() string {
	return `
data "coralogix_custom_role" "test_by_name" {
  name = coralogix_custom_role.test.name
}
`
}
