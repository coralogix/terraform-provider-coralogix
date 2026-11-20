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

// The generated data source looks a user up by id. The released user_name
// lookup filtered the Search with the configured username, which the generator
// cannot express yet (V6): the scenarios that exercised it are gone, and a
// configuration that sets user_name is rejected as a read-only attribute.

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
				ExpectError: regexp.MustCompile("Missing required argument"),
			},
		},
	})
}

func TestAccCoralogixDataSourceUser_userNameRejected(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccCoralogixDataSourceUser_readByUserNameOnly(randUserName()),
				ExpectError: regexp.MustCompile("Value for unconfigurable attribute"),

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

func testAccCoralogixDataSourceUser_readByUserNameOnly(userName string) string {
	return `data "coralogix_user" "by_user_name" {
	  user_name = "` + userName + `"
	}
`
}
