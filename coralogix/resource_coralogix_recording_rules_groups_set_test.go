package coralogix

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"terraform-provider-coralogix/coralogix/clientset"
	recordingrules "terraform-provider-coralogix/coralogix/clientset/grpc/recording-rules-groups-sets/v1"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var recordingRulesGroupsSetResourceName = "coralogix_recording_rules_groups_set.test"

func TestAccCoralogixRecordingRulesGroupsSetFromYaml(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	parent := filepath.Dir(wd)
	filePath := parent + "/examples/recording_rules_groups_set/rule-group-set.yaml"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRecordingRulesGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceRecordingRulesGroupsSetFromYaml(filePath),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(recordingRulesGroupsSetResourceName, "id"),
					resource.TestCheckTypeSetElemNestedAttrs(recordingRulesGroupsSetResourceName, "groups.*",
						map[string]string{
							"name":     "Foo",
							"interval": "180",
							"rules.#":  "2",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(recordingRulesGroupsSetResourceName, "groups.*",
						map[string]string{
							"name":     "Bar",
							"interval": "60",
							"rules.#":  "2",
						},
					),
				),
			},
		},
	})
}

func TestAccCoralogixRecordingRulesGroupsExplicit(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRecordingRulesGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceRecordingRulesGroupsSetExplicit(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(recordingRulesGroupsSetResourceName, "id"),
					resource.TestCheckTypeSetElemNestedAttrs(recordingRulesGroupsSetResourceName, "groups.*",
						map[string]string{
							"name":     "Foo",
							"interval": "180",
							"rules.#":  "2",
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(recordingRulesGroupsSetResourceName, "groups.*",
						map[string]string{
							"name":     "Bar",
							"interval": "60",
							"rules.#":  "2",
						},
					),
				),
			},
		},
	})
}

func testAccCheckRecordingRulesGroupDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).RecordingRuleGroupsSets()
	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_recording_rules_groups_set" {
			continue
		}

		req := &recordingrules.FetchRuleGroupSet{Id: rs.Primary.ID}
		resp, err := client.GetRecordingRuleGroupsSet(ctx, req)
		if err == nil {
			if resp != nil && resp.Id == rs.Primary.ID {
				return fmt.Errorf("coralogix_recording_rules_groups_set still exists: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}

func testAccCoralogixResourceRecordingRulesGroupsSetFromYaml(filePath string) string {
	return fmt.Sprintf(
		`resource "coralogix_recording_rules_groups_set" "test" {
					yaml_content = file("%s")
				}
`, filePath)
}

func testAccCoralogixResourceRecordingRulesGroupsSetExplicit() string {
	return `resource "coralogix_recording_rules_groups_set" test {
            name   = "Name"
            groups = [
              {
                name     = "Foo"
                interval = 180
                limit   = 100
                rules    = [
                  {
                    record = "ts3db_live_ingester_write_latency:3m"
                    expr   = "sum(rate(ts3db_live_ingester_write_latency_seconds_count{CX_LEVEL=\"staging\",pod=~\"ts3db-live-ingester.*\"}[2m])) by (pod)"
                  },
                  {
                    record = "job:http_requests_total:sum"
                    expr   = "sum(rate(http_requests_total[5m])) by (job)"
                  },
                ]
              },
              {
                name     = "Bar"
                interval = 180
                limit   = 100
                rules    = [
                  {
                    record = "ts3db_live_ingester_write_latency:3m"
                    expr   = "sum(rate(ts3db_live_ingester_write_latency_seconds_count{CX_LEVEL=\"staging\",pod=~\"ts3db-live-ingester.*\"}[2m])) by (pod)"
                  },
                  {
                    record = "job:http_requests_total:sum"
                    expr   = "sum(rate(http_requests_total[5m])) by (job)"
                  },
                ]
              },
            ]
		}
`
}
