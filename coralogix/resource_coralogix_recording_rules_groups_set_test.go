// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package coralogix

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"terraform-provider-coralogix/coralogix/clientset"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
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
	filePath := parent + "/examples/resources/coralogix_recording_rules_groups_set/rule-group-set.yaml"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { TestAccPreCheck(t) },
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
		PreCheck:                 func() { TestAccPreCheck(t) },
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

		req := &cxsdk.GetRuleGroupSetRequest{Id: rs.Primary.ID}
		resp, err := client.Get(ctx, req)
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
                interval = 60
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
