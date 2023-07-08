package coralogix

import (
	"context"
	"encoding/json"
	"fmt"

	"terraform-provider-coralogix/coralogix/clientset"

	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var tcoPolicyOverrideResourceName = "coralogix_tco_policy_override.test"

//func TestAccCoralogixResourceTCOPolicyOverrideCreate(t *testing.T) {
//	resource.Test(t, resource.TestCase{
//		PreCheck:          func() { testAccPreCheck(t) },
//		ProviderFactories: testAccProviderFactories,
//		CheckDestroy:      testAccTCOPolicyOverrideCheckDestroy,
//		Steps: []resource.TestStep{
//			{
//				Config: testAccCoralogixResourceTCOPolicyOverride(),
//				Check: resource.ComposeTestCheckFunc(
//					resource.TestCheckResourceAttr(tcoPolicyOverrideResourceName, "priority", "medium"),
//					resource.TestCheckResourceAttr(tcoPolicyOverrideResourceName, "severity", "debug"),
//					resource.TestCheckResourceAttr(tcoPolicyOverrideResourceName, "application_name", "prod"),
//					resource.TestCheckResourceAttr(tcoPolicyOverrideResourceName, "subsystem_name", "mobile"),
//				),
//			},
//			{
//				ResourceName:      tcoPolicyOverrideResourceName,
//				ImportState:       true,
//				ImportStateVerify: true,
//			},
//			{
//				Config: testAccCoralogixUpdatedResourceTCOPolicyOverride(),
//				Check: resource.ComposeTestCheckFunc(
//					resource.TestCheckResourceAttr(tcoPolicyOverrideResourceName, "priority", "low"),
//					resource.TestCheckTypeSetElemAttr(tcoPolicyOverrideResourceName, "severity", "warning"),
//					resource.TestCheckResourceAttr(tcoPolicyOverrideResourceName, "application_name", "dev"),
//					resource.TestCheckTypeSetElemAttr(tcoPolicyOverrideResourceName, "subsystem_name", "web"),
//				),
//			},
//		},
//	})
//}

func testAccTCOPolicyOverrideCheckDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).TCOPoliciesOverrides()
	ctx := context.TODO()
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_tco_policy_override" {
			continue
		}

		if resp, err := client.GetTCOPolicyOverride(ctx, rs.Primary.ID); err == nil {
			var m map[string]interface{}
			if err = json.Unmarshal([]byte(resp), &m); err == nil {
				if id, ok := m["id"]; ok && id.(string) == rs.Primary.ID {
					return fmt.Errorf("tco-policy-override still exists: %s", id)
				}
			}
		}
	}

	return nil
}

func testAccCoralogixResourceTCOPolicyOverride() string {
	return fmt.Sprintf(
		`resource "coralogix_tco_policy_override" test {
 					priority         = "medium"
  					severity         = "debug"
  					application_name = "prod"
  					subsystem_name   = "mobile"
				}
	`)
}

func testAccCoralogixUpdatedResourceTCOPolicyOverride() string {
	return fmt.Sprintf(
		`resource "coralogix_tco_policy_override" test {
 					  priority         = "high"
  						severity         = "error"
  						application_name = "staging"
  						subsystem_name   = "web"
				}
	`)
}
