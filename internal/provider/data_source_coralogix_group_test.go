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
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var groupDataSourceName = "data." + groupResourceName

func TestAccCoralogixDataSourceGroup_basic(t *testing.T) {
	userName := randUserName()
	displayName := acctest.RandomWithPrefix("tf-acc-test-group")
	scopeName := acctest.RandomWithPrefix("tf-acc-test-scope")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceGroup(userName, displayName, scopeName) +
					testAccCoralogixDataSourceGroup_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(groupDataSourceName, "display_name", displayName),
				),
			},
		},
	})
}

func TestAccCoralogixDataSourceGroupByName(t *testing.T) {
	userName := randUserName()
	displayName := acctest.RandomWithPrefix("tf-acc-test-group")
	scopeName := acctest.RandomWithPrefix("tf-acc-test-scope")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceGroup(userName, displayName, scopeName) +
					testAccCoralogixDataSourceGroupByName_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(groupDataSourceName, "display_name", displayName),
					// The id pair is what proves the lookup resolved the right group.
					resource.TestCheckResourceAttrPair(groupDataSourceName, "id", groupResourceName, "id"),
					resource.TestCheckResourceAttrPair(groupDataSourceName, "role", groupResourceName, "role"),
					resource.TestCheckResourceAttrPair(groupDataSourceName, "scope_id", groupResourceName, "scope_id"),
					resource.TestCheckResourceAttrPair(groupDataSourceName, "members.#", groupResourceName, "members.#"),
					resource.TestCheckResourceAttrPair(groupDataSourceName, "members.0", groupResourceName, "members.0"),
				),
			},
		},
	})
}

// TestAccCoralogixDataSourceGroupByName_exactMatch guards against a prefix match: one group
// name is a strict prefix of the other, and the lookup must return the shorter one.
func TestAccCoralogixDataSourceGroupByName_exactMatch(t *testing.T) {
	userName := randUserName()
	displayName := acctest.RandomWithPrefix("tf-acc-test-group")
	scopeName := acctest.RandomWithPrefix("tf-acc-test-scope")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceGroup(userName, displayName, scopeName) +
					testAccCoralogixDataSourceGroupPrefixSibling(displayName) +
					testAccCoralogixDataSourceGroupByName_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(groupDataSourceName, "display_name", displayName),
					resource.TestCheckResourceAttrPair(groupDataSourceName, "id", groupResourceName, "id"),
				),
			},
		},
	})
}

func TestAccCoralogixDataSourceGroupByName_notFound(t *testing.T) {
	displayName := acctest.RandomWithPrefix("tf-acc-test-group-missing")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccCoralogixDataSourceGroupByMissingName_read(displayName),
				ExpectError: regexp.MustCompile("Group with display name .* not found"),
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

func testAccCoralogixDataSourceGroupPrefixSibling(displayName string) string {
	return fmt.Sprintf(`
	resource "coralogix_group" "prefix_sibling" {
		display_name = "%s-suffix"
		role         = "Read Only"
	}
`, displayName)
}

func testAccCoralogixDataSourceGroupByMissingName_read(displayName string) string {
	return fmt.Sprintf(`data "coralogix_group" "missing" {
	display_name = "%s"
}
`, displayName)
}
