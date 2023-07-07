package coralogix

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var recordingRulesGroupsSetDataSourceName = "data." + recordingRulesGroupsSetResourceName

func TestAccCoralogixDataSourceRecordingRulesGroupsSet_basic(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	parent := filepath.Dir(wd)
	filePath := parent + "/examples/recording_rules_groups_set/rule-group-set.yaml"

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceRecordingRulesGroupsSetFromYaml(filePath) +
					testAccCoralogixDataSourceRecordingRulesGroupsSet_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(recordingRulesGroupsSetDataSourceName, "id"),
				),
			},
		},
	})
}

func testAccCoralogixDataSourceRecordingRulesGroupsSet_read() string {
	return `data "coralogix_recording_rules_groups_set" "test" {
		id = coralogix_recording_rules_groups_set.test.id
}
`
}
