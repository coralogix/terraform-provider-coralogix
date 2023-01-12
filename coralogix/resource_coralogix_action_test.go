package coralogix

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"terraform-provider-coralogix/coralogix/clientset"
	actionsv2 "terraform-provider-coralogix/coralogix/clientset/grpc/com/coralogix/actions/v2"
)

type actionTestParams struct {
	name, url, sourceType    string
	applications, subsystems []string
	isPrivate, isHidden      bool
}

func TestAccCoralogixResourceAction(t *testing.T) {
	resourceName := "coralogix_action.test"

	action := actionTestParams{
		name:         "google search action",
		url:          "https://www.google.com/",
		sourceType:   selectRandomlyFromSlice(actionValidSourceTypes),
		applications: []string{acctest.RandomWithPrefix("tf-acc-test")},
		subsystems:   []string{acctest.RandomWithPrefix("tf-acc-test")},
		isPrivate:    true,
		isHidden:     false,
	}

	updatedAction := actionTestParams{
		name:         "bing search action",
		url:          "https://www.bing.com/search?q={{$p.selected_value}}",
		sourceType:   selectRandomlyFromSlice(actionValidSourceTypes),
		applications: []string{acctest.RandomWithPrefix("tf-acc-test")},
		subsystems:   []string{acctest.RandomWithPrefix("tf-acc-test")},
		isPrivate:    true,
		isHidden:     false,
	}

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckActionDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAction(action),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", action.name),
					resource.TestCheckResourceAttr(resourceName, "url", action.url),
					resource.TestCheckResourceAttr(resourceName, "source_type", action.sourceType),
					resource.TestCheckResourceAttr(resourceName, "applications.0", action.applications[0]),
					resource.TestCheckResourceAttr(resourceName, "subsystems.0", action.subsystems[0]),
					resource.TestCheckResourceAttr(resourceName, "is_private", fmt.Sprintf("%t", action.isPrivate)),
					resource.TestCheckResourceAttr(resourceName, "is_hidden", fmt.Sprintf("%t", action.isHidden)),
				),
			},
			{
				ResourceName: resourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAction(updatedAction),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("resource.coralogix_action.test", "id"),
					resource.TestCheckResourceAttr("resource.coralogix_action.test", "name", updatedAction.name),
					resource.TestCheckResourceAttr("resource.coralogix_action.test", "url", updatedAction.url),
					resource.TestCheckResourceAttr("resource.coralogix_action.test", "source_type", updatedAction.sourceType),
					resource.TestCheckResourceAttr("resource.coralogix_action.test", "applications.0", updatedAction.applications[0]),
					resource.TestCheckResourceAttr("resource.coralogix_action.test", "subsystems.0", updatedAction.subsystems[0]),
					resource.TestCheckResourceAttr("resource.coralogix_action.test", "is_private", fmt.Sprintf("%t", updatedAction.isPrivate)),
					resource.TestCheckResourceAttr("resource.coralogix_action.test", "is_hidden", fmt.Sprintf("%t", updatedAction.isHidden)),
				),
			},
		},
	})
}

func testAccCheckActionDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).Actions()
	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_action" {
			continue
		}

		req := &actionsv2.GetActionRequest{
			Id: wrapperspb.String(rs.Primary.ID),
		}

		resp, err := client.GetAction(ctx, req)
		if err == nil {
			if resp.Action.Id.Value == rs.Primary.ID {
				return fmt.Errorf("action still exists: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}

func testAccCoralogixResourceAction(action actionTestParams) string {
	return fmt.Sprintf(
		`resource "coralogix_action" "test" {
  						name               = "%s"
  						url			       = "%s"
  						source_type		   = "%s"
  						applications       =  %s
  						subsystems 		   =  %s
  						is_private         =  %t
}
`, action.name, action.url, action.sourceType, sliceToString(action.applications), sliceToString(action.subsystems), action.isPrivate)
}
