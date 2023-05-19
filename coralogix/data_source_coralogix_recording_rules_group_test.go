package coralogix

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

var recordingRulesGroupsDataSourceName = "data." + recordingRulesGroupsResourceName

func TestAccCoralogixDataSourceRecordingRulesGroups_basic(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	parent := filepath.Dir(wd)
	filePath := parent + "/examples/recording_rules_group/rule-group.yaml"

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceRecordingRulesGroupsSetFromYaml(filePath) +
					testAccCoralogixDataSourceRecordingRulesGroups_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(recordingRulesGroupsDataSourceName, "group.0.rules.#", "2"),
				),
			},
		},
	})
}

func testAccCoralogixDataSourceRecordingRulesGroups_read() string {
	return `data "coralogix_recording_rules_groups_set" "test" {
		id = coralogix_recording_rules_groups_set.test.id
}
`
}
