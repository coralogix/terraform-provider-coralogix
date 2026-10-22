package quotaallocationruleset_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const unspecifiedConfig = `resource "coralogix_quota_allocation_rule_set" "test" {
  rules = [
    {
      allocation = 10
      allocation_type = "unspecified"
      can_overflow = true
      enabled = true
      entity_type = "logs"
    },
  ]
}`

// TestAccCoralogixResourceQuotaAllocationEntityTypeRuleSetUnspecifiedAlias creates a rule with
// allocation_type = "unspecified", the alias of "percentage" that the API stores. Terraform checks
// the plan of a create against the configuration, and allows the changed value only inside a set.
// Every step plans again after the apply and fails when that plan is not empty.
func TestAccCoralogixResourceQuotaAllocationEntityTypeRuleSetUnspecifiedAlias(t *testing.T) {
	run := newRun()
	percentage := resource.TestCheckTypeSetElemNestedAttrs(resourceAddress, "rules.*", map[string]string{
		"entity_type":     "logs",
		"allocation_type": "percentage",
	})
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{Config: config(run, unspecifiedConfig), Check: percentage},
			{Config: config(run, fullConfig), Check: percentage},
			{Config: config(run, unspecifiedConfig), Check: percentage},
			importStep,
		},
	})
}
