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

var groupDataSourceName = "data." + groupResourceName

func TestAccCoralogixDataSourceGroup_basic(t *testing.T) {
	userName := randUserName()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { TestAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceGroup(userName) +
					testAccCoralogixDataSourceGroup_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(groupDataSourceName, "display_name", "example"),
				),
			},
		},
	})
}

func TestAccCoralogixDataSourceGroupbyName(t *testing.T) {
	userName := randUserName()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { TestAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceGroup(userName) +
					testAccCoralogixDataSourceGroupByName_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(groupDataSourceName, "display_name", "example"),
				),
			},
		},
	})
}

func testAccCoralogixDataSourceGroup_read() string {
	return `data "coralogix_group" "test" {
	id = coralogix_group.test.id
}
`
}

func testAccCoralogixDataSourceGroupByName_read() string {
	return `data "coralogix_group" "test" {
	display_name = coralogix_group.test.display_name
}
`
}
