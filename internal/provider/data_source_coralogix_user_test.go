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
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

var userDataSourceName = "data." + userResourceName

var (
	userByUserNameDataSourceName          = "data.coralogix_user.by_user_name"
	userByMixedCaseUserNameDataSourceName = "data.coralogix_user.by_mixed_case_user_name"
)

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
				),
			},
		},
	})
}

func TestAccCoralogixDataSourceUser_byUserName(t *testing.T) {
	userName := randUserName()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceUser(userName) +
					testAccCoralogixDataSourceUser_readByUserName(userName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(userByUserNameDataSourceName, "id", userResourceName, "id"),
					resource.TestCheckResourceAttr(userByUserNameDataSourceName, "user_name", userName),
					resource.TestCheckResourceAttrPair(userByMixedCaseUserNameDataSourceName, "id", userResourceName, "id"),
					resource.TestCheckResourceAttr(userByMixedCaseUserNameDataSourceName, "user_name", strings.ToUpper(userName)),
					resource.TestCheckResourceAttrPair(userByUserNameDataSourceName, "name.given_name", userResourceName, "name.given_name"),
				),
			},
		},
	})
}

func TestAccCoralogixDataSourceUser_byUserNameNotFound(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccCoralogixDataSourceUser_readByUserNameOnly(randUserName()),
				ExpectError: regexp.MustCompile("User with user_name .* not found"),
			},
		},
	})
}

func TestAccCoralogixDataSourceUser_bothLookupKeysSet(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccCoralogixDataSourceUser_readByBothKeys(),
				ExpectError: regexp.MustCompile("Invalid Attribute Combination"),
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
				ExpectError: regexp.MustCompile("Missing Attribute Configuration"),
			},
		},
	})
}

func TestAccCoralogixDataSourceUser_emptyUserNameRejected(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "coralogix_user" "empty_user_name" {
				  user_name = ""
				}`,
				ExpectError: regexp.MustCompile("Invalid Attribute Value Length"),
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

func testAccCoralogixDataSourceUser_readByUserName(userName string) string {
	return fmt.Sprintf(`
	data "coralogix_user" "by_user_name" {
	  user_name  = "%s"
	  depends_on = [coralogix_user.test]
	}

	data "coralogix_user" "by_mixed_case_user_name" {
	  user_name  = "%s"
	  depends_on = [coralogix_user.test]
	}
`, userName, strings.ToUpper(userName))
}

func testAccCoralogixDataSourceUser_readByBothKeys() string {
	return `data "coralogix_user" "both_keys" {
	  id        = "00000000-0000-0000-0000-000000000000"
	  user_name = "both-keys@example.com"
	}
`
}

func testAccCoralogixDataSourceUser_readByUserNameOnly(userName string) string {
	return fmt.Sprintf(`
	data "coralogix_user" "by_user_name" {
	  user_name = "%s"
	}
`, userName)
}
