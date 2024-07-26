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

//
//import (
//	"context"
//	"fmt"
//	"testing"
//
//	"terraform-provider-coralogix/coralogix/clientset"
//
//	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
//	"github.com/hashicorp/terraform-plugin-testing/terraform"
//)
//
//var teamResourceName = "coralogix_team.test"
//
//func TestAccCoralogixResourceTeam(t *testing.T) {
//	resource.Test(t, resource.TestCase{
//		PreCheck:                 func() { testAccPreCheck(t) },
//		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
//		CheckDestroy:             testAccCheckUserDestroy,
//		Steps: []resource.TestStep{
//			{
//				Config: testAccCoralogixResourceTeam(),
//				Check: resource.ComposeAggregateTestCheckFunc(
//					resource.TestCheckResourceAttrSet(userResourceName, "id"),
//					resource.TestCheckResourceAttr(userResourceName, "name", "example"),
//					resource.TestCheckResourceAttr(userResourceName, "retention", "1"),
//					resource.TestCheckResourceAttr(userResourceName, "daily_quota", "0.025"),
//				),
//			},
//			{
//				ResourceName:      userResourceName,
//				ImportState:       true,
//				ImportStateVerify: true,
//			},
//			{
//				Config: testAccCoralogixResourceUpdatedTeam(),
//				Check: resource.ComposeAggregateTestCheckFunc(
//					resource.TestCheckResourceAttrSet(userResourceName, "id"),
//					resource.TestCheckResourceAttr(userResourceName, "name", "updated_example"),
//					resource.TestCheckTypeSetElemAttr(userResourceName, "team_admins_emails.*", "example@coralogix.com"),
//					resource.TestCheckResourceAttr(userResourceName, "retention", "1"),
//					resource.TestCheckResourceAttr(userResourceName, "daily_quota", "0.1"),
//				),
//			},
//		},
//	})
//}
//
//func testAccCheckTeamDestroy(s *terraform.State) error {
//	client := testAccProvider.Meta().(*clientset.ClientSet).Teams()
//
//	ctx := context.TODO()
//
//	for _, rs := range s.RootModule().Resources {
//		if rs.Type != "coralogix_team" {
//			continue
//		}
//
//		resp, err := client.GetTeam(ctx, rs.Primary.ID)
//		if err == nil && resp != nil {
//			return fmt.Errorf("team still exists and active: %s", rs.Primary.ID)
//		}
//	}
//
//	return nil
//}
//
//func testAccCoralogixResourceTeam() string {
//	return `resource "coralogix_team" "example" {
//  		name                    = "example"
//  		retention               = 1
//  		daily_quota             = 0.025
//	}
//	`
//}
//
//func testAccCoralogixResourceUpdatedTeam() string {
//	return `resource "coralogix_team" "example" {
//  		name                    = "updated_example
//  		team_admins_emails      = ["example@coralogix.com"]
//  		retention               = 1
//  		daily_quota             = 0.1
//	}
//	`
//}
