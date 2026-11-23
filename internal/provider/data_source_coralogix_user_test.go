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
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var userDataSourceName = "data." + userResourceName

// The data source looks a user up by id or by user_name, as the released data
// source did. The wrapper resolves a configured user_name through the Search
// username filter and hands the resolved id to the generated read.

func TestAccCoralogixDataSourceUser_basic(t *testing.T) {
	userName := randUserName()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceUser(userName) +
					testAccCoralogixDataSourceUser_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(userDataSourceName, "user_name", userName),
					resource.TestCheckResourceAttrPair(userDataSourceName, "id", userResourceName, "id"),
				),
			},
		},
	})
}

func TestAccCoralogixDataSourceUser_noLookupKeySet(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      `data "coralogix_user" "no_key" {}`,
				ExpectError: regexp.MustCompile("(?i)exactly one of these attributes must be configured"),
			},
		},
	})
}

func TestAccCoralogixDataSourceUser_userNameLookup(t *testing.T) {
	userName := randUserName()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// The user exists: the test creates it with the same user_name
				// the lookup configures.
				Config: testAccCoralogixResourceUser(userName) + testAccCoralogixDataSourceUser_readByUserNameOnly(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.coralogix_user.by_user_name", "user_name", userName),
					resource.TestCheckResourceAttrSet("data.coralogix_user.by_user_name", "id"),
				),
			},
		},
	})
}

func testAccCoralogixDataSourceUser_read() string {
	return `data "coralogix_user" "test" {
	id = coralogix_user.test.id
	}
`
}

func testAccCoralogixDataSourceUser_readByUserNameOnly() string {
	// The reference orders the read after the create: a literal would plan the
	// lookup during the first plan, before the user exists.
	return `data "coralogix_user" "by_user_name" {
	  user_name = coralogix_user.test.user_name
	}
`
}
