package coralogix

import (
	"context"
	"fmt"
	"testing"

	"terraform-provider-coralogix/coralogix/clientset"
	actions "terraform-provider-coralogix/coralogix/clientset/grpc/actions/v2"

	terraform2 "github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var actionResourceName = "coralogix_action.test"

type actionTestParams struct {
	name, url, sourceType    string
	applications, subsystems []string
	isPrivate, isHidden      bool
}

func TestAccCoralogixResourceAction(t *testing.T) {
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
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckActionDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAction(action),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(actionResourceName, "id"),
					resource.TestCheckResourceAttr(actionResourceName, "name", action.name),
					resource.TestCheckResourceAttr(actionResourceName, "url", action.url),
					resource.TestCheckResourceAttr(actionResourceName, "source_type", action.sourceType),
					resource.TestCheckResourceAttr(actionResourceName, "applications.0", action.applications[0]),
					resource.TestCheckResourceAttr(actionResourceName, "subsystems.0", action.subsystems[0]),
					resource.TestCheckResourceAttr(actionResourceName, "is_private", fmt.Sprintf("%t", action.isPrivate)),
					resource.TestCheckResourceAttr(actionResourceName, "is_hidden", fmt.Sprintf("%t", action.isHidden)),
				),
			},
			{
				ResourceName: actionResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceAction(updatedAction),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(actionResourceName, "id"),
					resource.TestCheckResourceAttr(actionResourceName, "name", updatedAction.name),
					resource.TestCheckResourceAttr(actionResourceName, "url", updatedAction.url),
					resource.TestCheckResourceAttr(actionResourceName, "source_type", updatedAction.sourceType),
					resource.TestCheckResourceAttr(actionResourceName, "applications.0", updatedAction.applications[0]),
					resource.TestCheckResourceAttr(actionResourceName, "subsystems.0", updatedAction.subsystems[0]),
					resource.TestCheckResourceAttr(actionResourceName, "is_private", fmt.Sprintf("%t", updatedAction.isPrivate)),
					resource.TestCheckResourceAttr(actionResourceName, "is_hidden", fmt.Sprintf("%t", updatedAction.isHidden)),
				),
			},
		},
	})
}

func testAccCheckActionDestroy(s *terraform.State) error {
	testAccProvider = OldProvider()
	rc := terraform2.ResourceConfig{}
	testAccProvider.Configure(context.Background(), &rc)
	client := testAccProvider.Meta().(*clientset.ClientSet).Actions()
	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_action" {
			continue
		}

		req := &actions.GetActionRequest{
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
