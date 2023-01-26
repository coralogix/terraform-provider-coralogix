package coralogix

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"terraform-provider-coralogix/coralogix/clientset"
	recordingrules "terraform-provider-coralogix/coralogix/clientset/grpc/recording-rules-groups/v1"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

var recordingRulesGroupsResourceName = "coralogix_recording_rules_group.test"

func TestAccCoralogixRecordingRulesGroupsFromYaml(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	parent := filepath.Dir(wd)
	filePath := parent + "/examples/recording_rules_group/rule-group.yaml"
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckRecordingRulesGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceRecordingRulesGroupsFromYaml(filePath),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(recordingRulesGroupsResourceName, "id"),
					resource.TestCheckResourceAttr(recordingRulesGroupsResourceName, "group.0.name", "Foo"),
					resource.TestCheckResourceAttr(recordingRulesGroupsResourceName, "group.0.interval", "180"),
					resource.TestCheckResourceAttr(recordingRulesGroupsResourceName, "group.0.rules.#", "2"),
				),
			},
		},
	})
}

func TestAccCoralogixRecordingRulesGroupsExplicit(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckRecordingRulesGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceRecordingRulesGroupsExplicit(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(recordingRulesGroupsResourceName, "id"),
					resource.TestCheckResourceAttr(recordingRulesGroupsResourceName, "group.0.name", "Foo"),
					resource.TestCheckResourceAttr(recordingRulesGroupsResourceName, "group.0.interval", "180"),
					resource.TestCheckResourceAttr(recordingRulesGroupsResourceName, "group.0.rules.#", "2"),
				),
			},
		},
	})
}

func testAccCheckRecordingRulesGroupDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).RecordingRuleGroups()
	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_recording_rules_group" {
			continue
		}

		req := &recordingrules.FetchRuleGroup{Name: rs.Primary.ID}
		resp, err := client.GetRecordingRuleGroup(ctx, req)
		if err == nil {
			if resp != nil && resp.RuleGroup != nil && resp.RuleGroup.Name == rs.Primary.ID {
				return fmt.Errorf("coralogix_recording_rules_group still exists: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}

func testAccCoralogixResourceRecordingRulesGroupsFromYaml(filePath string) string {
	return fmt.Sprintf(
		`resource "coralogix_recording_rules_group" "test" {
					yaml_content = file("%s")
				}
`, filePath)
}

func testAccCoralogixResourceRecordingRulesGroupsExplicit() string {
	return `resource "coralogix_recording_rules_group" "test" {
  				group {
					name     = "Foo"
    				interval = 180
    				 rules {
      					record = "ts3db_live_ingester_write_latency:3m"
      					expr   = "sum(rate(ts3db_live_ingester_write_latency_seconds_count{CX_LEVEL=\"staging\",pod=~\"ts3db-live-ingester.*\"}[2m])) by (pod)"
   					 }
    				rules {
      					record = "job:http_requests_total:sum"
      					expr = "sum(rate(http_requests_total[5m])) by (job)"
    				}
  				}
	}
`
}
