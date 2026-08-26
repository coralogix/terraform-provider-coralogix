package provider

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const rulesGroupOrderResourceName = "coralogix_rules_group.test"

func testAccCheckRuleGroupDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).RuleGroups()
	ctx := context.TODO()
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_rules_group" {
			continue
		}
		resp, err := client.Get(ctx, &cxsdk.GetRuleGroupRequest{GroupId: rs.Primary.ID})
		if err == nil && resp.GetRuleGroup().GetId().GetValue() == rs.Primary.ID {
			return fmt.Errorf("rule-group still exists: %s", rs.Primary.ID)
		}
	}
	return nil
}

func testAccRulesGroupPartiallySetRuleOrder(name string) string {
	return fmt.Sprintf(`resource "coralogix_rules_group" "test" {
  name        = "%s"
  description = "one rule sets order, the other leaves it to the provider"

  rule_subgroups {
    rules {
      extract {
        name               = "first"
        source_field       = "text"
        regular_expression = "(?P<first>.*)"
      }
    }

    rules {
      extract {
        name               = "second"
        order              = 1
        source_field       = "text"
        regular_expression = "(?P<second>.*)"
      }
    }
  }
}
`, name)
}

func testAccRulesGroupDuplicateRuleOrder(name string) string {
	return fmt.Sprintf(`resource "coralogix_rules_group" "test" {
  name        = "%s"
  description = "two rules claim the same order"

  rule_subgroups {
    rules {
      extract {
        name               = "first"
        order              = 1
        source_field       = "text"
        regular_expression = "(?P<first>.*)"
      }
    }

    rules {
      extract {
        name               = "second"
        order              = 1
        source_field       = "text"
        regular_expression = "(?P<second>.*)"
      }
    }
  }
}
`, name)
}

func testAccRulesGroupDuplicateSubgroupOrder(name string) string {
	return fmt.Sprintf(`resource "coralogix_rules_group" "test" {
  name        = "%s"
  description = "two rule-subgroups claim the same order"

  rule_subgroups {
    order = 1
    rules {
      extract {
        name               = "first"
        source_field       = "text"
        regular_expression = "(?P<first>.*)"
      }
    }
  }

  rule_subgroups {
    order = 1
    rules {
      extract {
        name               = "second"
        source_field       = "text"
        regular_expression = "(?P<second>.*)"
      }
    }
  }
}
`, name)
}

func testAccRulesGroupUnsetRuleOrder(name string) string {
	return fmt.Sprintf(`resource "coralogix_rules_group" "test" {
  name        = "%s"
  description = "no rule sets order, so the provider numbers them by position"

  rule_subgroups {
    rules {
      extract {
        name               = "first"
        source_field       = "text"
        regular_expression = "(?P<first>.*)"
      }
    }

    rules {
      extract {
        name               = "second"
        source_field       = "text"
        regular_expression = "(?P<second>.*)"
      }
    }
  }
}
`, name)
}

func TestAccCoralogixResourceRulesGroupRejectsAmbiguousOrder(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-rules-group-order")

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckRuleGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config:             testAccRulesGroupPartiallySetRuleOrder(name),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
				ExpectError:        regexp.MustCompile(`rule_subgroups\[0\]\.rules\[1\] sets order = 1 while rule_subgroups\[0\]\.rules\[0\] leaves order unset`),
			},
			{
				Config:             testAccRulesGroupDuplicateRuleOrder(name),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
				ExpectError:        regexp.MustCompile(`rule_subgroups\[0\]\.rules\[0\] and rule_subgroups\[0\]\.rules\[1\] both set order = 1`),
			},
			{
				Config:             testAccRulesGroupDuplicateSubgroupOrder(name),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
				ExpectError:        regexp.MustCompile(`rule_subgroups\[0\] and rule_subgroups\[1\] both set order = 1`),
			},
		},
	})
}

func TestAccCoralogixResourceRulesGroupNumbersUnsetOrderByPosition(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-rules-group-order")

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckRuleGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRulesGroupUnsetRuleOrder(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rulesGroupOrderResourceName, "rule_subgroups.#", "1"),
					resource.TestCheckResourceAttr(rulesGroupOrderResourceName, "rule_subgroups.0.rules.#", "2"),
					resource.TestCheckResourceAttr(rulesGroupOrderResourceName, "rule_subgroups.0.rules.0.extract.0.order", "1"),
					resource.TestCheckResourceAttr(rulesGroupOrderResourceName, "rule_subgroups.0.rules.1.extract.0.order", "2"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:      rulesGroupOrderResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
