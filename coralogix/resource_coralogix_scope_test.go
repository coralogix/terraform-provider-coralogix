package coralogix

import (
	"context"
	"fmt"
	"testing"

	"terraform-provider-coralogix/coralogix/clientset"
	scopes "terraform-provider-coralogix/coralogix/clientset/grpc/scopes"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccCoralogixResourceScope(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckScopeDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceScope(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(userResourceName, "id"),
					resource.TestCheckResourceAttr(userResourceName, "display_name", "ExampleScope"),
					resource.TestCheckResourceAttr(userResourceName, "team_id", "4013254"),
					resource.TestCheckResourceAttr(userResourceName, "default_expression", "true"),
					resource.TestCheckResourceAttr(userResourceName, "filters.0.entity_type", "logs"),
					resource.TestCheckResourceAttr(userResourceName, "filters.0.expression", "(subsystemName == 'purchases') || (subsystemName == 'signups')"),
				),
			},
			{
				ResourceName:      userResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccCoralogixResourceUpdatedScope(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(userResourceName, "id"),
					resource.TestCheckResourceAttr(userResourceName, "display_name", "NewExampleScope"),
					resource.TestCheckResourceAttr(userResourceName, "team_id", "4013254"),
					resource.TestCheckResourceAttr(userResourceName, "default_expression", "true"),
					resource.TestCheckResourceAttr(userResourceName, "filters.0.entity_type", "logs"),
					resource.TestCheckResourceAttr(userResourceName, "filters.0.expression", "(subsystemName == 'purchases') || (subsystemName == 'signups')"),
				),
			},
		},
	})
}

func testAccCheckScopeDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).Scopes()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_scope" {
			continue
		}
		ctx := context.TODO()

		resp, err := client.Get(ctx, &scopes.GetTeamScopesByIdsRequest{
			Ids: []string{rs.Primary.ID},
		})
		if err == nil && resp != nil {
			return fmt.Errorf("Scopes still exists: %v", rs.Primary.ID)
		}
	}

	return nil
}

func testAccCoralogixResourceScope() string {
	return `resource "coralogix_scope" "test" {
		display_name       = "ExampleScope"
		default_expression = "true"
		filters            = [
		  {
			entity_type = "logs"
			expression  = "(subsystemName == 'purchases') || (subsystemName == 'signups')"
		  }
		]
	}
	`
}

func testAccCoralogixResourceUpdatedScope() string {
	return `resource "coralogix_scope" "test_upgraded" {  
		display_name       = "NewExampleScope"
		default_expression = "true"
		filters            = [
		{
			entity_type = "logs"
			expression  = "(subsystemName == 'purchases') || (subsystemName == 'signups')"
		}
		]
	`
}
