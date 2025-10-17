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
	"context"
	"fmt"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var userResourceName = "coralogix_user.test"

func TestAccCoralogixResourceUser(t *testing.T) {
	userName := randUserName()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceUser(userName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(userResourceName, "id"),
					resource.TestCheckResourceAttr(userResourceName, "user_name", userName),
					resource.TestCheckResourceAttr(userResourceName, "name.given_name", "Test"),
					resource.TestCheckResourceAttr(userResourceName, "name.family_name", "User"),
				),
			},
			{
				ResourceName:      userResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckUserDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).Users()

	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_user" {
			continue
		}

		resp, err := client.Get(ctx, rs.Primary.ID)
		if err == nil && resp != nil {
			if *resp.ID == rs.Primary.ID && resp.Active {
				return fmt.Errorf("user still exists and active: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}

func randUserName() string {
	return "test@coralogix.com"
}

func testAccCoralogixResourceUser(userName string) string {
	return fmt.Sprintf(`
	resource "coralogix_user" "test" {
	  user_name = "%s"
	  name = {
		given_name = "Test"
		family_name = "User"
      }
	}
`, userName)
}
