package coralogix

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"terraform-provider-coralogix/coralogix/clientset"
	recordingrules "terraform-provider-coralogix/coralogix/clientset/grpc/recording-rules-groups-sets/v1"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
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
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckRecordingRulesGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceRecordingRulesGroupsSetFromYaml(filePath),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(recordingRulesGroupsSetResourceName, "id"),
					resource.TestCheckResourceAttr(recordingRulesGroupsSetResourceName, "group.0.name", "Foo"),
					resource.TestCheckResourceAttr(recordingRulesGroupsSetResourceName, "group.0.interval", "180"),
					resource.TestCheckResourceAttr(recordingRulesGroupsSetResourceName, "group.0.rules.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs(events2metricResourceName, "group.0.rules.*",
						map[string]string{
							"record": "ts3db_live_ingester_write_latency:3m",
							"expr":   `sum(rate(ts3db_live_ingester_write_latency_seconds_count{CX_LEVEL=\"staging\",pod=~\"ts3db-live-ingester.*\"}[2m])) by (pod)`,
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(events2metricResourceName, "group.0.rules.*",
						map[string]string{
							"record": "job:http_requests_total:sum",
							"expr":   "sum(rate(http_requests_total[5m])) by (job)",
						},
					),
					resource.TestCheckResourceAttrSet(recordingRulesGroupsSetResourceName, "id"),
					resource.TestCheckResourceAttr(recordingRulesGroupsSetResourceName, "group.1.name", "Bar"),
					resource.TestCheckResourceAttr(recordingRulesGroupsSetResourceName, "group.1.interval", "60"),
					resource.TestCheckResourceAttr(recordingRulesGroupsSetResourceName, "group.1.rules.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs(events2metricResourceName, "group.0.rules.*",
						map[string]string{
							"record": "ts3db_live_ingester_write_latency:3m",
							"expr":   `sum(rate(ts3db_live_ingester_write_latency_seconds_count{CX_LEVEL=\"staging\",pod=~\"ts3db-live-ingester.*\"}[2m])) by (pod)`,
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(events2metricResourceName, "group.0.rules.*",
						map[string]string{
							"record": "job:http_requests_total:sum",
							"expr":   "sum(rate(http_requests_total[5m])) by (job)",
						},
					),
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
				Config: testAccCoralogixResourceRecordingRulesGroupsSetExplicit(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(recordingRulesGroupsSetResourceName, "id"),
					resource.TestCheckResourceAttr(recordingRulesGroupsSetResourceName, "group.0.name", "Foo"),
					resource.TestCheckResourceAttr(recordingRulesGroupsSetResourceName, "group.0.interval", "180"),
					resource.TestCheckResourceAttr(recordingRulesGroupsSetResourceName, "group.0.rules.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs(events2metricResourceName, "group.0.rules.*",
						map[string]string{
							"record": "ts3db_live_ingester_write_latency:3m",
							"expr":   `sum(rate(ts3db_live_ingester_write_latency_seconds_count{CX_LEVEL=\"staging\",pod=~\"ts3db-live-ingester.*\"}[2m])) by (pod)`,
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(events2metricResourceName, "group.0.rules.*",
						map[string]string{
							"record": "job:http_requests_total:sum",
							"expr":   "sum(rate(http_requests_total[5m])) by (job)",
						},
					),
					resource.TestCheckResourceAttrSet(recordingRulesGroupsSetResourceName, "id"),
					resource.TestCheckResourceAttr(recordingRulesGroupsSetResourceName, "group.1.name", "Bar"),
					resource.TestCheckResourceAttr(recordingRulesGroupsSetResourceName, "group.1.interval", "60"),
					resource.TestCheckResourceAttr(recordingRulesGroupsSetResourceName, "group.1.rules.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs(events2metricResourceName, "group.0.rules.*",
						map[string]string{
							"record": "ts3db_live_ingester_write_latency:3m",
							"expr":   `sum(rate(ts3db_live_ingester_write_latency_seconds_count{CX_LEVEL=\"staging\",pod=~\"ts3db-live-ingester.*\"}[2m])) by (pod)`,
						},
					),
					resource.TestCheckTypeSetElemNestedAttrs(events2metricResourceName, "group.0.rules.*",
						map[string]string{
							"record": "job:http_requests_total:sum",
							"expr":   "sum(rate(http_requests_total[5m])) by (job)",
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
	return `resource "coralogix_recording_rules_groups_set" "test"" {
				name = "Name"
  				group {
  				  name     = "Foo"
  				  interval = 180
  				  rule {
  				    record = "ts3db_live_ingester_write_latency:3m"
  				    expr   = "sum(rate(ts3db_live_ingester_write_latency_seconds_count{CX_LEVEL=\"staging\",pod=~\"ts3db-live-ingester.*\"}[2m])) by (pod)"
  				  }
  				  rule {
  				    record = "job:http_requests_total:sum"
  				    expr   = "sum(rate(http_requests_total[5m])) by (job)"
  				  }
  				}
  				group {
  				  name     = "Bar"
  				  interval = 60
  				  rule {
  				    record = "ts3db_live_ingester_write_latency:3m"
  				    expr   = "sum(rate(ts3db_live_ingester_write_latency_seconds_count{CX_LEVEL=\"staging\",pod=~\"ts3db-live-ingester.*\"}[2m])) by (pod)"
  				  }
  				  rule {
  				    record = "job:http_requests_total:sum"
  				    expr   = "sum(rate(http_requests_total[5m])) by (job)"
  				  }
  				}
			}
`
}
