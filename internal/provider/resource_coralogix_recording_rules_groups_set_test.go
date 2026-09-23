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

package provider

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

var recordingRulesGroupsSetResourceName = "coralogix_recording_rules_groups_set.test"

func TestAccCoralogixRecordingRulesGroupsSetFromYaml(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	parent := filepath.Dir(filepath.Dir(wd))
	filePath := parent + "/examples/resources/coralogix_recording_rules_groups_set/rule-group-set.yaml"
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

func TestAccCoralogixRecordingRulesGroupsSetFromYamlWithName(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-rr-set")
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	parent := filepath.Dir(filepath.Dir(wd))
	filePath := parent + "/examples/resources/coralogix_recording_rules_groups_set/rule-group-set.yaml"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRecordingRulesGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceRecordingRulesGroupsSetFromYamlWithName(filePath, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(recordingRulesGroupsSetResourceName, "id"),
					resource.TestCheckResourceAttr(recordingRulesGroupsSetResourceName, "name", name),
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

func TestAccCoralogixRecordingRulesGroupsSetUpdateName(t *testing.T) {
	var idAfterCreate string
	name := acctest.RandomWithPrefix("tf-acc-rr-set")
	nameUpdated := acctest.RandomWithPrefix("tf-acc-rr-set-upd")
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	parent := filepath.Dir(filepath.Dir(wd))
	filePath := parent + "/examples/resources/coralogix_recording_rules_groups_set/rule-group-set.yaml"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRecordingRulesGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceRecordingRulesGroupsSetFromYamlWithName(filePath, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					func(s *terraform.State) error {
						for resName, rs := range s.RootModule().Resources {
							if rs.Type != "coralogix_recording_rules_groups_set" {
								continue
							}
							if rs.Primary == nil || rs.Primary.ID == "" {
								return fmt.Errorf("resource %s has no primary id", resName)
							}
							idAfterCreate = rs.Primary.ID
							return nil
						}
						return fmt.Errorf("no coralogix_recording_rules_groups_set resource in state")
					},
					resource.TestCheckResourceAttrSet(recordingRulesGroupsSetResourceName, "id"),
					resource.TestCheckResourceAttr(recordingRulesGroupsSetResourceName, "name", name),
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
			{
				Config: testAccCoralogixResourceRecordingRulesGroupsSetFromYamlWithNameUpdated(filePath, nameUpdated),
				Check: resource.ComposeAggregateTestCheckFunc(
					func(s *terraform.State) error {
						for _, rs := range s.RootModule().Resources {
							if rs.Type != "coralogix_recording_rules_groups_set" {
								continue
							}
							if rs.Primary == nil {
								return fmt.Errorf("resource has no primary state")
							}
							if rs.Primary.ID != idAfterCreate {
								return fmt.Errorf("id mismatch: got %s, want %s", rs.Primary.ID, idAfterCreate)
							}
							return nil
						}
						return fmt.Errorf("no coralogix_recording_rules_groups_set resource in state")
					},
					resource.TestCheckResourceAttr(recordingRulesGroupsSetResourceName, "name", nameUpdated),
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
	name := acctest.RandomWithPrefix("tf-acc-rr-set")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRecordingRulesGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceRecordingRulesGroupsSetExplicit(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(recordingRulesGroupsSetResourceName, "id"),
					resource.TestCheckResourceAttr(recordingRulesGroupsSetResourceName, "name", name),
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

// evaluation_delay_ms is a plain Optional attribute: the backend never
// materializes it, distinguishes unset from an explicit 0, and clears it when
// the full-replace PUT omits it. Each apply is followed by an empty-plan check
// so a wrong Computed flag or a zero pointer in the expand cannot hide.
func TestAccCoralogixRecordingRulesGroupsSetEvaluationDelay(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-rr-set")

	// One group, two rules: the first carries the delay, the second omits it.
	delayedAndOmitted := func(delay knownvalue.Check) statecheck.StateCheck {
		return statecheck.ExpectKnownValue(
			recordingRulesGroupsSetResourceName,
			tfjsonpath.New("groups"),
			knownvalue.SetExact([]knownvalue.Check{
				knownvalue.ObjectPartial(map[string]knownvalue.Check{
					"name": knownvalue.StringExact("Foo"),
					"rules": knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							"record":              knownvalue.StringExact("job:http_requests_total:sum"),
							"evaluation_delay_ms": delay,
						}),
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							"record":              knownvalue.StringExact("ts3db_live_ingester_write_latency:3m"),
							"evaluation_delay_ms": knownvalue.Null(),
						}),
					}),
				}),
			}),
		)
	}
	emptyPlan := resource.ConfigPlanChecks{
		PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRecordingRulesGroupDestroy,
		Steps: []resource.TestStep{
			// Set on one rule, omitted on its sibling. The omitted sibling is
			// the assertion that matters: it must stay null, not become 0.
			{
				Config:            testAccCoralogixResourceRecordingRulesGroupsSetEvaluationDelay(name, "evaluation_delay_ms = 60000"),
				ConfigStateChecks: []statecheck.StateCheck{delayedAndOmitted(knownvalue.Int64Exact(60000))},
				ConfigPlanChecks:  emptyPlan,
			},
			// Change the value.
			{
				Config:            testAccCoralogixResourceRecordingRulesGroupsSetEvaluationDelay(name, "evaluation_delay_ms = 300000"),
				ConfigStateChecks: []statecheck.StateCheck{delayedAndOmitted(knownvalue.Int64Exact(300000))},
				ConfigPlanChecks:  emptyPlan,
			},
			// Remove it from config: the full-replace PUT clears the delay.
			{
				Config:            testAccCoralogixResourceRecordingRulesGroupsSetEvaluationDelay(name, ""),
				ConfigStateChecks: []statecheck.StateCheck{delayedAndOmitted(knownvalue.Null())},
				ConfigPlanChecks:  emptyPlan,
			},
			// An explicit 0 is a real value, distinct from unset. Regression
			// guard against anyone later adding a Computed zero default.
			{
				Config:            testAccCoralogixResourceRecordingRulesGroupsSetEvaluationDelay(name, "evaluation_delay_ms = 0"),
				ConfigStateChecks: []statecheck.StateCheck{delayedAndOmitted(knownvalue.Int64Exact(0))},
				ConfigPlanChecks:  emptyPlan,
			},
			{
				ResourceName:      recordingRulesGroupsSetResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckRecordingRulesGroupDestroy(s *terraform.State) error {
	meta := testAccProvider.Meta()
	if meta == nil {
		return nil
	}
	client := meta.(*clientset.ClientSet).RecordingRuleGroupsSets()
	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_recording_rules_groups_set" {
			continue
		}

		resp, _, err := client.RuleGroupSetsFetch(ctx, rs.Primary.ID).Execute()
		if err == nil {
			if resp != nil && resp.Id == rs.Primary.ID {
				return fmt.Errorf("coralogix_recording_rules_groups_set still exists: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}

func testAccCoralogixResourceRecordingRulesGroupsSetFromYamlWithName(filePath, name string) string {
	return fmt.Sprintf(
		`resource "coralogix_recording_rules_groups_set" "test" {
					yaml_content = file("%s")
					name = %q
				}
`, filePath, name)
}

func testAccCoralogixResourceRecordingRulesGroupsSetFromYamlWithNameUpdated(filePath, name string) string {
	return fmt.Sprintf(
		`resource "coralogix_recording_rules_groups_set" "test" {
					yaml_content = file("%s")
					name = %q
				}
`, filePath, name)
}

func testAccCoralogixResourceRecordingRulesGroupsSetFromYaml(filePath string) string {
	return fmt.Sprintf(
		`resource "coralogix_recording_rules_groups_set" "test" {
					yaml_content = file("%s")
				}
`, filePath)
}

func testAccCoralogixResourceRecordingRulesGroupsSetExplicit(name string) string {
	return fmt.Sprintf(`resource "coralogix_recording_rules_groups_set" "test" {
            name   = %q
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
`, name)
}

// firstRuleDelay is either an `evaluation_delay_ms = <n>` assignment or the
// empty string, so the same config exercises set, change and remove.
func testAccCoralogixResourceRecordingRulesGroupsSetEvaluationDelay(name, firstRuleDelay string) string {
	return fmt.Sprintf(`resource "coralogix_recording_rules_groups_set" "test" {
            name   = %q
            groups = [
              {
                name     = "Foo"
                interval = 180
                limit    = 100
                rules    = [
                  {
                    record = "job:http_requests_total:sum"
                    expr   = "sum(rate(http_requests_total[5m])) by (job)"
                    %s
                  },
                  {
                    record = "ts3db_live_ingester_write_latency:3m"
                    expr   = "sum(rate(ts3db_live_ingester_write_latency_seconds_count{CX_LEVEL=\"staging\",pod=~\"ts3db-live-ingester.*\"}[2m])) by (pod)"
                  },
                ]
              },
            ]
		}
`, name, firstRuleDelay)
}
