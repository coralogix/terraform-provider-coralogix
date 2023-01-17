package coralogix

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"terraform-provider-coralogix/coralogix/clientset"
)

var recordingRulesGroupsResourceName = "coralogix_recording_rules_group.test"

func TestAccCoralogixRecordingRulesGroups(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	parent := filepath.Dir(wd)
	filePath := parent + "/examples/recording_rules_group/rule_groups.yaml"
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckRecordingRulesGroupsDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceRecordingRulesGroups(filePath),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(recordingRulesGroupsResourceName, "id"),
				),
			},
		},
	})
}

func testAccCheckRecordingRulesGroupsDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).RecordingRulesGroups()
	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_recording_rules_group" {
			continue
		}

		resp, err := client.GetRecordingRuleRules(ctx)
		if err == nil {
			if resp == rs.Primary.ID {
				return fmt.Errorf("coralogix_recording_rules_group still exists: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}

func testAccCoralogixResourceRecordingRulesGroups(filePath string) string {
	return fmt.Sprintf(`resource "coralogix_recording_rules_group" "test" {
  	yaml_content = file("%s")
}
`,
		filePath)
}
