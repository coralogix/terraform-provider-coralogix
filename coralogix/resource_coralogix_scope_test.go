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
					resource.TestCheckResourceAttrSet("coralogix_scope.test", "id"),
					resource.TestCheckResourceAttr("coralogix_scope.test", "display_name", "ExampleScope"),
					resource.TestCheckResourceAttr("coralogix_scope.test", "team_id", "4013254"),
					resource.TestCheckResourceAttr("coralogix_scope.test", "default_expression", "<v1>true"),
					resource.TestCheckResourceAttr("coralogix_scope.test", "filters.0.entity_type", "logs"),
					resource.TestCheckResourceAttr("coralogix_scope.test", "filters.0.expression", "<v1>(subsystemName == 'purchases') || (subsystemName == 'signups')"),
				),
			},
			{
				ResourceName:      "coralogix_scope.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccCoralogixResourceUpdatedScope(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("coralogix_scope.test_upgraded", "id"),
					resource.TestCheckResourceAttr("coralogix_scope.test_upgraded", "display_name", "NewExampleScope"),
					resource.TestCheckResourceAttr("coralogix_scope.test_upgraded", "team_id", "4013254"),
					resource.TestCheckResourceAttr("coralogix_scope.test_upgraded", "default_expression", "<v1>true"),
					resource.TestCheckResourceAttr("coralogix_scope.test_upgraded", "filters.0.entity_type", "logs"),
					resource.TestCheckResourceAttr("coralogix_scope.test_upgraded", "filters.0.expression", "<v1>(subsystemName == 'purchases') || (subsystemName == 'signups')"),
				),
			},
		},
	})
}

func testAccCheckScopeDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).Scopes()
	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_scope" {
			continue
		}

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
		default_expression = "<v1>true"
		filters            = [
		  {
			entity_type = "logs"
			expression  = "<v1>(subsystemName == 'purchases') || (subsystemName == 'signups')"
		  }
		]
	}
	`
}

func testAccCoralogixResourceUpdatedScope() string {
	return `resource "coralogix_scope" "test_upgraded" {  
		display_name       = "NewExampleScope"
		default_expression = "<v1>true"
		filters            = [
		{
			entity_type = "logs"
			expression  = "<v1>(subsystemName == 'purchases') || (subsystemName == 'signups')"
		}
		]
	`
}
